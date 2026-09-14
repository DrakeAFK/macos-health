package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/drakeafk/macos-health/internal/stats"
	"github.com/drakeafk/macos-health/internal/ui"
)

const historyLimit = 3600

type SnapshotCollector interface {
	Collect(context.Context) (stats.Snapshot, error)
}

type Config struct {
	Context   context.Context
	Collector SnapshotCollector
	Interval  time.Duration
	Timeout   time.Duration
	NoColor   bool
	ASCII     bool
	Redact    bool
	Theme     string
	Page      ui.Page
	Group     bool
	Replay    bool
	Save      func(ui.Page, string, bool, bool) error
}

type Model struct {
	context   context.Context
	collector SnapshotCollector
	interval  time.Duration
	timeout   time.Duration

	width  int
	height int

	snapshot      stats.Snapshot
	history       []stats.Snapshot
	healthHistory []stats.Snapshot
	err           error

	page           ui.Page
	processSort    ui.ProcessSort
	processOffset  int
	paused         bool
	showHelp       bool
	theme          string
	group          bool
	replay         bool
	ended          bool
	query          string
	searching      bool
	selectedPID    int32
	selectedBirth  time.Time
	detail         bool
	processHistory []ui.ProcessPoint
	events         []stats.Event
	window         time.Duration
	notice         string
	save           func(ui.Page, string, bool, bool) error
	redact         bool
	noColor        bool
	ascii          bool

	inFlight  bool
	requestID uint64
}

type tickMsg time.Time

type snapshotMsg struct {
	id       uint64
	snapshot stats.Snapshot
	err      error
}

func NewModel(config Config) Model {
	interval := config.Interval
	if interval <= 0 {
		interval = time.Second
	}
	timeout := config.Timeout
	if timeout <= 0 {
		timeout = 1500 * time.Millisecond
	}
	if config.Context == nil {
		config.Context = context.Background()
	}
	collector := config.Collector
	if collector == nil {
		collector = stats.NewCollector()
	}
	return Model{
		collector:   collector,
		context:     config.Context,
		interval:    interval,
		timeout:     timeout,
		width:       80,
		height:      24,
		processSort: ui.SortCPU,
		theme:       config.Theme, page: config.Page, group: config.Group, replay: config.Replay, save: config.Save, window: 5 * time.Minute,
		redact:    config.Redact,
		noColor:   config.NoColor,
		ascii:     config.ASCII,
		inFlight:  true,
		requestID: 1,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.fetch(1), tick(m.interval))
}

func tick(interval time.Duration) tea.Cmd {
	return tea.Tick(interval, func(now time.Time) tea.Msg { return tickMsg(now) })
}

func (m Model) fetch(id uint64) tea.Cmd {
	collector := m.collector
	parent := m.context
	timeout := m.timeout
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(parent, timeout)
		defer cancel()
		snapshot, err := collector.Collect(ctx)
		return snapshotMsg{id: id, snapshot: snapshot, err: err}
	}
}

