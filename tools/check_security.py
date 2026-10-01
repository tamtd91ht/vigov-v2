#!/usr/bin/env python3
"""Rule 13 — the repo-wide half of the security baseline. Runs in `make check`, needs no network.

WHY TWO SHAPES: `.claude/hooks/security_guard.py` blocks a downgrade at the moment it is
TYPED, but a hook sees one edit and never re-reads what is already on disk. This answers the
other question — "does the whole repository hold the baseline TODAY" — the same split rule 5
(check_quyen.py) and rule 6 (check_audit_actor.py) needed.

WHAT IT CHECKS
  1. Every pattern security_guard knows, over every in-scope file. Imported, not copied: two
     pattern lists are two lists that drift, and the looser one is the one that runs.
  2. Every Next.js app sends the security headers TCVN 14423 §5.17.2.3 implies (CSP, HSTS,
     nosniff, Referrer-Policy, framing) — in its next.config or at the ingress.

THE DEBT LEDGER (tools/security_debt.json) IS A RATCHET, NOT AN ALLOW-LIST
  A violation already on disk when this gate was built is listed there with a reason, the TCVN
  clause and an expiry. The gate goes red when:
    · a violation is NOT listed               -> new debt; fix it or argue it into the ledger
    · a listed entry has EXPIRED              -> the promise to fix it was broken; say so
    · a listed entry matches NOTHING any more -> it was fixed; delete the line so it cannot
                                                 silently cover the next regression in that file
  The third rule is what keeps the ledger from becoming the "known issues" file nobody reads —
  an allow-list that only grows is a gate that has quietly switched itself off.

  A line marked `// @security-exception: debt=<label>` is a debt the user agreed to AT the line:
  it is not listed as a violation, but it counts as "seen" for entry (file, <label>) — so that
  entry expires like any other and is never reported FIXED while the marker stands. A marker
  whose entry does not exist -> MISSING (red).
"""

from __future__ import annotations

import datetime
import json
import os
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
sys.path.insert(0, os.path.join(ROOT, ".claude", "hooks"))
import security_guard as sg  # noqa: E402

DEBT_FILE = os.path.join(ROOT, "tools", "security_debt.json")

# Directories that are not source this repository writes.
SKIP_DIRS = {".git", "node_modules", ".next", "dist", "build", "gen", "vendor", ".codegraph",
             "kb", "docs", "rule", "tasks", "__pycache__", ".claude"}

REQUIRED_HEADERS = ("Content-Security-Policy", "Strict-Transport-Security",
                    "X-Content-Type-Options", "Referrer-Policy")
# Framing may be refused either way; CSP `frame-ancestors` is the modern one.
FRAMING = ("X-Frame-Options", "frame-ancestors")
INGRESS = os.path.join(ROOT, "deploy", "base", "mang", "ingress.yaml")

# citizen-app is NOT checked for headers, deliberately: it is a Zalo Mini App whose pages are
# served by Zalo's own host (ADR 0044), so no file in this repository can set its headers.
HEADERLESS_BY_DESIGN = {"citizen-app"}


def rd(p: str) -> str:
    try:
        with open(p, encoding="utf-8", errors="ignore") as f:
            return f.read()
    except OSError:
        return ""


def source_files() -> list[str]:
    """Files git tracks — what ships — plus untracked files that are not ignored.

    NOT a bare directory walk: measured on the first run, a walk found a gitignored `tmp/`
    preview bundle on one workstation, so the gate's colour depended on which machine ran it.
    Untracked-but-not-ignored files ARE included, because that is a new file somebody is about
    to commit, and the gate exists to catch it before it lands.
    """
    import subprocess
    try:
        r = subprocess.run(["git", "ls-files", "--cached", "--others", "--exclude-standard", "-z"],
                           cwd=ROOT, capture_output=True, check=True)
        files = [p for p in r.stdout.decode("utf-8", "replace").split("\0") if p]
    except (OSError, subprocess.CalledProcessError):
        files = []
        for dp, dns, fns in os.walk(ROOT):
            dns[:] = [d for d in dns if d not in SKIP_DIRS]
            files += [os.path.relpath(os.path.join(dp, f), ROOT) for f in fns]
    return [f.replace("\\", "/") for f in files
            if f.replace("\\", "/").split("/", 1)[0] not in SKIP_DIRS]


