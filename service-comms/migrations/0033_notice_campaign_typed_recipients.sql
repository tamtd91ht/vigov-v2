-- 0033 — NOTICE CAMPAIGNS TO TYPED / IMPORTED NUMBERS (ADR 0099 §Sửa đổi 17/10/2026, S1–S7; TASK D-7).
--
-- A campaign may now go to numbers a member of staff TYPED (audience 'danh-sach-so', ≤ 1 000) or IMPORTED
-- from a file read in the browser (audience 'tep-nhap', ≤ 10 000). Those numbers are not in any directory,
-- so the 0032 shape — "the ledger holds a directory ref + the masked number, identity reveals the number at
-- send time" — cannot carry them. What this file adds, and why each piece is the shape it is:
--
--   notice_campaign
--     audience_kind          + 'danh-sach-so', 'tep-nhap' (ADR 0011 values); audience_ids empty for both
--     template_ref           NULL for SMS to typed / imported numbers — the owner chose free text there, no
--                            eSMS template (S3: "không kiểm khớp mẫu eSMS trước — dòng sai mẫu do eSMS từ
--                            chối"); NOT NULL everywhere else, as 0032
--     sms_content_mode       'cot-trong-tep' (each row's own text) | 'noi-dung-chung' (one text for all, with
--                            optional {{variable_i}}) — present exactly for SMS to typed / imported numbers
--     template_content       for 'noi-dung-chung' the SHARED text staff typed (variables unexpanded); NULL for
--                            'cot-trong-tep' (there is no shared text); unchanged otherwise
--     invalid_count          the rows whose number did not normalise (S5: they are kept, 'khong-hop-le');
--                            0 for the directory audiences
--     recipient_mapping      'tep-nhap' only: which file column (its row-1 HEADER) fed the number, each
--                            row's text, each {{variable_i}}, each ZNS parameter — owner 17/10/2026 "lịch sử
--                            nên lưu thông tin map biến". Headers, never a row's value; NULL for every other
--                            audience; fixed at the send like the rest of the campaign
--   notice_recipient
--     recipient_kind         + 'so-nhap' — a typed / imported number; recipient_ref is THE ROW'S OWN ULID
--                            (nothing to point at), so 0032's (campaign, ref) key still holds; the app drops
--                            duplicate normalised numbers before the insert
--     status                 + 'khong-hop-le' — the number did not normalise: final at insert, never sent
--     phone_sealed           S6: the full number, SEALED under the commune's DEK, only while the row is owed
--     payload_sealed         S6: this row's own message (SMS text, or the ZNS parameters merged for it), JSON,
--                            sealed, only while owed
--     error_message          S5: the provider's message on 'that-bai', ≤ 500 characters, digit runs of 7+
--                            masked by Go before it is written and REFUSED here if one is left
--     failure_class          CLOSED to the list internal/domain names (owed since 0032: "OWED BY THE GO CARD")
--
-- WHY THE FULL NUMBER IS STORED AT ALL — the one place 0032's "never the full number" bends: a typed or
-- imported number has no directory row identity could reveal it from, and the owner decided (S6) to keep it
-- "mã hoá bằng khoá của xã … xoá bản mã hoá khi dòng có kết quả cuối". So it is SEALED (never plain), it is
-- bound to its row (AAD below), and the DATABASE refuses a finished row that still holds it.
--
-- AAD OWED BY GO (0032's discipline): additional data naming the FIELD, the COMMUNE and the ROW —
--     phone    "notice_recipient/phone/<tenant_id>/<id>"
--     payload  "notice_recipient/payload/<tenant_id>/<id>"
-- Bytes copied onto another row, into another commune, or from one field to the other do not open.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. ROWS PER COMMUNE: NONE WRITTEN, none rewritten. Three columns added to notice_recipient (NULLable,
--      no default — no table rewrite), three to notice_campaign (sms_content_mode and recipient_mapping NULLable; invalid_count
--      NOT NULL DEFAULT 0, a constant default — no rewrite from PostgreSQL 11). One NOT NULL relaxed. CHECKs
--      replaced by WIDER ones under new names, then the narrower ones dropped (0025's order). One
--      non-unique index added. Steady state: up to 10 000 notice_recipient rows per imported campaign — the
--      reason the sealed columns are emptied at the outcome rather than kept.
--   2. IF IT STOPS HALF-WAY: cannot — one file, one transaction. Every statement is IF [NOT] EXISTS / CREATE
--      OR REPLACE / guarded by pg_constraint, so a retry costs nothing.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. READ PATHS THAT CHANGE MEANING:
--        * "recipient_count" stays THE ROWS TO SEND; an imported campaign's unparseable numbers are in
--          invalid_count, never in recipient_count or failed_count. Every 0032 CHECK on the counts holds.
--        * A campaign that sent every valid row but had invalid ones is 'gui-mot-phan', not 'da-gui'
--          (S5: the staff member must see that some numbers were never reached). Go's settle statement.
--        * template_ref may be NULL — a reader that joins it to sms_template or ZBS must expect none.
--        * A 'so-nhap' row has no directory ref: the sender must NOT ask identity for its number; it opens
--          phone_sealed. A sender that reveals by ref would ask identity for a ULID of ours.
--        * failure_class is now a closed list: a class Go invents later is refused until a migration adds it.
--   5. RETENTION: nothing dropped, retyped, emptied or renumbered by this file. The sealed columns are
--      emptied by Go at each row's outcome — that IS the owner's retention decision (S6), and the guard
--      lets them only go from a value to NULL, never be rewritten.
--
-- THE LOCK: ACCESS EXCLUSIVE on notice_campaign, notice_recipient and their partitions until COMMIT; the
-- new CHECKs validate by scanning. Both tables are new (0032, the same day) and small.
--
-- PERSONAL DATA (rule 3): phone_sealed / payload_sealed hold personal data SEALED only, and only while the
-- row is owed. phone_masked keeps 0032's CHECK (≤ 6 digits, a '*'). error_message is refused if it still
-- carries a run of 7 digits. NEVER log a number, a payload, or a provider message.
--
-- PG FLOOR: 13 (0032's). Cluster: 16.
-- ---------------------------------------------------------------------------

-- ===========================================================================
-- notice_campaign
-- ===========================================================================

ALTER TABLE notice_campaign ADD COLUMN IF NOT EXISTS sms_content_mode TEXT;
ALTER TABLE notice_campaign ADD COLUMN IF NOT EXISTS invalid_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE notice_campaign ADD COLUMN IF NOT EXISTS recipient_mapping JSONB;

-- SMS to typed / imported numbers carries no template (S3). Relaxed on the parent; the partitions follow.
-- The CHECK below puts the NOT NULL back for every other campaign.
ALTER TABLE notice_campaign ALTER COLUMN template_ref DROP NOT NULL;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = 'notice_campaign'::regclass
                   AND conname = 'notice_campaign_audience_kind_with_imports') THEN
        ALTER TABLE notice_campaign ADD CONSTRAINT notice_campaign_audience_kind_with_imports
            CHECK (audience_kind IN ('nguoi-dan-tat-ca', 'nguoi-dan-theo-nhom', 'nguoi-dan-theo-nhan', 'can-bo-tat-ca',
                                     'can-bo-theo-bo-phan', 'danh-sach-so', 'tep-nhap'));
    END IF;

    -- An array; empty exactly for the two "all" kinds and the two typed kinds. CASE so a non-array is
    -- refused, never an error.
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = 'notice_campaign'::regclass
                   AND conname = 'notice_campaign_audience_ids_shape_with_imports') THEN
        ALTER TABLE notice_campaign ADD CONSTRAINT notice_campaign_audience_ids_shape_with_imports
            CHECK (CASE WHEN jsonb_typeof(audience_ids) <> 'array' THEN false
                        WHEN audience_kind IN ('nguoi-dan-tat-ca', 'can-bo-tat-ca', 'danh-sach-so', 'tep-nhap')
                            THEN jsonb_array_length(audience_ids) = 0
                        ELSE jsonb_array_length(audience_ids) > 0 END);
    END IF;

    -- NULL exactly for SMS to typed / imported numbers; otherwise 0032's shape. COALESCE because a CHECK
    -- that evaluates to NULL passes.
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = 'notice_campaign'::regclass
                   AND conname = 'notice_campaign_template_ref_by_audience') THEN
        ALTER TABLE notice_campaign ADD CONSTRAINT notice_campaign_template_ref_by_audience
            CHECK (CASE WHEN channel = 'sms' AND audience_kind IN ('danh-sach-so', 'tep-nhap') THEN template_ref IS NULL
                        ELSE COALESCE(btrim(template_ref) <> '' AND char_length(template_ref) <= 64, false) END);
    END IF;

    -- Present exactly for SMS to typed / imported numbers, and then one of the two modes.
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = 'notice_campaign'::regclass
                   AND conname = 'notice_campaign_sms_content_mode_known') THEN
        ALTER TABLE notice_campaign ADD CONSTRAINT notice_campaign_sms_content_mode_known
            CHECK (CASE WHEN channel = 'sms' AND audience_kind IN ('danh-sach-so', 'tep-nhap')
                            THEN COALESCE(sms_content_mode IN ('cot-trong-tep', 'noi-dung-chung'), false)
                        ELSE sms_content_mode IS NULL END);
    END IF;

    -- The SMS text kept as sent: ZNS none; SMS to a directory audience the template text (0032); SMS to
    -- typed / imported numbers the shared text for 'noi-dung-chung', none for 'cot-trong-tep' (each row's
    -- text is its own, sealed in its row, gone at the outcome).
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = 'notice_campaign'::regclass
                   AND conname = 'notice_campaign_template_content_by_mode') THEN
        ALTER TABLE notice_campaign ADD CONSTRAINT notice_campaign_template_content_by_mode
            CHECK (CASE WHEN channel = 'zalo-zns' THEN template_content IS NULL
                        WHEN audience_kind IN ('danh-sach-so', 'tep-nhap')
                            THEN (COALESCE(sms_content_mode, '') = 'noi-dung-chung') = (template_content IS NOT NULL)
                        ELSE template_content IS NOT NULL END);
    END IF;

    -- Unparseable numbers exist only where numbers were typed or imported.
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = 'notice_campaign'::regclass
                   AND conname = 'notice_campaign_invalid_count_shape') THEN
        ALTER TABLE notice_campaign ADD CONSTRAINT notice_campaign_invalid_count_shape
            CHECK (invalid_count >= 0 AND (invalid_count = 0 OR audience_kind IN ('danh-sach-so', 'tep-nhap')));
    END IF;

    -- An imported file's mapping: present exactly for 'tep-nhap', an object naming at least the phone
    -- column. COALESCE because a CHECK that evaluates to NULL passes.
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = 'notice_campaign'::regclass
                   AND conname = 'notice_campaign_recipient_mapping_shape') THEN
        ALTER TABLE notice_campaign ADD CONSTRAINT notice_campaign_recipient_mapping_shape
            CHECK (CASE WHEN audience_kind = 'tep-nhap'
                            THEN COALESCE(jsonb_typeof(recipient_mapping) = 'object'
                                          AND jsonb_typeof(recipient_mapping -> 'phone_column') = 'string', false)
                        ELSE recipient_mapping IS NULL END);
    END IF;
