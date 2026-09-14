package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/drakeafk/macos-health/internal/app"
	"github.com/drakeafk/macos-health/internal/session"
	"github.com/drakeafk/macos-health/internal/stats"
)

// One worker owns collection, assessment, history and recording in all modes.
// Only compact trends are retained; snapshots handed to callers are immutable.
type pipeline struct {
	source         app.SnapshotCollector
	record         *session.Recorder
	redact, replay bool
	previous       stats.Snapshot
	history        []stats.Snapshot
}

func (p *pipeline) Collect(ctx context.Context) (stats.Snapshot, error) {
	s, err := p.source.Collect(ctx)
	if s.SampledAt.IsZero() {
		return s, err
	}
	if !p.replay {
		if !p.previous.SampledAt.IsZero() {
			s = stats.MergeLastGood(s, p.previous)
		}
		s.Health = stats.Assess(s, p.history)
		{
			p.history = append(p.history, stats.Trend(s))
			if len(p.history) > 64 {
				copy(p.history, p.history[len(p.history)-64:])
				p.history = p.history[:64]
			}
		}
	}
	p.previous = s
	if p.redact {
		s = s.Redacted()
	}
	if p.record != nil {
		if e := p.record.Write(s); e != nil {
			return s, fmt.Errorf("record: %w", e)
		}
	}
	return s, err
}
func stream(ctx context.Context, c app.SnapshotCollector, o options, w, errors io.Writer) int {
	timer := time.NewTimer(0)
	defer timer.Stop()
	for i := 0; o.samples == 0 || i < o.samples; i++ {
		select {
		case <-ctx.Done():
			return 0
		case <-timer.C:
		}
		s, err := collectWithTimeout(ctx, c, 3*time.Second)
		if err == io.EOF {
			return 0
		}
		if s.SampledAt.IsZero() {
			fmt.Fprintln(errors, err)
			return 1
		}
		if e := writeSnapshot(w, s, o); e != nil {
			fmt.Fprintln(errors, e)
			return 1
		}
		if err != nil {
			fmt.Fprintln(errors, err)
			return 1
		}
		if o.check && healthExit(s.Health) != 0 && s.Health.Status != "sampling" {
			return healthExit(s.Health)
		}
		timer.Reset(o.interval)
	}
	return 0
}

func serve(ctx context.Context, c app.SnapshotCollector, o options, log io.Writer) error {
	host, _, err := net.SplitHostPort(o.serve)
	if err != nil {
		return fmt.Errorf("serve: use a loopback IP and port, e.g. 127.0.0.1:9797")
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("serve binds only to a literal loopback IP")
	}
	listener, err := net.Listen("tcp", o.serve)
	if err != nil {
		return err
	}
	var mu sync.RWMutex
	var current stats.Snapshot
	mux := http.NewServeMux()
	mux.HandleFunc("/snapshot", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			w.WriteHeader(405)
			return
		}
		mu.RLock()
		s := current
		mu.RUnlock()
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		if s.SampledAt.IsZero() {
			w.WriteHeader(503)
		}
		_ = json.NewEncoder(w).Encode(s)
	})
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			w.WriteHeader(405)
			return
		}
		mu.RLock()
		s := current
		mu.RUnlock()
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		if s.SampledAt.IsZero() {
			w.WriteHeader(503)
			return
		}
		_, _ = io.WriteString(w, stats.Prometheus(s))
	})
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		mu.RLock()
		s := current
		mu.RUnlock()
		w.Header().Set("Content-Type", "application/json")
		if s.SampledAt.IsZero() || time.Since(s.SampledAt) > o.interval+5*time.Second {
			w.WriteHeader(503)
		}
		_ = json.NewEncoder(w).Encode(s.Health)
	})
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 8192}
	failures := make(chan error, 1)
	go func() {
		e := server.Serve(listener)
		if e != http.ErrServerClosed {
			failures <- e
		}
	}()
	defer server.Close()
	fmt.Fprintln(log, "Serving local snapshots at http://"+listener.Addr().String())
	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case e := <-failures:
			return e
		case <-timer.C:
			s, e := collectWithTimeout(ctx, c, 3*time.Second)
			if e != nil {
				fmt.Fprintln(log, e)
			}
			if !s.SampledAt.IsZero() {
				mu.Lock()
				current = s
				mu.Unlock()
			}
			timer.Reset(o.interval)
		}
	}
}

type demoCollector struct{ index int }

