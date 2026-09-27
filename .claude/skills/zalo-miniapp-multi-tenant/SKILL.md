---
name: zalo-miniapp-multi-tenant
description: Use when building citizen-facing screens or flows in the Zalo Mini App, especially resolving which commune the citizen is acting with, the main app vs a commune's own app, App ID, deep links, QR codes, or ZNS notifications. Also load it when someone proposes a commune picker, GPS suggestion or a switch-commune button — ADR 0044 removed all three. Triggers on: Zalo, Mini App, zmp-sdk, citizen app, App ID, deep link, QR, commune selection, commune picker, GPS, location, miniapp, citizen channel, ZNS, OA, notification.
---

# Skill: Zalo Mini App in a multi-commune system

## The contradiction to understand before writing anything

"Tell communes apart by domain" is right for the admin webs and **does not apply** here — and
the reason is not ours to change:

| Platform fact | Consequence |
|---|---|
| A Mini App is identified by its **Zalo App ID**, not a domain | No request Host to key off. The shared app receives the commune's domain as a **URL parameter** instead — a lookup key the server resolves, never a trusted fact (ADR 0047) |
| Each App ID needs its own registration and review | A commune's own app is a Zalo filing plus the commune's official letter — procedure, not code |
| Citizens find the app by searching Zalo | Similar names cause wrong choices, worst for elderly users |

**Two modes, ONE build of `citizen-app`** (ADR 0044, which partly supersedes ADR 0005; the
QR parameter and the build flow were changed by ADR 0047):

| | Main (shared) app | A commune's own app |
|---|---|---|
| Commune comes from | QR/URL parameter carrying the commune's **domain** → server resolves it (`ResolveHost`) → citizen confirms once → server writes the **ULID** into the session. The citizen then works inside the shared app; no redirect to the own app | Verified App ID → platform table `app_id → tenant_id` → server binds it into the session |
| Opened with no QR | Commune confirmed before → **back to that commune**. Never → **group introduction only** | Straight into the app's commune |

Every session works with **exactly one commune**. There is no commune picker, no switch button,
and no submitting to another commune. The full table, and why the server — not the client or
the build — decides the mode: ADR 0044, not copied here.

Never branch the build per commune (env var, constant, code branch) — that is ADR 0044 stop
condition #1 and rule 1 invariant 10. The one exception is ADR 0047's **target-selection** file
under `citizen-app/scripts/`: a domain passed at build time only picks **which App ID the same
bundle is uploaded to**; nothing from it enters the bundle. It is **not** the source of truth for
which commune an App ID serves — the platform `MiniApp` table is, and when the two disagree the
server wins.

Second, narrower exception (ADR 0047 §6, 27/09/2026): `deploy.mjs --domain=<x> --vao-thang` bakes
the domain `<x>` — and nothing else — into `__VIGOV_XA_CO_DINH__`, so a commune's own app opens
straight into that commune with no group introduction and no confirmation step. It only steers the
UI, like `d` on a public QR; the server still resolves the commune. Only `deploy.mjs` sets it, always
equal to `--domain`. Widening it (another value, another source) is a new decision.

---

## The three layers — keep them separate

ADR 0044 did not replace this. Collapsing discovery into session is the mistake this whole skill
exists to prevent.

| Layer | Answers | Source | Trusted? |
|---|---|---|---|
| **Discovery** | Which commune does this open point at | QR / deep link (main app) · App ID read by the client (own app) | **No — drives the UI only** |
| **Session** | Which commune is this session *acting in* | Server: after the citizen confirms a QR, or from the App ID the app secret verified | **Yes — server-issued** |
| **Authorization** | What may this citizen read/write there | Citizen↔commune relationship + rule 4 | **Yes** |

A deep-link parameter, and an App ID the client reads from its own URL, are **client-supplied
data**. Rule 1 forbids accepting a commune from the client — so they drive the UI, and the
server decides what they mean. An App ID that does not resolve, or resolves to an inactive
commune → **refuse**; there is no default commune.

