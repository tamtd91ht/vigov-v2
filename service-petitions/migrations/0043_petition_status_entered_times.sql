-- 0043 — two STATUS-ENTRY TIMES on the petition, for the citizen's timeline (ADR 0047 :269 G4, owner
-- confirmation 11/10/2026). Schema only: no row is written, no existing value changes.
--
--   forwarded_for_processing_at  when the petition last ENTERED `da-chuyen-xu-ly` ("Đã chuyển xử lý")
--   awaiting_confirmation_at     when the petition last ENTERED `cho-dan-xac-nhan` ("Chờ dân xác nhận")
--
-- G4 AS DECIDED: written AT THE TRANSITION by the use case that makes it; a REOPENED petition that passes
-- through the status again takes the NEW time (the column holds the latest entry, not the first); the
-- citizen read does NOT read `nhat_ky_phan_anh` (the internal log stays internal — rule 4, forbidden #5;
-- rule 10, invariant 7). The other times the citizen sees already have columns (0004: `vao_so_luc`,
-- `phan_loai_luc`, `xu_ly_xong_luc`, `dong_luc`).
--
-- NO BACKFILL, DELIBERATELY. Existing petitions that already passed these statuses keep NULL. The only
-- source for the old times would be `nhat_ky_phan_anh`, which G4 excludes, and it cannot say reliably
-- which entry was the LATEST entry into the status across reopenings and corrections — a reconstructed
-- time shown to a citizen as fact is worse than none. NULL reads as "time not recorded"; the citizen
-- timeline shows the step without a time.
--
-- NAMES (rule 12, ADR 0051): English, on a Vietnamese-named table whose existing columns are not renamed
-- (precedent 0017 `rating_comment`, 0032 `zalo_account_id`, 0037 `merged_at`). The status codes are
-- 0004's closed list (ADR 0027), untouched.
--
-- WHY A NEW FILE: core/migrate compares the checksum of every applied file at startup.
--
-- ---------------------------------------------------------------------------
-- PG 16 ON A PARTITIONED TABLE (the real cluster, ADR 0010): ADD COLUMN on the partitioned parent
-- propagates to all 8 partitions (a partition cannot have a column the parent lacks, nor be altered
-- alone); a nullable column with no default is catalogue-only, no rewrite. ADD CONSTRAINT … CHECK on the
-- parent is inherited by every partition and validated there. NOT VALID is avoided — not portable on a
-- partitioned table (comms 0011:332-341); nothing to gain, every value validated is NULL.
--
-- THE LOCK: ACCESS EXCLUSIVE on phieu_phan_anh and its 8 partitions until COMMIT; each CHECK scans
-- the register once and passes vacuously. The register is small — AN ASSUMPTION, NOT A MEASUREMENT; the
-- operator checks first:
--
--   SELECT count(*), pg_size_pretty(pg_total_relation_size('phieu_phan_anh')) FROM phieu_phan_anh;
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. ROWS PER COMMUNE: zero written. Every petition gains two NULL columns without a rewrite.
--   2. IF IT STOPS HALF-WAY: it cannot land half-applied — one file, one transaction (core/migrate), with
--      its progress row. ADD COLUMN IF NOT EXISTS and constraints behind a pg_constraint lookup bound to
--      the table's oid make a retry cost nothing. No per-commune loop: nothing to backfill.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. READ PATHS THAT CHANGE MEANING: none can see it half-applied. AFTER it, before the Go card: every
--      read of phieu_phan_anh names its columns (store `cotPhieu`, no `SELECT *`), so no reader sees
--      them. Every existing write passes: the INSERT does not name them (NULL), no UPDATE names them.
--      ONCE THE GO CARD WRITES THEM: old petitions stay NULL forever (no backfill), so any reader must
--      treat NULL as "not recorded", never as "never entered the status" — the STATUS is the authority
--      on whether it was entered, these columns only on when. A figure computed from them (e.g. time to
--      forward) must say it covers petitions from this migration on.
--   5. RETENTION: a petition is an ARCHIVAL RECORD (rule 7). Nothing is dropped, retyped or renumbered;
--      `ma_tra_cuu` and 0004's guard are untouched. The columns are overwritten on re-entry by design
--      (G4: "mở lại lấy giờ mới nhất"); the history of every entry is `nhat_ky_phan_anh`'s.
--
-- PERSONAL DATA (rule 3): none — two timestamps.
-- ---------------------------------------------------------------------------

