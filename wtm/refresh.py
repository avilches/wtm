import os
import subprocess
import threading
import time

_XPATH = "/opt/homebrew/bin:/opt/homebrew/sbin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin"
_event = threading.Event()
_started = False
_lock = threading.Lock()


def _make_env():
    e = os.environ.copy()
    e["PATH"] = _XPATH + ":" + e.get("PATH", "")
    return e


def start(root, interval=60):
    global _started
    with _lock:
        if _started:
            return
        _started = True

    def _worker():
        env = _make_env()
        while True:
            try:
                subprocess.run(
                    ["git", "-C", root, "fetch", "--all", "--quiet"],
                    capture_output=True, env=env,
                )
                _event.set()
            except Exception:
                pass
            time.sleep(interval)

    threading.Thread(target=_worker, daemon=True).start()


def pending():
    if _event.is_set():
        _event.clear()
        return True
    return False
