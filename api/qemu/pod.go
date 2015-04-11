package qemu

import (
    "encoding/json"
)

//change first letter to uppercase and add json tag (thanks GNU sed):
//  gsed -ie 's/^    \([a-z]\)\([a-zA-Z]*\)\( \{1,\}[^ ]\{1,\}.*\)$/    \U\1\E\2\3 `json:"\1\2"`/' pod.go

type UserContainerPort struct {
    HostPort      int `json:"hostPort"`
    ContainerPort int `json:"containerPort"`
    ServicePort   int `json:"servicePort"`
}

type UserEnvironmentVar struct {
    Env string `json:"env"`
    Value string `json:"value"`
}

type UserVolumeReference struct {
    Path    string `json:"path"`
    Volume  string `json:"volume"`
    ReadOnly  bool `json:"readOnly"`
}

type UserFileReference struct {
    Path    string `json:"path"`
    Filename string `json:"filename"`
    Perm    string `json:"perm"`
    User    string `json:"user"`
    Group   string `json:"group"`
}

type UserContainer struct {
    Name    string `json:"name"`
    Image   string `json:"image"`
    Command string `json:"command"`
    Ports   []UserContainerPort `json:"ports"`
    Envs    []UserEnvironmentVar `json:"envs"`
    Volumes []UserVolumeReference `json:"volumes"`
    Files   []UserFileReference `json:"files"`
    RestartPolicy string `json:"restartPolicy"`
}

type UserResource struct {
    Vcpu    int `json:"vcpu"`
    Memory  int `json:"memory"`
}

type UserFile struct {
    Name     string `json:"name"`
    Encoding string `json:"encoding"`
    Uri      string `json:"uri"`
    Contents string `json:"contents"`
}

type UserVolume struct {
    Name    string `json:"name"`
    Source  string `json:"source"`
    Driver  string `json:"driver"`
}

type UserPod struct {
    Name string `json:"name"`
    Containers  []UserContainer `json:"containers"`
    Resource    UserResource `json:"resource"`
    Files       []UserFile `json:"files"`
    Volumes     []UserVolume `json:"volumes"`
}

// Vm DataStructure
type VmVolumeDescriptor struct {
    Device   string `json:"device"`
    Mount    string `json:"mount"`
    Fstype   string `json:"fstype"`
    ReadOnly bool `json:"readOnly"`
}

type VmFsmapDescriptor struct {
    Source string `json:"source"`
    Path   string `json:"path"`
    ReadOnly bool `json:"readOnly"`
}

type VmEnvironmentVar struct {
    Env   string `json:"env"`
    Value string `json:"value"`
}

type VmContainer struct {
    Id      string `json:"id"`
    Rootfs  string `json:"rootfs"`
    Fstype  string `json:"fstype"`
    Images  []string `json:"images"`
    Volumes []VmVolumeDescriptor `json:"volumes"`
    Fsmap   []VmFsmapDescriptor `json:"fsmap"`
    Tty     string `json:"tty"`
    Workdir string `json:"workdir"`
    Cmd     string `json:"cmd"`
    Envs    []VmEnvironmentVar `json:"envs"`
    RestartPolicy   string `json:"restartPolicy"`
}

type VmNetworkInf struct {
    Device      string `json:"device"`
    IpAddress   string `json:"ipAddress"`
    NetMask     string `json:"netMask"`
}

type VmRoute struct {
    Dest        string `json:"dest"`
    Gateway     string `json:"gateway"`
    Device      string `json:"device"`
}

type VmPod struct {
    Hostname    string `json:"hostname"`
    Containers  []VmContainer `json:"containers"`
//    Devices     []string `json:"devices"`
    Interfaces  []VmNetworkInf `json:"interfaces"`
    Routes      []VmRoute `json:"routes"`
    Socket      string `json:"socket"`
    ShareDir    string `json:"shareDir"`
}

func (pod *VmPod) Serialize() (*VmMessage,error) {
    jv,err := json.Marshal(pod)
    if err != nil {
        return nil, err
    }
    buf := newVmMessage(jv)
    return buf,nil
}

func (spec *UserPod) MapToVmSpec(ctx *QemuContext) *VmPod {
    containers := make([]VmContainer, len(spec.Containers))
    voltype:= make(map[string]bool)
    for _,vol := range spec.Volumes {
        if vol.Source == nil && vol.Source == "" {
            voltype[vol.Name] = true
        } else if vol.Driver == "raw" || vol.Driver == "qcow2" {
            voltype[vol.Name] = true
        } else {
            voltype[vol.Name] = false
        }
    }
    for i,container := range spec.Containers {

        //volumes
        vols := make([]VmVolumeDescriptor, len(container.Volumes))
        fsmap := make([]VmFsmapDescriptor, len(container.Volumes))
//
//        for j,v := range container.Volumes {
//            vols[j] = VmVolumeDescriptor{
//                Device: "",
//                Mount:  v.Path,
//            }
//        }
//
//        //fsmap

        //Env
        envs := make([]VmEnvironmentVar, len(container.Envs))
        for j,e := range container.Envs {
            envs[j] = VmEnvironmentVar{
                Env:    e.Env,
                Value:  e.Value,
            }
        }

        containers[i] = VmContainer{
            Id:     nil,
            Rootfs: "rootfs",
            Fstype: "ext4",
            Images:  nil,
            Volumes: vols,
            Fsmap:   fsmap,
            Tty:     nil,
            Workdir: nil,
            Cmd:     nil,
            Envs:    envs,
            RestartPolicy: container.RestartPolicy,
        }
    }
    return &VmPod{
        Hostname:       spec.Name,
        Containers:     containers,
        Interfaces:     nil,
        Routes:         nil,
        Socket:         ctx.dvmSockName,
        ShareDir:       ctx.shareDir,
    }
}