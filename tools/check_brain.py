#!/usr/bin/env python3
"""Check the 6 structural invariants of the .claude brain — anti-drift.

WHY: a brain drifting away from reality is something that WILL happen, not something that
might. Keeping dozens of Markdown files in sync by human discipline does not scale — an
earlier ViGov project once had a dedicated commit cleaning up traces of a dropped module
across 31 files, and still missed 11. It has to be machine-checked.

Run:  python tools/check_brain.py     (or /check-brain)
Exit code: 0 = all passed, 1 = an invariant failed.
"""

from __future__ import annotations

import io
import json
import os
import re
import sys

for _s in (sys.stdout, sys.stderr):
    try:
        _s.reconfigure(encoding="utf-8", errors="replace")
    except Exception:
        pass

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
CLAUDE = os.path.join(ROOT, ".claude")
TOKEN_BUDGET = 20000

fails: list[str] = []


def rd(p: str) -> str:
    try:
        return io.open(p, encoding="utf-8", errors="ignore").read()
    except Exception:
        return ""


def est_tokens(s: str) -> int:
    # Rough estimate: accented Vietnamese is ~2.2 chars per token
    return int(len(s) / 2.2)


def report(ok: bool, name: str, numbers: str, detail: list[str] | None = None) -> None:
    print(f"[{'PASS' if ok else 'FAIL'}] {name} — {numbers}")
    if not ok:
        fails.append(name)
        for d in (detail or [])[:10]:
            print(f"        {d}")


# ---- 1. Every rule has exactly one hook, and vice versa ------------------
rules = sorted(f for f in os.listdir(os.path.join(CLAUDE, "rules", "critical"))
               if f.endswith(".md"))
settings = json.loads(rd(os.path.join(CLAUDE, "settings.json")))
registered = set()
for ev in settings.get("hooks", {}).values():
    for grp in ev:
        for h in grp.get("hooks", []):
            m = re.search(r"hooks/([a-z_]+)\.py", h.get("command", ""))
            if m:
                registered.add(m.group(1))

on_disk = {f[:-3] for f in os.listdir(os.path.join(CLAUDE, "hooks"))
            if f.endswith(".py") and not f.startswith("_")}

unregistered = sorted(on_disk - registered)
missing_file = sorted(registered - on_disk)

# Every rule file must point at a hook that exists
rule_hook_gaps = []
for r in rules:
    body = rd(os.path.join(CLAUDE, "rules", "critical", r))
    hooks_in = re.findall(r"hooks/([a-z_]+)\.py", body)
    if not hooks_in:
        rule_hook_gaps.append(f"{r}: points at no hook")
    for h in hooks_in:
        if h not in on_disk:
            rule_hook_gaps.append(f"{r}: points at a hook that does not exist: '{h}'")

report(len(rules) == 9 and not unregistered and not missing_file and not rule_hook_gaps,
    "1. Rules <-> hooks, one to one",
    f"{len(rules)} rules · {len(on_disk)} hooks on disk · {len(registered)} registered",
    rule_hook_gaps + [f"hook not registered: {x}" for x in unregistered]
    + [f"dang ky nhung khong co file: {x}" for x in missing_file])


# ---- 2. Every hook has >=1 block case and >=1 pass case ------------------
tests = rd(os.path.join(ROOT, "tools", "test_hooks.py"))
cases = re.findall(r'\("([a-z_]+)",\s*"[^"]*",\s*(BLOCK|PASS)', tests)
tally: dict[str, dict[str, int]] = {}
for hook, kind in cases:
    tally.setdefault(hook, {"BLOCK": 0, "PASS": 0})[kind] += 1

# session_start / drift_guard / stop_verify_guard need a real environment — smoke-tested instead
EXEMPT = {"session_start", "drift_guard", "stop_verify_guard"}
gaps = []
for h in sorted(on_disk - EXEMPT):
    d = tally.get(h, {"BLOCK": 0, "PASS": 0})
    if d["BLOCK"] < 1:
        gaps.append(f"{h}: no BLOCK case")
    if d["PASS"] < 1:
        gaps.append(f"{h}: no PASS case")

report(not gaps, "2. Every hook has a block case and a pass case",
    f"{len(cases)} cases across {len(tally)} hooks ({len(EXEMPT)} exempt: need a real environment)",
    gaps)


# ---- 3. Every path referenced under .claude/** exists --------------------
PATH_RE = re.compile(r"`((?:\.claude|kb|tools|services|web|deploy)/[A-Za-z0-9_\-./]+"
                     r"\.(?:md|py|json|yaml|yml|go|ts|tsx|proto))`")
dead = []
refs = 0
for dp, _, fn in os.walk(CLAUDE):
    if "logs" in dp:
        continue
    for f in fn:
        if not f.endswith(".md"):
            continue
        for m in PATH_RE.findall(rd(os.path.join(dp, f))):
            refs += 1
            if not os.path.exists(os.path.join(ROOT, m.replace("/", os.sep))):
                dead.append(f"{os.path.relpath(os.path.join(dp, f), ROOT)} -> {m}")

# Skills are referenced by directory name
SKILL_RE = re.compile(r"`?skills/([a-z0-9\-]+)`?")
for dp, _, fn in os.walk(CLAUDE):
    for f in fn:
        if not f.endswith(".md"):
            continue
        for s in set(SKILL_RE.findall(rd(os.path.join(dp, f)))):
            refs += 1
            if not os.path.isdir(os.path.join(CLAUDE, "skills", s)):
                dead.append(f"{os.path.relpath(os.path.join(dp, f), ROOT)} -> skills/{s}")

