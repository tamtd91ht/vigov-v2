package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"testing"

	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/app"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// The four routes of /api/v1/citizen-letter-deadline-rules. Rule 5, invariant 7 asks four cases of
// each — 401 no token · 403 wrong permission (the harness default `admin.user`) · 403 right permission
// in the wrong commune · 2xx both correct — then what the HANDLER owns: body mapping, which identifier
// reaches the trail, and the status each refusal becomes. The rules themselves are proved in
// domain/ and app/.

// --- fakes ----------------------------------------------------------------------------------------

type letterRulesReadFake struct {
	mu    sync.Mutex
	calls int
	rules []domain.CitizenLetterDeadlineRule
	err   error
}

const letterRuleIDFixture = "01JLETTERRULE0000000000000"

func letterRulesReadSample() *letterRulesReadFake {
	return &letterRulesReadFake{rules: []domain.CitizenLetterDeadlineRule{
		{ID: letterRuleIDFixture, LetterType: domain.CitizenLetterKhieuNai, Kind: domain.CitizenLetterResolution,
			Amount: 30, Unit: domain.UnitCalendarDays},
		// Stored before the lock — shown with its problem, never hidden.
		{ID: "01JLETTERRULEBROKEN0000000", LetterType: domain.CitizenLetterToCao, Kind: domain.CitizenLetterProcessing,
			Amount: 7, Unit: domain.UnitWorkingHours},
	}}
}

func (f *letterRulesReadFake) List(ctx context.Context) ([]domain.CitizenLetterDeadlineRule, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	_ = tenant.MustFrom(ctx) // reached with a commune in context, or this panics
	return f.rules, f.err
}

func (f *letterRulesReadFake) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

type letterRulesWriteFake struct {
	mu sync.Mutex

	calls       int
	lastActor   app.NguoiThucHien
	lastCommune tenant.ID
	lastID      string
	lastCreate  app.CreateCitizenLetterDeadlineRuleRequest
	lastUpdate  app.UpdateCitizenLetterDeadlineRuleRequest
	lastReason  string

	result domain.CitizenLetterDeadlineRule
	err    error
}

func letterRulesWriteSample() *letterRulesWriteFake {
	return &letterRulesWriteFake{result: domain.CitizenLetterDeadlineRule{
		ID: letterRuleIDFixture, LetterType: domain.CitizenLetterKhieuNai, Kind: domain.CitizenLetterResolution,
		Amount: 30, Unit: domain.UnitCalendarDays}}
}

func (f *letterRulesWriteFake) note(ctx context.Context, actor app.NguoiThucHien) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.lastActor = actor
	f.lastCommune = tenant.MustFrom(ctx)
}

func (f *letterRulesWriteFake) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func (f *letterRulesWriteFake) Create(ctx context.Context, req app.CreateCitizenLetterDeadlineRuleRequest,
	actor app.NguoiThucHien) (domain.CitizenLetterDeadlineRule, error) {
	f.note(ctx, actor)
	f.lastCreate = req
	return f.result, f.err
}

func (f *letterRulesWriteFake) Update(ctx context.Context, id string, req app.UpdateCitizenLetterDeadlineRuleRequest,
	actor app.NguoiThucHien) (domain.CitizenLetterDeadlineRule, error) {
	f.note(ctx, actor)
	f.lastID, f.lastUpdate = id, req
	return f.result, f.err
}

func (f *letterRulesWriteFake) Remove(ctx context.Context, id, reason string, actor app.NguoiThucHien) error {
	f.note(ctx, actor)
	f.lastID, f.lastReason = id, reason
	return f.err
}

// --- harness --------------------------------------------------------------------------------------

const createLetterRuleBody = `{"letter_type":"khieu-nai","deadline_kind":"giai-quyet","amount":30,"unit":"ngay-lich"}`

func letterRuleRoutes() []tuyenSLA {
	return []tuyenSLA{
		{"đọc quy tắc", "GET", "/api/v1/citizen-letter-deadline-rules", "", http.StatusOK},
		{"thêm quy tắc", "POST", "/api/v1/citizen-letter-deadline-rules", createLetterRuleBody, http.StatusCreated},
		{"sửa quy tắc", "PATCH", "/api/v1/citizen-letter-deadline-rules/" + letterRuleIDFixture, `{"amount":45}`, http.StatusOK},
		{"xoá quy tắc", "DELETE", "/api/v1/citizen-letter-deadline-rules/" + letterRuleIDFixture, `{"reason":"Chờ pháp chế"}`, http.StatusNoContent},
	}
}

func (m *mayChu) letterRuleCalls() int { return m.letterRules.count() + m.writeLetterRules.count() }

// --- rule 5, invariant 7: four cases, every route --------------------------------------------------

func TestCitizenLetterRules_401WithoutToken(t *testing.T) {
	m := dungMayChuSLA(t)
	for _, tg := range letterRuleRoutes() {
		if w := m.goiSLA(t, tg, hostA, ""); w.Code != http.StatusUnauthorized {
			t.Errorf("%s: mã = %d, muốn 401 — %s", tg.ten, w.Code, w.Body.String())
		}
	}
	if n := m.letterRuleCalls(); n != 0 {
		t.Errorf("chưa đăng nhập mà tầng dưới chạy %d lần", n)
	}
}

