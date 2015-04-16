package qemu

import (
    "os/exec"
    "net"
    "dvm/api/pod"
    "dvm/api/network"
    "dvm/api/types"
    "dvm/lib/glog"
    "encoding/json"
    "io"
    "strings"
    "fmt"
    "strconv"
)

// Event messages for chan-ctrl

type QemuEvent interface {
    Event() int
}

type QemuExitEvent struct {
    message string
}

type InitConnectedEvent struct {
    conn *net.UnixConn
}

type RunPodCommand struct {
    Spec *pod.UserPod
}

type ShutdownCommand struct {}

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
    Fd          string
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

func (qe* QemuExitEvent)            Event() int { return EVENT_QEMU_EXIT }
func (qe* InitConnectedEvent)       Event() int { return EVENT_INIT_CONNECTED }
func (qe* RunPodCommand)            Event() int { return COMMAND_RUN_POD }
func (qe* ContainerCreatedEvent)    Event() int { return EVENT_CONTAINER_ADD }
func (qe* VolumeReadyEvent)         Event() int { return EVENT_VOLUME_ADD }
func (qe* BlockdevInsertedEvent)    Event() int { return EVENT_BLOCK_INSERTED }
func (qe* CommandAck)               Event() int { return COMMAND_ACK }
func (qe* InterfaceCreated)         Event() int { return EVENT_INTERFACE_ADD }
func (qe* NetDevInsertedEvent)      Event() int { return EVENT_INTERFACE_INSERTED }
func (qe* ShutdownCommand)          Event() int { return COMMAND_SHUTDOWN }

// routines:

func CreateInterface(index int, pciAddr int, name string, isDefault bool, callback chan QemuEvent) {
    inf, err := network.Allocate("")
    if err != nil {
        glog.Error("interface creating failed", err.Error())
        callback <- &InterfaceCreated{
            Index:      index,
            PCIAddr:    pciAddr,
            DeviceName: name,
            Fd:         "",
            IpAddr:     "",
            NetMask:    "",
            RouteTable: nil,
        }
        return
    }

    interfaceGot(index, pciAddr, name, isDefault, callback, inf)
}

func interfaceGot(index int, pciAddr int, name string, isDefault bool, callback chan QemuEvent, inf *network.Settings) {

    ip,nw,err := net.ParseCIDR(fmt.Sprintf("%s/%d", inf.IPAddress, inf.IPPrefixLen))
    if err != nil {
        glog.Error("can not parse cidr")
        callback <- &InterfaceCreated{
            Index:      index,
            PCIAddr:    pciAddr,
            DeviceName: name,
            Fd:         "",
            IpAddr:     "",
            NetMask:    "",
            RouteTable: nil,
        }
        return
    }
    var tmp []byte = nw.Mask
    var mask net.IP = tmp

    rt:=[]*RouteRule{
        &RouteRule{
            Destination: fmt.Sprintf("%s/%d", nw.IP.String(), inf.IPPrefixLen),
            Gateway:"", ViaThis:true,
        },
    }
    if isDefault {
        rt = append(rt, &RouteRule{
            Destination: "0.0.0.0/24",
            Gateway: inf.Gateway, ViaThis: true,
        })
    }

    event := &InterfaceCreated{
        Index:      index,
        PCIAddr:    pciAddr,
        DeviceName: name,
        Fd:         strconv.FormatUint(uint64(inf.File.Fd()), 10),
        IpAddr:     ip.String(),
        NetMask:    mask.String(),
        RouteTable: rt,
    }

    callback <- event
}

func printDebugOutput(tag string, out io.ReadCloser) {
    buf := make([]byte, 1024)
    for {
        n,err:=out.Read(buf)
        if err == io.EOF {
            glog.V(1).Info("%s finish", tag)
            break
        } else if err != nil {
            glog.Error(err)
        }
        glog.V(1).Info("got %s: %s", tag, string(buf[:n]))
    }
}

