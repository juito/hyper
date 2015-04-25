package client

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"net"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"
	"bytes"

	"dvm/api"
	"dvm/lib/term"
	"dvm/lib/promise"
	"dvm/dvmversion"
)

func (cli *DvmClient) dial() (net.Conn, error) {
	return net.Dial(cli.proto, cli.addr)
}

func (cli *DvmClient) hijack(method, path string, setRawTerminal bool, in io.ReadCloser, stdout, stderr io.Writer, started chan io.Closer, data interface{}) error {
	defer func() {
		if started != nil {
			close(started)
		}
	}()

	params, err := cli.encodeData(data)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(method, fmt.Sprintf("/v%s%s", api.APIVERSION, path), params)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Dvm-Client/"+dvmversion.VERSION)
	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "tcp")
	req.Host = cli.addr

	dial, err := cli.dial()
	if err != nil {
		return err
	}
	// When we set up a TCP connection for hijack, there could be long periods
	// of inactivity (a long running command with no output) that in certain
	// network setups may cause ECONNTIMEOUT, leaving the client in an unknown
	// state. Setting TCP KeepAlive on the socket connection will prohibit
	// ECONNTIMEOUT unless the socket connection truly is broken
	if tcpConn, ok := dial.(*net.TCPConn); ok {
		tcpConn.SetKeepAlive(true)
		tcpConn.SetKeepAlivePeriod(30 * time.Second)
	}
	if err != nil {
		if strings.Contains(err.Error(), "connection refused") {
			return fmt.Errorf("Cannot connect to the Docker daemon. Is 'dameon' running on this host?")
		}
		return err
	}
	clientconn := httputil.NewClientConn(dial, nil)
	defer clientconn.Close()

	// Server hijacks the connection, error 'connection closed' expected
	_, err = clientconn.Do(req)
	if err != nil {
		fmt.Printf("Client DO: %s\n", err.Error())
	}

	rwc, br := clientconn.Hijack()
	defer rwc.Close()

	if started != nil {
		started <- rwc
	}

	var (
		receiveStdout chan error
		oldState *term.State
		stop = 0
	)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	go func() {
		<-sigChan
		stop = 1
		io.Copy(rwc, bytes.NewReader([]byte("exit")))
		os.Exit(0)
	}()

	if in != nil && setRawTerminal {
		oldState, err = term.SetRawTerminal(cli.inFd)
		if err != nil {
			return err
		}
		defer term.RestoreTerminal(cli.inFd, oldState)
	}

	if stdout != nil || stderr != nil {
		receiveStdout = promise.Go(func() (err error) {
			defer func() {
				if in != nil {
					if setRawTerminal {
						term.RestoreTerminal(cli.inFd, oldState)
					}
				}
			}()

			_, err = io.Copy(stdout, br)
			fmt.Printf("[hijack] End of stdout\n")
			return err
		})
	}

	sendStdin := promise.Go(func() error {
		if in != nil {
			io.Copy(rwc, in)
			fmt.Printf("[hijack] End of stdin\n")
		}

		if conn, ok := rwc.(interface {
			CloseWrite() error
		}); ok {
			if err := conn.CloseWrite(); err != nil {
				fmt.Printf("Couldn't send EOF: %s", err.Error())
			}
		}
		// Discard errors due to pipe interruption
		return nil
	})

	if stdout != nil || stderr != nil {
		if err := <-receiveStdout; err != nil {
			fmt.Printf("Error receiveStdout: %s\n", err.Error())
			return err
		}
	}

	if !cli.isTerminalIn {
		if err := <-sendStdin; err != nil {
			fmt.Printf("Error sendStdin: %s\n", err.Error())
			return err
		}
	}
	return nil
}