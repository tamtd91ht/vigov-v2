package http

import (
	"context"
	"encoding/json"
	"errors"
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
	"github.com/vihat/vigov/service-petitions/internal/app"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	dmstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// --- harness ------------------------------------------------------------------------------------
//
// ITS OWN, and not routes_test.go's — see the note in the file header of the template this was
// generated from. In short: Deps differs per service, and the "right permission, wrong commune"
// case needs a checker whose grants are KEYED BY COMMUNE, which is the one property
// service-identity/internal/store.Checker has and a flat permission set cannot express.

const idCanBoGhiUuTien = "nd-01JINTERNALIDCUACANBO"

// maCanBoGhiUuTien is the BUSINESS CODE of that same person, and the two constants exist SEPARATELY because
// one principal carries both and they are read by different code for different reasons:
// authorisation joins on the internal id, the audit trail records the business code (rule 6,
// invariant 2). One value used for both is a test that cannot tell the two apart — which is
// exactly the state this file was in on 2026-09-22, while the route wrote the wrong one.
const maCanBoGhiUuTien = "CB-00123"

func canBoGhiUuTien(xa tenant.ID) *authz.Principal {
	return &authz.Principal{ID: idCanBoGhiUuTien, Ma: maCanBoGhiUuTien, Kind: "staff", TenantID: xa}
}

type khoaChuTheGhiUuTien struct{}

// chuTheGhiUuTien INJECTS A PRINCIPAL DIRECTLY, in place of the real authentication edge, and
// deliberately does NOT copy the commune resolved from Host onto it — that would make every request
// self-consistent and the wrong-commune cases untestable.
func chuTheGhiUuTien(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, ok := r.Context().Value(khoaChuTheGhiUuTien{}).(authz.Principal)
		if !ok {
			next.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r.WithContext(authz.Into(r.Context(), p)))
	})
}

// checkerDanhMucGiaUuTien keys its grants BY COMMUNE — see the note above.
type checkerDanhMucGiaUuTien struct {
	co    map[tenant.ID]map[authz.Perm]struct{}
	goi   int
	hoiGi []authz.Perm
}

func (c *checkerDanhMucGiaUuTien) Allows(ctx context.Context, p authz.Principal, perm authz.Perm) bool {
	c.goi++
	c.hoiGi = append(c.hoiGi, perm)
	// The commune comes from the CONTEXT, never from the principal: a checker reading p.TenantID
	// would answer about the commune the token claims rather than the one the request arrived at.
	if p.TenantID != tenant.MustFrom(ctx) {
		return false
	}
	_, ok := c.co[tenant.MustFrom(ctx)][perm]
	return ok
}

func (c *checkerDanhMucGiaUuTien) hoiKhoaCuoi() authz.Perm {
	if len(c.hoiGi) == 0 {
		return ""
	}
	return c.hoiGi[len(c.hoiGi)-1]
}

// ghiDanhMucGiaUuTien stands in for the write use case, RECORDING THE COMMUNE IT WAS CALLED IN — read
// from the context exactly as *store.Scoped reads it. A fake that ignored the commune would let a
// wrong-commune case pass while proving nothing.
type ghiDanhMucGiaUuTien struct {
	ra  domain.MucUuTienNhiemVu
	loi error

	themGoi, suaGoi, xoaGoi int
	xaCuoi                  tenant.ID
	nguoiCuoi               audit.Actor
	themCuoi                app.YeuCauThemMucUuTien
	suaCuoi                 app.YeuCauSuaMucUuTien
	idCuoi, lyDoCuoi        string
}

func (g *ghiDanhMucGiaUuTien) ghiNhan(ctx context.Context, nguoi audit.Actor) {
	g.xaCuoi = tenant.MustFrom(ctx)
	g.nguoiCuoi = nguoi
}

func (g *ghiDanhMucGiaUuTien) Them(ctx context.Context, yc app.YeuCauThemMucUuTien,
	nguoi audit.Actor) (domain.MucUuTienNhiemVu, error) {
	g.themGoi++
	g.themCuoi = yc
	g.ghiNhan(ctx, nguoi)
	if g.loi != nil {
		return domain.MucUuTienNhiemVu{}, g.loi
	}
	return g.ra, nil
}

func (g *ghiDanhMucGiaUuTien) Sua(ctx context.Context, id string, yc app.YeuCauSuaMucUuTien,
	nguoi audit.Actor) (domain.MucUuTienNhiemVu, error) {
	g.suaGoi++
	g.idCuoi, g.suaCuoi = id, yc
	g.ghiNhan(ctx, nguoi)
	if g.loi != nil {
		return domain.MucUuTienNhiemVu{}, g.loi
	}
	return g.ra, nil
}

