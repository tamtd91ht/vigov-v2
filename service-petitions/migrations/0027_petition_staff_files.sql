-- 0027 — two kinds of STAFF file on a petition (menu phan-anh-nguoi-dan). Schema only: no row is written.
--
--   'petition-verification-photo' — the "after processing" photo (ảnh sau xử lý, docs/ui-ux/09 §8.4,
--       §14.2 :339). Owner decision C of 02/10/2026, reversing deferral G8 (ADR 0047:255): staff upload
--       them, the citizen will SEE them on their own petition in the commune app. At most 5 per
--       petition. PRIVATE, never public.
--   'petition-log-attachment' — a file attached to a petition processing-log entry (nhat_ky_phan_anh,
--       docs/ui-ux/09 :197, :312). Owner decision B of 02/10/2026: STAFF-ONLY, never shown to a citizen,
--       like the log it hangs off (0013: "IT IS STAFF-INTERNAL").
--
-- Size, MIME and the platform-side count are service-platform 0015's, SET BY PRECEDENT 02/10/2026 and
-- changeable by a later migration there. The 5 written below is the floor under that count, and
-- follows the same precedent (the citizen photo's 5, 0026); changing it is a later migration here too.
--
-- WHICH CLASS, AND WHY `records` FOR BOTH. ADR 0052 §6 has three retention classes for private files:
-- `citizen-media` ("files a citizen uploads", 24 months after the petition closes — placeholder),
-- `content-source` (Mini App sources) and `records` (administrative records, never purged
-- automatically). Neither file is a citizen's upload; both are what an OFFICER put on an archival
-- record as evidence of the commune's own act — exactly task-attachment's standing (0021, core/storage
-- key.go: "staff-uploaded work evidence … ClassRecords"). The log attachment is that analogy one to
-- one. The verification photo is the evidence a petition was closed on (ADR 0008 #3) — purging it after
-- 24 months would leave a closed petition whose closing condition can no longer be shown.
-- COST, stated: `records` objects cannot be produced by the server (core/storage PutServerProduced
-- refuses the class, ADR 0052 §1 second addendum), so the citizen-photo path of re-encoding to strip
-- EXIF is NOT available to these files as the code stands. Whether staff photos must be EXIF-stripped
-- before a citizen sees them is a write-path question this file does not decide.
--
-- NO NEW ROW SHAPE FOR THE UPLOADER: `uploaded_by` is the staff BUSINESS CODE (rule 6, invariant 8), as
-- for task files. 0026's stored_file_citizen_upload_shape already keeps the citizen marker 'cong-dan'
-- off both purposes (a citizen uploads `petition-photo` and nothing else), and is not touched.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. ROWS PER COMMUNE: zero are written. The two ADD CONSTRAINTs validate every existing
--      `stored_file` row; both are vacuous for every row today (no row carries either new purpose — no
--      code path can issue one before this release). The NOTICE lines below measure rows per commune,
--      subject and purpose (counts and tenant ids only). `petition_log_attachment` is new and empty.
--      Validation takes ACCESS EXCLUSIVE on `stored_file` for one scan.
--   2. IF IT STOPS HALF-WAY: it cannot land half-applied — one file, one transaction (core/migrate),
--      with its progress row. Every statement is DROP … IF EXISTS / CREATE … IF NOT EXISTS / CREATE OR
--      REPLACE, so a retry costs nothing. No per-commune loop: nothing to backfill.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. READ PATHS THAT CHANGE MEANING WHILE HALF-APPLIED: none can see it half-applied. AFTER it, a
--      `petition` subject may hold STAFF rows beside the citizen's photos. Who reads petition rows:
--        * store/petition_photo.go PetitionPhotos — binds purpose = 'petition-photo' AND
--          uploaded_by = 'cong-dan', so a staff file never appears among the citizen's photos, on the
--          staff screen or in the citizen app;
--        * store/stored_file.go CountForSubject(Tx) — keyed on purpose, so staff files never consume
--          the citizen's 5 slots (and vice versa);
--        * 0026's stored_file_petition_check — counts `uploaded_by = 'cong-dan' AND purpose =
--          'petition-photo'` only; it also requires the petition to exist in this commune, unchanged;
--        * task_log_attachment_check (0021) refuses `subject_type <> 'task'`;
--        * store ForUpdate / ByID read by id; the task use case re-checks the subject (0026 Q4).
--      THE ONE PATH TO WATCH when it is built: a citizen-facing read of verification photos must bind
--      purpose = 'petition-verification-photo' explicitly — never "every petition file", which would
--      hand the citizen the staff-only log attachments (rule 4, forbidden #5).
--   5. RETENTION: both are parts of an archival record. `records` class: retain_until stays NULL and
--      disposal follows the records schedule, never a command (0021's guard refuses purging a stored
--      `records` file). stored_file is soft delete only (0021's stored_file_guard refuses DELETE, not
--      touched). petition_log_attachment is APPEND-ONLY like the log it belongs to; hiding a file is a
--      soft delete of its stored_file row.
--
-- ---------------------------------------------------------------------------

-- A VERIFICATION PHOTO IS A PRIVATE RECORD ON A PETITION, ALWAYS. "Never public" is a column value here,
-- not a promise of the write path. The object-key CHECK of 0021 then forces the key to start with
-- `records/t_<tenant>/` and to carry `/petitions/petition-verification-photo/<id>/`.
ALTER TABLE stored_file DROP CONSTRAINT IF EXISTS stored_file_petition_verification_photo_shape;
ALTER TABLE stored_file ADD CONSTRAINT stored_file_petition_verification_photo_shape
    CHECK (purpose <> 'petition-verification-photo'
           OR (subject_type = 'petition' AND retention_class = 'records' AND bucket = 'private'));

-- A LOG ATTACHMENT IS A PRIVATE RECORD ON A PETITION, ALWAYS — same reasoning, same shape.
ALTER TABLE stored_file DROP CONSTRAINT IF EXISTS stored_file_petition_log_attachment_shape;
ALTER TABLE stored_file ADD CONSTRAINT stored_file_petition_log_attachment_shape
    CHECK (purpose <> 'petition-log-attachment'
           OR (subject_type = 'petition' AND retention_class = 'records' AND bucket = 'private'));

-- ---------------------------------------------------------------------------
-- stored_file_verification_photo_check — AT MOST 5 VERIFICATION PHOTOS per petition, per commune, that
-- REACHED THE DESTINATION (not deleted, status `stored` / `processing` / `ready`), counted when a row
-- ENTERS that set (INSERT, or the `scanning → stored` edge). 0026's floor, for the staff purpose, in
-- its OWN function so the two counts can never be confused and this file reverses by dropping only
-- what it created. The petition row is locked FOR UPDATE, so two completions on one petition are
-- serialised and cannot both pass at 4 (0026's trigger, which fires first by name, takes the same
-- lock; taking it again in one transaction costs nothing and keeps this check self-standing).
--
-- WHY STORED ROWS ONLY, NOT PENDING: 0026's reason, unchanged — an abandoned pending row would hold a
-- slot for ever. The issue-time limit, pending included, is platform's max_files_per_subject.
--
-- Messages name the relation and the rule, never an id or a code (rule 3, forbidden #3).
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION stored_file_verification_photo_check() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
    live_photos INT;
BEGIN
    IF NEW.purpose <> 'petition-verification-photo'
       OR NEW.deleted_at IS NOT NULL
       OR NEW.status NOT IN ('stored', 'processing', 'ready') THEN
        RETURN NEW;
    END IF;

    -- Only what changes the answer re-runs the check: a new row, or a row entering the stored set.
    IF TG_OP = 'UPDATE' AND OLD.status IN ('stored', 'processing', 'ready') THEN
        RETURN NEW;
    END IF;

    PERFORM 1
       FROM phieu_phan_anh p
      WHERE p.tenant_id = NEW.tenant_id AND p.id = NEW.subject_id AND p.deleted_at IS NULL
        FOR UPDATE;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'stored_file: petition not found in this commune';
    END IF;

    SELECT count(*) INTO live_photos
      FROM stored_file f
     WHERE f.tenant_id = NEW.tenant_id
       AND f.subject_type = 'petition' AND f.subject_id = NEW.subject_id
       AND f.purpose = 'petition-verification-photo'
       AND f.deleted_at IS NULL
       AND f.status IN ('stored', 'processing', 'ready')
       AND f.id <> NEW.id;
    IF live_photos >= 5 THEN
        RAISE EXCEPTION 'stored_file: a petition holds at most 5 verification photos'
            USING HINT = 'Set by precedent 02/10/2026 (the citizen photo limit); platform upload_policy '
                         'petition-verification-photo.';
    END IF;

    RETURN NEW;
END $$;

-- BEFORE INSERT OR UPDATE OF status: purpose, subject and uploader are frozen by 0021's guard and
-- `deleted_at` is write-once there, so the status column is the only way an existing row can enter the
-- counted set. Row-level, so PostgreSQL clones it onto every partition (PG ≥ 13).
DROP TRIGGER IF EXISTS stored_file_verification_photo_check ON stored_file;
CREATE TRIGGER stored_file_verification_photo_check
    BEFORE INSERT OR UPDATE OF status ON stored_file
    FOR EACH ROW EXECUTE FUNCTION stored_file_verification_photo_check();

-- ---------------------------------------------------------------------------
-- @entity: PetitionLogAttachment
-- @scope:  tenant
--
-- petition_log_attachment — "this file was written with this petition log entry". The petition twin of
-- 0021's task_log_attachment, for 0021's reasons, which are not repeated: APPEND-ONLY like the log it
-- hangs off (rule 7, forbidden #5) and therefore with no soft-delete columns — hiding a file later is a
-- soft delete of its `stored_file` row, which every reader of this table must honour; the link itself
-- says what was attached, and that stays true. The primary key is (tenant_id, stored_file_id): a file
-- belongs to at most ONE entry.
--
-- STAFF-INTERNAL, like `nhat_ky_phan_anh` (0013). A citizen-facing read of this table is a design
-- error, not a feature (rule 4, forbidden #5; rule 10, invariant 7).
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS petition_log_attachment (
    tenant_id       TEXT        NOT NULL,
    stored_file_id  TEXT        NOT NULL,
    -- `nhat_ky_phan_anh.id`. Not a foreign key, for 0013's reason; the trigger below checks it.
    log_entry_id    TEXT        NOT NULL,
    attached_at     TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (tenant_id, stored_file_id)
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS petition_log_attachment_p%s PARTITION OF petition_log_attachment '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

-- The drawer reads one page of entries, then their attachments in ONE batched statement.
CREATE INDEX IF NOT EXISTS petition_log_attachment_by_entry
    ON petition_log_attachment (tenant_id, log_entry_id, attached_at);

-- ---------------------------------------------------------------------------
-- petition_log_attachment_check — the floor under the write path, for every writer (0021's, adapted):
--
--   * the entry exists IN THIS COMMUNE and was written IN THIS TRANSACTION (`tao_luc = now()`, 0021
--     explains the instant and its limit). A link added later is an edit of a historical record;
--   * the file exists IN THIS COMMUNE, was uploaded FOR THIS ENTRY'S PETITION, BY THIS ENTRY'S AUTHOR,
--     AS A LOG ATTACHMENT, is not soft-deleted, and reached the destination (`stored` / `ready`).
--     The purpose is checked here although 0021 does not check it for tasks: a petition holds the
--     citizen's photos and the citizen-visible verification photos beside it, and linking one of
--     those to a staff-only log entry would make one file mean two things to two audiences;
--   * the file row is locked FOR SHARE, so a concurrent soft delete waits for this transaction.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION petition_log_attachment_check() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
    entry_petition_id TEXT;
    entry_author      TEXT;
    entry_created_at  TIMESTAMPTZ;
    file_row          RECORD;
BEGIN
    SELECT l.phieu_phan_anh_id, l.nguoi_ma, l.tao_luc
      INTO entry_petition_id, entry_author, entry_created_at
      FROM nhat_ky_phan_anh l
     WHERE l.tenant_id = NEW.tenant_id AND l.id = NEW.log_entry_id;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'petition_log_attachment: log entry not found in this commune';
    END IF;
    IF entry_created_at <> now() THEN
        RAISE EXCEPTION 'petition_log_attachment: an attachment is written with its log entry, never later'
            USING HINT = 'The log is append-only (0013); attaching to an older entry edits it.';
    END IF;

    SELECT f.subject_type, f.subject_id, f.uploaded_by, f.purpose, f.status, f.deleted_at
      INTO file_row
      FROM stored_file f
     WHERE f.tenant_id = NEW.tenant_id AND f.id = NEW.stored_file_id
       FOR SHARE;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'petition_log_attachment: stored file not found in this commune';
    END IF;
    IF file_row.deleted_at IS NOT NULL
       OR file_row.subject_type <> 'petition'
       OR file_row.subject_id <> entry_petition_id
       OR file_row.uploaded_by <> entry_author
       OR file_row.purpose <> 'petition-log-attachment' THEN
        RAISE EXCEPTION 'petition_log_attachment: file was not uploaded as a log attachment for this petition by this author';
    END IF;
    IF file_row.status NOT IN ('stored', 'ready') THEN
        RAISE EXCEPTION 'petition_log_attachment: file status % cannot be attached', file_row.status;
    END IF;

    RETURN NEW;
END $$;

DROP TRIGGER IF EXISTS petition_log_attachment_check ON petition_log_attachment;
CREATE TRIGGER petition_log_attachment_check
    BEFORE INSERT ON petition_log_attachment
    FOR EACH ROW EXECUTE FUNCTION petition_log_attachment_check();

-- The append-only guard, 0021's shape: row-level UPDATE/DELETE on the parent (cloned onto every
-- partition), TRUNCATE leaf by leaf (not allowed on a partitioned parent, statement triggers not cloned).
CREATE OR REPLACE FUNCTION petition_log_attachment_append_only() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'petition_log_attachment is append-only: % on % refused', TG_OP, TG_TABLE_NAME
        USING HINT = 'An attachment is part of its log entry (rule 7, forbidden #5). To hide a file, '
                     'soft-delete its stored_file row.';
END $$;

DROP TRIGGER IF EXISTS petition_log_attachment_no_change ON petition_log_attachment;
CREATE TRIGGER petition_log_attachment_no_change
    BEFORE UPDATE OR DELETE ON petition_log_attachment
    FOR EACH ROW EXECUTE FUNCTION petition_log_attachment_append_only();

DO $$
DECLARE part regclass;
BEGIN
    FOR part IN
        SELECT inhrelid::regclass FROM pg_inherits
        WHERE inhparent = 'petition_log_attachment'::regclass
    LOOP
        EXECUTE format('DROP TRIGGER IF EXISTS petition_log_attachment_no_truncate ON %s', part);
        EXECUTE format(
            'CREATE TRIGGER petition_log_attachment_no_truncate BEFORE TRUNCATE ON %s '
            'FOR EACH STATEMENT EXECUTE FUNCTION petition_log_attachment_append_only()', part);
    END LOOP;
END $$;

-- ---------------------------------------------------------------------------
-- MEASUREMENT (question 1): rows per commune, subject and purpose. Counts and tenant ids only.
-- ---------------------------------------------------------------------------
DO $$
DECLARE r record;
BEGIN
    FOR r IN
        SELECT tenant_id, subject_type, purpose, count(*) AS n
        FROM stored_file
        GROUP BY tenant_id, subject_type, purpose
        ORDER BY tenant_id, subject_type, purpose
    LOOP
        RAISE NOTICE '0027: commune % holds % stored_file row(s) for subject % purpose %',
            r.tenant_id, r.n, r.subject_type, r.purpose;
    END LOOP;
END $$;

-- ---------------------------------------------------------------------------
-- WHAT THIS FILE DOES NOT DECIDE, said so nobody reads it as decided:
--   * whether STAFF may upload `petition-photo` (a scene photo on a petition booked on a citizen's
--     behalf — decision A of 02/10/2026, "nhập hộ"). Nothing here allows or forbids it; 0026 binds
--     only the citizen marker;
--   * uploads to a closed petition; the stored MIME type (Go, platform 0015); EXIF on staff photos
--     (header, WHICH CLASS); TRUNCATE on `stored_file` and owner DDL (0021);
--   * the per-commune switch that makes a verification photo mandatory before closing (ADR 0008 #3) —
--     no per-commune petition settings table exists in this service, and where it lives is open.
--
-- REVERSAL (rule 7, invariant 4). Run by a person, in ONE transaction; core/migrate has no automatic
-- rollback (ADR 0013). Lossless WHILE NO ROW CARRIES EITHER PURPOSE AND NO LINK EXISTS — check
--   SELECT count(*) FROM stored_file
--    WHERE purpose IN ('petition-verification-photo', 'petition-log-attachment');  -- must be 0
--   SELECT count(*) FROM petition_log_attachment;                                    -- must be 0
-- then:
--   DROP TABLE petition_log_attachment;   (partitions and their triggers go with it)
--   DROP FUNCTION petition_log_attachment_check(), petition_log_attachment_append_only();
--   DROP TRIGGER IF EXISTS stored_file_verification_photo_check ON stored_file;
--   DROP FUNCTION stored_file_verification_photo_check();
--   ALTER TABLE stored_file DROP CONSTRAINT stored_file_petition_log_attachment_shape;
--   ALTER TABLE stored_file DROP CONSTRAINT stored_file_petition_verification_photo_shape;
--   and remove this file's row from `schema_migration`.
-- 0021's and 0026's objects are not touched by this file and stay. ONCE ONE FILE OF EITHER PURPOSE
-- EXISTS, the reversal would strand an archival record's evidence without its constraints, and the
-- link table cannot be dropped without destroying part of a historical record — rule 7's stop
-- condition, the user's decision. From then on the way back is a NEW migration.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- BACKSTOP: every partitioned table in this schema must actually have partitions (0002, §BACKSTOP)
-- — repeated because this file declares a new PARTITION BY table.
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
