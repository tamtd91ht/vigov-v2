---
name: events-and-queues
description: Use when working with events, queues, consumers, background jobs, or scheduled work. Triggers on: event, queue, consumer, producer, publish, subscribe, NATS, Kafka, RabbitMQ, idempotent, retry, DLQ, dead letter, background job, cron, scheduler.
---

# Skill: Events and queues

## REQUIRED

| # | Practice |
|---|---|
| 1 | Messages carry `tenant_id`. A consumer without it **refuses; it never guesses** (rule 1) |
| 2 | Messages carry a stable `message_id` for deduplication |
| 3 | Consumers are **idempotent**: reprocessing the same message yields the same result |
| 4 | Event names are in the **past tense** plus a version: `petitions.received.v1` |
| 5 | Events are published **after** the transaction commits (outbox), never inside it |
| 6 | Exhausted retries go to a dead-letter queue plus a flag staff can see |
| 7 | Scheduled work runs **per commune**, and is resumable |

## Kafka names — the profile is part of every name

dev and prod share **one** Kafka cluster (owner, 17/10/2026). The event NAME stays
profile-free (`comms.zalo_oa_event_received.v1`, inside the envelope and the `.proto`); the
Kafka NAMES carry `ENV` as a **prefix**:

| Kafka object | Shape | Example | Built by |
|---|---|---|---|
| Topic | `<ENV>.<event name>` | `prod.comms.zalo_oa_event_received.v1` | `events.Topic(cfg.Env, name)` |
| Consumer group | `<ENV>.<service>.<purpose>` | `prod.comms.zalo-oa-events` | `events.ConsumerGroup(cfg.Env, service, purpose)` |

- `ENV` is the existing variable (`dev` · `staging` · `prod`). Missing or unknown → **refused**
  (`events.ErrNoProfile`), never defaulted — the default would be another environment's stream.
- Prefix, not suffix: the broker's ACLs grant `prod.` as one prefixed resource.
- Only `core/events/**` imports a Kafka client and builds a name. Services publish through
  `events.Publisher` (`core/events/kafka`) and never type a topic or group.
- Ops create each topic once per profile (`dev.…` and `prod.…`); code never auto-creates.

**Why:** an unprefixed name is the same topic for both environments — a dev pod publishes test
events into prod's stream, and a dev consumer joining the same group takes prod's partitions,
processing prod messages against the dev database. Silent, tests green.

→ Enforcement: `hooks/service_boundary_guard.py` check 5 (BLOCK) · code: `core/events/names.go`

## FORBIDDEN

A Kafka topic or consumer group typed by hand, or without the `<ENV>.` prefix ·
publishing inside an uncommitted transaction (the event arrives before the data) · adding a
**required** field to a published event · consumers assuming ordering · messages carrying
full personal data (carry the **code**; the receiver looks it up).

## Why messages must not carry personal data

Queues are persisted, backed up, and inspected during debugging. A message carrying a full
phone number means that number lives in the queue infrastructure indefinitely — rule 3.

→ Rule 2 · Rule 3 · `skills/transaction-boundary`
