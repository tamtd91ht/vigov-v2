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

// THE WRITE ROUTES OF THE ORG CHART — POST /api/v1/org-units and PATCH /api/v1/org-units/{id}, both
// `admin.org`.
//
// What is asserted here is what the HANDLER owns: the permission, the status, the wire mapping and
// which of a principal's two identifiers reaches the trail. The slug, the cycle walk and the audit
// transaction are decisions of the use case, proved over a real transaction boundary in
// app/so_do_to_chuc_test.go; asserting them again through a fake would assert the fake.

// THE ORDER AND THE STAFF COUNT REACH THE WIRE on the read, and an empty unit prints 0 rather than
// dropping the field. The fixture gives ThuTu and SoCanBo different values so a transposition shows.
func TestBoPhanTraThuTuVaSoCanBo(t *testing.T) {
	m := dungMayChu(t)
	w := m.goi(t, "GET", hostA, duongBoPhan, "", m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)

	ra := docBoPhan(t, w.Body.Bytes())
	if ra.Items[0].Order != 1 || ra.Items[0].StaffCount != 3 {
		t.Errorf("bộ phận đầu: order=%d staff_count=%d, muốn 1 và 3", ra.Items[0].Order, ra.Items[0].StaffCount)
	}
	if !strings.Contains(w.Body.String(), `"staff_count":0`) {
		t.Errorf("bộ phận không có ai phải in staff_count: 0, không được bỏ trường: %s", w.Body.String())
	}
}

// ghiBoPhanGia stands in for *app.SoDoToChuc. It records the actor, the commune and the request, so
// a handler that dropped or swapped any of them turns something red.
type ghiBoPhanGia struct {
	goi       int
	nguoiCuoi app.NguoiThucHien
	xaCuoi    tenant.ID
	idCuoi    string
	themCuoi  app.YeuCauThemBoPhan
	suaCuoi   app.YeuCauSuaBoPhan

	kq  domain.BoPhan
	loi error
}

func ghiBoPhanMau() *ghiBoPhanGia {
	return &ghiBoPhanGia{kq: domain.BoPhan{
		ID: "bp-003", Ma: "van-phong-hdnd", Ten: "VĂN PHÒNG HĐND", ChaID: "bp-001", ThuTu: 5,
	}}
}

func (g *ghiBoPhanGia) Them(ctx context.Context, yc app.YeuCauThemBoPhan, nguoi app.NguoiThucHien) (domain.BoPhan, error) {
	g.goi++
	g.nguoiCuoi, g.xaCuoi, g.themCuoi = nguoi, tenant.MustFrom(ctx), yc
	return g.kq, g.loi
}

func (g *ghiBoPhanGia) Sua(ctx context.Context, id string, yc app.YeuCauSuaBoPhan, nguoi app.NguoiThucHien) (domain.BoPhan, error) {
	g.goi++
	g.nguoiCuoi, g.xaCuoi, g.idCuoi, g.suaCuoi = nguoi, tenant.MustFrom(ctx), id, yc
	return g.kq, g.loi
}

// dungMayChuSoDo grants `admin.org` in commune A and nothing in commune B. The harness default
// grants `admin.user`, which makes the "wrong permission" case the shipped default.
func dungMayChuSoDo(t *testing.T) *mayChu {
	t.Helper()
	m := dungMayChu(t)
	m.dungLai(t, func(d *Deps) {
		d.Checker = checkerGia{quyen: map[tenant.ID]map[string]map[authz.Perm]bool{
			xaA: {idNoiBo: {authz.Perm("admin.org"): true}},
			xaB: {},
		}}
	})
	return m
}

type tuyenSoDo struct {
	ten, method, duong, than string
	ok                       int
}

func moiTuyenSoDo() []tuyenSoDo {
	return []tuyenSoDo{
		{"thêm bộ phận", "POST", duongBoPhan, `{"name":"VĂN PHÒNG HĐND","parent_id":"bp-001"}`, http.StatusCreated},
		{"sửa bộ phận", "PATCH", duongBoPhan + "/bp-002", `{"name":"TỔ MỘT CỬA LIÊN THÔNG"}`, http.StatusOK},
	}
}

// goiSoDo sends with an Idempotency-Key: POST declares idem.Required, and a harness that omitted the
// header would turn every assertion about that route into one about the header.
func (m *mayChu) goiSoDo(t *testing.T, tg tuyenSoDo, host, tok string) *httptest.ResponseRecorder {
	t.Helper()
	return m.goiIdem(t, tg.method, host, tg.duong, tg.than, tok)
}

