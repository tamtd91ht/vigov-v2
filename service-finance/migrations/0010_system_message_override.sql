-- 0010 — a commune's rewording of a system sentence this service ships (14-cau-hinh §7
-- "Lời hệ thống", the finance half: ADR 0024 §`loi_he_thong`).
--
-- WHAT IS DECIDED, AND BY WHOM (user decision 2026-09-28, following the recommendations):
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
-- WHY THE SPECIFICATION'S TABLE (`loi_he_thong(id, nhom, khoa, mo_ta, noi_dung, di_kem_phan_mem,
-- dang_dung)`) IS NOT THE SHAPE HERE: it stores the default, the description and an on/off flag per
-- row, which is a copy of the code's catalogue per commune (rule 9: two copies drift) plus a switch
-- the user decided not to offer. What a commune actually owns is ONE thing — its own wording — and
-- that is all this table holds. The name is English (rule 12) and says exactly that.
--
-- ONE TABLE PER OWNING SERVICE, NOT ONE SHARED TABLE. ADR 0024 splits the 39 keys by the service
-- that RAISES the sentence: `budget.scope_notice` is finance's, the six `feedback.*` keys are
-- petitions'. petitions gets its own `system_message_override` in its own schema (rule 2,
-- invariant 2: no service reads another's database).
--
-- @entity: SystemMessageOverride
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

    -- THE CLOSED SET OF KEYS THIS SERVICE RAISES, and it has to agree with
    -- domain.shippedMessages. A key outside it is a sentence no code in this service shows, so a
    -- row for it would be configuration that silently does nothing. Adding a key later is a new
    -- migration that widens this CHECK — the price of the database refusing what the code refuses.
    message_key    TEXT        NOT NULL,

    -- The commune's wording. Validated in internal/domain first (a sentence the caller can act on)
    -- and again here (the floor that holds against every writer): trimmed, 1..1000 characters, no
    -- control character (a line break inside a one-line banner, or an escape sequence in a log
    -- viewer), no `<` or `>` (the sentence is TEXT; web-admin and the Mini App render it as text,
    -- and refusing markup here means no renderer ever has to be the only defence — rule 13 inv 3).
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

    -- AT MOST ONE LIVE OVERRIDE PER KEY PER COMMUNE, WITHOUT A PARTIAL UNIQUE INDEX. The same device
    -- 0003 uses for `moc_mac_dinh`: the marker is the key on the live row and NULL on every
    -- soft-deleted one, and NULLs do not collide — so UNIQUE (tenant_id, live_key) admits one live
    -- row and any number of reverted ones beside it. `WHERE deleted_at IS NULL` on a unique index is
    -- what tools/check_khoa_duy_nhat.py refuses (rule 7, invariant 3), and nothing here is an issued
    -- code, but one device for one shape is cheaper than two.
    live_key       TEXT GENERATED ALWAYS AS
                       (CASE WHEN deleted_at IS NULL THEN message_key END) STORED,

    PRIMARY KEY (tenant_id, id),
    UNIQUE (tenant_id, live_key),

    CONSTRAINT system_message_override_key_known
        CHECK (message_key IN ('budget.scope_notice')),
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
-- HAS a row, dropping the table silently puts that commune back on the software's sentence and
-- destroys the history of its wording — that needs the user (rule 7, stop condition 2), not a
-- command. `audit_log` keeps every before/after text either way.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- BACKSTOP: every partitioned table in this schema must actually have partitions — repeated from
-- 0005 because it only verifies the state after a file that carries it.
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
