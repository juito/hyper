package daemon

import (
	"dvm/engine"
)

func (daemon *Daemon) CmdCreate(job *engine.Job) error {
	cli := daemon.dockerCli
	err := cli.SendCmdCreate("tomcat:latest")
	if err != nil {
		return err
	}

	return nil
}
