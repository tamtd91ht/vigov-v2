"""PreToolUse — BLOCK documentation inflation.  [RULE 9]

WHY BLOCK: AI writes documentation to PROVE it did work; humans write it to AVOID HAVING TO
REMEMBER. The first kind loses its value in a day but is never deleted, because it looks
like the second. Measured on a real ViGov project: 158 .md files · 803 KB · 38% of cited
paths already dead · 96% of plan files referenced by nothing · one convention spread across
24 files.

The consequence is not "a bit messy". Many copies of one fact drift apart; once they drift,
ALL of them lose credibility; once credibility is gone the agent ignores documentation and
goes back to scanning source. Documentation becomes pure cost. For a 100%-AI project that
is the dead end.

Five rules, each with an explicit way out.
"""

from __future__ import annotations

import io
import os
import re
import sys
import unicodedata

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import _common as c  # noqa: E402

HOOK = "doc_guard"

# Where documentation is ALLOWED
ALLOWED = (
    re.compile(r"(^|/)kb/"),
    # Bố cục phẳng: mỗi đơn vị triển khai nằm ở cấp một (`identity/`, `web-admin/`), nên
    # README của nó là `<đơn vị>/README.md`. Vẫn CHỈ một tệp README ở đúng cấp ấy — không mở
    # cửa cho .md rải rác bên trong (luật 9, cấm #1).
    re.compile(r"(^|/)[a-z0-9_\-]+/README\.md$"),
    re.compile(r"(^|/)\.claude/"),
    re.compile(r"(^|/)(README|CLAUDE|Makefile)\.md$"),
)

# docs/ui-ux/ — EDITING an existing file only, never creating a new one.
#
# tools/check_brain.py invariant 6 already names this directory as an exception: it holds the
# UI specification transcribed from an external running prototype, answering "what does the
# screen look like", which kb/ deliberately does not own. This hook did not know that, so the
# two halves of the brain disagreed — check_brain passed the directory while doc_guard refused
# every edit to it. A guard that contradicts the checker is the drift both exist to prevent.
#
# Narrower than check_brain on purpose, and the reason is in check_brain's own words: "a NAMED
# exception, not an open door". Editing the transcribed spec — correcting a stale field name,
# or REDACTING the real names and mobile numbers it arrived with (rule 3, forbidden #5) — is
# maintenance of a file that already exists. Creating a NEW .md there is how documentation
# starts scattering again, so that stays blocked.
SUA_DUOC = re.compile(r"(^|/)docs/ui-ux/[^/]+\.md$")

# GENERATED tiers — hand edits are silently lost on the next generate
GENERATED_TIERS = (re.compile(r"/kb/20-contracts/"), re.compile(r"/kb/30-indexes/"))

FM_REQUIRED = ("tier", "source", "owner")
TIER_T5 = re.compile(r"^tier:\s*T5\s*$", re.M)
HAS_EXPIRY = re.compile(r"^expires:\s*\d{4}-\d{2}-\d{2}\s*$", re.M)

FM_BLOCK = re.compile(r"\A---\r?\n(.*?)\r?\n---\r?\n", re.S)
OWNS = re.compile(r"^\s*-\s+(.+?)\s*$", re.M)


def norm(s: str) -> str:
    """Fold Vietnamese diacritics so fact matching is robust — kb prose stays Vietnamese.

    DECOMPOSE, do not enumerate. The previous version listed 19 characters by hand and
    Vietnamese has well over a hundred: every letter it missed fell through to the final
    `[^a-z0-9 ]` filter and became a SPACE. So a real fact folded to
    "v  sao platform khong toi du c th  m i host ..." — gaps where ì, ợ, ỗ used to be.
    It still matched itself, which is why nothing looked broken, but it could not reliably
    match the same sentence written anywhere else. The rule this function serves — one fact,
    one owner — was partially blind the whole time, and blind in a way that only ever produced
    silence.

    NFD splits a letter into base + combining mark; dropping category Mn leaves the base. `đ`
    is not a composition of anything, so it stays a table of one.
    """
    s = unicodedata.normalize("NFD", s.lower().strip())
    s = "".join(ch for ch in s if unicodedata.category(ch) != "Mn")
    s = s.replace("đ", "d")
    return re.sub(r"[^a-z0-9 ]+", " ", s).strip()


