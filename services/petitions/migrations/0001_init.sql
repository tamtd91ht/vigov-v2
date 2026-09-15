-- petitions — initial schema
--
-- Invariants enforced here, not by convention (ADR 0004):
--   * every business table carries tenant_id NOT NULL
--   * every unique key is COMPOSITE with tenant_id  — a single-column key breaks commune #2
--   * every index starts with tenant_id             — it is the shard key
--   * large tables are PARTITIONED by tenant from the start: adding partitioning later is a
--     migration over archival records, out of hours, with real risk. Now it costs nothing.
--
-- Migrations run PER COMMUNE, are resumable, and record progress.

-- Audit trail. One table per service, on purpose: rule 6 requires the entry to share a
-- transaction with the business write, and two services cannot share a transaction.
-- Append-only: no UPDATE, no DELETE, not even by an administrator.
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

CREATE INDEX IF NOT EXISTS audit_log_lookup
    ON audit_log (tenant_id, subject, at DESC);

-- TODO(skeleton): business tables for petitions.
-- Declared entities (see README): petitions, petition_categories, sla_rules, tasks
--
-- Shape every one of them must follow:
--
--   CREATE TABLE example (
--       tenant_id  TEXT NOT NULL,
--       id         TEXT NOT NULL,
--       code       TEXT NOT NULL,            -- business code, never reissued
--       deleted_at TIMESTAMPTZ,              -- soft delete: archival records are never removed
--       deleted_by TEXT,
--       delete_reason TEXT,
--       PRIMARY KEY (tenant_id, id),
--       UNIQUE (tenant_id, code)             -- COMPOSITE, always
--   ) PARTITION BY HASH (tenant_id);
--
-- Read paths filter deleted rows with  deleted_at IS NULL  in EVERY query — lists,
-- statistics, search, reports and background jobs alike.
