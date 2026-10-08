package app

// What these tests defend (ADR 0058):
//   - claim → run → outcome recorded, with counts; `now` is claimed_at, never a local clock;
//   - two communes never see each other's records, recipients or notices;
//   - the same business item gives the same key across retried runs with different run ids;
//   - escalation levels, digest contents, restricted petitions fail closed;
//   - configuration missing, outages, a panicking commune — each ends in the right outcome and never
//     stops the next commune; the ticker stops on cancellation.
//
// Every instant identity "answers" is a fixed value from the fake; these tests add no hours.

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vihat/vigov/core/commsclient"
	commsv1 "github.com/vihat/vigov/core/gen/vigov/comms/v1"
	identityv1 "github.com/vihat/vigov/core/gen/vigov/identity/v1"
	"github.com/vihat/vigov/core/identityclient"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

var (
	communeA = tenant.ID("01JA" + strings.Repeat("A", 22))
	communeB = tenant.ID("01JB" + strings.Repeat("B", 22))

	runAt       = time.Date(2026, 9, 29, 1, 0, 0, 0, time.UTC) // 08:00 in Viet Nam
	missed      = time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC)
	soonDue     = time.Date(2026, 9, 29, 9, 0, 0, 0, time.UTC)
	soonCutoff  = time.Date(2026, 10, 2, 1, 0, 0, 0, time.UTC)
	headDueAt   = time.Date(2026, 9, 28, 2, 0, 0, 0, time.UTC) // already passed at runAt
	chairDueAt  = time.Date(2026, 10, 1, 2, 0, 0, 0, time.UTC) // not yet
	holdStart   = time.Date(2026, 9, 20, 2, 0, 0, 0, time.UTC)
	holdDueAt   = time.Date(2026, 9, 22, 2, 0, 0, 0, time.UTC)
	quietLogger = slog.New(slog.NewTextHandler(io.Discard, nil))
)

func communeOf(ctx context.Context) tenant.ID {
	id, _ := tenant.From(ctx)
	return id
}

// --- fakes -----------------------------------------------------------------------------------------

type fakeCommunes struct {
	ids []tenant.ID
	err error
}

func (f *fakeCommunes) CommunesWithRecords(context.Context) ([]tenant.ID, error) { return f.ids, f.err }

type fakeLocks struct {
	held     map[string]bool
	mu       sync.Mutex
	released int
}

func (f *fakeLocks) TryLockJobs(_ context.Context, jobs []string) (map[string]bool, func(), error) {
	out := map[string]bool{}
	for _, j := range jobs {
		out[j] = f.held == nil || f.held[j]
	}
	return out, func() {
		f.mu.Lock()
		f.released++
		f.mu.Unlock()
	}, nil
}

func (f *fakeLocks) releases() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.released
}

type fakeRegistry struct {
	inactive map[tenant.ID]bool
	failing  map[tenant.ID]bool
}

// vi-name-ok: the method name of core/platformclient.Directory that CommuneRegistry mirrors
func (f *fakeRegistry) XaTrongNguCanh(ctx context.Context) (tenant.Tenant, bool, error) {
	id := communeOf(ctx)
	if f.failing[id] {
		return tenant.Tenant{}, false, errors.New("platform xuống")
	}
	return tenant.Tenant{ID: id, Active: !f.inactive[id]}, true, nil
}

type fakeAutomationIdentity struct {
	runs      map[tenant.ID][]identityclient.AutomationRun
	claimed   map[tenant.ID][]*identityv1.AutomationRunScope
	recorded  map[string]identityclient.AutomationOutcome
	recordTen map[string]tenant.ID

	dueSoonMissing bool
	chairDue       time.Time
	holdDisabled   bool
	holders        map[tenant.ID]map[string]map[string][]string // commune -> key -> unit -> codes
	leaders        map[tenant.ID][]string
	unavailable    bool

	// cutoffByKey answers DueSoonCutoff for one SLA row key; a key not in it gets soonCutoff.
	cutoffByKey map[string]time.Time
	// asked records, per lookup, every SLA row key (field / priority) it was asked about.
	asked map[string][]string
}

func (f *fakeAutomationIdentity) ask(lookup, key string) {
	if f.asked == nil {
		f.asked = map[string][]string{}
	}
	f.asked[lookup] = append(f.asked[lookup], key)
}

func newFakeIdentity() *fakeAutomationIdentity {
	return &fakeAutomationIdentity{
		runs: map[tenant.ID][]identityclient.AutomationRun{}, claimed: map[tenant.ID][]*identityv1.AutomationRunScope{},
		recorded: map[string]identityclient.AutomationOutcome{}, recordTen: map[string]tenant.ID{},
		chairDue: chairDueAt, holders: map[tenant.ID]map[string]map[string][]string{},
		leaders: map[tenant.ID][]string{},
	}
}

func (f *fakeAutomationIdentity) ClaimDueAutomationRuns(ctx context.Context, scopes []*identityv1.AutomationRunScope) (
	[]identityclient.AutomationRun, error) {
	id := communeOf(ctx)
	f.claimed[id] = scopes
	runs := f.runs[id]
	delete(f.runs, id) // a slot is claimed once
	return runs, nil
}

func (f *fakeAutomationIdentity) RecordAutomationRunOutcome(ctx context.Context, runID string, o identityclient.AutomationOutcome) error {
	f.recorded[runID] = o
	f.recordTen[runID] = communeOf(ctx)
	return nil
}

func (f *fakeAutomationIdentity) DueSoonCutoff(_ context.Context, _ identityv1.WorkKind, key string, asOf time.Time) (time.Time, error) {
	f.ask("due_soon", key)
	if f.unavailable {
		return time.Time{}, identityclient.ErrIdentityUnavailable
	}
	if f.dueSoonMissing {
		return time.Time{}, identityclient.ErrDueSoonNotConfigured
	}
	if !asOf.Equal(runAt) {
		return time.Time{}, fmt.Errorf("as_of %s không phải claimed_at", asOf)
	}
	if c, ok := f.cutoffByKey[key]; ok {
		return c, nil
	}
	return soonCutoff, nil
}

