---
description: Security review against TCVN 14423:2026 — transport, crypto, auth hardening, headers, logs, dependencies
group: Rà soát
argument-hint: "[scope: diff | service | all] — empty means diff"
allowed-tools: Read, Grep, Glob, Bash
---

# /review-security

Reviews rule 13 and the part of TCVN 14423:2026 that lives in code. Run it on a diff that
touches login, sessions, passwords, cryptography, uploads or an unauthenticated route; on
`all` before every release; and **every 6 months** regardless — the standard asks for
periodic review (§5.6.2.1d, §5.7.2.1b, §5.8.2.1), and a review nobody schedules is one
nobody runs. For depth, dispatch the `security-reviewer` agent with the same scope.

## Steps

1. `make security` — rule 13 patterns over the whole repo, plus the debt ledger.
2. `make vuln` if the network is available — say so plainly if it is not; never report it green.
3. Review what no pattern sees:

| # | Check | TCVN |
|---|---|---|
| 1 | Every `// @security-exception:` — is the reason still true | 5.17.2.3 |
| 2 | Every entry in `tools/security_debt.json` — expiry realistic, owner known | 5.7.2.1 |
| 3 | Routes reachable without a staff session — rate limit, identical errors, no enumeration | 5.12.2.4e |
| 4 | Login: failure counting, lockout, identical message for wrong email and wrong password | 5.5.2.2 |
| 5 | Session: absolute cap, idle lock, revocation on password/role change | 5.5.2.2 · 5.12.2.4c |
| 6 | Password rules, MFA, 45-day inactive accounts, default accounts | 5.6.2.2–5.6.2.4 |
| 7 | Input validated before use; output encoded; errors carry no internals and no personal data | 5.17.2.3 |
| 8 | Security events emitted with the fields `skills/security-logging` lists, and no personal data | 5.8.2.1 |
| 9 | Uploads: size and type policy, magic bytes, malware scan, SHA-256 stored (ADR 0052) | 5.10 · 5.4.2.5 |
| 10 | A threshold written into code while its open question (#35–#39) is still OPEN | rule 13 STOP |

## Report

| Group | Location | Severity | TCVN | Fix |
|---|---|---|---|---|

Severity: **EXPOSED** (exploitable today) · **GAP** (a requirement unmet, not yet exploitable) ·
**DEBT** (met, but fragile or undocumented).

End with what must be fixed **before release**, and which open question blocks what.

→ Rule 13 · `skills/security-baseline` · `skills/security-logging` · `skills/session-and-token`
