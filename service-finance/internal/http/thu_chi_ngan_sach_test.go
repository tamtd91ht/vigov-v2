package http

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
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
)

// WHAT THIS FILE IS FOR: the EIGHT routes of the budget board — the screen whose figures go into a
// document the commune sends to a higher authority.
//
// SIX THINGS, each of which fails silently if it stops holding:
//
//  1. the four cases of rule 5, invariant 7, on EVERY one of the eight — and the third case is the
//     one that is easy to fake, see TestNganSach_403DungQuyenSaiXa;
//  2. each route asks for the key the SPECIFICATION's group assigns (§9 rule 7) and that key exists
//     in the `quyen` table. A key no migration seeds is a route that answers 403 to every account
//     forever while every test stays green (rule 5, invariant 3c);
//  3. WITH NO ROW STARRED THERE IS NO TOTAL, and a SENTENCE says so — not 0 (ADR 0035 §A);
//  4. `Dự toán TP giao` left blank makes the indicator DISAPPEAR with a sentence — not 0, not NaN;
//  5. `method`, `level` and `is_headline` are REFUSED on the write bodies, not ignored;
//  6. the commune and the acting person reach the use case, because they are what the audit entry is
//     filed under (rule 6, invariant 2).

// --- fakes ---------------------------------------------------------------------------------------

// nganSachGia is the read side, KEYED BY COMMUNE and by (year, kind), reading the commune from the
// context exactly as *store.Scoped reads it. A fake that ignored the commune would let the
// wrong-commune case pass while proving nothing.
type nganSachGia struct {
	theo map[tenant.ID]map[string]domain.BangDayDu
	loi  error
	goi  int
}

func khoaBang(nam int, loai domain.LoaiBang) string {
	return string(loai) + ":" + strconv.Itoa(nam)
}

func (n *nganSachGia) BangDayDu(ctx context.Context, nam int,
	loai domain.LoaiBang) (domain.BangDayDu, error) {

	n.goi++
	if n.loi != nil {
		return domain.BangDayDu{}, n.loi
	}
	d, co := n.theo[tenant.MustFrom(ctx)][khoaBang(nam, loai)]
	if !co {
		return domain.BangDayDu{}, fistore.ErrKhongThayBangNganSach
	}
	return d, nil
}

// ghiNganSachGia stands in for the write use case, RECORDING THE COMMUNE IT WAS CALLED IN.
type ghiNganSachGia struct {
	ra     domain.KhoanMucNganSach
	raBang domain.BangNganSach
	loi    error

	taoBangGoi, goBangGoi            int
	themGoi, suaGoi, goGoi, tongGoiN int

	xaCuoi    tenant.ID
	nguoiCuoi audit.Actor
	taoCuoi   app.YeuCauTaoBang
	themCuoi  app.YeuCauThemKhoanMuc
	suaCuoi   app.YeuCauSuaKhoanMuc
	idCuoi    string
	lyDoCuoi  string
}

func (g *ghiNganSachGia) ghiNhan(ctx context.Context, nguoi audit.Actor) {
	g.xaCuoi = tenant.MustFrom(ctx)
	g.nguoiCuoi = nguoi
}

func (g *ghiNganSachGia) TaoBang(ctx context.Context, yc app.YeuCauTaoBang,
	nguoi audit.Actor) (domain.BangNganSach, error) {
	g.taoBangGoi++
	g.taoCuoi = yc
	g.ghiNhan(ctx, nguoi)
	if g.loi != nil {
		return domain.BangNganSach{}, g.loi
	}
	return g.raBang, nil
}

func (g *ghiNganSachGia) GoBang(ctx context.Context, id, lyDo string, nguoi audit.Actor) error {
	g.goBangGoi++
	g.idCuoi, g.lyDoCuoi = id, lyDo
	g.ghiNhan(ctx, nguoi)
	return g.loi
}

func (g *ghiNganSachGia) ThemKhoanMuc(ctx context.Context, yc app.YeuCauThemKhoanMuc,
	nguoi audit.Actor) (domain.KhoanMucNganSach, error) {
	g.themGoi++
	g.themCuoi = yc
	g.ghiNhan(ctx, nguoi)
	if g.loi != nil {
		return domain.KhoanMucNganSach{}, g.loi
	}
	return g.ra, nil
}

func (g *ghiNganSachGia) SuaKhoanMuc(ctx context.Context, id string, yc app.YeuCauSuaKhoanMuc,
	nguoi audit.Actor) (domain.KhoanMucNganSach, error) {
	g.suaGoi++
	g.idCuoi, g.suaCuoi = id, yc
	g.ghiNhan(ctx, nguoi)
	if g.loi != nil {
		return domain.KhoanMucNganSach{}, g.loi
	}
	return g.ra, nil
}

func (g *ghiNganSachGia) GoKhoanMuc(ctx context.Context, id, lyDo string, nguoi audit.Actor) error {
	g.goGoi++
	g.idCuoi, g.lyDoCuoi = id, lyDo
	g.ghiNhan(ctx, nguoi)
	return g.loi
}

func (g *ghiNganSachGia) DatDongTong(ctx context.Context, id string,
	nguoi audit.Actor) (domain.KhoanMucNganSach, error) {
	g.tongGoiN++
	g.idCuoi = id
	g.ghiNhan(ctx, nguoi)
	if g.loi != nil {
		return domain.KhoanMucNganSach{}, g.loi
	}
	return g.ra, nil
}

