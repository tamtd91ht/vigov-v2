-- petitions — THE TASK REGISTER (`nhiem_vu`), its progress log (`nhat_ky_nhiem_vu`) and the
-- extension requests filed against it (`de_nghi_lui_han`).
--
-- This is the table FOUR other subsystems stand on: Nhiệm vụ, Sổ tay lãnh đạo, Biên bản họp, and
-- half of Tổng quan. Until this file there was no task table at all — only the two catalogues of
-- 0003.
--
-- WHY A NEW FILE AND NOT AN EDIT OF 0003/0004/0005: those have been applied and core/migrate
-- compares the checksum of every applied file at startup. Editing an applied file either stops the
-- service (ErrChecksumLech) or leaves two databases claiming one schema version while holding two
-- different schemas.
--
-- ENTITY OWNERSHIP is declared with the `-- @entity` / `-- @scope` marks above each CREATE TABLE
-- (ADR 0021).
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS, answered before the SQL.
--
--   1. HOW MANY ROWS PER COMMUNE: zero. All three tables are new and this file writes no row into
--      any of them. There is no seed, for the reason 0003 states at length: a row here carries
--      tenant_id, so seeding would mean seeding for ONE NAMED COMMUNE, and the step that sows a
--      commune's first rows — onboarding — does not exist in this repository.
--   2. IF IT STOPS HALF-WAY: it cannot land half-applied. One file, one transaction, with the
--      progress row written inside it. Every statement is IF NOT EXISTS or CREATE OR REPLACE, so a
--      retry costs nothing.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. WHICH READ PATHS CHANGE MEANING WHILE IT IS HALF-APPLIED: none. This file only ADDS tables,
--      two trigger functions and triggers. No existing table, column, index or constraint is
--      touched, and no query that runs today returns anything different. The two catalogues of
--      0003 gain INBOUND foreign keys, which constrains what may be written to `nhiem_vu` and
--      changes nothing about how `loai_nhiem_vu` / `muc_uu_tien_nhiem_vu` are read.
--   5. RETENTION: a task is an ADMINISTRATIVE RECORD (rule 7). `docs/ui-ux/02-nhiem-vu.md:356`
--      says so in the specification's own words — "nên xoá mềm để giữ nhật ký". Soft delete only,
--      and hard DELETE is refused by a trigger rather than by convention. The progress log is
--      APPEND-ONLY and has no soft-delete columns at all — see its own comment.
--
-- ---------------------------------------------------------------------------
-- WHAT THIS FILE DELIBERATELY DOES NOT CREATE, so the absence is not read as unfinished work:
--
--   * `nhiem_vu_cha_id` — THE SUB-TASK TREE. docs/ui-ux/02-nhiem-vu.md:208 and :306 want it, and
--     it is a STOP CONDITION rather than a column: the specification says a task has sub-tasks and
--     answers none of the three questions a self-referencing tree forces — how many levels deep,
--     whether a sub-task inherits the parent's deadline, and what happens to the children when the
--     parent is soft-deleted. §11.4 adds a fourth ("cha hoàn thành chỉ khi toàn bộ con đã hoàn
--     thành"), which is a rule about a shape nobody has fixed. Guessing produces a tree whose
--     semantics are invented by whoever writes the first write route, and by then there are rows.
--     ADDING IT LATER IS CHEAP — one nullable column on a table that is empty in every environment
--     — so the cost of waiting is close to nothing and the cost of guessing is not.
--
--   * `nhiem_vu_van_ban` — the three groups of related documents of §7.2 and §5.4 (`cap-tren-giao`
--     / `chi-dao-dang-uy` / `san-pham-dau-ra`). A separate pass; nothing below references it, and
--     the detail read route this migration serves renders the task without that block.
--
--   * A CATALOGUE TABLE FOR TASK STATUS. Open question #21 was DECIDED on 2026-09-22 (ADR 0035
--     §C): the commune may change the LABEL and the ORDER, never the LIST OF CODES. The code list
--     therefore belongs in a CHECK here — the same shape `phieu_phan_anh_trang_thai_hop_le` has in
--     0004 — and NOT in a `danh_muc` table with a `Tắt` button. ADR 0035 §C spells the consequence
--     out: "nút `Tắt` trên danh mục trạng thái nhiệm vụ phải KHÔNG CÓ", because disabling a status
--     that has tasks sitting in it drops those tasks out of every filter. The label/order override
--     surface (tier 2 of ADR 0026, the shape `nhan_linh_vuc` has) is a later pass and is reported
--     as a finding, not left to be inferred from this absence.
--
--   * ANY SEQUENCE FOR `ma`. §7.1 mints `NV01, NV02…` PER COMMUNE, and a PostgreSQL sequence is
--     per-table, not per-commune: one sequence would hand commune B the number after commune A's,
--     so the second commune's register starts at NV58. Minting belongs to the write path, inside
--     the transaction, against `UNIQUE (tenant_id, ma)` — which is what actually guarantees it.
--
-- ---------------------------------------------------------------------------
-- THE TWO DEADLINE COLUMNS, AND WHY ONE OF THEM MAY NEVER BE EDITED.
--
--   han_xu_ly    THE CURRENT COMMITMENT. Moves when an extension is approved (§11.2).
--   han_ban_dau  THE COMMITMENT AS FIRST MADE. Written once, at creation, and IMMUTABLE — the
--                trigger below refuses every change to it.
--
-- §5.6 states the rule on the screen ("HẠN BAN ĐẦU không đổi khi gia hạn — dùng cho thống kê đúng
-- hạn") and §11.3 states what it is for ("Tỷ lệ đúng hạn tính trên ngay_hoan_thanh ≤ han_ban_dau").
-- THAT IS WHY IT IS A TRIGGER AND NOT A CONVENTION: an UPDATE that moved `han_ban_dau` alongside
-- `han_xu_ly` would not fail, would not log, and would make every extension granted look like a
-- deadline met — retroactively, in a figure that goes upward.
--
-- BOTH ARE TIMESTAMPTZ AND THE SPECIFICATION'S §9 SAYS `date`. The divergence is deliberate and is
-- the same one service-documents already made (`0004_so_van_ban.sql:76-79`): the figure behind a
-- deadline is a count of WORKING HOURS (rule 10, invariant 4; ADR 0007), the `sla` table already
-- carries `loai_viec = 'nhiem-vu'` with hour columns
-- (service-identity/migrations/0008_sla.sql:176-178, :223), and no count in days can express "2
-- working hours". A DATE column would round every commitment to a day, silently, and would also
-- leave "midnight in which timezone" unanswered.
--
-- ⚠ NOTHING IN THIS FILE COMPUTES A DEADLINE, and nothing may. The arithmetic belongs to
-- identity.AdvanceWorkingHours and has exactly one implementation (ADR 0007). The act that FIXES
-- `han_xu_ly` is task creation, and it stores the result once (rule 10, invariant 2; ADR 0028) —
-- it is never recomputed on read.
--
-- THERE IS NO `qua_han` COLUMN AND THERE MUST NEVER BE ONE. Overdue is DERIVED, by comparing a
-- stored deadline with now (rule 10, invariant 3). A stored flag is wrong the moment a job is late
-- or a holiday is entered, and the stale copy is the one that reaches the report.
--
-- AND THERE IS NO "sắp đến hạn" THRESHOLD HERE EITHER. §3 offers that filter with a default of 72
-- hours, and the number is the commune's own `sla.gio_sap_den_han`
-- (service-identity/migrations/0008_sla.sql:178) — ONE threshold, read from that column, never a
-- second constant in source. identity exposes no RPC that returns the hours today (ADR 0029 §118),
-- so the filter is not implemented rather than implemented against an invented 72.

-- ---------------------------------------------------------------------------
-- PostgreSQL 13 is the floor, checked explicitly so the failure is readable.
--
-- BEFORE ... FOR EACH ROW triggers on a PARTITIONED table were only allowed from PostgreSQL 13. On
-- 11 and 12 the CREATE TRIGGER statements below fail with a message that reads like a syntax
-- mistake and invites somebody to "fix" it by moving the trigger down onto the partitions — where
-- a partition added later arrives silently unprotected.
-- ---------------------------------------------------------------------------
DO $$ BEGIN
    IF current_setting('server_version_num')::int < 130000 THEN
        RAISE EXCEPTION
            'nhiem_vu needs PostgreSQL 13 or newer (server is %). Do not weaken this migration to '
            'fit an older server — the triggers below are what stop an issued task number from '
            'being renumbered and an original deadline from being moved under a report.',
            current_setting('server_version');
    END IF;
END $$;

-- ---------------------------------------------------------------------------
-- nhiem_vu_bat_bien — the archival guard for the task register.
--
-- IT IS NOT ho_so_luu_tru_bat_bien (0004). That function guards the PETITION register and reads
-- `ma_tra_cuu` / `goc_dem_han` / `kenh_tiep_nhan`, none of which a task has; attaching it here
-- would fail at runtime on the first UPDATE, inside the business transaction — which, because the
-- audit entry shares that transaction (rule 6, invariant 3), rolls the whole act back.
--
-- WHAT EACH REFUSAL IS FOR:
--
--   DELETE         a task is an administrative record with the commune's own log hanging off it.
--                  §11.5 says so outright ("nên xoá mềm để giữ nhật ký"): a hard delete would take
--                  the timeline of who was told to do what with it, and would free the number.
--   `ma`           an issued number is never reissued or renumbered (rule 7, invariant 3). `NV19`
--                  is written on paper minutes, quoted in meeting conclusions and printed in the
--                  Sổ theo dõi; two tasks that have ever carried one number cannot be told apart
--                  afterwards.
--   `han_ban_dau`  the commitment as first made, and the denominator of §11.3's on-time ratio.
--                  Moving it makes every granted extension read as a deadline met.
--
-- The messages name the operation and the relation and nothing else: an error message travels into
-- logs and back to clients (rule 3, forbidden #3).
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION nhiem_vu_bat_bien() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'administrative record %: hard delete refused', TG_TABLE_NAME
            USING HINT = 'A task carries the commune''s own progress log and an issued number: '
                         'soft delete only (deleted_at, deleted_by, delete_reason) — rule 7, '
                         'invariant 1, and docs/ui-ux/02-nhiem-vu.md:356.';
    END IF;

    IF NEW.ma IS DISTINCT FROM OLD.ma THEN
        RAISE EXCEPTION 'administrative record %: `ma` is immutable', TG_TABLE_NAME
            USING HINT = 'An issued task number is never reissued and never renumbered (rule 7, '
                         'invariant 3). It is quoted in meeting minutes and printed in the Sổ '
                         'theo dõi, and nothing rewrites those.';
    END IF;

    IF NEW.han_ban_dau IS DISTINCT FROM OLD.han_ban_dau THEN
        RAISE EXCEPTION 'administrative record %: `han_ban_dau` is immutable', TG_TABLE_NAME
            USING HINT = 'It is the commitment as FIRST made and the denominator of the on-time '
                         'ratio (docs/ui-ux/02-nhiem-vu.md:188, :354). Granting an extension moves '
                         '`han_xu_ly` and nothing else; moving this one makes every extension read '
                         'as a deadline met, in a figure that goes upward.';
    END IF;

    RETURN NEW;
END $$;

-- ---------------------------------------------------------------------------
-- @entity: Task
-- @scope:  tenant
--
-- nhiem_vu — one task. `docs/ui-ux/02-nhiem-vu.md` calls this "module lõi": documents, meeting
-- conclusions and petitions all end up pouring work into this table.
--
-- IT HOLDS NO CITIZEN PERSONAL DATA and must not start to. The people named on it are MEMBERS OF
-- STAFF, referenced by their staff business code. A task born from a petition points at that
-- petition through `nguon_id` and carries none of the reporter's details across — see the note on
-- that column.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS nhiem_vu (
    tenant_id                     TEXT        NOT NULL,
    id                            TEXT        NOT NULL,

    -- THE ISSUED NUMBER — `NV19` (§4.1, §5.4). Minted per commune, in sequence, by the write path.
    --
    -- IT IS SEQUENTIAL AND THAT IS CORRECT HERE, which is worth saying because the neighbouring
    -- register forbids exactly that. `phieu_phan_anh.ma_tra_cuu` is handed to a CITIZEN and sits on
    -- a lookup path, so a guessable code reads other people's petitions (rule 4, invariant 4; rule
    -- 10, forbidden #5). A task number never leaves the staff surface: there is no citizen route to
    -- a task, and enumeration buys an attacker nothing they are not already authenticated for.
    ma                            TEXT        NOT NULL,

    -- The commune's task-type code, held as a value AND checked by a real foreign key.
    --
    -- A FOREIGN KEY IS RIGHT HERE and would be wrong for `khoi` below, and the difference is the
    -- service boundary, not taste: `loai_nhiem_vu` lives in THIS service's schema
    -- (0003_danh_muc_nhiem_vu.sql), so the key crosses nothing. It is the cheap version of the
    -- write-time check ADR 0024 asks for, and the evidence that it is needed is already in the
    -- specification: docs/ui-ux/14-cau-hinh.md §8 holds an SLA row pointing at a code that no
    -- longer exists in its catalogue, and the screen renders the raw slug. Same reasoning, same
    -- shape as service-identity/migrations/0005:369-374.
    --
    -- ⚠ CONSEQUENCE, STATED RATHER THAN DISCOVERED: the catalogues are EMPTY in every commune —
    -- 0003 seeds nothing, because seeding means naming a commune and the onboarding step does not
    -- exist. So NO TASK CAN BE CREATED until a commune's `loai_nhiem_vu` has rows. That is
    -- fail-closed and is the same state the petition intake is in (409 `sla_chua_cau_hinh`); the
    -- write route must answer with a sentence naming the catalogue, not with a constraint error.
    loai                          TEXT        NOT NULL,

    -- `khoi-uy-ban` | `khoi-dang` | `khac` — the commune's TaskBloc code, held AS A VALUE.
    --
    -- NO FOREIGN KEY, AND THERE CAN NEVER BE ONE: `khoi_nhiem_vu` lives in service `identity`
    -- (service-identity/migrations/0005_don_vi_dan_cu_va_danh_muc.sql:317), because the list
    -- changes in step with the ORG CHART rather than with tasks (ADR 0024:65). A key into another
    -- service's schema is rule 2, forbidden #2, and ADR 0024:130 forbids "tidying" the table over
    -- here to make one possible. The check belongs to the write path, over the contract.
    --
    -- NULL is "— Chưa xác định —", which §7.1 offers as a real choice on the form.
    khoi                          TEXT,

    -- "Nội dung nhiệm vụ / Trích yếu văn bản" for `theo-van-ban`, "Tên nhiệm vụ" for `co-ban`
    -- (§7.2, §7.3). ONE COLUMN FOR BOTH: the two labels name the same fact — what this task is —
    -- and two columns would leave every reader asking which one to render.
    tieu_de                       TEXT        NOT NULL,
    mo_ta                         TEXT,

    -- ONE OF SEVEN CODES. THE LIST IS CLOSED — open question #21, DECIDED 2026-09-22 (ADR 0035
    -- §C): a commune may change the label and the order, never the list. See the CHECK below and
    -- the note at the top of this file on why there is no catalogue table.
    trang_thai                    TEXT        NOT NULL DEFAULT 'moi-giao',

    -- The commune's priority code, foreign key into this service's own catalogue — same reasoning
    -- as `loai`.
    --
    -- NULLABLE, WHERE `loai` IS NOT. §7.1 marks the type required (✔) and the priority merely
    -- defaulted, and with an empty catalogue a NOT NULL here would assert a rank nobody chose. NULL
    -- means "not set"; the write path picks the catalogue's `la_mac_dinh` row, which is a decision
    -- made against the commune's own configuration rather than against a constant in source.
    muc_uu_tien                   TEXT,

    -- WHERE THE WORK CAME FROM (§3, §4.2). Four codes, closed: each names a different originating
    -- register, so a fifth is a new integration rather than a new label.
    nguon_giao                    TEXT        NOT NULL DEFAULT 'truc-tiep',

    -- The id of the record in that register — `ket_luan_hop.id` / `don_thu.id` / `phan_anh.id`
    -- (§9). NO FOREIGN KEY, and each of the three has its own reason: meeting conclusions have no
    -- table anywhere yet, `don_thu` belongs to service-documents, and `phieu_phan_anh` is in this
    -- schema but a task must survive as a record even if the petition it came from is soft-deleted.
    --
    -- IT IS AN ID AND NOT A COPY OF ANYTHING. A task born from a petition must not carry the
    -- reporter's name, number or text across: this table has no personal-data column and gains one
    -- the day somebody "denormalises for the list screen" (rule 3).
    nguon_id                      TEXT,

    -- WHO IS ANSWERABLE. `bo_phan_id` is identity's `bo_phan` id; the two staff columns hold STAFF
    -- BUSINESS CODES (`CB-2026-7K3M9Q`) and say so in their names — rule 6, invariant 8, and the
    -- same convention as service-documents (`can_bo_xu_ly_ma`, `nguoi_tao_ma`). §9 models them as
    -- uuid; a column holding both kinds of identifier is a column nobody can query, and the two are
    -- indistinguishable on sight.
    --
    -- BOTH NULLABLE: "— Chưa xác định —" for the department and `Chưa phân công` for the officer
    -- are real states the screen renders (§4.1, §4.2), and §11.1 makes "assigned to a department
    -- with nobody named for too long" a thing the system REPORTS rather than prevents.
    bo_phan_id                    TEXT,
    nguoi_thuc_hien_ma            TEXT,

    -- THE LEADER WHO ASSIGNED THE WORK — and the person an extension request is sent to (§5.8,
    -- §7.1: "Đề nghị lùi hạn sẽ gửi tới người này, qua chuông và qua thư").
    --
    -- A COLUMN OF ITS OWN, SEPARATE FROM `nguoi_tao_ma`, AND THAT SEPARATION IS THE WHOLE POINT.
    -- The BA/PM repository paid for this one and wrote down why (`kb/50-doi-chieu/`
    -- `2026-09-23-feat-m8-multitenant-foundation.md` §M1, migration `0042`): "văn thư nhập hộ phần
    -- lớn nhiệm vụ, và đề nghị lùi hạn nằm lại trên bàn văn thư". Routing the request to whoever
    -- typed the row sends it to a clerk who cannot decide it. THIS REPOSITORY'S OWN SPECIFICATION
    -- AGREES — §9 lists `lanh_dao_giao_viec_id` with the note "nhận đề nghị lùi hạn" — so the two
    -- sources say one thing and there is nothing to reconcile.
    lanh_dao_giao_viec_ma         TEXT,

    -- The `theo-van-ban` half of §5.4: "Cơ quan chủ trì tham mưu" (a department) and "Chuyên viên
    -- Văn phòng tham mưu / theo dõi" (an officer). Both NULL on a `co-ban` task, where §7.3 removes
    -- the fields from the form entirely.
    co_quan_chu_tri_id            TEXT,
    chuyen_vien_theo_doi_ma       TEXT,

    -- THE TWO CLOCKS. See the long note at the top of this file before touching either.
    han_xu_ly                     TIMESTAMPTZ,
    han_ban_dau                   TIMESTAMPTZ,

    -- "0% tiến độ ghi nhận" (§5.3). A number the officer reports, not one the system infers from
    -- the status: §6's lifecycle and §5.3's percentage are two different statements about the same
    -- work, and collapsing them would make one of the two a lie on every screen.
    tien_do                       INT         NOT NULL DEFAULT 0,

    tom_tat_ket_qua               TEXT,
    ghi_chu                       TEXT,

    -- THE TWO TICK BOXES OF §5.4, and §179 states the rule they carry in the interface itself:
    -- "Hai ô này đánh dấu bằng tay và KHÔNG LÀM ĐỔI TRẠNG THÁI NHIỆM VỤ". They are a record of an
    -- approval that happened on paper; the lifecycle is `trang_thai` and nothing else.
    lanh_dao_phe_duyet_hoan_thanh BOOLEAN     NOT NULL DEFAULT false,
    cap_tren_cong_nhan_hoan_thanh BOOLEAN     NOT NULL DEFAULT false,

    -- WHEN THE WORK WAS FINISHED. It is the numerator's instant in §11.3 ("ngay_hoan_thanh ≤
    -- han_ban_dau"), which is why the CHECK below ties it to the status: a task in `hoan-thanh`
    -- with no instant is a row the on-time ratio cannot classify, and it would be dropped from the
    -- denominator silently.
    ngay_hoan_thanh               TIMESTAMPTZ,

    -- Who booked the row, as a staff business code. NOT the same fact as `lanh_dao_giao_viec_ma`.
    nguoi_tao_ma                  TEXT        NOT NULL,

    deleted_at                    TIMESTAMPTZ,
    deleted_by                    TEXT,
    delete_reason                 TEXT,
    tao_luc                       TIMESTAMPTZ NOT NULL DEFAULT now(),
    cap_nhat_luc                  TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (tenant_id, id),

    -- COMPOSITE WITH tenant_id, and counting soft-deleted rows: an issued number is never reissued
    -- (rule 7, invariant 3). Restricted to live rows, a commune could soft-delete NV19 and mint a
    -- second NV19 — and the meeting minutes quoting the first would then name the second.
    UNIQUE (tenant_id, ma),

    -- SAME SERVICE, SAME SCHEMA — the key crosses no boundary. See the note on `loai`.
    FOREIGN KEY (tenant_id, loai) REFERENCES loai_nhiem_vu (tenant_id, ma),
    FOREIGN KEY (tenant_id, muc_uu_tien) REFERENCES muc_uu_tien_nhiem_vu (tenant_id, ma),

    CONSTRAINT nhiem_vu_ma_khong_rong CHECK (btrim(ma) <> ''),
    CONSTRAINT nhiem_vu_tieu_de_khong_rong CHECK (btrim(tieu_de) <> ''),

    -- THE SEVEN CODES OF §6, COPIED VERBATIM. Open question #21 decided the list is closed; ADR
    -- 0035 §C decided there is no `Tắt` for them either. Changing one of these strings is from now
    -- on a migration over records a commune has already closed, not a rename.
    CONSTRAINT nhiem_vu_trang_thai_hop_le CHECK (trang_thai IN (
        'moi-giao', 'da-tiep-nhan', 'dang-thuc-hien', 'cho-duyet', 'hoan-thanh',
        'tam-dung', 'chuyen-tiep')),

    CONSTRAINT nhiem_vu_nguon_giao_hop_le CHECK (nguon_giao IN (
        'truc-tiep', 'ket-luan-hop', 'van-ban-den', 'phan-anh')),

    -- A DIRECTLY ASSIGNED TASK POINTS AT NOTHING. The reverse implication — that every task from a
    -- meeting conclusion carries its id — is NOT enforced, deliberately: §8 imports tasks from a
    -- spreadsheet whose columns nobody has fixed, and a constraint that refuses real configuration
    -- is discovered on the day a commune is being set up (the lesson written into
    -- service-identity/migrations/0008_sla.sql:237-242). WHAT THAT COSTS, stated: a task marked
    -- `ket-luan-hop` with no `nguon_id` is invisible to §11.7's "x/y nhiệm vụ đã hoàn thành" on the
    -- Biên bản page, which under-counts rather than errors. Reported as a finding.
    CONSTRAINT nhiem_vu_truc_tiep_khong_co_nguon
        CHECK (nguon_giao <> 'truc-tiep' OR nguon_id IS NULL),

    CONSTRAINT nhiem_vu_tien_do_hop_le CHECK (tien_do BETWEEN 0 AND 100),

    -- THE TWO DEADLINES ARRIVE AND LEAVE TOGETHER. `han_ban_dau` is written once, from `han_xu_ly`,
    -- at creation. A row with one and not the other is either a commitment with no recorded origin
    -- — so §11.3's ratio has no denominator for it — or an origin for a commitment that was never
    -- made. Both are unreadable from the row afterwards.
    CONSTRAINT nhiem_vu_hai_han_cung_co_cung_khong
        CHECK ((han_xu_ly IS NULL) = (han_ban_dau IS NULL)),

    -- FINISHED IF AND ONLY IF THE INSTANT IS RECORDED. §11.3 computes the on-time ratio from
    -- `ngay_hoan_thanh`; a task in `hoan-thanh` without one silently leaves the numerator, and an
    -- instant on a task that is not finished would put it in.
    CONSTRAINT nhiem_vu_hoan_thanh_co_ngay
        CHECK ((trang_thai = 'hoan-thanh') = (ngay_hoan_thanh IS NOT NULL)),

    -- A SOFT DELETE CARRIES WHO AND WHY, OR IT IS NOT ONE (rule 7, invariant 1). Same shape
    -- service-documents uses on its two registers.
    CONSTRAINT nhiem_vu_xoa_mem_day_du
        CHECK ((deleted_at IS NULL AND deleted_by IS NULL AND delete_reason IS NULL)
            OR (deleted_at IS NOT NULL AND deleted_by IS NOT NULL AND delete_reason IS NOT NULL))
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS nhiem_vu_p%s PARTITION OF nhiem_vu '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

-- The register screen, newest first, and the tie-break the cursor pages on (core/store.QueryPage
-- orders by `(sort, id)`). Soft-deleted rows excluded EVERYWHERE, ALWAYS (rule 7, invariant 2).
CREATE INDEX IF NOT EXISTS nhiem_vu_so
    ON nhiem_vu (tenant_id, tao_luc DESC, id) WHERE deleted_at IS NULL;

-- The "Chỉ việc quá hạn" filter and the reminder job. OVERDUE IS STILL DERIVED — this index holds
-- the deadline, never a verdict about it (rule 10, invariant 3). Tasks with no deadline are OUT of
-- the index for the same reason they are out of the figure: nothing was promised.
CREATE INDEX IF NOT EXISTS nhiem_vu_han
    ON nhiem_vu (tenant_id, han_xu_ly)
    WHERE deleted_at IS NULL AND han_xu_ly IS NOT NULL;

-- `Giao cho tôi` (§3) — the screen an officer opens first thing in the morning.
CREATE INDEX IF NOT EXISTS nhiem_vu_nguoi_thuc_hien
    ON nhiem_vu (tenant_id, nguoi_thuc_hien_ma, tao_luc DESC) WHERE deleted_at IS NULL;

-- The department filter, and the `Liên quan đến tôi` branch that reads "bộ phận tôi đang giữ".
CREATE INDEX IF NOT EXISTS nhiem_vu_bo_phan
    ON nhiem_vu (tenant_id, bo_phan_id, tao_luc DESC) WHERE deleted_at IS NULL;

-- The Kanban board: five counted columns, one query per column (§4.1).
CREATE INDEX IF NOT EXISTS nhiem_vu_trang_thai
    ON nhiem_vu (tenant_id, trang_thai, tao_luc DESC) WHERE deleted_at IS NULL;

DROP TRIGGER IF EXISTS nhiem_vu_luu_tru ON nhiem_vu;
CREATE TRIGGER nhiem_vu_luu_tru
    BEFORE UPDATE OR DELETE ON nhiem_vu
    FOR EACH ROW EXECUTE FUNCTION nhiem_vu_bat_bien();

-- ---------------------------------------------------------------------------
-- @entity: TaskLogEntry
-- @scope:  tenant
--
-- nhat_ky_nhiem_vu — "Nhật ký & Trao đổi" (§5.9): what was done, what is stuck, and who was
-- holding the task at that moment.
--
-- APPEND-ONLY, ENFORCED BY A TRIGGER (rule 7, forbidden #5), and therefore WITH NO SOFT-DELETE
-- COLUMNS: there is no state of this table other than "everything that happened". Same shape and
-- same reasoning as service-documents' `lich_su_chuyen_van_ban`.
--
-- IT IS NOT THE AUDIT LOG AND DOES NOT REPLACE IT. `audit_log` answers "who changed what" for the
-- whole service and is invisible to a commune; this is a BUSINESS record the drawer renders and an
-- officer reads and writes. Both are written, in the same transaction, for the same act.
--
-- # WHY `bo_phan_id` AND `nguoi_phu_trach_ma` SIT ON A LOG ROW AT ALL
--
-- The task's own `bo_phan_id` / `nguoi_thuc_hien_ma` say who is holding it NOW. §5.9 renders "thông
-- tin bộ phận/phụ trách khi có thay đổi phân công", i.e. who it was handed to AT THAT STEP, and
-- that cannot be recovered from the current values. The BA/PM repository hit the same wall and
-- recorded it (`kb/50-doi-chieu/2026-09-23-feat-m8-multitenant-foundation.md` §M4, `92a722e`):
-- without the column, "ai giữ việc lúc quá hạn" has no answer.
--
-- # AND WHY THERE IS NO `tu_bo_phan_id` / `tu_nguoi_phu_trach_ma` PAIR
--
-- The BA/PM input asks for from→to on both axes (§M1, migration `0043`). THIS REPOSITORY'S
-- SPECIFICATION ASKS FOR ONE SIDE — §9 lists `bo_phan_id, nguoi_phu_trach_id` and nothing else —
-- and the specification wins where the two differ. It is also the answer this repository already
-- reached for the identical question about `service-documents`, with the reasoning written out in
-- that same note (§Đã sửa, row 5): THE "FROM" IS DERIVABLE — it is the value on the previous log
-- row of this task, and NULL when there is none. A second copy of a fact already in the table is
-- what rule 9's one-line test forbids, and the copy that drifts is the one a screen reads.
-- The divergence is reported rather than silently absorbed.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS nhat_ky_nhiem_vu (
    tenant_id                TEXT        NOT NULL,
    id                       TEXT        NOT NULL,

    -- The task this entry belongs to. Not a foreign key, for the reason
    -- service-documents/migrations/0004_so_van_ban.sql:500-503 gives: a historical entry must
    -- survive a future reshaping of the register, and the write path reads the task under a row
    -- lock in the same transaction — a stronger check than a constraint on an id.
    nhiem_vu_id              TEXT        NOT NULL,

    thoi_diem                TIMESTAMPTZ NOT NULL,

    -- WHO WROTE IT, as a staff business code (`CB-2026-7K3M9Q`) — rule 6, invariant 8. This is the
    -- name somebody reads years later, when a ULID would name nobody.
    nguoi_ma                 TEXT        NOT NULL,

    -- The state the task was in AT THIS MOMENT — §5.9 renders it as a chip on the timeline row.
    -- Stored rather than derived: the task's current state is one value and the timeline needs the
    -- state at each step.
    trang_thai_tai_thoi_diem TEXT        NOT NULL,

    -- Who was holding the task at this step. Both NULL when this entry changed no assignment.
    bo_phan_id               TEXT,
    nguoi_phu_trach_ma       TEXT,

    -- "Đã làm được gì, còn vướng gì…". MANDATORY: an entry with nothing in it is a row nobody can
    -- act on, and this table is never edited afterwards.
    noi_dung                 TEXT        NOT NULL,

    -- The `📎 Đính kèm` list of §5.9, as protojson-shaped references.
    --
    -- ⚠ WHAT MAY NOT GO IN IT: a file NAME chosen by a citizen or describing a case (rule 3,
    -- forbidden #4 — personal data in file names). There is no file store in this repository yet,
    -- so nothing writes this column today; it defaults to an empty array so a reader never has to
    -- distinguish "no attachments" from "column not set".
    dinh_kem                 JSONB       NOT NULL DEFAULT '[]'::jsonb,

    tao_luc                  TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (tenant_id, id),

    -- The same seven codes as the register. Written out rather than referenced, because a CHECK
    -- cannot borrow another table's — and a timeline holding a status the register would refuse is
    -- a timeline that contradicts the row it hangs off.
    CONSTRAINT nhat_ky_nhiem_vu_trang_thai_hop_le CHECK (trang_thai_tai_thoi_diem IN (
        'moi-giao', 'da-tiep-nhan', 'dang-thuc-hien', 'cho-duyet', 'hoan-thanh',
        'tam-dung', 'chuyen-tiep')),

    CONSTRAINT nhat_ky_nhiem_vu_noi_dung_khong_rong CHECK (btrim(noi_dung) <> '')
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS nhat_ky_nhiem_vu_p%s PARTITION OF nhat_ky_nhiem_vu '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

-- The drawer reads one task's timeline, newest first.
CREATE INDEX IF NOT EXISTS nhat_ky_theo_nhiem_vu
    ON nhat_ky_nhiem_vu (tenant_id, nhiem_vu_id, thoi_diem DESC);

-- ---------------------------------------------------------------------------
-- The append-only guard. Same shape as service-documents' `lich_su_chuyen_chi_them` and for the
-- same reasons, including why the TRUNCATE half has to be attached per partition:
--
--   * a row-level trigger on the PARENT is cloned onto every existing partition and onto every
--     partition added later, and an UPDATE against a partitioned table fires on the leaf — so a
--     statement typed straight at `nhat_ky_nhiem_vu_p07` hits the clone too;
--   * PostgreSQL refuses a TRUNCATE trigger on a partitioned table, and statement-level triggers
--     are not cloned — so TRUNCATE is covered leaf by leaf. MODULUS 32 is fixed for the life of the
--     system (ADR 0010), and this file is re-runnable if that ever changes.
--
-- INSERT is absent from the event list on purpose: the write path pays nothing.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION nhat_ky_nhiem_vu_chi_them() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'nhat_ky_nhiem_vu is append-only: % on % refused', TG_OP, TG_TABLE_NAME
        USING HINT = 'A progress entry is a historical record (rule 7, forbidden #5): never '
                     'modified, never removed, never truncated. To correct one, write another '
                     'entry carrying the correction.';
END $$;

DROP TRIGGER IF EXISTS nhat_ky_nhiem_vu_khong_sua_xoa ON nhat_ky_nhiem_vu;
CREATE TRIGGER nhat_ky_nhiem_vu_khong_sua_xoa
    BEFORE UPDATE OR DELETE ON nhat_ky_nhiem_vu
    FOR EACH ROW EXECUTE FUNCTION nhat_ky_nhiem_vu_chi_them();

DO $$
DECLARE part regclass;
BEGIN
    FOR part IN
        SELECT inhrelid::regclass FROM pg_inherits
        WHERE inhparent = 'nhat_ky_nhiem_vu'::regclass
    LOOP
        EXECUTE format('DROP TRIGGER IF EXISTS nhat_ky_nhiem_vu_khong_truncate ON %s', part);
        EXECUTE format(
            'CREATE TRIGGER nhat_ky_nhiem_vu_khong_truncate BEFORE TRUNCATE ON %s '
            'FOR EACH STATEMENT EXECUTE FUNCTION nhat_ky_nhiem_vu_chi_them()', part);
    END LOOP;
END $$;

-- ---------------------------------------------------------------------------
-- @entity: TaskExtensionRequest
-- @scope:  tenant
--
-- de_nghi_lui_han — "Đề nghị lùi hạn" (§5.8): a new deadline, a reason, and a decision.
--
-- # "CHỜ DUYỆT LÙI HẠN" IS A LABEL, NOT A TASK STATUS — AND THAT IS WHY THIS IS A TABLE
--
-- A pending request does NOT move `nhiem_vu.trang_thai`. The BA/PM repository states the cost of
-- getting this wrong in one line (`kb/50-doi-chieu/2026-09-23-feat-m8-multitenant-foundation.md`
-- §M1): folding it into the status set makes the progress board COUNT WRONG — every task awaiting
-- a decision leaves the column it is actually in, so `Đang thực hiện` under-reports and the Kanban
-- header shows a number nobody can reconcile. THIS REPOSITORY'S SPECIFICATION AGREES BY OMISSION:
-- §6's seven codes contain no such state. The screen reads this table and draws a chip.
--
-- # THE ORIGINAL DEADLINE IS NOT TOUCHED BY ANY OF THIS
--
-- §5.8 promises it on the screen — "Hạn gốc vẫn được giữ lại để báo cáo đúng hạn không bị lùi
-- theo" — and `nhiem_vu_bat_bien` above is what makes the promise true rather than intended.
-- Approving a request moves `nhiem_vu.han_xu_ly` and nothing else.
--
-- # WHO DECIDES IT
--
-- The request goes to `nhiem_vu.lanh_dao_giao_viec_ma` — the leader named ON THE RECORD (§5.8,
-- §7.1) — and the permission that names the act is `task.extend`, already seeded at
-- service-identity/migrations/0001_init.sql:303 ("Duyệt gia hạn"). NO KEY IS INVENTED HERE.
-- Whether holding the key is enough, or whether being the named leader is also required, is a
-- WRITE-PATH question and is not decided by this schema: the two facts are both recorded, and the
-- route that uses them is not in this pass.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS de_nghi_lui_han (
    tenant_id        TEXT        NOT NULL,
    id               TEXT        NOT NULL,

    -- Not a foreign key, for the same reason the log's is not: the write path reads the task under
    -- a row lock in the same transaction.
    nhiem_vu_id      TEXT        NOT NULL,

    -- Staff business codes (rule 6, invariant 8). `nguoi_duyet_ma` is NULL until somebody decides.
    nguoi_de_nghi_ma TEXT        NOT NULL,
    nguoi_duyet_ma   TEXT,

    -- The deadline being asked for. TIMESTAMPTZ for the reason the register's two columns are —
    -- see the note at the top of this file.
    han_moi          TIMESTAMPTZ NOT NULL,

    -- MANDATORY. §5.8 puts `Lý do` on the form, and a request with no reason is one the leader
    -- cannot decide and nobody can account for afterwards.
    ly_do            TEXT        NOT NULL,

    trang_thai       TEXT        NOT NULL DEFAULT 'cho-duyet',

    -- When it was filed. §9 calls this `thoi_diem`; the decision instant is a SECOND fact the
    -- specification does not list, and it is here because without it "when was this commitment
    -- moved" has no answer in the schema — which is exactly the question an inspection asks after a
    -- deadline slips. Stated as an addition rather than smuggled in.
    thoi_diem        TIMESTAMPTZ NOT NULL,
    duyet_luc        TIMESTAMPTZ,

    deleted_at       TIMESTAMPTZ,
    deleted_by       TEXT,
    delete_reason    TEXT,
    tao_luc          TIMESTAMPTZ NOT NULL DEFAULT now(),
    cap_nhat_luc     TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- AT MOST ONE REQUEST AWAITING A DECISION PER TASK.
    --
    -- Two pending requests carrying two different `han_moi` is a leader with two buttons and no way
    -- to tell what approving either one means; approving both moves the commitment twice, and the
    -- second move is invisible on a screen showing one chip.
    --
    -- THE SHAPE IS THE ONE 0003 AND service-identity/0008 ALREADY PROVED OUT, and it is deliberate:
    -- a PARTIAL unique index would be the obvious form, but unique keys on a PARTITIONED table are
    -- restricted and whether the partial variant is accepted cannot be verified from this
    -- repository — no PostgreSQL is reachable and VIGOV_TEST_DSN is unset, so the integration
    -- suites skip without running a statement. A migration that fails at startup stops the service.
    -- So: a GENERATED column that folds the state into NULL, and a plain unique key over plain
    -- columns. NULLs do not collide, so a rejected or approved request frees the slot by itself.
    moc_cho_duyet    BOOLEAN GENERATED ALWAYS AS
                         (CASE WHEN trang_thai = 'cho-duyet' AND deleted_at IS NULL THEN true END)
                         STORED,

    PRIMARY KEY (tenant_id, id),
    UNIQUE (tenant_id, nhiem_vu_id, moc_cho_duyet),

    CONSTRAINT de_nghi_lui_han_trang_thai_hop_le
        CHECK (trang_thai IN ('cho-duyet', 'da-duyet', 'tu-choi')),

    CONSTRAINT de_nghi_lui_han_ly_do_khong_rong CHECK (btrim(ly_do) <> ''),

    -- A DECIDED REQUEST NAMES ITS DECIDER AND ITS INSTANT; A PENDING ONE HAS NEITHER. Without this,
    -- an approval that moved a commitment could sit in the register attributed to nobody — and rule
    -- 6, invariant 8 exists because that is the row an inspection asks about.
    CONSTRAINT de_nghi_lui_han_quyet_dinh_day_du
        CHECK ((trang_thai = 'cho-duyet'
                    AND nguoi_duyet_ma IS NULL AND duyet_luc IS NULL)
            OR (trang_thai <> 'cho-duyet'
                    AND nguoi_duyet_ma IS NOT NULL AND duyet_luc IS NOT NULL)),

    CONSTRAINT de_nghi_lui_han_xoa_mem_day_du
        CHECK ((deleted_at IS NULL AND deleted_by IS NULL AND delete_reason IS NULL)
            OR (deleted_at IS NOT NULL AND deleted_by IS NOT NULL AND delete_reason IS NOT NULL))
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS de_nghi_lui_han_p%s PARTITION OF de_nghi_lui_han '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

-- The drawer reads one task's requests, newest first.
CREATE INDEX IF NOT EXISTS de_nghi_lui_han_theo_nhiem_vu
    ON de_nghi_lui_han (tenant_id, nhiem_vu_id, thoi_diem DESC) WHERE deleted_at IS NULL;

-- ---------------------------------------------------------------------------
-- de_nghi_lui_han_bat_bien — the request AS MADE is a historical fact; only the decision moves.
--
-- NOT APPEND-ONLY, unlike the progress log: this row has exactly one legitimate transition, from
-- `cho-duyet` to a decision. What may never change is what was ASKED FOR — editing `han_moi` after
-- an approval would move a commitment that a leader approved at a different number, and the audit
-- entry would still name the act they actually performed.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION de_nghi_lui_han_bat_bien() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'administrative record %: hard delete refused', TG_TABLE_NAME
            USING HINT = 'An extension request records who asked to move a commitment and what the '
                         'answer was: soft delete only (rule 7, invariant 1).';
    END IF;

    IF NEW.nhiem_vu_id  IS DISTINCT FROM OLD.nhiem_vu_id
    OR NEW.nguoi_de_nghi_ma IS DISTINCT FROM OLD.nguoi_de_nghi_ma
    OR NEW.han_moi      IS DISTINCT FROM OLD.han_moi
    OR NEW.ly_do        IS DISTINCT FROM OLD.ly_do
    OR NEW.thoi_diem    IS DISTINCT FROM OLD.thoi_diem THEN
        RAISE EXCEPTION 'administrative record %: the request as filed is immutable', TG_TABLE_NAME
            USING HINT = 'Only the decision may move (trang_thai, nguoi_duyet_ma, duyet_luc). '
                         'Rewriting the requested date or the reason after a decision leaves an '
                         'approval attached to something the approver never read.';
    END IF;

    RETURN NEW;
END $$;

DROP TRIGGER IF EXISTS de_nghi_lui_han_luu_tru ON de_nghi_lui_han;
CREATE TRIGGER de_nghi_lui_han_luu_tru
    BEFORE UPDATE OR DELETE ON de_nghi_lui_han
    FOR EACH ROW EXECUTE FUNCTION de_nghi_lui_han_bat_bien();

-- ---------------------------------------------------------------------------
-- WHAT THIS FILE DOES NOT STOP, said plainly rather than left to be discovered: TRUNCATE on
-- `nhiem_vu` and `de_nghi_lui_han`, and DDL by the table owner (ALTER TABLE ... DISABLE TRIGGER,
-- dropping a table). Same line ADR 0013 draws for the audit ledger — the job is to make the
-- accidental and the convenient impossible, not to defeat an administrator who has decided to
-- destroy data and is willing to be seen doing it.
--
-- REVERSAL (migration question 3). Every object here is new, and while the three tables are still
-- empty — which they are in every environment today — the reversal is complete and loses nothing:
-- drop the three tables, then the three trigger functions, and in the same transaction remove this
-- file's row from `schema_migration`, otherwise the runner still believes the schema is in place.
-- The 96 partitions and the triggers go with their parents. The two foreign keys into 0003's
-- catalogues go with `nhiem_vu`; nothing in 0003 is altered by this file, so nothing there has to
-- be put back.
--
-- ONCE A COMMUNE HAS ONE TASK HERE, THAT IS NO LONGER A REVERSAL — it is the destruction of
-- administrative records AND of an issued number series, which is rule 7's first stop condition and
-- needs the user, not a command. From that point the way back is a NEW migration, and core/migrate
-- has no automatic rollback for exactly this reason (ADR 0013).
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- BACKSTOP: every partitioned table in this schema must actually have partitions.
--
-- A table declared PARTITION BY and given no partitions REJECTS EVERY INSERT, silently, until the
-- first real write — and because the audit entry shares the business transaction (rule 6, invariant
-- 3), that first write rolls back entirely. The check is repeated at the end of every migration
-- that declares a partitioned table, because it only verifies the state after a file that CARRIES
-- it (0002, §BACKSTOP).
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
