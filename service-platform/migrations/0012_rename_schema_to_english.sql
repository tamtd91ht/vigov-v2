-- platform — layer B of the English rename campaign (ADR 0061): every Vietnamese table, column,
-- constraint, index, trigger and function name of this schema gets its dictionary name
-- (kb/00-foundation/ubiquitous-language.md §Từ điển đổi tên). RENAME ONLY: no row is copied, moved
-- or rewritten, and no table is dropped or recreated.
--
-- WHY A NEW FILE AND NOT AN EDIT OF 0001–0011: those have been applied, core/migrate compares the
-- checksum of every applied file at startup, and the file NAME is the key of its progress row
-- (schema_migration.ten). Their comments keep the old names; the dictionary is where a reader
-- translates them.
--
-- THE FIVE MIGRATION QUESTIONS:
--
--   1. HOW MANY ROWS PER COMMUNE: none are touched. A rename is a catalogue change. The one physical
--      write is the table rewrite caused by the compatibility columns below (§ROLLING UPDATE) on
--      tenant, tenant_domain and mini_app — registry tables of a handful of rows per commune.
--   2. IF IT STOPS HALF-WAY: it cannot. core/migrate applies this file in ONE transaction together
--      with its progress row, and PostgreSQL DDL is transactional. Nothing here commits early and
--      nothing here needs to run outside a transaction (no CREATE INDEX CONCURRENTLY).
--      lock_timeout makes a lock that cannot be had fail the run instead of queueing behind a long
--      reader while every later reader of the registry queues behind it; a failed run leaves the
--      schema exactly as it was and the pod retries on restart.
--   3. HOW IT IS REVERSED: migrations/reverse/0012_rename_schema_to_english.sql — a runnable script,
--      kept OUT of this directory because go:embed *.sql would ship it and core/migrate would apply
--      it. It drops the compatibility aliases, renames every object back, restores the two function
--      bodies and three comments verbatim, and removes this file's progress row, in one
--      transaction. Before running it on a database with real data: a verified, restore-tested
--      backup (ADR 0061 §Lớp B — both conditions, not one of them).
--   4. WHICH READ PATHS CHANGE MEANING WHILE IT IS HALF-APPLIED: there is no half-applied state
--      (question 2). The window that DOES exist is the rolling update: pods of the previous image
--      keep querying the old names after this commits. See §ROLLING UPDATE.
--   5. RETENTION: nothing is removed. Rows, their values and the append-only trail are untouched.
--      Values stored in columns stay as they are ('chinh'/'rieng', 'petition-photo', the field
--      codes): that is layer C. The seeded platform_audit_log rows keep action
--      'petition_field.seeded' forever — they are append-only history (ADR 0061 §Dòng chỉ-thêm).
--
-- ROLLING UPDATE — WHY THE LAST SECTION ADDS OLD NAMES BACK.
--
-- deploy/base/platform/deployment.yaml runs 3 replicas with RollingUpdate maxSurge 1,
-- maxUnavailable 0 (staging: 1 replica, same strategy). The first pod of this image applies this
-- file at startup while the previous image's pods keep serving. Those pods run SQL with the old
-- names, and platform is the Host → commune resolver of the whole system: a failed lookup there is
-- a 404 for every commune whose request lands on an old pod, for as long as the rollout lasts.
--
-- So this file ends by recreating EXACTLY the old names the previous image reads at runtime, as
-- read-only aliases of the renamed objects:
--   * a column kept in its table but renamed → a STORED generated column under the old name;
--   * a renamed table → a plain view under the old name, old column names aliased.
-- Nothing else gets an alias: the previous image reads no other renamed name (tenant_succession and
-- province are read by nothing at runtime, and the timestamps/signers are never selected).
-- store/rename_schema_pg_test.go runs the previous image's statements verbatim against this schema.
--
-- THE ALIASES ARE TEMPORARY AND MUST BE DROPPED BY A LATER MIGRATION, SHIPPED IN A LATER RELEASE —
-- after every environment has finished rolling out this image. Shipping that migration in THIS
-- release would drop the aliases while the old pods still read them, which is the outage this
-- section exists to prevent. It is not written yet on purpose; its content is two DROP VIEW and six
-- DROP COLUMN of generated columns, which destroys nothing (each is a copy of another column), and
-- it needs the owner's go-ahead like every column drop (rule 7).
--
-- What the aliases do NOT cover: a pod of the previous image that RESTARTS after this file
-- committed refuses to start (core/migrate ErrThieuTep: the database has a migration its binary
-- lacks). That is fail-closed and the rollout replaces it anyway. The same is why an image rollback
-- needs the reverse script first.
--
-- FIVE TECHNICAL NAMES THE DICTIONARY DOES NOT LIST, named by the coordinator on 2026-09-29 as
-- technical names rather than business concepts (no dictionary row needed):
--   tenant_domain_host_thuong            → tenant_domain_host_lowercase
--   tenant_domain_mot_chinh              → tenant_domain_one_primary
--   tenant_succession_khong_tro_chinh_no → tenant_succession_not_self
--   tenant_succession_chan_vong_lap      → tenant_succession_no_cycle (trigger AND function)
--   mini_app_xa_khi_va_chi_khi_rieng     → mini_app_tenant_iff_dedicated
--
-- Every other name comes from the dictionary, from its composition rules (`la_*` → `is_*`,
-- `co_*` → `has_*`), from PostgreSQL's own default spelling over dictionary names (`_pkey`,
-- `<column>_fkey`), or from an English name this schema already uses for the same construct:
-- `_not_blank`, `_signed`, `_soft_delete_complete`, `_known` (0008, 0011), `_no_hard_delete`
-- (0008), `_by_<column>`, `_list`, `_unique` (comms 0007, identity 0012). `tenant_domain_not_reserved`
-- is domain.IsReservedHost's word.

