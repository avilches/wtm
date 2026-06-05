package tui

import "github.com/charmbracelet/lipgloss"

var (
	styleBold      = lipgloss.NewStyle().Bold(true)
	styleDim       = lipgloss.NewStyle().Faint(true)
	styleGreen     = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	styleYellow    = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	styleRed       = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	styleCyan      = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	styleBlue      = lipgloss.NewStyle().Foreground(lipgloss.Color("4"))
	styleWhite     = lipgloss.NewStyle().Foreground(lipgloss.Color("15"))
	stylePROpen    = lipgloss.NewStyle().Background(lipgloss.Color("28")).Foreground(lipgloss.Color("15"))
	stylePRMerged  = lipgloss.NewStyle().Background(lipgloss.Color("93")).Foreground(lipgloss.Color("15"))
	stylePRClosed  = lipgloss.NewStyle().Background(lipgloss.Color("160")).Foreground(lipgloss.Color("15"))
	stylePRDraft   = lipgloss.NewStyle().Background(lipgloss.Color("240")).Foreground(lipgloss.Color("15"))
	styleUnderline = lipgloss.NewStyle().Underline(true)
	styleBGSel     = lipgloss.NewStyle().Background(lipgloss.Color("237"))
)
