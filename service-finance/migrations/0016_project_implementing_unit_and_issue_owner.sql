-- 0016 — three nullable columns the giai-ngan screens of the prototype carry and this schema did not:
-- the project's free-text "Đơn vị thực hiện", and who follows an issue and by when.
--
-- AUTHORITY: owner instruction 07/10/2026 — "nếu đụng backend cho phép xây dựng backend luôn cho
-- đồng bộ, còn lại mọi việc đều phải tuân thủ prototype". The prototype backend is
-- ../vigov-require/apps/api/app/modules/budget/models.py:
--   :302-304  BudgetItem.implementing_unit  String(255), "free text because it is often a contractor
--             rather than a department of the commune"
--   :445-475  BudgetIssue.owner_user_id (FK users.id) and due_on (Date)
--
-- WHAT IS DELIBERATELY NOT TAKEN FROM THE PROTOTYPE:
--   * owner_user_id becomes `owner_code` — the STAFF BUSINESS CODE (CB-00123), never an internal id,
--     for rule 6 invariant 8's reason, and because GET /api/v1/staff-directory (the only staff list a
--     `budget.*` account can read) carries `code` and no id — 0015's reasoning for mentions.
--   * BudgetIssue.status / IssueStatus (models.py:61-66, 'open'|'resolving'|'resolved') — NOT added.
--     0015's resolution stays one-way (resolved_at/resolved_by, no reopen): the prototype UI has one
--     "Đã gỡ xong" button and never sets 'resolving'.
--   * BudgetIssue.kind — not part of this card; not added.
--
-- WHY A NEW FILE: 0001..0015 have been applied and core/migrate compares the checksum of every applied
-- file at startup (ErrChecksumLech).
--
-- RULE 1: tenant_id, both primary keys and the HASH (tenant_id) partitioning of `du_an` and
-- `project_issues` are untouched. No key and no index is added, so invariants 6/7 have nothing new to
-- satisfy. A column added to a partitioned parent is added to every partition by PostgreSQL itself.
--
-- RULE 3: `du_an.implementing_unit` is FREE TEXT and may name a company or, by a clerk's choice, a
-- person — the same exposure class as `chung_tu_giai_ngan.doi_tac` (0004). The Go code never logs it.
-- `project_issues.owner_code` is a staff business code, not personal data. `due_on` is a date.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. HOW MANY ROWS PER COMMUNE: none written. ADD COLUMN with no DEFAULT is catalogue-only — no row
--      is rewritten — and every existing row reads NULL, which every CHECK below admits. No backfill:
--      the prototype has no source the old rows could be filled from, and inventing one would be
--      deciding a business fact.
--   2. IF IT STOPS HALF-WAY: it cannot land half-applied — one file, one transaction (core/migrate),
--      with the progress row. Every statement is ADD COLUMN IF NOT EXISTS, a constraint guarded by a
--      lookup pinned to the PARENT table (`conrelid`, as 0008/0011/0012 do — partitions inherit a CHECK
--      under the same name, so `conname` alone matches 33 rows), or CREATE OR REPLACE. A retry costs
--      nothing.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. WHICH READ PATHS CHANGE MEANING WHILE IT IS HALF-APPLIED: none can see it half-applied (one
--      transaction). After it: NONE TODAY. The store reads and writes both tables through explicit
--      column lists that do not name the new columns (internal/store/du_an.go:69,
--      du_an_ghi.go:126/354/392; project_discussion.go:292-295 and `cotIssue`), so every existing read
--      returns what it did. ROLLING DEPLOY: an old replica's project UPDATE (du_an_ghi.go:392) names
--      its columns, so it leaves `implementing_unit` as a new replica wrote it — it does not blank it.
--      An old replica inserts issues with owner_code/due_on NULL ("nobody named, no date") — a true
--      state, not a corrupt one.
--   5. RETENTION: both tables are archival (rule 7). Nothing is dropped, retyped, emptied or
--      overwritten; no business code is issued or renumbered. The issue guard is REPLACED by a
--      strictly stronger one: every protection of 0015 is kept verbatim and the two new columns join
--      the frozen set.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- du_an.implementing_unit — "Đơn vị thực hiện" as typed (often a contractor), ALONGSIDE
-- don_vi_thuc_hien_id (0004), which is kept unchanged: one names a unit of the commune by id, this one
-- holds the words the prototype stores. 255 = the prototype's String(255). Blank is refused because
-- '' reads as "set" while saying nothing; "not named" is NULL.
-- ---------------------------------------------------------------------------
ALTER TABLE du_an ADD COLUMN IF NOT EXISTS implementing_unit TEXT;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint
                   WHERE conrelid = 'du_an'::regclass
                     AND conname = 'du_an_implementing_unit_valid') THEN
        ALTER TABLE du_an ADD CONSTRAINT du_an_implementing_unit_valid
            CHECK (implementing_unit IS NULL
                   OR (btrim(implementing_unit) <> '' AND char_length(implementing_unit) <= 255));
    END IF;
