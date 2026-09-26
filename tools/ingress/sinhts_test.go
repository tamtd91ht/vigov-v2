package main

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// These checks guard web-admin/src/lib/api/dinh-tuyen.gen.ts, the table the admin web proxies
// /api/v1/* by. In production that proxy — not the Ingress — decides which service answers an
// admin request, so a stale table is a request answered by a service that does not own it.
//
// Like doichieu_test.go they read the FILE ON DISK: the failure guarded against is a file that
// stopped matching (edited by hand, or never regenerated after the contract moved).

// TestTepTSKhopVoiHopDong is the staleness gate for the TS table, the twin of
// TestTepSinhRaKhopVoiHopDong.
func TestTepTSKhopVoiHopDong(t *testing.T) {
	root := goc(t)
	tep, err := sinhTuKho(root)
	if err != nil {
		t.Fatalf("bộ sinh DỪNG: %v", err)
	}
	muon := tep[duongTepTS]
	co, err := os.ReadFile(filepath.Join(root, duongTepTS))
	if err != nil {
		t.Fatalf("đọc %s: %v", duongTepTS, err)
	}
	if string(co) != string(muon) {
		t.Fatalf("%s KHÔNG khớp %s.\n"+
			"Sinh lại: go run ./tools/ingress   (hoặc `make kb`)\n"+
			"Tệp trên đĩa %d byte, bản sinh ra %d byte.\n"+
			"Nếu bạn vừa sửa tay tệp ấy: đừng — nó là tầng SINH RA (luật 9, bất biến 8).",
			duongTepTS, duongHopDong, len(co), len(muon))
	}
}

var (
	mauDongTS  = regexp.MustCompile(`^  \{ tienTo: "([^"]+)", dichVu: "([^"]+)" \},$`)
	mauKieuTS  = regexp.MustCompile(`^export type DichVuAPI = (.+);$`)
	mauTenKieu = regexp.MustCompile(`^"([^"]+)"$`)
)

// docTepTS is an independent reader of the TS file: a regex over lines, sharing nothing with
// the renderer. Every line that mentions `tienTo:` must match the entry shape — otherwise a
// reformatted line would silently drop out of the comparison and the check would pass on less.
func docTepTS(t *testing.T) (cacCap [][2]string, kieu []string) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(goc(t), duongTepTS))
	if err != nil {
		t.Fatalf("đọc %s: %v", duongTepTS, err)
	}
	s := string(raw)
	if strings.Contains(s, "\r") {
		t.Errorf("%s có CR — tệp sinh ra phải dùng LF", duongTepTS)
	}
	if !strings.HasSuffix(s, "\n") {
		t.Errorf("%s không kết thúc bằng xuống dòng", duongTepTS)
	}
	for _, dong := range strings.Split(s, "\n") {
		if m := mauKieuTS.FindStringSubmatch(dong); m != nil {
			for _, p := range strings.Split(m[1], " | ") {
				mm := mauTenKieu.FindStringSubmatch(p)
				if mm == nil {
					t.Fatalf("DichVuAPI có phần tử lạ %q", p)
				}
				kieu = append(kieu, mm[1])
			}
			continue
		}
		if strings.HasPrefix(dong, "//") || !strings.Contains(dong, "tienTo:") {
			continue
		}
		if strings.Contains(dong, "readonly tienTo") {
			continue // the type annotation of DINH_TUYEN_API
		}
		m := mauDongTS.FindStringSubmatch(dong)
		if m == nil {
			t.Fatalf("dòng %q nhắc tienTo mà không đúng khuôn — bộ đọc sẽ bỏ sót nó", dong)
		}
		cacCap = append(cacCap, [2]string{m[1], m[2]})
	}
	if len(cacCap) == 0 {
		t.Fatalf("%s không có dòng định tuyến nào", duongTepTS)
	}
	return cacCap, kieu
}

// TestBangTSKhopHostDichVuIngress — the TS table and ingress.yaml, both read from disk, name the
// same set of services: every service the web proxy forwards to has its own API host, and no
// API host exists for a service the table never routes to. Two surfaces, one list.
func TestBangTSKhopHostDichVuIngress(t *testing.T) {
	ts, kieu := docTepTS(t)

	dichVuTS := map[string]bool{}
	for _, c := range ts {
		dichVuTS[c[1]] = true
		if c[0] == "/" || strings.HasSuffix(c[0], "/") {
			t.Errorf("tienTo %q phải không có gạch chéo cuối và không là `/`", c[0])
		}
	}
	var host []string
	for _, r := range docTepSinh(t).Spec.Rules[1:] {
		for _, p := range r.HTTP.Paths {
			host = append(host, p.Backend.Service.Name)
		}
	}
	sort.Strings(host)
	tsDS := make([]string, 0, len(dichVuTS))
	for d := range dichVuTS {
		tsDS = append(tsDS, d)
	}
	sort.Strings(tsDS)
	if strings.Join(host, ",") != strings.Join(tsDS, ",") {
		t.Errorf("host dịch vụ trong ingress.yaml = %v, dịch vụ trong %s = %v", host, duongTepTS, tsDS)
	}
	if strings.Join(kieu, ",") != strings.Join(tsDS, ",") {
		t.Errorf("DichVuAPI = %v, mong đúng tập dịch vụ có dòng, đã sắp: %v", kieu, tsDS)
	}
}