func (g *ghiDanhMucGiaUuTien) Xoa(ctx context.Context, id, lyDo string, nguoi audit.Actor) error {
	g.xoaGoi++
	g.idCuoi, g.lyDoCuoi = id, lyDo
	g.ghiNhan(ctx, nguoi)
	return g.loi
}

func (g *ghiDanhMucGiaUuTien) tongGoi() int { return g.themGoi + g.suaGoi + g.xoaGoi }

type mayChuGhiUuTien struct {
	h       http.Handler
	ghi     *ghiDanhMucGiaUuTien
	checker *checkerDanhMucGiaUuTien
}

// dungMayChuGhiUuTien mounts the REAL routes through Register, behind the REAL edge chain in the real
// order — including idem.Middleware with a NIL store, which is a valid deployment (local
// development with no Redis) and is what makes each route's declared CheDoHong the thing under test
// rather than Redis behaviour. It sits INSIDE TenantMiddleware because the idempotency key is
// prefixed with the commune (rule 1, invariant 7).
func dungMayChuGhiUuTien(t *testing.T) *mayChuGhiUuTien {
	t.Helper()

	ghi := &ghiDanhMucGiaUuTien{ra: domain.MucUuTienNhiemVu{
		ID: "dm-moi", Ma: "bao-cao", Nhan: "Báo cáo", ThuTu: 5,
		DangDung: true, Nguon: domain.NguonDonVi,
	}}
	checker := &checkerDanhMucGiaUuTien{}
	im := slog.New(slog.NewTextHandler(io.Discard, nil))

	mux := http.NewServeMux()
	Register(mux, Deps{
		Checker:        checker,
		LoaiNhiemVu:    loaiNhiemVuMau(),
		MucUuTien:      mucUuTienMau(),
		GhiLoaiNhiemVu: &ghiDanhMucGia{},
		GhiMucUuTien:   ghi,
		Phieu:          phieuMau(),
		NhanLinhVuc:    nhanLinhVucMau(),
		Vet:            &vetXemGia{},
		// Present because Register refuses incomplete Deps at construction, and never called here —
		// see the same note in loai_nhiem_vu_ghi_test.go.
		DanhSachPhieu: danhSachTuPhieuMau(phieuMau()),
		XuLyPhieu:     &xuLyPhieuGia{},
		// The two TASK read routes. Register refuses a nil dependency at construction, and
		// nothing in this file calls them — their own suite is nhiem_vu_test.go.
		NhiemVu:         nhiemVuMau(),
		DanhSachNhiemVu: nhiemVuMau(),
		Log:             im,
	})

	var h http.Handler = mux
	h = chuTheGhiUuTien(h)
	h = idem.Middleware(nil, im)(h)
	h = httpx.TenantMiddleware(thuMucMau())(h)
	h = httpx.Recover(func(context.Context) string { return "test-trace" })(h)
	h = httpx.StripTenantHeaders(h)

	return &mayChuGhiUuTien{h: h, ghi: ghi, checker: checker}
}

// capQuyenUuTien grants permissions INSIDE ONE COMMUNE. It rebuilds nothing: the checker reads the map
// on each call, so a test may grant mid-way through.
func (m *mayChuGhiUuTien) capQuyen(xa tenant.ID, perm ...authz.Perm) {
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

func (m *mayChuGhiUuTien) goi(t *testing.T, method, host, path string, p *authz.Principal) *httptest.ResponseRecorder {
	t.Helper()
	return m.goiThan(t, method, host, path, p, "")
}

// goiThan ALWAYS SENDS AN Idempotency-Key. POST declares idem.Required, which refuses a request
// without the header BEFORE it reaches the handler — so a harness that omitted it would turn every
// POST assertion into an assertion about the header. The two cases that care about the header build
// their own request.
func (m *mayChuGhiUuTien) goiThan(t *testing.T, method, host, path string, p *authz.Principal, than string) *httptest.ResponseRecorder {
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
		r = r.WithContext(context.WithValue(r.Context(), khoaChuTheGhiUuTien{}, *p))
	}
	w := httptest.NewRecorder()
	m.h.ServeHTTP(w, r)
	return w
}

