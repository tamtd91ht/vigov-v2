-- 0017 — a petition's PUBLICATION STATUS becomes three-state (`publication_status`), and the
-- citizen's star rating carries an optional comment (`rating_comment`).
--
-- WHY THIS FILE EXISTS. User decisions 28/09/2026, ADR 0050 points 2 and 8
-- (kb/10-decisions/0050-kenh-cong-dan-theo-kho-yeu-cau.md:54, :60), following ../vigov-require:
--
--   point 2   the citizen rates 1–5 stars "kèm nhận xét tuỳ ý" — WITH AN OPTIONAL COMMENT.
--   point 8   no public board on the citizen side, but the server owes STAFF MODERATION with its own
--             three states, separate from the nine lifecycle statuses (require `05-nghiep-vu.md:192-193`:
--             pending / approved / hidden); a staff-conduct petition is NEVER public
--             (`05-nghiep-vu.md:204`, require `never_public`) and is born hidden (`service.py:301-307`).
--
-- WHY A NEW FILE AND NOT AN EDIT OF 0004: core/migrate compares the checksum of every applied file at
-- startup. Editing an applied file either stops the service (ErrChecksumLech) or leaves two databases
-- claiming one schema version while holding two schemas.
--
-- ---------------------------------------------------------------------------
-- A SUPERSESSION RECORDED HERE BECAUSE 0004 CANNOT BE EDITED.
--
-- 0004:314-315 says of `so_lan_mo_lai`: "ADR 0008 caps reopening with the per-commune
-- `so_lan_mo_lai_toi_da` (default 1)". THAT CAP NO LONGER EXISTS. ADR 0050 point 2 (round 2 and 3,
-- 28/09/2026) replaced ADR 0008:50-53: a rating of 1 or 2 stars reopens the petition automatically,
-- with NO cap on the number of reopenings, NO recomputation of any deadline, and NO per-commune switch
-- to disable it. `so_lan_mo_lai` stays what it is — a count — and is now read only as a monitoring
-- figure ("một phiếu có thể quay vòng mãi; theo dõi bằng so_lan_mo_lai", ADR 0050 §Cái giá). No code
-- may read `so_lan_mo_lai_toi_da`, `nguong_sao_mo_lai`, `tinh_lai_han_khi_mo_lai` or
-- `cho_phep_mo_lai`: none of them is built, and none may be. This file changes nothing about the
-- column itself.
--
-- ---------------------------------------------------------------------------
-- THE TWO NEW COLUMNS. English names (rule 12, ADR 0051); enum VALUES Vietnamese without diacritics
-- (ADR 0011:47, :83-100).
--
--   publication_status   'cho-duyet'  awaiting moderation — the default: a petition is not public
--                                     until a member of staff has read it (docs/ui-ux/09 §14.4, the
--                                     label `CHỜ KIỂM DUYỆT` at 09:45 and :298)
--                        'cong-khai'  approved for publication
--                        'an'         hidden by staff, or never publishable (staff conduct)
--
--   THE THREE VALUES ARE NOT IN kb/00-foundation/ubiquitous-language.md — it has no row for the
--   moderation concept at all, so there was no existing term to follow and none to conflict with. The
--   values are the user's (this session). Two neighbours, stated so a reader does not "unify" them:
--     * `nhiem_vu.trang_thai` and `de_nghi_lui_han.trang_thai` already use 'cho-duyet'
--       (0006:341, :604) for "awaiting APPROVAL of a task / an extension" — a different column on a
--       different table; the word means "awaiting a staff decision" in all three.
--     * service-comms 0006:371 moderates Mini App content with ('dang-hien', 'cho-duyet', 'an'). Same
--       shape, a different service's entity; 'cong-khai' here is "approved for a public page", not
--       "currently displayed".
--
--   IT IS SEPARATE FROM `trang_thai`, and must stay separate: the nine lifecycle codes are closed and
--   approved verbatim by the customer (ADR 0027, 0004:275-279). Whether a petition may be shown in
--   public is a staff decision ABOUT the record, not a step in handling it — a petition can be
--   `da-dong` and hidden, or `dang-xu-ly` and public.
--
--   rating_comment       the citizen's optional free text accompanying `diem_hai_long`. NULL = no
--                        comment. Replaced together with the stars when the citizen re-rates after a
--                        reopening (ADR 0050 point 2: "lần mới thay lần cũ").
--
--   ⚠ 1000 CHARACTERS IS A VENDOR RECOMMENDATION, NOT THE CUSTOMER'S NUMBER. It caps abuse of a field
--   a WEAK identity writes (rule 4: phone + OTP) and that staff must read; it is the same order as the
--   2000 cap on the staff-written paragraphs of this register (0011:185 `ly_do_ket_thuc_nhanh`,
--   0013:208 `nhat_ky_phan_anh.noi_dung`), and half of it because a rating comment is a remark, not a
--   report — the report itself is `noi_dung`. Raising it later is one migration; lowering it after
--   citizens have written longer comments is not. The write path must validate the same number with
--   utf8.RuneCountInString so a client sees a 400, not a 500.
--
--   `rating_comment` IS CITIZEN PERSONAL DATA in practice (rule 3): free text eventually names people
--   and places. It never enters a log line, an error message or an event payload — same standing as
--   `noi_dung`.
--
-- ---------------------------------------------------------------------------
-- `hien_cong_khai` (0004:309) IS SUPERSEDED, NOT DROPPED.
--
-- Measured 28/09/2026 by grep over the repository: NO code path ever writes it to true. Intake INSERTs
-- it from domain.PhieuPhanAnh.HienCongKhai (internal/store/phieu_phan_anh.go:351, :366), which intake
-- leaves false (internal/app/gui_phan_anh.go:353); no UPDATE names it. It IS READ: the store scans it
-- (phieu_phan_anh.go:57, :296) and the staff API returns it as `public` (internal/http/phieu_phan_anh.go:205),
-- shown by web-admin (`nhanHienCongKhai`). It is kept — rule 7 forbids a lossy migration, and a column
-- whose every value is `false` is still a column of an archival record — and marked below, with
-- COMMENT ON COLUMN, as never to be read again. Moving the read path to `publication_status` is the
-- Go change that follows this file; until then `public: false` stays true of every row (question 4).
--
-- ---------------------------------------------------------------------------
-- THE LOCK THIS TAKES, AND WHY IT IS ACCEPTABLE — read before applying to a large register.
--
--   * ADD COLUMN on the partitioned parent cascades to all 32 partitions. With a CONSTANT default
--     (`'cho-duyet'`) or none (`rating_comment`), PostgreSQL 11+ records the default in the catalogue
--     (attmissingval) and REWRITES NO ROW — metadata only. It still takes an ACCESS EXCLUSIVE lock on
--     the parent and on every partition: READS AND WRITES BOTH WAIT.
--   * The ADD CONSTRAINT … CHECK statements VALIDATE by scanning every row of every partition, under
--     that same ACCESS EXCLUSIVE lock.
--   * The backfill UPDATE touches only the rows it matches (staff-conduct petitions, and any row with
--     `hien_cong_khai = true`, expected zero) — each one a new row version, with its audit entry.
--
-- Because core/migrate runs the whole file in ONE transaction with its progress row
-- (core/migrate/migrate.go:32-33, 46-49), the ACCESS EXCLUSIVE locks are held UNTIL COMMIT — for the
-- sum of the scans and the backfill, not for one statement. Citizen intake, staff transitions and the
-- citizen's own list all queue behind it.
--
-- ACCEPTABLE AT CURRENT SIZES: the register has been live for one commune only (0014's header), so it
-- holds on the order of hundreds to low thousands of rows; the scans are milliseconds. THIS IS AN
-- ASSUMPTION ABOUT PRODUCTION, NOT A MEASUREMENT. The operator applying it checks first:
--
--   SELECT count(*), pg_size_pretty(pg_total_relation_size('phieu_phan_anh')) FROM phieu_phan_anh;
--   SELECT tenant_id, count(*) FROM phieu_phan_anh WHERE linh_vuc = 'can-bo' GROUP BY tenant_id;
--
-- If the first ever reads in the millions, this file is the wrong tool at service start-up: the
-- constraints would then be added NOT VALID and validated separately, by a person, out of hours.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS, answered before the SQL.
--
--   1. HOW MANY ROWS PER COMMUNE: every row gains two columns without a rewrite. The backfill WRITES
--      only (a) the commune's petitions in field `can-bo`, SOFT-DELETED ONES INCLUDED, and (b) rows with
--      `hien_cong_khai = true`, expected ZERO (see above). Each written row gets one audit entry with
--      the system principal (rule 6, invariant 6). The NOTICE lines report, per commune, how many rows
--      now hold a status other than the default — counts and tenant ids only (rule 3).
--      PER COMMUNE AND RESUMABLE (rule 7, invariant 5): ONE STATEMENT PER CASE, NOT A PER-COMMUNE LOOP,
--      for the reason 0015's header gives (service-identity/migrations/0003_nguoi_dung_co_tai_khoan.sql:
--      100-107): the runner gives the file one transaction, so a loop inside it could not be resumed
--      either, and the genuinely resumable per-commune mechanism does not exist yet in core/migrate.
--   2. IF IT STOPS HALF-WAY: it cannot land half-applied — one file, one transaction, with the progress
--      row. A failure rolls back columns, backfill, audit entries and constraints together. A retry
--      costs nothing: ADD COLUMN IF NOT EXISTS, constraints guarded by a pg_constraint lookup, and the
--      backfill matches only rows still at the default `'cho-duyet'` — a row already moved is never
--      matched, so it is never written or audited twice.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom. `hien_cong_khai` is untouched, so the prior
--      schema is fully recoverable while no staff moderation decision and no rating comment exist.
--   4. WHICH READ PATHS CHANGE MEANING WHILE IT IS HALF-APPLIED: none can see it half-applied (one
--      transaction). BETWEEN THIS FILE AND THE GO CHANGE:
--        * No read path reads either new column; the store's column list is unchanged. The staff API's
--          `public` still reads `hien_cong_khai`, which is false on every row — still a true statement,
--          since nothing is public in either column.
--        * ⚠ NEW STAFF-CONDUCT ROWS ARE BORN 'cho-duyet', NOT 'an', until the write path sets it: intake
--          (a staff-booked petition carries its field at creation; ADR 0050 point 1 lets the citizen
--          pick it too) and classification into `can-bo` do not know the column. The backfill fixes
--          existing rows only. What stays guaranteed for EVERY writer is the half that matters most:
--          the CHECK `phieu_phan_anh_staff_conduct_never_public` refuses 'cong-khai' on a `can-bo`
--          row, so none can be published. The Go change owes: set 'an' at insert and in the same
--          UPDATE that classifies a petition into `can-bo` — a public petition reclassified into
--          `can-bo` without it is refused by that CHECK (fail closed, a 500 until the path is written).
--          A STRICTER CHECK ("`can-bo` ⇒ 'an'") is deliberately NOT added now: today it would turn every
--          staff-booked `can-bo` intake and every classification into `can-bo` into a failure.
--   5. RETENTION: `phieu_phan_anh` is an ARCHIVAL RECORD (rule 7). No column is dropped, retyped or
--      emptied; `ho_so_luu_tru_bat_bien` (0004) and `phieu_phan_anh_ket_thuc_nhanh_bat_bien` (0011)
--      are unchanged and both pass the backfill (it touches none of their columns). No business code
--      is issued or renumbered.
--
-- ---------------------------------------------------------------------------
-- WHAT THIS FILE DELIBERATELY DOES NOT DO:
--
--   * No index. The moderation queue does not exist yet; its index belongs with its query, like 0014.
--   * No moderation route, no permission key. A key the `quyen` table lacks is a finding for open
--     question #27 (rule 5, invariant 3c), not something a migration seeds.
--   * No trigger that forces 'an' on `can-bo` rows. It would change a column silently inside another
--     act's UPDATE; the write path sets it explicitly and audits it (question 4).
--   * Nothing about the reopening rule beyond the header note: it is a transition, not a column.

