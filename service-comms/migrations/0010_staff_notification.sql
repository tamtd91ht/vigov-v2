-- comms — THE HEADER-BELL INBOX: system notices to ONE member of staff (docs/ui-ux/08-thong-bao.md §8,
-- docs/ui-ux/15-phu-luc-giao-dien-chung.md §3.2 — `hop_thu_thong_bao`).
--
-- WHY IT EXISTS NOW, when 0005 refused to create it: 0005:63-69 held back because the inbox is fed
-- by OTHER services, which makes it an inter-service contract whose source of truth is `.proto`.
-- That contract now exists — CommsService.DeliverStaffNotifications (proto/vigov/comms/v1/comms.proto,
-- commit 1796286) — and ADR 0058 §3 (user, 2026-09-29) puts the automation jobs' notices HERE, not in
-- the announcement book of 0005.
--
-- NAMED IN ENGLISH (rule 12, ADR 0051): the specification's `hop_thu_thong_bao` is the design-time
-- name. The entity is `StaffNotification`, the proto message's own name, and the URL noun is
-- `notifications` — the third meaning of `thông báo` in kb/00-foundation/ubiquitous-language.md:151.
-- The spec's `nguoi_dung_id` becomes `recipient_code`, a STAFF BUSINESS CODE (rule 6, invariant 8):
-- the inbox is read by (commune, the code the session carries), with no lookup in between, and the
-- contract addresses recipients by code.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. HOW MANY ROWS PER COMMUNE: zero today. In steady state about one due-soon digest per person per
--      working day, one overdue notice per late record per day, a few escalations and one weekly
--      digest per leader per week: a commune of forty staff lands in the low tens of thousands a
--      year. Partitioned like every other table (ADR 0010).
--   2. IF IT STOPS HALF-WAY: one file, one transaction; every statement is IF NOT EXISTS or
--      CREATE OR REPLACE, so a retry costs nothing.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. WHICH READ PATHS CHANGE MEANING WHILE HALF-APPLIED: none. A new table, a new function, a new
--      trigger and two indexes on that table. Nothing existing is altered.
--   5. RETENTION: a notice is evidence that a member of staff WAS TOLD — a late record escalated to
--      the chairman is exactly what a complaint asks about later. Soft delete only (no path writes it
--      today), hard DELETE refused by the trigger, content immutable, `read_at` one-way.
--
-- ---------------------------------------------------------------------------
-- PERSONAL DATA (rule 3). `title` and `body` are sentences the CALLER composes, and the contract
-- forbids citizen personal data in them (comms.proto, "WHAT A NOTICE MAY SAY"). Nothing here can
-- verify that: they are bounded, stored, shown — and never logged, never copied into the audit
-- delta, never put in a cache key.
-- ---------------------------------------------------------------------------

-- PostgreSQL 13 is already the floor (0003 checks it): BEFORE ... FOR EACH ROW on a partitioned table.

