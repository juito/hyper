package qemu

import (
    "dvm/api/pod"
    "dvm/api/types"
    "dvm/lib/glog"
    "encoding/json"
    "fmt"
    "time"
)

func onQemuExit(ctx *QemuContext) {
    ctx.client <- &types.QemuResponse{
        VmId: ctx.id,
        Code: types.E_SHUTDOWM,
        Cause: "qemu shut down",
    }
    ctx.Close()
}

func prepareDevice(ctx *QemuContext, spec *pod.UserPod) {
    networks := 1
    ctx.InitDeviceContext(spec, networks)
    res,_ := json.MarshalIndent(*ctx.vmSpec, "    ", "    ")
    glog.V(2).Info("initial vm spec: ",string(res))
    go CreateContainer(spec, ctx.shareDir, ctx.hub)
    if networks > 0 {
        for i:=0; i < networks; i++ {
            name := fmt.Sprintf("eth%d", i)
            addr := ctx.nextPciAddr()
            go CreateInterface(i, addr, name, i == 0, ctx.hub)
        }
    }
    for i:=0; i < len(ctx.userSpec.Containers); i++ {
        go attachSerialPort(ctx, i)
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
        ctx.hub <- &InitFailedEvent{
            reason: "Generated wrong run profile " + err.Error(),
        }
        return
    }
    ctx.vm <- &DecodedMessage{
        code: INIT_STARTPOD,
        message: pod,
    }
}

// state machine
func commonStateHandler(ctx *QemuContext, ev QemuEvent) bool {
    processed := true
    switch ev.Event() {
    case EVENT_QEMU_EXIT:
        glog.Info("Qemu has exit, go to cleaning up")
        ctx.Become(stateCleaningUp)
        onQemuExit(ctx)
    case EVENT_QMP_EVENT:
        event := ev.(*QmpEvent)
        if event.Type == QMP_EVENT_SHUTDOWN {
            glog.Info("Got QMP shutdown event, go to cleaning up")
            ctx.Become(stateCleaningUp)
        } else {
            processed = false
        }
    case ERROR_INTERRUPTED:
        glog.Info("Connection interrupted, quit...")
        ctx.Become(stateTerminating)
        time.AfterFunc(3*time.Second, func(){
            if ctx.handler != nil {
                ctx.hub <- &QemuTimeout{}
            }
        })
    case COMMAND_SHUTDOWN:
        ctx.vm <- &DecodedMessage{ code: INIT_SHUTDOWN, message: []byte{}, }
        time.AfterFunc(3*time.Second, func(){
            if ctx.handler != nil {
                ctx.hub <- &QemuTimeout{}
            }
        })
        glog.Info("shutdown command sent, now get into terminating state")
        ctx.Become(stateTerminating)
    default:
        processed = false
    }
    return processed
}

func deviceInitHandler(ctx *QemuContext, ev QemuEvent) bool {
    processed := true
    switch ev.Event() {
        case EVENT_CONTAINER_ADD:
            info := ev.(*ContainerCreatedEvent)
            needInsert := ctx.containerCreated(info)
            if needInsert {
                sid := ctx.nextScsiId()
                ctx.qmp <- newDiskAddSession(ctx, info.Image, "image", info.Image, "raw", sid)
            }
        case EVENT_VOLUME_ADD:
            info := ev.(*VolumeReadyEvent)
            needInsert := ctx.volumeReady(info)
            if needInsert {
                sid := ctx.nextScsiId()
                ctx.qmp <- newDiskAddSession(ctx, info.Name, "volume", info.Filepath, info.Format, sid)
            }
        case EVENT_BLOCK_INSERTED:
            info := ev.(*BlockdevInsertedEvent)
            ctx.blockdevInserted(info)
        case EVENT_INTERFACE_ADD:
            info := ev.(*InterfaceCreated)
            ctx.interfaceCreated(info)
            ctx.qmp <- newNetworkAddSession(ctx, info.Fd, info.DeviceName, info.Index, info.PCIAddr)
        case EVENT_INTERFACE_INSERTED:
            info := ev.(*NetDevInsertedEvent)
            ctx.netdevInserted(info)
        case EVENT_SERIAL_ADD:
            info := ev.(*SerialAddEvent)
            ctx.serialAttached(info)
        case EVENT_TTY_OPEN:
            info := ev.(*TtyOpenEvent)
            ctx.ttyOpened(info)
        default:
            processed = false
    }
    return processed
}

