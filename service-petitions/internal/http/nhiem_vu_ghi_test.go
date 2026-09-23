package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/vihat/vigov/core/idem"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/app"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	petstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// Tests for the six STAFF WRITE routes of the task register.
//
//	PROVED HERE   each route's four cases (rule 5, invariant 7): 401 no session · 403 wrong
//	              permission · 401 right permission WRONG COMMUNE · 2xx both correct — and in the
//	              first three cases the use case is NOT REACHED, which is what proves the guard runs
//	              before any data is touched · the acting principal reaches the use case as its
//	              BUSINESS CODE · the `task.approve` fact is read from THAT key and handed down ·
//	              ADR 0038's refusals answer 403 and the tree refusals answer 409 · a decision body
//	              that is neither `approve` nor `reject` is refused rather than read as a rejection.
//
//	NOT PROVED    the transaction boundaries, the tree walks and the two ADRs themselves. Those are
//	              properties of internal/app over the real store, and they live in
//	              internal/app/nhiem_vu_test.go.

// --- the fake -------------------------------------------------------------------------------------

// ghiNhiemVuGia is the six acts. IT RECORDS THE COMMUNE AND THE ACTOR of every call, because those
// are the two things a route can get wrong in a way no status code shows.
type ghiNhiemVuGia struct {
	goi   int
	viec  string
	xa    tenant.ID
	nguoi audit.Actor
	maDa  string

	ycTao       app.YeuCauTaoNhiemVu
	ycSua       petstore.SuaNhiemVu
	ycTrangT    app.YeuCauDoiTrangThai
	ycDeNghi    app.YeuCauDeNghiLuiHan
	ycQuyetDinh app.YeuCauQuyetDinhLuiHan
	lyDoXoa     string
	deNghiID    string

	// duyet is the fact the STATUS route hands down: does the caller hold `task.approve`?
	//
	// RECORDED RATHER THAN ACTED ON. Which moves it refuses is app.duocHoanThanh's and is proved over
	// the real store in internal/app; what this layer can get wrong — and what is asserted here — is
	// WHICH FACT is handed down and whether it is read from the right key.
	duyet app.QuyenDuyetHoanThanh

	loi error
}

func (g *ghiNhiemVuGia) ghi(ctx context.Context, viec, ma string, nguoi audit.Actor) {
	g.goi++
	g.viec, g.maDa, g.nguoi = viec, ma, nguoi
	g.xa = tenant.MustFrom(ctx)
}

func (g *ghiNhiemVuGia) tra() (domain.NhiemVu, error) {
	if g.loi != nil {
		return domain.NhiemVu{}, g.loi
	}
	return domain.NhiemVu{
		ID: "nv-001", Ma: "NV19", Loai: "theo-van-ban", TieuDe: "Rà soát tiến độ tuyến đường",
		TrangThai: domain.DangThucHien, NguonGiao: domain.NguonTrucTiep,
		LanhDaoGiaoViecMa: "CB-00007", NguoiTaoMa: maCanBo,
	}, nil
}

func (g *ghiNhiemVuGia) traDeNghi() (domain.DeNghiLuiHan, error) {
	if g.loi != nil {
		return domain.DeNghiLuiHan{}, g.loi
	}
	return domain.DeNghiLuiHan{
		ID: "dn-001", NhiemVuID: "nv-001", NguoiDeNghiMa: maCanBo,
		HanMoi: time.Date(2026, 8, 20, 9, 30, 0, 0, time.UTC), LyDo: "Chờ số liệu.",
		TrangThai: domain.ChoDuyetLuiHan, ThoiDiem: time.Date(2026, 7, 1, 2, 0, 0, 0, time.UTC),
	}, nil
}

func (g *ghiNhiemVuGia) Tao(ctx context.Context, yc app.YeuCauTaoNhiemVu, nguoi audit.Actor) (
	domain.NhiemVu, error) {
	g.ghi(ctx, "tao", "", nguoi)
	g.ycTao = yc
	return g.tra()
}

