package daemon

import (
	"dvm/engine"
	"dvm/api/pod"
	"dvm/api/qemu"
	"dvm/lib/glog"
	"dvm/api/types"
)

func (daemon *Daemon) CmdPod(job *engine.Job) error {
	podArgs := job.Args[0]
	userPod, err := pod.ProcessPodBytes([]byte(podArgs))
	if err != nil {
		return err
	}
	glog.V(3).Info("Began to run the QEMU process to start the VM!\n")
	qemuPodEvent := make(chan qemu.QemuEvent, 128)
	qemuStatus := make(chan types.QemuResponse)
	qemuStatus.Vmid = userPod.Name
	go qemu.QemuLoop(userPod.Name, qemuPodEvent, &qemuStatus, 1, 128)
	runPodEvent := &qemu.RunPodCommand {
		Spec: userPod,
	}
	qemuPodEvent <- runPodEvent
	// wait for the qemu response
	var qemuResponse types.QemuResponse
	for {
		qemuResponse <-qemuStatus
		if qemuResponse.Name == userPod.Name {
			break
		}
	}
	close(qemuStatus)

	// Prepare the qemu status to client
	v := &engine.Env{}
	v.Set("ID", qemuResponse.Name)
	v.SetInt("Code", qemuResponse.Code)
	v.Set("Cause", qemuResponse.Cause)
	if _, err := v.WriteTo(job.Stdout); err != nil {
		return err
	}

	return nil
}
