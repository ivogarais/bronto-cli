package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/NimbleMarkets/ntcharts/v2/barchart"

	"github.com/ivogarais/bronto-cli/datasource"
	"github.com/ivogarais/bronto-cli/spec"
)

type tickMsg time.Time

type queryResultMsg struct {
	widgetID string
	kind     datasource.Kind
	bar      *datasource.BarResult
	table    *datasource.TableResult
	err      error
}

type Model struct {
	Spec     *spec.DashboardSpec
	SpecPath string

	DS datasource.DataSource

	Bar    barchart.Model
	Tables map[string]table.Model

	Refresh time.Duration
	Now     time.Time

	LastUpdated time.Time
	Status      string

	Width  int
	Height int
}

func NewModel(s *spec.DashboardSpec, specPath string, ds datasource.DataSource) Model {
	now := time.Now()
	if ds == nil {
		ds = datasource.NewFake()
	}

	// Default size; resized on WindowSizeMsg.
	bar := barchart.New(50, 12)

	tables := make(map[string]table.Model)
	for _, w := range s.Widgets {
		if w.Type != "table" {
			continue
		}

		cols := make([]table.Column, 0, len(w.Columns))
		for _, c := range w.Columns {
			cols = append(cols, table.Column{Title: c, Width: max(10, len(c)+2)})
		}

		t := table.New(
			table.WithColumns(cols),
			table.WithRows([]table.Row{}),
			table.WithFocused(true),
			table.WithWidth(50),
			table.WithHeight(10),
		)
		tables[w.ID] = t
	}

	refresh := time.Second
	if s.Refresh.EveryMs > 0 {
		refresh = time.Duration(s.Refresh.EveryMs) * time.Millisecond
	}

	return Model{
		Spec:     s,
		SpecPath: specPath,
		DS:       ds,
		Bar:      bar,
		Tables:   tables,
		Refresh:  refresh,
		Now:      now,
		Status:   "Running (live updates; fake datasource)",
	}
}

func (m Model) Init() tea.Cmd {
	// Immediately fetch once, then start ticking.
	return tea.Batch(m.fetchAllWidgets(), m.tickCmd())
}

func (m Model) tickCmd() tea.Cmd {
	return tea.Tick(m.Refresh, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m *Model) setBarData(labels []string, values []float64) {
	data := make([]barchart.BarData, 0, len(labels))
	for i, l := range labels {
		v := 0.0
		if i < len(values) {
			v = values[i]
		}
		data = append(data, barchart.BarData{
			Label: l,
			Values: []barchart.BarValue{
				{Name: l, Value: v},
			},
		})
	}
	m.Bar.Clear()
	m.Bar.PushAll(data)
	m.Bar.Draw()
}

func (m Model) fetchAllWidgets() tea.Cmd {
	cmds := make([]tea.Cmd, 0, len(m.Spec.Widgets))
	for _, w := range m.Spec.Widgets {
		w := w
		q, ok := m.Spec.Queries[w.QueryRef]
		if !ok {
			continue
		}

		var kind datasource.Kind
		switch q.Kind {
		case "bar":
			kind = datasource.KindBar
		case "table":
			kind = datasource.KindTable
		default:
			kind = datasource.Kind(q.Kind)
		}

		cmds = append(cmds, func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			res, err := m.DS.RunQuery(ctx, datasource.Query{
				ID:   w.QueryRef,
				Kind: kind,
				Tool: q.Tool,
				Args: q.Args,
			})
			if err != nil {
				return queryResultMsg{widgetID: w.ID, kind: kind, err: err}
			}

			msg := queryResultMsg{widgetID: w.ID, kind: kind}
			if res.Kind != "" {
				msg.kind = res.Kind
			}
			if res.Bar != nil {
				msg.bar = res.Bar
			}
			if res.Table != nil {
				msg.table = res.Table
			}
			return msg
		})
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		}
		// Let tables handle navigation keys
		for id, t := range m.Tables {
			var cmd tea.Cmd
			t, cmd = t.Update(msg)
			m.Tables[id] = t
			if cmd != nil {
				return m, cmd
			}
		}
		return m, nil

	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height

		// Resize chart
		chartW := msg.Width - 8
		if chartW < 20 {
			chartW = 20
		}
		chartH := 12
		m.Bar.Resize(chartW, chartH)
		m.Bar.Draw()

		// Resize table columns
		for id, t := range m.Tables {
			cols := t.Columns()
			if len(cols) > 0 {
				available := msg.Width - 14
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
			}
			t.SetHeight(10)
			m.Tables[id] = t
		}
		return m, nil

	case tickMsg:
		m.Now = time.Time(msg)
		// Fetch new data + schedule next tick
		return m, tea.Batch(m.fetchAllWidgets(), m.tickCmd())

	case queryResultMsg:
		if msg.err != nil {
			m.Status = fmt.Sprintf("Last update error for %s: %v", msg.widgetID, msg.err)
			return m, nil
		}

		// Update widgets based on result kind
		switch msg.kind {
		case datasource.KindBar:
			if msg.bar != nil {
				m.setBarData(msg.bar.Labels, msg.bar.Values)
				m.LastUpdated = time.Now()
				m.Status = "Updated"
			}
		case datasource.KindTable:
			if msg.table != nil {
				t, ok := m.Tables[msg.widgetID]
				if !ok {
					return m, nil
				}

				// If columns differ, update them
				if len(msg.table.Columns) > 0 {
					cols := make([]table.Column, 0, len(msg.table.Columns))
					for _, c := range msg.table.Columns {
						cols = append(cols, table.Column{Title: c, Width: max(10, len(c)+2)})
					}
					t.SetColumns(cols)
				}

				// Prepend new rows (simulate live logs)
				existing := t.Rows()
				newRows := make([]table.Row, 0, len(msg.table.Rows)+len(existing))
				for _, r := range msg.table.Rows {
					row := append(table.Row(nil), r...)
					newRows = append(newRows, row)
				}
				newRows = append(newRows, existing...)
				// Cap size so it doesn't grow forever
				if len(newRows) > 200 {
					newRows = newRows[:200]
				}
				t.SetRows(newRows)
				m.Tables[msg.widgetID] = t

				m.LastUpdated = time.Now()
				m.Status = "Updated"
			}
		}
		return m, nil
	}

	return m, nil
}

func (m Model) View() tea.View {
	titleStyle := lipgloss.NewStyle().Bold(true).Padding(0, 1)
	headerBox := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
	panelStyle := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1, 2)

	last := "never"
	if !m.LastUpdated.IsZero() {
		last = m.LastUpdated.Format("15:04:05")
	}

	title := "Dashboard"
	if m.Spec != nil && m.Spec.Title != "" {
		title = m.Spec.Title
	}

	header := headerBox.Render(
		fmt.Sprintf("%s\nSpec: %s   Refresh: %s   Last: %s\nStatus: %s\n(press q to quit)",
			titleStyle.Render(title),
			m.SpecPath,
			m.Refresh,
			last,
			m.Status,
		),
	)

	var panels []string
	if m.Spec != nil {
		for _, w := range m.Spec.Widgets {
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
				panels = append(panels, panelStyle.Render(w.Title+"\n\n(unsupported widget type)"))
			}
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
