package client

import (
	"crypto/tls"
	"encoding/json"
	"io"
	"fmt"
	"net"
	"net/http"
	"os"
	"reflect"
	"strings"
	"text/template"
	"time"
	"dvm/lib/term"
)

type DvmClient struct {
	proto      string
	addr       string
	scheme	   string
	in         io.ReadCloser
	out        io.Writer
	err        io.Writer
	inFd uintptr
	outFd uintptr
	isTerminalIn bool
	isTerminalOut bool
	tlsConfig	*tls.Config
	transport     *http.Transport
}

var funcMap = template.FuncMap{
	"json": func(v interface{}) string {
		a, _ := json.Marshal(v)
		return string(a)
	},
}

func (cli *DvmClient) getMethod(args ...string) (func(...string) error, bool) {
	camelArgs := make([]string, len(args))
	for i, s := range args {
		if len(s) == 0 {
			return nil, false
		}
		camelArgs[i] = strings.ToUpper(s[:1]) + strings.ToLower(s[1:])
	}
	methodName := "DvmCmd" + strings.Join(camelArgs, "")
	method := reflect.ValueOf(cli).MethodByName(methodName)
	if !method.IsValid() {
		return nil, false
	}
	return method.Interface().(func(...string) error), true
}

// Cmd executes the specified command.
func (cli *DvmClient) Cmd(args ...string) error {
	if len(args) > 1 {
		method, exists := cli.getMethod(args[:2]...)
		if exists {
			return method(args[2:]...)
		}
	}
	if len(args) > 0 {
		method, exists := cli.getMethod(args[0])
		if !exists {
			fmt.Printf("DVM: '%s' is not a docker command. See 'docker --help'.\n", args[0])
			os.Exit(1)
		}
		return method(args[1:]...)
	}
	return cli.DvmCmdHelp()
}

func NewDvmClient(proto, addr string, tlsConfig *tls.Config) *DvmClient {
	var (
		inFd          uintptr
		outFd         uintptr
		isTerminalIn  = false
		isTerminalOut = false
		scheme        = "http"
	)

	if tlsConfig != nil {
		scheme = "https"
	}

	// The transport is created here for reuse during the client session
	tran := &http.Transport{
		TLSClientConfig: tlsConfig,
	}

	// Why 32? See issue 8035
	timeout := 32 * time.Second
	if proto == "unix" {
		// no need in compressing for local communications
		tran.DisableCompression = true
		tran.Dial = func(_, _ string) (net.Conn, error) {
			return net.DialTimeout(proto, addr, timeout)
		}
	} else {
		tran.Proxy = http.ProxyFromEnvironment
		tran.Dial = (&net.Dialer{Timeout: timeout}).Dial
	}

	inFd, isTerminalIn = term.GetFdInfo(os.Stdin)
	outFd, isTerminalOut = term.GetFdInfo(os.Stdout)

	return &DvmClient {
		proto:         proto,
		addr:          addr,
		in:            os.Stdin,
		out:           os.Stdout,
		err:           os.Stdout,
		inFd:          inFd,
		outFd:         outFd,
		isTerminalIn:  isTerminalIn,
		isTerminalOut: isTerminalOut,
		scheme:        scheme,
		transport:     tran,
	}
}
