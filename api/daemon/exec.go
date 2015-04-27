package daemon

import (
	"fmt"
	"io"
	"bufio"
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
		input = make(chan string, 1)
		cStdout io.Writer
		cStdin  io.ReadCloser
		ttyIO *qemu.TtyIO
		ttyIOChan = make(chan *qemu.TtyIO, 1)
	)
	defer close(stop)
	defer close(input)
	defer close(ttyIOChan)
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
	cStdout = job.Stdout
	go func() {
		for {
			select {
			case output := <-ttyIO.Output:
				glog.V(1).Infof("%s", output)
				fmt.Fprintf(cStdout, "%s\n", output)
			case <-stop:
				return
			}
		}
	} ()


	go func () {
		for {
			reader := bufio.NewReader(cStdin)
			data, _, _ := reader.ReadLine()
			command := string(data)
			if command == "" {
				continue
			}
			glog.V(1).Infof("command from client : %s!", command)
			input <- command
			if command == "exit" {
				break
			}
		}
	} ()

	for {
		select {
		case <-stop:
			glog.V(1).Info("The output program is stopped!")
		case command := <-input:
			glog.Infof("find a command, %s", command)
			if command != "" {
				if command == "exit" {
					stop <- true
					return nil
				} else {
					ttyIO.Input <- command +"\015\012"
				}
			}
		}
	}
	return nil
}
