package devicemapper

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
	"strconv"

	"dvm/lib/glog"
)

type jsonMetadata struct {
	Device_id int      `json:"device_id"`
	Size      int      `json:"size"`
	Transaction_id int `json:"transaction_id"`
	Initialized bool   `json:"initialized"`
}

// For device mapper, we do not need to mount the container to sharedDir.
// All of we need to provide the block device name of container.
func MountContainerToSharedDir(containerId, sharedDir, devPrefix string) (string, error) {
    devFullName := fmt.Sprintf("/dev/mapper/%s-%s", devPrefix, containerId)
	return devFullName, nil
}


func CreateNewDevice(containerId, devPrefix, rootPath string) error {
	var	metadataPath = fmt.Sprintf("%s/metadata/", rootPath)
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
	deviceId := dat.Device_id
	deviceSize := dat.Size
	// Activate the device for that device ID
	devName := fmt.Sprintf("%s-%s", devPrefix, containerId)
	poolName := fmt.Sprintf("/dev/mapper/%s-pool", devPrefix)
	createDeviceCmd := fmt.Sprintf("dmsetup create %s --table \"0 %d thin %s %d\"", devName, deviceSize/512, poolName, deviceId)
	createDeviceCommand := exec.Command("/bin/sh", "-c", createDeviceCmd)
	output, err := createDeviceCommand.Output()
	if err != nil {
		glog.Error(output)
		return err
	}

	return nil
}

func AttachFiles(containerId, devPrefix, fromFile, toDir, rootPath, perm string) error {
	if containerId == "" {
		return fmt.Errorf("Please make sure the arguments are not NULL!\n")
	}
	permInt, err := strconv.Atoi(perm)
	if err != nil {
		return err
	}
	// Define the basic directory, need to get them via the 'info' command
	var (

		mntPath = fmt.Sprintf("%s/mnt/", rootPath)
		devName = fmt.Sprintf("%s-%s", devPrefix, containerId)
	)

	// Get the mount point for the container ID
	idMountPath := path.Join(mntPath, containerId)
	rootFs := path.Join(idMountPath, "rootfs")
	targetDir := path.Join(rootFs, toDir)

	// Whether we have the mounter directory
	if _, err := os.Stat(idMountPath); err != nil && os.IsNotExist(err) {
		return err
	}

	// Mount the block device to that mount point
	var flags uintptr = syscall.MS_MGC_VAL
	devFullName := fmt.Sprintf("/dev/mapper/%s", devName)
	fstype, err := ProbeFsType(devFullName)
	if err != nil {
		return err
	}
	glog.V(3).Infof("The filesytem type is %s\n", fstype)
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
		syscall.Unmount(idMountPath, syscall.MNT_DETACH)
		return nil
	}
	// Make a new file with the given premission and wirte the source file content in it
	if _, err := os.Stat(fromFile); err != nil && os.IsNotExist(err) {
		// The given file is not exist, we need to unmout the device and return
		syscall.Unmount(idMountPath, syscall.MNT_DETACH)
		return err
	}
	buf, err := ioutil.ReadFile(fromFile)
	if err != nil {
		// unmout the device
		syscall.Unmount(idMountPath, syscall.MNT_DETACH)
		return err
	}
	targetInfo, err := os.Stat(targetDir)
	targetFile := targetDir
	if err != nil && os.IsNotExist(err) {
		if targetInfo.IsDir() {
			// we need to create a target directory with given premission
			if err := os.MkdirAll(targetDir, os.FileMode(permInt)); err != nil {
				// we need to unmout the device
				syscall.Unmount(idMountPath, syscall.MNT_DETACH)
				return err
			}
			targetFile = path.Join(targetDir, filepath.Base(fromFile))
		} else {
			tmpDir := filepath.Dir(targetDir)
			if _, err := os.Stat(tmpDir); err != nil && os.IsNotExist(err) {
				if err := os.MkdirAll(tmpDir, os.FileMode(permInt)); err != nil {
					// we need to unmout the device
					syscall.Unmount(idMountPath, syscall.MNT_DETACH)
					return err
				}
			}
		}
	} else {
		targetFile = path.Join(targetDir, filepath.Base(fromFile))
	}
	err = ioutil.WriteFile(targetFile, buf, os.FileMode(permInt))
	if err != nil {
		// we need to unmout the device
		syscall.Unmount(idMountPath, syscall.MNT_DETACH)
		return err
	}
	// finally we need to unmout the device
	syscall.Unmount(idMountPath, syscall.MNT_DETACH)
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
	if strings.Contains(string(fileCmdOutput), "ext2") {
		return "ext2", nil
	}
	if strings.Contains(string(fileCmdOutput), "ext3") {
		return "ext3", nil
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