func initFailureHandler(ctx *QemuContext, ev QemuEvent) bool {
    processed := true
    switch ev.Event() {
        case ERROR_INIT_FAIL:
            reason := ev.(*InitFailedEvent).reason
            ctx.client <- &types.QemuResponse{
                VmId: ctx.id,
                Code: types.E_INIT_FAIL,
                Cause: reason,
            }
        case ERROR_QMP_FAIL:
            reason := "QMP protocol exception"
            if ev.(*DeviceFailed).session != nil {
                reason = "QMP protocol exception: failed while waiting " + EventString(ev.(*DeviceFailed).session.Event())
            }
            glog.Error(reason)
            ctx.client <- &types.QemuResponse{
                VmId: ctx.id,
                Code: types.E_DEVICE_FAIL,
                Cause: reason,
            }
        default:
            processed = false
    }
    return processed
}

func stateInit(ctx *QemuContext, ev QemuEvent) {
    if processed := commonStateHandler(ctx, ev); processed {
        //processed by common
    } else if processed := deviceInitHandler(ctx, ev); processed {
        if ctx.deviceReady() {
            glog.V(1).Info("device ready, could run pod.")
            runPod(ctx)
        }
    } else if processed := initFailureHandler(ctx, ev); processed {
        ctx.hub <- &ShutdownCommand{}
    } else {
        switch ev.Event() {
            case EVENT_INIT_CONNECTED:
                event := ev.(*InitConnectedEvent)
                glog.Info("begin to wait dvm commands")
                go waitCmdToInit(ctx, event.conn)
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
            default:
                glog.Warning("got event during pod initiating")
        }
    }
}

func stateRunning(ctx *QemuContext, ev QemuEvent) {
    if processed := commonStateHandler(ctx, ev); !processed {
        switch ev.Event() {
            case COMMAND_EXEC:
            cmd := ev.(*ExecCommand)
            pkg,err := json.Marshal(*cmd)
            if err != nil {
                ctx.client <- &types.QemuResponse{
                    VmId: ctx.id,
                    Code: types.E_JSON_PARSE_FAIL,
                    Cause: fmt.Sprintf("command %s parse failed", cmd.Command,),
                }
                return
            }
            ctx.vm <- &DecodedMessage{
                code: INIT_EXECCMD,
                message: pkg,
            }
            case COMMAND_ACK:
            ack := ev.(*CommandAck)
            if ack.reply == INIT_EXECCMD {
                glog.Info("exec dvm run confirmed", string(ack.msg))
            } else {
                glog.Warning("[Running] wrong reply to ", string(ack.reply), string(ack.msg))
            }
            case COMMAND_ATTACH:
                cmd := ev.(*AttachCommand)
                if cmd.container == "" { //console
                    glog.V(1).Info("Allocating vm console tty.")
                    cmd.callback <- ctx.consoleTty.Get()
                } else if idx := ctx.Lookup( cmd.container ); idx >= 0 {
                    glog.V(1).Info("Allocating tty for ", cmd.container)
                    tc := ctx.devices.ttyMap[idx]
                    cmd.callback <- tc.Get()
                }
            case COMMAND_DETACH:
                cmd := ev.(*DetachCommand)
                if cmd.container == "" {
                    glog.V(1).Info("Drop vm console tty.")
                    ctx.consoleTty.Drop(cmd.tty)
                } else if idx := ctx.Lookup( cmd.container ); idx >= 0 {
                    glog.V(1).Info("Drop tty for ", cmd.container)
                    tc := ctx.devices.ttyMap[idx]
                    tc.Drop(cmd.tty)
                }
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
                    glog.Info("Shutting down command was accepted by init", string(ack.msg))
                } else {
                    glog.Warning("[Terminating] wrong reply to ", string(ack.reply), string(ack.msg))
                }
            case EVENT_QEMU_TIMEOUT:
                glog.Warning("Qemu did not exit in time, try to stop it")
                ctx.qmp <- newQuitSession()
                time.AfterFunc(10*time.Second, func(){
                    if ctx != nil && ctx.handler != nil {
                        ctx.wdt <- "kill"
                    }
                })
        }
    }
}

func stateCleaningUp(ctx *QemuContext, ev QemuEvent) {
    switch ev.Event() {
        case EVENT_QEMU_EXIT:
            glog.Info("Qemu has exit [cleaning up]")
            onQemuExit(ctx)
            ctx.Become(nil)
        default:
            glog.Warning("got event during pod cleaning up")
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
        ev,ok := <-context.hub
        if !ok {
            glog.Error("hub chan has already been closed")
            break
        }
        glog.V(1).Infof("main event loop got message %d(%s)", ev.Event(), EventString(ev.Event()))
        context.handler(context, ev)
    }
}
