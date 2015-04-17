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
    COMMAND_ACK
)

const(
    QMP_SESSION = iota
    QMP_RESULT
    QMP_ERROR
    QMP_FINISH
    QMP_EVENT
    QMP_INTERNAL_ERROR
    QMP_QUIT
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