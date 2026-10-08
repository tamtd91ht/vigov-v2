-- 0024 — THE ZALO "SẮP ĐẾN HẠN: NHẮC TRƯỚC" LEAD (ADR 0079 lô 5 Q13, owner 08/10/2026: "Zalo thu hẹp, chuông
-- giữ cột SLA"; and its "Bổ sung 08/10/2026" row: comms filters on receipt, no separate schedule).
--
-- THREE ADDITIONS, NOTHING REWRITTEN:
--
--   zalo_channel_setting.due_soon_days   the commune's lead, 1 to 14 days (the prototype's box). NULL = no
--                                        Zalo filter: the Zalo copy of a due-soon digest is the bell's
--                                        title and body, exactly what every commune receives today.
--   zalo_delivery.due_soon_items         the {code, deadline} list a due-soon digest carried
--                                        (comms.proto StaffNotification.due_soon_items), kept ONLY on the
--                                        owed Zalo row — what the sender narrows to the lead and words the
--                                        message from. NULL = the producer sent none (an old producer, or
--                                        not a due-soon kind): today's message.
--   zalo_delivery skip reason            'ngoai-nhac-truoc' — the digest named records, none inside the
--                                        commune's lead, so nothing was sent on Zalo. A skip is recorded
--                                        with its reason (ADR 0074 "mỗi lần bỏ qua ghi lý do").
--
-- WHY A SECOND, NARROWING THRESHOLD — DELIBERATE (Q13): the bell's due-soon window is the SLA column of
-- each row (identity.v1.ResolveDueSoonCutoff, in working hours) and the producer sends exactly that set.
-- The commune's Zalo lead can only NARROW it: a record outside the bell's window never reaches comms, so
-- a lead longer than the window keeps everything. Nothing here computes a deadline or an overdue state
-- (rule 10, invariants 2 and 3): each deadline is the producer's STORED one, compared with a cut-off.
--
-- WHY THE ITEMS LIVE ON zalo_delivery AND NOT ON staff_notification OR A TABLE OF THEIR OWN:
--   * the bell row is unchanged by Q13 ("the items never narrow, re-word or re-count the bell"); a
--     column there would be bell data nothing on the bell reads.
--   * zalo_delivery already holds exactly one row per (notice, recipient) that Zalo may send, is frozen
--     at creation (0018 guard, extended below), and is what the sender claims. A table keyed by
--     (tenant_id, idempotency_key) would be one more key, one more join and one more guard for a list
--     that today's producers send to ONE recipient per notice (DueSoonNotice: Recipients = [recipient]).
--   * it is not a second copy of anything: comms stores the deadlines nowhere else. 0018's "what was
--     said is not copied here" still holds — the title and body stay the bell's.
--   * written only on a row that is QUEUED ('cho-gui'): a row skipped at enqueue never sends, so it
--     carries none.
--
-- WHY A NEW FILE: core/migrate compares the checksum of every applied file at startup.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. ROWS PER COMMUNE: NONE WRITTEN. Two nullable columns without a default (catalogue-only). Each ADD
--      CONSTRAINT scans its table once to validate: zalo_channel_setting ≤ 1 per commune; zalo_delivery up
--      to low tens of thousands per commune per year (0018).
--   2. IF IT STOPS HALF-WAY: cannot — one file, one transaction. Columns are ADD COLUMN IF NOT EXISTS, new
--      constraints are added behind a pg_constraint lookup BEFORE the old skip-reason CHECK is dropped
--      (DROP … IF EXISTS), and the guard is CREATE OR REPLACE. A retry costs nothing.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. READ PATHS THAT CHANGE MEANING: none. Both columns are NULL on every existing row, and NULL is
--      today's behaviour; the skip-reason CHECK only widens.
--   5. RETENTION: nothing dropped, retyped, emptied or renumbered.
--
-- WHY VALIDATED, NOT `NOT VALID`: every existing row satisfies each new CHECK (the columns are NULL; the
-- skip-reason list is a superset), so the scan cannot fail; and NOT VALID on a partitioned table is not
-- portable across PG versions (0011:332-341, 0017).
--
-- THE LOCK: ACCESS EXCLUSIVE on zalo_channel_setting, zalo_delivery and their partitions until COMMIT — the
-- validating scan of zalo_delivery is the cost. The bell is not touched. On a large deployment, run it in
-- a quiet window.
--
-- PERSONAL DATA (rule 3): none. The items are business codes (a task's `ma`, a petition's `ma_tra_cuu`,
-- a document's code) and deadlines — comms.proto DueSoonItem. PG FLOOR: 13. Cluster: 16.
-- ---------------------------------------------------------------------------

ALTER TABLE zalo_channel_setting ADD COLUMN IF NOT EXISTS due_soon_days SMALLINT;
ALTER TABLE zalo_delivery        ADD COLUMN IF NOT EXISTS due_soon_items JSONB;

DO $$
BEGIN
    -- zalo_channel_setting.due_soon_days: the prototype's 1–14 box. NULL passes (no filter).
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = 'zalo_channel_setting'::regclass
                   AND conname = 'zalo_channel_setting_due_soon_days_range') THEN
        ALTER TABLE zalo_channel_setting ADD CONSTRAINT zalo_channel_setting_due_soon_days_range
            CHECK (due_soon_days BETWEEN 1 AND 14);
    END IF;

    -- zalo_delivery.due_soon_items: a non-empty JSON array of at most 500 items (comms.proto's bound), and
    -- only on a due-soon kind — a deadline list on any other kind is the illegal pair the contract refuses.
    -- The SHAPE of each item is Go's (domain.ValidateDeliveries); this is the floor under it.
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = 'zalo_delivery'::regclass
                   AND conname = 'zalo_delivery_due_soon_items_shape') THEN
        ALTER TABLE zalo_delivery ADD CONSTRAINT zalo_delivery_due_soon_items_shape
            CHECK (due_soon_items IS NULL
                   OR (jsonb_typeof(due_soon_items) = 'array'
                       AND jsonb_array_length(due_soon_items) BETWEEN 1 AND 500
                       AND kind IN ('sap-den-han', 'nhiem-vu.sap-den-han', 'van-ban.sap-den-han',
                                    'phan-anh.sap-den-han')));
    END IF;

    -- zalo_delivery.skip_reason: 0018's four, plus 'ngoai-nhac-truoc'.
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = 'zalo_delivery'::regclass
                   AND conname = 'zalo_delivery_skip_reason_with_lead') THEN
        ALTER TABLE zalo_delivery ADD CONSTRAINT zalo_delivery_skip_reason_with_lead
            CHECK (skip_reason IS NULL
                   OR skip_reason IN ('chua-lien-ket', 'kenh-tat', 'loai-tat', 'bot-chua-cau-hinh',
                                      'ngoai-nhac-truoc'));
    END IF;