// WHAT THIS FILE IS FOR: the THREE WRITE ROUTES of the document-type catalogue — the first business
// write routes in this repository. Before them the whole REST contract was 21 GETs plus sign-in and
// sign-out, so every property asserted here is being asserted for the first time.
//
// FOUR THINGS, each of which fails silently if it stops holding:
//
//  1. the four cases of rule 5, invariant 7, on EVERY one of the three routes — and the third case
//     is the one that is easy to fake, see TestGhiMucUuTien_403DungQuyenSaiXa;
//  2. `source` and `tier` are REFUSED, not ignored. They decide which tier a row is in, and a
//     client that could set them could put its own row out of reach of every guard;
//  3. each refusal reaches the caller as the status that tells them what to do — 409 for a rule
//     about the row, 400 for a rule about the request, 404 for a row that is not there;
//  4. the commune and the acting person reach the use case, because they are what the audit entry
//     is filed under (rule 6, invariant 2).

const duongGhiMucUuTien = "/api/v1/task-priorities"

// QuyenDanhMucUuTien is the key the three write routes declare. It lives HERE, in the test, and not in
// routes.go — tools/apidoc refuses a key that is not a string literal at the RequirePermission call
// site, so the routes spell it out three times (see the block above Register).
//
// A second spelling is a second place to drift, so the drift itself is what
// TestKhoaQuyenDungChuoiCuaBangQuyenUuTien asserts: it reads the key the ROUTE asked for and compares it
// with the literal, rather than with this constant.
const QuyenDanhMucUuTien authz.Perm = "admin.lookup"

func duongMotUuTien(id string) string { return duongGhiMucUuTien + "/" + id }

// thanThemUuTien is the smallest valid create body. Written out rather than marshalled from a struct so a
// test can send a field the Go type does not have — which is the whole point of case (2).
const thanThemUuTien = `{"code":"bao-cao","label":"Báo cáo","order":5}`

func docMotUuTien(t *testing.T, w *httptest.ResponseRecorder) mucUuTienRa {
	t.Helper()
	var ra mucUuTienRa
	if err := json.Unmarshal(w.Body.Bytes(), &ra); err != nil {
		t.Fatalf("thân không phải JSON: %q", w.Body.String())
	}
	return ra
}

// motTuyenUuTien is one of the three write routes, so the four permission cases are asserted on ALL of
// them rather than on whichever one was written first.
type motTuyenUuTien struct {
	ten    string
	method string
	duong  string
	than   string
	ok     int // the status a correctly-permitted call returns
}

func baTuyenGhiUuTien() []motTuyenUuTien {
	return []motTuyenUuTien{
		{"POST", http.MethodPost, duongGhiMucUuTien, thanThemUuTien, http.StatusCreated},
		{"PATCH", http.MethodPatch, duongMotUuTien("muu-001"), `{"label":"Công văn mới"}`, http.StatusOK},
		{"DELETE", http.MethodDelete, duongMotUuTien("muu-001"), `{"reason":"gộp vào loại khác"}`, http.StatusNoContent},
	}
}

// --- the permission key itself ------------------------------------------------------------------------

func TestKhoaQuyenDungChuoiCuaBangQuyenUuTien_MucUuTien(t *testing.T) {
	// THE KEY EACH ROUTE ACTUALLY ASKS FOR, COMPARED AGAINST A LITERAL — and that is the whole point.
	//
	// Every other assertion in this file grants QuyenDanhMucUuTien and then expects that same value to be
	// accepted, so changing the key to ANY string at all leaves them all green: the fake checker
	// grants whatever it was handed. That is not hypothetical. This repository carried three invented
	// keys — `finance.read`, `map.read`, `document.approve` — through several sessions with every
	// suite green, because a route holding a key the `quyen` table lacks answers 403 to EVERY
	// account, forever, and nothing says so (rule 5, invariant 3c).
	//
	// `admin.lookup` is seeded at service-identity/migrations/0001_init.sql:274 and listed as
	// "Quản lý danh mục" at docs/ui-ux/14-cau-hinh.md:109 — the permission matrix row for the very
	// `Danh mục` tab (§5) these routes serve.
	//
	// tools/check_quyen.py scans the WHOLE repository against that table on every `make check` and
	// is the guard a fixture cannot fool. This is the cheap half that turns red in `go test` too.
	for _, tc := range baTuyenGhiUuTien() {
		t.Run(tc.ten, func(t *testing.T) {
			m := dungMayChuGhiUuTien(t)
			m.goiThan(t, tc.method, hostA, tc.duong, canBoGhiUuTien(xaA), tc.than)

			if got := m.checker.hoiKhoaCuoi(); got != "admin.lookup" {
				t.Fatalf("tuyến hỏi khoá %q, muốn \"admin.lookup\" — một khoá bảng `quyen` không có "+
					"là một tuyến trả 403 với MỌI tài khoản, mãi mãi, và không phép kiểm nào đỏ", got)
			}
		})
	}
}

