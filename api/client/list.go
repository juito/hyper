package client

import (
	"fmt"
	"encoding/json"
	"strings"
	"net/url"
	"dvm/engine"
    "dvm/api/pod"
)


func (cli *DvmClient) DvmCmdList(args ...string) error {
	var item string
	if len(args) == 0 {
		item = "pod"
	} else {
		item = args[0]
	}

	if item != "pod" && item != "vm" && item != "container" {
		return fmt.Errorf("Error, the dvm can not support %s list!", item)
	}

	v := url.Values{}
	v.Set("item", item)
	body, _, err := readBody(cli.call("GET", "/list?"+v.Encode(), nil, nil));
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

	var (
		vmResponse = make([]string, 100)
		podResponse = make([]string, 100)
		containerResponse = make([]string, 100)
	)
	if remoteInfo.Exists("item") {
		item = remoteInfo.Get("item")
	}
	if remoteInfo.Exists("Error") {
		fmt.Printf("Found an error while getting %s list: %s", item, remoteInfo.Get("Error"))
	}

	if item == "vm" || item == "pod" {
		vmResponse = remoteInfo.GetList("vmData")
	}
	if item == "pod" {
		podResponse = remoteInfo.GetList("podData")
	}
	if item == "container" {
		containerResponse = remoteInfo.GetList("cData")
	}

	var (
		tempPod pod.UserPod
	)

	fmt.Printf("Item is %s\n", item)
	if item == "vm" {
		fmt.Printf("     VM name\n")
		for _, vm := range vmResponse {
			vmid  := vm[:strings.Index(vm, ":")]
			fmt.Printf("%15s\n", vmid)
		}
	}

	if item == "pod" {
		fmt.Printf("%15s%25s%20s%10s\n", "POD ID", "POD Name", "VM name", "Status")
		for i, vm := range vmResponse {
			fields := strings.Split(vm, ":")
			if err := json.Unmarshal([]byte(podResponse[i]), &tempPod); err != nil {
				return err
			}
			fmt.Printf("%15s%25s%20s%10s\n", fields[1], tempPod.Name, fields[0], fields[2])
		}
	}

	if item == "container" {
		fmt.Printf("%-66s%15s%10s\n", "Container ID", "POD ID", "Status")
		for _, c := range containerResponse {
			fields := strings.Split(c, ":")
			fmt.Printf("%-66s%15s%10s\n", fields[0], fields[1], fields[2])
		}
	}
	return nil
}
