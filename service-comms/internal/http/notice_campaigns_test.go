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
	"github.com/vihat/vigov/core/identityclient"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/app"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

// The five campaign routes on the staff harness (checkerDanhMucGia, chuTheGhi, canBoGhi). The fake records
// WHICH commune and WHICH actor reached it; the transaction, identity and the sources are internal/app's
// tests.
//
//	notice.send: 401 no session · 403 wrong key · 403 right key other commune · 200

const testCampaignID = "01JCAMPAIGN000000000000001"

var _ NoticeCampaignUseCase = (*app.NoticeCampaigns)(nil)

type fakeNoticeCampaigns struct {
	calls       int
	lastCommune tenant.ID
	lastActor   audit.Actor
	lastInput   app.NoticeInput
	lastCursor  string
	lastID      string
	err         error
}

func (f *fakeNoticeCampaigns) note(ctx context.Context) {
	f.calls++
	f.lastCommune = tenant.MustFrom(ctx)
}

func sampleNoticeItem() app.NoticeCampaignItem {
	at := time.Date(2026, 10, 17, 3, 0, 0, 0, time.UTC)
	return app.NoticeCampaignItem{SourceName: "eSMS chính · XaMau", Skipped: 2, Campaign: domain.NoticeCampaign{
		ID: testCampaignID, Title: "Họp dân", Channel: domain.NoticeChannelSMS, SmsAccountID: "01JSMSROW00000000000000001",
		TemplateRef: "TPL1", TemplateName: "Moi ba con", Audience: domain.NoticeAudience{Kind: domain.NoticeAudienceCitizenAll},
		RecipientCount: 10, SkippedCount: 2, UnitPrice: "350.00", EstimatedCost: "3500.00", Status: domain.NoticeCampaignSending,
		CreatedAt: at, CreatedBy: maCanBoGhi}}
}

func (f *fakeNoticeCampaigns) Templates(ctx context.Context) (app.NoticeTemplates, error) {
	f.note(ctx)
	return app.NoticeTemplates{
		ZNS: []app.ZnsNoticeTemplate{{OARowID: "OA1", OAName: "UBND", TemplateID: "1001", TemplateName: "Họp", Tag: "CUSTOMER_CARE",
			Params: []domain.TemplateParam{{Name: "su_kien", MaxLength: 30}}, UnitPrice: "200.00"}},
		SMS: []app.SmsNoticeTemplate{{SmsAccountID: "ACC1", AccountLabel: "eSMS chính", Brandname: "XaMau", TemplateRef: "TPL1",
			Content: "{P1,20} hop {P1,5}", Params: []domain.SmsTemplateParam{{Name: "P1", MaxLen: 20}, {Name: "P1", MaxLen: 5}}}},
	}, f.err
}

func (f *fakeNoticeCampaigns) Preview(ctx context.Context, in app.NoticeInput) (app.NoticePreview, error) {
	f.note(ctx)
	f.lastInput = in
	return app.NoticePreview{RecipientCount: 10, SkippedCount: 2, MissingParams: []string{"thoi_gian"}}, f.err
}

func (f *fakeNoticeCampaigns) Send(ctx context.Context, in app.NoticeInput, a audit.Actor) (app.NoticeCampaignItem, error) {
	f.note(ctx)
	f.lastInput, f.lastActor = in, a
	return sampleNoticeItem(), f.err
}

func (f *fakeNoticeCampaigns) List(ctx context.Context, cursor string) ([]app.NoticeCampaignItem, string, error) {
	f.note(ctx)
	f.lastCursor = cursor
	return []app.NoticeCampaignItem{sampleNoticeItem()}, "NEXT", f.err
}

func (f *fakeNoticeCampaigns) Get(ctx context.Context, id string) (app.NoticeCampaignItem, error) {
	f.note(ctx)
	f.lastID = id
	return sampleNoticeItem(), f.err
}

type noticeNames struct{ err error }

func (n noticeNames) TenCanBoTheoMa(_ context.Context, codes []string) (map[string]identityclient.TenCanBo, error) { // vi-name-ok: the method of the existing app.StaffNames interface (core/identityclient)
	if n.err != nil {
		return nil, n.err
	}
	out := map[string]identityclient.TenCanBo{}
	for _, c := range codes {
		out[c] = identityclient.TenCanBo{HoTen: "Cán bộ Mẫu"}
	}
	return out, nil
}

type noticeServer struct {
	h       http.Handler
	fake    *fakeNoticeCampaigns
	checker *checkerDanhMucGia
	keys    int
}

