# Secret và ConfigMap của ViGov

Tên đối tượng theo **cụm đang chạy** (dựng tay trong Rancher, namespace `vigov-prod`). Giá trị dưới
đây là **hình dạng**, anh tự điền giá trị thật. Key viết **gạch dưới** (`DATABASE_DSN`), không gạch
ngang — manifest dùng `envFrom`, key sai dạng bị bỏ qua im lặng.

Thêm hay sửa key xong: `kubectl -n vigov-prod rollout restart deploy/vigov-service-<dịch vụ>`.

(`deploy/base/` đọc đúng các tên này qua `envFrom` — `core/config/hints_test.go` kiểm.)

**Pod dừng vì thiếu biến?** Chính dòng log đã in, cho từng biến thiếu: nó là gì, lấy giá trị thế nào,
đặt vào đối tượng nào, dạng giá trị ra sao. Nguồn của đoạn chữ ấy là `core/config/hints.go`; cột
"Là gì · lấy giá trị" dưới đây là bản tóm tắt của cùng bảng đó — lệch nhau thì `hints.go` đúng.

**Cột "Bắt buộc":** `có` — thiếu thì pod của các dịch vụ ghi sau dấu — **không khởi động**, mọi môi
trường. `có (prod)` — như vậy ở **staging và prod**; dev cho thiếu (tính năng tắt). `không` — có mặc
định hoặc tắt là đúng thiết kế. Dịch vụ không dùng một key thì **bỏ qua** key đó dù nó có trong
`common-config`.

**Trước khi rollout bản có `config.Uses` (29/09/2026)**: đặt đủ mọi key `có (prod)` bên dưới, nếu không
pod sẽ dừng và gọi tên key thiếu. Dễ sót nhất: `TRUSTED_PROXY_CIDRS`, `CITIZEN_SESSION_BRIDGE_*`,
`PETITIONS_GRPC_ADDR`/`DOCUMENTS_GRPC_ADDR`, `COMMS_GRPC_ADDR`, `OBJECT_STORAGE_*`, `MALWARE_SCANNER_ADDRESS`,
`SECRET_ENCRYPTION_KEYS`.

## 1. Secret

### `<dịch vụ>-secrets` — type `Opaque`, mỗi dịch vụ một cái

`platform-secrets` · `identity-secrets` · `documents-secrets` · `finance-secrets` ·
`petitions-secrets` · `comms-secrets` · `reporting-secrets`

**Key chung, cả 7 Secret:**

| Key | Bắt buộc | Là gì · lấy giá trị | Value |
|---|---|---|---|
| `DATABASE_DSN` | **có** — mọi dịch vụ | Kết nối PostgreSQL tới CSDL riêng của dịch vụ. Người quản trị PostgreSQL cấp user + mật khẩu | `postgres://<user>:<mật khẩu>@<host>:5432/vigov_<dịch vụ>?sslmode=require` — mỗi dịch vụ một CSDL riêng |
| `GRPC_CALLER_KEY` | **có** — platform, identity, documents, finance, petitions, comms, reporting | Khoá xác thực lời gọi gRPC giữa các dịch vụ (ADR 0025). Sinh một lần, dùng lại ở cả 7 | `openssl rand -base64 48` — **cùng một giá trị** ở cả 7 |
| `REDIS_DSN` | **có (prod)** — identity, documents, finance, petitions, comms | Redis chống gửi trùng + giới hạn tần suất. Người vận hành Redis cấp | `redis://:<mật khẩu>@<host>:6379/0` — 5 Secret, **không** có ở `platform-secrets` và `reporting-secrets` |

**Key thêm, chỉ ở một Secret:**

