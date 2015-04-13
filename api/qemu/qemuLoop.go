package qemu

import (
    "os/exec"
    "net"
    "strconv"
    "dvm/api/pod"
    "encoding/json"
)

// helpers
type recoverOp func()

func cleanup(op recoverOp) {
    if err := recover(); err != nil { op() }
}

// Event messages for chan-ctrl

type QemuEvent interface {
    Event() int
}

type QemuExitEvent struct {
    message string
}

type InitConnectedEvent struct {
    conn *net.Conn
}

type RunPodCommand struct {
    Spec *pod.UserPod
}

type CommandAck struct {
    msg []byte
}

type ContainerCreatedEvent struct {
    Index   uint
    Id      string
    Rootfs  string
    Image   string          // if fstype is `dir`, this should be a path relative to share_dir
                            // which described the mounted aufs or overlayfs dir.
    Fstype  string
    Workdir string
    Cmd     []string
    Envs    map[string]string
}

type VolumeReadyEvent struct {
    Name        string      //volumen name in spec
    Filepath    string      //block dev absolute path, or dir path relative to share dir
    Fstype      string      //"xfs", "ext4" etc. for block dev, or "dir" for dir path
    Format      string      //"raw" (or "qcow2") for volume, no meaning for dir path
}

type BlockdevInsertedEvent struct {
    Name        string
    SourceType  string //image or volume
    DeviceName  string
}

func (qe* QemuExitEvent)            Event() int { return EVENT_QEMU_EXIT }
func (qe* InitConnectedEvent)       Event() int { return EVENT_INIT_CONNECTED }
func (qe* RunPodCommand)            Event() int { return COMMAND_RUN_POD }
func (qe* ContainerCreatedEvent)    Event() int { return EVENT_CONTAINER_ADD }
func (qe* VolumeReadyEvent)         Event() int { return EVENT_VOLUME_ADD }
func (qe* BlockdevInsertedEvent)    Event() int { return EVENT_BLOCK_INSERTED }
func (qe* CommandAck)               Event() int { return COMMAND_ACK }

// routines:

// launchQemu run qemu and wait it's quit, includes
func launchQemu(ctx *QemuContext) {
    qemu,err := exec.LookPath("qemu-system-x86_64")
    if  err != nil {
        ctx.hub <- &QemuExitEvent{message:"can not find qemu executable"}
        return
    }

    cmd := exec.Command(qemu, ctx.QemuArguments()...)

    if err := cmd.Start();err != nil {
        ctx.hub <- &QemuExitEvent{message:"try to start qemu failed"}
        return
    }

    err = cmd.Wait()
    ctx.hub <- &QemuExitEvent{message:"qemu exit with " + strconv.Itoa(err)}
}

func prepareDevice(ctx *QemuContext, spec *pod.UserPod) {
    InitDeviceContext(ctx,spec)
    go CreateContainer(spec, ctx.shareDir, ctx.hub)
    for blk,_ := range ctx.progress.adding.blockdevs {
        info := ctx.devices.volumeMap[blk]
        sid := ctx.nextScsiId()
        ctx.qmp <- newDiskAddSession(ctx, info.info.name, "volume", info.info.filename, info.info.format, sid)
    }
    //call create volumes
}

func runPod(ctx *QemuContext) {
    pod,err := json.Marshal(*ctx.vmSpec)
    if err != nil {
        //TODO: fail exit
        return
    }
    ctx.vm <- newVmMessage(0, pod)
}

// state machine
func commonStateHandler(ctx *QemuContext, ev QemuEvent) bool {
    switch ev.Event() {
    case EVENT_QEMU_EXIT:
        ctx.Close()
        ctx.Become(nil)
        return true
    default:
        return false
    }
}

func stateInit(ctx *QemuContext, ev QemuEvent) {
    if processed := commonStateHandler(ctx, ev); !processed {
        switch ev.Event() {
            case EVENT_INIT_CONNECTED:
                if InitConnectedEvent(*ev).conn != nil {
                    go waitCmdToInit(ctx, ev.(*InitConnectedEvent).conn)
                } else {
                    // TODO: fail exit
                }
            case COMMAND_RUN_POD:
                prepareDevice(ctx, ev.(*RunPodCommand).Spec)
            case COMMAND_ACK:
                println("run scucess")
            case EVENT_CONTAINER_ADD:
                info := ev.(*ContainerCreatedEvent)
                needInsert := ctx.containerCreated(info)
                if needInsert {
                    sid := ctx.nextScsiId()
                    ctx.qmp <- newDiskAddSession(ctx, info.Image, "image", info.Image, "raw", sid)
                } else if ctx.deviceReady() {
                    runPod(ctx)
                }
            case EVENT_VOLUME_ADD:
                info := ev.(*VolumeReadyEvent)
                needInsert := ctx.volumeReady(info)
                if needInsert {
                    sid := ctx.nextScsiId()
                    ctx.qmp <- newDiskAddSession(ctx, info.Name, "volume", info.Filepath, info.Format, sid)
                } else if ctx.deviceReady() {
                    runPod(ctx)
                }
            case EVENT_BLOCK_INSERTED:
                info := ev.(*BlockdevInsertedEvent)
                ctx.blockdevInserted(info)
                if ctx.deviceReady() {
                    runPod(ctx)
                }
        }
    }
}

//func stateRunning

// main loop

func qemuLoop(dvmId string, hub chan QemuEvent, cpu, memory int) {
    context := initContext(dvmId, hub, cpu, memory)

    //launch routines
    go qmpHandler(context)
    go waitInitReady(context)
    go launchQemu(context)

    for context != nil && context.handler != nil {
        ev := <-context.hub
        context.handler(context, ev)
    }
}

//func main() {
//    qemuChan := make(chan QemuEvent, 128)
//    go qemuLoop("mydvm", qemuChan, 1, 128)
//    //qemuChan <- podSpec
//    for {
//    }
//}
