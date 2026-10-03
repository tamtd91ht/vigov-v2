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
	"net/url"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/ratelimit"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/app"
	"github.com/vihat/vigov/service-comms/internal/domain"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

// The five external-contact routes. Staff harness: the commune-keyed checker (checkerDanhMucGia), the
// principal injector (chuTheGhi) and the principal whose internal id and business code differ
// (canBoGhi) — what the "right permission, wrong commune" and "the trail names the business code"
// cases need. Public harness: the platform fake (ckNenTang) and the real PublicNewsRead limiter over an
// in-memory counter (memCounter), the same ones the news routes are tested with.
//
// WHICH ROWS the store returns (live only, this commune only, display order) is the store's predicate —
// internal/app/external_contact_test.go (SQL, fake driver) and internal/store/external_contact_pg_test.go
// (PostgreSQL, SKIPS without VIGOV_TEST_DSN). The fakes here apply that predicate per commune so the
// handler's own half — which commune it asks for — is what is under test.

// fakeExternalContacts stands in for the store and the use case, RECORDING THE COMMUNE it was called
// in (from the context, as *store.Scoped reads it) and the actor.
type fakeExternalContacts struct {
	byCommune map[tenant.ID][]domain.ExternalContact
	deleted   map[string]bool // ids soft-deleted: the store predicate the fake applies
	row       domain.ExternalContact
	err       error

	calls       int
	lastCommune tenant.ID
	lastActor   audit.Actor
	lastInput   app.ExternalContactInput
	lastPatch   app.ExternalContactPatch
	lastID      string
	lastReason  string
}

func (f *fakeExternalContacts) note(ctx context.Context) {
	f.calls++
	f.lastCommune = tenant.MustFrom(ctx)
}

func (f *fakeExternalContacts) List(ctx context.Context) ([]domain.ExternalContact, error) {
	f.note(ctx)
	if f.err != nil {
		return nil, f.err
	}
	var out []domain.ExternalContact
	for _, c := range f.byCommune[tenant.MustFrom(ctx)] {
		if !f.deleted[c.ID] {
			out = append(out, c)
		}
	}
	return out, nil
}

func (f *fakeExternalContacts) Create(ctx context.Context, in app.ExternalContactInput, actor audit.Actor) (domain.ExternalContact, error) {
	f.note(ctx)
	f.lastInput, f.lastActor = in, actor
	return f.row, f.err
}

func (f *fakeExternalContacts) Update(ctx context.Context, id string, p app.ExternalContactPatch, actor audit.Actor) (domain.ExternalContact, error) {
	f.note(ctx)
	f.lastID, f.lastPatch, f.lastActor = id, p, actor
	return f.row, f.err
}

func (f *fakeExternalContacts) Delete(ctx context.Context, id, reason string, actor audit.Actor) error {
	f.note(ctx)
	f.lastID, f.lastReason, f.lastActor = id, reason, actor
	return f.err
}

func contactFixture() *fakeExternalContacts {
	return &fakeExternalContacts{
		byCommune: map[tenant.ID][]domain.ExternalContact{
			xaA: {
				{ID: "ec-a-1", Name: "Công an xã", Category: "Công an", Phone: "113", DisplayOrder: intPtr(1)},
				{ID: "ec-a-2", Name: "<b>Trạm Y tế</b> xã", Category: "Y tế", Phone: "0900000000", Address: "Thôn 1"},
				{ID: "ec-a-gone", Name: "Bưu điện cũ", Category: "Khác", Phone: "0900000000"},
			},
			xaB: {{ID: "ec-b-1", Name: "LIÊN HỆ CỦA XÃ B", Category: "Khác", Phone: "114"}},
		},
		deleted: map[string]bool{"ec-a-gone": true},
		row:     domain.ExternalContact{ID: "ec-a-2", Name: "Trạm Y tế xã", Category: "Y tế", Phone: "0900000000"},
	}
}

// --- staff ------------------------------------------------------------------------------------

type contactServer struct {
	h       http.Handler
	fake    *fakeExternalContacts
	checker *checkerDanhMucGia
}

