# RULE 7 — Data preservation

Administrative files (documents, petitions, feedback, disbursements, one-stop-shop files)
are **archival records** with statutory retention periods. Deleting a row here is not
"cleaning up data" — it is **destroying a record**, something that may only happen through
an administrative procedure, never through a command.

## INVARIANTS

| # | Invariant |
|---|---|
| 1 | Business data is **soft deleted**: `deleted_at`, `deleted_by`, `delete_reason` |
| 2 | Every read path excludes deleted rows — **everywhere**: lists, statistics, search, reports, background jobs |
| 3 | A code that has been issued is **never reissued**, even after a soft delete |
| 4 | Migrations are **reversible**, or run after a verified backup |
| 5 | Migrations run **per commune**, resumable, recording progress |
| 6 | A merged commune is **marked inactive**; its data and codes are kept unchanged |
| 7 | An erasure request under Decree 13/2023 means **anonymise**: replace identifying fields, keep the business record |

## STRICTLY FORBIDDEN

| # | Forbidden |
|---|---|
| 1 | `DELETE FROM`, bulk delete, or `Drop` on business data |
| 2 | `UPDATE` with no filter, or an empty filter |
| 3 | A migration dropping a populated column without a prior backup |
| 4 | Renumbering file codes that have already been issued |
| 5 | Editing historical records (audit entries, routing history, issued documents) |

## STOP CONDITIONS

1. A request to **truly delete** a business record
2. A migration that **loses** a column or changes a type irreversibly
3. A citizen requesting erasure of personal data
4. Handling data of a merged or dissolved commune

→ Enforcement: `hooks/data_safety_guard.py` (BLOCK)
→ Skills: `skills/admin-unit-merge` · `skills/mask-personal-data`
