package client

import (
	"fmt"
	"dvm/engine"
	"net/url"
)

func (cli *DvmClient) DvmCmdCreate(args ...string) error {
	// we need to get the image name which will be used to create a container
	imageName := args[0]
	if imageName == "" {
		return fmt.Errorf("DVM ERROR: \"create\" requires a minimum of 1 argument.\n")
	}
	containerValues := url.Values{}
	containerValues.Set("imageName", imageName)
	body, _, err := readBody(cli.call("POST", "/container/create?"+containerValues.Encode(), nil, nil))
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
