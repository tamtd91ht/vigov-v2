-- REVERSE of service-platform/migrations/0012_rename_schema_to_english.sql (ADR 0061 §Lớp B;
-- rule 7, invariant 4).
--
-- WHY THIS FILE IS IN migrations/reverse/ AND NOT BESIDE 0012: the migrations package embeds
-- `*.sql` of its own directory only (migrations/embed.go), core/migrate reads the root of that FS
-- and skips directories (core/migrate/migrate.go docNguon), and the repository tools read
-- `migrations/*.sql` (tools/kb/ownership.go, tools/check_khoa_duy_nhat.py). Placed beside 0012, it
-- would ship inside the binary and be APPLIED at the next startup, renaming everything back.
--
-- WHAT IT DOES, in one transaction, and nothing else:
--   1. refuses unless 0012 is the NEWEST applied migration and its aliases are what 0012 made;
--   2. drops 0012's rolling-update aliases (two views, six GENERATED columns — each a computed copy
--      of another column, so dropping them loses nothing);
--   3. renames every object back, restores the two function bodies and the three comments 0012
--      rewrote, VERBATIM from 0003, 0004, 0006 and 0011;
--   4. removes 0012's progress row, so the runner applies 0012 again the next time the current image
--      starts. Forward → reverse → forward is store/rename_schema_pg_test.go.
-- No row of any business or registry table is read, written or removed.
--
-- BEFORE RUNNING IT on a database holding real data: a verified, restore-tested backup. Two
-- conditions, not one (ADR 0061 §Lớp B).
--
-- HOW TO RUN IT, and the order that matters. The previous image refuses to start while 0012's row
-- exists (core/migrate ErrThieuTep), and the current image re-applies 0012 on any restart once the
-- row is gone. So an image rollback is:
--   1. `kubectl rollout undo` for platform — the previous image's new pods fail to start, the
--      current pods keep serving (maxUnavailable 0);
--   2. this script:  psql -v ON_ERROR_STOP=1 -f <this file>   (it opens its own transaction);
--   3. the previous image's pods now start and take over.
-- Between steps 2 and 3 the remaining current-image pods query names that no longer exist: a
-- short window of failed Host lookups. There is no alias in this direction; if that window is not
-- acceptable, say so before a rollback is ever needed.
--
-- Every `-- vi-name-ok` below marks a name RESTORED to what 0001–0011 created, not a new one.

BEGIN;

SET LOCAL lock_timeout = '5s';

DO $$
DECLARE n int;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM schema_migration WHERE ten = '0012_rename_schema_to_english.sql') THEN
        RAISE EXCEPTION 'reverse 0012: 0012_rename_schema_to_english.sql is not applied here; nothing to reverse';
    END IF;
    IF EXISTS (SELECT 1 FROM schema_migration WHERE ten > '0012_rename_schema_to_english.sql') THEN
        RAISE EXCEPTION 'reverse 0012: a later migration is applied; reverse it first';
    END IF;
    -- The six columns about to be dropped must be the GENERATED aliases 0012 added. Anything else
    -- under one of these names would be real data, and this script never drops real data.
    SELECT count(*) INTO n
    FROM pg_attribute a
    WHERE NOT a.attisdropped AND a.attgenerated = 's'
      AND ((a.attrelid = 'tenant'::regclass AND a.attname IN ('ten', 'tinh_thanh', 'dang_hoat_dong'))
        OR (a.attrelid = 'tenant_domain'::regclass AND a.attname = 'la_chinh')
        OR (a.attrelid = 'mini_app'::regclass AND a.attname IN ('che_do', 'dang_hoat_dong')));
    IF n <> 6 THEN
        RAISE EXCEPTION 'reverse 0012: found % of the 6 generated alias columns 0012 adds; refusing', n;
    END IF;
END $$;

LOCK TABLE tenant, tenant_domain, tenant_succession, province, mini_app, commune_profile,
    citizen_report_field IN ACCESS EXCLUSIVE MODE;

