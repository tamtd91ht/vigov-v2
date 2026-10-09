-- 0040 — VALIDATE the timeline act list 0039 added NOT VALID. Schema only: no row is written.
--
-- WHY A FILE OF ITS OWN: 0039's header. In short — the scan of every existing row runs here under SHARE
-- UPDATE EXCLUSIVE, which lets every commune keep reading and writing its timeline meanwhile, instead of
-- under 0039's ACCESS EXCLUSIVE. core/migrate gives each file its own transaction, so 0039's lock is
-- released before this scan starts.
--
-- IT CANNOT FAIL ON GOOD DATA: every existing row was admitted by 0030's eleven codes, all of which are
-- in 0039's list. If it DOES fail, a row holds a code that no migration ever allowed — the schema was
-- altered by hand. That is a finding for a person, never something to work around here: the constraint
-- stays NOT VALID (still enforced on every new write) and the service refuses to start until it is
-- resolved.
--
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. ROWS PER COMMUNE: zero written; every row of all 32 partitions is read once (0039's NOTICE gave
--      the counts).
--   2. IF IT STOPS HALF-WAY: one transaction; nothing is half-validated. Re-running is a no-op once valid.
--   3. HOW IT IS REVERSED: together with 0039 — see 0039's REVERSAL. "Validated" has no object to drop.
--   4. READ PATHS THAT CHANGE MEANING: none. Validation changes which rows exist by nothing.
--   5. RETENTION: nothing deleted or edited.

ALTER TABLE nhat_ky_phan_anh VALIDATE CONSTRAINT nhat_ky_phan_anh_hanh_vi_hop_le;

-- ---------------------------------------------------------------------------
-- POST-CHECK: the constraint is present and VALIDATED on the parent AND on every partition. A partition
-- that lacks it accepts any code; one left NOT VALID was never scanned. Either is refused here, loudly,
-- rather than assumed from PostgreSQL's recursion rules.
-- ---------------------------------------------------------------------------
DO $$
DECLARE
    expected int;
    present  int;
    pending  int;
BEGIN
    SELECT 1 + count(*) INTO expected
    FROM pg_inherits WHERE inhparent = 'nhat_ky_phan_anh'::regclass;

    SELECT count(*), count(*) FILTER (WHERE NOT c.convalidated) INTO present, pending
    FROM pg_constraint c
    WHERE c.conname = 'nhat_ky_phan_anh_hanh_vi_hop_le'
      AND c.contype = 'c'
      AND (c.conrelid = 'nhat_ky_phan_anh'::regclass
           OR c.conrelid IN (SELECT inhrelid FROM pg_inherits
                             WHERE inhparent = 'nhat_ky_phan_anh'::regclass));

    IF present <> expected OR pending <> 0 THEN
        RAISE EXCEPTION
            '0040: nhat_ky_phan_anh_hanh_vi_hop_le on % of % relations, % not validated',
            present, expected, pending
            USING HINT = 'Every partition must carry the validated act list (0039). Do not add it per '
                         'partition by hand: find why the parent''s constraint did not reach it.';
    END IF;
END $$;

-- ---------------------------------------------------------------------------
-- BACKSTOP: every partitioned table in this schema must actually have partitions (0002, §BACKSTOP).
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