def tach_facts(fm: str) -> list:
    """Facts declared under owns_facts: in one frontmatter block.

    ONE parser for both callers on purpose. They used to differ by a single `.strip('"')`:
    owned_facts() stripped the quotes and the caller in main() did not, so the two sets could
    never match and the exemption they were meant to agree on was dead code. A rule split
    across two near-identical parsers is a rule that silently stops applying.
    """
    if "owns_facts:" not in fm:
        return []
    block = re.split(r"\n\w+:", fm.split("owns_facts:", 1)[1])[0]
    out = []
    for fact in OWNS.findall(block):
        fact = fact.strip().strip('"').strip("'")
        if len(fact) >= 12:
            out.append(fact)
    return out


def owned_facts(root: str) -> dict:
    """{normalised fact: owning file} — scans owns_facts frontmatter across kb/.

    NOTE the shape: one owner per fact, last writer wins. That is deliberate here, and it is
    also why the SECOND check in main() exists — this map alone cannot tell you that two files
    both claimed a fact, it can only tell you who claimed it most recently.
    """
    out = {}
    kb = os.path.join(root, "kb")
    if not os.path.isdir(kb):
        return out
    for dp, _, fn in os.walk(kb):
        for f in fn:
            if not f.endswith(".md"):
                continue
            p = os.path.join(dp, f)
            try:
                head = io.open(p, encoding="utf-8", errors="ignore").read(3000)
            except Exception:
                continue
            m = FM_BLOCK.search(head)
            if not m:
                continue
            for fact in tach_facts(m.group(1)):
                out[norm(fact)] = os.path.relpath(p, root).replace("\\", "/")
    return out


