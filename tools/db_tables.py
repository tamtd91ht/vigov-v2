#!/usr/bin/env python3
"""List every table of each service — purpose and structure — derived from its migrations.

WHY FROM MIGRATIONS AND NOT FROM A LIVE DATABASE: the migrations ARE the schema in this repo
(core/migrate applies them in order and refuses an edited file), they are on every machine, and
reading them needs no DSN and no credential (rule 8). The cost, stated: a change made to a real
database by hand, outside a migration, is invisible here — which is also a change nobody should
have made.

WHY A LOCAL CACHE: parsing ~90 files is cheap but not free, and a session asking "what does
table X hold" should not pay for it every time. The cache lives in tmp/db-tables/ (git-ignored):
it is a derived copy, not documentation, and putting it in kb/ would make it a second source of
the schema (rule 9). Each cache file carries a fingerprint of the migration files it was built
from; a mismatch is REPORTED (exit 3), never silently rebuilt — `--reload` is the user's act.

WHAT "PURPOSE" MEANS: the comment block written right above `CREATE TABLE`, the `@entity` /
`@scope` marks (ADR 0021), and any `COMMENT ON`. A table with none of these says so — this tool
never invents a description.

Run:   python tools/db_tables.py                     every service (cache when fresh)
       python tools/db_tables.py finance             one service (`service-finance` works too)
       python tools/db_tables.py finance --reload    ignore the cache, re-parse, overwrite it
       python tools/db_tables.py --brief             one line per table, no columns
       python tools/db_tables.py --json              the cached model itself
       python tools/db_tables.py [service] --excel [path.xlsx]   workbook, default under tmp/db-tables/
Exit:  0 = printed · 2 = unknown service · 3 = printed FROM A STALE CACHE (run --reload)
       4 = xlsx path is inside the repo and not git-ignored · 5 = a cell looks like a real phone /
       national ID (nothing written) · 6 = xlsx target locked (open in Excel)
"""

from __future__ import annotations

import argparse
import datetime as dt
import glob
import hashlib
import json
import os
import re
import sys

for _stream in (sys.stdout, sys.stderr):
    try:
        _stream.reconfigure(encoding="utf-8", errors="replace")
    except Exception:
        pass

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
CACHE_DIR = os.path.join(ROOT, "tmp", "db-tables")

# Bump when the parser changes what it extracts: the version is part of the fingerprint, so every
# cache built by an older parser reads as stale instead of being trusted.
PARSER_VERSION = 2

IDENT = r'(?:"[^"]+"|[A-Za-z_][A-Za-z0-9_$]*)'
QNAME = rf"{IDENT}(?:\.{IDENT})?"

# Words that end a column's type and begin its constraints.
COLUMN_KEYWORDS = ("NOT", "NULL", "DEFAULT", "PRIMARY", "UNIQUE", "CHECK", "REFERENCES",
                   "CONSTRAINT", "GENERATED", "COLLATE")
TABLE_CONSTRAINT_STARTS = ("CONSTRAINT", "PRIMARY", "UNIQUE", "CHECK", "FOREIGN", "EXCLUDE", "LIKE")


# --------------------------------------------------------------------------------------------
# Lexing: blank out comments and string contents while keeping every offset, so structure is
# found on the masked text and display text is cut from the same offsets of the unmasked one.
# --------------------------------------------------------------------------------------------

def lex(sql: str) -> tuple[str, str, dict[int, str], list[tuple[int, int]]]:
    """Return (code, masked, line_comments, literal_spans).

    code    — sql with comments replaced by spaces (strings kept).
    masked  — code with string-literal contents replaced by spaces too.
    line_comments — 1-based line number -> text of the `--` comment on that line.
    literal_spans — (start, end) offsets of each string literal's contents.
    Dollar-quoted bodies ($$ … $$) are kept as code: in these migrations they are DO blocks and
    trigger functions whose statements are real DDL.
    """
    code = list(sql)
    masked = list(sql)
    comments: dict[int, str] = {}
    literals: list[tuple[int, int]] = []
    i, n, line = 0, len(sql), 1
    while i < n:
        c = sql[i]
        if c == "\n":
            line += 1
            i += 1
        elif sql.startswith("--", i):
            j = sql.find("\n", i)
            j = n if j < 0 else j
            comments[line] = sql[i + 2:j].strip()
            for k in range(i, j):
                code[k] = masked[k] = " "
            i = j
        elif sql.startswith("/*", i):
            j = sql.find("*/", i + 2)
            j = n if j < 0 else j + 2
            for k in range(i, j):
                if sql[k] == "\n":
                    line += 1
                else:
                    code[k] = masked[k] = " "
            i = j
        elif c == "'":
            j = i + 1
            while j < n:
                if sql[j] == "'" and j + 1 < n and sql[j + 1] == "'":
                    j += 2
                elif sql[j] == "'":
                    break
                else:
                    j += 1
            for k in range(i + 1, min(j, n)):
                if sql[k] == "\n":
                    line += 1
                else:
                    masked[k] = " "
            literals.append((i + 1, min(j, n)))
            i = j + 1
        else:
            i += 1
    return "".join(code), "".join(masked), comments, literals


