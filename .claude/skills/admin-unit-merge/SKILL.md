---
name: admin-unit-merge
description: Use when handling a merge, split, rename, or deactivation of an administrative unit. Triggers on: merge, split, rename commune, administrative unit, new tenant, onboard commune, offboard, dissolution, consolidation.
---

# Skill: Merging and splitting administrative units

Vietnam periodically reorganises commune-level administrative units. A platform serving this
level **will** encounter: two communes merging, one splitting, a rename, a commune becoming
a ward.

## The foundational decision — must be right from day one

> `tenant_id` is an **opaque, immutable identifier** (ULID). Commune name, administrative
> code, domain and active status are all **time-versioned attributes** of that tenant.

Choosing `tenant_id = "tan-phu"` or an administrative code means the first merger forces
**rewriting foreign keys across all historical data** — rewriting archival records. That
wrong choice costs **nothing** at the start and nearly **everything** later.

## Invariants when merging

| # | Invariant | Why |
|---|---|---|
| 1 | The old commune is **marked inactive**, never deleted | Archival records (rule 7) |
| 2 | Files keep their original `tenant_id` and the **issuing authority as of issuance** | Legal weight attaches to the issuing authority; it cannot be rewritten |
| 3 | Duplicate document numbers across the two old communes: **keep both**, distinguished by origin | Renumbering is not permitted |
| 4 | Lookups by an old code still resolve, annotated that the unit was merged | Citizens only have the old code |
| 5 | Whether old-commune staff keep access to old files is a **separate question** | A permissions question, not a data question |
| 6 | The new commune is a **new** tenant with a "merged from" mapping | Commune A does not "become" commune B |

## Stop condition — always

A merger is an **administrative decision**, not a technical operation. Every time: ask the
user about access to historical data, about what citizens should see, and about the cutover date.

→ Rule 1 · Rule 7 · `kb/00-foundation/open-questions.json` question 1
