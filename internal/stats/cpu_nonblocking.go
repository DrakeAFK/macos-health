package stats

import (
	"sync"

	"github.com/shirou/gopsutil/v3/cpu"
)

var (
	cpuMu      sync.Mutex
	lastCPUAll *cpu.TimesStat
)

func cpuPercentNonBlocking() (float64, error) {
	cpuMu.Lock()
	defer cpuMu.Unlock()

	cur, err := cpu.Times(false)
	if err != nil || len(cur) == 0 {
		return 0, err
	}
	now := cur[0]

	if lastCPUAll == nil {
		lastCPUAll = &now
		return 0, nil
	}

	prev := *lastCPUAll
	lastCPUAll = &now

	prevTotal := totalCPUTime(prev)
	nowTotal := totalCPUTime(now)
	deltaTotal := nowTotal - prevTotal
	if deltaTotal <= 0 {
		return 0, nil
	}

	prevIdle := prev.Idle
	nowIdle := now.Idle
	deltaIdle := nowIdle - prevIdle
	if deltaIdle < 0 {
		deltaIdle = 0
	}

	used := deltaTotal - deltaIdle
	if used < 0 {
		used = 0
	}

	return (used / deltaTotal) * 100.0, nil
}

func totalCPUTime(t cpu.TimesStat) float64 {
	return t.User + t.System + t.Idle + t.Nice + t.Iowait + t.Irq + t.Softirq + t.Steal + t.Guest + t.GuestNice
}
