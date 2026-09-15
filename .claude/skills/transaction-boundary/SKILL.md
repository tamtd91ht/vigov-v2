---
name: transaction-boundary
description: Use when designing a multi-step business flow that spans services — consistency level, compensation, saga, transactions. Triggers on: transaction, consistency, saga, compensation, eventual, rollback, business flow, multi-step, two services, distributed transaction.
---

# Skill: Transaction boundaries

In public administration this is **not a purely technical matter**: an administrative file
must **never be half-processed**. If "petition received" succeeds but "audit entry written"
fails, the system sits in a state the records-retention rules do not permit.

## Decide before writing the first line

| Question | If... | Then |
|---|---|---|
| May the two steps disagree for a moment? | **No** | **Strong** consistency — one service, one transaction |
| | **Yes, a few seconds** | **Eventual** consistency — events plus compensation |
| If the later step fails, must the earlier one be undone? | **Yes** | Needs **explicit compensation**, not a retry |

## REQUIRED

| # | Practice |
|---|---|
| 1 | Every multi-step flow is declared in `kb/30-indexes/transaction-boundaries.json`, including the `why` field |
| 2 | The business write and its audit entry (rule 6) share **one transaction**, always |
| 3 | Under eventual consistency the **intermediate state must be visible** to staff |
| 4 | Compensation is an **audited business action**, not a silent rollback |
| 5 | Consumers are **idempotent** — queues deliver at least once |
| 6 | Exhausting retries goes to a dead-letter queue **plus a visible flag** — never silence |

## Why the `why` field cannot be dropped

No tool can infer it. Without it, a future session will "fix" an eventually consistent step
into a synchronous call **to be safe**, and take down petition intake every time the third
party has an outage.

```json
{
  "notify_citizen": {
    "consistency": "eventual",
    "max_lag": "30 seconds",
    "compensation": "retry 3x -> dead-letter -> staff sees a 'not notified' flag",
    "why": "The messaging provider can be down. Petition intake must NEVER fail because a notification could not be sent."
  }
}
```

→ Rule 2 · Rule 6 · `skills/events-and-queues`
