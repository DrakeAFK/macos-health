package ui

import (
	"github.com/drakeafk/macos-health/internal/stats"
	"strings"
	"testing"
	"time"
)

func TestAllDashboardPagesAndModesFit(t *testing.T) {
	s, h := fixture()
	s.Processes.All = append([]stats.ProcessRow(nil), s.Processes.TopCPU...)
	s.Silicon = stats.SiliconStats{Enabled: true, Status: stats.MetricStatus{Available: true}, GPUPercent: stats.SensorValue{Available: true, Value: 30}, FanRPM: []float64{1000, 2000}}
	s.Services = stats.ServicesStats{Enabled: true, Status: stats.MetricStatus{Available: true}, Listeners: []stats.Listener{{PID: 1, Process: "node", Address: "127.0.0.1:3000"}}}
	for _, size := range [][2]int{{1, 1}, {12, 4}, {40, 12}, {80, 24}, {120, 40}, {180, 60}} {
		for page := PageOverview; page < PageCount; page++ {
			for _, theme := range []string{"ocean", "amber", "violet"} {
				for _, group := range []bool{false, true} {
					d := Data{Snapshot: s, History: h, Width: size[0], Height: size[1], Page: page, Theme: theme, Group: group, Window: time.Minute, Query: "", ProcessOffset: 100000, Notice: "ready"}
					assertFrameSize(t, Render(d), size[0], size[1])
					d.Detail = true
					d.ProcessHistory = []ProcessPoint{{At: s.SampledAt, Row: s.Processes.All[0]}}
					assertFrameSize(t, Render(d), size[0], size[1])
				}
			}
		}
	}
}
func TestProcessSearchHidesUnmatchedRows(t *testing.T) {
	s, _ := fixture()
	s.Processes.All = s.Processes.TopCPU
	out := Render(Data{Snapshot: s, Page: PageProcesses, Query: "Brave", Width: 80, Height: 24, Plain: true})
	if !strings.Contains(out, "Brave") || strings.Contains(out, "Xcode") {
		t.Fatal(out)
	}
}
func BenchmarkRenderHourHistory(b *testing.B) {
	s := benchmarkSnapshot()
	history := make([]stats.Snapshot, 3600)
	for i := range history {
		history[i] = stats.Trend(s)
		history[i].SampledAt = s.SampledAt.Add(time.Duration(i-3599) * time.Second)
	}
	d := Data{Snapshot: s, History: history, Page: PageHistory, Width: 120, Height: 40, Window: time.Hour, Plain: true}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Render(d)
	}
}
