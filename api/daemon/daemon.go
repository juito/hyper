package daemon

import (
	"fmt"
	"os"
	"runtime"
	apiserver "dvm/api/server"
	"dvm/engine"
	"dvm/lib/portallocator"
	"dvm/api/docker"
)

type Daemon struct {
	ID               string
	eng              *engine.Engine
	dockerCli		 *docker.DockerCli
}

// Install installs daemon capabilities to eng.
func (daemon *Daemon) Install(eng *engine.Engine) error {
	// Now, we just install a command 'info' to set/get the information of the docker and DVM daemon
	for name, method := range map[string]engine.Handler{
		"info":              daemon.CmdInfo,
		"create":			 daemon.CmdCreate,
		"pull":				 daemon.CmdPull,
		"serveapi":			 apiserver.ServeApi,
		"acceptconnections": apiserver.AcceptConnections,
	} {
		fmt.Printf("Engine Register: name= %s\n", name)
		if err := eng.Register(name, method); err != nil {
			return err
		}
	}
	return nil
}

func NewDaemon(eng *engine.Engine) (*Daemon, error) {
	daemon, err := NewDaemonFromDirectory(eng)
	if err != nil {
		return nil, err
	}
	return daemon, nil
}

func NewDaemonFromDirectory(eng *engine.Engine) (*Daemon, error) {
	// register portallocator release on shutdown
	eng.OnShutdown(func() {
		if err := portallocator.ReleaseAll(); err != nil {
			fmt.Printf("portallocator.ReleaseAll(): %s", err)
		}
	})
	// Check that the system is supported and we have sufficient privileges
	if runtime.GOOS != "linux" {
		return nil, fmt.Errorf("The Docker daemon is only supported on linux")
	}
	if os.Geteuid() != 0 {
		return nil, fmt.Errorf("The Docker daemon needs to be run as root")
	}
	if err := checkKernel(); err != nil {
		return nil, err
	}

	os.Setenv("TMPDIR", "/var/tmp/dvm/")

	var realRoot = "/var/run/dvm/"
	// Create the root directory if it doesn't exists
	if err := os.MkdirAll(realRoot, 0700); err != nil && !os.IsExist(err) {
		return nil, err
	}

	var (
		proto = "unix"
		addr = "/var/run/docker.sock"
	)
	dockerCli := docker.NewDockerCli("", proto, addr, nil)
	daemon := &Daemon{
		ID:               "1024",
		eng:              eng,
		dockerCli:		  dockerCli,
	}

	eng.OnShutdown(func() {
		if err := daemon.shutdown(); err != nil {
			fmt.Printf("Error during daemon.shutdown(): %v", err)
		}
	})

	return daemon, nil
}

func (daemon *Daemon) shutdown() error {
	fmt.Printf("The daemon will be shutdown\n")
	return nil
}

// Now, the daemon can be ran for any linux kernel
func checkKernel() error {
	return nil
}
