package main

import (
	"strings"
	"testing"
)

// LỚP XÃ CỦA TUYẾN CÔNG DÂN — ADR 0022.
//
// Ràng buộc "danh sách endpoint KhongThuocXa phải liệt kê được" được giải bằng SINH, không
// bằng một danh sách giữ tay: một danh sách giữ tay cạnh các tuyến nghiệp vụ là lời mời thêm
// nhầm một tuyến nghiệp vụ vào đó, và không có gì đỏ khi điều ấy xảy ra.
//
// dauFile của route_test.go đã import authz và idem; lớp xã nằm ở gói httpx nên các bài dưới
// đây tự viết phần đầu tệp.

const dauFileCongDan = `package http

import (
	"net/http"

	"github.com/vihat/vigov/core/authz"
	"github.com/vihat/vigov/core/httpx"
)

func Register(mux *http.ServeMux) {
`

func TestTuyenCongDanKhongKhaiLopXaLaLoi(t *testing.T) {
	_, errs := trich(t, dauFileCongDan+`
	// @summary  Phản ánh của tôi
	// @reply    200 -
	mux.Handle("GET /api/v1/phan-anh/{ma}",
		authz.CitizenOnly()(http.HandlerFunc(nil)))
}
`)
	if len(errs) != 1 || !strings.Contains(errs[0].Error(), "lớp xã") {
		t.Fatalf("mong lỗi thiếu khai lớp xã, được: %v", errs)
	}
}

func TestTuyenCongDanKhaiXaTuPhienThiQua(t *testing.T) {
	ts, errs := trich(t, dauFileCongDan+`
	// @summary  Phản ánh của tôi
	// @reply    200 -
	mux.Handle("GET /api/v1/phan-anh/{ma}",
		authz.CitizenOnly()(
			httpx.XaTuPhien()(http.HandlerFunc(nil))))
}
`)
	if len(errs) != 0 {
		t.Fatalf("không mong đợi lỗi: %v", errs)
	}
	if len(ts) != 1 || ts[0].Xa.Kind != "tu-phien" {
		t.Fatalf("lớp xã đọc sai: %+v", ts)
	}
}

// Lý do của KhongThuocXa phải vào được hợp đồng — đó là toàn bộ điểm của việc SINH danh sách
// miễn trừ thay vì giữ tay.
func TestKhongThuocXaMangLyDoVaoHopDong(t *testing.T) {
	ts, errs := trich(t, dauFileCongDan+`
	// @summary  Danh mục xã
	// @reply    200 -
	mux.Handle("GET /api/v1/communes",
		authz.CitizenOnly()(
			httpx.KhongThuocXa("màn hình chọn xã: công dân chưa chọn xã nào, đây là lời gọi TẠO RA lựa chọn đó")(
				http.HandlerFunc(nil))))
}
`)
	if len(errs) != 0 {
		t.Fatalf("không mong đợi lỗi: %v", errs)
	}
	if ts[0].Xa.Kind != "khong-thuoc-xa" || !strings.Contains(ts[0].Xa.LyDo, "chọn xã") {
		t.Fatalf("lý do không vào được hợp đồng: %+v", ts[0].Xa)
	}

	o := xaJSON(ts[0].Xa)
	if !o.co("reason") || o.gt["tenant_in_context"] != false {
		t.Fatalf("x-vigov-tenant-class thiếu lý do hoặc nói sai về context: %+v", o.gt)
	}
}

func TestKhongThuocXaKhongCoLyDoLaLoi(t *testing.T) {
	_, errs := trich(t, dauFileCongDan+`
	// @summary  Danh mục xã
	// @reply    200 -
	mux.Handle("GET /api/v1/communes",
		authz.CitizenOnly()(
			httpx.KhongThuocXa("")(http.HandlerFunc(nil))))
}
`)
	if len(errs) != 1 || !strings.Contains(errs[0].Error(), "lý do") {
		t.Fatalf("mong lỗi thiếu lý do, được: %v", errs)
	}
}