-- ---------------------------------------------------------------------------
-- THE TWO COLUMNS.
-- ---------------------------------------------------------------------------
ALTER TABLE phieu_phan_anh
    ADD COLUMN IF NOT EXISTS publication_status TEXT NOT NULL DEFAULT 'cho-duyet';

ALTER TABLE phieu_phan_anh ADD COLUMN IF NOT EXISTS rating_comment TEXT;

-- ---------------------------------------------------------------------------
-- BACKFILL, with its audit entries, in this order:
--
--   (a) `can-bo` → 'an'. First, so that (b) can never publish a staff-conduct row.
--   (b) `hien_cong_khai = true` → 'cong-khai', staff conduct excluded. Expected zero rows: nothing has
--       ever written true. If a row does match, it carries a publication decision somebody made, and
--       carrying it over is the lossless reading of it.
--
-- SOFT-DELETED ROWS ARE INCLUDED ON PURPOSE: the column states what may be shown of a record, and a
-- soft-deleted staff-conduct petition is no more publishable than a live one.
--
-- THE AUDIT ENTRY is written in the same statement as the change (a data-modifying CTE), so a row
-- cannot move without its entry (rule 6, invariant 3). Shape as core/audit.Write: actor 'system' /
-- kind 'system' (core/audit.SystemActor), subject = `ma_tra_cuu`, the business code — never `id`
-- (rule 6, invariant 8). The delta holds the two status values only: no personal data (rule 6,
-- forbidden #4).
-- ---------------------------------------------------------------------------
DO $$
DECLARE
    hidden_rows    int;
    published_rows int;
    r              record;
BEGIN
    WITH changed AS (
        UPDATE phieu_phan_anh
        SET publication_status = 'an', cap_nhat_luc = now()
        WHERE linh_vuc = 'can-bo' AND publication_status = 'cho-duyet'
        RETURNING tenant_id, ma_tra_cuu
    )
    INSERT INTO audit_log (tenant_id, actor_id, actor_kind, actor_ip, action, subject, at, delta)
    SELECT tenant_id, 'system', 'system', '', 'dat_trang_thai_cong_khai', ma_tra_cuu, now(),
           jsonb_build_object(
               'truoc', jsonb_build_object('publication_status', 'cho-duyet'),
               'sau',   jsonb_build_object('publication_status', 'an'),
               'ly_do', 'migration 0017: staff-conduct petitions are never public (ADR 0050 point 8)')
    FROM changed;
    GET DIAGNOSTICS hidden_rows = ROW_COUNT;

    WITH changed AS (
        UPDATE phieu_phan_anh
        SET publication_status = 'cong-khai', cap_nhat_luc = now()
        WHERE hien_cong_khai = true
          AND linh_vuc IS DISTINCT FROM 'can-bo'
          AND publication_status = 'cho-duyet'
        RETURNING tenant_id, ma_tra_cuu
    )
    INSERT INTO audit_log (tenant_id, actor_id, actor_kind, actor_ip, action, subject, at, delta)
    SELECT tenant_id, 'system', 'system', '', 'dat_trang_thai_cong_khai', ma_tra_cuu, now(),
           jsonb_build_object(
               'truoc', jsonb_build_object('publication_status', 'cho-duyet'),
               'sau',   jsonb_build_object('publication_status', 'cong-khai'),
               'ly_do', 'migration 0017: carried over from hien_cong_khai = true')
    FROM changed;
    GET DIAGNOSTICS published_rows = ROW_COUNT;

    RAISE NOTICE '0017 backfill: % staff-conduct petition(s) set to an, % set to cong-khai from hien_cong_khai',
        hidden_rows, published_rows;

    FOR r IN
        SELECT tenant_id, publication_status, count(*) AS n
        FROM phieu_phan_anh
        WHERE publication_status <> 'cho-duyet'
        GROUP BY tenant_id, publication_status
        ORDER BY tenant_id, publication_status
    LOOP
        RAISE NOTICE '0017 backfill: commune % holds % petition(s) with publication_status %',
            r.tenant_id, r.n, r.publication_status;
    END LOOP;
END $$;

-- ---------------------------------------------------------------------------
-- THE CONSTRAINTS. After the backfill, so the never-public CHECK validates rows already corrected.
--
-- DO $$ … $$ RATHER THAN A BARE ALTER: `ADD CONSTRAINT` has no IF NOT EXISTS, and a retry must cost
-- nothing (question 2). Lookup by name, the pattern of 0005 and 0011.
-- ---------------------------------------------------------------------------
DO $$ BEGIN
    -- The closed list of three. A fourth value is a migration, never a string a handler invents.
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'phieu_phan_anh_publication_status_valid'
    ) THEN
        ALTER TABLE phieu_phan_anh ADD CONSTRAINT phieu_phan_anh_publication_status_valid
            CHECK (publication_status IN ('cho-duyet', 'cong-khai', 'an'));
    END IF;

    -- A STAFF-CONDUCT PETITION IS NEVER PUBLIC (ADR 0050 point 8 and its stop condition #2), enforced
    -- for every writer rather than promised by one route. The field code is domain.LinhVucHanChe
    -- (internal/domain/xu_ly_phan_anh.go:68); migrations/petition_publication_test.go reads that file
    -- so the two cannot drift. IS DISTINCT FROM, so an unclassified petition (NULL field) is unaffected
    -- and the arm can only evaluate to TRUE or FALSE, never NULL — a CHECK treats NULL as passed.
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'phieu_phan_anh_staff_conduct_never_public'
    ) THEN
        ALTER TABLE phieu_phan_anh ADD CONSTRAINT phieu_phan_anh_staff_conduct_never_public
            CHECK (linh_vuc IS DISTINCT FROM 'can-bo' OR publication_status <> 'cong-khai');
    END IF;

    -- Present means non-blank and bounded IN CHARACTERS: char_length counts characters, not bytes, so
    -- Vietnamese diacritics do not shorten the allowance. '' is not "no comment" — NULL is.
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'phieu_phan_anh_rating_comment_valid'
    ) THEN
        ALTER TABLE phieu_phan_anh ADD CONSTRAINT phieu_phan_anh_rating_comment_valid
            CHECK (rating_comment IS NULL
                   OR (btrim(rating_comment) <> '' AND char_length(rating_comment) <= 1000));
    END IF;

    -- A comment ACCOMPANIES a rating (ADR 0050 point 2: "1–5 sao kèm nhận xét tuỳ ý"); a comment with
    -- no stars is a second free-text channel nobody decided to open.
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'phieu_phan_anh_rating_comment_needs_rating'
    ) THEN
        ALTER TABLE phieu_phan_anh ADD CONSTRAINT phieu_phan_anh_rating_comment_needs_rating
            CHECK (rating_comment IS NULL OR diem_hai_long IS NOT NULL);
    END IF;
