-- platform — tinh_thanh: the catalogue of province-level administrative units.
--
-- WHY A NEW FILE AND NOT AN EDIT OF 0001: 0001 has been applied and core/migrate compares the
-- checksum of every applied file at startup. An applied migration is immutable.
--
-- WHY IT LIVES IN service-platform: ADR 0003 puts the commune registry here. A province is the
-- unit a commune belongs to, and the only consumer is the commune picker of the citizen channel
-- (ADR 0005), which reads the registry through this service. No commune owns this list.
--
-- WHY THIS TABLE EXISTS AT ALL — AND IT IS NOT "BECAUSE THE LIST IS STABLE".
--
-- `tenant.tinh_thanh` is a display string, typed per commune. The province filter in the picker
-- is therefore `SELECT DISTINCT tinh_thanh`, and the first time two communes in one province
-- spell it differently — one writes "Đà Nẵng", the other "Thành phố Đà Nẵng" — the citizen sees
-- TWO provinces, each holding half the communes of one. Nothing turns red. No test fails. The
-- person who finds out is a citizen who cannot find their own commune in the list, and they
-- have no way to report that the list is what is wrong.
--
-- A closed catalogue with the provider seeding it and the commune only CHOOSING removes the
-- input that produces the defect. That is the reason; stability of the list is incidental, and
-- the list is in fact not stable — see the note on the key.
--
-- THE FIVE MIGRATION QUESTIONS:
--
--   1. HOW MANY ROWS PER COMMUNE: none — this table has no commune column. It holds one row per
--      province-level unit; 34 after the 2025 reorganisation.
--   2. IF IT STOPS HALF-WAY: it cannot. One file, one transaction, progress row inside it.
--   3. HOW IT IS REVERSED: see "REVERSAL" at the bottom.
--   4. WHICH READ PATHS CHANGE MEANING WHILE IT IS HALF-APPLIED: none. The table is new and
--      nothing reads it. `tenant.tinh_thanh` is untouched and every existing query returns
--      exactly what it returned before. That is deliberate — see "WHAT THIS FILE DOES NOT DO".
--   5. RETENTION: nothing is removed, no column is dropped, no type is changed.
--
-- WHAT THIS FILE DOES NOT DO, AND WHY IT IS A SEPARATE MIGRATION:
--
-- It does NOT add tinh_thanh_id to `tenant`, and it does NOT touch `tenant.tinh_thanh`.
-- Connecting the two is a migration over a POPULATED column (rule 7), and the safe shape for it
-- is well known: add a NULLABLE foreign-key column beside the old one, leave the old one intact,
-- backfill per commune, move the read paths, and only then consider the old column — as a
-- separate, later, explicitly decided migration.
--
-- The reason none of that is here is narrower and more concrete: THE CATALOGUE SHIPS EMPTY (see
-- the seed note below). A nullable foreign key pointing at an empty table can never hold a
-- value. Adding it today would put a column into `tenant` that is NULL on every row, for an
-- unknown length of time, while reading as though the join had been made. The column belongs in
-- the same migration as the confirmed seed and the per-commune backfill that fills it, because
-- those three are one change and only make sense together.

-- ---------------------------------------------------------------------------
-- tinh_thanh — province-level administrative units.
--
-- NO tenant_id, same exemption as `tenant` and `tenant_domain`: this is registry data about the
-- shape of the country, not business data belonging to a commune. Reads go through the platform
-- registry repository, the sanctioned unscoped reader, never through core/store.Scoped.
--
-- THE KEY IS AN OPAQUE ULID, NOT A STATISTICAL CODE, AND 2025 IS THE PROOF THAT MATTERS.
-- Rule 1, invariant 2 says an identifier must not carry meaning, and the usual objection is
-- that provinces are stable enough to be an exception. They are not: the 2025 reorganisation
-- cut 63 province-level units to 34. Had the statistical code been the key, that single event
-- would have forced rewriting foreign keys across historical data — rewriting archival records.
-- The same argument as for `tenant.id`, and it costs nothing to get right now.
--
-- THERE IS DELIBERATELY NO STATISTICAL-CODE COLUMN EITHER. It would only be needed for
-- reporting upward to district or province level, and open question #4 ("Cấp huyện/tỉnh xem
-- tổng hợp nhiều xã tới mức chi tiết nào?") is still OPEN. ADR 0021, rule 4: record what
-- exists, never an intention. Adding the column the day that question is answered is one line
-- in a new migration; adding it now means a column nobody fills, which the next person will
-- either fill with a guess or treat as authoritative.
--
-- `dang_hoat_dong` RATHER THAN REMOVAL, for the same reason a merged commune is deactivated and
-- kept (rule 7, invariant 6): if a province is reorganised out of existence, communes that were
-- recorded under it keep pointing at a row that still exists and still has a name.
-- ---------------------------------------------------------------------------
-- @entity: Province
-- @scope:  platform
CREATE TABLE IF NOT EXISTS tinh_thanh (
    id             TEXT        PRIMARY KEY,           -- ULID, 26 chars, opaque
    -- The display name, exactly as it must appear on a citizen's screen. UNIQUE without any
    -- tenant_id, and that uniqueness IS the point of the table: two spellings of one province
    -- is the defect this catalogue exists to make impossible.
    ten            TEXT        NOT NULL,
    thu_tu         INT         NOT NULL DEFAULT 0,
    dang_hoat_dong BOOLEAN     NOT NULL DEFAULT true,
    tao_luc        TIMESTAMPTZ NOT NULL DEFAULT now(),
    cap_nhat_luc   TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT tinh_thanh_id_la_ulid CHECK (length(id) = 26),
    CONSTRAINT tinh_thanh_ten_duy_nhat UNIQUE (ten),
    CONSTRAINT tinh_thanh_ten_khong_rong CHECK (btrim(ten) <> '')
);

