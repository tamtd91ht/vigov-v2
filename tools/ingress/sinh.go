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

// The environments are read from deploy/hosts.yaml (hosts.go), never typed here. The first entry
// is also what base carries (see sinhYAML). Every overlay still patches every host, prod to the
// same value, so no environment depends on base being right.
//
// The web host written into base is that first entry's wildcard, never an empty host: an Ingress
// rule with no host matches EVERY host, which on the isolation path is the one thing that must
// never be the default (rule 1). parseHostPlan refuses an empty web_host for that reason.

// nhanDNS — a k8s Service name is already a DNS-1035 label, but the host is built from it by
// concatenation, so it is checked here rather than trusted: a name that is not one label would
// produce a host outside the `*.<DuoiAPI>` wildcard and a TLS error nobody could trace.
var nhanDNS = regexp.MustCompile(`^[a-z]([-a-z0-9]{0,61}[a-z0-9])?$`)

// servicesWithoutAPIHost are the services that get no public `<service>.<api_suffix>` host, each with
// the reason. A per-service host rule is `path: /` — it forwards EVERY path of that pod's REST port.
//
//	platform  its REST port ALSO serves the operator realm (ADR 0048), separated from the commune chain
//	          only by an in-process Host comparison against OPERATOR_HOST. A public `/` rule would put
//	          /api/v1/operator-sessions one spoofed Host (or one OPERATOR_HOST misconfiguration) away from
//	          the internet — exactly what ADR 0048 #6(c) keeps off the generated Ingress. Its only commune
//	          routes (ADR 0069, /api/v1/commune-branding) are staff routes, and staff reach them through
//	          web-admin in-cluster (PLATFORM_HTTP_ADDR); it has no citizen or public route at all.
//
// Adding a citizen/public route to platform is therefore a decision, not a generator run: it needs its
// own path-scoped rule and an ADR, never this entry removed.
var servicesWithoutAPIHost = map[string]string{
	"platform": "REST port shared with the operator realm (ADR 0048 #6(c)); staff-only commune routes (ADR 0069)",
}

// pathScopedPublicRoutes are the ONLY paths of a servicesWithoutAPIHost service that reach the
// internet: each one gets a `pathType: Exact` rule on `<service>.<api_suffix>` — never `/`, never a
// Prefix — so nothing else of that pod's REST port (the operator realm above all) is routable through
// it, and the Host the pod sees is the API host, never OPERATOR_HOST. The value is the decision.
//
// A HAND LIST ON PURPOSE, checked against the contract (checkPathScopedPublicRoutes): the path must
// exist, belong to that service, and be authz.Public on every method. Adding a line exposes a route of
// the operator pod to the internet — an owner decision first, then here.
var pathScopedPublicRoutes = map[string]map[string]string{
	"platform": {
		"/api/v1/mini-app-ids": "owner chose option A, 06/10/2026: the Mini App deploy script reads App IDs from platform's mini_app registry; public read approved by the owner",
	},
}

// checkPathScopedPublicRoutes refuses the run when an allow-listed path is missing from the contract,
// owned by another service, or not public on every method — an Exact rule to a staff route would put
// it on the internet on a host no web session reaches.
func checkPathScopedPublicRoutes(routes []tuyenHopDong) error {
	byPath := map[string]tuyenHopDong{}
	for _, r := range routes {
		byPath[r.Duong] = r
	}
	for svc, paths := range pathScopedPublicRoutes {
		if _, ok := servicesWithoutAPIHost[svc]; !ok {
			return fmt.Errorf("pathScopedPublicRoutes: %s đã có host `/` — tuyến theo đường dẫn chỉ dành cho dịch vụ trong servicesWithoutAPIHost", svc)
		}
		for p := range paths {
			r, ok := byPath[p]
			switch {
			case !ok:
				return fmt.Errorf("pathScopedPublicRoutes: %s không có trong %s — xoá dòng hoặc khai route", p, duongHopDong)
			case r.DichVu != svc:
				return fmt.Errorf("pathScopedPublicRoutes: %s thuộc %s, không thuộc %s", p, r.DichVu, svc)
			case !r.Public:
				return fmt.Errorf("pathScopedPublicRoutes: %s không phải authz.Public trên mọi phương thức — không mở ra internet", p)
			}
		}
	}
	return nil
}

