---
name: mask-personal-data
description: Use when returning personal data from an API, displaying phone numbers or national IDs, logging, exporting reports, or anonymising on an erasure request. Triggers on: MaskPhone, MaskCccd, mask, anonymise, anonymize, PII, personal data, redact, national ID, right to erasure, Decree 13.
---

# Skill: Masking and anonymising personal data

## One implementation only

The masking helpers live in `pkg/privacy`. **No service reimplements them.** Three copies of
one masking function are three behaviours that will drift — and nobody will know which way.

```go
package privacy

func MaskPhone(s string) string  // 0912345678 -> 09****5678
func MaskCccd(s string) string   // 079123456789 -> 079*****789
func MaskName(s string) string   // Nguyễn Văn An -> Nguyễn V. A.
```

## REQUIRED

| # | Practice |
|---|---|
| 1 | Masked by default. Returning full values is a separate permission and is **audited** (rule 6) |
| 2 | Exports are masked by default; full export is a separate feature, separate permission, audited |
| 3 | Audit entries are masked too — the trail must not become a personal-data store |
| 4 | Examples and tests use the fake number `0900000000` |

## Anonymising on an erasure request (Decree 13/2023)

**Anonymise, never hard delete.** The business record is an archival record (rule 7); only
identifying fields are replaced:

| Field | After anonymisation |
|---|---|
| Full name | `"[anonymised]"` |
| Phone number | `""` |
| National ID | `""` |
| Address | keep the **commune level**, drop the street number |
| Petition contents | **unchanged** — it is the record's content, not an identifier |
| File code, dates, status | **unchanged** |

Audit the anonymisation itself: who requested it, who performed it, when, on what basis.

→ Rule 3 · Rule 7
