package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// moduleGiaPhanTrang dựng một kho giả có `core/page` thật, vì `@page` giải biến bằng cách đọc
// chính gói `page` — cả danh sách cột lẫn hai hằng số giới hạn trang.
func moduleGiaPhanTrang(t *testing.T, kho string) (*giaiMa, string) {
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
	viet("core/go.mod", "module vd.test/core\n\ngo 1.26.0\n")
	viet("core/page/page.go", "package page\n\n"+
		"const (\n\tDefaultLimit = 20\n\tMaxLimit     = 100\n)\n\n"+
		"type Dir string\n\nconst (\n\tAsc  Dir = \"asc\"\n\tDesc Dir = \"desc\"\n)\n\n"+
		"type Kind string\n\nconst (\n\tKindText Kind = \"text\"\n\tKindTime Kind = \"time\"\n)\n\n"+
		"type Column struct{}\ntype Allowlist struct{}\n\n"+
		"func Col(a, b string, k Kind) Column { return Column{} }\n"+
		"func NewAllowlist(d Dir, c Column, r ...Column) Allowlist { return Allowlist{} }\n")
	viet("thu/go.mod", "module vd.test/thu\n\ngo 1.26.0\n")
	viet("thu/cmd/server/main.go", "package main\n")
	viet("thu/internal/store/kho.go", kho)
	// Nhập KHÔNG dùng `_`: bí danh của một import trắng là `_`, nên `@page store.SapXep` sẽ
	// không giải được — đúng như bộ sinh sẽ báo, nhưng đó không phải ca bài này muốn kiểm.
	viet("thu/internal/http/api.go", "package http\n\nimport \"vd.test/thu/internal/store\"\n")

	gm, err := moGiaiMa(goc)
	if err != nil {
		t.Fatal(err)
	}
	return gm, filepath.Join(goc, "thu", "internal", "http")
}

const khoSapXep = "package store\n\n" +
	"import \"vd.test/core/page\"\n\n" +
	"var SapXep = page.NewAllowlist(page.Asc,\n" +
	"\tpage.Col(\"code\", \"ma\", page.KindText),\n" +
	"\tpage.Col(\"created_at\", \"tao_luc\", page.KindTime),\n" +
	")\n"

// `@page` là một CON TRỎ tới danh sách trắng, không phải một bản sao của nó. Bài này ghim rằng
// bộ sinh thật sự ĐỌC biến Go ấy — cả tên cột, chiều mặc định, lẫn hai hằng số giới hạn trang.
// Nếu nó chép sẵn ở đâu đó thì thay đổi trong mã Go sẽ không tới được hợp đồng, và web sẽ gõ
// tay lại danh sách cột — đúng chỗ đã từng phải gõ tay.
func TestPageDocDuocDanhSachTrangTuMaGo(t *testing.T) {
	gm, dir := moduleGiaPhanTrang(t, khoSapXep)

	pt, err := gm.docPhanTrang(dir, "store.SapXep")
	if err != nil {
		t.Fatalf("không đọc được danh sách trắng: %v", err)
	}
	if got := strings.Join(pt.Cot, ","); got != "code,created_at" {
		t.Errorf("cột = %q, muốn code,created_at", got)
	}
	if pt.ChieuMacD != "asc" {
		t.Errorf("chiều mặc định = %q, muốn asc", pt.ChieuMacD)
	}
	// Hai con số này đọc từ `core/page`, KHÔNG gõ trong apidoc — nếu gõ thì apidoc lại thành
	// bản chép thứ hai, đúng thứ nó sinh ra để xoá.
	if pt.LimitMacD != 20 || pt.LimitMax != 100 {
		t.Errorf("giới hạn = %d/%d, muốn 20/100 — đọc từ hằng số của gói page", pt.LimitMacD, pt.LimitMax)
	}
}

