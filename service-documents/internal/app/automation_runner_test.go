package app

// What these tests defend (ADR 0058), mirroring service-petitions' runner tests:
//   - claim → run → outcome recorded, with counts; `now` is claimed_at, never a local clock;
//   - VAN_BAN_DEN and DON_THU scopes are claimed — never a petitions kind; a DON_THU run reads the
//     letter register only, asks identity with DON_THU, routes by `petition.create`, and keys every
//     notice under `don-thu` (ADR 0079 lô 5 Q18);
//   - two communes never see each other's documents, recipients or notices;
//   - the same document gives the same key across retried runs with different run ids;
//   - the recipient chain, escalation levels, digest contents;
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
	"github.com/vihat/vigov/service-documents/internal/domain"
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
	kindsSeen map[identityv1.WorkKind]bool
	permsSeen map[string]bool

	dueSoonMissing   bool
	escalationMissed bool
	chairDue         time.Time
	holdDisabled     bool
	holders          map[tenant.ID]map[string][]string // commune -> unit -> codes (document.route)
	leaders          map[tenant.ID][]string
	unavailable      bool
}

func newFakeIdentity() *fakeAutomationIdentity {
	return &fakeAutomationIdentity{
		runs: map[tenant.ID][]identityclient.AutomationRun{}, claimed: map[tenant.ID][]*identityv1.AutomationRunScope{},
		recorded: map[string]identityclient.AutomationOutcome{}, recordTen: map[string]tenant.ID{},
		kindsSeen: map[identityv1.WorkKind]bool{}, permsSeen: map[string]bool{},
		chairDue: chairDueAt, holders: map[tenant.ID]map[string][]string{}, leaders: map[tenant.ID][]string{},
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

func (f *fakeAutomationIdentity) DueSoonCutoff(_ context.Context, k identityv1.WorkKind, linhVuc string, asOf time.Time) (time.Time, error) {
	f.kindsSeen[k] = true
	if f.unavailable {
		return time.Time{}, identityclient.ErrIdentityUnavailable
	}
	if f.dueSoonMissing {
		return time.Time{}, identityclient.ErrDueSoonNotConfigured
	}
	if !asOf.Equal(runAt) || linhVuc != "" {
		return time.Time{}, fmt.Errorf("as_of %s / linh_vuc %q không phải claimed_at / hàng mặc định", asOf, linhVuc)
	}
	return soonCutoff, nil
}

func (f *fakeAutomationIdentity) EscalationInstants(_ context.Context, k identityv1.WorkKind, _ string,
	missedAt []time.Time) (map[time.Time]identityclient.EscalationInstants, error) {
	f.kindsSeen[k] = true
	if f.escalationMissed {
		return nil, identityclient.ErrAutomationNotConfigured
	}
	out := map[time.Time]identityclient.EscalationInstants{}
	for _, m := range missedAt {
		out[m.UTC()] = identityclient.EscalationInstants{UnitHeadDueAt: headDueAt, ChairmanDueAt: f.chairDue}
	}
	return out, nil
}

func (f *fakeAutomationIdentity) UnassignedHoldInstants(_ context.Context, k identityv1.WorkKind, _ string,
	starts []time.Time) (bool, map[time.Time]time.Time, error) {
	f.kindsSeen[k] = true
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
	f.permsSeen[key] = true
	out := map[string][]string{}
	if key != permDocumentRoute && key != permLetterBook {
		return out, nil
	}
	for _, u := range units {
		if codes := f.holders[communeOf(ctx)][u]; len(codes) > 0 {
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

type fakeIncomingReader struct {
	byCommune map[tenant.ID][]domain.AutomationRecord
	panicFor  tenant.ID
	err       error
}

func (f *fakeIncomingReader) OpenIncomingForAutomation(ctx context.Context) ([]domain.AutomationRecord, error) {
	if f.panicFor != "" && communeOf(ctx) == f.panicFor {
		panic("giả lập: kho hỏng")
	}
	if f.err != nil {
		return nil, f.err
	}
	return f.byCommune[communeOf(ctx)], nil
}

// fakeLetterReader is the letter register's read, keyed by commune like the incoming one.
type fakeLetterReader struct {
	byCommune map[tenant.ID][]domain.AutomationRecord
	reads     int
}

func (f *fakeLetterReader) OpenLettersForAutomation(ctx context.Context) ([]domain.AutomationRecord, error) {
	f.reads++
	return f.byCommune[communeOf(ctx)], nil
}

type harness struct {
	runner   *AutomationRunner
	identity *fakeAutomationIdentity
	comms    *fakeComms
	incoming *fakeIncomingReader
	letters  *fakeLetterReader
	locks    *fakeLocks
	registry *fakeRegistry
}

func newHarness(t *testing.T, communes ...tenant.ID) *harness {
	t.Helper()
	h := &harness{
		identity: newFakeIdentity(), comms: &fakeComms{},
		incoming: &fakeIncomingReader{byCommune: map[tenant.ID][]domain.AutomationRecord{}},
		letters:  &fakeLetterReader{byCommune: map[tenant.ID][]domain.AutomationRecord{}},
		locks:    &fakeLocks{}, registry: &fakeRegistry{},
	}
	r, err := NewAutomationRunner(AutomationDeps{Communes: &fakeCommunes{ids: communes}, Locks: h.locks,
		Registry: h.registry, Identity: h.identity, Comms: h.comms, Incoming: h.incoming, Letters: h.letters,
		Log: quietLogger})
	if err != nil {
		t.Fatal(err)
	}
	h.runner = r
	return h
}

func (h *harness) claim(id tenant.ID, runID string, job identityv1.AutomationJob) {
	h.identity.runs[id] = append(h.identity.runs[id], identityclient.AutomationRun{RunID: runID, Job: job,
		WorkKind: kindIncoming, ClaimedAt: runAt})
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

func (h *harness) recipientsByKey() map[string]string {
	to := map[string]string{}
	for _, d := range h.comms.delivered {
		to[d.n.IdempotencyKey] = strings.Join(d.n.RecipientMa, ",")
	}
	return to
}

const (
	jobSLA       = identityv1.AutomationJob_AUTOMATION_JOB_SLA_REMINDERS
	jobEscalate  = identityv1.AutomationJob_AUTOMATION_JOB_ESCALATION
	jobDigest    = identityv1.AutomationJob_AUTOMATION_JOB_WEEKLY_DIGEST
	kindIncoming = identityv1.WorkKind_WORK_KIND_VAN_BAN_DEN
	kindLetter   = identityv1.WorkKind_WORK_KIND_DON_THU
	succeeded    = identityv1.AutomationRunOutcome_AUTOMATION_RUN_OUTCOME_SUCCEEDED
)

func doc(id string, no int, deadline time.Time) domain.AutomationRecord {
	return domain.AutomationRecord{ID: id, Code: domain.MaVanBanDen(2026, no), Deadline: deadline}
}

// letter is an open citizen letter WITH a clerk-set deadline — the only kind the store returns.
func letter(id string, no int, deadline time.Time) domain.AutomationRecord {
	return domain.AutomationRecord{ID: id, Code: domain.LetterAuditSubject(2026, no), Deadline: deadline}
}

func (h *harness) claimLetters(id tenant.ID, runID string, job identityv1.AutomationJob) {
	h.identity.runs[id] = append(h.identity.runs[id], identityclient.AutomationRun{RunID: runID, Job: job,
		WorkKind: kindLetter, ClaimedAt: runAt})
}

// --- tests -----------------------------------------------------------------------------------------

func TestNewAutomationRunnerRefusesMissingDependency(t *testing.T) {
	if _, err := NewAutomationRunner(AutomationDeps{}); err == nil {
		t.Error("thiếu phụ thuộc mà vẫn dựng được bộ chạy")
	}
}

func TestClaimRunAndRecordOutcome(t *testing.T) {
	h := newHarness(t, communeA)
	late, soon, calm := doc("vb-late", 1, missed), doc("vb-soon", 2, soonDue), doc("vb-calm", 3, time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC))
	late.AssigneeMa, soon.AssigneeMa, calm.AssigneeMa = "CB-001", "CB-001", "CB-001"
	h.incoming.byCommune[communeA] = []domain.AutomationRecord{late, soon, calm}
	h.claim(communeA, "run-1", jobSLA)
	h.runner.Tick(context.Background())

	// Six scopes: three jobs × {VAN_BAN_DEN, DON_THU}. Never a petitions kind.
	got := h.identity.claimed[communeA]
	if len(got) != 6 {
		t.Fatalf("claimed %d phạm vi, muốn 6", len(got))
	}
	perKind := map[identityv1.WorkKind]int{}
	for _, s := range got {
		perKind[s.GetWorkKind()]++
	}
	if perKind[kindIncoming] != 3 || perKind[kindLetter] != 3 {
		t.Errorf("phạm vi theo loại = %v, muốn 3 văn bản đến + 3 đơn thư", perKind)
	}
	want := []string{"sla_reminders:due_soon:van-ban-den:2026-09-29:CB-001", "sla_reminders:overdue:van-ban-den:vb-late:2026-09-29"}
	if keys := h.keys(communeA); strings.Join(keys, "|") != strings.Join(want, "|") {
		t.Errorf("khoá = %v, muốn %v", keys, want)
	}
	o := h.identity.recorded["run-1"]
	if o.Outcome != succeeded || o.RecordsExamined != 3 || o.NoticesDelivered != 2 || o.RecordsWithoutRecipient != 0 {
		t.Errorf("kết quả = %+v", o)
	}
	for k := range h.identity.kindsSeen {
		if k != kindIncoming {
			t.Errorf("hỏi identity với loại việc %v", k)
		}
	}
	if h.locks.releases() != 1 {
		t.Errorf("khoá nhả %d lần, muốn 1", h.locks.releases())
	}
}

func TestTwoCommunesNeverMix(t *testing.T) {
	h := newHarness(t, communeA, communeB)
	a, b := doc("vb-alpha", 1, missed), doc("vb-beta", 1, missed)
	a.AssigneeMa, b.AssigneeMa = "CB-ALPHA", "CB-BETA"
	h.incoming.byCommune[communeA] = []domain.AutomationRecord{a}
	h.incoming.byCommune[communeB] = []domain.AutomationRecord{b}
	h.claim(communeA, "run-a", jobSLA)
	h.claim(communeB, "run-b", jobSLA)
	h.runner.Tick(context.Background())

	for _, d := range h.comms.delivered {
		text := strings.ToLower(d.n.IdempotencyKey + strings.Join(d.n.RecipientMa, ","))
		mine, other := "alpha", "beta"
		if d.commune == communeB {
			mine, other = "beta", "alpha"
		}
		if strings.Contains(text, other) || !strings.Contains(text, mine) {
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

// A crashed run is redone by a later run with ANOTHER run id: the keys are the same, and comms creates
// nothing twice.
func TestKeysStableAcrossRetriedRuns(t *testing.T) {
	h := newHarness(t, communeA)
	d := doc("vb-1", 1, missed)
	d.AssigneeMa = "CB-001"
	h.incoming.byCommune[communeA] = []domain.AutomationRecord{d}
	h.claim(communeA, "run-1", jobSLA)
	h.runner.Tick(context.Background())
	h.claim(communeA, "run-2", jobSLA)
	h.runner.Tick(context.Background())

	all := h.keys(communeA)
	if len(all) != 2 || all[0] != all[1] {
		t.Errorf("khoá qua hai lượt = %v", all)
	}
	if o := h.identity.recorded["run-2"]; o.NoticesDelivered != 0 || o.Outcome != succeeded {
		t.Errorf("lượt làm lại tạo thêm thông báo: %+v", o)
	}
}

func TestRecipientChainAndUnassignedHold(t *testing.T) {
	h := newHarness(t, communeA)
	h.identity.holders[communeA] = map[string][]string{"bp-1": {"CB-ROUTE"}}
	h.identity.leaders[communeA] = []string{"CB-LD"}
	unit := doc("vb-unit", 1, missed)
	unit.OrgUnitID, unit.HoldStartedAt = "bp-1", holdStart
	none := doc("vb-none", 2, missed)
	h.incoming.byCommune[communeA] = []domain.AutomationRecord{unit, none}
	h.claim(communeA, "run-1", jobSLA)
	h.runner.Tick(context.Background())

	to := h.recipientsByKey()
	if to["sla_reminders:overdue:van-ban-den:vb-unit:2026-09-29"] != "CB-ROUTE" {
		t.Errorf("văn bản của bộ phận không tới người phân luồng: %v", to)
	}
	if to["sla_reminders:overdue:van-ban-den:vb-none:2026-09-29"] != "CB-LD" {
		t.Errorf("văn bản không người không bộ phận không tới lãnh đạo: %v", to)
	}
	if to[domain.UnassignedKey(domain.AutomationIncomingDocument, "vb-unit", holdStart)] != "CB-ROUTE" {
		t.Errorf("không báo bộ phận giữ văn bản chưa phân công: %v", to)
	}
	if !h.identity.permsSeen[permDocumentRoute] || len(h.identity.permsSeen) != 1 {
		t.Errorf("hỏi người giữ theo khoá %v, muốn đúng document.route", h.identity.permsSeen)
	}

	// The commune's NULL: nothing reported for the hold, nothing counted as a fault.
	h2 := newHarness(t, communeA)
	h2.identity.holdDisabled = true
	h2.identity.holders = h.identity.holders
	h2.incoming.byCommune = h.incoming.byCommune
	h2.claim(communeA, "run-2", jobSLA)
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

// End to end through Tick (ADR 0079 lô 5 Q13): each recipient's due-soon notice reaches comms with
// exactly the documents its body counts — the named holder's own, the unit's routers' unit documents —
// each with the deadline AS STORED, earliest first; overdue notices carry none.
func TestDueSoonItemsReachCommsPerRecipient(t *testing.T) {
	h := newHarness(t, communeA)
	h.identity.holders[communeA] = map[string][]string{"bp-1": {"CB-ROUTE"}}
	dLater := soonDue.Add(90 * time.Minute)
	late := doc("vb-late", 1, missed)
	late.AssigneeMa = "CB-001"
	named2, named3 := doc("vb-named2", 2, dLater), doc("vb-named3", 3, soonDue)
	named2.AssigneeMa, named3.AssigneeMa = "CB-001", "CB-001"
	unit := doc("vb-unit", 4, dLater)
	unit.OrgUnitID = "bp-1"
	h.incoming.byCommune[communeA] = []domain.AutomationRecord{late, named2, named3, unit}
	h.claim(communeA, "run-1", jobSLA)
	h.runner.Tick(context.Background())

	type item struct {
		code string
		at   time.Time
	}
	want := map[string][]item{
		"sla_reminders:due_soon:van-ban-den:2026-09-29:CB-001":   {{"VB-DEN-2026-0003", soonDue}, {"VB-DEN-2026-0002", dLater}},
		"sla_reminders:due_soon:van-ban-den:2026-09-29:CB-ROUTE": {{"VB-DEN-2026-0004", dLater}},
	}
	seen := 0
	for _, d := range h.comms.delivered {
		if d.n.Kind != commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_DOCUMENT_DUE_SOON {
			if len(d.n.DueSoonItems) != 0 {
				t.Errorf("%s không phải sắp đến hạn mà mang %d mục", d.n.IdempotencyKey, len(d.n.DueSoonItems))
			}
			continue
		}
		seen++
		w, ok := want[d.n.IdempotencyKey]
		if !ok {
			t.Errorf("khoá sắp đến hạn lạ %q", d.n.IdempotencyKey)
			continue
		}
		got := d.n.DueSoonItems
		if len(got) != len(w) {
			t.Errorf("%s: mục %+v, muốn %+v", d.n.IdempotencyKey, got, w)
			continue
		}
		for i := range w {
			if got[i].Code != w[i].code || !got[i].Deadline.Equal(w[i].at) || !strings.Contains(d.n.Body, w[i].code) {
				t.Errorf("%s: mục %d = %+v, muốn %s@%v (và có trong nội dung %q)", d.n.IdempotencyKey, i, got[i], w[i].code, w[i].at, d.n.Body)
			}
		}
	}
	if seen != len(want) {
		t.Errorf("comms nhận %d thông báo sắp đến hạn, muốn %d", seen, len(want))
	}
}

func TestNobodyToTellIsCounted(t *testing.T) {
	h := newHarness(t, communeA) // no holder, no leadership flagged
	h.incoming.byCommune[communeA] = []domain.AutomationRecord{doc("vb-1", 1, missed)}
	h.claim(communeA, "run-1", jobSLA)
	h.runner.Tick(context.Background())
	if o := h.identity.recorded["run-1"]; o.RecordsWithoutRecipient != 1 || h.comms.calls != 0 || o.Outcome != succeeded {
		t.Errorf("không người nhận: %+v, comms gọi %d lần", o, h.comms.calls)
	}
}

func TestEscalationLevels(t *testing.T) {
	h := newHarness(t, communeA)
	h.identity.holders[communeA] = map[string][]string{"bp-1": {"CB-HEAD"}}
	h.identity.leaders[communeA] = []string{"CB-LD"}
	late := doc("vb-1", 1, missed)
	late.OrgUnitID, late.AssigneeMa = "bp-1", "CB-X"
	h.incoming.byCommune[communeA] = []domain.AutomationRecord{late, doc("vb-early", 2, time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC))}
	h.claim(communeA, "run-1", jobEscalate)
	h.runner.Tick(context.Background())
	if got := h.keys(communeA); strings.Join(got, "|") != "escalation:van-ban-den:vb-1:unit_head" {
		t.Fatalf("mức 1 = %v, muốn đúng một khoá unit_head", got)
	}
	if to := h.comms.delivered[0].n.RecipientMa; strings.Join(to, ",") != "CB-HEAD" {
		t.Errorf("mức 1 phải tới người phân luồng của bộ phận, không phải người xử lý: %v", to)
	}

	// Later: both thresholds passed. The unit head is not told twice; the leadership once.
	h.identity.chairDue = headDueAt
	h.claim(communeA, "run-2", jobEscalate)
	h.runner.Tick(context.Background())
	if got := h.keys(communeA); strings.Join(got, "|") !=
		"escalation:van-ban-den:vb-1:chairman|escalation:van-ban-den:vb-1:unit_head|escalation:van-ban-den:vb-1:unit_head" {
		t.Errorf("hai mức = %v", got)
	}
	if o := h.identity.recorded["run-2"]; o.NoticesDelivered != 1 {
		t.Errorf("lượt 2 tạo %d thông báo, muốn 1 (chỉ mức lãnh đạo mới)", o.NoticesDelivered)
	}

	// No configuration: nothing escalates from a default, and the run says so.
	h3 := newHarness(t, communeA)
	h3.identity.escalationMissed = true
	h3.identity.leaders = h.identity.leaders
	h3.incoming.byCommune = h.incoming.byCommune
	h3.claim(communeA, "run-3", jobEscalate)
	h3.runner.Tick(context.Background())
	if o := h3.identity.recorded["run-3"]; o.Outcome != identityv1.AutomationRunOutcome_AUTOMATION_RUN_OUTCOME_CONFIGURATION_MISSING ||
		h3.comms.calls != 0 {
		t.Errorf("thiếu cấu hình leo thang: %+v, comms %d", o, h3.comms.calls)
	}
}

// Every notice shape → its per-domain wire kind (ADR 0079 lô 2 Q3), under the SAME keys it had when it
// went out as the legacy DUE_SOON / OVERDUE / ESCALATION. The keys are literals on purpose: comms
// deduplicates on (commune, key, recipient), so a key that moved with the kind would re-deliver every
// notice already sent that day.
var documentNoticeShapes = map[string]commsv1.StaffNotificationKind{
	"sla_reminders:due_soon:van-ban-den:2026-09-29:CB-001":    commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_DOCUMENT_DUE_SOON,
	"sla_reminders:overdue:van-ban-den:vb-unit:2026-09-29":    commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_DOCUMENT_OVERDUE,
	"sla_reminders:unassigned:van-ban-den:vb-unit:1789869600": commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_DOCUMENT_UNASSIGNED,
	"escalation:van-ban-den:vb-unit:unit_head":                commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_DOCUMENT_ESCALATION,
	"weekly_digest:van-ban-den:2026-W40":                      commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_WEEKLY_DIGEST,
}

// documentShapesHarness has one document of each shape and one run of each job claimed.
func documentShapesHarness(t *testing.T) *harness {
	t.Helper()
	h := newHarness(t, communeA)
	h.identity.holders[communeA] = map[string][]string{"bp-1": {"CB-ROUTE"}}
	h.identity.leaders[communeA] = []string{"CB-LD"}
	soon := doc("vb-soon", 2, soonDue)
	soon.AssigneeMa = "CB-001"
	held := doc("vb-unit", 1, missed)
	held.OrgUnitID, held.HoldStartedAt = "bp-1", holdStart
	h.incoming.byCommune[communeA] = []domain.AutomationRecord{soon, held}
	h.claim(communeA, "run-sla", jobSLA)
	h.claim(communeA, "run-esc", jobEscalate)
	h.claim(communeA, "run-dig", jobDigest)
	return h
}

func TestEveryNoticeGoesOutUnderItsDocumentKind(t *testing.T) {
	h := documentShapesHarness(t)
	h.runner.Tick(context.Background())

	got := map[string]commsv1.StaffNotificationKind{}
	for _, d := range h.comms.delivered {
		got[d.n.IdempotencyKey] = d.n.Kind
	}
	if len(got) != len(documentNoticeShapes) {
		t.Errorf("giao %d khoá, muốn %d: %v", len(got), len(documentNoticeShapes), got)
	}
	for key, want := range documentNoticeShapes {
		if k, ok := got[key]; !ok || k != want {
			t.Errorf("khoá %s: loại = %v (có=%v), muốn %v", key, k, ok, want)
		}
	}
}

// A commune told under the legacy kinds this morning is not told again after the switch: the keys and
// recipients are what comms already holds.
func TestSwitchingKindMidDayDoesNotRedeliver(t *testing.T) {
	h := documentShapesHarness(t)
	h.comms.seen = map[string]bool{}
	for key, ma := range map[string]string{
		"sla_reminders:due_soon:van-ban-den:2026-09-29:CB-001":    "CB-001",
		"sla_reminders:overdue:van-ban-den:vb-unit:2026-09-29":    "CB-ROUTE",
		"sla_reminders:unassigned:van-ban-den:vb-unit:1789869600": "CB-ROUTE",
		"escalation:van-ban-den:vb-unit:unit_head":                "CB-ROUTE",
		"weekly_digest:van-ban-den:2026-W40":                      "CB-LD",
	} {
		h.comms.seen[string(communeA)+"|"+key+"|"+ma] = true
	}
	h.runner.Tick(context.Background())

	for _, run := range []string{"run-sla", "run-esc", "run-dig"} {
		if o := h.identity.recorded[run]; o.Outcome != succeeded || o.NoticesDelivered != 0 {
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
	for _, k := range []domain.NoticeKind{domain.NoticeDueSoon, domain.NoticeOverdue, domain.NoticeUnassigned,
		domain.NoticeEscalation, domain.NoticeWeeklyDigest} {
		w, ok := commsKind(k)
		if !ok || legacy[w] {
			t.Errorf("loại %d → %v (ok=%v)", k, w, ok)
		}
	}
	if _, ok := commsKind(domain.NoticeKind(0)); ok {
		t.Error("loại 0 được nhận")
	}
}

func TestWeeklyDigestContent(t *testing.T) {
	h := newHarness(t, communeA)
	h.identity.leaders[communeA] = []string{"CB-LD1", "CB-LD2"}
	h.incoming.byCommune[communeA] = []domain.AutomationRecord{
		doc("1", 1, missed),
		doc("2", 2, soonDue),
		doc("3", 3, time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)), // beyond the week
	}
	h.claim(communeA, "run-d", jobDigest)
	h.runner.Tick(context.Background())

	if len(h.comms.delivered) != 1 {
		t.Fatalf("giao %d bản tin, muốn 1", len(h.comms.delivered))
	}
	n := h.comms.delivered[0].n
	if n.IdempotencyKey != "weekly_digest:van-ban-den:2026-W40" ||
		n.Body != "1 văn bản đến quá hạn xử lý, 1 văn bản đến sắp đến hạn trong 7 ngày tới." ||
		strings.Join(n.RecipientMa, ",") != "CB-LD1,CB-LD2" ||
		n.Kind != commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_WEEKLY_DIGEST {
		t.Errorf("bản tin văn bản = %+v", n)
	}
	if o := h.identity.recorded["run-d"]; o.RecordsExamined != 3 || o.Outcome != succeeded {
		t.Errorf("kết quả bản tin = %+v", o)
	}

	// No leadership flagged: nothing sent, the summarised documents counted — never a fallback.
	h2 := newHarness(t, communeA)
	h2.incoming.byCommune = h.incoming.byCommune
	h2.claim(communeA, "run-d2", jobDigest)
	h2.runner.Tick(context.Background())
	if h2.comms.calls != 0 || h2.identity.recorded["run-d2"].RecordsWithoutRecipient != 2 {
		t.Errorf("không có lãnh đạo: gọi comms %d lần, kết quả %+v", h2.comms.calls, h2.identity.recorded["run-d2"])
	}
}

// No due-soon threshold withholds only what depended on it; the run says so.
func TestConfigurationMissingIsRecordedAndNothingInvented(t *testing.T) {
	h := newHarness(t, communeA)
	h.identity.dueSoonMissing = true
	late, soon := doc("vb-late", 1, missed), doc("vb-soon", 2, soonDue)
	late.AssigneeMa, soon.AssigneeMa = "CB-001", "CB-001"
	h.incoming.byCommune[communeA] = []domain.AutomationRecord{late, soon}
	h.claim(communeA, "run-1", jobSLA)
	h.runner.Tick(context.Background())
	if got := h.keys(communeA); len(got) != 1 || !strings.Contains(got[0], "overdue") {
		t.Errorf("khoá = %v — chỉ thông báo quá hạn (không cần cấu hình) được gửi", got)
	}
	if o := h.identity.recorded["run-1"]; o.Outcome != identityv1.AutomationRunOutcome_AUTOMATION_RUN_OUTCOME_CONFIGURATION_MISSING {
		t.Errorf("kết quả = %v", o.Outcome)
	}
}

func TestOutagesAreDependencyUnavailable(t *testing.T) {
	unavailable := identityv1.AutomationRunOutcome_AUTOMATION_RUN_OUTCOME_DEPENDENCY_UNAVAILABLE

	h := newHarness(t, communeA)
	h.comms.err = fmt.Errorf("wrap: %w", commsclient.ErrCommsUnavailable)
	d := doc("vb", 1, missed)
	d.AssigneeMa = "CB-1"
	h.incoming.byCommune[communeA] = []domain.AutomationRecord{d}
	h.claim(communeA, "run-1", jobSLA)
	h.runner.Tick(context.Background())
	if o := h.identity.recorded["run-1"]; o.Outcome != unavailable {
		t.Errorf("comms xuống: %v", o.Outcome)
	}

	h2 := newHarness(t, communeA)
	h2.identity.unavailable = true
	s := doc("vb", 1, soonDue)
	s.AssigneeMa = "CB-1"
	h2.incoming.byCommune[communeA] = []domain.AutomationRecord{s}
	h2.claim(communeA, "run-2", jobSLA)
	h2.runner.Tick(context.Background())
	if o := h2.identity.recorded["run-2"]; o.Outcome != unavailable || h2.comms.calls != 0 {
		t.Errorf("identity xuống: %v, comms gọi %d lần — không được giao từ kế hoạch dở", o.Outcome, h2.comms.calls)
	}

	h3 := newHarness(t, communeA)
	h3.incoming.err = errors.New("pg xuống")
	h3.claim(communeA, "run-3", jobSLA)
	h3.runner.Tick(context.Background())
	if o := h3.identity.recorded["run-3"]; o.Outcome != unavailable {
		t.Errorf("CSDL của documents xuống: %v", o.Outcome)
	}
}

func TestOneCommuneFailingNeverStopsTheNext(t *testing.T) {
	h := newHarness(t, communeA, communeB)
	h.incoming.panicFor = communeA
	b := doc("vb-b", 1, missed)
	b.AssigneeMa = "CB-B"
	h.incoming.byCommune[communeB] = []domain.AutomationRecord{b}
	h.claim(communeA, "run-a", jobSLA)
	h.claim(communeB, "run-b", jobSLA)
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

// A run for a kind this runner does not execute is FAILED, never run as one of its own registers.
func TestForeignKindRunIsFailed(t *testing.T) {
	h := newHarness(t, communeA)
	h.incoming.byCommune[communeA] = []domain.AutomationRecord{doc("vb-1", 1, missed)}
	h.identity.runs[communeA] = []identityclient.AutomationRun{{RunID: "run-x", Job: jobSLA,
		WorkKind: identityv1.WorkKind_WORK_KIND_PHAN_ANH, ClaimedAt: runAt}}
	h.runner.Tick(context.Background())
	if o := h.identity.recorded["run-x"]; o.Outcome != identityv1.AutomationRunOutcome_AUTOMATION_RUN_OUTCOME_FAILED ||
		h.comms.calls != 0 || h.letters.reads != 0 {
		t.Errorf("lượt phản ánh: %+v", o)
	}
}

// --- citizen letters (ADR 0079 lô 5 Q18) -------------------------------------------------------------

// A DON_THU run reads the LETTER register only, asks identity with DON_THU, routes by
// `petition.create`, names letters by their DT- code and keys every notice under `don-thu` — so a
// letter's keys can never collide with an incoming document's, even for the same internal id.
func TestLetterRunUsesTheLetterRegisterKindKeyAndRouteKey(t *testing.T) {
	h := newHarness(t, communeA)
	h.identity.holders[communeA] = map[string][]string{"bp-1": {"CB-BOOK"}}
	h.identity.leaders[communeA] = []string{"CB-LD"}
	late := letter("same-id", 7, missed)
	late.OrgUnitID, late.HoldStartedAt = "bp-1", holdStart
	soon := letter("dt-soon", 8, soonDue)
	soon.AssigneeMa = "CB-001"
	h.letters.byCommune[communeA] = []domain.AutomationRecord{late, soon}
	// An incoming document with the SAME internal id must not be read, nor keyed alike.
	h.incoming.byCommune[communeA] = []domain.AutomationRecord{doc("same-id", 7, missed)}
	h.claimLetters(communeA, "run-l", jobSLA)
	h.runner.Tick(context.Background())

	want := []string{
		"sla_reminders:due_soon:don-thu:2026-09-29:CB-001",
		"sla_reminders:overdue:don-thu:same-id:2026-09-29",
		"sla_reminders:unassigned:don-thu:same-id:" + fmt.Sprint(holdStart.Unix()),
	}
	if got := h.keys(communeA); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("khoá = %v, muốn %v", got, want)
	}
	for k := range h.identity.kindsSeen {
		if k != kindLetter {
			t.Errorf("lượt đơn thư hỏi identity với loại việc %v", k)
		}
	}
	if !h.identity.permsSeen[permLetterBook] || len(h.identity.permsSeen) != 1 {
		t.Errorf("hỏi người giữ theo khoá %v, muốn đúng petition.create", h.identity.permsSeen)
	}
	to := h.recipientsByKey()
	if to["sla_reminders:overdue:don-thu:same-id:2026-09-29"] != "CB-BOOK" {
		t.Errorf("đơn của bộ phận không tới người tiếp nhận của bộ phận: %v", to)
	}
	for _, d := range h.comms.delivered {
		n := d.n
		if strings.Contains(n.Title+n.Body, "VB-DEN") || strings.Contains(n.Title, "văn bản") {
			t.Errorf("thông báo đơn thư nói về văn bản đến: %+v", n)
		}
		switch n.Kind {
		case commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_DOCUMENT_OVERDUE:
			if n.Title != "Đơn thư DT-2026-0007 đã quá hạn xử lý" {
				t.Errorf("tiêu đề quá hạn = %q", n.Title)
			}
		case commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_DOCUMENT_DUE_SOON:
			// The deadline travels AS STORED (the clerk's instant), never recomputed (rule 10).
			if len(n.DueSoonItems) != 1 || n.DueSoonItems[0].Code != "DT-2026-0008" || !n.DueSoonItems[0].Deadline.Equal(soonDue) ||
				n.Title != "Bạn có 1 đơn thư sắp đến hạn xử lý" {
				t.Errorf("sắp đến hạn = %+v", n)
			}
		case commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_DOCUMENT_UNASSIGNED:
		default:
			t.Errorf("loại thông báo đơn thư = %v — phải là loại DOCUMENT_*", n.Kind)
		}
	}
	if o := h.identity.recorded["run-l"]; o.Outcome != succeeded || o.RecordsExamined != 2 || o.NoticesDelivered != 3 {
		t.Errorf("kết quả lượt đơn thư = %+v", o)
	}
}

// A commune whose letters all have "Không đặt" gets nothing — the store returns no record, and the run
// is an honest SUCCEEDED over zero letters, never a reminder from a default deadline.
func TestLettersWithoutDeadlineAreNeverReminded(t *testing.T) {
	h := newHarness(t, communeA)
	h.identity.leaders[communeA] = []string{"CB-LD"}
	for _, j := range []identityv1.AutomationJob{jobSLA, jobEscalate, jobDigest} {
		h.claimLetters(communeA, "run-"+j.String(), j)
	}
	h.runner.Tick(context.Background())
	for _, d := range h.comms.delivered {
		if d.n.Kind != commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_WEEKLY_DIGEST {
			t.Errorf("không đơn nào có hạn mà vẫn nhắc: %+v", d.n)
		}
	}
	for _, j := range []identityv1.AutomationJob{jobSLA, jobEscalate, jobDigest} {
		if o := h.identity.recorded["run-"+j.String()]; o.Outcome != succeeded || o.RecordsExamined != 0 {
			t.Errorf("%s: %+v", j, o)
		}
	}
}

func TestLetterEscalationAndDigest(t *testing.T) {
	h := newHarness(t, communeA)
	h.identity.holders[communeA] = map[string][]string{"bp-1": {"CB-BOOK"}}
	h.identity.leaders[communeA] = []string{"CB-LD"}
	late := letter("dt-late", 3, missed)
	late.OrgUnitID = "bp-1"
	h.letters.byCommune[communeA] = []domain.AutomationRecord{late, letter("dt-soon", 4, soonDue)}
	h.claimLetters(communeA, "run-e", jobEscalate)
	h.claimLetters(communeA, "run-d", jobDigest)
	h.runner.Tick(context.Background())

	got := map[string]commsclient.Notice{}
	for _, d := range h.comms.delivered {
		got[d.n.IdempotencyKey] = d.n
	}
	esc, ok := got["escalation:don-thu:dt-late:unit_head"]
	if !ok || strings.Join(esc.RecipientMa, ",") != "CB-BOOK" || esc.Title != "Báo cáo việc trễ hạn: đơn thư DT-2026-0003" ||
		esc.Kind != commsv1.StaffNotificationKind_STAFF_NOTIFICATION_KIND_DOCUMENT_ESCALATION {
		t.Errorf("leo thang đơn thư = %+v (có=%v)", esc, ok)
	}
	dig, ok := got["weekly_digest:don-thu:2026-W40"]
	if !ok || dig.Title != "Bản tin đầu tuần: đơn thư" ||
		dig.Body != "1 đơn thư quá hạn xử lý, 1 đơn thư sắp đến hạn trong 7 ngày tới." {
		t.Errorf("bản tin đơn thư = %+v (có=%v)", dig, ok)
	}
	if len(got) != 2 {
		t.Errorf("khoá = %v, muốn đúng leo thang mức 1 và bản tin", got)
	}
}

func TestOnlyLockedJobsAreClaimed(t *testing.T) {
	h := newHarness(t, communeA)
	h.locks.held = map[string]bool{"escalation": true}
	h.runner.Tick(context.Background())
	got := h.identity.claimed[communeA]
	if len(got) != 2 || got[0].GetJob() != jobEscalate || got[1].GetJob() != jobEscalate {
		t.Errorf("phạm vi = %v, muốn chỉ escalation × {văn bản đến, đơn thư}", got)
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
	notices = append(notices, domain.StaffNotice{Key: "k-wide", Kind: domain.NoticeOverdue, Recipients: wide, Title: "t"})
	for i := 0; i < 150; i++ {
		notices = append(notices, domain.StaffNotice{Key: fmt.Sprintf("k-%03d", i), Kind: domain.NoticeDueSoon,
			Recipients: []string{"CB-1"}, Title: "t"})
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
