package qemu

// #include <termios.h>
import "C"
import (
    "errors"
    "os"
    "syscall"
    "unsafe"
    "os/signal"
    "dvm/lib/glog"
)

type winsize struct {
    ws_row uint16
    ws_col uint16
    ws_xpixel uint16
    ws_ypixel uint16
}

func GetTermSize(file *os.File) (*WindowSize, error) {
    var size winsize
    _,_,err := syscall.Syscall(syscall.SYS_IOCTL, uintptr(file.Fd()), uintptr(syscall.TIOCGWINSZ), uintptr(unsafe.Pointer(&size)))
    if int(err) < 0 {
        return nil, errors.New("Get window size failed")
    }

    return &WindowSize{Row:size.ws_row, Column:size.ws_col,}, nil
}

func SetTermSize(file *os.File, size *WindowSize) error {
    s := winsize{
        ws_row: size.Row,
        ws_col: size.Column,
    }
    _,_,err := syscall.Syscall(syscall.SYS_IOCTL, uintptr(file.Fd()), uintptr(syscall.TIOCSWINSZ), uintptr(unsafe.Pointer(&s)))
    if int(err) < 0 {
        return errors.New("Set window size failed")
    }
    return nil
}

func TtySizeMonitor(ctx *QemuContext, id string) {
    sigs := make(chan os.Signal, 4)
    signal.Notify(sigs, syscall.SIGWINCH)

    go func(){
        for {
            _,ok := <-sigs
            if !ok {
                return
            }
            ws,err := GetTermSize(os.Stdin)
            if err != nil {
                return
            }
            ctx.hub <- &WindowSizeCommand{
                Container:  id,
                Size:       ws,
            }
            glog.V(1).Infof("Window size changed to %dx%d", ws.Row, ws.Column)
        }
    }()
}

func setNoEcho(file *os.File) error {
    var stermios C.struct_termios
    err := C.tcgetattr(C.int(file.Fd()), &stermios)
    if int(err) < 0 {
        return errors.New("failed to get attr")
    }

    stermios.c_lflag &= ^C.tcflag_t(C.ECHO|C.ECHOE|C.ECHOK|C.ECHONL)
    stermios.c_oflag &= ^C.tcflag_t(C.ONLCR)

    err = C.tcsetattr(C.int(file.Fd()), C.TCSANOW, &stermios)
    if int(err) < 0 {
        return errors.New("failed to set attr")
    }

    return nil
}
