package http

import (
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
	"github.com/vihat/vigov/core/crypto"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/app"
	"github.com/vihat/vigov/service-comms/internal/domain"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

// The seven SMS-account routes on the staff harness (checkerDanhMucGia, chuTheGhi, canBoGhi). The fake
// records WHICH commune and WHICH actor reached it; the transactions, the sealing and the entries are
// internal/app's tests.
//
//	admin.sms: 401 no session · 403 wrong key · 403 right key other commune · 200

// Fixtures, not credentials.
const (
	testSmsAPIKey    = "FAKE-esms-api-key-NOT-REAL"
	testSmsSecretKey = "FAKE-esms-secret-key-NOT-REAL"
	testSmsID        = "01JSMSROW00000000000000001"
)

type fakeSmsAccounts struct {
	calls       int
	lastCommune tenant.ID
	lastActor   audit.Actor
	lastID      string
	lastCreate  app.CreateSmsAccountInput
	lastUpdate  app.UpdateSmsAccountInput
	keysSeen    string
	lastReason  string
	err         error
}

func (f *fakeSmsAccounts) note(ctx context.Context, a audit.Actor, id string) {
	f.calls++
	f.lastCommune, f.lastActor, f.lastID = tenant.MustFrom(ctx), a, id
}

func sampleSmsAccount() domain.SmsAccount {
	at := time.Date(2026, 10, 17, 2, 0, 0, 0, time.UTC)
	return domain.SmsAccount{ID: testSmsID, Provider: "esms", Label: "eSMS chính", Brandname: "XaMau", IsDefault: true,
		LastCheckAt: at, LastCheckResult: domain.SmsCheckOK, LastBalance: "36693.00", SetAt: at, SetBy: "CB-00099", CreatedAt: at}
}

func (f *fakeSmsAccounts) ListAccounts(ctx context.Context) ([]domain.SmsAccount, error) {
	f.note(ctx, audit.Actor{}, "")
	return []domain.SmsAccount{sampleSmsAccount()}, f.err
}

func (f *fakeSmsAccounts) CreateAccount(ctx context.Context, in app.CreateSmsAccountInput, a audit.Actor) (domain.SmsAccount, error) {
	f.note(ctx, a, "")
	f.lastCreate = in
	f.keysSeen = string(in.APIKey.Lo()) + "|" + string(in.SecretKey.Lo())
	return sampleSmsAccount(), f.err
}

func (f *fakeSmsAccounts) UpdateAccount(ctx context.Context, id string, in app.UpdateSmsAccountInput, a audit.Actor) (domain.SmsAccount, error) {
	f.note(ctx, a, id)
	f.lastUpdate = in
	f.keysSeen = string(in.APIKey.Lo()) + "|" + string(in.SecretKey.Lo())
	return sampleSmsAccount(), f.err
}

func (f *fakeSmsAccounts) RetireAccount(ctx context.Context, id, reason string, a audit.Actor) error {
	f.note(ctx, a, id)
	f.lastReason = reason
	return f.err
}

func (f *fakeSmsAccounts) Check(ctx context.Context, id string, a audit.Actor) (app.SmsCheckOutput, error) {
	f.note(ctx, a, id)
	return app.SmsCheckOutput{Result: domain.SmsCheckKeyRejected, CheckedAt: time.Date(2026, 10, 17, 3, 0, 0, 0, time.UTC)}, f.err
}

func (f *fakeSmsAccounts) ListTemplates(ctx context.Context, id string) ([]app.SmsTemplateGroup, error) {
	f.note(ctx, audit.Actor{}, id)
	return []app.SmsTemplateGroup{{Content: "{P1,20} la ma", Params: []domain.SmsTemplateParam{{Name: "P1", MaxLen: 20}},
		Networks: []app.SmsTemplateNetwork{{NetworkID: 6, ProviderTemplateID: "458"}, {NetworkID: 1, ProviderTemplateID: "12884"}}}}, f.err
}

func (f *fakeSmsAccounts) SyncTemplates(ctx context.Context, id string, a audit.Actor) (app.SmsTemplateSyncOutput, error) {
	f.note(ctx, a, id)
	return app.SmsTemplateSyncOutput{Added: 1, Updated: 2, Vanished: 3, Skipped: 4}, f.err
}

type smsServer struct {
	h       http.Handler
	fake    *fakeSmsAccounts
	checker *checkerDanhMucGia
	keys    int
}

func newSmsServer(t *testing.T) *smsServer {
	t.Helper()
	fake := &fakeSmsAccounts{}
	checker := &checkerDanhMucGia{}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	mux := http.NewServeMux()
	RegisterSmsAccounts(mux, SmsAccountDeps{Checker: checker, Accounts: fake, Log: log})
	var h http.Handler = mux
	h = chuTheGhi(h)
	h = idem.Middleware(moKhoIdemGia(), log)(h)
	h = httpx.TenantMiddleware(thuMucMau())(h)
	h = httpx.Recover(func(context.Context) string { return "test-trace" })(h)
	h = httpx.StripTenantHeaders(h)
	return &smsServer{h: h, fake: fake, checker: checker}
}

func (s *smsServer) grant(commune tenant.ID, perms ...authz.Perm) {
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

func (s *smsServer) call(t *testing.T, method, path, host string, p *authz.Principal, body string) *httptest.ResponseRecorder {
	t.Helper()
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	r := httptest.NewRequest(method, "https://"+host+path, rd)
	r.Host = host
	r.RemoteAddr = "10.0.0.7:51000"
	r.Header.Set("Content-Type", "application/json")
	s.keys++
	r.Header.Set(idem.Header, fmt.Sprintf("01JIDEMSMSACCOUNT%09d", s.keys))
	if p != nil {
		r = r.WithContext(context.WithValue(r.Context(), khoaChuTheGhi{}, *p))
	}
	w := httptest.NewRecorder()
	s.h.ServeHTTP(w, r)
	return w
}

const (
	smsPath       = "/api/v1/sms-accounts"
	smsOnePath    = smsPath + "/" + testSmsID
	smsCreateBody = `{"label":"eSMS chính","api_key":"` + testSmsAPIKey + `","secret_key":"` + testSmsSecretKey +
		`","brandname":"XaMau","is_default":true}`
	smsPatchBody  = `{"label":"eSMS dự phòng","secret_key":"` + testSmsSecretKey + `","is_default":false}`
	smsRetireBody = `{"reason":"Hết hợp đồng"}`
)

// smsRoutes — the wrong keys are real and adjacent: admin.zalo is the sibling tab's key, and content.read
// a reader on the same screens.
func smsRoutes() []mapAssetRoute {
	return []mapAssetRoute{
		{"list", http.MethodGet, smsPath, "", "admin.sms", "admin.zalo", http.StatusOK},
		{"create", http.MethodPost, smsPath, smsCreateBody, "admin.sms", "admin.zalo", http.StatusCreated},
		{"update", http.MethodPatch, smsOnePath, smsPatchBody, "admin.sms", "content.read", http.StatusOK},
		{"retire", http.MethodDelete, smsOnePath, smsRetireBody, "admin.sms", "admin.zalo", http.StatusOK},
		{"check", http.MethodPost, smsOnePath + "/check", "", "admin.sms", "admin.zalo", http.StatusOK},
		{"templates", http.MethodGet, smsOnePath + "/templates", "", "admin.sms", "notice.send", http.StatusOK},
		{"sync", http.MethodPost, smsOnePath + "/templates/sync", "", "admin.sms", "admin.zalo", http.StatusOK},
	}
}

func TestSmsRoutesAskForSeededKey(t *testing.T) {
	for _, tc := range smsRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			s := newSmsServer(t)
			s.call(t, tc.method, tc.path, hostA, canBoGhi(xaA), tc.body)
			// Compared with a LITERAL: a fake checker grants any string (rule 5, invariant 3c).
			if len(s.checker.hoiGi) == 0 || s.checker.hoiGi[0] != "admin.sms" {
				t.Fatalf("asked for %v, want admin.sms (seeded at service-identity/migrations/0035_sms_channel_permission.sql)", s.checker.hoiGi)
			}
		})
	}
}

