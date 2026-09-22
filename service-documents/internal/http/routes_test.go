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

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/httpx"
	"github.com/vihat/vigov/core/idem"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-documents/internal/app"
	"github.com/vihat/vigov/service-documents/internal/domain"
)

// The harness for this package's route tests. Nothing here is a PostgreSQL or a Redis: the
// properties under test are ordering, isolation and refusal, and a test that needs infrastructure
// is a test that stops being run.

// --- fixtures -----------------------------------------------------------------------------------

const (
	hostA = "xa-a.example.gov.vn"
	hostB = "xa-b.example.gov.vn"

	idCanBo = "nd-01JINTERNALIDCUACANBO"
)

var (
	xaA = tenant.ID("01JA" + strings.Repeat("A", 22))
	xaB = tenant.ID("01JB" + strings.Repeat("B", 22))
)

// canBoCua builds the principal the authentication edge would have built for a signed-in member of
// staff of that commune.
//
// Roles IS LEFT EMPTY, exactly as service-identity's XacThuc leaves it: permissions are read from
// the database on every request, never carried on the principal, so a role change takes effect on
// the next request rather than when the session ends.
func canBoCua(xa tenant.ID) *authz.Principal {
	return &authz.Principal{ID: idCanBo, Kind: "staff", TenantID: xa}
}

// --- fakes --------------------------------------------------------------------------------------

type thuMucGia map[string]tenant.Tenant

func (m thuMucGia) ByHost(_ context.Context, host string) (tenant.Tenant, bool) {
	t, ok := m[host]
	return t, ok
}

// thuMucMau is the platform registry for the two test communes. A NEW MAP PER CALL, so a test that
// rewrites one copy cannot change the other.
func thuMucMau() thuMucGia {
	return thuMucGia{
		hostA: {ID: xaA, Host: hostA, Name: "Xã Thăng Bình", Province: "Thành phố Đà Nẵng", Active: true},
		hostB: {ID: xaB, Host: hostB, Name: "Xã Bình Dương", Active: true},
	}
}

// loaiVanBanGia is the document-type catalogue, KEYED BY COMMUNE, reading the commune from the
// context exactly as *store.Scoped reads it. Keyed any other way — by nothing, or by a field set
// at construction — the isolation test below would pass while proving nothing at all.
type loaiVanBanGia struct {
	theo map[tenant.ID][]domain.LoaiVanBan
	loi  error
	goi  int
}

func (l *loaiVanBanGia) DanhSach(ctx context.Context) ([]domain.LoaiVanBan, error) {
	l.goi++
	if l.loi != nil {
		return nil, l.loi
	}
	return l.theo[tenant.MustFrom(ctx)], nil
}

// loaiVanBanMau gives commune A two types and commune B one with a DIFFERENT code and label. Two
// communes whose catalogues were named the same could not show a leak.
//
// The order is the one the store returns (ORDER BY thu_tu, nhan) and is deliberately NOT
// alphabetical by label ("Quyết định" before "Công văn"): any re-sort in the handler turns the
// order test red.
//
// Commune A's second row is `dang_dung: false` and its first is the default. A fixture where every
// row looked the same could not tell "the flag is read" from "the flag is always true".
func loaiVanBanMau() *loaiVanBanGia {
	return &loaiVanBanGia{theo: map[tenant.ID][]domain.LoaiVanBan{
		xaA: {
			{ID: "lvb-001", Ma: "quyet-dinh", Nhan: "Quyết định", DangDung: true, LaMacDinh: true},
			{ID: "lvb-002", Ma: "cong-van", Nhan: "Công văn", DangDung: false},
		},
		xaB: {
			{ID: "lvb-b-001", Ma: "to-trinh", Nhan: "Tờ trình xã B", DangDung: true},
		},
	}}
}

// checkerGia answers permissions and COUNTS the times it was asked.
//
// BOTH HALVES MATTER. The default set is empty, which is what makes "an account holding nothing
// still gets 200 on the read route" a real assertion; the count is what shows that route consults
// no permission at all, rather than happening to hold one.
//
// IT COMPARES THE COMMUNE, and that is not decoration. A checker that ignored the commune would
// grant one commune's roles inside another (rule 5, invariant 3), and the "right permission, wrong
// commune" case of invariant 7 would pass while proving nothing. The real one —
// service-identity/internal/store.Checker — reads the grants scoped to the commune in the context;
// this reproduces that one property and nothing else.
// THE GRANTS ARE KEYED BY COMMUNE, and that is the whole reason this fake is not a bool.
//
// Rule 5, invariant 7 asks for a "right permission, WRONG COMMUNE" case. With a flat permission set
// that case cannot be expressed at all: whoever holds the key holds it everywhere, so the test
// would assert something else and go green. Keyed by commune, a grant made in commune A is silent
// in commune B — which is exactly what cross-commune escalation looks like, and what
// service-identity/internal/store.Checker prevents by reading the grants scoped to the commune in
// the context.
type checkerGia struct {
	co    map[tenant.ID]map[authz.Perm]struct{}
	goi   int
	hoiGi []authz.Perm
}

