---
name: petition-lifecycle
description: Use when building or changing anything about citizen petitions (phản ánh) — intake, classification, assignment, processing, field acceptance, closing, citizen rating, deadlines and overdue state. Triggers on: phan anh, phản ánh, petition, feedback, kiến nghị, phiếu, SLA, deadline, hạn xử lý, quá hạn, overdue, tiếp nhận, phân loại, phân công, nghiệm thu, đóng phiếu, lĩnh vực, ngày làm việc, working days, tra cứu, lookup code.
---

# Skill: The petition lifecycle

A petition is the one object a **citizen** creates and then watches. It is the surface the
commune is judged on. Rule 10 states the invariants; this is how to implement them.

## The seven steps

| # | Step | Vietnamese | Who | Citizen notified | Audit |
|---|---|---|---|---|---|
| 1 | Intake | `tiep_nhan` | receptionist / auto from Mini App | **yes** — lookup code + deadline | yes |
| 2 | Classify | `phan_loai` | officer | no | yes |
| 3 | Assign | `phan_cong` | leader / officer | no | yes |
| 4 | Process | `dang_xu_ly` | assigned officer | **yes** — on entering | yes |
| 5 | Field acceptance | `nghiem_thu` | officer (usually with a photo) | no | yes |
| 6 | Close | `da_dong` | officer / leader | **yes** — with the result | yes |
| 7 | Citizen rating | `danh_gia` | citizen | — | yes |

**The deadline clock starts at step 1**, not at step 3. A petition sitting unassigned is
already consuming the commitment made to the citizen.

Steps 2, 3 and 5 are internal. The citizen sees **status + result**, never staff notes or
routing history (rule 4).

## Deadlines

| Rule | Implementation |
|---|---|
| Stored once at intake | `p.SLADeadline = sla.Deadline(ctx, receivedAt, p.Field)` |
| Working days only | skip Saturday, Sunday and the public-holiday table |
| Per commune | the holiday table and the per-field day counts are **tenant config**, not constants |
| Overdue is derived | `func (p Petition) IsOverdue(now) bool` — never a stored column |

```go
// Derive, never store. A stored flag is wrong the moment a job is late or a
// holiday is added, and the stale copy is the one that reaches a report.
func (p Petition) IsOverdue(now time.Time) bool {
    return p.ClosedAt.IsZero() && now.After(p.SLADeadline)
}
```

Statutory periods counted in **calendar** days (complaints, denunciations) are the exception
and must be declared: `// @sla-ok: <which law, which article>`.

## Fields (`linh_vuc`)

Rubbish · traffic · environmental sanitation · urban order · public security · construction ·
staff conduct · other.

**This list and its per-field day counts are open question #6 — do not hardcode them and do
not invent new ones.** They are per-commune configuration.

## Lookup codes

| Rule | Why |
|---|---|
| Not sequential, not short | A guessable code enumerates other citizens' petitions (rule 4) |
| Issued once, never reissued | Rule 7 — even after a soft delete |
| Printed on paper and read aloud | Avoid easily confused characters (`0/O`, `1/l/I`) |

## Notifying the citizen

Steps 1, 4 and 6 must notify. The notification carries: **what changed**, **what happens
next**, and **the lookup code**. It is written for a citizen, not a clerk — see
`skills/administrative-language` and `skills/accessibility-elderly`.

Closing with no readable result is forbidden (rule 10, invariant 6). "Đã xử lý" alone is not
a result; say what was actually done.

## STOP — ask, never decide alone

1. Changing a deadline formula, or any field's SLA
2. Adding or removing a status
3. Who may close a petition, and whether field acceptance is mandatory
4. Whether a citizen may reopen a closed petition

→ Rule 10: `.claude/rules/critical/10-citizen-commitment.md`
→ Open questions #6, #7, #8: `kb/00-foundation/open-questions.json`
→ Terminology: `kb/00-foundation/ubiquitous-language.md`
