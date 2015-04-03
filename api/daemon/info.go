package daemon

import (
	"os"

	"dvm/dvmversion"
	"dvm/engine"
)

func (daemon *Daemon) CmdInfo(job *engine.Job) error {
	v := &engine.Env{}
	v.SetJson("ID", daemon.ID)
	v.SetInt("Containers", 10)
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
