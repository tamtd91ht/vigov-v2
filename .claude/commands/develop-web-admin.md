---
description: Continue developing one business menu on the staff Web Admin — discovery, tagged proposal, confirmation, per-task commits, per-menu ledger
group: Phát triển
argument-hint: "<menu> — bắt buộc, ví dụ: Nhiệm vụ"
allowed-tools: Read, Grep, Glob, Bash, Edit, Write, Agent, AskUserQuestion
---

# /develop-web-admin `<menu>`

Menu: **$ARGUMENTS**

**Read `.claude/skills/develop-menu/SKILL.md` in full and follow it.** It holds the whole
procedure; this file holds only what is specific to the Web Admin.

## Platform check (skill §0)

The menu must be one the staff app has: a row in `web-admin/src/components/muc-menu.ts`.
`duong: null` there means the screen is not built yet — still in scope, as a new screen.
No row at all → STOP and ask; do not add a menu entry on your own.

## Scope

| In scope — `web-admin/**` | Owner (ROUTING §3) |
|---|---|
| Page · component · state · API integration (`src/lib/api/`) · validation · permission gating in the UI · UX workflow · tests | `admin-web-builder`; tests with `test-designer` |

The backend may be **read** to learn the contract (`kb/20-contracts/openapi.json`,
`web-admin/src/lib/api/schema.gen.ts`). Anything the screen needs that the server does not
serve is a `BACKEND DEPENDENCY` card (skill §5) — never a server edit from this command.

Permission shown in the UI is convenience only; the service layer is what enforces it (rule
5, forbidden #1). A screen that hides a button the API would still accept is a finding for
`/develop-backend-api`, not a fix here.

→ Skills that apply: `nextjs-multi-tenant` · `administrative-language` · `rest-api-design`
(reading the contract)
