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
| 4 | Event names are in the **past tense** plus a version: `petition.received.v1` |
| 5 | Events are published **after** the transaction commits (outbox), never inside it |
| 6 | Exhausted retries go to a dead-letter queue plus a flag staff can see |
| 7 | Scheduled work runs **per commune**, and is resumable |

## FORBIDDEN

Publishing inside an uncommitted transaction (the event arrives before the data) · adding a
**required** field to a published event · consumers assuming ordering · messages carrying
full personal data (carry the **code**; the receiver looks it up).

## Why messages must not carry personal data

Queues are persisted, backed up, and inspected during debugging. A message carrying a full
phone number means that number lives in the queue infrastructure indefinitely — rule 3.

→ Rule 2 · Rule 3 · `skills/transaction-boundary`
