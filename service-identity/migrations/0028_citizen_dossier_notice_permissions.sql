-- 0028 — six permission keys for three modules that have no route yet, and their default grant to
-- the two leader roles and the administrator role of every existing commune.
--
--   DANH BẠ NGƯỜI DÂN   citizen.read   · citizen.update
--   HỒ SƠ CÔNG DÂN      dossier.import · dossier.read · dossier.update
--   GỬI THÔNG BÁO       notice.send
--
-- WHY THIS FILE EXISTS, AND WHY IT BREAKS THE PATTERN OF 0007 / 0010 / 0022. USER DECISION 09/10/2026,
-- explicit, taken after being shown the conflict: seed these keys NOW so the Phân quyền matrix matches
-- the prototype, although NO ROUTE CHECKS ANY OF THEM YET ("chờ module — người dùng chốt 09/10/2026
-- seed để ma trận khớp prototype"). It knowingly overrides ADR 0035 / open question #27 ("seed a key
-- only when a real route needs it"). The cost, stated rather than discovered: until the modules land,
-- six tick boxes on the matrix grant nothing, and tools/check_quyen.py reports them in its
-- informational "seeded but named by no Go code" count (not a failure — check_quyen.py:75-82).
--
-- AND, UNLIKE 0010 / 0022, THIS FILE GRANTS. Same decision: `chu-tich-ubnd` and `pho-chu-tich-ubnd`
-- of every existing commune get all six. `quan-tri-he-thong` gets them too (user decision 09/10/2026,
-- second answer): an administrator role created AFTER this file holds every catalogue key anyway
-- (CapMoiQuyen), and one created BEFORE it would otherwise lack them — which makes POST
-- /api/v1/roles/defaults refuse (#14, app/role_template.go:112-116: the caller must hold every key a
-- template grants) with no way out, since an administrator cannot edit its own role. Every other
-- role gets none. The roles are matched by their
-- stable CODE (`vai_tro.ma`, UNIQUE (tenant_id, ma)), never by the editable name. New communes get the
-- same grant through the template roles (domain.RoleTemplates, ADR 0055), changed in the same commit.
--
-- WHY A NEW FILE AND NOT AN EDIT TO AN EARLIER ONE: core/migrate checksums every applied file at
-- startup and stops the service when one has changed (ErrChecksumLech).
--
-- NAMED IN ENGLISH (rule 12, ADR 0051). Group labels and key labels are DATA a person reads, in
-- Vietnamese, upper-case like every group 0001 seeds (the matrix also upper-cases them in CSS).
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
-- 1. HOW MANY ROWS PER COMMUNE. Catalogue: six rows in `quyen`, platform-wide (no tenant_id, 0001).
--    Grants: at most 18 rows of `vai_tro_quyen` per commune (6 keys x 3 roles), and only in a commune
--    that HAS live roles with those codes — one that never pressed "Tạo vai trò mẫu" and never created
--    them gets nothing here. Plus ONE audit_log entry per role that actually gained a key. The NOTICE
--    lines measure it per commune (tenant ids and counts only). The operator may check first:
--      SELECT tenant_id, ma, count(*) FROM vai_tro
--      WHERE deleted_at IS NULL AND ma IN ('chu-tich-ubnd', 'pho-chu-tich-ubnd', 'quan-tri-he-thong')
--      GROUP BY tenant_id, ma;
--    PER COMMUNE AND RESUMABLE (rule 7, invariant 5): ONE STATEMENT, NOT A PER-COMMUNE LOOP, for the
--    reason 0003 and 0024 give — core/migrate gives the file one transaction, so a loop inside it
--    could not be resumed either. Every grant row and every audit entry carries its own tenant_id.
--
-- 2. IF IT STOPS HALF-WAY. It cannot land half-applied: one file, one transaction, with its progress
--    row. A failure rolls back the catalogue rows, every grant and every entry together. A retry
--    costs nothing: the catalogue seed is ON CONFLICT DO UPDATE (rewrites the same label), the grant
--    is ON CONFLICT DO NOTHING, and the audit entry is written only from the grant's own RETURNING —
--    so a key a role already holds produces neither a second row nor a second entry (IDEMPOTENT).
--
-- 3. HOW IT IS REVERSED. See REVERSAL at the bottom.
--
-- 4. WHICH READ PATHS CHANGE MEANING WHILE IT IS HALF-APPLIED. None can see it half-applied (one
--    transaction). AFTER IT: the Phân quyền matrix lists three more groups at its end; the two leader
--    columns show the six ticked. store.Checker answers "yes" for these keys to holders of those roles
--    — which changes no behaviour today, because no route asks. The day a module's route lands, the
--    leaders of every commune that had the roles at THIS moment can use it immediately; that is the
--    decision, stated here so the module's author is not surprised by it.
--    POST /api/v1/roles/defaults: the caller must hold every key any template grants (#14,
--    app/role_template.go:112-116). After this commit that set includes the six keys, which is why
--    the administrator role (`quan-tri-he-thong`) is granted them here as well.
--
-- 5. RETENTION. Nothing is removed, retyped, overwritten or renumbered; no issued code is touched.
--    Rule 7's stop conditions are not reached. Grants are configuration, not an archival record, but
--    each one is audited (rule 6, invariant 6) with the system principal.
--
-- MERGED / DISSOLVED COMMUNES: identity does not hold a commune's active/inactive status (that lives
-- outside this database), so this file cannot exclude them. A merged commune's leader roles, if they
-- exist and are live, gain six keys no route checks. Reported, not hidden.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- 1. THE CATALOGUE.
--
-- `thu_tu` 38-43. Measured: 0001 seeds 1-33, 0007 seeds 34-35, 0010 seeds 36, 0022 seeds 37, and no
-- other file inserts into quyen. The column has no unique constraint, so a collision would raise
-- nothing — migrations/danh_ba_mini_app_test.go TestQuyenThuTuKhongTrungGiuaCacMigration catches it.
-- Appended, not interleaved: putting the groups between existing ones would mean renumbering rows
-- earlier migrations own, and the matrix orders by thu_tu (store/quyen.go:85-93), so the three groups
-- sit after NHIỆM VỤ.
-- ---------------------------------------------------------------------------
INSERT INTO quyen (ma, nhom, nhan, thu_tu) VALUES
    ('citizen.read',    'DANH BẠ NGƯỜI DÂN', 'Xem danh bạ người dân',                    38),
    ('citizen.update',  'DANH BẠ NGƯỜI DÂN', 'Thêm, sửa, xoá người dân và nhóm',         39),
    ('dossier.import',  'HỒ SƠ CÔNG DÂN',    'Nhập hồ sơ công dân từ báo cáo Một cửa',   40),
    ('dossier.read',    'HỒ SƠ CÔNG DÂN',    'Xem hồ sơ công dân',                       41),
    ('dossier.update',  'HỒ SƠ CÔNG DÂN',    'Thêm, sửa, xoá hồ sơ công dân',            42),
    ('notice.send',     'GỬI THÔNG BÁO',     'Gửi thông báo tới người dân, cán bộ',      43)
