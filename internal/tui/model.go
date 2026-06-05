package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"wtm/internal/actions"
	"wtm/internal/git"
)

type mode int

const (
	modeNormal mode = iota
	modeCreating
	modeActionResult
	modeConfirm
)

// Mensajes internos del TUI.
type prLoadedMsg struct {
	m       map[string]git.PRInfo
	repoURL string
}
type worktreesMsg struct{ entries []git.WorktreeEntry }
type refreshTickMsg struct{}
type refreshDoneMsg struct{}
type spinTickMsg struct{}

type confirmPending struct {
	path   string
	branch string
}

// Model es el modelo principal bubbletea.
type Model struct {
	worktrees []git.WorktreeEntry
	selected  int
	scroll    int
	width     int
	height    int

	prMap     map[string]git.PRInfo
	prLoading bool
	spinFrame int
	repoURL   string

	mode mode

	actionLog []string

	confirm confirmPending

	create *CreateModel

	cdfile     string
	root       string
	selectPath string
	colWidths  ColWidths
}

// NewModel crea el modelo inicial con los worktrees ya cargados.
func NewModel(entries []git.WorktreeEntry, cdfile, selectPath string) Model {
	root := ""
	for _, e := range entries {
		if e.IsMain {
			root = e.Path
			break
		}
	}

	m := Model{
		worktrees:  entries,
		prLoading:  true,
		cdfile:     cdfile,
		root:       root,
		selectPath: selectPath,
		prMap:      make(map[string]git.PRInfo),
		colWidths:  ComputeColWidths(entries),
		mode:       modeNormal,
	}

	if selectPath != "" {
		for i, e := range entries {
			if e.Path == selectPath {
				m.selected = i
				break
			}
		}
	}
	return m
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		loadPRsCmd(m.root),
		refreshTickCmd(),
		spinTickCmd(),
	)
}

func loadPRsCmd(root string) tea.Cmd {
	return func() tea.Msg {
		prMap := git.FetchPRMap(root)
		repoURL := git.RepoURL(root)
		if prMap == nil {
			prMap = make(map[string]git.PRInfo)
		}
		return prLoadedMsg{m: prMap, repoURL: repoURL}
	}
}

func refreshTickCmd() tea.Cmd {
	return tea.Tick(60*time.Second, func(t time.Time) tea.Msg {
		return refreshTickMsg{}
	})
}

func spinTickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return spinTickMsg{}
	})
}

func reloadWorktreesCmd() tea.Cmd {
	return func() tea.Msg {
		entries, err := git.Load()
		if err != nil || entries == nil {
			return worktreesMsg{entries: nil}
		}
		return worktreesMsg{entries: entries}
	}
}

