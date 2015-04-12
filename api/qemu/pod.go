package qemu

import (
    "encoding/json"
    "dvm/api/pod"
)

const (
)

//change first letter to uppercase and add json tag (thanks GNU sed):
//  gsed -ie 's/^    \([a-z]\)\([a-zA-Z]*\)\( \{1,\}[^ ]\{1,\}.*\)$/    \U\1\E\2\3 `json:"\1\2"`/' pod.go


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

type PreparingItem interface {
    ItemType() string
}

type ContainerInfo struct {
    Id      string
    Fstype  string
    Images  []string
    Workdir string
    Cmd     string
}

func (pod *VmPod) Serialize() (*VmMessage,error) {
    jv,err := json.Marshal(pod)
    if err != nil {
        return nil, err
    }
    buf := newVmMessage(jv)
    return buf,nil
}

//validate
// 1. volume name, file name is unique
// 2. source mount to only one pos in one container
// 3. container should not use volume not in volume list
func ValidateUserPod(spec *pod.UserPod) error {
    return nil
}
