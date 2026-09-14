package ui

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/drakeafk/macos-health/internal/stats"
)

// Page identifies one of the four dashboard views.
type Page uint8

const (
	PageOverview Page = iota
	PageProcesses
	PageBattery
	PageSystem
	PageSilicon
	PageHistory
	PageInsights
	PageWorkloads
	PageCount
)

// ProcessSort controls the process page's primary ordering and compact value.
type ProcessSort string

const (
	SortCPU    ProcessSort = "cpu"
	SortMemory ProcessSort = "memory"
)

// Data is the complete, immutable input to Render. Now is optional and mainly
// exists to make freshness output deterministic in tests. Plain implies both
// ASCII glyphs and no terminal styling.
type ProcessPoint struct {
	At  time.Time
	Row stats.ProcessRow
}

type Data struct {
	Theme          string
	Group          bool
	Query          string
	Searching      bool
	Detail         bool
	SelectedPID    int32
	ProcessHistory []ProcessPoint
	Events         []stats.Event
	Window         time.Duration
	Notice         string
	Replay         bool
	Snapshot       stats.Snapshot
	History        []stats.Snapshot
	Width          int
	Height         int
	Page           Page
	ProcessSort    ProcessSort
	ProcessOffset  int
	Now            time.Time
	Loading        bool
	Paused         bool
	ShowHelp       bool
	Plain          bool
	NoColor        bool
	ASCII          bool
	Error          error
}

type renderer struct {
	data   Data
	theme  theme
	glyphs glyphSet
	width  int
	height int
}

// Render returns a complete dashboard frame. Every output line is constrained
// by terminal cell width and every frame by terminal height.
func Render(data Data) string {
	if data.Width <= 0 || data.Height <= 0 {
		return ""
	}
	if data.Page >= PageCount {
		data.Page = PageOverview
	}
	if data.ProcessSort != SortMemory {
		data.ProcessSort = SortCPU
	}
	if data.Now.IsZero() {
		data.Now = time.Now()
	}
	if data.Plain {
		data.NoColor = true
		data.ASCII = true
	}

	r := renderer{
		data:   data,
		theme:  namedTheme(data.NoColor, data.Plain, data.Theme),
		glyphs: newGlyphs(data.ASCII),
		width:  data.Width,
		height: data.Height,
	}
	return r.render()
}

func (r renderer) render() string {
	// Even tiny terminals retain a useful health state rather than a fragment
	// of a larger layout.
	if r.height == 1 {
		return constrain([]string{r.statusLine()}, r.width, r.height, r.glyphs.ellipsis)
	}

	header := r.header()
	footerHeight := 0
	if r.height >= 6 {
		footerHeight = 1
	}
	bodyHeight := r.height - len(header) - footerHeight
	if bodyHeight < 0 {
		bodyHeight = 0
	}

	var body []string
	if r.data.ShowHelp {
		body = r.help(bodyHeight)
	} else {
		switch r.data.Page {
		case PageProcesses:
			body = r.processes(bodyHeight)
		case PageBattery:
			body = r.battery(bodyHeight)
		case PageSystem:
			body = r.system(bodyHeight)
		case PageSilicon:
			body = r.silicon(bodyHeight)
		case PageHistory:
			body = r.timeline(bodyHeight)
		case PageInsights:
			body = r.insights(bodyHeight)
		case PageWorkloads:
			body = r.workloads(bodyHeight)
		default:
			body = r.overview(bodyHeight)
		}
	}

	lines := append(header, body...)
	if footerHeight == 1 {
		for len(lines) < r.height-1 {
			lines = append(lines, "")
		}
		lines = append(lines, r.footer())
	}
	return constrain(lines, r.width, r.height, r.glyphs.ellipsis)
}

func (r renderer) header() []string {
	status, summary := r.health()
	if r.data.Error != nil {
		if status == "healthy" {
			status = "partial"
		}
		if status != "critical" && status != "warning" {
			summary = "Telemetry error: " + clean(r.data.Error.Error())
		}
	}
	left := r.theme.title.Render("macos-health") + " " + r.statusBadge(status)
	if r.data.Replay {
		left += " [REPLAY]"
	}
	if r.data.Snapshot.Health.Confidence == "partial" && (status == "critical" || status == "warning") {
		left += " (partial telemetry)"
	}
	if r.data.Paused {
		left += " " + r.theme.warning.Render("[PAUSED]")
	}
	right := r.freshness()
	if r.width < 24 {
		left = r.statusBadge(status)
	}

	lines := []string{joinSides(left, r.theme.muted.Render(right), r.width, r.glyphs.ellipsis)}
	if r.height >= 3 {
		lines = append(lines, r.themeForHealth(status).Render(summary))
	}
	if r.height >= 5 {
		lines = append(lines, r.tabs())
	}
	return lines
}

func (r renderer) health() (string, string) {
	if r.data.Loading || r.data.Snapshot.SampledAt.IsZero() {
		return "sampling", "Sampling system activity"
	}
	status := strings.ToLower(clean(r.data.Snapshot.Health.Status))
	if status == "" {
		status = "partial"
	}
	summary := clean(r.data.Snapshot.Health.Summary)
	if summary == "" {
		switch status {
		case "healthy":
			summary = "Your Mac looks healthy"
		case "critical":
			summary = "Critical system pressure detected"
		case "warning":
			summary = "One or more readings need attention"
		default:
			summary = "Some telemetry is unavailable"
		}
	}
	return status, summary
}