func (f *fakeAutomationIdentity) EscalationInstants(_ context.Context, _ identityv1.WorkKind, key string,
	missedAt []time.Time) (map[time.Time]identityclient.EscalationInstants, error) {
	f.ask("escalation", key)
	out := map[time.Time]identityclient.EscalationInstants{}
	for _, m := range missedAt {
		out[m.UTC()] = identityclient.EscalationInstants{UnitHeadDueAt: headDueAt, ChairmanDueAt: f.chairDue}
	}
	return out, nil
}

func (f *fakeAutomationIdentity) UnassignedHoldInstants(_ context.Context, _ identityv1.WorkKind, key string,
	starts []time.Time) (bool, map[time.Time]time.Time, error) {
	f.ask("unassigned", key)
	if f.holdDisabled {
		return true, nil, nil
	}
	out := map[time.Time]time.Time{}
	for _, s := range starts {
		out[s.UTC()] = holdDueAt
	}
	return false, out, nil
}

func (f *fakeAutomationIdentity) OrgUnitPermissionHolders(ctx context.Context, units []string, key string) (map[string][]string, error) {
	out := map[string][]string{}
	for _, u := range units {
		if codes := f.holders[communeOf(ctx)][key][u]; len(codes) > 0 {
			out[u] = codes
		}
	}
	return out, nil
}

func (f *fakeAutomationIdentity) LeadershipStaff(ctx context.Context) ([]string, error) {
	return f.leaders[communeOf(ctx)], nil
}

type deliveredNotice struct {
	commune tenant.ID
	n       commsclient.Notice
}

type fakeComms struct {
	err       error
	delivered []deliveredNotice
	seen      map[string]bool // commune|key|recipient — comms' dedupe
	calls     int
}

func (f *fakeComms) DeliverStaffNotifications(ctx context.Context, notices []commsclient.Notice) (map[string]commsclient.Delivery, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	if f.seen == nil {
		f.seen = map[string]bool{}
	}
	out := map[string]commsclient.Delivery{}
	for _, n := range notices {
		f.delivered = append(f.delivered, deliveredNotice{commune: communeOf(ctx), n: n})
		var d commsclient.Delivery
		for _, ma := range n.RecipientMa {
			k := string(communeOf(ctx)) + "|" + n.IdempotencyKey + "|" + ma
			if f.seen[k] {
				d.AlreadyDelivered++
			} else {
				f.seen[k] = true
				d.Created++
			}
		}
		out[n.IdempotencyKey] = d
	}
	return out, nil
}

type fakeTaskReader struct {
	byCommune map[tenant.ID][]domain.AutomationRecord
	panicFor  tenant.ID
}

func (f *fakeTaskReader) OpenTasksForAutomation(ctx context.Context) ([]domain.AutomationRecord, error) {
	if f.panicFor != "" && communeOf(ctx) == f.panicFor {
		panic("giả lập: kho hỏng")
	}
	return f.byCommune[communeOf(ctx)], nil
}

type fakeCitizenReportReader struct {
	byCommune map[tenant.ID][]domain.AutomationRecord
	counts    domain.CitizenReportDigestCounts
	sawFrom   time.Time
}

func (f *fakeCitizenReportReader) OpenCitizenReportsForAutomation(ctx context.Context) ([]domain.AutomationRecord, error) {
	return f.byCommune[communeOf(ctx)], nil
}

func (f *fakeCitizenReportReader) CitizenReportDigestCounts(_ context.Context, _, ratedFrom time.Time) (domain.CitizenReportDigestCounts, error) {
	f.sawFrom = ratedFrom
	return f.counts, nil
}

type harness struct {
	runner   *AutomationRunner
	identity *fakeAutomationIdentity
	comms    *fakeComms
	tasks    *fakeTaskReader
	reports  *fakeCitizenReportReader
	locks    *fakeLocks
	registry *fakeRegistry
}

func newHarness(t *testing.T, communes ...tenant.ID) *harness {
	t.Helper()
	h := &harness{
		identity: newFakeIdentity(), comms: &fakeComms{},
		tasks:   &fakeTaskReader{byCommune: map[tenant.ID][]domain.AutomationRecord{}},
		reports: &fakeCitizenReportReader{byCommune: map[tenant.ID][]domain.AutomationRecord{}},
		locks:   &fakeLocks{}, registry: &fakeRegistry{},
	}
	r, err := NewAutomationRunner(AutomationDeps{Communes: &fakeCommunes{ids: communes}, Locks: h.locks,
		Registry: h.registry, Identity: h.identity, Comms: h.comms, Tasks: h.tasks, CitizenReports: h.reports,
		Log: quietLogger})
	if err != nil {
		t.Fatal(err)
	}
	h.runner = r
	return h
}

func (h *harness) claim(id tenant.ID, runID string, job identityv1.AutomationJob, kind identityv1.WorkKind) {
	h.identity.runs[id] = append(h.identity.runs[id], identityclient.AutomationRun{RunID: runID, Job: job,
		WorkKind: kind, ClaimedAt: runAt})
}

func (h *harness) keys(id tenant.ID) []string {
	var out []string
	for _, d := range h.comms.delivered {
		if d.commune == id {
			out = append(out, d.n.IdempotencyKey)
		}
	}
	sort.Strings(out)
	return out
}

const (
	jobSLA      = identityv1.AutomationJob_AUTOMATION_JOB_SLA_REMINDERS
	jobEscalate = identityv1.AutomationJob_AUTOMATION_JOB_ESCALATION
	jobDigest   = identityv1.AutomationJob_AUTOMATION_JOB_WEEKLY_DIGEST
	kindTask    = identityv1.WorkKind_WORK_KIND_NHIEM_VU
	kindReport  = identityv1.WorkKind_WORK_KIND_PHAN_ANH
	succeeded   = identityv1.AutomationRunOutcome_AUTOMATION_RUN_OUTCOME_SUCCEEDED
)

