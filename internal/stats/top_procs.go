package stats

import (
	"context"
	"sort"

	"github.com/shirou/gopsutil/v3/process"
)

func topProcessesDual(ctx context.Context, n int) ([]ProcRow, []ProcRow, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}

	procs, err := process.ProcessesWithContext(ctx)
	if err != nil {
		return nil, nil, err
	}

	rows := make([]ProcRow, 0, len(procs))
	for _, p := range procs {
		if err := ctx.Err(); err != nil {
			return rows, rows, err
		}

		name, err := p.NameWithContext(ctx)
		if err != nil || name == "" {
			continue
		}

		cpuPct, _ := p.CPUPercentWithContext(ctx)
		memPct, _ := p.MemoryPercentWithContext(ctx)

		rows = append(rows, ProcRow{
			Name:       name,
			CPUPercent: cpuPct,
			MemPercent: float64(memPct),
		})
	}

	topCPU := make([]ProcRow, len(rows))
	copy(topCPU, rows)
	sort.Slice(topCPU, func(i, j int) bool {
		if topCPU[i].CPUPercent == topCPU[j].CPUPercent {
			return topCPU[i].MemPercent > topCPU[j].MemPercent
		}
		return topCPU[i].CPUPercent > topCPU[j].CPUPercent
	})

	topMem := make([]ProcRow, len(rows))
	copy(topMem, rows)
	sort.Slice(topMem, func(i, j int) bool {
		if topMem[i].MemPercent == topMem[j].MemPercent {
			return topMem[i].CPUPercent > topMem[j].CPUPercent
		}
		return topMem[i].MemPercent > topMem[j].MemPercent
	})

	if len(topCPU) > n {
		topCPU = topCPU[:n]
	}
	if len(topMem) > n {
		topMem = topMem[:n]
	}

	return topCPU, topMem, nil
}
