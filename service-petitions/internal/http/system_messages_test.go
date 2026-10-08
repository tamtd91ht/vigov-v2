package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/store"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/app"
	"github.com/vihat/vigov/service-petitions/internal/domain"
)

// "Lời hệ thống", petitions half.
//
//	PART 1  the three configuration routes — the rule 5 invariant 7 set on each, plus what the handler
//	        owns: which key and text reach the use case, who the actor is, how each refusal is answered.
//	PART 2  the refusal branches that now read their sentence through Text: the commune's override
//	        when set, the default otherwise, the default when the read FAILS — with the status and the
//	        code unchanged in all three.
//
// The fallback rule and the transaction are app/system_message_test.go's.

const systemMessagesPath = "/api/v1/petitions-system-messages"

func overridePath(code string) string { return systemMessagesPath + "/" + code + "/override" }

// systemMessagesFake records the commune read from the context, as *store.Scoped reads it.
//
// Text FOLLOWS THE CONTRACT app.SystemMessages.Text keeps (proved there): the override of THIS commune,
// else the default; on a failed read the default WITH the error. `calls` counts the three route
// methods only, so a refusal reading its sentence does not look like a route reaching the use case.
type systemMessagesFake struct {
	calls   int
	commune tenant.ID
	actor   audit.Actor
	key     string
	text    string
	op      string
	err     error
	list    []domain.SystemMessage
	result  domain.SystemMessage

	overrides map[tenant.ID]map[string]string
	textErr   error
	textCalls int

	active  *bool
	created app.NewCustomMessage
	edited  app.CustomMessageEdit
	reason  string
}

func (f *systemMessagesFake) record(ctx context.Context, op string) {
	f.calls++
	f.op = op
	f.commune = tenant.MustFrom(ctx)
}

func (f *systemMessagesFake) setOverride(xa tenant.ID, key, text string) {
	if f.overrides == nil {
		f.overrides = map[tenant.ID]map[string]string{}
	}
	if f.overrides[xa] == nil {
		f.overrides[xa] = map[string]string{}
	}
	f.overrides[xa][key] = text
}

func (f *systemMessagesFake) Messages(ctx context.Context) ([]domain.SystemMessage, error) {
	f.record(ctx, "list")
	return f.list, f.err
}

func (f *systemMessagesFake) Text(ctx context.Context, key string) (string, error) {
	f.textCalls++
	m, ok := domain.LookupShippedMessage(key)
	if !ok {
		return "", domain.ErrUnknownMessageKey
	}
	if f.textErr != nil {
		return m.DefaultText, f.textErr
	}
	if t, ok := f.overrides[tenant.MustFrom(ctx)][key]; ok {
		return t, nil
	}
	return m.DefaultText, nil
}

func (f *systemMessagesFake) Reword(ctx context.Context, key, text string, actor audit.Actor) (domain.SystemMessage, error) {
	f.record(ctx, "reword")
	f.key, f.text, f.actor = key, text, actor
	return f.result, f.err
}

func (f *systemMessagesFake) Restore(ctx context.Context, key string, actor audit.Actor) (domain.SystemMessage, error) {
	f.record(ctx, "restore")
	f.key, f.actor = key, actor
	return f.result, f.err
}

func (f *systemMessagesFake) SetActive(ctx context.Context, key string, active bool, actor audit.Actor) (domain.SystemMessage, error) {
	f.record(ctx, "switch")
	f.key, f.actor, f.active = key, actor, &active
	return f.result, f.err
}

func (f *systemMessagesFake) CreateCustom(ctx context.Context, in app.NewCustomMessage, actor audit.Actor) (domain.SystemMessage, error) {
	f.record(ctx, "create")
	f.key, f.actor, f.created = in.Key, actor, in
	return f.result, f.err
}

func (f *systemMessagesFake) EditCustom(ctx context.Context, key string, in app.CustomMessageEdit, actor audit.Actor) (domain.SystemMessage, error) {
	f.record(ctx, "edit")
	f.key, f.actor, f.edited = key, actor, in
	return f.result, f.err
}