-- 2. The rolling-update aliases.
DROP VIEW petition_field;
DROP VIEW ho_so_hien_thi_xa;
ALTER TABLE mini_app DROP COLUMN dang_hoat_dong, DROP COLUMN che_do;
ALTER TABLE tenant_domain DROP COLUMN la_chinh;
ALTER TABLE tenant DROP COLUMN dang_hoat_dong, DROP COLUMN tinh_thanh, DROP COLUMN ten;

-- 3a. petition_field (0011).
ALTER TRIGGER citizen_report_field_guard ON citizen_report_field RENAME TO petition_field_guard;
-- The body of 0011:101-115, verbatim.
CREATE OR REPLACE FUNCTION citizen_report_field_guard() RETURNS trigger
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
ALTER FUNCTION citizen_report_field_guard() RENAME TO petition_field_guard;
ALTER TABLE citizen_report_field RENAME CONSTRAINT citizen_report_field_signed TO petition_field_signed;
ALTER TABLE citizen_report_field RENAME CONSTRAINT citizen_report_field_tone_known TO petition_field_tone_known;
ALTER TABLE citizen_report_field RENAME CONSTRAINT citizen_report_field_sort_order_positive TO petition_field_sort_order_positive;
ALTER TABLE citizen_report_field RENAME CONSTRAINT citizen_report_field_label_not_blank TO petition_field_label_not_blank;
ALTER TABLE citizen_report_field RENAME CONSTRAINT citizen_report_field_code_shape TO petition_field_code_shape;
ALTER TABLE citizen_report_field RENAME CONSTRAINT citizen_report_field_pkey TO petition_field_pkey;
ALTER TABLE citizen_report_field RENAME COLUMN is_active TO active;
ALTER TABLE citizen_report_field RENAME TO petition_field;
-- 0011:89-91, verbatim.
COMMENT ON TABLE petition_field IS
    'Bo ma linh vuc phan anh TANG 1 (ADR 0026, ADR 0060): mot bo cho moi xa. Ma khong bao gio doi, '
    'khong bao gio xoa - phieu luu tru giu ma duoi dang gia tri. Ngung dung = active false.';

-- 3b. The shared hard-delete refusal (0006).
ALTER FUNCTION platform_no_hard_delete() RENAME TO platform_cam_xoa_cung;                                        -- vi-name-ok: restores the pre-0012 name

-- 3c. ho_so_hien_thi_xa (0006).
ALTER TRIGGER commune_profile_no_hard_delete ON commune_profile RENAME TO ho_so_hien_thi_xa_cam_xoa_cung;          -- vi-name-ok: restores the pre-0012 name
ALTER TABLE commune_profile RENAME CONSTRAINT commune_profile_soft_delete_complete TO ho_so_hien_thi_xa_xoa_mem_day_du;          -- vi-name-ok: restores the pre-0012 name
ALTER TABLE commune_profile RENAME CONSTRAINT commune_profile_signed TO ho_so_hien_thi_xa_nguoi_ghi_khong_rong;                   -- vi-name-ok: restores the pre-0012 name
ALTER TABLE commune_profile RENAME CONSTRAINT commune_profile_logo_not_blank TO ho_so_hien_thi_xa_logo_khong_rong;               -- vi-name-ok: restores the pre-0012 name
ALTER TABLE commune_profile RENAME CONSTRAINT commune_profile_tenant_id_fkey TO ho_so_hien_thi_xa_tenant_id_fkey;                -- vi-name-ok: restores the pre-0012 name
ALTER TABLE commune_profile RENAME CONSTRAINT commune_profile_pkey TO ho_so_hien_thi_xa_pkey;                                    -- vi-name-ok: restores the pre-0012 name
ALTER TABLE commune_profile RENAME COLUMN updated_by TO cap_nhat_boi;                                           -- vi-name-ok: restores the pre-0012 name
ALTER TABLE commune_profile RENAME COLUMN updated_at TO cap_nhat_luc;                                           -- vi-name-ok: restores the pre-0012 name
ALTER TABLE commune_profile RENAME COLUMN created_by TO tao_boi;                                                -- vi-name-ok: restores the pre-0012 name
ALTER TABLE commune_profile RENAME COLUMN created_at TO tao_luc;                                                -- vi-name-ok: restores the pre-0012 name
ALTER TABLE commune_profile RENAME COLUMN introduction TO gioi_thieu;                                           -- vi-name-ok: restores the pre-0012 name
ALTER TABLE commune_profile RENAME COLUMN office_hours_text TO gio_lam_viec_hien_thi;                           -- vi-name-ok: restores the pre-0012 name
ALTER TABLE commune_profile RENAME COLUMN hotline TO duong_day_nong;                                            -- vi-name-ok: restores the pre-0012 name
ALTER TABLE commune_profile RENAME COLUMN office_address TO dia_chi_tru_so;                                     -- vi-name-ok: restores the pre-0012 name
-- @entity and @scope marks stay in 0006; this file is not read by the tools.
ALTER TABLE commune_profile RENAME TO ho_so_hien_thi_xa;                                                        -- vi-name-ok: restores the pre-0012 name