func (r renderer) statusLine() string {
	status, summary := r.health()
	return r.statusBadge(status) + " " + r.themeForHealth(status).Render(summary)
}

func (r renderer) statusBadge(status string) string {
	var label string
	switch status {
	case "healthy":
		label = "[OK]"
	case "warning":
		label = "[WARN]"
	case "critical":
		label = "[CRIT]"
	case "sampling":
		label = "[....]"
	default:
		label = "[PART]"
	}
	return r.themeForHealth(status).Render(label)
}

func (r renderer) tabs() string {
	labels := []string{"1 Overview", "2 Processes", "3 Battery", "4 System", "5 Silicon", "6 History", "7 Insights", "8 Workloads"}
	if r.width < 116 {
		labels = []string{"1 Home", "2 Proc", "3 Batt", "4 Sys", "5 Chip", "6 Time", "7 Why", "8 Work"}
	}
	if r.width < 68 {
		return r.theme.tabActive.Render("["+labels[r.data.Page]+"]") + r.theme.muted.Render("  Tab: next / 1-8: jump")
	}
	parts := make([]string, len(labels))
	for i, label := range labels {
		if Page(i) == r.data.Page && !r.data.ShowHelp {
			parts[i] = r.theme.tabActive.Render("[" + label + "]")
		} else {
			parts[i] = r.theme.tab.Render(label)
		}
	}
	return strings.Join(parts, " ")
}

func (r renderer) freshness() string {
	if r.data.Replay {
		return r.data.Snapshot.SampledAt.Format("15:04:05")
	}
	if r.data.Paused {
		return "paused"
	}
	at := r.data.Snapshot.SampledAt
	if at.IsZero() {
		return "sampling"
	}
	age := r.data.Now.Sub(at)
	if age < 0 {
		age = 0
	}
	switch {
	case age < time.Second:
		return "now"
	case age < time.Minute:
		return fmt.Sprintf("%ds ago", int(age.Seconds()))
	case age < time.Hour:
		return fmt.Sprintf("%dm ago", int(age.Minutes()))
	default:
		return at.Format("15:04")
	}
}

func (r renderer) footer() string {
	if r.data.Searching {
		return r.theme.info.Render("/ " + clean(r.data.Query) + "_  Enter apply / Esc clear")
	}
	if r.data.ShowHelp {
		return r.theme.muted.Render("Esc/? close help   q quit")
	}
	if r.data.Notice != "" {
		return r.theme.info.Render(clean(r.data.Notice)) + r.theme.muted.Render("   ? help  q quit")
	}
	switch r.data.Page {
	case PageProcesses:
		return r.theme.muted.Render("/ search  j/k select  Enter inspect  a group  s sort  Esc back  ? help")
	case PageHistory:
		return r.theme.muted.Render("[ ] time window  space pause  7 events  ? help  q quit")
	default:
		if r.width >= 76 {
			return r.theme.muted.Render("1-8/Tab pages  space pause  t theme  w save  x privacy  ? help  q quit")
		}
	}
	return r.theme.muted.Render("Tab pages  space pause  ? help  q quit")
}
func (r renderer) help(height int) []string {
	return take([]string{r.section("KEYBOARD"), "1-8 / Tab / h/l      change page", "j/k / arrows        select process; PgUp/PgDn jump", "/                   search name, app, PID, AI or Dev", "Enter / Esc         inspect process / return", "a / s               group apps / sort CPU or memory", "[ / ]               history window (1m through 1h)", "Space / r           pause / refresh", "t / w               theme / save preferences", "x                   redact host identity and addresses", "? / Esc             help / close", "q / Ctrl+C          quit", "", r.theme.muted.Render("Process CPU: 100% is one core. App RSS may double-count shared pages."), r.theme.muted.Render("Silicon sensors are best effort. Missing data never implies zero.")}, height)
}

func (r renderer) overview(height int) []string {
	if height <= 0 {
		return nil
	}
	if r.width >= 110 && r.height >= 20 {
		return r.overviewWide(height)
	}

	s := r.data.Snapshot
	lines := make([]string, 0, height)
	if issue := topIssue(s.Health.Issues); issue != nil && height >= 8 {
		lines = append(lines, r.issueLine(*issue))
	}
	lines = append(lines,
		r.cpuLine(),
		r.memoryLine(),
		r.batteryLine(),
		r.diskLine(),
		r.networkLine(),
	)

	remaining := height - len(lines)
	if remaining > 0 {
		rows := r.culpritLines(remaining)
		lines = append(lines, rows...)
	}
	return take(lines, height)
}

func (r renderer) overviewWide(height int) []string {
	left := []string{
		r.section("PERFORMANCE"),
		r.cpuLine(),
		r.loadLine(),
		r.coreLine(),
		r.thermalLine(),
		r.siliconSummary(),
	}
	right := []string{
		r.section("MEMORY & POWER"),
		r.memoryLine(),
		r.memoryDetailLine(),
		r.batteryLine(),
		r.diskLine(),
		r.networkLine(),
	}

	if issue := topIssue(r.data.Snapshot.Health.Issues); issue != nil {
		left = append(left, "", r.section("WHY"), r.issueLine(*issue))
	}

	baseHeight := maxInt(len(left), len(right))
	for len(left) < baseHeight {
		left = append(left, "")
	}
	for len(right) < baseHeight {
		right = append(right, "")
	}
	room := height - baseHeight
	if room >= 3 {
		left = append(left, "", r.section("TOP CPU"))
		right = append(right, "", r.section("TOP MEMORY"))
		count := room - 2
		left = append(left, r.compactProcessRows(r.data.Snapshot.Processes.TopCPU, count, SortCPU)...)
		right = append(right, r.compactProcessRows(r.data.Snapshot.Processes.TopMemory, count, SortMemory)...)
	}
	return joinColumns(left, right, r.width, height, r.glyphs.vertical, r.glyphs.ellipsis)
}

