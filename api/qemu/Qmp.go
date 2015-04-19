package qemu

import (
    "fmt"
    "encoding/json"
    "net"
    "strconv"
    "dvm/lib/glog"
    "time"
    "syscall"
    "errors"
)

type QmpInteraction interface {
    MessageType() int
}

type QmpQuit struct{}

type QmpInternalError struct { cause string}

type QmpSession struct {
    commands []*QmpCommand
    callback QemuEvent
}

type QmpFinish struct {
    success bool
    reason  map[string]interface{}
    callback QemuEvent
}

type QmpCommand struct {
    Execute string `json:"execute"`
    Arguments map[string]interface{} `json:"arguments,omitempty"`
    Scm []byte `json:"-"`
}

type QmpResponse struct {
    msg QmpInteraction
}

type QmpError struct {
    Cause map[string]interface{} `json:"error"`
}

type QmpResult struct {
    Return map[string]interface{} `json:"return"`
}

type QmpTimeStamp struct {
    Seconds         uint64 `json:"seconds"`
    Microseconds    uint64 `json:"microseconds"`
}

type QmpEvent struct {
    Type        string          `json:"event"`
    Timestamp   QmpTimeStamp    `json:"timestamp"`
    Data        interface{}     `json:"data,omitempty"`
}

func (qmp *QmpQuit)             MessageType() int { return QMP_QUIT }
func (qmp *QmpInternalError)    MessageType() int { return QMP_INTERNAL_ERROR }
func (qmp *QmpSession)          MessageType() int { return QMP_SESSION }
func (qmp *QmpFinish)           MessageType() int { return QMP_FINISH }

func (qmp *QmpResult)           MessageType() int { return QMP_RESULT }

func (qmp *QmpError)            MessageType() int { return QMP_ERROR }

func (qmp *QmpEvent)            MessageType() int { return QMP_EVENT }
func (qmp *QmpEvent)            Event() int { return EVENT_QMP_EVENT }
func (qmp *QmpEvent)            timestamp() uint64 { return qmp.Timestamp.Microseconds + qmp.Timestamp.Seconds * 1000000}

func (qmp *QmpResponse)         UnmarshalJSON(raw []byte) error {
    var tmp map[string]interface{}
    var err error = nil
    json.Unmarshal(raw, &tmp)
    glog.V(2).Info("got a message ", string(raw))
    if _,ok := tmp["event"] ; ok {
        msg := &QmpEvent{}
        err = json.Unmarshal(raw, msg)
        glog.V(2).Info("got event: ", msg.Type)
        qmp.msg = msg
    } else if r,ok := tmp["return"]; ok {
        msg := &QmpResult{}
        switch r.(type) {
            case string:
            msg.Return = map[string]interface{}{
                "return": r.(string),
            }
            default:
            err = json.Unmarshal(raw, msg)
        }
        qmp.msg = msg
    } else if _,ok := tmp["error"]; ok {
        msg := &QmpError{}
        err = json.Unmarshal(raw, msg)
        qmp.msg = msg
    } else if _,ok := tmp["OMP"]; ok {
        qmp.msg = nil
    }
    return err
}

func qmpCmdSend(c *net.UnixConn, cmd *QmpCommand) error {
    msg,err := json.Marshal(*cmd)
    if err != nil {
        return err
    }
    _, err = c.Write(msg)
    return err
}

func qmpReceiver(ch chan QmpInteraction, decoder *json.Decoder) {
    for {
        rsp := &QmpResponse{}
        if err := decoder.Decode(rsp); err != nil {
            ch <- &QmpInternalError{cause:err.Error()}
            return
        }
        msg := rsp.msg
        ch <- msg

        if msg.MessageType() == QMP_EVENT && msg.(*QmpEvent).Type == QMP_EVENT_SHUTDOWN {
            return
        }
    }
}

func qmpInit(s *net.UnixListener) (*net.UnixConn, *json.Decoder, error) {
    var msg map[string]interface{}

    conn, err := s.AcceptUnix()
    if err != nil {
        glog.Error("accept socket error ", err.Error())
        return nil, nil, err
    }
    decoder := json.NewDecoder(conn)

    glog.Info("begin qmp init...")

    err = decoder.Decode(&msg)
    if err != nil {
        glog.Error("get qmp welcome failed: ", err.Error())
        return nil, nil, err
    }

    glog.Info("got qmp welcome, now sending command qmp_capabilities")

    err = qmpCmdSend(conn, &QmpCommand{Execute:"qmp_capabilities"})
    if err != nil {
        glog.Error("qmp_capabilities send failed")
        return nil, nil, err
    }

    glog.Info("waiting for response")
    rsp := &QmpResponse{}
    err = decoder.Decode(rsp)
    if err != nil {
        glog.Error("response receive failed", err.Error())
        return nil, nil, err
    }

    glog.Info("got for response")

    if rsp.msg.MessageType() == QMP_RESULT {
        glog.Info("QMP connection initialized")
        return conn, decoder, nil
    }

    return nil, nil, errors.New("handshake failed")
}

func scsiId2Name(id int) string {
    var ch byte= 'a' + byte(id%26)
    if id >= 26 {
        return scsiId2Name(id/26 - 1) + string(ch)
    }
    return "sd" + string(ch)
}

