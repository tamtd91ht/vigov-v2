---
name: rest-api-design
description: Use when adding or changing any HTTP route — URL shape, resource naming, methods, status codes, pagination, and protection against duplicate requests. Triggers on: REST, RESTful, API, route, endpoint, URL, path, HTTP, GET, POST, PUT, PATCH, DELETE, status code, pagination, phan trang, cursor, versioning, idempotency, Idempotency-Key, duplicate request, trung request, double submit, retry, high traffic, list, filter, sort.
---

# Skill: REST API design

A URL path is a **contract with whoever integrates with the commune** — the Mini App, the
provincial portal, another authority's system. An internal name can be renamed in an
afternoon; a path that a third party already calls cannot. There is no cheap second chance,
so the cost of getting this right is only ever paid at the moment of writing.

## REQUIRED

| # | Practice |
|---|---|
| 1 | Path segments are **English**, plural, kebab-case, **nouns**: `/api/v1/incoming-documents` |
| 2 | **Enum VALUES stay Vietnamese without diacritics**: `?type=khieu-nai`, never `?type=complaint` |
| 3 | Prefix `/api/v1/`. Two versions run side by side during a migration, as events already do (rule 2, invariant 4) |
| 4 | `tenant_id` **never** appears in a path or a query. It is derived from `Host` at the edge (rule 1, invariant 3) |
| 5 | Citizen routes are separated from staff routes **at routing level**, never sharing a handler (rule 4, invariant 5) |
| 6 | Every state-changing route declares duplicate protection **in the same statement** as the route — §4 |
| 7 | Lists are **cursor-paginated** by default and always bounded — §5 |
| 8 | A non-CRUD action is a **nominalised sub-resource**, not a verb: `POST .../closure`, not `POST .../close` — §3 |

## FORBIDDEN

| # | Forbidden | Why |
|---|---|---|
| 1 | A Vietnamese word in a path segment | The path is the one surface read by people who do not read Vietnamese — integrators, log tooling, API gateways |
| 2 | **Translating a legal term into an enum value** (`complaint`, `denunciation`) | `khieu_nai` and `to_cao` run under two different statutes, with two different clocks. English collapses them — `kb/00-foundation/ubiquitous-language.md:87` |
| 3 | A verb as a path segment (`.../create`, `.../close`, `.../approve`) | An action with legal consequence must be a record you can `GET` back: who did it, when, with what result |
| 4 | An unbounded list route | One commune's five years of petitions in one response, on a shared process serving 200+ communes |
| 5 | A state-changing route with no duplicate declaration | §4 — the resulting duplicate is permanent, because rule 7 forbids hard delete |
| 6 | Personal data in a path or query string (`?phone=`, `/citizens/0901234567`) | Rule 3, forbidden #4. Paths land in access logs, proxy caches and browser history |
| 7 | A lookup code that is sequential or short in a public path | Rule 4, invariant 4 — enumeration reads every record |

## 1. Where English stops

`kb/00-foundation/ubiquitous-language.md` owns the naming table. It covers tables, Go types,
events, services — **the URL path row belongs there too**; this skill states only how to
apply it.

| Layer | Language | Example |
|---|---|---|
| Path segment | **English** | `/api/v1/citizen-letters` |
| Query parameter name | **English** | `?type=`, `?status=`, `?cursor=` |
| **Business enum value** | **Vietnamese, no diacritics** | `?type=khieu-nai`, `?status=dang-xu-ly` |
| Permission key | English, singular group | `petition.read` — `identity/migrations/0001_init.sql` |
| JSON field name | English | `lookup_code`, `created_at` |
| Error `code` | English, snake_case | `missing_idempotency_key` — matches `httpx.Error` |
| Any string a person reads, incl. error `message` | **Vietnamese with diacritics** | `"Không có nhiệm vụ"` |

**Row 3 is the one that gets lost.** Translating a path is a naming choice. Translating a
*value* is a business claim: it asserts that the Vietnamese concept and the English word mean
the same thing. For `kien-nghi` / `phan-anh` / `khieu-nai` / `to-cao` / `de-nghi` they do not,
and the difference decides which statute applies and how long the authority has to answer.

**It applies to BUSINESS concepts only.** A technical error code carries no administrative
meaning, so it follows the machine-readable rule and stays English. `core/httpx/edge.go`
declares one error shape for the whole system and fills `Code: "internal"`; a second
convention inside that same shape is the drift rule 9 exists to stop. The split to hold onto:
**`code` is an identifier, `message` is a sentence someone reads.**