func waitConsoleOutput(ctx *QemuContext) {
    buf := make([]byte, 1)

    conn, err := ctx.consoleSock.AcceptUnix()
    if err != nil {
        glog.Warning(err.Error())
        return
    }

    line := []byte{}
    for {
        _,err := conn.Read(buf)
        if err == io.EOF {
            glog.Info("The end")
            return
        } else if err != nil {
            glog.Warning("Unhandled error ", err.Error())
            return
        }

        if buf[0] == '\n' && len(line) > 0 {
            glog.V(1).Info("[console] %s", string(line[:len(line)-1]))
            line = []byte{}
        } else {
            line = append(line, buf[0])
        }
    }
}

// launchQemu run qemu and wait it's quit, includes
func launchQemu(ctx *QemuContext) {
    qemu,err := exec.LookPath("qemu-system-x86_64")
    if  err != nil {
        ctx.hub <- &QemuExitEvent{message:"can not find qemu executable"}
        return
    }

    args := ctx.QemuArguments()

    glog.V(1).Info("cmdline arguments: ", strings.Join(args, " "))

    cmd := exec.Command(qemu, args...)

    stderr,err := cmd.StderrPipe()
    if err != nil {
        glog.Warning("Cannot get stderr of qemu")
    }

//    stdout, err := cmd.StdoutPipe()
//    if err != nil {
//        log.Println("Cannot get stderr of qemu")
//    }

    //go printDebugOutput("stdout", stdout)
    go printDebugOutput("stderr", stderr)

    if err := cmd.Start();err != nil {
        glog.Error("try to start qemu failed")
        ctx.hub <- &QemuExitEvent{message:"try to start qemu failed"}
        return
    }

    glog.V(1).Info("Waiting for command to finish...")

    err = cmd.Wait()
    glog.Info("qemu exit with ", err.Error())
    ctx.hub <- &QemuExitEvent{message:"qemu exit with " + err.Error()}
}

func prepareDevice(ctx *QemuContext, spec *pod.UserPod) {
    networks := 1
    ctx.InitDeviceContext(spec, networks)
    go CreateContainer(spec, ctx.shareDir, ctx.hub)
    if networks > 0 {
        // TODO: go create interfaces here
        for i:=0; i < networks; i++ {
            name := fmt.Sprint("eth%d", i)
            addr := ctx.nextPciAddr()
            go CreateInterface(i, addr, name, i == 0, ctx.hub)
        }
    }
    for blk,_ := range ctx.progress.adding.blockdevs {
        info := ctx.devices.volumeMap[blk]
        sid := ctx.nextScsiId()
        ctx.qmp <- newDiskAddSession(ctx, info.info.name, "volume", info.info.filename, info.info.format, sid)
    }
}

func runPod(ctx *QemuContext) {
    pod,err := json.Marshal(*ctx.vmSpec)
    if err != nil {
        //TODO: fail exit
        return
    }
    ctx.vm <- &DecodedMessage{
        code: INIT_STARTPOD,
        message: pod,
    }
}

// state machine
func commonStateHandler(ctx *QemuContext, ev QemuEvent) bool {
    switch ev.Event() {
    case EVENT_QEMU_EXIT:
        glog.Info("Qemu has exit")
        ctx.Close()
        ctx.Become(stateCleaningUp)
        return true
    case EVENT_QMP_EVENT:
        event := ev.(*QmpEvent)
        if event.event == QMP_EVENT_SHUTDOWN {
            glog.Info("Got QMP shutdown event")
            ctx.Close()
            ctx.Become(stateCleaningUp)
            return true
        }
        return false
    case COMMAND_SHUTDOWN:
        ctx.vm <- &DecodedMessage{ code: INIT_SHUTDOWN, message: []byte{}, }
        ctx.Become(stateTerminating)
        return true
    default:
        return false
    }
}

