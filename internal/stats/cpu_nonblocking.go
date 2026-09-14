//go:build darwin

package stats

import (
	"context"
	"fmt"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
)

func (c *Collector) collectCPU(ctx context.Context, now time.Time) CPUStats {
	result := CPUStats{Status: MetricStatus{SampledAt: now}}
	l1, l5, l15, loadErr := loadAvg(ctx)
	if loadErr == nil {
		result.Load1, result.Load5, result.Load15 = l1, l5, l15
		result.LoadAvailable = true
	}

	current, cpuErr := cpu.TimesWithContext(ctx, true)
	if cpuErr != nil || len(current) == 0 {
		if cpuErr == nil {
			cpuErr = fmt.Errorf("CPU counters returned no cores")
		}
		result.Status = unavailableStatus(now, joinErrors(cpuErr, loadErr))
		return result
	}

	c.cpuMu.Lock()
	defer c.cpuMu.Unlock()

	perCore := make([]float64, 0, len(current))
	hadBaseline := len(c.lastCPU) > 0
	validCores := 0
	for _, cur := range current {
		prev, ok := c.lastCPU[cur.CPU]
		c.lastCPU[cur.CPU] = cur
		if !ok {
			perCore = append(perCore, 0)
			continue
		}
		value, ok := cpuDelta(prev, cur)
		// Apple-silicon cores can be power-gated for an entire interval, leaving
		// every tick unchanged. That represents an idle core, not a failed
		// system-wide sample.
		if !ok && totalCPUTime(cur) == totalCPUTime(prev) {
			value, ok = 0, true
		}
		if !ok {
			perCore = append(perCore, 0)
			continue
		}
		perCore = append(perCore, value)
		validCores++
	}

	if !hadBaseline {
		result.WarmingUp = true
		result.Status = MetricStatus{SampledAt: now}
		if loadErr != nil {
			result.Status.Error = cleanText(loadErr.Error())
		}
		return result
	}
	if validCores == 0 {
		result.Status = unavailableStatus(now, fmt.Errorf("CPU counters did not advance"))
		return result
	}

	var total float64
	for _, value := range perCore {
		total += value
	}
	result.UsagePercent = total / float64(len(perCore))
	result.PerCorePercent = perCore
	result.Status = availableStatus(now)
	var partialErr error
	if validCores != len(current) {
		partialErr = fmt.Errorf("%d of %d CPU core counters reset", len(current)-validCores, len(current))
	}
	if err := joinErrors(loadErr, partialErr); err != nil {
		result.Status.Error = cleanText(err.Error())
	}
	return result
}

func cpuDelta(prev, current cpu.TimesStat) (float64, bool) {
	prevTotal := totalCPUTime(prev)
	currentTotal := totalCPUTime(current)
	deltaTotal := currentTotal - prevTotal
	if deltaTotal <= 0 {
		return 0, false
	}

	deltaIdle := current.Idle - prev.Idle
	if deltaIdle < 0 || deltaIdle > deltaTotal {
		return 0, false
	}
	value := (deltaTotal - deltaIdle) / deltaTotal * 100
	if value < 0 {
		value = 0
	}
	if value > 100 {
		value = 100
	}
	return value, true
}

func totalCPUTime(t cpu.TimesStat) float64 {
	return t.User + t.System + t.Idle + t.Nice + t.Iowait + t.Irq + t.Softirq + t.Steal + t.Guest + t.GuestNice
}
