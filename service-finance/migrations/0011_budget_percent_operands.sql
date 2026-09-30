-- 0011 — a percentage column of the budget board names its two operand COLUMNS by id, so the
-- server can compute the per-row % (docs/ui-ux/07-thu-chi-ngan-sach.md §9 rule 3: a % is never
-- stored, it is computed on render). User decision 30/09/2026: "máy chủ tự tính".
--
-- WHY A NEW FILE AND NOT AN EDIT OF 0006: 0001..0010 have been applied and core/migrate compares the
-- checksum of every applied file at startup (ErrChecksumLech). Same reason 0008 gives.
--
-- WHAT WAS THERE BEFORE: `cong_thuc` — free text like `col_4 / col_2 * 100` (0006:248-252) that
-- nothing evaluates, and whose `col_N` names a column by POSITION. A position is not an identity: it
-- moves when a column is inserted before it, and two columns may share `thu_tu` (0006:293-294 is an
-- index, not a unique key). An id cannot drift and cannot be ambiguous.
--
--   numerator_column_id     the `so` column on top of the fraction
--   denominator_column_id   the `so` column below it
--
-- Both are English (rule 12). TEXT, the type of `cot_ngan_sach.id`. `cong_thuc` IS KEPT, UNCHANGED,
-- and stays required on a % column by 0006's `cot_ngan_sach_cong_thuc_dung_kieu` — this file does not
-- touch that constraint. It is the commune's own wording of the formula and an archival fact (rule 7).
--
-- ---------------------------------------------------------------------------
-- WHAT THE DATABASE ENFORCES (four CHECKs below, all on one row):
--
--   * a `so` column has NO operands;
--   * a `phan_tram` column has BOTH or NEITHER. NEITHER = a legacy formula this file could not
--     resolve unambiguously (see BACKFILL). The read path must show such a column as "cannot be
--     computed" with a reason — never guess from `cong_thuc` (that guess is exactly what this file
--     refuses to make);
--   * numerator <> denominator, and neither is the % column itself;
--   * neither is a blank string — '' joins to no column and reads as "set" (0008's wording: "an empty
--     string is not no reference, it is a broken one").
--
-- WHAT THE DATABASE DOES NOT ENFORCE, and who does: that each operand is a LIVE `kieu = 'so'` column
-- OF THE SAME SHEET AND THE SAME COMMUNE. That is a cross-row fact. A CHECK cannot read another row,
-- and the only declarative form — a self-referencing composite FK `(tenant_id, bang_id,
-- numerator_column_id) -> (tenant_id, bang_id, id)` plus a pinned `kieu` — is a foreign key on a
-- HASH-partitioned table, the form 0003 declined and 0004/0006/0008 restated declining (no PostgreSQL
-- reachable from this build environment to verify it; a migration failing at startup stops the
-- service). THE COST, STATED: nothing in the database stops a writer naming a column of another sheet
-- or a percentage column as an operand. The domain layer (TASK-02) refuses both inside the write
-- transaction, with tenant_id bound on every lookup; this file's backfill satisfies them by
-- construction.
--
-- Columns are never edited after creation (domain.ErrTruongBangKhongSua,
-- internal/domain/thu_chi_ngan_sach.go:251-256). That is a DOMAIN rule: at the database level
-- `cot_ngan_sach` carries only the BEFORE DELETE trigger `cot_ngan_sach_cam_xoa_cung` (0006:296-299)
-- and no UPDATE trigger, so the backfill UPDATE below is not refused by anything. Verified by reading
-- every migration of this service, not by running one.
--
-- ---------------------------------------------------------------------------
-- BACKFILL — the rule, per % column, deterministic:
--
--   `cong_thuc` must be EXACTLY `col_A / col_B * 100`, whitespace-tolerant, case-sensitive, A and B
--   written without leading zeros, 1..999. Any other shape (`col_2/col_1*100.0`, `(col_1+col_2)/…`,
--   `COL_1 …`, `col_02 …`) → left NULL.
--   A = B → left NULL (it would violate the distinctness CHECK, and a column divided by itself is not
--   a ratio anybody meant).
--   `col_N` = the N-th LIVE `so` column of the same sheet, 1-based, ordered by `thu_tu` — the reading
--   the web starter set uses (web-admin/src/features/thu-chi/nhan-thu-chi.ts:811, :819).
--   AMBIGUOUS → left NULL. `col_N` resolves only when:
--     (a) exactly one `so` column sits at position N — a `thu_tu` shared by two `so` columns makes the
--         position ambiguous, and `rank()` + a tie count detects it; AND
--     (b) that column is ALSO the N-th among ALL live columns of the sheet, with no other column
--         sharing its `thu_tu`. This is STRICTER than "N-th number column" on purpose: `col_N`
--         could equally have been written meaning "the N-th column", and the two readings differ
--         exactly when a % column sits before an operand. Where they differ, one of them prints a
--         wrong ratio that looks like a right one — the failure the web already refuses to risk
--         (nhan-thu-chi.ts:737-740). Where they agree (every starter-set sheet), the fill is certain.
--   The % column itself must be live and its sheet row must exist (the audit subject is the sheet's
--   `ma`; a column whose sheet is missing is left NULL rather than filled without a trail).
--   SHEETS THAT WERE REMOVED (`bang_ngan_sach.deleted_at` set) ARE INCLUDED: a removed sheet is still
--   an archival record and should read the same way if it is ever inspected — 0025's reasoning in
--   service-petitions for soft-deleted tasks.
--
-- ONLY EMPTY PAIRS ARE FILLED (`numerator_column_id IS NULL AND denominator_column_id IS NULL`), so a
-- value a writer has already set is never overwritten, and a re-run changes nothing.
--
-- THE AUDIT ENTRY is written in the same statement as the change (a data-modifying CTE), so a row
-- cannot move without its entry (rule 6, invariant 3) — the shape service-petitions 0017/0025 use.
-- Actor 'system' / kind 'system' (rule 6, invariant 6); subject = the SHEET's `ma`, because a column
-- has no business code of its own — the same choice 0006 made for budget lines (rule 6, invariant 8).
-- The delta holds column ids and the formula text only; no personal data exists on this table.
-- Action code in Vietnamese without diacritics, like every finance action (ADR 0011;
-- internal/app/thu_chi_ngan_sach.go:117-122).
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. HOW MANY ROWS PER COMMUNE: at most one write per `phan_tram` column — a handful per sheet (the
--      starter set has one per sheet; domain.SoCotToiDa caps a sheet at 30 columns). The NOTICE lines
--      MEASURE it per commune (counts and tenant ids only). The operator may check first:
--        SELECT tenant_id, count(*) FROM cot_ngan_sach WHERE kieu = 'phan_tram' GROUP BY tenant_id;
--   2. IF IT STOPS HALF-WAY: it cannot land half-applied — one file, one transaction (core/migrate),
--      with the progress row. A failure rolls back the columns, the constraints, every fill and every
--      audit entry together.
--      PER COMMUNE AND RESUMABLE (rule 7, invariant 5): ONE STATEMENT, NOT A PER-COMMUNE LOOP — the
--      reason service-petitions 0025 gives (0025:62-66): core/migrate gives the file one transaction,
--      so a loop inside it could not be resumed either, and the per-commune resumable backfill
--      mechanism does not exist (core/migrate/migrate.go:16-26). What stands in for it: the statement
--      is IDEMPOTENT — only empty pairs are matched, so a retry or a manual re-run never touches a row
--      twice and never writes a second audit entry for the same fill. Every row carries its own
--      tenant_id into its audit entry.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. WHICH READ PATHS CHANGE MEANING WHILE IT IS HALF-APPLIED: none can see it half-applied (one
--      transaction). After it: NONE TODAY — the store reads columns through an explicit list
--      (internal/store/thu_chi_ngan_sach.go:170, `cotCot`) and inserts through one
--      (thu_chi_ngan_sach.go:537-539, `chenCot`), neither naming the new columns, so every existing
--      read returns exactly what it did. The meaning changes when TASK-02 starts reading them.
--      ROLLING DEPLOY: an old replica still creates % columns with both operands NULL — admitted by
--      the CHECKs as "unresolved". Such a column is not filled by this file (it runs once). Re-running
--      the BACKFILL block by hand after the rollout fills it, with its audit entry; running the whole
--      file again is also safe (every statement is guarded).
--   5. RETENTION: `cot_ngan_sach` describes a sheet that feeds a report sent upward — an archival
--      record (rule 7). Nothing is dropped, retyped or emptied; `cong_thuc` is not touched; no value
--      already present is overwritten (only NULL pairs are filled). No business code is issued or
--      renumbered.
-- ---------------------------------------------------------------------------

ALTER TABLE cot_ngan_sach ADD COLUMN IF NOT EXISTS numerator_column_id   TEXT;
ALTER TABLE cot_ngan_sach ADD COLUMN IF NOT EXISTS denominator_column_id TEXT;

-- Guarded by a lookup of their own name pinned to the PARENT table (`conrelid`), as 0008 does:
-- partitions inherit a CHECK under the same name, so `conname` alone matches 33 rows. The ADD
-- validates every existing row — all of which have both columns NULL at this point, which every one
-- of these four admits — and recurses into the 32 partitions.
DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint
                   WHERE conrelid = 'cot_ngan_sach'::regclass
                     AND conname = 'cot_ngan_sach_operands_only_on_percent') THEN
        ALTER TABLE cot_ngan_sach ADD CONSTRAINT cot_ngan_sach_operands_only_on_percent
            CHECK (kieu = 'phan_tram'
                   OR (numerator_column_id IS NULL AND denominator_column_id IS NULL));
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_constraint
                   WHERE conrelid = 'cot_ngan_sach'::regclass
                     AND conname = 'cot_ngan_sach_operands_both_or_neither') THEN
        ALTER TABLE cot_ngan_sach ADD CONSTRAINT cot_ngan_sach_operands_both_or_neither
            CHECK ((numerator_column_id IS NULL) = (denominator_column_id IS NULL));
    END IF;

    -- `<>` against NULL is NULL, which a CHECK treats as passed — so the NEITHER case is admitted
    -- here and constrained by the constraint above, not by accident.
    IF NOT EXISTS (SELECT 1 FROM pg_constraint
                   WHERE conrelid = 'cot_ngan_sach'::regclass
                     AND conname = 'cot_ngan_sach_operands_distinct') THEN
        ALTER TABLE cot_ngan_sach ADD CONSTRAINT cot_ngan_sach_operands_distinct
            CHECK (numerator_column_id <> denominator_column_id
                   AND numerator_column_id <> id
                   AND denominator_column_id <> id);
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_constraint
                   WHERE conrelid = 'cot_ngan_sach'::regclass
                     AND conname = 'cot_ngan_sach_operands_not_blank') THEN
        ALTER TABLE cot_ngan_sach ADD CONSTRAINT cot_ngan_sach_operands_not_blank
            CHECK (btrim(numerator_column_id) <> '' AND btrim(denominator_column_id) <> '');
    END IF;
