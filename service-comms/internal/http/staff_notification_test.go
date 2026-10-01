package http

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
	commsstore "github.com/vihat/vigov/service-comms/internal/store"
)

// THE HEADER BELL — the signed-in staff member's own inbox. What these tests defend:
//
//  1. the permission shape of AnyAuthenticated: 401 without a session, 401 for a session of another
//     commune, 403 for a non-staff principal, 200 for staff of the commune;
//  2. ISOLATION BETWEEN STAFF: the recipient every read and write is filtered by is the session's
//     OWN code — never a query parameter, never a body field — so a colleague's notice is invisible
//     and cannot be marked;
//  3. the unread count and both mark-read routes reach the use case with the session's code as the
//     actor (the trail's "who").

// fakeInbox holds notices KEYED BY (commune, recipient code) and reads the commune from the context
// exactly as *store.Scoped does. Keyed any other way, the isolation cases would pass proving nothing.
type fakeInbox struct {
	rows map[tenant.ID]map[string][]domain.StaffNotification

	listCalls, countCalls, markCalls, markAllCalls int
	lastRecipient                                  string
	lastActor                                      audit.Actor
	lastTenant                                     tenant.ID
}

func (f *fakeInbox) ListOwn(ctx context.Context, recipientCode string, _ page.Request) (
	page.Result[domain.StaffNotification], error) {
	f.listCalls++
	f.lastRecipient, f.lastTenant = recipientCode, tenant.MustFrom(ctx)
	res := page.NewResult[domain.StaffNotification]()
	res.Items = append(res.Items, f.rows[tenant.MustFrom(ctx)][recipientCode]...)
	return res, nil
}

func (f *fakeInbox) CountUnread(ctx context.Context, recipientCode string) (int, error) {
	f.countCalls++
	f.lastRecipient, f.lastTenant = recipientCode, tenant.MustFrom(ctx)
	n := 0
	for _, r := range f.rows[tenant.MustFrom(ctx)][recipientCode] {
		if !r.Read() {
			n++
		}
	}
	return n, nil
}

func (f *fakeInbox) MarkRead(ctx context.Context, id string, actor audit.Actor) (domain.StaffNotification, error) {
	f.markCalls++
	f.lastActor, f.lastTenant = actor, tenant.MustFrom(ctx)
	own := f.rows[tenant.MustFrom(ctx)][actor.ID]
	for i := range own {
		if own[i].ID == id {
			if !own[i].Read() {
				own[i].ReadAt = time.Date(2026, 9, 29, 3, 0, 0, 0, time.UTC)
			}
			return own[i], nil
		}
	}
	return domain.StaffNotification{}, commsstore.ErrStaffNotificationNotFound
}

func (f *fakeInbox) MarkAllRead(ctx context.Context, actor audit.Actor) (int, error) {
	f.markAllCalls++
	f.lastActor, f.lastTenant = actor, tenant.MustFrom(ctx)
	n := 0
	own := f.rows[tenant.MustFrom(ctx)][actor.ID]
	for i := range own {
		if !own[i].Read() {
			own[i].ReadAt = time.Date(2026, 9, 29, 3, 0, 0, 0, time.UTC)
			n++
		}
	}
	return n, nil
}

const colleagueCode = "CB-00999"

func inboxFixture() *fakeInbox {
	at := time.Date(2026, 9, 29, 1, 0, 0, 0, time.UTC)
	return &fakeInbox{rows: map[tenant.ID]map[string][]domain.StaffNotification{
		xaA: {
			maCanBoGhi: {
				{ID: "tb-mine-1", RecipientCode: maCanBoGhi, Kind: domain.StaffNotificationDueSoon,
					Title: "Bạn có 3 việc sắp đến hạn", Link: "/nhiem-vu?soon=true", CreatedAt: at},
				{ID: "tb-mine-2", RecipientCode: maCanBoGhi, Kind: domain.StaffNotificationOverdue,
					Title: "Việc quá hạn", CreatedAt: at, ReadAt: at},
			},
			colleagueCode: {
				{ID: "tb-colleague", RecipientCode: colleagueCode, Kind: domain.StaffNotificationEscalation,
					Title: "THONG-BAO-CUA-DONG-NGHIEP", CreatedAt: at},
			},
		},
		xaB: {
			maCanBoGhi: {
				{ID: "tb-other-commune", RecipientCode: maCanBoGhi, Kind: domain.StaffNotificationDueSoon,
					Title: "THONG-BAO-XA-KHAC", CreatedAt: at},
			},
		},
	}}
}

