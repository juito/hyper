package main

import (
	"flag"
	"fmt"
	"dvm/dvmversion"
	"dvm/api/client"
)

func main() {
	flag.Parse()

	if (flag.NArg() < 0)  {
		showHelp()
		return
	}

	var (
		proto	= "unix"
		//addr	= "/var/run/docker.sock"
		addr = "/var/run/dvm.sock"
	)
	cli := client.NewDvmClient(proto, addr, nil)

	if err := cli.Cmd(flag.Args()...); err != nil {
		fmt.Printf("There is something worng during executing the command!\n");
	}
}

func showVersion() {
	fmt.Printf("DVM version %s, build %s\n", dvmversion.VERSION, dvmversion.GITCOMMIT)
}

func showHelp() {
	fmt.Printf("DVM help info:\n  Currently, we just support 'dvm info' command\n");
}
