package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type tickMsg time.Time

type Model struct {
	Title          string
	SpecPath       string
	RefreshEveryMs int

	StartedAt time.Time
	Now       time.Time
	Status    string
	Width     int
	Height    int
}

func NewModel(title, specPath string, refreshEveryMs int) Model {
	now := time.Now()
	return Model{
		Title:          title,
		SpecPath:       specPath,
		RefreshEveryMs: refreshEveryMs,
		StartedAt:      now,
		Now:            now,
		Status:         "Running (spec loaded; no MCP yet)",
	}
}

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m Model) Init() tea.Cmd {
	return tick()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	case tickMsg:
		m.Now = time.Time(msg)
		return m, tick()
	}
	return m, nil
}

func (m Model) View() string {
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39")).Render(m.Title)
	uptime := m.Now.Sub(m.StartedAt).Round(time.Second).String()

	refresh := "default (1000ms)"
	if m.RefreshEveryMs > 0 {
		refresh = fmt.Sprintf("%dms", m.RefreshEveryMs)
	}

	body := fmt.Sprintf(
		"Spec:    %s\nStatus:  %s\nRefresh: %s\nUptime:  %s\n\nPress q to quit.",
		m.SpecPath,
		m.Status,
		refresh,
		uptime,
	)

	return fmt.Sprintf("%s\n\n%s", title, body)
}
