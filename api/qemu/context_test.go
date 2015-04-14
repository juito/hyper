package qemu

import (
    "testing"
)

func TestInitContext(t *testing.T) {

    ctx := initContext("vmid", nil, 1, 128)
    if ctx.id != "vmid" {
        t.Error("id should be vmid, but is ", ctx.id)
    }
    t.Log("id check finished.")
    ctx.Close()
}

func TestRemoveSock(t *testing.T) {
    initContext("vmid", nil, 1, 128)
    initContext("vmid", nil, 1, 128)
    t.Log("repeat initiated them.")
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
        "memory": "128"
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
