package identityclient

// The seven RPCs an automation runner asks identity (ADR 0058, ADR 0029 §Bổ sung 29/09): claim the
// due runs, record their outcome, and the four lookups a run needs — escalation instants, unassigned
// hold instants, unit permission holders, leadership. Every contract, bound and status code is
// stated once in proto/vigov/identity/v1/identity.proto; these wrappers keep its promises on the Go
// side: nothing unset is read as the Unix epoch, nothing missing is read as "nobody", and
// FAILED_PRECONDITION is the commune's configuration, never a number.

import (
	"context"
	"errors"
	"fmt"
	"time"

	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
)

// ErrAutomationNotConfigured marks FAILED_PRECONDITION from an automation lookup: the commune has no
// usable `sla` row, a faulty working calendar, or an anomaly the contract refuses rather than
// truncates (more than 50 leadership accounts). The runner records CONFIGURATION_MISSING and
// notifies nobody from that answer — never a default number of hours.
var ErrAutomationNotConfigured = errors.New("identityclient: xã chưa cấu hình đủ cho việc nền tự động hoá")

// Automation bounds of the contract. The wrappers refuse more instead of truncating: a truncated
// question leaves late records unreported with nothing saying so.
const (
	MaxAutomationScopes   = 10
	MaxInstantsPerCall    = 500
	MaxOrgUnitsPerCall    = 50
	MaxLeadershipAccounts = 50
)

// AutomationRun is one claimed run. ClaimedAt is the runner's `now` for the WHOLE run.
type AutomationRun struct {
	RunID     string
	Job       identityv1.AutomationJob
	WorkKind  identityv1.WorkKind
	ClaimedAt time.Time
	// ScheduledReportPeriod is WEEK or MONTH on a SCHEDULED_REPORTS run, UNSPECIFIED on every other job
	// (identity.proto AutomationRun.scheduled_report_period). Copied as received: a runner that gets
	// UNSPECIFIED on a SCHEDULED_REPORTS run sends nothing and records FAILED — it never reads its own
	// clock for the period (ADR 0086 B2), so this wrapper does not either.
	ScheduledReportPeriod identityv1.ScheduledReportPeriod
}

// AutomationOutcome is what RecordAutomationRunOutcome writes — a closed outcome and three counts.
type AutomationOutcome struct {
	Outcome                 identityv1.AutomationRunOutcome
	RecordsExamined         uint32
	NoticesDelivered        uint32
	RecordsWithoutRecipient uint32
}

// EscalationInstants are the two thresholds of one missed deadline. Never stored (rule 10, inv. 3).
type EscalationInstants struct {
	UnitHeadDueAt time.Time
	ChairmanDueAt time.Time
}

// ClaimDueAutomationRuns claims, for the commune in ctx, the scopes that are due now. Absent means
// "nothing to do now". NEVER RETRIED IN A LOOP: a retry after a lost reply answers absent (contract).
func (c *Client) ClaimDueAutomationRuns(ctx context.Context, scopes []*identityv1.AutomationRunScope) (
	[]AutomationRun, error) {

	if len(scopes) == 0 || len(scopes) > MaxAutomationScopes {
		return nil, fmt.Errorf("identityclient: ClaimDueAutomationRuns với %d phạm vi, cần 1–%d", len(scopes),
			MaxAutomationScopes)
	}
	ctx, cancel := context.WithTimeout(ctx, HanGoi)
	defer cancel()

	resp, err := c.cl.ClaimDueAutomationRuns(ctx, &identityv1.ClaimDueAutomationRunsRequest{Scopes: scopes})
	if err != nil {
		c.log.WarnContext(ctx, "CẢNH BÁO: không nhận được lượt chạy tự động hoá", "ma_loi", status.Code(err).String())
		return nil, wrapCallError("ClaimDueAutomationRuns", err, nil)
	}
	asked := make(map[[2]int32]bool, len(scopes))
	for _, s := range scopes {
		asked[[2]int32{int32(s.GetJob()), int32(s.GetWorkKind())}] = true
	}
	out := make([]AutomationRun, 0, len(resp.GetRuns()))
	for _, r := range resp.GetRuns() {
		sc := r.GetScope()
		if r.GetRunId() == "" || !asked[[2]int32{int32(sc.GetJob()), int32(sc.GetWorkKind())}] {
			// A run for a scope this runner does not own would be run by the wrong service.
			return nil, errors.New("identityclient: ClaimDueAutomationRuns trả lượt không được hỏi — lỗi hợp đồng")
		}
		at, err := instant(r.GetClaimedAt(), "claimed_at")
		if err != nil {
			return nil, err
		}
		out = append(out, AutomationRun{RunID: r.GetRunId(), Job: sc.GetJob(), WorkKind: sc.GetWorkKind(), ClaimedAt: at,
			ScheduledReportPeriod: r.GetScheduledReportPeriod()})
	}
	return out, nil
}

