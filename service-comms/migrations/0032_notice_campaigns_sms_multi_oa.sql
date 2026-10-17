-- 0032 — NOTICE CAMPAIGNS OVER ZNS / SMS: several Zalo OAs per commune, several eSMS accounts per
-- commune, and the ledger of each campaign and each of its recipients (ADR 0099, owner 17/10/2026,
-- "Gửi tin ZNS / SMS", block D).
--
--   (A) zalo_commune_oa     one-live-OA-per-commune key REMOVED; the platform-wide floor on a live
--                           oa_id and one-template-per-milestone are KEPT (ADR 0099 #1)
--   sms_account             tenant   plain   an eSMS account of the commune: ApiKey + SecretKey sealed,
--                                            Brandname, one default per commune (ADR 0099 #2)
--   sms_template            tenant   plain   the CSKH templates eSMS returns for an account (GetTemplate)
--   notice_campaign         tenant   HASH 8  one campaign: who sent what, through which OA / account,
--                                            to which audience, and how it went (ADR 0099 #7, #8)
--   notice_recipient        tenant   HASH 8  one recipient of one campaign: directory row code + the
--                                            MASKED number, never the full number (ADR 0099 #5)
--
-- PARTITIONING (ADR 0096 #1, #3): sms_account and sms_template are configuration — a handful of rows
-- per commune, not growing with time — so PLAIN. notice_campaign grows with every campaign and
-- notice_recipient with every campaign × every citizen of the commune, so both are PARTITION BY HASH
-- (tenant_id), MODULUS 8, leaves created in this same file (ADR 0013 #4).
--
-- NAMED IN ENGLISH (rule 12, ADR 0051): SmsAccount / sms_account, SmsTemplate / sms_template,
-- NoticeCampaign / notice_campaign, NoticeRecipient / notice_recipient are the names the owner fixed on
-- 17/10/2026 (ADR 0099 owns_facts; ubiquitous-language.md). Enum values of OUR OWN are Vietnamese
-- without diacritics, kebab-case (ADR 0011) — with the exception stated under VENDOR CHOICES.
--
-- WHY A NEW FILE AND NOT AN EDIT TO 0031: core/migrate checksums every applied file at startup and
-- stops the service when one has changed (ErrChecksumLech). ADR 0096's one-off "edit the applied file"
-- exception was for that ADR's batch only.
--
-- ---------------------------------------------------------------------------
-- WHAT IS DECIDED, and where each lands:
--
--   ADR 0099 #1   many OAs per commune; one OA never live in two   DROP INDEX ..._one_live_per_commune;
--                 communes; one template per milestone              ..._one_live_per_oa and
--                                                                   zalo_oa_template_one_per_milestone
--                                                                   untouched
--   ADR 0099 #2   many eSMS accounts; keys sealed under the         sms_account: *_sealed, brandname,
--                 commune's DEK; one default; "ngừng dùng" is a     is_default + its key, retired_*
--                 soft delete; check = get balance                   (soft delete), last_check_*,
--                                                                   last_balance
--   ADR 0099 #2   templates loaded with GetTemplate                 sms_template
--   ADR 0099 #5   the ledger holds directory row code + MASKED      notice_recipient.recipient_ref,
--                 number, never the full number                     phone_masked + its CHECKs
--   ADR 0099 #7   estimated cost shown, never a gate                unit_price / estimated_cost, both
--                                                                   NULLable (an estimate that could not
--                                                                   be had does not stop a send)
--   ADR 0099 #8   campaign + every recipient row ('cho-gui') in     notice_campaign / notice_recipient;
--                 ONE transaction; background sender per commune;   lease_until for the sender; status
--                 RequestId / tracking_id = row id; recipient       vocabularies as the ADR spells them
--                 da-gui / that-bai / bo-qua; campaign dang-gui ->
--                 da-gui / gui-mot-phan / that-bai
--   ADR 0099 #9   "đã gửi" = the provider ACCEPTED it               provider_message_id on 'da-gui'; no
--                                                                   delivered-to-handset column yet
--   rule 6 inv 8  who configured / who sent: STAFF business code    set_by, retired_by, created_by
--
-- ---------------------------------------------------------------------------
-- VENDOR CHOICES, stated so a reviewer can overrule them:
--
--   * NO `label` COLUMN ON zalo_commune_oa. The OA picker shows `oa_name`, which Zalo returns from
--     `getoa` at connection and at every "Kiểm tra" (0031) — the name the citizen sees on the message.
--     Two OAs of one commune are two different OAs at Zalo and carry two different names there; a
--     second, commune-typed name would be a second name for one sender that drifts from the first.
--     Add it in a later file if the owner asks for one.
--   * sms_account's SOFT DELETE IS SPELLED retired_at / retired_by / retire_reason, AS 0022 AND 0031
--     SPELL IT, NOT deleted_*. The trio IS rule 7 invariant 1's soft delete ("ngừng dùng"). The
--     spelling matters for one reason: the label key must be partial on "not retired" so a commune can
--     retire "eSMS chính" and add a new "eSMS chính", and tools/check_khoa_duy_nhat.py refuses a unique
--     key partial on deleted_at — because on a table that ISSUES CODES that would reissue a code (rule 7
--     invariant 3). An account label is not an issued code; renaming the column says so instead of
--     weakening the gate. A retired account row is frozen whole, as 0031's retired OA.
--   * ONE DEFAULT ACCOUNT PER COMMUNE IS `UNIQUE (tenant_id) WHERE is_default`, plus a CHECK that a
--     retired row is never the default. So retiring the default must clear is_default in the same
--     UPDATE (Go), and the key does not need "AND not retired" — the CHECK carries it.
--   * sms_template IS ONE ROW PER (eSMS TempId, NetworkID), NOT ONE ROW PER CONTENT. GetTemplate returns
--     `BrandnameTemplates[{NetworkID, TempContent, TempId}]`: the same text is registered once per
--     network, each with its own TempId. Storing exactly what eSMS returns keeps the sync a plain upsert
--     on eSMS's own identity, and "vanished" is decided per TempId — the unit eSMS adds and removes. A
--     row per content would need a unique key on long free text (or a hash of it) and an array of
--     (network, TempId) pairs that can drift out of step. The cost: the picker groups rows by content
--     (Go) — and SENDING NEEDS NO TempId AT ALL: SendMultipleMessage_V4 takes `Content`, and eSMS
--     matches it to the recipient's network's registered template. The key includes network_id so a
--     TempId eSMS reuses across networks is still two rows.
--   * notice_campaign.template_content IS A COPY OF THE SMS TEXT AT SEND TIME. sms_template.content is
--     rewritten by every sync; the campaign is the archival record of what was said (rule 7 forbidden
--     #5), so it keeps its own copy of the template text — the text with `{P1,20}` placeholders, plus
--     `params` (the values staff typed for the whole campaign). Neither is personal data (ADR 0099 #6:
--     no parameter comes from a recipient's record). For ZNS the column is NULL: Zalo's template is
--     identified by its template_id and 0031 stores no ZNS text either — the 0004 ledger's shape.
--   * failure_class IS A SHAPE CHECK, NOT A CLOSED LIST — for now. The eSMS adapter (`internal/esms`)
--     and its error classes do not exist yet; a list invented here would be a second source the adapter
--     then has to match. The CHECK admits only lower-case letters and hyphens (`sai-khoa`,
--     `khong-kha-dung`), so an error STRING, a provider message or a phone number cannot be stored —
--     which is what matters for rule 3. OWED BY THE GO CARD: close it with the adapter's list in a later
--     migration, as 0018 did for zalo_delivery.
--   * notice_recipient status ADDS 'bo-qua' to the card's three. ADR 0099 #8 lists the outcomes as
--     da-gui / that-bai / bo-qua; a row skipped at send time (e.g. the directory row turned inactive
--     between the campaign and the send) must be countable and must not sit in the retry set.
--   * audience_kind AND recipient_kind ARE ADR 0011 VALUES — Vietnamese without diacritics, kebab-case
--     ('nguoi-dan-tat-ca', 'nguoi-dan-theo-nhom', 'nguoi-dan-theo-nhan', 'can-bo-tat-ca',
--     'can-bo-theo-bo-phan'; 'nguoi-dan' / 'can-bo'), decided by the main session 17/10/2026 while the
--     table is empty. The identity RPC's proto enum (NoticeAudienceKind, NoticeRecipientKind) is CODE and
--     stays English; comms maps one to the other in its domain package — one table of six lines.
--   * NO SOFT DELETE ON notice_campaign / notice_recipient / sms_template. A sent campaign is an
--     archival record of what a public authority told whom; there is no act that "deletes" it, and DELETE
--     is refused. sms_template follows 0031's zalo_oa_template: a template that vanished at eSMS is
--     `vanished_at`, the row stays (campaigns name it).
--   * FOREIGN KEYS BETWEEN THE TWO PARTITIONED TABLES ARE KEPT. notice_recipient (partitioned) ->
--     notice_campaign (partitioned) is valid from PostgreSQL 12 (a foreign key REFERENCING a partitioned
--     table); the referenced key (tenant_id, id) is the parent's primary key and contains the partition
--     key, as PostgreSQL requires. The same shape already runs here: 0005 thong_bao_nguoi_nhan ->
--     thong_bao, 0018 zalo_delivery -> staff_notification. notice_campaign -> zalo_commune_oa /
--     sms_account (partitioned -> plain) is valid from PostgreSQL 11. The BEFORE ROW triggers on the two
--     partitioned parents need 13 — 0004 already refuses an older server. PG FLOOR: 13. Cluster: 16.
--
-- THE SEALED COLUMNS ARE UNDER THE COMMUNE'S DEK (data_encryption_key, 0008), 0022 / 0031's discipline.
-- AAD OWED BY GO: additional data naming the SECRET, the COMMUNE and the ROW:
--     ApiKey      "sms_account/api_key/<tenant_id>/<id>"
--     SecretKey   "sms_account/secret_key/<tenant_id>/<id>"
-- Bound to the row id, so bytes copied from a retired account onto a live one do not open; to the
-- tenant, so they do not open in another commune; to the secret name, so an ApiKey does not open as a
-- SecretKey. Both are WRITE-ONLY: no route returns them, the audit trail never carries a *_sealed
-- column, and nothing logs a request body to eSMS (it carries both keys in clear — ADR 0099 table).
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. ROWS PER COMMUNE: NONE WRITTEN — no seed, no default account, no template, no campaign. One
--      index removed (A) and one non-unique index added in its place; nothing else existing is touched.
--      Steady state: a few sms_account rows, tens of sms_template rows per account, one notice_campaign
--      row per campaign, and per campaign one notice_recipient row per reachable recipient — up to the
--      commune's whole citizen directory (thousands) per "Toàn bộ người dân" campaign. That last figure
--      is why notice_recipient is partitioned.
--   2. IF IT STOPS HALF-WAY: cannot — one file, one transaction. Every statement is IF EXISTS / IF NOT
--      EXISTS / CREATE OR REPLACE / DROP TRIGGER IF EXISTS, so a retry costs nothing. No backfill, nothing
--      to resume per commune.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom. (A) is NOT freely reversible once any commune has
--      two live OAs.
--   4. READ PATHS THAT CHANGE MEANING:
--        * (A) "THE LIVE OA OF THE COMMUNE" STOPS BEING ONE ROW. service-comms/internal/store/
--          zalo_commune_oa.go reads `WHERE tenant_id = … AND retired_at IS NULL` and takes the FIRST row
--          (:86, :106, :118 — the screen, the sealed read, the FOR UPDATE read). Today nothing can create
--          a second live row: the connect flow retires the live row before inserting (0031 header), so
--          applying this file BEFORE the Go change changes no answer. THE DAY GO STOPS RETIRING ON
--          CONNECT, those three reads must become reads BY ROW ID first, or they return an arbitrary OA —
--          the screen describes one OA and the sender uses another. The cross-commune refresh job
--          (store/crosstenant/zalo_oa.go:69) already iterates every live row and is unaffected.
--          MauTinXa (milestone -> template -> that template's OA row) is unaffected: the OA is reached
--          through the template, never as "the commune's OA".
--        * A LIVE ACCOUNT IS NOT A USABLE ACCOUNT: `retired_at IS NULL` alone admits an account whose last
--          check said 'sai-khoa' or 'brandname-chua-kich-hoat'. The send path reads the result it gets
--          from eSMS, not last_check_result — that column describes the last "Kiểm tra", nothing more.
--        * A VANISHED TEMPLATE (`vanished_at IS NOT NULL`) IS NOT OFFERED for a new campaign, and a
--          template of a RETIRED account is not either: the picker filters both.
--        * A 'cho-gui' RECIPIENT WITH lease_until IN THE PAST IS RECLAIMABLE (its sender died); one with
--          lease_until in the future belongs to a running sender. A reader that treats every 'cho-gui' as
--          free would send twice — the provider's RequestId / tracking_id = row id is the second layer.
--        * A 'dang-gui' CAMPAIGN'S sent_count / failed_count ARE PARTIAL until finished_at is set.
--   5. RETENTION: nothing dropped, retyped, emptied or renumbered; no issued business code exists in
--      these tables. All four new tables refuse DELETE; a retired account and a finished campaign or
--      recipient are frozen; templates are kept through vanishing. The one DROP is an INDEX (A), which
--      holds no data.
--
-- PERSONAL DATA (rule 3): the ONLY personal datum stored here is `notice_recipient.phone_masked` — the
-- MASKED number (core/privacy.MaskPhone: 09****0000), refused by CHECK unless it carries the mask
-- character and at most six digits. The full number lives in identity's directory (ADR 0097) and is held
-- in comms' memory only for the provider call (ADR 0099 #5). `recipient_ref` is an opaque directory /
-- staff row code. No name, no address, no rendered message per person. `params` and `title` are typed by
-- staff for the whole campaign (ADR 0099 #6). NEVER log a full number, an ApiKey / SecretKey, an eSMS
-- request body, or a provider error message — classes only.
-- ---------------------------------------------------------------------------

-- ===========================================================================
-- (A) zalo_commune_oa — several live OAs per commune (ADR 0099 #1)
-- ===========================================================================

-- 0031's "at most one live OA per commune" key goes; replacing an OA no longer has to retire the old one
-- first. KEPT, untouched: zalo_commune_oa_one_live_per_oa (one OA never live in two communes — the
-- floor) and zalo_oa_template_one_per_milestone (each petition milestone still has ONE template in the
-- whole commune, so a citizen notice goes out through the OA of the template attached to it, and no
-- "default OA" is needed).
DROP INDEX IF EXISTS zalo_commune_oa_one_live_per_commune;

-- The same read the dropped key served, without the uniqueness: the commune's live OAs, oldest first,
-- for the OA list and the campaign's OA picker. Leads with tenant_id (rule 1).
CREATE INDEX IF NOT EXISTS zalo_commune_oa_live_by_commune
    ON zalo_commune_oa (tenant_id, created_at)
    WHERE retired_at IS NULL;

-- ===========================================================================
-- sms_account
-- ===========================================================================

-- ---------------------------------------------------------------------------
-- sms_account_guard
--
--   DELETE                    refused: retire with retired_at / retired_by / retire_reason.
--   a retired row             frozen whole — the record of an account campaigns went out through.
--   tenant_id, id, provider,  fixed. Moving a row to another commune moves sealed bytes bound to the
--   created_at                first one; another provider is another account.
--   new keys or brandname     refused unless set_at changed — the screen's "đặt lúc … bởi …" must never
--   without a new set_at      describe the old keys (0022's rule: ONE writer here, a member of staff).
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION sms_account_guard() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION '%: delete refused', TG_TABLE_NAME
            USING HINT = 'Retire the account with retired_at, retired_by, retire_reason instead.';
    END IF;
    IF OLD.retired_at IS NOT NULL THEN
        RAISE EXCEPTION '%: a retired account is not edited', TG_TABLE_NAME
            USING HINT = 'A retired account is a historical record (rule 7, forbidden #5). Add a new row.';
    END IF;
    IF NEW.tenant_id  IS DISTINCT FROM OLD.tenant_id
    OR NEW.id         IS DISTINCT FROM OLD.id
    OR NEW.provider   IS DISTINCT FROM OLD.provider
    OR NEW.created_at IS DISTINCT FROM OLD.created_at THEN
        RAISE EXCEPTION '%: commune, row and provider are immutable', TG_TABLE_NAME
            USING HINT = 'Another provider, or another commune, is a new row.';
    END IF;
    IF (NEW.api_key_sealed IS DISTINCT FROM OLD.api_key_sealed
        OR NEW.secret_key_sealed IS DISTINCT FROM OLD.secret_key_sealed
        OR NEW.brandname IS DISTINCT FROM OLD.brandname)
       AND NEW.set_at IS NOT DISTINCT FROM OLD.set_at THEN
        RAISE EXCEPTION '%: new keys or brandname need a new set_at and set_by', TG_TABLE_NAME
            USING HINT = 'The screen shows when and by whom the credentials were set.';
    END IF;
    RETURN NEW;
END $$;

-- ---------------------------------------------------------------------------
-- @entity: SmsAccount
-- @scope:  tenant
--
-- sms_account — one eSMS account of the commune (ADR 0099 #2): the keys eSMS issued to it, the
-- Brandname it sends under, a name the commune gives it. Chosen per campaign; one may be the default the
-- picker preselects. NO LIVE ROW = THE COMMUNE SENDS NO SMS. "Kiểm tra kết nối" = eSMS's get-balance
-- call; its class and the balance it returned are kept.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS sms_account (
    tenant_id          TEXT        NOT NULL,
    id                 TEXT        NOT NULL,            -- ULID, internal
    -- The SMS provider. One today (ADR 0099: eSMS of ViHAT); a CHECK, not free text, so a second
    -- provider is one line in a new migration and every count by provider stays right.
    provider           TEXT        NOT NULL,
    -- The commune's name for this account, shown in the picker ("eSMS chính", "eSMS dự phòng").
    label              TEXT        NOT NULL,
    -- Sealed under the commune's DEK (0008), each with its own additional data (header). Write-only.
    api_key_sealed     BYTEA       NOT NULL,
    secret_key_sealed  BYTEA       NOT NULL,
    -- The sender name eSMS sends under — registered with the networks, not secret.
    brandname          TEXT        NOT NULL,
    is_default         BOOLEAN     NOT NULL DEFAULT false,
    -- The last "Kiểm tra kết nối" (eSMS get-balance). A CLOSED CLASS, never eSMS's text:
    --   thanh-cong                 eSMS answered with a balance
    --   sai-khoa                   CodeResult 101 — ApiKey / SecretKey refused
    --   brandname-chua-kich-hoat   CodeResult 104 — the Brandname is not active on this account
    --   khong-kha-dung             timeout / 5xx / network
    --   bi-tu-choi                 any other refusal code
    --   phan-hoi-sai-dang          the answer could not be read
    last_check_at      TIMESTAMPTZ,
    last_check_result  TEXT,
    -- The balance that check returned. Not a secret; present only with a 'thanh-cong' check, so a
    -- failed check never leaves an older balance looking current.
    last_balance       NUMERIC(16, 2),
    -- Who set the current keys / brandname and when: the STAFF business code (`CB-…`, rule 6 inv 8).
    set_at             TIMESTAMPTZ NOT NULL,
    set_by             TEXT        NOT NULL,
    -- RETIRED ("ngừng dùng"): this table's soft delete, spelled as 0022 / 0031 (header, VENDOR CHOICES).
    retired_at         TIMESTAMPTZ,
    retired_by         TEXT,
    retire_reason      TEXT,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, id),
    CONSTRAINT sms_account_provider_known CHECK (provider IN ('esms')),
    CONSTRAINT sms_account_label_shape
        CHECK (btrim(label) <> '' AND char_length(label) <= 100),
    -- 1 + 12 + 16 bytes of envelope overhead, plus at least one byte of secret (0008).
    CONSTRAINT sms_account_api_key_sealed_length CHECK (octet_length(api_key_sealed) > 29),
    CONSTRAINT sms_account_secret_key_sealed_length CHECK (octet_length(secret_key_sealed) > 29),
    -- An alphanumeric sender name: letters, digits, and the separators networks accept (space . _ -),
    -- starting with a letter or digit, no surrounding space, at most 32 (the card's bound; the networks'
    -- own limit is 11 for most Brandnames, which eSMS enforces at registration — not repeated here).
    CONSTRAINT sms_account_brandname_shape
        CHECK (brandname ~ '^[A-Za-z0-9][A-Za-z0-9 ._-]{0,31}$' AND brandname = btrim(brandname)),
    CONSTRAINT sms_account_set_by_signed
        CHECK (btrim(set_by) <> '' AND char_length(set_by) <= 64),
    CONSTRAINT sms_account_check_pair
        CHECK ((last_check_at IS NULL) = (last_check_result IS NULL)),
    CONSTRAINT sms_account_check_result_known
        CHECK (last_check_result IS NULL
               OR last_check_result IN ('thanh-cong', 'sai-khoa', 'brandname-chua-kich-hoat',
                                        'khong-kha-dung', 'bi-tu-choi', 'phan-hoi-sai-dang')),
    CONSTRAINT sms_account_balance_from_success
        CHECK (last_balance IS NULL OR last_check_result = 'thanh-cong'),
    -- All three, or none (the shape rule 7 invariant 1 asks of a soft delete).
    CONSTRAINT sms_account_retire_complete
        CHECK ((retired_at IS NULL) = (retired_by IS NULL)
               AND (retired_at IS NULL) = (retire_reason IS NULL)),
    CONSTRAINT sms_account_retire_signed
        CHECK (retired_by IS NULL OR (btrim(retired_by) <> '' AND char_length(retired_by) <= 64)),
    CONSTRAINT sms_account_retire_reason_length
        CHECK (retire_reason IS NULL OR (btrim(retire_reason) <> '' AND char_length(retire_reason) <= 200)),
    CONSTRAINT sms_account_retired_after_created
        CHECK (retired_at IS NULL OR retired_at >= created_at),
    -- A retired account is never the default: retiring it clears is_default in the same UPDATE.
    CONSTRAINT sms_account_retired_not_default
        CHECK (NOT (is_default AND retired_at IS NOT NULL))
);

-- AT MOST ONE DEFAULT ACCOUNT PER COMMUNE. Composite with tenant_id (rule 1, invariant 6); "not retired"
-- is carried by sms_account_retired_not_default. Changing the default clears the old one BEFORE setting
-- the new one (a partial unique index is checked immediately) — Go, one transaction.
CREATE UNIQUE INDEX IF NOT EXISTS sms_account_one_default_per_commune
    ON sms_account (tenant_id)
    WHERE is_default;

-- ONE LIVE ACCOUNT PER NAME IN A COMMUNE, case-insensitively — two "eSMS chính" in one picker cannot be
-- told apart. Partial on retired_at: a retired name may be used again (an account label is not an issued
-- code — header, VENDOR CHOICES).
CREATE UNIQUE INDEX IF NOT EXISTS sms_account_live_label
    ON sms_account (tenant_id, lower(label))
    WHERE retired_at IS NULL;

DROP TRIGGER IF EXISTS sms_account_guard ON sms_account;
CREATE TRIGGER sms_account_guard
    BEFORE UPDATE OR DELETE ON sms_account
    FOR EACH ROW EXECUTE FUNCTION sms_account_guard();

-- ===========================================================================
-- sms_template
-- ===========================================================================

-- ---------------------------------------------------------------------------
-- sms_template_guard
--
--   DELETE                 refused. A template eSMS no longer lists is `vanished_at`; the row stays —
--                          campaigns name it in template_ref.
--   tenant_id, id,         fixed: eSMS's identity of the template (TempId on a network) under one
--   account_id,            account. The same TempId under another account is another row.
--   provider_template_id,
--   network_id, created_at
--   everything else        eSMS's: rewritten freely by every sync.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION sms_template_guard() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION '%: delete refused', TG_TABLE_NAME
            USING HINT = 'A template eSMS no longer lists is recorded with vanished_at; the row stays.';
    END IF;
    IF NEW.tenant_id            IS DISTINCT FROM OLD.tenant_id
    OR NEW.id                   IS DISTINCT FROM OLD.id
    OR NEW.account_id           IS DISTINCT FROM OLD.account_id
    OR NEW.provider_template_id IS DISTINCT FROM OLD.provider_template_id
    OR NEW.network_id           IS DISTINCT FROM OLD.network_id
    OR NEW.created_at           IS DISTINCT FROM OLD.created_at THEN
        RAISE EXCEPTION '%: commune, row, account, TempId and network are immutable', TG_TABLE_NAME
            USING HINT = 'Another template, network or account is a new row.';
    END IF;
    RETURN NEW;
END $$;

-- ---------------------------------------------------------------------------
-- @entity: SmsTemplate
-- @scope:  tenant
--
-- sms_template — one CSKH template eSMS returns for an account (`GetTemplate`, SmsType=2, the account's
-- Brandname): one row per (TempId, NetworkID), exactly as eSMS lists it (header, VENDOR CHOICES). The
-- picker groups rows of one account by `content`.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS sms_template (
    tenant_id             TEXT        NOT NULL,
    id                    TEXT        NOT NULL,         -- ULID, internal
    account_id            TEXT        NOT NULL,
    -- eSMS's TempId, verbatim.
    provider_template_id  TEXT        NOT NULL,
    -- eSMS's NetworkID, verbatim (a small positive integer per mobile network at eSMS).
    network_id            SMALLINT    NOT NULL,
    -- eSMS's TempContent, verbatim, placeholders included (`{P1,20}`). Not personal data: it is the text
    -- the commune registered with the networks.
    content               TEXT        NOT NULL,
    -- The placeholders Go read out of `content`: an array of {name, max_length}. Recomputed every sync.
    params                JSONB       NOT NULL DEFAULT '[]'::jsonb,
    -- When this row last matched eSMS.
    synced_at             TIMESTAMPTZ NOT NULL,
    -- When a sync first found eSMS no longer listing it; cleared if it reappears.
    vanished_at           TIMESTAMPTZ,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, id),
    -- One row per eSMS template per network per account, in the commune (rule 1, invariant 6).
    UNIQUE (tenant_id, account_id, provider_template_id, network_id),
    FOREIGN KEY (tenant_id, account_id) REFERENCES sms_account (tenant_id, id),
    CONSTRAINT sms_template_provider_template_id_shape
        CHECK (provider_template_id ~ '^[0-9A-Za-z_-]{1,64}$'),
    CONSTRAINT sms_template_network_id_positive CHECK (network_id > 0),
    CONSTRAINT sms_template_content_length
        CHECK (btrim(content) <> '' AND char_length(content) <= 1000),
    CONSTRAINT sms_template_params_is_array CHECK (jsonb_typeof(params) = 'array'),
    CONSTRAINT sms_template_vanished_after_created
        CHECK (vanished_at IS NULL OR vanished_at >= created_at)
);

DROP TRIGGER IF EXISTS sms_template_guard ON sms_template;
CREATE TRIGGER sms_template_guard
    BEFORE UPDATE OR DELETE ON sms_template
    FOR EACH ROW EXECUTE FUNCTION sms_template_guard();

-- ===========================================================================
-- notice_campaign
-- ===========================================================================

-- ---------------------------------------------------------------------------
-- notice_campaign_guard — the campaign is the record of what a public authority sent to whom.
--
--   DELETE                   refused. There is no soft delete either: a sent campaign is archival.
--   what was decided at      title, channel, the OA / account, the template and its text, the params,
--   the send                 the audience, the counts and cost shown in Xem trước, who sent it and when
--                            — all fixed at insert. Editing one rewrites what was sent (rule 7 #5).
--   a finished campaign      ('da-gui', 'gui-mot-phan', 'that-bai') keeps its outcome: status, counts
--                            and finished_at are final. So the status only moves forward: dang-gui ->
--                            one terminal value, once.
--   sent_count, failed_count never decrease.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION notice_campaign_guard() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION '%: delete refused', TG_TABLE_NAME
            USING HINT = 'A campaign is the record of what the commune sent; it is never removed (rule 7).';
    END IF;
    IF NEW.tenant_id        IS DISTINCT FROM OLD.tenant_id
    OR NEW.id               IS DISTINCT FROM OLD.id
    OR NEW.title            IS DISTINCT FROM OLD.title
    OR NEW.channel          IS DISTINCT FROM OLD.channel
    OR NEW.oa_row_id        IS DISTINCT FROM OLD.oa_row_id
    OR NEW.sms_account_id   IS DISTINCT FROM OLD.sms_account_id
    OR NEW.template_ref     IS DISTINCT FROM OLD.template_ref
    OR NEW.template_name    IS DISTINCT FROM OLD.template_name
    OR NEW.template_content IS DISTINCT FROM OLD.template_content
    OR NEW.params           IS DISTINCT FROM OLD.params
    OR NEW.audience_kind    IS DISTINCT FROM OLD.audience_kind
    OR NEW.audience_ids     IS DISTINCT FROM OLD.audience_ids
    OR NEW.recipient_count  IS DISTINCT FROM OLD.recipient_count
    OR NEW.skipped_count    IS DISTINCT FROM OLD.skipped_count
    OR NEW.unit_price       IS DISTINCT FROM OLD.unit_price
    OR NEW.estimated_cost   IS DISTINCT FROM OLD.estimated_cost
    OR NEW.created_at       IS DISTINCT FROM OLD.created_at
    OR NEW.created_by       IS DISTINCT FROM OLD.created_by THEN
        RAISE EXCEPTION '%: what was sent, to whom, through what and by whom is immutable', TG_TABLE_NAME
            USING HINT = 'These are fixed when the campaign is sent. A different message is a new campaign.';
    END IF;
    IF OLD.status <> 'dang-gui'
       AND (NEW.status       IS DISTINCT FROM OLD.status
         OR NEW.sent_count   IS DISTINCT FROM OLD.sent_count
         OR NEW.failed_count IS DISTINCT FROM OLD.failed_count
         OR NEW.finished_at  IS DISTINCT FROM OLD.finished_at) THEN
        RAISE EXCEPTION '%: a finished campaign keeps its outcome', TG_TABLE_NAME
            USING HINT = 'Sent, partly sent and failed are final.';
    END IF;
    IF NEW.sent_count < OLD.sent_count OR NEW.failed_count < OLD.failed_count THEN
        RAISE EXCEPTION '%: sent_count and failed_count cannot decrease', TG_TABLE_NAME
            USING HINT = 'A message the provider accepted cannot become unsent.';
    END IF;
    RETURN NEW;
END $$;

-- ---------------------------------------------------------------------------
-- @entity: NoticeCampaign
-- @scope:  tenant
--
-- notice_campaign — one "Gửi tin ZNS / SMS" campaign (ADR 0099 #8): written with every one of its
-- recipient rows in ONE transaction, with its audit entry (rule 6). The background sender moves it from
-- 'dang-gui' to a terminal status when no recipient is left 'cho-gui'.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS notice_campaign (
    tenant_id         TEXT          NOT NULL,
    id                TEXT          NOT NULL,       -- ULID, internal
    title             TEXT          NOT NULL,
    -- 'zalo-zns' is 0004's spelling of the ZNS channel; one spelling for one channel.
    channel           TEXT          NOT NULL,
    -- THE SOURCE, chosen at send (ADR 0099 #1, #2): the OA row for ZNS, the account for SMS. Exactly one,
    -- matching the channel.
    oa_row_id         TEXT,
    sms_account_id    TEXT,
    -- The template: ZBS template_id (ZNS) or sms_template.id (SMS). Not a foreign key: the two channels
    -- name different things, and a ZNS template_id is Zalo's, not a row of ours.
    template_ref      TEXT          NOT NULL,
    template_name     TEXT          NOT NULL DEFAULT '',
    -- SMS: the template text as it stood at send (header, VENDOR CHOICES). ZNS: NULL.
    template_content  TEXT,
    -- The values staff typed for the template's parameters, for the whole campaign — not any recipient's
    -- data (ADR 0099 #6). An object {param name: value}.
    params            JSONB         NOT NULL DEFAULT '{}'::jsonb,
    -- WHO IT WAS FOR (ADR 0099 #5), as chosen on the screen: all citizens, citizen groups, citizen tags,
    -- all staff, staff units. audience_ids holds the chosen group / tag / unit ids; empty for the two
    -- "all" kinds.
    audience_kind     TEXT          NOT NULL,
    audience_ids      JSONB         NOT NULL DEFAULT '[]'::jsonb,
    -- Xem trước's figures, fixed at send: the rows written, the inactive directory rows identity skipped
    -- (ADR 0099 #5, "Bỏ qua"), the unit price and the estimated cost. The two money columns are an
    -- ESTIMATE and may be absent (ADR 0099 #7: an estimate is never a gate) — both or neither.
    recipient_count   INTEGER       NOT NULL,
    skipped_count     INTEGER       NOT NULL DEFAULT 0,
    unit_price        NUMERIC(14, 2),
    estimated_cost    NUMERIC(14, 2),
    status            TEXT          NOT NULL DEFAULT 'dang-gui',
    sent_count        INTEGER       NOT NULL DEFAULT 0,
    failed_count      INTEGER       NOT NULL DEFAULT 0,
    created_at        TIMESTAMPTZ   NOT NULL DEFAULT now(),
    -- Who sent it: the STAFF business code (`CB-…`, rule 6 invariant 8).
    created_by        TEXT          NOT NULL,
    finished_at       TIMESTAMPTZ,
    PRIMARY KEY (tenant_id, id),
    FOREIGN KEY (tenant_id, oa_row_id) REFERENCES zalo_commune_oa (tenant_id, id),
    FOREIGN KEY (tenant_id, sms_account_id) REFERENCES sms_account (tenant_id, id),
    CONSTRAINT notice_campaign_title_shape
        CHECK (btrim(title) <> '' AND char_length(title) <= 200),
    CONSTRAINT notice_campaign_channel_known CHECK (channel IN ('zalo-zns', 'sms')),
    -- ZNS goes through an OA and nothing else; SMS through an account and nothing else.
    CONSTRAINT notice_campaign_source_matches_channel
        CHECK ((channel = 'zalo-zns' AND oa_row_id IS NOT NULL AND sms_account_id IS NULL)
            OR (channel = 'sms' AND sms_account_id IS NOT NULL AND oa_row_id IS NULL)),
    CONSTRAINT notice_campaign_template_ref_shape
        CHECK (btrim(template_ref) <> '' AND char_length(template_ref) <= 64),
    CONSTRAINT notice_campaign_template_name_length CHECK (char_length(template_name) <= 255),
    CONSTRAINT notice_campaign_template_content_for_sms
        CHECK ((channel = 'sms') = (template_content IS NOT NULL)),
    CONSTRAINT notice_campaign_template_content_length
        CHECK (template_content IS NULL OR (btrim(template_content) <> '' AND char_length(template_content) <= 1000)),
    CONSTRAINT notice_campaign_params_is_object CHECK (jsonb_typeof(params) = 'object'),
    CONSTRAINT notice_campaign_audience_kind_known
        CHECK (audience_kind IN ('nguoi-dan-tat-ca', 'nguoi-dan-theo-nhom', 'nguoi-dan-theo-nhan', 'can-bo-tat-ca', 'can-bo-theo-bo-phan')),
    -- An array; empty exactly for the two "all" kinds. CASE so a non-array is refused, never an error.
    CONSTRAINT notice_campaign_audience_ids_shape
        CHECK (CASE WHEN jsonb_typeof(audience_ids) <> 'array' THEN false
                    WHEN audience_kind IN ('nguoi-dan-tat-ca', 'can-bo-tat-ca') THEN jsonb_array_length(audience_ids) = 0
                    ELSE jsonb_array_length(audience_ids) > 0 END),
    CONSTRAINT notice_campaign_counts_non_negative
        CHECK (recipient_count >= 0 AND skipped_count >= 0 AND sent_count >= 0 AND failed_count >= 0),
    CONSTRAINT notice_campaign_counts_within_recipients
        CHECK (sent_count + failed_count <= recipient_count),
    CONSTRAINT notice_campaign_estimate_pair
        CHECK ((unit_price IS NULL) = (estimated_cost IS NULL)),
    CONSTRAINT notice_campaign_estimate_non_negative
        CHECK (unit_price IS NULL OR (unit_price >= 0 AND estimated_cost >= 0)),
    --   dang-gui       recipients are still 'cho-gui'
    --   da-gui         every recipient row was accepted by the provider
    --   gui-mot-phan   some accepted, some failed or skipped
    --   that-bai       none accepted
    CONSTRAINT notice_campaign_status_known
        CHECK (status IN ('dang-gui', 'da-gui', 'gui-mot-phan', 'that-bai')),
    CONSTRAINT notice_campaign_finished_has_time
        CHECK ((status = 'dang-gui') = (finished_at IS NULL)),
    CONSTRAINT notice_campaign_finished_after_created
        CHECK (finished_at IS NULL OR finished_at >= created_at),
    CONSTRAINT notice_campaign_created_by_signed
        CHECK (btrim(created_by) <> '' AND char_length(created_by) <= 64)
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..7 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS notice_campaign_p%s PARTITION OF notice_campaign '
            'FOR VALUES WITH (MODULUS 8, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

-- The "Đã gửi" tab: the commune's campaigns, newest first.
CREATE INDEX IF NOT EXISTS notice_campaign_by_time
    ON notice_campaign (tenant_id, created_at DESC, id DESC);

-- The sender's outer loop: campaigns still running in a commune. Finished ones leave the index.
CREATE INDEX IF NOT EXISTS notice_campaign_running
    ON notice_campaign (tenant_id, created_at)
    WHERE status = 'dang-gui';

DROP TRIGGER IF EXISTS notice_campaign_guard ON notice_campaign;
CREATE TRIGGER notice_campaign_guard
    BEFORE UPDATE OR DELETE ON notice_campaign
    FOR EACH ROW EXECUTE FUNCTION notice_campaign_guard();

-- ===========================================================================
-- notice_recipient
-- ===========================================================================

-- ---------------------------------------------------------------------------
-- notice_recipient_guard — the 0018 zalo_delivery discipline: the outbox is also the history.
--
--   DELETE                refused. A deleted row would also let a retry send the message again.
--   who and which         tenant_id, id, campaign_id, recipient_ref, recipient_kind, phone_masked,
--   campaign              created_at: fixed at the campaign's transaction. Editing one turns a true
--                         statement about one recipient into a false one about another.
--   a finished row        ('da-gui', 'that-bai', 'bo-qua') keeps its outcome: a message the provider
--                         accepted cannot become unsent ('da-gui' never moves back), and a recorded
--                         failure is not rewritten. Only updated_at may still change.
--   attempts              never decreases.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION notice_recipient_guard() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION '%: delete refused', TG_TABLE_NAME
            USING HINT = 'A recipient row is the record that the commune sent, or failed to send, one '
                         'message (rule 7). It is never removed.';
    END IF;
    IF NEW.tenant_id      IS DISTINCT FROM OLD.tenant_id
    OR NEW.id             IS DISTINCT FROM OLD.id
    OR NEW.campaign_id    IS DISTINCT FROM OLD.campaign_id
    OR NEW.recipient_ref  IS DISTINCT FROM OLD.recipient_ref
    OR NEW.recipient_kind IS DISTINCT FROM OLD.recipient_kind
    OR NEW.phone_masked   IS DISTINCT FROM OLD.phone_masked
    OR NEW.created_at     IS DISTINCT FROM OLD.created_at THEN
        RAISE EXCEPTION '%: recipient, number and campaign are immutable', TG_TABLE_NAME
            USING HINT = 'Who a message was owed to is fixed when the campaign is written.';
    END IF;
    IF OLD.status <> 'cho-gui'
       AND (NEW.status              IS DISTINCT FROM OLD.status
         OR NEW.provider_message_id IS DISTINCT FROM OLD.provider_message_id
         OR NEW.failure_class       IS DISTINCT FROM OLD.failure_class
         OR NEW.attempts            IS DISTINCT FROM OLD.attempts
         OR NEW.lease_until         IS DISTINCT FROM OLD.lease_until
         OR NEW.sent_at             IS DISTINCT FROM OLD.sent_at) THEN
        RAISE EXCEPTION '%: a finished recipient keeps its outcome', TG_TABLE_NAME
            USING HINT = 'Sent, failed and skipped are final. Moving a sent row back would send it '
                         'a second time.';
    END IF;
    IF NEW.attempts < OLD.attempts THEN
        RAISE EXCEPTION '%: attempts cannot decrease', TG_TABLE_NAME
            USING HINT = 'The attempt count is evidence of how hard the channel was tried.';
    END IF;
    RETURN NEW;
END $$;

-- ---------------------------------------------------------------------------
-- @entity: NoticeRecipient
-- @scope:  tenant
--
-- notice_recipient — one message of one campaign to one recipient. The row's id is the provider's
-- de-duplication key (eSMS RequestId, Zalo tracking_id — ADR 0099 #8). THE FULL NUMBER IS NEVER HERE:
-- comms asks identity for it at send time and holds it in memory for the one provider call (ADR 0099 #5).
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS notice_recipient (
    tenant_id            TEXT        NOT NULL,
    id                   TEXT        NOT NULL,          -- ULID, internal; the provider's RequestId
    campaign_id          TEXT        NOT NULL,
    -- The directory row (citizen, ADR 0097) or staff row the message is owed to — an opaque code, the
    -- key the sender hands identity to get the number back.
    recipient_ref        TEXT        NOT NULL,
    recipient_kind       TEXT        NOT NULL,
    -- core/privacy.MaskPhone of the number at the campaign's write: 09****0000. NEVER the full number.
    phone_masked         TEXT        NOT NULL,
    status               TEXT        NOT NULL DEFAULT 'cho-gui',
    -- eSMS SMSID / Zalo msg_id of an accepted message.
    provider_message_id  TEXT,
    -- A CLASS, never a provider message or an error string (header, VENDOR CHOICES).
    failure_class        TEXT,
    attempts             SMALLINT    NOT NULL DEFAULT 0,
    -- Held by a sender until this time; a 'cho-gui' row past it is reclaimable.
    lease_until          TIMESTAMPTZ,
    sent_at              TIMESTAMPTZ,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, id),
    -- ONE MESSAGE PER RECIPIENT PER CAMPAIGN, in the commune (rule 1, invariant 6): a directory row
    -- reached through two chosen groups is still one message.
    UNIQUE (tenant_id, campaign_id, recipient_ref),
    FOREIGN KEY (tenant_id, campaign_id) REFERENCES notice_campaign (tenant_id, id),
    CONSTRAINT notice_recipient_ref_shape
        CHECK (btrim(recipient_ref) <> '' AND char_length(recipient_ref) <= 64),
    CONSTRAINT notice_recipient_kind_known CHECK (recipient_kind IN ('nguoi-dan', 'can-bo')),
    -- THE MASK IS ENFORCED BY THE DATABASE (0004's reasoning): a masked number always carries the mask
    -- character, a raw one never does. Stronger than 0004: MaskPhone keeps six characters, so at most
    -- six DIGITS may remain — a number with one '*' typed into it at a psql prompt is still refused.
    CONSTRAINT notice_recipient_phone_must_be_masked
        CHECK (phone_masked LIKE '%*%'
               AND char_length(regexp_replace(phone_masked, '[^0-9]', '', 'g')) <= 6
               AND char_length(phone_masked) <= 20),
    --   cho-gui    owed, not yet accepted (possibly leased by a sender, possibly retrying)
    --   da-gui     the provider ACCEPTED it (eSMS CodeResult 100 / Zalo msg_id) — not "on the handset"
    --   that-bai   refused, or retries ran out — failure_class says how
    --   bo-qua     deliberately not sent at send time — failure_class says why
    CONSTRAINT notice_recipient_status_known
        CHECK (status IN ('cho-gui', 'da-gui', 'that-bai', 'bo-qua')),
    -- "Sent" without a time is a claim with no date on it (0004); and without the provider's id it is a
    -- claim nobody can check with the provider.
    CONSTRAINT notice_recipient_sent_has_time
        CHECK ((status = 'da-gui') = (sent_at IS NOT NULL)),
    CONSTRAINT notice_recipient_sent_has_provider_id
        CHECK (status <> 'da-gui' OR provider_message_id IS NOT NULL),
    CONSTRAINT notice_recipient_provider_id_shape
        CHECK (provider_message_id IS NULL OR provider_message_id ~ '^[!-~]{1,128}$'),
    CONSTRAINT notice_recipient_failure_has_class
        CHECK (status NOT IN ('that-bai', 'bo-qua') OR failure_class IS NOT NULL),
    CONSTRAINT notice_recipient_sent_has_no_failure
        CHECK (status <> 'da-gui' OR failure_class IS NULL),
    CONSTRAINT notice_recipient_failure_class_shape
        CHECK (failure_class IS NULL
               OR (failure_class ~ '^[a-z]+(-[a-z]+)*$' AND char_length(failure_class) <= 40)),
    -- Only an owed row is leased.
    CONSTRAINT notice_recipient_lease_only_pending
        CHECK (status = 'cho-gui' OR lease_until IS NULL),
    CONSTRAINT notice_recipient_attempts_non_negative CHECK (attempts >= 0)
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..7 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS notice_recipient_p%s PARTITION OF notice_recipient '
            'FOR VALUES WITH (MODULUS 8, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

-- THE SENDER'S QUEUE, per commune (background work carries tenant_id — rule 1, invariant 9): rows still
-- owed, by lease. Finished rows leave the index, so it stays the size of the backlog. A campaign's own
-- rows are reached through the unique key above (tenant_id, campaign_id, …).
CREATE INDEX IF NOT EXISTS notice_recipient_pending
    ON notice_recipient (tenant_id, status, lease_until)
    WHERE status = 'cho-gui';

DROP TRIGGER IF EXISTS notice_recipient_guard ON notice_recipient;
CREATE TRIGGER notice_recipient_guard
    BEFORE UPDATE OR DELETE ON notice_recipient
    FOR EACH ROW EXECUTE FUNCTION notice_recipient_guard();

-- ---------------------------------------------------------------------------
-- WHAT THIS FILE DOES NOT STOP: TRUNCATE and DDL by the table owner — the line 0003, 0004 and ADR 0013
-- draw. Nor a campaign whose recipient rows disagree with recipient_count, nor a terminal status that
-- disagrees with its recipients' outcomes (Go writes both in one transaction; a CHECK cannot see other
-- rows), nor a recipient row inserted already finished.
--
-- REVERSAL (question 3). Run by a person, in ONE transaction (ADR 0013). Prose, because a runnable line
-- is a line that gets run.
--
-- THE FOUR NEW TABLES — LOSSLESS ONLY WHILE ALL FOUR ARE EMPTY. Then, in this order (the foreign keys
-- point from recipient to campaign, from campaign to account and OA, from template to account): drop
-- the table notice_recipient, then notice_campaign (their eight leaves, indexes and triggers go with
-- them), then sms_template, then sms_account; drop the four functions notice_recipient_guard,
-- notice_campaign_guard, sms_template_guard, sms_account_guard.
--
-- (A) THE ONE-LIVE-OA-PER-COMMUNE KEY — RESTORABLE ONLY WHILE EVERY COMMUNE HAS AT MOST ONE LIVE OA.
-- Measure first: SELECT tenant_id, count(*) FROM zalo_commune_oa WHERE retired_at IS NULL GROUP BY
-- tenant_id HAVING count(*) > 1 — any row returned means a commune is USING several OAs, and restoring
-- the key would first require retiring OAs a commune chose to keep: that is the commune's decision, not
-- a reversal. With no row returned: drop the index zalo_commune_oa_live_by_commune and recreate
-- zalo_commune_oa_one_live_per_commune exactly as 0031 writes it (tenant_id, partial on retired_at).
--
-- Finally remove this file's row from `schema_migration` (ten = '0032_notice_campaigns_sms_multi_oa.sql').
--
-- ONCE ANY ROW EXISTS in the four tables it is no longer a reversal: an account row holds a commune's
-- sealed keys and the record of which account campaigns went through; a campaign and its recipients are
-- the record of what a public authority sent to whom. Rule 7 stop conditions #1/#2 — the user, a
-- verified backup, and a NEW migration.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- BACKSTOP: every partitioned table in this schema must actually have partitions (0002, §BACKSTOP).
-- This file declares two; the check describes the state after the newest migration that carries it.
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