func (g *ghiNhiemVuGia) Sua(ctx context.Context, ma string, sua petstore.SuaNhiemVu,
	nguoi audit.Actor) (domain.NhiemVu, error) {
	g.ghi(ctx, "sua", ma, nguoi)
	g.ycSua = sua
	return g.tra()
}

func (g *ghiNhiemVuGia) DoiTrangThai(ctx context.Context, ma string, yc app.YeuCauDoiTrangThai,
	nguoi audit.Actor, duyet app.QuyenDuyetHoanThanh) (domain.NhiemVu, error) {
	g.ghi(ctx, "trang-thai", ma, nguoi)
	g.ycTrangT, g.duyet = yc, duyet
	return g.tra()
}

func (g *ghiNhiemVuGia) Xoa(ctx context.Context, ma, lyDo string, nguoi audit.Actor) error {
	g.ghi(ctx, "xoa", ma, nguoi)
	g.lyDoXoa = lyDo
	return g.loi
}

func (g *ghiNhiemVuGia) DeNghiLuiHan(ctx context.Context, ma string, yc app.YeuCauDeNghiLuiHan,
	nguoi audit.Actor) (domain.DeNghiLuiHan, error) {
	g.ghi(ctx, "de-nghi", ma, nguoi)
	g.ycDeNghi = yc
	return g.traDeNghi()
}

func (g *ghiNhiemVuGia) QuyetDinhLuiHan(ctx context.Context, ma, deNghiID string,
	yc app.YeuCauQuyetDinhLuiHan, nguoi audit.Actor) (domain.DeNghiLuiHan, error) {
	g.ghi(ctx, "quyet-dinh", ma, nguoi)
	g.deNghiID, g.ycQuyetDinh = deNghiID, yc
	return g.traDeNghi()
}

// --- fixtures --------------------------------------------------------------------------------------

const duongTasks = "/api/v1/tasks"

func duongNV(ma string) string          { return duongTasks + "/" + ma }
func duongTrangThaiNV(ma string) string { return duongNV(ma) + "/status" }
func duongDeNghi(ma string) string      { return duongNV(ma) + "/extensions" }
func duongQuyetDinh(ma, id string) string {
	return duongDeNghi(ma) + "/" + id + "/decision"
}

const maNVThu = "NV19"

func thanTaoNV() taoNhiemVuVao {
	return taoNhiemVuVao{AutoCode: true, Type: "theo-van-ban", Title: "Rà soát tiến độ tuyến đường"}
}

// goiGhiNV issues a request to a task WRITE route, ALWAYS carrying an Idempotency-Key.
//
// POST /api/v1/tasks declares idem.Required, which refuses a request without the header BEFORE it
// reaches the handler — so a harness that omitted it would turn every create assertion into an
// assertion about the header. The other five routes declare idem.KhongCan and ignore it, which is
// why one helper serves all six: a test should not have to know which declaration a route carries
// to assert what the route DOES.
func (m *mayChu) goiGhiNV(t *testing.T, method, host, path string, p *authz.Principal,
	than any) *httptest.ResponseRecorder {
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
	}
	r.Host = host
	r.RemoteAddr = "10.0.0.7:51000"
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set(idem.Header, "01JIDEMPOTENCYKEYCUATEST")
	if p != nil {
		r = r.WithContext(authz.Into(r.Context(), *p))
	}
	w := httptest.NewRecorder()
	m.h.ServeHTTP(w, r)
	return w
}

// --- rule 5, invariant 7: four cases per route -------------------------------------------------------
//
// ONE TABLE FOR THE SIX ROUTES, and the table is the point rather than a convenience: written out
// six times, one route's 403 case quietly uses the key that route does not need. Each row names the
// method, the path, the body, THE KEY THAT MUST OPEN IT, and a key of this subsystem that must NOT.

type caGhiNhiemVu struct {
	ten     string
	method  string
	duong   string
	than    any
	khoa    authz.Perm
	khoaSai authz.Perm
	ok      int
}

