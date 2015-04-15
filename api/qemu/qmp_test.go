package qemu

import (
    "testing"
    "net"
    "encoding/json"
)

func testQmpInitHelper(t *testing.T, s string) {
    t.Log("connecting to ", s)

    c,err := net.Dial("unix", s)
    if err != nil {
        t.Error("cannot connect to qmp socket", err.Error())
    }
    defer c.Close()

    t.Log("connected")

    banner := `{"QMP": {"version": {"qemu": {"micro": 0,"minor": 0,"major": 2},"package": ""},"capabilities": []}}`
    t.Log("Writting", banner)

    nr,err := c.Write([]byte(banner))
    if err != nil {
        t.Error("write banner fail ", err.Error())
    }
    t.Log("wrote hello ", nr)

    buf := make([]byte, 1024)
    nr,err = c.Read(buf)
    if err != nil {
        t.Error("fail to get init message")
    }

    t.Log("received message", string(buf[:nr]))

    var msg interface{}
    err = json.Unmarshal(buf[:nr], &msg)
    if err != nil {
        t.Error("can not read init message to json ", string(buf[:nr]))
    }

    hello := msg.(map[string]interface{})
    if hello["execute"].(string) != "qmp_capabilities" {
        t.Error("message wrong", string(buf[:nr]))
    }

    c.Write([]byte(`{ "return": {}}`))

    c.Write([]byte(`{ "event": "SHUTDOWN", "timestamp": { "seconds": 1265044230, "microseconds": 450486 } }`))
}

func TestQmpHello(t *testing.T) {

    t.Log("begin test qmp")

    qemuChan := make(chan QemuEvent, 128)
    ctx := initContext("vmid", qemuChan, 1, 128)

    t.Log("launcher handler")
    go qmpHandler(ctx)
    t.Log("launcher tester")

    testQmpInitHelper(t, ctx.qmpSockName)
    t.Log("waiting the end")

    ev := <- qemuChan
    if ev.Event() != EVENT_QMP_EVENT {
        t.Error("should got an event")
    }
    event := ev.(*QmpEvent)
    if event.event != "SHUTDOWN" {
        t.Error("message is not shutdown, is ", event.event)
    }

    t.Log("qmp finished")
}