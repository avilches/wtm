import os
import signal
import subprocess
import sys
import termios
import time
import tty as tty_mod

_XPATH = "/opt/homebrew/bin:/opt/homebrew/sbin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin"


def _env():
    e = os.environ.copy()
    e["PATH"] = _XPATH + ":" + e.get("PATH", "")
    return e


def _parse_worktree_list(output):
    worktrees = []
    current = {}
    for line in output.splitlines():
        if line.startswith("worktree "):
            if current:
                worktrees.append(current)
            current = {"path": line[9:], "prunable": False, "locked": False, "detached": False}
        elif line.startswith("HEAD "):
            current["head"] = line[5:]
        elif line.startswith("branch "):
            ref = line[7:]
            current["branch"] = ref[len("refs/heads/"):] if ref.startswith("refs/heads/") else ref
        elif line == "detached":
            current["detached"] = True
            current.setdefault("branch", None)
        elif line.startswith("prunable"):
            current["prunable"] = True
        elif line.startswith("locked"):
            current["locked"] = True
    if current:
        worktrees.append(current)
    return worktrees


def _current_worktree_path(cwd, raw_wts):
    """Return the path of the most specific worktree that contains cwd."""
    best, best_len = None, -1
    for raw in raw_wts:
        p = raw["path"]
        if cwd == p or cwd.startswith(p + os.sep):
            if len(p) > best_len:
                best, best_len = p, len(p)
    return best


def _build_entry(raw, cwd, current_path, main_branch, main_sha, env):
    path = raw["path"]
    branch = raw.get("branch")
    head = raw.get("head", "")
    prunable = raw.get("prunable", False)

    is_current = path == current_path

    main_local = main_branch.split("/", 1)[1] if main_branch and "/" in main_branch else (main_branch or "")
    is_main_wt = bool(branch and branch == main_local)

    # main_state
    if prunable:
        main_state = "orphan"
    elif is_main_wt:
        main_state = "is_main"
    elif not head:
        main_state = "empty"
    elif main_sha and head == main_sha:
        main_state = "same_commit"
    elif main_branch:
        r = subprocess.run(
            ["git", "-C", path, "merge-base", main_branch, "HEAD"],
            capture_output=True, text=True, env=env,
        )
        main_state = "integrated" if (r.returncode == 0 and r.stdout.strip() == head) else ""
    else:
        main_state = ""

    # Commit info
    commit = {}
    r = subprocess.run(
        ["git", "-C", path, "log", "-1", "--format=%at%x00%s%x00%H%x00%h"],
        capture_output=True, text=True, env=env,
    )
    if r.returncode == 0 and r.stdout.strip():
        parts = r.stdout.strip().split("\x00")
        if len(parts) >= 4:
            try:
                commit = {
                    "timestamp": int(parts[0]),
                    "message": parts[1],
                    "sha": parts[2],
                    "short_sha": parts[3],
                }
            except ValueError:
                pass

    # Working tree status
    working_tree = {"staged": False, "modified": False, "untracked": False, "deleted": False, "renamed": False}
    if not prunable:
        r = subprocess.run(
            ["git", "-C", path, "status", "--porcelain=v1"],
            capture_output=True, text=True, env=env,
        )
        if r.returncode == 0:
            for line in r.stdout.splitlines():
                if len(line) < 2:
                    continue
                x, y = line[0], line[1]
                if x == "?" and y == "?":
                    working_tree["untracked"] = True
                elif x == "D" or (x == " " and y == "D"):
                    working_tree["deleted"] = True
                elif x == "R" or (x == " " and y == "R"):
                    working_tree["renamed"] = True
                elif x not in (" ", "?", "!"):
                    working_tree["staged"] = True
                    if y not in (" ", "?", "!"):
                        working_tree["modified"] = True
                elif y not in (" ", "?", "!"):
                    working_tree["modified"] = True

    # Main ahead/behind
    main_ahead_behind = {"ahead": 0, "behind": 0}
    if not is_main_wt and not prunable and main_branch:
        r = subprocess.run(
            ["git", "-C", path, "rev-list", "--left-right", "--count", f"{main_branch}...HEAD"],
            capture_output=True, text=True, env=env,
        )
        if r.returncode == 0:
            parts = r.stdout.strip().split()
            if len(parts) == 2:
                main_ahead_behind = {"behind": int(parts[0]), "ahead": int(parts[1])}

    # Remote ahead/behind
    remote_info = None
    r = subprocess.run(
        ["git", "-C", path, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}"],
        capture_output=True, text=True, env=env,
    )
    if r.returncode == 0:
        upstream = r.stdout.strip()
        r2 = subprocess.run(
            ["git", "-C", path, "rev-list", "--left-right", "--count", f"{upstream}...HEAD"],
            capture_output=True, text=True, env=env,
        )
        if r2.returncode == 0:
            parts = r2.stdout.strip().split()
            if len(parts) == 2:
                up_parts = upstream.split("/", 1)
                remote_info = {
                    "name": up_parts[0] if len(up_parts) == 2 else "origin",
                    "branch": up_parts[1] if len(up_parts) == 2 else upstream,
                    "behind": int(parts[0]),
                    "ahead": int(parts[1]),
                }

    return {
        "branch": branch,
        "path": path,
        "kind": "worktree",
        "commit": commit,
        "working_tree": working_tree,
        "main_state": main_state,
        "main": main_ahead_behind,
        "remote": remote_info,
        "worktree": {"detached": raw.get("detached", False), "prunable": prunable},
        "is_main": is_main_wt,
        "is_current": is_current,
        "is_previous": False,
    }