END $$;

-- The wider constraints are in place; the narrower 0032 ones they replace go. Dropped on the partitioned
-- parent, each goes from every partition with it.
ALTER TABLE notice_campaign DROP CONSTRAINT IF EXISTS notice_campaign_audience_kind_known;
ALTER TABLE notice_campaign DROP CONSTRAINT IF EXISTS notice_campaign_audience_ids_shape;
ALTER TABLE notice_campaign DROP CONSTRAINT IF EXISTS notice_campaign_template_ref_shape;
ALTER TABLE notice_campaign DROP CONSTRAINT IF EXISTS notice_campaign_template_content_for_sms;

-- ---------------------------------------------------------------------------
-- notice_campaign_guard — 0032's, plus the three new fixed-at-send columns (sms_content_mode, invalid_count,
-- recipient_mapping).
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
    OR NEW.sms_content_mode IS DISTINCT FROM OLD.sms_content_mode
    OR NEW.params           IS DISTINCT FROM OLD.params
    OR NEW.audience_kind    IS DISTINCT FROM OLD.audience_kind
    OR NEW.audience_ids     IS DISTINCT FROM OLD.audience_ids
    OR NEW.recipient_count  IS DISTINCT FROM OLD.recipient_count
    OR NEW.skipped_count    IS DISTINCT FROM OLD.skipped_count
    OR NEW.invalid_count    IS DISTINCT FROM OLD.invalid_count
    OR NEW.recipient_mapping IS DISTINCT FROM OLD.recipient_mapping
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