func (m Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch message := message.(type) {
	case tea.WindowSizeMsg:
		m.width = message.Width
		m.height = message.Height
		return m, nil

	case tickMsg:
		commands := []tea.Cmd{tick(m.interval)}
		if !m.paused && !m.inFlight && !m.ended {
			m.requestID++
			m.inFlight = true
			commands = append(commands, m.fetch(m.requestID))
		}
		return m, tea.Batch(commands...)

	case snapshotMsg:
		if message.id != m.requestID {
			return m, nil
		}
		m.inFlight = false
		m.err = message.err
		if errors.Is(message.err, io.EOF) {
			m.ended = true
			m.paused = true
			m.err = nil
			m.notice = "Replay complete - history remains available"
			return m, nil
		}
		if message.snapshot.SampledAt.IsZero() && message.err != nil {
			m.ended = m.replay
			m.notice = "Collection failed"
			return m, nil
		}
		next := message.snapshot
		if !m.snapshot.SampledAt.IsZero() {
			next = stats.MergeLastGood(next, m.snapshot)
		}
		if !m.replay {
			next.Health = stats.Assess(next, m.healthHistory)
		}
		m.events = append(m.events, stats.Events(m.snapshot, next)...)
		if len(m.events) > 200 {
			m.events = append([]stats.Event(nil), m.events[len(m.events)-200:]...)
		}
		m.snapshot = next
		m.healthHistory = append(m.healthHistory, stats.Trend(next))
		if len(m.healthHistory) > 64 {
			copy(m.healthHistory, m.healthHistory[len(m.healthHistory)-64:])
			m.healthHistory = m.healthHistory[:64]
		}
		if !next.SampledAt.IsZero() {
			if len(m.history) == 0 || next.SampledAt.Sub(m.history[len(m.history)-1].SampledAt) >= time.Second {
				m.history = append(m.history, stats.Trend(next))
			}
			if len(m.history) > historyLimit {
				copy(m.history, m.history[len(m.history)-historyLimit:])
				m.history = m.history[:historyLimit]
			}
		}
		m.trackProcess(next)
		return m, nil

	case tea.KeyMsg:
		key := message.String()
		if m.searching {
			switch key {
			case "ctrl+c":
				return m, tea.Quit
			case "enter":
				m.searching = false
				m.processOffset = 0
			case "esc":
				m.searching = false
				m.query = ""
				m.processOffset = 0
			case "backspace", "ctrl+h":
				if len(m.query) > 0 {
					_, n := utf8.DecodeLastRuneInString(m.query)
					m.query = m.query[:len(m.query)-n]
				}
			default:
				if message.Type == tea.KeyRunes && len(m.query) < 128 {
					m.query += string(message.Runes)
				}
			}
			return m, nil
		}
		if key == "esc" && m.detail {
			m.detail = false
			m.processHistory = nil
			return m, nil
		}

		if m.showHelp {
			switch key {
			case "?", "esc":
				m.showHelp = false
				return m, nil
			}
		}

		switch key {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "?":
			m.showHelp = !m.showHelp
		case " ":
			m.paused = !m.paused
		case "r":
			if !m.inFlight && !m.ended {
				m.requestID++
				m.inFlight = true
				return m, m.fetch(m.requestID)
			}
		case "/":
			m.page = ui.PageProcesses
			m.searching = true
			m.detail = false
		case "enter":
			if m.page == ui.PageProcesses && !m.group {
				rows := stats.ProcessRows(m.snapshot.Processes, m.query, string(m.processSort))
				if len(rows) > 0 {
					i := min(m.processOffset, len(rows)-1)
					m.selectedPID = rows[i].PID
					m.selectedBirth = m.snapshot.Processes.Status.SampledAt.Add(-time.Duration(rows[i].AgeSeconds * float64(time.Second)))
					if !rows[i].StartedAt.IsZero() {
						m.selectedBirth = rows[i].StartedAt
					}
					m.detail = true
					m.processHistory = nil
					m.trackProcess(m.snapshot)
				}
			}
		case "esc":
			m.query = ""
			m.processOffset = 0
			m.notice = ""
		case "a":
			m.group = !m.group
			m.processOffset = 0
			m.detail = false
		case "t":
			switch m.theme {
			case "amber":
				m.theme = "violet"
			case "violet":
				m.theme = "ocean"
			default:
				m.theme = "amber"
			}
		case "w":
			if m.save != nil {
				if err := m.save(m.page, m.theme, m.redact, m.group); err != nil {
					m.notice = "Save failed: " + err.Error()
				} else {
					m.notice = "Preferences saved"
				}
			}
		case "[":
			if m.window > time.Minute {
				m.window /= 5
				if m.window < time.Minute {
					m.window = time.Minute
				}
			}
		case "]":
			if m.window < time.Hour {
				m.window *= 5
				if m.window > time.Hour {
					m.window = time.Hour
				}
			}
		case "5":
			m.page = ui.PageSilicon
		case "6":
			m.page = ui.PageHistory
		case "7":
			m.page = ui.PageInsights
		case "8":
			m.page = ui.PageWorkloads
		case "1":
			m.page = ui.PageOverview
			m.processOffset = 0
		case "2":
			m.page = ui.PageProcesses
			m.processOffset = 0
		case "3":
			m.page = ui.PageBattery
			m.processOffset = 0
		case "4":
			m.page = ui.PageSystem
			m.processOffset = 0
		case "tab", "l", "right":
			m.page = (m.page + 1) % ui.PageCount
			m.processOffset = 0
		case "shift+tab", "h", "left":
			m.page = (m.page + ui.PageCount - 1) % ui.PageCount
			m.processOffset = 0
		case "j", "down":
			if m.page == ui.PageProcesses {
				m.processOffset++
				if n := len(stats.ProcessRows(m.snapshot.Processes, m.query, string(m.processSort))); n > 0 && m.processOffset >= n {
					m.processOffset = n - 1
				}
			}
		case "k", "up":
			if m.page == ui.PageProcesses && m.processOffset > 0 {
				m.processOffset--
			}
		case "pgdown":
			if m.page == ui.PageProcesses {
				m.processOffset += 10
				if n := len(stats.ProcessRows(m.snapshot.Processes, m.query, string(m.processSort))); n > 0 && m.processOffset >= n {
					m.processOffset = n - 1
				}
			}
		case "pgup":
			if m.page == ui.PageProcesses {
				m.processOffset -= 10
				if m.processOffset < 0 {
					m.processOffset = 0
				}
			}
		case "home", "g":
			if m.page == ui.PageProcesses {
				m.processOffset = 0
			}
		case "s":
			m.processOffset = 0
			if m.processSort == ui.SortCPU {
				m.processSort = ui.SortMemory
			} else {
				m.processSort = ui.SortCPU
			}
		case "x":
			m.redact = !m.redact
		}
	}
	return m, nil
}

