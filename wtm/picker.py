import os
import select
import shutil
import subprocess
import termios
import threading
import tty as tty_mod

from .colors import RESET, BOLD, WHITE, GREEN, BLUE, RED, DIM, CYAN
from .renderer import build_line, detail_line
from .predicates import can_pull, can_push, can_rebase_or_merge, can_delete, is_orphan
from . import pr as _pr

_LAZYGIT = shutil.which("lazygit")


def get_all_branches(root, data):
    """All local + remote branches annotated with existing worktree path."""
    wt_map = {}
    for wt in data:
        b = wt.get("branch")
        if b:
            wt_map[b] = wt.get("path", "")

    local_r = subprocess.run(
        ["git", "-C", root, "branch", "--sort=-committerdate", "--format=%(refname:short)"],
        capture_output=True, text=True,
    )
    local_branches = [b.strip() for b in local_r.stdout.splitlines() if b.strip()]

    remote_r = subprocess.run(
        ["git", "-C", root, "branch", "-r", "--sort=-committerdate", "--format=%(refname:short)"],
        capture_output=True, text=True,
    )
    remote_raw = [b.strip() for b in remote_r.stdout.splitlines()
                  if b.strip() and "HEAD" not in b]

    seen = set()
    result = []
    for b in local_branches:
        seen.add(b)
        result.append((b, False, wt_map.get(b)))
    for r in remote_raw:
        parts = r.split("/", 1)
        b = parts[1] if len(parts) == 2 else r
        if b not in seen:
            seen.add(b)
            result.append((b, True, wt_map.get(b)))

    return result[:60]


