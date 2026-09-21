package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// This file holds the check that pays for the whole tool: the generated Ingress and the REST
// contract must still agree. It reads the FILE ON DISK — not the model the generator just
// built — because the failure being guarded against is a file that stopped matching: a rule
// deleted by hand, or a route added in Go and never regenerated here.
//
// Delete one rule from deploy/base/mang/ingress.yaml and both
// TestTepSinhRaKhopVoiHopDong and TestMoiTuyenHopDongCoDungMotLuatIngress turn red.

// ingressTep is a deliberately independent reader of the generated file. It shares no code
// with the renderer, so a test passing means two separate pieces of code agree about the
// content — not that one piece agrees with itself.
type ingressTep struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	Spec struct {
		IngressClassName string `yaml:"ingressClassName"`
		TLS              []any  `yaml:"tls"`
		Rules            []struct {
			Host string `yaml:"host"`
			HTTP struct {
				Paths []struct {
					Path     string `yaml:"path"`
					PathType string `yaml:"pathType"`
					Backend  struct {
						Service struct {
							Name string `yaml:"name"`
							Port struct {
								Name string `yaml:"name"`
							} `yaml:"port"`
						} `yaml:"service"`
					} `yaml:"backend"`
				} `yaml:"paths"`
			} `yaml:"http"`
		} `yaml:"rules"`
	} `yaml:"spec"`
}

func goc(t *testing.T) string {
	t.Helper()
	root, err := timGoc()
	if err != nil {
		t.Fatalf("không tìm được gốc kho: %v", err)
	}
	return root
}

func docTepSinh(t *testing.T) ingressTep {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(goc(t), duongTepSinh))
	if err != nil {
		t.Fatalf("đọc %s: %v", duongTepSinh, err)
	}
	var ing ingressTep
	if err := yaml.Unmarshal(raw, &ing); err != nil {
		t.Fatalf("%s không phải YAML hợp lệ: %v", duongTepSinh, err)
	}
	if ing.Kind != "Ingress" || ing.Metadata.Name != "vigov" {
		t.Fatalf("%s: mong Ingress tên vigov, có %q/%q", duongTepSinh, ing.Kind, ing.Metadata.Name)
	}
	return ing
}

func docTuyenHopDong(t *testing.T) []tuyenHopDong {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(goc(t), duongHopDong))
	if err != nil {
		t.Fatalf("đọc %s: %v", duongHopDong, err)
	}
	tuyens, err := docHopDong(raw)
	if err != nil {
		t.Fatalf("hợp đồng REST không phân giải được chủ sở hữu: %v", err)
	}
	return tuyens
}

// TestTepSinhRaKhopVoiHopDong is the staleness gate. It fails the moment the checked-in file
// differs from what the generator produces today — whether because someone edited it, or
// because a route was added in Go, `make kb` refreshed openapi.json, and nobody re-ran this.
func TestTepSinhRaKhopVoiHopDong(t *testing.T) {
	root := goc(t)
	muon, err := sinhTuKho(root)
	if err != nil {
		t.Fatalf("bộ sinh DỪNG: %v", err)
	}
	co, err := os.ReadFile(filepath.Join(root, duongTepSinh))
	if err != nil {
		t.Fatalf("đọc %s: %v", duongTepSinh, err)
	}
	if string(co) != string(muon) {
		t.Fatalf("%s KHÔNG khớp %s.\n"+
			"Sinh lại: go run ./tools/ingress   (hoặc `make kb`)\n"+
			"Tệp trên đĩa %d byte, bản sinh ra %d byte.\n"+
			"Nếu bạn vừa sửa tay tệp ấy: đừng — nó là tầng SINH RA (luật 9, bất biến 8).",
			duongTepSinh, duongHopDong, len(co), len(muon))
	}
}

// TestMoiTuyenHopDongCoDungMotLuatIngress is the correspondence the whole exercise exists for:
// every path of the REST contract must be matched by exactly one API rule, and that rule must
// point at the service that owns the path.
//
// It also checks the FIRST match in file order, which is what makes the ordering claim in the
// generated file more than a comment: it holds under a controller that stops at the first
// match as well as under ingress-nginx, which sorts by descending length.
func TestMoiTuyenHopDongCoDungMotLuatIngress(t *testing.T) {
	ing := docTepSinh(t)
	tuyens := docTuyenHopDong(t)

	if len(ing.Spec.Rules) != 1 {
		t.Fatalf("mong ĐÚNG MỘT quy tắc host, có %d", len(ing.Spec.Rules))
	}
	luats := ing.Spec.Rules[0].HTTP.Paths
	if len(luats) == 0 {
		t.Fatal("bảng định tuyến rỗng — mọi tuyến REST sẽ 404")
	}

	for _, tuyen := range tuyens {
		var khop []int
		for i, l := range luats {
			if l.Path == "/" {
				continue // bắt hết — cố ý phủ mọi thứ, xét riêng bên dưới
			}
			if l.PathType != "Prefix" {
				t.Errorf("luật %s có pathType %q — phép đối chiếu này chỉ đúng với Prefix", l.Path, l.PathType)
			}
			if laTienToDoan(l.Path, tuyen.Duong) {
				khop = append(khop, i)
			}
		}
		switch len(khop) {
		case 0:
			t.Errorf("tuyến %s (%s) KHÔNG có luật ingress nào — nó sẽ rơi vào luật bắt hết `/` và nhận 404 của web quản trị",
				tuyen.Duong, tuyen.DichVu)
			continue
		case 1:
		default:
			var ds []string
			for _, i := range khop {
				ds = append(ds, luats[i].Path)
			}
			t.Errorf("tuyến %s khớp %d luật (%s) — thứ tự quyết định kết quả, tức có luật nuốt luật",
				tuyen.Duong, len(khop), strings.Join(ds, ", "))
			continue
		}
		l := luats[khop[0]]
		if l.Backend.Service.Name != tuyen.DichVu {
			t.Errorf("tuyến %s thuộc %s nhưng luật %s trỏ tới %s — yêu cầu tới một dịch vụ KHÔNG sở hữu dữ liệu ấy",
				tuyen.Duong, tuyen.DichVu, l.Path, l.Backend.Service.Name)
		}
		if l.Backend.Service.Port.Name == "" {
			t.Errorf("luật %s không khai cổng theo tên", l.Path)
		}
	}
}

