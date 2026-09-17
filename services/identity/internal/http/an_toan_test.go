package http

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vihat/vigov/pkg/authz"
	"github.com/vihat/vigov/pkg/httpx"
	"github.com/vihat/vigov/pkg/tenant"
	"github.com/vihat/vigov/pkg/token"
	"github.com/vihat/vigov/services/identity/internal/domain"
	idstore "github.com/vihat/vigov/services/identity/internal/store"
)

// Three defect classes that routes_test.go does not reach:
//
//  1. PERSONAL DATA ON THE LOG PATH (rule 3, invariant 1). Nobody sees it until somebody reads
//     centralised logging — by which time the value is also in backups and at a monitoring
//     vendor, and cannot be recalled.
//  2. A CLIENT NAMING ITS OWN COMMUNE (rule 1, forbidden #2). The chain strips those headers;
//     nothing asserted that it does, so removing the middleware would break nothing visible.
//  3. CONCURRENCY. One process serves every commune at once. A principal that leaks between two
//     in-flight requests is a cross-commune read that no sequential test can produce.

// --- harness with a readable log ---------------------------------------------------------------

// mayChuLog is dungMayChu with the logger pointed at a buffer, so what the edge writes can be
// asserted on. Everything else is the real chain, in the real order.
func mayChuLog(t *testing.T) (*mayChu, *bytes.Buffer) {
	t.Helper()
	m := dungMayChu(t)

	var buf bytes.Buffer
	m.them = func(mux *http.ServeMux, d Deps) {
		mux.Handle("GET "+duongThu,
			authz.RequirePermission(d.Checker, quyenThu)(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					p, _ := authz.From(r.Context())
					vietJSON(w, http.StatusOK, map[string]string{
						"principal_id": p.ID,
						"tenant_id":    string(p.TenantID),
					})
				})))
	}
	// The REAL chain, rebuilt in the real order with only the logger swapped — see dungLai. Built
	// by hand here until now, which meant this copy had to be kept in step with the other two by
	// hand as well.
	m.dungLai(t, func(d *Deps) {
		d.Log = slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	})
	return m, &buf
}

func TestLogCuaBienKhongChuaDuLieuCaNhanHayThongTinDangNhap(t *testing.T) {
	// Rule 3, invariant 1: personal data never enters logs, not even at debug level. The account
	// used here carries the agreed fake number and a fake hash precisely so this assertion has
	// something to look for.
	m, log := mayChuLog(t)

	// Every path the edge can take, including the ones that log on purpose.
	m.goi(t, "POST", hostA, "/api/v1/sessions",
		`{"email":"`+emailDung+`","password":"`+matKhauDung+`"}`, "")
	m.goi(t, "POST", hostA, "/api/v1/sessions",
		`{"email":"`+emailDung+`","password":"`+matKhauDung+`"}`, "token-hong-khong-giai-duoc")
	m.goi(t, "POST", hostA, "/api/v1/sessions", `{khong-phai-json`, "")
	m.goi(t, "GET", hostB, "/api/v1/staff", "", m.tokenCho(t, xaA, sidA)) // the security alert
	m.goi(t, "GET", hostA, "/api/v1/staff", "", m.tokenCho(t, xaA, sidA))
	m.goi(t, "DELETE", hostA, "/api/v1/sessions/"+sidA, "", m.tokenCho(t, xaA, sidA))

	ra := log.String()
	cam := map[string]string{
		"0900000000":   "số điện thoại — dữ liệu cá nhân theo Nghị định 13/2023",
		matKhauDung:    "mật khẩu thô",
		"argon2":       "chuỗi băm mật khẩu",
		"Nguyễn Văn A": "họ tên — dữ liệu cá nhân",
	}
	for xau, vi := range cam {
		if strings.Contains(ra, xau) {
			t.Errorf("log của biên chứa %s:\n%s", vi, ra)
		}
	}
}

