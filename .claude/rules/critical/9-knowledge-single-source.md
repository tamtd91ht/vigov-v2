# RULE 9 — One fact, one source

This project's code is written by AI, and AI has **no memory between sessions**. If
knowledge does not sit somewhere cheap and trustworthy to load, every session rediscovers
the system — expensive, slow, and **reaching a different conclusion each time**.

Documentation inflation is **irrecoverable damage at scale**: many copies of one fact drift
apart; once they drift, **all of them lose credibility**; once credibility is gone the agent
ignores documentation and goes back to scanning source — and documentation becomes pure cost.

## THREE QUESTIONS, THREE SOURCES

| Question | Source |
|---|---|
| **What · How** | Source code |
| **Where · How many** | Generated indexes (`kb/30-indexes/`) |
| **Why** | Hand-written documentation — **this question only** |

**One-line test:** *"If I delete this line, can a tool rebuild it from the source code?"*
**Yes** → hand-writing it is forbidden. **No** → this is exactly what is worth writing.

## INVARIANTS

| # | Invariant |
|---|---|
| 1 | **Never hand-write what can be generated**: service lists, endpoints, environment variables, directory trees, file paths |
| 2 | Every fact has **one owning file**, declared in `owns_facts`. Everywhere else **links**, never copies |
| 3 | Every file in `kb/` carries frontmatter: `tier`, `source`, `owner`, `derived_from_commit`, `expires` |
| 4 | **T5 files must carry an `expires` date.** Without one it lives forever |
| 5 | New documentation is created **only inside its tier**. No `.md` dropped wherever is convenient |
| 6 | Documentation is tiered by **lifetime**, not by topic |
| 7 | Missing knowledge means **telling the user** and proposing the right tier — never writing a new file to fill the gap |
| 8 | `kb/30-indexes/` is the **GENERATED** tier — edit the source and run `make kb`, never the output |

## STRICTLY FORBIDDEN

| # | Forbidden | Why |
|---|---|---|
| 1 | Creating `.md` outside `kb/`, `*/README.md`, `.claude/` | Scattered documentation is documentation nobody reads |
| 2 | Copying content that already has an owning file | Two copies are two copies that will drift |
| 3 | Writing documentation as a **session log** | Worthless after a day, yet never deleted |
| 4 | Hand-editing the generated tier | Silently lost on the next generate |

## STOP CONDITIONS

1. Needing a document whose **tier is unclear**
2. Two files both claiming ownership of one fact
3. Needing to record a contested architectural decision (→ ADR, `kb/10-decisions/`)

→ Enforcement: `hooks/doc_guard.py` (BLOCK)
→ Skill: `skills/write-knowledge`
→ Command: `/knowledge-health`
