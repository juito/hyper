package qemu
import (
    "encoding/binary"
    "net"
)

// Message
type DecodedMessage struct {
    code    uint32
    message []byte
}

func newVmMessage(m *DecodedMessage) []byte {
    length := len(m.message)
    msg := make([]byte, 8 + length)
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
    res := make([]byte)
    for read < needRead {
        want := needRead - read
        if want > 512 {
            want = 512
        }
        nr,err := conn.Read(buf[:want])
        if err != nil {
            return nil, err
        }

        res = append(res, buf[:nr])
        read = read + nr

        if length == 0 && read >= 8 {
            length = binary.BigEndian.Uint32(res[4:8])
            if length > 0 {
                needRead = needRead + length
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
            ctx.hub <- &InitConnectedEvent{conn:nil}
            return
        }

        connected := true

        for connected {
            msg,err := readVmMessage(conn)
            if err != nil {
                connected = false
                conn.Close()
            } else if msg.code == INIT_READY {
                ctx.hub <- &InitConnectedEvent{conn:conn}
                return
            } else {
                connected = false
                conn.Close()
            }
        }

    }
}

func waitCmdToInit(ctx *QemuContext, init *net.UnixConn) {
    for {
        cmd := <- ctx.vm
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