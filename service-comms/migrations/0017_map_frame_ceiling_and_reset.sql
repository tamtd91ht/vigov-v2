-- 0017 — MAP FRAME, second revision of ADR 0072 (§"Sửa đổi 04/10/2026 (lần 2)", K2 / K4 / K5):
--
--   (a) the radius bound becomes `> 0 AND <= 50` km — the owner's HARD ceiling (K2), replacing 0016's
--       owner-pending 1–30 km proposal (`map_frame_radius_range`);
--   (b) a commune can go back to the PLATFORM DEFAULT frame ("Về mặc định", K4) without its row being
--       deleted: a column, `is_enabled`, exactly as 0016:42-46 said such a state would have to be.
--
-- WHY A NEW FILE: core/migrate compares the checksum of every applied file at startup. 0016 is NOT
-- edited; its "~109.8°E at the 30 km ceiling" note stays as history — K2's table is the current figure
-- (≈ 109.95–109.99°E at 50 km, still ≥ 1.2° short of Hoàng Sa / Trường Sa).
--
-- ---------------------------------------------------------------------------
-- VENDOR CHOICES, stated so a reviewer can overrule them:
--
--   * THE COLUMN IS `is_enabled`, NOT `cleared_at` / `cleared_by`. `is_enabled` is this service's
--     existing word for "one configuration row per commune, switched off without being deleted"
--     (0008 mail_settings, 0013 portal_sync_settings: "turn it off with is_enabled = false"). The
--     WHO and WHEN of a switch-off are already on the row — the reset is an UPDATE, so the use case
--     sets `updated_at` / `updated_by` like any other change — and durably in the audit entry the
--     same transaction writes (rule 6), with before/after. A `cleared_at` / `cleared_by` pair would
--     be a second copy of that fact, and one that goes stale the moment the commune saves its own
--     frame again: the pair then has to be NULLed (erasing who reset it) or left standing beside an
--     enabled frame (contradicting it). The trail keeps the history; the row keeps the current state.
--   * DEFAULT true, unlike 0008/0013's false. Every row that exists today is a frame a commune SAVED
--     and is applying now. A false default would switch every such commune over to the platform
--     default at deploy — a read path changing meaning under nobody's decision. Constant default →
--     catalogue-only on PG 11+, no row rewritten.
--   * NOT NULL, so there is no third state: `is_enabled` and `is_enabled = true` agree on every row,
--     including rows that predate this file (agent invariant 6, the `$ne: true` trap, cannot occur).
--   * A DISABLED ROW KEEPS ITS CENTRE AND RADIUS. They are the commune's last own frame — what the
--     form can offer back — and its audited decision. Nothing here NULLs them, and the columns stay
--     NOT NULL with every CHECK still applying to them.
--   * THE LEGAL-NOTICE ACKNOWLEDGEMENT (K4, K6 version `2026-10-04.1`) HAS NO COLUMN. K4 puts it in
--     the AUDIT ENTRY of the save ("dòng vết ... ghi đã xác nhận + phiên bản văn bản"), beside the
--     before/after values. No configuration table in this repository keeps a "last acknowledged
--     notice version" on the row (searched every service's migrations, 04/10/2026), and a column
--     would only say which version was acknowledged LAST, overwritten by the next save — the trail
--     records every one. The audit trail is the record.
--   * THE GUARD FUNCTION IS REPLACED ONLY TO CORRECT ITS DELETE HINT, which told an operator that H3
--     has no "no frame" state — false since K4. Its logic is 0016's, verbatim (pinned by
--     map_frame_ceiling_test.go): DELETE refused; tenant_id, created_at, created_by frozen;
--     everything else editable, `is_enabled` included.
--
-- OWED BY GO, NOT BY THIS FILE (service-comms/internal/**, another task card):
--   * the read treats `is_enabled = false` exactly as "no row" — the commune has no own frame, and
--     the platform default (K3) applies;
--   * the save's upsert sets `is_enabled = true` in BOTH its VALUES and its ON CONFLICT SET list.
--     Today's upsert (internal/store/map_frame.go:28-33) names neither, so after a reset a commune's
--     next save would update the centre and leave the frame OFF;
--   * domain bounds (internal/domain/map_frame.go:36-37, 1.0 / 30.0) follow K2: > 0 and <= 50.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. ROWS PER COMMUNE: at most one (tenant_id IS the primary key, 0016). NO ROW IS WRITTEN. ADD
--      COLUMN with a constant default is catalogue-only. ADD CONSTRAINT scans the table once to
--      validate — at most one row per commune, so the scan is the number of communes.
--   2. IF IT STOPS HALF-WAY: it cannot land half-applied — one file, one transaction. Every statement
--      is IF NOT EXISTS / IF EXISTS / CREATE OR REPLACE, or a constraint added behind a pg_constraint
--      lookup; a retry costs nothing. No backfill, so nothing to resume per commune.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. READ PATHS THAT CHANGE MEANING WHILE HALF-APPLIED: none can see it half-applied (one
--      transaction). AFTER it, before the Go change:
--        * internal/store names its columns explicitly (mapFrameColumns, no `SELECT *`), so no read
--          sees `is_enabled` until it asks; every existing row is `true` = applying, today's meaning.
--          No row can become false until the "Về mặc định" use case exists.
--        * the old 1–30 bound is still enforced by the domain (map_frame.go:36-37), so no row outside
--          1–30 can be written until the Go change either. The schema is WIDER than the service for
--          that window, never narrower: no save the service accepts is refused here.
--   5. RETENTION: nothing is dropped, retyped, emptied or renumbered. The row is still never removed
--      — the guard still refuses DELETE; "Về mặc định" is `is_enabled = false` on a kept row. Its
--      history is the audit trail the service writes for every change (rule 6).
--
-- WHY THE NEW CHECK IS VALIDATED, not `NOT VALID`: it only WIDENS. Every value 0016's CHECK admitted
-- (1.0 ≤ r ≤ 30.0) satisfies `r > 0 AND r <= 50`, so the validating scan cannot fail on any row that
-- exists — there is nothing for NOT VALID to defer. It would also be the wrong tool here: NOT VALID on
-- a partitioned table is not portable across PG versions (0011:332-341), and there is no lock to save
-- — ADD COLUMN already holds ACCESS EXCLUSIVE until COMMIT.
--
-- ORDER: the new CHECK is ADDED BEFORE the old one is DROPPED, so at no statement inside the
-- transaction is the radius unbounded. A constraint dropped on the partitioned parent is dropped from
-- its 32 partitions with it (they inherit it; none was declared locally — 0016's loop uses PARTITION OF).
--
-- THE LOCK: ACCESS EXCLUSIVE on map_frame and its 32 partitions until COMMIT. Table size is at most one
-- row per commune.
--
-- PERSONAL DATA (rule 3): none. PG FLOOR: 13 (0016). Cluster: 16.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- map_frame.is_enabled — false = the commune does not use its own frame; the platform default applies
-- (ADR 0072 K3/K4). See the header for why this column and why DEFAULT true.
-- ---------------------------------------------------------------------------
ALTER TABLE map_frame ADD COLUMN IF NOT EXISTS is_enabled BOOLEAN NOT NULL DEFAULT true;

-- ---------------------------------------------------------------------------
-- THE RADIUS CEILING — ADR 0072 K2: > 0 and <= 50 km, the owner's hard bound. Raising it past 50 km is
-- stop condition 1 of the second revision ("Điểm dừng — thêm ở lần 2"), the user's call. numeric(4,1) makes "> 0" mean ≥ 0.1 km in practice.
-- ---------------------------------------------------------------------------
DO $$
DECLARE parent oid := 'map_frame'::regclass;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = parent
                   AND conname = 'map_frame_radius_ceiling') THEN
        ALTER TABLE map_frame ADD CONSTRAINT map_frame_radius_ceiling
            CHECK (radius_km > 0 AND radius_km <= 50);
    END IF;
END $$;

ALTER TABLE map_frame DROP CONSTRAINT IF EXISTS map_frame_radius_range;

-- ---------------------------------------------------------------------------
-- map_frame_guard — 0016's logic unchanged; only the DELETE hint now names the reset that exists.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION map_frame_guard() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION '%: delete refused', TG_TABLE_NAME
            USING HINT = 'To return a commune to the platform default frame, set is_enabled = false '
                         '(ADR 0072 K4). The row and its last centre and radius are kept.';
    END IF;

    IF NEW.tenant_id  IS DISTINCT FROM OLD.tenant_id
    OR NEW.created_at IS DISTINCT FROM OLD.created_at
    OR NEW.created_by IS DISTINCT FROM OLD.created_by THEN
        RAISE EXCEPTION '%: identity columns are immutable', TG_TABLE_NAME
            USING HINT = 'Commune and who first set the frame are fixed at creation.';
    END IF;

    RETURN NEW;
END $$;

-- The trigger itself (0016) is untouched: BEFORE UPDATE OR DELETE, FOR EACH ROW, and CREATE OR REPLACE
-- FUNCTION rebinds nothing — it keeps calling this function by name.

-- ---------------------------------------------------------------------------
-- REVERSAL (migration question 3). Run by a person, in ONE transaction; core/migrate has no automatic
-- rollback (ADR 0013).
--
-- LOSSLESS ONLY WHILE BOTH READ ZERO:
--
--   SELECT count(*) FROM map_frame WHERE radius_km < 1 OR radius_km > 30;   -- (A)
--   SELECT count(*) FROM map_frame WHERE NOT is_enabled;                    -- (B)
--
-- BOTH ZERO → in this order:
--
--   ALTER TABLE map_frame ADD CONSTRAINT map_frame_radius_range CHECK (radius_km BETWEEN 1 AND 30);
--   ALTER TABLE map_frame DROP CONSTRAINT map_frame_radius_ceiling;
--   ALTER TABLE map_frame DROP COLUMN is_enabled;
--   -- restore 0016's map_frame_guard: re-run its CREATE OR REPLACE FUNCTION block verbatim
--   DELETE FROM schema_migration WHERE <this file's row>;  -- the runner's own bookkeeping, not business data
--
-- (A) NON-ZERO → a commune saved a radius the owner allowed (K2). The ADD CONSTRAINT above refuses
-- rather than loses, but making it pass means rewriting an audited decision by a named official — the
-- user's call, never a command.
--
-- (B) NON-ZERO → those communes chose "Về mặc định". Dropping the column silently switches their own
-- frame back ON — a read path changing meaning against a recorded decision. Rule 7 stop condition #2:
-- the user plus a verified backup, and the way back is a NEW migration.
-- ---------------------------------------------------------------------------
