"""PreToolUse — BLOCK queries without tenant scope, and single-column unique keys.  [RULE 1]

Scope: *.go under services/ (excluding tests, generated code, the platform layer)

WHY BLOCK RATHER THAN ADVISE: ViGov runs ONE system for MANY communes. A query missing
tenant_id raises no error, turns no test red, and returns rows from EVERY commune. A clerk
in commune A sees commune B's petitions — a data leak between two government bodies. Nobody
finds out until somebody complains.

Escape hatches — all EXPLICIT, none implicit:
  - go through a scoped repository:  s.scoped(ctx) · repo.For(ctx) · WithTenant(ctx)
  - the filter itself carries tenant_id
  - the line above declares:  // @cross-tenant: <business reason>
  - the file lives in services/platform/ (platform admin, not commune business)
"""

from __future__ import annotations

import os
import re
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import _common as c  # noqa: E402

HOOK = "tenant_scope_guard"

# Calls that touch the data store
DB_CALL = re.compile(
    r"\.\s*(Find|FindOne|FindAll|First|Get|GetAll|List|Query|QueryRow|Exec|Select|"
    r"Where|Count|Aggregate|Update|Updates|Save|Insert|Create|Delete)\s*\("
)

# The RIGHT path — a repository already scoped from context
SCOPED_OK = re.compile(
    r"(scoped\s*\(|\.For\s*\(\s*ctx|WithTenant\s*\(|TenantFrom\s*\(|tenantFrom\s*\(|"
    r"MustTenant\s*\(|scopedRepo|tenantRepo)"
)

HAS_TENANT = re.compile(r"tenant_?[Ii]d|TenantID")

# Explicit escape hatch — logged, and auditable via /review-isolation
ESCAPE = re.compile(r"//\s*@cross-tenant:\s*\S+")

# Single-column unique key — the second commune will collide
UNIQUE_ONE_COL = re.compile(
    r"(?:UNIQUE\s*\(\s*([a-z_]+)\s*\)|"                      # SQL
    r"uniqueIndex(?!:)|"                                      # bare gorm tag
    r"gorm:\"[^\"]*\bunique\b[^\"]*\")",                      # gorm unique tag
    re.IGNORECASE,
)

# A default value on the isolation path — absolutely forbidden (rule 1, forbidden #1)
DEFAULT_TENANT = re.compile(
    r"(tenant_?[Ii]d\s*(?::?=|==)\s*[\"'`]|"
    r"[Tt]enantID\s*\|\|\s*|"
    r"if\s+tenant_?[Ii]d\s*==\s*\"\"\s*\{[^}]{0,60}(default|\"default\"))"
)

# Tenant taken from the client — absolutely forbidden (rule 1, forbidden #2)
# Matches both  .Query("tenant_id")  and  .Query().Get("tenant_id")
TENANT_FROM_CLIENT = re.compile(
    r"((?:Query|FormValue|PostForm|Param|URLParam)\s*\(\s*[\"'`]tenant|"
    r"\.\s*Get\s*\(\s*[\"'`](?:tenant[_-]?id|X-Tenant[\w-]*)[\"'`]|"
    r"json:\"tenant_?id\")",
    re.IGNORECASE,
)

WATCH_EXT = (".go",)
PLATFORM = ("/services/platform/", "/internal/platform/")


def in_scope(path: str) -> bool:
    if not path.endswith(WATCH_EXT) or c.should_skip(path):
        return False
    if any(p in path for p in PLATFORM):
        return False
    return "/services/" in path or "/internal/" in path


def scan(content: str) -> list[str]:
    hits: list[str] = []
    lines = content.splitlines()

    for i, line in enumerate(lines):
        prev = lines[i - 1] if i else ""
        # Filters often wrap across lines — look at a small window around the call
        ctx = "\n".join(lines[max(0, i - 2): i + 4])

        if DB_CALL.search(line):
            if not (SCOPED_OK.search(ctx) or HAS_TENANT.search(ctx) or ESCAPE.search(prev)):
                hits.append(f"line {i+1}: {line.strip()[:70]} — no tenant scope")

        if UNIQUE_ONE_COL.search(line) and not HAS_TENANT.search(line):
            hits.append(f"line {i+1}: single-column unique key — must be composite with tenant_id")

        if DEFAULT_TENANT.search(line):
            hits.append(f"line {i+1}: default value for tenant_id — STRICTLY FORBIDDEN")

        if TENANT_FROM_CLIENT.search(line):
            hits.append(f"line {i+1}: tenant taken from client — STRICTLY FORBIDDEN")

    return hits


def main() -> None:
    c.utf8_streams()
    data = c.read_input()
    tool = c.tool_of(data)
    if tool not in ("Edit", "Write", "MultiEdit"):
        sys.exit(0)

    ti = c.input_of(data)
    path = c.path_of(ti)
    if not path or not in_scope(path):
        sys.exit(0)

    content = c.new_content(ti)
    if not content or c.is_generated(content):
        sys.exit(0)

    hits = scan(content)
    if not hits:
        sys.exit(0)

    c.block(
        HOOK,
        f"query without tenant scope — {os.path.basename(path)}",
        hits,
        [
            "  ViGov runs ONE system for MANY communes. A query missing tenant_id returns rows",
            "  from every commune — clerk in commune A sees commune B's files. Tests stay green.",
            "",
            "  Correct approach:",
            "    repo := s.scoped(ctx)                 // repository scoped from context",
            "    dt, err := repo.Find(ctx, filter)     // tenant_id added by the repository",
            "",
            "    // unique keys must be COMPOSITE",
            "    UNIQUE (tenant_id, code)",
            "",
            "  Genuinely need a cross-commune read (district/province reporting)? Be EXPLICIT:",
            "    // @cross-tenant: district-level petition rollup, aggregates only, audited",
            "",
            "  → Rule 1: .claude/rules/critical/1-tenant-isolation.md",
            "  → Skill: .claude/skills/go-tenant-context/SKILL.md",
        ],
        tool=tool,
        path=path,
    )


if __name__ == "__main__":
    main()
