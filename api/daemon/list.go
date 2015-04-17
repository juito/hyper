package daemon

import (
	"fmt"
	"strings"
	"dvm/engine"
	"dvm/lib/glog"
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
		i int
	)
	// Prepare the qemu status to client
	v := &engine.Env{}
	v.Set("item", item)
	if item == "vm" || item == "pod" {
		iter := (daemon.db).NewIterator(util.BytesPrefix([]byte("pod-vm-")), nil)
		i = 0
		for iter.Next() {
			key := iter.Key()
			value := iter.Value()
			vmJsonResponse[i] = string(value)[3:]+"-"+string(key)[7:]
			i = i + 1
		}
		iter.Release()
		err = iter.Error()
		if err != nil {
			v.Set("Error", err.Error())
		}
	}

	var j = 0
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

	v.SetList("vmData", vmJsonResponse[:i])
	v.SetList("podData", podJsonResponse[:j])
	if _, err := v.WriteTo(job.Stdout); err != nil {
		return err
	}

	return nil
}