ON CONFLICT (ma) DO UPDATE SET
    nhom   = EXCLUDED.nhom,
    nhan   = EXCLUDED.nhan,
    thu_tu = EXCLUDED.thu_tu;

-- ---------------------------------------------------------------------------
-- 2. THE DEFAULT GRANT, with its audit entry IN THE SAME STATEMENT (data-modifying CTE, the shape of
-- 0024), so a grant cannot land without its entry (rule 6, invariant 3).
--
-- @cross-tenant: one-off system migration granting the six keys to the leader roles of EVERY commune
-- and the administrator role (user decision 09/10/2026); each row keeps its own tenant_id and gets its own entry in that commune.
--
-- ROLE FILTER: live roles only (`deleted_at IS NULL`). A soft-deleted role is a commune's decision and
-- is not touched — the same rule app/role_template.go applies ("DELETED MEANS DECIDED").
--
-- THE ENTRY uses the verb the Phân quyền save writes (app.HanhViLuuPhanQuyenVaiTro,
-- `luu_phan_quyen_vai_tro`) with the role id as subject, so an inspection following one role reads
-- one continuous history. Its delta has that save's shape — truoc / sau / them / bo — plus `ly_do`.
-- `truoc` is read in the SAME statement: every part of a data-modifying WITH sees the snapshot taken
-- before the statement, so it is the set the role held before this grant. Keys only; nothing personal.
-- Actor 'system' / kind 'system' (rule 6, invariant 6; domain.SystemActor), as 0024.
-- ---------------------------------------------------------------------------
DO $$
DECLARE
    granted int;
    r       record;
