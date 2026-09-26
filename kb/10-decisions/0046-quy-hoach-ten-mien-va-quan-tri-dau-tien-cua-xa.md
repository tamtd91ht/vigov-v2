---
id: 0046-quy-hoach-ten-mien-va-quan-tri-dau-tien-cua-xa
tier: T1
source: CURATED
owner: architecture
derived_from_commit: 59c72be
expires: null
owns_facts:
  - "vì sao nền tảng dùng bốn dạng tên miền (<xa>.vigov.vn · <xa>.stg.vigov.vn · <service>.api.vigov.vn · <service>.api-stg.vigov.vn) và vì sao chúng không chồng lên nhau"
  - "vì sao admin, admin-stg, api, api-stg, stg, www và mọi host dưới .api(-stg).vigov.vn không bao giờ phân giải thành xã"
  - "vì sao hai dòng tenant_domain admin.vigov.vn / admin-stg.vigov.vn được giữ mà không phân giải, và vì sao không VALIDATE ràng buộc 0007"
  - "vì sao web cán bộ không gọi thẳng <service>.api.vigov.vn"
  - "vì sao người quản trị đầu tiên của xã được gieo ở lần đăng nhập admin đầu tiên bằng IDENTITY_ADMIN_SEED_PASSWORD, và nó căng với ADR 0003 ở đâu"
---

# 0046. Quy hoạch tên miền, và người quản trị đầu tiên của xã

**Trạng thái:** đã chốt · **Ngày:** 2026-09-26 · **Chủ dự án chốt** cả hai quyết định ·
Dựng ở commit `59c72be` (tên miền) và `b55835a` (quản trị mặc định)

Hai quyết định chung một ADR vì chúng gặp nhau ở cùng một chỗ: **Host quyết xã** (luật 1 bất
biến 3). Quyết định 1 định nghĩa host nào là của xã; quyết định 2 dùng chính host ấy để biết gieo
người quản trị cho xã nào.

## Bối cảnh

| Điều đo được | Nơi |
|---|---|
| Đường triển khai đã gắn `admin.vigov.vn` và `admin-stg.vigov.vn` cho **Xã Thăng Bình** — địa chỉ cổng nhà cung cấp trở thành bề mặt của một xã, đúng ranh giới ADR 0003 vạch | `deploy/Jenkinsfile:196-198` |
| Biên Go suy ra xã **chỉ từ `r.Host`**; host lạ là 404 | `core/httpx/edge.go:24-26` |
| Cookie phiên **không có thuộc tính `Domain`** → host-only | `service-identity/internal/http/cookie.go:28-35` |
| Xã mới không có đường sản xuất nào tạo vai trò, quyền hay người quản trị đầu tiên → ~97 tuyến `RequirePermission` trả 403 cho mọi tài khoản | `kb/90-ephemeral/tien-do/service-identity.json:185-188` (mục `xa-moi-khong-co-vai-tro-va-quyen`) |

## Quyết định 1 — quy hoạch tên miền

| Host | Tới |
|---|---|
| `<xa>.vigov.vn` | web cán bộ prod; `/api/v1/*` đi qua cổng web-admin (ADR 0043) |
| `<xa>.stg.vigov.vn` | web cán bộ staging |
| `<service>.api.vigov.vn` | API từng dịch vụ prod — vận hành, gỡ lỗi, Mini App về sau |
| `<service>.api-stg.vigov.vn` | API từng dịch vụ staging |
| `admin.vigov.vn` | cổng quản trị tổng liên xã của Vihat — **CHƯA DỰNG** |

### Vì sao bốn dạng không chồng nhau

Dấu `*` trong host của Ingress, trong chứng chỉ TLS (RFC 6125) và trong DNS đều khớp **đúng một
nhãn**. `identity.api.vigov.vn` vì thế không bao giờ khớp `*.vigov.vn`, và
`<xa>.stg.vigov.vn` không khớp `*.vigov.vn`. Cái giá: **4 bản ghi DNS wildcard + 4 chứng chỉ
wildcard**; chứng chỉ wildcard của Let's Encrypt chỉ cấp qua **DNS-01**.