func (g *ghiNganSachGia) tongGoi() int {
	return g.taoBangGoi + g.goBangGoi + g.themGoi + g.suaGoi + g.goGoi + g.tongGoiN
}

// --- fixtures ------------------------------------------------------------------------------------

// The same figures §3 prints, in đồng. See internal/domain/thu_chi_ngan_sach_test.go for why the
// specification's own numbers are used rather than invented ones.
func trieuDong(phanMuoi int64) domain.Dong { return domain.Dong(phanMuoi * 100_000) }

const (
	idBangChi = "01JBANGCHI0000000000000000"
	idBangThu = "01JBANGTHU0000000000000000"
	idDongChi = "01JKHOANMUCTONGCHI00000000"
	idDongThu = "01JKHOANMUCTONGTHU00000000"
)

// bangChiCuaXaA is the chi sheet: `Tổng số` MARKED, and `A. CHI NGÂN SÁCH NHÀ NƯỚC` beside it as a
// SIBLING carrying LARGER figures of its own. The sibling is the fixture's point — anything that
// summed or picked among top-level rows would produce a bigger number than the sheet's own total.
func bangChiCuaXaA() domain.BangDayDu {
	return domain.BangDayDu{
		Bang: domain.BangNganSach{
			ID: idBangChi, Ma: "NS-2026-CHI-01", Nam: 2026, Loai: domain.BangChi, Lan: 1,
			TieuDe:    "BÁO CÁO CHI NGÂN SÁCH NHÀ NƯỚC XÃ THĂNG BÌNH NĂM 2026",
			DonViTinh: "Triệu đồng",
			LuyKeDen:  time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC),
		},
		Cot: []domain.CotNganSach{
			{ID: "c-dt", BangID: idBangChi, Ten: "Dự toán năm", ThuTu: 1,
				Kieu: domain.CotSo, VaiTro: domain.VaiTroDuToanNam},
			{ID: "c-chi", BangID: idBangChi, Ten: "Chi ngân sách", ThuTu: 2,
				Kieu: domain.CotSo, VaiTro: domain.VaiTroChiNganSach},
			{ID: "c-ty", BangID: idBangChi, Ten: "So sánh TH/DT (%)", ThuTu: 3,
				Kieu: domain.CotPhanTram, CongThuc: "col_2 / col_1 * 100"},
		},
		KhoanMuc: []domain.KhoanMucNganSach{
			{ID: idDongChi, BangID: idBangChi, Ten: "Tổng số", ThuTu: 1,
				CachTinh: domain.TinhTay, LaDongTong: true},
			{ID: "k-a", BangID: idBangChi, TT: "A", Ten: "CHI NGÂN SÁCH NHÀ NƯỚC", ThuTu: 2,
				CachTinh: domain.TinhTay},
		},
		Gia: map[string]map[string]domain.Dong{
			idDongChi: {"c-dt": trieuDong(37_947_400), "c-chi": trieuDong(34_634_592)},
			"k-a":     {"c-dt": trieuDong(55_026_600), "c-chi": trieuDong(34_016_733)},
		},
	}
}

// bangThuCuaXaA is the thu sheet with all four columns of §3.2 — the two revenue ones differ by over
// a million units, which is what ADR 0035 #32 turns on.
func bangThuCuaXaA() domain.BangDayDu {
	return domain.BangDayDu{
		Bang: domain.BangNganSach{
			ID: idBangThu, Ma: "NS-2026-THU-01", Nam: 2026, Loai: domain.BangThu, Lan: 1,
			TieuDe: "THU NGÂN SÁCH XÃ THĂNG BÌNH NĂM 2026", DonViTinh: "Triệu đồng",
		},
		Cot: []domain.CotNganSach{
			{ID: "c-tp", BangID: idBangThu, Ten: "Dự toán 2026 TP giao", ThuTu: 1,
				Kieu: domain.CotSo, VaiTro: domain.VaiTroDuToanTPGiao},
			{ID: "c-xa", BangID: idBangThu, Ten: "Dự toán 2026 Xã giao", ThuTu: 2,
				Kieu: domain.CotSo, VaiTro: domain.VaiTroDuToanXaGiao},
			{ID: "c-nsnn", BangID: idBangThu, Ten: "Thu ngân sách NSNN", ThuTu: 3,
				Kieu: domain.CotSo, VaiTro: domain.VaiTroThuNSNN},
			{ID: "c-huong", BangID: idBangThu, Ten: "Thu ngân sách Thu xã hưởng", ThuTu: 4,
				Kieu: domain.CotSo, VaiTro: domain.VaiTroThuXaHuong},
		},
		KhoanMuc: []domain.KhoanMucNganSach{
			{ID: idDongThu, BangID: idBangThu, TT: "A",
				Ten: "TỔNG THU NỘI ĐỊA PHÁT SINH TRÊN ĐỊA BÀN", ThuTu: 1,
				CachTinh: domain.TinhTay, LaDongTong: true},
		},
		Gia: map[string]map[string]domain.Dong{
			idDongThu: {
				"c-tp":    trieuDong(39_930_100),
				"c-xa":    trieuDong(46_812_900),
				"c-nsnn":  trieuDong(43_167_643),
				"c-huong": trieuDong(33_008_005),
			},
		},
	}
}

