package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/audit"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-petitions/internal/app"
	"github.com/vihat/vigov/service-petitions/internal/domain"
	dmstore "github.com/vihat/vigov/service-petitions/internal/store"
)

// GET /api/v1/task-statuses and PATCH /api/v1/task-statuses/{code} — open question #21, ADR 0035 §C.
//
// Runs on dungMayChuGhi (loai_nhiem_vu_ghi_test.go): the REAL Register behind the REAL edge chain,
// with a checker whose grants are keyed BY COMMUNE, which is what the third case of rule 5
// invariant 7 needs.

// --- fakes ------------------------------------------------------------------------------------

// docTrangThaiGia returns overrides KEYED BY THE COMMUNE IN THE CONTEXT, exactly as Scoped reads it.
// A fake ignoring the commune would let the isolation case pass while proving nothing.
type docTrangThaiGia struct {
	theoXa map[tenant.ID][]domain.NhanTrangThaiNhiemVu
	loi    error
}

func (d *docTrangThaiGia) DanhSach(ctx context.Context) ([]domain.NhanTrangThaiNhiemVu, error) {
	if d.loi != nil {
		return nil, d.loi
	}
	return d.theoXa[tenant.MustFrom(ctx)], nil
}

// ghiTrangThaiGia records the commune, the actor and the request it was called with.
type ghiTrangThaiGia struct {
	loi       error
	goi       int
	xaCuoi    tenant.ID
	nguoiCuoi audit.Actor
	maCuoi    string
	ycCuoi    app.YeuCauSuaNhanTrangThai
}

func (g *ghiTrangThaiGia) Sua(ctx context.Context, ma string, yc app.YeuCauSuaNhanTrangThai,
	nguoi audit.Actor) (domain.TrangThaiHienThi, error) {
	g.goi++
	g.xaCuoi, g.nguoiCuoi, g.maCuoi, g.ycCuoi = tenant.MustFrom(ctx), nguoi, ma, yc
	if g.loi != nil {
		return domain.TrangThaiHienThi{}, g.loi
	}
	md, _ := domain.TimMacDinhTrangThai(ma)
	return domain.HienThiMot(md, &domain.NhanTrangThaiNhiemVu{Ma: md.Ma, Nhan: "Đã sửa", ThuTu: md.ThuTu}), nil
}

// docTrangThaiMau: commune A has re-worded `tam-dung` and moved it first; commune B has re-worded
// `moi-giao`. Each commune must see ONLY its own.
func docTrangThaiMau() *docTrangThaiGia {
	return &docTrangThaiGia{theoXa: map[tenant.ID][]domain.NhanTrangThaiNhiemVu{
		xaA: {{Ma: domain.TamDung, Nhan: "Đang treo", ThuTu: 1, CapNhatBoi: "CB-00123"}},
		xaB: {{Ma: domain.MoiGiao, Nhan: "Chưa thực hiện", ThuTu: 1, CapNhatBoi: "CB-00999"}},
	}}
}

const duongTrangThai = "/api/v1/task-statuses"

func docDanhSachTrangThai(t *testing.T, body []byte) []trangThaiNhiemVuRa {
	t.Helper()
	var ra danhSachTrangThaiNhiemVuRa
	if err := json.Unmarshal(body, &ra); err != nil {
		t.Fatalf("thân không phải JSON: %q", body)
	}
	return ra.Items
}

// --- GET: AnyAuthenticated — 401 · 200 · commune isolation --------------------------------------

func TestTrangThaiNhiemVu_Doc401KhongPhien(t *testing.T) {
	m := dungMayChuGhi(t)
	doiMa(t, m.goi(t, http.MethodGet, hostA, duongTrangThai, nil), http.StatusUnauthorized)
}

func TestTrangThaiNhiemVu_Doc401PhienCuaXaKhac(t *testing.T) {
	// A principal of commune A presented at commune B's host: refused before any read.
	m := dungMayChuGhi(t)
	doiMa(t, m.goi(t, http.MethodGet, hostB, duongTrangThai, canBoGhi(xaA)), http.StatusUnauthorized)
}

func TestTrangThaiNhiemVu_Doc200BayMaGopVaCachLyXa(t *testing.T) {
	m := dungMayChuGhi(t)
	// NO PERMISSION GRANTED AT ALL: AnyAuthenticated needs none. A 403 here would mean somebody put
	// a configuration key on the read that fills every Kanban header.
	w := m.goi(t, http.MethodGet, hostA, duongTrangThai, canBoGhi(xaA))
	doiMa(t, w, http.StatusOK)

	items := docDanhSachTrangThai(t, w.Body.Bytes())
	if len(items) != 7 {
		t.Fatalf("trả %d trạng thái, muốn đủ 7", len(items))
	}
	// Commune A's override: `tam-dung` at 1, tie with `moi-giao` broken by default order.
	if items[0].Code != "moi-giao" || items[1].Code != "tam-dung" {
		t.Fatalf("thứ tự sai: %s, %s", items[0].Code, items[1].Code)
	}
	td := items[1]
	if td.Label != "Đang treo" || td.Order != 1 || !td.Customised || td.Role != "re-nhanh" ||
		td.DefaultLabel != "Tạm dừng" || td.DefaultOrder != 6 {
		t.Errorf("tam-dung = %+v", td)
	}
	// RULE 1: commune B's wording for `moi-giao` must not reach commune A.
	if items[0].Label != "Mới giao" || items[0].Customised || items[0].Role != "chinh" {
		t.Errorf("moi-giao của xã A = %+v — nhãn của xã khác lọt sang, hoặc mặc định sai", items[0])
	}
}

