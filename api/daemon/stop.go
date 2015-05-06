package daemon

import (
	"fmt"
	"dvm/engine"
	"dvm/api/qemu"
	"dvm/api/types"
	"dvm/lib/glog"
)

func (daemon *Daemon) CmdStop(job *engine.Job) error {
	if len(job.Args) == 0 {
		return fmt.Errorf("Can not execute 'stop' command without any pod name!")
	}
	podId := job.Args[0]

	glog.V(1).Infof("Prepare to stop the POD: %s", podId)
	// We need find the vm id which running POD, and stop it
	vmid, err := daemon.GetPodVmByName(podId)
	if err != nil {
		return err
	}
	if daemon.podList[podId].Status != types.S_ONLINE {
		return fmt.Errorf("The POD %s has aleady stopped, can not stop again!", podId)
	}
	qemuPodEvent, qemuStatus, err := daemon.GetQemuChan(string(vmid))
	if err != nil {
		return err
	}

	shutdownPodEvent := &qemu.ShutdownCommand { }
	qemuPodEvent.(chan qemu.QemuEvent) <-shutdownPodEvent
	// wait for the qemu response
	var qemuResponse *types.QemuResponse
	for {
		qemuResponse =<-qemuStatus.(chan *types.QemuResponse)
		glog.V(1).Infof("Got response: %d: %s", qemuResponse.Code, qemuResponse.Cause)
		if qemuResponse.Code == types.E_SHUTDOWM {
			break
		}
	}
	close(qemuStatus.(chan *types.QemuResponse))

	// Prepare the qemu status to client
	v := &engine.Env{}
	v.Set("ID", podId)
	v.SetInt("Code", qemuResponse.Code)
	v.Set("Cause", qemuResponse.Cause)
	if _, err := v.WriteTo(job.Stdout); err != nil {
		return err
	}

	defer func() {
		daemon.podList[podId].Status = types.S_STOP
		daemon.SetContainerStatus(podId, types.S_STOP)
		daemon.DeleteQemuChan(string(vmid))
	} ()

	return nil
}
