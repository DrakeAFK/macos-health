package stats

import (
	"errors"
	"strings"
	"testing"
	"time"
	"unicode"
)

func TestMergeLastGoodCarriesEveryFailedMetric(t *testing.T) {
	t.Parallel()

	oldAt := time.Date(2026, time.July, 9, 12, 0, 0, 0, time.UTC)
	newAt := oldAt.Add(time.Second)
	old := availableStatus(oldAt)
	previous := Snapshot{
		CPU:       CPUStats{Status: old, UsagePercent: 42},
		Memory:    MemoryStats{Status: old, UsedBytes: 2},
		Disk:      DiskStats{Status: old, FreeBytes: 3},
		Network:   NetworkStats{Status: old, Interface: "en0"},
		Battery:   BatteryStats{Status: old, Present: true, Percent: 80},
		Thermal:   ThermalStats{Status: old, State: "Nominal"},
		Processes: ProcessStats{Status: old, TopCPU: []ProcessRow{{PID: 7}}},
		Host:      HostInfo{Status: old, Chip: "Apple M3 Pro"},
		System:    SystemStats{Status: old, UptimeSeconds: 123},
	}
	failed := func(message string) MetricStatus {
		return MetricStatus{SampledAt: newAt, Error: message}
	}
	current := Snapshot{
		SampledAt: newAt,
		CPU:       CPUStats{Status: failed("cpu failed")},
		Memory:    MemoryStats{Status: failed("memory failed")},
		Disk:      DiskStats{Status: failed("disk failed")},
		Network:   NetworkStats{Status: failed("network failed")},
		Battery:   BatteryStats{Status: failed("battery failed")},
		Thermal:   ThermalStats{Status: failed("thermal failed")},
		Processes: ProcessStats{Status: failed("process failed")},
		Host:      HostInfo{Status: failed("host failed")},
		System:    SystemStats{Status: failed("system failed")},
	}

	got := MergeLastGood(current, previous)
	checks := []struct {
		name   string
		status MetricStatus
		value  bool
		err    string
	}{
		{name: "CPU", status: got.CPU.Status, value: got.CPU.UsagePercent == 42, err: "cpu failed"},
		{name: "memory", status: got.Memory.Status, value: got.Memory.UsedBytes == 2, err: "memory failed"},
		{name: "disk", status: got.Disk.Status, value: got.Disk.FreeBytes == 3, err: "disk failed"},
		{name: "network", status: got.Network.Status, value: got.Network.Interface == "en0", err: "network failed"},
		{name: "battery", status: got.Battery.Status, value: got.Battery.Percent == 80, err: "battery failed"},
		{name: "thermal", status: got.Thermal.Status, value: got.Thermal.State == "Nominal", err: "thermal failed"},
		{name: "processes", status: got.Processes.Status, value: len(got.Processes.TopCPU) == 1 && got.Processes.TopCPU[0].PID == 7, err: "process failed"},
		{name: "host", status: got.Host.Status, value: got.Host.Chip == "Apple M3 Pro", err: "host failed"},
		{name: "system", status: got.System.Status, value: got.System.UptimeSeconds == 123, err: "system failed"},
	}
	for _, check := range checks {
		if !check.value {
			t.Errorf("%s last-good value was not retained", check.name)
		}
		if !check.status.Available || !check.status.Stale {
			t.Errorf("%s status = %+v, want available and stale", check.name, check.status)
		}
		if !check.status.SampledAt.Equal(oldAt) {
			t.Errorf("%s SampledAt = %v, want original sample time %v", check.name, check.status.SampledAt, oldAt)
		}
		if check.status.Error != check.err {
			t.Errorf("%s error = %q, want latest failure %q", check.name, check.status.Error, check.err)
		}
	}
	if !got.SampledAt.Equal(newAt) {
		t.Errorf("snapshot SampledAt = %v, want %v", got.SampledAt, newAt)
	}
	if got.Health.Status != "partial" {
		t.Errorf("health status = %q, want partial for stale core metrics", got.Health.Status)
	}
}

func TestMergeLastGoodDoesNotReplaceSuccessfulCurrentMetric(t *testing.T) {
	t.Parallel()

	current := Snapshot{CPU: CPUStats{Status: MetricStatus{Available: true}, UsagePercent: 7}}
	previous := Snapshot{CPU: CPUStats{Status: MetricStatus{Available: true}, UsagePercent: 99}}
	got := MergeLastGood(current, previous)
	if got.CPU.UsagePercent != 7 || got.CPU.Status.Stale {
		t.Fatalf("successful current CPU metric was replaced: %+v", got.CPU)
	}
}

func TestSnapshotRedactedDoesNotMutateOriginal(t *testing.T) {
	t.Parallel()

	original := Snapshot{
		Host: HostInfo{ComputerName: "Drake's Mac", Hostname: "private.local"},
		Network: NetworkStats{Addresses: []IPAddr{
			{Interface: "en0", Address: "192.0.2.10"},
			{Interface: "utun3", Address: "2001:db8::10"},
		}},
	}
	redacted := original.Redacted()
	if redacted.Host.ComputerName != "redacted" || redacted.Host.Hostname != "redacted" {
		t.Fatalf("host identity not redacted: %+v", redacted.Host)
	}
	for _, address := range redacted.Network.Addresses {
		if address.Address != "redacted" {
			t.Errorf("network address was not redacted: %+v", address)
		}
	}
	if original.Host.ComputerName != "Drake's Mac" || original.Host.Hostname != "private.local" {
		t.Fatalf("Redacted mutated original host: %+v", original.Host)
	}
	wantAddresses := []string{"192.0.2.10", "2001:db8::10"}
	for i, want := range wantAddresses {
		if original.Network.Addresses[i].Address != want {
			t.Fatalf("Redacted mutated original address %d to %q, want %q", i, original.Network.Addresses[i].Address, want)
		}
	}
}

func TestCleanTextRemovesTerminalAndUnicodeControls(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "plain Unicode", input: "  München 💻  ", want: "München 💻"},
		{name: "ANSI SGR", input: "\x1b[31mDanger\x1b[0m", want: "[31mDanger[0m"},
		{name: "newlines and tabs", input: " hello\nworld\t", want: "helloworld"},
		{name: "C1 and delete", input: "\u009bvalue\u007f", want: "value"},
		{name: "bidirectional override", input: "safe\u202eevil", want: "safeevil"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := cleanText(tt.input)
			if got != tt.want {
				t.Fatalf("cleanText(%q) = %q, want %q", tt.input, got, tt.want)
			}
			if strings.IndexFunc(got, unicode.IsControl) >= 0 {
				t.Fatalf("cleanText left a control character in %q", got)
			}
		})
	}
}

func TestUnavailableStatusSanitizesError(t *testing.T) {
	t.Parallel()

	status := unavailableStatus(time.Time{}, errors.New("\x1b[31mfailed\nnow\x1b[0m"))
	if status.Error != "[31mfailednow[0m" {
		t.Fatalf("unavailableStatus error = %q", status.Error)
	}
}
