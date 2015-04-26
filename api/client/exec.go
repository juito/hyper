package client

import (
	"fmt"
	"io"
	"net/url"
	"strings"
	"dvm/lib/promise"
)

func (cli *DvmClient) DvmCmdExec(args ...string) error {
	if len(args) == 0 {
		return fmt.Errorf("Can not accept the 'exec' command without container ID!")
	}
	if len(args) == 1 {
		return fmt.Errorf("Can not accept the 'exec' command without command!")
	}
	podName := args[0]
	command := strings.Join(args[1:], "")

	fmt.Printf("The pod name is %s, command is %s\n", podName, command)

	v := url.Values{}
	v.Set("podname", podName)
	v.Set("command", command)

	var (
		hijacked    = make(chan io.Closer)
		errCh       chan error
	)
	// Block the return until the chan gets closed
	defer func() {
		fmt.Printf("End of CmdExec(), Waiting for hijack to finish.\n")
		if _, ok := <-hijacked; ok {
			fmt.Printf("Hijack did not finish (chan still open)\n")
		}
	}()

	errCh = promise.Go(func() error {
		return cli.hijack("POST", "/exec?"+v.Encode(), true, cli.in, cli.out, cli.out, hijacked, nil)
	})

	// Acknowledge the hijack before starting
	select {
	case closer := <-hijacked:
		// Make sure that hijack gets closed when returning. (result
		// in closing hijack chan and freeing server's goroutines.
		if closer != nil {
			defer closer.Close()
		}
	case err := <-errCh:
		if err != nil {
			fmt.Printf("Error hijack: %s", err.Error())
			return err
		}
	}

	if err := <-errCh; err != nil {
		fmt.Printf("Error hijack: %s", err.Error())
		return err
	}
	fmt.Printf("Success to exec the command %s for POD %s!\n", command, podName)
	return nil
}
