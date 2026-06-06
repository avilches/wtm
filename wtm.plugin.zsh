#!/usr/bin/env zsh
# wtm.plugin.zsh — source this from ~/.zshrc:
#   source /path/to/wtm/wtm.plugin.zsh

_WTM_DIR="${0:A:h}"

function wtm() {
  local _tmp; _tmp=$(mktemp)
  "$_WTM_DIR/bin/wtm" "$_tmp"
  local _path; _path=$(<"$_tmp" 2>/dev/null)
  rm -f "$_tmp"
  [[ -n "$_path" ]] && cd "$_path"
}
