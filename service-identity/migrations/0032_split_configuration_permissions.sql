-- 0032 — four configuration permission keys split out of `admin.lookup` / `admin.org`, and their
-- pre-grant to every live role that already reaches those screens through the old keys.
--
--   admin.residential_unit   Cấu hình thôn/tổ dân phố
--   admin.system_message     Cấu hình lời hệ thống
--   admin.mail_server        Cấu hình máy chủ thư
--   admin.zalo_channel       Cấu hình kênh Zalo
--
-- OWNER DECISION 15/10/2026 (tester bug sheet web-admin-141026, row 22): the four Cấu hình tabs get
-- their own keys instead of riding on `admin.lookup` (Quản lý danh mục) / `admin.org` (Quản lý sơ đồ
-- tổ chức). Same decision, second answer: "cấp sẵn" — nobody loses a screen they can open today.
-- Hence the grant rule, which mirrors what guards those screens TODAY:
--   a live role holding `admin.lookup`                 gets all four keys;
--   a live role holding `admin.org` (without lookup)   gets `admin.residential_unit` only
--   (the thôn/tổ dân phố screen is reached under both keys today — ADR 0059 §2, 0018).
--
-- THIS FILE CHANGES NO ROUTE. Every route still checks admin.lookup / admin.org until the follow-up
-- commit switches RequirePermission; until then the four keys grant nothing extra, and
-- tools/check_quyen.py lists them in its informational "seeded but named by no Go code" count (not a
-- failure — the 0028 precedent). The grant is written NOW so that the day the routes switch, the
-- people who use these tabs are already holding the key.
--
-- WHY A NEW FILE AND NOT AN EDIT TO 0001: core/migrate checksums every applied file at startup and
-- stops the service when one has changed (ErrChecksumLech).
--
-- NAMED IN ENGLISH (rule 12, ADR 0051). Group and key labels are DATA a person reads, in Vietnamese;
-- the group is 'QUẢN TRỊ' exactly as 0001:280-285 spells it, so the matrix files the keys under the
-- same band (http/quyen.go gomTheoNhom merges by the exact `nhom` string).
--
-- New communes: their administrator role (`quan-tri-he-thong`) is filled by store.CapMoiQuyen with
-- EVERY catalogue key, so it holds the four automatically. The template roles (domain.RoleTemplates,
-- ADR 0055/0095) list keys by name; `chu-tich-ubnd` and `chanh-van-phong` — the two that hold
-- admin.lookup — gain the four in the same commit, so a commune seeded after this file and one
-- seeded before end in the same state.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
-- 1. HOW MANY ROWS PER COMMUNE. Catalogue: four new rows in `quyen` (platform-wide, no tenant_id) and
--    the display rank of four existing rows. Grants: at most 4 x (live roles holding admin.lookup) +
--    1 x (live roles holding admin.org only) rows of `vai_tro_quyen` per commune — in practice the
--    administrator role, Chủ tịch and Chánh Văn phòng where they exist, so ~12 rows. Plus ONE
--    audit_log entry per role that gained a key. The operator may measure it first:
--      SELECT vq.tenant_id, vq.quyen_ma, count(*) FROM vai_tro_quyen vq
--      JOIN vai_tro vt ON vt.tenant_id = vq.tenant_id AND vt.id = vq.vai_tro_id
--      WHERE vt.deleted_at IS NULL AND vq.quyen_ma IN ('admin.lookup', 'admin.org')
--      GROUP BY vq.tenant_id, vq.quyen_ma;
--    PER COMMUNE AND RESUMABLE (rule 7, invariant 5): ONE STATEMENT, NOT A PER-COMMUNE LOOP, as in
--    0028 / 0031 — core/migrate runs the file in one transaction with its progress row, so a loop
--    inside it could not be resumed any better. It is acceptable here because the change is PURELY
--    ADDITIVE AND IDEMPOTENT: it inserts grants, never rewrites or removes one, and a rerun inserts
--    nothing. Every grant row and every audit entry carries its own commune's tenant_id.
--
-- 2. IF IT STOPS HALF-WAY. It cannot land half-applied: one file, one transaction. A failure rolls
--    back catalogue, ranks, grants and entries together. A retry costs nothing: the catalogue seed is
--    ON CONFLICT DO UPDATE (same values), the grant is ON CONFLICT DO NOTHING, and the audit entry is
--    built from the grant's own RETURNING — a key already held yields neither a row nor an entry.
--
-- 3. HOW IT IS REVERSED. See REVERSAL at the bottom.
--
-- 4. WHICH READ PATHS CHANGE MEANING WHILE IT IS HALF-APPLIED. None can see it half-applied (one
--    transaction). AFTER IT: the Phân quyền matrix shows four more rows in QUẢN TRỊ, right after
--    "Quản lý danh mục", ticked for the roles granted here. store.Checker answers "yes" for the four
--    keys to those roles — no behaviour changes, because no route asks yet.
--    POST /api/v1/roles/defaults (#14, app/role_template.go): the caller must hold every key a
--    template grants; the templates now grant the four, and every administrator role created before
--    this file holds admin.lookup (CapMoiQuyen), so it is granted the four here — no administrator is
--    locked out of the template button.
--
-- 5. RETENTION. Nothing is removed, retyped or renumbered; no issued business code is touched. The
--    only value overwritten is `quyen.thu_tu` of four catalogue rows — a display rank of the
--    platform's catalogue, not a record (see §1 below). Grants are configuration, not an archival
--    record, but each one is audited with the system principal (rule 6, invariant 6).
--
-- MERGED / DISSOLVED COMMUNES: identity does not hold a commune's active/inactive status, so this file
-- cannot exclude them. A merged commune's live roles that hold admin.lookup / admin.org gain the
-- corresponding new keys — exactly the screens they could already open. Reported, not hidden.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- 1. THE CATALOGUE, AND ITS ORDER.
--
-- The matrix orders by `thu_tu, ma` (store/quyen.go:85-93) and groups by `nhom` in first-seen order,
-- merging a group whose keys are not contiguous (http/quyen.go:136-160). `thu_tu` is INT with no
-- unique constraint; migrations/danh_ba_mini_app_test.go TestQuyenThuTuKhongTrungGiuaCacMigration
-- forbids two keys sharing a rank. Used so far: 1-33 (0001), 34-35 (0007), 36 (0010), 37 (0022),
-- 38-43 (0028); no other file inserts into quyen.
--
-- "RIGHT AFTER admin.lookup" (rank 2) cannot be had by inserting between 2 and 3. So the four new keys
-- take 44-47 and the four QUẢN TRỊ keys that followed admin.lookup — admin.org, admin.role,
-- admin.sla, admin.user (0001 ranks 3-6) — move to 48-51. Result inside QUẢN TRỊ: admin.audit (1),
-- admin.lookup (2), the four new keys (44-47), admin.org (48), admin.role (49), admin.sla (50),
-- admin.user (51). QUẢN TRỊ stays the first band (admin.audit, rank 1); no other group moves; ranks
-- 3-6 are left unused rather than reused, so no other row is touched. `thu_tu` is read ONLY by that
-- ORDER BY — no authorisation, no export, no template reads it.
-- ---------------------------------------------------------------------------
INSERT INTO quyen (ma, nhom, nhan, thu_tu) VALUES
    ('admin.residential_unit', 'QUẢN TRỊ', 'Cấu hình thôn/tổ dân phố', 44),
    ('admin.system_message',   'QUẢN TRỊ', 'Cấu hình lời hệ thống',    45),
    ('admin.mail_server',      'QUẢN TRỊ', 'Cấu hình máy chủ thư',     46),
    ('admin.zalo_channel',     'QUẢN TRỊ', 'Cấu hình kênh Zalo',       47)
ON CONFLICT (ma) DO UPDATE SET
    nhom   = EXCLUDED.nhom,
    nhan   = EXCLUDED.nhan,
    thu_tu = EXCLUDED.thu_tu;

-- The rank move. ONLY `thu_tu` is updated: the label and group of these four rows stay whatever 0001
-- wrote (the values below repeat 0001:282-285 only because the INSERT form needs them, and so the
-- rank-collision test above can read the new ranks). The rows always exist (0001), so the INSERT arm
-- never fires.
INSERT INTO quyen (ma, nhom, nhan, thu_tu) VALUES
    ('admin.org',  'QUẢN TRỊ', 'Quản lý sơ đồ tổ chức',   48),
    ('admin.role', 'QUẢN TRỊ', 'Phân quyền',              49),
    ('admin.sla',  'QUẢN TRỊ', 'Cấu hình thời hạn xử lý', 50),
    ('admin.user', 'QUẢN TRỊ', 'Quản lý người dùng',      51)
ON CONFLICT (ma) DO UPDATE SET
    thu_tu = EXCLUDED.thu_tu;

-- ---------------------------------------------------------------------------
-- 2. THE PRE-GRANT, with its audit entry IN THE SAME STATEMENT (data-modifying CTE, the shape of
-- 0024 / 0028 / 0031), so a grant cannot land without its entry (rule 6, invariant 3).
--
-- @cross-tenant: one-off system migration granting the four split-out keys to every live role, in
-- every commune, that already holds admin.lookup / admin.org (owner decision 15/10/2026, "cấp sẵn");
-- each row keeps its own tenant_id and each role gets its own entry in its own commune.
--
-- ROLE FILTER: live roles only (`vt.deleted_at IS NULL`). A soft-deleted role is a commune's decision
-- and is not touched — the rule app/role_template.go applies ("DELETED MEANS DECIDED"). `vai_tro_quyen`
-- has no soft delete (0001:142-150): a row there IS a live grant.
--
-- THE ENTRY uses the verb the Phân quyền save writes (`luu_phan_quyen_vai_tro`) with the role id as
-- subject, and that save's delta shape — truoc / sau / them / bo — plus `ly_do`, exactly as 0028.
-- `truoc` is read in the same statement, so it is the set held before this grant. Keys only; nothing
-- personal. Actor 'system' / kind 'system' (rule 6, invariant 6; domain.SystemActor).
-- ---------------------------------------------------------------------------
WITH grant_rule (held, granted) AS (
    VALUES ('admin.lookup', 'admin.residential_unit'),
           ('admin.lookup', 'admin.system_message'),
           ('admin.lookup', 'admin.mail_server'),
           ('admin.lookup', 'admin.zalo_channel'),
           ('admin.org',    'admin.residential_unit')
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
           'ly_do', 'migration 0032: tách quyền cấu hình khỏi admin.lookup/admin.org, cấp sẵn cho vai trò đang giữ (chủ dự án 15/10/2026)')
FROM per_role;

-- Measured per commune (tenant ids and counts only — nothing personal).
DO $$
DECLARE r record;
BEGIN
    FOR r IN
        SELECT tenant_id, count(*) AS n
        FROM audit_log
        WHERE action = 'luu_phan_quyen_vai_tro' AND actor_kind = 'system'
          AND delta->>'ly_do' = 'migration 0032: tách quyền cấu hình khỏi admin.lookup/admin.org, cấp sẵn cho vai trò đang giữ (chủ dự án 15/10/2026)'
        GROUP BY tenant_id
        ORDER BY tenant_id
    LOOP
        RAISE NOTICE '0032: commune % — % role(s) gained configuration keys', r.tenant_id, r.n;
    END LOOP;
END $$;

-- ---------------------------------------------------------------------------
-- REVERSAL (rule 7, invariant 4). Run by a person; core/migrate has no automatic rollback (ADR 0013).
-- Prose, not runnable lines, because a runnable line is a line that gets run. FIRST revert the
-- follow-up commit that makes routes check these keys — otherwise removing a key locks every commune
-- out of the four tabs (the route would ask for a key no role can hold, rule 5 invariant 3c).
--
-- 1. THE GRANTS. Every grant this file made is listed, per commune and per role, by
--      SELECT tenant_id, subject AS role_id, delta->'them' AS keys FROM audit_log
--      WHERE action = 'luu_phan_quyen_vai_tro' AND actor_kind = 'system'
--        AND delta->>'ly_do' LIKE 'migration 0032:%';
--    Withdraw them through the Phân quyền screen (PUT /api/v1/roles/{id}/permissions), commune by
--    commune, so each withdrawal writes its own audit entry. Do not remove rows by hand: a key an
--    administrator has since granted on purpose cannot be told from this file's grant by the row
--    alone, and the entries above are never edited or deleted (rule 6).
-- 2. THE KEYS. Once no `vai_tro_quyen` row references them (FK to quyen.ma), the four catalogue rows
--    may be removed by a person, ma IN the four keys above.
-- 3. THE RANKS. Put admin.org, admin.role, admin.sla, admin.user back to thu_tu 3, 4, 5, 6 (the
--    values of 0001:282-285), each row by its `ma`.
-- 4. Remove this file's progress row from `schema_migration` (ten =
--    '0032_split_configuration_permissions.sql') in the same transaction as steps 2-3, and revert the
--    template change (domain.RoleTemplates) shipped with this file, or the next template run grants
--    keys the catalogue no longer has (the FK refuses).
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- BACKSTOP, carried forward from 0002–0031: a partitioned table with no partitions rejects every
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
