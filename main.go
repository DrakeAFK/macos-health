package main

import (
	"context"
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/drakeafk/macos-health/internal/stats"
	"github.com/drakeafk/macos-health/internal/ui"
)

type model struct {
	width  int
	height int

	lastUpdated time.Time
	snapshot    stats.Snapshot
	err         error
}

type tickMsg time.Time
type snapMsg struct {
	snap stats.Snapshot
	err  error
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m model) Init() tea.Cmd {
	return tea.Batch(fetchSnapshot(), tickCmd())
}

func fetchSnapshot() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2500*time.Millisecond)
		defer cancel()

		snap, err := stats.Collect(ctx)
		return snapMsg{snap: snap, err: err}
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tickMsg:
		return m, tea.Batch(fetchSnapshot(), tickCmd())

	case snapMsg:
		m.lastUpdated = time.Now()
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.err = nil
		m.snapshot = msg.snap
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m model) View() string {
	header := ui.Header(m.lastUpdated, m.err)
	body := ui.Dashboard(m.snapshot, m.width)
	footer := ui.Footer()
	return fmt.Sprintf("%s\n%s\n%s\n", header, body, footer)
}

func main() {
	p := tea.NewProgram(model{}, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
