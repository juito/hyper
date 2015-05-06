package daemon

import (
	"fmt"
	"strings"
	"dvm/engine"
	"dvm/lib/glog"
	"dvm/api/types"
	"github.com/syndtr/goleveldb/leveldb/util"
)

func (daemon *Daemon) CmdList(job *engine.Job) error {
	var item string
	if len(job.Args) == 0 {
		item = "pod"
	} else {
		item = job.Args[0]
	}
	if item != "pod" && item != "container" && item != "vm" {
		return fmt.Errorf("Can not support %s list!", item)
	}

	var (
		err error
		vmJsonResponse = make([]string, 100)
		podJsonResponse = make([]string, 100)
		containerJsonResponse = make([]string, 100)
		i = 0
		k = 0
		j = 0
		status string
	)

	// Prepare the qemu status to client
	v := &engine.Env{}
	v.Set("item", item)
	if item == "vm" || item == "pod" {
		iter := (daemon.db).NewIterator(util.BytesPrefix([]byte("pod-vm-")), nil)
		for iter.Next() {
			key := iter.Key()
			value := iter.Value()
			switch daemon.podList[string(key)[7:]].Status {
			case types.S_ONLINE:
				status = "online"
				break
			case types.S_STOP:
				status = "stop"
				break
			default:
				status = ""
				break
			}
			vmJsonResponse[i] = string(value)[3:]+":"+string(key)[7:]+":"+status
			i = i + 1
		}
		iter.Release()
		err = iter.Error()
		if err != nil {
			v.Set("Error", err.Error())
		}
	}

	if item == "pod" {
		iter := (daemon.db).NewIterator(util.BytesPrefix([]byte("pod-")), nil)
		for iter.Next() {
			key := iter.Key()
			value := iter.Value()
			if strings.Contains(string(key), "pod-vm-") {
				continue
			}
			podJsonResponse[j] = string(value)
			j = j + 1
			glog.V(1).Infof("Get the pod item, pod is %s!", key)
		}
		iter.Release()
		err = iter.Error()
		if err != nil {
			v.Set("Error", err.Error())
		}
	}

	if item == "container" {
		for v, c := range daemon.containerList {
			switch c.Status {
			case types.S_ONLINE:
				status = "online"
				break
			case types.S_STOP:
				status = "stop"
				break
			default:
				status = ""
			}
			containerJsonResponse[k] = v+":"+c.PodId+":"+status
			k ++
		}
	}

	v.SetList("vmData", vmJsonResponse[:i])
	v.SetList("podData", podJsonResponse[:j])
	if item == "container" {
		v.SetList("cData", containerJsonResponse[:k])
	}
	if _, err := v.WriteTo(job.Stdout); err != nil {
		return err
	}

	return nil
}
