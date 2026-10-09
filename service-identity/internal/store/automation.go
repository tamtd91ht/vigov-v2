package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/service-identity/internal/domain"
)

// AutomationStore reads and writes the three tables of migration 0017: the commune's job settings,
// the per-scope lease, and the runs. SQL only; every decision is in app.Automation and domain.
//
// THE COMMUNE IS $1 IN EVERY STATEMENT, from the context or from the caller's transaction (rule 1,
// invariants 4 and 5). No method takes a commune, so no caller can name another commune's jobs.
type AutomationStore struct {
	db *store.DB
}

func NewAutomationStore(db *store.DB) *AutomationStore { return &AutomationStore{db: db} }

// ErrAutomationRunNotFound is a run id that matches no run OF THIS COMMUNE. Unknown and another
// commune's run are one answer (contract: RecordAutomationRunOutcome NOT_FOUND; rule 1).
var ErrAutomationRunNotFound = errors.New("tự động hoá: không có lượt chạy này")

// ErrAutomationSettingExists is the primary key refusing a second first-time row for one job — two
// administrators saving the same never-configured job at the same instant. A lost race, not a bug.
var ErrAutomationSettingExists = errors.New("tự động hoá: cấu hình việc này vừa được người khác lưu")

// automationSettingColumns IS READ BY POSITION in scanSetting. Four adjacent nullable INTEGER
// cadence columns: a swap produces no error, only a job running at another hour.
const automationSettingColumns = `job, enabled, interval_minutes, run_hour, run_minute, weekday, ` +
	`enabled_at, run_requested_at, run_requested_by, updated_at, updated_by`

// Settings reads every job setting this commune saved — at most four rows (PRIMARY KEY (tenant_id,
// job) and a CHECK on `job`), so there is nothing to page. An absent job is OFF.
//
// NOT INSIDE A TRANSACTION, DELIBERATELY: the claim decides on this read and then writes through a
// compare-and-set (ClaimScope) that fails if anything moved, so the read needs no lock — which keeps
// a one-minute tick per commune down to two small SELECTs when nothing is due.
func (s *AutomationStore) Settings(ctx context.Context) ([]domain.AutomationSetting, error) {
	rows, err := s.db.For(ctx).Query(ctx, automationSettingColumns, "automation_job_setting", `ORDER BY job`)
	if err != nil {
		return nil, fmt.Errorf("tự động hoá: đọc cấu hình: %w", err)
	}
	defer rows.Close()

	out := make([]domain.AutomationSetting, 0, 4)
	for rows.Next() {
		st, err := scanSetting(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("tự động hoá: đọc dòng cấu hình: %w", err)
		}
		out = append(out, st)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("tự động hoá: duyệt cấu hình: %w", err)
	}
	return out, nil
}

// SettingForUpdate reads one job's row inside the caller's transaction and holds it. FALSE when the
// commune never saved this job (it is off).
//
// FOR UPDATE because PUT and "run now" are read-decide-write on the same row: without the lock two
// administrators both read the old state and the second write silently discards the first.
func (s *AutomationStore) SettingForUpdate(ctx context.Context, tx *store.ScopedTx,
	job domain.AutomationJob) (domain.AutomationSetting, bool, error) {

	const stmt = `SELECT ` + automationSettingColumns + ` FROM automation_job_setting ` +
		`WHERE tenant_id = $1 AND job = $2 FOR UPDATE`
	st, err := scanSetting(tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), string(job)).Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.AutomationSetting{}, false, nil
	}
	if err != nil {
		return domain.AutomationSetting{}, false, fmt.Errorf("tự động hoá: đọc cấu hình để ghi: %w", err)
	}
	return st, true, nil
}

// InsertSetting writes a job's FIRST row. No ON CONFLICT: a lost race arrives as
// ErrAutomationSettingExists and the caller answers 409 rather than overwriting the winner.
//
// The constraint is matched as a SUBSTRING of the error, following dichLoiGhiSLA: the table is
// PARTITION BY HASH, so PostgreSQL names the partition's key (`automation_job_setting_p07_pkey`).
func (s *AutomationStore) InsertSetting(ctx context.Context, tx *store.ScopedTx, st domain.AutomationSetting) error {
	const stmt = `INSERT INTO automation_job_setting
			(tenant_id, job, enabled, interval_minutes, run_hour, run_minute, weekday,
			 enabled_at, run_requested_at, run_requested_by, updated_at, updated_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`
	iv, h, m, wd := cadenceColumns(st)
	_, err := tx.Exec(ctx, stmt, string(tx.TenantID()), string(st.Job), st.Enabled, iv, h, m, wd,
		timeOrNull(st.EnabledAt), timeOrNull(st.RunRequestedAt), stringOrNull(st.RunRequestedBy),
		st.UpdatedAt, st.UpdatedBy)
	if err != nil {
		if strings.Contains(err.Error(), "automation_job_setting") && strings.Contains(err.Error(), "pkey") {
			return ErrAutomationSettingExists
		}
		return fmt.Errorf("tự động hoá: chèn cấu hình: %w", err)
	}
	return nil
}

