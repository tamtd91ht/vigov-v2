-- identity — residential units, and the two reference catalogues identity owns (ADR 0024).
--
-- WHY A NEW FILE AND NOT AN EDIT OF AN EARLIER ONE: 0001–0004 have been applied and
-- core/migrate compares the checksum of every applied file at startup. Editing an applied file
-- either stops the service (ErrChecksumLech) or leaves two databases claiming one schema
-- version while holding two different schemas.
--
-- WHY THREE TABLES IN ONE FILE: core/migrate gives each file exactly ONE transaction, and
-- thon_to_dan_pho carries a foreign key into loai_don_vi_dan_cu. Split across files there would
-- be an intermediate state in which that key has no target. All three, or none. khoi_nhiem_vu
-- rides along because it is the same decision from the same ADR, applied to the same service.
--
-- ENTITY OWNERSHIP is declared with the `-- @entity` / `-- @scope` marks above each CREATE
-- TABLE (ADR 0021). The generator that reads them does not exist yet; the marks are written now
-- because the moment a table is born is the only moment the answer is certain.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS, answered before the SQL.
--
--   1. HOW MANY ROWS PER COMMUNE: zero. All three tables are new and this file writes no row
--      into any of them — see "WHERE THE he-thong ROWS COME FROM" below. The ceiling in sight
--      is a handful of catalogue rows and a few dozen residential units per commune.
--   2. IF IT STOPS HALF-WAY: it cannot land half-applied. One file, one transaction, with the
--      progress row written inside it. A failure leaves nothing behind and the next start
--      retries from the beginning; every statement is IF NOT EXISTS or CREATE OR REPLACE, so a
--      retry costs nothing.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. WHICH READ PATHS CHANGE MEANING WHILE IT IS HALF-APPLIED: none. This file only ADDS
--      tables, functions and triggers. No existing table, column, index or constraint is
--      touched, so every query that runs today returns exactly what it returned before and no
--      row changes visibility. This is the question that causes incidents, and the honest
--      answer is that the risk moves to the NEXT change — the first one that writes rows here.
--   5. RETENTION: nothing is removed. Residential-unit records are referenced by commune
--      business data and are archival in the same sense, so the soft-delete columns are present
--      from the start and hard DELETE is refused by a trigger rather than by convention.
--
-- ---------------------------------------------------------------------------
-- WHERE THE `nguon = 'he-thong'` ROWS COME FROM — answered, not left hanging.
--
-- THIS FILE CREATES TABLES AND SEEDS NOTHING. Not an omission, and not a convenience: two
-- decisions already in force settle it.
--
--   * A catalogue row here carries tenant_id, so every seeded row would belong to ONE NAMED
--     COMMUNE (ADR 0024, §Vì sao MỌI bảng ở đây mang tenant_id, consequence #2).
--   * The migration runner has no commune in it and deliberately accepts none: the schema is
--     one set of tables for the whole service database, partitioned by tenant_id (ADR 0013,
--     §Giới hạn — "đường chạy này không có xã trong nó"; ADR 0010). There is no list of
--     communes on this code path, and fetching one would mean reading `platform`'s tenant
--     registry from another service's migration — rule 2, forbidden #2.
--
-- So system rows are seed sown PER COMMUNE, and the step that sows them is COMMUNE ONBOARDING.
-- THAT STEP DOES NOT EXIST IN THIS REPOSITORY YET, and designing it is not a decision a
-- migration may take on its own: it fixes who writes a commune's first rows, under which
-- principal in the audit trail (rule 6, invariant 6), and what happens to the communes already
-- onboarded when the shipped list later changes. That question is raised with the user; it is
-- NOT settled here.
--
-- WHAT HOLDS UNTIL IT IS SETTLED: these tables are empty for every commune. A catalogue screen
-- reading them shows an empty list — visibly empty, which is the failure people report, unlike
-- a half-seeded catalogue that looks complete.
--
-- The code lists themselves stay where they already live — docs/ui-ux/14-cau-hinh.md §5. Rule
-- 9: one fact, one owning file. Copying them into an inert SQL comment would create the second
-- copy, and two copies drift.
--
-- ---------------------------------------------------------------------------
-- THE SHAPE OF A REFERENCE CATALOGUE TABLE, and the three things that are easy to get wrong.
--
-- (1) UNIQUE (tenant_id, ma) COUNTS SOFT-DELETED ROWS TOO. It is a plain unique key, not an
--     index restricted to live rows, and that single line decides whether old files can still
--     be read: an issued code is never reissued (rule 7, invariant 3). Restrict it to live rows
--     and a commune can soft-delete `cong-van` and create a new, unrelated `cong-van` — every
--     archival record still holding the old value then silently reads as the new one. Nothing
--     errors; the label on old files is simply wrong from then on.
--
-- (2) THREE TIERS, NOT TWO (ADR 0024 §6). `nguon` answers "who added this row". It does not
--     answer "what may be done to it", and those are different questions:
--
--       tier 1  nguon = 'don-vi'                          soft delete YES  disable YES  relabel YES
--       tier 2  nguon = 'he-thong'                        soft delete NO   disable YES  relabel YES
--       tier 3  nguon = 'he-thong' AND ma_nguon_re_nhanh  soft delete NO   disable NO   relabel YES
--
--     Tier 3 exists because the specification permits exactly the operation that breaks the
--     system: docs/ui-ux/14-cau-hinh.md:182 says a system row cannot be deleted BUT CAN BE
--     DISABLED. Disabling a code the source code branches on leaves that branch with no
--     reachable row, and the screen that offers the button reports nothing wrong — the same
--     failure shape open question #21 describes for task statuses.
--
--     Enforced by TRIGGER, not by a promise in the application layer: same argument ADR 0013
--     makes for append-only. A rule the database does not refuse holds only until somebody
--     writes the UPDATE, and nothing turns red on the day they do.
--
-- (3) EXACTLY ONE DEFAULT PER COMMUNE, via `moc_mac_dinh` — see the comment on that column.
--
-- WHY THE COLUMN IS `ma_nguon_re_nhanh` AND NOT `khong_tat_duoc` / `khoa`: it is named after
-- what it IS, not after what the screen does with it (ADR 0017). The fact it records is a
-- dependency of the SOURCE CODE on this row's `ma` — read it as `mã nguồn · rẽ nhánh`, "the
-- source code branches on this code", not as `mã · nguồn`. "Cannot be disabled" is today's
-- consequence of that fact; name the consequence and the column is wrong the day a second
-- consequence appears (an export, a report grouping), and nobody will dare touch it because the
-- name no longer says what the value means.

-- ---------------------------------------------------------------------------
-- PostgreSQL 13 is the floor, checked explicitly so the failure is readable.
--
-- BEFORE ... FOR EACH ROW triggers on a PARTITIONED table were only allowed from PostgreSQL 13.
-- On 11 and 12 the CREATE TRIGGER statements below fail with "Partitioned tables cannot have
-- BEFORE / FOR EACH ROW triggers", which reads like a syntax mistake and invites somebody to
-- "fix" it by moving the trigger down onto the partitions — where a partition added later
-- arrives silently unprotected. The same floor is already checked in 0002; it is repeated here
-- because a file that carries triggers must state its own requirement.
-- ---------------------------------------------------------------------------
DO $$ BEGIN
    IF current_setting('server_version_num')::int < 130000 THEN
        RAISE EXCEPTION
            'reference catalogues need PostgreSQL 13 or newer (server is %). Do not weaken '
            'this migration to fit an older server — the tier-3 guard is what stops a commune '
            'from disabling a code the source code branches on.',
            current_setting('server_version');
    END IF;
END $$;

-- ---------------------------------------------------------------------------
-- danh_muc_ba_tang — the tier guard, shared by every reference catalogue in this service.
--
-- ONE FUNCTION FOR SEVERAL TABLES on purpose: the tiers are a property of the SHAPE, not of one
-- catalogue. A copy per table is a copy that gets fixed in two places and forgotten in the
-- third. Every table using it must carry ma · nguon · ma_nguon_re_nhanh · dang_dung ·
-- deleted_at; a table without them is not a catalogue and must not attach this trigger.
--
-- WHAT EACH REFUSAL IS FOR, in the order they are checked:
--
--   DELETE            — catalogue rows are commune business data: soft delete only (rule 7,
--                       invariant 1). A hard delete would also free the code for reuse.
--   `ma`              — an issued code is never renumbered (rule 7, invariant 3). Records in
--                       OTHER services hold this code as a VALUE, not as a foreign key
--                       (ADR 0024, §Cái giá của dòng Khối nhiệm vụ), and nothing rewrites them:
--                       editing `ma` turns those references into the raw-code display the ADR
--                       cites as evidence already sitting in the specification.
--   `nguon`           — provenance decides the tier. Were it editable, every guard below could
--                       be walked around by flipping `he-thong` to `don-vi` first.
--   tier 3 -> tier 2  — tightening (false to true) is allowed, because that is how a future
--                       migration records "the code now branches on this row". Loosening is
--                       refused: it is the one edit that makes the next disable succeed, and it
--                       looks harmless in a diff. A genuine demotion is a deliberate act with
--                       DDL behind it (ALTER TABLE ... DISABLE TRIGGER), which leaves a trace in
--                       the server log — the same line ADR 0013 draws.
--   soft delete       — a system row is disabled, never deleted (ADR 0024, consequence #4;
--     of tier 2 / 3     docs/ui-ux/14-cau-hinh.md:182 and §12.2 rule 2).
--   disable tier 3    — the operation the specification allows and the system cannot survive.
--
-- The messages name the operation and the relation and nothing else. TG_TABLE_NAME reports the
-- leaf partition actually hit, which is what an operator needs in order to find out what
-- happened; no row value is printed, because an error message travels into logs and back to
-- clients (rule 3, forbidden #3).
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION danh_muc_ba_tang() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'catalogue %: hard delete refused', TG_TABLE_NAME
            USING HINT = 'Catalogue rows are a commune''s business data: soft delete only '
                         '(deleted_at, deleted_by, delete_reason) — rule 7, invariant 1. '
                         'A hard delete would also free an issued code for reuse, which '
                         'rule 7 invariant 3 forbids.';
    END IF;

    IF NEW.ma IS DISTINCT FROM OLD.ma THEN
        RAISE EXCEPTION 'catalogue %: `ma` is immutable', TG_TABLE_NAME
            USING HINT = 'An issued code is never renumbered (rule 7, invariant 3). Other '
                         'services hold this code AS A VALUE and nothing rewrites them. '
                         'To change the wording, edit `nhan`; to replace the concept, add a '
                         'new row and take this one out of use.';
    END IF;

    IF NEW.nguon IS DISTINCT FROM OLD.nguon THEN
        RAISE EXCEPTION 'catalogue %: `nguon` is immutable', TG_TABLE_NAME
            USING HINT = 'Provenance decides which tier this row is in. If it could be '
                         'edited, every other guard here could be stepped around by setting '
                         'nguon = ''don-vi'' first.';
    END IF;

    IF OLD.ma_nguon_re_nhanh AND NOT NEW.ma_nguon_re_nhanh THEN
        RAISE EXCEPTION 'catalogue %: a tier-3 row cannot be demoted by UPDATE', TG_TABLE_NAME
            USING HINT = 'ma_nguon_re_nhanh records that the source code branches on this '
                         'row''s code. It may be set (a migration recording a new branch), '
                         'never cleared: clearing it is what makes the next disable succeed.';
    END IF;

    IF OLD.nguon = 'he-thong' AND OLD.deleted_at IS NULL AND NEW.deleted_at IS NOT NULL THEN
        RAISE EXCEPTION 'catalogue %: a system row is disabled, never deleted', TG_TABLE_NAME
            USING HINT = 'nguon = ''he-thong'' rows ship with the software (rule 7; ADR 0024, '
                         'consequence #4). Set dang_dung = false to take one out of use.';
    END IF;

    IF OLD.ma_nguon_re_nhanh AND OLD.dang_dung AND NOT NEW.dang_dung THEN
        RAISE EXCEPTION 'catalogue %: a tier-3 row cannot be taken out of use', TG_TABLE_NAME
            USING HINT = 'The source code branches on this row''s code, so taking it out of '
                         'use leaves that branch with nothing to reach and breaks the flow '
                         'silently — the failure shape of open question #21. Relabel it '
                         'instead, or remove the branch first and demote the row in a '
                         'migration.';
    END IF;

    RETURN NEW;
END $$;

-- ---------------------------------------------------------------------------
-- @entity: ResidentialUnitType
-- @scope:  tenant
--
-- loai_don_vi_dan_cu — `thon` / `to-dan-pho`, the classification of thon_to_dan_pho.
--
-- Owned by identity because it classifies a table that lives here (ADR 0024).
--
-- THE TWO SHIPPED CODES ARE TIER 3. `thon` and `to-dan-pho` are the values thon_to_dan_pho.loai
-- takes, and the foreign key below points at them — a commune taking one out of use is taking
-- out of use a value its own residential-unit records already carry.
--
-- READ THIS BEFORE ADDING TO THIS TABLE — it may not be a catalogue at all. If the two kinds of
-- residential unit are FIXED BY LAW, this should be an enum column on thon_to_dan_pho rather
-- than a catalogue table, because a catalogue is by definition something a commune can switch
-- off, and switching off `thon` is precisely the failure open question #21 describes for task
-- statuses: the specification permits the operation, the screen offers the button, and nothing
-- reports the breakage. The tier-3 flag closes the disable path; it does not answer the
-- modelling question. → kb/00-foundation/open-questions.json #21 (same failure shape, a
-- different list).
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS loai_don_vi_dan_cu (
    tenant_id         TEXT        NOT NULL,
    id                TEXT        NOT NULL,
    ma                TEXT        NOT NULL,          -- "thon"
    nhan              TEXT        NOT NULL,          -- "Thôn"
    thu_tu            INT         NOT NULL DEFAULT 0,
    nguon             TEXT        NOT NULL DEFAULT 'don-vi',
    -- The source code branches on this row's `ma` — tier 3. Written by the step that sows the
    -- system rows, never by a commune.
    ma_nguon_re_nhanh BOOLEAN     NOT NULL DEFAULT false,
    la_mac_dinh       BOOLEAN     NOT NULL DEFAULT false,
    dang_dung         BOOLEAN     NOT NULL DEFAULT true,
    deleted_at        TIMESTAMPTZ,
    deleted_by        TEXT,
    delete_reason     TEXT,
    -- EXACTLY ONE DEFAULT PER COMMUNE. Two defaults make the item the screen pre-selects depend
    -- on read order, which is to say random, and nothing on the screen shows that it is.
    --
    -- The obvious form is a partial unique index — UNIQUE (tenant_id) WHERE la_mac_dinh — and it
    -- is deliberately not used. Unique keys on a PARTITIONED table are restricted (they must
    -- contain every partition-key column), and whether the partial variant is accepted could
    -- not be verified against a real server from this repository: no PostgreSQL is reachable
    -- from the build environment and VIGOV_TEST_DSN is unset, so the integration suites skip
    -- without running a single statement (ADR 0013, last section). A migration that fails at
    -- startup stops the service, so the form that needs no such verification wins.
    --
    -- This one rests only on the standard rule that NULLs do not collide: the marker is true for
    -- the single live default and NULL for every other row, so UNIQUE (tenant_id, moc_mac_dinh)
    -- admits at most one default per commune and any number of non-defaults beside it. Soft
    -- deleting the default frees the slot by itself, with no second statement to forget.
    moc_mac_dinh      BOOLEAN GENERATED ALWAYS AS
                          (CASE WHEN la_mac_dinh AND deleted_at IS NULL THEN true END) STORED,
    tao_luc           TIMESTAMPTZ NOT NULL DEFAULT now(),
    cap_nhat_luc      TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, id),
    -- Composite with tenant_id, and counting soft-deleted rows — see (1) in the header.
    UNIQUE (tenant_id, ma),
    UNIQUE (tenant_id, moc_mac_dinh),
    CONSTRAINT loai_don_vi_dan_cu_nguon_hop_le
        CHECK (nguon IN ('he-thong', 'don-vi')),
    -- A commune's own row can never be tier 3: the software does not branch on a code it has
    -- never seen.
    CONSTRAINT loai_don_vi_dan_cu_re_nhanh_thi_he_thong
        CHECK (NOT ma_nguon_re_nhanh OR nguon = 'he-thong')
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS loai_don_vi_dan_cu_p%s PARTITION OF loai_don_vi_dan_cu '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

-- The catalogue screen lists every row of one commune in display order, rows out of use
-- included (they carry a "Đã tắt" chip); only soft-deleted rows drop out — everywhere, always
-- (rule 7, invariant 2).
CREATE INDEX IF NOT EXISTS loai_don_vi_dan_cu_danh_sach
    ON loai_don_vi_dan_cu (tenant_id, thu_tu) WHERE deleted_at IS NULL;

DROP TRIGGER IF EXISTS loai_don_vi_dan_cu_ba_tang ON loai_don_vi_dan_cu;
CREATE TRIGGER loai_don_vi_dan_cu_ba_tang
    BEFORE UPDATE OR DELETE ON loai_don_vi_dan_cu
    FOR EACH ROW EXECUTE FUNCTION danh_muc_ba_tang();

-- ---------------------------------------------------------------------------
-- @entity: TaskBloc
-- @scope:  tenant
--
-- khoi_nhiem_vu — `khoi-uy-ban` / `khoi-dang` / `khac`.
--
-- THIS TABLE LIVES IN identity ALTHOUGH ITS ENTITY NAME CARRIES "Task", AND THAT IS NOT A SLIP.
-- Do not "tidy it up" by moving it to petitions: that rebuilds exactly the two-node dependency
-- ADR 0024 spent a page of argument removing — see
-- kb/10-decisions/0024-so-huu-danh-muc-tham-chieu.md line 130, which forbids the move in
-- advance, and line 65, which gives the reason it sits here: the list changes in step with the
-- ORG CHART (khối Uỷ ban / khối Đảng), not in step with tasks.
--
-- THE PRINCIPLE, written down so the next reader does not have to rediscover it: THE NAME
-- FOLLOWS THE CONCEPT, OWNERSHIP FOLLOWS THE RATE OF CHANGE. The two are allowed to differ, and
-- here they do, for a reason. The consumer of these codes is the task record in `petitions`,
-- which holds the code as a VALUE — no foreign key across the service boundary, no JOIN into
-- this schema (ADR 0024, §Cái giá của dòng Khối nhiệm vụ).
--
-- WHEN TO CHANGE YOUR MIND, so it is not argued again from scratch: if `bo_phan` grows a `khoi`
-- column, the concept has moved to the org chart and the NAME must follow it. That is a rename
-- of the entity mark, not a move of the table.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS khoi_nhiem_vu (
    tenant_id         TEXT        NOT NULL,
    id                TEXT        NOT NULL,
    ma                TEXT        NOT NULL,          -- "khoi-uy-ban"
    nhan              TEXT        NOT NULL,          -- "Khối Uỷ ban"
    thu_tu            INT         NOT NULL DEFAULT 0,
    nguon             TEXT        NOT NULL DEFAULT 'don-vi',
    ma_nguon_re_nhanh BOOLEAN     NOT NULL DEFAULT false,
    la_mac_dinh       BOOLEAN     NOT NULL DEFAULT false,
    dang_dung         BOOLEAN     NOT NULL DEFAULT true,
    deleted_at        TIMESTAMPTZ,
    deleted_by        TEXT,
    delete_reason     TEXT,
    moc_mac_dinh      BOOLEAN GENERATED ALWAYS AS
                          (CASE WHEN la_mac_dinh AND deleted_at IS NULL THEN true END) STORED,
    tao_luc           TIMESTAMPTZ NOT NULL DEFAULT now(),
    cap_nhat_luc      TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, id),
    UNIQUE (tenant_id, ma),
    UNIQUE (tenant_id, moc_mac_dinh),
    CONSTRAINT khoi_nhiem_vu_nguon_hop_le
        CHECK (nguon IN ('he-thong', 'don-vi')),
    CONSTRAINT khoi_nhiem_vu_re_nhanh_thi_he_thong
        CHECK (NOT ma_nguon_re_nhanh OR nguon = 'he-thong')
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS khoi_nhiem_vu_p%s PARTITION OF khoi_nhiem_vu '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

CREATE INDEX IF NOT EXISTS khoi_nhiem_vu_danh_sach
    ON khoi_nhiem_vu (tenant_id, thu_tu) WHERE deleted_at IS NULL;

DROP TRIGGER IF EXISTS khoi_nhiem_vu_ba_tang ON khoi_nhiem_vu;
CREATE TRIGGER khoi_nhiem_vu_ba_tang
    BEFORE UPDATE OR DELETE ON khoi_nhiem_vu
    FOR EACH ROW EXECUTE FUNCTION danh_muc_ba_tang();

-- ---------------------------------------------------------------------------
-- @entity: ResidentialUnit
-- @scope:  tenant
--
-- thon_to_dan_pho — the residential units of a commune.
--
-- NOT A REFERENCE CATALOGUE: it holds data of its own (household and population counts) and is
-- referenced by the commune's business records, so it keeps the soft-delete columns and the
-- hard-delete refusal but none of the tier machinery (ADR 0024, §Cộng thêm).
--
-- WHY `loai` IS A REAL FOREIGN KEY HERE while the same ADR forbids one from the task record to
-- khoi_nhiem_vu: both tables are in THIS service's schema, so the key crosses no boundary. It is
-- the cheap version of the check ADR 0024 asks for at WRITE time — the specification already
-- contains one code that outlived its catalogue entry and now prints raw
-- (docs/ui-ux/14-cau-hinh.md §8, `ve-sinh-moi-truong`), and within one schema the database will
-- simply refuse that.
--
-- `loai` IS NULLABLE because the specification's own column list shows `—` as a legitimate
-- value (docs/ui-ux/14-cau-hinh.md:54): a unit whose classification has not been entered yet.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS thon_to_dan_pho (
    tenant_id     TEXT        NOT NULL,
    id            TEXT        NOT NULL,
    ten           TEXT        NOT NULL,              -- "Thôn Bình An"
    ma            TEXT        NOT NULL,              -- slug: "thon-binh-an"
    loai          TEXT,                              -- code from loai_don_vi_dan_cu; NULL = "—"
    -- Household and population counts. NULLABLE ON PURPOSE: 0 is a statement about a unit ("no
    -- households"), NULL is the absence of one ("not entered"). A commune importing a
    -- spreadsheet without that column must not have zeros asserted on its behalf, because a
    -- zero travels onward into reports as a number.
    so_ho         INT,
    nhan_khau     INT,
    dang_dung     BOOLEAN     NOT NULL DEFAULT true,
    deleted_at    TIMESTAMPTZ,
    deleted_by    TEXT,
    delete_reason TEXT,
    tao_luc       TIMESTAMPTZ NOT NULL DEFAULT now(),
    cap_nhat_luc  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, id),
    -- Composite with tenant_id, and counting soft-deleted rows: two communes both having a
    -- `thon-binh-an` is normal and must stay normal (rule 1, forbidden #4), while one commune
    -- must never reuse a code its own archival records already point at.
    UNIQUE (tenant_id, ma),
    FOREIGN KEY (tenant_id, loai) REFERENCES loai_don_vi_dan_cu (tenant_id, ma),
    CONSTRAINT thon_to_dan_pho_so_lieu_khong_am
        CHECK ((so_ho IS NULL OR so_ho >= 0) AND (nhan_khau IS NULL OR nhan_khau >= 0))
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS thon_to_dan_pho_p%s PARTITION OF thon_to_dan_pho '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

CREATE INDEX IF NOT EXISTS thon_to_dan_pho_danh_sach
    ON thon_to_dan_pho (tenant_id, ten) WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS thon_to_dan_pho_theo_loai
    ON thon_to_dan_pho (tenant_id, loai) WHERE deleted_at IS NULL;

-- ---------------------------------------------------------------------------
-- The same two refusals as the catalogue guard, minus the tiers this table does not have.
-- A separate function because plpgsql resolves NEW.nguon when the trigger RUNS and this table
-- has no such column: attaching danh_muc_ba_tang() here would fail at the first UPDATE rather
-- than at CREATE TRIGGER.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION thon_to_dan_pho_giu_ban_ghi() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'thon_to_dan_pho: hard delete on % refused', TG_TABLE_NAME
            USING HINT = 'Residential units are referenced by the commune''s business records: '
                         'soft delete only (deleted_at, deleted_by, delete_reason) — rule 7, '
                         'invariant 1. Removing the row would leave those records pointing at '
                         'nothing.';
    END IF;

    IF NEW.ma IS DISTINCT FROM OLD.ma THEN
        RAISE EXCEPTION 'thon_to_dan_pho: `ma` is immutable'
            USING HINT = 'The code is what other records hold (rule 7, invariant 3). Rename the '
                         'unit with `ten`; a genuinely new unit is a new row.';
    END IF;

    RETURN NEW;
END $$;

DROP TRIGGER IF EXISTS thon_to_dan_pho_giu ON thon_to_dan_pho;
CREATE TRIGGER thon_to_dan_pho_giu
    BEFORE UPDATE OR DELETE ON thon_to_dan_pho
    FOR EACH ROW EXECUTE FUNCTION thon_to_dan_pho_giu_ban_ghi();

-- ---------------------------------------------------------------------------
-- WHAT THIS FILE DOES NOT STOP, said plainly rather than left to be discovered: TRUNCATE, and
-- DDL by the table owner (ALTER TABLE ... DISABLE TRIGGER, dropping the table). Same line
-- ADR 0013 draws for the audit ledger — the job here is to make the accidental and the
-- convenient impossible (an UPDATE written by mistake in Go, a statement typed at a psql
-- prompt), not to defeat an administrator who has decided to destroy data and is willing to be
-- seen doing it.
--
-- REVERSAL (migration question 3). Every object here is new, and while the tables are still
-- empty the reversal is complete and loses nothing: drop the three tables in dependency order
-- (thon_to_dan_pho first — it holds the foreign key), then the two trigger functions, and in
-- the same transaction remove this file's row from `schema_migration`, otherwise the runner
-- still believes the schema is in place. The 32 partitions and the triggers go with their
-- parent tables.
--
-- ONCE A COMMUNE HAS ROWS HERE, THAT IS NO LONGER A REVERSAL — it is the destruction of commune
-- records, which is rule 7's first stop condition and needs the user, not a command. From that
-- point the way back is a NEW migration, and core/migrate has no automatic rollback for exactly
-- this reason (ADR 0013): undoing DDL over archival records is an administrative procedure, not
-- something a process decides at 3 a.m.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- BACKSTOP: every partitioned table in this schema must actually have partitions.
--
-- A table declared PARTITION BY and given no partitions REJECTS EVERY INSERT, silently, until
-- the first real write — and because the audit entry shares the business transaction (rule 6,
-- invariant 3), that first write rolls back entirely. The check is repeated at the end of every
-- migration that declares a partitioned table, because it only verifies the state after a file
-- that CARRIES it (0002, §BACKSTOP).
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