// `admin.user` — a real key, the harness default — is not `admin.sla`.
func TestCitizenLetterRules_403WrongPermission(t *testing.T) {
	m := dungMayChu(t)
	tok := m.tokenCho(t, xaA, sidA)
	for _, tg := range letterRuleRoutes() {
		if w := m.goiSLA(t, tg, hostA, tok); w.Code != http.StatusForbidden {
			t.Errorf("%s: mã = %d, muốn 403 — %s", tg.ten, w.Code, w.Body.String())
		}
	}
	if n := m.letterRuleCalls(); n != 0 {
		t.Errorf("sai quyền mà tầng dưới chạy %d lần", n)
	}
}

// `admin.sla` held in commune A, signed in at commune B: the grant does not exist there.
func TestCitizenLetterRules_403RightPermissionWrongCommune(t *testing.T) {
	m := dungMayChuSLA(t)
	tok := m.tokenCho(t, xaB, sidB)
	for _, tg := range letterRuleRoutes() {
		if w := m.goiSLA(t, tg, hostB, tok); w.Code != http.StatusForbidden {
			t.Errorf("%s: mã = %d, muốn 403 — %s", tg.ten, w.Code, w.Body.String())
		}
	}
	if n := m.letterRuleCalls(); n != 0 {
		t.Errorf("sai xã mà tầng dưới chạy %d lần", n)
	}
}

func TestCitizenLetterRules_2xxBothCorrect(t *testing.T) {
	m := dungMayChuSLA(t)
	tok := m.tokenCho(t, xaA, sidA)
	for _, tg := range letterRuleRoutes() {
		if w := m.goiSLA(t, tg, hostA, tok); w.Code != tg.ok {
			t.Errorf("%s: mã = %d, muốn %d — %s", tg.ten, w.Code, tg.ok, w.Body.String())
		}
	}
	if m.writeLetterRules.lastCommune != xaA {
		t.Errorf("use case chạy ở xã %q, muốn %q", m.writeLetterRules.lastCommune, xaA)
	}
}

// --- rule 6, invariant 8: the staff code is the trail's actor ---------------------------------------

func TestCitizenLetterRulesActorIsStaffCode(t *testing.T) {
	m := dungMayChuSLA(t)
	tok := m.tokenCho(t, xaA, sidA)
	for _, tg := range letterRuleRoutes()[1:] {
		m.writeLetterRules.lastActor = app.NguoiThucHien{}
		if w := m.goiSLA(t, tg, hostA, tok); w.Code != tg.ok {
			t.Fatalf("%s: mã = %d — %s", tg.ten, w.Code, w.Body.String())
		}
		a := m.writeLetterRules.lastActor
		if a.Vet.ID != maCanBo || a.ID != idNoiBo {
			t.Errorf("%s: vết %q, quyết định %q — muốn mã cán bộ %q và id nội bộ %q", tg.ten, a.Vet.ID, a.ID, maCanBo, idNoiBo)
		}
	}
}

// --- what the handler owns ------------------------------------------------------------------------

// The list carries each rule, its required unit, and the problem of a rule the lock refuses.
func TestListCitizenLetterRulesShape(t *testing.T) {
	m := dungMayChuSLA(t)
	w := m.goiSLA(t, letterRuleRoutes()[0], hostA, m.tokenCho(t, xaA, sidA))
	if w.Code != http.StatusOK {
		t.Fatalf("mã = %d", w.Code)
	}
	var out citizenLetterDeadlineRulesOut
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil || len(out.Items) != 2 {
		t.Fatalf("thân = %s (%v)", w.Body.String(), err)
	}
	ok, broken := out.Items[0], out.Items[1]
	if ok.LetterType != "khieu-nai" || ok.DeadlineKind != "giai-quyet" || ok.Amount != 30 || ok.Unit != "ngay-lich" ||
		ok.RequiredUnit != "ngay-lich" || ok.Problem != nil {
		t.Errorf("quy tắc dùng được = %+v", ok)
	}
	if broken.RequiredUnit != "ngay-lam-viec" || broken.Problem == nil {
		t.Errorf("quy tắc vi phạm khoá = %+v — phải nêu vấn đề", broken)
	}
}

// An empty table is `items: []`, never null.
func TestListCitizenLetterRulesEmptyIsArray(t *testing.T) {
	m := dungMayChuSLA(t)
	m.letterRules.rules = nil
	w := m.goiSLA(t, letterRuleRoutes()[0], hostA, m.tokenCho(t, xaA, sidA))
	var out map[string]json.RawMessage
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &out) != nil || string(out["items"]) != "[]" {
		t.Errorf("mã = %d, thân = %q — muốn items là mảng rỗng", w.Code, w.Body.String())
	}
}

