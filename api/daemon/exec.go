package daemon

import (
	"fmt"
	"io"
	"strings"
	"dvm/engine"
	"dvm/lib/glog"
	"dvm/api/qemu"
	"dvm/lib/utils"
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
		cStdout io.WriteCloser
		cStdin  io.ReadCloser
		ttyIO *qemu.TtyIO
		ttyIOChan = make(chan *qemu.TtyIO, 1)
	)

	var attachCommand = &qemu.AttachCommand {
		Callback: ttyIOChan,
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
	ttyIO = <-ttyIOChan

	execCommand := &qemu.ExecCommand {
		Command: strings.Split(command, " "),
		Container: "",
	}
	qemuEvent.(chan qemu.QemuEvent) <-execCommand

	r, w := io.Pipe()
	go func() {
		defer w.Close()
		defer glog.V(1).Info("Close the io Pipe!")
		io.Copy(w, job.Stdin)
	} ()
	cStdin = r
	go func() {
		io.Copy(ttyIO.Input, cStdin)
		defer glog.V(1).Info("Close the Stdin!")
	} ()
	cStdout = job.Stdout
	go func() {
		utils.CopyEscapable(cStdout, ttyIO.Output)
		defer glog.V(1).Info("Close the Stdout!")
	}()

	defer func() {
		close(stop)
		close(ttyIOChan)

		if cStdout != nil {
			cStdout.Close()
		}

		var detachCommand = &qemu.DetachCommand {
			Tty: ttyIO,
		}
		if typeKey == "pod" {
			detachCommand.Container = ""
		} else {
			detachCommand.Container = typeVal
		}
		qemuEvent.(chan qemu.QemuEvent) <-detachCommand
	} ()
	return nil
}