func caCacTuyenGhiNhiemVu() []caGhiNhiemVu {
	return []caGhiNhiemVu{
		// `khoaSai` IS A REAL KEY OF THIS SUBSYSTEM, never a nonsense string: a 403 against
		// `feedback.read` would also pass if the route checked nothing at all against the task keys.
		{"giao việc mới", http.MethodPost, duongTasks, thanTaoNV(),
			authz.Perm("task.create"), authz.Perm("task.read"), http.StatusCreated},
		{"sửa", http.MethodPatch, duongNV(maNVThu), suaNhiemVuVao{},
			authz.Perm("task.update"), authz.Perm("task.create"), http.StatusOK},
		{"chuyển trạng thái", http.MethodPost, duongTrangThaiNV(maNVThu),
			doiTrangThaiVao{Status: string(domain.ChoDuyet)},
			authz.Perm("task.update"), authz.Perm("task.approve"), http.StatusOK},
		{"xoá", http.MethodDelete, duongNV(maNVThu), xoaNhiemVuVao{Reason: "Trùng với NV05."},
			authz.Perm("task.delete"), authz.Perm("task.update"), http.StatusNoContent},
		// ⚠ FILING AN EXTENSION IS `task.update`, NOT `task.extend` — ADR 0038: the second key is
		// "Duyệt gia hạn", the right to DECIDE. A route guarded by it would mean only people who can
		// approve an extension may ask for one.
		{"đề nghị lùi hạn", http.MethodPost, duongDeNghi(maNVThu),
			deNghiLuiHanVao{NewDueAt: time.Date(2026, 8, 20, 9, 30, 0, 0, time.UTC), Reason: "Chờ số liệu."},
			authz.Perm("task.update"), authz.Perm("task.extend"), http.StatusCreated},
		{"quyết định lùi hạn", http.MethodPost, duongQuyetDinh(maNVThu, "dn-001"),
			quyetDinhLuiHanVao{Decision: "approve"},
			authz.Perm("task.extend"), authz.Perm("task.update"), http.StatusOK},
	}
}

func TestTuyenGhiNhiemVuKhongCoPhienThi401(t *testing.T) {
	for _, ca := range caCacTuyenGhiNhiemVu() {
		t.Run(ca.ten, func(t *testing.T) {
			m := dungMayChu(t)
			w := m.goiGhiNV(t, ca.method, hostA, ca.duong, nil, ca.than)

			doiMa(t, w, http.StatusUnauthorized)
			// THE COUNT IS THE ASSERTION. A route that refused only AFTER touching the data would
			// still have written to a commune's register.
			if m.ghiNhiemVu.goi != 0 {
				t.Errorf("đã chạm dữ liệu dù chưa có phiên (%d lần)", m.ghiNhiemVu.goi)
			}
		})
	}
}

func TestTuyenGhiNhiemVuSaiQuyenThi403(t *testing.T) {
	for _, ca := range caCacTuyenGhiNhiemVu() {
		t.Run(ca.ten, func(t *testing.T) {
			m := dungMayChu(t)
			// An account of commune A holding a REAL key of this subsystem — just not this route's.
			// Rule 5, invariant 3b: these rights are not a Cartesian product, and `task.approve` must
			// not open the route that MOVES work, nor `task.update` the one that DECIDES an extension.
			m.capQuyen(t, ca.khoaSai)

			w := m.goiGhiNV(t, ca.method, hostA, ca.duong, canBoCuaXa(xaA), ca.than)

			doiMa(t, w, http.StatusForbidden)
			if m.ghiNhiemVu.goi != 0 {
				t.Errorf("đã chạm dữ liệu dù sai quyền (%d lần)", m.ghiNhiemVu.goi)
			}
		})
	}
}

