-- 0012 — the LIFECYCLE of meeting minutes (draft -> signed) and the fields the user added on
-- 25/09/2026: secretary, the conclusion notice (Thông báo kết luận), supplementary minutes, and the
-- "no task arises" mark on a conclusion.
--
-- THE DECISIONS THIS FILE STORES (user, 25/09/2026 — ledger service-petitions/bien-ban-hop-tang-
-- du-lieu; reconciliation note kb/50-doi-chieu/2026-09-25-feat-m8-multitenant-foundation-bien-ban-
-- hop.md, rows #1-#3):
--
--   (1) du-thao -> da-ky. A draft may be edited and soft deleted, conclusions included. Once SIGNED,
--       content, conclusions, chair and secretary are LOCKED by a trigger. A mistake found after
--       signing is corrected by SUPPLEMENTARY minutes pointing at the original; the original does
--       not change by one word. Supplementary minutes number their own conclusions from 1.
--   (2) Secretary (staff code) + number/date of the conclusion notice — optional, TRANSCRIBED, never
--       minted here (the office clerk issues it under Decree 30/2020, Art. 15).
--   (3) A conclusion that already has tasks is locked even in draft — APP-ENFORCED, see below.
--   (4) "không phát sinh nhiệm vụ": set only while the conclusion has no task, counts as done,
--       removable while draft, locked after signing, audited.
--
-- THIS ANSWERS THE STOP CONDITION 0007:81-89 LEFT OPEN ("may a conclusion be edited once tasks were
-- split from it"). 0007 promised the answer would be "one trigger in a LATER migration"; this is it.
--
-- WHY A NEW FILE AND NOT AN EDIT OF 0007: core/migrate compares the checksum of every applied file at
-- startup (ErrChecksumLech). Editing 0007 stops the service, or leaves two databases claiming one
-- schema version while holding two schemas.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS, answered before the SQL.
--
--   1. HOW MANY ROWS PER COMMUNE: this file WRITES no row itself. Two columns carry a DEFAULT
--      (`trang_thai`, `khong_phat_sinh`); on PostgreSQL 11+ a constant default is stored in the
--      catalogue, so existing rows are NOT rewritten and read back the default. Every other column
--      is NULLABLE. Rows that may exist: the write route exists since 2c8cd78 (POST /meetings,
--      /conclusions), so any environment where staff typed minutes has rows; no commune is
--      onboarded in production today.
--   2. IF IT STOPS HALF-WAY: it cannot land half-applied. One file, one transaction
--      (core/migrate/migrate.go:192), the progress row written inside it. Every statement is
--      ADD COLUMN IF NOT EXISTS / guarded ADD CONSTRAINT / CREATE OR REPLACE / DROP TRIGGER IF
--      EXISTS / CREATE INDEX IF NOT EXISTS, so a retry costs nothing.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. WHICH READ PATHS CHANGE MEANING WHILE IT IS HALF-APPLIED: none while it runs (one
--      transaction; readers see before or after). AFTER it lands:
--        * every existing minutes row reads as `du-thao` — THE LOSSLESS CHOICE, and the only honest
--          one: no signing act has ever existed in this system, so no row was ever signed HERE.
--          Stamping existing rows `da-ky` would invent a signer and an instant nobody recorded, and
--          would lock rows a clerk may still need to correct. A paper copy may well have been signed
--          — the commune signs it here, as a recorded act, and that act is what the lock rests on.
--        * every existing conclusion reads `khong_phat_sinh = false` — true: nobody ever set it.
--        * the store's column lists are explicit (internal/store/bien_ban_hop*.go), so no reader
--          picks up the new columns by accident, and the only write paths today are the two INSERTs
--          (store/bien_ban_hop_ghi.go:172,203). Both keep working: the first gets `du-thao` by
--          default, the second targets a parent that is necessarily `du-thao` (nothing signs yet).
--        * the new index changes which PLAN is picked, never which rows come back.
--   5. RETENTION: minutes and conclusions are ARCHIVAL RECORDS (rule 7). Nothing is dropped or
--      retyped. The whole point of this file is to make a signed record immutable in the database,
--      not by convention (rule 7, forbidden #5).
--
-- ---------------------------------------------------------------------------
-- WHAT IS APP-ENFORCED, NOT HERE, AND WHY:
--
--   * "A conclusion with a live task is locked even in draft" (decision 3), and "khong_phat_sinh may
--     only be set while the conclusion has no live task" (decision 4). Both need a read of
--     `nhiem_vu` by the blurred pair (`nguon_giao = 'ket-luan-hop' AND nguon_id = ket_luan_hop.id`),
--     which has NO foreign key and cannot have one (0007:72-76). A trigger doing that cross-table
--     read would also have to guard the OTHER direction (a task created against a marked or
--     locked conclusion) with a trigger on `nhiem_vu` — a second table's write path, whose rules
--     (0006) are not this file's to change. The use case does it, inside the transaction that
--     writes the audit entry.
--   * "khong_phat_sinh may be cleared only while draft" IS enforced here (the conclusion trigger
--     freezes every column once the parent is signed); "set/clear leaves a trail" is rule 6 in the
--     use case.
--   * The ORIGINAL a supplementary minutes points at should be signed (a draft is corrected by
--     editing it). Not a trigger: the use case checks it with the same row it already reads for the
--     tenant check. The FK below guarantees only that the original exists in the SAME commune.
--
-- WHAT THIS FILE DELIBERATELY DOES NOT DO:
--
--   * No sequence, no unique key on `tb_so_ky_hieu`. It is transcribed from a notice the office
--     clerk numbered; this system does not mint it, and like `so_hieu` (0007:91-96) the number
--     restarts every year, so a unique key would refuse real data.
--   * No counter or status column on `ket_luan_hop` for "chưa giao / đang thực hiện / quá hạn /
--     hoàn thành": decision (4) says DERIVED from tasks (rule 10, invariant 3's reasoning). Only the
--     human-set mark is stored, because nothing can derive a person's judgement.
--   * No drop of `bien_ban_hop_so` (0007): the store still pages on `tao_luc` until the read path
--     switches to the new index. Dropping an index a live query uses is a performance incident.

-- ---------------------------------------------------------------------------
-- PostgreSQL 13 is the floor — BEFORE ... FOR EACH ROW triggers on a PARTITIONED table (13+), and a
-- foreign key REFERENCING a partitioned table (12+). On older servers the statements below fail with
-- messages that read like syntax mistakes and invite moving triggers onto partitions, where a
-- partition added later arrives silently unprotected.
-- ---------------------------------------------------------------------------
DO $$ BEGIN
    IF current_setting('server_version_num')::int < 130000 THEN
        RAISE EXCEPTION
            'bien_ban_hop lifecycle needs PostgreSQL 13 or newer (server is %). Do not weaken this '
            'migration to fit an older server: the triggers below are what keep signed minutes '
            'from being rewritten.',
            current_setting('server_version');
    END IF;
END $$;

-- ---------------------------------------------------------------------------
-- bien_ban_hop — THE NEW COLUMNS.
-- ---------------------------------------------------------------------------

-- The lifecycle. DEFAULT 'du-thao' is what makes the add lossless (question 4) AND what the create
-- route needs: new minutes are always drafts; signing is a separate, audited act.
ALTER TABLE bien_ban_hop ADD COLUMN IF NOT EXISTS trang_thai TEXT NOT NULL DEFAULT 'du-thao';

-- The signing act: WHEN, and WHO as a STAFF BUSINESS CODE (rule 6, invariant 8 — never
-- Principal.ID). Both NULL on a draft, both set on signed minutes (CHECK below).
ALTER TABLE bien_ban_hop ADD COLUMN IF NOT EXISTS ky_luc TIMESTAMPTZ;
ALTER TABLE bien_ban_hop ADD COLUMN IF NOT EXISTS ky_boi_ma TEXT;

-- The secretary, as a STAFF BUSINESS CODE — same shape and same cap as `chu_tri_ma`
-- (domain.ChuTriMaToiDa = 32). Optional: small meetings have no named secretary.
ALTER TABLE bien_ban_hop ADD COLUMN IF NOT EXISTS thu_ky_ma TEXT;

-- The conclusion notice (Thông báo kết luận): reference number + date, TRANSCRIBED. Both or none.
-- DATE for the same reason as `ngay_hop` (0007:180-193): a calendar fact on a paper document.
ALTER TABLE bien_ban_hop ADD COLUMN IF NOT EXISTS tb_so_ky_hieu TEXT;
ALTER TABLE bien_ban_hop ADD COLUMN IF NOT EXISTS tb_ngay DATE;

-- Supplementary minutes: the original they correct. NULL on ordinary minutes.
ALTER TABLE bien_ban_hop ADD COLUMN IF NOT EXISTS bo_sung_cho_id TEXT;

-- ---------------------------------------------------------------------------
-- ket_luan_hop — THE "NO TASK ARISES" MARK.
--
-- BOOLEAN NOT NULL DEFAULT false: one spelling for "not marked" (a NULL would be a third state that
-- every counting query would have to remember). Who and when travel with it, as in every
-- human-set fact on an archival table here.
-- ---------------------------------------------------------------------------
ALTER TABLE ket_luan_hop ADD COLUMN IF NOT EXISTS khong_phat_sinh BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE ket_luan_hop ADD COLUMN IF NOT EXISTS khong_phat_sinh_luc TIMESTAMPTZ;
ALTER TABLE ket_luan_hop ADD COLUMN IF NOT EXISTS khong_phat_sinh_boi_ma TEXT;

-- ---------------------------------------------------------------------------
-- THE CONSTRAINTS. DO $$ ... $$ because ADD CONSTRAINT has no IF NOT EXISTS and a retry must cost
-- nothing. Lookup by name, same pattern as 0011.
--
-- Every arm is written so it evaluates to TRUE or FALSE, never NULL — a CHECK treats NULL as passed,
-- so `btrim(x) <> ''` alone would accept an absent value.
--
-- Every CHECK is validated against existing rows by ADD CONSTRAINT: all of them pass by
-- construction (new columns are NULL / default), and if that reasoning is ever wrong the file FAILS
-- and rolls back instead of lying.
-- ---------------------------------------------------------------------------
DO $$ BEGIN
    -- The closed list of statuses. migrations/bien_ban_vong_doi_test.go parses it and compares it
    -- with the domain constants.
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'bien_ban_hop_trang_thai_hop_le') THEN
        ALTER TABLE bien_ban_hop ADD CONSTRAINT bien_ban_hop_trang_thai_hop_le
            CHECK (trang_thai IN ('du-thao', 'da-ky'));
    END IF;

    -- da-ky <=> signer and instant both set; du-thao => both NULL. The ELSE arm is what keeps a
    -- stray signer off a draft — a draft carrying `ky_boi_ma` reads as signed to one reader and not
    -- to another.
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'bien_ban_hop_ky_du_truong') THEN
        ALTER TABLE bien_ban_hop ADD CONSTRAINT bien_ban_hop_ky_du_truong CHECK (
            CASE trang_thai
                WHEN 'da-ky' THEN
                    ky_luc IS NOT NULL AND ky_boi_ma IS NOT NULL AND btrim(ky_boi_ma) <> ''
                ELSE
                    ky_luc IS NULL AND ky_boi_ma IS NULL
            END);
    END IF;

    -- STAFF CODES: bounded in CHARACTERS, 32 = domain.ChuTriMaToiDa. NULL passes; blank does not
    -- (an empty string is not "no secretary", it is a broken reference).
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'bien_ban_hop_ky_boi_ma_toi_da') THEN
        ALTER TABLE bien_ban_hop ADD CONSTRAINT bien_ban_hop_ky_boi_ma_toi_da
            CHECK (char_length(ky_boi_ma) <= 32);
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'bien_ban_hop_thu_ky_ma_hop_le') THEN
        ALTER TABLE bien_ban_hop ADD CONSTRAINT bien_ban_hop_thu_ky_ma_hop_le
            CHECK (thu_ky_ma IS NULL OR (btrim(thu_ky_ma) <> '' AND char_length(thu_ky_ma) <= 32));
    END IF;

    -- The conclusion notice: both or none, number non-blank, bounded like `so_hieu`
    -- (domain.SoHieuBienBanToiDa = 64). Half a reference ("số 12" with no date, or a date with no
    -- number) is a citation nobody can find in the office's register.
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'bien_ban_hop_thong_bao_du_truong') THEN
        ALTER TABLE bien_ban_hop ADD CONSTRAINT bien_ban_hop_thong_bao_du_truong CHECK (
            (tb_so_ky_hieu IS NULL AND tb_ngay IS NULL)
            OR (tb_so_ky_hieu IS NOT NULL AND btrim(tb_so_ky_hieu) <> '' AND tb_ngay IS NOT NULL));
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'bien_ban_hop_tb_so_ky_hieu_toi_da') THEN
        ALTER TABLE bien_ban_hop ADD CONSTRAINT bien_ban_hop_tb_so_ky_hieu_toi_da
            CHECK (char_length(tb_so_ky_hieu) <= 64);
    END IF;

    -- Supplementary minutes never correct themselves, and '' is a broken reference, not "none".
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'bien_ban_hop_bo_sung_khong_tu_tro') THEN
        ALTER TABLE bien_ban_hop ADD CONSTRAINT bien_ban_hop_bo_sung_khong_tu_tro
            CHECK (bo_sung_cho_id IS NULL OR (btrim(bo_sung_cho_id) <> '' AND bo_sung_cho_id <> id));
    END IF;

    -- THE ORIGINAL IS A REAL ROW IN THE SAME COMMUNE. A composite self-referencing foreign key, the
    -- same shape as ket_luan_hop -> bien_ban_hop (0007:321): the referenced pair is the partitioned
    -- parent's primary key, which PostgreSQL 12+ accepts. `tenant_id` on both sides means a
    -- supplement cannot point at another commune's minutes even if two ids ever collided (rule 1).
    -- MATCH SIMPLE (the default): a NULL `bo_sung_cho_id` is not checked, so ordinary minutes pass.
    -- Validation over existing rows is trivial — every one is NULL.
    -- The original can never vanish from under a supplement: hard DELETE is refused (0007) and a
    -- SIGNED original cannot be soft deleted (trigger below).
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'bien_ban_hop_bo_sung_cho_fk') THEN
        ALTER TABLE bien_ban_hop ADD CONSTRAINT bien_ban_hop_bo_sung_cho_fk
            FOREIGN KEY (tenant_id, bo_sung_cho_id) REFERENCES bien_ban_hop (tenant_id, id);
    END IF;

    -- The mark and its who/when travel together: set -> both present; cleared -> both NULL. So
    -- clearing the mark in draft clears its who/when too (the trail of both acts lives in the audit
    -- ledger, rule 6), and a stale `khong_phat_sinh_boi_ma` can never sit on an unmarked row.
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'ket_luan_hop_khong_phat_sinh_du_truong') THEN
        ALTER TABLE ket_luan_hop ADD CONSTRAINT ket_luan_hop_khong_phat_sinh_du_truong CHECK (
            (khong_phat_sinh AND khong_phat_sinh_luc IS NOT NULL
                AND khong_phat_sinh_boi_ma IS NOT NULL AND btrim(khong_phat_sinh_boi_ma) <> '')
            OR (NOT khong_phat_sinh AND khong_phat_sinh_luc IS NULL
                AND khong_phat_sinh_boi_ma IS NULL));
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'ket_luan_hop_khong_phat_sinh_boi_ma_toi_da') THEN
        ALTER TABLE ket_luan_hop ADD CONSTRAINT ket_luan_hop_khong_phat_sinh_boi_ma_toi_da
            CHECK (char_length(khong_phat_sinh_boi_ma) <= 32);
    END IF;
