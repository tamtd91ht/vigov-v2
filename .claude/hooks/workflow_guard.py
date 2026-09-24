"""Stop — ADVISE when a session changed code the workflow cannot exempt, and never ran discovery.

WHY THIS HOOK EXISTS: ROUTING §0 says no code is edited before the two discovery scouts have
reported and the gate has passed. A workflow nothing checks is a workflow that drifts — and
the drift has a known direction: the agent judges its own change "small and obvious", skips
discovery, and the change that compiles and passes its tests is the one that extended a
behaviour three recent commits were replacing.

WHAT IT CAN DECIDE, AND WHAT IT CANNOT. ROUTING §0.0 exempts a change only when it touches
none of the risky paths AND edits only comments, labels, text or docs. The second half is not
decidable from a path, so this hook only speaks when the FIRST half already fails:

  A. a code edit landed on a path §0.0 can never exempt (`vung_rui_ro`), or
     code edits touched at least NGUONG_SO_TEP files — no longer "one or two files"
  B. the session never dispatched BOTH `context-scout` and `cross-context-scout`
A and B → advise. Everything else is left to the agent's stated NOT APPLICABLE reason.

ADVISORY, NOT A BLOCK: like stop_verify_guard it exits 2 once so the message reaches the
agent; `stop_hook_active` lets the second stop through, so a session that states why
discovery did not apply can always end.
"""

from __future__ import annotations

import json
import os
import re
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import _common as c  # noqa: E402
from stop_verify_guard import is_code  # noqa: E402 — one definition of "code", never two

HOOK = "workflow_guard"

# Paths ROUTING §0.0 never exempts. Each is a place where a "two-line" change is a contract,
# schema, shared-library or permission change — the exact shapes the §0.3 gate lists.
RUI_RO = (
    "/proto/",            # contract between services
    "/migrations/",       # schema on populated tables
    "/core/",             # shared by every service
    "/internal/http/",    # REST surface: routes, permissions, response shapes
    "/internal/grpc/",    # gRPC surface
    "/internal/event/",   # published / consumed events
    "/authz/",            # authorisation
)

# "One or two files" is ROUTING §6's own wording; three is where it stops being true.
NGUONG_SO_TEP = 3

SCOUT = re.compile(r'"subagent_type"\s*:\s*"((?:cross-)?context-scout)"')


def vung_rui_ro(path: str) -> bool:
    """True when an edit to `path` can never be ROUTING §0.0-exempt."""
    p = (path or "").replace("\\", "/")
    if not is_code(p):
        return False
    if not p.startswith("/"):
        p = "/" + p
    return any(d in p for d in RUI_RO)


def can_canh_bao(tep_ma: list[str], scouts: set[str]) -> bool:
    """The pure decision: code edits the exemption cannot cover, and discovery incomplete."""
    if {"context-scout", "cross-context-scout"} <= scouts:
        return False
    return any(vung_rui_ro(p) for p in tep_ma) or len(set(tep_ma)) >= NGUONG_SO_TEP


def walk_transcript(path: str) -> tuple[list[str], set[str]]:
    """(code files edited, scout agent types dispatched)."""
    tep_ma: list[str] = []
    scouts: set[str] = set()
    if not path or not os.path.exists(path):
        return tep_ma, scouts
    try:
        with open(path, encoding="utf-8", errors="ignore") as f:
            for line in f:
                line = line.strip()
                if not line:
                    continue
                try:
                    rec = json.loads(line)
                except Exception:
                    continue
                blob = json.dumps(rec, ensure_ascii=False)
                for m in SCOUT.finditer(blob):
                    scouts.add(m.group(1))
                if any(t in blob for t in ('"Edit"', '"Write"', '"MultiEdit"')):
                    for m in re.finditer(r'"file_path"\s*:\s*"([^"]+)"', blob):
                        p = m.group(1).replace("\\\\", "/")
                        if is_code(p) and p not in tep_ma:
                            tep_ma.append(p)
    except Exception:
        pass
    return tep_ma, scouts


def main() -> None:
    c.utf8_streams()
    data = c.read_input()
    if data.get("stop_hook_active"):
        sys.exit(0)                      # already spoke once — the stated reason stands

    tep_ma, scouts = walk_transcript(data.get("transcript_path") or "")
    if not can_canh_bao(tep_ma, scouts):
        sys.exit(0)

    rui_ro = [os.path.basename(p) for p in tep_ma if vung_rui_ro(p)]
    thieu = sorted({"context-scout", "cross-context-scout"} - scouts)
    c.warn(
        HOOK,
        "code changed where ROUTING §0.0 cannot exempt it, and discovery did not run",
        [f"risky path: {t}" for t in rui_ro[:6]]
        + [f"code files edited: {len(tep_ma)}", f"scouts never dispatched: {', '.join(thieu)}"],
        [
            "  ROUTING §0: context-scout ‖ cross-context-scout, then the §0.3 gate, THEN code.",
            "",
            "  Already done and it was genuinely exempt? State the NOT APPLICABLE reason for",
            "  discovery in your reply and stop again — this hook speaks once per stop.",
            "  Not exempt? Dispatch both scouts now and re-check the change against their reports",
            "  before calling it done.",
            "",
            "  → .claude/agents/ROUTING.md §0.0 · §0.9",
        ],
        tool="Stop",
        path="",
    )


if __name__ == "__main__":
    main()