| Key | Bắt buộc | Secret | Là gì · lấy giá trị | Value |
|---|---|---|---|---|
| `SESSION_SIGNING_KEYS` | **có (prod)** — identity | `identity-secrets` **chỉ nơi này** | Khoá ký phiên đăng nhập cán bộ. Tự sinh | `<khoá mới>,<khoá cũ>` (mỗi khoá `openssl rand -base64 48`) |
| `CITIZEN_SESSION_BRIDGE_KEYS` | **có (prod)** — identity | `identity-secrets` | Khoá backend `vihat-miniapp` gửi khi đổi phiên công dân (ADR 0045). Tự sinh, đặt cùng giá trị ở hai bên | `<khoá mới>,<khoá cũ>`, mỗi khoá ≥ 32 byte (`openssl rand -base64 48`). Cùng giá trị ở Secret của `vihat-miniapp`. Khác `GRPC_CALLER_KEY` |
| `IDENTITY_ADMIN_SEED_PASSWORD` | không — kể cả prod | `identity-secrets` | Công tắc một lần tạo `admin` cho xã mới | mật khẩu ≥ 12 ký tự — tài khoản `admin` đầu tiên của xã mới. Gỡ key khi mọi xã đã đổi mật khẩu |
| `OPERATOR_SESSION_SIGNING_KEYS` | **có (prod)** — identity (bên kiểm chữ ký chưa nối, ADR 0048 bước 2) | `identity-secrets` · `platform-secrets` — **cùng một giá trị** | Khoá ký phiên nhà vận hành: identity ký, platform kiểm chữ ký (ADR 0048 §01/10 #2). Tự sinh | `<khoá mới>,<khoá cũ>`, mỗi khoá ≥ 32 byte. Khác mọi khoá của `SESSION_SIGNING_KEYS` |
| `OPERATOR_TOTP_ENCRYPTION_KEY` | **có (prod)** — identity | `identity-secrets` | Khoá mã hoá bí mật TOTP nhà vận hành. Tự sinh | `<khoá mới>,<khoá cũ>`, mỗi khoá `openssl rand -base64 32` |
| `SECRET_ENCRYPTION_KEYS` | **có (prod)** — comms, identity | `comms-secrets` · `identity-secrets` | Khoá mã hoá bí mật riêng của từng xã trong CSDL (ADR 0009): comms niêm mật khẩu máy chủ thư, identity niêm secret Zalo của app riêng từng xã (ADR 0066). Tự sinh | `<khoá mới>,<khoá cũ>`, mỗi khoá `openssl rand -base64 32` — **mỗi Secret một giá trị riêng** (hai CSDL, hai bộ khoá dữ liệu). **Sao lưu riêng trước khi lưu bí mật đầu tiên** — mất là mất hết |
| `OBJECT_STORAGE_ACCESS_KEY` | **có (prod)** — petitions | `petitions-secrets` | Cặp khoá MinIO riêng của dịch vụ (ADR 0052 §3). Tạo user trong MinIO (`mc admin user add`), policy chỉ bucket của môi trường | access key MinIO riêng của dịch vụ |
| `OBJECT_STORAGE_SECRET_KEY` | **có (prod)** — petitions | `petitions-secrets` | Nửa kia của cặp trên | secret key đi cặp |

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

| Key | Bắt buộc | Là gì · lấy giá trị | Value |
|---|---|---|---|
| `ENV` | **có** — mọi dịch vụ | Môi trường chạy | `prod` (staging: `staging`) |
| `TRUSTED_PROXY_CIDRS` | **có (prod)** — mọi dịch vụ | Dải IP Pod của ingress-nginx và web-admin — trạm được tin khi báo IP người dùng qua `X-Forwarded-For`; thiếu thì vết kiểm toán ghi IP pod thay vì IP người thao tác. Cách lấy: ngay dưới bảng | dải IP pod ingress-nginx và web-admin, vd `10.42.0.0/16` |
| `CITIZEN_CORS_ALLOWED_ORIGINS` | **có (prod)** — identity, petitions, comms | Miền Zalo Mini App được gọi API công dân (CORS). Chép đúng giá trị bên phải | `https://h5.zdn.vn,https://zalo.me,https://*.zdn.vn,https://*.zalo.me` |
| `CITIZEN_SESSION_TTL` | không — mặc định `720h` | Thời hạn phiên công dân | `720h` |
| `OBJECT_STORAGE_ENDPOINT` | **có (prod)** — petitions | MinIO nội bộ lưu tệp đính kèm (ADR 0052). Người vận hành MinIO | `https://<minio nội bộ>:<cổng>` — đúng một host |
| `OBJECT_STORAGE_PUBLIC_ENDPOINT` | **có (prod)** — petitions | MinIO trình duyệt thấy, nằm trong presigned URL. Người vận hành MinIO | `https://<minio trình duyệt thấy>` |
| `OBJECT_STORAGE_PUBLIC_MEDIA_BASE_URL` | không — chưa dịch vụ nào dùng | URL gốc bucket media công khai | `https://<host media>/vigov-prod-public` |
| `OBJECT_STORAGE_REGION` | không — mặc định `us-east-1` | Region của MinIO | `us-east-1` (phải trùng region của MinIO) |
| `OBJECT_STORAGE_BUCKET_PREFIX` | **có (prod)** — petitions | Tiền tố bucket: `<tiền tố>-private` · `-public` · `-temp` | `vigov-prod` |
| `MALWARE_SCANNER_ADDRESS` | **có (prod)** — petitions | clamd quét mã độc tệp tải lên (ADR 0052 §9). Tên Service ClamAV + `3310`, không `tcp://`. Dựng clamd và đặt key: mục 6, việc `dung-clamav` | `vigov-clamav:3310` (Service của [`deploy/cluster/clamav.yaml`](../cluster/clamav.yaml)) |

**Lấy `TRUSTED_PROXY_CIDRS`:**

```sh
# dải Pod của từng node — gộp lại, RKE2/k3s thường là 10.42.0.0/16
kubectl get nodes -o jsonpath='{range .items[*]}{.spec.podCIDR}{"\n"}{end}'
# IP pod ingress-nginx (RKE2: namespace kube-system) và web-admin — phải nằm trong dải trên
kubectl get pods -A -o wide | grep -E 'ingress|web-admin'
```

ingress-nginx chạy `hostNetwork` thì thêm dải IP node (`kubectl get nodes -o wide`). **Không bao giờ**
`0.0.0.0/0` hay `::/0` — pod từ chối khởi động: tin mọi địa chỉ là tin `X-Forwarded-For` giả của bất kỳ ai.

## 3. Env viết thẳng trong Deployment (không qua ConfigMap)

| Key | Bắt buộc | Deployment | Là gì · lấy giá trị | Value |
|---|---|---|---|---|
| `LISTEN_ADDR` | không — mặc định `:8080` | cả 7 | Cổng REST | `:8080` |
| `PLATFORM_GRPC_ADDR` | **có (prod)** — identity, documents, finance, petitions, comms, reporting | 6 dịch vụ trừ `platform` | gRPC của platform — phân giải tên miền ra xã. Tên Service + `9090` | `platform:9090` |
| `IDENTITY_GRPC_ADDR` | **có (prod)** — documents, finance, petitions, comms, reporting | 5 dịch vụ trừ `identity` và `platform` | gRPC của identity — đổi phiên cán bộ thành người dùng. Tên Service + `9090` | `identity:9090` |
| `PETITIONS_GRPC_ADDR` | **có (prod)** — identity | `vigov-service-identity` | gRPC của petitions — hỏi trước khi xoá mềm đơn vị. Tên Service + `9090` | `petitions:9090` |
| `DOCUMENTS_GRPC_ADDR` | **có (prod)** — identity | `vigov-service-identity` | gRPC của documents — như trên | `documents:9090` |
| `COMMS_GRPC_ADDR` | **có (prod)** — petitions, documents | `vigov-service-petitions`, `vigov-service-documents` | gRPC của comms — hộp nhắc việc của bộ chạy tự động hoá (ADR 0058). Tên Service + `9090` | `comms:9090` |
| `OPERATOR_HOST` | không — kể cả prod; vắng là khu vận hành **tắt**, mọi tuyến vận hành trả 404 (ADR 0048) | `vigov-service-platform` **chỉ nơi này** | Host duy nhất của khu vận hành ViHAT. Chỉ đặt khi host đã có DNS + TLS + Ingress, và sau khi xong còn mở #12 của ADR 0048 | tên host trần, chữ thường, không giao thức / cổng / đường dẫn, vd `admin.vigov.vn` (staging: `admin-stg.vigov.vn`). Host dạng xã dưới `vigov.vn` → platform **không khởi động** |
| `CITIZEN_SESSION_BRIDGE_LISTEN_ADDR` | **có (prod)** — identity | `vigov-service-identity` **chỉ nơi này** | Cổng cầu phiên công dân, chỉ `vihat-miniapp` gọi (ADR 0045). Chốt `9091` ngày 30/09/2026; manifest `deploy/base/identity` đã đặt sẵn | `:9091` — khớp cổng `bridge` của Deployment/Service identity và quy tắc 8 của `deploy/base/mang/netpol.yaml`; phía `vihat-miniapp` đặt `VIGOV_CITIZEN_SESSION_BRIDGE_ADDRESS=identity:9091`. Chỉ đặt khi đã có `CITIZEN_SESSION_BRIDGE_KEYS`: có một mà thiếu cái kia thì pod **không khởi động** — vì vậy **không** đặt vào `common-config` |

Tên host là tên Service trên cụm (`kubectl -n vigov-prod get svc`); Service petitions/documents/comms
phải mở cổng `9090` (comms: đích `DeliverStaffNotifications` của bộ chạy tự động hoá, ADR 0058).

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

## 6. Đăng nhập app riêng của xã (ADR 0066) — thứ tự đưa lên cụm

Mọi bước là job `vigov-deploy` (`deploy/Jenkinsfile`, cùng thứ tự ở khối chú thích trước stage
"Tạo khoá mã hoá identity" — sửa một thì sửa cả hai), trừ bước 4 và bước cuối. Key và dạng giá trị:
bảng mục 1 và mục 3.

1. `tao-khoa-ma-hoa-identity` (`MT=prod`, `XAC_NHAN=vigov-prod`) — sinh `SECRET_ENCRYPTION_KEYS` vào
   `identity-secrets` của `vigov-prod` và **cùng giá trị** vào `vigov-staging` (chung CSDL). Đã có ở
   prod thì **dừng, không đổi prod**; staging có giá trị khác thì dừng. Không in giá trị — log in lệnh
   sao lưu; **sao lưu ngay**, tách khỏi bản sao lưu CSDL.
2. `mo-netpol-identity-zalo` (`MT=prod`, rồi `MT=staging`; `XAC_NHAN` = namespace) — mở 443 cho pod
   identity **chỉ khi** đường ra của nó đang bị NetworkPolicy hạn chế; không thì không làm gì.
3. `bo-sung-cau-hinh-identity` (`MT=prod`, rồi `MT=staging`; `XAC_NHAN` = namespace) — bốn biến ảnh
   identity mới đòi (thiếu là crash-loop, sự cố 01/10/2026); dạng và chỗ đặt: hàng
   `CITIZEN_SESSION_BRIDGE_KEYS` ở mục 1, hàng `PETITIONS_GRPC_ADDR` · `DOCUMENTS_GRPC_ADDR` ·
   `CITIZEN_SESSION_BRIDGE_LISTEN_ADDR` ở mục 3. Ba biến env **chỉ thêm khi chưa có**, có rồi thì giữ
   nguyên và in giá trị; tên Service petitions/documents không đúng thì dừng. Khoá: cùng bảng quyết định
   với bước 1 (prod có thì giữ, staging khác thì dừng, chỉ lượt `MT=prod` sinh khoá). Đủ cả bốn mới
   khởi động lại identity của `MT` và đợi 3 phút; hỏng thì in 60 dòng log. Service petitions/documents
   thiếu cổng `9090` chỉ bị **cảnh báo**: identity chạy được, nhưng xoá mềm đơn vị hỏng tới khi người
   vận hành mở cổng ấy. Log in lệnh đọc khoá ra tệp để đặt vào Secret của `vihat-miniapp`.
   Ảnh mới đã triển khai và đang crash-loop thì bước này tự đưa nó lên — không cần chạy lại bước 4.
4. Job `service-identity` — ảnh có migration 0021 và `/operatorctl`, khởi động lại pod để đọc khoá.
   Rồi `kiem-tra-dang-nhap-app-rieng` (chỉ đọc): key, ảnh, Ingress `identity.api.vigov.vn`, và
   `POST /api/v1/citizen-sessions` thân `{}` phải trả `400 invalid_body` (`404` = ảnh cũ hoặc Ingress sai).
5. `bat-demo-mini-app` (`MT=prod`, `XAC_NHAN=vigov-prod`, `TENANT_ID`, `APP_ID`, `TICKET` bắt buộc) —
   `operatorctl mini-app-demo on` trong một pod một lượt từ ảnh identity đang chạy.
6. `cd citizen-app && npm run zmp:deploy -- --domain=thangbinh-danang.vigov.vn --vao-thang --demo`

Trước khi Zalo duyệt app: `tat-demo-mini-app` — danh tính demo mở phiên không xác minh số điện thoại.

### ClamAV cho petitions — TRƯỚC job `service-petitions`

Độc lập với sáu bước trên. Ảnh petitions có `config.Uses` **không khởi động** ở staging/prod khi
`MALWARE_SCANNER_ADDRESS` trống (ADR 0052 §9, ADR 0057).

1. Job `vigov-deploy`, việc `dung-clamav` (`MT=prod`, rồi `MT=staging`; `XAC_NHAN` = namespace) — áp
   [`deploy/cluster/clamav.yaml`](../cluster/clamav.yaml) (Deployment + Service `vigov-clamav`, cổng
   3310, không Ingress), đợi sẵn sàng tối đa 10 phút (lần đầu tải chữ ký), thử `PING`→`PONG` qua
   Service, rồi đặt `MALWARE_SCANNER_ADDRESS=vigov-clamav:3310` vào `common-config` **chỉ khi chưa có**
   (có rồi thì giữ nguyên và in giá trị). Không khởi động lại dịch vụ nào. Hỏng thì in sự kiện + 80
   dòng log của pod clamd. RAM, đường ra `database.clamav.net:443`, giới hạn cỡ tệp: đầu tệp manifest.
2. Đặt đủ `OBJECT_STORAGE_*` (mục 1 và 2) — thiếu thì ảnh petitions mới cũng không khởi động.
3. Job `service-petitions`.