func fetchAndReloadCmd(root string) tea.Cmd {
	return func() tea.Msg {
		exec.Command("git", "-C", root, "fetch", "--all", "--quiet").Run()
		return refreshDoneMsg{}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.scroll = clampScroll(m.selected, m.scroll, m.pageSize())
		if m.create != nil {
			m.create.width, m.create.height = msg.Width, msg.Height
		}
		return m, nil

	case spinTickMsg:
		if m.prLoading {
			m.spinFrame++
			return m, spinTickCmd()
		}
		return m, nil

	case prLoadedMsg:
		m.prMap = msg.m
		m.repoURL = msg.repoURL
		m.prLoading = false
		return m, nil

	case worktreesMsg:
		if msg.entries != nil {
			m.worktrees = msg.entries
			m.colWidths = ComputeColWidths(msg.entries)
			if m.selectPath != "" {
				for i, e := range m.worktrees {
					if e.Path == m.selectPath {
						m.selected = i
						break
					}
				}
				m.selectPath = ""
			}
			m.selected = clamp(m.selected, 0, len(m.worktrees)-1)
			m.scroll = clampScroll(m.selected, m.scroll, m.pageSize())
		}
		return m, nil

	case actions.ActionDoneMsg:
		if msg.CDPath != "" {
			if m.cdfile != "" {
				os.WriteFile(m.cdfile, []byte(msg.CDPath), 0644)
			}
			return m, tea.Quit
		}
		if len(msg.Log) > 0 {
			m.actionLog = msg.Log
			m.mode = modeActionResult
		}
		if msg.SelectPath != "" {
			m.selectPath = msg.SelectPath
		}
		return m, reloadWorktreesCmd()

	case createDoneMsg:
		m.mode = modeNormal
		m.create = nil
		if msg.path != "" {
			m.selectPath = msg.path
		}
		return m, reloadWorktreesCmd()

	case createCancelledMsg:
		m.mode = modeNormal
		m.create = nil
		return m, nil

	case refreshTickMsg:
		return m, tea.Batch(fetchAndReloadCmd(m.root), refreshTickCmd())

	case refreshDoneMsg:
		return m, reloadWorktreesCmd()

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	// Delegar al sub-modelo create si está activo.
	if m.mode == modeCreating && m.create != nil {
		newCreate, cmd := m.create.Update(msg)
		m.create = newCreate
		return m, cmd
	}

	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Overlay de resultado: cualquier tecla lo cierra.
	if m.mode == modeActionResult {
		m.mode = modeNormal
		m.actionLog = nil
		return m, nil
	}

	// Modo confirmación de borrado.
	if m.mode == modeConfirm {
		switch msg.String() {
		case "y", "Y", "enter":
			m.mode = modeNormal
			return m, actions.DeleteCmd(m.confirm.path, false, m.confirm.branch)
		default:
			m.mode = modeNormal
			return m, nil
		}
	}

	// Delegar al sub-modelo create.
	if m.mode == modeCreating && m.create != nil {
		newCreate, cmd := m.create.Update(msg)
		m.create = newCreate
		return m, cmd
	}

	n := len(m.worktrees)
	wt := func() git.WorktreeEntry {
		if n == 0 {
			return git.WorktreeEntry{}
		}
		return m.worktrees[m.selected]
	}

	switch msg.String() {
	case "q", "ctrl+c", "esc":
		return m, tea.Quit

	case "enter":
		e := wt()
		return m, actions.CDCmd(e.Path, m.cdfile)

	case "j", "down":
		if m.selected < n-1 {
			m.selected++
			m.scroll = clampScroll(m.selected, m.scroll, m.pageSize())
		}

	case "k", "up":
		if m.selected > 0 {
			m.selected--
			m.scroll = clampScroll(m.selected, m.scroll, m.pageSize())
		}

	case "g":
		m.selected = 0
		m.scroll = 0

	case "G":
		m.selected = max(0, n-1)
		m.scroll = clampScroll(m.selected, m.scroll, m.pageSize())

	case "l":
		e := wt()
		if _, err := exec.LookPath("lazygit"); err == nil {
			cmd := exec.Command("lazygit")
			cmd.Dir = e.Path
			return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
				return actions.ActionDoneMsg{SelectPath: e.Path}
			})
		}

	case "p":
		e := wt()
		if canPull(e) {
			return m, actions.PullCmd(e.Path)
		}

	case "P":
		e := wt()
		if canPush(e) {
			return m, actions.PushCmd(e.Path, e.Branch)
		}

	case "r":
		e := wt()
		if canRebaseOrMerge(e) {
			return m, actions.RebaseCmd(e.Path, git.GetMainBranch(m.root))
		}

	case "u":
		e := wt()
		if canRebaseOrMerge(e) {
			return m, actions.MergeCmd(e.Path, git.GetMainBranch(m.root))
		}

	case "f":
		return m, actions.FetchCmd(m.root)

	case "d":
		e := wt()
		if canDelete(e) {
			if isOrphan(e) {
				// orphan: borrar directamente sin confirmación
				return m, actions.DeleteCmd(e.Path, true, e.Branch)
			}
			// regular: pedir confirmación
			m.mode = modeConfirm
			m.confirm = confirmPending{path: e.Path, branch: e.Branch}
		}

	case "D":
		return m, actions.PruneAllCmd(m.root)

	case "C":
		m.mode = modeCreating
		m.create = NewCreateModel(m.worktrees, m.root, m.width, m.height)
		return m, m.create.Init()
	}

	return m, nil
}

func (m Model) View() string {
	switch m.mode {
	case modeActionResult:
		return m.viewActionOverlay()
	case modeConfirm:
		return m.viewConfirm()
	case modeCreating:
		if m.create != nil {
			return m.create.View()
		}
	}
	return m.viewNormal()
}

