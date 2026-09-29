# Secret và ConfigMap của ViGov

Tên đối tượng theo **cụm đang chạy** (dựng tay trong Rancher, namespace `vigov-prod`). Giá trị dưới
đây là **hình dạng**, anh tự điền giá trị thật. Key viết **gạch dưới** (`DATABASE_DSN`), không gạch
ngang — manifest dùng `envFrom`, key sai dạng bị bỏ qua im lặng.

Thêm hay sửa key xong: `kubectl -n vigov-prod rollout restart deploy/vigov-service-<dịch vụ>`.

(`deploy/base/` gọi các đối tượng này là `bi-mat-<dịch vụ>` và `cau-hinh-chung` — chỉ dùng khi dựng
cụm mới bằng `kubectl apply -k`.)

**Cột "Bắt buộc":** `có` — thiếu thì pod của các dịch vụ ghi sau dấu — **không khởi động**, mọi môi
trường. `có (prod)` — như vậy ở **staging và prod**; dev cho thiếu (tính năng tắt). `không` — có mặc
định hoặc tắt là đúng thiết kế. Dịch vụ không dùng một key thì **bỏ qua** key đó dù nó có trong
`common-config`.

**Trước khi rollout bản có `config.Uses` (29/09/2026)**: đặt đủ mọi key `có (prod)` bên dưới, nếu không
pod sẽ dừng và gọi tên key thiếu. Dễ sót nhất: `TRUSTED_PROXY_CIDRS`, `CITIZEN_SESSION_BRIDGE_*`,
`PETITIONS_GRPC_ADDR`/`DOCUMENTS_GRPC_ADDR`, `OBJECT_STORAGE_*`, `MALWARE_SCANNER_ADDRESS`,
`SECRET_ENCRYPTION_KEYS`.

## 1. Secret

### `<dịch vụ>-secrets` — type `Opaque`, mỗi dịch vụ một cái

`platform-secrets` · `identity-secrets` · `documents-secrets` · `finance-secrets` ·
`petitions-secrets` · `comms-secrets` · `reporting-secrets`

**Key chung, cả 7 Secret:**

| Key | Bắt buộc | Value |
|---|---|---|
| `DATABASE_DSN` | **có** — mọi dịch vụ | `postgres://<user>:<mật khẩu>@<host>:5432/vigov_<dịch vụ>?sslmode=require` — mỗi dịch vụ một CSDL riêng |
| `GRPC_CALLER_KEY` | **có** — platform, identity, documents, finance, petitions, comms, reporting | `openssl rand -base64 48` — **cùng một giá trị** ở cả 7 |
| `REDIS_DSN` | **có (prod)** — identity, documents, finance, petitions, comms | `redis://:<mật khẩu>@<host>:6379/0` — 5 Secret, **không** có ở `platform-secrets` và `reporting-secrets` |

**Key thêm, chỉ ở một Secret:**

| Key | Bắt buộc | Secret | Value |
|---|---|---|---|
| `SESSION_SIGNING_KEYS` | **có (prod)** — identity | `identity-secrets` **chỉ nơi này** | `<khoá mới>,<khoá cũ>` (mỗi khoá `openssl rand -base64 48`) |
| `CITIZEN_SESSION_BRIDGE_KEYS` | **có (prod)** — identity | `identity-secrets` | `<khoá mới>,<khoá cũ>`, mỗi khoá ≥ 32 byte (`openssl rand -base64 48`). Cùng giá trị ở Secret của `vihat-miniapp`. Khác `GRPC_CALLER_KEY` |
| `IDENTITY_ADMIN_SEED_PASSWORD` | không — kể cả prod | `identity-secrets` | mật khẩu ≥ 12 ký tự — tài khoản `admin` đầu tiên của xã mới. Gỡ key khi mọi xã đã đổi mật khẩu |
| `OPERATOR_SESSION_SIGNING_KEYS` | không — chưa dịch vụ nào dùng (ADR 0048 chưa nối) | `identity-secrets` | `<khoá mới>,<khoá cũ>`, mỗi khoá ≥ 32 byte. Khác mọi khoá của `SESSION_SIGNING_KEYS` |
| `OPERATOR_TOTP_ENCRYPTION_KEY` | không — như trên | `identity-secrets` | `<khoá mới>,<khoá cũ>`, mỗi khoá `openssl rand -base64 32` |
| `SECRET_ENCRYPTION_KEYS` | **có (prod)** — comms | `comms-secrets` | `<khoá mới>,<khoá cũ>`, mỗi khoá `openssl rand -base64 32`. **Sao lưu riêng trước khi lưu mật khẩu thư đầu tiên** — mất là mất hết |
| `OBJECT_STORAGE_ACCESS_KEY` | **có (prod)** — petitions | `petitions-secrets` | access key MinIO riêng của dịch vụ |
| `OBJECT_STORAGE_SECRET_KEY` | **có (prod)** — petitions | `petitions-secrets` | secret key đi cặp |

**Việc cần làm trên cụm đang chạy — `SESSION_SIGNING_KEYS`:** chỉ để ở `identity-secrets`. **Xoá key
này khỏi** `platform-secrets`, `documents-secrets`, `finance-secrets`, `petitions-secrets`,
`comms-secrets`, `reporting-secrets` (nếu có), rồi `rollout restart` các dịch vụ đó. Chỉ identity ký và đọc phiên cán bộ; ai cầm khoá
này **giả được phiên cán bộ của mọi xã**, nên mỗi bản sao thừa là thêm một chỗ để lộ.

### `harbor-vigov` — type `kubernetes.io/dockerconfigjson`

Server `harbor.omicrm.services`, tài khoản + mật khẩu Harbor. Cả 8 pod kéo ảnh bằng nó.

