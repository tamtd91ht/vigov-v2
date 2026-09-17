-- platform — tenant_succession: which unit an old commune's identifier leads to today.
--
-- WHY A NEW FILE AND NOT AN EDIT OF 0001: 0001 has been applied and core/migrate compares the
-- checksum of every applied file at startup. An applied migration is immutable.
--
-- WHY IT LIVES IN service-platform: ADR 0003 puts the commune registry here, and this table is
-- registry metadata about communes themselves — the same category as `tenant` and
-- `tenant_domain`, not business data belonging to any one commune. It is also what
-- ResolveTenantAlias in proto/vigov/platform/v1/platform.proto reads, and that RPC is served
-- by this service.
--
-- "SUCCESSION", NOT "ALIAS", AND THE WORD MATTERS. An alias is a second name for one thing.
-- .claude/skills/admin-unit-merge, invariant 6, says the opposite of that: "Commune A does not
-- 'become' commune B" — there are two legal entities, the old one deactivated and kept, the new
-- one created. Calling the relation an alias invites the next person to write code that treats
-- the two ids as interchangeable, and interchangeable is exactly what they are not: an issued
-- document keeps the issuing authority it had at issuance (invariant 2), and a document number
-- from the old commune is never renumbered into the new one's series (invariant 3).
--
-- THE FIVE MIGRATION QUESTIONS:
--
--   1. HOW MANY ROWS PER COMMUNE: this table has no commune column, and it holds one row per
--      (old unit, successor unit) edge. After one merger of two communes into one: two rows.
--      It stays in the tens for the lifetime of the platform.
--   2. IF IT STOPS HALF-WAY: it cannot. One file, one transaction, progress row inside it.
--   3. HOW IT IS REVERSED: see "REVERSAL" at the bottom.
--   4. WHICH READ PATHS CHANGE MEANING WHILE IT IS HALF-APPLIED: none. The table is new and
--      empty, and nothing reads it yet — ResolveTenantAlias has no implementation behind it.
--      No existing query changes its answer.
--   5. RETENTION: nothing is removed. Rows written here are the administrative record of a
--      reorganisation and are never deleted — see the note on `can_cu`.

-- ---------------------------------------------------------------------------
-- tenant_succession — one row per edge: an old unit, and ONE unit that succeeded it.
--
-- NO tenant_id, AND FOR THE SAME REASON `tenant` HAS NONE: this is not data belonging to a
-- commune, it is a statement about which communes exist and what replaced what. Scoping it by
-- commune would require knowing the commune before being able to find out which commune the
-- identifier leads to — the question the table exists to answer. Access goes through the
-- platform registry repository, the sanctioned unscoped reader, never through core/store.Scoped.
--
-- ONE ROW PER EDGE, NOT ONE ROW PER OLD UNIT, BECAUSE A SPLIT HAS SEVERAL SUCCESSORS. The key
-- is (tu_id, den_id), so one old unit may appear many times. A key of (tu_id) alone would make
-- a one-to-one relation impossible to widen later without a migration over archival mapping
-- rows, and it would contradict a contract that is already published:
-- ResolveTenantAliasResponse.tenants is `repeated` precisely for this case.
--
-- CHAINS ARE ANSWERED BY WALKING, NOT BY DENORMALISING. A -> B and later B -> C makes A's
-- printed QR lead to C, and the query that says so is:
--
--     WITH RECURSIVE ke_thua(id) AS (
--         SELECT $1
--         UNION
--         SELECT s.den_id FROM tenant_succession s JOIN ke_thua k ON s.tu_id = k.id
--     )
--     SELECT t.* FROM tenant t JOIN ke_thua k ON t.id = k.id WHERE t.dang_hoat_dong;
--
-- Storing a "leads to today" column instead would be a second source for a fact the edges
-- already contain, and the day B -> C is entered, every row that named B would have to be
-- rewritten. Rewriting is what this table exists to avoid.
--
-- `can_cu` AND `hieu_luc_tu` ARE WHAT MAKE THIS AN ADMINISTRATIVE RECORD RATHER THAN A LOOKUP
-- TABLE. Every merger of administrative units has a document behind it and a date it took
-- effect. Without those two columns, the answer to "on what authority does this system send a
-- citizen who scanned commune A's QR to commune B" is "because somebody typed it in". The
-- moment that question gets asked is a boundary dispute, and that is not the moment to find out
-- the system never recorded the answer. Both are NOT NULL: a row without them is the row that
-- cannot be defended.
--
-- CYCLES ARE BLOCKED BY A TRIGGER, NOT BY TRUST. See the function below.
--
-- THIS TABLE IS NEVER A URL RESOURCE. It is read through ResolveTenantAlias, which answers with
-- the commune an identifier leads to; exposing the succession graph itself would let anyone
-- enumerate which communes have existed and how they were reorganised, which is the same
-- enumeration the HTTP edge refuses when it answers 404 to both an unknown host and a
-- deactivated commune.
-- ---------------------------------------------------------------------------
-- @entity: TenantSuccession
-- @scope:  platform
CREATE TABLE IF NOT EXISTS tenant_succession (
    -- The unit that no longer receives work. It stays in `tenant`, deactivated, never deleted
    -- (rule 7, invariant 6) — the foreign key is what enforces that it still exists.
    tu_id        TEXT        NOT NULL REFERENCES tenant (id),
    -- One unit that succeeded it. Several rows share a tu_id when a unit was split.
    den_id       TEXT        NOT NULL REFERENCES tenant (id),
    -- Number/symbol of the resolution or decision that ordered the reorganisation.
    can_cu       TEXT        NOT NULL,
    -- The date it took legal effect, which is not the date somebody entered the row.
    hieu_luc_tu  DATE        NOT NULL,
    ghi_chu      TEXT        NOT NULL DEFAULT '',
    tao_luc      TIMESTAMPTZ NOT NULL DEFAULT now(),
    tao_boi      TEXT        NOT NULL DEFAULT '',
    PRIMARY KEY (tu_id, den_id),
    CONSTRAINT tenant_succession_khong_tro_chinh_no CHECK (tu_id <> den_id),
    CONSTRAINT tenant_succession_co_can_cu CHECK (btrim(can_cu) <> '')
);

