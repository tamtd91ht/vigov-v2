-- 0026 — tenant.deactivated_at, and the soft-delete triple tenant.deleted_at / deleted_by /
-- delete_reason. Owner decisions 13/10/2026 (ADR 0048 amendment).
--
-- deactivated_at — SINCE WHEN the commune has been inactive (dang_hoat_dong = false); NULL while it is
-- active. Stored, not derived from the trail on every read: the trail is append-only history, the
-- column is the current fact, and tenant_deactivated_iff_inactive (below) keeps the two columns of
-- this row from disagreeing.
--
-- deleted_at / deleted_by / delete_reason — a commune REMOVED from the registry (rule 7, invariant 1).
-- Soft, never a row delete: the row is what every archival record of that commune points at
-- (tenant_id), and its domains stay reserved by tenant_domain's primary key, which this file does not
-- touch. The three columns move together; deleted_by is an operator's business code `VH-…` (rule 6,
-- invariant 8 — the same shape as service-identity's operator_account.code and this service's 0020),
-- and the reason is never blank. A commune is removed only once it is inactive: an active commune
-- is still resolving hosts and receiving citizens' petitions.
--
-- THE THREE CHECKS:
--   * tenant_deleted_complete           — all three soft-delete columns present, or all absent.
--   * tenant_deleted_only_when_inactive — deleted_at IS NULL OR NOT dang_hoat_dong. Also means a
--     removed commune cannot be switched back on without first being restored.
--   * tenant_deactivated_iff_inactive   — (deactivated_at IS NULL) = dang_hoat_dong. Added AFTER the
--     backfill, which makes it hold for every existing row (active rows: NULL untouched; inactive
--     rows: filled). If a row still breaks it, ADD CONSTRAINT fails and the whole file rolls back.
--
-- CONSEQUENCE OF THE THIRD CHECK, stated — it couples this file to Go in the SAME release (migrations
-- are embedded in the binary, embed.go): from the moment it is applied,
--   * store/operator_writes.go SetActivation must write deactivated_at = now() when switching off and
--     deactivated_at = NULL when switching on, in the same UPDATE as dang_hoat_dong — otherwise every
--     deactivation and reactivation FAILS with 23514;
--   * every INSERT INTO tenant with dang_hoat_dong = false must name deactivated_at (the pg test
--     helper themXa in internal/store does this today without it).
-- Loud failure is the intended direction: a deactivation that silently left deactivated_at NULL
-- would be a commune whose "since when" nobody can answer.
--
-- NAMES (rule 12, ADR 0051): English. The table's older columns are Vietnamese (ADR 0061 revert);
-- they are not renamed.
--
-- WHY A NEW FILE: core/migrate compares the checksum of every applied file at startup.
--
-- ---------------------------------------------------------------------------
-- THE BACKFILL — deactivated_at for every commune inactive today.
--
-- SOURCE: the trail. SetActivation (store/operator_writes.go:449-519) writes one audit_log entry per
-- switch, in the same transaction, with action ActionDeactivate = 'ngung_hoat_dong_xa'
-- (operator_writes.go:73) or ActionReactivate = 'mo_lai_hoat_dong_xa' (:74), under the commune's own
-- tenant_id (audit_log.tenant_id, 0001_init.sql:23), at = the time of the switch (0001_init.sql:29).
-- The value taken is the LATEST 'ngung_hoat_dong_xa' entry of that tenant that NO LATER
-- 'mo_lai_hoat_dong_xa' entry follows — i.e. the deactivation that is still in force. A commune
-- deactivated, reactivated and then made inactive by some path that left no entry gets no stale
-- date from an earlier episode.
--
-- FALLBACK: now() (this transaction's start) where no such entry exists — a commune inserted
-- inactive, or switched off before SetActivation existed. That value means "inactive since NO LATER
-- than this migration", not the true date; the trail entry below says which of the two each row got
-- ('source'), so the two are never confused afterwards.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. ROWS PER COMMUNE: at most one — the commune's own registry row, and only if it is inactive
--      today. The table is one row per commune ever onboarded (single digits today). PER COMMUNE AND
--      RESUMABLE (rule 7, invariant 5): ONE STATEMENT, not a per-commune loop, for 0021/0024's
--      reason: core/migrate gives a file one transaction, so a loop inside it could not be resumed
--      either. Every row carries its own tenant id into its own audit entry.
--   2. IF IT STOPS HALF-WAY: it cannot land half-applied — one file, one transaction with the
--      runner's progress row; columns, values, trail entries and constraints commit or roll back
--      together. A re-run is free: ADD COLUMN IF NOT EXISTS, the backfill only touches inactive rows
--      whose deactivated_at IS NULL, every CHECK sits behind a pg_constraint lookup.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. READ PATHS THAT CHANGE MEANING: none while half-applied (one transaction). After it: no read
--      path names the new columns yet, and store/ reads name their columns (no `SELECT *` on
--      tenant), so every read returns what it did. Once a read path filters on deleted_at, it must
--      use `deleted_at IS NULL` (a NULL test, so rows that predate the column match). The WRITE
--      paths that change are SetActivation and inactive INSERTs — see CONSEQUENCE above.
--   5. RETENTION: nothing dropped, retyped or renumbered; no existing value changes. Columns are
--      ADDED; deactivated_at is FILLED for inactive communes — merged and dissolved ones included,
--      because the third CHECK cannot skip them. Their name, status, domains, codes and history are
--      untouched (rule 7, invariant 6). cap_nhat_luc is NOT bumped (0021/0024: it keeps naming the
--      last human change; this file's who/when is the audit entry). tenant_domain is not touched:
--      a removed commune's domains stay reserved by its primary key.
--
-- AUDIT (rule 6, invariant 6): each backfilled commune gets one audit_log entry under THAT commune,
-- actor 'system' / kind 'system', in the same statement as the write (invariant 3) — the 0024
-- precedent. Action `deactivated_at.recorded` (the dotted English shape of launch_code.issued).
-- Active communes are not written, so they get no entry.
--
-- PERSONAL DATA (rule 3): none. Timestamps, an operator's business code, a reason an operator types
-- about a commune.
-- ---------------------------------------------------------------------------

ALTER TABLE tenant ADD COLUMN IF NOT EXISTS deactivated_at TIMESTAMPTZ NULL;
ALTER TABLE tenant ADD COLUMN IF NOT EXISTS deleted_at     TIMESTAMPTZ NULL;
ALTER TABLE tenant ADD COLUMN IF NOT EXISTS deleted_by     TEXT        NULL;
ALTER TABLE tenant ADD COLUMN IF NOT EXISTS delete_reason  TEXT        NULL;

DO $$ BEGIN
    -- Both pass on every existing row: the three columns were just added, NULL everywhere.
    IF NOT EXISTS (SELECT 1 FROM pg_constraint
                   WHERE conrelid = 'tenant'::regclass
                     AND conname = 'tenant_deleted_complete') THEN
        ALTER TABLE tenant ADD CONSTRAINT tenant_deleted_complete
            CHECK ((deleted_at IS NULL AND deleted_by IS NULL AND delete_reason IS NULL)
                OR (deleted_at IS NOT NULL AND deleted_by ~ '^VH-[0-9]{5,}$'
                    AND btrim(delete_reason) <> ''));
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_constraint
                   WHERE conrelid = 'tenant'::regclass
                     AND conname = 'tenant_deleted_only_when_inactive') THEN
        ALTER TABLE tenant ADD CONSTRAINT tenant_deleted_only_when_inactive
            CHECK (deleted_at IS NULL OR NOT dang_hoat_dong);
    END IF;
END $$;

DO $$
DECLARE recorded int;
BEGIN
    WITH still_in_force AS (
        -- The latest deactivation entry that no later reactivation entry follows, per inactive
        -- commune; NULL when there is none.
        SELECT t.id,
               (SELECT max(d.at) FROM audit_log d
                WHERE d.tenant_id = t.id
                  AND d.action = 'ngung_hoat_dong_xa'
                  AND NOT EXISTS (SELECT 1 FROM audit_log r
                                  WHERE r.tenant_id = t.id
                                    AND r.action = 'mo_lai_hoat_dong_xa'
                                    AND r.at > d.at)) AS entry_at
        FROM tenant t
        WHERE NOT t.dang_hoat_dong AND t.deactivated_at IS NULL
    ), changed AS (
        UPDATE tenant t
        SET deactivated_at = coalesce(s.entry_at, now())
        FROM still_in_force s
        WHERE t.id = s.id AND NOT t.dang_hoat_dong AND t.deactivated_at IS NULL
        RETURNING t.id, t.ten, t.deactivated_at, (s.entry_at IS NOT NULL) AS from_trail
    )
    INSERT INTO audit_log (tenant_id, actor_id, actor_kind, actor_ip, action, subject, at, delta)
    SELECT c.id, 'system', 'system', '', 'deactivated_at.recorded', c.ten, now(),
           jsonb_build_object(
               'deactivated_at', c.deactivated_at,
               'source',         CASE WHEN c.from_trail
                                      THEN 'audit_log ngung_hoat_dong_xa'
                                      ELSE 'migration time, no ngung_hoat_dong_xa entry in force' END,
               'qua',            'migration 0026_tenant_deactivated_and_soft_delete.sql')
    FROM changed c;
    GET DIAGNOSTICS recorded = ROW_COUNT;

    RAISE NOTICE '0026: deactivated_at recorded for % inactive commune(s), one deactivated_at.recorded entry each',
        recorded;
END $$;

DO $$ BEGIN
    -- After the backfill, never before: until it ran, every inactive row broke this.
    IF NOT EXISTS (SELECT 1 FROM pg_constraint
                   WHERE conrelid = 'tenant'::regclass
                     AND conname = 'tenant_deactivated_iff_inactive') THEN
        ALTER TABLE tenant ADD CONSTRAINT tenant_deactivated_iff_inactive
            CHECK ((deactivated_at IS NULL) = dang_hoat_dong);
    END IF;
END $$;

COMMENT ON COLUMN tenant.deactivated_at IS
    'Since when the commune is inactive (dang_hoat_dong = false); NULL while active, never otherwise '
    '(tenant_deactivated_iff_inactive). Owner decision 13/10/2026. Rows backfilled by 0026 carry a '
    'deactivated_at.recorded audit entry saying whether the value came from the trail or is the '
    'migration time.';
COMMENT ON COLUMN tenant.deleted_at IS
    'Soft delete (rule 7): the commune is removed from the registry, the row and every record '
    'pointing at it are kept. Only an inactive commune. Set with deleted_by and delete_reason.';
COMMENT ON COLUMN tenant.deleted_by IS
    'Business code of the operator who removed the commune (VH-...), never an internal id.';
COMMENT ON COLUMN tenant.delete_reason IS
    'Why the commune was removed, as the operator wrote it. Never blank when deleted_at is set.';

-- ---------------------------------------------------------------------------
-- REVERSAL (rule 7, invariant 4). Run by a person, in ONE transaction; core/migrate has no automatic
-- rollback (ADR 0013). Prose, not runnable — a runnable line is a line that gets run.
--
-- Drop constraints tenant_deactivated_iff_inactive, tenant_deleted_only_when_inactive and
-- tenant_deleted_complete, then columns delete_reason, deleted_by, deleted_at and deactivated_at, and
-- remove this file's row from `schema_migration` (ten = '0026_tenant_deactivated_and_soft_delete.sql').
-- The Go paths writing these columns (SetActivation, any commune removal) must be reverted in the
-- same release, or they fail on a missing column.
--
-- WHAT IS LOST: deactivated_at values — recoverable: the backfilled ones from this file's
-- deactivated_at.recorded entries, later ones from SetActivation's entries (append-only, 0002, never
-- touched by reversal):
--
--   SELECT tenant_id, delta->>'deactivated_at', delta->>'source' FROM audit_log
--   WHERE action = 'deactivated_at.recorded' AND actor_kind = 'system'
--     AND delta->>'qua' = 'migration 0026_tenant_deactivated_and_soft_delete.sql';
--
-- ONCE ANY COMMUNE HAS BEEN REMOVED (deleted_at set), dropping deleted_at would silently bring it
-- back into every read that does not also check dang_hoat_dong — that is no longer a reversal but a
-- user decision with a verified backup (rule 7, stop condition #2).
-- ---------------------------------------------------------------------------

-- BACKSTOP, carried forward from 0002: a table declared PARTITION BY with no partitions rejects every
-- INSERT. This file declares no partitioned table; it runs anyway.
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