END $$;

-- The new CHECK is in place; the narrower one it subsumes goes. Dropped on the partitioned parent, it goes
-- from every partition with it (inherited through PARTITION OF, never declared locally).
ALTER TABLE zalo_delivery DROP CONSTRAINT IF EXISTS zalo_delivery_skip_reason_known;

-- ---------------------------------------------------------------------------
-- zalo_delivery_guard — 0018's body VERBATIM, with ONE line added: due_soon_items is fixed at creation
-- like the other identity columns. What a reminder was about is evidence of what the commune was told;
-- a list rewritten after the row was queued would make the sent message and its record disagree.
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
    OR NEW.due_soon_items  IS DISTINCT FROM OLD.due_soon_items
    OR NEW.created_at      IS DISTINCT FROM OLD.created_at THEN
        RAISE EXCEPTION '%: a queued delivery is immutable', TG_TABLE_NAME
            USING HINT = 'Which notice, to whom, of which kind, about which records and when queued are fixed.';
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

-- The trigger itself (0018) is unchanged: it names the function, and the function is replaced in place.

-- ---------------------------------------------------------------------------
-- REVERSAL (migration question 3). Run by a person, in ONE transaction (ADR 0013). Prose, not runnable.
--
-- LOSSLESS ONLY WHILE no zalo_channel_setting row has due_soon_days set, no zalo_delivery row has
-- due_soon_items set, and no zalo_delivery row has skip_reason = 'ngoai-nhac-truoc' (three counts, all
-- zero). Then: re-create 0018's zalo_delivery_guard VERBATIM, re-add 0018's zalo_delivery_skip_reason_known
-- VERBATIM, drop the three constraints this file added and the two columns, and remove this file's row
-- from `schema_migration` (ten = '0024_zalo_due_soon_lead.sql').
--
-- ONCE ANY ROW HOLDS ONE OF THEM it is no longer a reversal: a commune's chosen lead is its configuration
-- (dropping it changes what its staff receive, with no audit entry), and a delivery's items and skip
-- reason are the record of what was — or was not — sent (rule 7, forbidden #5). That is a new migration
-- and the user's call.
-- ---------------------------------------------------------------------------