// --- (1) the four cases of rule 5, invariant 7 ------------------------------------------------------

func TestGhiMucUuTien_401KhongPhien(t *testing.T) {
	for _, tc := range baTuyenGhiUuTien() {
		t.Run(tc.ten, func(t *testing.T) {
			m := dungMayChuGhiUuTien(t)
			m.capQuyen(xaA, QuyenDanhMucUuTien) // granted, and still refused: there is nobody to grant it to

			doiMa(t, m.goiThan(t, tc.method, hostA, tc.duong, nil, tc.than), http.StatusUnauthorized)
			if m.ghi.tongGoi() != 0 {
				t.Error("chưa đăng nhập mà use case ghi đã chạy")
			}
		})
	}
}

func TestGhiMucUuTien_403SaiQuyen(t *testing.T) {
	// A signed-in account of the right commune holding a DIFFERENT permission. `document.create`
	// is deliberately a real key of this subsystem — the failure being guarded against is not "an
	// account with nothing", it is a clerk who may register documents being able to rewrite the
	// list documents are registered under.
	for _, tc := range baTuyenGhiUuTien() {
		t.Run(tc.ten, func(t *testing.T) {
			m := dungMayChuGhiUuTien(t)
			m.capQuyen(xaA, "document.create")

			w := m.goiThan(t, tc.method, hostA, tc.duong, canBoGhiUuTien(xaA), tc.than)
			doiMa(t, w, http.StatusForbidden)
			if m.ghi.tongGoi() != 0 {
				t.Error("sai quyền mà use case ghi vẫn chạy")
			}
			if m.checker.hoiKhoaCuoi() != QuyenDanhMucUuTien {
				t.Errorf("tuyến hỏi khoá %q, muốn %q — một khoá khác là một quyền khác",
					m.checker.hoiKhoaCuoi(), QuyenDanhMucUuTien)
			}
		})
	}
}

func TestGhiMucUuTien_403DungQuyenSaiXa(t *testing.T) {
	// THE CASE THAT IS EASIEST TO FAKE AND HARDEST TO GET RIGHT, so read what it actually sets up.
	//
	// The account belongs to commune B and is signed in AT COMMUNE B: nothing about the request is
	// malformed, and authz's own commune comparison passes. What is wrong is the GRANT — the right
	// to manage the catalogue was given in commune A. A checker that ignored the commune would
	// answer yes here, and commune A's administrator would be administering commune B.
	//
	// That is rule 5, invariant 3 in one sentence: a permission missing its commune is
	// cross-commune escalation, not a lesser bug. And it answers 403 rather than 401 because the
	// session is perfectly valid — it is the authority that is absent.
	for _, tc := range baTuyenGhiUuTien() {
		t.Run(tc.ten, func(t *testing.T) {
			m := dungMayChuGhiUuTien(t)
			m.capQuyen(xaA, QuyenDanhMucUuTien)

			w := m.goiThan(t, tc.method, hostB, tc.duong, canBoGhiUuTien(xaB), tc.than)
			doiMa(t, w, http.StatusForbidden)
			if m.ghi.tongGoi() != 0 {
				t.Error("quyền cấp ở xã khác mà vẫn ghi được vào xã này")
			}
		})
	}
}

func TestGhiMucUuTien_401PhienCuaXaKhac(t *testing.T) {
	// The other shape of "wrong commune": a principal issued by commune A presented at commune B's
	// domain. authz.RequirePermission compares the commune BEFORE the permission and answers 401,
	// not 403 — a browser does not send a cookie across hosts, so this is never an ordinary user
	// error. Asserted so that nobody "corrects" it to 403 and turns a deliberate probe into
	// something that reads like a permissions problem.
	for _, tc := range baTuyenGhiUuTien() {
		t.Run(tc.ten, func(t *testing.T) {
			m := dungMayChuGhiUuTien(t)
			m.capQuyen(xaA, QuyenDanhMucUuTien)
			m.capQuyen(xaB, QuyenDanhMucUuTien)

			doiMa(t, m.goiThan(t, tc.method, hostB, tc.duong, canBoGhiUuTien(xaA), tc.than),
				http.StatusUnauthorized)
			if m.ghi.tongGoi() != 0 {
				t.Error("phiên của xã khác mà vẫn ghi được")
			}
		})
	}
}

