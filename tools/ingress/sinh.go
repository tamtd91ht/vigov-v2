package main

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// moiTruong is the host shape of one environment. Owner decision, 2026-09-26:
//
//	<xa>.vigov.vn               prod web (web-admin; it proxies /api/v1/* itself — ADR 0043)
//	<xa>.stg.vigov.vn           staging web
//	<service>.api.vigov.vn      prod service API
//	<service>.api-stg.vigov.vn  staging service API
//
// A wildcard in an Ingress host, a TLS SAN and a DNS record matches EXACTLY ONE label. That is
// what keeps the four families apart: `*.vigov.vn` never matches `x.stg.vigov.vn` nor
// `identity.api.vigov.vn`, so a staging commune never lands on a prod rule and a service API
// host never reaches web-admin. `TestKhongMoiTruongNaoKhopHostCuaMoiTruongKhac` pins that.
type moiTruong struct {
	Ten     string // overlay directory name: deploy/overlays/<Ten>/
	HostWeb string // wildcard host of the commune web, one label per commune
	DuoiAPI string // suffix of the service API hosts: <service>.<DuoiAPI>
	TLSWeb  string // TLS Secret covering HostWeb — created outside this repository
	TLSAPI  string // TLS Secret covering *.<DuoiAPI> — created outside this repository
}

// cacMoiTruong — the first entry is also what base carries (see sinhYAML). Every overlay
// still patches every host, prod to the same value, so no environment depends on base being
// right. The TLS Secret names of the web families are the ones already in use before
// 2026-09-26; the two API ones are new names this generator introduced with the API hosts.
var cacMoiTruong = []moiTruong{
	{Ten: "prod", HostWeb: "*.vigov.vn", DuoiAPI: "api.vigov.vn", TLSWeb: "vigov-wildcard-tls", TLSAPI: "vigov-api-wildcard-tls"},
	{Ten: "staging", HostWeb: "*.stg.vigov.vn", DuoiAPI: "api-stg.vigov.vn", TLSWeb: "vigov-staging-tls", TLSAPI: "vigov-api-staging-tls"},
}

// hostMacDinh is the web host written into base. It is a wildcard and not an empty host on
// purpose: an Ingress rule with no host matches EVERY host, which on the isolation path is the
// one thing that must never be the default (rule 1).
var hostMacDinh = cacMoiTruong[0].HostWeb

// nhanDNS — a k8s Service name is already a DNS-1035 label, but the host is built from it by
// concatenation, so it is checked here rather than trusted: a name that is not one label would
// produce a host outside the `*.<DuoiAPI>` wildcard and a TLS error nobody could trace.
var nhanDNS = regexp.MustCompile(`^[a-z]([-a-z0-9]{0,61}[a-z0-9])?$`)

// hostDichVu is one per-service host rule: the service, its REST port name, and the resource
// prefixes it owns (rendered as a comment only — the host rule itself routes `/`).
type hostDichVu struct {
	DichVu string
	Cong   string
	TienTo []string
}

// cacHostDichVu derives the per-service host rules from the SAME resolved rule list the admin
// web proxy table is rendered from. No hand list: a service gets a host the day the contract
// gives it a route, and loses it the day it has none. Sorted by name, so rule indices are
// deterministic — the overlays address rules by index.
func cacHostDichVu(luats []luatIngress) ([]hostDichVu, error) {
	theoTen := map[string]*hostDichVu{}
	for _, l := range luats {
		if l.BatHet {
			continue
		}
		if !nhanDNS.MatchString(l.DichVu) {
			return nil, fmt.Errorf("dịch vụ %q không phải một nhãn DNS — không dựng được host %s.%s", l.DichVu, l.DichVu, cacMoiTruong[0].DuoiAPI)
		}
		if l.DichVu == dichVuWeb {
			// web-admin answers the commune hosts; a contract path owned by it would give it
			// a service API host as well, and the edge would then see two host families for
			// one pod. Nothing in the contract does that today — stop rather than guess.
			return nil, fmt.Errorf("tuyến %s thuộc %s — web quản trị không có host API riêng", l.Duong, dichVuWeb)
		}
		h, co := theoTen[l.DichVu]
		if !co {
			h = &hostDichVu{DichVu: l.DichVu, Cong: l.Cong}
			theoTen[l.DichVu] = h
		}
		if h.Cong != l.Cong {
			return nil, fmt.Errorf("dịch vụ %s có hai tên cổng (%s và %s)", l.DichVu, h.Cong, l.Cong)
		}
		h.TienTo = append(h.TienTo, l.Duong)
	}
	if len(theoTen) == 0 {
		return nil, fmt.Errorf("không có dịch vụ nào sở hữu tuyến REST — không sinh Ingress không có host API")
	}
	ra := make([]hostDichVu, 0, len(theoTen))
	for _, h := range theoTen {
		sort.Strings(h.TienTo)
		ra = append(ra, *h)
	}
	sort.Slice(ra, func(i, j int) bool { return ra[i].DichVu < ra[j].DichVu })
	return ra, nil
}

