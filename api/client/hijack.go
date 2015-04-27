package client

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"

	"dvm/api"
	"dvm/lib/terminal"
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
		term *terminal.Terminal
	)

	if in != nil && setRawTerminal {
		term, err = terminal.NewWithStdInOut()
		if err != nil {
			return err
		}
		defer term.ReleaseFromStdInOut()
		term.SetPrompt("root@helloworld: # ")
	}

	if stdout != nil || stderr != nil {
		receiveStdout = promise.Go(func() (err error) {
			defer func() {
				if in != nil {
					if setRawTerminal {
						term.ReleaseFromStdInOut()
					}
				}
			}()

			for {
				line := make([]byte, 1024)
				_, err := br.Read(line)
				if err == io.EOF {
					break
				}
				term.Write(line)
			}
			fmt.Printf("[hijack] End of stdout\n")
			return err
		})
	}

	sendStdin := promise.Go(func() error {
		if in != nil {
			line , err := term.ReadLine()
			for {
				if err == io.EOF {
					term.Write([]byte(line))
					fmt.Println()
					break
				}
				if (err != nil && strings.Contains(err.Error(), "control-c break")) || len(line) == 0{
					line, err = term.ReadLine()
				} else {
					//term.Write([]byte(line+"\r\n"))
					io.Copy(rwc, strings.NewReader(line+"\r\n"))
					line, err = term.ReadLine()
				}
			}
			io.Copy(rwc, strings.NewReader(line))
			fmt.Printf("[hijack] End of stdin\n")
		}

		if conn, ok := rwc.(interface {
			CloseWrite() error
		}); ok {
			if err := conn.CloseWrite(); err != nil {
				fmt.Printf("Couldn't send EOF: %s", err.Error())
			}
		}
		if conn, ok := rwc.(interface {
			CloseRead() error
		}); ok {
			if err := conn.CloseRead(); err != nil {
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
	if in != nil {
		if err := <-sendStdin ; err != nil {
			fmt.Printf("Error sendStdin: %s\n", err.Error())
			return err
		}
	}

	return nil
}