-- ---------------------------------------------------------------------------
-- staff_notification_guard — the floor under the application.
--
--   DELETE                 refused (rule 7, invariant 1). A hard delete would also free the
--                          (key, recipient) pair and let a retried run deliver the notice again.
--   a soft-deleted row     is a historical record; not edited (rule 7, forbidden #5).
--   everything but         who was told what, when, under which key — fixed at creation. A notice
--   read_at, updated_at,   rewritten after the person read it leaves `read_at` pointing at words
--   the soft-delete trio   they never saw.
--   read_at                NULL -> a value, once. Clearing it would put the notice back in the
--                          unread count with no event behind it.
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION staff_notification_guard() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION '%: hard delete refused', TG_TABLE_NAME
            USING HINT = 'Soft delete only (deleted_at, deleted_by, delete_reason) — rule 7, '
                         'invariant 1. A hard delete would also let a retried run deliver it again.';
    END IF;

    IF OLD.deleted_at IS NOT NULL THEN
        RAISE EXCEPTION '%: a deleted notice is not edited', TG_TABLE_NAME
            USING HINT = 'A soft-deleted row is a historical record (rule 7, forbidden #5).';
    END IF;

    IF NEW.tenant_id       IS DISTINCT FROM OLD.tenant_id
    OR NEW.id              IS DISTINCT FROM OLD.id
    OR NEW.recipient_code  IS DISTINCT FROM OLD.recipient_code
    OR NEW.idempotency_key IS DISTINCT FROM OLD.idempotency_key
    OR NEW.kind            IS DISTINCT FROM OLD.kind
    OR NEW.title           IS DISTINCT FROM OLD.title
    OR NEW.body            IS DISTINCT FROM OLD.body
    OR NEW.link            IS DISTINCT FROM OLD.link
    OR NEW.created_at      IS DISTINCT FROM OLD.created_at THEN
        RAISE EXCEPTION '%: a delivered notice is immutable', TG_TABLE_NAME
            USING HINT = 'Who was told what, and when, is fixed at delivery. Only read_at and the '
                         'soft-delete columns change.';
    END IF;

    IF OLD.read_at IS NOT NULL AND NEW.read_at IS DISTINCT FROM OLD.read_at THEN
        RAISE EXCEPTION '%: read_at cannot be cleared or moved', TG_TABLE_NAME
            USING HINT = 'A person read this notice. Clearing it would raise the unread count with '
                         'no event behind it.';
    END IF;

    RETURN NEW;
END $$;

-- ---------------------------------------------------------------------------
-- @entity: StaffNotification
-- @scope:  tenant
--
-- staff_notification — one notice in ONE staff member's bell, in ONE commune.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS staff_notification (
    tenant_id       TEXT        NOT NULL,
    id              TEXT        NOT NULL,          -- ULID, internal — what PATCH addresses

    -- `CB-…`, nguoi_dung.ma of identity. NO FOREIGN KEY: the staff directory is identity's (rule 2).
    -- A code naming nobody here yields a row nobody here can read (comms.proto, "WHO MAY BE ADDRESSED").
    recipient_code  TEXT        NOT NULL,

    -- The caller's key (comms.proto recipes). Opaque here.
    idempotency_key TEXT        NOT NULL,

    -- ADR 0011: Vietnamese without diacritics. comms maps the proto enum onto these.
    kind            TEXT        NOT NULL,

    title           TEXT        NOT NULL,
    body            TEXT        NOT NULL DEFAULT '',
    -- A RELATIVE web-admin path, or '' for "leads nowhere". The CHECK is the floor under the
    -- application's parse: a stored absolute URL is an open redirect in every recipient's bell.
    link            TEXT        NOT NULL DEFAULT '',

    read_at         TIMESTAMPTZ,

    deleted_at      TIMESTAMPTZ,
    deleted_by      TEXT,
    delete_reason   TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (tenant_id, id),

    -- AT MOST ONCE PER (commune, key, recipient) — rule 2, invariant 5, and the contract's own
    -- sentence. Composite with tenant_id (rule 1, invariant 6). A plain key that COUNTS SOFT-DELETED
    -- ROWS: a notice somebody retired must not be delivered again by the next retry.
    UNIQUE (tenant_id, idempotency_key, recipient_code),

    CONSTRAINT staff_notification_kind_known
        CHECK (kind IN ('sap-den-han', 'qua-han', 'leo-thang', 'ban-tin-tuan')),
    CONSTRAINT staff_notification_recipient_code_length
        CHECK (char_length(recipient_code) BETWEEN 1 AND 64),
    CONSTRAINT staff_notification_idempotency_key_length
        CHECK (char_length(idempotency_key) BETWEEN 1 AND 200),
    CONSTRAINT staff_notification_title_length
        CHECK (btrim(title) <> '' AND char_length(title) <= 200),
    CONSTRAINT staff_notification_body_length
        CHECK (char_length(body) <= 500),
    CONSTRAINT staff_notification_link_relative
        CHECK (link = '' OR (char_length(link) <= 300 AND left(link, 1) = '/'
                             AND left(link, 2) <> '//' AND position('\' in link) = 0)),
    -- Rule 7, invariant 1: all three soft-delete columns, or none.
    CONSTRAINT staff_notification_soft_delete_complete
        CHECK ((deleted_at IS NULL) = (deleted_by IS NULL)
               AND (deleted_at IS NULL) = (delete_reason IS NULL))
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS staff_notification_p%s PARTITION OF staff_notification '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

-- The bell's list: one person's notices, newest first, the cursor's tie-break last (core/store.QueryPage
-- orders by (created_at, id)). Soft-deleted rows excluded everywhere (rule 7, invariant 2).
CREATE INDEX IF NOT EXISTS staff_notification_inbox
    ON staff_notification (tenant_id, recipient_code, created_at DESC, id DESC)
    WHERE deleted_at IS NULL;

-- The badge: `Thông báo — N thông báo chưa đọc`, asked on every page load. Only the unread rows —
-- the small half by construction. A PLAIN index, not UNIQUE, so its predicate frees no key.
CREATE INDEX IF NOT EXISTS staff_notification_unread
    ON staff_notification (tenant_id, recipient_code)
    WHERE read_at IS NULL AND deleted_at IS NULL;

DROP TRIGGER IF EXISTS staff_notification_guard ON staff_notification;
CREATE TRIGGER staff_notification_guard
    BEFORE UPDATE OR DELETE ON staff_notification
    FOR EACH ROW EXECUTE FUNCTION staff_notification_guard();

-- ---------------------------------------------------------------------------
-- WHAT THIS FILE DOES NOT STOP: TRUNCATE and DDL by the table owner — the same line 0003 and ADR 0013
-- draw.
--
-- REVERSAL (question 3). While the table is empty the reversal is complete: drop the table
-- (partitions, indexes and trigger go with it), then the function, and in the same transaction
-- remove this file's row from `schema_migration`. Once a commune has rows here it is the destruction
-- of the record that staff were told — rule 7's first stop condition, which needs the user.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- BACKSTOP: every partitioned table in this schema must actually have partitions (0002, §BACKSTOP).
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
