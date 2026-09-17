# RULE 2 — Service boundaries and data ownership

Without this rule the system becomes a **distributed monolith**: the full cost of
microservices with none of the benefit. The decay has **no symptoms** — tests green,
features working, it is just that nothing can be changed any more.

## INVARIANTS

| # | Invariant |
|---|---|
| 1 | Every entity has **exactly one owning service**, declared in `kb/30-indexes/data-ownership.json` |
| 2 | A service **never** opens a connection to another service's database |
| 3 | Reading another service's data: **gRPC** (sync) or **events** (async). There is no third path |
| 4 | Events carry the **version in the name** (`petitions.received.v1`); two versions run side by side during a migration |
| 5 | Consumers are **idempotent** — queues deliver at least once |
| 6 | Every business flow declares its **consistency level** and **compensation** in `kb/30-indexes/transaction-boundaries.json` |
| 7 | `.proto` is the **source of truth** for contracts **between services**. Code is generated from it, never written by hand. The browser-facing REST surface is a different surface — ADR 0014 |
| 8 | Every inter-service call carries `tenant_id` in metadata (rule 1) |

## STRICTLY FORBIDDEN

| # | Forbidden | Why |
|---|---|---|
| 1 | Importing another service's `internal/` package | The boundary is broken at compile time |
| 2 | A connection string to a database this service does not own | A read path around the contract |
| 3 | Hand-editing files generated from `.proto` | Silently lost on the next generate |
| 4 | Adding a **required** field to a published event | Existing consumers break. Add optional, or go to `.v2` |
| 5 | A synchronous call inside a flow declared eventually consistent | It fails whenever the third party does — see invariant 6 |

**Why invariant #6 is heavier in public administration:** an administrative file must
**never be half-processed**. If "petition received" succeeds but "audit entry written"
fails, the system sits in a state the records-retention rules do not permit. **Transaction
boundaries are mandatory documentation, not a nice-to-have.**

## STOP CONDITIONS

1. A new entity where it is **unclear which service owns it**
2. Needing another service's data that the **current contract does not expose**
3. A new business flow where **strong vs eventual consistency is undecided**
4. Needing to change an event that **already has consumers**

→ Enforcement: `hooks/service_boundary_guard.py` (BLOCK)
→ Skills: `skills/go-service-pattern` · `skills/proto-contract` · `skills/transaction-boundary`
