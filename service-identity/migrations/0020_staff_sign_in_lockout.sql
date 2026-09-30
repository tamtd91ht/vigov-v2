-- 0020 — the AUTOMATIC sign-in lockout of a staff account: open question #39, DECIDED 2026-09-30
-- ("5 lần đăng nhập sai LIÊN TIẾP → khoá tự động 12 giờ ... quản trị viên xã mở khoá sớm được ...
-- mọi lần khoá/mở có vết. Khác và không thay khoá thủ công (lockout) đang có"). TCVN 14423 §5.5.2.2.
--
-- TWO COLUMNS, AND NEITHER IS `dang_hoat_dong`. That boolean is the MANUAL lock of #10 — a business
-- state ("retired", "transferred") set by an administrator and lifted only by one. The automatic lock
-- is a security state that EXPIRES BY ITSELF. Folded into one column, the first lockout that expired
-- would have to flip `dang_hoat_dong` back — and would then also re-open the account of somebody an
-- administrator had retired in the meantime. Two facts, two columns.
--
--   failed_sign_in_count   consecutive failed sign-ins since the last success or the last lock.
--                          Restarts at 0 on a success, on the failure that locks, and on an
--                          administrator's early unlock (domain.SignInLock).
--   sign_in_locked_until   the instant the automatic lock ends; NULL = never locked, or cleared.
--                          The lock is DERIVED — `sign_in_locked_until > now` — so there is no
--                          `is_locked` flag somebody must remember to clear, and nothing can drift
--                          (the same shape as operator_account.locked_until, migration 0012).
--
-- Names are English (rule 12, ADR 0051); the table keeps its existing name.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. ROWS PER COMMUNE: none are rewritten. `ADD COLUMN … DEFAULT 0` with a constant default is a
--      catalogue change since PostgreSQL 11 (the cluster is 16), and a nullable column with no
--      default is one too. Every existing account starts at "no failures, not locked" — the true
--      state, since nothing counted failures before this file.
--   2. IF IT STOPS HALF-WAY: it cannot — core/migrate runs the file in ONE transaction with its
--      progress row; every statement is IF NOT EXISTS / idempotent, so a retry starts clean.
--   3. HOW IT IS REVERSED: see REVERSAL below.
--   4. READ PATHS THAT CHANGE MEANING WHILE HALF-APPLIED: none. The only readers are the sign-in
--      use case and the administrator's unlock, both shipped with this file.
--   5. RETENTION: nothing is removed. The history of every lock and unlock is in audit_log, not
--      here; these two columns are the CURRENT state only.
--
-- TENANT SCOPE. `nguoi_dung` is already keyed (tenant_id, id) and partitioned by tenant (0001); a
-- column added to the parent is added to all 32 partitions. No new key, so no new composite key.
--
-- LOCKS. ACCESS EXCLUSIVE on the parent and its partitions for the catalogue change, released at
-- COMMIT; the CHECK scans every partition once — tens of rows per commune. Sign-in waits that long.
-- ---------------------------------------------------------------------------

ALTER TABLE nguoi_dung ADD COLUMN IF NOT EXISTS failed_sign_in_count INT NOT NULL DEFAULT 0;
ALTER TABLE nguoi_dung ADD COLUMN IF NOT EXISTS sign_in_locked_until TIMESTAMPTZ;

-- A negative count would let the next failure land short of the threshold — a sixth guess for free.
DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'nguoi_dung_failed_sign_in_count_range') THEN
        ALTER TABLE nguoi_dung ADD CONSTRAINT nguoi_dung_failed_sign_in_count_range
            CHECK (failed_sign_in_count >= 0);
    END IF;
END $$;

COMMENT ON COLUMN nguoi_dung.failed_sign_in_count IS
    'So lan dang nhap sai lien tiep (cau hoi mo #39). Ve 0 khi dang nhap dung, khi khoa tu dong, khi quan tri vien mo khoa.';
COMMENT ON COLUMN nguoi_dung.sign_in_locked_until IS
    'Khoa tu dong do dang nhap sai het hieu luc luc nay. KHAC dang_hoat_dong (khoa thu cong, #10). NULL = khong khoa.';

-- ---------------------------------------------------------------------------
-- REVERSAL (rule 7, invariant 4). By hand, in ONE transaction, after the Go code that reads the two
-- columns has been rolled back:
--
--   drop constraint nguoi_dung_failed_sign_in_count_range; drop column sign_in_locked_until; drop
--   column failed_sign_in_count; remove this file's row from schema_migration.
--
-- What is lost: only the CURRENT lock state — any account locked at that moment is unlocked early.
-- The record of every lock and unlock stays in audit_log. Written as prose, not a runnable line,
-- because a runnable line is a line that gets run.
-- ---------------------------------------------------------------------------