func TestPageThemMotCotTrongMaGoThiHopDongDoiTheo(t *testing.T) {
	kho := strings.Replace(khoSapXep,
		"\tpage.Col(\"created_at\", \"tao_luc\", page.KindTime),\n",
		"\tpage.Col(\"created_at\", \"tao_luc\", page.KindTime),\n\tpage.Col(\"email\", \"email\", page.KindText),\n",
		1)
	gm, dir := moduleGiaPhanTrang(t, kho)

	pt, err := gm.docPhanTrang(dir, "store.SapXep")
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(pt.Cot, ","); got != "code,created_at,email" {
		t.Fatalf("cột = %q — thay đổi trong mã Go KHÔNG tới được hợp đồng", got)
	}
}

// Dựng danh sách bằng vòng lặp hay bằng một hàm khác thì bộ sinh phải DỪNG, không đoán. Một hợp
// đồng công bố danh sách cột sai mà trông đúng là thứ web dựng màn hình lên trên.
func TestPageKhaiKhongDocDuocThiTuChoi(t *testing.T) {
	for ten, kho := range map[string]string{
		"dựng bằng hàm khác": "package store\n\nimport \"vd.test/core/page\"\n\n" +
			"func dung() page.Allowlist { return page.NewAllowlist(page.Asc, page.Col(\"code\", \"ma\", page.KindText)) }\n\n" +
			"var SapXep = dung()\n",
		"cột không phải page.Col": "package store\n\nimport \"vd.test/core/page\"\n\n" +
			"var mot = page.Col(\"code\", \"ma\", page.KindText)\n\n" +
			"var SapXep = page.NewAllowlist(page.Asc, mot)\n",
		"chiều mặc định lạ": "package store\n\nimport \"vd.test/core/page\"\n\n" +
			"var SapXep = page.NewAllowlist(page.Kind(\"\"), page.Col(\"code\", \"ma\", page.KindText))\n",
	} {
		t.Run(ten, func(t *testing.T) {
			gm, dir := moduleGiaPhanTrang(t, kho)
			if _, err := gm.docPhanTrang(dir, "store.SapXep"); err == nil {
				t.Error("nhận một khai báo không đọc được — apidoc đang đoán danh sách cột")
			}
		})
	}
}

func TestPageTroSaiThiDungChuKhongImLang(t *testing.T) {
	gm, dir := moduleGiaPhanTrang(t, khoSapXep)

	if _, err := gm.docPhanTrang(dir, "store.KhongCo"); err == nil {
		t.Error("nhận một tên biến không tồn tại")
	}
	if _, err := gm.docPhanTrang(dir, "khongcogoi.SapXep"); err == nil {
		t.Error("nhận một bí danh import không tồn tại")
	}
}

// khoThuPhanTrang dựng một kho giả có `core/page` THẬT-ĐỦ-DÙNG và một tệp route, để chạy trọn bộ
// sinh trên nó — khác `moduleGiaPhanTrang` ở chỗ có tuyến, nên đo được cả phần openapi.
func khoThuPhanTrang(t *testing.T, tepRoute string) string {
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
	viet("core/go.mod", "module vd.test/core\n\ngo 1.26.0\n")
	viet("core/page/page.go", "package page\n\n"+
		"const (\n\tDefaultLimit = 20\n\tMaxLimit     = 100\n)\n\n"+
		"type Dir string\n\nconst (\n\tAsc  Dir = \"asc\"\n\tDesc Dir = \"desc\"\n)\n\n"+
		"type Kind string\n\nconst (\n\tKindText Kind = \"text\"\n\tKindTime Kind = \"time\"\n)\n\n"+
		"type Column struct{}\ntype Allowlist struct{}\ntype Request struct{}\n\n"+
		"func Col(a, b string, k Kind) Column { return Column{} }\n"+
		"func NewAllowlist(d Dir, c Column, r ...Column) Allowlist { return Allowlist{} }\n"+
		"func Parse(q map[string][]string, a Allowlist) (Request, error) { return Request{}, nil }\n")
	viet("thu/go.mod", "module vd.test/thu\n\ngo 1.26.0\n")
	viet("thu/cmd/server/main.go", "package main\n")
	viet("thu/internal/store/kho.go", khoSapXep)
	viet("thu/internal/http/routes.go", tepRoute)
	return goc
}

