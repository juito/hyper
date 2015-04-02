package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"dvm/dvmversion"
	"dvm/api/daemon"
)

func main() {
	mainDaemon()
}

func mainDaemon() {
	if flag.NArg() != 0 {
		fmt.Printf("Please specify your arguments!\n")
		return
	}

	// load the daemon in the background so we can immediately start
	// the http api so that connections don't fail while the daemon
	// is booting
	daemonInitWait := make(chan error)
	go func() {
		d, err := daemon.NewDaemon(daemonCfg, eng)
		if err != nil {
			daemonInitWait <- err
			return
		}

		fmt.Printf("DVM daemon: %s %s",
			dockerversion.VERSION,
			dockerversion.GITCOMMIT,
		)

		if err := d.Install(eng); err != nil {
			daemonInitWait <- err
			return
		}

		b := &builder.BuilderJob{eng, d}
		b.Install()

		// after the daemon is done setting up we can tell the api to start
		// accepting connections
		if err := eng.Job("acceptconnections").Run(); err != nil {
			daemonInitWait <- err
			return
		}
		daemonInitWait <- nil
	}()

	job := eng.Job("serveapi", flHosts...)

	// The serve API job never exits unless an error occurs
	// We need to start it as a goroutine and wait on it so
	// daemon doesn't exit
	serveAPIWait := make(chan error)
	go func() {
		if err := job.Run(); err != nil {
			log.Errorf("ServeAPI error: %v", err)
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
			outStr = "Shutting down daemon due to reported errors"
		}
		// we must "fatal" exit here as the API server may be happy to
		// continue listening forever if the error had no impact to API
		log.Fatal(outStr)
	} else {
		log.Info("Daemon has completed initialization")
	}

	// Daemon is fully initialized and handling API traffic
	// Wait for serve API job to complete
	errAPI := <-serveAPIWait
	// If we have an error here it is unique to API (as daemonErr would have
	// exited the daemon process above)
	eng.Shutdown()
	if errAPI != nil {
		log.Fatalf("Shutting down due to ServeAPI error: %v", errAPI)
	}

}
