#!/usr/bin/env bash
# wtm.plugin.bash — source this from ~/.bashrc or ~/.bash_profile:
#   source /path/to/wtm/wtm.plugin.bash

_WTM_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

wtm() {
  local _tmp _path
  _tmp=$(mktemp)
  "$_WTM_DIR/bin/wtm" "$_tmp"
  _path=$(<"$_tmp" 2>/dev/null)
  rm -f "$_tmp"
  [[ -n "$_path" ]] && cd "$_path"
}
