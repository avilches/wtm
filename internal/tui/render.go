package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"wtm/internal/git"
)

var spinnerFrames = []rune("⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏")

// ColWidths almacena el ancho de cada columna calculado dinámicamente.
type ColWidths struct {
	Branch  int
	Changes int
	State   int
	Remote  int
	Age     int
}

// ComputeColWidths calcula el ancho mínimo de cada columna para todos los entries.
func ComputeColWidths(entries []git.WorktreeEntry) ColWidths {
	bw := len("Branch")
	fw := len("Changes")
	sw := len("State")
	rw := len("Remote")
	aw := len("Age")
	now := time.Now().Unix()
	for _, e := range entries {
		b := e.Branch
		if b == "" {
			b = "(detached)"
		}
		if len(b) > bw {
			bw = len(b)
		}
		if v := lipgloss.Width(stateStr(e)); v > sw {
			sw = v
		}
		if v := lipgloss.Width(remoteSyncStr(e)); v > rw {
			rw = v
		}
		if v := len(ageStr(e.Commit.Timestamp, now)); v > aw {
			aw = v
		}
	}
	return ColWidths{Branch: bw, Changes: fw, State: sw, Remote: rw, Age: aw}
}

func ageStr(ts, now int64) string {
	d := now - ts
	switch {
	case d < 3600:
		return fmt.Sprintf("%dm", d/60)
	case d < 86400:
		return fmt.Sprintf("%dh", d/3600)
	default:
		return fmt.Sprintf("%dd", d/86400)
	}
}

func changesStr(e git.WorktreeEntry) string {
	var b strings.Builder
	w := e.WorkingTree
	if w.Staged {
		b.WriteByte('S')
	}
	if w.Modified {
		b.WriteByte('M')
	}
	if w.Untracked {
		b.WriteByte('?')
	}
	if w.Deleted {
		b.WriteByte('D')
	}
	if b.Len() == 0 {
		return "-"
	}
	return b.String()
}

func stateStr(e git.WorktreeEntry) string {
	m := e.Main
	switch e.MainState {
	case git.MainStateIsMain:
		return styleDim.Render("main")
	case git.MainStateIntegrated:
		return styleDim.Render("MERGED")
	case git.MainStateOrphan:
		return styleDim.Render("ORPHAN")
	case git.MainStateSameCommit:
		return styleDim.Render("=")
	case git.MainStateEmpty:
		return styleDim.Render("empty")
	}
	var parts []string
	if m.Ahead > 0 {
		parts = append(parts, styleGreen.Render(fmt.Sprintf("←%d", m.Ahead)))
	}
	if m.Behind > 0 {
		parts = append(parts, styleBlue.Render(fmt.Sprintf("→%d", m.Behind)))
	}
	if len(parts) == 0 {
		return styleDim.Render("=")
	}
	return strings.Join(parts, " ")
}

func remoteSyncStr(e git.WorktreeEntry) string {
	r := e.Remote
	if r == nil {
		return styleDim.Render("no-remote")
	}
	if r.Ahead == 0 && r.Behind == 0 {
		return styleDim.Render("up to date")
	}
	var parts []string
	if r.Behind > 0 {
		parts = append(parts, styleBlue.Render(fmt.Sprintf("↓%d", r.Behind)))
	}
	if r.Ahead > 0 {
		parts = append(parts, styleGreen.Render(fmt.Sprintf("↑%d", r.Ahead)))
	}
	return strings.Join(parts, " ")
}

