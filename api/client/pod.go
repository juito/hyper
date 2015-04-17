package client

import (
	"fmt"
    "dvm/api/pod"
	"io/ioutil"
	"net/url"
	"dvm/engine"
	"dvm/api/types"
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

	jsonbody, err := ioutil.ReadFile(jsonFile)
	if err != nil {
		return err
	}

	v := url.Values{}
	v.Set("podArgs", string(jsonbody))
	body, _, err := readBody(cli.call("POST", "/pod/create?"+v.Encode(), nil, nil));
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
	if remoteInfo.Exists("ID") {
		fmt.Printf("Pod ID: %s\n", remoteInfo.Get("ID"))
	}
	// TODO we need to get the qemu response and process them
	switch remoteInfo.GetInt("Code") {
		case types.E_OK:
			fmt.Println("VM is successful to start!")
			break
		case types.E_CONTEXT_INIT_FAIL:
		case types.E_DEVICE_FAIL:
		case types.E_QMP_INIT_FAIL:
		case types.E_QMP_COMMAND_FAIL:
			fmt.Println(remoteInfo.Get("Cause"))
			break
		default:
			fmt.Println("Error getting unexpected qemu response!")
	}
	return nil
}
