//go:build darwin

package stats

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var thermalValuePattern = regexp.MustCompile(`(?m)^\s*(CPU_Speed_Limit|Scheduler_Limit|Thermal_Level)\s*=\s*(\d+)\s*$`)

func collectThermal(ctx context.Context) (ThermalStats, error) {
	output, err := commandOutput(ctx, "/usr/bin/pmset", "-g", "therm")
	if err != nil {
		return ThermalStats{}, err
	}
	return parseThermal(string(output))
}

func parseThermal(output string) (ThermalStats, error) {
	result := ThermalStats{State: "Unknown"}
	lower := strings.ToLower(output)
	matches := thermalValuePattern.FindAllStringSubmatch(output, -1)
	cpuLimitFound, schedulerLimitFound := false, false
	for _, match := range matches {
		value, _ := strconv.Atoi(match[2])
		switch match[1] {
		case "CPU_Speed_Limit":
			result.CPUSpeedLimitPercent = value
			cpuLimitFound = true
		case "Scheduler_Limit":
			result.SchedulerLimitPercent = value
			schedulerLimitFound = true
		case "Thermal_Level":
			result.ThermalLevel = value
		}
	}

	switch {
	case cpuLimitFound && result.CPUSpeedLimitPercent < 100:
		result.State = "Throttled"
	case schedulerLimitFound && result.SchedulerLimitPercent < 100:
		result.State = "Throttled"
	case result.ThermalLevel > 0:
		result.State = "Elevated"
	case strings.Contains(lower, "no thermal warning level") && strings.Contains(lower, "no performance warning level"):
		result.State = "Nominal"
	case len(matches) > 0:
		result.State = "Nominal"
	case strings.TrimSpace(output) == "":
		return result, fmt.Errorf("pmset thermal output was empty")
	default:
		return result, fmt.Errorf("pmset thermal output was not recognized")
	}
	return result, nil
}
