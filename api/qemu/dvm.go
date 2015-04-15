package qemu

import (
	"fmt"
	"syscall"
	"os/exec"
	"path"
	"strings"
    "dvm/api/docker"
    "dvm/api/pod"
    "dvm/engine"
	dm "dvm/api/storage/devicemapper"
)

func CreateContainer(userPod *pod.UserPod, sharedDir string, hub chan QemuEvent) (string, error) {
    var (
        proto = "unix"
        addr = "/var/run/docker.sock"
		fstype string
		poolName string
		devPrefix string
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
	fmt.Printf("Process the Containers section in POD SPEC\n")
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
			var jsonResponse *docker.ConfigJSON
			if jsonResponse, err = cli.GetContainerInfo(containerId); err != nil {
				return "", err
			}

			devFullName, err := dm.MountContainerToSharedDir(containerId, sharedDir, devPrefix)
			if err != nil {
				return "", err
			}

			var rootPath = "/var/lib/docker/devicemapper/"
			for _, f := range c.Files {
				targetPath := f.Path
				fromFile := files[f.Filename].Uri
				if fromFile == "" {
					continue
				}
				err := dm.AttachFiles(containerId, devPrefix, fromFile, targetPath, rootPath, f.Perm)
				if err != nil {
					return "", err
				}
			}

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
					fscmd, err = exec.LookPath("mkfs.ext4")
				} else {
					fscmd, err = exec.LookPath("mkfs.xfs")
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
				continue

			} else {
				// Make sure the v.Name is given
				v.Source = path.Join("/var/tmp/", v.Name)
				if _, err := os.Stat(v.Source); err != nil && os.IsNotExist(err) {
					if err := os.MkdirAll(targetDir, os.FileMode(0777)); err != nil {
						return "", nil
					}
				}
			}
		}

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

	return containerId, nil
}
