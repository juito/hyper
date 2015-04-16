package types

const (
    E_OK = iota
    E_CONTEXT_INIT_FAIL
    E_DEVICE_FAIL
    E_QMP_INIT_FAIL
    E_QMP_COMMAND_FAIL
)

type QemuResponse struct {
    VmId string
    Code int
    Cause string
}
