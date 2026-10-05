-- platform — a commune's own Mini App rows SWITCHED OFF before 05/10/2026 become SOFT-DELETED.
-- Data backfill, audited, reversible; no column, constraint or trigger is touched.
--
-- WHY THIS FILE EXISTS. ADR 0070 §"Sửa đổi 05/10/2026" #2: "Gỡ khỏi xã" and "Đổi App ID" now
-- soft-delete the old mini_app row (deleted_at / deleted_by / delete_reason), and reactivation is
-- withdrawn. Before that date both acts only set dang_hoat_dong = false — the Jenkins stage
-- `doi-app-id-thang-binh` (01/10/2026) and the console of ADR 0070 as first built. Those rows are
-- the legacy state the Go code still has to name (store/operator_writes.go ErrMiniAppInactive). The
-- owner chose, 05/10/2026, to convert them rather than leave two meanings of "removed" side by side.
--
-- THE SHAPE IS THE GO PATH'S, store/operator_writes.go softDeleteOwnMiniApp + RemoveMiniApp:
--   * row:   deleted_at = now(), deleted_by, delete_reason. dang_hoat_dong is already false.
--   * trail: action `tat_mini_app` (ActionDeactivateMiniApp), subject 'MiniApp <app_id>', delta
--            removalDelta(app_id, wasActive = false, reason) + `xa` (commune name), in audit_log
--            under the ROW'S commune, in the same statement as the change (rule 6, invariant 3).
--   TWO DELIBERATE DIFFERENCES:
--   * actor 'system' / kind 'system' (core/audit.SystemActor; rule 6, invariant 6) — a migration is
--     a system act, and no operator pressed anything. The Jenkins stage wrote its `tat_mini_app` with
--     kind 'system' as well, so both entries for the same App ID sit in the same lane. Consequence,
--     stated: like every 'system' entry, it shows on the commune's own audit screen (core/audit
--     Log.Read hides only kind 'operator') and NOT on the operator log (store/operator_log.go reads
--     kind 'operator' only).
--   * cap_nhat_luc / cap_nhat_boi are NOT touched. They keep naming the last person who changed the
--     row (whoever switched it off); this file's who/when is in deleted_by / deleted_at and in the
--     trail. It also makes the reversal exact: clearing the three deleted_* columns gives back the
--     row byte for byte.
--   `qua` in the delta names this file, the key the Jenkins stage used for its channel.
--
-- WHICH ROWS — every one of:
--   che_do = 'rieng'              a commune's own app. 'chinh' (the shared app) is NEVER touched:
--                                 its switch-off is a different act (store/shared_mini_app.go).
--   dang_hoat_dong = false        switched off. A running row is never removed by a migration.
--   deleted_at IS NULL            not already removed — this is what makes a re-run touch nothing.
--   the commune is ACTIVE and has NO tenant_succession row as predecessor. A merged or dissolved
--                                 commune's registry is kept unchanged (rule 7 invariant 6; rule 1
--                                 forbidden #7) — RemoveMiniApp refuses it the same way
--                                 (ErrCommuneInactive). Such rows are SKIPPED and COUNTED in a NOTICE
--                                 per commune; converting them is the user's decision, not this file's.
-- Known in prod when written: App ID 3291993990104489440 (Xã Thăng Bình, switched off 01/10/2026 by
-- the Jenkins stage), plus whatever the console switched off between 02/10 and 05/10/2026.
--
-- THE FIVE MIGRATION QUESTIONS:
--
--   1. HOW MANY ROWS PER COMMUNE: the commune's switched-off own apps — expected 0 or 1, a handful
--      across the platform. The NOTICE lines MEASURE it per commune (tenant id and count only). The
--      operator may check first:
--        SELECT m.tenant_id, count(*) FROM mini_app m
--        WHERE m.che_do = 'rieng' AND NOT m.dang_hoat_dong AND m.deleted_at IS NULL
--        GROUP BY m.tenant_id;
--      PER COMMUNE AND RESUMABLE (rule 7, invariant 5): ONE STATEMENT, NOT A PER-COMMUNE LOOP, for
--      the reason service-petitions 0025 gives: core/migrate gives a file one transaction, so a loop
--      inside it could not be resumed either, and the per-commune backfill mechanism does not exist
--      (core/migrate/migrate.go:16-26). At a handful of rows there is nothing to batch. Every row
--      carries its own tenant_id into its own audit entry.
--   2. IF IT STOPS HALF-WAY: it cannot land half-applied — one file, one transaction with the
--      runner's progress row. A failure rolls back every row and every entry together. A re-run (by
--      hand) is free: a converted row has deleted_at set and is never matched again, so it gets no
--      second entry. A concurrent "Gỡ khỏi xã" on the same row: the UPDATE re-checks
--      deleted_at IS NULL after the row lock, skips it, and RETURNING — hence the trail — omits it.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom. The rows are identified exactly by
--      deleted_by = 'system' AND delete_reason = this file's reason (the Go path always writes a VH-
--      code into deleted_by, so no operator removal can match), and the trail lists them again.
--   4. WHICH READ PATHS CHANGE MEANING: none can see it half-applied (one transaction). AFTER IT:
--        * Citizen sign-in (store/mini_app.go Directory.MiniApp): NO change — it already refused on
--          dang_hoat_dong = false; it now refuses on deleted_at too.
--        * Commune detail in the operator console (store/operator_registry.go Commune, deleted_at IS
--          NULL): the switched-off App IDs DISAPPEAR from the list of the commune's apps. That is the
--          point of #2 ("chỉ còn là lịch sử + nhật ký"). Their history is the audit trail.
--        * HeldMiniApp (retiring a secret left live under an App ID the commune no longer holds): NO
--          change — it reads soft-deleted rows on purpose. A live secret of such an App ID in
--          service-identity is NOT touched here (another service's database, rule 2); the console's
--          "khoá còn sống của App ID đã gỡ" lists it for the operator to retire.
--        * ReplaceMiniApp / RemoveMiniApp: the legacy "switched off, not removed" branch
--          (ErrMiniAppInactive; "its trail entry says it was already off") becomes unreachable for
--          these rows. Attaching one of these App IDs again answers ErrMiniAppRemoved instead of
--          ErrMiniAppTaken — the same refusal, now with its reason.
--        * Reactivation: already withdrawn; nothing reads dang_hoat_dong to bring a row back.
--   5. RETENTION: nothing is removed. The mini_app row stays (it decided the commune of earlier
--      citizen sessions — rule 7); its key keeps the App ID from ever being bound again (rule 7,
--      invariant 3). mini_app_cam_xoa_cung (0006) fires on DELETE only and is never reached: this
--      file runs no DELETE. mini_app_xoa_mem_day_du (0006) requires all three deleted_* together —
--      they are set together. No trigger is disabled. audit_log stays append-only (0002): INSERT only.
--
-- PERSONAL DATA (rule 3): none. An App ID and a commune name are not personal data; the NOTICE lines
-- carry tenant ids and counts only.

DO $$
DECLARE
    converted int;
    skipped   int;
    r         record;
BEGIN
    WITH params AS (
        -- The reason is written ONCE: it is both delete_reason and the trail's ly_do, and it is the
        -- key the reversal matches on. Changing it after this file is applied breaks the checksum.
        SELECT 'Chuyển sang xoá mềm theo quyết định chủ dự án 05/10/2026 (ADR 0070 §Sửa đổi)'::text AS reason
    ), target AS (
        SELECT m.app_id, m.tenant_id, t.ten AS commune_name
        FROM mini_app m
        JOIN tenant t ON t.id = m.tenant_id
        WHERE m.che_do = 'rieng'
          AND m.dang_hoat_dong = false
          AND m.deleted_at IS NULL
          AND t.dang_hoat_dong
          AND NOT EXISTS (SELECT 1 FROM tenant_succession s WHERE s.tu_id = m.tenant_id)
    ), changed AS (
        UPDATE mini_app m
        SET deleted_at = now(), deleted_by = 'system', delete_reason = p.reason
        FROM target x, params p
        WHERE m.app_id = x.app_id
          AND m.tenant_id = x.tenant_id
          AND m.che_do = 'rieng'
          AND m.dang_hoat_dong = false
          AND m.deleted_at IS NULL
        RETURNING m.tenant_id, m.app_id, m.delete_reason, x.commune_name
    )
    INSERT INTO audit_log (tenant_id, actor_id, actor_kind, actor_ip, action, subject, at, delta)
    SELECT c.tenant_id, 'system', 'system', '', 'tat_mini_app', 'MiniApp ' || c.app_id, now(),
           jsonb_build_object(
               'app_id', c.app_id,
               'truoc',  jsonb_build_object('dang_hoat_dong', false, 'da_xoa_mem', false),
               'sau',    jsonb_build_object('dang_hoat_dong', false, 'da_xoa_mem', true),
               'ly_do',  c.delete_reason,
               'xa',     c.commune_name,
               'qua',    'migration 0021_own_mini_app_switched_off_to_soft_delete.sql')
    FROM changed c;
    GET DIAGNOSTICS converted = ROW_COUNT;

    RAISE NOTICE '0021: % switched-off own Mini App row(s) soft-deleted, one tat_mini_app entry each', converted;

    FOR r IN
        SELECT tenant_id, count(*) AS n
        FROM audit_log
        WHERE action = 'tat_mini_app' AND actor_kind = 'system' AND actor_id = 'system'
          AND delta->>'qua' = 'migration 0021_own_mini_app_switched_off_to_soft_delete.sql'
        GROUP BY tenant_id
        ORDER BY tenant_id
    LOOP
        RAISE NOTICE '0021: commune % — % row(s) converted', r.tenant_id, r.n;
    END LOOP;

    -- Rows left alone because their commune is inactive or succeeded: measured, never written.
    SELECT count(*) INTO skipped
    FROM mini_app m
    JOIN tenant t ON t.id = m.tenant_id
    WHERE m.che_do = 'rieng' AND m.dang_hoat_dong = false AND m.deleted_at IS NULL
      AND (NOT t.dang_hoat_dong
           OR EXISTS (SELECT 1 FROM tenant_succession s WHERE s.tu_id = m.tenant_id));
    IF skipped > 0 THEN
        FOR r IN
            SELECT m.tenant_id, count(*) AS n
            FROM mini_app m
            JOIN tenant t ON t.id = m.tenant_id
            WHERE m.che_do = 'rieng' AND m.dang_hoat_dong = false AND m.deleted_at IS NULL
              AND (NOT t.dang_hoat_dong
                   OR EXISTS (SELECT 1 FROM tenant_succession s WHERE s.tu_id = m.tenant_id))
            GROUP BY m.tenant_id
            ORDER BY m.tenant_id
        LOOP
            RAISE NOTICE '0021: commune % is inactive or succeeded — % switched-off row(s) LEFT UNCHANGED (rule 7 invariant 6; the user decides)',
                r.tenant_id, r.n;
        END LOOP;
    END IF;
END $$;

-- ---------------------------------------------------------------------------
-- REVERSAL (rule 7, invariant 4). Run by a person, in ONE transaction; core/migrate has no automatic
-- rollback (core/migrate/migrate.go:42-44). It is itself a system act and writes its own audit entries (rule 6, invariant
-- 6); it never edits or deletes the entries above (rule 6, forbidden #3).
--
-- Reversing the SCHEMA does not reverse the DECISION: after it the rows are back in the legacy
-- "switched off, not removed" state, which ADR 0070 §Sửa đổi 05/10/2026 #2 no longer allows. The
-- user decides. Exact for exactly the rows this file touched — and only those: the Go path writes a
-- VH- code into deleted_by, so `deleted_by = 'system'` plus this file's reason matches no operator
-- removal:
--
--   WITH undone AS (
--       UPDATE mini_app m
--       SET deleted_at = NULL, deleted_by = NULL, delete_reason = NULL
--       WHERE m.che_do = 'rieng'
--         AND m.dang_hoat_dong = false
--         AND m.deleted_by = 'system'
--         AND m.delete_reason = 'Chuyển sang xoá mềm theo quyết định chủ dự án 05/10/2026 (ADR 0070 §Sửa đổi)'
--       RETURNING m.tenant_id, m.app_id
--   )
--   INSERT INTO audit_log (tenant_id, actor_id, actor_kind, actor_ip, action, subject, at, delta)
--   SELECT tenant_id, 'system', 'system', '', 'hoan_xoa_mem_mini_app', 'MiniApp ' || app_id, now(),
--          jsonb_build_object('app_id', app_id,
--                             'truoc', jsonb_build_object('dang_hoat_dong', false, 'da_xoa_mem', true),
--                             'sau',   jsonb_build_object('dang_hoat_dong', false, 'da_xoa_mem', false),
--                             'ly_do', 'reversal of migration 0021')
--   FROM undone;
--
-- Then remove this file's progress row from the schema_migration table, keyed on
-- ten = '0021_own_mini_app_switched_off_to_soft_delete.sql', in the same transaction — otherwise the
-- runner still believes it has run. Written as commented SQL rather than as runnable lines, because a
-- runnable line is a line that gets run.
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