func newContactServer(t *testing.T) *contactServer {
	t.Helper()
	fake := contactFixture()
	checker := &checkerDanhMucGia{}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	mux := http.NewServeMux()
	RegisterExternalContacts(mux, ExternalContactDeps{Checker: checker, Reader: fake, Writer: fake, Log: log})

	// The real edge chain in the real order; a working idempotency store, because POST declares
	// DongKhiHong and a nil store would (correctly) answer 503.
	var h http.Handler = mux
	h = chuTheGhi(h)
	h = idem.Middleware(moKhoIdemGia(), log)(h)
	h = httpx.TenantMiddleware(thuMucMau())(h)
	h = httpx.Recover(func(context.Context) string { return "test-trace" })(h)
	h = httpx.StripTenantHeaders(h)
	return &contactServer{h: h, fake: fake, checker: checker}
}

func (s *contactServer) grant(commune tenant.ID, perms ...authz.Perm) {
	if s.checker.co == nil {
		s.checker.co = map[tenant.ID]map[authz.Perm]struct{}{}
	}
	if s.checker.co[commune] == nil {
		s.checker.co[commune] = map[authz.Perm]struct{}{}
	}
	for _, p := range perms {
		s.checker.co[commune][p] = struct{}{}
	}
}

func (s *contactServer) call(t *testing.T, method, host, path string, p *authz.Principal, body string) *httptest.ResponseRecorder {
	t.Helper()
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	r := httptest.NewRequest(method, "https://"+host+path, rd)
	r.Host = host
	r.RemoteAddr = "10.0.0.7:51000"
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set(idem.Header, "01JIDEMPOTENCYKEYCONTACTS")
	if p != nil {
		r = r.WithContext(context.WithValue(r.Context(), khoaChuTheGhi{}, *p))
	}
	w := httptest.NewRecorder()
	s.h.ServeHTTP(w, r)
	return w
}

const contactPath = "/api/v1/external-contacts"

type contactRoute struct {
	name, method, path, body string
	perm, wrongPerm          authz.Perm
	ok                       int
}

// contactRoutes — every permission case is asserted on ALL four. The wrong keys are real and adjacent:
// a map reader must not read the list; a content READER must not write it.
func contactRoutes() []contactRoute {
	return []contactRoute{
		{"GET", http.MethodGet, contactPath, "", "content.read", "asset.read", http.StatusOK},
		{"POST", http.MethodPost, contactPath, `{"name":"Trạm Y tế xã","category":"Y tế","phone":"0900000000"}`,
			"content.update", "content.read", http.StatusCreated},
		{"PATCH", http.MethodPatch, contactPath + "/ec-a-2", `{"phone":"0900000000"}`, "content.update", "content.read", http.StatusOK},
		{"DELETE", http.MethodDelete, contactPath + "/ec-a-2", `{"reason":"đã sáp nhập"}`, "content.update", "content.read", http.StatusNoContent},
	}
}

func TestExternalContactRoutesAskForSeededKeys(t *testing.T) {
	// LITERALS: a fake checker grants any string, so a key the `quyen` table lacks would stay green
	// here while answering 403 to every account (rule 5, invariant 3c).
	want := map[string]authz.Perm{"GET": "content.read", "POST": "content.update", "PATCH": "content.update", "DELETE": "content.update"}
	for _, tc := range contactRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			s := newContactServer(t)
			s.call(t, tc.method, hostA, tc.path, canBoGhi(xaA), tc.body)
			if got := s.checker.hoiKhoaCuoi(); got != want[tc.name] {
				t.Fatalf("route asked for %q, want %q", got, want[tc.name])
			}
		})
	}
}

func TestExternalContactRoutes_401NoSession(t *testing.T) {
	for _, tc := range contactRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			s := newContactServer(t)
			s.grant(xaA, tc.perm)
			doiMa(t, s.call(t, tc.method, hostA, tc.path, nil, tc.body), http.StatusUnauthorized)
			if s.fake.calls != 0 {
				t.Error("no session and the store/use case still ran")
			}
		})
	}
}

func TestExternalContactRoutes_403WrongPermission(t *testing.T) {
	for _, tc := range contactRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			s := newContactServer(t)
			s.grant(xaA, tc.wrongPerm)
			doiMa(t, s.call(t, tc.method, hostA, tc.path, canBoGhi(xaA), tc.body), http.StatusForbidden)
			if s.fake.calls != 0 {
				t.Error("wrong permission and the store/use case still ran")
			}
		})
	}
}

func TestExternalContactRoutes_403RightPermissionWrongCommune(t *testing.T) {
	// Signed in at commune B as a member of B, the grant in commune A. A checker that ignored the
	// commune would let A's officer edit the numbers on B's directory.
	for _, tc := range contactRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			s := newContactServer(t)
			s.grant(xaA, tc.perm)
			doiMa(t, s.call(t, tc.method, hostB, tc.path, canBoGhi(xaB), tc.body), http.StatusForbidden)
			if s.fake.calls != 0 {
				t.Error("a grant in another commune was enough")
			}
		})
	}
}

