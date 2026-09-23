-- petitions — MEETING MINUTES (`bien_ban_hop`) and the numbered CONCLUSIONS inside them
-- (`ket_luan_hop`): the bridge between a meeting and the work it produces.
--
-- `docs/ui-ux/04-bien-ban-hop.md:9` states the whole purpose in the interface's own words: "Nhập
-- một biên bản, tách thành nhiều nhiệm vụ. Mỗi nhiệm vụ giữ liên kết ngược về kết luận gốc để truy
-- vết được về sau."
--
-- WHY THESE TWO TABLES ARE IN `petitions` AND NOT IN `documents`. Both readings stand on their own
-- — a conclusion becomes a TASK (here), minutes are a DOCUMENT the office types (there) — and
-- `kb/30-indexes/data-ownership.json` had no entry either way. The tie is broken by EVIDENCE rather
-- than by preference: the running sibling implementation declares `meetings` and `conclusions` in
-- `apps/api/app/modules/tasks/models.py` — the SAME MODULE as its task register, not its document
-- module. `kb/90-ephemeral/ke-hoach-so-tay-bien-ban.md` §S1 records that reading and the project
-- owner's instruction of 2026-09-23 to consult that repository where this one has no answer.
--
-- THE CONSEQUENCE IS WHAT MAKES IT THE RIGHT ANSWER, not the vote count: the back-link from a task
-- to its conclusion stays INSIDE ONE SERVICE. Across services it would be rule 2, forbidden #2 —
-- and the "x/y nhiệm vụ đã hoàn thành" badge of §2 would need a cross-service call per card.
--
-- WHY A NEW FILE AND NOT AN EDIT OF 0006: 0006 has been applied and core/migrate compares the
-- checksum of every applied file at startup. Editing an applied file either stops the service
-- (ErrChecksumLech) or leaves two databases claiming one schema version while holding two different
-- schemas.
--
-- ENTITY OWNERSHIP is declared with the `-- @entity` / `-- @scope` marks above each CREATE TABLE
-- (ADR 0021).
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS, answered before the SQL.
--
--   1. HOW MANY ROWS PER COMMUNE: zero. Both tables are new and this file writes no row into
--      either. There is no seed — §8's sample minutes name ONE commune's meeting, and a row here
--      carries `tenant_id`, so seeding would mean seeding for one named commune. The step that sows
--      a commune's first rows (onboarding) does not exist in this repository.
--   2. IF IT STOPS HALF-WAY: it cannot land half-applied. One file, one transaction, with the
--      progress row written inside it. Every statement is IF NOT EXISTS or CREATE OR REPLACE, so a
--      retry costs nothing.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. WHICH READ PATHS CHANGE MEANING WHILE IT IS HALF-APPLIED — the question that is usually
--      skipped and usually causes the incident. Answer: NONE, and it is worth saying why rather
--      than asserting it. This file adds two tables, one trigger function, three triggers and ONE
--      INDEX ON AN EXISTING TABLE (`nhiem_vu_nguon`, below). It alters no column, no constraint and
--      no existing index. The index changes which PLAN the planner picks and never which rows come
--      back. `nhiem_vu.nguon_giao` / `nguon_id` already exist and are already readable: a task
--      created before this file with `nguon_giao = 'ket-luan-hop'` points at a conclusion that has
--      no row yet, and 0006:347-353 already wrote down what that costs — it is counted SHORT by
--      §11.7's badge, not an error. That state is unchanged by this file and ends when the write
--      path exists.
--   5. RETENTION: minutes and their conclusions are ADMINISTRATIVE RECORDS (rule 7). §7.1 says so
--      in the specification's own words — deleting a conclusion that has already produced tasks is
--      "không xoá cứng, cảnh báo và giữ liên kết", because a task must stay traceable to what it
--      came from. Soft delete only, and hard DELETE is refused by a trigger rather than by
--      convention.
--
-- ---------------------------------------------------------------------------
-- NO `ket_luan_id` COLUMN IS ADDED TO `nhiem_vu`, AND THAT IS THE MAIN SCHEMA DECISION OF THIS
-- FILE — read this before "completing" the link with a column of its own.
--
-- The back-link already exists. 0006:246-258 declared the BLURRED PAIR `nguon_giao` + `nguon_id`,
-- with `'ket-luan-hop'` among the four codes of `nhiem_vu_nguon_giao_hop_le` and the id of the
-- originating record in `nguon_id`. §5 of the specification asks for exactly that shape:
-- "`nhiem_vu.nguon_giao = 'ket-luan-hop'` và `nhiem_vu.nguon_id = ket_luan_hop.id`", and §5's own
-- counting query is written against those two columns.
--
-- A DEDICATED `ket_luan_id` WOULD BE A SECOND REPRESENTATION OF ONE FACT (rule 9, the one-line
-- test: a tool can rebuild it from `nguon_giao`/`nguon_id`, so writing it by hand is forbidden).
-- The two copies would not stay in step, and the one that drifts is the one a screen reads. It
-- would also be the wrong shape twice over: chapter 05 needs the same link for `van-ban-den`, and
-- a column per source register means a new column — a migration on archival records — for every
-- integration.
--
-- THE PAIR'S KNOWN COST, RESTATED SO IT IS NOT DISCOVERED LATER: there is no foreign key from
-- `nhiem_vu.nguon_id` to `ket_luan_hop.id`, and there cannot be one — the same column also holds
-- `don_thu.id` (another service) and `phieu_phan_anh.id`. So a conclusion cannot learn from the
-- database that tasks point at it; the counting query is what finds them, and a task whose
-- `nguon_id` names a conclusion that was never created counts SHORT rather than failing.
--
-- ---------------------------------------------------------------------------
-- WHAT THIS FILE DELIBERATELY DOES NOT CREATE, so the absence is not read as unfinished work:
--
--   * NO IMMUTABILITY TRIGGER ON `ket_luan_hop.noi_dung`, `thu_tu` OR `bien_ban_id`. Whether a
--     conclusion may still be EDITED once tasks have been split from it is a STOP CONDITION, not a
--     column: §1 says the back-link exists "để truy vết được về sau", and editing the text of a
--     conclusion changes the thing a task is pointing at. §7.1 answers the DELETE half only. Both
--     possible answers are defensible — a clerk's typo must be fixable; an approved conclusion a
--     task was born from must not silently become a different sentence — and the schema takes
--     NEITHER side today. That is affordable precisely because this pass builds no write route:
--     nothing can edit a conclusion at all, so no default has been baked in. When the answer
--     arrives it is one trigger in a LATER migration, over empty tables.
--
--   * NO UNIQUE KEY ON `so_hieu` (`31/BB-UBND`). It looks like an issued number and is not one in
--     the sense rule 7, invariant 3 means: it is typed by hand, it is OPTIONAL (§4 marks it not
--     required, and the prototype's draft carries `12/233`), and Vietnamese reference numbers
--     restart each year — next year's 31st minutes are `31/BB-UBND` again. A unique key here would
--     refuse REAL configuration, which is the lesson written into
--     service-identity/migrations/0008_sla.sql:237-242 and paid for again in this repository since.
--
--   * NO `so_ket_luan` / `so_nhiem_vu` / `so_nhiem_vu_xong` COUNTER COLUMN on `bien_ban_hop`. The
--     badge of §2 (`{n} kết luận · {x}/{y} nhiệm vụ xong`) is DERIVED by counting, exactly as §5
--     writes it. A stored counter is wrong from the moment a task is soft-deleted or a status
--     changes, it cannot be fixed by a screen, and the stale copy is the one that reaches a report
--     — the same reasoning rule 10, invariant 3 applies to `qua_han`.
--
--   * NO INDEX ON `ngay_hop`. Nothing sorts or filters by it yet: the register list pages on
--     `tao_luc` (see `bien_ban_hop_so` below and the note on the sort in internal/store). An index
--     nothing reads is a write cost with no reader and a reader's false assurance that some query
--     is cheap.
--
--   * NO TABLE FOR ATTENDEES. §4's "Thành phần tham dự" is "multi-select cán bộ / text" — a list
--     with no stated structure and no screen that queries ACROSS meetings by attendee. It is held
--     as JSONB until something needs to be asked of it; a join table invented now would fix a shape
--     nobody has chosen.

-- ---------------------------------------------------------------------------
-- PostgreSQL 13 is the floor, checked explicitly so the failure is readable — the same check and
-- the same reason as 0006: BEFORE ... FOR EACH ROW triggers on a PARTITIONED table were only
-- allowed from PostgreSQL 13, and on 11/12 the CREATE TRIGGER statements below fail with a message
-- that reads like a syntax mistake and invites somebody to move the trigger down onto the
-- partitions — where a partition added later arrives silently unprotected.
-- ---------------------------------------------------------------------------
DO $$ BEGIN
    IF current_setting('server_version_num')::int < 130000 THEN
        RAISE EXCEPTION
            'bien_ban_hop needs PostgreSQL 13 or newer (server is %). Do not weaken this migration '
            'to fit an older server — the trigger below is what stops a meeting conclusion a task '
            'was born from being destroyed with one statement.',
            current_setting('server_version');
    END IF;
END $$;

-- ---------------------------------------------------------------------------
-- ho_so_luu_tru_cam_xoa_cung — one function, attached to every archival table it fits.
--
-- It refuses a hard removal and NOTHING ELSE, so it can be attached to a table whatever its columns
-- are. Rule 7, forbidden #1: business data is soft deleted (deleted_at, deleted_by, delete_reason).
--
-- IT IS NOT `ho_so_luu_tru_bat_bien` (0004) AND NOT `nhiem_vu_bat_bien` (0006), although all three
-- guard archival tables in this schema. Those two read columns of the register they belong to
-- (`ma_tra_cuu`, `goc_dem_han`, `kenh_tiep_nhan`; `ma`, `han_ban_dau`), and attaching either here
-- would fail at runtime on the first UPDATE — inside the business transaction, which, because the
-- audit entry shares it (rule 6, invariant 3), rolls the whole act back.
--
-- THE NAME IS THE ONE service-documents AND service-finance ALREADY USE for this exact job
-- (`0004` in both). Three services, three schemas, one name — a reader who has met it once does not
-- have to read it twice. This is its first copy in `petitions`.
--
-- ENFORCED IN THE DATABASE, NOT IN THE APPLICATION, for the reason ADR 0013 gives for the audit
-- ledger: a promise the application layer makes is a promise a migration script, a psql session or
-- the next service to connect never heard.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION ho_so_luu_tru_cam_xoa_cung() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'archival table %: hard delete refused', TG_TABLE_NAME
        USING HINT = 'Meeting minutes and their conclusions are archival records (rule 7, invariant '
                     '1), and a task carries a permanent back-link to the conclusion it came from '
                     '(docs/ui-ux/04-bien-ban-hop.md §7.1). Soft delete instead: set deleted_at, '
                     'deleted_by and delete_reason.';
END $$;

-- ---------------------------------------------------------------------------
-- @entity: Meeting
-- @scope:  tenant
--
-- bien_ban_hop — the minutes of ONE meeting (§4, §5).
--
-- IT HOLDS NO CITIZEN PERSONAL DATA BY DESIGN, and one column has to be watched: `noi_dung` is the
-- full text of the minutes as typed, and a commune's minutes can quote a case. Nothing here may be
-- logged, put in a file name, a URL or a cache key (rule 3, forbidden #1 and #4). The people named
-- on this record are MEMBERS OF STAFF, by business code.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS bien_ban_hop (
    tenant_id     TEXT        NOT NULL,
    id            TEXT        NOT NULL,

    -- "Tên cuộc hop" (§4) — `Giao ban Uỷ ban nhân dân xã tháng 8 năm 2026`. Required: a card with
    -- no title is a card nobody can pick out of the list.
    ten_cuoc_hop  TEXT        NOT NULL,

    -- WHEN THE MEETING WAS HELD. `DATE` AND NOT `TIMESTAMPTZ`, and that is the opposite choice from
    -- 0006's two deadline columns — deliberately, because the two are different KINDS of fact:
    --
    --   a deadline    is arithmetic in WORKING HOURS (rule 10, invariant 4; ADR 0007). A day-
    --                 resolution column silently rounds a commitment, so 0006 diverged from its
    --                 specification and stored an instant.
    --   a meeting day is a calendar fact the form collects as a date (§4: "Ngày họp | date") and
    --                 the card renders as `5/8/2026`. Nothing counts from it. Storing an instant
    --                 would force the write path to invent a time of day nobody recorded, and
    --                 "midnight in which timezone" would decide which DAY the card shows.
    --
    -- The same choice service-documents made for `ngay_den` / `ngay_van_ban` and service-finance for
    -- `ngay_chi`. Required, as §4 marks it.
    ngay_hop      DATE        NOT NULL,

    -- The reference number of the written minutes, `31/BB-UBND` (§4, optional). NOT UNIQUE and not
    -- an issued number in this system's sense — see the note at the top of this file.
    so_hieu       TEXT,

    dia_diem      TEXT,

    -- WHO CHAIRED IT, as a STAFF BUSINESS CODE (`CB-2026-7K3M9Q`) — rule 6, invariant 8, and the
    -- column name says so. §5 models it as `chu_tri_id uuid`; THE SPECIFICATION LOSES TO THE RULE
    -- here, exactly as 0006 decided for `nguoi_thuc_hien_ma` and service-documents before it. A
    -- column holding both kinds of identifier is a column nobody can query, and the two are
    -- indistinguishable on sight.
    chu_tri_ma    TEXT,

    -- "Thành phần tham dự" (§4) — protojson-shaped, staff codes and/or free text. DEFAULT '[]' so a
    -- reader never has to tell "nobody recorded the attendees" from "column not set".
    --
    -- ⚠ IT IS NOT A PLACE FOR CITIZEN DETAILS. A meeting's attendee list is staff; the day somebody
    -- writes a reporter's name and number in here, this table has become a personal-data store
    -- (rule 3).
    thanh_phan    JSONB       NOT NULL DEFAULT '[]'::jsonb,

    -- The minutes in full (§4, optional: "nhập nháp trước, bổ sung sau"). Read the personal-data
    -- warning on this table before putting this column anywhere but a screen.
    noi_dung      TEXT,

    -- The scan of the signed minutes (§4). Nothing writes it today — there is no file store in this
    -- repository yet — and it defaults to an empty array for the same reason `thanh_phan` does.
    --
    -- ⚠ A FILE NAME CHOSEN BY A CITIZEN OR DESCRIBING A CASE MAY NOT GO IN IT (rule 3, forbidden #4).
    dinh_kem      JSONB       NOT NULL DEFAULT '[]'::jsonb,

    -- Who typed the minutes, as a staff business code. §4's screen is the office's.
    nguoi_tao_ma  TEXT        NOT NULL,

    deleted_at    TIMESTAMPTZ,
    deleted_by    TEXT,
    delete_reason TEXT,
    tao_luc       TIMESTAMPTZ NOT NULL DEFAULT now(),
    cap_nhat_luc  TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (tenant_id, id),

    CONSTRAINT bien_ban_hop_ten_khong_rong CHECK (btrim(ten_cuoc_hop) <> ''),

    -- A SOFT DELETE CARRIES WHO AND WHY, OR IT IS NOT ONE (rule 7, invariant 1). Same shape as every
    -- other register in this service.
    CONSTRAINT bien_ban_hop_xoa_mem_day_du
        CHECK ((deleted_at IS NULL AND deleted_by IS NULL AND delete_reason IS NULL)
            OR (deleted_at IS NOT NULL AND deleted_by IS NOT NULL AND delete_reason IS NOT NULL))
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS bien_ban_hop_p%s PARTITION OF bien_ban_hop '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

-- The page of cards, newest first, and the tie-break the cursor pages on (core/store.QueryPage
-- orders by `(sort, id)`). Soft-deleted rows excluded EVERYWHERE, ALWAYS (rule 7, invariant 2).
--
-- ON `tao_luc` AND NOT ON `ngay_hop`, although §2 says "mới nhất ở trên" about the meetings: a
-- cursor column must be NOT NULL and must compare identically in Go and in SQL, and a DATE column
-- compared against a bound `time.Time` is cast through the session's time zone. `tao_luc` is the
-- same key every other register in this repository pages on.
CREATE INDEX IF NOT EXISTS bien_ban_hop_so
    ON bien_ban_hop (tenant_id, tao_luc DESC, id) WHERE deleted_at IS NULL;

DROP TRIGGER IF EXISTS bien_ban_hop_cam_xoa_cung ON bien_ban_hop;
CREATE TRIGGER bien_ban_hop_cam_xoa_cung
    BEFORE DELETE ON bien_ban_hop
    FOR EACH ROW EXECUTE FUNCTION ho_so_luu_tru_cam_xoa_cung();

-- ---------------------------------------------------------------------------
-- @entity: MeetingConclusion
-- @scope:  tenant
--
-- ket_luan_hop — ONE numbered conclusion inside a meeting's minutes (§2, §5).
--
-- WHY CONCLUSIONS ARE ROWS AND NOT A JSONB ARRAY ON THE MINUTES: a task points at ONE of them
-- (`nhiem_vu.nguon_id`), and §2 counts "x/y nhiệm vụ đã hoàn thành" PER CONCLUSION. An element of a
-- JSON array has no stable identity to point at and nothing to count against.
--
-- ⚠ `noi_dung` IS FREE TEXT A CLERK TYPES, and a conclusion routinely quotes a case ("rà soát tiến
-- độ tuyến đường Hà Lam – Bình Trị"). It is staff-facing business text, and it must not travel into
-- a log line, an error message returned to a client, or a file name (rule 3).
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ket_luan_hop (
    tenant_id     TEXT        NOT NULL,
    id            TEXT        NOT NULL,

    -- The minutes this conclusion belongs to. A REAL FOREIGN KEY, unlike `nhiem_vu.nguon_id`, and
    -- the difference is that this one names exactly one table in this service's own schema — it
    -- crosses no service boundary and has no second meaning. Composite with `tenant_id`, so it
    -- cannot reach another commune's minutes even if two ids ever collided.
    bien_ban_id   TEXT        NOT NULL,

    -- The number in the circle, ① ② ③ (§2). §7.2: "đánh số liên tục từ 1 trong phạm vi một biên
    -- bản; thêm mới thì nối tiếp" — so the write path mints it as max+1 within the meeting, against
    -- the unique key below, which is what actually guarantees it.
    thu_tu        INT         NOT NULL,

    noi_dung      TEXT        NOT NULL,

    deleted_at    TIMESTAMPTZ,
    deleted_by    TEXT,
    delete_reason TEXT,
    tao_luc       TIMESTAMPTZ NOT NULL DEFAULT now(),
    cap_nhat_luc  TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (tenant_id, id),

    -- COMPOSITE WITH tenant_id (rule 1, invariant 6) AND COUNTING SOFT-DELETED ROWS.
    --
    -- NOT `... WHERE deleted_at IS NULL`, and the omission is the point: a partial unique key would
    -- let a commune soft-delete conclusion ② and mint a second ② — while the first one's tasks are
    -- still pointing at it and the printed minutes still say "②". §7.2 says new conclusions are
    -- APPENDED, never renumbered, which is rule 7's invariant 3 in this table's own words. A gap
    -- after a removal is the correct outcome. tools/check_khoa_duy_nhat.py refuses the partial form
    -- for exactly this reason.
    UNIQUE (tenant_id, bien_ban_id, thu_tu),

    -- SAME SERVICE, SAME SCHEMA — the key crosses nothing (rule 2, forbidden #2 does not apply).
    -- Both sides are partitioned by hash on `tenant_id` and the referenced pair is the parent's
    -- primary key, which is the shape `nhiem_vu` -> `loai_nhiem_vu` already proved out in 0006.
    FOREIGN KEY (tenant_id, bien_ban_id) REFERENCES bien_ban_hop (tenant_id, id),

    -- ① IS THE FIRST NUMBER. A zero or a negative ordinal would render as a circle nobody can read
    -- and would break "đánh số liên tục từ 1".
    CONSTRAINT ket_luan_hop_thu_tu_tu_mot CHECK (thu_tu >= 1),

    CONSTRAINT ket_luan_hop_noi_dung_khong_rong CHECK (btrim(noi_dung) <> ''),

    CONSTRAINT ket_luan_hop_xoa_mem_day_du
        CHECK ((deleted_at IS NULL AND deleted_by IS NULL AND delete_reason IS NULL)
            OR (deleted_at IS NOT NULL AND deleted_by IS NOT NULL AND delete_reason IS NOT NULL))
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS ket_luan_hop_p%s PARTITION OF ket_luan_hop '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

-- One card's conclusions, in the order the circles are drawn (§2). The read path fetches them for a
-- whole page of cards at once, so this index carries the meeting id AND the ordinal.
CREATE INDEX IF NOT EXISTS ket_luan_theo_bien_ban
    ON ket_luan_hop (tenant_id, bien_ban_id, thu_tu) WHERE deleted_at IS NULL;

DROP TRIGGER IF EXISTS ket_luan_hop_cam_xoa_cung ON ket_luan_hop;
CREATE TRIGGER ket_luan_hop_cam_xoa_cung
    BEFORE DELETE ON ket_luan_hop
    FOR EACH ROW EXECUTE FUNCTION ho_so_luu_tru_cam_xoa_cung();

-- ---------------------------------------------------------------------------
-- THE ONE CHANGE THIS FILE MAKES TO AN EXISTING TABLE, and it adds no column and alters no row.
--
-- `nhiem_vu_nguon` is what makes §2's badge affordable. The counting query of §5 asks `WHERE
-- nguon_giao = 'ket-luan-hop' AND nguon_id = :ket_luan_id`, ONCE PER CONCLUSION on a page of cards;
-- 0006 created five indexes on `nhiem_vu` and none of them covers that predicate, so every badge
-- would scan the commune's whole register. It is cheap today (every register is empty) and
-- unaffordable to add on the day a commune notices — which is the day the register is large.
--
-- IT ALSO SERVES §7.4's FILTER, `Nguồn giao = Từ kết luận họp`, on the task register itself.
--
-- PARTIAL ON TWO PREDICATES, AND NEITHER IS THE UNIQUE-KEY TRAP: this is a PLAIN index, so
-- `WHERE deleted_at IS NULL` here frees no issued number and reissues nothing — it only keeps
-- deleted rows out of a read that must exclude them anyway (rule 7, invariant 2). `nguon_id IS NOT
-- NULL` drops every directly-assigned task, which is most of them and none of which this query can
-- ever match (0006's `nhiem_vu_truc_tiep_khong_co_nguon`).
--
-- IT STARTS WITH `tenant_id`, like every index in this system.
-- ---------------------------------------------------------------------------
CREATE INDEX IF NOT EXISTS nhiem_vu_nguon
    ON nhiem_vu (tenant_id, nguon_giao, nguon_id)
    WHERE deleted_at IS NULL AND nguon_id IS NOT NULL;

-- ---------------------------------------------------------------------------
-- WHAT THIS FILE DOES NOT STOP, said plainly rather than left to be discovered: TRUNCATE on either
-- table, and DDL by the table owner (ALTER TABLE ... DISABLE TRIGGER, dropping a table). Same line
-- ADR 0013 draws for the audit ledger — the job is to make the accidental and the convenient
-- impossible, not to defeat an administrator who has decided to destroy data and is willing to be
-- seen doing it.
--
-- REVERSAL (migration question 3). Every object here is new, and while the two tables are still
-- empty — which they are in every environment today — the reversal is complete and loses nothing:
--
--   DROP TABLE ket_luan_hop;               -- its 32 partitions and its trigger go with it
--   DROP TABLE bien_ban_hop;               -- same; drop this one SECOND, the FK points at it
--   DROP FUNCTION ho_so_luu_tru_cam_xoa_cung();
--   DROP INDEX nhiem_vu_nguon;             -- the only object this file put on an existing table
--
-- and in the SAME transaction remove this file's row from `schema_migration`, otherwise the runner
-- still believes the schema is in place. Nothing in 0006 is altered by this file, so nothing there
-- has to be put back — dropping `nhiem_vu_nguon` restores 0006's exact set of five indexes.
--
-- ONCE A COMMUNE HAS ONE MEETING RECORDED HERE, THAT IS NO LONGER A REVERSAL — it is the
-- destruction of administrative records, AND it orphans the back-link of every task split from
-- those conclusions. That is rule 7's first stop condition and needs the user, not a command. From
-- that point the way back is a NEW migration, and core/migrate has no automatic rollback for
-- exactly this reason (ADR 0013).
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- BACKSTOP: every partitioned table in this schema must actually have partitions.
--
-- A table declared PARTITION BY and given no partitions REJECTS EVERY INSERT, silently, until the
-- first real write — and because the audit entry shares the business transaction (rule 6, invariant
-- 3), that first write rolls back entirely. The check is repeated at the end of every migration
-- that declares a partitioned table, because it only verifies the state after a file that CARRIES
-- it (0002, §BACKSTOP).
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
