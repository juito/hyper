package pod

import (
	"encoding/json"
	"fmt"
    "io/ioutil"
    "os"
)

// Pod Data Structure
type UserContainerPort struct {
    HostPort            int           `json:"hostPort"`
    ContainerPort       int           `json:"containerPort"`
    ServicePort         int           `json:"servicePort"`
}

type UserEnvironmentVar struct {
    Env                 string         `json:"env"`
    Value               string         `json:"value"`
}

type UserVolumeReference struct {
    Path                string         `json:"path"`
    Volume              string         `json:"volume"`
    ReadOnly            bool           `json:"readOnly"`
}

type UserFileReference struct {
    Path               string          `json:"path"`
    Filename           string          `json:"filename"`
    Perm               string          `json:"perm"`
    User               string          `json:"user"`
    Group              string          `json:"group"`
}

type UserContainer struct {
    Name               string                  `json:"name"`
    Image              string                  `json:"image"`
    Command            string                  `json:"command"`
    EntryPoint         string                  `json:"entryPoint"`
    Ports              []UserContainerPort     `json:"ports"`
    Envs               []UserEnvironmentVar    `json:"envs"`
    Volumes            []UserVolumeReference   `json:"volumes"`
    Files              []UserFileReference     `json:"files"`
    RestartPolicy      string                  `json:"restartPolicy"`
}

type UserResource struct {
    Vcpu              int              `json:"vcpu"`
    Memory            int              `json:"memory"`
}

type UserFile struct {
    Name              string           `json:"name"`
    Encoding          string           `json:"encoding"`
    Uri               string           `json:"uri"`
    Contents          string           `json:"content"`
}

type UserVolume struct {
    Name             string             `json:"name"`
    Source           string             `json:"source"`
    Driver           string             `json:"driver"`
}

type UserPod struct {
    Name            string               `json:"id"`
    Containers      []UserContainer      `json:"containers"`
    Resource        UserResource         `json:"resource"`
    Files           []UserFile           `json:"files"`
    Volumes         []UserVolume         `json:"volumes"`
}

func ProcessPodFile(jsonFile string) (*UserPod, error) {
    if _, err := os.Stat(jsonFile); err != nil && os.IsNotExist(err) {
        return nil, err
    }
    body, err := ioutil.ReadFile(jsonFile)
    if err != nil {
        return nil, err
    }
    var userPod UserPod
    if err := json.Unmarshal(body, &userPod); err != nil {
        return &userPod, nil

    } else {
        return nil, err
    }
}
