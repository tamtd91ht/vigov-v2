-- 0022 — the permission key for revoking a staff member's SIGN-IN ACCOUNT while keeping the
-- directory row: `admin.user.revoke` (#27, ADR 0035 table "nạp khi dựng tuyến").
--
-- WHAT THE KEY GUARDS (user decision 2026-10-03, TASK-02b builds the route): revoke = remove the
-- password and revoke every session; the row stays in the directory as "directory only"; a reason
-- is mandatory and the act is audited. The account can be issued again later through the existing
-- "Cấp tài khoản" route (POST /api/v1/staff/{id}/account, `admin.user`).
--
-- SEEDED NOW because the route that checks it lands in the next card of the same run — ADR 0035's
-- rule is "seed a key only when a real route needs it", the same reason 0010 §4 seeded
-- `admin.user.delete` with TASK-04. This file copies 0010 §4's shape exactly.
--
-- A SEPARATE KEY FROM `admin.user` AND FROM `admin.user.delete`, on purpose (rule 5, invariant 3b —
-- rights are not a Cartesian product). Issuing an account, revoking one, and soft-deleting a
-- directory row are three different acts; revoking cuts a person off from the system immediately,
-- and a commune may well want it in fewer hands than account issuance.
--
-- WHY A NEW FILE AND NOT AN EDIT TO 0010: core/migrate checksums every applied file at startup and
-- stops the service when one has changed (ErrChecksumLech).
--
-- NAMED IN ENGLISH (rule 12, ADR 0051). The key string itself follows the existing
-- `<group>.<object>.<act>` shape of `admin.user.delete`.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
-- 1. HOW MANY ROWS PER COMMUNE. None. `quyen` is the platform-wide catalogue (0001, one row per
--    key, no tenant_id — it holds no commune's data); this file adds ONE catalogue row and writes
--    nothing into any commune's `vai_tro_quyen`.
--
-- 2. IF IT STOPS HALF-WAY. It cannot land half-applied: core/migrate runs the file in ONE
--    transaction together with its progress row. The seed is ON CONFLICT DO UPDATE, so a second
--    run is a no-op that rewrites the same label.
--
-- 3. HOW IT IS REVERSED. See REVERSAL at the bottom.
--
-- 4. WHICH READ PATHS CHANGE MEANING WHILE IT IS HALF-APPLIED. None. The Phân quyền screen lists
--    one more key under QUẢN TRỊ. No commune's role gains a right: the key is GRANTED TO NO ROLE
--    here (below), so until an administrator ticks it, the TASK-02b route answers 403 to everyone —
--    the intended closed-by-default state (rule 5).
--
-- 5. RETENTION. Nothing is removed, retyped or renumbered; no issued code is touched. Rule 7's
--    stop conditions are not reached by this file. The revoke ACT itself (TASK-02b) keeps the row,
--    which is why it needs no retention decision here.
--
-- PER-COMMUNE AND RESUMABLE (rule 7, invariant 5): NO BACKFILL and no per-commune row is written,
-- so there is nothing to resume and no commune to iterate.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- GRANT BEHAVIOUR — GRANTED TO NO ROLE, exactly as 0010 did for `admin.user.delete` and 0007 for
-- `feedback.classify` / `feedback.unmask`. Granting it here would be this migration deciding who in
-- a public authority may cut a colleague off from the system.
--
-- What that means per commune, stated rather than left to be discovered:
--   * `quan-tri-he-thong` created AFTER this file runs: holds the key by construction —
--     store.CapMoiQuyen grants every key in `quyen` at the first `admin` sign-in.
--   * `quan-tri-he-thong` created BEFORE this file runs: does NOT hold it (CapMoiQuyen runs once,
--     at creation). The administrator ticks it on the Phân quyền screen, which that role can reach
--     through `admin.role`. Identical to the state 0010 left `admin.user.delete` in.
--   * The eight template roles (domain.RoleTemplates): not granted. That list is written out by
--     name on purpose (role_template.go header); adding the key there is TASK-02b's / the user's
--     call, not a migration's.
--
-- `thu_tu` 37. Measured: 0001 seeds 1-33, 0007 seeds 34-35, 0010 seeds 36, and no other file
-- inserts into quyen. The column has no unique constraint, so a collision would raise nothing —
-- migrations/danh_ba_mini_app_test.go TestQuyenThuTuKhongTrungGiuaCacMigration catches it.
-- ---------------------------------------------------------------------------
INSERT INTO quyen (ma, nhom, nhan, thu_tu) VALUES
    ('admin.user.revoke', 'QUẢN TRỊ', 'Thu hồi tài khoản đăng nhập, giữ dòng danh bạ', 37)
ON CONFLICT (ma) DO UPDATE SET
    nhom   = EXCLUDED.nhom,
    nhan   = EXCLUDED.nhan,
    thu_tu = EXCLUDED.thu_tu;

-- ---------------------------------------------------------------------------
-- REVERSAL (rule 7, invariant 4).
--
-- core/migrate has no automatic rollback, on purpose: undoing a change on the permission catalogue
-- is an administrative act carried out with a person present. The reverse is written here, in prose.
--
-- THE KEY: an unused key is harmless and "reverting" it means ceasing to check it. A key already
-- granted to roles is referenced by vai_tro_quyen (FK to quyen.ma), and removing it would silently
-- revoke a right administrators granted. Not written as a runnable line.
--
-- AND AFTERWARDS, for the file to be applied again, its progress row has to be removed from
-- `schema_migration`, keyed on ten = '0022_revoke_sign_in_account_permission.sql'. Prose, not a
-- runnable line, for the same reason.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- BACKSTOP, carried forward from 0002–0021: a partitioned table with no partitions rejects every
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
