package client

import (
	"fmt"
)

func (cli *DvmClient) DvmCmdHelp(args ...string) error {
	var helpMessage = `Usage:
  dvmcli [OPTIONS] COMMAND [ARGS...]

Command:
  attach                 Attach to a running container
  create                 Create a new container
  exec                   Run a command in a running container
  info                   Display system-wide information
  list                   List the PODs, VMs or containers
  pod                    Create a new POD
  pull                   Pull an image or a repository from a Docker registry server
  run                    Run a command in a new container
  stop                   Stop a running container

Help Options:
  -h, --help             Show this help message

Run 'dvm COMMAND --help' for more information on a command.
`
	fmt.Printf(helpMessage)
	return nil
}
