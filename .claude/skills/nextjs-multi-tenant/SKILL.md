---
name: nextjs-multi-tenant
description: Use when building the staff admin web in Next.js under multi-tenancy — per-domain configuration, route protection, session cookies. Triggers on: Next.js, App Router, admin web, runtime config, NEXT_PUBLIC, middleware, cookie, domain, subdomain, staff UI.
---

# Skill: Next.js admin web under multi-tenancy

## The rule that decides everything else

> **A commune-specific value is never baked into the bundle.**

`NEXT_PUBLIC_*` is substituted at **build time**. One bundle cannot carry the names of 300
communes. Baking it in means building and hosting separately per commune — which is exactly
the per-commune packaging model this project rejected.

| Value kind | Example | Where it lives |
|---|---|---|
| Platform constant | API base URL, product name | `NEXT_PUBLIC_*` |
| **Per commune** | commune name, parent authority, logo, map centre, SLA, catalogues | **Read at runtime by `Host`** |

## REQUIRED

| # | Practice |
|---|---|
| 1 | Commune configuration is loaded server-side from `Host` and passed down through context |
| 2 | A `Host` matching no commune → **404**, never a fallback commune |
| 3 | Session cookies are scoped to each commune's own host. **Never** `domain=.vigov.vn` |
| 4 | Route protection lives in **server-side middleware**, not only in hidden buttons |
| 5 | Hiding buttons by permission is **UX**, not security — the backend still checks |

## Why item 3 deserves underlining

A cookie scoped to the parent domain is sent to **every subdomain**. One line of
configuration, harmless-looking, written once — and it **disables the entire isolation
between communes**, while every functional test stays green.

→ Rule 1 · Rule 5 · `kb/00-foundation/multi-tenant-model.md`
