package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/page"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/app"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// Tests for the five STAFF processing routes.
//
//	PROVED HERE   each route's four cases (rule 5, invariant 7): 401 no session · 403 wrong
//	              permission · 401 right permission WRONG COMMUNE · 200 both correct — and in the
//	              first three cases the use case is NOT REACHED, which is what proves the guard runs
//	              before any data is touched · the register list excludes the restricted field
//	              `can-bo` from the FILTER for an account without `feedback.restricted`, and includes
//	              it for one that holds it · the reporter is masked on the list even for a holder of
//	              `feedback.unmask` · a closing with no readable result is refused · a state refusal
//	              is 409 and not 403 · the acting principal reaches the use case as its BUSINESS CODE.
//
//	NOT PROVED    the transaction boundaries, the outbox row, the working-hours arithmetic. Those are
//	              properties of internal/app over the real store, and they live in
//	              internal/app/xu_ly_phan_anh_test.go.

// --- fakes ---------------------------------------------------------------------------------------

// danhSachPhieuGia is the register list, KEYED BY COMMUNE, reading the commune from the context
// exactly as *store.Scoped does. Keyed any other way, the isolation cases would pass while proving
// nothing.
//
// IT RECORDS THE FILTER IT WAS GIVEN, and that is the point of the type: the restricted-field
// decision is made in the handler and travels to the store as `LocPhieu.ChoPhepHanChe`, so the only
// honest place to assert it is the value the store actually received.
type danhSachPhieuGia struct {
	theo map[tenant.ID][]domain.PhieuPhanAnh
	loi  error
	goi  int
	loc  petstore.LocPhieu
}

func (d *danhSachPhieuGia) DanhSach(ctx context.Context, loc petstore.LocPhieu, _ page.Request) (
	page.Result[domain.PhieuPhanAnh], error) {

	d.goi++
	d.loc = loc
	if d.loi != nil {
		return page.NewResult[domain.PhieuPhanAnh](), d.loi
	}
	ra := page.NewResult[domain.PhieuPhanAnh]()
	for _, p := range d.theo[tenant.MustFrom(ctx)] {
		// THE FAKE APPLIES THE FILTER IT WAS GIVEN, rather than returning everything. Without this
		// the "restricted field is excluded" case would assert only on a struct field and would stay
		// green if the store ignored it — which is the half of the defect that actually leaks.
		if !loc.ChoPhepHanChe && p.LinhVuc == domain.LinhVucHanChe {
			continue
		}
		// Same reason for the "Giao cho tôi" filter: honoured, so a case asserting on the ITEMS fails
		// if the handler hands down the wrong code, not only if it hands down none.
		if loc.CanBoXuLyID != "" && p.CanBoXuLyID != loc.CanBoXuLyID {
			continue
		}
		ra.Items = append(ra.Items, p)
	}
	return ra, nil
}

// danhSachTuPhieuMau builds the list fake from the SAME fixture the single-petition reader uses.
//
// ONE FIXTURE FOR BOTH SURFACES, on purpose: two would let the list's restricted-field case pass
// against a petition the detail route does not have, which proves nothing about the register a
// commune actually holds. The order within a commune is not meaningful here — the store's ORDER BY
// is not under test at this layer.
func danhSachTuPhieuMau(p *phieuGia) *danhSachPhieuGia {
	theo := make(map[tenant.ID][]domain.PhieuPhanAnh, len(p.theo))
	for xa, theoMa := range p.theo {
		for _, mot := range theoMa {
			theo[xa] = append(theo[xa], mot)
		}
	}
	return &danhSachPhieuGia{theo: theo}
}

// xuLyPhieuGia is the four staff acts. IT RECORDS THE COMMUNE AND THE ACTOR of every call, because
// those are the two things a route can get wrong in a way no status code shows.
type xuLyPhieuGia struct {
	goi   int
	viec  string
	xa    tenant.ID
	nguoi audit.Actor
	maDa  string

	ycLinhVuc  app.YeuCauChotLinhVuc
	ycPhanCong app.YeuCauPhanCong
	ketQua     string
	lyDo       string
	ycChuyen   app.YeuCauChuyenCapTren

	// quyenCaXa is the fact the ADVANCE route hands down: does the caller hold `feedback.resolve`?
	//
	// RECORDED RATHER THAN ACTED ON. The holding rule is app.duocTienTrangThai's and is proved over the
	// real store in internal/app; what this layer can get wrong — and what is asserted here — is
	// WHICH FACT is handed down and whether it is read from the right key.
	quyenCaXa app.QuyenXuLyCaXa

	// hanChe is the fact ALL FOUR write routes hand down: does the caller hold
	// `feedback.restricted`? Recorded rather than acted on, for the same reason as quyenCaXa — which
	// petitions it refuses is app.duocChamPhieuHanChe's and is proved over the real store.
	hanChe app.QuyenXemHanChe

	// linhVuc is the field of the petition this fake answers with. IT EXISTS FOR ONE CASE ONLY: the
	// SECOND layer in traPhieu, which is unreachable while the use case refuses properly. Setting it
	// to `can-bo` is how a test plays the fifth write route that forgot to hand the fact down.
	linhVuc string

	// ghiChu is the optional internal note the three string-signature acts and the manual note route
	// hand down (migration 0013). The struct-carried ones are on ycLinhVuc / ycPhanCong / ycChuyen.
	ghiChu string

	// quyenGhiChu is the fact the manual note route hands down: ANY of resolve/assign/classify.
	quyenGhiChu app.QuyenGhiChuCaXa

	loi error
}

func (x *xuLyPhieuGia) ghi(ctx context.Context, viec, ma string, nguoi audit.Actor,
	hanChe app.QuyenXemHanChe) {

	x.goi++
	x.viec, x.maDa, x.nguoi, x.hanChe = viec, ma, nguoi, hanChe
	x.xa = tenant.MustFrom(ctx)
}

func (x *xuLyPhieuGia) tra() (domain.PhieuPhanAnh, error) {
	if x.loi != nil {
		return domain.PhieuPhanAnh{}, x.loi
	}
	linhVuc := x.linhVuc
	if linhVuc == "" {
		linhVuc = "rac-thai"
	}
	return domain.PhieuPhanAnh{
		MaTraCuu: maPhieuThuong, Kenh: domain.KenhZaloMiniApp, CongDanID: "cd-001",
		NoiDung: "Đống rác ở đầu ngõ đã ba ngày chưa ai dọn.",
		LinhVuc: linhVuc, TrangThai: domain.DangPhanLoai,
		// ASSIGNED TO THE OFFICER THE FIXTURE PRINCIPAL IS, by the BUSINESS CODE the column holds
		// (rule 6, invariant 8). The holding-rule cases below read as what they claim to be only if
		// this petition really is the one that officer was handed.
		BoPhanID: "bp-001", CanBoXuLyID: maCanBo,
		GocDemHan: mocGui, VaoSoLuc: mocVaoSo,
		HanTiepNhan: mocTiepNh, HanXuLyXong: mocXuLy,
	}, nil
}

func (x *xuLyPhieuGia) ChotLinhVuc(ctx context.Context, ma string, yc app.YeuCauChotLinhVuc,
	nguoi audit.Actor, hanChe app.QuyenXemHanChe) (domain.PhieuPhanAnh, error) {

	x.ghi(ctx, "phan-loai", ma, nguoi, hanChe)
	x.ycLinhVuc = yc
	return x.tra()
}

func (x *xuLyPhieuGia) PhanCong(ctx context.Context, ma string, yc app.YeuCauPhanCong,
	nguoi audit.Actor, hanChe app.QuyenXemHanChe) (domain.PhieuPhanAnh, error) {

	x.ghi(ctx, "phan-cong", ma, nguoi, hanChe)
	x.ycPhanCong = yc
	return x.tra()
}

func (x *xuLyPhieuGia) TienTrangThai(ctx context.Context, ma, ghiChu string, nguoi audit.Actor,
	quyen app.QuyenXuLyCaXa, hanChe app.QuyenXemHanChe) (domain.PhieuPhanAnh, error) {

	x.ghi(ctx, "tien", ma, nguoi, hanChe)
	x.quyenCaXa, x.ghiChu = quyen, ghiChu
	return x.tra()
}

func (x *xuLyPhieuGia) Dong(ctx context.Context, ma, ketQua, ghiChu string, nguoi audit.Actor,
	hanChe app.QuyenXemHanChe) (domain.PhieuPhanAnh, error) {

	x.ghi(ctx, "dong", ma, nguoi, hanChe)
	x.ketQua, x.ghiChu = ketQua, ghiChu
	return x.tra()
}

