package docker

import (
	"fmt"
	"net/url"
	"dvm/lib/glog"
)

func (cli *DockerCli) SendCmdDelete(args ...string) ([]byte, int, error) {
	container := args[0]
	glog.V(1).Infof("Prepare to delete the container : %s", container)
	v := url.Values{}
	v.Set("v", "1")
	v.Set("link", "1")
	v.Set("force", "1")
	_, statusCode, err := readBody(cli.Call("DELETE", "/containers/"+container[:12]+"?"+v.Encode(), nil, nil))
	if err != nil {
		return nil, statusCode, fmt.Errorf("Error to remove the container(%s), %s", container, err.Error())
	}
	glog.V(1).Infof("status code is %d", statusCode)

	return nil, statusCode, nil
}
