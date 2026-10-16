-- 0031 — A COMMUNE'S ZALO OA, AUTHORIZED TO THE PLATFORM'S SHARED ZALO APP (ADR 0098, block A; owner
-- 16/10/2026: "1 app dùng chung cho toàn bộ dự án không phải riêng cho từng xã"). ONE Zalo App for the
-- whole platform calls the OA / ZBS APIs; EACH commune authorizes ITS OWN OA to that app through OAuth v4
-- + PKCE, from web-admin (Cấu hình › Kênh Zalo › OA của xã). The commune's OA is the sender a citizen
-- sees (ADR 0006, 0018): its templates, its quota, its name.
--
--   zalo_commune_oa         tenant   one LIVE row per commune: the authorized OA, both tokens sealed
--   zalo_oa_authorization   tenant   one authorization request (a link handed to the OA admin), found
--                                    again FROM `state` when Zalo calls back
--   zalo_oa_template        tenant   the ZBS templates loaded from the commune's OA, and which petition
--                                    milestone each one is attached to
--
-- All three are PLAIN tables (ADR 0096 #3): a handful of rows per commune. Two of them also carry a
-- platform-wide unique key, which a partitioned table could not hold (0018 zalo_link's reason).
--
-- NAMED IN ENGLISH (rule 12, ADR 0051); the entity names, table names and URL nouns are the ones
-- `kb/00-foundation/ubiquitous-language.md` §"OA Zalo của xã" fixes (owner 16/10/2026). Enum values of
-- OUR OWN are Vietnamese without diacritics, kebab-case (ADR 0011) — with ONE named exception, below.
--
-- ---------------------------------------------------------------------------
-- WHAT IS DECIDED, and where each lands:
--
--   ADR 0098 #1   one platform app; the commune authorizes its OA   the whole file: no app id, no app
--                                                                    secret here — those are environment
--                                                                    (ADR 0098 #2), never a row
--   ADR 0098 #4   Zalo's callback has no commune Host; the commune   zalo_oa_authorization: state_hash,
--                 is found FROM `state` — a cross-commune read       platform-wide unique, A LOOKUP BY
--                 carrying `// @cross-tenant`, returning only the    DESIGN (the one deliberate
--                 commune's identity                                 departure from 0022's "floor only")
--   ADR 0098 #5   both tokens, and the pending code_verifier, are    access_token_sealed,
--                 sealed under the commune's own DEK; write-only     refresh_token_sealed,
--                                                                    code_verifier_sealed
--   ADR 0098 #6   a background job refreshes the access token; a    token_refreshed_at (changes on EVERY
--                 refused or 3-month-old refresh token puts the row  reseal), needs_reauthorization_since
--                 into "needs re-authorization"                      / _reason
--   ADR 0098 #8   templates loaded from the OA; only ENABLE ones     zalo_oa_template: zalo_status is
--                 may be attached to a milestone; one template per   Zalo's own value; milestone +
--                 milestone                                          one-per-milestone unique key
--   ADR 0098 #9   the three template parameters are REQUIRED BY      params (what Zalo declared),
--                 NAME: ma_tra_cuu, moc_nhan, viec_tiep_theo; no     params_match (Go's verdict), read
--                 mapping table                                      together at attach time
--   ADR 0098 #11  "Ngắt OA" soft-deletes the row, keeps the sealed   retired_at / retired_by /
--                 tokens as history (rule 7), detaches milestones    retire_reason
--   ADR 0098 #12  audited in comms' audit_log in the same            NOT A NEW AUDIT TABLE; before/after
--                 transaction; a sealed column never in the trail    never carry a *_sealed column (Go)
--   rule 6 inv 8  who authorized / requested / attached: the STAFF   authorized_by, requested_by,
--                 business code `CB-…`, never an internal id          milestone_set_by
--
-- ---------------------------------------------------------------------------
-- VENDOR CHOICES, stated so a reviewer can overrule them:
--
--   * TWO TIMESTAMPS ON THE OA ROW, NOT 0022's ONE (ADR 0098 §"Hai cột thời điểm"). 0022 refuses "a new
--     token without a new set_at" so the screen never describes an old token. The OA row has TWO
--     writers of its sealed columns: the member of staff who connects (authorized_at / authorized_by,
--     a person) and the BACKGROUND REFRESH JOB (a system principal, rule 6 invariant 6). So the guard
--     here demands a new `token_refreshed_at` whenever either sealed column changes, and leaves
--     authorized_* immutable. Copying 0022's guard verbatim would refuse the very first refresh.
--   * ONE ROW PER AUTHORIZATION, NOT ONE ROW PER COMMUNE. `oa_id` is immutable on a row. Refreshing
--     tokens for the SAME OA is an in-place reseal; authorizing a DIFFERENT OA is: retire the live row,
--     insert a new one (ADR 0098 #11 "ngắt rồi nối, dòng mới"). The table is therefore the history of
--     which OA served the commune when, and `zalo_oa_template.oa_row_id` points at a row whose OA never
--     changes meaning.
--   * THE SEALED COLUMNS ARE UNDER THE COMMUNE'S DEK (data_encryption_key, 0008), 0022's discipline.
--     OWED BY GO: additional data naming the SECRET, the COMMUNE and the ROW:
--        access token    "zalo_commune_oa/access_token/<tenant_id>/<id>"
--        refresh token   "zalo_commune_oa/refresh_token/<tenant_id>/<id>"
--        code verifier   "zalo_oa_authorization/code_verifier/<tenant_id>/<id>"
--     Bound to the row id, so bytes copied from a retired row onto the live one do not open; to the
--     tenant, so they do not open in another commune; to the secret name, so a refresh token does not
--     open as an access token.
--   * A RETIRED ROW KEEPS ITS SEALED TOKENS. Nothing here destroys a secret (rule 7). BUT RETIRING HERE
--     REVOKES NOTHING AT ZALO: the app keeps the OA's grant until the OA admin revokes it in OA Manager.
--     OWED BY GO (ADR 0098 #11): the "Ngắt OA" result must SAY SO to the commune.
--   * `state` IS STORED ONLY AS ITS SHA-256 (state_hash), like 0018's pairing code. The value itself
--     travels through Zalo's redirect and the OA admin's browser; a table of plain values would be a
--     table of live credentials for "become this commune's OA". The hash is 32 CSPRNG bytes hashed, so
--     the lookup by hash is exact and unguessable; the code_verifier is sealed because it must be
--     RECOVERED (sent to Zalo with the code), not merely compared.
--   * ONE LIVE PLACE PER OA, PLATFORM-WIDE — A UNIQUENESS FLOOR. Two communes holding one OA live would
--     send one commune's petitions under the other's name. Hence the cross-tenant unique key on live
--     oa_id. Nothing resolves a commune from it (see the index).
--   * `zalo_status` AND `zalo_quality` HOLD ZALO'S OWN VALUES VERBATIM — THE ONE NAMED EXCEPTION TO ADR
--     0011. These are a THIRD PARTY's enumerations, compared against their API on every sync
--     (`template/all?status=…`, `template/info`); a Vietnamese translation would be a second copy of
--     Zalo's list that drifts the day Zalo adds a value. The column NAMES say whose values they are.
--     Every enumeration of OUR OWN in this file (check results, outcomes, reasons, milestones) is ADR
--     0011's kebab-case Vietnamese.
--   * NO CHECK TIES `milestone` TO `zalo_status` / `params_match`. Zalo changes a template's status on
--     ITS side; a sync that finds an attached template REJECTed or DELETEd must be able to RECORD that
--     without first detaching the milestone (the record of what the commune attached is the record).
--     The `MauTinXa` implementation checks AT READ TIME: live OA, milestone attached, zalo_status =
--     'ENABLE', params_match — anything else is `ErrXaChuaCauHinhKenh` (ADR 0098 #10). Attaching a
--     milestone to a non-ENABLE or non-matching template is refused BY GO at the PATCH (ADR 0098 #8, #9).
--   * TEMPLATES HAVE NO SOFT-DELETE TRIO. A template that vanished at Zalo is `zalo_status = 'DELETE'`
--     (Zalo's own word for it); every row ever synced stays. Deleting the row would lose which template
--     once carried a milestone — and the ledger rows (0004) that went out under its template_id.
--   * `zalo_oa_authorization.oa_row_id` HAS A FOREIGN KEY to the OA row it produced (same commune). The
--     card asked only for the CHECK; the key costs nothing and makes "success produced THIS row" a fact
--     the database holds. The CHECK is written both ways: success records a row, and only success does.
--
-- NOT PARTITIONED (ADR 0096 #3): configuration tables, a handful of rows per commune; and the two
-- platform-wide unique keys (`zalo_commune_oa_one_live_per_oa`, `zalo_oa_authorization_state_hash`)
-- cannot contain tenant_id, which PostgreSQL requires of every unique index on a partitioned table.
-- Every read is still by commune through the scoped store (rule 1, invariant 5) — except the ONE
-- declared callback lookup.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. ROWS PER COMMUNE: NONE WRITTEN — no seed, no default OA, no template. Steady state: zero rows
--      (the commune has not connected an OA) or one live OA row plus one retired row per change; a few
--      authorization rows per connection attempt (each link handed out is a row); tens of template rows
--      per OA. Nothing existing is touched: three new tables, no ALTER.
--   2. IF IT STOPS HALF-WAY: cannot — one file, one transaction. Every statement is IF NOT EXISTS /
--      CREATE OR REPLACE / DROP TRIGGER IF EXISTS, so a retry costs nothing. No backfill, nothing to
--      resume per commune.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. READ PATHS THAT CHANGE MEANING: none today — nothing reads these tables yet. ONCE THE GO CARD
--      WRITES THEM, three traps for every future reader, stated here because the schema cannot stop them:
--        * A LIVE OA ROW IS NOT A USABLE OA. `retired_at IS NULL` alone admits a row whose refresh token
--          Zalo refused (needs_reauthorization_since IS NOT NULL) and a row whose access token has
--          expired (access_expires_at <= now()). The sender (block B) and MauTinXa must read all three;
--          the first two are "needs re-authorization" (ledger `chua-cau-hinh-kenh`, ADR 0098 #6), the
--          third is "refresh first, never send with it".
--        * AN ATTACHED TEMPLATE IS NOT A SENDABLE TEMPLATE. `milestone IS NOT NULL` alone admits a
--          template Zalo has since REJECTed, DISABLEd or DELETEd, or one whose parameters no longer
--          match. Read `zalo_status = 'ENABLE' AND params_match` with it (ADR 0098 #10).
--        * AN AUTHORIZATION ROW IS SPENT WHEN `used_at IS NOT NULL` — WHATEVER THE OUTCOME. A callback
--          that found the row but failed at Zalo still spends it (one authorization code, one try); the
--          OA admin gets a new link. A reader that treats `outcome <> 'thanh-cong'` as "still open"
--          would let a `state` be replayed.
--   5. RETENTION: nothing dropped, retyped, emptied or renumbered. All three tables refuse DELETE; a
--      retired OA row and a spent authorization row are frozen whole; a template row is kept through
--      every Zalo status, DELETE included.
--
-- PERSONAL DATA (rule 3): none. An OA's id and name, a template's name and parameter NAMES, and staff
-- business codes are not a person's data. The tokens are sealed and write-only. NEVER log a token, the
-- authorization `code`, a `state`, or a callback URL (ADR 0098 #13) — the hash is what is stored, the
-- class is what is logged. PG FLOOR: 13. Cluster: 16.
-- ---------------------------------------------------------------------------

-- ===========================================================================
-- zalo_commune_oa
-- ===========================================================================

-- ---------------------------------------------------------------------------
-- zalo_commune_oa_guard
--
--   DELETE                   refused: retire with retired_at / retired_by / retire_reason.
--   a retired row            frozen whole — it is the record of an OA that served this commune
--                            (rule 7, forbidden #5). Authorize again = a new row.
--   tenant_id, id, oa_id,    fixed. A different OA is a different row; moving a row to another commune
--   created_at,              moves sealed bytes bound to the first one; who connected the OA and when
--   authorized_at,           is the trail's "who" (rule 6 invariant 8) and does not change on refresh.
--   authorized_by
--   a resealed token         refused unless token_refreshed_at changed. Both the connecting member of
--   without a new            staff and the refresh job reseal; both must leave the time. A screen
--   token_refreshed_at       reading "làm mới lúc …" must never describe an older token (ADR 0098
--                            §"Hai cột thời điểm" — deliberately NOT 0022's set_at rule).
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION zalo_commune_oa_guard() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION '%: delete refused', TG_TABLE_NAME
            USING HINT = 'Retire the OA with retired_at, retired_by, retire_reason instead.';
    END IF;
    IF OLD.retired_at IS NOT NULL THEN
        RAISE EXCEPTION '%: a retired OA is not edited', TG_TABLE_NAME
            USING HINT = 'A retired OA is a historical record (rule 7, forbidden #5). Add a new row.';
    END IF;
    IF NEW.tenant_id     IS DISTINCT FROM OLD.tenant_id
    OR NEW.id            IS DISTINCT FROM OLD.id
    OR NEW.oa_id         IS DISTINCT FROM OLD.oa_id
    OR NEW.created_at    IS DISTINCT FROM OLD.created_at
    OR NEW.authorized_at IS DISTINCT FROM OLD.authorized_at
    OR NEW.authorized_by IS DISTINCT FROM OLD.authorized_by THEN
        RAISE EXCEPTION '%: commune, row, OA and who authorized it are immutable', TG_TABLE_NAME
            USING HINT = 'A different OA, or a new authorization, is a new row: retire this one first.';
    END IF;
    IF (NEW.access_token_sealed IS DISTINCT FROM OLD.access_token_sealed
        OR NEW.refresh_token_sealed IS DISTINCT FROM OLD.refresh_token_sealed)
       AND NEW.token_refreshed_at IS NOT DISTINCT FROM OLD.token_refreshed_at THEN
        RAISE EXCEPTION '%: resealed tokens need a new token_refreshed_at', TG_TABLE_NAME
            USING HINT = 'Both the connecting staff member and the refresh job reseal; the row must '
                         'say when the tokens in it were obtained.';
    END IF;
    RETURN NEW;
END $$;

-- ---------------------------------------------------------------------------
-- @entity: ZaloCommuneOA
-- @scope:  tenant
--
-- zalo_commune_oa — the commune's Zalo OA, authorized to the platform's shared Zalo App. NO LIVE ROW =
-- THE COMMUNE SENDS NOTHING THROUGH ZALO (ledger rows become `chua-cau-hinh-kenh`, ADR 0006 consequence
-- 3). Both tokens are sealed and WRITE-ONLY: no route returns them; reads report "nối lúc … bởi …" and
-- "làm mới lúc …". The access token travels in a HEADER, not a URL (ADR 0098 #13), but the rule is the
-- same as the bot's: never log a response body, never store an error string — last_check_result is a
-- closed class.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS zalo_commune_oa (
    tenant_id                    TEXT        NOT NULL,
    id                           TEXT        NOT NULL,      -- ULID, internal
    -- The OA's id as Zalo returns it on the callback (`oa_id`) and from `getoa`. Digits only at Zalo.
    oa_id                        TEXT        NOT NULL,
    -- The OA's display name from `getoa` at connection and at every "Kiểm tra"; shown to staff.
    oa_name                      TEXT        NOT NULL DEFAULT '',
    -- Sealed under the commune's DEK (0008), each with its own additional data (header). Resealed
    -- together on every refresh: Zalo rotates the refresh token and voids the old access token.
    access_token_sealed          BYTEA       NOT NULL,
    refresh_token_sealed         BYTEA       NOT NULL,
    -- When the access token stops working (Zalo: 25 hours from issue). The refresh job acts while
    -- less than 2 hours remain (ADR 0098 #6); nothing sends past it.
    access_expires_at            TIMESTAMPTZ NOT NULL,
    -- When the CURRENT refresh token was issued: it lives 3 months from here (Zalo), unused. Set at
    -- connection, moved forward at every refresh.
    refresh_issued_at            TIMESTAMPTZ NOT NULL,
    -- When the sealed columns were last written — by the connecting member of staff or by the refresh
    -- job. The guard refuses a reseal that does not move it.
    token_refreshed_at           TIMESTAMPTZ NOT NULL,
    -- Who connected this OA and when: the STAFF business code (`CB-…`, rule 6 invariant 8), fixed for
    -- the life of the row. The refresh job never writes these two.
    authorized_at                TIMESTAMPTZ NOT NULL,
    authorized_by                TEXT        NOT NULL,
    -- The last "Kiểm tra kết nối" (`getoa`): 0022's seven classes, one vocabulary for one Zalo call.
    last_check_at                TIMESTAMPTZ,
    last_check_result            TEXT,
    -- NEEDS RE-AUTHORIZATION (ADR 0098 #6): Zalo refused the refresh token, or it aged past 3 months
    -- unused. The row stays live (the commune still "has" this OA, and its templates and milestones are
    -- intact) but nothing may send through it until the OA admin authorizes again — which is a NEW row;
    -- this one is then retired. Both or neither.
    needs_reauthorization_since  TIMESTAMPTZ,
    needs_reauthorization_reason TEXT,
    -- RETIRED: "Ngắt OA", or replaced by another OA. The trio is this table's soft delete (rule 7,
    -- invariant 1), spelled for what it means, as 0022's. retired_by: the staff code of whoever did it.
    retired_at                   TIMESTAMPTZ,
    retired_by                   TEXT,
    retire_reason                TEXT,
    created_at                   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, id),
    CONSTRAINT zalo_commune_oa_oa_id_shape CHECK (oa_id ~ '^[0-9]{1,32}$'),
    CONSTRAINT zalo_commune_oa_oa_name_length CHECK (char_length(oa_name) <= 255),
    -- 1 + 12 + 16 bytes of envelope overhead, plus at least one byte of secret (0008).
    CONSTRAINT zalo_commune_oa_access_token_sealed_length CHECK (octet_length(access_token_sealed) > 29),
    CONSTRAINT zalo_commune_oa_refresh_token_sealed_length CHECK (octet_length(refresh_token_sealed) > 29),
    CONSTRAINT zalo_commune_oa_authorized_by_signed
        CHECK (btrim(authorized_by) <> '' AND char_length(authorized_by) <= 64),
    CONSTRAINT zalo_commune_oa_check_pair
        CHECK ((last_check_at IS NULL) = (last_check_result IS NULL)),
    CONSTRAINT zalo_commune_oa_check_result_known
        CHECK (last_check_result IS NULL
               OR last_check_result IN ('thanh-cong', 'chua-cau-hinh', 'token-bi-tu-choi',
                                        'gioi-han-tan-suat', 'khong-kha-dung', 'bi-tu-choi',
                                        'phan-hoi-sai-dang')),
    CONSTRAINT zalo_commune_oa_reauthorization_pair
        CHECK ((needs_reauthorization_since IS NULL) = (needs_reauthorization_reason IS NULL)),
    CONSTRAINT zalo_commune_oa_reauthorization_reason_known
        CHECK (needs_reauthorization_reason IS NULL
               OR needs_reauthorization_reason IN ('refresh-token-bi-tu-choi', 'refresh-token-het-han')),
    -- All three, or none (the shape rule 7 invariant 1 asks of a soft delete).
    CONSTRAINT zalo_commune_oa_retire_complete
        CHECK ((retired_at IS NULL) = (retired_by IS NULL)
               AND (retired_at IS NULL) = (retire_reason IS NULL)),
    CONSTRAINT zalo_commune_oa_retire_signed
        CHECK (retired_by IS NULL OR (btrim(retired_by) <> '' AND char_length(retired_by) <= 64)),
    CONSTRAINT zalo_commune_oa_retire_reason_length
        CHECK (retire_reason IS NULL OR (btrim(retire_reason) <> '' AND char_length(retire_reason) <= 200)),
    CONSTRAINT zalo_commune_oa_retired_after_created
        CHECK (retired_at IS NULL OR retired_at >= created_at)
);

-- AT MOST ONE LIVE OA PER COMMUNE. Composite with tenant_id (rule 1, invariant 6). Partial on
-- retired_at, not deleted_at, and it frees no issued code — an OA row issues none; a retired row stays
-- as history. Replacing the OA retires the old row BEFORE inserting the new one (a partial unique index
-- is checked immediately, never deferred). This is the sender's and the screen's read: WHERE tenant_id
-- = <the Host's commune> AND retired_at IS NULL — one commune, no cross-commune read.
CREATE UNIQUE INDEX IF NOT EXISTS zalo_commune_oa_one_live_per_commune
    ON zalo_commune_oa (tenant_id)
    WHERE retired_at IS NULL;

-- @scope: cross-tenant — A UNIQUENESS FLOOR, NOT A LOOKUP. One live place per OA across the platform:
-- two communes holding one OA live would each send the other's petitions under their own name, and the
-- citizen would read commune A's notice signed by commune B. NOTHING READS oa_id -> tenant_id THROUGH
-- THIS INDEX: the callback finds the commune from `state` (zalo_oa_authorization, below), never from
-- the oa_id Zalo sends; a callback whose oa_id is live in ANOTHER commune is answered to the requesting
-- commune as outcome 'oa-dang-o-xa-khac' and names no commune. The only cross-commune touch here is the
-- database's own uniqueness check, which answers yes/no and returns nothing about another commune.
CREATE UNIQUE INDEX IF NOT EXISTS zalo_commune_oa_one_live_per_oa
    ON zalo_commune_oa (oa_id)
    WHERE retired_at IS NULL;

DROP TRIGGER IF EXISTS zalo_commune_oa_guard ON zalo_commune_oa;
CREATE TRIGGER zalo_commune_oa_guard
    BEFORE UPDATE OR DELETE ON zalo_commune_oa
    FOR EACH ROW EXECUTE FUNCTION zalo_commune_oa_guard();

-- ===========================================================================
-- zalo_oa_authorization
-- ===========================================================================

-- ---------------------------------------------------------------------------
-- zalo_oa_authorization_guard — a one-time credential, so the guard is mostly "set once" (0018's
-- pairing code).
--
--   DELETE                  refused. A deleted row frees its state hash, and loses the record of who
--                           asked for an authorization that may have succeeded.
--   a used row              frozen whole. A spent request must never become open again: the `state`
--                           would be replayable.
--   tenant_id, id,          fixed at issue. Moving a request to another commune hands THAT commune the
--   state_hash,             OA the admin authorizes; a longer expiry re-opens a dead link.
--   code_verifier_sealed,
--   requested_at,
--   requested_by,
--   expires_at, created_at
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION zalo_oa_authorization_guard() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION '%: delete refused', TG_TABLE_NAME
            USING HINT = 'An authorization request is closed by used_at and outcome, never removed.';
    END IF;
    IF OLD.used_at IS NOT NULL THEN
        RAISE EXCEPTION '%: a used authorization request is not edited', TG_TABLE_NAME
            USING HINT = 'A spent request must never become open again. Issue a new link.';
    END IF;
    IF NEW.tenant_id            IS DISTINCT FROM OLD.tenant_id
    OR NEW.id                   IS DISTINCT FROM OLD.id
    OR NEW.state_hash           IS DISTINCT FROM OLD.state_hash
    OR NEW.code_verifier_sealed IS DISTINCT FROM OLD.code_verifier_sealed
    OR NEW.requested_at         IS DISTINCT FROM OLD.requested_at
    OR NEW.requested_by         IS DISTINCT FROM OLD.requested_by
    OR NEW.expires_at           IS DISTINCT FROM OLD.expires_at
    OR NEW.created_at           IS DISTINCT FROM OLD.created_at THEN
        RAISE EXCEPTION '%: an issued authorization request is immutable', TG_TABLE_NAME
            USING HINT = 'Commune, state, verifier, who asked and until when are fixed at issue.';
    END IF;
    RETURN NEW;
END $$;

-- ---------------------------------------------------------------------------
-- @entity: ZaloOAAuthorization
-- @scope:  tenant
--
-- zalo_oa_authorization — one authorization request: a member of staff asked for a link
-- (`POST zalo-oas/current/authorization-links`), the OA admin opened it at Zalo, Zalo called back
-- (`GET zalo-oa-authorizations` on the platform's callback host). Lives 10 minutes (Go sets expires_at;
-- the same lifetime as Zalo's authorization code), one use.
--
-- THE CALLBACK HAS NO COMMUNE HOST (ADR 0098 #4): Zalo has ONE callback URL per app, so it arrives on
-- `ZALO_OA_CALLBACK_HOST`, and the commune is found FROM `state`. The lookup by state_hash is therefore
-- cross-commune BY DESIGN — `zalo_oa_authorization_state_hash` below, the one index here that does not
-- lead with tenant_id. OWED BY GO: that read carries `// @cross-tenant:`, returns ONLY the commune's
-- identity (and the row id), and everything after it runs in that commune's context through
-- `core/store`. No match, expired, already used: 404 with one sentence, naming no commune.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS zalo_oa_authorization (
    tenant_id             TEXT        NOT NULL,
    id                    TEXT        NOT NULL,            -- ULID, internal
    -- SHA-256 of the 32-byte CSPRNG `state` placed in the link. NEVER THE STATE ITSELF (see VENDOR
    -- CHOICES): the stored value opens nothing.
    state_hash            BYTEA       NOT NULL,
    -- The PKCE code_verifier, sealed under the commune's DEK: it must be RECOVERED at the callback and
    -- sent to Zalo with the code, so it cannot be a hash.
    code_verifier_sealed  BYTEA       NOT NULL,
    -- Who asked for the link and when: the STAFF business code (`CB-…`, rule 6 invariant 8). The
    -- callback is an act outside any session; this code is the trail's "who" for it (ADR 0098 #12).
    requested_at          TIMESTAMPTZ NOT NULL,
    requested_by          TEXT        NOT NULL,
    expires_at            TIMESTAMPTZ NOT NULL,
    -- SPENT, whatever happened: the callback found this row, or Go closed it as expired. One state, one
    -- try. Paired with outcome — both or neither.
    used_at               TIMESTAMPTZ,
    -- What came of it. OUR values (ADR 0011): 'thanh-cong' (an OA row was written), 'zalo-tu-choi'
    -- (Zalo returned an error to the token exchange), 'khong-kha-dung' (Zalo unreachable),
    -- 'phan-hoi-sai-dang' (Zalo's answer could not be read), 'het-han' (nobody used the link in time),
    -- 'oa-dang-o-xa-khac' (the OA is live in another commune — the floor above refused it),
    -- 'sai-khuon' (the callback's parameters were not the shape Zalo documents).
    outcome               TEXT,
    -- The OA row this request produced; present exactly when outcome = 'thanh-cong'.
    oa_row_id             TEXT,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, id),
    FOREIGN KEY (tenant_id, oa_row_id) REFERENCES zalo_commune_oa (tenant_id, id),
    CONSTRAINT zalo_oa_authorization_state_hash_length CHECK (octet_length(state_hash) = 32),
    CONSTRAINT zalo_oa_authorization_code_verifier_sealed_length
        CHECK (octet_length(code_verifier_sealed) > 29),
    CONSTRAINT zalo_oa_authorization_requested_by_signed
        CHECK (btrim(requested_by) <> '' AND char_length(requested_by) <= 64),
    CONSTRAINT zalo_oa_authorization_expiry_after_request CHECK (expires_at > requested_at),
    CONSTRAINT zalo_oa_authorization_outcome_pair CHECK ((used_at IS NULL) = (outcome IS NULL)),
    CONSTRAINT zalo_oa_authorization_outcome_known
        CHECK (outcome IS NULL
               OR outcome IN ('thanh-cong', 'zalo-tu-choi', 'khong-kha-dung', 'phan-hoi-sai-dang',
                              'het-han', 'oa-dang-o-xa-khac', 'sai-khuon')),
    -- Success records the row it made; nothing else records one.
    CONSTRAINT zalo_oa_authorization_oa_row_on_success
        CHECK ((outcome IS NOT DISTINCT FROM 'thanh-cong') = (oa_row_id IS NOT NULL))
);

-- @scope: cross-tenant — A LOOKUP, BY DESIGN (ADR 0098 #4), unlike the floor on zalo_commune_oa above.
-- Zalo's callback arrives on the platform host with no commune in it; THIS is how the commune is found:
-- state_hash -> (tenant_id, id). Platform-wide and TOTAL (not partial on used_at): a hash once issued is
-- never issued again anywhere, so a replayed `state` meets a used row and is refused, never a fresh one.
-- The same shape as 0018's `zalo_pairing_code_open_by_hash`, with the stronger key the plain table
-- allows (0018 could not, being partitioned at the time). OWED BY GO: the read carries
-- `// @cross-tenant:`, returns only the commune's identity, and everything that follows — the token
-- exchange, the OA row, the audit entry — runs in that commune's context.
CREATE UNIQUE INDEX IF NOT EXISTS zalo_oa_authorization_state_hash
    ON zalo_oa_authorization (state_hash);

-- THE SWEEP'S READ: open requests past their expiry, per commune, closed as 'het-han' by Go. Leads with
-- tenant_id (rule 1); partial on used_at so it stays the size of the requests alive right now.
CREATE INDEX IF NOT EXISTS zalo_oa_authorization_open_by_expiry
    ON zalo_oa_authorization (tenant_id, expires_at)
    WHERE used_at IS NULL;

DROP TRIGGER IF EXISTS zalo_oa_authorization_guard ON zalo_oa_authorization;
CREATE TRIGGER zalo_oa_authorization_guard
    BEFORE UPDATE OR DELETE ON zalo_oa_authorization
    FOR EACH ROW EXECUTE FUNCTION zalo_oa_authorization_guard();

-- ===========================================================================
-- zalo_oa_template
-- ===========================================================================

-- ---------------------------------------------------------------------------
-- zalo_oa_template_guard
--
--   DELETE                 refused. A template Zalo removed is zalo_status = 'DELETE'; the row is the
--                          record of which template carried which milestone, and under which
--                          template_id the ledger (0004) went out.
--   tenant_id, id,         fixed. A template belongs to the OA row it was loaded from; the same
--   oa_row_id,             template_id under a new OA row (re-authorization) is a NEW row.
--   template_id, created_at
--   a changed milestone    refused unless milestone_set_at changed — attach, re-attach and detach all
--   without a new          leave who and when (rule 6 invariant 8); the screen's "gắn lúc … bởi …" must
--   milestone_set_at       never describe an earlier attachment.
--   everything else        Zalo's: rewritten freely by every sync.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION zalo_oa_template_guard() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION '%: delete refused', TG_TABLE_NAME
            USING HINT = 'A template Zalo removed is recorded as zalo_status = DELETE; the row stays.';
    END IF;
    IF NEW.tenant_id   IS DISTINCT FROM OLD.tenant_id
    OR NEW.id          IS DISTINCT FROM OLD.id
    OR NEW.oa_row_id   IS DISTINCT FROM OLD.oa_row_id
    OR NEW.template_id IS DISTINCT FROM OLD.template_id
    OR NEW.created_at  IS DISTINCT FROM OLD.created_at THEN
        RAISE EXCEPTION '%: commune, row, OA and template id are immutable', TG_TABLE_NAME
            USING HINT = 'A template under another OA row, or another template, is a new row.';
    END IF;
    IF NEW.milestone IS DISTINCT FROM OLD.milestone
       AND NEW.milestone_set_at IS NOT DISTINCT FROM OLD.milestone_set_at THEN
        RAISE EXCEPTION '%: a changed milestone needs a new milestone_set_at and milestone_set_by', TG_TABLE_NAME
            USING HINT = 'The screen shows when and by whom the milestone was attached.';
    END IF;
    RETURN NEW;
END $$;

-- ---------------------------------------------------------------------------
-- @entity: ZaloOATemplate
-- @scope:  tenant
--
-- zalo_oa_template — a ZBS template as loaded from the commune's OA (`template/all` + `template/info`,
-- ADR 0098 #8), plus the ONE thing the commune adds: which petition milestone it announces. Everything
-- Zalo owns is overwritten by the next sync; `milestone*` is the commune's and survives syncs.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS zalo_oa_template (
    tenant_id          TEXT        NOT NULL,
    id                 TEXT        NOT NULL,            -- ULID, internal
    -- The OA row this template was loaded from. Retiring that row leaves its templates in place as
    -- history; a new OA row loads its own.
    oa_row_id          TEXT        NOT NULL,
    -- Zalo's template id, as `template/all` returns it.
    template_id        TEXT        NOT NULL,
    template_name      TEXT        NOT NULL DEFAULT '',
    -- ZALO'S OWN VALUES, VERBATIM — the named exception to ADR 0011 (header, VENDOR CHOICES): they are
    -- compared against Zalo's API on every sync; a translation would be a copy that drifts. The column
    -- name says whose value it is. 'DELETE' is Zalo's word for a template that no longer exists there.
    zalo_status        TEXT        NOT NULL,
    zalo_quality       TEXT        NOT NULL DEFAULT 'UNDEFINED',
    -- Zalo's tag (TRANSACTION / CUSTOMER_CARE / PROMOTION today). Not a CHECK: Zalo adds tags without
    -- asking; the sync must be able to record whatever it sends.
    zalo_tag           TEXT        NOT NULL DEFAULT '',
    -- What Zalo declared: an array of {name, required, type, max_length}, written by Go from
    -- `template/info`. The names are what ADR 0098 #9 compares.
    params             JSONB       NOT NULL DEFAULT '[]'::jsonb,
    -- Go's verdict on `params` (ADR 0098 #9): the template declares exactly the three parameters
    -- ma_tra_cuu, moc_nhan, viec_tiep_theo by name. Recomputed at every sync. False = cannot be
    -- attached, and the screen says which name is missing.
    params_match       BOOLEAN     NOT NULL DEFAULT false,
    -- THE COMMUNE'S PART. The petition milestone this template announces: ADR 0041's target statuses
    -- plus ADR 0087's merge / split. NULL = not attached. Trio all or none; set_by is the staff code.
    milestone          TEXT,
    milestone_set_at   TIMESTAMPTZ,
    milestone_set_by   TEXT,
    -- When this row last matched Zalo.
    synced_at          TIMESTAMPTZ NOT NULL,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, id),
    -- One row per template per OA row, in the commune (rule 1, invariant 6).
    UNIQUE (tenant_id, oa_row_id, template_id),
    FOREIGN KEY (tenant_id, oa_row_id) REFERENCES zalo_commune_oa (tenant_id, id),
    CONSTRAINT zalo_oa_template_template_id_shape CHECK (template_id ~ '^[0-9A-Za-z_-]{1,64}$'),
    CONSTRAINT zalo_oa_template_name_length CHECK (char_length(template_name) <= 255),
    CONSTRAINT zalo_oa_template_zalo_status_known
        CHECK (zalo_status IN ('ENABLE', 'PENDING_REVIEW', 'REJECT', 'DISABLE', 'DELETE')),
    CONSTRAINT zalo_oa_template_zalo_quality_known
        CHECK (zalo_quality IN ('HIGH', 'MEDIUM', 'LOW', 'UNDEFINED')),
    CONSTRAINT zalo_oa_template_zalo_tag_length CHECK (char_length(zalo_tag) <= 32),
    CONSTRAINT zalo_oa_template_params_is_array CHECK (jsonb_typeof(params) = 'array'),
    CONSTRAINT zalo_oa_template_milestone_known
        CHECK (milestone IS NULL
               OR milestone IN ('da-tiep-nhan', 'da-chuyen-xu-ly', 'cho-dan-xac-nhan', 'da-dong',
                                'khong-tiep-nhan', 'chuyen-cap-tren', 'gop-phieu', 'tach-phieu')),
    CONSTRAINT zalo_oa_template_milestone_complete
        CHECK ((milestone IS NULL) = (milestone_set_at IS NULL)
               AND (milestone IS NULL) = (milestone_set_by IS NULL)),
    CONSTRAINT zalo_oa_template_milestone_set_by_signed
        CHECK (milestone_set_by IS NULL
               OR (btrim(milestone_set_by) <> '' AND char_length(milestone_set_by) <= 64))
);

-- ONE TEMPLATE PER MILESTONE PER COMMUNE (ADR 0098 #8 "một mốc một mẫu"). Composite with tenant_id;
-- partial on milestone, so every unattached template is outside it. Across OA rows: a retired OA's
-- templates must be DETACHED when the OA is retired (ADR 0098 #11 "gỡ mọi mốc đã gắn" — Go, in the same
-- transaction), or the next OA's template cannot take the milestone. This is MauTinXa's read: WHERE
-- tenant_id = <commune> AND milestone = <status> — then zalo_status / params_match / the OA's liveness,
-- as question 4 says.
CREATE UNIQUE INDEX IF NOT EXISTS zalo_oa_template_one_per_milestone
    ON zalo_oa_template (tenant_id, milestone)
    WHERE milestone IS NOT NULL;

DROP TRIGGER IF EXISTS zalo_oa_template_guard ON zalo_oa_template;
CREATE TRIGGER zalo_oa_template_guard
    BEFORE UPDATE OR DELETE ON zalo_oa_template
    FOR EACH ROW EXECUTE FUNCTION zalo_oa_template_guard();

-- ---------------------------------------------------------------------------
-- WHAT THIS FILE DOES NOT STOP: TRUNCATE and DDL by the table owner — the same line 0003 and ADR 0013
-- draw. Nor an OA row inserted ALREADY retired, nor a template attached to a milestone while not ENABLE
-- (refused by Go at the PATCH; tolerated in the table so a sync can record Zalo's later verdict).
--
-- REVERSAL (question 3). Run by a person, in ONE transaction (ADR 0013). Prose, because a runnable line
-- is a line that gets run.
--
-- LOSSLESS ONLY WHILE ALL THREE TABLES ARE EMPTY. Then, in this order (the foreign keys point at
-- zalo_commune_oa): drop the table zalo_oa_template, then zalo_oa_authorization, then zalo_commune_oa
-- (each one's indexes and trigger go with it); drop the three functions zalo_commune_oa_guard,
-- zalo_oa_authorization_guard, zalo_oa_template_guard; remove this file's row from `schema_migration`
-- (ten = '0031_zalo_commune_oa.sql'). Nothing outside these three tables was changed.
--
-- ONCE ANY ROW EXISTS it is no longer a reversal: an OA row holds a commune's sealed tokens and the
-- record of which OA served it; an authorization row records who asked for a grant the app may still
-- hold at Zalo; a template row records which template announced which milestone, and the ledger (0004)
-- names its template_id. Rule 7 stop condition #1/#2 — the user, a verified backup, and a NEW migration.
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