func TestCreateCitizenLetterRuleMapsBody(t *testing.T) {
	m := dungMayChuSLA(t)
	w := m.goiSLA(t, letterRuleRoutes()[1], hostA, m.tokenCho(t, xaA, sidA))
	if w.Code != http.StatusCreated {
		t.Fatalf("mã = %d — %s", w.Code, w.Body.String())
	}
	want := app.CreateCitizenLetterDeadlineRuleRequest{LetterType: domain.CitizenLetterKhieuNai,
		Kind: domain.CitizenLetterResolution, Amount: 30, Unit: domain.UnitCalendarDays}
	if m.writeLetterRules.lastCreate != want {
		t.Errorf("yêu cầu = %+v, muốn %+v", m.writeLetterRules.lastCreate, want)
	}
}

// POST without an Idempotency-Key is refused before the use case.
func TestCreateCitizenLetterRuleNeedsIdempotencyKey(t *testing.T) {
	m := dungMayChuSLA(t)
	w := m.goi(t, "POST", hostA, "/api/v1/citizen-letter-deadline-rules", createLetterRuleBody, m.tokenCho(t, xaA, sidA))
	if w.Code < 400 || m.writeLetterRules.count() != 0 {
		t.Errorf("thiếu Idempotency-Key: mã = %d, use case %d lần", w.Code, m.writeLetterRules.count())
	}
}

func TestUpdateCitizenLetterRuleMapsBodyAndRefusesEmpty(t *testing.T) {
	m := dungMayChuSLA(t)
	tok := m.tokenCho(t, xaA, sidA)
	w := m.goi(t, "PATCH", hostA, "/api/v1/citizen-letter-deadline-rules/"+letterRuleIDFixture,
		`{"amount":45,"unit":"ngay-lich"}`, tok)
	if w.Code != http.StatusOK {
		t.Fatalf("mã = %d — %s", w.Code, w.Body.String())
	}
	u := m.writeLetterRules.lastUpdate
	if m.writeLetterRules.lastID != letterRuleIDFixture || u.Amount == nil || *u.Amount != 45 ||
		u.Unit == nil || *u.Unit != domain.UnitCalendarDays {
		t.Errorf("id %q, yêu cầu %+v", m.writeLetterRules.lastID, u)
	}

	before := m.writeLetterRules.count()
	w = m.goi(t, "PATCH", hostA, "/api/v1/citizen-letter-deadline-rules/"+letterRuleIDFixture, `{}`, tok)
	if w.Code != http.StatusBadRequest || m.writeLetterRules.count() != before {
		t.Errorf("{}: mã = %d, use case chạy thêm %d lần", w.Code, m.writeLetterRules.count()-before)
	}
}

func TestRemoveCitizenLetterRulePassesReason(t *testing.T) {
	m := dungMayChuSLA(t)
	w := m.goiSLA(t, letterRuleRoutes()[3], hostA, m.tokenCho(t, xaA, sidA))
	if w.Code != http.StatusNoContent || w.Body.Len() != 0 {
		t.Fatalf("mã = %d, thân %q", w.Code, w.Body.String())
	}
	if m.writeLetterRules.lastID != letterRuleIDFixture || m.writeLetterRules.lastReason != "Chờ pháp chế" {
		t.Errorf("id %q, lý do %q", m.writeLetterRules.lastID, m.writeLetterRules.lastReason)
	}
}

func TestCitizenLetterRuleRefusalsMapToStatus(t *testing.T) {
	cases := []struct {
		err    error
		status int
		code   string
	}{
		{idstore.ErrCitizenLetterDeadlineRuleNotFound, http.StatusNotFound, "citizen_letter_deadline_rule_not_found"},
		{app.ErrCitizenLetterDeadlineRuleExists, http.StatusConflict, "citizen_letter_deadline_rule_exists"},
		{domain.ErrCitizenLetterDeadlineUnitLocked, http.StatusBadRequest, "invalid_request"},
		{domain.ErrCitizenLetterNoResolutionDeadline, http.StatusBadRequest, "invalid_request"},
		{domain.ErrCitizenLetterAmountTooLarge, http.StatusBadRequest, "invalid_request"},
		{domain.ErrCitizenLetterDeleteReasonMissing, http.StatusBadRequest, "invalid_request"},
		{errors.New("db down"), http.StatusInternalServerError, "internal"},
	}
	for _, c := range cases {
		for _, tg := range letterRuleRoutes()[1:] {
			m := dungMayChuSLA(t)
			m.writeLetterRules.err = c.err
			w := m.goiSLA(t, tg, hostA, m.tokenCho(t, xaA, sidA))
			if w.Code != c.status || errorCode(t, w.Body.Bytes()) != c.code {
				t.Errorf("%s / %v: mã = %d %s, muốn %d %s", tg.ten, c.err, w.Code, w.Body.String(), c.status, c.code)
			}
		}
	}
}

func TestListCitizenLetterRulesFailureIs500(t *testing.T) {
	m := dungMayChuSLA(t)
	m.letterRules.err = idstore.ErrTooManyCitizenLetterDeadlineRules
	if w := m.goiSLA(t, letterRuleRoutes()[0], hostA, m.tokenCho(t, xaA, sidA)); w.Code != http.StatusInternalServerError {
		t.Errorf("mã = %d, muốn 500", w.Code)
	}
}