def line_of(text: str, pos: int) -> int:
    return text.count("\n", 0, pos) + 1


def matching_paren(masked: str, open_pos: int) -> int:
    depth = 0
    for k in range(open_pos, len(masked)):
        if masked[k] == "(":
            depth += 1
        elif masked[k] == ")":
            depth -= 1
            if depth == 0:
                return k
    return -1


def statement_end(masked: str, start: int) -> int:
    """Offset of the `;` closing the statement that starts at `start` (paren depth 0)."""
    depth = 0
    for k in range(start, len(masked)):
        ch = masked[k]
        if ch == "(":
            depth += 1
        elif ch == ")":
            depth -= 1
        elif ch == ";" and depth <= 0:
            return k
    return len(masked)


def split_top(masked: str, start: int, end: int) -> list[tuple[int, int]]:
    """Spans of the comma-separated items in masked[start:end], at paren depth 0."""
    spans, depth, s = [], 0, start
    for k in range(start, end):
        ch = masked[k]
        if ch == "(":
            depth += 1
        elif ch == ")":
            depth -= 1
        elif ch == "," and depth == 0:
            spans.append((s, k))
            s = k + 1
    spans.append((s, end))
    return [(a, b) for a, b in spans if masked[a:b].strip()]


def squash(text: str) -> str:
    return re.sub(r"\s+", " ", text).strip()


def unquote(name: str) -> str:
    name = name.split(".")[-1] if not name.startswith('"') else name
    return name.strip('"')


def string_literals(code: str) -> str:
    """Concatenate the string literals in `code` (adjacent literals form one value in SQL)."""
    return "".join(m.group(1).replace("''", "'") for m in re.finditer(r"'((?:[^']|'')*)'", code))


# --------------------------------------------------------------------------------------------
# The comment block right above a line: the table's / column's purpose as its author wrote it.
# --------------------------------------------------------------------------------------------

def comment_block_above(lines: list[str], comments: dict[int, str], lineno: int) -> list[str]:
    block: list[str] = []
    k = lineno - 1
    while k >= 1 and lines[k - 1].lstrip().startswith("--"):
        block.append(comments.get(k, ""))
        k -= 1
    block.reverse()
    # Drop separator rulers, keep the rest verbatim.
    return [b for b in block if not re.fullmatch(r"-{3,}|={3,}", b)]


def describe_table(block: list[str], table: str) -> tuple[str, str, str, str]:
    """(description, full_block, entity, scope) from the block above a CREATE TABLE.

    The description is the paragraph that names the table ("x — …") when there is one, else the
    FIRST paragraph: authors open with what the table is and close with caveats, so the last
    paragraph reads as a description and is not one.
    """
    entity = scope = ""
    text: list[str] = []
    for b in block:
        m = re.match(r"@entity:\s*(\S+)", b)
        if m:
            entity = m.group(1)
            continue
        m = re.match(r"@scope:\s*(\S+)", b)
        if m:
            scope = m.group(1)
            continue
        text.append(b)
    # Paragraphs separated by bare `--` lines. Prefer the one naming the table ("x — …").
    paragraphs, cur = [], []
    for b in text:
        if b == "":
            if cur:
                paragraphs.append(" ".join(cur))
                cur = []
        else:
            cur.append(b)
    if cur:
        paragraphs.append(" ".join(cur))
    chosen = next((p for p in paragraphs if re.match(rf"`?{re.escape(table)}`?\s*[—–-]", p)), "")
    if not chosen and paragraphs:
        chosen = paragraphs[0]
    chosen = re.sub(rf"^`?{re.escape(table)}`?\s*[—–-]\s*", "", chosen)
    return squash(chosen), "\n\n".join(squash(p) for p in paragraphs), entity, scope


# --------------------------------------------------------------------------------------------
# Column and constraint parsing.
# --------------------------------------------------------------------------------------------

def first_keyword_at(masked: str, words: tuple[str, ...]) -> int:
    """Offset of the first top-level occurrence of one of `words` in masked, or len(masked)."""
    depth = 0
    for m in re.finditer(r"\(|\)|\b[A-Za-z]+\b", masked):
        tok = m.group(0)
        if tok == "(":
            depth += 1
        elif tok == ")":
            depth -= 1
        elif depth == 0 and tok.upper() in words:
            return m.start()
    return len(masked)