func (x *xuLyPhieuGia) KhongTiepNhan(ctx context.Context, ma, lyDo, ghiChu string, nguoi audit.Actor,
	hanChe app.QuyenXemHanChe) (domain.PhieuPhanAnh, error) {

	x.ghi(ctx, "khong-tiep-nhan", ma, nguoi, hanChe)
	x.lyDo, x.ghiChu = lyDo, ghiChu
	return x.tra()
}

// GhiChuNoiBo RECORDS the two facts and the note, and answers with a row shaped like the real one.
// Whether the note is ALLOWED is app.duocGhiChu's and is proved over the real store in internal/app.
func (x *xuLyPhieuGia) GhiChuNoiBo(ctx context.Context, ma, ghiChu string, nguoi audit.Actor,
	quyen app.QuyenGhiChuCaXa, hanChe app.QuyenXemHanChe) (domain.NhatKyPhanAnh, error) {

	x.ghi(ctx, "ghi-chu", ma, nguoi, hanChe)
	x.ghiChu, x.quyenGhiChu = ghiChu, quyen
	if x.loi != nil {
		return domain.NhatKyPhanAnh{}, x.loi
	}
	return domain.NhatKyPhanAnh{
		ID: "nkpa-01JTHU", PhieuPhanAnhID: "pa-001", ThoiDiem: mocVaoSo, NguoiMa: nguoi.ID,
		HanhVi: domain.NhatKyGhiChu, TrangThai: domain.DangPhanLoai, NoiDung: ghiChu,
	}, nil
}

func (x *xuLyPhieuGia) ChuyenCapTren(ctx context.Context, ma string, yc app.YeuCauChuyenCapTren,
	nguoi audit.Actor, hanChe app.QuyenXemHanChe) (domain.PhieuPhanAnh, error) {

	x.ghi(ctx, "chuyen-cap-tren", ma, nguoi, hanChe)
	x.ycChuyen = yc
	return x.tra()
}

// --- fixtures ------------------------------------------------------------------------------------

// The four write routes, named once so a case cannot drift from the route it is testing.
func duongPhanLoai(ma string) string  { return duong(ma) + "/classification" }
func duongPhanCong(ma string) string  { return duong(ma) + "/assignment" }
func duongTienTrang(ma string) string { return duong(ma) + "/status" }
func duongDong(ma string) string      { return duong(ma) + "/closure" }

func duongKhongTiepNhan(ma string) string { return duong(ma) + "/rejection" }
func duongChuyenCapTren(ma string) string { return duong(ma) + "/referral" }

const duongDanhSach = "/api/v1/citizen-reports"

// ketQuaThat is a result of the shape rule 10, invariant 6 asks for: it says what was actually done,
// not that something was done.
const ketQuaThat = "Đội vệ sinh đã thu gom toàn bộ rác tại đầu ngõ ngày 24/9 và dựng biển cấm đổ rác."

// goiThan issues a request with a JSON body. A nil principal means "not signed in" — the 401 case.
func (m *mayChu) goiThan(t *testing.T, method, host, path string, p *authz.Principal, than any) *httptest.ResponseRecorder {
	t.Helper()
	var r *http.Request
	if than == nil {
		r = httptest.NewRequest(method, "https://"+host+path, nil)
	} else {
		b, err := json.Marshal(than)
		if err != nil {
			t.Fatalf("mã hoá thân: %v", err)
		}
		r = httptest.NewRequest(method, "https://"+host+path, bytes.NewReader(b))
		r.Header.Set("Content-Type", "application/json")
	}
	r.Host = host
	r.RemoteAddr = "10.0.0.7:51000"
	if p != nil {
		r = r.WithContext(authz.Into(r.Context(), *p))
	}
	w := httptest.NewRecorder()
	m.h.ServeHTTP(w, r)
	return w
}

// capQuyen rebuilds the chain with commune A's account holding exactly the listed keys.
//
// EXACTLY, not "at least": every case below names the key it needs and nothing more, so a route that
// consulted the wrong key would fail rather than pass on a neighbouring one.
func (m *mayChu) capQuyen(t *testing.T, khoa ...authz.Perm) {
	t.Helper()
	cho := map[authz.Perm]bool{}
	for _, k := range khoa {
		cho[k] = true
	}
	m.dungLai(t, func(d *Deps) {
		d.Checker = checkerGia{quyen: map[tenant.ID]map[string]map[authz.Perm]bool{
			xaA: {idCanBo: cho},
			xaB: {},
		}}
	})
}

// --- rule 5, invariant 7: four cases per route ----------------------------------------------------
//
// ONE TABLE FOR THE FIVE ROUTES, and the table is the point rather than a convenience: writing them
// out five times invites the version where one route's 403 case quietly uses the key that route does
// not need. Each row names the method, the path, the body and THE KEY THAT MUST OPEN IT.

type caTuyen struct {
	ten    string
	method string
	duong  string
	than   any
	khoa   authz.Perm
	// khoaSai is a key the caller DOES hold and which must NOT open this route. It is a real key of
	// this subsystem, not a nonsense string: a 403 against `task.read` would also pass if the route
	// checked nothing at all against the petitions keys.
	khoaSai authz.Perm
}

func caCacTuyen() []caTuyen {
	return []caTuyen{
		{"danh sách", http.MethodGet, duongDanhSach, nil,
			authz.Perm("feedback.read"), authz.Perm("feedback.resolve")},
		// The "Giao cho tôi" tab is the SAME route with the same key — the filter narrows what
		// `feedback.read` already opens and grants nothing of its own, so it must fail the same way.
		{"danh sách · giao cho tôi", http.MethodGet, duongDanhSach + "?scope=mine", nil,
			authz.Perm("feedback.read"), authz.Perm("feedback.resolve")},
		{"phân loại", http.MethodPost, duongPhanLoai(maPhieuThuong), phanLoaiVao{Field: "rac-thai"},
			authz.Perm("feedback.classify"), authz.Perm("feedback.assign")},
		{"phân công", http.MethodPost, duongPhanCong(maPhieuThuong), phanCongVao{Unit: "bp-001"},
			authz.Perm("feedback.assign"), authz.Perm("feedback.classify")},
		// `feedback.read` AND NOT `feedback.resolve` SINCE 2026-09-23 — the holding rule. The gate is
		// still a real, explicit key: an account holding `feedback.assign` and nothing else is refused
		// here exactly as before, which is what keeps this from being AnyAuthenticated wearing a
		// declaration.
		{"chuyển trạng thái", http.MethodPost, duongTienTrang(maPhieuThuong), nil,
			authz.Perm("feedback.read"), authz.Perm("feedback.assign")},
		{"đóng phiếu", http.MethodPost, duongDong(maPhieuThuong), dongPhieuVao{Result: ketQuaThat},
			authz.Perm("feedback.resolve"), authz.Perm("feedback.assign")},
		// THE TWO BRANCHES (25/09/2026). `feedback.classify` opens them; `feedback.read` — the key
		// every reader of the register holds — must not, and neither must `feedback.resolve`, the
		// commune-wide working right: refusing a petition is a classification outcome, not work on it.
		{"không tiếp nhận", http.MethodPost, duongKhongTiepNhan(maPhieuThuong),
			khongTiepNhanVao{Reason: lyDoThatHTTP},
			authz.Perm("feedback.classify"), authz.Perm("feedback.read")},
		{"chuyển cấp trên", http.MethodPost, duongChuyenCapTren(maPhieuThuong),
			chuyenCapTrenVao{Reason: lyDoThatHTTP, ReceivingBody: coQuanThatHTTP},
			authz.Perm("feedback.classify"), authz.Perm("feedback.resolve")},
	}
}

func TestTuyenXuLyKhongCoPhienThi401(t *testing.T) {
	for _, ca := range caCacTuyen() {
		t.Run(ca.ten, func(t *testing.T) {
			m := dungMayChu(t)
			w := m.goiThan(t, ca.method, hostA, ca.duong, nil, ca.than)

			doiMa(t, w, http.StatusUnauthorized)
			// THE COUNT IS THE ASSERTION. A route that refused only AFTER touching the data would
			// still have read — or written — a commune's register.
			if m.danhSach.goi != 0 || m.xuLy.goi != 0 {
				t.Errorf("đã chạm dữ liệu dù chưa có phiên (đọc %d, ghi %d)", m.danhSach.goi, m.xuLy.goi)
			}
		})
	}
}

