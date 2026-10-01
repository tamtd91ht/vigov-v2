-- 0026 — `stored_file` may hold a CITIZEN's scene photo on a petition (menu phan-anh-nguoi-dan,
-- "ảnh hiện trường khi gửi phản ánh"; ADR 0052 §12, ADR 0047 G3). Schema only: no row is written.
--
-- Owner decisions of 02/10/2026: optional, AT MOST 5 per petition, images only, re-encoded by the
-- server with EXIF stripped (ADR 0052 §1a step c, ADR 0047 G3), PRIVATE bucket and never public,
-- read through short signed links bound to the sender and the commune (rule 4 invariant 7),
-- retention class `citizen-media`. Uploaded by the CITIZEN, through a citizen session, AFTER the
-- petition exists; staff read them. Staff "after" photos (G8) are deferred — nothing here
-- precludes them (see WHAT G8 STILL HAS below).
--
-- NO NEW TABLE, so no ownership mark here: the rows stay entity PetitionsStoredFile, declared by
-- 0021 above its CREATE TABLE (tools/kb/ownership.go requires a mark to own the CREATE TABLE that
-- follows it, so a second mark here would be an error, not a note).
--
-- ---------------------------------------------------------------------------
-- WHO UPLOADED A CITIZEN PHOTO — `uploaded_by` = 'cong-dan', and nothing else.
--
-- THIS IS THE REPOSITORY'S EXISTING CONVENTION, not a new one. A business row a citizen causes
-- carries the FIXED MARKER domain.CitizenLogActor (internal/domain/nhat_ky_phan_anh.go:59-68) in its
-- "who" column — `nhat_ky_phan_anh.nguoi_ma` on a rating (internal/app/petition_rating.go:209) —
-- and the REAL actor goes to audit_log: actor_id = the citizen id, actor_kind = 'citizen'
-- (core/audit/audit.go:30-31; GuiPhanAnh.Gui and RatePetition.Rate pass the session's audit.Actor).
-- The reasons given there apply unchanged to `stored_file`:
--   * a staff screen reads this row (petition detail, "TRƯỚC KHI XỬ LÝ"), and an internal citizen id
--     on it would let staff link one person's ANONYMOUS reports together (ADR 0008);
--   * rule 6 invariant 8 forbids an internal id as the "who" of a row a person reads, and a citizen
--     HAS NO business code. 'cong-dan' is not a fallback for a missing code: it is the only value a
--     citizen upload may carry, and the CHECK below makes it so.
-- WHICH citizen is never lost: a citizen photo hangs off ONE petition, and only that petition's
-- `cong_dan_id` (0004) may upload to it — the use case checks it against the session (rule 4
-- invariant 2), and the trigger below refuses a petition with no citizen at all. The signed read
-- link binds to that same `phieu_phan_anh.cong_dan_id`, not to a column of this table.
--
-- 'cong-dan' cannot collide with a staff code (`CB-…`) and passes 0021's non-blank CHECK.
--
-- WHY subject_type = 'petition': the subject value names the ENTITY, in English, as 'task' does
-- (domain.StoredFileSubjectTask) — and the entity on `phieu_phan_anh` is `Petition` (0004's mark).
-- `subject_id` is the petition's INTERNAL id (`phieu_phan_anh.id`), as it is the task's for 'task';
-- never the lookup code, which is what a citizen holds and must not reach an object key or a join.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. ROWS PER COMMUNE: zero are written. ADD CONSTRAINT validates every existing `stored_file` row
--      (all are `task` files, uploaded_by a staff code, purpose `task-attachment`, and every new CHECK
--      is vacuous for them). The NOTICE lines below measure the rows per commune and per subject
--      (counts and tenant ids only). Operator pre-check:
--        SELECT tenant_id, subject_type, purpose, count(*) FROM stored_file GROUP BY 1, 2, 3;
--      Validation takes ACCESS EXCLUSIVE on `stored_file` for the length of one scan; the table holds
--      only task attachments today.
--   2. IF IT STOPS HALF-WAY: it cannot land half-applied — one file, one transaction (core/migrate),
--      with its progress row. Every statement is DROP … IF EXISTS / CREATE OR REPLACE, so a retry
--      costs nothing. No per-commune loop: there is nothing to backfill, and core/migrate gives a
--      file one transaction anyway (0025 question 1).
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. READ PATHS THAT CHANGE MEANING WHILE HALF-APPLIED: none can see it half-applied. AFTER it,
--      every reader of `stored_file` that does not filter on subject_type may meet a `petition` row:
--        * store/stored_file.go ForUpdate / ByID read by id only — the task use case re-checks
--          SubjectType == StoredFileSubjectTask (domain/stored_file.go:209, :225;
--          app/task_attachment.go:492), so a petition photo id offered to a task is refused there;
--        * task_log_attachment_check (0021) refuses `subject_type <> 'task'` at the link;
--        * the purge worker (not built) keys on retention_class, which here is `citizen-media`.
--      Nothing reads stored_file across subjects today.
--   5. RETENTION: a scene photo is CITIZEN PERSONAL DATA (rule 3: "scene photographs") inside an
--      archival record. Class `citizen-media`, retained 24 months (ADR 0052 §6, placeholder the
--      owner confirmed as temporary). `retain_until` stays NULL at upload and is fixed once, later,
--      by the act that fixes it (0021's write-once guard) — that act is not built here. Soft delete
--      only (0021's stored_file_guard refuses DELETE and is not touched).
--
-- ---------------------------------------------------------------------------

-- The subject list, widened in place under ITS OWN NAME, so a reader who finds 0021's constraint
-- finds this one. 0021:136 said a later migration would do exactly this.
ALTER TABLE stored_file DROP CONSTRAINT IF EXISTS stored_file_subject_type_known;
ALTER TABLE stored_file ADD CONSTRAINT stored_file_subject_type_known
    CHECK (subject_type IN ('task', 'petition'));

-- A CITIZEN UPLOADS SCENE PHOTOS AND NOTHING ELSE. The marker on a task file, or on any other
-- purpose, is a citizen inside a staff-only path — refused here, for every writer.
ALTER TABLE stored_file DROP CONSTRAINT IF EXISTS stored_file_citizen_upload_shape;
ALTER TABLE stored_file ADD CONSTRAINT stored_file_citizen_upload_shape
    CHECK (uploaded_by <> 'cong-dan'
           OR (subject_type = 'petition' AND purpose = 'petition-photo'));

-- A SCENE PHOTO IS PRIVATE CITIZEN MEDIA ON A PETITION, ALWAYS. "Never public" (owner, 02/10/2026)
-- is a column value here, not a promise of the write path: a row naming the public bucket, or the
-- `records` / `public-media` class, is refused. The object-key CHECK of 0021 then forces the key to
-- start with `citizen-media/t_<tenant>/` and to carry `/petitions/petition-photo/<id>/`.
ALTER TABLE stored_file DROP CONSTRAINT IF EXISTS stored_file_petition_photo_shape;
ALTER TABLE stored_file ADD CONSTRAINT stored_file_petition_photo_shape
    CHECK (purpose <> 'petition-photo'
           OR (subject_type = 'petition' AND retention_class = 'citizen-media' AND bucket = 'private'));

-- ---------------------------------------------------------------------------
-- stored_file_petition_check — the floor under the citizen upload path, for every writer.
--
--   * the petition exists IN THIS COMMUNE and is not soft-deleted (rule 7 invariant 2);
--   * a citizen upload names a petition a CITIZEN filed (`cong_dan_id` set): a staff-booked petition
--     with no citizen account has nobody whose session could have uploaded to it;
--   * AT MOST 5 CITIZEN SCENE PHOTOS per petition that REACHED THE DESTINATION — not deleted, status
--     `stored` / `processing` / `ready` — counted when a row ENTERS that set (INSERT, or the
--     `scanning → stored` edge). The petition row is locked FOR UPDATE first, so two completions on
--     one petition are serialised and cannot both pass at 4.
--
-- WHY THE FLOOR COUNTS ONLY STORED ROWS, NOT PENDING ONES: the issue-time limit — pending rows
-- included while their presigned form is still alive — is platform's `max_files_per_subject`, applied
-- by the use case inside its transaction (store/stored_file.go:291-325, the task path's
-- CountForSubjectTx). Counting `pending` here would let an abandoned upload hold a slot for ever,
-- because nothing moves it out of `pending` (store/stored_file.go:296-300), and the database cannot
-- know the form's lifetime without copying it. This floor guarantees the stated outcome — never more
-- than five photos a person can see — whatever the write path does.
--
-- WHY 5 IS WRITTEN HERE: it is the owner's ceiling of 02/10/2026, the same number platform's
-- `petition-photo` policy holds (service-platform/migrations/0008_upload_policy.sql:199). Raising the
-- policy without a migration here fails CLOSED (the sixth photo is refused), never open.
--
-- Messages name the relation and the rule, never an id or a code (rule 3, forbidden #3).
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION stored_file_petition_check() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
    petition_citizen TEXT;
    live_photos      INT;
BEGIN
    IF NEW.subject_type <> 'petition' THEN
        RETURN NEW;
    END IF;

    -- Only what changes the answer re-runs the check: a new row, or a row entering the stored set.
    IF TG_OP = 'UPDATE'
       AND NOT (NEW.status IN ('stored', 'processing', 'ready')
                AND OLD.status NOT IN ('stored', 'processing', 'ready')) THEN
        RETURN NEW;
    END IF;

    SELECT p.cong_dan_id INTO petition_citizen
      FROM phieu_phan_anh p
     WHERE p.tenant_id = NEW.tenant_id AND p.id = NEW.subject_id AND p.deleted_at IS NULL
       FOR UPDATE;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'stored_file: petition not found in this commune';
    END IF;

    IF NEW.uploaded_by = 'cong-dan' AND (petition_citizen IS NULL OR btrim(petition_citizen) = '') THEN
        RAISE EXCEPTION 'stored_file: a citizen upload needs a petition filed by a citizen'
            USING HINT = 'phieu_phan_anh.cong_dan_id is empty: no citizen session can own this petition.';
    END IF;

    IF NEW.purpose = 'petition-photo' AND NEW.uploaded_by = 'cong-dan'
       AND NEW.deleted_at IS NULL AND NEW.status IN ('stored', 'processing', 'ready') THEN
        SELECT count(*) INTO live_photos
          FROM stored_file f
         WHERE f.tenant_id = NEW.tenant_id
           AND f.subject_type = 'petition' AND f.subject_id = NEW.subject_id
           AND f.uploaded_by = 'cong-dan' AND f.purpose = 'petition-photo'
           AND f.deleted_at IS NULL
           AND f.status IN ('stored', 'processing', 'ready')
           AND f.id <> NEW.id;
        IF live_photos >= 5 THEN
            RAISE EXCEPTION 'stored_file: a petition holds at most 5 citizen scene photos'
                USING HINT = 'Owner decision 02/10/2026; platform upload_policy petition-photo.';
        END IF;
    END IF;

    RETURN NEW;
END $$;

-- BEFORE INSERT OR UPDATE OF status: the identity columns (subject, uploader, purpose) are frozen by
-- 0021's stored_file_guard and `deleted_at` is write-once there, so the status column is the only
-- way an existing row can enter the counted set. Row-level, so PostgreSQL clones it onto every
-- partition (PG ≥ 13).
DROP TRIGGER IF EXISTS stored_file_petition_check ON stored_file;
CREATE TRIGGER stored_file_petition_check
    BEFORE INSERT OR UPDATE OF status ON stored_file
    FOR EACH ROW EXECUTE FUNCTION stored_file_petition_check();

-- ---------------------------------------------------------------------------
-- MEASUREMENT (question 1): rows per commune and subject. Counts and tenant ids only.
-- ---------------------------------------------------------------------------
DO $$
DECLARE r record;
BEGIN
    FOR r IN
        SELECT tenant_id, subject_type, count(*) AS n
        FROM stored_file
        GROUP BY tenant_id, subject_type
        ORDER BY tenant_id, subject_type
    LOOP
        RAISE NOTICE '0026: commune % holds % stored_file row(s) for subject %', r.tenant_id, r.n, r.subject_type;
    END LOOP;
END $$;

-- ---------------------------------------------------------------------------
-- WHAT G8 STILL HAS. Staff "after" photos on a petition can use subject 'petition' with a STAFF code
-- in uploaded_by and THEIR OWN purpose (one line in core/storage, one policy row in platform): the
-- citizen-shape CHECK only binds the 'cong-dan' marker, and the 5-photo count only counts the
-- citizen's photos, so staff photos never consume the citizen's slots. Reusing `petition-photo` for
-- them is NOT open: the purpose's class/bucket CHECK would hold, but the two would share one
-- platform count limit.
--
-- WHAT THIS FILE DOES NOT STOP: the stored MIME type (re-encoding output is the write path's — the
-- MIME list lives in Go, 0021 and service-platform 0008 explain why it is not copied into a CHECK);
-- uploads to a closed petition (a business rule nobody has decided); TRUNCATE and owner DDL (0021).
--
-- REVERSAL (rule 7, invariant 4). Run by a person, in ONE transaction; core/migrate has no automatic
-- rollback (ADR 0013). Lossless WHILE NO ROW HAS subject_type = 'petition' — check
--   SELECT count(*) FROM stored_file WHERE subject_type = 'petition';  -- must be 0
-- then:
--   DROP TRIGGER IF EXISTS stored_file_petition_check ON stored_file;
--   DROP FUNCTION IF EXISTS stored_file_petition_check();
--   ALTER TABLE stored_file DROP CONSTRAINT IF EXISTS stored_file_petition_photo_shape;
--   ALTER TABLE stored_file DROP CONSTRAINT IF EXISTS stored_file_citizen_upload_shape;
--   ALTER TABLE stored_file DROP CONSTRAINT stored_file_subject_type_known;
--   ALTER TABLE stored_file ADD CONSTRAINT stored_file_subject_type_known CHECK (subject_type IN ('task'));
--   and remove this file's row from `schema_migration`.
-- ONCE ONE PHOTO EXISTS, narrowing the CHECK fails on it, and making it pass would mean deleting a
-- citizen's photo row — personal data inside an archival record, rule 7's stop condition and the
-- owner's decision. From then on the way back is a NEW migration, never a reversal of this one.
-- ---------------------------------------------------------------------------
