-- comms — the commune's own mail server (docs/ui-ux/14-cau-hinh.md §10, "Máy chủ thư") and the
-- per-commune data key that seals its password (ADR 0009, amendment of 28/09/2026).
--
-- WHY A NEW FILE: 0001-0007 have been applied and core/migrate compares the checksum of every
-- applied file at startup (see 0003's header).
--
-- NAMED IN ENGLISH (rule 12, ADR 0051). The specification's `cau_hinh_thu(… mat_khau_ma_hoa …)` is
-- the design-time name; the columns here are `password_sealed` and friends.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS, answered before the SQL.
--
--   1. HOW MANY ROWS PER COMMUNE: at most one in each table, by construction — tenant_id IS the
--      primary key of both. This file writes no row.
--   2. IF IT STOPS HALF-WAY: one file, one transaction; every statement is IF NOT EXISTS or
--      CREATE OR REPLACE, so a retry from the beginning costs nothing.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. WHICH READ PATHS CHANGE MEANING WHILE IT IS HALF-APPLIED: none. Two new tables, two new
--      functions, two new triggers; nothing existing is touched.
--   5. RETENTION: nothing is ever removed from either table — both triggers refuse DELETE.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- data_encryption_key_guard — the floor under core/crypto.DEKStore's contract.
--
--   DELETE             — a deleted DEK is every sealed secret of that commune destroyed, with no
--                        recovery path (core/crypto/keyring.go:18-22). Rule 7, and worse: the row
--                        cannot even be restored from a backup without also holding the KEK.
--   tenant_id change   — the wrapped bytes are bound to the tenant_id (the GCM additional data),
--                        so moving a row to another commune yields a DEK that does not open there;
--                        refusing it keeps the failure at the write, not at the next Open.
--
-- UPDATE of kek_id / wrapped IS allowed: that is Envelope.RewrapDEK during a KEK rotation, and it
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
-- @entity: DataEncryptionKey
-- @scope:  tenant
--
-- data_encryption_key — ONE ROW PER COMMUNE: that commune's data key (DEK), wrapped under a
-- platform KEK from SECRET_ENCRYPTION_KEYS. It is ciphertext, not a secret: without the KEK these
-- bytes are random (ADR 0009 §Vì sao đây KHÔNG vi phạm luật 8).
--
-- Owned by comms because the secrets it opens live in comms' schema (rule 2 — a DEK table per
-- owning service, core/crypto/doc). Written ONLY through store.DataEncryptionKeyStore, which
-- implements crypto.DEKStore.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS data_encryption_key (
    tenant_id   TEXT        NOT NULL,
    -- 8 hex characters naming the KEK that wrapped this row (core/crypto/keyring.go:55-58). A
    -- column of its own so a rotation run can find rows still under an old KEK without unwrapping.
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
-- mail_settings_guard — refuses DELETE and a change of commune.
--
-- NO SOFT-DELETE COLUMNS, AND THAT IS A DECISION, NOT AN OMISSION: this is one configuration row
-- per commune, overwritten in place, and nothing in §10 or in ../vigov-require removes it — the
-- screen turns it off with `Dùng máy chủ thư này cho xã` (is_enabled). A row that is never deleted
-- has no deleted_at to exclude on the read path. Every change is an audit entry with before/after
-- (rule 6), which is the history; the DELETE refusal below is the floor that keeps "never deleted"
-- true for every writer, psql included.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION mail_settings_guard() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION '%: delete refused', TG_TABLE_NAME
            USING HINT = 'Turn the mail server off with is_enabled = false instead.';
    END IF;
    IF NEW.tenant_id IS DISTINCT FROM OLD.tenant_id THEN
        RAISE EXCEPTION '%: tenant_id is immutable', TG_TABLE_NAME
            USING HINT = 'The sealed password is bound to its commune and does not open elsewhere.';
    END IF;
    RETURN NEW;
END $$;

-- ---------------------------------------------------------------------------
-- @entity: MailSettings
-- @scope:  tenant
--
-- mail_settings — the commune's own SMTP server: internal notifications go out from the commune's
-- official address (§10). One row per commune.
--
-- THE PASSWORD IS NEVER STORED IN THE CLEAR — `password_sealed` is core/crypto.Envelope output,
-- AES-256-GCM under the commune's DEK (data_encryption_key above), bound to the commune and to this
-- table and column by its additional data. This is where v2 deliberately differs from
-- ../vigov-require, which keeps `password VARCHAR(512)` in the clear (ADR 0009 §Vì sao ngược kho
-- yêu cầu). It is never returned to a client: reads report only whether one is set.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS mail_settings (
    tenant_id        TEXT        NOT NULL,
    host             TEXT        NOT NULL,           -- "smtp.danang.gov.vn"
    port             INT         NOT NULL,           -- 587
    -- starttls | tls. THERE IS NO PLAINTEXT VALUE, although §10's sketch lists 'none': the
    -- password crosses this connection, and rule 13 #1 wants traffic encrypted and verified.
    security         TEXT        NOT NULL,
    username         TEXT        NOT NULL,
    from_address     TEXT        NOT NULL,           -- "ubnd@xa.danang.gov.vn"
    from_name        TEXT        NOT NULL DEFAULT '',
    is_enabled       BOOLEAN     NOT NULL DEFAULT false,
    password_sealed  BYTEA       NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- The BUSINESS CODE of the last person to save (rule 6, invariant 8) — never an internal id.
    updated_by       TEXT        NOT NULL,
    PRIMARY KEY (tenant_id),
    CONSTRAINT mail_settings_security_known CHECK (security IN ('starttls', 'tls')),
    CONSTRAINT mail_settings_port_range CHECK (port BETWEEN 1 AND 65535),
    CONSTRAINT mail_settings_host_length CHECK (char_length(host) BETWEEN 1 AND 253),
    CONSTRAINT mail_settings_username_length CHECK (char_length(username) BETWEEN 1 AND 254),
    CONSTRAINT mail_settings_from_address_length CHECK (char_length(from_address) BETWEEN 3 AND 254),
    CONSTRAINT mail_settings_from_name_length CHECK (char_length(from_name) <= 100),
    -- 1 + 12 + 16 bytes of envelope overhead, plus at least one byte of password.
    CONSTRAINT mail_settings_password_sealed_length CHECK (octet_length(password_sealed) > 29)
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS mail_settings_p%s PARTITION OF mail_settings '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

DROP TRIGGER IF EXISTS mail_settings_guard ON mail_settings;
CREATE TRIGGER mail_settings_guard
    BEFORE UPDATE OR DELETE ON mail_settings
    FOR EACH ROW EXECUTE FUNCTION mail_settings_guard();

-- ---------------------------------------------------------------------------
-- WHAT THIS FILE DOES NOT STOP: TRUNCATE and DDL by the table owner — the same line 0003 and
-- ADR 0013 draw.
--
-- REVERSAL (question 3). While both tables are empty the reversal is complete and loses nothing:
-- drop both tables (partitions and triggers go with them), then both functions, and in the same
-- transaction remove this file's row from `schema_migration`. Once a commune has a row in
-- data_encryption_key, dropping it DESTROYS that commune's stored secrets permanently — that is
-- not a reversal, it is rule 7's first stop condition and needs the user.
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