// --- tests -----------------------------------------------------------------------------------------

func TestClaimRunAndRecordOutcome(t *testing.T) {
	h := newHarness(t, communeA)
	h.tasks.byCommune[communeA] = []domain.AutomationRecord{
		{ID: "nv-late", Code: "NV01", Status: "dang-thuc-hien", Deadline: missed, AssigneeMa: "CB-001"},
		{ID: "nv-soon", Code: "NV02", Status: "dang-thuc-hien", Deadline: soonDue, AssigneeMa: "CB-001"},
		{ID: "nv-calm", Code: "NV03", Status: "moi-giao", Deadline: time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC), AssigneeMa: "CB-001"},
	}
	h.claim(communeA, "run-1", jobSLA, kindTask)
	h.runner.Tick(context.Background())

	// Six scopes claimed: three jobs × this service's two kinds, never a document kind.
	if got := h.identity.claimed[communeA]; len(got) != 6 {
		t.Fatalf("claimed %d phạm vi, muốn 6", len(got))
	}
	for _, s := range h.identity.claimed[communeA] {
		if s.GetWorkKind() != kindTask && s.GetWorkKind() != kindReport {
			t.Errorf("nhận phạm vi của service khác: %v", s)
		}
	}
	want := []string{"sla_reminders:due_soon:nhiem-vu:2026-09-29:CB-001", "sla_reminders:overdue:nhiem-vu:nv-late:2026-09-29"}
	if got := h.keys(communeA); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("khoá = %v, muốn %v", got, want)
	}
	o := h.identity.recorded["run-1"]
	if o.Outcome != succeeded || o.RecordsExamined != 3 || o.NoticesDelivered != 2 || o.RecordsWithoutRecipient != 0 {
		t.Errorf("kết quả = %+v", o)
	}
	if h.locks.releases() != 1 {
		t.Errorf("khoá nhả %d lần, muốn 1", h.locks.releases())
	}
}

func TestTwoCommunesNeverMix(t *testing.T) {
	h := newHarness(t, communeA, communeB)
	h.tasks.byCommune[communeA] = []domain.AutomationRecord{{ID: "nv-a", Code: "NV-A", Deadline: missed, AssigneeMa: "CB-A"}}
	h.tasks.byCommune[communeB] = []domain.AutomationRecord{{ID: "nv-b", Code: "NV-B", Deadline: missed, AssigneeMa: "CB-B"}}
	h.claim(communeA, "run-a", jobSLA, kindTask)
	h.claim(communeB, "run-b", jobSLA, kindTask)
	h.runner.Tick(context.Background())

	for _, d := range h.comms.delivered {
		text := d.n.Title + d.n.IdempotencyKey + strings.Join(d.n.RecipientMa, ",")
		mine, other := "A", "B"
		if d.commune == communeB {
			mine, other = "B", "A"
		}
		if strings.Contains(text, "-"+other) || !strings.Contains(text, "-"+mine) {
			t.Errorf("thông báo ở xã %s mang dữ liệu của xã kia: %+v", d.commune, d.n)
		}
	}
	if h.identity.recordTen["run-a"] != communeA || h.identity.recordTen["run-b"] != communeB {
		t.Errorf("kết quả ghi sai xã: %v", h.identity.recordTen)
	}
	if len(h.keys(communeA)) != 1 || len(h.keys(communeB)) != 1 {
		t.Errorf("A=%v B=%v", h.keys(communeA), h.keys(communeB))
	}
}

// A crashed run is redone by a later run with ANOTHER run id: the keys are the same, and comms
// creates nothing twice.
func TestKeysStableAcrossRetriedRuns(t *testing.T) {
	h := newHarness(t, communeA)
	h.tasks.byCommune[communeA] = []domain.AutomationRecord{{ID: "nv-1", Code: "NV01", Deadline: missed, AssigneeMa: "CB-001"}}
	h.claim(communeA, "run-1", jobSLA, kindTask)
	h.runner.Tick(context.Background())
	h.claim(communeA, "run-2", jobSLA, kindTask)
	h.runner.Tick(context.Background())

	all := h.keys(communeA)
	if len(all) != 2 || all[0] != all[1] {
		t.Errorf("khoá qua hai lượt = %v", all)
	}
	if o := h.identity.recorded["run-2"]; o.NoticesDelivered != 0 || o.Outcome != succeeded {
		t.Errorf("lượt làm lại tạo thêm thông báo: %+v", o)
	}
}

func TestUnassignedFallsBackToUnitThenLeadership(t *testing.T) {
	h := newHarness(t, communeA)
	h.identity.holders[communeA] = map[string]map[string][]string{permTaskAssign: {"bp-1": {"CB-HEAD"}}}
	h.identity.leaders[communeA] = []string{"CB-LD"}
	h.tasks.byCommune[communeA] = []domain.AutomationRecord{
		{ID: "nv-unit", Code: "NV10", Deadline: missed, OrgUnitID: "bp-1", HoldStartedAt: holdStart},
		{ID: "nv-none", Code: "NV11", Deadline: missed},
	}
	h.claim(communeA, "run-1", jobSLA, kindTask)
	h.runner.Tick(context.Background())

	to := map[string]string{}
	for _, d := range h.comms.delivered {
		to[d.n.IdempotencyKey] = strings.Join(d.n.RecipientMa, ",")
	}
	if to["sla_reminders:overdue:nhiem-vu:nv-unit:2026-09-29"] != "CB-HEAD" {
		t.Errorf("việc của bộ phận không tới người giao việc: %v", to)
	}
	if to["sla_reminders:overdue:nhiem-vu:nv-none:2026-09-29"] != "CB-LD" {
		t.Errorf("việc không người không bộ phận không tới lãnh đạo: %v", to)
	}
	if to[domain.UnassignedKey(domain.AutomationTask, "nv-unit", holdStart)] != "CB-HEAD" {
		t.Errorf("không báo bộ phận giữ việc chưa phân công: %v", to)
	}

	// The commune's NULL: nothing reported for the hold, nothing counted as a fault.
	h2 := newHarness(t, communeA)
	h2.identity.holdDisabled = true
	h2.identity.holders = h.identity.holders
	h2.tasks.byCommune = h.tasks.byCommune
	h2.claim(communeA, "run-2", jobSLA, kindTask)
	h2.runner.Tick(context.Background())
	for _, k := range h2.keys(communeA) {
		if strings.Contains(k, "unassigned") {
			t.Errorf("xã tắt báo việc chưa phân công mà vẫn báo: %s", k)
		}
	}
	if h2.identity.recorded["run-2"].Outcome != succeeded {
		t.Errorf("tắt báo là câu trả lời, không phải lỗi: %+v", h2.identity.recorded["run-2"])
	}
}