func TestTuyenXuLySaiQuyenThi403(t *testing.T) {
	for _, ca := range caCacTuyen() {
		t.Run(ca.ten, func(t *testing.T) {
			m := dungMayChu(t)
			// An account of commune A holding a REAL key of this subsystem — just not this route's.
			// Rule 5, invariant 3b: these rights are not a Cartesian product, and `feedback.assign`
			// must not open the route that ISSUES THE COMMUNE'S PROMISE.
			m.capQuyen(t, ca.khoaSai)

			w := m.goiThan(t, ca.method, hostA, ca.duong, canBoCuaXa(xaA), ca.than)

			doiMa(t, w, http.StatusForbidden)
			if m.danhSach.goi != 0 || m.xuLy.goi != 0 {
				t.Errorf("đã chạm dữ liệu dù sai quyền (đọc %d, ghi %d)", m.danhSach.goi, m.xuLy.goi)
			}
		})
	}
}

// TestTuyenXuLyDungQuyenSaiXaThi401 is the case no test of a single commune can produce.
//
// IT ANSWERS 401 AND NOT 403, and that is authz.xacNhanXa's behaviour rather than a slip: the commune
// on the principal is compared with the commune resolved from Host BEFORE the permission is
// consulted, so a token from another commune never reaches the permission check. The property being
// asserted is that NO DATA IS TOUCHED — a leak between two public authorities is heavier than a
// software bug (rule 1).
func TestTuyenXuLyDungQuyenSaiXaThi401(t *testing.T) {
	for _, ca := range caCacTuyen() {
		t.Run(ca.ten, func(t *testing.T) {
			m := dungMayChu(t)
			// The account holds the RIGHT key — in commune A. The request arrives at commune B's host.
			m.capQuyen(t, ca.khoa)

			w := m.goiThan(t, ca.method, hostB, ca.duong, canBoCuaXa(xaA), ca.than)

			doiMa(t, w, http.StatusUnauthorized)
			if m.danhSach.goi != 0 || m.xuLy.goi != 0 {
				t.Errorf("đã chạm dữ liệu của xã B bằng phiên của xã A (đọc %d, ghi %d) — "+
					"đây là rò rỉ giữa hai cơ quan nhà nước, không phải một lỗi phần mềm thường",
					m.danhSach.goi, m.xuLy.goi)
			}
		})
	}
}

func TestTuyenXuLyDungQuyenDungXaThi200(t *testing.T) {
	for _, ca := range caCacTuyen() {
		t.Run(ca.ten, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(t, ca.khoa)

			w := m.goiThan(t, ca.method, hostA, ca.duong, canBoCuaXa(xaA), ca.than)

			doiMa(t, w, http.StatusOK)
		})
	}
}

// --- the acting person reaches the use case as a BUSINESS CODE ------------------------------------

// TestTuyenXuLyChuTheLaMaCanBo pins rule 6, invariant 8 at the boundary it is broken at.
//
// `idCanBo` AND `maCanBo` ARE DIFFERENT STRINGS IN THE FIXTURES on purpose. A handler passing
// `Principal.ID` would produce a perfectly valid-looking audit entry naming nobody — which is exactly
// what six write paths in this repository did until 2026-09-22, with no test turning red.
func TestTuyenXuLyChuTheLaMaCanBo(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, authz.Perm("feedback.resolve"))

	w := m.goiThan(t, http.MethodPost, hostA, duongDong(maPhieuThuong), canBoCuaXa(xaA),
		dongPhieuVao{Result: ketQuaThat})
	doiMa(t, w, http.StatusOK)

	if m.xuLy.nguoi.ID != maCanBo {
		t.Errorf("chủ thể = %q, muốn MÃ cán bộ %q — vết kiểm toán đọc được nhiều năm sau bởi người "+
			"xử lý một khiếu nại, và một ULID ở đó không gọi tên ai", m.xuLy.nguoi.ID, maCanBo)
	}
	if m.xuLy.nguoi.Kind != "staff" {
		t.Errorf("kind = %q, muốn \"staff\"", m.xuLy.nguoi.Kind)
	}
	if m.xuLy.nguoi.IP == "" {
		t.Error("vết không mang IP — luật 6 bất biến 2 đòi sáu thứ, IP là một trong sáu")
	}
	if m.xuLy.xa != xaA {
		t.Errorf("xã = %q, muốn %q", m.xuLy.xa, xaA)
	}
}

// TestTuyenXuLyKhongCoMaCanBoThi500 — identity older than the `ma` field sends a principal with no
// business code. A fallback to the internal id would put two kinds of identifier into
// `audit_log.actor_id` one deployment window at a time, with every test green.
func TestTuyenXuLyKhongCoMaCanBoThi500(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, authz.Perm("feedback.classify"))

	p := canBoCuaXa(xaA)
	p.Ma = ""
	w := m.goiThan(t, http.MethodPost, hostA, duongPhanLoai(maPhieuThuong), p,
		phanLoaiVao{Field: "rac-thai"})

	doiMa(t, w, http.StatusInternalServerError)
	if m.xuLy.goi != 0 {
		t.Error("đã gọi use case dù không có mã cán bộ — luật 6 không cho một lần ghi nghiệp vụ mà " +
			"vết không gọi tên được người làm")
	}
}

// --- the holding rule at the route, and the negative half that carries the whole change ------------
//
// Decided by the owner on 2026-09-23 ("theo require"). THE FOUR CELLS OF THE RULE ITSELF are proved
// over the real store in internal/app — the use case is what decides them. What only this layer can
// prove, and what would be silently wrong if nobody did:
//
//	the ADVANCE route lets an account holding ONLY `feedback.read` through its gate
//	the CLOSING route does NOT, for the same account, EVEN WHEN THAT ACCOUNT IS THE ASSIGNEE
//	the fact handed down is read from `feedback.resolve` and from no neighbouring key
//	the value compared against `can_bo_xu_ly_id` is the BUSINESS CODE, not the internal id

// TestTienTrangThaiChiCoFeedbackReadThiQuaCongVoiQuyenCaXaLaFalse is the THIRD CELL at the route: the
// hamlet leader who was handed one petition and holds nothing but `feedback.read`.
//
// THREE ASSERTIONS AND EACH IS A DIFFERENT DEFECT. The status proves the gate let them through; the
// recorded flag proves the handler did not quietly grant the commune-wide right (a `true` here opens
// every petition in the commune to every reader); the actor's ID proves the value the use case will
// compare against `can_bo_xu_ly_id` is `CB-00123` and not `nd-01J…` — two identifiers that are
// indistinguishable on sight and never match each other.
//
// ĐỘT BIẾN: đổi `p.Ma` thành `p.ID` ở nguoiThucHien và ca này ĐỎ — mọi cán bộ rơi vào nhánh "không
// phải người được giao", tính năng không chạy, và không gì khác báo.
func TestTienTrangThaiChiCoFeedbackReadThiQuaCongVoiQuyenCaXaLaFalse(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, authz.Perm("feedback.read"))

	w := m.goiThan(t, http.MethodPost, hostA, duongTienTrang(maPhieuThuong), canBoCuaXa(xaA), nil)

	doiMa(t, w, http.StatusOK)
	if m.xuLy.goi != 1 {
		t.Fatalf("gọi use case %d lần, muốn 1 — cổng chặn mất người được giao thì tính năng này "+
			"không tồn tại", m.xuLy.goi)
	}
	if m.xuLy.quyenCaXa {
		t.Error("tuyến báo xuống là CÓ quyền xử lý cả xã dù tài khoản chỉ có feedback.read — " +
			"một `true` ở đây mở mọi phiếu của xã cho mọi người đọc được sổ")
	}
	if m.xuLy.nguoi.ID != maCanBo {
		t.Errorf("chủ thể xuống use case = %q, muốn MÃ cán bộ %q — cột `can_bo_xu_ly_id` giữ mã cán "+
			"bộ, nên so bằng id nội bộ thì KHÔNG AI khớp và không gì đỏ", m.xuLy.nguoi.ID, maCanBo)
	}
}

// TestTienTrangThaiCoFeedbackResolveThiQuyenCaXaLaTrue — cells one and two. The commune-wide right
// still works exactly as it did, whoever the petition was assigned to.
func TestTienTrangThaiCoFeedbackResolveThiQuyenCaXaLaTrue(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, authz.Perm("feedback.read"), QuyenXuLyCaXa)

	w := m.goiThan(t, http.MethodPost, hostA, duongTienTrang(maPhieuThuong), canBoCuaXa(xaA), nil)

	doiMa(t, w, http.StatusOK)
	if !m.xuLy.quyenCaXa {
		t.Error("tuyến báo xuống là KHÔNG có quyền cả xã dù tài khoản có feedback.resolve — quyền " +
			"cấp ra mà không mở gì là một ô tích không làm gì trong màn Phân quyền")
	}
}

