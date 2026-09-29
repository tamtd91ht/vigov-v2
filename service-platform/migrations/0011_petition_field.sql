-- platform — petition_field: TIER 1 of the petition field catalogue (ADR 0026), the closed code set
-- every commune shares, read by business services over PlatformService.ListPetitionFields (ADR 0060).
--
-- WHY A NEW FILE AND NOT AN EDIT OF 0001–0010: those have been applied and core/migrate compares the
-- checksum of every applied file at startup. An applied migration is immutable.
--
-- WHY IN service-platform: ADR 0026 §Bổ sung 2026-09-20 — the customer placed the code set on the
-- vendor's admin screen ("admin tổng"), i.e. this service. Writing this table in any business
-- service is that ADR's stop condition #1, and a replica of it is ADR 0060's stop condition #1.
--
-- THE FIVE MIGRATION QUESTIONS:
--
--   1. HOW MANY ROWS PER COMMUNE: none — no commune column. Twelve rows platform-wide today, one per
--      tier-1 code, and twelve platform_audit_log entries.
--   2. IF IT STOPS HALF-WAY: it cannot. One file, one transaction (core/migrate), progress row inside
--      it. Every CREATE is IF NOT EXISTS / CREATE OR REPLACE / DROP TRIGGER IF EXISTS, and the seed is
--      ON CONFLICT DO NOTHING with the trail built from RETURNING, so a retry writes nothing twice.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. WHICH READ PATHS CHANGE MEANING WHILE IT IS HALF-APPLIED: none. One NEW table, one NEW trigger
--      function. Until this file is applied ListPetitionFields fails (no table), which every caller
--      treats as "platform unavailable" → refuse the intake: the safe direction (ADR 0060 §3).
--   5. RETENTION: nothing is ever removed. A row can be neither deleted nor have its code changed —
--      both refused by a trigger below. Retiring a code is `active = false`.

