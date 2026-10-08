-- 0019 — MAIL SERVER: the result of the last test message is kept on the commune's row (ADR 0079 #5,
-- "lưu kết quả gửi thư thử"; spec Cấu hình 10/12 `EmailSettings.last_test { at, to, ok, error }`; owner
-- answer E, 08/10/2026). Four additive, nullable columns on 0008's `mail_settings`.
--
-- WHY A NEW FILE: core/migrate compares the checksum of every applied file at startup. 0008 is not edited.
--
-- NAMED IN ENGLISH (rule 12, ADR 0051). The spec's `last_test.to` / `.error` are the screen's names; the
-- columns say what they hold — a MASKED address, an error CLASS.
--
-- ---------------------------------------------------------------------------
-- VENDOR CHOICES, stated so a reviewer can overrule them:
--
--   * THE RECIPIENT IS STORED MASKED ONLY (rule 3). `last_test_to_masked` is core/privacy.MaskEmail's
--     output — first character of the local part, "***", the whole domain ("o***@xa.gov.vn") — the same
--     value the audit entry of the send already carries (app/mail_settings.go:247). The address a person
--     typed is personal data; the screen needs only enough to recognise WHICH test it was. The CHECK
--     below is the floor: a raw address does not have that shape and is refused at the write.
--   * THE ERROR IS A CLASS, NEVER SMTP TEXT. internal/mail already drops the server's reply text and
--     returns one of ten sentinels (internal/mail/sender.go:60-69); the HTTP layer maps each to its own
--     sentence (http/mail_settings.go:159-180). The class list is those ten, ONE-TO-ONE, plus 'khac' for
--     an error that is none of them (the handler's 500 branch). NOT the six-way grouping the card offered
--     as an example: certificate vs TLS handshake vs missing STARTTLS carry DIFFERENT advice on the
--     screen, and a stored class coarser than the sentinels would lose which sentence to show.
--   * VALUES ARE VIETNAMESE WITHOUT DIACRITICS (ADR 0011, ADR 0051 table: enum values are data, never
--     translated), as 0018's zalo_delivery.error_class. Mapping, sentinel → class:
--
--        mail.ErrConnect            khong-ket-noi            mail.ErrAuthUnsupported  khong-ho-tro-dang-nhap
--        mail.ErrTimeout            het-thoi-gian            mail.ErrAuthRejected     sai-tai-khoan
--        mail.ErrCertificate        chung-chi-khong-hop-le   mail.ErrRecipientRejected tu-choi-dia-chi
--        mail.ErrTLS                loi-tls                  mail.ErrProtocol         sai-giao-thuc
--        mail.ErrStartTLSMissing    khong-co-starttls        anything else            khac
--        mail.ErrPlaintextRefused   tu-choi-khong-ma-hoa
--
--   * NO `last_test_by`. The spec's last_test has no "who", and the audit entry the send already writes
--     BEFORE sending (app/mail_settings.go:255-264) names the staff code. A column would be a second copy.
--   * NO HISTORY TABLE. The row keeps the LAST result for the screen; every attempt is already an audit
--     entry (ActionSendTestMail). Overwriting the four columns loses nothing that is not in the trail.
--
-- OWED BY GO (service-comms/internal/**, the go-service-builder card that follows):
--   * after the send returns, write all four columns in ONE UPDATE scoped by tenant_id — and NOT touch
--     updated_at / updated_by: recording a test is not a change of the configuration, and bumping
--     updated_by would show the tester as the last person to SAVE the server settings.
--   * a save that changes host, port, security or username should clear the four columns (all NULL —
--     the CHECKs allow exactly that): the old result describes a destination that is no longer
--     configured. Not enforced here — whether to clear it is a screen decision, and the guard below
--     cannot tell a re-save of the same destination from a new one without duplicating Go's rule.
--   * a failure BEFORE the send (no row, platform encryption missing, the password not opening) records
--     NOTHING: no test reached the server, so there is no server result to show.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. ROWS PER COMMUNE: at most one (tenant_id is the primary key, 0008). NO ROW IS WRITTEN. ADD COLUMN
--      without a default is catalogue-only; each ADD CONSTRAINT scans the table once — one row per
--      commune at most.
--   2. IF IT STOPS HALF-WAY: it cannot land half-applied — one file, one transaction. Every statement is
--      IF NOT EXISTS, or a constraint added behind a pg_constraint lookup; a retry costs nothing. No
--      backfill, nothing to resume per commune.
--   3. HOW IT IS REVERSED: see REVERSAL at the bottom.
--   4. READ PATHS THAT CHANGE MEANING: none. internal/store names its mail_settings columns explicitly
--      (store/mail_settings.go); nothing reads the new columns until Go asks. Every existing row has
--      all four NULL = "never tested", which is the truth for every row today.
--   5. RETENTION: nothing dropped, retyped or emptied. The four columns are overwritten by the next
--      test; the durable record of each test is its audit entry (rule 6).
--
-- THE LOCK: ACCESS EXCLUSIVE on mail_settings and its 32 partitions until COMMIT. One row per commune.
-- PERSONAL DATA (rule 3): a MASKED address only. PG FLOOR: 13 (0003). Cluster: 16.
-- ---------------------------------------------------------------------------

ALTER TABLE mail_settings ADD COLUMN IF NOT EXISTS last_test_at          TIMESTAMPTZ;
ALTER TABLE mail_settings ADD COLUMN IF NOT EXISTS last_test_to_masked   TEXT;
ALTER TABLE mail_settings ADD COLUMN IF NOT EXISTS last_test_ok          BOOLEAN;
ALTER TABLE mail_settings ADD COLUMN IF NOT EXISTS last_test_error_class TEXT;

DO $$
DECLARE parent oid := 'mail_settings'::regclass;
BEGIN
    -- When, to whom and whether: all three, or none (never tested).
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = parent
                   AND conname = 'mail_settings_last_test_complete') THEN
        ALTER TABLE mail_settings ADD CONSTRAINT mail_settings_last_test_complete
            CHECK ((last_test_at IS NULL) = (last_test_ok IS NULL)
                   AND (last_test_at IS NULL) = (last_test_to_masked IS NULL));
    END IF;

    -- A failure has a class; a success has none; "never tested" has none. Written as CASE because
    -- `last_test_ok = (last_test_error_class IS NULL)` is NULL — and a CHECK PASSES on NULL — when
    -- last_test_ok is NULL, which would admit a class on a row that was never tested.
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = parent
                   AND conname = 'mail_settings_last_test_outcome') THEN
        ALTER TABLE mail_settings ADD CONSTRAINT mail_settings_last_test_outcome
            CHECK (CASE WHEN last_test_ok IS NULL THEN last_test_error_class IS NULL
                        ELSE last_test_ok = (last_test_error_class IS NULL) END);
    END IF;

    -- Closed: every failure is countable, and no free text (a server banner, an account name) can
    -- ride in. A new class is one line in a new migration.
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = parent
                   AND conname = 'mail_settings_last_test_error_class_known') THEN
        ALTER TABLE mail_settings ADD CONSTRAINT mail_settings_last_test_error_class_known
            CHECK (last_test_error_class IS NULL
                   OR last_test_error_class IN ('khong-ket-noi', 'het-thoi-gian', 'chung-chi-khong-hop-le',
                                                'loi-tls', 'khong-co-starttls', 'tu-choi-khong-ma-hoa',
                                                'khong-ho-tro-dang-nhap', 'sai-tai-khoan',
                                                'tu-choi-dia-chi', 'sai-giao-thuc', 'khac'));
    END IF;

    -- MaskEmail's shape for a well-formed address: one character, three stars, "@", a domain. A raw
    -- address does not match (rule 3 floor). 254 + "***" bounds the length.
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = parent
                   AND conname = 'mail_settings_last_test_to_masked_shape') THEN
        ALTER TABLE mail_settings ADD CONSTRAINT mail_settings_last_test_to_masked_shape
            CHECK (last_test_to_masked IS NULL
                   OR (last_test_to_masked ~ '^[^@[:space:]][*][*][*]@[^@[:space:]]+$'
                       AND char_length(last_test_to_masked) <= 260));
    END IF;
END $$;

-- mail_settings_guard (0008) is untouched: it refuses DELETE and a change of commune, and leaves every
-- other column editable — the four above included.

-- ---------------------------------------------------------------------------
-- REVERSAL (migration question 3). Run by a person, in ONE transaction; core/migrate has no automatic
-- rollback (ADR 0013). Written as prose, because a runnable line is a line that gets run.
--
-- The four columns hold only the last test's result, and every test is also an audit entry, so the
-- reversal loses nothing the trail does not keep: drop the four constraints named above, then the four
-- columns, then remove this file's row from `schema_migration` (ten = '0019_mail_settings_last_test.sql').
-- Do it only after the Go code that writes the columns has been rolled back, or its next test fails.
-- ---------------------------------------------------------------------------
