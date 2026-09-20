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
| 1 | Intake | `tiep_nhan` | receptionist / auto from Mini App | **yes** — lookup code. On the citizen channel there is **no resolve date yet**: say *"sẽ được xem trong N giờ làm việc"*, never invent one (ADR 0028) | yes |
| 2 | Classify | `phan_loai` | officer | no | yes — **and this is where `han_xu_ly_xong` is fixed** |
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
| **TWO deadlines, set at TWO moments** | `han_tiep_nhan` at row creation, from the **default** SLA row. `han_xu_ly_xong` at the **first classification** for a citizen-submitted petition, from the field just settled — ADR 0028 decision E. A petition recorded by staff sets both at once, because its form carries the field |
| Each one stored once, at the act that fixes it | never recomputed on read. `AdvanceWorkingHours` is called **twice** on the citizen channel — once per deadline, each at its own moment — not once for both |
| Working **HOURS**, not days | ADR 0007 decided hours on 2026-09-16, and the difference is not cosmetic: `An ninh trật tự` must be **acknowledged in 2 working hours**. The smallest thing a count in days can express is one day — **four times** that promise (a working day is 8 hours), in the field where lateness matters most |
| **THREE** tables, not two | `lich_lam_viec` (the week, one row per SESSION so a lunch break is two rows) · `ngay_nghi_le` (closed) · `ngay_lam_bu` (**open although the week says otherwise** — the Prime Minister's annual swap days). Dropping the third counts straight through every swap day, and swap days cluster at Tết and National Day, when the backlog is largest |
| Do not compute it here | `identity` owns the three tables and answers `AdvanceWorkingHours` over gRPC. A second implementation is a second set of answers to one question |
| Received outside working hours | The clock starts at the **next session's opening** — ADR 0007 decision 8, settled 2026-09-20 |
| **Where `count_from` is** | The moment the **citizen pressed send** — for **both** clocks, ADR 0027 decision D, **unchanged by 0028**. Setting a deadline later does not move its origin: the wait before classification is **charged against** the resolve deadline |
| `count_from` for a staff-recorded petition | The moment the citizen **actually reported it**, if the officer can record it (optional field "dân phản ánh lúc"); empty → the moment it was booked. **Bounded**: not earlier than 7 days before booking, not later than booking. Outside → **refuse**, never clamp — ADR 0028 decision F |
| **What stops the acknowledge clock** | The first act by a **human**: entering `dang-phan-loai`. Not the creation of the row |
| **Field changed at classification** | The **first** settling is the act that *fixes* `han_xu_ly_xong`. From the **second** change onward the deadline may only be **SHORTENED**: the earlier of *the deadline already promised* and *the one the new field yields from the same `count_from`* — ADR 0027 decision C. Never the later one |
| Per commune | every number and every table row is **tenant config**, not a constant |
| Overdue is derived | `func (p Petition) IsOverdue(now) bool` — never a stored column |

**Both deadline columns are NULLABLE, and the two `NULL`s mean opposite things.** Getting this
backwards is how a wrong number reaches a leader:

| Column | `NULL` means | What a report must do |
|---|---|---|
| `han_xu_ly_xong` | **not yet** — the petition is still unclassified | exclude it, and show the **count of unclassified petitions** beside any on-time ratio (open **#26**) |
| `han_tiep_nhan` | **not applicable** — staff-recorded petition; the officer *is* the reader | exclude it from every acknowledge-time average. **Never** `COALESCE(..., 0)`: zeroes drag the commune's average acknowledge time toward 0 — a pretty, false figure |

Decision C is **not** a breach of rule 10 invariant 2. That invariant forbids recomputing on
**read**; here a stored value is lowered by a human act, in the same transaction as the act,
with before/after in the audit entry. ADR 0008 already set the precedent: `tinh_lai_han_khi_mo_lai`
writes a new deadline when a petition is reopened.

**Decision E bends the WORDS of invariant 2, and you must read this before quoting them.** The
invariant says the deadline is computed "once, **at intake**, and stored". Still true: computed
once, stored, never recomputed on read. What changed is what "at intake" names — for
`han_xu_ly_xong` it is **the act of classification**, not the insertion of the row (ADR 0028
§Ba cái giá, (c)). A session that reads "at intake" as "when the row is created" will rebuild
the 56-hour ceiling ADR 0028 just removed, **while believing it is obeying the rule**.

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
| The code set | **not the commune** — the **vendor's** admin screen, one operation, not a release | **`platform`**, settled 2026-09-20 (#22) |
| The label | the commune, `(tenant_id, ma)` | `petitions` |

`petitions` keeps the code as a **value**, validated on write. **No foreign key, no `JOIN`** across
the service boundary. The call direction is **business → platform, never the reverse** — a client
from `platform` into a business service is the moment ADR 0003 reopens, not an implementation
detail. **Which shape that read takes — a live gRPC call or a replica fed by events — is still
undecided, and it is an engineering call, not a customer question.** It needs its own ADR the day
`petitions` first reads the set; it is deliberately **not** in `open-questions.json`.

**The ceiling this used to create is GONE — do not re-create it.** The officer settles the field at
classification, so on the citizen channel nothing is known when the row is created. The old answer
set both deadlines there, which combined with decision C made the default row (8h / 56h) a
**ceiling**. ADR 0028 decision E removes it by setting `han_xu_ly_xong` **later**, at classification,
from the same `count_from`. All 12 SLA rows are reachable on the citizen channel. What stays
unreachable: the three `gio_tiep_nhan` = **2 hours** rows, because nobody knows the field before
reading the petition — so the app must say plainly that a **real emergency calls 113 / 114 / 115**
rather than going through this channel.

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
| B — which statuses exist | **Nine, a closed list**, and the customer approved the nine strings **verbatim** on 2026-09-20. Changing one is now an archival migration (rule 7), not a rename. Codes and labels: `kb/00-foundation/ubiquitous-language.md` | ADR 0027 |
| C — field changed at classification | The deadline **only shortens** — from the **second** settling onward | ADR 0027 |
| D — when the clock starts | The citizen's send, for **both** clocks; the acknowledge clock stops at the first staff act | ADR 0027 |
| #22 — which service holds the code set | **`platform`**; adding a 13th code is one operation on the vendor's admin screen. A commune that needs a new code must **ask the vendor**, and until then its petitions fall under `khac` | ADR 0026 |
| #23 — may the citizen pick the field | **No** — and the ceiling is removed instead, by setting the two deadlines at **two moments** | ADR 0028 |
| #24 — the staff-recorded petition | `count_from` = when the citizen actually reported it, bounded to **7 days**, refused outside. `han_tiep_nhan` = **`NULL`**, meaning *not applicable* — never `0` | ADR 0028 |

Both 2026-09-20 decisions in ADR 0028 were made **under delegation** ("theo rule chung của hành
chính xã" · "bạn đề xuất đi"), not chosen by the customer with their own numbers. That makes them
cheaper to reopen — **but only until the first petition carries a real deadline**, after which rule
10 invariant 2 makes them as expensive as any other.

Those first three were answered as **configuration**, not as constants. So the implementation rule
is the same in every case: read the commune's value, never write the default into a branch.

The four decisions of 2026-09-20 went the **other** way — fixed, not configurable — and that is
the point, not an inconsistency. What a commune may shape is how it works inside its own walls
(#6, #7, #8). What is fixed is what leaves the commune: a number that must add up with 200 other
communes (A), a lifecycle a commune must not be able to stop (B), and a promise already spoken to
a citizen (C, D).

## STOP — ask, never decide alone

1. Changing how a deadline is **computed** (the formula, not a commune's number)
2. Adding or removing a **status**, **or changing one of the nine strings** — closed by ADR 0027
   and approved verbatim, so this is a change to a decision and a migration of archival records
3. Writing the field-code table anywhere other than **`platform`**, or writing the read path for
   it **without an ADR** choosing gRPC vs an event-fed replica
4. Setting `han_xu_ly_xong` **when the row is created** for a citizen-submitted petition — that
   rebuilds the 56-hour ceiling, and the code will look correct
5. Writing anything but **`NULL`** into `han_tiep_nhan` for a staff-recorded petition, or
   `COALESCE`-ing either deadline column to `0` in a report query
6. Choosing a **classification deadline** so that unclassified petitions stop skewing the on-time
   ratio — that is open **#26**, and it makes the software issue a promise on its own
7. Any behaviour ADR 0008 does **not** already express as a flag
8. Any path that would make a deadline **later** than the one already promised
9. A **fifth** `kenh_tiep_nhan` whose form does not say whether the field is picked at booking

→ Rule 10: `.claude/rules/critical/10-citizen-commitment.md`
→ Open questions — **#4 and #26 still open**, #22 · #23 · #24 now decided: `kb/00-foundation/open-questions.json`
→ Decisions: `kb/10-decisions/0007-sla-working-hours.md` · `0008-petition-lifecycle-config.md`
→ Decisions of 2026-09-20: `kb/10-decisions/0026-linh-vuc-phan-anh-hai-tang.md` · `kb/10-decisions/0027-trang-thai-va-dong-ho-phieu-phan-anh.md` · `kb/10-decisions/0028-moc-dat-han-hai-dong-ho.md`
→ Terminology, and the nine status codes: `kb/00-foundation/ubiquitous-language.md`
