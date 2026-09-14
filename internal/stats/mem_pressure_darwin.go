//go:build darwin

package stats

import (
	"bufio"
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v3/mem"
	"golang.org/x/sys/unix"
)

type vmCounters struct {
	PageSize        uint64
	CompressedPages uint64
	PageOuts        uint64
}

func (c *Collector) collectMemory(ctx context.Context, now time.Time) MemoryStats {
	result := MemoryStats{Status: MetricStatus{SampledAt: now}, Pressure: "Unknown"}
	vm, vmErr := mem.VirtualMemoryWithContext(ctx)
	if vmErr != nil {
		result.Status = unavailableStatus(now, vmErr)
		return result
	}
	result.Status = availableStatus(now)
	result.UsedBytes = vm.Used
	result.TotalBytes = vm.Total
	result.UsedPercent = vm.UsedPercent

	pressure, available, pressureErr := memoryPressure()
	if pressureErr == nil {
		result.Pressure = pressure
		result.AvailablePercent = available
		result.PressureAvailable = true
	}

	swap, swapErr := mem.SwapMemoryWithContext(ctx)
	if swapErr == nil {
		result.SwapUsedBytes = swap.Used
	}

	counters, countersErr := nativeVMCounters()
	if countersErr != nil {
		output, err := commandOutput(ctx, "/usr/bin/vm_stat")
		countersErr = err
		if err == nil {
			counters, countersErr = parseVMCounters(string(output))
		}
	}

	if countersErr == nil {
		result.CompressedBytes = counters.CompressedPages * counters.PageSize
		result.CompressionAvailable = true

		c.memoryMu.Lock()
		if !c.lastMemoryAt.IsZero() && counters.PageOuts >= c.lastPageOuts {
			dt := now.Sub(c.lastMemoryAt).Seconds()
			if dt > 0 && dt < 30 {
				result.PageOutBytesPerSecond = float64((counters.PageOuts-c.lastPageOuts)*counters.PageSize) / dt
				result.PageOutRateAvailable = true
			}
		}
		c.lastPageOuts = counters.PageOuts
		c.lastMemoryAt = now
		c.memoryMu.Unlock()
	}

	if err := joinErrors(pressureErr, swapErr, countersErr); err != nil {
		result.Status.Error = cleanText(err.Error())
	}
	return result
}

func memoryPressure() (state string, availablePercent float64, err error) {
	level, levelErr := unix.SysctlUint32("kern.memorystatus_vm_pressure_level")
	available, availableErr := unix.SysctlUint32("kern.memorystatus_level")
	if levelErr != nil || availableErr != nil {
		return "Unknown", 0, joinErrors(levelErr, availableErr)
	}
	if available > 100 {
		return "Unknown", 0, fmt.Errorf("memory headroom out of range: %d", available)
	}

	switch {
	case level&4 != 0:
		state = "Critical"
	case level&2 != 0:
		state = "Warning"
	case level&1 != 0:
		state = "Normal"
	default:
		state = "Unknown"
	}
	return state, float64(available), nil
}

func parseVMCounters(output string) (vmCounters, error) {
	var result vmCounters
	foundCompressed, foundPageOuts := false, false
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.Contains(line, "page size of") {
			fields := strings.Fields(line)
			for i, field := range fields {
				if field == "of" && i+1 < len(fields) {
					value, err := strconv.ParseUint(fields[i+1], 10, 64)
					if err == nil {
						result.PageSize = value
					}
				}
			}
			continue
		}

		name, raw, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		value, err := parseVMValue(raw)
		if err != nil {
			continue
		}
		switch strings.TrimSpace(name) {
		case "Pages occupied by compressor":
			result.CompressedPages = value
			foundCompressed = true
		case "Pageouts", "Pages paged out":
			result.PageOuts = value
			foundPageOuts = true
		}
	}
	if err := scanner.Err(); err != nil {
		return result, fmt.Errorf("vm_stat: %w", err)
	}
	if result.PageSize == 0 {
		return result, fmt.Errorf("vm_stat: page size missing")
	}
	if !foundCompressed || !foundPageOuts {
		return result, fmt.Errorf("vm_stat: compressor or page-out counters missing")
	}
	return result, nil
}

func parseVMValue(raw string) (uint64, error) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimSuffix(raw, ".")
	raw = strings.ReplaceAll(raw, ",", "")
	fields := strings.Fields(raw)
	if len(fields) == 0 {
		return 0, fmt.Errorf("empty vm_stat value")
	}
	value, err := strconv.ParseUint(fields[0], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("vm_stat value %q: %w", fields[0], err)
	}
	return value, nil
}
