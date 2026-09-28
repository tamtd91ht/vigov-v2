-- platform — upload_policy (per-purpose upload limits, ADR 0052 §10) and platform_audit_log (the
-- trail of PLATFORM-WIDE configuration changes, which has no commune to be filed under).
--
-- WHY A NEW FILE AND NOT AN EDIT OF 0001–0007: those have been applied and core/migrate compares
-- the checksum of every applied file at startup. An applied migration is immutable.
--
-- WHY BOTH LIVE IN service-platform: ADR 0052 §10 — "Vihat cấu hình ở khu vận hành; platform sở
-- hữu". The contract that serves the first table is PlatformService.ListUploadPolicies
-- (proto/vigov/platform/v1/platform.proto); read its comment before changing a column here.
--
-- THE FIVE MIGRATION QUESTIONS:
--
--   1. HOW MANY ROWS PER COMMUNE: none — neither table has a commune column. upload_policy holds at
--      most one row per purpose (six today); platform_audit_log one row per change to platform-wide
--      configuration, a handful a year.
--   2. IF IT STOPS HALF-WAY: it cannot. One file, one transaction (core/migrate), progress row
--      inside it. Every CREATE is IF NOT EXISTS / CREATE OR REPLACE / DROP TRIGGER IF EXISTS, and
--      every seed INSERT is ON CONFLICT DO NOTHING / guarded by NOT EXISTS, so a retry is free.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. WHICH READ PATHS CHANGE MEANING WHILE IT IS HALF-APPLIED: none. Two NEW tables, one NEW
--      trigger function. Until this file is applied ListUploadPolicies fails (no table), which every
--      caller treats as "platform unavailable" → refuse the upload: the safe direction.
--   5. RETENTION: nothing is removed. upload_policy is soft delete only (hard DELETE refused by a
--      trigger); platform_audit_log is append-only (UPDATE, DELETE and TRUNCATE refused).

-- ---------------------------------------------------------------------------
-- upload_policy — the limit set for ONE purpose: maximum bytes per file, allowed SNIFFED MIME
-- types, and an optional count of live files per business record.
--
-- NO tenant_id, and that is the owner's decision recorded in the contract, not an omission:
-- TODAY every commune gets the platform-wide policy; a per-commune override is OPEN (not decided,
-- not built). ListUploadPolicies is already worded as "the effective policy for the commune in
-- x-tenant-id", so an override, if ever decided, is a NEW table beside this one plus a server-side
-- change — never a tenant_id retro-fitted into this key.
--
-- THE KEY IS THE PURPOSE STRING, spelled exactly as core/storage.Purpose ("content-video"). The
-- proto enum UploadPurpose is DERIVED from that spelling (strip UPLOAD_PURPOSE_, lowercase, _ → -),
-- and service-platform/internal/grpc maps a row to the enum by that rule — no mapping table, which
-- would be a third copy of the list. The CHECK below is the second copy, and it is the one the
-- database can enforce; internal/store/upload_policy_test.go compares it with the Go list.
--
-- max_bytes IS BYTES, NOT MB (10 MB is 10485760). The upper bound 5 GiB is S3's single-PUT/POST
-- limit, which ADR 0052's presigned POST cannot exceed — a larger value would be a limit no upload
-- could ever reach, and the operator setting it would believe something was allowed that is not.
--
-- allowed_mime_types HOLDS IANA MEDIA TYPES, spelled as the core/storage MIME constants. A policy
-- may only NARROW core/storage's allow-list; the platform write path (not built — ADR 0048) must
-- refuse a value outside it, and every caller drops such a value anyway (see the RPC). Not
-- constrained against the list here, deliberately: the list lives in Go and grows with a Go change;
-- a CHECK copy would have to be migrated in lock-step, and the seed below is checked against the Go
-- list by a test instead.
--
-- max_files_per_subject: NULL means Vihat deliberately set NO count limit (an explicit choice, not
-- a default — size and type limits still apply). When set, at least 1.
--
-- Read through store.UploadPolicyStore (upload_policy.go), a raw-handle reader: there is no commune
-- column to scope by, so core/store.Scoped — which always injects WHERE tenant_id = $1 — cannot
-- read it.
-- ---------------------------------------------------------------------------
-- @entity: UploadPolicy
-- @scope:  platform
CREATE TABLE IF NOT EXISTS upload_policy (
    purpose               TEXT        PRIMARY KEY,
    max_bytes             BIGINT      NOT NULL,
    allowed_mime_types    TEXT[]      NOT NULL,
    max_files_per_subject INT,

    -- WHO entered / last changed the row: a business code, never an internal id (rule 6,
    -- invariant 8). 'system' for rows this migration seeds. NOT NULL without a default — a row
    -- nobody signed is the row nobody can defend.
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by            TEXT        NOT NULL,
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by            TEXT        NOT NULL,

    deleted_at            TIMESTAMPTZ,
    deleted_by            TEXT,
    delete_reason         TEXT,

    CONSTRAINT upload_policy_purpose_known CHECK (purpose IN (
        'content-video', 'content-image', 'content-attachment',
        'tenant-logo', 'petition-photo', 'document-scan')),
    CONSTRAINT upload_policy_max_bytes_range CHECK (max_bytes > 0 AND max_bytes <= 5368709120),
    -- cardinality, not array_length: array_length('{}', 1) is NULL, and a CHECK that evaluates to
    -- NULL PASSES — the empty list would be accepted by the very constraint meant to refuse it.
    CONSTRAINT upload_policy_mime_types_not_empty CHECK (cardinality(allowed_mime_types) > 0),
    CONSTRAINT upload_policy_max_files_positive
        CHECK (max_files_per_subject IS NULL OR max_files_per_subject >= 1),
    CONSTRAINT upload_policy_signed CHECK (btrim(created_by) <> '' AND btrim(updated_by) <> ''),
    CONSTRAINT upload_policy_soft_delete_complete
        CHECK ((deleted_at IS NULL AND deleted_by IS NULL AND delete_reason IS NULL)
            OR (deleted_at IS NOT NULL AND deleted_by IS NOT NULL AND delete_reason IS NOT NULL))
);

