package actions

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"wtm/internal/git"
)

// ActionDoneMsg es el mensaje que devuelven todas las acciones al completar.
type ActionDoneMsg struct {
	Log        []string // líneas de output a mostrar en el overlay
	SelectPath string   // worktree a preseleccionar al reabrir
	CDPath     string   // si no vacío, escribir en cdfile y hacer quit
}

func logLines(output string) []string {
	var lines []string
	for _, l := range strings.Split(strings.TrimSpace(output), "\n") {
		if l != "" {
			lines = append(lines, l)
		}
	}
	return lines
}

// CDCmd escribe el path en cdfile y hace quit.
func CDCmd(path, cdfile string) tea.Cmd {
	return func() tea.Msg {
		if cdfile != "" {
			os.WriteFile(cdfile, []byte(path), 0644)
		}
		return ActionDoneMsg{CDPath: path}
	}
}

// PullCmd hace `git pull` en el worktree dado.
func PullCmd(path string) tea.Cmd {
	return func() tea.Msg {
		log := []string{fmt.Sprintf("  Pulling %s...", filepath.Base(path))}
		cmd := exec.Command("git", "-C", path, "pull")
		out, err := cmd.CombinedOutput()
		log = append(log, logLines(string(out))...)
		if err != nil {
			log = append(log, "  \033[31mPull failed\033[0m")
		} else {
			log = append(log, "  \033[32mPulled successfully\033[0m")
		}
		return ActionDoneMsg{Log: log, SelectPath: path}
	}
}

// PushCmd hace `git push -u origin <branch>`.
func PushCmd(path, branch string) tea.Cmd {
	return func() tea.Msg {
		log := []string{fmt.Sprintf("  Pushing %s...", filepath.Base(path))}
		if branch == "" {
			out, _ := exec.Command("git", "-C", path, "rev-parse", "--abbrev-ref", "HEAD").Output()
			branch = strings.TrimSpace(string(out))
		}
		cmd := exec.Command("git", "-C", path, "push", "-u", "origin", branch)
		out, err := cmd.CombinedOutput()
		log = append(log, logLines(string(out))...)
		if err != nil {
			log = append(log, "  \033[31mPush failed\033[0m")
		} else {
			log = append(log, "  \033[32mPushed successfully\033[0m")
		}
		return ActionDoneMsg{Log: log, SelectPath: path}
	}
}

// RebaseCmd hace fetch + rebase desde mainBranch.
func RebaseCmd(path, mainBranch string) tea.Cmd {
	return func() tea.Msg {
		log := []string{"  Fetching from remote..."}
		exec.Command("git", "-C", path, "fetch", "--quiet").Run()

		mainLocal := mainBranch
		if idx := strings.Index(mainBranch, "/"); idx >= 0 {
			mainLocal = mainBranch[idx+1:]
		}
		if _, err := exec.Command("git", "-C", path, "rev-parse", "--verify", mainLocal).Output(); err == nil {
			log = append(log, fmt.Sprintf("  Updating local %s...", mainLocal))
			exec.Command("git", "-C", path, "fetch", "origin", mainLocal+":"+mainLocal, "--update-head-ok").Run()
		}

		log = append(log, fmt.Sprintf("  Rebasing %s from %s...", filepath.Base(path), mainBranch))
		cmd := exec.Command("git", "-C", path, "rebase", mainBranch)
		out, err := cmd.CombinedOutput()
		log = append(log, logLines(string(out))...)
		if err != nil {
			exec.Command("git", "-C", path, "rebase", "--abort").Run()
			log = append(log, "  \033[31mConflicts - rebase aborted. Resolve manually.\033[0m")
		} else {
			log = append(log, "  \033[32mRebased successfully\033[0m")
		}
		return ActionDoneMsg{Log: log, SelectPath: path}
	}
}

