---
name: cross-tenant-reporting
description: Use when building aggregate reporting across communes for district or province level, or any cross-commune read path. Triggers on: report, rollup, aggregate, district, province, multiple communes, cross-tenant, cross read, system-wide statistics, leadership dashboard.
---

# Skill: Cross-commune reporting, under control

Communes sit under districts, districts under provinces. As soon as several communes of one
district are live, the request *"let district leadership see feedback across all 20
communes"* **will** arrive. It is legitimate and within their authority.

But it is also the **one door the design deliberately opens through the isolation boundary** —
and therefore where every serious leak will pass.

## REQUIRED

| # | Invariant |
|---|---|
| 1 | Cross reads are **never the default** — declare `// @cross-tenant: <business reason>` |
| 2 | **Read only**, never write — a higher level does not process a lower level's files |
| 3 | **Aggregates only**: counts, sums, ratios. **Never** names, phone numbers, petition contents |
| 4 | The scope is derived **from the org tree**, never accepted from the client |
| 5 | **Every** cross read is audited: who, which communes, when, how many rows touched |
| 6 | Small-group threshold: groups under 5 records show no detail — individuals are re-identifiable |

## Why item 4 matters most

`?tenantIds=` sent from the browser is a **vulnerability, not a feature**: editing the
parameter reads communes outside one's authority. Scope must be derived server-side from the
**logged-in user's position** in the org tree.

## Stop condition

A request to see **record-level detail** (not aggregates) from another commune → **ask the
user**. That is an expansion of authority requiring an administrative basis, not a technical call.

→ Rule 1 · Rule 5 · Rule 6