**Bẫy DNS (RFC 4592).** Khi đã có `*.api.vigov.vn`, tên `api.vigov.vn` trở thành một nút trung
gian rỗng (empty non-terminal): `*.vigov.vn` **không còn phủ nó**. Tương tự cho `api-stg` và `stg`.
Muốn các tên ấy trả lời thì phải khai riêng.

### Vì sao web cán bộ không gọi thẳng `<service>.api.vigov.vn`

1. Cookie phiên host-only (`cookie.go:28-35`) — trình duyệt không gửi nó tới `*.api.vigov.vn`.
2. Biên Go chỉ lấy xã từ Host (`edge.go:24-25`); host `*.api` là host dành riêng (dưới đây) → 404.

Hai lý do độc lập; nới một cái (cookie `Domain=.vigov.vn`) là vi phạm luật 1 cấm #3. Đường của web
vẫn là cổng ADR 0043.

### Tên miền dành riêng — dựng ở `59c72be`

- `domain.LaTenMienDanhRieng` (`service-platform/internal/domain/ten_mien_danh_rieng.go:51`):
  gốc `vigov.vn`, `stg.vigov.vn`; nhãn đầu `admin | admin-stg | api | api-stg | stg | www` ngay dưới
  `vigov.vn` hoặc `stg.vigov.vn`; mọi host kết thúc `.api.vigov.vn` / `.api-stg.vigov.vn`. Nhãn chỉ
  **chứa** các chữ ấy (`apixa`) vẫn là xã.
- Từ chối **trước khi tra bảng**, cùng câu trả lời với host lạ — không để phân biệt "dành riêng"
  với "chưa ai nhận" (`service-platform/internal/store/directory.go:105`, `:143`;
  `service-platform/internal/grpc/server.go:96`).
- Migration `0007_tenant_domain_khong_danh_rieng.sql` thêm CHECK **NOT VALID**, cùng một luật với
  hàm Go, giữ khớp bằng ca kiểm đọc chính tệp SQL.

**Hai dòng cũ `admin.vigov.vn` / `admin-stg.vigov.vn` → Xã Thăng Bình được GIỮ** (luật 7; chủ dự
án quyết việc gỡ). Chúng không phân giải nữa vì phép tra từ chối trước, không phải vì ai sửa
chúng. Hệ quả cố ý:

- **Không bao giờ `VALIDATE CONSTRAINT`** khi hai dòng còn đó — nó sẽ thất bại, và thất bại ấy đúng.
- `UPDATE` hai dòng ấy (kể cả đổi `la_chinh`) nay bị PostgreSQL từ chối: chúng là bằng chứng
  đóng băng của một ánh xạ cũ.

**Thứ tự triển khai** (chủ dự án chốt): DNS/TLS/Ingress → stage Jenkins `doi-ten-mien-thang-binh`
(gắn `thangbinh-danang.vigov.vn` làm tên chính + `thangbinh-danang.stg.vigov.vn`) → job
service-platform. Đảo thứ tự thì Xã Thăng Bình mất host trước khi có host mới. Sau đó mỗi dịch vụ
vẫn trả ánh xạ Host→xã cũ trong tối đa `TENANT_CACHE_TTL` (mặc định 30s,
`core/config/config.go:535`).

### `admin.vigov.vn` — chưa dựng, và cần ADR riêng

Cổng quản trị tổng là vai trò **vượt một xã**: luật 1 điều kiện dừng #2 và #5, luật 5 điều kiện
dừng #3. Hôm nay nó chỉ là một host dành riêng không phân giải thành xã nào. Dựng nó cần ADR riêng
nối tiếp ADR 0003.

