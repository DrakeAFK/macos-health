package ui

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/drakeafk/macos-health/internal/stats"
)

func TestRenderSizeInvariant(t *testing.T) {
	snapshot, history := fixture()
	sizes := []struct {
		width  int
		height int
	}{
		{40, 12},
		{60, 18},
		{80, 24},
		{100, 30},
		{110, 30},
		{160, 50},
	}
	pages := []Page{PageOverview, PageProcesses, PageBattery, PageSystem}
	modes := []struct {
		name    string
		plain   bool
		noColor bool
		ascii   bool
	}{
		{"default", false, false, false},
		{"no-color", false, true, false},
		{"ascii", false, false, true},
		{"plain", true, false, false},
	}

	for _, size := range sizes {
		for _, page := range pages {
			for _, mode := range modes {
				name := mode.name + "/" + pageName(page)
				t.Run(name, func(t *testing.T) {
					output := Render(Data{
						Snapshot:    snapshot,
						History:     history,
						Width:       size.width,
						Height:      size.height,
						Page:        page,
						ProcessSort: SortCPU,
						Now:         snapshot.SampledAt.Add(time.Second),
						Plain:       mode.plain,
						NoColor:     mode.noColor,
						ASCII:       mode.ascii,
					})
					assertFrameSize(t, output, size.width, size.height)
					if strings.TrimSpace(ansi.Strip(output)) == "" {
						t.Fatal("rendered frame is empty")
					}
				})
			}
		}
	}
}

func TestRenderSmallOverviewIsUseful(t *testing.T) {
	snapshot, history := fixture()
	output := ansi.Strip(Render(Data{
		Snapshot: snapshot,
		History:  history,
		Width:    40,
		Height:   12,
		Page:     PageOverview,
		Now:      snapshot.SampledAt,
		Plain:    true,
	}))
	for _, wanted := range []string{"[WARN]", "Memory pressure", "CPU", "MEM", "BAT", "DISK", "NET", "CULPRITS"} {
		if !strings.Contains(output, wanted) {
			t.Fatalf("small overview missing %q:\n%s", wanted, output)
		}
	}
	if !strings.Contains(output, "132%") {
		t.Fatalf("compact culprit row lost its value:\n%s", output)
	}
}

func TestRenderWideOverviewUsesColumns(t *testing.T) {
	snapshot, history := fixture()
	output := Render(Data{
		Snapshot: snapshot,
		History:  history,
		Width:    110,
		Height:   30,
		Page:     PageOverview,
		Now:      snapshot.SampledAt,
		Plain:    true,
	})
	if !strings.Contains(output, "PERFORMANCE") || !strings.Contains(output, "MEMORY & POWER") || !strings.Contains(output, " | ") {
		t.Fatalf("wide overview did not use two-column layout:\n%s", output)
	}
	alignedTables := false
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, "TOP CPU") && strings.Contains(line, "TOP MEMORY") {
			alignedTables = true
			break
		}
	}
	if !alignedTables {
		t.Fatalf("wide process tables are not aligned:\n%s", output)
	}
}

func TestRenderProcessSortMemory(t *testing.T) {
	snapshot, history := fixture()
	output := ansi.Strip(Render(Data{
		Snapshot:    snapshot,
		History:     history,
		Width:       80,
		Height:      24,
		Page:        PageProcesses,
		ProcessSort: SortMemory,
		Now:         snapshot.SampledAt,
		Plain:       true,
	}))
	if !strings.Contains(output, "PROCESSES / MEMORY") {
		t.Fatalf("sort label missing:\n%s", output)
	}
	if strings.Index(output, "Brave") > strings.Index(output, "Xcode") {
		t.Fatalf("memory rows are not sorted by RSS:\n%s", output)
	}
}