func (d *demoCollector) Collect(ctx context.Context) (stats.Snapshot, error) {
	if err := ctx.Err(); err != nil {
		return stats.Snapshot{}, err
	}
	d.index++
	now := time.Now()
	ok := stats.MetricStatus{Available: true, SampledAt: now}
	value := func(v float64) stats.SensorValue { return stats.SensorValue{Available: true, Value: v} }
	s := stats.Snapshot{SchemaVersion: stats.SnapshotSchemaVersion, SampledAt: now, CollectionDurationMS: 3,
		CPU:     stats.CPUStats{Status: ok, UsagePercent: float64(22 + d.index%30), PerCorePercent: []float64{12, 18, 25, 14, 70, 81, 42, 65}, LoadAvailable: true, Load1: 3.2, Load5: 2.8, Load15: 2.1},
		Memory:  stats.MemoryStats{Status: ok, TotalBytes: 36 << 30, UsedBytes: 23 << 30, UsedPercent: 64, AvailablePercent: 36, Pressure: "Normal", PressureAvailable: true, CompressionAvailable: true, CompressedBytes: 2 << 30, SwapUsedBytes: 512 << 20},
		Disk:    stats.DiskStats{Status: ok, Mount: "/System/Volumes/Data", Device: "disk0", TotalBytes: 1 << 40, FreeBytes: 340 << 30, UsedPercent: 67, IOAvailable: true, ReadBytesPerSecond: 18 << 20, WriteBytesPerSecond: 3 << 20},
		Network: stats.NetworkStats{Status: ok, Interface: "en0", RatesAvailable: true, DownBytesPerSecond: 2 << 20, UpBytesPerSecond: 320 << 10},
		Battery: stats.BatteryStats{Status: ok, Present: true, Percent: 82, State: "Discharging", HealthPercent: 94, DetailsAvailable: true, Condition: "Normal", CycleCount: 124, TimeRemainingAvailable: true, TimeRemainingMinutes: 243, TemperatureAvailable: true, TemperatureC: 31},
		Thermal: stats.ThermalStats{Status: ok, State: "Nominal"},
		Host:    stats.HostInfo{Status: ok, ComputerName: "Demo Mac", Hostname: "demo.local", Chip: "Apple M3 Pro (demo)", LogicalCores: 12, PerformanceCores: 6, EfficiencyCores: 6, GPU: "Apple M3 Pro", GPUCores: 18, OSVersion: "26.0", Architecture: "arm64"},
		Silicon: stats.SiliconStats{Status: ok, Enabled: true, Source: "Synthetic demo", CPUWatts: value(8.4), GPUWatts: value(12.1), ANEWatts: value(.2), GPUPercent: value(float64(35 + d.index%35)), CPUTemperature: value(57), GPUTemperature: value(61), FanRPM: []float64{1800, 1800}},
		System:  stats.SystemStats{Status: ok, UptimeSeconds: 283400}}
	rows := []stats.ProcessRow{{PID: 101, Name: "ollama", App: "ollama", Kind: "AI", CPUAvailable: true, CPUPercent: 142, RSSBytes: 7 << 30, MemoryPercent: 19.4, AgeSeconds: float64(300 + d.index)}, {PID: 202, Name: "Code Helper", App: "Visual Studio Code", Kind: "Dev", CPUAvailable: true, CPUPercent: 32, RSSBytes: 512 << 20, MemoryPercent: 1.4, AgeSeconds: float64(800 + d.index)}, {PID: 203, Name: "Code", App: "Visual Studio Code", Kind: "Dev", CPUAvailable: true, CPUPercent: 12, RSSBytes: 256 << 20, AgeSeconds: float64(800 + d.index)}, {PID: 303, Name: "node", App: "node", Kind: "Dev", CPUAvailable: true, CPUPercent: 18, RSSBytes: 320 << 20, AgeSeconds: float64(100 + d.index)}}
	for i := range rows {
		rows[i].MemoryAvailable = true
		rows[i].FootprintAvailable = true
		rows[i].FootprintBytes = rows[i].RSSBytes
		rows[i].IOAvailable = true
	}
	s.Processes = stats.ProcessStats{Status: ok, All: rows, Total: len(rows), Source: "synthetic demo"}
	s.Processes.TopCPU = stats.ProcessRows(s.Processes, "", "cpu")
	s.Processes.TopMemory = stats.ProcessRows(s.Processes, "", "memory")
	s.Health = stats.Assess(s, nil)
	return s, nil
}