ALTER TABLE phieu_phan_anh ADD COLUMN IF NOT EXISTS forwarded_for_processing_at TIMESTAMPTZ;
ALTER TABLE phieu_phan_anh ADD COLUMN IF NOT EXISTS awaiting_confirmation_at    TIMESTAMPTZ;

DO $$ BEGIN
    -- Entering a status happens after the petition entered the register (0037's merged_after_intake
    -- shape). NULL passes.
    IF NOT EXISTS (SELECT 1 FROM pg_constraint
                   WHERE conrelid = 'phieu_phan_anh'::regclass
                     AND conname = 'phieu_phan_anh_forwarded_after_intake') THEN
        ALTER TABLE phieu_phan_anh ADD CONSTRAINT phieu_phan_anh_forwarded_after_intake
            CHECK (forwarded_for_processing_at IS NULL OR forwarded_for_processing_at >= vao_so_luc);
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_constraint
                   WHERE conrelid = 'phieu_phan_anh'::regclass
                     AND conname = 'phieu_phan_anh_awaiting_confirmation_after_intake') THEN
        ALTER TABLE phieu_phan_anh ADD CONSTRAINT phieu_phan_anh_awaiting_confirmation_after_intake
            CHECK (awaiting_confirmation_at IS NULL OR awaiting_confirmation_at >= vao_so_luc);
    END IF;
END $$;

COMMENT ON COLUMN phieu_phan_anh.forwarded_for_processing_at IS
    'Lan GAN NHAT phieu vao da-chuyen-xu-ly (ADR 0047 G4). NULL = khong ghi nhan (phieu truoc migration 0043 '
    'khong duoc dien lai). Trang thai moi la nguon cho viec da vao hay chua.';
COMMENT ON COLUMN phieu_phan_anh.awaiting_confirmation_at IS
    'Lan GAN NHAT phieu vao cho-dan-xac-nhan (ADR 0047 G4). NULL = khong ghi nhan (phieu truoc migration 0043 '
    'khong duoc dien lai). Trang thai moi la nguon cho viec da vao hay chua.';

-- ---------------------------------------------------------------------------
-- REVERSAL (rule 7, invariant 4). Run by a person, in ONE transaction; core/migrate has no automatic
-- rollback (ADR 0013). Prose, not runnable.
--
-- LOSSLESS ONLY WHILE THIS READS ZERO:
--
--   SELECT count(*) FROM phieu_phan_anh
--    WHERE forwarded_for_processing_at IS NOT NULL OR awaiting_confirmation_at IS NOT NULL;
--
-- ZERO → drop constraints phieu_phan_anh_awaiting_confirmation_after_intake and
-- phieu_phan_anh_forwarded_after_intake, drop columns awaiting_confirmation_at and
-- forwarded_for_processing_at, and remove this file's row from `schema_migration`
-- (ten = '0043_petition_status_entered_times.sql').
--
-- NON-ZERO → the times shown to citizens about their own petitions. Dropping them is rule 7 stop
-- condition #2 — the user plus a verified backup — and the way back is a NEW migration.
-- ---------------------------------------------------------------------------

-- BACKSTOP: every partitioned table in this schema must actually have partitions (0002, §BACKSTOP).
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
                         'MODULUS 8 partition loop (ADR 0096) in the same migration that '
                         'declares PARTITION BY.';
    END IF;
END $$;
