import json
import re
import shutil
import subprocess
import threading

pr_map: dict = {}
_pr_repo_url: str = ""
_pr_ready = threading.Event()
_pr_state = {"loading": True, "frame": 0}
_SPINNER = list("⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏")


def fetch_pr_map(cwd=None) -> dict:
    """Returns {branch: pr_info} for all PRs. No-op if gh is unavailable."""
    global _pr_repo_url
    try:
        url = subprocess.check_output(
            ["git", "remote", "get-url", "origin"],
            stderr=subprocess.DEVNULL, text=True, cwd=cwd,
        ).strip()
        m = re.match(r"^https://([^/]+)/", url) or re.match(r"^git@([^:]+):", url)
        hostname_flag = []
        if m:
            host = m.group(1)
            if "github" in host and host != "github.com":
                hostname_flag = ["--hostname", host]
        m_https = re.match(r"^(https://[^/]+/[^/]+/[^/]+?)(?:\.git)?$", url)
        m_ssh = re.match(r"^git@([^:]+):([^/]+)/([^/]+?)(?:\.git)?$", url)
        if m_https:
            _pr_repo_url = m_https.group(1)
        elif m_ssh:
            _pr_repo_url = f"https://{m_ssh.group(1)}/{m_ssh.group(2)}/{m_ssh.group(3)}"
        gh = shutil.which("gh") or "/opt/homebrew/bin/gh"
        out = subprocess.check_output(
            [gh, "pr", "list", "--json", "number,headRefName,state,isDraft",
             "--limit", "200", "--state", "all"] + hostname_flag,
            stderr=subprocess.DEVNULL, text=True, cwd=cwd,
        )
        return {
            pr["headRefName"]: {
                "number": pr["number"],
                "state":  pr["state"],
                "draft":  pr.get("isDraft", False),
            }
            for pr in json.loads(out)
        }
    except Exception:
        return {}
