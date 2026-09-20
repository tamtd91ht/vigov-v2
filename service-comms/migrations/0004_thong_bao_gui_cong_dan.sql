-- comms — sổ thông báo gửi công dân: the ledger of what this commune told its citizens.
--
-- WHY THIS TABLE EXISTS AT ALL. Rule 10, invariant 5: every status transition notifies the
-- citizen and leaves an audit entry. A notification that is sent and not recorded cannot be
-- distinguished, three months later, from one that was never sent — and the question always
-- arrives from the other side ("nobody told me"), where the authority is the one that has to
-- answer. `skills/petition-lifecycle` states it plainly: silence is how trust in the channel
-- dies, and a channel nobody trusts stops receiving the reports the commune actually needs.
--
-- WHY A NEW FILE AND NOT AN EDIT OF 0001: 0001, 0002 and 0003 have been applied and
-- core/migrate compares the checksum of every applied file at startup. Editing an applied file
-- either stops the service (ErrChecksumLech) or leaves two databases claiming one schema
-- version while holding two different schemas.
--
-- ---------------------------------------------------------------------------
-- THE NAME, AND WHY IT IS NOT `thong_bao`. Read this before renaming anything.
--
-- `thong_bao` IS ALREADY TAKEN, by a different module, in this same service. Three different
-- things share that one Vietnamese word (kb/00-foundation/ubiquitous-language.md:148 —
-- `announcements` · `public-notices` · `notifications`), and the specification names the table
-- for the FIRST of them: docs/ui-ux/08-thong-bao.md §6 declares `thong_bao`,
-- `thong_bao_bo_phan`, `thong_bao_nguoi_nhan` for the INTERNAL staff announcement module,
-- guarded by `announcement.create` (§9.1). The header bell is a fourth table again,
-- `hop_thu_thong_bao` (§8).
--
-- This table is none of those. It records a message sent OUT OF THE COMMUNE, to ONE CITIZEN,
-- about THAT CITIZEN'S OWN RECORD, through an external channel. Taking the bare noun for it is
-- exactly the failure that mapping row warns about: "gộp một danh từ thì thông báo nội bộ chạy
-- sang kênh công dân" — one table name, two audiences, and an internal notice one join away
-- from the citizen channel.
--
-- THE URL NOUN FOR THIS CONCEPT IS NOT SETTLED AND IS NOT DECIDED HERE. `notifications` in
-- that mapping row is the header bell (`skills/rest-api-design` §2 spells it out). This concept
-- has NO row in the table that owns URL nouns, and both that file and rule-5's stop conditions
-- say the same thing about that situation: stop and ask, never translate on the spot. So this
-- migration ships the table and the store; no HTTP route is mounted. A path cannot be taken
-- back once a commune is live, and a table name can — while the table is still empty.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS, answered before the SQL.
--
--   1. HOW MANY ROWS PER COMMUNE: zero today — this file writes no row, and nothing writes one
--      until a producer exists (see WHAT IS DELIBERATELY ABSENT). In steady state the ceiling
--      is roughly (petitions per year) × (notifying transitions per petition, which is three:
--      intake, processing, closing — `skills/petition-lifecycle`), so a commune taking a few
--      thousand petitions a year writes low tens of thousands of rows a year. That is why the
--      table is partitioned and why the read path is indexed by business record rather than
--      scanned.
--   2. IF IT STOPS HALF-WAY: it cannot land half-applied. One file, one transaction, with the
--      progress row written inside it. Every statement is IF NOT EXISTS or CREATE OR REPLACE,
--      so a retry costs nothing.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. WHICH READ PATHS CHANGE MEANING WHILE IT IS HALF-APPLIED: none. This file only ADDS a
--      table, a function and a trigger. No existing table, column, index or constraint is
--      touched. The risk moves to the NEXT change — the first one that writes rows here.
--   5. RETENTION: nothing is ever removed. This ledger is the evidence that a commitment to a
--      citizen was kept, so it follows the audit trail's retention rather than a cache's: rows
--      are soft deleted, hard DELETE is refused by a trigger, and the fields that say WHAT was
--      told to WHOM about WHICH record are immutable after insert.
--
-- ---------------------------------------------------------------------------
-- PERSONAL DATA — THE THING THIS TABLE IS MOST LIKELY TO GET WRONG, so it is settled by the
-- schema rather than by discipline.
--
-- A notification ledger is the classic place a whole commune's phone numbers end up in one
-- file: the obvious design stores the recipient's number and the rendered message, and the
-- rendered message of a petition notification quotes the petition. Under Decree 13/2023 a
-- phone number, a name and the contents of a petition are all personal data (rule 3), and this
-- table is backed up, replicated and read by whoever debugs the send path.
--
-- So THREE columns that the obvious design would have are deliberately not here:
--
--   NOT HERE        WHY, AND WHAT IS HERE INSTEAD
--   the raw phone   `nguoi_nhan_ma` holds the OPAQUE citizen identity id, which identifies the
--                   recipient exactly and means nothing to a reader of a backup. The adapter
--                   resolves the real number at send time, holds it in memory for one call and
--                   never writes it back.
--   a readable      `nguoi_nhan_che` holds the MASKED number (core/privacy.MaskPhone —
--   phone for       09****0000) and exists only so a member of staff can see WHICH number was
--   staff           told. It is NULL until the send is attempted, because the fact is not known
--                   before then. A CHECK below refuses any value with no mask character in it:
--                   a raw ten-digit number has none, so the constraint catches the exact
--                   mistake it exists for, in the database, on every path including psql.
--   the rendered    `mau_ma` + `tham_so` hold the approved ZNS template and EXACTLY the
--   message text    parameters declared in the domain type (rule 3, invariant 6: only the
--                   fields actually needed, declared in the adapter). None of those parameters
--                   is personal data — see the comment on `tham_so`.
--
-- GET THIS WRONG AND THE CONSEQUENCE IS NOT A BUG REPORT. It is an entire commune's phone
-- numbers, permanently, in backups and log aggregation nobody on this project controls, with
-- no lawful basis for the processing. It cannot be undone by deleting a column later: the
-- copies are already made.
--
-- ---------------------------------------------------------------------------
-- WHAT IS DELIBERATELY ABSENT, so its absence does not read as unfinished work.
--
--   NO PRODUCER. Nothing publishes into this ledger yet. `petitions` owns the status
--   transitions, `comms` owns the channel, and the fact that has to travel between them is an
--   INTER-SERVICE CONTRACT — rule 2, and `.proto` is its source of truth. Inventing an event
--   name here would put one half of a contract in a migration, where no consumer can see it.
--
--   NO ZALO CALL, NO CREDENTIAL. The adapter is a separate piece of work. ADR 0006 consequence
--   2 puts each commune's OA key in the secret store, never in source and never in a process
--   environment variable (rule 8; rule 11, invariant 6).
--
--   NO STAFF NOTIFICATION. README lists notifications "to citizens and staff"; this table is
--   the CITIZEN half only, for the reason argued under THE NAME above.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- PostgreSQL 13 is the floor, checked explicitly so the failure is readable.
--
-- BEFORE ... FOR EACH ROW triggers on a PARTITIONED table were only allowed from PostgreSQL 13.
-- On 11 and 12 the CREATE TRIGGER below fails with "Partitioned tables cannot have BEFORE /
-- FOR EACH ROW triggers", which reads like a syntax mistake and invites somebody to "fix" it by
-- moving the trigger down onto the partitions — where a partition added later arrives silently
-- unprotected.
-- ---------------------------------------------------------------------------
DO $$ BEGIN
    IF current_setting('server_version_num')::int < 130000 THEN
        RAISE EXCEPTION
            'the citizen notification ledger needs PostgreSQL 13 or newer (server is %). Do '
            'not weaken this migration to fit an older server — the trigger below is what '
            'stops the record of a commitment to a citizen from being rewritten.',
            current_setting('server_version');
    END IF;