func (m Model) viewNormal() string {
	header := MakeHeader(m.colWidths)
	var sb strings.Builder
	sb.WriteString(styleBold.Render(header) + "\n")
	sb.WriteString("  " + strings.Repeat("-", max(0, len(header)-2)) + "\n")

	page := m.pageSize()
	end := min(m.scroll+page, len(m.worktrees))
	for i, e := range m.worktrees[m.scroll:end] {
		idx := m.scroll + i
		line := BuildLine(e, m.colWidths, m.prMap, m.prLoading, m.spinFrame, m.repoURL, idx == m.selected)
		sb.WriteString("  " + line + "\n")
	}

	if len(m.worktrees) > 0 {
		e := m.worktrees[m.selected]
		sb.WriteString("\n" + DetailLine(e) + "\n")
		sb.WriteString("\n" + m.viewHints(e) + "\n")

		absPath := e.Path
		relPath, _ := filepath.Rel(m.root, absPath)
		displayPath := absPath
		if relPath != "." && relPath != "" {
			displayPath = relPath
		}
		sb.WriteString(styleDim.Render("  "+displayPath) + "\n")
	}

	return sb.String()
}

func (m Model) viewHints(e git.WorktreeEntry) string {
	h := func(key, label string) string {
		return styleBold.Render(key) + styleDim.Render(": "+label)
	}

	hints := []string{h("C", "create"), h("f", "fetch"), h("D", "prune all")}
	if _, err := exec.LookPath("lazygit"); err == nil {
		hints = append(hints[:2], append([]string{h("l", "lazygit")}, hints[2:]...)...)
	}
	if canPull(e) {
		hints = append(hints, h("p", fmt.Sprintf("pull %s", styleBlue.Render(fmt.Sprintf("↓%d", e.Remote.Behind)))))
	}
	if canPush(e) {
		if e.Remote != nil && e.Remote.Ahead > 0 {
			hints = append(hints, h("P", fmt.Sprintf("push %s", styleGreen.Render(fmt.Sprintf("↑%d", e.Remote.Ahead)))))
		} else {
			hints = append(hints, h("P", "push"))
		}
	}
	if canRebaseOrMerge(e) {
		hints = append(hints, h("r", "rebase"), h("u", "merge"))
	}
	if canDelete(e) {
		hints = append(hints, h("d", "delete"))
	}
	hints = append(hints, h("q", "quit"))

	sep := styleDim.Render(" | ")
	return styleDim.Render("  ") + strings.Join(hints, sep)
}

func (m Model) viewActionOverlay() string {
	var sb strings.Builder
	sb.WriteString("\n")
	for _, line := range m.actionLog {
		sb.WriteString("  " + line + "\n")
	}
	sb.WriteString("\n" + styleDim.Render("  Press any key to continue...") + "\n")
	return sb.String()
}

func (m Model) viewConfirm() string {
	e := m.worktrees[m.selected]
	name := filepath.Base(e.Path)
	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("  Delete worktree %s?\n\n", styleCyan.Render(name)))
	sb.WriteString("  " + styleBold.Render("Y") + styleDim.Render("/Enter = confirm  ") +
		styleBold.Render("n") + styleDim.Render(" = cancel") + "\n")
	return sb.String()
}

func (m Model) pageSize() int {
	ps := m.height - 9
	if ps < 1 {
		return 1
	}
	return ps
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func clampScroll(selected, scroll, page int) int {
	if selected < scroll {
		return selected
	}
	if selected >= scroll+page {
		return selected - page + 1
	}
	return scroll
}

// Predicados de acción (equivalente a predicates.py).

func canPull(e git.WorktreeEntry) bool {
	return e.Remote != nil && e.Remote.Behind > 0
}

func canPush(e git.WorktreeEntry) bool {
	return e.Remote == nil || e.Remote.Ahead > 0
}

func canRebaseOrMerge(e git.WorktreeEntry) bool {
	return e.Main.Behind > 0
}

func canDelete(e git.WorktreeEntry) bool {
	return e.MainState != git.MainStateIsMain
}

func isOrphan(e git.WorktreeEntry) bool {
	return e.MainState == git.MainStateOrphan
}