## Quyết định 2 — người quản trị đầu tiên của xã

Dựng ở `b55835a`: `service-identity/internal/app/gieo_quan_tri.go`,
`service-identity/internal/store/quan_tri_mac_dinh.go`.

Tài khoản `admin` được tạo ở **lần đăng nhập đầu tiên tại tên miền của xã** khi **đủ cả bốn**:

| Điều kiện | Vì sao |
|---|---|
| Email gõ vào đúng chuỗi `admin` | Không địa chỉ nào khác gieo được |
| Xã chưa có email `admin` ở **bất kỳ trạng thái nào** (xoá mềm, khoá, chỉ danh bạ đều tính) | Có dòng nghĩa là ai đó đã chọn; gieo cạnh nó là đảo ngược lặng lẽ quyết định ấy |
| Biến tuỳ chọn `IDENTITY_ADMIN_SEED_PASSWORD` có giá trị (Secret, chỉ `identity`; ngắn hơn độ dài tối thiểu = tắt) | Tắt được bằng cách gỡ key |
| Mật khẩu gõ khớp biến ấy, **so hằng thời gian** | Sai thì trả lời y hệt sai mật khẩu thường — không lộ xã nào còn gieo được |

Xã là xã mà Host đã phân giải ở biên; không gì client gửi chọn được xã (luật 1).

**Một giao dịch** (luật 6 bất biến 3): vai trò `quan-tri-he-thong` "Quản trị hệ thống" nhận **mọi
khoá của bảng `quyen`** (đọc từ bảng, không từ danh sách Go — luật 5 bất biến 3c), tài khoản
`phai_doi_mat_khau = true`, vết `gieo_quan_tri_mac_dinh` với chủ thể **system**. Áp cả production.
Đường này **không bao giờ ghi đè hay hồi sinh** một `admin` đã có; vai trò `quan-tri-he-thong` đã
xoá mềm thì từ chối, không tạo lại.

### Phương án đã bỏ

| Phương án | Vì sao bỏ |
|---|---|
| Băm mật khẩu `admin` ghi thẳng trong SQL migration (chủ dự án muốn trước tiên) | Tầng phân quyền của phiên làm việc từ chối đưa một thông tin xác thực vào git — bản băm vào git là không thu hồi được (luật 8). Chủ dự án chấp nhận thiết kế biến môi trường |
| Không làm gì, chờ cổng quản trị tổng | Mọi xã thật đứng ở 403 cho tới ngày có cổng ấy |

### Căng thẳng — nói thẳng

- **ADR 0003** chốt nhà cung cấp chỉ thao tác siêu dữ liệu. **Ai giữ bí mật này mở được quyền
  quản trị ở mọi xã chưa gieo** — quyền vào dữ liệu nghiệp vụ của xã, không phải siêu dữ liệu.
- **Câu mở #13** (`kb/00-foundation/open-questions.json:268-292`) hỏi *"Có chặn thao tác làm xã mất
  người quản trị CUỐI CÙNG không"* — **khác câu này**. Nhưng quyết định của #13 viết: *"KHÔNG mở
  đường khôi phục cho nhà cung cấp — lối ấy là mở rộng ADR 0003 và cần ADR riêng."* Gieo không phải
  khôi phục: nó chỉ chạy khi xã **chưa từng** có `admin`, và không mở lại được một xã đã tự khoá
  mình. Nên ADR này **không** đóng, không sửa #13; lối khôi phục vẫn bị cấm.
- Giảm thiểu: bắt đổi mật khẩu ở lần đăng nhập đầu; **gỡ key khỏi Secret khi các xã đã đổi mật
  khẩu `admin`**; không ghi đè, không hồi sinh. Giảm thiểu, không triệt tiêu: trong khoảng một xã
  đã có tên miền mà chưa gieo, người giữ bí mật vào được trước cán bộ của xã.

## Hệ quả

