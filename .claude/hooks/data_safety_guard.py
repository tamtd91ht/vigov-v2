"""PreToolUse (Bash + Edit/Write) — BLOCK operations that can destroy data.  [RULE 7]

Administrative files are ARCHIVAL RECORDS with statutory retention periods. Deleting a row
here is not "cleaning up data" — it is DESTROYING A RECORD, which may only happen through an
administrative procedure, never through a command.

Different from v1: the `rm` pattern now catches SPLIT FLAGS. Measured on v1: `rm -rf` was
blocked but `rm -r -f`, `rm --recursive --force`, `find -delete` and `rimraf` all passed,
because the old pattern required r and f inside the SAME flag cluster. This is an
ACCIDENT-prevention layer, not a sandbox: `settings.json` is where the permission layer
denies these.
"""

from __future__ import annotations

import os
import re
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import _common as c  # noqa: E402

HOOK = "data_safety_guard"

DANGEROUS_BASH = [
    (r"\brm\b(?=(?:[^\n|;&]*\s-{1,2}[rR]\w*\b))(?=(?:[^\n|;&]*\s-{1,2}\w*[fF]\w*\b))",
     "recursive forced rm",
     "Unrecoverable directory removal. Name specific paths and remove them individually."),
    (r"\brm\s+--recursive\b|\brm\s+--force\b", "rm --recursive/--force",
     "Same as above, long-flag form."),
    (r"\bfind\b[^\n|;&]*\s-delete\b|\bfind\b[^\n|;&]*-exec\s+rm\b", "find -delete / -exec rm",
     "Bulk removal by pattern — easily hits things you did not mean."),
    (r"\b(rimraf|del\s+/s|Remove-Item\b[^\n]*-Recurse)\b", "other recursive delete tools",
     "Same consequence as rm -rf."),
    (r"\bgit\s+reset\s+--hard\b", "git reset --hard",
     "Loses every uncommitted change. Use `git stash` to keep them."),
    (r"\bgit\s+clean\b", "git clean",
     "Removes untracked files — possibly real config files."),
    (r"\bgit\s+push\b[^\n|;&]*\s(--force(?!-with-lease)\b|-f\b)", "git push --force",
     "Overwrites someone else's history. Use --force-with-lease, after asking."),
    (r"\bgit\s+filter-branch\b|\bgit\s+filter-repo\b", "history rewrite",
     "Changes every commit hash. Only with the whole team's agreement."),
    (r"\bDROP\s+(TABLE|DATABASE|SCHEMA)\b|\bTRUNCATE\b", "DROP / TRUNCATE",
     "Destroys structure and data."),
    (r"\bdropDatabase\s*\(|\bdb\.dropDatabase\b", "dropDatabase", "Destroys the whole database."),
    (r"\b(psql|mongosh?)\b[^\n|;&]*(-c|--eval)\b", "direct command against the database",
     "Changing real data this way leaves no trail. Go through the API so it is audited (rule 6)."),
    (r"\bdocker\s+(compose\s+)?down\b[^\n]*(-v|--volumes)\b", "docker compose down -v",
     "Removes volumes — all container data lost."),
    (r"\bdocker\s+(volume\s+(rm|prune)|system\s+prune)\b", "docker volume removal",
     "Data in volumes lost."),
    (r"\bgit\s+add\s+(-f|--force)\b[^\n]*\.env(?!\.example|\.sample)", "git add -f on an env file",
     ".gitignore blocking env files is BY DESIGN. Only the template is committed."),
]

HARD_DELETE = [
    (r"\bDELETE\s+FROM\b", "DELETE FROM"),
    (r"\.\s*(Delete|DeleteAll|HardDelete)\s*\(", "hard delete"),
    (r"\.\s*Unscoped\s*\(\s*\)\s*\.\s*Delete", "Unscoped().Delete — bypasses soft delete"),
    (r"\bdb\.Migrator\(\)\.DropTable|\bDropColumn\b", "dropping a table / column"),
]