func TestExternalContactRoutes_200RightPermissionRightCommune(t *testing.T) {
	for _, tc := range contactRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			s := newContactServer(t)
			s.grant(xaA, tc.perm)
			doiMa(t, s.call(t, tc.method, hostA, tc.path, canBoGhi(xaA), tc.body), tc.ok)
			if s.fake.lastCommune != xaA {
				t.Errorf("store/use case ran in %q, want the Host commune %q", s.fake.lastCommune, xaA)
			}
			if tc.method != http.MethodGet && s.fake.lastActor.ID != maCanBoGhi {
				// Rule 6, invariant 8: the trail records `CB-…`, never the internal id.
				t.Errorf("actor = %q, want the business code %q", s.fake.lastActor.ID, maCanBoGhi)
			}
		})
	}
}

func TestExternalContactStaffListShape(t *testing.T) {
	s := newContactServer(t)
	s.grant(xaA, "content.read")
	w := s.call(t, http.MethodGet, hostA, contactPath, canBoGhi(xaA), "")
	doiMa(t, w, http.StatusOK)
	var got struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Items) != 2 || got.Items[0]["id"] != "ec-a-1" || got.Items[1]["id"] != "ec-a-2" {
		t.Fatalf("items = %v", got.Items)
	}
	// Absent, not null, when unset; unmasked phone for the officer who edits it.
	if _, ok := got.Items[0]["address"]; ok {
		t.Error("address present on a row with none")
	}
	if _, ok := got.Items[1]["display_order"]; ok {
		t.Error("display_order present on a row with none")
	}
	if got.Items[1]["phone"] != "0900000000" || got.Items[1]["address"] != "Thôn 1" {
		t.Errorf("row 2 = %v", got.Items[1])
	}
}

func TestExternalContactWriteBodiesReachTheUseCase(t *testing.T) {
	s := newContactServer(t)
	s.grant(xaA, "content.update")
	doiMa(t, s.call(t, http.MethodPost, hostA, contactPath, canBoGhi(xaA),
		`{"name":"Điện lực","category":"Điện","phone":"19001006","address":"Thôn 3","display_order":4}`), http.StatusCreated)
	if in := s.fake.lastInput; in.Name != "Điện lực" || in.Address != "Thôn 3" || in.DisplayOrder == nil || *in.DisplayOrder != 4 {
		t.Errorf("create input = %+v", in)
	}
	doiMa(t, s.call(t, http.MethodPatch, hostA, contactPath+"/ec-a-2", canBoGhi(xaA), `{"address":""}`), http.StatusOK)
	if p := s.fake.lastPatch; s.fake.lastID != "ec-a-2" || p.Address == nil || *p.Address != "" || p.Name != nil {
		t.Errorf("patch = %+v (id %q)", p, s.fake.lastID)
	}
	doiMa(t, s.call(t, http.MethodDelete, hostA, contactPath+"/ec-a-2", canBoGhi(xaA), `{"reason":"trùng"}`), http.StatusNoContent)
	if s.fake.lastReason != "trùng" {
		t.Errorf("reason = %q", s.fake.lastReason)
	}
}

func TestExternalContactCreateNeedsIdempotencyKey(t *testing.T) {
	s := newContactServer(t)
	s.grant(xaA, "content.update")
	r := httptest.NewRequest(http.MethodPost, "https://"+hostA+contactPath,
		strings.NewReader(`{"name":"N","category":"C","phone":"113"}`))
	r.Host = hostA
	r = r.WithContext(context.WithValue(r.Context(), khoaChuTheGhi{}, *canBoGhi(xaA)))
	w := httptest.NewRecorder()
	s.h.ServeHTTP(w, r)
	doiMa(t, w, http.StatusBadRequest)
	if s.fake.calls != 0 {
		t.Error("a create without Idempotency-Key reached the use case")
	}
}

