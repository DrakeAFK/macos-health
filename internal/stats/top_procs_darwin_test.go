//go:build darwin

package stats

import (
	"math"
	"strings"
	"testing"
	"unicode"
)

func TestParseClock(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  float64
	}{
		{input: "12.5", want: 12.5},
		{input: "01:02", want: 62},
		{input: "01:02.50", want: 62.5},
		{input: "02:03:04", want: 7384},
		{input: "3-04:05:06", want: 3*86400 + 4*3600 + 5*60 + 6},
	}
	for _, tt := range tests {
		got, err := parseClock(tt.input)
		if err != nil {
			t.Errorf("parseClock(%q) returned error: %v", tt.input, err)
			continue
		}
		if got != tt.want {
			t.Errorf("parseClock(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestParseClockRejectsMalformedOrOutOfRangeValues(t *testing.T) {
	t.Parallel()

	inputs := []string{
		"",
		"one:two",
		"1:2:3:4",
		"-1",
		"1-2-03:04:05",
		"00:60",
		"01:60:00",
		"00:00:60",
		"1.5-00:00:00",
		"NaN",
		"+Inf",
	}
	for _, input := range inputs {
		got, err := parseClock(input)
		if err == nil {
			t.Errorf("parseClock(%q) = %v, want error", input, got)
		}
		if math.IsNaN(got) || math.IsInf(got, 0) {
			t.Errorf("parseClock(%q) leaked non-finite result %v", input, got)
		}
	}
}

func TestParseProcesses(t *testing.T) {
	t.Parallel()

	input := `
101 01:02 00:10.50 1.5 2048 /Applications/My App.app/Contents/MacOS/My App
bad row that is ignored
202 2-03:04:05 01:02:03 0.25 512 /usr/local/bin/worker
303 00:10 00:00.25 0.1 64 /tmp/evil` + "\x1b" + `[31m
`
	got, err := parseProcesses(input)
	if err != nil {
		t.Fatalf("parseProcesses returned error: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("parseProcesses returned %d rows, want 3: %+v", len(got), got)
	}
	if row := got[0]; row.row.PID != 101 || row.row.Name != "My App" || row.row.MemoryPercent != 1.5 || row.row.RSSBytes != 2048*1024 || row.cpuSeconds != 10.5 || row.elapsed != 62 {
		t.Errorf("first row = %+v", row)
	}
	if row := got[1]; row.row.PID != 202 || row.row.Name != "worker" || row.row.RSSBytes != 512*1024 || row.cpuSeconds != 3723 || row.elapsed != 2*86400+3*3600+4*60+5 {
		t.Errorf("second row = %+v", row)
	}
	if strings.IndexFunc(got[2].row.Name, unicode.IsControl) >= 0 {
		t.Errorf("process name contains controls: %q", got[2].row.Name)
	}
}

func TestParseProcessesRejectsInputWithoutValidRows(t *testing.T) {
	t.Parallel()

	for _, input := range []string{"", "header only\n", "abc 00:01 00:01 1 2 command\n", "1 invalid invalid 1 2 command\n"} {
		if got, err := parseProcesses(input); err == nil {
			t.Errorf("parseProcesses(%q) = %+v, want error", input, got)
		}
	}
}

func TestParseProcessesRejectsInvalidDomains(t *testing.T) {
	t.Parallel()

	inputs := []string{
		"0 00:01 00:00 1 2 process\n",
		"-7 00:01 00:00 1 2 process\n",
		"7 00:01 00:00 -1 2 process\n",
		"7 00:01 00:00 NaN 2 process\n",
	}
	for _, input := range inputs {
		if got, err := parseProcesses(input); err == nil {
			t.Errorf("parseProcesses(%q) = %+v, want error", input, got)
		}
	}
}