COMMENT ON TABLE tenant_succession IS
    'Don vi cu -> don vi ke thua. HAI PHAP NHAN, khong phai mot thu hai ten. Mot don vi cu co '
    'the co NHIEU don vi ke thua (chia tach).';
COMMENT ON COLUMN tenant_succession.can_cu IS
    'So/ky hieu nghi quyet, quyet dinh sap nhap. Bat buoc: day la can cu hanh chinh, khong phai ghi chu.';

-- Walking backwards: "which old units lead here", for a commune that wants to see what it
-- inherited. The primary key already serves the forward walk.
CREATE INDEX IF NOT EXISTS tenant_succession_theo_den ON tenant_succession (den_id);

-- ---------------------------------------------------------------------------
-- Cycle prevention.
--
-- WHY A TRIGGER AND NOT A CHECK: a CHECK constraint sees one row. A cycle is a property of the
-- whole graph — A -> B, B -> C, C -> A is three individually blameless rows. Only a walk can
-- see it.
--
-- WHY IT MUST BE BLOCKED AT ALL RATHER THAN HANDLED ON READ: a cycle here is not a slow query,
-- it is a QR code that resolves to nothing. The recursive walk in the read path uses UNION and
-- therefore terminates, but it terminates having found no active commune, and the citizen is
-- told the identifier leads nowhere. The cause would be a typo in one id entered months
-- earlier, and nothing would report it.
--
-- HONEST LIMIT: two concurrent transactions each inserting one edge can each pass this check
-- and together create a cycle, because neither sees the other's uncommitted row. Closing that
-- needs SERIALIZABLE or a table-level lock on every insert. It is not done here because a
-- reorganisation is entered by a person, from a document, a handful of times per year — the
-- concurrency window is theoretical while the cost of locking the registry is not. If this
-- table ever gains a bulk import path, that path takes an advisory lock; the trigger stays.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION tenant_succession_chan_vong_lap() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF EXISTS (
        WITH RECURSIVE di(id) AS (
            SELECT NEW.den_id
            UNION
            SELECT s.den_id FROM tenant_succession s JOIN di ON s.tu_id = di.id
        )
        SELECT 1 FROM di WHERE id = NEW.tu_id
    ) THEN
        RAISE EXCEPTION
            'tenant_succession: canh % -> % tao thanh vong lap ke thua', NEW.tu_id, NEW.den_id
            USING HINT = 'Don vi ke thua da dan nguoc ve don vi cu. Kiem tra lai ULID da nhap.';
    END IF;
    RETURN NEW;
END $$;

DROP TRIGGER IF EXISTS tenant_succession_chan_vong_lap ON tenant_succession;
CREATE TRIGGER tenant_succession_chan_vong_lap
    BEFORE INSERT OR UPDATE ON tenant_succession
    FOR EACH ROW EXECUTE FUNCTION tenant_succession_chan_vong_lap();

-- ---------------------------------------------------------------------------
-- REVERSAL (rule 7, invariant 4).
--
-- BEFORE THE FIRST REORGANISATION IS RECORDED — the table is empty and reversal is exact:
-- remove the trigger, the function and the table, then remove this file's progress row from the
-- schema_migration table, keyed on ten = '0003_tenant_succession.sql'. Written as prose rather
-- than as a runnable line, because a runnable line is a line that gets run.
--
-- AFTER ANY ROW EXISTS — there is no reversal. A row here is the record of an administrative
-- reorganisation, carrying the resolution it was based on. Discarding it is rule 7 stop
-- condition #1 and needs an explicit decision by the user plus a verified backup.
--
-- PER-COMMUNE AND RESUMABLE (rule 7, invariant 5): no backfill, no existing row rewritten,
-- nothing to resume. The whole file is one transaction with its progress row.
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
