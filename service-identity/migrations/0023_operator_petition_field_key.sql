-- 0023 — the SEVENTH operator key, `ops.petition_field.manage` (ADR 0073 #3, owner 04/10/2026):
-- editing the tier-1 petition field codes (ADR 0060) in the operator area of service-platform.
--
-- WHAT CHANGES: the closed list held by the CHECK operator_permission_grant_key_known (0012) grows
-- from six keys to seven. Nothing else. The Go copy of the list is domain.OperatorPermissions();
-- internal/store/operatorstore TestPermissionCheckMatchesDomainList compares the two, reading the
-- NEWEST migration that defines the constraint — this file from now on.
--
-- NOT A ROW OF `quyen`, on purpose (ADR 0048 §28/09 #3): `quyen` is a commune's register; an
-- operator key is a cross-commune right of the operator realm and is granted straight to an account
-- (operatorctl grant / the Jenkins job tao-tai-khoan-van-hanh). tools/check_quyen.py therefore has
-- nothing to check here.
--
-- WHY A NEW FILE AND NOT AN EDIT TO 0012: core/migrate checksums every applied file at startup and
-- stops the service when one has changed.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
-- 1. HOW MANY ROWS PER COMMUNE. None — the table has no commune column. No row is written: the key
--    is GRANTED TO NOBODY here. An operator receives it through operatorctl grant (trailed in
--    operator_audit_log), exactly like the six keys before it.
--
-- 2. IF IT STOPS HALF-WAY. It cannot: core/migrate runs the file in ONE transaction with its
--    progress row, and the DROP + ADD below are one ALTER TABLE statement. A retry is free —
--    DROP CONSTRAINT IF EXISTS, then the same ADD.
--
-- 3. HOW IT IS REVERSED. See REVERSAL at the bottom.
--
-- 4. WHICH READ PATHS CHANGE MEANING WHILE IT IS HALF-APPLIED. None. The CHECK only WIDENS: every
--    row valid under 0012's six keys is valid under these seven, so re-validating existing grants
--    cannot fail, and no read path filters on the constraint. Until this file runs, a grant of the
--    seventh key is refused by the database — the closed direction.
--
-- 5. RETENTION. Nothing is removed, retyped or renumbered. Grants are never deleted (0012).
--
-- LOCKING: ALTER TABLE takes an ACCESS EXCLUSIVE lock on operator_permission_grant for the length of
-- one re-validation over a table of a few dozen rows — operator sign-ins resolving keys wait
-- milliseconds. PostgreSQL 16 (the real cluster) supports every statement here.
--
-- PER-COMMUNE AND RESUMABLE (rule 7, invariant 5): no backfill, no commune to iterate.
-- ---------------------------------------------------------------------------

ALTER TABLE operator_permission_grant
    DROP CONSTRAINT IF EXISTS operator_permission_grant_key_known,
    ADD CONSTRAINT operator_permission_grant_key_known CHECK (permission_key IN (
        'ops.tenant.manage', 'ops.domain.manage', 'ops.profile.manage',
        'ops.mini_app.manage', 'ops.upload_policy.manage', 'ops.qr.issue',
        'ops.petition_field.manage'));

-- ---------------------------------------------------------------------------
-- REVERSAL (rule 7, invariant 4).
--
-- While NO grant of the seventh key exists (live or revoked): put back 0012's six-key CHECK in one
-- ALTER TABLE (drop the constraint, add it with the six keys), then remove this file's progress row
-- from `schema_migration`, keyed on ten = '0023_operator_petition_field_key.sql'.
--
-- ONCE A GRANT EXISTS it is not a reversal: the six-key CHECK would refuse to validate the table,
-- and grants are never deleted (0012) — they are the history of who held which right. Shrinking the
-- closed list then is an owner decision (ADR 0048 stop condition #1).
--
-- Written as prose rather than as runnable lines, because a runnable line is a line that gets run.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- BACKSTOP, carried forward from 0002–0022: a partitioned table with no partitions rejects every
-- INSERT. This file adds no table, so it is expected to find nothing; it runs anyway, because the
-- check only describes the state after the newest migration that carries it.
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
