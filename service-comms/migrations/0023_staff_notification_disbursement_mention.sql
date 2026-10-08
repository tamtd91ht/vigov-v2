-- 0023 — A BELL-ONLY STAFF NOTIFICATION KIND: MENTIONED IN A DISBURSEMENT PROJECT'S DISCUSSION
-- (ADR 0081 #5, owner 08/10/2026: "nhắc tên trong Trao đổi của dự án tạo thông báo CHUÔNG web-admin …
-- không gửi Zalo"; prototype vigov-require apps/api/app/modules/budget/service.py:1065-1076).
--
--   kind                 producer / shape                                 wire (comms.proto)
--   giai-ngan.nhac-ten   service-finance, a mention in a project's        STAFF_NOTIFICATION_KIND_
--                        "Trao đổi", one notice per person mentioned      DISBURSEMENT_MENTION = 17
--
-- Value Vietnamese without diacritics, `<domain>.<shape>` as 0021's kinds (ADR 0011, ADR 0051 table):
-- `giai-ngan` is the disbursement module, `nhac-ten` the mention.
--
-- WIDENS ONE CHECK, additively: staff_notification.kind (0021 staff_notification_kind_per_domain).
--
-- AND DELIBERATELY NOT THE OTHER TWO — THIS IS HOW "BELL ONLY" IS HELD BY THE DATABASE:
--   zalo_channel_setting.kinds   (0021 zalo_channel_setting_kinds_per_domain) unchanged: no commune can
--                                tick a Zalo box for this kind.
--   zalo_delivery.kind           (0021 zalo_delivery_kind_per_domain) unchanged: no Zalo delivery row,
--                                queued or skipped, can carry this kind. Go never tries — the enqueue
--                                statement selects only domain.ZaloQueueableKinds
--                                (internal/store/zalo_link.go enqueueZaloDeliveries $6) — and this CHECK
--                                is the floor under it if Go ever did.
--   Widening either later is a decision (ADR 0081 #5 says no), not a fix.
--
-- WHY A NEW FILE: core/migrate compares the checksum of every applied file at startup. 0021 is not edited.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. ROWS PER COMMUNE: NONE WRITTEN. ADD CONSTRAINT scans staff_notification once to validate — up to
--      low tens of thousands of rows per commune per year (0010).
--   2. IF IT STOPS HALF-WAY: cannot — one file, one transaction. The new constraint is added behind a
--      pg_constraint lookup BEFORE the old one is dropped (DROP … IF EXISTS), so the column is never
--      unconstrained, and a retry costs nothing.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. READ PATHS THAT CHANGE MEANING: none. The CHECK only widens; no row changes.
--   5. RETENTION: nothing dropped, retyped, emptied or renumbered.
--
-- WHY VALIDATED, NOT `NOT VALID`: the new CHECK admits every value the old one did, so the scan cannot
-- fail; and NOT VALID on a partitioned table is not portable across PG versions (0011:332-341, 0017).
--
-- THE LOCK: ACCESS EXCLUSIVE on staff_notification and its partitions until COMMIT — the bell inbox waits
-- for the validating scan. Run it in a quiet window on a large deployment.
--
-- PERSONAL DATA (rule 3): none. PG FLOOR: 13. Cluster: 16.
-- ---------------------------------------------------------------------------

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = 'staff_notification'::regclass
                   AND conname = 'staff_notification_kind_with_bell_only') THEN
        ALTER TABLE staff_notification ADD CONSTRAINT staff_notification_kind_with_bell_only
            CHECK (kind IN ('sap-den-han', 'qua-han', 'leo-thang', 'ban-tin-tuan',
                            'nhiem-vu.sap-den-han', 'nhiem-vu.qua-han', 'nhiem-vu.chua-cu-nguoi', 'nhiem-vu.leo-thang',
                            'van-ban.sap-den-han', 'van-ban.qua-han', 'van-ban.chua-cu-nguoi', 'van-ban.leo-thang',
                            'phan-anh.sap-den-han', 'phan-anh.qua-han', 'phan-anh.chua-cu-nguoi', 'phan-anh.leo-thang',
                            'giai-ngan.nhac-ten'));
    END IF;
END $$;

-- The new constraint is in place; the narrower one it subsumes goes. Dropped on the partitioned parent, it
-- goes from every partition with it (inherited through PARTITION OF, never declared locally).
ALTER TABLE staff_notification DROP CONSTRAINT IF EXISTS staff_notification_kind_per_domain;

-- ---------------------------------------------------------------------------
-- REVERSAL (migration question 3). Run by a person, in ONE transaction (ADR 0013). Prose, not runnable.
--
-- LOSSLESS ONLY WHILE no staff_notification row has kind = 'giai-ngan.nhac-ten'. Then: re-add 0021's
-- staff_notification_kind_per_domain VERBATIM, drop staff_notification_kind_with_bell_only, and remove
-- this file's row from `schema_migration` (ten = '0023_staff_notification_disbursement_mention.sql').
--
-- ONCE ANY ROW HOLDS IT, the old CHECK refuses to be re-added — and it should: a delivered notice is an
-- immutable record (0010 guard); rewriting its kind is editing history (rule 7 forbidden #5).
-- ---------------------------------------------------------------------------
