-- 0013 — PORTAL SYNC: the commune's public portal (Cổng thông tin điện tử, the Đà Nẵng `cttdt` API)
-- feeds the Mini App content register (docs/ui-ux/11-noi-dung-mini-app.md §3, §8, §10). Project
-- owner's decisions of 01/10/2026 (the ADR recording them is being written alongside this file).
--
-- WHY THIS FILE EXISTS. 0006:61-84 declined all three sync tables, each for a stated reason. Each
-- reason is now answered:
--   * `cau_hinh_dong_bo_cong` had no way to hold the portal key — "Cần core/crypto — chưa tồn tại".
--     core/crypto exists now and 0008 (mail_settings) is the precedent this file follows EXACTLY:
--     the key is core/crypto.Envelope output under the commune's DEK in `data_encryption_key`.
--   * `chuyen_muc_cong` — "whether it is a table at all is not yet a settled question". The owner
--     settled it: a table holding the commune's SELECTION and MAPPING only, not a mirror of the
--     portal's tree (the tree is still asked of the portal when the modal opens).
--   * "no sync run and nothing that records one" — the owner decided the job is built; the run log
--     below is its record.
--
-- NAMED IN ENGLISH (rule 12, ADR 0051): `cau_hinh_dong_bo_cong` → `portal_sync_settings`,
-- `chuyen_muc_cong` → `portal_categories`, and the run record is `portal_sync_runs`. ENUM VALUES stay
-- Vietnamese without diacritics (ADR 0011:47 — they sit in the database and in the API, and are not
-- translated): `dang-thang` / `cho-duyet` (§8's own), `theo-lich` / `chay-tay` for what started a
-- run, `thanh-cong` / `mot-phan` / `that-bai` for how it ended.
--
-- `portal_categories` IS NOT `danh_muc_mini_app`, AND THE TWO MUST NEVER BE MERGED (0006:141-146).
-- This table is a FOREIGN system's list as last seen, plus the commune's choice of which entries to
-- take and as what `loai`. No foreign key, no join, no shared id between the two.
--
-- WHY A NEW FILE: core/migrate compares the checksum of every applied file at startup.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. ROWS PER COMMUNE: this file writes NO row. Steady state: `portal_sync_settings` at most ONE
--      (tenant_id is the key); `portal_categories` ~60 (§3's sample portal); `portal_sync_runs` one per
--      run — every 6 h by default is ~1 460 a year, plus manual runs. `noi_dung_mini_app` gains one
--      NULLABLE column with no default: catalogue-only, no row rewritten.
--   2. IF IT STOPS HALF-WAY: it cannot land half-applied — one file, one transaction, progress row
--      inside it. Every statement is IF NOT EXISTS / CREATE OR REPLACE / DROP TRIGGER IF EXISTS or a
--      constraint behind a pg_constraint lookup pinned to the parent; a retry costs nothing. No
--      backfill, so nothing to resume per commune.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom — exact SQL and its lossless condition.
--   4. READ PATHS THAT CHANGE MEANING WHILE HALF-APPLIED: none can see it half-applied. After it:
--      three NEW tables nothing reads yet, and `noi_dung_mini_app.portal_category_id`, NULL on every
--      row; internal/store names its columns explicitly, so no existing read sees it. No sync writer
--      exists before the Go change, so no row anywhere has `nguon = 'dong-bo-cong'` (0006:303-306) and
--      the new CHECK binding the column to that provenance is satisfied by every existing row.
--   5. RETENTION: settings — never deleted (trigger), turned off with `is_enabled`. Categories — never
--      deleted (trigger): imported items point at them, and the name a resident saw an article filed
--      under must stay resolvable. Runs — append-only except the single finish fill (trigger), never
--      deleted: the run log is the record of what the commune's channel published unattended.
--      Nothing existing is dropped, retyped or renumbered; `nguon_id_ngoai`'s non-partial UNIQUE
--      (0006:343-358) is untouched and keeps counting deleted rows, so a removed article is never
--      re-imported.
--
-- PG FLOOR: 13 (BEFORE ... FOR EACH ROW triggers on partitioned tables; 0006 checks it) and 12 for a
-- foreign key that references a partitioned table. The cluster is 16. Nothing here needs more.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- portal_sync_settings_guard — refuses DELETE and a change of commune; 0008's mail_settings_guard
-- with this table's hint. A separate function rather than mail_settings_guard itself, because that
-- one's hint names the mail server and an operator reading it here would be told the wrong thing.
--
-- NO SOFT-DELETE COLUMNS, for 0008:93-98's reason: one configuration row per commune, overwritten in
-- place; the screen turns it off (`Đang bật` / `Đang tắt`, is_enabled). Every change is an audit
-- entry with before/after (rule 6) — never the sealed key, which is reported only as set / not set.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION portal_sync_settings_guard() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION '%: delete refused', TG_TABLE_NAME
            USING HINT = 'Turn the portal sync off with is_enabled = false instead.';
    END IF;
    IF NEW.tenant_id IS DISTINCT FROM OLD.tenant_id THEN
        RAISE EXCEPTION '%: tenant_id is immutable', TG_TABLE_NAME
            USING HINT = 'The sealed portal key is bound to its commune and does not open elsewhere.';
    END IF;
    RETURN NEW;
END $$;

-- ---------------------------------------------------------------------------
-- @entity: PortalSyncSettings
-- @scope:  tenant
--
-- portal_sync_settings — how ONE commune's portal is read (§3's `Cấu hình` modal, §8's
-- `cau_hinh_dong_bo_cong`). One row per commune.
--
-- THE PORTAL KEY IS NEVER STORED IN THE CLEAR. `api_key_sealed` is core/crypto.Envelope output —
-- AES-256-GCM under the commune's DEK (`data_encryption_key`, 0008), the version byte and nonce
-- INSIDE the value, bound by its additional data to this table, this column and this commune (the
-- shape internal/app/mail_settings.go uses: "<table>/<column>/<tenant_id>"). There is no key-id
-- column, exactly as mail_settings has none: the KEK id lives on the DEK row, and a rotation re-wraps
-- the DEK without touching this value. It is never returned to a client — §3's `Đang dùng ****654bf`
-- would need the plaintext to draw, so the screen reports only that a key is set.
--
-- THE BOUNDS ON THE THREE NUMBERS ARE VENDOR GUARDS AGAINST A BOGUS VALUE, NOT CUSTOMER FIGURES. The
-- DEFAULTS are the owner's (01/10/2026). Raising a bound is one migration; lowering it after a
-- commune saved a larger value is not.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS portal_sync_settings (
    tenant_id          TEXT        NOT NULL,

    -- Which portal API dialect. ONE today; a second province's portal is a new value AND a new
    -- adapter, never a guess at the first one's format.
    provider           TEXT        NOT NULL,

    -- §3: `https://thangbinh.danang.gov.vn/DesktopModules/cttdt/api/apichiase`. The service calls
    -- it from inside the cluster, so it is an SSRF surface: the CHECK pins https and a host under
    -- `.gov.vn`; the Go adapter re-checks the parsed host (and refuses an IP literal / redirect off
    -- `.gov.vn`), because a regular expression is a floor, not a URL parser.
    api_url            TEXT        NOT NULL,

    api_key_sealed     BYTEA       NOT NULL,

    -- §10.2: `dang-thang` lands imported articles `dang-hien`, `cho-duyet` lands them `cho-duyet`.
    -- The DEFAULT is the cautious one: nothing reaches residents until a member of staff approves it.
    publish_mode       TEXT        NOT NULL DEFAULT 'cho-duyet',

    -- §3's `Mỗi 6 giờ`. 0 = MANUAL ONLY: the scheduler never starts a run, `⟳ Đồng bộ ngay` still does.
    interval_hours     INT         NOT NULL DEFAULT 6,
    -- How far back a run looks on the portal, in days.
    window_days        INT         NOT NULL DEFAULT 90,
    -- Ceiling on items IMPORTED by one run, so a first run against a portal holding thousands of
    -- articles (§3: `bỏ qua 3563`) cannot flood the register — or the residents, in `dang-thang`.
    max_items_per_run  INT         NOT NULL DEFAULT 100,
    -- Imported articles keep a credit line / link to the portal original (`nguon_url`).
    keep_source_credit BOOLEAN     NOT NULL DEFAULT true,

    is_enabled         BOOLEAN     NOT NULL DEFAULT false,

    -- WHEN THE LAST RUN STARTED — the scheduler's due-check, written in the same transaction that
    -- inserts the run's row in portal_sync_runs. THE RUN LOG IS THE SOURCE OF TRUTH for what a run
    -- did; this column says only "when was one last started", so a screen reading outcomes from here
    -- would be reading a second copy. NULL = never run.
    last_run_at        TIMESTAMPTZ,

    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- The BUSINESS CODE of the last person to save (rule 6, invariant 8) — never an internal id.
    -- The scheduler's write of last_run_at does not change it: that is not a configuration change.
    updated_by         TEXT        NOT NULL,

    PRIMARY KEY (tenant_id),

    CONSTRAINT portal_sync_settings_provider_known CHECK (provider IN ('cttdt-danang')),

    -- https only; host = one or more labels then `gov.vn`; optional port; then path/query, no
    -- whitespace or control characters; no userinfo (`@` cannot appear before the host ends — the host
    -- class has no `@`, and the host must END in `.gov.vn` right before `:`, `/` or the end).
    CONSTRAINT portal_sync_settings_api_url_shape CHECK (
        char_length(api_url) <= 2048
        AND api_url !~ '[[:space:][:cntrl:]\\]'
        AND api_url ~* '^https://([a-z0-9-]+\.)+gov\.vn(:[0-9]{1,5})?(/.*)?$'),

    -- 1 + 12 + 16 bytes of envelope overhead, plus at least one byte of key (0008:150-151).
    CONSTRAINT portal_sync_settings_api_key_sealed_length CHECK (octet_length(api_key_sealed) > 29),

    CONSTRAINT portal_sync_settings_publish_mode_known CHECK (publish_mode IN ('cho-duyet', 'dang-thang')),
    CONSTRAINT portal_sync_settings_interval_range CHECK (interval_hours BETWEEN 0 AND 24),
    CONSTRAINT portal_sync_settings_window_range CHECK (window_days BETWEEN 1 AND 3650),
    CONSTRAINT portal_sync_settings_max_items_range CHECK (max_items_per_run BETWEEN 1 AND 1000),
    CONSTRAINT portal_sync_settings_updated_by_present CHECK (btrim(updated_by) <> '')
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS portal_sync_settings_p%s PARTITION OF portal_sync_settings '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

DROP TRIGGER IF EXISTS portal_sync_settings_guard ON portal_sync_settings;
CREATE TRIGGER portal_sync_settings_guard
    BEFORE UPDATE OR DELETE ON portal_sync_settings
    FOR EACH ROW EXECUTE FUNCTION portal_sync_settings_guard();

-- ---------------------------------------------------------------------------
-- portal_categories_guard — what may move on a category row.
--
--   DELETE                refused: imported items point at the row (portal_category_id), and the
--                         category name they were shown under must stay resolvable (rule 7).
--   tenant_id, id,        frozen. `external_id` is the PORTAL's handle; moving it would re-point
--   external_id,          every item already imported from the old category at a different one.
--   created_at/by
--   name, target_kind,    editable: `name` is "as last seen" and the portal may rename; `target_kind`
--   is_selected, updated  is the commune's mapping and applies to FUTURE imports only — an item
--                         already imported keeps the `loai` it was imported as.
-- Messages name the operation and the relation only (rule 3, forbidden #3).
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION portal_categories_guard() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'administrative record %: hard delete refused', TG_TABLE_NAME
            USING HINT = 'Stop taking a portal category with is_selected = false. Imported items '
                         'still point at the row.';
    END IF;
    IF NEW.tenant_id   IS DISTINCT FROM OLD.tenant_id
    OR NEW.id          IS DISTINCT FROM OLD.id
    OR NEW.external_id IS DISTINCT FROM OLD.external_id
    OR NEW.created_at  IS DISTINCT FROM OLD.created_at
    OR NEW.created_by  IS DISTINCT FROM OLD.created_by THEN
        RAISE EXCEPTION 'administrative record %: identity columns are immutable', TG_TABLE_NAME
            USING HINT = 'A different portal category is a new row (UNIQUE (tenant_id, external_id)).';
    END IF;
    RETURN NEW;
END $$;

-- ---------------------------------------------------------------------------
-- @entity: PortalCategory
-- @scope:  tenant
--
-- portal_categories — the commune's CHOICE among its portal's categories, and the `loai` each chosen
-- one is imported as (§3's checkbox + `ánh xạ loại nội dung`; §8's `chuyen_muc_cong`).
--
-- NOT A MIRROR. A row exists for a category the commune has selected at least once; the tree the modal
-- draws is still asked of the portal on open. That is why there is no parent column (the tree is the
-- portal's, and this table would only hold a stale copy of it) and no row for a category never chosen.
--
-- A FLAG, NOT A SOFT DELETE: unticking a category is `is_selected = false`; ticking it again flips it
-- back on the SAME row, so items imported under it before and after share one category. A deleted_at
-- beside is_selected would be two ways to say "not taken", and two readers would disagree (the
-- argument service-finance 0012 makes for its closes).
--
-- `target_kind` IS THREE OF THE SIX `loai` — the owner's decision of 01/10/2026: §3 lists all six, but
-- a portal article is imported as news, an event or a notice. `truyen-thanh`, `video` and `banner`
-- need a file, a link or an image the portal API does not carry.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS portal_categories (
    tenant_id     TEXT        NOT NULL,
    id            TEXT        NOT NULL,          -- ULID, internal

    -- THE PORTAL'S OWN id for the category (§8's `ma_chuyen_muc`), as its API returns it.
    external_id   TEXT        NOT NULL,

    -- The category's name AS LAST SEEN on the portal — refreshed by the modal / a run, never typed by
    -- staff. A portal category name is an editorial label ("Nội chính › Công an"), not personal data.
    name          TEXT        NOT NULL,

    target_kind   TEXT        NOT NULL,
    is_selected   BOOLEAN     NOT NULL DEFAULT true,

    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- Business codes (rule 6, invariant 8); 'system' when a run refreshes `name`.
    created_by    TEXT        NOT NULL,
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by    TEXT        NOT NULL,

    PRIMARY KEY (tenant_id, id),

    -- Composite with tenant_id (rule 1, invariant 6): two communes' portals hand out the same ids.
    -- NOT partial — there is no soft delete, and one portal category is one row for ever.
    UNIQUE (tenant_id, external_id),

    CONSTRAINT portal_categories_external_id_shape CHECK (
        btrim(external_id) <> '' AND char_length(external_id) <= 200
        AND external_id !~ '[[:cntrl:]]'),
    CONSTRAINT portal_categories_name_shape CHECK (
        btrim(name) <> '' AND char_length(name) <= 500 AND name !~ '[[:cntrl:]]'),
    CONSTRAINT portal_categories_target_kind_known
        CHECK (target_kind IN ('tin-tuc', 'su-kien', 'thong-bao')),
    CONSTRAINT portal_categories_signed CHECK (btrim(created_by) <> '' AND btrim(updated_by) <> '')
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS portal_categories_p%s PARTITION OF portal_categories '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

DROP TRIGGER IF EXISTS portal_categories_guard ON portal_categories;
CREATE TRIGGER portal_categories_guard
    BEFORE UPDATE OR DELETE ON portal_categories
    FOR EACH ROW EXECUTE FUNCTION portal_categories_guard();

-- ---------------------------------------------------------------------------
-- portal_sync_run_finish_once — a run row never changes, except ONE fill of its outcome.
--
-- DENY BY DEFAULT, service-finance 0012's budget_period_close_immutable shape: the WHOLE ROW minus the
-- finish columns must be unchanged, so a column a later migration adds is frozen the day it exists.
-- The finish columns may go NULL → value ONCE (the all-or-none CHECK keeps them together), and never
-- again. A run that crashed is left unfinished; a later reaper may finish it as `that-bai` — that is
-- still the one fill.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION portal_sync_run_finish_once() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF (to_jsonb(NEW) - 'finished_at' - 'outcome' - 'fetched_count' - 'imported_count'
                      - 'skipped_existing_count' - 'skipped_deleted_count' - 'failed_count'
                      - 'error_summary')
       IS DISTINCT FROM
       (to_jsonb(OLD) - 'finished_at' - 'outcome' - 'fetched_count' - 'imported_count'
                      - 'skipped_existing_count' - 'skipped_deleted_count' - 'failed_count'
                      - 'error_summary') THEN
        RAISE EXCEPTION 'archival record %: a recorded sync run cannot be edited', TG_TABLE_NAME
            USING HINT = 'When a run started, what started it and who are written once (rule 7).';
    END IF;

    IF OLD.finished_at IS NOT NULL
       AND (to_jsonb(NEW) IS DISTINCT FROM to_jsonb(OLD)) THEN
        RAISE EXCEPTION 'archival record %: a finished sync run stays as it finished', TG_TABLE_NAME
            USING HINT = 'The outcome of a run is filled once. A new run is a new row.';
    END IF;

    RETURN NEW;
END $$;

-- ---------------------------------------------------------------------------
-- @entity: PortalSyncRun
-- @scope:  tenant
--
-- portal_sync_runs — one run of the portal sync for one commune (§3's `Chạy lần cuối`, `{n} tin mới`,
-- `bỏ qua {n}`, the red error block). Append-only.
--
-- THE COUNTS, per run (NULL until it finishes, then all set):
--   fetched_count            articles the run read from the portal across the selected categories
--   imported_count           became a new noi_dung_mini_app row (§3's `{n} tin mới`)
--   skipped_existing_count   `nguon_id_ngoai` already held by a live row (§10.1, `bỏ qua {n}`)
--   skipped_deleted_count    held by a SOFT-DELETED row — staff removed it; it is never brought back
--   failed_count             an article that could not be imported
-- How they add up is the writer's arithmetic, stated in its code; not constrained here, because a CHECK
-- that the writer's definition disagreed with would leave a run that can never be finished.
--
-- error_summary — §10.3: one category failing does not fail the run; it is recorded here and drawn as
-- `{Tên chuyên mục}: {mã lỗi}`. A JSON ARRAY of small objects — the category's external id and name,
-- an error CLASS (`ConnectError`, `http-503`, `parse`), and a count.
--
-- IT CARRIES NO PERSONAL DATA, AND THAT IS A RULE ON THE WRITER, NOT A HOPE: a run's input is article
-- text that routinely names residents (0006:100-113), and this row is never deleted. So the summary
-- never holds an article's title, body, summary, URL or slug (a slug is the title), never a portal
-- response body, never an exception message that might quote one, and never the api_url's query or
-- the key. A category name and an error class are editorial and technical labels, nothing more.
-- The size bound below keeps an accidental dump from being stored at all.
--
-- `actor` — who started it: 'system' for the scheduler, the staff BUSINESS CODE for `⟳ Đồng bộ ngay`
-- (rule 6, invariant 8; rule 6, invariant 6 for the system principal).
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS portal_sync_runs (
    tenant_id              TEXT        NOT NULL,
    id                     TEXT        NOT NULL,          -- ULID

    trigger_kind           TEXT        NOT NULL,          -- theo-lich | chay-tay
    actor                  TEXT        NOT NULL,

    started_at             TIMESTAMPTZ NOT NULL DEFAULT now(),

    finished_at            TIMESTAMPTZ,
    outcome                TEXT,                          -- thanh-cong | mot-phan | that-bai
    fetched_count          INT,
    imported_count         INT,
    skipped_existing_count INT,
    skipped_deleted_count  INT,
    failed_count           INT,
    error_summary          JSONB,

    PRIMARY KEY (tenant_id, id),

    CONSTRAINT portal_sync_runs_trigger_kind_known CHECK (trigger_kind IN ('theo-lich', 'chay-tay')),

    -- The scheduler is 'system' and nobody else is: a manual run signed 'system' is a run nobody can
    -- answer for, and a scheduled run signed by a person claims they pressed a button.
    CONSTRAINT portal_sync_runs_actor_matches_trigger CHECK (
        (trigger_kind = 'theo-lich' AND actor = 'system')
        OR (trigger_kind = 'chay-tay' AND actor <> 'system' AND btrim(actor) <> '')),

    CONSTRAINT portal_sync_runs_outcome_known
        CHECK (outcome IS NULL OR outcome IN ('thanh-cong', 'mot-phan', 'that-bai')),

    -- FINISH IS ALL OR NONE. Every arm spelled out with IS [NOT] NULL so no arm can evaluate to NULL.
    CONSTRAINT portal_sync_runs_finish_all_or_none CHECK (
        (finished_at IS NULL AND outcome IS NULL
            AND fetched_count IS NULL AND imported_count IS NULL
            AND skipped_existing_count IS NULL AND skipped_deleted_count IS NULL
            AND failed_count IS NULL AND error_summary IS NULL)
        OR (finished_at IS NOT NULL AND outcome IS NOT NULL
            AND fetched_count IS NOT NULL AND imported_count IS NOT NULL
            AND skipped_existing_count IS NOT NULL AND skipped_deleted_count IS NOT NULL
            AND failed_count IS NOT NULL AND error_summary IS NOT NULL)),

    CONSTRAINT portal_sync_runs_counts_non_negative CHECK (
        fetched_count >= 0 AND imported_count >= 0 AND skipped_existing_count >= 0
        AND skipped_deleted_count >= 0 AND failed_count >= 0),

    CONSTRAINT portal_sync_runs_finish_after_start CHECK (finished_at >= started_at),

    CONSTRAINT portal_sync_runs_error_summary_shape CHECK (
        error_summary IS NULL
        OR (jsonb_typeof(error_summary) = 'array' AND octet_length(error_summary::text) <= 65536))
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS portal_sync_runs_p%s PARTITION OF portal_sync_runs '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

-- The card's "last run" and the run history, newest first. Leads with tenant_id.
CREATE INDEX IF NOT EXISTS portal_sync_runs_recent
    ON portal_sync_runs (tenant_id, started_at DESC, id);

DROP TRIGGER IF EXISTS portal_sync_runs_no_hard_delete ON portal_sync_runs;
CREATE TRIGGER portal_sync_runs_no_hard_delete
    BEFORE DELETE ON portal_sync_runs
    FOR EACH ROW EXECUTE FUNCTION ho_so_luu_tru_cam_xoa_cung();

DROP TRIGGER IF EXISTS portal_sync_runs_finish_once ON portal_sync_runs;
CREATE TRIGGER portal_sync_runs_finish_once
    BEFORE UPDATE ON portal_sync_runs
    FOR EACH ROW EXECUTE FUNCTION portal_sync_run_finish_once();

-- ---------------------------------------------------------------------------
-- noi_dung_mini_app.portal_category_id — which portal category a synced item came in under, so the
-- staff list can show the portal category's name. NULLABLE, no default, English (rule 12).
--
--   * only on a synced row (`nguon = 'dong-bo-cong'`): a hand-composed item has no portal category,
--     and one carrying it would read as a synced article;
--   * WRITE-ONCE: NULL → value, never changed or cleared. It is provenance, like `nguon_id_ngoai`
--     (0006:446-447); a re-sync never touches an existing item (§10.1), so nothing legitimate moves it.
--     Enforced by its own small trigger rather than by replacing `noi_dung_mini_app_bat_bien`, so the
--     0006/0011 function and its verbatim-carry-over test are left exactly as they are.
--   * nothing else about provenance changes: `nguon`, `nguon_url`, `nguon_id_ngoai`, `da_sua_tay` and
--     their rules stand as 0006 wrote them.
-- ---------------------------------------------------------------------------
ALTER TABLE noi_dung_mini_app ADD COLUMN IF NOT EXISTS portal_category_id TEXT;

DO $$
DECLARE parent oid := 'noi_dung_mini_app'::regclass;
BEGIN
    -- SAME SERVICE, SAME SCHEMA, composite with tenant_id: an item cannot name another commune's
    -- portal category. Both sides hash-partitioned on tenant_id; the referenced pair is the PK.
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = parent
                   AND conname = 'noi_dung_mini_app_portal_category_fk') THEN
        ALTER TABLE noi_dung_mini_app ADD CONSTRAINT noi_dung_mini_app_portal_category_fk
            FOREIGN KEY (tenant_id, portal_category_id) REFERENCES portal_categories (tenant_id, id);
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = parent
                   AND conname = 'noi_dung_mini_app_portal_category_only_for_synced') THEN
        ALTER TABLE noi_dung_mini_app ADD CONSTRAINT noi_dung_mini_app_portal_category_only_for_synced
            CHECK (portal_category_id IS NULL OR nguon = 'dong-bo-cong');
    END IF;
END $$;

CREATE OR REPLACE FUNCTION noi_dung_mini_app_portal_category_frozen() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.portal_category_id IS NOT NULL
       AND NEW.portal_category_id IS DISTINCT FROM OLD.portal_category_id THEN
        RAISE EXCEPTION 'noi_dung_mini_app: the portal category of a synced item is immutable'
            USING HINT = 'It records which portal category the item was imported under — '
                         'provenance, like nguon_id_ngoai.';
    END IF;
    RETURN NEW;
END $$;

DROP TRIGGER IF EXISTS noi_dung_mini_app_portal_category_frozen ON noi_dung_mini_app;
CREATE TRIGGER noi_dung_mini_app_portal_category_frozen
    BEFORE UPDATE OF portal_category_id ON noi_dung_mini_app
    FOR EACH ROW EXECUTE FUNCTION noi_dung_mini_app_portal_category_frozen();

-- ---------------------------------------------------------------------------
-- WHAT THIS FILE DOES NOT STOP, said plainly: TRUNCATE and DDL by the table owner (ADR 0013's line).
-- Nor TWO RUNS AT ONCE for one commune (scheduler and `⟳ Đồng bộ ngay`): a unique "one unfinished run"
-- index would lock a commune out for ever after one crash, with no reaper yet to finish the row. The
-- double IMPORT is already impossible (`UNIQUE (tenant_id, nguon_id_ngoai)`); the double COUNT is the
-- job's to prevent — it owes a per-commune advisory lock taken before inserting the run row.
--
-- REVERSAL (migration question 3). Run by a person, in ONE transaction; core/migrate has no automatic
-- rollback (ADR 0013) and no down file may live here (0011:532-535).
--
-- LOSSLESS ONLY WHILE ALL OF THESE READ ZERO:
--
--   SELECT count(*) FROM portal_sync_settings;
--   SELECT count(*) FROM portal_categories;
--   SELECT count(*) FROM portal_sync_runs;
--   SELECT count(*) FROM noi_dung_mini_app WHERE portal_category_id IS NOT NULL;
--
-- ZERO → in this order:
--
--   DROP TRIGGER noi_dung_mini_app_portal_category_frozen ON noi_dung_mini_app;
--   DROP FUNCTION noi_dung_mini_app_portal_category_frozen();
--   ALTER TABLE noi_dung_mini_app
--       DROP CONSTRAINT noi_dung_mini_app_portal_category_only_for_synced,
--       DROP CONSTRAINT noi_dung_mini_app_portal_category_fk,
--       DROP COLUMN portal_category_id;
--   DROP TABLE portal_sync_runs;       -- partitions, index, triggers go with it
--   DROP TABLE portal_categories;      -- AFTER the FK above is gone
--   DROP TABLE portal_sync_settings;
--   DROP FUNCTION portal_sync_run_finish_once();
--   DROP FUNCTION portal_categories_guard();
--   DROP FUNCTION portal_sync_settings_guard();
--   DELETE FROM schema_migration WHERE <this file's row>;  -- the runner's own bookkeeping, not business data
--
-- `ho_so_luu_tru_cam_xoa_cung()` STAYS (0005's; other tables use it). `data_encryption_key` STAYS
-- (0008's) — its DEK also seals the mail password.
--
-- NON-ZERO →
--   * a settings row holds the commune's sealed portal key: dropping it loses a credential the commune
--     must then re-enter (recoverable from the commune, not from us — say so before doing it);
--   * a run row is the record of what the channel published unattended, and a category row is what
--     imported items were shown under. Dropping either is rule 7 stop condition #2 — the user plus a
--     verified backup, never a command — and the way back is a NEW migration.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- BACKSTOP: every partitioned table in this schema must actually have partitions (0002, §BACKSTOP).
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
