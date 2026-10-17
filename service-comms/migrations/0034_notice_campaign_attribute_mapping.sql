-- 0034 — NOTICE CAMPAIGNS MAPPED TO THE RECIPIENT'S OWN ATTRIBUTES (ADR 0101 #12, owner 17/10/2026; TASK E-2).
--
-- A campaign to a DIRECTORY audience may fill a template parameter (a ZNS parameter, an SMS slot) from each
-- recipient's own attribute — `full_name`, `address`, `email`, `household_code` or a dynamic field of the
-- commune (identity's citizen_field). What this file adds, and why it is the shape it is:
--
--   notice_campaign
--     attribute_mapping      {"<param key>": "<attribute code>"} — the param key as `params` keys it (ZNS
--                            parameter name, SMS slot position "1", "2", …), the code as identity's
--                            ListNoticeAttributes names it. CODES ONLY, NEVER A VALUE: the sender asks identity
--                            for each batch's values (RevealNoticeRecipientPhones attribute_codes), renders in
--                            memory, sends, forgets (ADR 0099 #5 held for the attributes as for the number).
--                            NULL when nothing is mapped, and ALWAYS NULL for 'danh-sach-so' / 'tep-nhap' (their
--                            columns are recipient_mapping, 0033). A staff audience maps `full_name` only. Fixed
--                            at the send like the rest of the campaign, so the history shows "Tham số → Thuộc
--                            tính" as it went out.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. ROWS PER COMMUNE: NONE WRITTEN, none rewritten. One NULLable column without a default (no table
--      rewrite), three CHECKs (validated by a scan of a small, new table — 0032 / 0033, the same week), and the
--      guard function replaced to freeze the new column.
--   2. IF IT STOPS HALF-WAY: cannot — one file, one transaction. IF NOT EXISTS / pg_constraint guards /
--      CREATE OR REPLACE: a retry costs nothing.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. READ PATHS THAT CHANGE MEANING: a campaign with attribute_mapping sends a DIFFERENT text per recipient —
--      `params` then holds only the FIXED values (a mapped key has none there), and `template_content` stays
--      the template as it was. Nothing that read the campaign before reads it differently when the column is
--      NULL, which every existing row is.
--   5. RETENTION: nothing dropped, retyped, emptied or renumbered.
--
-- THE LOCK: ACCESS EXCLUSIVE on notice_campaign and its partitions until COMMIT (ADD COLUMN, ADD CONSTRAINT).
--
-- PERSONAL DATA (rule 3): none in this column by construction — a code shape is enforced below, so a value
-- typed into it by mistake is refused, not stored.
--
-- PG FLOOR: 13 (0032's; jsonpath like_regex and keyvalue() are PG 12). Cluster: 16.
-- ---------------------------------------------------------------------------

ALTER TABLE notice_campaign ADD COLUMN IF NOT EXISTS attribute_mapping JSONB;

DO $$
BEGIN
    -- An object of 1..20 entries whose every value is a string shaped like an attribute code (lower-case, digits,
    -- underscore; the static names have underscores, the dynamic ones are [a-z][a-z0-9]{5,7}). COALESCE because
    -- a CHECK that evaluates to NULL passes.
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = 'notice_campaign'::regclass
                   AND conname = 'notice_campaign_attribute_mapping_shape') THEN
        ALTER TABLE notice_campaign ADD CONSTRAINT notice_campaign_attribute_mapping_shape
            CHECK (attribute_mapping IS NULL
                   OR COALESCE(jsonb_typeof(attribute_mapping) = 'object'
                               AND jsonb_array_length(jsonb_path_query_array(attribute_mapping, '$.keyvalue()')) BETWEEN 1 AND 20
                               AND NOT jsonb_path_exists(attribute_mapping,
                                       '$.* ? (@.type() != "string" || !(@ like_regex "^[a-z_][a-z0-9_]{1,31}$"))'),
                               false));
    END IF;

    -- Directory audiences only: typed / imported numbers have no directory row to read an attribute from.
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = 'notice_campaign'::regclass
                   AND conname = 'notice_campaign_attribute_mapping_directory_only') THEN
        ALTER TABLE notice_campaign ADD CONSTRAINT notice_campaign_attribute_mapping_directory_only
            CHECK (attribute_mapping IS NULL OR audience_kind NOT IN ('danh-sach-so', 'tep-nhap'));
    END IF;

    -- A staff audience maps `full_name` only — the staff directory exposes nothing else to a campaign.
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = 'notice_campaign'::regclass
                   AND conname = 'notice_campaign_attribute_mapping_staff_name_only') THEN
        ALTER TABLE notice_campaign ADD CONSTRAINT notice_campaign_attribute_mapping_staff_name_only
            CHECK (attribute_mapping IS NULL OR audience_kind NOT IN ('can-bo-tat-ca', 'can-bo-theo-bo-phan')
                   OR NOT jsonb_path_exists(attribute_mapping, '$.* ? (@ != "full_name")'));
    END IF;
END $$;

-- ---------------------------------------------------------------------------
-- notice_campaign_guard — 0033's, plus attribute_mapping among the fixed-at-send columns.
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
    OR NEW.attribute_mapping IS DISTINCT FROM OLD.attribute_mapping
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
-- WHAT THIS FILE DOES NOT STOP: a mapped code identity no longer lists (comms checks against
-- ListNoticeAttributes at the send; identity resolves retired codes at reveal), a param key that is not one of
-- the template's (Go checks it against the template at the send; a CHECK cannot see the template).
--
-- REVERSAL (question 3). Run by a person, in ONE transaction (ADR 0013). Prose, because a runnable line is a
-- line that gets run.
--
-- LOSSLESS ONLY WHILE NO ROW USES IT. Measure first: notice_campaign rows with attribute_mapping IS NOT NULL
-- must be zero. Then: recreate notice_campaign_guard exactly as 0033 writes it; drop the three constraints this
-- file added; drop the column attribute_mapping (all NULL by the measurement — no data lost); finally remove
-- this file's row from `schema_migration` (ten = '0034_notice_campaign_attribute_mapping.sql').
--
-- ONCE ANY SUCH ROW EXISTS it is no longer a reversal: the mapping is part of the record of what a public
-- authority sent (rule 7 stop conditions #1/#2 — the user, a verified backup, and a NEW migration).
-- ---------------------------------------------------------------------------