func TestTrangThaiNhiemVu_DocXaChuaSuaGiLaBayMacDinh(t *testing.T) {
	m := dungMayChuGhi(t)
	m.docTT.theoXa = nil
	w := m.goi(t, http.MethodGet, hostA, duongTrangThai, canBoGhi(xaA))
	doiMa(t, w, http.StatusOK)
	for i, it := range docDanhSachTrangThai(t, w.Body.Bytes()) {
		if it.Customised || it.Label != it.DefaultLabel || it.Order != i+1 {
			t.Errorf("dòng %d = %+v, muốn mặc định", i, it)
		}
	}
}

func TestTrangThaiNhiemVu_DocLoiKhoLa500KhongTraMacDinh(t *testing.T) {
	// Falling back to the defaults would silently show a commune wording it replaced.
	m := dungMayChuGhi(t)
	m.docTT.loi = errors.New("cơ sở dữ liệu không phản hồi")
	w := m.goi(t, http.MethodGet, hostA, duongTrangThai, canBoGhi(xaA))
	doiMa(t, w, http.StatusInternalServerError)
	if strings.Contains(w.Body.String(), "cơ sở dữ liệu") || strings.Contains(w.Body.String(), "Mới giao") {
		t.Errorf("thân 500 lộ lỗi nội bộ hoặc trả mặc định: %s", w.Body.String())
	}
}

// --- PATCH: rule 5, invariant 7 ------------------------------------------------------------------

const thanSuaTrangThai = `{"label":"Chưa thực hiện"}`

func duongSuaTrangThai(ma string) string { return duongTrangThai + "/" + ma }

func TestTrangThaiNhiemVu_SuaKhoaQuyenLaAdminLookup(t *testing.T) {
	// Compared against a LITERAL: a fake checker grants any string, so only this catches a key the
	// `quyen` table does not have (rule 5, invariant 3c).
	m := dungMayChuGhi(t)
	m.goiThan(t, http.MethodPatch, hostA, duongSuaTrangThai("moi-giao"), canBoGhi(xaA), thanSuaTrangThai)
	if got := m.checker.hoiKhoaCuoi(); got != "admin.lookup" {
		t.Fatalf("tuyến hỏi khoá %q, muốn \"admin.lookup\"", got)
	}
}

func TestTrangThaiNhiemVu_Sua401KhongPhien(t *testing.T) {
	m := dungMayChuGhi(t)
	m.capQuyen(xaA, QuyenDanhMuc)
	doiMa(t, m.goiThan(t, http.MethodPatch, hostA, duongSuaTrangThai("moi-giao"), nil, thanSuaTrangThai),
		http.StatusUnauthorized)
	if m.ghiTT.goi != 0 {
		t.Error("chưa đăng nhập mà use case đã chạy")
	}
}

func TestTrangThaiNhiemVu_Sua403SaiQuyen(t *testing.T) {
	// `task.update` is a real key of this subsystem: the officer who works tasks must not be able to
	// rename the columns everybody works them in.
	m := dungMayChuGhi(t)
	m.capQuyen(xaA, "task.update")
	doiMa(t, m.goiThan(t, http.MethodPatch, hostA, duongSuaTrangThai("moi-giao"), canBoGhi(xaA), thanSuaTrangThai),
		http.StatusForbidden)
	if m.ghiTT.goi != 0 {
		t.Error("sai quyền mà use case vẫn chạy")
	}
}

func TestTrangThaiNhiemVu_Sua403DungQuyenSaiXa(t *testing.T) {
	// `admin.lookup` granted in commune A; the account of commune B, signed in at B, is refused.
	m := dungMayChuGhi(t)
	m.capQuyen(xaA, QuyenDanhMuc)
	doiMa(t, m.goiThan(t, http.MethodPatch, hostB, duongSuaTrangThai("moi-giao"), canBoGhi(xaB), thanSuaTrangThai),
		http.StatusForbidden)
	if m.ghiTT.goi != 0 {
		t.Error("quyền cấp ở xã khác mà vẫn sửa được nhãn của xã này")
	}
}

