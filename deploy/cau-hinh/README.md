# Secret và ConfigMap của ViGov

Tên đối tượng theo **cụm đang chạy** (dựng tay trong Rancher, namespace `vigov-prod`; `vigov-dev` dựng cùng
tên, CSDL và khoá riêng — ADR 0046 §Sửa đổi 14/10/2026). Giá trị dưới
đây là **hình dạng**, anh tự điền giá trị thật. Key viết **gạch dưới** (`DATABASE_DSN`), không gạch
ngang — manifest dùng `envFrom`, key sai dạng bị bỏ qua im lặng.

Thêm hay sửa key xong: `kubectl -n vigov-prod rollout restart deploy/vigov-service-<dịch vụ>`.

(`deploy/base/` đọc đúng các tên này qua `envFrom` — `core/config/hints_test.go` kiểm.)

**Pod dừng vì thiếu biến?** Chính dòng log đã in, cho từng biến thiếu: nó là gì, lấy giá trị thế nào,
đặt vào đối tượng nào, dạng giá trị ra sao. Nguồn của đoạn chữ ấy là `core/config/hints.go`; cột
"Là gì · lấy giá trị" dưới đây là bản tóm tắt của cùng bảng đó — lệch nhau thì `hints.go` đúng.

**Cột "Bắt buộc":** `có` — thiếu thì pod của các dịch vụ ghi sau dấu — **không khởi động**, mọi môi
trường. `có (prod)` — như vậy ở **prod** (và `ENV=staging`); `ENV=dev` cho thiếu (tính năng tắt) — cụm
`vigov-dev` chạy `ENV=dev`, chủ dự án chấp nhận (ADR 0046 §Sửa đổi 14/10/2026). `không` — có mặc
định hoặc tắt là đúng thiết kế. Dịch vụ không dùng một key thì **bỏ qua** key đó dù nó có trong
`common-config`.

**Trước khi rollout bản có `config.Uses` (29/09/2026)**: đặt đủ mọi key `có (prod)` bên dưới, nếu không
pod sẽ dừng và gọi tên key thiếu. Dễ sót nhất: `TRUSTED_PROXY_CIDRS`, `CITIZEN_SESSION_BRIDGE_*`,
`PETITIONS_GRPC_ADDR`/`DOCUMENTS_GRPC_ADDR`, `COMMS_GRPC_ADDR`, `OBJECT_STORAGE_*`, `MALWARE_SCANNER_ADDRESS`,
`SECRET_ENCRYPTION_KEYS`.

**Trước khi rollout platform có logo/banner xã (ADR 0069, 02/10/2026):** platform nay khai thêm
`IdentityClient`, `ObjectStore`, `PublicMedia`, `MalwareScan`, nên thiếu một trong `IDENTITY_GRPC_ADDR`
(manifest đã có), `OBJECT_STORAGE_ENDPOINT` · `_PUBLIC_ENDPOINT` · `_BUCKET_PREFIX` ·
`_PUBLIC_MEDIA_BASE_URL` · `MALWARE_SCANNER_ADDRESS` (common-config) hoặc `OBJECT_STORAGE_ACCESS_KEY` ·
`_SECRET_KEY` (`platform-secrets`, cặp MinIO riêng `…/platform/*` + ghi bucket public) là pod platform
**không khởi động** — và platform chết là mọi xã trả 404.

**Trước khi rollout petitions có "chuyển đơn thư thành nhiệm vụ" (ADR 0085, 09/10/2026):** petitions
nay khai thêm `DocumentsClient`, nên thiếu `DOCUMENTS_GRPC_ADDR` trong `env:` của
`vigov-service-petitions` là pod petitions **không khởi động** ở prod (dev: khởi động, tính năng tắt). Cụm dựng tay không nhận
dòng mới của `deploy/base/petitions` — đặt tay trước khi đổi ảnh:
`kubectl -n <namespace> set env deploy/vigov-service-petitions DOCUMENTS_GRPC_ADDR=<tên Service của documents>:9090`.
Cách làm sẵn: job vận hành `deploy/Jenkinsfile`, `HANH_DONG=bo-sung-cau-hinh-petitions` (MT=dev, rồi MT=prod;
`XAC_NHAN` = namespace) — chỉ thêm biến khi thiếu, rồi khởi động lại petitions.
Cùng lúc áp hai luật mạng mới (petitions → documents 9090, quy tắc 3d + 4b của
`deploy/base/mang/netpol.yaml`) — thiếu thì pod vẫn chạy nhưng tuyến ấy treo tới hết hạn rồi trả 503.

## 1. Secret

### `<dịch vụ>-secrets` — type `Opaque`, mỗi dịch vụ một cái

`platform-secrets` · `identity-secrets` · `documents-secrets` · `finance-secrets` ·
`petitions-secrets` · `comms-secrets` · `reporting-secrets`

**Key chung, cả 7 Secret:**

