-- 0006 — SỔ ĐƠN THƯ CÔNG DÂN: the citizen-letter register, its processing log, and a third series on
-- 0004's number counter. Schema only: no business row is written.
--
-- WHY THIS SERVICE: ADR 0039 (the register follows the clerk who issues the number, not the reception
-- desk). WHAT IT HOLDS: the C-list of 24/09/2026 and 30/09/2026 in
-- kb/90-ephemeral/tien-do/service-documents.json → `so-don-thu-cong-dan`, applied by ADR 0078 #2–#4.
--
-- WHY A NEW FILE AND NOT AN EDIT OF 0004: core/migrate compares the checksum of every applied file at
-- startup. Editing an applied file stops the service or leaves two databases with one version number.
--
-- ENTITY OWNERSHIP is declared with the `-- @entity` / `-- @scope` marks above each CREATE TABLE
-- (ADR 0021); tools/kb reads them into kb/30-indexes/data-ownership.json. Nobody edits that file.
--
-- NAMES (rule 12, ADR 0051): every NEW table, column, constraint, index and function here is English.
-- Enum VALUES stay Vietnamese, unaccented, kebab-case (ADR 0011) — `khieu-nai`, `thu-ly`, `don-thu`.
-- The English noun is the one the URL already fixed: `citizen-letters`
-- (kb/00-foundation/ubiquitous-language.md:142). Columns that hold an id or a business code say which
-- in their name (`holding_unit_id`, `assignee_code`, `actor_code`) for the reason 0004:80-83 gives:
-- the two are indistinguishable on sight, and a column that might hold either is unqueryable.
--
-- ---------------------------------------------------------------------------
-- THE ISSUED NUMBER — same three mechanisms as 0004's header, and NOT a second mechanism.
--
--   1. `UNIQUE (tenant_id, year, number)` WITHOUT `WHERE deleted_at IS NULL` — soft-deleting letter 7
--      does not free number 7 (rule 7, invariant 3).
--   2. The counter is 0004's `day_so_van_ban`, with a THIRD `so_sach` value, `don-thu`. ADR 0039 chose
--      this service precisely because that counter and its guards already exist and have been
--      mutation-tested; a parallel counter would have to prove all of that again.
--   3. `FOR UPDATE` on the counter row — internal/store/day_so_van_ban.go `CapSo`, unchanged. The Go
--      side only needs a third `SoSach` constant whose value is 'don-thu'.
--
-- `year` IS THE YEAR OF THE BOOKING ACT (C-list: "NĂM dãy số = năm của hành vi vào sổ"), the same
-- precedent as the incoming register — NOT the year of `received_date`. A letter received on 30/12 and
-- booked on 02/01 takes the new year's series. It is not derived by a CHECK from `created_at`: the
-- application picks the year before the transaction starts, and a CHECK against `now()` would refuse
-- one booking a year at midnight on 31/12 for a reason no clerk could act on.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. ROWS PER COMMUNE: zero written. Two tables are new and empty. The one existing table touched is
--      `day_so_van_ban` (one counter row per commune × register × year — a handful per commune); its
--      `so_sach` CHECK is WIDENED, which validates those rows once under ACCESS EXCLUSIVE. Every row
--      holds 'den' or 'di' and passes. The NOTICE block below reports counter rows per commune.
--   2. IF IT STOPS HALF-WAY: it cannot land half-applied — one file, one transaction (core/migrate),
--      progress row inside it. The DROP and ADD of the widened CHECK commit together, so there is no
--      instant without a CHECK. Every CREATE is IF NOT EXISTS and every ADD CONSTRAINT is guarded, so a
--      retry costs nothing. No per-commune loop: there is nothing to backfill.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. READ PATHS THAT CHANGE MEANING WHILE HALF-APPLIED: none can see it half-applied. AFTER it, the
--      only existing reader affected is anything that lists `day_so_van_ban` rows: from the first
--      booked letter it also sees `so_sach = 'don-thu'` rows. internal/store/day_so_van_ban.go always
--      filters by `so_sach`, so the two document series read exactly as before. A future "series
--      overview" screen must not assume two kinds.
--   5. RETENTION: the register is an ARCHIVAL RECORD (Luật Khiếu nại 2011, Luật Tố cáo 2018 files).
--      Hard delete is refused (`ho_so_luu_tru_cam_xoa_cung`, 0004), the log is append-only, the counter
--      only goes up (0004's `day_so_khong_lui`, untouched). Nothing here removes or rewrites a row.
--
-- ---------------------------------------------------------------------------
-- PERSONAL DATA (rule 3, Decree 13/2023): `sender_name`, `sender_phone`, `sender_address`, `summary`,
-- `result_summary` and the log's `content`. A `to-cao` letter carries a WHISTLEBLOWER's identity
-- (Luật Tố cáo 2018 Đ.8): ADR 0078 #4 — no list, report, export, notification or duplicate warning
-- carries identity or summary of a `to-cao` letter. That is enforced by the read paths, not here; the
-- schema's part is to put NO personal-data column in any index except the one the duplicate check
-- needs (see `citizen_letter_duplicate_by_sender`). Erasure under Decree 13 = anonymise the three
-- sender columns by UPDATE (they are not frozen by the trigger below), never a hard delete.
--
-- ---------------------------------------------------------------------------
-- WHAT IS DELIBERATELY NOT HERE:
--
--   any overdue column     overdue is DERIVED from the two due columns against now (rule 10, inv. 3).
--   computed deadlines     `processing_due_at` / `resolution_due_at` stay NULL this run (C8, ADR 0078
--                          #3): identity's `sla.loai_viec` does not accept a letter kind yet, and ADR
--                          0064's calendar-day rule for `khieu-nai` / `to-cao` still has open points
--                          #3–#5. No default, no arithmetic here (rule 10, forbidden #2).
--   a "sender unknown"     C7: it is DERIVED — all three sender columns NULL. A flag would be a second
--   flag                   source for one fact, and the two would drift.
--   a trigram index        pg_trgm is enabled NOWHERE in this repository (no `CREATE EXTENSION` in any
--                          migration). Adding an extension is a platform decision, not this card's.
--                          The summary-similarity half of C11 runs over the candidate rows the
--                          sender/date index narrows to.
--   attachments, OCR,      no file store and no OCR contract exist (ADR 0078 #6).
--   source / representative / mass-letter fields
--                          domain-expert's TT 05/2021 list (24/09) is unverified against the legal text;
--                          the ledger says verify before writing it down. Additive later.
--   a state machine        transitions (C3) are enforced by the Go domain, as for every other register
--                          in this repository. The schema closes the SET and binds the timestamps to
--                          the statuses; it does not encode the arrows.
-- ---------------------------------------------------------------------------

-- PostgreSQL 13 is the floor for BEFORE … FOR EACH ROW triggers on a partitioned table (0004:106-122).
DO $$ BEGIN
    IF current_setting('server_version_num')::int < 130000 THEN
        RAISE EXCEPTION
            'the citizen-letter register needs PostgreSQL 13 or newer (server is %). Do not weaken '
            'this migration to fit an older server — the triggers below are what stop an issued '
            'letter number from being rewritten.',
            current_setting('server_version');
    END IF;
END $$;

-- ---------------------------------------------------------------------------
-- THE THIRD SERIES on 0004's counter.
--
-- 'don-thu' joins 'den' and 'di'. An enum value, so Vietnamese (ADR 0011). One register, one
-- continuous series per commune per year for all four letter types — the glossary is explicit that
-- splitting the register by type splits an archival series (ubiquitous-language.md:142).
--
-- Same widening shape as service-petitions 0030: DROP … IF EXISTS and ADD with the same name, in one
-- transaction. The new list is a SUPERSET of the old one, so every existing row passes.
-- ---------------------------------------------------------------------------
ALTER TABLE day_so_van_ban DROP CONSTRAINT IF EXISTS day_so_van_ban_so_sach_hop_le;
ALTER TABLE day_so_van_ban ADD CONSTRAINT day_so_van_ban_so_sach_hop_le
    CHECK (so_sach IN ('den', 'di', 'don-thu'));

-- MEASUREMENT (question 1): counter rows per commune. Counts and tenant ids only.
DO $$
DECLARE r record;
BEGIN
    FOR r IN
        SELECT tenant_id, count(*) AS n
        FROM day_so_van_ban
        GROUP BY tenant_id
        ORDER BY tenant_id
    LOOP
        RAISE NOTICE '0006: commune % holds % number-series row(s)', r.tenant_id, r.n;
    END LOOP;
END $$;

-- ---------------------------------------------------------------------------
-- citizen_letter_number_immutable — what an issued letter can never change.
--
-- 0004's `so_van_ban_bat_bien` cannot be reused: it branches on the table name and reads `so_di` on
-- anything that is not `van_ban_den`, which on this table is a runtime error on every UPDATE.
--
-- FROZEN: the commune and the id (the partition key and the identity), the number and the year (rule
-- 7, invariant 3 / forbidden #4 — the unique key does NOT catch 7 → 99, this does), and who booked it
-- when (`created_at`, `created_by_code`: an inspection compares them against the number's position in
-- the series).
--
-- NOT FROZEN: status, holding unit, assignee, sender fields (correction and Decree 13 anonymisation),
-- result fields, soft-delete columns. Each of those writes leaves an audit entry (rule 6) and, for the
-- business acts, a log row.
--
-- The messages name columns only, never values: an error travels to logs and clients (rule 3,
-- forbidden #3).
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION citizen_letter_number_immutable() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.tenant_id IS DISTINCT FROM OLD.tenant_id OR NEW.id IS DISTINCT FROM OLD.id THEN
        RAISE EXCEPTION 'citizen_letter: tenant_id and id are immutable'
            USING HINT = 'Moving a letter between communes is a breach between two public '
                         'authorities (rule 1), not an update.';
    END IF;
    IF NEW.year IS DISTINCT FROM OLD.year OR NEW.number IS DISTINCT FROM OLD.number THEN
        RAISE EXCEPTION 'citizen_letter: number and year are immutable'
            USING HINT = 'An issued register number is never reissued and never renumbered (rule 7, '
                         'invariant 3). Correct the entry, or remove it with a reason and book a new '
                         'one — which takes the NEXT number.';
    END IF;
    IF NEW.created_at IS DISTINCT FROM OLD.created_at
       OR NEW.created_by_code IS DISTINCT FROM OLD.created_by_code THEN
        RAISE EXCEPTION 'citizen_letter: created_at and created_by_code are immutable'
            USING HINT = 'Who booked an entry and when is what an inspection compares against the '
                         'position of its number in the series.';
    END IF;
    RETURN NEW;
END $$;

-- ---------------------------------------------------------------------------
-- @entity: CitizenLetter
-- @scope:  tenant
--
-- citizen_letter — sổ đơn thư công dân (`don_thu` in the glossary; URL `citizen-letters`).
--
-- `holding_unit_id` AND `assignee_code` POINT AT identity's rows (`bo_phan`, `nguoi_dung`) AND ARE NOT
-- VALIDATED HERE — a cross-service foreign key is rule 2, forbidden #2 (same stance as 0004:303-307).
-- Assignment is an ATTRIBUTE, not a status (C3).
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS citizen_letter (
    tenant_id             TEXT        NOT NULL,
    id                    TEXT        NOT NULL,

    -- THE ISSUED NUMBER, from `day_so_van_ban` (so_sach = 'don-thu'). Auto-issued, never edited (C6).
    number                INT         NOT NULL,
    year                  INT         NOT NULL,

    -- The day the letter ARRIVED. Distinct from `created_at`, the instant a clerk booked it.
    received_date         DATE        NOT NULL,

    -- C4: four types. `phan-anh` alone is NOT a type here — that word belongs to service-petitions'
    -- citizen reports, and conflating the two applies the wrong statutory procedure.
    letter_type           TEXT        NOT NULL,

    -- ⚠ PERSONAL DATA. ALL OPTIONAL (C7): anonymous and partly identified letters are real. NULL means
    -- "not given"; '' is refused so that `IS NULL` finds every absent value.
    sender_name           TEXT,
    sender_phone          TEXT,
    sender_address        TEXT,

    -- ⚠ May hold personal data (it quotes the letter).
    summary               TEXT        NOT NULL,

    status                TEXT        NOT NULL DEFAULT 'moi-vao-so',

    -- WHO HOLDS IT NOW. The log holds who held it at each step.
    holding_unit_id       TEXT,
    -- A STAFF BUSINESS CODE (`CB-…`), never an internal id (rule 6, invariant 8).
    assignee_code         TEXT,

    -- THE TWO COMMITMENTS (C8), each fixed ONCE at the act that sets it and never recomputed on read
    -- (rule 10, invariant 2): processing at booking, resolution at `thu-ly`. TIMESTAMPTZ, not DATE,
    -- because identity returns an instant (ADR 0064 "Trả gì"). NULL this run — see the header.
    processing_due_at     TIMESTAMPTZ,
    resolution_due_at     TIMESTAMPTZ,

    -- When the letter was accepted for resolution (`thu-ly`), and when resolution ended.
    accepted_at           TIMESTAMPTZ,
    resolved_at           TIMESTAMPTZ,

    -- C11: the letter a clerk CONFIRMED this one duplicates. Set only by a person pressing confirm on
    -- the warning; the server never links on its own.
    related_letter_id     TEXT,

    -- C10: the result is an ISSUED DOCUMENT plus a summary. Not a reference to `van_ban_di`: the
    -- answer may be issued by another body (district, inspectorate), so the register records what the
    -- paper says, like `van_ban_di.nguoi_ky`. Not sent anywhere (Zalo OA is rule 3 stop condition #2).
    result_document_no    TEXT,
    result_document_date  DATE,
    result_signer         TEXT,
    result_issuer         TEXT,
    -- ⚠ May hold personal data.
    result_summary        TEXT,

    created_by_code       TEXT        NOT NULL,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by_code       TEXT,
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now(),

    deleted_at            TIMESTAMPTZ,
    deleted_by            TEXT,
    delete_reason         TEXT,

    PRIMARY KEY (tenant_id, id),
    -- COMPOSITE WITH tenant_id (rule 1, invariant 6) AND DELIBERATELY NOT PARTIAL (rule 7, invariant 3).
    UNIQUE (tenant_id, year, number),

    CONSTRAINT citizen_letter_number_positive CHECK (number >= 1),
    CONSTRAINT citizen_letter_year_valid CHECK (year BETWEEN 2000 AND 2200),

    CONSTRAINT citizen_letter_type_valid
        CHECK (letter_type IN ('kien-nghi-phan-anh', 'khieu-nai', 'to-cao', 'de-nghi')),

    -- C3, TT 05/2021: moi-vao-so → dang-xu-ly-don → {thu-ly | khong-thu-ly | huong-dan | chuyen-don |
    -- luu-don}; thu-ly → dang-giai-quyet → {da-giai-quyet | dinh-chi}. The log's two status CHECKs carry
    -- the SAME ten codes; migrations/citizen_letter_test.go compares the lists.
    CONSTRAINT citizen_letter_status_valid
        CHECK (status IN ('moi-vao-so', 'dang-xu-ly-don', 'thu-ly', 'khong-thu-ly', 'huong-dan',
                          'chuyen-don', 'luu-don', 'dang-giai-quyet', 'da-giai-quyet', 'dinh-chi')),

    -- Optional sender fields: NULL or non-blank, bounded in CHARACTERS. 200/300 mirror
    -- domain.NhanToiDa / TranCoQuan; 20 is a phone number with country code and separators.
    CONSTRAINT citizen_letter_sender_name_valid
        CHECK (sender_name IS NULL OR (btrim(sender_name) <> '' AND char_length(sender_name) <= 200)),
    CONSTRAINT citizen_letter_sender_phone_valid
        CHECK (sender_phone IS NULL OR (btrim(sender_phone) <> '' AND char_length(sender_phone) <= 20)),
    CONSTRAINT citizen_letter_sender_address_valid
        CHECK (sender_address IS NULL OR (btrim(sender_address) <> '' AND char_length(sender_address) <= 500)),

    -- 2000 = domain.TranTrichYeu, the summary bound of the incoming register.
    CONSTRAINT citizen_letter_summary_valid
        CHECK (btrim(summary) <> '' AND char_length(summary) <= 2000),

    -- 64 = domain.TranBoPhanID / TranMaCanBo.
    CONSTRAINT citizen_letter_holding_unit_valid
        CHECK (holding_unit_id IS NULL OR (btrim(holding_unit_id) <> '' AND char_length(holding_unit_id) <= 64)),
    CONSTRAINT citizen_letter_assignee_valid
        CHECK (assignee_code IS NULL OR (btrim(assignee_code) <> '' AND char_length(assignee_code) <= 64)),
    CONSTRAINT citizen_letter_created_by_valid
        CHECK (btrim(created_by_code) <> '' AND char_length(created_by_code) <= 64),
    CONSTRAINT citizen_letter_updated_by_valid
        CHECK (updated_by_code IS NULL OR (btrim(updated_by_code) <> '' AND char_length(updated_by_code) <= 64)),

    -- `accepted_at` IS SET EXACTLY ON THE `thu-ly` BRANCH. A letter in `khong-thu-ly` carrying an
    -- acceptance time reads as accepted to one report and not to another. Every arm is TRUE or FALSE,
    -- never NULL — a CHECK treats NULL as passed.
    CONSTRAINT citizen_letter_accepted_at_matches_status CHECK (
        CASE WHEN status IN ('thu-ly', 'dang-giai-quyet', 'da-giai-quyet', 'dinh-chi')
             THEN accepted_at IS NOT NULL
             ELSE accepted_at IS NULL
        END),
    -- `resolved_at` IS SET EXACTLY ON THE TWO ENDS OF RESOLUTION, and never before acceptance.
    CONSTRAINT citizen_letter_resolved_at_matches_status CHECK (
        CASE WHEN status IN ('da-giai-quyet', 'dinh-chi')
             THEN resolved_at IS NOT NULL AND accepted_at IS NOT NULL AND resolved_at >= accepted_at
             ELSE resolved_at IS NULL
        END),
    -- The resolution deadline is fixed AT `thu-ly` (C8); it cannot exist on a letter never accepted.
    CONSTRAINT citizen_letter_resolution_due_needs_acceptance
        CHECK (resolution_due_at IS NULL OR accepted_at IS NOT NULL),

    -- A letter never duplicates itself, and '' is a broken reference, not "none".
    CONSTRAINT citizen_letter_related_not_self
        CHECK (related_letter_id IS NULL OR (btrim(related_letter_id) <> '' AND related_letter_id <> id)),

    -- RESULT FIELDS: NULL or non-blank, bounded like the incoming register's matching columns
    -- (TranSoKyHieu 100, TranNguoiKy 200, TranCoQuan 300, TranTrichYeu 2000).
    CONSTRAINT citizen_letter_result_document_no_valid
        CHECK (result_document_no IS NULL OR (btrim(result_document_no) <> '' AND char_length(result_document_no) <= 100)),
    CONSTRAINT citizen_letter_result_signer_valid
        CHECK (result_signer IS NULL OR (btrim(result_signer) <> '' AND char_length(result_signer) <= 200)),
    CONSTRAINT citizen_letter_result_issuer_valid
        CHECK (result_issuer IS NULL OR (btrim(result_issuer) <> '' AND char_length(result_issuer) <= 300)),
    CONSTRAINT citizen_letter_result_summary_valid
        CHECK (result_summary IS NULL OR (btrim(result_summary) <> '' AND char_length(result_summary) <= 2000)),
    -- Half a citation ("số 12" with no date) is a document nobody can find in a register.
    CONSTRAINT citizen_letter_result_document_pair
        CHECK ((result_document_no IS NULL) = (result_document_date IS NULL)),
    -- C10: `da-giai-quyet` IS CLOSED WITH ITS RESULT — the issued document (number, date, signer,
    -- issuing body) and a summary. Closing silently is what the citizen cannot tell from being ignored.
    -- Only this status is bound: whether `dinh-chi` and the processing-phase outcomes require a result
    -- document is not decided anywhere (see the card report), so the schema does not decide it.
    CONSTRAINT citizen_letter_resolved_has_result CHECK (
        status <> 'da-giai-quyet'
        OR (result_document_no IS NOT NULL AND result_document_date IS NOT NULL
            AND result_signer IS NOT NULL AND result_issuer IS NOT NULL
            AND result_summary IS NOT NULL)),

    -- A soft delete is all three columns or none (rule 7, invariant 1), as 0004:370-374.
    CONSTRAINT citizen_letter_soft_delete_complete
        CHECK ((deleted_at IS NULL AND deleted_by IS NULL AND delete_reason IS NULL)
            OR (deleted_at IS NOT NULL AND deleted_by IS NOT NULL AND btrim(deleted_by) <> ''
                AND delete_reason IS NOT NULL AND btrim(delete_reason) <> ''
                AND char_length(delete_reason) <= 500))
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS citizen_letter_p%s PARTITION OF citizen_letter '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

-- THE CONFIRMED DUPLICATE IS A REAL LETTER OF THE SAME COMMUNE. Composite self-referencing key, the
-- shape of service-petitions 0012 `bien_ban_hop_bo_sung_cho_fk`: `tenant_id` on both sides, so a link
-- cannot cross communes even if two ids ever collided (rule 1). MATCH SIMPLE: a NULL link is not
-- checked. The target can never vanish — hard delete is refused below. Added after the partitions,
-- guarded, so a re-run is free.
DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'citizen_letter_related_letter_fk') THEN
        ALTER TABLE citizen_letter ADD CONSTRAINT citizen_letter_related_letter_fk
            FOREIGN KEY (tenant_id, related_letter_id) REFERENCES citizen_letter (tenant_id, id);
    END IF;
END $$;

-- INDEXES. Every one starts with tenant_id (rule 1) and drops soft-deleted rows (rule 7, invariant 2).
-- The register order itself — one commune, one year, newest number first — is served by the UNIQUE
-- key above read backwards; it is not duplicated.

-- The register list filtered by status (the status chips), newest number first.
CREATE INDEX IF NOT EXISTS citizen_letter_by_year_status
    ON citizen_letter (tenant_id, year, status, number DESC) WHERE deleted_at IS NULL;

-- "Đang giữ" / department filter.
CREATE INDEX IF NOT EXISTS citizen_letter_by_holding_unit
    ON citizen_letter (tenant_id, holding_unit_id, received_date DESC) WHERE deleted_at IS NULL;

-- "Giao cho tôi" / officer filter (C18 asks for it as an addition).
CREATE INDEX IF NOT EXISTS citizen_letter_by_assignee
    ON citizen_letter (tenant_id, assignee_code, received_date DESC) WHERE deleted_at IS NULL;

-- Received-date range filter and reports (C16/C17). `id` is the cursor tie-break.
CREATE INDEX IF NOT EXISTS citizen_letter_by_received_date
    ON citizen_letter (tenant_id, received_date DESC, id DESC) WHERE deleted_at IS NULL;

-- THE DUPLICATE CHECK (C11): same sender name, received within a date window, same commune. The
-- candidates it returns are then compared on summary similarity in the application (no pg_trgm).
--
-- ⚠ THIS IS THE ONLY INDEX HOLDING PERSONAL DATA, and it holds the NAME only. That is the minimum the
-- check needs: C11 matches on the sender, and the phone/address are not part of the match — so they
-- are in no index at all, and an index is one more copy of a value that backups and replicas carry.
-- Rows without a name are excluded: an anonymous letter has nothing to match on by name.
--
-- THE EXPRESSION IS PART OF THE CONTRACT: the query must say `lower(sender_name)` byte for byte, or the
-- planner will not use this index. Values are stored trimmed (the write path), so no btrim here.
CREATE INDEX IF NOT EXISTS citizen_letter_duplicate_by_sender
    ON citizen_letter (tenant_id, lower(sender_name), received_date)
    WHERE deleted_at IS NULL AND sender_name IS NOT NULL;

DROP TRIGGER IF EXISTS citizen_letter_no_hard_delete ON citizen_letter;
CREATE TRIGGER citizen_letter_no_hard_delete
    BEFORE DELETE ON citizen_letter
    FOR EACH ROW EXECUTE FUNCTION ho_so_luu_tru_cam_xoa_cung();

DROP TRIGGER IF EXISTS citizen_letter_number_immutable ON citizen_letter;
CREATE TRIGGER citizen_letter_number_immutable
    BEFORE UPDATE ON citizen_letter
    FOR EACH ROW EXECUTE FUNCTION citizen_letter_number_immutable();

-- ---------------------------------------------------------------------------
-- @entity: CitizenLetterLogEntry
-- @scope:  tenant
--
-- citizen_letter_log — the processing log (nhật ký) of one letter: who did what, when, in which status.
--
-- APPEND-ONLY, ENFORCED BY A TRIGGER (rule 7, forbidden #5), and therefore WITH NO SOFT-DELETE COLUMNS.
-- Same shape as `lich_su_chuyen_van_ban` (0004) and service-petitions' `nhat_ky_phan_anh` (0013).
--
-- A ROW NEED NOT CHANGE THE STATUS: the logbook is separate from the status change (cross-check
-- 430c9ae in the ledger) — a note or a sender correction is a row with both status columns NULL.
--
-- NOT THE AUDIT LOG: `audit_log` is invisible to a commune; this is the business timeline the drawer
-- renders. Both are written in the same transaction for the same act (rule 6, invariant 3).
--
-- STAFF-INTERNAL: never shown to a citizen (rule 4, forbidden #5).
--
-- ⚠ `content` MAY HOLD PERSONAL DATA. And a `sua-nguoi-gui` row records THAT the sender was corrected,
-- never the old or new values — an append-only table holding raw before/after personal data is a
-- personal-data store nobody can ever anonymise (rule 6, forbidden #4; rule 3, invariant 7). The
-- schema cannot see that; the write path must not put them there.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS citizen_letter_log (
    tenant_id      TEXT        NOT NULL,
    id             TEXT        NOT NULL,

    -- NOT a foreign key, for the reason 0004:500-503 gives: a historical entry must survive a future
    -- reshaping of the register, and the write path reads the letter under a row lock in the same
    -- transaction.
    letter_id      TEXT        NOT NULL,

    at             TIMESTAMPTZ NOT NULL,
    -- WHO, as a staff business code — rule 6, invariant 8. A system act writes its own code.
    actor_code     TEXT        NOT NULL,

    -- WHAT KIND OF ACT. Enum VALUES, Vietnamese (ADR 0011): chuyen-trang-thai (status change) ·
    -- luan-chuyen (routing; the glossary's word, ubiquitous-language.md:51) · ghi-chu (note) ·
    -- ket-qua (result recorded) · sua-nguoi-gui (sender corrected).
    kind           TEXT        NOT NULL,

    from_status    TEXT,
    to_status      TEXT,

    from_unit_id   TEXT,
    to_unit_id     TEXT,
    assignee_code  TEXT,

    -- ⚠ PERSONAL DATA possible.
    content        TEXT,

    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (tenant_id, id),

    CONSTRAINT citizen_letter_log_kind_valid CHECK (kind IN (
        'chuyen-trang-thai', 'luan-chuyen', 'ghi-chu', 'ket-qua', 'sua-nguoi-gui')),

    -- The SAME ten codes as `citizen_letter_status_valid`. A log row holding a status the register
    -- would refuse contradicts the row it hangs off.
    CONSTRAINT citizen_letter_log_from_status_valid CHECK (from_status IN (
        'moi-vao-so', 'dang-xu-ly-don', 'thu-ly', 'khong-thu-ly', 'huong-dan',
        'chuyen-don', 'luu-don', 'dang-giai-quyet', 'da-giai-quyet', 'dinh-chi')),
    CONSTRAINT citizen_letter_log_to_status_valid CHECK (to_status IN (
        'moi-vao-so', 'dang-xu-ly-don', 'thu-ly', 'khong-thu-ly', 'huong-dan',
        'chuyen-don', 'luu-don', 'dang-giai-quyet', 'da-giai-quyet', 'dinh-chi')),

    CONSTRAINT citizen_letter_log_actor_valid
        CHECK (btrim(actor_code) <> '' AND char_length(actor_code) <= 64),

    -- THE STATUS PAIR travels together and is a real change: both NULL, or both set and different.
    -- `chuyen-trang-thai` must carry it; `ghi-chu` and `sua-nguoi-gui` must not (they change nothing);
    -- `luan-chuyen` and `ket-qua` may (routing can move `moi-vao-so` → `dang-xu-ly-don`, a result
    -- closes the letter).
    CONSTRAINT citizen_letter_log_status_pair CHECK (
        (from_status IS NULL AND to_status IS NULL)
        OR (from_status IS NOT NULL AND to_status IS NOT NULL AND from_status <> to_status)),
    CONSTRAINT citizen_letter_log_status_by_kind CHECK (
        CASE kind
            WHEN 'chuyen-trang-thai' THEN to_status IS NOT NULL
            WHEN 'ghi-chu'           THEN to_status IS NULL
            WHEN 'sua-nguoi-gui'     THEN to_status IS NULL
            ELSE TRUE
        END),

    -- THE ROUTING TRIPLE IS BOUND TO `luan-chuyen`, both directions: destination required; origin NULL
    -- only for the first routing; officer optional ("để bộ phận tự phân công" is a real answer). Any
    -- other kind carries none of them — a stray unit on a note row reads as a hand-over that never
    -- happened. 64 = TranBoPhanID / TranMaCanBo.
    CONSTRAINT citizen_letter_log_routing_by_kind CHECK (
        CASE kind
            WHEN 'luan-chuyen' THEN
                to_unit_id IS NOT NULL AND btrim(to_unit_id) <> ''
                AND (from_unit_id IS NULL OR btrim(from_unit_id) <> '')
                AND (assignee_code IS NULL OR btrim(assignee_code) <> '')
            ELSE
                from_unit_id IS NULL AND to_unit_id IS NULL AND assignee_code IS NULL
        END),
    CONSTRAINT citizen_letter_log_from_unit_max CHECK (char_length(from_unit_id) <= 64),
    CONSTRAINT citizen_letter_log_to_unit_max CHECK (char_length(to_unit_id) <= 64),
    CONSTRAINT citizen_letter_log_assignee_max CHECK (char_length(assignee_code) <= 64),

    -- CONTENT: mandatory on `ghi-chu` (an empty note) and on `luan-chuyen` (a routing with no reason is
    -- an instruction nobody can account for — 0004:522-524); optional elsewhere but never blank.
    -- 2000 = TranTrichYeu.
    CONSTRAINT citizen_letter_log_content_by_kind CHECK (
        CASE kind
            WHEN 'ghi-chu'     THEN content IS NOT NULL AND btrim(content) <> ''
            WHEN 'luan-chuyen' THEN content IS NOT NULL AND btrim(content) <> ''
            ELSE content IS NULL OR btrim(content) <> ''
        END),
    CONSTRAINT citizen_letter_log_content_max CHECK (char_length(content) <= 2000)
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS citizen_letter_log_p%s PARTITION OF citizen_letter_log '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

-- The drawer reads one letter's log, newest first; `id` is the tie-break for two acts in one
-- transaction sharing `at`.
CREATE INDEX IF NOT EXISTS citizen_letter_log_by_letter
    ON citizen_letter_log (tenant_id, letter_id, at DESC, id DESC);

-- ---------------------------------------------------------------------------
-- The append-only guard. Same shape as 0004's `lich_su_chuyen_chi_them`: the row-level trigger on
-- the parent is cloned to every partition, existing and future; TRUNCATE is covered leaf by leaf
-- because PostgreSQL refuses a TRUNCATE trigger on a partitioned table. INSERT is not in the list.
-- The message names the operation and relation only — `content` may hold personal data.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION citizen_letter_log_append_only() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'citizen_letter_log is append-only: % on % refused', TG_OP, TG_TABLE_NAME
        USING HINT = 'A letter log entry is a historical record (rule 7, forbidden #5): never '
                     'modified, never removed, never truncated. To correct one, write another '
                     'entry carrying the correction.';
END $$;

DROP TRIGGER IF EXISTS citizen_letter_log_no_update_delete ON citizen_letter_log;
CREATE TRIGGER citizen_letter_log_no_update_delete
    BEFORE UPDATE OR DELETE ON citizen_letter_log
    FOR EACH ROW EXECUTE FUNCTION citizen_letter_log_append_only();

DO $$
DECLARE part regclass;
BEGIN
    FOR part IN
        SELECT inhrelid::regclass FROM pg_inherits
        WHERE inhparent = 'citizen_letter_log'::regclass
    LOOP
        EXECUTE format('DROP TRIGGER IF EXISTS citizen_letter_log_no_truncate ON %s', part);
        EXECUTE format(
            'CREATE TRIGGER citizen_letter_log_no_truncate BEFORE TRUNCATE ON %s '
            'FOR EACH STATEMENT EXECUTE FUNCTION citizen_letter_log_append_only()', part);
    END LOOP;
END $$;

-- ---------------------------------------------------------------------------
-- WHAT THIS FILE DOES NOT STOP — same line as 0004:588-593: TRUNCATE on `citizen_letter` itself, and
-- DDL by the table owner (DISABLE TRIGGER, dropping a table).
--
-- REVERSAL (rule 7, invariant 4). Run by a person, in ONE transaction; core/migrate has no automatic
-- rollback (ADR 0013). LOSSLESS ONLY WHILE NO LETTER HAS BEEN BOOKED. Check first:
--
--   SELECT count(*) FROM citizen_letter;                                  -- must be 0
--   SELECT count(*) FROM citizen_letter_log;                              -- must be 0
--   SELECT count(*) FROM day_so_van_ban WHERE so_sach = 'don-thu';        -- must be 0
--
-- then: drop `citizen_letter_log` and `citizen_letter` (their 64 partitions, indexes and triggers go
-- with them), drop functions `citizen_letter_log_append_only` and `citizen_letter_number_immutable`
-- (NOT `ho_so_luu_tru_cam_xoa_cung` — it belongs to 0004), restore 0004's two-value CHECK on
-- `day_so_van_ban` (DROP CONSTRAINT day_so_van_ban_so_sach_hop_le, ADD it back with ('den', 'di')),
-- and delete this file's row from `schema_migration`.
--
-- ONCE ONE LETTER IS BOOKED, THIS IS NO LONGER A REVERSAL: it destroys archival records and an issued
-- number series — rule 7 stop condition #1, the user's decision, never a command. From then on the way
-- back is a new migration.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- BACKSTOP: every partitioned table in this schema must actually have partitions (0002, §BACKSTOP).
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
