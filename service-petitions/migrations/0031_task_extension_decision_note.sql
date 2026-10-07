-- 0031 — an extension request carries the decider's note (`decision_note`) on its own row.
-- Schema only: no row is written.
--
-- WHY THIS FILE EXISTS. User decision 07/10/2026: the extension history must show the note the leader
-- wrote when approving or rejecting. Until now that note was stored NOWHERE on the request: the decision
-- route accepts it (internal/http/nhiem_vu_ghi.go:307-312, body field `note`) and the use case writes it
-- only as the TEXT of a task-log entry (internal/app/nhiem_vu.go:1785-1794), where it is indistinguishable
-- from any other progress remark and cannot be joined back to the request it answered. Writing it is the
-- next card; this file only gives it a place.
--
-- WHY A NEW FILE AND NOT AN EDIT OF 0006: core/migrate compares the checksum of every applied file at
-- startup. Editing an applied file either stops the service or leaves two databases claiming one schema
-- version while holding two schemas.
--
-- ---------------------------------------------------------------------------
-- THE NAME: `decision_note`. English (rule 12, ADR 0051). Reasons, in order:
--
--   * kb/00-foundation/ubiquitous-language.md has NO term for a note/remark in English, so there is no
--     existing word to follow and none a second word would duplicate (rule 12, forbidden #2).
--   * It follows the contract the column serves: the act is the sub-resource
--     `…/extensions/{id}/decision` (ubiquitous-language row "Đề nghị lùi hạn nhiệm vụ") and its body
--     field is `note`. `decision` + `note` is the wire's own vocabulary.
--   * It names WHOSE text it is. This row already holds one free text, `ly_do`, written by the
--     REQUESTER; a bare `note` would let a reader take it for the requester's addendum.
--   * Neighbour precedent for an English column added to a Vietnamese-named table: 0017's
--     `rating_comment` on `phieu_phan_anh`. Existing columns of this table are not renamed (rule 12,
--     invariant 3).
--
-- THE LENGTH: 5000 CHARACTERS, the same number the write path already enforces on this exact field —
-- QuyetDinhLuiHan validates `GhiChu` with domain.KiemVanBanTuyChon(…, domain.NoiDungNhatKyToiDa, …)
-- (internal/app/nhiem_vu.go:1725-1726; NoiDungNhatKyToiDa = 5000 at internal/domain/nhiem_vu_ghi.go:43),
-- counted in runes after TrimSpace. char_length counts characters, not bytes, so the two agree and
-- Vietnamese diacritics do not shorten the allowance. A different number here would turn a note the
-- route accepted into a 500. migrations/task_extension_decision_note_test.go reads the Go constant so
-- the two cannot drift silently.
--
-- '' IS NOT "NO NOTE" — NULL IS. The write path trims and may end with ''; the next card must store NULL
-- then. Same convention as 0017's `rating_comment`.
--
-- THE NOTE IS STAFF FREE TEXT ABOUT A RECORD and in practice names people (rule 3): it never enters a
-- log line, an error message or an event payload. No statement in this file reads it.
--
-- ---------------------------------------------------------------------------
-- WHEN THE NOTE MAY BE WRITTEN — enforced by the database for every writer, not promised by one route:
--
--   * CHECK: a PENDING request holds no note. A note written before the decision would be a decision
--     text with no decision, and approving later would attach it to an act it did not describe.
--   * TRIGGER: the note may change ONLY in the UPDATE that moves `cho-duyet` → `da-duyet` / `tu-choi`.
--     Afterwards it is frozen like the request as filed: rewriting the reason a leader gave, after the
--     commitment moved (or did not), leaves a decision whose stated grounds are not the ones given.
--     The trigger itself (`de_nghi_lui_han_luu_tru`, BEFORE UPDATE OR DELETE, every column) is 0006's and
--     is NOT touched; only the function body it calls is replaced, keeping every refusal 0006 wrote.
--
-- ---------------------------------------------------------------------------
-- THE LOCK THIS TAKES. ADD COLUMN with no default is catalogue-only (no row rewritten) but takes ACCESS
-- EXCLUSIVE on the parent and all 32 partitions. The two ADD CONSTRAINT … CHECK statements scan every
-- row under that lock; on a column that is NULL in every row both pass. core/migrate holds the locks
-- until COMMIT. The register is small (one commune live, 0014's header) — AN ASSUMPTION, NOT A
-- MEASUREMENT; the operator checks first:
--
--   SELECT count(*), pg_size_pretty(pg_total_relation_size('de_nghi_lui_han')) FROM de_nghi_lui_han;
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. ROWS PER COMMUNE: zero written. Every row gains a NULL column without a rewrite; the CHECKs scan
--      every row once. NO BACKFILL, deliberately: decisions taken before this file have their note (if
--      any) only in the task log, as free text with no link to the request — copying it over would be
--      guessing which log entry answered which request. Those rows stay NULL and their text stays where
--      it was written. The NOTICE below reports, per commune, how many requests exist and how many are
--      already decided (= rows that will never carry a note) — counts and tenant ids only (rule 3).
--   2. IF IT STOPS HALF-WAY: it cannot land half-applied — one file, one transaction (core/migrate), with
--      its progress row. ADD COLUMN IF NOT EXISTS, CREATE OR REPLACE FUNCTION and constraints guarded by
--      a pg_constraint lookup make a retry cost nothing. No per-commune loop: nothing to backfill.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. READ PATHS THAT CHANGE MEANING WHILE HALF-APPLIED: none can see it half-applied. AFTER it, before
--      the Go card: every read of this table names its columns (store/nhiem_vu_ghi.go `cotDeNghi`,
--      store/de_nghi_lui_han_cho_duyet.go), so no reader sees the column. Every existing write passes:
--      the INSERT names its columns (note NULL, row pending — CHECK holds); the decision UPDATE
--      (store/nhiem_vu_ghi.go:849-851) does not name the note, so it stays NULL and the trigger's new arm
--      does not fire; the soft delete does not name it either. The task log keeps receiving the note
--      text exactly as today.
--   5. RETENTION: an extension request is an ARCHIVAL RECORD (rule 7). Nothing is dropped, retyped,
--      emptied or renumbered; the new arm makes the record STRICTER, never looser.
-- ---------------------------------------------------------------------------

ALTER TABLE de_nghi_lui_han ADD COLUMN IF NOT EXISTS decision_note TEXT;

-- ---------------------------------------------------------------------------
-- THE CONSTRAINTS. DO $$ … $$ because ADD CONSTRAINT has no IF NOT EXISTS (question 2). The lookup is
-- bound to this table's oid: a CHECK on a partitioned parent is copied to every partition under the same
-- name, and a bare conname match could be satisfied by some other table's constraint.
-- ---------------------------------------------------------------------------
DO $$ BEGIN
    -- Present means non-blank and bounded in characters (domain.NoiDungNhatKyToiDa).
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid = 'de_nghi_lui_han'::regclass
          AND conname = 'de_nghi_lui_han_decision_note_valid'
    ) THEN
        ALTER TABLE de_nghi_lui_han ADD CONSTRAINT de_nghi_lui_han_decision_note_valid
            CHECK (decision_note IS NULL
                   OR (btrim(decision_note) <> '' AND char_length(decision_note) <= 5000));
    END IF;

    -- A pending request holds no note: the note belongs to the decision, not to the request.
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid = 'de_nghi_lui_han'::regclass
          AND conname = 'de_nghi_lui_han_decision_note_needs_decision'
    ) THEN
        ALTER TABLE de_nghi_lui_han ADD CONSTRAINT de_nghi_lui_han_decision_note_needs_decision
            CHECK (trang_thai <> 'cho-duyet' OR decision_note IS NULL);
    END IF;