COMMENT ON TABLE upload_policy IS
    'Gioi han tai len theo muc dich (ADR 0052 muc 10): max_bytes moi tep, kieu MIME cho phep, so tep '
    'moi ho so. Khong co dong cho mot muc dich -> tu choi tai len muc dich do. Ap chung moi xa.';
COMMENT ON COLUMN upload_policy.max_bytes IS
    'Tinh bang BYTE (10 MB = 10485760). Tran 5 GiB = gioi han mot lan POST cua S3.';
COMMENT ON COLUMN upload_policy.max_files_per_subject IS
    'NULL = Vihat chu y khong gioi han so tep; khong phai mac dinh. Co gia tri thi >= 1.';

-- Hard delete refused. The function is 0006's, reused rather than copied: same schema, same
-- message, and a second function would be a second place to fix the hint.
DROP TRIGGER IF EXISTS upload_policy_no_hard_delete ON upload_policy;
CREATE TRIGGER upload_policy_no_hard_delete
    BEFORE DELETE ON upload_policy
    FOR EACH ROW EXECUTE FUNCTION platform_cam_xoa_cung();

-- ---------------------------------------------------------------------------
-- platform_audit_log — who changed PLATFORM-WIDE configuration, what, when, from where.
--
-- A SEPARATE TABLE FROM audit_log, AND WITHOUT tenant_id (owner's decision, 2026-09-28). audit_log
-- is keyed and partitioned by tenant_id NOT NULL, and a change to a platform-wide limit belongs to
-- no commune. The two ways to force it in are both forbidden: a made-up "platform" tenant_id is a
-- default on the isolation path (rule 1, forbidden #1), and writing one entry per commune would
-- claim that each commune changed it. core/audit keeps its tenant requirement untouched.
--
-- WHAT RULE 6 ASKS, COLUMN BY COLUMN: who = actor (business code; 'system' for migrations and
-- jobs, rule 6 invariant 6), what = action, on which record = subject, when = occurred_at, from
-- which IP = actor_ip ('' when there is no request, as for a migration), in which commune = none,
-- by definition of this table. before/after hold the significant fields; this table's subjects
-- carry no personal data, and nothing that does may ever be written here (rule 3).
--
-- NOT PARTITIONED: a handful of rows a year, and no tenant_id to hash on.
--
-- APPEND-ONLY, enforced by triggers, the same shape as 0002 for audit_log: row-level BEFORE UPDATE
-- OR DELETE, statement-level BEFORE TRUNCATE (this table is not partitioned, so both attach to it
-- directly and there is no partition gap). The REVOKE argument of 0002 applies unchanged and is not
-- repeated: it belongs to database provisioning.
-- ---------------------------------------------------------------------------
-- @entity: PlatformAuditEntry
-- @scope:  platform
CREATE TABLE IF NOT EXISTS platform_audit_log (
    id          BIGSERIAL   PRIMARY KEY,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    actor       TEXT        NOT NULL,          -- business code, or 'system'
    actor_ip    TEXT        NOT NULL DEFAULT '',
    action      TEXT        NOT NULL,          -- business verb: 'upload_policy.seeded'
    subject     TEXT        NOT NULL,          -- the record's business key: a purpose
    before      JSONB,                         -- NULL when the record did not exist
    after       JSONB,                         -- NULL when the record stopped existing
    reason      TEXT,

    CONSTRAINT platform_audit_log_actor_not_blank CHECK (btrim(actor) <> ''),
    CONSTRAINT platform_audit_log_action_not_blank CHECK (btrim(action) <> ''),
    CONSTRAINT platform_audit_log_subject_not_blank CHECK (btrim(subject) <> ''),
    -- An entry recording nothing on either side is an entry with no content to defend.
    CONSTRAINT platform_audit_log_has_change CHECK (before IS NOT NULL OR after IS NOT NULL)
);

CREATE INDEX IF NOT EXISTS platform_audit_log_lookup
    ON platform_audit_log (subject, occurred_at DESC);

COMMENT ON TABLE platform_audit_log IS
    'Vet thay doi cau hinh CAP NEN TANG (khong thuoc xa nao). Chi ghi them: cam UPDATE, DELETE, '
    'TRUNCATE. Khong co tenant_id theo quyet dinh cua chu du an 28/09/2026 - khong dung xa gia.';

-- The message names the operation and the table, never the row: before/after travel nowhere
-- but this table (rule 3, forbidden #3).
CREATE OR REPLACE FUNCTION platform_audit_log_append_only() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'platform_audit_log is append-only: % on % refused', TG_OP, TG_TABLE_NAME
        USING HINT = 'Audit entries are archival records (rule 6, invariant 4): never '
                     'modified, never removed, never truncated. To correct a wrong entry, '
                     'append a new one.';
END $$;

DROP TRIGGER IF EXISTS platform_audit_log_no_update_delete ON platform_audit_log;
CREATE TRIGGER platform_audit_log_no_update_delete
    BEFORE UPDATE OR DELETE ON platform_audit_log
    FOR EACH ROW EXECUTE FUNCTION platform_audit_log_append_only();

DROP TRIGGER IF EXISTS platform_audit_log_no_truncate ON platform_audit_log;
CREATE TRIGGER platform_audit_log_no_truncate
    BEFORE TRUNCATE ON platform_audit_log
    FOR EACH STATEMENT EXECUTE FUNCTION platform_audit_log_append_only();

-- ---------------------------------------------------------------------------
-- SEED — the owner's values of 2026-09-28 (ADR 0052 §10 proposal, confirmed), and one trail entry
-- per seeded row, in this same transaction (rule 6, invariant 3).
--
-- MIME spellings are the core/storage constants (core/storage/mime.go); a test in
-- internal/store/upload_policy_test.go parses these rows and checks every value against
-- storage.ExtForMIME, so a typo here turns that test red instead of silently narrowing a policy.
--
-- ONE STATEMENT WRITES BOTH: the INSERT into upload_policy RETURNS what it inserted, and the trail
-- is built from that — so a row that already existed (a re-run) produces no second entry, and the
-- entry records exactly the values stored, not a second typing of them.
-- ---------------------------------------------------------------------------
WITH seeded AS (
    INSERT INTO upload_policy
        (purpose, max_bytes, allowed_mime_types, max_files_per_subject, created_by, updated_by)
    VALUES
        ('content-video',      2147483648, ARRAY['video/mp4', 'video/quicktime'],                           NULL, 'system', 'system'),
        ('content-image',        10485760, ARRAY['image/jpeg', 'image/png', 'image/webp', 'image/heic'],    NULL, 'system', 'system'),
        ('tenant-logo',          10485760, ARRAY['image/jpeg', 'image/png', 'image/webp', 'image/heic'],    NULL, 'system', 'system'),
        ('petition-photo',       10485760, ARRAY['image/jpeg', 'image/png', 'image/webp', 'image/heic'],       5, 'system', 'system'),
        ('document-scan',        52428800, ARRAY['application/pdf', 'image/jpeg', 'image/png'],              NULL, 'system', 'system'),
        ('content-attachment',   52428800, ARRAY['application/pdf'],                                         NULL, 'system', 'system')
    ON CONFLICT (purpose) DO NOTHING
    RETURNING purpose, max_bytes, allowed_mime_types, max_files_per_subject
)
INSERT INTO platform_audit_log (actor, action, subject, before, after, reason)
SELECT 'system', 'upload_policy.seeded', s.purpose, NULL,
       jsonb_build_object(
           'max_bytes', s.max_bytes,
           'allowed_mime_types', to_jsonb(s.allowed_mime_types),
           'max_files_per_subject', s.max_files_per_subject),
       'migration 0008_upload_policy.sql: initial values confirmed by the owner 2026-09-28 (ADR 0052 §10)'
FROM seeded s;

-- ---------------------------------------------------------------------------
-- REVERSAL (rule 7, invariant 4).
--
-- Down steps, in one transaction, while no operator has edited a policy (every environment today):
--   1. drop table upload_policy (its trigger goes with it);
--   2. drop table platform_audit_log, then the function platform_audit_log_append_only;
--   3. remove this file's progress row from the schema_migration table, keyed on
--      ten = '0008_upload_policy.sql'.
-- platform_cam_xoa_cung stays: 0006 owns it. Written as prose rather than as runnable lines,
-- because a runnable line is a line that gets run.
--
-- ONCE AN OPERATOR HAS CHANGED A POLICY it is no longer a reversal: platform_audit_log then holds
-- the only record of who set which limit, and discarding it is rule 6 forbidden #3 plus rule 7
-- stop condition #1 — the user decides, with a verified backup.
--
-- PER-COMMUNE AND RESUMABLE (rule 7, invariant 5): no commune column, no backfill, no existing row
-- rewritten — nothing to iterate and nothing to resume.
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