func TestSoDo_401KhongToken(t *testing.T) {
	m := dungMayChuSoDo(t)
	for _, tg := range moiTuyenSoDo() {
		if w := m.goiSoDo(t, tg, hostA, ""); w.Code != http.StatusUnauthorized {
			t.Errorf("%s: mã = %d, muốn 401 — %s", tg.ten, w.Code, w.Body.String())
		}
	}
	if m.ghiBoPhan.goi != 0 {
		t.Errorf("chưa đăng nhập mà use case đã chạy %d lần", m.ghiBoPhan.goi)
	}
}

// 403 WITH `admin.user` — a real key, the one the staff screen uses, which is not `admin.org`.
func TestSoDo_403SaiQuyen(t *testing.T) {
	m := dungMayChu(t)
	tok := m.tokenCho(t, xaA, sidA)
	for _, tg := range moiTuyenSoDo() {
		if w := m.goiSoDo(t, tg, hostA, tok); w.Code != http.StatusForbidden {
			t.Errorf("%s: mã = %d, muốn 403 — %s", tg.ten, w.Code, w.Body.String())
		}
	}
	if m.ghiBoPhan.goi != 0 {
		t.Errorf("sai quyền mà use case đã chạy %d lần", m.ghiBoPhan.goi)
	}
}

// 403 WITH `admin.org` HELD IN COMMUNE A, signed in properly at commune B (rule 5, invariant 3).
func TestSoDo_403DungQuyenSaiXa(t *testing.T) {
	m := dungMayChuSoDo(t)
	tok := m.tokenCho(t, xaB, sidB)
	for _, tg := range moiTuyenSoDo() {
		if w := m.goiSoDo(t, tg, hostB, tok); w.Code != http.StatusForbidden {
			t.Errorf("%s: mã = %d, muốn 403 — %s", tg.ten, w.Code, w.Body.String())
		}
	}
	if m.ghiBoPhan.goi != 0 {
		t.Errorf("sai xã mà use case đã chạy %d lần", m.ghiBoPhan.goi)
	}
}

// 2xx, THE COMMUNE OF THE HOST REACHES THE USE CASE, AND THE TRAIL GETS THE STAFF CODE.
//
// MUTATION THAT MUST TURN THIS RED: build audit.Actor from p.ID in nguoiThucHienCanBo.
func TestSoDo_2xxVaChuTheVetLaMaCanBo(t *testing.T) {
	m := dungMayChuSoDo(t)
	tok := m.tokenCho(t, xaA, sidA)
	for _, tg := range moiTuyenSoDo() {
		m.ghiBoPhan.nguoiCuoi = app.NguoiThucHien{}
		w := m.goiSoDo(t, tg, hostA, tok)
		if w.Code != tg.ok {
			t.Fatalf("%s: mã = %d, muốn %d — %s", tg.ten, w.Code, tg.ok, w.Body.String())
		}
		n := m.ghiBoPhan.nguoiCuoi
		if n.Vet.ID != maCanBo || n.Vet.ID == idNoiBo || n.ID != idNoiBo {
			t.Errorf("%s: Vet.ID=%q ID=%q — vết phải mang MÃ CÁN BỘ %q, quyết định mang id nội bộ",
				tg.ten, n.Vet.ID, n.ID, maCanBo)
		}
		if m.ghiBoPhan.xaCuoi != xaA {
			t.Errorf("%s: use case chạy ở xã %q, muốn %q", tg.ten, m.ghiBoPhan.xaCuoi, xaA)
		}
		// The write response carries NO staff_count — a zero there would be a false figure.
		if strings.Contains(w.Body.String(), "staff_count") {
			t.Errorf("%s: phản hồi ghi mang staff_count: %s", tg.ten, w.Body.String())
		}
	}
}

func TestThemBoPhanAnhXaThan(t *testing.T) {
	m := dungMayChuSoDo(t)
	tok := m.tokenCho(t, xaA, sidA)
	w := m.goiIdem(t, "POST", hostA, duongBoPhan,
		`{"name":"VĂN PHÒNG HĐND","parent_id":"bp-001","order":5,"code":"vp-hdnd"}`, tok)
	doiMa(t, w, http.StatusCreated)
	yc := m.ghiBoPhan.themCuoi
	if yc.Ten != "VĂN PHÒNG HĐND" || yc.ChaID != "bp-001" || yc.ThuTu == nil || *yc.ThuTu != 5 || yc.Ma != "vp-hdnd" {
		t.Errorf("yêu cầu tới use case sai: %+v", yc)
	}
	var ra boPhanDaGhiRa
	if err := json.Unmarshal(w.Body.Bytes(), &ra); err != nil || ra.ID != "bp-003" || ra.Code != "van-phong-hdnd" ||
		ra.Order != 5 || ra.ParentID != "bp-001" {
		t.Errorf("phản hồi: %+v (%v)", ra, err)
	}
}