SET LOCAL lock_timeout = '5s';

-- One statement, one order, so two sessions cannot each hold half of these and wait on the other.
LOCK TABLE tenant, tenant_domain, tenant_succession, tinh_thanh, mini_app, ho_so_hien_thi_xa,
    petition_field IN ACCESS EXCLUSIVE MODE;

-- ---------------------------------------------------------------------------
-- tenant — columns only; the table name is already English.
-- ---------------------------------------------------------------------------
ALTER TABLE tenant RENAME COLUMN ten TO name;
-- `province_name`, NOT `province_code`: the column holds the province's display NAME typed per
-- commune (0001, 0004), not a key. A `_code` suffix would invite someone to join on it. Whether a
-- real key into `province` is added is 0004's follow-up.
ALTER TABLE tenant RENAME COLUMN tinh_thanh TO province_name;
ALTER TABLE tenant RENAME COLUMN dang_hoat_dong TO is_active;
ALTER TABLE tenant RENAME COLUMN tao_luc TO created_at;
ALTER TABLE tenant RENAME COLUMN cap_nhat_luc TO updated_at;
ALTER TABLE tenant RENAME CONSTRAINT tenant_id_la_ulid TO tenant_id_is_ulid;

-- ---------------------------------------------------------------------------
-- tenant_domain — columns, the reserved-host CHECK, one index.
--
-- The CHECK keeps its body and its NOT VALID state: a rename re-validates nothing, so the two kept
-- admin rows of 0007 stay exactly as they are. Its body is still the one 0007 wrote, which is the
-- file domain.TestReservedHostRuleGoAndSQLAgree parses.
-- ---------------------------------------------------------------------------
ALTER TABLE tenant_domain RENAME COLUMN la_chinh TO is_primary;
ALTER TABLE tenant_domain RENAME COLUMN tao_luc TO created_at;
ALTER TABLE tenant_domain RENAME CONSTRAINT tenant_domain_khong_danh_rieng TO tenant_domain_not_reserved;
ALTER TABLE tenant_domain RENAME CONSTRAINT tenant_domain_host_thuong TO tenant_domain_host_lowercase;
ALTER INDEX tenant_domain_theo_xa RENAME TO tenant_domain_by_tenant;
ALTER INDEX tenant_domain_mot_chinh RENAME TO tenant_domain_one_primary;

