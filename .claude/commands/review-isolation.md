---
description: Review all three isolation dimensions across the source — commune, citizen, role
argument-hint: "[scope: diff | service | all] — empty means diff"
allowed-tools: Read, Grep, Glob, Bash
---

# /review-isolation

The three isolation dimensions must **all** hold. Getting two of three right is still a leak.

| Dimension | Rule | What to review |
|---|---|---|
| **Commune** | 1 | Queries missing `tenant_id` · single-column unique keys · defaults on the isolation path · tenant taken from the client · realtime rooms / cache keys / file paths / queues without a prefix |
| **Citizen** | 4 | Identity taken from the request instead of the session · guessable lookup codes · citizen and staff sharing a handler |
| **Role** | 5 | Routes with no permission · `Public()` with no reason · roles that skip the commune check |

## Steps

1. Run the hooks in full-scan mode (`tenant_scope_guard`, `citizen_scope_guard`, `rbac_guard`)
2. Review what the hooks **cannot** catch:
   - session cookies scoped to the parent domain
   - every `// @cross-tenant:` escape — is each one still correct, is the reason still valid
   - events and queue messages missing `tenant_id`
   - aggregate reports leaking personal data
3. Cross-check `kb/30-indexes/data-ownership.json`: entities with no declared owner

## Report

| Dimension | Location | Severity | Fix |
|---|---|---|---|

Severity: **BREACH** (real leak) · **RISK** (a path exists, not yet exercised) · **DEBT**
(correct but fragile).

## Isolation test — always run before release

| Case | Expected |
|---|---|
| Token for commune A sent to commune B's domain | 401 + alert entry |
| `tenant_id` edited in the request body | Ignored entirely |
| Unknown domain | 404, revealing nothing about which communes exist |
| A and B both create file `01/2026` | Both succeed |
| Realtime event in commune A | No client of commune B receives it |
| Signed file link from A used in a B session | Rejected |
| District-level report | Aggregates only, audited |