def _load_data():
    env = _env()
    r = subprocess.run(
        ["git", "worktree", "list", "--porcelain"],
        capture_output=True, text=True, env=env,
    )
    if r.returncode != 0:
        print(r.stderr or "Failed to list worktrees", file=sys.stderr)
        sys.exit(r.returncode)

    raw_wts = _parse_worktree_list(r.stdout)
    if not raw_wts:
        return []

    main_path = raw_wts[0]["path"]
    main_branch = _get_main_branch(main_path)

    main_sha = ""
    if main_branch:
        r2 = subprocess.run(
            ["git", "-C", main_path, "rev-parse", main_branch],
            capture_output=True, text=True, env=env,
        )
        if r2.returncode == 0:
            main_sha = r2.stdout.strip()

    cwd = os.getcwd()
    current_path = _current_worktree_path(cwd, raw_wts)
    return [_build_entry(raw, cwd, current_path, main_branch, main_sha, env) for raw in raw_wts]


def _get_main_branch(path):
    env = _env()
    r = subprocess.run(
        ["git", "-C", path, "symbolic-ref", "refs/remotes/origin/HEAD"],
        capture_output=True, text=True, env=env,
    )
    if r.returncode == 0:
        ref = r.stdout.strip()
        if ref.startswith("refs/remotes/"):
            return ref[len("refs/remotes/"):]
    for candidate in ("origin/main", "origin/master"):
        r = subprocess.run(
            ["git", "-C", path, "rev-parse", "--verify", candidate],
            capture_output=True, env=env,
        )
        if r.returncode == 0:
            return candidate
    return None


def _pause(msg="  Press any key to return to picker..."):
    with open("/dev/tty", "r+b", buffering=0) as tty:
        tty.write(msg.encode())
        fd = tty.fileno()
        old = termios.tcgetattr(fd)
        try:
            tty_mod.setraw(fd)
            tty.read(1)
        finally:
            termios.tcsetattr(fd, termios.TCSADRAIN, old)
    print()


def _confirm_yes(msg):
    """Returns True if user presses Y/y/Enter."""
    with open("/dev/tty", "r+b", buffering=0) as tty:
        tty.write(msg.encode())
        fd = tty.fileno()
        old = termios.tcgetattr(fd)
        try:
            tty_mod.setraw(fd)
            ch = tty.read(1)
        finally:
            termios.tcsetattr(fd, termios.TCSADRAIN, old)
    print()
    return ch in (b"Y", b"y", b"\r", b"\n")


def _reset_sigint():
    signal.signal(signal.SIGINT, signal.SIG_DFL)


