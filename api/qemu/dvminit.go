package qemu
import (
    "encoding/binary"
    "net"
    "dvm/lib/glog"
)

// Message
type DecodedMessage struct {
    code    uint32
    message []byte
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
    for {
        conn, err := ctx.dvmSock.AcceptUnix()
        if err != nil {
            glog.Error("Cannot accept dvm socket ", err.Error())
            ctx.hub <- &InitConnectedEvent{conn:nil}
            return
        }

        glog.Info("Wating for init messages...")
        connected := true

        for connected {
            msg,err := readVmMessage(conn)
            if err != nil {
                connected = false
                glog.Error("read init message failed... ", err.Error())
                conn.Close()
            } else if msg.code == INIT_READY {
                glog.Info("Get init ready message")
                ctx.hub <- &InitConnectedEvent{conn:conn}
                return
            } else {
                glog.Warningf("Get init message %d", msg.code)
                connected = false
                conn.Close()
            }
        }

    }
}

func waitCmdToInit(ctx *QemuContext, init *net.UnixConn) {
    looping := true
    for looping {
        cmd := <- ctx.vm
        if cmd.code == INIT_SHUTDOWN {
            glog.Info("Sending shutdown command, last round of command to init")
            looping = false
        }
        init.Write(newVmMessage(cmd))
        res,err := readVmMessage(init)
        if err != nil {
            //TODO: deal with error
        } else if res.code == INIT_ACK {
            ctx.hub <- &CommandAck{
                reply: cmd.code,
                msg:res.message,
            }
        }

    }
}