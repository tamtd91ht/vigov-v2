-- identity — the citizen channel: identity, commune relationship, session, session pairing.
--
-- WHY A NEW FILE AND NOT AN EDIT OF 0001: 0001 has been applied and core/migrate compares the
-- checksum of every applied file at startup. Editing an applied file either stops the service
-- (ErrChecksumLech) or, where it does not, leaves two databases claiming one schema version
-- while holding two different schemas.
--
-- WHY ALL FOUR TABLES IN ONE FILE: core/migrate gives each file exactly ONE transaction. These
-- four tables reference each other (relationship -> identity, session -> identity, pairing ->
-- session), so splitting them across files would create an intermediate state in which a
-- foreign key has no target. All four, or none.
--
-- ENTITY OWNERSHIP is declared with the `-- @entity` / `-- @scope` marks above each CREATE
-- TABLE, per ADR 0021. The generator that reads them does not exist yet; the marks are written
-- now because the moment a table is born is the only moment the answer is certain.
--
-- THE FIVE MIGRATION QUESTIONS, answered before the SQL:
--
--   1. HOW MANY ROWS PER COMMUNE: zero. Every table here is new; no commune has a citizen
--      channel today. The ceiling in sight is one identity row per citizen platform-wide and
--      one relationship row per (citizen, commune) pair.
--   2. IF IT STOPS HALF-WAY: it cannot land half-applied. One file, one transaction, with the
--      progress row written inside it. A failure leaves nothing behind and the next start
--      retries from the beginning. Every statement is IF NOT EXISTS, so a retry costs nothing.
--   3. HOW IT IS REVERSED: see "REVERSAL" at the bottom.
--   4. WHICH READ PATHS CHANGE MEANING WHILE IT IS HALF-APPLIED: none. This file only ADDS
--      tables. No existing table, column, index or constraint is touched, so every query that
--      runs today returns exactly what it returned before, and no row changes visibility.
--      This is the question that causes incidents, and here the honest answer is that the risk
--      moves to the NEXT change — the first one that writes rows into these tables.
--   5. RETENTION: nothing is removed. The tables created here will hold archival material
--      (the commune relationship is the basis of a citizen's access to their own files), so
--      soft-delete columns are present from the start where the concept applies — see the
--      note on dinh_danh_cong_dan for the one place it deliberately does not.
--
-- PERSONAL DATA (rule 3, ADR 0020 invariant 1): the phone number lives in this schema and
-- NOWHERE ELSE in it. It is not in an index name, not in a constraint name, not in an example,
-- and there is no seed data in this file at all.

-- ---------------------------------------------------------------------------
-- dinh_danh_cong_dan — one citizen, platform-wide.
--
-- THE SECOND TABLE IN THIS SERVICE WITHOUT tenant_id, AND FOR A COMPLETELY DIFFERENT REASON
-- THAN THE FIRST. `quyen` has no tenant_id because the set of rights the software can enforce
-- is a property of the SOFTWARE. This table has none because ADR 0002 decided that a phone
-- number is ONE RECORD FOR THE WHOLE PLATFORM: a citizen deals with several communes at once —
-- registered residence in one, temporary residence in another, a report about a pothole seen
-- while passing through a third. Key the phone per commune and a citizen is confined to
-- exactly one commune forever, which is wrong in real life in at least four ways.
--
-- WHY THE TABLE IS CALLED `dinh_danh_cong_dan` AND NOT `cong_dan`. It is a deliberate refusal,
-- not a clumsy name. This record holds exactly two things: an opaque identifier and a verified
-- phone number. A table called `cong_dan` is an open invitation for the next person to add
-- `ho_ten`, `so_cccd`, `ngay_sinh` — and this is the one region of the schema with NO tenant_id
-- to contain the damage. Every heaviest obligation of Decree 13/2023 would land here at once:
-- a single table holding identifying data for citizens of every commune on the platform, with
-- no commune boundary to limit who a mistaken query returns. The name states what the record is
-- so the next person has to argue with it rather than drift past it.
--
-- WHAT DOES NOT GO HERE, for the avoidance of doubt: full name, national ID number, date of
-- birth, address, home coordinates, family relationships. Those belong to a business record
-- inside a commune, scoped by tenant_id, where a wrong query reaches one commune's citizens
-- instead of all of them.
--
-- UNIQUE ON THE PHONE NUMBER, PLATFORM-WIDE, WITH NO tenant_id — and that is the decision of
-- ADR 0002, not a breach of rule 1 forbidden #4. That rule forbids a single-column unique key
-- on the BUSINESS DATA OF A COMMUNE, because the second commune onboarding then collides.
-- Here the uniqueness is the point: two rows for one phone number means one human being seen
-- as two people, and the citizen loses sight of files they themselves filed. Same shape as
-- `tenant_domain.host` in service-platform — a global unique key that MAKES the model work
-- rather than breaking it.
--
-- THE NUMBER IS STORED AS IT IS, NOT ENCRYPTED, AND THAT IS A GAP THAT MUST BE NAMED. ADR 0009
-- is per-commune envelope encryption: one DEK per commune, so leaking one commune leaks one
-- commune. This table belongs to no commune, so there is no commune whose DEK would wrap it,
-- and the policy DOES NOT APPLY HERE — it is not merely unimplemented. Inventing a
-- platform-level key on the spot would be a security mechanism designed by whoever happened to
-- write this migration, which is worse than a stated gap. A deterministic hash is not an
-- option either: the uniqueness check needs to find the row from the number, and ADR 0020
-- requires the real number to exist so it can be masked on the way out (MaskPhone) — a hash
-- cannot be masked, it can only be absent. So: plain text, and the protections that do hold
-- are rule 3 (never logged, masked at the API) plus database-level access control.
--
-- NO deleted_at / deleted_by / delete_reason ON THIS TABLE, ON PURPOSE. Every citizen business
-- record in the system points at this row; soft-deleting it would leave archival records
-- pointing at a subject the system has agreed to stop returning, which is the same damage as
-- deleting them. An erasure request under Decree 13/2023 is ANONYMISATION (rule 7, invariant 7):
-- replace the phone number, keep the row and its id, so the business records keep a subject.
-- THAT MECHANISM IS DELIBERATELY NOT BUILT HERE — it needs a decision about what the number is
-- replaced with and who may request it, and neither has been made. Room is left for it by the
-- shape of the table (the id never changes; only the number would), not by an unused column.
-- ---------------------------------------------------------------------------
-- @entity: CitizenIdentity
-- @scope:  cross-tenant
CREATE TABLE IF NOT EXISTS dinh_danh_cong_dan (
    -- Opaque ULID. Never derived from the phone number: an identifier that can be reversed
    -- into personal data is personal data, and this one appears in audit entries (ADR 0020,
    -- invariant 5 — log the citizen id, never the number).
    id             TEXT        PRIMARY KEY,
    -- Verified by Zalo's getPhoneNumber (ADR 0020). PERSONAL DATA — rule 3.
    so_dien_thoai  TEXT        NOT NULL,
    tao_luc        TIMESTAMPTZ NOT NULL DEFAULT now(),
    cap_nhat_luc   TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT dinh_danh_cong_dan_id_la_ulid CHECK (length(id) = 26),
    CONSTRAINT dinh_danh_cong_dan_so_duy_nhat UNIQUE (so_dien_thoai)
);

COMMENT ON TABLE dinh_danh_cong_dan IS
    'Mot cong dan, toan nen tang (ADR 0002). Chi giu dinh danh va so dien thoai — khong them '
    'truong nhan dang nao khac vao day (Nghi dinh 13/2023).';
COMMENT ON COLUMN dinh_danh_cong_dan.so_dien_thoai IS
    'DU LIEU CA NHAN. Khong vao log, che khi ra API (MaskPhone), khong vao URL / ten tep / '
    'khoa cache / ten phong realtime.';

-- ---------------------------------------------------------------------------
-- quan_he_cong_dan_xa — the many-to-many between a citizen and a commune.
--
-- TWO DIFFERENT FACTS, TWO COLUMNS, AND MERGING THEM IS THE DEFECT THIS TABLE EXISTS TO AVOID:
--
--   khai_cu_tru          — what the CITIZEN SAID: thuong_tru | tam_tru | chua_khai
--   trang_thai_xac_thuc  — whether a COMMUNE OFFICER HAS CHECKED IT: cho_xac_thuc | da_xac_thuc
--
-- WHY THIS IS SERIOUS ENOUGH TO SPEND A COLUMN ON. Under Luật Cư trú 2020, thường trú and tạm
-- trú are states REGISTERED WITH THE POLICE. An unverified declaration is not a legal status.
-- A report that says "1.200 công dân thường trú" will be read as 1,200 people holding a
-- registration, and it goes upward to leadership. One column carrying both facts makes that
-- figure wrong by construction, with nothing turning red: the number is produced by a correct
-- query over a column that quietly means two things.
--
-- The column is named `khai_cu_tru` — a DECLARATION — rather than `cu_tru` or `loai_cu_tru`,
-- so that anyone writing `WHERE khai_cu_tru = 'thuong_tru'` has to notice they are counting
-- claims. The name is the cheapest place to put that warning.
--
-- THE THIRD VALUE IS `chua_khai`, NOT `vang_lai`. ADR 0005 creates this row the moment a
-- citizen submits a report to a commune, and that includes a person sitting in Hà Nội
-- reporting something to a commune in Đà Nẵng. "Vãng lai" asserts the person is physically
-- present in the area. Nothing in the data supports that assertion, and a status the data
-- cannot support is a status somebody will eventually count.
--
-- `nguon_khai` RECORDS WHO PUT THE VALUE THERE — the citizen in the app, or an officer
-- recording it at the counter. Without it, a year from now the two kinds of row are
-- indistinguishable, and "the citizen claimed this" and "an officer wrote this down" are not
-- the same evidence when a dispute arrives.
--
-- A REJECTED DECLARATION GETS ITS OWN STATE AND KEEPS THE DECLARATION. It does not quietly go
-- back to `chua_khai`. After an officer rejects, the row still answers all three questions:
-- WHAT the citizen declared (khai_cu_tru, untouched), THAT it was rejected
-- (trang_thai_xac_thuc = 'tu_choi'), and WHY (ly_do_tu_choi).
--
-- Resetting to `chua_khai` would leave the citizen unable to tell "nobody has looked at this
-- yet" from "this was refused". That is the same failure rule 10, invariant 6 forbids on a
-- petition — closing without a result the citizen can read, never silently. Different surface,
-- identical principle: silence is how trust in the channel dies.
--
-- `ly_do_tu_choi` IS PROSE WRITTEN BY AN OFFICER FOR A CITIZEN TO READ, not an internal code
-- and not a staff note. It reaches the citizen's screen, which has two consequences: it is
-- written in plain Vietnamese, and it must not carry personal data — no phone number, no
-- national ID number, no document number (rule 3, forbidden #3). It describes the DECLARATION,
-- not the person.
--
-- THE LIFECYCLE THIS SHAPE SUPPORTS, and it needs no further column and no migration over
-- existing rows to walk it:
--
--     declare -> cho_xac_thuc -> da_xac_thuc
--                            \-> tu_choi -> (citizen declares again) -> cho_xac_thuc -> ...
--
-- Re-declaring writes khai_cu_tru, sets trang_thai_xac_thuc back to 'cho_xac_thuc' and clears
-- the three review columns — which the constraints below enforce as one consistent move. The
-- previous rejection and its reason are not lost by that: they are in the audit entry, which
-- is where history belongs (rule 6, invariant 5) and which is append-only.
--
-- EVERY TRANSITION HERE IS A BUSINESS WRITE. Verifying, rejecting and re-declaring each leave
-- an audit entry written IN THE SAME TRANSACTION as the change (rule 6, invariant 3) — who,
-- when, from which IP, in which commune. An officer rejecting a citizen's declaration is
-- exactly the act somebody will later be asked to account for.
--
-- STILL OPEN, AND NOT DECIDED HERE (ADR 0023, §CÒN MỞ #3): whether a citizen may edit a
-- declaration that has ALREADY been verified, and whether doing so needs a procedure. The shape
-- allows it — the declaration changes and the state returns to waiting — but whether it is
-- allowed is an administrative process question, not a schema question.
--
-- `da_xac_thuc` IS NOT LEGAL GROUNDS FOR ANYTHING, AND THIS PARAGRAPH IS THE REASON IT IS
-- WRITTEN DOWN HERE RATHER THAN ASSUMED. The customer answered this question directly: a
-- verified relationship is CONVENIENCE DATA — pre-filling a form, suggesting a commune,
-- filtering a list. Any procedure with legal consequences still runs through its own paperwork.
--
-- Why an officer's verification does not raise it above that: the officer can confirm THAT THIS
-- ACCOUNT DECLARED THIS. They cannot confirm that the person holding the phone is the person
-- named. Citizen identity is deliberately weak (rule 4) and stays weak after verification —
-- Zalo attests which account a number belongs to, never who is holding the handset (ADR 0020,
-- invariant 3). Verification raises the quality of the data; it does not change the class of
-- the identity behind it.
--
-- THE FAILURE MODE THIS PARAGRAPH EXISTS TO PREVENT: a state called "verified" sitting alone in
-- a table reads, to whoever writes the dossier module next year, as a travel pass — the check
-- has been done, so the dossier can rely on it. Nothing would turn red. The limit has to be
-- stated where that person is looking, which is here and in COMMENT ON COLUMN below, the same
-- way `quyen` states in plain words why it has no tenant_id.
--
-- NO `hieu_luc_den` (RELATIONSHIP EXPIRY), AND THIS IS A DECISION, NOT AN OVERSIGHT. ADR 0002
-- lists a term on the relationship. Three reasons not to create the column today: (a) the term
-- that exists in law belongs to the REGISTERED residence record held by the police, not to a
-- declaration made in an app, so the column would have no source of truth; (b) ADR 0002
-- attached the term to `vang_lai`, and `vang_lai` is exactly the value the customer replaced
-- with `chua_khai` — which asserts nothing about presence and therefore has nothing to expire;
-- (c) an unused expiry date invites a nightly job to "expire" relationships, and that job
-- would silently remove a citizen's access to their own files — a derived fact written into a
-- column, which is the failure rule 10 invariant 3 names. Adding the column later is additive
-- and cheap; removing a job that has already hidden somebody's file is not.
-- ---------------------------------------------------------------------------
-- @entity: CitizenCommune
-- @scope:  tenant
CREATE TABLE IF NOT EXISTS quan_he_cong_dan_xa (
    tenant_id           TEXT        NOT NULL,
    cong_dan_id         TEXT        NOT NULL REFERENCES dinh_danh_cong_dan (id),

    -- WHAT THE CITIZEN SAID. Not a legal status. See the block above before counting it.
    khai_cu_tru         TEXT        NOT NULL DEFAULT 'chua_khai',
    -- Who put that value there: the citizen in the app, or an officer at the counter.
    nguon_khai          TEXT        NOT NULL,
    khai_luc            TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- WHETHER THE COMMUNE HAS CHECKED IT. Independent of the line above, on purpose.
    -- cho_xac_thuc | da_xac_thuc | tu_choi. `da_xac_thuc` is CONVENIENCE DATA, never grounds
    -- for a procedure with legal consequences — see the block above.
    trang_thai_xac_thuc TEXT        NOT NULL DEFAULT 'cho_xac_thuc',
    -- The officer who decided, and when. Named for the act of reviewing rather than for one of
    -- its outcomes, so verification and rejection share one pair of columns instead of two
    -- pairs that would then have to be kept consistent with each other.
    xet_duyet_boi       TEXT,
    xet_duyet_luc       TIMESTAMPTZ,
    -- Set only when the declaration is rejected. PROSE THE CITIZEN READS, written by the
    -- officer — not an internal code, not a staff note. No personal data in it (rule 3).
    ly_do_tu_choi       TEXT,

    deleted_at          TIMESTAMPTZ,
    deleted_by          TEXT,
    delete_reason       TEXT,
    tao_luc             TIMESTAMPTZ NOT NULL DEFAULT now(),
    cap_nhat_luc        TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- COMPOSITE WITH tenant_id (rule 1, invariant 6). One citizen holds at most one
    -- relationship row per commune, and commune A's row can never collide with commune B's.
    PRIMARY KEY (tenant_id, cong_dan_id),

    CONSTRAINT quan_he_khai_cu_tru_hop_le
        CHECK (khai_cu_tru IN ('thuong_tru', 'tam_tru', 'chua_khai')),
    CONSTRAINT quan_he_nguon_khai_hop_le
        CHECK (nguon_khai IN ('cong_dan', 'can_bo')),
    CONSTRAINT quan_he_trang_thai_xac_thuc_hop_le
        CHECK (trang_thai_xac_thuc IN ('cho_xac_thuc', 'da_xac_thuc', 'tu_choi')),
    -- Written against "not waiting" rather than against "verified": whatever the outcome, a row
    -- that has left the queue was decided by an officer, and we record which officer and when.
    -- A further outcome added one day is then a constraint swap, not a re-think.
    CONSTRAINT quan_he_da_xet_thi_co_nguoi_xet
        CHECK ((trang_thai_xac_thuc <> 'cho_xac_thuc') = (xet_duyet_luc IS NOT NULL)
               AND (xet_duyet_luc IS NOT NULL) = (xet_duyet_boi IS NOT NULL)),
    -- A rejection ALWAYS carries a reason, and a reason exists ONLY on a rejection. The first
    -- half is the one that matters: "rejected, no reason given" is a dead end presented to a
    -- citizen, and the database refusing it is the only thing that makes it impossible rather
    -- than merely discouraged. The second half stops a stale reason surviving a re-declaration.
    CONSTRAINT quan_he_tu_choi_phai_co_ly_do
        CHECK ((trang_thai_xac_thuc = 'tu_choi') = (ly_do_tu_choi IS NOT NULL)
               AND (ly_do_tu_choi IS NULL OR btrim(ly_do_tu_choi) <> ''))
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS quan_he_cong_dan_xa_p%s PARTITION OF quan_he_cong_dan_xa '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

COMMENT ON COLUMN quan_he_cong_dan_xa.khai_cu_tru IS
    'LOI KHAI cua cong dan, KHONG phai trang thai phap ly. Thuong tru / tam tru theo Luat Cu tru '
    '2020 la trang thai DA DANG KY voi co quan cong an. Doc kem trang_thai_xac_thuc truoc khi dem.';
COMMENT ON COLUMN quan_he_cong_dan_xa.trang_thai_xac_thuc IS
    'cho_xac_thuc | da_xac_thuc | tu_choi. Doc lap voi khai_cu_tru. da_xac_thuc LA DU LIEU TIEN '
    'LOI (dien san bieu mau, goi y xa, loc danh sach) — KHONG phai can cu phap ly: can bo xac '
    'thuc duoc "nguoi nay khai the", khong xac thuc duoc "nguoi dang cam may chinh la nguoi do".';
COMMENT ON COLUMN quan_he_cong_dan_xa.ly_do_tu_choi IS
    'Van ban can bo viet CHO CONG DAN DOC khi tu choi loi khai. Khong phai ma loi noi bo, khong '
    'phai ghi chu noi bo, va khong chua du lieu ca nhan (luat 3).';

-- The staff verification queue, and the only safe basis for a residence figure: both need the
-- commune, the verification state and the declared value together.
CREATE INDEX IF NOT EXISTS quan_he_cong_dan_xa_hang_cho
    ON quan_he_cong_dan_xa (tenant_id, trang_thai_xac_thuc, khai_cu_tru)
    WHERE deleted_at IS NULL;

-- "WHICH COMMUNES IS THIS CITIZEN KNOWN TO" — deliberately NOT prefixed with tenant_id, and
-- therefore deliberately not prunable. It is the one question in this table that cannot carry a
-- commune: ADR 0002 makes a citizen many-to-many with communes, and the app has to show them
-- the list. The Go query behind it is a cross-commune read within the service ADR 0002 allows
-- one: it belongs in service-identity/internal/store/crosstenant/ (ADR 0021) and it still needs
-- its own `// @cross-tenant: <reason>` (rule 1, forbidden #6). The index is declared on the
-- parent, so PostgreSQL maintains one child index per partition and the lookup touches 32 of
-- them. That cost is accepted here because the alternative is a full scan of all 32.
CREATE INDEX IF NOT EXISTS quan_he_cong_dan_xa_theo_cong_dan
    ON quan_he_cong_dan_xa (cong_dan_id)
    WHERE deleted_at IS NULL;

-- ---------------------------------------------------------------------------
-- phien_cong_dan — the citizen session registry.
--
-- SEPARATE FROM `phien`, WHICH IS THE STAFF REGISTRY, AND THE TWO MUST NEVER MERGE. `phien` has
-- a foreign key to `nguoi_dung`; a staff identity is ACCOUNTABLE and a citizen identity is a
-- phone number, which rule 4 calls deliberately weak. One table would put two trust levels
-- behind one lookup, and the weaker one would decide the shape.
--
-- SHAPE MATCHES core/httpx.CitizenSession EXACTLY — three identifiers and no personal data:
--   ID        -> phien_cong_dan.id            (the sid)
--   CitizenID -> phien_cong_dan.cong_dan_id
--   TenantID  -> phien_cong_dan.tenant_id
--
-- tenant_id MAY BE THE EMPTY STRING, AND THAT IS A REAL ANSWER RATHER THAN A MISSING VALUE.
-- ADR 0005 separates discovery from session: a citizen signs in before choosing a commune, and
-- the screen that offers the choice is called WITH that session. core/httpx documents the same
-- thing from the other side. So: NOT NULL, and no DEFAULT — rule 1 forbidden #1 bans a default
-- on the isolation path, and '' here is written by the code that issues the session, knowingly,
-- never filled in by the schema on a row that forgot to say.
--
-- CHANGING COMMUNE ISSUES A NEW SESSION, IT DOES NOT UPDATE THIS ROW (ADR 0005: "đổi xã là hành
-- động tường minh, phát hành lại phiên, ghi vết"). That is also what keeps tenant_id immutable,
-- which matters more here than elsewhere: it is the partition key, so an update would move the
-- row between partitions and the old sid would silently keep working.
--
-- BOTH LOOKUP SHAPES ARE SUPPORTED ON PURPOSE, because which one is used is a decision for the
-- store implementation and not for this file: `bam_token` for an opaque bearer, and
-- (tenant_id, id) for a signed token that carries tid+sid. Note that core/token cannot be
-- reused as-is for citizen sessions — it refuses to sign a claim set with an empty TenantID,
-- and an empty TenantID is legitimate here.
--
-- THE TOKEN ITSELF IS NEVER STORED, only SHA-256 of it, exactly as `phien.refresh_hash` does
-- (store/phien.go:bamRefresh). A leaked backup must not hand over working sessions.
--
-- "ĐÃ HẾT HẠN" IS NOT A COLUMN. It is `het_han_luc <= now()`, computed at read time. A boolean
-- written by a job is wrong the moment the job is late or the clock skews, and the stale value
-- is the one that reaches the caller (rule 10, invariant 3).
--
-- THE TTL FOR nguon = 'ghep' IS NOT ENFORCED HERE. ADR 0019 requires it to be shorter than an
-- app session but leaves the number open (§CÒN MỞ #1: the TTL cannot be set before anyone
-- decides what the shared screen actually is). Writing a number into a CHECK now would be
-- deciding that open question in the one place that is expensive to change. The relation is
-- enforced where the policy lives, and this file only guarantees that an expiry exists at all.
-- ---------------------------------------------------------------------------
-- @entity: CitizenSession
-- @scope:  tenant
CREATE TABLE IF NOT EXISTS phien_cong_dan (
    -- '' means "signed in, has not chosen a commune yet". See the block above.
    tenant_id      TEXT        NOT NULL,
    id             TEXT        NOT NULL,           -- the sid; CitizenSession.ID
    cong_dan_id    TEXT        NOT NULL REFERENCES dinh_danh_cong_dan (id),
    -- SHA-256 of the bearer token, hex. Never the token.
    bam_token      TEXT        NOT NULL,
    -- 'app'  — issued to the citizen's own Mini App
    -- 'ghep' — issued to a shared screen through a pairing code (ADR 0019 / ADR 0023). A paired
    --          session is NOT a separate entity; it is this entity with a different origin, and
    --          it is weaker because it lives on a device the citizen walks away from.
    nguon          TEXT        NOT NULL DEFAULT 'app',
    tao_luc        TIMESTAMPTZ NOT NULL DEFAULT now(),
    het_han_luc    TIMESTAMPTZ NOT NULL,
    thu_hoi_luc    TIMESTAMPTZ,
    thu_hoi_ly_do  TEXT,
    dung_gan_nhat  TIMESTAMPTZ,
    ip_tao         TEXT        NOT NULL DEFAULT '',
    -- User agent or screen descriptor, truncated. Never personal data (rule 3).
    thiet_bi       TEXT        NOT NULL DEFAULT '',
    PRIMARY KEY (tenant_id, id),
    CONSTRAINT phien_cong_dan_nguon_hop_le CHECK (nguon IN ('app', 'ghep')),
    CONSTRAINT phien_cong_dan_co_han CHECK (het_han_luc > tao_luc),
    -- Composite with tenant_id (rule 1, invariant 6). See the note under the global index below
    -- for why this does NOT make the hash globally unique, and why that is acceptable.
    CONSTRAINT phien_cong_dan_bam_token_duy_nhat UNIQUE (tenant_id, bam_token)
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS phien_cong_dan_p%s PARTITION OF phien_cong_dan '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

-- THE HOT PATH, AND THE ONE INDEX HERE THAT CANNOT START WITH tenant_id.
--
-- The citizen edge (ADR 0022) derives the commune FROM the session, so at the moment of this
-- lookup there is no commune to scope by. That is not a rule 1 violation to be fixed; it is
-- the consequence of the decision ADR 0022 already made, and the alternative — taking the
-- commune from something the client sent — is precisely what rule 1 forbidden #2 closes.
--
-- WHAT IT COSTS, stated rather than discovered later: the lookup probes one child index per
-- partition, 32 of them, on every single citizen request. ADR 0022 already says this path must
-- be treated as hot from the start (short-TTL cache, invalidated on revocation), and this is
-- the measurement behind that sentence.
--
-- WHAT IT DOES NOT GUARANTEE: global uniqueness of bam_token. A unique index on a hash
-- partitioned table must contain the partition key, so the UNIQUE above is per commune. With a
-- token of 256 bits of randomness a collision across communes is not a practical concern; it is
-- written down because "unique" in the constraint name would otherwise be read as more than it
-- is. The Go lookup must therefore still check the row it found rather than assume one exists.
CREATE INDEX IF NOT EXISTS phien_cong_dan_theo_token
    ON phien_cong_dan (bam_token);

-- Revoking every live session a citizen holds in one commune — the action behind the "log this
-- screen out" button of ADR 0019 step 6.
CREATE INDEX IF NOT EXISTS phien_cong_dan_theo_cong_dan
    ON phien_cong_dan (tenant_id, cong_dan_id)
    WHERE thu_hoi_luc IS NULL;

-- ---------------------------------------------------------------------------
-- ghep_phien — the pairing CODE. Not a second kind of session.
--
-- The entity here is the short-lived secret a shared screen displays, and the session that
-- comes out of redeeming it is an ordinary row in phien_cong_dan with nguon = 'ghep'. Keeping
-- that distinction is what stops "screen session" from becoming a parallel session model with
-- its own revocation rules and its own gaps.
--
-- THE CODE IS STORED HASHED, NOT RAW, AND THIS IS THE MOST IMPORTANT LINE IN THIS TABLE. The
-- code is a bearer secret for two minutes: whoever can read it can take over the pairing and
-- have a shared screen issued a session in somebody else's name. Storing it raw puts that
-- capability into every database backup and every read-only reporting account. SHA-256 hex,
-- the same shape the staff registry already uses for refresh tokens (store/phien.go:bamRefresh)
-- — reused rather than re-invented, and the reason it needs no salt or work factor is that the
-- input is 128+ bits of randomness, not a password.
--
-- THE FOUR COMPARISON CHARACTERS ARE STORED RAW, AND THAT IS CORRECT. They are not a secret:
-- both the screen and the phone display them so the citizen can compare them by eye (ADR 0019,
-- invariant 4 — several identical-looking screens stand side by side at the one-stop counter,
-- and this is the only thing a person can check). An attacker holding them still cannot pair,
-- because they do not hold the code.
--
-- THERE IS NO `trang_thai` COLUMN. ADR 0019 requires the state to live on the server, and it
-- does — in three timestamps, from which the state is DERIVED and therefore cannot drift:
--
--   waiting   het_han_luc > now()  AND dung_luc IS NULL AND huy_luc IS NULL
--   used      dung_luc IS NOT NULL
--   cancelled huy_luc  IS NOT NULL
--   expired   het_han_luc <= now() AND dung_luc IS NULL AND huy_luc IS NULL
--
-- "Expired" in particular must never become a column: it is a comparison against a clock
-- (rule 10, invariant 3), and a column holding it is wrong for as long as no job has run.
--
-- SINGLE USE is enforced by the database, not by the code path: the constraints below tie
-- dung_luc, cong_dan_id and phien_id together, and a redeem writes all three under the
-- condition that dung_luc is still NULL. Two screens racing for the same code cannot both win.
--
-- ROWS ARE KEPT AFTER USE. They are evidence that a specific citizen authorised a specific
-- screen at a specific time, which is what an audit asks about (ADR 0019, invariant 6). Purging
-- them is an operational decision with an owner, not a line in a migration.
-- ---------------------------------------------------------------------------
-- @entity: SessionPairing
-- @scope:  tenant
CREATE TABLE IF NOT EXISTS ghep_phien (
    -- The commune OF THE SCREEN. ADR 0019 invariant 8 requires it to match the commune of the
    -- citizen's session at redeem time; a mismatch is 401 + alert, not a quiet "not found".
    tenant_id       TEXT        NOT NULL,
    id              TEXT        NOT NULL,
    -- SHA-256 hex of a code carrying at least 128 bits of randomness (ADR 0019, invariant 1).
    bam_ma          TEXT        NOT NULL,
    -- The four characters shown on BOTH devices for the citizen to compare by eye.
    ky_tu_doi_chieu TEXT        NOT NULL,
    tao_luc         TIMESTAMPTZ NOT NULL DEFAULT now(),
    het_han_luc     TIMESTAMPTZ NOT NULL,

    -- Set together, exactly once, when the code is redeemed.
    dung_luc        TIMESTAMPTZ,
    cong_dan_id     TEXT REFERENCES dinh_danh_cong_dan (id),
    phien_id        TEXT,

    -- The screen or the citizen abandoning the pairing before it is redeemed.
    huy_luc         TIMESTAMPTZ,
    huy_ly_do       TEXT,

    ip_tao          TEXT        NOT NULL DEFAULT '',
    -- Which screen asked. A device label, never personal data (rule 3).
    thiet_bi        TEXT        NOT NULL DEFAULT '',

    PRIMARY KEY (tenant_id, id),
    -- Composite with tenant_id (rule 1, invariant 6).
    CONSTRAINT ghep_phien_bam_ma_duy_nhat UNIQUE (tenant_id, bam_ma),
    CONSTRAINT ghep_phien_bon_ky_tu CHECK (length(ky_tu_doi_chieu) = 4),
    -- TTL <= 120 SECONDS, ENFORCED BY THE DATABASE (ADR 0019, invariant 1). Unlike the paired
    -- session TTL, this number IS decided — the ADR states it — so it is enforced where it
    -- cannot be forgotten by a caller.
    CONSTRAINT ghep_phien_ttl_toi_da_120s
        CHECK (het_han_luc > tao_luc AND het_han_luc <= tao_luc + interval '120 seconds'),
    -- Redeemed means all three facts, or none of them.
    CONSTRAINT ghep_phien_dung_thi_du_ba_thu
        CHECK ((dung_luc IS NULL) = (cong_dan_id IS NULL)
               AND (dung_luc IS NULL) = (phien_id IS NULL)),
    -- A code cannot be both redeemed and cancelled.
    CONSTRAINT ghep_phien_khong_vua_dung_vua_huy
        CHECK (dung_luc IS NULL OR huy_luc IS NULL),
    -- The issued session must be in the SAME commune as the code. The foreign key carries
    -- tenant_id, so the database itself refuses a pairing that crosses communes — invariant 8
    -- backed by a constraint and not only by a check in Go.
    FOREIGN KEY (tenant_id, phien_id) REFERENCES phien_cong_dan (tenant_id, id)
) PARTITION BY HASH (tenant_id);

DO $$ BEGIN
    FOR i IN 0..31 LOOP
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS ghep_phien_p%s PARTITION OF ghep_phien '
            'FOR VALUES WITH (MODULUS 32, REMAINDER %s)', lpad(i::text, 2, '0'), i);
    END LOOP;
END $$;

-- REDEEM LOOKS THE CODE UP WITHOUT A COMMUNE, ON PURPOSE, AND THIS INDEX IS WHAT ALLOWS IT.
--
-- Scoping the lookup by the citizen's own commune would be simpler and would still fail closed
-- — a code from another commune would come back "not found". But ADR 0019 invariant 8 asks for
-- more than failing closed: a mismatch must produce 401 AND AN ALERT, and an alert requires
-- knowing that the code exists somewhere else rather than nowhere. So the lookup finds the row
-- first and compares the two communes afterwards.
--
-- That read crosses communes and must be written as such: inside
-- service-identity/internal/store/crosstenant/ (ADR 0021), carrying its own
-- `// @cross-tenant: <reason>` (rule 1, forbidden #6). The table holds at most a couple of
-- minutes of pairing codes, so 32 index probes on it are cheap.
CREATE INDEX IF NOT EXISTS ghep_phien_theo_ma
    ON ghep_phien (bam_ma);

-- ---------------------------------------------------------------------------
-- REVERSAL (rule 7, invariant 4).
--
-- core/migrate has no automatic rollback, on purpose: undoing DDL on archival records is an
-- administrative act carried out with a person present, not something a process decides at 3am.
-- So the reverse is written here, to be run by hand.
--
-- THERE ARE TWO DIFFERENT REVERSALS AND CONFUSING THEM IS THE WHOLE RISK:
--
--   BEFORE THE FIRST CITIZEN SIGNS IN — all four tables are empty. Reversal is exact and loses
--   nothing: drop them in dependency order (ghep_phien, phien_cong_dan, quan_he_cong_dan_xa,
--   dinh_danh_cong_dan; the 128 partitions go with their parents), then remove this file's
--   progress row from the schema_migration table, keyed on ten = '0004_kenh_cong_dan.sql', so
--   the runner will apply it again. Written as prose and not as a runnable line because a
--   runnable line is a line that gets run.
--
--   AFTER ANY CITIZEN HAS SIGNED IN — there is no reversal. quan_he_cong_dan_xa is the basis of
--   a citizen's access to their own files and dinh_danh_cong_dan is the subject every citizen
--   business record points at. Dropping either destroys archival material, which is rule 7
--   stop condition #1: it needs an explicit decision by the user and a verified backup, and it
--   is not an operation this file offers.
--
-- The cut between the two is the first row in dinh_danh_cong_dan. Check before acting.
--
-- PER-COMMUNE AND RESUMABLE (rule 7, invariant 5): this file contains no backfill — it creates
-- empty tables and rewrites no existing row — so there is nothing to resume and no commune to
-- iterate. core/migrate runs the whole file in one transaction and records progress in
-- schema_migration. The genuinely per-commune resumable backfill mechanism still does not
-- exist (ADR 0013, §Giới hạn); the first migration that rewrites rows in these tables is the
-- one that will need it, and it must not be written as a single statement then.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- BACKSTOP, carried forward from 0002 and 0003: a table declared PARTITION BY with no
-- partitions rejects every INSERT, silently, until the first business write fails. The check
-- only describes the state after the NEWEST migration that carries it, so every new file ends
-- with it. This file declares three partitioned tables, so it has real work to do here.
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
