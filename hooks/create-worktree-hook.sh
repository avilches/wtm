#!/usr/bin/env bash
set -euo pipefail

WORKTREE_PATH="${1:?Usage: create-worktree-hook.sh <worktree-path> <worktree-name>}"
WT_NAME="${2:?Usage: create-worktree-hook.sh <worktree-path> <worktree-name>}"

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
DIM='\033[2m'
NC='\033[0m'

# ── Sync settings (.claude, .idea, .run) ─────────────────────────────────────
if command -v sync-settings >/dev/null 2>&1; then
    echo -e "  ${YELLOW}->  ${NC}Syncing settings..."
    if sync-settings --path "$WORKTREE_PATH" 2>/dev/null; then
        echo -e "  ${GREEN}✓${NC} Settings synced"
    else
        echo -e "  ${DIM}(sync-settings: no changes)${NC}"
    fi
    echo
fi

# ── Patch .idea/.name ────────────────────────────────────────────────────────
idea_name_file="$WORKTREE_PATH/.idea/.name"
if [[ -f "$idea_name_file" ]]; then
    base_name=$(cat "$idea_name_file")
    printf '%s:%s' "$base_name" "$WT_NAME" > "$idea_name_file"
    echo -e "  ${GREEN}✓${NC} .idea/.name: ${CYAN}${base_name}:${WT_NAME}${NC}"
    echo
fi
