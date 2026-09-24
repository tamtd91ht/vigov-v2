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


def _token_budget() -> int:
    """Đọc trần từ `kb/INDEX.yaml`, NGUỒN CHUẨN DUY NHẤT của con số này.

    VÌ SAO KHÔNG VIẾT CỨNG Ở ĐÂY: trần từng nằm ở CẢ HAI chỗ — `budget.always_load_max` trong
    INDEX.yaml và một hằng trong tệp này. Hai bản sao của một con số là hai bản sẽ lệch, và
    lệch ở đây hỏng theo kiểu tệ nhất trong hai kiểu: người dùng nâng trần trong INDEX.yaml
    (nơi chính tệp ấy khai rằng trần do người dùng chốt), cổng vẫn so với con số cũ, và câu
    trả lời là một lần đỏ không ai hiểu vì sao — đo đúng ngày 21/09/2026.

    Đọc bằng regex chứ không bằng PyYAML: `tools/` không được thêm phụ thuộc cho một phép kiểm
    phải chạy được trên mọi máy, kể cả máy chưa `pip install` gì.

    Không đọc được -> giữ 25000. HỎNG VỀ PHÍA CHẶT: một INDEX.yaml hỏng không được biến thành
    một cổng không còn trần nào.
    """
    try:
        with open(os.path.join(ROOT, "kb", "INDEX.yaml"), encoding="utf-8") as f:
            m = re.search(r"^\s*always_load_max:\s*(\d+)", f.read(), re.MULTILINE)
        if m:
            return int(m.group(1))
    except Exception:
        pass
    return 25000


TOKEN_BUDGET = _token_budget()

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
duong_tuong_doi = []
for ev in settings.get("hooks", {}).values():
    for grp in ev:
        for h in grp.get("hooks", []):
            cmd = h.get("command", "")
            m = re.search(r"hooks/([a-z_]+)\.py", cmd)
            if m:
                registered.add(m.group(1))
                # The path must be anchored at the project root. A RELATIVE path
                # ("python .claude/hooks/x.py") resolves against the working directory,
                # so the moment the session cd's into a subdirectory EVERY hook fails to
                # open its own file and the entire enforcement layer stops running.
                # It fails closed, so it announces itself — but it announces itself by
                # deadlocking the session, and the only way out is editing the very file
                # being blocked. Measured on 2026-09-17; this check is what would have
                # caught it before the commit.
                if "${CLAUDE_PROJECT_DIR}" not in cmd:
                    duong_tuong_doi.append(f"{m.group(1)}: {cmd}")

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

report(len(rules) >= 9 and not unregistered and not missing_file and not rule_hook_gaps
    and not duong_tuong_doi,
    "1. Rules <-> hooks, one to one",
    f"{len(rules)} rules · {len(on_disk)} hooks on disk · {len(registered)} registered"
    + (" · all paths anchored" if registered and not duong_tuong_doi else ""),
    rule_hook_gaps + [f"hook not registered: {x}" for x in unregistered]
    + [f"dang ky nhung khong co file: {x}" for x in missing_file]
    + [f"relative hook path — breaks outside the repo root: {x}" for x in duong_tuong_doi])


# ---- 2. Every hook has >=1 block case and >=1 pass case ------------------
tests = rd(os.path.join(ROOT, "tools", "test_hooks.py"))
cases = re.findall(r'\("([a-z_]+)",\s*"[^"]*",\s*(BLOCK|PASS)', tests)
tally: dict[str, dict[str, int]] = {}
for hook, kind in cases:
    tally.setdefault(hook, {"BLOCK": 0, "PASS": 0})[kind] += 1

# session_start / drift_guard / stop_verify_guard need a real session transcript, so they have no
# PAYLOAD case here.
#
# THE EXEMPTION IS ABOUT THE PAYLOAD, NOT ABOUT THE HOOK, and reading it the wider way cost
# something real: `stop_verify_guard.is_code()` is a pure function of a path deciding what counts
# as code at all, it needs no environment whatever — and it went untested until a missing
# `/tools/` made every edit to the contract generator invisible to the gate. Whatever part of an
# exempt hook is pure is testable, and `tools/test_hooks.py` now carries those cases separately.
# `workflow_guard` (2026-09-24) reads the same transcript; its decision `can_canh_bao` is pure
# and carried in WORKFLOW_CASES.
# `codegraph_sync` (2026-09-24) is a SIDE-EFFECT hook that never blocks by design, so a BLOCK
# payload case cannot exist; when it runs (`can_sync_sau`) is pure and carried in CAN_SYNC_CASES.
EXEMPT = {"session_start", "drift_guard", "stop_verify_guard", "workflow_guard", "codegraph_sync"}
gaps = []
for h in sorted(on_disk - EXEMPT):
    d = tally.get(h, {"BLOCK": 0, "PASS": 0})
    if d["BLOCK"] < 1:
        gaps.append(f"{h}: no BLOCK case")
    if d["PASS"] < 1:
        gaps.append(f"{h}: no PASS case")

report(not gaps, "2. Every hook has a block case and a pass case",
    f"{len(cases)} payload cases across {len(tally)} hooks "
    f"({len(EXEMPT)} exempt from payload cases; their pure parts are tested separately)",
    gaps)


# ---- 3. Every path referenced under .claude/** exists --------------------
PATH_RE = re.compile(r"`((?:\.claude|kb|tools|services|web|deploy)/[A-Za-z0-9_\-./]+"
                     r"\.(?:md|py|json|yaml|yml|go|ts|tsx|proto))`")

# Bare DIRECTORY references, e.g. `kb/20-contracts/`. Without this, a route pointing at a
# directory that does not exist stayed invisible — which is exactly how `kb/20-contracts/`
# and `kb/40-runbooks/` survived while being cited as live routes.
DIR_RE = re.compile(r"`(kb/[0-9]{2}-[a-z\-]+/)`")

