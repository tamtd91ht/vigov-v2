-- identity — the Vihat operator realm: accounts, sessions, permission grants, recovery codes and
-- the operator trail (ADR 0048 §Chốt của chủ dự án — 28/09/2026, step 1 of "Thứ tự dựng").
--
-- WHY IN service-identity, IN TABLES OF ITS OWN (owner's decision #1): identity already holds
-- password hashing and a session registry; putting operators in service-platform would duplicate
-- the sign-in code. SEPARATE tables, not a kind column on nguoi_dung, so that no query written for
-- commune staff can ever reach an operator row, and no operator query can reach a staff row.
--
-- NO TABLE HERE HAS tenant_id, AND THAT IS THE DECISION, NOT AN OMISSION. An operator belongs to no
-- commune: the realm exists to manage platform metadata ACROSS communes. Inventing a "platform"
-- tenant_id to fill the gap would be a default on the isolation path (rule 1, forbidden #1). Every
-- table therefore declares `@scope: platform`, and every Go statement that reads them carries its
-- own `// @cross-tenant:` mark (internal/store/operatorstore).
--
-- WHY A NEW FILE AND NOT AN EDIT OF 0001–0011: core/migrate checksums every applied file at startup
-- and stops the service when one has changed (ErrChecksumLech).
--
-- WHY ALL FIVE TABLES IN ONE FILE: core/migrate gives each file exactly ONE transaction, and four of
-- the five reference operator_account. Splitting them would create an intermediate state with a
-- foreign key pointing at nothing. All five, or none.
--
-- THE FIVE MIGRATION QUESTIONS, answered before the SQL:
--
--   1. HOW MANY ROWS PER COMMUNE: none — no commune column exists here. Platform-wide, a handful of
--      operator accounts (Vihat staff), a few sessions per working day, six grants per account at
--      most, ten recovery codes per batch, and one trail row per sign-in or administrative act.
--   2. IF IT STOPS HALF-WAY: it cannot land half-applied. One file, one transaction, progress row
--      inside it. Every statement is IF NOT EXISTS / CREATE OR REPLACE / DROP TRIGGER IF EXISTS, so
--      a retry costs nothing.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. WHICH READ PATHS CHANGE MEANING WHILE IT IS HALF-APPLIED: none. Five NEW tables, one NEW
--      sequence, one NEW trigger function. No existing table, column, index or constraint is
--      touched, so every query that runs today returns exactly what it returned before.
--   5. RETENTION: nothing is removed, and nothing here can be removed through the application —
--      there is no DELETE path in the store. Accounts are disabled (soft, rule 7), grants and
--      sessions are revoked, recovery codes are used or voided, and the trail is append-only.
--
-- PER-COMMUNE AND RESUMABLE (rule 7, invariant 5): no commune column, no backfill, no existing row
-- rewritten — nothing to iterate and nothing to resume.
--
-- PERSONAL DATA (rule 3): an operator's email and display name live in operator_account and
-- NOWHERE ELSE in this file — not in an index name, a constraint name, a comment example, or the
-- trail. There is no seed data: the first operator is created by the server-side CLI (TASK-03),
-- audited as "system" with its ticket number.
--
-- SECRETS ARE NEVER STORED IN THE CLEAR: the password is a hash; the TOTP secret is SEALED
-- (encrypted by service-identity/internal/operatorauth before it reaches this schema); the session
-- id and every recovery code are stored as SHA-256 hex, so a leaked backup hands over no credential
-- that can be replayed.

-- ---------------------------------------------------------------------------
-- operator_code_seq — the number behind every operator business code (`VH-00001`).
--
-- A SEQUENCE, BECAUSE A SEQUENCE NEVER HANDS A VALUE BACK — not even when the transaction that drew
-- it rolls back. That is exactly rule 7, invariant 3: a code issued once is never reissued, a gap is
-- harmless, a reuse is two people in one line of the trail. MAX(code)+1 would reissue the code of
-- the newest row the moment that row stopped being counted. The Go side formats the number
-- (domain.FormatOperatorCode) — NOT lpad() here, because lpad TRUNCATES a value longer than its
-- width: at the 100000th operator, lpad(…, 5, '0') would mint the code of operator 10000.
-- NO CYCLE is PostgreSQL's default and is written anyway: a cycling sequence is a reissuing one.
-- ---------------------------------------------------------------------------
CREATE SEQUENCE IF NOT EXISTS operator_code_seq AS BIGINT START WITH 1 MINVALUE 1 NO CYCLE;

-- ---------------------------------------------------------------------------
-- operator_account — one Vihat operator.
--
-- code — the audit "who" (rule 6, invariant 8). `^VH-[0-9]{5,}$`: five digits is a minimum, not a
-- width, so the 100000th operator is `VH-100000` rather than a refused insert or a truncated code.
--
-- email — the sign-in name. Unique on lower(email) (index below), because two accounts differing
-- only by letter case are one mailbox and one person, and the sign-in lookup folds case.
--
-- must_change_password — true for an account created by the CLI with a temporary password. Cleared
-- when the operator sets their own password (together with TOTP activation, store.ActivateTOTP).
--
-- TOTP, THREE STATES held by three columns:
--   not enrolled   totp_secret_sealed NULL, pending_totp_secret_sealed NULL
--   enrolling      pending_totp_secret_sealed set — a secret shown once as a QR, not yet proven
--   enrolled       totp_secret_sealed + totp_enrolled_at set
-- totp_last_step — the last TOTP time step accepted. A code is accepted only for a step STRICTLY
-- GREATER than this (store.RecordTOTPStep, one atomic UPDATE), so a code read over a shoulder
-- cannot be replayed within its 30-second window.
--
-- failed_attempts / locked_until — the lockout (owner: 5 consecutive failures, password or TOTP →
-- 15 minutes), kept in PostgreSQL so every replica sees the same count. The lock is DERIVED from
-- locked_until vs now; there is no "is_locked" flag to forget to clear.
--
-- disabled_at / disabled_by / disabled_reason — the ADMINISTRATIVE lock (CLI). Soft, never a delete
-- (rule 7): the row carries the code the trail quotes, and removing it would orphan years of trail.
-- The three columns move together (CHECK below). Re-enabling clears them; the trail keeps the
-- history.
-- ---------------------------------------------------------------------------
-- @entity: OperatorAccount
-- @scope:  platform
CREATE TABLE IF NOT EXISTS operator_account (
    id                         TEXT        PRIMARY KEY,
    code                       TEXT        NOT NULL,
    email                      TEXT        NOT NULL,
    display_name               TEXT        NOT NULL,
    password_hash              TEXT        NOT NULL,
    must_change_password       BOOLEAN     NOT NULL DEFAULT true,

    totp_secret_sealed         BYTEA,
    totp_enrolled_at           TIMESTAMPTZ,
    totp_last_step             BIGINT,
    pending_totp_secret_sealed BYTEA,

    failed_attempts            INT         NOT NULL DEFAULT 0,
    locked_until               TIMESTAMPTZ,

    disabled_at                TIMESTAMPTZ,
    disabled_by                TEXT,
    disabled_reason            TEXT,

    created_at                 TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- A business code or 'system' — never an internal id (rule 6, invariant 8).
    created_by                 TEXT        NOT NULL,
    updated_at                 TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT operator_account_id_is_ulid CHECK (length(id) = 26),
    CONSTRAINT operator_account_code_unique UNIQUE (code),
    CONSTRAINT operator_account_code_shape CHECK (code ~ '^VH-[0-9]{5,}$'),
    CONSTRAINT operator_account_email_not_blank CHECK (btrim(email) <> ''),
    CONSTRAINT operator_account_display_name_not_blank CHECK (btrim(display_name) <> ''),
    CONSTRAINT operator_account_password_hash_not_blank CHECK (btrim(password_hash) <> ''),
    CONSTRAINT operator_account_created_by_shape
        CHECK (created_by = 'system' OR created_by ~ '^VH-[0-9]{5,}$'),
    CONSTRAINT operator_account_totp_enrolled_complete
        CHECK ((totp_secret_sealed IS NULL) = (totp_enrolled_at IS NULL)),
    -- A remembered step without a secret is a replay guard for a factor that does not exist.
    CONSTRAINT operator_account_totp_step_needs_secret
        CHECK (totp_last_step IS NULL OR totp_secret_sealed IS NOT NULL),
    CONSTRAINT operator_account_failed_attempts_range CHECK (failed_attempts >= 0),
    CONSTRAINT operator_account_disabled_complete
        CHECK ((disabled_at IS NULL AND disabled_by IS NULL AND disabled_reason IS NULL)
            OR (disabled_at IS NOT NULL AND btrim(disabled_by) <> '' AND btrim(disabled_reason) <> ''))
);

-- @scope: platform — operator_account has no commune, so the sign-in name is unique platform-wide.
-- NOT partial on disabled_at, deliberately: a disabled account keeps its email occupied, so a new
-- account can never silently take over the sign-in name — and the trail — of a disabled one.
CREATE UNIQUE INDEX IF NOT EXISTS operator_account_email_unique
    ON operator_account (lower(email));

COMMENT ON TABLE operator_account IS
    'Vihat operator account (ADR 0048). No tenant_id by decision: the operator realm belongs to no '
    'commune. Never deleted: disabled_* is the administrative lock.';
COMMENT ON COLUMN operator_account.email IS
    'Personal data (Decree 13/2023): never logged, never in the audit trail. The code identifies.';

-- ---------------------------------------------------------------------------
-- operator_session — the operator session registry, checked on EVERY request (rule 5, invariant 4).
--
-- id IS THE SHA-256 HEX OF THE sid, NEVER THE sid. The sid is the bearer credential inside the
-- operator token; storing it raw would let whoever reads a backup sign in as any operator with a
-- live session. The CHECK refuses a raw value reaching the column through a path that forgot to
-- hash.
--
-- expires_at is ABSOLUTE: created_at + 8 hours (domain.SessionLifetime), no refresh. last_seen_at
-- is recorded for the operator's own "where am I signed in" view and never extends expires_at.
--
-- Revoked, never deleted: a revoked row is the record that the session existed.
-- ---------------------------------------------------------------------------
-- @entity: OperatorSession
-- @scope:  platform
CREATE TABLE IF NOT EXISTS operator_session (
    id                  TEXT        PRIMARY KEY,
    operator_account_id TEXT        NOT NULL REFERENCES operator_account (id),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at          TIMESTAMPTZ NOT NULL,
    revoked_at          TIMESTAMPTZ,
    revoked_reason      TEXT,
    created_ip          TEXT        NOT NULL DEFAULT '',
    user_agent          TEXT        NOT NULL DEFAULT '',
    last_seen_at        TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT operator_session_id_is_sha256 CHECK (id ~ '^[0-9a-f]{64}$'),
    CONSTRAINT operator_session_expires_after_created CHECK (expires_at > created_at),
    CONSTRAINT operator_session_revoked_complete
        CHECK ((revoked_at IS NULL AND revoked_reason IS NULL)
            OR (revoked_at IS NOT NULL AND btrim(revoked_reason) <> ''))
);

-- Revoking every live session of one account (password change, disable, grant change).
CREATE INDEX IF NOT EXISTS operator_session_live_by_account
    ON operator_session (operator_account_id)
    WHERE revoked_at IS NULL;

-- ---------------------------------------------------------------------------
-- operator_permission_grant — one `ops.*` key granted straight to one account (owner's decision
-- #3: few people, no roles).
--
-- permission_key IS A CLOSED LIST, and the CHECK is the database's copy of
-- domain.OperatorPermissions(); internal/store/operatorstore compares the two. These keys are NOT
-- rows of `quyen`: that table is a commune's register, and a cross-commune right has no seat in it.
--
-- NEVER DELETED. Revoking stamps revoked_at / revoked_by / revoke_reason; granting again is a NEW
-- row. The table is therefore the full history of who held which right, and when.
--
-- THE PARTIAL UNIQUE INDEX IS DELIBERATE and is not the reissued-code trap tools/check_khoa_duy_nhat
-- guards against: a grant is a STATE, not an issued code. At most one LIVE grant per (account, key);
-- any number of revoked ones.
-- ---------------------------------------------------------------------------
-- @entity: OperatorPermissionGrant
-- @scope:  platform
CREATE TABLE IF NOT EXISTS operator_permission_grant (
    id                  TEXT        PRIMARY KEY,
    operator_account_id TEXT        NOT NULL REFERENCES operator_account (id),
    permission_key      TEXT        NOT NULL,
    granted_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    granted_by          TEXT        NOT NULL,
    grant_reason        TEXT        NOT NULL,
    revoked_at          TIMESTAMPTZ,
    revoked_by          TEXT,
    revoke_reason       TEXT,

    CONSTRAINT operator_permission_grant_id_is_ulid CHECK (length(id) = 26),
    CONSTRAINT operator_permission_grant_key_known CHECK (permission_key IN (
        'ops.tenant.manage', 'ops.domain.manage', 'ops.profile.manage',
        'ops.mini_app.manage', 'ops.upload_policy.manage', 'ops.qr.issue')),
    CONSTRAINT operator_permission_grant_granted_by_shape
        CHECK (granted_by = 'system' OR granted_by ~ '^VH-[0-9]{5,}$'),
    CONSTRAINT operator_permission_grant_reason_not_blank CHECK (btrim(grant_reason) <> ''),
    CONSTRAINT operator_permission_grant_revoked_complete
        CHECK ((revoked_at IS NULL AND revoked_by IS NULL AND revoke_reason IS NULL)
            OR (revoked_at IS NOT NULL
                AND (revoked_by = 'system' OR revoked_by ~ '^VH-[0-9]{5,}$')
                AND btrim(revoke_reason) <> ''))
);

-- @scope: platform — at most one LIVE grant per (account, key); see the block above.
CREATE UNIQUE INDEX IF NOT EXISTS operator_permission_grant_live_unique
    ON operator_permission_grant (operator_account_id, permission_key)
    WHERE revoked_at IS NULL;

-- ---------------------------------------------------------------------------
-- operator_recovery_code — single-use codes for when the authenticator app is lost.
--
-- code_hash is SHA-256 hex, never the code: a used-once secret stored in the clear is a secret any
-- reader of a backup can use once. Unique platform-wide — 10 random codes per batch make a
-- collision practically impossible, and the key turns the impossible case into a refused insert
-- rather than two accounts sharing one code.
--
-- batch_id groups the ten codes of one generation. Regenerating VOIDS every unused code of the
-- account (voided_at) and inserts a new batch, in one transaction. Nothing is deleted: a used or
-- voided row is the record that the code existed. A code is used XOR voided, never both.
-- ---------------------------------------------------------------------------
-- @entity: OperatorRecoveryCode
-- @scope:  platform
CREATE TABLE IF NOT EXISTS operator_recovery_code (
    id                  TEXT        PRIMARY KEY,
    operator_account_id TEXT        NOT NULL REFERENCES operator_account (id),
    code_hash           TEXT        NOT NULL,
    batch_id            TEXT        NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    used_at             TIMESTAMPTZ,
    voided_at           TIMESTAMPTZ,

    CONSTRAINT operator_recovery_code_id_is_ulid CHECK (length(id) = 26),
    CONSTRAINT operator_recovery_code_batch_is_ulid CHECK (length(batch_id) = 26),
    CONSTRAINT operator_recovery_code_hash_unique UNIQUE (code_hash),
    CONSTRAINT operator_recovery_code_hash_is_sha256 CHECK (code_hash ~ '^[0-9a-f]{64}$'),
    CONSTRAINT operator_recovery_code_used_xor_voided CHECK (used_at IS NULL OR voided_at IS NULL)
);

CREATE INDEX IF NOT EXISTS operator_recovery_code_live_by_account
    ON operator_recovery_code (operator_account_id)
    WHERE used_at IS NULL AND voided_at IS NULL;

-- ---------------------------------------------------------------------------
-- operator_audit_log — who did what in the operator realm, when, from where.
--
-- A SEPARATE TABLE FROM audit_log AND FROM service-platform's platform_audit_log. audit_log is keyed
-- and partitioned by tenant_id NOT NULL, and an operator signing in belongs to no commune; a made-up
-- tenant_id is rule 1, forbidden #1. platform_audit_log belongs to ANOTHER service's database (rule
-- 2, invariant 2). An operator act that TARGETS one commune is still audited by the service that
-- owns that commune's data, in its audit_log under that commune (ADR 0048 §Thiết kế #6) — this
-- table holds the realm's own events: sign-in, sign-out, enrolment, grants, locks, resets.
--
-- WHAT RULE 6 ASKS, COLUMN BY COLUMN: who = actor (an operator code, or 'system' for the CLI with
-- its ticket number in reason — the CHECK refuses anything else, including an internal id); what =
-- action; on which record = subject (the operator code the act was about — never an email); when =
-- occurred_at; from which IP = actor_ip (NULL when there is no request, as for the CLI); in which
-- commune = none, by definition of this table. before/after hold significant fields only, never
-- personal data and never a secret (rule 6, forbidden #4).
--
-- THE ACTION VOCABULARY is closed in Go (domain.OperatorAuditAction); the CHECK here only pins its
-- shape, so a new event is a reviewed code change rather than a migration.
--
-- NOT PARTITIONED: a few rows per working day, no tenant_id to hash on.
--
-- APPEND-ONLY, enforced by triggers, the same shape as 0002 and service-platform 0008: row-level
-- BEFORE UPDATE OR DELETE, statement-level BEFORE TRUNCATE.
-- ---------------------------------------------------------------------------
-- @entity: OperatorAuditEntry
-- @scope:  platform
CREATE TABLE IF NOT EXISTS operator_audit_log (
    id          BIGSERIAL   PRIMARY KEY,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    actor       TEXT        NOT NULL,
    actor_ip    TEXT,
    action      TEXT        NOT NULL,
    subject     TEXT        NOT NULL,
    before      JSONB,
    after       JSONB,
    reason      TEXT,

    CONSTRAINT operator_audit_log_actor_not_blank CHECK (btrim(actor) <> ''),
    CONSTRAINT operator_audit_log_actor_shape
        CHECK (actor = 'system' OR actor ~ '^VH-[0-9]{5,}$'),
    -- The CLI acts as 'system'; without the ticket number, 'system' names nobody.
    CONSTRAINT operator_audit_log_system_has_reason
        CHECK (actor <> 'system' OR btrim(coalesce(reason, '')) <> ''),
    CONSTRAINT operator_audit_log_action_shape CHECK (action ~ '^operator\.[a-z_]+$'),
    CONSTRAINT operator_audit_log_subject_shape CHECK (subject ~ '^VH-[0-9]{5,}$')
);

CREATE INDEX IF NOT EXISTS operator_audit_log_lookup
    ON operator_audit_log (subject, occurred_at DESC);

COMMENT ON TABLE operator_audit_log IS
    'Operator realm trail (ADR 0048). Append-only: UPDATE, DELETE and TRUNCATE refused. No tenant_id '
    'by the owner decision of 2026-09-28. Never holds an email, a name or a secret.';

-- The message names the operation and the table, never the row (rule 3, forbidden #3).
CREATE OR REPLACE FUNCTION operator_audit_log_append_only() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'operator_audit_log is append-only: % on % refused', TG_OP, TG_TABLE_NAME
        USING HINT = 'Audit entries are archival records (rule 6, invariant 4): never '
                     'modified, never removed, never truncated. To correct a wrong entry, '
                     'append a new one.';
END $$;

DROP TRIGGER IF EXISTS operator_audit_log_no_update_delete ON operator_audit_log;
CREATE TRIGGER operator_audit_log_no_update_delete
    BEFORE UPDATE OR DELETE ON operator_audit_log
    FOR EACH ROW EXECUTE FUNCTION operator_audit_log_append_only();

DROP TRIGGER IF EXISTS operator_audit_log_no_truncate ON operator_audit_log;
CREATE TRIGGER operator_audit_log_no_truncate
    BEFORE TRUNCATE ON operator_audit_log
    FOR EACH STATEMENT EXECUTE FUNCTION operator_audit_log_append_only();

-- ---------------------------------------------------------------------------
-- REVERSAL (rule 7, invariant 4).
--
-- core/migrate has no automatic rollback, on purpose. The reverse is written here as prose, and
-- NONE OF IT IS A RUNNABLE LINE — a runnable line is a line that gets run.
--
-- WHILE NO OPERATOR ACCOUNT EXISTS (every environment until the TASK-03 CLI first runs), in one
-- transaction and in dependency order: drop the three tables that reference operator_account
-- (operator_session, operator_permission_grant, operator_recovery_code), then operator_account,
-- then the sequence operator_code_seq; drop operator_audit_log, then the function
-- operator_audit_log_append_only; then remove this file's progress row from schema_migration,
-- keyed on ten = '0012_operator_accounts.sql'.
--
-- ONCE AN OPERATOR HAS BEEN CREATED it is no longer a reversal: operator_audit_log then holds the
-- only record of who held which platform-wide right and when, and the sequence holds the promise
-- that no code is reissued. Discarding either is rule 6 forbidden #3 plus rule 7 stop condition #1
-- — the user decides, with a verified backup. "Reverting" by ceasing to read and write the tables
-- costs nothing and loses nothing.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- BACKSTOP, carried forward from 0002–0011: a partitioned table with no partitions rejects every
-- INSERT. This file declares no partitioned table and is expected to find nothing; it runs anyway,
-- because the check only describes the state after the newest migration that carries it.
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
