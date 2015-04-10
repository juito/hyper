package client

import (
	"fmt"
	"net/url"
)

func (cli *DvmClient) DvmCmdPull(args ...string) error {
	// we need to get the image name which will be used to create a container
	imageName := args[0]
	if imageName == "" {
		return fmt.Errorf("DVM ERROR: \"create\" requires a minimum of 1 argument.\n")
	}
	v := url.Values{}
	v.Set("imageName", imageName)
	err := cli.stream("POST", "/image/create?"+v.Encode(), nil, nil, nil)
	if err != nil {
		return err
	}

	return nil
}