// nganSachMau gives commune A both sheets of 2026 and commune B a chi sheet with a DIFFERENT title
// and different figures. Two communes whose sheets looked alike could not show a leak.
func nganSachMau() *nganSachGia {
	bDangChi := bangChiCuaXaA()
	bDangChi.Bang.TieuDe = "BÁO CÁO CHI NGÂN SÁCH NHÀ NƯỚC XÃ BÌNH DƯƠNG NĂM 2026"
	bDangChi.Bang.Ma = "NS-2026-CHI-01"
	bDangChi.Gia = map[string]map[string]domain.Dong{
		idDongChi: {"c-dt": trieuDong(1_000_000), "c-chi": trieuDong(500_000)},
	}
	return &nganSachGia{theo: map[tenant.ID]map[string]domain.BangDayDu{
		xaA: {
			khoaBang(2026, domain.BangChi): bangChiCuaXaA(),
			khoaBang(2026, domain.BangThu): bangThuCuaXaA(),
		},
		xaB: {khoaBang(2026, domain.BangChi): bDangChi},
	}}
}

// --- harness -------------------------------------------------------------------------------------
//
// ITS OWN, and not routes_test.go's, for the same reason the voucher suite has one: the "right
// permission, wrong commune" case needs a checker whose grants are KEYED BY COMMUNE, which a flat
// permission set cannot express.

type mayChuNganSach struct {
	h       http.Handler
	doc     *nganSachGia
	ghi     *ghiNganSachGia
	checker *checkerDanhMucGia
}

func dungMayChuNganSach(t *testing.T) *mayChuNganSach {
	t.Helper()

	doc := nganSachMau()
	ghi := &ghiNganSachGia{
		ra: domain.KhoanMucNganSach{
			ID: "01JKHOANMUCMOI000000000000", BangID: idBangChi, TT: "1.1",
			Ten: "Chi quốc phòng", ThuTu: 10, CachTinh: domain.TinhTay, Cap: 2,
		},
		raBang: domain.BangNganSach{
			ID: idBangChi, Ma: "NS-2026-CHI-01", Nam: 2026, Loai: domain.BangChi, Lan: 1,
			TieuDe: "BÁO CÁO CHI NGÂN SÁCH NHÀ NƯỚC XÃ THĂNG BÌNH NĂM 2026", DonViTinh: "Triệu đồng",
		},
	}
	checker := &checkerDanhMucGia{}
	im := slog.New(slog.NewTextHandler(io.Discard, nil))

	mux := http.NewServeMux()
	Register(mux, Deps{
		Checker:     checker,
		HangMuc:     hangMucMau(),
		GhiHangMuc:  &ghiDanhMucGia{},
		DuAn:        duAnMau(),
		GhiChungTu:  &ghiChungTuGia{},
		Nguong:      nguongMacDinh(),
		NganSach:    doc,
		GhiNganSach: ghi,
		Nay:         func() time.Time { return lucDaQua7096 },
		Log:         im,
	})

	var h http.Handler = mux
	h = chuTheGhi(h)
	h = idem.Middleware(moiKhoIdem(), im)(h)
	h = httpx.TenantMiddleware(thuMucMau())(h)
	h = httpx.Recover(func(context.Context) string { return "test-trace" })(h)
	h = httpx.StripTenantHeaders(h)

	return &mayChuNganSach{h: h, doc: doc, ghi: ghi, checker: checker}
}

