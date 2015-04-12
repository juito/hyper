package qemu

const (
    BaseDir     = "/var/run/dvm"
    DvmSockName = "dvm.sock"
    QmpSockName = "qmp.sock"
    ShareDir    = "share_dir"
    Kernel      = "vmlinuz"
    Initrd      = "initrd.img"

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
    COMMAND_RUN_POD

    QMP_COMMAND = iota
    QMP_RESULT
    QMP_ERROR
    QMP_EVENT
    QMP_INTERNAL_ERROR
    QMP_QUIT

    QMP_EVENT_SHUTDOWN = "SHUTDOWN"

    PREPARING_CONTAINER = iota
    PREPARING_VOLUME
    PREPARING_BLOCK
    PREPARING_BIND_DIR
)