# Commands are referenced as /<name> — a dead one here is invisible until somebody types it
CMD_RE = re.compile(r"`/([a-z][a-z0-9\-]{2,})`")
known_cmds = {f[:-3] for f in os.listdir(os.path.join(CLAUDE, "commands")) if f.endswith(".md")}
for dp, _, fn in os.walk(CLAUDE):
    if "logs" in dp:
        continue
    for f in fn:
        if not f.endswith(".md"):
            continue
        for cmd in set(CMD_RE.findall(rd(os.path.join(dp, f)))):
            refs += 1
            if cmd not in known_cmds:
                dead.append(f"{os.path.relpath(os.path.join(dp, f), ROOT)} -> /{cmd}")

report(not dead, "3. Every reference resolves",
    f"{refs} references · {len(dead)} dead", dead)


# ---- 4. Frontmatter is valid --------------------------------------------
fm_gaps = []
for d in os.listdir(os.path.join(CLAUDE, "skills")):
    p = os.path.join(CLAUDE, "skills", d, "SKILL.md")
    if not os.path.exists(p):
        fm_gaps.append(f"skills/{d}: no SKILL.md")
        continue
    head = rd(p)[:1500]
    if not re.search(r"\Aname:|^name:", head, re.M) or not re.search(r"^description:", head, re.M):
        fm_gaps.append(f"skills/{d}: missing name or description")

for f in os.listdir(os.path.join(CLAUDE, "commands")):
    if not f.endswith(".md"):
        continue
    head = rd(os.path.join(CLAUDE, "commands", f))[:1200]
    if not re.search(r"^description:", head, re.M):
        fm_gaps.append(f"commands/{f}: missing description")

# Agents, excluding ROUTING.md — it is a decision table, not an agent definition
AGENT_DIR = os.path.join(CLAUDE, "agents")
agent_files = sorted(f for f in os.listdir(AGENT_DIR)
                     if f.endswith(".md") and f != "ROUTING.md")
for f in agent_files:
    head = rd(os.path.join(AGENT_DIR, f))[:1500]
    if not re.search(r"^name:", head, re.M) or not re.search(r"^description:", head, re.M):
        fm_gaps.append(f"agents/{f}: missing name or description")

n_sk = len([d for d in os.listdir(os.path.join(CLAUDE, "skills"))])
n_cm = len([f for f in os.listdir(os.path.join(CLAUDE, "commands")) if f.endswith(".md")])
report(not fm_gaps, "4. Frontmatter is valid",
    f"{n_sk} skills · {n_cm} commands · {len(agent_files)} agents", fm_gaps)


# ---- 5. Always-loaded budget ---------------------------------------------
total = est_tokens(rd(os.path.join(ROOT, "CLAUDE.md")))
chi_tiet = [f"CLAUDE.md: ~{total}"]
for r in rules:
    n = est_tokens(rd(os.path.join(CLAUDE, "rules", "critical", r)))
    total += n
idx = rd(os.path.join(ROOT, "kb", "INDEX.yaml"))
total += est_tokens(idx)
for m in re.findall(r"^\s+-\s+(kb/[^\s]+)", idx, re.M):
    total += est_tokens(rd(os.path.join(ROOT, m.replace("/", os.sep))))

report(total <= TOKEN_BUDGET, "5. Always-loaded budget",
    f"~{total} / {TOKEN_BUDGET} tokens ({total*100//TOKEN_BUDGET}%)",
    ["over budget — REMOVE something; never raise the ceiling"])


# ---- 6. No .md outside kb/, services/*/README.md, .claude/ ---------------
ALLOWED = (
    re.compile(r"^kb[/\\]"), re.compile(r"^\.claude[/\\]"),
    re.compile(r"^services[/\\][a-z0-9_\-]+[/\\]README\.md$"),
    re.compile(r"^apps[/\\][a-z0-9_\-]+[/\\]README\.md$"),
    re.compile(r"^(README|CLAUDE)\.md$"),
)
stray = []
for dp, dn, fn in os.walk(ROOT):
    dn[:] = [d for d in dn if d not in ("node_modules", ".git", "vendor", "dist", "__pycache__")]
    for f in fn:
        if not f.endswith(".md"):
            continue
        rel = os.path.relpath(os.path.join(dp, f), ROOT)
        if not any(p.match(rel) for p in ALLOWED):
            stray.append(rel)

report(not stray, "6. No documentation in the wrong place", f"{len(stray)} stray files", stray)


# ---- 7. ROUTING.md and the agent files describe the same set ------------
# WHY: v1 dropped a module and still had 11 stale references after a dedicated cleanup
# commit. A routing map that names an agent which no longer exists sends the session
# nowhere; an agent absent from the map is one nobody will ever reach.
routing = rd(os.path.join(AGENT_DIR, "ROUTING.md"))
on_disk_agents = {f[:-3] for f in agent_files}
# Names in ROUTING that look like an agent handle
looks_agent = set(re.findall(r"`([a-z][a-z0-9]*(?:-[a-z0-9]+)*-(?:builder|designer|reviewer|keeper|expert))`", routing))
routing_gaps = []
for a in sorted(looks_agent - on_disk_agents):
    routing_gaps.append(f"ROUTING.md names '{a}' but .claude/agents/{a}.md does not exist")
for a in sorted(on_disk_agents - looks_agent):
    routing_gaps.append(f"agents/{a}.md exists but ROUTING.md never routes to it")

report(not routing_gaps, "7. ROUTING matches the agents on disk",
       f"{len(on_disk_agents)} agents · {len(looks_agent)} routed", routing_gaps)


print()
print(f"Total: 7 invariants · passed {7-len(fails)} · failed {len(fails)}")
sys.exit(1 if fails else 0)
