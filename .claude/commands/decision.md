---
description: Record an architectural decision as an ADR under kb/10-decisions/
argument-hint: "<decision name, e.g. tenant_id is an opaque ULID>"
allowed-tools: Read, Grep, Glob, Bash, Write
---

# /decision

An ADR answers **WHY** — the one question neither the source code nor any tool can answer,
and the thing a future session needs most.

## When to write one

| Write | Do not write |
|---|---|
| The decision has a **real trade-off**; another option was also reasonable | Only one way to do it |
| The decision is **hard to reverse** | Easy to change later |
| The decision contradicts what a reader would assume by default | It follows the default |
| Somebody will ask about it again in six months | — |

## Template

```markdown
---
id: NNNN-<slug>
tier: T1
source: CURATED
owner: architecture
derived_from_commit: <sha>
expires: null
owns_facts:
  - "<this decision>"
---

# NNNN. <Decision name>

**Status:** proposed | accepted | superseded by [NNNN]
**Date:** YYYY-MM-DD

## Context
<What forced a decision. Which constraints are real.>

## Options
| Option | Gains | Costs |

## Decision
<What was chosen. One sentence.>

## Consequences
<What becomes easy. What becomes hard. What will have to be paid later.>
```

Prose inside the ADR is written in **Vietnamese** — it is documentation.

## Invariant

**An ADR is never edited and never deleted.** When a decision changes, write a new ADR and
mark the old one *"superseded by NNNN"*. Editing the old one erases why people thought
differently that day — which is the most valuable thing it holds.
