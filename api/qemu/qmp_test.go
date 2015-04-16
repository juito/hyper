package qemu

import (
    "testing"
    "net"
    "encoding/json"
)

func testQmpInitHelper(t *testing.T, s string) net.Conn {
    t.Log("connecting to ", s)

    c,err := net.Dial("unix", s)
    if err != nil {
        t.Error("cannot connect to qmp socket", err.Error())
    }

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

    return c
}

func TestQmpHello(t *testing.T) {

    qemuChan := make(chan QemuEvent, 128)
    ctx := initContext("vmid", qemuChan, 1, 128)

    go qmpHandler(ctx)

    c := testQmpInitHelper(t, ctx.qmpSockName)
    defer c.Close()

    c.Write([]byte(`{ "event": "SHUTDOWN", "timestamp": { "seconds": 1265044230, "microseconds": 450486 } }`))

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

func TestQmpDiskSession(t *testing.T) {

    qemuChan := make(chan QemuEvent, 128)
    ctx := initContext("vmid", qemuChan, 1, 128)

    go qmpHandler(ctx)

    c := testQmpInitHelper(t, ctx.qmpSockName)
    defer c.Close()

    ctx.qmp <- newDiskAddSession(ctx, "vol1", "volume", "/dev/dm7", "raw", 5)

    buf := make([]byte, 1024)
    nr,err := c.Read(buf)
    if err != nil {
        t.Error("cannot read command 0 in session", err.Error())
    }
    t.Log("received ", string(buf[:nr]))

    c.Write([]byte(`{ "return": {}}`))

    nr,err = c.Read(buf)
    if err != nil {
        t.Error("cannot read command 1 in session", err.Error())
    }
    t.Log("received ", string(buf[:nr]))

    c.Write([]byte(`{ "return": {}}`))

    msg := <- qemuChan
    if msg.Event() != EVENT_BLOCK_INSERTED {
        t.Error("wrong type of message", msg.Event())
    }

    info := msg.(*BlockdevInsertedEvent)
    t.Log("got block device", info.Name, info.SourceType, info.DeviceName)
}

func TestQmpNetSession(t *testing.T) {

    qemuChan := make(chan QemuEvent, 128)
    ctx := initContext("vmid", qemuChan, 1, 128)

    go qmpHandler(ctx)

    c := testQmpInitHelper(t, ctx.qmpSockName)
    defer c.Close()

    ctx.qmp <- newNetworkAddSession(ctx, "12", "eth0", 0, 3)

    buf := make([]byte, 1024)
    nr,err := c.Read(buf)
    if err != nil {
        t.Error("cannot read command 0 in session", err.Error())
    }
    t.Log("received ", string(buf[:nr]))

    c.Write([]byte(`{ "return": {}}`))

    nr,err = c.Read(buf)
    if err != nil {
        t.Error("cannot read command 1 in session", err.Error())
    }
    t.Log("received ", string(buf[:nr]))

    c.Write([]byte(`{ "return": {}}`))

    msg := <- qemuChan
    if msg.Event() != EVENT_INTERFACE_INSERTED {
        t.Error("wrong type of message", msg.Event())
    }

    info := msg.(*NetDevInsertedEvent)
    t.Log("got net device", info.Address, info.Index, info.DeviceName)
}

func TestSessionQueue(t *testing.T) {
    qemuChan := make(chan QemuEvent, 128)
    ctx := initContext("vmid", qemuChan, 1, 128)

    go qmpHandler(ctx)

    c := testQmpInitHelper(t, ctx.qmpSockName)
    defer c.Close()

    ctx.qmp <- newNetworkAddSession(ctx, "12", "eth0", 0, 3)
    ctx.qmp <- newNetworkAddSession(ctx, "13", "eth1", 1, 4)

    buf := make([]byte, 1024)
    nr,err := c.Read(buf)
    if err != nil {
        t.Error("cannot read command 0 in session", err.Error())
    }
    t.Log("received ", string(buf[:nr]))

    c.Write([]byte(`{ "return": {}}`))

    nr,err = c.Read(buf)
    if err != nil {
        t.Error("cannot read command 1 in session", err.Error())
    }
    t.Log("received ", string(buf[:nr]))

    c.Write([]byte(`{ "return": {}}`))

    msg := <- qemuChan
    if msg.Event() != EVENT_INTERFACE_INSERTED {
        t.Error("wrong type of message", msg.Event())
    }

    info := msg.(*NetDevInsertedEvent)
    t.Log("got block device", info.Address, info.Index, info.DeviceName)
    if info.Address != 0x03 || info.Index != 0 || info.DeviceName != "eth0" {
        t.Error("net dev 0 creation failed")
    }

    nr,err = c.Read(buf)
    if err != nil {
        t.Error("cannot read command 0 in session", err.Error())
    }
    t.Log("received ", string(buf[:nr]))

    c.Write([]byte(`{ "return": {}}`))

    nr,err = c.Read(buf)
    if err != nil {
        t.Error("cannot read command 1 in session", err.Error())
    }
    t.Log("received ", string(buf[:nr]))

    c.Write([]byte(`{ "return": {}}`))

    msg = <- qemuChan
    if msg.Event() != EVENT_INTERFACE_INSERTED {
        t.Error("wrong type of message", msg.Event())
    }

    info = msg.(*NetDevInsertedEvent)
    t.Log("got block device", info.Address, info.Index, info.DeviceName)
    if info.Address != 0x04 || info.Index != 1 || info.DeviceName != "eth1" {
        t.Error("net dev 1 creation failed")
    }

}