def parse_column(code: str, masked: str) -> dict | None:
    m = re.match(rf"\s*({IDENT})\s+", masked)
    if not m:
        return None
    name = unquote(m.group(1))
    rest_c, rest_m = code[m.end():], masked[m.end():]
    cut = first_keyword_at(rest_m, COLUMN_KEYWORDS)
    col = {
        "name": name,
        "type": squash(rest_c[:cut]),
        "nullable": True,
        "default": None,
        "primary_key": False,
        "unique": False,
        "references": None,
        "check": None,
        "comment": "",
    }
    tail_c, tail_m = rest_c[cut:], rest_m[cut:]
    up = tail_m.upper()
    if re.search(r"\bNOT\s+NULL\b", up):
        col["nullable"] = False
    if re.search(r"\bPRIMARY\s+KEY\b", up):
        col["primary_key"] = True
        col["nullable"] = False
    if re.search(r"\bUNIQUE\b", up):
        col["unique"] = True
    d = re.search(r"\bDEFAULT\b", up)
    if d:
        after_m = tail_m[d.end():]
        stop = first_keyword_at(after_m, COLUMN_KEYWORDS)
        col["default"] = squash(tail_c[d.end():d.end() + stop])
    g = re.search(r"\bGENERATED\b", up)
    if g:
        col["default"] = squash(tail_c[g.start():])
    r = re.search(rf"\bREFERENCES\s+({QNAME})\s*(\([^)]*\))?", tail_m, re.I)
    if r:
        col["references"] = unquote(r.group(1)) + (squash(r.group(2)) if r.group(2) else "")
    ch = re.search(r"\bCHECK\s*\(", tail_m, re.I)
    if ch:
        o = ch.end() - 1
        e = matching_paren(tail_m, o)
        col["check"] = squash(tail_c[ch.start():e + 1]) if e > 0 else None
    return col


def parse_constraint(code: str, masked: str) -> dict:
    name = ""
    m = re.match(rf"\s*CONSTRAINT\s+({IDENT})\s+", masked, re.I)
    body_c, body_m = code, masked
    if m:
        name = unquote(m.group(1))
        body_c, body_m = code[m.end():], masked[m.end():]
    kind_m = re.match(r"\s*(PRIMARY\s+KEY|UNIQUE|CHECK|FOREIGN\s+KEY|EXCLUDE|LIKE)", body_m, re.I)
    kind = squash(kind_m.group(1)).upper() if kind_m else "?"
    return {"name": name, "kind": kind, "definition": squash(body_c)}


def parse_create_table(code: str, masked: str, open_pos: int, close_pos: int,
                       lines: list[str], comments: dict[int, str]) -> tuple[list, list]:
    columns, constraints = [], []
    for a, b in split_top(masked, open_pos + 1, close_pos):
        seg_m = masked[a:b]
        head = seg_m.strip().split(None, 1)[0].upper() if seg_m.strip() else ""
        if head in TABLE_CONSTRAINT_STARTS:
            constraints.append(parse_constraint(code[a:b], seg_m))
            continue
        col = parse_column(code[a:b], seg_m)
        if not col:
            continue
        first = a + (len(seg_m) - len(seg_m.lstrip()))
        lineno = line_of(code, first)
        above = comment_block_above(lines, comments, lineno)
        trailing = comments.get(lineno, "")
        parts = [squash(" ".join(x for x in above if x))] + ([trailing] if trailing else [])
        col["comment"] = " ".join(p for p in parts if p)
        if col["primary_key"]:
            constraints.append({"name": "", "kind": "PRIMARY KEY",
                                "definition": f"PRIMARY KEY ({col['name']})"})
        columns.append(col)
    return columns, constraints


# --------------------------------------------------------------------------------------------
# One service: replay every migration in order.
# --------------------------------------------------------------------------------------------

def new_table(name: str, source: str) -> dict:
    return {"name": name, "description": "", "description_full": "", "entity": "", "scope": "",
            "table_comment": "",
            "columns": [], "constraints": [], "indexes": [], "triggers": [],
            "partition_by": "", "partitions": "", "created_in": source, "altered_in": [],
            "dropped_in": ""}


