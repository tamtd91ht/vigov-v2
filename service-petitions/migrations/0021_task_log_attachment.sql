-- 0021 — files attached to a task progress-log entry (docs/ui-ux/02-nhiem-vu.md §5.9 `📎 Đính kèm`,
-- data model :325 `nhat_ky_nhiem_vu … dinh_kem jsonb`): this service's own `stored_file` (ADR 0052
-- §5) and the append-only link from a log entry to the files written with it.
--
-- ENTITY OWNERSHIP is declared with the `-- @entity` / `-- @scope` marks above each CREATE TABLE
-- (ADR 0021).
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. ROWS PER COMMUNE: zero. Both tables are new, nothing is seeded, nothing is backfilled —
--      `nhat_ky_nhiem_vu.dinh_kem` is '[]' on every row today (0006: "nothing writes this column"),
--      so there is no existing attachment to carry over.
--   2. IF IT STOPS HALF-WAY: it cannot land half-applied. One file, one transaction, progress row
--      inside it. Every statement is IF NOT EXISTS / CREATE OR REPLACE / DROP TRIGGER IF EXISTS, so
--      a retry costs nothing.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. READ PATHS THAT CHANGE MEANING WHILE HALF-APPLIED: none. No existing table, column, index,
--      trigger or function is altered. `nhat_ky_nhiem_vu` is only READ by the link trigger below.
--   5. RETENTION: a task and its log are ADMINISTRATIVE RECORDS (rule 7). A file written with a log
--      entry is part of that record: the link is append-only like the log, and `stored_file` is
--      soft-deleted only, never hard-deleted. Removing the OBJECT is the purge of ADR 0052 §6, and
--      the trigger below refuses it for a `records` file that reached the private bucket.
--
-- ---------------------------------------------------------------------------
-- WHY `nhat_ky_nhiem_vu.dinh_kem` IS NOT USED, AND MUST NOT START TO BE.
--
-- ADR 0052 §1 puts file metadata in `stored_file`, with a status machine, a hash, a retention class
-- and a legal hold — none of which a JSONB array can hold with a constraint. Writing the list into
-- `dinh_kem` as well would be a second copy of the link table (rule 9, forbidden #2), and it could
-- never be corrected: the log row is append-only. The column stays '[]'; the reader of an entry's
-- attachments reads `task_log_attachment`. Dropping the column is NOT done here — it is populated
-- (with '[]') on every row, and dropping a populated column is the user's decision (rule 7, stop 2).
--
-- WHY ATTACHING IS A SEPARATE, EARLIER STEP THAN WRITING THE ENTRY.
--
-- ADR 0052 §1: the browser asks for an upload (a `pending` row here), uploads straight to MinIO,
-- then asks the service to complete it (scan, hash, copy → `stored`). All of that happens BEFORE the
-- officer presses `➤ Ghi nhật ký`, i.e. before the log entry exists. So a `stored_file` is bound at
-- issue time to ONE TASK (`subject_type = 'task'`, `subject_id` = the task's internal id) and ONE
-- COMMUNE, and the link to the log entry is written later, IN THE SAME TRANSACTION AS THE ENTRY.
-- `subject_*` never changes afterwards: a file uploaded for task A cannot be attached to task B.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- @entity: PetitionsStoredFile
-- @scope:  tenant
--
-- stored_file — one object this service put into object storage (ADR 0052 §5), in the column list
-- the ADR fixes. THE ENTITY NAME CARRIES THE SERVICE because every file-owning service keeps its own
-- `stored_file` (ADR 0052 §1, rule 2 invariant 1) and tools/kb reports one entity with two owners —
-- the same device 0020 uses for `PetitionsSystemMessageOverride`.
--
-- TWO COLUMNS THE ADR DOES NOT LIST, stated rather than smuggled in: `created_at` (the `yyyy/mm` of
-- the object key, and the age of a `pending` row that was never completed) and `updated_at` (when
-- the status last moved). ONE READING THE ADR LEAVES OPEN, also stated: `bucket` holds the bucket's
-- ROLE (`private` | `public`, core/storage.Bucket) and not its name — the name is
-- `{OBJECT_STORAGE_BUCKET_PREFIX}-{role}`, an environment setting, and a row that stored the name
-- would point at the wrong bucket the day a backup is restored into another environment.
--
-- `object_key` IS THE DESTINATION KEY (ADR 0052 §3), fixed when the upload is issued and never
-- changed. While the row is `pending` / `scanning` the bytes sit at `upload/` + object_key in the
-- temp bucket (core/storage Key.UploadPath) — derivable, so not stored twice.
--
-- `id` IS THE `{object_id}` SEGMENT OF THE KEY (a server ULID), and the CHECK below binds the two:
-- one row, one object, and the key names this commune, this service and this purpose.
--
-- `original_name` IS PERSONAL DATA WHEN IT DESCRIBES A CASE (rule 3): masked on the way out, never
-- logged, never in a key (ADR 0052 §3). It is the one descriptive column an anonymisation under
-- Decree 13 may rewrite (rule 7, invariant 7), so the guard does not freeze it.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS stored_file (
    tenant_id        TEXT        NOT NULL,
    id               TEXT        NOT NULL,

    bucket           TEXT        NOT NULL,
    object_key       TEXT        NOT NULL,

    retention_class  TEXT        NOT NULL,
    purpose          TEXT        NOT NULL,
    subject_type     TEXT        NOT NULL,
    subject_id       TEXT        NOT NULL,

    original_name    TEXT        NOT NULL,

    -- The SNIFFED type (magic bytes, never the client's Content-Type), the size and the hash, all
    -- measured by the complete step. NULL until then; write-once afterwards (guard below).
    mime_type        TEXT,
    size_bytes       BIGINT,
    sha256           TEXT,

    status           TEXT        NOT NULL DEFAULT 'pending',

    -- The staff BUSINESS CODE (`CB-00123`), never the internal id (rule 6, invariant 8). A citizen
    -- upload (ADR 0052 §12) is blocked on the citizen-session bridge and has no row shape here yet:
    -- `subject_type` admits only 'task', which only staff reach.
    uploaded_by      TEXT        NOT NULL,

    -- Fixed ONCE, at the act that fixes it (ADR 0052 §5, rule 10 invariant 2's principle); NULL for
    -- a `records` file, whose disposal follows the records schedule and is never automatic (§6).
    retain_until     TIMESTAMPTZ,
    legal_hold       BOOLEAN     NOT NULL DEFAULT false,

    purged_at        TIMESTAMPTZ,
    purged_by        TEXT,
    purge_reason     TEXT,

    deleted_at       TIMESTAMPTZ,
    deleted_by       TEXT,
    delete_reason    TEXT,

    created_at       TIMESTAMPTZ NOT NULL,
    updated_at       TIMESTAMPTZ NOT NULL,

    PRIMARY KEY (tenant_id, id),

    -- ONE ROW PER OBJECT, per commune. Composite with tenant_id (rule 1, invariant 6), although the
    -- key already carries `t_<tenant_id>` — the CHECK below is what makes those two agree.
    UNIQUE (tenant_id, object_key),

    CONSTRAINT stored_file_status_known CHECK (status IN (
        'pending', 'scanning', 'stored', 'processing', 'ready', 'failed', 'rejected', 'purged')),

    -- core/storage's closed Class list. The purge decision keys on this column, so an unknown class
    -- is refused here rather than left for the purge worker to interpret.
    CONSTRAINT stored_file_retention_class_known CHECK (retention_class IN (
        'records', 'citizen-media', 'content-source', 'public-media')),

    CONSTRAINT stored_file_bucket_known CHECK (bucket IN ('private', 'public')),

    -- The closed purpose list lives in core/storage (Purpose, checked by Key.Path) and in platform's
    -- upload policy; it is NOT copied here, so a new purpose is not a migration. The shape is.
    CONSTRAINT stored_file_purpose_shape CHECK (purpose ~ '^[a-z0-9]+(-[a-z0-9]+)*$'),

    -- Widened by a later migration when a second subject exists (a petition's scene photos).
    CONSTRAINT stored_file_subject_type_known CHECK (subject_type IN ('task')),
    CONSTRAINT stored_file_subject_id_present CHECK (btrim(subject_id) <> ''),

    -- THE KEY NAMES THIS COMMUNE, THIS SERVICE, THIS PURPOSE AND THIS ROW (ADR 0052 §3, rule 1
    -- invariant 7). A key built for another commune's tree cannot be recorded under this one.
    CONSTRAINT stored_file_object_key_shape CHECK (
        object_key ~ '^[a-z0-9_./-]+$'
        AND starts_with(object_key, retention_class || '/t_' || lower(tenant_id) || '/')
        AND strpos(object_key, '/petitions/' || purpose || '/' || lower(id) || '/') > 0),

    CONSTRAINT stored_file_original_name_shape CHECK (
        btrim(original_name) <> '' AND char_length(original_name) <= 255
        AND original_name !~ '[[:cntrl:]]'),

    CONSTRAINT stored_file_uploaded_by_present CHECK (btrim(uploaded_by) <> ''),

    CONSTRAINT stored_file_size_non_negative CHECK (size_bytes IS NULL OR size_bytes >= 0),
    CONSTRAINT stored_file_sha256_shape CHECK (sha256 IS NULL OR sha256 ~ '^[0-9a-f]{64}$'),
    CONSTRAINT stored_file_mime_present CHECK (mime_type IS NULL OR btrim(mime_type) <> ''),

    -- A file that reached the destination was measured: all three facts, or it is not stored.
    CONSTRAINT stored_file_stored_facts_complete CHECK (
        status NOT IN ('stored', 'processing', 'ready')
        OR (mime_type IS NOT NULL AND size_bytes IS NOT NULL AND sha256 IS NOT NULL)),

    -- `purged` and the three purge columns say the same thing, or neither.
    --
    -- `x IS NOT NULL AND btrim(x) <> ''`, never `btrim(x) <> ''` alone: with x NULL the latter is
    -- NULL, the whole branch is NULL, and a CHECK that evaluates to NULL PASSES — a purge with no
    -- reason would be accepted in silence.
    CONSTRAINT stored_file_purge_complete CHECK (
        (status = 'purged' AND purged_at IS NOT NULL AND purged_by IS NOT NULL
             AND purge_reason IS NOT NULL AND btrim(purge_reason) <> '')
        OR (status <> 'purged' AND purged_at IS NULL AND purged_by IS NULL AND purge_reason IS NULL)),

    CONSTRAINT stored_file_delete_complete CHECK (
        (deleted_at IS NULL AND deleted_by IS NULL AND delete_reason IS NULL)
        OR (deleted_at IS NOT NULL AND deleted_by IS NOT NULL
             AND delete_reason IS NOT NULL AND btrim(delete_reason) <> ''))
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS stored_file_p%s PARTITION OF stored_file '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

-- "The files I uploaded for this task" — the drawer's not-yet-attached list, and the per-subject
-- count platform's `max_files_per_subject` asks for (prefix tenant_id, subject_type, subject_id).
CREATE INDEX IF NOT EXISTS stored_file_by_subject_uploader
    ON stored_file (tenant_id, subject_type, subject_id, uploaded_by, created_at DESC)
    WHERE deleted_at IS NULL;

-- ---------------------------------------------------------------------------
-- stored_file_guard — what may move on a metadata row, and in which direction.
--
--   DELETE             refused: the row is the only trace that an object existed (rule 7 inv 1).
--   identity columns   frozen: which commune, which object, which class, which subject, who.
--   measured facts     write-once: a hash that changes after the scan is a hash nobody scanned.
--   retain_until       write-once: computed once at the act that fixes it (ADR 0052 §5).
--   status             only along the edges below, mirrored by domain.StoredFileStatus.CanMoveTo.
--   purge              refused under legal_hold, and refused for a `records` file whose object
--                      reached the destination (ADR 0052 §7 and ĐIỀU KIỆN DỪNG #1).
--   purge / delete     write-once: a second purge or delete cannot rewrite who did it, or why.
--
-- THE `scanning → pending` EDGE is the retry ADR 0052 §9 allows when the scanner is unreachable
-- ("từ chối hoặc cho thử lại"): the row goes back to waiting, never forward to `stored`.
--
-- The messages name the operation and the relation and nothing else (rule 3, forbidden #3).
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION stored_file_guard() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'administrative record %: hard delete refused', TG_TABLE_NAME
            USING HINT = 'File metadata is soft-deleted (deleted_at, deleted_by, delete_reason); '
                         'the object is removed only by the purge of ADR 0052 §6.';
    END IF;

    IF NEW.tenant_id       IS DISTINCT FROM OLD.tenant_id
    OR NEW.id              IS DISTINCT FROM OLD.id
    OR NEW.bucket          IS DISTINCT FROM OLD.bucket
    OR NEW.object_key      IS DISTINCT FROM OLD.object_key
    OR NEW.retention_class IS DISTINCT FROM OLD.retention_class
    OR NEW.purpose         IS DISTINCT FROM OLD.purpose
    OR NEW.subject_type    IS DISTINCT FROM OLD.subject_type
    OR NEW.subject_id      IS DISTINCT FROM OLD.subject_id
    OR NEW.uploaded_by     IS DISTINCT FROM OLD.uploaded_by
    OR NEW.created_at      IS DISTINCT FROM OLD.created_at THEN
        RAISE EXCEPTION 'administrative record %: identity columns are immutable', TG_TABLE_NAME
            USING HINT = 'Commune, object, class, purpose, subject and uploader are fixed when the '
                         'upload is issued (ADR 0052 §1a). A new version is a new object and a new row.';
    END IF;

    IF (OLD.mime_type    IS NOT NULL AND NEW.mime_type    IS DISTINCT FROM OLD.mime_type)
    OR (OLD.size_bytes   IS NOT NULL AND NEW.size_bytes   IS DISTINCT FROM OLD.size_bytes)
    OR (OLD.sha256       IS NOT NULL AND NEW.sha256       IS DISTINCT FROM OLD.sha256)
    OR (OLD.retain_until IS NOT NULL AND NEW.retain_until IS DISTINCT FROM OLD.retain_until) THEN
        RAISE EXCEPTION 'administrative record %: measured facts are write-once', TG_TABLE_NAME
            USING HINT = 'Type, size and hash are measured once at completion; retain_until is fixed '
                         'once at the act that fixes it (ADR 0052 §5).';
    END IF;

    IF NEW.status IS DISTINCT FROM OLD.status
       AND (OLD.status, NEW.status) NOT IN (
           ('pending', 'scanning'), ('pending', 'failed'), ('pending', 'rejected'),
           ('scanning', 'stored'), ('scanning', 'failed'), ('scanning', 'rejected'),
           ('scanning', 'pending'),
           ('stored', 'processing'), ('stored', 'purged'),
           ('processing', 'ready'), ('processing', 'failed'),
           ('ready', 'purged'),
           ('failed', 'purged'),
           ('rejected', 'purged')) THEN
        RAISE EXCEPTION 'administrative record %: status % -> % refused', TG_TABLE_NAME,
            OLD.status, NEW.status
            USING HINT = 'ADR 0052 §5: pending -> scanning -> stored -> processing -> ready | failed | '
                         'rejected -> purged.';
    END IF;

    IF NEW.status = 'purged' AND OLD.status <> 'purged' THEN
        IF NEW.legal_hold THEN
            RAISE EXCEPTION 'administrative record %: purge refused under legal hold', TG_TABLE_NAME
                USING HINT = 'A file held for a complaint or an inspection is never purged (ADR 0052 §5).';
        END IF;
        IF NEW.retention_class = 'records' AND OLD.status IN ('stored', 'processing', 'ready') THEN
            RAISE EXCEPTION 'administrative record %: a records file is never purged', TG_TABLE_NAME
                USING HINT = 'Disposal of a records file follows the records schedule, never a command '
                             '(ADR 0052 §6, §7; rule 7 stop condition 1).';
        END IF;
    END IF;

    IF (OLD.purged_at IS NOT NULL
            AND (NEW.purged_at    IS DISTINCT FROM OLD.purged_at
              OR NEW.purged_by    IS DISTINCT FROM OLD.purged_by
              OR NEW.purge_reason IS DISTINCT FROM OLD.purge_reason))
    OR (OLD.deleted_at IS NOT NULL
            AND (NEW.deleted_at    IS DISTINCT FROM OLD.deleted_at
              OR NEW.deleted_by    IS DISTINCT FROM OLD.deleted_by
              OR NEW.delete_reason IS DISTINCT FROM OLD.delete_reason)) THEN
        RAISE EXCEPTION 'administrative record %: purge and delete facts are write-once', TG_TABLE_NAME
            USING HINT = 'Who removed a file, when and why is written once (rule 7, rule 6).';
    END IF;

    RETURN NEW;
END $$;

DROP TRIGGER IF EXISTS stored_file_guard ON stored_file;
CREATE TRIGGER stored_file_guard
    BEFORE UPDATE OR DELETE ON stored_file
    FOR EACH ROW EXECUTE FUNCTION stored_file_guard();

-- ---------------------------------------------------------------------------
-- @entity: TaskLogAttachment
-- @scope:  tenant
--
-- task_log_attachment — "this file was written with this log entry". APPEND-ONLY, like the log it
-- hangs off (rule 7, forbidden #5), and with no soft-delete columns for the same reason: an entry
-- and the files written with it are ONE historical record. Hiding a file later is a soft delete of
-- its `stored_file` row (an administrative path not built here), which every reader of this table
-- honours; the link itself says what was attached, and that stays true.
--
-- WHY THE PRIMARY KEY IS (tenant_id, stored_file_id) AND NOT (tenant_id, log_entry_id, …): a file
-- belongs to at most ONE entry. Two entries showing one object is one upload quoted twice, and the
-- second quote is an attachment its author never uploaded for that entry.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS task_log_attachment (
    tenant_id       TEXT        NOT NULL,
    stored_file_id  TEXT        NOT NULL,
    -- `nhat_ky_nhiem_vu.id`. Not a foreign key, for 0006's reason; the trigger below checks it.
    log_entry_id    TEXT        NOT NULL,
    attached_at     TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (tenant_id, stored_file_id)
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS task_log_attachment_p%s PARTITION OF task_log_attachment '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

-- The timeline reads one page of entries, then their attachments in ONE batched statement.
CREATE INDEX IF NOT EXISTS task_log_attachment_by_entry
    ON task_log_attachment (tenant_id, log_entry_id, attached_at);

-- ---------------------------------------------------------------------------
-- task_log_attachment_check — the floor under the write path, for every writer.
--
--   * the entry exists IN THIS COMMUNE, and was written IN THIS TRANSACTION. A link added later is
--     an edit of a historical record. "This transaction" is tested as `tao_luc = now()`: the entry's
--     `tao_luc` defaults to now(), and now() is the transaction's start instant — the same value for
--     every statement of one transaction. (It is an instant, not a transaction id: two transactions
--     starting in the same microsecond would pass. The write path is the primary guard.)
--   * the file exists IN THIS COMMUNE, was uploaded FOR THIS ENTRY'S TASK and BY THIS ENTRY'S
--     AUTHOR, is not soft-deleted, and reached the destination (`stored` or `ready`). A pending or
--     rejected file is not something an officer can have read before attaching it.
--   * the file row is locked FOR SHARE, so a concurrent soft delete waits for this transaction.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION task_log_attachment_check() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
    entry_task_id    TEXT;
    entry_author     TEXT;
    entry_created_at TIMESTAMPTZ;
    file_row         RECORD;
BEGIN
    SELECT l.nhiem_vu_id, l.nguoi_ma, l.tao_luc
      INTO entry_task_id, entry_author, entry_created_at
      FROM nhat_ky_nhiem_vu l
     WHERE l.tenant_id = NEW.tenant_id AND l.id = NEW.log_entry_id;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'task_log_attachment: log entry not found in this commune';
    END IF;
    IF entry_created_at <> now() THEN
        RAISE EXCEPTION 'task_log_attachment: an attachment is written with its log entry, never later'
            USING HINT = 'The log is append-only (0006); attaching to an older entry edits it.';
    END IF;

    SELECT f.subject_type, f.subject_id, f.uploaded_by, f.status, f.deleted_at
      INTO file_row
      FROM stored_file f
     WHERE f.tenant_id = NEW.tenant_id AND f.id = NEW.stored_file_id
       FOR SHARE;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'task_log_attachment: stored file not found in this commune';
    END IF;
    IF file_row.deleted_at IS NOT NULL
       OR file_row.subject_type <> 'task'
       OR file_row.subject_id <> entry_task_id
       OR file_row.uploaded_by <> entry_author THEN
        RAISE EXCEPTION 'task_log_attachment: file was not uploaded for this task by this author';
    END IF;
    IF file_row.status NOT IN ('stored', 'ready') THEN
        RAISE EXCEPTION 'task_log_attachment: file status % cannot be attached', file_row.status;
    END IF;

    RETURN NEW;
END $$;

DROP TRIGGER IF EXISTS task_log_attachment_check ON task_log_attachment;
CREATE TRIGGER task_log_attachment_check
    BEFORE INSERT ON task_log_attachment
    FOR EACH ROW EXECUTE FUNCTION task_log_attachment_check();

-- The append-only guard, the shape of 0006's `nhat_ky_nhiem_vu_chi_them`: row-level UPDATE/DELETE on
-- the parent (cloned onto every partition), TRUNCATE leaf by leaf (not allowed on a partitioned
-- parent, and statement triggers are not cloned).
CREATE OR REPLACE FUNCTION task_log_attachment_append_only() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'task_log_attachment is append-only: % on % refused', TG_OP, TG_TABLE_NAME
        USING HINT = 'An attachment is part of its log entry (rule 7, forbidden #5). To hide a file, '
                     'soft-delete its stored_file row.';
END $$;

DROP TRIGGER IF EXISTS task_log_attachment_no_change ON task_log_attachment;
CREATE TRIGGER task_log_attachment_no_change
    BEFORE UPDATE OR DELETE ON task_log_attachment
    FOR EACH ROW EXECUTE FUNCTION task_log_attachment_append_only();

DO $$
DECLARE part regclass;
BEGIN
    FOR part IN
        SELECT inhrelid::regclass FROM pg_inherits
        WHERE inhparent = 'task_log_attachment'::regclass
    LOOP
        EXECUTE format('DROP TRIGGER IF EXISTS task_log_attachment_no_truncate ON %s', part);
        EXECUTE format(
            'CREATE TRIGGER task_log_attachment_no_truncate BEFORE TRUNCATE ON %s '
            'FOR EACH STATEMENT EXECUTE FUNCTION task_log_attachment_append_only()', part);
    END LOOP;
END $$;

-- ---------------------------------------------------------------------------
-- WHAT THIS FILE DOES NOT STOP: TRUNCATE on `stored_file`, and DDL by the table owner — the line
-- 0006 and ADR 0013 draw.
--
-- REVERSAL. Lossless while both tables are empty, which they are everywhere today: in one
-- transaction DROP TABLE task_log_attachment, stored_file (partitions and triggers go with them),
-- DROP FUNCTION task_log_attachment_check(), task_log_attachment_append_only(), stored_file_guard(),
-- and remove this file's row from `schema_migration`. ONCE ONE FILE IS ATTACHED, that is the
-- destruction of part of an administrative record and of the only index to objects still in MinIO —
-- rule 7's stop condition, the user's decision, and from then on a NEW migration.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- BACKSTOP: every partitioned table in this schema must actually have partitions (0002, §BACKSTOP)
-- — repeated because it only verifies the state after a file that carries it.
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
