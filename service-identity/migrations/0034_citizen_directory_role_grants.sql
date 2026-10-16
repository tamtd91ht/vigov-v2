-- 0034 — default grants of the two DANH BẠ NGƯỜI DÂN keys (seeded by 0028) to the staff roles the
-- prototype's permission matrix ticks, in every existing commune. No catalogue row is added.
--
--   citizen.read     → chanh-van-phong · truong-bo-phan · chuyen-vien · can-bo-mot-cua · truong-thon
--   citizen.update   → chanh-van-phong · can-bo-mot-cua · truong-thon
--
-- OWNER DECISION 16/10/2026. The owner's full list also names `chu-tich-ubnd` and `pho-chu-tich-ubnd`
-- for both keys: 0028 ALREADY GRANTED THEM (user decision 09/10/2026, 0028 §2), together with
-- `quan-tri-he-thong`, so they are NOT repeated here — on purpose, not only for tidiness. A grant
-- re-run on a role would silently undo a withdrawal an administrator made since 0028 (rule 5,
-- invariant 5: that withdrawal is an audited decision). This file touches only roles 0028 did not.
--
-- `tiep-nhan` IS `can-bo-mot-cua`. The prototype's matrix names a role code `tiep-nhan`; its own
-- catalogue declares that code with the name "Cán bộ một cửa"
-- (`../vigov-require/apps/api/app/modules/org/data/default_config.json:306-307`), and ADR 0055 §4
-- already maps the prototype's codes to this repository's (`chu-tich` → `chu-tich-ubnd`,
-- `tiep-nhan` → `can-bo-mot-cua`). So the grant lands on `can-bo-mot-cua` — the owner confirmed the
-- mapping on 16/10/2026 after an impact assessment of a ninth template role (a duplicate of this one
-- under a second code; rejected). migrations/citizen_directory_role_grants_test.go refuses any code
-- role_template.go does not carry.
--
-- THIS FILE CHANGES NO ROUTE. No route checks either key yet (0028's header); the grant is written now
-- so the day the directory's routes land, the people who use the screen already hold the key.
--
-- NEW COMMUNES get their roles from the templates (domain.RoleTemplates). The template change that
-- matches this file — the same six (role, key) pairs — belongs to internal/domain and ships in the
-- same commit by the service builder, so a commune seeded after this file and one seeded before end
-- in the same state (the 0028 / 0032 pattern). Until that lands, a new commune's five roles lack the
-- keys; role_template_test.go pins the per-role counts (20 / 12 / 8 / 6 / 4) and must move with it.
--
-- WHY A NEW FILE AND NOT AN EDIT TO 0028: core/migrate checksums every applied file at startup and
-- stops the service when one has changed (ErrChecksumLech).
--
-- NAMED IN ENGLISH (rule 12, ADR 0051). Role codes and keys are DATA (Vietnamese slugs, ADR 0011).
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
-- 1. HOW MANY ROWS PER COMMUNE. Catalogue: none. Grants: at most 8 rows of `vai_tro_quyen` per
--    commune (5 roles × citizen.read + 3 roles × citizen.update), only where those roles exist and
--    are live — a commune that never ran "Tạo vai trò mẫu" and never created them gets nothing. Plus
--    ONE audit_log entry per role that actually gained a key. The operator may measure first:
--      SELECT tenant_id, ma, count(*) FROM vai_tro
--      WHERE deleted_at IS NULL
--        AND ma IN ('chanh-van-phong', 'truong-bo-phan', 'chuyen-vien', 'can-bo-mot-cua', 'truong-thon')
--      GROUP BY tenant_id, ma;
--    PER COMMUNE AND RESUMABLE (rule 7, invariant 5): ONE STATEMENT, NOT A PER-COMMUNE LOOP, as in
--    0028 / 0031 / 0032 — core/migrate runs the file in one transaction with its progress row, so a
--    loop inside it could not be resumed any better. Acceptable because the change is PURELY ADDITIVE
--    AND IDEMPOTENT: it inserts grants, never rewrites or removes one, and a rerun inserts nothing.
--    Every grant row and every audit entry carries its own commune's tenant_id.
--
-- 2. IF IT STOPS HALF-WAY. It cannot land half-applied: one file, one transaction. A failure rolls
--    back every grant and every entry together. A retry costs nothing: the grant is ON CONFLICT DO
--    NOTHING, and the audit entry is built from the grant's own RETURNING — a key already held yields
--    neither a row nor an entry.
--
-- 3. HOW IT IS REVERSED. See REVERSAL at the bottom.
--
-- 4. WHICH READ PATHS CHANGE MEANING WHILE IT IS HALF-APPLIED. None can see it half-applied (one
--    transaction). AFTER IT: the Phân quyền matrix shows the two DANH BẠ NGƯỜI DÂN rows ticked for
--    five more columns. store.Checker answers "yes" for the two keys to holders of those roles — no
--    behaviour changes, because no route asks yet. POST /api/v1/roles/defaults (#14): the caller
--    must hold every key a template grants; the two keys are already in the chairman's template
--    (0028), so the set the caller must hold does not grow.
--
-- 5. RETENTION. Nothing is removed, retyped, overwritten or renumbered; no issued business code is
--    touched. Grants are configuration, not an archival record, but each one is audited with the
--    system principal (rule 6, invariant 6).
--
-- MERGED / DISSOLVED COMMUNES: identity does not hold a commune's active/inactive status, so this file
-- cannot exclude them. A merged commune's live roles with these codes gain keys no route checks.
-- Reported, not hidden.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- THE GRANT, with its audit entry IN THE SAME STATEMENT (data-modifying CTE, the shape of 0024 /
-- 0028 / 0031 / 0032), so a grant cannot land without its entry (rule 6, invariant 3).
--
-- @cross-tenant: one-off system migration granting the two directory keys to five staff roles, by
-- stable code, in every commune (owner decision 16/10/2026); each row keeps its own tenant_id and
-- each role gets its own entry in its own commune.
--
-- ROLE FILTER: live roles only (`vt.deleted_at IS NULL`), matched by CODE (`vai_tro.ma`, UNIQUE
-- (tenant_id, ma)), never by the editable name. A soft-deleted role is a commune's decision and is not
-- touched ("DELETED MEANS DECIDED", app/role_template.go). `vai_tro_quyen` has no soft delete
-- (0001:126-134): a row there IS a live grant.
--
-- THE ENTRY uses the verb the Phân quyền save writes (`luu_phan_quyen_vai_tro`) with the role id as
-- subject, and that save's delta shape — truoc / sau / them / bo — plus `ly_do`, exactly as 0028 /
-- 0032. `truoc` is read in the same statement, so it is the set held before this grant. Keys only;
-- nothing personal. Actor 'system' / kind 'system' (rule 6, invariant 6; domain.SystemActor).
-- ---------------------------------------------------------------------------
WITH grant_rule (role_code, granted) AS (
    VALUES ('chanh-van-phong', 'citizen.read'),
           ('truong-bo-phan',  'citizen.read'),
           ('chuyen-vien',     'citizen.read'),
           ('can-bo-mot-cua',  'citizen.read'),
           ('truong-thon',     'citizen.read'),
           ('chanh-van-phong', 'citizen.update'),
           ('can-bo-mot-cua',  'citizen.update'),
           ('truong-thon',     'citizen.update')
), target AS (
    SELECT vt.tenant_id, vt.id AS role_id, r.granted
    FROM vai_tro vt
    JOIN grant_rule r ON r.role_code = vt.ma
    WHERE vt.deleted_at IS NULL
), inserted AS (
    INSERT INTO vai_tro_quyen (tenant_id, vai_tro_id, quyen_ma, cap_luc, cap_boi)
    SELECT t.tenant_id, t.role_id, t.granted, now(), 'system'
    FROM target t
    ON CONFLICT (tenant_id, vai_tro_id, quyen_ma) DO NOTHING
    RETURNING tenant_id, vai_tro_id, quyen_ma
), per_role AS (
    SELECT i.tenant_id, i.vai_tro_id,
           array_agg(i.quyen_ma ORDER BY i.quyen_ma) AS added,
           COALESCE((SELECT array_agg(vq.quyen_ma ORDER BY vq.quyen_ma)
                     FROM vai_tro_quyen vq
                     WHERE vq.tenant_id = i.tenant_id AND vq.vai_tro_id = i.vai_tro_id),
                    ARRAY[]::text[]) AS held_before
    FROM inserted i
    GROUP BY i.tenant_id, i.vai_tro_id
)
INSERT INTO audit_log (tenant_id, actor_id, actor_kind, actor_ip, action, subject, at, delta)
SELECT tenant_id, 'system', 'system', '', 'luu_phan_quyen_vai_tro', vai_tro_id, now(),
       jsonb_build_object(
           'truoc', to_jsonb(held_before),
           'sau',   to_jsonb(ARRAY(SELECT x FROM unnest(held_before || added) AS x ORDER BY x)),
           'them',  to_jsonb(added),
           'bo',    '[]'::jsonb,
           'ly_do', 'migration 0034: quyền danh bạ người dân mặc định theo prototype (chủ dự án 16/10/2026)')
FROM per_role;

-- Measured per commune (tenant ids and counts only — nothing personal).
DO $$
DECLARE r record;
BEGIN
    FOR r IN
        SELECT tenant_id, count(*) AS n
        FROM audit_log
        WHERE action = 'luu_phan_quyen_vai_tro' AND actor_kind = 'system'
          AND delta->>'ly_do' = 'migration 0034: quyền danh bạ người dân mặc định theo prototype (chủ dự án 16/10/2026)'
        GROUP BY tenant_id
        ORDER BY tenant_id
    LOOP
        RAISE NOTICE '0034: commune % — % role(s) gained citizen directory keys', r.tenant_id, r.n;
    END LOOP;
END $$;

-- ---------------------------------------------------------------------------
-- REVERSAL (rule 7, invariant 4). Run by a person; core/migrate has no automatic rollback (ADR 0013).
-- Prose, not runnable lines, because a runnable line is a line that gets run.
--
-- 1. THE GRANTS. Every grant this file made is listed, per commune and per role, by
--      SELECT tenant_id, subject AS role_id, delta->'them' AS keys FROM audit_log
--      WHERE action = 'luu_phan_quyen_vai_tro' AND actor_kind = 'system'
--        AND delta->>'ly_do' LIKE 'migration 0034:%';
--    Withdraw them through the Phân quyền screen (PUT /api/v1/roles/{id}/permissions), commune by
--    commune, so each withdrawal writes its own audit entry. Do not remove rows by hand: a key an
--    administrator has since granted on purpose cannot be told from this file's grant by the row
--    alone, and the entries above are never edited or deleted (rule 6).
-- 2. THE KEYS stay: 0028 owns them, and this file added none.
-- 3. Remove this file's progress row from `schema_migration` (ten =
--    '0034_citizen_directory_role_grants.sql'), and revert the matching template change
--    (domain.RoleTemplates), or the next template run grants what this file withdrew.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- BACKSTOP, carried forward from 0002–0033: a partitioned table with no partitions rejects every
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
                         'MODULUS 8 partition loop (ADR 0096) in the same migration that '
                         'declares PARTITION BY.';
    END IF;
END $$;
