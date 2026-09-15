# platform

**Nền tảng** — Commune registry, domains, quotas, commune lifecycle. Cross-tenant BY DESIGN; the only service besides identity with a legitimate cross-commune data area.

## Owns

Tenant · Domain · Quota · TenantLifecycleEvent · **TenantAlias**

Ownership is authoritative in `kb/30-indexes/data-ownership.json` (GENERATED — run `make kb`).
No other service may open this service's schema; they read through gRPC or events (rule 2).

## TenantAlias — why it exists

`old tenant / old name / old code -> current tenant`.

A QR code printed on a commune noticeboard is a physical object that outlives renames, domain
changes and mergers. Without this table every printed QR becomes waste at the first merger —
and mergers are certain, not hypothetical. The picker's search reads it too: the name a
citizen knows may no longer be the official one.

→ ADR 0005

## Layout

```
cmd/server/      wiring only
internal/
  domain/        business types and pure rules — imports NOTHING but the standard library
  app/           use cases; takes interfaces, never concrete types
  store/         the only place that knows SQL
  http/          routing and explicit permission declarations
  event/         publishing and consuming
migrations/      per-commune, resumable, reversible
```

## Non-negotiables

- Every query goes through `store.For(ctx)` — there is no path to the raw database
- Every business write shares a transaction with its `audit.Write` (rule 6)
- Every unique key is composite with `tenant_id`; every index starts with it (ADR 0004)
- Every route declares a permission explicitly (rule 5)
