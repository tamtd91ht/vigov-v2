-- 0033 — the commune's CITIZEN CONTACT DIRECTORY ("Danh bạ người dân"): three tables, no row written.
--
--   citizen_contact        one person the commune can reach — name, phone, residential unit, labels
--   citizen_group          a named group the commune defines ("Hộ nghèo", "Tổ 3") to address many at once
--   citizen_contact_group  the many-to-many membership between the two
--
-- Owner decisions of 16/10/2026 fix the columns, the enum values, the keys and which tables are
-- partitioned; this file writes them down and says where it had to choose.
--
-- WHY A NEW FILE AND NOT AN EDIT TO 0004 / 0005: core/migrate checksums every applied file at startup
-- and stops the service when one has changed (ErrChecksumLech).
--
-- WHY THIS IS NOT `dinh_danh_cong_dan` (0004): that table is a platform-wide identity a citizen
-- creates by signing in to the Mini App (ADR 0002, 0020). A directory row is created BY STAFF, about a
-- person who may never open the app, and belongs to ONE commune. Two different facts; joining them
-- (which contact is which app user) is a later decision, not a column here.
--
-- ENTITY OWNERSHIP via the `-- @entity` / `-- @scope` marks (ADR 0021); tools/kb reads them into
-- kb/30-indexes/data-ownership.json. Nobody edits that file.
--
-- NAMES (rule 12, ADR 0051): tables, columns and constraints are English. The timestamp columns are
-- `created_at` / `updated_at` as in 0027 and 0031 — the two files written under rule 12 — not the
-- older `tao_luc` / `cap_nhat_luc` of 0004 / 0005, which stay where they are (rule 12, invariant 3).
-- Enum VALUES are Vietnamese without diacritics (ADR 0011), because a person reads them in a query
-- years later: `source` is 'nhap-tay' | 'nhap-excel'.
--
-- ---------------------------------------------------------------------------
-- PARTITIONING (ADR 0096). Decision #1: only a table whose rows grow with time or with the number of
-- citizens is partitioned; decision #3: catalogues and configuration are plain tables.
--
--   citizen_contact         grows WITH CITIZENS (one row per person, thousands per commune after an
--                           Excel import)                      → PARTITION BY HASH (tenant_id), MODULUS 8
--   citizen_contact_group   grows with citizens × groups        → PARTITION BY HASH (tenant_id), MODULUS 8
--   citizen_group           a few dozen rows a commune defines  → plain table (#3)
--
-- The two partitioned tables are added to migrations/partitioning_test.go `hashPartitionedTables`,
-- which IS the current list (ADR 0096: "danh sách hiện hành về sau đọc từ migration").
--
-- FOREIGN KEYS FROM A PARTITIONED TABLE: PostgreSQL has allowed a partitioned table to REFERENCE a
-- regular table since 11, and to reference another partitioned table since 12; the cluster is 16. The
-- referenced key must be unique across every partition, which `PRIMARY KEY (tenant_id, id)` is — it
-- contains the partition key. Every key below is composite with `tenant_id` (rule 1, invariant 6), so
-- another commune's residential unit, contact or group is refused by the key itself.
--
-- ---------------------------------------------------------------------------
-- PERSONAL DATA (rule 3). `phone`, `full_name`, `address` and `full_name_folded` (derived from the
-- name) are citizen personal data under Decree 13/2023. Never logged, masked at the API (MaskPhone),
-- never in a URL, file name, cache key or event. The column COMMENTs below say so, as 0004 does for
-- `dinh_danh_cong_dan.so_dien_thoai`, so a person reading `\d+` in psql sees it too.
--
-- `phone` IS STORED NORMALISED — digits only, 9 to 15 of them. The APPLICATION normalises ("+84 9xx"
-- → "849xx", spaces and dots removed); the CHECK refuses anything else so a half-normalised number
-- can never make the same person two rows. Test data uses 0900000000 (rule 3, invariant 5).
--
-- ---------------------------------------------------------------------------
-- WHY PHONE UNIQUENESS COUNTS LIVE ROWS ONLY (`live_key`) — and why this is not the trap
-- tools/check_khoa_duy_nhat.py guards against. Rule 7 invariant 3 forbids REISSUING AN ISSUED CODE:
-- a document number, a petition lookup code, a staff code — something the commune printed and
-- signed. A phone number is not a code the commune issues; it belongs to the person. A contact
-- removed by mistake and added back the next day is the normal case, and refusing it would leave the
-- commune unable to reach that person at all. So: UNIQUE (tenant_id, phone, live_key) — TRUE while
-- live, NULL once soft-deleted, NULLs never collide (the `sla.linh_vuc_khoa` shape of 0008, 0027,
-- 0031). A GENERATED column + composite UNIQUE rather than a partial unique index, because
-- `make khoaduynhat` rejects `UNIQUE … WHERE deleted_at IS NULL` on sight, and because a partial
-- unique index on a partitioned table could not be verified on this machine.
--
-- The same holds for `citizen_group.name`: a group name is not an issued code. Uniqueness is on
-- `name_folded`, so "Hộ nghèo" and "Ho ngheo" are one group, not two.
--
-- ---------------------------------------------------------------------------
-- ACCENT-INSENSITIVE SEARCH: `full_name_folded` and `name_folded` are STORED GENERATED columns
-- computed by the expression core/textfold.SQL renders — the precedent of service-petitions 0041.
-- migrations/citizen_directory_test.go fails if this text and that function ever disagree; the store
-- must fold the search term with the SAME SQL expression (textfold package comment: never in Go).
-- NO INDEX ON THEM: the search is `LIKE '%term%'`, which no btree serves (0041's reasoning); the scan
-- is bounded by one commune's partition.
--
-- ---------------------------------------------------------------------------
-- MEMBERSHIP ROWS ARE SOFT-REMOVED, NOT DELETED — the staff↔role precedent (0031, owner 13/10/2026).
-- Three reasons, in order of weight:
--   1. Rule 7, invariant 1: business data is soft-deleted. A group is how the commune addresses
--      people (notices, ADR 0041's channel); "was this person in the group when that notice went
--      out" is a question an inspection may ask, and only a kept row answers it.
--   2. The app layer could not hard-delete anyway: hooks/data_safety_guard.py blocks a hard-delete
--      statement on any table whose name contains `citizen`.
--   3. Consistency: one shape for every membership table in this service.
-- CONSEQUENCE FOR THE KEY. The owner's card names PRIMARY KEY (tenant_id, contact_id, group_id).
-- With soft removal, a person removed from a group and added back later must be a SECOND row, never
-- an edit of the first (rule 7, forbidden #5; 0031 says the same) — and a PK on the triple alone
-- refuses that second row. So, exactly as 0031: PRIMARY KEY (tenant_id, contact_id, group_id,
-- created_at) and UNIQUE (tenant_id, contact_id, group_id, live_key) for "one LIVE membership per
-- person and group". Stated here rather than decided silently; if the owner prefers the triple as
-- PK, the alternative is hard removal, and that is their call (rule 7, stop condition #1).
--
-- ---------------------------------------------------------------------------
-- RESERVED COLUMNS, SAID OUT LOUD. `household_code` is reserved (owner, 16/10/2026) and NOTHING reads
-- or writes it; it is nullable with no CHECK so the day it is used is an app change, not a migration.
-- `import_batch_id` names the Excel import run a row came from (service-side ULID of the run), so a
-- bad import can be listed — and reviewed by a person, never bulk-removed (rule 7, forbidden #1).
-- A THIRD `source` VALUE (e.g. the citizen's own registration through the Mini App) is deliberately
-- NOT in the CHECK: adding it is a decision about which app user becomes which contact (see the
-- `dinh_danh_cong_dan` note above), and a CHECK is the cheap place to widen later.
--
-- AUDIT: rows are written only by staff routes of this service (not yet built), which write their
-- audit_log entry in the same transaction (rule 6). No trigger here — no identity business table
-- carries one except the append-only logs (0002).
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. ROWS PER COMMUNE: zero written. Three empty tables, 16 partitions, 4 indexes. Expected size
--      once used: thousands of contacts a commune (one Excel import), tens of groups, contacts × a
--      few memberships.
--   2. IF IT STOPS HALF-WAY: it cannot land half-applied — one file, one transaction with its progress
--      row (core/migrate). Every statement is IF NOT EXISTS, so a retry costs nothing. No backfill.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. READ PATHS THAT CHANGE MEANING: none. Three new empty tables; no existing object is touched.
--      `thon_to_dan_pho` gains an incoming foreign key, which changes one thing for ITS writers: a
--      residential unit that a live or removed contact points at can no longer be hard-deleted —
--      which its own trigger (0005 thon_to_dan_pho_giu_ban_ghi) already refuses.
--   5. RETENTION: a contact row is personal data with a business history (who was told what, through
--      which group). The soft-delete columns are present on every table before the first row is
--      written (rule 7, invariant 1); an erasure request under Decree 13 means anonymise the
--      identifying columns and keep the row (rule 7, invariant 7). The folded name is GENERATED, so
--      anonymising `full_name` anonymises it in the same UPDATE.
-- ---------------------------------------------------------------------------

-- @entity: CitizenContact
-- @scope:  tenant
CREATE TABLE IF NOT EXISTS citizen_contact (
    tenant_id            TEXT        NOT NULL,
    id                   TEXT        NOT NULL,   -- ULID

    -- PERSONAL DATA (rule 3). The fold is derived from it and is personal data too.
    full_name            TEXT        NOT NULL,
    full_name_folded     TEXT        GENERATED ALWAYS AS (
        lower(translate(translate(full_name, U&'\0300\0301\0302\0303\0306\0309\031B\0323', ''), 'àáảãạăằắẳẵặâầấẩẫậÀÁẢÃẠĂẰẮẲẴẶÂẦẤẨẪẬèéẻẽẹêềếểễệÈÉẺẼẸÊỀẾỂỄỆìíỉĩịÌÍỈĨỊòóỏõọôồốổỗộơờớởỡợÒÓỎÕỌÔỒỐỔỖỘƠỜỚỞỠỢùúủũụưừứửữựÙÚỦŨỤƯỪỨỬỮỰỳýỷỹỵỲÝỶỸỴđĐÐ', 'aaaaaaaaaaaaaaaaaAAAAAAAAAAAAAAAAAeeeeeeeeeeeEEEEEEEEEEEiiiiiIIIIIoooooooooooooooooOOOOOOOOOOOOOOOOOuuuuuuuuuuuUUUUUUUUUUUyyyyyYYYYYdDD'))
    ) STORED,
    -- PERSONAL DATA (rule 3). Digits only, normalised by the application (header).
    phone                TEXT        NOT NULL,

    -- The residential unit (thôn / tổ dân phố) of THIS commune, or NULL when not entered. A composite
    -- foreign key with one NULL column is not checked (MATCH SIMPLE), which is exactly the meaning:
    -- no unit, no reference.
    residential_unit_id  TEXT,
    -- PERSONAL DATA (rule 3).
    address              TEXT,
    note                 TEXT,
    -- Free labels the commune types ("hộ nghèo", "cựu chiến binh"). Not a catalogue, by decision.
    tags                 TEXT[]      NOT NULL DEFAULT '{}',
    -- 'nhap-tay' (typed by staff) · 'nhap-excel' (Excel import). A third value is left for later
    -- (header, RESERVED COLUMNS).
    source               TEXT        NOT NULL,
    -- Switched off = kept in the directory but not addressed. Distinct from soft delete: an inactive
    -- row is still listed and editable.
    is_active            BOOLEAN     NOT NULL DEFAULT true,
    -- RESERVED (owner, 16/10/2026). Nothing reads or writes it yet.
    household_code       TEXT,
    -- The import run this row came from (ULID), NULL for a typed row.
    import_batch_id      TEXT,

    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at           TIMESTAMPTZ,
    deleted_by           TEXT,
    delete_reason        TEXT,

    -- TRUE while live, NULL once soft-deleted (header: WHY PHONE UNIQUENESS COUNTS LIVE ROWS ONLY).
    live_key             BOOLEAN GENERATED ALWAYS AS (CASE WHEN deleted_at IS NULL THEN TRUE END) STORED,

    -- Composite with tenant_id (rule 1, invariant 6); contains the partition key.
    PRIMARY KEY (tenant_id, id),
    CONSTRAINT citizen_contact_phone_live_unique
        UNIQUE (tenant_id, phone, live_key),

    CONSTRAINT citizen_contact_residential_unit_fkey
        FOREIGN KEY (tenant_id, residential_unit_id) REFERENCES thon_to_dan_pho (tenant_id, id),

    CONSTRAINT citizen_contact_id_ulid CHECK (length(id) = 26),
    CONSTRAINT citizen_contact_full_name_not_blank CHECK (btrim(full_name) <> ''),
    -- Digits only, 9–15: the application normalises; the schema refuses what it missed.
    CONSTRAINT citizen_contact_phone_digits CHECK (phone ~ '^[0-9]{9,15}$'),
    CONSTRAINT citizen_contact_source_valid
        CHECK (source IN ('nhap-tay', 'nhap-excel')),
    CONSTRAINT citizen_contact_import_batch_ulid
        CHECK (import_batch_id IS NULL OR length(import_batch_id) = 26),

    -- Soft-delete columns are all three or none (rule 7, invariant 1); `deleted_by` is a staff
    -- business code (rule 6, invariant 8).
    CONSTRAINT citizen_contact_soft_delete_complete
        CHECK ((deleted_at IS NULL AND deleted_by IS NULL AND delete_reason IS NULL)
            OR (deleted_at IS NOT NULL AND deleted_by IS NOT NULL AND btrim(deleted_by) <> ''
                AND delete_reason IS NOT NULL AND btrim(delete_reason) <> ''
                AND char_length(delete_reason) <= 500))
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..7 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS citizen_contact_p%s PARTITION OF citizen_contact '
            'FOR VALUES WITH (MODULUS 8, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

COMMENT ON TABLE citizen_contact IS
    'Danh ba nguoi dan cua xa (chu du an chot 16/10/2026). Mot dong = mot nguoi can bo co the lien '
    'he; khong phai dinh danh cong dan Mini App (dinh_danh_cong_dan). Xoa mem, khong xoa that; '
    'yeu cau xoa theo Nghi dinh 13 = an danh cac cot dinh danh, giu dong.';
COMMENT ON COLUMN citizen_contact.full_name IS
    'DU LIEU CA NHAN (luat 3). Khong vao log, khong vao URL / ten tep / khoa cache.';
COMMENT ON COLUMN citizen_contact.full_name_folded IS
    'DU LIEU CA NHAN (luat 3), sinh tu full_name de tim khong dau (core/textfold). Khong SELECT ra API.';
COMMENT ON COLUMN citizen_contact.phone IS
    'DU LIEU CA NHAN (luat 3). Chi chu so, 9-15 ky tu, tang ung dung chuan hoa. Khong vao log, che '
    'khi ra API (MaskPhone), khong vao URL / ten tep / khoa cache / ten phong realtime.';
COMMENT ON COLUMN citizen_contact.address IS
    'DU LIEU CA NHAN (luat 3). Khong vao log, khong vao URL / ten tep / khoa cache.';
COMMENT ON COLUMN citizen_contact.source IS
    'nhap-tay | nhap-excel (ADR 0011). Gia tri thu ba (nguoi dan tu dang ky) de dang sau, chua quyet.';
COMMENT ON COLUMN citizen_contact.household_code IS
    'DU TRU (chu du an 16/10/2026). Chua gi doc hay ghi.';

-- The directory list: this commune's live contacts, active ones first or only. Starts with tenant_id
-- (rule 1), drops removed rows (rule 7, invariant 2).
CREATE INDEX IF NOT EXISTS citizen_contact_by_active
    ON citizen_contact (tenant_id, is_active)
    WHERE deleted_at IS NULL;
-- Filter by residential unit (the prototype's thôn/tổ filter).
CREATE INDEX IF NOT EXISTS citizen_contact_by_residential_unit
    ON citizen_contact (tenant_id, residential_unit_id)
    WHERE deleted_at IS NULL;

-- ---------------------------------------------------------------------------
-- @entity: CitizenGroup
-- @scope:  tenant
CREATE TABLE IF NOT EXISTS citizen_group (
    tenant_id      TEXT        NOT NULL,
    id             TEXT        NOT NULL,   -- ULID

    name           TEXT        NOT NULL,
    name_folded    TEXT        GENERATED ALWAYS AS (
        lower(translate(translate(name, U&'\0300\0301\0302\0303\0306\0309\031B\0323', ''), 'àáảãạăằắẳẵặâầấẩẫậÀÁẢÃẠĂẰẮẲẴẶÂẦẤẨẪẬèéẻẽẹêềếểễệÈÉẺẼẸÊỀẾỂỄỆìíỉĩịÌÍỈĨỊòóỏõọôồốổỗộơờớởỡợÒÓỎÕỌÔỒỐỔỖỘƠỜỚỞỠỢùúủũụưừứửữựÙÚỦŨỤƯỪỨỬỮỰỳýỷỹỵỲÝỶỸỴđĐÐ', 'aaaaaaaaaaaaaaaaaAAAAAAAAAAAAAAAAAeeeeeeeeeeeEEEEEEEEEEEiiiiiIIIIIoooooooooooooooooOOOOOOOOOOOOOOOOOuuuuuuuuuuuUUUUUUUUUUUyyyyyYYYYYdDD'))
    ) STORED,
    description    TEXT,
    sort_order     INTEGER     NOT NULL DEFAULT 0,

    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at     TIMESTAMPTZ,
    deleted_by     TEXT,
    delete_reason  TEXT,

    -- TRUE while live, NULL once soft-deleted: a removed group's name may be used again (header).
    live_key       BOOLEAN GENERATED ALWAYS AS (CASE WHEN deleted_at IS NULL THEN TRUE END) STORED,

    -- Composite with tenant_id (rule 1, invariant 6).
    PRIMARY KEY (tenant_id, id),
    CONSTRAINT citizen_group_name_live_unique
        UNIQUE (tenant_id, name_folded, live_key),

    CONSTRAINT citizen_group_id_ulid CHECK (length(id) = 26),
    CONSTRAINT citizen_group_name_not_blank CHECK (btrim(name) <> ''),

    -- Soft delete: `deleted_at` and `deleted_by` together or neither; the reason is OPTIONAL for a
    -- group (owner, 16/10/2026) but never blank when given.
    CONSTRAINT citizen_group_soft_delete_complete
        CHECK ((deleted_at IS NULL AND deleted_by IS NULL AND delete_reason IS NULL)
            OR (deleted_at IS NOT NULL AND deleted_by IS NOT NULL AND btrim(deleted_by) <> ''
                AND (delete_reason IS NULL
                     OR (btrim(delete_reason) <> '' AND char_length(delete_reason) <= 500))))
);

COMMENT ON TABLE citizen_group IS
    'Nhom nguoi dan do xa tu dat ten (chu du an chot 16/10/2026) de gui thong bao / loc danh ba. '
    'Bang thuong (ADR 0096 #3). Xoa mem; ten nhom da xoa duoc dung lai.';

-- The group list of one commune, in the order the commune set.
CREATE INDEX IF NOT EXISTS citizen_group_list
    ON citizen_group (tenant_id, sort_order, name_folded)
    WHERE deleted_at IS NULL;

-- ---------------------------------------------------------------------------
-- @entity: CitizenContactGroup
-- @scope:  tenant
CREATE TABLE IF NOT EXISTS citizen_contact_group (
    tenant_id      TEXT        NOT NULL,
    contact_id     TEXT        NOT NULL,   -- citizen_contact.id
    group_id       TEXT        NOT NULL,   -- citizen_group.id

    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- The STAFF CODE of whoever added the membership (rule 6, invariant 8), or 'system' (import).
    created_by     TEXT        NOT NULL,

    deleted_at     TIMESTAMPTZ,
    deleted_by     TEXT,
    delete_reason  TEXT,

    -- ONE LIVE MEMBERSHIP PER (commune, person, group) — the 0031 shape (header: MEMBERSHIP ROWS).
    live_key       BOOLEAN GENERATED ALWAYS AS (CASE WHEN deleted_at IS NULL THEN TRUE END) STORED,

    -- created_at in the key: removed and added back later is a SECOND row, never an edit of the
    -- first (rule 7, forbidden #5). Composite with tenant_id (rule 1, invariant 6).
    PRIMARY KEY (tenant_id, contact_id, group_id, created_at),
    CONSTRAINT citizen_contact_group_live_unique
        UNIQUE (tenant_id, contact_id, group_id, live_key),

    -- Composite with the commune, so another commune's contact or group is refused by the key itself.
    CONSTRAINT citizen_contact_group_contact_fkey
        FOREIGN KEY (tenant_id, contact_id) REFERENCES citizen_contact (tenant_id, id),
    CONSTRAINT citizen_contact_group_group_fkey
        FOREIGN KEY (tenant_id, group_id) REFERENCES citizen_group (tenant_id, id),

    CONSTRAINT citizen_contact_group_created_by_not_blank CHECK (btrim(created_by) <> ''),

    -- Soft-delete columns are all three or none (rule 7, invariant 1).
    CONSTRAINT citizen_contact_group_soft_delete_complete
        CHECK ((deleted_at IS NULL AND deleted_by IS NULL AND delete_reason IS NULL)
            OR (deleted_at IS NOT NULL AND deleted_by IS NOT NULL AND btrim(deleted_by) <> ''
                AND delete_reason IS NOT NULL AND btrim(delete_reason) <> ''))
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..7 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS citizen_contact_group_p%s PARTITION OF citizen_contact_group '
            'FOR VALUES WITH (MODULUS 8, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

COMMENT ON TABLE citizen_contact_group IS
    'Thanh vien nhom: nguoi dan nao thuoc nhom nao. Go khoi nhom = xoa mem (theo mau 0031), them '
    'lai = dong moi. Phan manh theo tenant_id vi tang theo so nguoi dan (ADR 0096 #1).';

-- "Who is in this group" — the notice-sending read. Starts with tenant_id, drops removed rows.
CREATE INDEX IF NOT EXISTS citizen_contact_group_by_group
    ON citizen_contact_group (tenant_id, group_id)
    WHERE deleted_at IS NULL;

-- ---------------------------------------------------------------------------
-- REVERSAL (rule 7, invariant 4). By a person, in ONE transaction; core/migrate has no automatic
-- rollback (ADR 0013). Written as prose, not a runnable line, because a runnable line gets run.
--
--   WHILE NO COMMUNE HAS A CONTACT — check first that `SELECT count(*)` on citizen_contact,
--   citizen_group and citizen_contact_group all return 0; then drop citizen_contact_group,
--   citizen_contact, citizen_group in that order (membership first, it references the other two;
--   partitions and indexes go with their parent) and remove this file's row from schema_migration
--   (ten = '0033_citizen_directory.sql'). Exact; loses nothing.
--
--   ONCE A CONTACT EXISTS — no reversal by command. The rows are personal data with a business
--   history (which notices reached whom). Rule 7 stop condition #1: the owner's decision with a
--   verified backup, never a line in a file.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- BACKSTOP, carried forward from 0002–0032: every partitioned table in this schema must have
-- partitions, or it rejects every INSERT.
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
                         'MODULUS 8 partition loop (ADR 0096) in the same migration that '
                         'declares PARTITION BY.';
    END IF;
END $$;
