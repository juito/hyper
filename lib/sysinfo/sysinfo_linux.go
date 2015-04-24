// +build linux

package sysinfo

import (
	"bufio"
	"bytes"
	"io"
	"io/ioutil"
	"strconv"
	"strings"
)

var (
	cpuInfoProcLinuxFile = "/proc/cpuinfo"
	memInfoProcLinuxFile = "/proc/meminfo"
)

func getMemInfo() (*MemInfo, error) {
	mem := &MemInfo{}
	// read in whole meminfo file with cpu usage information ;"/proc/meminfo"
	contents, err := ioutil.ReadFile(memInfoProcLinuxFile)
	if err != nil {
		return nil, err
	}

	reader := bufio.NewReader(bytes.NewBuffer(contents))
	for {
		line, _, err := reader.ReadLine()
		if err == io.EOF {
			break
		}

		fields := strings.Fields(string(line))
		fieldName := fields[0]

		if len(fields) == 3 {
			val, err := strconv.ParseUint(fields[1], 10, 64)
			if err != nil {
				return nil, err
			}
			switch fieldName {
			case "MemTotal:":
				mem.MemTotal = val
			case "MemFree:":
				mem.MemFree = val
			case "MemAvailable:":
				mem.MemAvailable = val
			case "Buffers:":
				mem.Buffers = val
			case "Cached:":
				mem.Cached = val
			case "SwapCached:":
				mem.SwapCached = val
			case "Active:":
				mem.Active = val
			case "Inactive:":
				mem.Inactive = val
			case "Active(anon):":
				mem.AnonActive = val
			case "Inactive(anon):":
				mem.AnonInactive = val
			case "Active(file):":
				mem.FileActive = val
			case "Inactive(file):":
				mem.FileInactive = val
			case "Unevictable:":
				mem.Unevictable = val
			case "Mlocked:":
				mem.Mlocked = val
			case "SwapTotal:":
				mem.SwapTotal = val
			case "SwapFree:":
				mem.SwapFree = val
			case "Dirty:":
				mem.Dirty = val
			case "Writeback:":
				mem.Writeback = val
			case "AnonPages:":
				mem.AnonPages = val
			case "Mapped:":
				mem.Mapped = val
			case "Shmem:":
				mem.Shmem = val
			case "Slab:":
				mem.Slab = val
			case "SReclaimable:":
				mem.SReclaimable = val
			case "SUnreclaim:":
				mem.SUnreclaim = val
			case "KernelStack:":
				mem.KernelStack = val
			case "PageTables:":
				mem.PageTables = val
			case "NFS_Unstable:":
				mem.NFS_Unstable = val
			case "Bounce:":
				mem.Bounce = val
			case "WritebackTmp:":
				mem.WritebackTmp = val
			case "CommitLimit:":
				mem.CommitLimit = val
			case "Committed_AS:":
				mem.Committed_AS = val
			case "VmallocTotal:":
				mem.VmallocTotal = val
			case "VmallocUsed:":
				mem.VmallocUsed = val
			case "VmallocChunk:":
				mem.VmallocChunk = val
			case "HardwareCorrupted:":
				mem.HardwareCorrupted = val
			case "AnonHugePages:":
				mem.AnonHugePages = val
			case "Hugepagesize:":
				mem.Hugepagesize = val
			case "DirectMap4k:":
				mem.DirectMap4k = val
			case "DirectMap2M:":
				mem.DirectMap2M = val
			case "DirectMap1G:":
				mem.DirectMap1G = val
			}
		}
	}
	return mem, nil
}

func getCpuInfo() (*CpuInfo, error) {
	return nil, nil
}
