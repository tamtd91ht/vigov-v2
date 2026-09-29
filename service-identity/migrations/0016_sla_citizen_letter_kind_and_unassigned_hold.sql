-- 0016 — the commune's deadline table (`sla`, 0008) gains a FOURTH kind of work and ONE nullable
-- threshold. User decisions 2026-09-29.
--
-- WHY A NEW FILE AND NOT AN EDIT OF 0008: core/migrate compares the checksum of every applied file at
-- startup. Editing an applied file stops the service or leaves two databases with one version number.
--
-- ---------------------------------------------------------------------------
-- 1. THE CITIZEN-LETTER REGISTER (`đơn thư`, URL resource `citizen-letters`) IS ADMITTED INTO
--    `sla.loai_viec` AS THE VALUE `don-thu`.
--
-- ADR 0029 stop condition #1 made a fourth value a decision rather than a deployment, and ADR 0039
-- (:100-102) listed "`sla.loai_viec` does not take đơn thư" among the conflicts it did NOT decide.
-- THE USER DECIDED IT ON 2026-09-29: đơn thư is its own kind of work in the deadline table. The
-- VALUE follows ADR 0011 (enum values stay Vietnamese without diacritics, kebab-case) and the three
-- existing values; `don-thu` is the register's own word (ADR 0039, `don_thu`).
--
-- The CHECK is replaced, not widened in place: PostgreSQL has no ALTER for a CHECK expression. Both
-- statements run in this file's one transaction, so there is no instant at which the column is
-- unconstrained. Dropping the parent's constraint drops the 32 partitions' inherited copies with it.
--
-- ---------------------------------------------------------------------------
-- 2. `unassigned_hold_hours` — working hours a unit may hold work with NOBODY assigned before the
--    `sla_reminders` job reports it (docs/ui-ux/14-cau-hinh.md:330, "Cả việc bộ phận giữ mà chưa phân
--    công ai"; ADR 0058 open question #5, answered by the user 2026-09-29: a per-commune number, one
--    nullable column on EACH row like the other thresholds).
--
-- NULL MEANS "DO NOT REPORT", AND NOTHING FALLS BACK FROM IT. The reference system hard-codes 24
-- wall-clock hours (../vigov-require/apps/api/app/workers/sla.py:42); neither the number nor the unit
-- may be copied (rule 10, forbidden #2 and #3). A NULL row reports nothing, and no reader may
-- substitute a number for it. POST /api/v1/sla/defaults writes 8 into the rows it seeds — a starting
-- point the commune owns and edits (domain.BoGieoSLA), never a value read on the job's path.
--
-- ZERO IS REFUSED, like every other column here (0008 `sla_gio_phai_duong`): a stored 0 would acquire
-- an invented meaning ("report at once") that cannot afterwards be told from a typo. The domain layer
-- folds NULL to 0 in Go for exactly that reason — 0 can never be a stored value.
--
-- ---------------------------------------------------------------------------
-- 3. THE ESCALATION ANCHOR IS DECIDED — the column comments 0008 wrote are superseded.
--
-- ADR 0029 §Bổ sung 29/09: `gio_bao_lanh_dao` and `gio_bao_chu_tich` count WORKING hours FROM THE
-- DEADLINE THE RECORD MISSED (grpc ResolveEscalationInstants). The user also decided that the chairman
-- threshold may not be below the unit head's (Y >= X). THAT RULE IS ENFORCED IN domain.KiemTraDongSLA
-- AND NOT AS A CHECK HERE, deliberately: a row already holding Y < X would make an ADD CONSTRAINT fail
-- at startup and stop the service, and `NOT VALID` on a CHECK of a partitioned table cannot be verified
-- from this repository (no PostgreSQL reachable, VIGOV_TEST_DSN unset). The edit path validates the
-- whole resulting row, so a bad row cannot be saved again untouched.
--
-- ---------------------------------------------------------------------------
-- THE FIVE MIGRATION QUESTIONS.
--
--   1. ROWS PER COMMUNE: none written. The new column is NULL on every existing row; no row changes
--      value, and no row's `loai_viec` is rewritten.
--   2. IF IT STOPS HALF-WAY: it cannot — one file, one transaction, progress row inside it.
--   3. HOW IT IS REVERSED: see REVERSAL below.
--   4. READ PATHS THAT CHANGE MEANING WHILE HALF-APPLIED: none. Existing reads select named columns
--      and never `*`; the CHECK only admits a value no row holds yet.
--   5. RETENTION: nothing is removed. An SLA row is the basis of issued commitments (0008).
-- ---------------------------------------------------------------------------

ALTER TABLE sla DROP CONSTRAINT IF EXISTS sla_loai_viec_hop_le;
ALTER TABLE sla ADD CONSTRAINT sla_loai_viec_hop_le
    CHECK (loai_viec IN ('van-ban-den', 'phan-anh', 'nhiem-vu', 'don-thu'));

ALTER TABLE sla ADD COLUMN IF NOT EXISTS unassigned_hold_hours INTEGER;
ALTER TABLE sla ADD CONSTRAINT sla_unassigned_hold_hours_positive
    CHECK (unassigned_hold_hours IS NULL OR unassigned_hold_hours > 0);

COMMENT ON COLUMN sla.unassigned_hold_hours IS
    'So gio lam viec mot bo phan duoc giu viec ma chua phan cong ai truoc khi bi bao. NULL = khong '
    'bao, khong co so thay the. Nguoi dung chot 29/09/2026.';
COMMENT ON COLUMN sla.gio_bao_lanh_dao IS
    'So gio lam viec, DEM TU HAN DA LO cua viec, truoc khi bao truong bo phan giu viec (ADR 0029 '
    'Bo sung 29/09). Thay chu thich cua 0008.';
COMMENT ON COLUMN sla.gio_bao_chu_tich IS
    'So gio lam viec, DEM TU HAN DA LO, truoc khi bao lanh dao xa. Khong nho hon gio_bao_lanh_dao — '
    'kiem o tang domain (nguoi dung chot 29/09/2026).';

-- ---------------------------------------------------------------------------
-- REVERSAL (rule 7, invariant 4). By hand, in ONE transaction, and ONLY while no row holds
-- `don-thu` and no row holds a non-NULL `unassigned_hold_hours` (check both first):
--
--   drop constraint sla_unassigned_hold_hours_positive, drop column unassigned_hold_hours,
--   replace sla_loai_viec_hop_le with the three-value CHECK of 0008, restore 0008's two comments,
--   and remove this file's row from schema_migration.
--
-- Written as prose, not a runnable line, because a runnable line is a line that gets run. Once a
-- commune has configured a `don-thu` row or a threshold, reversal destroys configuration that
-- commitments were computed from — rule 7 stop condition #2, a decision for the user with a verified
-- backup.
-- ---------------------------------------------------------------------------