type inboxServer struct {
	h     http.Handler
	inbox *fakeInbox
}

func newInboxServer(t *testing.T) *inboxServer {
	t.Helper()
	inbox := inboxFixture()
	im := slog.New(slog.NewTextHandler(io.Discard, nil))
	mux := http.NewServeMux()
	Register(mux, Deps{
		Checker:              &checkerDanhMucGia{},
		LoaiTaiNguyen:        danhMucMau(),
		GhiLoaiTaiNguyen:     &ghiDanhMucGia{},
		ThongBao:             &soThongBaoGia{},
		GhiThongBao:          &ghiThongBaoGia{},
		NoiDung:              &soNoiDungGia{},
		GhiNoiDung:           &ghiNoiDungGia{},
		DanhMucNoiDung:       &soDanhMucNDGia{},
		GhiDanhMucNoiDung:    &ghiDanhMucNDGia{},
		ContentCovers:        &fakeCovers{},
		ContentAudio:         &fakeAudio{},
		MapFieldSchemas:      &fakeMapFieldSchemas{},
		WriteMapFieldSchemas: &fakeMapFieldSchemas{},
		MailSettings:         &fakeMailSettings{},
		WriteMailSettings:    &fakeMailSettings{},
		AuditLog:             &auditLogFake{},
		StaffInbox:           inbox,
		WriteStaffInbox:      inbox,
		PortalSync:           &fakePortalSync{},
		WritePortalSync:      &fakePortalSync{},
		Log:                  im,
	})
	var h http.Handler = mux
	h = chuTheGhi(h)
	h = idem.Middleware(nil, im)(h)
	h = httpx.TenantMiddleware(thuMucMau())(h)
	h = httpx.Recover(func(context.Context) string { return "test-trace" })(h)
	h = httpx.StripTenantHeaders(h)
	return &inboxServer{h: h, inbox: inbox}
}

func (s *inboxServer) call(t *testing.T, method, host, path string, p *authz.Principal, body string) *httptest.ResponseRecorder {
	t.Helper()
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	r := httptest.NewRequest(method, "https://"+host+path, rd)
	r.Host = host
	r.RemoteAddr = "10.0.0.7:51000"
	r.Header.Set("Content-Type", "application/json")
	if p != nil {
		r = r.WithContext(context.WithValue(r.Context(), khoaChuTheGhi{}, *p))
	}
	w := httptest.NewRecorder()
	s.h.ServeHTTP(w, r)
	return w
}

type inboxRoute struct {
	name, method, path, body string
}

func inboxRoutes() []inboxRoute {
	return []inboxRoute{
		{"list", http.MethodGet, "/api/v1/notifications", ""},
		{"unread-count", http.MethodGet, "/api/v1/notifications/unread-count", ""},
		{"mark-one", http.MethodPatch, "/api/v1/notifications/tb-mine-1", `{"read":true}`},
		{"mark-all", http.MethodPatch, "/api/v1/notifications", `{"read":true}`},
	}
}

func (f *fakeInbox) total() int { return f.listCalls + f.countCalls + f.markCalls + f.markAllCalls }

// --- (1) the permission shape --------------------------------------------------------------------

func TestInbox_401WithoutSession(t *testing.T) {
	for _, rt := range inboxRoutes() {
		t.Run(rt.name, func(t *testing.T) {
			s := newInboxServer(t)
			doiMa(t, s.call(t, rt.method, hostA, rt.path, nil, rt.body), http.StatusUnauthorized)
			if s.inbox.total() != 0 {
				t.Error("chưa đăng nhập mà đã chạm hộp thư")
			}
		})
	}
}

