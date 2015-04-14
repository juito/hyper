package qemu

import (
    "testing"
    "encoding/json"
    "dvm/api/pod"
)

func TestInitContext(t *testing.T) {

    ctx := initContext("vmid", nil, 3, 202)

    if ctx.id != "vmid" {
        t.Error("id should be vmid, but is ", ctx.id)
    }
    if ctx.cpu != 3 {
        t.Error("cpu should be 3, but is ", string(ctx.cpu))
    }
    if ctx.memory != 202 {
        t.Error("memory should be 202, but is ", string(ctx.memory))
    }

    t.Log("id check finished.")
    ctx.Close()
}

func TestRemoveSock(t *testing.T) {
    initContext("vmid", nil, 1, 128)
    initContext("vmid", nil, 1, 128)
    t.Log("repeat initiated them.")
}

func TestParseSpec(t *testing.T) {
    ctx := initContext("vmmid", nil, 1, 128)

    spec := pod.UserPod{}
    err := json.Unmarshal([]byte(testJson("basic")), &spec)
    if err != nil {
        t.Error("parse json failed ", err.Error())
    }

    ctx.InitDeviceContext(&spec, 0)

    if ctx.userSpec != &spec {
        t.Error("user pod assignment fail")
    }

    if len(ctx.vmSpec.Containers) != 1 {
        t.Error("wrong containers in vm spec")
    }

    if ctx.vmSpec.ShareDir != "share_dir" {
        t.Error("shareDir in vmSpec is ", ctx.vmSpec.ShareDir)
    }

    if ctx.vmSpec.Containers[0].RestartPolicy != "never" {
        t.Error("Default restartPolicy is ", ctx.vmSpec.Containers[0].RestartPolicy)
    }

    res,err := json.MarshalIndent(*ctx.vmSpec, "    ", "    ")
    if err != nil {
        t.Error("vmspec to json failed")
    }
    t.Log(string(res))
}

func testJson(key string) string {
    jsons := make(map[string]string)

    jsons["basic"] =`{
    "name": "hostname",
    "containers" : [{
        "image": "nginx:latest",
        "files":  [{
            "path": "/var/lib/xxx/xxxx",
            "filename": "filename"
        }]
    }],
    "resource": {
        "vcpu": 1,
        "memory": 128
    },
    "files": [{
        "name": "filename",
        "encoding": "raw",
        "uri": "https://s3.amazonaws/bucket/file.conf",
        "content": ""
    }],
    "volumes": []}`

    return jsons[key]
}