# An UPDATE or DELETE with no WHERE touches every row of the table.
#
# CASE-SENSITIVE on purpose, and requiring the SQL keyword that must follow. Measured cost of
# getting this wrong: the earlier case-insensitive form matched Go's built-in `delete(m, k)`,
# which removes one key from an in-memory map and has nothing to do with the database. Every
# Go file holding a map was blocked, and a guard that cries wolf is a guard someone switches
# off — which costs more than the rule it was protecting.
#
# SQL keywords are written upper-case throughout this codebase, so requiring upper case here
# loses no real coverage. The ORM forms below stay case-sensitive for the same reason.
#
# The lookahead spans up to the statement terminator, NOT to the end of the line: a readable
# multi-line UPDATE puts its WHERE on the next line, and a line-bounded check flagged every one
# of them. Same lesson as the Go `delete()` case above — the guard has to match how the code is
# actually written, or it gets switched off.
EMPTY_FILTER = re.compile(
    r"\bDELETE\s+FROM\b(?![^;]*?\bWHERE\b)"
    r"|\bUPDATE\s+\w+\s+SET\b(?![^;]*?\bWHERE\b)"
    r"|\.\s*(Updates?|Delete)\s*\(\s*\)")

BUSINESS = re.compile(
    r"(don_?thu|van_?ban|phan_?anh|nhiem_?vu|giai_?ngan|ho_?so|cong_?dan|can_?bo|"
    r"audit|nhat_?ky|document|dossier|petition|feedback|task|citizen|staff)", re.I)


def check_bash(cmd: str) -> None:
    for pattern, name, why in DANGEROUS_BASH:
        if re.search(pattern, cmd, re.I):
            c.block(HOOK, f"command that can destroy data: {name}",
                    [f"Command: {cmd[:150]}", why],
                    ["  ViGov is a government system — administrative files are archival records",
                     "  with statutory retention. If this command is TRULY needed, state plainly",
                     "  what it will affect and wait for explicit confirmation.",
                     "",
                     "  → Rule 7: .claude/rules/critical/7-data-preservation.md"],
                    tool="Bash", path="")


def check_code(content: str, path: str, tool: str) -> None:
    hits = []
    for pattern, name in HARD_DELETE:
        for m in re.finditer(pattern, content, re.I):
            ctx = content[max(0, m.start() - 150): m.end() + 80]
            if BUSINESS.search(ctx) or BUSINESS.search(path):
                hits.append(f"{name} on business data")
    for m in EMPTY_FILTER.finditer(content):
        hits.append(f"{m.group(0).strip()[:40]} — no filter, affects EVERY row")

    if not hits:
        return

    c.block(HOOK, f"hard delete of business data in {os.path.basename(path)}",
            list(dict.fromkeys(hits)),
            ["  Administrative files (documents, petitions, feedback, disbursements, accounts)",
             "  are ARCHIVAL RECORDS. The correct approach is SOFT DELETE:",
             "    - set deleted_at / deleted_by / delete_reason",
             "    - exclude deleted rows from EVERY read path, statistics and background jobs included",
             "    - never reissue a code that was already used",
             "",
             "  The only lawful exception: a citizen requesting erasure under Decree 13/2023 —",
             "  and that means ANONYMISE, not hard delete.",
             "",
             "  → Rule 7: .claude/rules/critical/7-data-preservation.md"],
            tool=tool, path=path)


def main() -> None:
    c.utf8_streams()
    data = c.read_input()
    tool = c.tool_of(data)
    ti = c.input_of(data)

    if tool == "Bash":
        cmd = ti.get("command") or ""
        if cmd:
            check_bash(cmd)
        sys.exit(0)

    if tool in ("Edit", "Write", "MultiEdit"):
        path = c.path_of(ti)
        if not path or c.should_skip(path):
            sys.exit(0)
        content = c.new_content(ti)
        if content and not c.is_generated(content, path):
            check_code(content, path, tool)

    sys.exit(0)


if __name__ == "__main__":
    main()
