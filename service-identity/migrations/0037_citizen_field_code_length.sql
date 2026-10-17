-- 0037 — widen the shape of citizen_field.code from 6–8 to 6–12 characters.
--
-- OWNER DECISION 17/10/2026 (ADR 0101 #1b, amended): the field code is the field's id and the key that
-- maps a variable in the ZNS / SMS flows; the web generates it RANDOMLY, 8–12 characters `[a-z][a-z0-9]`,
-- editable before saving, and the server refuses a duplicate within the commune (UNIQUE (tenant_id,
-- code), 0036). NEW codes must be 8–12 — domain.CheckNewCitizenField enforces that. The DATABASE accepts
-- 6–12 so that a code of 6–8 characters created under 0036's rule (the hours between 0036 and this file)
-- stays a valid row: rewriting a code would orphan every value stored under it in
-- citizen_contact.attributes and every campaign's attribute_mapping (rule 7 — no silent edits).
--
-- WHY A NEW FILE AND NOT AN EDIT TO 0036: core/migrate checksums every applied file at startup and stops
-- the service when one has changed (ErrChecksumLech).
--
-- THE FIVE MIGRATION QUESTIONS.
-- 1. ROWS: none written. One CHECK replaced on citizen_field (≤ 50 live rows per commune).
-- 2. LOCK: ADD CONSTRAINT validates every row under ACCESS EXCLUSIVE — milliseconds at this size.
-- 3. LOSS: none; the new CHECK is strictly wider than the old one, so every existing row passes.
-- 4. RESUMABLE: one transaction; re-running is a no-op (DROP … IF EXISTS, then ADD).
-- 5. REVERSAL: DROP CONSTRAINT citizen_field_code_shape; ADD it back with '^[a-z][a-z0-9]{5,7}$' — refused
--    if any 9–12 character code exists by then, which is the correct refusal (those codes carry data).

ALTER TABLE citizen_field DROP CONSTRAINT IF EXISTS citizen_field_code_shape;
ALTER TABLE citizen_field ADD CONSTRAINT citizen_field_code_shape CHECK (code ~ '^[a-z][a-z0-9]{5,11}$');