END $$;

-- ---------------------------------------------------------------------------
-- project_issues.owner_code / due_on — who follows the issue and by when (prototype
-- models.py:472-475). Both optional. SET AT INSERT ONLY: the prototype UI never edits them, and the
-- guard below freezes them after insert like the issue's text. owner_code is NOT validated against
-- identity here (a synchronous cross-service call); the write path must accept only a code of THIS
-- commune's directory, as 0015 says for mentions.
-- ---------------------------------------------------------------------------
ALTER TABLE project_issues ADD COLUMN IF NOT EXISTS owner_code TEXT;
ALTER TABLE project_issues ADD COLUMN IF NOT EXISTS due_on DATE;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint
                   WHERE conrelid = 'project_issues'::regclass
                     AND conname = 'project_issues_owner_code_not_blank') THEN
        ALTER TABLE project_issues ADD CONSTRAINT project_issues_owner_code_not_blank
            CHECK (owner_code IS NULL OR btrim(owner_code) <> '');
    END IF;
END $$;

-- project_issue_guard — 0015's function, REPLACED IN PLACE (same name, so the trigger
-- project_issues_guard created by 0015 picks it up without being recreated). The ONLY change is two
-- lines in the first IF: owner_code and due_on are frozen after insert. Inserts are not guarded (the
-- trigger is BEFORE UPDATE), which is exactly "settable at insert only". Resolve-once, link-task-once
-- and soft-delete-only are unchanged; hard DELETE stays refused by project_issues_no_hard_delete.
CREATE OR REPLACE FUNCTION project_issue_guard() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.tenant_id   IS DISTINCT FROM OLD.tenant_id
    OR NEW.project_id  IS DISTINCT FROM OLD.project_id
    OR NEW.title       IS DISTINCT FROM OLD.title
    OR NEW.description IS DISTINCT FROM OLD.description
    OR NEW.recorded_by IS DISTINCT FROM OLD.recorded_by
    OR NEW.recorded_at IS DISTINCT FROM OLD.recorded_at
    OR NEW.owner_code  IS DISTINCT FROM OLD.owner_code
    OR NEW.due_on      IS DISTINCT FROM OLD.due_on THEN
        RAISE EXCEPTION 'project_issues: a recorded issue cannot be edited'
            USING HINT = 'An issue is a record of what a member of staff reported, when. Record a '
                         'new issue instead of rewriting this one.';
    END IF;
    IF OLD.resolved_at IS NOT NULL
       AND (NEW.resolved_at IS DISTINCT FROM OLD.resolved_at
            OR NEW.resolved_by IS DISTINCT FROM OLD.resolved_by) THEN
        RAISE EXCEPTION 'project_issues: a resolution cannot be changed or withdrawn'
            USING HINT = 'Resolution is one-way (prototype: one "Đã gỡ xong" button, no reopen). '
                         'Record a new issue if the obstacle returns.';
    END IF;
    IF OLD.tracking_task_id IS NOT NULL
       AND NEW.tracking_task_id IS DISTINCT FROM OLD.tracking_task_id THEN
        RAISE EXCEPTION 'project_issues: the tracking task link is set once'
            USING HINT = 'The link points at the task raised for this issue; re-pointing it would '
                         'leave that task chasing an issue that no longer names it.';
    END IF;
    RETURN NEW;
END $$;

-- ---------------------------------------------------------------------------
-- NOT YET RUN ON A REAL POSTGRESQL when written (07/10/2026, VIGOV_TEST_DSN absent):
-- project_implementing_unit_test.go proves the SQL is WRITTEN, not that PostgreSQL accepts it.
--
-- REVERSAL (migration question 3). Run by a person, in ONE transaction, AFTER A VERIFIED BACKUP of
-- `du_an` and `project_issues` once any new column holds a value (they are archival — dropping a
-- populated column destroys a record and is the user's decision, never an agent's); core/migrate has
-- no automatic rollback (ADR 0013).
--
--   1. Roll back the Go code that reads and writes the new columns first.
--   2. Restore 0015's guard FIRST (copy its CREATE OR REPLACE FUNCTION project_issue_guard verbatim):
--      the version above names owner_code/due_on and would fail on every UPDATE once they are gone.
--   3. ALTER TABLE project_issues DROP CONSTRAINT project_issues_owner_code_not_blank;
--      ALTER TABLE project_issues DROP COLUMN due_on, DROP COLUMN owner_code;
--      ALTER TABLE du_an DROP CONSTRAINT du_an_implementing_unit_valid;
--      ALTER TABLE du_an DROP COLUMN implementing_unit;
--   4. Remove this file's row from `schema_migration`, otherwise the runner still believes it has run.
-- ---------------------------------------------------------------------------