func newDiskAddSession(ctx *QemuContext, name, sourceType, filename, format string, id int) *QmpSession {
    commands := make([]*QmpCommand, 2)
    commands[0] = &QmpCommand{
        Execute: "human-monitor-command",
        Arguments: map[string]interface{}{
            "command-line":"drive_add dummy file=" +
            filename + ",if=none,id=" + "scsi-disk" + strconv.Itoa(id) + ",format=" + format + ",cache=writeback",
        },
    }
    commands[1] = &QmpCommand{
        Execute: "device_add",
        Arguments: map[string]interface{}{
            "driver":"scsi-hd","bus":"scsi0.0","scsi-id":strconv.Itoa(id),
            "drive":"scsi-disk0","id": "scsi-disk" + strconv.Itoa(id),
        },
    }
    devName := scsiId2Name(id)
    return &QmpSession{
        commands: commands,
        callback: &BlockdevInsertedEvent{
            Name: name,
            SourceType: sourceType,
            DeviceName: devName,
        },
    }
}

func newNetworkAddSession(ctx *QemuContext, fd uint64, device string, index, addr int) *QmpSession {
    busAddr := fmt.Sprintf("0x%x", addr)
    commands := make([]*QmpCommand, 3)
    scm := syscall.UnixRights(int(fd))
    glog.V(1).Infof("send net to qemu at %d", int(fd))
    commands[0] = &QmpCommand{
        Execute: "getfd",
        Arguments: map[string]interface{}{
            "fdname":"fd"+device,
        },
        Scm:scm,
    }
    commands[1] = &QmpCommand{
        Execute: "netdev_add",
        Arguments: map[string]interface{}{
            "type":"tap","id":device,"fd":"fd"+device,
        },
    }
    commands[2] = &QmpCommand{
        Execute: "device_add",
        Arguments: map[string]interface{}{
            "driver":"virtio-net-pci",
            "netdev":device,
            "bus":"pci.0",
            "addr":busAddr,
        },
    }

    return &QmpSession{
        commands: commands,
        callback: &NetDevInsertedEvent{
            Index:      index,
            DeviceName: device,
            Address:    addr,
        },
    }
}

func qmpCommander(handler chan QmpInteraction, conn *net.UnixConn, session *QmpSession, feedback chan QmpInteraction) {
    for _,cmd := range session.commands {
        msg,err := json.Marshal(*cmd)
        if err != nil {
            handler <- &QmpFinish{
                success: false,
                reason: map[string]interface{}{
                    "error": "cannot marshal command",
                },
                callback: session.callback,
            }
            return
        }

        success := false
        var qe *QmpError = nil
        for repeat:=0; !success && repeat < 3; repeat++ {

            if len(cmd.Scm) > 0 {
                glog.V(1).Infof("send cmd with scm (%d bytes) (%d) %s", len(cmd.Scm), repeat +1 , string(msg))
                f,_ := conn.File()
                fd := f.Fd()
                syscall.Sendmsg(int(fd), msg, cmd.Scm, nil, 0)
            } else {
                glog.V(1).Infof("sending command (%d) %s", repeat + 1, string(msg))
                conn.Write(msg)
            }

            res := <-feedback
            switch res.MessageType() {
                case QMP_RESULT:
                success = true
                break
                //success
                case QMP_ERROR:
                glog.Warning("got one qmp error")
                qe = res.(*QmpError)
                time.Sleep(1000*time.Millisecond)
                default:
                //            response,_ := json.Marshal(*res)
                handler <- &QmpFinish{
                    success: false,
                    reason: map[string]interface{}{
                        "error": "unknown received message type",
                        "response": []byte{},
                    },
                    callback: session.callback,
                }
                return
            }
        }

        if ! success {
            handler <- &QmpFinish{
                success: false,
                reason: qe.Cause,
                callback: session.callback,
            }
            return
        }
    }
    handler <- &QmpFinish{
        success: true,
        callback: session.callback,
    }
    return
}

func qmpHandler(ctx *QemuContext) {
    conn,decoder,err := qmpInit(ctx.qmpSock)
    if err != nil {
        glog.Error("failed to initialize QMP connection with qemu", err.Error())
        //TODO: should send back to hub
        return
    }

    buf := []*QmpSession{}
    res := make(chan QmpInteraction, 128)

    //routine for get message
    go qmpReceiver(ctx.qmp, decoder)

    for {
        msg := <-ctx.qmp
        switch msg.MessageType() {
        case QMP_SESSION:
            glog.Info("got new session")
            buf = append(buf, msg.(*QmpSession))
            if len(buf) == 1 {
                go qmpCommander(ctx.qmp, conn, msg.(*QmpSession), res)
            }
        case QMP_FINISH:
            glog.Infof("session finished, buffer size %d", len(buf))
            r := msg.(*QmpFinish)
            if r.success {
                glog.V(1).Info("success ")
                ctx.hub <- r.callback
            } else {
                // TODO: fail...
            }
            buf = buf[1:]
            if len(buf) > 0 {
                go qmpCommander(ctx.qmp, conn, buf[0], res)
            }
        case QMP_RESULT:
            res <- msg
        case QMP_ERROR:
            res <- msg
        case QMP_EVENT:
            ev := msg.(*QmpEvent)
            ctx.hub <- ev
            if ev.Type == QMP_EVENT_SHUTDOWN {
                glog.Info("got QMP shutdown event, quit...")
                return
            }
        case QMP_INTERNAL_ERROR:
            go qmpReceiver(ctx.qmp, decoder)
        case QMP_QUIT:
            return
        }
    }
}

