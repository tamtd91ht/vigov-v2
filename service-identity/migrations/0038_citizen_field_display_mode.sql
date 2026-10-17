-- 0038 — citizen_field.display_mode (how a combobox / singleBox shows its options on the contact form) and
-- citizen_field.layout_width (half or full row of the form's two-column layout).
--
-- OWNER DECISION 17/10/2026 ("nên cho cấu hình singlebox combobox là hiển thị full hay mở dạng dropdown"):
--   'full'     every option is on screen — checkboxes for combobox, radio buttons for singleBox;
--   'dropdown' the options open from one control.
-- Only the two choice types have options, so only they carry a mode; every other type stores ''.
-- The value is MUTABLE (PATCH): it changes how the form is drawn, never what is stored for a person, so
-- the 0036 immutability trigger (code, field_type, option removal) does not cover it and is not touched.
--
-- citizen_field.layout_width (owner, same day: the contact form lays the dynamic fields out in TWO
-- columns): 'half' takes one column, 'full' spans both. The LAYOUT is sort_order (the existing reorder
-- route) plus this width, flowing left-to-right into two columns — no row/column coordinates are stored,
-- so no second ordering exists to drift from sort_order. Every type may take either width. MUTABLE (PATCH),
-- outside the 0036 trigger for the same reason as display_mode.
--
-- WHY A NEW FILE AND NOT AN EDIT TO 0036: core/migrate checksums every applied file at startup and stops
-- the service when one has changed (ErrChecksumLech).
--
-- WHY '' AND NOT NULL for the types without a mode: one spelling of "does not apply" for the store and the
-- wire (`display_mode: ""`), and the CHECK below pins it — '' is not a default the application can fall
-- back on, it is the only value those types may hold.
--
-- THE BACKFILL. Existing combobox / singleBox rows were created before a mode existed and must hold one
-- for the CHECK to validate. They get 'full' — the option list as it was described at creation (ADR 0101
-- #1: options shown as a list of {value, label}). The UPDATE is FILTERED to exactly the rows the CHECK
-- would refuse — `field_type IN ('combobox','singleBox') AND display_mode = ''` — so it never touches a
-- row of another type, and a re-run finds nothing (rule 7: no unfiltered UPDATE). It writes neither
-- updated_at nor an audit entry: no person's data and no definition the commune made changes meaning; the
-- column is new and the value records the behaviour already on screen. The 0036 trigger fires on it and
-- passes (code, type and options are untouched).
--
-- THE SECOND BACKFILL. layout_width's constant default 'half' suits one-line inputs; an existing multiText
-- field is a multi-line text area, which in half a row would be cramped next to a one-line neighbour. It
-- gets 'full' — the same width a new multiText gets when the create body omits it. FILTERED to
-- `field_type = 'multiText' AND layout_width = 'half'` (rule 7: no unfiltered UPDATE); a retry of the
-- same transaction finds those rows already 'full' and writes nothing. Same no-audit reasoning as above:
-- a new column's first value, no definition the commune made changes meaning.
--
-- THE FIVE MIGRATION QUESTIONS.
-- 1. ROWS: two columns added to citizen_field (≤ 50 live rows per commune, plus retired ones); the
--    backfills write only that commune's combobox / singleBox rows and its multiText rows. No other table.
-- 2. LOCK: ADD COLUMN with a CONSTANT default is catalogue-only on PostgreSQL 11+ (the cluster is 16), no
--    rewrite; the UPDATE and ADD CONSTRAINT take ACCESS EXCLUSIVE for milliseconds at this size.
-- 3. LOSS: none. Nothing is dropped, retyped or renamed; no existing value is read into another.
-- 4. RESUMABLE: one transaction (core/migrate); ADD COLUMN IF NOT EXISTS, a filtered UPDATE that is a
--    no-op the second time, DROP CONSTRAINT IF EXISTS then ADD.
-- 5. REVERSAL: DROP CONSTRAINT citizen_field_display_mode_valid, citizen_field_layout_width_valid; DROP
--    COLUMN display_mode, layout_width — both hold configuration only (how the form is drawn), never data
--    about a person, so dropping them loses only the communes' choices, which they would set again.

ALTER TABLE citizen_field ADD COLUMN IF NOT EXISTS display_mode TEXT NOT NULL DEFAULT '';

UPDATE citizen_field SET display_mode = 'full'
 WHERE field_type IN ('combobox', 'singleBox') AND display_mode = '';

ALTER TABLE citizen_field DROP CONSTRAINT IF EXISTS citizen_field_display_mode_valid;
ALTER TABLE citizen_field ADD CONSTRAINT citizen_field_display_mode_valid
    CHECK ((field_type IN ('combobox', 'singleBox') AND display_mode IN ('full', 'dropdown'))
        OR (field_type NOT IN ('combobox', 'singleBox') AND display_mode = ''));

ALTER TABLE citizen_field ADD COLUMN IF NOT EXISTS layout_width TEXT NOT NULL DEFAULT 'half';

UPDATE citizen_field SET layout_width = 'full'
 WHERE field_type = 'multiText' AND layout_width = 'half';

ALTER TABLE citizen_field DROP CONSTRAINT IF EXISTS citizen_field_layout_width_valid;
ALTER TABLE citizen_field ADD CONSTRAINT citizen_field_layout_width_valid
    CHECK (layout_width IN ('half', 'full'));