// RecordAutomationRunOutcome writes one run's result. Idempotent on the server: first write wins,
// so the caller may retry after an error.
func (c *Client) RecordAutomationRunOutcome(ctx context.Context, runID string, o AutomationOutcome) error {
	if runID == "" || o.Outcome == identityv1.AutomationRunOutcome_AUTOMATION_RUN_OUTCOME_UNSPECIFIED {
		return errors.New("identityclient: RecordAutomationRunOutcome thiếu mã lượt hoặc kết quả")
	}
	ctx, cancel := context.WithTimeout(ctx, HanGoi)
	defer cancel()
	_, err := c.cl.RecordAutomationRunOutcome(ctx, &identityv1.RecordAutomationRunOutcomeRequest{
		RunId: runID, Outcome: o.Outcome, RecordsExamined: o.RecordsExamined,
		NoticesDelivered: o.NoticesDelivered, RecordsWithoutRecipient: o.RecordsWithoutRecipient,
	})
	if err != nil {
		c.log.WarnContext(ctx, "CẢNH BÁO: không ghi được kết quả lượt tự động hoá", "ma_loi", status.Code(err).String())
		return wrapCallError("RecordAutomationRunOutcome", err, nil)
	}
	return nil
}

// EscalationInstants answers, per DISTINCT missed deadline (1–500), the two escalation instants.
// The map is keyed by the UTC instant; a reply missing any requested deadline is a contract fault.
func (c *Client) EscalationInstants(ctx context.Context, kind identityv1.WorkKind, linhVuc string,
	missed []time.Time) (map[time.Time]EscalationInstants, error) {

	distinct, req, err := instantsRequest(kind, missed, "EscalationInstants")
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, HanGoi)
	defer cancel()
	resp, err := c.cl.ResolveEscalationInstants(ctx, &identityv1.ResolveEscalationInstantsRequest{
		WorkKind: kind, LinhVuc: linhVuc, MissedDeadlines: req,
	})
	if err != nil {
		c.log.WarnContext(ctx, "CẢNH BÁO: không lấy được mốc leo thang", "ma_loi", status.Code(err).String(),
			"loai_viec", kind.String())
		return nil, wrapCallError("ResolveEscalationInstants", err, ErrAutomationNotConfigured)
	}
	out := make(map[time.Time]EscalationInstants, len(distinct))
	for _, it := range resp.GetItems() {
		key, err := instant(it.GetMissedDeadline(), "missed_deadline")
		if err != nil {
			return nil, err
		}
		head, err := instant(it.GetUnitHeadDueAt(), "unit_head_due_at")
		if err != nil {
			return nil, err
		}
		chair, err := instant(it.GetChairmanDueAt(), "chairman_due_at")
		if err != nil {
			return nil, err
		}
		out[key] = EscalationInstants{UnitHeadDueAt: head, ChairmanDueAt: chair}
	}
	if err := coversAll(distinct, func(t time.Time) bool { _, ok := out[t]; return ok }, "ResolveEscalationInstants"); err != nil {
		return nil, err
	}
	return out, nil
}

// UnassignedHoldInstants answers, per DISTINCT hold start (1–500), when the hold becomes reportable.
// disabled=true is the commune's NULL — "do not report" — and the map is then nil. It is a real
// answer, not a fault, and nothing substitutes a number.
func (c *Client) UnassignedHoldInstants(ctx context.Context, kind identityv1.WorkKind, linhVuc string,
	starts []time.Time) (disabled bool, due map[time.Time]time.Time, err error) {

	distinct, req, err := instantsRequest(kind, starts, "UnassignedHoldInstants")
	if err != nil {
		return false, nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, HanGoi)
	defer cancel()
	resp, err := c.cl.ResolveUnassignedHoldInstants(ctx, &identityv1.ResolveUnassignedHoldInstantsRequest{
		WorkKind: kind, LinhVuc: linhVuc, HoldStartedAt: req,
	})
	if err != nil {
		c.log.WarnContext(ctx, "CẢNH BÁO: không lấy được mốc báo việc chưa phân công", "ma_loi",
			status.Code(err).String(), "loai_viec", kind.String())
		return false, nil, wrapCallError("ResolveUnassignedHoldInstants", err, ErrAutomationNotConfigured)
	}
	if resp.GetReportingDisabled() {
		if len(resp.GetItems()) != 0 {
			return false, nil, errors.New("identityclient: ResolveUnassignedHoldInstants vừa tắt vừa trả mốc — lỗi hợp đồng")
		}
		return true, nil, nil
	}
	out := make(map[time.Time]time.Time, len(distinct))
	for _, it := range resp.GetItems() {
		key, err := instant(it.GetHoldStartedAt(), "hold_started_at")
		if err != nil {
			return false, nil, err
		}
		at, err := instant(it.GetUnassignedReportDueAt(), "unassigned_report_due_at")
		if err != nil {
			return false, nil, err
		}
		out[key] = at
	}
	if err := coversAll(distinct, func(t time.Time) bool { _, ok := out[t]; return ok }, "ResolveUnassignedHoldInstants"); err != nil {
		return false, nil, err
	}
	return false, out, nil
}