func newNoticeServer(t *testing.T) *noticeServer {
	t.Helper()
	fake := &fakeNoticeCampaigns{}
	checker := &checkerDanhMucGia{}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	mux := http.NewServeMux()
	RegisterNoticeCampaigns(mux, NoticeCampaignDeps{Checker: checker, Campaigns: fake, Names: noticeNames{}, Log: log})
	var h http.Handler = mux
	h = chuTheGhi(h)
	h = idem.Middleware(moKhoIdemGia(), log)(h)
	h = httpx.TenantMiddleware(thuMucMau())(h)
	h = httpx.Recover(func(context.Context) string { return "test-trace" })(h)
	h = httpx.StripTenantHeaders(h)
	return &noticeServer{h: h, fake: fake, checker: checker}
}

func (s *noticeServer) grant(commune tenant.ID, perms ...authz.Perm) {
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

func (s *noticeServer) call(t *testing.T, method, path, host string, p *authz.Principal, body string) *httptest.ResponseRecorder {
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
	r.Header.Set(idem.Header, fmt.Sprintf("01JIDEMNOTICE%013d", s.keys))
	if p != nil {
		r = r.WithContext(context.WithValue(r.Context(), khoaChuTheGhi{}, *p))
	}
	w := httptest.NewRecorder()
	s.h.ServeHTTP(w, r)
	return w
}

const (
	noticePreviewBody = `{"channel":"zalo-zns","oa_id":"OA1","template_ref":"1001",` +
		`"audience":{"kind":"nguoi-dan-theo-nhom","ids":["G1"]},"params":{"su_kien":"Họp dân"}}`
	noticeSendBody = `{"title":"Họp dân","channel":"sms","sms_account_id":"ACC1","template_ref":"TPL1",` +
		`"audience":{"kind":"nguoi-dan-theo-nhan","tags":["hộ nghèo"]},"params":{"1":"Mẫu"}}`
)

// noticeRoutes — the wrong keys are real and adjacent: admin.sms / admin.zalo configure the channels the
// campaign sends through, content.read reads on the same menu group.
func noticeRoutes() []mapAssetRoute {
	return []mapAssetRoute{
		{"templates", http.MethodGet, "/api/v1/notice-templates", "", "notice.send", "admin.sms", http.StatusOK},
		{"preview", http.MethodPost, "/api/v1/notice-campaign-previews", noticePreviewBody, "notice.send", "admin.zalo", http.StatusOK},
		{"send", http.MethodPost, "/api/v1/notice-campaigns", noticeSendBody, "notice.send", "admin.sms", http.StatusCreated},
		{"list", http.MethodGet, "/api/v1/notice-campaigns?cursor=abc", "", "notice.send", "content.read", http.StatusOK},
		{"get", http.MethodGet, "/api/v1/notice-campaigns/" + testCampaignID, "", "notice.send", "admin.zalo", http.StatusOK},
	}
}

func TestNoticeRoutesAskForSeededKey(t *testing.T) {
	for _, tc := range noticeRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			s := newNoticeServer(t)
			s.call(t, tc.method, tc.path, hostA, canBoGhi(xaA), tc.body)
			// Compared with a LITERAL: a fake checker grants any string (rule 5, invariant 3c).
			if len(s.checker.hoiGi) == 0 || s.checker.hoiGi[0] != "notice.send" {
				t.Fatalf("asked for %v, want notice.send (seeded at service-identity/migrations/0028_citizen_dossier_notice_permissions.sql)", s.checker.hoiGi)
			}
		})
	}
}

func TestNoticeRoutes_401NoSession(t *testing.T) {
	for _, tc := range noticeRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			s := newNoticeServer(t)
			s.grant(xaA, tc.perm)
			doiMa(t, s.call(t, tc.method, tc.path, hostA, nil, tc.body), http.StatusUnauthorized)
			if s.fake.calls != 0 {
				t.Error("no session and the use case ran")
			}
		})
	}
}

func TestNoticeRoutes_403WrongPermission(t *testing.T) {
	for _, tc := range noticeRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			s := newNoticeServer(t)
			s.grant(xaA, tc.wrongPerm)
			doiMa(t, s.call(t, tc.method, tc.path, hostA, canBoGhi(xaA), tc.body), http.StatusForbidden)
			if s.fake.calls != 0 {
				t.Error("wrong permission and the use case ran")
			}
		})
	}
}

func TestNoticeRoutes_403RightPermissionWrongCommune(t *testing.T) {
	// Signed in at B as a member of B; the grant is in A. A checker ignoring the commune would let A's staff
	// send a campaign to B's whole directory — on B's money.
	for _, tc := range noticeRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			s := newNoticeServer(t)
			s.grant(xaA, tc.perm)
			doiMa(t, s.call(t, tc.method, tc.path, hostB, canBoGhi(xaB), tc.body), http.StatusForbidden)
			if s.fake.calls != 0 {
				t.Error("a grant in another commune was enough")
			}
		})
	}
}

