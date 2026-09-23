-- finance — funding sources, the allocation of a project's plan across them, and the one column on
-- chung_tu_giai_ngan that says which source a payment was drawn from
-- (docs/ui-ux/06-giai-ngan.md §6, §11, §13 rule 6).
--
-- WHY A NEW FILE AND NOT AN EDIT OF 0004: 0001..0006 have been applied and core/migrate compares
-- the checksum of every applied file at startup. Editing an applied file either stops the service
-- (ErrChecksumLech) or leaves two databases claiming one schema version while holding two
-- different schemas. 0004:70-75 says in as many words that nguon_von was POSTPONED, not rejected,
-- and that `nguon_von_id` is the first column that would have to be added — this file is that
-- addition, written where 0004 said it would have to be written.
--
-- ---------------------------------------------------------------------------
-- MONEY. Every amount here is BIGINT, counted in ĐỒNG. The full reasoning is at the top of
-- 0004_du_an_va_chung_tu_giai_ngan.sql and is not repeated: not double precision, not real, not
-- float, not money. `tong_nguon` and `so_tien_phan_bo` are the denominators of the three progress
-- bars of §6, so a representation error here is a wrong percentage on a screen leadership reads.
--
-- ---------------------------------------------------------------------------
-- THE ONE RULE THIS FILE MUST NOT GET WRONG: `chung_tu_giai_ngan.nguon_von_id` IS NULLABLE.
--
-- §13 rule 6, verbatim: "Chứng từ chưa gắn nguồn vốn vẫn cộng vào tổng đã giải ngân, nhưng bị nêu ở
-- cảnh báo 'đã chi nhưng chưa ghi rút từ nguồn nào'." §6 prints that warning as a real figure on a
-- real commune ("Còn 3,4 tỷ đã chi nhưng chưa ghi rút từ nguồn nào").
--
-- So a voucher with no funding source is a STATE THE SPECIFICATION DEFINES AND A SCREEN REPORTS —
-- not a defect to be prevented at the column. NOT NULL here would refuse exactly the operation the
-- specification permits, and the commune would discover it as a 500 while trying to record a payment
-- that has already left the account. NULL means "not yet recorded against a source", and it is the
-- value the warning counts. There is also no DEFAULT: a default source would silently attribute
-- money to a source nobody chose, which is the same figure being wrong with nothing on the screen
-- saying so.
--
-- ---------------------------------------------------------------------------
-- TWO UNIQUE KEYS ARE DELIBERATELY NOT DECLARED HERE. READ THIS BEFORE ADDING ONE.
--
-- Both are STOP CONDITIONS under rule 7 and neither is answered by the specification. They are left
-- out rather than guessed, and leaving them out is free TODAY and only today: both tables are empty
-- in every commune, and this slice ships NO write path, so no row can exist to collide. The day a
-- write path exists, answering these becomes a migration over live catalogue data.
--
--   (a) nguon_von — is a source name unique within a commune, and within which scope?
--       The only shape consistent with the rest of the schema is (tenant_id, nam, ten): `nam` is on
--       the row, and §13 rule 8 makes each budget year its own set, so "Ngân sách xã, phường" must
--       be able to exist in 2026 AND in 2027 — which (tenant_id, ten) alone would forbid.
--       WHAT MAKES IT A QUESTION RATHER THAN A DEDUCTION: a unique key in this repository counts
--       SOFT-DELETED ROWS (see (b) of the note below), so declaring it means a source the commune
--       removes can never be re-added under the same name, in that year, ever. For an ISSUED CODE
--       that is exactly right (rule 7, invariant 3). For a name somebody typed and mistyped it is a
--       commune permanently stuck with "Ngân sách xã phường 2" — and `nguon_von` has no `ma` column
--       at all in §11, so nothing here is an issued code.
--
--   (b) phan_bo_nguon_von — may one project have TWO rows for the SAME source?
--       §11 defines the chip as "Đủ · N nguồn" over SUM(so_tien_phan_bo), and §6 counts "Số dự án"
--       per source. Both readings work with either answer, and they differ in what they COUNT: with
--       duplicates allowed, "3 nguồn" may be two sources listed twice, and no screen would show it.
--       The read path in internal/store therefore counts DISTINCT sources and projects, so a
--       duplicate cannot inflate a count while this is undecided — but that is a repair, not the
--       constraint, and the constraint is the customer's call.
--
-- NEITHER GAP IS SILENT: the PRIMARY KEY (tenant_id, id) exists on both tables, so nothing here is
-- unaddressable; what is missing is only the refusal of a duplicate, and the refusal is a business
-- rule nobody in this repository is entitled to write.
--
-- ---------------------------------------------------------------------------
-- WHY NO `UNIQUE (…) WHERE deleted_at IS NULL` ANYWHERE IN THIS FILE, stated because it is the
-- shape a reader will reach for first: a PARTIAL unique index counts only live rows, so soft
-- deleting a row and inserting the same key again succeeds — a code already issued, issued again
-- (rule 7, invariant 3). `tools/check_khoa_duy_nhat.py` refuses that shape across the whole
-- repository and would fail this file. The NON-unique indexes below do carry the predicate, and
-- that is a different thing entirely: they only decide which rows a read has to look at.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS, answered before the SQL.
--
--   1. HOW MANY ROWS PER COMMUNE: zero, in every commune, on all three objects. nguon_von and
--      phan_bo_nguon_von are new and this file seeds nothing (§6's four sample sources are
--      PROTOTYPE data, like §14, and are deliberately not sown). chung_tu_giai_ngan is empty
--      everywhere — 0004 seeded nothing and the write routes landed after it — so the ADD COLUMN
--      touches no row today. The ceiling in sight is §6's own figure: a handful of sources per
--      budget year, and at most a few per project.
--   2. IF IT STOPS HALF-WAY: it cannot land half-applied. One file, one transaction (ADR 0013,
--      decision 5), with the progress row written inside it. Every statement is IF NOT EXISTS,
--      CREATE OR REPLACE, or guarded by a check for its own object, so a retry after a failure
--      costs nothing.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom. It is complete today and stops being
--      complete the moment a voucher names a source.
--   4. WHICH READ PATHS CHANGE MEANING WHILE IT IS HALF-APPLIED: none, and this is the question
--      that usually gets skipped. The one figure every disbursement screen totals — `da_giai_ngan`
--      — is SUM(chung_tu_giai_ngan.so_tien) over live vouchers (0004:33-42), and NO statement here
--      touches so_tien, deleted_at, trang_thai or any predicate that sum depends on. The new column
--      is nullable with no default, so every existing row keeps satisfying every constraint and
--      every existing query returns exactly what it returned before. The two new tables have no
--      reader until this slice's store is deployed. The one behaviour that DOES change is a
--      refusal, not a result: see the trigger note below.
--   5. WHAT IT COSTS ON THE LARGEST COMMUNE: nothing measurable. ADD COLUMN with no DEFAULT does
--      not rewrite a table on any supported PostgreSQL; the two new tables are 64 empty partitions
--      and four indexes over no rows.
--
-- ---------------------------------------------------------------------------
-- WHAT THIS FILE DELIBERATELY DOES NOT CREATE, so the absence reads as a decision:
--
--   FOREIGN KEYS       phan_bo_nguon_von.du_an_id -> du_an(tenant_id, id),
--                      phan_bo_nguon_von.nguon_von_id -> nguon_von(tenant_id, id) and
--                      chung_tu_giai_ngan.nguon_von_id -> nguon_von(tenant_id, id) are LOGICAL
--                      references with no FK declared, following the precedent 0003 set and 0004
--                      restated (0004:182-190): a real FK between two HASH-partitioned tables is
--                      accepted by PostgreSQL 13, but no PostgreSQL is reachable from this
--                      repository's build environment, and a migration that fails at startup stops
--                      the service. THE COST IS STATED: nothing in the database stops a row from
--                      naming a source that does not exist. Every read path joins on
--                      (tenant_id, nguon_von_id) with $1 bound on BOTH sides, so such a row shows
--                      as having no source name — the same state as NULL — rather than leaking a
--                      source from another commune.
--   nam_ngan_sach      still not created, for the reason 0005:195-207 gives.
--   vuong_mac          still not created (0004:76-78).
--   the warning total  "đã chi nhưng chưa ghi rút từ nguồn nào" (§6, §13 rule 6) is a READ, and it
--                      is not in this slice. The column that makes it answerable is; the query is
--                      not, so that number cannot yet appear half-right anywhere.
-- ---------------------------------------------------------------------------

