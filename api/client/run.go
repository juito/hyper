package client

import (
	"io"
	"fmt"
	"strings"
	"net/url"
	"encoding/json"
	gflag "github.com/jessevdk/go-flags"

	"dvm/engine"
	"dvm/api/pod"
	"dvm/lib/promise"
)

// dvmcli run [OPTIONS] image [COMMAND] [ARGS...]
func (cli *DvmClient) DvmCmdRun(args ...string) error {
	if len(args) == 0 {
		return fmt.Errorf("DVM ERROR: Can not accept the 'run' command without argument!\n")
	}
	var opts struct {
		Name     string   `short:"n" long:"name" value-name:"\"\"" description:"Assign a name to the container"`
		Attach   bool     `short:"a" long:"attach" default:"true" default-mask:"-" description:"Attach the stdin, stdout and stderr to the container"`
		Workdir  string   `short:"w" long:"workdir"  value-name:"\"\"" default-mask:"-" description:"Working directory inside the container"`
		Tty      bool     `short:"t" long:"tty" default:"true" default-mask:"-" description:"Allocate a pseudo-TTY"`
		Cpu      uint     `short:"c" long:"cpu" default:"1" value-name:"0" default-mask:"-" description:"CPU shares (relative weight)"`
		Memory   uint     `short:"m" long:"memory" default:"128" value-name:"0" default-mask:"-" description:"Memory limit (format: <number><optional unit>, where unit = b, k, m or g)"`
		Env      []string `short:"e" long:"env" value-name:"[]" default-mask:"-" description:"Set environment variables"`
		EntryPoint      string   `long:"entrypoint" value-name:"\"\"" default-mask:"-" description:"Overwrite the default ENTRYPOINT of the image"`
		RestartPolicy   string   `short:"r" long:"restart" default:"never" value-name:"\"\"" default-mask:"-" description:"Restart policy to apply when a container exits (no, on-failure[:max-retry], always)"`
	}

	args, err := gflag.ParseArgs(&opts, args)
	if err != nil {
		return nil
	}
	if len(args) == 0 {
		return fmt.Errorf("DVM: \"run\" requires a minimum of 1 argument, please provide the image.")
	}
	var (
		image = args[0]
		command = []string{}
		env = []pod.UserEnvironmentVar{}
	)
	if len(args) > 1 {
		command = args[1:]
	}
	for _, v := range opts.Env {
		userEnv := pod.UserEnvironmentVar {
			Env:    v[:strings.Index(v, "=")],
			Value:  v[strings.Index(v, "=") + 1:],
		}
		env = append(env, userEnv)
	}

	var containerList = []pod.UserContainer{}
	var container = pod.UserContainer{
		Name:           opts.Name,
		Image:          image,
		Command:        command,
		Workdir:        opts.Workdir,
		Entrypoint:     []string{},
		Ports:          []pod.UserContainerPort{},
		Envs:           env,
		Volumes:        []pod.UserVolumeReference{},
		Files:          []pod.UserFileReference{},
		RestartPolicy:  opts.RestartPolicy,
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
	// we need to create a POD
	podId, err := cli.CreatePod(string(jsonString))
	if err != nil {
		return err
	}
	// Get the container ID of this POD
	containerId, err := cli.GetContainerByPod(podId)
	if err != nil {
		return err
	}
	var (
		cmd = strings.Join(command, "")
		tag = cli.GetTag()
		hijacked    = make(chan io.Closer)
		errCh       chan error
	)
	v := url.Values{}
	v.Set("type", "container")
	v.Set("value", containerId)
	v.Set("command", cmd)
	v.Set("tag", tag)

	// Block the return until the chan gets closed
	defer func() {
		fmt.Printf("End of CmdExec(), Waiting for hijack to finish.\n")
		if _, ok := <-hijacked; ok {
			fmt.Printf("Hijack did not finish (chan still open)\n")
		}
	}()

	errCh = promise.Go(func() error {
		return cli.hijack("POST", "/exec?"+v.Encode(), true, cli.in, cli.out, cli.out, hijacked, nil, "")
	})

	if err := cli.monitorTtySize(podId, tag); err != nil {
		fmt.Printf("Monitor tty size fail for %s!\n", podId)
	}

	// Acknowledge the hijack before starting
	select {
	case closer := <-hijacked:
		// Make sure that hijack gets closed when returning. (result
		// in closing hijack chan and freeing server's goroutines.
		if closer != nil {
			defer closer.Close()
		}
	case err := <-errCh:
		if err != nil {
			fmt.Printf("Error hijack: %s", err.Error())
			return err
		}
	}

	if err := <-errCh; err != nil {
		fmt.Printf("Error hijack: %s", err.Error())
		return err
	}
	fmt.Printf("Success to exec the command %s for POD %s!\n", command, podId)
	return nil
}

func (cli *DvmClient)GetContainerByPod(podId string) (string ,error) {
	v := url.Values{}
	v.Set("item", "container")
	body, _, err := readBody(cli.call("GET", "/list?"+v.Encode(), nil, nil));
	if err != nil {
		return "", err
	}
	out := engine.NewOutput()
	remoteInfo, err := out.AddEnv()
	if err != nil {
		return "", err
	}

	if _, err := out.Write(body); err != nil {
		fmt.Printf("Error reading remote info: %s", err)
		return "", err
	}
	out.Close()
	var containerResponse = make([]string, 100)
	containerResponse = remoteInfo.GetList("cData")
	for _, c := range containerResponse {
		fields := strings.Split(c, ":")
		containerId := fields[0]
		if podId == fields[1] {
			return containerId, nil
		}
	}

	return "", fmt.Errorf("Container not found")
}
