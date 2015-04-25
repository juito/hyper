package pod

import (
    "fmt"
    "os"
    "io/ioutil"
    "crypto/rand"
    "encoding/json"
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
    Command            []string                `json:"command"`
    Workdir            string                  `json:"workdir"`
    EntryPoint         []string                `json:"entryPoint"`
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

func ProcessPodFile(jsonFile string) (*UserPod, error ) {
    if _, err := os.Stat(jsonFile); err != nil && os.IsNotExist(err) {
        return nil, err
    }
    body, err := ioutil.ReadFile(jsonFile)
    if err != nil {
        return nil, err
    }
    return ProcessPodBytes(body)
}

func ProcessPodBytes(body []byte) (*UserPod, error) {

    var userPod UserPod
    if err := json.Unmarshal(body, &userPod); err != nil {
        return nil, err
    }

    // we need to validate the given POD file
    if userPod.Name == "" {
        userPod.Name = RandStr(10, "alphanum")
    }

    if userPod.Resource.Vcpu == 0 {
        userPod.Resource.Vcpu = 1
    }
    if userPod.Resource.Memory == 0 {
        userPod.Resource.Memory = 128
    }

    var (
        v UserContainer
        vol UserVolume
        num = 0
    )
    for _, v = range userPod.Containers {
        if v.Image == "" {
            return nil, fmt.Errorf("Please specific your image for your container, it can not be null!\n")
        }
        num ++
    }
    if num == 0 {
        return nil, fmt.Errorf("Please correct your POD file, the container section can not be null!\n")
    }
    for _, vol = range userPod.Volumes {
        if vol.Name == "" {
            return nil, fmt.Errorf("DVM ERROR: please specific your volume name, it can not be null!\n")
        }
    }

    return &userPod, nil
}

func RandStr(strSize int, randType string) string {
    var dictionary string
    if randType == "alphanum" {
        dictionary = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
    }

    if randType == "alpha" {
        dictionary = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
    }

    if randType == "number" {
        dictionary = "0123456789"
    }

    var bytes = make([]byte, strSize)
    rand.Read(bytes)
    for k, v := range bytes {
        bytes[k] = dictionary[v%byte(len(dictionary))]
    }
    return string(bytes)
}
