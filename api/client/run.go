package client

import (
	"fmt"
	"encoding/json"
	gflag "github.com/jessevdk/go-flags"

	"dvm/api/pod"
)

// dvmcli run [OPTIONS] image [COMMAND] [ARGS...]
func (cli *DvmClient) DvmCmdRun(args ...string) error {
	if len(args) == 0 {
		return fmt.Errorf("DVM ERROR: Can not accept the 'run' command without argument!\n")
	}
	var opts struct {
		Name     string   `short:"n" long:"name" description:"Assign a name to the container"`
		Attach   bool     `short:"a" long:"attach" description:"Attach the stdin, stdout and stderr to the container"`
		Workdir  string   `short:"w" long:"workdir"  description:"Working directory inside the container"`
		Tty      bool     `short:"t" long:"tty" description:"Allocate a pseudo-TTY"`
		Cpu      uint     `short:"c" long:"cpu" description:"CPU shares (relative weight)"`
		Memory   uint     `short:"m" long:"memory" description:"Memory limit (format: <number><optional unit>, where unit = b, k, m or g)"`
	}

	args, _= gflag.ParseArgs(&opts, args)

	fmt.Printf("Name is %s!\n", opts.Name)
	fmt.Printf("Attach is %v!\n", opts.Attach)
	fmt.Printf("Work Dir is %s!\n", opts.Workdir)
	fmt.Printf("TTY is %v!\n", opts.Tty)
	fmt.Printf("CPU is %d!\n", opts.Cpu)
	fmt.Printf("Memory is %d!\n", opts.Memory)

	var containerList = []pod.UserContainer{}
	var container = pod.UserContainer{
		Name:           opts.Name,
		Image:          args[0],
		Command:        args[1:],
		Workdir:        opts.Workdir,
		Entrypoint:     []string{},
		Ports:          []pod.UserContainerPort{},
		Envs:           []pod.UserEnvironmentVar{},
		Volumes:        []pod.UserVolumeReference{},
		Files:          []pod.UserFileReference{},
		RestartPolicy:  "never",
	}
	containerList = append(containerList, container)

	var userPod = &pod.UserPod{
		Name:           opts.Name,
		Containers:     containerList,
		Resource:       pod.UserResource{},
		Files:          []pod.UserFile{},
		Volumes:        []pod.UserVolume{},
		Tty:            opts.Tty,
	}
/*
	if err := userPod.Validate(); err != nil {
		return err
	}
*/
	jsonString, _ := json.Marshal(userPod)
	fmt.Printf("User Pod data is %s\n", string(jsonString))
	return nil
}
