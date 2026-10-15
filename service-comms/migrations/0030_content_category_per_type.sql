-- 0030 — Mini App content categories: ONE TREE PER CONTENT TYPE instead of one tree per commune.
--
-- WHY THIS FILE EXISTS. Project owner, 14/10/2026 (PM requirement), overriding ADR 0067 §3 ("one tree per
-- commune"): each of `tin-tuc`, `su-kien`, `video`, `truyen-thanh`, `thong-bao` has its OWN category tree.
-- A news category is no longer offered when filing an event. Decided in the same breath:
--
--   * `banner` HAS NO CATEGORY. It is therefore NOT a value of the new column, and the CHECK below refuses
--     it — a "banner tree" would be a tree nothing may be filed under.
--   * Names are unique per (type, parent). That rule is the WRITE PATH's (TASK-02), under the commune's
--     tree lock, exactly where today's per-parent name rule lives; this file adds no key for it (a partial
--     UNIQUE is refused by tools/check_khoa_duy_nhat.py, and a non-partial one would hold a deleted name
--     for ever).
--   * The slug stays unique PER COMMUNE — 0006's UNIQUE (tenant_id, slug) is NOT touched. A slug is a
--     business code held by every article filed under it; it is never reissued (rule 7, invariant 3),
--     and narrowing it to (tenant_id, loai, slug) would let two trees issue the same code.
--   * The owner confirmed there is NO category data in any environment. This file does not RELY on that:
--     it backfills deterministically, audits every row it writes, and reports what it found (below).
--
-- THE COLUMN IS NAMED `loai`, NOT `type`. It holds exactly the values of `noi_dung_mini_app.loai` (0006),
-- minus `banner`, and the two are compared on every write; one concept under two column names in one
-- module is the drift rule 12 invariant 3 / forbidden #2 exist to prevent. ENUM VALUES stay Vietnamese
-- without diacritics (ADR 0011). Every OTHER new name here (constraint, index) is English.
--
-- NO COMPOSITE FOREIGN KEY item -> (category, type). It would need UNIQUE (tenant_id, id, loai) on the
-- category and a matching column pair on `noi_dung_mini_app`, and it would make the item's `loai` and its
-- category move in lock-step on an EDITABLE column (0006:454: `loai` stays editable) — an edit changing an
-- item's type would have to clear or move its category in the same statement or be refused by the
-- database with a message nobody on the screen can act on. The rule "an item's category has the item's
-- type, and a banner has none" is enforced IN THE APPLICATION, under the commune's tree lock
-- (pg_advisory_xact_lock, the same lock today's re-parenting takes), at create and at edit (TASK-02).
-- Rows that already break it are COUNTED below, not rewritten.
--
-- WHY A NEW FILE AND NOT AN EDIT OF 0006 / 0012: core/migrate compares the checksum of every applied file
-- at startup (ErrChecksumLech).
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. ROWS PER COMMUNE: every `danh_muc_mini_app` row whose `loai` is NULL — i.e. every category, once,
--      soft-deleted ones included (NOT NULL needs a value on every row, and a deleted category is still
--      an archival record that must read the same way if it is ever inspected). 0006's figure is ~60 per
--      commune; the owner's statement is zero everywhere today. One audit entry per row written. NO ROW
--      of `noi_dung_mini_app` is written. The operator may count first:
--        SELECT tenant_id, count(*) FROM danh_muc_mini_app GROUP BY tenant_id;
--   2. IF IT STOPS HALF-WAY: it cannot land half-applied — one file, one transaction, with the progress
--      row. ONE STATEMENT, NOT A PER-COMMUNE LOOP, for the reason service-petitions 0025 (lines 62-66)
--      gives: core/migrate gives the file one transaction, so a loop inside it could not be resumed
--      either, and core/migrate has no per-commune resumable mechanism. Every row carries its own
--      tenant_id into its audit entry, and the NOTICE lines report per commune. A retry costs nothing:
--      ADD COLUMN IF NOT EXISTS, the backfill only matches `loai IS NULL`, the CHECK is added behind a
--      pg_constraint lookup, the index is IF NOT EXISTS.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. READ PATHS THAT CHANGE MEANING WHILE IT IS HALF-APPLIED: none can see it half-applied (one
--      transaction). AFTER IT, BEFORE THE GO CHANGE (TASK-02) — the part that matters:
--        * READS: unchanged. internal/store names its columns explicitly (no `SELECT *` in service-comms,
--          0011 question 4), so no list, filter or tree read sees `loai` until it asks. Until TASK-02
--          filters by type, the category picker still shows ALL trees together, as today.
--        * WRITES: today's category INSERT (internal/store/noi_dung_mini_app.go `chenDanhMucMiniApp`)
--          does not name `loai`, so after this file EVERY CATEGORY CREATE IS REFUSED by NOT NULL until
--          TASK-02 ships. That is deliberate: a DEFAULT would let a writer that never chose a type file
--          the category under `tin-tuc` silently — the guess this file refuses to make on the write path.
--          THIS FILE AND TASK-02 SHIP TOGETHER; the embedded migration runs on the release carrying it.
--        * Item <-> category type mismatches that already exist (an item of another type, or a banner,
--          filed under a category) stay as they are, and are counted in the NOTICE lines. TASK-02's
--          edit path must tolerate reading them and must not refuse an unrelated edit because of them.
--        * The old tree index `danh_muc_mini_app_cay` (0006:219-221) is KEPT: it still serves today's
--          reads that do not name a type. Retiring it is a later migration, once no query lacks `loai`.
--   5. RETENTION: a category is business data (rule 7). Nothing is dropped, retyped, emptied or
--      renumbered; the slug key is untouched; no item is refiled. The only rows written are new values
--      in a column created here, each with its audit entry (rule 6, invariant 6: system principal).
--
-- THE LOCK: ADD COLUMN, SET NOT NULL and ADD CONSTRAINT take ACCESS EXCLUSIVE on `danh_muc_mini_app`
-- until COMMIT; SET NOT NULL and the CHECK each scan the table once. `noi_dung_mini_app`
-- is only READ (by the backfill). Acceptable at ~60 rows per commune — AN ASSUMPTION, NOT A MEASUREMENT:
--   SELECT count(*), pg_size_pretty(pg_total_relation_size('danh_muc_mini_app')) FROM danh_muc_mini_app;
--
-- PERSONAL DATA (rule 3): none. A category name is how the authority files its own news
-- (internal/app/noi_dung_mini_app.go, the note above the create audit); the NOTICE lines carry tenant ids
-- and counts only, the audit delta carries the type and the basis for it.
--
-- PG FLOOR: 13, already enforced by 0006. Cluster: 16.
-- ---------------------------------------------------------------------------

-- vi-name-ok: same column name and same values as noi_dung_mini_app.loai (0006), one concept, one name
ALTER TABLE danh_muc_mini_app ADD COLUMN IF NOT EXISTS loai TEXT;

-- ---------------------------------------------------------------------------
-- THE BACKFILL — deterministic, filtered, audited in the same statement (rule 6, invariant 3).
--
-- For every category whose `loai` is NULL:
--
--   the LIVE (deleted_at IS NULL), NON-BANNER items filed under it have EXACTLY ONE distinct `loai`
--                                     → that `loai`              (basis 'theo-noi-dung')
--   anything else — no such item, or two or more types among them
--                                     → 'tin-tuc'                (basis 'mac-dinh')
--
-- WHY `tin-tuc` AS THE FALLBACK: the only screen that has ever created a category labels it "Danh mục
-- tin" (docs/ui-ux/11-noi-dung-mini-app.md §6, the `⊞ Danh mục tin` button), so a category nobody can
-- read a type from was created as a news category.
--
-- WHY BANNERS ARE LEFT OUT OF THE COUNT: the owner decided a banner has no category, so a banner filed
-- under one says nothing about which tree the category belongs to. It is still COUNTED as a mismatch below.
--
-- Soft-deleted ITEMS are left out of the count too: a removed article does not decide where a live tree
-- sits. Soft-deleted CATEGORIES are backfilled like live ones (question 1).
--
-- The audit verb is the existing category-edit verb (`sua_danh_muc_mini_app`, internal/app
-- ActionUpdateContentCategory), subject = the slug, the business code (rule 6, invariant 8), actor
-- 'system' / kind 'system' — the shape service-petitions 0025 uses for a system backfill.
-- ---------------------------------------------------------------------------
DO $$
DECLARE
    written         int;
    audited         int;
    summary         jsonb;
    r               record;
BEGIN
    -- ONE statement: the UPDATE, its audit entries and the per-commune summary. Data-modifying WITH
    -- clauses run once and to completion whether or not the outer SELECT reads them; the summary is taken
    -- from `changed` itself, so this never re-scans the audit ledger.
    WITH item_types AS (
        SELECT i.tenant_id, i.danh_muc_id,
               count(DISTINCT i.loai) AS type_count,
               min(i.loai)            AS only_type
        FROM noi_dung_mini_app i
        WHERE i.danh_muc_id IS NOT NULL
          AND i.deleted_at IS NULL
          AND i.loai <> 'banner'
        GROUP BY i.tenant_id, i.danh_muc_id
    ), target AS (
        SELECT c.tenant_id, c.id,
               CASE WHEN t.type_count = 1 THEN t.only_type ELSE 'tin-tuc' END         AS new_type,
               CASE WHEN t.type_count = 1 THEN 'theo-noi-dung' ELSE 'mac-dinh' END    AS basis,
               coalesce(t.type_count, 0)                                              AS type_count
        FROM danh_muc_mini_app c
        LEFT JOIN item_types t ON t.tenant_id = c.tenant_id AND t.danh_muc_id = c.id
        WHERE c.loai IS NULL
    ), changed AS (
        UPDATE danh_muc_mini_app d
        SET loai = t.new_type, cap_nhat_luc = now()
        FROM target t
        WHERE d.tenant_id = t.tenant_id AND d.id = t.id
          AND d.loai IS NULL
        RETURNING d.tenant_id, d.slug, t.new_type, t.basis, t.type_count
    ), logged AS (
        INSERT INTO audit_log (tenant_id, actor_id, actor_kind, actor_ip, action, subject, at, delta)
        SELECT tenant_id, 'system', 'system', '', 'sua_danh_muc_mini_app', slug, now(),
               jsonb_build_object(
                   'truoc',  jsonb_build_object('loai', NULL::text),
                   'sau',    jsonb_build_object('loai', new_type),
                   'can_cu', basis,
                   'so_loai_noi_dung', type_count,
                   'ly_do',  'migration 0030: one category tree per content type (owner 14/10/2026)')
        FROM changed
        RETURNING 1
    )
    SELECT coalesce(sum(s.n), 0),
           (SELECT count(*) FROM logged),
           coalesce(jsonb_agg(to_jsonb(s) ORDER BY s.tenant_id), '[]'::jsonb)
      INTO written, audited, summary
      FROM (SELECT tenant_id,
                   count(*)                                                    AS n,
                   count(*) FILTER (WHERE basis = 'theo-noi-dung')             AS by_items,
                   count(*) FILTER (WHERE basis = 'mac-dinh')                  AS by_default,
                   count(*) FILTER (WHERE basis = 'mac-dinh' AND type_count > 1) AS mixed
              FROM changed
             GROUP BY tenant_id) s;

    -- A row moved without its entry is the state rule 6 invariant 3 forbids. Cannot happen in one
    -- statement; checked anyway, because the check costs nothing and the failure would be silent.
    IF audited <> written THEN
        RAISE EXCEPTION '0030: % categor(ies) typed but % audit entr(ies) written', written, audited;
    END IF;

    RAISE NOTICE '0030: % categor(ies) given a type', written;

    -- Per commune: how many took the type of their items, how many fell back to tin-tuc, and how many of
    -- those fell back because their items had TWO OR MORE types (the case a person may want to look at).
    FOR r IN
        SELECT * FROM jsonb_to_recordset(summary)
            AS x(tenant_id text, n int, by_items int, by_default int, mixed int)
    LOOP
        RAISE NOTICE '0030: commune %: % by items, % defaulted to tin-tuc (% of them had mixed item types)',
            r.tenant_id, r.by_items, r.by_default, r.mixed;
    END LOOP;

    -- WHAT THIS FILE LEAVES INCONSISTENT, measured rather than fixed (no item is refiled; question 4):
    -- live items whose category is of another type — banners filed under a category included — and child
    -- categories whose parent sits in another tree.
    FOR r IN
        SELECT i.tenant_id,
               count(*) FILTER (WHERE i.loai = 'banner')  AS banners_filed,
               count(*) FILTER (WHERE i.loai <> 'banner') AS other_type
        FROM noi_dung_mini_app i
        JOIN danh_muc_mini_app c ON c.tenant_id = i.tenant_id AND c.id = i.danh_muc_id
        WHERE i.deleted_at IS NULL
          AND i.loai IS DISTINCT FROM c.loai
        GROUP BY i.tenant_id
        ORDER BY i.tenant_id
    LOOP
        RAISE NOTICE '0030: commune %: % live banner(s) filed under a category, % live item(s) under a category of another type — left as they are',
            r.tenant_id, r.banners_filed, r.other_type;
    END LOOP;

    FOR r IN
        SELECT ch.tenant_id, count(*) AS n
        FROM danh_muc_mini_app ch
        JOIN danh_muc_mini_app p ON p.tenant_id = ch.tenant_id AND p.id = ch.cha_id
        WHERE ch.loai IS DISTINCT FROM p.loai
        GROUP BY ch.tenant_id
        ORDER BY ch.tenant_id
    LOOP
        RAISE NOTICE '0030: commune %: % categor(ies) whose parent is in another type tree — left as they are',
            r.tenant_id, r.n;
    END LOOP;
END $$;

-- ---------------------------------------------------------------------------
-- NOT NULL, then the CHECK. NO DEFAULT, on purpose (question 4): the writer names the tree.
--
-- The CHECK is the five category-bearing values of 0006's `noi_dung_mini_app_loai_hop_le`, in its order,
-- WITHOUT `banner`. VALIDATED, not NOT VALID (0011's reasons: not portable on a partitioned table, and no
-- lock to save — ADD COLUMN already holds ACCESS EXCLUSIVE until COMMIT). Every value it validates was
-- written by the backfill above from that same list.
-- ---------------------------------------------------------------------------
ALTER TABLE danh_muc_mini_app ALTER COLUMN loai SET NOT NULL;

DO $$
DECLARE parent oid := 'danh_muc_mini_app'::regclass;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = parent
                   AND conname = 'danh_muc_mini_app_type_known') THEN
        ALTER TABLE danh_muc_mini_app ADD CONSTRAINT danh_muc_mini_app_type_known
            CHECK (loai IN ('tin-tuc', 'su-kien', 'thong-bao', 'truyen-thanh', 'video'));
    END IF;
