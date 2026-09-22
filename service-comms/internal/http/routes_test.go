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

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-comms/internal/domain"
)

// Harness for the comms routes. Nothing here is a PostgreSQL: the properties under test are
// ordering and isolation, and a test that needs infrastructure is a test that stops being run —
// this repository's integration suites skip themselves silently without VIGOV_TEST_DSN.
//
// WHAT THIS HARNESS STILL FAKES, said plainly so nobody reads more into a green run than it proves:
// `xacThucGia` below puts a principal into the context directly, so the tests here prove what
// authz.AnyAuthenticated and the handler do WITH a principal, and nothing about how one is obtained.
//
// The real edge exists now — core/staffauth.Middleware, asking identity over gRPC, mounted by
// cmd/server.dungBien — and it is exercised where it is wired, in cmd/server/main_test.go. Keeping
// the injection here is deliberate rather than leftover: the route's own properties (ordering,
// isolation, refusal) must stay testable without a resolver, and a harness that had to stand up a
// fake identity to assert a sort order is a harness that gets bypassed.

// --- fixtures ---------------------------------------------------------------------------

const (
	hostA = "xa-a.example.gov.vn"
	hostB = "xa-b.example.gov.vn"

	// The internal staff id, the value identity's Checker matches on. Never the business code.
	idCanBo = "nd-01JINTERNALIDCUACANBO"
)

var (
	xaA = tenant.ID("01JA" + strings.Repeat("A", 22))
	xaB = tenant.ID("01JB" + strings.Repeat("B", 22))
)

type thuMucGia map[string]tenant.Tenant

func (m thuMucGia) ByHost(_ context.Context, host string) (tenant.Tenant, bool) {
	t, ok := m[host]
	return t, ok
}

// thuMucMau is the platform registry for the two test communes. A NEW MAP PER CALL, so a test that
// rewrites one copy cannot change another's.
func thuMucMau() thuMucGia {
	return thuMucGia{
		hostA: {ID: xaA, Host: hostA, Name: "Xã Thăng Bình", Province: "Thành phố Đà Nẵng", Active: true},
		hostB: {ID: xaB, Host: hostB, Name: "Xã Bình Dương", Active: true},
	}
}

// danhMucGia is the map-asset-type catalogue, KEYED BY COMMUNE, reading the commune from the
// context exactly as *store.Scoped does. Keyed any other way, the isolation case in
// loai_tai_nguyen_ban_do_test.go would pass while proving nothing.
type danhMucGia struct {
	theo map[tenant.ID][]domain.LoaiTaiNguyenBanDo
	loi  error
	goi  int
}

func (d *danhMucGia) DanhSach(ctx context.Context) ([]domain.LoaiTaiNguyenBanDo, error) {
	d.goi++
	if d.loi != nil {
		return nil, d.loi
	}
	return d.theo[tenant.MustFrom(ctx)], nil
}

// danhMucMau gives commune A three groups and commune B one with a DIFFERENT name. Two communes
// whose catalogues were spelled the same could not show a leak.
//
// THE CODES ARE DELIBERATELY NOT ANY OF THE SPECIFICATION'S. docs/ui-ux/10-ban-do-kinh-te-so.md
// contradicts itself about this list — eleven groups at :37 against eight at :53, spelled
// `enterprise` in one place and `doanh-nghiep` in the other — and the table ships EMPTY until the
// user settles both the count and the spelling (migration 0003, REASON TWO). A fixture spelling
// either version would read like a side being taken.
//
// The order is the one the store returns (ORDER BY thu_tu, ma) and is deliberately NOT the
// alphabetical order of the codes: `mau-ba` sorts first alphabetically and last by `thu_tu`, so any
// re-sort in the handler turns the order test red.
//
// The third row is `DangDung: false` — a group taken out of use. It is part of the list on purpose;
// see the note on domain.LoaiTaiNguyenBanDo.DangDung.
func danhMucMau() *danhMucGia {
	return &danhMucGia{theo: map[tenant.ID][]domain.LoaiTaiNguyenBanDo{
		xaA: {
			{ID: "ltn-001", Ma: "mau-mot", Nhan: "Nhóm mẫu một", ThuTu: 1, LaMacDinh: true, DangDung: true},
			{ID: "ltn-002", Ma: "mau-hai", Nhan: "Nhóm mẫu hai", ThuTu: 2, DangDung: true},
			{ID: "ltn-003", Ma: "mau-ba", Nhan: "Nhóm mẫu ba", ThuTu: 3, DangDung: false},
		},
		xaB: {
			{ID: "ltn-b-001", Ma: "mau-cua-xa-b", Nhan: "Nhóm mẫu của xã B", ThuTu: 1, DangDung: true},
		},
	}}
}