-- ---------------------------------------------------------------------------
-- tenant_succession — columns, constraints, one index, and the cycle guard's BODY.
-- ---------------------------------------------------------------------------
ALTER TABLE tenant_succession RENAME COLUMN tu_id TO from_tenant_id;
ALTER TABLE tenant_succession RENAME COLUMN den_id TO to_tenant_id;
ALTER TABLE tenant_succession RENAME COLUMN can_cu TO legal_basis;
ALTER TABLE tenant_succession RENAME COLUMN hieu_luc_tu TO effective_from;
ALTER TABLE tenant_succession RENAME COLUMN ghi_chu TO note;
ALTER TABLE tenant_succession RENAME COLUMN tao_luc TO created_at;
ALTER TABLE tenant_succession RENAME COLUMN tao_boi TO created_by;
ALTER TABLE tenant_succession RENAME CONSTRAINT tenant_succession_tu_id_fkey TO tenant_succession_from_tenant_id_fkey;
ALTER TABLE tenant_succession RENAME CONSTRAINT tenant_succession_den_id_fkey TO tenant_succession_to_tenant_id_fkey;
ALTER TABLE tenant_succession RENAME CONSTRAINT tenant_succession_co_can_cu TO tenant_succession_has_legal_basis;
ALTER TABLE tenant_succession RENAME CONSTRAINT tenant_succession_khong_tro_chinh_no TO tenant_succession_not_self;
ALTER INDEX tenant_succession_theo_den RENAME TO tenant_succession_by_to_tenant;

-- THE BODY MUST BE REWRITTEN (ADR 0061 §Cạm bẫy). plpgsql resolves NEW.<column> and the columns of
-- its query when the function first runs, not when the column is renamed: left as 0003 wrote it,
-- the first INSERT after this file fails with "record new has no field tu_id" — at an operator
-- recording a real merger, never in a test that only applies migrations. Same logic, new names;
-- new body first, then the name (ADR 0061 §Lớp B).
CREATE OR REPLACE FUNCTION tenant_succession_chan_vong_lap() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF EXISTS (
        WITH RECURSIVE di(id) AS (
            SELECT NEW.to_tenant_id
            UNION
            SELECT s.to_tenant_id FROM tenant_succession s JOIN di ON s.from_tenant_id = di.id
        )
        SELECT 1 FROM di WHERE id = NEW.from_tenant_id
    ) THEN
        RAISE EXCEPTION
            'tenant_succession: canh % -> % tao thanh vong lap ke thua', NEW.from_tenant_id, NEW.to_tenant_id
            USING HINT = 'Don vi ke thua da dan nguoc ve don vi cu. Kiem tra lai ULID da nhap.';
    END IF;
    RETURN NEW;
END $$;
ALTER FUNCTION tenant_succession_chan_vong_lap() RENAME TO tenant_succession_no_cycle;
ALTER TRIGGER tenant_succession_chan_vong_lap ON tenant_succession RENAME TO tenant_succession_no_cycle;

-- ---------------------------------------------------------------------------
-- province (was tinh_thanh, 0004).
-- ---------------------------------------------------------------------------
-- @entity: Province
-- @scope:  platform
ALTER TABLE tinh_thanh RENAME TO province;
ALTER TABLE province RENAME COLUMN ten TO name;
ALTER TABLE province RENAME COLUMN thu_tu TO sort_order;
ALTER TABLE province RENAME COLUMN dang_hoat_dong TO is_active;
ALTER TABLE province RENAME COLUMN tao_luc TO created_at;
ALTER TABLE province RENAME COLUMN cap_nhat_luc TO updated_at;
ALTER TABLE province RENAME CONSTRAINT tinh_thanh_pkey TO province_pkey;
ALTER TABLE province RENAME CONSTRAINT tinh_thanh_id_la_ulid TO province_id_is_ulid;
ALTER TABLE province RENAME CONSTRAINT tinh_thanh_ten_duy_nhat TO province_name_unique;
ALTER TABLE province RENAME CONSTRAINT tinh_thanh_ten_khong_rong TO province_name_not_blank;
ALTER INDEX tinh_thanh_danh_sach RENAME TO province_list;