func TestSmsRoutes_401NoSession(t *testing.T) {
	for _, tc := range smsRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			s := newSmsServer(t)
			s.grant(xaA, tc.perm)
			doiMa(t, s.call(t, tc.method, tc.path, hostA, nil, tc.body), http.StatusUnauthorized)
			if s.fake.calls != 0 {
				t.Error("no session and the use case ran")
			}
		})
	}
}

func TestSmsRoutes_403WrongPermission(t *testing.T) {
	for _, tc := range smsRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			s := newSmsServer(t)
			s.grant(xaA, tc.wrongPerm)
			doiMa(t, s.call(t, tc.method, tc.path, hostA, canBoGhi(xaA), tc.body), http.StatusForbidden)
			if s.fake.calls != 0 {
				t.Error("wrong permission and the use case ran")
			}
		})
	}
}

func TestSmsRoutes_403RightPermissionWrongCommune(t *testing.T) {
	// Signed in at B as a member of B; the grant is in A. A checker ignoring the commune would let A's
	// administrator read and replace B's eSMS keys — and spend B's money.
	for _, tc := range smsRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			s := newSmsServer(t)
			s.grant(xaA, tc.perm)
			doiMa(t, s.call(t, tc.method, tc.path, hostB, canBoGhi(xaB), tc.body), http.StatusForbidden)
			if s.fake.calls != 0 {
				t.Error("a grant in another commune was enough")
			}
		})
	}
}