// TestTienTrangThaiKhongDocTuKhoaQuyenLangGieng — the flag must come from `feedback.resolve` and from
// nothing that merely sits next to it in the same group.
//
// WITHOUT THIS CASE, a handler reading `feedback.assign` or `feedback.classify` would pass every other
// test in this file: the route opens on `feedback.read`, so the flag is the only thing that differs,
// and a wrong key produces a plausible `false` in exactly the cases that are supposed to be `true`.
func TestTienTrangThaiKhongDocTuKhoaQuyenLangGieng(t *testing.T) {
	for _, khoa := range []authz.Perm{"feedback.assign", "feedback.classify", "feedback.restricted"} {
		t.Run(string(khoa), func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(t, authz.Perm("feedback.read"), khoa)

			w := m.goiThan(t, http.MethodPost, hostA, duongTienTrang(maPhieuThuong), canBoCuaXa(xaA), nil)

			doiMa(t, w, http.StatusOK)
			if m.xuLy.quyenCaXa {
				t.Errorf("khoá %q mở quyền xử lý cả xã — luật 5 bất biến 3b: các quyền này KHÔNG phải "+
					"tích Đề-các, và một khoá mở thay khoá khác là một ô tích cấp ra thứ nó không nói",
					khoa)
			}
		})
	}
}

// TestNguoiDuocGiaoTienDuocNhungKhongDongDuoc IS THE NEGATIVE HALF OF THIS WHOLE CHANGE.
//
// ONE ACCOUNT, ONE PETITION, TWO ROUTES. The account holds only `feedback.read` and IS the officer the
// fixture petition was assigned to — the exact person the holding rule was widened for. The advance
// route must let them through; the closing route must refuse them at the gate, and the use case must
// not be reached at all.
//
// WHY IT IS WORTH A TEST OF ITS OWN: closing records a RESULT THE CITIZEN READS (rule 10, invariant 6)
// and is precisely what open question #7 settled on 2026-09-16 — "`feedback.resolve` quyết định ai
// đóng được". The five routes the other repository lowered are all WORKING routes and none of them is
// the closing. Without this case, one line copied from the route above in the name of consistency
// would hand the closing of a commitment to a citizen to every account that can read the register,
// and nothing anywhere would turn red.
//
// ĐỘT BIẾN: đổi `/closure` sang `feedback.read` trong routes.go và ca này ĐỎ.
func TestNguoiDuocGiaoTienDuocNhungKhongDongDuoc(t *testing.T) {
	// The fixture petition is assigned to this very officer — see xuLyPhieuGia.tra.
	if m := dungMayChu(t); m.xuLy.quyenCaXa {
		t.Fatal("bộ đồ nghề khởi tạo sai")
	}

	t.Run("tiến trạng thái thì ĐƯỢC", func(t *testing.T) {
		m := dungMayChu(t)
		m.capQuyen(t, authz.Perm("feedback.read"))

		w := m.goiThan(t, http.MethodPost, hostA, duongTienTrang(maPhieuThuong), canBoCuaXa(xaA), nil)

		doiMa(t, w, http.StatusOK)
		if m.xuLy.goi != 1 {
			t.Errorf("gọi use case %d lần, muốn 1", m.xuLy.goi)
		}
	})

	t.Run("đóng phiếu thì KHÔNG", func(t *testing.T) {
		m := dungMayChu(t)
		m.capQuyen(t, authz.Perm("feedback.read"))

		w := m.goiThan(t, http.MethodPost, hostA, duongDong(maPhieuThuong), canBoCuaXa(xaA),
			dongPhieuVao{Result: ketQuaThat})

		doiMa(t, w, http.StatusForbidden)
		if m.xuLy.goi != 0 {
			t.Errorf("đã gọi use case đóng phiếu %d lần dù tài khoản không có feedback.resolve — "+
				"người được giao đóng được phiếu là lật câu hỏi mở #7 khách đã chốt 16/09/2026",
				m.xuLy.goi)
		}
	})
}

// TestTienTrangThaiKhongPhaiNguoiDuocGiaoThi403 — the fourth cell as it reaches a client.
//
// The use case refuses; what this layer decides is the STATUS CODE, and it is 403 rather than the 409
// every other refusal on this route maps to. A 409 would tell an officer to reload a petition they
// were never allowed to move, hiding a permission problem behind a sentence about state.
func TestTienTrangThaiKhongPhaiNguoiDuocGiaoThi403(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, authz.Perm("feedback.read"))
	m.xuLy.loi = app.ErrKhongPhaiNguoiDuocGiao

	w := m.goiThan(t, http.MethodPost, hostA, duongTienTrang(maPhieuThuong), canBoCuaXa(xaA), nil)

	doiMa(t, w, http.StatusForbidden)
	if loiTra(t, w).Code != "forbidden" {
		t.Errorf("mã lỗi = %q, muốn forbidden", loiTra(t, w).Code)
	}
}

// --- the restricted field on the FOUR WRITE ROUTES ---------------------------------------------------
//
// The leak these close was measured on 2026-09-23: both READ paths refused a caller without
// `feedback.restricted`, all four WRITE paths asked nothing, and all four return the whole petition on
// success — so one POST read a report about a member of staff out to a colleague of that person.
//
// WHICH HALF LIVES WHERE. The refusal itself is app.duocChamPhieuHanChe's, inside the transaction, and
// is proved over the real store in internal/app — including that it writes nothing. What only this
// layer can prove, and what would be silently wrong if nobody did: the fact handed down is read from
// `feedback.restricted` and from no neighbouring key, the refusal becomes a 404 and not a 403, and the
// response body carries nothing about the petition.

// tuyenGhi is the four write routes as a case needs them: a path and the key that opens its gate.
// DERIVED FROM caCacTuyen SO THE TWO CANNOT DRIFT — a route added there and forgotten here would
// silently stop being covered by everything below.
func tuyenGhi() []caTuyen {
	var ra []caTuyen
	for _, ca := range caCacTuyen() {
		if ca.method == http.MethodPost {
			ra = append(ra, ca)
		}
	}
	return ra
}

// TestBonTuyenGhiTruyenXuongSuThatVeQuyenHanChe — the handler answers ONE question and hands the
// answer down. Both directions are asserted, because a `true` that never varies and a `false` that
// never varies are both wrong and both look right in half the suite.
func TestBonTuyenGhiTruyenXuongSuThatVeQuyenHanChe(t *testing.T) {
	for _, ca := range tuyenGhi() {
		t.Run(ca.ten+"/không có quyền hạn chế", func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(t, ca.khoa)

			w := m.goiThan(t, ca.method, hostA, ca.duong, canBoCuaXa(xaA), ca.than)

			doiMa(t, w, http.StatusOK)
			if m.xuLy.hanChe {
				t.Error("tuyến báo xuống là CÓ quyền xem lĩnh vực hạn chế dù tài khoản không có " +
					"feedback.restricted — một `true` ở đây mở đơn tố cáo cán bộ cho chính đồng nghiệp họ")
			}
		})
		t.Run(ca.ten+"/có quyền hạn chế", func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(t, ca.khoa, QuyenHanChe)

			w := m.goiThan(t, ca.method, hostA, ca.duong, canBoCuaXa(xaA), ca.than)

			doiMa(t, w, http.StatusOK)
			if !m.xuLy.hanChe {
				t.Error("tuyến báo xuống là KHÔNG có quyền dù tài khoản có feedback.restricted — " +
					"quyền cấp ra mà không mở gì là một ô tích không làm gì trong màn Phân quyền")
			}
		})
	}
}

// TestBonTuyenGhiKhongDocTuKhoaQuyenLangGieng — the fact must come from `feedback.restricted` and from
// nothing that merely sits beside it in the same group (rule 5, invariant 3b: these rights are not a
// Cartesian product).
//
// WITHOUT THIS CASE a handler reading `feedback.resolve` would pass every other test here: the closing
// route's own gate is that key, so the flag would read `true` on exactly the route where the tests
// grant it.
func TestBonTuyenGhiKhongDocTuKhoaQuyenLangGieng(t *testing.T) {
	for _, khoa := range []authz.Perm{"feedback.resolve", "feedback.assign", "feedback.classify",
		"feedback.unmask"} {
		for _, ca := range tuyenGhi() {
			t.Run(ca.ten+"/"+string(khoa), func(t *testing.T) {
				m := dungMayChu(t)
				m.capQuyen(t, ca.khoa, khoa)

				w := m.goiThan(t, ca.method, hostA, ca.duong, canBoCuaXa(xaA), ca.than)

				doiMa(t, w, http.StatusOK)
				if m.xuLy.hanChe {
					t.Errorf("khoá %q mở lĩnh vực hạn chế — nó không phải feedback.restricted", khoa)
				}
			})
		}
	}
}

