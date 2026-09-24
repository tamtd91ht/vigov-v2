---
description: Measure the health of the kb/ knowledge layer — dead links, orphans, expired, budget
group: Bộ não
allowed-tools: Read, Bash, Glob, Grep
---

# /knowledge-health

Documentation inflation is **irrecoverable damage at scale**: many copies of one fact drift
apart; once they drift, **all of them lose credibility**; once credibility is gone the agent
ignores documentation and goes back to scanning source. Documentation becomes pure cost.

What is not measured cannot be managed.

## Four metrics

| Metric | How | Threshold |
|---|---|---|
| **Dead links** | file paths cited by `kb/**` that do not exist | **0** |
| **Orphans** | `kb/` files absent from `INDEX.yaml` and referenced by nothing | **< 5%** |
| **Stale** | `derived_from_commit` more than 50 commits behind HEAD **and** a cited file changed since | **0** |
| **Budget** | total tokens in the always-loaded tier | **≤ 25,000** |

Plus two quick checks:

- T5 files past their `expires` date, not yet removed → **0**
- `.md` files outside `kb/`, `*/README.md`, `.claude/` → **0**

## Report

One line per metric with real numbers and the list of offending locations. End with a
concrete cleanup proposal, file by file.

**Propose deletions, never perform them** — deletion is destructive; the user confirms.