BEGIN
    WITH new_key (ma) AS (
        VALUES ('citizen.read'), ('citizen.update'),
               ('dossier.import'), ('dossier.read'), ('dossier.update'),
               ('notice.send')
    ), target AS (
        SELECT vt.tenant_id, vt.id AS role_id
        FROM vai_tro vt
        WHERE vt.deleted_at IS NULL
          AND vt.ma IN ('chu-tich-ubnd', 'pho-chu-tich-ubnd', 'quan-tri-he-thong')
    ), inserted AS (
        INSERT INTO vai_tro_quyen (tenant_id, vai_tro_id, quyen_ma, cap_luc, cap_boi)
        SELECT t.tenant_id, t.role_id, k.ma, now(), 'system'
        FROM target t CROSS JOIN new_key k
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
               'ly_do', 'quyền mặc định theo quyết định người dùng 09/10/2026, ma trận khớp prototype')
    FROM per_role;
    GET DIAGNOSTICS granted = ROW_COUNT;

    RAISE NOTICE '0028: % leader/administrator role(s) gained the new keys', granted;

    FOR r IN
        SELECT tenant_id, count(*) AS n
        FROM audit_log
        WHERE action = 'luu_phan_quyen_vai_tro' AND actor_kind = 'system'
          AND delta->>'ly_do' = 'quyền mặc định theo quyết định người dùng 09/10/2026, ma trận khớp prototype'
        GROUP BY tenant_id
        ORDER BY tenant_id
    LOOP
        RAISE NOTICE '0028: commune % holds % default-grant entr(ies)', r.tenant_id, r.n;
    END LOOP;
END $$;

-- ---------------------------------------------------------------------------
-- REVERSAL (rule 7, invariant 4). Run by a person; core/migrate has no automatic rollback (ADR 0013).
-- Prose, not runnable lines, because a runnable line is a line that gets run.
--
-- 1. THE GRANTS. Every grant this file made is listed, per commune and per role, by
--      SELECT tenant_id, subject AS role_id, delta->'them' AS keys FROM audit_log
--      WHERE action = 'luu_phan_quyen_vai_tro' AND actor_kind = 'system'
--        AND delta->>'ly_do' = 'quyền mặc định theo quyết định người dùng 09/10/2026, ma trận khớp prototype';
--    Withdraw them through the Phân quyền screen (PUT /api/v1/roles/{id}/permissions), commune by
--    commune, so each withdrawal writes its own audit entry. Do not remove the rows by hand: a key an
--    administrator has since granted to the same role on purpose cannot be told apart from this
--    file's grant by the row alone, and the entries above are never edited or deleted (rule 6).
-- 2. THE KEYS. Once no `vai_tro_quyen` row references them (FK to quyen.ma), the six catalogue rows
--    may be removed by a person, ma IN the six keys above. While any role still holds one, removing it
--    would silently revoke a right an administrator granted — so this is not written as a line.
-- 3. Remove this file's progress row from `schema_migration` (ten =
--    '0028_citizen_dossier_notice_permissions.sql') in the same transaction as step 2, otherwise the
--    runner still believes it has run. And revert the template change (domain.RoleTemplates) shipped
--    with this file, or the next template run grants keys the catalogue no longer has (the FK refuses).
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- BACKSTOP, carried forward from 0002–0027: a partitioned table with no partitions rejects every
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
