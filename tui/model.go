package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"github.com/NimbleMarkets/ntcharts/v2/barchart"
	"github.com/NimbleMarkets/ntcharts/v2/linechart"

	brontotheme "github.com/ivogarais/bronto-cli/internal/theme"
	"github.com/ivogarais/bronto-cli/spec"
)

type Model struct {
	Spec     *spec.AppSpec
	SpecPath string
	Theme    brontotheme.BrontoTheme

	HasChartsTab bool
	HasLogsTab   bool
	ActiveTab    string

	chartsLayout spec.Node
	logsLayout   spec.Node

	Charts map[string]barchart.Model
	Lines  map[string]linechart.Model
	Tables map[string]table.Model

	FocusablePanels []focusPanel
	PanelNumberByID map[string]int
	FocusedPanel    int
	FocusScrollY    int

	TableBaseRows   map[string][]table.Row
	TableFilter     map[string]string
	TableFilterMode bool

	Width    int
	Height   int
	ContentH int
	ScrollY  int
	Status   string
	LoadedAt time.Time
}

func NewModel(s *spec.AppSpec, specPath string) Model {
	density := "comfortable"
	if s != nil {
		density = s.Theme.Density
	}

	m := Model{
		Spec:            s,
		SpecPath:        specPath,
		Theme:           brontotheme.NewBrontoTheme(density),
		Charts:          map[string]barchart.Model{},
		Lines:           map[string]linechart.Model{},
		Tables:          map[string]table.Model{},
		PanelNumberByID: map[string]int{},
		TableBaseRows:   map[string][]table.Row{},
		TableFilter:     map[string]string{},
		Status:          "Snapshot loaded",
		LoadedAt:        time.Now(),
	}

	m.applyDefaultLayoutStructure()
	m.resolveComponents()
	m.indexFocusablePanels()
	m.resizeForLayout(120, 36)
	return m
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.TableFilterMode {
			if tableRef, ok := m.focusedTableRef(); ok {
				if m.handleTableFilterInput(tableRef, msg) {
					return m, nil
				}
			} else {
				m.TableFilterMode = false
			}
		}

		if m.hasTabs() {
			switch msg.String() {
			case "c":
				if m.switchTab(tabCharts) {
					return m, nil
				}
			case "l":
				if m.switchTab(tabLogs) {
					return m, nil
				}
			}
		}

		if number, ok := parseFocusNumberKey(msg); ok {
			if _, exists := m.focusPanelByNumber(number); exists {
				m.FocusedPanel = number
				m.ScrollY = 0
				m.FocusScrollY = 0
				m.TableFilterMode = false
				m.resizeForLayout(m.Width, m.Height)
			}
			return m, nil
		}

		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "esc", "0":
			if m.FocusedPanel > 0 {
				m.FocusedPanel = 0
				m.FocusScrollY = 0
				m.TableFilterMode = false
				m.resizeForLayout(m.Width, m.Height)
				return m, nil
			}
			if msg.String() == "esc" {
				return m, tea.Quit
			}
		case "tab":
			if len(m.FocusablePanels) > 0 {
				next := m.FocusedPanel + 1
				if next > len(m.FocusablePanels) || next <= 0 {
					next = 1
				}
				m.FocusedPanel = next
				m.FocusScrollY = 0
				m.TableFilterMode = false
				m.resizeForLayout(m.Width, m.Height)
			}
			return m, nil
		case "shift+tab":
			if len(m.FocusablePanels) > 0 {
				prev := m.FocusedPanel - 1
				if prev <= 0 || prev > len(m.FocusablePanels) {
					prev = len(m.FocusablePanels)
				}
				m.FocusedPanel = prev
				m.FocusScrollY = 0
				m.TableFilterMode = false
				m.resizeForLayout(m.Width, m.Height)
			}
			return m, nil
		}

		if tableRef, ok := m.focusedTableRef(); ok {
			switch msg.String() {
			case "/":
				m.TableFilterMode = true
				if _, exists := m.TableFilter[tableRef]; !exists {
					m.TableFilter[tableRef] = ""
				}
				return m, nil
			case "x":
				if m.TableFilter[tableRef] != "" {
					m.TableFilter[tableRef] = ""
					m.applyTableFilter(tableRef)
				}
				return m, nil
			}

			t, exists := m.Tables[tableRef]
			if !exists {
				return m, nil
			}
			var cmd tea.Cmd
			t, cmd = t.Update(msg)
			m.Tables[tableRef] = t
			return m, cmd
		}

		if m.FocusedPanel > 0 {
			switch msg.String() {
			case "down", "j", "ctrl+n":
				m.focusScrollBy(1)
				return m, nil
			case "up", "k", "ctrl+p":
				m.focusScrollBy(-1)
				return m, nil
			case "pgdown", "ctrl+f", "ctrl+d":
				m.focusScrollBy(maxInt(1, m.Height-4))
				return m, nil
			case "pgup", "ctrl+b", "ctrl+u":
				m.focusScrollBy(-maxInt(1, m.Height-4))
				return m, nil
			case "home", "g":
				m.FocusScrollY = 0
				return m, nil
			case "end", "G":
				m.FocusScrollY = m.focusMaxScroll()
				return m, nil
			}
			return m, nil
		}

		switch msg.String() {
		case "down", "j", "ctrl+n":
			m.scrollBy(1)
			return m, nil
		case "up", "k", "ctrl+p":
			m.scrollBy(-1)
			return m, nil
		case "pgdown", "ctrl+f", "ctrl+d":
			m.scrollBy(maxInt(1, m.Height-4))
			return m, nil
		case "pgup", "ctrl+b", "ctrl+u":
			m.scrollBy(-maxInt(1, m.Height-4))
			return m, nil
		case "home", "g":
			m.ScrollY = 0
			return m, nil
		case "end", "G":
			m.ScrollY = m.maxScroll()
			return m, nil
		}

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
		m.resizeForLayout(msg.Width, msg.Height)
		return m, nil
	}

	return m, nil
}