func TestGhiMucUuTien_DungQuyenDungXa(t *testing.T) {
	for _, tc := range baTuyenGhiUuTien() {
		t.Run(tc.ten, func(t *testing.T) {
			m := dungMayChuGhiUuTien(t)
			m.capQuyen(xaA, QuyenDanhMucUuTien)

			w := m.goiThan(t, tc.method, hostA, tc.duong, canBoGhiUuTien(xaA), tc.than)
			doiMa(t, w, tc.ok)
			if m.ghi.tongGoi() != 1 {
				t.Fatalf("use case ghi chạy %d lần, muốn 1", m.ghi.tongGoi())
			}
			// Rule 6, invariant 2: WHO, and IN WHICH COMMUNE. Both have to reach the layer that
			// writes the entry, or the trail cannot answer the only question it exists for.
			if m.ghi.xaCuoi != xaA {
				t.Errorf("use case chạy trong xã %q, muốn %q", m.ghi.xaCuoi, xaA)
			}
			// THE TRAIL CARRIES THE BUSINESS CODE, AND THE SECOND CHECK NAMES THE WRONG VALUE
			// OUTRIGHT. Asserting only "equals the code" would stay green the day somebody made
			// the two constants the same string, and it was green on 2026-09-22 while this route
			// wrote the INTERNAL id into `audit_log.actor_id` — a column nobody can then query,
			// because it held two kinds of identifier at once (rule 6, invariant 2).
			if m.ghi.nguoiCuoi.ID != maCanBoGhiUuTien || m.ghi.nguoiCuoi.Kind != "staff" {
				t.Errorf("chủ thể vết = %+v, muốn MÃ CÁN BỘ %q", m.ghi.nguoiCuoi, maCanBoGhiUuTien)
			}
			if m.ghi.nguoiCuoi.ID == idCanBoGhiUuTien {
				t.Errorf("vết mang ID NỘI BỘ %q — luật 6 bất biến 2 đòi mã nghiệp vụ", idCanBoGhiUuTien)
			}
			// The IP is taken from this process's own socket, never from X-Forwarded-For: rule 6
			// wants the address the request really arrived from.
			if m.ghi.nguoiCuoi.IP != "10.0.0.7" {
				t.Errorf("IP trong vết = %q, muốn 10.0.0.7", m.ghi.nguoiCuoi.IP)
			}
		})
	}
}

// --- (2) `source` and `tier` are refused, not ignored -------------------------------------------------

func TestGhiMucUuTienTuChoiNguonTuClient(t *testing.T) {
	// THE ONE THAT MUST NEVER GO GREEN BY ACCIDENT. `nguon` decides which tier a row is in, and the
	// migration says what a writable `nguon` would cost: "every guard below could be stepped around
	// by setting nguon = 'don-vi' first". The store writes it as a LITERAL, which is what actually
	// makes it impossible; this route answers 400 so a client learns it rather than watching the
	// field disappear.
	//
	// BOTH DIRECTIONS ARE SENT. `he-thong` is the one that would buy something (a row nobody can
	// delete); `don-vi` is the one that looks harmless and is refused just as firmly, because the
	// rule is "not from the client", not "not that value".
	for ten, than := range map[string]string{
		"POST nguon he-thong":  `{"code":"bao-cao","label":"Báo cáo","source":"he-thong"}`,
		"POST nguon don-vi":    `{"code":"bao-cao","label":"Báo cáo","source":"don-vi"}`,
		"POST tang 3":          `{"code":"bao-cao","label":"Báo cáo","tier":3}`,
		"PATCH nguon he-thong": `{"label":"Báo cáo","source":"he-thong"}`,
		"PATCH tang 3":         `{"label":"Báo cáo","tier":3}`,
	} {
		t.Run(ten, func(t *testing.T) {
			m := dungMayChuGhiUuTien(t)
			m.capQuyen(xaA, QuyenDanhMucUuTien)

			method, duong := http.MethodPost, duongGhiMucUuTien
			if strings.HasPrefix(ten, "PATCH") {
				method, duong = http.MethodPatch, duongMotUuTien("muu-001")
			}
			w := m.goiThan(t, method, hostA, duong, canBoGhiUuTien(xaA), than)
			doiMa(t, w, http.StatusBadRequest)
			if m.ghi.tongGoi() != 0 {
				t.Fatal("yêu cầu tự đặt nguồn mà vẫn đi tiếp tới use case")
			}
			// A 400 whose message does not say WHICH field was refused sends the caller to guess.
			if e := loiTra(t, w); !strings.Contains(e.Message, "source") {
				t.Errorf("thông điệp không nói rõ trường bị từ chối: %q", e.Message)
			}
		})
	}
}

