-- 0013 — the petition processing logbook `nhat_ky_phan_anh` (docs/ui-ux/09 §8.7, the right-hand
-- timeline column of the petition drawer).
--
-- 0004 (header, "WHAT THIS FILE DELIBERATELY DOES NOT CREATE") and 0005:34-36 both recorded that
-- this table was deliberately NOT created and left for a separate pass. THIS FILE IS THAT PASS: the
-- table now exists. Nothing in 0004/0005 has to change for it, and neither is edited.
--
-- WHY A NEW FILE AND NOT AN EDIT OF 0004/0005: core/migrate compares the checksum of every applied
-- file at startup. Editing an applied file either stops the service (ErrChecksumLech) or leaves two
-- databases claiming one schema version while holding two schemas.
--
-- The column set is the domain-expert recommendation the project owner authorised on 2026-09-26.
-- The routes that write and read it are the next card; this file is the schema half.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS, answered before the SQL.
--
--   1. HOW MANY ROWS PER COMMUNE: zero. The table is new and this file writes no row. There is no
--      backfill: the acts that happened before this table existed are in `audit_log`, and
--      reconstructing business timeline rows from a technical trail would be writing history that
--      was never recorded as such (rule 7, forbidden #5).
--   2. IF IT STOPS HALF-WAY: it cannot land half-applied. One file, one transaction, with the
--      progress row written inside it. Every statement is IF NOT EXISTS / CREATE OR REPLACE /
--      DROP TRIGGER IF EXISTS, so a retry costs nothing.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. WHICH READ PATHS CHANGE MEANING WHILE IT IS HALF-APPLIED: none. This file only ADDS one table,
--      its partitions, one index, one function and triggers. No existing table, column, index,
--      constraint or function is touched, and no store query reads this table yet.
--   5. RETENTION: every row is a HISTORICAL RECORD of an act on an archival record (rule 7,
--      forbidden #5). Append-only, enforced by a trigger — see the OPEN DECISION below for the one
--      place that collides with Decree 13/2023.
--
-- ---------------------------------------------------------------------------
-- ⚠ OPEN DECISION FOR THE OWNER — Decree 13/2023 erasure vs append-only. NOT SOLVED HERE.
--
-- `noi_dung` is staff free text and WILL eventually hold citizen personal data (a name, a phone
-- number, an address quoted from the report). An erasure request under Decree 13 means ANONYMISE
-- (rule 3, invariant 7; rule 7, invariant 7) — which is an UPDATE, and the trigger below refuses
-- every UPDATE. This is the same conflict `nhat_ky_nhiem_vu` (0006) and the frozen branch reason
-- (0011, trigger comment) already carry. No erasure path exists in this repository. The day one is
-- built, how it treats this column is the owner's decision, made there — never by loosening this
-- trigger in passing.
--
-- ---------------------------------------------------------------------------
-- WHAT THIS FILE DELIBERATELY DOES NOT DO:
--
--   * No soft-delete columns. There is no state of this table other than "everything that
--     happened"; a row that could be hidden is a timeline that can be made to say something else.
--   * No foreign key to `phieu_phan_anh` — the reason is on the column.
--   * No "from" side of an assignment (`tu_bo_phan_id` …). Same reasoning as 0006:439-448: the
--     "from" is the previous `phan-cong` row of the same petition, and a second copy of a derivable
--     fact is what rule 9's one-line test forbids.
--   * No file store. `dinh_kem` is reserved; see its comment.
--   * No change to `ho_so_luu_tru_bat_bien` (0004) or to 0006's `nhat_ky_nhiem_vu_chi_them`. A
--     separate function means this file reverses by dropping only what it created.

-- ---------------------------------------------------------------------------
-- PostgreSQL 13 is the floor, checked explicitly so the failure is readable.
--
-- BEFORE ... FOR EACH ROW triggers on a PARTITIONED table are only allowed from PostgreSQL 13. On
-- 11 and 12 the CREATE TRIGGER below fails with a message that reads like a syntax mistake and
-- invites moving the trigger onto the partitions — where a partition added later arrives silently
-- unprotected.
-- ---------------------------------------------------------------------------
DO $$ BEGIN
    IF current_setting('server_version_num')::int < 130000 THEN
        RAISE EXCEPTION
            'nhat_ky_phan_anh needs PostgreSQL 13 or newer (server is %). Do not weaken this '
            'migration to fit an older server — the triggers below are what keep a historical '
            'entry from being edited or removed.',
            current_setting('server_version');
    END IF;
END $$;

-- ---------------------------------------------------------------------------
-- @entity: PetitionLogEntry
-- @scope:  tenant
--
-- nhat_ky_phan_anh — the processing timeline of one petition: who did what to it, when, and in
-- which status it stood at that moment.
--
-- APPEND-ONLY, ENFORCED BY A TRIGGER (rule 7, forbidden #5), and therefore WITH NO SOFT-DELETE
-- COLUMNS. Same shape and same reasoning as `nhat_ky_nhiem_vu` (0006:416-552) and
-- service-documents' `lich_su_chuyen_van_ban`.
--
-- IT IS NOT THE AUDIT LOG AND DOES NOT REPLACE IT. `audit_log` answers "who changed what" for the
-- whole service and is invisible to a commune; this is a BUSINESS record the drawer renders and an
-- officer reads and writes. Both are written, in the same transaction, for the same act (rule 6,
-- invariant 3).
--
-- IT IS STAFF-INTERNAL. Routing history and staff notes never reach the citizen (rule 4, forbidden
-- #5; rule 10, invariant 7). A citizen-facing read of this table is a design error, not a feature.
--
-- IT MAY HOLD CITIZEN PERSONAL DATA (rule 3, Decree 13/2023) through `noi_dung`, exactly as
-- `phieu_phan_anh.noi_dung` does. Nothing from this table may reach a log line, an error message or
-- a file name.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS nhat_ky_phan_anh (
    tenant_id                TEXT        NOT NULL,
    id                       TEXT        NOT NULL,

    -- The petition this entry belongs to. NOT a foreign key, for the reason 0006:454-457 gives: a
    -- historical entry must survive a future reshaping of the register, and the write path reads the
    -- petition under a row lock in the same transaction — a stronger check than a constraint on an
    -- id. The petition itself is never hard-deleted (0004, `ho_so_luu_tru_bat_bien`).
    phieu_phan_anh_id        TEXT        NOT NULL,

    thoi_diem                TIMESTAMPTZ NOT NULL,

    -- WHO DID IT, as a staff business code (`CB-00123`) — rule 6, invariant 8. The name somebody
    -- reads years later, when a ULID would name nobody. A system principal writes its own code, never
    -- an empty string.
    nguoi_ma                 TEXT        NOT NULL,

    -- WHAT KIND OF ACT. Closed list; see the CHECK.
    hanh_vi                  TEXT        NOT NULL,

    -- The status the petition stood in AT THIS MOMENT — the chip on the timeline row. Stored rather
    -- than derived: the petition's current status is one value, the timeline needs the status at each
    -- step. For a status-changing act it is the status AFTER the act.
    trang_thai_tai_thoi_diem TEXT        NOT NULL,

    -- Who the petition was handed to AT THIS STEP. Only on `phan-cong`: the petition's own
    -- `bo_phan_id` / `can_bo_xu_ly_id` say who holds it NOW, and "who held it when it went overdue"
    -- cannot be recovered from the current values (same reasoning as 0006:431-437).
    --
    -- `can_bo_xu_ly_ma` holds a STAFF BUSINESS CODE and says so in its name, although the petition's
    -- column is spelled `can_bo_xu_ly_id` — that column holds a business code too
    -- (internal/app/xu_ly_phan_anh.go:225). The new column is named for what it holds (rule 6,
    -- invariant 8), not copied from a misleading name.
    bo_phan_id               TEXT,
    can_bo_xu_ly_ma          TEXT,

    -- ⚠ PERSONAL DATA (rule 3). Staff free text: the note on `ghi-chu`, and an optional remark on any
    -- other act. It will eventually quote the reporter, their number or their address. Never logged,
    -- never in an error message, never on an event. See the OPEN DECISION in the header on erasure.
    noi_dung                 TEXT,

    -- RESERVED: the attachment list, as protojson-shaped references. THERE IS NO FILE STORE IN THIS
    -- REPOSITORY YET, so nothing writes this column today; it defaults to an empty array so a reader
    -- never has to tell "no attachments" from "column not set".
    --
    -- ⚠ WHAT MAY NOT GO IN IT: a file NAME chosen by a citizen or describing a case (rule 3,
    -- forbidden #4 — personal data in file names).
    dinh_kem                 JSONB       NOT NULL DEFAULT '[]'::jsonb,

    tao_luc                  TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (tenant_id, id),

    -- The acts this timeline records. Closed: a new act is a migration, so the drawer's renderer and
    -- this list cannot silently disagree.
    CONSTRAINT nhat_ky_phan_anh_hanh_vi_hop_le CHECK (hanh_vi IN (
        'phan-loai', 'phan-cong', 'chuyen-trang-thai', 'dong-phieu', 'khong-tiep-nhan',
        'chuyen-cap-tren', 'ghi-chu')),

    -- The same NINE codes as the register (0004:332-334, ADR 0027). Written out rather than
    -- referenced, because a CHECK cannot borrow another table's — and a timeline holding a status the
    -- register would refuse contradicts the row it hangs off. migrations/nhat_ky_phan_anh_test.go
    -- compares the two lists.
    CONSTRAINT nhat_ky_phan_anh_trang_thai_hop_le CHECK (trang_thai_tai_thoi_diem IN (
        'da-tiep-nhan', 'dang-phan-loai', 'da-chuyen-xu-ly', 'dang-xu-ly', 'da-xu-ly',
        'cho-dan-xac-nhan', 'da-dong', 'khong-tiep-nhan', 'chuyen-cap-tren')),

    -- STAFF CODE bounded in CHARACTERS: 64 = domain.CanBoToiDa, the bound this service already puts
    -- on a staff business code (internal/domain/xu_ly_phan_anh.go:406-409). Non-blank: '' is not
    -- "nobody", it is a trail entry naming nobody (rule 6, invariant 8 — no fallback).
    CONSTRAINT nhat_ky_phan_anh_nguoi_ma_hop_le
        CHECK (btrim(nguoi_ma) <> '' AND char_length(nguoi_ma) <= 64),

    -- THE ASSIGNMENT PAIR IS BOUND TO `phan-cong`, in both directions:
    --
    --   phan-cong   department present and non-blank · officer optional, non-blank when present
    --   otherwise   both NULL
    --
    -- The department is required because domain.KiemPhanCong requires it (ErrThieuBoPhan); the
    -- officer is optional because "— Để bộ phận phân công —" is a real choice (docs/ui-ux/09 §8.5).
    -- The ELSE arm stops a stray assignee on a note row, which a reader of "who held it at step N"
    -- would take as a hand-over that never happened. Every arm evaluates to TRUE or FALSE, never
    -- NULL — a CHECK treats NULL as passed.
    --
    -- Bounds: 64 = domain.BoPhanToiDa / domain.CanBoToiDa, the same bounds as the petition's own
    -- columns are validated with.
    CONSTRAINT nhat_ky_phan_anh_phan_cong_du_truong CHECK (
        CASE hanh_vi
            WHEN 'phan-cong' THEN
                bo_phan_id IS NOT NULL AND btrim(bo_phan_id) <> ''
                AND (can_bo_xu_ly_ma IS NULL OR btrim(can_bo_xu_ly_ma) <> '')
            ELSE
                bo_phan_id IS NULL AND can_bo_xu_ly_ma IS NULL
        END),
    CONSTRAINT nhat_ky_phan_anh_bo_phan_id_toi_da CHECK (char_length(bo_phan_id) <= 64),
    CONSTRAINT nhat_ky_phan_anh_can_bo_xu_ly_ma_toi_da CHECK (char_length(can_bo_xu_ly_ma) <= 64),

    -- THE NOTE: mandatory and non-blank on `ghi-chu` (a note with nothing in it is a row nobody can
    -- act on, and this table is never edited afterwards); optional elsewhere, but never blank — ''
    -- is not "no remark". The `IS NOT NULL` is what makes an absent note a refusal.
    CONSTRAINT nhat_ky_phan_anh_noi_dung_hop_le CHECK (
        CASE hanh_vi
            WHEN 'ghi-chu' THEN noi_dung IS NOT NULL AND btrim(noi_dung) <> ''
            ELSE noi_dung IS NULL OR btrim(noi_dung) <> ''
        END),

    -- BOUNDED, IN CHARACTERS: char_length counts characters, not bytes, so Vietnamese diacritics do
    -- not shorten the allowance. 2000 = domain.KetQuaToiDa, the bound on the other staff-written
    -- paragraph on a petition. The next card's route must validate the same number with
    -- utf8.RuneCountInString so a client sees a 400, not a 500.
    CONSTRAINT nhat_ky_phan_anh_noi_dung_toi_da CHECK (char_length(noi_dung) <= 2000)
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS nhat_ky_phan_anh_p%s PARTITION OF nhat_ky_phan_anh '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

-- The drawer reads one petition's timeline, newest first. `id` last as the tie-break the cursor
-- pages on (core/store.QueryPage orders by `(sort, id)`): two acts in one transaction share a
-- `thoi_diem`, and without it their order would be read-order, which is to say random.
CREATE INDEX IF NOT EXISTS nhat_ky_theo_phieu_phan_anh
    ON nhat_ky_phan_anh (tenant_id, phieu_phan_anh_id, thoi_diem DESC, id DESC);

-- ---------------------------------------------------------------------------
-- The append-only guard. Same shape as 0006's `nhat_ky_nhiem_vu_chi_them` and for the same
-- reasons, including why the TRUNCATE half has to be attached per partition:
--
--   * a row-level trigger on the PARENT is cloned onto every existing partition and onto every
--     partition added later, and an UPDATE against a partitioned table fires on the leaf — so a
--     statement typed straight at `nhat_ky_phan_anh_p07` hits the clone too;
--   * PostgreSQL refuses a TRUNCATE trigger on a partitioned table, and statement-level triggers
--     are not cloned — so TRUNCATE is covered leaf by leaf. MODULUS 32 is fixed for the life of the
--     system (ADR 0010), and this file is re-runnable if that ever changes.
--
-- INSERT is absent from the event list on purpose: the write path pays nothing.
--
-- The message names the operation and the relation only: an error message travels into logs and
-- back to clients (rule 3, forbidden #3), and `noi_dung` may hold personal data.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION nhat_ky_phan_anh_chi_them() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'nhat_ky_phan_anh is append-only: % on % refused', TG_OP, TG_TABLE_NAME
        USING HINT = 'A petition timeline entry is a historical record (rule 7, forbidden #5): '
                     'never modified, never removed, never truncated. To correct one, write '
                     'another entry carrying the correction.';
END $$;

DROP TRIGGER IF EXISTS nhat_ky_phan_anh_khong_sua_xoa ON nhat_ky_phan_anh;
CREATE TRIGGER nhat_ky_phan_anh_khong_sua_xoa
    BEFORE UPDATE OR DELETE ON nhat_ky_phan_anh
    FOR EACH ROW EXECUTE FUNCTION nhat_ky_phan_anh_chi_them();

DO $$
DECLARE part regclass;
BEGIN
    FOR part IN
        SELECT inhrelid::regclass FROM pg_inherits
        WHERE inhparent = 'nhat_ky_phan_anh'::regclass
    LOOP
        EXECUTE format('DROP TRIGGER IF EXISTS nhat_ky_phan_anh_khong_truncate ON %s', part);
        EXECUTE format(
            'CREATE TRIGGER nhat_ky_phan_anh_khong_truncate BEFORE TRUNCATE ON %s '
            'FOR EACH STATEMENT EXECUTE FUNCTION nhat_ky_phan_anh_chi_them()', part);
    END LOOP;
END $$;

-- ---------------------------------------------------------------------------
-- WHAT THIS FILE DOES NOT STOP, said plainly: DDL by the table owner (ALTER TABLE ... DISABLE
-- TRIGGER, dropping the table), and TRUNCATE ... CASCADE through a parent this file does not own.
-- Same line ADR 0013 draws for the audit ledger — the job is to make the accidental and the
-- convenient impossible, not to defeat an administrator who has decided to destroy data and is
-- willing to be seen doing it.
--
-- REVERSAL (migration question 3). Every object here is new. WHILE THE TABLE IS EMPTY — true in
-- every environment until the next card's route ships — the reversal is complete and loses nothing.
-- In ONE transaction:
--
--   DROP TABLE nhat_ky_phan_anh;            (the 32 partitions and their triggers go with it)
--   DROP FUNCTION nhat_ky_phan_anh_chi_them();
--
-- and remove this file's row from `schema_migration` in the same transaction, otherwise the runner
-- still believes the schema is in place.
--
-- BEFORE DROPPING, CHECK IT IS EMPTY: `SELECT count(*) FROM nhat_ky_phan_anh` must be zero. ONCE
-- ONE ENTRY EXISTS, dropping the table is destroying historical records of acts on archival records
-- — rule 7's first stop condition, which needs the user and a verified backup, not a command. From
-- that point the way back is a NEW migration (core/migrate has no automatic rollback — ADR 0013).
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- BACKSTOP: every partitioned table in this schema must actually have partitions. Same check as
-- 0004's, repeated because this file declares a new PARTITION BY table: one with no partitions
-- REJECTS EVERY INSERT, silently, until the first real write — and because the audit entry shares
-- the business transaction (rule 6, invariant 3), that first write rolls back entirely.
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
