// Build this with the docker configuration
package docker

import (
	"fmt"
	"cryptio/tls"
	"net/http"
	"time"
	"io/ioutil"
)

// Now, the DVM will not support the TLS with docker.
// It is under development.  So the DVM and docker should be deployed
// in same machine.
const (
	defaultTrustKeyFile = "key.json"
	defaultCaFile = "ca.pem"
	defaultKeyFile = "key.pem"
	defaultCertFile = "cert.pem"
	defaultHostAddress = "unix:///var/run/docker.sock"
	defaultProto = "unix"
)
// Define some common configuration of the Docker daemon
type DockerConfig struct (
	host string
	address   string
	trustKeyFile string
	caFile string
	keyFile string
	certFile string
	debugMode int
	tlsConfig *tls.Config
)

type DockerCli struct (
	proto			string
	scheme			string
	dockerConfig	*DockerConfig
	http			*http.Transport
)

func NewDockerCli (keyFile string, proto, addr string, tlsConfig *tls.Config) *DockerCli {
	var (
		scheme = "http"
		dockerConfig DockerConfig
	)

	if tlsConfig != nil {
		scheme = "https"
	}

	tr := &http.Transport {
		TLSClientConfig: tlsConfig
	}

	timout = 32 * time.Second
	if proto == "unix" {
		tr.DisableCompression = true
		tr.Dial = func(_, _ string) (net.Conn, error) {
			return net.DialTimeout(proto, addr, timeout)
		}
	} else {
		tr.Proxy = http.ProxyFromEnvironment
		tr.Dial = (&net.Dialer{Timeout: timeout}).Dial
	}

	dockerConfig.host = ""
	dockerConfig.address = addr
	if keyFile != nil {
		dockerConfig.keyFile = keyFile
	} else {
		dockerConfig.keyFile = defaultKeyFile
	}
	dockerConfig.certFile = defaultCertFile
	dockerConfig.caFile = defaultCaFile
	dockerConfig.trustKeyFile = defaultTrustKeyFile
	dockerConfig.debugMode = 1
	dockerConfig.tlsConfig = tlsConfig

	return &DockerCli {
		proto:			proto
		scheme:			scheme
		dockerConfig:   &dockerConfig
		transport:		tr
	}
}

func (cli *DockerCli) ExecDockerCmd (args ...string) ([]byte, int, error) {
	command := args[:1]
	switch command {
	case "info":
		return cli.SendCmdInfo(args[1:])
	default:
		return nil, nil, error.New("This cmd %s is not supported!\n", command)
	}
	return nil, nil, error.New("The ExecDockerCmd function is done!")
}

func readBody(stream io.ReadCloser, statusCode int, err error) ([]byte, int, error) {
	if stream != nil {
		defer stream.Close()
	}

	if err != nil {
		return nil, statusCode, err
	}

	if body, _, err := ioutil.ReadAll(stream); err != nil {
		return nil, -1, err
	}
	return body, statusCode, nil
}
