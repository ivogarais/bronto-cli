package theme

import "charm.land/lipgloss/v2"

const (
	BrontoBlack       = "#0B0B0B"
	BrontoPanelBg     = "#111111"
	BrontoBorder      = "#2A2A2A"
	BrontoPrimary     = "#E6A400"
	BrontoPrimarySoft = "#F2C94C"
	BrontoDanger      = "#FF5C5C"
	BrontoText        = "#EAEAEA"
	BrontoMuted       = "#888888"
	BrontoSelectedBg  = "#1C1C1C"
)

type BrontoTheme struct {
	Density string

	AppBg      lipgloss.Style
	HeaderBox  lipgloss.Style
	Panel      lipgloss.Style
	PanelTitle lipgloss.Style

	Text    lipgloss.Style
	Muted   lipgloss.Style
	Primary lipgloss.Style
	Danger  lipgloss.Style

	TableHeader   lipgloss.Style
	TableCell     lipgloss.Style
	TableSelected lipgloss.Style

	ChartAxis  lipgloss.Style
	ChartLabel lipgloss.Style
	ChartBar   lipgloss.Style
}

func NewBrontoTheme(density string) BrontoTheme {
	if density != "compact" {
		density = "comfortable"
	}

	padding := 1
	hPadding := 2
	if density == "compact" {
		padding = 0
		hPadding = 1
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

	header := panel.Copy().Foreground(lipgloss.Color(BrontoPrimarySoft))

	return BrontoTheme{
		Density: density,

		AppBg: lipgloss.NewStyle().
			Background(lipgloss.Color(BrontoBlack)).
			Foreground(lipgloss.Color(BrontoText)),

		HeaderBox:  header,
		Panel:      panel,
		PanelTitle: basePrimary,

		Text:    baseText,
		Muted:   baseMuted,
		Primary: basePrimary,
		Danger: lipgloss.NewStyle().
			Foreground(lipgloss.Color(BrontoDanger)).
			Bold(true),

		TableHeader: lipgloss.NewStyle().
			Foreground(lipgloss.Color(BrontoPrimary)).
			Bold(true),
		TableCell: baseText,
		TableSelected: lipgloss.NewStyle().
			Foreground(lipgloss.Color(BrontoText)).
			Background(lipgloss.Color(BrontoSelectedBg)).
			Bold(true),

		ChartAxis:  baseMuted,
		ChartLabel: baseText,
		ChartBar: lipgloss.NewStyle().
			Foreground(lipgloss.Color(BrontoPrimary)),
	}
}
