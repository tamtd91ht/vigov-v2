package http

import (
	"net/http"
	"os"
	"regexp"
	"testing"

	"github.com/vihat/vigov/core/authz"
)

// EVERY WRITE ROUTE OF THE MEETING REGISTER, ENUMERATED FROM routes.go, MUST REACH A KNOWN ACT OF
// GhiBienBanUseCase.
//
// The archival promise (signed minutes cannot be changed) is proved per ACT in internal/app —
// bien_ban_hop_da_ky_quet_test.go enumerates GhiBienBanHop's methods by reflection. That proof is only
// worth something if every door into the register goes through one of those acts. A new write route
// wired to a handler that updates the store another way, or to a new use case type, would bypass it
// with every existing test still green — nothing lists the routes.
//
// So this file reads the registrations from the source and fails on a write route under
// /api/v1/meetings it does not know, then drives each known one and checks which act it reached.
// A new route fails here until somebody names its act — and a new act fails the app sweep until
// somebody says what it does to signed minutes.

// tuyenGhiBienBan maps each write route to the act the fake records (ghiBienBanGia.viec).
var tuyenGhiBienBan = map[string]struct {
	duong string
	than  any
	khoa  authz.Perm
	viec  string
}{
	"POST /api/v1/meetings":                                         {duongMeetings, thanTaoBienBan(), "task.create", "tao"},
	"POST /api/v1/meetings/{id}/conclusions":                        {duongKetLuan(idBBThu), themKetLuanVao{Content: "Một kết luận mới."}, "task.create", "them-ket-luan"},
	"POST /api/v1/meetings/{id}/conclusions/{stt}/task":             {duongTachNV(idBBThu, 1), thanTachNhiemVu(), "task.create", "tach"},
	"PATCH /api/v1/meetings/{id}":                                   {duongBB(idBBThu), suaBienBanVao{Title: ptr("Tên mới")}, "task.create", "sua"},
	"DELETE /api/v1/meetings/{id}":                                  {duongBB(idBBThu), xoaBienBanVao{Reason: "Nhập trùng."}, "task.create", "xoa"},
	"POST /api/v1/meetings/{id}/signature":                          {duongBB(idBBThu) + "/signature", nil, "task.approve", "ky"},
	"PATCH /api/v1/meetings/{id}/conclusions/{stt}":                 {duongKL(idBBThu, 1), suaKetLuanVao{Content: "Câu mới."}, "task.create", "sua-ket-luan"},
	"DELETE /api/v1/meetings/{id}/conclusions/{stt}":                {duongKL(idBBThu, 1), xoaBienBanVao{Reason: "Ghi nhầm."}, "task.create", "xoa-ket-luan"},
	"PUT /api/v1/meetings/{id}/conclusions/{stt}/no-task-marker":    {duongDauKL(idBBThu, 1), nil, "task.create", "danh-dau"},
	"DELETE /api/v1/meetings/{id}/conclusions/{stt}/no-task-marker": {duongDauKL(idBBThu, 1), nil, "task.create", "bo-dau"},
}

func TestTuyenGhiBienBan_MoiTuyenTrongRoutesDeuDaBiet(t *testing.T) {
	nguon, err := os.ReadFile("routes.go")
	if err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(`mux\.Handle\("((?:POST|PUT|PATCH|DELETE) /api/v1/meetings[^"]*)"`)
	thay := map[string]bool{}
	for _, m := range re.FindAllStringSubmatch(string(nguon), -1) {
		thay[m[1]] = true
		if _, co := tuyenGhiBienBan[m[1]]; !co {
			t.Errorf("tuyến ghi MỚI %q vào sổ biên bản chưa được gắn với hành vi nào của GhiBienBanUseCase — "+
				"thêm vào tuyenGhiBienBan, và nếu là hành vi mới thì phân loại nó trong "+
				"internal/app/bien_ban_hop_da_ky_quet_test.go", m[1])
		}
	}
	if len(thay) == 0 {
		t.Fatal("không đọc được tuyến nào từ routes.go — biểu thức dò đã lệch với cách đăng ký tuyến")
	}
	for tuyen := range tuyenGhiBienBan {
		if !thay[tuyen] {
			t.Errorf("tuyenGhiBienBan có %q nhưng routes.go không còn đăng ký", tuyen)
		}
	}
}

func TestTuyenGhiBienBan_MoiTuyenDiQuaDungHanhVi(t *testing.T) {
	for tuyen, ca := range tuyenGhiBienBan {
		t.Run(tuyen, func(t *testing.T) {
			m := dungMayChu(t)
			m.capQuyen(t, ca.khoa)
			method := regexp.MustCompile(`^\S+`).FindString(tuyen)
			w := m.goiGhiNV(t, method, hostA, ca.duong, canBoCuaXa(xaA), ca.than)
			if w.Code >= http.StatusBadRequest {
				t.Fatalf("mã %d: %s", w.Code, w.Body.String())
			}
			if m.ghiBienBan.goi != 1 || m.ghiBienBan.viec != ca.viec {
				t.Errorf("tới hành vi %q (%d lần), muốn %q đúng một lần", m.ghiBienBan.viec, m.ghiBienBan.goi, ca.viec)
			}
		})
	}
}
