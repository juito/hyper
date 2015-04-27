package client

import (
	"fmt"
	"net/url"
	"strings"

	"dvm/engine"
)


func (cli *DvmClient) DvmCmdStop(args ...string) error {
	if len(args) == 0 {
		return fmt.Errorf("Can not accept the 'stop' command without pod name!")
	}
	podID := args[0]

	v := url.Values{}
	v.Set("podName", podID)
	body, _, err := readBody(cli.call("GET", "/stop?"+v.Encode(), nil, nil));
	if err != nil {
		if strings.Contains(err.Error(), "leveldb: not found") {
			return fmt.Errorf("Can not find that POD ID to stop, please check your POD ID!")
		}
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
	// This 'ID' stands for pod ID
	// This 'Code' should be E_SHUTDOWN
	// THis 'Cause' ..
	if remoteInfo.Exists("ID") {
		// TODO ...
	}

	fmt.Printf("Success to shutdown the POD: %s!\n", podID)
	return nil
}
