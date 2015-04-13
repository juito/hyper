package qemu

import (
    "encoding/binary"
    "os/exec"
    "net"
    "strconv"
    "dvm/api/pod"
)

// helpers
type recoverOp func()

func cleanup(op recoverOp) {
    if err := recover(); err != nil { op() }
}

// Message
type VmMessage struct {
    message []byte
}

func newVmMessage(m string) *VmMessage {
    msg := &VmMessage{}
    msg.message = make([]byte, len(m) +  9)
    binary.BigEndian.PutUint32(msg.message[:], uint32(65))
    binary.BigEndian.PutUint32(msg.message[4:], uint32(len(m)))
    copy(msg.message[8:], m)
    return msg
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

func waitInitReady(ctx *QemuContext) {
    buf := make([]byte, 512)
    for {
        conn, err := ctx.dvmSock.AcceptUnix()
        if err != nil {
            ctx.hub <- &InitConnectedEvent{conn:nil}
            return
        }

        connected := true

        for connected {
            nr, err := conn.Read(buf)
            if err != nil {
                connected = false
            } else if nr == 4 {
                msg := binary.BigEndian.Uint32(buf[:4])
                if msg == 0 {
                    ctx.hub <- &InitConnectedEvent{conn:conn}
                    return
                }
            } else {
                connected = false
                close(conn)
            }
        }

    }
}

func waitCmdToInit(ctx *QemuContext, init *net.UnixConn) {
    for {
        cmd := <- ctx.vm
        init.Write(cmd.message)
        //read any response?
    }
}

func prepareDevice(ctx *QemuContext, spec *pod.UserPod) {
    InitDeviceContext(ctx,spec)
    //call create containers
    //call create volumes
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
                //
            }
        case COMMAND_RUN_POD:
            go prepareDevice(ctx, ev.(*RunPodCommand).Spec)
        case EVENT_CONTAINER_ADD:
            needInserts := ctx.containerCreated(ev.(*ContainerCreatedEvent))
            if len(needInserts) != 0 {

            } else if ctx.deviceReady() {

            }
        }
    }
}

//func stateInitReady

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
