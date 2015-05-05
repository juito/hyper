package qemu

import (
    "testing"
    "encoding/json"
    "dvm/api/pod"
    "os"
)

func TestInitContext(t *testing.T) {

    ctx,_ := initContext("vmid", nil, nil, 3, 202, Kernel, Initrd)

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

func TestFailInitContext(t *testing.T) {
    os.Remove("/var/run/dvm/vmid/dvm.sock")
    err := os.MkdirAll("/var/run/dvm/vmid/dvm.sock/something/whatever", 777)
    _,err = initContext("vmid", nil, nil, 3, 202, Kernel, Initrd)
    if err == nil {
        t.Error("should not complete")
    } else {
        t.Log("should get an error", err.Error())
    }
}

func TestRemoveSock(t *testing.T) {
    initContext("vmid", nil, nil, 1, 128, Kernel, Initrd)
    initContext("vmid", nil, nil, 1, 128, Kernel, Initrd)
    t.Log("repeat initiated them.")
}

func TestParseSpec(t *testing.T) {
    ctx,_ := initContext("vmmid", nil, nil, 1, 128, Kernel, Initrd)

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

    if ctx.vmSpec.Containers[0].Envs[1].Env != "JAVA_HOME" {
        t.Error("second environment should not be ", ctx.vmSpec.Containers[0].Envs[1].Env)
    }

    res,err := json.MarshalIndent(*ctx.vmSpec, "    ", "    ")
    if err != nil {
        t.Error("vmspec to json failed")
    }
    t.Log(string(res))
}

func TestParseVolumes(t *testing.T) {
    ctx,_ := initContext("vmmid", nil, nil, 1, 128, Kernel, Initrd)

    spec := pod.UserPod{}
    err := json.Unmarshal([]byte(testJson("with_volumes")), &spec)
    if err != nil {
        t.Error("parse json failed ", err.Error())
    }

    ctx.InitDeviceContext(&spec, 0)

    res,err := json.MarshalIndent(*ctx.vmSpec, "    ", "    ")
    if err != nil {
        t.Error("vmspec to json failed")
    }
    t.Log(string(res))

    vol1 := ctx.devices.volumeMap["vol1"]
    if vol1.pos[0] != "/var/dir1" {
        t.Error("vol1 (/var/dir1) path is ", vol1.pos[0])
    }

    if !vol1.readOnly[0] {
        t.Error("vol1 on container 0 should be read only")
    }

    ref1 := blockDescriptor{ name:"vol1", filename:"", format:"", fstype:"", deviceName:"" }
    if *vol1.info != ref1 {
        t.Errorf("info of vol1: %q %q %q %q %q",
            vol1.info.name, vol1.info.filename, vol1.info.format, vol1.info.fstype,vol1.info.deviceName)
    }

    vol2 := ctx.devices.volumeMap["vol2"]
    if vol2.pos[0] != "/var/dir2" {
        t.Error("vol1 (/var/dir2) path is ", vol2.pos[0])
    }

    if vol2.readOnly[0] {
        t.Error("vol2 on container 0 should not be read only")
    }

    ref2 := blockDescriptor{ name:"vol2", filename:"/home/whatever", format:"vfs", fstype:"dir", deviceName:""}
    if *vol2.info != ref2 {
        t.Errorf("info of vol2: %q %q %q %q %q",
        vol2.info.name, vol2.info.filename, vol2.info.format, vol2.info.fstype,vol2.info.deviceName)
    }
}

func TestVolumeReady(t *testing.T) {
    ctx,_ := initContext("vmmid", nil, nil, 1, 128, Kernel, Initrd)

    spec := pod.UserPod{}
    err := json.Unmarshal([]byte(testJson("with_volumes")), &spec)
    if err != nil {
        t.Error("parse json failed ", err.Error())
    }

    ctx.InitDeviceContext(&spec, 0)

    ready := &VolumeReadyEvent{
        Name: "vol2",
        Filepath: "/a1b2c3d4/whatever",
        Fstype: "dir",
        Format: "",
    }
    ctx.volumeReady(ready)

    ready = &VolumeReadyEvent{
        Name: "vol1",
        Filepath: "/dev/dm17",
        Fstype: "xfs",
        Format: "raw",
    }
    ctx.volumeReady(ready)

    bevent := &BlockdevInsertedEvent{
        Name: "vol1",
        SourceType: "volume",
        DeviceName: "sda",
    }
    ctx.blockdevInserted(bevent)

    res,err := json.MarshalIndent(*ctx.vmSpec, "    ", "    ")
    if err != nil {
        t.Error("vmspec to json failed")
    }
    t.Log(string(res))
}

func dumpProgress(t *testing.T, pm *processingMap) {
    t.Log("containers:")
    for id,ready := range pm.containers {
        t.Logf("\t%d\t%d", id, ready)
    }
}

func TestContainerCreated(t *testing.T) {
    ctx,_ := initContext("vmmid", nil, nil, 1, 128, Kernel, Initrd)

    spec := pod.UserPod{}
    err := json.Unmarshal([]byte(testJson("basic")), &spec)
    if err != nil {
        t.Error("parse json failed ", err.Error())
    }

    ctx.InitDeviceContext(&spec, 0)

    dumpProgress(t, ctx.progress.adding)

    if ctx.deviceReady() {
        t.Error("should not ready when containers are not ready")
    }

    ready := &ContainerCreatedEvent{
        Index:0,
        Id:"a1b2c3d4",
        Rootfs:"/rootfs",
        Image:"/dev/dm7",
        Fstype:"ext4",
        Workdir:"/",
        Cmd: []string{"run.sh","gogogo"},
        Envs: map[string]string{
            "JAVA_HOME":"/user/share/java",
            "GOPATH":"/",
        },
    }

    ctx.containerCreated(ready)

    if ctx.deviceReady() {
        t.Error("should not ready when volume are not inserted")
    }

    bevent := &BlockdevInsertedEvent{
        Name: "/dev/dm7",
        SourceType: "image",
        DeviceName: "sda",
    }
    ctx.blockdevInserted(bevent)

    if !ctx.deviceReady() {
        t.Error("after image inserted, it should ready now")
    }

    res,err := json.MarshalIndent(*ctx.vmSpec, "    ", "    ")
    if err != nil {
        t.Error("vmspec to json failed")
    }
    t.Log(string(res))
}

func TestNetworkCreated(t *testing.T) {
    ctx,_ := initContext("vmmid", nil, nil, 1, 128, Kernel, Initrd)

    spec := pod.UserPod{}
    err := json.Unmarshal([]byte(testJson("basic")), &spec)
    if err != nil {
        t.Error("parse json failed ", err.Error())
    }

    ctx.InitDeviceContext(&spec, 1)

    dumpProgress(t, ctx.progress.adding)

    ready := &ContainerCreatedEvent{
        Index:0,
        Id:"a1b2c3d4",
        Rootfs:"/rootfs",
        Image:"/dev/dm7",
        Fstype:"ext4",
        Workdir:"/",
        Cmd: []string{"run.sh","gogogo"},
        Envs: map[string]string{
            "JAVA_HOME":"/user/share/java",
            "GOPATH":"/",
        },
    }

    ctx.containerCreated(ready)

    bevent := &BlockdevInsertedEvent{
        Name: "/dev/dm7",
        SourceType: "image",
        DeviceName: "sda",
    }
    ctx.blockdevInserted(bevent)

    if ctx.deviceReady() {
        t.Error("should not ready when network are not inserted")
    }

    ievent := &InterfaceCreated{
        Index: 0,
        PCIAddr: 4,
        DeviceName: "eth0",
        IpAddr: "192.168.12.34",
        NetMask: "255.255.255.0",
        RouteTable: []*RouteRule{
            &RouteRule{
                Destination:"192.168.12.0/24",
                Gateway: "",
                ViaThis: true,
            },
            &RouteRule{
                Destination:"0.0.0.0/0",
                Gateway: "192.168.12.1",
                ViaThis: false,
            },
        },
    }
    ctx.interfaceCreated(ievent)

    if ctx.deviceReady() {
        t.Error("should not ready when network are not inserted")
    }

    fevent := &NetDevInsertedEvent{
        Index: 0,
        DeviceName: "eth0",
    }
    ctx.netdevInserted(fevent)

    if !ctx.deviceReady() {
        t.Error("after nic inserted, it should ready now")
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
        }],
        "envs":  [{
            "env": "JAVA_OPT",
            "value": "-XMx=256m"
        },{
            "env": "JAVA_HOME",
            "value": "/usr/local/java"
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

    jsons["with_volumes"] = `{
    "name": "hostname",
    "containers" : [{
        "image": "nginx:latest",
        "files":  [{
            "path": "/var/lib/xxx/xxxx",
            "filename": "filename"
        }],
        "volumes": [{
            "path": "/var/dir1",
            "volume": "vol1",
            "readOnly": true
        },{
            "path": "/var/dir2",
            "volume": "vol2",
            "readOnly": false
        },{
            "path": "/var/dir3",
            "volume": "vol3",
            "readOnly": false
        },{
            "path": "/var/dir4",
            "volume": "vol4",
            "readOnly": false
        },{
            "path": "/var/dir5",
            "volume": "vol5",
            "readOnly": false
        },{
            "path": "/var/dir6",
            "volume": "vol6",
            "readOnly": false
        }]
    }],
    "resource": {
        "vcpu": 1,
        "memory": 128
    },
    "files": [],
    "volumes": [{
        "name": "vol1",
        "source": "",
        "driver": ""
    },{
        "name": "vol2",
        "source": "/home/whatever",
        "driver": "vfs"
    },{
        "name": "vol3",
        "source": "/home/what/file",
        "driver": "raw"
    },{
        "name": "vol4",
        "source": "",
        "driver": ""
    },{
        "name": "vol5",
        "source": "/home/what/file2",
        "driver": "vfs"
    },{
        "name": "vol6",
        "source": "/home/what/file3",
        "driver": "qcow2"
    }]
    }`

    return jsons[key]
}
