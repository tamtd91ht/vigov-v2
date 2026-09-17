"""PreToolUse — BLOCK queries without tenant scope, and single-column unique keys.  [RULE 1]

Scope: *.go inside a service (excluding tests, generated code, the platform layer)

WHY BLOCK RATHER THAN ADVISE: ViGov runs ONE system for MANY communes. A query missing
tenant_id raises no error, turns no test red, and returns rows from EVERY commune. A clerk
in commune A sees commune B's petitions — a data leak between two government bodies. Nobody
finds out until somebody complains.

Escape hatches — all EXPLICIT, none implicit:
  - go through a scoped repository:  s.scoped(ctx) · repo.For(ctx) · WithTenant(ctx)
  - the filter itself carries tenant_id
  - the line above declares:  // @cross-tenant: <business reason>
  - the file belongs to the `platform` service (platform admin, not commune business)
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
PLATFORM = ("/internal/platform/",)

# A table declared PARTITION BY and given no partitions REJECTS EVERY INSERT. Because the
# audit entry shares the business transaction (rule 6, invariant 3), the first real business
# write rolls back entirely — not "the trail is missing", the OPERATION CANNOT HAPPEN. In a
# petitions service that first write is a clerk taking a citizen's report.
#
# Six of eight services shipped exactly that, and it was not a slip: the skeleton template in
# those files wrote `) PARTITION BY HASH (tenant_id);` and then stopped, so every business
# table still to be written from it would have inherited the same hole.
#
# This guard never saw it because WATCH_EXT is `.go` only — migrations were outside every
# hook in the brain. `0002` carries a DO-block backstop, but that one only checks the state
# after a migration that CARRIES the block, and "remember to append the block" is precisely
# the instruction that already failed here.
DECLARE_PART = re.compile(
    r"CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?([a-z_][a-z0-9_]*)\b[^;]*?\bPARTITION\s+BY\b",
    re.IGNORECASE | re.DOTALL)
CREATE_PART = re.compile(r"\bPARTITION\s+OF\s+([a-z_][a-z0-9_]*)", re.IGNORECASE)
SQL_COMMENT = re.compile(r"--[^\n]*")


def in_scope_sql(path: str) -> bool:
    return path.endswith(".sql") and "/migrations/" in path and not c.should_skip(path)


def scan_sql(content: str) -> list[str]:
    """Every table declared PARTITION BY must get its partitions in the SAME file.

    Comments are stripped first: the skeleton template shows the declaration in prose, and
    flagging the very comment that teaches the shape would be a guard nobody keeps.
    """
    sql = SQL_COMMENT.sub("", content)
    khai = {m.group(1).lower() for m in DECLARE_PART.finditer(sql)}
    tao = {m.group(1).lower() for m in CREATE_PART.finditer(sql)}
    return [f"table '{t}' is PARTITION BY with no PARTITION OF — every INSERT will fail"
            for t in sorted(khai - tao)]


def in_scope(path: str) -> bool:
    if not path.endswith(WATCH_EXT) or c.should_skip(path):
        return False
    if any(p in path for p in PLATFORM):
        return False
    # Dịch vụ `platform` giữ sổ đăng ký xã — dữ liệu của NỀN TẢNG, không phải dữ liệu nghiệp
    # vụ của một xã, nên nó không có `tenant_id` để phạm vi hoá (ADR 0003).
    #
    # So bằng TÊN NGHIỆP VỤ chứ không bằng tên thư mục: thư mục nay là `service-platform`, và
    # loại trừ này từng được viết là `/services/platform/` — một chuỗi nay không còn tồn tại.
    # Guard vẫn chạy, chỉ là mất đúng cái loại trừ nó cần, và hậu quả là nó kêu trên mọi tệp
    # của platform cho tới khi ai đó tắt nó đi.
    dv = c.dich_vu_cua(path)
    if dv is not None and c.ten_nghiep_vu(dv) == "platform":
        return False
    return dv is not None or "/internal/" in path


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
    if not path:
        sys.exit(0)

    content = c.new_content(ti)
    if not content or c.is_generated(content, path):
        sys.exit(0)

    if in_scope_sql(path):
        sql_hits = scan_sql(content)
        if sql_hits:
            c.block(HOOK, f"partitioned table with no partitions — {os.path.basename(path)}",
                    sql_hits,
                    ["  A table declared PARTITION BY and given no partitions REJECTS EVERY",
                     "  INSERT. The audit entry shares the business transaction (rule 6,",
                     "  invariant 3), so the first real business write ROLLS BACK ENTIRELY —",
                     "  not a missing trail, the operation itself cannot happen. In petitions",
                     "  that first write is a clerk taking a citizen's report.",
                     "",
                     "  Create the partitions in the SAME file as the declaration. Splitting",
                     "  them leaves the file broken when run alone, and running it alone is what",
                     "  every integration test does:",
                     "",
                     "    DO $$ BEGIN",
                     "      FOR i IN 0..31 LOOP",
                     "        EXECUTE format(",
                     "          'CREATE TABLE IF NOT EXISTS %I PARTITION OF audit_log '",
                     "          'FOR VALUES WITH (MODULUS 32, REMAINDER %s)',",
                     "          'audit_log_p' || lpad(i::text, 2, '0'), i);",
                     "      END LOOP;",
                     "    END $$;",
                     "",
                     "  MODULUS 32 is fixed by ADR 0010 — do not pick a different number.",
                     "",
                     "  → Rule 1: .claude/rules/critical/1-tenant-isolation.md",
                     "  → ADR 0010: kb/10-decisions/0010-data-infrastructure.md"],
                    tool=tool, path=path)
        sys.exit(0)

    if not in_scope(path):
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
