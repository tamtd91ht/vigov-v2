package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/vihat/vigov/core/tenant"
	"github.com/vihat/vigov/service-identity/internal/app"
	"github.com/vihat/vigov/service-identity/internal/domain"
	idstore "github.com/vihat/vigov/service-identity/internal/store"
)

// WHAT THIS FILE IS FOR: PUT /api/v1/roles/{id}/permissions — the route that decides who may grant
// what. Here: the four cases of rule 5 invariant 7 on the real chain, which identifier reaches the
// trail, and that every refusal of the use case reaches the client with the right status and code.
// The refusals THEMSELVES (#13, #14, the audit transaction) are proven in
// app/phan_quyen_vai_tro_test.go, against the fake driver.

const duongLuuPhanQuyen = "/api/v1/roles/vt-chuyen-vien/permissions"

// phanQuyenGhiGia stands in for *app.PhanQuyenVaiTro and records what the handler passed.
type phanQuyenGhiGia struct {
	mu        sync.Mutex
	goi       int
	nguoiCuoi app.NguoiThucHien
	xaCuoi    tenant.ID
	idCuoi    string
	dsCuoi    []string

	kq  []string
	loi error
}

func phanQuyenGhiMau() *phanQuyenGhiGia {
	return &phanQuyenGhiGia{kq: []string{"document.read", "task.read"}}
}

func (g *phanQuyenGhiGia) Luu(ctx context.Context, id string, ds []string, nguoi app.NguoiThucHien) ([]string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.goi++
	g.nguoiCuoi, g.xaCuoi, g.idCuoi, g.dsCuoi = nguoi, tenant.MustFrom(ctx), id, ds
	return g.kq, g.loi
}

func (g *phanQuyenGhiGia) soLanGoi() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.goi
}

const thanLuuPhanQuyen = `{"permissions":["task.read","document.read"]}`

// --- rule 5, invariant 7 --------------------------------------------------------------------------

func TestLuuPhanQuyen_401KhongToken(t *testing.T) {
	m := mayChuPhanQuyen(t, xaA)
	doiMa(t, m.goi(t, "PUT", hostA, duongLuuPhanQuyen, thanLuuPhanQuyen, ""), http.StatusUnauthorized)
	if n := m.ghiPhanQuyen.soLanGoi(); n != 0 {
		t.Errorf("chưa đăng nhập mà use case lưu phân quyền đã chạy %d lần", n)
	}
}

// 403 without `admin.role`: the default harness grants `admin.user` — the neighbouring tab's key,
// the realistic near miss.
func TestLuuPhanQuyen_403SaiQuyen(t *testing.T) {
	m := dungMayChu(t)
	w := m.goi(t, "PUT", hostA, duongLuuPhanQuyen, thanLuuPhanQuyen, m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusForbidden)
	if n := m.ghiPhanQuyen.soLanGoi(); n != 0 {
		t.Errorf("thiếu admin.role mà use case lưu phân quyền đã chạy %d lần", n)
	}
}

// 403 with `admin.role` in commune A, properly signed in at commune B.
func TestLuuPhanQuyen_403DungQuyenSaiXa(t *testing.T) {
	m := mayChuPhanQuyen(t, xaA)
	w := m.goi(t, "PUT", hostB, duongLuuPhanQuyen, thanLuuPhanQuyen, m.tokenCho(t, xaB, sidB))
	doiMa(t, w, http.StatusForbidden)
	if n := m.ghiPhanQuyen.soLanGoi(); n != 0 {
		t.Errorf("sai xã mà use case lưu phân quyền đã chạy %d lần", n)
	}
}

func TestLuuPhanQuyen_200(t *testing.T) {
	m := mayChuPhanQuyen(t, xaA)
	w := m.goi(t, "PUT", hostA, duongLuuPhanQuyen, thanLuuPhanQuyen, m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)

	var ra cotPhanQuyenRa
	if err := json.Unmarshal(w.Body.Bytes(), &ra); err != nil {
		t.Fatalf("thân không phải JSON: %s", w.Body.String())
	}
	if ra.RoleID != "vt-chuyen-vien" || strings.Join(ra.Permissions, ",") != "document.read,task.read" {
		t.Errorf("phản hồi = %+v", ra)
	}
	g := m.ghiPhanQuyen
	if g.idCuoi != "vt-chuyen-vien" || g.xaCuoi != xaA {
		t.Errorf("use case nhận id %q xã %q", g.idCuoi, g.xaCuoi)
	}
	if strings.Join(g.dsCuoi, ",") != "task.read,document.read" {
		t.Errorf("use case nhận tập %v, muốn đúng tập đã gửi", g.dsCuoi)
	}
	// Rule 6 invariant 8: the trail's actor is the STAFF CODE, the decider the internal id.
	if g.nguoiCuoi.Vet.ID != maCanBo || g.nguoiCuoi.ID != idNoiBo {
		t.Errorf("người thực hiện = %+v, muốn Vet.ID=%q ID=%q", g.nguoiCuoi, maCanBo, idNoiBo)
	}
}

