package http

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/crypto"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/app"
	"github.com/vihat/vigov/service-comms/internal/domain"
	"github.com/vihat/vigov/service-comms/internal/mail"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

// The three mail-settings routes. Harness pieces are the catalogue write suite's (checkerDanhMucGia,
// chuTheGhi, canBoGhi) because "right permission, wrong commune" needs a checker keyed by commune.

// testPassword is a fixture, not a credential: no server anywhere accepts it.
const testPassword = "fixture-not-a-real-password-7Q"

// fakeMailSettings stands in for app.MailSettingsAdmin, RECORDING THE COMMUNE it was called in and
// the actor and request it received.
type fakeMailSettings struct {
	view app.MailSettingsView
	err  error

	calls         int
	lastCommune   tenant.ID
	lastActor     audit.Actor
	lastSave      app.SaveMailSettingsRequest
	lastPassword  string // copied before the handler clears the secret
	lastRecipient string
}

func (f *fakeMailSettings) note(ctx context.Context) {
	f.calls++
	f.lastCommune = tenant.MustFrom(ctx)
}

func (f *fakeMailSettings) Get(ctx context.Context) (app.MailSettingsView, error) {
	f.note(ctx)
	return f.view, f.err
}

func (f *fakeMailSettings) Save(ctx context.Context, req app.SaveMailSettingsRequest,
	actor audit.Actor) (app.MailSettingsView, error) {
	f.note(ctx)
	f.lastSave, f.lastActor = req, actor
	f.lastPassword = string(req.Password.Lo())
	return f.view, f.err
}

func (f *fakeMailSettings) SendTestMessage(ctx context.Context, recipient string, actor audit.Actor) error {
	f.note(ctx)
	f.lastRecipient, f.lastActor = recipient, actor
	return f.err
}

type mailServer struct {
	h       http.Handler
	fake    *fakeMailSettings
	checker *checkerDanhMucGia
}

func newMailServer(t *testing.T) *mailServer {
	t.Helper()
	fake := &fakeMailSettings{view: app.MailSettingsView{
		MailSettings: domain.MailSettings{
			Host: "smtp.example.test", Port: 587, Security: domain.MailSecurityStartTLS,
			Username: "ubnd@example.test", FromAddress: "ubnd@example.test", FromName: "UBND xã",
			IsEnabled: true, PasswordSet: true,
		},
		Configured: true, EncryptionConfigured: true,
	}}
	checker := &checkerDanhMucGia{}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	mux := http.NewServeMux()
	Register(mux, Deps{
		Checker:              checker,
		LoaiTaiNguyen:        danhMucMau(),
		GhiLoaiTaiNguyen:     &ghiDanhMucGia{},
		ThongBao:             &soThongBaoGia{},
		GhiThongBao:          &ghiThongBaoGia{},
		NoiDung:              &soNoiDungGia{},
		GhiNoiDung:           &ghiNoiDungGia{},
		DanhMucNoiDung:       &soDanhMucNDGia{},
		GhiDanhMucNoiDung:    &ghiDanhMucNDGia{},
		MapFieldSchemas:      &fakeMapFieldSchemas{},
		WriteMapFieldSchemas: &fakeMapFieldSchemas{},
		MailSettings:         fake,
		WriteMailSettings:    fake,
		AuditLog:             &auditLogFake{},
		Log:                  log,
	})

	var h http.Handler = mux
	h = chuTheGhi(h)
	h = idem.Middleware(nil, log)(h)
	h = httpx.TenantMiddleware(thuMucMau())(h)
	h = httpx.Recover(func(context.Context) string { return "test-trace" })(h)
	h = httpx.StripTenantHeaders(h)
	return &mailServer{h: h, fake: fake, checker: checker}
}

func (s *mailServer) grant(commune tenant.ID, perms ...authz.Perm) {
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

func (s *mailServer) call(t *testing.T, method, host, path string, p *authz.Principal, body string) *httptest.ResponseRecorder {
	t.Helper()
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	r := httptest.NewRequest(method, "https://"+host+path, rd)
	r.Host = host
	r.RemoteAddr = "10.0.0.7:51000"
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set(idem.Header, "01JIDEMPOTENCYKEYMAILSET")
	if p != nil {
		r = r.WithContext(context.WithValue(r.Context(), khoaChuTheGhi{}, *p))
	}
	w := httptest.NewRecorder()
	s.h.ServeHTTP(w, r)
	return w
}

const (
	mailPath     = "/api/v1/mail-settings"
	mailTestPath = "/api/v1/mail-settings/test-messages"
)

var putMailBody = `{"host":"smtp.example.test","port":587,"security":"starttls","username":"ubnd@example.test",` +
	`"from_address":"ubnd@example.test","from_name":"UBND xã","is_enabled":true,"password":"` + testPassword + `"}`

type mailRoute struct {
	name, method, path, body string
	wrongPerm                authz.Perm
	ok                       int
}

// threeRoutes — every permission case is asserted on all three. The wrong key is real and adjacent:
// the audit reader of the same tab (`admin.audit`) must not be able to read or change the server.
func threeRoutes() []mailRoute {
	return []mailRoute{
		{"GET", http.MethodGet, mailPath, "", "admin.audit", http.StatusOK},
		{"PUT", http.MethodPut, mailPath, putMailBody, "admin.audit", http.StatusOK},
		{"POST test", http.MethodPost, mailTestPath, `{"recipient":"canbo@example.test"}`, "admin.user", http.StatusOK},
	}
}

func TestMailRoutesAskForSeededKey(t *testing.T) {
	// Compared with a LITERAL: a fake checker grants any string (rule 5, invariant 3c).
	for _, tc := range threeRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			s := newMailServer(t)
			s.call(t, tc.method, hostA, tc.path, canBoGhi(xaA), tc.body)
			if got := s.checker.hoiKhoaCuoi(); got != "admin.lookup" {
				t.Fatalf("route asked for %q, want admin.lookup", got)
			}
		})
	}
}