COMMENT ON TABLE tinh_thanh IS
    'Danh muc don vi hanh chinh cap tinh. Nha cung cap seed, xa chi duoc CHON. Ly do: '
    'tenant.tinh_thanh la chuoi tu nhap, hai cach viet mot tinh thi cong dan thay hai tinh.';
COMMENT ON COLUMN tinh_thanh.id IS
    'ULID vo nghia. KHONG dung ma thong ke lam khoa: nam 2025 da chung minh tinh cung sap nhap duoc.';

-- The picker lists active provinces in a fixed order. Sorting on `ten` instead would order by
-- Vietnamese collation, which differs between database installations — the same list in a
-- different order for different deployments of the same software.
CREATE INDEX IF NOT EXISTS tinh_thanh_danh_sach
    ON tinh_thanh (thu_tu, ten) WHERE dang_hoat_dong;

-- ---------------------------------------------------------------------------
-- THE TABLE SHIPS EMPTY. THIS IS A REFUSAL, NOT AN OMISSION.
--
-- The 34 province-level units that came out of the 2025 reorganisation are the seed this table
-- wants, and it is not written here because the exact OFFICIAL FORM of each name has not been
-- confirmed against the resolution that created them — in particular whether a row reads
-- "Đà Nẵng" or "Thành phố Đà Nẵng", "Lai Châu" or "Tỉnh Lai Châu". That distinction is not
-- cosmetic here: this column is not an internal label, it is the string printed on a citizen's
-- screen by a government body, and a province name that a government body gets wrong is an
-- incident somebody has to answer for.
--
-- The cost of the two mistakes is not symmetric. An empty catalogue is visible immediately —
-- the picker shows no provinces and somebody asks. A seeded catalogue with a wrong name looks
-- finished, ships, and is discovered by a citizen.
--
-- SEEDING IS A FOLLOW-UP MIGRATION, and it carries the per-commune backfill of
-- `tenant.tinh_thanh` with it, because those two are one change (see "WHAT THIS FILE DOES NOT
-- DO" at the top). It needs, from a person: the 34 names in their official written form.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- REVERSAL (rule 7, invariant 4).
--
-- Exact and lossless at any point before the catalogue is seeded and referenced: remove the
-- index and the table, then remove this file's progress row from the schema_migration table,
-- keyed on ten = '0004_tinh_thanh.sql'. Written as prose rather than as a runnable line,
-- because a runnable line is a line that gets run.
--
-- Once `tenant` rows reference it, the table can no longer be removed without either dropping
-- that reference or orphaning it, and that is a decision for the user with a verified backup.
-- Nothing in this file creates that situation; the follow-up migration will.
--
-- PER-COMMUNE AND RESUMABLE (rule 7, invariant 5): no backfill, no existing row rewritten, so
-- there is nothing to resume and no commune to iterate. The backfill that WILL need it is the
-- follow-up migration that maps each commune's existing `tinh_thanh` string onto a row here —
-- that one touches a populated column on every commune, and it must be written per commune,
-- resumable, recording progress, rather than as one UPDATE. The mechanism for that does not
-- exist yet (ADR 0013, §Giới hạn) and building it is part of that migration, not of this one.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- BACKSTOP, carried forward from 0002: a table declared PARTITION BY with no partitions rejects
-- every INSERT until the first business write fails. The check only describes the state after
-- the NEWEST migration carrying it, so every new file ends with it. This file declares no
-- partitioned table and is expected to find nothing; it runs anyway, because the run after
-- which it is missing is the one that needed it.
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