// PHÂN TRANG SUY TỪ `page.Parse`, KHÔNG ĐÒI `@page`.
//
// LỖ HỔNG ĐÃ ĐO 23/09/2026: sáu tuyến của kho gọi `page.Parse`, đúng MỘT có dòng `@page`. Năm
// tuyến còn lại phân trang thật mà hợp đồng im về `limit · cursor · sort · order` — kể cả DANH
// SÁCH CỘT được phép sắp xếp, thứ web đã từng phải gõ tay.
func TestPhanTrangSuyTuLoiGoiParse(t *testing.T) {
	goc := khoThuPhanTrang(t, `package http

import (
	"net/http"

	"vd.test/core/authz"
	"vd.test/core/page"
	"vd.test/thu/internal/store"
)

type Handler struct{}

func (h *Handler) DanhSach(w http.ResponseWriter, r *http.Request) {
	yc, err := page.Parse(r.URL.Query(), store.SapXep)
	if err != nil {
		http.Error(w, "", http.StatusBadRequest)
		return
	}
	_ = yc
}

func Register(mux *http.ServeMux, h *Handler) {
	// @summary  Danh sách
	// @reply    200 -
	mux.Handle("GET /api/v1/so", authz.Public("lý do")(http.HandlerFunc(h.DanhSach)))
}
`)
	ds := thamSoCua(t, goc, "/api/v1/so", "get")
	for _, ten := range []string{"limit", "cursor", "sort", "order"} {
		if timThamSo(ds, ten) == nil {
			t.Fatalf("tuyến gọi page.Parse mà hợp đồng thiếu %q: %v", ten, ds)
		}
	}
	// `sort` phải mang ĐÚNG danh sách cột của biến Go — đó là toàn bộ điểm của việc suy từ mã.
	s, _ := timThamSo(ds, "sort")["schema"].(*om)
	if s == nil {
		t.Fatalf("`sort` không có schema: %v", ds)
	}
	enum, _ := s.gt["enum"].([]any)
	if len(enum) != 2 || enum[0] != "code" || enum[1] != "created_at" {
		t.Errorf("`sort` không mang danh sách cột thật của store.SapXep: %v", enum)
	}
}