func TestSmsRoutes_200RightPermissionRightCommune(t *testing.T) {
	for _, tc := range smsRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			s := newSmsServer(t)
			s.grant(xaA, tc.perm)
			doiMa(t, s.call(t, tc.method, tc.path, hostA, canBoGhi(xaA), tc.body), tc.ok)
			if s.fake.calls != 1 || s.fake.lastCommune != xaA {
				t.Fatalf("use case ran %d times in %q, want once in %q", s.fake.calls, s.fake.lastCommune, xaA)
			}
			if tc.method != http.MethodGet && (s.fake.lastActor.ID != maCanBoGhi || s.fake.lastActor.IP != "10.0.0.7") {
				// Rule 6, invariant 8: the BUSINESS CODE, never the internal id.
				t.Errorf("actor = %+v, want business code %q", s.fake.lastActor, maCanBoGhi)
			}
			if strings.Contains(tc.path, testSmsID) && s.fake.lastID != testSmsID {
				t.Errorf("id = %q", s.fake.lastID)
			}
		})
	}
}

// --- shapes --------------------------------------------------------------------------------------------

func TestSmsListCarriesNoKeyAndTheContractFields(t *testing.T) {
	s := newSmsServer(t)
	s.grant(xaA, "admin.sms")
	w := s.call(t, http.MethodGet, smsPath, hostA, canBoGhi(xaA), "")
	doiMa(t, w, http.StatusOK)
	body := w.Body.String()
	for _, bad := range []string{"api_key\"", "secret_key", "sealed", "hint"} {
		if strings.Contains(body, bad) {
			t.Fatalf("GET carries %q: %s", bad, body)
		}
	}
	var out struct {
		Items []map[string]json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil || len(out.Items) != 1 {
		t.Fatalf("%s %v", body, err)
	}
	for _, k := range []string{"id", "provider", "label", "brandname", "is_default", "set_at", "set_by", "last_check",
		"created_at"} {
		if _, ok := out.Items[0][k]; !ok {
			t.Errorf("item lacks %q: %s", k, body)
		}
	}
	if !strings.Contains(body, `"balance":"36693.00"`) || !strings.Contains(body, `"result":"thanh-cong"`) {
		t.Errorf("last_check = %s", body)
	}
}

func TestSmsCreatePassesKeysAsSecretsAndNeverEchoesThem(t *testing.T) {
	s := newSmsServer(t)
	s.grant(xaA, "admin.sms")
	w := s.call(t, http.MethodPost, smsPath, hostA, canBoGhi(xaA), smsCreateBody)
	doiMa(t, w, http.StatusCreated)
	if s.fake.keysSeen != testSmsAPIKey+"|"+testSmsSecretKey || s.fake.lastCreate.Label != "eSMS chính" ||
		s.fake.lastCreate.Brandname != "XaMau" || !s.fake.lastCreate.IsDefault {
		t.Errorf("use case received %+v", s.fake.lastCreate)
	}
	if strings.Contains(fmt.Sprintf("%v %+v", s.fake.lastCreate, s.fake.lastCreate), testSmsAPIKey) {
		t.Error("the input formats with a key in it")
	}
	if strings.Contains(w.Body.String(), testSmsAPIKey) || strings.Contains(w.Body.String(), testSmsSecretKey) {
		t.Fatal("POST echoes a key")
	}
}

func TestSmsPatchKeepsWhatIsAbsent(t *testing.T) {
	s := newSmsServer(t)
	s.grant(xaA, "admin.sms")
	doiMa(t, s.call(t, http.MethodPatch, smsOnePath, hostA, canBoGhi(xaA), smsPatchBody), http.StatusOK)
	u := s.fake.lastUpdate
	// keysSeen is captured DURING the call: the handler clears the key buffers once the use case returns.
	if u.Label == nil || *u.Label != "eSMS dự phòng" || u.Brandname != nil || !u.APIKey.Rong() ||
		s.fake.keysSeen != "|"+testSmsSecretKey || u.IsDefault == nil || *u.IsDefault {
		t.Fatalf("update = %+v, keys %q", u, s.fake.keysSeen)
	}
	for _, b := range u.SecretKey.Lo() {
		if b != 0 {
			t.Fatal("the SecretKey buffer was not cleared after the call")
		}
	}
}

func TestSmsRetireCarriesTheReason(t *testing.T) {
	s := newSmsServer(t)
	s.grant(xaA, "admin.sms")
	w := s.call(t, http.MethodDelete, smsOnePath, hostA, canBoGhi(xaA), smsRetireBody)
	doiMa(t, w, http.StatusOK)
	if s.fake.lastReason != "Hết hợp đồng" || !strings.Contains(w.Body.String(), `"retired":true`) {
		t.Fatalf("reason %q, reply %s", s.fake.lastReason, w.Body.String())
	}
}

func TestSmsCheckAFailingResultIs200WithNullBalance(t *testing.T) {
	s := newSmsServer(t)
	s.grant(xaA, "admin.sms")
	w := s.call(t, http.MethodPost, smsOnePath+"/check", hostA, canBoGhi(xaA), "")
	doiMa(t, w, http.StatusOK)
	if !strings.Contains(w.Body.String(), `"result":"sai-khoa"`) || !strings.Contains(w.Body.String(), `"balance":null`) {
		t.Fatalf("reply = %s", w.Body.String())
	}
}

func TestSmsTemplatesShape(t *testing.T) {
	s := newSmsServer(t)
	s.grant(xaA, "admin.sms")
	w := s.call(t, http.MethodGet, smsOnePath+"/templates", hostA, canBoGhi(xaA), "")
	doiMa(t, w, http.StatusOK)
	for _, want := range []string{`"content":"{P1,20} la ma"`, `"params":[{"name":"P1","max_length":20}]`,
		`{"network_id":6,"template_id":"458"}`, `"synced_at"`} {
		if !strings.Contains(w.Body.String(), want) {
			t.Errorf("reply lacks %s: %s", want, w.Body.String())
		}
	}
	w = s.call(t, http.MethodPost, smsOnePath+"/templates/sync", hostA, canBoGhi(xaA), "")
	if !strings.Contains(w.Body.String(), `"added":1,"updated":2,"vanished":3,"skipped":4`) {
		t.Errorf("sync reply = %s", w.Body.String())
	}
}

func TestSmsErrorMapping(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"no KEK", crypto.ErrNotConfigured, http.StatusServiceUnavailable, "encryption_not_configured"},
		{"keys", &app.SmsProviderError{Class: domain.SmsCheckKeyRejected}, http.StatusUnprocessableEntity, "sms_key_rejected"},
		{"brandname", &app.SmsProviderError{Class: domain.SmsCheckBrandnameInactive}, http.StatusUnprocessableEntity, "sms_brandname_inactive"},
		{"refused", &app.SmsProviderError{Class: domain.SmsCheckRejected}, http.StatusUnprocessableEntity, "sms_rejected"},
		{"down", &app.SmsProviderError{Class: domain.SmsCheckUnavailable}, http.StatusBadGateway, "sms_unavailable"},
		{"garbled", &app.SmsProviderError{Class: domain.SmsCheckMalformed}, http.StatusBadGateway, "sms_unavailable"},
		{"not found", fmt.Errorf("x: %w", app.ErrSmsAccountNotFound), http.StatusNotFound, "not_found"},
		{"label", fmt.Errorf("save: %w", commsstore.ErrSmsLabelTaken), http.StatusConflict, "sms_label_taken"},
		{"changed", fmt.Errorf("save: %w", commsstore.ErrSmsAccountChanged), http.StatusConflict, "sms_account_changed"},
		{"bad brandname", fmt.Errorf("x: %w", domain.ErrSmsBrandname), http.StatusBadRequest, "sms_account_invalid"},
		{"keys missing", domain.ErrSmsKeysRequired, http.StatusBadRequest, "sms_account_invalid"},
		{"broken", errors.New("boom: " + string(xaB) + " " + testSmsAPIKey), http.StatusInternalServerError, "internal"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := newSmsServer(t)
			s.grant(xaA, "admin.sms")
			s.fake.err = c.err
			w := s.call(t, http.MethodPatch, smsOnePath, hostA, canBoGhi(xaA), smsPatchBody)
			doiMa(t, w, c.status)
			e := loiTra(t, w)
			if e.Code != c.code {
				t.Errorf("code = %q, want %q", e.Code, c.code)
			}
			for _, bad := range []string{string(xaB), "boom", testSmsAPIKey, testSmsSecretKey, "sms_account:"} {
				if strings.Contains(e.Message, bad) {
					t.Errorf("message leaks %q: %q", bad, e.Message)
				}
			}
		})
	}
}

func TestRegisterSmsAccountsRefusesIncompleteWiring(t *testing.T) {
	for name, d := range map[string]SmsAccountDeps{
		"checker":  {Accounts: &fakeSmsAccounts{}},
		"accounts": {Checker: &checkerDanhMucGia{}},
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("incomplete wiring accepted")
				}
			}()
			RegisterSmsAccounts(http.NewServeMux(), d)
		})
	}
}
