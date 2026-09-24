---
description: Compliance review — administrative terminology, Vietnamese, accessibility, legal duties
group: Rà soát
argument-hint: "[scope: diff | service | all] — empty means diff"
allowed-tools: Read, Grep, Glob, Bash
---

# /review-compliance

Reviews what **a machine cannot enforce** — and therefore has no hook, following the brain's
invariant: *a rule you cannot enforce is not a rule*.

This command carries the work the rule tier deliberately declines.

## Four groups

### 1. Administrative terminology — getting it wrong is a business error

| Check | Why |
|---|---|
| Distinguish **feedback / complaint / denunciation** in field names, labels, messages | Mislabelling applies the **wrong procedure and the wrong statutory deadline** |
| Incoming vs outgoing documents — the correct terms | |
| Accept · route · issue — the correct administrative verbs | |
| Schema field names follow business terminology | Wrong at the schema level means wrong at every layer above; fixing it later is a data migration |

Cross-check `kb/00-foundation/ubiquitous-language.md` — the single authoritative table.

### 2. Vietnamese written for citizens

| Check | Threshold |
|---|---|
| Spelling and punctuation | 0 errors |
| Capitalisation of authority names | Per convention |
| Error messages say **what to do next**, not an error code | Every message |
| No abbreviations in text shown to citizens | 0 |

### 3. Accessibility

Citizens **do not choose** this software — being unable to use it means **being unable to
reach a public service**. That is a rights issue, not a UX issue.

Body text ≥ 16px · touch targets ≥ 44×44px · contrast ≥ 4.5:1 · status never conveyed by
colour alone · labels above inputs, never placeholder-only.

### 4. Legal duties

| Check | Basis |
|---|---|
| Personal data collected has a **clear purpose** and is the minimum necessary | Decree 13/2023 |
| A citizen erasure path exists — and it **anonymises**, never hard deletes | Decree 13/2023 + rule 7 |
| Audit retention ≥ 12 months | Rule 6 |
| No hard-delete path exists for archival records | Rule 7 |
| Document numbering is **per authority**, restarting at 01 each year | Records regulations |

## Report

| Group | Location | Severity | Fix |
|---|---|---|---|

Severity: **VIOLATION** (against regulation) · **WRONG** (terminology, spelling) · **DEBT**
(correct but hard to use).

End with: what **must be fixed before release**, and what can be carried as debt.

→ `skills/administrative-language` · `skills/accessibility-elderly` · `skills/mask-personal-data`