// OrgUnitPermissionHolders answers, for up to 50 org units, the staff codes sitting in each that hold
// permissionKey there. A unit absent from the map has nobody — an ordinary answer. Grants nothing:
// never use it in a guard.
func (c *Client) OrgUnitPermissionHolders(ctx context.Context, orgUnitIDs []string, permissionKey string) (
	map[string][]string, error) {

	if permissionKey == "" {
		return nil, errors.New("identityclient: OrgUnitPermissionHolders thiếu khoá quyền")
	}
	if len(orgUnitIDs) == 0 {
		return map[string][]string{}, nil
	}
	if len(orgUnitIDs) > MaxOrgUnitsPerCall {
		return nil, fmt.Errorf("identityclient: OrgUnitPermissionHolders với %d bộ phận, tối đa %d — bên gọi phải chia trang",
			len(orgUnitIDs), MaxOrgUnitsPerCall)
	}
	asked := make(map[string]bool, len(orgUnitIDs))
	for _, id := range orgUnitIDs {
		asked[id] = true
	}
	ctx, cancel := context.WithTimeout(ctx, HanGoi)
	defer cancel()
	resp, err := c.cl.ResolveOrgUnitPermissionHolders(ctx, &identityv1.ResolveOrgUnitPermissionHoldersRequest{
		OrgUnitIds: orgUnitIDs, PermissionKey: permissionKey,
	})
	if err != nil {
		c.log.WarnContext(ctx, "CẢNH BÁO: không lấy được người giữ quyền của bộ phận", "ma_loi", status.Code(err).String())
		return nil, wrapCallError("ResolveOrgUnitPermissionHolders", err, nil)
	}
	out := make(map[string][]string, len(resp.GetItems()))
	for _, it := range resp.GetItems() {
		if !asked[it.GetOrgUnitId()] {
			return nil, errors.New("identityclient: ResolveOrgUnitPermissionHolders trả bộ phận không được hỏi — lỗi hợp đồng")
		}
		out[it.GetOrgUnitId()] = append([]string(nil), it.GetStaffMa()...)
	}
	return out, nil
}

// LeadershipStaff answers the staff codes of the commune's leadership. Empty is ordinary. More than
// 50 arrives as FAILED_PRECONDITION → ErrAutomationNotConfigured.
func (c *Client) LeadershipStaff(ctx context.Context) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, HanGoi)
	defer cancel()
	resp, err := c.cl.ResolveLeadershipStaff(ctx, &identityv1.ResolveLeadershipStaffRequest{})
	if err != nil {
		c.log.WarnContext(ctx, "CẢNH BÁO: không lấy được danh sách lãnh đạo", "ma_loi", status.Code(err).String())
		return nil, wrapCallError("ResolveLeadershipStaff", err, ErrAutomationNotConfigured)
	}
	if len(resp.GetStaffMa()) > MaxLeadershipAccounts {
		return nil, errors.New("identityclient: ResolveLeadershipStaff trả quá 50 mã — lỗi hợp đồng")
	}
	return append([]string(nil), resp.GetStaffMa()...), nil
}

// instantsRequest validates and de-duplicates the instants of one lookup, keyed by UTC instant.
func instantsRequest(kind identityv1.WorkKind, in []time.Time, method string) (
	[]time.Time, []*timestamppb.Timestamp, error) {

	if kind == identityv1.WorkKind_WORK_KIND_UNSPECIFIED {
		return nil, nil, fmt.Errorf("identityclient: %s không có loại việc", method)
	}
	seen := make(map[time.Time]bool, len(in))
	var distinct []time.Time
	for _, t := range in {
		if t.IsZero() {
			return nil, nil, fmt.Errorf("identityclient: %s với một mốc rỗng", method)
		}
		k := t.UTC()
		if !seen[k] {
			seen[k] = true
			distinct = append(distinct, k)
		}
	}
	if len(distinct) == 0 || len(distinct) > MaxInstantsPerCall {
		return nil, nil, fmt.Errorf("identityclient: %s với %d mốc, cần 1–%d — bên gọi phải chia trang",
			method, len(distinct), MaxInstantsPerCall)
	}
	req := make([]*timestamppb.Timestamp, len(distinct))
	for i, t := range distinct {
		req[i] = timestamppb.New(t)
	}
	return distinct, req, nil
}

// instant reads a Timestamp the contract says is never unset. Unset or invalid is a contract fault —
// never the Unix epoch, which is `<= as_of` and would read as "due since 1970".
func instant(ts *timestamppb.Timestamp, field string) (time.Time, error) {
	if ts == nil {
		return time.Time{}, fmt.Errorf("identityclient: thiếu %s trong câu trả lời — lỗi hợp đồng", field)
	}
	if err := ts.CheckValid(); err != nil {
		return time.Time{}, fmt.Errorf("identityclient: %s không hợp lệ: %w", field, err)
	}
	return ts.AsTime().UTC(), nil
}

func coversAll(distinct []time.Time, has func(time.Time) bool, method string) error {
	for _, t := range distinct {
		if !has(t) {
			return fmt.Errorf("identityclient: %s trả thiếu mốc — lỗi hợp đồng, không phải \"không có gì\"", method)
		}
	}
	return nil
}