func TestExternalContactErrorMapping(t *testing.T) {
	for name, tc := range map[string]struct {
		err  error
		code int
		want string
	}{
		"not found":   {fmt.Errorf("wrapped: %w", commsstore.ErrExternalContactNotFound), http.StatusNotFound, "not_found"},
		"full":        {commsstore.ErrExternalContactsFull, http.StatusConflict, "catalogue_full"},
		"bad phone":   {fmt.Errorf("x: %w", domain.ErrExternalContactPhoneShape), http.StatusBadRequest, "invalid_request"},
		"no reason":   {domain.ErrThieuLyDoXoa, http.StatusBadRequest, "invalid_request"},
		"store fault": {errors.New("pq: connection reset"), http.StatusInternalServerError, "internal"},
	} {
		t.Run(name, func(t *testing.T) {
			s := newContactServer(t)
			s.grant(xaA, "content.update")
			s.fake.err = tc.err
			w := s.call(t, http.MethodPatch, hostA, contactPath+"/ec-a-2", canBoGhi(xaA), `{"name":"X"}`)
			doiMa(t, w, tc.code)
			e := loiTra(t, w)
			if e.Code != tc.want {
				t.Errorf("code = %q, want %q", e.Code, tc.want)
			}
			// The wire sentence is a fixed one: no package prefix, no backtick, no driver text.
			if strings.Contains(e.Message, "external_contact:") || strings.Contains(e.Message, "`") ||
				strings.Contains(e.Message, "pq:") {
				t.Errorf("message leaks the error chain: %q", e.Message)
			}
		})
	}
}

func TestExternalContactWriteWithoutBusinessCodeRefuses(t *testing.T) {
	s := newContactServer(t)
	s.grant(xaA, "content.update")
	p := canBoGhi(xaA)
	p.Ma = "" // an identity older than `ma`: refuse, never fall back to the internal id
	doiMa(t, s.call(t, http.MethodPatch, hostA, contactPath+"/ec-a-2", p, `{"name":"X"}`), http.StatusInternalServerError)
	if s.fake.calls != 0 {
		t.Error("a write ran with no business code to put in the trail")
	}
}

func TestRegisterExternalContactsRefusesIncompleteWiring(t *testing.T) {
	f := contactFixture()
	for name, d := range map[string]ExternalContactDeps{
		"no checker": {Reader: f, Writer: f},
		"no reader":  {Checker: &checkerDanhMucGia{}, Writer: f},
		"no writer":  {Checker: &checkerDanhMucGia{}, Reader: f},
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%s: registered anyway", name)
				}
			}()
			RegisterExternalContacts(http.NewServeMux(), d)
		}()
	}
}

// --- public -----------------------------------------------------------------------------------

