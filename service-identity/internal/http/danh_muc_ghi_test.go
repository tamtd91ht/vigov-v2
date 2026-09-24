package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/app"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// THE SIX WRITE ROUTES OF THE TWO CATALOGUES — POST · PATCH · DELETE on residential-unit-types and
// task-blocs, all `admin.lookup`. TABLE-DRIVEN OVER BOTH CATALOGUES, so no assertion is made on
// whichever one was written first.
//
// What is asserted here is what the HANDLER owns: the permission, the status, the wire mapping, the
// refusal of `source`/`tier`/`code` before the use case, and which of a principal's two identifiers
// reaches the trail. The tier rules and the one-transaction audit are decisions of the use case,
// proved over a real transaction boundary in app/danh_muc_ghi_test.go.

// ghiDanhMucGia stands in for *app.DanhMucGhi[T], recording the commune (read from the context, as
// *store.Scoped reads it), the actor and the request.
type ghiDanhMucGia[T any] struct {
	ra  T
	loi error

	themGoi, suaGoi, xoaGoi int
	xaCuoi                  tenant.ID
	nguoiCuoi               app.NguoiThucHien
	themCuoi                app.YeuCauThemDanhMuc
	suaCuoi                 app.YeuCauSuaDanhMuc
	idCuoi, lyDoCuoi        string
}

func (g *ghiDanhMucGia[T]) Them(ctx context.Context, yc app.YeuCauThemDanhMuc, nguoi app.NguoiThucHien) (T, error) {
	g.themGoi++
	g.themCuoi, g.xaCuoi, g.nguoiCuoi = yc, tenant.MustFrom(ctx), nguoi
	return g.ra, g.loi
}

func (g *ghiDanhMucGia[T]) Sua(ctx context.Context, id string, yc app.YeuCauSuaDanhMuc, nguoi app.NguoiThucHien) (T, error) {
	g.suaGoi++
	g.idCuoi, g.suaCuoi, g.xaCuoi, g.nguoiCuoi = id, yc, tenant.MustFrom(ctx), nguoi
	return g.ra, g.loi
}

func (g *ghiDanhMucGia[T]) Xoa(ctx context.Context, id, lyDo string, nguoi app.NguoiThucHien) error {
	g.xoaGoi++
	g.idCuoi, g.lyDoCuoi, g.xaCuoi, g.nguoiCuoi = id, lyDo, tenant.MustFrom(ctx), nguoi
	return g.loi
}

func (g *ghiDanhMucGia[T]) tong() int { return g.themGoi + g.suaGoi + g.xoaGoi }

func ghiLoaiDonViDanCuMau() *ghiDanhMucGia[domain.LoaiDonViDanCu] {
	return &ghiDanhMucGia[domain.LoaiDonViDanCu]{ra: domain.LoaiDonViDanCu{
		ID: "ldv-moi", Ma: "khu-pho", Nhan: "Khu phố", ThuTu: 5, DangDung: true, Nguon: domain.NguonDonVi,
	}}
}

func ghiKhoiNhiemVuMau() *ghiDanhMucGia[domain.KhoiNhiemVu] {
	return &ghiDanhMucGia[domain.KhoiNhiemVu]{ra: domain.KhoiNhiemVu{
		ID: "knv-moi", Ma: "khoi-mat-tran", Nhan: "Khối Mặt trận", ThuTu: 5, DangDung: true, Nguon: domain.NguonDonVi,
	}}
}

// dungMayChuDanhMuc grants `admin.lookup` in commune A and nothing in commune B. The harness default
// grants `admin.user` — a real key that is not this one — which is the "wrong permission" case.
func dungMayChuDanhMuc(t *testing.T) *mayChu {
	t.Helper()
	m := dungMayChu(t)
	m.dungLai(t, func(d *Deps) {
		d.Checker = checkerGia{quyen: map[tenant.ID]map[string]map[authz.Perm]bool{
			xaA: {idNoiBo: {authz.Perm("admin.lookup"): true}},
			xaB: {},
		}}
	})
	return m
}

// danhMucThu is one catalogue as the tests see it: its path, and the counters of its fake.
type danhMucThu struct {
	ten   string
	duong string
	goi   func(m *mayChu) int
	xa    func(m *mayChu) tenant.ID
	nguoi func(m *mayChu) app.NguoiThucHien
}

