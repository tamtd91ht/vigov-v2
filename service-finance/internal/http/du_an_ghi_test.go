package http

// The four permission cases of the investment project WRITE routes, plus what each one is allowed to
// refuse (docs/ui-ux/06-giai-ngan.md §9, §8).
//
// RULE 5, INVARIANT 7 ASKS FOR FOUR CASES ON EVERY NEW ENDPOINT, and they are asserted on ALL THREE
// routes rather than on whichever one was convenient:
//
//	401  no session
//	403  a signed-in account WITHOUT the key the route declares
//	403  the RIGHT key, but held in ANOTHER commune       (rule 1 · rule 5, invariant 3)
//	2xx  the right key in the right commune
//
// THE THIRD CASE IS THE ONE THAT IS EASY TO FAKE AND THE ONE THAT MATTERS MOST. It needs a checker
// whose grants are KEYED BY COMMUNE — a flat permission set cannot express "holds budget.update, but
// in commune B" — which is why this file uses checkerDanhMucGia and its own harness rather than
// routes_test.go's.
//
// THE PERMISSION SPLIT UNDER TEST IS THE SPECIFICATION'S OWN (06-giai-ngan.md:202): `budget.update`
// enters and corrects, and the REMOVAL takes `budget.confirm` — a choice routes.go argues in full,
// because the specification assigns the removal no key at all.

import (
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
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-finance/internal/app"
	"github.com/vihat/vigov/service-finance/internal/domain"
	fistore "github.com/vihat/vigov/service-finance/internal/store"

	"context"
)

// --- fakes ---------------------------------------------------------------------------------------

// ghiDuAnGia stands in for the write use case, RECORDING THE COMMUNE IT WAS CALLED IN — read from the
// context exactly as *store.Scoped reads it. A fake that ignored the commune would let a
// wrong-commune case pass while proving nothing.
type ghiDuAnGia struct {
	ra     domain.DuAn
	phanBo []domain.PhanBoNguonVon
	loi    error

	themGoi, suaGoi, xoaGoi int
	xaCuoi                  tenant.ID
	nguoiCuoi               audit.Actor
	themCuoi                app.YeuCauThemDuAn
	suaCuoi                 app.YeuCauSuaDuAn
	idCuoi, lyDoCuoi        string
}

func (g *ghiDuAnGia) ghiNhan(ctx context.Context, nguoi audit.Actor) {
	g.xaCuoi = tenant.MustFrom(ctx)
	g.nguoiCuoi = nguoi
}

func (g *ghiDuAnGia) Them(ctx context.Context, yc app.YeuCauThemDuAn,
	nguoi audit.Actor) (app.KetQuaThemDuAn, error) {
	g.themGoi++
	g.themCuoi = yc
	g.ghiNhan(ctx, nguoi)
	if g.loi != nil {
		return app.KetQuaThemDuAn{}, g.loi
	}
	return app.KetQuaThemDuAn{DuAn: g.ra, PhanBo: g.phanBo}, nil
}

func (g *ghiDuAnGia) Sua(ctx context.Context, id string, yc app.YeuCauSuaDuAn,
	nguoi audit.Actor) (domain.DuAn, error) {
	g.suaGoi++
	g.idCuoi, g.suaCuoi = id, yc
	g.ghiNhan(ctx, nguoi)
	if g.loi != nil {
		return domain.DuAn{}, g.loi
	}
	return g.ra, nil
}

func (g *ghiDuAnGia) Xoa(ctx context.Context, id, lyDo string, nguoi audit.Actor) error {
	g.xoaGoi++
	g.idCuoi, g.lyDoCuoi = id, lyDo
	g.ghiNhan(ctx, nguoi)
	return g.loi
}

func (g *ghiDuAnGia) tongGoi() int { return g.themGoi + g.suaGoi + g.xoaGoi }

// --- harness -------------------------------------------------------------------------------------

