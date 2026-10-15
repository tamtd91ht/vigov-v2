-- 0029 — ZALO GROUP CHATS (ADR 0092, owner 11/10/2026; contract comms.proto StaffNotification fields 8
-- `holding_org_unit_id` and 9 `group_posting`; transaction boundaries `gan_nhom_zalo_bang_ma` (strong) and
-- `gui_tin_nhom_zalo` (eventual)). The bot posts staff notices into Zalo GROUP chats a commune linked.
-- Three new tables, and the bot-switch checks of 0018/0022 widened to the new link table:
--
--   zalo_group_pairing_code   tenant     the one-time code a member of staff issues for ONE scope and
--                                        types into a group chat
--   zalo_group_link           tenant *   one Zalo group chat of one bot <-> one commune, with a scope
--                                        (one org unit, or the whole commune) and its own kinds
--   zalo_group_delivery       tenant     outbox + history: one notice -> one message per matching group
--
--   * tenant-owned, NOT partitioned — 0018 zalo_link's deliberate departure, for the same reason.
--
--   zalo_link                 (0018)     new INSERT trigger: a chat live as a GROUP is not paired personally
--   zalo_group_link           (here)     the same trigger the other way, and 0022's bot-liveness trigger
--   zalo_bot_shared_account_change_check   (0018)  REPLACED: also refuses live GROUP links of the old bot
--   zalo_commune_bot_adopt_check           (0022)  REPLACED: also refuses live shared GROUP links
--   zalo_commune_bot_retire_check          (0022)  REPLACED: also refuses live GROUP links of the bot
--
-- WHY A NEW FILE: core/migrate compares the checksum of every applied file at startup. 0018–0028 are not
-- edited; the three functions are replaced here by CREATE OR REPLACE, which keeps the triggers that name
-- them (0018:817, 0022:558, 0022:592) attached and unchanged.
--
-- NAMED IN ENGLISH (rule 12, ADR 0051): `group` beside 0018's `zalo_link` / `zalo_pairing_code` /
-- `zalo_delivery`, one word per concept. ENUM VALUES stay Vietnamese without diacritics (ADR 0011): the
-- kinds are zalo_channel_setting's own (0025), the delivery states and reasons are 0018/0024's.
--
-- ---------------------------------------------------------------------------
-- WHAT IS DECIDED, and where each lands:
--
--   ADR 0092 #1  many groups per commune; scope = one bo_phan or   zalo_group_link.org_unit_id, NULL =
--                the whole commune; through the commune's bot      commune-wide; bot_ref as zalo_link
--   ADR 0092 #2  kinds per group, a subset of the commune's;       kinds TEXT[], CHECKed against the
--                personal kinds NEVER go to a group                 GROUP list below (subset-of-commune
--                                                                   is Go's, at save and at send)
--   ADR 0092 #3  linked by a GROUP code issued in web-admin        zalo_group_pairing_code, issued_by
--   ADR 0092 #5  one notice = one group message, by key            UNIQUE (tenant_id, group_link_id,
--                                                                   idempotency_key) on the outbox
--   ADR 0092 #8  unlinked ONLY from web-admin, with a trail        unlinked_* trio; the audit entry is
--                                                                   in comms' audit_log (Go)
--   ADR 0092 #9  a group live in commune A cannot be linked by B   zalo_group_link_one_live_per_chat,
--                until A unlinks                                    platform-wide on (bot_ref, chat_id)
--   ADR 0092 §Hệ quả  group chat_id is personal data              never logged / returned / exported
--
-- THE GROUP KIND LIST — 0025's zalo_channel_setting list MINUS what ADR 0092 #2 and comms.proto
-- GroupPosting (GROUP_POSTING_ALLOWED: "NOT for digests or per-person notices") keep out of a group:
--
--   admitted (14)               nhiem-vu / van-ban / phan-anh  .qua-han .chua-cu-nguoi .leo-thang (9)
--                               nhiem-vu.giao-moi, nhiem-vu.cho-duyet, van-ban.chuyen-toi,
--                               phan-anh.phan-cong, phan-anh.mo-lai (5)
--   refused, per-person / digest  sap-den-han and the three *.sap-den-han (daily digest PER PERSON),
--                               ban-tin-tuan (digest), bao-cao.san-sang (REPORT_READY, per period),
--                               nhiem-vu.nhac-ten (a mention names one person), nhiem-vu.de-nghi-lui-han
--                               (ADR 0092 #2 names it), giai-ngan.nhac-ten (bell only, ADR 0081 #5)
--   refused, legacy             qua-han, leo-thang — domainless producers predate field 9 and send
--                               GROUP_POSTING_UNSPECIFIED, so a group ticking them would never receive one;
--                               a box that can never fire is a lie on the screen
--   refused, NOT DECIDED        thong-bao.moi — comms' own announcement has no producer to set
--                               group_posting, and an announcement may be addressed to a subset of staff
--                               a group's members were not checked against. FAIL CLOSED: widening is one
--                               CHECK in a new migration once the owner decides. Flagged to the caller.
--   refused                     thu-nghiem — no group test message is decided.
--
-- ---------------------------------------------------------------------------
-- VENDOR CHOICES, stated so a reviewer can overrule them:
--
--   * THE LINK RECORDS THE CODE THAT MADE IT (zalo_group_link.pairing_code_id, FK, unique per commune):
--     the scope and kinds were copied from that code, and linked_by is its issuer (rule 6 inv 8 — the
--     person typing in the group is not a ViGov identity; transaction boundary gan_nhom_zalo_bang_ma).
--   * SCOPE AND KINDS ARE EDITABLE ON A LIVE LINK (updated_at), every other column is fixed. Each edit is
--     an audit entry with before/after in comms' audit_log, in the same transaction (Go). Moving a link
--     to another chat or bot stays impossible: end it and link again.
--   * NO "ONE OPEN CODE PER ISSUER" key, unlike zalo_pairing_code_one_open_per_staff: a member of staff
--     may prepare codes for two groups at once, and nothing decided says otherwise. Codes still expire
--     and are one-use. Go may cancel older codes; the schema does not force it.
--   * RE-LINKING THE SAME GROUP IN THE SAME COMMUNE (transaction boundary gan_nhom_zalo_bang_ma, CHƯA CHỐT
--     (b): replace the scope or refuse) IS NOT DECIDED HERE. Both are expressible: refuse, or end the old
--     link and insert the new one in one transaction. zalo_group_link_one_live_per_chat allows exactly one
--     live row either way.
--   * THE OUTBOX SNAPSHOTS title / body / link. 0018's "what was said is not copied" does not carry over:
--     a group message repeats NO single bell row — one notice writes one bell row per recipient, and a
--     group is posted even when no recipient is linked (comms.proto field 8–9 header). GROUP_POSTING_ALLOWED
--     guarantees the text is the SAME for every recipient, so the snapshot is that one text, bounded by
--     0010's bell CHECKs. The bell rows remain the record; this is the copy that was posted.
--   * NO LEASE COLUMN on the outbox. zalo_delivery has none: ZaloDispatcher claims due rows FOR UPDATE
--     SKIP LOCKED (internal/store/zalo_link.go ClaimDueDeliveries). The group outbox follows it.
--   * ONE CHAT, ONE KIND OF LINK: a (bot_ref, chat_id) live in zalo_link may not be live in
--     zalo_group_link, and the reverse. A group chat paired personally is the hole ADR 0092 §Bối cảnh
--     closes in the webhook; this is the floor under it. Enforced by one trigger function on both tables
--     under a per-chat transaction lock (two partial unique indexes on two tables cannot see each other).
--
-- NOT PARTITIONED (zalo_group_link) — the departure 0018 states above zalo_link (0018:336-346), for the
-- same reason: `zalo_group_link_one_live_per_chat` is platform-wide and cannot contain tenant_id, and the
-- webhook's lookup is BY CHAT, with no commune known yet. Hundreds to low thousands of rows platform-wide.
-- Of the two other tables, zalo_group_delivery is partitioned and zalo_group_pairing_code is plain (ADR 0096).
--
-- SWITCHING BOTS (ADR 0079 Q1 #4, 0018's account change) ENDS LIVE GROUP LINKS TOO. Same three points, same
-- locks as 0022: the commune lock 'zalo-commune-bot-tenant:' || tenant_id is taken by the pairing trigger
-- (now also on zalo_group_link) and by the adopt/retire checks. Ending the links and auditing each one in
-- the commune's audit_log stays the application's job; these checks read, never write.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. ROWS PER COMMUNE: NONE WRITTEN — no seed, no default group. Steady state: zalo_group_link a few live
--      rows per commune (one per unit group, plus at most a few commune-wide), plus one ended row per unlink;
--      zalo_group_pairing_code a few per link; zalo_group_delivery at most one row per (notice, matching
--      group) — bounded by the bell's own volume (0010: low tens of thousands a year per commune) times the
--      matching groups, usually one or two.
--   2. IF IT STOPS HALF-WAY: cannot — one file, one transaction with its progress row. IF NOT EXISTS /
--      CREATE OR REPLACE / DROP TRIGGER IF EXISTS throughout; a retry from the top costs nothing. No
--      backfill, nothing to resume per commune.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. READ PATHS THAT CHANGE MEANING WHILE IT IS HALF-APPLIED: none — and none after. The three replaced
--      functions add an EXISTS on zalo_group_link, which is empty until Go writes it, so every bot switch
--      passes or fails exactly as yesterday. The new zalo_link trigger refuses a personal pairing only for
--      a chat live as a group — none exists. No existing column, constraint, index or row changes.
--   5. RETENTION: nothing is ever hard-deleted — every new table's guard refuses DELETE.
--        zalo_group_delivery  evidence that a group WAS (or was not) told; soft delete only; outcome final.
--        zalo_group_link      which group received which commune's notices, from when to when, under
--                             which scope; an ended link is frozen whole (rule 7 forbidden #5).
--        zalo_group_pairing_code  a hash and a scope — no personal data; kept as the linking history.
--      chat_id is PERSONAL DATA (ADR 0092 §Hệ quả, as ADR 0074): an erasure request on an ended link means
--      anonymising chat_id, which the guard refuses today — rule 3 stop condition #3, the user, then a
--      migration (0018's stance, unchanged).
--
-- THE LOCKS. New tables cost nothing. On EXISTING tables:
--   zalo_link        DROP TRIGGER IF EXISTS + CREATE TRIGGER zalo_link_chat_not_group: ACCESS EXCLUSIVE
--                    (DROP TRIGGER) until COMMIT. Webhook chat lookups, pairings and the dispatcher's
--                    link reads wait for this transaction — milliseconds, as it scans no table.
--   zalo_bot_shared, zalo_commune_bot   NO table lock: only their trigger FUNCTIONS are replaced
--                    (CREATE OR REPLACE FUNCTION locks the function, not the table). A switch in flight
--                    runs whichever body it started with.
--   zalo_group_link's FK to zalo_group_pairing_code and zalo_group_delivery's FK to zalo_group_link are
--   between new tables only.
--
-- THE LOCK PATTERN — lock, then EXISTS — RELIES ON READ COMMITTED, as 0022 states (0022:309-312).
--
-- PERSONAL DATA (rule 3): zalo_group_link.chat_id. Nothing else here is a person's: org unit ids, staff
-- business codes, a hash, and notice text that comms.proto already bounds to business codes. PG FLOOR: 13
-- (BEFORE ROW triggers on partitioned tables; FK to a partitioned table needs 12). Cluster: 16.
-- ---------------------------------------------------------------------------

-- ===========================================================================
-- zalo_group_pairing_code
-- ===========================================================================

-- ---------------------------------------------------------------------------
-- zalo_group_pairing_code_guard — 0018's zalo_pairing_code_guard, with the SCOPE fixed at issue too.
--
--   DELETE              refused. A deleted code row frees its hash for reuse in this commune.
--   identity columns    commune, issuer, hash, expiry AND scope (org_unit_id, kinds): fixed at issue.
--                       Widening the scope of an issued code opens a read the issuer never chose.
--   used_at,            NULL -> a value, once. A closed code is frozen whole.
--   cancelled_at
--   failed_attempts     never decreases.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION zalo_group_pairing_code_guard() RETURNS trigger
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
    IF NEW.tenant_id   IS DISTINCT FROM OLD.tenant_id
    OR NEW.id          IS DISTINCT FROM OLD.id
    OR NEW.issued_by   IS DISTINCT FROM OLD.issued_by
    OR NEW.org_unit_id IS DISTINCT FROM OLD.org_unit_id
    OR NEW.kinds       IS DISTINCT FROM OLD.kinds
    OR NEW.code_hash   IS DISTINCT FROM OLD.code_hash
    OR NEW.expires_at  IS DISTINCT FROM OLD.expires_at
    OR NEW.created_at  IS DISTINCT FROM OLD.created_at THEN
        RAISE EXCEPTION '%: an issued code is immutable', TG_TABLE_NAME
            USING HINT = 'Commune, issuer, scope, hash and expiry are fixed at issue.';
    END IF;
    IF NEW.failed_attempts < OLD.failed_attempts THEN
        RAISE EXCEPTION '%: failed_attempts cannot decrease', TG_TABLE_NAME
            USING HINT = 'Lowering the count hands out fresh guesses against the same code.';
    END IF;
    RETURN NEW;
END $$;

-- ---------------------------------------------------------------------------
-- @entity: ZaloGroupPairingCode
-- @scope:  tenant
--
-- zalo_group_pairing_code — the code a member of staff issues in web-admin (Cấu hình → Kênh Zalo,
-- admin.lookup, ADR 0092 #3) for ONE scope, and types into a group chat (also as '@Bot CODE'). Only the
-- hash is stored; length, alphabet, lifetime and the wrong-try limit live in Go, as for 0018's code.
--
-- A SEPARATE TABLE FROM zalo_pairing_code, ON PURPOSE: the webhook decides by chat type (internal/zalobot
-- ChatType) — a GROUP chat looks a code up HERE only, a PRIVATE chat in zalo_pairing_code only. A personal
-- code typed in a group is refused without lookup (ADR 0092 #4); a group code typed in a private chat is
-- simply not found. The same 8 characters existing in both tables therefore resolve nothing ambiguously.
--
-- THE SHARED BOT HAS NO COMMUNE HOST: the commune is found FROM this code — the cross-commune lookup
-- `zalo_group_pairing_code_open_by_hash`, the one index here that does not lead with tenant_id (0018's
-- zalo_pairing_code_open_by_hash precedent). OWED BY GO: that read carries `// @cross-tenant:`, and more
-- than one open match is NO match (fail closed). A commune's own bot resolves the commune from Host and
-- reads within it.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS zalo_group_pairing_code (
    tenant_id        TEXT        NOT NULL,
    id               TEXT        NOT NULL,            -- ULID, internal
    -- `CB-…`, nguoi_dung.ma of the member of staff who issued it — the session's own code. Becomes the
    -- link's linked_by (rule 6 inv 8). NO FOREIGN KEY: the staff directory is identity's (rule 2).
    issued_by        TEXT        NOT NULL,
    -- The identity `bo_phan` the group will follow (its ULID), or NULL for the whole commune. NO FOREIGN
    -- KEY: identity owns bo_phan (rule 2; precedent 0005 thong_bao_bo_phan.bo_phan_id). Whether the unit
    -- exists and is live is Go's check at issue, through identity.
    org_unit_id      TEXT,
    -- The kinds the group will receive. Non-empty, each from the group list (header). That they are a
    -- subset of the kinds the commune has switched on is checked by Go at issue.
    kinds            TEXT[]      NOT NULL,
    -- SHA-256 / HMAC-SHA-256 of the code: 32 bytes. Never the code itself.
    code_hash        BYTEA       NOT NULL,
    expires_at       TIMESTAMPTZ NOT NULL,
    -- The code produced a link (zalo_group_link.pairing_code_id names it). Exclusive with cancelled_at.
    used_at          TIMESTAMPTZ,
    -- Closed without use: cancelled by staff, or too many wrong tries.
    cancelled_at     TIMESTAMPTZ,
    failed_attempts  SMALLINT    NOT NULL DEFAULT 0,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, id),
    -- NOT partial: a hash once issued in a commune is never issued there again, used or not.
    UNIQUE (tenant_id, code_hash),
    CONSTRAINT zalo_group_pairing_code_hash_length CHECK (octet_length(code_hash) = 32),
    CONSTRAINT zalo_group_pairing_code_issued_by_length
        CHECK (btrim(issued_by) <> '' AND char_length(issued_by) <= 64),
    CONSTRAINT zalo_group_pairing_code_org_unit_shape
        CHECK (org_unit_id IS NULL OR org_unit_id ~ '^[0-9A-HJKMNP-TV-Z]{26}$'),
    CONSTRAINT zalo_group_pairing_code_kinds_present CHECK (cardinality(kinds) > 0),
    CONSTRAINT zalo_group_pairing_code_kinds_known
        CHECK (kinds <@ ARRAY['nhiem-vu.qua-han', 'nhiem-vu.chua-cu-nguoi', 'nhiem-vu.leo-thang',
                              'van-ban.qua-han', 'van-ban.chua-cu-nguoi', 'van-ban.leo-thang',
                              'phan-anh.qua-han', 'phan-anh.chua-cu-nguoi', 'phan-anh.leo-thang',
                              'nhiem-vu.giao-moi', 'nhiem-vu.cho-duyet', 'van-ban.chuyen-toi',
                              'phan-anh.phan-cong', 'phan-anh.mo-lai']::text[]),
    CONSTRAINT zalo_group_pairing_code_expiry_after_issue CHECK (expires_at > created_at),
    CONSTRAINT zalo_group_pairing_code_closed_once CHECK (used_at IS NULL OR cancelled_at IS NULL),
    CONSTRAINT zalo_group_pairing_code_attempts_non_negative CHECK (failed_attempts >= 0)
);

-- THE WEBHOOK'S LOOKUP: hash -> (commune, scope). Cross-commune by nature (see above); open codes only,
-- so the index stays the size of the codes alive right now. Not unique — see zalo_pairing_code (0018:229-235).
CREATE INDEX IF NOT EXISTS zalo_group_pairing_code_open_by_hash
    ON zalo_group_pairing_code (code_hash)
    WHERE used_at IS NULL AND cancelled_at IS NULL;

-- The screen's list of a commune's open codes, newest first.
CREATE INDEX IF NOT EXISTS zalo_group_pairing_code_open_by_commune
    ON zalo_group_pairing_code (tenant_id, created_at DESC)
    WHERE used_at IS NULL AND cancelled_at IS NULL;

DROP TRIGGER IF EXISTS zalo_group_pairing_code_guard ON zalo_group_pairing_code;
CREATE TRIGGER zalo_group_pairing_code_guard
    BEFORE UPDATE OR DELETE ON zalo_group_pairing_code
    FOR EACH ROW EXECUTE FUNCTION zalo_group_pairing_code_guard();

-- ===========================================================================
-- zalo_group_link
-- ===========================================================================

-- ---------------------------------------------------------------------------
-- zalo_group_link_guard
--
--   DELETE             refused. The row is the record of where a commune's notices went.
--   an ended link      (unlinked_at set) is history: frozen whole, rule 7 forbidden #5.
--   identity columns   commune, bot, chat, the code that made it, who linked and when: fixed. Re-pointing
--                      a link at another chat sends a commune's notices to another group's members.
--   scope, kinds       editable while live (updated_at); every edit is an audit entry (Go).
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION zalo_group_link_guard() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION '%: delete refused', TG_TABLE_NAME
            USING HINT = 'End a group link with unlinked_at, unlinked_by, unlink_reason instead.';
    END IF;
    IF OLD.unlinked_at IS NOT NULL THEN
        RAISE EXCEPTION '%: an ended group link is not edited', TG_TABLE_NAME
            USING HINT = 'An ended link is a historical record (rule 7, forbidden #5). Link again.';
    END IF;
    IF NEW.tenant_id       IS DISTINCT FROM OLD.tenant_id
    OR NEW.id              IS DISTINCT FROM OLD.id
    OR NEW.bot_ref         IS DISTINCT FROM OLD.bot_ref
    OR NEW.chat_id         IS DISTINCT FROM OLD.chat_id
    OR NEW.pairing_code_id IS DISTINCT FROM OLD.pairing_code_id
    OR NEW.linked_at       IS DISTINCT FROM OLD.linked_at
    OR NEW.linked_by       IS DISTINCT FROM OLD.linked_by
    OR NEW.created_at      IS DISTINCT FROM OLD.created_at THEN
        RAISE EXCEPTION '%: a group link is immutable', TG_TABLE_NAME
            USING HINT = 'Which bot, which chat, which code, who and since when are fixed. End it and link again.';
    END IF;
    RETURN NEW;
END $$;

-- ---------------------------------------------------------------------------
-- @entity: ZaloGroupLink
-- @scope:  tenant
--
-- zalo_group_link — one Zalo GROUP chat of one bot <-> one commune, with a scope. Rows are owned by a
-- commune (read and written through the scoped store, rule 1 inv 5), but ONE invariant is platform-wide:
-- a group chat is live in ONE commune at a time (ADR 0092 #9 — the second commune is refused until the
-- first unlinks, never moved silently). Hence not partitioned (header).
--
-- SOFT DELETE IS SPELLED `unlinked_*`, as on zalo_link (0018:348-354) and for its reason: a link ends, the
-- ended row stays as history, and live-only uniqueness is the business rule. A link issues no code.
--
-- chat_id IS PERSONAL DATA (ADR 0092 §Hệ quả). Stored raw because sending needs it; NEVER logged, never
-- returned to a client, never exported, never in a cache key (rule 3).
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS zalo_group_link (
    tenant_id        TEXT        NOT NULL,
    id               TEXT        NOT NULL,              -- ULID, internal; zalo_group_delivery points here
    -- WHICH BOT, as zalo_link.bot_ref (0022 zalo_link_bot_ref_shape): 'shared', or a commune bot's
    -- 'commune:<bot_account_id>'. NO DEFAULT (0022:614-617). WHICH bot is allowed is
    -- zalo_link_commune_bot_live's decision, attached below.
    bot_ref          TEXT        NOT NULL,
    chat_id          TEXT        NOT NULL,
    -- The identity `bo_phan` whose records this group follows, or NULL = the whole commune. A notice
    -- reaches a unit group only when its holding_org_unit_id equals this (comms.proto field 8); empty is
    -- never "every unit". NO FOREIGN KEY (rule 2).
    org_unit_id      TEXT,
    kinds            TEXT[]      NOT NULL,
    -- The code that made this link: its scope and kinds were copied from it, its issuer is linked_by.
    pairing_code_id  TEXT        NOT NULL,
    linked_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- The code ISSUER's business code (`CB-…`, rule 6 inv 8) — not whoever typed the code in the group.
    linked_by        TEXT        NOT NULL,
    unlinked_at      TIMESTAMPTZ,
    -- The staff code of whoever unlinked it in web-admin (ADR 0092 #8), or the staff code that switched
    -- the commune's bot (ADR 0079 Q1 #4), or the operator code (`VH-…`) when the shared bot's account
    -- changed (0018:708-712).
    unlinked_by      TEXT,
    unlink_reason    TEXT,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, id),
    -- ONE CODE, ONE LINK. Composite with tenant_id (rule 1 inv 6). Not partial: a used code stays used.
    UNIQUE (tenant_id, pairing_code_id),
    FOREIGN KEY (tenant_id, pairing_code_id) REFERENCES zalo_group_pairing_code (tenant_id, id),
    CONSTRAINT zalo_group_link_bot_ref_shape
        CHECK (bot_ref = 'shared' OR bot_ref ~ '^commune:[!-~]{1,128}$'),
    CONSTRAINT zalo_group_link_chat_id_length CHECK (char_length(chat_id) BETWEEN 1 AND 128),
    CONSTRAINT zalo_group_link_org_unit_shape
        CHECK (org_unit_id IS NULL OR org_unit_id ~ '^[0-9A-HJKMNP-TV-Z]{26}$'),
    -- An empty set is a group that receives nothing yet looks linked. End the link instead.
    CONSTRAINT zalo_group_link_kinds_present CHECK (cardinality(kinds) > 0),
    CONSTRAINT zalo_group_link_kinds_known
        CHECK (kinds <@ ARRAY['nhiem-vu.qua-han', 'nhiem-vu.chua-cu-nguoi', 'nhiem-vu.leo-thang',
                              'van-ban.qua-han', 'van-ban.chua-cu-nguoi', 'van-ban.leo-thang',
                              'phan-anh.qua-han', 'phan-anh.chua-cu-nguoi', 'phan-anh.leo-thang',
                              'nhiem-vu.giao-moi', 'nhiem-vu.cho-duyet', 'van-ban.chuyen-toi',
                              'phan-anh.phan-cong', 'phan-anh.mo-lai']::text[]),
    CONSTRAINT zalo_group_link_linked_by_signed
        CHECK (btrim(linked_by) <> '' AND char_length(linked_by) <= 64),
    -- All three, or none (the shape rule 7 invariant 1 asks of a soft delete).
    CONSTRAINT zalo_group_link_unlink_complete
        CHECK ((unlinked_at IS NULL) = (unlinked_by IS NULL)
               AND (unlinked_at IS NULL) = (unlink_reason IS NULL)),
    CONSTRAINT zalo_group_link_unlink_signed
        CHECK (unlinked_by IS NULL OR (btrim(unlinked_by) <> '' AND char_length(unlinked_by) <= 64)),
    CONSTRAINT zalo_group_link_unlink_reason_length
        CHECK (unlink_reason IS NULL OR (btrim(unlink_reason) <> '' AND char_length(unlink_reason) <= 200)),
    CONSTRAINT zalo_group_link_unlinked_after_linked
        CHECK (unlinked_at IS NULL OR unlinked_at >= linked_at)
);

-- THE ENQUEUE'S READ (transaction boundary gui_tin_nhom_zalo): a commune's live groups, by unit. NULL
-- org_unit_id (commune-wide) is indexed too.
CREATE INDEX IF NOT EXISTS zalo_group_link_live_by_unit
    ON zalo_group_link (tenant_id, org_unit_id)
    WHERE unlinked_at IS NULL;

-- @scope: cross-tenant — ONE LIVE LINK PER GROUP CHAT ACROSS THE PLATFORM (ADR 0092 #9). A group linked in
-- commune A cannot be linked by commune B's code until A unlinks; and the webhook's lookup (bot, chat) ->
-- (commune, link) needs one answer. OWED BY GO: that read carries `// @cross-tenant:`; a code typed in a
-- group live in ANOTHER commune is refused, never ends the other commune's link.
CREATE UNIQUE INDEX IF NOT EXISTS zalo_group_link_one_live_per_chat
    ON zalo_group_link (bot_ref, chat_id)
    WHERE unlinked_at IS NULL;

DROP TRIGGER IF EXISTS zalo_group_link_guard ON zalo_group_link;
CREATE TRIGGER zalo_group_link_guard
    BEFORE UPDATE OR DELETE ON zalo_group_link
    FOR EACH ROW EXECUTE FUNCTION zalo_group_link_guard();

-- WHICH BOT a new group link may name: 0022's function, unchanged, on the second link table — a 'shared'
-- link only while the commune has no live own bot; a 'commune:…' link only through the commune's live own
-- bot. Same per-commune lock, so a group linking and a bot switch serialise. One function, two triggers:
-- one rule, one source.
DROP TRIGGER IF EXISTS zalo_group_link_commune_bot_live ON zalo_group_link;
CREATE TRIGGER zalo_group_link_commune_bot_live
    BEFORE INSERT ON zalo_group_link
    FOR EACH ROW EXECUTE FUNCTION zalo_link_commune_bot_live();

-- ---------------------------------------------------------------------------
-- zalo_chat_one_link_kind — a (bot_ref, chat_id) is live as a PERSONAL link or as a GROUP link, never as
-- both. Attached BEFORE INSERT to both tables (bot_ref and chat_id are immutable on each, and an ended row
-- cannot be revived — the guards), so an INSERT is the only way in.
--
-- Under a per-chat transaction lock taken by BOTH triggers, so two inserts of the same chat into the two
-- tables cannot both pass. The lock key is a hash of the chat — inside the database, never logged; the
-- error names the table, never the chat (rule 3). Reads across communes BY DESIGN, as the platform-wide
-- chat keys do; it answers yes/no and returns nothing about the other row.
--
-- Trigger names sort BEFORE the *_commune_bot_live triggers (PostgreSQL fires same-event triggers in name
-- order), so every insert takes the chat lock, then the commune lock — one order, no deadlock between them.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION zalo_chat_one_link_kind() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    PERFORM pg_advisory_xact_lock(hashtextextended('zalo-chat:' || NEW.bot_ref || ':' || NEW.chat_id, 0));
    IF TG_TABLE_NAME = 'zalo_link' THEN
        IF EXISTS (SELECT 1 FROM zalo_group_link
                   WHERE bot_ref = NEW.bot_ref AND chat_id = NEW.chat_id AND unlinked_at IS NULL) THEN
            RAISE EXCEPTION '%: this chat is a linked group; it is not paired to a person', TG_TABLE_NAME
                USING HINT = 'A group chat is linked with a group code from web-admin (ADR 0092 #3, #4).';
        END IF;
    ELSE
        IF EXISTS (SELECT 1 FROM zalo_link
                   WHERE bot_ref = NEW.bot_ref AND chat_id = NEW.chat_id AND unlinked_at IS NULL) THEN
            RAISE EXCEPTION '%: this chat is paired to a person; it is not linked as a group', TG_TABLE_NAME
                USING HINT = 'A personal chat receives one person''s reminders only (ADR 0092 §Bối cảnh).';
        END IF;
    END IF;
    RETURN NEW;
END $$;

DROP TRIGGER IF EXISTS zalo_link_chat_not_group ON zalo_link;
CREATE TRIGGER zalo_link_chat_not_group
    BEFORE INSERT ON zalo_link
    FOR EACH ROW EXECUTE FUNCTION zalo_chat_one_link_kind();

DROP TRIGGER IF EXISTS zalo_group_link_chat_not_personal ON zalo_group_link;
CREATE TRIGGER zalo_group_link_chat_not_personal
    BEFORE INSERT ON zalo_group_link
    FOR EACH ROW EXECUTE FUNCTION zalo_chat_one_link_kind();

-- ===========================================================================
-- THE BOT SWITCH — the three checks of 0018/0022, now also over zalo_group_link.
-- Each body below is the original VERBATIM plus ONE EXISTS on zalo_group_link with the same predicate.
-- ===========================================================================

-- 0018:801-814. A different shared-bot account: no live link of the shared bot, personal OR GROUP, linked
-- before this save may survive the commit.
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
    IF NEW.bot_account_id IS DISTINCT FROM OLD.bot_account_id
       AND EXISTS (SELECT 1 FROM zalo_group_link
                   WHERE bot_ref = NEW.bot_ref
                     AND unlinked_at IS NULL
                     AND linked_at < NEW.set_at) THEN
        RAISE EXCEPTION '%: the bot account changed but group links to the old bot are still live', TG_TABLE_NAME
            USING HINT = 'End every live zalo_group_link of this bot (unlinked_by = the operator code) '
                         'in the same transaction as the account change.';
    END IF;
    RETURN NULL;
END $$;

-- 0022:542-555. Shared -> own: no live 'shared' link, personal OR GROUP, in the commune at commit.
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
    IF EXISTS (SELECT 1 FROM zalo_commune_bot
               WHERE tenant_id = NEW.tenant_id AND id = NEW.id AND retired_at IS NULL)
       AND EXISTS (SELECT 1 FROM zalo_group_link
                   WHERE tenant_id = NEW.tenant_id AND bot_ref = 'shared' AND unlinked_at IS NULL) THEN
        RAISE EXCEPTION '%: an own bot was adopted but shared-bot group links are still live', TG_TABLE_NAME
            USING HINT = 'End every live shared zalo_group_link of this commune (unlinked_by = the staff code) '
                         'in the same transaction — groups are linked again through the own bot.';
    END IF;
    RETURN NULL;
END $$;

-- 0022:574-589. A bot retired in this transaction: no live link of it, personal OR GROUP, in its commune.
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
        IF EXISTS (SELECT 1 FROM zalo_group_link
                   WHERE tenant_id = NEW.tenant_id
                     AND bot_ref = NEW.bot_ref
                     AND unlinked_at IS NULL) THEN
            RAISE EXCEPTION '%: the bot was retired but its group links are still live', TG_TABLE_NAME
                USING HINT = 'End every live zalo_group_link of this bot (unlinked_by = the staff code) in '
                             'the same transaction as the retirement — groups are linked again.';
        END IF;
    END IF;
    RETURN NULL;
END $$;

-- The three triggers that call these (0018 zalo_bot_shared_account_change_check, 0022
-- zalo_commune_bot_adopt_check / zalo_commune_bot_retire_check) are unchanged: each names its function,
-- and the function is replaced in place. Still DEFERRABLE INITIALLY DEFERRED.

-- ===========================================================================
-- zalo_group_delivery
-- ===========================================================================

-- ---------------------------------------------------------------------------
-- zalo_group_delivery_guard — 0024's zalo_delivery_guard, for the group outbox.
--
--   DELETE                refused (rule 7 inv 1). A hard delete would also free the dedupe key and let a
--                         retry post the same notice to the group twice.
--   a soft-deleted row    a historical record; not edited.
--   identity columns      which group, which key, which kind, WHAT WAS POSTED, when queued: fixed.
--   a terminal row        ('da-gui', 'bo-qua', 'that-bai') keeps its outcome.
--   attempts              never decreases.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION zalo_group_delivery_guard() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION '%: hard delete refused', TG_TABLE_NAME
            USING HINT = 'Soft delete only (deleted_at, deleted_by, delete_reason) — rule 7, '
                         'invariant 1. A hard delete would also let a retry post it again.';
    END IF;
    IF OLD.deleted_at IS NOT NULL THEN
        RAISE EXCEPTION '%: a deleted group delivery is not edited', TG_TABLE_NAME
            USING HINT = 'A soft-deleted row is a historical record (rule 7, forbidden #5).';
    END IF;
    IF NEW.tenant_id       IS DISTINCT FROM OLD.tenant_id
    OR NEW.id              IS DISTINCT FROM OLD.id
    OR NEW.group_link_id   IS DISTINCT FROM OLD.group_link_id
    OR NEW.idempotency_key IS DISTINCT FROM OLD.idempotency_key
    OR NEW.kind            IS DISTINCT FROM OLD.kind
    OR NEW.title           IS DISTINCT FROM OLD.title
    OR NEW.body            IS DISTINCT FROM OLD.body
    OR NEW.link            IS DISTINCT FROM OLD.link
    OR NEW.created_at      IS DISTINCT FROM OLD.created_at THEN
        RAISE EXCEPTION '%: a queued group delivery is immutable', TG_TABLE_NAME
            USING HINT = 'Which group, which notice, of which kind, what text and when queued are fixed.';
    END IF;
    IF OLD.status <> 'cho-gui'
       AND (NEW.status          IS DISTINCT FROM OLD.status
         OR NEW.skip_reason     IS DISTINCT FROM OLD.skip_reason
         OR NEW.attempts        IS DISTINCT FROM OLD.attempts
         OR NEW.next_attempt_at IS DISTINCT FROM OLD.next_attempt_at
         OR NEW.sent_at         IS DISTINCT FROM OLD.sent_at
         OR NEW.error_class     IS DISTINCT FROM OLD.error_class) THEN
        RAISE EXCEPTION '%: a finished group delivery keeps its outcome', TG_TABLE_NAME
            USING HINT = 'Sent, skipped and failed are final. Moving a sent row back would post it '
                         'a second time.';
    END IF;
    IF NEW.attempts < OLD.attempts THEN
        RAISE EXCEPTION '%: attempts cannot decrease', TG_TABLE_NAME
            USING HINT = 'The attempt count is evidence of how hard the channel was tried.';
    END IF;
    RETURN NEW;
END $$;

-- ---------------------------------------------------------------------------
-- @entity: ZaloGroupDelivery
-- @scope:  tenant
--
-- zalo_group_delivery — one Zalo message owed, or posted, to one linked group. Queued IN the transaction
-- that writes the bell rows (DeliverStaffNotifications), under its own SAVEPOINT as zalo_delivery's enqueue
-- (transaction boundary gui_tin_nhom_zalo): only for group_posting = GROUP_POSTING_ALLOWED, only to live
-- groups whose kinds hold the notice's kind and whose scope is the whole commune or the notice's
-- holding_org_unit_id. The dispatcher re-checks the link is live when it sends; ended = 'bo-qua'
-- 'chua-lien-ket', never posted.
--
-- A Zalo refusal (bot removed from the group, group dissolved) is 'that-bai' with its class — it does NOT
-- end the link: unlinking is a staff act on ViGov, with a trail (ADR 0092 #8).
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS zalo_group_delivery (
    tenant_id        TEXT        NOT NULL,
    id               TEXT        NOT NULL,            -- ULID, internal
    group_link_id    TEXT        NOT NULL,
    -- The notice's idempotency_key (comms.proto StaffNotification field 1), as staff_notification stores it.
    idempotency_key  TEXT        NOT NULL,
    -- staff_notification.kind's stored value, from the group list only.
    kind             TEXT        NOT NULL,
    -- WHAT WAS POSTED — the notice's title / body / link, the same for every recipient (GROUP_POSTING_ALLOWED).
    -- Bounded exactly as 0010's bell row. See VENDOR CHOICES for why this one copy exists.
    title            TEXT        NOT NULL,
    body             TEXT        NOT NULL DEFAULT '',
    link             TEXT        NOT NULL DEFAULT '',
    status           TEXT        NOT NULL DEFAULT 'cho-gui',
    skip_reason      TEXT,
    attempts         SMALLINT    NOT NULL DEFAULT 0,
    -- When the sender may try next: now, after a back-off, or the end of the commune's quiet hours (the
    -- commune's own setting, ADR 0092 §Hệ quả — no group-specific hours).
    next_attempt_at  TIMESTAMPTZ DEFAULT now(),
    sent_at          TIMESTAMPTZ,
    -- A CLASS, never Zalo's text or a Go error string: the bot token sits in the URL (0018:502-503).
    error_class      TEXT,
    deleted_at       TIMESTAMPTZ,
    deleted_by       TEXT,
    delete_reason    TEXT,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, id),
    -- ONE MESSAGE PER (NOTICE KEY, GROUP) — ADR 0092 #5, first delivery wins (rule 2 inv 5). Composite with
    -- tenant_id; NOT partial, so a retired row still blocks a re-post.
    UNIQUE (tenant_id, group_link_id, idempotency_key),
    FOREIGN KEY (tenant_id, group_link_id) REFERENCES zalo_group_link (tenant_id, id),
    CONSTRAINT zalo_group_delivery_kind_known
        CHECK (kind IN ('nhiem-vu.qua-han', 'nhiem-vu.chua-cu-nguoi', 'nhiem-vu.leo-thang',
                        'van-ban.qua-han', 'van-ban.chua-cu-nguoi', 'van-ban.leo-thang',
                        'phan-anh.qua-han', 'phan-anh.chua-cu-nguoi', 'phan-anh.leo-thang',
                        'nhiem-vu.giao-moi', 'nhiem-vu.cho-duyet', 'van-ban.chuyen-toi',
                        'phan-anh.phan-cong', 'phan-anh.mo-lai')),
    CONSTRAINT zalo_group_delivery_idempotency_key_length
        CHECK (char_length(idempotency_key) BETWEEN 1 AND 200),
    CONSTRAINT zalo_group_delivery_title_length
        CHECK (btrim(title) <> '' AND char_length(title) <= 200),
    CONSTRAINT zalo_group_delivery_body_length
        CHECK (char_length(body) <= 500),
    CONSTRAINT zalo_group_delivery_link_relative
        CHECK (link = '' OR (char_length(link) <= 300 AND left(link, 1) = '/'
                             AND left(link, 2) <> '//' AND position('\' in link) = 0)),
    -- 0018's four states.
    CONSTRAINT zalo_group_delivery_status_known
        CHECK (status IN ('cho-gui', 'da-gui', 'bo-qua', 'that-bai')),
    -- 0018's four reasons, read for a group:
    --   chua-lien-ket      the group link ended before the send
    --   kenh-tat           the commune's channel is off, or has no settings row
    --   loai-tat           the commune switched this kind off since the notice was queued
    --   bot-chua-cau-hinh  the link's bot has no token
    -- NOT 0024's 'ngoai-nhac-truoc': due-soon digests never reach a group.
    CONSTRAINT zalo_group_delivery_skip_reason_known
        CHECK (skip_reason IS NULL
               OR skip_reason IN ('chua-lien-ket', 'kenh-tat', 'loai-tat', 'bot-chua-cau-hinh')),
    CONSTRAINT zalo_group_delivery_skip_has_reason
        CHECK ((status = 'bo-qua') = (skip_reason IS NOT NULL)),
    -- 0018's ZaloBotCallOutcome classes, one vocabulary for one Zalo call.
    CONSTRAINT zalo_group_delivery_error_class_known
        CHECK (error_class IS NULL
               OR error_class IN ('token-bi-tu-choi', 'gioi-han-tan-suat', 'khong-kha-dung',
                                  'bi-tu-choi', 'phan-hoi-sai-dang')),
    CONSTRAINT zalo_group_delivery_failure_has_class
        CHECK (status <> 'that-bai' OR error_class IS NOT NULL),
    CONSTRAINT zalo_group_delivery_sent_has_time
        CHECK ((status = 'da-gui') = (sent_at IS NOT NULL)),
    CONSTRAINT zalo_group_delivery_pending_is_scheduled
        CHECK ((status = 'cho-gui') = (next_attempt_at IS NOT NULL)),
    CONSTRAINT zalo_group_delivery_attempts_non_negative CHECK (attempts >= 0),
    CONSTRAINT zalo_group_delivery_soft_delete_complete
        CHECK ((deleted_at IS NULL) = (deleted_by IS NULL)
               AND (deleted_at IS NULL) = (delete_reason IS NULL))
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..7 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS zalo_group_delivery_p%s PARTITION OF zalo_group_delivery '
            'FOR VALUES WITH (MODULUS 8, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

-- THE SENDER'S QUEUE, per commune (rule 1 inv 9): what is owed and due. Finished rows leave the index.
CREATE INDEX IF NOT EXISTS zalo_group_delivery_due
    ON zalo_group_delivery (tenant_id, next_attempt_at)
    WHERE status = 'cho-gui' AND deleted_at IS NULL;

-- One group's history, newest first.
CREATE INDEX IF NOT EXISTS zalo_group_delivery_by_link
    ON zalo_group_delivery (tenant_id, group_link_id, created_at DESC, id DESC)
    WHERE deleted_at IS NULL;

DROP TRIGGER IF EXISTS zalo_group_delivery_guard ON zalo_group_delivery;
CREATE TRIGGER zalo_group_delivery_guard
    BEFORE UPDATE OR DELETE ON zalo_group_delivery
    FOR EACH ROW EXECUTE FUNCTION zalo_group_delivery_guard();

-- ---------------------------------------------------------------------------
-- WHAT THIS FILE DOES NOT STOP: TRUNCATE and DDL by the table owner — the line 0003 and ADR 0013 draw.
-- Nor that a group's kinds are a subset of the kinds its commune switched on (ADR 0092 #2): the commune can
-- untick a kind later, so the floor would refuse the commune's own settings save. Go checks it at issue and
-- at save, and the dispatcher skips 'loai-tat' at send.
--
-- REVERSAL (question 3). Run by a person, in ONE transaction (ADR 0013). Prose, because a runnable line is a
-- line that gets run.
--
-- LOSSLESS ONLY WHILE the three new tables are EMPTY. Then: restore the three functions
-- zalo_bot_shared_account_change_check (0018:801-814), zalo_commune_bot_adopt_check (0022:542-555) and
-- zalo_commune_bot_retire_check (0022:574-589) VERBATIM from their files; drop the trigger
-- zalo_link_chat_not_group on zalo_link; drop the tables zalo_group_delivery, zalo_group_link,
-- zalo_group_pairing_code in that order (partitions, indexes and triggers go with them); drop the
-- functions zalo_group_delivery_guard, zalo_group_link_guard, zalo_group_pairing_code_guard,
-- zalo_chat_one_link_kind (zalo_link_commune_bot_live is 0022's and stays); remove this file's row from
-- `schema_migration` (ten = '0029_zalo_group_link.sql').
--
-- ONCE ANY ROW EXISTS it is no longer a reversal: zalo_group_link and zalo_group_delivery are the record of
-- which group was told what, and zalo_group_link holds personal data under retention. Rule 7 stop
-- conditions #1/#2 — the user, a verified backup, and a NEW migration.
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
                         'MODULUS 8 partition loop (ADR 0096) in the same migration that '
                         'declares PARTITION BY.';
    END IF;
END $$;
