-- 0021 — where identity keeps a commune's OWN Mini App's Zalo app secret, and the per-commune data
-- key that seals it. ADR 0066, both "Đã quyết 01/10/2026" sections:
--
--   * a commune's dedicated app signs in straight to ViGov, so identity must hold THAT App ID's
--     app secret to exchange accessToken/phoneToken (ADR 0066 §Quyết định);
--   * "Bảng mới thuộc service-identity: App ID, secret mã hoá AES-GCM, ngày đặt, người đặt" — sealed
--     with SECRET_ENCRYPTION_KEYS + core/crypto, one DEK per commune (ADR 0066 câu 6, ADR 0009);
--   * "Cột bật danh tính cố định theo App ID, mặc định tắt" — the `--demo` switch of a dedicated app
--     awaiting Zalo review (ADR 0066 §Đã quyết, row `--demo`).
--
-- WHY A TABLE AND NOT AN ENVIRONMENT VARIABLE: the secret differs per commune, and rule 1 invariant
-- 10 / rule 11 forbidden #6 keep per-commune values out of the environment. WHY SEALED AND NOT IN
-- THE CLEAR: rule 8 — the only platform secret is the KEK in SECRET_ENCRYPTION_KEYS (k8s Secret);
-- what this file stores is ciphertext that is random bytes without it (ADR 0009).
--
-- NAMED IN ENGLISH (rule 12, ADR 0051). `mini_app` follows service-platform's `mini_app` registry
-- (platform 0006), which is where an App ID is bound to its commune.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. ROWS PER COMMUNE: this file writes none. In use: one LIVE mini_app_secret row per dedicated
--      App ID of the commune (today one), plus one superseded row per past change; one
--      data_encryption_key row per commune, ever.
--   2. IF IT STOPS HALF-WAY: core/migrate runs the file in ONE transaction with its progress row;
--      every statement is IF NOT EXISTS / CREATE OR REPLACE / DROP TRIGGER IF EXISTS, so a retry
--      starts clean.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. READ PATHS THAT CHANGE MEANING WHILE HALF-APPLIED: none. Two new tables; nothing existing is
--      read differently. The dedicated-app sign-in route that reads them ships after this file.
--   5. RETENTION: nothing is ever removed. Both guards refuse DELETE; mini_app_secret keeps every
--      superseded version as a soft-deleted row (see "REPLACING" below).
--
-- TENANT SCOPE. Both tables are keyed by tenant_id first and HASH-partitioned ×32 on it (ADR 0010).
-- The App ID → commune binding is NOT decided here: service-platform's `mini_app` (platform scope,
-- app_id PRIMARY KEY) is the one place that says which commune an App ID belongs to. The sign-in
-- path resolves the commune there first and reads this table only inside that commune's context —
-- never an unscoped lookup by app_id across communes.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- data_encryption_key_guard — the floor under core/crypto.DEKStore's contract. Same body and same
-- reasons as service-comms 0008, repeated here because each service owns its own DEK table
-- (rule 2; core/crypto/envelope.go DEKStore: "in the owning service's own database").
--
--   DELETE             — a deleted DEK is every sealed secret of that commune destroyed, with no
--                        recovery path. Here that means the commune's app can no longer sign
--                        anybody in until an operator re-enters the secret.
--   tenant_id change   — the wrapped bytes are bound to the tenant_id (GCM additional data), so a
--                        moved row would not open in its new commune; refused at the write.
--
-- UPDATE of kek_id / wrapped IS allowed: that is Envelope.RewrapDEK during a KEK rotation, which
-- re-wraps the SAME data key, so nothing sealed under it changes.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION data_encryption_key_guard() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION '%: delete refused', TG_TABLE_NAME
            USING HINT = 'Deleting a data key destroys every secret sealed under it for that '
                         'commune (ADR 0009). Rows are only created and re-wrapped.';
    END IF;
    IF NEW.tenant_id IS DISTINCT FROM OLD.tenant_id THEN
        RAISE EXCEPTION '%: tenant_id is immutable', TG_TABLE_NAME
            USING HINT = 'A wrapped data key is bound to its commune and does not open elsewhere.';
    END IF;
    RETURN NEW;
END $$;

