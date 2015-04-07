package docker

import (
	"fmt"
	"net/url"
	"strings"
)

func (cli *DockerCli) SendCmdCreate(args ...string) error {
	// We need to create a container via an image object.  If the image
	// is not stored locally, so we need to pull the image from the Docker HUB.
	// After that, we have prepared the whole stuffs to create a container.

	// Get a Repository name and tag name from the argument, but be careful
	// with the Repository name with a port number.  For example:
	//      localdomain:5000/samba/hipache:latest
	image := args[0]
	repos, tag := parseTheGivenImageName(image)
	if tag == "" {
		tag = "latest"
	}

	// Pull the image from the docker HUB
	v := url.Values{}
	v.Set("fromImage", repos)
	v.Set("tag", tag)
	_, statusCode, err := cli.Call("POST", "/image/create?"+ v.Encode(), nil, nil)
	if err != nil {
		return err
	}
	fmt.Printf("The returned status code is %s", statusCode)
	//response, err := cli.createContainer(config, hostConfig, hostConfig.ContainerIDFile, *flName)
	return nil
}

func parseTheGivenImageName(image string) (string, string) {
	n := strings.Index(image, "@")
	if n > 0 {
		parts := strings.Split(image, "@")
		return parts[0], parts[1]
	}

	n = strings.LastIndex(image, ":")
	if n < 0 {
		return image, ""
	}
	if tag := image[n+1:]; !strings.Contains(tag, "/") {
		return image[:n], tag
	}
	return image, ""
}
