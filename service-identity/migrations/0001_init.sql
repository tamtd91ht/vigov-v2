-- identity — initial schema
--
-- Invariants enforced here, not by convention (ADR 0004):
--   * every business table carries tenant_id NOT NULL
--   * every unique key is COMPOSITE with tenant_id  — a single-column key breaks commune #2
--   * every index starts with tenant_id             — it is the shard key
--   * large tables are PARTITIONED by tenant from the start: adding partitioning later is a
--     migration over archival records, out of hours, with real risk. Now it costs nothing.
--
-- MODULUS 32 everywhere (ADR 0010).
--
-- Migrations run PER COMMUNE, are resumable, and record progress.

-- ---------------------------------------------------------------------------
-- Audit trail. One table per service, on purpose: rule 6 requires the entry to share a
-- transaction with the business write, and two services cannot share a transaction.
-- Append-only: entries are never modified and never removed, not even by an administrator.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS audit_log (
    id          BIGSERIAL,
    tenant_id   TEXT        NOT NULL,
    actor_id    TEXT        NOT NULL,
    actor_kind  TEXT        NOT NULL,
    actor_ip    TEXT        NOT NULL DEFAULT '',
    action      TEXT        NOT NULL,
    subject     TEXT        NOT NULL,
    at          TIMESTAMPTZ NOT NULL,
    delta       JSONB,
    PRIMARY KEY (tenant_id, id)
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS audit_log_p%s PARTITION OF audit_log '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

CREATE INDEX IF NOT EXISTS audit_log_lookup
    ON audit_log (tenant_id, subject, at DESC);

-- ---------------------------------------------------------------------------
-- quyen — the catalogue of permission keys.
--
-- PLATFORM-WIDE ON PURPOSE, and the only table here without tenant_id: the set of rights the
-- software can enforce is a property of the SOFTWARE, not of a commune. A commune decides
-- which role holds which right (vai_tro_quyen, per commune); it cannot invent a right the code
-- does not check, because a key nobody checks grants nothing and would only mislead the
-- administrator ticking it.
--
-- `ma` is the flat key the Phân quyền screen shows and the code passes to
-- authz.RequirePermission — one string, no translation layer to drift.
--
-- @scope:  platform
-- The prose above has said this since the table was written; the token is what a MACHINE can
-- read. Added 23/09/2026, when tools/check_khoa_duy_nhat.py learned to inspect PRIMARY KEY at
-- all — until then it read UNIQUE only, so this single-column key went unexamined rather than
-- approved. A rule stated in prose and enforced nowhere is a rule that survives exactly as long
-- as the people who remember it.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS quyen (
    ma      TEXT PRIMARY KEY,           -- "task.extend"
    nhom    TEXT NOT NULL,              -- "NHIỆM VỤ" — for grouping on the matrix only
    nhan    TEXT NOT NULL,              -- "Duyệt gia hạn"
    thu_tu  INT  NOT NULL DEFAULT 0,
    CONSTRAINT quyen_ma_co_dau_cham CHECK (position('.' in ma) > 1)
);

-- ---------------------------------------------------------------------------
-- bo_phan — the org chart, a parent/child tree within one commune.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS bo_phan (
    tenant_id     TEXT        NOT NULL,
    id            TEXT        NOT NULL,
    ten           TEXT        NOT NULL,          -- "VĂN PHÒNG ĐẢNG ỦY", upper-case by convention
    ma            TEXT        NOT NULL,          -- slug: "van-phong-dang-uy"
    cha_id        TEXT,                          -- NULL at the root
    thu_tu        INT         NOT NULL DEFAULT 0,
    deleted_at    TIMESTAMPTZ,
    deleted_by    TEXT,
    delete_reason TEXT,
    tao_luc       TIMESTAMPTZ NOT NULL DEFAULT now(),
    cap_nhat_luc  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, id),
    UNIQUE (tenant_id, ma),
    FOREIGN KEY (tenant_id, cha_id) REFERENCES bo_phan (tenant_id, id)
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS bo_phan_p%s PARTITION OF bo_phan '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

CREATE INDEX IF NOT EXISTS bo_phan_cay ON bo_phan (tenant_id, cha_id, thu_tu);

-- ---------------------------------------------------------------------------
-- vai_tro — roles, defined per commune.
--
-- Per commune and not platform-wide because the same title carries different authority in
-- different places: a "Trưởng bộ phận" in a commune of eight staff signs off work that a
-- commune of forty routes to a deputy chairman.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS vai_tro (
    tenant_id     TEXT        NOT NULL,
    id            TEXT        NOT NULL,
    ten           TEXT        NOT NULL,          -- "Chủ tịch UBND"
    ma            TEXT        NOT NULL,          -- "chu-tich-ubnd"
    la_lanh_dao   BOOLEAN     NOT NULL DEFAULT false,
    thu_tu        INT         NOT NULL DEFAULT 0,
    deleted_at    TIMESTAMPTZ,
    deleted_by    TEXT,
    delete_reason TEXT,
    tao_luc       TIMESTAMPTZ NOT NULL DEFAULT now(),
    cap_nhat_luc  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, id),
    UNIQUE (tenant_id, ma)
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS vai_tro_p%s PARTITION OF vai_tro '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

-- ---------------------------------------------------------------------------
-- vai_tro_quyen — which role holds which permission, WITHIN one commune.
--
-- The tenant_id here is what stops commune A's grant from applying in commune B. Rule 5,
-- invariant 3: a permission check that forgets the commune is cross-commune escalation, not a
-- lesser bug — and this composite key is where that is decided.
--
-- The foreign key to quyen(ma) is declared because the catalogue is platform-wide: it stops a
-- commune from being granted a right the code never checks, which would put a tick box on the
-- Phân quyền screen that silently does nothing.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS vai_tro_quyen (
    tenant_id   TEXT        NOT NULL,
    vai_tro_id  TEXT        NOT NULL,
    quyen_ma    TEXT        NOT NULL REFERENCES quyen (ma),
    cap_luc     TIMESTAMPTZ NOT NULL DEFAULT now(),
    cap_boi     TEXT        NOT NULL DEFAULT '',
    PRIMARY KEY (tenant_id, vai_tro_id, quyen_ma),
    FOREIGN KEY (tenant_id, vai_tro_id) REFERENCES vai_tro (tenant_id, id)
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS vai_tro_quyen_p%s PARTITION OF vai_tro_quyen '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

-- ---------------------------------------------------------------------------
-- nguoi_dung — staff accounts.
--
-- Staff identity is ACCOUNTABLE (rule 4): every action traces back to a person. That is the
-- opposite of citizen identity, which is a phone number plus an OTP and is deliberately weak.
-- The two never share a table.
--
-- `email` is unique WITHIN a commune, not globally: one person may legitimately hold accounts
-- in two communes after a merger, and a global unique key would make the second one
-- impossible to create.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS nguoi_dung (
    tenant_id         TEXT        NOT NULL,
    id                TEXT        NOT NULL,
    ma                TEXT        NOT NULL,          -- staff code used in the audit trail
    ho_ten            TEXT        NOT NULL,
    email             TEXT        NOT NULL,
    chuc_vu           TEXT        NOT NULL DEFAULT '',
    bo_phan_id        TEXT,
    vai_tro_id        TEXT,
    dien_thoai        TEXT        NOT NULL DEFAULT '',
    -- Argon2id hash. NEVER the password itself, and never logged (rule 3, rule 8).
    mat_khau_hash     TEXT        NOT NULL DEFAULT '',
    dang_hoat_dong    BOOLEAN     NOT NULL DEFAULT true,
    dang_nhap_gan_nhat TIMESTAMPTZ,
    deleted_at        TIMESTAMPTZ,
    deleted_by        TEXT,
    delete_reason     TEXT,
    tao_luc           TIMESTAMPTZ NOT NULL DEFAULT now(),
    cap_nhat_luc      TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, id),
    UNIQUE (tenant_id, ma),
    UNIQUE (tenant_id, email),
    FOREIGN KEY (tenant_id, bo_phan_id) REFERENCES bo_phan (tenant_id, id),
    FOREIGN KEY (tenant_id, vai_tro_id) REFERENCES vai_tro (tenant_id, id)
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS nguoi_dung_p%s PARTITION OF nguoi_dung '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

-- Every read path filters deleted rows, so the index carries that condition too.
CREATE INDEX IF NOT EXISTS nguoi_dung_theo_bo_phan
    ON nguoi_dung (tenant_id, bo_phan_id) WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS nguoi_dung_dang_nhap
    ON nguoi_dung (tenant_id, email) WHERE deleted_at IS NULL AND dang_hoat_dong;

-- ---------------------------------------------------------------------------
-- phien — the session registry.
--
-- WHY A TABLE AND NOT ONLY A SIGNED TOKEN: a token that is merely signed cannot be taken
-- back. Locking an account, changing a role or changing a password must end every open
-- session AT ONCE (skills/session-and-token, required #7) — with a stateless token the former
-- employee keeps working until it expires on its own.
--
-- Every request looks up `sid` here, so this table sits on the hot path and is indexed for
-- exactly that lookup.
--
-- A session belongs to ONE commune. A staff member with accounts in two communes holds two
-- separate sessions, and neither can be used against the other's domain.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS phien (
    tenant_id      TEXT        NOT NULL,
    id             TEXT        NOT NULL,           -- the `sid` carried in the token
    nguoi_dung_id  TEXT        NOT NULL,
    -- SHA-256 of the refresh token. The token itself is never stored: a leaked backup must not
    -- hand over working sessions.
    refresh_hash   TEXT        NOT NULL,
    -- Rotating refresh tokens: reusing a superseded one revokes the whole chain, which is how
    -- a stolen token is detected (skills/session-and-token, required #4).
    thay_the_boi   TEXT,
    tao_luc        TIMESTAMPTZ NOT NULL DEFAULT now(),
    het_han_luc    TIMESTAMPTZ NOT NULL,
    thu_hoi_luc    TIMESTAMPTZ,
    thu_hoi_ly_do  TEXT,
    dung_gan_nhat  TIMESTAMPTZ,
    ip_tao         TEXT        NOT NULL DEFAULT '',
    -- User agent, truncated. Never personal data (rule 3).
    thiet_bi       TEXT        NOT NULL DEFAULT '',
    PRIMARY KEY (tenant_id, id),
    FOREIGN KEY (tenant_id, nguoi_dung_id) REFERENCES nguoi_dung (tenant_id, id)
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS phien_p%s PARTITION OF phien '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

-- The hot-path lookup: is this sid still valid, right now.
CREATE INDEX IF NOT EXISTS phien_con_hieu_luc
    ON phien (tenant_id, id) WHERE thu_hoi_luc IS NULL;

-- Revoking every session of one staff member, which happens on lock, role change and password
-- change — three operations that must not scan the table.
CREATE INDEX IF NOT EXISTS phien_theo_can_bo
    ON phien (tenant_id, nguoi_dung_id) WHERE thu_hoi_luc IS NULL;

CREATE INDEX IF NOT EXISTS phien_refresh
    ON phien (tenant_id, refresh_hash);

-- ---------------------------------------------------------------------------
-- Seed: the 33 permission keys from the specification.
--
-- Seeded here rather than created by an administrator because the code is what enforces them:
-- a key that no route checks grants nothing, and a route checking a key that does not exist
-- can never be reached. The two lists must match, and this is the one that ships with the code.
--
-- The specification's heading says 43 while listing 33. The ten unaccounted keys are an OPEN
-- QUESTION, not something to invent here: a permission invented by a developer is a right
-- granted by nobody in particular.
-- ---------------------------------------------------------------------------
INSERT INTO quyen (ma, nhom, nhan, thu_tu) VALUES
    ('admin.audit',          'QUẢN TRỊ',           'Xem nhật ký hệ thống',                 1),
    ('admin.lookup',         'QUẢN TRỊ',           'Quản lý danh mục',                     2),
    ('admin.org',            'QUẢN TRỊ',           'Quản lý sơ đồ tổ chức',                3),
    ('admin.role',           'QUẢN TRỊ',           'Phân quyền',                           4),
    ('admin.sla',            'QUẢN TRỊ',           'Cấu hình thời hạn xử lý',              5),
    ('admin.user',           'QUẢN TRỊ',           'Quản lý người dùng',                   6),
    ('announcement.create',  'THÔNG BÁO',          'Soạn và gửi thông báo',                7),
    ('asset.read',           'BẢN ĐỒ TÀI NGUYÊN',  'Xem bản đồ tài nguyên',                8),
    ('asset.update',         'BẢN ĐỒ TÀI NGUYÊN',  'Cập nhật tài nguyên',                  9),
    ('budget.confirm',       'GIẢI NGÂN',          'Xác nhận, khoá khoản giải ngân',      10),
    ('budget.read',          'GIẢI NGÂN',          'Xem giải ngân',                       11),
    ('budget.update',        'GIẢI NGÂN',          'Cập nhật giải ngân',                  12),
    ('content.read',         'NỘI DUNG MINI APP',  'Xem nội dung và danh bạ Mini App',    13),
    ('content.update',       'NỘI DUNG MINI APP',  'Sửa nội dung và danh bạ Mini App',    14),
    ('document.create',      'VĂN BẢN',            'Vào sổ văn bản',                      15),
    ('document.read',        'VĂN BẢN',            'Xem văn bản',                         16),
    ('document.route',       'VĂN BẢN',            'Phân luồng văn bản',                  17),
    ('feedback.assign',      'PHẢN ÁNH',           'Phân công xử lý phản ánh',            18),
    ('feedback.create',      'PHẢN ÁNH',           'Tiếp nhận phản ánh',                  19),
    ('feedback.read',        'PHẢN ÁNH',           'Xem phản ánh',                        20),
    ('feedback.resolve',     'PHẢN ÁNH',           'Kết thúc xử lý phản ánh',             21),
    ('feedback.restricted',  'PHẢN ÁNH',           'Xem phản ánh về tác phong cán bộ',    22),
    ('petition.create',      'ĐƠN THƯ',            'Tiếp nhận đơn thư',                   23),
    ('petition.read',        'ĐƠN THƯ',            'Xem đơn thư',                         24),
    ('report.export',        'BÁO CÁO',            'Xuất báo cáo',                        25),
    ('report.read',          'BÁO CÁO',            'Xem báo cáo',                         26),
    ('task.approve',         'NHIỆM VỤ',           'Duyệt hoàn thành',                    27),
    ('task.assign',          'NHIỆM VỤ',           'Giao nhiệm vụ',                       28),
    ('task.create',          'NHIỆM VỤ',           'Tạo nhiệm vụ',                        29),
    ('task.delete',          'NHIỆM VỤ',           'Xoá nhiệm vụ khỏi sổ',                30),
    ('task.extend',          'NHIỆM VỤ',           'Duyệt gia hạn',                       31),
    ('task.read',            'NHIỆM VỤ',           'Xem nhiệm vụ',                        32),
    ('task.update',          'NHIỆM VỤ',           'Cập nhật tiến độ',                    33)
ON CONFLICT (ma) DO UPDATE SET
    nhom   = EXCLUDED.nhom,
    nhan   = EXCLUDED.nhan,
    thu_tu = EXCLUDED.thu_tu;