-- The comment named the old column of `tenant`; only that name changes.
COMMENT ON TABLE province IS
    'Danh muc don vi hanh chinh cap tinh. Nha cung cap seed, xa chi duoc CHON. Ly do: '
    'tenant.province_name la chuoi tu nhap, hai cach viet mot tinh thi cong dan thay hai tinh.';

-- ---------------------------------------------------------------------------
-- mini_app — columns, constraints, the hard-delete trigger.
-- ---------------------------------------------------------------------------
ALTER TABLE mini_app RENAME COLUMN che_do TO mode;
ALTER TABLE mini_app RENAME COLUMN dang_hoat_dong TO is_active;
ALTER TABLE mini_app RENAME COLUMN ghi_chu TO note;
ALTER TABLE mini_app RENAME COLUMN tao_luc TO created_at;
ALTER TABLE mini_app RENAME COLUMN tao_boi TO created_by;
ALTER TABLE mini_app RENAME COLUMN cap_nhat_luc TO updated_at;
ALTER TABLE mini_app RENAME COLUMN cap_nhat_boi TO updated_by;
ALTER TABLE mini_app RENAME CONSTRAINT mini_app_app_id_khong_rong TO mini_app_app_id_not_blank;
ALTER TABLE mini_app RENAME CONSTRAINT mini_app_che_do_hop_le TO mini_app_mode_known;
ALTER TABLE mini_app RENAME CONSTRAINT mini_app_xa_khi_va_chi_khi_rieng TO mini_app_tenant_iff_dedicated;
ALTER TABLE mini_app RENAME CONSTRAINT mini_app_nguoi_ghi_khong_rong TO mini_app_signed;
ALTER TABLE mini_app RENAME CONSTRAINT mini_app_xoa_mem_day_du TO mini_app_soft_delete_complete;
ALTER TRIGGER mini_app_cam_xoa_cung ON mini_app RENAME TO mini_app_no_hard_delete;

-- The column comment named `che_do`; the table comment's "che do" is prose and stays.
COMMENT ON COLUMN mini_app.tenant_id IS
    'Xa gan app rieng. NULL khi va chi khi mode = chinh. Xa ngung hoat dong thi app bi tu choi, '
    'KHONG tu di theo tenant_succession (ADR 0045); gan lai la viec cua nguoi van hanh.';

-- ---------------------------------------------------------------------------
-- commune_profile (was ho_so_hien_thi_xa, 0006). X5: the display profile belongs to the
-- ADMINISTRATIVE UNIT, hence `commune`, not `tenant`. Its comments name no renamed object and
-- follow the table by OID unchanged.
-- ---------------------------------------------------------------------------
-- @entity: CommuneProfile
-- @scope:  tenant
ALTER TABLE ho_so_hien_thi_xa RENAME TO commune_profile;
ALTER TABLE commune_profile RENAME COLUMN dia_chi_tru_so TO office_address;
ALTER TABLE commune_profile RENAME COLUMN duong_day_nong TO hotline;
ALTER TABLE commune_profile RENAME COLUMN gio_lam_viec_hien_thi TO office_hours_text;
ALTER TABLE commune_profile RENAME COLUMN gioi_thieu TO introduction;
ALTER TABLE commune_profile RENAME COLUMN tao_luc TO created_at;
ALTER TABLE commune_profile RENAME COLUMN tao_boi TO created_by;
ALTER TABLE commune_profile RENAME COLUMN cap_nhat_luc TO updated_at;
ALTER TABLE commune_profile RENAME COLUMN cap_nhat_boi TO updated_by;
ALTER TABLE commune_profile RENAME CONSTRAINT ho_so_hien_thi_xa_pkey TO commune_profile_pkey;
ALTER TABLE commune_profile RENAME CONSTRAINT ho_so_hien_thi_xa_tenant_id_fkey TO commune_profile_tenant_id_fkey;
ALTER TABLE commune_profile RENAME CONSTRAINT ho_so_hien_thi_xa_logo_khong_rong TO commune_profile_logo_not_blank;
ALTER TABLE commune_profile RENAME CONSTRAINT ho_so_hien_thi_xa_nguoi_ghi_khong_rong TO commune_profile_signed;
ALTER TABLE commune_profile RENAME CONSTRAINT ho_so_hien_thi_xa_xoa_mem_day_du TO commune_profile_soft_delete_complete;
ALTER TRIGGER ho_so_hien_thi_xa_cam_xoa_cung ON commune_profile RENAME TO commune_profile_no_hard_delete;