| Operation | How the commune is authorised |
|---|---|
| **Read** own records | Citizen identity from session **and** an existing relationship |
| **Submit** something new | **Only to the session's commune.** If the citizen has no prior link to it, the submission creates a `chua_khai` relationship (no residence declaration for this pair — ADR 0023) |
| **Switch** commune | **No switch action.** Main app only: scanning another commune's QR asks for confirmation, then the server issues a **new** session — audited. Own app: never |

---

## The API host is singular

The Mini App talks to **one citizen API host**, in both modes. It never constructs a
per-commune URL.

> *"The client says which commune"* — **forbidden**.
> *"The server records which commune this session is acting in"* — **correct**.

Pointing the app at per-commune domains re-creates the problem the platform was built to
avoid: N CORS configurations, N certificates, a client choosing its own backend, and printed
QR codes that break when a commune's domain is reassigned.

---

## Deep link format

`t=<tenant_ulid>` is **gone** (ADR 0047, 2026-09-27; no QR had been issued). The parameter now
carries the commune's **domain** (e.g. `xa-a.vigov.vn`); its exact name and the fate of `v` are
open in ADR 0047.

| Param | Why it exists |
|---|---|
| domain | A **lookup key** the server resolves to a commune via platform `ResolveHost`. Never stored as the commune reference: session, remembered commune and audit keep the **ULID** (rule 1 invariant 2 / forbidden #5). After a merger the operator re-points the old domain to the successor commune, audited, so printed QRs keep opening |
| `src` | `qr` (a QR we issued, naming one commune) · `zns` (the commune sent it). **Nothing else** — `share` was removed by the owner on 2026-09-25 (ADR 0045 §Trả lời). Also shows which channel actually works |

The confirmation screen's commune name comes from an identity route (domain → name, province;
`CitizenOnly` + `KhongThuocXa`, **never returns the ULID**) and the domain reaches the session
bridge in a **new optional** `OpenCitizenSessionRequest` field — `tenant_hint` is never repurposed
to carry a domain (rule 2 forbidden #4); the `.proto` owns the new field's name and shape. Both are **not built yet** (ADR 0047 answers 6, 7).

### Trust weighting by source

| `src` | Meaning | Behaviour |
|---|---|---|
| `qr` | Scanned at the commune's own noticeboard | Show the commune, **one tap to confirm** |
| `zns` | The commune sent it to this citizen | Show the commune, one tap to confirm |
| *any other `src`, or a domain without `src`* | Not a channel we issue | **The domain is ignored** — behave exactly as *(none)*. Never pre-select, never "confirm explicitly" |
| *(none)* | Opened from the app list or Zalo search | Main app: the server's remembered commune, else introduction only. Own app: the app's commune |

Someone standing at the commune office should not be made to search for the commune they are
standing in. A commune enters the main app only through a link **we** issued that names it —
there is no forwarded-link path to design for, so there is no picker to fall back on.

### A deep link never carries access

A citizen forwards "my petition" to a neighbour; the neighbour must not see it (rule 4). The
link may carry a **commune**, never a record identifier that grants a read.

---

## Design for the case with NO parameter — that is the common path

This is where a "route by parameter" design usually breaks.

| How the app is opened | Parameter present? |
|---|---|
| Scanning the QR at the office | yes |
| Tapping a link the commune sent | yes |
| **Opening from the pinned app list** | **no** |
| **Finding it in Zalo search** | **no** |
| **Coming back next week** | **no** |

In the main app the QR is how a commune enters **the first time**. From then on the **server**
remembers the confirmed commune against the citizen's identity; the client stores no commune on
the device. That needs the citizen's identity **before** the server can answer — how a reopen
without QR obtains it is the session-bridge design's question (ADR 0044 §*Hệ quả*), not
something to improvise in the client. A design that treats the parameter as the main road fails
on every user's second open.

---

## No commune picker — and why that is not a gap

ADR 0044 replaced ADR 0005's no-parameter path (picker, GPS, profile). Do not rebuild it:

- **Main app**: without a QR the citizen sees only the group introduction. A picker would let
  anyone enter any commune's content from a public store listing.
- **Own app**: the commune is fixed by the verified App ID. A picker would be a second, weaker
  source for a fact the server already knows.
- **GPS** has no role in choosing a commune: locations are spoofable, and urban boundaries run
  down the middle of streets.

A QR printed for a **merged** commune must still resolve. Since ADR 0047 that is done by
re-pointing the old **domain** to the successor tenant (a tenant may hold several domains —
`service-platform/internal/domain/tenant.go:19-25`), audited — not by a `t` lookup.

---

## Notifications: one verifying OA, many sending OAs

Each commune has **its own Zalo OA** for notifications, because citizens expect the sender to be
*"UBND xã X"*. A commune's standing with its own residents is part of what the platform exists
to serve.

| # | Practice |
|---|---|
| 1 | The notification channel sits behind an adapter; business code never names Zalo |
| 2 | OA credentials are **per commune**, held in the secret store, never in configuration files |
| 3 | A commune with no OA configured yet **degrades visibly** — staff see a "not notified" flag, never silence |
| 4 | Sending is eventually consistent with the business write: a petition must **never** fail to be accepted because a notification could not be sent |
| 5 | Notification payloads carry a **business code**, never personal data — queues are persisted and backed up |

The **verifying** OA is a different role: one OA, `Vihat` (ADR 0031), verifies **every** app —
the main app and each commune's own app. The project owner confirmed 2026-09-25 that one OA can
verify several apps (ADR 0044 answer 5), so ADR 0018 stands unchanged.

→ ADR `kb/10-decisions/0006-per-commune-zalo-oa.md` — **superseded by**
`kb/10-decisions/0018-oa-xac-thuc-tach-khoi-oa-thong-bao.md` (verifying OA separate from
per-commune notification OA; read 0018 before touching this section) · ADR 0031 · ADR 0044

---

## REQUIRED

| # | Practice |
|---|---|
| 1 | The commune comes from the **server** — confirmed QR (main app) or verified App ID (own app). Never from GPS, a picker, or a client-read App ID |
| 2 | The session's commune name appears on **every** screen |
| 3 | No switch action. A new commune in the main app = new QR, explicit confirmation, **new session, audited** |
| 4 | API base URL and commune data are read **at runtime**, never baked into the bundle; one build serves every App ID — except the commune domain of a `--vao-thang` build (ADR 0047 §6) |
| 5 | Confirm the commune at the **final step** before submitting anything |
| 6 | The app works correctly when opened with **no parameter at all** — in both modes |
| 7 | Nothing personal in logs, URLs, or file names |

Items 2 and 5 are business rules, not polish: submitting to the wrong commune means that
commune receives work outside its territory, has to redirect or refuse, and the citizen waits
for nothing and then loses trust.

---

## STOP CONDITIONS

1. A flow that would need the client to pick which backend to call
2. A link that would grant access to a record rather than name a commune
3. A commune that has merged — what a QR printed for the old one should do
   (→ ADR 0047: re-point the old domain to the successor, audited. Not "alias": commune A does
   not *become* commune B)
4. Anything depending on a Zalo platform behaviour nobody has verified — **check the vendor
   documentation, do not assume**
5. A proposal to put **per-commune values in the build** beyond choosing the target App ID, or
   to let the **client choose the commune** from the App ID or a URL parameter (ADR 0044 stop
   conditions #1, #2; ADR 0047). Storing the **domain** as the commune reference anywhere
   (ADR 0047 stop condition #1)
6. A request for one citizen, inside one app, to act with **several communes**; or for a
   commune's People's Committee to **own** its app (ADR 0044 stop conditions #3, #4)

→ Rule 1 · Rule 4 · `skills/accessibility-elderly` · `skills/citizen-identity-multi-tenant`
→ `kb/00-foundation/multi-tenant-model.md` · ADR 0005 (three layers, deep link — partly
  superseded by 0044) · ADR 0006 (superseded by 0018) · ADR 0018 (one verifying OA,
  per-commune notification OA) · ADR 0019 (QR session pairing) · ADR 0020 (citizen phone
  verification via `getPhoneNumber`) · ADR 0023 (entity, table and URL-resource names for this
  channel; two-level local government) · ADR 0031 (ViHAT Group, OA `Vihat`) · **ADR 0044 (two
  modes, one build)** · **ADR 0047 (two build flows by domain; domain parameter replaces `t`)**
