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
| Enum **values** (the string stored and sent) | **English**, kebab-case (X14), since 2026-09-29 (user decision; supersedes ADR 0011) — existing Vietnamese values are migrated by the rename campaign with a dual-accept transition | `"signed"`, `"assigned"` |
| k8s object names, ConfigMap/Secret names, NetworkPolicy names, labels in `deploy/**` | **English**, kebab-case; match the REAL cluster (`common-config`, `<service>-secrets`, `vigov-service-<service>`) | `allow-egress`, `vigov.vn/surface` |
| REST path segments `/api/v1/...` | the ONLY place a Vietnamese segment may remain (user decision 2026-09-29); new ones English per `skills/rest-api-design` | `/api/v1/citizen-reports` |

ADR 0051 also keeps **Mini App screen routes** Vietnamese. Today `citizen-app/` has **no router**
(`citizen-app/src/App.tsx`, "WHY NOT A ROUTER"), so there is no route directory to exempt and every
new file there is English. The day a router arrives, add its route root to `ROUTE_ROOTS` in
`.claude/hooks/english_identifier_guard.py` in the same change.

Both the constant and its value are English for anything new. An EXISTING Vietnamese value that
is still on the wire keeps working until the campaign migrates it (dual-accept, then removal):

```go
const StatusSigned = "signed"
```

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
| Kết luận họp | `MeetingConclusion` (X9) · `conclusions` |
| Giải ngân | `Disbursement` · `disbursements` |
| Dự án đầu tư | `InvestmentProject` (X6) · `investment-projects` |
| Cán bộ | `Staff` — not `User` (a citizen is a user too; rule 4) |
| Công dân | `Citizen` |
| Xã | `Tenant` = isolation boundary / ID · `Commune` = the administrative unit shown (`CommuneProfile`) (X5) · `communes` on URLs |
| Thôn / Tổ dân phố | `ResidentialUnit` |
| Loại văn bản | `DocumentType` |
| Ngày nghỉ lễ / ngày làm bù | `PublicHoliday` / `SwapWorkingDay` |
| Lịch làm việc | `WorkingHours` |
| Nội dung / danh mục Mini App | `ContentItem` / `ContentCategory` |

Not in the table: read the mapping file first — it has more rows than this one.
**A business concept with no English name there → ask the user** (rule 12 stop condition). A
technical helper (`parseCursor`, `retryCount`) needs no mapping entry.

**Legal terms** (`khieu_nai` / `to_cao` / `phan_anh`, `thu_ly` / `tiep_nhan`): DECIDED 29/09 (X19) —
English per the glossary's translation table, legal meaning recorded in ADR 0061. They must never
collapse into one English word: they run under different statutes and clocks (ADR 0011).

**Every naming decision (X1–X25) and its reason** lives in the glossary §Từ điển đổi tên — the one
place to read before renaming and to update when a new conflict is settled. Do not restate it here.

## 3. Database naming

- `snake_case`, English, singular table names as the existing tables are: `meeting_attachment`.
- `tenant_id` on every business table (rule 1), `deleted_at` · `deleted_by` · `delete_reason` (rule 7).
- A foreign key names the **English** concept: `meeting_id`, even when it references the old table
  `bien_ban_hop`. Renaming the old table is the campaign's layer B (reversible `RENAME` migration,
  ADR 0061), never a side effect of a feature change.
- A new column on an **old** Vietnamese table is still English: `ALTER TABLE bien_ban_hop ADD
  COLUMN signed_file_key TEXT`.
- A migration file name is new: `0019_meeting_attachment.sql`.

## 4. Adding to an old Vietnamese file

Correct, and expected:

```go
// in service-petitions/internal/app/bien_ban_hop.go
func (u *UseCase) ListDrafts(ctx context.Context) ([]domain.Meeting, error) { ... }
```

- Renaming the neighbours is the **rename campaign's** job (user decision 2026-09-29: rename
  everything, service by service, in dedicated commits following the VN→EN dictionary). Outside a
  campaign commit, don't mix a mass rename into a feature change — it buries the real change.
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
