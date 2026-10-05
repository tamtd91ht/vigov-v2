-- 0024 — retire every LIVE demo-only own-app setting: the `--demo` fixed identity is removed.
--
-- WHY THIS FILE EXISTS. Owner decision 05/10/2026 (ADR 0066 §Sửa đổi 05/10/2026, ADR 0070 §Sửa đổi):
-- a commune's own app always signs in for real, with accessToken + phoneToken exchanged under THAT
-- App ID's secret; the fixed demo identity — a citizen session with NO phone verification — is gone,
-- in citizen-app (d3f725f3) and in identity (the same change as this file). Identity no longer reads
-- demo_identity_enabled at all, so a live row whose ONLY purpose was the demo identity (no sealed
-- secret, demo on — 0021's "stage 2" row) can now sign nobody in. 0021 already says what such a row
-- is: "a row … that can sign nobody in — the operator retires it instead of keeping it". This file
-- is that retirement, done once for every commune, by the system, with a trail entry per row.
--
-- WHAT IS KEPT, ON PURPOSE (rule 7):
--   * The column demo_identity_enabled. Dropping a populated column is rule 7 stop condition #2, and
--     the history rows carrying `true` are the only answer to "which App ID opened sessions without a
--     verified phone, from when to when". It is marked retired below, with COMMENT ON COLUMN.
--   * The CHECK mini_app_secret_sealed_or_demo, UNCHANGED. Every version Go writes from now on carries
--     a sealed secret (store.ErrMiniAppSecretNoSecret refuses one without), so the check is always met
--     by the first half; changing it would be DDL on a populated table for no behaviour.
--   * Every retired version, every audit entry already written (bat_/tat_danh_tinh_demo_app_rieng,
--     danh_tinh_demo in session entries), and every citizen account, session and petition opened under
--     the demo identity. None of them is touched here.
--   * A live row that HAS a sealed secret and also demo on. It stays live: its secret is what signs
--     citizens in, and the demo flag on it is inert because nothing reads it. Retiring it would take a
--     working app off the air. Its next version (set or replace) writes demo off.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS, answered before the SQL.
--
--   1. HOW MANY ROWS PER COMMUNE: the live rows with app_secret_sealed IS NULL AND
--      demo_identity_enabled — at most one per (commune, App ID) by UNIQUE (tenant_id, live_app_id);
--      in practice zero or one per commune that had a dedicated app awaiting Zalo review. The NOTICE
--      lines measure it per commune (tenant ids and counts only). The operator may check first:
--        SELECT tenant_id, count(*) FROM mini_app_secret
--        WHERE deleted_at IS NULL AND app_secret_sealed IS NULL AND demo_identity_enabled
--        GROUP BY tenant_id;
--      PER COMMUNE AND RESUMABLE (rule 7, invariant 5): ONE STATEMENT, NOT A PER-COMMUNE LOOP, for the
--      reason 0003 gives (0003_nguoi_dung_co_tai_khoan.sql) and service-petitions 0025 repeats:
--      core/migrate gives the file one transaction, so a loop inside it could not be resumed either.
--      Every row carries its own tenant_id into its own audit entry.
--   2. IF IT STOPS HALF-WAY: it cannot land half-applied — one file, one transaction, with its
--      progress row. A failure rolls back every retirement and every entry together. A retry costs
--      nothing: a retired row has deleted_at set and is never matched again, so no row is retired
--      twice and no entry is written twice (IDEMPOTENT by its own filter).
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. WHICH READ PATHS CHANGE MEANING WHILE IT IS HALF-APPLIED: none can see it half-applied (one
--      transaction). AFTER IT: the own-app sign-in answers 422 app_not_ready for those App IDs — which
--      it already did, because the Go change shipped with this file refuses a live row with no secret
--      and no longer has a demo path. ListMiniAppSecretStatuses stops listing them; platform-admin
--      shows "chưa đặt" for the App ID, which is what is true.
--      DURING A ROLLING DEPLOY an OLD replica can still run `operatorctl mini-app-demo on` and write a
--      new demo-only row after this file ran. Such a row signs nobody in on a new replica (no secret,
--      no demo path). Reported, not hidden: re-running the DO block below by hand after the rollout
--      retires it, with its entry.
--   5. RETENTION: nothing is deleted, retyped or overwritten. A retirement is the ONE mutation
--      mini_app_secret_guard (0021) allows on a live row: the soft-delete trio plus updated_at. The
--      guard refuses any other column change, so this file cannot alter a version even by mistake.
--
-- THE AUDIT ENTRY is written in the same statement as the retirement (a data-modifying CTE), so a row
-- cannot be retired without its entry (rule 6, invariant 3) — the shape service-petitions 0017/0025
-- use. Actor 'system' / kind 'system' (rule 6, invariant 6; domain.SystemActor); action
-- 'ngung_cau_hinh_app_rieng', the SAME verb an operator's retire writes (app.ActionRetireOwnAppConfig),
-- so "every retirement of this App ID" is one query; subject = the App ID, as for that act. The delta
-- has that act's shape — app_id, ly_do, truoc, sau = null — and holds no secret (there is none on
-- these rows) and no personal data (rule 3).
-- ---------------------------------------------------------------------------

