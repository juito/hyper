package daemon

import (
	"fmt"
	"syscall"
	"os/exec"
	"path"
	"path/filepath"
	"os"
	"io/ioutil"
	"encoding/json"
	"strings"
	"dvm/engine"
)

type jsonMetadata struct {
	Device_id int      `json:"device_id"`
	Size      int      `json:"size"`
	Transaction_id int `json:"transaction_id"`
	Initialized bool   `json:"initialized"`
}

func (daemon *Daemon) CmdCreate(job *engine.Job) error {
	cli := daemon.dockerCli
	body, _, err := cli.SendCmdCreate("tomcat:latest")
	if err != nil {
		return err
	}
	out := engine.NewOutput()
	remoteInfo, err := out.AddEnv()
	if err != nil {
		return err
	}
	if _, err := out.Write(body); err != nil {
		return fmt.Errorf("Error while reading remote info!\n")
	}
	out.Close()

	v := &engine.Env{}
	v.SetJson("ID", daemon.ID)
	if remoteInfo.Exists("Id") {
		v.Set("ContainerID", remoteInfo.Get("Id"))
		fmt.Printf("The ContainerID is %s\n", remoteInfo.Get("Id"))
		containerId := remoteInfo.Get("Id")
		err := attachFiles(containerId, "/home/lei/src/github.com/gorilla/context/README.md", "/usr/local/man/")
		if err != nil {
			return err
		}
	} else {
		return fmt.Errorf("AN error encountered during creating container!\n")
	}

	if _, err := v.WriteTo(job.Stdout); err != nil {
		return err
	}

	return nil
}

func attachFiles(containerId, fromFile, toDir string) error {
	if containerId == "" {
		return fmt.Errorf("Please make sure the arguments are not NULL!\n")
	}
	// FIXME: Define the basic directory, need to get them via the 'info' command
	var (
		rootPath = "/var/lib/docker/devicemapper"
		metadataPath = fmt.Sprintf("%s/metadata/", rootPath)
		mntPath = fmt.Sprintf("%s/mnt/", rootPath)
		deviceId int
		deviceSize int
	)

	// Get device id from the metadata file
	idMetadataFile := path.Join(metadataPath, containerId)
	if _, err := os.Stat(idMetadataFile); err != nil && os.IsNotExist(err) {
		return err
	}
	jsonData, err := ioutil.ReadFile(idMetadataFile)
	if err != nil {
		return err
	}
	var dat jsonMetadata
	if err := json.Unmarshal(jsonData, &dat); err != nil {
		return err
	}
	deviceId = dat.Device_id
	deviceSize = dat.Size

	// Get the mount point for the container ID
	idMountPath := path.Join(mntPath, containerId)
	rootFs := path.Join(idMountPath, "rootfs")
	targetDir := path.Join(rootFs, toDir)

	// Whether we have the mounter directory
	if _, err := os.Stat(idMountPath); err != nil && os.IsNotExist(err) {
		return err
	}

	// Activate the device for that device ID
	// FIXME: this prefix of a device name shoule be got from the docker daemon
	devName := fmt.Sprintf("docker-8:1-268188-%s", containerId)
	poolName := "/dev/mapper/docker-8:1-268188-pool"
	createDeviceCmd := fmt.Sprintf("dmsetup create %s --table \"0 %d thin %s %d\"", devName, deviceSize/512, poolName, deviceId)
	createDeviceCommand := exec.Command("/bin/sh", "-c", createDeviceCmd)
	_, err = createDeviceCommand.Output()
	if err != nil {
		fmt.Printf("Error while creating a new block device!\n")
		return err
	}

	// Mount the block device to that mount point
	var flags uintptr = syscall.MS_MGC_VAL
	devFullName := fmt.Sprintf("/dev/mapper/%s", devName)
	fstype, err := ProbeFsType(devFullName)
	if err != nil {
		return err
	}
	fmt.Printf("The filesytem type is %s\n", fstype)
	options := ""
	if fstype == "xfs" {
		// XFS needs nouuid or it can't mount filesystems with the same fs
		options = joinMountOptions(options, "nouuid")
	}

	err = syscall.Mount(devFullName, idMountPath, fstype, flags, joinMountOptions("discard", options))
	if err != nil && err == syscall.EINVAL {
		err = syscall.Mount(devName, idMountPath, fstype, flags, options)
	}
	if err != nil {
		return fmt.Errorf("Error mounting '%s' on '%s': %s", devName, idMountPath, err)
	}

	// It just need the block device without copying any files
	if fromFile == "" || toDir == "" {
		// we need to unmout the device from the mounted directory
		syscall.Unmount(devFullName, syscall.MNT_DETACH)
		return nil
	}
	// Make a new file with the given premission and wirte the source file content in it
	if _, err := os.Stat(fromFile); err != nil && os.IsNotExist(err) {
		// The gived file is not exist, we need to unmout the device and return
		syscall.Unmount(devFullName, syscall.MNT_DETACH)
		return nil
	}
	buf, err := ioutil.ReadFile(fromFile)
	if err != nil {
		// unmout the device
		syscall.Unmount(devFullName, syscall.MNT_DETACH)
		return err
	}
	targetInfo, err := os.Stat(targetDir)
	targetFile := targetDir
	if err != nil && os.IsNotExist(err) {
		if targetInfo.IsDir() {
			// we need to create a target directory with given premission
			// FIXME: we need to modify this 0755 with the given argument
			if err := os.MkdirAll(targetDir, 0755); err != nil {
				// we need to unmout the device
				syscall.Unmount(devFullName, syscall.MNT_DETACH)
				return err
			}
			targetFile = path.Join(targetDir, filepath.Base(fromFile))
		} else {
			tmpDir := filepath.Dir(targetDir)
			if _, err := os.Stat(tmpDir); err != nil && os.IsNotExist(err) {
				if err := os.MkdirAll(tmpDir, 0755); err != nil {
					// we need to unmout the device
					syscall.Unmount(devFullName, syscall.MNT_DETACH)
					return err
				}
			}
		}
	} else {
		targetFile = path.Join(targetDir, filepath.Base(fromFile))
	}
	err = ioutil.WriteFile(targetFile, buf, 0755)
	if err != nil {
		// we need to unmout the device
		syscall.Unmount(devFullName, syscall.MNT_DETACH)
		return err
	}
	// finally we need to unmout the device
	syscall.Unmount(devFullName, syscall.MNT_DETACH)
	return nil
}

func ProbeFsType(device string) (string, error) {
	// The daemon will only be run on Linux platform, so 'file -s' command
	// will be used to test the type of filesystem which the device located.
	cmd := fmt.Sprintf("file -s %s", device)
	command := exec.Command("/bin/sh", "-c", cmd)
	fileCmdOutput, err := command.Output()
	if err != nil {
		return "", nil
	}

	if strings.Contains(string(fileCmdOutput), "ext4") {
		return "ext4", nil
	}
	if strings.Contains(string(fileCmdOutput), "xfs") {
		return "xfs", nil
	}

	return "", fmt.Errorf("Unknown filesystem type on %s", device)
}

func joinMountOptions(a, b string) string {
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	return a + "," + b
}
