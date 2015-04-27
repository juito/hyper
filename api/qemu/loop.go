package qemu

import (
    "dvm/api/pod"
    "dvm/api/types"
    "dvm/lib/glog"
    "encoding/json"
    "fmt"
    "time"
    "os"
)

func onQemuExit(ctx *QemuContext) {
    ctx.client <- &types.QemuResponse{
        VmId: ctx.id,
        Code: types.E_SHUTDOWM,
        Cause: "qemu shut down",
    }

    ctx.timer = time.AfterFunc(60 * time.Second, func(){ ctx.hub <- &QemuTimeout{} })

    removeDevice(ctx)

    if ctx.deviceReady() {
        glog.V(1).Info("no device to release/remove/umount, quit")
        ctx.timer.Stop()
        ctx.Close()
    }
}

func removeDevice(ctx *QemuContext) {

    ctx.releaseVolumeDir()
    ctx.releaseAufsDir()

    for idx,tty := range ctx.devices.ttyMap {
        glog.V(1).Infof("remove %d tty sock: %s", idx, tty.socketName)
        os.Remove(tty.socketName)
    }

    for idx,nic := range ctx.devices.networkMap {
        glog.V(1).Infof("remove network card %d: %s", idx, nic.IpAddr)
        ctx.progress.deleting.networks[idx] = true
        go ReleaseInterface(idx, nic.IpAddr, nic.Fd, ctx.hub)
    }
}

