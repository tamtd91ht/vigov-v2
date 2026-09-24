---
description: Verify the brain's hooks — each one gets a must-block case and a must-pass case
group: Bộ não
allowed-tools: Read, Bash, Glob
---

# /check-hooks

Hooks are the **only** enforcement layer of this brain. A hook with no test **may already be
dead**: one syntax error, one broken pattern, and the protection is off in silence —
everything still looks normal, it just stops blocking.

## Steps

1. `python tools/test_hooks.py`
2. Cross-check: every hook in `.claude/settings.json` has **≥ 1 block case and ≥ 1 pass case**
3. A hook missing a case → report it, and **propose the concrete case**, not just the gap
4. A failing case → diagnose **which pattern** is wrong, fix the pattern, run again

## Report

| Hook | Block cases | Pass cases | Result |
|---|---|---|---|

End with one line: `N cases · passed M · failed K`.

**A "should pass but was blocked" failure is more serious than the reverse** — false
positives make people disable the hook, and then the whole protection layer is gone, which
is worse than never having had it.