def apply_alter(t: dict, code: str, masked: str, a: int, b: int, source: str,
                warnings: list[str]) -> str | None:
    """Apply one ALTER TABLE action. Returns a new table name on RENAME TO."""
    seg_c, seg_m = code[a:b], masked[a:b]
    up = squash(seg_m).upper()
    cols = {c["name"]: c for c in t["columns"]}

    m = re.match(rf"\s*ADD\s+COLUMN\s+(?:IF\s+NOT\s+EXISTS\s+)?", seg_m, re.I)
    if m:
        col = parse_column(seg_c[m.end():], seg_m[m.end():])
        if col and col["name"] not in cols:
            col["comment"] = f"(thêm ở {source})"
            t["columns"].append(col)
        return None
    m = re.match(r"\s*ADD\s+", seg_m, re.I)
    if m:
        t["constraints"].append(parse_constraint(seg_c[m.end():], seg_m[m.end():]))
        return None
    m = re.match(rf"\s*DROP\s+COLUMN\s+(?:IF\s+EXISTS\s+)?({IDENT})", seg_m, re.I)
    if m:
        t["columns"] = [c for c in t["columns"] if c["name"] != unquote(m.group(1))]
        return None
    m = re.match(rf"\s*DROP\s+CONSTRAINT\s+(?:IF\s+EXISTS\s+)?({IDENT})", seg_m, re.I)
    if m:
        t["constraints"] = [c for c in t["constraints"] if c["name"] != unquote(m.group(1))]
        return None
    m = re.match(rf"\s*ALTER\s+(?:COLUMN\s+)?({IDENT})\s+", seg_m, re.I)
    if m:
        col = cols.get(unquote(m.group(1)))
        rest_c, rest_u = seg_c[m.end():], squash(seg_m[m.end():]).upper()
        if col is None:
            warnings.append(f"{source}: ALTER cột không có trong bảng {t['name']}: {m.group(1)}")
        elif rest_u.startswith("SET NOT NULL"):
            col["nullable"] = False
        elif rest_u.startswith("DROP NOT NULL"):
            col["nullable"] = True
        elif rest_u.startswith("SET DEFAULT"):
            col["default"] = squash(re.sub(r"(?i)^\s*SET\s+DEFAULT", "", rest_c))
        elif rest_u.startswith("DROP DEFAULT"):
            col["default"] = None
        elif rest_u.startswith(("TYPE", "SET DATA TYPE")):
            new_type = re.sub(r"(?i)^\s*(SET\s+DATA\s+)?TYPE\s+", "", rest_c)
            col["type"] = squash(re.split(r"(?i)\bUSING\b", new_type)[0])
        else:
            warnings.append(f"{source}: ALTER COLUMN chưa hiểu: {squash(seg_c)[:80]}")
        return None
    m = re.match(rf"\s*RENAME\s+COLUMN\s+({IDENT})\s+TO\s+({IDENT})", seg_m, re.I)
    if m:
        col = cols.get(unquote(m.group(1)))
        if col:
            col["name"] = unquote(m.group(2))
        return None
    m = re.match(rf"\s*RENAME\s+CONSTRAINT\s+({IDENT})\s+TO\s+({IDENT})", seg_m, re.I)
    if m:
        for c in t["constraints"]:
            if c["name"] == unquote(m.group(1)):
                c["name"] = unquote(m.group(2))
        return None
    m = re.match(rf"\s*RENAME\s+TO\s+({IDENT})", seg_m, re.I)
    if m:
        return unquote(m.group(1))
    if re.match(r"(ENABLE|DISABLE|OWNER|SET|RESET|ATTACH|DETACH|FORCE|NO FORCE|VALIDATE)\b", up):
        return None
    warnings.append(f"{source}: ALTER TABLE chưa hiểu: {squash(seg_c)[:80]}")
    return None


