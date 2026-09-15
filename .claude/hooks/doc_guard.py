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

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import _common as c  # noqa: E402

HOOK = "doc_guard"

# Where documentation is ALLOWED
ALLOWED = (
    re.compile(r"(^|/)kb/"),
    re.compile(r"(^|/)services/[a-z0-9_\-]+/README\.md$"),
    re.compile(r"(^|/)apps/[a-z0-9_\-]+/README\.md$"),
    re.compile(r"(^|/)\.claude/"),
    re.compile(r"(^|/)(README|CLAUDE|Makefile)\.md$"),
)

# GENERATED tiers — hand edits are silently lost on the next generate
GENERATED_TIERS = (re.compile(r"/kb/20-contracts/"), re.compile(r"/kb/30-indexes/"))

FM_REQUIRED = ("tier", "source", "owner")
TIER_T5 = re.compile(r"^tier:\s*T5\s*$", re.M)
HAS_EXPIRY = re.compile(r"^expires:\s*\d{4}-\d{2}-\d{2}\s*$", re.M)

FM_BLOCK = re.compile(r"\A---\r?\n(.*?)\r?\n---\r?\n", re.S)
OWNS = re.compile(r"^\s*-\s+(.+?)\s*$", re.M)


def norm(s: str) -> str:
    """Fold Vietnamese diacritics so fact matching is robust — kb prose stays Vietnamese."""
    s = s.lower().strip()
    for a, b in (("à", "a"), ("á", "a"), ("ả", "a"), ("ã", "a"), ("ạ", "a"),
                 ("ă", "a"), ("â", "a"), ("đ", "d"), ("ê", "e"), ("ô", "o"),
                 ("ơ", "o"), ("ư", "u"), ("é", "e"), ("è", "e"), ("ị", "i"),
                 ("ớ", "o"), ("ữ", "u"), ("ộ", "o"), ("ệ", "e")):
        s = s.replace(a, b)
    return re.sub(r"[^a-z0-9 ]+", " ", s).strip()


def owned_facts(root: str) -> dict:
    """{normalised fact: owning file} — scans owns_facts frontmatter across kb/."""
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
            if not m or "owns_facts:" not in m.group(1):
                continue
            block = m.group(1).split("owns_facts:", 1)[1]
            block = re.split(r"\n\w+:", block)[0]
            for fact in OWNS.findall(block):
                fact = fact.strip().strip('"').strip("'")
                if len(fact) >= 12:
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
    if not any(p.search("/" + rel) for p in ALLOWED):
        c.block(HOOK, f"documentation in the wrong place — {rel}",
                [".md files belong in kb/, services/<name>/README.md, or .claude/"],
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
    mine = {norm(f) for f in OWNS.findall(m.group(1))} if "owns_facts:" in m.group(1) else set()
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
