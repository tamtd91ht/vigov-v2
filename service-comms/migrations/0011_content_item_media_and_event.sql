-- 0011 — Mini App content gains an uploaded COVER IMAGE, a FIRST-PUBLISH INSTANT, an EVENT window
-- and place, and a VIDEO LINK. This service's own `stored_file` (ADR 0052 §5) is created for the
-- first of the four.
--
-- WHY THIS FILE EXISTS. User decisions 30/09/2026, ADR 0047 §6
-- (kb/10-decisions/0047-hai-luong-dung-citizen-app-theo-ten-mien.md:253-254):
--
--   (1)  news image: uploaded to object storage (ADR 0052), derivative in the PUBLIC bucket, the
--        public route returns `image_url`.
--   (2)  publish time; event time and place.       (docs/ui-ux/11-noi-dung-mini-app.md:131)
--   (3)  Video: a link that opens outside.          Audio is a LATER card — no audio column here.
--   G1   the publish instant is written at the FIRST publish and never changes; `ngay_dang` stays.
--
-- 0006:248-254 declined §7's closing note because "nên bổ sung" was the specification proposing to
-- itself. The user has now decided it, so the reason 0006 gave no longer holds. Banner columns
-- (link, order) from the same note are NOT decided and are NOT added.
--
-- WHY A NEW FILE AND NOT AN EDIT OF 0006: core/migrate compares the checksum of every applied file at
-- startup (ErrChecksumLech).
--
-- ENTITY OWNERSHIP is declared with the `-- @entity` / `-- @scope` marks above CREATE TABLE (ADR 0021).
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. ROWS PER COMMUNE: `stored_file` is new and empty. `noi_dung_mini_app` gains six NULLABLE
--      columns with no default — PostgreSQL records them in the catalogue and REWRITES NO ROW. NO ROW
--      IS WRITTEN: there is no backfill (see `published_at` below). The ADD CONSTRAINT statements scan
--      the register once to validate; every value they read is NULL.
--   2. IF IT STOPS HALF-WAY: it cannot land half-applied — one file, one transaction, progress row
--      inside it. Every statement is IF NOT EXISTS / CREATE OR REPLACE / DROP TRIGGER IF EXISTS or a
--      constraint added behind a pg_constraint lookup, so a retry costs nothing.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. READ PATHS THAT CHANGE MEANING WHILE HALF-APPLIED: none can see it half-applied (one
--      transaction). BETWEEN THIS FILE AND THE GO CHANGE: internal/store names its columns
--      explicitly (no `SELECT *` anywhere in service-comms), so no read path sees the new columns
--      until it asks for them. Every new column is NULL on every row, so every new CHECK and both
--      trigger additions are inert until a writer sets one. `anh_dai_dien_url` keeps its meaning and
--      its readers; nothing here reads or writes it.
--   5. RETENTION: a published article is business data (0006 question 5). No column is dropped,
--      retyped or emptied; `anh_dai_dien_url` is KEPT (rule 7). The 0006 immutability rules are all
--      re-stated verbatim in the replaced trigger function below; G1 is added to them. No business
--      code is issued or renumbered.
--
-- THE LOCK: ADD COLUMN and ADD CONSTRAINT take ACCESS EXCLUSIVE on `noi_dung_mini_app` and its 32
-- partitions until COMMIT. Acceptable at today's size (zero to low thousands of rows per commune,
-- 0006 question 1). THIS IS AN ASSUMPTION, NOT A MEASUREMENT; check first:
--   SELECT count(*), pg_size_pretty(pg_total_relation_size('noi_dung_mini_app')) FROM noi_dung_mini_app;
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- @entity: CommsStoredFile
-- @scope:  tenant
--
-- stored_file — one object THIS service put into object storage (ADR 0052 §5). THE ENTITY NAME
-- CARRIES THE SERVICE because every file-owning service keeps its own `stored_file` (ADR 0052 §1,
-- rule 2 invariant 1) and tools/kb would otherwise report one entity with two owners — the device
-- service-petitions/migrations/0021_task_log_attachment.sql:46 uses for `PetitionsStoredFile`.
--
-- THE SHAPE IS 0021's, column for column, so a reader who has met it once does not reason twice.
-- WHAT DIFFERS, and why:
--
--   * `subject_type` admits only 'content-item' (`noi_dung_mini_app.id`).
--   * `bucket` admits only 'private' and `retention_class` only 'content-source': the ORIGINAL is
--     always private (ADR 0052 §2, "bản gốc luôn ở private"). The public copy is not a second row —
--     it is `public_object_key` on this row, because it is a DERIVATIVE of this object (same
--     `{object_id}` segment, ADR 0052 §3), published and withdrawn with the article (§11).
--   * no `records` purge branch in the guard: no row here can be a `records` file.
--   * `retain_until` may go value → NULL → value: ADR 0052 §6's placeholder for `content-source` is
--     "12 tháng sau khi gỡ đăng", and an article can be unpublished and published again. A retention
--     date fixed at the first unpublish would let the purge remove the cover of an article that is
--     live again. What stays refused is REWRITING a set date into another one.
--
-- `original_name` is typed by staff but can still name a person ("trao-qua-ong-nguyen-van-a.jpg"):
-- same standing as `tieu_de` (0006 PERSONAL DATA) — never logged, never in a key, never in the audit
-- delta. It is the one descriptive column an anonymisation may rewrite, so the guard does not freeze it.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS stored_file (
    tenant_id         TEXT        NOT NULL,
    id                TEXT        NOT NULL,     -- the `{object_id}` segment of object_key (server ULID)

    bucket            TEXT        NOT NULL,     -- the bucket ROLE (core/storage.Bucket), never its name
    object_key        TEXT        NOT NULL,     -- destination key of the ORIGINAL, fixed at issue

    retention_class   TEXT        NOT NULL,
    purpose           TEXT        NOT NULL,
    subject_type      TEXT        NOT NULL,
    subject_id        TEXT        NOT NULL,     -- noi_dung_mini_app.id; no FK — see the cover trigger

    original_name     TEXT        NOT NULL,

    -- Sniffed type, size, hash: measured by the complete step, NULL until then, write-once after.
    mime_type         TEXT,
    size_bytes        BIGINT,
    sha256            TEXT,

    status            TEXT        NOT NULL DEFAULT 'pending',

    -- The APPROVED DERIVATIVE in the PUBLIC bucket (ADR 0052 §2, §11). NULL until the article is
    -- published; set by the publish step after the copy succeeds; cleared by the unpublish step after
    -- the public object is removed. It is the only path a public URL may be built from — the
    -- original's `object_key` never leaves the private bucket.
    public_object_key TEXT,

    -- The staff BUSINESS CODE (`CB-…`), never the internal id (rule 6, invariant 8). No citizen
    -- uploads into this service.
    uploaded_by       TEXT        NOT NULL,

    retain_until      TIMESTAMPTZ,
    legal_hold        BOOLEAN     NOT NULL DEFAULT false,

    purged_at         TIMESTAMPTZ,
    purged_by         TEXT,
    purge_reason      TEXT,

    deleted_at        TIMESTAMPTZ,
    deleted_by        TEXT,
    delete_reason     TEXT,

    created_at        TIMESTAMPTZ NOT NULL,
    updated_at        TIMESTAMPTZ NOT NULL,

    PRIMARY KEY (tenant_id, id),

    -- Composite with tenant_id (rule 1, invariant 6). No `WHERE deleted_at IS NULL`: a key names one
    -- object for ever.
    UNIQUE (tenant_id, object_key),
    UNIQUE (tenant_id, public_object_key),

    -- ADR 0052 §5's machine, identical to petitions 0021 — one lifecycle for every service's files.
    CONSTRAINT stored_file_status_known CHECK (status IN (
        'pending', 'scanning', 'stored', 'processing', 'ready', 'failed', 'rejected', 'purged')),

    -- Widened by a later migration when this service stores something other than a content source.
    CONSTRAINT stored_file_retention_class_known CHECK (retention_class IN ('content-source')),
    CONSTRAINT stored_file_bucket_known CHECK (bucket IN ('private')),

    -- The closed purpose list lives in core/storage (Purpose) and platform's upload policy; the SHAPE
    -- is checked here, as in 0021, so a new content purpose is not a migration.
    CONSTRAINT stored_file_purpose_shape CHECK (purpose ~ '^[a-z0-9]+(-[a-z0-9]+)*$'),

    CONSTRAINT stored_file_subject_type_known CHECK (subject_type IN ('content-item')),
    CONSTRAINT stored_file_subject_id_present CHECK (btrim(subject_id) <> ''),

    -- THE KEY NAMES THIS COMMUNE, THIS SERVICE, THIS PURPOSE AND THIS ROW (ADR 0052 §3, rule 1
    -- invariant 7).
    CONSTRAINT stored_file_object_key_shape CHECK (
        object_key ~ '^[a-z0-9_./-]+$'
        AND starts_with(object_key, retention_class || '/t_' || lower(tenant_id) || '/')
        AND strpos(object_key, '/comms/' || purpose || '/' || lower(id) || '/') > 0),

    -- THE PUBLIC KEY IS A DERIVATIVE OF THIS SAME OBJECT, IN THIS COMMUNE, AND NEVER THE ORIGINAL:
    -- class `public-media`, same `/comms/{purpose}/{object_id}/` segment, and a variant other than
    -- `original` (ADR 0052 §2: only approved derivatives go public). A key built from another row or
    -- another commune cannot be recorded here.
    CONSTRAINT stored_file_public_object_key_shape CHECK (
        public_object_key IS NULL
        OR (public_object_key ~ '^[a-z0-9_./-]+$'
            AND starts_with(public_object_key, 'public-media/t_' || lower(tenant_id) || '/')
            AND strpos(public_object_key, '/comms/' || purpose || '/' || lower(id) || '/') > 0
            AND public_object_key !~ '/original\.[a-z0-9]+$')),

    -- A derivative exists only after processing finished (`processing → ready`, ADR 0052 §11), and a
    -- soft-deleted row is never public. Purged is excluded by the first half.
    CONSTRAINT stored_file_public_only_when_ready CHECK (
        public_object_key IS NULL OR (status = 'ready' AND deleted_at IS NULL)),

    CONSTRAINT stored_file_original_name_shape CHECK (
        btrim(original_name) <> '' AND char_length(original_name) <= 255
        AND original_name !~ '[[:cntrl:]]'),

    CONSTRAINT stored_file_uploaded_by_present CHECK (btrim(uploaded_by) <> ''),

    CONSTRAINT stored_file_size_non_negative CHECK (size_bytes IS NULL OR size_bytes >= 0),
    CONSTRAINT stored_file_sha256_shape CHECK (sha256 IS NULL OR sha256 ~ '^[0-9a-f]{64}$'),
    CONSTRAINT stored_file_mime_present CHECK (mime_type IS NULL OR btrim(mime_type) <> ''),

    CONSTRAINT stored_file_stored_facts_complete CHECK (
        status NOT IN ('stored', 'processing', 'ready')
        OR (mime_type IS NOT NULL AND size_bytes IS NOT NULL AND sha256 IS NOT NULL)),

    -- `x IS NOT NULL AND btrim(x) <> ''`, never `btrim(x) <> ''` alone: with x NULL the latter is
    -- NULL and a CHECK that evaluates to NULL PASSES (0021:163-165).
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

-- "The files uploaded for this article" — the modal's current and previous covers, and the
-- per-subject count platform's `max_files_per_subject` asks for. Leads with tenant_id (rule 1).
CREATE INDEX IF NOT EXISTS stored_file_by_subject
    ON stored_file (tenant_id, subject_type, subject_id, created_at DESC)
    WHERE deleted_at IS NULL;

-- ---------------------------------------------------------------------------
-- stored_file_guard — what may move on a metadata row, and in which direction. 0021's guard, minus
-- the `records` branch (no such row here), with `retain_until` loosened as stated above.
--
--   DELETE             refused: the row is the only trace that an object existed (rule 7 inv 1).
--   identity columns   frozen: which commune, which object, which class, which subject, who.
--   measured facts     write-once: a hash that changes after the scan is a hash nobody scanned.
--   retain_until       NULL ↔ value only; one set date is never rewritten into another.
--   status             only along ADR 0052 §5's edges — the same set as 0021.
--   purge              refused under legal_hold.
--   purge / delete     write-once.
--   public_object_key  free to set and clear; its CHECKs bind it to this object, `ready`, not deleted.
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

    IF (OLD.mime_type  IS NOT NULL AND NEW.mime_type  IS DISTINCT FROM OLD.mime_type)
    OR (OLD.size_bytes IS NOT NULL AND NEW.size_bytes IS DISTINCT FROM OLD.size_bytes)
    OR (OLD.sha256     IS NOT NULL AND NEW.sha256     IS DISTINCT FROM OLD.sha256) THEN
        RAISE EXCEPTION 'administrative record %: measured facts are write-once', TG_TABLE_NAME
            USING HINT = 'Type, size and hash are measured once at completion (ADR 0052 §5).';
    END IF;

    IF OLD.retain_until IS NOT NULL AND NEW.retain_until IS NOT NULL
       AND NEW.retain_until <> OLD.retain_until THEN
        RAISE EXCEPTION 'administrative record %: a set retention date is never rewritten', TG_TABLE_NAME
            USING HINT = 'retain_until is fixed at the act that fixes it (unpublish). A republish '
                         'clears it; the next unpublish fixes a new one (ADR 0052 §5, §6).';
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

    IF NEW.status = 'purged' AND OLD.status <> 'purged' AND NEW.legal_hold THEN
        RAISE EXCEPTION 'administrative record %: purge refused under legal hold', TG_TABLE_NAME
            USING HINT = 'A file held for a complaint or an inspection is never purged (ADR 0052 §5).';
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
-- noi_dung_mini_app — THE SIX NEW COLUMNS. All NULLABLE, no default, English names (rule 12).
--
--   cover_image_file_id  → stored_file (tenant_id, id). The uploaded cover. `anh_dai_dien_url` (0006)
--                          is NOT dropped and NOT migrated: it is a URL to a file some other system
--                          serves, not an object this service holds, so there is nothing to copy.
--   published_at         G1: the instant of the FIRST publish, never changed afterwards.
--
--                        ⚠ NO BACKFILL, ON PURPOSE. A row already `dang-hien` keeps NULL: the moment
--                        it was first published was never recorded, and `ngay_dang` is a DATE the
--                        staff may backdate (0006:287-290) — copying it into an instant would invent
--                        a time of day and present an editable date as a fixed fact. The service
--                        FALLS BACK TO `ngay_dang` when `published_at` is NULL.
--   event_starts_at      `su-kien` only (§7 closing note: "thời gian & địa điểm").
--   event_ends_at        `su-kien` only; ≥ event_starts_at; needs event_starts_at.
--   event_place          `su-kien` only; free text, bounded. Can name a person or a household
--                        ("sân nhà ông …"): same standing as `tieu_de` — kept out of logs and the
--                        audit delta.
--   video_url            `video` only; an EXTERNAL link opened outside the Mini App (ADR 0047 §6 (3)).
--                        http(s) only, so `javascript:` and `data:` can never reach a citizen's tap.
-- ---------------------------------------------------------------------------
ALTER TABLE noi_dung_mini_app ADD COLUMN IF NOT EXISTS cover_image_file_id TEXT;
ALTER TABLE noi_dung_mini_app ADD COLUMN IF NOT EXISTS published_at        TIMESTAMPTZ;
ALTER TABLE noi_dung_mini_app ADD COLUMN IF NOT EXISTS event_starts_at     TIMESTAMPTZ;
ALTER TABLE noi_dung_mini_app ADD COLUMN IF NOT EXISTS event_ends_at       TIMESTAMPTZ;
ALTER TABLE noi_dung_mini_app ADD COLUMN IF NOT EXISTS event_place         TEXT;
ALTER TABLE noi_dung_mini_app ADD COLUMN IF NOT EXISTS video_url           TEXT;

-- ---------------------------------------------------------------------------
-- THE CONSTRAINTS. Added VALIDATED, not `NOT VALID` + `VALIDATE`, and the reason is specific:
--
--   * `NOT VALID` on a partitioned table is not a form every supported PostgreSQL accepts
--     (service-finance/migrations/0008_dot_thu_chi.sql:155; service-identity 0016:44), and a
--     foreign key NOT VALID on a partitioned table is refused outright before PostgreSQL 18.
--   * Its only benefit is a shorter lock, and there is none to gain here: ADD COLUMN above already
--     holds ACCESS EXCLUSIVE until COMMIT (one transaction per file), so a VALIDATE in the same file
--     scans under the same lock.
--   * Every value the validation reads is NULL — the columns were created three statements up — so
--     every CHECK passes on every existing row by construction.
--
-- Lookup by name pinned to the PARENT (`conrelid`): partitions inherit the name, so `conname` alone
-- matches 33 rows (0008:149-150). ADD CONSTRAINT has no IF NOT EXISTS; a retry must cost nothing.
--
-- `loai` REMAINS EDITABLE (0006:454). The per-type CHECKs therefore mean: a write that changes an
-- article away from `su-kien` / `video` must clear that type's columns IN THE SAME UPDATE, or it is
-- refused. That is intended — an event window left on a news article is a date a citizen reads.
-- ---------------------------------------------------------------------------
DO $$
DECLARE parent oid := 'noi_dung_mini_app'::regclass;
BEGIN
    -- SAME SERVICE, SAME SCHEMA, composite with tenant_id: a cover cannot be another commune's file.
    -- Both sides are hash-partitioned on tenant_id; the referenced pair is stored_file's primary key.
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = parent
                   AND conname = 'noi_dung_mini_app_cover_image_file_fk') THEN
        ALTER TABLE noi_dung_mini_app ADD CONSTRAINT noi_dung_mini_app_cover_image_file_fk
            FOREIGN KEY (tenant_id, cover_image_file_id) REFERENCES stored_file (tenant_id, id);
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = parent
                   AND conname = 'noi_dung_mini_app_event_only_for_su_kien') THEN
        ALTER TABLE noi_dung_mini_app ADD CONSTRAINT noi_dung_mini_app_event_only_for_su_kien
            CHECK (loai = 'su-kien'
                   OR (event_starts_at IS NULL AND event_ends_at IS NULL AND event_place IS NULL));
    END IF;

    -- An end with no start is not a window. Both arms are TRUE/FALSE, never NULL: the NULL cases are
    -- spelled out, because a CHECK that evaluates to NULL passes.
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = parent
                   AND conname = 'noi_dung_mini_app_event_window_valid') THEN
        ALTER TABLE noi_dung_mini_app ADD CONSTRAINT noi_dung_mini_app_event_window_valid
            CHECK (event_ends_at IS NULL
                   OR (event_starts_at IS NOT NULL AND event_ends_at >= event_starts_at));
    END IF;

    -- Bounded IN CHARACTERS (char_length, so diacritics do not shorten it). '' is not "no place" —
    -- NULL is. 500 is a vendor bound against abuse, not a customer number; raising it is one
    -- migration, lowering it after staff wrote longer text is not.
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = parent
                   AND conname = 'noi_dung_mini_app_event_place_valid') THEN
        ALTER TABLE noi_dung_mini_app ADD CONSTRAINT noi_dung_mini_app_event_place_valid
            CHECK (event_place IS NULL
                   OR (btrim(event_place) <> '' AND char_length(event_place) <= 500
                       AND event_place !~ '[[:cntrl:]]'));
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = parent
                   AND conname = 'noi_dung_mini_app_video_url_only_for_video') THEN
        ALTER TABLE noi_dung_mini_app ADD CONSTRAINT noi_dung_mini_app_video_url_only_for_video
            CHECK (loai = 'video' OR video_url IS NULL);
    END IF;

    -- http(s) ONLY, no whitespace or control character anywhere, 2048 characters at most. The
    -- scheme is matched case-insensitively (`~*`) because RFC 3986 makes it so; the write path
    -- should still store it lower-cased.
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = parent
                   AND conname = 'noi_dung_mini_app_video_url_valid') THEN
        ALTER TABLE noi_dung_mini_app ADD CONSTRAINT noi_dung_mini_app_video_url_valid
            CHECK (video_url IS NULL
                   OR (video_url ~* '^https?://[^[:space:][:cntrl:]]+$'
                       AND char_length(video_url) <= 2048));
    END IF;
