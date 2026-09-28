---
name: session-and-token
description: Use when working on login, JWT, refresh tokens, session revocation, account locking, or password handling. Triggers on: JWT, token, refresh, login, logout, session, revoke, sid, password, bcrypt, argon2, account lock, auth.
---

# Skill: Sessions and tokens under multi-tenancy

## REQUIRED

| # | Practice |
|---|---|
| 1 | Tokens carry `tenant_id` and `sid` (session id) |
| 2 | **Every request** compares the token's `tenant_id` with the commune from `Host`. Mismatch = 401 + **alert** |
| 3 | `sid` is checked against a session registry on every request — tokens must be **revocable** |
| 4 | Short-lived access tokens; **rotating** refresh tokens, where reuse of an old one revokes the chain |
| 5 | Cookies scoped to each commune's own host. **Never** the parent domain |
| 6 | Passwords hashed with **argon2id** (or bcrypt cost ≥ 12) |
| 7 | Locking an account, changing a role or changing a password **revokes every open session** |

## Why item 2 is an "alert", not an "error"

A normal user **never** sends commune A's token to commune B's domain — browsers do not send
cookies across hosts. Seeing it means somebody is deliberately probing, or a token has been
stolen. That is a security signal: log it and raise an alert, do not just return 401.

## TCVN 14423 — what the standard adds (thresholds are open questions #36–#39)

| Control | How, once the number is decided | State today |
|---|---|---|
| **Login lockout** (#39) | Count failures per account AND per source address in Redis (`t:<tenant>:rl:…`); lock with an expiry; an admin unlock is audited. Keep it apart from the manual `lockout` resource — that one is a business state (retired, transferred) | no counter — failure only logs |
| **Idle lock** (#38) | Compare `phien.dung_gan_nhat` with now in the session check; admin sessions may get a shorter limit. The absolute 12 h cap (open question #18) stays | column written, never read |
| **MFA** (#37) | TOTP (RFC 6238); the seed encrypted per ADR 0009, never logged; recovery codes stored hashed, single use; enrolment and reset audited and revoke sessions. Operator zone: decided, ADR 0048 | none |
| **Password age · history** (#36) | `mat_khau_doi_luc` column; a history table of argon2id hashes, compared on change | none |
| **45-day inactive accounts** | Background job, system principal, audited per account (rule 6 inv. 6); uses `dang_nhap_gan_nhat` | none |
| **Account inventory** | Type (admin · staff · technical · citizen), activation date, disable date | type and dates missing |

Every event above also goes to the security log — `skills/security-logging`.

## FORBIDDEN

Non-expiring tokens · tokens that cannot be revoked · passwords hashed with a fast hash
(MD5/SHA) · embedding roles in the token without re-checking (permission changes then take
effect only when the token expires).

→ Rule 1 · Rule 5 · Rule 13 · `skills/security-baseline`
