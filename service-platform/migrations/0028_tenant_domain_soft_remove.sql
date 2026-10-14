-- 0028 — a commune's domain can be REMOVED, softly, and the removed host can be taken again.
-- Owner decisions 14/10/2026 (ADR 0048 §Sửa đổi 14/10/2026), reversing §01/10 #5 ("no domain removal"):
--   * the operator console removes a NON-primary domain of a commune (the primary is moved first);
--   * the operator re-types the domain and gives a reason;
--   * once removed, the host is FREE — another commune may be given it.
--
-- WHAT CHANGES IN THE TABLE:
--   * removed_at / removed_by / remove_reason — the soft-delete triple (rule 7 invariant 1), same
--     shape as tenant.deleted_* (0026): the three move together, removed_by is an operator's
--     business code `VH-…` (rule 6 invariant 8), the reason is never blank. A removed row is never
--     the primary (la_chinh = false) — removing the primary is refused, and this CHECK is the
--     database's copy of that refusal.
--   * the PRIMARY KEY (host) is replaced by a PARTIAL UNIQUE INDEX on host WHERE removed_at IS NULL.
--     "One host names at most one commune" still holds for every LIVE row — that is the invariant
--     the edge's isolation rests on (0001's note on this table). What the key no longer does is keep
--     a removed host reserved forever, which is exactly the owner's decision. Removed rows of one
--     host may pile up (removed from commune A, given to B, removed again …); each is a record of one
--     act and none of them is ever resolved.
--
-- CONSEQUENCE, stated — it couples this file to Go in the SAME release (migrations are embedded):
-- every read of tenant_domain that answers "which commune holds this host" or "which hosts does this
-- commune hold" MUST filter `removed_at IS NULL` (rule 7 invariant 2). Without it a removed host
-- would keep resolving to the commune it was removed from — and, once re-used, a host lookup could
-- return either commune. store/directory.go, store/operator_registry.go, store/operator_writes.go and
-- store/shared_mini_app.go are changed in the commit that adds this file.
--
-- `ON CONFLICT (host)` (deploy/Jenkinsfile, stage doi-ten-mien-thang-binh — a one-off stage that has
-- already run) stops working with no primary key: PostgreSQL only infers a PARTIAL index from
-- `ON CONFLICT (host) WHERE removed_at IS NULL`. The stage refuses before it writes on a re-run, so
-- it is not changed here.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. ROWS PER COMMUNE: none written. Columns are added (NULL everywhere), one index is built, one
--      constraint dropped, one CHECK added that every existing row passes (all three columns NULL).
--   2. IF IT STOPS HALF-WAY: it cannot — one file, one transaction with the runner's progress row.
--      A re-run is free: every column and index is added only if missing, the key is dropped only
--      if present, the CHECK sits behind a pg_constraint lookup.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. READ PATHS THAT CHANGE MEANING: none while no row is removed — every row is live, so
--      `removed_at IS NULL` is true for all of them. See CONSEQUENCE above for after.
--   5. RETENTION: nothing dropped, retyped or renumbered. The primary key constraint is replaced by
--      an index enforcing the same uniqueness over the same (today: all) rows.
--
-- AUDIT (rule 6): this file writes no business row, so no entry. Every removal writes its own
-- `go_ten_mien` entry in the commune's audit_log, in the same transaction (RemoveDomain).
--
-- PERSONAL DATA (rule 3): none. Host names, an operator's business code, a reason about a commune.
--
-- NAMES (rule 12, ADR 0051): English. The table's older columns are Vietnamese; they are not renamed.
-- ---------------------------------------------------------------------------

ALTER TABLE tenant_domain ADD COLUMN IF NOT EXISTS removed_at    TIMESTAMPTZ NULL;
ALTER TABLE tenant_domain ADD COLUMN IF NOT EXISTS removed_by    TEXT        NULL;
ALTER TABLE tenant_domain ADD COLUMN IF NOT EXISTS remove_reason TEXT        NULL;

-- The index BEFORE the key is dropped, so no moment of this transaction leaves host unguarded.
--
-- @scope:  platform
-- Same argument as 0001's on this table: a host that resolved to two communes would make isolation
-- undecidable at the edge, so host is unique WITHOUT tenant_id. PARTIAL on purpose, and NOT the
-- "re-issued code" shape rule 7 invariant 3 forbids: a host is an address, not a code printed on a
-- record, and freeing a removed one is the owner's explicit decision of 14/10/2026 (ADR 0048
-- §Sửa đổi 14/10/2026 #2) — the one condition tools/check_khoa_duy_nhat.py names for a partial key.
CREATE UNIQUE INDEX IF NOT EXISTS tenant_domain_live_host
    ON tenant_domain (host) WHERE removed_at IS NULL;

ALTER TABLE tenant_domain DROP CONSTRAINT IF EXISTS tenant_domain_pkey;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint
                   WHERE conrelid = 'tenant_domain'::regclass
                     AND conname = 'tenant_domain_removed_complete') THEN
        ALTER TABLE tenant_domain ADD CONSTRAINT tenant_domain_removed_complete
            CHECK ((removed_at IS NULL AND removed_by IS NULL AND remove_reason IS NULL)
                OR (removed_at IS NOT NULL AND removed_by ~ '^VH-[0-9]{5,}$'
                    AND btrim(remove_reason) <> '' AND NOT la_chinh));
    END IF;
END $$;

-- Lookups by host now filter on removed_at; the partial unique index above serves them. This one
-- serves hostFree's cooldown question over removed rows. Not unique.
-- @scope:  platform
CREATE INDEX IF NOT EXISTS tenant_domain_removed_by_host
    ON tenant_domain (host, removed_at) WHERE removed_at IS NOT NULL;

COMMENT ON COLUMN tenant_domain.removed_at IS
    'Soft removal (rule 7): the host stops naming this commune and may be given to another. Set with '
    'removed_by and remove_reason. Never on the primary host. Owner decision 14/10/2026.';
COMMENT ON COLUMN tenant_domain.removed_by IS
    'Business code of the operator who removed the domain (VH-...), never an internal id.';
COMMENT ON COLUMN tenant_domain.remove_reason IS
    'Why the domain was removed, as the operator wrote it. Never blank when removed_at is set.';

-- ---------------------------------------------------------------------------
-- REVERSAL (rule 7, invariant 4). Run by a person, in ONE transaction; core/migrate has no automatic
-- rollback (ADR 0013). Prose, not runnable — a runnable line is a line that gets run.
--
-- WHILE NO ROW IS REMOVED (SELECT count(*) FROM tenant_domain WHERE removed_at IS NOT NULL = 0):
-- add back PRIMARY KEY (host) as tenant_domain_pkey, drop index tenant_domain_removed_by_host,
-- index tenant_domain_live_host, constraint tenant_domain_removed_complete, columns remove_reason,
-- removed_by, removed_at, and remove this file's row from `schema_migration`
-- (ten = '0028_tenant_domain_soft_remove.sql'). The Go paths that read or write these columns must be
-- reverted in the same release.
--
-- ONCE ANY ROW IS REMOVED, the primary key cannot come back (a host removed and re-used has two
-- rows), and dropping removed_at would bring every removed host back to the commune it was removed
-- from. That is no longer a reversal but a user decision with a verified backup (rule 7 stop #2).
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
