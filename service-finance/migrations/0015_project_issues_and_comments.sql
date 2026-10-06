-- 0015 — §8.1 "Vướng mắc" and §8.4 "Trao đổi" of the project detail screen (docs/ui-ux/06-giai-ngan.md
-- §8.1, §8.4, §11 `vuong_mac`, §13 rule 4). User decision 06/10/2026: issues and discussion live in
-- finance NOW; the tracking task §13 rule 4 asks for (owned by petitions) and the @mention
-- notification (owned by comms) come later, through events. This file only makes room for them.
--
-- WHY A NEW FILE: 0001..0014 have been applied and core/migrate compares the checksum of every
-- applied file at startup (ErrChecksumLech).
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. HOW MANY ROWS PER COMMUNE: zero now. A project carries a handful of issues and a few dozen
--      comments over its life (§14: 63 projects a year), so thousands per commune per year.
--   2. IF IT STOPS HALF-WAY: one file, one transaction (core/migrate). Every statement is IF NOT
--      EXISTS / CREATE OR REPLACE / DROP TRIGGER IF EXISTS, so a retry costs nothing. No backfill.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. WHICH READ PATHS CHANGE MEANING: none. Two new tables; nothing existing reads them until the
--      Go code of the same release does. ROLLING DEPLOY: an old replica never touches them.
--   5. RETENTION: archival, like the project they hang off — soft delete only, hard DELETE refused
--      by ho_so_luu_tru_cam_xoa_cung (0004).
--
-- PERSONAL DATA (rule 3): both tables hold FREE TEXT typed by staff. It is meant to be about a public
-- works project, but nothing can stop a clerk typing a household's name into it, so the Go code never
-- logs it and the audit delta never copies it (the row itself is immutable — see the triggers — so
-- the row IS the record of what was written).
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- @entity: ProjectIssue
-- @scope:  tenant
--
-- project_issues — one obstacle recorded against one investment project (§8.1, §11 `vuong_mac`).
--
-- TITLE + DESCRIPTION, NOT ONE `noi_dung`: §11 names one text column, the prototype stores the first
-- line as the title and the rest as the description (IssueForm.tsx:23-44) and the list's "Vướng mắc
-- mới nhất" column (§7.2) shows the title only. The split is done by the server (domain
-- .SplitIssueText), so every client splits the same way.
--
-- du_an_id REFERENCES du_an(tenant_id, id) LOGICALLY, with no foreign key — 0004's reason (no
-- PostgreSQL in the build environment to verify an FK between two HASH-partitioned tables). The write
-- path reads the live project in the same transaction before inserting, and every read joins on
-- (tenant_id, du_an_id), so an orphan is invisible rather than misattributed.
--
-- RESOLUTION IS ONE-WAY. The prototype has a single "Đã gỡ xong" button and no reopen
-- (BudgetItemDetail.tsx:593-595, 693-731): a resolved issue stays in the timeline, struck through.
-- `resolved_at` and `resolved_by` are set together, once; the trigger below refuses clearing them.
--
-- tracking_task_id IS NULLABLE AND NOTHING WRITES IT YET. §13 rule 4: an issue on a project with an
-- officer in charge raises a tracking task (petitions owns tasks). That arrives later through an event
-- (user decision 06/10/2026); the column is created now because adding it then would be a migration
-- on a populated table. It may move from NULL to a value once, never back and never to another value.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS project_issues (
    tenant_id         TEXT        NOT NULL,
    id                TEXT        NOT NULL,
    project_id        TEXT        NOT NULL,
    title             TEXT        NOT NULL,
    description       TEXT,
    -- The STAFF BUSINESS CODE (CB-00123), never the internal id — rule 6, invariant 8's reason: this
    -- column is read years later by somebody with no lookup still alive.
    recorded_by       TEXT        NOT NULL,
    recorded_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    resolved_at       TIMESTAMPTZ,
    resolved_by       TEXT,
    tracking_task_id  TEXT,
    deleted_at        TIMESTAMPTZ,
    deleted_by        TEXT,
    delete_reason     TEXT,
    PRIMARY KEY (tenant_id, id),
    CONSTRAINT project_issues_title_present CHECK (btrim(title) <> ''),
    CONSTRAINT project_issues_title_length CHECK (char_length(title) <= 500),
    CONSTRAINT project_issues_description_length
        CHECK (description IS NULL OR char_length(description) <= 4000),
    -- Who and when are one fact: a resolution with no author is one nobody can account for.
    CONSTRAINT project_issues_resolution_whole
        CHECK ((resolved_at IS NULL) = (resolved_by IS NULL))
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS project_issues_p%s PARTITION OF project_issues '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

-- §8.1's timeline (one project, newest first) and §7.2's "latest issue" per project.
CREATE INDEX IF NOT EXISTS project_issues_by_project
    ON project_issues (tenant_id, project_id, recorded_at DESC, id DESC) WHERE deleted_at IS NULL;

DROP TRIGGER IF EXISTS project_issues_no_hard_delete ON project_issues;
CREATE TRIGGER project_issues_no_hard_delete
    BEFORE DELETE ON project_issues
    FOR EACH ROW EXECUTE FUNCTION ho_so_luu_tru_cam_xoa_cung();

-- project_issue_guard — what was recorded stays recorded (rule 7, forbidden #5: no silent edits).
--
-- There is no edit route; this is the floor under that absence, for a psql session or the next
-- service to connect. The allowed moves are exactly: resolve once, link the tracking task once, soft
-- delete.
CREATE OR REPLACE FUNCTION project_issue_guard() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.tenant_id   IS DISTINCT FROM OLD.tenant_id
    OR NEW.project_id  IS DISTINCT FROM OLD.project_id
    OR NEW.title       IS DISTINCT FROM OLD.title
    OR NEW.description IS DISTINCT FROM OLD.description
    OR NEW.recorded_by IS DISTINCT FROM OLD.recorded_by
    OR NEW.recorded_at IS DISTINCT FROM OLD.recorded_at THEN
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

DROP TRIGGER IF EXISTS project_issues_guard ON project_issues;
CREATE TRIGGER project_issues_guard
    BEFORE UPDATE ON project_issues
    FOR EACH ROW EXECUTE FUNCTION project_issue_guard();

-- ---------------------------------------------------------------------------
-- @entity: ProjectComment
-- @scope:  tenant
--
-- project_comments — one message of §8.4's free discussion between staff about one project.
--
-- MENTIONS ARE A JSONB ARRAY OF STAFF BUSINESS CODES (CB-…), NOT A CHILD TABLE. A mention is part of the
-- message as written: it is never edited, never queried on its own in this service, and the only
-- consumer that will ever look inside it (the comms notification, later) receives it in an event.
-- A child table would be a second archival table with its own soft delete for a value that cannot
-- change. JSONB rather than TEXT[]: database/sql cannot scan a PostgreSQL array without a
-- driver-specific type, and `chung_tu_giai_ngan.tep_dinh_kem` (0004) is the precedent.
--
-- Codes, not internal ids: the only staff list a `budget.*` account can read is
-- GET /api/v1/staff-directory, which carries `code` and no id, so the code is what the picker can send;
-- it is also what `recorded_by` / `author_code` and the projects' `assignee_id` hold. Not personal
-- data. NOT VALIDATED AGAINST identity here (that would be a synchronous cross-service call — not this
-- round); the notification follow-up must resolve each code to an ACTIVE account of THIS commune
-- (comms, through identity) and skip any code that resolves to nobody before sending anything.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS project_comments (
    tenant_id            TEXT        NOT NULL,
    id                   TEXT        NOT NULL,
    project_id           TEXT        NOT NULL,
    body                 TEXT        NOT NULL,
    -- The STAFF BUSINESS CODE, as project_issues.recorded_by.
    author_code          TEXT        NOT NULL,
    mentioned_staff_codes  JSONB       NOT NULL DEFAULT '[]'::jsonb,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at           TIMESTAMPTZ,
    deleted_by           TEXT,
    delete_reason        TEXT,
    PRIMARY KEY (tenant_id, id),
    CONSTRAINT project_comments_body_present CHECK (btrim(body) <> ''),
    CONSTRAINT project_comments_body_length CHECK (char_length(body) <= 4000),
    CONSTRAINT project_comments_mentions_array
        CHECK (jsonb_typeof(mentioned_staff_codes) = 'array' AND jsonb_array_length(mentioned_staff_codes) <= 20)
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS project_comments_p%s PARTITION OF project_comments '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

-- §8.4's thread: one project, oldest first.
CREATE INDEX IF NOT EXISTS project_comments_by_project
    ON project_comments (tenant_id, project_id, created_at, id) WHERE deleted_at IS NULL;

DROP TRIGGER IF EXISTS project_comments_no_hard_delete ON project_comments;
CREATE TRIGGER project_comments_no_hard_delete
    BEFORE DELETE ON project_comments
    FOR EACH ROW EXECUTE FUNCTION ho_so_luu_tru_cam_xoa_cung();

-- project_comment_guard — a message, once posted, is not rewritten (rule 7, forbidden #5). Only the
-- soft-delete columns may move.
CREATE OR REPLACE FUNCTION project_comment_guard() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.tenant_id           IS DISTINCT FROM OLD.tenant_id
    OR NEW.project_id          IS DISTINCT FROM OLD.project_id
    OR NEW.body                IS DISTINCT FROM OLD.body
    OR NEW.author_code         IS DISTINCT FROM OLD.author_code
    OR NEW.mentioned_staff_codes IS DISTINCT FROM OLD.mentioned_staff_codes
    OR NEW.created_at          IS DISTINCT FROM OLD.created_at THEN
        RAISE EXCEPTION 'project_comments: a posted comment cannot be edited'
            USING HINT = 'A discussion is a record of who said what, when. Post a new message.';
    END IF;
    RETURN NEW;
END $$;

DROP TRIGGER IF EXISTS project_comments_guard ON project_comments;
CREATE TRIGGER project_comments_guard
    BEFORE UPDATE ON project_comments
    FOR EACH ROW EXECUTE FUNCTION project_comment_guard();

-- ---------------------------------------------------------------------------
-- REVERSAL (migration question 3). Run by a person, in ONE transaction, AFTER A VERIFIED BACKUP of
-- both tables once they hold rows (they are archival); core/migrate has no automatic rollback
-- (ADR 0013).
--
--   1. Roll back the Go code that reads and writes them first.
--   2. DROP TABLE project_comments; DROP TABLE project_issues;   (partitions and triggers go with them)
--      DROP FUNCTION project_comment_guard(); DROP FUNCTION project_issue_guard();
--      ho_so_luu_tru_cam_xoa_cung STAYS — it is 0004's and other tables are attached to it.
--   3. Remove this file's row from `schema_migration`, otherwise the runner still believes it has run.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- BACKSTOP: every partitioned table in this schema must actually have partitions (see 0008).
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
