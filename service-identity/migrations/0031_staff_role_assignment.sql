-- 0031 — a member of staff may hold SEVERAL roles (owner decision 13/10/2026, customer sheet
-- web-admin-131026 row 10; the prototype's user form is a checkbox grid that sends `role_ids`,
-- ../vigov-require/apps/admin/src/components/admin/UserFormDialog.tsx:223-245). This REVERSES the
-- "one role per staff" choice of 08/10/2026 (domain.NewStaffRoles carries the history).
--
-- WHY A TABLE AND NOT AN ARRAY COLUMN ON `nguoi_dung`: the permission check joins person → role →
-- grant, bound to the commune at every table (store/checker.go). An array cannot carry a composite
-- foreign key to `vai_tro (tenant_id, id)`, so another commune's role id — or an invented one — would
-- sit in the column unrefused, and the soft removal rule 7 requires (who removed which role, when,
-- why) has no row to live on.
--
-- `nguoi_dung.vai_tro_id` IS KEPT, AND KEPT IN SYNC AS THE PRIMARY ROLE. Rule 7 forbids a lossy
-- migration, and nine readers still use the column for DISPLAY (the register's role column, the
-- sign-in account list, BatchGetStaff's one role, GET /api/v1/sessions/current's `role`). Every write
-- path keeps it equal to the FIRST role of the set, NULL when the set is empty (store.SetStaffRoles).
-- WHO HOLDS A PERMISSION is decided by THIS table only — store.noiGiuQuyen.
--
-- ENTITY OWNERSHIP via the `-- @entity` / `-- @scope` marks (ADR 0021); tools/kb reads them into
-- kb/30-indexes/data-ownership.json. Names are English (rule 12, ADR 0051).
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. ROWS PER COMMUNE: one row per staff row whose `vai_tro_id` is set — tens per commune. Nothing
--      in `nguoi_dung` is rewritten.
--   2. IF IT STOPS HALF-WAY: it cannot — core/migrate runs the file in ONE transaction with its
--      progress row. The backfill is ON CONFLICT DO NOTHING, so a retry adds nothing twice.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. READ PATHS THAT CHANGE MEANING WHILE HALF-APPLIED: none. The code that reads this table ships
--      with this file; until it runs, the backfilled rows say exactly what `vai_tro_id` says.
--   5. RETENTION: a removed role is SOFT-removed (deleted_at / deleted_by / delete_reason) and the row
--      kept, so "since when did this person hold this role" stays answerable beside the audit trail.
-- ---------------------------------------------------------------------------