// AnyAuthenticated has no permission to lack; the commune check is what remains, and a session of
// another commune presented here is 401 — the same answer authz gives on every staff route.
func TestInbox_401SessionOfAnotherCommune(t *testing.T) {
	for _, rt := range inboxRoutes() {
		t.Run(rt.name, func(t *testing.T) {
			s := newInboxServer(t)
			doiMa(t, s.call(t, rt.method, hostA, rt.path, canBoGhi(xaB), rt.body), http.StatusUnauthorized)
			if s.inbox.total() != 0 {
				t.Error("phiên của xã khác mà đã chạm hộp thư của xã này")
			}
		})
	}
}

func TestInbox_403NonStaffPrincipal(t *testing.T) {
	for _, rt := range inboxRoutes() {
		t.Run(rt.name, func(t *testing.T) {
			s := newInboxServer(t)
			citizen := &authz.Principal{ID: "cd-opaque", Kind: "citizen", TenantID: xaA}
			doiMa(t, s.call(t, rt.method, hostA, rt.path, citizen, rt.body), http.StatusForbidden)
			if s.inbox.total() != 0 {
				t.Error("chủ thể không phải cán bộ mà đã chạm hộp thư")
			}
		})
	}
}

func TestInbox_500StaffWithoutCodeNoFallbackToID(t *testing.T) {
	for _, rt := range inboxRoutes() {
		t.Run(rt.name, func(t *testing.T) {
			s := newInboxServer(t)
			p := canBoGhi(xaA)
			p.Ma = ""
			doiMa(t, s.call(t, rt.method, hostA, rt.path, p, rt.body), http.StatusInternalServerError)
			if s.inbox.total() != 0 {
				t.Error("không có mã cán bộ mà vẫn chạm hộp thư — có lẽ đã dùng id nội bộ thay mã")
			}
		})
	}
}

func TestInbox_200StaffOfTheCommune(t *testing.T) {
	for _, rt := range inboxRoutes() {
		t.Run(rt.name, func(t *testing.T) {
			s := newInboxServer(t)
			doiMa(t, s.call(t, rt.method, hostA, rt.path, canBoGhi(xaA), rt.body), http.StatusOK)
			if s.inbox.lastTenant != xaA {
				t.Errorf("chạy trong xã %q, muốn %q", s.inbox.lastTenant, xaA)
			}
		})
	}
}

// --- (2) isolation between staff -----------------------------------------------------------------

func TestInbox_ListShowsOnlyOwnNotices(t *testing.T) {
	s := newInboxServer(t)
	// A `recipient` parameter is IGNORED: nothing on this route reads one.
	w := s.call(t, http.MethodGet, hostA, "/api/v1/notifications?recipient="+colleagueCode+"&recipient_code="+colleagueCode,
		canBoGhi(xaA), "")
	doiMa(t, w, http.StatusOK)
	if s.inbox.lastRecipient != maCanBoGhi {
		t.Fatalf("đọc hộp thư của %q, muốn của chính phiên %q", s.inbox.lastRecipient, maCanBoGhi)
	}
	var out page.Result[notificationOut]
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Items) != 2 {
		t.Fatalf("có %d mục, muốn 2 mục của chính mình", len(out.Items))
	}
	for _, it := range out.Items {
		if it.ID == "tb-colleague" || it.ID == "tb-other-commune" {
			t.Errorf("lộ thông báo không phải của mình: %s", it.ID)
		}
	}
	if strings.Contains(w.Body.String(), "DONG-NGHIEP") || strings.Contains(w.Body.String(), "XA-KHAC") {
		t.Error("thân trả về chứa thông báo của người khác hoặc xã khác")
	}
	if !out.Items[1].Read || out.Items[1].ReadAt == nil || out.Items[0].Read || out.Items[0].ReadAt != nil {
		t.Errorf("trạng thái đọc sai: %+v", out.Items)
	}
}