DO $$
DECLARE
    retired int;
    r       record;
BEGIN
    WITH target AS (
        SELECT tenant_id, id
        FROM mini_app_secret
        WHERE deleted_at IS NULL
          AND app_secret_sealed IS NULL
          AND demo_identity_enabled
    ), changed AS (
        UPDATE mini_app_secret m
        SET deleted_at    = now(),
            deleted_by    = 'system',
            delete_reason = 'gỡ danh tính demo theo quyết định 05/10/2026',
            updated_at    = now()
        FROM target t
        WHERE m.tenant_id = t.tenant_id AND m.id = t.id
          AND m.deleted_at IS NULL
        RETURNING m.tenant_id, m.id, m.app_id
    )
    INSERT INTO audit_log (tenant_id, actor_id, actor_kind, actor_ip, action, subject, at, delta)
    SELECT tenant_id, 'system', 'system', '', 'ngung_cau_hinh_app_rieng', app_id, now(),
           jsonb_build_object(
               'app_id', app_id,
               'ly_do',  'gỡ danh tính demo theo quyết định 05/10/2026',
               'truoc',  jsonb_build_object('phien_ban', id, 'co_secret', false, 'danh_tinh_demo', true),
               'sau',    NULL::jsonb)
    FROM changed;
    GET DIAGNOSTICS retired = ROW_COUNT;

    RAISE NOTICE '0024: % demo-only own-app setting(s) retired', retired;

    FOR r IN
        SELECT tenant_id, count(*) AS n
        FROM audit_log
        WHERE action = 'ngung_cau_hinh_app_rieng' AND actor_kind = 'system'
          AND delta->>'ly_do' = 'gỡ danh tính demo theo quyết định 05/10/2026'
        GROUP BY tenant_id
        ORDER BY tenant_id
    LOOP
        RAISE NOTICE '0024: commune % holds % demo retirement entr(ies)', r.tenant_id, r.n;
    END LOOP;
END $$;

-- ---------------------------------------------------------------------------
-- The retired column, marked where a person reading the schema will see it.
-- ---------------------------------------------------------------------------
COMMENT ON COLUMN mini_app_secret.demo_identity_enabled IS
    'RETIRED by migration 0024 (ADR 0066 Sua doi 05/10/2026): the demo fixed identity is removed. '
    'Kept because rule 7 forbids dropping a populated column; true on a row is history (who opened '
    'sessions without a verified phone, and when). New versions write false; nothing reads it.';

-- ---------------------------------------------------------------------------
-- REVERSAL (rule 7, invariant 4). Run by a person; core/migrate has no automatic rollback (ADR 0013).
--
-- A RETIRED VERSION CANNOT BE UN-RETIRED: mini_app_secret_guard (0021) refuses every change to a row
-- whose deleted_at is set, by design — it is history. Reversing this file therefore means making a
-- NEW live version with demo on for each App ID, which only the pre-05/10/2026 Go code can do
-- (`operatorctl mini-app-demo on`, written as a version row + its own audit entry). So:
--
--   1. Roll back the Go change that removed the demo identity — WITHOUT it, a demo version would be
--      inert, and the owner's decision would have to be reversed first (ADR 0066 §Sửa đổi).
--   2. For each App ID listed by
--        SELECT tenant_id, subject FROM audit_log
--        WHERE action = 'ngung_cau_hinh_app_rieng' AND actor_kind = 'system'
--          AND delta->>'ly_do' = 'gỡ danh tính demo theo quyết định 05/10/2026';
--      run `operatorctl mini-app-demo on --tenant <id> --app-id <subject> --ticket <n>`.
--   3. COMMENT ON COLUMN mini_app_secret.demo_identity_enabled with 0021's text, and remove this
--      file's row from `schema_migration` (ten = '0024_retire_demo_identity_settings.sql') in the same
--      transaction as step 3, otherwise the runner still believes it has run.
--
-- The entries above are never edited or deleted (rule 6, forbidden #3); a reversal writes its own.
-- Prose, not runnable lines, because a runnable line is a line that gets run.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- BACKSTOP, carried forward from 0002–0023: a partitioned table with no partitions rejects every
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
