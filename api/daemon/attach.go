package daemon

import (
    "fmt"
    "dvm/engine"
    "dvm/lib/glog"
    "dvm/api/qemu"
    "dvm/api/types"
)

func (daemon *Daemon) CmdAttach(job *engine.Job) (err error) {
    if len(job.Args) == 0 {
        return fmt.Errorf("Can not execute 'attach' command without any container/pod ID!")
    }
    if len(job.Args) == 1 {
        return fmt.Errorf("Can not execute 'attach' command without any command!")
    }
    typeKey := job.Args[0]
    typeVal := job.Args[1]
    var podName string

    // We need find the vm id which running POD, and stop it
    if typeKey == "pod" {
        podName = typeVal
    } else {
        container := typeVal
        podName, err = daemon.GetPodByContainer(container)
        if err != nil {
            return
        }
    }
    vmid, err := daemon.GetPodVmByName(podName)
    if err != nil {
        return err
    }
    var (
        ttyIO qemu.TtyIO
        qemuCallback = make(chan *types.QemuResponse, 1)
        qemuResponse *types.QemuResponse
        sequence   uint64
    )

    ttyIO.Stdin = job.Stdin
    ttyIO.Stdout = job.Stdout

    var attachCommand = &qemu.AttachCommand {
        Streams: &ttyIO,
        Size:    nil,
        Callback: qemuCallback,
    }
    if typeKey == "pod" {
        attachCommand.Container = ""
    } else {
        attachCommand.Container = typeVal
    }
    qemuEvent, qemuStatus, err := daemon.GetQemuChan(string(vmid))
    if err != nil {
        return err
    }
    qemuEvent.(chan qemu.QemuEvent) <-attachCommand
    attachResponse := <-qemuCallback
    sequence = attachResponse.Data.(uint64)

    for {
        qemuResponse =<-qemuStatus.(chan *types.QemuResponse)
        if qemuResponse.Data == sequence {
            break
        }
    }
    defer func() {
        glog.V(2).Info("Defer function for exec!")
    } ()
    return nil
}