// Hai danh sách trắng KHÁC NHAU cùng với tới được từ một handler là một mâu thuẫn thật — chọn bừa
// một bên là công bố một danh sách cột có thể sai, nên bộ sinh DỪNG và nói khai `@page`.
func TestHaiDanhSachTrangThiDungChuKhongChonBua(t *testing.T) {
	goc := khoThuPhanTrang(t, `package http

import (
	"net/http"

	"vd.test/core/authz"
	"vd.test/core/page"
	"vd.test/thu/internal/store"
)

type Handler struct{}

func (h *Handler) DanhSach(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("kieu") == "khac" {
		yc, _ := page.Parse(r.URL.Query(), store.SapXepKhac)
		_ = yc
		return
	}
	yc, _ := page.Parse(r.URL.Query(), store.SapXep)
	_ = yc
}

func Register(mux *http.ServeMux, h *Handler) {
	// @summary  Danh sách
	// @reply    200 -
	mux.Handle("GET /api/v1/so", authz.Public("lý do")(http.HandlerFunc(h.DanhSach)))
}
`)
	// Biến thứ hai, để hai lời gọi trỏ vào hai danh sách thật khác nhau.
	p := filepath.Join(goc, "thu", "internal", "store", "kho2.go")
	noi := "package store\n\nimport \"vd.test/core/page\"\n\n" +
		"var SapXepKhac = page.NewAllowlist(page.Desc, page.Col(\"updated_at\", \"sua_luc\", page.KindTime))\n"
	if err := os.WriteFile(p, []byte(noi), 0o644); err != nil {
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
	if _, _, err := dungTaiLieu(tuyens, gm); err == nil {
		t.Fatal("bộ sinh chọn bừa một trong hai danh sách trắng thay vì dừng")
	} else if !strings.Contains(err.Error(), "@page") {
		t.Errorf("thông báo không chỉ ra cách sửa (khai `@page`): %v", err)
	}
}

// `@page` THẮNG khi có mặt: nó là con trỏ do người viết đặt, và một suy luận âm thầm sửa hộ là
// một hợp đồng không ai giải thích được.
func TestPageKhaiTayThangSuyLuan(t *testing.T) {
	goc := khoThuPhanTrang(t, `package http

import (
	"net/http"

	"vd.test/core/authz"
	"vd.test/core/page"
	"vd.test/thu/internal/store"
)

type Handler struct{}

func (h *Handler) DanhSach(w http.ResponseWriter, r *http.Request) {
	yc, _ := page.Parse(r.URL.Query(), store.SapXep)
	_ = yc
}

func Register(mux *http.ServeMux, h *Handler) {
	// @summary  Danh sách
	// @page     store.SapXepKhac
	// @reply    200 -
	mux.Handle("GET /api/v1/so", authz.Public("lý do")(http.HandlerFunc(h.DanhSach)))
}
`)
	p := filepath.Join(goc, "thu", "internal", "store", "kho2.go")
	noi := "package store\n\nimport \"vd.test/core/page\"\n\n" +
		"var SapXepKhac = page.NewAllowlist(page.Desc, page.Col(\"updated_at\", \"sua_luc\", page.KindTime))\n"
	if err := os.WriteFile(p, []byte(noi), 0o644); err != nil {
		t.Fatal(err)
	}

	ds := thamSoCua(t, goc, "/api/v1/so", "get")
	s, _ := timThamSo(ds, "sort")["schema"].(*om)
	if s == nil {
		t.Fatalf("`sort` không có schema: %v", ds)
	}
	enum, _ := s.gt["enum"].([]any)
	if len(enum) != 1 || enum[0] != "updated_at" {
		t.Errorf("suy luận đã đè lên `@page` do người viết đặt: %v", enum)
	}
}

// Sáu tuyến THẬT của kho gọi `page.Parse`, và cả sáu phải có đủ bốn tham số phân trang — kể cả
// năm tuyến không có dòng `@page` nào.
func TestSauTuyenThatCoDuThamSoPhanTrang(t *testing.T) {
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
	for khoa, sapXep := range map[string]string{
		"GET /api/v1/tasks":              "petstore.SapXepNhiemVu",
		"GET /api/v1/citizen-reports":    "petstore.SapXepPhieu",
		"GET /api/v1/meetings":           "petstore.SapXepBienBan",
		"GET /api/v1/incoming-documents": "docstore.SapXepVanBanDen",
		"GET /api/v1/outgoing-documents": "docstore.SapXepVanBanDi",
		"GET /api/v1/staff":              "idstore.SapXepCanBo",
	} {
		x, ok := theoKhoa[khoa]
		if !ok {
			t.Errorf("không trích được %s", khoa)
			continue
		}
		ten := x.Page
		if ten == "" {
			ten, err = gm.sapXepSuyTuMa(x.pkgDir, x.Handler)
			if err != nil {
				t.Errorf("%s: %v", khoa, err)
				continue
			}
		}
		if ten != sapXep {
			t.Errorf("%s: chờ danh sách trắng %q, gặp %q", khoa, sapXep, ten)
			continue
		}
		if _, err := gm.docPhanTrang(x.pkgDir, ten); err != nil {
			t.Errorf("%s: không đọc được %s: %v", khoa, ten, err)
		}
	}
}