### TLS — type `kubernetes.io/tls`, key `tls.crt` · `tls.key`

| Môi trường | Web | API |
|---|---|---|
| staging | `vigov-staging-tls` (`*.stg.vigov.vn`) | `vigov-api-staging-tls` (`*.api-stg.vigov.vn`) |
| prod | `vigov-wildcard-tls` (`*.vigov.vn`) | `vigov-api-wildcard-tls` (`*.api.vigov.vn`) |

## 2. ConfigMap `common-config` — dùng chung cho 7 pod Go

| Key | Bắt buộc | Value |
|---|---|---|
| `ENV` | **có** — mọi dịch vụ | `prod` (staging: `staging`) |
| `TRUSTED_PROXY_CIDRS` | **có (prod)** — mọi dịch vụ | dải IP pod ingress-nginx và web-admin, vd `10.42.0.0/16` |
| `CITIZEN_CORS_ALLOWED_ORIGINS` | **có (prod)** — identity, petitions, comms | `https://h5.zdn.vn,https://zalo.me,https://*.zdn.vn,https://*.zalo.me` |
| `CITIZEN_SESSION_TTL` | không — mặc định `720h` | `720h` |
| `OBJECT_STORAGE_ENDPOINT` | **có (prod)** — petitions | `https://<minio nội bộ>:<cổng>` — đúng một host |
| `OBJECT_STORAGE_PUBLIC_ENDPOINT` | **có (prod)** — petitions | `https://<minio trình duyệt thấy>` |
| `OBJECT_STORAGE_PUBLIC_MEDIA_BASE_URL` | không — chưa dịch vụ nào dùng | `https://<host media>/vigov-prod-public` |
| `OBJECT_STORAGE_REGION` | không — mặc định `us-east-1` | `us-east-1` (phải trùng region của MinIO) |
| `OBJECT_STORAGE_BUCKET_PREFIX` | **có (prod)** — petitions | `vigov-prod` |
| `MALWARE_SCANNER_ADDRESS` | **có (prod)** — petitions | `clamav:3310` |

## 3. Env viết thẳng trong Deployment (không qua ConfigMap)

| Key | Bắt buộc | Deployment | Value |
|---|---|---|---|
| `LISTEN_ADDR` | không — mặc định `:8080` | cả 7 | `:8080` |
| `PLATFORM_GRPC_ADDR` | **có (prod)** — identity, documents, finance, petitions, comms, reporting | 6 dịch vụ trừ `platform` | `platform:9090` |
| `IDENTITY_GRPC_ADDR` | **có (prod)** — documents, finance, petitions, comms, reporting | 5 dịch vụ trừ `identity` và `platform` | `identity:9090` |
| `PETITIONS_GRPC_ADDR` | **có (prod)** — identity | `vigov-service-identity` | `petitions:9090` |
| `DOCUMENTS_GRPC_ADDR` | **có (prod)** — identity | `vigov-service-identity` | `documents:9090` |
| `CITIZEN_SESSION_BRIDGE_LISTEN_ADDR` | **có (prod)** — identity | `vigov-service-identity` **chỉ nơi này** | `:<cổng>` — cổng riêng, khác `9090`, phải khớp quy tắc NetworkPolicy (chưa có trong `deploy/base/mang/netpol.yaml`). Chỉ đặt khi đã có `CITIZEN_SESSION_BRIDGE_KEYS`: có một mà thiếu cái kia thì pod **không khởi động** — vì vậy **không** đặt vào `common-config` |

Tên host là tên Service trên cụm (`kubectl -n vigov-prod get svc`); Service petitions/documents phải
mở cổng `9090`.

## 4. Không đặt

| Key | Bắt buộc | Vì sao |
|---|---|---|
| `GRPC_LISTEN_ADDR` | không — mặc định `:9090` | mặc định `:9090` |
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

Mô hình bốn dạng host, nhãn dành riêng (`admin` · `admin-stg` · `api` · `api-stg` · `stg` ·
`www`), vì sao chúng không chồng nhau và bẫy DNS nút trung gian rỗng: **ADR 0046**
(`kb/10-decisions/0046-quy-hoach-ten-mien-va-quan-tri-dau-tien-cua-xa.md`). Ở đây chỉ có việc phải làm.

| Việc | Ở đâu |
|---|---|
| 4 bản ghi DNS wildcard + 4 chứng chỉ wildcard (2 Secret TLS mỗi môi trường, tên ở mục 1) | ngoài kho |
| Host của Ingress | `deploy/base/mang/ingress.yaml` + `overlays/<mt>/ingress-moi-truong.yaml` — **SINH RA** (`go run ./tools/ingress`). Cụm thật dựng tay trong Rancher (`deploy/README.md` đầu tệp) thì chép host theo hai tệp ấy, không theo trí nhớ |
| Tên miền của một xã | một hàng `tenant_domain` của `platform` — không bao giờ một nhãn dành riêng, không bao giờ dưới `.api(-stg).vigov.vn` (platform từ chối) |

**Thứ tự đổi tên miền Xã Thăng Bình** — sai thứ tự là xã mất đăng nhập hoặc bước 2 không chạy được:

1. DNS + TLS + Ingress cho `thangbinh-danang.vigov.vn` và `thangbinh-danang.stg.vigov.vn` chạy thật.
2. Job `vigov-deploy`, việc `doi-ten-mien-thang-binh` (`MT=prod`, `XAC_NHAN=vigov-prod`).
3. Job `service-platform` (migration 0007). Sau 0007 mọi `UPDATE` dòng `admin.vigov.vn` bị CHECK
   từ chối, nên bước 2 **tự dừng, không ghi gì** nếu 0007 đã chạy.
