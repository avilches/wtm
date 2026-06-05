package git

import "strings"

type rawWorktree struct {
	Path     string
	Head     string
	Branch   string
	Detached bool
	Prunable bool
	Locked   bool
}

// parseWorktreeList parsea la salida de `git worktree list --porcelain`.
func parseWorktreeList(output string) []rawWorktree {
	var result []rawWorktree
	var cur rawWorktree
	started := false
	for _, line := range strings.Split(output, "\n") {
		switch {
		case strings.HasPrefix(line, "worktree "):
			if started {
				result = append(result, cur)
			}
			cur = rawWorktree{Path: strings.TrimPrefix(line, "worktree ")}
			started = true
		case strings.HasPrefix(line, "HEAD "):
			cur.Head = strings.TrimPrefix(line, "HEAD ")
		case strings.HasPrefix(line, "branch "):
			ref := strings.TrimPrefix(line, "branch ")
			if strings.HasPrefix(ref, "refs/heads/") {
				cur.Branch = strings.TrimPrefix(ref, "refs/heads/")
			} else {
				cur.Branch = ref
			}
		case line == "detached":
			cur.Detached = true
		case strings.HasPrefix(line, "prunable"):
			cur.Prunable = true
		case strings.HasPrefix(line, "locked"):
			cur.Locked = true
		}
	}
	if started {
		result = append(result, cur)
	}
	return result
}