END $$;

-- ---------------------------------------------------------------------------
-- BACKFILL. See the rule at the top. Re-runnable on its own.
-- ---------------------------------------------------------------------------
DO $$
DECLARE
    filled int;
    r      record;
BEGIN
    WITH all_columns AS (
        -- Position among ALL live columns of the sheet — reading (b) of the rule.
        SELECT tenant_id, bang_id, id, kieu, thu_tu,
               rank()   OVER (PARTITION BY tenant_id, bang_id ORDER BY thu_tu)         AS position_all,
               count(*) OVER (PARTITION BY tenant_id, bang_id, thu_tu)                 AS ties_all
        FROM cot_ngan_sach
        WHERE deleted_at IS NULL
    ), number_columns AS (
        -- Position among live `so` columns — reading (a), the one the web starter set writes.
        -- Window functions run after WHERE, so this ranks number columns only.
        SELECT tenant_id, bang_id, id, position_all, ties_all,
               rank()   OVER (PARTITION BY tenant_id, bang_id ORDER BY thu_tu)         AS position,
               count(*) OVER (PARTITION BY tenant_id, bang_id, thu_tu)                 AS ties
        FROM all_columns
        WHERE kieu = 'so'
    ), resolvable AS (
        -- A position that names exactly one column under BOTH readings. At most one row per
        -- (tenant, sheet, position): a unique rank with no tie is one row.
        SELECT tenant_id, bang_id, id, position
        FROM number_columns
        WHERE ties = 1 AND ties_all = 1 AND position = position_all
    ), parsed AS (
        SELECT c.tenant_id, c.bang_id, c.id, c.cong_thuc,
               regexp_match(c.cong_thuc,
                   '^\s*col_([1-9][0-9]{0,2})\s*/\s*col_([1-9][0-9]{0,2})\s*\*\s*100\s*$') AS m
        FROM cot_ngan_sach c
        WHERE c.kieu = 'phan_tram'
          AND c.deleted_at IS NULL
          AND c.numerator_column_id IS NULL
          AND c.denominator_column_id IS NULL
    ), target AS (
        SELECT p.tenant_id, p.id, p.cong_thuc, b.ma AS sheet_code,
               n.id AS numerator_id, d.id AS denominator_id
        FROM parsed p
        JOIN bang_ngan_sach b
          ON b.tenant_id = p.tenant_id AND b.id = p.bang_id
        JOIN resolvable n
          ON n.tenant_id = p.tenant_id AND n.bang_id = p.bang_id AND n.position = p.m[1]::int
        JOIN resolvable d
          ON d.tenant_id = p.tenant_id AND d.bang_id = p.bang_id AND d.position = p.m[2]::int
        WHERE p.m IS NOT NULL
          AND p.m[1] <> p.m[2]
    ), changed AS (
        UPDATE cot_ngan_sach c
        SET numerator_column_id   = t.numerator_id,
            denominator_column_id = t.denominator_id,
            cap_nhat_luc          = now()
        FROM target t
        WHERE c.tenant_id = t.tenant_id
          AND c.id = t.id
          AND c.kieu = 'phan_tram'
          AND c.numerator_column_id IS NULL
          AND c.denominator_column_id IS NULL
        RETURNING c.tenant_id, c.id, t.sheet_code, t.cong_thuc, t.numerator_id, t.denominator_id
    )
    INSERT INTO audit_log (tenant_id, actor_id, actor_kind, actor_ip, action, subject, at, delta)
    SELECT tenant_id, 'system', 'system', '', 'dien_toan_hang_cot_phan_tram', sheet_code, now(),
           jsonb_build_object(
               'cot_id',    id,
               'cong_thuc', cong_thuc,
               'truoc',     jsonb_build_object('numerator_column_id', NULL,
                                               'denominator_column_id', NULL),
               'sau',       jsonb_build_object('numerator_column_id', numerator_id,
                                               'denominator_column_id', denominator_id),
               'ly_do',     'migration 0011: operand columns resolved from the col_A / col_B * 100 formula')
    FROM changed;
    GET DIAGNOSTICS filled = ROW_COUNT;

    RAISE NOTICE '0011: % percentage column(s) given explicit operand columns', filled;

    -- What is left unresolved, per commune: counts and tenant ids only.
    FOR r IN
        SELECT tenant_id, count(*) AS n
        FROM cot_ngan_sach
        WHERE kieu = 'phan_tram' AND deleted_at IS NULL
          AND numerator_column_id IS NULL
        GROUP BY tenant_id
        ORDER BY tenant_id
    LOOP
        RAISE NOTICE '0011: commune % has % percentage column(s) left unresolved (formula ambiguous or of another shape)',
            r.tenant_id, r.n;
    END LOOP;
