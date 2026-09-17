---
name: write-knowledge
description: Use before writing any documentation — choosing the tier, the location, and whether to write it at all. Triggers on: write documentation, document, record, kb, ADR, README, documentation, update docs, notes, plan, report.
---

# Skill: Writing knowledge, not documentation

## Ask first

> **"If I delete this line, can a tool rebuild it from the source code?"**
> **Yes** → hand-writing it is forbidden. **No** → this is exactly what is worth writing.

| Question | Source of the answer |
|---|---|
| What · How | **Source code** |
| Where · How many | **Generated indexes** (`kb/30-indexes/`) |
| **Why** | **Hand-written documentation** — this question only |

AI inflates documentation because it hand-writes the answers to *"what"* and *"where"* — two
questions that code and tooling answer more accurately, more cheaply, and always correctly.
The hand-written copy is wrong by the next day.

## Choose the tier by LIFETIME, not by topic

| Tier | Lifetime | Contents |
|---|---|---|
| `kb/00-foundation/` T0 | Years | Domain, boundaries, invariants, **why the cut is there** |
| `kb/10-decisions/` T1 | Permanent | ADRs — never edited, never deleted |
| `kb/30-indexes/` T3 | Per commit | **GENERATED** — never hand-edited |
| `kb/20-contracts/` T2 | Per API version | **GENERATED** — `openapi.json` from `tools/apidoc`. The REST surface only; `proto/` owns the contract between services |
| _`kb/40-runbooks/` T4_ | Per incident | **Not created yet** — create on the first real incident |
| `kb/90-ephemeral/` T5 | Days–weeks | Session handovers. **`expires` is mandatory** — a handover with no expiry is read as fact six months on |

A tier in italics does not exist on disk. Create it when there is a real first occupant —
**never an empty directory just to complete the set** (that is inflation, invariant 7).

Organising by topic puts a fact with a five-year lifetime next to one with a five-day
lifetime in the same file. The mechanical consequence: any single line changing makes **the
whole file suspect**; re-verifying the whole file is too expensive so nobody does it; and the
file rots entirely — including the part that would have been correct for years.

## REQUIRED

| # | Practice |
|---|---|
| 1 | Every fact has **one owning file**, declared in `owns_facts`. Everywhere else **links**, never copies |
| 2 | Complete frontmatter: `tier`, `source`, `owner`, `derived_from_commit`, `expires` |
| 3 | T5 files carry a **real** expiry date |
| 4 | Missing knowledge → **tell the user**; never write a new file to fill the gap |
| 5 | Prose inside `kb/` is written in **Vietnamese** — the supervisors read Vietnamese |

## What not to write

Session logs · restatements of the code · narratives of how the work went · lists derivable
from the code · "summary documents" that gather what already exists elsewhere.

→ Rule 9 · `/knowledge-health` · `/decision`
