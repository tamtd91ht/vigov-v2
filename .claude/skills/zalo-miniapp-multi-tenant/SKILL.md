---
name: zalo-miniapp-multi-tenant
description: Use when building citizen-facing screens or flows in the Zalo Mini App, especially resolving which commune the citizen is acting with. Triggers on: Zalo, Mini App, zmp-sdk, citizen app, deep link, QR, commune selection, GPS, location, miniapp, citizen channel.
---

# Skill: Zalo Mini App in a multi-commune system

## The contradiction to understand before writing anything

"Tell communes apart by domain" is right for the admin web and **does not apply** to the
Mini App — and the reason is not ours to change:

| Platform fact | Consequence |
|---|---|
| A Mini App is identified by its **Zalo App ID**, not a domain | There is no "commune domain" to key off |
| Each App ID needs its own registration and review | 300 communes cannot be 300 mini apps |
| Citizens find the Mini App by searching Zalo | 300 similarly named apps cause wrong choices, especially for elderly users |

**One Mini App serves every commune.** The commune is resolved at runtime, not at build time.

## Resolution order

| Priority | Source | Confidence |
|---|---|---|
| 1 | **Deep link** — QR posted at the commune office, links the commune sends by message | High |
| 2 | **Previously selected commune**, stored on the citizen profile | High |
| 3 | **GPS** matched against administrative boundaries | Medium — a **hint** only |
| 4 | **Manual selection** from a searchable list | Last resort, and **always available** |

## REQUIRED

| # | Practice |
|---|---|
| 1 | GPS **suggests**, never **decides** — location can be spoofed, and urban boundaries run down the middle of streets |
| 2 | Once selected, **show the commune name on every screen** |
| 3 | Switching commune is an **explicit action**, never automatic from GPS |
| 4 | The API base URL and commune name are read **at runtime**, never baked into the bundle |
| 5 | Before submitting feedback: **confirm the commune** at the final step |

## Why items 2 and 5 are business rules, not UI polish

Submitting feedback to the wrong commune is a **real business incident**: the commune
receives work outside its territory, has to redirect or refuse, and the citizen waits for
nothing and then loses trust.

→ Rule 4 · `skills/accessibility-elderly` · `kb/00-foundation/multi-tenant-model.md`
