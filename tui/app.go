package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/NimbleMarkets/ntcharts/v2/barchart"

	"github.com/ivogarais/bronto-cli/spec"
)

type tickMsg time.Time

type Model struct {
	Title          string
	SpecPath       string
	RefreshEveryMs int
	Widgets        []spec.Widget
	Bar            barchart.Model
	Tables         map[string]table.Model

	StartedAt time.Time
	Now       time.Time
	Status    string
	Width     int
	Height    int
}

func NewModel(s *spec.DashboardSpec, specPath string) Model {
	now := time.Now()
	bar := barchart.New(50, 12)
	setFakeBarData(&bar)
	tables := make(map[string]table.Model)

	for _, w := range s.Widgets {
		if w.Type != "table" {
			continue
		}

		cols := make([]table.Column, 0, len(w.Columns))
		for _, c := range w.Columns {
			cols = append(cols, table.Column{Title: c, Width: max(10, len(c)+2)})
		}

		rawRows := [][]string{
			{"2026-02-22T12:00:01Z", "api", "NullPointerException"},
			{"2026-02-22T12:00:03Z", "worker", "timeout contacting db"},
			{"2026-02-22T12:00:04Z", "web", "500 /checkout"},
		}
		rows := make([]table.Row, 0, len(rawRows))
		for _, rr := range rawRows {
			row := make(table.Row, len(w.Columns))
			for i := range w.Columns {
				if i < len(rr) {
					row[i] = rr[i]
				} else {
					row[i] = "-"
				}
			}
			rows = append(rows, row)
		}

		t := table.New(
			table.WithColumns(cols),
			table.WithRows(rows),
			table.WithFocused(true),
			table.WithWidth(50),
			table.WithHeight(8),
		)
		tables[w.ID] = t
	}

	return Model{
		Title:          s.Title,
		SpecPath:       specPath,
		RefreshEveryMs: s.Refresh.EveryMs,
		Widgets:        s.Widgets,
		Bar:            bar,
		Tables:         tables,
		StartedAt:      now,
		Now:            now,
		Status:         "Running (ntcharts + bubbles/table; fake data)",
	}
}

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func setFakeBarData(m *barchart.Model) {
	labels := []string{"api", "worker", "web", "db"}
	values := []float64{120, 80, 60, 20}

	data := make([]barchart.BarData, 0, len(labels))
	for i, label := range labels {
		data = append(data, barchart.BarData{
			Label: label,
			Values: []barchart.BarValue{
				{Name: label, Value: values[i]},
			},
		})
	}

	m.Clear()
	m.PushAll(data)
	m.Draw()
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
		for id, t := range m.Tables {
			var cmd tea.Cmd
			t, cmd = t.Update(msg)
			m.Tables[id] = t
			if cmd != nil {
				return m, cmd
			}
		}
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height

		chartW := msg.Width - 6
		if chartW < 20 {
			chartW = 20
		}
		chartH := 12
		m.Bar.Resize(chartW, chartH)
		m.Bar.Draw()

		for id, t := range m.Tables {
			cols := t.Columns()
			if len(cols) == 0 {
				continue
			}

			available := msg.Width - 10
			if available < 40 {
				available = 40
			}
			per := available / len(cols)
			if per < 10 {
				per = 10
			}

			for i := range cols {
				cols[i].Width = per
			}
			t.SetColumns(cols)
			t.SetWidth(available)
			t.SetHeight(10)
			m.Tables[id] = t
		}
	case tickMsg:
		m.Now = time.Time(msg)
		setFakeBarData(&m.Bar)
		return m, tick()
	}
	return m, nil
}

func (m Model) View() tea.View {
	titleStyle := lipgloss.NewStyle().Bold(true).Padding(0, 1)

	headerBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(0, 1)

	panelStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1, 2)

	uptime := m.Now.Sub(m.StartedAt).Truncate(time.Second)

	refresh := "default (1000ms)"
	if m.RefreshEveryMs > 0 {
		refresh = fmt.Sprintf("%dms", m.RefreshEveryMs)
	}

	header := headerBox.Render(
		fmt.Sprintf("%s\nSpec: %s   Refresh: %s   Uptime: %s\nStatus: %s\n(press q to quit)",
			titleStyle.Render(m.Title),
			m.SpecPath,
			refresh,
			uptime,
			m.Status,
		),
	)

	var panels []string
	for _, w := range m.Widgets {
		switch w.Type {
		case "barchart":
			panels = append(panels, panelStyle.Render(w.Title+"\n\n"+m.Bar.View()))
		case "table":
			if t, ok := m.Tables[w.ID]; ok {
				panels = append(panels, panelStyle.Render(w.Title+"\n\n"+t.View()))
			} else {
				panels = append(panels, panelStyle.Render(w.Title+"\n\n(no table state found)"))
			}
		default:
			panels = append(panels, panelStyle.Render(fmt.Sprintf("%s\n(unknown widget type)", w.Title)))
		}
	}

	v := tea.NewView(header + "\n\n" + strings.Join(panels, "\n\n") + "\n")
	v.AltScreen = true
	return v
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