func (f *systemMessagesFake) DeleteCustom(ctx context.Context, key, reason string, actor audit.Actor) error {
	f.record(ctx, "delete")
	f.key, f.actor, f.reason = key, actor, reason
	return f.err
}

// --- PART 1: the configuration routes ------------------------------------------------------------

// systemMessagesServer is dungMayChu's real routes with the Checker swapped for the commune-keyed
// checkerDanhMucGia, because "right permission, wrong commune" needs grants per commune.
type systemMessagesServer struct {
	m       *mayChu
	svc     *systemMessagesFake
	checker *checkerDanhMucGia
}

func newSystemMessagesServer(t *testing.T) *systemMessagesServer {
	t.Helper()
	svc := &systemMessagesFake{}
	checker := &checkerDanhMucGia{}
	m := dungMayChu(t)
	m.dungLai(t, func(d *Deps) {
		d.Checker = checker
		d.SystemMessages = svc
	})
	return &systemMessagesServer{m: m, svc: svc, checker: checker}
}

func (s *systemMessagesServer) grant(xa tenant.ID, perms ...authz.Perm) {
	if s.checker.co == nil {
		s.checker.co = map[tenant.ID]map[authz.Perm]struct{}{}
	}
	if s.checker.co[xa] == nil {
		s.checker.co[xa] = map[authz.Perm]struct{}{}
	}
	for _, p := range perms {
		s.checker.co[xa][p] = struct{}{}
	}
}

// call sends a RAW body, so a malformed one can be sent too.
func (s *systemMessagesServer) call(method, host, path string, p *authz.Principal, body string) *httptest.ResponseRecorder {
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	r := httptest.NewRequest(method, "https://"+host+path, rd)
	r.Host = host
	r.RemoteAddr = "10.0.0.7:51000"
	r.Header.Set("Content-Type", "application/json")
	// The create route declares idem.Required; the others ignore the header.
	r.Header.Set("Idempotency-Key", "tn-system-message-0001")
	if p != nil {
		r = r.WithContext(authz.Into(r.Context(), *p))
	}
	w := httptest.NewRecorder()
	s.m.h.ServeHTTP(w, r)
	return w
}

type systemMessageRoute struct {
	name, method, path, body string
	ok                       int
	key                      string // the key the use case must receive; "" for the list
}

const customCode = "chung.loi-chao"

func systemMessageRoutes() []systemMessageRoute {
	k := domain.KeyFeedbackReasonRequired
	return []systemMessageRoute{
		{"GET", http.MethodGet, systemMessagesPath, "", http.StatusOK, ""},
		{"PUT override", http.MethodPut, overridePath(k), `{"text":"Câu của xã."}`, http.StatusOK, k},
		{"DELETE override", http.MethodDelete, overridePath(k), "", http.StatusNoContent, k},
		// ADR 0079 Q2 (migration 0033).
		{"PATCH override (switch)", http.MethodPatch, overridePath(k), `{"is_active":false}`, http.StatusOK, k},
		{"POST commune sentence", http.MethodPost, systemMessagesPath,
			`{"group_code":"chung","code":"` + customCode + `","text":"Xin chào."}`, http.StatusCreated, customCode},
		{"PATCH commune sentence", http.MethodPatch, systemMessagesPath + "/" + customCode, `{"text":"Mới."}`, http.StatusOK, customCode},
		{"DELETE commune sentence", http.MethodDelete, systemMessagesPath + "/" + customCode, `{"reason":"Không dùng"}`, http.StatusNoContent, customCode},
	}
}

func TestSystemMessages_401WithoutSession(t *testing.T) {
	for _, rt := range systemMessageRoutes() {
		t.Run(rt.name, func(t *testing.T) {
			s := newSystemMessagesServer(t)
			s.grant(xaA, "admin.lookup")
			doiMa(t, s.call(rt.method, hostA, rt.path, nil, rt.body), http.StatusUnauthorized)
			if s.svc.calls != 0 {
				t.Error("use case ran with no session")
			}
		})
	}
}

