package daemon

import (
	"fmt"
	"io"
	"bufio"
	"dvm/engine"
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
	fmt.Printf("%s\n", vmid)
	var (
		stop = make(chan bool, 1)
		input = make(chan string, 1)
		cStdout io.Writer
		cStdin  io.ReadCloser
	)
	r, w := io.Pipe()
	go func() {
		defer w.Close()
		defer glog.V(1).Info("Close the io Pipe!")
		io.Copy(w, job.Stdin)
	} ()
	cStdin = r
	cStdout = job.Stdout
	go func() {
		for i:=0; i< 10; i ++ {
			glog.V(1).Infof("i: %d!", i)
			fmt.Fprintf(cStdout, "i : %d\n", i)
		}
		glog.V(1).Info("Send stop signal to main process!")
		stop <- true
	} ()


	go func () {
		for {
			reader := bufio.NewReader(cStdin)
			data, _, _ := reader.ReadLine()
			command := string(data)
			glog.V(1).Infof("command from client : %s!", command)
			input <- command
		}
	} ()

	for {
		select {
		case <-stop:
				glog.Info("The output program is stopped!")
		case command := <-input:
				glog.Infof("find a command, %s", command)
				if command == "hello" {
					return nil
				}
		}
	}
	return nil
}
