"""SessionStart — warn when the CODE is silently deciding a customer's open question.

WHY THIS HOOK EXISTS — the signature, and most expensive, failure mode of a 100%-AI project:

    The agent breaks NO rule. It simply does not notice that a chain of local decisions,
    each reasonable on its own, is adding up to an ARCHITECTURAL DECISION it has no
    authority to make.

This happened on ViGov v1. "One commune or many" was open question #8, the customer had not
decided, and the estimate said "real multi-tenant: +1-2 days". The brain even had a rule:
"never decide the customer's open questions". But that rule was one sentence of prose with
nothing checking it. After 103 commits the code had decided — SINGLE-COMMUNE, in 12 places:
12 global unique keys, commune name from an environment variable, zero occurrences of
tenant_id. Cost to reverse once found: 20-28 days.

Every line was innocent. The sum had already decided. This hook makes the sum VISIBLE.

Source of truth: kb/00-foundation/open-questions.json
"""

from __future__ import annotations

import json
import os
import re
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import _common as c  # noqa: E402

HOOK = "drift_guard"

SCAN_EXT = (".go", ".ts", ".tsx", ".sql", ".proto")

# SCAN BY EXCLUSION, NOT BY WHITELIST — and that choice is the whole fix here.
#
# This hook shipped with SCAN_DIRS = ("services", "web", "internal", "pkg", "cmd", "proto",
# "migrations"). After the flat layout (ADR 0015/0016) exactly ONE of those seven exists at the
# repo root: proto/. Every service, core/, every web app and every migrations/ sat outside the
# walk, so the hook whose entire job is to notice the code deciding something read almost no
# code — and check_brain stayed green, because it verifies that each rule NAMES a hook, not
# that the hook can SEE anything.
#
# A whitelist of directory names is the wrong shape for a repository whose organising decision
# is "one flat directory per deployable unit": the next unit added is silently unscanned, and
# nothing goes red on the day it happens. An exclusion list fails the other way — a new source
# directory is scanned until somebody deliberately excludes it, and over-scanning costs a few
# milliseconds while under-scanning costs the failure this hook exists to prevent.
SKIP_DIRS = frozenset((
    ".git", ".claude", ".github", "node_modules", "vendor", "dist", "build", "testdata",
    "kb",       # documentation — statements ABOUT decisions, not the code making them
    "docs",     # ditto: the UI/UX specification is not an implementation
    "tasks",    # the web work queue is JSON status, not source
    "gen",      # generated from .proto — generated code decides nothing
))

MAX_FILES = 4000          # safety cap — this hook must not slow session start


def iter_files(root: str):
    """Yield source files, and report whether the cap truncated the walk.

    The cap is returned rather than applied silently: a hook that quietly stops looking after
    N files reports "no drift" for a repository it only partly read, which is the same class of
    false confidence the whitelist above produced.
    """
    n = 0
    for dp, dn, fn in os.walk(root):
        dn[:] = [x for x in dn if x not in SKIP_DIRS]
        for f in fn:
            if not f.endswith(SCAN_EXT):
                continue
            if f.endswith("_test.go") or f.endswith(".pb.go"):
                continue
            yield os.path.join(dp, f)
            n += 1
            if n >= MAX_FILES:
                print(f"[ViGov] drift_guard: đã đọc {MAX_FILES} tệp và DỪNG — "
                      f"kết quả dưới đây chỉ tính trên phần đã đọc.")
                return


def load_questions(root: str) -> list:
    p = os.path.join(root, "kb", "00-foundation", "open-questions.json")
    try:
        with open(p, encoding="utf-8") as f:
            data = json.load(f)
        return data if isinstance(data, list) else []
    except Exception:
        return []


def main() -> None:
    c.utf8_streams()
    c.read_input()
    root = c.project_root()

    questions = [q for q in load_questions(root)
                 if q.get("status") in ("OPEN", "SILENTLY_DECIDED")]
    if not questions:
        sys.exit(0)

    # Compile once, scan files ONCE — never rescan per question
    # Each signal carries its OWN threshold, and honouring it is the second half of this fix.
    # The reporting rule below used a flat `top_n < 3`, so every `"threshold": 1` written in
    # open-questions.json was dead text — and threshold 1 is exactly what the expensive
    # questions declare, because for them ONE occurrence already is the decision (one
    # `tenant_id := "xa-tan-phu"` decides that the identifier carries meaning; a second one
    # adds nothing).
    probes = []
    for qi, q in enumerate(questions):
        for sig in q.get("code_signals", []):
            try:
                probes.append((qi, re.compile(sig["pattern"]), sig.get("means", "?"),
                               int(sig.get("threshold", 3))))
            except Exception:
                continue
    if not probes:
        sys.exit(0)

    # means -> threshold, per question. Declared once here so the reporting loop reads it
    # rather than re-deriving it.
    nguong: dict = {}
    for qi, _pat, means, thr in probes:
        nguong.setdefault(qi, {})[means] = min(nguong.get(qi, {}).get(means, thr), thr)

    tally: dict = {}
    for path in iter_files(root):
        try:
            with open(path, encoding="utf-8", errors="ignore") as f:
                text = f.read()
        except Exception:
            continue
        for qi, pat, means, _thr in probes:
            n = len(pat.findall(text))
            if n:
                tally.setdefault(qi, {}).setdefault(means, 0)
                tally[qi][means] += n

    lines = []
    for qi, q in enumerate(questions):
        counts = tally.get(qi)
        if not counts:
            continue
        # Report only when ONE direction dominates and the other is nearly absent.
        #
        # The dominance test stays: two directions present in comparable numbers means the code
        # is inconsistent, not decided, and a warning there would cry wolf on every session.
        # What changed is the count it is measured against — the signal's own threshold rather
        # than a flat 3.
        ranked = sorted(counts.items(), key=lambda kv: -kv[1])
        top, top_n = ranked[0]
        rest = sum(v for _, v in ranked[1:])
        if top_n < nguong.get(qi, {}).get(top, 3) or rest * 3 > top_n:
            continue
        detail = " · ".join(f"{k}: {v} places" for k, v in ranked)
        lines += [
            f"[ViGov] WARNING: open question #{q.get('id','?')} \"{q.get('question','')}\" "
            f"is being SILENTLY DECIDED toward: {top}.",
            f"        Signals: {detail}. The customer has NOT decided.",
            f"        Reversal cost: {q.get('reversal_cost','grows over time')}.",
            f"        -> Raise it with the user BEFORE: "
            f"{', '.join(q.get('ask_before', ['adding a new entity']))}",
        ]

    if lines:
        print("\n" + "\n".join(lines) + "\n")
        c.log_guard(HOOK, "SessionStart", "", "open question being silently decided", len(lines) // 4)

    sys.exit(0)


if __name__ == "__main__":
    main()
