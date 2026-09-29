package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/app"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// The routes behind Cấu hình → Tự động hoá (docs/ui-ux/14-cau-hinh.md §9, ADR 0058):
//
//	GET  /api/v1/automation-jobs              the three jobs, their settings, last run per kind of work
//	PUT  /api/v1/automation-jobs/{job}        save one job's settings (switch + cadence)
//	POST /api/v1/automation-jobs/{job}/runs   request a run now — claimed at the runners' next tick
//
// ALL THREE DECLARE `admin.sla` — the key the reference system uses for these routes
// (../vigov-require/apps/api/app/modules/admin/router.py:43-69) and ADR 0058 §2 fixes; it is a row of
// `quyen` (migration 0001). No key invented (rule 5, invariant 3c).
//
// THE NOUN `automation-jobs`: ubiquitous-language.md has no row for this concept, and the spec's
// `/tu-dong-hoa` and the reference system's `/automation` are a Vietnamese segment and a singular
// mass noun — rest-api-design REQUIRED #1 wants a plural English noun. `automation-jobs` is the plural of
// the contract's own `AutomationJob`. STATED, not settled: renaming before the web ships is one path
// string here plus `make kb`. `runs` is the nominalised sub-resource for "chạy ngay" (REQUIRED #8) —
// the contract's own `AutomationRun`.
//
// {job} IS THE SOFTWARE KEY (`sla_reminders` · `escalation` · `weekly_digest`), the same string the
// contract's AutomationJob comment names and the table stores — one spelling from column to URL.

// AutomationJobs is the use case behind the three routes, declared at the point of use.
// *app.Automation satisfies it; every method is scoped to the commune in the context by the store.
type AutomationJobs interface {
	Overview(ctx context.Context) ([]app.AutomationJobView, error)
	SaveSetting(ctx context.Context, job domain.AutomationJob, req app.AutomationSettingRequest, actor app.NguoiThucHien) (app.AutomationJobView, error)
	RequestRun(ctx context.Context, job domain.AutomationJob, actor app.NguoiThucHien) (app.AutomationJobView, error)
}

// automationBodyMax bounds a PUT body: five small fields.
const automationBodyMax = 4 << 10

// automationRunOut is one kind of work's latest run, as it leaves the API. Codes and counts only.
type automationRunOut struct {
	// WorkKind is the kind of work the run swept: `van-ban-den` · `phan-anh` · `nhiem-vu` (· `don-thu`
	// once the contract carries it).
	WorkKind string `json:"work_kind"`
	RunID    string `json:"run_id"`
	// Trigger is `schedule` or `request` (run now).
	Trigger   string    `json:"trigger"`
	ClaimedAt time.Time `json:"claimed_at"`
	// Outcome is null while the run has not reported — a runner that died shows as a run with no
	// result. Otherwise `succeeded` · `configuration_missing` · `dependency_unavailable` · `failed`.
	Outcome                 *string    `json:"outcome"`
	RecordsExamined         *int       `json:"records_examined"`
	NoticesDelivered        *int       `json:"notices_delivered"`
	RecordsWithoutRecipient *int       `json:"records_without_recipient"`
	RecordedAt              *time.Time `json:"recorded_at"`
}

// automationJobOut is one job card.
type automationJobOut struct {
	Job string `json:"job"`
	// ScheduleKind decides which cadence fields the card shows: `interval` · `daily` · `weekly`.
	ScheduleKind string `json:"schedule_kind"`
	// Configured is false when the commune never saved this job — it is OFF, and the cadence fields
	// below are the suggested prefill (15 minutes; 07:00; Monday 07:30), not a running schedule.
	Configured bool `json:"configured"`
	Enabled    bool `json:"enabled"`

	IntervalMinutes    *int `json:"interval_minutes"`     // interval jobs only, else null
	MinIntervalMinutes *int `json:"min_interval_minutes"` // interval jobs only: 5
	RunHour            *int `json:"run_hour"`             // daily/weekly, 0–23, Asia/Ho_Chi_Minh
	RunMinute          *int `json:"run_minute"`           // daily/weekly, 0–59
	// Weekday is ISO: 1 = Monday … 7 = Sunday (NOT the reference system's 0 = Monday).
	Weekday *int `json:"weekday"`

	Timezone       string     `json:"timezone"`         // always Asia/Ho_Chi_Minh
	EnabledAt      *time.Time `json:"enabled_at"`       // last switch-on; slots before it never run
	RunRequestedAt *time.Time `json:"run_requested_at"` // last "run now" press, or null

	// LastRuns is one entry per kind of work this job has ever run over — a job runs in several
	// services, so there is no single "last run". Always an array.
	LastRuns []automationRunOut `json:"last_runs"`
}

type automationJobsOut struct {
	Items []automationJobOut `json:"items"`
}

func intPtr(v int) *int { return &v }