-- 3d. mini_app (0006).
ALTER TRIGGER mini_app_no_hard_delete ON mini_app RENAME TO mini_app_cam_xoa_cung;                              -- vi-name-ok: restores the pre-0012 name
ALTER TABLE mini_app RENAME CONSTRAINT mini_app_soft_delete_complete TO mini_app_xoa_mem_day_du;                -- vi-name-ok: restores the pre-0012 name
ALTER TABLE mini_app RENAME CONSTRAINT mini_app_signed TO mini_app_nguoi_ghi_khong_rong;                        -- vi-name-ok: restores the pre-0012 name
ALTER TABLE mini_app RENAME CONSTRAINT mini_app_tenant_iff_dedicated TO mini_app_xa_khi_va_chi_khi_rieng;       -- vi-name-ok: restores the pre-0012 name
ALTER TABLE mini_app RENAME CONSTRAINT mini_app_mode_known TO mini_app_che_do_hop_le;                           -- vi-name-ok: restores the pre-0012 name
ALTER TABLE mini_app RENAME CONSTRAINT mini_app_app_id_not_blank TO mini_app_app_id_khong_rong;                 -- vi-name-ok: restores the pre-0012 name
ALTER TABLE mini_app RENAME COLUMN updated_by TO cap_nhat_boi;                                                  -- vi-name-ok: restores the pre-0012 name
ALTER TABLE mini_app RENAME COLUMN updated_at TO cap_nhat_luc;                                                  -- vi-name-ok: restores the pre-0012 name
ALTER TABLE mini_app RENAME COLUMN created_by TO tao_boi;                                                       -- vi-name-ok: restores the pre-0012 name
ALTER TABLE mini_app RENAME COLUMN created_at TO tao_luc;                                                       -- vi-name-ok: restores the pre-0012 name
ALTER TABLE mini_app RENAME COLUMN note TO ghi_chu;                                                             -- vi-name-ok: restores the pre-0012 name
ALTER TABLE mini_app RENAME COLUMN is_active TO dang_hoat_dong;                                                 -- vi-name-ok: restores the pre-0012 name
ALTER TABLE mini_app RENAME COLUMN mode TO che_do;                                                              -- vi-name-ok: restores the pre-0012 name
-- 0006:102-104, verbatim.
COMMENT ON COLUMN mini_app.tenant_id IS
    'Xa gan app rieng. NULL khi va chi khi che_do = chinh. Xa ngung hoat dong thi app bi tu choi, '
    'KHONG tu di theo tenant_succession (ADR 0045); gan lai la viec cua nguoi van hanh.';

