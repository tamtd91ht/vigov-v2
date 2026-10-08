-- 0021 — ZALO REMINDER KINDS PER DOMAIN, phase 1 (ADR 0079 #4 and §"Còn mở"; owner answer A2, 08/10/2026:
-- "Chia hai giai đoạn"). The four kinds of 0010/0018 cannot express spec Cấu hình 11's per-domain events
-- ("Việc quá hạn" for tasks but not for documents). Phase 1 adds a per-domain kind for every notice that
-- ALREADY HAS A PRODUCER; phase 2 (task.assigned, document.transferred, feedback.reopened, …) waits for
-- producers and is not here.
--
-- Widens three CHECKs, additively:
--   staff_notification.kind              0010 staff_notification_kind_known
--   zalo_channel_setting.kinds           0018 zalo_channel_setting_kinds_known (+ the overdue-cadence CHECK)
--   zalo_delivery.kind                   0018 zalo_delivery_kind_known
--
-- WHY A NEW FILE: core/migrate compares the checksum of every applied file at startup.
--
-- ---------------------------------------------------------------------------
-- THE PRODUCERS, READ 08/10/2026 — the evidence the list below is built from:
--
--   service-petitions  domain/automation.go:33-34   work kinds 'nhiem-vu' (task), 'phan-anh' (petition)
--                      app/automation_jobs.go:193   DueSoonNotice      → proto DUE_SOON
--                      app/automation_jobs.go:201   OverdueNotice      → proto OVERDUE
--                      app/automation_jobs.go:209   UnassignedNotice   → proto OVERDUE (domain/automation.go:307-309:
--                                                   "comms has no kind of its own for it")
--                      app/automation_jobs.go:270,277  EscalationNotice → proto ESCALATION
--                      app/automation_jobs.go:312,338  weekly digests  → proto WEEKLY_DIGEST
--   service-documents  domain/automation.go:37     work kind 'van-ban-den' (incoming documents)
--                      app/automation_jobs.go:188/196/204/258,265/307 — the same five shapes
--
--   So EVERY ONE of the three domains produces due-soon, overdue, unassigned, escalation and a weekly
--   digest — fifteen producer/shape pairs. Spec 11 names nine of them as events.
--
-- ---------------------------------------------------------------------------
-- THE KIND LIST. Values Vietnamese without diacritics (ADR 0011, ADR 0051 table), `<domain>.<shape>`:
--
--   kind                     producer / shape                         spec 11 event
--   nhiem-vu.sap-den-han     petitions, task,     DueSoonNotice       task.due_soon
--   nhiem-vu.qua-han         petitions, task,     OverdueNotice       task.overdue
--   nhiem-vu.chua-cu-nguoi   petitions, task,     UnassignedNotice    task.unassigned_too_long
--   nhiem-vu.leo-thang       petitions, task,     EscalationNotice    task.escalated
--   van-ban.sap-den-han      documents, incoming, DueSoonNotice       document.due_soon
--   van-ban.qua-han          documents, incoming, OverdueNotice       document.overdue
--   van-ban.chua-cu-nguoi    documents, incoming, UnassignedNotice    (no spec code; own box, Q3)
--   van-ban.leo-thang        documents, incoming, EscalationNotice    (no spec code; own box, Q3)
--   phan-anh.sap-den-han     petitions, petition, DueSoonNotice       feedback.due_soon
--   phan-anh.qua-han         petitions, petition, OverdueNotice       feedback.overdue
--   phan-anh.chua-cu-nguoi   petitions, petition, UnassignedNotice    (no spec code; own box, Q3)
--   phan-anh.leo-thang       petitions, petition, EscalationNotice    (no spec code; own box, Q3)
--   ban-tin-tuan             all three digests (UNCHANGED value)      digest.weekly
--
--   `van-ban`, not `van-ban-den`: spec 11 groups "Văn bản, đơn thư" as ONE event family, so the đơn thư
--   register, when it gets a producer, lands under the same prefix instead of a fourth.
--   `chua-cu-nguoi` = spec 11's "Bộ phận chưa cử người làm"; split OUT of `qua-han`, where it travels today.
--   `ban-tin-tuan` stays one value: spec 11 has one digest event, and the three digests are told apart
--   by their idempotency key, not by a kind the commune could switch separately.
--
-- WHY ALL TWELVE, AND NOT ONLY THE NINE SPEC EVENT CODES — DECIDED, ADR 0079 Q3 "Chia hai giai đoạn"
-- (08/10/2026): every kind that ALREADY HAS A PRODUCER becomes its own on/off box this round — due soon /
-- overdue per task, document–đơn thư and petition; escalation ("đôn đốc"); unit-not-assigned, SPLIT OUT of
-- overdue; the weekly digest. The four rows marked "no spec code" have producers today, so they are
-- admitted here and get their own box; web maps each kind to the spec's code where one exists (ADR 0079
-- Q1 #6). Filing them under `*.qua-han` instead would have frozen a wrong classification into
-- staff_notification rows that can never be edited (0010 guard). The ten spec events with NO producer yet
-- are phase 2 (ADR 0079 Q3, §"Còn mở" row 9) and are not admitted here.
--
-- ---------------------------------------------------------------------------
-- THE OLD FOUR VALUES STAY VALID, IN ALL THREE CHECKS, AND NO ROW IS REWRITTEN. The decision the card
-- asked for — migrate settings rows per tenant, or keep them valid and map at read:
--
--   staff_notification, zalo_delivery   NOT A CHOICE. Their `kind` is frozen by their guards (0010:72,
--                                       0018:446): a notice already delivered keeps the kind it was
--                                       delivered under. Old values must stay valid forever.
--   zalo_channel_setting.kinds          KEPT VALID, MAPPED AT READ. Chosen because rewriting NOW makes a
--                                       read path change meaning (migration question 4): the dispatcher
--                                       skips with 'loai-tat' when `n.kind = ANY (s.kinds)` fails
--                                       (internal/store/zalo_link.go:404), and the producers still send
--                                       the OLD kinds until comms.proto and both runners change. A
--                                       commune whose row was rewritten to `nhiem-vu.qua-han` would stop
--                                       receiving every overdue reminder, silently, from the deploy of
--                                       this file until the last producer is redeployed. Keeping the row
--                                       loses nothing, needs no backfill, no per-commune progress and no
--                                       system audit entry, and is reversed by doing nothing.
--
-- THE READ-TIME MAP (OWED BY GO — one table, in internal/domain, used by the settings read, the
-- dispatcher's kind test and the screen):
--
--   stored old value   means
--   sap-den-han        nhiem-vu.sap-den-han, van-ban.sap-den-han, phan-anh.sap-den-han
--   qua-han            nhiem-vu.qua-han, van-ban.qua-han, phan-anh.qua-han,
--                      nhiem-vu.chua-cu-nguoi, van-ban.chua-cu-nguoi, phan-anh.chua-cu-nguoi
--                      (unassigned notices were SENT as qua-han — a commune that ticked qua-han was
--                      receiving them; the expansion keeps exactly what it was getting)
--   leo-thang          nhiem-vu.leo-thang, van-ban.leo-thang, phan-anh.leo-thang
--   ban-tin-tuan       ban-tin-tuan
--
-- The map is lossless both ways for a commune that has not saved since: every old selection names a set
-- of new kinds, and the new kinds received are exactly the notices received before. The commune's NEXT
-- save writes new values only (Go refuses old values on save once the producers send new kinds); a
-- later migration may then rewrite the remaining rows and narrow the settings CHECK — after every
-- producer sends per-domain kinds, never before.
--
-- OWED ELSEWHERE, NOT BY THIS FILE:
--   * comms.proto (contract-designer): the request carries no domain. Either new StaffNotificationKind
--     values or an optional field naming the domain; both runners set it. Until then comms keeps
--     mapping the proto enum onto the OLD four, and the new values here are admitted but unused.
--   * internal/domain/zalo_link.go ZaloReminderKinds and the dispatcher's kind test (go-service-builder);
--     store/zalo_link_pg_test.go asserts the Go list agrees with the CHECK — it must take the new list.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. ROWS PER COMMUNE: NONE WRITTEN. Each ADD CONSTRAINT scans its table once to validate:
--      zalo_channel_setting ≤ 1 per commune; staff_notification and zalo_delivery up to low tens of
--      thousands per commune per year (0010, 0018).
--   2. IF IT STOPS HALF-WAY: cannot — one file, one transaction. New constraints are added behind a
--      pg_constraint lookup BEFORE the old ones are dropped (DROP … IF EXISTS), so at no statement is a
--      column unconstrained, and a retry costs nothing. No backfill, nothing to resume.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. READ PATHS THAT CHANGE MEANING: none. Every CHECK only WIDENS; no row changes; the dispatcher's
--      `n.kind = ANY (s.kinds)` compares the same values it compared yesterday.
--   5. RETENTION: nothing dropped, retyped, emptied or renumbered. Notices and deliveries keep their
--      kinds; settings rows keep theirs.
--
-- WHY VALIDATED, NOT `NOT VALID`: each new CHECK admits every value the old one did, so the scan cannot
-- fail; and NOT VALID on a partitioned table is not portable across PG versions (0011:332-341, 0017).
--
-- THE LOCK: ACCESS EXCLUSIVE on staff_notification, zalo_channel_setting, zalo_delivery and their
-- partitions until COMMIT — the validating scans of staff_notification and zalo_delivery are the cost.
-- The bell inbox waits for them. Today's row counts are small; on a large deployment, run it in a quiet
-- window.
--
-- PERSONAL DATA (rule 3): none. PG FLOOR: 13. Cluster: 16.
-- ---------------------------------------------------------------------------

DO $$
BEGIN
    -- staff_notification.kind ---------------------------------------------------------------------
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = 'staff_notification'::regclass
                   AND conname = 'staff_notification_kind_per_domain') THEN
        ALTER TABLE staff_notification ADD CONSTRAINT staff_notification_kind_per_domain
            CHECK (kind IN ('sap-den-han', 'qua-han', 'leo-thang', 'ban-tin-tuan',
                            'nhiem-vu.sap-den-han', 'nhiem-vu.qua-han', 'nhiem-vu.chua-cu-nguoi', 'nhiem-vu.leo-thang',
                            'van-ban.sap-den-han', 'van-ban.qua-han', 'van-ban.chua-cu-nguoi', 'van-ban.leo-thang',
                            'phan-anh.sap-den-han', 'phan-anh.qua-han', 'phan-anh.chua-cu-nguoi', 'phan-anh.leo-thang'));
    END IF;

    -- zalo_channel_setting.kinds ------------------------------------------------------------------
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = 'zalo_channel_setting'::regclass
                   AND conname = 'zalo_channel_setting_kinds_per_domain') THEN
        ALTER TABLE zalo_channel_setting ADD CONSTRAINT zalo_channel_setting_kinds_per_domain
            CHECK (kinds <@ ARRAY['sap-den-han', 'qua-han', 'leo-thang', 'ban-tin-tuan',
                                  'nhiem-vu.sap-den-han', 'nhiem-vu.qua-han', 'nhiem-vu.chua-cu-nguoi', 'nhiem-vu.leo-thang',
                                  'van-ban.sap-den-han', 'van-ban.qua-han', 'van-ban.chua-cu-nguoi', 'van-ban.leo-thang',
                                  'phan-anh.sap-den-han', 'phan-anh.qua-han', 'phan-anh.chua-cu-nguoi', 'phan-anh.leo-thang']::text[]);
    END IF;

    -- The overdue cadence (0018:154-155) is the commune's to set before ANY overdue kind is switched
    -- on — the old value or a per-domain one. Unassigned and escalation notices are one-shot per hold /
    -- per level (their idempotency keys), so they need no cadence and are not listed.
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = 'zalo_channel_setting'::regclass
                   AND conname = 'zalo_channel_setting_overdue_kinds_need_cadence') THEN
        ALTER TABLE zalo_channel_setting ADD CONSTRAINT zalo_channel_setting_overdue_kinds_need_cadence
            CHECK (NOT (kinds && ARRAY['qua-han', 'nhiem-vu.qua-han', 'van-ban.qua-han', 'phan-anh.qua-han']::text[])
                   OR overdue_start_after_days IS NOT NULL);
    END IF;

    -- zalo_delivery.kind --------------------------------------------------------------------------
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = 'zalo_delivery'::regclass
                   AND conname = 'zalo_delivery_kind_per_domain') THEN
        ALTER TABLE zalo_delivery ADD CONSTRAINT zalo_delivery_kind_per_domain
            CHECK (kind IN ('sap-den-han', 'qua-han', 'leo-thang', 'ban-tin-tuan', 'thu-nghiem',
                            'nhiem-vu.sap-den-han', 'nhiem-vu.qua-han', 'nhiem-vu.chua-cu-nguoi', 'nhiem-vu.leo-thang',
                            'van-ban.sap-den-han', 'van-ban.qua-han', 'van-ban.chua-cu-nguoi', 'van-ban.leo-thang',
                            'phan-anh.sap-den-han', 'phan-anh.qua-han', 'phan-anh.chua-cu-nguoi', 'phan-anh.leo-thang'));
    END IF;
