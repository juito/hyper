package qemu

const (
    BaseDir     = "/var/run/dvm"
    DvmSockName = "dvm.sock"
    QmpSockName = "qmp.sock"
    ConsoleSockName = "console.sock"
    ShareDir    = "share_dir"
    Kernel      = "/sources/dvminit/test/kernel"
    Initrd      = "/sources/dvminit/test/initrd-dvm.img"
)

const(
    EVENT_QEMU_EXIT = iota
    EVENT_QEMU_TIMEOUT
    EVENT_INIT_CONNECTED
    EVENT_QMP_EVENT
    EVENT_CONTAINER_ADD
    EVENT_CONTAINER_DELETE
    EVENT_VOLUME_ADD
    EVENT_VOLUME_DELETE
    EVENT_PATH_BOUND
    EVENT_PATH_UNBOUND
    EVENT_BLOCK_INSERTED
    EVENT_BLOCK_EJECTED
    EVENT_INTERFACE_ADD
    EVENT_INTERFACE_DELETE
    EVENT_INTERFACE_INSERTED
    EVENT_INTERFACE_EJECTED
    COMMAND_RUN_POD
    COMMAND_SHUTDOWN
    COMMAND_EXEC
    COMMAND_ACK
    ERROR_INIT_FAIL
)

const(
    QMP_INIT = iota
    QMP_SESSION
    QMP_FINISH
    QMP_EVENT
    QMP_INTERNAL_ERROR
    QMP_QUIT
    QMP_TIMEOUT
    QMP_RESULT
    QMP_ERROR
)

const(
    INIT_SETDVM = iota
    INIT_STARTPOD
    INIT_GETPOD
    INIT_RMPOD
    INIT_NEWCONTAINER
    INIT_EXECCMD
    INIT_SHUTDOWN
    INIT_READY
    INIT_ACK
    INIT_ERROR
)

const (
    QMP_EVENT_SHUTDOWN = "SHUTDOWN"
)

const (
    PREPARING_CONTAINER = iota
    PREPARING_VOLUME
    PREPARING_BLOCK
    PREPARING_BIND_DIR
)

func EventString(ev int) string {
    switch ev {
        case EVENT_QEMU_EXIT: return "EVENT_QEMU_EXIT"
        case EVENT_QEMU_TIMEOUT: return "EVENT_QEMU_TIMEOUT"
        case EVENT_INIT_CONNECTED: return "EVENT_INIT_CONNECTED"
        case EVENT_QMP_EVENT: return "EVENT_QMP_EVENT"
        case EVENT_CONTAINER_ADD: return "EVENT_CONTAINER_ADD"
        case EVENT_CONTAINER_DELETE: return "EVENT_CONTAINER_DELETE"
        case EVENT_VOLUME_ADD: return "EVENT_VOLUME_ADD"
        case EVENT_VOLUME_DELETE: return "EVENT_VOLUME_DELETE"
        case EVENT_PATH_BOUND: return "EVENT_PATH_BOUND"
        case EVENT_PATH_UNBOUND: return "EVENT_PATH_UNBOUND"
        case EVENT_BLOCK_INSERTED: return "EVENT_BLOCK_INSERTED"
        case EVENT_BLOCK_EJECTED: return "EVENT_BLOCK_EJECTED"
        case EVENT_INTERFACE_ADD: return "EVENT_INTERFACE_ADD"
        case EVENT_INTERFACE_DELETE: return "EVENT_INTERFACE_DELETE"
        case EVENT_INTERFACE_INSERTED: return "EVENT_INTERFACE_INSERTED"
        case EVENT_INTERFACE_EJECTED: return "EVENT_INTERFACE_EJECTED"
        case COMMAND_RUN_POD: return "COMMAND_RUN_POD"
        case COMMAND_SHUTDOWN: return "COMMAND_SHUTDOWN"
        case COMMAND_EXEC: return "COMMAND_EXEC"
        case COMMAND_ACK: return "COMMAND_ACK"
        case ERROR_INIT_FAIL: return "ERROR_INIT_FAIL"
    }
    return "UNKNOWN"
}