-- identity — the staff directory's Mini App columns (open question #12) and the permission key
-- for soft-deleting a duplicated directory row (#10, #27, ADR 0035).
--
-- Every section names the decision it carries out. Nothing here is an agent's reading of what the
-- customer probably meant:
--
--   #12 (DECIDED 2026-09-22) Publishing a staff member to the Zalo Mini App is PER PERSON, never
--       bulk, and needs that person's RECORDED consent. Lean form decided by the user on
--       2026-09-24: the administrator ticks "đã hỏi ý và người này đồng ý"; the system stores
--       WHEN and WHO recorded it. Unpublishing clears the consent marks, so the next publish must
--       ask again; the history of every publish/unpublish lives in `audit_log` (rule 6).
--   #10 / #27 / ADR 0035 — seed `admin.user.delete` now, because the route that checks it lands in
--       the same run. ADR 0035's rule is "seed a key only when a real route needs it".
--
-- WHY A NEW FILE AND NOT AN EDIT TO 0001/0009: core/migrate checksums every applied file at
-- startup and stops the service when one has changed (ErrChecksumLech).
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS, answered before the SQL.
--
-- 1. HOW MANY ROWS PER COMMUNE. Tens, not millions. Since 0009 a write path for `nguoi_dung`
--    exists (internal/store/can_bo_ghi.go, `chenCanBo`), so a development or staging commune MAY
--    now hold rows; the specification's seed is 26 directory entries + 12 accounts per commune.
--    This file REWRITES NONE OF THEM: every column is added with a default, and every default
--    satisfies every constraint below, so no existing row needs a value it does not already have.
--
-- 2. IF IT STOPS HALF-WAY. It cannot land half-applied: core/migrate runs the file in ONE
--    transaction together with its progress row. Every statement is also safe on a second run
--    (ADD COLUMN IF NOT EXISTS, COMMENT ON replaces, constraints guarded by pg_constraint,
--    the seed is ON CONFLICT DO UPDATE).
--
-- 3. HOW IT IS REVERSED. See REVERSAL at the bottom.
--
-- 4. WHICH READ PATHS CHANGE MEANING WHILE IT IS HALF-APPLIED.
--
--      * NO ROW CHANGES VISIBILITY. Nothing here touches `deleted_at`, an index predicate or a
--        WHERE clause. Every existing query returns exactly the rows it returned before.
--      * `hien_tren_mini_app` DEFAULT false IS THE SAFETY PROPERTY. The day the public Mini App
--        directory route starts reading this column, it shows NOBODY until an administrator
--        publishes a person with recorded consent. The opposite default would publish every
--        staff member's personal mobile to a public channel at deploy time — the exact
--        Decree 13/2023/NĐ-CP publication #12 forbids without consent, and one that cannot be
--        undone (a number that reached a public channel has been copied).
--      * The three CHECKs are WRITE-path constraints. No read changes.
--      * `admin.user.delete` is granted to NO role here. Until a commune administrator ticks it on
--        the Phân quyền screen, the route that checks it answers 403 to everyone — the intended
--        closed-by-default state (rule 5).
--
-- 5. RETENTION. Nothing is removed, retyped or renumbered; no issued code is touched. Rule 7's
--    stop conditions are not reached by this file.
--
-- PER-COMMUNE AND RESUMABLE (rule 7, invariant 5): NO BACKFILL — no row is rewritten — so there is
-- nothing to resume and no commune to iterate. §3 still REPORTS per `tenant_id` before adding the
-- constraints, for the one case where it can matter: a re-apply after the REVERSAL below, when
-- rows written in between may no longer satisfy them. Counts and tenant ids only (rule 3).
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- 1. co_zalo and thu_tu_danh_ba — plain directory attributes (docs/ui-ux/12-danh-ba-can-bo.md §7).
--
-- `co_zalo` says the directory's mobile number can be reached on Zalo — the "Có Zalo" sub-line
-- under "Di động" in the list. It describes THE PERSONAL MOBILE (`di_dong_ca_nhan`, 0009 §2), so it
-- travels with that number: shown where the number is shown, published only under the same #12
-- consent. NOT constrained to a non-empty mobile: whether "Có Zalo" may be ticked for an office
-- landline is not decided anywhere, and a constraint would decide it.
--
-- `thu_tu_danh_ba` NULL means "no explicit order" — the list then falls back to its other sort
-- keys (the spec: "sắp theo thu_tu_danh_ba rồi tên bộ phận"). NULL rather than DEFAULT 0 because 0
-- is a real position; a default of 0 would silently put every unordered person in a tie at the top.
-- Negative values are refused: they have no meaning on a form that asks for a display position.
-- ---------------------------------------------------------------------------
ALTER TABLE nguoi_dung
    ADD COLUMN IF NOT EXISTS co_zalo        BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS thu_tu_danh_ba INTEGER;

COMMENT ON COLUMN nguoi_dung.co_zalo IS
    'The directory mobile is reachable on Zalo (docs/ui-ux/12-danh-ba-can-bo.md §7). Describes '
    'di_dong_ca_nhan, so it is shown and published only where that number is, under the same '
    'open question #12 consent. See migration 0010 §1.';

COMMENT ON COLUMN nguoi_dung.thu_tu_danh_ba IS
    'Explicit position in the staff directory / Mini App directory. NULL = no explicit order '
    '(falls back to the other sort keys); never negative. See migration 0010 §1.';

-- ---------------------------------------------------------------------------
-- 2. hien_tren_mini_app + the consent marks — open question #12.
--
-- PUBLISHING A PERSONAL MOBILE TO THE ZALO MINI APP IS PUBLICATION OF PERSONAL DATA ON A PUBLIC
-- CHANNEL under Decree 13/2023/NĐ-CP — a different act from colleagues in one commune seeing each
-- other's numbers (#11). The customer decided it needs the person's own consent, and that the
-- system must keep the evidence: it is the ONLY answer to "căn cứ nào để đưa số tôi lên".
--
--   dong_y_cong_khai_luc      WHEN the administrator recorded that the person agreed.
--   dong_y_cong_khai_ghi_boi  WHO recorded it — the STAFF CODE (`CB-…`, nguoi_dung.ma) of the
--                             administrator, never an internal id. Same reasoning as rule 6
--                             invariant 8: this value is read years later by somebody handling a
--                             complaint, and a staff code names a person with no lookup still
--                             alive; a ULID names nobody.
--
-- `dong_y_cong_khai_ghi_boi` IS `TEXT NOT NULL DEFAULT ''`, NOT `TEXT NULL`, AND THAT IS
-- LOAD-BEARING FOR §3. A CHECK constraint PASSES when its expression is NULL. With a nullable
-- column, `dong_y_cong_khai_ghi_boi <> ''` is NULL for a NULL recorder, the whole CHECK evaluates
-- to NULL, and PostgreSQL ACCEPTS a published row that names no recorder — fail-open on the one
-- constraint this section exists for. One representation of "nobody" ('') closes that, and matches
-- the argument 0009 §2 made for `di_dong_ca_nhan`. The timestamp has no such sentinel, so it stays
-- NULL and §3 tests it with IS NOT NULL / IS NULL explicitly.
--
-- WHAT IS DELIBERATELY NOT HERE:
--   * No consent history table. The decided lean form keeps only the CURRENT consent on the row;
--     every publish and unpublish is its own `audit_log` entry (rule 6), which is the history.
--   * No format CHECK on the recorder (e.g. LIKE 'CB-%'). The staff-code format belongs to
--     domain.SinhMaCanBo; 0009 §4 refused to restate it in the database, and so does this file.
--   * No link to di_dong_ca_nhan being non-empty. A person may be published with only the office
--     landline; nothing decided says otherwise.
-- ---------------------------------------------------------------------------
ALTER TABLE nguoi_dung
    ADD COLUMN IF NOT EXISTS hien_tren_mini_app       BOOLEAN     NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS dong_y_cong_khai_luc     TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS dong_y_cong_khai_ghi_boi TEXT        NOT NULL DEFAULT '';

COMMENT ON COLUMN nguoi_dung.hien_tren_mini_app IS
    'Published to the public Zalo Mini App directory. Publication of personal data on a public '
    'channel under Decree 13/2023/NĐ-CP (open question #12, decided 2026-09-22): PER PERSON, '
    'never bulk, and only with recorded consent — the database refuses true without '
    'dong_y_cong_khai_luc and dong_y_cong_khai_ghi_boi. DEFAULT false is fail-closed. '
    'Read paths must still exclude deleted_at IS NOT NULL. See migration 0010 §2-§3.';

COMMENT ON COLUMN nguoi_dung.dong_y_cong_khai_luc IS
    'When the administrator recorded that this person agreed to Mini App publication (#12). '
    'Set in the same write that publishes; cleared by unpublishing, so the next publish must ask '
    'again. History is in audit_log. See migration 0010 §2.';

COMMENT ON COLUMN nguoi_dung.dong_y_cong_khai_ghi_boi IS
    'STAFF CODE (nguoi_dung.ma, CB-…) of the administrator who recorded the consent — never an '
    'internal id (rule 6, invariant 8). '''' = nobody; NOT NULL on purpose, because a CHECK passes '
    'on NULL. See migration 0010 §2.';

-- ---------------------------------------------------------------------------
-- 3. THE CONSTRAINTS. Fail closed in the database, not only in the application: a psql prompt, a
--    backfill script or a future service that has read none of this must hit the same wall.
--
--   nguoi_dung_cong_khai_phai_co_dong_y   published ⇒ consent recorded (when AND who).
--   nguoi_dung_rut_cong_khai_xoa_dong_y   not published ⇒ no consent marks left behind.
--
-- Together they make the pair a biconditional, and the SECOND one is the decision "unpublishing
-- clears the consent marks" made enforceable. Without it, a write path that forgets to clear the
-- marks leaves a stale consent on the row, and the next publish can reuse it without asking —
-- consent given once, for one publication, silently standing in for a later one. Nothing would
-- report that; the row would look perfectly consented.
--
-- A SOFT-DELETED ROW is not exempt. Soft-deleting a published person does not have to unpublish
-- them (the read paths exclude deleted rows), but whatever state it keeps must still be coherent.
--
-- ON A PARTITIONED TABLE: ADD CONSTRAINT ... CHECK on the parent recurses into all 32 partitions
-- and validates each. Tens of rows per commune: instant.
-- ---------------------------------------------------------------------------
DO $$
DECLARE
    vi_pham int;
    r       record;
BEGIN
    -- PER-COMMUNE REPORT BEFORE THE ATTEMPT. On a first apply every row holds the defaults and
    -- this finds nothing. It exists for a re-apply after the REVERSAL below: `ALTER TABLE` on a bad
    -- row gives one SQLSTATE 23514 naming a partition, which tells an operator nothing about which
    -- commune to look at. Counts and tenant ids only — never a name or a number (rule 3).
    SELECT count(*) INTO vi_pham
      FROM nguoi_dung
     WHERE (hien_tren_mini_app
            AND (dong_y_cong_khai_luc IS NULL OR dong_y_cong_khai_ghi_boi = ''))
        OR (NOT hien_tren_mini_app
            AND (dong_y_cong_khai_luc IS NOT NULL OR dong_y_cong_khai_ghi_boi <> ''))
        OR thu_tu_danh_ba < 0;

    IF vi_pham > 0 THEN
        FOR r IN
            SELECT tenant_id, count(*) AS n
              FROM nguoi_dung
             WHERE (hien_tren_mini_app
                    AND (dong_y_cong_khai_luc IS NULL OR dong_y_cong_khai_ghi_boi = ''))
                OR (NOT hien_tren_mini_app
                    AND (dong_y_cong_khai_luc IS NOT NULL OR dong_y_cong_khai_ghi_boi <> ''))
                OR thu_tu_danh_ba < 0
             GROUP BY tenant_id
             ORDER BY tenant_id
        LOOP
            RAISE NOTICE '0010: commune % has % directory row(s) violating the 0010 constraints',
                r.tenant_id, r.n;
        END LOOP;

        RAISE EXCEPTION
            '% nguoi_dung row(s) would violate the 0010 constraints', vi_pham
            USING HINT = 'Open question #12: a person is on the Mini App only with recorded '
                         'consent (when + staff code of the recorder), and an unpublished row '
                         'carries no consent marks. Whether each row above should be unpublished '
                         'or had its consent recorded is a person''s call, not this migration''s — '
                         'it refuses rather than guessing.';
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
         WHERE conname = 'nguoi_dung_thu_tu_danh_ba_khong_am'
           AND conrelid = 'nguoi_dung'::regclass
    ) THEN
        ALTER TABLE nguoi_dung
            ADD CONSTRAINT nguoi_dung_thu_tu_danh_ba_khong_am
            CHECK (thu_tu_danh_ba IS NULL OR thu_tu_danh_ba >= 0);
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
         WHERE conname = 'nguoi_dung_cong_khai_phai_co_dong_y'
           AND conrelid = 'nguoi_dung'::regclass
    ) THEN
        ALTER TABLE nguoi_dung
            ADD CONSTRAINT nguoi_dung_cong_khai_phai_co_dong_y
            CHECK (NOT hien_tren_mini_app
                   OR (dong_y_cong_khai_luc IS NOT NULL AND dong_y_cong_khai_ghi_boi <> ''));
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
         WHERE conname = 'nguoi_dung_rut_cong_khai_xoa_dong_y'
           AND conrelid = 'nguoi_dung'::regclass
    ) THEN
        ALTER TABLE nguoi_dung
            ADD CONSTRAINT nguoi_dung_rut_cong_khai_xoa_dong_y
            CHECK (hien_tren_mini_app
                   OR (dong_y_cong_khai_luc IS NULL AND dong_y_cong_khai_ghi_boi = ''));
    END IF;
END $$;

-- ---------------------------------------------------------------------------
-- NO NEW INDEX — measured against what the upcoming list filters need, not assumed.
--
--   filter by unit (bo_phan_id)   served by `nguoi_dung_theo_bo_phan`
--                                 ON (tenant_id, bo_phan_id) WHERE deleted_at IS NULL (0001).
--   filter by published           tens of rows per commune. The PRIMARY KEY (tenant_id, id)
--   / the public Mini App list    already narrows to one commune inside one hash partition; the
--                                 remaining filter and the sort by thu_tu_danh_ba run over a few
--                                 dozen rows. A partial index here would cost 32 partition-level
--                                 indexes and a write on every publish for no measurable read gain.
--
-- Revisit only with a measured slow query. Any index added then must start with tenant_id and
-- carry `WHERE deleted_at IS NULL` like its two neighbours.
--
-- SOFT DELETE for TASK-04: `deleted_at`, `deleted_by`, `delete_reason` already exist on
-- nguoi_dung (0001_init.sql:185-187). Nothing to add here (rule 7, invariant 1).
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- 4. admin.user.delete — #10 / #27 / ADR 0035 ("chỉ nạp khoá nào có tuyến thật cần").
--
-- A SEPARATE KEY FROM `admin.user`, on purpose (rule 5, invariant 3b — rights are not a Cartesian
-- product). Managing accounts and removing a row from an archival directory are different acts:
-- the removal is a soft delete of a record the audit trail points at, and a commune may well want
-- it in fewer hands.
--
-- `thu_tu` 36. Measured: 0001 seeds 1-33, 0007 seeds 34-35, and no other file inserts into quyen.
-- The column has no unique constraint, so a collision would raise nothing — it would only make the
-- order on the Phân quyền screen depend on the order PostgreSQL returns rows (0007's header). The
-- screen groups by `nhom`, so the key still appears under QUẢN TRỊ.
--
-- GRANTED TO NO ROLE. The key exists so a commune administrator can tick it; granting it here
-- would be this migration deciding who in a public authority may remove a directory record.
-- ---------------------------------------------------------------------------
INSERT INTO quyen (ma, nhom, nhan, thu_tu) VALUES
    ('admin.user.delete', 'QUẢN TRỊ', 'Xoá dòng danh bạ nhập trùng', 36)
ON CONFLICT (ma) DO UPDATE SET
    nhom   = EXCLUDED.nhom,
    nhan   = EXCLUDED.nhan,
    thu_tu = EXCLUDED.thu_tu;

-- ---------------------------------------------------------------------------
-- REVERSAL (rule 7, invariant 4).
--
-- core/migrate has no automatic rollback, on purpose: undoing DDL on archival records is an
-- administrative act carried out with a person present. The reverse is written here, by hand.
--
-- THE CONSTRAINTS hold no data and reverse losslessly:
--
--      ALTER TABLE nguoi_dung DROP CONSTRAINT IF EXISTS nguoi_dung_cong_khai_phai_co_dong_y;
--      ALTER TABLE nguoi_dung DROP CONSTRAINT IF EXISTS nguoi_dung_rut_cong_khai_xoa_dong_y;
--      ALTER TABLE nguoi_dung DROP CONSTRAINT IF EXISTS nguoi_dung_thu_tu_danh_ba_khong_am;
--
--   Dropping the first two is, however, RE-OPENING #12 rather than a technical undo: it allows a
--   personal mobile on a public channel with no recorded consent. That needs the customer.
--
-- THE COMMENTS hold no data.
--
-- THE KEY: an unused key is harmless; a key already granted to roles is referenced by
-- vai_tro_quyen (FK to quyen.ma), and removing it would silently revoke a right administrators
-- granted. Not written as a runnable line.
--
-- THE FIVE COLUMNS ARE ADDITIVE: "reverting" them is achieved by ceasing to read them, at no cost.
-- Actually DROPPING one once it holds data is a different act: `dong_y_cong_khai_luc` and
-- `dong_y_cong_khai_ghi_boi` are the ONLY evidence of the consent #12 requires, and dropping them
-- destroys the answer to "căn cứ nào để đưa số tôi lên" for every person already published. Rule 7
-- stop condition #2: explicit user decision plus a verified backup. Not runnable on purpose:
--
--      -- ALTER TABLE nguoi_dung DROP COLUMN dong_y_cong_khai_ghi_boi;  -- USER DECISION + BACKUP ONLY
--      -- ALTER TABLE nguoi_dung DROP COLUMN dong_y_cong_khai_luc;      -- USER DECISION + BACKUP ONLY
--      -- ALTER TABLE nguoi_dung DROP COLUMN hien_tren_mini_app;        -- USER DECISION + BACKUP ONLY
--      -- ALTER TABLE nguoi_dung DROP COLUMN thu_tu_danh_ba;            -- USER DECISION + BACKUP ONLY
--      -- ALTER TABLE nguoi_dung DROP COLUMN co_zalo;                   -- USER DECISION + BACKUP ONLY
--
-- AND AFTERWARDS, for the file to be applied again, its progress row has to be removed from
-- `schema_migration`, keyed on ten = '0010_danh_ba_mini_app_va_khoa_xoa_dong_trung.sql'. Prose,
-- not a runnable line, for the same reason.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- BACKSTOP, carried forward from 0002–0009: a partitioned table with no partitions rejects every
-- INSERT. This file adds no table, so it is expected to find nothing; it runs anyway, because the
-- check only describes the state after the newest migration that carries it.
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
