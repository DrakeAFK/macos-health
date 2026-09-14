package ui

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/drakeafk/macos-health/internal/stats"
)

func sensor(v stats.SensorValue, unit string) string {
	if !v.Available {
		return "unavailable"
	}
	return fmt.Sprintf("%.1f%s", v.Value, unit)
}
func (r renderer) siliconSummary() string {
	s := r.data.Snapshot.Silicon
	if !s.Enabled {
		return r.metric("GPU", "sensors off")
	}
	return r.metric("GPU", markStale(sensor(s.GPUPercent, "%")+"  "+sensor(s.GPUWatts, "W"), s.Status))
}
func (r renderer) silicon(height int) []string {
	s := r.data.Snapshot.Silicon
	lines := []string{r.section("APPLE SILICON / POWER & THERMALS")}
	if !s.Enabled {
		return take(append(lines, "Sensors disabled. Enable with --sensors.", "Uses read-only IOReport and AppleSMC; no sudo."), height)
	}
	if !s.Status.Available {
		return take(append(lines, r.missing(s.Status), "Sensors vary by chip and macOS version."), height)
	}
	if s.Status.Stale {
		lines = append(lines, r.theme.warning.Render("STALE - displaying last known sensor readings"))
	}
	lines = append(lines, r.metric("CPU", sensor(s.CPUWatts, " W")+"  /  "+sensor(s.CPUTemperature, " C")), r.metric("GPU", sensor(s.GPUPercent, "% active")+"  /  "+sensor(s.GPUWatts, " W")+"  /  "+sensor(s.GPUTemperature, " C")), r.metric("ANE", sensor(s.ANEWatts, " W")))
	for i, rpm := range s.FanRPM {
		lines = append(lines, r.metric(fmt.Sprintf("FAN %d", i+1), fmt.Sprintf("%.0f RPM", rpm)))
	}
	lines = append(lines, "", r.theme.muted.Render("Power is component energy / time, not wall power."), r.theme.muted.Render("Temperatures average recognized sensors; mappings are best effort."))
	lines = append(lines, r.theme.muted.Render(clean(s.Source)))
	if height-len(lines) >= 6 {
		lines = append(lines, r.chart("GPU ACTIVE", "%", func(s stats.Snapshot) (float64, bool) {
			return s.Silicon.GPUPercent.Value, s.Silicon.Status.Available && !s.Silicon.Status.Stale && s.Silicon.GPUPercent.Available
		}, 100, 3)...)
	}
	return take(lines, height)
}

func (r renderer) processGroups(height int, workOnly bool) []string {
	groups := stats.GroupProcesses(r.data.Snapshot.Processes.All)
	if r.data.ProcessSort == SortMemory {
		sort.SliceStable(groups, func(i, j int) bool { return groups[i].RSSBytes > groups[j].RSSBytes })
	}
	filtered := groups[:0]
	for _, g := range groups {
		if workOnly && g.Kind == "" {
			continue
		}
		if r.data.Query != "" && !strings.Contains(strings.ToLower(g.Name+" "+g.Kind), strings.ToLower(r.data.Query)) {
			continue
		}
		filtered = append(filtered, g)
	}
	groups = filtered
	lines := []string{r.section("APPLICATION GROUPS"), r.theme.muted.Render("APP / PROCESSES                          CPU        RSS")}
	if !r.data.Snapshot.Processes.Status.Available {
		return take(append(lines, r.missing(r.data.Snapshot.Processes.Status)), height)
	}
	if len(groups) == 0 {
		return take(append(lines, "No matching groups."), height)
	}
	visible := maxInt(1, height-3)
	cursor := minInt(maxInt(r.data.ProcessOffset, 0), len(groups)-1)
	offset := (cursor / visible) * visible
	for i := offset; i < len(groups) && i < offset+visible; i++ {
		g := groups[i]
		name := fmt.Sprintf("%s (%d) %s", g.Name, g.Count, g.Kind)
		value := fmt.Sprintf("%.0f%%  %s", g.CPUPercent, bytes(g.RSSBytes))
		lines = append(lines, summaryRow("", name, value, r.width, r.glyphs.ellipsis))
	}
	lines = append(lines, r.theme.muted.Render("RSS sum may include shared pages. a: individual processes"))
	return take(lines, height)
}
func (r renderer) processDetail(height int) []string {
	points := r.data.ProcessHistory
	if len(points) == 0 {
		return take([]string{r.section("PROCESS"), "No samples for this process yet. Esc returns."}, height)
	}
	last := points[len(points)-1]
	p := last.Row
	lines := []string{r.section(fmt.Sprintf("%s / PID %d", p.Name, p.PID)), r.metric("APP", clean(p.App)), r.metric("PARENT", fmt.Sprintf("PID %d", p.ParentPID)), r.metric("CPU", fmt.Sprintf("%.1f%% (100%% = one logical core)", p.CPUPercent)), r.metric("RSS", bytes(p.RSSBytes)+"  /  "+fmt.Sprintf("%.1f%% of RAM", p.MemoryPercent)), r.metric("AGE", uptime(uint64(p.AgeSeconds))), r.metric("SAMPLE", last.At.Format("15:04:05"))}
	if p.FootprintAvailable {
		lines = append(lines, r.metric("FOOTPRINT", bytes(p.FootprintBytes)+" charged physical memory"))
	}
	if p.IOAvailable {
		lines = append(lines, r.metric("DISK", rate(p.ReadBytesPerSecond)+" read / "+rate(p.WriteBytesPerSecond)+" write"))
	}
	found := false
	for _, row := range r.data.Snapshot.Processes.All {
		if row.PID == p.PID && math.Abs(row.AgeSeconds-p.AgeSeconds-r.data.Snapshot.Processes.Status.SampledAt.Sub(last.At).Seconds()) < 3 {
			found = true
			break
		}
	}
	if !found {
		lines = append(lines, r.theme.warning.Render("Process exited, became inaccessible, or PID was reused."))
	}
	cpu, mem := []float64{}, []float64{}
	for _, point := range points {
		if point.Row.CPUAvailable {
			cpu = append(cpu, point.Row.CPUPercent)
		}
		mem = append(mem, float64(point.Row.RSSBytes))
	}
	if height >= 10 {
		lines = append(lines, "", r.metric("CPU", r.spark(cpu, minInt(r.width-8, 80), 0)), r.metric("RSS", r.spark(mem, minInt(r.width-8, 80), 0)), r.theme.muted.Render(fmt.Sprintf("Since selection: %s - %s / %d samples", points[0].At.Format("15:04:05"), last.At.Format("15:04:05"), len(points))))
	}
	lines = append(lines, "", r.theme.muted.Render("Esc back. Read-only inspection; no process signals are sent."))
	return take(lines, height)
}