def parse_service(service_dir: str) -> dict:
    service = os.path.basename(service_dir)
    files = sorted(glob.glob(os.path.join(service_dir, "migrations", "*.sql")))
    tables: dict[str, dict] = {}
    partitions_of: dict[str, list[str]] = {}
    warnings: list[str] = []

    for path in files:
        rel = os.path.relpath(path, ROOT).replace("\\", "/")
        fname = os.path.basename(path)
        with open(path, encoding="utf-8") as fh:
            sql = fh.read()
        code, masked, comments, literals = lex(sql)
        lines = sql.split("\n")
        events: list[tuple[int, str, re.Match]] = []
        patterns = {
            "create": rf"\bCREATE\s+(?:UNLOGGED\s+)?TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?({QNAME})",
            "alter": rf"\bALTER\s+TABLE\s+(?:IF\s+EXISTS\s+)?(?:ONLY\s+)?({QNAME})",
            "drop": rf"\bDROP\s+TABLE\s+(?:IF\s+EXISTS\s+)?({QNAME})",
            "index": rf"\bCREATE\s+(UNIQUE\s+)?INDEX\s+(?:CONCURRENTLY\s+)?(?:IF\s+NOT\s+EXISTS\s+)?"
                     rf"({IDENT})?\s*ON\s+(?:ONLY\s+)?({QNAME})",
            "dropindex": rf"\bDROP\s+INDEX\s+(?:CONCURRENTLY\s+)?(?:IF\s+EXISTS\s+)?({QNAME})",
            "trigger": rf"\bCREATE\s+(?:OR\s+REPLACE\s+)?(?:CONSTRAINT\s+)?TRIGGER\s+({IDENT})",
            "droptrigger": rf"\bDROP\s+TRIGGER\s+(?:IF\s+EXISTS\s+)?({IDENT})\s+ON\s+({QNAME})",
            "comment": rf"\bCOMMENT\s+ON\s+(TABLE|COLUMN)\s+({QNAME}(?:\.{IDENT})?)\s+IS\b",
        }
        for kind, pat in patterns.items():
            for m in re.finditer(pat, masked, re.I):
                events.append((m.start(), kind, m))
        events.sort(key=lambda e: e[0])

        for pos, kind, m in events:
            end = statement_end(masked, pos)
            src = f"{fname}:{line_of(sql, pos)}"
            if kind == "create":
                name = unquote(m.group(1))
                after = masked[m.end():end]
                po = re.match(rf"\s*PARTITION\s+OF\s+({QNAME})", after, re.I)
                if po:
                    partitions_of.setdefault(unquote(po.group(1)), []).append(name)
                    continue
                op = masked.find("(", m.end())
                if op < 0 or op > end:
                    continue
                cl = matching_paren(masked, op)
                t = tables.get(name) or new_table(name, src)
                if name in tables:
                    # IF NOT EXISTS re-declaration of a live table: it changes nothing.
                    continue
                desc, full, entity, scope = describe_table(
                    comment_block_above(lines, comments, line_of(sql, pos)), name)
                t.update(description=desc, description_full=full, entity=entity, scope=scope)
                t["columns"], t["constraints"] = parse_create_table(
                    code, masked, op, cl, lines, comments)
                pb = re.search(r"\bPARTITION\s+BY\s+(.+)", code[cl + 1:end], re.I | re.S)
                if pb:
                    t["partition_by"] = squash(pb.group(1))
                tables[name] = t
            elif kind == "alter":
                name = unquote(m.group(1))
                t = tables.get(name)
                if t is None:
                    warnings.append(f"{src}: ALTER TABLE {name} — bảng chưa được tạo trong service")
                    continue
                if src not in t["altered_in"] and fname not in t["altered_in"]:
                    t["altered_in"].append(fname)
                for a, b in split_top(masked, m.end(), end):
                    renamed = apply_alter(t, code, masked, a, b, fname, warnings)
                    if renamed:
                        tables[renamed] = tables.pop(name)
                        t["name"] = renamed
                        name = renamed
            elif kind == "drop":
                name = unquote(m.group(1))
                if name in tables:
                    tables[name]["dropped_in"] = src
            elif kind == "index":
                table = unquote(m.group(3))
                t = tables.get(table)
                if t is None:
                    continue
                iname = unquote(m.group(2)) if m.group(2) else ""
                t["indexes"] = [i for i in t["indexes"] if not iname or i["name"] != iname]
                t["indexes"].append({
                    "name": iname,
                    "unique": bool(m.group(1)),
                    "definition": squash(code[m.end():end]),
                    "source": src,
                })
            elif kind == "dropindex":
                iname = unquote(m.group(1))
                for t in tables.values():
                    t["indexes"] = [i for i in t["indexes"] if i["name"] != iname]
            elif kind == "trigger":
                stmt = code[m.end():end]
                on = re.search(rf"\bON\s+({QNAME})", masked[m.end():end], re.I)
                if not on:
                    continue
                t = tables.get(unquote(on.group(1)))
                if t is None:
                    continue
                tname = unquote(m.group(1))
                when = squash(stmt[:on.start()])
                fn = re.search(rf"EXECUTE\s+(?:FUNCTION|PROCEDURE)\s+({QNAME})", stmt, re.I)
                t["triggers"] = [x for x in t["triggers"] if x["name"] != tname]
                t["triggers"].append({"name": tname, "when": when,
                                      "function": unquote(fn.group(1)) if fn else ""})
            elif kind == "droptrigger":
                t = tables.get(unquote(m.group(2)))
                if t is not None:
                    t["triggers"] = [x for x in t["triggers"] if x["name"] != unquote(m.group(1))]
            elif kind == "comment":
                target = m.group(2).replace('"', "").split(".")
                text = squash(string_literals(code[m.end():end]))
                if m.group(1).upper() == "TABLE":
                    t = tables.get(target[-1])
                    if t:
                        t["table_comment"] = text
                else:
                    t = tables.get(target[-2]) if len(target) >= 2 else None
                    col = next((c for c in t["columns"] if c["name"] == target[-1]), None) if t else None
                    if col:
                        col["comment"] = (col["comment"] + " · " if col["comment"] else "") + \
                            f"COMMENT ON: {text}"

        # Partitions created in a loop by EXECUTE format('… PARTITION OF x ' 'FOR VALUES WITH
        # (MODULUS n …') — the literal is usually split in two, hence the quotes allowed between.
        for pm in re.finditer(rf"PARTITION\s+OF\s+({IDENT})[\s']*FOR\s+VALUES\s+WITH\s*\(\s*MODULUS\s+(\d+)",
                              code, re.I):
            t = tables.get(unquote(pm.group(1)))
            if t is not None and not t["partitions"]:
                t["partitions"] = f"{pm.group(2)} phân vùng hash (tạo động)"
        # DDL hidden inside a string literal that is not a partition: said, not guessed.
        for a, b in literals:
            lit = sql[a:b]
            if re.search(r"\b(CREATE|ALTER)\s+TABLE\b", lit, re.I) and \
                    not re.search(r"\bPARTITION\s+OF\b", lit, re.I):
                warnings.append(f"{fname}:{line_of(sql, a)}: DDL động trong chuỗi — không phân tích")

    for parent, kids in partitions_of.items():
        if parent in tables and not tables[parent]["partitions"]:
            tables[parent]["partitions"] = f"{len(kids)} phân vùng"

    live = [t for t in tables.values() if not t["dropped_in"]]
    dropped = [t["name"] for t in tables.values() if t["dropped_in"]]
    return {
        "service": service,
        "fingerprint": fingerprint(files),
        "generated_at": dt.datetime.now().astimezone().isoformat(timespec="seconds"),
        "parser_version": PARSER_VERSION,
        "migration_count": len(files),
        "tables": sorted(live, key=lambda t: t["name"]),
        "dropped_tables": sorted(dropped),
        "warnings": warnings,
    }


