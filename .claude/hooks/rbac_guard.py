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

# HandleFunc comes BEFORE Handle: with `Handle` first the engine matches it against
# "HandleFunc(", then needs `\s*\(` and finds "Func(" instead. It does backtrack into the
# next alternative, but relying on that is how the case got missed in the first place —
# mux.HandleFunc("POST /...") registered a real route that this guard never saw.
ROUTE = re.compile(
    r"""\.\s*(HandleFunc|Handle|Get|Post|Put|Patch|Delete|Head|Options|Method)\s*\(\s*["'`]""")

AUTH_DECL = re.compile(
    r"(RequirePermission\s*\(|RequireAnyPermission\s*\(|RequireRole\s*\(|Public\s*\(|"
    r"AnyAuthenticated\s*\(|CitizenOnly\s*\(|RequireScope\s*\()")

# authz.RequireAnyPermission panics at wiring on fewer than two keys, and tools/apidoc refuses
# the same shape; this catches it at the write. One key is RequirePermission's spelling.
ANY_PERMISSION_ONE_KEY = re.compile(
    r"RequireAnyPermission\s*\(\s*[\w.]*\s*(?:,\s*\"[^\"]*\"\s*)?\)")

# Public()/AnyAuthenticated() must state a REASON — without one, nobody dares remove it later
REASON_REQUIRED = re.compile(r"(Public|AnyAuthenticated)\s*\(\s*\)")

# THE OPERATOR REALM (ADR 0048): service-platform's operator edge declares routes with
# service-platform/internal/opauth — RequireKey(auth, Key…) · SignedIn(auth, "<reason>") ·
# Public("<reason>"), the counterparts of authz.RequirePermission / AnyAuthenticated / Public.
# Accepted ONLY qualified as `opauth.` and ONLY under service-platform/ (the only module Go lets
# import that internal package), so a bare `SignedIn(` or `RequireKey(` on a commune route is
# still "NO permission declared". Same reason discipline as above, and the same shapes
# tools/apidoc/route.go refuses — two readers of one declaration must not disagree.
OPAUTH_DECL = re.compile(r"\bopauth\.(?:RequireKey|SignedIn)\s*\(")
OPAUTH_SCOPE = "service-platform/"
# SignedIn with the auth argument but no reason string after it.
OPAUTH_SIGNED_IN_NO_REASON = re.compile(r"\bopauth\.SignedIn\s*\(\s*[\w.]*\s*(?:,\s*\"\s*\"\s*)?\)")
# RequireKey naming no key: every listed key is the authorisation; none listed means "any operator".
OPAUTH_NO_KEY = re.compile(r"\bopauth\.RequireKey\s*\(\s*[\w.]*\s*\)")
OPAUTH_PUBLIC_EMPTY = re.compile(r"\bopauth\.Public\s*\(\s*\"\s*\"\s*\)")
# One realm per route: authz.* and opauth.* in one statement leaves the realm undecidable.
AUTHZ_ANY = re.compile(r"\bauthz\.\w+\s*\(")
OPAUTH_ANY = re.compile(r"\bopauth\.(?:RequireKey|SignedIn|Public)\s*\(")

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

    operator_ok = OPAUTH_SCOPE in path.replace("\\", "/")
    hits = []
    for lineno, stmt in statements(content):
        if not ROUTE.search(stmt):
            continue
        declared = AUTH_DECL.search(stmt) or (operator_ok and OPAUTH_DECL.search(stmt))
        if not declared:
            hits.append(f"line {lineno}: {stmt[:70]} — NO permission declared")
        elif REASON_REQUIRED.search(stmt):
            hits.append(f"line {lineno}: Public()/AnyAuthenticated() states NO reason")
        elif ANY_PERMISSION_ONE_KEY.search(stmt):
            hits.append(f"line {lineno}: RequireAnyPermission() names fewer than two keys — "
                        "use RequirePermission for one")
        elif OPAUTH_PUBLIC_EMPTY.search(stmt) or OPAUTH_SIGNED_IN_NO_REASON.search(stmt):
            hits.append(f"line {lineno}: opauth.Public()/SignedIn() states NO reason")
        elif OPAUTH_NO_KEY.search(stmt):
            hits.append(f"line {lineno}: opauth.RequireKey() names NO key")
        elif AUTHZ_ANY.search(stmt) and OPAUTH_ANY.search(stmt):
            hits.append(f"line {lineno}: authz.* and opauth.* in one route — choose ONE realm")

    if not hits:
        sys.exit(0)

    c.block(HOOK, f"route without permission — {base}", hits,
            ["  The global guard rejects users who are NOT LOGGED IN. It does not check",
             "  permissions. A route with no declaration is callable by EVERY staff role —",
             "  including roles with nothing to do with that subsystem. Silent: no error, no",
             "  failing test.",
             "",
             "  Declare EXPLICITLY, one of four:",
             "    mux.Handle(\"GET /phan-anh\", authz.RequirePermission(c, \"feedback.read\")(h.List))",
             "      (one route, two screens whose specs name different keys:",
             "       authz.RequireAnyPermission(c, \"admin.lookup\", \"budget.update\") — any one key suffices)",
             "    mux.Handle(\"GET /...\", authz.AnyAuthenticated(\"<why any account must reach this>\")(h))",
             "    mux.Handle(\"GET /cong-dan/phan-anh\", authz.CitizenOnly()(h.MyList))",
             "    mux.Handle(\"GET /tra-cuu\", authz.Public(\"<specific reason it is public>\")(h.Lookup))",
             "",
             "  Operator edge only (service-platform, ADR 0048) — the operator realm's three:",
             "    opauth.RequireKey(d.Auth, opauth.KeyTenantManage)(h) · opauth.SignedIn(d.Auth, \"<reason>\")(h)",
             "    · opauth.Public(\"<reason>\")(h). Never mixed with authz.* in one route.",
             "",
             "  A permission is ONE flat key, \"<nhóm>.<việc>\" — the same string the Phân quyền",
             "  screen shows and the same one stored in `quyen`. Not (subsystem, action): the",
             "  rights granted here are not a Cartesian product, and \"task.approve\" (duyệt hoàn",
             "  thành) is deliberately not \"task.extend\" (duyệt gia hạn).",
             "",
             "  Permissions are always WITHIN one commune — a missing tenant_id is cross-commune",
             "  privilege escalation (rule 1).",
             "  Tests: 401 no token · 403 wrong permission · 403 right permission WRONG COMMUNE · 200 both right.",
             "",
             "  → Rule 5: .claude/rules/critical/5-rbac.md"],
            tool=tool, path=path)


if __name__ == "__main__":
    main()
