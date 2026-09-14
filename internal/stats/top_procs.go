//go:build darwin

package stats

import (
	"context"
	"fmt"
	"math"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

type processSample struct {
	cpuAvailable          bool
	name, app             string
	readBytes, writeBytes uint64
	ioAvailable           bool
	cpuSeconds            float64
	startedAt             time.Time
	at                    time.Time
	smoothed              float64
}

type parsedProcess struct {
	cpuAvailable          bool
	startedAt             time.Time
	readBytes, writeBytes uint64
	ioAvailable           bool
	row                   ProcessRow
	cpuSeconds            float64
	elapsed               float64
}

func (c *Collector) collectProcesses(ctx context.Context, now time.Time, limit int) (ProcessStats, error) {
	var parsed []parsedProcess
	var err error
	source := "libproc (accessible processes)"
	if c.systemProcesses {
		source = "ps (extended system visibility)"
		var out []byte
		out, err = commandOutput(ctx, "/bin/ps", "-axo", "pid=,ppid=,etime=,time=,pmem=,rss=,comm=")
		if err == nil {
			parsed, err = parseProcesses(string(out))
		}
	} else {
		parsed, err = c.readProcesses(ctx, now)
	}
	if err != nil {
		return ProcessStats{}, err
	}

	rows := make([]ProcessRow, 0, len(parsed))
	validCPU := 0
	next := make(map[int32]processSample, len(parsed))

	c.processMu.Lock()
	for _, process := range parsed {
		started := process.startedAt
		if started.IsZero() {
			started = now.Add(-time.Duration(process.elapsed * float64(time.Second)))
		}
		sample := processSample{cpuAvailable: process.cpuAvailable, cpuSeconds: process.cpuSeconds, startedAt: started, at: now, name: process.row.Name, app: process.row.App, readBytes: process.readBytes, writeBytes: process.writeBytes, ioAvailable: process.ioAvailable}
		if previous, ok := c.lastProcesses[process.row.PID]; ok && (started.Equal(previous.startedAt) || (process.startedAt.IsZero() && math.Abs(started.Sub(previous.startedAt).Seconds()) <= 3)) {
			dt := now.Sub(previous.at).Seconds()
			delta := process.cpuSeconds - previous.cpuSeconds
			if dt > 0 && dt < 30 && delta >= 0 && process.cpuAvailable && previous.cpuAvailable {
				raw := delta / dt * 100
				maximum := float64(runtime.NumCPU()) * 100
				if raw > maximum {
					raw = maximum
				}
				if previous.smoothed > 0 {
					sample.smoothed = raw*0.7 + previous.smoothed*0.3
				} else {
					sample.smoothed = raw
				}
				process.row.CPUPercent = sample.smoothed
				process.row.CPUAvailable = true
				validCPU++
				if previous.ioAvailable && process.ioAvailable && process.readBytes >= previous.readBytes && process.writeBytes >= previous.writeBytes {
					process.row.ReadBytesPerSecond = float64(process.readBytes-previous.readBytes) / dt
					process.row.WriteBytesPerSecond = float64(process.writeBytes-previous.writeBytes) / dt
					process.row.IOAvailable = true
				}
			}
		}
		next[process.row.PID] = sample
		rows = append(rows, process.row)
	}
	c.lastProcesses = next
	c.processMu.Unlock()

	topCPU := append([]ProcessRow(nil), rows...)
	sort.SliceStable(topCPU, func(i, j int) bool {
		if topCPU[i].CPUPercent == topCPU[j].CPUPercent {
			return topCPU[i].RSSBytes > topCPU[j].RSSBytes
		}
		return topCPU[i].CPUPercent > topCPU[j].CPUPercent
	})
	topMemory := append([]ProcessRow(nil), rows...)
	sort.SliceStable(topMemory, func(i, j int) bool {
		if topMemory[i].RSSBytes == topMemory[j].RSSBytes {
			return topMemory[i].CPUPercent > topMemory[j].CPUPercent
		}
		return topMemory[i].RSSBytes > topMemory[j].RSSBytes
	})
	if limit > 0 && len(topCPU) > limit {
		topCPU = topCPU[:limit]
	}
	if limit > 0 && len(topMemory) > limit {
		topMemory = topMemory[:limit]
	}
	if validCPU == 0 {
		topCPU = nil
	}
	return ProcessStats{Source: source, All: rows, Total: len(rows), WarmingUp: validCPU == 0, TopCPU: topCPU, TopMemory: topMemory}, nil
}

func parseProcesses(output string) ([]parsedProcess, error) {
	rows := make([]parsedProcess, 0, 128)
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}
		var parent int64
		if len(fields) >= 7 {
			if n, e := strconv.ParseInt(fields[1], 10, 32); e == nil && n >= 0 {
				parent = n
				fields = append([]string{fields[0]}, fields[2:]...)
			}
		}
		pid, pidErr := strconv.ParseInt(fields[0], 10, 32)
		elapsed, elapsedErr := parseClock(fields[1])
		cpuSeconds, cpuErr := parseClock(fields[2])
		memoryPercent, memoryErr := strconv.ParseFloat(fields[3], 64)
		rssKB, rssErr := strconv.ParseUint(fields[4], 10, 64)
		if pidErr != nil || elapsedErr != nil || cpuErr != nil || memoryErr != nil || rssErr != nil {
			continue
		}
		if pid <= 0 || memoryPercent < 0 || memoryPercent > 100 || math.IsNaN(memoryPercent) || math.IsInf(memoryPercent, 0) {
			continue
		}
		command := cleanText(strings.Join(fields[5:], " "))
		name := cleanText(filepath.Base(command))
		if name == "" || name == "." || name == string(filepath.Separator) {
			name = command
		}
		rows = append(rows, parsedProcess{
			row: ProcessRow{
				PID:             int32(pid),
				ParentPID:       int32(parent),
				Name:            name,
				App:             processApp(command),
				Kind:            processKind(name),
				AgeSeconds:      elapsed,
				MemoryPercent:   memoryPercent,
				MemoryAvailable: true,
				RSSBytes:        rssKB * 1024,
			},
			cpuSeconds:   cpuSeconds,
			cpuAvailable: true,
			elapsed:      elapsed,
		})
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("ps: no process rows parsed")
	}
	return rows, nil
}

