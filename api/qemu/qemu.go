package qemu

import (
    "io"
    "dvm/lib/glog"
    "os/exec"
    "strings"
)

func printDebugOutput(tag string, out io.ReadCloser) {
    buf := make([]byte, 1024)
    for {
        n,err:=out.Read(buf)
        if err == io.EOF {
            glog.V(0).Infof("%s finish", tag)
            break
        } else if err != nil {
            glog.Error(err)
        }
        glog.V(0).Infof("got %s: %s", tag, string(buf[:n]))
    }
}

func waitConsoleOutput(ctx *QemuContext) {
    conn, err := ctx.consoleSock.AcceptUnix()
    if err != nil {
        glog.Warning(err.Error())
        return
    }

    tc := setupTty(ctx.consoleSockName, conn, make(chan interface{}))
    tty := tc.Get()
    ctx.consoleTty = tc
    tc.start()

    for {
        line,ok := <- tty.Output
        if ok {
            glog.V(1).Info("[console] ", line)
        } else {
            glog.Info("console output end")
            break
        }
    }
}

func watchDog(ctx* QemuContext) {
    for {
        msg,ok := <- ctx.wdt
        if ok {
            switch msg {
                case "quit":
                glog.V(1).Info("quit watch dog.")
                return
                case "kill":
                if ctx.process != nil {
                    glog.V(0).Infof("kill Qemu... %d", ctx.process.Pid)
                    ctx.process.Kill()
                } else {
                    glog.Warning("no process to be killed")
                }
                return
            }
        } else {
            glog.V(1).Info("chan closed, quit watch dog.")
        }
    }
}

// launchQemu run qemu and wait it's quit, includes
func launchQemu(ctx *QemuContext) {
    qemu,err := exec.LookPath("qemu-system-x86_64")
    if  err != nil {
        ctx.hub <- &QemuExitEvent{message:"can not find qemu executable"}
        return
    }

    args := ctx.QemuArguments()

    glog.V(0).Info("cmdline arguments: ", strings.Join(args, " "))

    cmd := exec.Command(qemu, args...)

    stderr,err := cmd.StderrPipe()
    if err != nil {
        glog.Warning("Cannot get stderr of qemu")
    }

    go printDebugOutput("stderr", stderr)

    if err := cmd.Start();err != nil {
        glog.Error("try to start qemu failed")
        ctx.hub <- &QemuExitEvent{message:"try to start qemu failed"}
        return
    }

    ctx.process = cmd.Process
    go watchDog(ctx)

    glog.V(0).Info("Waiting for command to finish...")

    err = cmd.Wait()
    if err != nil {
        glog.Info("qemu exit with ", err.Error())
        ctx.hub <- &QemuExitEvent{message:"qemu exit with " + err.Error()}
    } else {
        glog.Info("qemu exit with 0")
        ctx.hub <- &QemuExitEvent{message:"qemu exit with 0"}
    }
    ctx.wdt <- "quit"
}