func prPartStr(e git.WorktreeEntry, prMap map[string]git.PRInfo, prLoading bool, spinFrame int, repoURL string) string {
	const prWidth = 10
	if prLoading {
		ch := string(spinnerFrames[spinFrame%len(spinnerFrames)])
		return styleDim.Render(fmt.Sprintf("%-*s", prWidth, ch))
	}
	pr, ok := prMap[e.Branch]
	if !ok {
		return strings.Repeat(" ", prWidth)
	}
	var badge string
	switch {
	case pr.Draft:
		badge = stylePRDraft.Render(" D ")
	case pr.State == "OPEN":
		badge = stylePROpen.Render(" O ")
	case pr.State == "MERGED":
		badge = stylePRMerged.Render(" M ")
	default:
		badge = stylePRClosed.Render(" C ")
	}
	prText := fmt.Sprintf("#%d", pr.Number)
	var link string
	if repoURL != "" {
		url := fmt.Sprintf("%s/pull/%d", repoURL, pr.Number)
		rendered := styleBlue.Inherit(styleUnderline).Render(prText)
		link = fmt.Sprintf("\033]8;;%s\033\\%s\033]8;;\033\\", url, rendered)
	} else {
		link = styleBlue.Inherit(styleUnderline).Render(prText)
	}
	pad := prWidth - 3 - 1 - len(prText)
	if pad < 0 {
		pad = 0
	}
	return badge + " " + link + strings.Repeat(" ", pad)
}

// MakeHeader devuelve la línea de cabecera con los nombres de columna.
func MakeHeader(w ColWidths) string {
	return fmt.Sprintf("    %-*s  %-*s  %-*s  %-*s  %-10s  %-*s  Commit",
		w.Branch, "Branch",
		w.Changes, "Changes",
		w.State, "State",
		w.Remote, "Remote",
		"PR",
		w.Age, "Age",
	)
}

// BuildLine construye la línea renderizada de un worktree.
func BuildLine(e git.WorktreeEntry, w ColWidths, prMap map[string]git.PRInfo, prLoading bool, spinFrame int, repoURL string, selected bool) string {
	now := time.Now().Unix()

	marker := " "
	if e.IsCurrent {
		marker = "@"
	}

	branch := e.Branch
	if branch == "" {
		branch = "(detached)"
	}

	var branchStyled string
	switch {
	case e.MainState == git.MainStateIntegrated || e.MainState == git.MainStateOrphan:
		branchStyled = styleDim.Render(fmt.Sprintf("%-*s", w.Branch, branch))
	case e.IsCurrent:
		branchStyled = styleBold.Foreground(lipgloss.Color("2")).Render(fmt.Sprintf("%-*s", w.Branch, branch))
	default:
		branchStyled = fmt.Sprintf("%-*s", w.Branch, branch)
	}

	changes := changesStr(e)
	var changesStyled string
	if changes != "-" {
		changesStyled = styleRed.Render(fmt.Sprintf("%-*s", w.Changes, changes))
	} else {
		changesStyled = styleDim.Render(fmt.Sprintf("%-*s", w.Changes, changes))
	}

	state := stateStr(e)
	statePadded := state + strings.Repeat(" ", max(0, w.State-lipgloss.Width(state)))

	remote := remoteSyncStr(e)
	remotePadded := remote + strings.Repeat(" ", max(0, w.Remote-lipgloss.Width(remote)))

	pr := prPartStr(e, prMap, prLoading, spinFrame, repoURL)

	a := ageStr(e.Commit.Timestamp, now)
	ageStyled := styleDim.Render(fmt.Sprintf("%-*s", w.Age, a))

	msg := e.Commit.Message
	if len([]rune(msg)) > 45 {
		msg = string([]rune(msg)[:45])
	}
	msgStyled := styleDim.Render(msg)

	line := fmt.Sprintf("%s %s  %s  %s  %s  %s  %s  %s",
		marker, branchStyled, changesStyled, statePadded, remotePadded, pr, ageStyled, msgStyled)

	if selected {
		line = "\033[48;5;237m" + line + "\033[0m"
	}
	return line
}

// DetailLine devuelve la línea de detalle del working tree del worktree seleccionado.
func DetailLine(e git.WorktreeEntry) string {
	w := e.WorkingTree
	var legend []string
	if w.Staged {
		legend = append(legend, "S=staged")
	}
	if w.Modified {
		legend = append(legend, "M=modified")
	}
	if w.Untracked {
		legend = append(legend, "?=untracked")
	}
	if w.Deleted {
		legend = append(legend, "D=deleted")
	}
	if len(legend) == 0 {
		return "  " + styleDim.Render("clean")
	}
	return "  " + styleDim.Render(strings.Join(legend, "  "))
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
