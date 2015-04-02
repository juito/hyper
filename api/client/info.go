package client

import (
	"fmt"
	"io"
	"sync"
	"encoding/json"
	"strconv"
	"strings"
)

type Env []string
func (env *Env) Map() map[string]string {
	m := make(map[string]string)
	for _, kv := range *env {
		parts := strings.SplitN(kv, "=", 2)
		m[parts[0]] = parts[1]
	}
	return m
}
func (env *Env) Set(key, value string) {
	*env = append(*env, key+"="+value)
}
func (env *Env) SetInt(key string, value int) {
	env.Set(key, fmt.Sprintf("%d", value))
}
func (env *Env) GetInt(key string) int {
	return int(env.GetInt64(key))
}

func (env *Env) GetInt64(key string) int64 {
	s := strings.Trim(env.Get(key), " \t")
	val, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return val
}

func (env *Env) SetInt64(key string, value int64) {
	env.Set(key, fmt.Sprintf("%d", value))
}

type Output struct {
	sync.Mutex
	dests []io.Writer
	tasks sync.WaitGroup
	used  bool
}

// NewOutput returns a new Output object with no destinations attached.
// Writing to an empty Output will cause the written data to be discarded.
func NewOutput() *Output {
	return &Output{}
}

func NewDecoder(src io.Reader) *Decoder {
	return &Decoder{
		json.NewDecoder(src),
	}
}

type Decoder struct {
	*json.Decoder
}

// Close unregisters all destinations and waits for all background
// AddTail and AddString tasks to complete.
// The Close method of each destination is called if it exists.
func (o *Output) Close() error {
	o.Lock()
	defer o.Unlock()
	var firstErr error
	for _, dst := range o.dests {
		if closer, ok := dst.(io.Closer); ok {
			err := closer.Close()
			if err != nil && firstErr == nil {
				firstErr = err
			}
		}
	}
	o.tasks.Wait()
	o.dests = nil
	return firstErr
}
// Add attaches a new destination to the Output. Any data subsequently written
// to the output will be written to the new destination in addition to all the others.
// This method is thread-safe.
func (o *Output) Add(dst io.Writer) {
	o.Lock()
	defer o.Unlock()
	o.dests = append(o.dests, dst)
}

// Set closes and remove existing destination and then attaches a new destination to
// the Output. Any data subsequently written to the output will be written to the new
// destination in addition to all the others. This method is thread-safe.
func (o *Output) Set(dst io.Writer) {
	o.Close()
	o.Lock()
	defer o.Unlock()
	o.dests = []io.Writer{dst}
}

// AddPipe creates an in-memory pipe with io.Pipe(), adds its writing end as a destination,
// and returns its reading end for consumption by the caller.
// This is a rough equivalent similar to Cmd.StdoutPipe() in the standard os/exec package.
// This method is thread-safe.
func (o *Output) AddPipe() (io.Reader, error) {
	r, w := io.Pipe()
	o.Add(w)
	return r, nil
}
// AddEnv starts a new goroutine which will decode all subsequent data
// as a stream of json-encoded objects, and point `dst` to the last
// decoded object.
// The result `env` can be queried using the type-neutral Env interface.
// It is not safe to query `env` until the Output is closed.
func (o *Output) AddEnv() (dst *Env, err error) {
	src, err := o.AddPipe()
	if err != nil {
		return nil, err
	}
	dst = &Env{}
	o.tasks.Add(1)
	go func() {
		defer o.tasks.Done()
		decoder := NewDecoder(src)
		for {
			env, err := decoder.Decode()
			if err != nil {
				return
			}
			*dst = *env
		}
	}()
	return dst, nil
}

// Write writes the same data to all registered destinations.
// This method is thread-safe.
func (o *Output) Write(p []byte) (n int, err error) {
	o.Lock()
	defer o.Unlock()
	o.used = true
	var firstErr error
	for _, dst := range o.dests {
		_, err := dst.Write(p)
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return len(p), firstErr
}

func (decoder *Decoder) Decode() (*Env, error) {
	m := make(map[string]interface{})
	if err := decoder.Decoder.Decode(&m); err != nil {
		return nil, err
	}
	env := &Env{}
	for key, value := range m {
		env.SetAuto(key, value)
	}
	return env, nil
}
 
// Get returns the last value associated with the given key. If there are no
// values associated with the key, Get returns the empty string.
func (env *Env) Get(key string) (value string) {
	// not using Map() because of the extra allocations https://github.com/docker/docker/pull/7488#issuecomment-51638315
	for _, kv := range *env {
		if strings.Index(kv, "=") == -1 {
			continue
		}
		parts := strings.SplitN(kv, "=", 2)
		if parts[0] != key {
			continue
		}
		if len(parts) < 2 {
			value = ""
		} else {
			value = parts[1]
		}
	}
	return
}

func (env *Env) Exists(key string) bool {
	_, exists := env.Map()[key]
	return exists
}

// Len returns the number of keys in the environment.
// Note that len(env) might be different from env.Len(),
// because the same key might be set multiple times.
func (env *Env) Len() int {
	return len(env.Map())
}

func (env *Env) Init(src *Env) {
	(*env) = make([]string, 0, len(*src))
	for _, val := range *src {
		(*env) = append((*env), val)
	}
}
// DecodeEnv decodes `src` as a json dictionary, and adds
// each decoded key-value pair to the environment.
//
// If `src` cannot be decoded as a json dictionary, an error
// is returned.
func (env *Env) Decode(src io.Reader) error {
	m := make(map[string]interface{})
	d := json.NewDecoder(src)
	// We need this or we'll lose data when we decode int64 in json
	d.UseNumber()
	if err := d.Decode(&m); err != nil {
		return err
	}
	for k, v := range m {
		env.SetAuto(k, v)
	}
	return nil
}

func (env *Env) SetAuto(k string, v interface{}) {
	// Issue 7941 - if the value in the incoming JSON is null then treat it
	// as if they never specified the property at all.
	if v == nil {
		return
	}

	// FIXME: we fix-convert float values to int, because
	// encoding/json decodes integers to float64, but cannot encode them back.
	// (See http://golang.org/src/pkg/encoding/json/decode.go#L46)
	if fval, ok := v.(float64); ok {
		env.SetInt64(k, int64(fval))
	} else if sval, ok := v.(string); ok {
		env.Set(k, sval)
	} else if val, err := json.Marshal(v); err == nil {
		env.Set(k, string(val))
	} else {
		env.Set(k, fmt.Sprintf("%v", v))
	}
}
// we need this *info* function to get the whole status from the docker daemon
func (cli *DvmClient) DvmCmdInfo(args ...string) error {
	body, _, err := readBody(cli.call("GET", "/info", nil, nil))
	if err != nil {
		fmt.Printf("The Error is encountered, %s\n", err)
		return err
	}

	out := NewOutput()
	remoteInfo, err := out.AddEnv()
	if err != nil {
		return err
	}

	if _, err := out.Write(body); err != nil {
		fmt.Printf("Error reading remote info: %s", err)
		return err
	}
	out.Close()
	if remoteInfo.Exists("Containers") {
		fmt.Printf("Containers: %d\n", remoteInfo.GetInt("Containers"))
	}

	return nil
}
