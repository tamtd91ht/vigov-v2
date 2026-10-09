-- 0035 — "Tắt / Bật" on EVERY shipped sentence, including one the commune never reworded (user
-- decision 09/10/2026, which replaces ADR 0079 lô 3 and the 409 `no_commune_wording` of 0033 §A).
--
-- THIS FILE REPLACES PART OF A DECISION WRITTEN IN 0033, which cannot be edited (core/migrate,
-- ErrChecksumLech). 0033 §A put the switch on the commune's WORDING only: a key with no wording had no
-- switch, and "a commune may not silence a shipped refusal". The project owner decided on 09/10/2026,
-- following the prototype (../vigov-require/apps/admin/src/components/admin/MessageTemplateTable.tsx:
-- 193-194), that every sentence has Tắt/Bật. Where 0033's comment and this file disagree, this file wins.
--
-- WHAT "TẮT" MEANS SINCE THIS FILE: the sentence is HIDDEN where it is used; a consumer that must say
-- something falls back to the software's sentence. Every key this service ships is a refusal, and a
-- refusal must say something, so every reader here keeps resolving the default while off
-- (domain.ResolveMessage) — the switch changes the configuration screen, not what an officer is told.
--
-- HOW A SWITCHED-OFF SENTENCE WITH NO WORDING IS STORED: a live `system_message_override` row whose
-- `message_text` is NULL and `is_active` is false. Not a copy of the default text: a row holding the
-- default would pin it, and the commune would silently stop receiving a corrected default in a later
-- release (0020, "NO SEED ROWS"). "Bật" on such a row soft deletes it (rule 7, invariant 1) — the commune
-- is back to "no row", which is the state of every commune that never touched the screen.
--
-- THE CHECK BELOW makes the one meaningless state unrepresentable: a live row with no wording that is
-- switched ON would be a row that changes nothing and that nobody can explain.
--
-- 0020's `system_message_override_text_shape` CHECK still governs every wording that IS present: a NULL
-- makes that CHECK evaluate to NULL, which PostgreSQL treats as satisfied, so it needs no change.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. ROWS PER COMMUNE: at most six live overrides per commune, plus reverted ones. DROP NOT NULL is a
--      catalogue change; the CHECK scans those few rows once to validate them, and every existing row
--      has a wording, so it passes.
--   2. IF IT STOPS HALF-WAY: it cannot — one file, one transaction with its progress row. The CHECK is
--      added only when absent, so a retry costs nothing. Nothing is backfilled.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. READ PATHS THAT CHANGE MEANING: none at landing — no row has a NULL wording until a commune
--      switches off a sentence it never reworded. From then on the readers must treat a NULL wording as
--      "not reworded" (store.scanOverride reads it as "", domain.ResolveMessage resolves the default).
--   5. RETENTION: nothing removed. The row is soft deleted on "Bật", never hard deleted (0033's trigger).
-- ---------------------------------------------------------------------------

ALTER TABLE system_message_override
    ALTER COLUMN message_text DROP NOT NULL;

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint
                   WHERE conname = 'system_message_override_wording_or_off'
                     AND conrelid = 'system_message_override'::regclass) THEN
        ALTER TABLE system_message_override
            ADD CONSTRAINT system_message_override_wording_or_off
            CHECK (message_text IS NOT NULL OR NOT is_active);
    END IF;
END $$;

-- ---------------------------------------------------------------------------
-- REVERSAL (rule 7, invariant 4). By a person, in ONE transaction, then remove this file's row from
-- `schema_migration` (core/migrate has no automatic rollback, ADR 0013):
--   ALTER TABLE system_message_override DROP CONSTRAINT system_message_override_wording_or_off;
--   ALTER TABLE system_message_override ALTER COLUMN message_text SET NOT NULL;
-- The second line FAILS while
--   SELECT count(*) FROM system_message_override WHERE message_text IS NULL;   -- must be 0
-- and making it pass means removing those rows, which silently switches those sentences back ON for
-- their communes — a user decision, not a command.
-- ---------------------------------------------------------------------------
