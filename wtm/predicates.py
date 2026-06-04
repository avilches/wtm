def can_pull(wt):
    r = wt.get("remote")
    return bool(r and r.get("behind", 0) > 0)


def can_push(wt):
    r = wt.get("remote")
    if not r:
        return True  # sin remote: permitir push para crearlo
    return r.get("ahead", 0) > 0


def can_rebase_or_merge(wt):
    return wt.get("main", {}).get("behind", 0) > 0


def can_delete(wt):
    return wt.get("main_state") != "is_main"


def is_orphan(wt):
    return wt.get("main_state") == "orphan"
