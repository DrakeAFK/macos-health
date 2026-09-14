//go:build darwin

package stats

import (
	"testing"

	"github.com/shirou/gopsutil/v3/cpu"
)

func TestCPUDelta(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		prev    cpu.TimesStat
		current cpu.TimesStat
		want    float64
	}{
		{
			name:    "thirty percent busy",
			prev:    cpu.TimesStat{User: 10, System: 10, Idle: 80},
			current: cpu.TimesStat{User: 30, System: 20, Idle: 150},
			want:    30,
		},
		{
			name:    "fully idle",
			prev:    cpu.TimesStat{User: 10, Idle: 90},
			current: cpu.TimesStat{User: 10, Idle: 190},
			want:    0,
		},
		{
			name:    "fully busy",
			prev:    cpu.TimesStat{User: 10, Idle: 90},
			current: cpu.TimesStat{User: 110, Idle: 90},
			want:    100,
		},
		{
			name:    "all busy categories participate",
			prev:    cpu.TimesStat{},
			current: cpu.TimesStat{User: 1, System: 1, Nice: 1, Iowait: 1, Irq: 1, Softirq: 1, Steal: 1, Guest: 1, GuestNice: 1, Idle: 1},
			want:    90,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := cpuDelta(tt.prev, tt.current)
			if !ok {
				t.Fatal("cpuDelta reported an invalid delta")
			}
			if got != tt.want {
				t.Fatalf("cpuDelta = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCPUDeltaRejectsCounterResetOrInvalidInterval(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		prev    cpu.TimesStat
		current cpu.TimesStat
	}{
		{name: "no advance", prev: cpu.TimesStat{User: 10, Idle: 90}, current: cpu.TimesStat{User: 10, Idle: 90}},
		{name: "full reset", prev: cpu.TimesStat{User: 20, Idle: 80}, current: cpu.TimesStat{User: 2, Idle: 8}},
		{name: "idle counter regressed", prev: cpu.TimesStat{User: 10, Idle: 90}, current: cpu.TimesStat{User: 110, Idle: 80}},
		{name: "idle delta exceeds total delta", prev: cpu.TimesStat{User: 10, Idle: 90}, current: cpu.TimesStat{User: 5, Idle: 100}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got, ok := cpuDelta(tt.prev, tt.current); ok {
				t.Fatalf("cpuDelta = (%v, true), want invalid delta", got)
			}
		})
	}
}