// POST WITHOUT AN Idempotency-Key IS REFUSED before the use case — the route has no natural key
// under a derived code, so the header is its only protection against a second permanent unit.
func TestThemBoPhanThieuKhoaIdemBiTuChoi(t *testing.T) {
	m := dungMayChuSoDo(t)
	w := m.goi(t, "POST", hostA, duongBoPhan, `{"name":"VĂN PHÒNG HĐND"}`, m.tokenCho(t, xaA, sidA))
	if w.Code/100 != 4 {
		t.Fatalf("mã = %d, muốn 4xx — %s", w.Code, w.Body.String())
	}
	if m.ghiBoPhan.goi != 0 {
		t.Error("thiếu khoá chống gửi lặp mà use case vẫn chạy")
	}
}

// `parent_id: ""` IS A MOVE TO THE ROOT; absent leaves the parent alone. The two must stay
// distinguishable all the way to the use case.
func TestSuaBoPhanPhanBietVeGocVaKhongDoi(t *testing.T) {
	m := dungMayChuSoDo(t)
	tok := m.tokenCho(t, xaA, sidA)

	doiMa(t, m.goi(t, "PATCH", hostA, duongBoPhan+"/bp-002", `{"parent_id":""}`, tok), http.StatusOK)
	if p := m.ghiBoPhan.suaCuoi.ChaID; p == nil || *p != "" {
		t.Errorf(`parent_id "" phải tới use case là con trỏ tới "", nhận %v`, p)
	}
	if m.ghiBoPhan.idCuoi != "bp-002" {
		t.Errorf("id = %q", m.ghiBoPhan.idCuoi)
	}

	doiMa(t, m.goi(t, "PATCH", hostA, duongBoPhan+"/bp-002", `{"order":3}`, tok), http.StatusOK)
	if yc := m.ghiBoPhan.suaCuoi; yc.ChaID != nil || yc.Ten != nil || yc.ThuTu == nil || *yc.ThuTu != 3 {
		t.Errorf("trường không gửi lên phải là nil: %+v", yc)
	}
}

// THE CODE IS NOT EDITABLE, and a body naming it is REFUSED — never silently ignored.
func TestSuaBoPhanMaKhongSuaDuoc(t *testing.T) {
	m := dungMayChuSoDo(t)
	tok := m.tokenCho(t, xaA, sidA)
	for _, than := range []string{`{"code":"ma-moi"}`, `{"name":"X","code":"ma-moi"}`, `{}`} {
		truoc := m.ghiBoPhan.goi
		w := m.goi(t, "PATCH", hostA, duongBoPhan+"/bp-002", than, tok)
		if w.Code != http.StatusBadRequest {
			t.Errorf("thân %s: mã = %d, muốn 400", than, w.Code)
		}
		if m.ghiBoPhan.goi != truoc {
			t.Errorf("thân %s: use case vẫn chạy", than)
		}
	}
	w := m.goi(t, "PATCH", hostA, duongBoPhan+"/bp-002", `{"code":"ma-moi"}`, tok)
	if got := loiTra(t, w).Code; got != "code_not_editable" {
		t.Errorf("code = %q, muốn code_not_editable", got)
	}
}

func TestGhiBoPhanAnhXaLoi(t *testing.T) {
	for _, c := range []struct {
		loi  error
		ma   int
		code string
	}{
		{idstore.ErrKhongTimThayBoPhan, http.StatusNotFound, "org_unit_not_found"},
		{idstore.ErrBoPhanChaKhongTonTai, http.StatusBadRequest, "parent_not_found"},
		{idstore.ErrMaBoPhanDaDung, http.StatusConflict, "org_unit_code_taken"},
		{app.ErrCayBoPhanVongLap, http.StatusConflict, "org_unit_cycle"},
		{domain.ErrThieuTenBoPhan, http.StatusBadRequest, "invalid_request"},
		{errors.New("cơ sở dữ liệu không phản hồi"), http.StatusInternalServerError, "internal"},
	} {
		m := dungMayChuSoDo(t)
		m.ghiBoPhan.loi = c.loi
		tok := m.tokenCho(t, xaA, sidA)
		for _, tg := range moiTuyenSoDo() {
			w := m.goiSoDo(t, tg, hostA, tok)
			if got := loiTra(t, w).Code; w.Code != c.ma || got != c.code {
				t.Errorf("%s / %v: mã = %d (%s), muốn %d (%s)", tg.ten, c.loi, w.Code, got, c.ma, c.code)
			}
			if strings.Contains(w.Body.String(), "không phản hồi") {
				t.Errorf("lỗi nội bộ lọt ra ngoài: %s", w.Body.String())
			}
		}
	}
}