func timePtr(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

func automationJobToOut(v app.AutomationJobView) automationJobOut {
	s := v.Setting
	out := automationJobOut{
		Job:            string(s.Job),
		ScheduleKind:   string(s.Job.Schedule()),
		Configured:     v.Configured,
		Enabled:        s.Enabled,
		Timezone:       domain.MuiGioHanhChinh,
		EnabledAt:      timePtr(s.EnabledAt),
		RunRequestedAt: timePtr(s.RunRequestedAt),
		LastRuns:       make([]automationRunOut, 0, len(v.LastRuns)),
	}
	switch s.Job.Schedule() {
	case domain.ScheduleInterval:
		out.IntervalMinutes, out.MinIntervalMinutes = intPtr(s.IntervalMinutes), intPtr(domain.MinIntervalMinutes)
	case domain.ScheduleDaily:
		out.RunHour, out.RunMinute = intPtr(s.RunHour), intPtr(s.RunMinute)
	case domain.ScheduleWeekly:
		out.RunHour, out.RunMinute, out.Weekday = intPtr(s.RunHour), intPtr(s.RunMinute), intPtr(s.Weekday)
	}
	for _, r := range v.LastRuns {
		ro := automationRunOut{
			WorkKind:  string(r.Scope.WorkKind),
			RunID:     r.ID,
			Trigger:   string(r.Trigger),
			ClaimedAt: r.ClaimedAt,
		}
		if r.Recorded() {
			o := string(r.Report.Outcome)
			ro.Outcome = &o
			ro.RecordsExamined = intPtr(r.Report.RecordsExamined)
			ro.NoticesDelivered = intPtr(r.Report.NoticesDelivered)
			ro.RecordsWithoutRecipient = intPtr(r.Report.RecordsWithoutRecipient)
			ro.RecordedAt = timePtr(r.RecordedAt)
		}
		out.LastRuns = append(out.LastRuns, ro)
	}
	return out
}

// automationSettingIn is a PUT body. `enabled` is required; the cadence fields of the job's kind are
// required and the others must be absent or null — refused rather than ignored.
type automationSettingIn struct {
	Enabled         *bool `json:"enabled"`
	IntervalMinutes *int  `json:"interval_minutes"`
	RunHour         *int  `json:"run_hour"`
	RunMinute       *int  `json:"run_minute"`
	Weekday         *int  `json:"weekday"`
}

// ListAutomationJobs serves the three job cards. GET /api/v1/automation-jobs
//
// NO AUDIT ENTRY: a commune's own configuration, read inside its own commune (rule 6, invariant 7
// audits full personal data and cross-commune reads only).
func (h *Handler) ListAutomationJobs(w http.ResponseWriter, r *http.Request) {
	views, err := h.d.Automation.Overview(r.Context())
	if err != nil {
		h.automationError(w, r, "đọc cấu hình tự động hoá", err)
		return
	}
	out := automationJobsOut{Items: make([]automationJobOut, 0, len(views))}
	for _, v := range views {
		out.Items = append(out.Items, automationJobToOut(v))
	}
	vietJSON(w, http.StatusOK, out)
}

// SaveAutomationJob saves one job's settings. PUT /api/v1/automation-jobs/{job}
//
// A SWITCH-ON APPLIES FROM THE NEXT SLOT (§9: "Đổi nhịp có hiệu lực ngay ở lượt chạy kế tiếp"): a
// slot that opened before the switch-on never runs. See app.Automation.SaveSetting.
func (h *Handler) SaveAutomationJob(w http.ResponseWriter, r *http.Request) {
	actor, ok := nguoiThucHienCanBo(r)
	if !ok {
		h.thieuNguoiThucHien(w, r)
		return
	}
	var body automationSettingIn
	r.Body = http.MaxBytesReader(w, r.Body, automationBodyMax)
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		// The decoder's message quotes the input and is not returned (rule 3, forbidden #3).
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request",
			"Nội dung gửi lên không phải JSON hợp lệ hoặc quá lớn.", "")
		return
	}
	req := app.AutomationSettingRequest{
		Enabled:         body.Enabled,
		IntervalMinutes: body.IntervalMinutes,
		RunHour:         body.RunHour,
		RunMinute:       body.RunMinute,
		Weekday:         body.Weekday,
	}
	v, err := h.d.Automation.SaveSetting(r.Context(), domain.AutomationJob(r.PathValue("job")), req, actor)
	if err != nil {
		h.automationError(w, r, "lưu cấu hình tự động hoá", err)
		return
	}
	vietJSON(w, http.StatusOK, automationJobToOut(v))
}

// RequestAutomationRun marks a job "run requested". POST /api/v1/automation-jobs/{job}/runs
//
// 202, NOT 201: nothing ran yet and no run exists to point a Location at — each runner claims the
// request at its next tick (≤ 1 minute), once per kind of work, and each run appears in `last_runs`.
func (h *Handler) RequestAutomationRun(w http.ResponseWriter, r *http.Request) {
	actor, ok := nguoiThucHienCanBo(r)
	if !ok {
		h.thieuNguoiThucHien(w, r)
		return
	}
	v, err := h.d.Automation.RequestRun(r.Context(), domain.AutomationJob(r.PathValue("job")), actor)
	if err != nil {
		h.automationError(w, r, "yêu cầu chạy ngay", err)
		return
	}
	vietJSON(w, http.StatusAccepted, automationJobToOut(v))
}

// automationError maps one use-case failure onto a status and a sentence — one function for the three
// routes, so the mapping cannot drift between them.
func (h *Handler) automationError(w http.ResponseWriter, r *http.Request, what string, err error) {
	switch {
	case errors.Is(err, app.ErrAutomationJobUnknown):
		httpx.WriteError(w, http.StatusNotFound, "automation_job_not_found",
			"Không có việc tự động hoá này.", "")
	case errors.Is(err, app.ErrAutomationJobDisabled):
		httpx.WriteError(w, http.StatusConflict, "automation_job_disabled",
			"Việc này đang tắt. Bật việc trước khi yêu cầu chạy ngay.", "")
	case errors.Is(err, idstore.ErrAutomationSettingExists):
		httpx.WriteError(w, http.StatusConflict, "automation_job_changed",
			"Cấu hình việc này vừa được người khác lưu. Hãy tải lại trang rồi thử lại.", "")
	case app.IsAutomationInputError(err):
		// The domain's own sentence: names the rule, carries no internal detail.
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error(), "")
	default:
		h.d.Log.Error("tự động hoá: "+what+" lỗi hệ thống",
			"xa", string(tenant.MustFrom(r.Context())), "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal",
			"Đã xảy ra lỗi. Vui lòng thử lại.", "")
	}
}
