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
  - "vì sao tên miền công khai có một nguồn duy nhất là tệp deploy/hosts.yaml chứ không phải ConfigMap, vì sao Mini App chỉ dựng với host prod (thay 14/10/2026 bởi #9), và vì sao Ingress trên cụm vẫn dựng tay (01/10/2026)"
  - "vì sao Mini App mang hai bảng host prod/dev trong một bundle, chọn theo mã profile pf= trên liên kết (chỉ digest SHA-256 trong bundle, theo lượt mở), và vì sao tham số không tên env (14/10/2026, #9)"
  - "hai môi trường vigov-dev / vigov-prod thay staging, dev có CSDL và khoá riêng, mô hình host dev (*.dev.vigov.vn, <service>-dev.api.vigov.vn, admin.dev.vigov.vn), TLS dev, vì sao nhãn stg vẫn dành riêng, và rủi ro ENV=dev chủ dự án chấp nhận (14/10/2026)"
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

> 01/10/2026 — chủ dự án chốt **nơi viết các host này**: một nguồn duy nhất `deploy/hosts.yaml`,
> giá trị không đổi. Xem [Sửa đổi 01/10/2026](#sửa-đổi-01102026--một-nguồn-cho-tên-miền-công-khai).
>
> 14/10/2026 — staging **bỏ**, thay bằng dev; hàng `<xa>.stg.vigov.vn` và `<service>.api-stg.vigov.vn`
> không còn đúng. Xem [Sửa đổi 14/10/2026](#sửa-đổi-14102026--hai-môi-trường-dev-và-prod-staging-bỏ).

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
| Mini App có **một** địa chỉ gốc, hiện rỗng (`citizen-app/src/cong-dan/api/dia-chi-vigov.ts:21`); dịch vụ Go **không có CORS** | Cả hai phải có trước khi Mini App gọi `<service>.api.vigov.vn`. **Cho phép origin nào là quyết định của chủ dự án** · 01/10/2026: bảng host API của Mini App sinh từ `deploy/hosts.yaml`, chỉ prod (Sửa đổi 01/10/2026) |
| Host `*.api` là host dành riêng → biên trả 404 mọi tuyến cần xã | Mini App gọi qua đó cần xã đến từ **phiên** (ADR 0005, 0045), không từ Host — chưa dựng |
| Gỡ hai dòng `admin*.vigov.vn` rồi `VALIDATE` | Quyết định của chủ dự án (luật 7) |
| Ca `_pg_test` của gieo (ON CONFLICT trên bảng phân mảnh, ba lần đăng nhập đầu đồng thời) chưa chạy trên PostgreSQL thật | Test xanh ở máy không Docker là SKIP, không phải bằng chứng |
| Ai được **tạo vai trò mới** | Chưa ai quyết; gieo chỉ tạo đúng một vai trò. **28/09/2026 → ADR 0055:** không có chức năng tạo vai trò; tám vai trò mẫu gieo bằng `POST /api/v1/roles/defaults` (`kb/10-decisions/0055-default-role-templates.md`) |

## Sửa đổi 26/09/2026 (cùng ngày, chủ dự án trả lời bảng "việc còn mở")

| Việc | Quyết định | Dựng ở |
|---|---|---|
| Hai dòng `admin*.vigov.vn` | **Gỡ** — `admin.vigov.vn` là tên miền của pod web quản trị tổng liên xã (chưa dựng) | bước `doi-ten-mien-thang-binh` (`deploy/Jenkinsfile`), DELETE lọc đúng host + đúng xã, cùng giao dịch với vết `doi_ten_mien`. Sau khi gỡ, `VALIDATE` 0007 làm được bằng một migration sau |
| Mini App và CORS | Chủ dự án giao nhà cung cấp đề xuất, cho **nới rộng một chút** vì triển khai phụ thuộc Zalo | `CITIZEN_CORS_ALLOWED_ORIGINS`, **chỉ** rìa công dân (`core/httpx/cors.go`); giá trị đề xuất `https://h5.zdn.vn,https://zalo.me,https://*.zdn.vn,https://*.zalo.me`; `*` trơn và `http` bị từ chối. Mini App gọi `https://petitions.api.vigov.vn` (`dia-chi-vigov.ts`) — rìa công dân lấy xã từ phiên nên host dành riêng không cản |
| Staging dùng chung CSDL/Redis/khoá | **Chấp nhận tạm** — thực tế chỉ dùng prod, staging để dự phòng. **01/10/2026:** vì thế Mini App không có trục staging — xem đoạn dưới | — |
| `<service>.api.vigov.vn` mở cả cổng rest | **Đồng ý** | — |

## Sửa đổi 01/10/2026 — một nguồn cho tên miền công khai

Chủ dự án chốt; giá trị host **không đổi** so với mô hình 26/09.

| # | Quyết định | Vì sao |
|---|---|---|
| 1 | Host công khai có **một nguồn** trong kho: [`deploy/hosts.yaml`](../../deploy/hosts.yaml). Mọi chỗ khác **đọc nó hoặc được `tools/ingress` sinh ra từ nó**: Ingress base + overlay, hằng host của luật tên miền dành riêng (service-platform) và phép kiểm `OPERATOR_HOST` (core/config, platform-admin), bảng host API theo dịch vụ của Mini App (sinh lúc build, tệp sinh được commit) | Cùng một host gõ lại ở nhiều nơi là nhiều bản sẽ lệch; bản bị quên là bản ai đó đọc |
| 2 | Mini App **chỉ dựng với host prod**, không có trục staging (`mini_app_environment`) | Staging dùng chung CSDL với prod (bảng trên); thêm một giá trị là thêm một bundle (ADR 0047). Câu "thêm dòng staging là quyết định của chủ dự án" coi như đã trả lời |
| 3 | Luật tên miền dành riêng theo cùng nguồn, **cùng một lượt** | CHECK SQL ở `service-platform/migrations/0007_tenant_domain_khong_danh_rieng.sql` không đọc được tệp: đổi `api_suffix` hay gốc `web_host` phải kèm **migration MỚI**; `TestLuatTenMienDanhRiengGoVaSQLKhop` (`service-platform/internal/domain/ten_mien_danh_rieng_sql_test.go:30`) đỏ tới khi có |
| 4 | Ingress trên cụm vẫn **dựng tay trong Rancher**; kho chỉ render (`kubectl kustomize deploy/overlays/<env>`) để nhập vào. **Không** có job Jenkins apply. DNS và Secret TLS nằm **ngoài** kho | Giữ nguyên cách vận hành cụm hiện có; kho là bản mẫu, không phải nơi ra lệnh cho cụm |
| 5 | Chủ dự án nói *"cấu hình hết vào ConfigMap cho linh động"*; dựng thành **tệp trong kho** | Không bên tiêu thụ nào đọc được ConfigMap của cụm: Mini App là bundle tĩnh trên Zalo, và host của Ingress không tham chiếu được ConfigMap |

## Sửa đổi 14/10/2026 — hai môi trường dev và prod, staging bỏ

Chủ dự án chốt 14/10/2026, cả tám điều. Dựng ở `825ab024` (namespace, RBAC Jenkins, bàn điều khiển),
`b6fe4c7e` (job `vigov-deploy`) và `02a0b01a` (host, `tools/ingress`, luật tên miền dành riêng).

| # | Quyết định | Dựng ở |
|---|---|---|
| 1 | Hai môi trường: `vigov-dev` (nhánh `dev`) và `vigov-prod` (nhánh `main`). **`vigov-staging` bỏ** | `deploy/cluster/namespace.yaml`, `rbac-jenkins.yaml`; `namespaceForBranch()` trong Jenkinsfile của từng dịch vụ (`web-admin/Jenkinsfile:367-373`); `deploy/overlays/staging/` gỡ ở `b6fe4c7e` |
| 2 | Dev có **CSDL riêng** — không còn "staging dùng chung CSDL với prod" | `deploy/overlays/dev/kustomization.yaml:3-5`; `deploy/Jenkinsfile:23-26` |
| 3 | Host dev: web `<xa>.dev.vigov.vn`; API `<service>-dev.api.vigov.vn` (`comms.api.vigov.vn` → `comms-dev.api.vigov.vn`); bàn điều khiển **`admin.dev.vigov.vn`** (chủ dự án sửa cùng ngày, thay `admin-dev.vigov.vn` chọn lúc đầu — xem dưới bảng rủi ro) | `deploy/hosts.yaml` mục `dev` (`api_label_suffix: "-dev"`); `core/config/operator.go:64` (nhãn chỉ còn `admin`); `platform-admin/src/lib/server/operator-host.ts:38`; `deploy/overlays/dev/kustomization.yaml` vá `OPERATOR_HOST` của platform-admin |
| 4 | TLS: web dev dùng Secret riêng `vigov-dev-wildcard-tls`. API dev dùng **cùng chứng chỉ** `*.api.vigov.vn`, Secret **cùng tên** `vigov-api-wildcard-tls` (một bản sao trong `vigov-dev`) | `deploy/hosts.yaml` mục `dev`; `tools/ingress` chỉ cho trùng Secret TLS API khi hai môi trường cùng `api_suffix` |
| 5 | Nhãn `stg`, `admin-stg`, `api-stg` và gốc `stg.vigov.vn`, `*.api-stg.vigov.vn` **giữ dành riêng**: CSDL prod còn hàng `<xa>.stg.vigov.vn` từ thời dùng chung | `deploy/hosts.yaml` mục `staging` `retired: true`; `service-platform/internal/domain/ten_mien_danh_rieng.go:51-60`; migration `service-platform/migrations/0027_reserve_dev_hosts.sql` |
| 6 | Cụm `vigov-dev` chạy **`ENV=dev`**. Chủ dự án **chấp nhận** ba rủi ro ở bảng dưới | `deploy/overlays/dev/kustomization.yaml:19` |
| 7 | ~~Mini App vẫn **chỉ dựng với host prod**~~ — **thay bởi #9 cùng ngày** (hai bảng host, chọn theo mã profile) | `deploy/hosts.yaml`, hai dòng cuối |
| 8 | Job vận hành: `SECRET_ENCRYPTION_KEYS` và `CITIZEN_SESSION_BRIDGE_KEYS` **sinh riêng mỗi namespace**, không bao giờ chép prod → dev. Tạo tài khoản vận hành, chỉ định owner, cấp quyền, đặt secret Mini App chạy được với `MT=dev` | `deploy/Jenkinsfile:158` (`MT` = `prod` \| `dev`), `:734-737`, `:956-960`; kiểm `MT` ở `:1394`, `:1507`, `:1633`, `:1748` |

**Rủi ro của `ENV=dev` trên cụm thật — chủ dự án chấp nhận:**

| Rủi ro | Mã |
|---|---|
| `config.Load` không chặt: thiếu biến của nhóm dịch vụ khai **không** chặn khởi động, chỉ từ chối lúc dùng. Luật "dùng thì bắt buộc" của ADR 0057 không áp ở dev | `core/config/config.go:604` (chặt chỉ khi `staging` hoặc `prod`) |
| `DANGEROUS_AUTH_BYPASS=true` **không** bị từ chối — chỉ còn dòng cảnh báo lúc khởi động | `core/config/config.go:875` (từ chối chỉ khi `prod`), `:894-896` |
| `OBJECT_STORAGE_PUBLIC_ENDPOINT` dạng `http://` được phép, không cả cảnh báo: presigned URL đi không mã hoá | `core/config/config.go:919` |

**Những dòng của ADR này nay không còn đúng** (số dòng theo bản đã có mục này):

| Dòng | Nội dung cũ | Nay |
|---|---|---|
| 41 | `<xa>.stg.vigov.vn` — web cán bộ staging | Web cán bộ dev là `<xa>.dev.vigov.vn`. Gốc `stg.vigov.vn` chỉ còn là gốc dành riêng (#5) |
| 43 | `<service>.api-stg.vigov.vn` — API staging | API dev là `<service>-dev.api.vigov.vn`, dưới cùng `api.vigov.vn` với prod |
| 56-57, 149 | "4 bản ghi DNS wildcard + 4 chứng chỉ wildcard" | Môi trường đang chạy cần 3 wildcard: `*.vigov.vn`, `*.api.vigov.vn` (prod và dev chung), `*.dev.vigov.vn`. `comms-dev.api.vigov.vn` khớp `*.api.vigov.vn` vì `comms-dev` là đúng một nhãn |
| 157 | Việc còn mở: staging và prod dùng chung CSDL, Redis, khoá ký phiên | Đã trả lời bằng #2 và #8. Redis của dev là giá trị Secret trên cụm — kho không kiểm được dev có Redis riêng hay chưa |
| 170 | "Chấp nhận tạm" việc dùng chung CSDL/Redis/khoá | Thay bằng #2 và #8 |
| 180 | Lý do của #2 (Sửa đổi 01/10): "Staging dùng chung CSDL với prod" | Lý do ấy hết đúng. Quyết định giữ nguyên theo #7; lý do còn lại là một giá trị thêm là một bundle thêm (ADR 0047) |

Không đổi: luật một nguồn `deploy/hosts.yaml`, Ingress trên cụm dựng tay (#4 của Sửa đổi 01/10), host
`admin.dev.vigov.vn` cũng **không** có trong `deploy/hosts.yaml` (cùng ngoại lệ của ADR 0048 cho
`admin.vigov.vn`). Bàn điều khiển chạy ở dev nên ADR 0048 §*Chốt triển khai 01/10/2026* #1 ("chỉ prod")
cũng hết đúng: `deploy/overlays/dev/kustomization.yaml:11` kéo `base/platform-admin`.

**Host bàn điều khiển dev — sửa cùng ngày 14/10/2026.** Chọn lúc đầu là `admin-dev.vigov.vn`; chủ dự
án đổi sang **`admin.dev.vigov.vn`** (đã dựng trên cụm). Hệ quả: host khớp `*.dev.vigov.vn`, nên
Ingress của nó dùng luôn `vigov-dev-wildcard-tls` — câu hỏi "Secret `*.vigov.vn` trong `vigov-dev`"
không còn. Nhãn bàn điều khiển chỉ còn `admin` (dưới gốc của từng môi trường); `admin-dev.vigov.vn`
nay bị `OPERATOR_HOST` từ chối nhưng nhãn `admin-dev` **vẫn dành riêng** — migration 0027 đã mang nó,
và gỡ một nhãn dành riêng chỉ nới thêm chỗ cho dòng của xã. Còn lại: `admin.stg.vigov.vn` vẫn qua phép
kiểm `OPERATOR_HOST` vì gốc `stg.vigov.vn` còn trong danh sách gốc dành riêng; không xã nào giữ được
host ấy nên không rò, chỉ là một giá trị cấu hình sai không bị chặn lúc khởi động.

## Sửa đổi 14/10/2026 (#9) — Mini App mang hai bảng host, chọn theo mã profile trên liên kết

Chủ dự án, 14/10/2026: *"ở hàm gen qr hãy thêm 1 params … để ứng dụng biết lấy data theo profile nào"*, rồi
*"dùng 1 chữ ký đi đừng lộ dev ra … chỉ mã này mới vào được dev"*, *"chỉ trong lượt mở đó"*, *"cùng App ID"*.
**Thay #2 (01/10) và #7 (14/10)** — Mini App không còn "chỉ dựng với host prod".

| # | Quyết định | Nơi trong mã |
|---|---|---|
| 9a | **Một bundle, một App ID, hai bảng host**: `mini_app_environment: prod` (mặc định) và `mini_app_dev_environment: dev`. Không phải bundle thứ hai | `deploy/hosts.yaml`; `tools/ingress/sinhhosts.go` `renderCitizenHosts` → `SERVICE_API_HOSTS` / `SERVICE_API_HOSTS_DEV` |
| 9b | Bảng dev chỉ dùng khi liên kết mở app mang `pf=<mã>` và SHA-256 của mã **khớp** digest trong bundle. Không `pf`, sai mã, lỗi băm, webview thiếu `crypto.subtle` → **prod**. Tên tham số không nói "dev"; bundle chỉ chứa digest, không chứa mã | `citizen-app/src/lib/runtime-profile.ts` |
| 9c | Profile quyết **một lần, trước lượt vẽ đầu**, không đổi trong lượt mở và **không lưu** — mở lại không có liên kết là prod | `citizen-app/src/main.tsx`; `resolveRuntimeProfile` |
| 9d | App chung lấy phiên dev từ `vihat-miniapp` **dev** (`VIGOV_API_HOST_DEV`), cầu phiên của nó trỏ identity dev. Thiếu biến thì lượt mở dev **không có** máy chủ đăng nhập — không lùi về host prod; `deploy.mjs` chặn đẩy khi thiếu | `citizen-app/src/api/dia-chi.ts`; `scripts/cau-hinh.mjs`; Jenkins credential `citizen-app-vigov-api-host-dev` |
| 9e | `npm run qr -- --profile=dev` thêm `pf`, tra App ID và mã `x` ở platform/identity **dev**, và từ chối mã không khớp digest. Mã nằm ở `citizen-app/.env.qr.local` (gitignore) hoặc biến shell `VIGOV_DEV_PROFILE_CODE` | `citizen-app/scripts/commune-qr.mjs`, `make-qr.mjs` |

**Không dùng tên `env`** (đề xuất ban đầu `env=dev`): `env` là tham số của Zalo (`env=TESTING` chọn bản thử),
một liên kết chỉ có một `env`, nên QR bản thử + dev sẽ không dựng được.

**Rủi ro chủ dự án chấp nhận:** host dev (`*-dev.api.vigov.vn`, máy chủ vihat-miniapp dev) **nằm trong bundle** —
bảng host không giấu được khỏi máy gọi nó; chỉ mã là không suy ra được. Ai cầm QR dev (in ra là mã ở dạng rõ) đều
vào dev; cụm dev chạy `ENV=dev` (#6). Xoay mã = mã mới + digest mới + bản app mới.

## Liên quan

[0003](0003-platform-admin-metadata-only.md) · [0005](0005-miniapp-tenant-resolution.md) ·
[0043](0043-web-admin-la-cong-api-k8s-chi-cap-cau-hinh.md) ·
[0045](0045-cau-phien-cong-dan-mini-app.md) · câu mở #13 (`kb/00-foundation/open-questions.json`)
