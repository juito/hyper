package aufs

import (
    "fmt"
    "os"
    "io/ioutil"
    "path"
    "testing"
)

var (
    aufsSupport = false
    temp = os.TempDir()
    aufsTempDir = path.Join(temp, "aufs-test")
    layersTempDir = path.Join(aufsTempDir, "layers")
    diffTempDir = path.Join(aufsTempDir, "diff")
)

func init() {
    aufsSupport = supportAufs()
}

func InitDir() error {
    if err := os.MkdirAll(aufsTempDir, 0755); err != nil {
        return err
    }

    if err := os.MkdirAll(layersTempDir, 0755); err != nil {
        return err
    }

    if err := os.MkdirAll(diffTempDir, 0755); err != nil {
        return err
    }
    return nil
}

func InitFile() error {
    testFile3 := fmt.Sprintf("%s/3", layersTempDir)
    testFile2 := fmt.Sprintf("%s/2", layersTempDir)
    testFile1 := fmt.Sprintf("%s/1", layersTempDir)

    file1Data := ""
    file2Data := "1"
    file3Data := "1\n2"

    if err := ioutil.WriteFile(testFile1, []byte(file1Data), 0755); err != nil {
        return err
    }
    if err := ioutil.WriteFile(testFile2, []byte(file2Data), 0755); err != nil {
        return err
    }
    if err := ioutil.WriteFile(testFile3, []byte(file3Data), 0755); err != nil {
        return err
    }

    return nil
}

func supportAufs() bool {
	// We can try to modprobe aufs first before looking at
	// proc/filesystems for when aufs is supported
	exec.Command("modprobe", "aufs").Run()

	f, err := os.Open("/proc/filesystems")
	if err != nil {
		return false
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	for s.Scan() {
		if strings.Contains(s.Text(), "aufs") {
			return true
		}
	}
	return false
}

func TestTempDirCreate(t *testing.T) {
    if err := InitDir(); err != nil {
        t.Fatalf("Error during creating the temp directory: %s\n", err.Error())
    }

    if err := InitFile(); err != nil {
        t.Fatalf("Error during creating the test file: %s\n", err.Error())
    }
}

func TestMountContainerToSharedDir(t *testing.T) {
    if supportAufs() == false {
        return
    }
}

func TestAttachFiles(t *testing.T) {
    if supportAufs() == false {
        return
    }
}

func TestGetParentIds(t *testing.T) {

}

func TestGetParentDiffPaths(t *testing.T) {
    if supportAufs() == false {
        return
    }
}

func TestAufsMount(t *testing.T) {
    if supportAufs() == false {
        return
    }
}


