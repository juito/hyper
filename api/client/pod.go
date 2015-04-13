package client

import (
	"fmt"
    "dvm/api/pod"
	"io/ioutil"
	"net/url"
)


// We neet to process the POD json data with the given file
func (cli *DvmClient) DvmCmdPod(args ...string) error {
	jsonFile := args[0]
	if jsonFile == "" {
		return fmt.Errorf("DVM ERROR: Can not accept the 'pod' command without file name!\n")
	}
    userPod, err := pod.ProcessPodFile(jsonFile)
	if err != nil {
		return err
	}
	fmt.Printf("User Pod Name is %s\n", userPod.Name)

    // TODO: we need to transfer this data from client to daemon
	body, err := ioutil.ReadFile(jsonFile)
	if err != nil {
		return err
	}

	v := url.Values{}
	v.Set("podArgs", string(body))
	if _, _, err := readBody(cli.call("POST", "/pod/create"+v.Encode(), nil, nil)); err != nil {
		return err
	}
	return nil
}