func TestRenderSanitizesAndTruncatesDynamicText(t *testing.T) {
	snapshot, history := fixture()
	snapshot.Processes.TopCPU[0].Name = "悪い\x1b[31m\nprocess-very-very-long"
	snapshot.Health.Summary = "warning\r\nspoof\u202edesrever"
	output := Render(Data{
		Snapshot: snapshot,
		History:  history,
		Width:    40,
		Height:   12,
		Page:     PageOverview,
		Now:      snapshot.SampledAt,
		Plain:    true,
		Error:    errors.New("failed\nforged line"),
	})
	assertFrameSize(t, output, 40, 12)
	if strings.Contains(output, "\x1b") || strings.Contains(output, "\u202e") || strings.Contains(output, "forged\n") {
		t.Fatalf("dynamic control characters reached output: %q", output)
	}
}

func TestRenderErrorCannotLookHealthy(t *testing.T) {
	snapshot, history := fixture()
	snapshot.Health.Status = "healthy"
	snapshot.Health.Summary = "Your Mac looks healthy"
	output := ansi.Strip(Render(Data{
		Snapshot: snapshot,
		History:  history,
		Width:    80,
		Height:   24,
		Now:      snapshot.SampledAt,
		Plain:    true,
		Error:    errors.New("collector timed out"),
	}))
	if !strings.Contains(output, "[PART]") || strings.Contains(output, "[OK]") {
		t.Fatalf("collector error rendered as healthy:\n%s", output)
	}
}

func TestStaleMetricsAreMarkedAndExcludedFromHistory(t *testing.T) {
	snapshot, history := fixture()
	snapshot.Memory.Status.Stale = true
	history[0].CPU.Status.Stale = true
	if got := len(historyCPU(history)); got != len(history)-1 {
		t.Fatalf("historyCPU retained stale point: got %d points", got)
	}
	output := ansi.Strip(Render(Data{
		Snapshot: snapshot,
		History:  history,
		Width:    80,
		Height:   24,
		Now:      snapshot.SampledAt,
		Plain:    true,
	}))
	if !strings.Contains(output, "MEM") || !strings.Contains(output, "stale") {
		t.Fatalf("stale memory value was not labeled:\n%s", output)
	}
}

func TestBinaryUnitLabels(t *testing.T) {
	if got := bytes(1024); got != "1 KiB" {
		t.Fatalf("bytes(1024) = %q, want binary unit", got)
	}
}

func TestRenderTextIsPlainAndComplete(t *testing.T) {
	snapshot, _ := fixture()
	output := RenderText(snapshot)
	if strings.Contains(output, "\x1b") {
		t.Fatal("text output contains ANSI escape sequences")
	}
	for _, wanted := range []string{"macos-health: WARNING", "CPU:", "Memory:", "Battery:", "System:", "Top CPU processes:", "Top memory processes:"} {
		if !strings.Contains(output, wanted) {
			t.Fatalf("text output missing %q:\n%s", wanted, output)
		}
	}
}

func TestRenderNonPositiveSizeIsEmpty(t *testing.T) {
	if got := Render(Data{}); got != "" {
		t.Fatalf("zero-size render = %q, want empty", got)
	}
}

func assertFrameSize(t *testing.T, output string, width, height int) {
	t.Helper()
	lines := strings.Split(output, "\n")
	if len(lines) > height {
		t.Fatalf("frame height %d exceeds %d", len(lines), height)
	}
	for i, line := range lines {
		if got := ansi.StringWidth(line); got > width {
			t.Fatalf("line %d width %d exceeds %d: %q", i+1, got, width, ansi.Strip(line))
		}
	}
}

func pageName(page Page) string {
	switch page {
	case PageProcesses:
		return "processes"
	case PageBattery:
		return "battery"
	case PageSystem:
		return "system"
	default:
		return "overview"
	}
}

