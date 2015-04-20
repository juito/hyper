package daemon

import (
	"fmt"
	"strings"
	"dvm/engine"
	"dvm/api/qemu"
	"dvm/api/types"
	"dvm/lib/glog"
)

func (daemon *Daemon) CmdExec(job *engine.Job) error {
	if len(job.Args) == 0 {
		return fmt.Errorf("Can not execute 'exec' command without any container ID!")
	}
	if len(job.Args) == 1 {
		return fmt.Errorf("Can not execute 'exec' command without any command!")
	}
	podName := job.Args[0]
	command := job.Args[1]

	glog.V(1).Infof("Prepare to execute the command : %s for POD(%s)", command, podName)
	// We need find the vm id which running POD, and stop it
	vmid, err := daemon.GetPodVmByName(podName)
	if err != nil {
		return err
	}

	qemuEvent, qemuStatus, err := daemon.GetQemuChan(string(vmid))
	if err != nil {
		return err
	}

	execCommandEvent := &qemu.ExecCommand {
		Command: strings.Split(command, " "),
	}
	qemuEvent.(chan qemu.QemuEvent) <-execCommandEvent
	// wait for the qemu response
	var qemuResponse *types.QemuResponse
	for {
		qemuResponse =<-qemuStatus.(chan *types.QemuResponse)
		if qemuResponse.Code == types.E_SHUTDOWM {
			break
		}
	}
	close(qemuStatus.(chan *types.QemuResponse))

	// Prepare the qemu status to client
	v := &engine.Env{}
	v.Set("ID", podName)
	v.SetInt("Code", qemuResponse.Code)
	v.Set("Cause", qemuResponse.Cause)
	if _, err := v.WriteTo(job.Stdout); err != nil {
		return err
	}

	return nil
}