func (r renderer) timeline(height int) []string {
	lines := []string{r.section("HISTORY / " + r.window().String() + " / [ ] change window")}
	charts := []struct {
		name, unit string
		max        float64
		pick       func(stats.Snapshot) (float64, bool)
	}{
		{"CPU", "%", 100, func(s stats.Snapshot) (float64, bool) {
			return s.CPU.UsagePercent, s.CPU.Status.Available && !s.CPU.Status.Stale && !s.CPU.WarmingUp
		}},
		{"MEMORY", "%", 100, func(s stats.Snapshot) (float64, bool) {
			return s.Memory.UsedPercent, s.Memory.Status.Available && !s.Memory.Status.Stale
		}},
		{"GPU", "%", 100, func(s stats.Snapshot) (float64, bool) {
			return s.Silicon.GPUPercent.Value, s.Silicon.Status.Available && !s.Silicon.Status.Stale && s.Silicon.GPUPercent.Available
		}},
		{"NETWORK DOWN", " MiB/s", 0, func(s stats.Snapshot) (float64, bool) {
			return s.Network.DownBytesPerSecond / (1 << 20), s.Network.Status.Available && !s.Network.Status.Stale && s.Network.RatesAvailable
		}},
		{"DISK WRITE", " MiB/s", 0, func(s stats.Snapshot) (float64, bool) {
			return s.Disk.WriteBytesPerSecond / (1 << 20), s.Disk.Status.Available && !s.Disk.Status.Stale && s.Disk.IOAvailable
		}},
	}
	rows := 3
	if height < 25 {
		rows = 2
	}
	for _, c := range charts {
		if height-len(lines) < rows+3 {
			break
		}
		lines = append(lines, r.chart(c.name, c.unit, c.pick, c.max, rows)...)
	}
	if len(lines) == 1 {
		lines = append(lines, "Expand the terminal to see charts.")
	}
	return take(lines, height)
}
func (r renderer) window() time.Duration {
	if r.data.Window <= 0 {
		return 5 * time.Minute
	}
	return r.data.Window
}
func (r renderer) chart(name, unit string, pick func(stats.Snapshot) (float64, bool), maximum float64, rows int) []string {
	width := minInt(maxInt(r.width-2, 1), 100)
	end := r.data.Snapshot.SampledAt
	start := end.Add(-r.window())
	values := make([]float64, width)
	counts := make([]int, width)
	sum, peak := 0., 0.
	n := 0
	for _, s := range r.data.History {
		if s.SampledAt.Before(start) || s.SampledAt.After(end) {
			continue
		}
		v, ok := pick(s)
		if !ok || math.IsNaN(v) || math.IsInf(v, 0) {
			continue
		}
		i := int(float64(s.SampledAt.Sub(start)) / float64(r.window()) * float64(width))
		if i >= width {
			i = width - 1
		}
		values[i] += v
		counts[i]++
		sum += v
		n++
		peak = math.Max(peak, v)
	}
	if n == 0 {
		return []string{r.section(name), r.theme.muted.Render("No fresh samples in this window."), ""}
	}
	for i := range values {
		if counts[i] > 0 {
			values[i] /= float64(counts[i])
		}
	}
	if maximum <= 0 {
		maximum = peak
	}
	if maximum <= 0 {
		maximum = 1
	}
	lines := []string{r.section(name) + r.theme.muted.Render(fmt.Sprintf("  avg %.1f%s / peak %.1f%s", sum/float64(n), unit, peak, unit))}
	for row := rows - 1; row >= 0; row-- {
		var b strings.Builder
		for i, v := range values {
			if counts[i] == 0 {
				b.WriteByte(' ')
				continue
			}
			amount := v/maximum*float64(rows) - float64(row)
			if amount <= 0 {
				b.WriteByte(' ')
			} else {
				idx := minInt(int(math.Ceil(amount*float64(len(r.glyphs.spark)-1))), len(r.glyphs.spark)-1)
				b.WriteString(r.glyphs.spark[idx])
			}
		}
		lines = append(lines, r.theme.spark.Render(b.String()))
	}
	lines = append(lines, joinSides(r.theme.muted.Render(start.Format("15:04:05")), r.theme.muted.Render(end.Format("15:04:05")), width, r.glyphs.ellipsis))
	return lines
}

