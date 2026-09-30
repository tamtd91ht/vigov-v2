-- 0025 — "chủ trì" and "người thực hiện" become ONE role on a task: the monitoring officer is the
-- assignee, the lead unit is the assigned unit. Data backfill, reversible, audited; NO column dropped.
--
-- WHY THIS FILE EXISTS. User decision 30/09/2026, ADR 0065 NV5
-- (kb/10-decisions/0065-vong-doi-nhiem-vu-theo-kho-yeu-cau.md), following vigov-require e1d0204:
-- "chuyên viên theo dõi ≡ người thực hiện, cơ quan chủ trì tham mưu ≡ bộ phận thực hiện". 0006 made
-- them two disjoint pairs (0006:269-270, :287-288):
--
--   SURVIVOR (read and written from now on)    RETIRED (kept, never read again)
--   nguoi_thuc_hien_ma                          chuyen_vien_theo_doi_ma
--   bo_phan_id                                  co_quan_chu_tri_id
--
-- THE SURVIVORS ARE THE ASSIGNEE COLUMNS, not the monitor columns, because every behaviour that already
-- exists hangs off them: `Giao cho tôi` (index nhiem_vu_nguoi_thuc_hien, 0006:400), the work right of
-- ADR 0065 NV7 (domain.TaskWorkRightFor), the "department with nobody named" clock
-- (store/automation.go taskHoldStartColumn) and the assignment route of task.assign.
--
-- ---------------------------------------------------------------------------
-- THE RULE, PER PAIR, PER ROW — deterministic, and stated because ADR 0065 open question #3 was closed
-- ("không phải hỏi") only on the ground that no real data exists, not by choosing a winner:
--
--   survivor empty, retired set          → survivor := retired. Audited ('gop_vai_tro_nhiem_vu').
--   survivor set, retired empty or equal → nothing to do. No entry.
--   survivor set, retired set, DIFFERENT → NOTHING IS WRITTEN TO THE ROW. The survivor (the assignee)
--                                          is kept, because choosing the other value would be a
--                                          REASSIGNMENT — a task.assign act with its own timeline
--                                          row and status reset — not a data merge. The retired value
--                                          stays in its column (never dropped) and is ALSO recorded in
--                                          an audit entry ('gop_vai_tro_nhiem_vu_lech'), so the person
--                                          who was monitoring is named in the trail of the act that
--                                          stopped reading them.
--   both empty                           → nothing. "Chưa phân công" stays a real state.
--
-- "Empty" is NULL OR a blank string: the store reads these columns through NULLIF(…, '')
-- (store/automation.go), so '' already means "nobody" to every reader.
--
-- WHAT THE MERGE DELIBERATELY DOES NOT DO:
--   * It does not reset `trang_thai` to `moi-giao`. domain.ResetStatus does that on a HAND-OVER; here
--     nobody is handed anything new — the person was already on the task, under the other label.
--   * It writes no timeline row (`nhat_ky_nhiem_vu`). The timeline records business acts by staff;
--     this is a schema decision, and the audit trail is where system acts go (rule 6, invariant 6).
--   * It notifies nobody. ADR 0065 open question #4 was closed on the same no-real-data ground; the
--     task register has no staff notification mechanism at all (app/task_assignment.go header).
--   * It does not drop, retype or empty the retired columns (rule 7, forbidden #3). They are marked
--     below with COMMENT ON COLUMN.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS, answered before the SQL.
--
--   1. HOW MANY ROWS PER COMMUNE: only the rows whose survivor is empty and whose retired half is set —
--      at most two column writes per task, SOFT-DELETED TASKS INCLUDED (a soft-deleted task is still an
--      archival record and must read the same way as a live one if it is ever restored or inspected).
--      The user confirmed 30/09/2026 that no environment holds real data, so the expected count is
--      zero everywhere; the NOTICE lines MEASURE it, per commune (counts and tenant ids only). The
--      operator may check first:
--        SELECT tenant_id,
--               count(*) FILTER (WHERE NULLIF(btrim(nguoi_thuc_hien_ma), '') IS NULL
--                                  AND NULLIF(btrim(chuyen_vien_theo_doi_ma), '') IS NOT NULL),
--               count(*) FILTER (WHERE NULLIF(btrim(bo_phan_id), '') IS NULL
--                                  AND NULLIF(btrim(co_quan_chu_tri_id), '') IS NOT NULL)
--        FROM nhiem_vu GROUP BY tenant_id;
--      PER COMMUNE AND RESUMABLE (rule 7, invariant 5): ONE STATEMENT PER PAIR, NOT A PER-COMMUNE LOOP,
--      for the reason 0015 and 0017 give (service-identity/migrations/0003_nguoi_dung_co_tai_khoan.sql:
--      100-107): core/migrate gives the file one transaction, so a loop inside it could not be resumed
--      either, and the genuinely resumable per-commune mechanism does not exist in core/migrate
--      (core/migrate/migrate.go:17-24). Every row carries its own tenant_id into its audit entry.
--   2. IF IT STOPS HALF-WAY: it cannot land half-applied — one file, one transaction, with the
--      progress row. A failure rolls back both backfills, every audit entry and the comments together.
--      A retry costs nothing: a filled row has a non-empty survivor and is never matched again; the
--      mismatch entries are guarded by NOT EXISTS, so they are never written twice.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom. The audit entries ARE the record of which row
--      and which half was filled, with the value it held before — so the reversal is exact, and skips
--      any row a person has reassigned since.
--   4. WHICH READ PATHS CHANGE MEANING WHILE IT IS HALF-APPLIED: none can see it half-applied (one
--      transaction). AFTER IT, for every filled row (ADR 0065 §Cái giá NV5):
--        * `Giao cho tôi` lists the task for the former monitoring officer;
--        * that officer gains the full work right (NV7) instead of "log only";
--        * the "department with nobody named" clock stops for the task;
--        * the department filter and `Liên quan đến tôi` (bo_phan_id) include the task for the former
--          lead unit — which `Liên quan đến tôi` already did through co_quan_chu_tri_id.
--      BETWEEN THIS FILE AND THE GO CHANGE: the migration is embedded, so it ships with the change that
--      stops the service reading the retired columns. During a rolling deploy an OLD replica can still
--      create or reassign a task with a monitor/lead unit and no assignee/unit; such a row is NOT
--      merged by this file (it runs once). Reported, not hidden: re-running the two backfill
--      statements by hand after the rollout merges them, with their audit entries.
--   5. RETENTION: `nhiem_vu` is an archival record (rule 7). No column is dropped, retyped or emptied;
--      no value is overwritten (only empty survivors are filled); `ma` is not touched (it is immutable
--      since 0024, which is why the audit subject — the task code — identifies the row for good).
--      `nhiem_vu_bat_bien` passes every UPDATE here: it touches neither `ma` nor `han_ban_dau`.
--
-- THE AUDIT ENTRY is written in the same statement as the change (a data-modifying CTE), so a row
-- cannot move without its entry (rule 6, invariant 3) — the shape 0017 uses. Actor 'system' / kind
-- 'system' (core/audit.SystemActor); subject = `ma`, the business code, never `id` (rule 6,
-- invariant 8). The delta holds staff business codes and unit ids only, like the assignment act's
-- (app/task_assignment.go assignmentAuditFields): no citizen personal data (rule 6, forbidden #4).

DO $$
DECLARE
    assignees_filled  int;
    units_filled      int;
    assignee_mismatch int;
    unit_mismatch     int;
    r                 record;
BEGIN
    -- (a) nguoi_thuc_hien_ma ← chuyen_vien_theo_doi_ma
    WITH target AS (
        SELECT tenant_id, id, nguoi_thuc_hien_ma AS old_value, chuyen_vien_theo_doi_ma AS new_value
        FROM nhiem_vu
        WHERE NULLIF(btrim(nguoi_thuc_hien_ma), '') IS NULL
          AND NULLIF(btrim(chuyen_vien_theo_doi_ma), '') IS NOT NULL
    ), changed AS (
        UPDATE nhiem_vu n
        SET nguoi_thuc_hien_ma = t.new_value, cap_nhat_luc = now()
        FROM target t
        WHERE n.tenant_id = t.tenant_id AND n.id = t.id
          AND NULLIF(btrim(n.nguoi_thuc_hien_ma), '') IS NULL
        RETURNING n.tenant_id, n.ma, t.old_value, t.new_value
    )
    INSERT INTO audit_log (tenant_id, actor_id, actor_kind, actor_ip, action, subject, at, delta)
    SELECT tenant_id, 'system', 'system', '', 'gop_vai_tro_nhiem_vu', ma, now(),
           jsonb_build_object(
               'cot_dien', 'nguoi_thuc_hien_ma',
               'tu_cot',   'chuyen_vien_theo_doi_ma',
               'truoc',    jsonb_build_object('nguoi_thuc_hien_ma', old_value),
               'sau',      jsonb_build_object('nguoi_thuc_hien_ma', new_value),
               'ly_do',    'migration 0025: monitoring officer and assignee are one role (ADR 0065 NV5)')
    FROM changed;
    GET DIAGNOSTICS assignees_filled = ROW_COUNT;

    -- (b) bo_phan_id ← co_quan_chu_tri_id
    WITH target AS (
        SELECT tenant_id, id, bo_phan_id AS old_value, co_quan_chu_tri_id AS new_value
        FROM nhiem_vu
        WHERE NULLIF(btrim(bo_phan_id), '') IS NULL
          AND NULLIF(btrim(co_quan_chu_tri_id), '') IS NOT NULL
    ), changed AS (
        UPDATE nhiem_vu n
        SET bo_phan_id = t.new_value, cap_nhat_luc = now()
        FROM target t
        WHERE n.tenant_id = t.tenant_id AND n.id = t.id
          AND NULLIF(btrim(n.bo_phan_id), '') IS NULL
        RETURNING n.tenant_id, n.ma, t.old_value, t.new_value
    )
    INSERT INTO audit_log (tenant_id, actor_id, actor_kind, actor_ip, action, subject, at, delta)
    SELECT tenant_id, 'system', 'system', '', 'gop_vai_tro_nhiem_vu', ma, now(),
           jsonb_build_object(
               'cot_dien', 'bo_phan_id',
               'tu_cot',   'co_quan_chu_tri_id',
               'truoc',    jsonb_build_object('bo_phan_id', old_value),
               'sau',      jsonb_build_object('bo_phan_id', new_value),
               'ly_do',    'migration 0025: lead unit and assigned unit are one role (ADR 0065 NV5)')
    FROM changed;
    GET DIAGNOSTICS units_filled = ROW_COUNT;

    -- (c) BOTH HALVES SET AND DIFFERENT: the row is not written; the retired value is recorded.
    INSERT INTO audit_log (tenant_id, actor_id, actor_kind, actor_ip, action, subject, at, delta)
    SELECT n.tenant_id, 'system', 'system', '', 'gop_vai_tro_nhiem_vu_lech', n.ma, now(),
           jsonb_build_object(
               'cot_giu',     'nguoi_thuc_hien_ma',
               'gia_tri_giu', n.nguoi_thuc_hien_ma,
               'cot_nghi',    'chuyen_vien_theo_doi_ma',
               'gia_tri_nghi', n.chuyen_vien_theo_doi_ma,
               'ly_do',       'migration 0025: assignee kept; the former monitoring officer is no longer read (ADR 0065 NV5)')
    FROM nhiem_vu n
    WHERE NULLIF(btrim(n.nguoi_thuc_hien_ma), '') IS NOT NULL
      AND NULLIF(btrim(n.chuyen_vien_theo_doi_ma), '') IS NOT NULL
      AND n.nguoi_thuc_hien_ma IS DISTINCT FROM n.chuyen_vien_theo_doi_ma
      AND NOT EXISTS (SELECT 1 FROM audit_log a
                      WHERE a.tenant_id = n.tenant_id AND a.subject = n.ma
                        AND a.action = 'gop_vai_tro_nhiem_vu_lech'
                        AND a.delta->>'cot_nghi' = 'chuyen_vien_theo_doi_ma');
    GET DIAGNOSTICS assignee_mismatch = ROW_COUNT;

    INSERT INTO audit_log (tenant_id, actor_id, actor_kind, actor_ip, action, subject, at, delta)
    SELECT n.tenant_id, 'system', 'system', '', 'gop_vai_tro_nhiem_vu_lech', n.ma, now(),
           jsonb_build_object(
               'cot_giu',     'bo_phan_id',
               'gia_tri_giu', n.bo_phan_id,
               'cot_nghi',    'co_quan_chu_tri_id',
               'gia_tri_nghi', n.co_quan_chu_tri_id,
               'ly_do',       'migration 0025: assigned unit kept; the former lead unit is no longer read (ADR 0065 NV5)')
    FROM nhiem_vu n
    WHERE NULLIF(btrim(n.bo_phan_id), '') IS NOT NULL
      AND NULLIF(btrim(n.co_quan_chu_tri_id), '') IS NOT NULL
      AND n.bo_phan_id IS DISTINCT FROM n.co_quan_chu_tri_id
      AND NOT EXISTS (SELECT 1 FROM audit_log a
                      WHERE a.tenant_id = n.tenant_id AND a.subject = n.ma
                        AND a.action = 'gop_vai_tro_nhiem_vu_lech'
                        AND a.delta->>'cot_nghi' = 'co_quan_chu_tri_id');
    GET DIAGNOSTICS unit_mismatch = ROW_COUNT;

    RAISE NOTICE '0025: % assignee(s) filled from the monitoring officer, % unit(s) filled from the lead unit',
        assignees_filled, units_filled;
    RAISE NOTICE '0025: % task(s) with a different monitoring officer, % with a different lead unit — row kept, retired value recorded in audit_log',
        assignee_mismatch, unit_mismatch;

    FOR r IN
        SELECT tenant_id, action, count(*) AS n
        FROM audit_log
        WHERE action IN ('gop_vai_tro_nhiem_vu', 'gop_vai_tro_nhiem_vu_lech') AND actor_kind = 'system'
        GROUP BY tenant_id, action
        ORDER BY tenant_id, action
    LOOP
        RAISE NOTICE '0025: commune % holds % % entr(ies)', r.tenant_id, r.n, r.action;
    END LOOP;
END $$;

-- ---------------------------------------------------------------------------
-- The retired columns, marked where a person reading the schema will see them.
-- ---------------------------------------------------------------------------
COMMENT ON COLUMN nhiem_vu.chuyen_vien_theo_doi_ma IS
    'DEPRECATED by migration 0025 (ADR 0065 NV5): the monitoring officer IS the assignee. Kept because '
    'rule 7 forbids dropping a populated column without a backup. Never read it; read nguoi_thuc_hien_ma.';

COMMENT ON COLUMN nhiem_vu.co_quan_chu_tri_id IS
    'DEPRECATED by migration 0025 (ADR 0065 NV5): the lead unit IS the assigned unit. Kept because '
    'rule 7 forbids dropping a populated column without a backup. Never read it; read bo_phan_id.';

-- ---------------------------------------------------------------------------
-- REVERSAL (migration question 3). Run by a person, in ONE transaction; core/migrate has no automatic
-- rollback (ADR 0013). It is itself a system act and writes its own audit entries (rule 6, invariant 6);
-- it never edits or deletes the entries above (rule 6, forbidden #3).
--
-- For each filled half, put back the value it held before — ONLY where the survivor still holds the
-- value this file wrote AND no assignment act has touched the task since (a later 'phan_cong_nhiem_vu'
-- entry means a person decided who holds it; undoing that is not a schema reversal):
--
--   WITH filled AS (
--       SELECT a.tenant_id, a.subject AS ma, a.at, a.delta->>'cot_dien' AS col,
--              a.delta->'truoc'->>(a.delta->>'cot_dien') AS old_value,
--              a.delta->'sau'->>(a.delta->>'cot_dien')   AS new_value
--       FROM audit_log a
--       WHERE a.action = 'gop_vai_tro_nhiem_vu' AND a.actor_kind = 'system'
--         AND NOT EXISTS (SELECT 1 FROM audit_log b
--                         WHERE b.tenant_id = a.tenant_id AND b.subject = a.subject
--                           AND b.action = 'phan_cong_nhiem_vu' AND b.at > a.at)
--   ), undone AS (
--       UPDATE nhiem_vu n
--       SET nguoi_thuc_hien_ma = CASE WHEN f.col = 'nguoi_thuc_hien_ma' THEN f.old_value ELSE n.nguoi_thuc_hien_ma END,
--           bo_phan_id         = CASE WHEN f.col = 'bo_phan_id'         THEN f.old_value ELSE n.bo_phan_id END,
--           cap_nhat_luc = now()
--       FROM filled f
--       WHERE n.tenant_id = f.tenant_id AND n.ma = f.ma
--         AND ((f.col = 'nguoi_thuc_hien_ma' AND n.nguoi_thuc_hien_ma = f.new_value)
--           OR (f.col = 'bo_phan_id'         AND n.bo_phan_id         = f.new_value))
--       RETURNING n.tenant_id, n.ma, f.col, f.old_value, f.new_value
--   )
--   INSERT INTO audit_log (tenant_id, actor_id, actor_kind, actor_ip, action, subject, at, delta)
--   SELECT tenant_id, 'system', 'system', '', 'hoan_gop_vai_tro_nhiem_vu', ma, now(),
--          jsonb_build_object('cot', col, 'truoc', new_value, 'sau', old_value,
--                             'ly_do', 'reversal of migration 0025')
--   FROM undone;
--
--   (A task carrying BOTH halves filled appears twice in `filled`; PostgreSQL applies one joined row
--   per target row per UPDATE, so run the statement twice — the second run finds the other half. A
--   third run changes nothing.)
--
-- Then COMMENT ON COLUMN … IS NULL for the two retired columns, and remove this file's row from
-- `schema_migration` in the same transaction, otherwise the runner still believes it has run. The
-- 'gop_vai_tro_nhiem_vu_lech' entries need no reversal: they recorded a value and wrote nothing.
--
-- REVERSING THE SCHEMA DOES NOT REVERSE THE DECISION: the Go code that reads one role must be rolled
-- back with it, or the service keeps reading only the survivor.
-- ---------------------------------------------------------------------------