func TestSuaMucUuTienTuChoiDoiMa(t *testing.T) {
	// An issued code is never renumbered (rule 7, invariant 3): document records hold it AS A VALUE
	// and nothing rewrites them. The trigger refuses the UPDATE and the store's statement does not
	// mention the column — this is the layer that says so in a sentence.
	m := dungMayChuGhiUuTien(t)
	m.capQuyen(xaA, QuyenDanhMucUuTien)

	w := m.goiThan(t, http.MethodPatch, hostA, duongMotUuTien("muu-001"), canBoGhiUuTien(xaA),
		`{"label":"Công văn","code":"cong-van-2"}`)
	doiMa(t, w, http.StatusBadRequest)
	if m.ghi.suaGoi != 0 {
		t.Error("yêu cầu đổi mã mà vẫn đi tiếp tới use case")
	}
}

// --- (3) every refusal reaches the caller as the right status -----------------------------------------

func TestGhiMucUuTienAnhXaLoiSangMaTrangThai(t *testing.T) {
	// ONE TABLE FOR ALL OF THEM, because the mapping is the thing that drifts: a refusal answered
	// as 500 reads to an operator as a broken server rather than as a rule doing its job, and a
	// failure answered as 400 makes a client retry different input forever while nobody is told the
	// database is down.
	//
	// 409 AND NOT 403 for the tier refusals: the caller holds `admin.lookup` and IS allowed to
	// manage the catalogue. What is refused is this operation on THIS row.
	for ten, tc := range map[string]struct {
		loi  error
		ma   int
		code string
	}{
		"không tồn tại": {dmstore.ErrDanhMucKhongTonTai, http.StatusNotFound, "not_found"},
		"mã đã dùng":    {dmstore.ErrMaDaTonTai, http.StatusConflict, "code_taken"},
		"đầy trần":      {dmstore.ErrDanhMucDayTran, http.StatusConflict, "catalogue_full"},
		"mục hệ thống":  {domain.ErrKhongXoaDuocMucHeThong, http.StatusConflict, "system_row"},
		"mục rẽ nhánh":  {domain.ErrKhongTatDuocMucReNhanh, http.StatusConflict, "code_branch_row"},
		"thiếu lý do":   {domain.ErrThieuLyDoXoa, http.StatusBadRequest, "invalid_request"},
		"mã sai dạng":   {domain.ErrMaSaiDinhDang, http.StatusBadRequest, "invalid_request"},
		"kho hỏng":      {errors.New("cơ sở dữ liệu không phản hồi"), http.StatusInternalServerError, "internal"},
	} {
		t.Run(ten, func(t *testing.T) {
			m := dungMayChuGhiUuTien(t)
			m.capQuyen(xaA, QuyenDanhMucUuTien)
			m.ghi.loi = tc.loi

			w := m.goiThan(t, http.MethodPost, hostA, duongGhiMucUuTien, canBoGhiUuTien(xaA), thanThemUuTien)
			doiMa(t, w, tc.ma)
			e := loiTra(t, w)
			if e.Code != tc.code {
				t.Errorf("code = %q, muốn %q", e.Code, tc.code)
			}
			// Rule 3, forbidden #3: an internal failure never travels back to a client. The store's
			// own wording carries table names, statements and — on other tables — personal data.
			if strings.Contains(e.Message, "cơ sở dữ liệu không phản hồi") {
				t.Errorf("lỗi nội bộ lọt ra ngoài: %q", e.Message)
			}
		})
	}
}

func TestGhiMucUuTienThanKhongPhaiJSONLa400(t *testing.T) {
	m := dungMayChuGhiUuTien(t)
	m.capQuyen(xaA, QuyenDanhMucUuTien)

	w := m.goiThan(t, http.MethodPost, hostA, duongGhiMucUuTien, canBoGhiUuTien(xaA), `{"code":`)
	doiMa(t, w, http.StatusBadRequest)
	if m.ghi.tongGoi() != 0 {
		t.Error("thân hỏng mà vẫn gọi use case")
	}
}

// --- (4) what each route passes on, and what it answers -----------------------------------------------