type mayChuDuAnGhi struct {
	h       http.Handler
	ghi     *ghiDuAnGia
	checker *checkerDanhMucGia
}

// dungMayChuDuAnGhi mounts the REAL routes through Register, behind the REAL edge chain in the real
// order — including idem.Middleware, which sits INSIDE TenantMiddleware because the idempotency key
// is prefixed with the commune (rule 1, invariant 7).
func dungMayChuDuAnGhi(t *testing.T) *mayChuDuAnGhi {
	t.Helper()

	ghi := &ghiDuAnGia{ra: domain.DuAn{
		ID:              "01JDUANMOI0000000000000000",
		Ma:              "DA-2026-be-tong-hoa-duong-ngo-xom",
		Nam:             2026,
		HangMucID:       "hm-chuyen-tiep",
		Ten:             "Bê tông hoá đường ngõ xóm tổ 6",
		KeHoachVonNam:   100_000_000,
		ThoiHanGiaiNgan: time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC),
	}}
	checker := &checkerDanhMucGia{}
	im := slog.New(slog.NewTextHandler(io.Discard, nil))

	mux := http.NewServeMux()
	Register(mux, Deps{
		Checker:    checker,
		HangMuc:    hangMucMau(),
		GhiHangMuc: &ghiDanhMucGia{},
		DuAn:       duAnMau(),
		GhiDuAn:    ghi,
		// Present so Register accepts the Deps; never called from this file.
		GhiChungTu:  &ghiChungTuGia{},
		Nguong:      nguongMau(),
		NganSach:    &nganSachGia{},
		GhiNganSach: &ghiNganSachGia{},
		Nay:         func() time.Time { return lucDaQua7096 },
		Log:         im,
	})

	var h http.Handler = mux
	h = chuTheGhi(h)
	h = idem.Middleware(moiKhoIdem(), im)(h)
	h = httpx.TenantMiddleware(thuMucMau())(h)
	h = httpx.Recover(func(context.Context) string { return "test-trace" })(h)
	h = httpx.StripTenantHeaders(h)

	return &mayChuDuAnGhi{h: h, ghi: ghi, checker: checker}
}