func (m Model) View() tea.View {
	if m.Spec == nil {
		v := tea.NewView("No spec loaded.\n")
		v.AltScreen = true
		return v
	}

	width := m.Width
	height := m.Height
	if width <= 0 {
		width = 120
	}
	if height <= 0 {
		height = 36
	}

	body := ""
	if panel, ok := m.focusPanelByNumber(m.FocusedPanel); ok {
		hints := []string{
			m.Theme.Muted.Render("(focus mode: 0/esc exit | tab next | shift+tab prev | q quit)"),
		}
		if panel.Node.Type == "table" {
			tableRef := panel.Node.TableRef
			query := m.TableFilter[tableRef]
			switch {
			case m.TableFilterMode:
				hints = append(hints, m.Theme.Primary.Render(fmt.Sprintf("filter> %s_", query)))
			case query != "":
				hints = append(hints, m.Theme.Muted.Render(fmt.Sprintf("filter: %q ( / edit | x clear )", query)))
			default:
				hints = append(hints, m.Theme.Muted.Render("table controls: arrows/pgup/pgdn | / filter | x clear"))
			}
		} else {
			hints = append(hints, m.Theme.Muted.Render("scroll focused panel: up/down | pgup/pgdn | home/end"))
		}

		hintBlock := strings.Join(hints, "\n")
		viewH := maxInt(1, height-len(hints))

		renderH := viewH
		if panel.Node.Type != "table" {
			renderH = m.focusRenderHeight(panel.Node, width, viewH)
		}

		panelBody := m.renderNode(panel.Node, width, renderH)
		if panel.Node.Type != "table" {
			panelBody = clampViewport(panelBody, m.FocusScrollY, viewH)
		}
		body = hintBlock + "\n" + panelBody
	} else {
		contentH := m.ContentH
		if contentH <= 0 {
			contentH = maxInt(height, m.preferredNodeHeight(m.Spec.Layout, width))
		}
		body = m.renderNode(m.Spec.Layout, width, contentH)
		body = clampViewport(body, m.ScrollY, height)
	}
	body = m.Theme.AppBg.Copy().Render(body)
	v := tea.NewView(body + "\n")
	v.AltScreen = true
	return v
}
