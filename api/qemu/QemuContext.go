package qemu

import (
    "net"
    "os"
    "strconv"
    "sync"
    "dvm/api/pod"
)

type QemuContext struct {
    id  string

    cpu     int
    memory  int
    pciAddr int  //next available pci addr for pci hotplug
    kernel  string
    initrd  string

    // Communication Context
    hub chan QemuEvent
    qmp chan QmpInteraction
    vm  chan *VmMessage

    qmpSockName string
    dvmSockName string
    shareDir    string

    qmpSock     *net.UnixListener
    dvmSock     *net.UnixListener

    handler     stateHandler

    // Specification
    userSpec    *pod.UserPod
    vmSpec      *VmPod
    devices     *deviceMap
    progress    *processingList

    // Internal Helper
    lock *sync.Mutex //protect update of context
}

type deviceMap struct {
    imageMap map[string]*imageInfo
    volumeMap map[string]*volumeInfo
    fsmapMap map[string]*fsmapInfo
}

type blockDescriptor struct {
    name        string
    filename    string
    format      string
    fstype      string
    deviceName  string
}

type imageInfo struct {
    info        *blockDescriptor
    pos         imagePosition
}

type volumeInfo struct {
    info        *blockDescriptor
    pos         volumePosition
}

type fsmapInfo struct {
    name        string
    dir         string
    pos         fsmapPosition
}

type imagePosition map[uint]uint        //containerIdx -> imageIdx
type volumePosition map[uint]string     //containerIdx -> mpoint
type fsmapPosition map[uint]string      //containerIdx -> mpoint

func newDeviceMap() *deviceMap {
    return &deviceMap{
        imageMap:   make(map[string]*imageInfo),
        volumeMap:  make(map[string]*volumeInfo),
        fsmapMap:   make(map[string]*fsmapInfo),
    }
}

type processingList struct {
    adding      *processingMap
    deleting    *processingMap
    finished    *processingMap
}

type processingMap struct {
    containers  map[uint]bool
    volumes     map[string]bool
    blockdevs   map[string]bool
    fsmap       map[string]bool
}

func (pm *processingMap) isEmpty() bool {
    return len(pm.containers) == 0 && len(pm.volumes) == 0 && len(pm.blockdevs) == 0 && len(pm.fsmap == 0)
}

type stateHandler func(ctx *QemuContext, event QemuEvent)

func newProcessingMap() *processingMap{
    return &processingMap{
        containers: make(map[uint]bool),    //to be create, and get images,
        volumes:    make(map[string]bool),  //to be create, and get volume
        blockdevs:  make(map[string]bool),  //to be insert to qemu, both volume and images
        fsmap:      make(map[string]bool),  //to be bind in share dir
    }
}

func newProcessingList() *processingList{
    return &processingList{
        adding:     newProcessingMap(),
        deleting:   newProcessingMap(),
        finished:   newProcessingMap(),
    }
}

func (ctx* QemuContext) containerCreated(info *ContainerCreatedEvent) []string {
    ctx.lock.Lock()
    defer ctx.lock.Unlock()

    needInsert := make([]string)

    c := &ctx.vmSpec.Containers[info.Index]
    c.Id     = info.Id
    c.Rootfs = info.Rootfs
    c.Fstype = info.Fstype
    c.Cmd    = info.Cmd
    c.Workdir = info.Workdir
    c.Images = make([]string, len(info.Images))
    for _,e := range c.Envs {
        if _,ok := info.Envs[e.Env]; ok {
            delete(info.Envs, e.Env)
        }
    }
    for e,v := range info.Envs {
        c.Envs = append(c.Envs, VmEnvironmentVar{Env:e, Value:v,})
    }

    for i,image := range info.Images {
        if img,ok := ctx.devices.imageMap[image]; ok {
            img.pos[info.Index] = i
        } else {
            ctx.devices.imageMap[image] = &imageInfo{
                info: &blockDescriptor{ name: image, filename: image, format:"raw", fstype:info.Fstype, deviceName: nil, },
                pos: make(map[uint]uint),
            }
            ctx.devices.imageMap[image].pos[info.Index] = i
        }
        if _,ok := ctx.progress.finished.blockdevs[image]; !ok {
            if _,adding := ctx.progress.adding.blockdevs[image]; !adding {
                ctx.progress.adding.blockdevs[image] = true
                needInsert = append(needInsert, image)
            }
        }
    }

    ctx.progress.finished.containers[info.Index] = true
    delete(ctx.progress.adding.containers, info.Index)

    return needInsert
}

