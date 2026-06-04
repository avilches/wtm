import json
import os
import signal
import subprocess
import sys
import termios
import tty as tty_mod

_XPATH = "/opt/homebrew/bin:/opt/homebrew/sbin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin"


def _env():
    e = os.environ.copy()
    e["PATH"] = _XPATH + ":" + e.get("PATH", "")
    return e


def _load_data():
    r = subprocess.run(
        ["/opt/homebrew/bin/wt", "list", "--format", "json"],
        capture_output=True, text=True, env=_env(),
    )
    if r.returncode != 0:
        print(r.stderr, file=sys.stderr)
        sys.exit(r.returncode)
    return json.loads(r.stdout)


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
        return True, None

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
        return True, None

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
        return True, None

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
        return True, None

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
        return True, None

    elif action == "PRUNEALL":
        print()
        print("  Pruning all merged/orphan worktrees...")
        r = subprocess.run(
            ["/opt/homebrew/bin/wt", "-C", root, "step", "prune"],
            env=env,
        )
        if r.returncode == 0:
            print("\033[32m  Prune complete\033[0m")
        else:
            print("\033[31m  Prune failed\033[0m")
        _pause()
        return True, None

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
        return True, None

    elif action == "CREATED":
        return True, path

    return True, None
