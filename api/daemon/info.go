package daemon

import (
	"os"
	"fmt"

	"dvm/dvmversion"
	"dvm/engine"
)

func (daemon *Daemon) CmdInfo(job *engine.Job) error {
	cli := daemon.dockerCli
	body, _, err := cli.SendCmdInfo("")
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
	if remoteInfo.Exists("Containers") {
		v.SetInt("Containers", remoteInfo.GetInt("Containers"))
	}
	v.SetInt("Images", 20)
	v.Set("Driver", "test-1")
	v.SetBool("Debug", true)
	v.SetInt("NFd", 0)
	v.SetInt("NGoroutines", 0)
	v.Set("SystemTime", "2015-04-03")
	v.Set("KernelVersion", "3.18")
	v.Set("OperatingSystem", "Linux")
	v.Set("InitSha1", dvmversion.INITSHA1)
	v.SetInt("NCPU", 2)
	v.SetInt64("MemTotal", 1024)
	if hostname, err := os.Hostname(); err == nil {
		v.SetJson("Name", hostname)
	}
	if _, err := v.WriteTo(job.Stdout); err != nil {
		return err
	}
	return nil
}