def _handle_action(action, path, cdfile, data):
    """Execute a picker action. Returns (continue_loop: bool, select_path: str|None)."""
    env = _env()
    root = next((wt.get("path", "") for wt in data if wt.get("main_state") == "is_main"), "")

    if action == "CD":
        if cdfile:
            with open(cdfile, "w") as f:
                f.write(path)
        return False, None

    elif action == "LAZYGIT":
        if cdfile:
            with open(cdfile, "w") as f:
                f.write(path)
        old_sigint = signal.signal(signal.SIGINT, signal.SIG_IGN)
        subprocess.run(["lazygit"], cwd=path, env=env, preexec_fn=_reset_sigint)
        signal.signal(signal.SIGINT, old_sigint)
        return True, path

    elif action == "PULL":
        print()
        print(f"  Pulling \033[36m{os.path.basename(path)}\033[0m...")
        r = subprocess.run(["git", "-C", path, "pull"], env=env)
        if r.returncode == 0:
            print("\033[32m  Pulled successfully\033[0m")
            _pause()
        else:
            print("\033[31m  Pull failed (conflicts or no tracking branch)\033[0m")
            _pause("  Press any key...")
        return True, path

    elif action == "PUSH":
        print()
        print(f"  Pushing \033[36m{os.path.basename(path)}\033[0m...")
        r = subprocess.run(
            ["git", "-C", path, "rev-parse", "--abbrev-ref", "HEAD"],
            capture_output=True, text=True, env=env,
        )
        branch = r.stdout.strip() if r.returncode == 0 else "HEAD"
        r = subprocess.run(["git", "-C", path, "push", "-u", "origin", branch], env=env)
        if r.returncode == 0:
            print("\033[32m  Pushed successfully\033[0m")
            _pause()
        else:
            print("\033[31m  Push failed (rejected or remote unreachable)\033[0m")
            _pause("  Press any key...")
        return True, path

    elif action in ("REBASE", "MERGE"):
        print()
        main_branch = _get_main_branch(path)
        if not main_branch:
            print("\033[31m  Could not determine main branch\033[0m")
            _pause("  Press any key...")
            return True, None

        print("  Fetching from remote...")
        subprocess.run(["git", "-C", path, "fetch", "--quiet"], env=env)

        main_local = main_branch.split("/", 1)[1] if "/" in main_branch else main_branch
        r = subprocess.run(
            ["git", "-C", path, "rev-parse", "--verify", main_local],
            capture_output=True, env=env,
        )
        if r.returncode == 0:
            print(f"  Updating local \033[33m{main_local}\033[0m...")
            subprocess.run(
                ["git", "-C", path, "fetch", "origin", f"{main_local}:{main_local}", "--update-head-ok"],
                capture_output=True, env=env,
            )

        if action == "REBASE":
            print(f"  Rebasing \033[36m{os.path.basename(path)}\033[0m from \033[33m{main_branch}\033[0m...")
            r = subprocess.run(["git", "-C", path, "rebase", main_branch], env=env)
            if r.returncode == 0:
                print("\033[32m  Rebased successfully\033[0m")
                _pause()
            else:
                subprocess.run(["git", "-C", path, "rebase", "--abort"], capture_output=True, env=env)
                print("\033[31m  Merge conflicts detected - rebase or merge from main manually\033[0m")
                _pause("  Press any key...")
                sys.exit(1)
        else:
            print(f"  Merging \033[33m{main_branch}\033[0m into \033[36m{os.path.basename(path)}\033[0m...")
            r = subprocess.run(["git", "-C", path, "merge", main_branch], env=env)
            if r.returncode == 0:
                print("\033[32m  Merged successfully\033[0m")
                _pause()
            else:
                subprocess.run(["git", "-C", path, "merge", "--abort"], capture_output=True, env=env)
                print("\033[31m  Merge conflicts detected - resolve manually\033[0m")
                _pause("  Press any key...")
                sys.exit(1)
        return True, path

    elif action == "DELETE":
        print()
        print(f"  Delete worktree \033[36m{os.path.basename(path)}\033[0m?")
        if _confirm_yes("  Are you sure? Y/n  "):
            r = subprocess.run(["git", "worktree", "remove", path], env=env)
            if r.returncode == 0:
                print("\033[32m  Worktree removed\033[0m")
            else:
                print("\033[31m  Failed - may have uncommitted changes (use lazygit to clean up first)\033[0m")
        else:
            print("  Cancelled.")
        _pause()
        return True, None

    elif action == "DELETEORPHAN":
        print()
        print(f"  Removing orphan worktree \033[36m{os.path.basename(path)}\033[0m...")
        r = subprocess.run(["git", "worktree", "remove", "--force", path], env=env)
        if r.returncode == 0:
            print("\033[32m  Worktree removed\033[0m")
        else:
            print("\033[31m  Failed to remove worktree\033[0m")
        _pause()
        return True, path

    elif action == "PRUNEALL":
        print()
        print("  Pruning: fetching remote refs...")
        subprocess.run(
            ["git", "-C", root, "fetch", "--prune", "--quiet"],
            env=env, capture_output=True,
        )
        subprocess.run(
            ["git", "-C", root, "worktree", "prune"],
            env=env, capture_output=True,
        )

        now_ts = time.time()
        fresh_data = _load_data()
        removed = 0
        skipped = 0

        for wt in fresh_data:
            if wt.get("main_state") != "integrated":
                continue
            if wt.get("path") == root:
                continue
            age = now_ts - wt.get("commit", {}).get("timestamp", now_ts)
            if age < 86400:
                continue
            wt_working = wt.get("working_tree", {})
            if any(wt_working.get(k) for k in ("staged", "modified", "untracked", "deleted", "renamed")):
                skipped += 1
                continue
            wt_path = wt.get("path", "")
            wt_branch = wt.get("branch")
            r = subprocess.run(
                ["git", "worktree", "remove", "--force", wt_path],
                env=env, capture_output=True,
            )
            if r.returncode == 0:
                removed += 1
                if wt_branch:
                    subprocess.run(
                        ["git", "-C", root, "branch", "-d", wt_branch],
                        env=env, capture_output=True,
                    )

        parts = []
        if removed:
            parts.append(f"\033[32m{removed} removed\033[0m")
        if skipped:
            parts.append(f"\033[33m{skipped} skipped (dirty)\033[0m")
        print("  " + ("  ".join(parts) if parts else "\033[32mNothing to prune\033[0m"))
        _pause()
        return True, path

    elif action == "FETCH":
        print()
        print("  Fetching from root...")
        r = subprocess.run(
            ["git", "-C", root, "fetch", "--all", "--no-write-fetch-head"],
            env=env,
        )
        if r.returncode == 0:
            print("\033[32m  Fetched successfully\033[0m")
            _pause()
        else:
            print("\033[31m  Fetch failed\033[0m")
            _pause("  Press any key...")
        return True, path

    elif action == "CREATED":
        return True, path

    return True, None