# --------------------------------------------------------------------------------------------
# Cache.
# --------------------------------------------------------------------------------------------

def fingerprint(files: list[str]) -> str:
    h = hashlib.sha256(f"parser:{PARSER_VERSION}\n".encode())
    for p in files:
        h.update(os.path.basename(p).encode() + b"\0")
        with open(p, "rb") as fh:
            h.update(fh.read())
        h.update(b"\0")
    return h.hexdigest()


def cache_path(service: str) -> str:
    return os.path.join(CACHE_DIR, f"{service}.json")


def load_or_build(service_dir: str, reload: bool) -> tuple[dict, str]:
    """(model, origin) — origin is 'scan', 'cache' or 'stale-cache'."""
    service = os.path.basename(service_dir)
    path = cache_path(service)
    if not reload and os.path.exists(path):
        try:
            with open(path, encoding="utf-8") as fh:
                model = json.load(fh)
            files = sorted(glob.glob(os.path.join(service_dir, "migrations", "*.sql")))
            fresh = model.get("fingerprint") == fingerprint(files)
            return model, "cache" if fresh else "stale-cache"
        except (OSError, ValueError):
            pass  # unreadable cache: rebuild, exactly as if it were absent
    model = parse_service(service_dir)
    os.makedirs(CACHE_DIR, exist_ok=True)
    with open(path, "w", encoding="utf-8") as fh:
        json.dump(model, fh, ensure_ascii=False, indent=2)
    return model, "scan"


# --------------------------------------------------------------------------------------------
# Rendering.
# --------------------------------------------------------------------------------------------

def cell(text: str | None, limit: int = 0) -> str:
    s = (text or "").replace("|", "\\|")
    if limit and len(s) > limit:
        s = s[:limit - 1] + "…"
    return s


def render(model: dict, origin: str, brief: bool) -> str:
    out = []
    origin_text = {"scan": "vừa quét lại", "cache": "bản lưu local (còn khớp migration)",
                   "stale-cache": "⚠ BẢN LƯU CŨ — migration đã đổi, chạy lại với --reload"}[origin]
    out.append(f"## {model['service']} — {len(model['tables'])} bảng · "
               f"{model['migration_count']} migration · {origin_text} · {model['generated_at']}")
    out.append("")
    out.append("| Bảng | Entity | Phạm vi | Cột | Chức năng |")
    out.append("|---|---|---|---|---|")
    for t in model["tables"]:
        purpose = t["table_comment"] or t["description"] or "_(chưa có mô tả trong migration)_"
        out.append(f"| `{t['name']}` | {cell(t['entity'])} | {cell(t['scope'])} | "
                   f"{len(t['columns'])} | {cell(purpose, 160)} |")
    if model["dropped_tables"]:
        out.append("")
        out.append("Bảng đã bị bỏ trong migration: " + ", ".join(f"`{x}`" for x in model["dropped_tables"]))
    if not brief:
        for t in model["tables"]:
            out.append("")
            out.append(f"### `{t['name']}`")
            purpose = t["table_comment"] or t["description"]
            out.append(purpose or "_(chưa có mô tả trong migration)_")
            meta = [f"tạo ở `{t['created_in']}`"]
            if t["altered_in"]:
                meta.append("sửa ở " + ", ".join(f"`{x}`" for x in t["altered_in"]))
            if t["partition_by"]:
                meta.append(f"PARTITION BY {t['partition_by']}"
                            + (f" — {t['partitions']}" if t["partitions"] else ""))
            out.append("")
            out.append(" · ".join(meta))
            out.append("")
            out.append("| Cột | Kiểu | NULL | Mặc định | Ghi chú |")
            out.append("|---|---|---|---|---|")
            for c in t["columns"]:
                flags = []
                if c["primary_key"]:
                    flags.append("PK")
                if c["unique"]:
                    flags.append("UNIQUE")
                if c["references"]:
                    flags.append(f"→ {c['references']}")
                note = " · ".join(flags + ([c["comment"]] if c["comment"] else []))
                out.append(f"| `{c['name']}` | {cell(c['type'])} | {'có' if c['nullable'] else 'không'} | "
                           f"{cell(c['default'], 40)} | {cell(note, 200)} |")
            if t["constraints"]:
                out.append("")
                out.append("Ràng buộc:")
                for k in t["constraints"]:
                    label = f"`{k['name']}` " if k["name"] else ""
                    out.append(f"- {label}{k['kind']}: {cell(k['definition'], 200)}")
            if t["indexes"]:
                out.append("")
                out.append("Chỉ mục:")
                for i in t["indexes"]:
                    u = "UNIQUE " if i["unique"] else ""
                    out.append(f"- {u}`{i['name']}` {cell(i['definition'], 200)}")
            if t["triggers"]:
                out.append("")
                out.append("Trigger:")
                for tr in t["triggers"]:
                    out.append(f"- `{tr['name']}` {tr['when']} → `{tr['function']}()`")
    if model["warnings"]:
        out.append("")
        out.append(f"Cảnh báo bộ phân tích ({len(model['warnings'])}) — phần này KHÔNG được phản ánh ở trên:")
        out.extend(f"- {w}" for w in model["warnings"])
    return "\n".join(out)


