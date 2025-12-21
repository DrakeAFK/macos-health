package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/drakeafk/macos-health/internal/stats"
)

func Header(last time.Time, err error) string {
	left := TitleStyle.Render("macos-health")
	right := MutedStyle.Render("q: quit")

	mid := ""
	if !last.IsZero() {
		mid = MutedStyle.Render(fmt.Sprintf("updated: %s", last.Format("15:04:05")))
	}

	line := join3(left, mid, right, 80)

	if err != nil {
		line += "\n" + ErrorStyle.Render("error: "+err.Error())
	}
	return line
}

func Footer() string {
	return MutedStyle.Render("Keys: q / Ctrl+C to quit")
}

func Dashboard(s stats.Snapshot, width int) string {
	if width <= 0 {
		width = 80
	}

	systemBox := BoxStyle.Width(min(width-2, 80)).Render(renderSystemSection(s))
	procBox := BoxStyle.Width(min(width-2, 80)).Render(renderProcessSection(s))

	return systemBox + "\n\n" + procBox
}

func renderSystemSection(s stats.Snapshot) string {
	lines := []string{
		SectionTitleStyle.Render("SYSTEM"),
		fmt.Sprintf("CPU:  %s    Load: %s", s.CPUString(), s.LoadString()),
		fmt.Sprintf("MEM:  %s", s.MemString()),
		fmt.Sprintf("DISK: %s", s.DiskString()),
		fmt.Sprintf("NET:  %s", s.NetString()),
		fmt.Sprintf("BAT:  %s", s.BatteryString()),
		fmt.Sprintf("UPT:  %s", s.UptimeString()),
	}
	return strings.Join(lines, "\n")
}

func renderProcessSection(s stats.Snapshot) string {
	lines := []string{SectionTitleStyle.Render("TOP PROCESSES")}
	if len(s.Top) == 0 {
		lines = append(lines, MutedStyle.Render("No process data yet."))
		return strings.Join(lines, "\n")
	}

	for _, p := range s.Top {
		lines = append(lines,
			fmt.Sprintf("%-14s %7.0f%% CPU   %6.1f%% MEM",
				trimTo(p.Name, 14),
				p.CPUPercent,
				p.MemPercent,
			),
		)
	}
	return strings.Join(lines, "\n")
}

func join3(a, b, c string, width int) string {
	if width <= 0 {
		width = 80
	}
	space := width - lipgloss.Width(a) - lipgloss.Width(b) - lipgloss.Width(c)
	if space < 2 {
		return a + " " + b + " " + c
	}
	leftSpace := space / 2
	rightSpace := space - leftSpace
	return a + strings.Repeat(" ", leftSpace) + b + strings.Repeat(" ", rightSpace) + c
}

func trimTo(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
