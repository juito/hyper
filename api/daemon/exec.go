package daemon

import (
	"fmt"
	"strings"
	"dvm/engine"
	"dvm/lib/glog"
	"dvm/api/qemu"
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
		stop = make(chan bool, 1)
		ttyIO qemu.TtyIO
	)

	ttyIO.Stdin = job.Stdin
	ttyIO.Stdout = job.Stdout

	var attachCommand = &qemu.AttachCommand {
		Streams: &ttyIO,
		Size:    nil,
	}
	if typeKey == "pod" {
		attachCommand.Container = ""
	} else {
		attachCommand.Container = typeVal
	}
	qemuEvent, _, err := daemon.GetQemuChan(string(vmid))
	if err != nil {
		return err
	}
	qemuEvent.(chan qemu.QemuEvent) <-attachCommand

	execCommand := &qemu.ExecCommand {
		Command: strings.Split(command, " "),
		Container: "",
	}
	qemuEvent.(chan qemu.QemuEvent) <-execCommand

	defer func() {
		close(stop)
		glog.V(2).Info("Defer function for exec!")
	} ()
	return nil
}
