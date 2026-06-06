package main

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"wtm/internal/git"
	"wtm/internal/tui"
)

func main() {
	var cdfile, selectPath string
	pickMode := false

	for _, arg := range os.Args[1:] {
		switch {
		case arg == "--pick":
			pickMode = true
		case strings.HasPrefix(arg, "--select="):
			selectPath = strings.TrimPrefix(arg, "--select=")
		case !strings.HasPrefix(arg, "-"):
			cdfile = arg
			pickMode = true
		}
	}

	entries, err := git.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error loading worktrees:", err)
		os.Exit(1)
	}

	if !pickMode {
		printTable(entries)
		return
	}

	m := tui.NewModel(entries, cdfile, selectPath)
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func printTable(entries []git.WorktreeEntry) {
	if len(entries) == 0 {
		fmt.Println("No worktrees found.")
		return
	}
	w := tui.ComputeColWidths(entries)
	header := tui.MakeHeader(w)
	fmt.Printf("\033[1m%s\033[0m\n", header)
	fmt.Println("  " + strings.Repeat("-", len(header)-2))
	for _, e := range entries {
		fmt.Println(tui.BuildLine(e, w, nil, false, 0, "", false))
	}
}