func TestInbox_MarkColleagueNoticeIs404AndChangesNothing(t *testing.T) {
	s := newInboxServer(t)
	w := s.call(t, http.MethodPatch, hostA, "/api/v1/notifications/tb-colleague", canBoGhi(xaA), `{"read":true}`)
	doiMa(t, w, http.StatusNotFound)
	if s.inbox.rows[xaA][colleagueCode][0].Read() {
		t.Error("đánh dấu được thông báo của đồng nghiệp")
	}
	if s.inbox.lastActor.ID != maCanBoGhi {
		t.Errorf("use case nhận chủ thể %q, muốn mã của phiên", s.inbox.lastActor.ID)
	}
	// The same answer as an id that exists nowhere — the existence of the colleague's notice is not
	// confirmed.
	w2 := s.call(t, http.MethodPatch, hostA, "/api/v1/notifications/khong-co", canBoGhi(xaA), `{"read":true}`)
	if w.Code != w2.Code || loiTra(t, w).Code != loiTra(t, w2).Code || loiTra(t, w).Message != loiTra(t, w2).Message {
		t.Error("thông báo của người khác và thông báo không tồn tại trả lời khác nhau")
	}
}

// --- (3) count and marks --------------------------------------------------------------------------

func TestInbox_UnreadCountIsOwnUnreadOnly(t *testing.T) {
	s := newInboxServer(t)
	w := s.call(t, http.MethodGet, hostA, "/api/v1/notifications/unread-count", canBoGhi(xaA), "")
	doiMa(t, w, http.StatusOK)
	var out unreadCountOut
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Unread != 1 {
		t.Errorf("unread = %d, muốn 1 (một chưa đọc của mình; của đồng nghiệp và xã khác không tính)", out.Unread)
	}
}

func TestInbox_MarkOneReadReturnsItAndPassesSessionActor(t *testing.T) {
	s := newInboxServer(t)
	w := s.call(t, http.MethodPatch, hostA, "/api/v1/notifications/tb-mine-1", canBoGhi(xaA), `{"read":true}`)
	doiMa(t, w, http.StatusOK)
	var out notificationOut
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if !out.Read || out.ReadAt == nil || out.ID != "tb-mine-1" {
		t.Errorf("= %+v", out)
	}
	if a := s.inbox.lastActor; a.ID != maCanBoGhi || a.Kind != "staff" || a.IP == "" {
		t.Errorf("chủ thể = %+v, muốn mã cán bộ của phiên, kind staff, có IP", a)
	}
	// And the badge drops.
	var cnt unreadCountOut
	_ = json.Unmarshal(s.call(t, http.MethodGet, hostA, "/api/v1/notifications/unread-count", canBoGhi(xaA), "").Body.Bytes(), &cnt)
	if cnt.Unread != 0 {
		t.Errorf("sau khi đọc, unread = %d, muốn 0", cnt.Unread)
	}
}

func TestInbox_MarkAllReadTouchesOnlyOwn(t *testing.T) {
	s := newInboxServer(t)
	w := s.call(t, http.MethodPatch, hostA, "/api/v1/notifications", canBoGhi(xaA), `{"read":true}`)
	doiMa(t, w, http.StatusOK)
	var out markAllReadOut
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Marked != 1 {
		t.Errorf("marked = %d, muốn 1", out.Marked)
	}
	if s.inbox.rows[xaA][colleagueCode][0].Read() || s.inbox.rows[xaB][maCanBoGhi][0].Read() {
		t.Error("Đọc hết đánh dấu cả thông báo của đồng nghiệp hoặc của xã khác")
	}
}

func TestInbox_MarkReadRefusesFalseOrMissing(t *testing.T) {
	for _, body := range []string{`{"read":false}`, `{}`, `nope`} {
		for _, path := range []string{"/api/v1/notifications/tb-mine-1", "/api/v1/notifications"} {
			s := newInboxServer(t)
			doiMa(t, s.call(t, http.MethodPatch, hostA, path, canBoGhi(xaA), body), http.StatusBadRequest)
			if s.inbox.markCalls+s.inbox.markAllCalls != 0 {
				t.Errorf("%s %s: thân sai mà use case vẫn chạy", path, body)
			}
		}
	}
}
