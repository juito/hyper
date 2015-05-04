package client

import (
    "fmt"
    "io"
    "net/url"
    "strings"
    "dvm/lib/promise"
)

func (cli *DvmClient) DvmCmdAttach(args ...string) error {
    if len(args) == 0 {
        return fmt.Errorf("Can not accept the 'attach' command without POD/Container ID!")
    }
    if len(args) > 1 {
        return fmt.Errorf("Can not accept the 'attach' command with additional command!")
    }
    var (
        podName = args[0]
        hostname = ""
        err error
        tag = cli.GetTag()
    )
    fmt.Printf("The pod name is %s", podName)

    v := url.Values{}
    if strings.Contains(podName, "pod-") {
        hostname, err = cli.GetPodInfo(podName)
        if err != nil {
            return err
        }
        if hostname == "" {
            return fmt.Errorf("The POD : %s does not exist, please create it before exec!", podName)
        }
        v.Set("type", "pod")
        v.Set("value", podName)
    } else {
        v.Set("type", "container")
        v.Set("value", podName)
    }
    v.Set("tag", tag)

    var (
        hijacked    = make(chan io.Closer)
        errCh       chan error
    )
    // Block the return until the chan gets closed
    defer func() {
        fmt.Printf("End of CmdExec(), Waiting for hijack to finish.\n")
        if _, ok := <-hijacked; ok {
            fmt.Printf("Hijack did not finish (chan still open)\n")
        }
    }()

    errCh = promise.Go(func() error {
        return cli.hijack("POST", "/attach?"+v.Encode(), true, cli.in, cli.out, cli.out, hijacked, nil, hostname)
    })

    if err := cli.monitorTtySize(podName, tag); err != nil {
        fmt.Printf("Monitor tty size fail for %s!\n", podName)
    }

    // Acknowledge the hijack before starting
    select {
    case closer := <-hijacked:
    // Make sure that hijack gets closed when returning. (result
    // in closing hijack chan and freeing server's goroutines.
        if closer != nil {
            defer closer.Close()
        }
    case err := <-errCh:
        if err != nil {
            fmt.Printf("Error hijack: %s", err.Error())
            return err
        }
    }

    if err := <-errCh; err != nil {
        fmt.Printf("Error hijack: %s", err.Error())
        return err
    }
    fmt.Printf("Success to attach to POD %s!\n", podName)
    return nil
}
