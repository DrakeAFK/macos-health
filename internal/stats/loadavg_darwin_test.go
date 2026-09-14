//go:build darwin

package stats

import (
	"math"
	"strings"
	"testing"
)

func TestParseLoadAvg(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		input  string
		want1  float64
		want5  float64
		want15 float64
	}{
		{name: "sysctl braces", input: "{ 1.25 2.5 3.75 }", want1: 1.25, want5: 2.5, want15: 3.75},
		{name: "bare values", input: "0 0.125 12", want1: 0, want5: 0.125, want15: 12},
		{name: "surrounding whitespace", input: "\n  {0.01\t0.02  0.03}\n", want1: 0.01, want5: 0.02, want15: 0.03},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got1, got5, got15, err := parseLoadAvg(tt.input)
			if err != nil {
				t.Fatalf("parseLoadAvg(%q) returned error: %v", tt.input, err)
			}
			if got1 != tt.want1 || got5 != tt.want5 || got15 != tt.want15 {
				t.Fatalf("parseLoadAvg(%q) = (%v, %v, %v), want (%v, %v, %v)", tt.input, got1, got5, got15, tt.want1, tt.want5, tt.want15)
			}
		})
	}
}

func TestParseLoadAvgRejectsMalformedValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
	}{
		{name: "empty", input: ""},
		{name: "empty braces", input: "{}"},
		{name: "too few", input: "1 2"},
		{name: "too many", input: "1 2 3 4"},
		{name: "non numeric first", input: "busy 2 3"},
		{name: "non numeric middle", input: "1 nope 3"},
		{name: "non numeric last", input: "1 2 nope"},
		{name: "NaN", input: "NaN 1 2"},
		{name: "positive infinity", input: "+Inf 1 2"},
		{name: "negative", input: "-0.1 1 2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got1, got5, got15, err := parseLoadAvg(tt.input)
			if err == nil {
				t.Fatalf("parseLoadAvg(%q) = (%v, %v, %v), want an error", tt.input, got1, got5, got15)
			}
			if math.IsNaN(got1) || math.IsNaN(got5) || math.IsNaN(got15) {
				t.Fatalf("parseLoadAvg(%q) leaked NaN values on failure", tt.input)
			}
			if strings.TrimSpace(err.Error()) == "" {
				t.Fatal("parseLoadAvg returned an empty error")
			}
		})
	}
}
