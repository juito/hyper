package daemon

import (
	"fmt"
	"strings"

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
		glog.V(1).Infof("Process POD file error: %s", err.Error())
		return err
	}
	vmid := fmt.Sprintf("vm-%s", pod.RandStr(10, "alpha"))
	podid := fmt.Sprintf("pod-%s", pod.RandStr(10, "alpha"))
	// store the UserPod into the db
	if err:= daemon.WritePodToDB(podid, []byte(podArgs)); err != nil {
		glog.V(1).Info("Found an error while saveing the POD file")
		return err
	}
	if err := daemon.WritePodAndVM(podid, vmid); err != nil {
		glog.V(1).Info("Found an error while saveing the VM info")
		return err
	}
	glog.V(1).Info("Began to run the QEMU process to start the VM!\n")
	qemuPodEvent := make(chan qemu.QemuEvent, 128)
	qemuStatus := make(chan *types.QemuResponse, 100)

	go qemu.QemuLoop(vmid, qemuPodEvent, qemuStatus, 1, 512)
	if err := daemon.SetQemuChan(vmid, qemuPodEvent, qemuStatus); err != nil {
		glog.V(1).Infof("SetQemuChan error: %s", err.Error())
		return err
	}
	runPodEvent := &qemu.RunPodCommand {
		Spec: userPod,
	}
	qemuPodEvent <- runPodEvent
	// wait for the qemu response
	var qemuResponse *types.QemuResponse
	for {
		qemuResponse =<-qemuStatus
		glog.V(1).Infof("Get the response from QEMU, VM id is %s!", qemuResponse.VmId)
		if qemuResponse.VmId == vmid {
			break
		}
	}

	// XXX we should not close qemuStatus chan, it will be closed in shutdown process

	// Prepare the qemu status to client
	v := &engine.Env{}
	v.Set("ID", podid)
	v.SetInt("Code", qemuResponse.Code)
	v.Set("Cause", qemuResponse.Cause)
	podData := qemuResponse.Data.(*qemu.RunningPod)
	for _, c := range podData.Containers {
		fmt.Printf("c.id = %s\n", c.Id)
		daemon.SetPodByContainer(c.Id, podid, "", "", make([]string, 0))
	}
	if _, err := v.WriteTo(job.Stdout); err != nil {
		return err
	}

	return nil
}

func (daemon *Daemon) CmdPodInfo(job *engine.Job) error {
	if len(job.Args) == 0 {
		return fmt.Errorf("Can not get POD info withou POD ID")
	}
	podName := job.Args[0]
	hostname := ""
	// We need to find the POD name
	data, err := daemon.GetPodByName(podName)
	glog.V(1).Infof("Process POD %s: GetPodByName()", podName)
	if err != nil && strings.Contains(err.Error(), "not found") {
		// bypass this error
	} else {
		userPod, err := pod.ProcessPodBytes(data)
		if err != nil {
			glog.V(1).Infof("Process POD file error: %s", err.Error())
			return err
		}
		hostname = userPod.Name
	}
	glog.V(1).Infof("Process POD %s: hostname is %s", podName, hostname)
	v := &engine.Env{}
	v.Set("hostname", hostname)
	if _, err := v.WriteTo(job.Stdout); err != nil {
		return err
	}

	return nil
}