→ `.claude/skills/administrative-language/SKILL.md` before naming any business concept.

## 2. Picking the resource noun

Three questions, in order:

1. **Is it a thing the authority keeps, or a thing it does?** Keeps → resource. Does → §3.
2. **Does the Vietnamese word cover more than the English one?** `thong_bao` is three
   different things — internal notice to staff, public notice on the Mini App, and the bell
   in the header. One word in Vietnamese, **three nouns in English**, or an internal notice
   reaches the citizen channel.
3. **Does the English word claim a distinction the data does not have?** `don_thu` is *one
   register with one continuous number series*. Splitting it into three paths would split the
   series — which is rewriting an archival record.

**The mapping itself is owned by `kb/00-foundation/ubiquitous-language.md`**, section *Tên tài
nguyên trên URL*. Look the concept up there before naming anything. It is deliberately not
repeated here: two copies of one mapping are two copies that drift, and the one that drifts is
the one a route ends up being named from (rule 9, forbidden #2).

A concept with **no row** there is not an invitation to translate it. Stop and ask. The row is
missing because nobody has established what the concept is called yet — and a wrong noun in a
path cannot be taken back once a commune is live.

## 3. Actions that are not CRUD

Administrative work is mostly transitions, not edits. Nominalise the transition and `POST` it
as a sub-resource:

```
POST /api/v1/citizen-reports/{code}/closure        not  .../close
POST /api/v1/outgoing-documents/{id}/number        not  .../assign-number
POST /api/v1/citizen-letters/{no}/admission        not  .../admit
PUT  /api/v1/tasks/{id}/assignment                 not  .../assign
```

Three things this buys, all of which a verb throws away:

| | Verb path | Nominalised path |
|---|---|---|
| "Who closed this, when, with what result?" | needs a separate table | `GET .../closure` |
| Second identical request | runs the action again | returns the existing record |
| Name of the row in the audit trail | invented at the call site | the resource name |

**Transitions with legal consequence** — these are where a duplicate hurts most, and they all
take `idem.Required(idem.DongKhiHong)`:

`receipt` (starts the SLA clock, rule 10 invariant 2) · `admission` · `escalation` (moves
jurisdiction out of the commune) · `closure` (tells the citizen it is done, rule 10 invariant
6) · `reopening` · `number` (an issued document number is never reissued, rule 7 invariant 3)
· `issuance` · `revocation` · `confirmation` and `lock` on a disbursement · approval of an
extension (moves a deadline already promised).

## 4. Duplicate requests

A double-submitted `POST` creates a second petition with a second lookup code **already shown
to the citizen**, or a second disbursement against real money. Rule 7 forbids hard delete, so
the duplicate is permanent — it can only be soft-deleted, and the code it consumed is never
reissued.

### The declaration

Opt-in **per route, in the same statement**, the way permissions already are. There is no
bare `idem.Required()`: the failure mode has to be chosen, never defaulted.

```go
mux.Handle("POST /api/v1/citizen-reports",
    authz.RequirePermission(d.Checker, "feedback.create")(
        idem.Required(idem.MoKhiHong)(
            http.HandlerFunc(h.create))))

mux.Handle("POST /api/v1/disbursements/{id}/confirmation",
    authz.RequirePermission(d.Checker, "budget.confirm")(
        idem.Required(idem.DongKhiHong)(
            http.HandlerFunc(h.confirm))))

mux.Handle("DELETE /api/v1/sessions/{sid}",
    authz.AnyAuthenticated("any signed-in account may end its own session")(
        idem.KhongCan("deleting an already-deleted session is the same outcome")(
            http.HandlerFunc(h.revoke))))
```

| Declaration | Redis unreachable | Use for |
|---|---|---|
| `idem.Required(idem.MoKhiHong)` | let it through, `log.Warn` | Intake paths. Refusing a citizen because a cache is down is worse than a rare duplicate |
| `idem.Required(idem.DongKhiHong)` | **503** | Legal consequence: money, document numbers, issuance, closure, privilege changes |
| `idem.KhongCan("<reason>")` | n/a | Naturally idempotent. **Reason mandatory** — same discipline as `authz.Public(reason)` |

`authz` wraps **outside** `idem`, for two reasons: an unauthorised request must not consume an
idempotency key, and the key needs the principal already in the context.

### One Redis key, holding a small value

ADR 0010 allows Redis for cache and rate limiting and **forbids it as durable storage**
(`kb/10-decisions/0010-data-infrastructure.md`). This stays inside that boundary: the truth is
the PostgreSQL row, the key is a 24-hour guard.

```
key   t:<tenant_id>:idem:<sha256(actor + method + path + Idempotency-Key)>
        actor = "<kind>:<id>" from the session, or "anon" when there is none
value "1"                       in flight          TTL 60s
      "2:<status>:<code>"       done               TTL 24h
```

```
SET key "1" NX EX 60
 ├─ acquired   → run handler → on success: SET key "2:201:PA-7F3K9Q" EX 86400
 └─ exists     → GET:  "1"   → 409 + Retry-After: 1
                      "2:…"  → replay 201 + the lookup code
```

Three details, each of which has a specific failure behind it:

| # | Detail | What breaks without it |
|---|---|---|
| 1 | **`t:<tenant_id>` prefix** (rule 1, invariant 7) | Two communes whose clients generate the same key collide, and commune B is served commune A's response. A data breach between two authorities, through a cache key |
| 1b | **The actor in the hash** | The prefix stops nothing *inside* one commune. The key is client-generated, so two staff members can present the same one — and the second is handed a business code for work they did not do, while their own record was never created. Take `kind` as well as `id`: staff ids and citizen ids come from different tables |
| 2 | **Short TTL while in flight, long TTL after success** | A handler that dies mid-request leaves `"1"` for 24 hours: every retry is refused although the petition was never created |
| 3 | **Store `status:code` only, never the response body** | The body holds names, phone numbers, petition text. Putting it in Redis builds a personal-data store outside PostgreSQL — no audit trail, no soft delete, rule 3 |

A bare flag with no value is not enough: the retry would get `409` and the citizen would never
learn their lookup code, while their petition sits in the system. Rule 10, invariant 1.

### Second layer, where it is free

Where a natural key exists, add `UNIQUE (tenant_id, …)` as well. The two layers fail
independently: a client that forgets the header still hits the constraint, and a table with no
natural key still has the header. Say in the route which layer is actually protecting it.

## 5. High traffic

One process serves 200+ communes. Every list route is a shared resource.

| # | Practice |
|---|---|
| 1 | **Cursor pagination** by default: `?limit=20&cursor=…`, response carries `next_cursor` + `has_more` |
| 2 | `limit` has a **server-side maximum**. A client asking for 10,000 gets the maximum, not an error and not 10,000 |
| 3 | Offset pagination only with a declared reason at the call site, when a screen genuinely needs page jumps |
| 4 | No `COUNT(*)` on a partitioned table to decorate a list. A total is a separate, cached route |
| 5 | Filtering and sorting run on **indexed columns only**, from an allowlist — never a client-supplied column name |
| 6 | Batch the reads behind a handler — `.claude/skills/load-data-once/SKILL.md` owns this |

**Why cursor is the default.** Offset re-scans every skipped row, so page 500 costs 500 pages
of work; and when a new petition arrives while a citizen is paging, offset shows them a record
twice and hides another. Cursor has neither problem. The cost is real: no jump to page 47 and
no page count — so a screen that needs those declares offset and says why.

`docs/ui-ux/` specifies **no pagination at all**; it was copied from a single-commune
prototype where every list rendered whole. Do not read that absence as permission.

## Checklist before adding a route

```
[ ] path English, plural, noun, kebab-case, under /api/v1/
[ ] enum values Vietnamese without diacritics
[ ] no tenant_id, no personal data in path or query
[ ] permission declared in the same statement (rule 5)
[ ] citizen route separate from the staff route (rule 4)
[ ] state-changing  -> idem.Required(MoKhiHong|DongKhiHong) or idem.KhongCan("<reason>")
[ ] list route      -> cursor, bounded limit, allowlisted sort
[ ] non-CRUD action -> nominalised sub-resource, not a verb
[ ] tests: 401 · 403 wrong permission · 403 right permission WRONG COMMUNE · 200
```

→ Enforcement: `.claude/hooks/rest_api_guard.py` (BLOCK on a Vietnamese path segment and on a
missing duplicate declaration; advisory on the rest)
→ Rules: `.claude/rules/critical/1-tenant-isolation.md` · `.claude/rules/critical/4-citizen-isolation.md` · `.claude/rules/critical/5-rbac.md` · `.claude/rules/critical/10-citizen-commitment.md`
→ Terminology: `kb/00-foundation/ubiquitous-language.md` · `.claude/skills/administrative-language/SKILL.md`
→ Infrastructure: `kb/10-decisions/0010-data-infrastructure.md`