func TestMailRoutes_401NoSession(t *testing.T) {
	for _, tc := range threeRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			s := newMailServer(t)
			s.grant(xaA, "admin.lookup")
			doiMa(t, s.call(t, tc.method, hostA, tc.path, nil, tc.body), http.StatusUnauthorized)
			if s.fake.calls != 0 {
				t.Error("no session and the use case still ran")
			}
		})
	}
}

func TestMailRoutes_403WrongPermission(t *testing.T) {
	for _, tc := range threeRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			s := newMailServer(t)
			s.grant(xaA, tc.wrongPerm)
			doiMa(t, s.call(t, tc.method, hostA, tc.path, canBoGhi(xaA), tc.body), http.StatusForbidden)
			if s.fake.calls != 0 {
				t.Error("wrong permission and the use case still ran")
			}
		})
	}
}

func TestMailRoutes_403RightPermissionWrongCommune(t *testing.T) {
	// Signed in at B as a member of B; the grant is in A. A checker ignoring the commune would let
	// A's administrator read B's mail server account — or point it somewhere.
	for _, tc := range threeRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			s := newMailServer(t)
			s.grant(xaA, "admin.lookup")
			doiMa(t, s.call(t, tc.method, hostB, tc.path, canBoGhi(xaB), tc.body), http.StatusForbidden)
			if s.fake.calls != 0 {
				t.Error("a grant in another commune was enough")
			}
		})
	}
}

func TestMailRoutes_401SessionOfAnotherCommune(t *testing.T) {
	for _, tc := range threeRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			s := newMailServer(t)
			s.grant(xaA, "admin.lookup")
			s.grant(xaB, "admin.lookup")
			doiMa(t, s.call(t, tc.method, hostB, tc.path, canBoGhi(xaA), tc.body), http.StatusUnauthorized)
			if s.fake.calls != 0 {
				t.Error("a session of another commune still reached the use case")
			}
		})
	}
}

func TestMailRoutes_RightPermissionRightCommune(t *testing.T) {
	for _, tc := range threeRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			s := newMailServer(t)
			s.grant(xaA, "admin.lookup")
			doiMa(t, s.call(t, tc.method, hostA, tc.path, canBoGhi(xaA), tc.body), tc.ok)
			if s.fake.calls != 1 || s.fake.lastCommune != xaA {
				t.Fatalf("use case ran %d times in commune %q, want once in %q", s.fake.calls, s.fake.lastCommune, xaA)
			}
			if tc.method == http.MethodGet {
				return
			}
			// Rule 6, invariant 8: the BUSINESS CODE, never the internal id.
			if s.fake.lastActor.ID != maCanBoGhi || s.fake.lastActor.IP != "10.0.0.7" {
				t.Errorf("actor = %+v, want business code %q from the socket address", s.fake.lastActor, maCanBoGhi)
			}
		})
	}
}

func TestGetMailSettingsNeverCarriesThePassword(t *testing.T) {
	s := newMailServer(t)
	s.grant(xaA, "admin.lookup")
	w := s.call(t, http.MethodGet, hostA, mailPath, canBoGhi(xaA), "")
	doiMa(t, w, http.StatusOK)
	body := w.Body.String()
	// The field that exists is the boolean; no key named `password` alone may appear at all.
	if !strings.Contains(body, `"password_set":true`) {
		t.Errorf("password_set missing: %s", body)
	}
	if strings.Contains(body, `"password":`) || strings.Contains(body, testPassword) {
		t.Fatalf("GET carries a password field: %s", body)
	}
}