// MergeCmd hace fetch + merge desde mainBranch.
func MergeCmd(path, mainBranch string) tea.Cmd {
	return func() tea.Msg {
		log := []string{"  Fetching from remote..."}
		exec.Command("git", "-C", path, "fetch", "--quiet").Run()

		mainLocal := mainBranch
		if idx := strings.Index(mainBranch, "/"); idx >= 0 {
			mainLocal = mainBranch[idx+1:]
		}
		if _, err := exec.Command("git", "-C", path, "rev-parse", "--verify", mainLocal).Output(); err == nil {
			log = append(log, fmt.Sprintf("  Updating local %s...", mainLocal))
			exec.Command("git", "-C", path, "fetch", "origin", mainLocal+":"+mainLocal, "--update-head-ok").Run()
		}

		log = append(log, fmt.Sprintf("  Merging %s into %s...", mainBranch, filepath.Base(path)))
		cmd := exec.Command("git", "-C", path, "merge", mainBranch)
		out, err := cmd.CombinedOutput()
		log = append(log, logLines(string(out))...)
		if err != nil {
			exec.Command("git", "-C", path, "merge", "--abort").Run()
			log = append(log, "  \033[31mConflicts - merge aborted. Resolve manually.\033[0m")
		} else {
			log = append(log, "  \033[32mMerged successfully\033[0m")
		}
		return ActionDoneMsg{Log: log, SelectPath: path}
	}
}

// FetchCmd hace `git fetch --all` desde el root del repo.
func FetchCmd(root string) tea.Cmd {
	return func() tea.Msg {
		log := []string{"  Fetching from root..."}
		cmd := exec.Command("git", "-C", root, "fetch", "--all", "--no-write-fetch-head")
		out, err := cmd.CombinedOutput()
		log = append(log, logLines(string(out))...)
		if err != nil {
			log = append(log, "  \033[31mFetch failed\033[0m")
		} else {
			log = append(log, "  \033[32mFetched successfully\033[0m")
		}
		return ActionDoneMsg{Log: log}
	}
}

// DeleteCmd borra el worktree. force=true para orphans (sin confirmación previa).
func DeleteCmd(path string, force bool, branch string) tea.Cmd {
	return func() tea.Msg {
		log := []string{fmt.Sprintf("  Removing worktree %s...", filepath.Base(path))}
		args := []string{"worktree", "remove", path}
		if force {
			args = []string{"worktree", "remove", "--force", path}
		}
		cmd := exec.Command("git", args...)
		out, err := cmd.CombinedOutput()
		log = append(log, logLines(string(out))...)
		if err != nil {
			log = append(log, "  \033[31mFailed to remove worktree\033[0m")
		} else {
			log = append(log, "  \033[32mWorktree removed\033[0m")
		}
		return ActionDoneMsg{Log: log}
	}
}

// PruneAllCmd hace fetch --prune + worktree prune + borra worktrees integrados limpios con >1d.
func PruneAllCmd(root string) tea.Cmd {
	return func() tea.Msg {
		log := []string{"  Pruning: fetching remote refs..."}
		exec.Command("git", "-C", root, "fetch", "--prune", "--quiet").Run()
		exec.Command("git", "-C", root, "worktree", "prune").Run()

		freshEntries, err := git.Load()
		if err != nil {
			log = append(log, "  \033[31mFailed to reload worktrees\033[0m")
			return ActionDoneMsg{Log: log}
		}

		nowTS := time.Now().Unix()
		removed, skipped := 0, 0

		for _, wt := range freshEntries {
			if wt.MainState != git.MainStateIntegrated {
				continue
			}
			if wt.IsMain {
				continue
			}
			age := nowTS - wt.Commit.Timestamp
			if age < 86400 {
				continue
			}
			w := wt.WorkingTree
			if w.Staged || w.Modified || w.Untracked || w.Deleted || w.Renamed {
				skipped++
				continue
			}
			r := exec.Command("git", "worktree", "remove", "--force", wt.Path)
			if err := r.Run(); err == nil {
				removed++
				if wt.Branch != "" {
					exec.Command("git", "-C", root, "branch", "-d", wt.Branch).Run()
				}
			}
		}

		var parts []string
		if removed > 0 {
			parts = append(parts, fmt.Sprintf("\033[32m%d removed\033[0m", removed))
		}
		if skipped > 0 {
			parts = append(parts, fmt.Sprintf("\033[33m%d skipped (dirty)\033[0m", skipped))
		}
		if len(parts) == 0 {
			log = append(log, "  \033[32mNothing to prune\033[0m")
		} else {
			log = append(log, "  "+strings.Join(parts, "  "))
		}
		return ActionDoneMsg{Log: log}
	}
}