func TestThemMucUuTienTraDongVuaTaoVaTraTang(t *testing.T) {
	m := dungMayChuGhiUuTien(t)
	m.capQuyen(xaA, QuyenDanhMucUuTien)

	w := m.goiThan(t, http.MethodPost, hostA, duongGhiMucUuTien, canBoGhiUuTien(xaA), thanThemUuTien)
	doiMa(t, w, http.StatusCreated)

	if m.ghi.themCuoi.Ma != "bao-cao" || m.ghi.themCuoi.Nhan != "Báo cáo" || m.ghi.themCuoi.ThuTu != 5 {
		t.Errorf("yêu cầu tới use case sai: %+v", m.ghi.themCuoi)
	}
	ra := docMotUuTien(t, w)
	if ra.Code != "bao-cao" || ra.Source != domain.NguonDonVi {
		t.Errorf("phản hồi sai: %+v", ra)
	}
	// TIER 1, AND THE SCREEN READS THIS TO DECIDE WHICH BUTTONS TO DRAW. A row the commune added is
	// the only kind it may later delete; answering anything else here offers a `Xoá` the database
	// will refuse, or hides one that would have worked.
	if ra.Tier != int(domain.TangDonVi) {
		t.Errorf("tầng trả về = %d, muốn %d", ra.Tier, domain.TangDonVi)
	}
}

func TestSuaMucUuTienChuyenNguyenConTroNil(t *testing.T) {
	// PATCH SEMANTICS, ASSERTED RATHER THAN ASSUMED: a field the body does not mention must arrive
	// at the use case as nil. Flattened to a value somewhere on the way, a dialog that edits only
	// the label would also move the row to position 0 and clear the commune's default — silently,
	// and on a screen that showed neither.
	m := dungMayChuGhiUuTien(t)
	m.capQuyen(xaA, QuyenDanhMucUuTien)

	w := m.goiThan(t, http.MethodPatch, hostA, duongMotUuTien("muu-001"), canBoGhiUuTien(xaA), `{"label":"Công văn mới"}`)
	doiMa(t, w, http.StatusOK)

	if m.ghi.idCuoi != "muu-001" {
		t.Errorf("id tới use case = %q, muốn muu-001", m.ghi.idCuoi)
	}
	yc := m.ghi.suaCuoi
	if yc.Nhan == nil || *yc.Nhan != "Công văn mới" {
		t.Errorf("nhãn không tới nơi: %+v", yc.Nhan)
	}
	if yc.ThuTu != nil || yc.DangDung != nil || yc.LaMacDinh != nil {
		t.Errorf("trường không được nhắc tới lại có giá trị: thu_tu=%v dang_dung=%v mac_dinh=%v",
			yc.ThuTu, yc.DangDung, yc.LaMacDinh)
	}
}

func TestSuaMucUuTienNhanGiaTriKhongLaGiaTriThat(t *testing.T) {
	// The other half of the same property: `false` and `0` MUST arrive, because they are the values
	// that turn a row off and move it to the front. A decoder that treated them as absent would make
	// the `Tắt` button do nothing at all.
	m := dungMayChuGhiUuTien(t)
	m.capQuyen(xaA, QuyenDanhMucUuTien)

	doiMa(t, m.goiThan(t, http.MethodPatch, hostA, duongMotUuTien("muu-001"), canBoGhiUuTien(xaA),
		`{"active":false,"order":0,"is_default":false}`), http.StatusOK)

	yc := m.ghi.suaCuoi
	if yc.DangDung == nil || *yc.DangDung {
		t.Errorf("active=false không tới nơi: %v", yc.DangDung)
	}
	if yc.ThuTu == nil || *yc.ThuTu != 0 {
		t.Errorf("order=0 không tới nơi: %v", yc.ThuTu)
	}
	if yc.LaMacDinh == nil || *yc.LaMacDinh {
		t.Errorf("is_default=false không tới nơi: %v", yc.LaMacDinh)
	}
}

func TestXoaMucUuTienChuyenLyDoVaTra204KhongThan(t *testing.T) {
	m := dungMayChuGhiUuTien(t)
	m.capQuyen(xaA, QuyenDanhMucUuTien)

	w := m.goiThan(t, http.MethodDelete, hostA, duongMotUuTien("muu-001"), canBoGhiUuTien(xaA),
		`{"reason":"gộp vào loại khác"}`)
	doiMa(t, w, http.StatusNoContent)

	if m.ghi.idCuoi != "muu-001" || m.ghi.lyDoCuoi != "gộp vào loại khác" {
		t.Errorf("id/lý do tới use case sai: %q / %q", m.ghi.idCuoi, m.ghi.lyDoCuoi)
	}
	// The row still exists — it carries deleted_at, deleted_by and delete_reason — but there is
	// nothing the caller can do with it, and returning it would invite a client to render a row it
	// has just taken off the screen.
	if w.Body.Len() != 0 {
		t.Errorf("204 mà vẫn có thân: %q", w.Body.String())
	}
}