-- ---------------------------------------------------------------------------
-- data_encryption_key — ONE ROW PER COMMUNE: that commune's DEK, wrapped under a platform KEK from
-- SECRET_ENCRYPTION_KEYS. Ciphertext, not a secret (ADR 0009 §Vì sao đây KHÔNG vi phạm luật 8).
--
-- THE TABLE NAME MATCHES service-comms' ON PURPOSE, THE ENTITY NAME DOES NOT. Same shape, so one
-- crypto.DEKStore implementation pattern serves both; but comms already owns the entity
-- `DataEncryptionKey` (kb/30-indexes/data-ownership.json), and one entity name owned by two
-- services is the ambiguity rule 2 invariant 1 forbids. The two tables hold DIFFERENT keys: a
-- comms DEK does not open an identity secret, and the reverse.
-- ---------------------------------------------------------------------------
-- @entity: IdentityDataEncryptionKey
-- @scope:  tenant
CREATE TABLE IF NOT EXISTS data_encryption_key (
    tenant_id   TEXT        NOT NULL,
    -- 8 hex characters naming the KEK that wrapped this row (core/crypto/keyring.go). Its own
    -- column so a rotation run finds rows still under an old KEK without unwrapping anything.
    kek_id      TEXT        NOT NULL,
    wrapped     BYTEA       NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id),
    CONSTRAINT data_encryption_key_kek_id_shape CHECK (kek_id ~ '^[0-9a-f]{8}$'),
    -- 1 + 4 + 12 + 32 + 16: the version-1 wrapped format. A row of any other length cannot open.
    CONSTRAINT data_encryption_key_wrapped_length CHECK (octet_length(wrapped) = 65)
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS data_encryption_key_p%s PARTITION OF data_encryption_key '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

DROP TRIGGER IF EXISTS data_encryption_key_guard ON data_encryption_key;
CREATE TRIGGER data_encryption_key_guard
    BEFORE UPDATE OR DELETE ON data_encryption_key
    FOR EACH ROW EXECUTE FUNCTION data_encryption_key_guard();