def run_create_picker(data, root, tty_file, fd):
    """
    Inline create-worktree subscreen.
    Returns 'CREATED:<path>' or None (cancelled).
    tty_file must already be open in raw mode.
    """
    def write(s):
        tty_file.write(s.encode() if isinstance(s, str) else s)

    def clear():
        write(b"\033[H\033[J")

    def term_rows():
        try:
            _, rows = os.get_terminal_size(fd)
        except OSError:
            rows = 24
        return rows

    def read_input_line(prompt, default=""):
        """Read text input in raw mode. Returns string or None (Esc/q)."""
        buf = list(default)

        def redraw():
            write(f"\r\033[2K  {DIM}{prompt}{RESET}{CYAN}{''.join(buf)}{RESET}")

        write(b"\033[?25h")
        redraw()

        while True:
            ch = tty_file.read(1)
            if ch in (b"\r", b"\n"):
                write(b"\033[?25l")
                return "".join(buf)
            elif ch == b"\x1b":
                r = select.select([tty_file], [], [], 0.05)[0]
                if not r:
                    write(b"\033[?25l")
                    return None
                seq = tty_file.read(1)
                if seq == b"[":
                    tty_file.read(1)  # consume arrow, treat as Esc
                write(b"\033[?25l")
                return None
            elif ch in (b"\x7f", b"\x08"):
                if buf:
                    buf.pop()
                    redraw()
            elif ch in (b"q", b"\x03"):
                write(b"\033[?25l")
                return None
            elif len(ch) == 1 and 32 <= ch[0] <= 126:
                buf.append(ch.decode())
                redraw()

    def pick_action_menu(options):
        """Horizontal button menu. Returns selected index or None (Esc/q)."""
        sel = 0

        def redraw():
            parts = []
            for i, opt in enumerate(options):
                if i == sel:
                    parts.append(f"{BOLD}{WHITE}[ {i + 1}) {opt} ]{RESET}")
                else:
                    parts.append(f"{DIM}  {i + 1}) {opt}  {RESET}")
            write(f"\r\033[2K  {'   '.join(parts)}")

        redraw()

        while True:
            ch = tty_file.read(1)
            if ch in (b"\r", b"\n"):
                return sel
            elif ch == b"\x1b":
                r = select.select([tty_file], [], [], 0.05)[0]
                if not r:
                    return None
                seq = tty_file.read(1)
                if seq == b"[":
                    arrow = tty_file.read(1)
                    if arrow == b"C" and sel < len(options) - 1:
                        sel += 1
                        redraw()
                    elif arrow == b"D" and sel > 0:
                        sel -= 1
                        redraw()
                else:
                    return None
            elif ch in (b"q", b"\x03"):
                return None
            else:
                for i in range(len(options)):
                    if ch == str(i + 1).encode():
                        return i

    branches = get_all_branches(root, data)

    # State
    filter_str = ""
    br_selected = 0
    br_scroll = 0
    chosen_branch = None
    chosen_is_remote = False
    chosen_has_wt = False
    flow = None  # "direct" | "new_branch"
    new_branch_name = None
    default_wt_name = ""
    wt_name = None

    step = "branches"

    while True:

        if step == "branches":
            fb = [(b, r, p) for b, r, p in branches
                  if not filter_str or filter_str.lower() in b.lower()]
            if br_selected >= len(fb):
                br_selected = max(0, len(fb) - 1)
            page = max(1, term_rows() - 9)
            if br_selected < br_scroll:
                br_scroll = br_selected
            elif br_selected >= br_scroll + page:
                br_scroll = br_selected - page + 1

            clear()
            write(f"{BOLD}  Create worktree — select base branch{RESET}\r\n")
            write(f"  {DIM}{'─' * 42}{RESET}\r\n")
            if filter_str:
                write(f"\r\n  {DIM}Filter: {RESET}{CYAN}{filter_str}{RESET}\r\n\r\n")
            else:
                write(f"\r\n  {DIM}Type to filter · ↑↓ navigate · Enter select · Esc back · q quit{RESET}\r\n\r\n")

            for i, (b, is_r, wt_p) in enumerate(fb[br_scroll:br_scroll + page]):
                cursor = f"{BOLD}{WHITE}>{RESET} " if br_scroll + i == br_selected else "  "
                annotation = ""
                if wt_p:
                    rel = os.path.relpath(wt_p, root) if root else wt_p
                    annotation = f"  {DIM}(worktree: {rel}){RESET}"
                dim = DIM if wt_p else ""
                write(f"{cursor}{dim}{b}{RESET}{annotation}\r\n")

            ch = tty_file.read(1)

            if ch in (b"q", b"\x03"):
                return None
            elif ch in (b"\r", b"\n"):
                if not fb:
                    continue
                chosen_branch, chosen_is_remote, wt_p = fb[br_selected]
                chosen_has_wt = bool(wt_p)
                if chosen_has_wt:
                    flow = "new_branch"
                    step = "new_branch_name"
                else:
                    step = "action"
            elif ch == b"\x1b":
                r = select.select([tty_file], [], [], 0.05)[0]
                if not r:
                    return None  # Esc -> back to main picker
                seq = tty_file.read(1)
                if seq == b"[":
                    arrow = tty_file.read(1)
                    if arrow == b"A" and br_selected > 0:
                        br_selected -= 1
                    elif arrow == b"B" and br_selected < len(fb) - 1:
                        br_selected += 1
            elif ch == b"j" and br_selected < len(fb) - 1:
                br_selected += 1
            elif ch == b"k" and br_selected > 0:
                br_selected -= 1
            elif ch == b"g":
                br_selected = 0
            elif ch == b"G":
                br_selected = max(0, len(fb) - 1)
            elif ch == b"\x7f":
                if filter_str:
                    filter_str = filter_str[:-1]
                    br_selected = 0
                    br_scroll = 0
            elif len(ch) == 1 and 32 <= ch[0] <= 126:
                filter_str += ch.decode()
                br_selected = 0
                br_scroll = 0

        elif step == "action":
            clear()
            write(f"{BOLD}  Create worktree — choose action{RESET}\r\n")
            write(f"  {DIM}{'─' * 42}{RESET}\r\n")
            write(f"\r\n  Base branch: {CYAN}{chosen_branch}{RESET}\r\n\r\n")
            write(f"  {DIM}← →  navigate · Enter  confirm · Esc  back{RESET}\r\n\r\n")
            write("  ")
            action_idx = pick_action_menu(["Use directly", "New branch from base"])
            if action_idx is None:
                step = "branches"
            elif action_idx == 0:
                flow = "direct"
                default_wt_name = chosen_branch.replace("/", "-")
                step = "wt_name"
            else:
                flow = "new_branch"
                step = "new_branch_name"

        elif step == "new_branch_name":
            clear()
            write(f"{BOLD}  Create worktree — new branch name{RESET}\r\n")
            write(f"  {DIM}{'─' * 42}{RESET}\r\n")
            write(f"\r\n  Base: {CYAN}{chosen_branch}{RESET}\r\n\r\n")
            result = read_input_line("New branch name: ")
            if result is None:
                step = "branches" if chosen_has_wt else "action"
            elif not result.strip():
                pass  # redraw same step
            else:
                new_branch_name = result.strip()
                default_wt_name = new_branch_name.replace("/", "-")
                step = "wt_name"

        elif step == "wt_name":
            clear()
            write(f"{BOLD}  Create worktree — name{RESET}\r\n")
            write(f"  {DIM}{'─' * 42}{RESET}\r\n\r\n")
            write(f"  Base: {CYAN}{chosen_branch}{RESET}")
            if new_branch_name:
                write(f"  {DIM}->{RESET}  {CYAN}{new_branch_name}{RESET}")
            write(f"\r\n\r\n")
            result = read_input_line("Worktree name: ", default=default_wt_name)
            if result is None:
                step = "new_branch_name" if flow == "new_branch" else "action"
            else:
                wt_name = result.strip() or default_wt_name
                step = "execute"

        elif step == "execute":
            worktrees_dir = os.path.join(root, ".claude", "worktrees")
            wt_path = os.path.join(worktrees_dir, wt_name)

            clear()
            write(f"{BOLD}  Creating worktree...{RESET}\r\n")
            write(f"  {DIM}{'─' * 42}{RESET}\r\n\r\n")
            write(f"  {DIM}Path: {wt_path}{RESET}\r\n\r\n")

            os.makedirs(worktrees_dir, exist_ok=True)

            if flow == "new_branch":
                base_ref = f"origin/{chosen_branch}" if chosen_is_remote else chosen_branch
                cmd = ["git", "-C", root, "worktree", "add",
                       "-b", new_branch_name, wt_path, base_ref]
            else:  # direct
                if chosen_is_remote:
                    cmd = ["git", "-C", root, "worktree", "add",
                           "--track", "-b", chosen_branch, wt_path, f"origin/{chosen_branch}"]
                else:
                    cmd = ["git", "-C", root, "worktree", "add", wt_path, chosen_branch]

            proc = subprocess.run(cmd, capture_output=True, text=True)

            if proc.returncode == 0:
                _local_hook = os.path.join(root, "hooks", "create-worktree-hook.sh")
                _hook_r = subprocess.run(["which", "create-worktree-hook.sh"], capture_output=True)
                _hook = _local_hook if os.path.isfile(_local_hook) else (
                    "create-worktree-hook.sh" if _hook_r.returncode == 0 else None
                )
                if _hook:
                    subprocess.run([_hook, wt_path, wt_name], capture_output=True)
                write(f"  {GREEN}✓ Worktree ready{RESET}\r\n")
                write(f"\r\n  {DIM}Press any key to return to picker...{RESET}")
                tty_file.read(1)
                return f"CREATED:{wt_path}"
            else:
                error = (proc.stderr or proc.stdout).strip()
                write(f"  {RED}✗ {error}{RESET}\r\n")
                write(f"\r\n  {DIM}Press any key to try again...{RESET}")
                tty_file.read(1)
                step = "branches"
                filter_str = ""
                br_selected = 0
                br_scroll = 0
                chosen_branch = None
                chosen_is_remote = False
                chosen_has_wt = False
                flow = None
                new_branch_name = None
                default_wt_name = ""
                wt_name = None


