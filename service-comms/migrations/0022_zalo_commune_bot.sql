-- 0022 — A COMMUNE'S OWN ZALO BOT (ADR 0079 #4 and §"Quan hệ với ADR 0074": replaces ADR 0074 #1, #3, #4
-- for the commune bot only; technical package ADR 0079 §Q1, owner 08/10/2026 "Chấp nhận cả 9"). The
-- commune configures it itself in web-admin (Cấu hình › Kênh Zalo, spec Cấu hình 11 §3), and can go back
-- to the shared bot ("Quay về bot chung").
--
--   zalo_commune_bot   tenant *   one LIVE row per commune: sealed token, sealed webhook secrets
--   zalo_link          (0018)     bot_ref widened: 'shared' or 'commune:<bot_account_id>'; no default
--   zalo_bot_shared    (0018)     a new trigger: its bot account may not be a live commune bot
--
--   * tenant-owned, NOT partitioned — the same deliberate departure as 0018's zalo_link, see below.
--
-- WHY A NEW FILE: core/migrate compares the checksum of every applied file at startup. 0018 is not edited.
--
-- NAMED IN ENGLISH (rule 12, ADR 0051). `zalo_commune_bot`, beside 0018's `zalo_bot_shared`: "commune" is
-- the ubiquitous-language word for xã. Enum values reuse 0018's (ADR 0011).
--
-- ---------------------------------------------------------------------------
-- WHAT IS DECIDED, and where each lands:
--
--   ADR 0079 Q1 #1  token sealed with the PER-COMMUNE data key   token_sealed, under 0008's
--                   (rule 8 stop condition #1, owner-approved)   data_encryption_key
--   ADR 0079 Q1 #2  the commune bot's webhook is received on the nothing here resolves a commune from a
--                   commune's OWN domain; the commune comes from bot — see the cross-tenant index below
--                   Host (rule 1 inv 3); no cross-commune read
--   ADR 0079 Q1 #3  webhook secret WRITE-ONLY, generated, shown webhook_secret_sealed + pending, as
--                   ONCE when generated — never typed, never    0018:746-750 (pending-then-promote)
--                   returned by a GET (ADR 0074 #4)
--   ADR 0079 Q1 #4  switching shared <-> own bot ENDS EVERY LIVE two DEFERRED checks + a link trigger,
--                   LINK; staff re-pair                          both directions — see below
--   owner A1        bot_account_id, bot_name, chat_url,          columns, as 0018's shared bot
--                   last_check_*, set_at/set_by (staff CODE,
--                   rule 6 inv 8), retire trio (no DELETE)
--   owner A1        bot_ref carries the bot's identity           'commune:<bot_account_id>' (0018:405's
--                                                                chat key is platform-wide on
--                                                                (bot_ref, chat_id))
--
-- ---------------------------------------------------------------------------
-- VENDOR CHOICES, stated so a reviewer can overrule them:
--
--   * ONE ROW PER BOT ADOPTION, NOT ONE ROW PER COMMUNE. `bot_account_id` is immutable on a row. A new
--     token for the SAME bot (getMe returns the same id) is an in-place update with a new set_at, as for
--     the shared bot. A DIFFERENT bot is: retire the live row, insert a new one, in one transaction. So
--     the table is the history of which bot served the commune when, and `bot_ref` stays a FACT of the
--     row: the links a bot made point at a bot_ref that never changes meaning.
--   * `bot_ref` IS A GENERATED COLUMN ('commune:' || bot_account_id), not a second copy typed by Go —
--     one fact, one source (rule 9). zalo_link.bot_ref is compared to it.
--   * THE TOKEN AND SECRETS ARE SEALED UNDER THE COMMUNE'S DEK (data_encryption_key, 0008), the mail
--     password's discipline (ADR 0079 Q1 #1). OWED BY GO: additional data that names the commune, the
--     ROW and the SECRET — not the column, because a pending webhook secret is promoted by copying its
--     bytes from one column to the other (app/zalo_bot_operator.go:80-87 does the same for the shared bot):
--        token            "zalo_commune_bot/token/<tenant_id>/<id>"
--        webhook secret   "zalo_commune_bot/webhook_secret/<tenant_id>/<id>"
--     Bound to the row id, so bytes copied from a retired row onto the live one do not open; and to
--     the tenant, so they do not open in another commune.
--   * A RETIRED ROW KEEPS ITS SEALED TOKEN. Nothing here destroys a secret (rule 7); the bytes are
--     unreadable without the commune's DEK and the platform KEK. BUT A RETIRED BOT'S TOKEN IS STILL VALID
--     AT ZALO: retiring it here revokes nothing there. OWED BY GO: the retire flow ("Quay về bot chung",
--     or replacing the bot) must TELL THE COMMUNE, in its confirmation and its result, to revoke or
--     regenerate the token in Zalo Bot Creator — only the commune can, and an unrevoked token is a live
--     credential for a bot that still carries the commune's name.
--   * ONE BOT ACCOUNT, ONE PLACE, PLATFORM-WIDE — A UNIQUENESS FLOOR ONLY. A bot has ONE webhook at Zalo.
--     If two communes, or a commune and the shared bot, held the same bot, the second "Đăng ký webhook"
--     would silently steal the first's updates — and a commune typing the SHARED bot's token as "its
--     own" would take the webhook of every commune on the platform. Hence the cross-tenant unique key on
--     live accounts, and a trigger on each side refusing an account the other side holds live.
--
-- NOT PARTITIONED — A DELIBERATE DEPARTURE FROM ADR 0010, for 0018 zalo_link's reason: the platform-wide
-- unique key `zalo_commune_bot_one_live_per_account` cannot contain tenant_id without ceasing to be
-- platform-wide, and PostgreSQL requires every unique index of a partitioned table to contain the
-- partition key. The table holds at most one live row per commune — hundreds platform-wide. Every read
-- is still by commune, through the scoped store (rule 1, invariant 5).
--
-- SWITCHING BOTS ENDS EVERY LIVE LINK, BOTH DIRECTIONS (ADR 0079 Q1 #4). Enforced at three points, all
-- under ONE per-commune transaction lock, 'zalo-commune-bot-tenant:' || tenant_id:
--   shared -> own    zalo_commune_bot_adopt_check (deferred, AFTER INSERT): at commit, a commune whose new
--                    bot is live has no live 'shared' link left.
--   own -> shared,   zalo_commune_bot_retire_check (deferred, AFTER UPDATE): at commit, a bot retired in
--   own -> own       this transaction has no live link left in its commune.
--   pairing          zalo_link_commune_bot_live (BEFORE INSERT on zalo_link): a 'shared' link is refused
--                    while the commune has a live own bot; a 'commune:…' link only through the commune's
--                    own LIVE bot.
--   The lock makes a pairing and a switch in the same commune serialise, so neither commits unseen by the
--   other. Ending the links (unlinked_by = the staff code, a fixed unlink_reason) and auditing each one in
--   the commune's audit_log stays the application's job; these checks read, never write.
--
-- NOT A NEW AUDIT TABLE. Every act on this table is a commune act, audited in comms' `audit_log` in the
-- same transaction (rule 6), before/after carrying set_at / set_by / bot_account_id / bot_name / chat_url
-- / retired_* — NEVER a sealed column.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. ROWS PER COMMUNE: NONE WRITTEN — no seed, no default bot. Steady state: zero rows (the commune uses
--      the shared bot) or one live row, plus one retired row per change of bot or return to shared.
--      The zalo_link CHECK swap scans zalo_link once (thousands of rows platform-wide, 0018).
--   2. IF IT STOPS HALF-WAY: cannot — one file, one transaction. IF NOT EXISTS / CREATE OR REPLACE /
--      DROP TRIGGER IF EXISTS / pg_constraint lookup; the new link CHECK is added before the old one is
--      dropped. No backfill, nothing to resume per commune.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. READ PATHS THAT CHANGE MEANING: none. Every existing zalo_link row has bot_ref = 'shared', which the
--      new CHECK admits. The new link trigger refuses a 'shared' pairing only where a live commune bot
--      exists — nowhere, while zalo_commune_bot is empty. The new zalo_bot_shared trigger refuses only an
--      account a live commune bot holds — likewise impossible today. DROP DEFAULT on zalo_link.bot_ref:
--      the only production writer names the column (internal/store/zalo_link.go:326); the one test that
--      relied on the default (platformstore/store_pg_test.go addLink) now names it too.
--   5. RETENTION: nothing dropped, retyped, emptied or renumbered. zalo_commune_bot refuses DELETE; a
--      retired row is frozen whole. Ended links stay as 0018 keeps them.
--
-- THE LOCK PATTERN — lock, then EXISTS — RELIES ON READ COMMITTED (the default; comms sets no other): a
-- query run after the advisory lock is granted takes a fresh snapshot and sees what the lock holder
-- committed. Under REPEATABLE READ or SERIALIZABLE the snapshot predates the wait, and the check could
-- pass on stale data.
--
-- PERSONAL DATA (rule 3): none in the new table — a bot's id, name and chat link are not a person's.
-- (zalo_link.chat_id stays personal data, 0018.) PG FLOOR: 13. Cluster: 16.
-- ---------------------------------------------------------------------------

-- ===========================================================================
-- zalo_commune_bot
-- ===========================================================================

-- ---------------------------------------------------------------------------
-- zalo_commune_bot_guard
--
--   DELETE                   refused: retire with retired_at / retired_by / retire_reason.
--   a retired row            frozen whole — it is the record of a bot that served this commune
--                            (rule 7, forbidden #5). Use the bot again = a new row.
--   tenant_id, id,           fixed. A different bot account is a different row; moving a row to another
--   bot_account_id,          commune moves sealed bytes bound to the first one.
--   created_at
--   a new token without      refused, as 0018's shared guard: the screen's "đặt lúc … bởi …" would
--   a new set_at             describe the OLD token.
--   a new webhook secret     refused likewise. The PENDING secret may be set, replaced or cleared freely:
--   without a new            it is not yet in force.
--   webhook_set_at
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION zalo_commune_bot_guard() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION '%: delete refused', TG_TABLE_NAME
            USING HINT = 'Retire the bot with retired_at, retired_by, retire_reason instead.';
    END IF;
    IF OLD.retired_at IS NOT NULL THEN
        RAISE EXCEPTION '%: a retired bot is not edited', TG_TABLE_NAME
            USING HINT = 'A retired bot is a historical record (rule 7, forbidden #5). Add a new row.';
    END IF;
    IF NEW.tenant_id      IS DISTINCT FROM OLD.tenant_id
    OR NEW.id             IS DISTINCT FROM OLD.id
    OR NEW.bot_account_id IS DISTINCT FROM OLD.bot_account_id
    OR NEW.created_at     IS DISTINCT FROM OLD.created_at THEN
        RAISE EXCEPTION '%: commune, row and bot account are immutable', TG_TABLE_NAME
            USING HINT = 'A different bot account is a different bot: retire this row and add a new one.';
    END IF;
    IF NEW.token_sealed IS DISTINCT FROM OLD.token_sealed
       AND NEW.set_at IS NOT DISTINCT FROM OLD.set_at THEN
        RAISE EXCEPTION '%: a new token needs a new set_at and set_by', TG_TABLE_NAME
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
-- @entity: ZaloCommuneBot
-- @scope:  tenant
--
-- zalo_commune_bot — a commune's own Zalo bot. NO LIVE ROW = THE COMMUNE USES THE SHARED BOT (0018);
-- "Quay về bot chung" retires the live row. Both secrets are sealed and WRITE-ONLY: never returned to a
-- client (the webhook secret is shown once, when generated — ADR 0079 Q1 #3); reads report only "set at …
-- by …". The token travels IN THE URL of every Zalo call (ADR 0074): never log a request URL, never store
-- an error string — last_check_result is a closed class.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS zalo_commune_bot (
    tenant_id                       TEXT        NOT NULL,
    id                              TEXT        NOT NULL,      -- ULID, internal
    -- The bot's own id as Zalo's getMe returns it when the token is saved. What tells "a new token for
    -- the same bot" (update in place) from "a different bot" (retire + new row). Printable ASCII, no
    -- space, so the bot_ref built from it is one unambiguous token.
    bot_account_id                  TEXT        NOT NULL,
    bot_ref                         TEXT        GENERATED ALWAYS AS ('commune:' || bot_account_id) STORED,
    token_sealed                    BYTEA       NOT NULL,
    -- Typed by the commune with the token (spec 11 §3 "Tên bot"; "bắt đầu bằng 'Bot'" is Go's check).
    bot_name                        TEXT        NOT NULL DEFAULT '',
    -- The link staff open to reach the bot (spec 11 §3 "Đường mở khung chat"). https only: rendered as a
    -- link and as a QR code.
    chat_url                        TEXT        NOT NULL DEFAULT '',
    -- When and by whom the token was set: the STAFF business code (`CB-…`, rule 6 invariant 8).
    set_at                          TIMESTAMPTZ NOT NULL,
    set_by                          TEXT        NOT NULL,
    -- The last "Kiểm tra kết nối": 0018's ZaloBotCallOutcome values (0018:730-738), one vocabulary for
    -- one Zalo call.
    last_check_at                   TIMESTAMPTZ,
    last_check_result               TEXT,
    -- The webhook secret IN FORCE and who registered it. All three, or none. Generated by the system
    -- (CSPRNG, rule 13 #2), never typed by a person (ADR 0079 Q1 #3).
    webhook_secret_sealed           BYTEA,
    webhook_set_at                  TIMESTAMPTZ,
    webhook_set_by                  TEXT,
    -- PENDING-THEN-PROMOTE (0018:746-750): stored first, registered with Zalo, then promoted. Between
    -- the two the webhook may accept either. Both, or neither.
    webhook_secret_pending_sealed   BYTEA,
    webhook_secret_pending_at       TIMESTAMPTZ,
    -- RETIRED: the commune went back to the shared bot, or replaced this bot with another. The trio is
    -- this table's soft delete (rule 7, invariant 1), spelled for what it means — like 0018's unlinked_*.
    -- retired_by: the staff code of whoever did it.
    retired_at                      TIMESTAMPTZ,
    retired_by                      TEXT,
    retire_reason                   TEXT,
    created_at                      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                      TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, id),
    CONSTRAINT zalo_commune_bot_account_id_shape CHECK (bot_account_id ~ '^[!-~]{1,128}$'),
    -- 1 + 12 + 16 bytes of envelope overhead, plus at least one byte of secret (0008).
    CONSTRAINT zalo_commune_bot_token_sealed_length CHECK (octet_length(token_sealed) > 29),
    CONSTRAINT zalo_commune_bot_bot_name_length CHECK (char_length(bot_name) <= 255),
    CONSTRAINT zalo_commune_bot_chat_url_shape
        CHECK (chat_url = '' OR (left(chat_url, 8) = 'https://' AND char_length(chat_url) <= 512)),
    CONSTRAINT zalo_commune_bot_set_by_signed
        CHECK (btrim(set_by) <> '' AND char_length(set_by) <= 64),
    CONSTRAINT zalo_commune_bot_check_pair
        CHECK ((last_check_at IS NULL) = (last_check_result IS NULL)),
    CONSTRAINT zalo_commune_bot_check_result_known
        CHECK (last_check_result IS NULL
               OR last_check_result IN ('thanh-cong', 'chua-cau-hinh', 'token-bi-tu-choi',
                                        'gioi-han-tan-suat', 'khong-kha-dung', 'bi-tu-choi',
                                        'phan-hoi-sai-dang')),
    CONSTRAINT zalo_commune_bot_webhook_complete
        CHECK ((webhook_secret_sealed IS NULL) = (webhook_set_at IS NULL)
               AND (webhook_secret_sealed IS NULL) = (webhook_set_by IS NULL)),
    CONSTRAINT zalo_commune_bot_webhook_secret_length
        CHECK (webhook_secret_sealed IS NULL OR octet_length(webhook_secret_sealed) > 29),
    CONSTRAINT zalo_commune_bot_webhook_set_by_signed
        CHECK (webhook_set_by IS NULL
               OR (btrim(webhook_set_by) <> '' AND char_length(webhook_set_by) <= 64)),
    CONSTRAINT zalo_commune_bot_webhook_pending_pair
        CHECK ((webhook_secret_pending_sealed IS NULL) = (webhook_secret_pending_at IS NULL)),
    CONSTRAINT zalo_commune_bot_webhook_pending_length
        CHECK (webhook_secret_pending_sealed IS NULL OR octet_length(webhook_secret_pending_sealed) > 29),
    -- All three, or none (the shape rule 7 invariant 1 asks of a soft delete).
    CONSTRAINT zalo_commune_bot_retire_complete
        CHECK ((retired_at IS NULL) = (retired_by IS NULL)
               AND (retired_at IS NULL) = (retire_reason IS NULL)),
    CONSTRAINT zalo_commune_bot_retire_signed
        CHECK (retired_by IS NULL OR (btrim(retired_by) <> '' AND char_length(retired_by) <= 64)),
    CONSTRAINT zalo_commune_bot_retire_reason_length
        CHECK (retire_reason IS NULL OR (btrim(retire_reason) <> '' AND char_length(retire_reason) <= 200)),
    CONSTRAINT zalo_commune_bot_retired_after_created
        CHECK (retired_at IS NULL OR retired_at >= created_at),
    -- A retired bot has no webhook switch in progress: the pending secret is cleared at retirement, or
    -- the frozen row would hold a secret the webhook might still accept.
    CONSTRAINT zalo_commune_bot_retired_has_no_pending
        CHECK (retired_at IS NULL OR webhook_secret_pending_sealed IS NULL)
);

-- AT MOST ONE LIVE BOT PER COMMUNE. Composite with tenant_id (rule 1, invariant 6). Partial on retired_at,
-- not deleted_at, and it frees no issued code — a bot row issues none; a retired row stays as history.
-- Switching bots retires the old row BEFORE inserting the new one (a partial unique index is checked
-- immediately, never deferred). THIS IS ALSO THE WEBHOOK'S READ (ADR 0079 Q1 #2): the update arrives on
-- the commune's own domain, the commune comes from Host, and the bot is read
-- WHERE tenant_id = <the Host's commune> AND retired_at IS NULL — one commune, no cross-commune read.
CREATE UNIQUE INDEX IF NOT EXISTS zalo_commune_bot_one_live_per_commune
    ON zalo_commune_bot (tenant_id)
    WHERE retired_at IS NULL;

-- @scope: cross-tenant — A UNIQUENESS FLOOR, NOT A LOOKUP. One live place per bot account across the
-- platform: a bot has one webhook at Zalo, and two communes holding one bot live would each re-register
-- it over the other. NOTHING READS bot_account_id -> tenant_id through this index: the commune is never
-- resolved from a bot (ADR 0079 Q1 #2 — from Host, no cross-commune read). The only cross-commune
-- touches are the database's own uniqueness check here and the account-exclusivity triggers below,
-- which answer yes/no and return nothing about another commune.
CREATE UNIQUE INDEX IF NOT EXISTS zalo_commune_bot_one_live_per_account
    ON zalo_commune_bot (bot_account_id)
    WHERE retired_at IS NULL;

DROP TRIGGER IF EXISTS zalo_commune_bot_guard ON zalo_commune_bot;
CREATE TRIGGER zalo_commune_bot_guard
    BEFORE UPDATE OR DELETE ON zalo_commune_bot
    FOR EACH ROW EXECUTE FUNCTION zalo_commune_bot_guard();

-- ---------------------------------------------------------------------------
-- zalo_bot_account_not_shared — a commune bot may not be the SHARED bot's account (see VENDOR CHOICES).
-- Taking the shared bot's token as a commune's "own" and pressing "Đăng ký webhook" would move every
-- commune's updates to one commune. Under a per-account transaction lock, taken by BOTH this trigger and
-- zalo_bot_shared_account_not_commune below, so the two checks cannot pass concurrently. Reads
-- zalo_bot_shared, which belongs to no commune; it reads, never writes.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION zalo_bot_account_not_shared() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    PERFORM pg_advisory_xact_lock(hashtextextended('zalo-bot-account:' || NEW.bot_account_id, 0));
    IF EXISTS (SELECT 1 FROM zalo_bot_shared WHERE bot_account_id = NEW.bot_account_id) THEN
        RAISE EXCEPTION '%: this bot account is the platform''s shared bot', TG_TABLE_NAME
            USING HINT = 'A commune''s own bot must be a bot the commune created. The shared bot is '
                         'already serving this commune when it has no own bot.';
    END IF;
    RETURN NEW;
END $$;

-- INSERT only: bot_account_id is immutable on a row (guard above).
DROP TRIGGER IF EXISTS zalo_bot_account_not_shared ON zalo_commune_bot;
CREATE TRIGGER zalo_bot_account_not_shared
    BEFORE INSERT ON zalo_commune_bot
    FOR EACH ROW EXECUTE FUNCTION zalo_bot_account_not_shared();

-- ---------------------------------------------------------------------------
-- zalo_bot_shared_account_not_commune — the other side: the operator may not make a bot that a commune
-- holds LIVE the shared bot. Same lock. Reads zalo_commune_bot across communes BY DESIGN — the shared bot
-- belongs to no commune and neither does this check; it reads, never writes, and returns nothing about
-- the commune it found.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION zalo_bot_shared_account_not_commune() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    PERFORM pg_advisory_xact_lock(hashtextextended('zalo-bot-account:' || NEW.bot_account_id, 0));
    IF EXISTS (SELECT 1 FROM zalo_commune_bot
               WHERE bot_account_id = NEW.bot_account_id AND retired_at IS NULL) THEN
        RAISE EXCEPTION '%: this bot account is a commune''s own bot', TG_TABLE_NAME
            USING HINT = 'The shared bot must be a bot no commune is using as its own.';
    END IF;
    RETURN NEW;
END $$;

DROP TRIGGER IF EXISTS zalo_bot_shared_account_not_commune ON zalo_bot_shared;
CREATE TRIGGER zalo_bot_shared_account_not_commune
    BEFORE INSERT OR UPDATE ON zalo_bot_shared
    FOR EACH ROW EXECUTE FUNCTION zalo_bot_shared_account_not_commune();

-- ---------------------------------------------------------------------------
-- zalo_commune_bot_adopt_check — shared -> own ends every live SHARED link (ADR 0079 Q1 #4).
--
-- DEFERRED TO COMMIT, so the save may insert the bot and end the shared links in either order inside one
-- transaction. At commit, if this row is still the commune's live bot, no live 'shared' link may remain in
-- its commune — staff "linked" to the shared bot would otherwise receive nothing (every message goes
-- through the own bot) and be told nothing. Scoped to NEW.tenant_id; reads, never writes.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION zalo_commune_bot_adopt_check() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    PERFORM pg_advisory_xact_lock(hashtextextended('zalo-commune-bot-tenant:' || NEW.tenant_id, 0));
    IF EXISTS (SELECT 1 FROM zalo_commune_bot
               WHERE tenant_id = NEW.tenant_id AND id = NEW.id AND retired_at IS NULL)
       AND EXISTS (SELECT 1 FROM zalo_link
                   WHERE tenant_id = NEW.tenant_id AND bot_ref = 'shared' AND unlinked_at IS NULL) THEN
        RAISE EXCEPTION '%: an own bot was adopted but shared-bot links are still live', TG_TABLE_NAME
            USING HINT = 'End every live shared zalo_link of this commune (unlinked_by = the staff code) '
                         'in the same transaction — staff re-pair with the own bot (ADR 0079 Q1 #4).';
    END IF;
    RETURN NULL;
END $$;

DROP TRIGGER IF EXISTS zalo_commune_bot_adopt_check ON zalo_commune_bot;
CREATE CONSTRAINT TRIGGER zalo_commune_bot_adopt_check
    AFTER INSERT ON zalo_commune_bot
    DEFERRABLE INITIALLY DEFERRED
    FOR EACH ROW EXECUTE FUNCTION zalo_commune_bot_adopt_check();

-- ---------------------------------------------------------------------------
-- zalo_commune_bot_retire_check — own -> shared and own -> own end every live link of the retired bot
-- (ADR 0079 Q1 #4); the equivalent of 0018's zalo_bot_shared_account_change_check (0018:801-820).
--
-- DEFERRED TO COMMIT, so the save may retire the row and end the links in either order inside one
-- transaction — and, when switching bots, insert the new row too. At commit: a bot retired in this
-- transaction may have NO live link left in its commune. A forgotten step then fails the save, instead of
-- leaving staff "linked" to a bot nothing sends through, with every reminder skipped and nobody told.
-- Scoped to the bot's own commune — zalo_link_commune_bot_live guarantees its links live nowhere else.
-- Reads, never writes.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION zalo_commune_bot_retire_check() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.retired_at IS NULL AND NEW.retired_at IS NOT NULL THEN
        PERFORM pg_advisory_xact_lock(hashtextextended('zalo-commune-bot-tenant:' || NEW.tenant_id, 0));
        IF EXISTS (SELECT 1 FROM zalo_link
                   WHERE tenant_id = NEW.tenant_id
                     AND bot_ref = NEW.bot_ref
                     AND unlinked_at IS NULL) THEN
            RAISE EXCEPTION '%: the bot was retired but its links are still live', TG_TABLE_NAME
                USING HINT = 'End every live zalo_link of this bot (unlinked_by = the staff code) in '
                             'the same transaction as the retirement — staff re-pair (ADR 0079 Q1 #4).';
        END IF;
    END IF;
    RETURN NULL;
END $$;

DROP TRIGGER IF EXISTS zalo_commune_bot_retire_check ON zalo_commune_bot;
CREATE CONSTRAINT TRIGGER zalo_commune_bot_retire_check
    AFTER UPDATE ON zalo_commune_bot
    DEFERRABLE INITIALLY DEFERRED
    FOR EACH ROW EXECUTE FUNCTION zalo_commune_bot_retire_check();

-- ===========================================================================
-- zalo_link — a link may now name a commune bot; bot_ref has no default
-- ===========================================================================

-- bot_ref: 'shared' (0018), or a commune bot's bot_ref. The CHECK fixes the SHAPE; the insert trigger
-- below fixes WHICH bot is allowed. Added before 0018's narrower CHECK is dropped.
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = 'zalo_link'::regclass
                   AND conname = 'zalo_link_bot_ref_shape') THEN
        ALTER TABLE zalo_link ADD CONSTRAINT zalo_link_bot_ref_shape
            CHECK (bot_ref = 'shared' OR bot_ref ~ '^commune:[!-~]{1,128}$');
    END IF;
END $$;

ALTER TABLE zalo_link DROP CONSTRAINT IF EXISTS zalo_link_bot_ref_known;

-- NO DEFAULT: with two kinds of bot, a writer that forgets bot_ref must fail, not silently pair the chat
-- to the shared bot of a commune that uses its own. Existing rows keep their value; only future INSERTs
-- must name the column (internal/store/zalo_link.go:326 already does).
ALTER TABLE zalo_link ALTER COLUMN bot_ref DROP DEFAULT;

-- ---------------------------------------------------------------------------
-- zalo_link_commune_bot_live — which bot a new link may name, given the commune's state:
--
--   the commune HAS a live own bot   only 'commune:<that bot>' — a 'shared' link is refused
--                                    (ADR 0079 Q1 #4: every message goes through the own bot)
--   the commune has NO live own bot  only 'shared' — a 'commune:…' link is refused (no such bot, or
--                                    another commune's, or a retired one)
--
-- Under the per-commune lock the two deferred checks also take, so a pairing and a switch of bot in the
-- same commune serialise and neither commits unseen by the other.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION zalo_link_commune_bot_live() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    PERFORM pg_advisory_xact_lock(hashtextextended('zalo-commune-bot-tenant:' || NEW.tenant_id, 0));
    IF NEW.bot_ref = 'shared' THEN
        IF EXISTS (SELECT 1 FROM zalo_commune_bot
                   WHERE tenant_id = NEW.tenant_id AND retired_at IS NULL) THEN
            RAISE EXCEPTION '%: this commune uses its own bot; a shared-bot link is refused', TG_TABLE_NAME
                USING HINT = 'Pair through the commune''s own bot (ADR 0079 Q1 #4).';
        END IF;
        RETURN NEW;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM zalo_commune_bot
                   WHERE tenant_id = NEW.tenant_id
                     AND bot_ref = NEW.bot_ref
                     AND retired_at IS NULL) THEN
        RAISE EXCEPTION '%: no live bot of this commune for this bot_ref', TG_TABLE_NAME
            USING HINT = 'Pair only through the commune''s live own bot, or the shared bot when it has none.';
    END IF;
    RETURN NEW;
END $$;

-- INSERT only: bot_ref is immutable on a link (0018 zalo_link_guard).
DROP TRIGGER IF EXISTS zalo_link_commune_bot_live ON zalo_link;
CREATE TRIGGER zalo_link_commune_bot_live
    BEFORE INSERT ON zalo_link
    FOR EACH ROW EXECUTE FUNCTION zalo_link_commune_bot_live();

-- ---------------------------------------------------------------------------
-- WHAT THIS FILE DOES NOT STOP: TRUNCATE and DDL by the table owner — the same line 0003 and ADR 0013
-- draw. Nor a commune bot inserted ALREADY retired (it would make no link and serve nothing).
--
-- REVERSAL (question 3). Run by a person, in ONE transaction (ADR 0013). Prose, because a runnable line
-- is a line that gets run.
--
-- LOSSLESS ONLY WHILE zalo_commune_bot IS EMPTY (and therefore no zalo_link row has a bot_ref other than
-- 'shared' — the insert trigger guarantees it). Then: restore zalo_link.bot_ref's DEFAULT 'shared'; re-add
-- 0018's zalo_link_bot_ref_known (bot_ref IN ('shared')) verbatim, drop zalo_link_bot_ref_shape; drop the
-- triggers zalo_link_commune_bot_live (on zalo_link) and zalo_bot_shared_account_not_commune (on
-- zalo_bot_shared); drop the table zalo_commune_bot (its indexes and triggers go with it); drop the six
-- functions zalo_commune_bot_guard, zalo_bot_account_not_shared, zalo_bot_shared_account_not_commune,
-- zalo_commune_bot_adopt_check, zalo_commune_bot_retire_check, zalo_link_commune_bot_live; remove this
-- file's row from `schema_migration` (ten = '0022_zalo_commune_bot.sql').
--
-- ONCE ANY ROW EXISTS it is no longer a reversal: the table holds a commune's sealed bot token and the
-- record of which bot served it, and zalo_link rows point at its bot_ref. Rule 7 stop condition #1/#2 —
-- the user, a verified backup, and a NEW migration.
-- ---------------------------------------------------------------------------