// luatWeb returns the "/" catch-all — the only rule of the commune web host.
func luatWeb(luats []luatIngress) (luatIngress, error) {
	for _, l := range luats {
		if l.BatHet {
			return l, nil
		}
	}
	return luatIngress{}, fmt.Errorf("bảng định tuyến không có luật bắt hết `/` cho %s", dichVuWeb)
}

// sinhYAML renders the generated Ingress. Every line is derived; nothing here is a value a
// person is expected to edit.
func sinhYAML(tuyens []tuyenHopDong, luats []luatIngress) ([]byte, error) {
	web, err := luatWeb(luats)
	if err != nil {
		return nil, err
	}
	hosts, err := cacHostDichVu(luats)
	if err != nil {
		return nil, err
	}
	var b strings.Builder

	demTheoDichVu := map[string]int{}
	for _, t := range tuyens {
		demTheoDichVu[t.DichVu]++
	}
	tenDichVu := make([]string, 0, len(demTheoDichVu))
	for d := range demTheoDichVu {
		tenDichVu = append(tenDichVu, d)
	}
	// Most routes first: the list reads as "who carries the weight of this surface".
	sort.Slice(tenDichVu, func(i, j int) bool {
		if demTheoDichVu[tenDichVu[i]] != demTheoDichVu[tenDichVu[j]] {
			return demTheoDichVu[tenDichVu[i]] > demTheoDichVu[tenDichVu[j]]
		}
		return tenDichVu[i] < tenDichVu[j]
	})
	var phanBo []string
	for _, d := range tenDichVu {
		phanBo = append(phanBo, fmt.Sprintf("%s %d", d, demTheoDichVu[d]))
	}
	duoi := cacMoiTruong[0].DuoiAPI

	b.WriteString("# TỆP NÀY ĐƯỢC SINH RA bởi tools/ingress. KHÔNG SỬA TAY — sửa tay là mất, im lặng, ở\n")
	b.WriteString("# lần sinh sau (luật 9, bất biến 8). Sinh lại: `go run ./tools/ingress` (hoặc `make kb`).\n")
	b.WriteString("#\n")
	b.WriteString("# NGUỒN CHUẨN: kb/20-contracts/openapi.json — chính nó sinh từ khai báo route trong\n")
	b.WriteString("# */internal/ (ADR 0014). Dịch vụ nào có tuyến REST thì có một host API; thêm tuyến KHÔNG\n")
	b.WriteString("# phải sửa tệp này: khai route trong Go, chạy `make kb`. Quên sinh lại thì\n")
	b.WriteString("# `go test ./tools/ingress` ĐỎ.\n")
	b.WriteString(fmt.Sprintf("#\n#     %d tuyến của %d dịch vụ — %s\n#\n", len(tuyens), len(tenDichVu), strings.Join(phanBo, " · ")))
	b.WriteString("# MÔ HÌNH TÊN MIỀN — chủ dự án chốt 26/09/2026:\n")
	b.WriteString("#   <xã>.vigov.vn          → web-admin, chỉ một luật `/`. Web tự chuyển tiếp /api/v1/* tới\n")
	b.WriteString("#                            dịch vụ chủ theo web-admin/src/lib/api/dinh-tuyen.gen.ts (ADR 0043)\n")
	b.WriteString("#   <dịch vụ>.api.vigov.vn → dịch vụ ấy, một luật `/`\n")
	b.WriteString("# Bảng tiền tố → dịch vụ vì thế KHÔNG còn là các luật `paths` ở đây; nó chỉ còn là chú\n")
	b.WriteString("# giải trên từng host dịch vụ, và là bảng TS kia — cùng một lần phân giải hợp đồng.\n")
	b.WriteString("#\n")
	b.WriteString("# HOST và TLS của từng môi trường do overlay vá vào (deploy/overlays/<mt>/ingress-moi-truong.yaml,\n")
	b.WriteString("# CŨNG SINH RA bởi bộ sinh này). Base mang host của prod.\n")
	b.WriteString(fmt.Sprintf("#\n# 1 host web + %d host dịch vụ.\n", len(hosts)))
	b.WriteString("apiVersion: networking.k8s.io/v1\n")
	b.WriteString("kind: Ingress\n")
	b.WriteString("metadata:\n")
	b.WriteString("  name: vigov\n")
	b.WriteString("  annotations:\n")
	b.WriteString("    # Bắt buộc HTTPS. Số điện thoại và nội dung phản ánh của công dân đi qua đây\n")
	b.WriteString("    # (Nghị định 13/2023/NĐ-CP).\n")
	b.WriteString("    nginx.ingress.kubernetes.io/ssl-redirect: \"true\"\n")
	b.WriteString("    nginx.ingress.kubernetes.io/proxy-body-size: \"25m\"\n")
	b.WriteString("spec:\n")
	b.WriteString("  ingressClassName: nginx\n")
	b.WriteString("  # `tls:` do overlay THÊM VÀO — tên secret khác nhau từng môi trường.\n")
	b.WriteString("  #\n")
	b.WriteString("  # THỨ TỰ `rules` CÓ Ý NGHĨA: overlay vá host theo CHỈ SỐ (`/spec/rules/<i>/host`). Chỉ số 0 là\n")
	b.WriteString("  # host web; 1..n là host dịch vụ, xếp theo tên. Bản vá overlay `test` host cũ trước khi\n")
	b.WriteString("  # `replace`, nên overlay lệch chỉ số là `kustomize build` ĐỔ chứ không vá nhầm host.\n")
	b.WriteString("  rules:\n")
	b.WriteString("    # HOST WEB — MỘT quy tắc ký tự đại diện cho MỌI xã. `Host` thật đi nguyên vẹn tới pod\n")
	b.WriteString("    # (ingress-nginx giữ nguyên Host mặc định) và `httpx.TenantMiddleware` phân giải nó thành\n")
	b.WriteString("    # xã ở rìa ngoài cùng. Host không khớp xã nào ⇒ 404, không bao giờ một xã mặc định\n")
	b.WriteString("    # (luật 1, bất biến 3). Thêm một xã = thêm DNS + một hàng trong sổ xã của `platform`.\n")
	b.WriteString("    #\n")
	b.WriteString("    # Nhãn đầu `admin` · `admin-stg` · `api` · `api-stg` · `stg` · `www` khớp ký tự đại diện này\n")
	b.WriteString("    # nhưng là nhãn DÀNH RIÊNG: `platform` từ chối gán chúng cho một xã, nên chúng nhận 404.\n")
	b.WriteString(fmt.Sprintf("    - host: %q\n", hostMacDinh))
	b.WriteString("      http:\n")
	b.WriteString("        paths:\n")
	b.WriteString("          # BẮT HẾT — web quản trị (Next.js). KHÔNG sinh từ hợp đồng. `/api/v1/*` cũng tới đây và\n")
	b.WriteString("          # web chuyển tiếp; tiền tố lạ nhận 404 JSON, không tới dịch vụ nào (ADR 0043).\n")
	viet1Path(&b, web.DichVu, web.Cong)
	for _, h := range hosts {
		b.WriteString("    #\n")
		for _, dong := range ghiChuPhu(fmt.Sprintf("%s — %d tài nguyên:", h.DichVu, len(h.TienTo)), h.TienTo) {
			b.WriteString("    # " + dong + "\n")
		}
		b.WriteString(fmt.Sprintf("    - host: %q\n", h.DichVu+"."+duoi))
		b.WriteString("      http:\n")
		b.WriteString("        paths:\n")
		viet1Path(&b, h.DichVu, h.Cong)
	}
	return []byte(b.String()), nil
}