// --- duplicate-request declaration --------------------------------------------------------------------

func TestThemMucUuTienDoiIdempotencyKey(t *testing.T) {
	// POST declares idem.Required, so a request with no key is refused BEFORE the handler runs and
	// nothing is written. Asserted here because the declaration is one call in a route statement:
	// delete it and every test above still passes, since they all send the header anyway.
	m := dungMayChuGhiUuTien(t)
	m.capQuyen(xaA, QuyenDanhMucUuTien)

	r := httptest.NewRequest(http.MethodPost, "https://"+hostA+duongGhiMucUuTien, strings.NewReader(thanThemUuTien))
	r.Host = hostA
	r.RemoteAddr = "10.0.0.7:51000"
	r = r.WithContext(ctxChuTheUuTien(r, canBoGhiUuTien(xaA)))
	w := httptest.NewRecorder()
	m.h.ServeHTTP(w, r)

	doiMa(t, w, http.StatusBadRequest)
	if e := loiTra(t, w); e.Code != "missing_idempotency_key" {
		t.Errorf("code = %q, muốn missing_idempotency_key", e.Code)
	}
	if m.ghi.themGoi != 0 {
		t.Error("thiếu Idempotency-Key mà use case vẫn chạy")
	}
}

func TestSuaVaXoaKhongDoiIdempotencyKey_MucUuTien(t *testing.T) {
	// The mirror of the test above, and it pins a DECISION rather than an implementation detail:
	// PATCH and DELETE declare idem.KhongCan, so a client without the header is served. Making them
	// Required would break every edit dialog in the admin web on the day somebody "tightened" it.
	m := dungMayChuGhiUuTien(t)
	m.capQuyen(xaA, QuyenDanhMucUuTien)

	for ten, tc := range map[string]struct {
		method, duong, than string
		ma                  int
	}{
		"PATCH":  {http.MethodPatch, duongMotUuTien("muu-001"), `{"label":"Công văn mới"}`, http.StatusOK},
		"DELETE": {http.MethodDelete, duongMotUuTien("muu-001"), `{"reason":"gộp"}`, http.StatusNoContent},
	} {
		t.Run(ten, func(t *testing.T) {
			r := httptest.NewRequest(tc.method, "https://"+hostA+tc.duong, strings.NewReader(tc.than))
			r.Host = hostA
			r.RemoteAddr = "10.0.0.7:51000"
			r = r.WithContext(ctxChuTheUuTien(r, canBoGhiUuTien(xaA)))
			w := httptest.NewRecorder()
			m.h.ServeHTTP(w, r)
			doiMa(t, w, tc.ma)
		})
	}
}

// --- the commune is still decided by the Host ---------------------------------------------------------

func TestGhiMucUuTienTenMienLa404TruocMoiThu(t *testing.T) {
	// A Host belonging to no commune is refused with 404 by httpx.TenantMiddleware, before the
	// permission declaration and before the use case. Asserted on a WRITE route because this is
	// where getting it wrong writes a row: a default commune here is rule 1, forbidden #1, and it
	// would file one commune's catalogue row — and its audit entry — in another commune's archive.
	m := dungMayChuGhiUuTien(t)
	m.capQuyen(xaA, QuyenDanhMucUuTien)

	w := m.goiThan(t, http.MethodPost, "khong-thuoc-xa-nao.example.vn", duongGhiMucUuTien,
		canBoGhiUuTien(xaA), thanThemUuTien)
	doiMa(t, w, http.StatusNotFound)
	if m.ghi.tongGoi() != 0 {
		t.Error("tên miền không thuộc xã nào mà vẫn ghi")
	}
}

// ctxChuThe is the one line goiThan does for a principal, for the two tests above that have to build
// their own request because they are about a HEADER goiThan always sets.
func ctxChuTheUuTien(r *http.Request, p *authz.Principal) context.Context {
	if p == nil {
		return r.Context()
	}
	return context.WithValue(r.Context(), khoaChuTheGhiUuTien{}, *p)
}
