package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// moduleGia builds a throwaway module so the type resolver has a real go.mod, a real package
// directory and a real cross-package import to follow — the same three things it has in the
// repository.
func moduleGia(t *testing.T, api, chung string) (*giaiMa, string) {
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
	viet("go.mod", "module vd.test\n\ngo 1.26.0\n")
	viet("core/chung/chung.go", chung)
	viet("thu/cmd/server/main.go", "package main\n")
	viet("thu/internal/http/api.go", api)

	gm, err := moGiaiMa(goc)
	if err != nil {
		t.Fatal(err)
	}
	return gm, filepath.Join(goc, "thu", "internal", "http")
}

const chungMacDinh = `package chung

type Loi struct {
	Ma string ` + "`json:\"code\"`" + `
}
`

// sinhSchema resolves one named type and returns its component JSON.
func sinhSchema(t *testing.T, api, ten string, nghiem bool) (string, error) {
	t.Helper()
	gm, dir := moduleGia(t, api, chungMacDinh)
	bs := moBoSchema(gm)
	comp, err := bs.refTheoTen(dir, ten, nghiem)
	if err != nil {
		return "", err
	}
	b, err := json.Marshal(bs.comps[comp])
	if err != nil {
		t.Fatal(err)
	}
	return string(b), nil
}

func TestTheJsonOmitemptyBoQuaLongNhauSliceThoiGian(t *testing.T) {
	api := `package http

import (
	"time"

	"vd.test/core/chung"
)

type Con struct {
	Ten string ` + "`json:\"ten\"`" + `
}

type Cha struct {
	Ma       string    ` + "`json:\"ma\"`" + `
	GhiChu   string    ` + "`json:\"ghi_chu,omitempty\"`" + `
	NoiBo    string    ` + "`json:\"-\"`" + `
	an       string    ` + "`json:\"an\"`" + `
	Con      Con       ` + "`json:\"con\"`" + `
	DanhSach []Con     ` + "`json:\"danh_sach\"`" + `
	TaoLuc   time.Time ` + "`json:\"tao_luc\"`" + `
	TuyChon  *string   ` + "`json:\"tuy_chon\"`" + `
	Loi      chung.Loi ` + "`json:\"loi\"`" + `
	SoLuong  int       ` + "`json:\"so_luong\"`" + `
}
`
	got, err := sinhSchema(t, api, "Cha", true)
	if err != nil {
		t.Fatal(err)
	}

	muon := []string{
		`"ma":{"type":"string"}`,
		`"ghi_chu":{"type":"string"}`,
		`"con":{"$ref":"#/components/schemas/thu.Con"}`,
		`"danh_sach":{"type":"array","items":{"$ref":"#/components/schemas/thu.Con"}}`,
		`"tao_luc":{"type":"string","format":"date-time"}`,
		`"tuy_chon":{"type":["string","null"]}`,
		`"loi":{"$ref":"#/components/schemas/chung.Loi"}`,
		`"so_luong":{"type":"integer"}`,
	}
	for _, m := range muon {
		if !strings.Contains(got, m) {
			t.Errorf("thiếu %s trong:\n%s", m, got)
		}
	}

	// json:"-" keeps a field off the wire; an unexported field never reaches it at all.
	if strings.Contains(got, "NoiBo") || strings.Contains(got, `"an"`) {
		t.Errorf("trường không được ra ngoài lại xuất hiện:\n%s", got)
	}

	// ONLY omitempty decides presence. A pointer decides NULLABILITY, which is a different
	// fact and is already written at `type: [T, "null"]` - see `tuy_chon` above.
	//
	// So `tuy_chon` IS required: encoding/json emits it on every response, as `null` when the
	// pointer is nil. It is `ghi_chu`, carrying omitempty, that may genuinely be absent.
	// Publishing a pointer as optional makes a client unable to tell "the server said: nothing
	// here" from "the server did not say", and makes it write a branch that never runs.
	if !strings.Contains(got, `"required":["con","danh_sach","loi","ma","so_luong","tao_luc","tuy_chon"]`) {
		t.Errorf("danh sách required sai:\n%s", got)
	}
}

// The single most expensive guess this tool could make.
//
// encoding/json would emit the field under its Go name. Inferring that here publishes a field
// name nobody chose — a contract that is wrong while looking right, which a web client then
// builds a screen on.
func TestTruongKhongCoTheJsonThiBaoLoi(t *testing.T) {
	api := `package http

type Cha struct {
	Ma      string ` + "`json:\"ma\"`" + `
	QuenThe string
}
`
	_, err := sinhSchema(t, api, "Cha", true)
	if err == nil {
		t.Fatal("mong lỗi, được nil — apidoc đã đoán tên trường")
	}
	if !strings.Contains(err.Error(), "QuenThe") || !strings.Contains(err.Error(), "không có thẻ json") {
		t.Fatalf("thông báo lỗi phải nêu đúng tên trường, được: %v", err)
	}
}

