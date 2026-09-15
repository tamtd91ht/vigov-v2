---
name: audit-trail
description: Use when writing a data-mutating operation, designing the audit trail, or deciding what must be recorded. Triggers on: audit, audit trail, history, who did what, business log, traceability, inspection, complaint, trace, change history.
---

# Skill: The audit trail

**An audit trail is not a technical log.** Logs exist for debugging, rotate by size, and may
be lost. The audit trail is **business data**: retained at least 12 months, never deleted,
never edited. When a complaint or an inspection arrives, this is what answers *"who did what,
and when"*.

## Entry shape

```go
type Entry struct {
    TenantID string    // which commune — rule 1
    Actor    string    // who (staff code, or "system")
    Action   string    // what — a BUSINESS verb, not a function name
    Subject  string    // on which record (BUSINESS code, never an internal id)
    At       time.Time
    FromIP   string
    Delta    []byte    // before/after values, personal data ALREADY MASKED
}
```

## REQUIRED

| # | Practice |
|---|---|
| 1 | The entry is written **in the same transaction** as the change |
| 2 | `Action` is a **business verb** (`accept_petition`), not a function name |
| 3 | `Subject` is a **business code** — an internal id means nothing to whoever reads the trail |
| 4 | Entries are **append-only**: never edited, never deleted, not even by an administrator |
| 5 | System actions are audited too, with a system principal |
| 6 | Reading **full** personal data, or reading **across communes**, is itself audited |

## Why it must share the transaction

Auditing "afterwards, if it works" means that sometimes the change succeeds and the entry
fails. The file has then changed and **nobody knows who changed it** — precisely what a
public authority is not allowed to have.

→ Rule 6 · `skills/transaction-boundary`
