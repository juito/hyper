package daemon

import (
	"dvm/engine"
	"dvm/api/pod"
	"dvm/api/qemu"
	"dvm/lib/glog"
)

func (daemon *Daemon) CmdPod(job *engine.Job) error {
	podArgs := job.Args[0]
	userPod, err := pod.ProcessPodBytes([]byte(podArgs))
	if err != nil {
		return err
	}
	glog.V(3).Info("Began to run the QEMU process to start the VM!\n")
	qemuPodEvent := make(chan qemu.QemuEvent, 128)
	go qemu.QemuLoop(userPod.Name, qemuPodEvent, 1, 128)
	runPodEvent := &qemu.RunPodCommand {
		Spec: userPod,
	}
	qemuPodEvent <- runPodEvent
	return nil
}
