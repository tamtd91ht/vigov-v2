-- 0039 — the petition timeline learns three more acts. Schema only: no row is written, no row is read
-- except to count.
--
--   `gop-phieu`   a petition was merged into a main petition (ADR 0087; routes of commit b97a6379). One
--                 row on EACH petition — the merged one and the main one — in the merge's transaction.
--   `tach-phieu`  a merged petition was taken out of its main petition (ADR 0087 §5). Same, both sides.
--   `go-tep`      a log attachment was soft-removed (internal/app/petition_log_attachment.go Remove, commit
--                 48fa2fd0). Until now written as `ghi-chu` because this list had no code for it.
--
-- Enum VALUES, so ADR 0011 governs them, not rule 12: Vietnamese, kebab-case, unaccented. `gop-phieu` and
-- `tach-phieu` are the SAME strings petition_merge_event.kind already holds (0037), so one act has one
-- name in both records. `go-tep` follows `dong-phieu` / `tao-nhiem-vu`: verb + object.
--
-- 0013's CASE constraints already bind the new rows, nothing else changes: the assignment pair is NULL on
-- every act but `phan-cong` (ELSE arm), and the note is optional-but-never-blank on every act but
-- `ghi-chu` (ELSE arm). A `go-tep` row keeps carrying domain.AttachmentRemovalLogText as its note.
--
-- WHY A NEW FILE AND NOT AN EDIT OF 0030: core/migrate compares the checksum of every applied file.
--
-- ---------------------------------------------------------------------------
-- WHY TWO FILES (0039 + 0040) AND NOT 0030's ONE-STATEMENT SWAP.
--
-- 0018/0023/0030 did DROP + ADD in one transaction: ADD CONSTRAINT scans every row of all 32 partitions
-- while holding ACCESS EXCLUSIVE, so every timeline read and write of every commune waits for the scan.
-- The table is now POPULATED and grows with every act on every petition. Here:
--
--   0039  DROP the old CHECK + ADD the new one NOT VALID, same name, one transaction. Catalogue-only: no
--         scan, ACCESS EXCLUSIVE held for milliseconds. From COMMIT on, every NEW row is checked against
--         the new list — a NOT VALID CHECK is enforced on INSERT; only the scan of OLD rows is deferred.
--         There is never an instant with no CHECK: DROP and ADD commit together or not at all.
--   0040  VALIDATE CONSTRAINT — the scan, under SHARE UPDATE EXCLUSIVE, which does NOT block INSERT or
--         SELECT. Recurses into every partition. Its own transaction (core/migrate: one per file).
--
-- Both in ONE file would buy nothing: the ACCESS EXCLUSIVE taken by ADD is held until COMMIT, so the
-- VALIDATE would run under it anyway.
--
-- THE SAME NAME is kept on purpose: internal/app TestLogActionsAreAllowedByTheSchema and
-- migrations/*_test.go find the latest list by `nhat_ky_phan_anh_hanh_vi_hop_le`, and a renamed
-- constraint would leave them reading 0030's list as current.
--
-- PER PARTITION: nothing. A CHECK added to the partitioned parent is created on every partition (and on
-- every partition added later) as an inherited constraint of the same name; DROP on the parent removes
-- them all, VALIDATE on the parent validates them all. 0040 asserts that afterwards, partition by partition.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. ROWS PER COMMUNE: zero written. The NOTICE below reports, per commune, the timeline rows and how
--      many of them are attachment removals recorded as `ghi-chu` (counts and tenant ids only, rule 3).
--   2. IF IT STOPS HALF-WAY: 0039 cannot land half-applied — one file, one transaction, with its progress
--      row; DROP … IF EXISTS makes a retry cost nothing. If the run stops BETWEEN 0039 and 0040, the
--      constraint stands NOT VALID and is still enforced on every write; the next start runs 0040.
--      VALIDATE on an already-valid constraint is a no-op, so 0040 is re-runnable too. No per-commune
--      loop: nothing is backfilled, so there is no per-commune progress to record.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom. It REFUSES once any row holds a new code.
--   4. READ PATHS THAT CHANGE MEANING WHILE HALF-APPLIED: none — no reader filters on these codes, and
--      between 0039 and 0040 every reader sees the same rows as before. AFTER the Go card that writes
--      them, three things change meaning and the card owns them:
--        * the drawer's timeline (web-admin nhan-phieu.ts NHAN_THAO_TAC_NHAT_KY) must name the three acts,
--          or the officer reads a row with no label;
--        * ATTACHMENT REMOVALS ARE SPLIT ACROSS TWO CODES FOREVER: rows written before the card stay
--          `ghi-chu` (the table is append-only; rewriting them would edit historical records, rule 7
--          forbidden #5). A count of removals by `hanh_vi = 'go-tep'` undercounts history; the
--          authoritative count is audit_log / the file's soft-delete columns. NOT BACKFILLED, on purpose;
--        * merge history now shows on BOTH petitions' timelines, besides petition_merge_event.
--      Citizen surfaces never read this table (0013: STAFF-INTERNAL), so nothing changes for a citizen.
--   5. RETENTION: the timeline is an archival record, append-only (0013's trigger, untouched). Widening
--      the list deletes and edits nothing.
-- ---------------------------------------------------------------------------

ALTER TABLE nhat_ky_phan_anh DROP CONSTRAINT IF EXISTS nhat_ky_phan_anh_hanh_vi_hop_le;
ALTER TABLE nhat_ky_phan_anh ADD CONSTRAINT nhat_ky_phan_anh_hanh_vi_hop_le CHECK (hanh_vi IN (
    'phan-loai', 'phan-cong', 'chuyen-trang-thai', 'dong-phieu', 'khong-tiep-nhan',
    'chuyen-cap-tren', 'ghi-chu', 'danh-gia', 'mo-lai-theo-danh-gia', 'tao-nhiem-vu', 'nhap-ho',
    'gop-phieu', 'tach-phieu', 'go-tep')) NOT VALID;

-- ---------------------------------------------------------------------------
-- MEASUREMENT (questions 1 and 4): timeline rows per commune, and how many are attachment removals that
-- will stay `ghi-chu`. The prefix is domain.AttachmentRemovalLogText's fixed opening; the note itself is
-- never printed (it carries the remover's reason — personal data, rule 3). Counts and tenant ids only.
-- An ESTIMATE: a hand-typed note opening with the same sentence is counted too.
-- ---------------------------------------------------------------------------
DO $$
DECLARE r record;
BEGIN
    FOR r IN
        SELECT tenant_id,
               count(*) AS n,
               count(*) FILTER (WHERE hanh_vi = 'ghi-chu'
                                  AND noi_dung LIKE 'Đã gỡ một tệp đính kèm.%') AS removals_as_note
        FROM nhat_ky_phan_anh
        GROUP BY tenant_id
        ORDER BY tenant_id
    LOOP
        RAISE NOTICE '0039: commune % holds % petition timeline row(s), % attachment removal(s) recorded as ghi-chu',
            r.tenant_id, r.n, r.removals_as_note;
    END LOOP;
END $$;

-- ---------------------------------------------------------------------------
-- REVERSAL (rule 7, invariant 4). Run by a person, in ONE transaction; core/migrate has no automatic
-- rollback (ADR 0013). It reverses 0040 and 0039 together (0040 alone needs no reversal: "validated" is a
-- property of 0039's constraint, not an object of its own).
--
-- LOSSLESS ONLY WHILE NO ROW HOLDS A NEW CODE. The reversal MUST refuse otherwise — removing those rows
-- would delete historical records from an append-only timeline (rule 7, forbidden #1 and #5), and
-- re-creating the narrow list over them fails anyway. Run first, inside the reversal's transaction:
--
--   DO $rev$ BEGIN
--       IF EXISTS (SELECT 1 FROM nhat_ky_phan_anh WHERE hanh_vi IN ('gop-phieu', 'tach-phieu', 'go-tep')) THEN
--           RAISE EXCEPTION 'reversal of 0039 refused: timeline rows hold gop-phieu / tach-phieu / go-tep';
--       END IF;
--   END $rev$;
--
-- then DROP CONSTRAINT nhat_ky_phan_anh_hanh_vi_hop_le, re-add it with 0030's ELEVEN codes exactly as
-- 0030 wrote it (validated — the scan runs under ACCESS EXCLUSIVE), and remove the rows of 0039 AND 0040
-- from `schema_migration`. The Go card that writes the new codes must be rolled back FIRST, or the next
-- merge / unmerge / attachment removal rolls back on the narrowed CHECK.
--
-- ONCE ONE ROW HOLDS A NEW CODE this migration is not reversed; wanting to is rule 7 stop condition #2,
-- a decision for the user.
--
-- (The re-add is written out in prose, not as SQL, on purpose: internal/app latestLogActionCheck reads
-- the LAST declaration of this constraint's list in the raw files, comments included, so a commented-out
-- re-add would be read as the current list. migrations TestRawFilesDeclareTheListOnce guards it.)
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- BACKSTOP: every partitioned table in this schema must actually have partitions (0002, §BACKSTOP).
-- This file declares no table; repeated so every file in this run ends on the same guarantee.
-- ---------------------------------------------------------------------------
DO $$
DECLARE missing_partitions text;
BEGIN
    SELECT string_agg(c.relname, ', ' ORDER BY c.relname) INTO missing_partitions
    FROM pg_class c
    JOIN pg_namespace n ON n.oid = c.relnamespace
    WHERE c.relkind = 'p'
      AND n.nspname = current_schema()
      AND NOT EXISTS (SELECT 1 FROM pg_inherits WHERE inhparent = c.oid);

    IF missing_partitions IS NOT NULL THEN
        RAISE EXCEPTION
            'partitioned table(s) with no partitions in schema %: %',
            current_schema(), missing_partitions
            USING HINT = 'A partitioned table with no partitions rejects every INSERT. Add the '
                         'MODULUS 32 partition loop (ADR 0010) in the same migration that '
                         'declares PARTITION BY.';
    END IF;
END $$;