func TestEscalationLevels(t *testing.T) {
	h := newHarness(t, communeA)
	h.identity.holders[communeA] = map[string]map[string][]string{permFeedbackAssign: {"bp-1": {"CB-HEAD"}}}
	h.identity.leaders[communeA] = []string{"CB-LD"}
	h.reports.byCommune[communeA] = []domain.AutomationRecord{
		{ID: "pa-1", Code: "PA-AAAA1111", Field: "moi-truong", Deadline: missed, OrgUnitID: "bp-1", AssigneeMa: "CB-X"},
		{ID: "pa-early", Code: "PA-BBBB2222", Field: "moi-truong", Deadline: time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC)},
	}
	h.claim(communeA, "run-1", jobEscalate, kindReport)
	h.runner.Tick(context.Background())
	if got := h.keys(communeA); strings.Join(got, "|") != "escalation:phan-anh:pa-1:unit_head" {
		t.Fatalf("mức 1 = %v, muốn đúng một khoá unit_head", got)
	}
	if to := h.comms.delivered[0].n.RecipientMa; strings.Join(to, ",") != "CB-HEAD" {
		t.Errorf("mức 1 phải tới người giao việc của bộ phận, không phải người xử lý: %v", to)
	}

	// Later: both thresholds passed. The unit head is not told twice; the leadership once.
	h.identity.chairDue = headDueAt
	h.claim(communeA, "run-2", jobEscalate, kindReport)
	h.runner.Tick(context.Background())
	if got := h.keys(communeA); strings.Join(got, "|") !=
		"escalation:phan-anh:pa-1:chairman|escalation:phan-anh:pa-1:unit_head|escalation:phan-anh:pa-1:unit_head" {
		t.Errorf("hai mức = %v", got)
	}
	if o := h.identity.recorded["run-2"]; o.NoticesDelivered != 1 {
		t.Errorf("lượt 2 tạo %d thông báo, muốn 1 (chỉ mức chủ tịch mới)", o.NoticesDelivered)
	}
}

// A report about a member of staff reaches only someone who may read it.
func TestRestrictedPetitionFailsClosed(t *testing.T) {
	h := newHarness(t, communeA)
	h.identity.holders[communeA] = map[string]map[string][]string{
		permFeedbackAssign:     {"bp-1": {"CB-HEAD", "CB-DEPUTY"}},
		permFeedbackRestricted: {"bp-1": {"CB-DEPUTY"}},
	}
	h.identity.leaders[communeA] = []string{"CB-LD"}
	h.identity.chairDue = headDueAt
	h.reports.byCommune[communeA] = []domain.AutomationRecord{
		{ID: "pa-r", Code: "PA-RRRR0000", Field: domain.LinhVucHanChe, Deadline: missed, OrgUnitID: "bp-1", Restricted: true},
	}
	h.claim(communeA, "run-1", jobEscalate, kindReport)
	h.runner.Tick(context.Background())
	if len(h.comms.delivered) != 1 {
		t.Fatalf("giao %d thông báo, muốn 1 (mức trưởng bộ phận)", len(h.comms.delivered))
	}
	for _, d := range h.comms.delivered {
		for _, ma := range d.n.RecipientMa {
			if ma != "CB-DEPUTY" {
				t.Errorf("phiếu hạn chế tới %s (%s)", ma, d.n.IdempotencyKey)
			}
		}
	}
	if o := h.identity.recorded["run-1"]; o.RecordsWithoutRecipient != 1 {
		t.Errorf("mức lãnh đạo của phiếu hạn chế phải là 'không người nhận': %+v", o)
	}
}