# --------------------------------------------------------------------------------------------
# Excel export. Reuses the standard-library xlsx writer of tools/xuat_tien_do.py — the repo's
# tools take no third-party package (Jenkins may not have openpyxl), and two hand-rolled xlsx
# writers would be two to keep valid.
# --------------------------------------------------------------------------------------------

XLSX_SHEETS = [
    ("1. Bảng", [("Service", 18), ("Bảng", 28), ("Entity", 24), ("Phạm vi", 12), ("Số cột", 8),
                 ("Chức năng", 70), ("Tạo ở", 34), ("Sửa ở", 40), ("Phân vùng", 34)]),
    ("2. Cột", [("Service", 18), ("Bảng", 28), ("#", 5), ("Cột", 26), ("Kiểu", 18), ("NULL", 7),
                ("Mặc định", 20), ("PK", 5), ("UNIQUE", 8), ("Tham chiếu", 28), ("Ghi chú", 70)]),
    ("3. Ràng buộc & chỉ mục", [("Service", 18), ("Bảng", 28), ("Loại", 18), ("Tên", 40),
                                ("Định nghĩa", 90)]),
]


def default_xlsx_path(service: str | None) -> str:
    suffix = f"-{service}" if service else ""
    return os.path.join(CACHE_DIR, f"db-tables{suffix}-{dt.date.today().isoformat()}.xlsx")


def export_xlsx(models: list[tuple[dict, str]], path: str) -> tuple[int, list[str]]:
    """Write the workbook. Returns (exit_code, messages); nothing is written on a non-zero code."""
    sys.path.insert(0, os.path.join(ROOT, "tools"))
    sys.path.insert(0, os.path.join(ROOT, ".claude", "hooks"))
    import xuat_tien_do as x  # noqa: E402

    # A generated file must never land in git (same refusal as /tien-do-san-pham --excel).
    if x.git_bo_qua(path) is False:
        return 4, [f"{path} nằm trong kho và git KHÔNG bỏ qua nó. Để mặc định (tmp/) "
                   "hoặc chọn một đường ngoài kho."]

    k = x.KieuO()
    sheets = [x.Sheet(name, cols, tab, k)
              for (name, cols), tab in zip(XLSX_SHEETS, (x.NAVY, x.TEAL, "548235"))]
    stale = [m["service"] for m, o in models if o == "stale-cache"]
    warnings = [f"{m['service']}: {w}" for m, _ in models for w in m["warnings"]]
    total = sum(len(m["tables"]) for m, _ in models)
    note = (f"{len(models)} service · {total} bảng · sinh từ service-*/migrations/*.sql lúc "
            f"{dt.datetime.now().astimezone().isoformat(timespec='seconds')}. "
            "Chức năng = chú thích trong migration, không ai viết thêm.")
    if stale:
        note += f" ⚠ BẢN LƯU CŨ hơn migration: {', '.join(stale)} — chạy lại với --reload."
    if warnings:
        note += f" ⚠ {len(warnings)} câu lệnh bộ phân tích chưa hiểu — xem cuối sheet 1."

    s1, s2, s3 = sheets
    s1.tieu_de("Bảng dữ liệu theo service", note)
    s2.tieu_de("Cột của từng bảng", note)
    s3.tieu_de("Ràng buộc, chỉ mục, trigger", note)
    for s in sheets:
        s.dong_bang = f"C{s.r + 1}"
        s.loc_start = s.r
        s.dau_bang([t for t, _w in s.cot])

    for m, _origin in models:
        svc = m["service"]
        for t in m["tables"]:
            partition = t["partition_by"] + (f" — {t['partitions']}" if t["partitions"] else "")
            s1.dong([svc, t["name"], t["entity"], t["scope"], len(t["columns"]),
                     t["table_comment"] or t["description"] or "(chưa có mô tả trong migration)",
                     t["created_in"], ", ".join(t["altered_in"]), partition], giua=(5,))
            for i, c in enumerate(t["columns"], 1):
                s2.dong([svc, t["name"], i, c["name"], c["type"], "có" if c["nullable"] else "không",
                         c["default"] or "", "✓" if c["primary_key"] else "",
                         "✓" if c["unique"] else "", c["references"] or "", c["comment"]],
                        giua=(3, 6, 8, 9))
            for ct in t["constraints"]:
                s3.dong([svc, t["name"], ct["kind"], ct["name"], ct["definition"]])
            for ix in t["indexes"]:
                s3.dong([svc, t["name"], "UNIQUE INDEX" if ix["unique"] else "INDEX", ix["name"],
                         ix["definition"]])
            for tr in t["triggers"]:
                s3.dong([svc, t["name"], "TRIGGER", tr["name"], f"{tr['when']} → {tr['function']}()"])

    for s in sheets:
        s.loc = f"A{s.loc_start}:{x.ten_cot(s.nc - 1)}{max(s.r - 1, s.loc_start)}"
    if warnings:
        s1.r += 1
        s1.muc("Cảnh báo bộ phân tích — phần này KHÔNG được phản ánh ở trên")
        for w in warnings:
            s1.ghi_chu(w, nghieng=False)

    # Rule 3: comments are free text written by people. Scan every cell with pii_guard's own
    # patterns; on a hit write nothing and name the cell, never its value.
    hits = [f"{s.ten}!{x.ten_cot(j - 1)}{r}" for s in sheets for (r, j), (v, _st) in s.o.items()
            if isinstance(v, str) and x.co_du_lieu_ca_nhan(v)]
    if hits:
        return 5, ["Có ô trông như số điện thoại / CCCD thật — KHÔNG ghi tệp. Ô: " + ", ".join(hits)]

    try:
        x.ghi_xlsx(path, sheets, k)
    except PermissionError:
        return 6, [f"Không ghi được {path} — tệp đang mở trong Excel? Đóng rồi chạy lại."]
    columns = sum(len(t["columns"]) for m, _ in models for t in m["tables"])
    return 0, [f"Đã ghi {os.path.abspath(path)} — {total} bảng, {columns} cột"]


