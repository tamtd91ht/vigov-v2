---
id: 0043-web-admin-la-cong-api-k8s-chi-cap-cau-hinh
tier: T1
source: CURATED
owner: architecture
derived_from_commit: ec3801f
expires: null
owns_facts:
  - "vì sao web-admin là cổng chuyển tiếp /api/v1/* tới dịch vụ Go, và k8s chỉ cấp Secret + ConfigMap"
  - "vì sao Ingress SINH từ openapi.json không còn là nơi định tuyến duy nhất, dù vẫn được sinh"
  - "vì sao mọi dịch vụ Go mặc định nghe :8080"
  - "vì sao IP khách trong nhật ký kiểm toán chỉ đúng sau khi đặt TRUSTED_PROXY_CIDRS, và giới hạn X-Forwarded-For đo được ở Next"
---

# 0043. web-admin là cổng `/api/v1/*` — k8s chỉ cấp cấu hình

**Trạng thái:** đã chốt · **Ngày:** 2026-09-25 · **Người dùng chốt** sau sự cố prod cùng ngày ·
Dựng ở commit `b4e526d`, `66ad9ec`, `1b02289`, `9441ab4`, `ec3801f`

## Bối cảnh

**Sự cố prod 25/09/2026.** Cụm đưa **mọi** đường dẫn của `admin.vigov.vn`, kể cả `/api/v1/*`,
vào web-admin. `proxy.ts` chuyển hướng lời gọi API về `/dang-nhap`; `resolveTenant` nhận 307 thay
cho JSON và ném lỗi → trang `/dang-nhap` trả 500 (digest `303334939`). Không trang nào mở được.

Ba điều đã đo trước đó (bàn giao 24/09) và vẫn đúng:

| Điều đo được | Nơi |
|---|---|
| `fetch` của Node (undici) **ghi đè `Host`** bằng host của URL — thử bốn cách, cả bốn đều gửi tên dịch vụ nội bộ | `web-admin/src/lib/may-chu/goi-noi-bo.ts:13-20` |
| Biên Go suy ra xã **chỉ từ `r.Host`** — gửi sai Host là 404 mọi trang, "chữa" bằng cách đăng ký host nội bộ thành một xã là rò giữa hai cơ quan (luật 1) | `core/httpx/edge.go:17-47` |
| Pod chạy thiếu `LISTEN_ADDR` nghe 8083/8084/8086/8087 trong khi Service, probe và NetworkPolicy đều giả định 8080 | commit `66ad9ec` |

Ngày 21/09 chủ dự án đã chọn **lối (b)**: SINH Ingress từ `openapi.json`, để cụm định tuyến theo
tiền tố (`tools/ingress/main.go:16-17`). Lối ấy đúng khi cụm **áp** `ingress.yaml`. Prod cho thấy
cụm không áp: nó chỉ có một luật `/` → web-admin. Sổ `web-admin/goc-api-noi-bo` lúc ấy treo ba
phương án A/B/C, chưa ai chọn.

## Các phương án

| Phương án | Được | Mất |
|---|---|---|
| Giữ lối (b), bắt cụm áp `ingress.yaml` sinh ra | Không thêm chặng; mỗi dịch vụ nhận thẳng từ ingress | Định tuyến là việc của người vận hành cụm — đúng chỗ đã hỏng ở prod, không ai trong kho kiểm được |
| A — `node:http` + gốc nội bộ **chỉ** cho `resolveTenant` | Chữa trang 500 | Trình duyệt vẫn gọi `/api/v1/*` vào web-admin → vẫn không ai trả lời |
| B — web-admin gọi ra `https://<Host>` qua 443 | Không đổi mã Go | Vòng lặp về chính web-admin (prod đã chứng minh); cần egress ra ngoài cụm |
| C — biên Go đọc `X-Forwarded-Host` | Một dòng Go | Thêm một nguồn thứ hai cho câu "xã nào" trên đường cách ly — thứ luật 1 cấm |
| **D (chọn)** — web-admin là cổng `/api/v1/*`, định tuyến nằm trong ứng dụng | k8s chỉ cần Secret + ConfigMap; bảng định tuyến nằm trong kho và có test | Thêm một chặng Node vào mọi lời gọi API cán bộ; web-admin nằm trên mọi đường API |

## Quyết định

