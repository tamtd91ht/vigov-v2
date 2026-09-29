# RULE 12 — Every name is English; only API URL paths stay Vietnamese

| # | Invariant |
|---|---|
| 1 | Code names, files, directories, tables, columns, **enum values**, buckets, **k8s names and labels** (`deploy/**`) are English. Existing Vietnamese names are renamed service by service (campaign ADR) |
| 2 | Vietnamese only in: API URL paths, web-admin `src/app/**` route dirs, UI strings, `kb/` prose |
| 3 | One VN→EN dictionary: `kb/00-foundation/ubiquitous-language.md`. Infra names = real cluster (`common-config`, `<service>-secrets`) |

| # | Forbidden |
|---|---|
| 1 | A new Vietnamese name without `vi-name-ok: <reason>` |
| 2 | A second English word for a concept the dictionary names |

**STOP:** a concept with no English name in the dictionary → ask the user.

→ Enforcement: `hooks/english_identifier_guard.py` (BLOCK) · Skill: `skills/naming-english`
