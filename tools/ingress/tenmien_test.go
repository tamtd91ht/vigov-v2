package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// Checks on the generated Ingress and its overlay patches under the owner's domain model of
// 2026-09-26. Like the rest of this package they read the FILES ON DISK.

// TestTepSinhRaKhopVoiHopDong is the staleness gate. It fails the moment a checked-in
// generated file — ingress.yaml, an overlay patch, or the TS table — differs from what the
// generator produces today: someone edited it, or a route was added in Go, `make kb` refreshed
// openapi.json, and nobody re-ran this.
func TestTepSinhRaKhopVoiHopDong(t *testing.T) {
	root := goc(t)
	tep, err := sinhTuKho(root)
	if err != nil {
		t.Fatalf("bộ sinh DỪNG: %v", err)
	}
	muonTep := []string{duongTepSinh, duongTepTS}
	for _, mt := range cacMoiTruong {
		muonTep = append(muonTep, duongOverlay(mt.Ten))
	}
	if len(tep) != len(muonTep) {
		t.Fatalf("bộ sinh trả %d tệp, mong %d", len(tep), len(muonTep))
	}
	for _, duong := range muonTep {
		muon, co := tep[duong]
		if !co {
			t.Errorf("bộ sinh không sinh %s", duong)
			continue
		}
		coTren, err := os.ReadFile(filepath.Join(root, duong))
		if err != nil {
			t.Errorf("đọc %s: %v", duong, err)
			continue
		}
		if string(coTren) != string(muon) {
			t.Errorf("%s KHÔNG khớp %s.\n"+
				"Sinh lại: go run ./tools/ingress   (hoặc `make kb`)\n"+
				"Tệp trên đĩa %d byte, bản sinh ra %d byte.\n"+
				"Nếu bạn vừa sửa tay tệp ấy: đừng — nó là tầng SINH RA (luật 9, bất biến 8).",
				duong, duongHopDong, len(coTren), len(muon))
		}
	}
}

// Literal copies of the owner's decision of 2026-09-26, deliberately NOT read from
// cacMoiTruong: a test that took its expectation from the generator's own table would pass
// whatever that table said.
const (
	webProd    = "*.vigov.vn"
	apiProd    = "api.vigov.vn"
	webStaging = "*.stg.vigov.vn"
	apiStaging = "api-stg.vigov.vn"
)

var moHinhChot = map[string][2]string{"prod": {webProd, apiProd}, "staging": {webStaging, apiStaging}}

func TestMoHinhTenMienDungQuyetDinhChuDuAn(t *testing.T) {
	if len(cacMoiTruong) != len(moHinhChot) {
		t.Fatalf("có %d môi trường, quyết định 26/09/2026 có %d", len(cacMoiTruong), len(moHinhChot))
	}
	if cacMoiTruong[0].Ten != "prod" {
		t.Errorf("môi trường đầu (thứ base mang) là %q, mong prod", cacMoiTruong[0].Ten)
	}
	tlsDaDung := map[string]string{}
	for _, mt := range cacMoiTruong {
		m, co := moHinhChot[mt.Ten]
		if !co {
			t.Errorf("môi trường lạ %q", mt.Ten)
			continue
		}
		if mt.HostWeb != m[0] || mt.DuoiAPI != m[1] {
			t.Errorf("%s: web %q · api %q — quyết định là %q · %q", mt.Ten, mt.HostWeb, mt.DuoiAPI, m[0], m[1])
		}
		for _, s := range []string{mt.TLSWeb, mt.TLSAPI} {
			if s == "" {
				t.Errorf("%s: tên Secret TLS rỗng", mt.Ten)
			}
			if cu, co := tlsDaDung[s]; co {
				t.Errorf("Secret TLS %q dùng hai lần (%s và %s)", s, cu, mt.Ten)
			}
			tlsDaDung[s] = mt.Ten
		}
	}
}

// TestHostWebChiTroVeWeb — rule 0 is the commune web wildcard and carries ONLY the "/"
// catch-all to web-admin. A path rule reappearing here would put a second routing answer for
// /api/v1/* beside the web proxy's (ADR 0043), and the two would drift.
func TestHostWebChiTroVeWeb(t *testing.T) {
	rules := docTepSinh(t).Spec.Rules
	if len(rules) < 2 {
		t.Fatalf("base khai %d quy tắc host — mong 1 host web + ít nhất 1 host dịch vụ", len(rules))
	}
	if rules[0].Host != webProd {
		t.Fatalf("quy tắc 0 là %q, mong host web %q — overlay vá chỉ số 0 là host web", rules[0].Host, webProd)
	}
	p := rules[0].HTTP.Paths
	if len(p) != 1 || p[0].Path != "/" || p[0].PathType != "Prefix" {
		t.Fatalf("host web phải có đúng một luật `/` Prefix, có %d luật", len(p))
	}
	if p[0].Backend.Service.Name != dichVuWeb || p[0].Backend.Service.Port.Name != congWeb {
		t.Errorf("host web trỏ %s:%s, mong %s:%s", p[0].Backend.Service.Name, p[0].Backend.Service.Port.Name, dichVuWeb, congWeb)
	}
	for _, r := range rules[1:] {
		if strings.Contains(r.Host, "*") {
			t.Errorf("quy tắc %q: chỉ quy tắc 0 được là ký tự đại diện", r.Host)
		}
	}
}