-- ===========================================================================
-- notice_recipient
-- ===========================================================================

ALTER TABLE notice_recipient ADD COLUMN IF NOT EXISTS phone_sealed BYTEA;
ALTER TABLE notice_recipient ADD COLUMN IF NOT EXISTS payload_sealed BYTEA;
ALTER TABLE notice_recipient ADD COLUMN IF NOT EXISTS error_message TEXT;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = 'notice_recipient'::regclass
                   AND conname = 'notice_recipient_kind_with_typed') THEN
        ALTER TABLE notice_recipient ADD CONSTRAINT notice_recipient_kind_with_typed
            CHECK (recipient_kind IN ('nguoi-dan', 'can-bo', 'so-nhap'));
    END IF;

    --   khong-hop-le   the typed / imported number did not normalise: written final, never sent (S5)
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = 'notice_recipient'::regclass
                   AND conname = 'notice_recipient_status_with_invalid') THEN
        ALTER TABLE notice_recipient ADD CONSTRAINT notice_recipient_status_with_invalid
            CHECK (status IN ('cho-gui', 'da-gui', 'that-bai', 'bo-qua', 'khong-hop-le'));
    END IF;

    -- THE CLOSED CLASS LIST 0032 left owed — internal/domain NoticeFailureClasses(), in its order. A class
    -- Go adds later needs a migration first; the shape CHECK of 0032 stays as the floor under it.
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = 'notice_recipient'::regclass
                   AND conname = 'notice_recipient_failure_class_known') THEN
        ALTER TABLE notice_recipient ADD CONSTRAINT notice_recipient_failure_class_known
            CHECK (failure_class IS NULL
                   OR failure_class IN ('sai-khoa', 'brandname-chua-kich-hoat', 'sai-mau', 'khong-kha-dung',
                                        'gioi-han-tan-suat', 'bi-tu-choi', 'phan-hoi-sai-dang', 'so-khong-hop-le',
                                        'oa-can-uy-quyen-lai', 'nguon-ngung-dung', 'nguoi-nhan-ngung-lien-he'));
    END IF;

    -- S6 IN ONE LINE: the sealed number and message exist EXACTLY while a typed / imported row is owed. A
    -- directory row never has them (its number is identity's); a finished row ('da-gui', 'that-bai',
    -- 'bo-qua', 'khong-hop-le') never keeps them — the outcome UPDATE must empty both.
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = 'notice_recipient'::regclass
                   AND conname = 'notice_recipient_sealed_exactly_while_owed') THEN
        ALTER TABLE notice_recipient ADD CONSTRAINT notice_recipient_sealed_exactly_while_owed
            CHECK (CASE WHEN recipient_kind = 'so-nhap' AND status = 'cho-gui'
                            THEN phone_sealed IS NOT NULL AND payload_sealed IS NOT NULL
                        ELSE phone_sealed IS NULL AND payload_sealed IS NULL END);
    END IF;

    -- 1 + 12 + 16 bytes of envelope overhead, plus at least one byte (0008) — a sealed value, never a bare
    -- number.
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = 'notice_recipient'::regclass
                   AND conname = 'notice_recipient_sealed_length') THEN
        ALTER TABLE notice_recipient ADD CONSTRAINT notice_recipient_sealed_length
            CHECK ((phone_sealed IS NULL OR octet_length(phone_sealed) > 29)
               AND (payload_sealed IS NULL OR octet_length(payload_sealed) > 29));
    END IF;

    -- An unparseable number is only ever a typed / imported one, and it never reached a provider.
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = 'notice_recipient'::regclass
                   AND conname = 'notice_recipient_invalid_never_sent') THEN
        ALTER TABLE notice_recipient ADD CONSTRAINT notice_recipient_invalid_never_sent
            CHECK (status <> 'khong-hop-le'
                   OR (recipient_kind = 'so-nhap' AND attempts = 0 AND provider_message_id IS NULL
                       AND failure_class IS NULL));
    END IF;

    -- The provider's own words, on a failure only (S5), bounded, and NEVER carrying a phone-length digit
    -- run: Go masks runs of 7+ digits before writing; this refuses what slipped through.
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = 'notice_recipient'::regclass
                   AND conname = 'notice_recipient_error_message_shape') THEN
        ALTER TABLE notice_recipient ADD CONSTRAINT notice_recipient_error_message_shape
            CHECK (error_message IS NULL
                   OR (status = 'that-bai' AND btrim(error_message) <> '' AND char_length(error_message) <= 500
                       AND error_message !~ '[0-9]{7}'));
    END IF;
