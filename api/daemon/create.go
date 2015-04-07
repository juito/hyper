package daemon

import (
	"dvm/engine"
)

func (daemon *Daemon) CmdCreate(job *engine.Job) error {
	cli := daemon.dockerCli
	err := cli.SendCmdCreate("hello-world:latest")
	if err != nil {
		return err
	}

	return nil
}
