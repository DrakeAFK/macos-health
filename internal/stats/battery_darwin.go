//go:build darwin

package stats

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	batteryPercentPattern = regexp.MustCompile(`(?m)(\d+(?:\.\d+)?)%`)
	batteryETAPattern     = regexp.MustCompile(`(?m)(\d+):(\d+)\s+remaining`)
	batteryTempPattern    = regexp.MustCompile(`(?m)"Temperature"\s*=\s*(\d+)`)
)

type batteryDetails struct {
	healthPercent   float64
	condition       string
	cycleCount      int
	temperatureC    float64
	temperatureOK   bool
	lowPowerBattery bool
	lowPowerAC      bool
	chargerWatts    int
}

type powerHealthInfo struct {
	CycleCount      int    `json:"sppower_battery_cycle_count"`
	Condition       string `json:"sppower_battery_health"`
	MaximumCapacity string `json:"sppower_battery_health_maximum_capacity"`
}

type powerProfilerEntry struct {
	Name          string          `json:"_name"`
	BatteryHealth powerHealthInfo `json:"sppower_battery_health_info"`
	ChargerWatts  int             `json:"sppower_ac_charger_watts"`
}

type powerProfilerReport struct {
	Entries []powerProfilerEntry `json:"SPPowerDataType"`
}

func (c *Collector) collectBattery(ctx context.Context, now time.Time) BatteryStats {
	output, err := commandOutput(ctx, "/usr/bin/pmset", "-g", "batt")
	if err != nil {
		return BatteryStats{Status: unavailableStatus(now, err)}
	}
	result, err := parsePMSetBattery(string(output))
	if err != nil {
		result.Status = unavailableStatus(now, err)
		return result
	}
	result.Status = availableStatus(now)
	if !result.Present {
		return result
	}

	details, detailsErr := c.batteryDetails(ctx, now)
	result.HealthPercent = details.healthPercent
	result.Condition = details.condition
	result.CycleCount = details.cycleCount
	result.TemperatureC = details.temperatureC
	result.TemperatureAvailable = details.temperatureOK
	result.ChargerWatts = details.chargerWatts
	result.DetailsAvailable = details.healthPercent > 0 || details.condition != "" || details.cycleCount > 0
	if strings.EqualFold(result.PowerSource, "AC Power") {
		result.LowPowerMode = details.lowPowerAC
	} else {
		result.LowPowerMode = details.lowPowerBattery
	}
	if detailsErr != nil {
		result.Status.Error = cleanText(detailsErr.Error())
	}
	return result
}

func parsePMSetBattery(output string) (BatteryStats, error) {
	result := BatteryStats{}
	lower := strings.ToLower(output)
	switch {
	case strings.Contains(lower, "battery power"):
		result.PowerSource = "Battery Power"
	case strings.Contains(lower, "ac power"):
		result.PowerSource = "AC Power"
	}
	if strings.Contains(lower, "no batteries") {
		return result, nil
	}
	result.Present = strings.Contains(output, "InternalBattery") || strings.Contains(lower, "present: true")
	if !result.Present {
		return result, fmt.Errorf("pmset: battery state not found")
	}

	match := batteryPercentPattern.FindStringSubmatch(output)
	if len(match) != 2 {
		return result, fmt.Errorf("pmset: battery percentage missing")
	}
	percent, err := strconv.ParseFloat(match[1], 64)
	if err != nil || percent < 0 || percent > 100 {
		return result, fmt.Errorf("pmset: invalid battery percentage %q", match[1])
	}
	result.Percent = percent

	switch {
	case strings.Contains(lower, "not charging"):
		result.State = "Not Charging"
	case strings.Contains(lower, "basking"):
		result.State = "On Hold (80%)"
	case strings.Contains(lower, "inhibited"):
		result.State = "Inhibited"
	case strings.Contains(lower, "discharging"):
		result.State = "Discharging"
	case strings.Contains(lower, "charging"):
		result.State = "Charging"
	case strings.Contains(lower, "charged"):
		result.State = "Charged"
	default:
		result.State = "Unknown"
	}

	if eta := batteryETAPattern.FindStringSubmatch(output); len(eta) == 3 {
		hours, hoursErr := strconv.Atoi(eta[1])
		minutes, minutesErr := strconv.Atoi(eta[2])
		if hoursErr == nil && minutesErr == nil && minutes < 60 {
			result.TimeRemainingMinutes = hours*60 + minutes
			result.TimeRemainingAvailable = true
		}
	}
	return result, nil
}

func collectBatteryDetails(ctx context.Context) (batteryDetails, error) {
	var result batteryDetails
	var errs []error

	output, err := commandOutput(ctx, "/usr/sbin/system_profiler", "SPPowerDataType", "-json", "-detailLevel", "mini")
	if err != nil {
		errs = append(errs, err)
	} else if err := parsePowerProfiler(output, &result); err != nil {
		errs = append(errs, err)
	}

	ioreg, err := commandOutput(ctx, "/usr/sbin/ioreg", "-r", "-c", "AppleSmartBattery", "-l")
	if err != nil {
		errs = append(errs, err)
	} else if match := batteryTempPattern.FindStringSubmatch(string(ioreg)); len(match) == 2 {
		raw, parseErr := strconv.ParseFloat(match[1], 64)
		if parseErr == nil {
			// AppleSmartBattery reports tenths of Kelvin on current Apple-silicon
			// MacBooks. Validate before exposing this best-effort registry value.
			celsius := raw/10 - 273.15
			if celsius >= -20 && celsius <= 100 {
				result.temperatureC = math.Round(celsius*10) / 10
				result.temperatureOK = true
			}
		}
	}

	custom, err := commandOutput(ctx, "/usr/bin/pmset", "-g", "custom")
	if err != nil {
		errs = append(errs, err)
	} else {
		result.lowPowerBattery, result.lowPowerAC = parseLowPowerModes(string(custom))
	}
	return result, joinErrors(errs...)
}

func parsePowerProfiler(output []byte, result *batteryDetails) error {
	var report powerProfilerReport
	if err := json.Unmarshal(output, &report); err != nil {
		return fmt.Errorf("system_profiler power JSON: %w", err)
	}
	for _, entry := range report.Entries {
		if entry.Name == "spbattery_information" {
			result.cycleCount = entry.BatteryHealth.CycleCount
			result.condition = cleanText(entry.BatteryHealth.Condition)
			capacity := strings.TrimSuffix(strings.TrimSpace(entry.BatteryHealth.MaximumCapacity), "%")
			result.healthPercent, _ = strconv.ParseFloat(capacity, 64)
		}
		if entry.ChargerWatts > 0 {
			result.chargerWatts = entry.ChargerWatts
		}
	}
	return nil
}

func parseLowPowerModes(output string) (battery, ac bool) {
	section := ""
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		switch trimmed {
		case "Battery Power:":
			section = "battery"
		case "AC Power:":
			section = "ac"
		default:
			fields := strings.Fields(trimmed)
			if len(fields) == 2 && fields[0] == "lowpowermode" && fields[1] == "1" {
				if section == "battery" {
					battery = true
				} else if section == "ac" {
					ac = true
				}
			}
		}
	}
	return battery, ac
}