func (c *checkerGia) Allows(ctx context.Context, p authz.Principal, perm authz.Perm) bool {
	c.goi++
	c.hoiGi = append(c.hoiGi, perm)
	// The commune comes from the CONTEXT, never from the principal — a checker reading p.TenantID
	// would answer about the commune the token claims rather than the one the request arrived at.
	if p.TenantID != tenant.MustFrom(ctx) {
		return false
	}
	_, ok := c.co[tenant.MustFrom(ctx)][perm]
	return ok
}

func (c *checkerGia) hoiKhoaCuoi() authz.Perm {
	if len(c.hoiGi) == 0 {
		return ""
	}
	return c.hoiGi[len(c.hoiGi)-1]
}

// ghiLoaiVanBanGia stands in for the write use case.
//
// IT RECORDS THE COMMUNE IT WAS CALLED IN, read from the context exactly as *store.Scoped reads it.
// A fake that ignored the commune would let a wrong-commune test pass while proving nothing — the
// same trap loaiVanBanGia avoids by keying its rows by commune.
type ghiLoaiVanBanGia struct {
	ra  domain.LoaiVanBan
	loi error

	themGoi, suaGoi, xoaGoi int
	xaCuoi                  tenant.ID
	nguoiCuoi               audit.Actor
	themCuoi                app.YeuCauThemLoaiVanBan
	suaCuoi                 app.YeuCauSuaLoaiVanBan
	idCuoi, lyDoCuoi        string
}

func (g *ghiLoaiVanBanGia) ghiNhan(ctx context.Context, nguoi audit.Actor) {
	g.xaCuoi = tenant.MustFrom(ctx)
	g.nguoiCuoi = nguoi
}

func (g *ghiLoaiVanBanGia) Them(ctx context.Context, yc app.YeuCauThemLoaiVanBan,
	nguoi audit.Actor) (domain.LoaiVanBan, error) {
	g.themGoi++
	g.themCuoi = yc
	g.ghiNhan(ctx, nguoi)
	if g.loi != nil {
		return domain.LoaiVanBan{}, g.loi
	}
	return g.ra, nil
}

func (g *ghiLoaiVanBanGia) Sua(ctx context.Context, id string, yc app.YeuCauSuaLoaiVanBan,
	nguoi audit.Actor) (domain.LoaiVanBan, error) {
	g.suaGoi++
	g.idCuoi, g.suaCuoi = id, yc
	g.ghiNhan(ctx, nguoi)
	if g.loi != nil {
		return domain.LoaiVanBan{}, g.loi
	}
	return g.ra, nil
}

func (g *ghiLoaiVanBanGia) Xoa(ctx context.Context, id, lyDo string, nguoi audit.Actor) error {
	g.xoaGoi++
	g.idCuoi, g.lyDoCuoi = id, lyDo
	g.ghiNhan(ctx, nguoi)
	return g.loi
}

func (g *ghiLoaiVanBanGia) tongGoi() int { return g.themGoi + g.suaGoi + g.xoaGoi }

// --- the edge -------------------------------------------------------------------------------------

type khoaChuTheThu struct{}

// chuTheThu INJECTS A PRINCIPAL DIRECTLY, in place of the real authentication edge.
//
// THE EDGE EXISTS NOW: core/staffauth.Middleware asks identity over gRPC, and cmd/server.dungBien
// mounts it — cmd/server/main_test.go drives that chain end to end, including the wrong-commune
// and identity-down cases. Keeping the injection here is deliberate rather than leftover: the
// properties this package owns (ordering, isolation, refusal) must stay testable without standing
// up a fake identity service, and a harness that needed one to assert a sort order is a harness
// that gets bypassed.
//
// What this stand-in reproduces is the ONE property the routes depend on: a principal carrying its
// OWN commune, put into the context INSIDE httpx.TenantMiddleware. It deliberately does NOT copy
// the commune resolved from Host onto the principal — that would quietly make every request
// self-consistent and the wrong-commune case below untestable.
func chuTheThu(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, ok := r.Context().Value(khoaChuTheThu{}).(authz.Principal)
		if !ok {
			next.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r.WithContext(authz.Into(r.Context(), p)))
	})
}

type mayChu struct {
	h       http.Handler
	d       Deps // kept so a test can rebuild the chain with ONE dependency swapped — see dungLai
	thuMuc  thuMucGia
	loai    *loaiVanBanGia
	ghi     *ghiLoaiVanBanGia
	checker *checkerGia
}

