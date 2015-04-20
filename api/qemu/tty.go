package qemu

import (
    "io"
    "net"
    "dvm/lib/glog"
    "strconv"
    "os"
    "fmt"
)

type TtyIO struct {
    Output chan string
    Input  chan interface{}
}

type ttyContext struct {
    socketName  string
    vmConn      *net.UnixConn
    observers   []chan string
    command     chan interface{}
}

func setupTty(name string, conn *net.UnixConn, input chan interface{}) *ttyContext {
    return &ttyContext{
        socketName: name,
        vmConn:     conn,
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
            glog.V(4).Info(tc.socketName, " ", msg)
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
                    glog.V(2).Info(tc.socketName, " Write msg to tty ", msg.(string))
                    _,err := tc.vmConn.Write([]byte(msg.(string)))
                    if err != nil {
                        glog.Error(tc.socketName, " tty write failed: ", err.Error())
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

func attachSerialPort(ctx *QemuContext, index int) {
    sockName := ctx.serialPortPrefix + strconv.Itoa(index) + ".sock"
    os.Remove(sockName)
    sock,err := net.ListenUnix("unix",  &net.UnixAddr{sockName, "unix"})
    if err != nil {
        ctx.hub <- &InitFailedEvent{
            reason: sockName + " init failed " + err.Error(),
        }
        return
    }
    go waitingSerialPort(ctx, sockName, sock, index)
    ctx.qmp <- newSerialPortSession(ctx, sockName, index)
}

func waitingSerialPort(ctx *QemuContext, sockName string, sock *net.UnixListener, index int) {
    conn, err := sock.AcceptUnix()
    if err != nil {
        glog.Error("Accept serial port failed", err.Error())
        ctx.hub <- &InitFailedEvent{
            reason: fmt.Sprintf("#%d serial port init failed: %s", index, err.Error()),
        }
        return
    }
    tc := setupTty(sockName, conn, make(chan interface{}))
    tc.start()

    ctx.hub <- &TtyOpenEvent{
        Index:  index,
        TC:     tc,
    }
}