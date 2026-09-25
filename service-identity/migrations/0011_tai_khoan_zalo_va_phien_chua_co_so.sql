-- identity — the Zalo account behind a Mini App session, and the citizen session that has no
-- verified phone yet (ADR 0045 §Phiên chưa có số điện thoại, owner's answer to CÒN MỞ #2).
--
-- Every section names the decision it carries out:
--
--   ADR 0045 §Phiên chưa có số, point 1 — a new entity `tai_khoan_zalo`, keyed on
--       (app_id, zalo_user_id), whose citizen identity MAY BE EMPTY, carrying the remembered
--       commune of the main app. No tenant_id, for the reason dinh_danh_cong_dan has none: it is
--       the thing that ANSWERS "which commune".
--   point 2 — the citizen identity stays anchored on the phone number (ADR 0002 unchanged). The
--       account only POINTS at an identity once a phone has been verified, this time or earlier.
--   point 3 — `phien_cong_dan` gains `tai_khoan_zalo_id`, and `cong_dan_id` becomes NULLABLE
--       (relaxing NOT NULL loses nothing). A NULL citizen id now means "phone not verified yet".
--
-- WHY A NEW FILE AND NOT AN EDIT TO 0004: core/migrate checksums every applied file at startup and
-- stops the service when one has changed (ErrChecksumLech).
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS, answered before the SQL.
--
-- 1. HOW MANY ROWS PER COMMUNE. `tai_khoan_zalo` is new and empty. `phien_cong_dan` holds, in the
--    worst case, test rows only: the only writers until this change were tests
--    (internal/store/phien_cong_dan.go, Tao and TaoChuaChonXa have no production caller). This
--    file REWRITES NONE OF THEM: the new column is added NULL, and every existing row already has
--    a cong_dan_id, so it satisfies the new CHECK as it stands.
--
-- 2. IF IT STOPS HALF-WAY. It cannot land half-applied: core/migrate runs the file in ONE
--    transaction together with its progress row. Every statement is also safe on a second run
--    (IF NOT EXISTS, DROP NOT NULL is idempotent, the constraint is guarded by pg_constraint,
--    COMMENT ON replaces).
--
-- 3. HOW IT IS REVERSED. See REVERSAL at the bottom.
--
-- 4. WHICH READ PATHS CHANGE MEANING WHILE IT IS HALF-APPLIED. None change visibility: no
--    predicate on `thu_hoi_luc` or `het_han_luc` is touched, so every session that resolved before
--    resolves after, to the same commune. ONE READ CHANGES TYPE: `phien_cong_dan.cong_dan_id` may
--    now be NULL, and the session lookup scans it as sql.NullString in the SAME commit
--    (store/phien_cong_dan.go, quetPhien) — scanning NULL into a plain string would be a Scan error,
--    which that function answers as "no session", signing such a citizen out on every request.
--    The four consumers of an empty citizen id change in the same commit too (ADR 0045 point 5):
--    ResolveCitizenSession, core/identityclient, core/httpx.XaTuPhien (which REFUSES it by
--    default) and the new explicit view-only class.
--
-- 5. RETENTION. Nothing is removed, retyped or renumbered; no issued code is touched. Rule 7's
--    stop conditions are not reached.
--
-- PER-COMMUNE AND RESUMABLE (rule 7, invariant 5): NO BACKFILL — no row is rewritten — so there is
-- nothing to resume and no commune to iterate.
--
-- PERSONAL DATA (rule 3). The Zalo user id is an online identifier under Decree 13/2023 and is
-- NEVER stored raw: only its SHA-256 (ADR 0045 §Dữ liệu cá nhân — "băm tra được vì không bao giờ
-- cần hiện ra"). Nothing in this file holds a phone number; the account points at
-- dinh_danh_cong_dan, which is the one place a number lives.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- 1. tai_khoan_zalo — one Zalo account as seen through ONE Mini App.
--
-- KEYED ON (app_id, the hash of zalo_user_id), NEVER ON THE ZALO ID ALONE. Whether Zalo scopes the
-- user id per app is ADR 0045 UNKNOWN #2 and nobody has measured it. If it is per app, one person
-- using two apps is two accounts here, correctly; if it is global, the composite key still holds and
-- merely stores the fact twice. Keying on the id alone would be right in only one of the two worlds.
--
-- THE ZALO ID IS STORED AS SHA-256 HEX, NOT RAW. It is only ever LOOKED UP, never shown: there is
-- no screen, report or export that needs it back, so a hash loses nothing and a leaked backup does
-- not hand over a list of citizens' Zalo accounts. WHAT THE HASH DOES NOT BUY, stated so nobody
-- assumes more: it is unsalted, because the lookup must be deterministic, so whoever already holds a
-- candidate id can test it. A keyed hash (HMAC) would close that, and needs a platform secret with a
-- rotation story — a new secret is rule 8 stop condition #1, not something to invent in a migration.
--
-- `cong_dan_id` NULL MEANS "PHONE NOT VERIFIED THROUGH THIS ACCOUNT YET". Once a verified phone
-- arrives (ADR 0045: asked ONCE, at the first act needing identity) it points at the identity of
-- that number, and later silent opens get a session WITH that identity without asking again — the
-- owner's "cờ đã xác thực số" IS this column being non-NULL. It is not a separate boolean: a flag
-- beside a foreign key is two sources for one fact, and the stale one is the one somebody reads.
--
-- A PHONE CHANGE MOVES THE POINTER AND DOES NOTHING ELSE. When a later phone token names a different
-- number, the account points at that number's identity and the move is audited. The two identities
-- are NOT merged and the old one's records are NOT transferred — ADR 0020 CÒN MỞ #1, ADR 0045
-- ĐIỀU KIỆN DỪNG #7.
--
-- `xa_da_nho` — THE REMEMBERED COMMUNE OF THE MAIN APP, NULL = NONE. Written only when the citizen
-- CONFIRMED a commune (QR + confirmation screen, ADR 0045 §Chế độ). A dedicated app never writes it:
-- its commune is the app's binding. NULL rather than '' because "no commune remembered" has no
-- counterpart in the isolation path here — this value is read, compared against the registry, and
-- only then used; it is never copied into a session unchecked.
--
-- NO deleted_at / deleted_by / delete_reason, AND THE REASON IS THE ONE dinh_danh_cong_dan GIVES
-- (0004): every session opened through this account points at the row, and soft-deleting it would
-- leave those rows pointing at a subject the system agreed to stop returning. There is no delete
-- path at all. An erasure request under Decree 13/2023 is ANONYMISATION (rule 7, invariant 7) —
-- replace the hash, keep the row and its id — and that mechanism is deliberately not built here,
-- for the same reason it is not built for dinh_danh_cong_dan: nobody has decided who may request it.
-- ---------------------------------------------------------------------------
-- @entity: ZaloAccount
-- @scope:  cross-tenant
CREATE TABLE IF NOT EXISTS tai_khoan_zalo (
    -- Opaque ULID, minted by the server. It is the audit "who" of a session opened before the
    -- phone is verified (ADR 0045 §Ghi vết) — never the Zalo id, never a phone number.
    id               TEXT        PRIMARY KEY,
    -- The Zalo Mini App the account was seen through. Not personal data (platform.proto).
    app_id           TEXT        NOT NULL,
    -- SHA-256 hex of the Zalo user id. PERSONAL DATA'S FINGERPRINT — never the raw id.
    bam_zalo_user_id TEXT        NOT NULL,
    -- The platform-wide citizen identity this account's verified phone belongs to. NULL = not yet.
    cong_dan_id      TEXT        REFERENCES dinh_danh_cong_dan (id),
    lien_ket_luc     TIMESTAMPTZ,
    -- The commune the citizen last CONFIRMED in the main app. NULL = none.
    xa_da_nho        TEXT,
    xa_da_nho_luc    TIMESTAMPTZ,
    tao_luc          TIMESTAMPTZ NOT NULL DEFAULT now(),
    cap_nhat_luc     TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT tai_khoan_zalo_id_la_ulid CHECK (length(id) = 26),
    -- Composite on purpose — see the block above. Not rule 1 forbidden #4: this table belongs to no
    -- commune, exactly like dinh_danh_cong_dan's platform-wide unique phone.
    CONSTRAINT tai_khoan_zalo_khoa_duy_nhat UNIQUE (app_id, bam_zalo_user_id),
    CONSTRAINT tai_khoan_zalo_app_id_co_gia_tri CHECK (btrim(app_id) <> ''),
    -- Refuses a raw id reaching the column through a future write path that forgot to hash.
    CONSTRAINT tai_khoan_zalo_bam_la_sha256 CHECK (bam_zalo_user_id ~ '^[0-9a-f]{64}$'),
    CONSTRAINT tai_khoan_zalo_lien_ket_co_thoi_diem
        CHECK ((cong_dan_id IS NULL) = (lien_ket_luc IS NULL)),
    CONSTRAINT tai_khoan_zalo_xa_da_nho_la_ulid
        CHECK (xa_da_nho IS NULL OR length(xa_da_nho) = 26),
    CONSTRAINT tai_khoan_zalo_xa_da_nho_co_thoi_diem
        CHECK ((xa_da_nho IS NULL) = (xa_da_nho_luc IS NULL))
);

COMMENT ON TABLE tai_khoan_zalo IS
    'Mot tai khoan Zalo nhin qua MOT Mini App (ADR 0045). Khong co tenant_id: no tra loi cau "xa nao". '
    'Chi giu BAM SHA-256 cua ma Zalo, khong bao gio ma tho (Nghi dinh 13/2023).';
COMMENT ON COLUMN tai_khoan_zalo.bam_zalo_user_id IS
    'SHA-256 hex cua zalo_user_id. Ma tho la du lieu ca nhan: khong luu, khong log, khong vao vet.';
COMMENT ON COLUMN tai_khoan_zalo.cong_dan_id IS
    'NULL = chua xac thuc so qua tai khoan nay. Khac NULL = co da xac thuc so (ADR 0045). Doi so thi '
    'tro sang dinh danh cua so moi, KHONG gop hai dinh danh (ADR 0020 CON MO #1).';
COMMENT ON COLUMN tai_khoan_zalo.xa_da_nho IS
    'Xa cong dan da XAC NHAN o app chinh. Chi ghi khi co xac nhan; doc xong phai doi chieu so dang ky '
    'xa (con hoat dong) truoc khi dung — khong bao gio chep thang vao phien.';

-- ---------------------------------------------------------------------------
-- 2. phien_cong_dan — the account reference, and a citizen id that may be absent.
--
-- `tai_khoan_zalo_id` is NULL for sessions that did not come through the bridge (a paired screen,
-- ADR 0019). `cong_dan_id` becomes NULLABLE: a bridge session opened before the phone is verified.
--
-- A SESSION STILL ALWAYS NAMES SOMEBODY. The CHECK below refuses a row with neither: a session that
-- belongs to no identity and no account could not be revoked by either path, and nobody could say
-- whose it was.
--
-- WHAT A NULL cong_dan_id MAY DO is decided at the edge, not here: core/httpx.XaTuPhien refuses it
-- on every route by default; only a route declared XaTuPhienChiXem, with a reason, accepts it. The
-- pairing path keeps refusing it on its own (store/ghep_phien.go, Dung requires a citizen id), and
-- that is correct — a shared screen reads the citizen's own records.
-- ---------------------------------------------------------------------------
ALTER TABLE phien_cong_dan
    ADD COLUMN IF NOT EXISTS tai_khoan_zalo_id TEXT REFERENCES tai_khoan_zalo (id);

ALTER TABLE phien_cong_dan
    ALTER COLUMN cong_dan_id DROP NOT NULL;

DO $$ BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
         WHERE conname = 'phien_cong_dan_co_chu_the'
           AND conrelid = 'phien_cong_dan'::regclass
    ) THEN
        ALTER TABLE phien_cong_dan
            ADD CONSTRAINT phien_cong_dan_co_chu_the
            CHECK (cong_dan_id IS NOT NULL OR tai_khoan_zalo_id IS NOT NULL);
    END IF;
END $$;

COMMENT ON COLUMN phien_cong_dan.cong_dan_id IS
    'NULL = phien qua cau Mini App CHUA xac thuc so (ADR 0045). httpx.XaTuPhien tu choi phien nay tren '
    'moi tuyen; chi tuyen khai XaTuPhienChiXem kem ly do moi nhan.';
COMMENT ON COLUMN phien_cong_dan.tai_khoan_zalo_id IS
    'Tai khoan Zalo da mo phien qua cau (ADR 0045). NULL voi phien ghep man hinh (ADR 0019).';

-- Revoking every live session of ONE Zalo account — the "switching commune revokes the old session
-- immediately" answer (ADR 0045, owner's answer to CÒN MỞ #3). NOT prefixed with tenant_id, on
-- purpose: the old session is in the OLD commune, which is exactly the one the request is leaving.
-- The Go statement behind it carries its own `// @cross-tenant:` and is keyed on one account, never
-- on a commune (store/phien_cong_dan.go, ThuHoiCuaTaiKhoanZalo). Declared on the parent, so it is 32
-- child index probes, on a predicate that keeps only live bridge sessions in the index.
CREATE INDEX IF NOT EXISTS phien_cong_dan_theo_tai_khoan_zalo
    ON phien_cong_dan (tai_khoan_zalo_id)
    WHERE thu_hoi_luc IS NULL AND tai_khoan_zalo_id IS NOT NULL;

-- ---------------------------------------------------------------------------
-- REVERSAL (rule 7, invariant 4).
--
-- core/migrate has no automatic rollback, on purpose: undoing DDL on archival records is an
-- administrative act carried out with a person present. The reverse is written here, by hand, and
-- NONE OF IT IS A RUNNABLE LINE — a runnable line is a line that gets run.
--
-- THE CHECK AND THE INDEX hold no data and reverse losslessly: drop constraint
-- phien_cong_dan_co_chu_the, drop index phien_cong_dan_theo_tai_khoan_zalo.
--
-- THE NOT NULL ON phien_cong_dan.cong_dan_id can be restored (ALTER COLUMN cong_dan_id SET NOT NULL)
-- ONLY WHILE NO ROW HOLDS NULL — that is, before the first bridge session opened without a verified
-- phone. After that, restoring it means first REVOKING those sessions (never deleting the rows: they
-- are the record that the session existed), and setting NOT NULL still fails on them, because a
-- revoked row keeps its NULL. So after the first such session the honest reversal is "stop issuing
-- them" — at the edge, in app/cau_phien_cong_dan.go — not a schema change. Check first:
-- `SELECT count(*) FROM phien_cong_dan WHERE cong_dan_id IS NULL`.
--
-- THE COLUMN phien_cong_dan.tai_khoan_zalo_id AND THE TABLE tai_khoan_zalo are additive: "reverting"
-- them is achieved by ceasing to read and write them, at no cost. Actually removing either once it
-- holds rows destroys the only link between a session and the account that opened it, and the only
-- record of which commune each citizen confirmed — rule 7 stop condition #2: an explicit user
-- decision plus a verified backup. Removal, if ever decided, goes in dependency order: the column
-- first (it references the table), then the table.
--
-- AND AFTERWARDS, for the file to be applied again, its progress row has to be removed from
-- `schema_migration`, keyed on ten = '0011_tai_khoan_zalo_va_phien_chua_co_so.sql'. Prose, not a
-- runnable line, for the same reason.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- BACKSTOP, carried forward from 0002–0010: a partitioned table with no partitions rejects every
-- INSERT. This file adds one table and it is NOT partitioned (it has no tenant_id to partition by),
-- so it is expected to find nothing; it runs anyway, because the check only describes the state
-- after the newest migration that carries it.
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