END $$;

-- ---------------------------------------------------------------------------
-- de_nghi_lui_han_bat_bien — 0006's guard, replaced in place under the same name. Everything above the
-- new arm is 0006:681-696 VERBATIM except the hint's list of movable fields, which now names the note.
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
            USING HINT = 'Only the decision may move (trang_thai, nguoi_duyet_ma, duyet_luc, '
                         'decision_note). Rewriting the requested date or the reason after a decision '
                         'leaves an approval attached to something the approver never read.';
    END IF;

    -- THE NOTE IS WRITTEN WITH THE DECISION, AND ONLY THEN (0031). Not before it (the CHECK
    -- de_nghi_lui_han_decision_note_needs_decision), not after it (here): any other UPDATE that changes
    -- it, setting, editing or clearing, is refused.
    IF NEW.decision_note IS DISTINCT FROM OLD.decision_note
       AND NOT (OLD.trang_thai = 'cho-duyet' AND NEW.trang_thai IN ('da-duyet', 'tu-choi')) THEN
        RAISE EXCEPTION 'administrative record %: decision_note is written only with the decision', TG_TABLE_NAME
            USING HINT = 'The note is set in the same UPDATE that moves cho-duyet to da-duyet or tu-choi, '
                         'and is frozen afterwards: the grounds a leader gave for a decision are part of '
                         'that decision (rule 7, forbidden #5).';
    END IF;

    RETURN NEW;