-- PostgreSQL 13 is the floor, for the same reason 0004 and 0005 state: BEFORE ... FOR EACH ROW
-- triggers on a PARTITIONED table were only allowed from 13, and both new tables below declare one.
DO $$ BEGIN
    IF current_setting('server_version_num')::int < 130000 THEN
        RAISE EXCEPTION
            'the disbursement register needs PostgreSQL 13 or newer (server is %). Do not weaken '
            'this migration to fit an older server — the triggers below are what stop an archival '
            'record from being destroyed or silently rewritten.',
            current_setting('server_version');
    END IF;
END $$;

-- ---------------------------------------------------------------------------
-- @entity: FundingSource
-- @scope:  tenant
--
-- nguon_von — one funding source of one commune, in one budget year (§6, §11).
--
-- PER BUDGET YEAR, because §11 puts `nam` on the row and §13 rule 8 makes each budget year its own
-- set of data. A single row carrying every year's `tong_nguon` would mean revising 2027's ceiling
-- silently rewrites the denominator of every 2026 progress bar that has already been reported.
--
-- `tong_nguon` IS THE CEILING OF THE SOURCE, NOT A BALANCE. §6 draws three bars against it —
-- allocated/total, disbursed/allocated, disbursed/total — and none of them writes back. Nothing
-- here decrements as money is spent, and nothing may be added that does: a stored remaining balance
-- is a second home for a number derived from two others, and the stale copy is the one that reaches
-- the report (the reasoning rule 10 gives for refusing an `is_overdue` column, applied to money).
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS nguon_von (
    tenant_id      TEXT        NOT NULL,
    id             TEXT        NOT NULL,
    -- "Ngân sách xã, phường". §11 gives this table NO `ma` column — a source is named, not coded —
    -- which is why rule 7, invariant 3 has nothing to bite on here and why the uniqueness of the
    -- name is an open question rather than a deduction (see the header).
    ten            TEXT        NOT NULL,
    nam            INT         NOT NULL,
    thu_tu         INT         NOT NULL DEFAULT 0,
    -- ĐỒNG. BIGINT, never floating point.
    tong_nguon     BIGINT      NOT NULL,
    deleted_at     TIMESTAMPTZ,
    deleted_by     TEXT,
    delete_reason  TEXT,
    tao_luc        TIMESTAMPTZ NOT NULL DEFAULT now(),
    cap_nhat_luc   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, id),
    -- A source of zero đồng is a real state: the commune knows the source exists before the figure
    -- is decided. A NEGATIVE total is not a state — it is a parsing accident or a sign error, and it
    -- would drag a commune's headline capital figure below the truth with no row looking wrong.
    CONSTRAINT nguon_von_tong_khong_am CHECK (tong_nguon >= 0),
    -- An unnamed source is a blank card on §6's screen that nobody can match to a decision. Blank
    -- and whitespace are checked together because a screen cannot tell them apart.
    CONSTRAINT nguon_von_ten_khong_rong CHECK (btrim(ten) <> ''),
    -- A budget year outside this window is a typo (1026, 20226) that would hide the source from
    -- every screen while it sits in the table looking healthy.
    CONSTRAINT nguon_von_nam_hop_le CHECK (nam BETWEEN 2000 AND 2100)
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS nguon_von_p%s PARTITION OF nguon_von '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