func detatchDevice(ctx *QemuContext) {

    ctx.releaseVolumeDir()
    ctx.releaseAufsDir()
    ctx.removeVolumeDrive()
    ctx.removeImageDrive()

    for idx,tty := range ctx.devices.ttyMap {
        glog.V(1).Infof("detach %d tty sock: %s", idx, tty.socketName)
        ctx.progress.deleting.serialPorts[idx] = true
        ctx.qmp <- newSerialDelSession(ctx, idx, &SerialDelEvent{Index:idx})
    }

    for idx,nic := range ctx.devices.networkMap {
        glog.V(1).Infof("remove network card %d: %s", idx, nic.IpAddr)
        ctx.progress.deleting.networks[idx] = true
        ctx.qmp <- newNetworkDelSession(ctx, nic.DeviceName, &NetDevRemovedEvent{Index:idx,})
    }
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
        addr := ctx.nextPciAddr()
        go attachSerialPort(ctx, i, addr)
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
        ctx.timer.Stop()
        ctx.Become(stateCleaningUp)
        onQemuExit(ctx)
    case EVENT_QMP_EVENT:
        event := ev.(*QmpEvent)
        if event.Type == QMP_EVENT_SHUTDOWN {
            glog.Info("Got QMP shutdown event, go to cleaning up")
            ctx.timer.Stop()
            ctx.Become(stateCleaningUp)
        } else {
            processed = false
        }
    case ERROR_INTERRUPTED:
        glog.Info("Connection interrupted, quit...")
        ctx.Become(stateTerminating)
        ctx.timer = time.AfterFunc(3*time.Second, func(){
            if ctx.handler != nil {
                ctx.hub <- &QemuTimeout{}
            }
        })
    case COMMAND_SHUTDOWN:
        ctx.vm <- &DecodedMessage{ code: INIT_SHUTDOWN, message: []byte{}, }
        ctx.transition = nil
        ctx.timer = time.AfterFunc(3*time.Second, func(){
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
            ctx.qmp <- newNetworkAddSession(ctx, uint64(info.Fd.Fd()), info.DeviceName, info.Index, info.PCIAddr)
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

func deviceRemoveHandler(ctx *QemuContext, ev QemuEvent) bool {
    processed := true
    switch ev.Event() {
        case EVENT_CONTAINER_DELETE:
            c := ev.(*ContainerUnmounted)
            if !c.Success && ctx.transition != nil {
                ctx.client <- &types.QemuResponse{
                    VmId: ctx.id,
                    Code: types.E_INIT_FAIL,
                    Cause: "unplug previous container failed",
                }
                ctx.hub <- &ShutdownCommand{}
            } else {
                if _,ok := ctx.progress.deleting.containers[c.Index]; ok {
                    glog.V(1).Infof("container %d umounted", c.Index)
                    delete(ctx.progress.deleting.containers, c.Index)
                }
            }
        case EVENT_INTERFACE_DELETE:
            nic := ev.(*InterfaceReleased)
            if !nic.Success && ctx.transition != nil {
                ctx.client <- &types.QemuResponse{
                    VmId: ctx.id,
                    Code: types.E_INIT_FAIL,
                    Cause: "unplug previous container failed",
                }
                ctx.hub <- &ShutdownCommand{}
            } else {
                if _,ok := ctx.progress.deleting.networks[nic.Index]; ok {
                    glog.V(1).Infof("interface %d released", nic.Index)
                    delete(ctx.progress.deleting.networks, nic.Index)
                }
            }
        case EVENT_VOLUME_DELETE:
            v := ev.(*VolumeUnmounted)
            if !v.Success && ctx.transition != nil {
                ctx.client <- &types.QemuResponse{
                    VmId: ctx.id,
                    Code: types.E_INIT_FAIL,
                    Cause: "unplug previous container failed",
                }
                ctx.hub <- &ShutdownCommand{}
            } else {
                if _, ok := ctx.progress.deleting.volumes[v.Name]; ok {
                    glog.V(1).Infof("volume %s umounted", v.Name)
                    delete(ctx.progress.deleting.volumes, v.Name)
                }
            }
        case EVENT_SERIAL_DELETE:
            s := ev.(*SerialDelEvent)
            tty := ctx.devices.ttyMap[s.Index]
            glog.V(1).Infof("remove %d tty sock: %s", s.Index, tty.socketName)
            os.Remove(tty.socketName)
            if _,ok := ctx.progress.deleting.serialPorts[s.Index]; ok {
                delete(ctx.progress.deleting.serialPorts, s.Index)
            }
        case EVENT_INTERFACE_EJECTED:
            n := ev.(*NetDevRemovedEvent)
            nic := ctx.devices.networkMap[n.Index]
            glog.V(1).Infof("release %d interface: %s", n.Index, nic.IpAddr)
            go ReleaseInterface(n.Index, nic.IpAddr, nic.Fd, ctx.hub)
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
        case EVENT_QEMU_TIMEOUT:
            reason := "Start POD timeout"
            ctx.client <- &types.QemuResponse{
                VmId: ctx.id,
                Code: types.E_COMMAND_TIMEOUT,
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
                ctx.transition = ev.(*RunPodCommand)
                ctx.timer = time.AfterFunc(60 * time.Second, func(){ ctx.hub <- &QemuTimeout{} } )
                prepareDevice(ctx, ev.(*RunPodCommand).Spec)
            case COMMAND_ACK:
                ack := ev.(*CommandAck)
                if ack.reply == INIT_STARTPOD {
                    glog.Info("run success", string(ack.msg))
                    ctx.transition = nil
                    ctx.client <- &types.QemuResponse{
                        VmId: ctx.id,
                        Code: types.E_OK,
                        Cause: "Start POD success",
                    }
                    ctx.timer.Stop()
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
            case COMMAND_REPLACE_POD:
                cmd := ev.(*ReplacePodCommand)
                if ctx.transition != nil {
                    ctx.client <- &types.QemuResponse{ VmId: ctx.id, Code: types.E_BUSY, Cause: "Command Running",}
                    return
                }
                ctx.transition = cmd
                ctx.vm <- &DecodedMessage{
                    code:       INIT_STOPPOD,
                    message:    []byte{},
                }
                ctx.Become(statePodTransiting)
            case COMMAND_EXEC:
                cmd := ev.(*ExecCommand)
                if ctx.transition != nil {
                    ctx.client <- &types.QemuResponse{ VmId: ctx.id, Code: types.E_BUSY, Cause: "Command Running",}
                    return
                }
                pkg,err := json.Marshal(*cmd)
                if err != nil {
                    ctx.client <- &types.QemuResponse{ VmId: ctx.id, Code: types.E_JSON_PARSE_FAIL,
                        Cause: fmt.Sprintf("command %s parse failed", cmd.Command,),
                    }
                    return
                }
                ctx.transition = cmd
                ctx.vm <- &DecodedMessage{
                    code: INIT_EXECCMD,
                    message: pkg,
                }
            case COMMAND_ACK:
                ack := ev.(*CommandAck)
                if ack.reply == INIT_EXECCMD {
                    glog.Info("exec dvm run confirmed", string(ack.msg))
                    ctx.transition = nil
                } else {
                    glog.Warning("[Running] wrong reply to ", string(ack.reply), string(ack.msg))
                }
            case COMMAND_ATTACH:
                cmd := ev.(*AttachCommand)
                if cmd.Container == "" { //console
                    glog.V(1).Info("Allocating vm console tty.")
                    cmd.Callback <- ctx.consoleTty.Get()
//                    tc := ctx.devices.ttyMap[0]
//                    cmd.Callback <- tc.Get()
                } else if idx := ctx.Lookup( cmd.Container ); idx >= 0 {
                    glog.V(1).Info("Allocating tty for ", cmd.Container)
                    tc := ctx.devices.ttyMap[idx]
                    cmd.Callback <- tc.Get()
                }
            case COMMAND_DETACH:
                cmd := ev.(*DetachCommand)
                if cmd.Container == "" {
                    glog.V(1).Info("Drop vm console tty.")
                    ctx.consoleTty.Drop(cmd.Tty)
                } else if idx := ctx.Lookup( cmd.Container ); idx >= 0 {
                    glog.V(1).Info("Drop tty for ", cmd.Container)
                    tc := ctx.devices.ttyMap[idx]
                    tc.Drop(cmd.Tty)
                }
            default:
                glog.Warning("got event during pod running")
        }
    }
}

func statePodTransiting(ctx *QemuContext, ev QemuEvent) {
    if processed := commonStateHandler(ctx, ev); processed {
    } else if processed := deviceRemoveHandler(ctx, ev); processed {
        if ctx.deviceReady() {
            glog.V(1).Info("device ready, could run pod.")
            prepareDevice(ctx, ctx.transition.(*ReplacePodCommand).NewSpec)
            ctx.Become(stateInit)
        }
    } else if processed := initFailureHandler(ctx, ev); processed {
        ctx.hub <- &ShutdownCommand{}
    } else {
        switch ev.Event() {
            case COMMAND_ACK:
                ack := ev.(*CommandAck)
                if ack.reply == INIT_STOPPOD {
                    glog.Info("POD stopped ", string(ack.msg))
                    detatchDevice(ctx)
                } else {
                    glog.Warning("[Transiting] wrong reply to ", string(ack.reply), string(ack.msg))
                }
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
                ctx.timer = time.AfterFunc(10*time.Second, func(){
                    if ctx != nil && ctx.handler != nil {
                        ctx.wdt <- "kill"
                    }
                })
        }
    }
}

func stateCleaningUp(ctx *QemuContext, ev QemuEvent) {
    if processed := deviceRemoveHandler(ctx, ev) ; processed {
        if ctx.deviceReady() {
            glog.V(1).Info("all devices released/removed/umounted, quit")
            ctx.timer.Stop()
            ctx.Close()
        }
    } else {
        switch ev.Event() {
            case EVENT_QEMU_EXIT:
                glog.Info("Qemu has exit [cleaning up]")
                onQemuExit(ctx)
            case EVENT_QEMU_TIMEOUT:
                glog.Info("Device removing timeout")
                ctx.Close()
            default:
                glog.Warning("got event during pod cleaning up")
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
        ev,ok := <-context.hub
        if !ok {
            glog.Error("hub chan has already been closed")
            break
        } else if ev == nil {
            glog.V(1).Info("got nil event.")
            continue
        }
        glog.V(1).Infof("main event loop got message %d(%s)", ev.Event(), EventString(ev.Event()))
        context.handler(context, ev)
    }
}