// Rule 3 and rule 8 — the case this whole guard exists for.
func TestTruongBiMatTrongPhanHoiThiDung(t *testing.T) {
	api := `package http

type PhanHoi struct {
	Ma          string ` + "`json:\"ma\"`" + `
	MatKhauHash string ` + "`json:\"mat_khau_hash\"`" + `
}
`
	_, err := sinhSchema(t, api, "PhanHoi", true)
	if err == nil {
		t.Fatal("mong DỪNG, được nil — hợp đồng vừa công bố một hình dạng chứa mật khẩu")
	}
	if !strings.Contains(err.Error(), "MatKhauHash") {
		t.Fatalf("thông báo phải nêu tên trường, được: %v", err)
	}
	if !strings.Contains(err.Error(), "luật 3") {
		t.Fatalf("thông báo phải trỏ về luật, được: %v", err)
	}
}

func TestNhieuTenBiMatDeuBiBat(t *testing.T) {
	for _, ten := range []string{
		"MatKhau", "Password", "Token", "Secret", "Khoa", "ApiKey", "RefreshToken",
		"TokenHoa", "MatKhauHash", "KhoaKy", "Otp",
	} {
		api := "package http\n\ntype PhanHoi struct {\n\t" + ten + " string `json:\"x\"`\n}\n"
		if _, err := sinhSchema(t, api, "PhanHoi", true); err == nil {
			t.Errorf("%s lọt qua bộ lọc bí mật", ten)
		}
	}
}

// The counterpart: a name that merely LOOKS like one of the short forms must not fire, or the
// guard becomes noise and somebody removes it — and then the layer is gone entirely.
//
// The short words (khoa, key, hash, otp) are matched as WHOLE camel/snake words, never as
// substrings: "Khoang" and "Monkey" contain them as letters and are innocent. The long ones
// (password, token, secret) are matched as substrings, so "TokenHoa" IS refused — a false
// refusal there costs one `json:"-"`, a false pass costs a published credential.
func TestTenVoToiKhongBiBat(t *testing.T) {
	api := `package http

type PhanHoi struct {
	Khoang  string ` + "`json:\"khoang\"`" + `
	Monkey  string ` + "`json:\"monkey\"`" + `
	Tokyo   string ` + "`json:\"tokyo\"`" + `
	Hashtag string ` + "`json:\"hashtag\"`" + `
	TraceID string ` + "`json:\"trace_id\"`" + `
}
`
	got, err := sinhSchema(t, api, "PhanHoi", true)
	if err != nil {
		t.Fatalf("bắt nhầm trường vô tội: %v", err)
	}
	if !strings.Contains(got, `"khoang"`) || !strings.Contains(got, `"trace_id"`) {
		t.Fatalf("thiếu trường hợp lệ:\n%s", got)
	}
}

// json:"-" is the sanctioned way out, and it must actually work.
func TestBiMatCoJsonGachNganThiQua(t *testing.T) {
	api := `package http

type PhanHoi struct {
	Ma      string ` + "`json:\"ma\"`" + `
	MatKhau string ` + "`json:\"-\"`" + `
}
`
	got, err := sinhSchema(t, api, "PhanHoi", true)
	if err != nil {
		t.Fatalf("json:\"-\" phải là lối ra hợp lệ: %v", err)
	}
	if strings.Contains(strings.ToLower(got), "khau") {
		t.Fatalf("trường đã loại trừ vẫn lọt ra:\n%s", got)
	}
}

// A request-only shape may carry the password the sign-in form has to send. It is marked
// writeOnly so no generator ever echoes it back.
func TestBiMatChiDiVaoThiDuocNhungPhaiWriteOnly(t *testing.T) {
	api := `package http

type YeuCau struct {
	Email   string ` + "`json:\"email\"`" + `
	MatKhau string ` + "`json:\"password\"`" + `
}
`
	got, err := sinhSchema(t, api, "YeuCau", false)
	if err != nil {
		t.Fatalf("hình dạng chỉ đi vào không được chặn: %v", err)
	}
	if !strings.Contains(got, `"password":{"type":"string","writeOnly":true}`) {
		t.Fatalf("thiếu writeOnly:\n%s", got)
	}
}

func TestTruongNhungKhongDoan(t *testing.T) {
	// encoding/json flattens an embedded struct. Re-implementing the promotion rules subtly
	// wrong would publish a shape the server does not send, so it refuses instead.
	api := `package http

type Con struct {
	Ten string ` + "`json:\"ten\"`" + `
}

type Cha struct {
	Con
	Ma string ` + "`json:\"ma\"`" + `
}
`
	_, err := sinhSchema(t, api, "Cha", true)
	if err == nil || !strings.Contains(err.Error(), "nhúng") {
		t.Fatalf("mong từ chối trường nhúng, được: %v", err)
	}
}

func TestKieuNgoaiModuleThiTuChoi(t *testing.T) {
	api := `package http

import "net/url"

type Cha struct {
	U url.Values ` + "`json:\"u\"`" + `
}
`
	_, err := sinhSchema(t, api, "Cha", true)
	if err == nil || !strings.Contains(err.Error(), "ngoài module") {
		t.Fatalf("mong từ chối kiểu ngoài module, được: %v", err)
	}
}