// An EMPTY set is a legitimate save and must reach the use case as [], never be refused.
func TestLuuPhanQuyenTapRongVanChuyenXuong(t *testing.T) {
	m := mayChuPhanQuyen(t, xaA)
	m.ghiPhanQuyen.kq = []string{}
	w := m.goi(t, "PUT", hostA, duongLuuPhanQuyen, `{"permissions":[]}`, m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusOK)
	if !strings.Contains(w.Body.String(), `"permissions":[]`) {
		t.Errorf("tập rỗng phải ra [] chứ không null: %s", w.Body.String())
	}
}

// A body that FORGOT the field is not a request to strip the role of everything.
func TestLuuPhanQuyenThieuTruongBiTuChoi(t *testing.T) {
	m := mayChuPhanQuyen(t, xaA)
	w := m.goi(t, "PUT", hostA, duongLuuPhanQuyen, `{}`, m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusBadRequest)
	if n := m.ghiPhanQuyen.soLanGoi(); n != 0 {
		t.Error("thiếu trường permissions mà use case vẫn chạy")
	}
}

func TestLuuPhanQuyenAnhXaTungLoi(t *testing.T) {
	cases := []struct {
		ten  string
		loi  error
		ma   int
		code string
	}{
		{"vai trò không có / xã khác", idstore.ErrVaiTroKhongTonTaiDeGhi, 404, "role_not_found"},
		{"tự lưu vai trò mình", app.ErrTuThaoTacChinhMinh, 403, "self_target_forbidden"},
		{"trao quyền không cầm", &app.LoiTraoQuyenKhongCam{Thieu: []string{"budget.confirm"}}, 403, "permission_escalation"},
		{"khoá không có", &app.LoiKhoaQuyenKhongTonTai{Thieu: []string{"x.khong-co"}}, 400, "permission_not_found"},
		{"mất admin.user", &app.LoiKhongConNguoiGiu{Khoa: "admin.user"}, 409, "last_holder"},
		{"mất admin.role", &app.LoiKhongConNguoiGiu{Khoa: "admin.role"}, 409, "last_holder"},
		{"khoá sai dạng", domain.ErrKhoaQuyenSaiDangThuc, 400, "invalid_request"},
		{"lỗi hệ thống", errors.New("pg: kết nối đứt"), 500, "internal"},
	}
	for _, c := range cases {
		t.Run(c.ten, func(t *testing.T) {
			m := mayChuPhanQuyen(t, xaA)
			m.ghiPhanQuyen.loi = c.loi
			w := m.goi(t, "PUT", hostA, duongLuuPhanQuyen, thanLuuPhanQuyen, m.tokenCho(t, xaA, sidA))
			doiMa(t, w, c.ma)
			if got := loiTra(t, w).Code; got != c.code {
				t.Errorf("code = %q, muốn %q", got, c.code)
			}
			if c.ma == 500 && strings.Contains(w.Body.String(), "kết nối") {
				t.Error("lỗi nội bộ lọt ra phản hồi")
			}
		})
	}
}

// The refused keys are NAMED — they are the same strings the matrix prints, not personal data.
func TestLuuPhanQuyenNeuTenKhoaBiTuChoi(t *testing.T) {
	m := mayChuPhanQuyen(t, xaA)
	m.ghiPhanQuyen.loi = &app.LoiTraoQuyenKhongCam{Thieu: []string{"budget.confirm", "document.route"}}
	w := m.goi(t, "PUT", hostA, duongLuuPhanQuyen, thanLuuPhanQuyen, m.tokenCho(t, xaA, sidA))
	doiMa(t, w, http.StatusForbidden)
	if msg := loiTra(t, w).Message; !strings.Contains(msg, "budget.confirm") || !strings.Contains(msg, "document.route") {
		t.Errorf("thông điệp không nêu khoá: %q", msg)
	}
}
