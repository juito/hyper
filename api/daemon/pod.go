package daemon

import (
	"fmt"
	"syscall"
	"os/exec"
	"path"
	"path/filepath"
	"os"
	"io/ioutil"
	"encoding/json"
	"strings"
	"strconv"
	"dvm/engine"
	"dvm/api/pod"
)

type jsonMetadata struct {
	Device_id int      `json:"device_id"`
	Size      int      `json:"size"`
	Transaction_id int `json:"transaction_id"`
	Initialized bool   `json:"initialized"`
}

func (daemon *Daemon) CmdPod(job *engine.Job) error {
	podArgs := job.Args[0]
	cli := daemon.dockerCli
	userPod, err := pod.ProcessPodBytes([]byte(podArgs))
	if err != nil {
		return err
	}
	qemuPodEvent := make(chan RunPodCommand)
	qemuPodEvent <- &userPod
	return nil
}
