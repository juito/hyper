package qemu

import (
    "net"
    "os"
    "strconv"
    "sync"
    "dvm/api/pod"
    "log"
)

type QemuContext struct {
    id  string

    cpu     int
    memory  int
    pciAddr int  //next available pci addr for pci hotplug
    scsiId  int  //next available scsi id for scsi hotplug
    kernel  string
    initrd  string

    // Communication Context
    hub chan QemuEvent
    qmp chan QmpInteraction
    vm  chan *DecodedMessage

    qmpSockName string
    dvmSockName string
    consoleSockName string
    shareDir    string

    qmpSock     *net.UnixListener
    dvmSock     *net.UnixListener
    consoleSock *net.UnixListener

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
    imageMap    map[string]*imageInfo
    volumeMap   map[string]*volumeInfo
    networkMap  map[int]*InterfaceCreated
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
    pos         int
}

type volumeInfo struct {
    info        *blockDescriptor
    pos         volumePosition
    readOnly    map[int]bool
}

type volumePosition map[int]string     //containerIdx -> mpoint

type processingList struct {
    adding      *processingMap
    deleting    *processingMap
    finished    *processingMap
}

type processingMap struct {
    containers  map[int]bool
    volumes     map[string]bool
    blockdevs   map[string]bool
    networks    map[int]bool
}

type stateHandler func(ctx *QemuContext, event QemuEvent)

func newDeviceMap() *deviceMap {
    return &deviceMap{
        imageMap:   make(map[string]*imageInfo),
        volumeMap:  make(map[string]*volumeInfo),
        networkMap: make(map[int]*InterfaceCreated),
    }
}

func newProcessingMap() *processingMap{
    return &processingMap{
        containers: make(map[int]bool),    //to be create, and get images,
        volumes:    make(map[string]bool),  //to be create, and get volume
        blockdevs:  make(map[string]bool),  //to be insert to qemu, both volume and images
        networks:   make(map[int]bool),
    }
}

func newProcessingList() *processingList{
    return &processingList{
        adding:     newProcessingMap(),
        deleting:   newProcessingMap(),
        finished:   newProcessingMap(),
    }
}

func initContext(id string, hub chan QemuEvent, cpu, memory int) *QemuContext {

    qmpChannel := make(chan QmpInteraction, 128)
    vmChannel  := make(chan *DecodedMessage, 128)
    defer cleanup(func(){ close(qmpChannel);close(vmChannel)})

    //dir and sockets:
    homeDir := BaseDir + "/" + id + "/"
    qmpSockName := homeDir + QmpSockName
    dvmSockName := homeDir + DvmSockName
    consoleSockName := homeDir + ConsoleSockName
    shareDir    := homeDir + ShareDir

    err := os.MkdirAll(shareDir, 0755)
    if err != nil {
        panic(err)
    }
    defer cleanup(func(){os.RemoveAll(homeDir)})

    mkSureNotExist(qmpSockName)
    mkSureNotExist(dvmSockName)
    mkSureNotExist(consoleSockName)

    consoleSock,err := net.ListenUnix("unix",  &net.UnixAddr{consoleSockName, "unix"})
    if err != nil {
        panic(err)
    }
    defer cleanup(func(){consoleSock.Close()})

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
        pciAddr:    0x04,
        kernel:     Kernel,
        initrd:     Initrd,
        hub:        hub,
        qmp:        qmpChannel,
        vm:         vmChannel,
        qmpSockName: qmpSockName,
        dvmSockName: dvmSockName,
        consoleSockName: consoleSockName,
        shareDir:   ShareDir,
        qmpSock:    qmpSock,
        dvmSock:    dvmSock,
        consoleSock: consoleSock,
        handler:    stateInit,
        userSpec:   nil,
        vmSpec:     nil,
        devices:    newDeviceMap(),
        progress:   newProcessingList(),
        lock:       &sync.Mutex{},
    }
}

func mkSureNotExist(filename string) error {
    if _, err := os.Stat(filename); os.IsNotExist(err) {
        log.Println("no such file: ", filename)
        return nil
    } else if err == nil {
        log.Println("try to remove exist file", filename)
        return os.Remove(filename)
    } else {
        log.Println("can not state file ", filename)
        return err
    }
}

func (pm *processingMap) isEmpty() bool {
    return len(pm.containers) == 0 && len(pm.volumes) == 0 && len(pm.blockdevs) == 0 && len(pm.networks) == 0
}

func (ctx* QemuContext) nextScsiId() int {
    ctx.lock.Lock()
    id := ctx.scsiId
    ctx.scsiId++
    ctx.lock.Unlock()
    return id
}

func (ctx* QemuContext) nextPciAddr() int {
    ctx.lock.Lock()
    addr := ctx.pciAddr
    ctx.pciAddr ++
    ctx.lock.Unlock()
    return addr
}

