"""PreToolUse — BLOCK routes with no explicit permission declaration.  [RULE 5]

WHY ANCHORED TO THE STATEMENT, NOT TO LINE DISTANCE: the v1 RBAC hook looked for a
permission declaration within ±6 lines of a route. In a normally written router, routes sit
3-5 lines apart, so route B with no permission "borrows" route A's declaration — the hook
reports clean while a delete route is open to every staff role. Measured on v1: a @Delete
with no permission, 4 lines below another route → MISSED; pushed 12 lines away → caught.
Distance was the only difference.

v2 anchors to the route registration STATEMENT (joining chained .With(...).Get(...) even
across lines), so "close enough to borrow" no longer exists.
"""

from __future__ import annotations

import os
import re
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import _common as c  # noqa: E402

HOOK = "rbac_guard"

ROUTE = re.compile(r"""\.\s*(Get|Post|Put|Patch|Delete|Head|Options|Handle|Method)\s*\(\s*["'`]""")

AUTH_DECL = re.compile(
    r"(RequirePermission\s*\(|RequireRole\s*\(|Public\s*\(|AnyAuthenticated\s*\(|"
    r"CitizenOnly\s*\(|RequireScope\s*\()")

# Public()/AnyAuthenticated() must state a REASON — without one, nobody dares remove it later
REASON_REQUIRED = re.compile(r"(Public|AnyAuthenticated)\s*\(\s*\)")

WATCH = ("router.go", "routes.go", "handler.go", "server.go", "api.go")


def statements(content: str) -> list[tuple[int, str]]:
    """Join multi-line statements: accumulate until parentheses balance."""
    out, buf, start, depth = [], [], 0, 0
    for i, line in enumerate(content.splitlines(), 1):
        if not buf:
            start = i
        buf.append(line)
        depth += line.count("(") - line.count(")")
        if depth <= 0:
            out.append((start, " ".join(x.strip() for x in buf)))
            buf, depth = [], 0
    if buf:
        out.append((start, " ".join(x.strip() for x in buf)))
    return out


def main() -> None:
    c.utf8_streams()
    data = c.read_input()
    tool = c.tool_of(data)
    if tool not in ("Edit", "Write", "MultiEdit"):
        sys.exit(0)

    ti = c.input_of(data)
    path = c.path_of(ti)
    base = os.path.basename(path)
    if not path.endswith(".go") or c.should_skip(path):
        sys.exit(0)
    if not (base in WATCH or "/transport/" in path or "/http/" in path):
        sys.exit(0)

    content = c.new_content(ti)
    if not content or c.is_generated(content, path):
        sys.exit(0)

    hits = []
    for lineno, stmt in statements(content):
        if not ROUTE.search(stmt):
            continue
        if not AUTH_DECL.search(stmt):
            hits.append(f"line {lineno}: {stmt[:70]} — NO permission declared")
        elif REASON_REQUIRED.search(stmt):
            hits.append(f"line {lineno}: Public()/AnyAuthenticated() states NO reason")

    if not hits:
        sys.exit(0)

    c.block(HOOK, f"route without permission — {base}", hits,
            ["  The global guard rejects users who are NOT LOGGED IN. It does not check",
             "  permissions. A route with no declaration is callable by EVERY staff role —",
             "  including roles with nothing to do with that subsystem. Silent: no error, no",
             "  failing test.",
             "",
             "  Declare EXPLICITLY, one of four:",
             "    r.With(auth.RequirePermission(\"petitions\", \"view\")).Get(\"/petitions\", h.List)",
             "    r.With(auth.AnyAuthenticated(\"<why any account must reach this>\")).Get(...)",
             "    r.With(auth.CitizenOnly()).Get(\"/citizen/petitions\", h.MyList)",
             "    r.With(auth.Public(\"<specific reason it is public>\")).Get(\"/lookup\", h.Lookup)",
             "",
             "  Permissions are always WITHIN one commune — a missing tenant_id is cross-commune",
             "  privilege escalation (rule 1).",
             "  Tests: 401 no token · 403 wrong permission · 403 right permission WRONG COMMUNE · 200 both right.",
             "",
             "  → Rule 5: .claude/rules/critical/5-rbac.md"],
            tool=tool, path=path)


if __name__ == "__main__":
    main()
