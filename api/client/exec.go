package client

import (
	"fmt"
	"net/url"
	"strings"
	"dvm/engine"
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
	body, _, err := readBody(cli.call("POST", "/exec?"+v.Encode(), nil, nil));
	if err != nil {
		return err
	}
	out := engine.NewOutput()
	remoteInfo, err := out.AddEnv()
	if err != nil {
		return err
	}

	if _, err := out.Write(body); err != nil {
		fmt.Printf("Error reading remote info: %s", err)
		return err
	}
	out.Close()
	// We need a result while executing a command
	// This 'ID' stands for pod name
	// This 'Code'
	// THis 'Cause' ..
	if remoteInfo.Exists("ID") {
		// TODO ...
	}

	fmt.Printf("Success to exec the command %s for POD %s!\n", command, podName)
	return nil
}