END $$;

-- ---------------------------------------------------------------------------
-- @entity: CitizenNotification
-- @scope:  tenant
--
-- thong_bao_gui_cong_dan — one row per notification this commune owes, or owed, one citizen.
--
-- A ROW IS CREATED WHEN THE OBLIGATION ARISES, NOT WHEN THE MESSAGE LEAVES. That ordering is
-- the point of the table and it follows ADR 0006 consequence 3 and 4: sending is EVENTUALLY
-- consistent with the business write, a petition is never refused because a message could not
-- go out, and a commune with no OA configured must degrade VISIBLY — staff see a "chưa báo
-- được" flag, never silence. A table that only recorded successful sends could not carry either
-- half: there would be nothing to show and nothing to retry.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS thong_bao_gui_cong_dan (
    tenant_id       TEXT        NOT NULL,
    id              TEXT        NOT NULL,          -- ULID, internal

    -- THE IDEMPOTENCY KEY, AND IT IS A KEY OF THE FACT, NOT OF THE MESSAGE.
    --
    -- Queues deliver at least once (rule 2, invariant 5), so the same fact WILL arrive twice.
    -- Deduplicating on the message id only stops a redelivery of that one message; it does not
    -- stop a producer that crashed after publishing and republished the same transition under a
    -- fresh id. The citizen feels the difference — they get the same SMS twice — so the key is
    -- built from the business fact: which record, which transition, which occurrence of it,
    -- which recipient, which channel. See domain.KhoaLanGui for the exact composition and for
    -- why `lan` is in it.
    khoa_lan_gui    TEXT        NOT NULL,

    -- WHICH BUSINESS RECORD. `doi_tuong_ma` is the BUSINESS code — the citizen's lookup code —
    -- never an internal id: the same discipline core/audit.Entry.Subject carries, and for the
    -- same reason. Whoever reads this ledger during a complaint is holding a piece of paper
    -- with that code on it and nothing else.
    doi_tuong_loai  TEXT        NOT NULL,
    doi_tuong_ma    TEXT        NOT NULL,

    -- WHICH TRANSITION, STORED AS AN OPAQUE VALUE. This is a petition status code today
    -- (`da-tiep-nhan`, `dang-xu-ly`, `da-dong` — the closed list of ADR 0027), and there is
    -- deliberately NO CHECK constraint listing them, unlike the columns below.
    --
    -- The list belongs to `petitions`. A copy of it here would be a second source for one fact
    -- (rule 9) living on the far side of a service boundary (rule 2), and the copy would be
    -- discovered to have drifted on the day a transition stopped notifying anybody — silently,
    -- because a status this service has never heard of would simply fail a constraint inside a
    -- consumer nobody is watching. Storing it as a value and validating nothing is the shape
    -- `petitions` already uses for `linh_vuc` across the same kind of boundary (ADR 0026).
    moc             TEXT        NOT NULL,

    -- WHICH OCCURRENCE of that transition on that record: 1 the first time, 2 after a reopening
    -- puts the petition back through it (ADR 0008 allows a citizen to reopen). Without it the
    -- idempotency key would make the SECOND legitimate closing notification look like a
    -- duplicate of the first and swallow it — a citizen told once about two different closings.
    lan             SMALLINT    NOT NULL DEFAULT 1,

    kenh            TEXT        NOT NULL,

    -- WHO IT WAS SENT TO. Read the PERSONAL DATA block in the header before adding anything
    -- here: `nguoi_nhan_ma` is opaque, `nguoi_nhan_che` is masked, and there is no third column.
    nguoi_nhan_ma   TEXT        NOT NULL,
    nguoi_nhan_che  TEXT,

    -- WHAT WAS SAID, as a template reference plus its declared parameters — never as rendered
    -- text. The parameter set is fixed by domain.ThamSoThongBao and is exactly three values,
    -- none of which is personal data:
    --
    --   ma_tra_cuu      the citizen's own lookup code (rule 10, invariant 1)
    --   moc_nhan        what changed, in words a citizen reads
    --   viec_tiep_theo  what happens next
    --
    -- Rule 10, invariant 6 and `skills/petition-lifecycle` both refuse a bare "Đã xử lý": a
    -- notification that names a state and not a consequence tells the citizen nothing they can
    -- act on. The refusal is implemented in the domain type, where it can carry an explanation;
    -- the CHECK below only holds the part SQL can hold.
    tham_so         JSONB       NOT NULL,
    mau_ma          TEXT        NOT NULL,

    trang_thai      TEXT        NOT NULL DEFAULT 'cho-gui',
    so_lan_thu      SMALLINT    NOT NULL DEFAULT 0,
    gui_luc         TIMESTAMPTZ,

    -- THE PROVIDER'S ERROR CODE, NEVER ITS MESSAGE. A message from a messaging gateway quotes
    -- the request back, and the request carries the recipient's number (rule 3, forbidden #3).
    loi_ma          TEXT,

    -- SOFT DELETE, although in practice nothing should ever delete a row here. The columns are
    -- present because rule 7, invariant 1 makes them the shape of business data and because
    -- `delete_reason` forces whoever does it to say why; the trigger below refuses the hard
    -- DELETE that would otherwise be the easy path.
    deleted_at      TIMESTAMPTZ,
    deleted_by      TEXT,
    delete_reason   TEXT,

    tao_luc         TIMESTAMPTZ NOT NULL DEFAULT now(),
    cap_nhat_luc    TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (tenant_id, id),

    -- COMPOSITE WITH tenant_id, like every unique key in this system (rule 1, invariant 6). It
    -- is also the second layer of duplicate protection: a consumer that forgets to check still
    -- hits this constraint, and `ON CONFLICT DO NOTHING` in the store turns it into the correct
    -- no-op rather than an error (`skills/rest-api-design` §4, "second layer where it is free").
    UNIQUE (tenant_id, khoa_lan_gui),

    CONSTRAINT thong_bao_gui_cong_dan_doi_tuong_hop_le
        CHECK (doi_tuong_loai IN ('phieu-phan-anh')),

    -- ONE CHANNEL TODAY, AND A CHECK RATHER THAN A FREE-TEXT COLUMN. An unconstrained
    -- discriminator is how `zalo-zns`, `zns` and `zalo_zns` end up as three values for one
    -- channel across three code paths, after which every count by channel is wrong and no
    -- single query shows it. Adding a channel is one line in a new migration, which is the
    -- correct amount of friction for a decision that changes what a citizen receives.
    CONSTRAINT thong_bao_gui_cong_dan_kenh_hop_le
        CHECK (kenh IN ('zalo-zns')),

    -- THE FOUR STATES, AND THE FOURTH IS NOT A CURIOSITY.
    --
    --   cho-gui             the obligation is recorded, the message has not gone out
    --   da-gui              the channel accepted it
    --   that-bai            the channel refused it, retries exhausted — staff must see this
    --   chua-cau-hinh-kenh  THE COMMUNE HAS NO OA CONFIGURED. ADR 0006 consequence 3 requires
    --                       this to degrade VISIBLY: a newly onboarded commune can take
    --                       petitions before its OA is approved, and the citizens of that
    --                       commune are then owed notifications nobody can send. Folding it
    --                       into `that-bai` would put it in the retry queue forever; leaving it
    --                       as `cho-gui` would make it invisible. It is a distinct, terminal,
    --                       reportable state.
    CONSTRAINT thong_bao_gui_cong_dan_trang_thai_hop_le
        CHECK (trang_thai IN ('cho-gui', 'da-gui', 'that-bai', 'chua-cau-hinh-kenh')),

    -- "SENT" WITHOUT A TIME IS A CLAIM WITH NO DATE ON IT, which is worth nothing to the
    -- inspection that asks when the citizen was told.
    CONSTRAINT thong_bao_gui_cong_dan_da_gui_thi_co_moc_gio
        CHECK (trang_thai <> 'da-gui' OR gui_luc IS NOT NULL),

    -- THE MASK IS ENFORCED BY THE DATABASE, not by remembering to call MaskPhone. A masked
    -- number always contains the mask character; a raw Vietnamese mobile number never does. So
    -- this constraint refuses exactly the write it exists to refuse, on every path — including
    -- an INSERT typed at a psql prompt during an incident, which is when the mistake is most
    -- likely to be made. See the PERSONAL DATA block in the header for what it costs to get
    -- this wrong once.
    CONSTRAINT thong_bao_gui_cong_dan_nguoi_nhan_phai_che
        CHECK (nguoi_nhan_che IS NULL OR nguoi_nhan_che LIKE '%*%'),

    -- `tham_so` MUST CARRY THE THREE DECLARED PARAMETERS AND MUST AGREE WITH THE ROW. The
    -- lookup code appears twice — as `doi_tuong_ma` and as a template parameter — because the
    -- template's parameter set is what Zalo approved and the adapter may not rebuild it from
    -- another column. Two copies of one fact are safe only when they cannot drift, so they are
    -- tied together here rather than left to agree by convention (rule 9, forbidden #2).
    CONSTRAINT thong_bao_gui_cong_dan_tham_so_day_du
        CHECK (tham_so ? 'ma_tra_cuu' AND tham_so ? 'moc_nhan' AND tham_so ? 'viec_tiep_theo'
               AND tham_so ->> 'ma_tra_cuu' = doi_tuong_ma),

    CONSTRAINT thong_bao_gui_cong_dan_lan_duong
        CHECK (lan >= 1),
    CONSTRAINT thong_bao_gui_cong_dan_so_lan_thu_khong_am
        CHECK (so_lan_thu >= 0)
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS thong_bao_gui_cong_dan_p%s PARTITION OF thong_bao_gui_cong_dan '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

-- THE READ PATH THIS TABLE EXISTS TO SERVE: "what has this commune told the citizen about THIS
-- record, and when". Newest first, because that is the question's real shape — the last thing
-- the citizen was told is what they are holding when they call.
CREATE INDEX IF NOT EXISTS thong_bao_gui_cong_dan_theo_ho_so
    ON thong_bao_gui_cong_dan (tenant_id, doi_tuong_loai, doi_tuong_ma, tao_luc DESC)
    WHERE deleted_at IS NULL;

-- THE SECOND READ PATH IS THE ONE ADR 0006 CONSEQUENCE 3 DEMANDS: everything not yet delivered.
-- `da-gui` rows are excluded from the index itself, so the set stays small however large the
-- ledger grows — which is what makes "chưa báo được" affordable to show on a screen rather than
-- a report somebody runs monthly.
CREATE INDEX IF NOT EXISTS thong_bao_gui_cong_dan_chua_bao_duoc
    ON thong_bao_gui_cong_dan (tenant_id, trang_thai, tao_luc)
    WHERE deleted_at IS NULL AND trang_thai <> 'da-gui';

-- ---------------------------------------------------------------------------
-- The immutability guard. NOT the same function as `danh_muc_ba_tang` from 0003: that one
-- encodes the three tiers of a reference catalogue, which has nothing to do with this table.
-- Sharing it would mean one function whose branches apply to different tables, which is how a
-- guard ends up being weakened for one caller and silently weakened for the other.
--
-- WHAT EACH REFUSAL IS FOR, in the order they are checked:
--
--   DELETE             this row is the evidence that a commitment to a citizen was met, or the
--                      evidence that it was not. Rule 7, invariant 1: soft delete only.
--   the FIVE identity  which record · which transition · which occurrence · who · which
--   columns            channel. Changing any of them turns a true statement about one citizen
--                      into a false statement about another, and nothing downstream could tell.
--                      This is the clause that makes the ledger evidence rather than a cache.
--   `tham_so`,         what was actually said. An edit here rewrites history: the citizen
--   `mau_ma`           received the old text.
--   `tao_luc`          when the obligation arose. It is the origin of every "how long did the
--                      commune take to tell them" figure.
--   leaving `da-gui`   a message the channel accepted cannot become unsent. Allowing it would
--                      let a failed retry downgrade a successful send and produce a duplicate.
--
-- WHAT STAYS EDITABLE, on purpose: `trang_thai` (forward only), `so_lan_thu`, `gui_luc`,
-- `loi_ma`, `nguoi_nhan_che`, `cap_nhat_luc`, and the soft-delete columns. Those are the
-- OUTCOME of the send, which is not known when the row is written.
--
-- The messages name the operation and the column and nothing else: an error message travels
-- into logs and back to clients (rule 3, forbidden #3).
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION thong_bao_gui_cong_dan_bat_bien() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'thong_bao_gui_cong_dan: hard delete refused'
            USING HINT = 'This row is the record that a commitment to a citizen was met, or '
                         'that it was not (rule 10, invariant 5). Soft delete only: '
                         'deleted_at, deleted_by, delete_reason — rule 7, invariant 1.';
    END IF;

    IF NEW.khoa_lan_gui  IS DISTINCT FROM OLD.khoa_lan_gui
    OR NEW.doi_tuong_loai IS DISTINCT FROM OLD.doi_tuong_loai
    OR NEW.doi_tuong_ma  IS DISTINCT FROM OLD.doi_tuong_ma
    OR NEW.moc           IS DISTINCT FROM OLD.moc
    OR NEW.lan           IS DISTINCT FROM OLD.lan
    OR NEW.kenh          IS DISTINCT FROM OLD.kenh
    OR NEW.nguoi_nhan_ma IS DISTINCT FROM OLD.nguoi_nhan_ma THEN
        RAISE EXCEPTION 'thong_bao_gui_cong_dan: identity columns are immutable'
            USING HINT = 'Which record, which transition, which occurrence, who and through '
                         'which channel are fixed when the obligation arises. Editing one '
                         'turns a true statement about one citizen into a false statement '
                         'about another. Record a NEW row instead.';
    END IF;

    IF NEW.tham_so IS DISTINCT FROM OLD.tham_so
    OR NEW.mau_ma  IS DISTINCT FROM OLD.mau_ma
    OR NEW.tao_luc IS DISTINCT FROM OLD.tao_luc THEN
        RAISE EXCEPTION 'thong_bao_gui_cong_dan: what was said, and when it was owed, are immutable'
            USING HINT = 'The citizen received the old wording. Rewriting it here rewrites '
                         'history (rule 7, forbidden #5).';
    END IF;

    IF OLD.trang_thai = 'da-gui' AND NEW.trang_thai <> 'da-gui' THEN
        RAISE EXCEPTION 'thong_bao_gui_cong_dan: a delivered notification cannot become undelivered'
            USING HINT = 'The channel accepted this message and the citizen has it. Moving the '
                         'row back would put it into the retry set and send it a second time.';
    END IF;

    RETURN NEW;
END $$;

DROP TRIGGER IF EXISTS thong_bao_gui_cong_dan_bat_bien ON thong_bao_gui_cong_dan;
CREATE TRIGGER thong_bao_gui_cong_dan_bat_bien
    BEFORE UPDATE OR DELETE ON thong_bao_gui_cong_dan
    FOR EACH ROW EXECUTE FUNCTION thong_bao_gui_cong_dan_bat_bien();

-- ---------------------------------------------------------------------------
-- WHAT THIS FILE DOES NOT STOP, said plainly rather than left to be discovered: TRUNCATE, and
-- DDL by the table owner (ALTER TABLE ... DISABLE TRIGGER, dropping the table). Same line
-- 0002 and 0003 draw — the job is to make the accidental and the convenient impossible, not to
-- defeat an administrator who has decided to destroy data and is willing to be seen doing it.
--
-- TRUNCATE is left uncovered here where 0002 covers it for audit_log, and the difference is
-- deliberate rather than an omission: 0002's per-partition TRUNCATE triggers have to be
-- re-applied whenever a partition is added, and that file says so. Repeating the mechanism
-- would repeat the same maintenance obligation for a second table. If this ledger is later
-- judged to need it, the block in 0002 is the form to copy — and it belongs in a new migration
-- alongside a decision about whether the same should apply to every business table.
--
-- REVERSAL (migration question 3). Every object here is new, and while the table is still empty
-- the reversal is complete and loses nothing: drop the table, then the trigger function, and in
-- the same transaction remove this file's row from `schema_migration`, otherwise the runner
-- still believes the schema is in place. The 32 partitions, the two indexes and the trigger go
-- with the parent table.
--
-- ONCE A COMMUNE HAS ROWS HERE, THAT IS NO LONGER A REVERSAL — it is the destruction of the
-- record of what a public authority told its citizens, which is rule 7's first stop condition
-- and needs the user, not a command. From that point the way back is a NEW migration, and
-- core/migrate has no automatic rollback for exactly this reason (ADR 0013).
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- BACKSTOP: every partitioned table in this schema must actually have partitions.
--
-- A table declared PARTITION BY and given no partitions REJECTS EVERY INSERT, silently, until
-- the first real write — and because the audit entry shares the business transaction (rule 6,
-- invariant 3), that first write rolls back entirely. The check is repeated at the end of every
-- migration that declares a partitioned table, because it only verifies the state after a file
-- that CARRIES it (0002, §BACKSTOP).
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
