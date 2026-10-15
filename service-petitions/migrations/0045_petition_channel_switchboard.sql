-- 0045 — the petition register learns a fifth intake channel: `tong-dai`, "Tổng đài điện thoại" (owner
-- decision 15/10/2026, tester bug sheet web-admin-141026, "Nhập hộ phản ánh"). Schema only: no row is
-- written, no existing value changes.
--
-- WHY THIS FILE EXISTS. The staff intake (internal/app/staff_intake.go) now records the channel the
-- officer picks on the form: `can-bo-nhap-ho` (default) · `tong-dai` · `web-xa` · `zalo-oa`. Three of
-- them are already in 0004's `phieu_phan_anh_kenh_hop_le`; `tong-dai` is not, so without this widening
-- every intake that picks it would roll back on PostgreSQL inside its own transaction.
--
-- THE TWO STAFF-SHAPE CHECKS FOLLOW IT. `tong-dai` is booked ONLY by staff (no citizen form exists for
-- it), so it carries the staff-booked deadline shape exactly as `can-bo-nhap-ho` does: the field at
-- booking (`phieu_phan_anh_nhap_ho_co_linh_vuc`) and `han_tiep_nhan` NULL = "KHÔNG ÁP DỤNG"
-- (`phieu_phan_anh_nhap_ho_khong_co_han_tiep_nhan`, ADR 0028 F5). Both are re-declared under THEIR OWN
-- NAMES with `NOT IN ('can-bo-nhap-ho', 'tong-dai')`, so the floor 0004 put under the staff channel is
-- under the new one too. Both are vacuous for every existing row (no row holds `tong-dai`).
--
-- WHAT THIS FILE DELIBERATELY DOES NOT DO — `web-xa` / `zalo-oa`. An officer may now record those two on
-- the staff form, and such a row has the staff shape too (owner: the act decides the deadline, not the
-- label). They are NOT added to the two staff-shape CHECKs: both names also describe a citizen-facing
-- form (ADR 0028 E puts them in the deferred-deadline branch), and a CHECK binding them to the staff
-- shape would refuse that form the day it is built — and refuse today's test rows that use `web-xa` as
-- a citizen channel. For those two the staff shape is held by the use case alone (one code path,
-- app.StaffIntake.Book, tested per channel). Stated, not hidden.
--
-- THE VALUE. An enum VALUE, so ADR 0011 governs it, not rule 12 — Vietnamese, kebab-case, unaccented,
-- like its siblings. The Go constant naming it is English (domain.ChannelSwitchboard).
--
-- WHY A NEW FILE AND NOT AN EDIT OF 0004: core/migrate compares the checksum of every applied file.
--
-- ---------------------------------------------------------------------------
-- PG 16 ON A PARTITIONED TABLE (ADR 0010): a CHECK added on the parent is created on all 8 partitions
-- and validated there. NOT VALID is avoided — not portable on a partitioned table (0044's note). DROP and
-- ADD of each constraint commit together (one file, one transaction), so there is never an instant with
-- no CHECK.
--
-- THE LOCK: ACCESS EXCLUSIVE on phieu_phan_anh and its partitions until COMMIT; each ADD scans the
-- register once and every row passes (widening, or vacuous). The register is small — AN ASSUMPTION, NOT
-- A MEASUREMENT; the operator checks first:
--
--   SELECT count(*), pg_size_pretty(pg_total_relation_size('phieu_phan_anh')) FROM phieu_phan_anh;
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. ROWS PER COMMUNE: zero written. The NOTICE below counts petitions per commune and channel (counts,
--      tenant ids and channel codes only).
--   2. IF IT STOPS HALF-WAY: it cannot land half-applied — one file, one transaction (core/migrate), with
--      its progress row. DROP … IF EXISTS makes a retry cost nothing. No per-commune loop: nothing to
--      backfill.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. READ PATHS THAT CHANGE MEANING: none can see it half-applied. AFTER it, a reader meets `tong-dai`
--      only once the staff intake writes one. Readers that LABEL a channel must know it: web-admin's
--      `nhanKenh` / `MOI_KENH` (src/features/phan-anh/nhan-phieu.ts) — "Tổng đài điện thoại". The Go
--      register filter accepts it through domain.KenhTiepNhan.HopLe. Citizen surfaces never see a
--      staff-booked petition (no owner). Reports that EXCLUDE `han_tiep_nhan IS NULL` rows already
--      exclude it, as intended (ADR 0028 F5).
--   5. RETENTION: a petition is an ARCHIVAL RECORD (rule 7). Nothing is dropped, retyped or renumbered;
--      `kenh_tiep_nhan` stays immutable (0004's ho_so_luu_tru_bat_bien, untouched).
--
-- PERSONAL DATA (rule 3): none — a channel code.
-- ---------------------------------------------------------------------------

ALTER TABLE phieu_phan_anh DROP CONSTRAINT IF EXISTS phieu_phan_anh_kenh_hop_le;
ALTER TABLE phieu_phan_anh ADD CONSTRAINT phieu_phan_anh_kenh_hop_le CHECK (kenh_tiep_nhan IN (
    'zalo-mini-app', 'zalo-oa', 'web-xa', 'can-bo-nhap-ho', 'tong-dai'));

ALTER TABLE phieu_phan_anh DROP CONSTRAINT IF EXISTS phieu_phan_anh_nhap_ho_co_linh_vuc;
ALTER TABLE phieu_phan_anh ADD CONSTRAINT phieu_phan_anh_nhap_ho_co_linh_vuc
    CHECK (kenh_tiep_nhan NOT IN ('can-bo-nhap-ho', 'tong-dai') OR linh_vuc IS NOT NULL);

ALTER TABLE phieu_phan_anh DROP CONSTRAINT IF EXISTS phieu_phan_anh_nhap_ho_khong_co_han_tiep_nhan;
ALTER TABLE phieu_phan_anh ADD CONSTRAINT phieu_phan_anh_nhap_ho_khong_co_han_tiep_nhan
    CHECK (kenh_tiep_nhan NOT IN ('can-bo-nhap-ho', 'tong-dai') OR han_tiep_nhan IS NULL);

-- ---------------------------------------------------------------------------
-- MEASUREMENT (question 1): petitions per commune and channel. Counts, tenant ids, channel codes only.
-- ---------------------------------------------------------------------------
DO $$
DECLARE r record;
BEGIN
    FOR r IN
        SELECT tenant_id, kenh_tiep_nhan, count(*) AS n
        FROM phieu_phan_anh
        GROUP BY tenant_id, kenh_tiep_nhan
        ORDER BY tenant_id, kenh_tiep_nhan
    LOOP
        RAISE NOTICE '0045: commune % holds % petition(s) on channel %', r.tenant_id, r.n, r.kenh_tiep_nhan;
    END LOOP;
END $$;

-- ---------------------------------------------------------------------------
-- REVERSAL (rule 7, invariant 4). Run by a person, in ONE transaction; core/migrate has no automatic
-- rollback (ADR 0013). Prose, not runnable.
--
-- LOSSLESS ONLY WHILE THIS READS ZERO:
--
--   SELECT count(*) FROM phieu_phan_anh WHERE kenh_tiep_nhan = 'tong-dai';
--
-- ZERO → re-declare the three constraints exactly as 0004 wrote them (DROP CONSTRAINT, ADD CONSTRAINT:
-- the channel list without 'tong-dai'; the two staff-shape CHECKs as `kenh_tiep_nhan <> 'can-bo-nhap-ho'
-- OR …`), and remove this file's row from `schema_migration` (ten = '0045_petition_channel_switchboard.sql').
--
-- NON-ZERO → those petitions are archival records whose channel is immutable; narrowing the list would
-- fail on them, and making it pass would mean rewriting `kenh_tiep_nhan` on archival rows (rule 7
-- forbidden #5). From then on the way back is a NEW migration and a decision of the owner, never a
-- reversal of this one.
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
