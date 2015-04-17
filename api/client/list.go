package client

import (
	"fmt"
//    "dvm/api/pod"
//	"io/ioutil"
	"strings"
	"net/url"
	"dvm/engine"
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
		vmResponse []string
		podResponse []string
	)
	if remoteInfo.Exists("item") {
		item = remoteInfo.Get("item")
	}
	if remoteInfo.Exists("Error") {
		fmt.Printf("Found an error while getting %s list: %s", item, remoteInfo.Get("Error"))
	}

	if item == "vm" || item == "pod" {
		vmResponse = remoteInfo.GetList("vm")
	}
	if item == "pod" {
		podResponse = remoteInfo.GetList("pod")
		fmt.Printf("%v\n", podResponse)
	}

	for _, vm := range vmResponse {
		fmt.Printf("Pod: %s, VM: %s\n", vm[:strings.Index(vm, "-")], vm[strings.Index(vm, "-")+1:])
	}
	return nil
}