# --------------------------------------------------------------------------------------------

def service_dirs() -> list[str]:
    return sorted(d for d in glob.glob(os.path.join(ROOT, "service-*"))
                  if os.path.isdir(os.path.join(d, "migrations")))


def resolve(arg: str, dirs: list[str]) -> list[str] | None:
    want = arg.strip().lower()
    if not want.startswith("service-"):
        want = "service-" + want
    hit = [d for d in dirs if os.path.basename(d).lower() == want]
    return hit or None


def main() -> int:
    ap = argparse.ArgumentParser(description="Liệt kê bảng theo service, đọc từ migration.")
    ap.add_argument("service", nargs="?", help="tên service (finance hoặc service-finance); bỏ trống = toàn bộ")
    ap.add_argument("--reload", action="store_true", help="bỏ qua bản lưu, quét lại và ghi đè")
    ap.add_argument("--brief", action="store_true", help="chỉ bảng tóm tắt, không in cột")
    ap.add_argument("--json", action="store_true", help="in mô hình JSON thay vì bảng")
    ap.add_argument("--excel", nargs="?", const="", metavar="ĐƯỜNG-DẪN.xlsx",
                    help="xuất ra Excel thay vì in; bỏ trống = tmp/db-tables/db-tables[-service]-<ngày>.xlsx")
    args = ap.parse_args()

    dirs = service_dirs()
    chosen = dirs
    if args.service:
        chosen = resolve(args.service, dirs)
        if not chosen:
            names = ", ".join(os.path.basename(d) for d in dirs)
            print(f"Không có service tên '{args.service}'. Có: {names}", file=sys.stderr)
            return 2

    stale = []
    models = []
    for d in chosen:
        model, origin = load_or_build(d, args.reload)
        models.append((model, origin))
        if origin == "stale-cache":
            stale.append(model["service"])

    if args.excel is not None:
        path = args.excel or default_xlsx_path(models[0][0]["service"] if args.service else None)
        code, messages = export_xlsx(models, path)
        for msg in messages:
            print(msg, file=sys.stderr if code else sys.stdout)
        if code:
            return code
    elif args.json:
        print(json.dumps([m for m, _ in models], ensure_ascii=False, indent=2))
    else:
        total = sum(len(m["tables"]) for m, _ in models)
        print(f"# Bảng dữ liệu — {len(models)} service · {total} bảng · nguồn: service-*/migrations/*.sql")
        print(f"Bản lưu: {os.path.relpath(CACHE_DIR, ROOT)}/<service>.json")
        for m, origin in models:
            print()
            print(render(m, origin, args.brief))
    if stale:
        print(f"\n⚠ Bản lưu cũ hơn migration: {', '.join(stale)}. "
              f"Chạy lại với --reload để quét và ghi đè.", file=sys.stderr)
        return 3
    return 0


if __name__ == "__main__":
    sys.exit(main())
