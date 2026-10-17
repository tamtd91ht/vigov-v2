-- 0035 — the permission key `admin.sms` "Cấu hình kênh SMS", and its pre-grant to every live role that
-- already holds `admin.zalo` "Cấu hình kênh Zalo".
--
-- OWNER DECISION 17/10/2026 (ADR 0099 #3): each commune configures SEVERAL eSMS accounts (ApiKey,
-- SecretKey, Brandname) in web-admin Cấu hình › Kênh SMS under a NEW key `admin.sms`; the migration
-- seeds it and grants it to exactly the roles that hold `admin.zalo` — the way 0032 granted
-- `admin.zalo` on 15/10. The reasoning is the same as 0032's "cấp sẵn": the people who configure the
-- commune's Zalo channel are the people who configure its SMS channel, and nobody should have to be
-- granted a tab by hand on the day it appears.
--
-- THIS FILE CHANGES NO ROUTE. No route checks `admin.sms` yet: the Kênh SMS routes (service-comms) land
-- with the Go card of ADR 0099. Until then the key grants nothing, and tools/check_quyen.py lists it in
-- its informational "seeded but named by no Go code" count (not a failure — the 0028 / 0032 precedent).
--
-- WHY A NEW FILE AND NOT AN EDIT TO 0032: core/migrate checksums every applied file at startup and
-- stops the service when one has changed (ErrChecksumLech).
--
-- NAMED IN ENGLISH (rule 12, ADR 0051). The group and label are DATA a person reads, in Vietnamese;
-- the group is 'QUẢN TRỊ' exactly as 0001 / 0032 spell it, so the matrix files the key under the same
-- band (http/quyen.go merges groups by the exact `nhom` string).
--
-- NEW COMMUNES: the administrator role (`quan-tri-he-thong`) is filled by store.CapMoiQuyen with EVERY
-- catalogue key, so it holds `admin.sms` automatically. The template roles (domain.RoleTemplates)
-- list keys BY NAME: `chu-tich-ubnd` and `chanh-van-phong` carry `admin.zalo`
-- (internal/domain/role_template.go:109, :140) and must gain `admin.sms` in the same commit, or a
-- commune seeded after this file differs from one seeded before it. That change belongs to
-- internal/domain — OWED BY THE SERVICE BUILDER, not written here.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
-- 1. HOW MANY ROWS PER COMMUNE. Catalogue: one new row in `quyen` (platform-wide, no tenant_id).
--    Grants: one `vai_tro_quyen` row per live role holding admin.zalo — in practice the administrator
--    role, Chủ tịch and Chánh Văn phòng where they exist, so ~3 rows per commune. Plus ONE audit_log
--    entry per role that gained the key. The operator may measure it first:
--      SELECT vq.tenant_id, count(*) FROM vai_tro_quyen vq
--      JOIN vai_tro vt ON vt.tenant_id = vq.tenant_id AND vt.id = vq.vai_tro_id
--      WHERE vt.deleted_at IS NULL AND vq.quyen_ma = 'admin.zalo'
--      GROUP BY vq.tenant_id;
--    PER COMMUNE AND RESUMABLE (rule 7, invariant 5): ONE STATEMENT, NOT A PER-COMMUNE LOOP, as in
--    0028 / 0032 / 0034 — core/migrate runs the file in one transaction with its progress row, so a
--    loop inside it could not be resumed any better. Acceptable because the change is PURELY ADDITIVE
--    AND IDEMPOTENT: it inserts grants, never rewrites or removes one, and a rerun inserts nothing.
--    Every grant row and every audit entry carries its own commune's tenant_id.
--
-- 2. IF IT STOPS HALF-WAY. It cannot land half-applied: one file, one transaction. A failure rolls back
--    the catalogue row, the grants and the entries together. A retry costs nothing: the catalogue seed
--    is ON CONFLICT DO UPDATE (same values), the grant is ON CONFLICT DO NOTHING, and the audit entry
--    is built from the grant's own RETURNING — a key already held yields neither a row nor an entry.
--
-- 3. HOW IT IS REVERSED. See REVERSAL at the bottom.
--
-- 4. WHICH READ PATHS CHANGE MEANING WHILE IT IS HALF-APPLIED. None can see it half-applied (one
--    transaction). AFTER IT: the Phân quyền matrix shows one more row in QUẢN TRỊ, ticked for the roles
--    granted here; store.Checker answers "yes" for `admin.sms` to those roles — no behaviour changes,
--    because no route asks yet. POST /api/v1/roles/defaults (#14, app/role_template.go): the caller
--    must hold every key a template grants; once the templates carry `admin.sms`, every administrator
--    role created before this file already holds it (granted here, since CapMoiQuyen gave it
--    admin.zalo through 0032), so no administrator is locked out of the template button.
--
-- 5. RETENTION. Nothing is removed, retyped, overwritten or renumbered; no issued business code is
--    touched; no existing catalogue row moves rank. Grants are configuration, not an archival record,
--    but each one is audited with the system principal (rule 6, invariant 6).
--
-- MERGED / DISSOLVED COMMUNES: identity does not hold a commune's active/inactive status, so this file
-- cannot exclude them. A merged commune's live roles that hold admin.zalo gain a key no route checks
-- yet. Reported, not hidden.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- 1. THE CATALOGUE ROW.
--
-- Rank 52: ranks used so far are 1-33 (0001), 34-35 (0007), 36 (0010), 37 (0022), 38-43 (0028), 44-51
-- (0032); migrations/danh_ba_mini_app_test.go TestQuyenThuTuKhongTrungGiuaCacMigration forbids a
-- shared rank. 52 places the key LAST in QUẢN TRỊ (after admin.user, 51), not right after admin.zalo
-- (47): ranks 48-51 are taken, and putting it next to Zalo would mean moving four more rows' ranks as
-- 0032 did. A display order is not worth a second rank shuffle; if the owner wants SMS beside Zalo,
-- it is one rank move in a later file.
-- ---------------------------------------------------------------------------
INSERT INTO quyen (ma, nhom, nhan, thu_tu) VALUES
    ('admin.sms', 'QUẢN TRỊ', 'Cấu hình kênh SMS', 52)
ON CONFLICT (ma) DO UPDATE SET
    nhom   = EXCLUDED.nhom,
    nhan   = EXCLUDED.nhan,
    thu_tu = EXCLUDED.thu_tu;

-- ---------------------------------------------------------------------------
-- 2. THE PRE-GRANT, with its audit entry IN THE SAME STATEMENT (data-modifying CTE, the shape of 0028 /
-- 0032 / 0034), so a grant cannot land without its entry (rule 6, invariant 3).
--
-- @cross-tenant: one-off system migration granting admin.sms to every live role, in every commune, that
-- already holds admin.zalo (owner decision 17/10/2026, ADR 0099 #3); each row keeps its own tenant_id
-- and each role gets its own entry in its own commune.
--
-- ROLE FILTER: live roles only (`vt.deleted_at IS NULL`). A soft-deleted role is a commune's decision
-- and is not touched ("DELETED MEANS DECIDED", app/role_template.go). `vai_tro_quyen` has no soft
-- delete: a row there IS a live grant — so a role from which an administrator WITHDREW admin.zalo
-- since 0032 holds no row and gains nothing here, which is the intended reading of "đang có".
--
-- THE ENTRY uses the verb the Phân quyền save writes (`luu_phan_quyen_vai_tro`) with the role id as
-- subject, and that save's delta shape — truoc / sau / them / bo — plus `ly_do`, exactly as 0032.
-- `truoc` is read in the same statement, so it is the set held before this grant. Keys only; nothing
-- personal. Actor 'system' / kind 'system' (rule 6, invariant 6; domain.SystemActor).
-- ---------------------------------------------------------------------------
WITH grant_rule (held, granted) AS (
    VALUES ('admin.zalo', 'admin.sms')
), target AS (
    SELECT DISTINCT vq.tenant_id, vq.vai_tro_id, r.granted
    FROM vai_tro_quyen vq
    JOIN vai_tro vt ON vt.tenant_id = vq.tenant_id AND vt.id = vq.vai_tro_id
    JOIN grant_rule r ON r.held = vq.quyen_ma
    WHERE vt.deleted_at IS NULL
), inserted AS (
    INSERT INTO vai_tro_quyen (tenant_id, vai_tro_id, quyen_ma, cap_luc, cap_boi)
    SELECT t.tenant_id, t.vai_tro_id, t.granted, now(), 'system'
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
           'ly_do', 'migration 0035: quyền cấu hình kênh SMS, cấp sẵn cho vai trò đang giữ admin.zalo (chủ dự án 17/10/2026, ADR 0099)')
FROM per_role;

-- Measured per commune (tenant ids and counts only — nothing personal).
DO $$
DECLARE r record;
BEGIN
    FOR r IN
        SELECT tenant_id, count(*) AS n
        FROM audit_log
        WHERE action = 'luu_phan_quyen_vai_tro' AND actor_kind = 'system'
          AND delta->>'ly_do' = 'migration 0035: quyền cấu hình kênh SMS, cấp sẵn cho vai trò đang giữ admin.zalo (chủ dự án 17/10/2026, ADR 0099)'
        GROUP BY tenant_id
        ORDER BY tenant_id
    LOOP
        RAISE NOTICE '0035: commune % — % role(s) gained admin.sms', r.tenant_id, r.n;
    END LOOP;
END $$;

-- ---------------------------------------------------------------------------
-- REVERSAL (rule 7, invariant 4). Run by a person; core/migrate has no automatic rollback (ADR 0013).
-- Prose, not runnable lines, because a runnable line is a line that gets run. FIRST revert the
-- follow-up commit that makes routes check `admin.sms` — otherwise removing the key locks every commune
-- out of the Kênh SMS tab (the route would ask for a key no role can hold, rule 5 invariant 3c).
--
-- 1. THE GRANTS. Every grant this file made is listed, per commune and per role, by
--      SELECT tenant_id, subject AS role_id, delta->'them' AS keys FROM audit_log
--      WHERE action = 'luu_phan_quyen_vai_tro' AND actor_kind = 'system'
--        AND delta->>'ly_do' LIKE 'migration 0035:%';
--    Withdraw them through the Phân quyền screen (PUT /api/v1/roles/{id}/permissions), commune by
--    commune, so each withdrawal writes its own audit entry. Do not remove rows by hand: a key an
--    administrator has since granted on purpose cannot be told from this file's grant by the row
--    alone, and the entries above are never edited or deleted (rule 6).
-- 2. THE KEY. Once no `vai_tro_quyen` row references it (FK to quyen.ma), the catalogue row
--    ma = 'admin.sms' may be removed by a person.
-- 3. Remove this file's progress row from `schema_migration` (ten = '0035_sms_channel_permission.sql')
--    in the same transaction as step 2, and revert the matching template change (domain.RoleTemplates),
--    or the next template run grants a key the catalogue no longer has (the FK refuses).
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- BACKSTOP, carried forward from 0002–0034: a partitioned table with no partitions rejects every
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