func (r renderer) cpuLine() string {
	c := r.data.Snapshot.CPU
	if !c.Status.Available {
		if c.WarmingUp || r.data.Loading {
			return r.metric("CPU", "sampling"+r.glyphs.ellipsis)
		}
		return r.metric("CPU", r.missing(c.Status))
	}
	value := fmt.Sprintf("%.0f%%", c.UsagePercent)
	history := historyCPU(r.data.History)
	if len(history) >= 2 && r.width >= 52 {
		value += " " + r.spark(history, minInt(12, r.width/7), 100)
	}
	if c.LoadAvailable && r.width >= 52 {
		value += fmt.Sprintf("  load %.1f", c.Load1)
		if cores := r.data.Snapshot.Host.LogicalCores; cores > 0 {
			value += fmt.Sprintf("/%d", cores)
		}
	}
	if r.data.Snapshot.Thermal.Status.Available && r.width >= 72 {
		value += "  thermal " + clean(r.data.Snapshot.Thermal.State)
	}
	return r.metric("CPU", markStale(value, c.Status))
}

func (r renderer) loadLine() string {
	c := r.data.Snapshot.CPU
	if !c.Status.Available || !c.LoadAvailable {
		return r.metric("LOAD", "unavailable")
	}
	cores := ""
	if r.data.Snapshot.Host.LogicalCores > 0 {
		cores = fmt.Sprintf("  %d logical cores", r.data.Snapshot.Host.LogicalCores)
	}
	return r.metric("LOAD", fmt.Sprintf("%.1f  %.1f  %.1f%s", c.Load1, c.Load5, c.Load15, cores))
}

func (r renderer) coreLine() string {
	values := r.data.Snapshot.CPU.PerCorePercent
	if len(values) == 0 {
		return r.metric("CORES", "unavailable")
	}
	return r.metric("CORES", r.spark(values, minInt(len(values), 24), 100))
}

func (r renderer) thermalLine() string {
	t := r.data.Snapshot.Thermal
	if !t.Status.Available {
		return r.metric("THERM", r.missing(t.Status))
	}
	value := fallback(clean(t.State), "Normal")
	if t.CPUSpeedLimitPercent > 0 && t.CPUSpeedLimitPercent < 100 {
		value += fmt.Sprintf("  CPU limit %d%%", t.CPUSpeedLimitPercent)
	}
	return r.metric("THERM", markStale(value, t.Status))
}

func (r renderer) memoryLine() string {
	m := r.data.Snapshot.Memory
	if !m.Status.Available {
		return r.metric("MEM", r.missing(m.Status))
	}
	value := fmt.Sprintf("%.0f%%", m.UsedPercent)
	history := historyMemory(r.data.History)
	if len(history) >= 2 && r.width >= 52 {
		value += " " + r.spark(history, minInt(12, r.width/7), 100)
	}
	value += "  " + bytes(m.UsedBytes) + "/" + bytes(m.TotalBytes)
	if m.PressureAvailable {
		value += "  pressure " + fallback(clean(m.Pressure), "unknown")
		if r.width >= 70 {
			value += fmt.Sprintf("  %.0f%% headroom", m.AvailablePercent)
		}
	}
	return r.metric("MEM", markStale(value, m.Status))
}

func (r renderer) memoryDetailLine() string {
	m := r.data.Snapshot.Memory
	if !m.Status.Available {
		return r.metric("DETAIL", r.missing(m.Status))
	}
	parts := make([]string, 0, 4)
	if m.PressureAvailable {
		parts = append(parts, fmt.Sprintf("headroom %.0f%%", m.AvailablePercent))
	}
	parts = append(parts, "swap "+bytes(m.SwapUsedBytes))
	if m.CompressionAvailable {
		parts = append(parts, "compressed "+bytes(m.CompressedBytes))
	}
	if m.PageOutRateAvailable && m.PageOutBytesPerSecond > 0 {
		parts = append(parts, "page-outs "+rate(m.PageOutBytesPerSecond))
	}
	return r.metric("DETAIL", markStale(strings.Join(parts, "  "), m.Status))
}

func (r renderer) batteryLine() string {
	b := r.data.Snapshot.Battery
	if !b.Status.Available {
		return r.metric("BAT", r.missing(b.Status))
	}
	if !b.Present {
		return r.metric("BAT", fallback(clean(b.PowerSource), "No battery"))
	}
	value := fmt.Sprintf("%.0f%%", b.Percent)
	if b.State != "" {
		value += " " + clean(b.State)
	}
	if b.TimeRemainingAvailable {
		value += " " + durationMinutes(b.TimeRemainingMinutes)
	}
	if b.DetailsAvailable && b.HealthPercent > 0 {
		value += fmt.Sprintf("  health %.0f%%", b.HealthPercent)
	}
	return r.metric("BAT", markStale(value, b.Status))
}

func (r renderer) diskLine() string {
	d := r.data.Snapshot.Disk
	if !d.Status.Available {
		return r.metric("DISK", r.missing(d.Status))
	}
	freePct := math.Max(0, 100-d.UsedPercent)
	value := fmt.Sprintf("%s free  %.0f%%", bytes(d.FreeBytes), freePct)
	if d.IOAvailable && r.width >= 64 {
		value += "  R " + rate(d.ReadBytesPerSecond) + " W " + rate(d.WriteBytesPerSecond)
	}
	return r.metric("DISK", markStale(value, d.Status))
}