END $$;

-- ---------------------------------------------------------------------------
-- bien_ban_hop_da_ky_bat_bien — signed minutes never change.
--
-- DENY BY DEFAULT, the pattern of service-finance 0008 `dot_thu_chi_bat_bien`: the WHOLE ROW is
-- compared minus an explicit list of what may still move, so a column a later migration adds is
-- frozen on signed minutes the day it exists, with nobody having to remember this function.
--
-- ONLY WHEN OLD IS SIGNED. The signing UPDATE itself (du-thao -> da-ky) passes, and may carry last
-- edits in the same statement — it is still a draft until that statement commits.
--
-- WHAT MAY STILL MOVE AFTER SIGNING:
--   * `tb_so_ky_hieu` / `tb_ngay`, ONCE (NULL -> value). The conclusion notice is routinely issued
--     AFTER the minutes are signed: the minutes record the meeting, the notice is a separate
--     document the office drafts from them, gets signed by the chair and numbers through the
--     clerk's register — often days later. Refusing it would force the notice's reference to be
--     typed before it exists. Once set it is frozen like the rest: a correction is an act on
--     paper, not an edit here.
--   * `cap_nhat_luc` — bookkeeping of the row, not content; the write that records the notice
--     bumps it.
--
-- WHAT IT REFUSES ON SIGNED MINUTES: everything else — content, title, day, reference, place, chair,
-- secretary, attendees, attachments, going back to `du-thao`, signer and instant, the supplementary
-- link — AND THE SOFT-DELETE TRIO: a signed record is not removed by a command; a mistake is
-- answered by supplementary minutes (decision 1).
--
-- ⚠ `dinh_kem` (the scan of the signed paper, 0007:220-224) IS FROZEN TOO, by default. Nothing
-- writes it today (no file store). If the product wants the scan attached AFTER signing — plausible,
-- since it is a scan OF the signed paper — that is a decision for the user and one more allowed
-- column in a later migration; this file does not decide it by leaving a hole.
--
-- Messages name the relation only. `noi_dung` can quote a case and an error message travels into
-- logs and back to clients (rule 3, forbidden #3), so no VALUE is interpolated.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION bien_ban_hop_da_ky_bat_bien() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.trang_thai = 'da-ky' THEN
        IF (to_jsonb(NEW) - 'tb_so_ky_hieu' - 'tb_ngay' - 'cap_nhat_luc')
           IS DISTINCT FROM
           (to_jsonb(OLD) - 'tb_so_ky_hieu' - 'tb_ngay' - 'cap_nhat_luc') THEN
            RAISE EXCEPTION 'archival record %: signed minutes cannot be edited or removed', TG_TABLE_NAME
                USING HINT = 'Signed minutes are locked (user decision 25/09/2026). Record a '
                             'correction as supplementary minutes pointing at this one '
                             '(bo_sung_cho_id); the original does not change.';
        END IF;

        IF (OLD.tb_so_ky_hieu IS NOT NULL OR OLD.tb_ngay IS NOT NULL)
           AND (NEW.tb_so_ky_hieu IS DISTINCT FROM OLD.tb_so_ky_hieu
                OR NEW.tb_ngay IS DISTINCT FROM OLD.tb_ngay) THEN
            RAISE EXCEPTION 'archival record %: the conclusion notice reference of signed minutes is recorded once',
                TG_TABLE_NAME
                USING HINT = 'The notice number and date may be added once after signing, because '
                             'the notice is often issued later. Once recorded they are frozen.';
        END IF;
    END IF;

    RETURN NEW;
END $$;

DROP TRIGGER IF EXISTS bien_ban_hop_da_ky_bat_bien ON bien_ban_hop;
CREATE TRIGGER bien_ban_hop_da_ky_bat_bien
    BEFORE UPDATE ON bien_ban_hop
    FOR EACH ROW EXECUTE FUNCTION bien_ban_hop_da_ky_bat_bien();

-- ---------------------------------------------------------------------------
-- ket_luan_hop_da_ky_bat_bien — the conclusions of signed minutes never change, and signed minutes
-- get no new conclusions.
--
-- THE PARENT ROW IS READ IN THE SAME TENANT (`tenant_id = NEW/OLD.tenant_id`), so the lookup is a
-- primary-key probe into one hash partition and can never read another commune's minutes.
--
-- `FOR SHARE`, AND WITHOUT IT THE LOCK HAS A HOLE. Under READ COMMITTED, a conclusion edit could read
-- the parent as `du-thao` while a concurrent transaction is signing it, and both would commit: an
-- edited conclusion under minutes that say signed. FOR SHARE conflicts with the signing UPDATE's
-- row lock (FOR NO KEY UPDATE) — the foreign key's own FOR KEY SHARE does NOT — so one of the two
-- waits, and a waiter re-reads the committed row: the edit then sees `da-ky` and is refused, or the
-- signing waits until the edit has committed.
--
-- WHOLE ROW MINUS `cap_nhat_luc`, the same deny-by-default shape: text, ordinal, the no-task mark
-- and its who/when, the soft-delete trio, and any column added later.
--
-- INSERT IS REFUSED TOO. Adding a conclusion to signed minutes changes the signed record as surely
-- as editing one ("the original does not change by one word", decision 1). The route adds
-- conclusions before signing, or to the supplementary minutes.
--
-- A PARENT NOT FOUND is left to the foreign key (0007:321), which reports it properly.
--
-- NOT IN THIS TRIGGER — "a conclusion with live tasks is locked even in draft" (decision 3): that is
-- a read of `nhiem_vu`, app-enforced; see the header.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION ket_luan_hop_da_ky_bat_bien() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
    tt_cu  text;
    tt_moi text;
BEGIN
    IF TG_OP = 'INSERT' THEN
        SELECT trang_thai INTO tt_moi FROM bien_ban_hop
         WHERE tenant_id = NEW.tenant_id AND id = NEW.bien_ban_id
           FOR SHARE;
        IF tt_moi = 'da-ky' THEN
            RAISE EXCEPTION 'archival record %: signed minutes take no new conclusion', TG_TABLE_NAME
                USING HINT = 'Signed minutes are locked (user decision 25/09/2026). Add the '
                             'conclusion to supplementary minutes pointing at the original.';
        END IF;
        RETURN NEW;
    END IF;

    SELECT trang_thai INTO tt_cu FROM bien_ban_hop
     WHERE tenant_id = OLD.tenant_id AND id = OLD.bien_ban_id
       FOR SHARE;
    IF tt_cu = 'da-ky'
       AND (to_jsonb(NEW) - 'cap_nhat_luc') IS DISTINCT FROM (to_jsonb(OLD) - 'cap_nhat_luc') THEN
        RAISE EXCEPTION 'archival record %: a conclusion of signed minutes cannot be edited or removed',
            TG_TABLE_NAME
            USING HINT = 'Signed minutes are locked with their conclusions (user decision '
                         '25/09/2026). Record the correction as supplementary minutes.';
    END IF;

    -- Moving a conclusion INTO signed minutes is adding one to them.
    IF NEW.tenant_id IS DISTINCT FROM OLD.tenant_id OR NEW.bien_ban_id IS DISTINCT FROM OLD.bien_ban_id THEN
        SELECT trang_thai INTO tt_moi FROM bien_ban_hop
         WHERE tenant_id = NEW.tenant_id AND id = NEW.bien_ban_id
           FOR SHARE;
        IF tt_moi = 'da-ky' THEN
            RAISE EXCEPTION 'archival record %: signed minutes take no new conclusion', TG_TABLE_NAME
                USING HINT = 'Signed minutes are locked (user decision 25/09/2026).';
        END IF;
    END IF;

    RETURN NEW;
END $$;

DROP TRIGGER IF EXISTS ket_luan_hop_da_ky_bat_bien ON ket_luan_hop;
CREATE TRIGGER ket_luan_hop_da_ky_bat_bien
    BEFORE INSERT OR UPDATE ON ket_luan_hop
    FOR EACH ROW EXECUTE FUNCTION ket_luan_hop_da_ky_bat_bien();

-- ---------------------------------------------------------------------------
-- THE ORDER THE REGISTER IS READ IN (decision 5): meeting day, newest first; same day by entry time;
-- `id` as the tie-break core/store.QueryPage pages on. Soft-deleted rows excluded (rule 7,
-- invariant 2). Starts with `tenant_id`, like every index in this system.
--
-- PLAIN `CREATE INDEX` ON THE PARTITIONED PARENT, the convention of 0007:261 and every migration in
-- this repository: CONCURRENTLY is refused on a partitioned parent, and core/migrate runs each file
-- in one transaction where CONCURRENTLY is refused anyway (core/migrate/migrate.go:47). The parent
-- index cascades to all 32 partitions and to every partition added later. Cost: each partition is
-- SHARE-locked (writes wait, reads do not) for the length of its build — milliseconds on registers
-- of this size.
--
-- ⚠ THE DATE-CURSOR CAVEAT OF 0007:257-260 STILL APPLIES to whoever switches the read path to this
-- index: a DATE compared against a bound `time.Time` is cast through the session time zone. The
-- cursor must carry `ngay_hop` as a DATE-shaped value (a 'YYYY-MM-DD' text bound as ::date), not as
-- an instant.
-- ---------------------------------------------------------------------------
CREATE INDEX IF NOT EXISTS bien_ban_hop_theo_ngay_hop
    ON bien_ban_hop (tenant_id, ngay_hop DESC, tao_luc DESC, id) WHERE deleted_at IS NULL;

-- ---------------------------------------------------------------------------
-- WHAT THIS FILE DOES NOT STOP, said plainly: TRUNCATE, and DDL by the table owner (ALTER TABLE ...
-- DISABLE TRIGGER, dropping a constraint). Same line ADR 0013 draws for the audit ledger.
--
-- REVERSAL (migration question 3). In ONE transaction, in this order:
--
--   DROP INDEX bien_ban_hop_theo_ngay_hop;
--   DROP TRIGGER ket_luan_hop_da_ky_bat_bien ON ket_luan_hop;
--   DROP FUNCTION ket_luan_hop_da_ky_bat_bien();
--   DROP TRIGGER bien_ban_hop_da_ky_bat_bien ON bien_ban_hop;
--   DROP FUNCTION bien_ban_hop_da_ky_bat_bien();
--   ALTER TABLE ket_luan_hop DROP CONSTRAINT ket_luan_hop_khong_phat_sinh_boi_ma_toi_da;
--   ALTER TABLE ket_luan_hop DROP CONSTRAINT ket_luan_hop_khong_phat_sinh_du_truong;
--   ALTER TABLE bien_ban_hop DROP CONSTRAINT bien_ban_hop_bo_sung_cho_fk;
--   ALTER TABLE bien_ban_hop DROP CONSTRAINT bien_ban_hop_bo_sung_khong_tu_tro;
--   ALTER TABLE bien_ban_hop DROP CONSTRAINT bien_ban_hop_tb_so_ky_hieu_toi_da;
--   ALTER TABLE bien_ban_hop DROP CONSTRAINT bien_ban_hop_thong_bao_du_truong;
--   ALTER TABLE bien_ban_hop DROP CONSTRAINT bien_ban_hop_thu_ky_ma_hop_le;
--   ALTER TABLE bien_ban_hop DROP CONSTRAINT bien_ban_hop_ky_boi_ma_toi_da;
--   ALTER TABLE bien_ban_hop DROP CONSTRAINT bien_ban_hop_ky_du_truong;
--   ALTER TABLE bien_ban_hop DROP CONSTRAINT bien_ban_hop_trang_thai_hop_le;
--   -- columns: ONLY after the emptiness check below
--   ALTER TABLE ket_luan_hop DROP COLUMN khong_phat_sinh_boi_ma;
--   ALTER TABLE ket_luan_hop DROP COLUMN khong_phat_sinh_luc;
--   ALTER TABLE ket_luan_hop DROP COLUMN khong_phat_sinh;
--   ALTER TABLE bien_ban_hop DROP COLUMN bo_sung_cho_id;
--   ALTER TABLE bien_ban_hop DROP COLUMN tb_ngay;
--   ALTER TABLE bien_ban_hop DROP COLUMN tb_so_ky_hieu;
--   ALTER TABLE bien_ban_hop DROP COLUMN thu_ky_ma;
--   ALTER TABLE bien_ban_hop DROP COLUMN ky_boi_ma;
--   ALTER TABLE bien_ban_hop DROP COLUMN ky_luc;
--   ALTER TABLE bien_ban_hop DROP COLUMN trang_thai;
--
-- and remove this file's row from `schema_migration` in the same transaction. Nothing of 0007 is
-- altered here, so nothing there has to be put back.
--
-- DROPPING THE INDEX, TRIGGERS AND CONSTRAINTS IS LOSSLESS AT ANY TIME. DROPPING THE COLUMNS IS
-- LOSSLESS ONLY WHILE THEY HOLD NOTHING — check, per commune, that zero rows have
-- `trang_thai = 'da-ky'`, `thu_ky_ma`, `tb_so_ky_hieu`, `bo_sung_cho_id` set, or
-- `khong_phat_sinh = true`. ONCE ONE MINUTES HAS BEEN SIGNED, the signer, the instant and the
-- status are part of an archival record, and dropping them is destroying it — rule 7's second stop
-- condition, which needs the user and a verified backup, not a command. From that point the way
-- back is a NEW migration (core/migrate has no automatic rollback — ADR 0013).
-- ---------------------------------------------------------------------------
