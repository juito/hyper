package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	"dvm/api"
	"dvm/dvmversion"
)

var (
	ErrConnectionRefused = errors.New("Cannot connect to the Docker daemon. Is 'docker -d' running on this host?")
)

func (cli *DvmClient) HTTPClient() *http.Client {
	return &http.Client{Transport: cli.transport}
}

func (cli *DvmClient) encodeData(data interface{}) (*bytes.Buffer, error) {
	params := bytes.NewBuffer(nil)
	if data != nil {
		buf, err := json.Marshal(data)
		if err != nil {
			return nil, err
		}
		if _, err := params.Write(buf); err != nil {
			return nil, err
		}
	}
	return params, nil
}

func (cli *DvmClient) clientRequest(method, path string, in io.Reader, headers map[string][]string) (io.ReadCloser, string, int, error) {
	expectedPayload := (method == "POST" || method == "PUT")
	if expectedPayload && in == nil {
		in = bytes.NewReader([]byte{})
	}
	req, err := http.NewRequest(method, fmt.Sprintf("/v%s%s", dvmversion.VERSION, path), in)
	if err != nil {
		return nil, "", -1, err
	}
	req.Header.Set("User-Agent", "Dvm-Client/"+dvmversion.VERSION)
	req.URL.Host = cli.addr
	req.URL.Scheme = cli.scheme

	if headers != nil {
		for k, v := range headers {
			req.Header[k] = v
		}
	}

	if expectedPayload && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "text/plain")
	}

	resp, err := cli.HTTPClient().Do(req)
	statusCode := -1
	if resp != nil {
		statusCode = resp.StatusCode
	}
	if err != nil {
		if strings.Contains(err.Error(), "connection refused") {
			return nil, "", statusCode, ErrConnectionRefused
		}

/*
		if cli.tlsConfig == nil {
			return nil, "", statusCode, fmt.Errorf("%v. Are you trying to connect to a TLS-enabled daemon without TLS?", err)
		}
*/
		return nil, "", statusCode, fmt.Errorf("An error occurred trying to connect: %v", err)
	}

	if statusCode < 200 || statusCode >= 400 {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, "", statusCode, err
		}
		if len(body) == 0 {
			return nil, "", statusCode, fmt.Errorf("Error: request returned %s for API route and version %s, check if the server supports the requested API version", http.StatusText(statusCode), req.URL)
		}
		return nil, "", statusCode, fmt.Errorf("Error response from daemon: %s", bytes.TrimSpace(body))
	}

	return resp.Body, resp.Header.Get("Content-Type"), statusCode, nil
}

func (cli *DvmClient) call(method, path string, data interface{}, headers map[string][]string) (io.ReadCloser, int, error) {
	params, err := cli.encodeData(data)
	if err != nil {
		return nil, -1, err
	}

	if data != nil {
		if headers == nil {
			headers = make(map[string][]string)
		}
		headers["Content-Type"] = []string{"application/json"}
	}

	body, _, statusCode, err := cli.clientRequest(method, path, params, headers)
	return body, statusCode, err
}

func (cli *DvmClient) stream(method, path string, in io.Reader, out io.Writer, headers map[string][]string) error {
	return cli.streamHelper(method, path, true, in, out, nil, headers)
}

func (cli *DvmClient) streamHelper(method, path string, setRawTerminal bool, in io.Reader, stdout, stderr io.Writer, headers map[string][]string) error {
	body, contentType, _, err := cli.clientRequest(method, path, in, headers)
	if err != nil {
		return err
	}
	return cli.streamBody(body, contentType, setRawTerminal, stdout, stderr)
}

func (cli *DvmClient) streamBody(body io.ReadCloser, contentType string, setRawTerminal bool, stdout, stderr io.Writer) error {
	defer body.Close()

	if api.MatchesContentType(contentType, "application/json") {
		for {
			buf := new(bytes.Buffer)
			buf.ReadFrom(body)
			str := buf.String()
			fmt.Printf(str)
		}
		return nil
	}
	fmt.Printf("There is something wrong in print the IO Stream!\n")
	return nil
}

func readBody(stream io.ReadCloser, statusCode int, err error) ([]byte, int, error) {
	if stream != nil {
		defer stream.Close()
	}
	if err != nil {
		return nil, statusCode, err
	}
	body, err := ioutil.ReadAll(stream)
	if err != nil {
		return nil, -1, err
	}
	return body, statusCode, nil
}