func TestCanhBaoTokenSaiXaKhongMangSidThat(t *testing.T) {
	// The alert exists to be read by a person investigating a probe, and it must be readable
	// WITHOUT carrying a working credential: a log line holding the sid is a second copy of that
	// session id, in a pipeline nobody can recall it from (rule 8). A fingerprint correlates the
	// lines just as well.
	m, log := mayChuLog(t)

	w := m.goi(t, "GET", hostB, "/api/v1/staff", "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusUnauthorized)

	ra := log.String()
	if !strings.Contains(ra, "CẢNH BÁO AN NINH") {
		t.Fatalf("token của xã khác không sinh cảnh báo an ninh:\n%s", ra)
	}
	if strings.Contains(ra, sidA) {
		t.Errorf("cảnh báo mang nguyên sid — đó là một bản sao của chính thông tin đăng nhập:\n%s", ra)
	}
	if !strings.Contains(ra, vanTay(sidA)) {
		t.Errorf("cảnh báo thiếu vân tay sid nên không đối chiếu được hai dòng log:\n%s", ra)
	}
	for _, phai := range []string{string(xaA), string(xaB), "10.0.0.7"} {
		if !strings.Contains(ra, phai) {
			t.Errorf("cảnh báo thiếu %q — người điều tra cần biết xã nào và IP nào:\n%s", phai, ra)
		}
	}
}

func TestLoi500KhongLoNoiDungLoiRaNgoai(t *testing.T) {
	// Rule 3, forbidden #3: personal data must never leave in an error message. A lower layer
	// wrapping a driver error can carry a column value with it, so the edge answers with a fixed
	// sentence and keeps the detail on the server side.
	m, _ := mayChuLog(t)
	m.dangXuat.loi = errors.New("pq: duplicate key value violates unique constraint (dien_thoai)=(0900000000)")

	w := m.goi(t, "DELETE", hostA, "/api/v1/sessions/"+sidA, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusInternalServerError)

	than := w.Body.String()
	for _, cam := range []string{"0900000000", "pq:", "dien_thoai", "constraint"} {
		if strings.Contains(than, cam) {
			t.Errorf("thân phản hồi 500 chứa %q: %s", cam, than)
		}
	}
	if got := loiTra(t, w).Code; got != "internal" {
		t.Errorf("code = %q, muốn internal", got)
	}
}

func TestHeaderXaDoKhachTuGuiBiBoQua(t *testing.T) {
	// Rule 1, forbidden #2: a client naming its own commune is a client granting itself access.
	// The commune comes from Host and from nowhere else — the header must be dropped before any
	// handler can read it, and the request must still resolve to the Host's commune.
	m, _ := mayChuLog(t)

	// duongThu, not the staff route: this case has to READ the commune the edge resolved, and
	// only the harness route echoes it.
	r := httptest.NewRequest("GET", "https://"+hostA+duongThu, nil)
	r.Host = hostA
	r.RemoteAddr = "10.0.0.7:51000"
	r.Header.Set("X-Tenant-Id", string(xaB))
	r.Header.Set("X-Tenant-Host", hostB)
	r.AddCookie(&http.Cookie{Name: CookiePhien, Value: m.tokenCho(t, xaA, sidA)})

	w := httptest.NewRecorder()
	m.h.ServeHTTP(w, r)

	doiMa(t, w, http.StatusOK)
	if strings.Contains(w.Body.String(), string(xaB)) {
		t.Fatalf("yêu cầu được phục vụ theo xã do khách tự khai: %s", w.Body.String())
	}
	if !strings.Contains(w.Body.String(), string(xaA)) {
		t.Fatalf("xã phải lấy từ Host: %s", w.Body.String())
	}
	if r.Header.Get("X-Tenant-Id") != "" {
		t.Error("header xã do khách gửi vẫn còn trên request khi tới handler")
	}
}

// --- concurrency --------------------------------------------------------------------------------

// Race-safe fakes. The ones in routes_test.go count their calls without a lock, which is correct
// for a sequential test and useless for this one.

type phienAnToan struct {
	mu    sync.Mutex
	phien map[string]idstore.Phien
}

func (p *phienAnToan) KiemTra(_ context.Context, sid string) (idstore.Phien, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	ph, ok := p.phien[sid]
	if !ok {
		return idstore.Phien{}, idstore.ErrPhienKhongTonTai
	}
	return ph, nil
}

func (p *phienAnToan) GhiNhanDung(context.Context, string) {}

type canBoAnToan struct {
	mu   sync.Mutex
	theo map[string]domain.CanBo
}

func (c *canBoAnToan) TheoID(_ context.Context, id string) (domain.CanBo, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	cb, ok := c.theo[id]
	if !ok {
		return domain.CanBo{}, idstore.ErrCanBoKhongTonTai
	}
	return cb, nil
}

func TestNhieuYeuCauDongThoiKhongLanXaSangNhau(t *testing.T) {
	// One process serves every commune at once, and the commune lives on the context. If it ever
	// lands anywhere shared — a struct field, a package variable, a reused buffer — two requests
	// in flight can swap communes. That is a cross-commune read produced by load alone, and no
	// sequential test can produce it. Run with -race.
	hetHan := time.Now().UTC().Add(idstore.ThoiHanPhien)
	phien := &phienAnToan{phien: map[string]idstore.Phien{
		sidA: {ID: sidA, NguoiDungID: idNoiBo, HetHanLuc: hetHan},
		sidB: {ID: sidB, NguoiDungID: idNoiBo, HetHanLuc: hetHan},
	}}
	canBo := &canBoAnToan{theo: map[string]domain.CanBo{idNoiBo: canBoMau()}}

	d := Deps{
		Checker: checkerGia{quyen: map[tenant.ID]map[string]map[authz.Perm]bool{
			xaA: {idNoiBo: {quyenThu: true}},
			xaB: {}, // the same person, no grant in commune B
		}},
		Signer:   dungMayChu(t).signer,
		Phien:    phien,
		CanBo:    canBo,
		DangNhap: &dangNhapGia{sid: sidA, hetHan: hetHan},
		DangXuat: &dangXuatGia{},
		Log:      slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)),
	}

	mux := http.NewServeMux()
	mux.Handle("GET /api/v1/staff",
		authz.RequirePermission(d.Checker, quyenThu)(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				p, _ := authz.From(r.Context())
				vietJSON(w, http.StatusOK, map[string]string{"tenant_id": string(p.TenantID)})
			})))
	var h http.Handler = mux
	h = XacThuc(d)(h)
	h = httpx.TenantMiddleware(thuMucGia{
		hostA: {ID: xaA, Host: hostA, Active: true},
		hostB: {ID: xaB, Host: hostB, Active: true},
	})(h)
	h = httpx.Recover(func(context.Context) string { return "test-trace" })(h)
	h = httpx.StripTenantHeaders(h)

	signer := d.Signer
	ky := func(xa tenant.ID, sid string) string {
		tok, err := signer.Ky(token.Claims{TenantID: xa, Sid: sid, ExpiresAt: hetHan})
		if err != nil {
			t.Fatalf("ký token: %v", err)
		}
		return tok
	}

	type ca struct {
		ten    string
		host   string
		tok    string
		ma     int
		thanCo string // expected substring in the body, "" = not checked
	}
	cases := []ca{
		{"xã A đúng quyền", hostA, ky(xaA, sidA), http.StatusOK, string(xaA)},
		{"token xã A tới host xã B", hostB, ky(xaA, sidA), http.StatusUnauthorized, "tenant_mismatch"},
		{"xã B không có quyền", hostB, ky(xaB, sidB), http.StatusForbidden, ""},
		{"không có token", hostA, "", http.StatusUnauthorized, ""},
		{"token hỏng", hostA, "token-hong", http.StatusUnauthorized, ""},
	}

	var wg sync.WaitGroup
	for lap := 0; lap < 40; lap++ {
		for _, c := range cases {
			wg.Add(1)
			go func(c ca) {
				defer wg.Done()
				r := httptest.NewRequest("GET", "https://"+c.host+"/api/v1/staff", nil)
				r.Host = c.host
				r.RemoteAddr = "10.0.0.7:51000"
				if c.tok != "" {
					r.AddCookie(&http.Cookie{Name: CookiePhien, Value: c.tok})
				}
				w := httptest.NewRecorder()
				h.ServeHTTP(w, r)

				if w.Code != c.ma {
					t.Errorf("%s: mã = %d, muốn %d — thân: %s", c.ten, w.Code, c.ma, w.Body.String())
					return
				}
				if c.thanCo != "" && !strings.Contains(w.Body.String(), c.thanCo) {
					t.Errorf("%s: thân thiếu %q: %s", c.ten, c.thanCo, w.Body.String())
				}
				// The decisive assertion: commune B must never appear in a response served for
				// commune A, however many requests are in flight.
				if c.host == hostA && strings.Contains(w.Body.String(), string(xaB)) {
					t.Errorf("%s: phản hồi của xã A mang mã xã B: %s", c.ten, w.Body.String())
				}
			}(c)
		}
	}
	wg.Wait()
}