-- The hard-delete refusal shared by mini_app, commune_profile and upload_policy. Its body names no
-- table or column (TG_TABLE_NAME, and the three soft-delete columns, which keep their names), so
-- only the name changes; the three triggers follow it by OID.
ALTER FUNCTION platform_cam_xoa_cung() RENAME TO platform_no_hard_delete;

-- ---------------------------------------------------------------------------
-- citizen_report_field (was petition_field, 0011). X1 retires `Petition` as a code name; X13 puts
-- `is_` on the flag.
-- ---------------------------------------------------------------------------
-- @entity: CitizenReportField
-- @scope:  platform
ALTER TABLE petition_field RENAME TO citizen_report_field;
ALTER TABLE citizen_report_field RENAME COLUMN active TO is_active;
ALTER TABLE citizen_report_field RENAME CONSTRAINT petition_field_pkey TO citizen_report_field_pkey;
ALTER TABLE citizen_report_field RENAME CONSTRAINT petition_field_code_shape TO citizen_report_field_code_shape;
ALTER TABLE citizen_report_field RENAME CONSTRAINT petition_field_label_not_blank TO citizen_report_field_label_not_blank;
ALTER TABLE citizen_report_field RENAME CONSTRAINT petition_field_sort_order_positive TO citizen_report_field_sort_order_positive;
ALTER TABLE citizen_report_field RENAME CONSTRAINT petition_field_tone_known TO citizen_report_field_tone_known;
ALTER TABLE citizen_report_field RENAME CONSTRAINT petition_field_signed TO citizen_report_field_signed;

-- New body first, then the name (ADR 0061 §Lớp B). The body reads only OLD.code / NEW.code, which
-- keep their name, so the guard never stopped working — but its messages named the old table and
-- told the operator to set a column that no longer exists (`active = false`). A hint pointing at a
-- missing column is an operator typing a failing UPDATE while an intake form still offers the code.
CREATE OR REPLACE FUNCTION petition_field_guard() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'citizen_report_field: delete of code % refused', OLD.code
            USING HINT = 'A tier-1 field code is held by archival petitions and SLA rows as a value '
                         '(ADR 0026, ADR 0060). Retire it with is_active = false; it is never removed.';
    END IF;
    IF NEW.code IS DISTINCT FROM OLD.code THEN
        RAISE EXCEPTION 'citizen_report_field: renaming code % refused', OLD.code
            USING HINT = 'Renaming a code orphans every petition and SLA row that holds it. Edit '
                         'default_label instead; a new code is a new row.';
    END IF;
    RETURN NEW;
END $$;
ALTER FUNCTION petition_field_guard() RENAME TO citizen_report_field_guard;
ALTER TRIGGER petition_field_guard ON citizen_report_field RENAME TO citizen_report_field_guard;

COMMENT ON TABLE citizen_report_field IS
    'Bo ma linh vuc phan anh TANG 1 (ADR 0026, ADR 0060): mot bo cho moi xa. Ma khong bao gio doi, '
    'khong bao gio xoa - phieu luu tru giu ma duoi dang gia tri. Ngung dung = is_active false.';

