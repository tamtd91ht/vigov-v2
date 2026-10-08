-- 0032 — a petition may be OWNED BY A ZALO ACCOUNT instead of a citizen identity: the petition sent
-- from a Mini App session that has NOT verified a phone number (ADR 0080, owner decisions 08/10/2026).
-- Schema only: no row is written.
--
-- WHY THIS FILE EXISTS. ADR 0080 decision 2: the name and phone typed on such a petition are CONTACT
-- DETAILS, not an identity — `cong_dan_id` stays NULL — and the owner is the session's Zalo account,
-- `tai_khoan_zalo.id` in service-identity (service-identity/migrations/0011_tai_khoan_zalo_va_phien_chua_co_so.sql:97).
-- Until now a petition had exactly two shapes: filed by a citizen (`cong_dan_id` set) or booked by
-- staff (`cong_dan_id` NULL, nobody's session can reach it). This file adds the third and gives it a
-- column of its own, so the three never have to be told apart by inference.
--
-- WHY A NEW FILE AND NOT AN EDIT OF 0004 / 0026: core/migrate compares the checksum of every applied
-- file at startup. Editing an applied file either stops the service or leaves two databases claiming
-- one schema version while holding two schemas.
--
-- ---------------------------------------------------------------------------
-- THE NAME: `zalo_account_id`. English (rule 12, ADR 0051), on a Vietnamese-named table whose
-- existing columns are not renamed (rule 12, invariant 3) — precedent 0017's `rating_comment`,
-- 0031's `decision_note`. It names the identity-side entity it points at (`tai_khoan_zalo` = Zalo
-- account) and the same field the citizen session token carries since 7c0e55c9
-- (proto, `zalo_account_id`), so the value is spelled one way from the token to the row.
--
-- IT IS AN OPAQUE ULID HELD AS A VALUE. NO FOREIGN KEY, AND THERE CAN NEVER BE ONE: `tai_khoan_zalo`
-- lives in service-identity's database, and a connection to it is rule 2, forbidden #2. The value
-- comes from the verified session (rule 4, invariant 2), never from a request body.
--
-- IT IS NOT A PHONE NUMBER AND NOT A ZALO USER ID. It names a row in identity; the Zalo user id
-- (`zalo_user_id`) stays in identity. Staff screens never need it and the API must not return it:
-- a citizen-side internal id on a staff screen would let staff link one person's ANONYMOUS reports
-- together (ADR 0008), the same reason 0026 keeps the citizen id off `stored_file`.
--
-- ---------------------------------------------------------------------------
-- NO SEPARATE "UNVERIFIED" FLAG COLUMN (ADR 0080 decision 5).
--
-- Decision 5 wants Web Admin's label "Số tự khai — chưa xác thực" to come from an EXPLICIT server
-- field, not from `kenh_tiep_nhan` + "has a citizen" — a staff-booked petition also has no citizen.
-- The stored fact that IS explicit is the OWNER KIND, and this column records it:
--
--   zalo_account_id IS NOT NULL   ⇔   the petition's contact details are self-declared, unverified.
--
-- The API field the label reads is computed from THIS column alone, at response time. A boolean
-- next to it would be a second copy of one fact, writable separately, and the day the two disagree
-- the label lies to staff about whether a phone number was verified (rule 9; rule 10's "one source"
-- reasoning for `is_overdue`). The CHECK below makes the two owner kinds exclusive, so the
-- derivation has no third case to guess about.
--
-- ---------------------------------------------------------------------------
-- WHAT THE DATABASE NOW ENFORCES, for every writer, not as a promise of one route:
--
--   * phieu_phan_anh_zalo_account_id_valid      present means non-blank: '' is not an owner, and a
--                                               blank owner would read as "owned" by the label and as
--                                               "nobody" by every lookup.
--   * phieu_phan_anh_single_owner               at most ONE owner kind: never both `cong_dan_id` and
--                                               `zalo_account_id`. A petition that had both would sit
--                                               in "Phản ánh của tôi" (ADR 0080 stop condition #1) and
--                                               be labelled unverified at the same time.
--   * phieu_phan_anh_zalo_owner_mini_app        a Zalo-owned petition came through the Mini App. Any
--                                               other channel with this owner is a no-phone path
--                                               OUTSIDE Zalo — ADR 0080 stop condition #3.
--   * phieu_phan_anh_zalo_owner_no_rating_reopen no rating, no rating comment, no reopen (ADR 0080
--                                               decision 8). The use case refuses first; this is the
--                                               floor under it.
--   * phieu_phan_anh_zalo_owner_frozen (trigger) the owner is written at intake and never changes: not
--                                               set later, not cleared, not moved. Clearing it and
--                                               setting `cong_dan_id` is exactly the "take over an old
--                                               petition" ADR 0080 cost #5 / ADR 0020 CÒN MỞ #1 leave
--                                               undecided; refusing is the fail-closed answer until
--                                               someone decides it.
--
-- AND IT ADMITS a citizen scene photo on a Zalo-owned petition (ADR 0080 decision 8) by replacing
-- 0026's stored_file_petition_check — see that section below.
--
-- ---------------------------------------------------------------------------
-- THE INDEX. Two citizen-side reads touch this column:
--
--   * LOOKUP BY CODE + SAME ACCOUNT (decision 3):
--       WHERE tenant_id = $1 AND ma_tra_cuu = $2 AND zalo_account_id = $3 AND deleted_at IS NULL
--     Already served by 0004's UNIQUE (tenant_id, ma_tra_cuu): one row at most, the account is then a
--     heap filter. No index needed for it — and adding one would not make it faster.
--   * THE 10-PER-DAY CEILING (decision 7):
--       WHERE tenant_id = $1 AND zalo_account_id = $2 AND vao_so_luc >= $3
--     No existing index carries the account, so the count would read the commune's whole register on
--     every send. `phieu_phan_anh_zalo_account` below serves it: tenant_id first (rule 1, and the hash
--     partition key), the equality, then the time range.
--
-- THE PARTIAL PREDICATE IS `zalo_account_id IS NOT NULL` AND NOTHING ELSE. PostgreSQL proves it from
-- any `zalo_account_id = $n` (the operator is strict), so the index serves the count whether the
-- write path counts soft-deleted petitions or not — which it should count is a ceiling question the
-- Go card states, not a planner question this file settles. Citizen-owned and staff-booked rows are
-- out of it, so it stays the size of the unverified traffic.
--
-- ---------------------------------------------------------------------------
-- THE LOCK THIS TAKES. ADD COLUMN with no default is catalogue-only (no row rewritten) but takes
-- ACCESS EXCLUSIVE on the parent and all 32 partitions. The four ADD CONSTRAINT … CHECK statements scan
-- every row under that lock; on a column that is NULL in every row each one passes vacuously
-- (`phieu_phan_anh_single_owner` included: no row has an account). The CREATE INDEX is plain, not
-- CONCURRENTLY — refused on a partitioned parent and inside core/migrate's one transaction (0014's
-- header gives the full reasoning) — and its build over a NULL-everywhere column indexes nothing.
-- core/migrate holds the locks until COMMIT. The register is small (one commune live, 0014's header) —
-- AN ASSUMPTION, NOT A MEASUREMENT; the operator checks first:
--
--   SELECT count(*), pg_size_pretty(pg_total_relation_size('phieu_phan_anh')) FROM phieu_phan_anh;
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. ROWS PER COMMUNE: zero written. Every petition gains a NULL column without a rewrite; the CHECKs
--      scan every row once. NO BACKFILL, deliberately: every existing petition was filed by a verified
--      citizen or booked by staff, so NULL is the TRUE value for all of them, not a placeholder. The
--      NOTICE below reports per commune how many petitions exist and how many carry no citizen (the
--      staff-booked rows, which must stay unowned) — counts and tenant ids only (rule 3).
--   2. IF IT STOPS HALF-WAY: it cannot land half-applied — one file, one transaction (core/migrate),
--      with its progress row. ADD COLUMN IF NOT EXISTS, constraints guarded by a pg_constraint lookup,
--      CREATE INDEX IF NOT EXISTS, CREATE OR REPLACE FUNCTION and DROP TRIGGER IF EXISTS before CREATE
--      TRIGGER make a retry cost nothing. No per-commune loop: nothing to backfill.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. READ PATHS THAT CHANGE MEANING WHILE HALF-APPLIED: none can see it half-applied. AFTER it, before
--      the Go card: every read of phieu_phan_anh names its columns (store/phieu_phan_anh.go `cotPhieu`;
--      no `SELECT *` or `RETURNING *` anywhere in internal/), so no reader sees the column. Every
--      existing write passes: the INSERT (store/phieu_phan_anh.go:373) names its columns, so the
--      account is NULL and every new CHECK holds; no UPDATE names the column, so the freeze never
--      fires. "Phản ánh của tôi" and the citizen lookup filter `cong_dan_id = $n`
--      (store/phieu_phan_anh.go:148, :227; store/petition_rating.go:43), which is never TRUE on a
--      Zalo-owned row — so those rows can never appear there, by construction (ADR 0080 decision 3).
--      The photo floor is only WIDER for Zalo-owned rows, of which none exist yet.
--   5. RETENTION: a petition is an ARCHIVAL RECORD (rule 7). Nothing is dropped, retyped, emptied or
--      renumbered; `ma_tra_cuu` and 0004's guard are untouched. The new column is not personal data in
--      itself (an opaque id), but it LINKS to identity's Zalo account — the API never returns it, and
--      it never enters a log line, an error message or an event payload. No message in this file names
--      a value.
-- ---------------------------------------------------------------------------

ALTER TABLE phieu_phan_anh ADD COLUMN IF NOT EXISTS zalo_account_id TEXT;

-- ---------------------------------------------------------------------------
-- THE CONSTRAINTS. DO $$ … $$ because ADD CONSTRAINT has no IF NOT EXISTS (question 2). The lookup is
-- bound to this table's oid: a CHECK on a partitioned parent is copied to every partition under the
-- same name, and a bare conname match could be satisfied by another table's constraint.
-- ---------------------------------------------------------------------------
DO $$ BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid = 'phieu_phan_anh'::regclass
          AND conname = 'phieu_phan_anh_zalo_account_id_valid'
    ) THEN
        ALTER TABLE phieu_phan_anh ADD CONSTRAINT phieu_phan_anh_zalo_account_id_valid
            CHECK (zalo_account_id IS NULL OR btrim(zalo_account_id) <> '');
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid = 'phieu_phan_anh'::regclass
          AND conname = 'phieu_phan_anh_single_owner'
    ) THEN
        ALTER TABLE phieu_phan_anh ADD CONSTRAINT phieu_phan_anh_single_owner
            CHECK (NOT (cong_dan_id IS NOT NULL AND zalo_account_id IS NOT NULL));
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid = 'phieu_phan_anh'::regclass
          AND conname = 'phieu_phan_anh_zalo_owner_mini_app'
    ) THEN
        ALTER TABLE phieu_phan_anh ADD CONSTRAINT phieu_phan_anh_zalo_owner_mini_app
            CHECK (zalo_account_id IS NULL OR kenh_tiep_nhan = 'zalo-mini-app');
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid = 'phieu_phan_anh'::regclass
          AND conname = 'phieu_phan_anh_zalo_owner_no_rating_reopen'
    ) THEN
        ALTER TABLE phieu_phan_anh ADD CONSTRAINT phieu_phan_anh_zalo_owner_no_rating_reopen
            CHECK (zalo_account_id IS NULL
                   OR (diem_hai_long IS NULL AND danh_gia_luc IS NULL
                       AND rating_comment IS NULL AND so_lan_mo_lai = 0));
    END IF;