func TestNoticeRoutes_200RightPermissionRightCommune(t *testing.T) {
	for _, tc := range noticeRoutes() {
		t.Run(tc.name, func(t *testing.T) {
			s := newNoticeServer(t)
			s.grant(xaA, tc.perm)
			doiMa(t, s.call(t, tc.method, tc.path, hostA, canBoGhi(xaA), tc.body), tc.ok)
			if s.fake.calls != 1 || s.fake.lastCommune != xaA {
				t.Fatalf("use case ran %d times in %q, want once in %q", s.fake.calls, s.fake.lastCommune, xaA)
			}
		})
	}
}

// --- shapes --------------------------------------------------------------------------------------------

func TestNoticeSendPassesTheBodyAndTheStaffCode(t *testing.T) {
	s := newNoticeServer(t)
	s.grant(xaA, "notice.send")
	w := s.call(t, http.MethodPost, "/api/v1/notice-campaigns", hostA, canBoGhi(xaA), noticeSendBody)
	doiMa(t, w, http.StatusCreated)
	in := s.fake.lastInput
	if in.Title != "Họp dân" || in.Channel != "sms" || in.SmsAccountID != "ACC1" || in.TemplateRef != "TPL1" ||
		in.AudienceKind != domain.NoticeAudienceCitizenTag || len(in.AudienceTags) != 1 || in.Params["1"] != "Mẫu" {
		t.Fatalf("input = %+v", in)
	}
	// Rule 6, invariant 8: the BUSINESS CODE, never the internal id.
	if s.fake.lastActor.ID != maCanBoGhi || s.fake.lastActor.IP != "10.0.0.7" {
		t.Fatalf("actor = %+v", s.fake.lastActor)
	}
	var out map[string]json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"id", "title", "channel", "source_name", "template_name", "audience_kind", "recipient_count",
		"skipped_count", "sent_count", "failed_count", "estimated_cost", "status", "created_at", "created_by", "created_by_name",
		"finished_at"} {
		if _, ok := out[k]; !ok {
			t.Errorf("reply lacks %q: %s", k, w.Body.String())
		}
	}
	if string(out["finished_at"]) != "null" || string(out["estimated_cost"]) != `"3500.00"` ||
		string(out["created_by_name"]) != `"Cán bộ Mẫu"` || string(out["oa_id"]) != "null" {
		t.Fatalf("reply = %s", w.Body.String())
	}
}

func TestNoticeSendNeedsAnIdempotencyKey(t *testing.T) {
	s := newNoticeServer(t)
	s.grant(xaA, "notice.send")
	r := httptest.NewRequest(http.MethodPost, "https://"+hostA+"/api/v1/notice-campaigns", strings.NewReader(noticeSendBody))
	r.Host = hostA
	r = r.WithContext(context.WithValue(r.Context(), khoaChuTheGhi{}, *canBoGhi(xaA)))
	w := httptest.NewRecorder()
	s.h.ServeHTTP(w, r)
	if w.Code == http.StatusCreated || s.fake.calls != 0 {
		t.Fatalf("a send without Idempotency-Key went through: %d", w.Code)
	}
}