func (m *mayChuNganSach) capQuyen(xa tenant.ID, perm ...authz.Perm) {
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

// goi ALWAYS SENDS AN Idempotency-Key. The two POST routes declare idem.Required, which refuses a
// request without the header BEFORE it reaches the handler — so a harness that omitted it would turn
// every POST assertion into an assertion about the header.
func (m *mayChuNganSach) goi(t *testing.T, method, host, path string,
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
	r.Header.Set(idem.Header, "01JIDEMPOTENCYKEYNGANSACH")
	if p != nil {
		r = r.WithContext(context.WithValue(r.Context(), khoaChuTheGhi{}, *p))
	}
	w := httptest.NewRecorder()
	m.h.ServeHTTP(w, r)
	return w
}

const (
	duongBang   = "/api/v1/budget-sheets"
	duongChiSo  = "/api/v1/budget-indicators"
	duongDong   = "/api/v1/budget-lines"
	thanTaoBang = `{"year":2027,"kind":"chi","title":"BÁO CÁO CHI NGÂN SÁCH 2027","unit":"Triệu đồng",` +
		`"columns":[{"name":"Dự toán năm","order":1,"type":"so","role":"du-toan-nam"},` +
		`{"name":"Chi ngân sách","order":2,"type":"so","role":"chi-ngan-sach"}]}`
	thanThemDong = `{"sheet_id":"01JBANGCHI0000000000000000","parent_id":"k-a","no":"1.1",` +
		`"name":"Chi quốc phòng","order":10}`
)

// motTuyenNganSach is one of the eight routes, with the key §9 rule 7's group assigns to it.
type motTuyenNganSach struct {
	ten    string
	method string
	duong  string
	than   string
	khoa   authz.Perm
	ok     int
	dem    func(m *mayChuNganSach) int
}

// tamTuyenNganSach — the four permission cases are asserted on ALL EIGHT rather than on whichever
// one was written first, and the `khoa` column is what makes the suite able to tell the three keys
// apart: a route that quietly asked for `budget.update` where this session assigned `budget.confirm`
// would let a clerk move the star that decides the commune's reported total.
func tamTuyenNganSach() []motTuyenNganSach {
	return []motTuyenNganSach{
		{"GET bang", http.MethodGet, duongBang + "?year=2026&kind=chi", "",
			"budget.read", http.StatusOK, func(m *mayChuNganSach) int { return m.doc.goi }},
		{"GET chi so", http.MethodGet, duongChiSo + "?year=2026", "",
			"budget.read", http.StatusOK, func(m *mayChuNganSach) int {
				// Two sheets are read for one request, so this counts requests rather than reads.
				if m.doc.goi > 0 {
					return 1
				}
				return 0
			}},
		{"POST bang", http.MethodPost, duongBang, thanTaoBang,
			"budget.update", http.StatusCreated, func(m *mayChuNganSach) int { return m.ghi.taoBangGoi }},
		{"DELETE bang", http.MethodDelete, duongBang + "/" + idBangChi, `{"reason":"nạp lại từ tệp đã sửa"}`,
			"budget.confirm", http.StatusNoContent, func(m *mayChuNganSach) int { return m.ghi.goBangGoi }},
		{"POST dong", http.MethodPost, duongDong, thanThemDong,
			"budget.update", http.StatusCreated, func(m *mayChuNganSach) int { return m.ghi.themGoi }},
		{"PATCH dong", http.MethodPatch, duongDong + "/" + idDongChi, `{"name":"Tổng số (đã sửa)"}`,
			"budget.update", http.StatusOK, func(m *mayChuNganSach) int { return m.ghi.suaGoi }},
		{"DELETE dong", http.MethodDelete, duongDong + "/" + idDongChi, `{"reason":"nhập trùng"}`,
			"budget.confirm", http.StatusNoContent, func(m *mayChuNganSach) int { return m.ghi.goGoi }},
		{"POST dong tong", http.MethodPost, duongDong + "/" + idDongChi + "/headline", "",
			"budget.confirm", http.StatusOK, func(m *mayChuNganSach) int { return m.ghi.tongGoiN }},
	}
}

func (m *mayChuNganSach) daChay() int { return m.doc.goi + m.ghi.tongGoi() }

// --- (2) the permission keys themselves ------------------------------------------------------------

func TestKhoaQuyenNganSachDungChuoiCuaBangQuyen(t *testing.T) {
	// THE KEY EACH ROUTE ACTUALLY ASKS FOR, COMPARED AGAINST A LITERAL — and that is the whole point.
	//
	// Every other assertion in this file grants the key and then expects it to be accepted, so
	// changing a route's key to ANY string leaves them all green: the fake checker grants whatever it
	// was handed. That is not hypothetical — this repository carried three invented keys through
	// several sessions with every suite green, because a route holding a key the `quyen` table lacks
	// answers 403 to EVERY account, forever, and nothing says so (rule 5, invariant 3c).
	//
	// All three keys are seeded at service-identity/migrations/0001_init.sql:282-284 and named for
	// this screen at docs/ui-ux/07-thu-chi-ngan-sach.md:236. tools/check_quyen.py scans the whole
	// repository against that table on every `make check` and is the guard a fixture cannot fool;
	// this is the cheap half that turns red in `go test` too.
	for _, tc := range tamTuyenNganSach() {
		t.Run(tc.ten, func(t *testing.T) {
			m := dungMayChuNganSach(t)
			m.goi(t, tc.method, hostA, tc.duong, canBoGhi(xaA), tc.than)

			if got := m.checker.hoiKhoaCuoi(); got != tc.khoa {
				t.Fatalf("tuyến hỏi khoá %q, muốn %q — một khoá bảng `quyen` không có là một tuyến "+
					"trả 403 với MỌI tài khoản, mãi mãi, và không phép kiểm nào đỏ", got, tc.khoa)
			}
		})
	}
}

// --- (1) the four cases of rule 5, invariant 7 -------------------------------------------------------

func TestNganSach_401KhongPhien(t *testing.T) {
	for _, tc := range tamTuyenNganSach() {
		t.Run(tc.ten, func(t *testing.T) {
			m := dungMayChuNganSach(t)
			m.capQuyen(xaA, "budget.read", "budget.update", "budget.confirm") // granted, still refused
			doiMa(t, m.goi(t, tc.method, hostA, tc.duong, nil, tc.than), http.StatusUnauthorized)
			if m.daChay() != 0 {
				t.Error("chưa đăng nhập mà kho/use case đã chạy")
			}
		})
	}
}

func TestNganSach_403SaiQuyen(t *testing.T) {
	// A signed-in account of the right commune holding a DIFFERENT permission. `admin.lookup` is
	// deliberately a REAL key of this service — the failure being guarded against is not "an account
	// with nothing", it is somebody whose job is the catalogue screen being able to move the figures
	// this commune reports upward.
	for _, tc := range tamTuyenNganSach() {
		t.Run(tc.ten, func(t *testing.T) {
			m := dungMayChuNganSach(t)
			m.capQuyen(xaA, "admin.lookup")

			doiMa(t, m.goi(t, tc.method, hostA, tc.duong, canBoGhi(xaA), tc.than),
				http.StatusForbidden)
			if m.daChay() != 0 {
				t.Error("sai quyền mà kho/use case vẫn chạy")
			}
		})
	}
}

func TestNganSach_403NguoiNhapKhongDongDuocDongTong(t *testing.T) {
	// THE SPLIT THIS SESSION DREW, ASSERTED AS A SEPARATE CASE because it is the one an ordinary
	// "grant the module's permissions" fixture would paper over: an accountant holding `budget.read`
	// and `budget.update` may create the sheet, add lines and type figures, and may NOT remove a
	// sheet, remove a line, or MOVE THE STAR. The star decides which row every summary cell and both
	// indicators are read from (ADR 0035 §A), which is the weight of a confirmation and not of data
	// entry.
	m := dungMayChuNganSach(t)
	m.capQuyen(xaA, "budget.read", "budget.update")

	for _, tc := range tamTuyenNganSach() {
		if tc.khoa != "budget.confirm" {
			continue
		}
		t.Run(tc.ten, func(t *testing.T) {
			doiMa(t, m.goi(t, tc.method, hostA, tc.duong, canBoGhi(xaA), tc.than),
				http.StatusForbidden)
		})
	}
	if m.ghi.tongGoi() != 0 {
		t.Errorf("use case ghi chạy %d lần với tài khoản chỉ có `budget.read` + `budget.update`",
			m.ghi.tongGoi())
	}
}

func TestNganSach_403DungQuyenSaiXa(t *testing.T) {
	// THE CASE THAT IS EASIEST TO FAKE AND HARDEST TO GET RIGHT, so read what it actually sets up.
	//
	// The account belongs to commune B and is signed in AT COMMUNE B: nothing about the request is
	// malformed, and authz's own commune comparison passes. What is wrong is the GRANT — the right to
	// read and write this board was given in commune A. A checker that ignored the commune would
	// answer yes here, and commune A's accountant would be editing commune B's budget.
	//
	// That is rule 5, invariant 3 in one sentence: a permission missing its commune is cross-commune
	// escalation, not a lesser bug. And it answers 403 rather than 401 because the session is
	// perfectly valid — it is the authority that is absent.
	for _, tc := range tamTuyenNganSach() {
		t.Run(tc.ten, func(t *testing.T) {
			m := dungMayChuNganSach(t)
			m.capQuyen(xaA, "budget.read", "budget.update", "budget.confirm")

			doiMa(t, m.goi(t, tc.method, hostB, tc.duong, canBoGhi(xaB), tc.than),
				http.StatusForbidden)
			if m.daChay() != 0 {
				t.Error("quyền cấp ở xã khác mà vẫn đọc/ghi được vào xã này")
			}
		})
	}
}

func TestNganSach_401PhienCuaXaKhac(t *testing.T) {
	// The other shape of "wrong commune": a principal issued by commune A presented at commune B's
	// domain. authz.RequirePermission compares the commune BEFORE the permission and answers 401,
	// not 403 — a browser does not send a cookie across hosts, so this is never an ordinary user
	// error.
	for _, tc := range tamTuyenNganSach() {
		t.Run(tc.ten, func(t *testing.T) {
			m := dungMayChuNganSach(t)
			m.capQuyen(xaA, "budget.read", "budget.update", "budget.confirm")
			m.capQuyen(xaB, "budget.read", "budget.update", "budget.confirm")

			doiMa(t, m.goi(t, tc.method, hostB, tc.duong, canBoGhi(xaA), tc.than),
				http.StatusUnauthorized)
			if m.daChay() != 0 {
				t.Error("phiên của xã khác mà vẫn đọc/ghi được")
			}
		})
	}
}

func TestNganSach_DungQuyenDungXa(t *testing.T) {
	for _, tc := range tamTuyenNganSach() {
		t.Run(tc.ten, func(t *testing.T) {
			m := dungMayChuNganSach(t)
			m.capQuyen(xaA, "budget.read", "budget.update", "budget.confirm")

			doiMa(t, m.goi(t, tc.method, hostA, tc.duong, canBoGhi(xaA), tc.than), tc.ok)
			if got := tc.dem(m); got != 1 {
				t.Fatalf("tầng dưới chạy %d lần, muốn 1", got)
			}
			if m.ghi.tongGoi() == 0 {
				return // a read route: the commune is asserted through the fixture below
			}
			// Rule 6, invariant 2: WHO, and IN WHICH COMMUNE. Both have to reach the layer that
			// writes the entry, or the trail cannot answer the only question it exists for.
			if m.ghi.xaCuoi != xaA {
				t.Errorf("use case chạy trong xã %q, muốn %q", m.ghi.xaCuoi, xaA)
			}
			// THE TRAIL CARRIES THE BUSINESS CODE, AND THE SECOND CHECK NAMES THE WRONG VALUE
			// OUTRIGHT. Asserting only "equals the code" would stay green the day somebody made the
			// two constants the same string.
			if m.ghi.nguoiCuoi.ID != maCanBoGhi || m.ghi.nguoiCuoi.ID == idNoiBo {
				t.Errorf("chủ thể vết = %q, muốn mã cán bộ %q (KHÔNG phải id nội bộ %q)",
					m.ghi.nguoiCuoi.ID, maCanBoGhi, idNoiBo)
			}
		})
	}
}

func TestDocBangCuaXaNaoThiRaBangCuaXaAy(t *testing.T) {
	// Rule 1 at the surface it is actually read from: two communes, one year, one kind, and the
	// titles differ. A store that ignored the commune would return commune A's sheet to commune B.
	m := dungMayChuNganSach(t)
	m.capQuyen(xaA, "budget.read")
	m.capQuyen(xaB, "budget.read")

	var a, b bangDayDuRa
	docJSON(t, m.goi(t, http.MethodGet, hostA, duongBang+"?year=2026&kind=chi", canBoGhi(xaA), ""), &a)
	docJSON(t, m.goi(t, http.MethodGet, hostB, duongBang+"?year=2026&kind=chi", canBoGhi(xaB), ""), &b)

	if a.Sheet.Title == b.Sheet.Title {
		t.Fatalf("hai xã nhận cùng một tiêu đề %q", a.Sheet.Title)
	}
	if !strings.Contains(a.Sheet.Title, "THĂNG BÌNH") || !strings.Contains(b.Sheet.Title, "BÌNH DƯƠNG") {
		t.Fatalf("bảng lẫn xã: A=%q B=%q", a.Sheet.Title, b.Sheet.Title)
	}
}

// --- (3) the marked row -------------------------------------------------------------------------------

func TestChuaDanhDauDongNaoThiKhongCoSoTongVaCoMotCau(t *testing.T) {
	// ADR 0035 §A at the surface the commune actually reads. THE ASSERTION IS ON THE WIRE FORMAT, not
	// on the domain: a handler that turned domain.SoTien's "no figure" into a JSON `0` would leave
	// every domain test green and put a zero on a government screen.
	m := dungMayChuNganSach(t)
	m.capQuyen(xaA, "budget.read")

	khong := bangChiCuaXaA()
	for i := range khong.KhoanMuc {
		khong.KhoanMuc[i].LaDongTong = false
	}
	m.doc.theo[xaA][khoaBang(2026, domain.BangChi)] = khong

	var ra bangDayDuRa
	docJSON(t, m.goi(t, http.MethodGet, hostA, duongBang+"?year=2026&kind=chi", canBoGhi(xaA), ""), &ra)

	if ra.Summary.HeadlineLineID != "" {
		t.Fatalf("không dòng nào được đánh dấu mà vẫn trả dòng tổng %q", ra.Summary.HeadlineLineID)
	}
	if len(ra.Summary.Cells) != 0 {
		t.Fatalf("không dòng tổng mà vẫn có %d ô tóm tắt", len(ra.Summary.Cells))
	}
	if ra.Summary.UnavailableReason == "" {
		t.Fatal("không có dòng tổng mà cũng không có CÂU nào nói ra — đó là một màn hình trông như hỏng")
	}
	if ra.Summary.Indicator.BasisPoints != nil {
		t.Fatalf("chỉ số vẫn ra %d phần vạn khi chưa có dòng tổng — phải là null",
			*ra.Summary.Indicator.BasisPoints)
	}
	if ra.Summary.Indicator.UnavailableReason == "" {
		t.Fatal("chỉ số biến mất mà không kèm lý do")
	}
}

func TestHaiDongCungDanhDauThiKhongCoSoTongVaNoiRaLaDemDoi(t *testing.T) {
	// THE DECISION OF THIS TURN, ASSERTED AT THE SURFACE. Two marked rows is a sheet with two
	// candidate totals; the thu sheet's two nested top-level rows and the chi sheet's `Tổng số`
	// beside A…E are exactly why summing them, or taking the first, is a DOUBLE COUNT.
	//
	// The write path makes the star a radio so this is unreachable through any route here; it is
	// asserted because the database carries no constraint for it (migration 0006 says why) and the
	// state can arrive from an import or a psql session.
	m := dungMayChuNganSach(t)
	m.capQuyen(xaA, "budget.read")

	hai := bangChiCuaXaA()
	hai.KhoanMuc[1].LaDongTong = true
	m.doc.theo[xaA][khoaBang(2026, domain.BangChi)] = hai

	var ra bangDayDuRa
	docJSON(t, m.goi(t, http.MethodGet, hostA, duongBang+"?year=2026&kind=chi", canBoGhi(xaA), ""), &ra)

	if ra.Summary.HeadlineLineID != "" || len(ra.Summary.Cells) != 0 {
		t.Fatal("hai dòng cùng đánh dấu mà vẫn chọn một dòng làm tổng")
	}
	if !strings.Contains(ra.Summary.UnavailableReason, "đếm đôi") {
		t.Fatalf("lý do không nói ra phép đếm đôi: %q", ra.Summary.UnavailableReason)
	}
	if ra.Summary.Indicator.BasisPoints != nil {
		t.Fatal("chỉ số vẫn ra số khi có hai dòng tổng")
	}
}

// --- (4) the two figures ADR 0035 §A fixed --------------------------------------------------------------

func TestChiSoNamHienCaHaiSoThuVaCanDoiLayThuXaHuong(t *testing.T) {
	// ADR 0035 §A's two mandatory consequences, on the wire:
	//   - BOTH revenue figures reach the screen, each by its own name;
	//   - the ONE thing there is a single answer for — `Cân đối` — is built on `Thu xã hưởng`.
	m := dungMayChuNganSach(t)
	m.capQuyen(xaA, "budget.read")

	var ra chiSoNamRa
	docJSON(t, m.goi(t, http.MethodGet, hostA, duongChiSo+"?year=2026", canBoGhi(xaA), ""), &ra)

	if len(ra.RevenueTotals) != 2 {
		t.Fatalf("màn hình nhận %d số thu, ADR 0035 §A bắt hiện CẢ HAI", len(ra.RevenueTotals))
	}
	vai := map[string]int64{}
	for _, o := range ra.RevenueTotals {
		if o.Value == nil {
			t.Fatalf("số thu %q không có giá trị", o.Name)
		}
		vai[o.Role] = *o.Value
	}
	if vai["thu-nsnn"] != int64(trieuDong(43_167_643)) {
		t.Errorf("Thu ngân sách NSNN = %d", vai["thu-nsnn"])
	}
	if vai["thu-xa-huong"] != int64(trieuDong(33_008_005)) {
		t.Errorf("Thu xã hưởng = %d", vai["thu-xa-huong"])
	}

	if ra.Balance.Amount == nil {
		t.Fatalf("Cân đối không ra số: %s", ra.Balance.UnavailableReason)
	}
	muon := int64(trieuDong(33_008_005) - trieuDong(34_634_592))
	if *ra.Balance.Amount != muon {
		t.Fatalf("Cân đối = %d, muốn %d", *ra.Balance.Amount, muon)
	}
	if *ra.Balance.Amount >= 0 {
		t.Fatal("Cân đối dương trên số liệu mẫu — đang lấy `Thu ngân sách NSNN` chứ không phải " +
			"`Thu xã hưởng` (ADR 0035 #32)")
	}

	if ra.RevenueAchievement.BasisPoints == nil || *ra.RevenueAchievement.BasisPoints != 10811 {
		t.Fatalf("Thu đạt dự toán = %v, muốn 10811 phần vạn (chia cho `Dự toán TP giao`, ADR 0035 #33)",
			ra.RevenueAchievement.BasisPoints)
	}
	if ra.ExpenditureAchievement.BasisPoints == nil || *ra.ExpenditureAchievement.BasisPoints != 9127 {
		t.Fatalf("Chi đạt dự toán = %v, muốn 9127 phần vạn", ra.ExpenditureAchievement.BasisPoints)
	}
}

func TestDuToanTPGiaoTrongThiChiSoBienMatKemCauChuKhongRa0(t *testing.T) {
	// ADR 0035 §A: `Dự toán TP giao` becomes a column that MUST have data, and a commune that leaves
	// it blank loses the indicator — "và điều đó phải hiện ra thành một CÂU, không thành một ô
	// trống". Neither 0 nor NaN, and the JSON field is `null` rather than absent so that a client
	// cannot read "the server does not report this" as "zero".
	m := dungMayChuNganSach(t)
	m.capQuyen(xaA, "budget.read")

	thu := bangThuCuaXaA()
	delete(thu.Gia[idDongThu], "c-tp")
	m.doc.theo[xaA][khoaBang(2026, domain.BangThu)] = thu

	var ra chiSoNamRa
	docJSON(t, m.goi(t, http.MethodGet, hostA, duongChiSo+"?year=2026", canBoGhi(xaA), ""), &ra)

	if ra.RevenueAchievement.BasisPoints != nil {
		t.Fatalf("Thu đạt dự toán vẫn ra %d khi `Dự toán TP giao` trống",
			*ra.RevenueAchievement.BasisPoints)
	}
	if ra.RevenueAchievement.UnavailableReason == "" {
		t.Fatal("chỉ số biến mất mà không có CÂU nào nói vì sao")
	}
	// The raw JSON is checked as well: `null` and `0` are one character apart in a struct that has
	// just been round-tripped, and a `omitempty` added to BasisPoints would make the field vanish
	// instead — which a client reads as "not reported" rather than as "no figure".
	w := m.goi(t, http.MethodGet, hostA, duongChiSo+"?year=2026", canBoGhi(xaA), "")
	if !strings.Contains(w.Body.String(), `"basis_points":null`) {
		t.Fatalf("thân JSON không mang `\"basis_points\":null`: %s", w.Body.String())
	}
}

func TestThieuBangChiThiCanDoiVaChiDatDuToanBienMatChuKhongRa0(t *testing.T) {
	// "Xã chưa nhập bảng chi" and "xã chi 0 đồng" are different statements and only one of them is
	// ever true. 200 with reasons, NOT 404: the card belongs on the screen, with a job to do on it.
	m := dungMayChuNganSach(t)
	m.capQuyen(xaA, "budget.read")
	delete(m.doc.theo[xaA], khoaBang(2026, domain.BangChi))

	w := m.goi(t, http.MethodGet, hostA, duongChiSo+"?year=2026", canBoGhi(xaA), "")
	doiMa(t, w, http.StatusOK)

	var ra chiSoNamRa
	if err := json.Unmarshal(w.Body.Bytes(), &ra); err != nil {
		t.Fatalf("thân không phải JSON: %v", err)
	}
	if ra.Balance.Amount != nil {
		t.Fatalf("thiếu bảng chi mà Cân đối vẫn ra %d", *ra.Balance.Amount)
	}
	if ra.Balance.UnavailableReason == "" || ra.ExpenditureAchievement.UnavailableReason == "" {
		t.Fatal("thiếu bảng chi mà không có lý do nào hiện ra")
	}
	// The revenue half is untouched: a commune that has entered its revenue sheet must still see it.
	if ra.RevenueAchievement.BasisPoints == nil {
		t.Fatal("thiếu bảng CHI mà chỉ số THU cũng mất")
	}
}

// --- (5) the fields a client may not set -----------------------------------------------------------------

func TestClientKhongDatDuocCachTinhCapVaDongTong(t *testing.T) {
	// REFUSED, NOT IGNORED, and each for its own reason:
	//   `method`      a parent marked `manual` is the direct entry anh Hà's 06/09/2026 rule forbids;
	//   `level`       a second answer to a question the tree already answers;
	//   `is_headline` a client believing it just made this row the commune's reported total.
	m := dungMayChuNganSach(t)
	m.capQuyen(xaA, "budget.update")

	for _, tc := range []struct{ ten, than string }{
		{"method", `{"sheet_id":"` + idBangChi + `","name":"X","order":1,"method":"manual"}`},
		{"level", `{"sheet_id":"` + idBangChi + `","name":"X","order":1,"level":3}`},
		{"is_headline", `{"sheet_id":"` + idBangChi + `","name":"X","order":1,"is_headline":true}`},
	} {
		t.Run("POST "+tc.ten, func(t *testing.T) {
			w := m.goi(t, http.MethodPost, hostA, duongDong, canBoGhi(xaA), tc.than)
			doiMa(t, w, http.StatusBadRequest)
		})
	}
	for _, tc := range []struct{ ten, than string }{
		{"method", `{"method":"children"}`},
		{"level", `{"level":0}`},
		{"is_headline", `{"is_headline":true}`},
		{"parent_id", `{"parent_id":"k-a"}`},
		{"sheet_id", `{"sheet_id":"` + idBangThu + `"}`},
	} {
		t.Run("PATCH "+tc.ten, func(t *testing.T) {
			w := m.goi(t, http.MethodPatch, hostA, duongDong+"/"+idDongChi, canBoGhi(xaA), tc.than)
			doiMa(t, w, http.StatusBadRequest)
		})
	}
	if m.ghi.tongGoi() != 0 {
		t.Errorf("use case chạy %d lần với thân yêu cầu đã bị từ chối", m.ghi.tongGoi())
	}
}

func TestGoThangVaoKhoanMucChaTra409VoiCauCuaKhach(t *testing.T) {
	// The customer's decision of 06/09/2026 reaching the accountant as a sentence, with 409 rather
	// than 403: the caller HOLDS `budget.update` and is allowed to type figures. What is refused is
	// this figure on THIS row, because the row has children.
	m := dungMayChuNganSach(t)
	m.capQuyen(xaA, "budget.update")
	m.ghi.loi = domain.ErrKhoanMucChaKhongGoThang

	w := m.goi(t, http.MethodPatch, hostA, duongDong+"/"+idDongChi, canBoGhi(xaA),
		`{"values":{"c-chi":1000000}}`)
	doiMa(t, w, http.StatusConflict)
	if e := loiTra(t, w); !strings.Contains(e.Message, "cộng từ các dòng con") {
		t.Fatalf("câu trả về không nói ra quy tắc: %q", e.Message)
	}
}

func TestONayTrongKhacONayBangKhong(t *testing.T) {
	// §9 rule 4 on the wire: a filled cell is a number, an empty one is `null`, and `0` is a filled
	// cell holding zero. A handler that emitted 0 for an empty cell would print `0` where the screen
	// must draw `—`, and that zero would be totalled into a figure sent upward.
	m := dungMayChuNganSach(t)
	m.capQuyen(xaA, "budget.read")

	b := bangChiCuaXaA()
	b.Gia["k-a"] = map[string]domain.Dong{"c-dt": 0} // 0 typed in; `c-chi` left empty
	m.doc.theo[xaA][khoaBang(2026, domain.BangChi)] = b

	var ra bangDayDuRa
	docJSON(t, m.goi(t, http.MethodGet, hostA, duongBang+"?year=2026&kind=chi", canBoGhi(xaA), ""), &ra)

	var dong dongRa
	for _, d := range ra.Lines {
		if d.ID == "k-a" {
			dong = d
		}
	}
	if dong.Values["c-dt"] == nil || *dong.Values["c-dt"] != 0 {
		t.Fatalf("ô ghi 0 trả về %v, muốn 0", dong.Values["c-dt"])
	}
	if v, co := dong.Values["c-chi"]; !co || v != nil {
		t.Fatalf("ô trống trả về %v, muốn null", v)
	}
	if _, co := dong.Values["c-ty"]; co {
		t.Error("cột phần trăm không được có ô giá trị — phần trăm tính khi hiển thị (§9 quy tắc 3)")
	}
}

func TestThieuNamHoacLoaiThiTuChoiChuKhongMacDinh(t *testing.T) {
	// A default here decides WHICH MONEY gets reported, invisibly. §13's reasoning for du_an applies
	// unchanged, and `kind` has no sensible default at all — the two tabs are two different reports.
	m := dungMayChuNganSach(t)
	m.capQuyen(xaA, "budget.read")

	for _, duong := range []string{
		duongBang,
		duongBang + "?kind=chi",
		duongBang + "?year=2026",
		duongBang + "?year=2026&kind=ca-hai",
		duongBang + "?year=1026&kind=chi",
		duongChiSo,
	} {
		t.Run(duong, func(t *testing.T) {
			doiMa(t, m.goi(t, http.MethodGet, hostA, duong, canBoGhi(xaA), ""), http.StatusBadRequest)
		})
	}
	if m.doc.goi != 0 {
		t.Errorf("kho chạy %d lần với tham số đã bị từ chối", m.doc.goi)
	}
}

func docJSON(t *testing.T, w *httptest.ResponseRecorder, vao any) {
	t.Helper()
	if w.Code != http.StatusOK {
		t.Fatalf("mã trạng thái = %d, muốn 200 — thân: %s", w.Code, w.Body.String())
	}
	if err := json.Unmarshal(w.Body.Bytes(), vao); err != nil {
		t.Fatalf("thân không phải JSON: %v — %s", err, w.Body.String())
	}
}
