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
