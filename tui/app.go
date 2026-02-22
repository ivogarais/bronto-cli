package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type tickMsg time.Time

type Model struct {
	title   string
	spec    string
	started time.Time
	now     time.Time
}

func NewModel(title, spec string) Model {
	t := time.Now()
	return Model{
		title:   title,
		spec:    spec,
		started: t,
		now:     t,
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
		m.now = time.Time(msg)
		return m, tick()
	}
	return m, nil
}

func (m Model) View() string {
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39")).Render(m.title)
	label := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	uptime := m.now.Sub(m.started).Round(time.Second).String()

	return fmt.Sprintf(
		"%s\n\n%s %s\n%s %s\n%s %s\n\n%s",
		title,
		label.Render("Spec:"), m.spec,
		label.Render("Started:"), m.started.Format(time.RFC3339),
		label.Render("Uptime:"), uptime,
		label.Render("Press q or ctrl+c to quit."),
	)
}