func (r renderer) insights(height int) []string {
	s := r.data.Snapshot
	lines := []string{r.section("INSIGHTS / EVIDENCE & NEXT STEPS")}
	if len(s.Health.Issues) == 0 {
		lines = append(lines, "No active health warnings in the available telemetry.")
	}
	for _, issue := range s.Health.Issues {
		lines = append(lines, r.issueLine(stats.Issue{Severity: issue.Severity, Title: issue.Title}))
		lines = append(lines, wrap(clean(issue.Detail), r.width)...)
		if strings.HasPrefix(issue.Code, "memory") || issue.Code == "page-outs" {
			if len(s.Processes.TopMemory) > 0 {
				p := s.Processes.TopMemory[0]
				lines = append(lines, fmt.Sprintf("Largest RSS: %s / %s. 2: inspect processes.", clean(p.Name), bytes(p.RSSBytes)))
			}
		}
		if issue.Code == "cpu-sustained" && len(s.Processes.TopCPU) > 0 {
			p := s.Processes.TopCPU[0]
			lines = append(lines, fmt.Sprintf("Highest CPU: %s / %.0f%%. High usage can be intentional.", clean(p.Name), p.CPUPercent))
		}
		lines = append(lines, "")
	}
	if height-len(lines) >= 4 {
		lines = append(lines, r.section("RECENT EVENTS"))
		for i := len(r.data.Events) - 1; i >= 0 && len(lines) < height-2; i-- {
			event := r.data.Events[i]
			state := strings.ToUpper(string(event.Severity))
			if event.Resolved {
				state = "CLEARED"
			}
			lines = append(lines, event.At.Format("15:04:05")+"  "+state+"  "+clean(event.Title))
		}
		if len(r.data.Events) == 0 {
			lines = append(lines, "Events appear when a health condition changes.")
		}
	}
	return take(lines, height)
}
func wrap(s string, width int) []string {
	if width < 1 {
		return nil
	}
	words := strings.Fields(s)
	lines := []string{}
	line := ""
	for _, w := range words {
		if cellWidth(line)+cellWidth(w)+1 > width && line != "" {
			lines = append(lines, line)
			line = ""
		}
		if line != "" {
			line += " "
		}
		line += w
	}
	if line != "" {
		lines = append(lines, line)
	}
	return lines
}

func (r renderer) workloads(height int) []string {
	s := r.data.Snapshot
	lines := []string{r.section("DEVELOPMENT & LOCAL AI"), r.memoryLine(), r.memoryDetailLine(), r.siliconSummary(), "", r.section("MODEL WEIGHT ESTIMATES / 4-BIT + 20% OVERHEAD")}
	if s.Services.Enabled {
		lines = lines[:len(lines)-1]
		lines = append(lines, r.section("LOCAL TCP LISTENERS"))
		if !s.Services.Status.Available {
			lines = append(lines, r.missing(s.Services.Status))
		} else if len(s.Services.Listeners) == 0 {
			lines = append(lines, "No visible TCP listeners.")
		} else {
			for i, l := range s.Services.Listeners {
				if i >= 5 {
					lines = append(lines, fmt.Sprintf("+%d more in JSON export", len(s.Services.Listeners)-i))
					break
				}
				lines = append(lines, summaryRow(fmt.Sprint(l.PID), clean(l.Process), clean(l.Address), r.width, r.glyphs.ellipsis))
			}
		}
		lines = append(lines, r.section("MODEL WEIGHT ESTIMATES / 4-BIT + 20% OVERHEAD"))
	}
	for _, b := range []float64{3, 7, 14, 32, 70} {
		size := b * 1e9 * .5 * 1.2
		value := bytes(uint64(size))
		if s.Memory.TotalBytes > 0 {
			value += fmt.Sprintf(" / %.0f%% of installed RAM", 100*size/float64(s.Memory.TotalBytes))
		}
		lines = append(lines, r.metric(fmt.Sprintf("%.0fB", b), value))
	}
	lines = append(lines, r.theme.muted.Render("Estimates exclude KV cache, activations and other apps."), r.theme.muted.Render("These are weight budgets, not model-fit or speed guarantees."))
	if height-len(lines) >= 5 {
		lines = append(lines, "")
		groups := r.processGroups(height-len(lines), true)
		lines = append(lines, groups...)
	}
	return take(lines, height)
}
