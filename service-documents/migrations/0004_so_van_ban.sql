-- documents — SỔ VĂN BẢN ĐẾN và SỔ VĂN BẢN ĐI, cùng dãy số của hai sổ ấy.
--
-- WHY A NEW FILE AND NOT AN EDIT OF AN EARLIER ONE: 0001–0003 have been applied and core/migrate
-- compares the checksum of every applied file at startup. Editing an applied file either stops the
-- service (ErrChecksumLech) or leaves two databases claiming one schema version while holding two
-- different schemas.
--
-- ENTITY OWNERSHIP is declared with the `-- @entity` / `-- @scope` marks above each CREATE TABLE
-- (ADR 0021); tools/kb reads them into kb/30-indexes/data-ownership.json.
--
-- ---------------------------------------------------------------------------
-- THE ONE THING IN THIS FILE THAT CANNOT BE FIXED LATER: THE ISSUED NUMBER.
--
-- `so_vao_so` (sổ đến) and `so_di` (sổ đi) are ISSUED NUMBERS. An outgoing document carries its
-- number on paper, under a seal, in somebody else's filing cabinet. Rule 7, invariant 3 and
-- forbidden #4: a number that has been issued is never reissued and never renumbered — not after a
-- soft delete, not at the start of a new year, not when the register is "cleaned up".
--
-- THREE MECHANISMS, and all three are needed because each one alone still leaks:
--
--   1. `UNIQUE (tenant_id, nam, so_vao_so)` WITHOUT `WHERE deleted_at IS NULL`. A partial unique
--      index counts only live rows, so soft-deleting number 7 would let number 7 be issued again —
--      two records with one number in an archival register, one of which somebody has signed. This
--      is the trap tools/check_khoa_duy_nhat.py exists to catch; the key here is deliberately total.
--   2. `day_so_van_ban` — a COUNTER PER (xã, sổ, năm) that only ever goes up. The obvious
--      implementation, `max(so_vao_so) + 1`, cannot hold: it is computed from rows, and a removed
--      row lowers it. A counter is not computed from rows, so removing a row cannot lower it.
--   3. `FOR UPDATE` on that counter row, in the allocating transaction. Two clerks pressing
--      "Thêm văn bản đi" in the same second both read `so_cuoi = 11`, both write 12, and the
--      UNIQUE key above rejects the second one — which is the SAFE failure, but it is a failure a
--      commune sees as the software being broken. The row lock is what makes the second clerk wait
--      and get 13. NOTHING IN THE APPLICATION LAYER CAN SUBSTITUTE FOR IT: a check in Go reads a
--      value that another transaction may already have moved.
--
-- THE SEQUENCE RESTARTS AT 1 EACH YEAR because the register does: `nam` is part of the counter's
-- key, so 2027 begins with a row that does not exist yet and `so_cuoi` starts at 0. That is
-- administrative practice, not a convenience — "Công văn số 12/2026" and "số 12/2027" are two
-- different documents and both are correct.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS, answered before the SQL.
--
--   1. HOW MANY ROWS PER COMMUNE: zero. All four tables are new and this file writes no row into
--      any of them. A commune fills them by using the register. The volume in sight is a few
--      thousand incoming documents per commune per year (§ "vài chục văn bản đến mỗi tuần").
--   2. IF IT STOPS HALF-WAY: it cannot land half-applied. One file, one transaction, with the
--      progress row written inside it. A failure leaves nothing behind and the next start retries
--      from the beginning; every statement is IF NOT EXISTS, so a retry costs nothing.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. WHICH READ PATHS CHANGE MEANING WHILE IT IS HALF-APPLIED: none. This file only ADDS tables,
--      indexes and triggers of its own. No existing table, column, index or constraint is touched,
--      so every query that runs today returns exactly what it returned before.
--   5. RETENTION: nothing is ever removed. Both registers are ARCHIVAL RECORDS with statutory
--      retention periods; `ho_so_luu_tru_cam_xoa_cung` refuses a hard removal on both, and the
--      routing history is append-only on top of that (rule 7, forbidden #5).
--
-- ---------------------------------------------------------------------------
-- WHAT THE SPECIFICATION SAYS, AND WHERE THIS FILE GOES BEYOND IT — read before "correcting"
-- anything below. docs/ui-ux/05-van-ban-don-thu.md §5 lists `van_ban_den`; the rest is stated
-- here as an assumption and reported to the user rather than hidden in a column:
--
--   `van_ban_di` HAS NO SPECIFICATION AT ALL. Not one field, not one screen, not one API row.
--      Every column of that table is a MIRROR of `van_ban_den` turned around (the commune issues
--      instead of receives), plus `noi_nhan` and `nguoi_ky`, which is what a Vietnamese commune's
--      outgoing register actually holds. kb/00-foundation/ubiquitous-language.md:47 and :143 are
--      the only written source: the concept is `van_ban_di`, the URL resource `outgoing-documents`,
--      and :48 says numbering runs per body and restarts at 01 each year — which is mechanism 2
--      above. THE FIELD LIST IS THIS SESSION'S, NOT THE CUSTOMER'S.
--   `trang_thai` OF `van_ban_den` — §5 says "enum" and never lists the codes. The six codes used
--      here are §3.2's, and the reason they are admissible is §2: the incoming-document half
--      "dùng chung khung sổ" with the citizen-letter half. Should the customer want a different
--      lifecycle for documents, that is a migration on an archival register.
--   `do_khan` — §5 says "enum" and never lists the values. The four here are the statutory list of
--      Nghị định 30/2020/NĐ-CP (Thường · Khẩn · Thượng khẩn · Hoả tốc). NULLABLE, so a commune that
--      does not record urgency records nothing rather than recording "Thường" it never chose.
--   `han_xu_ly` — §5 says `date`. THIS FILE STORES `han_xu_ly_xong TIMESTAMPTZ` instead, and the
--      difference is not a preference: §7 rule 3 sets the commitment in WORKING HOURS (tiếp nhận 8
--      giờ, xử lý xong 40 giờ) and ADR 0007 fixes that a count in days cannot express a count in
--      working hours. A DATE column would round the commitment to a day and widen it silently.
--   `can_bo_xu_ly_id` — §5 says uuid. THIS FILE STORES A STAFF BUSINESS CODE (`CB-2026-7K3M9Q`) and
--      says so in the column NAME (`can_bo_xu_ly_ma`), for the reason rule 6, invariant 8 gives: a
--      column that holds both kinds of identifier is a column nobody can query, and the two are
--      indistinguishable on sight. Same convention as service-finance's voucher register.
--
-- ---------------------------------------------------------------------------
-- WHAT IS DELIBERATELY NOT HERE:
--
--   `han_tiep_nhan`      the `sla` table has a `gio_tiep_nhan` figure for `van-ban-den`, and this
--                        register does not use it. The "tiếp nhận" clock measures how long a record
--                        waits before a human reads it — and an incoming document is BOOKED BY a
--                        member of staff, so the act that creates the row is itself the reading
--                        (kb/00-foundation/ubiquitous-language.md:75, ADR 0028 decision E). Counting
--                        it from `ngay_van_ban` or `ngay_den` instead is a DIFFERENT origin that
--                        would let a clerk book an already-overdue document by typing an old date;
--                        that is the customer's question, not this file's.
--   any `qua_han` column overdue is DERIVED from `han_xu_ly_xong` against now (rule 10, invariant
--                        3). A column set by a nightly job is wrong the moment the job is late.
--   `dinh_kem`           §5 lists attachments on `don_thu`, not on `van_ban_den`. File storage has
--                        no answer in this repository yet.
--   a state machine for  `van_ban_di` has no lifecycle in any source. A number is allocated when
--   `van_ban_di`         the document is issued; that IS the event. Inventing `nhap` -> `da-ky` ->
--                        `da-phat-hanh` would be inventing a workflow a commune then has to follow.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- PostgreSQL 13 is the floor, checked explicitly so the failure is readable.
--
-- BEFORE ... FOR EACH ROW triggers on a PARTITIONED table were only allowed from PostgreSQL 13. On
-- 11 and 12 the CREATE TRIGGER statements below fail with "Partitioned tables cannot have BEFORE /
-- FOR EACH ROW triggers", which reads like a syntax mistake and invites somebody to "fix" it by
-- moving the trigger down onto the partitions — where a partition added later arrives silently
-- unprotected.
-- ---------------------------------------------------------------------------
DO $$ BEGIN
    IF current_setting('server_version_num')::int < 130000 THEN
        RAISE EXCEPTION
            'the document registers need PostgreSQL 13 or newer (server is %). Do not weaken this '
            'migration to fit an older server — the triggers below are what stop an issued '
            'document number from being rewritten.',
            current_setting('server_version');
    END IF;
END $$;

-- ---------------------------------------------------------------------------
-- ho_so_luu_tru_cam_xoa_cung — one function, attached to every archival table in this service.
--
-- It refuses a hard removal and nothing else, so it can be attached to a table whatever its columns
-- are. Rule 7, forbidden #1: business data is soft deleted (deleted_at, deleted_by, delete_reason).
-- An entry in a commune's document register is an archival record with a statutory retention
-- period, and destroying one is an administrative procedure, never a statement somebody types.
--
-- ENFORCED IN THE DATABASE, NOT IN THE APPLICATION, for the reason ADR 0013 gives for the audit
-- ledger: a promise the application layer makes is a promise a migration script, a psql session or
-- the next service to connect never heard.
--
-- THE NAME IS THE ONE service-finance ALREADY USES for the same job (0004 there). Two services, two
-- schemas, one name — a reader who has met it once does not have to read it twice.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION ho_so_luu_tru_cam_xoa_cung() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'archival table %: hard delete refused', TG_TABLE_NAME
        USING HINT = 'Document register entries are archival records (rule 7, invariant 1). Soft '
                     'delete instead: set deleted_at, deleted_by and delete_reason. Destroying one '
                     'is an administrative procedure, not a statement.';
END $$;

-- ---------------------------------------------------------------------------
-- so_van_ban_bat_bien — the issued number, the year and the register a row belongs to can never
-- be changed by an UPDATE.
--
-- WHY A TRIGGER AND NOT A REVIEW HABIT: the whole of rule 7's numbering promise rests on two
-- integers, and the statement that breaks it is a single assignment to `so_vao_so` that looks like
-- a correction. The unique key does NOT catch it — renumbering 7 to 99 collides with nothing. This
-- does.
--
-- `tao_luc` IS FROZEN TOO. It is the instant the register entry was created, which is the fact an
-- inspection compares against the number's position in the series; a rewritable creation time makes
-- the whole series unauditable.
--
-- IT DOES NOT FREEZE `trang_thai`, `bo_phan_dang_giu_id` or the business fields: correcting a
-- mistyped summary and moving a document between departments are the ordinary work of the register,
-- and both leave an audit entry (rule 6).
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION so_van_ban_bat_bien() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.nam IS DISTINCT FROM OLD.nam THEN
        RAISE EXCEPTION 'table %: nam is immutable', TG_TABLE_NAME
            USING HINT = 'A register entry belongs to the year its number was issued in. Moving it '
                         'to another year renumbers an issued document (rule 7, forbidden #4).';
    END IF;
    IF TG_TABLE_NAME LIKE 'van_ban_den%' THEN
        IF NEW.so_vao_so IS DISTINCT FROM OLD.so_vao_so THEN
            RAISE EXCEPTION 'van_ban_den: so_vao_so is immutable'
                USING HINT = 'An issued register number is never reissued and never renumbered '
                             '(rule 7, invariant 3). Correct the entry, or remove it with a reason '
                             'and book a new one — which takes the NEXT number.';
        END IF;
    ELSE
        IF NEW.so_di IS DISTINCT FROM OLD.so_di THEN
            RAISE EXCEPTION 'van_ban_di: so_di is immutable'
                USING HINT = 'An outgoing number is on paper, under a seal, outside this commune. '
                             'It is never renumbered (rule 7, forbidden #4).';
        END IF;
    END IF;
    IF NEW.tao_luc IS DISTINCT FROM OLD.tao_luc THEN
        RAISE EXCEPTION 'table %: tao_luc is immutable', TG_TABLE_NAME
            USING HINT = 'The instant a register entry was created is what an inspection compares '
                         'against the position of its number in the series.';
    END IF;
    RETURN NEW;
END $$;

-- ---------------------------------------------------------------------------
-- @entity: DocumentNumberSeries
-- @scope:  tenant
--
-- day_so_van_ban — ONE COUNTER PER (xã, sổ, năm). This is mechanism 2 of the header.
--
-- IT IS NOT AN ARCHIVAL RECORD AND HAS NO SOFT-DELETE COLUMNS, on purpose: it holds no fact about
-- any document. It holds the high-water mark of a series, and the only thing that may ever happen
-- to it is going up. Giving it a `deleted_at` would invite somebody to "reset the series", which is
-- the one operation the whole file exists to prevent.
--
-- `so_sach` IS A VALUE, NOT A TABLE NAME: 'den' | 'di'. Two registers, two independent series —
-- văn bản đến số 7 and văn bản đi số 7 are two different documents in the same commune in the same
-- year, and both are correct.
--
-- THERE IS NO PostgreSQL SEQUENCE HERE, AND THAT IS DELIBERATE. A sequence is not transactional:
-- `nextval` keeps its value when the transaction rolls back, so a failed booking would BURN a
-- number and the commune's register would have a hole in it. A register with gaps is a register an
-- inspector asks about. A counter row inside the transaction rolls back with everything else.
--
-- THE COST OF THAT CHOICE, STATED: all bookings of one register in one commune in one year
-- SERIALISE on this row. That is one row lock held for the length of one INSERT — and at "vài chục
-- văn bản đến mỗi tuần" it is invisible. It would matter at thousands per minute, and that is not
-- this system.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS day_so_van_ban (
    tenant_id    TEXT        NOT NULL,
    so_sach      TEXT        NOT NULL,
    nam          INT         NOT NULL,
    -- The LAST number issued. 0 means "nothing issued yet this year", so the first document takes
    -- 1 — administrative practice numbers from 1, never from 0.
    so_cuoi      INT         NOT NULL DEFAULT 0,
    tao_luc      TIMESTAMPTZ NOT NULL DEFAULT now(),
    cap_nhat_luc TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- Composite with tenant_id (rule 1, invariant 6) and the whole identity of a series. Not a
    -- partial index: there is nothing to exclude, a counter row is never removed.
    PRIMARY KEY (tenant_id, so_sach, nam),
    CONSTRAINT day_so_van_ban_so_sach_hop_le
        CHECK (so_sach IN ('den', 'di')),
    -- A counter that can go negative is a counter that can be made to reissue a number.
    CONSTRAINT day_so_van_ban_khong_am
        CHECK (so_cuoi >= 0),
    -- A register year outside this range is a typo, and a typo here creates a whole parallel
    -- series nobody can see on any screen.
    CONSTRAINT day_so_van_ban_nam_hop_le
        CHECK (nam BETWEEN 2000 AND 2200)
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS day_so_van_ban_p%s PARTITION OF day_so_van_ban '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

-- A counter row is never removed — not soft, not hard. Without this, taking the row away resets a
-- commune's series to zero and every number of the year is issued a second time.
DROP TRIGGER IF EXISTS day_so_van_ban_cam_xoa_cung ON day_so_van_ban;
CREATE TRIGGER day_so_van_ban_cam_xoa_cung
    BEFORE DELETE ON day_so_van_ban
    FOR EACH ROW EXECUTE FUNCTION ho_so_luu_tru_cam_xoa_cung();

-- ---------------------------------------------------------------------------
-- day_so_khong_lui — the counter may only ever go UP.
--
-- The lock in the allocating transaction stops two clerks from taking one number. It does NOT stop
-- a single statement typed at a prompt that lowers `so_cuoi` back to zero, which hands the whole
-- year's numbers out a second time. This refuses that, and it costs one comparison per allocation.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION day_so_khong_lui() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.so_cuoi < OLD.so_cuoi THEN
        RAISE EXCEPTION 'day_so_van_ban: so_cuoi % -> % would reissue a number', OLD.so_cuoi, NEW.so_cuoi
            USING HINT = 'A document number that has been issued is never reissued (rule 7, '
                         'invariant 3) — not after a soft delete, not after a correction. The '
                         'series only moves forward.';
    END IF;
    IF NEW.tenant_id IS DISTINCT FROM OLD.tenant_id
       OR NEW.so_sach IS DISTINCT FROM OLD.so_sach
       OR NEW.nam IS DISTINCT FROM OLD.nam THEN
        RAISE EXCEPTION 'day_so_van_ban: the identity of a series is immutable'
            USING HINT = 'Moving a counter between communes, registers or years hands one series '
                         'the high-water mark of another.';
    END IF;
    RETURN NEW;
END $$;

DROP TRIGGER IF EXISTS day_so_van_ban_khong_lui ON day_so_van_ban;
CREATE TRIGGER day_so_van_ban_khong_lui
    BEFORE UPDATE ON day_so_van_ban
    FOR EACH ROW EXECUTE FUNCTION day_so_khong_lui();

-- ---------------------------------------------------------------------------
-- @entity: IncomingDocument
-- @scope:  tenant
--
-- van_ban_den — sổ văn bản đến (docs/ui-ux/05-van-ban-don-thu.md §5).
--
-- `loai_van_ban` HOLDS THE CATALOGUE CODE, NOT A FOREIGN KEY TO `loai_van_ban.id`, and the reason
-- is rule 7 rather than convenience: the code is unique per commune and is NEVER REISSUED (0003,
-- `UNIQUE (tenant_id, ma)` counting soft-deleted rows), so it is a stable reference for the life of
-- an archival record — while a row the commune later retires keeps its code and its label, which is
-- what an old document must still render. The write path checks the code against the LIVE catalogue
-- of the commune inside the same transaction; that check is the only one there is, and it is stated
-- rather than implied.
--
-- `bo_phan_dang_giu_id` AND `can_bo_xu_ly_ma` POINT AT ANOTHER SERVICE'S ROWS (identity's `bo_phan`
-- and `nguoi_dung`) AND ARE NOT VALIDATED HERE. A foreign key across a service boundary is rule 2,
-- forbidden #2, and validating them would mean a gRPC call inside the write transaction. WHAT THAT
-- COSTS, stated: a department id that no longer exists renders as a blank cell on the screen. The
-- answer is a check at write time when identity publishes one, never a constraint here.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS van_ban_den (
    tenant_id           TEXT        NOT NULL,
    id                  TEXT        NOT NULL,

    -- THE ISSUED NUMBER. See the three mechanisms in the header.
    so_vao_so           INT         NOT NULL,
    nam                 INT         NOT NULL,

    -- The day the document ARRIVED at the commune. Distinct from `ngay_van_ban`, the day the
    -- issuing body signed it, and from `tao_luc`, the instant somebody typed it in. All three
    -- differ routinely and each answers a different question.
    ngay_den            DATE        NOT NULL,

    -- "1742-CV/BTCTU" — the issuing body's own number. OPTIONAL: documents arrive without one.
    so_ky_hieu          TEXT,
    ngay_van_ban        DATE,
    co_quan_ban_hanh    TEXT        NOT NULL,

    loai_van_ban        TEXT        NOT NULL,
    trich_yeu           TEXT        NOT NULL,
    do_khan             TEXT,

    bo_phan_dang_giu_id TEXT,
    -- A STAFF BUSINESS CODE (`CB-2026-7K3M9Q`), never an internal id — the name says so. Rule 6,
    -- invariant 8: a column holding both kinds is a column nobody can query.
    can_bo_xu_ly_ma     TEXT,

    -- THE COMMITMENT, FIXED ONCE AT THE ACT THAT SETS IT AND NEVER RECOMPUTED ON READ (rule 10,
    -- invariant 2). TIMESTAMPTZ and not DATE: the figure behind it is a count of WORKING HOURS
    -- (ADR 0007) and no count in days can express it. Computed by identity's AdvanceWorkingHours
    -- from this commune's `sla` row and this commune's calendar — never by arithmetic here.
    han_xu_ly_xong      TIMESTAMPTZ NOT NULL,

    trang_thai          TEXT        NOT NULL DEFAULT 'moi-vao-so',

    -- Who booked it, as a staff business code. Same convention as `can_bo_xu_ly_ma`.
    nguoi_tao_ma        TEXT        NOT NULL,

    deleted_at          TIMESTAMPTZ,
    deleted_by          TEXT,
    delete_reason       TEXT,
    tao_luc             TIMESTAMPTZ NOT NULL DEFAULT now(),
    cap_nhat_luc        TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (tenant_id, id),
    -- COMPOSITE WITH tenant_id (rule 1, invariant 6) AND DELIBERATELY NOT PARTIAL (rule 7,
    -- invariant 3). A `WHERE deleted_at IS NULL` here would let a commune soft-delete number 7 and
    -- book a second number 7 — two entries with one number in an archival register.
    UNIQUE (tenant_id, nam, so_vao_so),
    CONSTRAINT van_ban_den_so_duong
        CHECK (so_vao_so >= 1),
    CONSTRAINT van_ban_den_nam_hop_le
        CHECK (nam BETWEEN 2000 AND 2200),
    -- The six codes of §3.2, shared with the citizen-letter register (§2, "dùng chung khung sổ").
    CONSTRAINT van_ban_den_trang_thai_hop_le
        CHECK (trang_thai IN ('moi-vao-so', 'da-phan-cong', 'dang-xu-ly',
                              'da-giai-quyet', 'chuyen-cap-tren', 'luu-khong-thu-ly')),
    -- Nghị định 30/2020/NĐ-CP. NULL is "the commune did not record an urgency", which is not the
    -- same statement as "Thường".
    CONSTRAINT van_ban_den_do_khan_hop_le
        CHECK (do_khan IS NULL OR do_khan IN ('thuong', 'khan', 'thuong-khan', 'hoa-toc')),
    -- A soft delete is all three columns or none of them (rule 7, invariant 1). Two of the three is
    -- a row that vanished from every screen with nobody's name and no reason attached.
    CONSTRAINT van_ban_den_xoa_mem_du_cot
        CHECK ((deleted_at IS NULL AND deleted_by IS NULL AND delete_reason IS NULL)
            OR (deleted_at IS NOT NULL AND deleted_by IS NOT NULL AND delete_reason IS NOT NULL))
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS van_ban_den_p%s PARTITION OF van_ban_den '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

-- The register screen is one commune, one year, newest number first (§3.1: "7, 6, 5… giảm dần").
-- Soft-deleted rows drop out — everywhere, always (rule 7, invariant 2).
CREATE INDEX IF NOT EXISTS van_ban_den_so_theo_nam
    ON van_ban_den (tenant_id, nam, so_vao_so DESC) WHERE deleted_at IS NULL;

-- The "Giao cho tôi" / department filters, and the overdue report, both start from the department
-- holding the file (§3.1 column ĐANG GIỮ).
CREATE INDEX IF NOT EXISTS van_ban_den_theo_bo_phan
    ON van_ban_den (tenant_id, bo_phan_dang_giu_id, han_xu_ly_xong) WHERE deleted_at IS NULL;

DROP TRIGGER IF EXISTS van_ban_den_cam_xoa_cung ON van_ban_den;
CREATE TRIGGER van_ban_den_cam_xoa_cung
    BEFORE DELETE ON van_ban_den
    FOR EACH ROW EXECUTE FUNCTION ho_so_luu_tru_cam_xoa_cung();

DROP TRIGGER IF EXISTS van_ban_den_so_bat_bien ON van_ban_den;
CREATE TRIGGER van_ban_den_so_bat_bien
    BEFORE UPDATE ON van_ban_den
    FOR EACH ROW EXECUTE FUNCTION so_van_ban_bat_bien();

-- ---------------------------------------------------------------------------
-- @entity: OutgoingDocument
-- @scope:  tenant
--
-- van_ban_di — sổ văn bản đi. NO SPECIFICATION EXISTS FOR THIS TABLE; see the header.
--
-- THE NUMBER HERE IS HEAVIER THAN THE ONE ON `van_ban_den`, and the difference is worth a sentence.
-- An incoming number is the commune's own bookkeeping: if it were ever wrong, the commune is the
-- only party affected. An OUTGOING number is printed on a document, sealed, and sent to another
-- body — a district office, a citizen, a court. Two outgoing documents sharing a number are two
-- documents nobody outside this commune can tell apart, and one of them has been signed.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS van_ban_di (
    tenant_id     TEXT        NOT NULL,
    id            TEXT        NOT NULL,

    so_di         INT         NOT NULL,
    nam           INT         NOT NULL,

    -- The day the commune signed and issued it. There is no second date: an outgoing document has
    -- no "arrival".
    ngay_van_ban  DATE        NOT NULL,

    loai_van_ban  TEXT        NOT NULL,
    trich_yeu     TEXT        NOT NULL,
    -- Where it goes. The counterpart of `co_quan_ban_hanh` on the incoming register.
    noi_nhan      TEXT        NOT NULL,
    -- Who signed it — a person's name as it appears under the seal, e.g. "Nguyễn Văn A, Chủ tịch".
    -- FREE TEXT and not a staff code: the signer may be an acting officer, and the register records
    -- what the paper says.
    nguoi_ky      TEXT,

    nguoi_tao_ma  TEXT        NOT NULL,

    deleted_at    TIMESTAMPTZ,
    deleted_by    TEXT,
    delete_reason TEXT,
    tao_luc       TIMESTAMPTZ NOT NULL DEFAULT now(),
    cap_nhat_luc  TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (tenant_id, id),
    -- Composite with tenant_id, counting soft-deleted rows. Same reasoning as `van_ban_den`, and
    -- the consequence of getting it wrong is one step worse — see the note above.
    UNIQUE (tenant_id, nam, so_di),
    CONSTRAINT van_ban_di_so_duong
        CHECK (so_di >= 1),
    CONSTRAINT van_ban_di_nam_hop_le
        CHECK (nam BETWEEN 2000 AND 2200),
    CONSTRAINT van_ban_di_xoa_mem_du_cot
        CHECK ((deleted_at IS NULL AND deleted_by IS NULL AND delete_reason IS NULL)
            OR (deleted_at IS NOT NULL AND deleted_by IS NOT NULL AND delete_reason IS NOT NULL))
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS van_ban_di_p%s PARTITION OF van_ban_di '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

CREATE INDEX IF NOT EXISTS van_ban_di_so_theo_nam
    ON van_ban_di (tenant_id, nam, so_di DESC) WHERE deleted_at IS NULL;

DROP TRIGGER IF EXISTS van_ban_di_cam_xoa_cung ON van_ban_di;
CREATE TRIGGER van_ban_di_cam_xoa_cung
    BEFORE DELETE ON van_ban_di
    FOR EACH ROW EXECUTE FUNCTION ho_so_luu_tru_cam_xoa_cung();

DROP TRIGGER IF EXISTS van_ban_di_so_bat_bien ON van_ban_di;
CREATE TRIGGER van_ban_di_so_bat_bien
    BEFORE UPDATE ON van_ban_di
    FOR EACH ROW EXECUTE FUNCTION so_van_ban_bat_bien();

-- ---------------------------------------------------------------------------
-- @entity: DocumentRouting
-- @scope:  tenant
--
-- lich_su_chuyen_van_ban — "Dòng thời gian chuyển tiếp" (§3.5): who sent this document to which
-- department, when, in what state, with what instruction.
--
-- APPEND-ONLY, ENFORCED BY A TRIGGER (rule 7, forbidden #5). This is the record of a chairman's
-- instruction and of which officer was made responsible; a routing entry that can be edited
-- afterwards is a record with no evidentiary value, which is precisely the thing it exists to be.
-- It therefore has NO soft-delete columns either: there is no state of this table other than
-- "everything that happened".
--
-- IT IS NOT THE AUDIT LOG AND DOES NOT REPLACE IT. `audit_log` answers "who changed what" for the
-- whole service and is invisible to a commune; this is a BUSINESS record the drawer renders and a
-- clerk reads. Both are written, in the same transaction, for the same act.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS lich_su_chuyen_van_ban (
    tenant_id                TEXT        NOT NULL,
    id                       TEXT        NOT NULL,

    -- The incoming document this entry belongs to. Not a foreign key: it is left out because a
    -- routing entry must survive as a historical record even if a future migration reshapes the
    -- register, and because the write path reads the document under a row lock in the same
    -- transaction — a stronger check than a constraint on an id.
    van_ban_den_id           TEXT        NOT NULL,

    thoi_diem                TIMESTAMPTZ NOT NULL,
    -- WHO ROUTED IT, as a staff business code (`CB-2026-7K3M9Q`) — rule 6, invariant 8. This is the
    -- name an inspection reads years later.
    nguoi_ma                 TEXT        NOT NULL,

    -- The state the document was moved INTO by this act. Stored rather than derived: the document's
    -- current state is one value, and the timeline needs the state AT EACH STEP.
    trang_thai_tai_thoi_diem TEXT        NOT NULL,

    -- "→ Một cửa → VĂN PHÒNG ĐẢNG UỶ". `tu_bo_phan_id` is NULL for the first routing, when nobody
    -- was holding the file yet.
    tu_bo_phan_id            TEXT,
    den_bo_phan_id           TEXT        NOT NULL,
    -- "👤 Phụ trách: …", optional — "— Để bộ phận tự phân công —" is a real answer (§3.5).
    can_bo_xu_ly_ma          TEXT,

    -- "Lý do chuyển" / the leader's instruction. MANDATORY: a routing with no reason is an
    -- instruction nobody can account for, and this table is never edited afterwards.
    noi_dung                 TEXT        NOT NULL,

    tao_luc                  TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (tenant_id, id),
    CONSTRAINT lich_su_chuyen_trang_thai_hop_le
        CHECK (trang_thai_tai_thoi_diem IN ('moi-vao-so', 'da-phan-cong', 'dang-xu-ly',
                                            'da-giai-quyet', 'chuyen-cap-tren', 'luu-khong-thu-ly'))
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS lich_su_chuyen_van_ban_p%s PARTITION OF lich_su_chuyen_van_ban '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

-- The drawer reads one document's timeline, newest first.
CREATE INDEX IF NOT EXISTS lich_su_chuyen_theo_van_ban
    ON lich_su_chuyen_van_ban (tenant_id, van_ban_den_id, thoi_diem DESC);

-- ---------------------------------------------------------------------------
-- The append-only guard. Same shape as `audit_log_append_only` (0002) and for the same reasons,
-- including why the TRUNCATE half has to be attached per partition:
--
--   * a row-level trigger on the PARENT is cloned onto every existing partition and onto every
--     partition added later, and an UPDATE against a partitioned table fires on the leaf — so a
--     statement typed straight at `lich_su_chuyen_van_ban_p07` hits the clone too;
--   * PostgreSQL refuses a TRUNCATE trigger on a partitioned table, and statement-level triggers
--     are not cloned — so TRUNCATE is covered leaf by leaf. A partition added AFTER this migration
--     would get the first guard and not the second; MODULUS 32 is fixed for the life of the system
--     (ADR 0010), and this file is re-runnable if that ever changes.
--
-- INSERT is absent from the event list on purpose: the write path pays nothing.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION lich_su_chuyen_chi_them() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'lich_su_chuyen_van_ban is append-only: % on % refused', TG_OP, TG_TABLE_NAME
        USING HINT = 'A routing entry is a historical record (rule 7, forbidden #5): never '
                     'modified, never removed, never truncated. To correct one, route again with '
                     'the correction as the reason.';
END $$;

DROP TRIGGER IF EXISTS lich_su_chuyen_khong_sua_xoa ON lich_su_chuyen_van_ban;
CREATE TRIGGER lich_su_chuyen_khong_sua_xoa
    BEFORE UPDATE OR DELETE ON lich_su_chuyen_van_ban
    FOR EACH ROW EXECUTE FUNCTION lich_su_chuyen_chi_them();

DO $$
DECLARE part regclass;
BEGIN
    FOR part IN
        SELECT inhrelid::regclass FROM pg_inherits
        WHERE inhparent = 'lich_su_chuyen_van_ban'::regclass
    LOOP
        EXECUTE format('DROP TRIGGER IF EXISTS lich_su_chuyen_khong_truncate ON %s', part);
        EXECUTE format(
            'CREATE TRIGGER lich_su_chuyen_khong_truncate BEFORE TRUNCATE ON %s '
            'FOR EACH STATEMENT EXECUTE FUNCTION lich_su_chuyen_chi_them()', part);
    END LOOP;
END $$;

-- ---------------------------------------------------------------------------
-- WHAT THIS FILE DOES NOT STOP, said plainly rather than left to be discovered: TRUNCATE on the two
-- registers, and DDL by the table owner (ALTER TABLE ... DISABLE TRIGGER, dropping a table). Same
-- line ADR 0013 draws for the audit ledger — the job is to make the accidental and the convenient
-- impossible, not to defeat an administrator who has decided to destroy data and is willing to be
-- seen doing it.
--
-- REVERSAL (migration question 3). Every object here is new, and while the tables are still empty
-- the reversal is complete and loses nothing: drop the four tables, then the four trigger functions
-- (`ho_so_luu_tru_cam_xoa_cung` is this service's only copy and goes too), and in the same
-- transaction remove this file's row from `schema_migration` — otherwise the runner still believes
-- the schema is in place. The 128 partitions and the triggers go with their parents.
--
-- ONCE A COMMUNE HAS BOOKED ONE DOCUMENT, THAT IS NO LONGER A REVERSAL — it is the destruction of
-- commune records AND of an issued number series, which is rule 7's first stop condition and needs
-- the user, not a command. From that point the way back is a NEW migration, and core/migrate has no
-- automatic rollback for exactly this reason (ADR 0013).
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