func TestSystemMessages_403WrongPermission(t *testing.T) {
	// Real, adjacent keys and deliberately not enough: working petitions, or reading the trail, is not
	// administering the commune's configuration.
	for _, rt := range systemMessageRoutes() {
		t.Run(rt.name, func(t *testing.T) {
			s := newSystemMessagesServer(t)
			s.grant(xaA, "feedback.read", "feedback.classify", "admin.audit")
			doiMa(t, s.call(rt.method, hostA, rt.path, canBoCuaXa(xaA), rt.body), http.StatusForbidden)
			if s.svc.calls != 0 {
				t.Error("use case ran without admin.lookup")
			}
			// Against a LITERAL: a fake checker grants any string (rule 5, invariant 3c).
			if got := s.checker.hoiKhoaCuoi(); got != "admin.lookup" {
				t.Errorf("route asked for %q, want admin.lookup", got)
			}
		})
	}
}

func TestSystemMessages_403RightPermissionWrongCommune(t *testing.T) {
	// Granted in commune A; the account is commune B's, signed in at commune B.
	for _, rt := range systemMessageRoutes() {
		t.Run(rt.name, func(t *testing.T) {
			s := newSystemMessagesServer(t)
			s.grant(xaA, "admin.lookup")
			doiMa(t, s.call(rt.method, hostB, rt.path, canBoCuaXa(xaB), rt.body), http.StatusForbidden)
			if s.svc.calls != 0 {
				t.Error("commune A's grant let commune B configure its sentences")
			}
		})
	}
}

func TestSystemMessages_200RightPermissionRightCommune(t *testing.T) {
	for _, rt := range systemMessageRoutes() {
		t.Run(rt.name, func(t *testing.T) {
			s := newSystemMessagesServer(t)
			s.grant(xaA, "admin.lookup")
			doiMa(t, s.call(rt.method, hostA, rt.path, canBoCuaXa(xaA), rt.body), rt.ok)
			if s.svc.calls != 1 || s.svc.commune != xaA {
				t.Fatalf("calls=%d commune=%q, want one call in the Host's commune", s.svc.calls, s.svc.commune)
			}
			if rt.method != http.MethodGet {
				// Rule 6, invariant 8: the trail names the BUSINESS CODE, never the internal id.
				if s.svc.actor.ID != maCanBo || s.svc.actor.ID == idCanBo || s.svc.actor.IP != "10.0.0.7" {
					t.Errorf("actor = %+v, want business code %s and socket IP", s.svc.actor, maCanBo)
				}
				if s.svc.key != rt.key {
					t.Errorf("key = %q, want %q", s.svc.key, rt.key)
				}
			}
		})
	}
}

func TestListSystemMessagesShape(t *testing.T) {
	s := newSystemMessagesServer(t)
	s.grant(xaA, "admin.lookup")
	at := time.Date(2026, 9, 28, 3, 0, 0, 0, time.UTC)
	s.svc.list = []domain.SystemMessage{{
		Key: domain.KeyFeedbackNeverPublic, Description: "d", DefaultText: "Mặc định.",
		CurrentText: "Câu của xã.", Overridden: true, UpdatedAt: &at, UpdatedBy: "CB-00777",
	}}
	w := s.call(http.MethodGet, hostA, systemMessagesPath+"?tenant_id=01JOTHERCOMMUNE", canBoCuaXa(xaA), "")
	doiMa(t, w, http.StatusOK)
	if s.svc.commune != xaA {
		t.Errorf("read in %q — the commune is the Host's, never a query parameter", s.svc.commune)
	}
	var out struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil || len(out.Items) != 1 {
		t.Fatalf("body %s", w.Body.String())
	}
	it := out.Items[0]
	for k, want := range map[string]any{
		"code": domain.KeyFeedbackNeverPublic, "default_text": "Mặc định.", "current_text": "Câu của xã.",
		"overridden": true, "updated_by": "CB-00777", "updated_at": "2026-09-28T03:00:00Z",
	} {
		if it[k] != want {
			t.Errorf("%s = %v, want %v", k, it[k], want)
		}
	}
	if _, has := it["key"]; has {
		t.Error("response carries `key` — tools/apidoc's credential guard refuses that field name")
	}
}