func haiDanhMuc() []danhMucThu {
	return []danhMucThu{
		{"loại đơn vị dân cư", duongLoaiDonViDanCu,
			func(m *mayChu) int { return m.ghiLoaiDonViDanCu.tong() },
			func(m *mayChu) tenant.ID { return m.ghiLoaiDonViDanCu.xaCuoi },
			func(m *mayChu) app.NguoiThucHien { return m.ghiLoaiDonViDanCu.nguoiCuoi }},
		{"khối nhiệm vụ", duongKhoiNhiemVu,
			func(m *mayChu) int { return m.ghiKhoiNhiemVu.tong() },
			func(m *mayChu) tenant.ID { return m.ghiKhoiNhiemVu.xaCuoi },
			func(m *mayChu) app.NguoiThucHien { return m.ghiKhoiNhiemVu.nguoiCuoi }},
	}
}

type tuyenDanhMuc struct {
	ten, method, duong, than string
	ok                       int
}

// baTuyen is the three write routes of one catalogue.
func (dm danhMucThu) baTuyen() []tuyenDanhMuc {
	return []tuyenDanhMuc{
		{dm.ten + " POST", "POST", dm.duong, `{"code":"khu-pho","label":"Khu phố","order":5}`, http.StatusCreated},
		{dm.ten + " PATCH", "PATCH", dm.duong + "/muc-001", `{"label":"Nhãn mới"}`, http.StatusOK},
		{dm.ten + " DELETE", "DELETE", dm.duong + "/muc-001", `{"reason":"nhập nhầm"}`, http.StatusNoContent},
	}
}

func moiTuyenDanhMuc() []struct {
	dm danhMucThu
	tg tuyenDanhMuc
} {
	var ra []struct {
		dm danhMucThu
		tg tuyenDanhMuc
	}
	for _, dm := range haiDanhMuc() {
		for _, tg := range dm.baTuyen() {
			ra = append(ra, struct {
				dm danhMucThu
				tg tuyenDanhMuc
			}{dm, tg})
		}
	}
	return ra
}

// --- the four cases of rule 5, invariant 7, on all six routes ---------------------------------------

func TestDanhMucGhi_401KhongToken(t *testing.T) {
	for _, c := range moiTuyenDanhMuc() {
		m := dungMayChuDanhMuc(t)
		if w := m.goiIdem(t, c.tg.method, hostA, c.tg.duong, c.tg.than, ""); w.Code != http.StatusUnauthorized {
			t.Errorf("%s: mã = %d, muốn 401", c.tg.ten, w.Code)
		}
		if c.dm.goi(m) != 0 {
			t.Errorf("%s: chưa đăng nhập mà use case đã chạy", c.tg.ten)
		}
	}
}

// 403 WITH `admin.user` — a real key, which is not `admin.lookup`.
func TestDanhMucGhi_403SaiQuyen(t *testing.T) {
	for _, c := range moiTuyenDanhMuc() {
		m := dungMayChu(t)
		w := m.goiIdem(t, c.tg.method, hostA, c.tg.duong, c.tg.than, m.tokenCho(t, xaA, sidA))
		if w.Code != http.StatusForbidden {
			t.Errorf("%s: mã = %d, muốn 403 — %s", c.tg.ten, w.Code, w.Body.String())
		}
		if c.dm.goi(m) != 0 {
			t.Errorf("%s: sai quyền mà use case đã chạy", c.tg.ten)
		}
	}
}

// 403 WITH `admin.lookup` HELD IN COMMUNE A, signed in properly at commune B (rule 5, invariant 3):
// the session is valid; the authority is absent in THIS commune.
func TestDanhMucGhi_403DungQuyenSaiXa(t *testing.T) {
	for _, c := range moiTuyenDanhMuc() {
		m := dungMayChuDanhMuc(t)
		w := m.goiIdem(t, c.tg.method, hostB, c.tg.duong, c.tg.than, m.tokenCho(t, xaB, sidB))
		if w.Code != http.StatusForbidden {
			t.Errorf("%s: mã = %d, muốn 403 — %s", c.tg.ten, w.Code, w.Body.String())
		}
		if c.dm.goi(m) != 0 {
			t.Errorf("%s: quyền cấp ở xã khác mà vẫn ghi được vào xã này", c.tg.ten)
		}
	}
}

