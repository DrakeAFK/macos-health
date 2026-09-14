package ui

import (
	"testing"
	"time"

	"github.com/drakeafk/macos-health/internal/stats"
)

func benchmarkSnapshot() stats.Snapshot {
	now := time.Now()
	avail := stats.MetricStatus{Available: true, SampledAt: now}
	return stats.Snapshot{
		SchemaVersion: 1,
		SampledAt:     now,
		CPU: stats.CPUStats{
			Status:         avail,
			UsagePercent:   32.5,
			PerCorePercent: []float64{40, 25, 10, 55, 30, 15, 60, 20},
			Load1:          2.1,
			Load5:          1.8,
			Load15:         1.5,
			LoadAvailable:  true,
		},
		Memory: stats.MemoryStats{
			Status:            avail,
			UsedBytes:         12 * 1024 * 1024 * 1024,
			TotalBytes:        18 * 1024 * 1024 * 1024,
			UsedPercent:       66.7,
			AvailablePercent:  33.3,
			Pressure:          "Normal",
			PressureAvailable: true,
			SwapUsedBytes:     512 * 1024 * 1024,
		},
		Disk: stats.DiskStats{
			Status:              avail,
			Mount:               "/System/Volumes/Data",
			Device:              "disk0",
			UsedBytes:           300 * 1024 * 1024 * 1024,
			FreeBytes:           200 * 1024 * 1024 * 1024,
			TotalBytes:          500 * 1024 * 1024 * 1024,
			UsedPercent:         60.0,
			ReadBytesPerSecond:  15 * 1024 * 1024,
			WriteBytesPerSecond: 5 * 1024 * 1024,
			IOAvailable:         true,
		},
		Network: stats.NetworkStats{
			Status:             avail,
			Interface:          "en0",
			DownBytesPerSecond: 2 * 1024 * 1024,
			UpBytesPerSecond:   300 * 1024,
			RatesAvailable:     true,
		},
		Battery: stats.BatteryStats{
			Status:                 avail,
			Present:                true,
			Percent:                85,
			State:                  "Discharging",
			PowerSource:            "Battery Power",
			TimeRemainingMinutes:   240,
			TimeRemainingAvailable: true,
			HealthPercent:          92,
			CycleCount:             150,
			DetailsAvailable:       true,
		},
		Thermal: stats.ThermalStats{
			Status: avail,
			State:  "Nominal",
		},
		Processes: stats.ProcessStats{
			Status: avail,
			TopCPU: []stats.ProcessRow{
				{PID: 101, Name: "Code Helper", CPUPercent: 45.0, MemoryPercent: 2.1, RSSBytes: 400 * 1024 * 1024},
				{PID: 202, Name: "Browser", CPUPercent: 22.0, MemoryPercent: 4.5, RSSBytes: 800 * 1024 * 1024},
				{PID: 303, Name: "WindowServer", CPUPercent: 12.0, MemoryPercent: 1.5, RSSBytes: 250 * 1024 * 1024},
			},
			TopMemory: []stats.ProcessRow{
				{PID: 202, Name: "Browser", CPUPercent: 22.0, MemoryPercent: 4.5, RSSBytes: 800 * 1024 * 1024},
				{PID: 101, Name: "Code Helper", CPUPercent: 45.0, MemoryPercent: 2.1, RSSBytes: 400 * 1024 * 1024},
			},
		},
		Host: stats.HostInfo{
			Status:           avail,
			ComputerName:     "Test Mac",
			Hostname:         "test-mac.local",
			ModelIdentifier:  "Mac14,7",
			Chip:             "Apple M2",
			LogicalCores:     8,
			PerformanceCores: 4,
			EfficiencyCores:  4,
			OSVersion:        "14.5",
			OSBuild:          "23F79",
		},
		System: stats.SystemStats{
			Status:        avail,
			UptimeSeconds: 86400 * 5,
		},
		Health: stats.Health{
			Status:  "healthy",
			Summary: "Your Mac looks healthy",
		},
	}
}

func BenchmarkRenderOverview(b *testing.B) {
	snap := benchmarkSnapshot()
	data := Data{
		Snapshot: snap,
		Width:    80,
		Height:   24,
		Page:     PageOverview,
		Now:      snap.SampledAt,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Render(data)
	}
}

func BenchmarkRenderProcesses(b *testing.B) {
	snap := benchmarkSnapshot()
	data := Data{
		Snapshot:    snap,
		Width:       80,
		Height:      24,
		Page:        PageProcesses,
		ProcessSort: SortCPU,
		Now:         snap.SampledAt,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Render(data)
	}
}

func BenchmarkRenderText(b *testing.B) {
	snap := benchmarkSnapshot()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = RenderText(snap)
	}
}
