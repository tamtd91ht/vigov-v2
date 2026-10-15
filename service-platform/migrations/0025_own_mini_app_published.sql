-- 0025 — mini_app.published_at / published_by: an operator's mark that a commune's OWN Mini App
-- (che_do = 'rieng') has been PUBLISHED on Zalo. Owner decision, 13/10/2026.
--
-- WHY THE MARK EXISTS. A QR `https://zalo.me/s/<App ID>/?src=qr` for an App ID Zalo has not
-- published opens only for that app's admin / developer accounts (a trial build); an ordinary
-- citizen cannot enter. This system cannot ask Zalo whether an app is published, so an operator
-- states it. The QR choice reads the mark: the commune's own app only when it is marked published,
-- otherwise the shared app link (`?x=<launch_code>`, 0024).
--
-- NULL MEANS "NOT PUBLISHED", AND THERE IS NO BACKFILL, ON PURPOSE. Every existing row starts
-- unpublished, including own apps that may in fact already be live on Zalo. That is the fail-safe
-- direction: a wrong NULL sends citizens to the shared app, which works; a wrong mark sends them to
-- an app they cannot open. Marking an app that IS live is an operator's act, audited by the Go path
-- that writes it — never guessed by a migration. Consequence, stated: until an operator marks them,
-- QRs of communes whose own app is already published point at the shared app.
--
-- THE TWO CHECKS:
--   * mini_app_published_both_or_neither — published_at and published_by are set together, and
--     published_by is never blank: a mark nobody signed is a mark nobody can defend (rule 6,
--     invariant 8: published_by holds the operator's BUSINESS code, e.g. VH-…, never an internal id).
--   * mini_app_published_own_app_only — only che_do = 'rieng' may carry the mark. The shared app
--     ('chinh') is the fallback itself and is live by definition; a mark on it would be a second,
--     meaningless source for "is the shared app usable". Same shape as 0006's
--     mini_app_xa_khi_va_chi_khi_rieng: a CHECK on che_do, on the same table.
--   Neither check looks at deleted_at / dang_hoat_dong: a removed or switched-off row KEEPS its mark
--   as history (rule 7, forbidden #5: a historical record is not edited away). Whether a
--   removed app is offered at all is the read path's job, exactly as for every other column here.
--
-- NAMES (rule 12, ADR 0051): English. The table's older columns are Vietnamese (ADR 0061 revert);
-- they are not renamed.
--
-- WHY A NEW FILE: core/migrate compares the checksum of every applied file at startup.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. ROWS PER COMMUNE: none are written. Two NULLable columns are ADDED with no default, which in
--      PG16 is a catalogue-only change — no row is rewritten. The two CHECKs scan mini_app once to
--      validate (one row per Zalo App ID: a handful per commune at most); every row passes, because
--      every new column is NULL. PER COMMUNE AND RESUMABLE (rule 7, invariant 5): there is no
--      backfill, so there is nothing to run per commune.
--   2. IF IT STOPS HALF-WAY: it cannot land half-applied — one file, one transaction with the
--      runner's progress row. A re-run is free: ADD COLUMN IF NOT EXISTS, each CHECK sits behind a
--      pg_constraint lookup.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. READ PATHS THAT CHANGE MEANING: none while half-applied (one transaction). After it: no read
--      path names published_at yet; store/ reads name their columns (no `SELECT *` on mini_app), and
--      every INSERT INTO mini_app names its columns, so existing reads and writes behave exactly as
--      before. When the QR path starts reading the mark, every commune's QR resolves to the shared
--      app until an operator marks its own app — the fail-safe stated above.
--   5. RETENTION: nothing dropped, retyped, renumbered or rewritten. No existing value changes;
--      cap_nhat_luc / cap_nhat_boi are not touched. Merged or dissolved communes' rows are not
--      written (rule 7, invariant 6): they gain two NULL columns, nothing else.
--
-- AUDIT (rule 6, invariant 6): this file changes no business value, so it writes no audit entry.
-- Setting or clearing the mark is a write by the Go path, which audits it in the same transaction.
--
-- PERSONAL DATA (rule 3): none. A timestamp and an operator's business code.
-- ---------------------------------------------------------------------------

ALTER TABLE mini_app ADD COLUMN IF NOT EXISTS published_at TIMESTAMPTZ NULL;
ALTER TABLE mini_app ADD COLUMN IF NOT EXISTS published_by TEXT NULL;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint
                   WHERE conrelid = 'mini_app'::regclass
                     AND conname = 'mini_app_published_both_or_neither') THEN
        ALTER TABLE mini_app ADD CONSTRAINT mini_app_published_both_or_neither
            CHECK ((published_at IS NULL) = (published_by IS NULL)
                   AND (published_by IS NULL OR btrim(published_by) <> ''));
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_constraint
                   WHERE conrelid = 'mini_app'::regclass
                     AND conname = 'mini_app_published_own_app_only') THEN
        ALTER TABLE mini_app ADD CONSTRAINT mini_app_published_own_app_only
            CHECK (published_at IS NULL OR che_do = 'rieng');
    END IF;
END $$;

COMMENT ON COLUMN mini_app.published_at IS
    'Luc nguoi van hanh danh dau app RIENG cua xa da phat hanh tren Zalo (quyet dinh chu du an 13/10/2026). '
    'NULL = chua phat hanh: QR dung app chung (?x=). Chi che_do = rieng. Khong suy tu Zalo.';
COMMENT ON COLUMN mini_app.published_by IS
    'Ma nghiep vu cua nguoi van hanh danh dau phat hanh (VH-...), khong phai id noi bo. '
    'Co khi va chi khi published_at co.';

-- ---------------------------------------------------------------------------
-- REVERSAL (rule 7, invariant 4). Run by a person, in ONE transaction; core/migrate has no automatic
-- rollback (ADR 0013). Prose, not runnable — a runnable line is a line that gets run.
--
-- Drop constraints mini_app_published_own_app_only and mini_app_published_both_or_neither, then
-- columns published_by and published_at, and remove this file's row from `schema_migration`
-- (ten = '0025_own_mini_app_published.sql'). Any Go path reading or writing the columns must be
-- reverted in the same release.
--
-- WHAT IS LOST: while no operator has marked any app — the state this file leaves — nothing; both
-- columns are NULL in every row. ONCE A MARK EXISTS, dropping the columns discards an operator's
-- signed statement and flips that commune's QR back to the shared app. The marks are recoverable
-- from the Go path's audit entries (append-only, 0002), which reversal never touches; restoring
-- them is a NEW migration. Once any mark exists, reversal is a user decision, not an operator's
-- (rule 7, stop condition #2).
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
                         'MODULUS 8 partition loop (ADR 0096) in the same migration that '
                         'declares PARTITION BY.';
    END IF;
END $$;