-- 3e. tinh_thanh (0004).
ALTER INDEX province_list RENAME TO tinh_thanh_danh_sach;                                                       -- vi-name-ok: restores the pre-0012 name
ALTER TABLE province RENAME CONSTRAINT province_name_not_blank TO tinh_thanh_ten_khong_rong;                    -- vi-name-ok: restores the pre-0012 name
ALTER TABLE province RENAME CONSTRAINT province_name_unique TO tinh_thanh_ten_duy_nhat;                         -- vi-name-ok: restores the pre-0012 name
ALTER TABLE province RENAME CONSTRAINT province_id_is_ulid TO tinh_thanh_id_la_ulid;                            -- vi-name-ok: restores the pre-0012 name
ALTER TABLE province RENAME CONSTRAINT province_pkey TO tinh_thanh_pkey;                                        -- vi-name-ok: restores the pre-0012 name
ALTER TABLE province RENAME COLUMN updated_at TO cap_nhat_luc;                                                  -- vi-name-ok: restores the pre-0012 name
ALTER TABLE province RENAME COLUMN created_at TO tao_luc;                                                       -- vi-name-ok: restores the pre-0012 name
ALTER TABLE province RENAME COLUMN is_active TO dang_hoat_dong;                                                 -- vi-name-ok: restores the pre-0012 name
ALTER TABLE province RENAME COLUMN sort_order TO thu_tu;                                                        -- vi-name-ok: restores the pre-0012 name
ALTER TABLE province RENAME COLUMN name TO ten;                                                                 -- vi-name-ok: restores the pre-0012 name
ALTER TABLE province RENAME TO tinh_thanh;                                                                      -- vi-name-ok: restores the pre-0012 name
-- 0004:91-93, verbatim.
COMMENT ON TABLE tinh_thanh IS
    'Danh muc don vi hanh chinh cap tinh. Nha cung cap seed, xa chi duoc CHON. Ly do: '
    'tenant.tinh_thanh la chuoi tu nhap, hai cach viet mot tinh thi cong dan thay hai tinh.';

-- 3f. tenant_succession (0003).
ALTER TRIGGER tenant_succession_no_cycle ON tenant_succession RENAME TO tenant_succession_chan_vong_lap;         -- vi-name-ok: restores the pre-0012 name
ALTER INDEX tenant_succession_by_to_tenant RENAME TO tenant_succession_theo_den;                                -- vi-name-ok: restores the pre-0012 name
ALTER TABLE tenant_succession RENAME CONSTRAINT tenant_succession_not_self TO tenant_succession_khong_tro_chinh_no;             -- vi-name-ok: restores the pre-0012 name
ALTER TABLE tenant_succession RENAME CONSTRAINT tenant_succession_has_legal_basis TO tenant_succession_co_can_cu;              -- vi-name-ok: restores the pre-0012 name
ALTER TABLE tenant_succession RENAME CONSTRAINT tenant_succession_to_tenant_id_fkey TO tenant_succession_den_id_fkey;          -- vi-name-ok: restores the pre-0012 name
ALTER TABLE tenant_succession RENAME CONSTRAINT tenant_succession_from_tenant_id_fkey TO tenant_succession_tu_id_fkey;         -- vi-name-ok: restores the pre-0012 name
ALTER TABLE tenant_succession RENAME COLUMN created_by TO tao_boi;                                              -- vi-name-ok: restores the pre-0012 name
ALTER TABLE tenant_succession RENAME COLUMN created_at TO tao_luc;                                              -- vi-name-ok: restores the pre-0012 name
ALTER TABLE tenant_succession RENAME COLUMN note TO ghi_chu;                                                    -- vi-name-ok: restores the pre-0012 name
ALTER TABLE tenant_succession RENAME COLUMN effective_from TO hieu_luc_tu;                                      -- vi-name-ok: restores the pre-0012 name
ALTER TABLE tenant_succession RENAME COLUMN legal_basis TO can_cu;                                              -- vi-name-ok: restores the pre-0012 name
ALTER TABLE tenant_succession RENAME COLUMN to_tenant_id TO den_id;                                             -- vi-name-ok: restores the pre-0012 name
ALTER TABLE tenant_succession RENAME COLUMN from_tenant_id TO tu_id;                                            -- vi-name-ok: restores the pre-0012 name
-- The body of 0003:128-144, verbatim, then the name back.
CREATE OR REPLACE FUNCTION tenant_succession_no_cycle() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF EXISTS (
        WITH RECURSIVE di(id) AS (
            SELECT NEW.den_id
            UNION
            SELECT s.den_id FROM tenant_succession s JOIN di ON s.tu_id = di.id
        )
        SELECT 1 FROM di WHERE id = NEW.tu_id
    ) THEN
        RAISE EXCEPTION
            'tenant_succession: canh % -> % tao thanh vong lap ke thua', NEW.tu_id, NEW.den_id
            USING HINT = 'Don vi ke thua da dan nguoc ve don vi cu. Kiem tra lai ULID da nhap.';
    END IF;
    RETURN NEW;
