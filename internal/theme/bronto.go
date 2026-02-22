package theme

import "charm.land/lipgloss/v2"

const (
	BrontoBlack      = "#0B0B0B"
	BrontoPanelBg    = "#121212"
	BrontoElevatedBg = "#1A1A1A"
	BrontoBorder     = "#2A2A2A"
	BrontoPrimary    = "#E6A400"
	BrontoSoft       = "#F2C94C"
	BrontoDanger     = "#FF5C5C"
	BrontoText       = "#EAEAEA"
	BrontoMuted      = "#7A7A7A"
)

type BrontoTheme struct {
	Density string

	AppBg       lipgloss.Style
	HeaderBox   lipgloss.Style
	Panel       lipgloss.Style
	PanelTitle  lipgloss.Style
	PanelAccent lipgloss.Style
	AppTitle    lipgloss.Style

	Text    lipgloss.Style
	Muted   lipgloss.Style
	Primary lipgloss.Style
	Danger  lipgloss.Style
	Divider lipgloss.Style

	TableHeader   lipgloss.Style
	TableCell     lipgloss.Style
	TableSelected lipgloss.Style

	ChartAxis   lipgloss.Style
	ChartLabel  lipgloss.Style
	ChartBar    lipgloss.Style
	ChartDanger lipgloss.Style
}

func NewBrontoTheme(density string) BrontoTheme {
	if density != "compact" {
		density = "comfortable"
	}

	padding := 1
	hPadding := 1
	if density == "compact" {
		padding = 0
		hPadding = 0
	}

	baseText := lipgloss.NewStyle().Foreground(lipgloss.Color(BrontoText))
	baseMuted := lipgloss.NewStyle().Foreground(lipgloss.Color(BrontoMuted))
	basePrimary := lipgloss.NewStyle().Foreground(lipgloss.Color(BrontoPrimary)).Bold(true)

	panel := lipgloss.NewStyle().
		Background(lipgloss.Color(BrontoPanelBg)).
		Foreground(lipgloss.Color(BrontoText)).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(BrontoBorder)).
		Padding(padding, hPadding)

	header := panel.Copy().Foreground(lipgloss.Color(BrontoSoft))

	return BrontoTheme{
		Density: density,

		AppBg: lipgloss.NewStyle().
			Background(lipgloss.Color(BrontoBlack)).
			Foreground(lipgloss.Color(BrontoText)),

		HeaderBox:  header,
		Panel:      panel,
		PanelTitle: basePrimary,
		PanelAccent: lipgloss.NewStyle().
			Foreground(lipgloss.Color(BrontoPrimary)).
			Bold(true),
		AppTitle: lipgloss.NewStyle().
			Foreground(lipgloss.Color(BrontoSoft)).
			Bold(true),

		Text:    baseText,
		Muted:   baseMuted,
		Primary: basePrimary,
		Danger: lipgloss.NewStyle().
			Foreground(lipgloss.Color(BrontoDanger)).
			Bold(true),
		Divider: lipgloss.NewStyle().
			Foreground(lipgloss.Color(BrontoBorder)),

		TableHeader: lipgloss.NewStyle().
			Foreground(lipgloss.Color(BrontoPrimary)).
			Bold(true),
		TableCell: baseText,
		TableSelected: lipgloss.NewStyle().
			Foreground(lipgloss.Color(BrontoText)).
			Background(lipgloss.Color(BrontoElevatedBg)).
			Bold(true),

		ChartAxis:  baseMuted,
		ChartLabel: baseText,
		ChartBar: lipgloss.NewStyle().
			Foreground(lipgloss.Color(BrontoPrimary)),
		ChartDanger: lipgloss.NewStyle().
			Foreground(lipgloss.Color(BrontoDanger)),
	}
}