func (ctx* QemuContext) volumeCreated(info *VolumeCreatedEvent) {
    ctx.lock.Lock()
    defer ctx.lock.Unlock()

    vol := ctx.devices.volumeMap[info.Name]
    vol.info.filename = info.Filename
    vol.info.format = info.Format
    vol.info.fstype = info.Fstype

    ctx.progress.finished.volumes[info.Name] == true
    if _,ok := ctx.progress.adding.volumes[info.Name] ; ok {
        delete(ctx.progress.adding.volumes, info.Name)
    }
}

func (ctx* QemuContext) pathBound(info *FsmapBoundEvent) {
    ctx.lock.Lock()
    defer ctx.lock.Unlock()

    path := ctx.devices.fsmapMap[info.Name]
    path.dir = info.Path

    for i,mnt := range path.pos {
        for j,dst := range ctx.vmSpec.Containers[i].Fsmap {
            if dst.Path == mnt {
                ctx.vmSpec.Containers[i].Fsmap[j].Source = info.Path
            }
        }
    }

    ctx.progress.finished.fsmap[info.Name] == true
    if _,ok := ctx.progress.adding.fsmap[info.Name] ; ok {
        delete(ctx.progress.adding.fsmap, info.Name)
    }
}

func (ctx* QemuContext) blockdevInserted(info *BlockdevInsertedEvent) {
    ctx.lock.Lock()
    defer ctx.lock.Unlock()

    if info.SourceType == "image" {
        image := ctx.devices.imageMap[info.Name]
        for c,img := range image.pos {
            ctx.vmSpec.Containers[c].Images[img] = info.DeviceName
        }
    } else if info.SourceType = "volume" {
        volume := ctx.devices.volumeMap[info.Name]
        volume.info.deviceName = info.DeviceName
        for c,vol := range volume.pos {
            for i,v := range ctx.vmSpec.Containers[c].Volumes {
                if v.Mount == vol {
                    ctx.vmSpec.Containers[c].Volumes[i].Device == info.DeviceName
                    ctx.vmSpec.Containers[c].Volumes[i].Fstype == volume.info.fstype
                }
            }
        }
    }

    ctx.progress.finished.blockdevs[info.Name] == true
    if _,ok := ctx.progress.adding.blockdevs[info.Name] ; ok {
        delete(ctx.progress.adding.blockdevs, info.Name)
    }
}

func (ctx* QemuContext) deviceReady() bool {
    return ctx.progress.adding.isEmpty() && ctx.progress.deleting.isEmpty()
}

func initContext(id string, hub chan QemuEvent, cpu, memory int) *QemuContext {

    qmpChannel := make(chan QmpInteraction, 128)
    vmChannel  := make(chan *VmMessage, 128)
    defer cleanup(func(){ close(qmpChannel);close(vmChannel)})

    //dir and sockets:
    homeDir := BaseDir + "/" + id + "/"
    qmpSockName := homeDir + QmpSockName
    dvmSockName := homeDir + DvmSockName
    shareDir    := homeDir + ShareDir

    err := os.MkdirAll(shareDir, 0755)
    if err != nil {
        panic(err)
    }
    defer cleanup(func(){os.RemoveAll(homeDir)})

    qmpSock,err := net.ListenUnix("unix",  &net.UnixAddr{qmpSockName, "unix"})
    if err != nil {
        panic(err)
    }
    defer cleanup(func(){qmpSock.Close()})

    dvmSock,err := net.ListenUnix("unix",  &net.UnixAddr{dvmSockName, "unix"})
    if err != nil {
        panic(err)
    }
    defer cleanup(func(){dvmSock.Close()})

    return &QemuContext{
        id:         id,
        cpu:        cpu,
        memory:     memory,
        pciAddr:    0x01,
        kernel:     Kernel,
        initrd:     Initrd,
        hub:        hub,
        qmp:        qmpChannel,
        vm:         vmChannel,
        homeDir:    homeDir,
        qmpSockName: qmpSockName,
        dvmSockName: dvmSockName,
        shareDir:   shareDir,
        qmpSock:    qmpSock,
        dvmSock:    dvmSock,
        handler:    stateInit,
        userSpec:   nil,
        vmSpec:     nil,
        devices:    newDeviceMap(),
        progress:   newProcessingList(),
        lock:       &sync.Mutex{},
    }
}

func (ctx *QemuContext) Close() {
    close(ctx.qmp)
    close(ctx.vm)
    ctx.qmpSock.Close()
    ctx.dvmSock.Close()
}

func (ctx *QemuContext) Become(handler stateHandler) {
    ctx.lock.Lock()
    ctx.handler = handler
    ctx.lock.Unlock()
}