// TestMoiTuyenHopDongCoDungMotDongTS is the prefix-level correspondence the Ingress used to
// carry: every path of the REST contract is matched by exactly one TS entry (by path SEGMENT,
// as the proxy matches), and that entry names the owning service. It also checks the FIRST
// match in file order, so the table is right under a first-match reader as well.
func TestMoiTuyenHopDongCoDungMotDongTS(t *testing.T) {
	ts, _ := docTepTS(t)
	for _, tuyen := range docTuyenHopDong(t) {
		var khop []int
		for i, c := range ts {
			if laTienToDoan(c[0], tuyen.Duong) {
				khop = append(khop, i)
			}
		}
		if len(khop) != 1 {
			t.Errorf("tuyến %s (%s) khớp %d dòng của %s, mong đúng 1", tuyen.Duong, tuyen.DichVu, len(khop), duongTepTS)
			continue
		}
		if ts[khop[0]][1] != tuyen.DichVu {
			t.Errorf("tuyến %s thuộc %s nhưng dòng %s trỏ tới %s", tuyen.Duong, tuyen.DichVu, ts[khop[0]][0], ts[khop[0]][1])
		}
	}
	for _, c := range ts {
		duoc := false
		for _, tuyen := range docTuyenHopDong(t) {
			if laTienToDoan(c[0], tuyen.Duong) && tuyen.DichVu == c[1] {
				duoc = true
				break
			}
		}
		if !duoc {
			t.Errorf("dòng %s → %s không tuyến nào trong %s biện minh — dòng thừa", c[0], c[1], duongHopDong)
		}
	}
}

// TestBangTSDaiTruocNgan — the file's ordering promise ("longest-first"), checked.
func TestBangTSDaiTruocNgan(t *testing.T) {
	ts, _ := docTepTS(t)
	for i := 1; i < len(ts); i++ {
		if len(ts[i][0]) > len(ts[i-1][0]) {
			t.Errorf("dòng %q đứng SAU %q — bảng phải viết dài-trước-ngắn", ts[i][0], ts[i-1][0])
		}
	}
}

func TestSinhTSBoLuatBatHetVaTuChoiDauVaoLa(t *testing.T) {
	luats := []luatIngress{
		{Duong: "/api/v1/role-permissions", DichVu: "identity"},
		{Duong: "/api/v1/petitions", DichVu: "petitions"},
		{Duong: "/", DichVu: dichVuWeb, BatHet: true},
	}
	ra, err := sinhTS(luats)
	if err != nil {
		t.Fatalf("sinhTS: %v", err)
	}
	s := string(ra)
	if strings.Contains(s, dichVuWeb) || strings.Contains(s, `tienTo: "/"`) {
		t.Errorf("luật bắt hết lọt vào bảng TS:\n%s", s)
	}
	if !strings.Contains(s, `export type DichVuAPI = "identity" | "petitions";`) {
		t.Errorf("DichVuAPI sai:\n%s", s)
	}
	if strings.Index(s, "role-permissions") > strings.Index(s, "/api/v1/petitions") {
		t.Errorf("thứ tự luật không được giữ nguyên:\n%s", s)
	}

	if _, err := sinhTS([]luatIngress{{Duong: "/", DichVu: dichVuWeb, BatHet: true}}); err == nil {
		t.Error("chỉ có luật bắt hết mà vẫn sinh — bảng rỗng phải DỪNG")
	}
	for _, xau := range []luatIngress{
		{Duong: `/api/v1/a"b`, DichVu: "identity"},
		{Duong: "/api/v1/a", DichVu: `x" | "y`},
		{Duong: "/api/v1/a/", DichVu: "identity"},
		{Duong: "/api/v1/a", DichVu: ""},
	} {
		if _, err := sinhTS([]luatIngress{xau}); err == nil {
			t.Errorf("sinhTS nhận %q → %q — phải DỪNG", xau.Duong, xau.DichVu)
		}
	}
}