### 1. k8s chỉ cấp Secret + ConfigMap; định tuyến nằm trong ứng dụng

- `web-admin/src/app/api/v1/[...duong]/route.ts` chuyển mọi `/api/v1/*` sang
  `web-admin/src/lib/may-chu/chuyen-tiep.ts`.
- Chủ sở hữu chọn theo **đoạn đường dẫn** (`chuyen-tiep.ts:55-60`) trên bảng
  `web-admin/src/lib/api/dinh-tuyen.gen.ts`, do `tools/ingress` **sinh từ `openapi.json`** cùng
  bảng đã phân giải với `deploy/base/mang/ingress.yaml` (`tools/ingress/main.go:57-60` ghi cả hai
  một lượt; `tools/ingress/sinhts_test.go` chống lệch với tệp trên đĩa).
- Tiền tố lạ → **404 JSON** dạng `httpx.Error`, **không có dịch vụ mặc định** (`chuyen-tiep.ts:122-123`).
- Gọi bằng `node:http`, không `fetch`, vì undici ghi đè Host. `Host` chuyển **nguyên vẹn**; biên Go
  vẫn là nơi duy nhất quyết xã.
- Bỏ khỏi yêu cầu: `x-tenant*`, `forwarded`, `x-forwarded-host`, header hop-by-hop
  (`chuyen-tiep.ts:43-50`, `:85-86`). Biên Go vẫn bỏ `x-tenant*` lần nữa (`core/httpx/edge.go:52-69`).
- Cổng **không** xác thực, không kiểm quyền: mỗi dịch vụ Go tự kiểm (luật 5) — kiểm ở đây là một
  bản sao sẽ lệch (`chuyen-tiep.ts:20-28`).
- Địa chỉ dịch vụ: `IDENTITY_HTTP_ADDR` · `DOCUMENTS_HTTP_ADDR` · `PETITIONS_HTTP_ADDR` ·
  `FINANCE_HTTP_ADDR` · `COMMS_HTTP_ADDR`, mặc định `http://<dịch vụ>:8080`
  (`web-admin/src/lib/may-chu/goc-dich-vu.ts:25-31`). Mặc định này được phép dù luật 1 cấm mặc
  định: nó chỉ tên **dịch vụ**, không bao giờ tên **xã** (`goc-dich-vu.ts:14-18`).
- `resolveTenant` gọi thẳng identity nội bộ với Host của xã (`web-admin/src/lib/tenant-config.ts:113`).
- `proxy.ts` loại `/api/` và `/healthz` khỏi `matcher` (`web-admin/src/proxy.ts:62`).
- Probe dùng `/healthz` — không hỏi xã, không gọi backend (`web-admin/src/app/healthz/route.ts`,
  `deploy/base/web-admin/deployment.yaml:66-74`).

**Thay thế:** lối (b) ngày 21/09 **không còn là nơi định tuyến duy nhất**. `ingress.yaml` vẫn được
sinh và vẫn đúng nếu một cụm áp nó — cả hai cùng một bảng, nên không lệch được. Quyết định này cũng
thay ba phương án A/B/C của sổ `web-admin/goc-api-noi-bo`.

### 2. Mọi dịch vụ Go mặc định nghe `:8080`

`LISTEN_ADDR` trống → `:8080` cho mọi dịch vụ (`core/config/config.go:77-91`, `:400`); bỏ
`ListenAddrHoac`. **Đảo** lý do của `0645284` (mặc định riêng từng dịch vụ để chạy nhiều tiến
trình trên một máy): Service, probe `httpGet` và NetworkPolicy đều giả định 8080, nên mặc định
riêng làm pod thiếu biến nghe một cổng không ai gọi. Chạy nhiều dịch vụ trên một máy: đặt
`LISTEN_ADDR` từng tiến trình (`.env.example:51`).

### 3. IP khách cho nhật ký kiểm toán: chỉ tin proxy đã CẤU HÌNH

Sau cổng, `r.RemoteAddr` ở Go chỉ còn là IP pod web-admin — nhật ký kiểm toán mất "từ IP nào"
(luật 6 bất biến 2).

- `TRUSTED_PROXY_CIDRS` (ConfigMap, không bắt buộc). **Trống = không tin ai** — đúng hành vi cũ.
  Mục sai hoặc `0.0.0.0/0`/`::/0` → dừng khởi động (`core/config/config.go:290-337`).
