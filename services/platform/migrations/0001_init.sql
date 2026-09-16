-- platform — initial schema
--
-- Invariants enforced here, not by convention (ADR 0004):
--   * every business table carries tenant_id NOT NULL
--   * every unique key is COMPOSITE with tenant_id  — a single-column key breaks commune #2
--   * every index starts with tenant_id             — it is the shard key
--   * large tables are PARTITIONED by tenant from the start: adding partitioning later is a
--     migration over archival records, out of hours, with real risk. Now it costs nothing.
--
-- MODULUS 32 everywhere (ADR 0010). At 200+ communes that is ~6 communes per partition.
-- Changing the modulus after real data exists rewrites the whole table, so it is fixed here
-- and never tuned per table.
--
-- Migrations run PER COMMUNE, are resumable, and record progress.

-- ---------------------------------------------------------------------------
-- Audit trail. One table per service, on purpose: rule 6 requires the entry to share a
-- transaction with the business write, and two services cannot share a transaction.
-- Append-only: entries are never modified and never removed, not even by an administrator.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS audit_log (
    id          BIGSERIAL,
    tenant_id   TEXT        NOT NULL,
    actor_id    TEXT        NOT NULL,     -- staff code, citizen id, or 'system'
    actor_kind  TEXT        NOT NULL,     -- staff | citizen | system
    actor_ip    TEXT        NOT NULL DEFAULT '',
    action      TEXT        NOT NULL,     -- business verb, never a function name
    subject     TEXT        NOT NULL,     -- business code, never an internal id
    at          TIMESTAMPTZ NOT NULL,
    delta       JSONB,                    -- before/after, personal data ALREADY masked
    PRIMARY KEY (tenant_id, id)
) PARTITION BY HASH (tenant_id);

-- A partitioned table with no partitions accepts no rows: every insert fails with "no
-- partition of relation found". The parent declaration above is only half the statement.
DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS audit_log_p%s PARTITION OF audit_log '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

CREATE INDEX IF NOT EXISTS audit_log_lookup
    ON audit_log (tenant_id, subject, at DESC);

-- ---------------------------------------------------------------------------
-- tenant — THE ONE TABLE WITHOUT tenant_id, and the only one that may be so.
--
-- WHY IT IS EXEMPT: this table DEFINES what a commune is. A row here is not data belonging
-- to a commune, it is the commune's own existence. Scoping it by tenant_id would require
-- knowing the commune before being able to look the commune up.
--
-- Consequence, and it is not optional: pkg/store.Scoped CANNOT read this table, because it
-- always injects WHERE tenant_id = $1. Access goes through the dedicated directory
-- repository, which is the only sanctioned unscoped reader in the whole system.
--
-- id is an opaque, immutable ULID. NOT the administrative code, NOT the domain, NOT the
-- name: Vietnam reorganises commune-level units periodically, and an identifier carrying
-- meaning would force rewriting foreign keys across archival records at the first merger
-- (rule 1, invariant 2).
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS tenant (
    id             TEXT        PRIMARY KEY,           -- ULID, 26 chars, opaque
    ten            TEXT        NOT NULL,              -- display name AT THIS MOMENT, not an id
    tinh_thanh     TEXT        NOT NULL DEFAULT '',   -- province, for display only
    dang_hoat_dong BOOLEAN     NOT NULL DEFAULT true, -- a merged commune is deactivated, kept
    tao_luc        TIMESTAMPTZ NOT NULL DEFAULT now(),
    cap_nhat_luc   TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT tenant_id_la_ulid CHECK (length(id) = 26)
);

-- ---------------------------------------------------------------------------
-- tenant_domain — the Host -> commune mapping, read on EVERY single request.
--
-- Separate from tenant because a commune may hold several hosts at once: the merger of two
-- communes must keep the old commune's address resolving while people learn the new one.
-- Rule 7 forbids discarding the old commune's data, and that includes its address.
--
-- host is globally unique WITHOUT tenant_id, and that is correct rather than a violation of
-- rule 1: a host that resolved to two communes would make the isolation undecidable at the
-- edge. This uniqueness is what MAKES the isolation work.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS tenant_domain (
    host        TEXT        PRIMARY KEY,             -- "tanphu.vigov.vn", lower-case, no port
    tenant_id   TEXT        NOT NULL REFERENCES tenant(id),
    la_chinh    BOOLEAN     NOT NULL DEFAULT false,  -- canonical host, used when building links
    tao_luc     TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT tenant_domain_host_thuong CHECK (host = lower(host))
);

CREATE INDEX IF NOT EXISTS tenant_domain_theo_xa ON tenant_domain (tenant_id);

-- Exactly one canonical host per commune. Without this, link building picks whichever row
-- comes back first and the same commune appears under different addresses in notifications.
CREATE UNIQUE INDEX IF NOT EXISTS tenant_domain_mot_chinh
    ON tenant_domain (tenant_id) WHERE la_chinh;