func TestListSystemMessagesOnDefaultOmitsWhoAndWhen(t *testing.T) {
	s := newSystemMessagesServer(t)
	s.grant(xaA, "admin.lookup")
	s.svc.list = []domain.SystemMessage{{Key: domain.KeyFeedbackReasonRequired, DefaultText: "M.", CurrentText: "M."}}
	w := s.call(http.MethodGet, hostA, systemMessagesPath, canBoCuaXa(xaA), "")
	doiMa(t, w, http.StatusOK)
	if strings.Contains(w.Body.String(), "updated_by") || strings.Contains(w.Body.String(), "updated_at") {
		t.Errorf("a key nobody changed names somebody: %s", w.Body.String())
	}
}

func TestRewordSystemMessagePassesTextAndReturnsMessage(t *testing.T) {
	s := newSystemMessagesServer(t)
	s.grant(xaA, "admin.lookup")
	s.svc.result = domain.SystemMessage{Key: domain.KeyFeedbackReasonRequired, CurrentText: "Câu của xã.", Overridden: true}
	w := s.call(http.MethodPut, hostA, overridePath(domain.KeyFeedbackReasonRequired), canBoCuaXa(xaA), `{"text":"Câu của xã."}`)
	doiMa(t, w, http.StatusOK)
	if s.svc.text != "Câu của xã." || s.svc.op != "reword" {
		t.Errorf("text=%q op=%q", s.svc.text, s.svc.op)
	}
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	if out["code"] != domain.KeyFeedbackReasonRequired || out["current_text"] != "Câu của xã." || out["overridden"] != true {
		t.Errorf("body %s", w.Body.String())
	}
}

func TestSystemMessageRefusalsMapped(t *testing.T) {
	for name, c := range map[string]struct {
		err  error
		want int
		code string
	}{
		"unknown key": {domain.ErrUnknownMessageKey, http.StatusNotFound, "not_found"},
		"empty text":  {domain.ErrMessageTextEmpty, http.StatusBadRequest, "invalid_request"},
		"markup":      {domain.ErrMessageTextMarkup, http.StatusBadRequest, "invalid_request"},
		"store down":  {errors.New("connection reset"), http.StatusInternalServerError, "internal"},
	} {
		t.Run(name, func(t *testing.T) {
			s := newSystemMessagesServer(t)
			s.grant(xaA, "admin.lookup")
			s.svc.err = c.err
			w := s.call(http.MethodPut, hostA, overridePath(domain.KeyFeedbackReasonRequired), canBoCuaXa(xaA), `{"text":""}`)
			doiMa(t, w, c.want)
			e := loiTra(t, w)
			if e.Code != c.code || strings.Contains(e.Message, "connection reset") {
				t.Errorf("error = %+v", e)
			}
		})
	}
}

func TestEmptyTextNamesTheRestoreAction(t *testing.T) {
	s := newSystemMessagesServer(t)
	s.grant(xaA, "admin.lookup")
	s.svc.err = domain.ErrMessageTextEmpty
	w := s.call(http.MethodPut, hostA, overridePath(domain.KeyFeedbackReasonRequired), canBoCuaXa(xaA), `{"text":"  "}`)
	doiMa(t, w, http.StatusBadRequest)
	if !strings.Contains(loiTra(t, w).Message, "Khôi phục câu mặc định") {
		t.Errorf("message %q does not point to the restore action", loiTra(t, w).Message)
	}
}

func TestUnknownKeyIs404OnBothWrites(t *testing.T) {
	for _, method := range []string{http.MethodPut, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			s := newSystemMessagesServer(t)
			s.grant(xaA, "admin.lookup")
			s.svc.err = domain.ErrUnknownMessageKey
			body := ""
			if method == http.MethodPut {
				body = `{"text":"x"}`
			}
			w := s.call(method, hostA, overridePath("budget.scope_notice"), canBoCuaXa(xaA), body)
			doiMa(t, w, http.StatusNotFound)
			if s.svc.key != "budget.scope_notice" {
				t.Errorf("key reaching use case = %q", s.svc.key)
			}
		})
	}
}