# A tier may be documented BEFORE it exists, as long as the line says so plainly. The line
# must carry the marker, so "not created yet" is a claim the reader sees too — it cannot be
# used to silence a genuinely broken link.
PLANNED_MARK = ("not created yet", "chưa tồn tại", "chua ton tai")
dead = []
refs = 0
for dp, _, fn in os.walk(CLAUDE):
    if "logs" in dp:
        continue
    for f in fn:
        if not f.endswith(".md"):
            continue
        body = rd(os.path.join(dp, f))
        planned = {ln.strip() for ln in body.splitlines()
                   if any(k in ln.lower() for k in PLANNED_MARK)}
        for m in PATH_RE.findall(body) + DIR_RE.findall(body):
            refs += 1
            if os.path.exists(os.path.join(ROOT, m.replace("/", os.sep))):
                continue
            if any(m in ln for ln in planned):      # declared as not-yet-existing
                continue
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
    # `group` is what `/vigov-help` (tools/vigov_help.py) files the command under. Missing, the
    # command still shows — under "Chưa xếp nhóm" — but a help page that grows an unsorted pile
    # is a help page people stop reading, so it is required here, where it costs one line.
    if not re.search(r"^group:\s*\S", head, re.M):
        fm_gaps.append(f"commands/{f}: missing group (read by /vigov-help)")

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

# kb/INDEX.yaml carries this same number under `in_use`, written by `make kb`, and it is the
# first thing a session reads. Until 2026-09-16 the two disagreed by 7x — INDEX.yaml claimed
# 11% of the ceiling was used while the real figure was 80%. Only the ceiling is enforced here;
# a stale mirror is REPORTED, because failing the gate on it would turn every edit to a rule
# into a broken build until someone remembered to run `make kb`.
ghi = re.search(r"^\s*in_use:\s*(\d+)", idx, re.M)
lech = ""
if ghi and abs(int(ghi.group(1)) - total) > 10:
    lech = f" · kb/INDEX.yaml ghi {ghi.group(1)} — chạy `make kb`"

report(total <= TOKEN_BUDGET, "5. Always-loaded budget",
    f"~{total} / {TOKEN_BUDGET} tokens ({total*100//TOKEN_BUDGET}%){lech}",
    ["over budget — REMOVE something; never raise the ceiling"])


# ---- 6. No .md outside kb/, */README.md, .claude/ ---------------
# docs/ui-ux/ is a NAMED exception, not an open door: it holds the UI specification
# transcribed from an external running prototype. It answers "what does the screen look
# like", which kb/ deliberately does not own — so it competes with no owning file (rule 9 #2).
# The exception is the exact directory, never docs/ as a whole: the moment any .md may live
# under docs/, documentation starts scattering again, which is what this invariant exists
# to prevent.
ALLOWED = (
    re.compile(r"^kb[/\\]"), re.compile(r"^\.claude[/\\]"),
    # Bố cục phẳng: mỗi đơn vị triển khai nằm ở cấp một (`identity/`, `web-admin/`), nên
    # README của nó là `<đơn vị>/README.md`. Vẫn CHỈ một tệp ở đúng cấp ấy — không mở cửa
    # cho .md rải rác bên trong (luật 9, cấm #1).
    re.compile(r"^[a-z0-9_\-]+[/\\]README\.md$"),
    re.compile(r"^docs[/\\]ui-ux[/\\][^/\\]+\.md$"),
    re.compile(r"^(README|CLAUDE)\.md$"),
)
stray = []
for dp, dn, fn in os.walk(ROOT):
    # `tmp` nằm trong .gitignore, tức kho đã tuyên bố nó không phải một phần của kho. Phép kiểm
    # này đi bằng hệ tệp chứ không đi bằng git, nên một tệp nháp cục bộ từng làm đỏ cả cổng — và
    # một cổng đỏ vì lý do không ai sửa được là cổng người ta học cách bỏ qua.
    dn[:] = [d for d in dn
             if d not in ("node_modules", ".git", "vendor", "dist", "__pycache__", "tmp")]
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
# Names in ROUTING that look like an agent handle.
#
# THE SUFFIX LIST IS THE VOCABULARY OF AGENT ROLES, and it is closed on purpose: a name that
# ends in none of these is not recognised as an agent, so an agent added under an invented
# suffix shows up as "never routed to" rather than passing unnoticed. That is the check doing
# its job, and the fix is to decide which role the new agent has — not to widen the pattern
# until everything matches.
#
# `watcher` was added on 2026-09-23 for `require-watcher`, and the decision is written here
# because the alternative was to mislabel it. It writes, so `reviewer` and `expert` are wrong
# — both mean READ ONLY everywhere else in this brain, and a name that lies about write access
# is worse than a new word. It builds no code (`builder`) and owns no inter-service contract
# (`designer`). `keeper` was the near miss: it does maintain one kb/ tier. What separates it is
# the SOURCE — every other agent reads this repository, this one watches a DIFFERENT repository
# and reports what moved there. That is a distinct role, so it gets a distinct word.
#
# `scout` was added on 2026-09-24 for `context-scout` and `cross-context-scout`, the two
# discovery agents of ROUTING §0.2. They are read only, but `reviewer` would lie in the other
# direction: a reviewer judges finished work against a rule, a scout maps an area BEFORE any
# work exists and judges nothing. `expert` means business authority (`domain-expert`), which
# a scout must not claim — its whole contract is to report conflicts without picking a side.
looks_agent = set(re.findall(
    r"`([a-z][a-z0-9]*(?:-[a-z0-9]+)*-(?:builder|designer|reviewer|keeper|expert|watcher|scout))`",
    routing))
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
