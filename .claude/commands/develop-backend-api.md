---
description: Continue developing one business menu on the Go backend — discovery, tagged proposal, confirmation, per-task commits, per-menu ledger
argument-hint: "<menu> — bắt buộc, ví dụ: Nhiệm vụ"
allowed-tools: Read, Grep, Glob, Bash, Edit, Write, Agent, AskUserQuestion
---

# /develop-backend-api `<menu>`

Menu: **$ARGUMENTS**

**Read `.claude/skills/develop-menu/SKILL.md` in full and follow it.** It holds the whole
procedure; this file holds only what is specific to the backend.

## Platform check (skill §0)

Find the owning service of the menu's entities in `kb/30-indexes/data-ownership.json`. No owner
for an entity the menu needs → rule 2 STOP CONDITION 1: ask, dispatch nothing.

## Scope — one command, several owners

"Backend API" is one scope for the user but **three write boundaries** in ROUTING §3. Each card
goes to the owner of what it changes, in the cross-layer order of ROUTING §3
(`contract → storage → behaviour`), sequentially:

| Card changes | Owner |
|---|---|
| `.proto`, events, entity ownership, transaction boundaries | `contract-designer` |
| a table that holds data, a migration, a backfill, an index | `data-migration-builder` |
| route · use case · domain · repository · validation · authorisation · integration | `go-service-builder` |
| tests | `test-designer`, after the builder, never alongside it |

Every new or changed endpoint meets rule 5: an explicit declaration, a permission key that
exists in `quyen` (3c — a missing key is a finding for open question #27, never an `INSERT`),
and the four tests (401 · 403 · 403 wrong commune · 200). Every write leaves an audit entry in
the same transaction (rule 6).

`web-admin/**` and `citizen-app/**` may be **read** to learn how the API is consumed. A consumer
that must change is a `WEB DEPENDENCY` / `MINIAPP DEPENDENCY` card (skill §5). After any route
change, `make kb` regenerates `openapi.json` and the web task queue — that is how the web side
learns about it, not an edit from here.

→ Skills that apply: `go-service-pattern` · `go-tenant-context` · `rest-api-design` ·
`audit-trail` · `load-data-once` · `transaction-boundary` (multi-step flows) ·
`petition-lifecycle` (menu `phan-anh-nguoi-dan`)
