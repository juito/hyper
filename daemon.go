package main

import (
	"os"
	"fmt"
	"flag"
	"syscall"
	"os/signal"

	"dvm/api/daemon"
	"dvm/engine"
	"dvm/dvmversion"
	"dvm/lib/glog"
	"dvm/lib/goconfig"
)

func main() {
	flConfig := flag.String("config", "", "Config file for DVM")
	flHelp := flag.Bool("help", false, "Print help message for DVM daemon")
	glog.Init()
	flag.Usage = func() {printHelp()}
	flag.Parse()
	if *flHelp == true {
		printHelp()
		return
	}
	mainDaemon(*flConfig)
}

func printHelp() {
	var helpMessage = `Usage:
  dvmd [OPTIONS]

Application Options:
  --config=""            configuration for DVM 
  --v=0                  log level fro V logs
  --logtostderr          log to standard error instead of files
  --alsologtostderr      log to standard error as well as files

Help Options:
  -h, --help             Show this help message

`
	fmt.Printf(helpMessage)
}

func mainDaemon(config string) {
	glog.V(0).Infof("The config file is %s", config)
	eng := engine.New()
	if config == "" {
		config = "/etc/dvm/config"
	}
	cfg, err := goconfig.LoadConfigFile(config)
	if err != nil {
		glog.Errorf("Read config file (%s) failed, %s", config, err.Error())
		return
	}
	kernel, _ := cfg.GetValue(goconfig.DEFAULT_SECTION, "Kernel")
	initrd, _ := cfg.GetValue(goconfig.DEFAULT_SECTION, "Initrd")
	glog.V(0).Infof("The config: kernel=%s, initrd=%s", kernel, initrd)
	os.Setenv("Kernel", kernel)
	os.Setenv("Initrd", initrd)
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
