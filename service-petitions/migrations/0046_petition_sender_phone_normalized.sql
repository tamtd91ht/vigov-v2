-- 0046 — the sender's phone number in NORMALISED NATIONAL FORM, and the index the contact-directory
-- link reads through (ADR 0102 #3).
--
-- WHY THIS COLUMN EXISTS: ADR 0102 #1 settles the petition ↔ citizen-directory link as DERIVED at read
-- time from the key (tenant_id, normalised phone). Neither side stores the other's id. identity owns
-- "is this number in the directory"; petitions owns "which petitions came from this number". To answer
-- the second question petitions must compare numbers in ONE form — `0912…`, `+84912…`, `84 912 …` and
-- `0912.…` are the same subscriber, and the raw column below holds whatever the sender typed.
--
-- `nguoi_gui_dien_thoai` (0004:264) IS NOT TOUCHED. It stays the record of what was filed (rule 7: no
-- silent edit of an archival record). This column is a DERIVED match key beside it, nothing more.
--
-- NO NORMALISATION LOGIC IN THIS FILE, deliberately. The rule that turns a typed number into the
-- national form lives in ONE place, core's normaliser (ADR 0102 #3), and identity normalises the
-- directory side with the same function. A second copy in SQL would be a second rule, and the day the
-- two disagree on one prefix a citizen's petitions silently stop appearing under their directory row —
-- no error, just a short list. So:
--   * new rows: the service writes the column at intake, through core's normaliser;
--   * old rows: a resumable, per-commune Go backfill job writes it, through the same normaliser.
-- Until that job has run for a commune, that commune's older petitions simply do not match — a SHORT
-- list, never a WRONG one (see question 4).
--
-- WHY NOT A GENERATED COLUMN (the 0041 shape): a generated column needs the rule in SQL, which is the
-- second copy refused above.
--
-- ⚠ PERSONAL DATA (rule 3, Decree 13/2023). This is the citizen's phone number in another spelling.
-- Everything 0004:212-217 says about `nguoi_gui_dien_thoai` applies: never logged, never in an error,
-- a URL, an event or a cache key. AND, UNLIKE 0041's generated columns, IT IS NOT RECOMPUTED BY
-- POSTGRESQL: an anonymisation under Decree 13 (rule 7, invariant 7) that clears the raw number MUST
-- clear this column in the same UPDATE, or the number survives the erasure.
--
-- ANONYMOUS PETITIONS NEVER MATCH (ADR 0102 #4, ADR 0008). The column is still written for them — the
-- raw number is stored for an anonymous petition too (0004:241-244), and a column that is NULL for some
-- reasons and absent for others is a column nobody can reason about. The EXCLUSION is in the read: the
-- index below does not contain anonymous rows, and the query it serves says `an_danh = false`.
--
-- WHY A NEW FILE AND NOT AN EDIT OF 0004: core/migrate compares the checksum of every applied file at
-- startup. Editing an applied file either stops the service (ErrChecksumLech) or leaves two databases
-- claiming one schema version while holding two schemas.
--
-- ---------------------------------------------------------------------------
-- THE QUERY THE INDEX SERVES (TASK-04, "Xem phản ánh" on a directory row), as page.QueryPage assembles
-- it for the register's default sort (`booked_at` = `vao_so_luc`, store/xu_ly_phan_anh.go:68-71):
--
--   WHERE tenant_id = $1 AND sender_phone_normalized = $2
--     AND an_danh = false AND deleted_at IS NULL
--     [AND (vao_so_luc, id) < ($x, $y)]
--   ORDER BY vao_so_luc DESC, id DESC LIMIT $n
--
-- WHY THESE COLUMNS, IN THIS ORDER:
--
--   tenant_id                 first, like every index in this system (rule 1). Also the hash partition
--                             key (0004:374, ADR 0096): the planner prunes to one partition first.
--   sender_phone_normalized   the equality that narrows one commune's register to one number.
--   vao_so_luc DESC, id DESC  the sort column and its tie-break, in the direction QueryPage writes
--                             them, so the keyset comparison is a range scan that stops after LIMIT
--                             rows — no sort node, no OFFSET.
--
-- THE PARTIAL PREDICATE. The planner uses a partial index only when it can prove the query's WHERE
-- implies the index's; TEXT-IDENTICAL clauses are the form that proof never misses, so the store must
-- write `an_danh = false AND deleted_at IS NULL` exactly so. `sender_phone_normalized IS NOT NULL` is
-- implied by the query's `= $2` (a strict operator), and keeps every petition without a usable number
-- out of the index. If the store ever writes a different predicate the index silently stops being used —
-- results stay correct, only the plan degrades.
--
-- `deleted_at IS NULL` is the soft-delete test (rule 7, invariant 2). It is the `$ne: true` shape: a
-- row predating the column has it NULL and matches. There is no boolean "deleted" flag on this table.
--
-- MERGED PETITIONS (`merged_into`, 0037) ARE NOT EXCLUDED. Whether a merged petition still appears in a
-- citizen's list is a read-path decision for TASK-04, not something to bake into a schema object.
--
-- ---------------------------------------------------------------------------
-- THE LOCKS THIS TAKES — same standing as 0014, read its section before applying to a large register.
--
--   * ADD COLUMN, nullable, NO default: catalog-only since PostgreSQL 11 — no table rewrite. It takes
--     ACCESS EXCLUSIVE on the parent and the 8 partitions for the catalog change only.
--   * The CHECK is validated by a scan of every row; every row is NULL in a column created a statement
--     earlier, so the scan reads and passes, under the same ACCESS EXCLUSIVE.
--   * CREATE INDEX: plain, not CONCURRENTLY — refused on a partitioned parent, and core/migrate runs the
--     file in one transaction anyway (0014 §THE LOCK). Index entries: none to build beyond a scan,
--     because no row has the column set yet. SHARE lock; reads proceed, writes wait.
-- One file, one transaction: every lock above is held until COMMIT. At current sizes (hundreds to low
-- thousands of rows per commune — an ASSUMPTION, not a measurement) that is milliseconds. The operator
-- checks first:   SELECT count(*), pg_size_pretty(pg_total_relation_size('phieu_phan_anh')) FROM phieu_phan_anh;
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. ROWS PER COMMUNE: zero written. No row is inserted, updated or deleted; the CHECK and the index
--      READ every row once. The NOTICE below gives the per-commune count the later backfill will face
--      (tenant ids and counts only — never a number).
--      PER COMMUNE: a column and an index are schema objects over the parent table, not commune data, so
--      there is no per-commune progress to record here. The per-commune, resumable part is the BACKFILL,
--      which is a Go job (TASK-04), not this file.
--   2. IF IT STOPS HALF-WAY: it cannot land half-applied — one file, one transaction with its progress row
--      (core/migrate). ADD COLUMN IF NOT EXISTS / CREATE INDEX IF NOT EXISTS make a retry cost nothing.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom. LOSSLESS — the column is derived from
--      `nguoi_gui_dien_thoai`, which is untouched, so dropping it loses nothing the backfill cannot
--      rebuild.
--   4. READ PATHS THAT CHANGE MEANING WHILE HALF-APPLIED: none during this file — no existing read names
--      the column. AFTER it and BEFORE the backfill has finished for a commune, the contact-directory
--      list reads a NULL column on that commune's older petitions and does not show them: the list is
--      SHORT. It is never wrong (NULL never equals a number, so no stranger's petition can appear), and
--      no existing screen — register, citizen list, statistics, reports — reads this column at all.
--      TASK-04 ships the backfill with the read path so the short window is per commune and bounded.
--   5. RETENTION: `phieu_phan_anh` is an ARCHIVAL RECORD (rule 7). No existing column, constraint or
--      trigger changes; `ho_so_luu_tru_bat_bien` (0004) still guards it. A later single-column UPDATE of
--      this column passes every trigger on the table: 0004, 0011 and 0032 compare named columns, and
--      0037's merge guard fires only on UPDATE OF merged_into, merged_at, merged_by, linh_vuc.
-- ---------------------------------------------------------------------------

-- Never the empty string: '' = '' would MATCH two unrelated senders who both left the box blank, and
-- link them to each other's directory row. No number → NULL. This is a shape backstop, not a phone rule:
-- what a valid national number looks like is core's normaliser's business alone.
ALTER TABLE phieu_phan_anh ADD COLUMN IF NOT EXISTS sender_phone_normalized TEXT
    CONSTRAINT phieu_phan_anh_sender_phone_normalized_not_empty
    CHECK (sender_phone_normalized IS NULL OR sender_phone_normalized <> '');

CREATE INDEX IF NOT EXISTS phieu_phan_anh_by_sender_phone
    ON phieu_phan_anh (tenant_id, sender_phone_normalized, vao_so_luc DESC, id DESC)
    WHERE an_danh = false AND deleted_at IS NULL AND sender_phone_normalized IS NOT NULL;

-- ---------------------------------------------------------------------------
-- MEASUREMENT (question 1): rows the backfill will face, per commune. Tenant ids and counts only.
-- ---------------------------------------------------------------------------
DO $$
DECLARE r record;
BEGIN
    FOR r IN SELECT tenant_id,
                    count(*) FILTER (WHERE nguoi_gui_dien_thoai IS NOT NULL) AS with_phone,
                    count(*) AS total
             FROM phieu_phan_anh GROUP BY tenant_id ORDER BY tenant_id LOOP
        RAISE NOTICE '0046: commune % holds % petition row(s), % with a sender phone to backfill',
            r.tenant_id, r.total, r.with_phone;
    END LOOP;
END $$;

-- ---------------------------------------------------------------------------
-- POST-CHECK: the column is on the parent AND on every partition, and the index cascaded to every
-- partition. A partition without the column would fail every intake on that commune's slice with
-- "column does not exist" once the service writes it; a partition without the index would answer
-- correctly but scan the commune's whole register.
-- ---------------------------------------------------------------------------
DO $$
DECLARE
    expected int;
    present  int;
    indexed  int;
BEGIN
    SELECT 1 + count(*) INTO expected FROM pg_inherits WHERE inhparent = 'phieu_phan_anh'::regclass;

    SELECT count(*) INTO present
    FROM pg_attribute a
    WHERE a.attname = 'sender_phone_normalized' AND NOT a.attisdropped
      AND (a.attrelid = 'phieu_phan_anh'::regclass
           OR a.attrelid IN (SELECT inhrelid FROM pg_inherits WHERE inhparent = 'phieu_phan_anh'::regclass));
    IF present <> expected THEN
        RAISE EXCEPTION '0046: phieu_phan_anh.sender_phone_normalized is on % of % relations',
            present, expected;
    END IF;

    SELECT 1 + count(*) INTO indexed
    FROM pg_inherits
    WHERE inhparent = 'phieu_phan_anh_by_sender_phone'::regclass;
    IF indexed <> expected THEN
        RAISE EXCEPTION '0046: phieu_phan_anh_by_sender_phone covers % of % relations', indexed, expected;
    END IF;
END $$;

-- ---------------------------------------------------------------------------
-- REVERSAL (rule 7, invariant 4). Run by a person, in ONE transaction; core/migrate has no automatic
-- rollback (ADR 0013). LOSSLESS: the column is derived from `nguoi_gui_dien_thoai`, which is untouched,
-- and the backfill job rebuilds it. Deploy the service build from before TASK-04 FIRST — the newer
-- build writes and reads the column.
--
--   DROP INDEX IF EXISTS phieu_phan_anh_by_sender_phone;          (the 8 partition indexes go with it)
--   ALTER TABLE phieu_phan_anh DROP COLUMN IF EXISTS sender_phone_normalized;   (its CHECK goes with it)
-- and remove this file's row from `schema_migration` in the same transaction.
--
-- The consequence of reversing is that the contact-directory link has no key to read — the
-- "Xem phản ánh" list is empty, never wrong.
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
                         'MODULUS 8 partition loop (ADR 0096) in the same migration that '
                         'declares PARTITION BY.';
    END IF;
END $$;
