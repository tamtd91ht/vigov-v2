-- 0019 — `nguoi_dung.email` becomes OPTIONAL: an empty address is stored as NULL, never as ''.
-- User decision 2026-09-29, ADR 0059 §1 ("Email trống lưu NULL, không lưu chuỗi rỗng"), for the staff
-- import: a commune imports its whole directory, and many of those people have no work address.
--
-- WHY IT IS NEEDED. 0001 declares `email TEXT NOT NULL` with `UNIQUE (tenant_id, email)`. The empty
-- string is a VALUE, so a commune could hold exactly ONE person without an address; the second would be
-- refused by the unique key (kb/90-ephemeral/tien-do/service-identity.json, item (4); the argument is at
-- domain.ChuanHoaEmail). Two NULLs are distinct under a unique key (PostgreSQL's default NULLS
-- DISTINCT), so any number of people without an address fit — which is exactly what the decision asks.
--
-- WHY A NEW FILE AND NOT AN EDIT OF 0001: core/migrate compares the checksum of every applied file.
--
-- WHAT DOES NOT CHANGE. Sign-in matches `email = $2` (store.CanBoStore.TheoEmail): NULL never equals
-- anything, so a person without an address can never sign in — and the account flow refuses to issue
-- one to them (app.ErrStaffHasNoEmail), because the address IS the login. The unique key, the sign-in index
-- and every read path are untouched; the reads COALESCE the column back to '' for the Go side, so no
-- response shape moves.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. ROWS PER COMMUNE: at most ONE row per commune can hold '' today (the unique key), and the create
--      route has refused an empty address since it was written (domain.ErrThieuEmail) — so in practice
--      none. The UPDATE below touches only those rows.
--   2. IF IT STOPS HALF-WAY: it cannot — core/migrate runs the file in ONE transaction with its
--      progress row. A failure leaves the column NOT NULL, every '' in place, and the next start
--      retries from the beginning; every statement is safe to retry.
--   3. HOW IT IS REVERSED: see REVERSAL below.
--   4. READ PATHS THAT CHANGE MEANING WHILE HALF-APPLIED: none. Between this file and the Go change
--      that ships with it no row holds NULL except the '' rows converted here, and the Go reads
--      COALESCE, so '' and NULL read identically.
--   5. RETENTION: nothing is removed. '' and NULL both mean "no address"; the conversion loses no fact.
--
-- LOCKS. `ALTER COLUMN … DROP NOT NULL` takes ACCESS EXCLUSIVE on the parent and its 32 partitions
-- (ADR 0010) and rewrites nothing — a catalogue change, released at COMMIT. `ADD CONSTRAINT … CHECK`
-- takes the same lock and SCANS every partition once to validate: tens of rows per commune (the
-- customer's own seed is 26), so milliseconds. Sign-in reads this table, so it waits for that long.
-- ---------------------------------------------------------------------------

ALTER TABLE nguoi_dung ALTER COLUMN email DROP NOT NULL;

-- '' → NULL, with a filter (rule 7, forbidden #2). `cap_nhat_luc` is deliberately NOT touched: it means
-- "a person last edited this record", and a schema normalisation is not a business edit (the same
-- reading as 0003 §2). NOTICE lines carry counts only — never an address (rule 3).
DO $$
DECLARE
    so_dong int;
BEGIN
    UPDATE nguoi_dung SET email = NULL WHERE email = '';
    GET DIAGNOSTICS so_dong = ROW_COUNT;
    RAISE NOTICE '0019: % row(s) with an empty email set to NULL', so_dong;
END $$;

-- THE FLOOR: '' can never come back, whichever writer tries. The Go write paths already send NULL for
-- a blank address (store: nullif(…,'')); this makes a blank written from anywhere else a refusal rather
-- than the second address-less person silently blocking the third. Validated against the rows the
-- UPDATE above has just normalised, in the same transaction, so it cannot fail on existing data.
ALTER TABLE nguoi_dung ADD CONSTRAINT nguoi_dung_email_not_blank
    CHECK (email IS NULL OR btrim(email) <> '');

COMMENT ON COLUMN nguoi_dung.email IS
    'Thu dien tu cong vu, cung la ten dang nhap. NULL = khong co (ADR 0059 muc 1); khong bao gio la chuoi rong.';

-- ---------------------------------------------------------------------------
-- REVERSAL (rule 7, invariant 4). By hand, in ONE transaction, and ONLY while no commune holds MORE
-- THAN ONE row with `email IS NULL` (check first — grouped by tenant_id):
--
--   drop constraint nguoi_dung_email_not_blank; set email = '' where email is null; set the column
--   back to NOT NULL; remove this file's row from schema_migration.
--
-- Once a commune holds two address-less people, reversal is impossible without inventing an address
-- for one of them — that is data the commune did not enter, so it is a decision for the user (rule 7,
-- stop condition #2), not a script. Written as prose, not a runnable line, because a runnable line is a
-- line that gets run.
-- ---------------------------------------------------------------------------