END $$;

-- ---------------------------------------------------------------------------
-- noi_dung_mini_app_cover_image_check — the floor under choosing a cover, for every writer.
--
-- The file must be IN THIS COMMUNE (the FK), uploaded FOR THIS ARTICLE (`subject_id = NEW.id`), for
-- the cover purpose, not soft-deleted, and past the scan (`stored`, `processing` or `ready`). A
-- pending or rejected object is not something staff can have looked at before choosing it.
--
-- NO FOREIGN KEY FROM stored_file.subject_id TO THE ARTICLE, deliberately: the modal of §7 uploads
-- the image BEFORE `Lưu`, so the upload may be issued against an article id the service generated
-- but has not inserted yet. The binding is checked HERE, when the article points at the file.
--
-- Fires only when the column is set or changed; clearing it (NULL) is always allowed. The file row
-- is locked FOR SHARE so a concurrent soft delete waits for this transaction.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION noi_dung_mini_app_cover_image_check() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE file_row RECORD;
BEGIN
    IF NEW.cover_image_file_id IS NULL
       OR (TG_OP = 'UPDATE' AND NEW.cover_image_file_id IS NOT DISTINCT FROM OLD.cover_image_file_id) THEN
        RETURN NEW;
    END IF;

    SELECT f.subject_type, f.subject_id, f.purpose, f.status, f.deleted_at
      INTO file_row
      FROM stored_file f
     WHERE f.tenant_id = NEW.tenant_id AND f.id = NEW.cover_image_file_id
       FOR SHARE;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'noi_dung_mini_app: cover file not found in this commune';
    END IF;
    IF file_row.deleted_at IS NOT NULL
       OR file_row.subject_type <> 'content-item'
       OR file_row.subject_id <> NEW.id
       OR file_row.purpose <> 'content-image' THEN
        RAISE EXCEPTION 'noi_dung_mini_app: cover file was not uploaded as this item''s image';
    END IF;
    IF file_row.status NOT IN ('stored', 'processing', 'ready') THEN
        RAISE EXCEPTION 'noi_dung_mini_app: cover file status % cannot be used', file_row.status;
    END IF;

    RETURN NEW;
