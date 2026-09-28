---
name: security-logging
description: Use when emitting, shipping or retaining security logs — login events, access logs, lockouts, token/commune mismatches, admin actions, configuration changes — and when deciding whether something belongs in the audit trail or in the security log. Triggers on: security log, nhật ký an ninh, access log, nhật ký truy cập, SIEM, log shipping, retention, lưu nhật ký, NTP, đồng bộ thời gian, login failed, đăng nhập sai, alert, cảnh báo, slog, middleware log, TCVN 5.8.
---

# Skill: Security logging (TCVN 14423 §5.8)

## Two logs, two purposes — never merge them

| | **Audit trail** (rule 6) | **Security log** (this skill) |
|---|---|---|
| Answers | who did WHAT to WHICH record — for a complaint or an inspection | is someone ATTACKING the system — for detection and incident response |
| Is | business data, in the owning service's DB, same transaction | an operational stream: stdout JSON → central store |
| Kept | ≥ 12 months, never deleted | ≥ 3 months (level 3) / ≥ 6 months (level 4) — open question #35 |
| Written by | `core/audit.Write` | `slog` with a fixed event shape |

A login is both: success goes to the audit trail (already, `service-identity`), and every
attempt — success or failure — goes to the security log.

## Events that must be emitted

| Event | Why (TCVN) |
|---|---|
| Login success / failure, logout | 5.8.2.1 access log · 5.6.1c account monitoring |
| Account locked / unlocked (manual or automatic) | 5.6 |
| Password changed / reset · MFA enrolled / reset | 5.6 |
| Token commune ≠ Host commune (rule 1 inv. 8) | an attack signal — raise an alert, not just 401 |
| Permission denied (403) on a staff route | probing |
| Rate limit hit | 5.12.2.4e |
| Role or permission granted / revoked · configuration changed | 6.8.2.1 application log (level 4) |
| Unmasked personal data read | 6.4.2.9 data-access log (level 4) — the audit trail already records it |

## Fields — and what must never be one

`event` · `outcome` · time in **UTC** · source IP · tenant · actor as the **business code**
(`CB-00123`), or an email **fingerprint** when there is no account yet · route · request id.

**Never** a password, token, OTP, raw email or phone, or request body — rule 3 applies to this
log exactly as to every other. `pii_guard` blocks the common shapes; it cannot see a value
laundered through a variable, so read what you log.

## Time, shipping, protection

- Clocks come from NTP on the nodes (infra). Code never invents a timezone: UTC in the log.
- Services write JSON to stdout; a node agent ships it. No service opens a connection to a
  log store — that is a third write path nobody audits.
- The central store is append-only for everyone but its operator, and access to it is itself
  logged (level 4, §6.8.2.3). Sending it to an outside SaaS is a rule 13 STOP condition.

→ Rule 3 · Rule 6 · Rule 13 · `skills/audit-trail` · `skills/security-baseline`
