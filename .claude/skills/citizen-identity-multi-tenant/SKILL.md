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
| Citizen ↔ commune relationship | **Many-to-many**, with a role (`permanent`/`temporary`/`transient`) and validity |
| Login session | Carries **one currently selected commune**, switchable, and **every switch is audited** |

## REQUIRED

| # | Practice |
|---|---|
| 1 | The unique key on phone number is **platform-wide**, not per commune |
| 2 | The unique key on the relationship is **(citizen, commune)** |
| 3 | Every citizen-path query filters by **session identity** and **selected commune** |
| 4 | The backend **does not trust** the commune sent by the client — it checks the session's relationships |
| 5 | OTP is a **weak identity**: never sufficient alone for an act with legal consequences |
| 6 | The OTP store is keyed by **(phone, commune)** — requesting an OTP in A must not block B |

## Stop condition

A citizen acting on behalf of someone else (representative, relative) → **ask the user**.
That is delegation with legal consequences, not a convenience feature.

→ Rule 4 · `kb/00-foundation/multi-tenant-model.md`
