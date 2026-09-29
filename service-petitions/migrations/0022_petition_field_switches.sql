-- 0022 — tier 2 of the petition field catalogue (`nhan_linh_vuc`, 0004) gains the two things ADR
-- 0026's default of 2026-09-20 grants a commune besides re-wording: switching a field OFF for new
-- submissions, and its own DISPLAY ORDER. User confirmation 2026-09-29; tier 1 is read over gRPC
-- from `platform` (ADR 0060).
--
-- WHY A NEW FILE AND NOT AN EDIT OF 0004: core/migrate compares the checksum of every applied file
-- at startup. Editing 0004 either stops the service (ErrChecksumLech) or leaves two databases
-- claiming one schema version while holding two schemas.
--
-- ---------------------------------------------------------------------------
-- WHAT A ROW MEANS AFTER THIS FILE — every column is an OVERRIDE, and "no override" has one
-- spelling per column. A row with all three at their "inherit" value is indistinguishable from no
-- row, which is exactly the point: "back to the default" is an UPDATE to those values, never a
-- DELETE and never a soft delete.
--
--   enabled   true  = the field is offered on the citizen's new-submission form (the default:
--                     NO ROW = ON, so a new commune offers every active tier-1 field)
--             false = hidden from the NEW-SUBMISSION path only. NEVER from a read path: a
--                     petition already carrying the code keeps its label, stays filterable and
--                     stays in every report (ADR 0026 §Bổ sung cuối ngày — the `WHERE dang_bat`
--                     trap). No list query may filter on this column.
--   nhan      NULL  = show the tier-1 DEFAULT label (platform, ADR 0060)
--             text  = the commune's own wording
--   thu_tu    0     = use the tier-1 sort_order (the existing column default, so rows written
--                     before this file already mean "inherit")
--             1..   = the commune's position. Compared in the SAME numeric space as tier-1
--                     sort_order, ties broken by tier-1 order then code — the model 0010 uses for
--                     task statuses, and for the same reason there is no UNIQUE on it.
--
-- WHY `enabled` IS ENGLISH AND `nhan` / `thu_tu` ARE NOT: new columns are named in English (rule
-- 12, ADR 0051); existing ones are not renamed.
--
-- WHY NULL FOR "DEFAULT LABEL" RATHER THAN COPYING THE DEFAULT IN: the default belongs to
-- `platform` and may be corrected there. A copied default freezes the old wording in this commune
-- with nothing on the screen saying it is a copy.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. HOW MANY ROWS PER COMMUNE: unchanged — at most one per tier-1 code, UNIQUE (tenant_id, ma).
--      This file writes no row. Existing rows get enabled = true (the column default), keep their
--      label and keep thu_tu, so nothing a commune sees changes.
--   2. IF IT STOPS HALF-WAY: it cannot land half-applied. One file, one transaction, with the
--      progress row written inside it. Every statement is IF NOT EXISTS or guarded, so a retry
--      costs nothing.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. WHICH READ PATHS CHANGE MEANING WHILE IT IS HALF-APPLIED: none. The file is one transaction,
--      and the readers treat a NULL label exactly as the absent row they already handle ("" — show
--      the default).
--   5. RETENTION: configuration, not an archival record. Its history is the audit entry every
--      write produces in the same transaction (rule 6, invariant 3).
--
-- ---------------------------------------------------------------------------
-- UNIQUE (tenant_id, ma) COUNTS SOFT-DELETED ROWS (0004). No write path has ever soft-deleted a row
-- here, and the one this change adds never does: it upserts on (tenant_id, ma) and refuses when the
-- row it would overwrite is soft-deleted, rather than silently reviving it.

ALTER TABLE nhan_linh_vuc ADD COLUMN IF NOT EXISTS enabled BOOLEAN NOT NULL DEFAULT true;

ALTER TABLE nhan_linh_vuc ALTER COLUMN nhan DROP NOT NULL;

-- A label that is present is a real label: not blank (a field nobody can name) and bounded in
-- CHARACTERS — char_length counts characters, so Vietnamese diacritics do not shorten the
-- allowance. 100 is the bound 0010 uses for status labels; the write path validates the same
-- number so a client sees a 400, not a 500.
DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'nhan_linh_vuc_label_bounded') THEN
        ALTER TABLE nhan_linh_vuc ADD CONSTRAINT nhan_linh_vuc_label_bounded
            CHECK (nhan IS NULL OR (btrim(nhan) <> '' AND char_length(nhan) <= 100));
    END IF;
END $$;

-- 0 is "inherit"; a negative position is one no screen draws. The upper bound is the column type's
-- business ceiling the other catalogues use (domain.ThuTuToiDa).
DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'nhan_linh_vuc_sort_order_range') THEN
        ALTER TABLE nhan_linh_vuc ADD CONSTRAINT nhan_linh_vuc_sort_order_range
            CHECK (thu_tu BETWEEN 0 AND 9999);
    END IF;
END $$;

-- ---------------------------------------------------------------------------
-- REVERSAL (migration question 3). In one transaction: drop the two constraints, drop the column
-- `enabled`, then — only after a verified backup — `UPDATE nhan_linh_vuc SET nhan = <the tier-1
-- default label> WHERE nhan IS NULL` per code and `ALTER COLUMN nhan SET NOT NULL`; finally remove
-- this file's row from `schema_migration`.
--
-- WHAT A REVERSAL COSTS ONCE COMMUNES HAVE WRITTEN ROWS: every field a commune switched off comes
-- back on its citizens' form, and "inherit the platform label" becomes a frozen copy of today's
-- default. No archival record is touched — petitions hold the CODE, not this row. It is still a
-- change to data communes entered, so it is the user's call, not a command's.
-- ---------------------------------------------------------------------------
