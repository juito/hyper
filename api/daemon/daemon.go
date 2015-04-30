package daemon

import (
	"fmt"
	"os"
	"runtime"
	apiserver "dvm/api/server"
	"dvm/engine"
	"dvm/lib/portallocator"
	"dvm/api/docker"
	"dvm/api/network"
	"dvm/lib/glog"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/util"
)

type Container struct {
	Name            string
	PodId           string
	Image           string
	Cmds            []string
	Status          string
}

type Daemon struct {
	ID               string
	db				 *leveldb.DB
	eng              *engine.Engine
	dockerCli		 *docker.DockerCli
	containerList    map[string]*Container
	qemuChan         map[string]interface{}
	qemuClientChan   map[string]interface{}
}

// Install installs daemon capabilities to eng.
func (daemon *Daemon) Install(eng *engine.Engine) error {
	// Now, we just install a command 'info' to set/get the information of the docker and DVM daemon
	for name, method := range map[string]engine.Handler{
		"info":              daemon.CmdInfo,
		"create":			 daemon.CmdCreate,
		"pull":				 daemon.CmdPull,
		"pod":				 daemon.CmdPod,
		"podInfo":			 daemon.CmdPodInfo,
		"list":              daemon.CmdList,
		"stop":              daemon.CmdStop,
		"exec":              daemon.CmdExec,
		"tty":               daemon.CmdTty,
		"serveapi":			 apiserver.ServeApi,
		"acceptconnections": apiserver.AcceptConnections,
	} {
		glog.V(3).Infof("Engine Register: name= %s\n", name)
		if err := eng.Register(name, method); err != nil {
			return err
		}
	}
	return nil
}

func NewDaemon(eng *engine.Engine) (*Daemon, error) {
	daemon, err := NewDaemonFromDirectory(eng)
	if err != nil {
		return nil, err
	}
	return daemon, nil
}

func NewDaemonFromDirectory(eng *engine.Engine) (*Daemon, error) {
	// register portallocator release on shutdown
	eng.OnShutdown(func() {
		if err := portallocator.ReleaseAll(); err != nil {
			glog.Errorf("portallocator.ReleaseAll(): %s", err.Error())
		}
	})
	// Check that the system is supported and we have sufficient privileges
	if runtime.GOOS != "linux" {
		return nil, fmt.Errorf("The Docker daemon is only supported on linux")
	}
	if os.Geteuid() != 0 {
		return nil, fmt.Errorf("The Docker daemon needs to be run as root")
	}
	if err := checkKernel(); err != nil {
		return nil, err
	}

	var tempdir = "/var/run/dvm/"
	os.Setenv("TMPDIR", tempdir)
	if err := os.MkdirAll(tempdir, 0755); err != nil && !os.IsExist(err) {
		return nil, err
	}

	var realRoot = "/var/run/dvm/"
	// Create the root directory if it doesn't exists
	if err := os.MkdirAll(realRoot, 0755); err != nil && !os.IsExist(err) {
		return nil, err
	}

	if err := network.InitNetwork("", "192.168.123.1/24"); err != nil {
		glog.Errorf("InitNetwork failed, %s\n", err.Error())
		return nil, err
	}

	var (
		proto = "unix"
		addr = "/var/run/docker.sock"
		db_file = fmt.Sprintf("%s/%d.db", realRoot, os.Getpid())
	)
	db, err := leveldb.OpenFile(db_file, nil)
	if err != nil {
		glog.Errorf("open leveldb file failed, %s\n", err.Error())
		return nil, err
	}
	dockerCli := docker.NewDockerCli("", proto, addr, nil)
	qemuchan := make(map[string]interface{}, 100)
	qemuclient := make(map[string]interface{}, 100)
	cList := make(map[string]*Container, 100)
	daemon := &Daemon{
		ID:               string(os.Getpid()),
		db:               db,
		eng:              eng,
		dockerCli:		  dockerCli,
		containerList:    cList,
		qemuChan:         qemuchan,
		qemuClientChan:   qemuclient,
	}

	eng.OnShutdown(func() {
		if err := daemon.shutdown(); err != nil {
			glog.Errorf("Error during daemon.shutdown(): %v", err)
		}
	})

	return daemon, nil
}

