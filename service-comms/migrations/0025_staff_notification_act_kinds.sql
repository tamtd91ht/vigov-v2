-- 0025 — STAFF NOTICES FIXED BY ONE ACT OF A PERSON (ADR 0079 Q3 phase 2, ADR 0086; contract committed in
-- d0b02227, comms.proto StaffNotificationKind 18–25). Phase 1 (0021) admitted every kind that already had a
-- producer; this file admits the nine phase-2 kinds whose producers the contract now names.
--
--   kind                        producer / shape                                  wire (comms.proto)
--   nhiem-vu.giao-moi           petitions, task created with / handed to someone  TASK_ASSIGNED = 18
--   nhiem-vu.de-nghi-lui-han    petitions, an extension request awaits decision   TASK_EXTENSION_REQUESTED = 19
--   nhiem-vu.cho-duyet          petitions, a task moved to `cho-duyet`            TASK_APPROVAL_REQUESTED = 20
--   nhiem-vu.nhac-ten           petitions, mentioned in a task's timeline entry   TASK_MENTION = 21
--   van-ban.chuyen-toi          documents, document / letter handed to an officer DOCUMENT_ASSIGNED = 22
--   phan-anh.phan-cong          petitions, a petition assigned to an officer      PETITION_ASSIGNED = 23
--   phan-anh.mo-lai             petitions, a petition reopened on a 1–2 star vote PETITION_REOPENED = 24
--   bao-cao.san-sang            identity's scheduled_reports job, report ready    REPORT_READY = 25
--   thong-bao.moi               comms ITSELF, an internal announcement issued     (no wire value, on purpose)
--
-- Values Vietnamese without diacritics, `<domain>.<shape>` as 0021's kinds (ADR 0011, ADR 0051 table).
--
-- WIDENS THREE CHECKS, additively, and every one of the nine goes into all three:
--   staff_notification.kind       0023 staff_notification_kind_with_bell_only
--   zalo_channel_setting.kinds    0021 zalo_channel_setting_kinds_per_domain
--   zalo_delivery.kind            0021 zalo_delivery_kind_per_domain
--
-- NOT BELL-ONLY, UNLIKE 0023's `giai-ngan.nhac-ten`: comms.proto says of 18–24 "comms queues a Zalo delivery
-- for these when the commune selected the kind and the recipient is linked", and the card for this file
-- extends the same to 25 and to `thong-bao.moi`. So each is a box a commune may tick, and a delivery row may
-- carry it. `giai-ngan.nhac-ten` stays OUT of both Zalo CHECKs — ADR 0081 #5 is unchanged here.
--
-- UNTOUCHED, AND STILL RIGHT:
--   zalo_channel_setting_overdue_kinds_need_cadence (0021) — none of the nine is an overdue kind; each is a
--     one-shot notice keyed by the act, so none needs a cadence.
--   zalo_delivery_due_soon_items_shape (0024) — none of the nine is due soon; a deadline list on one of them
--     stays refused.
--   every guard — they freeze `kind`, whatever its value.
--
-- WHY A NEW FILE: core/migrate compares the checksum of every applied file at startup. 0021 and 0023 are not
-- edited.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. ROWS PER COMMUNE: NONE WRITTEN. Each ADD CONSTRAINT scans its table once to validate:
--      zalo_channel_setting ≤ 1 per commune; staff_notification and zalo_delivery up to low tens of
--      thousands per commune per year (0010, 0018).
--   2. IF IT STOPS HALF-WAY: cannot — one file, one transaction. Each new constraint is added behind a
--      pg_constraint lookup BEFORE the old one is dropped (DROP … IF EXISTS), so no column is ever
--      unconstrained, and a retry costs nothing. No backfill, nothing to resume.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. READ PATHS THAT CHANGE MEANING: none. Every CHECK only widens; no row changes. The dispatcher's
--      `n.kind = ANY (s.kinds)` compares the same values it compared yesterday; a commune's stored
--      selection admits none of the nine until it ticks one.
--   5. RETENTION: nothing dropped, retyped, emptied or renumbered.
--
-- WHY VALIDATED, NOT `NOT VALID`: each new CHECK admits every value the old one did, so the scan cannot
-- fail; and NOT VALID on a partitioned table is not portable across PG versions (0011:332-341, 0017).
--
-- THE LOCK: ACCESS EXCLUSIVE on staff_notification, zalo_channel_setting, zalo_delivery and their partitions
-- until COMMIT — the validating scans of staff_notification and zalo_delivery are the cost. The bell inbox
-- waits for them. On a large deployment, run it in a quiet window.
--
-- PERSONAL DATA (rule 3): none. PG FLOOR: 13. Cluster: 16.
-- ---------------------------------------------------------------------------

