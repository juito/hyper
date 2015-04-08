package daemon

import (
	"fmt"
	"dvm/engine"
)

func (daemon *Daemon) CmdCreate(job *engine.Job) error {
	cli := daemon.dockerCli
	body, _, err := cli.SendCmdCreate("hello-world:latest")
	if err != nil {
		return err
	}
	out := engine.NewOutput()
	remoteInfo, err := out.AddEnv()
	if err != nil {
		return err
	}
	if _, err := out.Write(body); err != nil {
		return fmt.Errorf("Error while reading remote info!\n")
	}
	out.Close()

	v := &engine.Env{}
	v.SetJson("ID", daemon.ID)
	if remoteInfo.Exists("Id") {
		v.Set("ContainerID", remoteInfo.Get("Id"))
	}
	fmt.Printf("The ContainerID is %s\n", remoteInfo.Get("Id"))
	if _, err := v.WriteTo(job.Stdout); err != nil {
		return err
	}

	return nil
}
