package stats

import (
	"testing"
	"time"
)

func baselineHealthySnapshot() Snapshot {
	return Snapshot{
		CPU: CPUStats{Status: MetricStatus{Available: true}, UsagePercent: 20},
		Memory: MemoryStats{
			Status:            MetricStatus{Available: true},
			Pressure:          "Normal",
			PressureAvailable: true,
		},
		Disk:    DiskStats{Status: MetricStatus{Available: true}, UsedPercent: 50},
		Thermal: ThermalStats{Status: MetricStatus{Available: true}, State: "Nominal"},
	}
}

func snapshotWithCPU(percent float64) Snapshot {
	s := baselineHealthySnapshot()
	s.CPU.UsagePercent = percent
	return s
}

func healthHasIssue(health Health, code string) bool {
	for _, issue := range health.Issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}

func TestAssessHealthyBaseline(t *testing.T) {
	t.Parallel()

	got := Assess(baselineHealthySnapshot(), nil)
	if got.Status != "healthy" || len(got.Issues) != 0 {
		t.Fatalf("Assess healthy baseline = %+v", got)
	}
}

func TestAssessOrdersCriticalIssuesAheadOfWarnings(t *testing.T) {
	t.Parallel()

	s := baselineHealthySnapshot()
	s.Memory.Pressure = "Warning"
	s.Disk.UsedPercent = 97
	got := Assess(s, nil)
	if got.Status != "critical" {
		t.Fatalf("status = %q, want critical: %+v", got.Status, got)
	}
	if len(got.Issues) != 2 {
		t.Fatalf("issues = %+v, want disk and memory issues", got.Issues)
	}
	if got.Issues[0].Code != "disk-full" || got.Issues[0].Severity != SeverityCritical {
		t.Fatalf("first issue = %+v, want critical disk-full", got.Issues[0])
	}
	if got.Issues[1].Code != "memory-pressure" || got.Issues[1].Severity != SeverityWarning {
		t.Fatalf("second issue = %+v, want warning memory-pressure", got.Issues[1])
	}
}

func TestAssessMemoryThresholds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		configure  func(*Snapshot)
		wantStatus string
		wantCode   string
	}{
		{name: "normal", configure: func(s *Snapshot) { s.Memory.Pressure = "Normal" }, wantStatus: "healthy"},
		{name: "warning pressure", configure: func(s *Snapshot) { s.Memory.Pressure = "Warning" }, wantStatus: "warning", wantCode: "memory-pressure"},
		{name: "critical pressure", configure: func(s *Snapshot) { s.Memory.Pressure = "Critical" }, wantStatus: "critical", wantCode: "memory-pressure"},
		{name: "below pageout threshold", configure: func(s *Snapshot) {
			s.Memory.PageOutRateAvailable = true
			s.Memory.PageOutBytesPerSecond = 10*1024*1024 - 1
		}, wantStatus: "healthy"},
		{name: "at pageout threshold", configure: func(s *Snapshot) {
			s.Memory.PageOutRateAvailable = true
			s.Memory.PageOutBytesPerSecond = 10 * 1024 * 1024
		}, wantStatus: "warning", wantCode: "page-outs"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s := baselineHealthySnapshot()
			tt.configure(&s)
			got := Assess(s, nil)
			if got.Status != tt.wantStatus {
				t.Fatalf("status = %q, want %q; health=%+v", got.Status, tt.wantStatus, got)
			}
			if tt.wantCode != "" && !healthHasIssue(got, tt.wantCode) {
				t.Fatalf("missing issue %q: %+v", tt.wantCode, got.Issues)
			}
		})
	}
}

func TestAssessDiskThresholds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		used       float64
		wantStatus string
		wantCode   string
	}{
		{used: 89.999, wantStatus: "healthy"},
		{used: 90, wantStatus: "warning", wantCode: "disk-low"},
		{used: 96.999, wantStatus: "warning", wantCode: "disk-low"},
		{used: 97, wantStatus: "critical", wantCode: "disk-full"},
	}
	for _, tt := range tests {
		s := baselineHealthySnapshot()
		s.Disk.UsedPercent = tt.used
		got := Assess(s, nil)
		if got.Status != tt.wantStatus || (tt.wantCode != "" && !healthHasIssue(got, tt.wantCode)) {
			t.Errorf("used %.3f: health=%+v, want status=%q code=%q", tt.used, got, tt.wantStatus, tt.wantCode)
		}
	}
}

