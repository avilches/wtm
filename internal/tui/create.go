package tui

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"wtm/internal/git"
)

type createStep int

const (
	stepBranches      createStep = iota
	stepAction                   // "Use directly" o "New branch from base"
	stepNewBranchName            // input nombre nueva rama
	stepWtName                   // input nombre worktree
	stepExecuting                // git worktree add en progreso
	stepDone                     // resultado antes de cerrar
)

// createDoneMsg se emite cuando el worktree se crea con éxito.
type createDoneMsg struct{ path string }

// createCancelledMsg se emite cuando el usuario cancela.
type createCancelledMsg struct{}

// createExecDoneMsg es interno: resultado de la ejecución async.
type createExecDoneMsg struct {
	path string
	log  []string
	err  bool
}

type BranchInfo struct {
	Name       string
	IsRemote   bool
	ExistingWT string // path del worktree existente, o ""
}

// CreateModel es la máquina de estados de la subpantalla de creación.
type CreateModel struct {
	step createStep

	branches   []BranchInfo
	filter     string
	brSelected int
	brScroll   int

	chosenBranch   string
	chosenIsRemote bool
	chosenHasWT    bool
	flow           string // "direct" | "new_branch"
	newBranchName  string
	defaultWtName  string

	textInput textinput.Model
	actionSel int // 0=Use directly, 1=New branch

	execLog  []string
	execErr  bool
	execPath string

	width  int
	height int
	root   string
	data   []git.WorktreeEntry
}

// NewCreateModel inicializa el modelo de creación.
func NewCreateModel(data []git.WorktreeEntry, root string, width, height int) *CreateModel {
	ti := textinput.New()
	ti.CharLimit = 80

	branches := loadBranches(root, data)

	return &CreateModel{
		step:      stepBranches,
		branches:  branches,
		textInput: ti,
		width:     width,
		height:    height,
		root:      root,
		data:      data,
	}
}

func (c *CreateModel) Init() tea.Cmd {
	return nil
}

func (c *CreateModel) Update(msg tea.Msg) (*CreateModel, tea.Cmd) {
	switch c.step {
	case stepBranches:
		return c.updateBranches(msg)
	case stepAction:
		return c.updateAction(msg)
	case stepNewBranchName:
		return c.updateTextInput(msg, "new_branch_name")
	case stepWtName:
		return c.updateTextInput(msg, "wt_name")
	case stepExecuting:
		return c.updateExecuting(msg)
	case stepDone:
		return c.updateDone(msg)
	}
	return c, nil
}

func (c *CreateModel) updateBranches(msg tea.Msg) (*CreateModel, tea.Cmd) {
	filtered := c.filteredBranches()
	page := c.pageSize()

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return nil, func() tea.Msg { return createCancelledMsg{} }
		case "esc":
			return nil, func() tea.Msg { return createCancelledMsg{} }
		case "enter":
			if len(filtered) == 0 {
				return c, nil
			}
			chosen := filtered[c.brSelected]
			c.chosenBranch = chosen.Name
			c.chosenIsRemote = chosen.IsRemote
			c.chosenHasWT = chosen.ExistingWT != ""
			if c.chosenHasWT {
				c.flow = "new_branch"
				c.step = stepNewBranchName
				c.textInput.SetValue("")
				c.textInput.Placeholder = "new branch name"
				c.textInput.Focus()
			} else {
				c.step = stepAction
				c.actionSel = 0
			}
		case "j", "down":
			if c.brSelected < len(filtered)-1 {
				c.brSelected++
			}
		case "k", "up":
			if c.brSelected > 0 {
				c.brSelected--
			}
		case "g":
			c.brSelected = 0
		case "G":
			c.brSelected = max(0, len(filtered)-1)
		case "backspace":
			if len(c.filter) > 0 {
				c.filter = c.filter[:len([]rune(c.filter))-1]
				c.brSelected = 0
				c.brScroll = 0
			}
		default:
			// Filtro: añadir caracteres imprimibles
			if len(msg.Runes) == 1 {
				c.filter += string(msg.Runes)
				c.brSelected = 0
				c.brScroll = 0
			}
		}
	}

	// Ajustar scroll
	if c.brSelected < c.brScroll {
		c.brScroll = c.brSelected
	} else if c.brSelected >= c.brScroll+page {
		c.brScroll = c.brSelected - page + 1
	}
	_ = filtered
	return c, nil
}

