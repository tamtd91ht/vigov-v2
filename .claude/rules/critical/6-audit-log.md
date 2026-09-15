# RULE 6 — Audit trail

Administrative files carry legal weight. When a complaint, dispute or inspection arrives,
the question is always **"who did what, and when"**. Not being able to answer is the
authority's problem.

An audit trail is **not a technical log**. Logs exist for debugging, rotate by size, and may
be lost. The audit trail is **business data**: retained at least 12 months, never deleted.

## INVARIANTS

| # | Invariant |
|---|---|
| 1 | Every **write** to business data leaves an entry |
| 2 | An entry carries: **who · what · on which record · when · from which IP · in which commune** |
| 3 | The entry is written **in the same transaction** as the change — never "afterwards, if it works" |
| 4 | Entries are **append-only**. Not editable, not deletable, not even by an administrator |
| 5 | Entries hold **before and after** values for significant fields — with personal data masked (rule 3) |
| 6 | System actions (background jobs, migrations) are audited too, with a system principal |
| 7 | Reading **full** personal data, or reading **across communes**, is itself audited |

## STRICTLY FORBIDDEN

| # | Forbidden | Why |
|---|---|---|
| 1 | Recording the trail with `log.Info(...)` | Logs rotate and are lost. The trail needs durable storage |
| 2 | Writing the entry outside the transaction | The change can succeed while the entry fails — a state the rules do not permit (rule 2) |
| 3 | Editing or deleting an audit entry | A trail that can be edited has no evidentiary value |
| 4 | Storing raw before/after values of personal-data fields | The audit log becomes a personal-data store |

## STOP CONDITIONS

1. A new operation where it is **unclear whether it must be audited**
2. A request to delete or edit audit entries
3. A request for a retention period other than 12 months

→ Enforcement: `hooks/audit_guard.py` (advisory)
→ Skill: `skills/audit-trail`
