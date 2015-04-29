package qemu

import (
    "io"
    "net"
    "dvm/lib/glog"
    "dvm/lib/telnet"
    "strconv"
    "os"
    "time"
)

type WindowSize struct {
    Row         uint16 `json:"row"`
    Column      uint16 `json:"column"`
}

type TtyIO struct {
    Stdin   io.ReadCloser
    Stdout  io.Writer
}

type ttyContext struct {
    socketName  string
    vmConn      net.Conn
    command     chan interface{}
}

func setupTty(name string, conn *net.UnixConn, tn bool, initIO *TtyIO) *ttyContext {
    var c net.Conn = conn
    if tn == true {
        tc,err := telnet.NewConn(conn)
        if err != nil {
            glog.Error("fail to init telnet connection to ", name, ": ", err.Error())
            return nil
        }
        glog.V(1).Infof("connected %s as telnet mode.", name)
        c = tc
    }

    ttyc := &ttyContext{
        socketName: name,
        vmConn:     c,
    }

    ttyc.connect(initIO)
    return ttyc
}

func (tc *ttyContext) connect(tty *TtyIO) {
    if tty.Stdin != nil {
        go io.Copy(tc.vmConn, tty.Stdin)
    }
    if tty.Stdout != nil {
        go io.Copy(tty.Stdout, tc.vmConn)
    }
}

func DropAllTty() *TtyIO {
    r,w := io.Pipe()
    go func(){
        buf := make([]byte, 256)
        for {
            _,err := r.Read(buf)
            if err != nil {
                return
            }
        }
    }()
    return &TtyIO{
        Stdin:  nil,
        Stdout: w,
    }
}

func LinerTty(output chan string) *TtyIO {
    r,w := io.Pipe()
    go ttyLiner(r, output)
    return &TtyIO{
        Stdin:  nil,
        Stdout: w,
    }
}

func ttyLiner(conn io.Reader, output chan string) {
    buf     := make([]byte, 1)
    line    := []byte{}
    cr      := false
    emit    := false
    for {

        nr,err := conn.Read(buf)
        if err != nil || nr < 1 {
            glog.V(1).Info("Input byte chan closed, close the output string chan")
            close(output)
            return
        }
        switch buf[0] {
            case '\n':
            emit = !cr
            cr = false
            case '\r':
            emit = true
            cr = true
            default:
            cr = false
            line = append(line, buf[0])
        }
        if emit {
            output <- string(line)
            line = []byte{}
            emit = false
        }
    }
}

func attachSerialPort(ctx *QemuContext, index,addr int) {
    sockName := ctx.serialPortPrefix + strconv.Itoa(index) + ".sock"
    os.Remove(sockName)
    ctx.qmp <- newSerialPortSession(ctx, sockName, index, addr)
//    ctx.qmp <- newSerialPortSession(ctx, sockName, index)

    for i:=0; i < 5; i++ {
        conn, err := net.Dial("unix", sockName)
        if err == nil {
            glog.V(1).Info("connected to ", sockName)
            go connSerialPort(ctx, sockName, conn.(*net.UnixConn), index)
            return
        }
        glog.Warningf("connect %s %d attempt: %s", sockName, i, err.Error())
        time.Sleep(200 * time.Millisecond)
    }

    ctx.hub <- &InitFailedEvent{
        reason: sockName + " init failed ",
    }
}

func connSerialPort(ctx *QemuContext, sockName string, conn *net.UnixConn, index int) {
    tc := setupTty(sockName, conn, true, DropAllTty())
//    directConnectConsole(ctx, sockName, tc)

    ctx.hub <- &TtyOpenEvent{
        Index:  index,
        TC:     tc,
    }
}