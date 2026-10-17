-- 0036 — DYNAMIC ATTRIBUTES of the citizen directory, the static `email` column, the identity EVENT
-- OUTBOX, and the permission key `admin.citizen_field` (ADR 0101, owner decisions 17/10/2026).
--
--   citizen_field                one attribute a commune defines for its directory ("Hộ nghèo", "Ngày sinh")
--   citizen_contact.email        the static e-mail column (ADR 0101 #6)
--   citizen_contact.attributes   the values of the commune's attributes for one person, {code: value}
--   event_outbox                 facts identity owes to Kafka, written in the business transaction
--   quyen admin.citizen_field    "Cấu hình thuộc tính danh bạ người dân", pre-granted to admin.zalo holders
--
-- WHY A NEW FILE AND NOT AN EDIT TO 0033: core/migrate checksums every applied file at startup and stops
-- the service when one has changed (ErrChecksumLech).
--
-- NAMES (rule 12, ADR 0051): English. The six `field_type` VALUES are the owner's own spelling
-- (singleText, multiText, combobox, singleBox, number, dateTime) — a deliberate departure from ADR 0011's
-- Vietnamese enum values, recorded in ADR 0101 #2. `number_kind` is 'integer' | 'decimal', the words of
-- the same decision.
--
-- ---------------------------------------------------------------------------
-- citizen_field — CONFIGURATION, NOT GROWING WITH CITIZENS: ≤ 50 live rows per commune (ADR 0101 #1,
-- enforced by the application, which counts inside the write transaction). A plain table (ADR 0096 #3).
--
-- THE KEY IS (tenant_id, code) AND A RETIRED CODE IS NEVER REUSED. The code is what every stored value
-- in citizen_contact.attributes is keyed by; "ngừng dùng" keeps the values (ADR 0101 #4), so a second
-- field under the same code would silently adopt another field's history. Hence no live_key here, unlike
-- 0033: the code is an issued identifier in the sense of rule 7, invariant 3.
--
-- WHAT THE SCHEMA REFUSES, so a defect in the application cannot write it:
--   * a code outside [a-z][a-z0-9]{5,7}; a type outside the six;
--   * number_kind on anything but `number` (and its absence on `number`);
--   * date_pattern on anything but `dateTime`, or outside the four display patterns;
--   * options on anything but combobox / singleBox, or an option list that is not an array of
--     {value, label, retired?} with value ~ '^[a-z0-9]+(-[a-z0-9]+)*$' (citizen_field_options_valid);
--   * a placeholder on anything but the two text types (ADR 0101 #1: "placeholder (ô chữ)");
--   * through the TRIGGER: changing code or field_type, REMOVING an option value (a value already chosen
--     for a person must stay resolvable — ADR 0101 #1 "chỉ được ngừng, không xoá"), and any hard delete.
-- Option values unique within a field, and ≤ 50 live fields, are the application's (a CHECK cannot count
-- rows; a unique-in-array check is clearer in Go).
--
-- AUDIT: written by the staff routes of this service in the same transaction as their audit_log entry
-- (rule 6). No trigger writes the trail.
--
-- ---------------------------------------------------------------------------
-- citizen_contact.email / .attributes — ADDITIVE ON A TABLE THAT MAY HOLD ROWS (0033 shipped 16/10/2026).
-- `email` is a NULL column with no default; `attributes` is NOT NULL with a CONSTANT default '{}', which
-- PostgreSQL 11+ records in the catalogue without rewriting the table (the cluster is 16). No existing
-- value is read, changed or moved. The two CHECKs hold for every existing row by construction (NULL e-mail,
-- '{}' attributes), so validating them is a scan, not a change.
--
-- PERSONAL DATA (rule 3). `email` is personal data: masked at the API (MaskEmail), revealed only through
-- the audited route, never logged, never in a URL, a cache key or an event; on Elasticsearch only as keyed
-- hashes (ADR 0101 #8). `attributes` MAY be personal data — the commune names its own attributes, so the
-- software cannot tell "Ngày sinh" from "Sở thích" (ADR 0101 §Hệ quả, risk accepted by the owner).
--
-- WHY THE VALUES ARE JSONB ON THE CONTACT AND NOT AN EAV TABLE: one person's values are read and written
-- together (the form saves the whole set), and the search reads them from Elasticsearch, not from here. An
-- EAV table would multiply rows by up to 50 per person on a partitioned table for no query that needs it.
-- Values of a RETIRED field stay in the object untouched (ADR 0101 #4).
--
-- ---------------------------------------------------------------------------
-- event_outbox — facts identity owes to Kafka (ADR 0101 #11; the outbox shape of petitions 0036). One row
-- per fact, written IN the transaction that wrote the fact; a relay publishes after the commit and stamps
-- `published_at`. GROWS WITH WRITES → PARTITION BY HASH (tenant_id), MODULUS 8 (ADR 0096 #1). The payload
-- carries NO personal data (proto/vigov/identity/v1/events.proto). Rows are infrastructure state, kept,
-- never deleted by the service (rule 7 is about business data, but nothing here needs a delete either).
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. ROWS PER COMMUNE: citizen_field and event_outbox start empty. citizen_contact gains two columns on
--      its existing rows (catalogue-only, no rewrite). Catalogue: one row in `quyen`. Grants: one
--      `vai_tro_quyen` row and one audit_log entry per live role holding admin.zalo (~3 per commune) —
--      measure first exactly as 0035 says, with quyen_ma = 'admin.zalo'.
--   2. IF IT STOPS HALF-WAY: it cannot land half-applied — one file, one transaction with its progress row
--      (core/migrate). Every statement is IF NOT EXISTS / ON CONFLICT / guarded, so a retry costs nothing.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. READ PATHS THAT CHANGE MEANING: none while it runs (one transaction). After it: SELECTs that name
--      columns are unchanged; the directory routes start reading `email` / `attributes`; the Phân quyền
--      matrix shows one more key in QUẢN TRỊ.
--   5. RETENTION: nothing removed, retyped or renumbered. A field definition is soft-retired, never
--      deleted; values are kept; grants are audited with the system principal (rule 6, invariant 6).
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- 1. citizen_field
-- ---------------------------------------------------------------------------

-- The shape of one option list. IMMUTABLE and free of table reads, so a CHECK may call it.
CREATE OR REPLACE FUNCTION citizen_field_options_valid(o jsonb) RETURNS boolean
LANGUAGE sql IMMUTABLE AS $$
    SELECT jsonb_typeof(o) = 'array'
       AND jsonb_array_length(o) BETWEEN 1 AND 100
       AND NOT EXISTS (
           SELECT 1 FROM jsonb_array_elements(o) AS e
           WHERE jsonb_typeof(e) <> 'object'
              OR jsonb_typeof(e->'value') IS DISTINCT FROM 'string'
              OR (e->>'value') !~ '^[a-z0-9]+(-[a-z0-9]+)*$'
              OR char_length(e->>'value') > 40
              OR jsonb_typeof(e->'label') IS DISTINCT FROM 'string'
              OR btrim(e->>'label') = ''
              OR char_length(e->>'label') > 100
              OR (e ? 'retired' AND jsonb_typeof(e->'retired') <> 'boolean'))
$$;

-- @entity: CitizenField
-- @scope:  tenant
CREATE TABLE IF NOT EXISTS citizen_field (
    tenant_id      TEXT        NOT NULL,
    -- Immutable after creation (trigger), never reused (header). 6–8 characters, a letter first.
    code           TEXT        NOT NULL,
    label          TEXT        NOT NULL,
    -- singleText | multiText | combobox | singleBox | number | dateTime (ADR 0101 #2). Immutable.
    field_type     TEXT        NOT NULL,
    placeholder    TEXT,
    -- [{"value": "ho-ngheo", "label": "Hộ nghèo", "retired": false}, …] for combobox / singleBox only.
    options        JSONB,
    -- 'integer' | 'decimal' for number only.
    number_kind    TEXT,
    -- One of four display patterns, for dateTime only. The VALUE is stored as epoch millis regardless.
    date_pattern   TEXT,
    required       BOOLEAN     NOT NULL DEFAULT false,
    show_in_list   BOOLEAN     NOT NULL DEFAULT false,
    -- The drag-and-drop order of the configuration screen (ADR 0101 #3).
    sort_order     INTEGER     NOT NULL DEFAULT 0,

    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- "Ngừng dùng" (ADR 0101 #4): hidden from the form and the filters, values kept. `retired_by` is the
    -- staff business code (rule 6, invariant 8).
    retired_at     TIMESTAMPTZ,
    retired_by     TEXT,
    retire_reason  TEXT,

    -- Composite with tenant_id (rule 1, invariant 6).
    PRIMARY KEY (tenant_id, code),

    CONSTRAINT citizen_field_code_shape CHECK (code ~ '^[a-z][a-z0-9]{5,7}$'),
    CONSTRAINT citizen_field_label_shape CHECK (btrim(label) <> '' AND char_length(label) <= 100),
    CONSTRAINT citizen_field_type_valid
        CHECK (field_type IN ('singleText', 'multiText', 'combobox', 'singleBox', 'number', 'dateTime')),
    CONSTRAINT citizen_field_placeholder_text_only
        CHECK (placeholder IS NULL
            OR (field_type IN ('singleText', 'multiText') AND char_length(placeholder) <= 200)),
    CONSTRAINT citizen_field_number_kind_valid
        CHECK ((field_type = 'number' AND number_kind IN ('integer', 'decimal'))
            OR (field_type <> 'number' AND number_kind IS NULL)),
    CONSTRAINT citizen_field_date_pattern_valid
        CHECK ((field_type = 'dateTime' AND date_pattern IN ('dd/MM/yyyy', 'dd/MM/yyyy HH:mm', 'MM/yyyy', 'yyyy'))
            OR (field_type <> 'dateTime' AND date_pattern IS NULL)),
    CONSTRAINT citizen_field_options_valid
        CHECK ((field_type IN ('combobox', 'singleBox') AND options IS NOT NULL AND citizen_field_options_valid(options))
            OR (field_type NOT IN ('combobox', 'singleBox') AND options IS NULL)),
    CONSTRAINT citizen_field_sort_order_range CHECK (sort_order BETWEEN 0 AND 9999),
    CONSTRAINT citizen_field_retire_complete
        CHECK ((retired_at IS NULL AND retired_by IS NULL AND retire_reason IS NULL)
            OR (retired_at IS NOT NULL AND retired_by IS NOT NULL AND btrim(retired_by) <> ''
                AND retire_reason IS NOT NULL AND btrim(retire_reason) <> ''
                AND char_length(retire_reason) <= 500))
);

COMMENT ON TABLE citizen_field IS
    'Thuoc tinh dong cua danh ba nguoi dan theo tung xa (ADR 0101, chu du an 17/10/2026). Ma va loai '
    'khong doi; ngung dung = an, giu gia tri; khong xoa that; ma da cap khong dung lai.';

-- The configuration screen and the form: one commune's fields in the order it set.
CREATE INDEX IF NOT EXISTS citizen_field_order ON citizen_field (tenant_id, sort_order, code);

-- The guard: code and type immutable, an option value never removed, no hard delete.
CREATE OR REPLACE FUNCTION citizen_field_keep() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'citizen_field: hard delete refused'
            USING HINT = 'Stored values in citizen_contact.attributes are keyed by this code: retire the '
                         'field (retired_at, retired_by, retire_reason) — rule 7, ADR 0101 #4.';
    END IF;
    IF NEW.tenant_id IS DISTINCT FROM OLD.tenant_id OR NEW.code IS DISTINCT FROM OLD.code THEN
        RAISE EXCEPTION 'citizen_field: `code` is immutable'
            USING HINT = 'Every stored value is keyed by the code (ADR 0101 #1). A new field is a new row.';
    END IF;
    IF NEW.field_type IS DISTINCT FROM OLD.field_type THEN
        RAISE EXCEPTION 'citizen_field: `field_type` is immutable'
            USING HINT = 'Values already stored were validated against the old type (ADR 0101 #1).';
    END IF;
    IF OLD.options IS NOT NULL AND EXISTS (
        SELECT 1 FROM jsonb_array_elements(OLD.options) AS o
        WHERE NOT EXISTS (SELECT 1 FROM jsonb_array_elements(coalesce(NEW.options, '[]'::jsonb)) AS n
                          WHERE n->>'value' = o->>'value')) THEN
        RAISE EXCEPTION 'citizen_field: an option value cannot be removed'
            USING HINT = 'A value chosen for a person must stay resolvable: retire the option instead '
                         '("retired": true) — ADR 0101 #1.';
    END IF;
    RETURN NEW;
END $$;

DROP TRIGGER IF EXISTS citizen_field_keep ON citizen_field;
CREATE TRIGGER citizen_field_keep
    BEFORE UPDATE OR DELETE ON citizen_field
    FOR EACH ROW EXECUTE FUNCTION citizen_field_keep();

-- ---------------------------------------------------------------------------
-- 2. citizen_contact: email and attributes (additive — header)
-- ---------------------------------------------------------------------------
ALTER TABLE citizen_contact ADD COLUMN IF NOT EXISTS email TEXT;
ALTER TABLE citizen_contact ADD COLUMN IF NOT EXISTS attributes JSONB NOT NULL DEFAULT '{}'::jsonb;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'citizen_contact_email_shape'
                   AND conrelid = 'citizen_contact'::regclass) THEN
        -- Lower case, one '@', a dot in the domain, no whitespace, ≤ 254 (RFC 5321's path limit). The
        -- application normalises; the schema refuses what it missed.
        ALTER TABLE citizen_contact ADD CONSTRAINT citizen_contact_email_shape
            CHECK (email IS NULL OR (char_length(email) <= 254 AND email = lower(email)
                   AND email ~ '^[^@[:space:]]+@[^@[:space:]]+\.[^@[:space:]]+$'));
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'citizen_contact_attributes_object'
                   AND conrelid = 'citizen_contact'::regclass) THEN
        ALTER TABLE citizen_contact ADD CONSTRAINT citizen_contact_attributes_object
            CHECK (jsonb_typeof(attributes) = 'object');
    END IF;
END $$;

COMMENT ON COLUMN citizen_contact.email IS
    'DU LIEU CA NHAN (luat 3). Chu thuong. Che khi ra API (MaskEmail), xem day du qua tuyen co ghi vet. '
    'Tren Elasticsearch chi o dang bam co khoa (ADR 0101 #8).';
COMMENT ON COLUMN citizen_contact.attributes IS
    'Gia tri thuoc tinh dong {ma: gia tri} theo citizen_field cua xa (ADR 0101). Co the la du lieu ca nhan. '
    'Gia tri cua thuoc tinh da ngung dung duoc giu nguyen.';

-- ---------------------------------------------------------------------------
-- 3. event_outbox
-- ---------------------------------------------------------------------------
-- @entity: IdentityEventOutbox
-- @scope:  tenant
CREATE TABLE IF NOT EXISTS event_outbox (
    tenant_id     TEXT        NOT NULL,
    -- ULID; also the envelope id (deduplication — queues deliver at least once).
    id            TEXT        NOT NULL,
    -- The fact WITH ITS VERSION (rule 2, invariant 4): `identity.citizen_contact_changed.v1`.
    name          TEXT        NOT NULL,
    -- protojson of the event message. NO PERSONAL DATA (proto/vigov/identity/v1/events.proto).
    payload       JSONB       NOT NULL,
    -- When the ACT happened — not when the relay published it.
    occurred_at   TIMESTAMPTZ NOT NULL,
    -- NULL until the broker acknowledged the record.
    published_at  TIMESTAMPTZ,
    -- Unsuccessful publish attempts — a count an operator reads; nothing decides on it.
    attempts      INTEGER     NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- Composite with tenant_id (rule 1, invariant 6).
    PRIMARY KEY (tenant_id, id),
    CONSTRAINT event_outbox_id_ulid CHECK (length(id) = 26),
    CONSTRAINT event_outbox_name_versioned CHECK (name ~ '^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)+\.v[1-9][0-9]*$'),
    CONSTRAINT event_outbox_payload_object CHECK (jsonb_typeof(payload) = 'object'),
    CONSTRAINT event_outbox_attempts_non_negative CHECK (attempts >= 0)
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..7 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS event_outbox_p%s PARTITION OF event_outbox '
            'FOR VALUES WITH (MODULUS 8, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

COMMENT ON TABLE event_outbox IS
    'Su kien identity no Kafka, ghi cung giao dich voi hanh vi (ADR 0101 #11). Khong chua du lieu ca nhan. '
    'Bo chuyen danh dau published_at sau khi broker xac nhan.';

-- The relay's read: one commune's unpublished rows, oldest act first.
CREATE INDEX IF NOT EXISTS event_outbox_pending
    ON event_outbox (tenant_id, occurred_at, id)
    WHERE published_at IS NULL;

-- ---------------------------------------------------------------------------
-- 4. THE PERMISSION KEY and its pre-grant — 0035's shape exactly (ADR 0101 #5).
--
-- Rank 53: ranks used so far are 1-52 (0001, 0007, 0010, 0022, 0028, 0032, 0035); the key goes last in
-- QUẢN TRỊ, after admin.sms (52). migrations/danh_ba_mini_app_test.go forbids a shared rank.
--
-- NEW COMMUNES: the administrator role receives every catalogue key (store.CapMoiQuyen); the template
-- roles that carry admin.zalo (Chủ tịch UBND, Chánh Văn phòng) carry admin.citizen_field in
-- internal/domain/role_template.go, changed in the same commit.
-- ---------------------------------------------------------------------------
INSERT INTO quyen (ma, nhom, nhan, thu_tu) VALUES
    ('admin.citizen_field', 'QUẢN TRỊ', 'Cấu hình thuộc tính danh bạ người dân', 53)
ON CONFLICT (ma) DO UPDATE SET
    nhom   = EXCLUDED.nhom,
    nhan   = EXCLUDED.nhan,
    thu_tu = EXCLUDED.thu_tu;

-- @cross-tenant: one-off system migration granting admin.citizen_field to every live role, in every
-- commune, that already holds admin.zalo (owner decision 17/10/2026, ADR 0101 #5); each row keeps its own
-- tenant_id and each role gets its own entry in its own commune.
WITH grant_rule (held, granted) AS (
    VALUES ('admin.zalo', 'admin.citizen_field')
), target AS (
    SELECT DISTINCT vq.tenant_id, vq.vai_tro_id, r.granted
    FROM vai_tro_quyen vq
    JOIN vai_tro vt ON vt.tenant_id = vq.tenant_id AND vt.id = vq.vai_tro_id
    JOIN grant_rule r ON r.held = vq.quyen_ma
    WHERE vt.deleted_at IS NULL
), inserted AS (
    INSERT INTO vai_tro_quyen (tenant_id, vai_tro_id, quyen_ma, cap_luc, cap_boi)
    SELECT t.tenant_id, t.vai_tro_id, t.granted, now(), 'system'
    FROM target t
    ON CONFLICT (tenant_id, vai_tro_id, quyen_ma) DO NOTHING
    RETURNING tenant_id, vai_tro_id, quyen_ma
), per_role AS (
    SELECT i.tenant_id, i.vai_tro_id,
           array_agg(i.quyen_ma ORDER BY i.quyen_ma) AS added,
           COALESCE((SELECT array_agg(vq.quyen_ma ORDER BY vq.quyen_ma)
                     FROM vai_tro_quyen vq
                     WHERE vq.tenant_id = i.tenant_id AND vq.vai_tro_id = i.vai_tro_id),
                    ARRAY[]::text[]) AS held_before
    FROM inserted i
    GROUP BY i.tenant_id, i.vai_tro_id
)
INSERT INTO audit_log (tenant_id, actor_id, actor_kind, actor_ip, action, subject, at, delta)
SELECT tenant_id, 'system', 'system', '', 'luu_phan_quyen_vai_tro', vai_tro_id, now(),
       jsonb_build_object(
           'truoc', to_jsonb(held_before),
           'sau',   to_jsonb(ARRAY(SELECT x FROM unnest(held_before || added) AS x ORDER BY x)),
           'them',  to_jsonb(added),
           'bo',    '[]'::jsonb,
           'ly_do', 'migration 0036: quyền cấu hình thuộc tính danh bạ người dân, cấp sẵn cho vai trò đang giữ admin.zalo (chủ dự án 17/10/2026, ADR 0101)')
FROM per_role;

-- Measured per commune (tenant ids and counts only — nothing personal).
DO $$
DECLARE r record;
BEGIN
    FOR r IN
        SELECT tenant_id, count(*) AS n
        FROM audit_log
        WHERE action = 'luu_phan_quyen_vai_tro' AND actor_kind = 'system'
          AND delta->>'ly_do' = 'migration 0036: quyền cấu hình thuộc tính danh bạ người dân, cấp sẵn cho vai trò đang giữ admin.zalo (chủ dự án 17/10/2026, ADR 0101)'
        GROUP BY tenant_id
        ORDER BY tenant_id
    LOOP
        RAISE NOTICE '0036: commune % — % role(s) gained admin.citizen_field', r.tenant_id, r.n;
    END LOOP;
END $$;

-- ---------------------------------------------------------------------------
-- REVERSAL (rule 7, invariant 4). By a person, in ONE transaction; core/migrate has no automatic rollback
-- (ADR 0013). Prose, not runnable lines, because a runnable line gets run. FIRST revert the commit whose
-- routes use these objects.
--
--   THE GRANTS AND THE KEY — exactly as 0035's REVERSAL, with ly_do LIKE 'migration 0036:%' and
--   ma = 'admin.citizen_field'; withdraw grants through the Phân quyền screen, never by hand; revert the
--   template change in domain.RoleTemplates.
--   event_outbox — droppable once every row has published_at set (nothing personal, infrastructure state).
--   citizen_field, citizen_contact.email, citizen_contact.attributes — WHILE all three are empty (no
--   citizen_field row, every email NULL, every attributes = '{}'): drop the two constraints and two
--   columns, the trigger, the two functions and the table, then remove this file's row from
--   schema_migration (ten = '0036_citizen_field.sql'). Exact; loses nothing.
--   ONCE A VALUE EXISTS — no reversal by command: e-mail addresses and attribute values are personal data
--   staff typed (rule 7 stop condition #2). The owner's decision, with a verified backup.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- BACKSTOP, carried forward from 0002–0035: every partitioned table in this schema must have
-- partitions, or it rejects every INSERT.
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
                         'MODULUS 8 partition loop (ADR 0096) in the same migration that '
                         'declares PARTITION BY.';
    END IF;
END $$;
