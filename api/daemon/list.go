package daemon

import (
	"fmt"
	"dvm/engine"
	//"dvm/lib/glog"
	"github.com/syndtr/goleveldb/leveldb/util"
)

func (daemon *Daemon) CmdList(job *engine.Job) error {
	item := job.Args[0]
	if item == "" {
		item = "pod"
	}
	if item != "pod" && item != "container" && item != "vm" {
		return fmt.Errorf("Can not support %s list!", item)
	}

	var (
		err error
		vmJsonResponse []string
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
			vmJsonResponse[i] = string(key)[7:]+"-"+string(value)[3:]
			i = i + 1
		}
		iter.Release()
		err = iter.Error()
		if err != nil {
			v.Set("Error", err.Error())
		}
	}

	var podJsonResponse []string
	i = 0
	if item == "pod" {
		iter := (daemon.db).NewIterator(util.BytesPrefix([]byte("pod-")), nil)
		for iter.Next() {
			key := iter.Key()
			value := iter.Value()
			podJsonResponse[i] = string(key)[4:]+"-"+string(value)
			i = i + 1
		}
		iter.Release()
		err = iter.Error()
		if err != nil {
			v.Set("Error", err.Error())
		}
	}

	v.SetList("vmData", vmJsonResponse)
	v.SetList("podData", podJsonResponse)
	if _, err := v.WriteTo(job.Stdout); err != nil {
		return err
	}

	return nil
}
