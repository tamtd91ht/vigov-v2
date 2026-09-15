# RULE 3 — Citizen personal data

Governed by **Decree 13/2023/ND-CP**. Process logs flow into centralised logging, into
backups, into third-party monitoring. One `log.Info(phone)` written while debugging stays
there and pushes an entire commune's phone numbers somewhere nobody controls.

**Personal data in ViGov:** phone number · full name · address · national ID number · scene
photographs · petition contents · home coordinates · family relationships.

## INVARIANTS

| # | Invariant |
|---|---|
| 1 | Personal data **never** enters logs — not error logs, not debug level |
| 2 | To trace something, log a **business code** (`code`, `arrival_no`) or a **masked** value |
| 3 | Anything leaving the API is masked (`MaskPhone`, `MaskCccd`) unless the caller holds explicit full-view permission |
| 4 | Exports (Excel/PDF) are masked by default; full export is a separate action, separate permission, **audited** |
| 5 | Examples and tests use the agreed fake number (`0900000000`) |
| 6 | Sending to an external service (OCR, messaging, monitoring): only the fields **actually needed**, declared in the adapter |
| 7 | An erasure request under Decree 13 means **anonymise**, never hard delete (rule 7) |

## STRICTLY FORBIDDEN

| # | Forbidden |
|---|---|
| 1 | `log.*(phone)`, `fmt.Printf("%+v", user)`, logging a whole `req.Body` / `dto` / `payload` |
| 2 | Real phone numbers or national IDs hardcoded in source |
| 3 | Personal data inside error messages returned to a client |
| 4 | Personal data in file names, URLs, cache keys, realtime room names |
| 5 | Personal data written into documentation, commit messages, or reports |

## STOP CONDITIONS

1. A request to show **full** personal data to a new role
2. Sending personal data to an **external service** for the first time
3. A citizen requesting **erasure of their personal data**

→ Enforcement: `hooks/pii_guard.py` (BLOCK)
→ Skills: `skills/mask-personal-data` · `skills/audit-trail`