-- §6 lists one commune's sources for one budget year, in the commune's own display order.
CREATE INDEX IF NOT EXISTS nguon_von_danh_sach
    ON nguon_von (tenant_id, nam, thu_tu) WHERE deleted_at IS NULL;

DROP TRIGGER IF EXISTS nguon_von_cam_xoa_cung ON nguon_von;
CREATE TRIGGER nguon_von_cam_xoa_cung
    BEFORE DELETE ON nguon_von
    FOR EACH ROW EXECUTE FUNCTION ho_so_luu_tru_cam_xoa_cung();

-- ---------------------------------------------------------------------------
-- @entity: FundingAllocation
-- @scope:  tenant
--
-- phan_bo_nguon_von — how much of a project's year plan is drawn from which source (§9, §11).
--
-- A PLAN, NOT A PAYMENT, and the distinction is the whole reason this table is separate from
-- chung_tu_giai_ngan. Allocation says where the money is SUPPOSED to come from; a voucher says
-- where it actually came from. §6 shows both on one card and they are allowed to disagree — that
-- disagreement is the information the screen exists to show. A design that derived one from the
-- other would make the card unable to say anything.
--
-- ALLOCATING IS OPTIONAL. §9: "Chưa gắn nguồn nào. Xã theo dõi kế hoạch vốn theo hạng mục thì để
-- trống cũng được." A project with no row here is a normal project, not an incomplete one, and
-- §11 names that state ("Chưa gắn nguồn"). Nothing below requires a row to exist.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS phan_bo_nguon_von (
    tenant_id        TEXT        NOT NULL,
    id               TEXT        NOT NULL,
    du_an_id         TEXT        NOT NULL,
    nguon_von_id     TEXT        NOT NULL,
    -- ĐỒNG. BIGINT, never floating point.
    so_tien_phan_bo  BIGINT      NOT NULL,
    deleted_at       TIMESTAMPTZ,
    deleted_by       TEXT,
    delete_reason    TEXT,
    tao_luc          TIMESTAMPTZ NOT NULL DEFAULT now(),
    cap_nhat_luc     TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, id),
    -- Zero is admitted for the same reason du_an.ke_hoach_von_nam admits it (0004:227-229): a
    -- source attached before its figure is agreed is a real intermediate state a commune types.
    -- Negative is not a state; it would subtract from the "đã phân bổ" bar of §6 invisibly.
    CONSTRAINT phan_bo_nguon_von_so_tien_khong_am CHECK (so_tien_phan_bo >= 0),
    -- AN EMPTY STRING IS NOT "NO REFERENCE", IT IS A BROKEN ONE. Both columns are NOT NULL, so the
    -- only way to write a row pointing nowhere is '' — which joins to nothing and reads on §6 as a
    -- project silently missing from a source's count. Refused at the floor rather than trusted to
    -- every future write path.
    CONSTRAINT phan_bo_nguon_von_du_an_co_that CHECK (btrim(du_an_id) <> ''),
    CONSTRAINT phan_bo_nguon_von_nguon_co_that CHECK (btrim(nguon_von_id) <> '')
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS phan_bo_nguon_von_p%s PARTITION OF phan_bo_nguon_von '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