func TestUnknownKeyIs404ThroughTheRealUseCase(t *testing.T) {
	// The fake above is told to refuse; this one is not a fake: the real use case refuses a key outside
	// the catalogue BEFORE it touches a store, so a nil store is enough.
	s := newSystemMessagesServer(t)
	s.grant(xaA, "admin.lookup")
	s.m.dungLai(t, func(d *Deps) { d.SystemMessages = app.NewSystemMessages(nil, nil) })
	doiMa(t, s.call(http.MethodPut, hostA, overridePath("budget.scope_notice"), canBoCuaXa(xaA), `{"text":"x"}`),
		http.StatusNotFound)
	doiMa(t, s.call(http.MethodDelete, hostA, overridePath("report.title"), canBoCuaXa(xaA), ""),
		http.StatusNotFound)
}

func TestRewordBadJSONIs400AndCallsNothing(t *testing.T) {
	s := newSystemMessagesServer(t)
	s.grant(xaA, "admin.lookup")
	doiMa(t, s.call(http.MethodPut, hostA, overridePath(domain.KeyFeedbackReasonRequired), canBoCuaXa(xaA), `{"text":`),
		http.StatusBadRequest)
	if s.svc.calls != 0 {
		t.Error("malformed body reached the use case")
	}
}

func TestSystemMessageWriteWithoutBusinessCodeIs500(t *testing.T) {
	// No `Ma`: the trail could not name who acted, so nothing is written.
	s := newSystemMessagesServer(t)
	s.grant(xaA, "admin.lookup")
	p := &authz.Principal{ID: idCanBo, Kind: "staff", TenantID: xaA}
	for _, rt := range systemMessageRoutes() {
		if rt.method == http.MethodGet {
			continue
		}
		w := s.call(rt.method, hostA, rt.path, p, rt.body)
		if w.Code != http.StatusInternalServerError {
			t.Errorf("%s: code = %d, want 500", rt.name, w.Code)
		}
	}
	if s.svc.calls != 0 {
		t.Error("write ran for a principal with no business code")
	}
}

// --- PART 2: the refusal branches read the commune's sentence -------------------------------------

func TestEveryWiredKeyIsInTheCatalogue(t *testing.T) {
	// A key typed wrong here would make its refusal fall to the generic sentence on every commune,
	// and no status code would show it.
	keys := []string{domain.KeyFeedbackNeverPublic}
	for _, c := range refusalMessageKeys {
		keys = append(keys, c.key)
	}
	for _, k := range keys {
		if _, ok := domain.LookupShippedMessage(k); !ok {
			t.Errorf("wired key %q is not in the catalogue", k)
		}
	}
	// And no configurable refusal still has a fixed sentence: one refusal, one source.
	for _, c := range refusalMessageKeys {
		if _, fixed := cauTuChoiPhieu(c.cause); fixed {
			t.Errorf("%v has both a fixed sentence and a configurable one", c.cause)
		}
	}
}

// wiredBranch is one refusal branch that reads a "Lời hệ thống" sentence.
type wiredBranch struct {
	name   string
	key    string
	perm   authz.Perm
	method string
	path   string
	body   any
	cause  error
	status int
	code   string
}