func (r renderer) networkLine() string {
	n := r.data.Snapshot.Network
	if !n.Status.Available {
		return r.metric("NET", r.missing(n.Status))
	}
	value := fallback(clean(n.Interface), "connected")
	if n.RatesAvailable {
		value += "  " + r.glyphs.down + rate(n.DownBytesPerSecond) + " " + r.glyphs.up + rate(n.UpBytesPerSecond)
	}
	return r.metric("NET", markStale(value, n.Status))
}

func (r renderer) culpritLines(height int) []string {
	if height <= 0 {
		return nil
	}
	p := r.data.Snapshot.Processes
	if !p.Status.Available {
		return []string{r.metric("TOP", r.missing(p.Status))}
	}
	if p.WarmingUp {
		return []string{r.metric("TOP", "sampling process activity"+r.glyphs.ellipsis)}
	}
	lines := []string{r.section("CULPRITS")}
	if len(p.TopCPU) > 0 && len(lines) < height {
		row := p.TopCPU[0]
		lines = append(lines, summaryRow("CPU", row.Name, fmt.Sprintf("%.0f%%", row.CPUPercent), r.width, r.glyphs.ellipsis))
	}
	if len(p.TopMemory) > 0 && len(lines) < height {
		row := p.TopMemory[0]
		lines = append(lines, summaryRow("MEM", row.Name, bytes(row.RSSBytes), r.width, r.glyphs.ellipsis))
	}
	if len(lines) == 1 {
		lines = append(lines, r.theme.muted.Render("No process data yet."))
	}
	return take(lines, height)
}

func (r renderer) processes(height int) []string {
	if height <= 0 {
		return nil
	}
	if r.data.Detail {
		return r.processDetail(height)
	}
	if r.data.Group {
		return r.processGroups(height, false)
	}
	p := r.data.Snapshot.Processes
	lines := []string{r.section("PROCESSES / " + strings.ToUpper(string(r.data.ProcessSort)))}
	if !p.Status.Available {
		return take(append(lines, r.missing(p.Status)), height)
	}
	rows := stats.ProcessRows(p, r.data.Query, string(r.data.ProcessSort))
	if len(rows) == 0 {
		return take(append(lines, "No matching processes. / search; Esc clear."), height)
	}
	cursor := minInt(maxInt(r.data.ProcessOffset, 0), len(rows)-1)
	visible := maxInt(1, height-2)
	offset := (cursor / visible) * visible
	end := minInt(offset+visible, len(rows))
	lines[0] = r.section(fmt.Sprintf("PROCESSES / %s / %d-%d of %d", strings.ToUpper(string(r.data.ProcessSort)), offset+1, end, len(rows)))
	if r.width >= 110 {
		lines[0] += " / " + clean(p.Source)
	}
	if r.data.Query != "" {
		lines[0] += " / " + clean(r.data.Query)
	}
	if p.Status.Stale {
		lines[0] += " (STALE)"
	}
	lines = append(lines, "  "+r.processHeader())
	narrow := r
	narrow.width = maxInt(1, r.width-2)
	for i := offset; i < end; i++ {
		prefix := "  "
		line := narrow.processRow(rows[i])
		if i == cursor {
			prefix = "> "
			line = r.theme.tabActive.Render(line)
		}
		lines = append(lines, prefix+line)
	}
	return take(lines, height)
}

func (r renderer) processHeader() string {
	if r.width < 46 {
		if r.data.ProcessSort == SortMemory {
			return r.theme.muted.Render("PID     PROCESS              RSS")
		}
		return r.theme.muted.Render("PID     PROCESS              CPU")
	}
	if r.width < 76 {
		return r.theme.muted.Render("PID     PROCESS                         CPU      RSS")
	}
	return r.theme.muted.Render("PID     PROCESS                                      CPU     MEM       RSS")
}

func (r renderer) processRow(row stats.ProcessRow) string {
	pid := fmt.Sprintf("%-7d", row.PID)
	if r.width < 46 {
		value := fmt.Sprintf("%.0f%%", row.CPUPercent)
		if r.data.ProcessSort == SortMemory {
			value = bytes(row.RSSBytes)
			if r.data.Snapshot.Processes.All != nil && !row.MemoryAvailable {
				value = "--"
			}
		} else if r.data.Snapshot.Processes.All != nil && !row.CPUAvailable {
			value = "--"
		}
		nameW := maxInt(4, r.width-cellWidth(pid)-cellWidth(value)-2)
		return pid + fit(clean(row.Name), nameW, r.glyphs.ellipsis) + "  " + value
	}
	if r.width < 76 {
		cpu := fmt.Sprintf("%5.0f%%", row.CPUPercent)
		if r.data.Snapshot.Processes.All != nil && !row.CPUAvailable {
			cpu = "    --"
		}
		rss := fmt.Sprintf("%8s", bytes(row.RSSBytes))
		if r.data.Snapshot.Processes.All != nil && !row.MemoryAvailable {
			rss = "      --"
		}
		nameW := maxInt(5, r.width-cellWidth(pid)-cellWidth(cpu)-cellWidth(rss)-2)
		return pid + fit(clean(row.Name), nameW, r.glyphs.ellipsis) + " " + cpu + " " + rss
	}
	cpu := fmt.Sprintf("%5.0f%%", row.CPUPercent)
	if r.data.Snapshot.Processes.All != nil && !row.CPUAvailable {
		cpu = "    --"
	}
	mem := fmt.Sprintf("%6.1f%%", row.MemoryPercent)
	rss := fmt.Sprintf("%9s", bytes(row.RSSBytes))
	if r.data.Snapshot.Processes.All != nil && !row.MemoryAvailable {
		rss = "       --"
		mem = "     --"
	}
	nameW := maxInt(8, r.width-cellWidth(pid)-cellWidth(cpu)-cellWidth(mem)-cellWidth(rss)-3)
	return pid + fit(clean(row.Name), nameW, r.glyphs.ellipsis) + " " + cpu + " " + mem + " " + rss
}

