//go:build darwin

package stats

import "testing"

func TestParseThermal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		input         string
		wantState     string
		wantCPU       int
		wantScheduler int
		wantLevel     int
	}{
		{
			name: "no recorded warning is nominal",
			input: `Note: No thermal warning level has been recorded
Note: No performance warning level has been recorded
Note: No CPU power status has been recorded
`,
			wantState: "Nominal",
		},
		{
			name: "numeric nominal",
			input: `CPU_Speed_Limit = 100
Scheduler_Limit = 100
Thermal_Level = 0
`,
			wantState: "Nominal", wantCPU: 100, wantScheduler: 100,
		},
		{
			name:      "CPU speed throttled",
			input:     "CPU_Speed_Limit = 75\nScheduler_Limit = 100\nThermal_Level = 0\n",
			wantState: "Throttled", wantCPU: 75, wantScheduler: 100,
		},
		{
			name:      "zero CPU limit is throttled",
			input:     "CPU_Speed_Limit = 0\nScheduler_Limit = 100\nThermal_Level = 0\n",
			wantState: "Throttled", wantScheduler: 100,
		},
		{
			name:      "scheduler throttled",
			input:     "CPU_Speed_Limit = 100\nScheduler_Limit = 80\nThermal_Level = 0\n",
			wantState: "Throttled", wantCPU: 100, wantScheduler: 80,
		},
		{
			name:      "elevated thermal level",
			input:     "CPU_Speed_Limit = 100\nScheduler_Limit = 100\nThermal_Level = 1\n",
			wantState: "Elevated", wantCPU: 100, wantScheduler: 100, wantLevel: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseThermal(tt.input)
			if err != nil {
				t.Fatalf("parseThermal returned error: %v", err)
			}
			if got.State != tt.wantState || got.CPUSpeedLimitPercent != tt.wantCPU || got.SchedulerLimitPercent != tt.wantScheduler || got.ThermalLevel != tt.wantLevel {
				t.Fatalf("parseThermal = %+v, want state=%q cpu=%d scheduler=%d level=%d", got, tt.wantState, tt.wantCPU, tt.wantScheduler, tt.wantLevel)
			}
		})
	}
}

func TestParseThermalRejectsEmptyOrUnrecognizedOutput(t *testing.T) {
	t.Parallel()

	for _, input := range []string{"", " \n\t", "pmset returned something new and unrecognized\n"} {
		if got, err := parseThermal(input); err == nil {
			t.Errorf("parseThermal(%q) = %+v, want error", input, got)
		}
	}
}
