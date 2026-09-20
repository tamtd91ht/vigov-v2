# comms

**Thông tin – truyền thông** — Articles, radio bulletins, video, map layers, the public directory, and notifications to citizens and staff.

## Owns

Article · RadioBulletin · Video · MapLayer · MapPin · GovContact · Notification · **CitizenNotification** · MapAssetType

`Notification` and `CitizenNotification` are **two entities, not one split in two**. The first is
the staff-facing notice (`docs/ui-ux/08-thong-bao.md`, table `thong_bao`, permission
`announcement.create`); the second is the ledger of messages sent **out of the commune to one
citizen about that citizen's own record** (table `thong_bao_gui_cong_dan`, migration 0004). One
Vietnamese word covers both — `kb/00-foundation/ubiquitous-language.md:148` — and merging them is
how an internal notice reaches the citizen channel.

Ownership is authoritative in `kb/30-indexes/data-ownership.json` (GENERATED — run `make kb`).
No other service may open this service's schema; they read through gRPC or events (rule 2).

## Notifications go out through a per-commune Official Account

Each commune has **its own Zalo OA**: citizens expect the sender to be *"UBND xã X"*. So this
service holds a **multi-OA adapter**, with credentials as per-commune configuration in the
secret store — never in a config file, never in a process environment variable.

A commune with no OA configured yet **degrades visibly**: staff see a "not notified" flag.
Sending is eventually consistent with the business write — a petition must never fail to be
accepted because a notification could not be sent.

ADR 0006's **unverified precondition** — that one Mini App can work with many OAs — was checked on
17/09/2026 and is **false**. ADR 0018 supersedes it: the app has **one** OA (platform-wide, for
authentication), messages have **many** (one per commune, over ZNS, by phone number). The six
architectural consequences above survive unchanged; read ADR 0018 first, then ADR 0006 for their
full text.

→ ADR 0018 (supersedes) · ADR 0006

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