func (ctx *QemuContext) QemuArguments() []string {
    return []string{
        "-machine", "pc-q35-2.0,accel=kvm,usb=off", "-global", "kvm-pit.lost_tick_policy=discard", "-cpu", "host",
        //"-machine", "pc-q35-2.0,usb=off", "-cpu", "core2duo", // this line for non-kvm env
        "-realtime", "mlock=off", "-no-user-config", "-nodefaults", "-no-acpi", "-no-hpet",
        "-rtc", "base=utc,driftfix=slew", "-no-reboot", "-display", "none", "-serial", "null", "-boot", "strict=on",
        "-m", strconv.Itoa(ctx.memory), "-smp", strconv.Itoa(ctx.cpu),
        "-kernel", ctx.kernel, "-initrd", ctx.initrd, "-append", "panic=1 console=ttyS0",
        "-qmp", "unix:" + ctx.qmpSockName,
        "-device", "virtio-serial-pci,id=virtio-serial0,bus=pci.0,addr=0x00",
        "-chardev", "socket,id=charch0,path=" + ctx.dvmSockName,
        "-device", "virtserialport,bus=virtio-serial0.0,nr=1,chardev=charch0,id=channel0,name=org.getdvm.channel.0",
        "-fsdev", "local,id=virtio9p,path=" + ctx.shareDir + ",security_model=none",
        "-device", "virtio-9p-pci,fsdev=virtio9p,mount_tag=share_dir",
    }
}

// InitDeviceContext will init device info in context
func InitDeviceContext(ctx *QemuContext, spec *pod.UserPod) {
    isFsmap:= make(map[string]bool)

    ctx.lock.Lock()
    defer ctx.lock.Unlock()

    //classify volumes, and generate device info and progress info
    for _,vol := range spec.Volumes {
        if vol.Source == nil || vol.Source == "" {
            isFsmap[vol.Name]    = false
            ctx.devices.volumeMap[vol.Name] = &volumeInfo{
                info: &blockDescriptor{ name: vol.Name, filename: nil, format:nil, fstype:nil, deviceName: nil, },
                pos:  make(map[uint]string),
            }
            ctx.progress.adding.volumes[vol.Name] = true
        } else if vol.Driver == "raw" || vol.Driver == "qcow2" {
            isFsmap[vol.Name]    = false
            ctx.devices.volumeMap[vol.Name] = &volumeInfo{
                info: &blockDescriptor{ name: vol.Name, filename: vol.Source, format:vol.Driver, fstype:"ext4", deviceName: nil, },
                pos:  make(map[uint]string),
            }
            ctx.progress.adding.blockdevs[vol.Name] = true
        } else if vol.Driver == "vfs" {
            isFsmap[vol.Name]    = true
            ctx.devices.fsmapMap[vol.Name] = &fsmapInfo{
                name: vol.Name, dir: nil, pos: make(map[uint]string),
            }
            ctx.progress.adding.fsmap[vol.Name] = true
        }
    }

    containers := make([]VmContainer, len(spec.Containers))

    for i,container := range spec.Containers {
        vols := []VmVolumeDescriptor{}
        fsmap := []VmFsmapDescriptor{}
        for _,v := range container.Volumes {
            if isFsmap[v.Volume] {
                fsmap = append(fsmap, VmFsmapDescriptor{
                    Source: nil,  Path: v.Path, ReadOnly: v.ReadOnly,
                })
                ctx.devices.fsmapMap[v.Volume].pos[i] = v.Path
            } else {
                vols = append(vols, VmVolumeDescriptor{
                    Device: nil, Mount:  v.Path, Fstype: "ext4", ReadOnly: v.ReadOnly,
                })
                ctx.devices.volumeMap[v.Volume].pos[i] = v.Path
            }
        }

        envs := make([]VmEnvironmentVar, len(container.Envs))
        for j,e := range container.Envs {
            envs[j] = VmEnvironmentVar{ Env: e.Env, Value: e.Value,}
        }

        containers[i] = VmContainer{
            Id:      nil,   Rootfs: "rootfs", Fstype: "ext4", Images:  nil,
            Volumes: vols,  Fsmap:   fsmap,   Tty:     nil,
            Workdir: nil,   Cmd:     nil,     Envs:    envs,
            RestartPolicy: container.RestartPolicy,
        }
    }

    ctx.vmSpec = &VmPod{
        Hostname:       spec.Name,
        Containers:     containers,
        Interfaces:     nil,
        Routes:         nil,
        Socket:         ctx.dvmSockName,
        ShareDir:       ctx.shareDir,
    }

    ctx.userSpec = spec
}