func (r renderer) compactProcessRows(rows []stats.ProcessRow, count int, sortBy ProcessSort) []string {
	rows = sortedProcesses(rows, sortBy)
	result := make([]string, 0, minInt(count, len(rows)))
	rowWidth := maxInt(1, (r.width-3)/2)
	for _, row := range rows {
		if len(result) >= count {
			break
		}
		value := fmt.Sprintf("%.0f%%", row.CPUPercent)
		if sortBy == SortMemory {
			value = bytes(row.RSSBytes)
		}
		result = append(result, summaryRow(fmt.Sprintf("%-7d", row.PID), row.Name, value, rowWidth, r.glyphs.ellipsis))
	}
	if len(result) == 0 && count > 0 {
		result = append(result, r.theme.muted.Render("No process data yet."))
	}
	return result
}

func (r renderer) battery(height int) []string {
	if height <= 0 {
		return nil
	}
	b := r.data.Snapshot.Battery
	lines := []string{r.section("BATTERY")}
	if !b.Status.Available {
		return take(append(lines, r.missing(b.Status)), height)
	}
	if !b.Present {
		return take(append(lines, "No internal battery detected.", r.metric("POWER", fallback(clean(b.PowerSource), "Unknown"))), height)
	}

	lines = append(lines,
		r.metric("CHARGE", markStale(fmt.Sprintf("%.0f%%  %s", b.Percent, fallback(clean(b.State), "Unknown")), b.Status)),
	)
	if history := historyBattery(r.data.History); len(history) >= 2 {
		lines = append(lines, r.metric("HISTORY", r.spark(history, minInt(24, maxInt(4, r.width-14)), 100)))
	}
	if b.TimeRemainingAvailable {
		lines = append(lines, r.metric("TIME", durationMinutes(b.TimeRemainingMinutes)+" remaining"))
	}
	if b.DetailsAvailable {
		health := "unavailable"
		if b.HealthPercent > 0 {
			health = fmt.Sprintf("%.0f%%", b.HealthPercent)
		}
		if b.Condition != "" {
			health += "  " + clean(b.Condition)
		}
		lines = append(lines,
			r.metric("HEALTH", health),
			r.metric("CYCLES", fmt.Sprintf("%d", b.CycleCount)),
		)
		if b.TemperatureAvailable {
			unit := " C"
			if !r.data.ASCII {
				unit = "" + string(rune(176)) + "C"
			}
			lines = append(lines, r.metric("TEMP", fmt.Sprintf("%.1f%s", b.TemperatureC, unit)))
		}
	}
	power := fallback(clean(b.PowerSource), "Unknown")
	if b.ChargerWatts > 0 {
		power += fmt.Sprintf("  %d W adapter", b.ChargerWatts)
	}
	lines = append(lines, r.metric("POWER", power), r.metric("LOW POWER", onOff(b.LowPowerMode)))

	for _, issue := range r.data.Snapshot.Health.Issues {
		if strings.HasPrefix(issue.Code, "battery-") && len(lines) < height {
			lines = append(lines, "", r.issueLine(issue))
			break
		}
	}
	return take(lines, height)
}

func (r renderer) system(height int) []string {
	if height <= 0 {
		return nil
	}
	left := r.systemHardware()
	right := r.systemRuntime()
	if r.width >= 110 && r.height >= 18 {
		return joinColumns(left, right, r.width, height, r.glyphs.vertical, r.glyphs.ellipsis)
	}
	lines := append(left, right...)
	return take(lines, height)
}

func (r renderer) systemHardware() []string {
	h := r.data.Snapshot.Host
	title := "HARDWARE"
	if h.Status.Stale {
		title += " (STALE)"
	}
	lines := []string{r.section(title)}
	if !h.Status.Available {
		return append(lines, r.missing(h.Status))
	}
	name := fallback(clean(h.MachineName), clean(h.ComputerName))
	lines = append(lines,
		r.metric("MAC", fallback(name, "Unknown Mac")),
		r.metric("MODEL", fallback(clean(h.ModelIdentifier), "Unknown")),
		r.metric("CHIP", fallback(clean(h.Chip), "Unknown")),
	)
	cores := fmt.Sprintf("%d logical", h.LogicalCores)
	if h.PerformanceCores > 0 || h.EfficiencyCores > 0 {
		cores += fmt.Sprintf("  %dP + %dE", h.PerformanceCores, h.EfficiencyCores)
	}
	lines = append(lines, r.metric("CORES", cores))
	if h.GPU != "" {
		gpu := clean(h.GPU)
		if h.GPUCores > 0 {
			gpu += fmt.Sprintf("  %d cores", h.GPUCores)
		}
		lines = append(lines, r.metric("GPU", gpu))
	}
	return lines
}