-- TWO READ SHAPES, both within one commune, and both are needed because §6 and §7 read this table
-- from opposite ends: the chip on a project row lists that project's sources, while a source card
-- totals every project drawing on it.
CREATE INDEX IF NOT EXISTS phan_bo_nguon_von_theo_du_an
    ON phan_bo_nguon_von (tenant_id, du_an_id) WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS phan_bo_nguon_von_theo_nguon
    ON phan_bo_nguon_von (tenant_id, nguon_von_id) WHERE deleted_at IS NULL;

DROP TRIGGER IF EXISTS phan_bo_nguon_von_cam_xoa_cung ON phan_bo_nguon_von;
CREATE TRIGGER phan_bo_nguon_von_cam_xoa_cung
    BEFORE DELETE ON phan_bo_nguon_von
    FOR EACH ROW EXECUTE FUNCTION ho_so_luu_tru_cam_xoa_cung();

-- ---------------------------------------------------------------------------
-- chung_tu_giai_ngan.nguon_von_id — which source this payment was drawn from.
--
-- NULLABLE, NO DEFAULT. The reasoning is at the top of this file and it is the reason the column
-- exists in this shape at all: §13 rule 6 makes "chi rồi nhưng chưa ghi nguồn" a state the system
-- must hold and report, not a state it must prevent.
-- ---------------------------------------------------------------------------
ALTER TABLE chung_tu_giai_ngan
    ADD COLUMN IF NOT EXISTS nguon_von_id TEXT;