def run_picker(wts, header, now, W, root="", select_path=None):
    """Interactive picker over /dev/tty. Returns sentinel string or None."""
    tty_file = open("/dev/tty", "r+b", buffering=0)
    fd = tty_file.fileno()
    old_settings = termios.tcgetattr(fd)
    selected = 0
    scroll = 0

    def make_items():
        return [(wt.get("path", ""), build_line(wt, now, W), wt) for wt in wts]

    items = make_items()
    selected = 0
    if select_path:
        for i, (path, _, _) in enumerate(items):
            if path == select_path:
                selected = i
                break

    def write(data):
        tty_file.write(data.encode() if isinstance(data, str) else data)

    def page_size():
        try:
            _, rows = os.get_terminal_size(fd)
        except OSError:
            rows = 24
        return max(1, rows - 9)

    def render():
        nonlocal scroll
        page = page_size()
        if selected < scroll:
            scroll = selected
        elif selected >= scroll + page:
            scroll = selected - page + 1

        write(b"\033[H\033[J")
        write(f"{BOLD}{header}{RESET}\r\n")
        write(f"  {'-' * (len(header) - 2)}\r\n")

        for i, (path, line, _wt) in enumerate(items[scroll : scroll + page]):
            cursor = f"{BOLD}\033[97m>\033[0m " if scroll + i == selected else "  "
            write(cursor + line + "\r\n")

        _path, _line, wt = items[selected]
        write(f"\r\n{detail_line(wt)}\r\n")

        def h(key, label):
            return f"{WHITE}{key}{RESET}{DIM}: {label}"

        hints = [h("C", "create"), h("f", "fetch"), h("D", "prune all")]
        if _LAZYGIT:
            hints.insert(2, h("l", "lazygit"))
        if can_pull(wt):
            rb = (wt.get("remote") or {}).get("behind", 0)
            hints.append(h("p", f"pull {RESET}{BLUE}↓{rb}{RESET}{DIM}"))
        if can_push(wt):
            ra = (wt.get("remote") or {}).get("ahead", 0)
            hints.append(h("P", f"push {RESET}{GREEN}↑{ra}{RESET}{DIM}" if ra else "push"))
        if can_rebase_or_merge(wt):
            hints.append(h("r", "rebase"))
            hints.append(h("u", "merge"))
        if can_delete(wt):
            hints.append(h("d", "delete"))
        hints.append(h("q", "quit"))
        sep = f"{RESET}{DIM} | "
        write(f"\r\n{DIM}  {sep.join(hints)}{RESET}\r\n")

        abs_path = wt.get("path", _path)
        rel_path = os.path.relpath(abs_path, root) if root else abs_path
        display_path = abs_path if rel_path == "." else rel_path
        write(f"{DIM}  {display_path}{RESET}\r\n")

    def _load_prs():
        _pr.pr_map = _pr.fetch_pr_map(cwd=root or None)
        _pr._pr_state["loading"] = False
        _pr._pr_ready.set()

    threading.Thread(target=_load_prs, daemon=True).start()

    try:
        tty_mod.setraw(fd)
        write(b"\033[?25l")
        render()

        while True:
            ready = select.select([tty_file], [], [], 0.1)[0]
            if _pr._pr_ready.is_set():
                _pr._pr_ready.clear()
                items = make_items()
                render()
            elif not ready and _pr._pr_state["loading"]:
                _pr._pr_state["frame"] += 1
                items = make_items()
                render()
            if not ready:
                continue
            ch = tty_file.read(1)

            if ch in (b"\r", b"\n"):
                return f"CD:{items[selected][0]}"

            elif ch in (b"q", b"\x03"):
                return None

            elif ch in (b"j", b"\x0e") and selected < len(items) - 1:
                selected += 1
                render()

            elif ch in (b"k", b"\x10") and selected > 0:
                selected -= 1
                render()

            elif ch == b"g":
                selected = 0
                render()

            elif ch == b"G":
                selected = len(items) - 1
                render()

            elif ch == b"l" and _LAZYGIT:
                return f"LAZYGIT:{items[selected][0]}"

            elif ch == b"r":
                if can_rebase_or_merge(items[selected][2]):
                    return f"REBASE:{items[selected][0]}"

            elif ch == b"u":
                if can_rebase_or_merge(items[selected][2]):
                    return f"MERGE:{items[selected][0]}"

            elif ch == b"p":
                if can_pull(items[selected][2]):
                    return f"PULL:{items[selected][0]}"

            elif ch == b"P":
                if can_push(items[selected][2]):
                    return f"PUSH:{items[selected][0]}"

            elif ch == b"f":
                return f"FETCH:{items[selected][0]}"

            elif ch == b"d":
                if can_delete(items[selected][2]):
                    if is_orphan(items[selected][2]):
                        return f"DELETEORPHAN:{items[selected][0]}"
                    else:
                        return f"DELETE:{items[selected][0]}"

            elif ch == b"D":
                return f"PRUNEALL:{items[selected][0]}"

            elif ch == b"C":
                result = run_create_picker(wts, root, tty_file, fd)
                if result:
                    return result
                else:
                    items = make_items()
                    render()

            elif ch == b"\x1b":
                ready = select.select([tty_file], [], [], 0.05)[0]
                if not ready:
                    return None
                seq = tty_file.read(1)
                if seq == b"[":
                    arrow = tty_file.read(1)
                    if arrow == b"A" and selected > 0:
                        selected -= 1
                        render()
                    elif arrow == b"B" and selected < len(items) - 1:
                        selected += 1
                        render()

    finally:
        try:
            termios.tcsetattr(fd, termios.TCSADRAIN, old_settings)
            write(b"\033[?25h\033[2J\033[H")
            tty_file.close()
        except Exception:
            pass

    return None
