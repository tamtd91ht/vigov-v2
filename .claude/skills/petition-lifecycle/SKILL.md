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
| **Where `count_from` is** | The moment the **citizen pressed send** — for **both** clocks, ADR 0027 decision D. `da-tiep-nhan` is written by the software, so counting to it measures the system against itself |
| **What stops the acknowledge clock** | The first act by a **human**: entering `dang-phan-loai`. Not the creation of the row |
| **Field changed at classification** | The deadline may only be **SHORTENED**: take the earlier of *the deadline already promised* and *the one the new field yields from the same `count_from`* — ADR 0027 decision C. Never the later one |
| Per commune | every number and every table row is **tenant config**, not a constant |
| Overdue is derived | `func (p Petition) IsOverdue(now) bool` — never a stored column |

Decision C is **not** a breach of rule 10 invariant 2. That invariant forbids recomputing on
**read**; here a stored value is lowered by a human act, in the same transaction as the act,
with before/after in the audit entry. ADR 0008 already set the precedent: `tinh_lai_han_khi_mo_lai`
writes a new deadline when a petition is reopened.

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

**The catalogue has TWO LAYERS — settled 2026-09-20, ADR 0026.** The customer confirmed that the
province aggregates **by field** across 200+ communes, so the codes are a **closed set granted by
the platform** and the commune may only **rename** them. It is not an ordinary per-commune
catalogue, and the seven catalogues of ADR 0024 are not a template for it.

| Layer | Who may change it | Where |
|---|---|---|
| The code set | **not the commune** | service still **undecided** — open question **#22** |
| The label | the commune, `(tenant_id, ma)` | `petitions` |

**Still do not write the migration.** Open **#22** asks who may add a 13th code and how, and that
answer names the service and the read path (gRPC, events, or neither). ADR 0024 stop condition 3
still applies to this exact table.

**The trap this creates — read before writing the intake path.** The commune's officer decides the
field at classification, so on the citizen channel nothing is known at intake and the deadline can
only come from the default row (8h / 56h). Combined with decision C (shorten only), **the default
row becomes a ceiling**: the six SLA rows longer than 56h, and the three 2-hour acknowledge rows,
are unreachable for a petition the citizen submitted. A petition recorded by staff (`nhập hộ`) picks
the field at intake, so it gets the full table — two deadlines for one pothole. That is open
question **#23**, and none of the three ways out is the agent's to pick.

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
| A — who owns `Lĩnh vực phản ánh` | **Two layers**: a closed code set granted by the platform, a per-commune label on top. The commune renames, never adds a code | ADR 0026 |
| B — which statuses exist | **Nine, a closed list.** The commune adds none and removes none, so the state machine may name the codes directly. Codes and labels: `kb/00-foundation/ubiquitous-language.md` | ADR 0027 |
| C — field changed at classification | The deadline **only shortens** | ADR 0027 |
| D — when the clock starts | The citizen's send, for **both** clocks; the acknowledge clock stops at the first staff act | ADR 0027 |

Those first three were answered as **configuration**, not as constants. So the implementation rule
is the same in every case: read the commune's value, never write the default into a branch.

The four decisions of 2026-09-20 went the **other** way — fixed, not configurable — and that is
the point, not an inconsistency. What a commune may shape is how it works inside its own walls
(#6, #7, #8). What is fixed is what leaves the commune: a number that must add up with 200 other
communes (A), a lifecycle a commune must not be able to stop (B), and a promise already spoken to
a citizen (C, D).

## STOP — ask, never decide alone

1. Changing how a deadline is **computed** (the formula, not a commune's number)
2. Adding or removing a **status** — the list is closed by ADR 0027, which makes this a change
   to a decision, not a gap to fill
3. Writing the migration for the platform-level field code table — open **#22**
4. Letting the citizen pick the field when submitting — open **#23**, and it moves classification
   out of the officer's hands
5. The intake moment, and the meaning of the acknowledge clock, for a petition recorded by
   staff — open **#24**
6. Any behaviour ADR 0008 does **not** already express as a flag
7. Any path that would make a deadline **later** than the one already promised

→ Rule 10: `.claude/rules/critical/10-citizen-commitment.md`
→ Open questions #4 · #22 · #23 · #24: `kb/00-foundation/open-questions.json`
→ Decisions: `kb/10-decisions/0007-sla-working-hours.md` · `0008-petition-lifecycle-config.md`
→ Decisions of 2026-09-20: `kb/10-decisions/0026-linh-vuc-phan-anh-hai-tang.md` · `kb/10-decisions/0027-trang-thai-va-dong-ho-phieu-phan-anh.md`
→ Terminology, and the nine status codes: `kb/00-foundation/ubiquitous-language.md`
