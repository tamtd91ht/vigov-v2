-- 0037 — merging duplicate petitions as a LINK (ADR 0087, owner answers of 09/10/2026). Schema only:
-- no row is written, no existing value changes.
--
-- WHAT ADR 0087 DECIDED, and the four "còn mở" items the owner answered on 09/10/2026 (relayed by the
-- main session; the ADR's own table still lists them open until it is amended):
--
--   * A merged petition carries `merged_into` → the MAIN petition. It is NOT closed and gains NO status
--     (ADR 0087 §1, stop condition #1). Each citizen keeps their own lookup code and stored deadline.
--   * Merge only while BOTH petitions are unresolved: `da-tiep-nhan`, `dang-phan-loai`,
--     `da-chuyen-xu-ly`, `dang-xu-ly`. Never from `cho-dan-xac-nhan` / `da-dong` / `khong-tiep-nhan` /
--     `chuyen-cap-tren`. `da-xu-ly` IS OUTSIDE THE SET TOO: the owner's range was "da-tiep-nhan …
--     dang-xu-ly" and "both not yet resolved"; `da-xu-ly` is "resolved". This is the writer's reading of
--     that range, stated so it can be challenged (open #2 of ADR 0087).
--   * NO CHAINS (open #3): a merged petition cannot receive merges, and a main petition that has merged
--     petitions cannot itself be merged. Always merge into the ROOT.
--   * DEADLINE (open #1): the main petition's `han_xu_ly_xong` becomes the EARLIER of the two; if one is
--     NULL ("chưa có", ADR 0028) the other's is taken. It only ever moves earlier (ADR 0087 stop #2).
--   * Unmerge is allowed, the reason is MANDATORY, the citizen is notified (ADR 0087 §5).
--   * Field `can-bo` is NEVER merged (ADR 0087 §5, ADR 0030).
--
-- ---------------------------------------------------------------------------
-- WHAT THE DATABASE ENFORCES vs WHAT THE USE CASE OWNS — decided here, stated once.
--
-- The rule for the split: an invariant that, if broken by ANY writer (a later route, a backfill, a
-- psql session), leaves the register in a shape ADR 0087 forbids goes in the database. An invariant
-- that needs another service, a notification, a permission or a person's choice stays in the app.
--
--   IN THE DATABASE (this file)                         | IN THE USE CASE (go-service-builder)
--   ----------------------------------------------------|-----------------------------------------------
--   same commune — composite FK (tenant_id, merged_into)| permission `feedback.classify` (ADR 0087 §5)
--   no self-merge — CHECK                               | choosing WHICH petition is the main one
--   no chain, both directions — trigger                 | writing the history row (below) and audit_log
--   both petitions unresolved, not deleted — trigger    |   in the SAME transaction (rule 6 inv 3)
--   `can-bo` never merged, either side — CHECK+trigger  | the citizen notification on merge / unmerge
--   main's deadline ALREADY the earlier — trigger       |   (ADR 0041 §Sửa đổi 09/10/2026)
--   who/when present while linked, cleared on unlink,   | the mandatory unmerge reason (history CHECK
--     frozen while the link stands — CHECK + trigger    |   enforces it on the history row only)
--   a petition is never BORN merged — trigger           | children following the main into
--   re-pointing a link directly refused — trigger       |   cho-dan-xac-nhan / da-dong (ADR 0087 §2)
--   history append-only; deadline never later — CHECK  | suspected-duplicate SEARCH (thresholds: 0038)
--                                                      | "only main petitions" predicate in reports
--
-- NOT ENFORCED ANYWHERE YET, said plainly: that every link change has a history row (and vice versa).
-- The pairing is the use case's, in one transaction, like nhat_ky_phan_anh's (0013).
--
-- ---------------------------------------------------------------------------
-- ⚠ A CONSEQUENCE THE OWNER HAS NOT SEEN — the deadline rule meets 0004's `phieu_phan_anh_han_sau_goc`.
--
-- 0004 requires `han_xu_ly_xong >= goc_dem_han` on every row. If the MERGED petition was reported
-- earlier and its deadline falls BEFORE the main petition's `goc_dem_han` (main reported later), then
-- "main takes the earlier deadline" writes a deadline that precedes the main's own origin, and 0004's
-- CHECK refuses the UPDATE. This file does NOT loosen that CHECK. The effect today is FAIL CLOSED: such a
-- merge is refused, and the officer must pick the OLDER petition as the main one. Whether that is the
-- intended rule, or 0004's CHECK should admit a merge-inherited deadline, is the OWNER's question.
--
-- ---------------------------------------------------------------------------
-- THE NAMES (rule 12, ADR 0051): English, on a Vietnamese-named table whose existing columns are not
-- renamed (precedent 0017 `rating_comment`, 0032 `zalo_account_id`). `merged_into` is the word ADR 0087
-- itself uses. Enum VALUES are Vietnamese without diacritics (ADR 0011): `gop-phieu`, `tach-phieu`,
-- shaped like 0013's `dong-phieu`. kb/00-foundation/ubiquitous-language.md has no row for "merge" yet.
--
-- ---------------------------------------------------------------------------
-- THE LOCK THIS TAKES. ADD COLUMN with no default is catalogue-only, but takes ACCESS EXCLUSIVE on the
-- parent and all 32 partitions; each ADD CONSTRAINT … CHECK scans every row under it and passes
-- vacuously (every new column is NULL). ADD FOREIGN KEY validates every row — trivially, MATCH SIMPLE
-- skips NULL. CREATE INDEX is plain (CONCURRENTLY is refused on a partitioned parent and inside
-- core/migrate's transaction — 0014's header). Held until COMMIT. The register is small — AN ASSUMPTION,
-- NOT A MEASUREMENT; the operator checks first:
--
--   SELECT count(*), pg_size_pretty(pg_total_relation_size('phieu_phan_anh')) FROM phieu_phan_anh;
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. ROWS PER COMMUNE: zero written. Every petition gains three NULL columns without a rewrite; the
--      NOTICE below reports per commune how many petitions exist (counts and tenant ids only, rule 3).
--      NO BACKFILL: no petition has ever been merged, so NULL is the TRUE value for every row.
--   2. IF IT STOPS HALF-WAY: it cannot land half-applied — one file, one transaction (core/migrate),
--      with its progress row. ADD COLUMN IF NOT EXISTS, constraints guarded by a pg_constraint lookup
--      bound to the table's oid, CREATE … IF NOT EXISTS, CREATE OR REPLACE FUNCTION and DROP TRIGGER IF
--      EXISTS before CREATE TRIGGER make a retry cost nothing. No per-commune loop: nothing to backfill.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. READ PATHS THAT CHANGE MEANING WHILE HALF-APPLIED: none can see it half-applied. AFTER it, before
--      the Go card: every read of phieu_phan_anh names its columns (store/phieu_phan_anh.go `cotPhieu`,
--      no `SELECT *`), so no reader sees the columns; every value is NULL, so "only main petitions"
--      (`merged_into IS NULL`) is every petition — today's reports are unchanged. Every existing write
--      passes: the INSERT names its columns (merged_into NULL → the new trigger's INSERT arm passes), and
--      the only existing UPDATE that names a watched column is classification setting `linh_vuc` — its
--      new arm refuses `can-bo` only on a petition that HAS merged petitions, of which none exist.
--      THE CARD THAT WIRES MERGING IN changes meaning: from then on every statistic ADR 0087 §7 lists
--      (on-time, by field, by department, by hamlet) must add `merged_into IS NULL`, and "Nhận vào" must
--      NOT. A report that forgets it counts one incident twice — silently.
--   5. RETENTION: a petition is an ARCHIVAL RECORD (rule 7). Nothing is dropped, retyped or renumbered;
--      `ma_tra_cuu` and 0004's guard are untouched. Every merge and unmerge is a historical record in
--      `petition_merge_event`, append-only. `reason` there is staff free text and may hold personal data.
-- ---------------------------------------------------------------------------

-- PostgreSQL 13 is the floor (BEFORE … FOR EACH ROW triggers on a partitioned table) — 0004's reasoning.
DO $$ BEGIN
    IF current_setting('server_version_num')::int < 130000 THEN
        RAISE EXCEPTION
            'petition merge needs PostgreSQL 13 or newer (server is %). Do not weaken this migration '
            'to fit an older server — the triggers below are what keep a merge from forming a chain.',
            current_setting('server_version');
    END IF;
END $$;

-- ---------------------------------------------------------------------------
-- 1. THE LINK ON THE PETITION.
--
-- `merged_into`  the MAIN petition's id, in the same commune. NULL = this petition is a main petition
--                (a "vụ việc") — which is every petition until somebody merges one.
-- `merged_at`    when the CURRENT link was made. Not the history: that is petition_merge_event.
-- `merged_by`    who made the current link — the staff BUSINESS CODE (`CB-00123`), never the internal
--                id (rule 6, invariant 8).
--
-- WHY who/when sit on the row AND in the history: the row's pair is the constraint that a link can
-- never exist with nobody answerable for it (CHECK below) — the history alone cannot say that, because
-- nothing ties its INSERT to the link's UPDATE. The two can still disagree if a use case writes one and
-- not the other; the history is the authority on the past, the row on the present.
-- ---------------------------------------------------------------------------
ALTER TABLE phieu_phan_anh ADD COLUMN IF NOT EXISTS merged_into TEXT;
ALTER TABLE phieu_phan_anh ADD COLUMN IF NOT EXISTS merged_at   TIMESTAMPTZ;
ALTER TABLE phieu_phan_anh ADD COLUMN IF NOT EXISTS merged_by   TEXT;

DO $$ BEGIN
    -- The three travel together: linked → all present and the who non-blank; unlinked → all NULL. So
    -- unmerging clears its who/when, and a stale `merged_by` never sits on an unlinked row. Every arm is
    -- TRUE or FALSE, never NULL (a CHECK treats NULL as passed).
    IF NOT EXISTS (SELECT 1 FROM pg_constraint
                   WHERE conrelid = 'phieu_phan_anh'::regclass
                     AND conname = 'phieu_phan_anh_merge_link_complete') THEN
        ALTER TABLE phieu_phan_anh ADD CONSTRAINT phieu_phan_anh_merge_link_complete CHECK (
            (merged_into IS NULL AND merged_at IS NULL AND merged_by IS NULL)
            OR (merged_into IS NOT NULL AND merged_at IS NOT NULL
                AND merged_by IS NOT NULL AND btrim(merged_by) <> ''));
    END IF;

    -- 64 = domain.CanBoToiDa, the bound this service already puts on a staff business code.
    IF NOT EXISTS (SELECT 1 FROM pg_constraint
                   WHERE conrelid = 'phieu_phan_anh'::regclass
                     AND conname = 'phieu_phan_anh_merged_by_max') THEN
        ALTER TABLE phieu_phan_anh ADD CONSTRAINT phieu_phan_anh_merged_by_max
            CHECK (char_length(merged_by) <= 64);
    END IF;

    -- A petition is not a duplicate of itself.
    IF NOT EXISTS (SELECT 1 FROM pg_constraint
                   WHERE conrelid = 'phieu_phan_anh'::regclass
                     AND conname = 'phieu_phan_anh_not_merged_into_self') THEN
        ALTER TABLE phieu_phan_anh ADD CONSTRAINT phieu_phan_anh_not_merged_into_self
            CHECK (merged_into IS NULL OR merged_into <> id);
    END IF;

    -- A merge is made after the petition entered the register.
    IF NOT EXISTS (SELECT 1 FROM pg_constraint
                   WHERE conrelid = 'phieu_phan_anh'::regclass
                     AND conname = 'phieu_phan_anh_merged_after_intake') THEN
        ALTER TABLE phieu_phan_anh ADD CONSTRAINT phieu_phan_anh_merged_after_intake
            CHECK (merged_at IS NULL OR merged_at >= vao_so_luc);
    END IF;

    -- THE MERGED SIDE IS NEVER `can-bo` (ADR 0087 §5): a staff-conduct petition carries its own read
    -- right (`feedback.restricted`, ADR 0030), and linking it to an ordinary one opens it to people
    -- without that right. As a row CHECK it also refuses RE-CLASSIFYING a merged petition into `can-bo`.
    -- The MAIN side is the trigger's (a CHECK cannot see another row). Same `IS DISTINCT FROM` shape as
    -- 0017's phieu_phan_anh_staff_conduct_never_public, so an unclassified (NULL) field passes.
    IF NOT EXISTS (SELECT 1 FROM pg_constraint
                   WHERE conrelid = 'phieu_phan_anh'::regclass
                     AND conname = 'phieu_phan_anh_staff_conduct_never_merged') THEN
        ALTER TABLE phieu_phan_anh ADD CONSTRAINT phieu_phan_anh_staff_conduct_never_merged
            CHECK (merged_into IS NULL OR linh_vuc IS DISTINCT FROM 'can-bo');
    END IF;

    -- SAME COMMUNE, BY CONSTRUCTION (rule 1, invariant 6; ADR 0087 §5 "không có đường gộp xuyên xã").
    -- Composite self-referencing FK onto the partitioned parent's primary key — the shape 0012 already
    -- uses for bien_ban_hop (0012:208-211). MATCH SIMPLE: a NULL merged_into is not checked. The main
    -- petition can never vanish from under its merged ones: hard DELETE is refused (0004).
    IF NOT EXISTS (SELECT 1 FROM pg_constraint
                   WHERE conrelid = 'phieu_phan_anh'::regclass
                     AND conname = 'phieu_phan_anh_merged_into_fk') THEN
        ALTER TABLE phieu_phan_anh ADD CONSTRAINT phieu_phan_anh_merged_into_fk
            FOREIGN KEY (tenant_id, merged_into) REFERENCES phieu_phan_anh (tenant_id, id);
    END IF;
END $$;

-- "The merged petitions of this main one": the drawer, the follow-the-main step on close, the trigger's
-- no-chain check, and the "only main petitions" statistics' anti-join. tenant_id first (rule 1, the
-- partition key). NOT filtered on deleted_at: a soft-deleted merged petition still points at its main,
-- and the no-chain check must see it (fail closed).
CREATE INDEX IF NOT EXISTS phieu_phan_anh_merged_children
    ON phieu_phan_anh (tenant_id, merged_into)
    WHERE merged_into IS NOT NULL;

-- SUSPECTED-DUPLICATE SEARCH (ADR 0087 §6): "petitions of this commune within N metres and D days".
-- No PostGIS (ADR 0072), so the query is a bounding box on lat/lng inside a time window, refined in Go.
--
--   * Time on `goc_dem_han` — when the citizen REPORTED it (ADR 0027 decision D), not when it was booked;
--     on the staff-booked channel the two differ by up to a week. The query must use the same column.
--   * Time BEFORE lat/lng: the window bounds one commune's rows to days of traffic; lat and lng in the
--     key let the box be filtered from the index without visiting the heap.
--   * `linh_vuc` is NOT in the key: an unclassified petition has none, and whether a candidate must share
--     the field is the use case's rule, not this index's. It is a heap filter if used.
--   * Partial: live rows, with coordinates, that can RECEIVE a merge — `merged_into IS NULL`, because a
--     merged petition is never a target (no chains). The status rule stays a filter: the index does not
--     churn on every status change.
CREATE INDEX IF NOT EXISTS phieu_phan_anh_duplicate_candidates
    ON phieu_phan_anh (tenant_id, goc_dem_han, lat, lng)
    WHERE deleted_at IS NULL AND merged_into IS NULL AND lat IS NOT NULL AND lng IS NOT NULL;

COMMENT ON COLUMN phieu_phan_anh.merged_into IS
    'ADR 0087: the MAIN petition this one is merged into, same commune (FK with tenant_id). NULL = a main '
    'petition (an incident). A link, not a status: the petition keeps its code, deadline and lifecycle. '
    'Statistics of work (on-time, by field/department/hamlet) count only merged_into IS NULL; intake '
    'counts every petition (ADR 0087 §7).';
COMMENT ON COLUMN phieu_phan_anh.merged_by IS
    'Staff business code of who made the CURRENT link (rule 6 inv 8). History: petition_merge_event.';

-- ---------------------------------------------------------------------------
-- phieu_phan_anh_merge_guard — the cross-row half of ADR 0087.
--
-- A SEPARATE FUNCTION AND TRIGGER, not a replacement of 0004's ho_so_luu_tru_bat_bien (three files pin
-- that one unchanged) — same choice as 0011 and 0032. Two BEFORE UPDATE triggers each refuse
-- independently, so their order changes no outcome.
--
-- CONCURRENCY. The main petition is read FOR UPDATE: two merges racing to form a chain (S→T while T→U)
-- serialise on T's row lock — whichever commits second re-reads the first's committed row (READ
-- COMMITTED re-check) or, as a plpgsql statement, takes a fresh snapshot for its children check, and is
-- refused. Two officers merging A→B and B→A at once is a lock cycle PostgreSQL detects and aborts.
--
-- MESSAGES name the relation and the rule only — never a code, a field value or a deadline: an error
-- message travels into logs and back to clients (rule 3, forbidden #3).
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION phieu_phan_anh_merge_guard() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
    main_status   TEXT;
    main_field    TEXT;
    main_parent   TEXT;
    main_deadline TIMESTAMPTZ;
    main_deleted  TIMESTAMPTZ;
BEGIN
    -- Born merged = a link with no merge act, no history row and no notification. Merging is a staff
    -- act on a petition that already exists.
    IF TG_OP = 'INSERT' THEN
        IF NEW.merged_into IS NOT NULL THEN
            RAISE EXCEPTION '%: a petition is never created already merged', TG_TABLE_NAME
                USING HINT = 'Insert it unlinked, then merge it as a separate audited act (ADR 0087).';
        END IF;
        RETURN NEW;
    END IF;

    -- THE MAIN SIDE OF `can-bo`: a petition that HAS merged petitions may not be re-classified into
    -- `can-bo` — that would hand ordinary reporters' incident to the restricted field (ADR 0030).
    IF NEW.linh_vuc IS NOT DISTINCT FROM 'can-bo' AND OLD.linh_vuc IS DISTINCT FROM 'can-bo'
       AND EXISTS (SELECT 1 FROM phieu_phan_anh c
                   WHERE c.tenant_id = NEW.tenant_id AND c.merged_into = NEW.id) THEN
        RAISE EXCEPTION '%: a petition with merged petitions cannot be classified can-bo', TG_TABLE_NAME
            USING HINT = 'Unmerge its petitions first (ADR 0087 §5: can-bo is never merged).';
    END IF;

    -- Link unchanged: its who/when are frozen while it stands. Rewriting them would make the row name
    -- somebody who did not make the link.
    IF NEW.merged_into IS NOT DISTINCT FROM OLD.merged_into THEN
        IF NEW.merged_at IS DISTINCT FROM OLD.merged_at OR NEW.merged_by IS DISTINCT FROM OLD.merged_by THEN
            RAISE EXCEPTION '%: merged_at and merged_by are frozen while the link stands', TG_TABLE_NAME;
        END IF;
        RETURN NEW;
    END IF;

    -- Re-pointing in one step skips the unmerge — its mandatory reason and its notification.
    IF OLD.merged_into IS NOT NULL AND NEW.merged_into IS NOT NULL THEN
        RAISE EXCEPTION '%: a merged petition cannot be moved to another main petition', TG_TABLE_NAME
            USING HINT = 'Unmerge it (reason required), then merge it again: two acts, two history rows.';
    END IF;

    -- UNMERGE. The link CHECK clears who/when; the reason lives on the history row (use case).
    IF NEW.merged_into IS NULL THEN
        RETURN NEW;
    END IF;

    -- MERGE from here on.
    IF NEW.merged_into = NEW.id THEN
        RAISE EXCEPTION '%: a petition cannot be merged into itself', TG_TABLE_NAME;
    END IF;
    IF NEW.deleted_at IS NOT NULL THEN
        RAISE EXCEPTION '%: a deleted petition cannot be merged', TG_TABLE_NAME;
    END IF;
    -- OLD and NEW both: a statement that merges AND moves the status cannot slip either way.
    IF OLD.trang_thai NOT IN ('da-tiep-nhan', 'dang-phan-loai', 'da-chuyen-xu-ly', 'dang-xu-ly')
       OR NEW.trang_thai NOT IN ('da-tiep-nhan', 'dang-phan-loai', 'da-chuyen-xu-ly', 'dang-xu-ly') THEN
        RAISE EXCEPTION '%: only an unresolved petition can be merged', TG_TABLE_NAME
            USING HINT = 'Allowed: da-tiep-nhan, dang-phan-loai, da-chuyen-xu-ly, dang-xu-ly (owner, '
                         '09/10/2026). Merging a resolved or closed record edits an archival record.';
    END IF;
    -- NO CHAINS, this side: a main petition with merged petitions is not merged itself.
    IF EXISTS (SELECT 1 FROM phieu_phan_anh c
               WHERE c.tenant_id = NEW.tenant_id AND c.merged_into = NEW.id) THEN
        RAISE EXCEPTION '%: a petition with merged petitions cannot itself be merged', TG_TABLE_NAME
            USING HINT = 'Merge into the root: pick this petition as the main one instead (no chains).';
    END IF;

    SELECT p.trang_thai, p.linh_vuc, p.merged_into, p.han_xu_ly_xong, p.deleted_at
      INTO main_status, main_field, main_parent, main_deadline, main_deleted
      FROM phieu_phan_anh p
     WHERE p.tenant_id = NEW.tenant_id AND p.id = NEW.merged_into
       FOR UPDATE;
    IF NOT FOUND THEN
        RAISE EXCEPTION '%: main petition not found in this commune', TG_TABLE_NAME;
    END IF;
    IF main_deleted IS NOT NULL THEN
        RAISE EXCEPTION '%: a deleted petition cannot be a main petition', TG_TABLE_NAME;
    END IF;
    -- NO CHAINS, the other side: the target is itself merged.
    IF main_parent IS NOT NULL THEN
        RAISE EXCEPTION '%: the main petition is itself merged', TG_TABLE_NAME
            USING HINT = 'Merge into the root petition (no chains, owner 09/10/2026).';
    END IF;
    IF main_status NOT IN ('da-tiep-nhan', 'dang-phan-loai', 'da-chuyen-xu-ly', 'dang-xu-ly') THEN
        RAISE EXCEPTION '%: the main petition is not unresolved', TG_TABLE_NAME
            USING HINT = 'Allowed: da-tiep-nhan, dang-phan-loai, da-chuyen-xu-ly, dang-xu-ly.';
    END IF;
    IF main_field IS NOT DISTINCT FROM 'can-bo' THEN
        RAISE EXCEPTION '%: a can-bo petition is never a main petition', TG_TABLE_NAME
            USING HINT = 'ADR 0087 §5, ADR 0030.';
    END IF;
    -- THE DEADLINE, as a FLOOR UNDER THE USE CASE: the main petition must ALREADY carry the earlier
    -- `han_xu_ly_xong` when the link is written — so the use case updates the main first, then links.
    -- Merged petition NULL → nothing to inherit. Main NULL and merged set → the main must have taken it.
    -- Without this, a merge that forgot the deadline would silently leave the earlier-promised citizen
    -- behind the later one's clock (ADR 0087 §1).
    IF NEW.han_xu_ly_xong IS NOT NULL
       AND (main_deadline IS NULL OR main_deadline > NEW.han_xu_ly_xong) THEN
        RAISE EXCEPTION '%: the main petition must already carry the earlier resolve deadline', TG_TABLE_NAME
            USING HINT = 'Move the main petition''s han_xu_ly_xong to the earlier of the two BEFORE '
                         'linking (ADR 0087 §1; owner 09/10/2026: NULL takes the other''s).';
    END IF;

    RETURN NEW;
END $$;

DROP TRIGGER IF EXISTS phieu_phan_anh_merge_guard ON phieu_phan_anh;
CREATE TRIGGER phieu_phan_anh_merge_guard
    BEFORE INSERT OR UPDATE OF merged_into, merged_at, merged_by, linh_vuc ON phieu_phan_anh
    FOR EACH ROW EXECUTE FUNCTION phieu_phan_anh_merge_guard();

-- ---------------------------------------------------------------------------
-- @entity: PetitionMergeEvent
-- @scope:  tenant
--
-- petition_merge_event — every merge and unmerge, as it happened: which petition, into which main one,
-- who, when, why, and what it did to the main petition's resolve deadline.
--
-- APPEND-ONLY, ENFORCED BY A TRIGGER (rule 7, forbidden #5), so NO SOFT-DELETE COLUMNS — same shape and
-- reasoning as nhat_ky_phan_anh (0013). It is a BUSINESS record; audit_log is written too, for the same
-- act, in the same transaction (rule 6, invariant 3).
--
-- STAFF-INTERNAL. The citizen of either petition never reads it (ADR 0087 §4; rule 4).
--
-- NO FOREIGN KEY to phieu_phan_anh, for 0013's reason: a historical entry must survive any future
-- reshaping of the register, and the write path holds both petitions under row lock in the same
-- transaction (the merge guard above). The petitions are never hard-deleted (0004).
--
-- `reason` MAY HOLD PERSONAL DATA (staff free text quoting a report). Never logged, never in an error,
-- never on an event. 0013's open decision on Decree 13 erasure vs append-only applies here unchanged.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS petition_merge_event (
    tenant_id             TEXT        NOT NULL,
    id                    TEXT        NOT NULL,

    -- The MERGED petition (the one carrying `merged_into`) and the MAIN one it was linked to / from.
    petition_id           TEXT        NOT NULL,
    main_petition_id      TEXT        NOT NULL,

    -- `gop-phieu` = merge, `tach-phieu` = unmerge. Enum values per ADR 0011.
    kind                  TEXT        NOT NULL,

    performed_at          TIMESTAMPTZ NOT NULL,
    -- Staff BUSINESS CODE (rule 6, invariant 8). No fallback: blank is refused.
    performed_by          TEXT        NOT NULL,

    -- Mandatory on unmerge (owner, 09/10/2026); optional on merge, never blank.
    reason                TEXT,

    -- The MAIN petition's `han_xu_ly_xong` immediately before and after this act. NULL = "chưa có"
    -- (ADR 0028), not "unbounded". The merged petition's own deadline never changes and is not copied.
    main_deadline_before  TIMESTAMPTZ,
    main_deadline_after   TIMESTAMPTZ,

    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (tenant_id, id),

    CONSTRAINT petition_merge_event_kind_valid CHECK (kind IN ('gop-phieu', 'tach-phieu')),
    CONSTRAINT petition_merge_event_not_self CHECK (petition_id <> main_petition_id),
    CONSTRAINT petition_merge_event_ids_present
        CHECK (btrim(petition_id) <> '' AND btrim(main_petition_id) <> ''),
    CONSTRAINT petition_merge_event_performed_by_valid
        CHECK (btrim(performed_by) <> '' AND char_length(performed_by) <= 64),

    -- Unmerge: reason present and non-blank. Merge: absent, or non-blank. Every arm TRUE/FALSE.
    CONSTRAINT petition_merge_event_reason_valid CHECK (
        CASE kind
            WHEN 'tach-phieu' THEN reason IS NOT NULL AND btrim(reason) <> ''
            ELSE reason IS NULL OR btrim(reason) <> ''
        END),
    -- 2000 = domain.KetQuaToiDa, the bound on the other staff-written paragraph of a petition.
    CONSTRAINT petition_merge_event_reason_max CHECK (char_length(reason) <= 2000),

    -- A DEADLINE NEVER MOVES LATER, on merge or unmerge (ADR 0087 stop condition #2). From "chưa có" it
    -- may take any value (owner: NULL takes the other's); once set it may stay or move earlier, and may
    -- never return to NULL.
    CONSTRAINT petition_merge_event_deadline_never_later CHECK (
        main_deadline_before IS NULL
        OR (main_deadline_after IS NOT NULL AND main_deadline_after <= main_deadline_before))
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS petition_merge_event_p%s PARTITION OF petition_merge_event '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

-- One petition's merge history (the merged petition's drawer), newest first; `id` the cursor tie-break.
CREATE INDEX IF NOT EXISTS petition_merge_event_by_petition
    ON petition_merge_event (tenant_id, petition_id, performed_at DESC, id DESC);
-- One main petition's history: who was merged into it and taken out again.
CREATE INDEX IF NOT EXISTS petition_merge_event_by_main
    ON petition_merge_event (tenant_id, main_petition_id, performed_at DESC, id DESC);

-- Append-only guard: 0013's shape. Row-level on the parent (cloned to every partition, now and later);
-- TRUNCATE per partition, because PostgreSQL refuses a TRUNCATE trigger on a partitioned table.
CREATE OR REPLACE FUNCTION petition_merge_event_append_only() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'petition_merge_event is append-only: % on % refused', TG_OP, TG_TABLE_NAME
        USING HINT = 'A merge or unmerge entry is a historical record (rule 7, forbidden #5): never '
                     'modified, never removed, never truncated. A correction is another act.';
END $$;

DROP TRIGGER IF EXISTS petition_merge_event_no_update_delete ON petition_merge_event;
CREATE TRIGGER petition_merge_event_no_update_delete
    BEFORE UPDATE OR DELETE ON petition_merge_event
    FOR EACH ROW EXECUTE FUNCTION petition_merge_event_append_only();

DO $$
DECLARE part regclass;
BEGIN
    FOR part IN
        SELECT inhrelid::regclass FROM pg_inherits
        WHERE inhparent = 'petition_merge_event'::regclass
    LOOP
        EXECUTE format('DROP TRIGGER IF EXISTS petition_merge_event_no_truncate ON %s', part);
        EXECUTE format(
            'CREATE TRIGGER petition_merge_event_no_truncate BEFORE TRUNCATE ON %s '
            'FOR EACH STATEMENT EXECUTE FUNCTION petition_merge_event_append_only()', part);
    END LOOP;
END $$;

-- ---------------------------------------------------------------------------
-- MEASUREMENT (question 1): petitions per commune, and how many carry coordinates (the ones the
-- duplicate search can ever see). Counts and tenant ids only.
-- ---------------------------------------------------------------------------
DO $$
DECLARE r record;
BEGIN
    FOR r IN
        SELECT tenant_id,
               count(*) AS total,
               count(*) FILTER (WHERE lat IS NOT NULL AND lng IS NOT NULL) AS with_coordinates
        FROM phieu_phan_anh
        GROUP BY tenant_id
        ORDER BY tenant_id
    LOOP
        RAISE NOTICE '0037: commune % holds % petition(s), % with coordinates (merged_into NULL on all)',
            r.tenant_id, r.total, r.with_coordinates;
    END LOOP;
END $$;

-- ---------------------------------------------------------------------------
-- WHAT THIS FILE DOES NOT STOP, said plainly: DDL by the table owner (dropping constraints, DISABLE
-- TRIGGER), TRUNCATE … CASCADE through a parent it does not own — ADR 0013's line. Nor: a link change
-- with no history row, soft-deleting a main petition that still has merged ones, or unmerging a petition
-- that already followed its main into cho-dan-xac-nhan / da-dong — all three are the use case's call and
-- the last two are not decided by anyone yet.
--
-- REVERSAL (rule 7, invariant 4). Run by a person, in ONE transaction; core/migrate has no automatic
-- rollback (ADR 0013). Lossless ONLY WHILE NOTHING HAS BEEN MERGED:
--
--   SELECT count(*) FROM phieu_phan_anh WHERE merged_into IS NOT NULL;   -- must be 0
--   SELECT count(*) FROM petition_merge_event;                           -- must be 0
--
-- BOTH ZERO → in this order:
--   DROP TABLE petition_merge_event;                     (partitions and triggers go with it)
--   DROP FUNCTION petition_merge_event_append_only();
--   DROP TRIGGER IF EXISTS phieu_phan_anh_merge_guard ON phieu_phan_anh;
--   DROP FUNCTION IF EXISTS phieu_phan_anh_merge_guard();
--   DROP INDEX IF EXISTS phieu_phan_anh_duplicate_candidates;
--   DROP INDEX IF EXISTS phieu_phan_anh_merged_children;
--   ALTER TABLE phieu_phan_anh DROP CONSTRAINT phieu_phan_anh_merged_into_fk;
--   ALTER TABLE phieu_phan_anh DROP CONSTRAINT phieu_phan_anh_staff_conduct_never_merged;
--   ALTER TABLE phieu_phan_anh DROP CONSTRAINT phieu_phan_anh_merged_after_intake;
--   ALTER TABLE phieu_phan_anh DROP CONSTRAINT phieu_phan_anh_not_merged_into_self;
--   ALTER TABLE phieu_phan_anh DROP CONSTRAINT phieu_phan_anh_merged_by_max;
--   ALTER TABLE phieu_phan_anh DROP CONSTRAINT phieu_phan_anh_merge_link_complete;
--   ALTER TABLE phieu_phan_anh DROP COLUMN merged_by;
--   ALTER TABLE phieu_phan_anh DROP COLUMN merged_at;
--   ALTER TABLE phieu_phan_anh DROP COLUMN merged_into;
-- and remove this file's row from `schema_migration`.
--
-- EITHER NON-ZERO → the links and their history are records of acts on archival records, and main
-- petitions' deadlines were moved by those acts. Dropping them leaves earlier deadlines no row can
-- explain. Rule 7 stop condition #2: a user decision plus a verified backup, never a command.
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
                         'MODULUS 32 partition loop (ADR 0010) in the same migration that '
                         'declares PARTITION BY.';
    END IF;
END $$;
