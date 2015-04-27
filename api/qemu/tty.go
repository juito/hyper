package qemu

import (
    "io"
    "net"
    "dvm/lib/glog"
    "dvm/lib/telnet"
    "strconv"
    "os"
    "time"
    "github.com/kr/pty"
)

type TtyIO struct {
    Output chan string
    Input  chan interface{}
}

type ttyContext struct {
    socketName  string
    vmConn      net.Conn
    observers   []chan string
    command     chan interface{}
}

func setupTty(name string, conn *net.UnixConn, input chan interface{}, tn bool) *ttyContext {
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

    return &ttyContext{
        socketName: name,
        vmConn:     c,
        observers:  []chan string{},
        command:    input,
    }
}

func (tc *ttyContext) Get() *TtyIO {
    ob := make(chan string, 128)
    tc.observers = append(tc.observers, ob)
    return &TtyIO{
        Output: ob,
        Input:  tc.command,
    }
}

func (tc *ttyContext) start() {
    go ttyReceiver(tc)
    go ttyController(tc)
}

func (tc *ttyContext) Drop(tty *TtyIO) {
    obs := []chan string{}
    var tbc chan string = nil
    for _,ob := range tc.observers {
        if ob == tty.Output {
            glog.V(1).Info(tc.socketName, " close unused tty channel")
            tbc = ob
        } else {
            obs = append(obs, ob)
        }
    }
    if tbc != nil {
        tc.observers = obs
        close(tbc)
    }
}

func (tc *ttyContext) closeObservers() {
    glog.V(1).Info("close all observer channels")
    for _,c := range tc.observers {
        close(c)
    }
}

func (tc *ttyContext) sendMessage(msg string) {
    for i,c := range tc.observers {
        select {
        case c <- msg:
            glog.V(4).Infof("%s msg sent to #%d observer", tc.socketName, i)
        default:
            glog.V(4).Infof("%s msg not sent to #%d observer", tc.socketName, i)
        }
    }
}

func ttyReceiver(tc *ttyContext) {
    buf := make([]byte, 1)

    line := []byte{}
    for {
        _,err := tc.vmConn.Read(buf)
        if err == io.EOF {
            glog.Info(tc.socketName, " The end of tty")
            tc.closeObservers()
            tc.command <- io.EOF
            return
        } else if err != nil {
            glog.Warning(tc.socketName, "Unhandled error ", err.Error())
            tc.closeObservers()
            tc.command <- io.EOF
            return
        }

        if buf[0] == '\n' && len(line) > 0 {
            msg := string(line[:len(line)-1])
            glog.V(4).Infof("[%s] %s", tc.socketName, msg)
            tc.sendMessage(msg)
            line = []byte{}
        } else {
            line = append(line, buf[0])
        }
    }
}

func ttyController(tc *ttyContext) {
    looping := true
    for looping {
        msg,ok := <- tc.command
        if ok {
            switch msg.(type) {
                case string:
                    glog.V(2).Infof("%s Write msg to tty %q", tc.socketName, msg.(string))
                    _,err := tc.vmConn.Write([]byte(msg.(string)))
                    if err != nil {
                        glog.Error(tc.socketName, " tty write failed: ", err.Error())
                        looping = false
                        tc.vmConn.Close()
                    }
                case byte:
                    glog.V(2).Infof("%s Write byte msg to tty %d", tc.socketName, msg.(byte))
                    _,err := tc.vmConn.Write([]byte{msg.(byte)})
                    if err != nil {
                        glog.Error(tc.socketName, " tty write byte failed: ", err.Error())
                        looping = false
                        tc.vmConn.Close()
                    }
                case error:
                    if msg.(error) == io.EOF {
                        glog.Info(tc.socketName, " tty receive close signal")
                        looping = false
                        tc.vmConn.Close()
                    }
            }
        } else {
            glog.Info(tc.socketName, " channel closed, quit.")
            looping = false
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
    tc := setupTty(sockName, conn, make(chan interface{}), true)
//    tc.start()
    directConnectConsole(ctx, sockName, tc)

    ctx.hub <- &TtyOpenEvent{
        Index:  index,
        TC:     tc,
    }
}

func directConnectConsole(ctx *QemuContext, sockName string, tc *ttyContext) error {
    pty, console, err := pty.Open()
    if err != nil {
        glog.Error("fail to open pty/tty: ", err.Error())
        ctx.hub <- &InitFailedEvent{
            reason: sockName + " init failed ",
        }
        return err
    }

    go io.Copy(tc.vmConn, os.Stdin)
    go io.Copy(console, tc.vmConn)
    go io.Copy(os.Stdout, pty)

    return nil
}
