# RULE 10 — The commitment made to the citizen

A petition (`phan_anh`) is the one object in this system a **citizen** creates and then
watches. Everything else — documents, dossiers, disbursements — is staff-facing. This is the
surface the commune is judged on, and the reason most citizens open the app at all.

A processing deadline is not a UI nicety: it is a **commitment by a public authority**,
counted in **working days**, and it differs per commune and per field. Getting it wrong
produces a figure reported to leadership that is simply false — and the citizen who was told
"within 3 days" is the one who finds out first.

## INVARIANTS

| # | Invariant |
|---|---|
| 1 | Every petition gets a **lookup code** returned to the citizen the moment it is received |
| 2 | `sla_deadline` is computed **once, at intake**, and stored. Never recomputed on read |
| 3 | **Overdue is DERIVED** from `sla_deadline` vs now — never a hand-set column or flag |
| 4 | Deadlines count **working days**, from **per-commune** configuration (rule 1, invariant 10) |
| 5 | Every status transition **notifies the citizen** and leaves an audit entry (rule 6) |
| 6 | Closing a petition records a **result the citizen can read**. Never close silently |
| 7 | The citizen sees their **own** petition's progress only — staff notes and routing history stay internal (rule 4) |

**Why #2 and #3 are not contradictory:** the *deadline* is **stored**, the *overdue state* is
**derived**. The deadline is an obligation fixed at intake — recomputing it later silently
moves a commitment already made to a citizen. The state is only a comparison against that
fixed point.

**Why invariant #3 is the expensive one:** an `is_overdue` column set by a nightly job is
wrong the moment the job is late, the clock skews, or a holiday is added. Two sources for one
fact drift, and the stale one is what reaches the report going upward. Derive it and it
cannot drift.

**Why invariant #5 matters more than it looks:** a citizen who is not told cannot tell the
difference between "being processed" and "ignored". Silence is how trust in the channel dies,
and a channel nobody trusts stops receiving the reports the commune actually needs.

## STRICTLY FORBIDDEN

| # | Forbidden | Why |
|---|---|---|
| 1 | An `is_overdue` / `overdue` column or struct field that is written to | Duplicates a derivable fact (invariant 3) |
| 2 | Counting a deadline in calendar days (`AddDate(0, 0, n)`, `24 * time.Hour * n`) | Weekends and public holidays are not working days |
| 3 | A commune's SLA figures hardcoded in source | One codebase serves many communes (rule 1) |
| 4 | Changing status without notifying the citizen | The commitment is the notification |
| 5 | A lookup code that is sequential or short enough to enumerate | Rule 4, invariant 4 |

## STOP CONDITIONS — ask the user, never decide alone

1. Changing how a deadline is calculated, or the SLA for any field
2. Adding or removing a **status** in the petition lifecycle
3. Closing a petition **without** notifying the citizen, for any reason
4. Deciding **who** may close a petition, or whether field acceptance is mandatory

→ Enforcement: `hooks/citizen_commitment_guard.py` (BLOCK)
→ Skill: `skills/petition-lifecycle`
→ Open questions: `kb/00-foundation/open-questions.json` #6, #7, #8