func (ctx* QemuContext) containerCreated(info *ContainerCreatedEvent) bool {
    ctx.lock.Lock()
    defer ctx.lock.Unlock()

    needInsert := false

    c := &ctx.vmSpec.Containers[info.Index]
    c.Id     = info.Id
    c.Rootfs = info.Rootfs
    c.Fstype = info.Fstype
    c.Cmd    = info.Cmd
    c.Workdir = info.Workdir
    for _,e := range c.Envs {
        if _,ok := info.Envs[e.Env]; ok {
            delete(info.Envs, e.Env)
        }
    }
    for e,v := range info.Envs {
        c.Envs = append(c.Envs, VmEnvironmentVar{Env:e, Value:v,})
    }

    if info.Fstype == "dir" {
        c.Image = info.Image
    } else {
        ctx.devices.imageMap[info.Image] = &imageInfo{
            info: &blockDescriptor{
                name: info.Image, filename: info.Image, format:"raw", fstype:info.Fstype, deviceName: "",},
            pos: info.Index,
        }
        ctx.progress.adding.blockdevs[info.Image] = true
        needInsert = true
    }

    ctx.progress.finished.containers[info.Index] = true
    delete(ctx.progress.adding.containers, info.Index)

    return needInsert
}

func (ctx* QemuContext) volumeReady(info *VolumeReadyEvent) bool {
    ctx.lock.Lock()
    defer ctx.lock.Unlock()

    needInsert := false

    vol := ctx.devices.volumeMap[info.Name]
    vol.info.filename = info.Filepath
    vol.info.format = info.Format
    vol.info.fstype = info.Fstype

    if info.Fstype != "dir" {
        ctx.progress.adding.blockdevs[info.Name] = true
        needInsert = true
    } else {
        for i,mount := range vol.pos {
            ctx.vmSpec.Containers[i].Fsmap = append(ctx.vmSpec.Containers[i].Fsmap, VmFsmapDescriptor{
                Source: info.Filepath,
                Path:   mount,
                ReadOnly: vol.readOnly[i],
            })
        }
    }

    ctx.progress.finished.volumes[info.Name] = true
    if _,ok := ctx.progress.adding.volumes[info.Name] ; ok {
        delete(ctx.progress.adding.volumes, info.Name)
    }

    return needInsert
}

func (ctx* QemuContext) blockdevInserted(info *BlockdevInsertedEvent) {
    ctx.lock.Lock()
    defer ctx.lock.Unlock()

    if info.SourceType == "image" {
        image := ctx.devices.imageMap[info.Name]
        ctx.vmSpec.Containers[image.pos].Image = info.DeviceName
    } else if info.SourceType == "volume" {
        volume := ctx.devices.volumeMap[info.Name]
        volume.info.deviceName = info.DeviceName
        for c,vol := range volume.pos {
            ctx.vmSpec.Containers[c].Volumes = append(ctx.vmSpec.Containers[c].Volumes,
                VmVolumeDescriptor{
                    Device:info.DeviceName,
                    Mount:vol,
                    Fstype:volume.info.fstype,
                    ReadOnly:volume.readOnly[c],
                })
        }
    }

    ctx.progress.finished.blockdevs[info.Name] = true
    if _,ok := ctx.progress.adding.blockdevs[info.Name] ; ok {
        delete(ctx.progress.adding.blockdevs, info.Name)
    }
}

func (ctx *QemuContext) interfaceCreated(info* InterfaceCreated) {
    ctx.lock.Lock()
    defer ctx.lock.Unlock()
    ctx.devices.networkMap[info.Index] = info
}

func (ctx* QemuContext) netdevInserted(info *NetDevInsertedEvent) {
    ctx.lock.Lock()
    defer ctx.lock.Unlock()
    ctx.progress.finished.networks[info.Index] = true
    if _,ok := ctx.progress.adding.networks[info.Index] ; ok {
        delete(ctx.progress.adding.networks, info.Index)
    }
    if len(ctx.progress.adding.networks) == 0 {
        count := len(ctx.devices.networkMap)
        infs := make([]VmNetworkInf, count)
        routes := []VmRoute{}
        for i:=0; i < count ; i++ {
            infs[i].Device    = ctx.devices.networkMap[i].DeviceName
            infs[i].IpAddress = ctx.devices.networkMap[i].IpAddr
            infs[i].NetMask   = ctx.devices.networkMap[i].NetMask

            for _,rl := range ctx.devices.networkMap[i].RouteTable {
                dev := ""
                if rl.ViaThis {
                    dev = infs[i].Device
                }
                routes = append(routes, VmRoute{
                    Dest:       rl.Destination,
                    Gateway:    rl.Gateway,
                    Device:     dev,
                })
            }
        }
        ctx.vmSpec.Interfaces = infs
        ctx.vmSpec.Routes = routes
    }
}

func (ctx* QemuContext) deviceReady() bool {
    return ctx.progress.adding.isEmpty() && ctx.progress.deleting.isEmpty()
}