func (m Model) View() string {
	snapshot := m.snapshot
	if m.redact {
		snapshot = snapshot.Redacted()
	}
	return ui.Render(ui.Data{
		Snapshot: snapshot,
		History:  m.history,
		Theme:    m.theme, Group: m.group, Query: m.query, Searching: m.searching, Detail: m.detail, SelectedPID: m.selectedPID, ProcessHistory: m.processHistory, Events: m.events, Window: m.window, Notice: m.notice, Replay: m.replay,
		Width:         m.width,
		Height:        m.height,
		Page:          m.page,
		ProcessSort:   m.processSort,
		ProcessOffset: m.processOffset,
		Loading:       snapshot.SampledAt.IsZero(),
		Paused:        m.paused,
		ShowHelp:      m.showHelp,
		NoColor:       m.noColor,
		ASCII:         m.ascii,
		Error:         m.err,
	})
}

func (m *Model) trackProcess(s stats.Snapshot) {
	if !m.detail {
		return
	}
	for _, p := range s.Processes.All {
		if p.PID == m.selectedPID {
			birth := s.Processes.Status.SampledAt.Add(-time.Duration(p.AgeSeconds * float64(time.Second)))
			if (!p.StartedAt.IsZero() && !p.StartedAt.Equal(m.selectedBirth)) || (p.StartedAt.IsZero() && (birth.Sub(m.selectedBirth) > 3*time.Second || m.selectedBirth.Sub(birth) > 3*time.Second)) {
				m.notice = fmt.Sprintf("PID %d was reused; reopen its details", p.PID)
				return
			}
			if len(m.processHistory) == 0 || s.Processes.Status.SampledAt.After(m.processHistory[len(m.processHistory)-1].At) {
				m.processHistory = append(m.processHistory, ui.ProcessPoint{At: s.Processes.Status.SampledAt, Row: p})
				if len(m.processHistory) > 600 {
					m.processHistory = append([]ui.ProcessPoint(nil), m.processHistory[len(m.processHistory)-600:]...)
				}
			}
			return
		}
	}
}