// The due-soon digest's items (ADR 0079 lô 5 Q13) are exactly the records its body counts for THAT
// recipient, with their stored deadlines: a restricted petition reaches the items of only those its
// body reaches — never the leadership, never a unit holder without `feedback.restricted`. Keys are the
// recipe's literals, unchanged by the items. Read from the plan: the wire mapping lives in
// core/commsclient.
func TestDueSoonItemsFollowTheBodyAndTheRestrictedRule(t *testing.T) {
	h := newHarness(t, communeA)
	h.identity.holders[communeA] = map[string]map[string][]string{
		permFeedbackAssign:     {"bp-1": {"CB-HEAD", "CB-DEPUTY"}},
		permFeedbackRestricted: {"bp-1": {"CB-DEPUTY"}},
	}
	h.identity.leaders[communeA] = []string{"CB-LD"}
	dOpen := soonDue
	dNamed := soonDue.Add(30 * time.Minute)
	dRestr := soonDue.Add(time.Hour).In(domain.AutomationZone)
	h.reports.byCommune[communeA] = []domain.AutomationRecord{
		{ID: "pa-open", Code: "PA-OPEN0001", Field: "moi-truong", Deadline: dOpen, OrgUnitID: "bp-1"},
		{ID: "pa-named", Code: "PA-NAMED001", Field: "moi-truong", Deadline: dNamed, AssigneeMa: "CB-HEAD"},
		{ID: "pa-restr", Code: "PA-RESTR001", Field: domain.LinhVucHanChe, Deadline: dRestr, OrgUnitID: "bp-1", Restricted: true},
		{ID: "pa-orphan", Code: "PA-ORPHAN01", Field: domain.LinhVucHanChe, Deadline: dOpen, Restricted: true},
		{ID: "pa-late", Code: "PA-LATE0001", Field: "moi-truong", Deadline: missed, AssigneeMa: "CB-HEAD"},
	}
	ctx := tenant.Into(context.Background(), communeA)
	p, err := h.runner.slaReminders(ctx, domain.AutomationCitizenReport, runAt)
	if err != nil {
		t.Fatal(err)
	}

	stored := map[string]time.Time{}
	for _, x := range h.reports.byCommune[communeA] {
		stored[x.Code] = x.Deadline
	}
	want := map[string]string{ // key -> the codes the body and the items both name
		"sla_reminders:due_soon:phan-anh:2026-09-29:CB-HEAD":   "PA-NAMED001,PA-OPEN0001",
		"sla_reminders:due_soon:phan-anh:2026-09-29:CB-DEPUTY": "PA-OPEN0001,PA-RESTR001",
	}
	got := 0
	for _, n := range p.notices {
		if n.Kind != domain.NoticeDueSoon {
			if len(n.DueSoonItems) != 0 {
				t.Errorf("%s không phải sắp đến hạn mà mang mục", n.Key)
			}
			continue
		}
		got++
		codes, ok := want[n.Key]
		if !ok {
			t.Errorf("khoá sắp đến hạn lạ %q — phiếu hạn chế tới người không được xem?", n.Key)
			continue
		}
		var itemCodes []string
		for _, it := range n.DueSoonItems {
			itemCodes = append(itemCodes, it.Code)
			if it.Deadline != stored[it.Code] { // the stored value itself, location included
				t.Errorf("%s: hạn của %s = %v, muốn hạn đã lưu %v", n.Key, it.Code, it.Deadline, stored[it.Code])
			}
		}
		sort.Strings(itemCodes)
		if strings.Join(itemCodes, ",") != codes || n.Body != "Gồm: "+strings.ReplaceAll(codes, ",", ", ")+"." {
			t.Errorf("%s: mục %v, nội dung %q — muốn cùng %s", n.Key, itemCodes, n.Body, codes)
		}
	}
	if got != len(want) {
		t.Errorf("%d thông báo sắp đến hạn, muốn %d", got, len(want))
	}
	if !p.withoutRecipient["pa-orphan"] {
		t.Errorf("phiếu hạn chế không bộ phận, không người phải là 'không người nhận', không tới lãnh đạo: %v", p.withoutRecipient)
	}
}

// End to end through Tick: what comms receives on a due-soon notice is the plan's items, each with
// the deadline as stored (same instant, nothing recomputed); every other kind carries none.
func TestDueSoonItemsReachComms(t *testing.T) {
	h := newHarness(t, communeA)
	dLater := soonDue.Add(45 * time.Minute)
	h.tasks.byCommune[communeA] = []domain.AutomationRecord{
		{ID: "nv-late", Code: "NV-FAKE-01", Status: "dang-thuc-hien", Deadline: missed, AssigneeMa: "CB-001"},
		{ID: "nv-soon2", Code: "NV-FAKE-03", Status: "dang-thuc-hien", Deadline: dLater, AssigneeMa: "CB-001"},
		{ID: "nv-soon1", Code: "NV-FAKE-02", Status: "dang-thuc-hien", Deadline: soonDue, AssigneeMa: "CB-001"},
	}
	h.claim(communeA, "run-1", jobSLA, kindTask)
	h.runner.Tick(context.Background())

	dueSoon := 0
	for _, d := range h.comms.delivered {
		if d.n.Kind != commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_TASK_DUE_SOON {
			if len(d.n.DueSoonItems) != 0 {
				t.Errorf("%s không phải sắp đến hạn mà mang %d mục", d.n.IdempotencyKey, len(d.n.DueSoonItems))
			}
			continue
		}
		dueSoon++
		it := d.n.DueSoonItems
		if len(it) != 2 || it[0].Code != "NV-FAKE-02" || !it[0].Deadline.Equal(soonDue) ||
			it[1].Code != "NV-FAKE-03" || !it[1].Deadline.Equal(dLater) {
			t.Errorf("mục tới comms = %+v, muốn NV-FAKE-02@%v rồi NV-FAKE-03@%v", it, soonDue, dLater)
		}
	}
	if dueSoon != 1 {
		t.Fatalf("comms nhận %d thông báo sắp đến hạn, muốn 1", dueSoon)
	}
}

// Every notice shape → its per-domain wire kind (ADR 0079 lô 2 Q3), under the SAME keys it had when it
// went out as the legacy DUE_SOON / OVERDUE / ESCALATION. The keys are literals on purpose: comms
// deduplicates on (commune, key, recipient), so a key that moved with the kind would re-deliver every
// notice already sent that day. `to` is the recipient shapesHarness tells.
type noticeShape struct {
	kind commsv1.StaffNotificationKind
	to   string
}

var petitionsNoticeShapes = map[string]noticeShape{
	"sla_reminders:due_soon:nhiem-vu:2026-09-29:CB-001":    {commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_TASK_DUE_SOON, "CB-001"},
	"sla_reminders:overdue:nhiem-vu:nv-unit:2026-09-29":    {commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_TASK_OVERDUE, "CB-ROUTE"},
	"sla_reminders:unassigned:nhiem-vu:nv-unit:1789869600": {commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_TASK_UNASSIGNED, "CB-ROUTE"},
	"escalation:nhiem-vu:nv-unit:unit_head":                {commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_TASK_ESCALATION, "CB-ROUTE"},
	"weekly_digest:nhiem-vu:2026-W40":                      {commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_WEEKLY_DIGEST, "CB-LD"},
	"sla_reminders:due_soon:phan-anh:2026-09-29:CB-002":    {commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_PETITION_DUE_SOON, "CB-002"},
	"sla_reminders:overdue:phan-anh:pa-unit:2026-09-29":    {commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_PETITION_OVERDUE, "CB-PROUTE"},
	"sla_reminders:unassigned:phan-anh:pa-unit:1789869600": {commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_PETITION_UNASSIGNED, "CB-PROUTE"},
	"escalation:phan-anh:pa-unit:unit_head":                {commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_PETITION_ESCALATION, "CB-PROUTE"},
	"weekly_digest:phan-anh:2026-W40":                      {commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_WEEKLY_DIGEST, "CB-LD"},
}

