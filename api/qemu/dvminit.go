package qemu
import (
    "encoding/binary"
    "net"
    "dvm/lib/glog"
    "time"
    "fmt"
    "dvm/api/types"
    "encoding/json"
)

// Message
type DecodedMessage struct {
    code    uint32
    message []byte
}

type FinishCmd struct {
    Seq uint64 `json:"seq"`
}

func newVmMessage(m *DecodedMessage) []byte {
    length := len(m.message) + 8
    msg := make([]byte, length)
    binary.BigEndian.PutUint32(msg[:], uint32(m.code))
    binary.BigEndian.PutUint32(msg[4:], uint32(length))
    copy(msg[8:], m.message)
    return msg
}

func readVmMessage(conn *net.UnixConn) (*DecodedMessage,error) {
    needRead := 8
    length   := 0
    read     :=0
    buf := make([]byte, 512)
    res := []byte{}
    for read < needRead {
        want := needRead - read
        if want > 512 {
            want = 512
        }
        glog.V(1).Infof("trying to read %d bytes", want)
        nr,err := conn.Read(buf[:want])
        if err != nil {
            glog.Error("read init data failed", )
            return nil, err
        }

        res = append(res, buf[:nr]...)
        read = read + nr

        glog.V(1).Infof("read %d/%d [length = %d]", read, needRead, length)

        if length == 0 && read >= 8 {
            length = int(binary.BigEndian.Uint32(res[4:8]))
            glog.V(1).Infof("data length is %d", length)
            if length > 8 {
                needRead = length
            }
        }
    }

    return &DecodedMessage{
        code: binary.BigEndian.Uint32(res[:4]),
        message: res[8:],
    },nil


}

func waitInitReady(ctx *QemuContext) {
    ctx.dvmSock.SetDeadline(time.Now().Add(30 * time.Second))
    conn, err := ctx.dvmSock.AcceptUnix()
    if err != nil {
        glog.Error("Cannot accept dvm socket ", err.Error())
        ctx.hub <- &InitFailedEvent{
            reason: "Cannot accept dvm socket " + err.Error(),
        }
        return
    }

    glog.Info("Wating for init messages...")

    msg,err := readVmMessage(conn)
    if err != nil {
        glog.Error("read init message failed... ", err.Error())
        ctx.hub <- &InitFailedEvent{
            reason: "read init message failed... " + err.Error(),
        }
        conn.Close()
    } else if msg.code == INIT_READY {
        glog.Info("Get init ready message")
        ctx.hub <- &InitConnectedEvent{conn:conn}
    } else {
        glog.Warningf("Get init message %d", msg.code)
        ctx.hub <- &InitFailedEvent{
            reason: fmt.Sprintf("Get init message %d", msg.code),
        }
        conn.Close()
    }
}

func waitCmdToInit(ctx *QemuContext, init *net.UnixConn) {
    looping := true
    cmds := []*DecodedMessage{}

    go waitInitAck(ctx, init)

    for looping {
        cmd,ok := <- ctx.vm
        if !ok {
            glog.Info("vm channel closed, quit")
            break
        }
        if cmd.code == INIT_ACK {
            if len(cmds) > 0 {
                ctx.hub <- &CommandAck{
                    reply: cmds[0].code,
                    msg:   cmd.message,
                }
                cmds = cmds[1:]
            } else {
                glog.Error("got ack but no command in queue")
            }
        } else{
            if cmd.code == INIT_SHUTDOWN {
                glog.Info("Sending shutdown command, last round of command to init")
                looping = false
            }
            init.Write(newVmMessage(cmd))
            cmds = append(cmds, cmd)
        }
    }
}

func waitInitAck(ctx *QemuContext, init *net.UnixConn) {
    for {
        res,err := readVmMessage(init)
        if err != nil {
            ctx.hub <- &Interrupted{ reason: "dvminit socket failed " + err.Error(), }
            return
        } else if res.code == INIT_ACK {
            ctx.vm <- res
        } else if res.code == INIT_FINISHCMD {
            jv := FinishCmd{}
            err = json.Unmarshal(res.message, &jv)
            if err != nil {
                glog.Errorf("finish cmd message failed, cannot parse json: ", err.Error())
            } else {
                seq := jv.Seq
                glog.V(1).Infof("got sequence %d", seq)

                if !ctx.consoleTty.findAndClose(seq) {
                    for idx,tty := range ctx.devices.ttyMap {
                        if tty.findAndClose(seq) {
                            glog.V(1).Infof("command on tty %d of container %d has finished, close it", seq, idx)
                            break
                        }
                    }
                } else {
                    glog.V(1).Infof("command on console tty %d has finished, close it", seq)
                }

                ctx.client <- &types.QemuResponse{
                    VmId:  ctx.id,
                    Code:  types.E_EXEC_FINISH,
                    Cause: "Command finished",
                    Data:  seq,
                }
            }
        }
    }
}