func (r renderer) systemRuntime() []string {
	s := r.data.Snapshot
	h := s.Host
	lines := []string{r.section("SYSTEM")}
	os := "macOS " + fallback(clean(h.OSVersion), "unknown")
	if h.OSBuild != "" {
		os += " (" + clean(h.OSBuild) + ")"
	}
	if h.Architecture != "" {
		os += "  " + clean(h.Architecture)
	}
	if h.RunningUnderRosetta {
		os += "  Rosetta"
	}
	lines = append(lines, r.metric("OS", os))
	if s.System.Status.Available {
		lines = append(lines, r.metric("UPTIME", markStale(uptime(s.System.UptimeSeconds), s.System.Status)))
	}
	if h.Hostname != "" {
		lines = append(lines, r.metric("HOST", clean(h.Hostname)))
	}
	lines = append(lines, r.diskLine(), r.networkLine())
	for i, addr := range s.Network.Addresses {
		if i >= 3 {
			lines = append(lines, r.metric("ADDRESS", fmt.Sprintf("+%d more", len(s.Network.Addresses)-i)))
			break
		}
		lines = append(lines, r.metric("ADDRESS", clean(addr.Interface)+"  "+clean(addr.Address)))
	}
	return lines
}

func (r renderer) issueLine(issue stats.Issue) string {
	prefix := "INFO"
	style := r.theme.info
	if issue.Severity == stats.SeverityWarning {
		prefix, style = "WARN", r.theme.warning
	} else if issue.Severity == stats.SeverityCritical {
		prefix, style = "CRIT", r.theme.critical
	}
	return style.Render(prefix+" "+clean(issue.Title)) + "  " + clean(issue.Detail)
}

func (r renderer) metric(label, value string) string {
	labelWidth := 8
	if r.width < 48 {
		labelWidth = 6
	}
	return r.theme.label.Render(padRight(clean(label), labelWidth)) + value
}

func (r renderer) section(s string) string {
	return r.theme.section.Render(clean(s))
}

func (r renderer) missing(status stats.MetricStatus) string {
	if r.data.Loading && !status.Available {
		return "sampling" + r.glyphs.ellipsis
	}
	return unavailable(status)
}

func (r renderer) spark(values []float64, width int, maximum float64) string {
	if width <= 0 || len(values) == 0 {
		return ""
	}
	if len(values) > width {
		values = values[len(values)-width:]
	}
	if maximum <= 0 {
		for _, value := range values {
			if value > maximum && !math.IsNaN(value) && !math.IsInf(value, 0) {
				maximum = value
			}
		}
	}
	if maximum <= 0 {
		maximum = 1
	}
	var b strings.Builder
	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
			value = 0
		}
		if value > maximum {
			value = maximum
		}
		idx := int(math.Round(value / maximum * float64(len(r.glyphs.spark)-1)))
		b.WriteString(r.glyphs.spark[idx])
	}
	return r.theme.spark.Render(b.String())
}

func (r renderer) themeForHealth(status string) lipgloss.Style {
	switch status {
	case "healthy":
		return r.theme.healthy
	case "warning":
		return r.theme.warning
	case "critical":
		return r.theme.critical
	default:
		return r.theme.info
	}
}

// RenderText creates a stable, non-interactive, ANSI-free system report for
// --once output, pipelines, screen readers, and diagnostics.
func RenderText(s stats.Snapshot) string {
	status := strings.ToUpper(fallback(clean(s.Health.Status), "PARTIAL"))
	summary := fallback(clean(s.Health.Summary), "Some telemetry is unavailable")
	lines := []string{
		"macos-health: " + status + " - " + summary,
		textCPU(s),
		textMemory(s),
		textDisk(s),
		textNetwork(s),
		textBattery(s),
		textThermal(s),
	}
	if s.System.Status.Available {
		lines = append(lines, markStale("Uptime: "+uptime(s.System.UptimeSeconds), s.System.Status))
	}
	if s.Host.Status.Available {
		chip := fallback(clean(s.Host.Chip), "unknown chip")
		cores := fmt.Sprintf("%d cores", s.Host.LogicalCores)
		if s.Host.PerformanceCores > 0 || s.Host.EfficiencyCores > 0 {
			cores = fmt.Sprintf("%dP + %dE", s.Host.PerformanceCores, s.Host.EfficiencyCores)
		}
		lines = append(lines, markStale(fmt.Sprintf("System: %s; %s; macOS %s (%s)", chip, cores, clean(s.Host.OSVersion), clean(s.Host.OSBuild)), s.Host.Status))
	}
	if len(s.Health.Issues) > 0 {
		lines = append(lines, "", "Issues:")
		for _, issue := range s.Health.Issues {
			lines = append(lines, fmt.Sprintf("- %s: %s - %s", strings.ToUpper(string(issue.Severity)), clean(issue.Title), clean(issue.Detail)))
		}
	}
	if len(s.Processes.TopCPU) > 0 {
		lines = append(lines, "", "Top CPU processes:")
		for _, row := range firstProcesses(s.Processes.TopCPU, 5) {
			lines = append(lines, fmt.Sprintf("- %d  %s  %.0f%% CPU  %s RSS", row.PID, clean(row.Name), row.CPUPercent, bytes(row.RSSBytes)))
		}
	}
	if len(s.Processes.TopMemory) > 0 {
		lines = append(lines, "", "Top memory processes:")
		for _, row := range firstProcesses(s.Processes.TopMemory, 5) {
			lines = append(lines, fmt.Sprintf("- %d  %s  %s RSS  %.1f%% memory", row.PID, clean(row.Name), bytes(row.RSSBytes), row.MemoryPercent))
		}
	}
	if s.Silicon.Enabled {
		r := renderer{data: Data{Snapshot: s}, width: 100, theme: newTheme(true, true), glyphs: newGlyphs(true)}
		lines = append(lines, "")
		lines = append(lines, r.silicon(12)...)
	}
	return strings.Join(lines, "\n")
}