func fixture() (stats.Snapshot, []stats.Snapshot) {
	now := time.Date(2026, time.July, 9, 21, 30, 0, 0, time.UTC)
	available := stats.MetricStatus{Available: true, SampledAt: now}
	processes := []stats.ProcessRow{
		{PID: 101, Name: "Xcode Helper Renderer with a long name", CPUPercent: 132, MemoryPercent: 4.2, RSSBytes: 780 * 1024 * 1024},
		{PID: 202, Name: "Brave Browser Helper", CPUPercent: 48, MemoryPercent: 8.8, RSSBytes: 1400 * 1024 * 1024},
		{PID: 303, Name: "日本語プロセス", CPUPercent: 12, MemoryPercent: 1.5, RSSBytes: 220 * 1024 * 1024},
	}
	snapshot := stats.Snapshot{
		SampledAt: now,
		CPU: stats.CPUStats{
			Status:         available,
			UsagePercent:   47,
			PerCorePercent: []float64{20, 40, 60, 80, 25, 30, 12, 45, 67, 33, 52},
			Load1:          4.2,
			Load5:          3.8,
			Load15:         2.9,
			LoadAvailable:  true,
		},
		Memory: stats.MemoryStats{
			Status:               available,
			UsedBytes:            14 * 1024 * 1024 * 1024,
			TotalBytes:           18 * 1024 * 1024 * 1024,
			UsedPercent:          78,
			AvailablePercent:     41,
			Pressure:             "Warning",
			PressureAvailable:    true,
			CompressedBytes:      2 * 1024 * 1024 * 1024,
			CompressionAvailable: true,
			SwapUsedBytes:        1700 * 1024 * 1024,
		},
		Disk: stats.DiskStats{
			Status: available, Mount: "/System/Volumes/Data", Device: "disk0",
			UsedBytes: 330 * 1024 * 1024 * 1024, FreeBytes: 130 * 1024 * 1024 * 1024,
			TotalBytes: 460 * 1024 * 1024 * 1024, UsedPercent: 71.7,
			ReadBytesPerSecond: 3 * 1024 * 1024, WriteBytesPerSecond: 512 * 1024, IOAvailable: true,
		},
		Network: stats.NetworkStats{
			Status:             available,
			Interface:          "en0",
			DownBytesPerSecond: 224 * 1024,
			UpBytesPerSecond:   9 * 1024,
			RatesAvailable:     true,
			Addresses:          []stats.IPAddr{{Interface: "en0", Address: "192.168.1.22"}, {Interface: "utun10", Address: "fd7a:115c:a1e0::1234:5678"}},
		},
		Battery: stats.BatteryStats{
			Status:                 available,
			Present:                true,
			Percent:                82,
			State:                  "Discharging",
			PowerSource:            "Battery Power",
			TimeRemainingMinutes:   252,
			TimeRemainingAvailable: true,
			HealthPercent:          89,
			Condition:              "Good",
			CycleCount:             387,
			TemperatureC:           30.1,
			TemperatureAvailable:   true,
			DetailsAvailable:       true,
		},
		Thermal: stats.ThermalStats{Status: available, State: "Normal"},
		Processes: stats.ProcessStats{
			Status:    available,
			TopCPU:    processes,
			TopMemory: []stats.ProcessRow{processes[1], processes[0], processes[2]},
		},
		Host: stats.HostInfo{
			Status:           available,
			ComputerName:     "Drake's MacBook Pro",
			Hostname:         "macbook.local",
			MachineName:      "MacBook Pro",
			ModelIdentifier:  "Mac15,6",
			OSVersion:        "15.7.3",
			OSBuild:          "24G419",
			Architecture:     "arm64",
			Chip:             "Apple M3 Pro",
			LogicalCores:     11,
			PerformanceCores: 5,
			EfficiencyCores:  6,
			GPU:              "Apple M3 Pro",
			GPUCores:         14,
		},
		System: stats.SystemStats{Status: available, UptimeSeconds: 67*86400 + 22*3600},
		Health: stats.Health{
			Status:  "warning",
			Summary: "Memory pressure is elevated",
			Issues:  []stats.Issue{{Severity: stats.SeverityWarning, Code: "memory-pressure", Title: "Memory pressure is elevated", Detail: "Check memory-heavy processes."}},
		},
	}

	history := make([]stats.Snapshot, 12)
	for i := range history {
		history[i] = stats.Snapshot{
			CPU:     stats.CPUStats{Status: available, UsagePercent: float64(10 + i*5)},
			Memory:  stats.MemoryStats{Status: available, UsedPercent: float64(60 + i)},
			Battery: stats.BatteryStats{Status: available, Present: true, Percent: float64(93 - i)},
		}
	}
	return snapshot, history
}