// TestBonTuyenGhiPhieuHanCheThi404ChuKhongPhai403 is the status-code half, and it is the assertion the
// whole shape turns on.
//
// A 403 HERE WOULD CONFIRM THAT A REPORT ABOUT A MEMBER OF STAFF EXISTS UNDER THIS CODE, to a
// colleague of that person, and the existence of such a report is precisely what the restriction
// protects. It is the same sentence Handler.khongTimThay already says for the read route — four causes,
// one answer — and this is the fifth cause folded in.
//
// THE BODY IS ASSERTED TOO, AND NOT AS AN EXTRA: a 404 whose body names the field, quotes the petition
// or spells `can-bo` would leak through the very response that was supposed to say nothing.
//
// ĐỘT BIẾN: đổi 404 thành 403 ở nhánh ErrPhieuHanChe trong traLoiLoiXuLy và ca này ĐỎ.
func TestBonTuyenGhiPhieuHanCheThi404ChuKhongPhai403(t *testing.T) {
	for _, ca := range tuyenGhi() {
		t.Run(ca.ten, func(t *testing.T) {
			m := dungMayChu(t)
			// The account holds this route's key and NOT `feedback.restricted` — the account the leak
			// was measured on.
			m.capQuyen(t, ca.khoa)
			m.xuLy.loi = app.ErrPhieuHanChe

			w := m.goiThan(t, ca.method, hostA, ca.duong, canBoCuaXa(xaA), ca.than)

			doiMa(t, w, http.StatusNotFound)
			if e := loiTra(t, w); e.Code != "not_found" {
				t.Errorf("mã lỗi = %q, muốn not_found — khác với mã một phiếu không tồn tại trả về là "+
					"phân biệt được hai nguyên nhân", e.Code)
			}
			than := w.Body.String()
			for _, cam := range []string{domain.LinhVucHanChe, "cán bộ tiếp dân", "tác phong",
				"Đống rác", "0900000000", "hạn chế"} {
				if strings.Contains(strings.ToLower(than), strings.ToLower(cam)) {
					t.Errorf("thân phản hồi 404 mang %q: %s", cam, than)
				}
			}
		})
	}
}

// TestTraPhieuLopThuHaiChePhieuHanChe exercises the SECOND layer, in Handler.traPhieu.
//
// It is unreachable through the four routes today — the use case refuses first — so the only way to
// reach it is what this case does: a use case that ANSWERS SUCCESSFULLY with a `can-bo` petition,
// which is precisely what a fifth write route written next year would produce if it forgot to hand
// the permission down.
//
// WHAT IT PROVES AND WHAT IT DOES NOT. It proves the record is withheld. It does NOT prove the act was
// refused — by this point the write has committed — and that limit is stated on traPhieu rather than
// implied here.
//
// ĐỘT BIẾN: xoá phép kiểm ở đầu traPhieu và ca này ĐỎ.
func TestTraPhieuLopThuHaiChePhieuHanChe(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, authz.Perm("feedback.classify"))
	m.xuLy.linhVuc = domain.LinhVucHanChe

	w := m.goiThan(t, http.MethodPost, hostA, duongPhanLoai(maPhieuThuong), canBoCuaXa(xaA),
		phanLoaiVao{Field: domain.LinhVucHanChe})

	doiMa(t, w, http.StatusNotFound)
	if strings.Contains(w.Body.String(), domain.LinhVucHanChe) {
		t.Errorf("thân phản hồi mang lĩnh vực hạn chế: %s", w.Body.String())
	}
}

// TestTraPhieuCoQuyenHanCheThiVanTraPhieu is the other half, and without it the case above would pass
// against a handler that answered 404 to EVERY `can-bo` petition — including for the officer whose key
// exists to open them.
func TestTraPhieuCoQuyenHanCheThiVanTraPhieu(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, authz.Perm("feedback.classify"), QuyenHanChe)
	m.xuLy.linhVuc = domain.LinhVucHanChe

	w := m.goiThan(t, http.MethodPost, hostA, duongPhanLoai(maPhieuThuong), canBoCuaXa(xaA),
		phanLoaiVao{Field: domain.LinhVucHanChe})

	doiMa(t, w, http.StatusOK)
	if ra := docPhieu(t, w.Body.Bytes()); ra.Field != domain.LinhVucHanChe {
		t.Errorf("lĩnh vực = %q, muốn %q", ra.Field, domain.LinhVucHanChe)
	}
}

// --- the restricted field on the LIST --------------------------------------------------------------

// TestDanhSachThieuQuyenHanCheThiKhongThayPhieuCanBo is the case the task brief calls out by name:
// a petition in `can-bo` must not appear for an ordinary officer — NOT EVEN IN A COUNT.
//
// IT ASSERTS BOTH HALVES. The filter that reached the store says CLOSED, and the items that came back
// contain no `can-bo` petition. Asserting only the first would stay green if the store ignored the
// flag; asserting only the second would stay green if the handler filtered in Go afterwards — which
// returns short pages while the cursor has already advanced past the rows it dropped.
func TestDanhSachThieuQuyenHanCheThiKhongThayPhieuCanBo(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, authz.Perm("feedback.read"))

	w := m.goiThan(t, http.MethodGet, hostA, duongDanhSach, canBoCuaXa(xaA), nil)
	doiMa(t, w, http.StatusOK)

	if m.danhSach.loc.ChoPhepHanChe {
		t.Error("bộ lọc mở lĩnh vực hạn chế cho tài khoản KHÔNG có feedback.restricted — " +
			"một cán bộ đọc được phiếu tố cáo chính đồng nghiệp mình")
	}
	for _, p := range docTrang(t, w.Body.Bytes()).Items {
		if p.Field == domain.LinhVucHanChe {
			t.Fatalf("phiếu lĩnh vực %q lọt vào danh sách", p.Field)
		}
	}
}

func TestDanhSachCoQuyenHanCheThiThayPhieuCanBo(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, authz.Perm("feedback.read"), QuyenHanChe)

	w := m.goiThan(t, http.MethodGet, hostA, duongDanhSach, canBoCuaXa(xaA), nil)
	doiMa(t, w, http.StatusOK)

	if !m.danhSach.loc.ChoPhepHanChe {
		t.Fatal("bộ lọc vẫn đóng dù tài khoản có feedback.restricted")
	}
	co := false
	for _, p := range docTrang(t, w.Body.Bytes()).Items {
		if p.Field == domain.LinhVucHanChe {
			co = true
		}
	}
	if !co {
		t.Error("không thấy phiếu lĩnh vực hạn chế dù có quyền — quyền cấp ra mà không mở gì là " +
			"một ô tích không làm gì trong màn Phân quyền")
	}
}

// TestDanhSachLuonCheNguoiGuiDuCoQuyenXemDayDu pins the decision stated on DanhSachPhieu.
//
// `feedback.unmask` OPENS ONE PETITION AT A TIME, on the detail route, and every such read writes an
// audit entry (rule 6, invariant 7; ADR 0030). A list cannot honour that shape — one request would
// disclose twenty reporters — so the list masks WHATEVER the caller holds.
//
// ĐỘT BIẾN: đổi `false` thành `true` ở lời gọi phieuRaNgoai trong DanhSachPhieu và ca này ĐỎ.
func TestDanhSachLuonCheNguoiGuiDuCoQuyenXemDayDu(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, authz.Perm("feedback.read"), QuyenXemDayDu)

	w := m.goiThan(t, http.MethodGet, hostA, duongDanhSach, canBoCuaXa(xaA), nil)
	doiMa(t, w, http.StatusOK)

	trang := docTrang(t, w.Body.Bytes())
	if len(trang.Items) == 0 {
		t.Fatal("danh sách rỗng — không có gì để kiểm")
	}
	for _, p := range trang.Items {
		if p.ReporterPhone == "" {
			continue
		}
		if !bytes.ContainsRune([]byte(p.ReporterPhone), '*') {
			t.Errorf("số điện thoại ra nguyên vẹn trên danh sách: một yêu cầu làm lộ nhiều người "+
				"gửi mà luật 6 bất biến 7 không ghi vết được từng lần (giá trị có %d ký tự)",
				len(p.ReporterPhone))
		}
	}
	if len(m.vet.ghi) != 0 {
		t.Errorf("danh sách ghi %d vết xem đầy đủ — hoặc nó đang lộ dữ liệu, hoặc nó đang ghi vết "+
			"cho một lần đọc không lộ gì", len(m.vet.ghi))
	}
}

