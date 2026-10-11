-- 0041 — accent-insensitive search on the task register and the petition register (customer bug sheet
-- 10/10, row 8): "nha van hoa" must find "Nhà văn hoá". Three STORED GENERATED columns; no row's
-- business data is written.
--
-- WHY GENERATED COLUMNS AND NOT `unaccent`: `unaccent` is an extension, and installing one is an
-- infrastructure decision (ADR 0010). The fold is `lower()` + `translate()` over an explicit Vietnamese
-- table — both IMMUTABLE — rendered by core/textfold.SQL. migrations/search_folded_columns_test.go
-- fails if the text below and that function ever disagree; the search term is folded in SQL by the
-- SAME function (store/nhiem_vu.go, store/xu_ly_phan_anh.go), so column and term cannot drift apart.
--
-- WHY A STORED COLUMN AND NOT AN EXPRESSION INDEX: the register search is `LIKE '%term%'`, which no
-- btree serves — an expression index would cost every write and help no read. The stored column saves
-- re-folding every row on every keystroke and gives the store one column name to read. The scan stays
-- bounded by one commune (`tenant_id = $1`, hash partition), as it was under ILIKE. A trigram index
-- would need `pg_trgm`, an extension: same decision as above, not taken here.
--
-- ⚠ TWO OF THE THREE COLUMNS ARE DERIVED FROM CITIZEN PERSONAL DATA (rule 3): `content_folded` from
-- `phieu_phan_anh.noi_dung`, `address_folded` from `dia_chi`. They are never selected (every read
-- names its columns; no `SELECT *` in internal/), never logged, never on an event. A GENERATED column
-- cannot be written, only recomputed — so anonymising the source (Decree 13, rule 7 invariant 7)
-- anonymises the fold in the same UPDATE, with nothing to forget.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. ROWS PER COMMUNE: no business value is written, but ADD COLUMN … STORED REWRITES both tables
--      (all 32 partitions each) under ACCESS EXCLUSIVE, computing the fold for every row. Reads and
--      writes of the two registers wait for the rewrite. Size today: hundreds to low thousands of rows
--      per commune — seconds. The NOTICE below gives the counts (tenant ids and counts only).
--   2. IF IT STOPS HALF-WAY: one file, one transaction (core/migrate); a rewrite either commits whole or
--      not at all. ADD COLUMN IF NOT EXISTS makes a retry cost nothing. No per-commune loop: nothing to
--      backfill — PostgreSQL computes the column.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom. Lossless: the three columns are derived, and the
--      columns they derive from are untouched.
--   4. READ PATHS THAT CHANGE MEANING: none while it runs. After it, the two register searches read the
--      folded columns — a SUPERSET of what ILIKE matched (folding never splits a match), so no row that
--      was found stops being found. Triggers on both tables compare named columns only (0004, 0006,
--      0011, 0015, 0024, 0032, 0037); none compares whole rows, so a generated column (NULL in NEW in a
--      BEFORE trigger) changes no guard's answer.
--   5. RETENTION: nothing deleted or edited. Both registers stay archival records (0004, 0006 guards).
-- ---------------------------------------------------------------------------

-- `tieu_de` is NOT NULL, so the fold is never NULL.
ALTER TABLE nhiem_vu ADD COLUMN IF NOT EXISTS title_folded TEXT GENERATED ALWAYS AS (
    lower(translate(translate(tieu_de, U&'\0300\0301\0302\0303\0306\0309\031B\0323', ''), 'àáảãạăằắẳẵặâầấẩẫậÀÁẢÃẠĂẰẮẲẴẶÂẦẤẨẪẬèéẻẽẹêềếểễệÈÉẺẼẸÊỀẾỂỄỆìíỉĩịÌÍỈĨỊòóỏõọôồốổỗộơờớởỡợÒÓỎÕỌÔỒỐỔỖỘƠỜỚỞỠỢùúủũụưừứửữựÙÚỦŨỤƯỪỨỬỮỰỳýỷỹỵỲÝỶỸỴđĐÐ', 'aaaaaaaaaaaaaaaaaAAAAAAAAAAAAAAAAAeeeeeeeeeeeEEEEEEEEEEEiiiiiIIIIIoooooooooooooooooOOOOOOOOOOOOOOOOOuuuuuuuuuuuUUUUUUUUUUUyyyyyYYYYYdDD'))
) STORED;