// hostDichVu is one per-service host rule: the service, its REST port name, and the resource
// prefixes it owns (rendered as a comment only — the host rule itself routes `/`).
//
// Exact is set only for a servicesWithoutAPIHost service with pathScopedPublicRoutes: the host then
// routes exactly those paths and nothing else.
type hostDichVu struct {
	DichVu string
	Cong   string
	TienTo []string
	Exact  []string
}

// cacHostDichVu derives the per-service host rules from the SAME resolved rule list the admin
// web proxy table is rendered from. No hand list: a service gets a host the day the contract
// gives it a route, and loses it the day it has none. Sorted by name, so rule indices are
// deterministic — the overlays address rules by index.
//
// A service in servicesWithoutAPIHost gets NO host rule: its commune routes are reached only through
// web-admin's in-cluster gateway (dinh-tuyen.gen.ts), never from the internet.
func cacHostDichVu(luats []luatIngress) ([]hostDichVu, error) {
	theoTen := map[string]*hostDichVu{}
	for _, l := range luats {
		if l.BatHet {
			continue
		}
		_, pathScopedOnly := servicesWithoutAPIHost[l.DichVu]
		var exact []string
		if pathScopedOnly {
			for _, p := range l.Phu {
				if _, ok := pathScopedPublicRoutes[l.DichVu][p]; ok {
					exact = append(exact, p)
				}
			}
			if len(exact) == 0 {
				continue
			}
		}
		if !nhanDNS.MatchString(l.DichVu) {
			return nil, fmt.Errorf("dịch vụ %q không phải một nhãn DNS — không dựng được host %s.<api_suffix>", l.DichVu, l.DichVu)
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
		if pathScopedOnly {
			h.TienTo = append(h.TienTo, exact...)
			h.Exact = append(h.Exact, exact...)
			continue
		}
		h.TienTo = append(h.TienTo, l.Duong)
	}
	if len(theoTen) == 0 {
		return nil, fmt.Errorf("không có dịch vụ nào sở hữu tuyến REST — không sinh Ingress không có host API")
	}
	ra := make([]hostDichVu, 0, len(theoTen))
	for _, h := range theoTen {
		sort.Strings(h.TienTo)
		sort.Strings(h.Exact)
		ra = append(ra, *h)
	}
	sort.Slice(ra, func(i, j int) bool { return ra[i].DichVu < ra[j].DichVu })
	return ra, nil
}

// The public edge is split into TWO Ingress objects because ingress-nginx reads
// `proxy-body-size` (and `proxy-request-buffering`) per Ingress OBJECT, never per host, and the
// owner set two different caps (ADR 0052 §Sửa đổi 09/10/2026): uploads now travel as one multipart
// request through the owning service, so the hosts that carry them take 55 MB (a 50 MB file plus
// the multipart envelope) while every other host keeps 25 MB.
//
// `vigov` keeps the commune web host at index 0 and the name the overlays already patch, so the
// existing patch target is unchanged; the hosts that take no upload move to `vigov-no-upload`.
// Name order is also apply order (kustomize sorts by name within a kind): `vigov` sheds those hosts
// before `vigov-no-upload` claims them, so ingress-nginx's admission webhook never sees one host/path
// in two objects during the rollout.
const (
	ingressUpload   = "vigov"
	ingressNoUpload = "vigov-no-upload"

	bodyCapUpload  = "55m"
	bodyCapDefault = "25m"

	// uploadProxyTimeout is the read/send timeout, in seconds, of the upload object only: the
	// owner-approved 180 s bound of one upload (ADR 0052 §Sửa đổi 09/10/2026). nginx's default is
	// 60 s, so a malware scan or image re-encode that answers after 60 s would reach the client as
	// a 504 while the service goes on to store the file — the client retries and uploads it twice.
	uploadProxyTimeout = "180"
)

// uploadHosts are the services whose PUBLIC host takes the upload cap, each with the reason. The
// owner named four hosts; anything not listed falls into the 25 MB object — the smaller cap is the
// default, never the larger one.
//
// web-admin is here because staff uploads reach the services through its /api/v1/* gateway
// (ADR 0043): the commune web host is where a staff upload enters the edge.
var uploadHosts = map[string]string{
	dichVuWeb:   "staff uploads of every service enter through the web gateway (ADR 0043)",
	"comms":     "cover image, body image, broadcast audio (ADR 0052 §Sửa đổi 09/10/2026)",
	"petitions": "citizen scene photos, after-processing photos, log and task attachments (ADR 0052 §Sửa đổi 09/10/2026)",
	"platform":  "commune logo/banner (ADR 0052 §Sửa đổi 09/10/2026); its public host routes only the Exact paths of pathScopedPublicRoutes",
}

// ingressObject is one generated Ingress: its name, the overlay patch file that sets its hosts per
// environment, its body cap, and its host rules in index order (the overlays patch by index).
type ingressObject struct {
	Name    string
	Patch   string // file name inside deploy/overlays/<env>/
	BodyCap string
	// Stream turns request buffering off: see the comment rendered next to the annotation.
	Stream bool
	// Web is true for the object carrying the commune web host; it is always rule 0.
	Web   bool
	Hosts []hostDichVu
}

// ingressObjects partitions the public hosts into the two objects. Fixed order: `vigov` first.
//
// FAIL CLOSED: an uploadHosts service with no public host is a stale list, not something to skip —
// the owner's cap would silently apply to nothing. An empty `vigov-no-upload` is refused too: an
// Ingress with no rule is not a manifest anyone meant to apply.
func ingressObjects(luats []luatIngress) ([]ingressObject, error) {
	hosts, err := cacHostDichVu(luats)
	if err != nil {
		return nil, err
	}
	up := ingressObject{Name: ingressUpload, Patch: "ingress-moi-truong.yaml", BodyCap: bodyCapUpload, Stream: true, Web: true}
	rest := ingressObject{Name: ingressNoUpload, Patch: "ingress-no-upload-environment.yaml", BodyCap: bodyCapDefault}
	seen := map[string]bool{}
	for _, h := range hosts {
		if _, ok := uploadHosts[h.DichVu]; ok {
			up.Hosts = append(up.Hosts, h)
			seen[h.DichVu] = true
			continue
		}
		rest.Hosts = append(rest.Hosts, h)
	}
	names := make([]string, 0, len(uploadHosts))
	for s := range uploadHosts {
		names = append(names, s)
	}
	sort.Strings(names)
	for _, s := range names {
		if s != dichVuWeb && !seen[s] {
			return nil, fmt.Errorf("uploadHosts: %s không có host công khai nào — trần %s không áp vào đâu; sửa danh sách (ADR 0052 §Sửa đổi 09/10/2026)", s, bodyCapUpload)
		}
	}
	if len(rest.Hosts) == 0 {
		return nil, fmt.Errorf("Ingress %s không còn host nào — không sinh một Ingress rỗng", ingressNoUpload)
	}
	return []ingressObject{up, rest}, nil
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
// base is the first environment of deploy/hosts.yaml.
func sinhYAML(tuyens []tuyenHopDong, luats []luatIngress, base moiTruong) ([]byte, error) {
	web, err := luatWeb(luats)
	if err != nil {
		return nil, err
	}
	objs, err := ingressObjects(luats)
	if err != nil {
		return nil, err
	}
	hostCount := 0
	for _, o := range objs {
		hostCount += len(o.Hosts)
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
	b.WriteString("# HOST và TLS của từng môi trường do overlay vá vào (deploy/overlays/<mt>/ingress-moi-truong.yaml\n")
	b.WriteString("# cho `vigov`, ingress-no-upload-environment.yaml cho `vigov-no-upload` — CŨNG SINH RA bởi bộ\n")
	b.WriteString("# sinh này). Base mang host của prod.\n")
	b.WriteString("#\n")
	b.WriteString("# HAI Ingress vì trần thân yêu cầu (`proxy-body-size`) của ingress-nginx đặt theo ĐỐI TƯỢNG\n")
	b.WriteString("# Ingress, không theo host. Chủ dự án chốt 09/10/2026 (ADR 0052 §Sửa đổi 09/10/2026): tải lên\n")
	b.WriteString(fmt.Sprintf("# đi qua service, nên host web + host có tải lên nhận %s; host khác giữ %s. Mỗi\n", bodyCapUpload, bodyCapDefault))
	b.WriteString("# Ingress có bản vá overlay riêng (danh sách uploadHosts ở tools/ingress/sinh.go).\n")
	b.WriteString(fmt.Sprintf("#\n# 1 host web + %d host dịch vụ.\n", hostCount))
	for _, o := range objs {
		b.WriteString("---\n")
		writeIngress(&b, o, web, base.DuoiAPI, base.HostWeb)
	}
	return []byte(b.String()), nil
}

// writeIngress renders one Ingress object of the generated file.
func writeIngress(b *strings.Builder, o ingressObject, web luatIngress, apiSuffix, webHost string) {
	b.WriteString("apiVersion: networking.k8s.io/v1\n")
	b.WriteString("kind: Ingress\n")
	b.WriteString("metadata:\n")
	b.WriteString("  name: " + o.Name + "\n")
	b.WriteString("  annotations:\n")
	b.WriteString("    # Bắt buộc HTTPS. Số điện thoại và nội dung phản ánh của công dân đi qua đây\n")
	b.WriteString("    # (Nghị định 13/2023/NĐ-CP).\n")
	b.WriteString("    nginx.ingress.kubernetes.io/ssl-redirect: \"true\"\n")
	if o.Stream {
		b.WriteString(fmt.Sprintf("    # %s: tệp 50 MB + phần bao multipart (ADR 0052 §Sửa đổi 09/10/2026). Giới hạn TỪNG loại\n", bodyCapUpload))
		b.WriteString("    # tệp vẫn do service kiểm theo cấu hình platform — trần này chỉ là chặn trên ở rìa.\n")
	} else {
		b.WriteString("    # Không host nào ở đây nhận tải lên tệp — giữ trần mặc định.\n")
	}
	b.WriteString(fmt.Sprintf("    nginx.ingress.kubernetes.io/proxy-body-size: %q\n", o.BodyCap))
	if o.Stream {
		b.WriteString("    # TRUYỀN THẲNG thân yêu cầu tới pod (chủ dự án chọn ghi luồng, 09/10/2026). Được: controller\n")
		b.WriteString("    # không đệm 55 MB mỗi lượt ra đĩa/RAM của nó, và pod bắt đầu ghi luồng sang MinIO từ byte đầu.\n")
		b.WriteString("    # Mất: (1) khách mạng chậm giữ một suất tải của pod (tối đa 4/pod) suốt thời gian truyền — cận\n")
		b.WriteString("    # 180 s mỗi lượt ở service chặn việc ấy; (2) nginx KHÔNG thử lại sang pod khác khi pod lỗi giữa\n")
		b.WriteString("    # chừng (proxy_next_upstream cần thân đã đệm) — client nhận lỗi và gửi lại.\n")
		b.WriteString("    nginx.ingress.kubernetes.io/proxy-request-buffering: \"off\"\n")
		b.WriteString(fmt.Sprintf("    # %s s: cận mỗi lượt tải lên chủ dự án duyệt (ADR 0052 §Sửa đổi 09/10/2026). Mặc định\n", uploadProxyTimeout))
		b.WriteString("    # 60 s của nginx trả 504 cho lượt quét mã độc / mã hoá lại ảnh xong sau 60 s, trong khi service\n")
		b.WriteString("    # vẫn lưu tệp — client gửi lại và tệp vào hai lần.\n")
		b.WriteString(fmt.Sprintf("    nginx.ingress.kubernetes.io/proxy-read-timeout: %q\n", uploadProxyTimeout))
		b.WriteString(fmt.Sprintf("    nginx.ingress.kubernetes.io/proxy-send-timeout: %q\n", uploadProxyTimeout))
	}
	b.WriteString("spec:\n")
	b.WriteString("  ingressClassName: nginx\n")
	b.WriteString("  # `tls:` do overlay THÊM VÀO — tên secret khác nhau từng môi trường.\n")
	b.WriteString("  #\n")
	if o.Web {
		b.WriteString("  # THỨ TỰ `rules` CÓ Ý NGHĨA: overlay vá host theo CHỈ SỐ (`/spec/rules/<i>/host`). Chỉ số 0 là\n")
		b.WriteString("  # host web; 1..n là host dịch vụ, xếp theo tên. Bản vá overlay `test` host cũ trước khi\n")
	} else {
		b.WriteString("  # THỨ TỰ `rules` CÓ Ý NGHĨA: overlay vá host theo CHỈ SỐ (`/spec/rules/<i>/host`), host dịch\n")
		b.WriteString("  # vụ xếp theo tên. Bản vá overlay `test` host cũ trước khi\n")
	}
	b.WriteString("  # `replace`, nên overlay lệch chỉ số là `kustomize build` ĐỔ chứ không vá nhầm host.\n")
	b.WriteString("  rules:\n")
	if o.Web {
		writeWebHost(b, web, webHost)
	}
	for i, h := range o.Hosts {
		if o.Web || i > 0 {
			b.WriteString("    #\n")
		}
		writeServiceHost(b, h, apiSuffix)
	}
}

func writeWebHost(b *strings.Builder, web luatIngress, webHost string) {
	b.WriteString("    # HOST WEB — MỘT quy tắc ký tự đại diện cho MỌI xã. `Host` thật đi nguyên vẹn tới pod\n")
	b.WriteString("    # (ingress-nginx giữ nguyên Host mặc định) và `httpx.TenantMiddleware` phân giải nó thành\n")
	b.WriteString("    # xã ở rìa ngoài cùng. Host không khớp xã nào ⇒ 404, không bao giờ một xã mặc định\n")
	b.WriteString("    # (luật 1, bất biến 3). Thêm một xã = thêm DNS + một hàng trong sổ xã của `platform`.\n")
	b.WriteString("    #\n")
	b.WriteString("    # Nhãn đầu `admin` · `admin-stg` · `api` · `api-stg` · `stg` · `www` khớp ký tự đại diện này\n")
	b.WriteString("    # nhưng là nhãn DÀNH RIÊNG: `platform` từ chối gán chúng cho một xã, nên chúng nhận 404.\n")
	b.WriteString(fmt.Sprintf("    - host: %q\n", webHost))
	b.WriteString("      http:\n")
	b.WriteString("        paths:\n")
	b.WriteString("          # BẮT HẾT — web quản trị (Next.js). KHÔNG sinh từ hợp đồng. `/api/v1/*` cũng tới đây và\n")
	b.WriteString("          # web chuyển tiếp; tiền tố lạ nhận 404 JSON, không tới dịch vụ nào (ADR 0043).\n")
	viet1Path(b, web.DichVu, web.Cong)
}

func writeServiceHost(b *strings.Builder, h hostDichVu, apiSuffix string) {
	host := h.DichVu + "." + apiSuffix
	if len(h.Exact) > 0 {
		b.WriteString(fmt.Sprintf("    # %s — CHỈ %d đường dẫn công khai, khớp ĐÚNG (Exact), không `/`: cổng REST của\n", h.DichVu, len(h.Exact)))
		b.WriteString("    # dịch vụ này còn phục vụ miền vận hành (ADR 0048), nên chỉ tuyến được chủ dự án mở mới\n")
		b.WriteString("    # ra internet — danh sách ở pathScopedPublicRoutes (tools/ingress/sinh.go).\n")
		b.WriteString(fmt.Sprintf("    - host: %q\n", host))
		b.WriteString("      http:\n")
		b.WriteString("        paths:\n")
		for _, p := range h.Exact {
			writePath(b, p, "Exact", h.DichVu, h.Cong)
		}
		return
	}
	for _, dong := range ghiChuPhu(fmt.Sprintf("%s — %d tài nguyên:", h.DichVu, len(h.TienTo)), h.TienTo) {
		b.WriteString("    # " + dong + "\n")
	}
	b.WriteString(fmt.Sprintf("    - host: %q\n", host))
	b.WriteString("      http:\n")
	b.WriteString("        paths:\n")
	viet1Path(b, h.DichVu, h.Cong)
}

func viet1Path(b *strings.Builder, dichVu, cong string) {
	writePath(b, "/", "Prefix", dichVu, cong)
}

func writePath(b *strings.Builder, path, pathType, dichVu, cong string) {
	b.WriteString("          - path: " + path + "\n")
	b.WriteString("            pathType: " + pathType + "\n")
	b.WriteString("            backend:\n")
	b.WriteString("              service:\n")
	b.WriteString("                name: " + dichVu + "\n")
	b.WriteString("                port: { name: " + cong + " }\n")
}

// duongOverlay is where the environment patch of one Ingress object lives in one overlay. The
// overlay's kustomization.yaml must name it with `target: … name: <object>` —
// TestOverlayKustomizationPatchesEveryIngress checks that, because a missing entry leaves that
// object carrying PROD hosts in the staging namespace.
func duongOverlay(mt, patch string) string {
	return "deploy/overlays/" + mt + "/" + patch
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
//
// One patch per Ingress object (ingressObjects); TLS lists only the Secrets covering that
// object's hosts — the web wildcard only where the web host is.
func sinhOverlay(mt, goc moiTruong, o ingressObject) []byte {
	var b strings.Builder
	b.WriteString("# TỆP NÀY ĐƯỢC SINH RA bởi tools/ingress (`go run ./tools/ingress`, hoặc `make kb`). KHÔNG SỬA\n")
	b.WriteString("# TAY. Phần MÔI TRƯỜNG `" + mt.Ten + "` của Ingress `" + o.Name + "` — host và TLS, và CHỈ hai thứ ấy. Bảng định\n")
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
	first := 0
	if o.Web {
		vaHost(0, goc.HostWeb, mt.HostWeb)
		first = 1
	}
	for i, h := range o.Hosts {
		vaHost(i+first, h.DichVu+"."+goc.DuoiAPI, h.DichVu+"."+mt.DuoiAPI)
	}
	b.WriteString("\n")
	b.WriteString("# `add` chứ không `replace`: base cố ý KHÔNG khai `tls`, và `replace` đổ khi đường dẫn chưa có.\n")
	b.WriteString("# Secret TLS tạo NGOÀI kho này — KHÔNG BAO GIỜ commit khoá. Tên: deploy/cau-hinh/README.md.\n")
	b.WriteString("- op: add\n")
	b.WriteString("  path: /spec/tls\n")
	b.WriteString("  value:\n")
	if o.Web {
		b.WriteString(fmt.Sprintf("    - hosts: [%q]\n", mt.HostWeb))
		b.WriteString("      secretName: " + mt.TLSWeb + "\n")
	}
	b.WriteString(fmt.Sprintf("    - hosts: [%q]\n", "*."+mt.DuoiAPI))
	b.WriteString("      secretName: " + mt.TLSAPI + "\n")
	return []byte(b.String())
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
