-- 0024 — tenant.launch_code: the opaque PUBLIC code a commune's QR / launch link carries
-- (`?x=<code>&src=qr`) instead of the commune's domain. Owner decision, 12/10/2026.
--
-- WHAT THE OWNER FIXED, and what this file therefore enforces:
--   * one code per commune, fixed (no rotation today);
--   * random — NOT derivable from the name, the domain or tenant.id. A code computed from any of
--     those is a code anyone can compute for another commune, and a code that changes when the
--     commune is renamed or merged (rule 1, forbidden #5's reason);
--   * NEVER reissued to another commune (rule 7, invariant 3): the unique index below counts every
--     row, active or not, with no WHERE clause;
--   * a merged (inactive) commune KEEPS its row and its code (rule 7, invariant 6). Refusing to
--     resolve an inactive commune's code is the read path's job, not this table's.
--
-- WHAT THE CODE IS NOT: a credential. It steers which commune's interface opens and grants nothing
-- (ADR 0005 · 0019 · 0022): the commune enters a session only by the citizen's explicit act. That is
-- why 48 random bits are enough here, and why it may sit in audit entries and on printed paper.
--
-- SHAPE: exactly 12 lowercase hex characters, taken from gen_random_uuid() — core PostgreSQL since
-- 13 (the cluster is PG16), no extension. It draws from the server's strong random source, the same
-- one pgcrypto uses. The FIRST 12 hex digits of a version-4 UUID are all random: the version nibble
-- is digit 13 and the variant bits are in digit 17, both outside the slice. Hash-of-something and
-- non-cryptographic generators are refused by rule 13 (forbidden #2) and by the owner's
-- "not derivable".
--
-- NO COLUMN DEFAULT, ON PURPOSE. New communes get their code minted in Go at creation
-- (store/operator_writes.go CreateCommune, TASK-03). A DEFAULT here would look like a safety net
-- and act as a second, silent minting path: the day the Go mint is forgotten or broken, communes
-- would still be created, with codes nobody in Go chose and no audit entry naming them. Without a
-- default, a forgotten mint fails NOT NULL on the INSERT — loudly, on the first commune, inside the
-- transaction that also writes the trail, so nothing is half-created.
-- CONSEQUENCE, stated: from the moment this file is applied, every INSERT INTO tenant that does not
-- name launch_code FAILS. This file and the Go change that mints the code must ship in the SAME
-- release (migrations are embedded in the binary, embed.go — same commit is enough).
--
-- NAME (rule 12, ADR 0051): English. The table's older columns are Vietnamese (ADR 0061 revert);
-- they are not renamed.
--
-- WHY A NEW FILE: core/migrate compares the checksum of every applied file at startup.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. ROWS PER COMMUNE: exactly one — the commune's own registry row. The whole table is one row
--      per commune ever onboarded (single digits today, at most a few thousand in any future).
--      PER COMMUNE AND RESUMABLE (rule 7, invariant 5): ONE STATEMENT, not a per-commune loop, for
--      the reason 0021 gives: core/migrate gives a file one transaction, so a loop inside it could
--      not be resumed either, and the per-commune backfill mechanism does not exist. At one row per
--      commune there is nothing to batch, and every row carries its own tenant id into its own audit
--      entry.
--   2. IF IT STOPS HALF-WAY: it cannot land half-applied — one file, one transaction with the
--      runner's progress row. A failure (including a code collision, see 5) rolls back the column,
--      every code and every audit entry together. A re-run is free: ADD COLUMN IF NOT EXISTS, the
--      backfill only touches `launch_code IS NULL`, the CHECK sits behind a pg_constraint lookup,
--      the index is IF NOT EXISTS.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. READ PATHS THAT CHANGE MEANING: none while half-applied (one transaction). After it: no read
--      path names launch_code yet, and store/ reads name their columns (no `SELECT *` on tenant), so
--      every existing read returns exactly what it did. The one WRITE path that changes is the
--      tenant INSERT — see "NO COLUMN DEFAULT" above.
--   5. RETENTION: nothing dropped, retyped or renumbered; no existing value is changed — a column is
--      ADDED and filled. Inactive / succeeded communes are filled too, because the owner's decision
--      is that they keep a code, and a NOT NULL column cannot skip them; their name, domains, status
--      and history are untouched. cap_nhat_luc is NOT bumped (0021's reason: it keeps naming the
--      last human change; this file's who/when is the audit entry). A collision between two freshly
--      drawn codes (probability ~ n^2 / 2^49) makes the UNIQUE index build FAIL and the whole file
--      roll back — never a silent duplicate; the next start draws again.
--
-- AUDIT (rule 6, invariant 6): each commune's code issue is one entry in audit_log under THAT
-- commune, actor 'system' / kind 'system', in the same statement as the write (invariant 3). Action
-- `launch_code.issued` (the dotted English shape of petition_field.created / upload_policy.changed);
-- the Go path that mints at creation should name the code in its own trail entry so both lanes read
-- the same. The entry holds the code itself: it is public by design, and it makes the reversal
-- recoverable (see REVERSAL).
--
-- PERSONAL DATA (rule 3): none. A random code and a commune name.
-- ---------------------------------------------------------------------------

ALTER TABLE tenant ADD COLUMN IF NOT EXISTS launch_code TEXT;

DO $$ BEGIN
    -- Before the backfill, so a malformed value fails at the UPDATE rather than later. NULL passes a
    -- CHECK; NOT NULL below is what forbids it.
    IF NOT EXISTS (SELECT 1 FROM pg_constraint
                   WHERE conrelid = 'tenant'::regclass
                     AND conname = 'tenant_launch_code_shape') THEN
        ALTER TABLE tenant ADD CONSTRAINT tenant_launch_code_shape
            CHECK (launch_code ~ '^[0-9a-f]{12}$');
    END IF;
END $$;

DO $$
DECLARE issued int;
BEGIN
    WITH changed AS (
        -- gen_random_uuid() is VOLATILE: evaluated once per row, so every commune draws its own code.
        UPDATE tenant t
        SET launch_code = substr(replace(gen_random_uuid()::text, '-', ''), 1, 12)
        WHERE t.launch_code IS NULL
        RETURNING t.id, t.ten, t.dang_hoat_dong, t.launch_code
    )
    INSERT INTO audit_log (tenant_id, actor_id, actor_kind, actor_ip, action, subject, at, delta)
    SELECT c.id, 'system', 'system', '', 'launch_code.issued', c.ten, now(),
           jsonb_build_object(
               'launch_code',    c.launch_code,
               'dang_hoat_dong', c.dang_hoat_dong,
               'qua',            'migration 0024_tenant_launch_code.sql')
    FROM changed c;
    GET DIAGNOSTICS issued = ROW_COUNT;

    RAISE NOTICE '0024: launch_code issued to % commune(s), one launch_code.issued entry each', issued;
END $$;

ALTER TABLE tenant ALTER COLUMN launch_code SET NOT NULL;

-- ---------------------------------------------------------------------------
-- tenant_launch_code_unique — ONE code names ONE commune, forever.
--
-- SINGLE-COLUMN, WITHOUT tenant_id, AND THAT IS CORRECT (rule 1, invariant 6 does not apply): the
-- code is resolved BEFORE any commune is known — it is how the commune is found, exactly like
-- tenant_domain.host (0001). A key composite with tenant_id would let two communes hold the same
-- code, and the resolution would be undecidable at the edge. `tenant` is the platform registry, not
-- a commune's business data.
--
-- NO `WHERE` CLAUSE (rule 7, invariant 3): an inactive commune's code stays in the index, so it can
-- never be drawn again for another commune.
--
-- A UNIQUE INDEX rather than ALTER TABLE … ADD CONSTRAINT UNIQUE so that tools/check_khoa_duy_nhat.py
-- examines it; the mark below is the argument above in the form that gate reads.
--
-- @scope:  platform
-- ---------------------------------------------------------------------------
CREATE UNIQUE INDEX IF NOT EXISTS tenant_launch_code_unique ON tenant (launch_code);

COMMENT ON COLUMN tenant.launch_code IS
    'Ma cong khai cua xa tren QR / lien ket mo app (?x=<ma>&src=qr), quyet dinh chu du an 12/10/2026. '
    '12 ky tu hex thuong, ngau nhien, khong suy ra tu ten / ten mien / id. Co dinh, khong cap lai cho '
    'xa khac; xa da sap nhap van giu ma. Cap trong Go khi tao xa - cot KHONG co gia tri mac dinh.';

-- ---------------------------------------------------------------------------
-- REVERSAL (rule 7, invariant 4). Run by a person, in ONE transaction; core/migrate has no automatic
-- rollback (ADR 0013). Prose, not runnable — a runnable line is a line that gets run.
--
-- Drop index tenant_launch_code_unique, constraint tenant_launch_code_shape, column launch_code, and
-- remove this file's row from `schema_migration` (ten = '0024_tenant_launch_code.sql'). The Go mint
-- (TASK-03) must be reverted in the same release, or commune creation fails on a missing column.
--
-- WHAT IS LOST: no business record — the column holds no citizen data and no administrative record.
-- But every QR already PRINTED with a code stops resolving: re-applying draws NEW codes. The old
-- codes are recoverable from the append-only trail (0002), which reversal never touches:
--
--   SELECT tenant_id, delta->>'launch_code' FROM audit_log
--   WHERE action = 'launch_code.issued' AND actor_kind = 'system'
--     AND delta->>'qua' = 'migration 0024_tenant_launch_code.sql';
--
-- plus the Go path's creation entries. Restoring them instead of drawing new ones is a NEW migration.
-- Once any QR is in circulation, reversal is a user decision, not an operator's.
-- ---------------------------------------------------------------------------

-- BACKSTOP, carried forward from 0002: a table declared PARTITION BY with no partitions rejects every
-- INSERT. This file declares no partitioned table; it runs anyway.
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
