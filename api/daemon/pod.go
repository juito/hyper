package daemon

import (
	"fmt"
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
	vmid := fmt.Sprintf("vm-%s", pod.RandStr(10, "alpha"))
	// store the UserPod into the db
	if err:= daemon.WritePodToDB(userPod.Name, []byte(podArgs)); err != nil {
		return err
	}
	if err := daemon.WritePodAndVM(userPod.Name, vmid); err != nil {
		return err
	}
	glog.V(1).Info("Began to run the QEMU process to start the VM!\n")
	qemuPodEvent := make(chan qemu.QemuEvent, 128)
	qemuStatus := make(chan *types.QemuResponse)

	go qemu.QemuLoop(vmid, qemuPodEvent, qemuStatus, 1, 512)
	runPodEvent := &qemu.RunPodCommand {
		Spec: userPod,
	}
	qemuPodEvent <- runPodEvent
	// wait for the qemu response
	var qemuResponse *types.QemuResponse
	for {
		qemuResponse =<-qemuStatus
		if qemuResponse.VmId == vmid {
			break
		}
	}
	close(qemuStatus)

	// Prepare the qemu status to client
	v := &engine.Env{}
	v.Set("ID", userPod.Name)
	v.SetInt("Code", qemuResponse.Code)
	v.Set("Cause", qemuResponse.Cause)
	if _, err := v.WriteTo(job.Stdout); err != nil {
		return err
	}

	return nil
}