DO $$ BEGIN
    -- '' IS NOT NULL, AND THAT DIFFERENCE IS THE WARNING FIGURE. The §6 note counts vouchers with
    -- no source; a write path that sent an empty string instead of NULL would drop those vouchers
    -- out of the warning while attaching them to no source either — money missing from both sides
    -- of the screen, with every row looking filled in. One constraint, at the floor, for a mistake
    -- that is otherwise invisible in every test that does not compare the two spellings.
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chung_tu_giai_ngan_nguon_von_khong_rong') THEN
        ALTER TABLE chung_tu_giai_ngan ADD CONSTRAINT chung_tu_giai_ngan_nguon_von_khong_rong
            CHECK (nguon_von_id IS NULL OR btrim(nguon_von_id) <> '');
    END IF;
END $$;

-- §6 totals disbursement per source, and the warning counts the vouchers with no source at all. A
-- btree index holds NULLs, so this one index serves both reads; it is partial on deleted_at for the
-- same reason every index in this service is (rule 7, invariant 2).
CREATE INDEX IF NOT EXISTS chung_tu_giai_ngan_theo_nguon_von
    ON chung_tu_giai_ngan (tenant_id, nguon_von_id) WHERE deleted_at IS NULL;

-- ---------------------------------------------------------------------------
-- chung_tu_da_khoa — REPLACED, to add `nguon_von_id` to the list of fields a LOCKED voucher
-- refuses to have changed.
--
-- THIS IS THE ONE THING IN THIS FILE THAT WOULD BE SILENT IF IT WERE FORGOTTEN, and 0004:137-139
-- wrote the instruction down in advance rather than leaving it to be discovered:
--
--	"A COLUMN ADDED LATER AND NOT ADDED HERE IS AN EDITABLE LOCKED FIELD. That is the known
--	 weakness of the allowlist-by-omission shape; it is written down rather than hidden, and
--	 nguon_von_id is the first column that will have to be added to this list when funding
--	 sources arrive."
--
-- Without this replacement, `nguon_von_id` would be the ONLY business fact of a locked voucher that
-- anybody could rewrite with a plain UPDATE — moving money between two funding sources, after the
-- figure was signed off and frozen, changing two cards on §6 and leaving the voucher itself looking
-- untouched.
--
-- WHY THIS FILE REPLACES THE FUNCTION WHILE 0005 DELIBERATELY REFUSED TO (0005:114-121): 0005 was
-- adding the unlock ledger, and its five columns are THE RECORD OF THE UNLOCK ITSELF — they must be
-- writable while the row is still locked, so touching the guard there would have been a rewrite
-- riding along with an unrelated change. Here the guard IS the subject: the column added is a
-- figure of the voucher, of exactly the kind the function already refuses, and adding it is the
-- instruction 0004 left.
--
-- The body below is 0004's, with ONE line added. Nothing else is altered: not the unlock branch,
-- not the soft-delete branch, not a message.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION chung_tu_da_khoa() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.trang_thai <> 'da-khoa' THEN
        RETURN NEW;
    END IF;

    IF NEW.du_an_id   IS DISTINCT FROM OLD.du_an_id
    OR NEW.ngay_chi   IS DISTINCT FROM OLD.ngay_chi
    OR NEW.so_tien    IS DISTINCT FROM OLD.so_tien
    OR NEW.noi_dung   IS DISTINCT FROM OLD.noi_dung
    OR NEW.doi_tac    IS DISTINCT FROM OLD.doi_tac
    OR NEW.so_chung_tu IS DISTINCT FROM OLD.so_chung_tu
    -- ADDED BY 0007. `IS DISTINCT FROM` and not `<>`: the column is nullable, and `<>` returns NULL
    -- rather than true whenever either side is NULL — which is precisely the interesting case here
    -- (attaching a source to a locked voucher that had none, or clearing one).
    OR NEW.nguon_von_id IS DISTINCT FROM OLD.nguon_von_id
    OR NEW.tep_dinh_kem IS DISTINCT FROM OLD.tep_dinh_kem THEN
        RAISE EXCEPTION 'chung_tu_giai_ngan: a locked voucher cannot be edited'
            USING HINT = 'Vòng đời: kế toán nhập -> đã xác nhận -> đã khoá (§8.2). Unlock it '
                         'first, in a statement of its own, with the budget.confirm permission — '
                         'so that the unlocking and the change are two separate audited events.';
    END IF;

    IF OLD.deleted_at IS NULL AND NEW.deleted_at IS NOT NULL THEN
        RAISE EXCEPTION 'chung_tu_giai_ngan: a locked voucher cannot be removed'
            USING HINT = 'Unlock it first (budget.confirm), then soft delete with a reason. A '
                         'locked voucher is a figure somebody has already signed off.';
    END IF;

    RETURN NEW;