func TestAssessThermalThresholds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		state      string
		wantStatus string
	}{
		{state: "Nominal", wantStatus: "healthy"},
		{state: "Fair", wantStatus: "warning"},
		{state: "Elevated", wantStatus: "warning"},
		{state: "Serious", wantStatus: "warning"},
		{state: "Throttled", wantStatus: "critical"},
		{state: "Critical", wantStatus: "critical"},
	}
	for _, tt := range tests {
		s := baselineHealthySnapshot()
		s.Thermal.State = tt.state
		got := Assess(s, nil)
		if got.Status != tt.wantStatus {
			t.Errorf("thermal state %q: status=%q, want %q", tt.state, got.Status, tt.wantStatus)
		}
	}
}

func TestAssessBatteryThresholds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		battery    BatteryStats
		wantStatus string
		wantCode   string
	}{
		{name: "21 percent discharging", battery: BatteryStats{Present: true, Percent: 21, State: "Discharging"}, wantStatus: "healthy"},
		{name: "20 percent discharging", battery: BatteryStats{Present: true, Percent: 20, State: "Discharging"}, wantStatus: "warning", wantCode: "battery-low"},
		{name: "10 percent discharging", battery: BatteryStats{Present: true, Percent: 10, State: "Discharging"}, wantStatus: "critical", wantCode: "battery-low"},
		{name: "low but charging", battery: BatteryStats{Present: true, Percent: 5, State: "Charging"}, wantStatus: "healthy"},
		{name: "capacity at 80", battery: BatteryStats{Present: true, HealthPercent: 80, DetailsAvailable: true, Condition: "Good"}, wantStatus: "healthy"},
		{name: "capacity below 80", battery: BatteryStats{Present: true, HealthPercent: 79.9, DetailsAvailable: true, Condition: "Good"}, wantStatus: "warning", wantCode: "battery-health"},
		{name: "service condition", battery: BatteryStats{Present: true, HealthPercent: 90, DetailsAvailable: true, Condition: "Service Recommended"}, wantStatus: "warning", wantCode: "battery-service"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s := baselineHealthySnapshot()
			tt.battery.Status = MetricStatus{Available: true}
			s.Battery = tt.battery
			got := Assess(s, nil)
			if got.Status != tt.wantStatus || (tt.wantCode != "" && !healthHasIssue(got, tt.wantCode)) {
				t.Fatalf("health=%+v, want status=%q code=%q", got, tt.wantStatus, tt.wantCode)
			}
		})
	}
}

func TestAssessSustainedCPU(t *testing.T) {
	for _, interval := range []time.Duration{250 * time.Millisecond, time.Second, 2 * time.Second} {
		t.Run(interval.String(), func(t *testing.T) {
			start := time.Unix(1000, 0)
			history := []Snapshot{}
			for elapsed := time.Duration(0); elapsed <= 12*time.Second; elapsed += interval {
				s := snapshotWithCPU(95)
				s.SampledAt = start.Add(elapsed)
				got := healthHasIssue(Assess(s, history), "cpu-sustained")
				if got != (elapsed >= 10*time.Second) {
					t.Fatalf("elapsed %v issue=%v", elapsed, got)
				}
				history = append(history, s)
			}
			current := history[len(history)-1]
			history = history[:len(history)-1]
			history[len(history)-2].CPU.Status.Stale = true
			if healthHasIssue(Assess(current, history), "cpu-sustained") {
				t.Fatal("stale gap counted")
			}
			current.SampledAt = current.SampledAt.Add(time.Minute)
			if healthHasIssue(Assess(current, history), "cpu-sustained") {
				t.Fatal("sleep gap counted")
			}
		})
	}
}
func TestCriticalHealthSurvivesTelemetryFailure(t *testing.T) {
	s := baselineHealthySnapshot()
	s.Memory.Pressure = "Critical"
	s.CPU.Status.Available = false
	h := Assess(s, nil)
	if h.Status != "critical" || h.Confidence != "partial" {
		t.Fatalf("%+v", h)
	}
}

func TestAssessSamplingAndStaleCoreTelemetry(t *testing.T) {
	t.Parallel()

	sampling := baselineHealthySnapshot()
	sampling.CPU.Status.Available = false
	sampling.CPU.WarmingUp = true
	if got := Assess(sampling, nil); got.Status != "sampling" {
		t.Errorf("warming health status = %q, want sampling", got.Status)
	}

	stale := baselineHealthySnapshot()
	stale.Disk.Status.Stale = true
	got := Assess(stale, nil)
	if got.Status != "partial" || !healthHasIssue(got, "stale-telemetry") {
		t.Errorf("stale health = %+v, want partial with stale-telemetry issue", got)
	}

	missingPressure := baselineHealthySnapshot()
	missingPressure.Memory.PressureAvailable = false
	if got := Assess(missingPressure, nil); got.Status != "partial" {
		t.Errorf("missing memory-pressure status = %+v, want partial", got)
	}
}