// TestKhongCoLuatIngressThua catches the other direction: a rule that no contract path
// justifies. A stale rule points at a service that no longer serves that resource, and the
// only symptom is a 404 nobody can trace back to a file.
func TestKhongCoLuatIngressThua(t *testing.T) {
	ing := docTepSinh(t)
	tuyens := docTuyenHopDong(t)

	for _, l := range ing.Spec.Rules[0].HTTP.Paths {
		if l.Path == "/" {
			continue
		}
		duoc := false
		for _, tuyen := range tuyens {
			if laTienToDoan(l.Path, tuyen.Duong) && tuyen.DichVu == l.Backend.Service.Name {
				duoc = true
				break
			}
		}
		if !duoc {
			t.Errorf("luật %s → %s không tuyến nào trong %s biện minh — luật thừa, sinh lại bằng `go run ./tools/ingress`",
				l.Path, l.Backend.Service.Name, duongHopDong)
		}
	}
}

// TestLuatBatHetDungCuoiVaTroVeWeb — the catch-all must be last. Written anywhere else it
// swallows every API rule below it on a controller that takes the first match, and the whole
// routing table becomes one line pointing at the Next.js app.
func TestLuatBatHetDungCuoiVaTroVeWeb(t *testing.T) {
	luats := docTepSinh(t).Spec.Rules[0].HTTP.Paths
	cuoi := luats[len(luats)-1]
	if cuoi.Path != "/" {
		t.Fatalf("luật cuối cùng là %q, mong `/` — luật bắt hết phải đứng cuối", cuoi.Path)
	}
	if cuoi.Backend.Service.Name != dichVuWeb {
		t.Fatalf("luật bắt hết trỏ tới %q, mong %q", cuoi.Backend.Service.Name, dichVuWeb)
	}
	for _, l := range luats[:len(luats)-1] {
		if l.Path == "/" {
			t.Fatalf("có luật `/` thứ hai ở giữa bảng")
		}
	}
}

// TestThuTuDaiTruocNgan — the file's ordering promise, checked rather than asserted in a
// comment.
func TestThuTuDaiTruocNgan(t *testing.T) {
	luats := docTepSinh(t).Spec.Rules[0].HTTP.Paths
	for i := 1; i < len(luats); i++ {
		if len(luats[i].Path) > len(luats[i-1].Path) {
			t.Errorf("luật %q (dài %d) đứng SAU %q (dài %d) — bảng phải viết dài-trước-ngắn",
				luats[i].Path, len(luats[i].Path), luats[i-1].Path, len(luats[i-1].Path))
		}
	}
}

// TestOverlayVaDungChiSoLuatDuyNhat guards the JSON pointer the overlays use.
//
// Both overlays patch /spec/rules/0/host. That index is only safe while base declares exactly
// one rule — one wildcard host covering every commune. If a second rule ever appears, the
// patch would silently rewrite the wrong one, and staging would answer on the production host.
// The overlay patches must also never mention `paths`: a patch that rewrites the path list
// puts the routing table back in a hand-edited file, which is the defect being removed.
func TestOverlayVaDungChiSoLuatDuyNhat(t *testing.T) {
	root := goc(t)
	if n := len(docTepSinh(t).Spec.Rules); n != 1 {
		t.Fatalf("base khai %d quy tắc host; bản vá overlay trỏ `/spec/rules/0` nên chỉ đúng khi n == 1", n)
	}
	for _, mt := range []string{"prod", "staging"} {
		duong := filepath.Join(root, "deploy", "overlays", mt, "ingress-moi-truong.yaml")
		raw, err := os.ReadFile(duong)
		if err != nil {
			t.Fatalf("đọc bản vá %s: %v", mt, err)
		}
		var ops []map[string]any
		if err := yaml.Unmarshal(raw, &ops); err != nil {
			t.Errorf("bản vá %s không phải danh sách thao tác JSON6902: %v", mt, err)
			continue
		}
		vaHost := false
		for _, op := range ops {
			con, _ := op["path"].(string)
			if con == "/spec/rules/0/host" {
				vaHost = true
			}
			// Checked on the OPERATIONS, not on the file text: the comments in these files
			// talk about `paths` on purpose, to say why it must not be patched here.
			if strings.Contains(con, "paths") {
				t.Errorf("overlay %s vá %q — bảng định tuyến chỉ được SINH ra ở %s, không vá tay", mt, con, duongTepSinh)
			}
			giaTri, _ := yaml.Marshal(op["value"])
			if strings.Contains(string(giaTri), "pathType") {
				t.Errorf("overlay %s đưa luật định tuyến vào giá trị bản vá — chúng thuộc %s", mt, duongTepSinh)
			}
		}
		if !vaHost {
			t.Errorf("overlay %s không vá `/spec/rules/0/host` — môi trường ấy sẽ dùng host mặc định của base", mt)
		}
	}
}