// shapesHarness has one task and one petition of each shape, and one run of each job × kind claimed.
func shapesHarness(t *testing.T) *harness {
	t.Helper()
	h := newHarness(t, communeA)
	h.identity.holders[communeA] = map[string]map[string][]string{
		permTaskAssign:     {"bp-1": {"CB-ROUTE"}},
		permFeedbackAssign: {"bp-1": {"CB-PROUTE"}},
	}
	h.identity.leaders[communeA] = []string{"CB-LD"}
	h.tasks.byCommune[communeA] = []domain.AutomationRecord{
		{ID: "nv-soon", Code: "NV01", Priority: "khan", Deadline: soonDue, AssigneeMa: "CB-001"},
		{ID: "nv-unit", Code: "NV02", Deadline: missed, OrgUnitID: "bp-1", HoldStartedAt: holdStart},
	}
	h.reports.byCommune[communeA] = []domain.AutomationRecord{
		{ID: "pa-soon", Code: "PA-AAAA1111", Field: "moi-truong", Deadline: soonDue, AssigneeMa: "CB-002"},
		{ID: "pa-unit", Code: "PA-BBBB2222", Field: "moi-truong", Deadline: missed, OrgUnitID: "bp-1", HoldStartedAt: holdStart},
	}
	for _, k := range []identityv1.WorkKind{kindTask, kindReport} {
		h.claim(communeA, "sla-"+k.String(), jobSLA, k)
		h.claim(communeA, "esc-"+k.String(), jobEscalate, k)
		h.claim(communeA, "dig-"+k.String(), jobDigest, k)
	}
	return h
}

func TestEveryNoticeGoesOutUnderItsDomainKind(t *testing.T) {
	h := shapesHarness(t)
	h.runner.Tick(context.Background())

	got := map[string]noticeShape{}
	for _, d := range h.comms.delivered {
		got[d.n.IdempotencyKey] = noticeShape{d.n.Kind, strings.Join(d.n.RecipientMa, ",")}
	}
	if len(got) != len(petitionsNoticeShapes) {
		t.Errorf("giao %d khoá, muốn %d: %v", len(got), len(petitionsNoticeShapes), got)
	}
	for key, want := range petitionsNoticeShapes {
		if g, ok := got[key]; !ok || g != want {
			t.Errorf("khoá %s: %+v (có=%v), muốn %+v", key, g, ok, want)
		}
	}
}

// A commune told under the legacy kinds this morning is not told again after the switch: the keys and
// recipients are what comms already holds.
func TestSwitchingKindMidDayDoesNotRedeliver(t *testing.T) {
	h := shapesHarness(t)
	h.comms.seen = map[string]bool{}
	for key, s := range petitionsNoticeShapes {
		h.comms.seen[string(communeA)+"|"+key+"|"+s.to] = true
	}
	h.runner.Tick(context.Background())

	if len(h.identity.recorded) != 6 {
		t.Fatalf("ghi %d kết quả, muốn 6", len(h.identity.recorded))
	}
	for run, o := range h.identity.recorded {
		if o.Outcome != succeeded || o.NoticesDelivered != 0 {
			t.Errorf("%s sau khi đổi loại: %+v, muốn 0 thông báo mới", run, o)
		}
	}
}

func TestCommsKindNeverSendsLegacyKinds(t *testing.T) {
	legacy := map[commsv1.StaffNotificationKind]bool{
		commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_DUE_SOON:   true,
		commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_OVERDUE:    true,
		commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_ESCALATION: true,
	}
	seen := map[commsv1.StaffNotificationKind]bool{}
	for _, work := range []domain.AutomationWorkKind{domain.AutomationTask, domain.AutomationCitizenReport} {
		for _, k := range []domain.NoticeKind{domain.NoticeDueSoon, domain.NoticeOverdue, domain.NoticeUnassigned,
			domain.NoticeEscalation, domain.NoticeWeeklyDigest} {
			w, ok := commsKind(work, k)
			if !ok || legacy[w] {
				t.Errorf("%s loại %d → %v (ok=%v)", work, k, w, ok)
			}
			if k != domain.NoticeWeeklyDigest && seen[w] {
				t.Errorf("%s loại %d → %v, trùng loại khác — hai phân hệ không tách được", work, k, w)
			}
			seen[w] = true
		}
	}
	if _, ok := commsKind(domain.AutomationTask, domain.NoticeKind(0)); ok {
		t.Error("loại 0 được nhận")
	}
	for _, k := range []domain.NoticeKind{domain.NoticeDueSoon, domain.NoticeWeeklyDigest} {
		if _, ok := commsKind("van-ban-den", k); ok {
			t.Errorf("loại việc lạ được nhận cho loại %d — không được gán phân hệ mặc định", k)
		}
	}
}

