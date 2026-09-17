# citizen-app

The citizen-facing **Zalo Mini App**. React + Vite + zmp-sdk.

Citizens are identified by phone plus OTP — a **weak identity**, not to be trusted. And they
**do not choose** this software: being unable to use it means being unable to reach a public
service. That makes accessibility a rights question, not a preference.

## Two phases, ONE App ID

A Mini App is identified by its App ID, and an App ID is what Zalo reviews and what a
verifying Official Account is bound to. That single identifier is why this app ships in two
phases instead of two apps.

| Phase | What ships | Why |
|---|---|---|
| **1 — now** | A static introduction to **VihatSoftware**, the company that publishes the app | This is the submission Zalo reviews, so the OA `VihatSoftware` can verify the App ID |
| **2 — next** | The commune / citizen surface | It lands on the **same App ID**, already verified |

**Nothing of phase 2 gets deleted to make room for phase 1.** `src/lib/commune-resolution.ts`
is the foundation phase 2 builds on and stays untouched.

Why the verifying OA is VihatSoftware and not a commune, and why the notification OA is a
different OA per commune: `kb/10-decisions/0018-oa-xac-thuc-tach-khoi-oa-thong-bao.md`. How the
commune gets resolved at runtime with no domain to key off:
`kb/10-decisions/0005-miniapp-tenant-resolution.md`. Both are read there, not repeated here.

### Phase 1 collects no personal data — keep it that way

No sign-in, no `getPhoneNumber`, no OTP, no form, no backend call, no commune logic, no
`tenant_id`. The only outbound links are `tel:`, `mailto:` and the company website.

That is a deliberate design choice, not an accident of scope. It makes rule 3 (personal data)
hold **by construction** rather than by argument, and it gives the Zalo review nothing to
weigh: an app that asks for nothing has no permission to justify. Adding any collection to
phase 1 changes what was submitted for review — raise it before writing it.

The hotline and email on the contact screen are **ViHAT Group corporate contact points**
published on its website. They identify no individual, so they are business data, not personal
data. No individual's number belongs in this app.

## One app, every commune

A Mini App is identified by its platform App ID, not a domain, so the "tell communes apart by
domain" strategy does not apply here. The commune is resolved at runtime — see
`src/lib/commune-resolution.ts`.

## Non-negotiables

| # | Rule |
|---|---|
| 1 | GPS **suggests**, never **decides** |
| 2 | Once selected, the commune name appears on **every** screen |
| 3 | Switching commune is an explicit action, never automatic |
| 4 | Identity comes from the session; the backend never trusts a client-supplied identity |
| 5 | The commune is confirmed again at the final step before submitting |
| 6 | Body text ≥ 16px · touch targets ≥ 44×44px · contrast ≥ 4.5:1 · status never by colour alone |
| 7 | Error messages say what to do next, never an error code |
| 8 | Nothing personal in logs, URLs, or file names |

Rules 6, 7 and 8 already bind phase 1. Rules 1–5 bind phase 2; rule 2 is why the header in
`src/App.tsx` reserves the slot that will hold the commune name.

## Error message shape

| Wrong | Right |
|---|---|
| `Error 422: Validation failed` | "Số điện thoại chưa đúng. Nhập 10 số, bắt đầu bằng 0." |
| `Unauthorized` | "Phiên đăng nhập đã hết. Đăng nhập lại để tiếp tục." |

## Where the content lives

Every user-visible string of phase 1 sits in `src/content/company-profile.ts`. One file,
because phase 2 replaces this content wholesale and the edit should land in one place.

**Nothing may be added to that file without a source.** The app carries the name of a real
legal entity: an unsourced founding year, customer name or award is a false statement
published under that name. Facts that are missing are left out, never filled in.

## Before submitting to Zalo — two things still to confirm

| # | What | Why it is not settled here |
|---|---|---|
| 1 | `app-config.json` key names and the expected upload folder | Written from secondary sources; Zalo's own documentation renders through JavaScript and could not be read directly. A wrong key is a submission sent back. Confirm against the developer console before uploading |
| 2 | App icon, screenshots and the store description | Not produced yet, and required by the review |

## Commands

| Command | What it does |
|---|---|
| `npm run dev` | Vite dev server |
| `npm run build` | Static bundle into `dist/` |
| `npm run typecheck` | `tsc --noEmit` |
| `npm test` | Vitest — pins the published facts and the screen registry |

→ Skills: `.claude/skills/zalo-miniapp-multi-tenant` · `.claude/skills/accessibility-elderly`