// parseClock accepts ps time forms including MM:SS.cc, HH:MM:SS, and
// DD-HH:MM:SS. It returns seconds.
func parseClock(value string) (float64, error) {
	var days float64
	if before, after, ok := strings.Cut(value, "-"); ok {
		parsed, err := strconv.ParseFloat(before, 64)
		if err != nil {
			return 0, err
		}
		if parsed < 0 || parsed != math.Trunc(parsed) || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
			return 0, fmt.Errorf("invalid day value %q", before)
		}
		days = parsed
		value = after
	}
	parts := strings.Split(value, ":")
	if len(parts) < 1 || len(parts) > 3 {
		return 0, fmt.Errorf("invalid clock %q", value)
	}
	parsed := make([]float64, len(parts))
	for i, part := range parts {
		field, err := strconv.ParseFloat(part, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid clock %q: %w", value, err)
		}
		if field < 0 || math.IsNaN(field) || math.IsInf(field, 0) {
			return 0, fmt.Errorf("invalid clock %q", value)
		}
		if i < len(parts)-1 && field != math.Trunc(field) {
			return 0, fmt.Errorf("invalid clock %q", value)
		}
		parsed[i] = field
	}
	if parsed[len(parsed)-1] >= 60 {
		return 0, fmt.Errorf("invalid clock %q", value)
	}
	if len(parsed) == 3 && parsed[1] >= 60 {
		return 0, fmt.Errorf("invalid clock %q", value)
	}
	seconds := days * 86400
	switch len(parsed) {
	case 1:
		seconds += parsed[0]
	case 2:
		seconds += parsed[0]*60 + parsed[1]
	case 3:
		seconds += parsed[0]*3600 + parsed[1]*60 + parsed[2]
	}
	return seconds, nil
}
