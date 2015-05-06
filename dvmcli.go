package main

import (
	"flag"
	"fmt"
	"dvm/api/client"
)

func main() {
	var (
		proto	= "unix"
		addr = "/var/run/dvm.sock"
	)
	cli := client.NewDvmClient(proto, addr, nil)

	// set the flag to output
	flHelp := flag.Bool("help", false, "Help Message")
	flVersion := flag.Bool("version", false, "Version Message")
	flag.Usage = func() {cli.Cmd("help")}
	flag.Parse()
	if flag.NArg() == 0 {
		cli.Cmd("help")
		return
	}
	if *flHelp == true {
		cli.Cmd("help")
	}
	if *flVersion == true {
		cli.Cmd("version")
	}

	if err := cli.Cmd(flag.Args()...); err != nil {
		fmt.Printf("DVM ERROR: %s\n", err.Error());
	}
}
