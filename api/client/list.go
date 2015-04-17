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

	var (
		tempPod pod.UserPod
		podVm   = make(map[string]string, len(vmResponse))
	)
	fmt.Printf("Item is %s\n", item)
	for _, vm := range vmResponse {
		podVm[vm[strings.Index(vm, "-")+1:]] = vm[:strings.Index(vm, "-")]
	}

	if item == "pod" {
		fmt.Printf("POD name                   VM name\n")
		for _, pod := range podResponse {
			if err := json.Unmarshal([]byte(pod), &tempPod); err != nil {
				return err
			}
			fmt.Printf("%s                         %s\n", tempPod.Name, podVm[tempPod.Name])
		}
	}

	return nil
}
