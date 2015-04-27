package main

import (
    "dvm/api/qemu"
    "dvm/api/types"
    "encoding/json"
    "dvm/api/pod"
)

func main() {
    qc := make(chan qemu.QemuEvent, 256)
    cb := make(chan *types.QemuResponse, 256)
    go qemu.QemuLoop("test", qc, cb, 1, 256)

    var pod pod.UserPod
    err := json.Unmarshal([]byte(`{
	"id": "test-container-create-1",
	"containers" : [{
	    "name": "web",
	    "image": "tomcat:latest",
	    "envs": [{"env":"PS1","value":"||||||| DUANG |||||||"}],
	"command":["/bin/bash"]
	}],
	"resource": {
	    "vcpu": 1,
	    "memory": 512
	},
	"files": [],
	"volumes": []
}`), &pod)
    if err != nil {
        println("error decode json")
    }

    qc <- &qemu.RunPodCommand{
        Spec: &pod,
    }

    for {
        r,ok := <-cb
        if !ok || r.Code != types.E_OK {
            return
        } else {
            println(r.Cause)
        }
    }
}