// --- closing without a readable result ---------------------------------------------------------------

// TestDongPhieuKhongCoKetQuaThi400 — rule 10, invariant 6: never close silently.
//
// THE REFUSAL ITSELF IS THE USE CASE'S AND IS PROVED IN internal/app, over the real store. What is
// under test HERE is the half that belongs to this layer and is just as easy to get wrong: the
// mapping. Both of these sentinels are refusals of what the officer TYPED, so both are 400 — and a
// default of "anything I do not recognise is a 500" would tell an operator the server is broken while
// an officer stares at a form they can fix.
func TestDongPhieuKhongCoKetQuaThi400(t *testing.T) {
	for ten, loi := range map[string]error{
		"rỗng":     domain.ErrThieuKetQua,
		"quá ngắn": domain.ErrKetQuaQuaNgan,
	} {
		t.Run(ten, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(t, authz.Perm("feedback.resolve"))
			m.xuLy.loi = loi

			w := m.goiThan(t, http.MethodPost, hostA, duongDong(maPhieuThuong), canBoCuaXa(xaA),
				dongPhieuVao{Result: "xong"})

			doiMa(t, w, http.StatusBadRequest)
			if loiTra(t, w).Code != "invalid_request" {
				t.Errorf("mã lỗi = %q, muốn invalid_request", loiTra(t, w).Code)
			}
		})
	}
}

// TestDongPhieuCoKetQuaThiKetQuaDiNguyenVenXuongUseCase — the result must arrive UNCHANGED. A handler
// that trimmed, truncated or re-encoded it would alter the sentence a citizen reads.
func TestDongPhieuCoKetQuaThiKetQuaDiNguyenVenXuongUseCase(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, authz.Perm("feedback.resolve"))

	w := m.goiThan(t, http.MethodPost, hostA, duongDong(maPhieuThuong), canBoCuaXa(xaA),
		dongPhieuVao{Result: ketQuaThat})
	doiMa(t, w, http.StatusOK)

	if m.xuLy.ketQua != ketQuaThat {
		t.Errorf("kết quả xuống use case = %q, muốn nguyên văn của người gõ", m.xuLy.ketQua)
	}
}

// --- a state refusal is 409, never 403 --------------------------------------------------------------

// TestTuyenXuLyPhieuDaChuyenTrangThi409 — the caller HOLDS the permission. What is refused is this act
// on THIS petition. A 403 would send an officer to the Phân quyền screen to be granted a right they
// already have.
func TestTuyenXuLyPhieuDaChuyenTrangThi409(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, authz.Perm("feedback.classify"))
	m.xuLy.loi = petstore.ErrPhieuDaChuyenTrang

	w := m.goiThan(t, http.MethodPost, hostA, duongPhanLoai(maPhieuThuong), canBoCuaXa(xaA),
		phanLoaiVao{Field: "rac-thai"})

	doiMa(t, w, http.StatusConflict)
	if loiTra(t, w).Code != "petition_state" {
		t.Errorf("mã lỗi = %q, muốn petition_state", loiTra(t, w).Code)
	}
}

// TestDongSaiBuocThi409 — closing a petition WITH a citizen from `da-xu-ly` (or any status the
// lifecycle does not close from) is a refusal about the STATE of the record: 409, never 400 or 403.
// Which petitions DongDuoc refuses is proved over the real store in internal/app.
func TestDongSaiBuocThi409(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, authz.Perm("feedback.resolve"))
	m.xuLy.loi = fmt.Errorf("xu_ly_phan_anh: đóng phiếu cho xã %s: %w", xaA, domain.ErrDongSaiLuc)

	w := m.goiThan(t, http.MethodPost, hostA, duongDong(maPhieuThuong), canBoCuaXa(xaA),
		dongPhieuVao{Result: ketQuaThat})

	doiMa(t, w, http.StatusConflict)
	if loiTra(t, w).Code != "petition_state" {
		t.Errorf("mã lỗi = %q, muốn petition_state", loiTra(t, w).Code)
	}
}

// TestTuChoiXuLyCauCoDinhKhongLoNoiBo — every refusal of the write routes answers its FIXED sentence,
// with its machine code unchanged, and the wire carries neither the commune's tenant id nor this
// service's wrapping. The error is wrapped EXACTLY as app.bocPhieu wraps it, because the wrapped form
// is what reaches the handler in production — a bare sentinel here would pass against the old code.
//
// AND THE FULL CHAIN IS IN THE LOG, with the officer's business code and without the lookup code:
// an operator must still be able to see which commune and which act refused (rule 3, invariant 2).
func TestTuChoiXuLyCauCoDinhKhongLoNoiBo(t *testing.T) {
	cacCa := []struct {
		goc    error
		status int
		ma     string
	}{
		{domain.ErrPhanCongSaiLuc, http.StatusConflict, "petition_state"},
		{domain.ErrDongSaiLuc, http.StatusConflict, "petition_state"},
		{domain.ErrKetThucNhanhSaiLuc, http.StatusConflict, "petition_state"},
		{domain.ErrKhongConCamKet, http.StatusConflict, "petition_state"},
		{domain.ErrThieuLinhVuc, http.StatusBadRequest, "invalid_request"},
		{domain.ErrLinhVucSaiDang, http.StatusBadRequest, "invalid_request"},
		{domain.ErrThieuBoPhan, http.StatusBadRequest, "invalid_request"},
		{domain.ErrBoPhanQuaDai, http.StatusBadRequest, "invalid_request"},
		{domain.ErrCanBoQuaDai, http.StatusBadRequest, "invalid_request"},
		{domain.ErrThieuKetQua, http.StatusBadRequest, "invalid_request"},
		{domain.ErrKetQuaQuaNgan, http.StatusBadRequest, "invalid_request"},
		{domain.ErrKetQuaQuaDai, http.StatusBadRequest, "invalid_request"},
		{domain.ErrThieuLyDo, http.StatusBadRequest, "invalid_request"},
		{domain.ErrLyDoQuaNgan, http.StatusBadRequest, "invalid_request"},
		{domain.ErrLyDoQuaDai, http.StatusBadRequest, "invalid_request"},
		{domain.ErrThieuCoQuanNhan, http.StatusBadRequest, "invalid_request"},
		{domain.ErrCoQuanNhanQuaDai, http.StatusBadRequest, "invalid_request"},
		{domain.ErrThieuGhiChu, http.StatusBadRequest, "invalid_request"},
		{domain.ErrGhiChuQuaDai, http.StatusBadRequest, "invalid_request"},
	}
	if len(cacCa) != len(cacCauTuChoiPhieu) {
		t.Fatalf("bảng câu có %d dòng, bài kiểm có %d — một từ chối mới phải có mặt ở cả hai",
			len(cacCauTuChoiPhieu), len(cacCa))
	}
	for _, ca := range cacCa {
		t.Run(ca.goc.Error(), func(t *testing.T) {
			m := dungMayChu(t)
			var nhatKy bytes.Buffer
			m.dungLai(t, func(d *Deps) { d.Log = slog.New(slog.NewTextHandler(&nhatKy, nil)) })
			m.capQuyen(t, authz.Perm("feedback.resolve"))
			// app.bocPhieu's shape, character for character, around the domain's own wrapping.
			m.xuLy.loi = fmt.Errorf("xu_ly_phan_anh: đóng phiếu cho xã %s: %w", xaA,
				fmt.Errorf("%w (tối đa 1 ký tự)", ca.goc))

			w := m.goiThan(t, http.MethodPost, hostA, duongDong(maPhieuThuong), canBoCuaXa(xaA),
				dongPhieuVao{Result: ketQuaThat})

			doiMa(t, w, ca.status)
			loi := loiTra(t, w)
			if loi.Code != ca.ma {
				t.Errorf("mã lỗi = %q, muốn %q", loi.Code, ca.ma)
			}
			muon, co := cauTuChoiPhieu(ca.goc)
			if !co {
				t.Fatalf("không có câu cố định cho %v", ca.goc)
			}
			if loi.Message != muon {
				t.Errorf("message = %q, muốn câu cố định %q", loi.Message, muon)
			}
			for _, cam := range []string{string(xaA), "xu_ly_phan_anh", "phan_anh:", "cho xã",
				ca.goc.Error()} {
				if strings.Contains(w.Body.String(), cam) {
					t.Errorf("thân lỗi lộ %q: %s", cam, w.Body.String())
				}
			}
			log := nhatKy.String()
			if !strings.Contains(log, string(xaA)) || !strings.Contains(log, maCanBo) {
				t.Errorf("nhật ký thiếu xã hoặc mã cán bộ: %s", log)
			}
			if strings.Contains(log, maPhieuThuong) || strings.Contains(log, idCanBo) {
				t.Errorf("nhật ký chứa mã tra cứu hoặc id nội bộ: %s", log)
			}
		})
	}
}

