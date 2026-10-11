-- 0010 — `citizen_letter_type`: the commune's LABEL TIER of "Loại đơn thư" (ADR 0039 §Hệ quả cho ADR
-- 0024 §2; bug sheet row 33, owner decision 10/10/2026 "theo prototype" — LookupTable.tsx:37).
--
-- ONE ROW = the commune's label / order / colour / on-off for ONE of C4's four statutory letter types.
-- The `code` CHECK below is the same closed set as `citizen_letter.type` (0006): a fifth code would be a
-- label no letter can be booked under. The app refuses it first, in Vietnamese
-- (domain.ErrLetterTypeCodeNotStatutory); this CHECK is the floor.
--
-- SHIPS EMPTY. Sowing a commune's first rows is commune onboarding, which does not exist here
-- (0003's header says the same for `loai_van_ban`).
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. ROWS PER COMMUNE: at most four live rows — the unique key and the CHECK allow no more.
--   2. IF IT STOPS HALF-WAY: it cannot — one file, one transaction with its progress row; every
--      statement is IF NOT EXISTS / guarded, so a retry costs nothing.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. READ PATHS THAT CHANGE MEANING: none. A new table, read only by GET /api/v1/citizen-letter-types.
--      The register keeps its own fixed labels.
--   5. RETENTION: nothing removed. Purely additive.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- @entity: CitizenLetterType
-- @scope:  tenant
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS citizen_letter_type (
    tenant_id     TEXT        NOT NULL,
    id            TEXT        NOT NULL,
    code          TEXT        NOT NULL,
    label         TEXT        NOT NULL,
    sort_order    INT         NOT NULL DEFAULT 0,
    active        BOOLEAN     NOT NULL DEFAULT true,
    color         TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at    TIMESTAMPTZ,
    deleted_by    TEXT,
    delete_reason TEXT,
    PRIMARY KEY (tenant_id, id),
    -- COUNTS SOFT-DELETED ROWS TOO (rule 7, invariant 3; tools/check_khoa_duy_nhat.py): a code once
    -- given a row in this commune is not given a second one. Cost, stated: a commune that soft-deletes
    -- the row of `khieu-nai` cannot add it back — it should switch the row off (`active`) instead.
    UNIQUE (tenant_id, code),
    CONSTRAINT citizen_letter_type_code_statutory
        CHECK (code IN ('kien-nghi-phan-anh', 'khieu-nai', 'to-cao', 'de-nghi')),
    CONSTRAINT citizen_letter_type_label_not_blank CHECK (btrim(label) <> ''),
    CONSTRAINT citizen_letter_type_label_max CHECK (char_length(label) <= 200),
    CONSTRAINT citizen_letter_type_order_range CHECK (sort_order BETWEEN 0 AND 9999),
    CONSTRAINT citizen_letter_type_color_shape CHECK (color ~ '^#[0-9A-Fa-f]{6}$'),
    -- SOFT DELETE IS ALL OR NONE (rule 7, invariant 1). 500 = domain.LyDoXoaToiDa.
    CONSTRAINT citizen_letter_type_soft_delete_complete CHECK (
        (deleted_at IS NULL AND deleted_by IS NULL AND delete_reason IS NULL)
        OR (deleted_at IS NOT NULL
            AND deleted_by IS NOT NULL AND btrim(deleted_by) <> ''
            AND delete_reason IS NOT NULL AND btrim(delete_reason) <> '')),
    CONSTRAINT citizen_letter_type_delete_reason_max CHECK (char_length(delete_reason) <= 500)
) PARTITION BY HASH (tenant_id);

-- 32 partitions, the modulus every partitioned table of this service uses (ADR 0010).
DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS citizen_letter_type_p%s PARTITION OF citizen_letter_type '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

-- The one read: the commune's live rows in its order (rule 7, invariant 2 — partial on deleted_at).
CREATE INDEX IF NOT EXISTS citizen_letter_type_list
    ON citizen_letter_type (tenant_id, sort_order, code) WHERE deleted_at IS NULL;

-- No hard delete (rule 7, forbidden #1) — the function 0004 defines for every archival table here.
DROP TRIGGER IF EXISTS citizen_letter_type_no_hard_delete ON citizen_letter_type;
CREATE TRIGGER citizen_letter_type_no_hard_delete
    BEFORE DELETE ON citizen_letter_type
    FOR EACH ROW EXECUTE FUNCTION ho_so_luu_tru_cam_xoa_cung();

-- ---------------------------------------------------------------------------
-- REVERSAL (rule 7, invariant 4). By a person, in ONE transaction, then remove this file's row from
-- the migration progress table (core/migrate has no automatic rollback, ADR 0013):
--   DROP TABLE citizen_letter_type;      -- its 32 partitions, index and trigger go with it
-- Lossless only while
--   SELECT count(*) FROM citizen_letter_type;   -- must be 0
-- otherwise it discards the labels communes chose — a user decision (rule 7, stop condition 2).
-- ---------------------------------------------------------------------------