// A task's reminder thresholds are read by its priority (ADR 0079 lô 2 Q4 b); a task with none asks
// the default row with "". A petition still asks by its field. No deadline is touched — the notice
// compares the stored deadline with identity's answer.
func TestTaskThresholdsAreAskedByPriority(t *testing.T) {
	h := newHarness(t, communeA)
	h.identity.holders[communeA] = map[string]map[string][]string{permTaskAssign: {"bp-1": {"CB-ROUTE"}}}
	// "khan" warns three days ahead; the default row only an hour ahead — before either deadline.
	h.identity.cutoffByKey = map[string]time.Time{"khan": soonCutoff, "": runAt.Add(time.Hour)}
	h.tasks.byCommune[communeA] = []domain.AutomationRecord{
		{ID: "nv-khan", Code: "NV01", Priority: "khan", Deadline: soonDue, AssigneeMa: "CB-K"},
		{ID: "nv-none", Code: "NV02", Deadline: soonDue, AssigneeMa: "CB-N"},
		{ID: "nv-held", Code: "NV03", Priority: "cao", Deadline: missed, OrgUnitID: "bp-1", HoldStartedAt: holdStart},
	}
	h.reports.byCommune[communeA] = []domain.AutomationRecord{
		{ID: "pa-1", Code: "PA-AAAA1111", Field: "moi-truong", Deadline: missed, AssigneeMa: "CB-P"},
	}
	h.claim(communeA, "run-t", jobSLA, kindTask)
	h.claim(communeA, "run-e", jobEscalate, kindTask)
	h.claim(communeA, "run-p", jobEscalate, kindReport)
	h.runner.Tick(context.Background())

	want := map[string]string{
		"due_soon":   "|khan",          // nv-held is late, so only the two not-yet-due tasks are asked about
		"unassigned": "cao",            // the held task, by its own priority
		"escalation": "cao|moi-truong", // the late task by its priority; the petition by its field
	}
	for lookup, w := range want {
		got := append([]string(nil), h.identity.asked[lookup]...)
		sort.Strings(got)
		if strings.Join(got, "|") != w {
			t.Errorf("%s hỏi theo %q, muốn %q", lookup, strings.Join(got, "|"), w)
		}
	}
	keys := strings.Join(h.keys(communeA), "|")
	if !strings.Contains(keys, "sla_reminders:due_soon:nhiem-vu:2026-09-29:CB-K") {
		t.Errorf("việc khẩn trong ngưỡng của mức khẩn không được nhắc: %s", keys)
	}
	if strings.Contains(keys, "CB-N") {
		t.Errorf("việc không mức ưu tiên được nhắc theo ngưỡng của mức khác: %s", keys)
	}
}

func TestWeeklyDigestContent(t *testing.T) {
	h := newHarness(t, communeA)
	h.identity.leaders[communeA] = []string{"CB-LD1", "CB-LD2"}
	h.tasks.byCommune[communeA] = []domain.AutomationRecord{
		{ID: "1", Deadline: missed},
		{ID: "2", Deadline: soonDue},
		{ID: "3", Deadline: time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)}, // beyond the week
		{ID: "4", Status: string(domain.ChoDuyet)},
	}
	h.reports.counts = domain.CitizenReportDigestCounts{Examined: 9, PastDeadline: 2, LowRated: 1, Hot: 3}
	h.claim(communeA, "run-t", jobDigest, kindTask)
	h.claim(communeA, "run-p", jobDigest, kindReport)
	h.runner.Tick(context.Background())

	bodies := map[string]commsclient.Notice{}
	for _, d := range h.comms.delivered {
		bodies[d.n.IdempotencyKey] = d.n
	}
	task := bodies["weekly_digest:nhiem-vu:2026-W40"]
	if task.Body != "1 nhiệm vụ quá hạn, 1 nhiệm vụ đến hạn trong 7 ngày tới, 1 nhiệm vụ chờ duyệt." ||
		strings.Join(task.RecipientMa, ",") != "CB-LD1,CB-LD2" ||
		task.Kind != commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_WEEKLY_DIGEST {
		t.Errorf("bản tin nhiệm vụ = %+v", task)
	}
	if rep := bodies["weekly_digest:phan-anh:2026-W40"]; !strings.HasPrefix(rep.Body, "3 phản ánh nóng") {
		t.Errorf("bản tin phản ánh = %+v", rep)
	}
	if want := time.Date(2026, 9, 22, 1, 0, 0, 0, time.UTC); !h.reports.sawFrom.Equal(want) {
		t.Errorf("cửa sổ đánh giá bắt đầu %s, muốn %s", h.reports.sawFrom, want)
	}
	if o := h.identity.recorded["run-p"]; o.RecordsExamined != 9 || o.Outcome != succeeded {
		t.Errorf("kết quả bản tin phản ánh = %+v", o)
	}

	// No leadership flagged: nothing sent, the summarised records counted — never a fallback.
	h2 := newHarness(t, communeA)
	h2.tasks.byCommune = h.tasks.byCommune
	h2.claim(communeA, "run-t", jobDigest, kindTask)
	h2.runner.Tick(context.Background())
	if h2.comms.calls != 0 || h2.identity.recorded["run-t"].RecordsWithoutRecipient != 3 {
		t.Errorf("không có lãnh đạo: gọi comms %d lần, kết quả %+v", h2.comms.calls, h2.identity.recorded["run-t"])
	}
}

// One field without configuration withholds only what depended on it; the run says so.
func TestConfigurationMissingIsRecordedAndNothingInvented(t *testing.T) {
	h := newHarness(t, communeA)
	h.identity.dueSoonMissing = true
	h.tasks.byCommune[communeA] = []domain.AutomationRecord{
		{ID: "nv-late", Code: "NV01", Deadline: missed, AssigneeMa: "CB-001"},
		{ID: "nv-soon", Code: "NV02", Deadline: soonDue, AssigneeMa: "CB-001"},
	}
	h.claim(communeA, "run-1", jobSLA, kindTask)
	h.runner.Tick(context.Background())
	if got := h.keys(communeA); len(got) != 1 || !strings.Contains(got[0], "overdue") {
		t.Errorf("khoá = %v — chỉ thông báo quá hạn (không cần cấu hình) được gửi", got)
	}
	if o := h.identity.recorded["run-1"]; o.Outcome != identityv1.AutomationRunOutcome_AUTOMATION_RUN_OUTCOME_CONFIGURATION_MISSING {
		t.Errorf("kết quả = %v", o.Outcome)
	}
}