- Go đọc `X-Forwarded-For` **chỉ khi** chặng trước nằm trong dải tin cậy, đi **từ phải sang
  trái**, dừng ở địa chỉ đầu tiên ngoài dải (`core/httpx/client_ip.go:18-31`). Phần bên trái do
  người gửi tự viết, không bao giờ được lấy. Gắn ở lớp ngoài cùng
  (`service-identity/cmd/server/main.go:411`).

**GIỚI HẠN ĐÃ ĐO, không phải lựa chọn.** Next chỉ điền `X-Forwarded-For` từ socket bằng `??=`
(`web-admin/node_modules/next/dist/server/base-server.js:612`), tức chỉ khi yêu cầu **chưa có**
header ấy; route handler không được trao socket. Nên web-admin **chuyển XFF nguyên vẹn**
(`chuyen-tiep.ts:89-95`) và không nối được địa chỉ chặng nó thấy. Sau ingress-nginx, mục ngoài cùng
bên phải là thứ **ingress** quan sát, không phải địa chỉ của ingress.

Hệ quả: tin dải pod web-admin chỉ an toàn **khi pod web-admin chỉ nhận được kết nối từ ingress**
(NetworkPolicy quy tắc 2, `cho-phep-rest-tu-ingress`). Ai nối thẳng được vào pod web-admin thì tự
viết được mục bên phải và chọn IP ghi vào nhật ký kiểm toán.

Người vận hành đặt `TRUSTED_PROXY_CIDRS` = dải pod web-admin. **Chưa đặt thì IP trong nhật ký
kiểm toán là IP pod web-admin** — không sai xã, nhưng không trả lời được "từ IP nào".

## Hệ quả

| Được | Mất / phải trả |
|---|---|
| Cụm cần đúng Secret + ConfigMap; một luật `/` → web-admin là đủ | **Thêm một chặng Node** trên mọi lời gọi API cán bộ — độ trễ và một điểm hỏng |
| Bảng định tuyến có test, sinh từ hợp đồng, không ai sửa tay | **web-admin nằm trên mọi đường API cán bộ**: web-admin chết là mọi tuyến cán bộ chết, kể cả khi dịch vụ Go xanh |
| Mọi pod nghe 8080 khớp Service/probe/netpol mà không cần biến | NetworkPolicy phải thêm **hai** luật: `cho-phep-rest-tu-web-admin` (REST 8080 nhận từ pod web-admin) và `cho-phep-web-admin-ra-rest` (web-admin ra 8080). Hai luật này **đang ở cây làm việc, chưa commit**, vì `deploy/base/mang/netpol.yaml` có sửa song song. Thiếu chúng: mọi lời gọi API qua cổng là 502 dù mọi pod xanh |
| IP khách thật khi cấu hình đúng | IP khách phụ thuộc NetworkPolicy **được cưỡng chế** — xem việc mở dưới |

## Việc còn mở — chưa ai quyết, chưa ai chứng minh

| Việc | Vì sao quan trọng |
|---|---|
| **Kiểm thử đột biến** trên các dòng canh của cổng (bỏ `x-tenant*`, 404 tiền tố lạ, không mặc định) **chưa chạy** — bộ phân loại của chế độ tự động từ chối | Test xanh chưa chứng minh chúng bắt được lỗi — xoá một dòng canh mà test vẫn xanh là rào đã chết |
| Commit hai luật NetworkPolicy mới | Chưa có trên `main` |
| Đặt `TRUSTED_PROXY_CIDRS` ở từng môi trường | Chưa đặt thì IP kiểm toán là IP pod |
| Quy tắc 2 chọn namespace `ingress-nginx`, nhưng RKE2 đặt controller ở `kube-system`; chỉ có flannel thì NetworkPolicy **không được cưỡng chế** (`kb/90-ephemeral/ban-giao-phien.md` §5) | Tiền đề an toàn của XFF ở mục 3 là chính NetworkPolicy ấy |
| Bảng sinh có cả tuyến công dân `my-*`; qua cổng, chúng tới được từ host cán bộ, chỉ còn `CitizenOnly` ở Go chặn | Tách hai bề mặt ở **tầng mạng** (`kb/00-foundation/ubiquitous-language.md:189`) chỉ còn khi cụm áp `ingress.yaml` — cần người dùng xác nhận chấp nhận |
