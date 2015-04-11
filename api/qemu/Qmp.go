package qemu

import (
    "encoding/json"
    "net"
    "errors"
)

const (
    QMP_COMMAND = iota
    QMP_RESULT
    QMP_ERROR
    QMP_EVENT
    QMP_INTERNAL_ERROR
    QMP_QUIT

    QMP_EVENT_SHUTDOWN = "SHUTDOWN"
)

type QmpWelcome struct {
    QMP     QmpInfo
}

type QmpInfo struct {
    Version qmpVersion  `json:"version"`
    Package string      `json:"package"`
    Cap     []interface{} `json:"capabilities"`
}

type qmpVersion struct {
    Major   int `json:"major"`
    Minor   int `json:"minor"`
    Micro   int `json:"micro"`
}

type QmpInteraction interface {
    MessageType() int
}

type QmpQuit struct{}
func (qmp *QmpQuit) MessageType(){ return QMP_QUIT }

type QmpInternalError struct { cause string}
func (qmp *QmpInternalError) MessageType(){ return QMP_INTERNAL_ERROR }

type QmpCommand struct {
    Execute string `json:"execute"`
    Arguments map[string]interface{} `json:"arguments,omitempty"`
}
func (qmp *QmpCommand) MessageType(){ return QMP_COMMAND }

type QmpResult struct { result map[string]interface{} }
func (qmp *QmpResult) MessageType(){ return QMP_RESULT }

type QmpError struct { cause map[string]interface{} }
func (qmp *QmpError) MessageType(){ return QMP_ERROR }

type QmpEvent struct {
    event       string
    timestamp   uint64
    data        map[string]interface{}
}

func (qmp *QmpEvent) MessageType(){ return QMP_EVENT }
func (qmp *QmpEvent) Event(){ return QmpEmit }

func parseQmpEvent(msg map[string]interface{}) (*QmpEvent,error) {
    ts := genericGetField(msg, "timestamp")
    if ts == nil {
        return nil, errors.New("cannot parse timestamp")
    }

    t := (*ts).(map[string]interface{})
    seconds := genericGetField(t, "seconds")
    microseconds := genericGetField(t, "microseconds")
    data := genericGetField(msg, "data")

    if seconds != nil && microseconds != nil {
        return &QmpEvent{
            event: genericGetField(msg, "event").(string),
            timestamp: seconds.(uint64) * 1000000 + microseconds.(uint64),
            data: data,
        },nil
    } else {
        return nil,errors.New("QMP Event parse failed")
    }
}

func genericGetField(msg map[string]interface{}, field string) *interface{} {
    if v,ok := msg[field]; ok {
        return &v
    }
    return nil
}

func qmpCmdSend(c *net.UnixConn, cmd *QmpCommand) error {
    msg,err := json.Marshal(*cmd)
    if err != nil {
        return err
    }
    _, err = c.Write(msg)
    return err
}

func qmpDecode(msg map[string]interface{}) (QmpInteraction, error) {
    if r,ok := msg["return"] ; ok {
        return &QmpResult{result:r.(map[string]interface{})}, nil
    } else if r,ok := msg["error"] ; ok {
        return &QmpError{cause:r.(map[string]interface{})}, nil
    } else if _,ok := msg["event"] ; ok {
        return parseQmpEvent(msg)
    } else {
        return nil,errors.New("Unhandled message type.")
    }
}

func sockJsonReceive(c *net.UnixConn, b []byte) (*interface{}, error) {
    nr, err := c.Read(b)
    if err != nil {
        return nil,err
    }
    var f interface{}
    err = json.Unmarshal(b[:nr], &f)
    return &f,err
}

func qmpReceiver(ch chan QmpInteraction, conn *net.UnixConn) {
    decoder := json.NewDecoder(conn)
    for {
        var msg map[string]interface{}
        if err := decoder.Decode(&msg); err != nil {
            ch <- &QmpInternalError{cause:err.Error()}
            return
        }
        qmp,err := qmpDecode(msg)
        if err != nil {
            ch <- &QmpInternalError{cause:err.Error()}
            return
        }
        ch <- qmp
        if qmp.MessageType() == QMP_EVENT && QmpEvent(*qmp).event == QMP_EVENT_SHUTDOWN {
            return
        }
    }
}

func qmpInit(s *net.UnixListener) (*net.UnixConn, error) {
    buf := make([]byte, 1024)

    conn, err := s.AcceptUnix()
    if err != nil {
        return nil, err
    }

    _,err = sockJsonReceive(conn, buf)
    if err != nil {
        return nil, err
    }

    err = qmpCmdSend(conn, &QmpCommand{Execute:"qmp_capabilities"})
    if err != nil {
        return nil,err
    }


    msg,err := sockJsonReceive(conn, buf)
    if err != nil {
        return nil, err
    }

    res := msg.(map[string]interface{})
    if _,ok:= res["return"]; ok {
        return conn,nil
    }

    return nil, "handshake failed"
}

func qmpHandler(ctx QemuContext) {
    conn,err := qmpInit(ctx.qmpSock)
    if err != nil {
        //should send back to hub
        return
    }
    //routine for get message
    go qmpReceiver(ctx.qmp, conn)
    for {
        msg := <- ctx.qmp
        switch msg.MessageType() {
        case QMP_COMMAND:
        case QMP_RESULT:
        case QMP_ERROR:
        case QMP_EVENT:
            ev := QmpEvent(*msg)
            ctx.hub <- &ev
            if ev.event == QMP_EVENT_SHUTDOWN {
                return
            }
        case QMP_INTERNAL_ERROR:
            go qmpReceiver(ctx.qmp, conn)
        case QMP_QUIT:
            return
        }
    }
}

