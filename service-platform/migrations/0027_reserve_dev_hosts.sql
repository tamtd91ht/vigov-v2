-- 0027 — tenant_domain_khong_danh_rieng now also reserves the DEV environment's platform hosts.
-- Owner decisions 14/10/2026: environments are `vigov-prod` and `vigov-dev`; staging is removed.
--
-- WHY: deploy/hosts.yaml gained the `dev` environment — commune web `<xa>.dev.vigov.vn`, service API
-- `<service>-dev.api.vigov.vn`, operator console `admin-dev.vigov.vn`. The Go half of the rule
-- (domain.LaTenMienDanhRieng) follows that file on the next `make kb`; this CHECK cannot read it, so
-- without this file a commune row could still be created on `dev.vigov.vn`, `admin-dev.vigov.vn` or
-- `admin.dev.vigov.vn` — an address the platform itself answers on.
--
-- WHY A NEW FILE AND NOT AN EDIT OF 0007: 0007 has been applied and core/migrate compares the checksum
-- of every applied file at startup. An applied migration is immutable.
--
-- THE RULE, stated once in words — the CHECK below and domain.LaTenMienDanhRieng are the same rule,
-- held together by domain.TestLuatTenMienDanhRiengGoVaSQLKhop, which parses the patterns out of THIS
-- FILE (the latest definition) and runs them beside the Go function over one table of hosts:
--   * the apex `vigov.vn`, and the environment roots `dev.vigov.vn` and `stg.vigov.vn`;
--   * a first label, directly under any of those three roots, of
--     admin | admin-dev | admin-stg | api | api-stg | dev | stg | www;
--   * anything ending in `.api.vigov.vn` (prod AND dev: `comms.` and `comms-dev.`) or
--     `.api-stg.vigov.vn`.
-- Changes from 0007: root `dev.vigov.vn`; labels `admin-dev` and `dev`. Nothing is released.
--
-- STAGING STAYS RESERVED, ON PURPOSE (owner, 14/10/2026): staging is retired, but prod's database still
-- holds a `<xa>.stg.vigov.vn` row from when staging shared it. `stg.vigov.vn`, `admin-stg`, `api-stg`
-- and `.api-stg.vigov.vn` remain refused, and commune labels under stg.vigov.vn keep the meaning 0007
-- gave them — so that row neither becomes a violation nor frees its address for anyone else.
--
-- NO `api-dev` LABEL: dev's API hosts live under `api.vigov.vn`, whose label `api` is already
-- reserved; `api-dev.vigov.vn` is not an address any surface uses.
--
-- SAME NAME, SAME SHAPE: store/operator_writes.go maps a violation of `tenant_domain_khong_danh_rieng`
-- to ErrCommuneHostReserved by NAME (reservedHostConstraint) — a renamed constraint would turn that
-- refusal into a 500. Two `host !~* '...'` patterns, NOT VALID, as 0007: the parity test reads exactly
-- that shape.
--
-- NOT VALID, for 0007's reason: the frozen `admin.vigov.vn` / `admin-stg.vigov.vn` rows still violate
-- the rule and are kept (rule 7 — the owner decides their removal). NOT VALID enforces the CHECK on
-- every INSERT and UPDATE from now on without scanning existing rows. A row that violates only the NEW
-- clauses (a commune already mapped to e.g. `admin-dev.vigov.vn`) is kept the same way and stops
-- resolving through the Go refusal; none is known.
--
-- THE FIVE MIGRATION QUESTIONS:
--
--   1. HOW MANY ROWS PER COMMUNE: none written, none read — NOT VALID skips the scan. tenant_domain is
--      a handful of rows per commune.
--   2. IF IT STOPS HALF-WAY: it cannot. DROP and ADD run in one transaction with the runner's progress
--      row; a failure leaves 0007's constraint exactly in place. There is never a moment with no CHECK.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. WHICH READ PATHS CHANGE MEANING WHILE IT IS HALF-APPLIED: none — a CHECK changes no read. The
--      reads that do change (dev platform hosts stop resolving) change in the Go shipped with this
--      file, whether or not this file has been applied.
--   5. RETENTION: nothing is removed or rewritten.
--
-- PER-COMMUNE AND RESUMABLE (rule 7, invariant 5): no backfill, no row touched, nothing to resume.

ALTER TABLE tenant_domain DROP CONSTRAINT IF EXISTS tenant_domain_khong_danh_rieng;

ALTER TABLE tenant_domain
    ADD CONSTRAINT tenant_domain_khong_danh_rieng CHECK (
        host !~* '^((admin|admin-dev|admin-stg|api|api-stg|dev|stg|www)\.)?((dev|stg)\.)?vigov\.vn\.*$'
        AND host !~* '\.(api|api-stg)\.vigov\.vn\.*$'
    ) NOT VALID;

COMMENT ON CONSTRAINT tenant_domain_khong_danh_rieng ON tenant_domain IS
    'Ten mien cua nen tang (admin, admin-dev, api, dev, stg, www, goc vigov.vn / dev.vigov.vn / '
    'stg.vigov.vn, *.api.vigov.vn) khong bao gio la cua mot xa. NOT VALID: cac dong admin cu duoc '
    'giu nguyen (luat 7), chi khong phan giai nua. Dinh nghia lai o 0027.';

-- ---------------------------------------------------------------------------
-- REVERSAL (rule 7, invariant 4).
--
-- Exact at any time, because the constraint owns no data. In ONE transaction: drop the constraint
-- tenant_domain_khong_danh_rieng from tenant_domain, add it again with 0007's two patterns exactly as
-- written in 0007_tenant_domain_khong_danh_rieng.sql (NOT VALID), and remove this file's progress
-- row from the schema_migration table, keyed on ten = '0027_reserve_dev_hosts.sql'. Written as prose
-- rather than as a runnable line, because a runnable line is a line that gets run.
--
-- Reversing this does NOT make the dev platform hosts resolvable as communes — the Go refusal is
-- independent of it, deliberately, so that neither layer relies on the other having been deployed.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- BACKSTOP, carried forward from 0002: a table declared PARTITION BY with no partitions rejects
-- every INSERT. This file declares no partitioned table and is expected to find nothing; it runs
-- anyway, because the run after which it is missing is the one that needed it.
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