-- ---------------------------------------------------------------------------
-- mini_app_secret_guard — a version row is IMMUTABLE; the only write after INSERT is retiring it.
--
-- REPLACING = SOFT-DELETE THE LIVE ROW + INSERT A NEW ONE, in one transaction, with one audit entry
-- that names the App ID and never the value (ADR 0066: "Thay = ghi bản mới, vết kiểm toán không chứa
-- giá trị"). That covers BOTH kinds of change — a new secret, and turning `--demo` on or off.
--
-- WHY NOT mail_settings' IN-PLACE UPDATE (service-comms 0008). mail_settings is one row per
-- commune, its history is the audit trail alone, and that was a fit for an SMTP password. Here the
-- ADR asks for a new version, and the row's own set_at / set_by are the answer to "since when, and
-- who, has this App ID signed citizens in with THIS secret / with demo identity on" — the question a
-- complaint about an account opened without phone verification asks (ADR 0066 §Hệ quả: the demo
-- identity is a session path with no phone check). Kept as rows, that answer survives log rotation
-- and needs no reconstruction from audit before/after values. One rule — every change is a new row
-- — also leaves the guard nothing to judge: it refuses every column change except retirement.
--
-- WHAT THE GUARD REFUSES:
--   DELETE                     — rule 7. Retire with deleted_at / deleted_by / delete_reason.
--   any change to a retired row — a superseded version is history (rule 7 forbidden #5).
--   any change of tenant_id, id, app_id, app_secret_sealed, demo_identity_enabled, set_at, set_by,
--   created_at                 — those ARE the version; changing one is a new version, i.e. a new row.
-- WHAT IT ALLOWS: on a LIVE row, setting the soft-delete trio (the CHECK below makes it all three or
-- none) and updated_at.
--
-- THE SEALED BYTES MAY BE CARRIED VERBATIM INTO THE NEXT VERSION (e.g. turning demo off keeps the
-- secret). That works only if the Envelope AAD binds the value to (table, column, commune, App ID)
-- and NOT to the row id — the convention for the store/app code: "mini_app_secret/app_secret_sealed/
-- <tenant_id>/<app_id>", the shape of comms' passwordAAD (internal/app/mail_settings.go). Bound to
-- the App ID, a secret copied onto another app's row does not open.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION mini_app_secret_guard() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION '%: delete refused', TG_TABLE_NAME
            USING HINT = 'Retire the row with deleted_at, deleted_by, delete_reason and insert the '
                         'new version (rule 7, ADR 0066).';
    END IF;
    IF OLD.deleted_at IS NOT NULL THEN
        RAISE EXCEPTION '%: a retired version is immutable', TG_TABLE_NAME
            USING HINT = 'Superseded rows are the history of this App ID; they are never edited.';
    END IF;
    IF NEW.tenant_id IS DISTINCT FROM OLD.tenant_id
       OR NEW.id IS DISTINCT FROM OLD.id
       OR NEW.app_id IS DISTINCT FROM OLD.app_id
       OR NEW.app_secret_sealed IS DISTINCT FROM OLD.app_secret_sealed
       OR NEW.demo_identity_enabled IS DISTINCT FROM OLD.demo_identity_enabled
       OR NEW.set_at IS DISTINCT FROM OLD.set_at
       OR NEW.set_by IS DISTINCT FROM OLD.set_by
       OR NEW.created_at IS DISTINCT FROM OLD.created_at THEN
        RAISE EXCEPTION '%: a version row is immutable', TG_TABLE_NAME
            USING HINT = 'To change the secret or the demo switch, retire this row and insert a '
                         'new one in the same transaction (ADR 0066).';
    END IF;
    RETURN NEW;
END $$;

-- ---------------------------------------------------------------------------
-- mini_app_secret — one version of the sign-in settings of ONE dedicated Mini App (App ID) of a
-- commune: its sealed Zalo app secret and its `--demo` identity switch.
--
-- AT MOST ONE LIVE ROW PER (commune, App ID) — `live_app_id` below. Retired rows are any number.
--
-- app_secret_sealed MAY BE NULL ONLY WHILE demo_identity_enabled IS TRUE. The case is real and is
-- stage 2 of ADR 0066's commune lifecycle: an app awaiting Zalo review is opened with `--demo`,
-- which calls no Zalo API at all ("Bản --demo không gọi lệnh Zalo nào"), so it can run before
-- anybody has entered a secret. Every other row must carry one: a row with no secret and demo off
-- is an App ID that can sign nobody in — the operator retires it instead of keeping it.
--
-- THE SECRET IS NEVER RETURNED TO A CLIENT, LOGGED, OR WRITTEN TO AUDIT (rule 8, ADR 0066). Reads
-- for an operator screen report only whether one is set and its set_at / set_by.
--
-- READ PATH: every read of the current settings filters `deleted_at IS NULL` (equivalently
-- `live_app_id = $1`) inside the commune's scoped context (rule 7 invariant 2, rule 1 invariant 5).
-- ---------------------------------------------------------------------------
-- @entity: MiniAppSecret
-- @scope:  tenant
CREATE TABLE IF NOT EXISTS mini_app_secret (
    tenant_id              TEXT        NOT NULL,
    id                     TEXT        NOT NULL,
    -- Zalo Mini App ID: digits only. Not a secret and not personal data. The bound is generous
    -- (Zalo App IDs seen are ~19 digits); it exists so a pasted secret or URL is refused here.
    app_id                 TEXT        NOT NULL,
    -- core/crypto Envelope.Seal output: 0x01 | nonce(12) | AES-256-GCM ciphertext + tag(16).
    app_secret_sealed      BYTEA,
    -- `--demo` for a dedicated app awaiting Zalo review: identity accepts {appId, demoIdentity:true}
    -- with no phoneToken and binds the fixed identity. OFF BY DEFAULT and must be off before the app
    -- is approved (ADR 0066) — a path that opens a citizen session without phone verification.
    demo_identity_enabled  BOOLEAN     NOT NULL DEFAULT false,
    -- When and by whom THIS version was set. Business code of the operator (rule 6 invariant 8:
    -- `VH-…` for ViHAT operations via operatorctl / platform-admin, the commune operator's code on
    -- premise — which is why the shape is not pinned to `VH-` here). No default: unsigned = refused.
    set_at                 TIMESTAMPTZ NOT NULL,
    set_by                 TEXT        NOT NULL,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at             TIMESTAMPTZ,
    deleted_by             TEXT,
    delete_reason          TEXT,

    -- THE LIVE-ROW KEY, AS A GENERATED COLUMN AND NOT A PARTIAL UNIQUE INDEX — the shape 0005 and
    -- 0008 (sla.linh_vuc_khoa) use, for their reason: a partial unique index on a partitioned table
    -- is not verifiable from this repository, and tools/check_khoa_duy_nhat.py refuses
    -- `UNIQUE … WHERE deleted_at IS NULL` because elsewhere it means a reissued code.
    --
    -- IT GOES NULL ON RETIREMENT, AND THAT IS CORRECT HERE: an App ID is not a code this commune
    -- issues — Zalo issues it, and service-platform's `mini_app` keeps the App ID → commune binding
    -- for good (app_id PRIMARY KEY, soft-deleted rows included). A new version for the same App ID
    -- is the same app's next setting, not a reused identifier.
    live_app_id            TEXT        GENERATED ALWAYS AS
                               (CASE WHEN deleted_at IS NULL THEN app_id END) STORED,

    -- Composite with tenant_id (rule 1, invariant 6); tenant_id first in both.
    PRIMARY KEY (tenant_id, id),
    UNIQUE (tenant_id, live_app_id),

    CONSTRAINT mini_app_secret_id_is_ulid CHECK (length(id) = 26),
    CONSTRAINT mini_app_secret_app_id_shape CHECK (app_id ~ '^[0-9]{1,32}$'),
    -- 1 + 12 + 16 bytes of envelope overhead, plus at least one byte of secret.
    CONSTRAINT mini_app_secret_sealed_length
        CHECK (app_secret_sealed IS NULL OR octet_length(app_secret_sealed) > 29),
    CONSTRAINT mini_app_secret_sealed_or_demo
        CHECK (app_secret_sealed IS NOT NULL OR demo_identity_enabled),
    CONSTRAINT mini_app_secret_set_by_shape
        CHECK (btrim(set_by) <> '' AND char_length(set_by) <= 64),
    CONSTRAINT mini_app_secret_soft_delete_complete
        CHECK ((deleted_at IS NULL AND deleted_by IS NULL AND delete_reason IS NULL)
            OR (deleted_at IS NOT NULL
                AND btrim(deleted_by) <> '' AND char_length(deleted_by) <= 64
                AND btrim(delete_reason) <> ''))
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS mini_app_secret_p%s PARTITION OF mini_app_secret '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

DROP TRIGGER IF EXISTS mini_app_secret_guard ON mini_app_secret;
CREATE TRIGGER mini_app_secret_guard
    BEFORE UPDATE OR DELETE ON mini_app_secret
    FOR EACH ROW EXECUTE FUNCTION mini_app_secret_guard();

COMMENT ON TABLE mini_app_secret IS
    'Cau hinh dang nhap cua app rieng cua xa theo App ID (ADR 0066): secret Zalo da niem phong '
    '(core/crypto) va cong tac danh tinh co dinh --demo. Moi thay doi = xoa mem dong song + them '
    'dong moi. Khong bao gio tra secret ra client, log hay vet kiem toan.';
COMMENT ON COLUMN mini_app_secret.demo_identity_enabled IS
    'Bat danh tinh co dinh --demo cho app dang cho Zalo duyet. Mac dinh tat; phai tat truoc khi app '
    'duoc duyet (ADR 0066).';

-- ---------------------------------------------------------------------------
-- WHAT THIS FILE DOES NOT STOP: TRUNCATE and DDL by the table owner — the same line comms 0008,
-- identity 0003 and ADR 0013 draw.
--
-- REVERSAL (rule 7, invariant 4). By hand, in ONE transaction, after the Go code that reads the
-- tables has been rolled back:
--
--   WHILE BOTH TABLES ARE EMPTY — exact and loses nothing: drop mini_app_secret, then
--   data_encryption_key (partitions and triggers go with them), then the functions
--   mini_app_secret_guard and data_encryption_key_guard, and remove this file's row from
--   schema_migration so the runner applies it again.
--
--   ONCE A COMMUNE HAS A ROW — dropping data_encryption_key destroys that commune's sealed secrets
--   permanently, and dropping mini_app_secret destroys the record of who set which secret and who
--   turned demo identity on, and when. Neither is a reversal: rule 7 stop condition #2, it needs
--   the user and a verified backup. Written as prose, not a runnable line, because a runnable line
--   is a line that gets run.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- BACKSTOP, carried forward from 0002–0020: a table declared PARTITION BY and given no partitions
-- REJECTS EVERY INSERT, silently, until the first real write. Every file that declares one ends with it.
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
