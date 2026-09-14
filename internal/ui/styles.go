package ui

import "github.com/charmbracelet/lipgloss"

type theme struct {
	title     lipgloss.Style
	section   lipgloss.Style
	label     lipgloss.Style
	muted     lipgloss.Style
	tab       lipgloss.Style
	tabActive lipgloss.Style
	healthy   lipgloss.Style
	warning   lipgloss.Style
	critical  lipgloss.Style
	info      lipgloss.Style
	spark     lipgloss.Style
}

func newTheme(noColor, plain bool) theme {
	if plain {
		return theme{}
	}

	t := theme{
		title:     lipgloss.NewStyle().Bold(true),
		section:   lipgloss.NewStyle().Bold(true),
		label:     lipgloss.NewStyle().Bold(true),
		muted:     lipgloss.NewStyle(),
		tab:       lipgloss.NewStyle(),
		tabActive: lipgloss.NewStyle().Bold(true),
		healthy:   lipgloss.NewStyle().Bold(true),
		warning:   lipgloss.NewStyle().Bold(true),
		critical:  lipgloss.NewStyle().Bold(true),
		info:      lipgloss.NewStyle().Bold(true),
		spark:     lipgloss.NewStyle(),
	}
	if noColor {
		return t
	}

	accent := lipgloss.AdaptiveColor{Light: "#006D77", Dark: "#7DD3FC"}
	muted := lipgloss.AdaptiveColor{Light: "#626262", Dark: "#9CA3AF"}
	green := lipgloss.AdaptiveColor{Light: "#147D31", Dark: "#56D364"}
	yellow := lipgloss.AdaptiveColor{Light: "#8A5A00", Dark: "#E3B341"}
	red := lipgloss.AdaptiveColor{Light: "#B42318", Dark: "#FF7B72"}

	t.title = t.title.Foreground(accent)
	t.section = t.section.Foreground(accent)
	t.label = t.label.Foreground(muted)
	t.muted = t.muted.Foreground(muted)
	t.tab = t.tab.Foreground(muted)
	t.tabActive = t.tabActive.Foreground(accent)
	t.healthy = t.healthy.Foreground(green)
	t.warning = t.warning.Foreground(yellow)
	t.critical = t.critical.Foreground(red)
	t.info = t.info.Foreground(accent)
	t.spark = t.spark.Foreground(accent)
	return t
}

type glyphSet struct {
	down     string
	up       string
	vertical string
	ellipsis string
	spark    []string
}

func newGlyphs(ascii bool) glyphSet {
	if ascii {
		return glyphSet{
			down:     "down ",
			up:       "up ",
			vertical: "|",
			ellipsis: "...",
			spark:    []string{".", ".", ":", "-", "=", "+", "*", "#", "@"},
		}
	}
	return glyphSet{
		down:     "↓",
		up:       "↑",
		vertical: "│",
		ellipsis: "…",
		spark:    []string{"▁", "▂", "▃", "▄", "▅", "▆", "▇", "█"},
	}
}

func namedTheme(noColor, plain bool, name string) theme {
	t := newTheme(noColor, plain)
	if noColor || plain || name == "" || name == "ocean" {
		return t
	}
	color := lipgloss.AdaptiveColor{Light: "#9A5700", Dark: "#F2C879"}
	if name == "violet" {
		color = lipgloss.AdaptiveColor{Light: "#7040AA", Dark: "#C4A7E7"}
	}
	t.title = t.title.Foreground(color)
	t.section = t.section.Foreground(color)
	t.tabActive = t.tabActive.Foreground(color)
	t.spark = t.spark.Foreground(color)
	t.info = t.info.Foreground(color)
	return t
}