| Được | Mất / phải trả |
|---|---|
| Host cổng nhà cung cấp không bao giờ đưa một xã vào context | 4 DNS wildcard + 4 chứng chỉ wildcard DNS-01; bẫy nút trung gian rỗng |
| Tên miền môi trường và dịch vụ không cần hàng `tenant_domain` | Hai dòng cũ đóng băng, ràng buộc 0007 treo NOT VALID tới khi chủ dự án quyết |
| Xã mới tự có người quản trị, có vết kiểm toán | Một bí mật dùng chung mở `admin` ở mọi xã chưa gieo |

## Việc còn mở — chưa ai quyết

| Việc | Vì sao quan trọng |
|---|---|
| Staging và prod **dùng chung CSDL, Redis, khoá ký phiên** | Tên miền chỉ tách môi trường **bằng mắt**; liên kết host chính (`la_chinh`) trên staging là liên kết prod |
| Mini App có **một** địa chỉ gốc, hiện rỗng (`citizen-app/src/cong-dan/api/dia-chi-vigov.ts:21`); dịch vụ Go **không có CORS** | Cả hai phải có trước khi Mini App gọi `<service>.api.vigov.vn`. **Cho phép origin nào là quyết định của chủ dự án** |
| Host `*.api` là host dành riêng → biên trả 404 mọi tuyến cần xã | Mini App gọi qua đó cần xã đến từ **phiên** (ADR 0005, 0045), không từ Host — chưa dựng |
| Gỡ hai dòng `admin*.vigov.vn` rồi `VALIDATE` | Quyết định của chủ dự án (luật 7) |
| Ca `_pg_test` của gieo (ON CONFLICT trên bảng phân mảnh, ba lần đăng nhập đầu đồng thời) chưa chạy trên PostgreSQL thật | Test xanh ở máy không Docker là SKIP, không phải bằng chứng |
| Ai được **tạo vai trò mới** | Chưa ai quyết; gieo chỉ tạo đúng một vai trò |

## Sửa đổi 26/09/2026 (cùng ngày, chủ dự án trả lời bảng "việc còn mở")

| Việc | Quyết định | Dựng ở |
|---|---|---|
| Hai dòng `admin*.vigov.vn` | **Gỡ** — `admin.vigov.vn` là tên miền của pod web quản trị tổng liên xã (chưa dựng) | bước `doi-ten-mien-thang-binh` (`deploy/Jenkinsfile`), DELETE lọc đúng host + đúng xã, cùng giao dịch với vết `doi_ten_mien`. Sau khi gỡ, `VALIDATE` 0007 làm được bằng một migration sau |
| Mini App và CORS | Chủ dự án giao nhà cung cấp đề xuất, cho **nới rộng một chút** vì triển khai phụ thuộc Zalo | `CITIZEN_CORS_ALLOWED_ORIGINS`, **chỉ** rìa công dân (`core/httpx/cors.go`); giá trị đề xuất `https://h5.zdn.vn,https://zalo.me,https://*.zdn.vn,https://*.zalo.me`; `*` trơn và `http` bị từ chối. Mini App gọi `https://petitions.api.vigov.vn` (`dia-chi-vigov.ts`) — rìa công dân lấy xã từ phiên nên host dành riêng không cản |
| Staging dùng chung CSDL/Redis/khoá | **Chấp nhận tạm** — thực tế chỉ dùng prod, staging để dự phòng | — |
| `<service>.api.vigov.vn` mở cả cổng rest | **Đồng ý** | — |

## Liên quan

[0003](0003-platform-admin-metadata-only.md) · [0005](0005-miniapp-tenant-resolution.md) ·
[0043](0043-web-admin-la-cong-api-k8s-chi-cap-cau-hinh.md) ·
[0045](0045-cau-phien-cong-dan-mini-app.md) · câu mở #13 (`kb/00-foundation/open-questions.json`)