func wiredBranches() []wiredBranch {
	// Each cause is wrapped the way it reaches the handler in production (app.bocPhieu), so a bare
	// sentinel does not pass against a matcher that forgot errors.Is.
	wrap := func(err error) error { return fmt.Errorf("xu_ly_phan_anh: thao tác cho xã %s: %w", xaA, err) }
	return []wiredBranch{
		{"không tiếp nhận · thiếu lý do", domain.KeyFeedbackReasonRequired, "feedback.classify",
			http.MethodPost, duongKhongTiepNhan(maPhieuThuong), khongTiepNhanVao{Reason: ""},
			wrap(domain.ErrThieuLyDo), http.StatusBadRequest, "invalid_request"},
		{"chuyển cấp trên · thiếu lý do", domain.KeyFeedbackReasonRequired, "feedback.classify",
			http.MethodPost, duongChuyenCapTren(maPhieuThuong), chuyenCapTrenVao{ReceivingBody: coQuanThatHTTP},
			wrap(domain.ErrThieuLyDo), http.StatusBadRequest, "invalid_request"},
		{"phân công · thiếu bộ phận", domain.KeyFeedbackAssignmentRequired, "feedback.assign",
			http.MethodPost, duongPhanCong(maPhieuThuong), phanCongVao{Unit: ""},
			wrap(domain.ErrThieuBoPhan), http.StatusBadRequest, "invalid_request"},
		{"chuyển trạng thái · không có bước tiếp", domain.KeyFeedbackInvalidTransition, "feedback.read",
			http.MethodPost, duongTienTrang(maPhieuThuong), nil,
			wrap(domain.ErrKhongConCamKet), http.StatusConflict, "petition_state"},
		{"công khai · phiếu tác phong cán bộ", domain.KeyFeedbackNeverPublic, "feedback.assign",
			http.MethodPut, duong(maPhieuThuong) + "/publication", publicationIn{Status: "cong-khai"},
			wrap(domain.ErrNeverPublic), http.StatusConflict, "never_public"},
	}
}

// runBranch issues the branch's request with the branch's refusal, and checks that the status and the
// code are the branch's own — whatever the sentence turns out to be.
func runBranch(t *testing.T, m *mayChu, b wiredBranch) string {
	t.Helper()
	m.xuLy.loi = b.cause
	w := m.goiThan(t, b.method, hostA, b.path, canBoCuaXa(xaA), b.body)
	doiMa(t, w, b.status)
	e := loiTra(t, w)
	if e.Code != b.code {
		t.Errorf("code = %q, want %q — only the sentence's source may change", e.Code, b.code)
	}
	for _, leak := range []string{string(xaA), "xu_ly_phan_anh", "phan_anh:"} {
		if strings.Contains(w.Body.String(), leak) {
			t.Errorf("body leaks %q: %s", leak, w.Body.String())
		}
	}
	return e.Message
}

func TestWiredRefusalReadsTheCommuneOverride(t *testing.T) {
	for _, b := range wiredBranches() {
		t.Run(b.name, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(t, b.perm)
			svc := &systemMessagesFake{}
			svc.setOverride(xaA, b.key, "Câu xã A đặt cho "+b.key+".")
			m.dungLai(t, func(d *Deps) { d.SystemMessages = svc })

			if got := runBranch(t, m, b); got != "Câu xã A đặt cho "+b.key+"." {
				t.Errorf("message = %q, want commune A's override", got)
			}
		})
	}
}

func TestWiredRefusalReadsTheDefaultWithoutOverride(t *testing.T) {
	for _, b := range wiredBranches() {
		t.Run(b.name, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(t, b.perm)
			svc := &systemMessagesFake{}
			// ANOTHER commune's override is not this commune's wording.
			svc.setOverride(xaB, b.key, "Câu của xã B.")
			m.dungLai(t, func(d *Deps) { d.SystemMessages = svc })

			want, _ := domain.LookupShippedMessage(b.key)
			if got := runBranch(t, m, b); got != want.DefaultText {
				t.Errorf("message = %q, want the shipped default %q", got, want.DefaultText)
			}
			if svc.textCalls != 1 {
				t.Errorf("Text called %d times, want once", svc.textCalls)
			}
		})
	}
}

// failingOverrideStore is the REAL use case's store, failing its one read. The use case above it is
// app.SystemMessages itself, so this proves the fallback end to end, not the fake's copy of it.
type failingOverrideStore struct{ reads int }