// TestTuyenGhiNhiemVuDungQuyenSaiXaThi401 is the case no test of a single commune can produce.
//
// IT ANSWERS 401 AND NOT 403, and that is authz.xacNhanXa's behaviour rather than a slip: the
// commune on the principal is compared with the commune resolved from Host BEFORE the permission is
// consulted, so a token from another commune never reaches the permission check. The property being
// asserted is that NO DATA IS TOUCHED — a leak between two public authorities is heavier than a
// software bug (rule 1).
func TestTuyenGhiNhiemVuDungQuyenSaiXaThi401(t *testing.T) {
	for _, ca := range caCacTuyenGhiNhiemVu() {
		t.Run(ca.ten, func(t *testing.T) {
			m := dungMayChu(t)
			// The account holds the RIGHT key — in commune A. The request arrives at commune B's host.
			m.capQuyen(t, ca.khoa)

			w := m.goiGhiNV(t, ca.method, hostB, ca.duong, canBoCuaXa(xaA), ca.than)

			doiMa(t, w, http.StatusUnauthorized)
			if m.ghiNhiemVu.goi != 0 {
				t.Errorf("đã GHI vào sổ nhiệm vụ của xã B bằng phiên của xã A (%d lần) — "+
					"đây là rò rỉ giữa hai cơ quan nhà nước, không phải một lỗi phần mềm thường",
					m.ghiNhiemVu.goi)
			}
		})
	}
}

func TestTuyenGhiNhiemVuDungQuyenDungXaThiQua(t *testing.T) {
	for _, ca := range caCacTuyenGhiNhiemVu() {
		t.Run(ca.ten, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(t, ca.khoa)

			w := m.goiGhiNV(t, ca.method, hostA, ca.duong, canBoCuaXa(xaA), ca.than)

			doiMa(t, w, ca.ok)
			if m.ghiNhiemVu.goi != 1 {
				t.Fatalf("gọi use case %d lần, muốn 1", m.ghiNhiemVu.goi)
			}
			// THE COMMUNE THE USE CASE SAW IS THE ONE RESOLVED FROM Host, never one a client named.
			if m.ghiNhiemVu.xa != xaA {
				t.Errorf("use case thấy xã %q, muốn %q", m.ghiNhiemVu.xa, xaA)
			}
		})
	}
}

// --- the acting person reaches the use case as a BUSINESS CODE ----------------------------------------

// TestTuyenGhiNhiemVuChuTheLaMaCanBo pins rule 6, invariant 8 at the boundary it is broken at.
//
// `idCanBo` AND `maCanBo` ARE DIFFERENT STRINGS IN THE FIXTURES on purpose. A handler passing
// `Principal.ID` would produce a perfectly valid-looking audit entry naming nobody — which is
// exactly what six write paths in this repository did until 2026-09-22, with no test turning red.
func TestTuyenGhiNhiemVuChuTheLaMaCanBo(t *testing.T) {
	for _, ca := range caCacTuyenGhiNhiemVu() {
		t.Run(ca.ten, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(t, ca.khoa)

			m.goiGhiNV(t, ca.method, hostA, ca.duong, canBoCuaXa(xaA), ca.than)

			if m.ghiNhiemVu.nguoi.ID != maCanBo {
				t.Errorf("chủ thể = %q, muốn mã cán bộ %q — một ULID ở cột này không gọi tên ai",
					m.ghiNhiemVu.nguoi.ID, maCanBo)
			}
			if m.ghiNhiemVu.nguoi.Kind != "staff" {
				t.Errorf("loại chủ thể = %q, muốn staff", m.ghiNhiemVu.nguoi.Kind)
			}
			if m.ghiNhiemVu.nguoi.IP == "" {
				t.Error("vết không mang địa chỉ IP — luật 6 bất biến 2 đòi 'từ IP nào'")
			}
		})
	}
}

// --- the second key on the status route ----------------------------------------------------------------

// TestDoiTrangThai_ChiCoTaskUpdateThiQuaCongVoiQuyenDuyetLaFalse proves the gate and the second key
// are two different questions: holding `task.update` opens the route, and the fact handed down says
// this account may NOT sign work off.
func TestDoiTrangThai_ChiCoTaskUpdateThiQuaCongVoiQuyenDuyetLaFalse(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, authz.Perm("task.update"))

	w := m.goiGhiNV(t, http.MethodPost, hostA, duongTrangThaiNV(maNVThu), canBoCuaXa(xaA),
		doiTrangThaiVao{Status: string(domain.ChoDuyet)})

	doiMa(t, w, http.StatusOK)
	if m.ghiNhiemVu.duyet {
		t.Error("quyền duyệt hoàn thành = true dù tài khoản không có `task.approve`")
	}
}

