# RULE 5 — Authorisation

The global guard rejects users who are **not logged in**. It does **not** check permissions.
An endpoint with no declaration is callable by **every staff role** — including roles with
nothing to do with that subsystem. This fails silently: nothing reports it, no test turns red.

Permissions always sit **within one commune** (rule 1). No role crosses communes except an
explicitly declared cross-read path.

## INVARIANTS

| # | Invariant |
|---|---|
| 1 | Every endpoint declares **explicitly** one of: `RequirePermission(...)`, `Public("<reason>")`, `AnyAuthenticated("<reason>")`, `CitizenOnly()` |
| 2 | No implicit default. A missing declaration means **deny**, not allow |
| 3 | Permission is checked against `(tenant_id, role, permission)` — a missing `tenant_id` is cross-commune escalation |
| 3b | A permission is **one flat key**, `"<nhóm>.<việc>"` (`task.extend`) — the same string the Phân quyền screen shows and `quyen` stores. Never `(subsystem, action)`: these rights are not a Cartesian product, and `task.approve` is deliberately not `task.extend` |
| 3c | The key must **exist in the `quyen` table**. A key no migration seeds is a right no administrator can grant, so that route answers 403 to **every** account, forever — and the tests stay green, because a fake checker grants any string. Needing a key the table lacks is a **finding for open question #27**, never a new `INSERT` |
| 4 | Tokens are revocable: they carry a `sid` checked against a session registry on every request |
| 5 | Privilege changes, role changes and account locks are all **audited** (rule 6) |
| 6 | Citizen routes do not use RBAC — they are isolated per rule 4 |
| 7 | Every new endpoint has tests: **401** no token · **403** wrong permission · **403** right permission wrong commune · **200** both correct |

## STRICTLY FORBIDDEN

| # | Forbidden | Why |
|---|---|---|
| 1 | Checking permissions in the UI instead of the service layer | Clients can be modified |
| 2 | A "superadmin" role that skips the commune check | One role collapses the whole isolation |
| 3 | Comparing role strings ad hoc across handlers | Not auditable, not testable |
| 4 | `Public()` with no **specific reason** | Six months later nobody dares remove it |

## STOP CONDITIONS

1. An endpoint that must be open to **every authenticated account**
2. An endpoint that must be **public without authentication**
3. A new role, or a role needing authority **beyond a single commune**

→ Enforcement: `hooks/rbac_guard.py` (advisory — runs PostToolUse, so it reports on the
  written file rather than preventing the write; a half-written route must not be blocked)
→ Enforcement (3c): `hooks/quyen_key_guard.py` (BLOCK, at the write) **and**
  `tools/check_quyen.py` (whole repo, in `make check`). Two shapes because a hook sees one
  file and never re-reads what is already on disk — which is exactly where three invented
  keys lived until 2026-09-21
→ Skills: `skills/session-and-token` · `skills/cross-tenant-reporting`
