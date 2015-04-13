package daemon

import (
	"fmt"
	"dvm/engine"
	"dvm/api/pod"
)

func (daemon *Daemon) CmdPod(job *engine.Job) error {
	podArgs := job.Args[0]
	cli := daemon.dockerCli
	userPod, err := pod.ProcessPodBytes([]byte(podArgs))
	if err != nil {
		return err
	}
	qemuPodEvent := make(chan QemuEvent, 128)
	go qemuLoop(userPod.Name, qemuPodEvent, 1, 128)
	qemuPodEvent <- &userPod
	return nil
}