-- `noi_dung` is NOT NULL; `dia_chi` is NULLable, and so is its fold (`NULL LIKE …` matches nothing,
-- exactly as `COALESCE(dia_chi,'') ILIKE '%x%'` did for a non-empty term).
ALTER TABLE phieu_phan_anh ADD COLUMN IF NOT EXISTS content_folded TEXT GENERATED ALWAYS AS (
    lower(translate(translate(noi_dung, U&'\0300\0301\0302\0303\0306\0309\031B\0323', ''), 'àáảãạăằắẳẵặâầấẩẫậÀÁẢÃẠĂẰẮẲẴẶÂẦẤẨẪẬèéẻẽẹêềếểễệÈÉẺẼẸÊỀẾỂỄỆìíỉĩịÌÍỈĨỊòóỏõọôồốổỗộơờớởỡợÒÓỎÕỌÔỒỐỔỖỘƠỜỚỞỠỢùúủũụưừứửữựÙÚỦŨỤƯỪỨỬỮỰỳýỷỹỵỲÝỶỸỴđĐÐ', 'aaaaaaaaaaaaaaaaaAAAAAAAAAAAAAAAAAeeeeeeeeeeeEEEEEEEEEEEiiiiiIIIIIoooooooooooooooooOOOOOOOOOOOOOOOOOuuuuuuuuuuuUUUUUUUUUUUyyyyyYYYYYdDD'))
) STORED;

ALTER TABLE phieu_phan_anh ADD COLUMN IF NOT EXISTS address_folded TEXT GENERATED ALWAYS AS (
    lower(translate(translate(dia_chi, U&'\0300\0301\0302\0303\0306\0309\031B\0323', ''), 'àáảãạăằắẳẵặâầấẩẫậÀÁẢÃẠĂẰẮẲẴẶÂẦẤẨẪẬèéẻẽẹêềếểễệÈÉẺẼẸÊỀẾỂỄỆìíỉĩịÌÍỈĨỊòóỏõọôồốổỗộơờớởỡợÒÓỎÕỌÔỒỐỔỖỘƠỜỚỞỠỢùúủũụưừứửữựÙÚỦŨỤƯỪỨỬỮỰỳýỷỹỵỲÝỶỸỴđĐÐ', 'aaaaaaaaaaaaaaaaaAAAAAAAAAAAAAAAAAeeeeeeeeeeeEEEEEEEEEEEiiiiiIIIIIoooooooooooooooooOOOOOOOOOOOOOOOOOuuuuuuuuuuuUUUUUUUUUUUyyyyyYYYYYdDD'))
) STORED;

-- ---------------------------------------------------------------------------
-- MEASUREMENT (question 1): rows each rewrite touched, per commune. Counts and tenant ids only.
-- ---------------------------------------------------------------------------
DO $$
DECLARE r record;
BEGIN
    FOR r IN SELECT tenant_id, count(*) AS n FROM nhiem_vu GROUP BY tenant_id ORDER BY tenant_id LOOP
        RAISE NOTICE '0041: commune % holds % task row(s)', r.tenant_id, r.n;
    END LOOP;
    FOR r IN SELECT tenant_id, count(*) AS n FROM phieu_phan_anh GROUP BY tenant_id ORDER BY tenant_id LOOP
        RAISE NOTICE '0041: commune % holds % petition row(s)', r.tenant_id, r.n;
    END LOOP;
END $$;

-- ---------------------------------------------------------------------------
-- POST-CHECK: each column is present and STORED-GENERATED on the parent AND on every partition. A
-- partition without it would fail every search on that commune's slice with "column does not exist".
-- ---------------------------------------------------------------------------
DO $$
DECLARE
    c        record;
    expected int;
    present  int;
BEGIN
    FOR c IN SELECT * FROM (VALUES ('nhiem_vu', 'title_folded'),
                                   ('phieu_phan_anh', 'content_folded'),
                                   ('phieu_phan_anh', 'address_folded')) AS v(tbl, col) LOOP
        SELECT 1 + count(*) INTO expected FROM pg_inherits WHERE inhparent = c.tbl::regclass;
        SELECT count(*) INTO present
        FROM pg_attribute a
        WHERE a.attname = c.col AND a.attgenerated = 's' AND NOT a.attisdropped
          AND (a.attrelid = c.tbl::regclass
               OR a.attrelid IN (SELECT inhrelid FROM pg_inherits WHERE inhparent = c.tbl::regclass));
        IF present <> expected THEN
            RAISE EXCEPTION '0041: %.% is a stored generated column on % of % relations',
                c.tbl, c.col, present, expected;
        END IF;
    END LOOP;
END $$;

-- ---------------------------------------------------------------------------
-- REVERSAL (rule 7, invariant 4). Run by a person, in ONE transaction; core/migrate has no automatic
-- rollback (ADR 0013). LOSSLESS: the three columns are derived and their sources are untouched. Deploy
-- the service build from before this migration FIRST — the current build's search reads the columns.
--
--   ALTER TABLE phieu_phan_anh DROP COLUMN IF EXISTS address_folded;
--   ALTER TABLE phieu_phan_anh DROP COLUMN IF EXISTS content_folded;
--   ALTER TABLE nhiem_vu       DROP COLUMN IF EXISTS title_folded;
-- and remove this file's row from `schema_migration`.
-- ---------------------------------------------------------------------------

-- BACKSTOP: every partitioned table in this schema must actually have partitions (0002, §BACKSTOP).
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
