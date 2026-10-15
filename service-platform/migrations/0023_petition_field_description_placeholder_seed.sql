-- 0023 — seed petition_field.description_placeholder (0022) for the five tier-1 codes that match a
-- prototype category (../vigov-require apps/miniapp/src/mocks/data.ts `feedbackCategories`): rac-thai,
-- giao-thong, cap-thoat-nuoc, trat-tu-do-thi, khac.
--
-- WHICH TEXT: option 2 of 0022's header, chosen by the main session on 11/10/2026 — each prototype
-- sentence keeps its shape ("Ví dụ: <what> <where, generically> <consequence>") with EVERY place name
-- removed (chợ Hà Lam, đường Nguyễn Thành, đường Tiểu La, Quốc lộ 1A, số nhà, kiệt) and "cán bộ xã"
-- written as "cán bộ", so a ward's resident reads nothing wrong. That is what ADR 0060 §4 and ADR 0079
-- ("Placeholder có tên xã — không ghi cứng tên xã") require of a tier-1 value every commune sees.
-- The three prototype categories whose ids match no tier-1 code (ve-sinh-moi-truong, dien-chieu-sang,
-- an-ninh-trat-tu) are not mapped onto a differently-named code: that would be choosing which field a
-- sentence describes. Their codes keep NULL (the client's generic hint).
--
-- WHY ONLY ROWS STILL NULL: a placeholder an operator has already typed is a sentence somebody chose
-- and signed; a migration does not overwrite it (0014's reasoning). Such a row gets no trail entry.
--
-- WHY A NEW FILE: core/migrate compares the checksum of every applied file at startup.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. ROWS PER COMMUNE: none — no commune column. At most FIVE petition_field rows rewritten and FIVE
--      platform_audit_log entries appended, platform-wide.
--   2. IF IT STOPS HALF-WAY: it cannot. One statement writes the rows and their trail entries, in one
--      transaction with the runner's progress row. A re-run is free: the rows are no longer NULL, the
--      UPDATE matches nothing, RETURNING yields nothing, no second entry.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. READ PATHS THAT CHANGE MEANING: none in the database (rows and entries commit together). Until
--      the Go card adds the column to ListPetitionFields nobody reads it; after that, the five codes
--      show an example sentence instead of the generic hint — presentation only.
--   5. RETENTION: nothing removed; codes untouched (petition_field_guard unchanged). The previous value
--      (NULL) survives in each entry's `before`; the trail is append-only (0008).
--
-- PERSONAL DATA (rule 3): none. Vendor-written example sentences, no person, no place.
-- ---------------------------------------------------------------------------

WITH seed (code, placeholder) AS (
    VALUES
        ('rac-thai',       'Ví dụ: Có đống rác lớn ở khu đất trống sau chợ, bốc mùi nhiều ngày chưa ai thu gom.'),
        ('giao-thong',     'Ví dụ: Mặt đường đoạn trước khu dân cư có ổ gà sâu, xe máy đi qua hay bị ngã.'),
        ('cap-thoat-nuoc', 'Ví dụ: Đường ống nước vỡ ở đầu đường vào khu dân cư, nước chảy tràn ra đường.'),
        ('trat-tu-do-thi', 'Ví dụ: Hàng quán bày bàn ghế lấn hết vỉa hè đoạn gần chợ, người đi bộ phải đi xuống lòng đường.'),
        ('khac',           'Ví dụ: Mô tả rõ sự việc, ở đâu, xảy ra từ khi nào để cán bộ nắm được.')
),
changed AS (
    UPDATE petition_field AS f
       SET description_placeholder = s.placeholder,
           updated_at              = now(),
           updated_by              = 'system'
      FROM seed AS s
     WHERE f.code = s.code
       AND f.description_placeholder IS NULL
    RETURNING f.code, f.description_placeholder
)
INSERT INTO platform_audit_log (actor, action, subject, before, after, reason)
SELECT 'system', 'petition_field.changed', c.code,
       jsonb_build_object('description_placeholder', NULL),
       jsonb_build_object('description_placeholder', c.description_placeholder),
       'migration 0023_petition_field_description_placeholder_seed.sql: cau vi du o Mo ta theo prototype, '
       'bo moi dia danh va chu xa (phien chinh chot phuong an 2, 11/10/2026; ADR 0060 muc 4, ADR 0079)'
FROM changed c;

-- ---------------------------------------------------------------------------
-- CHECK AFTER APPLYING. Silent when it matches nothing, by design.
--
--   SELECT code, description_placeholder, updated_by FROM petition_field ORDER BY sort_order;
--   SELECT subject, occurred_at FROM platform_audit_log
--    WHERE action = 'petition_field.changed' AND reason LIKE 'migration 0023%';
--
-- A code with no such entry already had an operator's sentence; it was left alone on purpose.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- REVERSAL (rule 7, invariant 4). Run by a person, in ONE transaction. Prose, not runnable.
--
-- POSSIBLE ONLY for a code whose row still holds THIS file's sentence and updated_by = 'system', and
-- whose LAST platform_audit_log entry is the one this file wrote. For each such code:
--   1. UPDATE it back to NULL, keyed on code AND description_placeholder = this file's sentence (never
--      an unfiltered UPDATE), with updated_at = now(), updated_by = 'system';
--   2. append one platform_audit_log entry (before = the sentence, after = NULL), reason naming the
--      reversal. The entry this file wrote STAYS — the trail is append-only;
--   3. remove this file's progress row from schema_migration
--      (ten = '0023_petition_field_description_placeholder_seed.sql').
-- A row an operator has changed since is no longer this file's to reverse.
-- ---------------------------------------------------------------------------

-- BACKSTOP, carried forward from 0002. This file declares no partitioned table; it runs anyway.
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
