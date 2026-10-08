-- 0027 — the commune's deadline rules for CITIZEN LETTERS, per letter type (ADR 0084 #3, ADR 0085 B,
-- ADR 0064). The table `IdentityService.ResolveCitizenLetterDeadline` reads
-- (proto/vigov/identity/v1/identity.proto). Schema only: no row is written.
--
-- WHY A NEW FILE AND NOT AN EDIT OF 0008 / 0016: core/migrate compares the checksum of every applied
-- file at startup. Editing an applied file stops the service or leaves two databases with one version
-- number.
--
-- WHY A SEPARATE TABLE AND NOT COLUMNS ON `sla` (ADR 0085 §Còn mở #2, "bảng riêng"): every `sla`
-- column is a count of WORKING HOURS and all five are NOT NULL reminder thresholds that do not apply to
-- a letter; a letter rule carries its own UNIT (calendar days for complaints, ADR 0064). Two entrances
-- reading one row with two meanings is the trap. `sla`'s `don-thu` row (0016) keeps its job — reminder
-- thresholds (ADR 0079 Q18) — and NOTHING FALLS BACK FROM THIS TABLE TO IT (identity.proto, the
-- ResolveCitizenLetterDeadline comment).
--
-- ONE ROW = (letter type, which deadline, amount, unit). NO ROW = "NOT CONFIGURED": the letter is
-- booked with no deadline ("Không đặt hạn", ADR 0084 #3). The reader must not invent a number.
--
-- ENTITY OWNERSHIP is declared with the `-- @entity` / `-- @scope` marks above the CREATE TABLE
-- (ADR 0021); tools/kb reads them into kb/30-indexes/data-ownership.json. Nobody edits that file.
--
-- NAMES (rule 12, ADR 0051): table, columns and constraints are English. Enum VALUES:
--   letter_type    Vietnamese (ADR 0011) — the SAME four codes as service-documents
--                  0006 `citizen_letter_type_valid`. Two services, one closed set: a value added on
--                  one side and not the other is a letter type nobody can configure.
--   unit           Vietnamese (ADR 0011): gio-lam-viec · ngay-lam-viec · ngay-lich.
--   deadline_kind  Vietnamese (ADR 0011): 'xu-ly-don' (hạn xử lý đơn) · 'giai-quyet' (hạn giải quyết) —
--                  CitizenLetterDeadlineKind PROCESSING / RESOLUTION on the wire, documents' columns
--                  `processing_due_at` / `resolution_due_at`. The stored VALUE is what a person reads
--                  in a query years later, so it follows ADR 0011 like `unit` does.
--
-- ---------------------------------------------------------------------------
-- NOTHING IS SEEDED (owner, 08/10/2026 — ADR 0085 §Còn mở #5 answered "để trống"). The 0008 header's
-- three reasons hold unchanged: a row belongs to one named commune and this path has none (ADR 0013);
-- a number here is a promise a public authority makes (rule 10, forbidden #3); and ADR 0064's 10 / 30
-- / 7 / 30 are a domain-expert's recollection, still flagged "cần pháp chế đối chiếu".
--
-- ---------------------------------------------------------------------------
-- WHICH UNIT FOR WHICH TYPE — ENFORCED IN THE DOMAIN (TASK-06), DELIBERATELY NOT A CHECK HERE.
--
-- Owner's answer of 08/10/2026 (ADR 0085 §Còn mở #3, and ADR 0064's table):
--
--   letter_type          xu-ly-don         giai-quyet
--   kien-nghi-phan-anh   ngay-lam-viec     —
--   de-nghi              ngay-lam-viec     —
--   khieu-nai            ngay-lich         ngay-lich
--   to-cao               ngay-lam-viec     ngay-lich
--
-- Why not a CHECK: the legal table behind it is UNVERIFIED (ADR 0064 "cần pháp chế đối chiếu"). If the
-- legal review moves one cell, a CHECK makes every commune's row for that cell fail at the next
-- migration's validation scan or forces a replacement migration before the fix can ship; a domain
-- rule is one release. The contract already names the outcome: a row whose unit the statute does not
-- allow is FAILED_PRECONDITION ("khoá đơn vị"), never silently used. `gio-lam-viec` stays in the set
-- because ADR 0084 §3 named working hours for the two non-statutory types (ADR 0085 §Còn mở #3 records
-- the conflict); the owner chose days, and the domain refuses hours until that changes.
--
-- WHAT IS A CHECK: `giai-quyet` only for `khieu-nai` / `to-cao`. That is not from the unverified
-- table — it is the shape of the register itself: only those two types pass through `thu-ly`, the act
-- that fixes `resolution_due_at` (ADR 0064, ADR 0085 B2: RESOLUTION counts from `accepted_at`). A
-- `giai-quyet` rule for `de-nghi` would be configuration nothing can ever read, shown on a screen as
-- if it promised something.
--
-- ---------------------------------------------------------------------------
-- THIS TABLE SUPPLIES NUMBERS, NOT ARITHMETIC. Turning (amount, unit) into an instant is
-- ResolveCitizenLetterDeadline's job, in this service and nowhere else (ADR 0064 #4; rule 10,
-- forbidden #2). No overdue column: overdue is derived (rule 10, invariant 3).
--
-- AUDIT: rows are written only by a staff route of this service (not yet built), which writes its
-- audit_log entry in the same transaction (rule 6), as for `sla`. No trigger here — no identity
-- configuration table carries one (0008, 0016, 0017); the append-only triggers are for logs.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. ROWS PER COMMUNE: zero written. Ceiling: 4 xu-ly-don + 2 giai-quyet = 6 live rows a commune.
--   2. IF IT STOPS HALF-WAY: it cannot land half-applied — one file, one transaction with its progress
--      row (core/migrate). Every statement is IF NOT EXISTS, so a retry costs nothing. No backfill.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. READ PATHS THAT CHANGE MEANING: none. One new empty table; no existing object is touched.
--      An empty table answers `not_configured` for every letter — which is exactly today's behaviour
--      (0006 books every letter with NULL deadlines).
--   5. RETENTION: a rule is THE BASIS OF ISSUED COMMITMENTS — when an inspection asks why a complaint
--      was due on a date, the answer is the row as it stood that day. So the soft-delete columns are
--      present from the start, a row is never hard-deleted (rule 7), and a removed row keeps its values.
-- ---------------------------------------------------------------------------

-- @entity: CitizenLetterDeadlineRule
-- @scope:  tenant
CREATE TABLE IF NOT EXISTS citizen_letter_deadline_rule (
    tenant_id      TEXT        NOT NULL,
    id             TEXT        NOT NULL,   -- ULID

    letter_type    TEXT        NOT NULL,
    deadline_kind  TEXT        NOT NULL,

    -- A COUNT IN `unit`. Read the two together, never one alone: "10" means nothing without its unit.
    amount         INTEGER     NOT NULL,
    unit           TEXT        NOT NULL,

    deleted_at     TIMESTAMPTZ,
    deleted_by     TEXT,
    delete_reason  TEXT,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- ONE LIVE RULE PER (commune, letter type, deadline kind) — the `sla.linh_vuc_khoa` shape (0008),
    -- for the same reason: a partial unique index on a partitioned table could not be verified here,
    -- and a migration that fails at startup stops the service. TRUE while the row is live, NULL once
    -- soft-deleted; NULLs never collide, so any number of removed rows may sit beside the live one.
    --
    -- A REMOVED RULE DOES NOT BLOCK A NEW ONE, as for `sla` and unlike the catalogues: a rule issues no
    -- code. The deadlines it produced are STORED on the letters (rule 10, invariant 2) and never
    -- recomputed from here, so a commune that replaces its rule changes policy from that moment on
    -- (ADR 0007 decision 6).
    live_key       BOOLEAN GENERATED ALWAYS AS (CASE WHEN deleted_at IS NULL THEN TRUE END) STORED,

    -- Composite with tenant_id (rule 1, invariant 6).
    PRIMARY KEY (tenant_id, id),
    CONSTRAINT citizen_letter_deadline_rule_live_unique
        UNIQUE (tenant_id, letter_type, deadline_kind, live_key),

    CONSTRAINT citizen_letter_deadline_rule_id_ulid CHECK (length(id) = 26),

    CONSTRAINT citizen_letter_deadline_rule_letter_type_valid
        CHECK (letter_type IN ('kien-nghi-phan-anh', 'khieu-nai', 'to-cao', 'de-nghi')),
    CONSTRAINT citizen_letter_deadline_rule_kind_valid
        CHECK (deadline_kind IN ('xu-ly-don', 'giai-quyet')),
    -- Only the two types that pass through `thu-ly` have a resolution deadline (see the header).
    CONSTRAINT citizen_letter_deadline_rule_resolution_type
        CHECK (deadline_kind <> 'giai-quyet' OR letter_type IN ('khieu-nai', 'to-cao')),

    -- ZERO IS A DEADLINE BREACHED AT THE INSTANT IT IS MADE; a negative one is a typo (0008
    -- `sla_gio_phai_duong`). No upper bound here: the contract's "over the ceiling" is a domain rule,
    -- and a bound in the schema would refuse a statutory figure nobody has verified yet.
    CONSTRAINT citizen_letter_deadline_rule_amount_positive CHECK (amount > 0),
    CONSTRAINT citizen_letter_deadline_rule_unit_valid
        CHECK (unit IN ('gio-lam-viec', 'ngay-lam-viec', 'ngay-lich')),

    -- Soft-delete columns are all three or none (rule 7, invariant 1); `deleted_by` is a staff
    -- business code (rule 6, invariant 8).
    CONSTRAINT citizen_letter_deadline_rule_soft_delete_complete
        CHECK ((deleted_at IS NULL AND deleted_by IS NULL AND delete_reason IS NULL)
            OR (deleted_at IS NOT NULL AND deleted_by IS NOT NULL AND btrim(deleted_by) <> ''
                AND delete_reason IS NOT NULL AND btrim(delete_reason) <> ''
                AND char_length(delete_reason) <= 500))
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS citizen_letter_deadline_rule_p%s PARTITION OF citizen_letter_deadline_rule '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

COMMENT ON TABLE citizen_letter_deadline_rule IS
    'Han don thu theo loai don cua xa (ADR 0084 #3, ADR 0085). Mot dong = (loai don, han nao, so, '
    'don vi). Khong co dong = xa chua cau hinh, don vao so khong han. Khong seed dong nao. Phep tinh '
    'thuoc ResolveCitizenLetterDeadline; khong lui ve dong don-thu cua bang sla.';
COMMENT ON COLUMN citizen_letter_deadline_rule.unit IS
    'Don vi cua amount. Loai don nao dung don vi nao kiem o tang domain, khong o CHECK: bang luat '
    'dinh chua duoc phap che doi chieu (ADR 0064).';

-- The one read: this commune's live rules, for one letter type and kind (ResolveCitizenLetterDeadline)
-- or all of them (the configuration screen). Starts with tenant_id (rule 1), drops removed rows
-- (rule 7, invariant 2).
CREATE INDEX IF NOT EXISTS citizen_letter_deadline_rule_list
    ON citizen_letter_deadline_rule (tenant_id, letter_type, deadline_kind)
    WHERE deleted_at IS NULL;

-- ---------------------------------------------------------------------------
-- REVERSAL (rule 7, invariant 4). By a person, in ONE transaction; core/migrate has no automatic
-- rollback (ADR 0013). Written as prose, not a runnable line, because a runnable line gets run.
--
--   WHILE NO COMMUNE HAS A RULE — check first that `SELECT count(*)` on the table returns 0; then
--   drop citizen_letter_deadline_rule (its 32 partitions and index go with it) and remove this file's
--   row from schema_migration. Exact; loses nothing.
--
--   ONCE A RULE EXISTS — no reversal. Deadlines stored on letters were computed from it; dropping it
--   destroys the only record of why a letter was due when it was due. Rule 7 stop condition #1: the
--   owner's decision with a verified backup, never a command.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- BACKSTOP, carried forward from 0002–0008: every partitioned table in this schema must have
-- partitions, or it rejects every INSERT.
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