END $$;

COMMENT ON COLUMN de_nghi_lui_han.decision_note IS
    'The approver''s or rejecter''s note, written only in the UPDATE that decides the request (0031) and '
    'frozen afterwards. NULL = no note, and NULL on every request decided before 0031: their note, if '
    'any, is only in nhat_ky_nhiem_vu.noi_dung.';

-- ---------------------------------------------------------------------------
-- MEASUREMENT (question 1): requests per commune, and how many are already decided (they keep NULL).
-- Counts and tenant ids only.
-- ---------------------------------------------------------------------------
DO $$
DECLARE r record;
BEGIN
    FOR r IN
        SELECT tenant_id,
               count(*) AS total,
               count(*) FILTER (WHERE trang_thai <> 'cho-duyet') AS decided
        FROM de_nghi_lui_han
        GROUP BY tenant_id
        ORDER BY tenant_id
    LOOP
        RAISE NOTICE '0031: commune % holds % extension request(s), % already decided (no decision_note)',
            r.tenant_id, r.total, r.decided;
    END LOOP;
END $$;

-- ---------------------------------------------------------------------------
-- WHAT THIS FILE DOES NOT STOP, said plainly: DDL by the table owner (dropping the constraints,
-- DISABLE TRIGGER). Same line ADR 0013 draws for the audit ledger.
--
-- REVERSAL (rule 7, invariant 4). Run by a person, in ONE transaction; core/migrate has no automatic
-- rollback (ADR 0013).
--
--   Lossless ONLY WHILE NO ROW HOLDS A NOTE:
--
--     SELECT count(*) FROM de_nghi_lui_han WHERE decision_note IS NOT NULL;   -- must be 0
--
--   ZERO → re-run 0006's CREATE OR REPLACE FUNCTION de_nghi_lui_han_bat_bien() verbatim (0006:678-699),
--   then
--     ALTER TABLE de_nghi_lui_han DROP CONSTRAINT de_nghi_lui_han_decision_note_needs_decision;
--     ALTER TABLE de_nghi_lui_han DROP CONSTRAINT de_nghi_lui_han_decision_note_valid;
--     ALTER TABLE de_nghi_lui_han DROP COLUMN decision_note;
--   and remove this file's row from `schema_migration`, otherwise the runner still believes the schema
--   is in place. Restore the function BEFORE dropping the column: the replaced body names it.
--
--   NON-ZERO → a leader's decision note exists on an archival record. Dropping the column destroys part
--   of that record — rule 7, stop condition #2: a user decision plus a verified backup, never a command.
--   Keeping the column and restoring only 0006's function is lossless at any time, but un-freezes the
--   note; that too is the user's call.
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