func TestNoticeTemplatesShape(t *testing.T) {
	s := newNoticeServer(t)
	s.grant(xaA, "notice.send")
	w := s.call(t, http.MethodGet, "/api/v1/notice-templates", hostA, canBoGhi(xaA), "")
	doiMa(t, w, http.StatusOK)
	var out struct {
		ZNS []map[string]any `json:"zns"`
		SMS []struct {
			Params []struct {
				Field     string `json:"field"`
				Name      string `json:"name"`
				MaxLength int    `json:"max_length"`
			} `json:"params"`
			UnitPrice *string `json:"unit_price"`
		} `json:"sms"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil || len(out.ZNS) != 1 || len(out.SMS) != 1 {
		t.Fatalf("%s %v", w.Body.String(), err)
	}
	if out.ZNS[0]["template_ref"] != "1001" || out.ZNS[0]["oa_id"] != "OA1" || out.ZNS[0]["unit_price"] != "200.00" ||
		out.ZNS[0]["channel"] != "zalo-zns" {
		t.Fatalf("zns = %v", out.ZNS[0])
	}
	// SMS slots are keyed by POSITION — the same name twice is two slots.
	p := out.SMS[0].Params
	if len(p) != 2 || p[0].Field != "1" || p[1].Field != "2" || p[1].Name != "P1" || p[1].MaxLength != 5 || out.SMS[0].UnitPrice != nil {
		t.Fatalf("sms = %+v", out.SMS[0])
	}
}

func TestNoticePreviewShapeAndListCursor(t *testing.T) {
	s := newNoticeServer(t)
	s.grant(xaA, "notice.send")
	w := s.call(t, http.MethodPost, "/api/v1/notice-campaign-previews", hostA, canBoGhi(xaA), noticePreviewBody)
	doiMa(t, w, http.StatusOK)
	if got := strings.TrimSpace(w.Body.String()); got != `{"content":null,"recipient_count":10,"skipped_count":2,"unit_price":null,`+
		`"estimated_cost":null,"missing_params":["thoi_gian"]}` {
		t.Fatalf("preview = %s", got)
	}
	if in := s.fake.lastInput; in.OARowID != "OA1" || in.AudienceKind != domain.NoticeAudienceCitizenGroup || in.AudienceIDs[0] != "G1" {
		t.Fatalf("input = %+v", in)
	}
	w = s.call(t, http.MethodGet, "/api/v1/notice-campaigns?cursor=abc", hostA, canBoGhi(xaA), "")
	doiMa(t, w, http.StatusOK)
	if s.fake.lastCursor != "abc" || !strings.Contains(w.Body.String(), `"next_cursor":"NEXT"`) {
		t.Fatalf("list = %s, cursor %q", w.Body.String(), s.fake.lastCursor)
	}
}

func TestNoticeErrorsMapToTheirCodes(t *testing.T) {
	cases := []struct {
		err    error
		status int
		code   string
	}{
		{domain.ErrNoticeAudience, http.StatusBadRequest, "notice_invalid"},
		{app.ErrNoticeSourceMissing, http.StatusNotFound, "not_found"},
		{app.ErrNoticeTemplateMissing, http.StatusNotFound, "not_found"},
		{app.ErrNoticeNoRecipients, http.StatusConflict, "no_recipients"},
		{app.ErrNoticeTemplateNotSendable, http.StatusConflict, "template_not_sendable"},
		{app.ErrNoticeSourceUnavailable, http.StatusConflict, "source_unavailable"},
		{&app.NoticeMissingParamsError{Missing: []string{"2"}}, http.StatusUnprocessableEntity, "missing_params"},
		{fmt.Errorf("%w: %w", app.ErrNoticeIdentityUnavailable, identityclient.ErrIdentityUnavailable), http.StatusServiceUnavailable, "identity_unavailable"},
		{crypto.ErrNotConfigured, http.StatusServiceUnavailable, "encryption_not_configured"},
		{errors.New("boom"), http.StatusInternalServerError, "internal"},
	}
	for _, tc := range cases {
		t.Run(tc.code+"/"+tc.err.Error(), func(t *testing.T) {
			s := newNoticeServer(t)
			s.grant(xaA, "notice.send")
			s.fake.err = tc.err
			w := s.call(t, http.MethodPost, "/api/v1/notice-campaigns", hostA, canBoGhi(xaA), noticeSendBody)
			if w.Code != tc.status || !strings.Contains(w.Body.String(), `"`+tc.code+`"`) {
				t.Fatalf("= %d %s", w.Code, w.Body.String())
			}
			if strings.Contains(w.Body.String(), "boom") || strings.Contains(w.Body.String(), "identityclient") {
				t.Fatalf("the internal error reached the client: %s", w.Body.String())
			}
		})
	}
	s := newNoticeServer(t)
	s.grant(xaA, "notice.send")
	s.fake.err = app.ErrNoticeCampaignNotFound
	doiMa(t, s.call(t, http.MethodGet, "/api/v1/notice-campaigns/nope", hostA, canBoGhi(xaA), ""), http.StatusNotFound)
}

func TestNoticeNamesDownStillAnswers(t *testing.T) {
	fake := &fakeNoticeCampaigns{}
	checker := &checkerDanhMucGia{co: map[tenant.ID]map[authz.Perm]struct{}{xaA: {"notice.send": {}}}}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	mux := http.NewServeMux()
	RegisterNoticeCampaigns(mux, NoticeCampaignDeps{Checker: checker, Campaigns: fake, Names: noticeNames{err: errors.New("down")}, Log: log})
	var h http.Handler = chuTheGhi(mux)
	h = httpx.TenantMiddleware(thuMucMau())(h)
	r := httptest.NewRequest(http.MethodGet, "https://"+hostA+"/api/v1/notice-campaigns", nil)
	r.Host = hostA
	r = r.WithContext(context.WithValue(r.Context(), khoaChuTheGhi{}, *canBoGhi(xaA)))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"created_by_name":""`) {
		t.Fatalf("= %d %s", w.Code, w.Body.String())
	}
}