func TestDoiTrangThai_CoTaskApproveThiQuyenDuyetLaTrue(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, authz.Perm("task.update"), authz.Perm("task.approve"))

	w := m.goiGhiNV(t, http.MethodPost, hostA, duongTrangThaiNV(maNVThu), canBoCuaXa(xaA),
		doiTrangThaiVao{Status: string(domain.HoanThanh)})

	doiMa(t, w, http.StatusOK)
	if !m.ghiNhiemVu.duyet {
		t.Error("quyền duyệt hoàn thành = false dù tài khoản có `task.approve`")
	}
}

// TestDoiTrangThai_KhongDocTuKhoaQuyenLangGieng keeps the handler honest about WHICH key it reads.
// An account holding every neighbouring task key but not `task.approve` must still be told no.
func TestDoiTrangThai_KhongDocTuKhoaQuyenLangGieng(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, authz.Perm("task.update"), authz.Perm("task.extend"),
		authz.Perm("task.create"), authz.Perm("task.delete"), authz.Perm("task.read"))

	m.goiGhiNV(t, http.MethodPost, hostA, duongTrangThaiNV(maNVThu), canBoCuaXa(xaA),
		doiTrangThaiVao{Status: string(domain.HoanThanh)})

	if m.ghiNhiemVu.duyet {
		t.Error("đọc quyền duyệt từ một khoá khác `task.approve`")
	}
}

// --- the bodies reach the use case unchanged -------------------------------------------------------------

func TestTaoNhiemVu_ThanDiNguyenVenXuongUseCase(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, authz.Perm("task.create"))

	han := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
	vao := thanTaoNV()
	vao.DueAt = &han
	vao.Parent = "nv-cha-001"
	vao.Assigner = "CB-00007"

	doiMa(t, m.goiGhiNV(t, http.MethodPost, hostA, duongTasks, canBoCuaXa(xaA), vao),
		http.StatusCreated)

	yc := m.ghiNhiemVu.ycTao
	if !yc.TuSinhMa || yc.Loai != "theo-van-ban" || yc.NhiemVuChaID != "nv-cha-001" {
		t.Fatalf("yêu cầu tới use case = %+v", yc)
	}
	// THE DEADLINE THE FORM CARRIED, UNCHANGED. Nothing between the wire and the column derives it.
	if !yc.HanXuLy.Equal(han) {
		t.Errorf("hạn xử lý = %v, muốn %v", yc.HanXuLy, han)
	}
	if yc.LanhDaoGiaoViecMa != "CB-00007" {
		t.Errorf("lãnh đạo giao việc = %q — ADR 0038 dựa đúng vào giá trị này", yc.LanhDaoGiaoViecMa)
	}
}

// TestTaoNhiemVu_KhongCoHanThiKhongDungHanNao pins that an absent `due_at` stays absent: a zero
// instant reaching the column would be a deadline in year 1, i.e. a task overdue the moment it was
// created.
func TestTaoNhiemVu_KhongCoHanThiKhongDungHanNao(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, authz.Perm("task.create"))

	doiMa(t, m.goiGhiNV(t, http.MethodPost, hostA, duongTasks, canBoCuaXa(xaA), thanTaoNV()),
		http.StatusCreated)

	if !m.ghiNhiemVu.ycTao.HanXuLy.IsZero() {
		t.Errorf("hạn xử lý = %v, muốn rỗng khi biểu mẫu không điền hạn", m.ghiNhiemVu.ycTao.HanXuLy)
	}
}