func (daemon *Daemon) GetPodNum() int64 {
	iter := (daemon.db).NewIterator(util.BytesPrefix([]byte("pod-vm-")), nil)
	var i int64 = 0
	for iter.Next() {
		i = i + 1
	}
	return i
}

func (daemon *Daemon) WritePodToDB(podName string, podData []byte) error {
	key := fmt.Sprintf("pod-%s", podName)
	err := (daemon.db).Put([]byte(key), podData, nil)
	if err != nil {
		return err
	}
	return nil
}

func (daemon *Daemon) GetPodByName(podName string) ([]byte, error) {
	key := fmt.Sprintf("pod-%s", podName)
	data, err := (daemon.db).Get([]byte(key), nil)
	if err != nil {
		return []byte(""), err
	}
	return data, nil
}

func (daemon *Daemon) DeletePodFromDB(podName string) error {
	key := fmt.Sprintf("pod-%s", podName)
	err := (daemon.db).Delete([]byte(key), nil)
	if err != nil {
		return err
	}
	return nil
}

func (daemon *Daemon) WritePodAndVM(podName, vmid string) error {
	key := fmt.Sprintf("pod-vm-%s", podName)
	err := (daemon.db).Put([]byte(key), []byte(vmid), nil)
	if err != nil {
		return err
	}
	return nil
}

func (daemon *Daemon) GetPodVmByName(podName string) ([]byte, error) {
	key := fmt.Sprintf("pod-vm-%s", podName)
	vmid, err := (daemon.db).Get([]byte(key), nil)
	if err != nil {
		return []byte(""), err
	}
	return vmid, nil
}

func (daemon *Daemon) DeletePodVmFromDB (podName string) error {
	key := fmt.Sprintf("pod-vm-%s", podName)
	err := (daemon.db).Delete([]byte(key), nil)
	if err != nil {
		return err
	}
	return nil
}

func (daemon *Daemon) GetQemuChan(vmid string) (interface{}, interface{}, error) {
	if daemon.qemuChan[vmid] != nil && daemon.qemuClientChan[vmid] != nil {
		return daemon.qemuChan[vmid], daemon.qemuClientChan[vmid], nil
	}
	return nil, nil, fmt.Errorf("Can not find the Qemu chan for pod: %s!", vmid)
}

func (daemon *Daemon) DeleteQemuChan(vmid string) error {
	if daemon.qemuChan[vmid] != nil {
		delete(daemon.qemuChan, vmid)
	}
	if daemon.qemuClientChan[vmid] != nil {
		delete(daemon.qemuClientChan, vmid)
	}

	return nil
}

func (daemon *Daemon) SetQemuChan(vmid string, qemuchan, qemuclient interface{}) error {
	if daemon.qemuChan[vmid] == nil {
		if qemuchan != nil {
			daemon.qemuChan[vmid] = qemuchan
		}
		if qemuclient != nil {
			daemon.qemuClientChan[vmid] = qemuclient
		}
		return nil
	}
	return fmt.Errorf("Already find a Qemu chan for vm: %s!", vmid)
}

func (daemon *Daemon) SetPodByContainer(containerId, podId, name, image string, cmds []string) error {
	container := &Container {
		Name:             name,
		PodId:            podId,
		Image:            image,
		Cmds:             cmds,
	}
	daemon.containerList[containerId] = container

	return nil
}

func (daemon *Daemon) GetPodByContainer(containerId string) (string, error) {
	container := daemon.containerList[containerId]
	if container == nil {
		return "", fmt.Errorf("Can not find that container!")
	}

	return container.PodId, nil
}

func (daemon *Daemon) shutdown() error {
	glog.V(0).Info("The daemon will be shutdown\n")
	(daemon.db).Close()
	glog.Flush()
	return nil
}

// Now, the daemon can be ran for any linux kernel
func checkKernel() error {
	return nil
}
