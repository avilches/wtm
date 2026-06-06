#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
DIM='\033[2m'
NC='\033[0m'

echo "Installing wtm from $SCRIPT_DIR"
echo ""

# 1. Make scripts executable
chmod +x "$SCRIPT_DIR/bin/wtm" "$SCRIPT_DIR/wtm.plugin.zsh" "$SCRIPT_DIR/wtm.plugin.bash"
echo -e "  ${GREEN}✓${NC} Scripts marked executable"

# Helper: adds a source line to a rc file if not already present
add_source_line() {
  local rcfile="$1" plugin="$2" shell_name="$3"
  if grep -q "wtm.plugin" "$rcfile" 2>/dev/null; then
    echo -e "  ${GREEN}✓${NC} source line already in $rcfile"
  else
    echo "" >> "$rcfile"
    echo "source \"$SCRIPT_DIR/$plugin\"" >> "$rcfile"
    echo -e "  ${GREEN}✓${NC} Added $shell_name source line to $rcfile"
  fi
}

# 2. zsh
if [[ -f "$HOME/.zshrc" ]]; then
  add_source_line "$HOME/.zshrc" "wtm.plugin.zsh" "zsh"
else
  echo -e "  ${YELLOW}!${NC}  ~/.zshrc not found, skipping zsh"
fi

# 3. bash — prefer .bashrc, fall back to .bash_profile
if [[ -f "$HOME/.bashrc" ]]; then
  add_source_line "$HOME/.bashrc" "wtm.plugin.bash" "bash"
elif [[ -f "$HOME/.bash_profile" ]]; then
  add_source_line "$HOME/.bash_profile" "wtm.plugin.bash" "bash"
else
  echo -e "  ${YELLOW}!${NC}  No bash config found (~/.bashrc or ~/.bash_profile), skipping bash"
fi

echo ""
echo -e "  Done. Reload your shell or run: ${DIM}source ~/.zshrc${NC}"