// TestMoiTuyenHopDongCoDungMotLuatIngress — every path of the REST contract is served by
// exactly one host rule `<owner>.api.vigov.vn`, whose single "/" rule points at that owner on
// its REST port. The service list is written nowhere by hand; this proves the generated one
// covers the contract.
func TestMoiTuyenHopDongCoDungMotLuatIngress(t *testing.T) {
	rules := docTepSinh(t).Spec.Rules
	for _, tuyen := range docTuyenHopDong(t) {
		var khop []int
		for i, r := range rules {
			if r.Host == tuyen.DichVu+"."+apiProd {
				khop = append(khop, i)
			}
		}
		if len(khop) != 1 {
			t.Errorf("tuyến %s (%s): %d quy tắc host %s.%s, mong đúng 1", tuyen.Duong, tuyen.DichVu, len(khop), tuyen.DichVu, apiProd)
			continue
		}
		r := rules[khop[0]]
		p := r.HTTP.Paths
		if len(p) != 1 || p[0].Path != "/" || p[0].PathType != "Prefix" {
			t.Errorf("host %s phải có đúng một luật `/` Prefix", r.Host)
			continue
		}
		if p[0].Backend.Service.Name != tuyen.DichVu {
			t.Errorf("host %s trỏ tới %s — yêu cầu tới một dịch vụ KHÔNG sở hữu %s", r.Host, p[0].Backend.Service.Name, tuyen.Duong)
		}
		if p[0].Backend.Service.Port.Name != congREST {
			t.Errorf("host %s trỏ cổng %q, mong %q (không bao giờ grpc)", r.Host, p[0].Backend.Service.Port.Name, congREST)
		}
	}
}

// TestKhongCoLuatIngressThua catches the other direction: a service host no contract path
// justifies — a stale host still pointing at a service that no longer serves REST, or a host
// whose name and backend disagree.
func TestKhongCoLuatIngressThua(t *testing.T) {
	coTuyen := map[string]bool{}
	for _, tuyen := range docTuyenHopDong(t) {
		coTuyen[tuyen.DichVu] = true
	}
	for _, r := range docTepSinh(t).Spec.Rules[1:] {
		ten, duoi, _ := strings.Cut(r.Host, ".")
		if duoi != apiProd {
			t.Errorf("host %q không có dạng <dịch vụ>.%s", r.Host, apiProd)
			continue
		}
		if len(r.HTTP.Paths) != 1 || r.HTTP.Paths[0].Backend.Service.Name != ten {
			t.Errorf("host %q không trỏ đúng một luật tới Service %q", r.Host, ten)
		}
		if !coTuyen[ten] {
			t.Errorf("host %q: %s không sở hữu tuyến nào trong %s — host thừa, sinh lại bằng `go run ./tools/ingress`",
				r.Host, ten, duongHopDong)
		}
	}
}

// thaoTacVa is one JSON6902 operation, read independently of the renderer.
type thaoTacVa struct {
	Op    string `yaml:"op"`
	Path  string `yaml:"path"`
	Value any    `yaml:"value"`
}

func docOverlay(t *testing.T, mt string) []thaoTacVa {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(goc(t), "deploy", "overlays", mt, "ingress-moi-truong.yaml"))
	if err != nil {
		t.Fatalf("đọc bản vá %s: %v", mt, err)
	}
	var ops []thaoTacVa
	if err := yaml.Unmarshal(raw, &ops); err != nil {
		t.Fatalf("bản vá %s không phải danh sách thao tác JSON6902: %v", mt, err)
	}
	return ops
}