-- ---------------------------------------------------------------------------
-- NOT NULL constraints — PostgreSQL 18 names them `<table>_<column>_not_null` and a rename does not
-- follow. On 13–17 they are not catalogued (contype 'n' does not exist) and this loop finds nothing.
-- Rule-based rather than listed, so the reverse script can run the same rule over the old names.
-- ---------------------------------------------------------------------------
DO $$
DECLARE r record;
BEGIN
    FOR r IN
        SELECT cl.relname, k.conname, a.attname
        FROM pg_constraint k
        JOIN pg_class cl ON cl.oid = k.conrelid
        JOIN pg_namespace n ON n.oid = cl.relnamespace
        JOIN pg_attribute a ON a.attrelid = k.conrelid AND a.attnum = k.conkey[1]
        WHERE k.contype = 'n'
          AND n.nspname = current_schema()
          AND cl.relname IN ('tenant', 'tenant_domain', 'tenant_succession', 'province', 'mini_app',
                             'commune_profile', 'citizen_report_field')
          AND k.conname LIKE '%\_not\_null'
          AND k.conname <> cl.relname || '_' || a.attname || '_not_null'
    LOOP
        EXECUTE format('ALTER TABLE %I RENAME CONSTRAINT %I TO %I',
                       r.relname, r.conname, r.relname || '_' || r.attname || '_not_null');
    END LOOP;
END $$;

-- ---------------------------------------------------------------------------
-- ROLLING-UPDATE ALIASES — old names for the previous image's pods (header, §ROLLING UPDATE).
--
-- Exactly the names that image's store/*.go reads at runtime, nothing more. Generated columns are
-- read-only, so an old pod cannot write through them; the views are simple views over one table,
-- so the base table's constraints and triggers still judge anything written through them.
-- ---------------------------------------------------------------------------
ALTER TABLE tenant
    ADD COLUMN ten TEXT GENERATED ALWAYS AS (name) STORED,
    ADD COLUMN tinh_thanh TEXT GENERATED ALWAYS AS (province_name) STORED,       -- vi-name-ok: rolling-update alias for the previous image, dropped by a later release
    ADD COLUMN dang_hoat_dong BOOLEAN GENERATED ALWAYS AS (is_active) STORED;   -- vi-name-ok: rolling-update alias for the previous image, dropped by a later release

ALTER TABLE tenant_domain
    ADD COLUMN la_chinh BOOLEAN GENERATED ALWAYS AS (is_primary) STORED;        -- vi-name-ok: rolling-update alias for the previous image, dropped by a later release

ALTER TABLE mini_app
    ADD COLUMN che_do TEXT GENERATED ALWAYS AS (mode) STORED,                   -- vi-name-ok: rolling-update alias for the previous image, dropped by a later release
    ADD COLUMN dang_hoat_dong BOOLEAN GENERATED ALWAYS AS (is_active) STORED;   -- vi-name-ok: rolling-update alias for the previous image, dropped by a later release

-- vi-name-ok: rolling-update alias for the previous image, dropped by a later release
CREATE VIEW ho_so_hien_thi_xa AS
    SELECT tenant_id,
           office_address    AS dia_chi_tru_so,
           logo_url,
           hotline           AS duong_day_nong,
           office_hours_text AS gio_lam_viec_hien_thi,
           introduction      AS gioi_thieu,
           created_at        AS tao_luc,
           created_by        AS tao_boi,
           updated_at        AS cap_nhat_luc,
           updated_by        AS cap_nhat_boi,
           deleted_at, deleted_by, delete_reason
    FROM commune_profile;

CREATE VIEW petition_field AS
    SELECT code, default_label, sort_order, icon, tone, is_active AS active,
           created_at, created_by, updated_at, updated_by
    FROM citizen_report_field;

COMMENT ON VIEW ho_so_hien_thi_xa IS
    'TAM THOI (0012): ten cu cua commune_profile cho pod ban truoc trong luc rolling update. '
    'Xoa bang migration cua ban phat hanh sau.';
COMMENT ON VIEW petition_field IS
    'TAM THOI (0012): ten cu cua citizen_report_field cho pod ban truoc trong luc rolling update. '
    'Xoa bang migration cua ban phat hanh sau.';

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