// PHÉP KIỂM THỨ HAI CỦA ADR 0022: hai trục vuông góc. `authz.*` trả lời AI, lớp xã trả lời XÃ
// NÀO. Một tuyến cán bộ mượn lối miễn trừ của kênh công dân là một tuyến cán bộ không có xã và
// không ai nhận ra — chặn lúc dựng.
func TestLopXaTrenTuyenKhongPhaiCongDanLaLoi(t *testing.T) {
	cac := []struct {
		ten   string
		nguon string
	}{
		{"KhongThuocXa trên tuyến có quyền", `
	// @summary  Tuyến cán bộ
	// @reply    200 -
	mux.Handle("GET /api/v1/ho-so",
		authz.RequirePermission(nil, "hoso.read")(
			httpx.KhongThuocXa("lý do nghe rất hợp lý")(http.HandlerFunc(nil))))
}
`},
		{"KhongThuocXa trên tuyến public", `
	// @summary  Tuyến công khai
	// @reply    200 -
	mux.Handle("GET /api/v1/cong-khai",
		authz.Public("trang giới thiệu")(
			httpx.KhongThuocXa("lý do nghe rất hợp lý")(http.HandlerFunc(nil))))
}
`},
		{"XaTuPhien trên tuyến cán bộ", `
	// @summary  Tuyến cán bộ
	// @reply    200 -
	mux.Handle("GET /api/v1/ho-so",
		authz.RequirePermission(nil, "hoso.read")(
			httpx.XaTuPhien()(http.HandlerFunc(nil))))
}
`},
	}
	for _, c := range cac {
		t.Run(c.ten, func(t *testing.T) {
			_, errs := trich(t, dauFileCongDan+c.nguon)
			if len(errs) != 1 || !strings.Contains(errs[0].Error(), "CHỈ thuộc tuyến công dân") {
				t.Fatalf("mong lỗi lớp xã trên tuyến không phải công dân, được: %v", errs)
			}
		})
	}
}

// Tuyến CÁN BỘ không khai lớp xã vẫn phải qua: xã của nó đến từ Host, ở TenantMiddleware. Nếu
// phép kiểm mới bắt nhầm cả tuyến cán bộ thì toàn bộ hợp đồng REST hiện có đỏ cùng lúc.
func TestTuyenCanBoKhongCanKhaiLopXa(t *testing.T) {
	ts, errs := trich(t, dauFileCongDan+`
	// @summary  Tuyến cán bộ
	// @reply    200 -
	mux.Handle("GET /api/v1/ho-so",
		authz.RequirePermission(nil, "hoso.read")(http.HandlerFunc(nil)))
}
`)
	if len(errs) != 0 || len(ts) != 1 {
		t.Fatalf("tuyến cán bộ bị bắt nhầm: %v", errs)
	}
	if ts[0].Xa.Kind != "" {
		t.Fatalf("tuyến cán bộ có lớp xã: %+v", ts[0].Xa)
	}
}

// ADR 0045: the view-only session class waives the PHONE, so its reason must reach the contract,
// and the two session classes must stay distinguishable there.
func TestXaTuPhienChiXemMangLyDoVaoHopDong(t *testing.T) {
	ts, errs := trich(t, dauFileCongDan+`
	// @summary  Hồ sơ hiển thị của xã
	// @reply    200 -
	mux.Handle("GET /api/v1/commune-profile",
		authz.CitizenOnly()(
			httpx.XaTuPhienChiXem("hồ sơ hiển thị của xã: ai mở app của xã cũng xem được")(
				http.HandlerFunc(nil))))
}
`)
	if len(errs) != 0 {
		t.Fatalf("không mong đợi lỗi: %v", errs)
	}
	if ts[0].Xa.Kind != "tu-phien-chi-xem" || !strings.Contains(ts[0].Xa.LyDo, "hồ sơ hiển thị") {
		t.Fatalf("lớp chỉ xem đọc sai: %+v", ts[0].Xa)
	}
	o := xaJSON(ts[0].Xa)
	if !o.co("reason") || o.gt["tenant_in_context"] != true || o.co("phone_verified_required") {
		t.Fatalf("x-vigov-tenant-class của lớp chỉ xem sai: %+v", o.gt)
	}
	if o := xaJSON(xaDecl{Kind: "tu-phien"}); o.gt["phone_verified_required"] != true {
		t.Fatalf("lớp tu-phien không ghi đòi số đã xác thực: %+v", o.gt)
	}
}

func TestXaTuPhienChiXemKhongCoLyDoLaLoi(t *testing.T) {
	_, errs := trich(t, dauFileCongDan+`
	// @summary  Hồ sơ hiển thị của xã
	// @reply    200 -
	mux.Handle("GET /api/v1/commune-profile",
		authz.CitizenOnly()(
			httpx.XaTuPhienChiXem("")(http.HandlerFunc(nil))))
}
`)
	if len(errs) != 1 || !strings.Contains(errs[0].Error(), "lý do") {
		t.Fatalf("mong lỗi thiếu lý do, được: %v", errs)
	}
}