// TestSuaNhiemVu_TruongKhongGuiThiKhongDongToi is what makes PATCH a patch: a field absent from the
// body arrives as nil, and the use case leaves the column alone. Sending zero values instead would
// blank a note nobody asked to clear.
func TestSuaNhiemVu_TruongKhongGuiThiKhongDongToi(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, authz.Perm("task.update"))

	tieuDe := "Tên mới"
	doiMa(t, m.goiGhiNV(t, http.MethodPatch, hostA, duongNV(maNVThu), canBoCuaXa(xaA),
		suaNhiemVuVao{Title: &tieuDe}), http.StatusOK)

	sua := m.ghiNhiemVu.ycSua
	if sua.TieuDe == nil || *sua.TieuDe != tieuDe {
		t.Fatalf("tiêu đề không tới use case: %+v", sua)
	}
	for ten, v := range map[string]bool{
		"mo_ta":       sua.MoTa != nil,
		"tien_do":     sua.TienDo != nil,
		"ghi_chu":     sua.GhiChu != nil,
		"muc_uu_tien": sua.MucUuTien != nil,
		"cha":         sua.NhiemVuChaID != nil,
	} {
		if v {
			t.Errorf("trường %q không gửi lên mà vẫn tới use case", ten)
		}
	}
}

func TestXoaNhiemVu_LyDoDiNguyenVenXuongUseCase(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, authz.Perm("task.delete"))

	const lyDo = "Trùng với nhiệm vụ NV05, đã gộp nội dung."
	doiMa(t, m.goiGhiNV(t, http.MethodDelete, hostA, duongNV(maNVThu), canBoCuaXa(xaA),
		xoaNhiemVuVao{Reason: lyDo}), http.StatusNoContent)

	if m.ghiNhiemVu.lyDoXoa != lyDo {
		t.Errorf("lý do xoá = %q, muốn %q", m.ghiNhiemVu.lyDoXoa, lyDo)
	}
}

func TestQuyetDinhLuiHan_MaDeNghiTrenDuongDanDiXuongUseCase(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, authz.Perm("task.extend"))

	doiMa(t, m.goiGhiNV(t, http.MethodPost, hostA, duongQuyetDinh(maNVThu, "dn-777"),
		canBoCuaXa(xaA), quyetDinhLuiHanVao{Decision: "reject"}), http.StatusOK)

	if m.ghiNhiemVu.deNghiID != "dn-777" {
		t.Errorf("mã đề nghị = %q, muốn dn-777", m.ghiNhiemVu.deNghiID)
	}
	if m.ghiNhiemVu.maDa != maNVThu {
		t.Errorf("mã nhiệm vụ = %q, muốn %q — đề nghị phải được tra theo CẶP", m.ghiNhiemVu.maDa, maNVThu)
	}
	if m.ghiNhiemVu.ycQuyetDinh.Duyet {
		t.Error("`reject` đi xuống use case thành DUYỆT")
	}
}

// TestQuyetDinhLuiHan_QuyetDinhLaKhongBietThiTuChoi refuses rather than reading an unknown value as
// a rejection: a typo that silently rejected an extension would move nothing, tell nobody why, and
// leave the officer waiting on a request that was already answered.
func TestQuyetDinhLuiHan_QuyetDinhLaKhongBietThiTuChoi(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, authz.Perm("task.extend"))

	w := m.goiGhiNV(t, http.MethodPost, hostA, duongQuyetDinh(maNVThu, "dn-001"), canBoCuaXa(xaA),
		quyetDinhLuiHanVao{Decision: "dong-y"})

	doiMa(t, w, http.StatusBadRequest)
	if e := loiTra(t, w); e.Code != "invalid_request" {
		t.Errorf("mã lỗi = %q, muốn invalid_request: %+v", e.Code, e)
	}
	if m.ghiNhiemVu.goi != 0 {
		t.Error("gọi use case với một quyết định không hợp lệ")
	}
}

// --- the refusals map onto the right status codes ---------------------------------------------------------