func TestTrangThaiNhiemVu_Sua200DungQuyenDungXa(t *testing.T) {
	m := dungMayChuGhi(t)
	m.capQuyen(xaA, QuyenDanhMuc)

	w := m.goiThan(t, http.MethodPatch, hostA, duongSuaTrangThai("moi-giao"), canBoGhi(xaA),
		`{"label":"Chưa thực hiện","order":2}`)
	doiMa(t, w, http.StatusOK)

	g := m.ghiTT
	if g.goi != 1 || g.xaCuoi != xaA || g.maCuoi != "moi-giao" {
		t.Fatalf("use case: gọi=%d xã=%q mã=%q", g.goi, g.xaCuoi, g.maCuoi)
	}
	if g.ycCuoi.Nhan == nil || *g.ycCuoi.Nhan != "Chưa thực hiện" || g.ycCuoi.ThuTu == nil || *g.ycCuoi.ThuTu != 2 {
		t.Errorf("yêu cầu tới use case sai: %+v", g.ycCuoi)
	}
	// RULE 6, INVARIANT 8: the BUSINESS code, and the internal id named as the wrong answer outright.
	if g.nguoiCuoi.ID != maCanBoGhi || g.nguoiCuoi.ID == idCanBoGhi || g.nguoiCuoi.Kind != "staff" ||
		g.nguoiCuoi.IP != "10.0.0.7" {
		t.Errorf("chủ thể vết = %+v, muốn mã cán bộ %q", g.nguoiCuoi, maCanBoGhi)
	}
	var ra trangThaiNhiemVuRa
	if err := json.Unmarshal(w.Body.Bytes(), &ra); err != nil || ra.Code != "moi-giao" || ra.Label != "Đã sửa" {
		t.Errorf("phản hồi = %s", w.Body.String())
	}
}

func TestTrangThaiNhiemVu_SuaChuTheKhongCoMaLa500KhongGhi(t *testing.T) {
	// NO FALLBACK TO p.ID: an empty business code refuses the write (rule 6, invariant 8).
	m := dungMayChuGhi(t)
	m.capQuyen(xaA, QuyenDanhMuc)
	p := canBoGhi(xaA)
	p.Ma = ""
	doiMa(t, m.goiThan(t, http.MethodPatch, hostA, duongSuaTrangThai("moi-giao"), p, thanSuaTrangThai),
		http.StatusInternalServerError)
	if m.ghiTT.goi != 0 {
		t.Error("không có mã cán bộ mà vẫn ghi")
	}
}

// --- PATCH: what a client may not do, and how refusals map ---------------------------------------

func TestTrangThaiNhiemVu_SuaTuChoiCodeVaActive(t *testing.T) {
	for ten, than := range map[string]string{
		"đổi mã":         `{"label":"X","code":"da-huy"}`,
		"tắt trạng thái": `{"active":false}`,
		"bật trạng thái": `{"active":true}`,
	} {
		t.Run(ten, func(t *testing.T) {
			m := dungMayChuGhi(t)
			m.capQuyen(xaA, QuyenDanhMuc)
			doiMa(t, m.goiThan(t, http.MethodPatch, hostA, duongSuaTrangThai("moi-giao"), canBoGhi(xaA), than),
				http.StatusBadRequest)
			if m.ghiTT.goi != 0 {
				t.Error("trường bị cấm mà vẫn tới use case")
			}
		})
	}
}

func TestTrangThaiNhiemVu_SuaAnhXaLoi(t *testing.T) {
	for ten, tc := range map[string]struct {
		loi error
		ma  int
	}{
		"mã ngoài bảy mã": {dmstore.ErrDanhMucKhongTonTai, http.StatusNotFound},
		"nhãn trống":      {domain.ErrNhanTrong, http.StatusBadRequest},
		"nhãn quá dài":    {domain.ErrNhanQuaDai, http.StatusBadRequest},
		"thứ tự < 1":      {domain.ErrThuTuNgoaiKhoang, http.StatusBadRequest},
		"kho hỏng":        {errors.New("cơ sở dữ liệu không phản hồi"), http.StatusInternalServerError},
	} {
		t.Run(ten, func(t *testing.T) {
			m := dungMayChuGhi(t)
			m.capQuyen(xaA, QuyenDanhMuc)
			m.ghiTT.loi = tc.loi
			w := m.goiThan(t, http.MethodPatch, hostA, duongSuaTrangThai("moi-giao"), canBoGhi(xaA), thanSuaTrangThai)
			doiMa(t, w, tc.ma)
			if strings.Contains(w.Body.String(), "cơ sở dữ liệu") {
				t.Errorf("lỗi nội bộ lọt ra: %s", w.Body.String())
			}
		})
	}
}

func TestTrangThaiNhiemVu_SuaKhongCanIdempotencyKey(t *testing.T) {
	// idem.KhongCan: an edit dialog without the header is served.
	m := dungMayChuGhi(t)
	m.capQuyen(xaA, QuyenDanhMuc)
	r := httptest.NewRequest(http.MethodPatch, "https://"+hostA+duongSuaTrangThai("moi-giao"),
		strings.NewReader(thanSuaTrangThai))
	r.Host = hostA
	r.RemoteAddr = "10.0.0.7:51000"
	r = r.WithContext(ctxChuThe(r, canBoGhi(xaA)))
	w := httptest.NewRecorder()
	m.h.ServeHTTP(w, r)
	doiMa(t, w, http.StatusOK)
}
