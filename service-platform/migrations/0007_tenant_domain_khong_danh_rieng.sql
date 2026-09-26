-- platform — tenant_domain may never again hold a PLATFORM address (owner's decision, 2026-09-26).
--
-- WHY A NEW FILE AND NOT AN EDIT OF 0001: 0001–0006 have been applied and core/migrate compares
-- the checksum of every applied file at startup. An applied migration is immutable.
--
-- THE DOMAIN MODEL: `<xa>.vigov.vn` is a commune's prod web, `<xa>.stg.vigov.vn` its staging web,
-- `<service>.api.vigov.vn` / `<service>.api-stg.vigov.vn` a service's API, and `admin.vigov.vn` the
-- vendor's cross-commune console. None of the platform addresses is a commune, and a row mapping
-- one to a commune puts that commune in context on a surface that is not the commune's.
--
-- THE RULE, stated once in words — the CHECK below and domain.LaTenMienDanhRieng are the same rule,
-- held together by domain.TestLuatTenMienDanhRiengGoVaSQLKhop, which parses the patterns out of THIS
-- FILE and runs them beside the Go function over one table of hosts:
--   * the apex `vigov.vn` and `stg.vigov.vn`;
--   * a first label, directly under vigov.vn or stg.vigov.vn, of admin | admin-stg | api | api-stg
--     | stg | www;
--   * anything ending in `.api.vigov.vn` or `.api-stg.vigov.vn`.
-- A label merely CONTAINING one of those words (`apixa`, `xa-api-moi`) is a commune and allowed.
-- Case-insensitive (`!~*`) and tolerant of trailing dots, so the rule does not lean on
-- tenant_domain_host_thuong to be complete; the Go side lower-cases and strips them too.
--
-- NOT VALID, ON PURPOSE. Two rows violate the rule today: `admin.vigov.vn` and `admin-stg.vigov.vn`,
-- both mapped to Xã Thăng Bình by the deploy pipeline. NOT VALID makes PostgreSQL enforce the
-- CHECK on every INSERT and UPDATE from now on WITHOUT scanning existing rows, so those two rows
-- stay exactly as they are (rule 7 — the owner decides their removal). They stop RESOLVING because
-- the lookup refuses a reserved host before it reads the table (store.Directory, grpc.ResolveHost),
-- not because anything here touches them. `VALIDATE CONSTRAINT` must not be run until those rows
-- have been dealt with by the owner's decision: it would fail on them, and that failure is correct.
--
-- CONSEQUENCE FOR THE TWO KEPT ROWS: an UPDATE of either (e.g. flipping la_chinh) is now refused,
-- because PostgreSQL checks a NOT VALID constraint against the new row version. That is wanted:
-- the rows are frozen evidence of a past mapping, not rows to keep maintaining.
--
-- THE FIVE MIGRATION QUESTIONS:
--
--   1. HOW MANY ROWS PER COMMUNE: tenant_domain holds a handful of hosts per commune (more after a
--      merger). This file writes no row and reads none — NOT VALID skips the scan.
--   2. IF IT STOPS HALF-WAY: it cannot. One statement plus the backstop, one transaction with the
--      runner's progress row. A failed run leaves nothing behind and a retry is free.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. WHICH READ PATHS CHANGE MEANING WHILE IT IS HALF-APPLIED: none — a CHECK changes no read.
--      The reads that DO change (reserved hosts stop resolving) change in the Go code shipped with
--      this file, and they change whether or not this file has been applied.
--   5. RETENTION: nothing is removed or rewritten. The two violating rows are kept.

ALTER TABLE tenant_domain
    ADD CONSTRAINT tenant_domain_khong_danh_rieng CHECK (
        host !~* '^((admin|admin-stg|api|api-stg|stg|www)\.)?(stg\.)?vigov\.vn\.*$'
        AND host !~* '\.(api|api-stg)\.vigov\.vn\.*$'
    ) NOT VALID;

COMMENT ON CONSTRAINT tenant_domain_khong_danh_rieng ON tenant_domain IS
    'Ten mien cua nen tang (admin, api, stg, www, goc vigov.vn, *.api.vigov.vn) khong bao gio la '
    'cua mot xa. NOT VALID: hai dong admin cu duoc giu nguyen (luat 7), chi khong phan giai nua.';

-- ---------------------------------------------------------------------------
-- REVERSAL (rule 7, invariant 4).
--
-- Exact at any time, because the constraint owns no data: drop the constraint
-- tenant_domain_khong_danh_rieng from tenant_domain (ALTER TABLE ... DROP CONSTRAINT), then remove
-- this file's progress row from the schema_migration table, keyed on
-- ten = '0007_tenant_domain_khong_danh_rieng.sql', in one transaction. Written as prose rather than
-- as a runnable line, because a runnable line is a line that gets run.
--
-- Reversing this does NOT make reserved hosts resolve again — the Go refusal is independent of it,
-- deliberately, so that neither layer relies on the other having been deployed.
--
-- PER-COMMUNE AND RESUMABLE (rule 7, invariant 5): no backfill, no existing row rewritten,
-- nothing to resume.
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
