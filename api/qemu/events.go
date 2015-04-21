package qemu

import (
    "net"
    "dvm/api/pod"
)

type QemuEvent interface {
    Event() int
}

type QemuExitEvent struct {
    message string
}

type QemuTimeout struct {}

type InitFailedEvent struct {
    reason string
}

type InitConnectedEvent struct {
    conn *net.UnixConn
}

type RunPodCommand struct {
    Spec *pod.UserPod
}

type ExecCommand struct {
    Command []string `json:"cmd"`
    Container string `json:"container,omitempty"`
}

type ShutdownCommand struct {}

type AttachCommand struct {
    container string
    callback  chan *TtyIO
}

type DetachCommand struct{
    container string
    tty       *TtyIO
}

type CommandAck struct {
    reply   uint32
    msg     []byte
}

type ContainerCreatedEvent struct {
    Index   int
    Id      string
    Rootfs  string
    Image   string          // if fstype is `dir`, this should be a path relative to share_dir
    // which described the mounted aufs or overlayfs dir.
    Fstype  string
    Workdir string
    Cmd     []string
    Envs    map[string]string
}

type VolumeReadyEvent struct {
    Name        string      //volumen name in spec
    Filepath    string      //block dev absolute path, or dir path relative to share dir
    Fstype      string      //"xfs", "ext4" etc. for block dev, or "dir" for dir path
    Format      string      //"raw" (or "qcow2") for volume, no meaning for dir path
}

type BlockdevInsertedEvent struct {
    Name        string
    SourceType  string //image or volume
    DeviceName  string
}

type InterfaceCreated struct {
    Index       int
    PCIAddr     int
    Fd          uint64
    DeviceName  string
    IpAddr      string
    NetMask     string
    RouteTable  []*RouteRule
}

type RouteRule struct {
    Destination string
    Gateway     string
    ViaThis     bool
}

type NetDevInsertedEvent struct {
    Index       int
    DeviceName  string
    Address     int
}

type SerialAddEvent struct {
    Index       int
    PortName    string
}

type TtyOpenEvent struct {
    Index       int
    TC          *ttyContext
}

type DeviceFailed struct {
    session     QemuEvent
}

func (qe* QemuExitEvent)            Event() int { return EVENT_QEMU_EXIT }
func (qe* QemuTimeout)              Event() int { return EVENT_QEMU_TIMEOUT }
func (qe* InitConnectedEvent)       Event() int { return EVENT_INIT_CONNECTED }
func (qe* RunPodCommand)            Event() int { return COMMAND_RUN_POD }
func (qe* ExecCommand)              Event() int { return COMMAND_EXEC }
func (qe* AttachCommand)            Event() int { return COMMAND_ATTACH }
func (qe* DetachCommand)            Event() int { return COMMAND_DETACH }
func (qe* ContainerCreatedEvent)    Event() int { return EVENT_CONTAINER_ADD }
func (qe* VolumeReadyEvent)         Event() int { return EVENT_VOLUME_ADD }
func (qe* BlockdevInsertedEvent)    Event() int { return EVENT_BLOCK_INSERTED }
func (qe* CommandAck)               Event() int { return COMMAND_ACK }
func (qe* InterfaceCreated)         Event() int { return EVENT_INTERFACE_ADD }
func (qe* NetDevInsertedEvent)      Event() int { return EVENT_INTERFACE_INSERTED }
func (qe* ShutdownCommand)          Event() int { return COMMAND_SHUTDOWN }
func (qe* InitFailedEvent)          Event() int { return ERROR_INIT_FAIL }
func (qe* TtyOpenEvent)             Event() int { return EVENT_TTY_OPEN }
func (qe* SerialAddEvent)           Event() int { return EVENT_SERIAL_ADD }
func (qe* DeviceFailed)             Event() int { return ERROR_QMP_FAIL }
