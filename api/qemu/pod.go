package qemu

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
    Image   string `json:"image"`
    Volumes []VmVolumeDescriptor `json:"volumes,omitempty"`
    Fsmap   []VmFsmapDescriptor `json:"fsmap,omitempty"`
    Tty     uint64 `json:"tty,omitempty"`
    Workdir string `json:"workdir"`
    Entrypoint []string `json:"-"`
    Cmd     []string `json:"cmd"`
    Envs    []VmEnvironmentVar `json:"envs,omitempty"`
    RestartPolicy   string `json:"restartPolicy"`
}

type VmNetworkInf struct {
    Device      string `json:"device"`
    IpAddress   string `json:"ipAddress"`
    NetMask     string `json:"netMask"`
}

type VmRoute struct {
    Dest        string `json:"dest"`
    Gateway     string `json:"gateway,omitempty"`
    Device      string `json:"device,omitempty"`
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

type RunningContainer struct {
    Id          string `json:"id"`
}

type RunningPod struct {
    Hostname    string `json:"hostname"`
    Containers  []RunningContainer
}

type PreparingItem interface {
    ItemType() string
}

func (p *VmPod) runningInfo() *RunningPod {
    containers := make([]RunningContainer, len(p.Containers))
    for idx, c := range p.Containers {
        containers[idx].Id = c.Id
    }
    return &RunningPod{
        Hostname:   p.Hostname,
        Containers: containers,
    }
}