END $$;

-- The 10-per-day ceiling's count (decision 7). See THE INDEX above for the columns and the predicate.
CREATE INDEX IF NOT EXISTS phieu_phan_anh_zalo_account
    ON phieu_phan_anh (tenant_id, zalo_account_id, vao_so_luc DESC)
    WHERE zalo_account_id IS NOT NULL;

COMMENT ON COLUMN phieu_phan_anh.zalo_account_id IS
    'Owner of a petition sent from a Mini App session WITHOUT a verified phone (ADR 0080): '
    'tai_khoan_zalo.id in service-identity, opaque, no FK (rule 2). NULL on every citizen-filed '
    '(cong_dan_id set) and staff-booked petition; never both owners (phieu_phan_anh_single_owner). '
    'NOT NULL here IS the server''s "self-declared contact, unverified" fact — no separate flag column. '
    'Written at intake, frozen afterwards. Never returned by the API, never logged.';

-- ---------------------------------------------------------------------------
-- phieu_phan_anh_zalo_owner_frozen — the owner is written by the INSERT and never by an UPDATE.
--
-- A SEPARATE FUNCTION AND TRIGGER, NOT A REPLACEMENT OF 0004's ho_so_luu_tru_bat_bien: three files
-- (0011, 0013, 0017) declare that function unchanged, and its tests pin that. Two BEFORE UPDATE
-- triggers on one table each refuse independently, so their order changes no outcome (0011:234-236).
--
-- WHAT IT DOES NOT COVER, said plainly: an erasure request under Decree 13 (rule 3 invariant 7) that
-- would anonymise the owner. Nobody has decided whether an opaque account id is an identifying field
-- to replace; until then this refuses, and the way through is a new migration with that decision.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION phieu_phan_anh_zalo_owner_frozen() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.zalo_account_id IS DISTINCT FROM OLD.zalo_account_id THEN
        RAISE EXCEPTION 'archival record %: `zalo_account_id` is immutable', TG_TABLE_NAME
            USING HINT = 'The owner of an unverified petition is fixed at intake (ADR 0080). Setting, '
                         'clearing or moving it later hands a record to another person, or turns it '
                         'into a citizen-identity petition — ADR 0080 cost #5, ADR 0020 CÒN MỞ #1.';
    END IF;
    RETURN NEW;
