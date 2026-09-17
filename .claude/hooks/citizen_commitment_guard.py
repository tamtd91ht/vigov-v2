"""PreToolUse — enforce rule 10: the commitment made to the citizen.

WHY THIS HOOK EXISTS: a petition deadline is a COMMITMENT BY A PUBLIC AUTHORITY, not a UI
detail. Two ways of getting it wrong are both silent and both produce a false figure in a
report that goes to leadership:

  1. `is_overdue` stored as a column. It is wrong the moment a job is late, the clock skews,
     or a holiday is added — and the stale copy is the one people report upward.
  2. Deadlines counted in calendar days. A 3-day SLA taken on Friday is then due Monday,
     which is one working day, not three. The commune is reported as on-time when it is late.

CALIBRATION: most services are still skeletons, so there is no real code to measure against. This
hook is therefore deliberately NARROW and conservative — it only fires on patterns that
cannot be right, and prefers a miss over a false positive. A noisy hook is a disabled hook,
and then the whole layer is gone. RE-MEASURE once real business code exists.
"""

from __future__ import annotations

import os
import re
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import _common as c  # noqa: E402

HOOK = "citizen_commitment_guard"

# --- rule 10, forbidden #1: a WRITTEN overdue field ------------------------------------
# Only a declaration or an assignment counts. Reading or deriving is fine, which is why
# `IsOverdue()` as a method and comparisons like `deadline.Before(now)` never match.
_OVERDUE = r"(?:is_?overdue|overdue|qua_?han|is_?late)"
OVERDUE_STORED = re.compile(
    r"(?:"
    # Go struct field / SQL column:  IsOverdue bool  |  is_overdue BOOLEAN
    r"\b" + _OVERDUE + r"\s+(?:bool|boolean|tinyint|smallint)\b"
    # assignment, with or without a receiver:  p.IsOverdue = true  |  overdue := x
    r"|(?:\.|\b)" + _OVERDUE + r"\s*(?::=|=)\s*(?!=)"
    r"|ADD COLUMN\s+" + _OVERDUE + r"\b"
    r")",
    re.IGNORECASE,
)

# --- rule 10, forbidden #2: calendar-day deadline maths --------------------------------
CALENDAR_DAYS = re.compile(
    r"(?:AddDate\(\s*0\s*,\s*0\s*,|"
    r"(?:24|48|72)\s*\*\s*time\.Hour|"
    r"time\.Hour\s*\*\s*(?:24|48|72)|"
    r"INTERVAL\s+'?\d+\s+days?)",
    re.IGNORECASE,
)

# The calendar-day check only applies where a DEADLINE is being computed. Without this
# gate every ordinary date arithmetic in the repo would fire.
DEADLINE_CONTEXT = re.compile(
    r"(?:sla|deadline|han_xu_ly|hanXuLy|due_?date|dueDate|thoi_han|qua_han)",
    re.IGNORECASE,
)

# Escape hatch — explicit, greppable, and logged, per the brain's own rule for hooks.
ESCAPE = re.compile(r"@sla-ok:\s*\S+")

WATCH_EXT = (".go", ".sql", ".ts", ".tsx")


def in_scope(path: str) -> bool:
    p = path.lower()
    if not p.endswith(WATCH_EXT) or c.should_skip(p):
        return False
    return True


def scan(content: str) -> list[str]:
    hits: list[str] = []
    lines = content.splitlines()
    has_deadline_ctx = bool(DEADLINE_CONTEXT.search(content))

    for i, line in enumerate(lines):
        window = "\n".join(lines[max(0, i - 2): i + 2])
        if ESCAPE.search(window):
            continue

        if OVERDUE_STORED.search(line):
            hits.append(
                f"line {i+1}: {line.strip()[:70]} — overdue STORED; it must be DERIVED"
            )

        # Only inside a file that is actually doing deadline work.
        if has_deadline_ctx and CALENDAR_DAYS.search(line) and DEADLINE_CONTEXT.search(window):
            hits.append(
                f"line {i+1}: {line.strip()[:70]} — calendar days, not working days"
            )

    return hits


def main() -> None:
    c.utf8_streams()
    data = c.read_input()
    if c.tool_of(data) not in ("Edit", "Write", "MultiEdit"):
        sys.exit(0)

    ti = c.input_of(data)
    path = c.path_of(ti)
    if not path or not in_scope(path):
        sys.exit(0)

    content = c.new_content(ti)
    if not content or c.is_generated(content, path):
        sys.exit(0)

    hits = scan(content)
    if not hits:
        sys.exit(0)

    c.block(
        HOOK,
        f"petition deadline handled unsafely — {os.path.basename(path)}",
        hits,
        [
            "  An SLA deadline is a COMMITMENT to a citizen by a public authority. Both of the",
            "  patterns above fail SILENTLY and end up as a false figure in a report to leadership.",
            "",
            "  Correct approach:",
            "    // store the deadline ONCE, at intake",
            "    p.SLADeadline = sla.Deadline(ctx, receivedAt, p.Field)  // working days, per commune",
            "",
            "    // derive overdue — never store it",
            "    func (p Petition) IsOverdue(now time.Time) bool {",
            "        return p.ClosedAt.IsZero() && now.After(p.SLADeadline)",
            "    }",
            "",
            "  Genuinely need calendar days (a statutory period counted in calendar days)?",
            "    // @sla-ok: Law on Complaints art. 28 counts calendar days, not working days",
            "",
            "  → Rule 10: .claude/rules/critical/10-citizen-commitment.md",
            "  → Skill: .claude/skills/petition-lifecycle/SKILL.md",
        ],
        tool=c.tool_of(data),
        path=path,
    )


if __name__ == "__main__":
    main()
