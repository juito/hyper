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
    ctx.removeDMDevice()

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
    for blk,_ := range ctx.progress.adding.blockdevs {
        info := ctx.devices.volumeMap[blk]
        sid := ctx.nextScsiId()
        ctx.qmp <- newDiskAddSession(ctx, info.info.name, "volume", info.info.filename, info.info.format, sid)
    }
}

func setWindowSize(ctx *QemuContext, tag string, size *WindowSize) error {
    if session,ok := ctx.ttySessions[tag] ; ok {
        cmd := map[string]interface{}{
            "seq": session,
            "row": size.Row,
            "column": size.Column,
        }
        msg, err := json.Marshal(cmd)
        if err != nil {
            ctx.client <- &types.QemuResponse{ VmId: ctx.id, Code: types.E_JSON_PARSE_FAIL,
                Cause: fmt.Sprintf("command window size parse failed",),
            }
            return err
        }
        ctx.vm <- &DecodedMessage{
            code: INIT_WINSIZE,
            message: msg,
        }
        return nil
    } else {
        err := fmt.Errorf("cannot resolve client tag %s", tag)
        glog.Error(err.Error())
        ctx.client <- &types.QemuResponse{ VmId: ctx.id, Code: types.E_NO_TTY,
            Cause: err.Error(),
        }
        return err
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
        ctx.vm <- &DecodedMessage{ code: INIT_DESTROYPOD, message: []byte{}, }
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
                if ctx.vmSpec.Containers[c.Index].Fstype != "dir" {
                    for name,image := range ctx.devices.imageMap {
                        if image.pos == c.Index {
                            glog.V(1).Info("need remove image dm file", image.info.filename)
                            ctx.progress.deleting.blockdevs[name] = true
                            go UmountDMDevice(image.info.filename, name, ctx.hub)
                        }
                    }
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
        case EVENT_BLOCK_EJECTED:
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
                vol := ctx.devices.volumeMap[v.Name]
                if vol.info.fstype != "dir" {
                    glog.V(1).Info("need remove dm file ", vol.info.filename)
                    ctx.progress.deleting.blockdevs[vol.info.name] = true
                    go UmountDMDevice(vol.info.filename, vol.info.name, ctx.hub)
                }
            }
        case EVENT_VOLUME_DELETE:
            v := ev.(*BlockdevRemovedEvent)
            if !v.Success && ctx.transition != nil {
                ctx.client <- &types.QemuResponse{
                    VmId: ctx.id,
                    Code: types.E_INIT_FAIL,
                    Cause: "unplug blockdev failed",
                }
                ctx.hub <- &ShutdownCommand{}
            } else {
                if _, ok := ctx.progress.deleting.blockdevs[v.Name]; ok {
                    glog.V(1).Infof("blockdev %s deleted", v.Name)
                    delete(ctx.progress.deleting.blockdevs, v.Name)
                }
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
                        Data: ctx.vmSpec.runningInfo(),
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
                    cmd.Streams.Callback <- &types.QemuResponse{
                        VmId: ctx.id, Code: types.E_BUSY, Cause: "Command Launching", Data: cmd.Sequence,
                    }
                    return
                }
                cmd.Sequence = ctx.nextAttachId()
                pkg,err := json.Marshal(*cmd)
                if err != nil {
                    cmd.Streams.Callback <- &types.QemuResponse{
                        VmId: ctx.id, Code: types.E_JSON_PARSE_FAIL,
                        Cause: fmt.Sprintf("command %s parse failed", cmd.Command,), Data: cmd.Sequence,
                    }
                    return
                }
                ctx.transition = cmd
                ctx.ptys.ptyConnect(ctx, ctx.Lookup(cmd.Container), cmd.Sequence, cmd.Streams)
                ctx.clientReg(cmd.Streams.ClientTag, cmd.Sequence)
                ctx.vm <- &DecodedMessage{
                    code: INIT_EXECCMD,
                    message: pkg,
                }
            case COMMAND_WINDOWSIZE:
                cmd := ev.(*WindowSizeCommand)
                if ctx.userSpec.Tty {
                    setWindowSize(ctx, cmd.ClientTag, cmd.Size)
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
                idx := ctx.Lookup( cmd.Container )
                if idx < 0 || idx > len(ctx.vmSpec.Containers) || ctx.vmSpec.Containers[idx].Tty == 0 {
                    glog.Warning("trying to attach a container, but do not has tty")
                    cmd.Streams.Callback <- &types.QemuResponse{
                        VmId: ctx.id,
                        Code: types.E_NO_TTY,
                        Cause: fmt.Sprintf("tty is not configured for %s", cmd.Container),
                        Data: uint64(0),
                    }
                    return
                }
                session := ctx.vmSpec.Containers[idx].Tty
                glog.V(1).Infof("Connecting tty for %s on session %d", cmd.Container, session)
                ctx.ptys.ptyConnect(ctx, idx, session, cmd.Streams)
                if cmd.Size != nil {
                    ctx.clientReg(cmd.Streams.ClientTag, session)
                    setWindowSize(ctx, cmd.Streams.ClientTag, cmd.Size)
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
            ctx.resetAddr()
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
                if ack.reply == INIT_DESTROYPOD {
                    glog.Info("Destroy pod command was accepted by init", string(ack.msg))
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
            case ERROR_INTERRUPTED:
                glog.V(1).Info("dvm init communication channel closed.")
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
            case ERROR_INTERRUPTED:
                glog.V(1).Info("dvm init communication channel closed.")
            default:
                glog.Warning("got event during pod cleaning up")
        }
    }
}

// main loop

func QemuLoop(dvmId string, hub chan QemuEvent, client chan *types.QemuResponse, cpu, memory int, kernel, initrd string) {
    if kernel == "" {
        kernel = Kernel
    }
    if initrd == "" {
        initrd = Initrd
    }

    context,err := initContext(dvmId, hub, client, cpu, memory, kernel, initrd)
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
    go waitPts(context)

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
