package types

const (
    E_OK = iota
    E_SHUTDOWM
    E_BUSY
    E_JSON_PARSE_FAIL
    E_CONTEXT_INIT_FAIL
    E_DEVICE_FAIL
    E_INIT_FAIL
    E_COMMAND_TIMEOUT
    E_QMP_COMMAND_FAIL
)

type QemuResponse struct {
    VmId string
    Code int
    Cause string
}