END $$;

-- ---------------------------------------------------------------------------
-- The superseded column, marked where a person reading the schema will see it.
-- ---------------------------------------------------------------------------
COMMENT ON COLUMN phieu_phan_anh.hien_cong_khai IS
    'SUPERSEDED by publication_status (migration 0017, ADR 0050 point 8). Kept because rule 7 forbids '
    'a lossy migration. Never read it again and never write it to true; read publication_status.';

COMMENT ON COLUMN phieu_phan_anh.publication_status IS
    'Staff moderation of the public page: cho-duyet | cong-khai | an. Separate from trang_thai. '
    'A can-bo petition is never cong-khai (CHECK phieu_phan_anh_staff_conduct_never_public).';

-- ---------------------------------------------------------------------------
-- WHAT THIS FILE DOES NOT STOP, said plainly: DDL by the table owner (dropping the constraints).
-- Same line ADR 0013 draws for the audit ledger.
--
-- REVERSAL (migration question 3). Run by a person, in ONE transaction; core/migrate has no automatic
-- rollback (ADR 0013).
--
--   Dropping the four constraints and the two comments is lossless at any time:
--
--     ALTER TABLE phieu_phan_anh DROP CONSTRAINT phieu_phan_anh_rating_comment_needs_rating;
--     ALTER TABLE phieu_phan_anh DROP CONSTRAINT phieu_phan_anh_rating_comment_valid;
--     ALTER TABLE phieu_phan_anh DROP CONSTRAINT phieu_phan_anh_staff_conduct_never_public;
--     ALTER TABLE phieu_phan_anh DROP CONSTRAINT phieu_phan_anh_publication_status_valid;
--     COMMENT ON COLUMN phieu_phan_anh.hien_cong_khai IS NULL;
--
--   Dropping the COLUMNS is lossless ONLY WHILE BOTH CHECKS BELOW READ ZERO:
--
--     SELECT count(*) FROM phieu_phan_anh WHERE rating_comment IS NOT NULL;
--     SELECT count(*) FROM phieu_phan_anh
--     WHERE publication_status <> 'cho-duyet'
--       AND NOT (publication_status = 'an' AND linh_vuc = 'can-bo')
--       AND NOT (publication_status = 'cong-khai' AND hien_cong_khai = true);
--
--   ZERO → every non-default value is one this file derived and could derive again, and no citizen
--   has written a comment: `ALTER TABLE phieu_phan_anh DROP COLUMN rating_comment;` and
--   `… DROP COLUMN publication_status;`, then remove this file's row from `schema_migration` in the
--   same transaction, otherwise the runner still believes the schema is in place. The backfill's audit
--   entries STAY — the trail is append-only (rule 6, invariant 4); a reversal is a new act, not an
--   erasure of the old one.
--
--   NON-ZERO → a member of staff has made a moderation decision, or a citizen has written a comment.
--   Dropping either column is destroying part of an archival record — rule 7's stop condition #2, a
--   user decision plus a verified backup, never a command. The way back is then a NEW migration.
-- ---------------------------------------------------------------------------
