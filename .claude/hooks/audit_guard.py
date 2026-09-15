"""PostToolUse — advise when a write operation leaves no audit trail.  [RULE 6]

WHY POST AND NOT PRE: a single Edit may have written only half a function. Blocking at Pre
would block work in progress. At Post the message enters the agent's context as something
to finish — and stop_verify_guard holds the session if verification has not run.

When a complaint, dispute or inspection arrives, the question is always "who did what, and
when". Not being able to answer is the authority's problem.
"""

from __future__ import annotations

import os
import re
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import _common as c  # noqa: E402

HOOK = "audit_guard"

WRITE_OP = re.compile(
    r"\.\s*(Create|Insert|Save|Update|Updates|Delete|SoftDelete|Exec)\s*\(|"
    r"[\"'`]\s*(INSERT\s+INTO|UPDATE\s+|DELETE\s+FROM)", re.I)

AUDIT_CALL = re.compile(r"(audit\.|Audit\(|WriteAudit|RecordAudit)", re.I)

FUNC_HEAD = re.compile(r"^func\s+(\([^)]*\)\s*)?([A-Za-z0-9_]+)\s*\(")


def funcs(content: str):
    """Split into functions — the audit check is WITHIN one function, not by line distance."""
    lines = content.splitlines()
    cur, start, name = [], 0, ""
    for i, line in enumerate(lines):
        m = FUNC_HEAD.match(line)
        if m:
            if cur:
                yield name, start + 1, "\n".join(cur)
            cur, start, name = [line], i, m.group(2)
        elif cur:
            cur.append(line)
    if cur:
        yield name, start + 1, "\n".join(cur)


def main() -> None:
    c.utf8_streams()
    data = c.read_input()
    tool = c.tool_of(data)
    if tool not in ("Edit", "Write", "MultiEdit"):
        sys.exit(0)

    ti = c.input_of(data)
    path = c.path_of(ti)
    if not path.endswith(".go") or c.should_skip(path):
        sys.exit(0)
    norm = "/" + path.lstrip("/")
    if "/services/" not in norm or "/platform/" in norm:
        sys.exit(0)

    content = c.new_content(ti)
    if not content or c.is_generated(content):
        sys.exit(0)

    hits = []
    for name, lineno, body in funcs(content):
        if WRITE_OP.search(body) and not AUDIT_CALL.search(body):
            hits.append(f"line {lineno}: {name}() writes data but records no audit entry")

    if not hits:
        sys.exit(0)

    c.warn(HOOK, f"{os.path.basename(path)} — write without an audit trail", hits,
           ["  An audit trail is NOT a technical log. Logs rotate by size and may be lost;",
            "  the audit trail is BUSINESS DATA: kept at least 12 months, never deleted.",
            "",
            "  Correct approach — write the entry IN THE SAME TRANSACTION as the change:",
            "    tx := repo.Begin(ctx)",
            "    tx.Update(ctx, dt)",
            "    audit.Write(ctx, tx, audit.Entry{",
            "        Action: \"update_petition\", Subject: dt.Code,",
            "        Before: audit.Diff(old, new),   // personal data already masked",
            "    })",
            "    tx.Commit()",
            "",
            "  Auditing outside the transaction means the change can succeed while the entry",
            "  fails — a state the records-retention rules do not allow.",
            "",
            "  → Rule 6: .claude/rules/critical/6-audit-log.md"],
           tool=tool, path=path)


if __name__ == "__main__":
    main()
