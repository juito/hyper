package daemon

import (
	"fmt"
	"strings"
	"dvm/engine"
	"dvm/lib/glog"
	"dvm/api/qemu"
	"dvm/api/types"
)

func (daemon *Daemon) CmdExec(job *engine.Job) (err error) {
	if len(job.Args) == 0 {
		return fmt.Errorf("Can not execute 'exec' command without any container ID!")
	}
	if len(job.Args) == 1 {
		return fmt.Errorf("Can not execute 'exec' command without any command!")
	}
	typeKey := job.Args[0]
	typeVal := job.Args[1]
	command := job.Args[2]
	tag := job.Args[3]
	var podId string

	// We need find the vm id which running POD, and stop it
	if typeKey == "pod" {
		podId = typeVal
	} else {
		container := typeVal
		glog.V(1).Infof("Get container id is %s", container)
		podId, err = daemon.GetPodByContainer(container)
		if err != nil {
			return
		}
	}
	vmid, err := daemon.GetPodVmByName(podId)
	if err != nil {
		return err
	}

	execCmd := &qemu.ExecCommand{
		Command: strings.Split(command, " "),
		Streams: &qemu.TtyIO{
			Stdin: job.Stdin,
			Stdout: job.Stdout,
			ClientTag: tag,
			Callback: make(chan *types.QemuResponse, 1),
		},
	}

	if typeKey == "pod" {
		execCmd.Container = ""
	} else {
		execCmd.Container = typeVal
	}

	qemuEvent, _, err := daemon.GetQemuChan(string(vmid))
	if err != nil {
		return err
	}

	qemuEvent.(chan qemu.QemuEvent) <- execCmd

	<-execCmd.Streams.Callback
	defer func() {
		glog.V(2).Info("Defer function for exec!")
	} ()
	return nil
}