-- ---------------------------------------------------------------------------
-- petition_field — one tier-1 field code: its default label, default order, default presentation,
-- and whether new petitions may still be filed under it.
--
-- NO tenant_id, AND THAT IS ADR 0026's DECISION, NOT AN OMISSION: the code set is ONE set shared by
-- every commune, because a petition count per field must add up across 200+ communes. A commune's
-- own wording, order and on/off switch are TIER 2 — `nhan_linh_vuc` in service-petitions
-- (service-petitions/migrations/0004_phieu_phan_anh.sql) — never a tenant_id retro-fitted here.
--
-- THE KEY IS THE CODE, AND THE CODE IS FOREVER. Archival petitions (`phieu_phan_anh.linh_vuc`) and
-- a commune's SLA rows (identity `sla.linh_vuc`) hold it AS A VALUE, with no foreign key possible
-- across the service boundary (rule 2, forbidden #2). So:
--   * a code is NEVER RENAMED — a rename would orphan every petition and SLA row carrying the old
--     spelling, which is exactly the `ve-sinh-moi-truong` defect ADR 0026 §2 cites as evidence;
--   * a code is NEVER DELETED, NOT EVEN SOFTLY — and this is why the table has no deleted_at, unlike
--     upload_policy. Rule 7 invariant 2 makes every read path exclude soft-deleted rows, so a
--     soft-deleted code would take the label away from every old petition that carries it, while
--     those petitions' clocks keep running. Retirement is `active = false`, which hides the code
--     from NEW intake and reclassification only; no read path ever filters on it (ADR 0060 §4).
-- Both are enforced by petition_field_guard below, not by convention.
--
-- CODE SPELLING is ADR 0011's enum-value shape: Vietnamese without diacritics, kebab-case. The
-- length bound 64 is service-petitions' domain.LinhVucToiDa, so a code this table accepts is a code
-- that service can store.
--
-- icon IS A lucide ICON NAME and tone ONE OF THE CITIZEN APP's SIX TONES, both copied from the
-- reference system's deployed catalogue (ADR 0060 §5). '' means "not declared" — the client renders
-- a neutral icon/tone, never guesses one. tone is CHECKed because its value set is closed by the
-- client's design tokens; icon is not, because the icon library is the client's and grows with it.
--
-- NO placeholder, NO hex colour, NO "restricted" flag — ADR 0060 §4 says why for each.
--
-- Read through store.PetitionFieldStore (petition_field.go), a raw-handle reader: there is no
-- commune column to scope by, so core/store.Scoped cannot read it.
-- ---------------------------------------------------------------------------
-- @entity: PetitionField
-- @scope:  platform
CREATE TABLE IF NOT EXISTS petition_field (
    code          TEXT        PRIMARY KEY,
    default_label TEXT        NOT NULL,
    sort_order    INT         NOT NULL,
    icon          TEXT        NOT NULL DEFAULT '',
    tone          TEXT        NOT NULL DEFAULT '',

    -- NO DEFAULT: every write states whether the code takes new petitions. A retired code quietly
    -- revived by a column default is a field reappearing on 200 communes' forms that nobody chose.
    active        BOOLEAN     NOT NULL,

    -- WHO entered / last changed the row: a business code, never an internal id (rule 6, invariant
    -- 8). 'system' for rows this migration seeds.
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by    TEXT        NOT NULL,
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by    TEXT        NOT NULL,

    CONSTRAINT petition_field_code_shape
        CHECK (code ~ '^[a-z0-9]+(-[a-z0-9]+)*$' AND length(code) <= 64),
    CONSTRAINT petition_field_label_not_blank CHECK (btrim(default_label) <> ''),
    CONSTRAINT petition_field_sort_order_positive CHECK (sort_order > 0),
    CONSTRAINT petition_field_tone_known
        CHECK (tone IN ('', 'blue', 'green', 'orange', 'purple', 'cyan', 'red')),
    CONSTRAINT petition_field_signed CHECK (btrim(created_by) <> '' AND btrim(updated_by) <> '')
);

COMMENT ON TABLE petition_field IS
    'Bo ma linh vuc phan anh TANG 1 (ADR 0026, ADR 0060): mot bo cho moi xa. Ma khong bao gio doi, '
    'khong bao gio xoa - phieu luu tru giu ma duoi dang gia tri. Ngung dung = active false.';
COMMENT ON COLUMN petition_field.active IS
    'false = khong nhan phieu moi, khong phan loai vao ma nay. Duong DOC khong bao gio loc theo cot nay.';

-- The one guard this table needs, in ONE function: refuse DELETE (hard — there is no soft delete to
-- point to) and refuse changing `code`. Separate from 0006's platform_cam_xoa_cung on purpose: that
-- function's hint tells the operator to soft delete, which here is the wrong advice.
--
-- The message names the operation and the table; a code is platform configuration, not personal
-- data, so naming it is harmless and saves the operator a lookup.
CREATE OR REPLACE FUNCTION petition_field_guard() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'petition_field: delete of code % refused', OLD.code
            USING HINT = 'A tier-1 field code is held by archival petitions and SLA rows as a value '
                         '(ADR 0026, ADR 0060). Retire it with active = false; it is never removed.';
    END IF;
    IF NEW.code IS DISTINCT FROM OLD.code THEN
        RAISE EXCEPTION 'petition_field: renaming code % refused', OLD.code
            USING HINT = 'Renaming a code orphans every petition and SLA row that holds it. Edit '
                         'default_label instead; a new code is a new row.';
    END IF;
    RETURN NEW;
END $$;

DROP TRIGGER IF EXISTS petition_field_guard ON petition_field;
CREATE TRIGGER petition_field_guard
    BEFORE UPDATE OR DELETE ON petition_field
    FOR EACH ROW EXECUTE FUNCTION petition_field_guard();

-- ---------------------------------------------------------------------------
-- SEED — the twelve codes of docs/ui-ux/09-phan-anh-nguoi-dan.md §5, in that order. The same twelve
-- codes key service-identity's SLA seed (internal/domain/sla_gieo.go) — a code spelled differently
-- here would be a field whose deadline silently falls to the default row (identity.proto,
-- ResolveDeadlines). internal/store/petition_field_test.go pins the set against that list, so the
-- two cannot drift without a red test. Icons and tones: the reference system's deployed catalogue
-- (ADR 0060 §5). One trail entry per seeded row, in the same statement (rule 6, invariant 3).
-- ---------------------------------------------------------------------------
WITH seeded AS (
    INSERT INTO petition_field
        (code, default_label, sort_order, icon, tone, active, created_by, updated_by)
    VALUES
        ('rac-thai',          'Rác thải – Vệ sinh môi trường',            1, 'Trash2',        'orange', true, 'system', 'system'),
        ('giao-thong',        'Hạ tầng giao thông',                       2, 'TrafficCone',   'blue',   true, 'system', 'system'),
        ('cap-thoat-nuoc',    'Cấp thoát nước',                           3, 'Droplets',      'cyan',   true, 'system', 'system'),
        ('dien',              'Điện',                                     4, 'Zap',           'orange', true, 'system', 'system'),
        ('trat-tu-do-thi',    'Trật tự đô thị – lấn chiếm vỉa hè',        5, 'Construction',  'purple', true, 'system', 'system'),
        ('an-ninh',           'An ninh trật tự',                          6, 'ShieldAlert',   'red',    true, 'system', 'system'),
        ('xay-dung',          'Xây dựng không phép',                      7, 'Hammer',        'cyan',   true, 'system', 'system'),
        ('o-nhiem',           'Ô nhiễm (tiếng ồn, khí thải, nước thải)',  8, 'Factory',       'green',  true, 'system', 'system'),
        ('y-te-giao-duc',     'Y tế – Giáo dục',                          9, 'Stethoscope',   'blue',   true, 'system', 'system'),
        ('can-bo',            'Thái độ / tác phong cán bộ',              10, 'UserRoundX',    'purple', true, 'system', 'system'),
        ('an-toan-thuc-pham', 'An toàn thực phẩm',                       11, 'Utensils',      'green',  true, 'system', 'system'),
        ('khac',              'Khác',                                    12, 'MessageSquare', 'blue',   true, 'system', 'system')
    ON CONFLICT (code) DO NOTHING
    RETURNING code, default_label, sort_order, icon, tone, active
)
INSERT INTO platform_audit_log (actor, action, subject, before, after, reason)
SELECT 'system', 'petition_field.seeded', s.code, NULL,
       jsonb_build_object(
           'default_label', s.default_label,
           'sort_order', s.sort_order,
           'icon', s.icon,
           'tone', s.tone,
           'active', s.active),
       'migration 0011_petition_field.sql: 12 ma cua docs/ui-ux/09 §5, nguoi dung chot phuong an A 29/09/2026 (ADR 0060)'
FROM seeded s;

-- ---------------------------------------------------------------------------
-- REVERSAL (rule 7, invariant 4).
--
-- POSSIBLE ONLY WHILE NO PETITION, SLA ROW OR TIER-2 LABEL HOLDS A CODE READ FROM HERE, and no
-- operator has edited a row. Down steps, in one transaction:
--   1. drop table petition_field (its trigger goes with it), then the function petition_field_guard;
--   2. remove this file's progress row from the schema_migration table, keyed on
--      ten = '0011_petition_field.sql'.
-- The twelve 'petition_field.seeded' entries in platform_audit_log STAY — that table is append-only
-- (0008), and an entry recording a seed that was later reversed is still true.
-- Written as prose rather than as runnable lines, because a runnable line is a line that gets run.
--
-- ONCE A BUSINESS SERVICE HAS VALIDATED A PETITION AGAINST THIS TABLE there is no reversal: the
-- table is then the authority those archival records were checked against. Rule 7 stop condition
-- #1 — the user decides, with a verified backup.
--
-- PER-COMMUNE AND RESUMABLE (rule 7, invariant 5): no commune column, no backfill, no existing row
-- rewritten — nothing to iterate and nothing to resume.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- BACKSTOP, carried forward from 0002: a table declared PARTITION BY with no partitions rejects
-- every INSERT. This file declares no partitioned table and is expected to find nothing; it runs
-- anyway, because the run after which it is missing is the one that needed it.
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
