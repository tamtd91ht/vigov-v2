---
name: naming-english
description: Use when naming anything in NEW code — a function, method, type, variable, constant, struct field, export, source file, directory, DB table or column, migration, object-storage bucket — or when english_identifier_guard blocks a write. Triggers on: naming, identifier, function name, variable name, type name, file name, đặt tên, tên hàm, tên biến, tên kiểu, tên tệp, English, tiếng Anh, bucket, migration column, table name, tên bảng, tên cột, vi-name-ok.
---

# Skill: Naming new code in English

Rule 12 states the invariant, ADR 0051 the decision (2026-09-28). This file says **how**.

## 1. What is English, what stays Vietnamese

| Surface | Language | Example |
|---|---|---|
| New Go/TS function, method, type, interface, variable, constant, struct field, export | **English** | `CreateMeeting`, `listTasks`, `MeetingDraft` |
| New source file / directory | **English** | `meeting_draft.go`, `features/meeting-drafts/` |
| New table / column (migration) | **English**, `snake_case` | `meeting_attachment.file_name` |
| New object-storage bucket, object-key segments | **English**, kebab-case | `meeting-attachments` |
| Route directory a user sees — `web-admin/src/app/**`, `platform-admin/src/app/**` | **Vietnamese**, no diacritics | `app/nhiem-vu/moi/page.tsx` |
| UI strings, messages to citizens, `kb/` prose | **Vietnamese** with diacritics | `"Danh sách nhiệm vụ"` |
| Enum **values** (the string stored and sent) | **Vietnamese**, no diacritics — ADR 0011 | `"khieu-nai"`, `"da-ky"` |
| REST path segments `/api/v1/...` | **English** — ADR 0011, `skills/rest-api-design` | `/api/v1/citizen-reports` |

ADR 0051 also keeps **Mini App screen routes** Vietnamese. Today `citizen-app/` has **no router**
(`citizen-app/src/App.tsx`, "WHY NOT A ROUTER"), so there is no route directory to exempt and every
new file there is English. The day a router arrives, add its route root to `ROUTE_ROOTS` in
`.claude/hooks/english_identifier_guard.py` in the same change.

An enum **value** stays Vietnamese; the **constant that names it** is new code and is English:

```go
const StatusSigned = "da-ky"        // name English, value per ADR 0011
```

A proto enum value name is serialised as the JSON value — treat it as a VALUE and mark it
`// vi-name-ok: enum value, ADR 0011` if it must spell the Vietnamese code.

## 2. The vocabulary — take it, do not invent it

`kb/00-foundation/ubiquitous-language.md` is the **only** mapping. Use its `@entity` column for types
and tables, its URL-resource column for REST nouns. A second English name for a concept that
already has one is the drift ADR 0011 was written against.

Types below come from the `@entity` column where it exists, otherwise from the singular of the
fixed URL noun — the same word, so the two surfaces agree. **Not yet a rule:** ADR 0051 §"Chưa
chốt" #2 leaves open whether new tables must follow `@entity`; it is the obvious reading, so
state it as your assumption in the proposal rather than deciding silently.

| Vietnamese | English type · URL noun |
|---|---|
| Phản ánh (phiếu phản ánh) | `CitizenReport` · URL `citizen-reports` — **not** `Feedback` |
| Đơn thư | `CitizenLetter` · `citizen-letters` |
| Văn bản đến / đi | `IncomingDocument` / `OutgoingDocument` |
| Nhiệm vụ | `Task` · `tasks` |
| Biên bản họp | `Meeting` · `meetings` — not `Minutes` |
| Kết luận họp | `Conclusion` · `conclusions` |
| Giải ngân | `Disbursement` · `disbursements` |
| Dự án đầu tư | `Project` (entity) · `investment-projects` (URL) |
| Cán bộ | `Staff` — not `User` (a citizen is a user too; rule 4) |
| Công dân | `Citizen` |
| Xã (đơn vị đang phục vụ) | `Tenant` in code · `communes` on URLs |
| Thôn / Tổ dân phố | `ResidentialUnit` |
| Loại văn bản | `DocumentType` |
| Ngày nghỉ lễ / ngày làm bù | `PublicHoliday` / `SwapWorkingDay` |
| Lịch làm việc | `WorkingHours` |
| Nội dung / danh mục Mini App | `ContentItem` / `ContentCategory` |

Not in the table: read the mapping file first — it has more rows than this one.
**A business concept with no English name there → ask the user** (rule 12 stop condition). A
technical helper (`parseCursor`, `retryCount`) needs no mapping entry.

**OPEN — ask before naming:** legal terms with no safe English equivalent (`khieu_nai` / `to_cao` /
`phan_anh`, `thu_ly` / `tiep_nhan`). ADR 0051 §"Chưa chốt" #1 leaves undecided whether new
identifiers for these use an English name or `vi-name-ok`. What IS fixed: they must never collapse
into one English word, because they run under different statutes and clocks (ADR 0011).

## 3. Database naming

- `snake_case`, English, singular table names as the existing tables are: `meeting_attachment`.
- `tenant_id` on every business table (rule 1), `deleted_at` · `deleted_by` · `delete_reason` (rule 7).
- A foreign key names the **English** concept: `meeting_id`, even when it references the old table
  `bien_ban_hop`. The old table keeps its name — renaming a populated table is a rule 7 migration.
- A new column on an **old** Vietnamese table is still English: `ALTER TABLE bien_ban_hop ADD
  COLUMN signed_file_key TEXT`.
- A migration file name is new: `0019_meeting_attachment.sql`.

## 4. Adding to an old Vietnamese file

Correct, and expected:

```go
// in service-petitions/internal/app/bien_ban_hop.go
func (u *UseCase) ListDrafts(ctx context.Context) ([]domain.Meeting, error) { ... }
```

- Do **not** rename the neighbours to match. ADR 0051 rules out renaming existing code — a huge diff
  with no business value that buries real changes in history. The one exception it allows: a change
  that rewrites the **whole file** anyway. A file mixing both languages is accepted.
- Editing an existing Vietnamese name (its body, its signature) is allowed and never blocked.
- A new test file named after an existing source file (`bien_ban_hop_test.go` beside
  `bien_ban_hop.go`, `can-bo.test.ts` beside `can-bo.ts`) follows its subject and is allowed; any
  other new test file is English.

## 5. The exemption

```go
// vi-name-ok: mirrors the JSON field of a contract already in production
type thanDangNhapCu struct { ... }
```

`vi-name-ok: <reason>` in a line comment (`//`, or `--` in SQL) on the declaration's line or in the
comment block directly above. The reason is mandatory; an empty one is still blocked. Legitimate
reasons: a live wire field or column you must match, an enum value (ADR 0011), a legal term with no
English equivalent that the mapping table records. "Consistency with the file" is **not** a reason.

## 6. What the guard can and cannot see

`english_identifier_guard` blocks a NEW declaration whose name carries **two or more** Vietnamese
syllables (`TaoBienBan`, `layDanhSach`, `bien_ban_moi`) and new Vietnamese path segments. It does
not see function parameters, TS object members, or a name with one Vietnamese syllable (`tenXa`
passes; `xa` alone passes). Those are still rule 12 — the guard enforces only the decidable part.

→ Rule: `.claude/rules/critical/12-english-identifiers.md`
→ Decision: `kb/10-decisions/0051-english-code-identifiers.md`
→ Mapping: `kb/00-foundation/ubiquitous-language.md`
→ Related: `skills/rest-api-design` (URL nouns) · `skills/administrative-language` (UI text)