-- @entity: StaffRoleAssignment
-- @scope:  tenant
CREATE TABLE IF NOT EXISTS staff_role_assignment (
    tenant_id      TEXT        NOT NULL,
    staff_id       TEXT        NOT NULL,   -- nguoi_dung.id
    role_id        TEXT        NOT NULL,   -- vai_tro.id

    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- The STAFF CODE of whoever assigned it (rule 6, invariant 8), or 'system' (backfill, seeding).
    created_by     TEXT        NOT NULL,

    deleted_at     TIMESTAMPTZ,
    deleted_by     TEXT,
    delete_reason  TEXT,

    -- ONE LIVE ROW PER (commune, person, role) — the `sla.linh_vuc_khoa` shape (0008): TRUE while the
    -- row is live, NULL once soft-removed, and NULLs never collide, so any number of removed rows may
    -- sit beside the live one. A partial unique index on a partitioned table is the alternative this
    -- schema has avoided since 0008.
    live_key       BOOLEAN GENERATED ALWAYS AS (CASE WHEN deleted_at IS NULL THEN TRUE END) STORED,

    -- created_at in the key: a role removed and given back later is a SECOND row, never an edit of
    -- the first (rule 7, forbidden #5). Composite with tenant_id (rule 1, invariant 6).
    PRIMARY KEY (tenant_id, staff_id, role_id, created_at),
    CONSTRAINT staff_role_assignment_live_unique UNIQUE (tenant_id, staff_id, role_id, live_key),

    -- Composite with the commune, so another commune's person or role is refused by the key itself.
    CONSTRAINT staff_role_assignment_staff_fkey
        FOREIGN KEY (tenant_id, staff_id) REFERENCES nguoi_dung (tenant_id, id),
    CONSTRAINT staff_role_assignment_role_fkey
        FOREIGN KEY (tenant_id, role_id) REFERENCES vai_tro (tenant_id, id),

    CONSTRAINT staff_role_assignment_created_by_not_blank CHECK (btrim(created_by) <> ''),

    -- Soft-delete columns are all three or none (rule 7, invariant 1).
    CONSTRAINT staff_role_assignment_soft_delete_complete
        CHECK ((deleted_at IS NULL AND deleted_by IS NULL AND delete_reason IS NULL)
            OR (deleted_at IS NOT NULL AND deleted_by IS NOT NULL AND btrim(deleted_by) <> ''
                AND delete_reason IS NOT NULL AND btrim(delete_reason) <> ''))
);

COMMENT ON TABLE staff_role_assignment IS
    'Vai tro cua can bo — mot can bo giu nhieu vai tro (chu dau tu chot 13/10/2026). Quyen = hop cac '
    'vai tro dang giu. nguoi_dung.vai_tro_id giu lai lam vai tro chinh, dong bo voi dong dau tien.';

-- The permission check's read: this person's live roles, in this commune.
CREATE INDEX IF NOT EXISTS staff_role_assignment_by_staff
    ON staff_role_assignment (tenant_id, staff_id) WHERE deleted_at IS NULL;
-- The Phân quyền matrix's holder count: live holders of one role.
CREATE INDEX IF NOT EXISTS staff_role_assignment_by_role
    ON staff_role_assignment (tenant_id, role_id) WHERE deleted_at IS NULL;

-- ---------------------------------------------------------------------------
-- BACKFILL: one row per existing `nguoi_dung.vai_tro_id`, with one SYSTEM audit entry per commune in
-- the SAME statement (rule 6, invariants 3 and 6; the data-modifying CTE shape of 0024 / 0028).
--
-- @cross-tenant: one-off system migration copying every commune's existing role column into the new
-- table; each row keeps its own tenant_id and each commune gets its own entry.
--
-- SOFT-DELETED PEOPLE ARE COPIED TOO: the table mirrors the column exactly, and every reader already
-- filters `nd.deleted_at`. NO PERMISSION CHANGES: the set each person holds is the one role they held.
-- ---------------------------------------------------------------------------
WITH copied AS (
    INSERT INTO staff_role_assignment (tenant_id, staff_id, role_id, created_at, created_by)
    SELECT nd.tenant_id, nd.id, nd.vai_tro_id, now(), 'system'
    FROM nguoi_dung nd
    WHERE nd.vai_tro_id IS NOT NULL
    ON CONFLICT DO NOTHING
    RETURNING tenant_id
), per_commune AS (
    SELECT tenant_id, count(*) AS copied_rows
    FROM copied
    GROUP BY tenant_id
)
INSERT INTO audit_log (tenant_id, actor_id, actor_kind, actor_ip, action, subject, at, delta)
SELECT tenant_id, 'system', 'system', '', 'gan_vai_tro_ban_dau', 'staff_role_assignment', now(),
       jsonb_build_object(
           'so_dong', copied_rows,
           'nguon',   'nguoi_dung.vai_tro_id',
           'ly_do',   'migration 0031: một cán bộ giữ nhiều vai trò (chủ đầu tư chốt 13/10/2026); sao vai trò hiện có, không đổi quyền')
FROM per_commune;

-- ---------------------------------------------------------------------------
-- REVERSAL (rule 7, invariant 4). By a person, in ONE transaction, after the Go code that reads this
-- table has been rolled back; core/migrate has no automatic rollback (ADR 0013). Prose, not a
-- runnable line, because a runnable line gets run.
--
--   WHILE NOBODY HOLDS A SECOND ROLE — check first that no (tenant_id, staff_id) has more than one
--   live row; then drop staff_role_assignment (its indexes go with it) and remove this
--   file's row from schema_migration. `nguoi_dung.vai_tro_id` still names each person's one role,
--   because every write kept it equal to the first role. The backfill's audit entries stay (rule 6).
--
--   ONCE SOMEBODY HOLDS TWO — the extra roles exist ONLY here. Dropping the table silently takes
--   permissions away from those people. List them first (live rows whose role_id differs from
--   nguoi_dung.vai_tro_id), have an administrator decide each case on the Người dùng screen, and only
--   then drop. That is the owner's decision, never a command.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- BACKSTOP, carried forward from 0002–0030: every partitioned table in this schema must have
-- partitions, or it rejects every INSERT.
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
                         'MODULUS 8 partition loop (ADR 0096) in the same migration that '
                         'declares PARTITION BY.';
    END IF;
END $$;
