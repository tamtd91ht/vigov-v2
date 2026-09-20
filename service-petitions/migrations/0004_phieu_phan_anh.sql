-- petitions — the petition register (`phieu_phan_anh`) and the commune's LABELS for the
-- field codes (`nhan_linh_vuc`, tier 2 of ADR 0026).
--
-- WHY A NEW FILE AND NOT AN EDIT OF 0001/0003: those have been applied and core/migrate compares
-- the checksum of every applied file at startup. Editing an applied file either stops the service
-- (ErrChecksumLech) or leaves two databases claiming one schema version while holding two
-- different schemas.
--
-- ENTITY OWNERSHIP is declared with the `-- @entity` / `-- @scope` marks above each CREATE TABLE
-- (ADR 0021).
--
-- ---------------------------------------------------------------------------
-- WHAT THIS FILE DELIBERATELY DOES NOT CREATE, so the absence is not read as an oversight:
--
--   * THE TIER-1 FIELD CODE SET. ADR 0026 §Bổ sung 2026-09-20 settles it in service `platform`,
--     and writing it here is that ADR's stop condition #1. `linh_vuc` below therefore holds a
--     CODE AS A VALUE: no foreign key, no JOIN, ever (ADR 0026 §2).
--   * THE `sla` TABLE (docs/ui-ux/14-cau-hinh.md §8). Its rows cover `van-ban-den` (documents),
--     `phan-anh` and `nhiem-vu` (petitions), so which service owns it is rule 2's stop
--     condition #1 — exactly the question the user answered for `lich_lam_viec` by putting the
--     three calendar tables in `identity`. Nobody has answered it for `sla`. Guessing here
--     would put one configuration screen's table in two services, or put another service's
--     numbers in this one.
--   * `anh_phan_anh` and `nhat_ky_phan_anh` (docs/ui-ux/09 §12). They are separate tables for a
--     later pass; nothing below references them.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS, answered before the SQL.
--
--   1. HOW MANY ROWS PER COMMUNE: zero. Both tables are new and this file writes no row. There
--      is no seed, for the reason 0003 states at length: a row here carries tenant_id, so
--      seeding would mean seeding for ONE NAMED COMMUNE, and the step that sows a commune's
--      first rows — onboarding — does not exist in this repository.
--   2. IF IT STOPS HALF-WAY: it cannot land half-applied. One file, one transaction, with the
--      progress row written inside it. Every statement is IF NOT EXISTS or CREATE OR REPLACE,
--      so a retry costs nothing.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. WHICH READ PATHS CHANGE MEANING WHILE IT IS HALF-APPLIED: none. This file only ADDS
--      tables, a function and triggers. No existing table, column, index or constraint is
--      touched.
--   5. RETENTION: a petition is an ARCHIVAL RECORD with a statutory retention period (rule 7).
--      Soft delete only, and hard DELETE is refused by a trigger rather than by convention.
--
-- ---------------------------------------------------------------------------
-- THE TWO DEADLINE COLUMNS, AND THE ONE THING A LATER SESSION WILL GET BACKWARDS.
--
-- A petition has TWO clocks, SET AT TWO MOMENTS, counted from ONE origin (ADR 0028, decision E):
--
--   han_tiep_nhan   set when the ROW IS CREATED, from the DEFAULT SLA row's `gio_tiep_nhan`
--   han_xu_ly_xong  set when the officer FIRST SETTLES THE FIELD (status -> `dang-phan-loai`),
--                   from that field's `gio_xu_ly_xong` — counted from the SAME `goc_dem_han`
--
-- A petition booked by staff (`can-bo-nhap-ho`) sets `han_xu_ly_xong` at creation instead,
-- because its form carries the field (docs/ui-ux/09 §11). The rule is written by FORM SHAPE and
-- not by channel list, so a fifth channel does not rewrite it.
--
-- BOTH COLUMNS ARE NULLABLE AND THE TWO NULLS MEAN OPPOSITE THINGS. This is the line to read
-- twice, because reading it backwards does not fail — it produces a pretty, false figure in a
-- report that goes to leadership:
--
--   han_tiep_nhan IS NULL    "KHÔNG ÁP DỤNG".  A staff-booked petition: the officer IS the
--                            reader, so the interval "how long until somebody read it" does not
--                            exist. CONSEQUENCE: every average acknowledge-time report must
--                            EXCLUDE these rows from its denominator. COALESCE(han_tiep_nhan, 0)
--                            makes every staff-booked petition a valid "acknowledged instantly"
--                            sample, and a commune that books a lot of them reports an average
--                            near zero — beautiful, wrong, and unreadable from the figure itself
--                            (ADR 0028, decision F #5 and §han_tiep_nhan = NULL).
--
--   han_xu_ly_xong IS NULL   "CHƯA CÓ".  The petition has not been classified yet, so no resolve
--                            commitment has been fixed. CONSEQUENCE: the screen must NOT invent a
--                            date to show the citizen, and an on-time ratio must exclude these
--                            rows AND show how many were excluded — which is open question #26,
--                            still the customer's.
--
-- THERE IS NO `is_overdue` COLUMN AND THERE MUST NEVER BE ONE. Overdue is DERIVED, by comparing
-- a deadline with now (rule 10, invariant 3). A stored flag is wrong the moment a job is late or
-- a holiday is entered, and the stale copy is the one that reaches the report.

-- ---------------------------------------------------------------------------
-- PostgreSQL 13 is the floor, checked explicitly so the failure is readable.
--
-- BEFORE ... FOR EACH ROW triggers on a PARTITIONED table were only allowed from PostgreSQL 13.
-- On 11 and 12 the CREATE TRIGGER statements below fail with a message that reads like a syntax
-- mistake and invites somebody to "fix" it by moving the trigger down onto the partitions —
-- where a partition added later arrives silently unprotected.
-- ---------------------------------------------------------------------------
DO $$ BEGIN
    IF current_setting('server_version_num')::int < 130000 THEN
        RAISE EXCEPTION
            'phieu_phan_anh needs PostgreSQL 13 or newer (server is %). Do not weaken this '
            'migration to fit an older server — the triggers below are what stop an archival '
            'record from being deleted or renumbered.',
            current_setting('server_version');
    END IF;
END $$;

-- ---------------------------------------------------------------------------
-- ho_so_luu_tru_bat_bien — the archival guard for the petition register.
--
-- IT IS NOT danh_muc_ba_tang (0003). That function guards REFERENCE CATALOGUES and requires
-- `nguon` / `ma_nguon_re_nhanh` / `dang_dung`, none of which a petition has. Attaching it here
-- would fail at runtime on the first UPDATE, inside the business transaction — which, because
-- the audit entry shares that transaction (rule 6, invariant 3), rolls the whole intake back.
--
-- WHAT EACH REFUSAL IS FOR:
--
--   DELETE          a petition is an archival record with a statutory retention period. Soft
--                   delete only (rule 7, invariant 1). A hard delete would also free the lookup
--                   code for reuse.
--   `ma_tra_cuu`    an issued code is never reissued or renumbered (rule 7, invariant 3). The
--                   citizen was handed this string; rewriting it breaks the one way they have of
--                   finding their own petition.
--   `goc_dem_han`   the origin both clocks were counted from. Editing it after the fact makes
--                   the two stored deadlines unexplainable — they would no longer be derivable
--                   from anything in the row, and an inspection cannot be answered with "the
--                   number came from somewhere".
--   `kenh_tiep_nhan` it decides which of the two NULL meanings applies to `han_tiep_nhan`.
--                   Flipping it turns "không áp dụng" into "chưa có" on rows already written,
--                   silently, in every report at once.
--
-- The messages name the operation and the relation and nothing else: an error message travels
-- into logs and back to clients (rule 3, forbidden #3), and this table holds citizen personal
-- data.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION ho_so_luu_tru_bat_bien() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'archival record %: hard delete refused', TG_TABLE_NAME
            USING HINT = 'A petition is an archival record with a statutory retention period: '
                         'soft delete only (deleted_at, deleted_by, delete_reason) — rule 7, '
                         'invariant 1. An erasure request under Decree 13/2023 means ANONYMISE '
                         'the identifying fields, keeping the business record.';
    END IF;

    IF NEW.ma_tra_cuu IS DISTINCT FROM OLD.ma_tra_cuu THEN
        RAISE EXCEPTION 'archival record %: `ma_tra_cuu` is immutable', TG_TABLE_NAME
            USING HINT = 'The lookup code was handed to the citizen the moment the petition was '
                         'received (rule 10, invariant 1). It is never reissued and never '
                         'renumbered (rule 7, invariant 3).';
    END IF;

    IF NEW.goc_dem_han IS DISTINCT FROM OLD.goc_dem_han THEN
        RAISE EXCEPTION 'archival record %: `goc_dem_han` is immutable', TG_TABLE_NAME
            USING HINT = 'Both stored deadlines were counted from this instant (ADR 0027 '
                         'decision D). Moving it leaves two commitments that can no longer be '
                         'explained from the row they sit in.';
    END IF;

    IF NEW.kenh_tiep_nhan IS DISTINCT FROM OLD.kenh_tiep_nhan THEN
        RAISE EXCEPTION 'archival record %: `kenh_tiep_nhan` is immutable', TG_TABLE_NAME
            USING HINT = 'The channel decides whether han_tiep_nhan IS NULL means "not '
                         'applicable" or is simply absent (ADR 0028). Changing it re-reads every '
                         'already-written row a different way.';
    END IF;

    RETURN NEW;
END $$;

-- ---------------------------------------------------------------------------
-- @entity: PetitionFieldLabel
-- @scope:  tenant
--
-- nhan_linh_vuc — TIER 2 of ADR 0026: the commune's own wording for a field code it may not add
-- to and may not remove from.
--
-- THE ONLY OPERATION THIS TABLE EXISTS FOR IS RE-WORDING. `ma` holds a tier-1 code owned by
-- service `platform`; the commune writes `nhan`. A row here is an OVERRIDE, not a catalogue
-- entry: a code with no row shows the platform's default label, and a screen reads the two
-- together (ADR 0026 §Quyết định).
--
-- NO FOREIGN KEY TO THE TIER-1 TABLE, AND THERE CAN NEVER BE ONE: it lives in another service's
-- database, and a connection to it is rule 2, forbidden #2. The check that `ma` names a real
-- code happens ON WRITE, in the application, against the tier-1 read path — which does not exist
-- yet and needs its own ADR (ADR 0026, stop condition #2). Until it does there is no write route
-- for this table, and the evidence for why that check cannot be skipped is already in the
-- repository: docs/ui-ux/14-cau-hinh.md:308 holds an SLA row pointing at `ve-sinh-moi-truong`, a
-- code no longer in the catalogue, and the screen renders the raw code.
--
-- The column is `nhan` — a LABEL — so the contract answers `label` and not `name`
-- (kb/00-foundation/ubiquitous-language.md owns that distinction).
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS nhan_linh_vuc (
    tenant_id     TEXT        NOT NULL,
    id            TEXT        NOT NULL,
    ma            TEXT        NOT NULL,          -- tier-1 code held AS A VALUE: "rac-thai"
    nhan          TEXT        NOT NULL,          -- "Rác thải – Vệ sinh môi trường"
    thu_tu        INT         NOT NULL DEFAULT 0,
    deleted_at    TIMESTAMPTZ,
    deleted_by    TEXT,
    delete_reason TEXT,
    tao_luc       TIMESTAMPTZ NOT NULL DEFAULT now(),
    cap_nhat_luc  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, id),
    -- COMPOSITE WITH tenant_id, and counting soft-deleted rows. One override per code per
    -- commune: two live rows for `rac-thai` would make the label a commune sees depend on read
    -- order, which is to say random, with nothing on the screen showing it.
    UNIQUE (tenant_id, ma)
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS nhan_linh_vuc_p%s PARTITION OF nhan_linh_vuc '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

CREATE INDEX IF NOT EXISTS nhan_linh_vuc_danh_sach
    ON nhan_linh_vuc (tenant_id, thu_tu, ma) WHERE deleted_at IS NULL;

-- ---------------------------------------------------------------------------
-- @entity: Petition
-- @scope:  tenant
--
-- phieu_phan_anh — one petition. The one object in this system a CITIZEN creates and then
-- watches, and the surface the commune is judged on (rule 10).
--
-- IT HOLDS CITIZEN PERSONAL DATA (rule 3, Decree 13/2023): the reporter's name, their phone
-- number, the address of the incident, its coordinates and the free text of the report itself.
-- Nothing on this table may reach a log line, an error message or a file name. Everything
-- leaving the API is masked unless the caller holds an explicit full-view permission — and no
-- such permission key exists in `quyen` today, which is a question for the customer, not a gap
-- to fill with a default.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS phieu_phan_anh (
    tenant_id            TEXT        NOT NULL,
    id                   TEXT        NOT NULL,

    -- THE CODE THE CITIZEN IS HANDED (rule 10, invariant 1). Not sequential and not short:
    -- a guessable code enumerates other citizens' petitions (rule 4, invariant 4). Generated in
    -- domain.SinhMaTraCuu — see the reasoning there on the alphabet and the length.
    --
    -- docs/ui-ux/09 §7 shows a SEQUENTIAL display code, `PA-2026-0021`. That is deliberately not
    -- this column and is deliberately not implemented: a sequential code on a lookup path is
    -- rule 10, forbidden #5. Whether the commune also wants a separate internal register number
    -- (like `so_den` for documents, which IS sequential per authority and per year) is a
    -- question nobody has answered.
    ma_tra_cuu           TEXT        NOT NULL,

    -- `zalo-mini-app` | `zalo-oa` | `web-xa` | `can-bo-nhap-ho` (docs/ui-ux/09 §12).
    -- IMMUTABLE — see ho_so_luu_tru_bat_bien.
    kenh_tiep_nhan       TEXT        NOT NULL,

    -- The citizen who filed it, opaque. NULL for a staff-booked petition where no citizen
    -- account was involved.
    --
    -- IT IS STORED EVEN WHEN `an_danh` IS TRUE. Anonymity means hidden from staff screens and
    -- from the public page, NOT "identity not recorded" (ADR 0008): without it there is no
    -- anti-spam, the citizen cannot find their own petition again, and a defamatory report
    -- becomes absolutely untraceable — which a public authority cannot accept.
    cong_dan_id          TEXT,

    noi_dung             TEXT        NOT NULL,

    -- TIER-1 FIELD CODE HELD AS A VALUE. No foreign key, no JOIN (ADR 0026 §2).
    --
    -- NULL UNTIL CLASSIFICATION on a citizen-submitted petition: nobody knows the field before a
    -- human reads the report, and letting the citizen pick it is closed in the other direction
    -- (open question #23, ADR 0028) because it would let every report claim the most urgent SLA.
    linh_vuc             TEXT,

    dia_chi              TEXT,
    thon_id              TEXT,
    -- For the heat map. NUMERIC and not float: a coordinate that drifts in the last digits
    -- between two reads is a marker that moves on a map for no reason.
    lat                  NUMERIC(9,6),
    lng                  NUMERIC(9,6),

    nguoi_gui_ho_ten     TEXT,
    nguoi_gui_dien_thoai TEXT,
    an_danh              BOOLEAN     NOT NULL DEFAULT false,

    -- One of the NINE codes. The list is CLOSED and the customer approved the nine strings
    -- verbatim on 2026-09-20 (ADR 0027): from now on changing one is a migration of archival
    -- records, not a rename. Table of codes and labels:
    -- kb/00-foundation/ubiquitous-language.md §Chín trạng thái.
    trang_thai           TEXT        NOT NULL,

    bo_phan_id           TEXT,
    can_bo_xu_ly_id      TEXT,

    -- THE ORIGIN BOTH CLOCKS ARE COUNTED FROM (ADR 0027, decision D): the instant the citizen
    -- pressed send. For a staff-booked petition it is when the citizen ACTUALLY reported it if
    -- the officer recorded that, otherwise the booking instant, bounded to 7 days — see the
    -- CHECK below. IMMUTABLE.
    goc_dem_han          TIMESTAMPTZ NOT NULL,

    -- When the row was created. It is NOT the same fact as `goc_dem_han` and the two must not be
    -- collapsed: on the staff-booked channel they differ by up to a week, and that gap is
    -- precisely the time the citizen has already been waiting.
    vao_so_luc           TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- NULL = "KHÔNG ÁP DỤNG". See the long note at the top of this file before touching it.
    -- NEVER write 0 hours here for a staff-booked petition (ADR 0028, stop condition #2).
    han_tiep_nhan        TIMESTAMPTZ,

    -- NULL = "CHƯA CÓ" — not yet classified. Opposite meaning to the column above.
    han_xu_ly_xong       TIMESTAMPTZ,

    -- The human act that stops the acknowledge clock and fixes `han_xu_ly_xong`.
    phan_loai_luc        TIMESTAMPTZ,
    xu_ly_xong_luc       TIMESTAMPTZ,
    dong_luc             TIMESTAMPTZ,

    -- Default false: a petition is not public until somebody has read it (docs/ui-ux/09 §14.4).
    -- The citizen who filed it can always find their own, which is a different path.
    hien_cong_khai       BOOLEAN     NOT NULL DEFAULT false,

    diem_hai_long        INT,
    danh_gia_luc         TIMESTAMPTZ,

    -- A COUNT, not a flag: ADR 0008 caps reopening with the per-commune `so_lan_mo_lai_toi_da`
    -- (default 1), and a boolean cannot enforce a cap of two.
    so_lan_mo_lai        INT         NOT NULL DEFAULT 0,

    deleted_at           TIMESTAMPTZ,
    deleted_by           TEXT,
    delete_reason        TEXT,
    tao_luc              TIMESTAMPTZ NOT NULL DEFAULT now(),
    cap_nhat_luc         TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (tenant_id, id),

    -- COMPOSITE, and counting soft-deleted rows: an issued lookup code is never reissued, even
    -- after a soft delete (rule 7, invariant 3). Restricted to live rows, a commune could
    -- soft-delete a petition and later mint the same code for an unrelated one — and the citizen
    -- holding the old slip would open somebody else's report.
    UNIQUE (tenant_id, ma_tra_cuu),

    CONSTRAINT phieu_phan_anh_trang_thai_hop_le CHECK (trang_thai IN (
        'da-tiep-nhan', 'dang-phan-loai', 'da-chuyen-xu-ly', 'dang-xu-ly', 'da-xu-ly',
        'cho-dan-xac-nhan', 'da-dong', 'khong-tiep-nhan', 'chuyen-cap-tren')),

    CONSTRAINT phieu_phan_anh_kenh_hop_le CHECK (kenh_tiep_nhan IN (
        'zalo-mini-app', 'zalo-oa', 'web-xa', 'can-bo-nhap-ho')),

    -- The staff-booked form REQUIRES the field at booking (docs/ui-ux/09 §11), and that is the
    -- whole reason its resolve deadline can be fixed at booking. A row that claims the channel
    -- and carries no field would sit with a NULL `han_xu_ly_xong` nobody will ever come back to
    -- fill, because the classification step it depends on never happens on that channel.
    CONSTRAINT phieu_phan_anh_nhap_ho_co_linh_vuc
        CHECK (kenh_tiep_nhan <> 'can-bo-nhap-ho' OR linh_vuc IS NOT NULL),

    -- ADR 0028, decision F #5, enforced rather than promised. `0 giờ` written here would make
    -- every staff-booked petition a valid "acknowledged instantly" sample in every average.
    CONSTRAINT phieu_phan_anh_nhap_ho_khong_co_han_tiep_nhan
        CHECK (kenh_tiep_nhan <> 'can-bo-nhap-ho' OR han_tiep_nhan IS NULL),

    -- A deadline is reached AT OR AFTER the instant it was counted from — always, including when
    -- the origin falls outside every working session (ADR 0007, decision 8). This catches the
    -- one mistake that looks right in a diff: writing `goc_dem_han` into a deadline column.
    CONSTRAINT phieu_phan_anh_han_sau_goc CHECK (
        (han_tiep_nhan  IS NULL OR han_tiep_nhan  >= goc_dem_han) AND
        (han_xu_ly_xong IS NULL OR han_xu_ly_xong >= goc_dem_han)),

    -- BOUNDED AT BOTH ENDS, AND OUT OF RANGE IS A REFUSAL. An arbitrary past mark is the right
    -- to manufacture an already-overdue petition for somebody else, or to hide a late one by
    -- moving its origin back — both are falsified figures produced through a form field, with no
    -- software fault involved.
    --
    -- @sla-ok: ADR 0028 quyết định F #3 — bảy ngày ở đây là NGÀY LỊCH (khoảng người dân còn nhớ
    -- được), KHÔNG phải một hạn xử lý. Hạn xử lý vẫn do identity.AdvanceWorkingHours đếm bằng
    -- giờ làm việc và không đi qua dòng này.
    --
    -- ĐÃ ĐO, KHÔNG PHỎNG ĐOÁN, và kết quả ngược với điều dễ tưởng: hôm nay dấu trên KHÔNG phải
    -- thứ cho dòng `INTERVAL '7 days'` đi qua `citizen_commitment_guard`. Gỡ dấu đi thì rào vẫn
    -- im, vì nó chỉ soi một dòng `INTERVAL … days` khi cửa sổ hai dòng quanh đó có chữ thuộc
    -- ngữ cảnh hạn (`sla`, `han_xu_ly`, `thoi_han`…) — mà khối này nói về `goc_dem_han` và
    -- `vao_so_luc`. Nói ra vì một dấu được tin là đang canh trong khi nó đã chết là đúng hạng
    -- lỗi kho này gặp lại nhiều lần nhất. Dấu vẫn giữ: nó ghi VÌ SAO ngày lịch là đúng ở đây, và
    -- nó là thứ có hiệu lực thật vào ngày rào được nới rộng.
    CONSTRAINT phieu_phan_anh_goc_dem_trong_khoang CHECK (
        goc_dem_han <= vao_so_luc AND goc_dem_han >= vao_so_luc - INTERVAL '7 days'),

    CONSTRAINT phieu_phan_anh_diem_hai_long_hop_le
        CHECK (diem_hai_long IS NULL OR diem_hai_long BETWEEN 1 AND 5),

    CONSTRAINT phieu_phan_anh_so_lan_mo_lai_khong_am
        CHECK (so_lan_mo_lai >= 0)
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS phieu_phan_anh_p%s PARTITION OF phieu_phan_anh '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

-- The register screen: one commune's petitions, newest first, soft-deleted rows excluded
-- EVERYWHERE, ALWAYS (rule 7, invariant 2).
CREATE INDEX IF NOT EXISTS phieu_phan_anh_so
    ON phieu_phan_anh (tenant_id, vao_so_luc DESC) WHERE deleted_at IS NULL;

-- The "đang trễ hạn" and "sắp đến hạn" screens. OVERDUE IS STILL DERIVED — this index holds the
-- deadline, never a verdict about it. Rows with no resolve deadline are OUT of the index for the
-- same reason they are out of the report: they have no commitment to compare with.
CREATE INDEX IF NOT EXISTS phieu_phan_anh_han_xu_ly
    ON phieu_phan_anh (tenant_id, han_xu_ly_xong)
    WHERE deleted_at IS NULL AND han_xu_ly_xong IS NOT NULL;

-- The classification queue: what has come in and nobody has read yet. It is the one thing
-- watching the gap ADR 0028 §Ba cái giá (a) describes.
CREATE INDEX IF NOT EXISTS phieu_phan_anh_cho_phan_loai
    ON phieu_phan_anh (tenant_id, han_tiep_nhan)
    WHERE deleted_at IS NULL AND trang_thai = 'da-tiep-nhan';

DROP TRIGGER IF EXISTS phieu_phan_anh_luu_tru ON phieu_phan_anh;
CREATE TRIGGER phieu_phan_anh_luu_tru
    BEFORE UPDATE OR DELETE ON phieu_phan_anh
    FOR EACH ROW EXECUTE FUNCTION ho_so_luu_tru_bat_bien();

-- ---------------------------------------------------------------------------
-- WHAT THIS FILE DOES NOT STOP, said plainly rather than left to be discovered: TRUNCATE, and
-- DDL by the table owner (ALTER TABLE ... DISABLE TRIGGER, dropping the table). Same line
-- ADR 0013 draws for the audit ledger — the job is to make the accidental and the convenient
-- impossible, not to defeat an administrator who has decided to destroy data and is willing to
-- be seen doing it.
--
-- REVERSAL (migration question 3). Every object here is new, and while the tables are still
-- empty the reversal is complete and loses nothing: drop the two tables, then the trigger
-- function, and in the same transaction remove this file's row from `schema_migration`. The 32
-- partitions and the triggers go with their parent tables.
--
-- ONCE A COMMUNE HAS A PETITION HERE, THAT IS NO LONGER A REVERSAL — it is the destruction of
-- archival records with a statutory retention period, which is rule 7's first stop condition and
-- needs the user, not a command. From that point the way back is a NEW migration.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- BACKSTOP: every partitioned table in this schema must actually have partitions.
--
-- A table declared PARTITION BY and given no partitions REJECTS EVERY INSERT, silently, until
-- the first real write — and because the audit entry shares the business transaction (rule 6,
-- invariant 3), that first write rolls back entirely. In this service the first write is a clerk
-- taking a citizen's report.
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
