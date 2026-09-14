//go:build darwin

package stats

import (
	"strings"
	"testing"
)

func TestParsePMSetBattery(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		input        string
		wantPresent  bool
		wantPercent  float64
		wantState    string
		wantSource   string
		wantETA      int
		wantETAValid bool
	}{
		{
			name: "zero percent is a valid battery reading",
			input: `Now drawing from 'Battery Power'
 -InternalBattery-0 (id=1234567)\t0%; discharging; 0:12 remaining present: true
`,
			wantPresent: true, wantPercent: 0, wantState: "Discharging", wantSource: "Battery Power", wantETA: 12, wantETAValid: true,
		},
		{
			name: "charging ETA",
			input: `Now drawing from 'AC Power'
 -InternalBattery-0 (id=1234567)\t82%; charging; 1:05 remaining present: true
`,
			wantPresent: true, wantPercent: 82, wantState: "Charging", wantSource: "AC Power", wantETA: 65, wantETAValid: true,
		},
		{
			name: "not charging takes precedence over charging",
			input: `Now drawing from 'AC Power'
 -InternalBattery-0 (id=1234567)\t100%; not charging; (no estimate) present: true
`,
			wantPresent: true, wantPercent: 100, wantState: "Not Charging", wantSource: "AC Power",
		},
		{
			name: "charged without ETA",
			input: `Now drawing from 'AC Power'
 -InternalBattery-0 (id=1234567)\t100%; charged; 0:00 present: true
`,
			wantPresent: true, wantPercent: 100, wantState: "Charged", wantSource: "AC Power",
		},
		{
			name: "basking charge limit at 80%",
			input: `Now drawing from 'AC Power'
 -InternalBattery-0 (id=1234567)\t80%; basking; (no estimate) present: true
`,
			wantPresent: true, wantPercent: 80, wantState: "On Hold (80%)", wantSource: "AC Power",
		},
		{
			name: "inhibited charge limit",
			input: `Now drawing from 'AC Power'
 -InternalBattery-0 (id=1234567)\t80%; inhibited; (no estimate) present: true
`,
			wantPresent: true, wantPercent: 80, wantState: "Inhibited", wantSource: "AC Power",
		},
		{
			name:        "no battery desktop",
			input:       "Now drawing from 'AC Power'\nNo batteries are currently attached.\n",
			wantPresent: false, wantSource: "AC Power",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := parsePMSetBattery(tt.input)
			if err != nil {
				t.Fatalf("parsePMSetBattery returned error: %v", err)
			}
			if got.Present != tt.wantPresent || got.Percent != tt.wantPercent || got.State != tt.wantState || got.PowerSource != tt.wantSource {
				t.Fatalf("parsePMSetBattery = %+v, want present=%v percent=%v state=%q source=%q", got, tt.wantPresent, tt.wantPercent, tt.wantState, tt.wantSource)
			}
			if got.TimeRemainingAvailable != tt.wantETAValid || got.TimeRemainingMinutes != tt.wantETA {
				t.Fatalf("ETA = (%d, available=%v), want (%d, available=%v)", got.TimeRemainingMinutes, got.TimeRemainingAvailable, tt.wantETA, tt.wantETAValid)
			}
		})
	}
}

func TestParsePMSetBatteryRejectsMalformedInput(t *testing.T) {
	t.Parallel()

	tests := []string{
		"",
		"Now drawing from 'Battery Power'\n",
		"Now drawing from 'Battery Power'\n -InternalBattery-0; discharging; present: true\n",
		"Now drawing from 'Battery Power'\n -InternalBattery-0 101%; discharging; present: true\n",
		"Now drawing from 'Battery Power'\n present: true\n",
	}
	for _, input := range tests {
		input := input
		t.Run(strings.ReplaceAll(input, "\n", "_"), func(t *testing.T) {
			t.Parallel()
			if got, err := parsePMSetBattery(input); err == nil {
				t.Fatalf("parsePMSetBattery(%q) = %+v, want error", input, got)
			}
		})
	}
}

func TestParsePMSetBatteryIgnoresInvalidETA(t *testing.T) {
	t.Parallel()

	got, err := parsePMSetBattery(`Now drawing from 'Battery Power'
 -InternalBattery-0 (id=1234567) 50%; discharging; 1:60 remaining present: true
`)
	if err != nil {
		t.Fatalf("parsePMSetBattery returned error: %v", err)
	}
	if got.TimeRemainingAvailable || got.TimeRemainingMinutes != 0 {
		t.Fatalf("invalid ETA was accepted: %+v", got)
	}
}

func TestParseLowPowerModes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		input       string
		wantBattery bool
		wantAC      bool
	}{
		{
			name: "battery only",
			input: `Battery Power:
 lowpowermode         1
AC Power:
 lowpowermode         0
`,
			wantBattery: true,
		},
		{
			name: "AC only with unrelated settings",
			input: `Battery Power:
 lidwake              1
 lowpowermode         0
AC Power:
 displaysleep         10
 lowpowermode         1
`,
			wantAC: true,
		},
		{
			name:        "both",
			input:       "Battery Power:\n\tlowpowermode\t1\nAC Power:\n\tlowpowermode\t1\n",
			wantBattery: true, wantAC: true,
		},
		{name: "empty", input: ""},
		{name: "value other than one", input: "Battery Power:\n lowpowermode 2\nAC Power:\n lowpowermode -1\n"},
		{name: "setting before section", input: "lowpowermode 1\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			battery, ac := parseLowPowerModes(tt.input)
			if battery != tt.wantBattery || ac != tt.wantAC {
				t.Fatalf("parseLowPowerModes = (%v, %v), want (%v, %v)", battery, ac, tt.wantBattery, tt.wantAC)
			}
		})
	}
}

func TestParsePowerProfiler(t *testing.T) {
	t.Parallel()

	input := []byte(`{
  "SPPowerDataType": [
    {
      "_name": "spbattery_information",
      "sppower_battery_health_info": {
        "sppower_battery_cycle_count": 387,
        "sppower_battery_health": "Good\u001b",
        "sppower_battery_health_maximum_capacity": "89%"
      }
    },
    {
      "_name": "sppower_ac_charger_information",
      "sppower_ac_charger_watts": 96
    }
  ]
}`)
	var got batteryDetails
	if err := parsePowerProfiler(input, &got); err != nil {
		t.Fatalf("parsePowerProfiler returned error: %v", err)
	}
	if got.cycleCount != 387 || got.condition != "Good" || got.healthPercent != 89 || got.chargerWatts != 96 {
		t.Fatalf("parsePowerProfiler result = %+v", got)
	}

	if err := parsePowerProfiler([]byte("not-json"), &batteryDetails{}); err == nil {
		t.Fatal("parsePowerProfiler accepted malformed JSON")
	}
}
