"""Read the `ALTER … RENAME` statements of one migration file, in order — and only read them.

WHY THIS FILE EXISTS (ADR 0061, layer 0). Several tools derive the schema by reading every
migration in order, and until 2026-09-29 each of them understood only `CREATE TABLE` / `ADD
COLUMN`. Layer B renames tables, columns, indexes and constraints with `ALTER … RENAME` in a
NEW migration (applied files are never edited). A tool that does not fold those statements goes
on checking a table that no longer exists — and stays green doing it, which is the failure this
repository has met most often ("a guard that looks on duty and is dead").

ONE READER, SEVERAL GATES: `check_khoa_duy_nhat.py` and `quyen_keys.py` (hence
`check_quyen.py` and `.claude/hooks/quyen_key_guard.py`) import it. Two copies of a reader are
two readers that drift, and the one that drifted is the silent one (rule 9). `tools/kb` is Go and
carries its own table-rename reader (`tools/kb/ownership.go`), because it only needs that one.

COMMENTS ARE STRIPPED FIRST, and that is not tidiness: identity's 0009 already writes its
reversal as `--      ALTER TABLE nguoi_dung RENAME COLUMN dien_thoai_co_quan TO dien_thoai;`.
Folding that line would undo the rename it documents.

WHAT IT DOES NOT READ, stated so nobody infers a guarantee: quoted identifiers (`"Ten"`), and
`ALTER SEQUENCE / TRIGGER / FUNCTION … RENAME` — no gate here keys on those names. A rename
built as a string inside `EXECUTE format(…)` is invisible, as is every dynamic statement.

Standard library only.
"""

from __future__ import annotations

import re
from typing import NamedTuple

_ID = r"[A-Za-z_][A-Za-z0-9_]*"

# `ALTER TABLE [IF EXISTS] [ONLY] [schema.]t RENAME …` — one pattern for the four ALTER TABLE
# forms, so they cannot disagree about what a table name is.
#   RENAME TO new                      the table
#   RENAME [COLUMN] old TO new         a column (PostgreSQL makes COLUMN optional)
#   RENAME CONSTRAINT old TO new       a constraint
_ALTER_TABLE = re.compile(
    rf"\bALTER\s+TABLE\s+(?:IF\s+EXISTS\s+)?(?:ONLY\s+)?(?:{_ID}\.)?({_ID})\s+RENAME\s+"
    rf"(?:(COLUMN|CONSTRAINT)\s+)?(?:({_ID})\s+)?TO\s+({_ID})",
    re.IGNORECASE)
_ALTER_INDEX = re.compile(
    rf"\bALTER\s+INDEX\s+(?:IF\s+EXISTS\s+)?(?:{_ID}\.)?({_ID})\s+RENAME\s+TO\s+({_ID})",
    re.IGNORECASE)


class Rename(NamedTuple):
    pos: int      # offset of `ALTER` in the ORIGINAL text — comments keep their length
    kind: str     # "table" | "column" | "constraint" | "index"
    table: str    # the table the statement names; for kind "table" it is the old name
    old: str
    new: str


def strip_comments(sql: str) -> str:
    """Blank `--` and `/* */` comments, keeping every offset and every newline.

    A `--` inside a '…' literal is text, not a comment. Dollar-quoted bodies (`$$ … $$`) are
    left as code: a DO block's statements are statements, and a rename inside one is a rename.
    """
    out = list(sql)
    i, n = 0, len(sql)
    while i < n:
        c = sql[i]
        if c == "'":
            i += 1
            while i < n and sql[i] != "'":
                i += 1
            i += 1
            continue
        if sql.startswith("--", i):
            while i < n and sql[i] != "\n":
                out[i] = " "
                i += 1
            continue
        if sql.startswith("/*", i):
            j = sql.find("*/", i + 2)
            j = n if j < 0 else j + 2
            for k in range(i, j):
                if sql[k] != "\n":
                    out[k] = " "
            i = j
            continue
        i += 1
    return "".join(out)


def renames(sql: str) -> list[Rename]:
    """Every rename in `sql`, in statement order. Names are lower-cased, as PostgreSQL folds them."""
    code = strip_comments(sql)
    out: list[Rename] = []
    for m in _ALTER_TABLE.finditer(code):
        table, keyword, old, new = m.group(1).lower(), m.group(2), m.group(3), m.group(4).lower()
        if old is None:
            if keyword:          # `RENAME COLUMN TO x` is not SQL; do not guess
                continue
            out.append(Rename(m.start(), "table", table, table, new))
        else:
            kind = "constraint" if (keyword or "").upper() == "CONSTRAINT" else "column"
            out.append(Rename(m.start(), kind, table, old.lower(), new))
    for m in _ALTER_INDEX.finditer(code):
        out.append(Rename(m.start(), "index", "", m.group(1).lower(), m.group(2).lower()))
    out.sort(key=lambda r: r.pos)
    return out
