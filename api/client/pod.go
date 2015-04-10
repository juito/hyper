package client

import (
	"encoding/json"
	"fmt"
	"dvm/engine"
    "dvm/api/pod"
)


// We neet to process the POD json data with the given file
func (*cli DvmClient) DvmCmdPod(args ...string) error {
	jsonFile := args[0]
	if jsonFile == "" {
		return fmt.Errorf("DVM ERROR: Can not accept the 'pod' command without file name!\n")
	}
    userPod := pod.ProcessPodFile(jsonFile)

    // TODO: we need to transfer this data from client to daemon
}
