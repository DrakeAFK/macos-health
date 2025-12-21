package stats

import (
	"context"
	"fmt"
	"time"

	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
)

type ProcRow struct {
	Name       string
	CPUPercent float64
	MemPercent float64
}

type Snapshot struct {
	CPUPercent float64
	Load1      float64
	Load5      float64
	Load15     float64

	MemUsedBytes  uint64
	MemTotalBytes uint64
	MemUsedPct    float64
	SwapUsedBytes uint64

	MemPressureLevel string
	MemPressurePct   float64

	DiskUsedBytes  uint64
	DiskTotalBytes uint64

	NetIface   string
	NetDownBps float64
	NetUpBps   float64

	BatteryPercent float64
	BatteryState   string
	BatteryETA     string

	UptimeSec uint64

	Host HostInfo
	Top  []ProcRow
}

func Collect(ctx context.Context) (Snapshot, error) {
	var s Snapshot

	if err := ctx.Err(); err != nil {
		return s, err
	}
	s.Host = getHostInfo(ctx)

	if err := ctx.Err(); err != nil {
		return s, err
	}
	if cpuPct, err := cpuPercentNonBlocking(); err == nil {
		s.CPUPercent = cpuPct
	}

	if err := ctx.Err(); err != nil {
		return s, err
	}
	l1, l5, l15, err := loadAvg(ctx)
	if err == nil {
		s.Load1, s.Load5, s.Load15 = l1, l5, l15
	}

	if err := ctx.Err(); err != nil {
		return s, err
	}
	vm, err := mem.VirtualMemory()
	if err != nil {
		return s, err
	}
	s.MemUsedBytes = vm.Used
	s.MemTotalBytes = vm.Total
	s.MemUsedPct = vm.UsedPercent

	if err := ctx.Err(); err != nil {
		return s, err
	}
	if swap, err := mem.SwapMemory(); err == nil {
		s.SwapUsedBytes = swap.Used
	}

	if err := ctx.Err(); err != nil {
		return s, err
	}
	level, pct := memPressure(ctx)
	s.MemPressureLevel = level
	s.MemPressurePct = pct

	if err := ctx.Err(); err != nil {
		return s, err
	}
	if du, err := disk.Usage("/"); err == nil {
		s.DiskUsedBytes = du.Used
		s.DiskTotalBytes = du.Total
	}

	if err := ctx.Err(); err != nil {
		return s, err
	}
	iface, down, up := activeNetRate(ctx)
	s.NetIface = iface
	s.NetDownBps = down
	s.NetUpBps = up

	if err := ctx.Err(); err != nil {
		return s, err
	}
	bp, state, eta := battery(ctx)
	s.BatteryPercent = bp
	s.BatteryState = state
	s.BatteryETA = eta

	if err := ctx.Err(); err != nil {
		return s, err
	}
	if hi, err := host.InfoWithContext(ctx); err == nil {
		s.UptimeSec = hi.Uptime
	}

	if err := ctx.Err(); err != nil {
		return s, err
	}
	if top, err := topProcesses(ctx, 5); err == nil {
		s.Top = top
	}

	return s, nil
}

func (s Snapshot) CPUString() string {
	return fmt.Sprintf("%4.0f%%", s.CPUPercent)
}

func (s Snapshot) LoadString() string {
	if s.Load1 == 0 && s.Load5 == 0 && s.Load15 == 0 {
		return "n/a"
	}
	return fmt.Sprintf("%.1f %.1f %.1f", s.Load1, s.Load5, s.Load15)
}

func (s Snapshot) MemString() string {
	pressure := "n/a"
	if s.MemPressureLevel != "" {
		if s.MemPressureLevel == "Low" || s.MemPressureLevel == "Medium" || s.MemPressureLevel == "High" {
			pressure = fmt.Sprintf("%s (%.0f%%)", s.MemPressureLevel, s.MemPressurePct)
		} else {
			pressure = s.MemPressureLevel
		}
	}

	return fmt.Sprintf("%s / %s (Used: %.0f%%, Pressure: %s, Swap: %s)",
		bytesHuman(s.MemUsedBytes),
		bytesHuman(s.MemTotalBytes),
		s.MemUsedPct,
		pressure,
		bytesHuman(s.SwapUsedBytes),
	)
}

func (s Snapshot) DiskString() string {
	if s.DiskTotalBytes == 0 {
		return "n/a"
	}
	return fmt.Sprintf("%s / %s",
		bytesHuman(s.DiskUsedBytes),
		bytesHuman(s.DiskTotalBytes),
	)
}

func (s Snapshot) NetString() string {
	if s.NetIface == "" {
		return "n/a"
	}
	return fmt.Sprintf("%s  ↓ %s/s  ↑ %s/s",
		s.NetIface,
		bytesHuman(uint64(s.NetDownBps)),
		bytesHuman(uint64(s.NetUpBps)),
	)
}

func (s Snapshot) BatteryString() string {
	if s.BatteryState == "" && s.BatteryPercent == 0 {
		return "n/a"
	}
	eta := s.BatteryETA
	if eta == "" {
		eta = "—"
	}
	return fmt.Sprintf("%.0f%% (%s, %s)", s.BatteryPercent, s.BatteryState, eta)
}

func (s Snapshot) UptimeString() string {
	if s.UptimeSec == 0 {
		return "n/a"
	}
	d := time.Duration(s.UptimeSec) * time.Second
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	mins := int(d.Minutes()) % 60
	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, mins)
	}
	return fmt.Sprintf("%dh %dm", hours, mins)
}

func bytesHuman(b uint64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
		TB = 1024 * GB
	)

	switch {
	case b >= TB:
		return fmt.Sprintf("%.1f TB", float64(b)/float64(TB))
	case b >= GB:
		return fmt.Sprintf("%.1f GB", float64(b)/float64(GB))
	case b >= MB:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(MB))
	case b >= KB:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(KB))
	default:
		return fmt.Sprintf("%d B", b)
	}
}
