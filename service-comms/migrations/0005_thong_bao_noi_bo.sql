-- comms — THÔNG BÁO NỘI BỘ: the channel one commune uses to tell its OWN STAFF something.
--
-- `docs/ui-ux/08-thong-bao.md` §1 states the purpose in the interface's own words: "Gửi tới các
-- bộ phận, kèm thư điện tử. Người nhận thấy ngay ở trang này." §6 names the three tables this
-- file creates, and this file follows that naming rather than inventing its own.
--
-- OWNERSHIP IS SETTLED AND IS NOT A STOP CONDITION: kb/00-foundation/domain-boundaries.md gives
-- `comms` "tin bài, truyền thanh, bản đồ, **thông báo**". The only question this file had to
-- answer about names was which of the three meanings of `thông báo` gets the bare noun, and
-- migration 0004 already answered it — see below.
--
-- WHY A NEW FILE AND NOT AN EDIT OF 0004: 0001–0004 have been applied and core/migrate compares
-- the checksum of every applied file at startup. Editing an applied file either stops the service
-- (ErrChecksumLech) or leaves two databases claiming one schema version while holding two
-- different schemas.
--
-- ---------------------------------------------------------------------------
-- THE NAME `thong_bao` IS TAKEN DELIBERATELY, AND 0004 IS WHERE THAT WAS DECIDED.
--
-- Three different things share the one Vietnamese word (kb/00-foundation/ubiquitous-language.md
-- :151 — `announcements` · `public-notices` · `notifications`), and 0004:16-27 reserved the bare
-- noun for THIS module while taking `thong_bao_gui_cong_dan` for the citizen channel. That file's
-- reasoning is not repeated here (rule 9): read it there.
--
-- WHAT FOLLOWS FROM IT, and it is the one thing to keep straight while reading this file: a row
-- in `thong_bao` is read by a MEMBER OF STAFF of this commune. A row in `thong_bao_gui_cong_dan`
-- leaves the commune, to a citizen. They are one join apart and they must never become one table.
--
-- THE URL NOUN IS SETTLED: `announcements`, from the mapping table row :151. It was looked up,
-- not translated — `thong-bao` is a blocked segment in hooks/rest_api_guard.py and a Vietnamese
-- path segment is refused by ADR 0011 anyway, so §7's `/api/thong-bao` sketch cannot ship.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS, answered before the SQL.
--
--   1. HOW MANY ROWS PER COMMUNE: zero today — this file writes no row, and there is no seed. In
--      steady state `thong_bao` is small (a commune issues announcements in the tens per year)
--      while `thong_bao_nguoi_nhan` is the one that grows: §10's sample notices have 7, 8 and 12
--      recipients, and a commune has a few dozen staff, so the ceiling is roughly (announcements
--      per year) × (staff). Low thousands of rows a year. That is why the recipient table is
--      partitioned like everything else and why its primary key starts with the pair the counter
--      query groups by.
--   2. IF IT STOPS HALF-WAY: it cannot land half-applied. One file, one transaction, with the
--      progress row written inside it. Every statement is IF NOT EXISTS or CREATE OR REPLACE, so
--      a retry costs nothing.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. WHICH READ PATHS CHANGE MEANING WHILE IT IS HALF-APPLIED — the question that gets skipped
--      and the one that causes the incident. Answer: NONE, and it is worth saying why rather than
--      asserting it. This file creates three NEW tables, one NEW shared function, five triggers
--      and two indexes ON THE TABLES IT CREATES. It alters no existing table, no existing column,
--      no existing constraint and no existing index. `thong_bao_gui_cong_dan` (0004) is a
--      different table with a different name and is not touched — in particular this file does
--      NOT reuse its `thong_bao_gui_cong_dan_bat_bien()` function, which reads columns that do
--      not exist here and would fail at runtime on the first UPDATE.
--   5. RETENTION: an issued announcement is a thing members of staff have READ and may have acted
--      on, and §4 records per-person acknowledgement of it. That makes it business data under
--      rule 7: soft delete only on `thong_bao`, hard DELETE refused by a trigger on all three
--      tables, and the content immutable once issued (§9.2).
--
-- ---------------------------------------------------------------------------
-- WHAT THIS FILE DELIBERATELY DOES NOT CREATE, so the absence is not read as unfinished work.
--
--   * NO `hop_thu_thong_bao` (§8, the header bell). It is a UNIFIED INBOX: §8's own table lists
--     four `loai` values and three of them are produced by service-petitions (đề nghị gia hạn,
--     được giao nhiệm vụ, nhắc sắp/quá hạn). A table fed by another service's events is an
--     INTER-SERVICE CONTRACT (rule 2), and `.proto` is its source of truth — inventing the event
--     name inside a migration would put one half of a contract where no consumer can see it. It
--     is also not obviously this service's table at all; deciding that from here would settle
--     ownership by writing SQL.
--
--   * NO EMAIL. §1 and §5 want a copy by mail through `Cấu hình → Máy chủ thư`, and neither that
--     configuration nor any SMTP adapter exists in this repository. The two COLUMNS that record
--     what was intended and what happened are here (`gui_thu_dien_tu`, `trang_thai_thu`,
--     `thu_gui_luc`, `thu_loi_ma`) because they are part of the record of an announcement; the
--     sender is a later pass. Until then `trang_thai_thu` stays `chua-gui`, which is honest, and
--     §3's `Đang gửi thư…` chip simply never appears.
--
--   * NO `so_nguoi_nhan` / `so_da_xac_nhan` COUNTER COLUMN. §3's `{x}/{y} đã xác nhận` is DERIVED
--     by counting `thong_bao_nguoi_nhan`, exactly as §6 writes it ("x = count(da_xac_nhan_luc IS
--     NOT NULL), y = count(*)"). A stored counter is wrong the moment a recipient acknowledges,
--     it cannot be repaired from a screen, and the stale copy is the one that reaches a report —
--     the same reasoning rule 10, invariant 3 applies to `qua_han`.
--
--   * NO INDEX ON `(tenant_id, nguoi_nhan_ma)`. It is exactly what §2's `Gửi cho tôi` filter
--     needs, and that read is NOT SHIPPED in this pass (see the note on the read route in
--     internal/http). An index nothing reads is a write cost with no reader plus a reader's false
--     assurance that some query is cheap. It is one line in a later migration, on a table that
--     will still be small.
--
--   * NO UNIQUE KEY ANYWHERE BUT THE THREE PRIMARY KEYS, and no partial unique index at all. An
--     announcement has no issued number: §3 and §4 identify one by its title and the moment it
--     was issued, and nothing in chapter 08 prints a reference on paper.
--
-- ---------------------------------------------------------------------------
-- PERSONAL DATA. This module is staff-to-staff, so the usual danger is absent — but two columns
-- are watched rather than assumed safe:
--
--   `noi_dung`   is free text a member of staff types, and an announcement about a case quotes
--                the case. It must not travel into a log line, an error message returned to a
--                client, a file name or a cache key (rule 3, forbidden #1 and #4).
--   `thu_loi_ma` is the provider's error CODE and never its message. §6 sketches `thu_loi text`;
--                an SMTP failure message quotes the envelope, and the envelope is the recipient's
--                address. Storing the sentence would make this table a mailbox directory in every
--                backup. The column is renamed to say what may go in it.
--
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- PostgreSQL 13 is the floor, checked explicitly so the failure is readable — the same check and
-- the same reason as 0004: BEFORE ... FOR EACH ROW triggers on a PARTITIONED table were only
-- allowed from PostgreSQL 13, and on 11/12 the CREATE TRIGGER statements below fail with a
-- message that reads like a syntax mistake and invites somebody to move the triggers down onto
-- the partitions — where a partition added later arrives silently unprotected.
-- ---------------------------------------------------------------------------
DO $$ BEGIN
    IF current_setting('server_version_num')::int < 130000 THEN
        RAISE EXCEPTION
            'thong_bao needs PostgreSQL 13 or newer (server is %). Do not weaken this migration '
            'to fit an older server — the triggers below are what stop an announcement staff have '
            'already read from being rewritten under them.',
            current_setting('server_version');
    END IF;
END $$;

-- ---------------------------------------------------------------------------
-- ho_so_luu_tru_cam_xoa_cung — one function, attached to every archival table it fits.
--
-- It refuses a hard removal and NOTHING ELSE, so it can be attached to a table whatever its
-- columns are. Rule 7, forbidden #1.
--
-- THE NAME IS THE ONE service-documents, service-finance AND service-petitions ALREADY USE for
-- this exact job. Four services, four schemas, one name — a reader who has met it once does not
-- have to read it twice. This is its first copy in `comms`.
--
-- IT IS NOT `thong_bao_gui_cong_dan_bat_bien()` FROM 0004, although both guard archival tables in
-- this schema. That one reads `khoa_lan_gui`, `doi_tuong_ma`, `moc`, `lan`, `kenh` and
-- `nguoi_nhan_ma` — columns this table does not have — so attaching it here would fail at runtime
-- on the first UPDATE, inside the business transaction, which (because the audit entry shares it,
-- rule 6 invariant 3) rolls the whole act back.
--
-- ENFORCED IN THE DATABASE, NOT IN THE APPLICATION, for the reason ADR 0013 gives for the audit
-- ledger: a promise the application layer makes is a promise a migration script, a psql session
-- or the next service to connect never heard.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION ho_so_luu_tru_cam_xoa_cung() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'archival table %: hard delete refused', TG_TABLE_NAME
        USING HINT = 'An issued announcement is something members of staff have read and may have '
                     'acknowledged (docs/ui-ux/08-thong-bao.md §4). Soft delete instead: set '
                     'deleted_at, deleted_by and delete_reason on thong_bao — rule 7, invariant 1. '
                     'The recipient and org-unit rows belong to the announcement and go with it.';
END $$;

-- ---------------------------------------------------------------------------
-- @entity: Announcement
-- @scope:  tenant
--
-- thong_bao — ONE internal announcement of ONE commune (§6).
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS thong_bao (
    tenant_id     TEXT        NOT NULL,
    id            TEXT        NOT NULL,          -- ULID, internal

    -- §5: both required. The card in §3 is a title and two lines of the body; neither can be
    -- blank without producing a row nobody can pick out of the list.
    tieu_de       TEXT        NOT NULL,
    noi_dung      TEXT        NOT NULL,

    -- THE THREE STATES OF §6, SPELLED WITH HYPHENS AND NOT UNDERSCORES.
    --
    -- §6 writes `nhap | da_phat_hanh | da_go`. Every enum value already in this system is
    -- hyphenated — `cho-gui`, `da-gui`, `phieu-phan-anh`, `don-vi` — and ADR 0011 fixes the FORM
    -- (Vietnamese, no diacritics) rather than the separator. One spelling across the repository
    -- is worth more than fidelity to a sketch, and the alternative is a service where half the
    -- codes use one separator and half the other, with no way to remember which is which.
    --
    --   nhap           drafted, no recipient list has been generated, nobody can see it but its
    --                  author. `Lưu nháp` in §5.
    --   da-phat-hanh   issued. Recipients exist, and §9.2 forbids editing the content from here on.
    --   da-go          withdrawn with §4's `🗑 Gỡ`. The row STAYS, with its recipients and their
    --                  acknowledgements: people read it, and a withdrawal does not un-read it.
    trang_thai    TEXT        NOT NULL DEFAULT 'nhap',

    -- §3: a pinned announcement shows at the head of the list with a pin icon.
    ghim          BOOLEAN     NOT NULL DEFAULT false,

    -- §5: turns on §3's orange chip and the `{x}/{y} đã xác nhận` counter, and §9.5 keeps the
    -- announcement on the bell of everyone who has not acknowledged it.
    bat_buoc_xac_nhan BOOLEAN NOT NULL DEFAULT false,

    -- §5: the checkbox is on by default. IT RECORDS WHAT WAS ASKED FOR, NOT WHAT HAPPENED —
    -- `trang_thai_thu` below records that, and it stays `chua-gui` until an SMTP adapter exists.
    gui_thu_dien_tu   BOOLEAN NOT NULL DEFAULT false,

    -- §3's mail chip: `Đang gửi thư…` / `Đã gửi thư` / `Gửi thư lỗi`, plus the state before any
    -- attempt. A CHECK below refuses any value other than `chua-gui` while `gui_thu_dien_tu` is
    -- false: a "mail sent" claim on an announcement that never asked for mail is a claim nothing
    -- in this system witnessed.
    trang_thai_thu    TEXT    NOT NULL DEFAULT 'chua-gui',

    -- WHO COMPOSED IT, AS A STAFF BUSINESS CODE (`CB-2026-7K3M9Q`) — rule 6, invariant 8, and the
    -- column name says so. §6 models it as `nguoi_soan_id uuid`; THE SPECIFICATION LOSES TO THE
    -- RULE here, exactly as service-petitions decided for `chu_tri_ma` and `nguoi_thuc_hien_ma`.
    -- A column holding both kinds of identifier is a column nobody can query, and the two are
    -- indistinguishable on sight.
    nguoi_soan_ma TEXT        NOT NULL,

    -- WHEN IT WAS ISSUED — the timestamp §3 prints on the card. NULL for a draft, and a CHECK
    -- below ties the two together so neither can drift from the other.
    phat_hanh_luc TIMESTAMPTZ,

    deleted_at    TIMESTAMPTZ,
    deleted_by    TEXT,
    delete_reason TEXT,
    tao_luc       TIMESTAMPTZ NOT NULL DEFAULT now(),
    cap_nhat_luc  TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (tenant_id, id),

    CONSTRAINT thong_bao_tieu_de_khong_rong CHECK (btrim(tieu_de) <> ''),
    CONSTRAINT thong_bao_noi_dung_khong_rong CHECK (btrim(noi_dung) <> ''),

    CONSTRAINT thong_bao_trang_thai_hop_le
        CHECK (trang_thai IN ('nhap', 'da-phat-hanh', 'da-go')),

    CONSTRAINT thong_bao_trang_thai_thu_hop_le
        CHECK (trang_thai_thu IN ('chua-gui', 'dang-gui', 'da-gui', 'loi')),

    -- A DRAFT HAS NO ISSUE TIME AND AN ISSUED ANNOUNCEMENT HAS ONE. Written as an equivalence
    -- rather than two one-way implications, because the two halves fail differently and both are
    -- real: an issued announcement with no timestamp is a card §3 cannot draw, and a draft
    -- carrying one is a draft that will read as issued the day somebody writes a report over this
    -- table.
    CONSTRAINT thong_bao_moc_phat_hanh_khop_trang_thai
        CHECK ((trang_thai = 'nhap') = (phat_hanh_luc IS NULL)),

    -- NO MAIL ASKED FOR MEANS NO MAIL STATE. Without this, a bug in a future sender could mark an
    -- announcement `da-gui` that never requested mail, and §3's chip would tell a member of staff
    -- that colleagues were emailed when they were not.
    CONSTRAINT thong_bao_thu_chi_khi_duoc_yeu_cau
        CHECK (gui_thu_dien_tu OR trang_thai_thu = 'chua-gui'),

    -- A SOFT DELETE CARRIES WHO AND WHY, OR IT IS NOT ONE (rule 7, invariant 1).
    CONSTRAINT thong_bao_xoa_mem_day_du
        CHECK ((deleted_at IS NULL AND deleted_by IS NULL AND delete_reason IS NULL)
            OR (deleted_at IS NOT NULL AND deleted_by IS NOT NULL AND delete_reason IS NOT NULL))
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS thong_bao_p%s PARTITION OF thong_bao '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

-- The page of cards, newest first, and the tie-break the cursor pages on (core/store.QueryPage
-- orders by `(sort, id)`). Soft-deleted rows excluded EVERYWHERE, ALWAYS (rule 7, invariant 2).
--
-- ON `tao_luc` AND NOT ON `phat_hanh_luc`, ALTHOUGH §3 PRINTS THE ISSUE TIME ON THE CARD. A
-- cursor column must be NOT NULL: `(col, id) > ($n, $n+1)` is NULL for a NULL col, so every draft
-- would vanish from every page after the first — silently, which is the failure mode core/page's
-- own comment warns about. `tao_luc` is NOT NULL and, for an announcement issued the moment it is
-- composed, differs from `phat_hanh_luc` by milliseconds.
--
-- `ghim` IS NOT IN THIS INDEX AND PINNED-FIRST IS NOT IMPLEMENTED — see the read route for why:
-- core/page carries ONE sort column plus `id`, so `ORDER BY ghim DESC, tao_luc DESC` is not a
-- cursor this repository can express today. Stating it here stops the next reader from adding a
-- column to the index in the belief that it would make the ordering work.
CREATE INDEX IF NOT EXISTS thong_bao_so
    ON thong_bao (tenant_id, tao_luc DESC, id) WHERE deleted_at IS NULL;

DROP TRIGGER IF EXISTS thong_bao_cam_xoa_cung ON thong_bao;
CREATE TRIGGER thong_bao_cam_xoa_cung
    BEFORE DELETE ON thong_bao
    FOR EACH ROW EXECUTE FUNCTION ho_so_luu_tru_cam_xoa_cung();

-- ---------------------------------------------------------------------------
-- thong_bao_bat_bien — §9.2 AS A CONSTRAINT INSTEAD OF AS A SENTENCE IN A DOCUMENT.
--
-- "Phát hành xong **không sửa nội dung** — muốn sửa thì gỡ và soạn lại." That rule protects
-- something specific: a member of staff has READ this announcement and may have acknowledged it.
-- Editing the text afterwards leaves the acknowledgement pointing at words nobody agreed to, and
-- nothing downstream could tell.
--
-- WHAT IS REFUSED, in the order it is checked:
--
--   `nguoi_soan_ma`,    fixed when the row is created, in every state. Who wrote it and when it
--   `tao_luc`           was written are the anchors of everything else on the record.
--   `tieu_de`,          once the announcement has left `nhap`. A draft is still the author's own
--   `noi_dung`,         and may be edited freely; an issued one may not (§9.2). `bat_buoc_xac_nhan`
--   `bat_buoc_xac_nhan` is in this group because it decides whether an acknowledgement was ever
--                       asked for, and turning it on afterwards would make `0/12` appear against
--                       an announcement nobody was asked to acknowledge.
--   `phat_hanh_luc`     once set. It is the moment §3 prints and the origin of any "how long did
--                       staff take to read it" figure.
--   going back to       an issued announcement cannot become a draft, and a withdrawn one cannot
--   `nhap`; leaving     be re-issued. §9.2's remedy is explicit: gỡ and compose again, which
--   `da-go`             produces a NEW row rather than reviving this one.
--
-- WHAT STAYS EDITABLE, on purpose: `ghim` (§3's pin is a display decision the commune may change
-- at any time), `trang_thai_thu` and `cap_nhat_luc` (the OUTCOME of sending, not known when the
-- row is written), `gui_thu_dien_tu` while still a draft, and the soft-delete columns.
--
-- The messages name the operation and the column and nothing else: an error message travels into
-- logs and back to clients (rule 3, forbidden #3), and `noi_dung` is free text about the commune's
-- own business.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION thong_bao_bat_bien() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.nguoi_soan_ma IS DISTINCT FROM OLD.nguoi_soan_ma
    OR NEW.tao_luc       IS DISTINCT FROM OLD.tao_luc THEN
        RAISE EXCEPTION 'thong_bao: author and creation time are immutable'
            USING HINT = 'Who wrote this announcement and when are fixed at creation. Record a '
                         'new announcement instead (rule 7, forbidden #5).';
    END IF;

    IF OLD.trang_thai <> 'nhap' THEN
        IF NEW.tieu_de           IS DISTINCT FROM OLD.tieu_de
        OR NEW.noi_dung          IS DISTINCT FROM OLD.noi_dung
        OR NEW.bat_buoc_xac_nhan IS DISTINCT FROM OLD.bat_buoc_xac_nhan THEN
            RAISE EXCEPTION 'thong_bao: an issued announcement cannot be edited'
                USING HINT = 'docs/ui-ux/08-thong-bao.md §9.2: staff have already read it, and an '
                             'acknowledgement must keep pointing at the words that were agreed to. '
                             'Withdraw it and compose a new one.';
        END IF;
    END IF;

    IF OLD.phat_hanh_luc IS NOT NULL
    AND NEW.phat_hanh_luc IS DISTINCT FROM OLD.phat_hanh_luc THEN
        RAISE EXCEPTION 'thong_bao: the moment of issue is immutable'
            USING HINT = 'It is the timestamp the card shows and the origin of every figure '
                         'counted from it.';
    END IF;

    IF OLD.trang_thai <> 'nhap' AND NEW.trang_thai = 'nhap' THEN
        RAISE EXCEPTION 'thong_bao: an issued announcement cannot become a draft again'
            USING HINT = 'Recipients exist and some of them have read it.';
    END IF;

    IF OLD.trang_thai = 'da-go' AND NEW.trang_thai <> 'da-go' THEN
        RAISE EXCEPTION 'thong_bao: a withdrawn announcement cannot be re-issued'
            USING HINT = 'Compose a new announcement — §9.2. Re-issuing this row would reuse the '
                         'acknowledgements collected against the withdrawn text.';
    END IF;

    RETURN NEW;
END $$;

DROP TRIGGER IF EXISTS thong_bao_bat_bien ON thong_bao;
CREATE TRIGGER thong_bao_bat_bien
    BEFORE UPDATE ON thong_bao
    FOR EACH ROW EXECUTE FUNCTION thong_bao_bat_bien();

-- ---------------------------------------------------------------------------
-- @entity: AnnouncementOrgUnit
-- @scope:  tenant
--
-- thong_bao_bo_phan — WHICH DEPARTMENTS THIS ANNOUNCEMENT WAS ADDRESSED TO (§4's `BỘ PHẬN NHẬN`
-- chips, §5's toggle list, §6's two-column table).
--
-- IT IS NOT THE RECIPIENT LIST AND MUST NEVER BE READ AS ONE. §9.3: "Chọn bộ phận sinh danh sách
-- người nhận TẠI THỜI ĐIỂM PHÁT HÀNH (cán bộ thêm sau không tự nhận)." So this table is the
-- ADDRESSING DECISION, kept because §4 draws it back on the detail panel, and
-- `thong_bao_nguoi_nhan` is the frozen result of applying it. Keeping only one of the two would
-- lose either what was chosen or who actually received it.
--
-- `bo_phan_id` REFERENCES A ROW THIS SERVICE DOES NOT OWN AND THERE IS NO FOREIGN KEY. `bo_phan`
-- lives in service-identity (kb/00-foundation/domain-boundaries.md), and a foreign key would mean
-- one schema reaching into another service's tables — rule 2, forbidden #2, and physically
-- impossible across two databases. The consequence, stated rather than discovered: a department
-- deleted in identity leaves a chip here that resolves to nothing, and the read path must render
-- that as a missing name rather than failing.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS thong_bao_bo_phan (
    tenant_id    TEXT        NOT NULL,
    thong_bao_id TEXT        NOT NULL,

    -- The org unit's ULID, as identity issues it. AN ID AND NOT A CODE, unlike every staff
    -- reference in this file: `bo_phan` has no business code — identity's own contract addresses
    -- org units by id — so there is nothing else to store.
    bo_phan_id   TEXT        NOT NULL,

    tao_luc      TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- COMPOSITE WITH tenant_id (rule 1, invariant 6), AND IT DOUBLES AS THE DUPLICATE GUARD: a
    -- department chosen twice in one request is one row, not two, and the write path's
    -- ON CONFLICT DO NOTHING turns that into the correct no-op rather than an error.
    PRIMARY KEY (tenant_id, thong_bao_id, bo_phan_id),

    -- SAME SERVICE, SAME SCHEMA — this key crosses nothing. Both sides are partitioned by hash on
    -- `tenant_id` and the referenced pair is the parent's primary key, which is the shape
    -- service-petitions proved out in its 0007. Composite with `tenant_id`, so a row here cannot
    -- reach another commune's announcement even if two ids ever collided.
    FOREIGN KEY (tenant_id, thong_bao_id) REFERENCES thong_bao (tenant_id, id)
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS thong_bao_bo_phan_p%s PARTITION OF thong_bao_bo_phan '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

-- NO SEPARATE INDEX. The primary key is `(tenant_id, thong_bao_id, bo_phan_id)` and the only read
-- is "the departments of these announcements", which is that key's leading pair.

DROP TRIGGER IF EXISTS thong_bao_bo_phan_cam_xoa_cung ON thong_bao_bo_phan;
CREATE TRIGGER thong_bao_bo_phan_cam_xoa_cung
    BEFORE DELETE ON thong_bao_bo_phan
    FOR EACH ROW EXECUTE FUNCTION ho_so_luu_tru_cam_xoa_cung();

-- ---------------------------------------------------------------------------
-- @entity: AnnouncementRecipient
-- @scope:  tenant
--
-- thong_bao_nguoi_nhan — ONE named member of staff who was sent ONE announcement, and what they
-- have done with it (§4's `NGƯỜI NHẬN (12)` list, §6's third table).
--
-- THERE ARE NO SOFT-DELETE COLUMNS HERE, AND THAT IS A DECISION RATHER THAN AN OVERSIGHT. A
-- recipient row has no lifecycle of its own: §9.3 freezes the list at the moment of issue and
-- nothing in chapter 08 removes one person from an announcement — §4's `🗑 Gỡ` withdraws the whole
-- announcement, which is `thong_bao.trang_thai = 'da-go'`. Giving this table its own soft delete
-- would create a state the specification does not have (an announcement with a recipient who is
-- and is not on the list) and a second place for a read path to forget a predicate. The hard
-- DELETE that would otherwise be the easy path is refused by the trigger below.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS thong_bao_nguoi_nhan (
    tenant_id     TEXT        NOT NULL,
    thong_bao_id  TEXT        NOT NULL,

    -- THE RECIPIENT, AS A STAFF BUSINESS CODE (`CB-2026-7K3M9Q`). §6 models it as
    -- `nguoi_dung_id uuid`; the rule wins for the same reason it wins on `nguoi_soan_ma` above,
    -- and it has a second benefit here: `authz.Principal` carries `.Ma`, so §2's `Gửi cho tôi`
    -- filter compares the session's own code against this column with no lookup in between.
    nguoi_nhan_ma TEXT        NOT NULL,

    -- §6: true = added by name in §5's `Gửi thêm đích danh`, false = generated from a department.
    -- IT IS NOT DERIVABLE FROM `thong_bao_bo_phan` and is therefore not a second copy of anything
    -- (rule 9): a person may be BOTH a member of a chosen department and named explicitly, and
    -- membership is identity's data, which can change after the list is frozen.
    dich_danh     BOOLEAN     NOT NULL,

    -- §4's three states, as two timestamps rather than an enum: `chưa mở` is both NULL, `đã mở` is
    -- the first one set, `✓ đã xác nhận` is both. Timestamps because §4 shows the moment on hover,
    -- and an enum plus a timestamp would be two representations of one fact.
    da_mo_luc       TIMESTAMPTZ,
    da_xac_nhan_luc TIMESTAMPTZ,

    -- The mail copy, per recipient (§3's chip is the roll-up of these). Nothing writes them yet —
    -- there is no SMTP adapter in this repository.
    thu_gui_luc   TIMESTAMPTZ,

    -- THE PROVIDER'S ERROR CODE, NEVER ITS MESSAGE. See the PERSONAL DATA block at the top: an
    -- SMTP failure message quotes the envelope, and the envelope is somebody's address.
    thu_loi_ma    TEXT,

    tao_luc       TIMESTAMPTZ NOT NULL DEFAULT now(),
    cap_nhat_luc  TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- COMPOSITE WITH tenant_id (rule 1, invariant 6). It is also what makes the two ways of
    -- reaching one person — through a department and by name — collapse into ONE row: a person
    -- cannot be sent the same announcement twice, and `{y}` in §3's counter cannot double-count.
    PRIMARY KEY (tenant_id, thong_bao_id, nguoi_nhan_ma),

    FOREIGN KEY (tenant_id, thong_bao_id) REFERENCES thong_bao (tenant_id, id),

    -- ACKNOWLEDGING IMPLIES HAVING OPENED IT. §4 lists the states in that order and §9.5 counts
    -- the second. A row acknowledged but never opened is a state no path can legitimately
    -- produce, and one that would make "chưa mở" an unreliable figure the moment it appeared.
    CONSTRAINT thong_bao_nguoi_nhan_xac_nhan_thi_da_mo
        CHECK (da_xac_nhan_luc IS NULL OR da_mo_luc IS NOT NULL)
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS thong_bao_nguoi_nhan_p%s PARTITION OF thong_bao_nguoi_nhan '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

-- THE COUNTER OF §3, `{x}/{y} đã xác nhận`, FOR A WHOLE PAGE OF CARDS AT ONCE.
--
-- The read groups by `(tenant_id, thong_bao_id)` and counts rows and acknowledged rows. That
-- prefix is already the primary key's, so no index is added for it — but `da_xac_nhan_luc` is
-- NOT in the key, and a count FILTERed on it has to reach the heap. This partial index carries
-- just the acknowledged rows, which is the smaller half by construction and the half the filter
-- selects.
--
-- IT IS A PLAIN INDEX, NOT A UNIQUE ONE, so `WHERE da_xac_nhan_luc IS NOT NULL` frees no issued
-- number and reissues nothing — tools/check_khoa_duy_nhat.py refuses that shape only for UNIQUE
-- indexes, and for a reason that does not apply here.
CREATE INDEX IF NOT EXISTS thong_bao_nguoi_nhan_da_xac_nhan
    ON thong_bao_nguoi_nhan (tenant_id, thong_bao_id)
    WHERE da_xac_nhan_luc IS NOT NULL;

DROP TRIGGER IF EXISTS thong_bao_nguoi_nhan_cam_xoa_cung ON thong_bao_nguoi_nhan;
CREATE TRIGGER thong_bao_nguoi_nhan_cam_xoa_cung
    BEFORE DELETE ON thong_bao_nguoi_nhan
    FOR EACH ROW EXECUTE FUNCTION ho_so_luu_tru_cam_xoa_cung();

-- ---------------------------------------------------------------------------
-- thong_bao_nguoi_nhan_bat_bien — the recipient list is frozen, and an acknowledgement is final.
--
--   `dich_danh`        how this person came to be on the list is a fact about the moment of issue.
--                      Flipping it would rewrite §4's account of who was addressed directly.
--   `da_mo_luc`,       ONCE SET, THEY CANNOT BE CLEARED OR MOVED. §9.5 keeps an unacknowledged
--   `da_xac_nhan_luc`  announcement on a person's bell until they acknowledge it; a path that
--                      could clear the timestamp could put it back there after the fact, and the
--                      `{x}/{y}` figure reported upward would go DOWN with no event behind it.
--                      They may still go from NULL to a value — that is the ordinary act.
--
-- `thu_gui_luc`, `thu_loi_ma` and `cap_nhat_luc` stay editable: they are the outcome of sending,
-- which is retried (§9.4) and is not known when the row is written.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION thong_bao_nguoi_nhan_bat_bien() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.dich_danh IS DISTINCT FROM OLD.dich_danh THEN
        RAISE EXCEPTION 'thong_bao_nguoi_nhan: how this recipient was addressed is immutable'
            USING HINT = 'The recipient list is frozen at the moment of issue '
                         '(docs/ui-ux/08-thong-bao.md §9.3).';
    END IF;

    IF (OLD.da_mo_luc IS NOT NULL AND NEW.da_mo_luc IS DISTINCT FROM OLD.da_mo_luc)
    OR (OLD.da_xac_nhan_luc IS NOT NULL AND NEW.da_xac_nhan_luc IS DISTINCT FROM OLD.da_xac_nhan_luc) THEN
        RAISE EXCEPTION 'thong_bao_nguoi_nhan: opening and acknowledgement cannot be undone'
            USING HINT = 'A person read this announcement. Clearing the timestamp would lower the '
                         'acknowledgement count with no event behind it and would put the notice '
                         'back on their bell (§9.5).';
    END IF;

    RETURN NEW;
END $$;

DROP TRIGGER IF EXISTS thong_bao_nguoi_nhan_bat_bien ON thong_bao_nguoi_nhan;
CREATE TRIGGER thong_bao_nguoi_nhan_bat_bien
    BEFORE UPDATE ON thong_bao_nguoi_nhan
    FOR EACH ROW EXECUTE FUNCTION thong_bao_nguoi_nhan_bat_bien();

-- ---------------------------------------------------------------------------
-- WHAT THIS FILE DOES NOT STOP, said plainly rather than left to be discovered: TRUNCATE on any
-- of the three tables, and DDL by the table owner (ALTER TABLE ... DISABLE TRIGGER, dropping a
-- table). Same line ADR 0013 draws for the audit ledger — the job is to make the accidental and
-- the convenient impossible, not to defeat an administrator who has decided to destroy data and
-- is willing to be seen doing it.
--
-- REVERSAL (migration question 3). Every object here is new, and while the three tables are still
-- empty — which they are in every environment today — the reversal is complete and loses nothing:
--
--   DROP TABLE thong_bao_nguoi_nhan;    -- its 32 partitions, its index and its triggers go with it
--   DROP TABLE thong_bao_bo_phan;       -- same
--   DROP TABLE thong_bao;               -- LAST: both foreign keys point at it
--   DROP FUNCTION thong_bao_nguoi_nhan_bat_bien();
--   DROP FUNCTION thong_bao_bat_bien();
--   DROP FUNCTION ho_so_luu_tru_cam_xoa_cung();
--
-- and in the SAME transaction remove this file's row from `schema_migration`, otherwise the
-- runner still believes the schema is in place. Nothing from 0001–0004 is altered by this file,
-- so nothing there has to be put back; in particular `thong_bao_gui_cong_dan` and its own trigger
-- function are untouched and survive the reversal intact.
--
-- ONCE A COMMUNE HAS ISSUED ONE ANNOUNCEMENT HERE, THAT IS NO LONGER A REVERSAL — it destroys the
-- record of something a public authority told its own staff, together with every acknowledgement
-- collected against it. That is rule 7's first stop condition and needs the user, not a command.
-- From that point the way back is a NEW migration, and core/migrate has no automatic rollback for
-- exactly this reason (ADR 0013).
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- BACKSTOP: every partitioned table in this schema must actually have partitions.
--
-- A table declared PARTITION BY and given no partitions REJECTS EVERY INSERT, silently, until the
-- first real write — and because the audit entry shares the business transaction (rule 6,
-- invariant 3), that first write rolls back entirely. The check is repeated at the end of every
-- migration that declares a partitioned table, because it only verifies the state after a file
-- that CARRIES it (0002, §BACKSTOP).
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