END $$;
ALTER FUNCTION tenant_succession_no_cycle() RENAME TO tenant_succession_chan_vong_lap;                          -- vi-name-ok: restores the pre-0012 name

-- 3g. tenant_domain (0001, 0007).
ALTER INDEX tenant_domain_one_primary RENAME TO tenant_domain_mot_chinh;                                        -- vi-name-ok: restores the pre-0012 name
ALTER INDEX tenant_domain_by_tenant RENAME TO tenant_domain_theo_xa;                                            -- vi-name-ok: restores the pre-0012 name
ALTER TABLE tenant_domain RENAME CONSTRAINT tenant_domain_host_lowercase TO tenant_domain_host_thuong;          -- vi-name-ok: restores the pre-0012 name
ALTER TABLE tenant_domain RENAME CONSTRAINT tenant_domain_not_reserved TO tenant_domain_khong_danh_rieng;       -- vi-name-ok: restores the pre-0012 name
ALTER TABLE tenant_domain RENAME COLUMN created_at TO tao_luc;                                                  -- vi-name-ok: restores the pre-0012 name
ALTER TABLE tenant_domain RENAME COLUMN is_primary TO la_chinh;                                                 -- vi-name-ok: restores the pre-0012 name

-- 3h. tenant (0001).
ALTER TABLE tenant RENAME CONSTRAINT tenant_id_is_ulid TO tenant_id_la_ulid;                                    -- vi-name-ok: restores the pre-0012 name
ALTER TABLE tenant RENAME COLUMN updated_at TO cap_nhat_luc;                                                    -- vi-name-ok: restores the pre-0012 name
ALTER TABLE tenant RENAME COLUMN created_at TO tao_luc;                                                         -- vi-name-ok: restores the pre-0012 name
ALTER TABLE tenant RENAME COLUMN is_active TO dang_hoat_dong;                                                   -- vi-name-ok: restores the pre-0012 name
ALTER TABLE tenant RENAME COLUMN province_name TO tinh_thanh;                                                  -- vi-name-ok: restores the pre-0012 name
ALTER TABLE tenant RENAME COLUMN name TO ten;                                                                   -- vi-name-ok: restores the pre-0012 name

-- 3i. PostgreSQL 18 NOT NULL constraint names — the same rule 0012 applies, over the old names.
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
          AND cl.relname IN ('tenant', 'tenant_domain', 'tenant_succession', 'tinh_thanh', 'mini_app',
                             'ho_so_hien_thi_xa', 'petition_field')
          AND k.conname LIKE '%\_not\_null'
          AND k.conname <> cl.relname || '_' || a.attname || '_not_null'
    LOOP
        EXECUTE format('ALTER TABLE %I RENAME CONSTRAINT %I TO %I',
                       r.relname, r.conname, r.relname || '_' || r.attname || '_not_null');
    END LOOP;
END $$;

-- ---------------------------------------------------------------------------
-- 4. The progress row, in this same transaction: without it the runner would still believe the
-- renamed schema is in place, and the previous image would keep refusing to start. Keyed on the
-- exact file name, one row, never an unfiltered statement.
-- ---------------------------------------------------------------------------
DELETE FROM schema_migration WHERE ten = '0012_rename_schema_to_english.sql';

COMMIT;
