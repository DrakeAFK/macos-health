//go:build darwin

package stats

import (
	"testing"
	"time"
)

func TestDiskIORates(t *testing.T) {
	start := time.Unix(100, 0)
	read, write, ok := diskIORates(
		diskTotals{read: 1000, write: 2000, at: start},
		diskTotals{read: 3000, write: 5000, at: start.Add(2 * time.Second)},
	)
	if !ok || read != 1000 || write != 1500 {
		t.Fatalf("diskIORates = (%v, %v, %v)", read, write, ok)
	}
}

func TestDiskIORatesRejectsWarmupResetAndLongGap(t *testing.T) {
	start := time.Unix(100, 0)
	tests := []struct {
		name     string
		previous diskTotals
		current  diskTotals
	}{
		{"warmup", diskTotals{}, diskTotals{read: 1, write: 1, at: start}},
		{"read reset", diskTotals{read: 2, write: 1, at: start}, diskTotals{read: 1, write: 2, at: start.Add(time.Second)}},
		{"write reset", diskTotals{read: 1, write: 2, at: start}, diskTotals{read: 2, write: 1, at: start.Add(time.Second)}},
		{"long gap", diskTotals{read: 1, write: 1, at: start}, diskTotals{read: 2, write: 2, at: start.Add(30 * time.Second)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, _, ok := diskIORates(tt.previous, tt.current); ok {
				t.Fatal("diskIORates accepted an invalid sample")
			}
		})
	}
}
