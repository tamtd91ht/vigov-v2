-- comms — NỘI DUNG MINI APP: what a commune publishes to its own residents inside the Zalo Mini
-- App (`docs/ui-ux/11-noi-dung-mini-app.md`).
--
-- §1 states the purpose in the interface's own words: "Tin tức, sự kiện, thông báo, bản tin
-- truyền thanh và video hiển thị cho bà con trên Zalo Mini App." It is a CMS, and §1 names two
-- sources for a row: composed by hand in ViGov, or synchronised from the commune's public portal.
--
-- OWNERSHIP IS SETTLED AND IS NOT A STOP CONDITION: kb/00-foundation/domain-boundaries.md gives
-- `comms` "tin bài, truyền thanh, bản đồ, thông báo" — the first two words are this chapter. The
-- permission keys the specification names (§10.5, `content.read` / `content.update`) are already
-- seeded in service-identity/migrations/0001_init.sql:292-293 under the group `NỘI DUNG MINI APP`,
-- which is the same answer arrived at from the other direction.
--
-- WHY A NEW FILE AND NOT AN EDIT OF 0005: 0001–0005 have been applied and core/migrate compares the
-- checksum of every applied file at startup. Editing an applied file either stops the service
-- (ErrChecksumLech) or leaves two databases claiming one schema version while holding two different
-- schemas.
--
-- ---------------------------------------------------------------------------
-- THE THIRD MEANING OF `thông báo` LIVES IN THIS FILE, AND IT IS A COLUMN VALUE, NOT A TABLE.
--
-- kb/00-foundation/ubiquitous-language.md:151 splits the one Vietnamese word into three resources
-- for three audiences — `announcements` · `public-notices` · `notifications`. Migration 0004 took
-- `thong_bao_gui_cong_dan` for the citizen message ledger and 0005 took the bare `thong_bao` for
-- the internal staff book. THIS chapter's `Thông báo` is the THIRD one: §5 calls it "thông báo cho
-- dân (khác module Thông báo nội bộ)", and it is one of six values of `noi_dung_mini_app.loai`.
--
-- Read that as the reason the table is NOT called anything with `thong_bao` in it: a row here is an
-- article on a public channel, and the one word it shares with two other tables is a value inside
-- it. Nothing in this file may be joined to `thong_bao` or to `thong_bao_gui_cong_dan`.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS, answered before the SQL.
--
--   1. HOW MANY ROWS PER COMMUNE: zero today — this file writes no row and there is no seed. In
--      steady state `noi_dung_mini_app` is the one that grows and it grows from the PORTAL, not
--      from typing: §3's sample run reports `bỏ qua 3563`, i.e. one commune's portal already holds
--      thousands of articles across 60 chuyên mục. Low tens of thousands of rows per commune over
--      the life of the system, which is why the page read below is indexed rather than bounded by a
--      ceiling the way a reference catalogue is. `danh_muc_mini_app` is small: §3's own figure is 60
--      chuyên mục, a two-level tree.
--   2. IF IT STOPS HALF-WAY: it cannot land half-applied. One file, one transaction, with the
--      progress row written inside it. Every statement is IF NOT EXISTS or CREATE OR REPLACE, so a
--      retry costs nothing.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. WHICH READ PATHS CHANGE MEANING WHILE IT IS HALF-APPLIED — the question that gets skipped and
--      the one that causes the incident. Answer: NONE. This file creates two NEW tables, one NEW
--      trigger function and four triggers and three indexes ON THE TABLES IT CREATES. It alters no
--      existing table, no existing column, no existing constraint and no existing index. It REUSES
--      `ho_so_luu_tru_cam_xoa_cung()` from 0005 without redefining it — that function reads no
--      column at all (it only raises), which is exactly why 0005 wrote it that way and why it is
--      safe to attach here.
--   5. RETENTION: a published article is something residents have READ, and §6 counts how many
--      times. That makes it business data under rule 7: soft delete only, hard DELETE refused by a
--      trigger on both tables, and provenance immutable once written.
--
-- ---------------------------------------------------------------------------
-- WHAT THIS FILE DELIBERATELY DOES NOT CREATE, so the absence is not read as unfinished work.
-- Each of the three is blocked on something specific, and none of them is blocked on effort.
--
--   * NO `cau_hinh_dong_bo_cong` (§8, third table — the portal sync configuration). Its central
--     column is §8's `ma_bao_mat`, the commune's API key for its own portal, and THERE IS NO WAY TO
--     STORE IT TODAY. ADR 0009 settled how a per-commune secret is held — envelope encryption,
--     ciphertext in the database, KEK outside it — and its decision 7 says in one line: "Cần
--     `core/crypto` — chưa tồn tại". It still does not exist (`core/` holds no crypto package).
--     A `ma_bao_mat TEXT` column here would be a plaintext credential for a government portal
--     sitting in every backup, created by the file whose comment block quotes rule 8 — and once the
--     column exists, something will write to it. Creating the table without the column is no better:
--     a sync configuration that cannot hold the key cannot sync, so the screen would show `Đang bật`
--     against a job that can never run.
--
--   * NO `chuyen_muc_cong` (§8, fourth table — the portal's own category tree, with `duoc_chon` and
--     `anh_xa_loai`). It is the CHECKBOX STATE of the modal in §3, so it only means anything
--     alongside the configuration above. The sibling implementation this repository measures against
--     does not store it at all — it asks the portal on every open
--     (`../vigov-require/apps/api/app/modules/content/router.py:282-291`, "Chuyên mục đang có trên
--     Cổng, hỏi thẳng Cổng mỗi lần mở") — so whether it is a table at all is not yet a settled
--     question, and settling it from here would settle it by writing SQL.
--
--   * NO SYNC RUN AND NOTHING THAT RECORDS ONE (§3's `Chạy lần cuối`, `{n} tin mới`, `bỏ qua {n}`,
--     `log_loi`). Those columns describe the OUTCOME of a job, and this repository has no job: no
--     scheduler for §3's `Mỗi 6 giờ`, and no outbound HTTP adapter for a per-commune third party.
--     Columns recording an outcome nothing produces are columns a screen reads as "has never run"
--     forever, which is indistinguishable from "is broken".
--
--   THE COLUMNS THAT DO SURVIVE FROM THE SYNC HALF ARE `nguon`, `nguon_url`, `nguon_id_ngoai` AND
--   `da_sua_tay`, and they are here for a different reason: they describe the ROW, not the job. A
--   row's provenance is part of the record whatever brought it in, §10.1 makes `nguon_id_ngoai` the
--   deduplication key, and §10.4's `da_sua_tay` is a promise made to a member of staff about their
--   own edit. Their writer arrives with the sync; the shape they have to fit is fixed now, while it
--   is still free.
--
--   * NO `luot_xem` WRITER. §6 draws `👁 {n}` and this file gives the column a default of 0 and a
--     trigger that refuses a DECREASE — but nothing increments it, because the only thing that
--     legitimately would is a resident opening the article, and the citizen-facing read route of §9
--     is not built (see the note on it in internal/http). A counter incremented by the STAFF list
--     route would count the office, not the commune.
--
-- ---------------------------------------------------------------------------
-- PERSONAL DATA. This module publishes OUTWARD, so the danger is the reverse of the usual one: the
-- risk is not that a column leaks, it is that a column carries something that should never have
-- been published in the first place. Three columns are watched rather than assumed safe:
--
--   `tieu_de`,   free text a member of staff types, or text a portal returns. An article about a
--   `tom_tat`,   commune's business routinely names people ("Trao quà cho gia đình ông Nguyễn Văn
--   `noi_dung`   A…"). They must not travel into a log line, an error message returned to a client,
--                a file name or a cache key (rule 3, forbidden #1 and #4) — and they are kept OUT of
--                the audit delta entirely, because that ledger is never deleted (rule 6, invariant 4
--                against rule 3, forbidden #5). The audit trail carries the id, never the text.
--
--   `tep_dinh_kem` is a JSON list of attachment descriptors. It holds URLs and file names ONLY —
--                never bytes, and never a name a citizen supplied: nothing on this chapter's screens
--                accepts an upload from outside the office.
--
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- PostgreSQL 13 is the floor, checked explicitly so the failure is readable — the same check and the
-- same reason as 0004 and 0005: BEFORE ... FOR EACH ROW triggers on a PARTITIONED table were only
-- allowed from PostgreSQL 13, and on 11/12 the CREATE TRIGGER statements below fail with a message
-- that reads like a syntax mistake and invites somebody to move the triggers down onto the
-- partitions — where a partition added later arrives silently unprotected.
-- ---------------------------------------------------------------------------
DO $$ BEGIN
    IF current_setting('server_version_num')::int < 130000 THEN
        RAISE EXCEPTION
            'noi_dung_mini_app needs PostgreSQL 13 or newer (server is %). Do not weaken this '
            'migration to fit an older server — the triggers below are what stop an article'
            's provenance from being rewritten under the sync that depends on it.',
            current_setting('server_version');
    END IF;
END $$;

-- ---------------------------------------------------------------------------
-- @entity: ContentCategory
-- @scope:  tenant
--
-- danh_muc_mini_app — the commune's OWN category tree for Mini App content (§8, second table;
-- §6's `Tất cả danh mục ▾` filter and `⊞ Danh mục tin` button; §7's `Danh mục` select).
--
-- IT IS NOT `chuyen_muc_cong`, AND THE TWO MUST NEVER BE MERGED. This table is the commune's
-- internal filing of its own Mini App content — §6 calls the button "quản lý danh mục nội bộ của
-- Mini App". `chuyen_muc_cong` (§8, fourth table, not created here) would be the PORTAL's tree, a
-- foreign system's list that the commune does not own and cannot rename. §3's modal maps one onto
-- the other; a single table would make a commune's own filing change whenever the portal reorganised.
--
-- THE SHAPE IS `bo_phan`'s, DELIBERATELY — service-identity/migrations/0001_init.sql:71-88. Same
-- question (a parent/child tree inside one commune), same answer, including the self-referencing
-- foreign key composed with `tenant_id`, which is what makes a child provably in the same commune
-- AND the same hash partition as its parent. A reader who has met that shape once does not have to
-- reason about it twice.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS danh_muc_mini_app (
    tenant_id     TEXT        NOT NULL,
    id            TEXT        NOT NULL,          -- ULID, internal

    -- §8 names it `ten`, and that is the right word rather than `nhan`: this is a NAME the commune
    -- gives a category of its own ("Chuyển đổi số"), not a relabelling of a fixed platform code the
    -- way the eight reference catalogues of ADR 0024 work. The contract field is therefore `name`
    -- and not `label` — kb/00-foundation/ubiquitous-language.md draws that line at the SCHEMA, and
    -- this column is on the `ten` side of it, next to `bo_phan.ten` and `thon_to_dan_pho.ten`.
    ten           TEXT        NOT NULL,

    -- THE BUSINESS CODE OF THIS ROW, and the only stable handle a URL, an export or an audit entry
    -- has on a category. §8 names it `slug`; the value is Vietnamese without diacritics, kebab-case,
    -- exactly like `bo_phan.ma` ("chuyen-doi-so").
    slug          TEXT        NOT NULL,

    -- NULL at the root. §3's sample list is a TWO-LEVEL tree (`Danh mục › Chuyển đổi số`), but
    -- nothing in the chapter says two is the limit, so the column expresses a tree and the write
    -- path — not the schema — is where a depth rule would live if the customer ever states one.
    cha_id        TEXT,

    thu_tu        INT         NOT NULL DEFAULT 0,

    deleted_at    TIMESTAMPTZ,
    deleted_by    TEXT,
    delete_reason TEXT,
    tao_luc       TIMESTAMPTZ NOT NULL DEFAULT now(),
    cap_nhat_luc  TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (tenant_id, id),

    -- COMPOSITE WITH tenant_id (rule 1, invariant 6), AND WITHOUT `WHERE deleted_at IS NULL`. The
    -- partial form is the reflex — "deleted it, so let me add it again" — and it is exactly what
    -- rule 7, invariant 3 forbids: a slug is held as a VALUE by every article filed under it, and
    -- reissuing it would silently refile old articles under a new, unrelated category.
    UNIQUE (tenant_id, slug),

    FOREIGN KEY (tenant_id, cha_id) REFERENCES danh_muc_mini_app (tenant_id, id),

    CONSTRAINT danh_muc_mini_app_ten_khong_rong CHECK (btrim(ten) <> ''),
    CONSTRAINT danh_muc_mini_app_slug_khong_rong CHECK (btrim(slug) <> ''),

    -- `IS DISTINCT FROM` AND NOT `<>`: the column is nullable and `NULL <> NULL` is NULL, which
    -- lets a CHECK pass in the commonest case of all (a root category). Same trap
    -- service-petitions/migrations/0008_nhiem_vu_cha.sql records. This stops the ONE-NODE cycle
    -- only; a cycle through two or more rows is not expressible in a CHECK — see the note on the
    -- write path below.
    CONSTRAINT danh_muc_mini_app_khong_tu_lam_cha CHECK (cha_id IS DISTINCT FROM id),

    -- A SOFT DELETE CARRIES WHO AND WHY, OR IT IS NOT ONE (rule 7, invariant 1).
    CONSTRAINT danh_muc_mini_app_xoa_mem_day_du
        CHECK ((deleted_at IS NULL AND deleted_by IS NULL AND delete_reason IS NULL)
            OR (deleted_at IS NOT NULL AND deleted_by IS NOT NULL AND delete_reason IS NOT NULL))
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS danh_muc_mini_app_p%s PARTITION OF danh_muc_mini_app '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

-- The tree read: children of one parent, in the commune's own order. NOT UNIQUE, so the partial
-- predicate here is legitimate — it decides how many rows are read, not whether an issued code can
-- be issued again (tools/check_khoa_duy_nhat.py:175 refuses that shape only for UNIQUE indexes).
CREATE INDEX IF NOT EXISTS danh_muc_mini_app_cay
    ON danh_muc_mini_app (tenant_id, cha_id, thu_tu)
    WHERE deleted_at IS NULL;

-- CYCLES THROUGH TWO OR MORE ROWS ARE THE WRITE PATH'S JOB, and the reason is stated here rather
-- than discovered: a CHECK cannot read another row, so `A → B → A` is not expressible above. Today
-- nothing in this service can produce one — the only write is a CREATE, and a brand-new id cannot
-- already be somebody's ancestor. The day a re-parenting route is written, it must walk up from the
-- proposed parent looking for itself BEFORE accepting, or every tree walk in this module loops
-- forever. Same rule, same words, as service-petitions/migrations/0008_nhiem_vu_cha.sql:41-46.

DROP TRIGGER IF EXISTS danh_muc_mini_app_cam_xoa_cung ON danh_muc_mini_app;
CREATE TRIGGER danh_muc_mini_app_cam_xoa_cung
    BEFORE DELETE ON danh_muc_mini_app
    FOR EACH ROW EXECUTE FUNCTION ho_so_luu_tru_cam_xoa_cung();

-- ---------------------------------------------------------------------------
-- @entity: ContentItem
-- @scope:  tenant
--
-- noi_dung_mini_app — ONE item of content one commune publishes to the Mini App (§8, first table;
-- §6's row; §7's modal).
--
-- ONE TABLE FOR ALL SIX `loai`, WHICH IS WHAT §8 MODELS AND IS NOT A SHORTCUT. The six tabs of §5
-- differ in how they are RENDERED, not in what they are: every one of them is a titled item with a
-- body, a category, a publication date and a visibility state, and §6 draws all six with the same
-- columns. Six tables would need six of every read path and would make `Tất cả danh mục` a six-way
-- union.
--
-- ⚠ §7's closing note ("Với loại `Video` nên bổ sung trường URL video; loại `Truyền thanh` bổ sung
-- file audio + thời lượng; loại `Sự kiện` bổ sung thời gian & địa điểm; loại `Banner` bổ sung link
-- đích và thứ tự hiển thị") IS NOT IMPLEMENTED, and the wording is why: "nên bổ sung" is the
-- specification proposing to itself, and §8's data model — the part that IS a statement — carries
-- none of those columns. Adding five columns nothing asks for would fix four different per-type
-- shapes into an applied migration on the strength of a suggestion. It is a question for the
-- customer, and it is in the report rather than in this file.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS noi_dung_mini_app (
    tenant_id     TEXT        NOT NULL,
    id            TEXT        NOT NULL,          -- ULID, internal

    -- THE SIX CODES ARE §5'S OWN, CHARACTER FOR CHARACTER. §5 is a table with a `Mã` column, so
    -- these were read rather than invented: `tin-tuc` · `su-kien` · `thong-bao` · `truyen-thanh` ·
    -- `video` · `banner`. Vietnamese without diacritics, hyphenated — ADR 0011, and the same
    -- spelling every other enum in this repository uses.
    loai          TEXT        NOT NULL,

    -- §7: `— Chưa xếp danh mục —`. NULL is a real, ordinary state and not a missing value: a row
    -- synchronised from the portal before the commune has built its own tree has nowhere to sit.
    danh_muc_id   TEXT,

    tieu_de       TEXT        NOT NULL,

    -- §7 marks both optional. `tom_tat` is §6's second line under the title; `noi_dung` is the
    -- article body and §8 says HTML.
    tom_tat       TEXT,
    noi_dung      TEXT,

    -- §7's `Ảnh đại diện`. A URL AND NOT BYTES: there is no `core/storage` in this repository, so
    -- nothing here can accept an upload, and this column holds a reference to a file that some other
    -- system already serves. §6 renders its presence as `🔗 Có ảnh`.
    anh_dai_dien_url TEXT,

    -- §8's `tep_dinh_kem jsonb`. A LIST OF DESCRIPTORS, defaulted to an empty array rather than left
    -- NULL: `[]` and NULL would be two spellings of "no attachments", and every read path would have
    -- to handle both. It holds URLs and file names only — see the PERSONAL DATA block at the top.
    tep_dinh_kem  JSONB       NOT NULL DEFAULT '[]'::jsonb,

    -- §6's `Ngày đăng`, printed `d/M/yyyy`. A DATE and not a timestamp, because that is what §8 says
    -- and what the screen shows; the moment the row was written is `tao_luc` beside it, and the two
    -- differ for an article backdated to the day the portal published it.
    ngay_dang     DATE        NOT NULL DEFAULT CURRENT_DATE,

    -- §6's `👁 {n}`. NOTHING INCREMENTS IT IN THIS PASS — see the header. The trigger below refuses a
    -- DECREASE, for the reason rule 10, invariant 3 gives about figures that move with no event
    -- behind them: a view count that can go down is a number no report can be written against.
    luot_xem      INT         NOT NULL DEFAULT 0,

    -- §6's chip, three values: `dang-hien` (xanh) · `cho-duyet` (cam) · `an` (xám).
    --
    --   an           composed and not published. §7's modal: "Chưa bật 'Đăng lên Mini App' thì bà
    --                con chưa thấy — soạn trước, đăng sau được." THE DEFAULT, because the checkbox
    --                in §7 starts unticked and a row that published itself would be a commune
    --                announcing something nobody decided to announce.
    --   cho-duyet    waiting for approval. REACHABLE ONLY FROM THE SYNC (§10.2: mode `chờ duyệt`
    --                lands articles here), and the sync is not built — so no path in this repository
    --                writes this value today. It is in the CHECK because §6 draws the chip and
    --                because leaving it out would make the sync's first pass a migration.
    --   dang-hien    live on the Mini App.
    trang_thai    TEXT        NOT NULL DEFAULT 'an',

    -- §8: `thu-cong` | `dong-bo-cong`. IT DECIDES WHAT MAY LATER BE DONE TO THE ROW, which is why
    -- the trigger below makes it immutable: §10.4 protects a hand-edited article from the next sync,
    -- and a row that could change its own provenance could step around that protection.
    nguon         TEXT        NOT NULL DEFAULT 'thu-cong',

    -- §8: the link to the original article on the portal, and the portal's own id for it.
    nguon_url     TEXT,
    nguon_id_ngoai TEXT,

    -- §10.4: "Bài do cán bộ sửa tay thì lần đồng bộ sau KHÔNG ghi đè (đặt cờ da_sua_tay)."
    --
    -- IT IS A PROMISE TO A PERSON, NOT A CACHE FLAG. A member of staff who corrected a portal
    -- article's title has been told their correction stands; a sync that overwrote it would undo
    -- work silently and repeatedly, every six hours. The trigger below therefore refuses to clear
    -- it, and the CHECK keeps it meaningless on a hand-composed row (which the sync never touches
    -- anyway, so a true there would claim a protection that protects nothing).
    da_sua_tay    BOOLEAN     NOT NULL DEFAULT false,

    -- WHO COMPOSED IT, AS A STAFF BUSINESS CODE (`CB-2026-7K3M9Q`) — rule 6, invariant 8, and the
    -- column name says so. §8 models it as `nguoi_tao_id`; THE SPECIFICATION LOSES TO THE RULE here,
    -- exactly as 0005 decided for `nguoi_soan_ma` and service-petitions for `chu_tri_ma`. A column
    -- holding both kinds of identifier is a column nobody can query, and the two are
    -- indistinguishable on sight.
    nguoi_tao_ma  TEXT        NOT NULL,

    deleted_at    TIMESTAMPTZ,
    deleted_by    TEXT,
    delete_reason TEXT,
    tao_luc       TIMESTAMPTZ NOT NULL DEFAULT now(),
    cap_nhat_luc  TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (tenant_id, id),

    -- §10.1: "Đồng bộ khử trùng theo `nguon_id_ngoai`; bài đã có thì bỏ qua." THIS KEY IS WHAT MAKES
    -- THAT TRUE, rather than a SELECT-then-INSERT that two concurrent runs both pass.
    --
    -- COMPOSITE WITH tenant_id (rule 1, invariant 6) — two communes whose portals hand out the same
    -- article id are two different articles, and a single-column key would make the second commune
    -- unable to take its own news.
    --
    -- NO `WHERE deleted_at IS NULL`, and here the reason is sharper than the general one: a
    -- soft-deleted article MUST keep holding its portal id, or the next sync would see the id as
    -- unused and bring the article straight back — undoing a removal a member of staff made on
    -- purpose, silently, within six hours (rule 7, invariant 3).
    --
    -- MULTIPLE NULLS ARE FINE AND ARE THE POINT: every hand-composed row has no portal id, and
    -- PostgreSQL does not consider two NULLs equal. That is why this can be a plain UNIQUE rather
    -- than a partial one.
    UNIQUE (tenant_id, nguon_id_ngoai),

    -- SAME SERVICE, SAME SCHEMA, composite with `tenant_id`: an article cannot be filed under
    -- another commune's category even if two ids ever collided. Both sides are partitioned by hash
    -- on `tenant_id` and the referenced pair is the parent's primary key.
    FOREIGN KEY (tenant_id, danh_muc_id) REFERENCES danh_muc_mini_app (tenant_id, id),

    CONSTRAINT noi_dung_mini_app_tieu_de_khong_rong CHECK (btrim(tieu_de) <> ''),

    CONSTRAINT noi_dung_mini_app_loai_hop_le
        CHECK (loai IN ('tin-tuc', 'su-kien', 'thong-bao', 'truyen-thanh', 'video', 'banner')),

    CONSTRAINT noi_dung_mini_app_trang_thai_hop_le
        CHECK (trang_thai IN ('dang-hien', 'cho-duyet', 'an')),

    CONSTRAINT noi_dung_mini_app_nguon_hop_le
        CHECK (nguon IN ('thu-cong', 'dong-bo-cong')),

    CONSTRAINT noi_dung_mini_app_luot_xem_khong_am CHECK (luot_xem >= 0),

    -- PROVENANCE AND THE PORTAL ID ARE ONE FACT, WRITTEN AS AN EQUIVALENCE rather than as two
    -- one-way implications, because the two halves fail differently and both are real: a synced
    -- article with no portal id is an article §10.1 cannot deduplicate, so the next run imports it
    -- again, and again; a hand-composed article carrying one is a row a sync would believe it owns
    -- and overwrite.
    CONSTRAINT noi_dung_mini_app_ma_ngoai_khop_nguon
        CHECK ((nguon = 'dong-bo-cong') = (nguon_id_ngoai IS NOT NULL)),

    -- §10.4's flag only means anything on a row a sync could overwrite. A `true` on a hand-composed
    -- row claims a protection that protects nothing, and a reader would take it as evidence the sync
    -- had touched the row.
    CONSTRAINT noi_dung_mini_app_da_sua_tay_chi_cho_bai_dong_bo
        CHECK (nguon = 'dong-bo-cong' OR NOT da_sua_tay),

    CONSTRAINT noi_dung_mini_app_xoa_mem_day_du
        CHECK ((deleted_at IS NULL AND deleted_by IS NULL AND delete_reason IS NULL)
            OR (deleted_at IS NOT NULL AND deleted_by IS NOT NULL AND delete_reason IS NOT NULL))
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS noi_dung_mini_app_p%s PARTITION OF noi_dung_mini_app '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

-- The page of §6's table, newest first, and the tie-break the cursor pages on (core/store.QueryPage
-- orders by `(sort, id)`). Soft-deleted rows excluded EVERYWHERE, ALWAYS (rule 7, invariant 2).
--
-- ON `tao_luc` AND NOT ON `ngay_dang`, although §6's column is the second one. A cursor column must
-- be NOT NULL — both are — but it must also not COLLIDE in bulk: `ngay_dang` is a DATE, and a sync
-- run that imports four hundred articles dated the same day gives four hundred rows one sort value,
-- so the `id` tie-break carries the whole order for a page and the order stops meaning what the
-- column name says. `tao_luc` is a timestamp and separates them. See the read route for why no
-- second sort is offered.
CREATE INDEX IF NOT EXISTS noi_dung_mini_app_so
    ON noi_dung_mini_app (tenant_id, tao_luc DESC, id) WHERE deleted_at IS NULL;

-- §6'S SIX TABS, WHICH ARE THE FILTER EVERY REQUEST FROM THAT SCREEN CARRIES. Without this index the
-- `Banner` tab of a commune with twenty thousand news articles reads the whole register to find
-- eight rows. It leads with the same pair as the index above so the two do not shadow each other:
-- this one serves a request WITH `?type=`, that one serves a request without.
CREATE INDEX IF NOT EXISTS noi_dung_mini_app_theo_loai
    ON noi_dung_mini_app (tenant_id, loai, tao_luc DESC, id) WHERE deleted_at IS NULL;

DROP TRIGGER IF EXISTS noi_dung_mini_app_cam_xoa_cung ON noi_dung_mini_app;
CREATE TRIGGER noi_dung_mini_app_cam_xoa_cung
    BEFORE DELETE ON noi_dung_mini_app
    FOR EACH ROW EXECUTE FUNCTION ho_so_luu_tru_cam_xoa_cung();

-- ---------------------------------------------------------------------------
-- noi_dung_mini_app_bat_bien — the five facts about an article that nothing may rewrite.
--
-- ENFORCED IN THE DATABASE, NOT IN THE APPLICATION, for the reason ADR 0013 gives for the audit
-- ledger: a promise the application layer makes is a promise a migration script, a psql session or
-- the next service to connect never heard. Every one of these five is also refused by the absence of
-- a parameter in the UPDATE statements in internal/store — both layers are meant, and the floor is
-- what holds when a future writer is added.
--
-- WHAT IS REFUSED, in the order it is checked:
--
--   `nguoi_tao_ma`,  fixed when the row is created. Who filed this article and when are the anchors
--   `tao_luc`        of everything else on the record.
--   `nguon`          an article does not change where it came from. A hand-composed row that could
--                    become `dong-bo-cong` would acquire a portal id it never had; a synced row that
--                    could become `thu-cong` would drop out of §10.1's deduplication and come back
--                    as a duplicate on the next run.
--   `nguon_id_ngoai` once set. It is §10.1's deduplication key: moving it frees the old value, and
--                    the portal article it named is imported again as a second row.
--   `da_sua_tay`     may go false → true and NEVER back. §10.4 promised a member of staff that their
--                    edit survives the next sync; clearing the flag withdraws that promise without
--                    anybody being told, and the next run overwrites the edit.
--   `luot_xem`       may only go up. A figure that can go down with no event behind it is the defect
--                    rule 10, invariant 3 describes for `qua_han`, in a different column.
--
-- WHAT STAYS EDITABLE, on purpose: everything §7's modal collects (`loai`, `danh_muc_id`, `tieu_de`,
-- `tom_tat`, `noi_dung`, `anh_dai_dien_url`, `tep_dinh_kem`, `ngay_dang`, `trang_thai`), `nguon_url`
-- (the portal can move an article), `cap_nhat_luc`, and the soft-delete columns.
--
-- The messages name the operation and the column and nothing else: an error message travels into
-- logs and back to clients (rule 3, forbidden #3), and `tieu_de` can name a person.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION noi_dung_mini_app_bat_bien() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.nguoi_tao_ma IS DISTINCT FROM OLD.nguoi_tao_ma
    OR NEW.tao_luc      IS DISTINCT FROM OLD.tao_luc THEN
        RAISE EXCEPTION 'noi_dung_mini_app: author and creation time are immutable'
            USING HINT = 'Who filed this item and when are fixed at creation. Record a new item '
                         'instead (rule 7, forbidden #5).';
    END IF;

    IF NEW.nguon IS DISTINCT FROM OLD.nguon THEN
        RAISE EXCEPTION 'noi_dung_mini_app: provenance is immutable'
            USING HINT = 'An item does not change where it came from. Changing it would move the '
                         'row in or out of the sync deduplication of '
                         'docs/ui-ux/11-noi-dung-mini-app.md §10.1.';
    END IF;

    IF OLD.nguon_id_ngoai IS NOT NULL
    AND NEW.nguon_id_ngoai IS DISTINCT FROM OLD.nguon_id_ngoai THEN
        RAISE EXCEPTION 'noi_dung_mini_app: the portal identifier is immutable'
            USING HINT = 'It is the deduplication key of §10.1. Freeing it imports the same portal '
                         'article again as a second row.';
    END IF;

    IF OLD.da_sua_tay AND NOT NEW.da_sua_tay THEN
        RAISE EXCEPTION 'noi_dung_mini_app: a hand-edited item cannot be marked un-edited'
            USING HINT = 'docs/ui-ux/11-noi-dung-mini-app.md §10.4: a member of staff was promised '
                         'that their edit survives the next sync. Clearing this flag withdraws that '
                         'promise and the next run overwrites their work.';
    END IF;

    IF NEW.luot_xem < OLD.luot_xem THEN
        RAISE EXCEPTION 'noi_dung_mini_app: the view counter cannot go down'
            USING HINT = 'A figure that decreases with no event behind it is what rule 10, '
                         'invariant 3 refuses. Correcting it is a new column, not a lower value.';
    END IF;

    RETURN NEW;
END $$;

DROP TRIGGER IF EXISTS noi_dung_mini_app_bat_bien ON noi_dung_mini_app;
CREATE TRIGGER noi_dung_mini_app_bat_bien
    BEFORE UPDATE ON noi_dung_mini_app
    FOR EACH ROW EXECUTE FUNCTION noi_dung_mini_app_bat_bien();

-- ---------------------------------------------------------------------------
-- WHAT THIS FILE DOES NOT STOP, said plainly rather than left to be discovered: TRUNCATE on either
-- table, and DDL by the table owner (ALTER TABLE ... DISABLE TRIGGER, dropping a table). Same line
-- ADR 0013 draws for the audit ledger — the job is to make the accidental and the convenient
-- impossible, not to defeat an administrator who has decided to destroy data and is willing to be
-- seen doing it.
--
-- REVERSAL (migration question 3). Every object here is new, and while the two tables are still
-- empty — which they are in every environment today — the reversal is complete and loses nothing:
--
--   DROP TABLE noi_dung_mini_app;    -- its 32 partitions, its two indexes and its triggers go with it
--   DROP TABLE danh_muc_mini_app;    -- LAST: the foreign key points at it
--   DROP FUNCTION noi_dung_mini_app_bat_bien();
--
-- and in the SAME transaction remove this file's row from `schema_migration`, otherwise the runner
-- still believes the schema is in place. `ho_so_luu_tru_cam_xoa_cung()` IS NOT DROPPED: it belongs
-- to migration 0005 and three of that file's tables still use it. Nothing from 0001–0005 is altered
-- by this file, so nothing there has to be put back.
--
-- ONCE A COMMUNE HAS PUBLISHED ONE ITEM HERE, THAT IS NO LONGER A REVERSAL — it destroys what a
-- public authority told its residents, together with the record of who published it. That is rule
-- 7's first stop condition and needs the user, not a command. From that point the way back is a NEW
-- migration, and core/migrate has no automatic rollback for exactly this reason (ADR 0013).
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- BACKSTOP: every partitioned table in this schema must actually have partitions.
--
-- A table declared PARTITION BY and given no partitions REJECTS EVERY INSERT, silently, until the
-- first real write — and because the audit entry shares the business transaction (rule 6, invariant
-- 3), that first write rolls back entirely. The check is repeated at the end of every migration that
-- declares a partitioned table, because it only verifies the state after a file that CARRIES it
-- (0002, §BACKSTOP).
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