END $$;

DROP TRIGGER IF EXISTS phieu_phan_anh_zalo_owner_frozen ON phieu_phan_anh;
CREATE TRIGGER phieu_phan_anh_zalo_owner_frozen
    BEFORE UPDATE ON phieu_phan_anh
    FOR EACH ROW EXECUTE FUNCTION phieu_phan_anh_zalo_owner_frozen();

-- ---------------------------------------------------------------------------
-- stored_file_petition_check — 0026's floor, replaced in place under the same name. The trigger
-- (0026, BEFORE INSERT OR UPDATE OF status) is NOT touched; only the function it calls.
--
-- WHAT THE OLD BODY COMPARED, measured, not assumed: NOT the uploader. `stored_file` carries no
-- uploader identity for a citizen — `uploaded_by` is the fixed marker 'cong-dan' (0026's header) and
-- the real actor goes to audit_log. The trigger checked only that the PETITION has a citizen owner;
-- that the session IS that owner is the use case's check (rule 4 invariant 2), and the signed read
-- link binds to the petition's owner column, not to a column of `stored_file`.
--
-- SO NO NEW COLUMN ON stored_file. An `uploader_zalo_account_id` there would be the internal-id-on-a-
-- staff-read row 0026 refused for the citizen id (ADR 0008: it links anonymous reports), and a second
-- copy of the petition's owner. The change is the one condition: "a petition with a citizen owner"
-- becomes "a petition with an owner a citizen session can hold" — `cong_dan_id` OR `zalo_account_id`.
-- Neither → refused, as before (fail closed). Staff uploads (uploaded_by = a staff code) never entered
-- that branch and still do not.
--
-- Everything else is 0026:119-166 VERBATIM: the subject filter, the stored-set entry, the commune and
-- soft-delete filter, the FOR UPDATE lock, the limit of 5 and its count — Zalo-owned petitions share
-- the same 5 slots, uploaded under the same marker.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION stored_file_petition_check() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
    petition_citizen TEXT;
    petition_zalo    TEXT;
    live_photos      INT;
BEGIN
    IF NEW.subject_type <> 'petition' THEN
        RETURN NEW;
    END IF;

    -- Only what changes the answer re-runs the check: a new row, or a row entering the stored set.
    IF TG_OP = 'UPDATE'
       AND NOT (NEW.status IN ('stored', 'processing', 'ready')
                AND OLD.status NOT IN ('stored', 'processing', 'ready')) THEN
        RETURN NEW;
    END IF;

    SELECT p.cong_dan_id, p.zalo_account_id INTO petition_citizen, petition_zalo
      FROM phieu_phan_anh p
     WHERE p.tenant_id = NEW.tenant_id AND p.id = NEW.subject_id AND p.deleted_at IS NULL
       FOR UPDATE;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'stored_file: petition not found in this commune';
    END IF;

    IF NEW.uploaded_by = 'cong-dan'
       AND (petition_citizen IS NULL OR btrim(petition_citizen) = '')
       AND (petition_zalo IS NULL OR btrim(petition_zalo) = '') THEN
        RAISE EXCEPTION 'stored_file: a citizen upload needs a petition owned by a citizen session'
            USING HINT = 'phieu_phan_anh.cong_dan_id and zalo_account_id are both empty: no citizen '
                         'session can own this petition (0026, ADR 0080).';
    END IF;

    IF NEW.purpose = 'petition-photo' AND NEW.uploaded_by = 'cong-dan'
       AND NEW.deleted_at IS NULL AND NEW.status IN ('stored', 'processing', 'ready') THEN
        SELECT count(*) INTO live_photos
          FROM stored_file f
         WHERE f.tenant_id = NEW.tenant_id
           AND f.subject_type = 'petition' AND f.subject_id = NEW.subject_id
           AND f.uploaded_by = 'cong-dan' AND f.purpose = 'petition-photo'
           AND f.deleted_at IS NULL
           AND f.status IN ('stored', 'processing', 'ready')
           AND f.id <> NEW.id;
        IF live_photos >= 5 THEN
            RAISE EXCEPTION 'stored_file: a petition holds at most 5 citizen scene photos'
                USING HINT = 'Owner decision 02/10/2026; platform upload_policy petition-photo.';
        END IF;
    END IF;

    RETURN NEW;
END $$;

-- ---------------------------------------------------------------------------
-- MEASUREMENT (question 1): petitions per commune, and how many carry no citizen (staff-booked; they
-- keep a NULL owner). Counts and tenant ids only.
-- ---------------------------------------------------------------------------
DO $$
DECLARE r record;
BEGIN
    FOR r IN
        SELECT tenant_id,
               count(*) AS total,
               count(*) FILTER (WHERE cong_dan_id IS NULL) AS no_citizen
        FROM phieu_phan_anh
        GROUP BY tenant_id
        ORDER BY tenant_id
    LOOP
        RAISE NOTICE '0032: commune % holds % petition(s), % with no citizen (zalo_account_id stays NULL on all)',
            r.tenant_id, r.total, r.no_citizen;
    END LOOP;
END $$;

-- ---------------------------------------------------------------------------
-- WHAT THIS FILE DOES NOT STOP, said plainly: DDL by the table owner (dropping the constraints,
-- DISABLE TRIGGER), TRUNCATE. Same line ADR 0013 draws for the audit ledger. Nor does it check that
-- the uploading session IS the owner — no column here can; that is the use case's (rule 4).
--
-- REVERSAL (rule 7, invariant 4). Run by a person, in ONE transaction; core/migrate has no automatic
-- rollback (ADR 0013).
--
--   Lossless ONLY WHILE NO PETITION IS ZALO-OWNED:
--
--     SELECT count(*) FROM phieu_phan_anh WHERE zalo_account_id IS NOT NULL;   -- must be 0
--
--   ZERO → in this order (restore the photo function BEFORE dropping the column: the replaced body
--   names it):
--     re-run 0026's CREATE OR REPLACE FUNCTION stored_file_petition_check() verbatim (0026:119-166);
--     DROP TRIGGER IF EXISTS phieu_phan_anh_zalo_owner_frozen ON phieu_phan_anh;
--     DROP FUNCTION IF EXISTS phieu_phan_anh_zalo_owner_frozen();
--     DROP INDEX IF EXISTS phieu_phan_anh_zalo_account;
--     ALTER TABLE phieu_phan_anh DROP CONSTRAINT phieu_phan_anh_zalo_owner_no_rating_reopen;
--     ALTER TABLE phieu_phan_anh DROP CONSTRAINT phieu_phan_anh_zalo_owner_mini_app;
--     ALTER TABLE phieu_phan_anh DROP CONSTRAINT phieu_phan_anh_single_owner;
--     ALTER TABLE phieu_phan_anh DROP CONSTRAINT phieu_phan_anh_zalo_account_id_valid;
--     ALTER TABLE phieu_phan_anh DROP COLUMN zalo_account_id;
--   and remove this file's row from `schema_migration`, otherwise the runner still believes the schema
--   is in place.
--
--   NON-ZERO → an unverified petition's ONLY owner is in that column: dropping it makes the petition
--   unreachable by the citizen who sent it and leaves the "unverified" label with nothing to read —
--   part of an archival record destroyed. Rule 7 stop condition #2: a user decision plus a verified
--   backup, never a command. Restoring only 0026's function is lossless at any time, but then refuses
--   further photos on those petitions; that too is the user's call.
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
