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
	if len(args) == 0 {
		return fmt.Errorf("DVM ERROR: Can not accept the 'pod' command without file name!\n")
	}
	jsonFile := args[0]

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
	errCode := remoteInfo.GetInt("Code")
	if errCode == types.E_OK {
		fmt.Println("VM is successful to start!")
	} else {
		// case types.E_CONTEXT_INIT_FAIL:
		// case types.E_DEVICE_FAIL:
		// case types.E_QMP_INIT_FAIL:
		// case types.E_QMP_COMMAND_FAIL:
		if errCode != types.E_CONTEXT_INIT_FAIL &&
		    errCode != types.E_DEVICE_FAIL &&
			errCode != types.E_QMP_INIT_FAIL &&
			errCode != types.E_QMP_COMMAND_FAIL {
			fmt.Println("DVM error: Got an unexpected error code during create POD!\n")
		} else {
			fmt.Printf("DVM error: %s\n", remoteInfo.Get("Cause"))
		}
	}
	return nil
}