// 2xx, THE COMMUNE OF THE HOST REACHES THE USE CASE, AND THE TRAIL GETS THE STAFF CODE.
//
// MUTATION THAT MUST TURN THIS RED: build audit.Actor from p.ID in nguoiThucHienCanBo.
func TestDanhMucGhi_2xxVaChuTheVetLaMaCanBo(t *testing.T) {
	for _, c := range moiTuyenDanhMuc() {
		m := dungMayChuDanhMuc(t)
		w := m.goiIdem(t, c.tg.method, hostA, c.tg.duong, c.tg.than, m.tokenCho(t, xaA, sidA))
		if w.Code != c.tg.ok {
			t.Fatalf("%s: mã = %d, muốn %d — %s", c.tg.ten, w.Code, c.tg.ok, w.Body.String())
		}
		if c.dm.goi(m) != 1 {
			t.Errorf("%s: use case chạy %d lần, muốn 1", c.tg.ten, c.dm.goi(m))
		}
		if c.dm.xa(m) != xaA {
			t.Errorf("%s: use case chạy ở xã %q, muốn %q", c.tg.ten, c.dm.xa(m), xaA)
		}
		n := c.dm.nguoi(m)
		if n.Vet.ID != maCanBo || n.Vet.ID == idNoiBo || n.ID != idNoiBo {
			t.Errorf("%s: Vet.ID=%q ID=%q — vết phải mang MÃ CÁN BỘ %q", c.tg.ten, n.Vet.ID, n.ID, maCanBo)
		}
		if n.Vet.IP != "10.0.0.7" {
			t.Errorf("%s: IP trong vết = %q", c.tg.ten, n.Vet.IP)
		}
	}
}

// --- `source` / `tier` / `code` are refused, not ignored ---------------------------------------------

// BOTH DIRECTIONS OF `source` ARE SENT: `he-thong` is the one that would buy something (a row nobody
// can delete); `don-vi` is refused just as firmly, because the rule is "not from the client".
//
// MUTATION THAT MUST TURN THIS RED: drop the `vao.Source != nil || vao.Tier != nil` check.
func TestDanhMucGhiTuChoiNguonVaTangTuClient(t *testing.T) {
	for _, dm := range haiDanhMuc() {
		for ten, c := range map[string]struct{ method, duong, than string }{
			"POST source he-thong":  {"POST", dm.duong, `{"code":"khu-pho","label":"Khu phố","source":"he-thong"}`},
			"POST source don-vi":    {"POST", dm.duong, `{"code":"khu-pho","label":"Khu phố","source":"don-vi"}`},
			"POST tier 1":           {"POST", dm.duong, `{"code":"khu-pho","label":"Khu phố","tier":1}`},
			"PATCH source he-thong": {"PATCH", dm.duong + "/muc-001", `{"label":"X","source":"he-thong"}`},
			"PATCH tier 3":          {"PATCH", dm.duong + "/muc-001", `{"label":"X","tier":3}`},
		} {
			m := dungMayChuDanhMuc(t)
			w := m.goiIdem(t, c.method, hostA, c.duong, c.than, m.tokenCho(t, xaA, sidA))
			if w.Code != http.StatusBadRequest {
				t.Errorf("%s / %s: mã = %d, muốn 400", dm.ten, ten, w.Code)
				continue
			}
			if dm.goi(m) != 0 {
				t.Errorf("%s / %s: yêu cầu tự đặt nguồn mà vẫn tới use case", dm.ten, ten)
			}
			if e := loiTra(t, w); e.Code != "invalid_request" || !strings.Contains(e.Message, "source") {
				t.Errorf("%s / %s: lỗi = %+v — phải nói rõ trường bị từ chối", dm.ten, ten, e)
			}
		}
	}
}

// AN ISSUED CODE IS NEVER RENUMBERED (rule 7, invariant 3): a PATCH naming `code` is refused 400 —
// even with other fields beside it.
func TestDanhMucGhiSuaTuChoiDoiMa(t *testing.T) {
	for _, dm := range haiDanhMuc() {
		for _, than := range []string{`{"code":"ma-moi"}`, `{"label":"X","code":"ma-moi"}`} {
			m := dungMayChuDanhMuc(t)
			w := m.goiIdem(t, "PATCH", hostA, dm.duong+"/muc-001", than, m.tokenCho(t, xaA, sidA))
			if w.Code != http.StatusBadRequest {
				t.Errorf("%s %s: mã = %d, muốn 400", dm.ten, than, w.Code)
			}
			if dm.goi(m) != 0 {
				t.Errorf("%s %s: yêu cầu đổi mã mà vẫn tới use case", dm.ten, than)
			}
		}
	}
}