END $$;

-- ---------------------------------------------------------------------------
-- REVERSAL (migration question 3). Run by a person, in ONE transaction; core/migrate has no
-- automatic rollback (ADR 0013). It never edits or removes the audit entries above (rule 6,
-- forbidden #3) — they remain the record that the fill happened.
--
--   1. ALTER TABLE cot_ngan_sach DROP CONSTRAINT cot_ngan_sach_operands_only_on_percent,
--                                DROP CONSTRAINT cot_ngan_sach_operands_both_or_neither,
--                                DROP CONSTRAINT cot_ngan_sach_operands_distinct,
--                                DROP CONSTRAINT cot_ngan_sach_operands_not_blank;
--   2. Remove the two columns (ALTER TABLE … DROP COLUMN numerator_column_id, denominator_column_id).
--   3. Remove this file's row from `schema_migration`, otherwise the runner still believes it has run.
--
-- LOSSLESS ONLY WHILE EVERY FILLED PAIR CAME FROM THIS FILE: those values are derivable again from
-- `cong_thuc` (kept unchanged) and are recorded in audit_log. ONCE TASK-02 SHIPS and staff create %
-- columns by choosing operands, the pair is the ONLY statement of which columns were meant — `cong_thuc`
-- may then be the commune's free wording and not parseable. Step 2 from that point DROPS A POPULATED
-- COLUMN: rule 7 stop condition 2, which needs the user and a verified backup, not a command. The Go
-- code reading the columns must be rolled back with the schema.
-- ---------------------------------------------------------------------------
