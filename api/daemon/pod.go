package daemon

import (
	"fmt"
	"dvm/engine"
	"dvm/api/pod"
	"dvm/api/qemu"
)

func (daemon *Daemon) CmdPod(job *engine.Job) error {
	podArgs := job.Args[0]
	cli := daemon.dockerCli
	userPod, err := pod.ProcessPodBytes([]byte(podArgs))
	if err != nil {
		return err
	}
	fmt.Printf("Began to run the QEMU process to start the VM!\n")
	qemuPodEvent := make(chan qemu.QemuEvent, 128)
	go qemu.qemuLoop(userPod.Name, qemuPodEvent, 1, 128)
	qemuPodEvent <- &userPod
	return nil
}