func viet1Path(b *strings.Builder, dichVu, cong string) {
	b.WriteString("          - path: /\n")
	b.WriteString("            pathType: Prefix\n")
	b.WriteString("            backend:\n")
	b.WriteString("              service:\n")
	b.WriteString("                name: " + dichVu + "\n")
	b.WriteString("                port: { name: " + cong + " }\n")
}

// duongOverlay is where the environment patch of one overlay lives.
func duongOverlay(mt string) string {
	return "deploy/overlays/" + mt + "/ingress-moi-truong.yaml"
}

// sinhOverlay renders the JSON6902 patch of one environment: every host of base, by index,
// and the TLS block. Generated for the same reason the Ingress is: the service hosts follow
// the contract, so a hand-written patch would be a hand list of services that goes stale —
// and a stale index-based patch rewrites the WRONG host, which on staging means a staging pod
// answering under a prod name.
//
// Each `replace` is preceded by a `test` on the base value. A base regenerated with a new
// service shifts the indices; with the `test` the overlay then fails at build time instead of
// patching a neighbour's host.
func sinhOverlay(mt moiTruong, luats []luatIngress) ([]byte, error) {
	hosts, err := cacHostDichVu(luats)
	if err != nil {
		return nil, err
	}
	goc := cacMoiTruong[0]
	var b strings.Builder
	b.WriteString("# TỆP NÀY ĐƯỢC SINH RA bởi tools/ingress (`go run ./tools/ingress`, hoặc `make kb`). KHÔNG SỬA\n")
	b.WriteString("# TAY. Phần MÔI TRƯỜNG `" + mt.Ten + "` của Ingress — host và TLS, và CHỈ hai thứ ấy. Bảng định\n")
	b.WriteString("# tuyến ở `deploy/base/mang/ingress.yaml`, giống nhau ở mọi môi trường.\n")
	b.WriteString("#\n")
	b.WriteString("# VÌ SAO LÀ BẢN VÁ JSON6902 CHỨ KHÔNG PHẢI MỘT `Ingress` ĐẦY ĐỦ: `spec.rules` là danh sách KHÔNG\n")
	b.WriteString("# có khoá trộn, nên một strategic-merge patch chỉ cần NHẮC TỚI `rules` là THAY CẢ DANH SÁCH —\n")
	b.WriteString("# và kustomize không kêu một tiếng. JSON6902 chạm đúng từng trường `host`.\n")
	b.WriteString("#\n")
	b.WriteString("# Mỗi `replace` đi sau một `test` trên giá trị của base: base sinh lại với một dịch vụ mới làm\n")
	b.WriteString("# lệch chỉ số thì `kustomize build` ĐỔ, thay vì vá nhầm host của dịch vụ bên cạnh.\n")
	b.WriteString("#\n")
	b.WriteString(fmt.Sprintf("# Host web `%s` và host API `*.%s` — ký tự đại diện khớp ĐÚNG MỘT nhãn, nên\n", mt.HostWeb, mt.DuoiAPI))
	b.WriteString("# không host nào của môi trường này rơi vào quy tắc của môi trường khác.\n")
	vaHost := func(i int, cu, moi string) {
		b.WriteString(fmt.Sprintf("- op: test\n  path: /spec/rules/%d/host\n  value: %q\n", i, cu))
		b.WriteString(fmt.Sprintf("- op: replace\n  path: /spec/rules/%d/host\n  value: %q\n", i, moi))
	}
	vaHost(0, goc.HostWeb, mt.HostWeb)
	for i, h := range hosts {
		vaHost(i+1, h.DichVu+"."+goc.DuoiAPI, h.DichVu+"."+mt.DuoiAPI)
	}
	b.WriteString("\n")
	b.WriteString("# `add` chứ không `replace`: base cố ý KHÔNG khai `tls`, và `replace` đổ khi đường dẫn chưa có.\n")
	b.WriteString("# Hai Secret TLS tạo NGOÀI kho này — KHÔNG BAO GIỜ commit khoá. Tên: deploy/cau-hinh/README.md.\n")
	b.WriteString("- op: add\n")
	b.WriteString("  path: /spec/tls\n")
	b.WriteString("  value:\n")
	b.WriteString(fmt.Sprintf("    - hosts: [%q]\n", mt.HostWeb))
	b.WriteString("      secretName: " + mt.TLSWeb + "\n")
	b.WriteString(fmt.Sprintf("    - hosts: [%q]\n", "*."+mt.DuoiAPI))
	b.WriteString("      secretName: " + mt.TLSAPI + "\n")
	return []byte(b.String()), nil
}

// ghiChuPhu wraps a heading and a list of items into comment lines of bounded width. The
// items are written out so a reviewer can check the grouping against the contract without
// running anything.
func ghiChuPhu(dau string, muc []string) []string {
	var dongs []string
	hienTai := dau
	for _, p := range muc {
		them := " " + p
		if hienTai != dau {
			them = " · " + p
		}
		if len(hienTai)+len(them) > 96 && hienTai != dau {
			dongs = append(dongs, hienTai)
			hienTai = "  " + p
			continue
		}
		hienTai += them
	}
	return append(dongs, hienTai)
}