def main() -> None:
    c.utf8_streams()
    data = c.read_input()
    tool = c.tool_of(data)
    if tool not in ("Edit", "Write", "MultiEdit"):
        sys.exit(0)

    ti = c.input_of(data)
    path = c.path_of(ti)
    if not path.endswith((".md", ".mdx")):
        sys.exit(0)

    root = c.project_root().replace("\\", "/")
    rel = path
    if rel.lower().startswith(root.lower()):
        rel = rel[len(root):].lstrip("/")

    content = c.new_content(ti)
    if not content:
        sys.exit(0)

    # RULE 3 — GENERATED tier, never hand-edited
    if any(p.search("/" + rel) for p in GENERATED_TIERS):
        c.block(HOOK, f"hand-editing a GENERATED tier — {rel}",
                ["kb/20-contracts/ and kb/30-indexes/ are produced by `make kb`"],
                ["  Edits here are SILENTLY lost on the next generate.",
                 "",
                 "  Correct approach: edit the SOURCE and regenerate.",
                 "    contracts → edit .proto, run `buf generate`",
                 "    indexes   → edit the Go code, run `make kb`",
                 "  Recording something NOT derivable from code? That is a CURATED tier (kb/00-, kb/10-).",
                 "",
                 "  → Rule 9: .claude/rules/critical/9-knowledge-single-source.md"],
                tool=tool, path=path)

    # RULE 1 — the right place
    #
    # The docs/ui-ux/ exception applies only to a file that ALREADY EXISTS: maintaining the
    # transcribed spec is allowed, starting a new one there is not.
    da_co = SUA_DUOC.search("/" + rel) and os.path.exists(
        os.path.join(c.project_root(), rel.replace("/", os.sep)))
    if not da_co and not any(p.search("/" + rel) for p in ALLOWED):
        c.block(HOOK, f"documentation in the wrong place — {rel}",
                [".md files belong in kb/, <đơn vị>/README.md, or .claude/"],
                ["  Scattered documentation is documentation nobody reads. Pick the tier by the",
                 "  LIFETIME of the fact, not by topic:",
                 "",
                 "    kb/00-foundation/  T0  years        — domain, boundaries, invariants, why",
                 "    kb/10-decisions/   T1  permanent    — ADR: context, options, trade-offs",
                 "    kb/40-runbooks/    T4  per incident — runbooks by SYMPTOM",
                 "    kb/90-ephemeral/   T5  days-weeks   — plans, notes (expires: REQUIRED)",
                 "",
                 "  Session notes → kb/90-ephemeral/ with an expiry, or do not write them.",
                 "  Unsure which tier → STOP CONDITION, ask the user.",
                 "",
                 "  → Rule 9: .claude/rules/critical/9-knowledge-single-source.md"],
                tool=tool, path=path)

    if not rel.startswith("kb/"):
        sys.exit(0)

    # RULE 2 — frontmatter required
    m = FM_BLOCK.search(content)
    missing = [] if m else list(FM_REQUIRED)
    if m:
        missing = [k for k in FM_REQUIRED if not re.search(rf"^{k}:", m.group(1), re.M)]
    if missing:
        c.block(HOOK, f"missing frontmatter — {rel}",
                [f"missing: {', '.join(missing)}"],
                ["  Frontmatter turns STALENESS into a number instead of a feeling. Required shape:",
                 "",
                 "    ---",
                 "    id: <kebab-case>",
                 "    tier: T0                  # T0..T5 — by LIFETIME",
                 "    source: CURATED           # CURATED | GENERATED | DERIVED",
                 "    owner: architecture",
                 "    derived_from_commit: <sha>",
                 "    expires: null             # REQUIRED date for T5",
                 "    owns_facts:",
                 "      - \"<the fact THIS file is the source of truth for>\"",
                 "    ---",
                 "",
                 "  → Rule 9: .claude/rules/critical/9-knowledge-single-source.md"],
                tool=tool, path=path)

    # RULE 4 — T5 must expire
    if TIER_T5.search(m.group(1)) and not HAS_EXPIRY.search(m.group(1)):
        c.block(HOOK, f"ephemeral document with no expiry — {rel}",
                ["tier: T5 but expires is empty"],
                ["  Without an expiry it lives forever. Measured on a real project: 41 of 51 plan",
                 "  files belonged to finished work and were still there; 49 of 51 were orphans.",
                 "",
                 "  Set a real date: expires: 2026-10-15",
                 "  Once past, /knowledge-health reports it for cleanup.",
                 "",
                 "  → Rule 9: .claude/rules/critical/9-knowledge-single-source.md"],
                tool=tool, path=path)

    # RULE 5 — do not copy a fact that already has an owner
    facts = owned_facts(c.project_root())
    body_n = norm(content)
    mine = {norm(f) for f in tach_facts(m.group(1))}

    # RULE 4 — TWO FILES CLAIMING ONE FACT. Rule 9 names this as a STOP CONDITION in so many
    # words ("Two files both claiming ownership of one fact") and until now nothing checked it:
    # owned_facts() assigns into a dict, so the second claimant silently overwrote the first and
    # the map looked healthy.
    #
    # It bites hardest on ADRs, because an ADR is never edited. Two of them owning one decision
    # cannot be merged later — one has to be superseded, and by then both have been cited. This
    # session came within one instruction of writing a second `0012`.
    #
    # Checked BEFORE the copy rule below: a file declaring a fact it does not own would otherwise
    # be reported as "copying", which points at the wrong fix.
    tranh = [f"\"{f[:50]}\" — đã thuộc: {facts[f]}"
             for f in sorted(mine)
             if f in facts and facts[f].lower() != rel.lower()]
    if tranh:
        c.block(HOOK, f"two files claiming one fact — {rel}", tranh,
                ["  Rule 9 calls this a STOP CONDITION, not a style problem. Two owners means two",
                 "  copies that drift, and once they drift BOTH lose credibility — the agent stops",
                 "  trusting documentation and goes back to scanning source.",
                 "",
                 "  It is worst for an ADR, because an ADR is never edited. Two ADRs owning one",
                 "  decision cannot be merged afterwards: one must be superseded, and by then both",
                 "  have been cited somewhere.",
                 "",
                 "  Pick one:",
                 "    - this file owns it  -> remove the fact from owns_facts in the other file",
                 "    - the other owns it  -> drop it here and LINK instead",
                 "    - the two facts only LOOK alike -> word them so a reader can tell them apart;",
                 "      if you cannot, they are the same fact",
                 "",
                 "  Genuinely unsure which file should own it -> ask the user. Do not pick.",
                 "",
                 "  → Rule 9: .claude/rules/critical/9-knowledge-single-source.md"],
                tool=tool, path=path)

    dup = []
    for fact_n, owner in facts.items():
        if owner.lower() == rel.lower() or fact_n in mine:
            continue
        if fact_n and fact_n in body_n:
            dup.append(f"\"{fact_n[:50]}\" — owned by: {owner}")

    if dup:
        c.block(HOOK, f"copying a fact that already has an owner — {rel}", dup,
                ["  Two copies of one fact are two copies that WILL drift apart. Once they do,",
                 "  both lose credibility, the agent goes back to scanning source, and the",
                 "  documentation becomes pure cost.",
                 "",
                 "  Correct approach: LINK to the owning file, do not copy.",
                 "    Details: see `kb/00-foundation/domain-boundaries.md`",
                 "",
                 "  If THIS file should own it: declare it in owns_facts here and remove it there.",
                 "  Two files both claiming it → STOP CONDITION, ask the user.",
                 "",
                 "  → Rule 9: .claude/rules/critical/9-knowledge-single-source.md"],
                tool=tool, path=path)

    sys.exit(0)


if __name__ == "__main__":
    main()
