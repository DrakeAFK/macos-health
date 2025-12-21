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

	boxW := min(width-2, 110)

	systemBox := BoxStyle.Width(boxW).Render(renderSystemSection(s))
	hostBox := BoxStyle.Width(boxW).Render(renderHostSection(s))
	cpuBox := BoxStyle.Width(boxW).Render(renderTopCPUSection(s))
	memBox := BoxStyle.Width(boxW).Render(renderTopMemSection(s))

	return systemBox + "\n\n" + hostBox + "\n\n" + cpuBox + "\n\n" + memBox
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

func renderHostSection(s stats.Snapshot) string {
	h := s.Host

	name := h.ComputerName
	if name == "" {
		name = "n/a"
	}
	host := h.Hostname
	if host == "" {
		host = "n/a"
	}

	osLine := "n/a"
	if h.OSVersion != "" && h.OSBuild != "" {
		osLine = fmt.Sprintf("macOS %s (%s)  %s", h.OSVersion, h.OSBuild, h.Arch)
	} else if h.OSVersion != "" {
		osLine = fmt.Sprintf("macOS %s  %s", h.OSVersion, h.Arch)
	} else if h.Arch != "" {
		osLine = h.Arch
	}

	cpuLine := "n/a"
	if h.CPUModel != "" && h.CPUCores > 0 {
		cpuLine = fmt.Sprintf("%s (%d cores)", h.CPUModel, h.CPUCores)
	} else if h.CPUModel != "" {
		cpuLine = h.CPUModel
	} else if h.CPUCores > 0 {
		cpuLine = fmt.Sprintf("%d cores", h.CPUCores)
	}

	gpuLine := h.GPUModel
	if gpuLine == "" {
		gpuLine = "n/a"
	}

	pwr := h.PowerSource
	if pwr == "" {
		pwr = "n/a"
	}

	ssid := h.WifiSSID
	if ssid == "" {
		ssid = "n/a"
	}

	ipLine := formatIPLine(h.IPs)
	if ipLine == "" {
		ipLine = "n/a"
	}

	lines := []string{
		SectionTitleStyle.Render("HOST"),
		fmt.Sprintf("NAME: %s", name),
		fmt.Sprintf("HOST: %s", host),
		fmt.Sprintf("WIFI: %s", ssid),
		fmt.Sprintf("IP:   %s", ipLine),
		fmt.Sprintf("OS:   %s", osLine),
		fmt.Sprintf("CPU:  %s", cpuLine),
		fmt.Sprintf("GPU:  %s", gpuLine),
		fmt.Sprintf("PWR:  %s", pwr),
	}

	return strings.Join(lines, "\n")
}

func formatIPLine(ips []stats.IPAddr) string {
	if len(ips) == 0 {
		return ""
	}

	max := 4
	if len(ips) < max {
		max = len(ips)
	}

	parts := make([]string, 0, max)
	for i := 0; i < max; i++ {
		parts = append(parts, fmt.Sprintf("%s %s", ips[i].Iface, ips[i].Addr))
	}
	if len(ips) > max {
		parts = append(parts, fmt.Sprintf("+%d more", len(ips)-max))
	}
	return strings.Join(parts, " | ")
}

func renderTopCPUSection(s stats.Snapshot) string {
	lines := []string{SectionTitleStyle.Render("TOP CPU PROCESSES")}
	if len(s.TopCPU) == 0 {
		lines = append(lines, MutedStyle.Render("No process data yet."))
		return strings.Join(lines, "\n")
	}

	for _, p := range s.TopCPU {
		lines = append(lines,
			fmt.Sprintf("%-18s %7.0f%% CPU   %6.1f%% MEM",
				trimTo(p.Name, 18),
				p.CPUPercent,
				p.MemPercent,
			),
		)
	}
	return strings.Join(lines, "\n")
}

func renderTopMemSection(s stats.Snapshot) string {
	lines := []string{SectionTitleStyle.Render("TOP MEM PROCESSES")}
	if len(s.TopMem) == 0 {
		lines = append(lines, MutedStyle.Render("No process data yet."))
		return strings.Join(lines, "\n")
	}

	for _, p := range s.TopMem {
		lines = append(lines,
			fmt.Sprintf("%-18s %7.0f%% CPU   %6.1f%% MEM",
				trimTo(p.Name, 18),
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
