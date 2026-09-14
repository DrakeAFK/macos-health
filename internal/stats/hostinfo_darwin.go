//go:build darwin

package stats

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

type profilerHardware struct {
	Name             string `json:"_name"`
	Chip             string `json:"chip_type"`
	MachineModel     string `json:"machine_model"`
	MachineName      string `json:"machine_name"`
	NumberProcessors string `json:"number_processors"`
}

type profilerDisplay struct {
	Name     string `json:"_name"`
	Model    string `json:"sppci_model"`
	GPUCores string `json:"sppci_cores"`
}

type profilerHostReport struct {
	Hardware []profilerHardware `json:"SPHardwareDataType"`
	Displays []profilerDisplay  `json:"SPDisplaysDataType"`
}

func collectHostInfo(ctx context.Context) (HostInfo, error) {
	value := HostInfo{Architecture: runtime.GOARCH}
	var partial []error

	if hostname, err := os.Hostname(); err == nil {
		value.Hostname = cleanText(hostname)
	} else {
		partial = append(partial, err)
	}
	if output, err := commandOutput(ctx, "/usr/sbin/scutil", "--get", "ComputerName"); err == nil {
		value.ComputerName = cleanText(string(output))
	} else {
		partial = append(partial, err)
	}
	if output, err := commandOutput(ctx, "/usr/bin/sw_vers", "-productVersion"); err == nil {
		value.OSVersion = cleanText(string(output))
	} else {
		partial = append(partial, err)
	}
	if output, err := commandOutput(ctx, "/usr/bin/sw_vers", "-buildVersion"); err == nil {
		value.OSBuild = cleanText(string(output))
	} else {
		partial = append(partial, err)
	}

	value.ModelIdentifier, _ = unix.Sysctl("hw.model")
	value.Chip, _ = unix.Sysctl("machdep.cpu.brand_string")
	if cores, err := unix.SysctlUint32("hw.logicalcpu"); err == nil {
		value.LogicalCores = int(cores)
	}
	for level := 0; level < 4; level++ {
		name, err := unix.Sysctl(fmt.Sprintf("hw.perflevel%d.name", level))
		if err != nil {
			continue
		}
		cores, err := unix.SysctlUint32(fmt.Sprintf("hw.perflevel%d.physicalcpu", level))
		if err != nil {
			continue
		}
		switch strings.ToLower(name) {
		case "performance":
			value.PerformanceCores += int(cores)
		case "efficiency":
			value.EfficiencyCores += int(cores)
		}
	}
	if arm, err := unix.SysctlUint32("hw.optional.arm64"); err == nil && arm == 1 {
		value.Architecture = "arm64"
	}
	if ne, err := unix.SysctlUint32("hw.optional.nepan"); err == nil && ne == 1 {
		value.NeuralEngine = true
	}
	if translated, err := unix.SysctlUint32("sysctl.proc_translated"); err == nil {
		value.RunningUnderRosetta = translated == 1
	}

	output, err := commandOutput(ctx, "/usr/sbin/system_profiler", "SPHardwareDataType", "SPDisplaysDataType", "-json", "-detailLevel", "mini")
	if err != nil {
		partial = append(partial, err)
	} else {
		var report profilerHostReport
		if err := json.Unmarshal(output, &report); err != nil {
			partial = append(partial, fmt.Errorf("system_profiler JSON: %w", err))
		} else {
			if len(report.Hardware) > 0 {
				hardware := report.Hardware[0]
				value.MachineName = cleanText(hardware.MachineName)
				if hardware.MachineModel != "" {
					value.ModelIdentifier = cleanText(hardware.MachineModel)
				}
				if hardware.Chip != "" {
					value.Chip = cleanText(hardware.Chip)
				}
			}
			if len(report.Displays) > 0 {
				display := report.Displays[0]
				value.GPU = cleanText(display.Model)
				if value.GPU == "" {
					value.GPU = cleanText(display.Name)
				}
				value.GPUCores, _ = strconv.Atoi(display.GPUCores)
			}
		}
	}

	value.ModelIdentifier = cleanText(value.ModelIdentifier)
	value.Chip = cleanText(value.Chip)
	if value.Chip == "" {
		value.Chip = value.ModelIdentifier
	}
	if value.LogicalCores == 0 {
		value.LogicalCores = runtime.NumCPU()
	}

	if value.Hostname == "" && value.Chip == "" && value.OSVersion == "" {
		return value, joinErrors(partial...)
	}
	return value, joinErrors(partial...)
}
