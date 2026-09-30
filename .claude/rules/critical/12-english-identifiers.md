# RULE 12 — New code is named in English

| # | Invariant |
|---|---|
| 1 | Every **new** function, method, type, variable, constant, struct field, export, file, directory, table, column and bucket is English |
| 2 | Vietnamese stays where a user reads it: route segments (`web-admin/src/app/**`), UI strings, `kb/` prose. Enum **values**: ADR 0011 |
| 3 | Existing names are **not renamed**; editing them is allowed |

| # | Forbidden |
|---|---|
| 1 | A new Vietnamese name without `vi-name-ok: <reason>` |
| 2 | A second English word for a concept `kb/00-foundation/ubiquitous-language.md` already names |

**STOP:** a business concept with no English name there → ask the user.

→ Enforcement: `hooks/english_identifier_guard.py` (BLOCK) · Skill: `skills/naming-english` · ADR 0051
