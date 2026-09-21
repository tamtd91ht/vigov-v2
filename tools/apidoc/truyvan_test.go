package main

import (
	"os"
	"path/filepath"
	"testing"
)

// khoThu dựng một kho giả có đúng hình dạng thật: mỗi dịch vụ là một module riêng, gốc kho
// không có go.mod.
func khoThu(t *testing.T, tepRoute string) string {
	t.Helper()
	goc := t.TempDir()
	viet := func(rel, noiDung string) {
		p := filepath.Join(goc, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(noiDung), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	viet("thu/go.mod", "module vd.test/thu\n\ngo 1.26.0\n")
	viet("core/go.mod", "module vd.test/core\n\ngo 1.26.0\n")
	viet("thu/cmd/server/main.go", "package main\n")
	viet("thu/internal/http/routes.go", tepRoute)
	return goc
}

// thamSoCua chạy trọn bộ sinh trên kho giả và trả về danh sách `parameters` của một operation.
func thamSoCua(t *testing.T, goc, duongDan, phuongThuc string) []map[string]any {
	t.Helper()
	tuyens, err := quetTuyen(goc)
	if err != nil {
		t.Fatalf("quetTuyen: %v", err)
	}
	gm, err := moGiaiMa(goc)
	if err != nil {
		t.Fatalf("moGiaiMa: %v", err)
	}
	doc, _, err := dungTaiLieu(tuyens, gm)
	if err != nil {
		t.Fatalf("dungTaiLieu: %v", err)
	}
	paths, _ := doc.gt["paths"].(*om)
	if paths == nil {
		t.Fatal("tài liệu không có paths")
	}
	muc, _ := paths.gt[duongDan].(*om)
	if muc == nil {
		t.Fatalf("không có đường dẫn %s", duongDan)
	}
	op, _ := muc.gt[phuongThuc].(*om)
	if op == nil {
		t.Fatalf("không có %s %s", phuongThuc, duongDan)
	}
	ds, _ := op.gt["parameters"].([]any)
	var ra []map[string]any
	for _, x := range ds {
		o, ok := x.(*om)
		if !ok {
			continue
		}
		ra = append(ra, o.gt)
	}
	return ra
}

func timThamSo(ds []map[string]any, ten string) map[string]any {
	for _, p := range ds {
		if p["name"] == ten {
			return p
		}
	}
	return nil
}

// Hai tham số truy vấn đọc thẳng trong handler: một cái có nhánh 400 (bắt buộc), một cái không.
//
// ĐÂY LÀ LỖ HỔNG ĐÃ ĐO ĐƯỢC. Trước bản vá này `openapi.json` không khai `parameters` nào cho
// `/api/v1/investment-projects`, nên `tsc` ở web không canh được TÊN tham số: gõ `nam` thay
// `year` thì không gì đỏ, màn hình chỉ lặng lẽ nhận 400.
func TestThamSoTruyVanDocTuHandler(t *testing.T) {
	goc := khoThu(t, `package http

import (
	"net/http"
	"strconv"

	"vd.test/core/authz"
)

type Handler struct{}

func (h *Handler) DanhSach(w http.ResponseWriter, r *http.Request) {
	thamSo := r.URL.Query()
	nam, err := strconv.Atoi(thamSo.Get("year"))
	if err != nil || nam < 2000 || nam > 2100 {
		http.Error(w, "", http.StatusBadRequest)
		return
	}
	_ = thamSo.Get("category")
	_ = nam
}

func Register(mux *http.ServeMux, h *Handler) {
	// @summary  Danh sách
	// @reply    200 -
	mux.Handle("GET /api/v1/du-an", authz.Public("lý do")(http.HandlerFunc(h.DanhSach)))
}
`)
	ds := thamSoCua(t, goc, "/api/v1/du-an", "get")
	if len(ds) != 2 {
		t.Fatalf("chờ 2 tham số truy vấn, có %d: %v", len(ds), ds)
	}
	// Thứ tự cố định theo tên — một tệp sinh đổi mỗi lượt chạy là một diff không ai đọc.
	if ds[0]["name"] != "category" || ds[1]["name"] != "year" {
		t.Errorf("thứ tự không theo tên: %v", ds)
	}
	nam := timThamSo(ds, "year")
	if nam["required"] != true {
		t.Errorf("`year` có nhánh 400 nên phải required: %v", nam)
	}
	if nam["in"] != "query" {
		t.Errorf("`year` phải là tham số truy vấn: %v", nam)
	}
	if hm := timThamSo(ds, "category"); hm["required"] != false {
		t.Errorf("`category` không có nhánh 400 nên phải required=false: %v", hm)
	}
}

// Tham số đọc trong một HÀM PHỤ cùng gói, không đọc trong chính handler.
//
// Đây là hình dạng của `/api/v1/public-holidays` và `/api/v1/swap-working-days`: cả hai gọi
// `docNamTruyVan(w, r)`, và một bộ đọc chỉ nhìn thân handler sẽ kết luận hai tuyến ấy không
// nhận tham số nào — sai, mà vẫn xanh.
func TestThamSoTruyVanQuaHamPhu(t *testing.T) {
	goc := khoThu(t, `package http

import (
	"net/http"
	"strconv"

	"vd.test/core/authz"
)

type Handler struct{}

func docNamTruyVan(w http.ResponseWriter, r *http.Request) (int, bool) {
	gt := r.URL.Query()["year"]
	if len(gt) != 1 {
		http.Error(w, "", http.StatusBadRequest)
		return 0, false
	}
	nam, err := strconv.Atoi(gt[0])
	if err != nil {
		http.Error(w, "", http.StatusBadRequest)
		return 0, false
	}
	return nam, true
}

func (h *Handler) NgayNghi(w http.ResponseWriter, r *http.Request) {
	nam, ok := docNamTruyVan(w, r)
	if !ok {
		return
	}
	_ = nam
}

func Register(mux *http.ServeMux, h *Handler) {
	// @summary  Ngày nghỉ lễ
	// @reply    200 -
	mux.Handle("GET /api/v1/ngay-nghi", authz.Public("lý do")(http.HandlerFunc(h.NgayNghi)))
}
`)
	ds := thamSoCua(t, goc, "/api/v1/ngay-nghi", "get")
	nam := timThamSo(ds, "year")
	if nam == nil {
		t.Fatalf("không thấy `year` đọc trong hàm phụ cùng gói: %v", ds)
	}
	if nam["required"] != true {
		t.Errorf("`year` bị từ chối bằng 400 nên phải required: %v", nam)
	}
}

// `@page` và tham số riêng của handler cùng nằm trong MỘT danh sách `parameters`, và tên trùng
// thì giữ bản của `@page` — bản ấy mang cả enum cột lẫn trần trang, còn hai tham số cùng tên
// trong một operation là tài liệu OpenAPI không hợp lệ.
func TestPhanTrangVaThamSoRiengKhongDeNhau(t *testing.T) {
	goc := khoThu(t, `package http

import (
	"net/http"

	"vd.test/core/authz"
	"vd.test/core/page"
)

type Handler struct{}

var SapXep = page.NewAllowlist(page.Asc, page.Col("code", "ma", page.KindText))

func (h *Handler) DanhSach(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	_ = q.Get("limit")
	_ = q.Get("status")
}

func Register(mux *http.ServeMux, h *Handler) {
	// @summary  Danh sách
	// @page     SapXep
	// @reply    200 -
	mux.Handle("GET /api/v1/thu", authz.Public("lý do")(http.HandlerFunc(h.DanhSach)))
}
`)
	// Gói `page` thật nằm ngoài kho giả, nên dựng một bản tối thiểu đủ cho `@page`.
	p := filepath.Join(goc, "core", "page", "page.go")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("package page\n\nconst (\n\tDefaultLimit = 20\n\tMaxLimit     = 100\n)\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ds := thamSoCua(t, goc, "/api/v1/thu", "get")
	dem := map[string]int{}
	for _, x := range ds {
		dem[x["name"].(string)]++
	}
	for ten, n := range dem {
		if n > 1 {
			t.Errorf("tham số %q xuất hiện %d lần trong một operation", ten, n)
		}
	}
	if timThamSo(ds, "status") == nil {
		t.Errorf("mất tham số riêng của handler khi tuyến có @page: %v", ds)
	}
	limit := timThamSo(ds, "limit")
	if limit == nil {
		t.Fatalf("mất `limit` của @page: %v", ds)
	}
	// Bản của `@page` mang schema kiểu số kèm trần; bản suy từ handler là chuỗi trơn.
	s, _ := limit["schema"].(*om)
	if s == nil || s.gt["type"] != "integer" {
		t.Errorf("`limit` phải giữ bản của @page (integer, có trần), gặp: %v", limit)
	}
}

// Ba tuyến thật của kho, đọc từ chính mã nguồn. Không phải golden file: golden file phải sửa tay
// mỗi lần đổi hợp pháp, còn thứ cần ghim ở đây là hợp đồng nói ĐÚNG cái handler làm.
func TestBaTuyenThatCoThamSoNam(t *testing.T) {
	goc, err := timGoc()
	if err != nil {
		t.Fatal(err)
	}
	tuyens, err := quetTuyen(goc)
	if err != nil {
		t.Fatal(err)
	}
	gm, err := moGiaiMa(goc)
	if err != nil {
		t.Fatal(err)
	}
	theoKhoa := map[string]tuyen{}
	for _, x := range tuyens {
		theoKhoa[x.Method+" "+x.Path] = x
	}

	for _, tr := range []struct {
		khoa string
		ten  []string
	}{
		{"GET /api/v1/investment-projects", []string{"category", "year"}},
		{"GET /api/v1/public-holidays", []string{"year"}},
		{"GET /api/v1/swap-working-days", []string{"year"}},
	} {
		x, ok := theoKhoa[tr.khoa]
		if !ok {
			t.Errorf("không trích được %s", tr.khoa)
			continue
		}
		ts, err := gm.thamSoTruyVanCua(x.pkgDir, x.Handler)
		if err != nil {
			t.Errorf("%s: %v", tr.khoa, err)
			continue
		}
		var ten []string
		for _, p := range ts {
			ten = append(ten, p.Ten)
			// `year` là bắt buộc ở cả ba tuyến: handler trả 400 khi thiếu.
			if p.Ten == "year" && !p.BatBuoc {
				t.Errorf("%s: `year` phải là bắt buộc", tr.khoa)
			}
		}
		if len(ten) != len(tr.ten) {
			t.Errorf("%s: chờ %v, gặp %v", tr.khoa, tr.ten, ten)
			continue
		}
		for i := range ten {
			if ten[i] != tr.ten[i] {
				t.Errorf("%s: chờ %v, gặp %v", tr.khoa, tr.ten, ten)
				break
			}
		}
	}
}