func publicContactServer(t *testing.T, nt *ckNenTang, f *fakeExternalContacts, c *memCounter, log *slog.Logger) http.Handler {
	t.Helper()
	if log == nil {
		log = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	if c == nil {
		c = &memCounter{}
	}
	mux := http.NewServeMux()
	RegisterPublicExternalContacts(mux, PublicExternalContactDeps{Xa: nt, Contacts: f, Limiter: ckLimiterOn(c), Log: log})
	return mux
}

func publicContactGet(h http.Handler, host, remote string, extra ...string) *httptest.ResponseRecorder {
	q := ""
	if host != "" {
		q = "?host=" + url.QueryEscape(host)
	}
	q += strings.Join(extra, "")
	r := httptest.NewRequest(http.MethodGet, "https://comms.api.vigov.vn"+PublicExternalContactsPath+q, nil)
	if remote != "" {
		r.RemoteAddr = remote
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

type publicContactPage struct {
	Items []map[string]any `json:"items"`
}

func readPublicContacts(t *testing.T, w *httptest.ResponseRecorder) publicContactPage {
	t.Helper()
	var p publicContactPage
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
		t.Fatalf("body is not JSON: %q", w.Body.String())
	}
	return p
}

func TestPublicExternalContactsPathMatchesRoute(t *testing.T) {
	mux := http.NewServeMux()
	f := contactFixture()
	RegisterPublicExternalContacts(mux, PublicExternalContactDeps{Xa: &ckNenTang{}, Contacts: f, Limiter: ckLimiter()})
	if _, pattern := mux.Handler(httptest.NewRequest(http.MethodGet, PublicExternalContactsPath, nil)); pattern != "GET "+PublicExternalContactsPath {
		t.Fatalf("%s matched %q", PublicExternalContactsPath, pattern)
	}
}

// Commune A's host returns A's live rows only — never B's, never a soft-deleted one — as plain text,
// and asks the store in A.
func TestPublicExternalContactsCommuneIsolationAndDeletedHidden(t *testing.T) {
	f := contactFixture()
	h := publicContactServer(t, &ckNenTang{}, f, nil, nil)

	w := publicContactGet(h, ckHostA, "")
	doiMa(t, w, http.StatusOK)
	if f.lastCommune != xaA {
		t.Fatalf("store asked in %q, want %q", f.lastCommune, xaA)
	}
	got := readPublicContacts(t, w)
	if len(got.Items) != 2 || got.Items[0]["id"] != "ec-a-1" || got.Items[1]["id"] != "ec-a-2" {
		t.Fatalf("commune A = %v", got.Items)
	}
	body := w.Body.String()
	for _, banned := range []string{"ec-a-gone", "Bưu điện cũ", "XÃ B", "ec-b-1", "<b>", string(xaA)} {
		if strings.Contains(body, banned) {
			t.Errorf("public body holds %q: %s", banned, body)
		}
	}
	if got.Items[1]["name"] != "Trạm Y tế xã" || got.Items[1]["phone"] != "0900000000" || got.Items[1]["address"] != "Thôn 1" {
		t.Errorf("row = %v", got.Items[1])
	}
	for _, field := range []string{"id", "name", "category", "phone"} {
		if _, ok := got.Items[0][field]; !ok {
			t.Errorf("field %q missing", field)
		}
	}

	w = publicContactGet(h, ckHostB, "")
	doiMa(t, w, http.StatusOK)
	if got := readPublicContacts(t, w); len(got.Items) != 1 || got.Items[0]["id"] != "ec-b-1" || f.lastCommune != xaB {
		t.Fatalf("commune B = %v (asked in %q)", got.Items, f.lastCommune)
	}
}

// Unknown, reserved and inactive domains: 200 `{"items":[]}`, byte-identical to a commune with nothing,
// and the store is never asked.
func TestPublicExternalContactsUnknownHostIsEmptyAndIdentical(t *testing.T) {
	empty := &fakeExternalContacts{byCommune: map[tenant.ID][]domain.ExternalContact{}}
	reference := publicContactGet(publicContactServer(t, &ckNenTang{}, empty, nil, nil), ckHostA, "")
	doiMa(t, reference, http.StatusOK)

	for _, host := range []string{ckHostKhongCo, ckHostNgung, ckHostRieng} {
		f := contactFixture()
		w := publicContactGet(publicContactServer(t, &ckNenTang{}, f, nil, nil), host, "")
		doiMa(t, w, http.StatusOK)
		if w.Body.String() != reference.Body.String() {
			t.Errorf("%s: %q, want the bytes of an empty commune %q", host, w.Body.String(), reference.Body.String())
		}
		if f.calls != 0 {
			t.Errorf("%s: the store was asked", host)
		}
	}
	if strings.TrimSpace(reference.Body.String()) != `{"items":[]}` {
		t.Errorf("empty body = %q", reference.Body.String())
	}
}

func TestPublicExternalContactsInvalidHost(t *testing.T) {
	for name, extra := range map[string][]string{
		"missing":   nil,
		"malformed": {"?host=not%20a%20host"},
		"repeated":  {"?host=" + ckHostA + "&host=" + ckHostB},
	} {
		t.Run(name, func(t *testing.T) {
			nt := &ckNenTang{}
			c := &memCounter{}
			f := contactFixture()
			w := publicContactGet(publicContactServer(t, nt, f, c, nil), "", "", extra...)
			doiMa(t, w, http.StatusBadRequest)
			if e := loiTra(t, w); e.Code != "invalid_host" {
				t.Errorf("code = %q", e.Code)
			}
			if nt.goi != 0 || len(c.keys) != 0 || f.calls != 0 {
				t.Errorf("a 400 asked the platform (%d), counted (%v) or read (%d)", nt.goi, c.keys, f.calls)
			}
		})
	}
}

func TestPublicExternalContactsPlatformDownIs503(t *testing.T) {
	f := contactFixture()
	w := publicContactGet(publicContactServer(t, &ckNenTang{chet: true}, f, nil, nil), ckHostA, "")
	doiMa(t, w, http.StatusServiceUnavailable)
	if f.calls != 0 {
		t.Error("an outage read the store")
	}
}

func TestPublicExternalContactsStoreFaultsAre500(t *testing.T) {
	for _, err := range []error{errors.New("pq: boom"), commsstore.ErrTooManyExternalContacts} {
		f := contactFixture()
		f.err = err
		w := publicContactGet(publicContactServer(t, &ckNenTang{}, f, nil, nil), ckHostA, "")
		doiMa(t, w, http.StatusInternalServerError)
		if strings.Contains(w.Body.String(), "pq:") {
			t.Error("driver text reached the client")
		}
	}
}

// RATE LIMIT — the EXISTING ratelimit.PublicNewsRead policy, keyed exactly like the news routes, so a
// client network's news and contact reads share one budget per host.
func TestPublicExternalContactsRateLimited(t *testing.T) {
	c := &memCounter{}
	f := contactFixture()
	h := publicContactServer(t, &ckNenTang{}, f, c, nil)
	for i := 0; i < ratelimit.PublicNewsReadLimit; i++ {
		if w := publicContactGet(h, ckHostA, "203.0.113.7:5000"); w.Code != http.StatusOK {
			t.Fatalf("request %d: %d", i+1, w.Code)
		}
	}
	readsBefore := f.calls
	w := publicContactGet(h, ckHostA, "203.0.113.7:5000")
	doiMa(t, w, http.StatusTooManyRequests)
	if w.Header().Get("Retry-After") == "" || loiTra(t, w).Code != "rate_limited" {
		t.Fatalf("429 shape: %v %s", w.Header(), w.Body.String())
	}
	if f.calls != readsBefore {
		t.Error("the store was read past the limit")
	}
	want := "t:" + string(xaA) + ":rl:public-news:host:" + ckHostA + ":ip:203.0.113.7"
	if c.keys[0] != want {
		t.Errorf("limiter key = %q, want %q (rule 1, invariant 7)", c.keys[0], want)
	}
	// An unknown host is limited at the same threshold, keyed by host with no tenant prefix.
	publicContactGet(h, ckHostKhongCo, "203.0.113.9:5000")
	if last := c.keys[len(c.keys)-1]; last != "rl:public-news:host:"+ckHostKhongCo+":ip:203.0.113.9" {
		t.Errorf("unknown-host key = %q", last)
	}
}

// One budget, not two: news reads and contact reads from one network to one host count together.
func TestPublicExternalContactsShareTheNewsBudget(t *testing.T) {
	c := &memCounter{}
	limiter := ckLimiterOn(c)
	nd, dm := ckDuLieu()
	mux := http.NewServeMux()
	RegisterCongKhai(mux, DepsCongKhai{Limiter: limiter, Xa: &ckNenTang{}, NoiDung: nd, Views: nd, DanhMuc: dm,
		CoverImages: &fakePublicCovers{}, Audio: &fakePublicAudio{}, Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
	RegisterPublicExternalContacts(mux, PublicExternalContactDeps{Xa: &ckNenTang{}, Contacts: contactFixture(), Limiter: limiter})

	for i := 0; i < ratelimit.PublicNewsReadLimit; i++ {
		if w := rlGet(mux, MauTinXa, ckHostA, "203.0.113.7:5000"); w.Code != http.StatusOK {
			t.Fatalf("news %d: %d", i+1, w.Code)
		}
	}
	doiMa(t, publicContactGet(mux, ckHostA, "203.0.113.7:5000"), http.StatusTooManyRequests)
}

func TestPublicExternalContactsServesWhenCounterIsDown(t *testing.T) {
	var buf bytes.Buffer
	c := &memCounter{fail: errors.New("dial tcp 10.0.0.9:6379: connection refused")}
	w := publicContactGet(publicContactServer(t, &ckNenTang{}, contactFixture(), c, slog.New(slog.NewJSONHandler(&buf, nil))),
		ckHostA, "203.0.113.7:5000")
	doiMa(t, w, http.StatusOK)
	if strings.Contains(buf.String(), "203.0.113.7") || strings.Contains(buf.String(), "0900000000") {
		t.Fatalf("log holds an address or a phone: %s", buf.String())
	}
}

func TestRegisterPublicExternalContactsRefusesIncompleteWiring(t *testing.T) {
	f := contactFixture()
	for name, d := range map[string]PublicExternalContactDeps{
		"no platform": {Contacts: f, Limiter: ckLimiter()},
		"no store":    {Xa: &ckNenTang{}, Limiter: ckLimiter()},
		// Rule 13 invariant 7: no public route without its rate limit.
		"no limiter": {Xa: &ckNenTang{}, Contacts: f},
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%s: registered anyway", name)
				}
			}()
			RegisterPublicExternalContacts(http.NewServeMux(), d)
		}()
	}
}
