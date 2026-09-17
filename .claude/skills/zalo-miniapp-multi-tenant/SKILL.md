---
name: zalo-miniapp-multi-tenant
description: Use when building citizen-facing screens or flows in the Zalo Mini App, especially resolving which commune the citizen is acting with, deep links, QR codes, the commune picker, or ZNS notifications. Triggers on: Zalo, Mini App, zmp-sdk, citizen app, deep link, QR, commune selection, commune picker, GPS, location, miniapp, citizen channel, ZNS, OA, notification.
---

# Skill: Zalo Mini App in a multi-commune system

## The contradiction to understand before writing anything

"Tell communes apart by domain" is right for the admin webs and **does not apply** here — and
the reason is not ours to change:

| Platform fact | Consequence |
|---|---|
| A Mini App is identified by its **Zalo App ID**, not a domain | There is no commune domain to key off |
| Each App ID needs its own registration and review | Hundreds of communes cannot be hundreds of apps |
| Citizens find the app by searching Zalo | Many similarly named apps cause wrong choices, worst for elderly users |

**One Mini App serves every commune.** The commune is resolved at runtime.

There is also no "app for Đà Nẵng" and no app-per-province. A province is a **filter in the
picker**, never a separate app: province boundaries themselves were redrawn in 2025, so a
discovery tree pinned to them is pinned to something that moves.

---

## The three layers — keep them separate

Collapsing discovery into session is the mistake this whole skill exists to prevent.

| Layer | Answers | Source | Trusted? |
|---|---|---|---|
| **Discovery** | Which commune does the citizen *want* | QR · deep link · GPS · picker · profile | **No — a hint only** |
| **Session** | Which commune is this session *acting in* | Server records it after the citizen confirms | **Yes — server-issued** |
| **Authorization** | What may this citizen read/write there | Citizen↔commune relationship + rule 4 | **Yes** |

A deep-link parameter is **client-supplied data**. Rule 1 forbids accepting a commune from the
client — so the parameter drives the UI, and the server decides what it means:

| Operation | How the commune is authorised |
|---|---|
| **Read** own records | Citizen identity from session **and** an existing relationship |
| **Submit** something new | Any **active** commune is allowed; the submission itself creates a `chua_khai` relationship — no residence declaration for this (citizen, commune) pair |
| **Switch** commune | Explicit action, re-issues the session, **audited** |

Submitting to a commune the citizen has no prior link to is legitimate — someone reports a
pothole they saw while passing through. That is what the third capacity value exists for. It is
**not** named "transient": the sender may be sitting in another province entirely, and the system
has no way to know where they are. `chua_khai` states only what is actually known — no residence
declaration yet. → ADR 0023.

---

## The API host is singular

The Mini App talks to **one citizen API host**. It never constructs a per-commune URL.

> *"The client says which commune"* — **forbidden**.
> *"The server records which commune this session is acting in"* — **correct**.

Pointing the app at per-commune domains re-creates the problem the platform was built to
avoid: N CORS configurations, N certificates, a client choosing its own backend, and printed
QR codes that break when a commune's domain is reassigned.

---

## Deep link format

```
https://zalo.me/s/<APP_ID>/?t=<tenant_ulid>&src=qr&v=1
```

| Param | Why it exists |
|---|---|
| `t` | The **opaque ULID**, never a name or an administrative code. A QR printed on a noticeboard outlives renames, domain changes and mergers — this is where ADR 0004 pays off |
| `src` | `qr` (noticeboard at the office) · `zns` (the commune sent it) · `share` (passed between citizens). Drives trust weighting below, and shows which channel actually works |
| `v` | Parameter schema version. **Printed QR codes live for years**; when the format changes, the old ones must still resolve |

### Trust weighting by source

