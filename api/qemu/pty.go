package qemu

// #include <termios.h>
import "C"
import (
    "os"
    "errors"
)

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