func (c *CreateModel) updateAction(msg tea.Msg) (*CreateModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return nil, func() tea.Msg { return createCancelledMsg{} }
		case "esc":
			c.step = stepBranches
		case "enter":
			if c.actionSel == 0 {
				c.flow = "direct"
				c.defaultWtName = strings.ReplaceAll(c.chosenBranch, "/", "-")
				c.step = stepWtName
				c.textInput.SetValue(c.defaultWtName)
				c.textInput.Placeholder = "worktree name"
				c.textInput.Focus()
			} else {
				c.flow = "new_branch"
				c.step = stepNewBranchName
				c.textInput.SetValue("")
				c.textInput.Placeholder = "new branch name"
				c.textInput.Focus()
			}
		case "left", "h":
			if c.actionSel > 0 {
				c.actionSel--
			}
		case "right", "l":
			if c.actionSel < 1 {
				c.actionSel++
			}
		case "1":
			c.actionSel = 0
		case "2":
			c.actionSel = 1
		}
	}
	return c, nil
}

func (c *CreateModel) updateTextInput(msg tea.Msg, field string) (*CreateModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return nil, func() tea.Msg { return createCancelledMsg{} }
		case "esc":
			if field == "new_branch_name" {
				if c.chosenHasWT {
					c.step = stepBranches
				} else {
					c.step = stepAction
				}
			} else { // wt_name
				if c.flow == "new_branch" {
					c.step = stepNewBranchName
					c.textInput.SetValue(c.newBranchName)
					c.textInput.Placeholder = "new branch name"
					c.textInput.Focus()
				} else {
					c.step = stepAction
				}
			}
			return c, nil
		case "enter":
			val := strings.TrimSpace(c.textInput.Value())
			if val == "" {
				return c, nil
			}
			if field == "new_branch_name" {
				c.newBranchName = val
				c.defaultWtName = strings.ReplaceAll(val, "/", "-")
				c.step = stepWtName
				c.textInput.SetValue(c.defaultWtName)
				c.textInput.Placeholder = "worktree name"
			} else { // wt_name
				wtName := val
				c.step = stepExecuting
				c.execLog = nil
				c.execErr = false
				return c, c.executeCmd(wtName)
			}
			return c, nil
		}
	}
	var cmd tea.Cmd
	c.textInput, cmd = c.textInput.Update(msg)
	return c, cmd
}

func (c *CreateModel) updateExecuting(msg tea.Msg) (*CreateModel, tea.Cmd) {
	switch msg := msg.(type) {
	case createExecDoneMsg:
		c.execLog = msg.log
		c.execErr = msg.err
		c.execPath = msg.path
		c.step = stepDone
	}
	return c, nil
}

func (c *CreateModel) updateDone(msg tea.Msg) (*CreateModel, tea.Cmd) {
	switch msg.(type) {
	case tea.KeyMsg:
		if c.execErr {
			// Resetear a stepBranches para reintentar
			c.step = stepBranches
			c.filter = ""
			c.brSelected = 0
			c.brScroll = 0
			c.chosenBranch = ""
			c.newBranchName = ""
			c.flow = ""
			return c, nil
		}
		path := c.execPath
		return nil, func() tea.Msg { return createDoneMsg{path: path} }
	}
	return c, nil
}

