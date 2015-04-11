package qemu

import (
    "encoding/binary"
//    "io"
    "os/exec"
    "net"
    "strconv"
)

// constants:

const (
    QemuExit = iota
    QemuConnected
    QmpEmit
    QemuRunPod
    NetworkInfAdd
    NetworkInfDel
    BlockDriveAdd
    BlockDriveDel
    DirAdd
    DirDel
    ContainerAdd
    ContainerDel
    PodCommit
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

type QemuConnection struct {
    conn *net.Conn
}

type QemuRunPodEvent struct {
    spec UserPod
}

func (qe* QemuExitEvent) Event() int {
    return QemuExit
}

func (qe* QemuConnection) Event() int {
    return QemuConnected
}

func (qe* QemuRunPodEvent) Event() int {
    return
}

// routines:

// launchQemu run qemu and wait it's quit, includes
func launchQemu(ctx *QemuContext) {
    qemu,err := exec.LookPath("qemu-system-x86_64")
    if  err != nil {
        ctx.hub <- &QemuExitEvent{message:"can not find qemu executable"}
        return
    }

    cmd := exec.Command(qemu, ctx.QemuArguments()...)

//    stderr,err := cmd.StderrPipe()
//    if err != nil {
//        hub <- &QemuExitEvent{message:"can not get stderr fd connected"}
//        return
//    }
    if err := cmd.Start();err != nil {
        ctx.hub <- &QemuExitEvent{message:"try to start qemu failed"}
        return
    }
//    buf := make([]byte, 1024)
//    for {
//        n,err:=stderr.Read(buf)
//        if err == io.EOF {
//            log.Println("stderr finish")
//            break
//        } else if err != nil {
//            log.Fatal(err)
//        }
//        log.Printf("got stderr: %s", string(buf[:n]))
//    }
    err = cmd.Wait()
    ctx.hub <- &QemuExitEvent{message:"qemu exit with " + strconv.Itoa(err)}
}

func waitInitReady(ctx *QemuContext) {
    buf := make([]byte, 512)
    for {
        conn, err := ctx.dvmSock.AcceptUnix()
        if err != nil {
            ctx.hub <- &QemuConnection{conn:nil}
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
                    ctx.hub <- &QemuConnection{conn:conn}
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

func prepareDevice(ctx *QemuContext, spec *UserPod) {

}

// state machine

func commonStateHandler(ctx *QemuContext, ev QemuEvent) bool {
    switch ev.Event() {
    case QemuExit:
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
        case QemuConnected:
            if QemuConnection(*ev).conn != nil {
                go waitCmdToInit(ctx, QemuConnection(*ev).conn)
            } else {
                //
            }
        case QemuRunPod:
            go prepareDevice(ctx, QemuRunPodEvent(*ev).spec)
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
