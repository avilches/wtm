# WTM - Worktree manager

Interactive TUI for navigating and operating git worktrees. Browse worktrees (list and change directory), run common git operations, and create new worktrees without leaving the picker.

## Requirements

- [`git`](https://git-scm.com)
- [Python 3.9+](https://www.python.org/downloads/) at `/opt/homebrew/bin/python3`
- [`lazygit`](https://github.com/jesseduffield/lazygit) in PATH (optional — if not installed, the `l` key is hidden)
- [`gh`](https://cli.github.com) in PATH (optional, for the PR column)

## Installation

Add to `~/.zshrc`:

```zsh
source /path/to/wtm/wtm.plugin.zsh
```

Then open a new shell and run `wtm`.

## Features

**Navigation**

- Lists all worktrees with branch, status flags, relation to main, remote sync, and associated PR
- Navigate with `↑↓` or `j/k`, jump to first/last with `g/G`
- `Enter` to `cd` into the selected worktree

**Git operations** (available based on worktree state)

- `p` — pull from remote tracking branch
- `P` — push (creates remote if it doesn't exist)
- `r` — rebase from main
- `u` — merge from main
- `f` — fetch --all from the repo root
- `d` — delete worktree (asks for confirmation)
- `D` — prune: removes all merged/orphan worktrees

**Create worktrees** (`C`)

- Lists local and remote branches with live text filtering
- Use a branch directly or create a new branch from it
- Creates the worktree under `.claude/worktrees/<name>` inside the repo

**Lazygit** (`l`)

- Opens lazygit in the selected worktree and returns to the picker on exit
- Ctrl+C closes lazygit without killing the picker

**PR column**

- Shows each PR's status (Open, Merged, Closed, Draft) with a color badge
- PR number is a clickable link to the browser (terminals with OSC 8 support)
- Loaded in the background when the picker opens

## Columns


| Column   | Meaning                                                                      |
| -------- | ---------------------------------------------------------------------------- |
| `@`      | current worktree                                                             |
| `Branch` | branch name                                                                  |
| `Flags`  | `S`=staged `M`=modified `?`=untracked `D`=deleted `-`=clean                  |
| `State`  | relation to main:`←N` ahead, `→N` behind, `MERGED`, `ORPHAN`, `C` conflict |
| `Remote` | `↑N` to push, `↓N` to pull, `up to date`, `no-remote`                      |
| `PR`     | pull request status for the branch                                           |
| `Age`    | time since last commit                                                       |
