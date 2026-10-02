-- 0030 — the petition timeline learns one more act: `nhap-ho`, a petition booked by an OFFICER on a
-- citizen's behalf (staff intake, POST /api/v1/citizen-reports, internal/app/staff_intake.go). Schema
-- only: no row is written.
--
-- WHY THIS FILE EXISTS. Owner decision 02/10/2026 (ADR 0028, Bổ sung 2026-10-02, row 6): staff intake
-- writes ONE processing-log row ("cán bộ X nhập hộ") besides its audit entry. 0013 closed `hanh_vi` on
-- purpose ("a new act is a migration"), and the row is written in the intake's own transaction (rule 6,
-- invariant 3), so without this widening every staff intake would roll back on PostgreSQL once the
-- use case writes the row (a later card) — internal/app TestLogActionsAreAllowedByTheSchema reads the
-- latest CHECK from migrations.FS to catch exactly that, as soon as that card adds the domain constant.
--
-- THE VALUE. `nhap-ho`: an enum VALUE, so ADR 0011 governs it, not rule 12 — Vietnamese, kebab-case,
-- unaccented, like its siblings (`tao-nhiem-vu`, `danh-gia`) and like the channel the same petition
-- carries (`kenh_tiep_nhan = 'can-bo-nhap-ho'`, ADR 0028 row 5). Act, not channel: the timeline row says
-- what the officer DID, so it drops the `can-bo-` that names who the channel belongs to.
--
-- THE ROW'S OTHER COLUMNS NEED NOTHING NEW. 0013's CASE constraints already bind it: `phan-cong` is the
-- only act carrying an assignee (ELSE arm: both NULL), `ghi-chu` the only act requiring a note (ELSE
-- arm: optional, never blank). `trang_thai_tai_thoi_diem` is `da-tiep-nhan` (ADR 0028 row 7), already
-- in 0013's list. `nguoi_ma` is the officer's business code (rule 6, invariant 8) — never 'cong-dan'.
--
-- WHY A NEW FILE AND NOT AN EDIT OF 0023: core/migrate compares the checksum of every applied file.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. ROWS PER COMMUNE: zero are written. ADD CONSTRAINT validates EVERY row of `nhat_ky_phan_anh` (all
--      32 partitions) under ACCESS EXCLUSIVE for one scan; every existing row holds one of 0023's ten
--      codes and passes. The NOTICE below measures rows per commune (counts and tenant ids only).
--   2. IF IT STOPS HALF-WAY: it cannot land half-applied — one file, one transaction (core/migrate), with
--      its progress row; the DROP and the ADD commit together or not at all, so there is no instant with
--      no CHECK. DROP … IF EXISTS makes a retry cost nothing. No per-commune loop: nothing to backfill.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. READ PATHS THAT CHANGE MEANING WHILE HALF-APPLIED: none can see it half-applied. AFTER it, no
--      reader changes meaning until a row with `nhap-ho` exists, and none can exist before the intake
--      card writes one. THAT card must make the drawer's timeline renderer (web-admin) name the act —
--      an unknown code there is a row the officer cannot read. Citizen surfaces never read this table
--      (0013: STAFF-INTERNAL), so nothing changes for a citizen.
--   5. RETENTION: the timeline is an archival record, append-only (0013's guard, not touched). Widening
--      the list deletes and edits nothing.
-- ---------------------------------------------------------------------------

ALTER TABLE nhat_ky_phan_anh DROP CONSTRAINT IF EXISTS nhat_ky_phan_anh_hanh_vi_hop_le;
ALTER TABLE nhat_ky_phan_anh ADD CONSTRAINT nhat_ky_phan_anh_hanh_vi_hop_le CHECK (hanh_vi IN (
    'phan-loai', 'phan-cong', 'chuyen-trang-thai', 'dong-phieu', 'khong-tiep-nhan',
    'chuyen-cap-tren', 'ghi-chu', 'danh-gia', 'mo-lai-theo-danh-gia', 'tao-nhiem-vu', 'nhap-ho'));

-- ---------------------------------------------------------------------------
-- MEASUREMENT (question 1): timeline rows per commune. Counts and tenant ids only.
-- ---------------------------------------------------------------------------
DO $$
DECLARE r record;
BEGIN
    FOR r IN
        SELECT tenant_id, count(*) AS n
        FROM nhat_ky_phan_anh
        GROUP BY tenant_id
        ORDER BY tenant_id
    LOOP
        RAISE NOTICE '0030: commune % holds % petition timeline row(s)', r.tenant_id, r.n;
    END LOOP;
END $$;

-- ---------------------------------------------------------------------------
-- REVERSAL (rule 7, invariant 4). Run by a person, in ONE transaction; core/migrate has no automatic
-- rollback (ADR 0013). Lossless ONLY WHILE NO ROW HOLDS THE NEW CODE — check
--   SELECT count(*) FROM nhat_ky_phan_anh WHERE hanh_vi = 'nhap-ho';   -- must be 0
-- then re-create 0023's ten-code CHECK (DROP CONSTRAINT, ADD CONSTRAINT with 0023's list) and remove
-- this file's row from `schema_migration`. ONCE ONE ROW HOLDS IT, reversing would require deleting
-- timeline rows, which rule 7 forbids — from then on this migration is not reversed, and a reversal is
-- a stop condition (#2) for the user.
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