// dungMayChu builds the edge chain in the real order, with the stand-in above where the
// authentication middleware will sit.
func dungMayChu(t *testing.T) *mayChu {
	t.Helper()

	loai := loaiVanBanMau()
	checker := &checkerGia{}
	ghi := &ghiLoaiVanBanGia{ra: domain.LoaiVanBan{
		ID: "lvb-moi", Ma: "bao-cao", Nhan: "Báo cáo", ThuTu: 5,
		DangDung: true, Nguon: domain.NguonDonVi,
	}}
	m := &mayChu{
		thuMuc:  thuMucMau(),
		loai:    loai,
		ghi:     ghi,
		checker: checker,
		d: Deps{
			Checker:       checker,
			LoaiVanBan:    loai,
			GhiLoaiVanBan: ghi,
			Log:           slog.New(slog.NewTextHandler(io.Discard, nil)),
		},
	}
	m.dungLai(t, nil)
	return m
}

// capQuyen grants permissions INSIDE ONE COMMUNE. It rebuilds nothing: checkerGia reads the map on
// each call, so a test may grant mid-way through.
func (m *mayChu) capQuyen(xa tenant.ID, perm ...authz.Perm) {
	if m.checker.co == nil {
		m.checker.co = map[tenant.ID]map[authz.Perm]struct{}{}
	}
	if m.checker.co[xa] == nil {
		m.checker.co[xa] = map[authz.Perm]struct{}{}
	}
	for _, p := range perm {
		m.checker.co[xa][p] = struct{}{}
	}
}

func (m *mayChu) dungLai(t *testing.T, sua func(d *Deps)) {
	t.Helper()
	if sua != nil {
		sua(&m.d)
	}

	mux := http.NewServeMux()
	Register(mux, m.d)

	var h http.Handler = mux
	h = chuTheThu(h)
	// THE SAME ORDER cmd/server.dungBien BUILDS, with a NIL store — which is a valid deployment
	// (local development with no Redis) and is what makes the declared CheDoHong of each route the
	// thing under test rather than Redis behaviour. Inside TenantMiddleware because the idempotency
	// key is prefixed with the commune (rule 1, invariant 7).
	h = idem.Middleware(nil, slog.New(slog.NewTextHandler(io.Discard, nil)))(h)
	h = httpx.TenantMiddleware(m.thuMuc)(h)
	h = httpx.Recover(func(context.Context) string { return "test-trace" })(h)
	h = httpx.StripTenantHeaders(h)
	m.h = h
}

// goi issues one request with no body. A nil principal is a request that carries no session at all.
func (m *mayChu) goi(t *testing.T, method, host, path string, p *authz.Principal) *httptest.ResponseRecorder {
	t.Helper()
	return m.goiThan(t, method, host, path, p, "")
}

// goiThan issues one request carrying a JSON body.
//
// IT ALWAYS SENDS AN Idempotency-Key. POST declares idem.Required, which refuses a request without
// the header BEFORE it reaches the handler — so a harness that omitted it would turn every POST
// assertion into an assertion about the header. The one test that cares about the absent header
// builds its own request.
func (m *mayChu) goiThan(t *testing.T, method, host, path string, p *authz.Principal, than string) *httptest.ResponseRecorder {
	t.Helper()
	var body io.Reader
	if than != "" {
		body = strings.NewReader(than)
	}
	r := httptest.NewRequest(method, "https://"+host+path, body)
	r.Host = host
	r.RemoteAddr = "10.0.0.7:51000"
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set(idem.Header, "01JIDEMPOTENCYKEYCUATEST")
	if p != nil {
		r = r.WithContext(context.WithValue(r.Context(), khoaChuTheThu{}, *p))
	}
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

// --- Register refuses incomplete wiring -----------------------------------------------------------

func TestRegisterTuChoiDepsThieuKho(t *testing.T) {
	// AT CONSTRUCTION, NOT AT REQUEST TIME. A route mounted without its store would answer every
	// call with a panic turned into a 500, and the first person to see it would be a member of
	// staff trying to register a document — long after the deployment that caused it.
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("Register nhận Deps thiếu kho mà không panic — tuyến sẽ chết lúc có người gọi")
		}
	}()
	Register(http.NewServeMux(), Deps{})
}

func TestRegisterTuChoiDepsThieuCheckerVaUseCaseGhi(t *testing.T) {
	// THE TWO DEPENDENCIES THE WRITE ROUTES ADDED, each on its own, because a switch that refused
	// only the first missing one would let the second through unnoticed.
	//
	// A nil Checker is the dangerous one: authz.RequirePermission would meet a nil interface at
	// request time, which is a panic turned into a 500 on the very screen an administrator uses to
	// fix the catalogue — long after the deployment that caused it.
	for ten, d := range map[string]Deps{
		"thiếu use case ghi": {Checker: &checkerGia{}, LoaiVanBan: loaiVanBanMau()},
		"thiếu Checker":      {LoaiVanBan: loaiVanBanMau(), GhiLoaiVanBan: &ghiLoaiVanBanGia{}},
	} {
		t.Run(ten, func(t *testing.T) {
			defer func() {
				if r := recover(); r == nil {
					t.Fatalf("Register nhận Deps %s mà không panic", ten)
				}
			}()
			Register(http.NewServeMux(), d)
		})
	}
}
