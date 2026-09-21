package main

import (
	"fmt"
	"sort"
	"strings"
)

// hostMacDinh is the host written into base. Every overlay patches it — prod to the same
// value, staging to its own — so no environment depends on this line being right. It is a
// wildcard and not an empty host on purpose: an Ingress rule with no host matches EVERY host,
// which on the isolation path is the one thing that must never be the default (rule 1).
const hostMacDinh = "*.vigov.vn"

// sinhYAML renders the generated Ingress. Every line is derived; nothing here is a value a
// person is expected to edit.
func sinhYAML(tuyens []tuyenHopDong, luats []luatIngress) []byte {
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
	soLuatAPI := 0
	for _, l := range luats {
		if !l.BatHet {
			soLuatAPI++
		}
	}

	b.WriteString("# TỆP NÀY ĐƯỢC SINH RA bởi tools/ingress. KHÔNG SỬA TAY — sửa tay là mất, im lặng, ở\n")
	b.WriteString("# lần sinh sau (luật 9, bất biến 8). Sinh lại: `go run ./tools/ingress` (hoặc `make kb`).\n")
	b.WriteString("#\n")
	b.WriteString("# NGUỒN CHUẨN: kb/20-contracts/openapi.json — chính nó sinh từ khai báo route trong\n")
	b.WriteString("# */internal/ (ADR 0014). Thêm một tuyến KHÔNG phải sửa tệp này: khai route trong Go,\n")
	b.WriteString("# chạy `make kb`, tệp này đổi theo. Quên sinh lại thì `go test ./tools/ingress` ĐỎ.\n")
	b.WriteString("#\n")
	b.WriteString("# VÌ SAO KHÔNG CÒN MỘT DÒNG `/api/v1` → identity: dòng ấy đúng 100% vào ngày nó được\n")
	b.WriteString("# viết, khi mọi tài nguyên REST đều thuộc identity. Nay hợp đồng có\n")
	b.WriteString(fmt.Sprintf("#\n#     %d tuyến của %d dịch vụ — %s\n#\n", len(tuyens), len(tenDichVu), strings.Join(phanBo, " · ")))
	b.WriteString("# nên dòng gộp ấy đưa mọi tuyến không phải của identity tới identity và trả 404 — 404\n")
	b.WriteString("# trên một tuyến hành chính thật. Chủ dự án chốt lối (b), SINH Ingress từ openapi.json,\n")
	b.WriteString("# ngày 21/09/2026.\n")
	b.WriteString("#\n")
	b.WriteString("# HOST và TLS KHÔNG nằm ở đây: chúng khác nhau giữa các môi trường, nên overlay vá vào\n")
	b.WriteString("# (deploy/overlays/<mt>/ingress-moi-truong.yaml). Bảng định tuyến thì giống nhau ở mọi\n")
	b.WriteString("# môi trường nên nó thuộc base — đúng ranh giới base/overlay kho này đang dùng.\n")
	b.WriteString(fmt.Sprintf("#\n# %d luật định tuyến cho %d tuyến hợp đồng, + 1 luật bắt hết cho web quản trị.\n", soLuatAPI, len(tuyens)))
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
	b.WriteString("  # `tls:` do overlay THÊM VÀO — tên secret và danh sách host khác nhau từng môi trường.\n")
	b.WriteString("  rules:\n")
	b.WriteString("    # MỘT quy tắc ký tự đại diện cho MỌI xã. `Host` thật đi nguyên vẹn tới pod —\n")
	b.WriteString("    # ingress-nginx giữ nguyên Host mặc định — và `httpx.TenantMiddleware` phân giải nó\n")
	b.WriteString("    # thành xã ở rìa ngoài cùng. Host không khớp xã nào ⇒ 404, không bao giờ một xã\n")
	b.WriteString("    # mặc định (luật 1, bất biến 3).\n")
	b.WriteString("    #\n")
	b.WriteString("    # Thêm một xã = thêm một bản ghi DNS + một hàng trong sổ đăng ký của `platform`.\n")
	b.WriteString("    # KHÔNG phải sửa tệp này. Đó là lý do nó là ký tự đại diện chứ không phải 200 dòng host.\n")
	b.WriteString("    #\n")
	b.WriteString("    # ĐÚNG MỘT phần tử trong `rules`, và overlay dựa vào điều đó: bản vá JSON6902 của nó\n")
	b.WriteString("    # trỏ vào `/spec/rules/0/host`. `TestOverlayVaDungChiSoLuatDuyNhat` giữ lời hứa ấy.\n")
	b.WriteString(fmt.Sprintf("    - host: %q\n", hostMacDinh))
	b.WriteString("      http:\n")
	b.WriteString("        # THỨ TỰ CÓ Ý NGHĨA, và bản kê này viết dài-trước-ngắn.\n")
	b.WriteString("        #\n")
	b.WriteString("        # ingress-nginx tự sắp location theo độ dài giảm dần, nên trên controller ấy thứ tự\n")
	b.WriteString("        # trong tệp không quyết định kết quả khớp. Bản kê KHÔNG dựa vào đặc tính đó: viết\n")
	b.WriteString("        # dài-trước nó cũng đúng dưới một controller lấy khớp ĐẦU TIÊN, và đọc theo đúng\n")
	b.WriteString("        # thứ tự một người sẽ suy luận.\n")
	b.WriteString("        #\n")
	b.WriteString("        # `pathType: Prefix` khớp theo ĐOẠN đường dẫn, không theo ký tự: `/api/v1/roles`\n")
	b.WriteString("        # KHÔNG nuốt `/api/v1/role-permissions`. Nhờ đó gom theo tài nguyên là an toàn —\n")
	b.WriteString("        # và vì sao gom đúng tới mức TÀI NGUYÊN chứ không hơn: `task-blocs` là identity\n")
	b.WriteString("        # trong khi `task-priorities` và `task-types` là petitions. Một tiền tố `task` sẽ\n")
	b.WriteString("        # gửi hai tuyến của petitions sang identity.\n")
	b.WriteString("        paths:\n")

	for _, l := range luats {
		if l.BatHet {
			b.WriteString("          # BẮT HẾT — web quản trị (Next.js). KHÔNG sinh từ hợp đồng.\n")
			b.WriteString("          # Một `/api/v1/<chưa định tuyến>` rơi vào đây và nhận 404 của web: nó KHÔNG tới\n")
			b.WriteString("          # một dịch vụ không sở hữu nó. Đó là hướng sai an toàn.\n")
		} else {
			for _, dong := range ghiChuPhu(l) {
				b.WriteString("          # " + dong + "\n")
			}
		}
		b.WriteString("          - path: " + l.Duong + "\n")
		b.WriteString("            pathType: Prefix\n")
		b.WriteString("            backend:\n")
		b.WriteString("              service:\n")
		b.WriteString("                name: " + l.DichVu + "\n")
		b.WriteString("                port: { name: " + l.Cong + " }\n")
	}
	return []byte(b.String())
}

// ghiChuPhu renders the comment above one rule: which service, and which contract paths it
// covers. The covered paths are written out so a reviewer can check the grouping against the
// contract without running anything.
func ghiChuPhu(l luatIngress) []string {
	dau := fmt.Sprintf("%s — %d tuyến:", l.DichVu, len(l.Phu))
	var dongs []string
	hienTai := dau
	for _, p := range l.Phu {
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
