package ui

import "github.com/charmbracelet/lipgloss"

var (
	TitleStyle = lipgloss.NewStyle().
			Bold(true)

	MutedStyle = lipgloss.NewStyle().
			Faint(true)

	ErrorStyle = lipgloss.NewStyle().
			Bold(true)

	BoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(0, 1)

	SectionTitleStyle = lipgloss.NewStyle().
				Bold(true)
)
