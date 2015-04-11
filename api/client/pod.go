package client

import (
	"fmt"
    "dvm/api/pod"
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
	return nil
}
