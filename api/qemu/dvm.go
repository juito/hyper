package qemu

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
    "dvm/api/docker"
    "dvm/api/pod"
    "dvm/engine"
)

type jsonMetadata struct {
	Device_id int      `json:"device_id"`
	Size      int      `json:"size"`
	Transaction_id int `json:"transaction_id"`
	Initialized bool   `json:"initialized"`
}

func CreateContainer(userPod *pod.UserPod, sharedDir string, hub chan QemuEvent) (string, error) {
    var (
        proto = "unix"
        addr = "/var/run/docker.sock"
		fstype string
		poolName string
		devPrefix string
		devFullName string
		storageDriver string
		mountSharedDir string
		containerId string
    )
    var cli = docker.NewDockerCli("", proto, addr, nil)

	// Process the 'Files' section
	files := make(map[string](pod.UserFile))
	for _, v := range userPod.Files {
		files[v.Name] = v
	}

	// Process the 'Containers' section
	for i, c := range userPod.Containers {
		imgName := c.Image
		body, _, err := cli.SendCmdCreate(imgName)
		if err != nil {
			return "", err
		}
		out := engine.NewOutput()
		remoteInfo, err := out.AddEnv()
		if err != nil {
			return "", err
		}
		if _, err := out.Write(body); err != nil {
			return "", fmt.Errorf("Error while reading remote info!\n")
		}
		out.Close()

		containerId := remoteInfo.Get("Id")
		storageDriver := remoteInfo.Get("Driver")
		if storageDriver == "devicemapper" {
			if remoteInfo.Exists("DriverStatus") {
				var driverStatus [][2]string
				if err := remoteInfo.GetJson("DriverStatus", &driverStatus); err != nil {
					return "", err
				}
				for _, pair := range driverStatus {
					if pair[0] == "Pool Name" {
						poolName = pair[1]
						break
					}
					if pair[0] == "Backing Filesystem" {
						if strings.Contains(pair[1], "ext") {
							fstype = "ext4"
						} else if strings.Contains(pair[1], "xfs") {
							fstype = "xfs"
						} else {
							fstype = "dir"
						}
					}
				}
			}
			devPrefix = poolName[:strings.Index(poolName, "-pool")]
		} else if storageDriver == "aufs" {
			// TODO
		}

		if containerId != "" {
			fmt.Printf("The ContainerID is %s\n", containerId)
			var jsonResponse docker.ConfigJSON
			if jsonResponse, err := cli.GetContainerInfo(containerId); err != nil {
				return "", err
			}

			var rootPath = "/var/lib/docker/devicemapper/"
			//var targetPath = path.Join("/var/run/dvm/", daemon.ID, containerId)
			for _, f := range c.Files {
				targetPath := f.Path
				fromFile := files[f.Filename].Uri
				if fromFile == "" {
					continue
				}
				err := attachFiles(containerId, devPrefix, fromFile, targetPath, rootPath, f.Perm)
				if err != nil {
					return "", err
				}
			}
			devFullName = fmt.Sprintf("/dev/mapper/%s-%s", devPrefix, containerId)
			env := make(map[string]string)
			for _, v := range jsonResponse.Config.Env {
				env[v[:strings.Index(v, "=")]] = v[strings.Index(v, "=")+1:]
			}
            containerCreateEvent := &ContainerCreatedEvent {
                Index: i+1,
                Id: containerId,
                Rootfs: "/rootfs",
                Image: devFullName,
                Fstype: fstype,
                Workdir: jsonResponse.Config.WorkingDir,
                Cmd: jsonResponse.Config.Cmd,
                Envs: env,
            }
            hub <- containerCreateEvent
		} else {
			return "", fmt.Errorf("AN error encountered during creating container!\n")
		}
	}

	// Process the 'Volumes' section
	for _, v := range userPod.Volumes {
		if v.Source == "" {
			if storageDriver == "devicemapper" {
				volName := fmt.Sprintf("%s-volume-", devPrefix, v.Name)
				vol, err  := exec.LookPath("dmsetup")
				if err != nil {
					return "", nil
				}
				createvVolArgs := fmt.Sprintf("create %s --table \"0 %d thin %s %d\"", volName, 10737418240/512, poolName, 100)
				createVolCommand := exec.Command(vol, createvVolArgs)
				if _, err := createVolCommand.Output(); err != nil {
					return "", err
				}
				// Need to make the filesystem on that volume
				var fscmd string
				if fstype == "ext4" {
					fscmd, err := exec.LookPath("mkfs.ext4")
				} else {
					fscmd, err := exec.LookPath("mkfs.xfs")
				}
				makeFsCmd := exec.Command(fscmd, path.Join("/dev/mapper/", volName))
				if _, err := makeFsCmd.Output(); err != nil {
					return "", err
				}
				myVolReadyEvent := &VolumeReadyEvent {
					Name: v.Name,
					Filepath: path.Join("/dev/mapper/", volName),
					Fstype: fstype,
					Format: "raw",
				}
				hub <- myVolReadyEvent

			} else if storageDriver == "aufs" {
				// TODO
			}
		} else {
			// Process the situation if the source is not NULL, we need to bind that dir to sharedDir
			var flags uintptr = syscall.MS_MGC_VAL

			mountSharedDir = pod.RandStr(10, "alpha")
			if err := syscall.Mount(v.Source, path.Join(sharedDir, mountSharedDir), fstype, flags, "--bind"); err != nil {
				return "", nil
			}
			myVolReadyEvent := &VolumeReadyEvent {
				Name: v.Name,
				Filepath: mountSharedDir,
				Fstype: "dir",
				Format: "",
			}
			hub <- myVolReadyEvent
		}
	}

	return containerId, nil
}

func attachFiles(containerId, devPrefix, fromFile, toDir, rootPath, perm string) error {
	if containerId == "" {
		return fmt.Errorf("Please make sure the arguments are not NULL!\n")
	}
	permInt, err := strconv.Atoi(perm)
	if err != nil {
		return err
	}
	// Define the basic directory, need to get them via the 'info' command
	var (
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
	devName := fmt.Sprintf("%s-%s", devPrefix, containerId)
	poolName := fmt.Sprintf("%s-pool", devPrefix)
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
		syscall.Unmount(idMountPath, syscall.MNT_DETACH)
		return nil
	}
	// Make a new file with the given premission and wirte the source file content in it
	if _, err := os.Stat(fromFile); err != nil && os.IsNotExist(err) {
		// The gived file is not exist, we need to unmout the device and return
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
