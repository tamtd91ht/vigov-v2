-- finance — audit_log: turn "append-only" from a comment into a database constraint.
--
-- WHY A NEW FILE INSTEAD OF EDITING 0001_init.sql: 0001 has already been applied. Editing an
-- applied migration leaves the schema in the repository and the schema in the database
-- disagreeing, and nothing reports the difference. Rule 7 forbids that while the data is still
-- only test data too, because the habit is what gets repeated once the data is real.
--
-- WHAT WAS WRONG: 0001_init.sql says audit_log is "append-only ... not even by an
-- administrator" — in a comment. A comment enforces nothing. Until this file, the only thing
-- holding the invariant up was that nobody had yet written an UPDATE against audit_log in Go.
-- Rule 6 invariant 4 requires entries that cannot be modified or removed: a ledger that can be
-- edited has no evidentiary value, and evidentiary value is the only reason the ledger exists.
--
-- RE-RUNNABLE, like 0001: CREATE OR REPLACE for the function, DROP TRIGGER IF EXISTS before
-- each CREATE TRIGGER. The integration suites apply migrations more than once. Note that
-- PostgreSQL has no CREATE TRIGGER IF NOT EXISTS, and CREATE OR REPLACE TRIGGER only exists
-- from version 14 — drop-then-create is the form that works everywhere and says what it does.

-- ---------------------------------------------------------------------------
-- PostgreSQL 13 is the floor, checked explicitly so the failure is readable.
--
-- BEFORE ... FOR EACH ROW triggers on a PARTITIONED table were only allowed from PostgreSQL 13
-- ("Allow BEFORE row-level triggers on partitioned tables"). On 11 and 12 the CREATE TRIGGER
-- below fails with "Partitioned tables cannot have BEFORE / FOR EACH ROW triggers", which
-- reads like a syntax mistake and invites somebody to "fix" it by moving the trigger down onto
-- the partitions — where a partition added later arrives silently unprotected.
--
-- Failing loudly here is the point. A migration that quietly skips the protection is worse
-- than no migration: the comment in 0001 would then be joined by a file that also claims to
-- enforce something and does not.
-- ---------------------------------------------------------------------------
DO $$ BEGIN
    IF current_setting('server_version_num')::int < 130000 THEN
        RAISE EXCEPTION
            'audit_log append-only needs PostgreSQL 13 or newer (server is %). Do not weaken '
            'this migration to fit an older server — the invariant is rule 6 #4.',
            current_setting('server_version');
    END IF;
END $$;

