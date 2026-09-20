-- identity — the commune's working calendar: the ordinary week, public holidays, swap days.
--
-- WHY A NEW FILE AND NOT AN EDIT OF AN EARLIER ONE: 0001–0005 have been applied and
-- core/migrate compares the checksum of every applied file at startup. Editing an applied file
-- either stops the service (ErrChecksumLech) or leaves two databases claiming one schema
-- version while holding two different schemas.
--
-- WHY THREE TABLES IN ONE FILE: core/migrate gives each file exactly ONE transaction, and the
-- three answer one question together — "was this authority working at this instant". A database
-- holding the weekly calendar but not yet the holidays would answer that question WRONGLY
-- rather than refusing to answer it, and a deadline computed from a wrong answer is a
-- commitment made to a citizen on a false basis (rule 10). All three, or none.
--
-- WHY IN `identity` AND NOT IN `petitions` — decided by the user, 2026-09-20. ADR 0007's own
-- SLA table counts deadlines for `Văn bản đến` as well as for `Phản ánh`, so the calendar is
-- read by at least two services and owned by neither of them. ADR 0024 settles it: a NAME
-- follows the concept, OWNERSHIP follows the CADENCE OF CHANGE. A commune's office hours change
-- with its administration — alongside `bo_phan`, `vai_tro` and `thon_to_dan_pho`, which already
-- live here — not with a petition. `khoi_nhiem_vu` is in this service for the same reason,
-- despite carrying `Task` in its name.
--
-- ENTITY OWNERSHIP is declared with the `-- @entity` / `-- @scope` marks above each CREATE
-- TABLE (ADR 0021). The generator that reads them does not exist yet; the marks are written now
-- because the moment a table is born is the only moment the answer is certain.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS, answered before the SQL.
--
--   1. HOW MANY ROWS PER COMMUNE: zero. All three tables are new and this file writes no row
--      into any of them — see "NOTHING IS SEEDED" below. The ceiling in sight is ~10 weekly
--      sessions, ~15 public holidays and a handful of swap days per commune per year.
--   2. IF IT STOPS HALF-WAY: it cannot land half-applied. One file, one transaction, with the
--      progress row written inside it. A failure leaves nothing behind and the next start
--      retries from the beginning; every statement is IF NOT EXISTS, so a retry costs nothing.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. WHICH READ PATHS CHANGE MEANING WHILE IT IS HALF-APPLIED: none. This file only ADDS
--      tables and indexes. No existing table, column, index or constraint is touched, so every
--      query that runs today returns exactly what it returned before and no row changes
--      visibility. The risk moves to the NEXT change — the first one that writes rows here,
--      and the first deadline computed from them.
--   5. RETENTION: nothing is removed. A working calendar is the BASIS OF AN ISSUED COMMITMENT:
--      when an inspection asks why a petition received on 30/04 was due on 05/05, the answer is
--      the calendar as it stood that day. So the soft-delete columns are present from the start
--      and a row is never hard-deleted — rule 7, and rule 10 invariant 2, which fixes a deadline
--      at intake and forbids recomputing it later.
--
-- ---------------------------------------------------------------------------
-- NOTHING IS SEEDED, AND THAT IS AN ANSWER RATHER THAN A GAP.
--
-- The seed row would carry `tenant_id`, so it would have to belong to a specific commune —
-- while the migration path has no commune in it and deliberately accepts none (ADR 0013).
-- Going to fetch the list of communes would mean reading `platform`'s registry from another
-- service's migration, which rule 2 forbidden #2 closes. Same answer as the catalogues in
-- `fa10cf1`: the first rows are sown by the commune-onboarding step, which does not exist yet.
--
-- WHAT AN EMPTY CALENDAR MEANS, and it has to be decided by the code that reads it, not here:
-- a commune with no `lich_lam_viec` row has NO working hours at all, so a deadline counted in
-- working hours never arrives. The reader must REFUSE to compute a deadline against an empty
-- calendar rather than fall back to "24 hours" or "Mon–Fri 08:00–17:00" — a silent default here
-- is a commitment invented by software and told to a citizen (rule 10, and the fail-closed
-- principle CLAUDE.md opens with).
--
-- ---------------------------------------------------------------------------
-- FIVE QUESTIONS ADR 0007 LEFT OPEN, AND WHERE EACH ONE LANDS IN THIS SCHEMA.
--
-- ADR 0007 §"Lỗ hổng đặc tả" lists five things nobody has answered. None of them is answered
-- here — a schema must not decide a commune's administrative practice. What this shape
-- guarantees is that every possible answer is EXPRESSIBLE without a second migration:
--
--   "Giờ hành chính của xã là mấy giờ tới mấy giờ?"  -> rows in lich_lam_viec
--   "Nghỉ trưa có trừ không?"                        -> TWO rows for that weekday, with a gap
--   "Thứ Bảy có làm không?"                          -> a row for thu = 6, or none
--   "Lễ địa phương có tính không?"                   -> rows in ngay_nghi_le, per commune
--   "Hồ sơ ngoài giờ đếm từ lúc nào?"                -> NOT a schema question. It is a rule in
--                                                       the deadline function, and ADR 0007
--                                                       proposes "from the start of the next
--                                                       working session" without deciding it
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- lich_lam_viec — the commune's ordinary week, one row per WORKING SESSION.
--
-- ONE ROW IS A SESSION, NOT A DAY, AND THAT IS THE DECISION THIS TABLE TURNS ON. A single
-- (start, end) pair per weekday cannot express a lunch break, and a lunch break is 1–1.5 hours
-- a day — over a 40-hour deadline that is a full working day of drift, in the direction that
-- makes the authority look late when it was not, or on time when it was not.
--
-- With sessions, every shape a commune can actually have is a row count:
--
--   Mon–Fri, 07:30–11:30 and 13:30–17:00   ten rows
--   Saturday morning duty, 07:30–11:30      one more row, thu = 6
--   A day the commune does not work         NO ROW for that weekday
--
-- Nothing here needs a "works on Saturday" flag, a "has lunch break" flag, or a half-day type.
-- Each of those would be a second way to say something the rows already say, and two ways to
-- say one thing is the drift rule 9 exists to prevent.
--
-- `thu` IS ISO 8601 WEEKDAY: 1 = Monday … 7 = Sunday. NOT the Vietnamese ordinal, where "thứ
-- Hai" is Monday and Sunday is "Chủ nhật" with no number. PostgreSQL's EXTRACT(ISODOW FROM d)
-- returns exactly this range, so the stored value and the computed one compare directly. Any
-- other convention needs a conversion at every single use, and a weekday conversion is where an
-- off-by-one lives for a year before anybody notices the deadline is a day out.
--
-- TIME WITHOUT TIME ZONE, on purpose. "07:30" is a wall-clock instruction to a commune's staff,
-- not an instant. The instant is produced by combining it with a DATE in Asia/Ho_Chi_Minh, once,
-- in the deadline function. Storing TIMETZ would attach an offset to a rule that has none, and
-- the offset would be whatever the writing session happened to hold.
--
-- NO OVERLAP CONSTRAINT, AND THE REASON IS A DEPENDENCY, NOT AN OVERSIGHT. Two overlapping
-- sessions on one weekday would double-count those hours. The database-level answer is an
-- EXCLUDE constraint, which needs the `btree_gist` extension — and a migration that fails
-- because an extension is unavailable does not degrade, it STOPS THE SERVICE (ADR 0013). This
-- file will not put the whole service's startup behind an extension nobody has confirmed is
-- present in the managed PostgreSQL. So: UNIQUE keeps two sessions from starting at the same
-- minute, and the write path — which does not exist yet — must reject an overlap and carry a
-- test for it. Written down so the next person meets the obligation rather than the silence.
-- ---------------------------------------------------------------------------
-- @entity: WorkingHours
-- @scope:  tenant
CREATE TABLE IF NOT EXISTS lich_lam_viec (
    tenant_id     TEXT        NOT NULL,
    id            TEXT        NOT NULL,

    -- ISO 8601: 1 = Monday … 7 = Sunday. See the block above before changing this.
    thu           SMALLINT    NOT NULL,
    bat_dau       TIME        NOT NULL,
    ket_thuc      TIME        NOT NULL,

    -- Free text the commune writes for itself — "Buổi sáng", "Ca trực thứ Bảy". Never parsed,
    -- never matched on: it is a label for a person reading the configuration screen.
    ghi_chu       TEXT        NOT NULL DEFAULT '',

    deleted_at    TIMESTAMPTZ,
    deleted_by    TEXT,
    delete_reason TEXT,
    tao_luc       TIMESTAMPTZ NOT NULL DEFAULT now(),
    cap_nhat_luc  TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- Composite with tenant_id (rule 1, invariant 6).
    PRIMARY KEY (tenant_id, id),

    CONSTRAINT lich_lam_viec_id_la_ulid CHECK (length(id) = 26),
    CONSTRAINT lich_lam_viec_thu_hop_le CHECK (thu BETWEEN 1 AND 7),
    -- A session that ends before it starts is not a short session, it is a typo that would make
    -- the hours count negative.
    CONSTRAINT lich_lam_viec_co_do_dai CHECK (ket_thuc > bat_dau),
    -- Two sessions cannot START at the same minute on the same weekday. This is weaker than
    -- "cannot overlap" — see the block above for why the stronger form is not here.
    CONSTRAINT lich_lam_viec_khong_trung_gio_mo UNIQUE (tenant_id, thu, bat_dau)
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS lich_lam_viec_p%s PARTITION OF lich_lam_viec '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

COMMENT ON TABLE lich_lam_viec IS
    'Tuan lam viec thong thuong cua xa. MOT DONG LA MOT CA LAM VIEC, khong phai mot ngay: '
    'nghi trua duoc dien dat bang hai dong co khoang ho o giua. Ngay khong co dong nao la ngay '
    'khong lam viec.';
COMMENT ON COLUMN lich_lam_viec.thu IS
    'ISO 8601: 1 = thu Hai ... 7 = Chu nhat. KHONG phai so thu tu tieng Viet. Khop truc tiep '
    'voi EXTRACT(ISODOW FROM ngay) cua PostgreSQL.';

-- The one query the deadline function runs, on every deadline it computes: this commune's
-- sessions for one weekday, in order.
CREATE INDEX IF NOT EXISTS lich_lam_viec_theo_thu
    ON lich_lam_viec (tenant_id, thu, bat_dau)
    WHERE deleted_at IS NULL;

-- ---------------------------------------------------------------------------
-- ngay_nghi_le — dates this commune does NOT work, whatever the weekly calendar says.
--
-- PER COMMUNE, INCLUDING NATIONAL HOLIDAYS, and that is ADR 0007 decision 4 rather than a
-- choice made here. The cost is real and should be visible to whoever pays it: Tết is stored
-- 200+ times, once per commune, and 200+ copies of one fact drift — one commune left without
-- the row counts a deadline through a national holiday, and the figure that reaches leadership
-- says the commune was late.
--
-- The alternative — a platform-level table of national holidays plus a per-commune table of
-- local ones — was not chosen, and the reason it is not reopened here: local festivals and
-- traditional days genuinely vary by commune, so BOTH tables would exist and every reader would
-- have to consult both and merge them. ADR 0007 took the simpler shape. What makes the cost
-- survivable is that the onboarding step sows the national list, so no human types it 200 times.
--
-- WHAT THIS TABLE IS NOT: it is not a calendar of events, not a list of days off for one member
-- of staff, and not a schedule. One row means "on this date this authority is closed".
-- ---------------------------------------------------------------------------
-- @entity: PublicHoliday
-- @scope:  tenant
CREATE TABLE IF NOT EXISTS ngay_nghi_le (
    tenant_id     TEXT        NOT NULL,
    id            TEXT        NOT NULL,

    ngay          DATE        NOT NULL,
    -- What a person reads: "Quốc khánh", "Giỗ Tổ Hùng Vương", "Lễ hội đình làng". Written by
    -- the commune, shown on the configuration screen, never matched on.
    ten           TEXT        NOT NULL,

    deleted_at    TIMESTAMPTZ,
    deleted_by    TEXT,
    delete_reason TEXT,
    tao_luc       TIMESTAMPTZ NOT NULL DEFAULT now(),
    cap_nhat_luc  TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (tenant_id, id),

    CONSTRAINT ngay_nghi_le_id_la_ulid CHECK (length(id) = 26),
    CONSTRAINT ngay_nghi_le_co_ten CHECK (btrim(ten) <> ''),
    -- One date, one row, per commune. A date listed twice would be counted once and look
    -- correct — the duplicate only ever surfaces as a confusing configuration screen.
    CONSTRAINT ngay_nghi_le_mot_dong_moi_ngay UNIQUE (tenant_id, ngay)
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS ngay_nghi_le_p%s PARTITION OF ngay_nghi_le '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

COMMENT ON TABLE ngay_nghi_le IS
    'Ngay xa KHONG lam viec, du lich tuan noi gi. Theo tung xa, gom ca le quoc gia lan le dia '
    'phuong (ADR 0007 quyet dinh 4).';

-- "Is this commune closed on this date" — asked once per day walked while counting a deadline.
CREATE INDEX IF NOT EXISTS ngay_nghi_le_theo_ngay
    ON ngay_nghi_le (tenant_id, ngay)
    WHERE deleted_at IS NULL;

-- ---------------------------------------------------------------------------
-- ngay_lam_bu — dates this commune DOES work although the weekly calendar says otherwise.
--
-- A THIRD TABLE ADR 0007 DOES NOT NAME, AND THE REASON IS THE VIETNAMESE WORKING YEAR RATHER
-- THAN A DESIGN PREFERENCE. Every year the Prime Minister announces the Tết and National Day
-- arrangements, and they routinely include làm bù: a Saturday that becomes a working day to
-- repay a long holiday run. Without this table the calendar is simply wrong for those dates —
-- every deadline crossing them is computed as if the commune were closed, and the error lands
-- on the days of the year when the backlog is largest.
--
-- IT CANNOT BE FOLDED INTO ngay_nghi_le WITH A `loai` COLUMN. A holiday is a date; a swap day
-- is a date PLUS the hours worked, because the weekday it falls on usually has no session rows
-- at all — a Saturday in a commune that does not work Saturdays. One table would then hold rows
-- where half the columns are meaningless, and a CHECK would have to keep the two halves apart.
-- Two tables, two shapes, each one always fully meaningful.
--
-- SESSIONS AGAIN, FOR THE SAME REASON AS lich_lam_viec: a swap day with a lunch break is two
-- rows. The shape matches so the deadline function reads both with the same logic.
--
-- PRECEDENCE, WHICH THE READER MUST IMPLEMENT AND THIS FILE CANNOT ENFORCE: a date present in
-- BOTH tables is a configuration error, not a puzzle to resolve with a rule. The function must
-- refuse rather than pick a winner — a silent precedence rule would make one of two visible
-- configuration rows do nothing, and nobody would ever see which.
-- ---------------------------------------------------------------------------
-- @entity: SwapWorkingDay
-- @scope:  tenant
CREATE TABLE IF NOT EXISTS ngay_lam_bu (
    tenant_id     TEXT        NOT NULL,
    id            TEXT        NOT NULL,

    ngay          DATE        NOT NULL,
    bat_dau       TIME        NOT NULL,
    ket_thuc      TIME        NOT NULL,
    -- "Làm bù nghỉ Tết theo Thông báo số …" — the announcement this row implements. Prose for a
    -- person; an inspection asking why a deadline ran through a Saturday reads this.
    ten           TEXT        NOT NULL,

    deleted_at    TIMESTAMPTZ,
    deleted_by    TEXT,
    delete_reason TEXT,
    tao_luc       TIMESTAMPTZ NOT NULL DEFAULT now(),
    cap_nhat_luc  TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (tenant_id, id),

    CONSTRAINT ngay_lam_bu_id_la_ulid CHECK (length(id) = 26),
    CONSTRAINT ngay_lam_bu_co_ten CHECK (btrim(ten) <> ''),
    CONSTRAINT ngay_lam_bu_co_do_dai CHECK (ket_thuc > bat_dau),
    CONSTRAINT ngay_lam_bu_khong_trung_gio_mo UNIQUE (tenant_id, ngay, bat_dau)
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS ngay_lam_bu_p%s PARTITION OF ngay_lam_bu '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

COMMENT ON TABLE ngay_lam_bu IS
    'Ngay xa CO lam viec du lich tuan noi khong — lam bu theo thong bao hang nam cua Thu tuong. '
    'Mang gio lam viec cua chinh ngay do, vi thu trong tuan cua no thuong khong co ca nao.';

CREATE INDEX IF NOT EXISTS ngay_lam_bu_theo_ngay
    ON ngay_lam_bu (tenant_id, ngay, bat_dau)
    WHERE deleted_at IS NULL;

-- ---------------------------------------------------------------------------
-- REVERSAL (rule 7, invariant 4).
--
-- core/migrate has no automatic rollback, on purpose: undoing DDL on the basis of issued
-- commitments is an administrative act carried out with a person present, not something a
-- process decides at 3am. So the reverse is written here, to be run by hand.
--
--   BEFORE ANY COMMUNE HAS A CALENDAR — all three tables are empty. Reversal is exact and loses
--   nothing: drop lich_lam_viec, ngay_nghi_le and ngay_lam_bu (the 96 partitions go with their
--   parents), then remove this file's progress row from schema_migration, keyed on
--   ten = '0006_lich_lam_viec.sql', so the runner applies it again. Written as prose and not as
--   a runnable line because a runnable line is a line that gets run.
--
--   AFTER THE FIRST DEADLINE HAS BEEN COMPUTED — there is no reversal. The calendar is the
--   basis on which a commitment was made to a citizen and reported upward; dropping it destroys
--   the only record of why a file was due when it was due. That is rule 7 stop condition #1: it
--   needs an explicit decision by the user and a verified backup, and it is not an operation
--   this file offers.
--
-- PER-COMMUNE AND RESUMABLE (rule 7, invariant 5): this file contains no backfill — it creates
-- empty tables and rewrites no existing row — so there is nothing to resume and no commune to
-- iterate. The genuinely per-commune resumable backfill mechanism still does not exist
-- (ADR 0013, §Giới hạn); the first migration that rewrites rows in these tables will need it.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- BACKSTOP, carried forward from 0002–0005.
--
-- A table declared PARTITION BY and given no partitions REJECTS EVERY INSERT, silently, until
-- the first real write — and because the audit entry shares the business transaction (rule 6,
-- invariant 3), that first write rolls back entirely. The check only describes the state after
-- the NEWEST migration that carries it, so every new file ends with it. This file declares
-- three partitioned tables, so it has real work to do here.
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
