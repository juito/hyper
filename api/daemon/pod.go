package daemon

import (
	"os"
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

	vmid := fmt.Sprintf("vm-%s", pod.RandStr(10, "alpha"))
	podid := fmt.Sprintf("pod-%s", pod.RandStr(10, "alpha"))

	code, cause, err := daemon.CreatePod(podArgs, vmid, podid)
	if err != nil {
		return err
	}
	containers := []*Container{}
	for _, v := range daemon.containerList {
		if v.PodId == podid {
			containers = append(containers, v)
		}
	}
	pod := &Pod {
		Id:           podid,
		Vm:           vmid,
		Containers:   containers,
		Status:       types.S_ONLINE,
	}
	daemon.AddPod(pod)
	// Prepare the qemu status to client
	v := &engine.Env{}
	v.Set("ID", podid)
	v.SetInt("Code", code)
	v.Set("Cause", cause)
	if _, err := v.WriteTo(job.Stdout); err != nil {
		return err
	}

	return nil
}

func (daemon *Daemon) CreatePod(podArgs, vmid, podid string) (code int, cause string, err error) {
	userPod, err := pod.ProcessPodBytes([]byte(podArgs))
	if err != nil {
		glog.V(1).Infof("Process POD file error: %s", err.Error())
		return -1, "", err
	}
	// store the UserPod into the db
	if err:= daemon.WritePodToDB(podid, []byte(podArgs)); err != nil {
		glog.V(1).Info("Found an error while saveing the POD file")
		return -1, "", err
	}
	if err := daemon.WritePodAndVM(podid, vmid); err != nil {
		glog.V(1).Info("Found an error while saveing the VM info")
		return -1, "", err
	}
	glog.V(1).Info("Began to run the QEMU process to start the VM!\n")
	qemuPodEvent := make(chan qemu.QemuEvent, 128)
	qemuStatus := make(chan *types.QemuResponse, 100)

	glog.V(1).Infof("The config: kernel=%s, initrd=%s", os.Getenv("Kernel"), os.Getenv("Initrd"))
	go qemu.QemuLoop(vmid, qemuPodEvent, qemuStatus, 1, 512, os.Getenv("Kernel"), os.Getenv("Initrd"))
	if err := daemon.SetQemuChan(vmid, qemuPodEvent, qemuStatus); err != nil {
		glog.V(1).Infof("SetQemuChan error: %s", err.Error())
		return -1, "", err
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
	if qemuResponse.Code == types.E_OK {
		podData := qemuResponse.Data.(*qemu.RunningPod)
		for _, c := range podData.Containers {
			fmt.Printf("c.id = %s\n", c.Id)
			daemon.SetPodByContainer(c.Id, podid, "", "", make([]string, 0), types.S_ONLINE)
		}
	}
	// XXX we should not close qemuStatus chan, it will be closed in shutdown process

	return qemuResponse.Code, qemuResponse.Cause, nil
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
