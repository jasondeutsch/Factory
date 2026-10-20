package monitor

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type tickMsg time.Time
type fetchedMsg struct {
	generation int
	data       snapshot
	err        error
}

type Model struct {
	client                    *Client
	ctx                       context.Context
	cancel                    context.CancelFunc
	project                   string
	before                    int64
	selected                  string
	data                      snapshot
	width, height             int
	generation                int
	busy, paused, detailFocus bool
	tab, scroll               int
	err                       error
	updated                   time.Time
}

func New(ctx context.Context, client *Client, project string) Model {
	return Model{ctx: ctx, client: client, project: project, width: 110, height: 34}
}
func tick() tea.Cmd { return tea.Tick(3*time.Second, func(t time.Time) tea.Msg { return tickMsg(t) }) }
func (m *Model) fetch() tea.Cmd {
	if m.cancel != nil {
		m.cancel()
	}
	ctx, cancel := context.WithCancel(m.ctx)
	m.cancel = cancel
	m.generation++
	generation, project, selected, before := m.generation, m.project, m.selected, m.before
	m.busy = true
	return func() tea.Msg {
		data, err := m.client.fetch(ctx, project, selected, before)
		return fetchedMsg{generation, data, err}
	}
}
func (m Model) Init() tea.Cmd {
	return tea.Batch(tick(), func() tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}} })
}
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case fetchedMsg:
		if msg.generation != m.generation {
			return m, nil
		}
		m.busy = false
		m.err = msg.err
		if msg.err == nil {
			m.data = msg.data
			m.selected = msg.data.selected
			m.updated = time.Now()
		}
	case tickMsg:
		if !m.paused && !m.busy {
			cmd := m.fetch()
			return m, tea.Batch(cmd, tick())
		}
		return m, tick()
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			if m.cancel != nil {
				m.cancel()
			}
			return m, tea.Quit
		case "r":
			cmd := m.fetch()
			return m, cmd
		case " ":
			m.paused = !m.paused
		case "tab":
			m.detailFocus = !m.detailFocus
		case "1", "2", "3":
			m.tab = int(msg.String()[0] - '1')
			m.scroll = 0
		case "pgdown":
			m.scroll += max(1, m.height-14)
		case "pgup":
			m.scroll = max(0, m.scroll-max(1, m.height-14))
		case "home":
			m.scroll = 0
		case "down", "j", "up", "k":
			delta := 1
			if msg.String() == "up" || msg.String() == "k" {
				delta = -1
			}
			if m.detailFocus {
				m.scroll = max(0, m.scroll+delta)
				break
			}
			index := m.selectedIndex() + delta
			if index >= 0 && index < len(m.data.tasks) {
				m.selected = m.data.tasks[index].Task.ID
				m.data.detail = nil
				m.scroll = 0
				cmd := m.fetch()
				return m, cmd
			}
		case "p":
			if len(m.data.projects) == 0 {
				cmd := m.fetch()
				return m, cmd
			}
			index := -1
			for i, p := range m.data.projects {
				if p == m.project {
					index = i
				}
			}
			m.project = m.data.projects[(index+1)%len(m.data.projects)]
			m.before = 0
			m.selected = ""
			m.data.tasks = nil
			m.data.detail = nil
			m.scroll = 0
			cmd := m.fetch()
			return m, cmd
		case "n":
			if len(m.data.tasks) == 50 {
				m.before = m.data.tasks[49].Task.Sequence
				m.selected = ""
				m.data.tasks = nil
				m.data.detail = nil
				m.scroll = 0
				cmd := m.fetch()
				return m, cmd
			}
		case "b":
			m.before = 0
			m.selected = ""
			m.data.tasks = nil
			m.data.detail = nil
			m.scroll = 0
			cmd := m.fetch()
			return m, cmd
		}
	}
	return m, nil
}
func (m Model) selectedIndex() int {
	for i, t := range m.data.tasks {
		if t.Task.ID == m.selected {
			return i
		}
	}
	return 0
}
