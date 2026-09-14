package app

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/drakeafk/macos-health/internal/stats"
	"github.com/drakeafk/macos-health/internal/ui"
)

type fakeCollector struct{}

func (fakeCollector) Collect(context.Context) (stats.Snapshot, error) {
	return stats.Snapshot{}, nil
}

func TestModelSingleFlightAndStaleResponse(t *testing.T) {
	m := NewModel(Config{Collector: fakeCollector{}, Interval: time.Second})
	if !m.inFlight || m.requestID != 1 {
		t.Fatalf("initial request state = inFlight %v id %d", m.inFlight, m.requestID)
	}

	updated, _ := m.Update(tickMsg(time.Now()))
	m = updated.(Model)
	if m.requestID != 1 {
		t.Fatalf("tick started overlapping request %d", m.requestID)
	}

	now := time.Now()
	updated, _ = m.Update(snapshotMsg{id: 1, snapshot: healthySnapshot(now)})
	m = updated.(Model)
	if m.inFlight {
		t.Fatal("completed request still marked in flight")
	}

	updated, _ = m.Update(tickMsg(now))
	m = updated.(Model)
	if !m.inFlight || m.requestID != 2 {
		t.Fatalf("next request state = inFlight %v id %d", m.inFlight, m.requestID)
	}

	older := healthySnapshot(now.Add(-time.Hour))
	updated, _ = m.Update(snapshotMsg{id: 1, snapshot: older})
	m = updated.(Model)
	if !m.snapshot.SampledAt.Equal(now) {
		t.Fatal("stale response replaced the current snapshot")
	}
}

func TestModelRetainsLastGoodMetric(t *testing.T) {
	m := NewModel(Config{Collector: fakeCollector{}})
	now := time.Now()
	updated, _ := m.Update(snapshotMsg{id: 1, snapshot: healthySnapshot(now)})
	m = updated.(Model)

	m.requestID = 2
	m.inFlight = true
	failed := healthySnapshot(now.Add(time.Second))
	failed.CPU = stats.CPUStats{Status: stats.MetricStatus{Error: "counter failed"}}
	updated, _ = m.Update(snapshotMsg{id: 2, snapshot: failed, err: errors.New("partial")})
	m = updated.(Model)
	if !m.snapshot.CPU.Status.Available || !m.snapshot.CPU.Status.Stale {
		t.Fatalf("CPU status = %+v, want retained stale value", m.snapshot.CPU.Status)
	}
	if m.snapshot.CPU.UsagePercent != 25 {
		t.Fatalf("CPU = %.1f, want last good 25", m.snapshot.CPU.UsagePercent)
	}
}

func TestModelKeysAndResize(t *testing.T) {
	m := NewModel(Config{Collector: fakeCollector{}})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 35})
	m = updated.(Model)
	if m.width != 120 || m.height != 35 {
		t.Fatalf("size = %dx%d", m.width, m.height)
	}

	for key, assertion := range map[string]func(Model) bool{
		"2": func(m Model) bool { return m.page == ui.PageProcesses },
		"3": func(m Model) bool { return m.page == ui.PageBattery },
		"4": func(m Model) bool { return m.page == ui.PageSystem },
	} {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
		m = updated.(Model)
		if !assertion(m) {
			t.Fatalf("key %q did not select expected page", key)
		}
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = updated.(Model)
	if !m.paused {
		t.Fatal("space did not pause")
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	m = updated.(Model)
	if m.processSort != ui.SortMemory {
		t.Fatalf("sort = %q", m.processSort)
	}

	// Test j/k process scrolling and h/l tab switching
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")})
	m = updated.(Model)
	if m.page != ui.PageProcesses {
		t.Fatalf("page = %v, want PageProcesses", m.page)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m = updated.(Model)
	if m.processOffset != 1 {
		t.Fatalf("processOffset after 'j' = %d, want 1", m.processOffset)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	m = updated.(Model)
	if m.processOffset != 0 {
		t.Fatalf("processOffset after 'k' = %d, want 0", m.processOffset)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	m = updated.(Model)
	if m.page != ui.PageBattery || m.processOffset != 0 {
		t.Fatalf("after 'l': page = %v, offset = %d", m.page, m.processOffset)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})
	m = updated.(Model)
	if m.page != ui.PageProcesses {
		t.Fatalf("after 'h': page = %v, want PageProcesses", m.page)
	}
}

func healthySnapshot(at time.Time) stats.Snapshot {
	available := stats.MetricStatus{Available: true, SampledAt: at}
	snapshot := stats.Snapshot{
		SampledAt: at,
		CPU: stats.CPUStats{
			Status:       available,
			UsagePercent: 25,
		},
		Memory: stats.MemoryStats{
			Status:            available,
			Pressure:          "Normal",
			PressureAvailable: true,
		},
		Disk: stats.DiskStats{
			Status:      available,
			UsedPercent: 50,
		},
	}
	snapshot.Health = stats.Assess(snapshot, nil)
	return snapshot
}

func TestSearchDoesNotTriggerCommandsAndReplayStops(t *testing.T) {
	m := NewModel(Config{Collector: fakeCollector{}})
	press := func(key string) {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
		m = updated.(Model)
	}
	press("/")
	press("q")
	press("s")
	if m.query != "qs" || m.processSort != ui.SortCPU {
		t.Fatal("search dispatched a shortcut")
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.searching || m.query != "" {
		t.Fatal("escape did not clear search")
	}
	for key, page := range map[string]ui.Page{"5": ui.PageSilicon, "6": ui.PageHistory, "7": ui.PageInsights, "8": ui.PageWorkloads} {
		press(key)
		if m.page != page {
			t.Fatalf("page %s", key)
		}
	}
	updated, _ = m.Update(snapshotMsg{id: m.requestID, err: io.EOF})
	m = updated.(Model)
	if !m.ended || !m.paused {
		t.Fatal("replay did not stop")
	}
	id := m.requestID
	press("r")
	if m.requestID != id {
		t.Fatal("EOF replay restarted")
	}
}