// --- every refusal reaches the caller as the SIBLINGS' status and code --------------------------------

func TestDanhMucGhiAnhXaLoi(t *testing.T) {
	for _, c := range []struct {
		loi  error
		ma   int
		code string
	}{
		{idstore.ErrDanhMucKhongTonTai, http.StatusNotFound, "not_found"},
		{idstore.ErrMaDaTonTai, http.StatusConflict, "code_taken"},
		{idstore.ErrDanhMucDayTran, http.StatusConflict, "catalogue_full"},
		{domain.ErrKhongXoaDuocMucHeThong, http.StatusConflict, "system_row"},
		{domain.ErrKhongTatDuocMucReNhanh, http.StatusConflict, "code_branch_row"},
		{domain.ErrThieuLyDoXoaDanhMuc, http.StatusBadRequest, "invalid_request"},
		{domain.ErrMaDanhMucSaiDinhDang, http.StatusBadRequest, "invalid_request"},
		{errors.New("cơ sở dữ liệu không phản hồi"), http.StatusInternalServerError, "internal"},
	} {
		for _, dm := range haiDanhMuc() {
			m := dungMayChuDanhMuc(t)
			m.ghiLoaiDonViDanCu.loi, m.ghiKhoiNhiemVu.loi = c.loi, c.loi
			for _, tg := range dm.baTuyen() {
				w := m.goiIdem(t, tg.method, hostA, tg.duong, tg.than, m.tokenCho(t, xaA, sidA))
				if got := loiTra(t, w).Code; w.Code != c.ma || got != c.code {
					t.Errorf("%s / %v: mã = %d (%s), muốn %d (%s)", tg.ten, c.loi, w.Code, got, c.ma, c.code)
				}
				if strings.Contains(w.Body.String(), "không phản hồi") {
					t.Errorf("lỗi nội bộ lọt ra ngoài: %s", w.Body.String())
				}
			}
		}
	}
}

// --- what each route passes on and answers -----------------------------------------------------------

func TestThemDanhMucAnhXaThanVaTraTang(t *testing.T) {
	m := dungMayChuDanhMuc(t)
	tok := m.tokenCho(t, xaA, sidA)

	w := m.goiIdem(t, "POST", hostA, duongLoaiDonViDanCu,
		`{"code":"khu-pho","label":"Khu phố","order":5,"is_default":true,"id":"bo-qua"}`, tok)
	doiMa(t, w, http.StatusCreated)
	yc := m.ghiLoaiDonViDanCu.themCuoi
	if yc.Ma != "khu-pho" || yc.Nhan != "Khu phố" || yc.ThuTu != 5 || !yc.LaMacDinh {
		t.Errorf("yêu cầu tới use case sai: %+v", yc)
	}
	var ra loaiDonViDanCuRa
	if err := json.Unmarshal(w.Body.Bytes(), &ra); err != nil || ra.ID != "ldv-moi" || ra.Code != "khu-pho" ||
		ra.Source != domain.NguonDonVi || ra.Tier != int(domain.TangDonVi) || ra.Order != 5 {
		t.Errorf("phản hồi: %+v (%v)", ra, err)
	}

	w = m.goiIdem(t, "POST", hostA, duongKhoiNhiemVu, `{"code":"khoi-mat-tran","label":"Khối Mặt trận"}`, tok)
	doiMa(t, w, http.StatusCreated)
	var rk khoiNhiemVuRa
	if err := json.Unmarshal(w.Body.Bytes(), &rk); err != nil || rk.Code != "khoi-mat-tran" || rk.Tier != 1 || rk.Source != "don-vi" {
		t.Errorf("phản hồi khối: %+v (%v)", rk, err)
	}
	if m.ghiLoaiDonViDanCu.themGoi != 1 || m.ghiKhoiNhiemVu.themGoi != 1 {
		t.Errorf("mỗi tuyến phải gọi đúng use case của mình: loai=%d khoi=%d",
			m.ghiLoaiDonViDanCu.themGoi, m.ghiKhoiNhiemVu.themGoi)
	}
}