func (m *mayChuDuAnGhi) capQuyen(xa tenant.ID, perm ...authz.Perm) {
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

// goi ALWAYS SENDS AN Idempotency-Key. POST /api/v1/investment-projects declares idem.Required, which
// refuses a request without the header BEFORE it reaches the handler — so a harness that omitted it
// would turn every POST assertion into an assertion about the header.
func (m *mayChuDuAnGhi) goi(t *testing.T, method, host, path string,
	p *authz.Principal, than string) *httptest.ResponseRecorder {
	t.Helper()
	var body io.Reader
	if than != "" {
		body = strings.NewReader(than)
	}
	r := httptest.NewRequest(method, "https://"+host+path, body)
	r.Host = host
	r.RemoteAddr = "10.0.0.7:51000"
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set(idem.Header, "01JIDEMPOTENCYKEYDUAN000")
	if p != nil {
		r = r.WithContext(context.WithValue(r.Context(), khoaChuTheGhi{}, *p))
	}
	w := httptest.NewRecorder()
	m.h.ServeHTTP(w, r)
	return w
}

const (
	duongDuAn  = "/api/v1/investment-projects"
	idDuAnMau  = "01JDUANDANGCO00000000000"
	thanThemDA = `{"code":"DA-2026-be-tong-hoa-duong-ngo-xom","year":2026,` +
		`"category_id":"hm-chuyen-tiep","name":"Bê tông hoá đường ngõ xóm tổ 6",` +
		`"planned_amount":100000000}`
	thanSuaDA = `{"name":"Bê tông hoá đường ngõ xóm tổ 6 (điều chỉnh)"}`
	thanXoaDA = `{"reason":"xã rút dự án khỏi kế hoạch vốn năm 2026"}`
)

func duongDuAnMot() string { return duongDuAn + "/" + idDuAnMau }

// motTuyenDuAn is one of the three write routes, with the key the specification assigns to it.
type motTuyenDuAn struct {
	ten    string
	method string
	duong  string
	than   string
	khoa   authz.Perm
	ok     int
	dem    func(g *ghiDuAnGia) int
}

// baTuyenGhiDuAn — the four permission cases are asserted on ALL THREE rather than on whichever one
// was convenient. A route that silently lost its declaration would otherwise be reachable by every
// signed-in account, and nothing would report it (rule 5).
func baTuyenGhiDuAn() []motTuyenDuAn {
	return []motTuyenDuAn{
		{
			ten: "thêm dự án", method: http.MethodPost, duong: duongDuAn, than: thanThemDA,
			khoa: "budget.update", ok: http.StatusCreated,
			dem: func(g *ghiDuAnGia) int { return g.themGoi },
		},
		{
			ten: "sửa dự án", method: http.MethodPatch, duong: duongDuAnMot(), than: thanSuaDA,
			khoa: "budget.update", ok: http.StatusOK,
			dem: func(g *ghiDuAnGia) int { return g.suaGoi },
		},
		{
			// `budget.confirm` AND NOT `budget.update`. The specification assigns the removal no key;
			// routes.go argues the choice, and this case is what makes the choice testable rather than
			// a sentence in a comment.
			ten: "xoá mềm dự án", method: http.MethodDelete, duong: duongDuAnMot(), than: thanXoaDA,
			khoa: "budget.confirm", ok: http.StatusNoContent,
			dem: func(g *ghiDuAnGia) int { return g.xoaGoi },
		},
	}
}

// --- the four cases ------------------------------------------------------------------------------

func TestGhiDuAnKhongCoPhienThi401(t *testing.T) {
	for _, tu := range baTuyenGhiDuAn() {
		t.Run(tu.ten, func(t *testing.T) {
			m := dungMayChuDuAnGhi(t)
			w := m.goi(t, tu.method, hostA, tu.duong, nil, tu.than)
			doiMa(t, w, http.StatusUnauthorized)
			if n := m.ghi.tongGoi(); n != 0 {
				t.Errorf("use case được gọi %d lần khi chưa đăng nhập, muốn 0", n)
			}
		})
	}
}

func TestGhiDuAnSaiQuyenThi403(t *testing.T) {
	for _, tu := range baTuyenGhiDuAn() {
		t.Run(tu.ten, func(t *testing.T) {
			m := dungMayChuDuAnGhi(t)
			// A real key of the same group, deliberately: `budget.read` is seeded beside the two keys
			// these routes declare, so this proves the route wants THAT key rather than merely "some
			// budget permission".
			m.capQuyen(xaA, "budget.read")
			w := m.goi(t, tu.method, hostA, tu.duong, canBoGhi(xaA), tu.than)
			doiMa(t, w, http.StatusForbidden)
			if n := m.ghi.tongGoi(); n != 0 {
				t.Errorf("use case được gọi %d lần khi sai quyền, muốn 0", n)
			}
		})
	}
}

// ⚠ THE CASE THAT DECIDES WHETHER THIS IS ONE SYSTEM OR A LEAK, so read what it actually sets up.
//
// The account belongs to commune B and is signed in AT COMMUNE B: nothing about the request is
// malformed, and authz's own commune comparison passes. What is wrong is the GRANT — the right to
// write projects was given in commune A. A checker that ignored the commune would answer yes here,
// and commune A's accountant would be entering projects into commune B's capital plan.
//
// That is rule 5, invariant 3 in one sentence: a permission missing its commune is cross-commune
// escalation, not a lesser bug. It answers 403 rather than 401 because the session is perfectly
// valid — it is the AUTHORITY that is absent.
func TestGhiDuAnDungQuyenNhungCapOXaKhacThi403(t *testing.T) {
	for _, tu := range baTuyenGhiDuAn() {
		t.Run(tu.ten, func(t *testing.T) {
			m := dungMayChuDuAnGhi(t)
			m.capQuyen(xaA, "budget.update", "budget.confirm")

			w := m.goi(t, tu.method, hostB, tu.duong, canBoGhi(xaB), tu.than)
			doiMa(t, w, http.StatusForbidden)
			if n := m.ghi.tongGoi(); n != 0 {
				t.Errorf("quyền cấp ở xã khác mà vẫn ghi được vào xã này (%d lần)", n)
			}
		})
	}
}

// THE OTHER SHAPE OF "wrong commune", and it is a DIFFERENT answer on purpose: a principal issued by
// commune B presented at commune A's domain. authz.RequirePermission compares the commune BEFORE the
// permission and answers 401 — rule 1, invariant 8, and the multi-tenant model's own table: "Token
// cấp cho xã A, gửi tới domain xã B → 401 + báo động". A browser does not send a cookie across hosts,
// so this is never an ordinary user error.
//
// ASSERTED SO THAT NOBODY "CORRECTS" IT TO 403 and turns a deliberate probe into an ordinary
// permission miss in the logs.
func TestGhiDuAnPhienCuaXaKhacThi401(t *testing.T) {
	for _, tu := range baTuyenGhiDuAn() {
		t.Run(tu.ten, func(t *testing.T) {
			m := dungMayChuDuAnGhi(t)
			m.capQuyen(xaA, tu.khoa)
			m.capQuyen(xaB, tu.khoa)

			w := m.goi(t, tu.method, hostA, tu.duong, canBoGhi(xaB), tu.than)
			doiMa(t, w, http.StatusUnauthorized)
			if n := m.ghi.tongGoi(); n != 0 {
				t.Errorf("phiên của xã khác mà vẫn ghi được (%d lần)", n)
			}
		})
	}
}

func TestGhiDuAnDungQuyenDungXaThiChay(t *testing.T) {
	for _, tu := range baTuyenGhiDuAn() {
		t.Run(tu.ten, func(t *testing.T) {
			m := dungMayChuDuAnGhi(t)
			m.capQuyen(xaA, tu.khoa)
			w := m.goi(t, tu.method, hostA, tu.duong, canBoGhi(xaA), tu.than)
			doiMa(t, w, tu.ok)
			if n := tu.dem(m.ghi); n != 1 {
				t.Errorf("use case được gọi %d lần, muốn 1", n)
			}
			// THE COMMUNE THE USE CASE RAN IN IS THE ONE THE HOST RESOLVED TO, never one a client named.
			// httpx.StripTenantHeaders removes any tenant header from outside; this asserts what
			// actually reached the store layer (rule 1, invariants 3 and 4).
			if m.ghi.xaCuoi != xaA {
				t.Errorf("use case chạy trong xã %q, muốn %q", m.ghi.xaCuoi, xaA)
			}
			// THE ACTOR IS THE STAFF BUSINESS CODE, never the internal id (rule 6, invariant 8).
			if m.ghi.nguoiCuoi.ID != maCanBoGhi {
				t.Errorf("chủ thể = %q, muốn mã cán bộ %q — không bao giờ id nội bộ",
					m.ghi.nguoiCuoi.ID, maCanBoGhi)
			}
			if m.ghi.nguoiCuoi.IP == "" {
				t.Error("vết không mang IP — luật 6 bất biến 2 đòi `từ IP nào`")
			}
		})
	}
}

// --- what the handler itself refuses --------------------------------------------------------------

// Rule 7, forbidden #4 and §13 rule 8, at the edge. Both are REFUSED rather than ignored: ignoring
// would leave the client believing the change landed while every screen still showed the old value.
func TestSuaDuAnTuChoiDoiMaVaDoiNamNganSach(t *testing.T) {
	for _, tc := range []struct {
		ten, than, manh string
	}{
		{"đổi mã", `{"code":"DA-2026-khac"}`, "mã đã cấp"},
		{"đổi năm ngân sách", `{"year":2027}`, "năm ngân sách khác"},
	} {
		t.Run(tc.ten, func(t *testing.T) {
			m := dungMayChuDuAnGhi(t)
			m.capQuyen(xaA, "budget.update")
			w := m.goi(t, http.MethodPatch, hostA, duongDuAnMot(), canBoGhi(xaA), tc.than)
			doiMa(t, w, http.StatusBadRequest)
			if m.ghi.suaGoi != 0 {
				t.Errorf("use case được gọi %d lần, muốn 0 — từ chối ở tầng biên", m.ghi.suaGoi)
			}
			if e := loiTra(t, w); !strings.Contains(e.Message, tc.manh) {
				t.Errorf("thông điệp = %q, muốn nhắc tới %q", e.Message, tc.manh)
			}
		})
	}
}

// §9's allocation list reaches the use case intact, one line per source, so nothing between the
// screen and the transaction can drop a source silently.
func TestThemDuAnChuyenPhanBoNguonVonXuongUseCase(t *testing.T) {
	m := dungMayChuDuAnGhi(t)
	m.capQuyen(xaA, "budget.update")

	than := `{"code":"DA-2026-moi","year":2026,"category_id":"hm-chuyen-tiep",` +
		`"name":"Dự án mới","planned_amount":100000000,` +
		`"funding_allocations":[{"funding_source_id":"nv-xa","amount":40000000},` +
		`{"funding_source_id":"nv-thanh-pho","amount":60000000}]}`

	w := m.goi(t, http.MethodPost, hostA, duongDuAn, canBoGhi(xaA), than)
	doiMa(t, w, http.StatusCreated)

	if n := len(m.ghi.themCuoi.PhanBo); n != 2 {
		t.Fatalf("use case nhận %d dòng phân bổ, muốn 2", n)
	}
	if got := m.ghi.themCuoi.PhanBo[0]; got.NguonVonID != "nv-xa" || got.SoTien != 40_000_000 {
		t.Errorf("dòng phân bổ đầu = %+v, muốn {nv-xa 40000000}", got)
	}
}

// §9's dynamic list is OPTIONAL and §11 names the state it produces (`Chưa gắn nguồn`). A route that
// required it would refuse exactly the project §9 calls ordinary.
func TestThemDuAnKhongKhaiNguonVonVan201(t *testing.T) {
	m := dungMayChuDuAnGhi(t)
	m.capQuyen(xaA, "budget.update")

	w := m.goi(t, http.MethodPost, hostA, duongDuAn, canBoGhi(xaA), thanThemDA)
	doiMa(t, w, http.StatusCreated)
	if n := len(m.ghi.themCuoi.PhanBo); n != 0 {
		t.Errorf("use case nhận %d dòng phân bổ, muốn 0", n)
	}

	var ra duAnGhiRa
	if err := json.Unmarshal(w.Body.Bytes(), &ra); err != nil {
		t.Fatalf("thân không phải JSON: %q", w.Body.String())
	}
	// ⚠ `funding_allocated_total` MUST BE PRESENT AND ZERO, not absent. A field that vanished at zero
	// would make a client unable to tell "nothing allocated" from "this server does not report it" —
	// and §11's `Chưa gắn nguồn` chip is derived from exactly this figure.
	if !strings.Contains(w.Body.String(), `"funding_allocated_total"`) {
		t.Errorf("thiếu `funding_allocated_total` khi chưa gắn nguồn: %q", w.Body.String())
	}
	if ra.FundingAllocatedTotal != 0 {
		t.Errorf("funding_allocated_total = %d, muốn 0", ra.FundingAllocatedTotal)
	}
}

// ⚠ §9 SAYS THE MISMATCH IS A WARNING, NOT A REFUSAL. The route returns the two raw figures —
// `planned_amount` and `funding_allocated_total` — and lets the screen make §9's warning and §11's
// chip out of them. This case is what stops a future reader from turning that sentence into a 400.
func TestThemDuAnTongNguonVuotKeHoachVan201VaTraVeHaiSo(t *testing.T) {
	m := dungMayChuDuAnGhi(t)
	m.capQuyen(xaA, "budget.update")
	m.ghi.phanBo = []domain.PhanBoNguonVon{
		{ID: "01JPB1", NguonVonID: "nv-xa", SoTien: 500_000_000},
	}

	than := `{"code":"DA-2026-moi","year":2026,"category_id":"hm-chuyen-tiep",` +
		`"name":"Dự án mới","planned_amount":100000000,` +
		`"funding_allocations":[{"funding_source_id":"nv-xa","amount":500000000}]}`

	w := m.goi(t, http.MethodPost, hostA, duongDuAn, canBoGhi(xaA), than)
	doiMa(t, w, http.StatusCreated)

	var ra duAnGhiRa
	if err := json.Unmarshal(w.Body.Bytes(), &ra); err != nil {
		t.Fatalf("thân không phải JSON: %q", w.Body.String())
	}
	if ra.FundingAllocatedTotal != 500_000_000 {
		t.Errorf("funding_allocated_total = %d, muốn 500000000", ra.FundingAllocatedTotal)
	}
	if ra.PlannedAmount != 100_000_000 {
		t.Errorf("planned_amount = %d, muốn 100000000", ra.PlannedAmount)
	}
}

// The refusals the use case raises, each mapped to the status a client can act on. 409 AND NOT 403 on
// every business refusal: the caller HOLDS the permission — what is refused is this operation on this
// record.
func TestGhiDuAnAnhXaLoiNghiepVuSangMaTrangThai(t *testing.T) {
	for _, tc := range []struct {
		ten    string
		loi    error
		method string
		duong  string
		than   string
		khoa   authz.Perm
		muon   int
		truong string
	}{
		{
			ten: "mã dự án đã cấp", loi: fistore.ErrMaDuAnDaTonTai,
			method: http.MethodPost, duong: duongDuAn, than: thanThemDA,
			khoa: "budget.update", muon: http.StatusConflict, truong: "code",
		},
		{
			ten: "hạng mục không có trong xã", loi: fistore.ErrKhongThayHangMuc,
			method: http.MethodPost, duong: duongDuAn, than: thanThemDA,
			khoa: "budget.update", muon: http.StatusNotFound, truong: "category_id",
		},
		{
			ten: "nguồn vốn sai năm ngân sách", loi: fistore.ErrKhongThayNguonVonPhanBo,
			method: http.MethodPost, duong: duongDuAn, than: thanThemDA,
			khoa: "budget.update", muon: http.StatusNotFound, truong: "funding_allocations",
		},
		{
			ten: "không có dự án", loi: fistore.ErrKhongThayDuAn,
			method: http.MethodPatch, duong: duongDuAnMot(), than: thanSuaDA,
			khoa: "budget.update", muon: http.StatusNotFound,
		},
		{
			// ⚠ THE STOP CONDITION, AT THE EDGE. Refusing is the only direction that writes nothing,
			// and the sentence has to name the way out or the screen looks broken: the project is
			// right there and the Delete button did nothing.
			ten: "dự án còn chứng từ", loi: fistore.ErrDuAnConChungTu,
			method: http.MethodDelete, duong: duongDuAnMot(), than: thanXoaDA,
			khoa: "budget.confirm", muon: http.StatusConflict,
		},
		{
			ten: "thiếu lý do xoá", loi: domain.ErrThieuLyDoXoaDuAn,
			method: http.MethodDelete, duong: duongDuAnMot(), than: `{"reason":""}`,
			khoa: "budget.confirm", muon: http.StatusBadRequest,
		},
		{
			ten: "thiếu mã dự án — hệ thống không tự sinh", loi: domain.ErrThieuMaDuAn,
			method: http.MethodPost, duong: duongDuAn, than: `{"year":2026}`,
			khoa: "budget.update", muon: http.StatusBadRequest,
		},
	} {
		t.Run(tc.ten, func(t *testing.T) {
			m := dungMayChuDuAnGhi(t)
			m.capQuyen(xaA, tc.khoa)
			m.ghi.loi = tc.loi

			w := m.goi(t, tc.method, hostA, tc.duong, canBoGhi(xaA), tc.than)
			doiMa(t, w, tc.muon)

			e := loiTra(t, w)
			if e.Message == "" {
				t.Error("thông điệp rỗng — người dùng không biết phải làm gì")
			}
			// THE FIELD AT FAULT IS NAMED IN THE SENTENCE, backtick-quoted.
			if tc.truong != "" && !strings.Contains(e.Message, "`"+tc.truong+"`") {
				t.Errorf("thông điệp = %q, muốn nêu trường `%s`", e.Message, tc.truong)
			}
			// ⚠ httpx.Error HAS NO `field` AT ALL — it carries `code`, `message` and `trace_id`
			// (core/httpx/edge.go:88-92) — so the FIFTH parameter of httpx.WriteError is `traceID`. A
			// field name passed there lands in `trace_id` and looks exactly like a real trace id,
			// sending an operator to search centralised logging for something that was never written.
			// The same defect shipped in service-petitions and was found on 2026-09-23.
			for _, xau := range []string{"code", "category_id", "funding_allocations", "id", "reason",
				"start_date", "completion_date", "disbursement_deadline", "year", "name"} {
				if e.TraceID == xau {
					t.Errorf("trace_id = %q — đó là TÊN TRƯỜNG chui vào chỗ traceID", e.TraceID)
				}
			}
		})
	}
}

// A malformed date must name the field that carried it, and must never reach the use case.
func TestThemDuAnNgaySaiDinhDangThi400VaNeuTenTruong(t *testing.T) {
	m := dungMayChuDuAnGhi(t)
	m.capQuyen(xaA, "budget.update")

	than := `{"code":"DA-2026-moi","year":2026,"category_id":"hm-chuyen-tiep",` +
		`"name":"Dự án mới","planned_amount":100000000,"start_date":"07/09/2026"}`

	w := m.goi(t, http.MethodPost, hostA, duongDuAn, canBoGhi(xaA), than)
	doiMa(t, w, http.StatusBadRequest)
	if m.ghi.themGoi != 0 {
		t.Errorf("use case được gọi %d lần, muốn 0", m.ghi.themGoi)
	}
	e := loiTra(t, w)
	if !strings.Contains(e.Message, "`start_date`") {
		t.Errorf("thông điệp = %q, muốn nêu trường `start_date`", e.Message)
	}
	// AND NOT IN `trace_id`, which is what the fifth parameter of httpx.WriteError really is.
	if e.TraceID == "start_date" {
		t.Errorf("trace_id = %q — tên trường chui vào chỗ traceID", e.TraceID)
	}
}

// The soft delete answers 204 with no body. Returning the project would invite a client to display
// one it has just taken off the screen.
func TestXoaDuAnTraVe204KhongThan(t *testing.T) {
	m := dungMayChuDuAnGhi(t)
	m.capQuyen(xaA, "budget.confirm")

	w := m.goi(t, http.MethodDelete, hostA, duongDuAnMot(), canBoGhi(xaA), thanXoaDA)
	doiMa(t, w, http.StatusNoContent)
	if w.Body.Len() != 0 {
		t.Errorf("thân = %q, muốn rỗng", w.Body.String())
	}
	// The reason reaches the use case, where it becomes `delete_reason` (rule 7, invariant 1).
	if m.ghi.lyDoCuoi == "" {
		t.Error("lý do xoá không tới được use case")
	}
	if m.ghi.idCuoi != idDuAnMau {
		t.Errorf("id = %q, muốn %q", m.ghi.idCuoi, idDuAnMau)
	}
}
