package types

const (
    E_OK = iota
    E_INIT_FAIL
)

type QemuResponse struct {
    VmId string
    Code int
    Cause string
}
