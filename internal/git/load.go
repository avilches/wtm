package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// gitCmd ejecuta un comando git en el directorio dado (puede ser "").
func gitCmd(dir string, args ...string) (string, error) {
	var cmdArgs []string
	if dir != "" {
		cmdArgs = append(cmdArgs, "-C", dir)
	}
	cmdArgs = append(cmdArgs, args...)
	cmd := exec.Command("git", cmdArgs...)
	out, err := cmd.Output()
	return string(out), err
}

// Load carga todos los worktrees del repo actual con sus datos.
func Load() ([]WorktreeEntry, error) {
	out, err := gitCmd("", "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	raws := parseWorktreeList(out)
	if len(raws) == 0 {
		return nil, nil
	}

	mainPath := raws[0].Path
	mainBranch := GetMainBranch(mainPath)

	mainSHA := ""
	if mainBranch != "" {
		s, err := gitCmd(mainPath, "rev-parse", mainBranch)
		if err == nil {
			mainSHA = strings.TrimSpace(s)
		}
	}

	cwd, _ := os.Getwd()
	currentPath := findCurrentWorktree(cwd, raws)

	entries := make([]WorktreeEntry, len(raws))
	for i, raw := range raws {
		entries[i] = buildEntry(raw, currentPath, mainBranch, mainSHA)
	}
	return entries, nil
}

// GetMainBranch devuelve "origin/main" o "origin/master" según el repo.
func GetMainBranch(path string) string {
	out, err := gitCmd(path, "symbolic-ref", "refs/remotes/origin/HEAD")
	if err == nil {
		ref := strings.TrimSpace(out)
		if strings.HasPrefix(ref, "refs/remotes/") {
			return strings.TrimPrefix(ref, "refs/remotes/")
		}
	}
	for _, candidate := range []string{"origin/main", "origin/master"} {
		_, err := gitCmd(path, "rev-parse", "--verify", candidate)
		if err == nil {
			return candidate
		}
	}
	return ""
}

func findCurrentWorktree(cwd string, raws []rawWorktree) string {
	best, bestLen := "", -1
	sep := string(filepath.Separator)
	for _, raw := range raws {
		p := raw.Path
		if cwd == p || strings.HasPrefix(cwd, p+sep) {
			if len(p) > bestLen {
				best, bestLen = p, len(p)
			}
		}
	}
	return best
}

func buildEntry(raw rawWorktree, currentPath, mainBranch, mainSHA string) WorktreeEntry {
	e := WorktreeEntry{
		Branch:    raw.Branch,
		Path:      raw.Path,
		Head:      raw.Head,
		Detached:  raw.Detached,
		Prunable:  raw.Prunable,
		Locked:    raw.Locked,
		IsCurrent: raw.Path == currentPath,
	}

	mainLocal := mainBranch
	if idx := strings.Index(mainBranch, "/"); idx >= 0 {
		mainLocal = mainBranch[idx+1:]
	}
	e.IsMain = raw.Branch != "" && raw.Branch == mainLocal

	// mainState
	switch {
	case raw.Prunable:
		e.MainState = MainStateOrphan
	case e.IsMain:
		e.MainState = MainStateIsMain
	case raw.Head == "":
		e.MainState = MainStateEmpty
	case mainSHA != "" && raw.Head == mainSHA:
		e.MainState = MainStateSameCommit
	case mainBranch != "":
		base, err := gitCmd(raw.Path, "merge-base", mainBranch, "HEAD")
		if err == nil && strings.TrimSpace(base) == raw.Head {
			e.MainState = MainStateIntegrated
		}
	}

	// commit info
	out, err := gitCmd(raw.Path, "log", "-1", "--format=%at\x00%s\x00%H\x00%h")
	if err == nil {
		parts := strings.Split(strings.TrimSpace(out), "\x00")
		if len(parts) >= 4 {
			ts, _ := strconv.ParseInt(parts[0], 10, 64)
			e.Commit = CommitInfo{
				Timestamp: ts,
				Message:   parts[1],
				SHA:       parts[2],
				ShortSHA:  parts[3],
			}
		}
	}

	// working tree status
	if !raw.Prunable {
		out, err = gitCmd(raw.Path, "status", "--porcelain=v1")
		if err == nil {
			e.WorkingTree = parseStatus(out)
		}
	}

	// main ahead/behind
	if !e.IsMain && !raw.Prunable && mainBranch != "" {
		out, err = gitCmd(raw.Path, "rev-list", "--left-right", "--count", mainBranch+"...HEAD")
		if err == nil {
			parts := strings.Fields(strings.TrimSpace(out))
			if len(parts) == 2 {
				behind, _ := strconv.Atoi(parts[0])
				ahead, _ := strconv.Atoi(parts[1])
				e.Main = MainInfo{Behind: behind, Ahead: ahead}
			}
		}
	}

	// remote upstream
	out, err = gitCmd(raw.Path, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}")
	if err == nil {
		upstream := strings.TrimSpace(out)
		out2, err2 := gitCmd(raw.Path, "rev-list", "--left-right", "--count", upstream+"...HEAD")
		if err2 == nil {
			parts := strings.Fields(strings.TrimSpace(out2))
			if len(parts) == 2 {
				behind, _ := strconv.Atoi(parts[0])
				ahead, _ := strconv.Atoi(parts[1])
				upParts := strings.SplitN(upstream, "/", 2)
				name, branch := "origin", upstream
				if len(upParts) == 2 {
					name, branch = upParts[0], upParts[1]
				}
				e.Remote = &RemoteInfo{Name: name, Branch: branch, Behind: behind, Ahead: ahead}
			}
		}
	}

	return e
}

func parseStatus(out string) WorkingTree {
	var wt WorkingTree
	for _, line := range strings.Split(out, "\n") {
		if len(line) < 2 {
			continue
		}
		x, y := line[0], line[1]
		switch {
		case x == '?' && y == '?':
			wt.Untracked = true
		case x == 'D' || (x == ' ' && y == 'D'):
			wt.Deleted = true
		case x == 'R' || (x == ' ' && y == 'R'):
			wt.Renamed = true
		case x != ' ' && x != '?' && x != '!':
			wt.Staged = true
			if y != ' ' && y != '?' && y != '!' {
				wt.Modified = true
			}
		case y != ' ' && y != '?' && y != '!':
			wt.Modified = true
		}
	}
	return wt
}
