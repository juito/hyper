package main

import (
	"os"
	"os/signal"
	"syscall"
	"flag"

	"dvm/api/daemon"
	"dvm/engine"
	"dvm/dvmversion"
	"dvm/lib/glog"
)

func main() {
	flag.Parse()
	mainDaemon()
}

func mainDaemon() {
	eng := engine.New()
	d, err := daemon.NewDaemon(eng)
	if err != nil {
		glog.Error("the daemon create failed, %s\n", err.Error())
		return
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	// Install the accepted jobs
	if err := d.Install(eng); err != nil {
		glog.Error("the daemin install failed, %s\n", err.Error())
		return
	}

	glog.V(0).Infof("DVM daemon: %s %s\n",
		dvmversion.VERSION,
		dvmversion.GITCOMMIT,
	)

	// after the daemon is done setting up we can tell the api to start
	// accepting connections
	if err := eng.Job("acceptconnections").Run(); err != nil {
		glog.Error("the acceptconnections job run failed!\n")
		return
	}
	defaulthost := "unix:///var/run/dvm.sock"

	job := eng.Job("serveapi", defaulthost)

	// The serve API job never exits unless an error occurs
	// We need to start it as a goroutine and wait on it so
	// daemon doesn't exit
	serveAPIWait := make(chan error)
	go func() {
		if err := job.Run(); err != nil {
			glog.Errorf("ServeAPI error: %v\n", err)
			serveAPIWait <- err
			return
		}
		serveAPIWait <- nil
	}()

	glog.V(0).Info("Daemon has completed initialization\n")

	// Daemon is fully initialized and handling API traffic
	// Wait for serve API job to complete
	select {
	case errAPI := <-serveAPIWait:
		// If we have an error here it is unique to API (as daemonErr would have
		// exited the daemon process above)
		eng.Shutdown()
		if errAPI != nil {
			glog.Warningf("Shutting down due to ServeAPI error: %v\n", errAPI)
		}
		break
	case <-stop:
		eng.Shutdown()
		break
	}
}
