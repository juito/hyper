package client

import (
	"fmt"
	"dvm/dvmversion"
)

func (cli *DvmClient) DvmCmdVersion(args ...string) error {
	fmt.Printf("The DVM version is %s\n", dvmversion.VERSION)
	return nil
}
