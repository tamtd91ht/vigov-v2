---
description: Continue developing one business menu on the citizen Zalo Mini App — discovery, tagged proposal, confirmation, per-task commits, per-menu ledger
argument-hint: "<menu> — bắt buộc, ví dụ: Phản ánh người dân"
allowed-tools: Read, Grep, Glob, Bash, Edit, Write, Agent, AskUserQuestion
---

# /develop-miniapp `<menu>`

Menu: **$ARGUMENTS**

**Read `.claude/skills/develop-menu/SKILL.md` in full and follow it.** It holds the whole
procedure; this file holds only what is specific to the Mini App.

## Two preconditions, before anything else

| Check | Missing means |
|---|---|
| `../vihat-miniapp` exists **beside** this repo | STOP — CLAUDE.md, "THE MINI APP SPANS TWO REPOSITORIES". Reasoning about the Mini App from the half that lives here is the error that put a Zalo webhook in the wrong repo (ADR 0032). `miniapp_sibling_guard` blocks it too |
| The menu has a **citizen surface**: its spec describes a Mini App screen, or `citizen-app/src/` already has one | STOP and ask. Most menus are staff-only (`/develop-miniapp Nhiệm vụ` — citizens never see tasks); an empty run is not a result |

## Scope

| In scope — `citizen-app/**` | Owner (ROUTING §3) |
|---|---|
| Page · component · state · API integration · navigation · validation · UX workflow · tests | `citizen-app-builder`; tests with `test-designer` |

The citizen channel is **not** the staff app with a different skin:

- no domain — the commune is chosen by the citizen's explicit act (ADR 0005 · 0019 · 0022);
  a QR or deep-link parameter steers the UI and grants nothing
- weak identity — OTP never alone backs an act with legal consequences (rule 4)
- the citizen sees **only their own** records, never staff notes or routing history (rule 4)
- elderly users — text size, touch targets, plain Vietnamese (`accessibility-elderly`)

Which repo a Zalo surface belongs to is decided by **which secret signs it** (CLAUDE.md table):
Mini App login, webhook and token exchange live in `vihat-miniapp`, not here. A card that
needs them is a `MINIAPP DEPENDENCY` on that repo, reported — never implemented from this one.
A card the backend must serve first is a `BACKEND DEPENDENCY` (skill §5).

→ Skills that apply: `zalo-miniapp-multi-tenant` · `citizen-identity-multi-tenant` ·
`accessibility-elderly` · `administrative-language` · `petition-lifecycle` (menu
`phan-anh-nguoi-dan`)