// UpdateSetting writes a job's settings. THE "RUN NOW" MARK IS NOT IN THE SET CLAUSE: it has its own
// statement (MarkRunRequested), so a settings save can never clear a pending run another person asked
// for, and a run request never rewrites the cadence.
func (s *AutomationStore) UpdateSetting(ctx context.Context, tx *store.ScopedTx, st domain.AutomationSetting) error {
	const stmt = `UPDATE automation_job_setting SET
			enabled = $3, interval_minutes = $4, run_hour = $5, run_minute = $6, weekday = $7,
			enabled_at = $8, updated_at = $9, updated_by = $10
		WHERE tenant_id = $1 AND job = $2`
	iv, h, m, wd := cadenceColumns(st)
	res, err := tx.Exec(ctx, stmt, string(tx.TenantID()), string(st.Job), st.Enabled, iv, h, m, wd,
		timeOrNull(st.EnabledAt), st.UpdatedAt, st.UpdatedBy)
	if err != nil {
		return fmt.Errorf("tự động hoá: cập nhật cấu hình: %w", err)
	}
	return exactlyOneRow(res, "cập nhật cấu hình")
}

// MarkRunRequested stamps the job's "run now" mark. One mark per job; each scope consumes it once by
// its own claim moving past it (domain.DueTrigger), so nothing ever clears it.
func (s *AutomationStore) MarkRunRequested(ctx context.Context, tx *store.ScopedTx,
	job domain.AutomationJob, at time.Time, by string) error {

	const stmt = `UPDATE automation_job_setting SET run_requested_at = $3, run_requested_by = $4
		WHERE tenant_id = $1 AND job = $2`
	res, err := tx.Exec(ctx, stmt, string(tx.TenantID()), string(job), at, by)
	if err != nil {
		return fmt.Errorf("tự động hoá: ghi yêu cầu chạy ngay: %w", err)
	}
	return exactlyOneRow(res, "ghi yêu cầu chạy ngay")
}

// ScopeStates reads every lease of this commune — at most 3 jobs × 4 kinds = 12 rows.
func (s *AutomationStore) ScopeStates(ctx context.Context) (map[domain.AutomationScope]domain.ScopeState, error) {
	rows, err := s.db.For(ctx).Query(ctx, `job, work_kind, last_run_id, last_claimed_at`,
		"automation_run_scope", `ORDER BY job, work_kind`)
	if err != nil {
		return nil, fmt.Errorf("tự động hoá: đọc trạng thái lượt chạy: %w", err)
	}
	defer rows.Close()

	out := make(map[domain.AutomationScope]domain.ScopeState, 12)
	for rows.Next() {
		var job, kind string
		var st domain.ScopeState
		if err := rows.Scan(&job, &kind, &st.LastRunID, &st.LastClaimedAt); err != nil {
			return nil, fmt.Errorf("tự động hoá: đọc dòng trạng thái: %w", err)
		}
		out[domain.AutomationScope{Job: domain.AutomationJob(job), WorkKind: domain.LoaiViec(kind)}] = st
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("tự động hoá: duyệt trạng thái: %w", err)
	}
	return out, nil
}

// claimScopeStmt IS THE CONDITIONAL WRITE THE CONTRACT PROMISES: "a conditional write that only one
// caller can win".
//
//	no lease yet   INSERT; a concurrent claimer's INSERT waits on the primary key, then takes the
//	               ON CONFLICT branch — whose WHERE compares the committed row's run id with the ""
//	               it read, which can never match (CHECK length = 26), so it writes nothing.
//	a lease exists the UPDATE branch writes only while `last_run_id` still equals the id the claimer
//	               read; under READ COMMITTED the second writer re-evaluates that WHERE against the
//	               first writer's committed row and finds it moved.
//
// Either way the loser sees 0 rows affected, and nothing else about it differs from "not due".
// THE COMPARE TOKEN IS THE RUN ID, NOT THE TIMESTAMP: text equality does not depend on how an instant
// round-trips through the driver.
const claimScopeStmt = `INSERT INTO automation_run_scope
		(tenant_id, job, work_kind, last_run_id, last_claimed_at)
	VALUES ($1, $2, $3, $4, $5)
	ON CONFLICT (tenant_id, job, work_kind) DO UPDATE
		SET last_run_id = EXCLUDED.last_run_id, last_claimed_at = EXCLUDED.last_claimed_at
		WHERE automation_run_scope.last_run_id = $6`