// TestTimPhieuQuaDaiKhongLoTienTo — the list route's one store-owned refusal is re-worded; the
// store sentinel's `phieu_phan_anh:` prefix does not reach the screen.
func TestTimPhieuQuaDaiKhongLoTienTo(t *testing.T) {
	m := dungMayChu(t)
	w := m.goi(t, http.MethodGet, hostA,
		"/api/v1/citizen-reports?q="+strings.Repeat("a", petstore.TimPhieuToiDa+1), canBoCuaXa(xaA))

	doiMa(t, w, http.StatusBadRequest)
	loi := loiTra(t, w)
	if loi.Code != "invalid_request" || strings.Contains(loi.Message, "phieu_phan_anh") {
		t.Errorf("lỗi = %+v", loi)
	}
}

// TestPhanLoaiXaChuaCauHinhSLAThi409 is the ORDINARY answer in every commune today: `sla` is empty
// everywhere and the onboarding step that fills it does not exist in this repository.
//
// THE SENTENCE MUST NAME THE SCREEN. A fallback deadline is refused outright (rule 10, forbidden #3),
// so the only useful thing to say is which configuration is missing.
func TestPhanLoaiXaChuaCauHinhSLAThi409(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, authz.Perm("feedback.classify"))
	m.xuLy.loi = app.ErrChuaAnDinhDuocHanXuLy

	w := m.goiThan(t, http.MethodPost, hostA, duongPhanLoai(maPhieuThuong), canBoCuaXa(xaA),
		phanLoaiVao{Field: "rac-thai"})

	doiMa(t, w, http.StatusConflict)
	if loiTra(t, w).Code != "sla_chua_cau_hinh" {
		t.Errorf("mã lỗi = %q, muốn sla_chua_cau_hinh", loiTra(t, w).Code)
	}
}

// TestTuyenXuLyPhieuKhongTonTaiThi404 — and it is the SAME 404 an unknown code answers, for the same
// reason Handler.khongTimThay gives: telling the causes apart tells somebody trying codes how close
// they are.
func TestTuyenXuLyPhieuKhongTonTaiThi404(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, authz.Perm("feedback.assign"))
	m.xuLy.loi = petstore.ErrPhieuKhongTonTai

	w := m.goiThan(t, http.MethodPost, hostA, duongPhanCong(maPhieuThuong), canBoCuaXa(xaA),
		phanCongVao{Unit: "bp-001"})

	doiMa(t, w, http.StatusNotFound)
}

// --- the assignee check (user decision 2026-09-25) ------------------------------------------------

// TestPhanCongCanBoKhongNhanDuocViecThi400 — ONE sentence for every reason identity answers "absent",
// and the sentence neither echoes the code nor names a reason.
func TestPhanCongCanBoKhongNhanDuocViecThi400(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, authz.Perm("feedback.assign"))
	m.xuLy.loi = app.ErrCanBoKhongNhanDuocViec

	w := m.goiThan(t, http.MethodPost, hostA, duongPhanCong(maPhieuThuong), canBoCuaXa(xaA),
		phanCongVao{Unit: "bp-001", Assignee: "CB-XA-KHAC-01"})

	doiMa(t, w, http.StatusBadRequest)
	than := w.Body.String()
	for _, cam := range []string{"CB-XA-KHAC-01", "khoá", "khóa", "xã khác", "tài khoản", "không tồn tại"} {
		if strings.Contains(than, cam) {
			t.Errorf("câu trả lời lộ %q — năm lý do phải là MỘT câu: %s", cam, than)
		}
	}
}

// TestPhanCongChuaKiemDuocCanBoThi503 — the check did not happen: retryable, never 400 ("bad
// assignee") and never 500 (this service is healthy).
func TestPhanCongChuaKiemDuocCanBoThi503(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, authz.Perm("feedback.assign"))
	m.xuLy.loi = fmt.Errorf("%w: %w", app.ErrChuaKiemDuocCanBo, errors.New("rpc error: code = Unavailable"))

	w := m.goiThan(t, http.MethodPost, hostA, duongPhanCong(maPhieuThuong), canBoCuaXa(xaA),
		phanCongVao{Unit: "bp-001", Assignee: "CB-00999"})

	doiMa(t, w, http.StatusServiceUnavailable)
	if loiTra(t, w).Code != "assignee_check_unavailable" {
		t.Errorf("mã lỗi = %q, muốn assignee_check_unavailable", loiTra(t, w).Code)
	}
}

// --- the list's filters ---------------------------------------------------------------------------

// TestDanhSachLocKhongHopLeThiTuChoi — a filter silently dropped returns the WHOLE register to a
// screen that asked for one slice of it, and the screen has no way to know.
func TestDanhSachLocKhongHopLeThiTuChoi(t *testing.T) {
	for ten, truyVan := range map[string]string{
		"trạng thái lạ": "?status=received",
		"kênh lạ":       "?channel=sms",
		"late=1":        "?late=1",
	} {
		t.Run(ten, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(t, authz.Perm("feedback.read"))

			w := m.goiThan(t, http.MethodGet, hostA, duongDanhSach+truyVan, canBoCuaXa(xaA), nil)

			doiMa(t, w, http.StatusBadRequest)
			if m.danhSach.goi != 0 {
				t.Error("đã chạy truy vấn dù bộ lọc bị từ chối")
			}
		})
	}
}

// TestDanhSachLocHopLeXuongDungKhoaBoLoc — every accepted filter must arrive at the store as the
// field it names. A filter read into the wrong field returns a plausible page of the wrong rows.
func TestDanhSachLocHopLeXuongDungKhoaBoLoc(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, authz.Perm("feedback.read"))

	w := m.goiThan(t, http.MethodGet, hostA,
		duongDanhSach+"?status=dang-phan-loai&channel=zalo-oa&field=rac-thai&hamlet=thon-1&unit=bp-9&late=true&q=%C4%91%E1%BA%A7u%20ng%C3%B5",
		canBoCuaXa(xaA), nil)
	doiMa(t, w, http.StatusOK)

	loc := m.danhSach.loc
	switch {
	case loc.TrangThai != "dang-phan-loai":
		t.Errorf("trạng thái = %q", loc.TrangThai)
	case loc.Kenh != "zalo-oa":
		t.Errorf("kênh = %q", loc.Kenh)
	case loc.LinhVuc != "rac-thai":
		t.Errorf("lĩnh vực = %q", loc.LinhVuc)
	case loc.ThonID != "thon-1":
		t.Errorf("thôn = %q", loc.ThonID)
	case loc.BoPhanID != "bp-9":
		t.Errorf("bộ phận = %q", loc.BoPhanID)
	case !loc.ChiTreHan:
		t.Error("bộ lọc trễ hạn không tới kho")
	case loc.Tim != "đầu ngõ":
		t.Errorf("chuỗi tìm = %q", loc.Tim)
	}
}

// --- "Giao cho tôi": scope=mine ----------------------------------------------------------------------
//
// THE ONE PROPERTY EVERY CASE BELOW TURNS ON: the officer code that filters the register comes from the
// SESSION (`Principal.Ma`), never from the request. `idCanBo` and `maCanBo` differ in the fixtures, so
// a handler that handed down the internal id would match nothing and fail here too.