func (c *CreateModel) executeCmd(wtName string) tea.Cmd {
	root := c.root
	chosenBranch := c.chosenBranch
	chosenIsRemote := c.chosenIsRemote
	flow := c.flow
	newBranchName := c.newBranchName

	return func() tea.Msg {
		worktreesDir := filepath.Join(root, ".claude", "worktrees")
		os.MkdirAll(worktreesDir, 0755)
		wtPath := filepath.Join(worktreesDir, wtName)

		var args []string
		if flow == "new_branch" {
			baseRef := chosenBranch
			if chosenIsRemote {
				baseRef = "origin/" + chosenBranch
			}
			args = []string{"-C", root, "worktree", "add", "-b", newBranchName, wtPath, baseRef}
		} else {
			if chosenIsRemote {
				args = []string{"-C", root, "worktree", "add", "--track", "-b", chosenBranch, wtPath, "origin/" + chosenBranch}
			} else {
				args = []string{"-C", root, "worktree", "add", wtPath, chosenBranch}
			}
		}

		cmd := exec.Command("git", args...)
		out, err := cmd.CombinedOutput()
		var log []string
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if line != "" {
				log = append(log, line)
			}
		}

		if err != nil {
			log = append(log, fmt.Sprintf("\033[31m✗ git worktree add failed\033[0m"))
			return createExecDoneMsg{log: log, err: true}
		}

		// Ejecutar hooks
		for _, hookPath := range readCreateHooks(root) {
			if _, err := os.Stat(hookPath); err != nil {
				log = append(log, fmt.Sprintf("  (hook not found: %s)", hookPath))
				continue
			}
			log = append(log, fmt.Sprintf("  $ %s", filepath.Base(hookPath)))
			hCmd := exec.Command(hookPath, wtPath, wtName)
			hOut, _ := hCmd.CombinedOutput()
			for _, line := range strings.Split(strings.TrimSpace(string(hOut)), "\n") {
				if line != "" {
					log = append(log, line)
				}
			}
		}

		log = append(log, "\033[32m✓ Worktree ready\033[0m")
		return createExecDoneMsg{path: wtPath, log: log, err: false}
	}
}

func (c *CreateModel) View() string {
	switch c.step {
	case stepBranches:
		return c.viewBranches()
	case stepAction:
		return c.viewAction()
	case stepNewBranchName:
		return c.viewTextInput("Create worktree — new branch name", fmt.Sprintf("Base: %s", styleCyan.Render(c.chosenBranch)))
	case stepWtName:
		extra := ""
		if c.newBranchName != "" {
			extra = fmt.Sprintf("  ->  %s", styleCyan.Render(c.newBranchName))
		}
		return c.viewTextInput("Create worktree — name", fmt.Sprintf("Base: %s%s", styleCyan.Render(c.chosenBranch), extra))
	case stepExecuting:
		return c.viewExecuting()
	case stepDone:
		return c.viewDone()
	}
	return ""
}

func (c *CreateModel) viewBranches() string {
	filtered := c.filteredBranches()
	page := c.pageSize()

	var sb strings.Builder
	sb.WriteString(styleBold.Render("  Create worktree — select base branch") + "\n")
	sb.WriteString("  " + styleDim.Render(strings.Repeat("─", 42)) + "\n\n")
	if c.filter != "" {
		sb.WriteString(fmt.Sprintf("  %s%s\n\n", styleDim.Render("Filter: "), styleCyan.Render(c.filter)))
	} else {
		sb.WriteString("  " + styleDim.Render("Type to filter · ↑↓ navigate · Enter select · Esc back · q quit") + "\n\n")
	}

	end := min(c.brScroll+page, len(filtered))
	for i, b := range filtered[c.brScroll:end] {
		idx := c.brScroll + i
		cursor := "  "
		if idx == c.brSelected {
			cursor = styleBold.Render(">") + " "
		}
		annotation := ""
		if b.ExistingWT != "" {
			rel, _ := filepath.Rel(c.root, b.ExistingWT)
			annotation = "  " + styleDim.Render(fmt.Sprintf("(worktree: %s)", rel))
		}
		name := b.Name
		if b.ExistingWT != "" {
			name = styleDim.Render(name)
		}
		sb.WriteString(fmt.Sprintf("%s%s%s\n", cursor, name, annotation))
	}

	return sb.String()
}

func (c *CreateModel) viewAction() string {
	opts := []string{"Use directly", "New branch from base"}
	var sb strings.Builder
	sb.WriteString(styleBold.Render("  Create worktree — choose action") + "\n")
	sb.WriteString("  " + styleDim.Render(strings.Repeat("─", 42)) + "\n\n")
	sb.WriteString(fmt.Sprintf("  Base branch: %s\n\n", styleCyan.Render(c.chosenBranch)))
	sb.WriteString("  " + styleDim.Render("← → navigate · Enter confirm · Esc back") + "\n\n  ")
	for i, opt := range opts {
		if i == c.actionSel {
			sb.WriteString(styleBold.Render(fmt.Sprintf("[ %d) %s ]", i+1, opt)))
		} else {
			sb.WriteString(styleDim.Render(fmt.Sprintf("  %d) %s  ", i+1, opt)))
		}
		if i < len(opts)-1 {
			sb.WriteString("   ")
		}
	}
	sb.WriteString("\n")
	return sb.String()
}

