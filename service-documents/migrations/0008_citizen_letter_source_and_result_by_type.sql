-- 0008 — citizen letters (0006): the ENTRY SOURCE of a letter, and the closing-result rule narrowed by
-- letter type. Owner decisions 08/10/2026, ADR 0084 #2 and #7.
--
-- WHY A NEW FILE AND NOT AN EDIT OF 0006: core/migrate compares the checksum of every applied file at
-- startup. Editing an applied file stops the service or leaves two databases with one version number.
--
-- NAMES (rule 12, ADR 0051): the new column and constraints are English, in 0006's shape
-- (`citizen_letter_<what>_valid`). Enum VALUES stay Vietnamese, unaccented, kebab-case (ADR 0011).
--
-- ---------------------------------------------------------------------------
-- 1. `source` — HOW THE LETTER ENTERED THE REGISTER (ADR 0084 #7, the "nguồn vào sổ" chip).
--
--   nhap-tay      typed in by a clerk                       ("Nhập tay")
--   nhap-excel    booked by the Excel import                ("Nhập từ Excel")
--   mini-app      arrived through the Zalo Mini App         ("Mini App")
--   thu-dien-tu   arrived by e-mail                         ("Thư điện tử")
--
-- NOT NULL DEFAULT 'nhap-tay': the owner decided that EVERY EXISTING letter reads as manual entry —
-- until today the register had no other way in, so that is a fact, not a guess. On PostgreSQL 11+
-- ADD COLUMN with a constant default is metadata-only: no row is rewritten, existing rows read the
-- default. The DEFAULT stays on the column afterwards so the current insert path, which does not
-- name the column yet, keeps booking manual entries until TASK-06 makes it explicit.
--
-- The source is a FACT OF THE BOOKING ACT. The Go domain must not let a later edit change it; the
-- schema does not freeze it here (0006's `citizen_letter_number_immutable` is unchanged — replacing
-- that function is a heavier change than this card, and the risk is a wrong chip, not a lost record).
--
-- ---------------------------------------------------------------------------
-- 2. THE CLOSING RESULT, NARROWED TO COMPLAINTS AND DENUNCIATIONS (ADR 0084 #2: C10 → KN/TC).
--
-- 0006 `citizen_letter_resolved_has_result` required, for EVERY type, the issued result document
-- (number, date, signer, issuing body) AND the summary before `da-giai-quyet`. New rule:
--
--   khieu-nai, to-cao              unchanged — all five fields required. These are the two statutory
--                                  procedures (Luật Khiếu nại 2011, Luật Tố cáo 2018) whose outcome IS
--                                  an issued decision/conclusion; a resolved complaint without one is a
--                                  record an inspection cannot verify.
--   kien-nghi-phan-anh, de-nghi    nothing required. The reply is one optional free-text summary
--                                  (`result_summary`); the document fields may still be filled.
--
-- Untouched, and still applying to every type: `citizen_letter_result_document_pair` (number and date
-- travel together), the per-field shape CHECKs, and the timestamp bindings.
--
-- LOOSENING ONLY: every row that satisfied the old CHECK satisfies the new one (the new expression is
-- the old one OR'ed with a type test). So the ADD is VALIDATED in place, NOT `NOT VALID` + VALIDATE:
-- the scan cannot fail, and NOT VALID on a CHECK of a partitioned table is not portable across the
-- PostgreSQL versions this repository supports (service-comms 0011:332-341, 0023:38-39). DROP and ADD
-- share this file's one transaction, so there is no instant without the rule.
--
-- THE GO DOMAIN IS STRICTER THAN THIS SCHEMA UNTIL TASK-06 (domain.CitizenLetter.HasResult and
-- ErrLetterNeedsResult still demand the result for every type). That order is the safe one: the
-- database never refuses what the application now allows.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. ROWS PER COMMUNE: none rewritten. The NOTICE block below reports letters per commune and type
--      (counts and tenant ids only — rule 3). The ADD CONSTRAINT scans `citizen_letter` once under
--      ACCESS EXCLUSIVE; the register is days old, so the scan is short.
--   2. IF IT STOPS HALF-WAY: it cannot land half-applied — one file, one transaction with its progress
--      row (core/migrate). ADD COLUMN IF NOT EXISTS and DROP CONSTRAINT IF EXISTS make a retry free.
--      Nothing is backfilled, so there is no per-commune loop to resume.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. READ PATHS THAT CHANGE MEANING: (a) `source` — readers name their columns (store/citizen_letter.go
--      never selects `*`), so nothing existing sees it until TASK-06 reads it. (b) the result rule —
--      after this file, a `kien-nghi-phan-anh` / `de-nghi` letter MAY sit in `da-giai-quyet` with no
--      result. Any reader that assumes "resolved ⇒ result document present" (a report column, an export
--      of the result number) must tolerate NULLs for those two types. Today none exists: the domain
--      still refuses such a close, so no such row can be written before TASK-06.
--   5. RETENTION: nothing removed or rewritten. A letter is an archival record; this file adds a column
--      and relaxes one rule for two types.
-- ---------------------------------------------------------------------------

ALTER TABLE citizen_letter
    ADD COLUMN IF NOT EXISTS source TEXT NOT NULL DEFAULT 'nhap-tay';

ALTER TABLE citizen_letter DROP CONSTRAINT IF EXISTS citizen_letter_source_valid;
ALTER TABLE citizen_letter ADD CONSTRAINT citizen_letter_source_valid
    CHECK (source IN ('nhap-tay', 'nhap-excel', 'mini-app', 'thu-dien-tu'));

-- Every arm is TRUE or FALSE, never NULL (`letter_type` and `status` are NOT NULL): a CHECK treats
-- NULL as passed.
ALTER TABLE citizen_letter DROP CONSTRAINT IF EXISTS citizen_letter_resolved_has_result;
ALTER TABLE citizen_letter ADD CONSTRAINT citizen_letter_resolved_has_result CHECK (
    status <> 'da-giai-quyet'
    OR letter_type IN ('kien-nghi-phan-anh', 'de-nghi')
    OR (result_document_no IS NOT NULL AND result_document_date IS NOT NULL
        AND result_signer IS NOT NULL AND result_issuer IS NOT NULL
        AND result_summary IS NOT NULL));

-- MEASUREMENT (question 1): letters per commune and type. Counts and tenant ids only.
DO $$
DECLARE r record;
BEGIN
    FOR r IN
        SELECT tenant_id, letter_type, count(*) AS n
        FROM citizen_letter
        GROUP BY tenant_id, letter_type
        ORDER BY tenant_id, letter_type
    LOOP
        RAISE NOTICE '0008: commune % holds % letter(s) of type %', r.tenant_id, r.n, r.letter_type;
    END LOOP;
END $$;

-- ---------------------------------------------------------------------------
-- REVERSAL (rule 7, invariant 4). By a person, in ONE transaction, then remove this file's row from
-- `schema_migration` (core/migrate has no automatic rollback, ADR 0013). Written as prose, not a
-- runnable line, because a runnable line is a line that gets run.
--
--   THE RESULT RULE. Restore 0006's expression: DROP CONSTRAINT citizen_letter_resolved_has_result and
--   ADD it back without the `letter_type IN (...)` arm (0006_citizen_letter.sql:324-328). That ADD
--   FAILS while any `kien-nghi-phan-anh` / `de-nghi` letter is `da-giai-quyet` without a full result:
--     SELECT count(*) FROM citizen_letter
--     WHERE status = 'da-giai-quyet' AND letter_type IN ('kien-nghi-phan-anh', 'de-nghi')
--       AND (result_document_no IS NULL OR result_document_date IS NULL OR result_signer IS NULL
--            OR result_issuer IS NULL OR result_summary IS NULL);          -- must be 0
--   If it is not 0, reverting means inventing result documents for closed archival records — never
--   done by a command; the owner decides (rule 7, stop condition #2).
--
--   THE SOURCE. DROP CONSTRAINT citizen_letter_source_valid, then DROP COLUMN source. Lossless only
--   while every row still holds the default:
--     SELECT count(*) FROM citizen_letter WHERE source <> 'nhap-tay';     -- must be 0
--   Otherwise it destroys how real letters entered the register — rule 7 stop condition #2, the
--   owner's decision with a verified backup.
-- ---------------------------------------------------------------------------