// ClaimScope moves the scope's lease to `run` if and only if its current run id is still prevRunID
// ("" when there was no lease). TRUE means this caller won the slot.
func (s *AutomationStore) ClaimScope(ctx context.Context, tx *store.ScopedTx, prevRunID string,
	run domain.AutomationRun) (bool, error) {

	res, err := tx.Exec(ctx, claimScopeStmt, string(tx.TenantID()), string(run.Scope.Job),
		string(run.Scope.WorkKind), run.ID, run.ClaimedAt, prevRunID)
	if err != nil {
		return false, fmt.Errorf("tự động hoá: nhận lượt chạy: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("tự động hoá: đếm dòng nhận lượt: %w", err)
	}
	return n == 1, nil
}

// InsertRun records a claimed run, in the same transaction as the lease that won it — so there is
// never a lease naming a run that does not exist. The report period is NULL on every job but
// `scheduled_reports` — the shape `automation_run_report_period_shape` pins (migration 0029).
func (s *AutomationStore) InsertRun(ctx context.Context, tx *store.ScopedTx, run domain.AutomationRun) error {
	const stmt = `INSERT INTO automation_run
			(tenant_id, id, job, work_kind, run_trigger, claimed_at, scheduled_report_period)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`
	_, err := tx.Exec(ctx, stmt, string(tx.TenantID()), run.ID, string(run.Scope.Job),
		string(run.Scope.WorkKind), string(run.Trigger), run.ClaimedAt, stringOrNull(string(run.ReportPeriod)))
	if err != nil {
		return fmt.Errorf("tự động hoá: ghi lượt chạy: %w", err)
	}
	return nil
}

// automationRunColumns IS READ BY POSITION in scanRun — three adjacent nullable INTEGER counts.
const automationRunColumns = `id, job, work_kind, run_trigger, claimed_at, outcome, ` +
	`records_examined, notices_delivered, records_without_recipient, recorded_at, scheduled_report_period`

// RunForUpdate reads one run of this commune inside the caller's transaction and holds it, so two
// replicas recording the same run serialise and the first write wins.
func (s *AutomationStore) RunForUpdate(ctx context.Context, tx *store.ScopedTx, id string) (domain.AutomationRun, error) {
	if id == "" {
		return domain.AutomationRun{}, ErrAutomationRunNotFound
	}
	const stmt = `SELECT ` + automationRunColumns + ` FROM automation_run ` +
		`WHERE tenant_id = $1 AND id = $2 FOR UPDATE`
	r, err := scanRun(tx.Underlying().QueryRowContext(ctx, stmt, string(tx.TenantID()), id).Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.AutomationRun{}, ErrAutomationRunNotFound
	}
	if err != nil {
		return domain.AutomationRun{}, fmt.Errorf("tự động hoá: đọc lượt chạy để ghi: %w", err)
	}
	return r, nil
}

// RecordRun writes a run's outcome ONCE: `outcome IS NULL` in the WHERE makes a second write change
// nothing even if a caller skipped the read above.
func (s *AutomationStore) RecordRun(ctx context.Context, tx *store.ScopedTx, run domain.AutomationRun) error {
	const stmt = `UPDATE automation_run SET
			outcome = $3, records_examined = $4, notices_delivered = $5,
			records_without_recipient = $6, recorded_at = $7
		WHERE tenant_id = $1 AND id = $2 AND outcome IS NULL`
	res, err := tx.Exec(ctx, stmt, string(tx.TenantID()), run.ID, string(run.Report.Outcome),
		run.Report.RecordsExamined, run.Report.NoticesDelivered, run.Report.RecordsWithoutRecipient,
		run.RecordedAt)
	if err != nil {
		return fmt.Errorf("tự động hoá: ghi kết quả lượt chạy: %w", err)
	}
	return exactlyOneRow(res, "ghi kết quả lượt chạy")
}

// lastRunsQuery joins each lease to the run it names — the latest claimed run of every scope.
// BOTH tables are bound to the commune: the run by `r.tenant_id = sc.tenant_id`, the lease by
// `sc.tenant_id = $1` (rule 1, the QueryJoin contract).
const lastRunsQuery = `
SELECT r.id, r.job, r.work_kind, r.run_trigger, r.claimed_at, r.outcome,
       r.records_examined, r.notices_delivered, r.records_without_recipient, r.recorded_at,
       r.scheduled_report_period
FROM automation_run_scope sc
JOIN automation_run r
  ON r.tenant_id = sc.tenant_id
 AND r.id        = sc.last_run_id
WHERE sc.tenant_id = $1
ORDER BY sc.job, sc.work_kind`

// LastRuns reads the latest run of every scope this commune has ever claimed — what "Chạy lần cuối
// … · kết quả" shows per kind of work. At most 12 rows.
func (s *AutomationStore) LastRuns(ctx context.Context) ([]domain.AutomationRun, error) {
	rows, err := s.db.For(ctx).QueryJoin(ctx, lastRunsQuery)
	if err != nil {
		return nil, fmt.Errorf("tự động hoá: đọc lượt chạy gần nhất: %w", err)
	}
	defer rows.Close()

	out := make([]domain.AutomationRun, 0, 12)
	for rows.Next() {
		r, err := scanRun(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("tự động hoá: đọc dòng lượt chạy: %w", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("tự động hoá: duyệt lượt chạy: %w", err)
	}
	return out, nil
}

// scanSetting scans automationSettingColumns, in its order. NULL cadence columns become 0 — the store
// only ever writes the columns of the job's kind (cadenceColumns), and the shape CHECK refuses the rest.
func scanSetting(scan func(...any) error) (domain.AutomationSetting, error) {
	var (
		st                     domain.AutomationSetting
		job                    string
		iv, h, m, wd           sql.NullInt64
		enabledAt, requestedAt sql.NullTime
		requestedBy            sql.NullString
	)
	if err := scan(&job, &st.Enabled, &iv, &h, &m, &wd, &enabledAt, &requestedAt, &requestedBy,
		&st.UpdatedAt, &st.UpdatedBy); err != nil {
		return domain.AutomationSetting{}, err
	}
	st.Job = domain.AutomationJob(job)
	if !st.Job.Valid() {
		// The CHECK should have refused it. The value is not put in the error (rule 3, forbidden #3 —
		// it came from the database).
		return domain.AutomationSetting{}, errors.New("tự động hoá: dòng cấu hình mang khoá việc lạ")
	}
	st.IntervalMinutes, st.RunHour, st.RunMinute, st.Weekday =
		int(iv.Int64), int(h.Int64), int(m.Int64), int(wd.Int64)
	st.EnabledAt, st.RunRequestedAt, st.RunRequestedBy = enabledAt.Time, requestedAt.Time, requestedBy.String
	return st, nil
}

// scanRun scans automationRunColumns, in its order.
func scanRun(scan func(...any) error) (domain.AutomationRun, error) {
	var (
		r                   domain.AutomationRun
		job, kind, trigger  string
		outcome             sql.NullString
		examined, delivered sql.NullInt64
		withoutRecipient    sql.NullInt64
		recordedAt          sql.NullTime
		period              sql.NullString
	)
	if err := scan(&r.ID, &job, &kind, &trigger, &r.ClaimedAt, &outcome,
		&examined, &delivered, &withoutRecipient, &recordedAt, &period); err != nil {
		return domain.AutomationRun{}, err
	}
	r.Scope = domain.AutomationScope{Job: domain.AutomationJob(job), WorkKind: domain.LoaiViec(kind)}
	r.Trigger = domain.RunTrigger(trigger)
	r.ReportPeriod = domain.ReportPeriod(period.String)
	r.Report = domain.RunReport{
		Outcome:                 domain.RunOutcome(outcome.String),
		RecordsExamined:         int(examined.Int64),
		NoticesDelivered:        int(delivered.Int64),
		RecordsWithoutRecipient: int(withoutRecipient.Int64),
	}
	r.RecordedAt = recordedAt.Time
	return r, nil
}

// cadenceColumns returns the four cadence columns of a setting: the fields of the job's kind, NULL for
// the rest — the exact shape `automation_job_setting_shape` admits.
func cadenceColumns(st domain.AutomationSetting) (interval, hour, minute, weekday any) {
	switch st.Job.Schedule() {
	case domain.ScheduleInterval:
		return st.IntervalMinutes, nil, nil, nil
	case domain.ScheduleDaily:
		return nil, st.RunHour, st.RunMinute, nil
	case domain.ScheduleWeekly, domain.ScheduleMonthlyAndWeekly:
		return nil, st.RunHour, st.RunMinute, st.Weekday
	}
	return nil, nil, nil, nil
}

func timeOrNull(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t
}

func stringOrNull(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// exactlyOneRow turns "the UPDATE matched nothing" into an error: the caller holds the row FOR UPDATE,
// so zero rows means the predicate here drifted from the read's — never a success that changed nothing.
func exactlyOneRow(res sql.Result, what string) error {
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("tự động hoá: %s: đếm dòng: %w", what, err)
	}
	if n == 0 {
		return fmt.Errorf("tự động hoá: %s: không dòng nào khớp", what)
	}
	return nil
}
