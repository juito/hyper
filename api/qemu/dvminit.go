package qemu
import (
    "encoding/binary"
    "net"
)

// Message
type VmMessage struct {
    head    [8]byte
    message []byte
}

type DecodedMessage struct {
    code    uint32
    length  uint32
    message []byte
}

func newVmMessage(t uint32, m []byte) *VmMessage {
    msg := &VmMessage{
        message: m,
    }
    binary.BigEndian.PutUint32(msg.head[:], uint32(t))
    binary.BigEndian.PutUint32(msg.head[4:], uint32(len(m)))
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
        length: length,
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
        init.Write(cmd.head)
        init.Write(cmd.message)

        res,err := readVmMessage(init)
        if err != nil {
            //TODO: deal with error
        } else if res.code == INIT_ACK {
            ctx.hub <- &CommandAck{
                msg:res.message,
            }
        }

    }
}