func (c *CreateModel) viewTextInput(title, subtitle string) string {
	var sb strings.Builder
	sb.WriteString(styleBold.Render("  "+title) + "\n")
	sb.WriteString("  " + styleDim.Render(strings.Repeat("─", 42)) + "\n\n")
	sb.WriteString("  " + subtitle + "\n\n")
	sb.WriteString("  " + c.textInput.View() + "\n")
	return sb.String()
}

func (c *CreateModel) viewExecuting() string {
	return styleBold.Render("  Creating worktree...") + "\n" +
		"  " + styleDim.Render(strings.Repeat("─", 42)) + "\n\n"
}

func (c *CreateModel) viewDone() string {
	var sb strings.Builder
	sb.WriteString(styleBold.Render("  Create worktree") + "\n")
	sb.WriteString("  " + styleDim.Render(strings.Repeat("─", 42)) + "\n\n")
	for _, line := range c.execLog {
		sb.WriteString("  " + line + "\n")
	}
	sb.WriteString("\n  " + styleDim.Render("Press any key to continue...") + "\n")
	return sb.String()
}

func (c *CreateModel) filteredBranches() []BranchInfo {
	if c.filter == "" {
		return c.branches
	}
	fl := strings.ToLower(c.filter)
	var result []BranchInfo
	for _, b := range c.branches {
		if strings.Contains(strings.ToLower(b.Name), fl) {
			result = append(result, b)
		}
	}
	return result
}

func (c *CreateModel) pageSize() int {
	ps := c.height - 9
	if ps < 1 {
		return 1
	}
	return ps
}

// loadBranches obtiene ramas locales y remotas del repo.
func loadBranches(root string, data []git.WorktreeEntry) []BranchInfo {
	wtMap := make(map[string]string)
	for _, wt := range data {
		if wt.Branch != "" {
			wtMap[wt.Branch] = wt.Path
		}
	}

	localOut, _ := exec.Command("git", "-C", root, "branch", "--sort=-committerdate", "--format=%(refname:short)").Output()
	remoteOut, _ := exec.Command("git", "-C", root, "branch", "-r", "--sort=-committerdate", "--format=%(refname:short)").Output()

	seen := make(map[string]bool)
	var result []BranchInfo

	for _, b := range strings.Split(strings.TrimSpace(string(localOut)), "\n") {
		b = strings.TrimSpace(b)
		if b == "" {
			continue
		}
		seen[b] = true
		result = append(result, BranchInfo{Name: b, IsRemote: false, ExistingWT: wtMap[b]})
	}
	for _, r := range strings.Split(strings.TrimSpace(string(remoteOut)), "\n") {
		r = strings.TrimSpace(r)
		if r == "" || strings.Contains(r, "HEAD") {
			continue
		}
		parts := strings.SplitN(r, "/", 2)
		b := r
		if len(parts) == 2 {
			b = parts[1]
		}
		if !seen[b] {
			seen[b] = true
			result = append(result, BranchInfo{Name: b, IsRemote: true, ExistingWT: wtMap[b]})
		}
	}
	if len(result) > 60 {
		result = result[:60]
	}
	return result
}

// readCreateHooks parsea .wtm-config.yaml y devuelve los hooks de create-worktree.
func readCreateHooks(root string) []string {
	configPath := filepath.Join(root, ".wtm-config.yaml")
	f, err := os.Open(configPath)
	if err != nil {
		return nil
	}
	defer f.Close()

	var results []string
	inHooks, inCreate := false, false

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		stripped := strings.TrimRight(line, " \t")
		if stripped == "" || strings.HasPrefix(strings.TrimSpace(stripped), "#") {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " \t"))
		if indent == 0 {
			inHooks = strings.TrimRight(stripped, ":") == "hooks"
			inCreate = false
		} else if inHooks && !inCreate {
			if strings.Contains(stripped, ":") {
				key, val, _ := strings.Cut(stripped, ":")
				if strings.TrimSpace(key) == "create-worktree" {
					v := strings.TrimSpace(val)
					if v != "" {
						return []string{v}
					}
					inCreate = true
				}
			}
		} else if inCreate {
			s := strings.TrimSpace(stripped)
			if strings.HasPrefix(s, "- ") {
				results = append(results, strings.TrimSpace(s[2:]))
			} else {
				break
			}
		}
	}
	return results
}