// PATCH SEMANTICS: a field not mentioned arrives as nil; `false` and `0` arrive as values.
func TestSuaDanhMucConTroNilVaGiaTriKhong(t *testing.T) {
	m := dungMayChuDanhMuc(t)
	tok := m.tokenCho(t, xaA, sidA)

	doiMa(t, m.goiIdem(t, "PATCH", hostA, duongKhoiNhiemVu+"/knv-001", `{"label":"Khối UBND"}`, tok), http.StatusOK)
	yc := m.ghiKhoiNhiemVu.suaCuoi
	if m.ghiKhoiNhiemVu.idCuoi != "knv-001" || yc.Nhan == nil || *yc.Nhan != "Khối UBND" {
		t.Errorf("id/nhãn sai: %q %+v", m.ghiKhoiNhiemVu.idCuoi, yc)
	}
	if yc.ThuTu != nil || yc.DangDung != nil || yc.LaMacDinh != nil {
		t.Errorf("trường không được nhắc tới lại có giá trị: %+v", yc)
	}

	doiMa(t, m.goiIdem(t, "PATCH", hostA, duongKhoiNhiemVu+"/knv-001",
		`{"active":false,"order":0,"is_default":false}`, tok), http.StatusOK)
	yc = m.ghiKhoiNhiemVu.suaCuoi
	if yc.DangDung == nil || *yc.DangDung || yc.ThuTu == nil || *yc.ThuTu != 0 || yc.LaMacDinh == nil || *yc.LaMacDinh {
		t.Errorf("false/0 không tới nơi: %+v", yc)
	}
}

func TestXoaDanhMucChuyenLyDoVaTra204KhongThan(t *testing.T) {
	m := dungMayChuDanhMuc(t)
	w := m.goiIdem(t, "DELETE", hostA, duongLoaiDonViDanCu+"/ldv-002", `{"reason":"nhập nhầm"}`, m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusNoContent)
	if m.ghiLoaiDonViDanCu.idCuoi != "ldv-002" || m.ghiLoaiDonViDanCu.lyDoCuoi != "nhập nhầm" {
		t.Errorf("id/lý do sai: %q / %q", m.ghiLoaiDonViDanCu.idCuoi, m.ghiLoaiDonViDanCu.lyDoCuoi)
	}
	if w.Body.Len() != 0 {
		t.Errorf("204 mà vẫn có thân: %q", w.Body.String())
	}
}

// --- duplicate-request declarations ------------------------------------------------------------------

// POST declares idem.Required: no key → refused before the use case. PATCH and DELETE declare
// idem.KhongCan: no key → served. Both halves pinned, because every other test sends the header.
func TestDanhMucGhiKhoaChongGuiLap(t *testing.T) {
	for _, dm := range haiDanhMuc() {
		m := dungMayChuDanhMuc(t)
		tok := m.tokenCho(t, xaA, sidA)
		w := m.goi(t, "POST", hostA, dm.duong, `{"code":"khu-pho","label":"Khu phố"}`, tok)
		if w.Code/100 != 4 || dm.goi(m) != 0 {
			t.Errorf("%s POST thiếu Idempotency-Key: mã = %d, use case %d lần", dm.ten, w.Code, dm.goi(m))
		}
		if w := m.goi(t, "PATCH", hostA, dm.duong+"/muc-001", `{"label":"X"}`, tok); w.Code != http.StatusOK {
			t.Errorf("%s PATCH không cần khoá: mã = %d", dm.ten, w.Code)
		}
		if w := m.goi(t, "DELETE", hostA, dm.duong+"/muc-001", `{"reason":"x"}`, tok); w.Code != http.StatusNoContent {
			t.Errorf("%s DELETE không cần khoá: mã = %d", dm.ten, w.Code)
		}
	}
}

// THE READS STAY AnyAuthenticated: adding writes under `admin.lookup` must not have tightened them.
func TestDanhMucDocVanKhongCanQuyen(t *testing.T) {
	m := dungMayChu(t)
	m.dungLai(t, func(d *Deps) { d.Checker = checkerGia{} })
	for _, dm := range haiDanhMuc() {
		var w *httptest.ResponseRecorder = m.goi(t, "GET", hostA, dm.duong, "", m.tokenCho(t, xaA, sidA))
		if w.Code != http.StatusOK {
			t.Errorf("%s GET: mã = %d, muốn 200", dm.ten, w.Code)
		}
	}
}