func stateInit(ctx *QemuContext, ev QemuEvent) {
    if processed := commonStateHandler(ctx, ev); !processed {
        switch ev.Event() {
            case EVENT_INIT_CONNECTED:
                event := ev.(*InitConnectedEvent)
                if event.conn != nil {
                    go waitCmdToInit(ctx, event.conn)
                } else {
                    // TODO: fail exit
                }
            case COMMAND_RUN_POD:
                glog.Info("got spec, prepare devices")
                prepareDevice(ctx, ev.(*RunPodCommand).Spec)
            case COMMAND_ACK:
                ack := ev.(*CommandAck)
                if ack.reply == INIT_STARTPOD {
                    glog.Info("run success", string(ack.msg))
                    ctx.client <- &types.QemuResponse{
                        VmId: ctx.id,
                        Code: types.E_OK,
                        Cause: "Start POD success",
                    }
                    ctx.Become(stateRunning)
                } else {
                    glog.Warning("wrong reply to ", string(ack.reply), string(ack.msg))
                }
            case EVENT_CONTAINER_ADD:
                info := ev.(*ContainerCreatedEvent)
                needInsert := ctx.containerCreated(info)
                if needInsert {
                    sid := ctx.nextScsiId()
                    ctx.qmp <- newDiskAddSession(ctx, info.Image, "image", info.Image, "raw", sid)
                } else if ctx.deviceReady() {
                    runPod(ctx)
                }
            case EVENT_VOLUME_ADD:
                info := ev.(*VolumeReadyEvent)
                needInsert := ctx.volumeReady(info)
                if needInsert {
                    sid := ctx.nextScsiId()
                    ctx.qmp <- newDiskAddSession(ctx, info.Name, "volume", info.Filepath, info.Format, sid)
                } else if ctx.deviceReady() {
                    runPod(ctx)
                }
            case EVENT_BLOCK_INSERTED:
                info := ev.(*BlockdevInsertedEvent)
                ctx.blockdevInserted(info)
                if ctx.deviceReady() {
                    runPod(ctx)
                }
            case EVENT_INTERFACE_ADD:
                info := ev.(*InterfaceCreated)
                if info.IpAddr != "" {
                    ctx.interfaceCreated(info)
                    ctx.qmp <- newNetworkAddSession(ctx, info.Fd, info.DeviceName, info.Index, info.PCIAddr)
                } else {
                    ctx.client <- &types.QemuResponse{
                        VmId: ctx.id,
                        Code: types.E_DEVICE_FAIL,
                        Cause: fmt.Sprintf("network interface %d creation fail", info.Index),
                    }
                }
            case EVENT_INTERFACE_INSERTED:
                info := ev.(*NetDevInsertedEvent)
                ctx.netdevInserted(info)
                if ctx.deviceReady() {
                    runPod(ctx)
                }
        }
    }
}

func stateRunning(ctx *QemuContext, ev QemuEvent) {
    if processed := commonStateHandler(ctx, ev); !processed {
        switch ev.Event() {
            default:
                glog.Warning("got event during pod running")
        }
    }
}

func stateTerminating(ctx *QemuContext, ev QemuEvent) {
    if processed := commonStateHandler(ctx, ev); !processed {
        switch ev.Event() {
            case COMMAND_ACK:
                ack := ev.(*CommandAck)
                if ack.reply == INIT_SHUTDOWN {
                    glog.Info("Shutting down", string(ack.msg))
                    ctx.Become(stateRunning)
                } else {
                    glog.Warning("[Terminating] wrong reply to ", string(ack.reply), string(ack.msg))
                }
        }
    }
}

func stateCleaningUp(ctx *QemuContext, ev QemuEvent) {
    if processed := commonStateHandler(ctx, ev); !processed {
        switch ev.Event() {
            default:
        }
    }
}

// main loop

func QemuLoop(dvmId string, hub chan QemuEvent, client chan *types.QemuResponse, cpu, memory int) {
    context,err := initContext(dvmId, hub, client, cpu, memory)
    if err != nil {
        client <- &types.QemuResponse{
            VmId: dvmId,
            Code: types.E_CONTEXT_INIT_FAIL,
            Cause: err.Error(),
        }
        return
    }

    //launch routines
    go qmpHandler(context)
    go waitInitReady(context)
    go launchQemu(context)
    go waitConsoleOutput(context)

    for context != nil && context.handler != nil {
        ev := <-context.hub
        context.handler(context, ev)
    }
}

//func main() {
//    qemuChan := make(chan QemuEvent, 128)
//    go qemuLoop("mydvm", qemuChan, 1, 128)
//    //qemuChan <- podSpec
//    for {
//    }
//}
