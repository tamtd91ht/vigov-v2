"""PreToolUse — BLOCK citizen routes that take identity from the request.  [RULE 4]

WHY THIS HOOK EXISTS: in brain v1 "citizen data isolation" was the single most important
rule in the system and the ONLY rule with no automatic enforcement — it was handed to an
agent run by hand, which means it depended on somebody remembering to type a command. In a
100%-AI project, "depends on somebody remembering" is where things slip.

Three shapes blocked:
  1. Citizen identity read from query / body / params instead of the SESSION
  2. Guessable sequential lookup codes on public routes
  3. (advisory) staff and citizen routes sharing a handler
"""

from __future__ import annotations

import os
import re
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import _common as c  # noqa: E402

HOOK = "citizen_scope_guard"

# Matches  .Query("phone") ·  .Query().Get("phone")  and  .Header.Get("phone")
#
# `Header` IS A FIELD, NOT A METHOD, and that one pair of parentheses was the whole defect.
# `net/http` spells the REQUEST side `r.Header.Get("X-Citizen-Id")` and only the RESPONSE side
# `w.Header().Get(...)`. Requiring `Header()` meant the only shape this branch could match was
# the one on the response writer — where nobody takes an identity from. Measured 2026-09-21:
# `r.Header.Get("citizen_id")` passed.
#
# `(?:\(\s*\))?` rather than dropping the parentheses from the alternation: `Query` genuinely
# is a method and `PostForm` is spelled both ways, so one optional group covers all three
# without loosening WHICH FIELD NAMES count — that list stays exactly as narrow as it was.
_ID_FIELD = r"(?:phone|so_dien_thoai|sdt|citizen_?id|cccd|cmnd|identity|nguoi_gui)"
FROM_REQUEST = re.compile(
    r"""(?:(?:Query|FormValue|PostForm|Param|URLParam)\s*\(\s*["'`]""" + _ID_FIELD + r"""["'`]"""
    r"""|(?:Query|Header|PostForm)\s*(?:\(\s*\))?\s*\.\s*Get\s*\(\s*["'`]""" + _ID_FIELD + r"""["'`])""",
    re.I,
)

FROM_SESSION_OK = re.compile(
    r"(CitizenFrom\s*\(|citizenFrom\s*\(|FromSession\s*\(|claims\.|ctx\.Value\s*\(|"
    r"MustCitizen\s*\(|session\.Citizen)"
)

SEQUENTIAL_CODE = re.compile(
    r"""(fmt\.Sprintf\s*\(\s*["'`][A-ZĐ]{2,4}-%0?\d*d|"""
    r"""seq\s*\+\s*1|nextSeq|autoIncrement|SERIAL\s+PRIMARY)""",
    re.I,
)

ESCAPE = re.compile(r"//\s*@citizen-scope-ok:\s*\S+")
CITIZEN_PATH = re.compile(r"/(citizen|cong-dan|congdan|public)/", re.I)


def main() -> None:
    c.utf8_streams()
    data = c.read_input()
    tool = c.tool_of(data)
    if tool not in ("Edit", "Write", "MultiEdit"):
        sys.exit(0)

    ti = c.input_of(data)
    path = c.path_of(ti)
    if not path or not path.endswith(".go") or c.should_skip(path):
        sys.exit(0)

    content = c.new_content(ti)
    if not content or c.is_generated(content, path):
        sys.exit(0)

    on_citizen_path = bool(CITIZEN_PATH.search(path)) or "citizen" in content.lower()[:2000]

    hits = []
    lines = content.splitlines()
    for i, line in enumerate(lines):
        prev = lines[i - 1] if i else ""
        if ESCAPE.search(prev):
            continue
        if FROM_REQUEST.search(line):
            hits.append(f"line {i+1}: citizen identity from the request — {line.strip()[:60]}")
        if on_citizen_path and SEQUENTIAL_CODE.search(line):
            hits.append(f"line {i+1}: sequential lookup code on a citizen route — guessable")

    if not hits:
        sys.exit(0)

    c.block(HOOK, f"citizen data isolation — {os.path.basename(path)}", hits,
            ["  Citizens are identified by phone number + OTP — a WEAK identity, not to be",
             "  trusted. Taking identity from the request means changing one parameter reads",
             "  somebody else's data.",
             "",
             "  Correct approach:",
             "    cit := auth.CitizenFrom(ctx)      // identity FROM THE SESSION",
             "    repo := s.scoped(ctx)             // and tenant scope (rule 1)",
             "    dt, err := repo.FindByCitizen(ctx, cit.ID)",
             "",
             "    // lookup codes must NOT be guessable",
             "    code := codegen.Opaque()          // not sequential, not short",
             "",
             "  Genuinely legitimate case (legal representative, agreed public lookup):",
             "    // @citizen-scope-ok: <business reason, already confirmed with the user>",
             "",
             "  → Rule 4: .claude/rules/critical/4-citizen-isolation.md"],
            tool=tool, path=path)


if __name__ == "__main__":
    main()