END $$;

-- ---------------------------------------------------------------------------
-- MIGRATION QUESTION 4, THE HALF THAT IS NOT "none": the replaced trigger changes one BEHAVIOUR,
-- and it changes it for rows that already exist.
--
-- From the moment this file is applied, an UPDATE that changes `nguon_von_id` on a voucher in state
-- `da-khoa` is REFUSED where a moment earlier it would have succeeded. That is the intended effect
-- and it cannot break a caller today — no code anywhere writes the column yet, and no voucher holds
-- a value in it. It is written down because "a guard got stricter" is the kind of change that
-- surfaces months later as an unexplained failure, and the explanation should be findable here.
--
-- REVERSAL (migration question 3), in order:
--
--   1. CREATE OR REPLACE FUNCTION chung_tu_da_khoa() with the body from 0004:141-168 — that is,
--      this body minus the one `nguon_von_id` line. The function is not versioned by the runner, so
--      reversing it means restating it; 0004 is where the previous text lives.
--   2. DROP INDEX chung_tu_giai_ngan_theo_nguon_von; ALTER TABLE chung_tu_giai_ngan DROP CONSTRAINT
--      chung_tu_giai_ngan_nguon_von_khong_rong, DROP COLUMN nguon_von_id.
--   3. DROP TABLE phan_bo_nguon_von, then nguon_von. The 64 partitions, the four indexes and the
--      two triggers go with their parent tables. ho_so_luu_tru_cam_xoa_cung STAYS — it is 0004's
--      function and other tables are attached to it.
--   4. In the same transaction, remove this file's row from `schema_migration`, otherwise the
--      runner still believes the schema is in place.
--
-- THAT IS A COMPLETE REVERSAL ONLY WHILE nguon_von_id IS NULL ON EVERY VOUCHER AND BOTH TABLES ARE
-- EMPTY — which is true in every commune on the day this is written, and is why the file was
-- written now rather than after the first real voucher (0004's own note, and the reason the
-- cross-reference of 23/09/2026 argued for doing it early). Once a commune has recorded which
-- source a payment was drawn from, step 2 DESTROYS THAT RECORD: it is not a schema tidy-up, it is
-- rule 7's first stop condition, and it needs the user rather than a command. From that point the
-- way back is a NEW migration — core/migrate has no automatic rollback, for exactly this reason
-- (ADR 0013).
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- BACKSTOP: every partitioned table in this schema must actually have partitions.
--
-- A table declared PARTITION BY and given no partitions REJECTS EVERY INSERT, silently, until the
-- first real write — and because the audit entry shares the business transaction (rule 6,
-- invariant 3), that first write rolls back entirely. The check is repeated at the end of every
-- migration that declares a partitioned table, because it only verifies the state after a file
-- that CARRIES it.
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