func (ctx *QemuContext) Close() {
    close(ctx.qmp)
    close(ctx.vm)
    ctx.qmpSock.Close()
    ctx.dvmSock.Close()
    os.Remove(ctx.dvmSockName)
    os.Remove(ctx.qmpSockName)
}

func (ctx *QemuContext) Become(handler stateHandler) {
    ctx.lock.Lock()
    ctx.handler = handler
    ctx.lock.Unlock()
}

func (ctx *QemuContext) QemuArguments() []string {
    platformParams := []string{
        "-machine", "pc-i440fx-2.0,accel=kvm,usb=off", "-global", "kvm-pit.lost_tick_policy=discard", "-cpu", "host",}
    if _, err := os.Stat("/dev/kvm"); os.IsNotExist(err) {
        log.Println("kvm not exist change to no kvm mode")
        platformParams = []string{"-machine", "pc-i440fx-2.0,usb=off", "-cpu", "core2duo",}
    }
    return append(platformParams,
        "-realtime", "mlock=off", "-no-user-config", "-nodefaults", "-no-hpet",
        "-rtc", "base=utc,driftfix=slew", "-no-reboot", "-display", "none", "-boot", "strict=on",
        "-m", strconv.Itoa(ctx.memory), "-smp", strconv.Itoa(ctx.cpu),
        "-kernel", ctx.kernel, "-initrd", ctx.initrd, "-append", "\"console=ttyS0 panic=1\"",
        "-qmp", "unix:" + ctx.qmpSockName, "-serial", "unix:" + ctx.consoleSockName,
        "-device", "virtio-serial-pci,id=virtio-serial0,bus=pci.0,addr=0x2","-device", "virtio-scsi-pci,id=scsi0,bus=pci.0,addr=0x3",
        "-chardev", "socket,id=charch0,path=" + ctx.dvmSockName,
        "-device", "virtserialport,bus=virtio-serial0.0,nr=1,chardev=charch0,id=channel0,name=org.getdvm.channel.0",
        "-fsdev", "local,id=virtio9p,path=" + ctx.shareDir + ",security_model=none",
        "-device", "virtio-9p-pci,fsdev=virtio9p,mount_tag=" + ShareDir,
    )
}

// InitDeviceContext will init device info in context
func (ctx *QemuContext) InitDeviceContext(spec *pod.UserPod, networks int) {
    isFsmap:= make(map[string]bool)

    ctx.lock.Lock()
    defer ctx.lock.Unlock()

    for i:=0; i< networks ; i++ {
        ctx.progress.adding.networks[i] = true
    }

    //classify volumes, and generate device info and progress info
    for _,vol := range spec.Volumes {
        if vol.Source == "" {
            isFsmap[vol.Name]    = false
            ctx.devices.volumeMap[vol.Name] = &volumeInfo{
                info: &blockDescriptor{ name: vol.Name, filename: "", format:"", fstype:"", deviceName:"", },
                pos:  make(map[int]string),
                readOnly: make(map[int]bool),
            }
        } else if vol.Driver == "raw" || vol.Driver == "qcow2" {
            isFsmap[vol.Name]    = false
            ctx.devices.volumeMap[vol.Name] = &volumeInfo{
                info: &blockDescriptor{
                    name: vol.Name, filename: vol.Source, format:vol.Driver, fstype:"ext4", deviceName: "", },
                pos:  make(map[int]string),
                readOnly: make(map[int]bool),
            }
            ctx.progress.adding.blockdevs[vol.Name] = true
        } else if vol.Driver == "vfs" {
            isFsmap[vol.Name]    = true
            ctx.devices.volumeMap[vol.Name] = &volumeInfo{
                info: &blockDescriptor{
                    name: vol.Name, filename: vol.Source, format:vol.Driver, fstype:"dir", deviceName: "", },
                pos:  make(map[int]string),
                readOnly: make(map[int]bool),
            }
        }
        ctx.progress.adding.volumes[vol.Name] = true
    }

    containers := make([]VmContainer, len(spec.Containers))

    for i,container := range spec.Containers {
        vols := []VmVolumeDescriptor{}
        fsmap := []VmFsmapDescriptor{}
        for _,v := range container.Volumes {
            ctx.devices.volumeMap[v.Volume].pos[i] = v.Path
            ctx.devices.volumeMap[v.Volume].readOnly[i] = v.ReadOnly
        }

        envs := make([]VmEnvironmentVar, len(container.Envs))
        for j,e := range container.Envs {
            envs[j] = VmEnvironmentVar{ Env: e.Env, Value: e.Value,}
        }

        restart := "never"
        if len(container.RestartPolicy) > 0 {
            restart = container.RestartPolicy
        }

        containers[i] = VmContainer{
            Id:      "",   Rootfs: "rootfs", Fstype: "ext4", Image:  "",
            Volumes: vols,  Fsmap:   fsmap,   Tty:     "",
            Workdir: "",   Cmd:     nil,     Envs:    envs,
            RestartPolicy: restart,
        }

        ctx.progress.adding.containers[i] = true
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
