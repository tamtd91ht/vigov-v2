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
| Stored once at intake | one `AdvanceWorkingHours` call returns **both** deadlines; store them, never recompute on read |
| Working **HOURS**, not days | ADR 0007 decided hours on 2026-09-16, and the difference is not cosmetic: `An ninh trật tự` must be **acknowledged in 2 working hours**. The smallest thing a count in days can express is one day — **four times** that promise (a working day is 8 hours), in the field where lateness matters most |
| **THREE** tables, not two | `lich_lam_viec` (the week, one row per SESSION so a lunch break is two rows) · `ngay_nghi_le` (closed) · `ngay_lam_bu` (**open although the week says otherwise** — the Prime Minister's annual swap days). Dropping the third counts straight through every swap day, and swap days cluster at Tết and National Day, when the backlog is largest |
| Do not compute it here | `identity` owns the three tables and answers `AdvanceWorkingHours` over gRPC. A second implementation is a second set of answers to one question |
| Received outside working hours | The clock starts at the **next session's opening** — ADR 0007 decision 8, settled 2026-09-20 |
| Per commune | every number and every table row is **tenant config**, not a constant |
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

**Open question #6 was DECIDED on 2026-09-16 — ADR 0007.** What it decided:

| | |
|---|---|
| Unit | **Working hours**, never days |
| Initial values | The **12** fields and their SLA table in `docs/ui-ux/14-cau-hinh.md` §8 — **seed data, not constants** |
| Ownership | Every number, and the field list itself, is **per-commune configuration** |
| Retroactivity | An SLA change is **not retroactive**. A petition keeps the deadline it was given at intake (rule 10, invariant 2) |

Two clocks per field, not one: **`tiep_nhan`** (acknowledge) and **`xu_ly_xong`** (resolve) —
table `sla(loai_viec, linh_vuc, gio_tiep_nhan, gio_xu_ly_xong, …)`, `linh_vuc = NULL` being the
default row. A single `deadline` field silently drops the acknowledge commitment, which is the
one the citizen feels first: for `An ninh trật tự` it is **2 working hours**.

**Do not copy the 12 names into source, and do not give the catalogue an owning service yet.**
ADR 0024 §"BA Ô ĐỂ TRỐNG" leaves this one deliberately unowned, waiting on open question **#4**:
if the district/province aggregates **by field** across 200+ communes, the codes must be a
**closed platform-level set** with a per-commune label on top — two layers, two owners, and the
commune may rename but not add. Building it as an ordinary per-commune catalogue first means
mapping 200 divergent code sets back to one by hand.

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

## ALREADY DECIDED — read the ADR, do not re-ask and do not hardcode

| Question | Answer | Where |
|---|---|---|
| #6 — fields and SLA | Working **hours**, per commune, not retroactive | ADR 0007 |
| #7 — who may close, is field acceptance mandatory | Permission `feedback.resolve`, plus two **per-commune flags**: `bat_buoc_nguoi_khac_dong` (default false) · `bat_buoc_anh_nghiem_thu` (default **true**) | ADR 0008 |
| #8 — may a citizen reopen | **Yes**, per-commune: `cho_phep_mo_lai` (true) · `nguong_sao_mo_lai` (2) · `so_lan_mo_lai_toi_da` (1) · `tinh_lai_han_khi_mo_lai` (true). Config changes are **not retroactive** | ADR 0008 |

All three were answered as **configuration**, not as constants. So the implementation rule is
the same in every case: read the commune's value, never write the default into a branch.

## STOP — ask, never decide alone

1. Changing how a deadline is **computed** (the formula, not a commune's number)
2. Adding or removing a **status**
3. Who owns the `Lĩnh vực phản ánh` catalogue — open **#4**, ADR 0024
4. Any behaviour ADR 0008 does **not** already express as a flag

→ Rule 10: `.claude/rules/critical/10-citizen-commitment.md`
→ Open question #4: `kb/00-foundation/open-questions.json`
→ Decisions: `kb/10-decisions/0007-sla-working-hours.md` · `0008-petition-lifecycle-config.md`
→ Terminology: `kb/00-foundation/ubiquitous-language.md`
