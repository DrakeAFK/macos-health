package stats

import (
	"context"
	"sort"

	"github.com/shirou/gopsutil/v3/process"
)

func topProcesses(ctx context.Context, n int) ([]ProcRow, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	procs, err := process.ProcessesWithContext(ctx)
	if err != nil {
		return nil, err
	}

	rows := make([]ProcRow, 0, len(procs))
	for _, p := range procs {
		if err := ctx.Err(); err != nil {
			return rows, err
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

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].CPUPercent == rows[j].CPUPercent {
			return rows[i].MemPercent > rows[j].MemPercent
		}
		return rows[i].CPUPercent > rows[j].CPUPercent
	})

	if len(rows) > n {
		rows = rows[:n]
	}
	return rows, nil
}
