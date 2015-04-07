package main

import (
	"fmt"
	"strings"

	"dvm/api/daemon"
	"dvm/engine"
	"dvm/dvmversion"
)

func main() {
	mainDaemon()
}

func mainDaemon() {
	eng := engine.New()
	d, err := daemon.NewDaemon(eng)
	if err != nil {
		fmt.Printf("ERROR: the daemin create failed!\n")
		return
	}

	daemonInitWait := make(chan error)
	// Install the accepted jobs
	if err := d.Install(eng); err != nil {
		fmt.Printf("ERROR: the daemin install failed!\n")
		return
	}

	fmt.Printf("DVM daemon: %s %s\n",
		dvmversion.VERSION,
		dvmversion.GITCOMMIT,
	)

	// after the daemon is done setting up we can tell the api to start
	// accepting connections
	if err := eng.Job("acceptconnections").Run(); err != nil {
		fmt.Printf("ERROR: the acceptconnections job run failed!\n")
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
			fmt.Printf("ServeAPI error: %v\n", err)
			serveAPIWait <- err
			return
		}
		serveAPIWait <- nil
	}()

	// Wait for the daemon startup goroutine to finish
	// This makes sure we can actually cleanly shutdown the daemon
	errDaemon := <-daemonInitWait
	if errDaemon != nil {
		eng.Shutdown()
		outStr := fmt.Sprintf("Shutting down daemon due to errors: %v", errDaemon)
		if strings.Contains(errDaemon.Error(), "engine is shutdown") {
			// if the error is "engine is shutdown", we've already reported (or
			// will report below in API server errors) the error
			outStr = "Shutting down daemon due to reported errors\n"
		}
		// we must "fatal" exit here as the API server may be happy to
		// continue listening forever if the error had no impact to API
		fmt.Printf(outStr)
	} else {
		fmt.Printf("Daemon has completed initialization\n")
	}

	// Daemon is fully initialized and handling API traffic
	// Wait for serve API job to complete
	errAPI := <-serveAPIWait
	// If we have an error here it is unique to API (as daemonErr would have
	// exited the daemon process above)
	eng.Shutdown()
	if errAPI != nil {
		fmt.Printf("Shutting down due to ServeAPI error: %v\n", errAPI)
	}
}