END $$;

ALTER TABLE notice_recipient DROP CONSTRAINT IF EXISTS notice_recipient_kind_known;
ALTER TABLE notice_recipient DROP CONSTRAINT IF EXISTS notice_recipient_status_known;

-- ---------------------------------------------------------------------------
-- notice_recipient_guard — 0032's, plus:
--
--   phone_sealed,       may only go from a value to NULL — never be set, swapped or restored by an UPDATE.
--   payload_sealed      They are written at INSERT (the campaign's transaction) and emptied at the outcome.
--   error_message       part of a finished row's outcome: frozen with it.
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
    IF (NEW.phone_sealed IS DISTINCT FROM OLD.phone_sealed AND NEW.phone_sealed IS NOT NULL)
    OR (NEW.payload_sealed IS DISTINCT FROM OLD.payload_sealed AND NEW.payload_sealed IS NOT NULL) THEN
        RAISE EXCEPTION '%: a sealed number or message may only be emptied', TG_TABLE_NAME
            USING HINT = 'They are written with the campaign and removed at the outcome (ADR 0099 S6).';
    END IF;
    IF OLD.status <> 'cho-gui'
       AND (NEW.status              IS DISTINCT FROM OLD.status
         OR NEW.provider_message_id IS DISTINCT FROM OLD.provider_message_id
         OR NEW.failure_class       IS DISTINCT FROM OLD.failure_class
         OR NEW.error_message       IS DISTINCT FROM OLD.error_message
         OR NEW.attempts            IS DISTINCT FROM OLD.attempts
         OR NEW.lease_until         IS DISTINCT FROM OLD.lease_until
         OR NEW.sent_at             IS DISTINCT FROM OLD.sent_at) THEN
        RAISE EXCEPTION '%: a finished recipient keeps its outcome', TG_TABLE_NAME
            USING HINT = 'Sent, failed, skipped and invalid are final. Moving a sent row back would send it '
                         'a second time.';
    END IF;
    IF NEW.attempts < OLD.attempts THEN
        RAISE EXCEPTION '%: attempts cannot decrease', TG_TABLE_NAME
            USING HINT = 'The attempt count is evidence of how hard the channel was tried.';
    END IF;
    RETURN NEW;
END $$;

-- THE PER-NUMBER HISTORY of one campaign (GET notice-campaigns/{id}/recipients), keyset on the row id —
-- rows of one campaign share created_at (one transaction). Leads with tenant_id (rule 1).
CREATE INDEX IF NOT EXISTS notice_recipient_by_campaign
    ON notice_recipient (tenant_id, campaign_id, id);

-- ---------------------------------------------------------------------------
-- WHAT THIS FILE DOES NOT STOP: a campaign whose invalid_count disagrees with its 'khong-hop-le' rows (Go
-- writes both in one transaction; a CHECK cannot see other rows); a sealed value that is not what its AAD
-- says (only Go's Open can tell).
--
-- REVERSAL (question 3). Run by a person, in ONE transaction (ADR 0013). Prose, because a runnable line is a
-- line that gets run.
--
-- LOSSLESS ONLY WHILE NO ROW USES WHAT THIS FILE ADDED. Measure first, all must be zero:
--   notice_campaign   audience_kind IN ('danh-sach-so', 'tep-nhap'), or template_ref IS NULL, or
--                     recipient_mapping IS NOT NULL;
--   notice_recipient  recipient_kind = 'so-nhap', or status = 'khong-hop-le', or error_message IS NOT NULL.
-- Then: re-add 0032's notice_campaign_audience_kind_known, notice_campaign_audience_ids_shape,
-- notice_campaign_template_ref_shape, notice_campaign_template_content_for_sms, notice_recipient_kind_known
-- and notice_recipient_status_known VERBATIM; drop the fourteen constraints this file added; SET NOT NULL on
-- notice_campaign.template_ref; recreate notice_campaign_guard and notice_recipient_guard exactly as 0032
-- writes them; drop the index notice_recipient_by_campaign; then drop the columns sms_content_mode,
-- invalid_count, recipient_mapping, phone_sealed, payload_sealed, error_message (empty or all-default by the measurement above —
-- no data lost); finally remove this file's row from `schema_migration`
-- (ten = '0033_notice_campaign_typed_recipients.sql').
--
-- ONCE ANY SUCH ROW EXISTS it is no longer a reversal: those rows are the record of what a public authority
-- sent to which numbers, and which numbers it could not reach (S5). Rule 7 stop conditions #1/#2 — the user,
-- a verified backup, and a NEW migration.
-- ---------------------------------------------------------------------------
