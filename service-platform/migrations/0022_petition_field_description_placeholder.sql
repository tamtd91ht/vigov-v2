-- 0022 — petition_field.description_placeholder: the example sentence shown inside the citizen's
-- "Mô tả" box once a field is chosen (the prototype's per-category `placeholder`, owner request of
-- 11/10/2026). Tier 1, platform-wide — the same sentence for every commune that has not overridden it.
--
-- ADR 0060 §4 left this column out ("gợi ý nhập — không") and said adding it later is one optional
-- field that breaks no contract. This is that field. ADR 0060 §4's REASON still stands, and it is why
-- THIS FILE SEEDS NOTHING:
--
--   * The only source text is the prototype (../vigov-require apps/miniapp/src/mocks/data.ts
--     `feedbackCategories`). Five of its eight ids match a tier-1 code here (rac-thai, giao-thong,
--     cap-thoat-nuoc, trat-tu-do-thi, khac). Four of those five sentences name ONE commune's places
--     (chợ Hà Lam, đường Nguyễn Thành, đường Tiểu La, Quốc lộ 1A), and the fifth says "cán bộ xã",
--     which is wrong on a ward's form. A tier-1 value is shown to EVERY commune: a resident of another
--     commune would be shown Thăng Bình's market as the example. ADR 0060 §4 and ADR 0079 ("Placeholder
--     có tên xã — không ghi cứng tên xã", rule 1 invariant 10) both refuse that.
--   * Which text to seed — the prototype verbatim, a de-localised rewrite, or nothing at tier 1 and a
--     per-commune value at tier 2 — is the owner's call. A seed is then one further migration
--     (UPDATE … WHERE code = … AND description_placeholder IS NULL, with a platform_audit_log entry in
--     the same statement, 0014's shape).
--
-- NULL = no example sentence; the client shows its generic hint. That is today's behaviour exactly.
--
-- NAME (rule 12, ADR 0051): English, matching the table's other columns.
--
-- WHY A NEW FILE: core/migrate compares the checksum of every applied file at startup.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. ROWS PER COMMUNE: none — no commune column. Twelve platform rows today gain one NULL column;
--      catalogue-only, no rewrite. The CHECK scans twelve rows.
--   2. IF IT STOPS HALF-WAY: it cannot — one file, one transaction (core/migrate), progress row inside
--      it. ADD COLUMN IF NOT EXISTS and a constraint behind a pg_constraint lookup; a retry costs nothing.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. READ PATHS THAT CHANGE MEANING: none. store/petition_field.go names its columns (no `SELECT *`),
--      so ListPetitionFields returns exactly what it did until the Go card adds the column to the read
--      and to platform.proto (optional field — ADR 0060 §4, rule 2 forbidden #4).
--   5. RETENTION: nothing dropped, retyped or renumbered; codes untouched; petition_field_guard
--      unchanged (it refuses DELETE and renaming a code, whatever the other columns are).
--
-- PERSONAL DATA (rule 3): none. The value is a vendor-written example; the CHECK keeps it bounded.
-- ---------------------------------------------------------------------------

ALTER TABLE petition_field ADD COLUMN IF NOT EXISTS description_placeholder TEXT;

DO $$ BEGIN
    -- NULL = none. A blank value is NULL's job, not a second spelling of it. 500 is a vendor bound
    -- (one or two sentences; the prototype's longest is ~120 characters).
    IF NOT EXISTS (SELECT 1 FROM pg_constraint
                   WHERE conrelid = 'petition_field'::regclass
                     AND conname = 'petition_field_description_placeholder_shape') THEN
        ALTER TABLE petition_field ADD CONSTRAINT petition_field_description_placeholder_shape
            CHECK (description_placeholder IS NULL
                   OR (btrim(description_placeholder) <> ''
                       AND char_length(description_placeholder) <= 500
                       AND description_placeholder !~ '[[:cntrl:]]'));
    END IF;
END $$;

COMMENT ON COLUMN petition_field.description_placeholder IS
    'Cau vi du trong o Mo ta cua cong dan, dung chung moi xa (tang 1). NULL = khong co. KHONG ghi ten '
    'dia danh cua mot xa (ADR 0060 muc 4, ADR 0079).';

-- ---------------------------------------------------------------------------
-- REVERSAL (rule 7, invariant 4). Run by a person, in ONE transaction; core/migrate has no automatic
-- rollback (ADR 0013). Prose, not runnable.
--
-- LOSSLESS ONLY WHILE THIS READS ZERO:
--
--   SELECT count(*) FROM petition_field WHERE description_placeholder IS NOT NULL;
--
-- ZERO → drop constraint petition_field_description_placeholder_shape, drop column
-- description_placeholder, and remove this file's row from `schema_migration`
-- (ten = '0022_petition_field_description_placeholder.sql').
--
-- NON-ZERO → an operator or a later seed wrote example sentences; their history is in
-- platform_audit_log (append-only, 0008), so dropping the column loses only the current value. Still a
-- user decision, and the way back is a NEW migration.
-- ---------------------------------------------------------------------------

-- BACKSTOP, carried forward from 0002: a table declared PARTITION BY with no partitions rejects every
-- INSERT. This file declares no partitioned table; it runs anyway.
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
