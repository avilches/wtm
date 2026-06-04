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
chmod +x "$SCRIPT_DIR/bin/wtm" "$SCRIPT_DIR/wtm.py" "$SCRIPT_DIR/wtm.plugin.zsh"
echo -e "  ${GREEN}✓${NC} Scripts marked executable"

# 2. Ensure source line is in ~/.zshrc
ZSHRC="$HOME/.zshrc"
if grep -q "wtm.plugin.zsh" "$ZSHRC" 2>/dev/null; then
    echo -e "  ${GREEN}✓${NC} source line already in $ZSHRC"
else
    echo "" >> "$ZSHRC"
    echo "source \"\${WTM_DIR:-$SCRIPT_DIR}/wtm.plugin.zsh\"" >> "$ZSHRC"
    echo -e "  ${GREEN}✓${NC} Added source line to $ZSHRC"
fi

# 3. If wtm is not at the default path encoded in .zshrc, set WTM_DIR in .zprofile.local
DEFAULT_PATH=$(grep "wtm.plugin.zsh" "$ZSHRC" | grep -o '".*wtm"' | tr -d '"' | head -1)
if [[ -n "$DEFAULT_PATH" && "$DEFAULT_PATH" != "$SCRIPT_DIR" ]]; then
    ZPROFILE_LOCAL="$HOME/.zprofile.local"
    if grep -q "WTM_DIR" "$ZPROFILE_LOCAL" 2>/dev/null; then
        sed -i '' "s|export WTM_DIR=.*|export WTM_DIR=\"$SCRIPT_DIR\"|" "$ZPROFILE_LOCAL"
        echo -e "  ${GREEN}✓${NC} Updated WTM_DIR in $ZPROFILE_LOCAL"
    else
        echo "export WTM_DIR=\"$SCRIPT_DIR\"" >> "$ZPROFILE_LOCAL"
        echo -e "  ${GREEN}✓${NC} Added WTM_DIR to $ZPROFILE_LOCAL"
    fi
    echo -e "  ${YELLOW}!${NC}  Path differs from .zshrc default — WTM_DIR set in .zprofile.local"
fi

echo ""
echo -e "  Done. Run: ${DIM}source ~/.zshrc${NC}"