func TestPutMailSettingsPassesPasswordAsSecretAndDoesNotEchoIt(t *testing.T) {
	s := newMailServer(t)
	s.grant(xaA, "admin.lookup")
	w := s.call(t, http.MethodPut, hostA, mailPath, canBoGhi(xaA), putMailBody)
	doiMa(t, w, http.StatusOK)
	if s.fake.lastPassword != testPassword {
		t.Errorf("use case did not receive the typed password")
	}
	if s.fake.lastSave.Input.Host != "smtp.example.test" || s.fake.lastSave.Input.Port != 587 {
		t.Errorf("input = %+v", s.fake.lastSave.Input)
	}
	// secret.Secret refuses to render: formatting the request cannot print the value.
	if strings.Contains(fmt.Sprintf("%v %+v", s.fake.lastSave, s.fake.lastSave), testPassword) {
		t.Error("the request formats with the password in it")
	}
	if strings.Contains(w.Body.String(), testPassword) {
		t.Error("PUT echoes the password")
	}
}

func TestPutMailSettingsBlankPasswordReachesUseCaseAsEmpty(t *testing.T) {
	s := newMailServer(t)
	s.grant(xaA, "admin.lookup")
	body := strings.Replace(putMailBody, `,"password":"`+testPassword+`"`, "", 1)
	doiMa(t, s.call(t, http.MethodPut, hostA, mailPath, canBoGhi(xaA), body), http.StatusOK)
	if len(s.fake.lastSave.Password) != 0 {
		t.Error("an omitted password did not arrive as empty (keep)")
	}
}

func TestMailErrorsMapToStatuses(t *testing.T) {
	for name, tc := range map[string]struct {
		method, path, body string
		err                error
		code               int
		key                string
	}{
		"no KEK on save":      {http.MethodPut, mailPath, putMailBody, crypto.ErrNotConfigured, http.StatusServiceUnavailable, "encryption_not_configured"},
		"no KEK on send":      {http.MethodPost, mailTestPath, `{"recipient":"a@example.test"}`, crypto.ErrNotConfigured, http.StatusServiceUnavailable, "encryption_not_configured"},
		"new host no pwd":     {http.MethodPut, mailPath, putMailBody, app.ErrMailPasswordRequiredForNewDestination, http.StatusBadRequest, "password_required_for_new_host"},
		"first save no pwd":   {http.MethodPut, mailPath, putMailBody, app.ErrMailPasswordRequired, http.StatusBadRequest, "password_required"},
		"bad port":            {http.MethodPut, mailPath, putMailBody, domain.ErrMailPortNotAllowed, http.StatusBadRequest, "invalid_request"},
		"plaintext mode":      {http.MethodPut, mailPath, putMailBody, domain.ErrMailSecurityUnknown, http.StatusBadRequest, "invalid_request"},
		"nothing saved":       {http.MethodPost, mailTestPath, `{"recipient":"a@example.test"}`, commsstore.ErrMailSettingsNotFound, http.StatusConflict, "mail_settings_missing"},
		"bad recipient":       {http.MethodPost, mailTestPath, `{"recipient":"x"}`, domain.ErrMailRecipient, http.StatusBadRequest, "invalid_request"},
		"auth rejected":       {http.MethodPost, mailTestPath, `{"recipient":"a@example.test"}`, fmt.Errorf("%w (smtp 535)", mail.ErrAuthRejected), http.StatusBadGateway, "mail_auth_rejected"},
		"certificate":         {http.MethodPost, mailTestPath, `{"recipient":"a@example.test"}`, mail.ErrCertificate, http.StatusBadGateway, "mail_certificate_invalid"},
		"no starttls":         {http.MethodPost, mailTestPath, `{"recipient":"a@example.test"}`, mail.ErrStartTLSMissing, http.StatusBadGateway, "mail_starttls_missing"},
		"timeout":             {http.MethodPost, mailTestPath, `{"recipient":"a@example.test"}`, mail.ErrTimeout, http.StatusBadGateway, "mail_timeout"},
		"connect":             {http.MethodPost, mailTestPath, `{"recipient":"a@example.test"}`, mail.ErrConnect, http.StatusBadGateway, "mail_connect_failed"},
		"tampered ciphertext": {http.MethodPost, mailTestPath, `{"recipient":"a@example.test"}`, crypto.ErrOpenFailed, http.StatusInternalServerError, "internal"},
		"store broken":        {http.MethodGet, mailPath, "", errors.New("cơ sở dữ liệu không phản hồi"), http.StatusInternalServerError, "internal"},
	} {
		t.Run(name, func(t *testing.T) {
			s := newMailServer(t)
			s.grant(xaA, "admin.lookup")
			s.fake.err = tc.err
			w := s.call(t, tc.method, hostA, tc.path, canBoGhi(xaA), tc.body)
			doiMa(t, w, tc.code)
			e := loiTra(t, w)
			if e.Code != tc.key {
				t.Errorf("code = %q, want %q", e.Code, tc.key)
			}
			// Never the server's words, never the wrapped chain.
			if strings.Contains(e.Message, "smtp 535") || strings.Contains(e.Message, "không phản hồi") {
				t.Errorf("message leaks the underlying error: %q", e.Message)
			}
		})
	}
}