END $$;

-- ---------------------------------------------------------------------------
-- The tree read of ONE type: children of one parent inside one type's tree, in the commune's order.
-- Leads with tenant_id (every index does). NOT UNIQUE, so the partial predicate is legitimate — it decides
-- how many rows are read, not whether a code can be issued again (tools/check_khoa_duy_nhat.py refuses
-- that shape for UNIQUE indexes only). ADDED beside 0006's `danh_muc_mini_app_cay`, not replacing it
-- (question 4).
-- ---------------------------------------------------------------------------
CREATE INDEX IF NOT EXISTS danh_muc_mini_app_tree_by_type
    ON danh_muc_mini_app (tenant_id, loai, cha_id, thu_tu)
    WHERE deleted_at IS NULL;

-- ---------------------------------------------------------------------------
-- WHAT THIS FILE DOES NOT STOP: an item filed under a category of another type, a banner filed under a
-- category, a child in another type's tree than its parent — all three are TASK-02's write path, under the
-- tree lock (header). TRUNCATE and DDL by the table owner, as 0006 / ADR 0013 say.
--
-- REVERSAL (question 3). Run by a person, in ONE transaction; core/migrate has no automatic rollback
-- (ADR 0013), and there is no down file because core/migrate would apply it as the next migration
-- (0011:532-535). Prose, because a runnable line is a line that gets run.
--
-- LOSSLESS ONLY WHILE every category's `loai` is still the value this file wrote — i.e. no category has
-- been created since (TASK-02 writes `loai` on create) and none has been moved to another tree. Check:
--
--   SELECT count(*) FROM danh_muc_mini_app c
--    WHERE NOT EXISTS (SELECT 1 FROM audit_log a
--                      WHERE a.tenant_id = c.tenant_id AND a.subject = c.slug
--                        AND a.action = 'sua_danh_muc_mini_app' AND a.actor_kind = 'system'
--                        AND a.delta->>'ly_do' LIKE 'migration 0030:%'
--                        AND a.delta->'sau'->>'loai' = c.loai)
--       OR EXISTS (SELECT 1 FROM audit_log a
--                  WHERE a.tenant_id = c.tenant_id AND a.subject = c.slug
--                    AND a.action IN ('them_danh_muc_mini_app', 'sua_danh_muc_mini_app')
--                    AND a.actor_kind <> 'system'
--                    AND a.delta->'sau' ? 'loai');
--
-- ZERO → in this order: drop the index danh_muc_mini_app_tree_by_type; drop the constraint
-- danh_muc_mini_app_type_known; drop the column `loai` of danh_muc_mini_app; remove this file's row from
-- `schema_migration` (ten = '0030_content_category_per_type.sql'). The audit entries written above are
-- NOT deleted (rule 6, forbidden #3) — they stay as the record that the column existed and what it held;
-- the reversal writes no entry of its own because it removes no value a person chose. The Go code that
-- reads one tree per type (TASK-02) must be rolled back with the schema.
--
-- NON-ZERO → a member of staff has filed a category into a tree. Dropping `loai` destroys that filing:
-- rule 7 stop condition #2 — the user plus a verified backup, never a command — and the way back is a NEW
-- migration.
-- ---------------------------------------------------------------------------
