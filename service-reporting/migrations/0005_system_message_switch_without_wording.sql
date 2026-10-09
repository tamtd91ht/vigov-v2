-- 0005 — "Tắt / Bật" on EVERY shipped report sentence, including one the commune never reworded (user
-- decision 09/10/2026, which replaces ADR 0079 lô 3 and the 409 `no_commune_wording` of 0004). The same
-- change as service-petitions' 0035 and service-finance's 0019, in this service's own schema (rule 2,
-- invariant 2).
--
-- THIS FILE REPLACES PART OF A DECISION WRITTEN IN 0004, which cannot be edited (core/migrate,
-- ErrChecksumLech). 0004 put the switch on the commune's WORDING only. The project owner decided on
-- 09/10/2026, following the prototype (../vigov-require/apps/admin/src/components/admin/
-- MessageTemplateTable.tsx:193-194), that every sentence has Tắt/Bật. Where 0004's comment and this file
-- disagree, this file wins.
--
-- WHAT "TẮT" MEANS SINCE THIS FILE: the sentence is HIDDEN where it is used; a consumer that must say
-- something falls back to the software's sentence. NOTHING IN THIS SERVICE PRINTS A `report.*` SENTENCE
-- YET (internal/app/system_message.go header): the export of `/bao-cao` and the report notifications are
-- not built. Whoever builds them decides, per sentence, "hidden" or "default" from the resolved
-- `Active` — the switch is stored here so that decision has something to read.
--
-- HOW A SWITCHED-OFF SENTENCE WITH NO WORDING IS STORED: a live `system_message_override` row whose
-- `message_text` is NULL and `is_active` is false — never a copy of the default, which would pin it
-- (0003, "NO SEED ROWS"). "Bật" on such a row soft deletes it (rule 7, invariant 1).
--
-- THE CHECK BELOW makes the one meaningless state unrepresentable: a live row with no wording that is
-- switched ON. 0003's text-shape CHECK still governs every wording that IS present: a NULL makes it
-- evaluate to NULL, which PostgreSQL treats as satisfied.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. ROWS PER COMMUNE: at most 38 live overrides per commune, plus reverted ones. DROP NOT NULL is a
--      catalogue change; the CHECK scans those rows once, and every existing row has a wording.
--   2. IF IT STOPS HALF-WAY: it cannot — one file, one transaction with its progress row. The CHECK is
--      added only when absent, so a retry costs nothing. Nothing is backfilled.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. READ PATHS THAT CHANGE MEANING: none at landing. From the first switched-off unworded sentence, a
--      NULL wording reads as "not reworded" (store.scanOverride reads it as "", domain.ResolveMessage
--      resolves the default).
--   5. RETENTION: nothing removed. The row is soft deleted on "Bật", never hard deleted (0004's trigger).
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
-- and making it pass means removing those rows, which silently switches those sentences back ON — a user
-- decision, not a command.
-- ---------------------------------------------------------------------------