END $$;

-- The new constraints are in place; the narrower ones they subsume go. Dropped on the partitioned parent,
-- each goes from every partition with it (inherited through PARTITION OF, never declared locally).
ALTER TABLE staff_notification   DROP CONSTRAINT IF EXISTS staff_notification_kind_known;
ALTER TABLE zalo_channel_setting DROP CONSTRAINT IF EXISTS zalo_channel_setting_kinds_known;
ALTER TABLE zalo_channel_setting DROP CONSTRAINT IF EXISTS zalo_channel_setting_overdue_needs_cadence;
ALTER TABLE zalo_delivery        DROP CONSTRAINT IF EXISTS zalo_delivery_kind_known;

-- Untouched, and still right: zalo_channel_setting_enabled_has_kind (any kind counts),
-- zalo_delivery_test_has_no_notice ('thu-nghiem' only), and every guard — they freeze `kind`, whatever
-- its value.

-- ---------------------------------------------------------------------------
-- REVERSAL (migration question 3). Run by a person, in ONE transaction (ADR 0013). Prose, not runnable.
--
-- LOSSLESS ONLY WHILE no row of the three tables holds a per-domain value (count rows whose kind — or
-- any element of kinds — contains a '.'; all three counts zero). Then: re-add 0010's
-- staff_notification_kind_known and 0018's zalo_channel_setting_kinds_known,
-- zalo_channel_setting_overdue_needs_cadence and zalo_delivery_kind_known VERBATIM, drop the four
-- constraints this file added, and remove this file's row from `schema_migration`
-- (ten = '0021_zalo_kinds_per_domain.sql').
--
-- ONCE ANY ROW HOLDS A PER-DOMAIN VALUE, the old CHECKs refuse to be re-added — and they should: notices
-- and deliveries are immutable records, and rewriting their kind back is editing history (rule 7
-- forbidden #5). A settings row could be mapped back (union of each value's old equivalent), but only
-- with a system audit entry per commune and only by a NEW migration — the user's call.
-- ---------------------------------------------------------------------------
