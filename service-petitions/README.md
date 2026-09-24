# petitions

**Tiếp dân** — Citizen feedback (phản ánh), the tasks that arise from it and from meeting conclusions, and their SLA. Citizen letters (`don_thu`: complaints, denunciations, proposals, requests) are NOT here — they belong to `documents` (ADR 0039).

## Owns

Petition · PetitionCategory · SlaRule · Task

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
- No repository or gRPC call inside a loop; data fetched once travels down as a parameter (`skills/load-data-once`)