// TestLoiNhiemVuAnhXaDungMa is the 403 / 409 / 404 line, and it is the one worth reading twice: a
// 409 for a permission problem sends an officer to reload a task they were never allowed to move,
// and a 403 for a state problem sends them to the Phân quyền screen to be granted a right they
// already hold.
func TestLoiNhiemVuAnhXaDungMa(t *testing.T) {
	for _, ca := range []struct {
		ten  string
		loi  error
		muon int
	}{
		{"không có nhiệm vụ", petstore.ErrNhiemVuKhongTonTai, http.StatusNotFound},
		{"không có đề nghị", petstore.ErrDeNghiKhongTonTai, http.StatusNotFound},

		// ADR 0038 — about WHO is acting.
		{"không phải lãnh đạo giao việc", domain.ErrKhongPhaiLanhDaoGiaoViec, http.StatusForbidden},
		{"chưa ghi lãnh đạo giao việc", domain.ErrChuaGhiLanhDaoGiaoViec, http.StatusForbidden},
		{"tự duyệt đề nghị của mình", domain.ErrTuDuyetDeNghiCuaMinh, http.StatusForbidden},
		{"thiếu quyền duyệt hoàn thành", app.ErrKhongDuocDuyetHoanThanh, http.StatusForbidden},

		// ADR 0037 — about the RECORD.
		{"còn việc con chưa xoá", domain.LoiConChuaXoa(3), http.StatusConflict},
		{"còn việc con chưa xong", domain.LoiConChuaXong([]string{"NV20"}), http.StatusConflict},
		{"chu trình cây", domain.ErrChuTrinhCayNhiemVu, http.StatusConflict},
		{"không có nhiệm vụ cha", domain.ErrChaKhongTonTai, http.StatusConflict},

		{"mã đã dùng", petstore.ErrMaNhiemVuDaTonTai, http.StatusConflict},
		{"đã chuyển trạng thái", petstore.ErrNhiemVuDaChuyenTrang, http.StatusConflict},
		{"bước chuyển sai lúc", domain.ErrChuyenTrangThaiNhiemVuSaiLuc, http.StatusConflict},
		{"đã có đề nghị chờ duyệt", domain.ErrDaCoDeNghiChoDuyet, http.StatusConflict},
		{"nhiệm vụ chưa có hạn", domain.ErrNhiemVuChuaCoHan, http.StatusConflict},

		// About WHAT WAS SENT.
		{"thiếu tiêu đề", domain.ErrThieuTieuDeNhiemVu, http.StatusBadRequest},
		{"thiếu lý do xoá", domain.ErrThieuLyDoXoaNhiemVu, http.StatusBadRequest},

		// A FAILURE IS NOT THE CLIENT'S FAULT. Answering 400 for a database outage makes a client
		// retry with different input for ever while nobody is told the server is broken.
		{"lỗi hệ thống", errors.New("kết nối cơ sở dữ liệu hỏng"), http.StatusInternalServerError},
	} {
		t.Run(ca.ten, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(t, authz.Perm("task.update"))
			m.ghiNhiemVu.loi = ca.loi

			w := m.goiGhiNV(t, http.MethodPost, hostA, duongTrangThaiNV(maNVThu), canBoCuaXa(xaA),
				doiTrangThaiVao{Status: string(domain.ChoDuyet)})

			doiMa(t, w, ca.muon)
		})
	}
}

// TestLoiNhiemVuKhongLoNoiDungHeThongRaClient keeps a store failure's text off the wire (rule 3,
// forbidden #3): the client is told to retry, and the commune and the cause go to the log.
func TestLoiNhiemVuKhongLoNoiDungHeThongRaClient(t *testing.T) {
	m := dungMayChu(t)
	m.capQuyen(t, authz.Perm("task.delete"))
	m.ghiNhiemVu.loi = errors.New("pq: connection to 10.1.2.3:5432 refused")

	w := m.goiGhiNV(t, http.MethodDelete, hostA, duongNV(maNVThu), canBoCuaXa(xaA),
		xoaNhiemVuVao{Reason: "Trùng."})

	doiMa(t, w, http.StatusInternalServerError)
	if than := w.Body.String(); contains(than, "10.1.2.3") || contains(than, "pq:") {
		t.Errorf("thân lỗi lộ chi tiết hệ thống: %s", than)
	}
}

func contains(s, sub string) bool {
	return len(sub) > 0 && len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}