// soGiaoViec is a commune-A register with rows assigned to the caller, to another officer, to nobody,
// and one `can-bo` petition assigned to the caller — the row the restricted case must still exclude.
func soGiaoViec(m *mayChu) {
	m.danhSach.theo = map[tenant.ID][]domain.PhieuPhanAnh{
		xaA: {
			{MaTraCuu: "PA-CUA-TOI-1", LinhVuc: "rac-thai", CanBoXuLyID: maCanBo},
			{MaTraCuu: "PA-CUA-TOI-2", LinhVuc: "", CanBoXuLyID: maCanBo},
			{MaTraCuu: "PA-NGUOI-KHAC", LinhVuc: "rac-thai", CanBoXuLyID: "CB-00999"},
			{MaTraCuu: "PA-CHUA-GIAO", LinhVuc: "rac-thai"},
			{MaTraCuu: "PA-CAN-BO", LinhVuc: domain.LinhVucHanChe, CanBoXuLyID: maCanBo},
		},
		xaB: {
			{MaTraCuu: "PA-XA-B", LinhVuc: "rac-thai", CanBoXuLyID: maCanBo},
		},
	}
}

func maCuaTrang(t *testing.T, than []byte) map[string]bool {
	t.Helper()
	ra := map[string]bool{}
	for _, p := range docTrang(t, than).Items {
		ra[p.Code] = true
	}
	return ra
}

// TestDanhSachGiaoChoToiChiTraPhieuGiaoChoMaCuaPhien — the filter reaching the store is the caller's
// BUSINESS CODE, and the page holds only the petitions assigned to it.
//
// ĐỘT BIẾN: đổi `p.Ma` thành `p.ID` trong DanhSachPhieu và ca này ĐỎ.
func TestDanhSachGiaoChoToiChiTraPhieuGiaoChoMaCuaPhien(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, authz.Perm("feedback.read"))
	soGiaoViec(m)

	w := m.goiThan(t, http.MethodGet, hostA, duongDanhSach+"?scope=mine", canBoCuaXa(xaA), nil)
	doiMa(t, w, http.StatusOK)

	if m.danhSach.loc.CanBoXuLyID != maCanBo {
		t.Fatalf("mã lọc xuống kho = %q, muốn MÃ cán bộ của phiên %q — cột `can_bo_xu_ly_id` giữ mã "+
			"cán bộ, id nội bộ không khớp dòng nào", m.danhSach.loc.CanBoXuLyID, maCanBo)
	}
	co := maCuaTrang(t, w.Body.Bytes())
	muon := map[string]bool{"PA-CUA-TOI-1": true, "PA-CUA-TOI-2": true}
	if len(co) != len(muon) {
		t.Errorf("trang = %v, muốn đúng %v", co, muon)
	}
	for ma := range muon {
		if !co[ma] {
			t.Errorf("thiếu phiếu %s giao cho chính người gọi", ma)
		}
	}
}

// TestDanhSachKhongScopeThiKhongLocTheoNguoiGiao — "Toàn xã" is unchanged: absent or `all` hands down
// no assignee filter at all.
func TestDanhSachKhongScopeThiKhongLocTheoNguoiGiao(t *testing.T) {
	for ten, truyVan := range map[string]string{"vắng": "", "all": "?scope=all"} {
		t.Run(ten, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(t, authz.Perm("feedback.read"))

			w := m.goiThan(t, http.MethodGet, hostA, duongDanhSach+truyVan, canBoCuaXa(xaA), nil)
			doiMa(t, w, http.StatusOK)
			if m.danhSach.loc.CanBoXuLyID != "" {
				t.Errorf("toàn xã mà vẫn lọc theo người được giao %q", m.danhSach.loc.CanBoXuLyID)
			}
		})
	}
}

// TestDanhSachMaCanBoTuTruyVanBiBoQua — a caller naming somebody else's code in the query must not
// reach the store with it. Every spelling a probe would try, with and without `scope=mine`.
func TestDanhSachMaCanBoTuTruyVanBiBoQua(t *testing.T) {
	for ten, ca := range map[string]struct {
		truyVan string
		muonLoc string
	}{
		"assignee một mình":         {"?assignee=CB-00999", ""},
		"scope=mine + assignee":     {"?scope=mine&assignee=CB-00999", maCanBo},
		"scope=mine + can_bo":       {"?scope=mine&can_bo_xu_ly_id=CB-00999", maCanBo},
		"assignee đứng trước scope": {"?assignee=CB-00999&scope=mine&ma=CB-00999", maCanBo},
	} {
		t.Run(ten, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(t, authz.Perm("feedback.read"))
			soGiaoViec(m)

			w := m.goiThan(t, http.MethodGet, hostA, duongDanhSach+ca.truyVan, canBoCuaXa(xaA), nil)
			doiMa(t, w, http.StatusOK)

			if m.danhSach.loc.CanBoXuLyID != ca.muonLoc {
				t.Fatalf("mã lọc xuống kho = %q, muốn %q — mã cán bộ lấy từ truy vấn là một người đọc "+
					"sổ tự chọn xem việc của ai", m.danhSach.loc.CanBoXuLyID, ca.muonLoc)
			}
			if maCuaTrang(t, w.Body.Bytes())["PA-NGUOI-KHAC"] && ca.muonLoc != "" {
				t.Error("phiếu giao cho CB-00999 lọt vào tab Giao cho tôi")
			}
		})
	}
}

// TestDanhSachScopeKhongHopLeThi400 — `related` is undecided with the customer and refused with its
// reason; anything else is refused as unknown. Neither reaches the store.
func TestDanhSachScopeKhongHopLeThi400(t *testing.T) {
	for _, truyVan := range []string{"?scope=related", "?scope=assigned", "?scope=MINE", "?scope=CB-00123"} {
		t.Run(truyVan, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(t, authz.Perm("feedback.read"))

			w := m.goiThan(t, http.MethodGet, hostA, duongDanhSach+truyVan, canBoCuaXa(xaA), nil)

			doiMa(t, w, http.StatusBadRequest)
			if m.danhSach.goi != 0 {
				t.Error("đã chạy truy vấn dù `scope` bị từ chối")
			}
		})
	}
}

// TestDanhSachGiaoChoToiKhongCoMaCanBoThiTuChoi — FAIL CLOSED. Dropping the filter would answer the
// "Giao cho tôi" tab with the whole commune register.
func TestDanhSachGiaoChoToiKhongCoMaCanBoThiTuChoi(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, authz.Perm("feedback.read"))

	p := canBoCuaXa(xaA)
	p.Ma = ""
	w := m.goiThan(t, http.MethodGet, hostA, duongDanhSach+"?scope=mine", p, nil)

	doiMa(t, w, http.StatusInternalServerError)
	if m.danhSach.goi != 0 {
		t.Error("đã đọc sổ dù phiên không có mã cán bộ — tab Giao cho tôi sẽ hiện cả sổ của xã")
	}
}

// TestDanhSachGiaoChoToiVanLoaiLinhVucHanChe — the two filters compose: a `can-bo` petition assigned to
// the caller still does not appear without `feedback.restricted`, and does with it.
func TestDanhSachGiaoChoToiVanLoaiLinhVucHanChe(t *testing.T) {
	t.Run("không có feedback.restricted", func(t *testing.T) {
		m := dungMayChu(t)
		m.capQuyen(t, authz.Perm("feedback.read"))
		soGiaoViec(m)

		w := m.goiThan(t, http.MethodGet, hostA, duongDanhSach+"?scope=mine", canBoCuaXa(xaA), nil)
		doiMa(t, w, http.StatusOK)

		if m.danhSach.loc.ChoPhepHanChe {
			t.Error("scope=mine mở lĩnh vực hạn chế cho tài khoản không có feedback.restricted")
		}
		if maCuaTrang(t, w.Body.Bytes())["PA-CAN-BO"] {
			t.Error("phiếu lĩnh vực can-bo lọt vào tab Giao cho tôi")
		}
	})
	t.Run("có feedback.restricted", func(t *testing.T) {
		m := dungMayChu(t)
		m.capQuyen(t, authz.Perm("feedback.read"), QuyenHanChe)
		soGiaoViec(m)

		w := m.goiThan(t, http.MethodGet, hostA, duongDanhSach+"?scope=mine", canBoCuaXa(xaA), nil)
		doiMa(t, w, http.StatusOK)

		if !maCuaTrang(t, w.Body.Bytes())["PA-CAN-BO"] {
			t.Error("không thấy phiếu can-bo giao cho mình dù có feedback.restricted")
		}
	})
}

// docTrang decodes one page of the register.
func docTrang(t *testing.T, than []byte) page.Result[phieuPhanAnhRa] {
	t.Helper()
	var ra page.Result[phieuPhanAnhRa]
	if err := json.Unmarshal(than, &ra); err != nil {
		t.Fatalf("thân phản hồi không phải một trang: %q", string(than))
	}
	return ra
}