func (f *failingOverrideStore) ListLive(context.Context) ([]domain.MessageOverride, error) {
	f.reads++
	return nil, errors.New("connection reset by peer")
}
func (f *failingOverrideStore) LiveForUpdate(context.Context, *store.ScopedTx, string) (*domain.MessageOverride, error) {
	return nil, errors.New("not used")
}
func (f *failingOverrideStore) AddOverride(context.Context, *store.ScopedTx, domain.MessageOverride) error {
	return errors.New("not used")
}
func (f *failingOverrideStore) UpdateText(context.Context, *store.ScopedTx, domain.MessageOverride) error {
	return errors.New("not used")
}
func (f *failingOverrideStore) SoftDelete(context.Context, *store.ScopedTx, string, string, string, time.Time) error {
	return errors.New("not used")
}
func (f *failingOverrideStore) SetActive(context.Context, *store.ScopedTx, string, bool, string, time.Time) error {
	return errors.New("not used")
}
func (f *failingOverrideStore) ListCustom(context.Context) ([]domain.CustomMessage, error) {
	return nil, errors.New("not used")
}
func (f *failingOverrideStore) CustomForUpdate(context.Context, *store.ScopedTx, string) (*domain.CustomMessage, error) {
	return nil, errors.New("not used")
}
func (f *failingOverrideStore) CustomKeyTaken(context.Context, *store.ScopedTx, string) (bool, error) {
	return false, errors.New("not used")
}
func (f *failingOverrideStore) CountLiveCustom(context.Context, *store.ScopedTx) (int, error) {
	return 0, errors.New("not used")
}
func (f *failingOverrideStore) AddCustom(context.Context, *store.ScopedTx, domain.CustomMessage) error {
	return errors.New("not used")
}
func (f *failingOverrideStore) UpdateCustom(context.Context, *store.ScopedTx, domain.CustomMessage) error {
	return errors.New("not used")
}
func (f *failingOverrideStore) SoftDeleteCustom(context.Context, *store.ScopedTx, string, string, string, time.Time) error {
	return errors.New("not used")
}

func TestWiredRefusalFallsBackToDefaultWhenTheLookupFails(t *testing.T) {
	for _, b := range wiredBranches() {
		t.Run(b.name, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(t, b.perm)
			var logBuf bytes.Buffer
			failing := &failingOverrideStore{}
			m.dungLai(t, func(d *Deps) {
				d.SystemMessages = app.NewSystemMessages(nil, failing)
				d.Log = slog.New(slog.NewTextHandler(&logBuf, nil))
			})

			// The STATUS AND THE CODE are the branch's own (runBranch) — never a 500 because a wording
			// could not be read — and the sentence is the default.
			want, _ := domain.LookupShippedMessage(b.key)
			if got := runBranch(t, m, b); got != want.DefaultText {
				t.Errorf("message = %q, want the default on a failed lookup", got)
			}
			if failing.reads != 1 {
				t.Errorf("override read %d times, want once", failing.reads)
			}
			log := logBuf.String()
			if !strings.Contains(log, "không đọc được câu của xã") || !strings.Contains(log, b.key) ||
				!strings.Contains(log, string(xaA)) {
				t.Errorf("failed lookup not logged with commune and key: %s", log)
			}
			// Rule 3: no lookup code, no internal id, no petition content in the log line.
			if strings.Contains(log, maPhieuThuong) || strings.Contains(log, idCanBo) {
				t.Errorf("log carries the petition code or the internal id: %s", log)
			}
		})
	}
}

func TestUnwiredRefusalDoesNotReadAWording(t *testing.T) {
	// A refusal with a fixed sentence reads no wording — one lookup less on every refusal, and proof
	// the table decides, not a blanket "every refusal asks".
	m := dungMayChu(t)
	m.capQuyen(t, "feedback.classify")
	svc := &systemMessagesFake{}
	m.dungLai(t, func(d *Deps) { d.SystemMessages = svc })
	m.xuLy.loi = domain.ErrLyDoQuaNgan
	w := m.goiThan(t, http.MethodPost, hostA, duongKhongTiepNhan(maPhieuThuong), canBoCuaXa(xaA),
		khongTiepNhanVao{Reason: "ngắn"})
	doiMa(t, w, http.StatusBadRequest)
	if svc.textCalls != 0 {
		t.Errorf("Text called %d times for a refusal with a fixed sentence", svc.textCalls)
	}
}