END $$;

DROP TRIGGER IF EXISTS noi_dung_mini_app_cover_image_check ON noi_dung_mini_app;
CREATE TRIGGER noi_dung_mini_app_cover_image_check
    BEFORE INSERT OR UPDATE OF cover_image_file_id ON noi_dung_mini_app
    FOR EACH ROW EXECUTE FUNCTION noi_dung_mini_app_cover_image_check();

-- ---------------------------------------------------------------------------
-- noi_dung_mini_app_bat_bien — REPLACED to add G1. The five 0006 rules (0006:461-499) are carried
-- over VERBATIM, in the same order; the trigger from 0006 keeps pointing at this name, so it is not
-- recreated. migrations/content_item_media_test.go asserts every 0006 clause is still present.
--
-- G1, TWO HALVES:
--   * once `published_at` is set it never changes, and is never cleared. Unpublishing sets
--     `trang_thai = 'an'` and leaves `published_at` as it is — it still says when the article first
--     went live.
--   * it is first set ONLY by the act of publishing: the same UPDATE that moves `trang_thai` INTO
--     `dang-hien`. That keeps an unrelated edit of a legacy published row (NULL, see above) from
--     stamping "first published now" on an article residents have been reading for months.
--
-- NOT ENFORCED HERE, and owed by the write path: at INSERT, set `published_at = now()` exactly when
-- the row is born `dang-hien` (§7's `☐ Đăng lên Mini App` ticked). A CHECK cannot say it — an
-- unpublished row legitimately carries a `published_at` — and this trigger is UPDATE-only.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION noi_dung_mini_app_bat_bien() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.nguoi_tao_ma IS DISTINCT FROM OLD.nguoi_tao_ma
    OR NEW.tao_luc      IS DISTINCT FROM OLD.tao_luc THEN
        RAISE EXCEPTION 'noi_dung_mini_app: author and creation time are immutable'
            USING HINT = 'Who filed this item and when are fixed at creation. Record a new item '
                         'instead (rule 7, forbidden #5).';
    END IF;

    IF NEW.nguon IS DISTINCT FROM OLD.nguon THEN
        RAISE EXCEPTION 'noi_dung_mini_app: provenance is immutable'
            USING HINT = 'An item does not change where it came from. Changing it would move the '
                         'row in or out of the sync deduplication of '
                         'docs/ui-ux/11-noi-dung-mini-app.md §10.1.';
    END IF;

    IF OLD.nguon_id_ngoai IS NOT NULL
    AND NEW.nguon_id_ngoai IS DISTINCT FROM OLD.nguon_id_ngoai THEN
        RAISE EXCEPTION 'noi_dung_mini_app: the portal identifier is immutable'
            USING HINT = 'It is the deduplication key of §10.1. Freeing it imports the same portal '
                         'article again as a second row.';
    END IF;

    IF OLD.da_sua_tay AND NOT NEW.da_sua_tay THEN
        RAISE EXCEPTION 'noi_dung_mini_app: a hand-edited item cannot be marked un-edited'
            USING HINT = 'docs/ui-ux/11-noi-dung-mini-app.md §10.4: a member of staff was promised '
                         'that their edit survives the next sync. Clearing this flag withdraws that '
                         'promise and the next run overwrites their work.';
    END IF;

    IF NEW.luot_xem < OLD.luot_xem THEN
        RAISE EXCEPTION 'noi_dung_mini_app: the view counter cannot go down'
            USING HINT = 'A figure that decreases with no event behind it is what rule 10, '
                         'invariant 3 refuses. Correcting it is a new column, not a lower value.';
    END IF;

    IF OLD.published_at IS NOT NULL
    AND NEW.published_at IS DISTINCT FROM OLD.published_at THEN
        RAISE EXCEPTION 'noi_dung_mini_app: the first publish time is immutable'
            USING HINT = 'ADR 0047 §6 G1: written at the first publish, never changed afterwards. '
                         'Unpublishing changes trang_thai only.';
    END IF;

    IF OLD.published_at IS NULL AND NEW.published_at IS NOT NULL
    AND NOT (NEW.trang_thai = 'dang-hien' AND OLD.trang_thai IS DISTINCT FROM 'dang-hien') THEN
        RAISE EXCEPTION 'noi_dung_mini_app: the first publish time is set only by publishing'
            USING HINT = 'Set published_at in the same UPDATE that moves trang_thai into dang-hien '
                         '(ADR 0047 §6 G1).';
    END IF;

    RETURN NEW;
END $$;

-- ---------------------------------------------------------------------------
-- WHAT THIS FILE DOES NOT STOP, said plainly: TRUNCATE on `stored_file`, DDL by the table owner — the
-- line 0006 and ADR 0013 draw. Nor does it stop the PURGE of a file that is still the cover of a
-- live article: that is a read across two tables at purge time, and the purge worker does not exist
-- yet (ADR 0052 §6). The worker owes that check.
--
-- REVERSAL (migration question 3). Run by a person, in ONE transaction; core/migrate has no automatic
-- rollback (ADR 0013). There is no separate down file ON PURPOSE: core/migrate applies EVERY *.sql
-- in this directory (core/migrate/migrate.go:295-312), so a down file placed here would run as the
-- next migration.
--
-- LOSSLESS ONLY WHILE ALL OF THESE READ ZERO:
--
--   SELECT count(*) FROM stored_file;
--   SELECT count(*) FROM noi_dung_mini_app
--    WHERE cover_image_file_id IS NOT NULL OR published_at IS NOT NULL
--       OR event_starts_at IS NOT NULL OR event_ends_at IS NOT NULL
--       OR event_place IS NOT NULL OR video_url IS NOT NULL;
--
-- ZERO → in this order:
--
--   DROP TRIGGER noi_dung_mini_app_cover_image_check ON noi_dung_mini_app;
--   DROP FUNCTION noi_dung_mini_app_cover_image_check();
--   -- restore 0006's function: re-run 0006_noi_dung_mini_app.sql:461-499 verbatim (CREATE OR REPLACE)
--   ALTER TABLE noi_dung_mini_app
--       DROP CONSTRAINT noi_dung_mini_app_video_url_valid,
--       DROP CONSTRAINT noi_dung_mini_app_video_url_only_for_video,
--       DROP CONSTRAINT noi_dung_mini_app_event_place_valid,
--       DROP CONSTRAINT noi_dung_mini_app_event_window_valid,
--       DROP CONSTRAINT noi_dung_mini_app_event_only_for_su_kien,
--       DROP CONSTRAINT noi_dung_mini_app_cover_image_file_fk,
--       DROP COLUMN video_url, DROP COLUMN event_place, DROP COLUMN event_ends_at,
--       DROP COLUMN event_starts_at, DROP COLUMN published_at, DROP COLUMN cover_image_file_id;
--   DROP TABLE stored_file;            -- AFTER the FK is gone; partitions, index, trigger go with it
--   DROP FUNCTION stored_file_guard();
--   DELETE FROM schema_migration WHERE <this file's row>;  -- the runner's own bookkeeping, not business data
--
-- NON-ZERO → a commune has uploaded a cover, published under G1, or entered an event or a video link.
-- Dropping any of it destroys part of what a public authority told its residents, and `stored_file`
-- is the only index to objects still in MinIO. That is rule 7's stop condition #2 — a user decision
-- plus a verified backup, never a command — and the way back is a NEW migration.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- BACKSTOP: every partitioned table in this schema must actually have partitions (0002, §BACKSTOP;
-- 0006:531-558) — repeated because it only verifies the state after a file that carries it.
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
