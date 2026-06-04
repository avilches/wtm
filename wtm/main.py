import json
import sys
import time

from .actions import _load_data, _handle_action
from .renderer import col_widths, make_header, build_line
from .picker import run_picker
from .colors import BOLD, RESET


def main():
    cdfile = sys.argv[1] if len(sys.argv) > 1 and not sys.argv[1].startswith("-") else None
    pick_mode = "--pick" in sys.argv or cdfile is not None

    if not sys.stdin.isatty():
        data = json.loads(sys.stdin.read())
    else:
        data = _load_data()

    now = time.time()
    root = next((wt.get("path", "") for wt in data if wt.get("main_state") == "is_main"), "")
    W = col_widths(data, now)
    header = make_header(W)

    if not pick_mode:
        print(f"{BOLD}{header}{RESET}")
        print("  " + "-" * (len(header) - 2))
        for wt in data:
            print(build_line(wt, now, W))
        return

    select_path = next((a[9:] for a in sys.argv if a.startswith("--select=")), None)

    while True:
        now = time.time()
        root = next((wt.get("path", "") for wt in data if wt.get("main_state") == "is_main"), "")
        W = col_widths(data, now)
        header = make_header(W)

        result = run_picker(data, header, now, W, root=root, select_path=select_path)
        select_path = None
        if result is None:
            break

        action, _, path = result.partition(":")
        cont, select_path = _handle_action(action, path, cdfile, data)
        if not cont:
            break

        data = _load_data()
