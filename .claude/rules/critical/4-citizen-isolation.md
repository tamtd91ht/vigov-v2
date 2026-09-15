# RULE 4 — Isolation between citizens

**Two classes of user, two very different trust levels:**

| Class | Identity | Trust |
|---|---|---|
| **Staff** | Account issued by an administrator, RBAC, every write audited | "Accountable" |
| **Citizen** | Phone number + OTP only | **Weak identity — not to be trusted** |

Phone numbers change hands. SIMs get taken over. OTPs get read over a shoulder. A citizen
identity therefore **never** stands alone as the basis for an act with legal consequences.

## INVARIANTS

| # | Invariant |
|---|---|
| 1 | A citizen sees **only their own data**. No "for convenience" exceptions |
| 2 | Citizen identity comes **from the session**, never from a request parameter |
| 3 | Every citizen-path query filters by session identity **and** `tenant_id` (rule 1) |
| 4 | Lookup codes (file code, petition code) are **not guessable** — not sequential, not short |
| 5 | Citizen routes and staff routes are **separated at routing level**, never sharing a handler |
| 6 | Citizens never receive lists — only **their own records** |
| 7 | Citizen attachments are served through **signed, expiring links** bound to identity and commune |

## STRICTLY FORBIDDEN

| # | Forbidden | Why |
|---|---|---|
| 1 | `GET /petitions?phone=...` — identity from the query | Changing the number reads somebody else's data |
| 2 | Returning different `404` vs `403` for another person's record | Leaks the existence of the record |
| 3 | Sequential file codes on a public lookup route | Enumeration reads everything |
| 4 | OTP as the sole basis for an act with legal consequences | Weak identity |
| 5 | Exposing internal fields (staff notes, routing history) to citizens | Outside the citizen's scope |

## STOP CONDITIONS

1. A citizen needing to see data that is **not their own** (family member, legal representative)
2. A request for **public lookup without authentication**
3. A citizen acting with **multiple communes** — see rule 1, stop condition #4

→ Enforcement: `hooks/citizen_scope_guard.py` (BLOCK)
→ Skills: `skills/citizen-identity-multi-tenant` · `skills/zalo-miniapp-multi-tenant`
