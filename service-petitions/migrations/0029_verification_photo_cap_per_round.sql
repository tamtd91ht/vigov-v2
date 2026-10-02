-- 0029 — the 5-photo cap on verification photos counts PER PROCESSING ROUND (menu phan-anh-nguoi-dan).
-- Schema only: no row is written.
--
-- WHY THIS FILE EXISTS. Owner decision 02/10/2026 (ADR 0047, row "Ảnh 'sau xử lý' của cán bộ — THAY G8",
-- item (e)): the cap of 5 verification photos ('petition-verification-photo', 0027) applies to EACH
-- processing round. A reopen (`mo-lai-theo-danh-gia`, ADR 0050 point 2) starts a new round allowed up to
-- 5 again; the earlier rounds' photos stay as evidence of what each round was closed on, and are never
-- deleted. 0027's trigger counts every stored photo of the petition, so after the first reopen of a
-- petition that already holds 5, no officer could ever add the photo the close gate now demands (app
-- XuLyPhanAnh.Dong requires one uploaded after the latest reopen) — the petition could never be closed.
--
-- WHAT CHANGES: ONLY the body of `stored_file_verification_photo_check()`. The trigger 0027 attached
-- (BEFORE INSERT OR UPDATE OF status ON stored_file) calls the function by name and is not touched.
--
-- THE ROUND BOUNDARY IS THE CLOSE GATE'S, TO THE LETTER, so the floor and the gate can never disagree on
-- what "this round" means:
--   * the round starts at the LATEST `nhat_ky_phan_anh.thoi_diem` with hanh_vi 'mo-lai-theo-danh-gia' for
--     this commune and this petition — store/petition_rating.go LatestReopenAtTx (max(thoi_diem), no
--     soft-delete predicate: the timeline is append-only and has none, 0013). Not `danh_gia_luc` on the
--     petition row: a later 3–5 star rating overwrites it (LatestReopenAtTx says why);
--   * a photo belongs to the round when its `created_at` is STRICTLY AFTER that instant —
--     store/stored_file.go countStoredCreatedAfterTail (`created_at > $5`). `created_at`, not
--     `updated_at`: the slot is issued when the officer picks the photo; `updated_at` moves on every
--     later transition. `created_at` is immutable (0021's stored_file_guard);
--   * never reopened → no bound → every stored photo counts, exactly as 0027.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. ROWS PER COMMUNE: zero are written and none is validated — CREATE OR REPLACE FUNCTION scans no
--      table. The function's new reading takes effect for the NEXT insert / `→ stored` transition only;
--      rows already stored are never re-checked. The NOTICE below measures, per commune, how many
--      petitions hold verification photos and how many have been reopened (counts and tenant ids only).
--   2. IF IT STOPS HALF-WAY: it cannot land half-applied — one file, one transaction (core/migrate), with
--      its progress row. CREATE OR REPLACE is idempotent, so a retry costs nothing. No per-commune loop:
--      nothing to backfill.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. READ PATHS THAT CHANGE MEANING WHILE HALF-APPLIED: none can see it half-applied (one transaction).
--      AFTER it, the floor is LOOSER on a reopened petition: it may hold more than 5 verification photos
--      in total (at most 5 per round). Who reads them:
--        * app XuLyPhanAnh.Dong (close gate) — counts ≥ 1 after the latest reopen; unaffected in meaning;
--        * store/petition_photo.go PetitionPhotos (purpose = 'petition-verification-photo' AND
--          uploaded_by <> 'cong-dan') — returns every round's photos, now possibly > 5. A screen that
--          assumed "at most 5" must page or group by round; the list is not truncated by this file;
--        * app petition_verification_photo.go issue (:232) and completion (:371) counts — STILL COUNT
--          EVERY ROUND (CountForSubjectTx, no created-after bound), as does platform's upload_policy
--          max_files_per_subject. Until those are narrowed (a later card, not migration territory), the
--          app refuses the 6th photo of a reopened petition BEFORE this trigger ever sees it; this file
--          only stops the DATABASE from being the reason it is refused.
--   5. RETENTION: unchanged. Verification photos are `records` (0027): retain_until NULL, never purged
--      by a command, soft delete only (0021's guard refuses DELETE). Earlier rounds' photos are KEPT —
--      this file narrows what is COUNTED, never what is kept.
--
-- KNOWN EDGE, stated rather than hidden: a slot issued BEFORE a reopen but completed (scanning → stored)
-- AFTER it has `created_at` before the round start, so it is counted in no round — not in the new one,
-- and the old round is no longer measured. An old round can therefore end with more than 5 stored
-- photos if uploads straddle a reopen. The close gate ignores such a photo too (same predicate), so
-- the two stay consistent; bounding it would need a per-round marker on the row that does not exist.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- stored_file_verification_photo_check — AT MOST 5 VERIFICATION PHOTOS per petition PER ROUND, per
-- commune, that REACHED THE DESTINATION (not deleted, status `stored` / `processing` / `ready`), counted
-- when a row ENTERS that set. Everything not about the round is 0027's, unchanged: the early returns,
-- the petition row locked FOR UPDATE FIRST (it serialises two completions on one petition AND a
-- completion against a reopen, which updates the same row), and the message naming no id (rule 3).
--
-- The reopen lookup is served by 0013's `nhat_ky_theo_phieu_phan_anh` (tenant_id, phieu_phan_anh_id,
-- thoi_diem DESC, id DESC): the equality prefix narrows to this petition's timeline; `hanh_vi` is a
-- filter over those few rows. The tenant predicate prunes to one hash partition.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION stored_file_verification_photo_check() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
    live_photos INT;
    round_start TIMESTAMPTZ;
BEGIN
    IF NEW.purpose <> 'petition-verification-photo'
       OR NEW.deleted_at IS NOT NULL
       OR NEW.status NOT IN ('stored', 'processing', 'ready') THEN
        RETURN NEW;
    END IF;

    -- Only what changes the answer re-runs the check: a new row, or a row entering the stored set.
    IF TG_OP = 'UPDATE' AND OLD.status IN ('stored', 'processing', 'ready') THEN
        RETURN NEW;
    END IF;

    PERFORM 1
       FROM phieu_phan_anh p
      WHERE p.tenant_id = NEW.tenant_id AND p.id = NEW.subject_id AND p.deleted_at IS NULL
        FOR UPDATE;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'stored_file: petition not found in this commune';
    END IF;

    -- The latest reopen of this petition in this commune (LatestReopenAtTx). NULL = never reopened.
    SELECT max(l.thoi_diem) INTO round_start
      FROM nhat_ky_phan_anh l
     WHERE l.tenant_id = NEW.tenant_id
       AND l.phieu_phan_anh_id = NEW.subject_id
       AND l.hanh_vi = 'mo-lai-theo-danh-gia';

    SELECT count(*) INTO live_photos
      FROM stored_file f
     WHERE f.tenant_id = NEW.tenant_id
       AND f.subject_type = 'petition' AND f.subject_id = NEW.subject_id
       AND f.purpose = 'petition-verification-photo'
       AND f.deleted_at IS NULL
       AND f.status IN ('stored', 'processing', 'ready')
       AND f.id <> NEW.id
       AND (round_start IS NULL OR f.created_at > round_start);
    IF live_photos >= 5 THEN
        RAISE EXCEPTION 'stored_file: a petition holds at most 5 verification photos per processing round'
            USING HINT = 'Set by precedent 02/10/2026, per round by owner decision 02/10/2026 (ADR 0047 (e)); '
                         'a reopen starts a new round. Platform upload_policy petition-verification-photo.';
    END IF;

    RETURN NEW;
END $$;

-- ---------------------------------------------------------------------------
-- MEASUREMENT (question 1): per commune, petitions holding verification photos, and reopened petitions.
-- Counts and tenant ids only.
-- ---------------------------------------------------------------------------
DO $$
DECLARE r record;
BEGIN
    FOR r IN
        SELECT f.tenant_id, count(DISTINCT f.subject_id) AS petitions, count(*) AS photos
        FROM stored_file f
        WHERE f.purpose = 'petition-verification-photo'
        GROUP BY f.tenant_id
        ORDER BY f.tenant_id
    LOOP
        RAISE NOTICE '0029: commune % holds % verification photo row(s) on % petition(s)',
            r.tenant_id, r.photos, r.petitions;
    END LOOP;
    FOR r IN
        SELECT l.tenant_id, count(DISTINCT l.phieu_phan_anh_id) AS petitions
        FROM nhat_ky_phan_anh l
        WHERE l.hanh_vi = 'mo-lai-theo-danh-gia'
        GROUP BY l.tenant_id
        ORDER BY l.tenant_id
    LOOP
        RAISE NOTICE '0029: commune % has % reopened petition(s)', r.tenant_id, r.petitions;
    END LOOP;
END $$;

-- ---------------------------------------------------------------------------
-- REVERSAL (rule 7, invariant 4). Run by a person, in ONE transaction; core/migrate has no automatic
-- rollback (ADR 0013). Re-run 0027's CREATE OR REPLACE FUNCTION stored_file_verification_photo_check()
-- verbatim (the whole-petition count), then remove this file's row from `schema_migration`.
-- LOSSLESS AT ANY TIME: it writes and deletes no row, and rows already stored are never re-checked —
-- a petition already holding more than 5 across rounds keeps every photo. Its cost: from then on no
-- further verification photo can be stored on such a petition, so a reopened petition holding 5 can no
-- longer satisfy the close gate. Reverting therefore re-opens the owner decision of 02/10/2026; it is
-- the user's call, not a command's.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- BACKSTOP: every partitioned table in this schema must actually have partitions (0002, §BACKSTOP).
-- This file declares no table; repeated so every file in this run ends on the same guarantee.
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
