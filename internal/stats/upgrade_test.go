package stats

import (
	"strings"
	"testing"
	"time"
)

func TestProcessFilterAndGrouping(t *testing.T) {
	p := ProcessStats{All: []ProcessRow{{PID: 12, Name: "helper", App: "Editor", CPUAvailable: true, CPUPercent: 25, RSSBytes: 100}, {PID: 13, Name: "node", App: "Editor", Kind: "Dev", CPUAvailable: true, CPUPercent: 10, RSSBytes: 200}, {PID: 14, Name: "ollama", App: "ollama", Kind: "AI", CPUAvailable: true, CPUPercent: 50, RSSBytes: 300}}}
	for query, n := range map[string]int{"editor": 2, "13": 1, "AI": 1, "missing": 0, "": 3} {
		if got := ProcessRows(p, query, "cpu"); len(got) != n {
			t.Fatalf("%s: %v", query, got)
		}
	}
	sorted := ProcessRows(p, "", "memory")
	if sorted[0].PID != 14 || p.All[0].PID != 12 {
		t.Fatal("sort incorrect or mutated source")
	}
	groups := GroupProcesses(p.All)
	for _, g := range groups {
		if g.Name == "Editor" && (g.Count != 2 || g.CPUPercent != 35 || g.RSSBytes != 300) {
			t.Fatalf("group=%+v", g)
		}
	}
	if processApp("/Applications/My App.app/Contents/Frameworks/Helper.app/Contents/MacOS/helper") != "My App" {
		t.Fatal("outer application lost")
	}
}
func TestTimelineTransitions(t *testing.T) {
	first := baselineHealthySnapshot()
	first.SampledAt = time.Now()
	first.Health = Assess(first, nil)
	next := first
	next.Memory.Pressure = "Critical"
	next.Health = Assess(next, nil)
	if len(Events(first, next)) != 1 || len(Events(next, next)) != 0 {
		t.Fatal("events are duplicated or missing")
	}
	if got := Events(next, first); len(got) != 1 || !got[0].Resolved {
		t.Fatalf("resolution=%+v", got)
	}
	thermal := first
	thermal.Health = Health{Confidence: "complete", Issues: []Issue{{Code: "thermal", Title: "Hot", Severity: SeverityCritical}}}
	absent := first
	absent.Thermal.Status.Available = false
	if len(Events(thermal, absent)) != 0 {
		t.Fatal("unavailable thermal reading cleared warning")
	}
}
func TestPrometheusOmitsUnavailableValues(t *testing.T) {
	s := baselineHealthySnapshot()
	s.CPU.Status.Stale = true
	s.Memory.Status.Available = false
	text := Prometheus(s)
	if strings.Contains(text, "macos_health_cpu_percent ") || strings.Contains(text, "macos_health_memory_used_bytes ") {
		t.Fatal("stale or missing readings exported as values")
	}
	if !strings.Contains(text, "macos_health_cpu_available 0") {
		t.Fatal("capability missing")
	}
}
func TestTrendDropsHeavyAndPrivateFields(t *testing.T) {
	s := Snapshot{Processes: ProcessStats{All: []ProcessRow{{PID: 1}}}, Host: HostInfo{Hostname: "private"}, Services: ServicesStats{Listeners: []Listener{{Address: "127.0.0.1:8"}}}, CPU: CPUStats{PerCorePercent: []float64{1, 2}}}
	tr := Trend(s)
	if tr.Processes.All != nil || tr.Host.Hostname != "" || tr.Services.Listeners != nil || tr.CPU.PerCorePercent != nil {
		t.Fatal("history retained heavyweight fields")
	}
}
func TestListenerRedactionDoesNotMutateSnapshot(t *testing.T) {
	s := Snapshot{Services: ServicesStats{Listeners: []Listener{{Address: "127.0.0.1:8000"}}}}
	r := s.Redacted()
	if r.Services.Listeners[0].Address != "redacted" || s.Services.Listeners[0].Address != "127.0.0.1:8000" {
		t.Fatal("redaction mutated source")
	}
}
