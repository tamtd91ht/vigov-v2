-- platform — the commune's own identity images (ADR 0069): this service's own `stored_file` (ADR 0052
-- §5), and two references from ho_so_hien_thi_xa to the CURRENT logo and the CURRENT web-admin banner.
--
-- WHY THIS FILE EXISTS. Owner decision 02/10/2026 (kb/10-decisions/0069-nhan-dien-xa-logo-banner.md):
-- the commune uploads its logo and its web-admin banner itself, in web-admin, under admin.org; saving
-- is publishing (no review); every set / clear is audited in the same transaction with the CB- code;
-- the image is scanned and normalised before it goes public. ADR 0069 §Hệ quả: "bảng stored_file
-- riêng của platform (luật 2: không dùng chung bảng của comms)" and "logo_url đổi nghĩa … migration
-- riêng" — this is that migration.
--
-- WHY A NEW FILE AND NOT AN EDIT OF 0006: core/migrate compares the checksum of every applied file at
-- startup. An applied migration is immutable.
--
-- ENTITY OWNERSHIP is declared with the `-- @entity` / `-- @scope` marks above CREATE TABLE (ADR 0021).
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. ROWS PER COMMUNE: `stored_file` is new and empty. ho_so_hien_thi_xa (at most one row per
--      commune, 0006) gains two NULLABLE columns with no default — PostgreSQL records them in the
--      catalogue and REWRITES NO ROW. NO ROW IS WRITTEN: no backfill. `logo_url` is NOT copied into
--      the new reference: it is a URL a person typed, pointing at an image some other system serves,
--      not an object this service holds — there is nothing to copy, and fetching it would be the
--      server downloading whatever host was typed. Today no write path for logo_url exists (0006,
--      kb/90-ephemeral/tien-do/service-platform.json "VẪN CHƯA"), so it is NULL on every row.
--   2. IF IT STOPS HALF-WAY: it cannot land half-applied — one file, one transaction, progress row
--      inside it. Every statement is IF NOT EXISTS / CREATE OR REPLACE / DROP TRIGGER IF EXISTS or a
--      constraint added behind a pg_constraint lookup, so a retry costs nothing.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. READ PATHS THAT CHANGE MEANING WHILE HALF-APPLIED: none can see it half-applied (one
--      transaction). BETWEEN THIS FILE AND THE GO CHANGE (TASK-3): internal/store/ho_so_hien_thi.go
--      names its columns explicitly (cotHoSoHienThi), so no read path sees the new columns until it
--      asks for them; both are NULL on every row, so the new FKs and the trigger are inert until a
--      writer sets one. `logo_url` keeps its meaning and its one reader (GetTenantProfile.logo_url,
--      now deprecated in platform.proto) — nothing here reads or writes it.
--   5. RETENTION: no column is dropped, retyped or emptied; `logo_url` is KEPT (rule 7) — only its
--      COMMENT changes. A replaced logo or banner stays a `stored_file` row; only its object may be
--      purged, by the worker ADR 0052 §6 describes, under its retention class. No business code is
--      issued or renumbered.
--
-- THE LOCK: ADD COLUMN and ADD CONSTRAINT take ACCESS EXCLUSIVE on ho_so_hien_thi_xa until COMMIT;
-- the FK also takes SHARE ROW EXCLUSIVE on the new, empty stored_file. ho_so_hien_thi_xa holds at most
-- one row per commune — milliseconds. Its readers (GetTenantProfile, the public commune-profile route
-- of identity) wait that long.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- @entity: PlatformStoredFile
-- @scope:  tenant
--
-- stored_file — one object THIS service put into object storage (ADR 0052 §5). THE ENTITY NAME
-- CARRIES THE SERVICE because every file-owning service keeps its own `stored_file` (ADR 0052 §1,
-- rule 2 invariant 1), and tools/kb would otherwise report one entity with two owners — the device of
-- service-comms/migrations/0011_content_item_media_and_event.sql:52 (`CommsStoredFile`) and
-- service-petitions/migrations/0021_task_log_attachment.sql:46 (`PetitionsStoredFile`).
--
-- THE SHAPE IS comms 0011's, column for column, so a reader who has met it once does not reason twice
-- (and TASK-3 can mirror service-comms/internal/store/stored_file.go). WHAT DIFFERS, and why:
--
--   * `purpose` is a CLOSED list here ('tenant-logo', 'tenant-banner'), not a shape: platform owns
--     exactly these two file kinds, and a third is a decision (ADR 0052 §10), not a free string.
--   * `subject_type` admits only 'tenant-display-profile' and `subject_id` MUST EQUAL `tenant_id`: the
--     record a logo or banner belongs to is the commune's display profile, whose key IS tenant_id
--     (ho_so_hien_thi_xa, 0006). Kept rather than dropped so the row reads like every other service's
--     (ADR 0052 §5) and a purge worker asks "is this still the current image" the same way.
--   * `retention_class` only 'content-source' and `bucket` only 'private': the original is always
--     private (ADR 0052 §2), and `content-source` is the ONLY class core/storage.PublishDerivative
--     accepts (records is never public, citizen-media is stop condition #2). ADR 0052 §6's placeholder
--     for it — "12 tháng sau khi gỡ đăng" — reads here as 12 months after the image stopped being the
--     current one; NOT decided (ADR 0052 Còn mở #1), so `retain_until` is left to the write path.
--   * `tenant_id` REFERENCES tenant (id): unlike comms, platform holds the commune registry in this
--     same schema, so the database can refuse a file for a commune that does not exist.
--   * NOT PARTITIONED, unlike comms/petitions: a commune replaces its logo or banner a handful of times
--     in its life (plus the pending/failed rows of abandoned uploads) — the reasoning 0006 gives for
--     ho_so_hien_thi_xa. A FK from a plain table to this one is the same either way (PG ≥ 12).
--
-- `public_object_key` is the ONLY path a public URL may be built from: the API returns
-- OBJECT_STORAGE_PUBLIC_MEDIA_BASE_URL + '/' + public_object_key. NO URL COLUMN, deliberately: the
-- base URL is per-environment configuration (ADR 0052 §4) and a CDN may be put in front later; a
-- stored URL would freeze today's host into every row.
--
-- `original_name` is typed by staff and can still name a person ("logo-chu-tich-nguyen-van-a.png"):
-- never logged, never in a key, never in the audit delta (rule 3). The guard does not freeze it, so
-- an anonymisation may rewrite it.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS stored_file (
    tenant_id         TEXT        NOT NULL REFERENCES tenant (id),
    id                TEXT        NOT NULL,     -- the `{object_id}` segment of object_key (server ULID)

    bucket            TEXT        NOT NULL,     -- the bucket ROLE (core/storage.Bucket), never its name
    object_key        TEXT        NOT NULL,     -- destination key of the ORIGINAL, fixed at issue

    retention_class   TEXT        NOT NULL,
    purpose           TEXT        NOT NULL,
    subject_type      TEXT        NOT NULL,
    subject_id        TEXT        NOT NULL,     -- = tenant_id: the commune's display profile

    original_name     TEXT        NOT NULL,

    -- Sniffed type, size, hash: measured by the complete step, NULL until then, write-once after.
    mime_type         TEXT,
    size_bytes        BIGINT,
    sha256            TEXT,

    status            TEXT        NOT NULL DEFAULT 'pending',

    -- The APPROVED, NORMALISED DERIVATIVE in the PUBLIC bucket (ADR 0052 §2, §11; ADR 0069 #4, #5):
    -- the 512px PNG of a logo, the 1600px-wide image of a banner. NULL until published; set by the
    -- publish step after the copy succeeds; cleared by the unpublish step after the public object is
    -- removed. The original's `object_key` never leaves the private bucket.
    public_object_key TEXT,

    -- The staff BUSINESS CODE (`CB-…`), never the internal id (rule 6, invariant 8). Only commune
    -- staff upload here (ADR 0069 #1: no platform-admin path), and no citizen ever does.
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

    -- ADR 0052 §5's machine, identical to comms 0011 and petitions 0021 — one lifecycle for every
    -- service's files.
    CONSTRAINT stored_file_status_known CHECK (status IN (
        'pending', 'scanning', 'stored', 'processing', 'ready', 'failed', 'rejected', 'purged')),

    CONSTRAINT stored_file_retention_class_known CHECK (retention_class IN ('content-source')),
    CONSTRAINT stored_file_bucket_known CHECK (bucket IN ('private')),

    -- Spelled exactly as core/storage.Purpose; widened by a later migration only.
    CONSTRAINT stored_file_purpose_known CHECK (purpose IN ('tenant-logo', 'tenant-banner')),

    CONSTRAINT stored_file_subject_type_known CHECK (subject_type IN ('tenant-display-profile')),
    CONSTRAINT stored_file_subject_is_own_commune CHECK (subject_id = tenant_id),

    -- THE KEY NAMES THIS COMMUNE, THIS SERVICE, THIS PURPOSE AND THIS ROW (ADR 0052 §3, rule 1
    -- invariant 7).
    CONSTRAINT stored_file_object_key_shape CHECK (
        object_key ~ '^[a-z0-9_./-]+$'
        AND starts_with(object_key, retention_class || '/t_' || lower(tenant_id) || '/')
        AND strpos(object_key, '/platform/' || purpose || '/' || lower(id) || '/') > 0),

    -- THE PUBLIC KEY IS A DERIVATIVE OF THIS SAME OBJECT, IN THIS COMMUNE, AND NEVER THE ORIGINAL:
    -- class `public-media`, same `/platform/{purpose}/{object_id}/` segment, and a variant other than
    -- `original` (ADR 0052 §2). A key built from another row or another commune cannot be recorded.
    CONSTRAINT stored_file_public_object_key_shape CHECK (
        public_object_key IS NULL
        OR (public_object_key ~ '^[a-z0-9_./-]+$'
            AND starts_with(public_object_key, 'public-media/t_' || lower(tenant_id) || '/')
            AND strpos(public_object_key, '/platform/' || purpose || '/' || lower(id) || '/') > 0
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
    -- NULL and a CHECK that evaluates to NULL PASSES (petitions 0021:163-165).
    CONSTRAINT stored_file_purge_complete CHECK (
        (status = 'purged' AND purged_at IS NOT NULL AND purged_by IS NOT NULL
             AND purge_reason IS NOT NULL AND btrim(purge_reason) <> '')
        OR (status <> 'purged' AND purged_at IS NULL AND purged_by IS NULL AND purge_reason IS NULL)),

    CONSTRAINT stored_file_delete_complete CHECK (
        (deleted_at IS NULL AND deleted_by IS NULL AND delete_reason IS NULL)
        OR (deleted_at IS NOT NULL AND deleted_by IS NOT NULL
             AND delete_reason IS NOT NULL AND btrim(delete_reason) <> ''))
);

COMMENT ON TABLE stored_file IS
    'Tep do service-platform dua vao kho doi tuong (ADR 0052 muc 5): logo xa va banner web-admin '
    '(ADR 0069). Ban goc o private; public_object_key la ban dan xuat da chuan hoa o public. '
    'Chi xoa mem; doi tuong chi bi purge theo ADR 0052 muc 6.';
COMMENT ON COLUMN stored_file.public_object_key IS
    'Khoa ban dan xuat o bucket public. URL cong khai = OBJECT_STORAGE_PUBLIC_MEDIA_BASE_URL + khoa nay; '
    'khong luu URL (ADR 0052 muc 4).';

-- "The files uploaded for this commune's profile, newest first" — the current and previous images, and
-- the per-subject count platform's `max_files_per_subject` would ask for. Leads with tenant_id (rule 1).
CREATE INDEX IF NOT EXISTS stored_file_by_subject
    ON stored_file (tenant_id, subject_type, subject_id, purpose, created_at DESC)
    WHERE deleted_at IS NULL;

-- ---------------------------------------------------------------------------
-- stored_file_guard — what may move on a metadata row, and in which direction. comms 0011's guard
-- verbatim in meaning: DELETE refused; identity columns frozen; measured facts write-once; a set
-- retain_until is never rewritten into another date (NULL ↔ value allowed: an image can stop being
-- current and, re-chosen, become current again); status only along ADR 0052 §5's edges; no purge under
-- legal_hold; purge / delete facts write-once; public_object_key free to set and clear (its CHECKs
-- bind it to this object, `ready`, not deleted).
--
-- A NEW FUNCTION IN THIS SCHEMA, NOT A BORROWED ONE: comms' stored_file_guard lives in another
-- service's database (rule 2, invariant 2). The messages name the operation and the relation and
-- nothing else (rule 3, forbidden #3).
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
            USING HINT = 'retain_until is fixed at the act that fixes it (the image stops being '
                         'current). Re-choosing it clears the date; the next replacement fixes a new one.';
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
-- ho_so_hien_thi_xa — TWO NEW COLUMNS, both NULLABLE, no default, English names (rule 12; the table
-- and its existing columns keep their names, rule 12 invariant 3).
--
--   logo_file_id              → stored_file (tenant_id, id), purpose 'tenant-logo'. The commune's
--                               CURRENT logo (ADR 0069 #4): web-admin sidebar + login, and the Mini App.
--   web_admin_banner_file_id  → stored_file (tenant_id, id), purpose 'tenant-banner'. The CURRENT
--                               web-admin banner (ADR 0069 #5). NOT the Mini App banner (#6).
--
-- NULL = not set: the consumer shows the building icon / draws no strip (ADR 0069 #7). Clearing a
-- reference is how "gỡ" is written; the file row stays.
--
-- A ROW FOR A COMMUNE THAT HAS NONE YET needs no change here: every 0006 text column defaults to '',
-- so the write path's upsert inserts (tenant_id, logo_file_id | web_admin_banner_file_id, tao_boi,
-- cap_nhat_boi) and nothing else.
-- ---------------------------------------------------------------------------
ALTER TABLE ho_so_hien_thi_xa ADD COLUMN IF NOT EXISTS logo_file_id             TEXT;
ALTER TABLE ho_so_hien_thi_xa ADD COLUMN IF NOT EXISTS web_admin_banner_file_id TEXT;

-- SAME SERVICE, SAME SCHEMA, composite with tenant_id: a commune's profile can never point at another
-- commune's file (rule 1). MATCH SIMPLE: a NULL reference is not checked, which is "not set".
-- ADD CONSTRAINT has no IF NOT EXISTS; looked up by name on this table so a retry costs nothing.
-- Added VALIDATED: every value the validation reads is NULL, created two statements up.
DO $$
DECLARE parent oid := 'ho_so_hien_thi_xa'::regclass;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = parent
                   AND conname = 'ho_so_hien_thi_xa_logo_file_fk') THEN
        ALTER TABLE ho_so_hien_thi_xa ADD CONSTRAINT ho_so_hien_thi_xa_logo_file_fk
            FOREIGN KEY (tenant_id, logo_file_id) REFERENCES stored_file (tenant_id, id);
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = parent
                   AND conname = 'ho_so_hien_thi_xa_web_admin_banner_file_fk') THEN
        ALTER TABLE ho_so_hien_thi_xa ADD CONSTRAINT ho_so_hien_thi_xa_web_admin_banner_file_fk
            FOREIGN KEY (tenant_id, web_admin_banner_file_id) REFERENCES stored_file (tenant_id, id);
    END IF;
END $$;

COMMENT ON COLUMN ho_so_hien_thi_xa.logo_url IS
    'DA LOI THOI (ADR 0069): URL go tay toi anh do he thong khac phuc vu. Thay bang logo_file_id. '
    'Giu nguyen nghia va du lieu (luat 7), khong ghi moi, khong sao sang logo_file_id; '
    'khong dung lam du phong cho logo.';
COMMENT ON COLUMN ho_so_hien_thi_xa.logo_file_id IS
    'Logo HIEN TAI cua xa (ADR 0069 muc 4): stored_file cung xa, purpose tenant-logo, da dang '
    '(status ready, co public_object_key). NULL = chua co logo -> hien bieu tuong toa nha.';
COMMENT ON COLUMN ho_so_hien_thi_xa.web_admin_banner_file_id IS
    'Banner web-admin HIEN TAI (ADR 0069 muc 5): stored_file cung xa, purpose tenant-banner, da dang. '
    'NULL = khong co dai banner. KHONG phai banner Mini App (noi dung banner cua comms).';
-- ADR 0069 #1 makes commune staff the ONLY writer of this table (no platform-admin path), so 0006's
-- wording "staff business code" holds; said on the columns, where a reader of the schema meets it.
COMMENT ON COLUMN ho_so_hien_thi_xa.tao_boi IS
    'Ma can bo CB- cua nguoi tao dong (luat 6 bat bien 8). Chi can bo cua xa ghi bang nay (ADR 0069 muc 1).';
COMMENT ON COLUMN ho_so_hien_thi_xa.cap_nhat_boi IS
    'Ma can bo CB- cua nguoi sua gan nhat (luat 6 bat bien 8). Chi can bo cua xa ghi bang nay (ADR 0069 muc 1).';

-- ---------------------------------------------------------------------------
-- ho_so_hien_thi_xa_branding_file_check — the floor under choosing a logo or a banner, for every
-- writer. The file must be IN THIS COMMUNE (the FK), uploaded FOR THIS PROFILE (subject), for the
-- matching purpose, not soft-deleted, and ALREADY PUBLISHED: status 'ready' with a public_object_key.
-- ADR 0069 #3 says saving is showing, and #4/#5 say the image is scanned and normalised before it is
-- public — so the profile may only point at something that is already safe to show. A pending,
-- rejected or unpublished file is refused here, not discovered as a broken image by a resident.
--
-- Fires only when a reference is set or changed; clearing it (NULL) is always allowed. The file row is
-- locked FOR SHARE so a concurrent soft delete or unpublish waits for this transaction.
--
-- WHAT IT DOES NOT STOP, owed by the write path (TASK-3): UNPUBLISHING or soft-deleting a file that is
-- still referenced. Replace in this order, in one transaction with the audit entry: point the profile
-- at the new file, THEN unpublish the old one. Readers must still treat a referenced file with
-- public_object_key NULL or deleted_at set as "not set".
-- ---------------------------------------------------------------------------
-- One check, two slots: a helper rather than the body written twice, so the logo and the banner can
-- never drift into two different rules.
CREATE OR REPLACE FUNCTION platform_branding_file_assert(p_tenant_id text, p_file_id text, p_purpose text)
RETURNS void
LANGUAGE plpgsql AS $$
DECLARE file_row RECORD;
BEGIN
    SELECT f.subject_type, f.subject_id, f.purpose, f.status, f.public_object_key, f.deleted_at
      INTO file_row
      FROM stored_file f
     WHERE f.tenant_id = p_tenant_id AND f.id = p_file_id
       FOR SHARE;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'ho_so_hien_thi_xa: % file not found in this commune', p_purpose;
    END IF;
    IF file_row.deleted_at IS NOT NULL
       OR file_row.subject_type <> 'tenant-display-profile'
       OR file_row.subject_id <> p_tenant_id
       OR file_row.purpose <> p_purpose THEN
        RAISE EXCEPTION 'ho_so_hien_thi_xa: file was not uploaded as this commune''s %', p_purpose;
    END IF;
    IF file_row.status <> 'ready' OR file_row.public_object_key IS NULL THEN
        RAISE EXCEPTION 'ho_so_hien_thi_xa: % file is not published (status %)', p_purpose, file_row.status
            USING HINT = 'ADR 0069: scan, normalise and publish the derivative first, then point the '
                         'profile at it.';
    END IF;
END $$;

-- On INSERT every non-NULL reference is checked; on UPDATE only a reference that CHANGED (OLD is
-- read only behind TG_OP = 'UPDATE', in its own IF, so no INSERT path ever touches it).
CREATE OR REPLACE FUNCTION ho_so_hien_thi_xa_branding_file_check() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
    logo_changed   boolean := true;
    banner_changed boolean := true;
BEGIN
    IF TG_OP = 'UPDATE' THEN
        logo_changed   := NEW.logo_file_id IS DISTINCT FROM OLD.logo_file_id;
        banner_changed := NEW.web_admin_banner_file_id IS DISTINCT FROM OLD.web_admin_banner_file_id;
    END IF;

    IF NEW.logo_file_id IS NOT NULL AND logo_changed THEN
        PERFORM platform_branding_file_assert(NEW.tenant_id, NEW.logo_file_id, 'tenant-logo');
    END IF;
    IF NEW.web_admin_banner_file_id IS NOT NULL AND banner_changed THEN
        PERFORM platform_branding_file_assert(NEW.tenant_id, NEW.web_admin_banner_file_id, 'tenant-banner');
    END IF;

    RETURN NEW;
END $$;

DROP TRIGGER IF EXISTS ho_so_hien_thi_xa_branding_file_check ON ho_so_hien_thi_xa;
CREATE TRIGGER ho_so_hien_thi_xa_branding_file_check
    BEFORE INSERT OR UPDATE OF logo_file_id, web_admin_banner_file_id ON ho_so_hien_thi_xa
    FOR EACH ROW EXECUTE FUNCTION ho_so_hien_thi_xa_branding_file_check();

-- ---------------------------------------------------------------------------
-- WHAT THIS FILE DOES NOT STOP, said plainly: TRUNCATE on `stored_file`, DDL by the table owner — the
-- line 0006 and ADR 0013 draw. Nor the PURGE of a file that is still the current logo or banner: a
-- read across two tables at purge time, owed by the purge worker (ADR 0052 §6), which does not exist.
--
-- REVERSAL (migration question 3). Run by a person, in ONE transaction; core/migrate has no automatic
-- rollback (ADR 0013). There is no separate down file ON PURPOSE: core/migrate applies EVERY *.sql in
-- this directory, so a down file placed here would run as the next migration.
--
-- LOSSLESS ONLY WHILE BOTH OF THESE READ ZERO:
--
--   SELECT count(*) FROM stored_file;
--   SELECT count(*) FROM ho_so_hien_thi_xa
--    WHERE logo_file_id IS NOT NULL OR web_admin_banner_file_id IS NOT NULL;
--
-- ZERO → in this order:
--
--   DROP TRIGGER ho_so_hien_thi_xa_branding_file_check ON ho_so_hien_thi_xa;
--   DROP FUNCTION ho_so_hien_thi_xa_branding_file_check();
--   DROP FUNCTION platform_branding_file_assert(text, text, text);
--   ALTER TABLE ho_so_hien_thi_xa
--       DROP CONSTRAINT ho_so_hien_thi_xa_web_admin_banner_file_fk,
--       DROP CONSTRAINT ho_so_hien_thi_xa_logo_file_fk,
--       DROP COLUMN web_admin_banner_file_id, DROP COLUMN logo_file_id;
--   COMMENT ON COLUMN ho_so_hien_thi_xa.logo_url / .tao_boi / .cap_nhat_boi IS NULL;  -- 0006 set none
--   DROP TABLE stored_file;            -- AFTER the FKs are gone; index and trigger go with it
--   DROP FUNCTION stored_file_guard();
--   DELETE FROM schema_migration WHERE <this file's row>;  -- the runner's own bookkeeping, not business data
--
-- NON-ZERO → a commune has uploaded a logo or a banner. `stored_file` is then the only index to
-- objects in MinIO and the only record of who set the image a public authority showed; dropping it is
-- rule 7's stop condition #2 — a user decision plus a verified backup, never a command — and the way
-- back is a NEW migration.
--
-- PER-COMMUNE AND RESUMABLE (rule 7, invariant 5): no backfill, no existing row rewritten — nothing to
-- iterate per commune and nothing to resume.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- BACKSTOP, carried forward from 0002: a table declared PARTITION BY with no partitions rejects
-- every INSERT. This file declares no partitioned table and is expected to find nothing; it runs
-- anyway, because the run after which it is missing is the one that needed it.
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
