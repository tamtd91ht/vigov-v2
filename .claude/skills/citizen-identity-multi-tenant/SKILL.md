---
name: citizen-identity-multi-tenant
description: Use when working on citizen identity, OTP, citizen sessions, or the relationship between a citizen and a commune. Triggers on: citizen, OTP, citizen auth, phone number, permanent residence, temporary residence, identity, weak identity, citizen session, switch commune, multiple communes.
---

# Skill: Citizen identity in a multi-commune system

## What the naive model misses

"Phone number unique across the system" translates, in business terms, to: *one citizen
belongs to exactly one commune, forever*. That is wrong in real life in at least four ways:

| Situation | Frequency |
|---|---|
| Registered in commune A, living and working in commune B | Very common in peri-urban areas |
| Standing in commune A, reporting an issue **seen in commune B** | Common |
| Moving household registration from A to B | Periodic |
| Filing on behalf of a relative in another commune | Common with elderly citizens |

## The correct model

| Thing | Scope |
|---|---|
| Citizen identity (verified phone number) | **Platform-wide**, one record |
| Citizen ↔ commune relationship | **Many-to-many** across the platform, with a role (`permanent`/`temporary`/`transient`) and validity — one person may use several communes' apps |
| Login session | Carries **exactly one commune**, set by the server (confirmed QR in the main app, verified App ID in a commune's own app — ADR 0044). No switch action: a different commune means a **new session**, and issuing it is **audited** |

## REQUIRED

| # | Practice |
|---|---|
| 1 | The unique key on phone number is **platform-wide**, not per commune |
| 2 | The unique key on the relationship is **(citizen, commune)** |
| 3 | Every citizen-path query filters by **session identity** and **the session's commune** |
| 4 | The backend **does not trust** the commune sent by the client — it checks the session's relationships |
| 5 | OTP is a **weak identity**: never sufficient alone for an act with legal consequences |
| 6 | The OTP store is keyed by **(phone, commune)** — requesting an OTP in A must not block B |

## Stop condition

A citizen acting on behalf of someone else (representative, relative) → **ask the user**.
That is delegation with legal consequences, not a convenience feature.

→ Rule 4 · `kb/00-foundation/multi-tenant-model.md` · ADR 0044 (one commune per session, two
  app modes) · `skills/zalo-miniapp-multi-tenant`
