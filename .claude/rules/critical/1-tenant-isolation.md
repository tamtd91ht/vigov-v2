# RULE 1 — Tenant isolation between communes

One system, many communes, told apart by **domain**. Data leaking from one commune to
another is a **data breach between two government bodies** — heavier than a software bug,
because it touches jurisdiction over a territory.

This is the **first** of three isolation dimensions. All three must hold:

| Rule | Question it answers |
|---|---|
| **1 (this file)** | Which **commune** does this request belong to |
| 4 | Whose data may this **citizen** see |
| 5 | What may this **staff member** do |

**Getting two of three right is still a data leak.**

## INVARIANTS

| # | Invariant |
|---|---|
| 1 | Every business entity carries `tenant_id`. Not an optional field |
| 2 | `tenant_id` is an **opaque, immutable identifier** (ULID) — NOT an administrative code, domain name, or commune name |
| 3 | The commune is derived from `Host` at the **outermost edge**, before any handler. Cannot resolve = **404** |
| 4 | `tenant_id` travels in `context.Context`, never as a function argument |
| 5 | Every query goes through a **scoped** repository. No path to the raw store |
| 6 | Every unique key is **composite with `tenant_id`** |
| 7 | Realtime rooms, cache keys, file paths, queues: prefixed `t:<tenant_id>` |
| 8 | Tokens carry `tenant_id`; every request compares it with the commune derived from `Host`. Mismatch = **401 + alert** |
| 9 | Background work carries `tenant_id` **inside the message**. A consumer without it **refuses; it never guesses** |
| 10 | Commune-specific values are read **at runtime** from per-tenant configuration — not from environment variables, never baked into a bundle |

## STRICTLY FORBIDDEN

| # | Forbidden | Why |
|---|---|---|
| 1 | A default on the isolation path (`?? default`, optional `tenant_id`) | One line collapses the whole isolation, **silently**, with tests still green |
| 2 | Accepting `tenant_id` from body / query / client header | A client naming its own commune is a client granting itself access |
| 3 | Session cookie scoped to the parent domain (`.vigov.vn`) | The session is sent to **every** other commune's subdomain |
| 4 | Single-column unique key on business data | The second commune cannot onboard; document numbering breaks records rules |
| 5 | Using an administrative code / domain / commune name as `tenant_id` | A commune merger would force rewriting keys on **archival records** |
| 6 | A cross-commune query without `// @cross-tenant: <reason>` | Cross reads must be explicit and audited |
| 7 | Deleting data of a merged commune | Archival records. Mark inactive, keep the data |

**Why invariant #2 is the expensive one:** Vietnam periodically reorganises commune-level
administrative units. If `tenant_id` carries meaning, the first merger forces **rewriting
foreign keys across all historical data** — rewriting archival records, which the law does
not permit. Getting it right now costs nothing.

## STOP CONDITIONS — ask the user, never decide alone

1. A new entity where it is **unclear** whether it belongs to a commune or to the platform
2. A need to read data across **multiple communes** (district/province reporting)
3. **Merging / splitting / renaming** an administrative unit
4. A citizen needing to act with **more than one commune**
5. A **vendor** administrator needing to touch a commune's business data

→ Enforcement: `hooks/tenant_scope_guard.py` (BLOCK)
→ Skills: `skills/go-tenant-context` · `skills/admin-unit-merge`
→ Foundation: `kb/00-foundation/multi-tenant-model.md`
