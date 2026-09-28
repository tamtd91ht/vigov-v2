#!/usr/bin/env python3
"""Rule 13 — known vulnerabilities in third-party code. `make vuln`; NEEDS THE NETWORK.

TCVN 14423:2026 §5.17.2.4: check the vulnerabilities of the source code AND of the third-party
libraries an application uses before it goes into operation.

WHY THIS IS NOT PART OF `make check` (decided by the project owner, 2026-09-28, option B):
both scanners fetch a vulnerability database on every run. Inside `make check` that means a
machine without Internet goes red for a reason unrelated to the change, and a commit that was
green yesterday goes red today because an advisory was published overnight — blocking a
session that is fixing something else. So it runs as its own gate: mandatory in the Jenkins
`vigov-gate` job, and nightly on the same job.

IT NEVER SKIPS. No network, a scanner that cannot start, output that does not parse — each is
RED with the reason printed. "Offline, so skipped" is the shape this repository keeps finding
in its own gates: a check that looks armed and has quietly stopped checking.

WHAT COUNTS
  Go   — govulncheck findings whose trace reaches a FUNCTION: code this repository actually
         calls. Module-level matches with no call path are printed, not failed — that is
         govulncheck's own default, and failing on them would bury the real ones.
  npm  — advisories of severity high or critical in production dependencies (--omit=dev).

EXCEPTIONS (tools/vuln_exceptions.json) follow the same ratchet as tools/security_debt.json:
an entry needs a reason, a mitigation and an expiry (TCVN §5.3.2.3b asks for exactly that list
of tolerated software, "with the controls that reduce the risk"); an expired entry is red; an
entry that no longer matches anything is red until deleted.
"""

from __future__ import annotations

import datetime
import json
import os
import shlex
import subprocess
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
EXCEPTIONS_FILE = os.path.join(ROOT, "tools", "vuln_exceptions.json")
NPM_FAIL = {"high", "critical"}


def rd(p: str) -> str:
    try:
        with open(p, encoding="utf-8") as f:
            return f.read()
    except OSError:
        return ""


def json_stream(text: str) -> list:
    """govulncheck -format json prints CONCATENATED objects, not one array."""
    dec, i, out = json.JSONDecoder(), 0, []
    while True:
        while i < len(text) and text[i].isspace():
            i += 1
        if i >= len(text):
            return out
        obj, i = dec.raw_decode(text, i)
        out.append(obj)


def go_modules() -> list[str]:
    r = subprocess.run(["go", "list", "-m", "-f", "{{.Dir}}"], cwd=ROOT,
                       capture_output=True, text=True, check=True)
    return [d for d in r.stdout.split() if d]


def scan_go(cmd: list[str]) -> tuple[list[tuple[str, str]], list[str]]:
    """[(scope, GO-id)] called findings, and error lines."""
    found: list[tuple[str, str]] = []
    errors: list[str] = []
    for mod in go_modules():
        scope = os.path.relpath(mod, ROOT).replace("\\", "/")
        try:
            r = subprocess.run(cmd + ["-format", "json", "./..."], cwd=mod,
                               capture_output=True, text=True, encoding="utf-8")
        except OSError as e:
            errors.append(f"{scope}: govulncheck could not start ({e})")
            continue
        try:
            msgs = json_stream(r.stdout)
        except ValueError:
            msgs = []
        if r.returncode != 0 or not any("config" in m for m in msgs):
            tail = (r.stderr or r.stdout).strip().splitlines()[-3:]
            errors.append(f"{scope}: govulncheck failed (rc={r.returncode}) — "
                          + " | ".join(tail))
            continue
        for m in msgs:
            f = m.get("finding")
            if not f:
                continue
            trace = f.get("trace") or [{}]
            if trace[0].get("function"):
                found.append((scope, f.get("osv", "?")))
            else:
                print(f"  note {scope}: {f.get('osv')} in a required module, not called")
    return sorted(set(found)), errors


def scan_npm() -> tuple[list[tuple[str, str]], list[str]]:
    found: list[tuple[str, str]] = []
    errors: list[str] = []
    for app in sorted(os.listdir(ROOT)):
        if not os.path.isfile(os.path.join(ROOT, app, "package-lock.json")):
            continue
        try:
            r = subprocess.run("npm audit --json --omit=dev", cwd=os.path.join(ROOT, app),
                               capture_output=True, text=True, encoding="utf-8", shell=True)
        except OSError as e:
            errors.append(f"{app}: npm audit could not start ({e})")
            continue
        try:
            rep = json.loads(r.stdout or "")
        except ValueError:
            rep = None
        if not isinstance(rep, dict) or "error" in rep or "vulnerabilities" not in rep:
            why = (rep or {}).get("error", {}) if isinstance(rep, dict) else {}
            msg = why.get("summary") if isinstance(why, dict) else None
            errors.append(f"{app}: npm audit gave no report — "
                          + (msg or (r.stderr.strip().splitlines() or ["no output"])[-1]))
            continue
        for pkg in rep["vulnerabilities"].values():
            for via in pkg.get("via", []):
                if isinstance(via, dict) and via.get("severity") in NPM_FAIL:
                    adv = (via.get("url") or "").rstrip("/").rsplit("/", 1)[-1] \
                        or str(via.get("source"))
                    found.append((app, f"{adv} ({via.get('name')}, {via.get('severity')})"))
    return sorted(set(found)), errors


def main() -> int:
    for s in (sys.stdout, sys.stderr):
        try:
            s.reconfigure(encoding="utf-8", errors="replace")
        except Exception:
            pass

    govuln = shlex.split(sys.argv[1]) if len(sys.argv) > 1 else ["govulncheck"]
    try:
        exc = json.loads(rd(EXCEPTIONS_FILE)).get("exceptions", [])
    except ValueError as e:
        print(f"[FAIL] tools/vuln_exceptions.json is not valid JSON: {e}")
        return 1

    go_found, go_err = scan_go(govuln)
    npm_found, npm_err = scan_npm()
    found = go_found + npm_found

    def adv_id(entry: str) -> str:
        return entry.split(" ", 1)[0]

    def covered(scope: str, entry: str) -> dict | None:
        for x in exc:
            if x.get("id") == adv_id(entry) and x.get("scope") in (scope, "*"):
                return x
        return None

    today = datetime.date.today().isoformat()
    problems = [f"ERROR    {e}" for e in go_err + npm_err]
    used: set[int] = set()
    for scope, entry in found:
        x = covered(scope, entry)
        if x is None:
            problems.append(f"VULN     {scope}: {entry}")
            continue
        used.add(id(x))
        if str(x.get("expires", "")) < today:
            problems.append(f"EXPIRED  {scope}: {entry} (exception expired {x.get('expires')})")
    for x in exc:
        if not (x.get("reason") and x.get("mitigation") and x.get("expires")):
            problems.append(f"INVALID  {x.get('id')} — needs reason, mitigation and expires")
        elif id(x) not in used and not (go_err or npm_err):
            problems.append(f"FIXED    {x.get('id')} ({x.get('scope')}) — delete this exception")

    if problems:
        print(f"[FAIL] third-party vulnerabilities (rule 13) — {len(problems)} problem(s)")
        for p in problems:
            print(f"        {p}")
        print("        Fix: upgrade the module/package; if impossible, add a dated exception with a")
        print("        mitigation to tools/vuln_exceptions.json — the user sees it in the commit.")
        return 1
    print(f"[PASS] third-party vulnerabilities (rule 13) — {len(go_found)} Go · "
          f"{len(npm_found)} npm finding(s), all under unexpired exceptions")
    return 0


if __name__ == "__main__":
    sys.exit(main())