// checkerGia grants NOTHING, in any commune, and counts every call.
//
// Both halves are the assertion. The route is AnyAuthenticated, so a checker that denies everything
// must not change the outcome — and the count proves the route does not consult it at all rather
// than consulting it and ignoring the answer.
type checkerGia struct{ goi int }

func (c *checkerGia) Allows(context.Context, authz.Principal, authz.Perm) bool {
	c.goi++
	return false
}

// --- harness ----------------------------------------------------------------------------

type mayChu struct {
	h       http.Handler
	d       Deps
	thuMuc  thuMucGia
	danhMuc *danhMucGia
	checker *checkerGia

	// phien is the principal this harness pretends an authentication edge resolved. nil means no
	// session at all — the 401 case. It is read PER REQUEST, so a test changes it without rebuilding
	// the chain.
	phien *authz.Principal
}

func dungMayChu(t *testing.T) *mayChu {
	t.Helper()

	danhMuc := danhMucMau()
	checker := &checkerGia{}

	m := &mayChu{
		thuMuc:  thuMucMau(),
		danhMuc: danhMuc,
		checker: checker,
		d: Deps{
			Checker:       checker,
			LoaiTaiNguyen: danhMuc,
			// The write use case, so Register accepts the Deps. NOTHING IN THIS FILE CALLS IT: the
			// three write routes have their own four-case suite in
			// loai_tai_nguyen_ban_do_ghi_test.go, with a fake that records the commune and the
			// acting person. Register refuses a nil dependency at construction, so it has to be
			// present — and a fake that is never invoked cannot answer anything wrongly.
			GhiLoaiTaiNguyen: &ghiDanhMucGia{},
			Log:              slog.New(slog.NewTextHandler(io.Discard, nil)),
		},
	}

	mux := http.NewServeMux()
	Register(mux, m.d)

	// The real edge chain minus the authentication middleware this service does not have. The order
	// of what IS here is the real one and is not negotiable: strip client-supplied commune headers,
	// recover panics into a traceable 500, resolve Host -> commune (404 when it resolves to none).
	var h http.Handler = mux
	h = m.xacThucGia(h)
	h = httpx.TenantMiddleware(m.thuMuc)(h)
	h = httpx.Recover(func(context.Context) string { return "test-trace" })(h)
	h = httpx.StripTenantHeaders(h)
	m.h = h
	return m
}

func (m *mayChu) xacThucGia(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if m.phien != nil {
			r = r.WithContext(authz.Into(r.Context(), *m.phien))
		}
		next.ServeHTTP(w, r)
	})
}

// daDangNhap makes the next requests carry a session issued BY THE NAMED COMMUNE. The commune is
// part of the principal, never of the request, which is what makes "signed in at A, calling B"
// expressible at all.
func (m *mayChu) daDangNhap(xa tenant.ID) {
	m.phien = &authz.Principal{ID: idCanBo, Kind: "staff", TenantID: xa}
}

func (m *mayChu) goi(t *testing.T, method, host, path string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, "https://"+host+path, nil)
	r.Host = host
	r.RemoteAddr = "10.0.0.7:51000"
	w := httptest.NewRecorder()
	m.h.ServeHTTP(w, r)
	return w
}

func doiMa(t *testing.T, w *httptest.ResponseRecorder, muon int) {
	t.Helper()
	if w.Code != muon {
		t.Fatalf("mã trạng thái = %d, muốn %d — thân: %s", w.Code, muon, w.Body.String())
	}
}

func loiTra(t *testing.T, w *httptest.ResponseRecorder) httpx.Error {
	t.Helper()
	var e httpx.Error
	if err := json.Unmarshal(w.Body.Bytes(), &e); err != nil {
		t.Fatalf("thân lỗi không phải JSON: %q", w.Body.String())
	}
	return e
}

// --- the edge -----------------------------------------------------------------------------

func TestHostKhongThuocXaNaoTra404(t *testing.T) {
	// Rule 1, invariant 3: cannot resolve the commune -> 404, never a default commune. 404 also
	// reveals nothing about which communes exist on the platform.
	m := dungMayChu(t)
	m.daDangNhap(xaA)

	doiMa(t, m.goi(t, "GET", "khong-ai-biet.example.gov.vn", duongLoaiTaiNguyen), http.StatusNotFound)
	if m.danhMuc.goi != 0 {
		t.Error("tên miền không thuộc xã nào mà vẫn đọc danh mục")
	}
}

func TestDepsThieuKhoThiPanicLucDung(t *testing.T) {
	// Refusing incomplete wiring at CONSTRUCTION, not at request time. Without this, the route is
	// mounted over a nil interface and every caller gets a recovered 500 — a broken screen whose
	// cause is nowhere in the message.
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("Register không panic khi thiếu kho danh mục")
		}
	}()
	Register(http.NewServeMux(), Deps{})
}
