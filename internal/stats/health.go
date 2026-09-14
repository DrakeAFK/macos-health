package stats

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// Assess turns raw readings into a small, explainable health result. It avoids
// a synthetic score: every warning has a concrete source and recommendation.
func Assess(current Snapshot, history []Snapshot) Health {
	issues := make([]Issue, 0, 6)
	missing := 0
	stale := 0

	for _, status := range []MetricStatus{
		current.CPU.Status,
		current.Memory.Status,
		current.Disk.Status,
	} {
		if !status.Available {
			missing++
		} else if status.Stale {
			stale++
		}
	}
	if current.Memory.Status.Available && !current.Memory.PressureAvailable {
		missing++
	}

	if current.Memory.Status.Available && current.Memory.PressureAvailable {
		switch strings.ToLower(current.Memory.Pressure) {
		case "critical":
			issues = append(issues, Issue{SeverityCritical, "memory-pressure", "Memory pressure is critical", "Close a memory-heavy app; macOS is under active memory pressure."})
		case "warning", "warn":
			issues = append(issues, Issue{SeverityWarning, "memory-pressure", "Memory pressure is elevated", "Check the memory process list before the system starts paging heavily."})
		}
	}
	if current.Memory.PageOutRateAvailable && current.Memory.PageOutBytesPerSecond >= 10*1024*1024 {
		issues = append(issues, Issue{SeverityWarning, "page-outs", "Memory is paging to disk", fmt.Sprintf("Page-outs are running at %.0f MiB/s.", current.Memory.PageOutBytesPerSecond/(1024*1024))})
	}

	if current.Disk.Status.Available {
		switch {
		case current.Disk.UsedPercent >= 97:
			issues = append(issues, Issue{SeverityCritical, "disk-full", "Internal storage is almost full", "Free space now; macOS needs working room for swap, updates, and APFS snapshots."})
		case current.Disk.UsedPercent >= 90:
			issues = append(issues, Issue{SeverityWarning, "disk-low", "Internal storage is running low", "Aim to keep at least 10% of the APFS container free."})
		}
	}

	if current.Thermal.Status.Available {
		switch strings.ToLower(current.Thermal.State) {
		case "critical", "throttled":
			issues = append(issues, Issue{SeverityCritical, "thermal", "Thermal throttling is active", "Reduce sustained load and make sure the Mac can dissipate heat."})
		case "serious", "elevated", "fair":
			issues = append(issues, Issue{SeverityWarning, "thermal", "Thermal pressure is elevated", "Performance may be limited until the Mac cools down."})
		}
	}

	if current.Battery.Status.Available && current.Battery.Present {
		condition := strings.ToLower(current.Battery.Condition)
		if condition != "" && condition != "normal" && condition != "good" {
			issues = append(issues, Issue{SeverityWarning, "battery-service", "Battery service may be needed", "macOS reports the battery condition as " + current.Battery.Condition + "."})
		} else if current.Battery.DetailsAvailable && current.Battery.HealthPercent > 0 && current.Battery.HealthPercent < 80 {
			issues = append(issues, Issue{SeverityWarning, "battery-health", "Battery capacity is below 80%", "Runtime is reduced; review Battery Health in System Settings."})
		}
		if strings.EqualFold(current.Battery.State, "Discharging") {
			switch {
			case current.Battery.Percent <= 10:
				issues = append(issues, Issue{SeverityCritical, "battery-low", "Battery is critically low", "Connect a charger soon."})
			case current.Battery.Percent <= 20:
				issues = append(issues, Issue{SeverityWarning, "battery-low", "Battery is low", "Connect a charger if you need uninterrupted runtime."})
			}
		}
	}

	if sustainedCPU(current, history, 10*time.Second) {
		issues = append(issues, Issue{SeverityWarning, "cpu-sustained", "CPU usage has stayed above 90% for 10 seconds", "Open Processes to identify the workload using the most CPU."})
	}

	if stale > 0 {
		issues = append(issues, Issue{SeverityInfo, "stale-telemetry", "Some readings are stale", "The last known values are shown while telemetry recovers."})
	}

	sort.SliceStable(issues, func(i, j int) bool {
		return severityRank(issues[i].Severity) > severityRank(issues[j].Severity)
	})

	health := Health{Issues: issues, Confidence: "complete"}
	if missing > 0 || stale > 0 {
		health.Confidence = "partial"
	}
	switch {
	case len(issues) > 0 && issues[0].Severity == SeverityCritical:
		health.Status = "critical"
		health.Summary = issues[0].Title
	case len(issues) > 0 && issues[0].Severity == SeverityWarning:
		health.Status = "warning"
		health.Summary = issues[0].Title
	case missing > 0 && current.CPU.WarmingUp:
		health.Status = "sampling"
		health.Summary = "Sampling system activity"
	case missing > 0:
		health.Status = "partial"
		health.Summary = "Some core telemetry is unavailable"
	case stale > 0:
		health.Status = "partial"
		health.Summary = "Showing last known telemetry"
	default:
		health.Status = "healthy"
		health.Summary = "Your Mac looks healthy"
	}
	return health
}

// Require continuous fresh readings over wall time. A long sleep/pause gap
// breaks the run; rapid manual refreshes cannot manufacture a sustained load.
func sustainedCPU(current Snapshot, history []Snapshot, window time.Duration) bool {
	valid := func(s Snapshot) bool {
		return s.CPU.Status.Available && !s.CPU.Status.Stale && !s.CPU.WarmingUp && s.CPU.UsagePercent >= 90 && !s.SampledAt.IsZero()
	}
	if !valid(current) {
		return false
	}
	end, previous := current.SampledAt, current.SampledAt
	for i := len(history) - 1; i >= 0; i-- {
		s := history[i]
		if !valid(s) || !s.SampledAt.Before(previous) || previous.Sub(s.SampledAt) > 3*time.Second {
			return false
		}
		if end.Sub(s.SampledAt) >= window {
			return true
		}
		previous = s.SampledAt
	}
	return false
}

func severityRank(s Severity) int {
	switch s {
	case SeverityCritical:
		return 3
	case SeverityWarning:
		return 2
	default:
		return 1
	}
}
