package git

import (
	"encoding/json"
	"os/exec"
	"regexp"
	"strings"
)

// FetchPRMap devuelve {branch: PRInfo} para todos los PRs via `gh` CLI.
// Devuelve nil si gh no está disponible o falla.
func FetchPRMap(root string) map[string]PRInfo {
	ghPath, err := exec.LookPath("gh")
	if err != nil {
		ghPath = "/opt/homebrew/bin/gh"
	}

	args := []string{"pr", "list", "--json", "number,headRefName,state,isDraft", "--limit", "200", "--state", "all"}
	// Fijar el repo a partir de `origin`. Sin esto, gh elige el repo base por su
	// heurística y en repos con remote `upstream` consultaría el repo ajeno.
	if repo := originRepo(root); repo != "" {
		args = append(args, "--repo", repo)
	}

	cmd := exec.Command(ghPath, args...)
	if root != "" {
		cmd.Dir = root
	}
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	var prs []struct {
		Number      int    `json:"number"`
		HeadRefName string `json:"headRefName"`
		State       string `json:"state"`
		IsDraft     bool   `json:"isDraft"`
	}
	if err := json.Unmarshal(out, &prs); err != nil {
		return nil
	}

	m := make(map[string]PRInfo, len(prs))
	for _, pr := range prs {
		cand := PRInfo{
			Number: pr.Number,
			State:  pr.State,
			Draft:  pr.IsDraft,
		}
		// Varios PRs pueden compartir headRefName. Quedarse con el más relevante:
		// preferir abierto, y a igualdad de estado el de número más alto (más reciente).
		if cur, ok := m[pr.HeadRefName]; ok && !moreRelevantPR(cand, cur) {
			continue
		}
		m[pr.HeadRefName] = cand
	}
	return m
}

// moreRelevantPR indica si a debe prevalecer sobre b para una misma rama.
func moreRelevantPR(a, b PRInfo) bool {
	pa, pb := prStateRank(a.State), prStateRank(b.State)
	if pa != pb {
		return pa > pb
	}
	return a.Number > b.Number
}

// prStateRank ordena estados por relevancia: abierto > merged > cerrado.
func prStateRank(state string) int {
	switch strings.ToUpper(state) {
	case "OPEN":
		return 2
	case "MERGED":
		return 1
	default:
		return 0
	}
}

// originRepo devuelve el identificador de repo que gh entiende para el remote
// `origin`: "owner/repo" en github.com, o "host/owner/repo" en GitHub Enterprise.
func originRepo(root string) string {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	if root != "" {
		cmd.Dir = root
	}
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	url := strings.TrimSpace(string(out))
	reHTTPS := regexp.MustCompile(`^https://([^/]+)/([^/]+)/([^/]+?)(?:\.git)?$`)
	reSSH := regexp.MustCompile(`^git@([^:]+):([^/]+)/([^/]+?)(?:\.git)?$`)
	var host, owner, repo string
	if m := reHTTPS.FindStringSubmatch(url); m != nil {
		host, owner, repo = m[1], m[2], m[3]
	} else if m := reSSH.FindStringSubmatch(url); m != nil {
		host, owner, repo = m[1], m[2], m[3]
	} else {
		return ""
	}
	if host == "github.com" {
		return owner + "/" + repo
	}
	return host + "/" + owner + "/" + repo
}

// RepoURL devuelve la URL base del repo (e.g. "https://github.com/user/repo")
// para construir links OSC 8 a PRs.
func RepoURL(root string) string {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	if root != "" {
		cmd.Dir = root
	}
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	url := strings.TrimSpace(string(out))
	reHTTPS := regexp.MustCompile(`^(https://[^/]+/[^/]+/[^/]+?)(?:\.git)?$`)
	reSSH := regexp.MustCompile(`^git@([^:]+):([^/]+)/([^/]+?)(?:\.git)?$`)
	if m := reHTTPS.FindStringSubmatch(url); m != nil {
		return m[1]
	}
	if m := reSSH.FindStringSubmatch(url); m != nil {
		return "https://" + m[1] + "/" + m[2] + "/" + m[3]
	}
	return ""
}