| Key | Bắt buộc | Là gì · lấy giá trị | Value |
|---|---|---|---|
| `DATABASE_DSN` | **có** — mọi dịch vụ | Kết nối PostgreSQL tới CSDL riêng của dịch vụ. Người quản trị PostgreSQL cấp user + mật khẩu | `postgres://<user>:<mật khẩu>@<host>:5432/vigov_<dịch vụ>?sslmode=require` — mỗi dịch vụ một CSDL riêng |
| `GRPC_CALLER_KEY` | **có** — platform, identity, documents, finance, petitions, comms, reporting | Khoá xác thực lời gọi gRPC giữa các dịch vụ (ADR 0025). Sinh một lần, dùng lại ở cả 7 | `openssl rand -base64 48` — **cùng một giá trị** ở cả 7 |
| `REDIS_DSN` | **có (prod)** — identity, documents, finance, petitions, comms, platform | Redis chống gửi trùng + giới hạn tần suất. platform dùng nó cho giới hạn đăng nhập vận hành 20 lần / 15 phút / IP (ADR 0048 §01/10 #3) — thiếu thì mọi bước đăng nhập vận hành trả 503. Người vận hành Redis cấp | `redis://:<mật khẩu>@<host>:6379/0` — 6 Secret (từ 01/10/2026 có cả `platform-secrets`), **không** có ở `reporting-secrets` |

**Key thêm, chỉ ở một Secret:**

| Key | Bắt buộc | Secret | Là gì · lấy giá trị | Value |
|---|---|---|---|---|
| `SESSION_SIGNING_KEYS` | **có (prod)** — identity | `identity-secrets` **chỉ nơi này** | Khoá ký phiên đăng nhập cán bộ. Tự sinh | `<khoá mới>,<khoá cũ>` (mỗi khoá `openssl rand -base64 48`) |
| `CITIZEN_SESSION_BRIDGE_KEYS` | **có (prod)** — identity | `identity-secrets` | Khoá backend `vihat-miniapp` gửi khi đổi phiên công dân (ADR 0045). Tự sinh, đặt cùng giá trị ở hai bên | `<khoá mới>,<khoá cũ>`, mỗi khoá ≥ 32 byte (`openssl rand -base64 48`). Cùng giá trị ở Secret của `vihat-miniapp`. Khác `GRPC_CALLER_KEY` |
| `IDENTITY_ADMIN_SEED_PASSWORD` | không — kể cả prod | `identity-secrets` | Công tắc một lần tạo `admin` cho xã mới | mật khẩu ≥ 12 ký tự — tài khoản `admin` đầu tiên của xã mới. Gỡ key khi mọi xã đã đổi mật khẩu |
| `OPERATOR_SESSION_SIGNING_KEYS` | **có (prod)** — identity, platform | `identity-secrets` · `platform-secrets` — **cùng một giá trị** | Khoá ký phiên nhà vận hành: identity ký, platform kiểm chữ ký (ADR 0048 §01/10 #2). Tự sinh | `<khoá mới>,<khoá cũ>`, mỗi khoá ≥ 32 byte. Khác mọi khoá của `SESSION_SIGNING_KEYS` |
| `OPERATOR_TOTP_ENCRYPTION_KEY` | **có (prod)** — identity | `identity-secrets` | Khoá mã hoá bí mật TOTP nhà vận hành. Tự sinh | `<khoá mới>,<khoá cũ>`, mỗi khoá `openssl rand -base64 32` |
| `SECRET_ENCRYPTION_KEYS` | **có (prod)** — comms, identity | `comms-secrets` · `identity-secrets` | Khoá mã hoá bí mật riêng của từng xã trong CSDL (ADR 0009): comms niêm mật khẩu máy chủ thư, identity niêm secret Zalo của app riêng từng xã (ADR 0066). Tự sinh | `<khoá mới>,<khoá cũ>`, mỗi khoá `openssl rand -base64 32` — **mỗi Secret một giá trị riêng** (hai CSDL, hai bộ khoá dữ liệu). **Sao lưu riêng trước khi lưu bí mật đầu tiên** — mất là mất hết |
| `OBJECT_STORAGE_ACCESS_KEY` | **có (prod)** — petitions, comms, platform (từ 02/10/2026) | `petitions-secrets` · `comms-secrets` · `platform-secrets` — **mỗi Secret một cặp khác** | Cặp khoá MinIO riêng của dịch vụ (ADR 0052 §3). Tạo user trong MinIO (`mc admin user add`), policy chỉ `…/<dịch vụ>/*` của môi trường; comms và platform còn cần ghi bucket public (bản dẫn xuất ảnh bìa; logo + banner web-admin của xã, ADR 0052 §11, ADR 0069) | access key MinIO riêng của dịch vụ |
| `OBJECT_STORAGE_SECRET_KEY` | **có (prod)** — petitions, comms, platform | `petitions-secrets` · `comms-secrets` · `platform-secrets` | Nửa kia của cặp trên | secret key đi cặp |
| `ZALO_OA_APP_SECRET` | **có (prod)** — comms | `comms-secrets` **chỉ nơi này** | Secret key của Zalo App **dùng chung** cả nền tảng mà mỗi xã uỷ quyền OA của mình cho (ADR 0098 #2). comms gửi làm header `secret_key` khi đổi mã uỷ quyền / làm mới token OA của xã. **Mở được token của mọi xã**: lộ là xoay ở Zalo rồi mọi xã uỷ quyền lại. Lấy ở developers.zalo.me > app của ViHAT > Cài đặt | Secret key của app — không dán vào chat, tài liệu, manifest. Đi cùng `ZALO_OA_APP_ID` (mục 2) và `ZALO_OA_CALLBACK_HOST` (mục 3); dev để trống cả ba = nhóm tắt |
| `ZALO_OA_WEBHOOK_SECRET` | **có (prod)** — comms | `comms-secrets` **chỉ nơi này** | "OA Secret Key" ở mục **Webhook** của Zalo App dùng chung — **khác** `ZALO_OA_APP_SECRET`. Zalo ký mọi sự kiện OA gửi tới `https://<ZALO_OA_CALLBACK_HOST>/api/v1/zalo-oa-events` (header `X-ZEvent-Signature`); sai chữ ký → 403, không chuyển gì lên Kafka. Ai giữ khoá giả được sự kiện của OA mọi xã. Lấy ở developers.zalo.me > app của ViHAT > Webhook | OA Secret Key — không dán vào chat, tài liệu, manifest. Cùng nhóm `ZALO_OA_*`: thiếu ở prod → comms **không khởi động** |
| `KAFKA_EVENT_USERNAME` | không — kể cả prod (chủ dự án 17/10/2026) | `comms-secrets` | Tài khoản SASL của Kafka sự kiện. Có cả hai `USERNAME` + `PASSWORD` thì kết nối là **SASL qua TLS** theo `KAFKA_EVENT_SASL_MECHANISM` (mục 2); vắng cả hai thì kết nối **không mã hoá, không xác thực** (chỉ cho broker trong cụm) | tên tài khoản do người vận hành Kafka cấp. Chỉ một trong hai có giá trị → comms **không khởi động** |
| `KAFKA_EVENT_PASSWORD` | không — kể cả prod | `comms-secrets` | Mật khẩu đi cặp với `KAFKA_EVENT_USERNAME` | mật khẩu do người vận hành Kafka cấp |
| `MINI_APP_PROFILE_CODE` | **không** — kể cả dev. **Đặt ở prod thì platform từ chối khởi động** | `platform-secrets` **chỉ ở namespace `vigov-dev`** | Mã profile dev platform gắn vào liên kết QR Mini App (`pf=<mã>`) ở platform-admin (chi tiết xã) và web-admin (Cấu hình), để QR in từ cụm dev mở app với máy chủ dev (ADR 0046 §Sửa đổi 14/10/2026 #9). Để trống thì QR không mang `pf` (liên kết prod) | **cùng mã** với `VIGOV_DEV_PROFILE_CODE` trong `citizen-app/.env.qr.local` (người giữ profile dev cấp); 20–128 ký tự `A-Z a-z 0-9 - _`. Không dán vào chat hay tài liệu |

**Việc cần làm trên cụm đang chạy — `SESSION_SIGNING_KEYS`:** chỉ để ở `identity-secrets`. **Xoá key
này khỏi** `platform-secrets`, `documents-secrets`, `finance-secrets`, `petitions-secrets`,
`comms-secrets`, `reporting-secrets` (nếu có), rồi `rollout restart` các dịch vụ đó. Chỉ identity ký và đọc phiên cán bộ; ai cầm khoá
này **giả được phiên cán bộ của mọi xã**, nên mỗi bản sao thừa là thêm một chỗ để lộ.

### `harbor-vigov` — type `kubernetes.io/dockerconfigjson`

Server `harbor.omicrm.services`, tài khoản + mật khẩu Harbor. Cả 8 pod kéo ảnh bằng nó.

### TLS — type `kubernetes.io/tls`, key `tls.crt` · `tls.key`

| Môi trường | Web | API |
|---|---|---|
| dev | `vigov-dev-wildcard-tls` (`*.dev.vigov.vn`) | `vigov-api-wildcard-tls` — **cùng tên, cùng chứng chỉ** `*.api.vigov.vn` với prod, một bản sao trong `vigov-dev` (host `<dịch vụ>-dev.api.vigov.vn`) |
| prod | `vigov-wildcard-tls` (`*.vigov.vn`) | `vigov-api-wildcard-tls` (`*.api.vigov.vn`) |

Staging (`vigov-staging-tls`, `vigov-api-staging-tls`) đã bỏ 14/10/2026. Bàn điều khiển dev
`admin.dev.vigov.vn` khớp `*.dev.vigov.vn`: Ingress dựng tay của nó dùng `vigov-dev-wildcard-tls`.

## 2. ConfigMap `common-config` — dùng chung cho 7 pod Go

| Key | Bắt buộc | Là gì · lấy giá trị | Value |
|---|---|---|---|
| `ENV` | **có** — mọi dịch vụ | Môi trường chạy | `prod` (dev: `dev` — overlay sinh sẵn) |
| `TRUSTED_PROXY_CIDRS` | **có (prod)** — mọi dịch vụ | Dải IP Pod của ingress-nginx và web-admin — trạm được tin khi báo IP người dùng qua `X-Forwarded-For`; thiếu thì vết kiểm toán ghi IP pod thay vì IP người thao tác. Cách lấy: ngay dưới bảng | dải IP pod ingress-nginx và web-admin, vd `10.42.0.0/16` |
| `CITIZEN_CORS_ALLOWED_ORIGINS` | **có (prod)** — identity, petitions, comms | Miền Zalo Mini App được gọi API công dân (CORS). Chép đúng giá trị bên phải | `https://h5.zdn.vn,https://zalo.me,https://*.zdn.vn,https://*.zalo.me` |
| `CITIZEN_SESSION_TTL` | không — mặc định `720h` | Thời hạn phiên công dân | `720h` |
| `OBJECT_STORAGE_ENDPOINT` | **có (prod)** — petitions, comms, platform | MinIO nội bộ lưu tệp đính kèm, ảnh bìa tin, logo/banner xã (ADR 0052, ADR 0069). Người vận hành MinIO | `https://<minio nội bộ>:<cổng>` — đúng một host |
| `OBJECT_STORAGE_PUBLIC_ENDPOINT` | **có (prod)** — petitions, comms, platform | **Cổng S3 API** của CÙNG MinIO với `OBJECT_STORAGE_ENDPOINT` mà điện thoại thấy. Từ 09/10/2026 **chỉ** dùng để ký đường dẫn **xem/tải về** (presigned GET) — thiết bị **không còn tải lên** vào đây, mọi lượt tải lên đi qua dịch vụ (ADR 0052 §Sửa đổi 09/10/2026). **Không phải MinIO Console** (giao diện web): Console không phục vụ S3, đường dẫn ký cho nó không mở ra tệp (09/10/2026 nó đã trả 200 cho lệnh tải lên mà không lưu gì). Không có địa chỉ nội bộ riêng thì đặt bằng `OBJECT_STORAGE_ENDPOINT` | `https://storage-api.vigov.vn` (prod) |
| `OBJECT_STORAGE_PUBLIC_MEDIA_BASE_URL` | **có (prod)** — comms (từ 01/10/2026), platform (từ 02/10/2026) | URL gốc bucket media công khai — `image_url` của tin Mini App và `logo_public_url` / `web_admin_banner_public_url` của xã dựng từ đây. Thiếu thì comms và platform **không khởi động** ở prod. Người vận hành MinIO: tạo bucket `<tiền tố>-public` trong MinIO đang có, chỉ `GetObject` ẩn danh — lệnh ở mục 6, "Bucket media công khai" | `<OBJECT_STORAGE_PUBLIC_ENDPOINT>/vigov-prod-public` — **không** phải `OBJECT_STORAGE_ENDPOINT` |
| `OBJECT_STORAGE_REGION` | không — mặc định `us-east-1` | Region của MinIO | `us-east-1` (phải trùng region của MinIO) |
| `OBJECT_STORAGE_BUCKET_PREFIX` | **có (prod)** — petitions, comms, platform | Tiền tố bucket: `<tiền tố>-private` · `-public` · `-temp` | `vigov-prod` |
| `MALWARE_SCANNER_ADDRESS` | **có (prod)** — petitions, comms, platform | clamd quét mã độc tệp tải lên (ADR 0052 §9). Tên Service ClamAV + `3310`, không `tcp://`. Dựng clamd và đặt key: mục 6, việc `dung-clamav` | `vigov-clamav:3310` (Service của [`deploy/cluster/clamav.yaml`](../cluster/clamav.yaml)) |
| `ZALO_OA_APP_ID` | **có (prod)** — comms | App ID của Zalo App **dùng chung** cả nền tảng mà mỗi xã uỷ quyền OA của mình cho (ADR 0098 #2) — một giá trị cho mọi xã, không phải app riêng từng xã (ADR 0066). Lấy ở developers.zalo.me > app của ViHAT > dòng "ID ứng dụng". Không phải bí mật: nó nằm trong link uỷ quyền admin OA mở | dãy số, 1–32 chữ số. Sai dạng → comms **không khởi động**. Đi cùng `ZALO_OA_APP_SECRET` (mục 1) và `ZALO_OA_CALLBACK_HOST` (mục 3) |
| `KAFKA_EVENT_ADDRESS` | **có (prod)** — comms | Kafka chở sự kiện nghiệp vụ (ADR 0010): comms chuyển mọi sự kiện webhook OA Zalo lên đây (topic `<ENV>.comms.zalo_oa_event_received.v1` — dev và prod dùng chung một cụm nên MỌI topic và consumer group mang tiền tố `ENV`, vd. `prod.comms.zalo_oa_event_received.v1`; khoá = xã). Thiếu ở dev thì webhook OA trả 503. **Topic phải được tạo sẵn** — comms không tự tạo (số partition, thời gian giữ là quyết định vận hành; topic chứa dữ liệu cá nhân, Nghị định 13). Người vận hành Kafka | `<broker 1>:9092,<broker 2>:9092` — đủ mọi broker, không `tcp://`. Cổng khác 9092 thì phải mở thêm ở NetworkPolicy `allow-egress` |
| `KAFKA_EVENT_SASL_MECHANISM` | không — chỉ cần khi có `KAFKA_EVENT_USERNAME/PASSWORD` | Cơ chế SASL của broker | `PLAIN` · `SCRAM-SHA-256` · `SCRAM-SHA-512`. Có tài khoản mà trống, hoặc có mà không có tài khoản → comms **không khởi động** |

**Lấy `TRUSTED_PROXY_CIDRS`:**

```sh
# dải Pod của từng node — gộp lại, RKE2/k3s thường là 10.42.0.0/16
kubectl get nodes -o jsonpath='{range .items[*]}{.spec.podCIDR}{"\n"}{end}'
# IP pod ingress-nginx (RKE2: namespace kube-system) và web-admin — phải nằm trong dải trên
kubectl get pods -A -o wide | grep -E 'ingress|web-admin'
```

ingress-nginx chạy `hostNetwork` thì thêm dải IP node (`kubectl get nodes -o wide`). **Không bao giờ**
`0.0.0.0/0` hay `::/0` — pod từ chối khởi động: tin mọi địa chỉ là tin `X-Forwarded-For` giả của bất kỳ ai.

### Key trong `common-config` mà web-admin cũng đọc — `BASEMAP_URL`

Bảng riêng vì web-admin không có `config.Uses` nên không suy được nó là người đọc từ `core/config`;
người đọc là dịch vụ Go thì suy như mọi key khác — `BASEMAP_URL` có cả hai: web-admin (bản đồ phản ánh, `/ban-do`) và comms (nhóm `config.Basemap`, nền bản đồ Mini App, ADR 0072
§Sửa đổi 11/10/2026 (Mini App) M1). Key viết **gạch dưới**, tới cả hai pod qua `envFrom` (luật 11 bất biến 4,
ADR 0093) — không có bước ánh xạ riêng nào.

| Key | Bắt buộc | Là gì · lấy giá trị | Value |
|---|---|---|---|
| `BASEMAP_URL` | không — kể cả prod; thiếu thì bản đồ phản ánh và `/ban-do` của web-admin **không có nền** + một câu, và tuyến công khai `GET /api/v1/commune-basemap/*` của comms trả 503 "Chưa cấu hình bản đồ nền." (bản đồ màn Gửi phản ánh của Mini App không có nền); cả hai pod vẫn chạy. **Sai dạng** (không https, có thông tin đăng nhập, có truy vấn) thì comms **không khởi động** (web-admin: không có nền) | Tiền tố trong MinIO chứa tệp nền PMTiles, glyph, sprite (ADR 0072 §Trả lời 09/10/2026; Mini App: §Sửa đổi 11/10/2026 (Mini App) M1). web-admin phát lại cùng gốc ở `/basemap/*`; comms phát lại cho Mini App (nhóm `config.Basemap`). Dựng, tải lên, kiểm chủ quyền: `kb/40-runbooks/basemap-pmtiles.md` — chỉ đặt sau khi kiểm có ký. Cổng phải khớp NetworkPolicy 6b (web-admin) và 7b (comms): 9000 | `https://<minio nội bộ>:9000/<tiền tố>-public/config/map/<YYYYMMDD>` — https, không thông tin đăng nhập, không truy vấn. **Không bao giờ** URL của OpenFreeMap hay host ngoài |

Đổi key xong: `kubectl -n <namespace> rollout restart deploy/vigov-web-admin deploy/vigov-service-comms`
(`envFrom` chỉ đọc lúc pod khởi động), hoặc chạy lại job `web-admin` và job của comms.

## 3. Env viết thẳng trong Deployment (không qua ConfigMap)

| Key | Bắt buộc | Deployment | Là gì · lấy giá trị | Value |
|---|---|---|---|---|
| `LISTEN_ADDR` | không — mặc định `:8080` | cả 7 | Cổng REST | `:8080` |
| `PLATFORM_GRPC_ADDR` | **có (prod)** — identity, documents, finance, petitions, comms, reporting | 6 dịch vụ trừ `platform` | gRPC của platform — phân giải tên miền ra xã. Tên Service + `9090` | `platform:9090` |
| `IDENTITY_GRPC_ADDR` | **có (prod)** — documents, finance, petitions, comms, reporting, platform (từ 02/10/2026) | 6 dịch vụ trừ `identity` (manifest platform đã đặt sẵn) | gRPC của identity — đổi phiên cán bộ thành người dùng. Tên Service + `9090`. Platform dùng lại biến này từ 02/10/2026 cho tuyến logo + banner web-admin của xã (ADR 0069, quyền `admin.org`); phiên vận hành vẫn đi cổng riêng, xem `IDENTITY_OPERATOR_GRPC_ADDR` | `identity:9090` |
| `IDENTITY_OPERATOR_GRPC_ADDR` | **có (prod)** — platform | `vigov-service-platform` **chỉ nơi này** | gRPC `OperatorService` của identity — tra / mở phiên vận hành (ADR 0048 §01/10 #2; cổng riêng chốt 01/10/2026). Cổng `9093`, **không** phải `9090`: 9090 nhận năm dịch vụ cán bộ, 9093 chỉ nhận platform (quy tắc 11 của `deploy/base/mang/netpol.yaml`). Danh sách `host:port[,host:port]` | `identity:9093` — khớp cổng `grpc-operator` của Deployment/Service identity |
| `PETITIONS_GRPC_ADDR` | **có (prod)** — identity | `vigov-service-identity` | gRPC của petitions — hỏi trước khi xoá mềm đơn vị. Tên Service + `9090` | `petitions:9090` |
| `DOCUMENTS_GRPC_ADDR` | **có (prod)** — identity, petitions | `vigov-service-identity`, `vigov-service-petitions` | gRPC của documents. identity: như trên. petitions: hỏi đơn thư trước khi chuyển thành nhiệm vụ (`ResolveCitizenLetterForTask`, ADR 0085) — thiếu ở dev thì `POST /api/v1/citizen-letter-tasks` trả 503. Tên Service + `9090` | `documents:9090` |
| `COMMS_GRPC_ADDR` | **có (prod)** — petitions, documents, finance, platform | `vigov-service-petitions`, `vigov-service-documents`, `vigov-service-finance`, `vigov-service-platform` | gRPC của comms — hộp nhắc việc của bộ chạy tự động hoá (ADR 0058); finance: thông báo nhắc tên trong trao đổi dự án (ADR 0081 #5); platform: màn Zalo Bot của khu vận hành (bot dùng chung, ADR 0074) — thiếu ở dev thì các tuyến `zalo-bots/shared…` trả 503. Tên Service + `9090` | `comms:9090` |
| `OPERATOR_HOST` | không — kể cả prod; vắng là khu vận hành **tắt**, mọi tuyến vận hành trả 404 (ADR 0048) | `vigov-service-platform` **chỉ nơi này** | Host duy nhất của khu vận hành ViHAT. Chỉ đặt khi host đã có DNS + TLS + Ingress, và sau khi xong còn mở #12 của ADR 0048 | tên host trần, chữ thường, không giao thức / cổng / đường dẫn, vd `admin.vigov.vn` (dev: `admin.dev.vigov.vn`). Host dạng xã dưới `vigov.vn` → platform **không khởi động** |
| `ZALO_BOT_WEBHOOK_HOST` | **có (prod)** — comms | `vigov-service-comms` **chỉ nơi này** | Host mà Zalo gửi tin của bot dùng chung tới; comms tự dựng `https://<host>/api/v1/zalo-bot-updates` khi trỏ webhook (ADR 0074 #5). Chỉ đặt khi host đã có DNS + TLS + Ingress vào comms | tên host trần, chữ thường, không giao thức / cổng / đường dẫn: `bot.api.vigov.vn` ở prod. Host dạng xã (`<xa>.vigov.vn`) → comms **không khởi động** |
| `ZALO_OA_CALLBACK_HOST` | **có (prod)** — comms | `vigov-service-comms` **chỉ nơi này** | Host mà Zalo gọi về sau khi admin OA của xã bấm Cho phép; comms tự dựng `https://<host>/api/v1/zalo-oa-authorizations` — đó là **Callback URL duy nhất** phải khai vào app trên developers.zalo.me (Zalo chỉ cho một callback mỗi app, nên host là của nền tảng, xã tìm bằng `state`, ADR 0098 #3, #4). Chỉ đặt khi host đã có DNS + TLS + Ingress vào comms | tên host trần, chữ thường, không giao thức / cổng / đường dẫn. **Được phép trùng** `ZALO_BOT_WEBHOOK_HOST` (`bot.api.vigov.vn` ở prod) — hai tên vì hai vai trò (luật 11 #3). Host dạng xã (`<xa>.vigov.vn`) → comms **không khởi động** |
| `CITIZEN_SESSION_BRIDGE_LISTEN_ADDR` | **có (prod)** — identity | `vigov-service-identity` **chỉ nơi này** | Cổng cầu phiên công dân, chỉ `vihat-miniapp` gọi (ADR 0045). Chốt `9091` ngày 30/09/2026; manifest `deploy/base/identity` đã đặt sẵn | `:9091` — khớp cổng `bridge` của Deployment/Service identity và quy tắc 8 của `deploy/base/mang/netpol.yaml`; phía `vihat-miniapp` đặt `VIGOV_CITIZEN_SESSION_BRIDGE_ADDRESS=identity:9091`. Chỉ đặt khi đã có `CITIZEN_SESSION_BRIDGE_KEYS`: có một mà thiếu cái kia thì pod **không khởi động** — vì vậy **không** đặt vào `common-config` |

Tên host là tên Service trên cụm (`kubectl -n vigov-prod get svc`); Service petitions/documents/comms
phải mở cổng `9090` (comms: đích `DeliverStaffNotifications` của bộ chạy tự động hoá, ADR 0058).

## 4. Không đặt

| Key | Bắt buộc | Vì sao |
|---|---|---|
| `GRPC_LISTEN_ADDR` | không — mặc định `:9090` | mặc định `:9090` |
| `OPERATOR_GRPC_LISTEN_ADDR` | không — mặc định `:9093` | mặc định `:9093` — cổng `OperatorService` của identity, khớp cổng `grpc-operator` và quy tắc 11 của netpol |
| `TENANT_CACHE_TTL` | không — mặc định `30s` | mặc định `30s` |
| `RABBITMQ_DSN` | không — chưa dịch vụ nào dùng | chưa dùng |
| `RABBITMQ_EXCHANGE` | không — chưa dịch vụ nào dùng | chưa dùng |
| `ELASTICSEARCH_ADDRS` | không — chưa dịch vụ nào dùng | chưa dùng |
| `ELASTICSEARCH_API_KEY` | không — chưa dịch vụ nào dùng | chưa dùng |
| `ELASTICSEARCH_INDEX_PREFIX` | không — chưa dịch vụ nào dùng | chưa dùng |
| `DANGEROUS_AUTH_BYPASS` | không | **không bao giờ đặt** — `ENV=prod` từ chối khởi động |

`tools/check_env_map.py` (trong `make check`) đối chiếu các bảng trên với `core/config` và với
`configUses` của từng `service-*/cmd/server/main.go`: thêm biến mà quên ghi ở đây, hoặc cột Bắt buộc
ghi sai dịch vụ, là đỏ. Vì sao từng biến nằm ở Secret hay ConfigMap: `deploy/README.md` mục 3.

## 5. Tên miền — việc của người vận hành

Mô hình host (prod + dev từ 14/10/2026), nhãn dành riêng (`admin` · `admin-dev` · `admin-stg` · `api` ·
`api-stg` · `dev` · `stg` · `www`; nhãn `stg` giữ dành riêng dù staging đã bỏ), vì sao chúng không chồng nhau và bẫy DNS nút trung gian rỗng: **ADR 0046**
(`kb/10-decisions/0046-quy-hoach-ten-mien-va-quan-tri-dau-tien-cua-xa.md`). Ở đây chỉ có việc phải làm.

| Việc | Ở đâu |
|---|---|
| 3 bản ghi DNS wildcard (`*.vigov.vn`, `*.api.vigov.vn`, `*.dev.vigov.vn`) + 3 chứng chỉ wildcard (2 Secret TLS mỗi môi trường, tên ở mục 1) | ngoài kho |
| Host của Ingress | `deploy/base/mang/ingress.yaml` + `overlays/<mt>/ingress-moi-truong.yaml` — **SINH RA** (`go run ./tools/ingress`). Cụm thật dựng tay trong Rancher (`deploy/README.md` đầu tệp) thì chép host theo hai tệp ấy, không theo trí nhớ |
| Tên miền của một xã | một hàng `tenant_domain` của `platform` — không bao giờ một nhãn dành riêng, không bao giờ dưới `.api.vigov.vn` hay `.api-stg.vigov.vn` (platform từ chối) |

**Thứ tự đổi tên miền Xã Thăng Bình** (lịch sử — viết cho mô hình 26/09/2026, thời còn staging) — sai thứ tự là xã mất đăng nhập hoặc bước 2 không chạy được:

1. DNS + TLS + Ingress cho `thangbinh-danang.vigov.vn` và `thangbinh-danang.stg.vigov.vn` chạy thật.
2. Job `vigov-deploy`, việc `doi-ten-mien-thang-binh` (`MT=prod`, `XAC_NHAN=vigov-prod`).
3. Job `service-platform` (migration 0007). Sau 0007 mọi `UPDATE` dòng `admin.vigov.vn` bị CHECK
   từ chối, nên bước 2 **tự dừng, không ghi gì** nếu 0007 đã chạy.

## 6. Đăng nhập app riêng của xã (ADR 0066) — thứ tự đưa lên cụm

Mọi bước là job `vigov-deploy` (`deploy/Jenkinsfile`, cùng thứ tự ở khối chú thích trước stage
"Tạo khoá mã hoá identity" — sửa một thì sửa cả hai), trừ bước 4 và bước cuối. Key và dạng giá trị:
bảng mục 1 và mục 3.

Từ 14/10/2026 mỗi bước chạy **riêng cho từng `MT`** (`dev`, `prod`; `XAC_NHAN` = namespace): hai namespace,
hai CSDL, hai bộ khoá. Job **không bao giờ** chép khoá giữa hai namespace.

1. `tao-khoa-ma-hoa-identity` (`MT`) — **sinh riêng** `SECRET_ENCRYPTION_KEYS` vào `identity-secrets` của
   namespace ấy. Đã có thì **dừng, không đổi**. Không in giá trị — log in lệnh sao lưu; **sao lưu ngay**,
   tách khỏi bản sao lưu CSDL.
2. `mo-netpol-identity-zalo` (`MT`) — mở 443 cho pod
   identity **chỉ khi** đường ra của nó đang bị NetworkPolicy hạn chế; không thì không làm gì.
3. `bo-sung-cau-hinh-identity` (`MT`) — bốn biến ảnh
   identity mới đòi (thiếu là crash-loop, sự cố 01/10/2026); dạng và chỗ đặt: hàng
   `CITIZEN_SESSION_BRIDGE_KEYS` ở mục 1, hàng `PETITIONS_GRPC_ADDR` · `DOCUMENTS_GRPC_ADDR` ·
   `CITIZEN_SESSION_BRIDGE_LISTEN_ADDR` ở mục 3. Ba biến env **chỉ thêm khi chưa có**, có rồi thì giữ
   nguyên và in giá trị; tên Service petitions/documents không đúng thì dừng. Khoá: cùng bảng quyết định
   với bước 1 (namespace có thì giữ, chưa có thì sinh riêng). Đủ cả bốn mới
   khởi động lại identity của `MT` và đợi 3 phút; hỏng thì in 60 dòng log. Service petitions/documents
   thiếu cổng `9090` chỉ bị **cảnh báo**: identity chạy được, nhưng xoá mềm đơn vị hỏng tới khi người
   vận hành mở cổng ấy. Log in lệnh đọc khoá ra tệp để đặt vào Secret của `vihat-miniapp` — chỉ khoá của prod: `vihat-miniapp` chỉ chạy ở `vigov-prod`.
   Ảnh mới đã triển khai và đang crash-loop thì bước này tự đưa nó lên — không cần chạy lại bước 4.
4. Job `service-identity` — ảnh có migration 0021 và `/operatorctl`, khởi động lại pod để đọc khoá.
   Rồi `kiem-tra-dang-nhap-app-rieng` (chỉ đọc): key, ảnh, Ingress `identity.api.vigov.vn` (dev: `identity-dev.api.vigov.vn`), và
   `POST /api/v1/citizen-sessions` thân `{}` phải trả `400 invalid_body` (`404` = ảnh cũ hoặc Ingress sai).
5. Đặt App Secret của App ID xã (lấy ở developers.zalo.me → app → Cài đặt → App Secret): platform-admin,
   chi tiết xã, nút "Đặt/đổi khoá bí mật"; hoặc job `vigov-deploy`, việc `dat-secret-mini-app` (`MT`,
   `XAC_NHAN` = namespace, `TENANT_ID`, `APP_ID`, `TICKET`, `APP_SECRET` bắt buộc). Thiếu khoá thì
   `POST /api/v1/citizen-sessions` trả `422 app_not_ready`.
6. `cd citizen-app && npm run zmp:deploy -- --domain=thangbinh-danang.vigov.vn` — **không**
   `--demo`: danh tính demo đã gỡ 05/10/2026 (ADR 0066 §Sửa đổi 05/10/2026); **không** `--vao-thang`:
   bỏ 06/10/2026, `--domain` nay luôn nung tên miền (ADR 0047 §Sửa đổi 06/10/2026 #5). `deploy.mjs`
   từ chối cả hai cờ. Mọi cờ khác: `citizen-app/README.md` §Nộp lên Zalo.

### ClamAV cho petitions — TRƯỚC job `service-petitions`

Độc lập với sáu bước trên. Ảnh petitions có `config.Uses` **không khởi động** ở prod khi
`MALWARE_SCANNER_ADDRESS` trống (ADR 0052 §9, ADR 0057).

1. Job `vigov-deploy`, việc `dung-clamav` (`MT=dev`, rồi `MT=prod`; `XAC_NHAN` = namespace) — áp
   [`deploy/cluster/clamav.yaml`](../cluster/clamav.yaml) (Deployment + Service `vigov-clamav`, cổng
   3310, không Ingress), đợi sẵn sàng tối đa 10 phút (lần đầu tải chữ ký), thử `PING`→`PONG` qua
   Service, rồi đặt `MALWARE_SCANNER_ADDRESS=vigov-clamav:3310` vào `common-config` **chỉ khi chưa có**
   (có rồi thì giữ nguyên và in giá trị). Không khởi động lại dịch vụ nào. Hỏng thì in sự kiện + 80
   dòng log của pod clamd. RAM, đường ra `database.clamav.net:443`, giới hạn cỡ tệp: đầu tệp manifest.
2. Đặt đủ `OBJECT_STORAGE_*` (mục 1 và 2) — thiếu thì ảnh petitions mới cũng không khởi động.
3. Job `service-petitions`.

**comms cũng vậy từ 01/10/2026** (ảnh bìa tin Mini App): `config.Uses` của comms khai `ObjectStore`,
`PublicMedia`, `MalwareScan`, nên trước job `service-comms` phải có `MALWARE_SCANNER_ADDRESS`,
`OBJECT_STORAGE_ENDPOINT` · `_PUBLIC_ENDPOINT` · `_BUCKET_PREFIX` · `_PUBLIC_MEDIA_BASE_URL` trong
`common-config`, và `OBJECT_STORAGE_ACCESS_KEY` · `_SECRET_KEY` trong `comms-secrets` — thiếu một là
comms **từ chối khởi động** và gọi tên key. NetworkPolicy `allow-comms-egress-storage` (quy tắc 7b của
`deploy/base/mang/netpol.yaml`) mở 9000/3310 cho pod comms.

### Bucket media công khai — trong MinIO đang có, TRƯỚC job `service-comms`

Một MinIO chung cho mọi dịch vụ; trong đó **ba bucket** ghép từ `OBJECT_STORAGE_BUCKET_PREFIX`
(`core/storage/storage.go` `Bucket`, ADR 0052 §2). `-private` và `-temp` petitions đã dùng; ảnh bìa tin
cần thêm `-public`. Người dùng chốt 01/10/2026: **bucket riêng**, không mở đọc ẩn danh một tiền tố trong
bucket private — sai một ký tự trong policy ấy là lộ hồ sơ của mọi xã, và private bật versioning nên ảnh
đã gỡ đăng vẫn còn bản cũ.

```sh
# <alias> = alias mc của MinIO; <tiền tố> = OBJECT_STORAGE_BUCKET_PREFIX (prod: vigov-prod, dev: vigov-dev)
mc mb <alias>/<tiền tố>-public                       # versioning để TẮT (mặc định)
mc anonymous set download <alias>/<tiền tố>-public   # ẩn danh CHỈ GetObject, không liệt kê, không ghi
mc anonymous get <alias>/<tiền tố>-public            # phải in: download
mc anonymous get <alias>/<tiền tố>-private           # phải in: private — KHÔNG BAO GIỜ mở bucket này
```

Policy của user MinIO **riêng của comms** (cặp khoá trong `comms-secrets`) cần thêm, ngoài quyền ở
`<tiền tố>-private` / `-temp` trên `…/comms/*`:

```json
{ "Effect": "Allow",
  "Action": ["s3:PutObject", "s3:DeleteObject"],
  "Resource": ["arn:aws:s3:::<tiền tố>-public/public-media/*"] }
```

Rồi đặt trong `common-config`:

```
OBJECT_STORAGE_PUBLIC_MEDIA_BASE_URL = <giá trị OBJECT_STORAGE_PUBLIC_ENDPOINT>/<tiền tố>-public
```

Vì sao không lấy `OBJECT_STORAGE_ENDPOINT`: `image_url` = giá trị này + `/` + khoá
(`core/storage/storage.go` `PublicURL`), và URL ấy tới **thẳng điện thoại người dân** — phải ra được
Internet, phải `https` (webview Zalo chặn `http` trộn), và phải mang tên bucket vì khoá không có. Tách
biến riêng để ngày đặt CDN phía trước chỉ đổi giá trị, không đổi mã.

**Kiểm:** đăng một tin có ảnh bìa, mở `image_url` trong trình duyệt ẩn danh → thấy ảnh; bỏ `/public-media/…`
đi, mở `<base>/` → **phải bị từ chối** (không liệt kê). Host công khai còn phải nằm trong danh sách tên
miền cho phép của App ID trên console Zalo (ADR 0052, Còn mở #3) — thiếu thì trình duyệt thấy ảnh mà Zalo
không.
