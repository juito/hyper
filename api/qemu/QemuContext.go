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

type stateHandler func(ctx *QemuContext, event QemuEvent)

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