// TestOverlayVaDungChiSoLuatDuyNhat guards the JSON pointers the overlays use.
//
// Both overlays patch `/spec/rules/<i>/host` by index. That is only safe while index i of base
// is the host the overlay thinks it is, so: every base rule gets exactly one `test` of its base
// host immediately followed by one `replace` to the environment's host; the TLS block covers
// both wildcards; and no operation touches `paths` — a patch that rewrites the path list puts
// the routing table back in an unchecked file.
func TestOverlayVaDungChiSoLuatDuyNhat(t *testing.T) {
	rules := docTepSinh(t).Spec.Rules
	for mt, m := range moHinhChot {
		ops := docOverlay(t, mt)
		daVa := map[string]bool{}
		var tls []any
		for k, op := range ops {
			if strings.Contains(op.Path, "paths") {
				t.Errorf("overlay %s vá %q — bảng định tuyến chỉ được SINH ra ở %s", mt, op.Path, duongTepSinh)
			}
			giaTri, _ := yaml.Marshal(op.Value)
			if strings.Contains(string(giaTri), "pathType") {
				t.Errorf("overlay %s đưa luật định tuyến vào giá trị bản vá", mt)
			}
			switch {
			case op.Op == "add" && op.Path == "/spec/tls":
				tls, _ = op.Value.([]any)
			case op.Op == "replace" && strings.HasPrefix(op.Path, "/spec/rules/") && strings.HasSuffix(op.Path, "/host"):
				if daVa[op.Path] {
					t.Errorf("overlay %s vá %s hai lần", mt, op.Path)
				}
				daVa[op.Path] = true
				if k == 0 || ops[k-1].Op != "test" || ops[k-1].Path != op.Path {
					t.Errorf("overlay %s: replace %s không đi ngay sau một test cùng đường dẫn", mt, op.Path)
				}
			case op.Op == "test":
			default:
				t.Errorf("overlay %s có thao tác lạ %s %s", mt, op.Op, op.Path)
			}
		}
		for i, r := range rules {
			duong := "/spec/rules/" + strconv.Itoa(i) + "/host"
			muonHost := m[0]
			if i > 0 {
				ten, _, _ := strings.Cut(r.Host, ".")
				muonHost = ten + "." + m[1]
			}
			var test, thay string
			for _, op := range ops {
				if op.Path != duong {
					continue
				}
				s, _ := op.Value.(string)
				switch op.Op {
				case "test":
					test = s
				case "replace":
					thay = s
				}
			}
			if test != r.Host {
				t.Errorf("overlay %s: test %s = %q, base có %q", mt, duong, test, r.Host)
			}
			if thay != muonHost {
				t.Errorf("overlay %s: %s → %q, mong %q", mt, duong, thay, muonHost)
			}
		}
		if len(daVa) != len(rules) {
			t.Errorf("overlay %s vá %d host, base có %d", mt, len(daVa), len(rules))
		}
		var tlsHosts []string
		for _, mu := range tls {
			mm, _ := mu.(map[string]any)
			hs, _ := mm["hosts"].([]any)
			for _, h := range hs {
				s, _ := h.(string)
				tlsHosts = append(tlsHosts, s)
			}
			if sn, _ := mm["secretName"].(string); sn == "" {
				t.Errorf("overlay %s: một mục tls không có secretName", mt)
			}
		}
		if strings.Join(tlsHosts, ",") != m[0]+",*."+m[1] {
			t.Errorf("overlay %s: tls phủ %v, mong [%s *.%s]", mt, tlsHosts, m[0], m[1])
		}
	}
}

// khopHost is how an Ingress host, a TLS SAN and a DNS wildcard match: `*` stands for EXACTLY
// ONE label.
func khopHost(mau, host string) bool {
	if !strings.HasPrefix(mau, "*.") {
		return mau == host
	}
	nhan, con, co := strings.Cut(host, ".")
	return co && nhan != "" && con == mau[2:]
}

func TestKhopHostMotNhan(t *testing.T) {
	for _, c := range []struct {
		mau, host string
		muon      bool
	}{
		{"*.vigov.vn", "thangbinh-danang.vigov.vn", true},
		{"*.vigov.vn", "thangbinh-danang.stg.vigov.vn", false},
		{"*.vigov.vn", "identity.api.vigov.vn", false},
		{"*.vigov.vn", "vigov.vn", false},
		{"*.stg.vigov.vn", "thangbinh-danang.stg.vigov.vn", true},
		{"identity.api.vigov.vn", "identity.api.vigov.vn", true},
	} {
		if got := khopHost(c.mau, c.host); got != c.muon {
			t.Errorf("khopHost(%q, %q) = %v, mong %v", c.mau, c.host, got, c.muon)
		}
	}
}

// TestKhongMoiTruongNaoKhopHostCuaMoiTruongKhac — the isolation property the domain model rests
// on: no rule of one environment matches a host of the other, and no web wildcard swallows a
// service API host. Checked on the overlay files as written, with a real commune host of each
// environment as a probe.
func TestKhongMoiTruongNaoKhopHostCuaMoiTruongKhac(t *testing.T) {
	hostCua := map[string][]string{}
	for mt := range moHinhChot {
		for _, op := range docOverlay(t, mt) {
			if op.Op == "replace" {
				s, _ := op.Value.(string)
				hostCua[mt] = append(hostCua[mt], s)
			}
		}
		if len(hostCua[mt]) < 2 {
			t.Fatalf("overlay %s vá %d host", mt, len(hostCua[mt]))
		}
	}
	xa := map[string]string{"prod": "thangbinh-danang.vigov.vn", "staging": "thangbinh-danang.stg.vigov.vn"}
	for mt, hs := range hostCua {
		for khac, hk := range hostCua {
			if khac == mt {
				continue
			}
			for _, mau := range hs {
				for _, h := range append([]string{xa[khac]}, hk...) {
					if khopHost(mau, strings.Replace(h, "*", "x", 1)) {
						t.Errorf("quy tắc %q của %s khớp host %q của %s", mau, mt, h, khac)
					}
				}
			}
		}
		web := hs[0]
		if !khopHost(web, xa[mt]) {
			t.Errorf("host web %q của %s không khớp host xã %q", web, mt, xa[mt])
		}
		for _, h := range hs[1:] {
			if khopHost(web, h) {
				t.Errorf("host web %q nuốt host dịch vụ %q", web, h)
			}
		}
	}
}