| `src` | Meaning | Behaviour |
|---|---|---|
| `qr` | Scanned at the commune's own noticeboard | Pre-select, **one tap to confirm** |
| `zns` | The commune sent it to this citizen | Pre-select, one tap to confirm |
| `share` | Passed between citizens, provenance unknown | **Always require explicit selection** |
| *(none)* | Opened from the app list or Zalo search | Fall back to the profile, then GPS, then the picker |

Someone standing at the commune office should not be made to search for the commune they are
standing in. Someone following a forwarded link should.

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

Deep link is priority 1 **for the first visit**. From the second visit onward the default path
is the saved profile. A design that treats the parameter as the main road fails on every
user's second open.

---

## The commune picker

Ordered for the citizen, not for the data model:

1. **Recently used** — most citizens deal with one or two communes, ever
2. **GPS suggestion** — labelled as a suggestion, one tap to accept, **never auto-selected**
3. **Search by name** — diacritic-insensitive, and **matching former names** (mergers mean the
   name a citizen knows may no longer be the official one)
4. **Browse by province** — the fallback, not the main route. **Two levels, not three**: since
   01/07/2025 local government is province → commune and the district level has ceased to exist.
   That is exactly why browsing comes last: one province expands into a flat list of a hundred
   rows. Never re-introduce districts as a grouping tier, not even from historical data (ADR 0023)

GPS suggests and never decides: locations are spoofable, and urban boundaries run down the
middle of streets. Someone on the wrong side of a road is not in another commune.

---

## Notifications: one app, many Official Accounts

Each commune has **its own Zalo OA**, because citizens expect the sender to be *"UBND xã X"*.
A commune's standing with its own residents is part of what the platform exists to serve.

| # | Practice |
|---|---|
| 1 | The notification channel sits behind an adapter; business code never names Zalo |
| 2 | OA credentials are **per commune**, held in the secret store, never in configuration files |
| 3 | A commune with no OA configured yet **degrades visibly** — staff see a "not notified" flag, never silence |
| 4 | Sending is eventually consistent with the business write: a petition must **never** fail to be accepted because a notification could not be sent |
| 5 | Notification payloads carry a **business code**, never personal data — queues are persisted and backed up |

→ ADR `kb/10-decisions/0006-per-commune-zalo-oa.md` — **superseded by**
`kb/10-decisions/0018-oa-xac-thuc-tach-khoi-oa-thong-bao.md` (Zalo binds ONE Mini App to ONE
verifying OA; read 0018 before touching this section)

---

## REQUIRED

| # | Practice |
|---|---|
| 1 | GPS **suggests**, never **decides** |
| 2 | Once selected, the commune name appears on **every** screen |
| 3 | Switching commune is explicit, never automatic, and audited |
| 4 | API base URL and commune data are read **at runtime**, never baked into the bundle |
| 5 | Confirm the commune at the **final step** before submitting anything |
| 6 | The app works correctly when opened with **no parameter at all** |
| 7 | Nothing personal in logs, URLs, or file names |

Items 2 and 5 are business rules, not polish: submitting to the wrong commune means that
commune receives work outside its territory, has to redirect or refuse, and the citizen waits
for nothing and then loses trust.

---

## STOP CONDITIONS

1. A flow that would need the client to pick which backend to call
2. A link that would grant access to a record rather than name a commune
3. A commune that has merged — what a QR printed for the old one should do
   (→ the platform service's **succession** table, `TenantSuccession`; ADR 0005, named in
   ADR 0023. Not "alias": commune A does not *become* commune B)
4. Anything depending on a Zalo platform behaviour nobody has verified — **check the vendor
   documentation, do not assume**

→ Rule 1 · Rule 4 · `skills/accessibility-elderly` · `skills/citizen-identity-multi-tenant`
→ `kb/00-foundation/multi-tenant-model.md` · ADR 0005 · ADR 0006 (superseded by 0018) · ADR 0018
  (one verifying OA, per-commune notification OA) · ADR 0019 (QR session pairing) · ADR 0020
  (citizen phone verification via `getPhoneNumber`) · ADR 0023 (entity, table and URL-resource
  names for this channel; two-level local government)
