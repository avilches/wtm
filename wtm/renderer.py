from .colors import (
    RESET, BOLD, GREEN, YELLOW, BLUE, RED, DIM,
    UNDERLINE, WHITE_FG, BG_GREEN, BG_PURPLE, BG_RED, BG_GRAY,
    _ANSI_RE,
)
from . import pr as _pr


def ansi_pad(s, width):
    visible = len(_ANSI_RE.sub("", s))
    return s + " " * max(0, width - visible)


def age(ts, now):
    d = now - ts
    if d < 3600:
        return f"{int(d / 60)}m"
    if d < 86400:
        return f"{int(d / 3600)}h"
    return f"{int(d / 86400)}d"


def flags(worktree):
    f = ""
    w = worktree.get("working_tree", {})
    if w.get("staged"):
        f += "S"
    if w.get("modified"):
        f += "M"
    if w.get("untracked"):
        f += "?"
    if w.get("deleted"):
        f += "D"
    return f or "-"


def state_info(worktree):
    ms = worktree.get("main_state", "")
    m = worktree.get("main", {})
    a, b = m.get("ahead", 0), m.get("behind", 0)
    if ms == "is_main":
        return f"{DIM}main{RESET}", False
    if ms == "integrated":
        return f"{DIM}MERGED{RESET}", True
    if ms == "orphan":
        return f"{DIM}ORPHAN{RESET}", True
    if ms == "same_commit":
        return f"{DIM}={RESET}", False
    if ms == "would_conflict":
        return f"{YELLOW}C{RESET}", False
    if ms == "empty":
        return f"{DIM}empty{RESET}", False
    parts = []
    if a:
        parts.append(f"{GREEN}←{a}{RESET}")
    if b:
        parts.append(f"{BLUE}→{b}{RESET}")
    return " ".join(parts) or f"{DIM}={RESET}", False


def remote_sync(worktree):
    r = worktree.get("remote")
    if not r:
        return f"{DIM}no-remote{RESET}"
    a, b = r.get("ahead", 0), r.get("behind", 0)
    if not a and not b:
        return f"{DIM}up to date{RESET}"
    parts = []
    if b:
        parts.append(f"{BLUE}↓{b}{RESET}")
    if a:
        parts.append(f"{GREEN}↑{a}{RESET}")
    return " ".join(parts)


def state_color(worktree):
    ms = worktree.get("main_state", "")
    if ms in ("integrated", "orphan"):
        return RED
    if ms == "would_conflict":
        return YELLOW
    if ms in ("same_commit", "is_main", "empty"):
        return DIM
    m = worktree.get("main", {})
    if m.get("behind", 0) > 0:
        return YELLOW
    if m.get("ahead", 0) > 0:
        return GREEN
    return DIM


def col_widths(data, now):
    bw = max(len("Branch"), max((len(wt.get("branch") or "(detached)") for wt in data), default=0))
    fw = max(len("Changes"), 4)
    sw = max(len("State"),  max((len(_ANSI_RE.sub("", state_info(wt)[0])) for wt in data), default=0))
    rw = max(len("Remote"), max((len(_ANSI_RE.sub("", remote_sync(wt))) for wt in data), default=0))
    aw = max(len("Age"),    max((len(age(wt.get("commit", {}).get("timestamp", now), now)) for wt in data), default=0))
    return bw, fw, sw, rw, aw  # branch, flags, state, remote, age


def make_header(W):
    bw, fw, sw, rw, aw = W
    return f"    {'Branch':<{bw}}  {'Changes':<{fw}}  {'State':<{sw}}  {'Remote':<{rw}}  {'PR':<10}  {'Age':<{aw}}  Commit"


def build_line(wt, now, W):
    bw, fw, sw, rw, aw = W
    marker = "@" if wt["is_current"] else " "
    branch = wt.get("branch") or "(detached)"
    f = flags(wt)
    state, prunable = state_info(wt)
    rs = remote_sync(wt)
    commit = wt.get("commit", {})
    a = age(commit.get("timestamp", now), now)
    msg = commit.get("message", "")[:45]
    pr_width = 10  # 3 badge + 1 sep + up to 6 for #NNNNN
    if _pr._pr_state["loading"]:
        _spin = _pr._SPINNER[_pr._pr_state["frame"] % len(_pr._SPINNER)]
        pr_part = f"{DIM}{_spin:<{pr_width}}{RESET}"
    else:
        pr_info = _pr.pr_map.get(branch)
        if pr_info:
            pr_num = pr_info["number"]
            pr_st  = pr_info["state"]
            draft  = pr_info["draft"]
            if draft:
                badge = f"{BG_GRAY}{WHITE_FG} D {RESET}"
            elif pr_st == "OPEN":
                badge = f"{BG_GREEN}{WHITE_FG} O {RESET}"
            elif pr_st == "MERGED":
                badge = f"{BG_PURPLE}{WHITE_FG} M {RESET}"
            else:
                badge = f"{BG_RED}{WHITE_FG} C {RESET}"
            pr_text = f"#{pr_num}"
            pad = " " * max(0, pr_width - 3 - 1 - len(pr_text))
            if _pr._pr_repo_url:
                _url = f"{_pr._pr_repo_url}/pull/{pr_num}"
                link = f"\033]8;;{_url}\033\\{BLUE}{UNDERLINE}{pr_text}{RESET}\033]8;;\033\\"
            else:
                link = f"{BLUE}{UNDERLINE}{pr_text}{RESET}"
            pr_part = f"{badge} {link}{pad}"
        else:
            pr_part = f"{DIM}{'':{pr_width}}{RESET}"

    bc = (
        DIM if prunable
        else BOLD + GREEN if wt["is_current"]
        else RESET
    )
    fc = RED if f != "-" else DIM

    return (
        f"{marker} {bc}{branch:<{bw}}{RESET}  "
        f"{fc}{f:<{fw}}{RESET}  "
        f"{ansi_pad(state, sw)}  "
        f"{ansi_pad(rs, rw)}  "
        f"{pr_part}  "
        f"{DIM}{a:<{aw}}{RESET}  "
        f"{DIM}{msg}{RESET}"
    )


def detail_line(wt):
    w = wt.get("working_tree", {})
    legend = []
    if w.get("staged"):    legend.append("S=staged")
    if w.get("modified"):  legend.append("M=modified")
    if w.get("untracked"): legend.append("?=untracked")
    if w.get("deleted"):   legend.append("D=deleted")
    if not legend:
        return f"  {DIM}clean{RESET}"
    return f"  {DIM}" + "  ".join(legend) + RESET