func textCPU(s stats.Snapshot) string {
	if !s.CPU.Status.Available {
		return "CPU: " + unavailable(s.CPU.Status)
	}
	value := fmt.Sprintf("CPU: %.0f%%", s.CPU.UsagePercent)
	if s.CPU.LoadAvailable {
		value += fmt.Sprintf("; load %.1f %.1f %.1f", s.CPU.Load1, s.CPU.Load5, s.CPU.Load15)
	}
	return markStale(value, s.CPU.Status)
}

func textMemory(s stats.Snapshot) string {
	if !s.Memory.Status.Available {
		return "Memory: " + unavailable(s.Memory.Status)
	}
	value := fmt.Sprintf("Memory: %s / %s (%.0f%%); swap %s", bytes(s.Memory.UsedBytes), bytes(s.Memory.TotalBytes), s.Memory.UsedPercent, bytes(s.Memory.SwapUsedBytes))
	if s.Memory.PressureAvailable {
		value += fmt.Sprintf("; pressure %s; %.0f%% headroom", clean(s.Memory.Pressure), s.Memory.AvailablePercent)
	}
	return markStale(value, s.Memory.Status)
}

func textDisk(s stats.Snapshot) string {
	if !s.Disk.Status.Available {
		return "Disk: " + unavailable(s.Disk.Status)
	}
	value := fmt.Sprintf("Disk: %s free of %s (%.0f%% used)", bytes(s.Disk.FreeBytes), bytes(s.Disk.TotalBytes), s.Disk.UsedPercent)
	if s.Disk.IOAvailable {
		value += "; read " + rate(s.Disk.ReadBytesPerSecond) + "; write " + rate(s.Disk.WriteBytesPerSecond)
	}
	return markStale(value, s.Disk.Status)
}

func textNetwork(s stats.Snapshot) string {
	if !s.Network.Status.Available {
		return "Network: " + unavailable(s.Network.Status)
	}
	value := "Network: " + fallback(clean(s.Network.Interface), "connected")
	if s.Network.RatesAvailable {
		value += "; down " + rate(s.Network.DownBytesPerSecond) + "; up " + rate(s.Network.UpBytesPerSecond)
	}
	return markStale(value, s.Network.Status)
}

func textBattery(s stats.Snapshot) string {
	if !s.Battery.Status.Available {
		return "Battery: " + unavailable(s.Battery.Status)
	}
	if !s.Battery.Present {
		return markStale("Battery: not present; power "+fallback(clean(s.Battery.PowerSource), "unknown"), s.Battery.Status)
	}
	value := fmt.Sprintf("Battery: %.0f%%; %s", s.Battery.Percent, fallback(clean(s.Battery.State), "unknown"))
	if s.Battery.TimeRemainingAvailable {
		value += "; " + durationMinutes(s.Battery.TimeRemainingMinutes) + " remaining"
	}
	if s.Battery.DetailsAvailable {
		if s.Battery.HealthPercent > 0 {
			value += fmt.Sprintf("; %.0f%% health", s.Battery.HealthPercent)
		}
		if s.Battery.CycleCount > 0 {
			value += fmt.Sprintf("; %d cycles", s.Battery.CycleCount)
		}
	}
	return markStale(value, s.Battery.Status)
}

func textThermal(s stats.Snapshot) string {
	if !s.Thermal.Status.Available {
		return "Thermal: " + unavailable(s.Thermal.Status)
	}
	return markStale("Thermal: "+fallback(clean(s.Thermal.State), "normal"), s.Thermal.Status)
}

func sortedProcesses(rows []stats.ProcessRow, by ProcessSort) []stats.ProcessRow {
	rows = append([]stats.ProcessRow(nil), rows...)
	sort.SliceStable(rows, func(i, j int) bool {
		if by == SortMemory {
			if rows[i].RSSBytes == rows[j].RSSBytes {
				return rows[i].CPUPercent > rows[j].CPUPercent
			}
			return rows[i].RSSBytes > rows[j].RSSBytes
		}
		if rows[i].CPUPercent == rows[j].CPUPercent {
			return rows[i].RSSBytes > rows[j].RSSBytes
		}
		return rows[i].CPUPercent > rows[j].CPUPercent
	})
	return rows
}

func topIssue(issues []stats.Issue) *stats.Issue {
	if len(issues) == 0 {
		return nil
	}
	return &issues[0]
}

func firstProcesses(rows []stats.ProcessRow, n int) []stats.ProcessRow {
	if len(rows) < n {
		n = len(rows)
	}
	return rows[:n]
}

func historyCPU(history []stats.Snapshot) []float64 {
	values := make([]float64, 0, len(history))
	for _, sample := range history {
		if sample.CPU.Status.Available && !sample.CPU.Status.Stale && !sample.CPU.WarmingUp {
			values = append(values, sample.CPU.UsagePercent)
		}
	}
	return values
}

func historyMemory(history []stats.Snapshot) []float64 {
	values := make([]float64, 0, len(history))
	for _, sample := range history {
		if sample.Memory.Status.Available && !sample.Memory.Status.Stale {
			values = append(values, sample.Memory.UsedPercent)
		}
	}
	return values
}