def pattern_violations() -> tuple[list[tuple[str, int, str]], list[tuple[str, int, str]]]:
    """(unexcused violations, debt-linked markers) as [(file, line, label)].

    For a marker the label is the DEBT label it names (`@security-exception: debt=<label>`), the
    key its ledger entry is matched on.
    """
    out: list[tuple[str, int, str]] = []
    marked: list[tuple[str, int, str]] = []
    for rel in source_files():
        if not sg.in_scope(rel) or not os.path.isfile(os.path.join(ROOT, rel)):
            continue
        text = rd(os.path.join(ROOT, rel))
        for line, label in sg.quet(text, rel):
            out.append((rel, line, label))
        for line, _pattern, debt in sg.scan_debt_markers(text, rel):
            marked.append((rel, line, debt))
    return out, marked


def header_violations() -> list[tuple[str, int, str]]:
    ingress = rd(INGRESS)

    def complete(text: str) -> bool:
        return all(h in text for h in REQUIRED_HEADERS) and any(f in text for f in FRAMING)

    out: list[tuple[str, int, str]] = []
    for app in sorted(os.listdir(ROOT)):
        pkg = os.path.join(ROOT, app, "package.json")
        if app in HEADERLESS_BY_DESIGN or not os.path.isfile(pkg):
            continue
        try:
            deps = json.loads(rd(pkg)).get("dependencies", {})
        except ValueError:
            deps = {}
        if "next" not in deps:
            continue
        conf = "".join(rd(os.path.join(ROOT, app, f"next.config.{x}")) for x in ("ts", "js", "mjs"))
        if not (complete(conf) or complete(ingress)):
            out.append((app, 0, "missing-security-headers"))
    return out


def judge(found: list[tuple[str, int, str]], marked: list[tuple[str, int, str]],
          debt: list[dict], today: str) -> list[str]:
    """The pure half: every problem line for these findings against this ledger. No I/O.

    `marked` are debt-linked exceptions (`@security-exception: debt=<label>`). They are NOT
    violations to list, but they DO keep their entry alive and DO expire with it — a plain
    marker would make the entry look FIXED and leave its date unwatched. A marker naming no entry
    is red: the promise it points at was never written down.
    """
    listed = {(d.get("file", ""), d.get("label", "")): d for d in debt}

    new = [(f, ln, lb) for f, ln, lb in found if (f, lb) not in listed]
    missing = [(f, ln, lb) for f, ln, lb in marked if (f, lb) not in listed]
    seen = {(f, lb) for f, _, lb in found} | {(f, lb) for f, _, lb in marked}
    stale = [k for k in listed if k not in seen]
    expired = [(k, d.get("expires", "")) for k, d in listed.items()
               if k in seen and str(d.get("expires", "")) < today]
    no_reason = [k for k, d in listed.items() if not d.get("reason") or not d.get("expires")]

    problems: list[str] = []
    problems += [f"NEW      {f}:{ln} {lb}" for f, ln, lb in new]
    problems += [f"MISSING  {f}:{ln} @security-exception: debt={lb} — no such entry in "
                 f"tools/security_debt.json (same file + label)" for f, ln, lb in missing]
    problems += [f"EXPIRED  {f} {lb} (expires {ex})" for (f, lb), ex in expired]
    problems += [f"FIXED    {f} {lb} — delete this entry from tools/security_debt.json"
                 for f, lb in stale]
    problems += [f"INVALID  {f} {lb} — an entry needs a reason and an expiry" for f, lb in no_reason]
    return problems


def main() -> int:
    for s in (sys.stdout, sys.stderr):
        try:
            s.reconfigure(encoding="utf-8", errors="replace")
        except Exception:
            pass

    try:
        debt = json.loads(rd(DEBT_FILE)).get("debt", [])
    except ValueError as e:
        print(f"[FAIL] tools/security_debt.json is not valid JSON: {e}")
        return 1

    today = datetime.date.today().isoformat()
    patterns, marked = pattern_violations()
    found = patterns + header_violations()
    problems = judge(found, marked, debt, today)

    if problems:
        print(f"[FAIL] security baseline (rule 13) — {len(problems)} problem(s)")
        for p in problems:
            print(f"        {p}")
        print("        → Rule 13: .claude/rules/critical/13-security-baseline.md")
        return 1

    print(f"[PASS] security baseline (rule 13) — {len(found)} known violation(s) + "
          f"{len({(f, lb) for f, _, lb in marked})} debt-linked exception(s), "
          f"all in the debt ledger and unexpired · 0 new")
    return 0


if __name__ == "__main__":
    sys.exit(main())