func TestOutagesAreDependencyUnavailable(t *testing.T) {
	h := newHarness(t, communeA)
	h.comms.err = fmt.Errorf("wrap: %w", commsclient.ErrCommsUnavailable)
	h.tasks.byCommune[communeA] = []domain.AutomationRecord{{ID: "nv", Code: "NV", Deadline: missed, AssigneeMa: "CB-1"}}
	h.claim(communeA, "run-1", jobSLA, kindTask)
	h.runner.Tick(context.Background())
	if o := h.identity.recorded["run-1"]; o.Outcome != identityv1.AutomationRunOutcome_AUTOMATION_RUN_OUTCOME_DEPENDENCY_UNAVAILABLE {
		t.Errorf("comms xuống: %v", o.Outcome)
	}

	h2 := newHarness(t, communeA)
	h2.identity.unavailable = true
	h2.tasks.byCommune[communeA] = []domain.AutomationRecord{{ID: "nv", Code: "NV", Deadline: soonDue, AssigneeMa: "CB-1"}}
	h2.claim(communeA, "run-2", jobSLA, kindTask)
	h2.runner.Tick(context.Background())
	if o := h2.identity.recorded["run-2"]; o.Outcome != identityv1.AutomationRunOutcome_AUTOMATION_RUN_OUTCOME_DEPENDENCY_UNAVAILABLE ||
		h2.comms.calls != 0 {
		t.Errorf("identity xuống: %v, comms gọi %d lần — không được giao từ kế hoạch dở", o.Outcome, h2.comms.calls)
	}
}

func TestOneCommuneFailingNeverStopsTheNext(t *testing.T) {
	h := newHarness(t, communeA, communeB)
	h.tasks.panicFor = communeA
	h.tasks.byCommune[communeB] = []domain.AutomationRecord{{ID: "nv-b", Code: "NV-B", Deadline: missed, AssigneeMa: "CB-B"}}
	h.claim(communeA, "run-a", jobSLA, kindTask)
	h.claim(communeB, "run-b", jobSLA, kindTask)
	h.runner.Tick(context.Background())
	if o := h.identity.recorded["run-a"]; o.Outcome != identityv1.AutomationRunOutcome_AUTOMATION_RUN_OUTCOME_FAILED {
		t.Errorf("xã A panic phải ghi FAILED: %+v", o)
	}
	if o := h.identity.recorded["run-b"]; o.Outcome != succeeded || len(h.keys(communeB)) != 1 {
		t.Errorf("xã B bị xã A chặn: %+v %v", o, h.keys(communeB))
	}

	// Platform failing for A, inactive B: neither claims, and neither stops the loop.
	h2 := newHarness(t, communeA, communeB)
	h2.registry.failing = map[tenant.ID]bool{communeA: true}
	h2.registry.inactive = map[tenant.ID]bool{communeB: true}
	h2.runner.Tick(context.Background())
	if len(h2.identity.claimed) != 0 {
		t.Errorf("xã lỗi hoặc không hoạt động vẫn được nhận lượt: %v", h2.identity.claimed)
	}
}

func TestOnlyLockedJobsAreClaimed(t *testing.T) {
	h := newHarness(t, communeA)
	h.locks.held = map[string]bool{"escalation": true}
	h.runner.Tick(context.Background())
	got := h.identity.claimed[communeA]
	if len(got) != 2 || got[0].GetJob() != jobEscalate || got[1].GetJob() != jobEscalate {
		t.Errorf("phạm vi = %v, muốn chỉ escalation × 2 loại việc", got)
	}

	h2 := newHarness(t, communeA)
	h2.locks.held = map[string]bool{}
	h2.runner.Tick(context.Background())
	if len(h2.identity.claimed) != 0 {
		t.Error("không giữ khoá nào mà vẫn hỏi identity")
	}
}

func TestRunStopsWhenContextIsCancelled(t *testing.T) {
	h := newHarness(t)
	h.runner.interval = time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		h.runner.Run(ctx)
		close(done)
	}()
	deadline := time.After(2 * time.Second)
	for h.locks.releases() < 2 {
		select {
		case <-deadline:
			t.Fatal("ticker không chạy nhịp thứ hai")
		case <-time.After(time.Millisecond):
		}
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run không dừng khi ngữ cảnh bị huỷ")
	}
}

func TestPagesRespectContractBounds(t *testing.T) {
	var notices []domain.StaffNotice
	wide := make([]string, 450)
	for i := range wide {
		wide[i] = fmt.Sprintf("CB-%04d", i)
	}
	notices = append(notices, domain.StaffNotice{Key: "k-wide", Kind: domain.NoticeOverdue, Work: domain.AutomationTask,
		Recipients: wide, Title: "t"})
	for i := 0; i < 150; i++ {
		notices = append(notices, domain.StaffNotice{Key: fmt.Sprintf("k-%03d", i), Kind: domain.NoticeDueSoon,
			Work: domain.AutomationCitizenReport, Recipients: []string{"CB-1"}, Title: "t"})
	}
	pages, err := automationPages(notices)
	if err != nil {
		t.Fatal(err)
	}
	total := 0
	for _, p := range pages {
		keys, recipients := map[string]bool{}, 0
		if len(p) > commsclient.MaxNoticesPerCall {
			t.Errorf("trang %d thông báo", len(p))
		}
		for _, n := range p {
			if keys[n.IdempotencyKey] {
				t.Errorf("khoá %s lặp trong một trang", n.IdempotencyKey)
			}
			keys[n.IdempotencyKey] = true
			if len(n.RecipientMa) > maxRecipientsPerNotice {
				t.Errorf("một thông báo %d người nhận", len(n.RecipientMa))
			}
			recipients += len(n.RecipientMa)
			total += len(n.RecipientMa)
		}
		if recipients > commsclient.MaxRecipientsPerCall {
			t.Errorf("trang %d người nhận", recipients)
		}
	}
	if total != 600 {
		t.Errorf("tổng người nhận %d, muốn 600", total)
	}
}
