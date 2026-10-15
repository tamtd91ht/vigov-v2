-- 0042 — `petition_staff_recorded_rating`: the star rating a citizen GAVE BY PHONE OR AT THE COUNTER, as
-- recorded by a member of staff. Schema only: no row is written, no existing value changes.
--
-- WHY IT EXISTS: the owner rule of 11/10/2026 ("chuẩn là prototype — tính năng trên prototype phải được
-- đảm bảo") and the customer bug sheet row 60 bring back the prototype's block "Ghi nhận đánh giá của
-- người dân" (vigov-require FeedbackDetailDrawer.tsx:516-540, POST /{feedback_id}/rating behind
-- `feedback.resolve`, router.py:391-405). That REVERSES ADR 0062 decisions 1 and 3 (staff never record a
-- rating; no such block). ADR 0062 decision 2 — every satisfaction figure counts ONLY the citizen's own
-- verdict — is KEPT, and this table is how.
--
-- # A TABLE OF ITS OWN, AND NOT A `source` COLUMN BESIDE `diem_hai_long` — the decision of this file
--
-- `diem_hai_long` is read by every satisfaction figure and filter this service has (citizen_report_summary
-- `rating_sample` / `low_rating`, citizen_report_figures' rating sum, the register's `rating_max`, the
-- automation's low-rating digest) and by the citizen's reopen rule. A second writer of that column would
-- need EVERY one of those readers, and every reader written later, to remember `AND source = 'citizen'`;
-- the one that forgets puts a staff-typed 5★ into a figure that goes upward, silently, with every test
-- green. Kept apart, NO reader of `diem_hai_long` can see a staff record at all — the two sources cannot
-- mix by construction. The `source` column below is still written, constrained to 'staff_recorded', so any
-- future UNION of the two is self-labelling.
--
-- # THE RULES (the use case enforces those that need the petition row; this file the rest)
--
--   * ONE staff record per petition — UNIQUE (tenant_id, petition_id). The prototype hides the block once
--     a rating exists; a second staff record would be staff overwriting what they themselves wrote.
--   * REFUSED once the citizen has rated (use case, under the petition's row lock — 409). The citizen's
--     own rating is authoritative.
--   * If staff recorded and the citizen rates LATER, the citizen's rating goes into `diem_hai_long` as it
--     always did (app.RatePetition is untouched) and this row stays as HISTORY. Nothing here is updated.
--   * A staff record NEVER reopens the petition and owes the citizen no message. The prototype's hint says
--     "Một đến hai sao sẽ tự mở lại phiếu"; not adopted here — a reopen is a petition-lifecycle
--     transition with a citizen notification (rule 10 stop conditions #2/#3, ADR 0041), and ADR 0062's
--     own reason applies to it: "phiếu mở lại vì 1–2 sao vẫn chỉ đến từ người dân". Reported for the
--     owner, not decided.
--
-- # NAMES (rule 12, ADR 0051): English table and columns. Enum VALUES per ADR 0011 are Vietnamese
-- kebab-case — `dien-thoai`, `tai-quay`, the two channels the prototype's hint names ("qua điện thoại hoặc
-- tại quầy"). `staff_recorded` is the value the main session fixed for `source`.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. ROWS PER COMMUNE: zero written. A new, empty table.
--   2. IF IT STOPS HALF-WAY: it cannot land half-applied — one file, one transaction (core/migrate), with
--      its progress row. CREATE … IF NOT EXISTS, CREATE OR REPLACE FUNCTION and DROP TRIGGER IF EXISTS
--      before CREATE TRIGGER make a retry cost nothing. No per-commune loop: nothing to backfill.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. READ PATHS THAT CHANGE MEANING: none. No existing statement reads this table; every satisfaction
--      figure keeps reading `diem_hai_long` only, which this file does not touch.
--   5. RETENTION: purely additive. The rows are records of acts on an archival record — append-only.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- @entity: PetitionStaffRecordedRating
-- @scope:  tenant
--
-- APPEND-ONLY, ENFORCED BY A TRIGGER (rule 7, forbidden #5), so NO SOFT-DELETE COLUMNS — the shape of
-- petition_merge_event (0037). audit_log is written for the same act in the same transaction (rule 6).
--
-- NO FOREIGN KEY to phieu_phan_anh, for 0013's reason: a historical entry must survive any future
-- reshaping of the register, and the write path holds the petition under row lock in the same transaction.
-- Petitions are never hard-deleted (0004).
--
-- `note` MAY HOLD PERSONAL DATA (what the citizen said, and how — staff free text). Never logged, never in
-- an error, never in the audit delta (its length is), never on an event, never shown to a citizen.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS petition_staff_recorded_rating (
    tenant_id    TEXT        NOT NULL,
    id           TEXT        NOT NULL,

    -- The petition's INTERNAL id (phieu_phan_anh.id), same commune.
    petition_id  TEXT        NOT NULL,

    stars        INTEGER     NOT NULL,

    -- How the citizen gave it: `dien-thoai` (by phone) or `tai-quay` (at the counter).
    channel      TEXT        NOT NULL,

    -- OPTIONAL: what the citizen said, and how. NULL = none; never blank.
    note         TEXT,

    -- Always 'staff_recorded'. Written explicitly by the use case; the CHECK makes any other value
    -- impossible, so a row here can never claim to be the citizen's own verdict.
    source       TEXT        NOT NULL DEFAULT 'staff_recorded',

    -- Staff BUSINESS CODE (`CB-00123`, rule 6 invariant 8). No fallback: blank is refused.
    recorded_by  TEXT        NOT NULL,
    recorded_at  TIMESTAMPTZ NOT NULL,

    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- COMPOSITE WITH tenant_id (rule 1, invariant 6), both keys.
    PRIMARY KEY (tenant_id, id),
    CONSTRAINT petition_staff_recorded_rating_once UNIQUE (tenant_id, petition_id),

    -- 1..5, migration 0004's `phieu_phan_anh_diem_hai_long_hop_le` range.
    CONSTRAINT petition_staff_recorded_rating_stars_valid CHECK (stars BETWEEN 1 AND 5),
    CONSTRAINT petition_staff_recorded_rating_channel_valid CHECK (channel IN ('dien-thoai', 'tai-quay')),
    CONSTRAINT petition_staff_recorded_rating_source_valid CHECK (source = 'staff_recorded'),
    -- 1000 = domain.RatingCommentMaxLen, the bound on the citizen's own comment.
    CONSTRAINT petition_staff_recorded_rating_note_valid
        CHECK (note IS NULL OR (btrim(note) <> '' AND char_length(note) <= 1000)),
    -- 64 = domain.CanBoToiDa, the bound this service already puts on a staff business code.
    CONSTRAINT petition_staff_recorded_rating_recorded_by_valid
        CHECK (btrim(recorded_by) <> '' AND char_length(recorded_by) <= 64),
    CONSTRAINT petition_staff_recorded_rating_petition_present CHECK (btrim(petition_id) <> '')
);

-- No secondary index: the UNIQUE key (tenant_id, petition_id) is exactly the drawer's read and the use
-- case's "already recorded?" check.

-- Append-only guard: 0013's / 0037's refusals. Row-level for UPDATE/DELETE, statement-level for TRUNCATE,
-- both on the table itself — it is a plain table (ADR 0096), so there are no partitions to cover one by one.
CREATE OR REPLACE FUNCTION petition_staff_recorded_rating_append_only() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'petition_staff_recorded_rating is append-only: % on % refused', TG_OP, TG_TABLE_NAME
        USING HINT = 'A recorded rating is a historical record (rule 7, forbidden #5): never modified, '
                     'never removed, never truncated. The citizen''s own later rating goes to the '
                     'petition row and leaves this row as history.';
END $$;

DROP TRIGGER IF EXISTS petition_staff_recorded_rating_no_update_delete ON petition_staff_recorded_rating;
CREATE TRIGGER petition_staff_recorded_rating_no_update_delete
    BEFORE UPDATE OR DELETE ON petition_staff_recorded_rating
    FOR EACH ROW EXECUTE FUNCTION petition_staff_recorded_rating_append_only();

DROP TRIGGER IF EXISTS petition_staff_recorded_rating_no_truncate ON petition_staff_recorded_rating;
CREATE TRIGGER petition_staff_recorded_rating_no_truncate
    BEFORE TRUNCATE ON petition_staff_recorded_rating
    FOR EACH STATEMENT EXECUTE FUNCTION petition_staff_recorded_rating_append_only();

COMMENT ON TABLE petition_staff_recorded_rating IS
    'Owner rule 11/10/2026 (ADR 0062 decisions 1/3 reversed): a rating the citizen gave by phone or at the '
    'counter, recorded by staff. NEVER counted in a satisfaction figure (those read only the citizen''s '
    'own verdict on the petition row, which stays authoritative). Append-only history.';

-- ---------------------------------------------------------------------------
-- WHAT THIS FILE DOES NOT STOP, said plainly: DDL by the table owner (dropping constraints, DISABLE
-- TRIGGER) — ADR 0013's line. Nor a row for a petition the citizen had already rated: that needs the
-- petition row, and the use case refuses it under that row's lock.
--
-- REVERSAL (rule 7, invariant 4). Run by a person, in ONE transaction; core/migrate has no automatic
-- rollback (ADR 0013). Lossless ONLY WHILE NOTHING HAS BEEN RECORDED:
--
--   SELECT count(*) FROM petition_staff_recorded_rating;   -- must be 0
--
-- ZERO → in this order:
--   DROP TABLE petition_staff_recorded_rating;             (its triggers go with it)
--   DROP FUNCTION petition_staff_recorded_rating_append_only();
-- and remove this file's row from `schema_migration`.
--
-- NON-ZERO → the rows are records of acts on archival records (and audit_log entries point at them).
-- Rule 7 stop condition #2: a user decision plus a verified backup, never a command.
-- ---------------------------------------------------------------------------

-- BACKSTOP: every partitioned table in this schema must actually have partitions (0002, §BACKSTOP).
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
