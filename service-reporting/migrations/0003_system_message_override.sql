-- 0003 — a commune's rewording of a system sentence this service ships (14-cau-hinh §7
-- "Lời hệ thống", the reporting half: ADR 0024 §Phụ, *Bổ sung 29/09/2026*).
--
-- A COPY OF service-finance's 0010 AND service-petitions' 0020, NOT A SHARED TABLE. ADR 0024 splits
-- the keys by the service that RAISES the sentence: the 38 `report.*` keys are this service's,
-- because the export of `/bao-cao` and the report notifications that print them are this service's
-- (user decision 29/09/2026). Each service owns its own `system_message_override` in its own schema
-- (rule 2, invariant 2: no service reads another's database), and the three CHECKs name disjoint
-- key sets.
--
-- WHAT IS DECIDED, AND BY WHOM (user decision 2026-09-28, the same one finance's 0010 records):
--
--   * A commune may REWORD a shipped sentence. It may not remove one and may not invent one: a key
--     no code raises is a key nothing will ever show, so there is no `+ Thêm câu mới`.
--   * NO SEED ROWS. The shipped default lives IN CODE (internal/domain/system_message.go). A row
--     exists here only for a commune that has reworded that key. An empty table is the correct
--     state of every commune today, and it means "every commune reads the software's sentence".
--   * "Khôi phục câu mặc định" (not "Tắt") takes the commune back to the default. It is a SOFT
--     DELETE of the live override (rule 7, invariant 1), so every wording a commune ever used stays
--     on disk beside the append-only audit entry that recorded the change.
--
-- THE COST ADR 0024 NAMES, PAID HERE: this is the first table in `reporting` that holds data a
-- commune edits — `reporting` is no longer a pure read model. The cost stops at wording: it still
-- owns no source business record.
--
-- THE ENTITY NAME CARRIES THE SERVICE because finance's 0010 already declares
-- `SystemMessageOverride` and petitions' 0020 `PetitionsSystemMessageOverride`; tools/kb reports one
-- entity with two owners (rule 2, invariant 1). The table name is the same in all three schemas.
--
-- @entity: ReportingSystemMessageOverride
-- @scope:  tenant
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. ROWS PER COMMUNE: zero. The table is new and nothing is seeded, deliberately (see above).
--   2. IF IT STOPS HALF-WAY: it cannot land half-applied — one file, one transaction, progress row
--      inside it. Every statement is IF NOT EXISTS, so a retry costs nothing.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. READ PATHS THAT CHANGE MEANING WHILE HALF-APPLIED: none. No existing table is touched.
--   5. COST ON THE LARGEST COMMUNE: nothing. 32 empty partitions.
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS system_message_override (
    tenant_id      TEXT        NOT NULL,
    id             TEXT        NOT NULL,

    -- THE CLOSED SET OF KEYS THIS SERVICE RAISES, and it has to agree with domain.shippedMessages
    -- (pinned by TestCatalogueAgreesWithMigrationCheck, in the same order). A key outside it is a
    -- sentence no code in this service shows, so a row for it would be configuration that silently
    -- does nothing. Adding a key later is a new migration that widens this CHECK.
    message_key    TEXT        NOT NULL,

    -- The commune's wording. Validated in internal/domain first (a sentence the caller can act on)
    -- and again here (the floor that holds against every writer): trimmed, 1..1000 characters, no
    -- control character, no `<` or `>` (the sentence is TEXT; it is printed on an exported report and
    -- rendered by web-admin as text, and refusing markup here means no renderer is ever the only
    -- defence — rule 13 inv 3).
    message_text   TEXT        NOT NULL,

    created_at     TIMESTAMPTZ NOT NULL,
    -- Who: the staff BUSINESS CODE (`CB-00123`), never the internal id — the same value
    -- audit_log.actor_id holds (rule 6, invariant 8).
    created_by     TEXT        NOT NULL,
    updated_at     TIMESTAMPTZ NOT NULL,
    updated_by     TEXT        NOT NULL,

    -- Rule 7, invariant 1. Written only by "Khôi phục câu mặc định".
    deleted_at     TIMESTAMPTZ,
    deleted_by     TEXT,
    delete_reason  TEXT,

    -- AT MOST ONE LIVE OVERRIDE PER KEY PER COMMUNE, WITHOUT A PARTIAL UNIQUE INDEX — the device
    -- finance's 0010 and petitions' 0020 use: the marker is the key on the live row and NULL on every
    -- soft-deleted one, and NULLs do not collide. `WHERE deleted_at IS NULL` on a unique index is
    -- what tools/check_khoa_duy_nhat.py refuses (rule 7, invariant 3).
    live_key       TEXT GENERATED ALWAYS AS
                       (CASE WHEN deleted_at IS NULL THEN message_key END) STORED,

    PRIMARY KEY (tenant_id, id),
    UNIQUE (tenant_id, live_key),

    CONSTRAINT system_message_override_key_known
        CHECK (message_key IN ('report.title', 'report.block.tasks', 'report.block.register', 'report.block.budget', 'report.block.feedback', 'report.block.economy', 'report.block.alerts', 'report.block.ranking', 'report.block.fiscal', 'report.metric.tasks.open', 'report.metric.tasks.overdue', 'report.metric.tasks.done', 'report.metric.tasks.on_time_percent', 'report.metric.register.arrived', 'report.metric.register.open', 'report.metric.register.overdue', 'report.metric.register.petition_arrived', 'report.metric.budget.disbursed_percent', 'report.metric.budget.time_percent', 'report.metric.budget.behind', 'report.metric.budget.open_issues', 'report.metric.budget.disbursed_amount', 'report.metric.feedback.received', 'report.metric.feedback.open', 'report.metric.feedback.on_time_percent', 'report.metric.feedback.late', 'report.metric.feedback.rating', 'report.metric.economy.enterprises', 'report.metric.economy.household_businesses', 'report.metric.economy.new_in_period', 'report.metric.economy.total', 'report.metric.fiscal.revenue_percent', 'report.metric.fiscal.revenue_amount', 'report.metric.fiscal.expense_percent', 'report.metric.fiscal.expense_amount', 'report.metric.fiscal.balance', 'report.notification.week', 'report.notification.month')),
    CONSTRAINT system_message_override_text_shape
        CHECK (message_text = btrim(message_text)
               AND char_length(message_text) BETWEEN 1 AND 1000
               AND message_text !~ '[[:cntrl:]]'
               AND message_text !~ '[<>]'),
    -- A revert carries all three of its facts, or it did not happen.
    CONSTRAINT system_message_override_delete_complete
        CHECK (deleted_at IS NULL
               OR (deleted_by IS NOT NULL AND btrim(delete_reason) <> ''))
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS system_message_override_p%s PARTITION OF system_message_override '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

-- ---------------------------------------------------------------------------
-- REVERSAL. Lossless while no commune has reworded anything: DROP TABLE system_message_override
-- (the partitions go with it) and remove this file's row from `schema_migration`. Once a commune
-- HAS a row, dropping the table silently puts that commune back on the software's sentence — on a
-- report its leadership signs — and destroys the history of its wording. That needs the user
-- (rule 7, stop condition 2), not a command. `audit_log` keeps every before/after text either way.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- BACKSTOP: every partitioned table in this schema must actually have partitions (0002, §BACKSTOP)
-- — repeated because it only verifies the state after a file that carries it.
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
