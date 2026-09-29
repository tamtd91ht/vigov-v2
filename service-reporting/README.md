# reporting

**Read model** — Statistics, rollups and cross-domain search. Owns NO source data — every row is a projection built from events.

## Owns

ReportingSystemMessageOverride — a commune's own wording of the 38 `report.*` sentences
(`system_message_override`, migration 0003; ADR 0024 §Phụ, *Bổ sung 29/09/2026*). The one table here
a commune edits; still NO source business data — everything else is a projection.

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
