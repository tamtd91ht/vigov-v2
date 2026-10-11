-- 0027 — A CITIZEN MAY TURN OFF ZALO NOTIFICATIONS (owner decision of 11/10/2026, following the
-- prototype's "Nhận thông báo" switch). Two changes that only make sense together:
--
--   A. `citizen_notification_preferences` — one row per citizen per commune holding the switch.
--      ABSENT ROW = ENABLED. The prototype's default is on, and a row is written only by the citizen's
--      own act, so every citizen who never touched the switch keeps receiving what rule 10 invariant 5
--      owes them.
--   B. `thong_bao_gui_cong_dan.trang_thai` gains a fifth value, `dan-da-tat-nhan` — "the citizen turned
--      notifications off". The obligation row is STILL WRITTEN (0004: a row is created when the
--      obligation arises, not when the message leaves); it simply records why nothing went out. A
--      ledger that skipped the row would make "not told because they opted out" indistinguishable,
--      later, from "never told" — the exact question 0004 exists to answer.
--
-- This REVERSES ADR 0047 decision (12) "không công tắc 'Nhận thông báo'" (30/09/2026); the ADR
-- recording the reversal is the main session's to write.
--
-- NAMES (rule 12, ADR 0051): English table and columns. The status VALUE follows 0004's existing
-- values — Vietnamese without diacritics, kebab-case (ADR 0011).
--
-- WHY A NEW FILE: core/migrate compares the checksum of every applied file at startup. 0004 is not
-- edited.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. ROWS PER COMMUNE: none written. Part A: at most one row per citizen of the commune who ever
--      touched the switch. Part B validates `thong_bao_gui_cong_dan` once (scan of the existing rows —
--      low tens of thousands per commune per year at most, 0004 question 1).
--   2. IF IT STOPS HALF-WAY: cannot — one file, one transaction. The wider CHECK is added behind a
--      pg_constraint lookup BEFORE the narrower one is dropped (DROP … IF EXISTS), 0025's order, so the
--      column is never unconstrained; every other statement is IF NOT EXISTS / CREATE OR REPLACE /
--      DROP TRIGGER IF EXISTS. A retry costs nothing. No backfill, nothing to resume per commune.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. READ PATHS THAT CHANGE MEANING: none today — no row holds the new value and the new table is
--      empty, so "absent row = enabled" is every citizen. ONCE THE GO CARD WRITES THEM:
--        * `thong_bao_gui_cong_dan_chua_bao_duoc` (0004) indexes every row with trang_thai <> 'da-gui',
--          so the staff "chưa báo được" list WILL include opted-out rows. That screen must show the
--          reason (the citizen chose this) rather than present it as a failure — a Go/UI decision.
--        * A reader that counts "not delivered" as `trang_thai IN ('that-bai', 'chua-cau-hinh-kenh')`
--          is unaffected; one that counts `<> 'da-gui'` now includes opt-outs. Each such figure must
--          say which it means.
--        * The preference is read with "no row → enabled". A reader written as `zalo_enabled = true`
--          over a JOIN drops citizens with no row and silences them — agent rule 6's trap. Read it as
--          `COALESCE(p.zalo_enabled, true)` / `NOT EXISTS (… AND zalo_enabled = false)`.
--   5. RETENTION: nothing dropped, retyped, emptied or renumbered. Preference rows are never deleted
--      (trigger); a citizen turning notifications back on is an UPDATE.
--
-- WHY VALIDATED, NOT `NOT VALID`: the new CHECK admits every value the old one did, so the scan cannot
-- fail; and NOT VALID on a partitioned table is not portable across PG versions (0011:332-341, 0025).
--
-- THE LOCK: ACCESS EXCLUSIVE on thong_bao_gui_cong_dan and its 32 partitions until COMMIT — the
-- validating scan is the cost. The send worker waits for it. On a large deployment, run it in a quiet
-- window.
--
-- PERSONAL DATA (rule 3): `citizen_id` is the OPAQUE identity id (proto/vigov/petitions/v1/events.proto
-- `PetitionStatusChanged.citizen_id`, the same value 0004 stores as `nguoi_nhan_ma`). No phone, no
-- name. Whether one person receives Zalo messages is a fact about that person: never logged with the id.
--
-- PG FLOOR: 13 (BEFORE ... FOR EACH ROW triggers on partitioned tables; 0004 checks it). Cluster: 16.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- citizen_notification_preferences_guard — DELETE refused (absence means "enabled", so deleting a
-- row silently turns a citizen's opt-out back on); the key is fixed.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION citizen_notification_preferences_guard() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION '%: delete refused', TG_TABLE_NAME
            USING HINT = 'An absent row means notifications are ON. Deleting a row would turn a '
                         'citizen''s opt-out back on without their act. Set zalo_enabled instead.';
    END IF;

    IF NEW.tenant_id  IS DISTINCT FROM OLD.tenant_id
    OR NEW.citizen_id IS DISTINCT FROM OLD.citizen_id
    OR NEW.created_at IS DISTINCT FROM OLD.created_at THEN
        RAISE EXCEPTION '%: identity columns are immutable', TG_TABLE_NAME
            USING HINT = 'Commune and citizen are the key; a preference is never moved to another '
                         'person.';
    END IF;

    RETURN NEW;
END $$;

-- ---------------------------------------------------------------------------
-- @entity: CitizenNotificationPreference
-- @scope:  tenant
--
-- citizen_notification_preferences — whether ONE citizen of ONE commune receives Zalo notifications
-- about their own records. No row = enabled.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS citizen_notification_preferences (
    tenant_id     TEXT        NOT NULL,
    -- OPAQUE citizen identity id (see PERSONAL DATA in the header). Never a phone number.
    citizen_id    TEXT        NOT NULL,

    -- NO DEFAULT: the only writer is the citizen's own act, which states the value.
    zalo_enabled  BOOLEAN     NOT NULL,

    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- One preference per citizen per commune; composite with tenant_id (rule 1, invariant 6).
    PRIMARY KEY (tenant_id, citizen_id),

    CONSTRAINT citizen_notification_preferences_citizen_id_shape CHECK (
        btrim(citizen_id) <> '' AND char_length(citizen_id) <= 64 AND citizen_id !~ '[[:space:][:cntrl:]]')
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS citizen_notification_preferences_p%s PARTITION OF citizen_notification_preferences '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

-- No secondary index: the primary key (tenant_id, citizen_id) is exactly the send path's lookup and the
-- citizen's own read.

DROP TRIGGER IF EXISTS citizen_notification_preferences_guard ON citizen_notification_preferences;
CREATE TRIGGER citizen_notification_preferences_guard
    BEFORE UPDATE OR DELETE ON citizen_notification_preferences
    FOR EACH ROW EXECUTE FUNCTION citizen_notification_preferences_guard();

-- ---------------------------------------------------------------------------
-- B. thong_bao_gui_cong_dan.trang_thai — 0004's four values + `dan-da-tat-nhan`.
--
--   dan-da-tat-nhan  THE CITIZEN TURNED NOTIFICATIONS OFF. Terminal, like `chua-cau-hinh-kenh`: it is
--                    never retried (it is not a failure) and never shown as "sent". 0004's guard is
--                    unchanged and still right: it freezes what was owed to whom, and refuses leaving
--                    `da-gui`, whatever the other values are.
-- ---------------------------------------------------------------------------
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = 'thong_bao_gui_cong_dan'::regclass
                   AND conname = 'thong_bao_gui_cong_dan_trang_thai_with_opt_out') THEN
        ALTER TABLE thong_bao_gui_cong_dan ADD CONSTRAINT thong_bao_gui_cong_dan_trang_thai_with_opt_out
            CHECK (trang_thai IN ('cho-gui', 'da-gui', 'that-bai', 'chua-cau-hinh-kenh', 'dan-da-tat-nhan'));
    END IF;
END $$;

-- The new constraint is in place; the narrower one it subsumes goes. Dropped on the partitioned parent,
-- it goes from every partition with it (inherited through PARTITION OF, never declared locally).
ALTER TABLE thong_bao_gui_cong_dan DROP CONSTRAINT IF EXISTS thong_bao_gui_cong_dan_trang_thai_hop_le;

-- ---------------------------------------------------------------------------
-- WHAT THIS FILE DOES NOT STOP: TRUNCATE and DDL by the table owner (ADR 0013's line).
--
-- REVERSAL (migration question 3). Run by a person, in ONE transaction (ADR 0013). Prose, not runnable.
--
-- LOSSLESS ONLY WHILE BOTH READ ZERO:
--
--   SELECT count(*) FROM citizen_notification_preferences;
--   SELECT count(*) FROM thong_bao_gui_cong_dan WHERE trang_thai = 'dan-da-tat-nhan';
--
-- ZERO → re-add 0004's `thong_bao_gui_cong_dan_trang_thai_hop_le` VERBATIM, drop
-- `thong_bao_gui_cong_dan_trang_thai_with_opt_out`, drop table citizen_notification_preferences
-- (partitions and trigger go with it), drop function citizen_notification_preferences_guard(), and
-- remove this file's row from `schema_migration` (ten = '0027_citizen_notification_preferences.sql').
--
-- NON-ZERO → a preference row is a citizen's stated choice; dropping it turns their notifications back
-- on without their act. A ledger row with the new value records why a citizen was not told, and 0004's
-- guard does not let it be rewritten into another status (rule 7 forbidden #5). Both: rule 7 stop
-- condition #2 — the user plus a verified backup — and the way back is a NEW migration.
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
                         'MODULUS 32 partition loop (ADR 0010) in the same migration that '
                         'declares PARTITION BY.';
    END IF;
END $$;