DO $$
BEGIN
    -- staff_notification.kind: 0023's list + the nine. ---------------------------------------------
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = 'staff_notification'::regclass
                   AND conname = 'staff_notification_kind_with_act_notices') THEN
        ALTER TABLE staff_notification ADD CONSTRAINT staff_notification_kind_with_act_notices
            CHECK (kind IN ('sap-den-han', 'qua-han', 'leo-thang', 'ban-tin-tuan',
                            'nhiem-vu.sap-den-han', 'nhiem-vu.qua-han', 'nhiem-vu.chua-cu-nguoi', 'nhiem-vu.leo-thang',
                            'van-ban.sap-den-han', 'van-ban.qua-han', 'van-ban.chua-cu-nguoi', 'van-ban.leo-thang',
                            'phan-anh.sap-den-han', 'phan-anh.qua-han', 'phan-anh.chua-cu-nguoi', 'phan-anh.leo-thang',
                            'giai-ngan.nhac-ten',
                            'nhiem-vu.giao-moi', 'nhiem-vu.de-nghi-lui-han', 'nhiem-vu.cho-duyet', 'nhiem-vu.nhac-ten',
                            'van-ban.chuyen-toi', 'phan-anh.phan-cong', 'phan-anh.mo-lai', 'bao-cao.san-sang',
                            'thong-bao.moi'));
    END IF;

    -- zalo_channel_setting.kinds: 0021's list + the nine (never 'giai-ngan.nhac-ten'). -------------
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = 'zalo_channel_setting'::regclass
                   AND conname = 'zalo_channel_setting_kinds_with_act_notices') THEN
        ALTER TABLE zalo_channel_setting ADD CONSTRAINT zalo_channel_setting_kinds_with_act_notices
            CHECK (kinds <@ ARRAY['sap-den-han', 'qua-han', 'leo-thang', 'ban-tin-tuan',
                                  'nhiem-vu.sap-den-han', 'nhiem-vu.qua-han', 'nhiem-vu.chua-cu-nguoi', 'nhiem-vu.leo-thang',
                                  'van-ban.sap-den-han', 'van-ban.qua-han', 'van-ban.chua-cu-nguoi', 'van-ban.leo-thang',
                                  'phan-anh.sap-den-han', 'phan-anh.qua-han', 'phan-anh.chua-cu-nguoi', 'phan-anh.leo-thang',
                                  'nhiem-vu.giao-moi', 'nhiem-vu.de-nghi-lui-han', 'nhiem-vu.cho-duyet', 'nhiem-vu.nhac-ten',
                                  'van-ban.chuyen-toi', 'phan-anh.phan-cong', 'phan-anh.mo-lai', 'bao-cao.san-sang',
                                  'thong-bao.moi']::text[]);
    END IF;

    -- zalo_delivery.kind: 0021's list + the nine (never 'giai-ngan.nhac-ten'). ---------------------
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = 'zalo_delivery'::regclass
                   AND conname = 'zalo_delivery_kind_with_act_notices') THEN
        ALTER TABLE zalo_delivery ADD CONSTRAINT zalo_delivery_kind_with_act_notices
            CHECK (kind IN ('sap-den-han', 'qua-han', 'leo-thang', 'ban-tin-tuan', 'thu-nghiem',
                            'nhiem-vu.sap-den-han', 'nhiem-vu.qua-han', 'nhiem-vu.chua-cu-nguoi', 'nhiem-vu.leo-thang',
                            'van-ban.sap-den-han', 'van-ban.qua-han', 'van-ban.chua-cu-nguoi', 'van-ban.leo-thang',
                            'phan-anh.sap-den-han', 'phan-anh.qua-han', 'phan-anh.chua-cu-nguoi', 'phan-anh.leo-thang',
                            'nhiem-vu.giao-moi', 'nhiem-vu.de-nghi-lui-han', 'nhiem-vu.cho-duyet', 'nhiem-vu.nhac-ten',
                            'van-ban.chuyen-toi', 'phan-anh.phan-cong', 'phan-anh.mo-lai', 'bao-cao.san-sang',
                            'thong-bao.moi'));
    END IF;
END $$;

-- The new constraints are in place; the narrower ones they subsume go. Dropped on the partitioned parent,
-- each goes from every partition with it (inherited through PARTITION OF, never declared locally).
ALTER TABLE staff_notification   DROP CONSTRAINT IF EXISTS staff_notification_kind_with_bell_only;
ALTER TABLE zalo_channel_setting DROP CONSTRAINT IF EXISTS zalo_channel_setting_kinds_per_domain;
ALTER TABLE zalo_delivery        DROP CONSTRAINT IF EXISTS zalo_delivery_kind_per_domain;

-- ---------------------------------------------------------------------------
-- REVERSAL (migration question 3). Run by a person, in ONE transaction (ADR 0013). Prose, not runnable.
--
-- LOSSLESS ONLY WHILE no row of the three tables holds one of the nine values (three counts: kind IN the
-- nine on staff_notification and zalo_delivery; kinds && ARRAY[the nine] on zalo_channel_setting; all zero).
-- Then: re-add 0023's staff_notification_kind_with_bell_only and 0021's zalo_channel_setting_kinds_per_domain
-- and zalo_delivery_kind_per_domain VERBATIM, drop the three constraints this file added, and remove this
-- file's row from `schema_migration` (ten = '0025_staff_notification_act_kinds.sql').
--
-- ONCE ANY ROW HOLDS ONE, the old CHECKs refuse to be re-added — and they should: notices and deliveries are
-- immutable records (0010, 0018 guards), and rewriting their kind is editing history (rule 7 forbidden #5).
-- A settings row could drop the ticked kinds, but only with a system audit entry per commune and only by a
-- NEW migration — the user's call.
-- ---------------------------------------------------------------------------
