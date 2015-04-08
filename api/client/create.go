package client

import (
	"fmt"
	"dvm/engine"
)

func (cli *DvmClient) DvmCmdCreate(args ...string) error {
	body, _, err := readBody(cli.call("POST", "/container/create", nil, nil))
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
	if remoteInfo.Exists("ContainerID") {
		fmt.Printf("New Container ID: %s\n", remoteInfo.Get("ContainerID"))
	}
	return nil
}