-- ---------------------------------------------------------------------------
-- The guard function. It never returns: there is no legitimate caller to let through, because
-- core/audit.Write only ever INSERTs. Nothing in the system loses a path it was using.
--
-- The message names the operation and the relation and nothing else. It must never name the
-- row: `delta` holds before/after values of business fields, and an error message travels into
-- logs and back to clients (rule 3, forbidden #3). TG_TABLE_NAME reports the leaf partition
-- that was actually hit, which is what an operator needs in order to find out what happened.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION audit_log_append_only() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'audit_log is append-only: % on % refused', TG_OP, TG_TABLE_NAME
        USING HINT = 'Audit entries are archival records (rule 6, invariant 4): never '
                     'modified, never removed, never truncated — not even by an '
                     'administrator. To correct a wrong entry, append a new one.';
END $$;

-- ---------------------------------------------------------------------------
-- UPDATE and DELETE — trigger on the PARENT, row level.
--
-- audit_log is PARTITION BY HASH (tenant_id), MODULUS 32 (ADR 0010), so WHERE this trigger is
-- attached decides whether it does anything at all:
--
--   * A row-level trigger created on the parent is CLONED onto every existing partition, and
--     onto every partition created or attached afterwards. That second half is the reason the
--     parent is the right place: a partition added by some future migration inherits the guard
--     without anybody having to remember it.
--   * UPDATE and DELETE against a partitioned table are carried out on the LEAF partition, so
--     the clone on the leaf is what fires — equally for `UPDATE audit_log` routed by the
--     planner and for `UPDATE audit_log_p07` typed straight into psql. The second case is the
--     one that matters here: an administrator at a psql prompt is exactly the threat rule 6
--     invariant 4 names, and that administrator can address a partition by name.
--   * FOR EACH STATEMENT would NOT do the job: statement-level triggers are not cloned to
--     partitions, so the direct `UPDATE audit_log_p07` above would walk straight past one.
--
-- INSERT is deliberately absent from the event list. The write path stays untouched and pays
-- nothing: audit entries are written inside the business transaction (rule 6, invariant 3), so
-- a cost added here would be a cost on every business write in the service.
-- ---------------------------------------------------------------------------
DROP TRIGGER IF EXISTS audit_log_no_update_delete ON audit_log;
CREATE TRIGGER audit_log_no_update_delete
    BEFORE UPDATE OR DELETE ON audit_log
    FOR EACH ROW EXECUTE FUNCTION audit_log_append_only();

-- ---------------------------------------------------------------------------
-- TRUNCATE — attached per partition, because it cannot be attached to the parent.
--
-- The trigger above does not see TRUNCATE, and TRUNCATE is the one command that empties the
-- whole ledger on a single line. Leaving that open would make everything above decorative.
--
-- PostgreSQL refuses a TRUNCATE trigger on a partitioned table ("Partitioned tables cannot
-- have TRUNCATE triggers"), and statement-level triggers are not cloned to partitions either.
-- A leaf partition, however, is an ordinary table and accepts one. TRUNCATE against the parent
-- expands to the leaves and fires their BEFORE TRUNCATE triggers, so covering the leaves
-- covers the parent as well.
--
-- THE GAP THIS LEAVES, stated instead of hidden: a partition created AFTER this migration gets
-- the UPDATE/DELETE clone automatically but NOT this TRUNCATE trigger. MODULUS 32 is fixed for
-- the life of the system (ADR 0010), so no new partition is expected; if one is ever added,
-- re-run this file — which is why it is written to be re-runnable.
-- ---------------------------------------------------------------------------
DO $$
DECLARE part regclass;
BEGIN
    FOR part IN
        SELECT inhrelid::regclass FROM pg_inherits
        WHERE inhparent = 'audit_log'::regclass
    LOOP
        EXECUTE format('DROP TRIGGER IF EXISTS audit_log_no_truncate ON %s', part);
        EXECUTE format(
            'CREATE TRIGGER audit_log_no_truncate BEFORE TRUNCATE ON %s '
            'FOR EACH STATEMENT EXECUTE FUNCTION audit_log_append_only()', part);
    END LOOP;
END $$;

-- ---------------------------------------------------------------------------
-- REVOKE is deliberately NOT here. This is the part worth arguing with, so here is the
-- argument in full.
--
-- The application role is whatever DATABASE_DSN connects as, and that differs per environment
-- (rule 8, invariant 5: environment-specific values do not belong in source). A migration that
-- names a role breaks in the first environment that spells it differently — and a migration
-- that breaks is a migration somebody edits under pressure.
--
-- The three ways to write it anyway, and why each is worse than leaving it out:
--
--   1. REVOKE ... FROM current_user — in this system the migration runs on the same connection
--      as the application (the integration suites apply the migration and then run the queries
--      through one handle), so current_user is the application role AND the table owner. An
--      owner grants the privilege back to itself in one statement, so this records an
--      intention while looking like a control. Something that looks like a control and is not
--      is how an invariant stops being checked.
--   2. A role name read from a setting, e.g. current_setting('vigov.app_role') — nothing sets
--      such a setting anywhere in this repository, so the block would be a no-op in every
--      environment while reading as protection. The worst of the three.
--   3. A literal role name — breaks in the first environment that does not match.
--
-- So REVOKE belongs to database provisioning, where the role name is known. Kept here so the
-- knowledge does not go missing:
--
--      REVOKE UPDATE, DELETE, TRUNCATE ON audit_log FROM <app_role>;
--      REVOKE UPDATE, DELETE, TRUNCATE ON audit_log_p00, ... , audit_log_p31 FROM <app_role>;
--
-- The second line is not redundant. Privileges are checked against the relation NAMED in the
-- statement and are not inherited from the parent, so `UPDATE audit_log_p07` is checked
-- against p07's own ACL. A REVOKE on the parent alone leaves all 32 partitions open — the same
-- trap as putting the trigger in the wrong place, in the privilege layer.
--
-- The trigger above covers what REVOKE cannot reach in any case: the owner and the superuser,
-- for whom privilege checks are skipped entirely.
--
-- WHAT NOTHING IN THIS FILE STOPS, said plainly: ALTER TABLE ... DISABLE TRIGGER, and dropping
-- the table, both available to the owner. Those are deliberate, single-purpose DDL acts that
-- leave a trace in the server log. This file exists to make the accidental and the casual
-- impossible — an UPDATE written by mistake in Go, a DELETE typed at a psql prompt — not to
-- defeat an administrator who has decided to destroy the record and is willing to be seen
-- doing it. Making that second case impossible is a database-permissions and backup question,
-- not a schema question.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- BACKSTOP: every partitioned table in this schema must actually have partitions.
--
-- WHY THIS EXISTS: `PARTITION BY HASH` without a partition-creation loop produces a table that
-- REJECTS EVERY INSERT ("no partition of relation ... found for row"). The fault is SILENT --
-- the migration succeeds, the schema looks correct, the table is listed -- and it surfaces at
-- the first INSERT. In a business service the first INSERT is a staff member receiving a
-- citizen's report, so the fault reaches a counter before it reaches a developer.
--
-- Not hypothetical: audit_log in comms, documents, dossiers, finance, petitions and reporting
-- was declared partitioned and given no partitions, and the handover note had already named
-- this exact trap. A trap the project already knows about and still falls into needs something
-- that CHECKS, not something that reminds.
--
-- SCOPED TO current_schema() ON PURPOSE: the integration suites create one schema per run in a
-- shared database, so a database-wide check would see another suite's half-built schema and
-- fail for reasons having nothing to do with this migration.
--
-- WHAT IT DOES NOT COVER, said plainly: it verifies the state after the NEWEST migration that
-- carries the block. A future 0003 that declares a partitioned table and forgets its partitions
-- is only caught if 0003 ends with this block too. That convention is unenforced by SQL; the
-- durable form of this check belongs in the repository's own verification layer.
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
