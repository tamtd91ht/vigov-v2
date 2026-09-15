# identity

**Tổ chức – cán bộ** — Staff accounts and RBAC (per commune); citizen identity (platform-wide) and the citizen↔commune relationship; sessions and OTP.

## Owns

Staff · Role · Permission · OrgNode · Citizen · CitizenCommune · Session · OtpCode

Ownership is authoritative in `kb/30-indexes/data-ownership.json` (GENERATED — run `make kb`).
No other service may open this service's schema; they read through gRPC or events (rule 2).

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