func historyBattery(history []stats.Snapshot) []float64 {
	values := make([]float64, 0, len(history))
	for _, sample := range history {
		if sample.Battery.Status.Available && !sample.Battery.Status.Stale && sample.Battery.Present {
			values = append(values, sample.Battery.Percent)
		}
	}
	return values
}

func joinColumns(left, right []string, width, height int, separator, ellipsis string) []string {
	if width < 3 || height <= 0 {
		return nil
	}
	gap := " " + separator + " "
	available := width - cellWidth(gap)
	leftWidth := available / 2
	rightWidth := available - leftWidth
	rows := minInt(height, maxInt(len(left), len(right)))
	result := make([]string, 0, rows)
	for i := 0; i < rows; i++ {
		l, rr := "", ""
		if i < len(left) {
			l = left[i]
		}
		if i < len(right) {
			rr = right[i]
		}
		result = append(result, fit(l, leftWidth, ellipsis)+gap+fit(rr, rightWidth, ellipsis))
	}
	return result
}

func summaryRow(prefix, name, value string, width int, ellipsis string) string {
	prefix = clean(prefix)
	name = clean(name)
	value = clean(value)
	fixed := cellWidth(prefix) + cellWidth(value) + 4
	nameWidth := width - fixed
	if nameWidth < 1 {
		return fit(prefix+"  "+value, width, ellipsis)
	}
	return prefix + "  " + fit(name, nameWidth, ellipsis) + "  " + value
}

func joinSides(left, right string, width int, ellipsis string) string {
	if width <= 0 {
		return ""
	}
	if right == "" {
		return fit(left, width, ellipsis)
	}
	lw, rw := cellWidth(left), cellWidth(right)
	if lw+rw+1 > width {
		leftWidth := maxInt(0, width-rw-1)
		if leftWidth == 0 {
			return fit(right, width, ellipsis)
		}
		return fit(left, leftWidth, ellipsis) + " " + right
	}
	return left + strings.Repeat(" ", width-lw-rw) + right
}

func constrain(lines []string, width, height int, ellipsis string) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	lines = take(lines, height)
	for i := range lines {
		lines[i] = fit(lines[i], width, ellipsis)
	}
	return strings.Join(lines, "\n")
}

func fit(s string, width int, ellipsis string) string {
	if width <= 0 {
		return ""
	}
	if cellWidth(s) > width {
		tail := ellipsis
		if cellWidth(tail) >= width {
			tail = ""
		}
		s = ansi.Truncate(s, width, tail)
	}
	w := cellWidth(s)
	if w < width {
		s += strings.Repeat(" ", width-w)
	}
	return s
}

func padRight(s string, width int) string {
	w := cellWidth(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}

func cellWidth(s string) int { return ansi.StringWidth(s) }

func clean(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	needsClean := false
	for i := 0; i < len(s); i++ {
		b := s[i]
		if b < 0x20 || b == 0x7f || b > 0x7e {
			needsClean = true
			break
		}
	}
	if !needsClean {
		return s
	}
	var b strings.Builder
	lastSpace := false
	for _, r := range s {
		if unicode.IsControl(r) || (r >= 0x7f && r <= 0x9f) || isBidiControl(r) {
			if !lastSpace {
				b.WriteByte(' ')
				lastSpace = true
			}
			continue
		}
		if unicode.IsSpace(r) {
			if !lastSpace {
				b.WriteByte(' ')
				lastSpace = true
			}
			continue
		}
		b.WriteRune(r)
		lastSpace = false
	}
	return strings.TrimSpace(b.String())
}

func unavailable(status stats.MetricStatus) string {
	if status.Stale {
		return "stale"
	}
	if status.Available {
		return "available"
	}
	return "unavailable"
}

func markStale(value string, status stats.MetricStatus) string {
	if status.Stale {
		return "stale  " + value
	}
	return value
}

func isBidiControl(r rune) bool {
	return r == '\u061c' || r == '\u200e' || r == '\u200f' ||
		(r >= '\u202a' && r <= '\u202e') ||
		(r >= '\u2066' && r <= '\u206f')
}

func bytes(value uint64) string {
	const (
		kib = 1024
		mib = 1024 * kib
		gib = 1024 * mib
		tib = 1024 * gib
	)
	switch {
	case value >= tib:
		return fmt.Sprintf("%.1f TiB", float64(value)/tib)
	case value >= gib:
		return fmt.Sprintf("%.1f GiB", float64(value)/gib)
	case value >= mib:
		return fmt.Sprintf("%.0f MiB", float64(value)/mib)
	case value >= kib:
		return fmt.Sprintf("%.0f KiB", float64(value)/kib)
	default:
		return fmt.Sprintf("%d B", value)
	}
}

func rate(value float64) string {
	if value < 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		value = 0
	}
	return bytes(uint64(value)) + "/s"
}

func durationMinutes(minutes int) string {
	if minutes < 0 {
		minutes = 0
	}
	if minutes < 60 {
		return fmt.Sprintf("%dm", minutes)
	}
	return fmt.Sprintf("%dh%02dm", minutes/60, minutes%60)
}

func uptime(seconds uint64) string {
	days := seconds / 86400
	hours := (seconds % 86400) / 3600
	minutes := (seconds % 3600) / 60
	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	}
	return fmt.Sprintf("%dh %dm", hours, minutes)
}

func onOff(value bool) string {
	if value {
		return "On"
	}
	return "Off"
}

func fallback(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func take(lines []string, n int) []string {
	if n <= 0 {
		return nil
	}
	if len(lines) <= n {
		return lines
	}
	return lines[:n]
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
