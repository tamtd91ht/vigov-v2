# comms

**Thông tin – truyền thông** — Articles, radio bulletins, video, map layers, the public directory, and notifications to citizens and staff.

## Owns

Article · RadioBulletin · Video · MapLayer · MapPin · GovContact · Notification

Ownership is authoritative in `kb/30-indexes/data-ownership.json` (GENERATED — run `make kb`).
No other service may open this service's schema; they read through gRPC or events (rule 2).

## Notifications go out through a per-commune Official Account

Each commune has **its own Zalo OA**: citizens expect the sender to be *"UBND xã X"*. So this
service holds a **multi-OA adapter**, with credentials as per-commune configuration in the
secret store — never in a config file, never in a process environment variable.

A commune with no OA configured yet **degrades visibly**: staff see a "not notified" flag.
Sending is eventually consistent with the business write — a petition must never fail to be
accepted because a notification could not be sent.

⚠ ADR 0006 records an **unverified precondition**: that one Mini App can work with many OAs.
Check the vendor documentation before writing the first line of this channel.

→ ADR 0006

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
