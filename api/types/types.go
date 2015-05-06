package types

const (
    E_OK = iota
    E_SHUTDOWM
    E_EXEC_FINISH
    E_BUSY
    E_NO_TTY
    E_JSON_PARSE_FAIL
    E_CONTEXT_INIT_FAIL
    E_DEVICE_FAIL
    E_INIT_FAIL
    E_COMMAND_TIMEOUT
    E_QMP_COMMAND_FAIL
)

// status for POD or container
const (
    S_ONLINE = iota
    S_STOP = iota
)

type QemuResponse struct {
    VmId string
    Code int
    Cause string
    Data interface{}
}
