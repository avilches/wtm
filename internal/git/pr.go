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

	hostnameFlag := hostnameFlag(root)
	args := append(
		[]string{"pr", "list", "--json", "number,headRefName,state,isDraft", "--limit", "200", "--state", "all"},
		hostnameFlag...,
	)

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
		m[pr.HeadRefName] = PRInfo{
			Number: pr.Number,
			State:  pr.State,
			Draft:  pr.IsDraft,
		}
	}
	return m
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

func hostnameFlag(root string) []string {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	if root != "" {
		cmd.Dir = root
	}
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	url := strings.TrimSpace(string(out))
	reHost := regexp.MustCompile(`^https://([^/]+)/`)
	reSSH := regexp.MustCompile(`^git@([^:]+):`)
	var host string
	if m := reHost.FindStringSubmatch(url); m != nil {
		host = m[1]
	} else if m := reSSH.FindStringSubmatch(url); m != nil {
		host = m[1]
	}
	if host != "" && strings.Contains(host, "github") && host != "github.com" {
		return []string{"--hostname", host}
	}
	return nil
}
