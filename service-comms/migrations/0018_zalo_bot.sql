-- 0018 — ZALO BOT, the staff reminder channel (ADR 0074, owner 05/10/2026). Seven tables:
--
--   zalo_channel_setting          tenant     one commune's switch, kinds, quiet hours, overdue cadence
--   zalo_pairing_code             tenant     the one-time code a member of staff types into the bot
--   zalo_link                     tenant *   member of staff <-> one Zalo chat of the shared bot
--   zalo_delivery                 tenant     outbox + history: one bell notice -> one Zalo message
--   platform_data_encryption_key  platform   the PLATFORM-scope data key that seals the bot's secrets
--   zalo_bot_shared               platform   the ONE shared bot: sealed token, sealed webhook secrets
--   platform_operator_audit_log   platform   append-only trail of operator acts on the two above
--
--   * tenant-owned, but NOT partitioned — see the block above `zalo_link`.
--
-- comms owns all of it (ADR 0074 #2): it already owns `staff_notification` (0010), which is what a
-- Zalo message repeats. platform-admin holds no data; it reaches the shared-bot rows through
-- service-platform -> comms gRPC under `ops.zalo_bot.manage` (identity migration 0025).
--
-- NAMED IN ENGLISH (rule 12, ADR 0051). ../vigov-require docs/spec/03-mo-hinh-du-lieu.md
-- `zalo_bot_link_codes` / `zalo_bot_links` / `zalo_bot_platform_settings` / `zalo_bot_settings` are the
-- design-time names. `zalo_pairing_code`, not `zalo_link_code`: the owner-decided URL is
-- `zalo-links/current/pairing-codes` (ADR 0074 §Tài nguyên URL), and one concept gets one English word
-- (rule 12, forbidden #2). ENUM VALUES stay Vietnamese without diacritics (ADR 0011, ADR 0051 table):
-- the four kinds are `staff_notification.kind`'s own values (0010) — 'sap-den-han', 'qua-han',
-- 'leo-thang', 'ban-tin-tuan' = the proto's DUE_SOON / OVERDUE / ESCALATION / WEEKLY_DIGEST — and the
-- delivery states reuse this service's outbox words from 0004 ('cho-gui', 'da-gui', 'that-bai').
--
-- WHY A NEW FILE: core/migrate compares the checksum of every applied file at startup.
--
-- ---------------------------------------------------------------------------
-- WHAT IS DELIBERATELY NOT A TABLE
--
--   * PER-CHAT PAIRING ATTEMPTS ("mỗi chat tối đa 5 lần thử/giờ", ADR 0074). A fixed-window counter is
--     exactly what core/ratelimit already is, in Redis, which ADR 0010 allows for rate limiting and
--     forbids as durable storage. Losing the count loses nothing but the count. A Postgres table would
--     be a second rate-limit mechanism with its own cleanup job. OWED BY GO: a Policy and a no-commune
--     key built from a DIGEST of chat_id (chat_id is personal data — rule 3, forbidden #4: never raw in
--     a cache key), on the same model as ratelimit.CitizenKey.
--   * A NEW COMMUNE AUDIT TABLE. Pairing, unlinking (including the links a bot-account change ends) and
--     settings saves are entries in the existing comms `audit_log` (rule 6), in the same transaction.
--     Only the PLATFORM-scope operator acts, which belong to no commune, get platform_operator_audit_log.
--   * MESSAGE TEMPLATES. ADR 0074 #8: fixed in code for wave 1.
--   * The spec's `display_name` on a link: the Zalo display name is personal data that no decided
--     screen shows (ADR 0074: chat_id is never shown; nothing decides the name is). Not stored.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. HOW MANY ROWS PER COMMUNE. This file writes NONE — no seed, no default commune row, no bot row.
--      Steady state: zalo_channel_setting at most 1; zalo_link about one live row per member of staff
--      who paired (tens), plus one ended row per unlink or re-pair; zalo_pairing_code a few per member
--      of staff per year; zalo_delivery at most one row per bell notice (0010 estimates low tens of
--      thousands a year for forty staff) plus test messages. Platform tables: exactly one row each
--      by construction, and platform_operator_audit_log a handful of rows a month.
--   2. IF IT STOPS HALF-WAY. It cannot land half-applied: one file, one transaction with its progress
--      row. Every statement is IF NOT EXISTS / CREATE OR REPLACE / DROP TRIGGER IF EXISTS, so a retry
--      from the top costs nothing. No backfill, so nothing to resume per commune.
--   3. HOW IT IS REVERSED. See REVERSAL at the bottom.
--   4. WHICH READ PATHS CHANGE MEANING WHILE IT IS HALF-APPLIED. None. Seven new tables, their functions,
--      triggers and indexes. The only existing objects touched are REFERENCED, not altered:
--      zalo_delivery has a foreign key to staff_notification (tenant_id, id), whose rows are never
--      hard-deleted anyway (0010's guard). No existing column, constraint or index changes.
--   5. RETENTION. Nothing here is ever hard-deleted — every table's guard refuses DELETE.
--        zalo_delivery   evidence that a member of staff WAS (or was not) reminded on Zalo, the same
--                        weight as the bell notice it repeats. Soft delete only; outcome forward-only.
--        zalo_link       who received reminders on which chat, from when to when. An ended link is
--                        kept, frozen (see the block above the table).
--        zalo_pairing_code  a hash of a one-time code — no personal data; kept as the pairing history.
--        settings, bot   configuration rows, overwritten in place; history is the audit trail.
--        platform_data_encryption_key  deleting it destroys the sealed token for good.
--        platform_operator_audit_log   append-only: UPDATE, DELETE and TRUNCATE refused (rule 6).
--      chat_id is PERSONAL DATA (ADR 0074): an erasure request under Decree 13 on an ENDED link means
--      anonymising chat_id, which the zalo_link guard refuses today — that is rule 3 stop condition #3
--      and needs the user, then a migration.
-- ---------------------------------------------------------------------------

-- PostgreSQL 13 is already the floor (0003/0004 check it): BEFORE ... FOR EACH ROW on a partitioned
-- table. PostgreSQL 12+ for a foreign key that REFERENCES a partitioned table (zalo_delivery ->
-- staff_notification). The real cluster is 16.

-- ===========================================================================
-- zalo_channel_setting
-- ===========================================================================

-- ---------------------------------------------------------------------------
-- zalo_channel_setting_guard — refuses DELETE, a change of commune, a rewritten creation time.
--
-- NO SOFT-DELETE COLUMNS, as in 0008 mail_settings: one configuration row per commune, overwritten in
-- place and switched off with is_enabled = false. Every save is an audit entry with before/after.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION zalo_channel_setting_guard() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION '%: delete refused', TG_TABLE_NAME
            USING HINT = 'Turn the Zalo channel off with is_enabled = false instead.';
    END IF;
    IF NEW.tenant_id  IS DISTINCT FROM OLD.tenant_id
    OR NEW.created_at IS DISTINCT FROM OLD.created_at THEN
        RAISE EXCEPTION '%: tenant_id and created_at are immutable', TG_TABLE_NAME
            USING HINT = 'One commune''s settings row stays that commune''s.';
    END IF;
    RETURN NEW;
END $$;

-- ---------------------------------------------------------------------------
-- @entity: ZaloChannelSetting
-- @scope:  tenant
--
-- zalo_channel_setting — one commune's Zalo reminder channel (`zalo-channel-settings`, admin.lookup,
-- ADR 0074 #6). NO ROW = THE CHANNEL IS OFF for that commune: nothing is sent, and the bell still
-- carries every notice (ADR 0074 #1). Fail-closed, and no vendor default decides that a commune's staff
-- are messaged on Zalo.
--
-- WHERE THE DEFAULTS LIVE, AND WHY THERE ARE ONLY TWO:
--   quiet_start / quiet_end   DEFAULT 21:00 / 06:00. ADR 0074 adopted the spec's "giờ yên tĩnh mặc
--                             định 21h–6h giờ Việt Nam" as a decided default; it is per-commune and
--                             overridable. Held here, in ONE place, not as a Go constant.
--   overdue cadence           NO default. ADR 0074 says "nhịp nhắc việc trễ do xã đặt" — the COMMUNE
--                             sets it. The spec's 0 / 1 were not adopted, so a vendor value here would
--                             decide it for them. NULL until the commune chooses; the CHECK below
--                             refuses turning OVERDUE reminders on without both numbers.
--   kinds                     DEFAULT empty, and an ENABLED channel must name at least one kind.
--                             Which kinds a commune wants is the commune's choice.
-- These are channel settings, not a deadline: rule 10 forbidden #3 (SLA hardcoded) is not engaged —
-- the "due soon" threshold stays the bell's (ADR 0074).
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS zalo_channel_setting (
    tenant_id                  TEXT        NOT NULL,
    -- `is_enabled`: this service's word for "a per-commune configuration row switched off without
    -- being deleted" (0008, 0013, 0017).
    is_enabled                 BOOLEAN     NOT NULL DEFAULT false,
    kinds                      TEXT[]      NOT NULL DEFAULT '{}',
    -- Vietnam local time (Asia/Ho_Chi_Minh), wall-clock, no zone: "21h–6h giờ Việt Nam". A window
    -- with quiet_start > quiet_end wraps midnight. Messages due inside it wait until quiet_end.
    quiet_start                TIME        NOT NULL DEFAULT '21:00',
    quiet_end                  TIME        NOT NULL DEFAULT '06:00',
    -- OVERDUE cadence, in days (the spec's unit): first reminder N days after the deadline passed
    -- (0 = the day it passes), then every M days.
    overdue_start_after_days   SMALLINT,
    overdue_repeat_every_days  SMALLINT,
    created_at                 TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                 TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- The BUSINESS CODE of the last person to save (rule 6, invariant 8) — never an internal id.
    updated_by                 TEXT        NOT NULL,
    PRIMARY KEY (tenant_id),
    CONSTRAINT zalo_channel_setting_kinds_known
        CHECK (kinds <@ ARRAY['sap-den-han', 'qua-han', 'leo-thang', 'ban-tin-tuan']::text[]),
    CONSTRAINT zalo_channel_setting_enabled_has_kind
        CHECK (NOT is_enabled OR cardinality(kinds) > 0),
    -- A zero-length window is ambiguous between "never quiet" and "always quiet". Neither is decided.
    CONSTRAINT zalo_channel_setting_quiet_window
        CHECK (quiet_start <> quiet_end),
    CONSTRAINT zalo_channel_setting_overdue_pair
        CHECK ((overdue_start_after_days IS NULL) = (overdue_repeat_every_days IS NULL)),
    CONSTRAINT zalo_channel_setting_overdue_needs_cadence
        CHECK (NOT ('qua-han' = ANY (kinds)) OR overdue_start_after_days IS NOT NULL),
    -- Sanity bounds, not policy: a year either way is beyond any reminder anybody would configure.
    CONSTRAINT zalo_channel_setting_overdue_start_range
        CHECK (overdue_start_after_days IS NULL OR overdue_start_after_days BETWEEN 0 AND 365),
    CONSTRAINT zalo_channel_setting_overdue_repeat_range
        CHECK (overdue_repeat_every_days IS NULL OR overdue_repeat_every_days BETWEEN 1 AND 365),
    CONSTRAINT zalo_channel_setting_signed
        CHECK (btrim(updated_by) <> '' AND char_length(updated_by) <= 64)
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS zalo_channel_setting_p%s PARTITION OF zalo_channel_setting '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

DROP TRIGGER IF EXISTS zalo_channel_setting_guard ON zalo_channel_setting;
CREATE TRIGGER zalo_channel_setting_guard
    BEFORE UPDATE OR DELETE ON zalo_channel_setting
    FOR EACH ROW EXECUTE FUNCTION zalo_channel_setting_guard();

-- ===========================================================================
-- zalo_pairing_code
-- ===========================================================================

-- ---------------------------------------------------------------------------
-- zalo_pairing_code_guard — a one-time credential, so the guard is mostly "set once".
--
--   DELETE              refused. A deleted code row frees its hash for reuse in this commune.
--   identity columns    which commune, which member of staff, which hash, until when: fixed at issue.
--                       Moving a code to another member of staff pairs THEIR account to whoever
--                       holds the code.
--   used_at,            NULL -> a value, once. A closed code (used or cancelled) is frozen whole:
--   cancelled_at        re-opening one would make a spent credential valid again.
--   failed_attempts     never decreases. Lowering it hands out fresh guesses.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION zalo_pairing_code_guard() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION '%: delete refused', TG_TABLE_NAME
            USING HINT = 'Close a code with cancelled_at instead. A deleted row frees its hash.';
    END IF;
    IF OLD.used_at IS NOT NULL OR OLD.cancelled_at IS NOT NULL THEN
        RAISE EXCEPTION '%: a used or cancelled code is not edited', TG_TABLE_NAME
            USING HINT = 'A spent one-time code must never become valid again. Issue a new code.';
    END IF;
    IF NEW.tenant_id  IS DISTINCT FROM OLD.tenant_id
    OR NEW.id         IS DISTINCT FROM OLD.id
    OR NEW.staff_code IS DISTINCT FROM OLD.staff_code
    OR NEW.code_hash  IS DISTINCT FROM OLD.code_hash
    OR NEW.expires_at IS DISTINCT FROM OLD.expires_at
    OR NEW.created_at IS DISTINCT FROM OLD.created_at THEN
        RAISE EXCEPTION '%: an issued code is immutable', TG_TABLE_NAME
            USING HINT = 'Commune, member of staff, hash and expiry are fixed at issue.';
    END IF;
    IF NEW.failed_attempts < OLD.failed_attempts THEN
        RAISE EXCEPTION '%: failed_attempts cannot decrease', TG_TABLE_NAME
            USING HINT = 'Lowering the count hands out fresh guesses against the same code.';
    END IF;
    RETURN NEW;
END $$;

-- ---------------------------------------------------------------------------
-- @entity: ZaloPairingCode
-- @scope:  tenant
--
-- zalo_pairing_code — the code a member of staff gets from `POST zalo-links/current/pairing-codes`
-- and types into the shared bot's chat. ADR 0074: 8 characters with no look-alikes, lives 10 minutes,
-- one use, ONLY THE HASH IS STORED; 3 wrong tries cancel it. Those numbers live in Go — the schema holds
-- the facts (expiry, uses, attempts), not the policy.
--
-- THE WEBHOOK HAS NO COMMUNE HOST (ADR 0074 #5): the commune is found FROM this code. The lookup by
-- hash is therefore cross-commune by nature — `zalo_pairing_code_open_by_hash` below, the one index
-- here that does not lead with tenant_id. OWED BY GO: that read carries `// @cross-tenant:`, and MORE
-- THAN ONE open match is answered as NO match (fail closed). Two communes holding the same live code at
-- once is possible in principle — the hash is unique per commune, not platform-wide, because a
-- partitioned table's unique key must contain tenant_id — and guessing between them would pair a chat
-- to the wrong commune's account.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS zalo_pairing_code (
    tenant_id        TEXT        NOT NULL,
    id               TEXT        NOT NULL,            -- ULID, internal
    -- `CB-…`, nguoi_dung.ma of identity — the session's own code (ADR 0074 #6). NO FOREIGN KEY: the
    -- staff directory is identity's (rule 2).
    staff_code       TEXT        NOT NULL,
    -- SHA-256 / HMAC-SHA-256 of the code: 32 bytes. Never the code itself.
    code_hash        BYTEA       NOT NULL,
    expires_at       TIMESTAMPTZ NOT NULL,
    -- The code produced a link. Mutually exclusive with cancelled_at.
    used_at          TIMESTAMPTZ,
    -- Closed without use: superseded by a newer code, or too many wrong tries.
    cancelled_at     TIMESTAMPTZ,
    failed_attempts  SMALLINT    NOT NULL DEFAULT 0,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, id),
    -- NOT partial: a hash once issued in a commune is never issued there again, used or not.
    UNIQUE (tenant_id, code_hash),
    CONSTRAINT zalo_pairing_code_hash_length CHECK (octet_length(code_hash) = 32),
    CONSTRAINT zalo_pairing_code_staff_code_length CHECK (char_length(staff_code) BETWEEN 1 AND 64),
    CONSTRAINT zalo_pairing_code_expiry_after_issue CHECK (expires_at > created_at),
    CONSTRAINT zalo_pairing_code_closed_once CHECK (used_at IS NULL OR cancelled_at IS NULL),
    CONSTRAINT zalo_pairing_code_attempts_non_negative CHECK (failed_attempts >= 0)
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS zalo_pairing_code_p%s PARTITION OF zalo_pairing_code '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

-- AT MOST ONE OPEN CODE PER MEMBER OF STAFF. "Open" = neither used nor cancelled; expiry cannot sit in
-- an index predicate (now() is not immutable), so issuing a new code CANCELS the previous open one in
-- the same transaction — expired or not. Partial on used/cancelled, NOT on deleted_at: there is no
-- soft delete here, and the predicate frees nothing that was ever issued (the hash key above is total).
CREATE UNIQUE INDEX IF NOT EXISTS zalo_pairing_code_one_open_per_staff
    ON zalo_pairing_code (tenant_id, staff_code)
    WHERE used_at IS NULL AND cancelled_at IS NULL;

-- THE WEBHOOK'S LOOKUP: hash -> (commune, member of staff). Cross-commune by nature (see above); open
-- codes only, so the index stays the size of the codes alive right now.
CREATE INDEX IF NOT EXISTS zalo_pairing_code_open_by_hash
    ON zalo_pairing_code (code_hash)
    WHERE used_at IS NULL AND cancelled_at IS NULL;

DROP TRIGGER IF EXISTS zalo_pairing_code_guard ON zalo_pairing_code;
CREATE TRIGGER zalo_pairing_code_guard
    BEFORE UPDATE OR DELETE ON zalo_pairing_code
    FOR EACH ROW EXECUTE FUNCTION zalo_pairing_code_guard();

-- ===========================================================================
-- zalo_link
-- ===========================================================================

-- ---------------------------------------------------------------------------
-- zalo_link_guard
--
--   DELETE             refused. The row is the record of where a member of staff's reminders went.
--   an ended link      (unlinked_at set) is history: frozen whole, rule 7 forbidden #5.
--   identity columns   commune, member of staff, bot, chat, when linked: fixed. Re-pointing a link at
--                      another chat sends one person's reminders to somebody else's phone.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION zalo_link_guard() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION '%: delete refused', TG_TABLE_NAME
            USING HINT = 'End a link with unlinked_at, unlinked_by, unlink_reason instead.';
    END IF;
    IF OLD.unlinked_at IS NOT NULL THEN
        RAISE EXCEPTION '%: an ended link is not edited', TG_TABLE_NAME
            USING HINT = 'An ended link is a historical record (rule 7, forbidden #5). Pair again.';
    END IF;
    IF NEW.tenant_id  IS DISTINCT FROM OLD.tenant_id
    OR NEW.id         IS DISTINCT FROM OLD.id
    OR NEW.staff_code IS DISTINCT FROM OLD.staff_code
    OR NEW.bot_ref    IS DISTINCT FROM OLD.bot_ref
    OR NEW.chat_id    IS DISTINCT FROM OLD.chat_id
    OR NEW.linked_at  IS DISTINCT FROM OLD.linked_at
    OR NEW.created_at IS DISTINCT FROM OLD.created_at THEN
        RAISE EXCEPTION '%: a link is immutable', TG_TABLE_NAME
            USING HINT = 'Who, which bot, which chat and since when are fixed. End it and pair again.';
    END IF;
    RETURN NEW;
END $$;

-- ---------------------------------------------------------------------------
-- @entity: ZaloLink
-- @scope:  tenant
--
-- zalo_link — one member of staff of one commune <-> one chat of one bot. Rows are owned by a commune
-- (tenant_id, read and written through the scoped store), but ONE invariant is platform-wide: a chat
-- belongs to ONE account across the whole platform (ADR 0074, "một chat một tài khoản"), because the
-- webhook resolves the commune FROM the chat (ADR 0074 #5). Two live links for one chat in two communes
-- would leave that resolution to a guess.
--
-- NOT PARTITIONED — A DELIBERATE DEPARTURE FROM ADR 0010's "PARTITION BY HASH (tenant_id) from the
-- first migration", stated so it can be overruled:
--   * PostgreSQL requires every unique index of a partitioned table to contain the partition key.
--     `zalo_link_one_live_per_chat` cannot contain tenant_id without ceasing to be platform-wide.
--   * The alternatives are worse: a separate claim table is a SECOND copy of (chat -> commune) that
--     must be kept in step by hand (rule 9 #2); a trigger that scans every partition under an advisory
--     lock is a uniqueness constraint the database does not know it has.
--   * ADR 0010's benefit is partition pruning. The hot read here — every webhook update — is BY CHAT,
--     with no commune known yet, so it could never prune. The table holds about one live row per paired
--     member of staff: thousands platform-wide, not millions.
--   The commune filter on every staff-side read is still enforced by the scoped store (rule 1, inv 5).
--
-- SOFT DELETE IS SPELLED `unlinked_*`, NOT `deleted_*`, ALSO DELIBERATELY. A link ENDS — the member of
-- staff unlinks, re-pairs ("ghép lại thì xoá mềm liên kết cũ", ADR 0074), or the chat is paired to
-- another account — and the ended row stays as history. Live-only uniqueness is the business rule
-- (re-pairing the same chat after unlinking must work), and tools/check_khoa_duy_nhat.py rightly refuses
-- `UNIQUE … WHERE deleted_at IS NULL`, because on a table with ISSUED CODES that predicate reissues a
-- code. A link issues no code. Precedent: identity 0012 `operator_permission_grant_live_unique … WHERE
-- revoked_at IS NULL`. Every read path excludes ended links exactly as it would exclude deleted rows.
--
-- chat_id IS PERSONAL DATA (ADR 0074). Stored raw because sending needs it; NEVER logged, never returned
-- to a client, never exported, never in a cache key (rule 3) — the per-chat rate limit keys a digest.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS zalo_link (
    tenant_id      TEXT        NOT NULL,
    id             TEXT        NOT NULL,              -- ULID, internal; zalo_delivery points here
    -- `CB-…`, identity's business code. NO FOREIGN KEY (rule 2).
    staff_code     TEXT        NOT NULL,
    -- WHICH BOT. 'shared' only, in wave 1 (ADR 0074 #1): room for a commune's own bot later, which
    -- widens this CHECK in a new migration. Not a foreign key to zalo_bot_shared for that reason.
    bot_ref        TEXT        NOT NULL DEFAULT 'shared',
    chat_id        TEXT        NOT NULL,
    linked_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    unlinked_at    TIMESTAMPTZ,
    -- Business code of whoever ended it (rule 6, invariant 8): the member of staff (`CB-…`); the
    -- OPERATOR (`VH-…`) when a change of bot account ended every link (owner, 05/10/2026); or
    -- core/audit.SystemActor ('system') when the chat was taken by a pairing in ANOTHER commune — that
    -- commune's staff code is never written into this commune's row.
    unlinked_by    TEXT,
    unlink_reason  TEXT,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, id),
    CONSTRAINT zalo_link_bot_ref_known CHECK (bot_ref IN ('shared')),
    CONSTRAINT zalo_link_staff_code_length CHECK (char_length(staff_code) BETWEEN 1 AND 64),
    CONSTRAINT zalo_link_chat_id_length CHECK (char_length(chat_id) BETWEEN 1 AND 128),
    -- All three, or none (the shape rule 7 invariant 1 asks of a soft delete).
    CONSTRAINT zalo_link_unlink_complete
        CHECK ((unlinked_at IS NULL) = (unlinked_by IS NULL)
               AND (unlinked_at IS NULL) = (unlink_reason IS NULL)),
    CONSTRAINT zalo_link_unlink_signed
        CHECK (unlinked_by IS NULL OR (btrim(unlinked_by) <> '' AND char_length(unlinked_by) <= 64)),
    CONSTRAINT zalo_link_unlink_reason_length
        CHECK (unlink_reason IS NULL OR (btrim(unlink_reason) <> '' AND char_length(unlink_reason) <= 200)),
    CONSTRAINT zalo_link_unlinked_after_linked
        CHECK (unlinked_at IS NULL OR unlinked_at >= linked_at)
);

-- ONE LIVE LINK PER MEMBER OF STAFF IN A COMMUNE ("một tài khoản một chat"). Composite with tenant_id
-- (rule 1, invariant 6): staff codes are unique per commune only.
CREATE UNIQUE INDEX IF NOT EXISTS zalo_link_one_live_per_staff
    ON zalo_link (tenant_id, staff_code)
    WHERE unlinked_at IS NULL;

-- @scope: cross-tenant — ONE LIVE LINK PER CHAT ACROSS THE PLATFORM ("một chat một tài khoản"). The
-- webhook resolves the commune from this row, so a second live row in another commune must be
-- impossible, not merely unlikely. Also the webhook's lookup index (bot, chat) -> (commune, staff).
-- OWED BY GO: that read carries `// @cross-tenant:`; pairing a chat that is live elsewhere ends the old
-- link (unlinked_by 'system') in the same transaction, before inserting the new one.
CREATE UNIQUE INDEX IF NOT EXISTS zalo_link_one_live_per_chat
    ON zalo_link (bot_ref, chat_id)
    WHERE unlinked_at IS NULL;

DROP TRIGGER IF EXISTS zalo_link_guard ON zalo_link;
CREATE TRIGGER zalo_link_guard
    BEFORE UPDATE OR DELETE ON zalo_link
    FOR EACH ROW EXECUTE FUNCTION zalo_link_guard();

-- ===========================================================================
-- zalo_delivery
-- ===========================================================================

-- ---------------------------------------------------------------------------
-- zalo_delivery_guard — the outbox is also the history, so the outcome only moves forward.
--
--   DELETE                refused (rule 7, invariant 1). A hard delete would also free the
--                         notification key and let a retry send the same reminder twice.
--   a soft-deleted row    is a historical record; not edited.
--   identity columns      which notice, to whom, which kind, when queued: fixed at creation.
--   a terminal row        ('da-gui', 'bo-qua', 'that-bai') keeps its outcome: a sent message cannot
--                         become unsent, and a recorded skip reason cannot be rewritten. Only the
--                         soft-delete columns and updated_at may still change.
--   attempts              never decreases.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION zalo_delivery_guard() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION '%: hard delete refused', TG_TABLE_NAME
            USING HINT = 'Soft delete only (deleted_at, deleted_by, delete_reason) — rule 7, '
                         'invariant 1. A hard delete would also let a retry send it again.';
    END IF;
    IF OLD.deleted_at IS NOT NULL THEN
        RAISE EXCEPTION '%: a deleted delivery is not edited', TG_TABLE_NAME
            USING HINT = 'A soft-deleted row is a historical record (rule 7, forbidden #5).';
    END IF;
    IF NEW.tenant_id       IS DISTINCT FROM OLD.tenant_id
    OR NEW.id              IS DISTINCT FROM OLD.id
    OR NEW.notification_id IS DISTINCT FROM OLD.notification_id
    OR NEW.staff_code      IS DISTINCT FROM OLD.staff_code
    OR NEW.kind            IS DISTINCT FROM OLD.kind
    OR NEW.created_at      IS DISTINCT FROM OLD.created_at THEN
        RAISE EXCEPTION '%: a queued delivery is immutable', TG_TABLE_NAME
            USING HINT = 'Which notice, to whom, of which kind and when queued are fixed.';
    END IF;
    IF OLD.status <> 'cho-gui'
       AND (NEW.status          IS DISTINCT FROM OLD.status
         OR NEW.skip_reason     IS DISTINCT FROM OLD.skip_reason
         OR NEW.link_id         IS DISTINCT FROM OLD.link_id
         OR NEW.attempts        IS DISTINCT FROM OLD.attempts
         OR NEW.next_attempt_at IS DISTINCT FROM OLD.next_attempt_at
         OR NEW.sent_at         IS DISTINCT FROM OLD.sent_at
         OR NEW.error_class     IS DISTINCT FROM OLD.error_class) THEN
        RAISE EXCEPTION '%: a finished delivery keeps its outcome', TG_TABLE_NAME
            USING HINT = 'Sent, skipped and failed are final. Moving a sent row back would send it '
                         'a second time.';
    END IF;
    IF NEW.attempts < OLD.attempts THEN
        RAISE EXCEPTION '%: attempts cannot decrease', TG_TABLE_NAME
            USING HINT = 'The attempt count is evidence of how hard the channel was tried.';
    END IF;
    RETURN NEW;
END $$;

-- ---------------------------------------------------------------------------
-- @entity: ZaloDelivery
-- @scope:  tenant
--
-- zalo_delivery — one Zalo message owed, or sent, to one member of staff. A row is created when the
-- BELL NOTICE is delivered (0010), not when the message leaves — the 0004 ordering: a commune or member
-- of staff that cannot receive it degrades VISIBLY ('bo-qua' + reason, ADR 0074 "mỗi lần bỏ qua ghi lý
-- do"), and a retryable failure has something to retry (429 / 408 / 5xx with back-off; anything else
-- stops — ADR 0074).
--
-- WHAT WAS SAID IS NOT COPIED HERE: the message is the bell notice's title + body (ADR 0074 #7), read
-- through notification_id. A second copy of the text would be a second source for one fact.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS zalo_delivery (
    tenant_id        TEXT        NOT NULL,
    id               TEXT        NOT NULL,            -- ULID, internal
    -- The bell notice this message repeats (0010, same service, same commune). NULL only for a test
    -- message (`POST zalo-links/current/test-messages`), which repeats no notice.
    notification_id  TEXT,
    -- The recipient's business code — staff_notification.recipient_code for a notice.
    staff_code       TEXT        NOT NULL,
    -- staff_notification.kind's values (0010), plus 'thu-nghiem' for a test message.
    kind             TEXT        NOT NULL,
    status           TEXT        NOT NULL DEFAULT 'cho-gui',
    skip_reason      TEXT,
    -- The link (and so the chat) a message was sent through, set at the send. Evidence of WHERE it
    -- went without a second copy of chat_id (personal data).
    link_id          TEXT,
    attempts         SMALLINT    NOT NULL DEFAULT 0,
    -- When the sender may try next: now, after a back-off, or the end of the commune's quiet hours.
    next_attempt_at  TIMESTAMPTZ DEFAULT now(),
    sent_at          TIMESTAMPTZ,
    -- AN ERROR CLASS, NEVER ZALO'S MESSAGE OR A GO ERROR STRING. The bot token sits IN THE URL
    -- (ADR 0074): a *url.Error prints it. A closed list cannot carry a token.
    error_class      TEXT,
    deleted_at       TIMESTAMPTZ,
    deleted_by       TEXT,
    delete_reason    TEXT,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, id),
    -- AT MOST ONE ZALO MESSAGE PER BELL NOTICE — queues deliver at least once (rule 2, invariant 5).
    -- Composite with tenant_id; NOT partial, so a retired row still blocks a resend. Test messages
    -- (NULL) are not constrained: NULLs are distinct.
    UNIQUE (tenant_id, notification_id),
    FOREIGN KEY (tenant_id, notification_id) REFERENCES staff_notification (tenant_id, id),
    FOREIGN KEY (tenant_id, link_id) REFERENCES zalo_link (tenant_id, id),
    CONSTRAINT zalo_delivery_kind_known
        CHECK (kind IN ('sap-den-han', 'qua-han', 'leo-thang', 'ban-tin-tuan', 'thu-nghiem')),
    CONSTRAINT zalo_delivery_test_has_no_notice
        CHECK ((kind = 'thu-nghiem') = (notification_id IS NULL)),
    --   cho-gui   owed, not yet sent (possibly waiting out quiet hours or a back-off)
    --   da-gui    Zalo accepted it
    --   bo-qua    deliberately not sent — skip_reason says why
    --   that-bai  Zalo refused it or retries ran out — error_class says how
    CONSTRAINT zalo_delivery_status_known
        CHECK (status IN ('cho-gui', 'da-gui', 'bo-qua', 'that-bai')),
    --   chua-lien-ket      the member of staff has no live link
    --   kenh-tat           the commune's channel is off, or has no settings row
    --   loai-tat           the commune did not enable this kind
    --   bot-chua-cau-hinh  the shared bot has no token
    -- Closed, so every skip is countable. A new reason is one line in a new migration.
    CONSTRAINT zalo_delivery_skip_reason_known
        CHECK (skip_reason IS NULL
               OR skip_reason IN ('chua-lien-ket', 'kenh-tat', 'loai-tat', 'bot-chua-cau-hinh')),
    CONSTRAINT zalo_delivery_skip_has_reason
        CHECK ((status = 'bo-qua') = (skip_reason IS NOT NULL)),
    -- The contract enum ZaloBotCallOutcome — the SAME values as zalo_bot_shared.last_check_result,
    -- one vocabulary for one Zalo call — minus OK (not an error) and NOT_CONFIGURED (a skip,
    -- 'bot-chua-cau-hinh', not a failure):
    --   token-bi-tu-choi   TOKEN_REJECTED        gioi-han-tan-suat  RATE_LIMITED (429, retried)
    --   khong-kha-dung     UNAVAILABLE (408 / 5xx / network, retried)
    --   bi-tu-choi         REJECTED              phan-hoi-sai-dang  MALFORMED_RESPONSE
    CONSTRAINT zalo_delivery_error_class_known
        CHECK (error_class IS NULL
               OR error_class IN ('token-bi-tu-choi', 'gioi-han-tan-suat', 'khong-kha-dung',
                                  'bi-tu-choi', 'phan-hoi-sai-dang')),
    CONSTRAINT zalo_delivery_failure_has_class
        CHECK (status <> 'that-bai' OR error_class IS NOT NULL),
    -- "Sent" without a time is a claim with no date on it (0004); and it went through SOME link.
    CONSTRAINT zalo_delivery_sent_has_time
        CHECK ((status = 'da-gui') = (sent_at IS NOT NULL)),
    CONSTRAINT zalo_delivery_sent_has_link
        CHECK (status <> 'da-gui' OR link_id IS NOT NULL),
    -- Only an owed message is scheduled; a finished one leaves the sender's index.
    CONSTRAINT zalo_delivery_pending_is_scheduled
        CHECK ((status = 'cho-gui') = (next_attempt_at IS NOT NULL)),
    CONSTRAINT zalo_delivery_attempts_non_negative CHECK (attempts >= 0),
    CONSTRAINT zalo_delivery_staff_code_length CHECK (char_length(staff_code) BETWEEN 1 AND 64),
    CONSTRAINT zalo_delivery_soft_delete_complete
        CHECK ((deleted_at IS NULL) = (deleted_by IS NULL)
               AND (deleted_at IS NULL) = (delete_reason IS NULL))
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS zalo_delivery_p%s PARTITION OF zalo_delivery '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

-- THE SENDER'S QUEUE, per commune (background work carries tenant_id — rule 1, invariant 9): what is
-- owed and due. Finished rows are outside the index, so it stays the size of the backlog.
CREATE INDEX IF NOT EXISTS zalo_delivery_due
    ON zalo_delivery (tenant_id, next_attempt_at)
    WHERE status = 'cho-gui' AND deleted_at IS NULL;

-- One member of staff's Zalo history, newest first.
CREATE INDEX IF NOT EXISTS zalo_delivery_by_staff
    ON zalo_delivery (tenant_id, staff_code, created_at DESC, id DESC)
    WHERE deleted_at IS NULL;

DROP TRIGGER IF EXISTS zalo_delivery_guard ON zalo_delivery;
CREATE TRIGGER zalo_delivery_guard
    BEFORE UPDATE OR DELETE ON zalo_delivery
    FOR EACH ROW EXECUTE FUNCTION zalo_delivery_guard();

-- ===========================================================================
-- PLATFORM SCOPE — below this line nothing belongs to a commune.
-- ===========================================================================

-- ---------------------------------------------------------------------------
-- platform_data_encryption_key_guard — 0008's data_encryption_key_guard, with `scope` in place of
-- tenant_id. UPDATE of kek_id / wrapped stays allowed: that is a KEK rotation re-wrapping the SAME
-- data key, so nothing sealed under it changes.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION platform_data_encryption_key_guard() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION '%: delete refused', TG_TABLE_NAME
            USING HINT = 'Deleting the platform data key destroys every platform secret sealed '
                         'under it — the shared Zalo Bot token first (ADR 0074 #4).';
    END IF;
    IF NEW.scope      IS DISTINCT FROM OLD.scope
    OR NEW.created_at IS DISTINCT FROM OLD.created_at THEN
        RAISE EXCEPTION '%: scope and created_at are immutable', TG_TABLE_NAME
            USING HINT = 'A wrapped data key is bound to its scope and does not open elsewhere.';
    END IF;
    RETURN NEW;
END $$;

-- ---------------------------------------------------------------------------
-- @entity: PlatformDataEncryptionKey
-- @scope:  platform
--
-- platform_data_encryption_key — THE data key (DEK) for secrets that belong to NO commune, wrapped
-- under a platform KEK from SECRET_ENCRYPTION_KEYS. Ciphertext, not a secret (ADR 0009). ADR 0074 #4.
--
-- WHY NOT A ROW OF data_encryption_key (0008) UNDER SOME tenant_id: that would be a FAKE COMMUNE on
-- the isolation path — rule 1 forbidden #1 (a default on the isolation path), and rule 1 invariant 2
-- (tenant_id is a commune's opaque id). A sentinel tenant_id would also be accepted by every scoped
-- store and every tenant-keyed cache as if it were a commune. A separate table, keyed by a scope that
-- can never be a ULID, keeps the two key spaces apart by construction.
--
-- `scope` is a closed list of ONE value. Single row by construction; a second platform purpose would
-- widen the CHECK in a new migration and decide whether it shares this key.
--
-- OWED BY GO (core/crypto, ADR 0074 #4): a platform sealing function whose wrap additional data binds
-- the scope instead of a tenant_id, with a label distinct from the commune wrap — so a wrapped key
-- copied between the two tables opens in neither. The length CHECK assumes the same version-1 wrapped
-- format as 0008 (1 + 4 + 12 + 32 + 16 = 65); a different format changes this CHECK in its migration.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS platform_data_encryption_key (
    scope       TEXT        PRIMARY KEY,
    -- 8 hex characters naming the KEK that wrapped this row (core/crypto/keyring.go:55-58).
    kek_id      TEXT        NOT NULL,
    wrapped     BYTEA       NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT platform_data_encryption_key_scope_known CHECK (scope IN ('platform')),
    CONSTRAINT platform_data_encryption_key_kek_id_shape CHECK (kek_id ~ '^[0-9a-f]{8}$'),
    CONSTRAINT platform_data_encryption_key_wrapped_length CHECK (octet_length(wrapped) = 65)
);

DROP TRIGGER IF EXISTS platform_data_encryption_key_guard ON platform_data_encryption_key;
CREATE TRIGGER platform_data_encryption_key_guard
    BEFORE UPDATE OR DELETE ON platform_data_encryption_key
    FOR EACH ROW EXECUTE FUNCTION platform_data_encryption_key_guard();

-- ---------------------------------------------------------------------------
-- zalo_bot_shared_guard
--
--   DELETE                   refused: the row is replaced in place, and its history is the audit trail.
--   bot_ref, created_at      fixed.
--   a new token / secret /   must come with a new set_at / webhook_set_at — a secret replaced without
--   bot account without a    its who-and-when shows the operator screen ("đặt lúc … bởi …", ADR 0074
--   new "when"               #4) a statement about the OLD secret.
--   the pending webhook      may be promoted, replaced or cleared freely: it is not yet in force.
--   secret
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION zalo_bot_shared_guard() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION '%: delete refused', TG_TABLE_NAME
            USING HINT = 'Replace the token in place; every change is an audit entry.';
    END IF;
    IF NEW.bot_ref    IS DISTINCT FROM OLD.bot_ref
    OR NEW.created_at IS DISTINCT FROM OLD.created_at THEN
        RAISE EXCEPTION '%: bot_ref and created_at are immutable', TG_TABLE_NAME
            USING HINT = 'There is one shared bot row.';
    END IF;
    IF (NEW.token_sealed   IS DISTINCT FROM OLD.token_sealed
        OR NEW.bot_account_id IS DISTINCT FROM OLD.bot_account_id)
       AND NEW.set_at IS NOT DISTINCT FROM OLD.set_at THEN
        RAISE EXCEPTION '%: a new token or bot account needs a new set_at and set_by', TG_TABLE_NAME
            USING HINT = 'The screen shows when and by whom the token was set.';
    END IF;
    IF NEW.webhook_secret_sealed IS DISTINCT FROM OLD.webhook_secret_sealed
       AND NEW.webhook_secret_sealed IS NOT NULL
       AND NEW.webhook_set_at IS NOT DISTINCT FROM OLD.webhook_set_at THEN
        RAISE EXCEPTION '%: a new webhook secret needs a new webhook_set_at', TG_TABLE_NAME
            USING HINT = 'The screen shows when and by whom the webhook was set.';
    END IF;
    RETURN NEW;
END $$;

-- ---------------------------------------------------------------------------
-- @entity: ZaloBotShared
-- @scope:  platform
--
-- zalo_bot_shared — the ONE bot every commune shares in wave 1 (ADR 0074 #1), managed from
-- platform-admin (`zalo-bots/shared`, `ops.zalo_bot.manage`). Single row by construction: bot_ref is the
-- key and admits only 'shared'. A commune's own bot, later, is commune-scoped configuration — a
-- different table, not a second row here. No row = no bot: every delivery is skipped as
-- 'bot-chua-cau-hinh'.
--
-- BOTH SECRETS ARE SEALED, WRITE-ONLY (ADR 0074 #4): `token_sealed` and `webhook_secret_sealed` are
-- core/crypto output under platform_data_encryption_key, bound to this table and column by their
-- additional data. Never returned to a client — reads report only "set at … by …". The token travels
-- IN THE URL of every Zalo call (ADR 0074): never log a request URL, never store an error string.
--
-- EVERY WRITE HERE writes one platform_operator_audit_log row in the same transaction (owner,
-- 05/10/2026; rule 6 invariant 3) — before/after carry set_at / set_by / bot_account_id / bot_name /
-- chat_url, NEVER a sealed column.
--
-- A DIFFERENT BOT ACCOUNT ENDS EVERY LINK (owner, 05/10/2026): a chat_id belongs to one bot, so after
-- bot_account_id changes, every live `zalo_link` with bot_ref = 'shared' points at a chat the new bot
-- cannot reach. The save ends them all in the same transaction (unlinked_by = the operator's code,
-- unlink_reason a fixed sentence — the `unlinked_*` trio IS this table family's soft delete, see
-- zalo_link). `zalo_bot_shared_account_change_check` below refuses the COMMIT if any is left live.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS zalo_bot_shared (
    bot_ref                         TEXT        PRIMARY KEY,
    token_sealed                    BYTEA       NOT NULL,
    -- The bot's own id as Zalo's getMe returns it, stored when the token is saved. Not a secret, not
    -- personal data. What tells "a new token for the same bot" from "a different bot".
    bot_account_id                  TEXT        NOT NULL,
    -- SUPPLIED BY THE OPERATOR with the token (SetSharedZaloBot, owner 05/10/2026) — not read from
    -- getMe. Shown to staff when they pair.
    bot_name                        TEXT        NOT NULL DEFAULT '',
    -- SUPPLIED BY THE OPERATOR: the link staff open to reach the bot's chat. https only: it is
    -- rendered as a link.
    chat_url                        TEXT        NOT NULL DEFAULT '',
    -- When and by whom the token was set: the OPERATOR's business code (`VH-…`, rule 6 invariant 8).
    set_at                          TIMESTAMPTZ NOT NULL,
    set_by                          TEXT        NOT NULL,
    -- The last `POST zalo-bots/shared/check`: a CLASS, never Zalo's text (the token is in the URL).
    -- Values are the contract enum ZaloBotCallOutcome, spelled per ADR 0011 (enum VALUES are
    -- Vietnamese without diacritics; the proto names are the English side of the mapping):
    --   thanh-cong         OK
    --   chua-cau-hinh      NOT_CONFIGURED
    --   token-bi-tu-choi   TOKEN_REJECTED
    --   gioi-han-tan-suat  RATE_LIMITED
    --   khong-kha-dung     UNAVAILABLE
    --   bi-tu-choi         REJECTED
    --   phan-hoi-sai-dang  MALFORMED_RESPONSE
    last_check_at                   TIMESTAMPTZ,
    last_check_result               TEXT,
    -- The webhook's secret_token IN FORCE (checked against X-Bot-Api-Secret-Token in constant time,
    -- ADR 0074 #5) and who registered it. All three, or none: no webhook registered yet.
    webhook_secret_sealed           BYTEA,
    webhook_set_at                  TIMESTAMPTZ,
    webhook_set_by                  TEXT,
    -- PENDING-THEN-PROMOTE (owner, 05/10/2026): a new secret is stored here first, registered with
    -- Zalo, then promoted into webhook_secret_sealed. Between the two the webhook may accept either,
    -- so an update Zalo signs with the new secret during the switch is not refused. Both, or neither.
    webhook_secret_pending_sealed   BYTEA,
    webhook_secret_pending_at       TIMESTAMPTZ,
    created_at                      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT zalo_bot_shared_single_row CHECK (bot_ref IN ('shared')),
    -- 1 + 12 + 16 bytes of envelope overhead, plus at least one byte of secret (0008).
    CONSTRAINT zalo_bot_shared_token_sealed_length CHECK (octet_length(token_sealed) > 29),
    CONSTRAINT zalo_bot_shared_bot_account_id_length
        CHECK (btrim(bot_account_id) <> '' AND char_length(bot_account_id) <= 128),
    CONSTRAINT zalo_bot_shared_webhook_pending_pair
        CHECK ((webhook_secret_pending_sealed IS NULL) = (webhook_secret_pending_at IS NULL)),
    CONSTRAINT zalo_bot_shared_webhook_pending_length
        CHECK (webhook_secret_pending_sealed IS NULL OR octet_length(webhook_secret_pending_sealed) > 29),
    CONSTRAINT zalo_bot_shared_bot_name_length CHECK (char_length(bot_name) <= 255),
    CONSTRAINT zalo_bot_shared_chat_url_shape
        CHECK (chat_url = '' OR (left(chat_url, 8) = 'https://' AND char_length(chat_url) <= 512)),
    CONSTRAINT zalo_bot_shared_set_by_signed
        CHECK (btrim(set_by) <> '' AND char_length(set_by) <= 64),
    CONSTRAINT zalo_bot_shared_check_pair
        CHECK ((last_check_at IS NULL) = (last_check_result IS NULL)),
    CONSTRAINT zalo_bot_shared_check_result_known
        CHECK (last_check_result IS NULL
               OR last_check_result IN ('thanh-cong', 'chua-cau-hinh', 'token-bi-tu-choi',
                                        'gioi-han-tan-suat', 'khong-kha-dung', 'bi-tu-choi',
                                        'phan-hoi-sai-dang')),
    CONSTRAINT zalo_bot_shared_webhook_complete
        CHECK ((webhook_secret_sealed IS NULL) = (webhook_set_at IS NULL)
               AND (webhook_secret_sealed IS NULL) = (webhook_set_by IS NULL)),
    CONSTRAINT zalo_bot_shared_webhook_secret_length
        CHECK (webhook_secret_sealed IS NULL OR octet_length(webhook_secret_sealed) > 29),
    CONSTRAINT zalo_bot_shared_webhook_set_by_signed
        CHECK (webhook_set_by IS NULL
               OR (btrim(webhook_set_by) <> '' AND char_length(webhook_set_by) <= 64))
);

DROP TRIGGER IF EXISTS zalo_bot_shared_guard ON zalo_bot_shared;
CREATE TRIGGER zalo_bot_shared_guard
    BEFORE UPDATE OR DELETE ON zalo_bot_shared
    FOR EACH ROW EXECUTE FUNCTION zalo_bot_shared_guard();

-- ---------------------------------------------------------------------------
-- zalo_bot_shared_account_change_check — the floor under "a different bot ends every link".
--
-- DEFERRED TO COMMIT, so the save may update this row and end the links in either order inside one
-- transaction. At commit: if bot_account_id changed, no link of the shared bot linked BEFORE this save
-- may still be live. A forgotten step then fails the save, instead of leaving staff "linked" to chats
-- the new bot cannot message, with every delivery failing and nobody told.
--
-- Reads zalo_link across communes BY DESIGN: the shared bot belongs to no commune, and neither does
-- this check. It reads, never writes — ending the links (and auditing each in its commune's audit_log)
-- is the application's job.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION zalo_bot_shared_account_change_check() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.bot_account_id IS DISTINCT FROM OLD.bot_account_id
       AND EXISTS (SELECT 1 FROM zalo_link
                   WHERE bot_ref = NEW.bot_ref
                     AND unlinked_at IS NULL
                     AND linked_at < NEW.set_at) THEN
        RAISE EXCEPTION '%: the bot account changed but links to the old bot are still live', TG_TABLE_NAME
            USING HINT = 'End every live zalo_link of this bot (unlinked_by = the operator code) '
                         'in the same transaction as the account change.';
    END IF;
    RETURN NULL;
END $$;

DROP TRIGGER IF EXISTS zalo_bot_shared_account_change_check ON zalo_bot_shared;
CREATE CONSTRAINT TRIGGER zalo_bot_shared_account_change_check
    AFTER UPDATE ON zalo_bot_shared
    DEFERRABLE INITIALLY DEFERRED
    FOR EACH ROW EXECUTE FUNCTION zalo_bot_shared_account_change_check();

-- ---------------------------------------------------------------------------
-- platform_operator_audit_log_append_only — UPDATE, DELETE and TRUNCATE refused (rule 6, inv 4).
-- The message names the operation and the table, never the row.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION platform_operator_audit_log_append_only() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'platform_operator_audit_log is append-only: % on % refused', TG_OP, TG_TABLE_NAME
        USING HINT = 'Audit entries are archival records (rule 6, invariant 4): never modified, '
                     'never removed, never truncated. To correct a wrong entry, append a new one.';
END $$;

-- ---------------------------------------------------------------------------
-- @entity: PlatformOperatorAuditEntry
-- @scope:  platform
--
-- platform_operator_audit_log — the trail of OPERATOR acts on comms' platform-scope data (owner,
-- 05/10/2026): every write on zalo_bot_shared, and the cross-commune read behind
-- `GET zalo-bots/shared/communes` (rule 6 invariant 7 — a read across communes is itself audited).
-- One row, in the same transaction as the act (rule 6 invariant 3).
--
-- WHY NOT comms' audit_log: it is keyed and partitioned by tenant_id NOT NULL, and these acts belong
-- to no commune. A made-up tenant_id is rule 1 forbidden #1; one entry per commune would claim each
-- commune did it. The same reasoning, and the same shape, as service-platform 0008 platform_audit_log
-- — a table per owning service, because comms cannot write platform's database (rule 2).
--
-- WHO IS ONLY AN OPERATOR: actor_code must look like an operator business code (`VH-` + at least five
-- digits, rule 6 invariant 8). No 'system', no staff code, no internal id — a row that names nobody
-- recognisable is refused at the write.
--
-- before/after: the significant fields, MASKED — never a sealed column, never a token, never a chat_id
-- (rule 6 forbidden #4, rule 3). Both NULL is allowed: a read changes nothing, and is still an entry.
--
-- NOT PARTITIONED: a handful of rows a month, and no tenant_id to hash on.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS platform_operator_audit_log (
    id          BIGSERIAL   PRIMARY KEY,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    actor_code  TEXT        NOT NULL,
    -- The client address the operator area observed, forwarded by service-platform. Required: an
    -- operator act always has a request behind it (rule 6 invariant 2).
    ip          TEXT        NOT NULL,
    -- Business verb, e.g. 'zalo_bot.token_set', 'zalo_bot.communes_read'.
    action      TEXT        NOT NULL,
    -- The record's business key, e.g. 'zalo_bot_shared/shared'.
    target      TEXT        NOT NULL,
    before      JSONB,
    after       JSONB,
    CONSTRAINT platform_operator_audit_log_actor_is_operator CHECK (actor_code ~ '^VH-[0-9]{5,}$'),
    CONSTRAINT platform_operator_audit_log_ip_not_blank CHECK (btrim(ip) <> '' AND char_length(ip) <= 64),
    CONSTRAINT platform_operator_audit_log_action_not_blank
        CHECK (btrim(action) <> '' AND char_length(action) <= 100),
    CONSTRAINT platform_operator_audit_log_target_not_blank
        CHECK (btrim(target) <> '' AND char_length(target) <= 200)
);

CREATE INDEX IF NOT EXISTS platform_operator_audit_log_by_target
    ON platform_operator_audit_log (target, created_at DESC);

DROP TRIGGER IF EXISTS platform_operator_audit_log_no_update_delete ON platform_operator_audit_log;
CREATE TRIGGER platform_operator_audit_log_no_update_delete
    BEFORE UPDATE OR DELETE ON platform_operator_audit_log
    FOR EACH ROW EXECUTE FUNCTION platform_operator_audit_log_append_only();

DROP TRIGGER IF EXISTS platform_operator_audit_log_no_truncate ON platform_operator_audit_log;
CREATE TRIGGER platform_operator_audit_log_no_truncate
    BEFORE TRUNCATE ON platform_operator_audit_log
    FOR EACH STATEMENT EXECUTE FUNCTION platform_operator_audit_log_append_only();

-- ---------------------------------------------------------------------------
-- WHAT THIS FILE DOES NOT STOP: TRUNCATE and DDL by the table owner — the same line 0003 and ADR 0013
-- draw.
--
-- REVERSAL (question 3). While every table here is empty the reversal is complete and loses nothing:
-- drop, in this order, zalo_delivery (it references zalo_link and staff_notification), zalo_link,
-- zalo_pairing_code, zalo_channel_setting, zalo_bot_shared, platform_data_encryption_key,
-- platform_operator_audit_log — partitions, indexes and triggers go with them — then the eight
-- functions (six guards, zalo_bot_shared_account_change_check, platform_operator_audit_log_append_only),
-- and in the same transaction remove this file's row from `schema_migration`, keyed on
-- ten = '0018_zalo_bot.sql'.
--
-- ONCE ANY ROW EXISTS it is no longer a reversal. A row in platform_data_encryption_key is the only
-- key to the sealed bot token; zalo_delivery and zalo_link are the record of who was reminded where;
-- platform_operator_audit_log is an audit trail (rule 6 stop condition #2). Dropping them is rule 7's
-- first stop condition and needs the user.
--
-- Written as prose rather than as runnable lines, because a runnable line is a line that gets run.
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
