# ConfigMap và Secret của ViGov trên k8s

Làm cho `vigov-staging` trước, rồi lặp lại cho `vigov-prod`. Giá trị anh tự điền; **tên đối
tượng và tên key thì không được đổi** — manifest tham chiếu đúng những chuỗi này.

## 1. Tạo gì

| Loại | Tên | Key | Pod dùng |
|---|---|---|---|
| ConfigMap | `cau-hinh-chung` | `ENV` | cả 6 pod Go — **kustomize tự sinh, KHÔNG tạo tay** |
| Secret | `bi-mat-platform` | `DATABASE_DSN` · `GRPC_CALLER_KEY` · `SESSION_SIGNING_KEYS` | `platform` |
| Secret | `bi-mat-identity` · `bi-mat-documents` · `bi-mat-finance` · `bi-mat-petitions` · `bi-mat-comms` | 3 key trên **+ `REDIS_DSN`** | dịch vụ cùng tên |
| Secret `docker-registry` | `harbor-vigov` | (k8s tự đặt) | cả 7 pod |
| Secret `tls` — web | staging: **`vigov-staging-tls`** (`*.stg.vigov.vn`) · prod: **`vigov-wildcard-tls`** (`*.vigov.vn`) | `tls.crt` · `tls.key` | Ingress |
| Secret `tls` — API | staging: **`vigov-api-staging-tls`** (`*.api-stg.vigov.vn`) · prod: **`vigov-api-wildcard-tls`** (`*.api.vigov.vn`) | `tls.crt` · `tls.key` | Ingress |

`web-admin` không đọc biến nào — không có Secret riêng.

## 2. Giá trị từng key

| Key | Hình dạng | Giống nhau giữa 6 Secret? |
|---|---|---|
| `DATABASE_DSN` | `postgres://…@<host>:5432/<db>?sslmode=require` — nhiều node thì `h1:5432,h2:5432` | **KHÁC** — mỗi dịch vụ một CSDL + một tài khoản riêng |
| `GRPC_CALLER_KEY` | chuỗi từ `openssl rand -base64 48` | **GIỐNG** — sinh một lần, dán vào cả 6 |
| `SESSION_SIGNING_KEYS` | `<khoa-moi>,<khoa-cu>` — prod cần ≥ 2 khoá, khoá mới đứng trước | **GIỐNG** |
| `REDIS_DSN` | `redis://…@<host>:6379/0` | giống được |
| `ENV` | `staging` / `prod` — sửa ở `overlays/<mt>/kustomization.yaml` | — |

⚠ **Key viết GẠCH DƯỚI** (`DATABASE_DSN`), không gạch ngang. Manifest dùng `envFrom`: key sai
dạng bị k8s bỏ qua im lặng, pod chết với "thiếu biến môi trường bắt buộc".

⚠ **Hai dịch vụ chung một CSDL không báo lỗi gì** nhưng ghi chung một bảng `audit_log`. Sáu DSN
phải trỏ sáu CSDL khác nhau.

## 3. Lệnh

```sh
NS=vigov-staging          # rồi vigov-prod
TLS=vigov-staging-tls           # prod: vigov-wildcard-tls
TLS_API=vigov-api-staging-tls   # prod: vigov-api-wildcard-tls

kubectl -n $NS create secret docker-registry harbor-vigov \
  --docker-server=harbor.omicrm.services --docker-username='<...>' --docker-password='<...>'

kubectl -n $NS create secret tls $TLS     --cert=<fullchain web.pem> --key=<privkey web.pem>
kubectl -n $NS create secret tls $TLS_API --cert=<fullchain api.pem> --key=<privkey api.pem>

kubectl -n $NS create secret generic bi-mat-platform \
  --from-literal=DATABASE_DSN='<dsn của vigov_platform>' \
  --from-literal=GRPC_CALLER_KEY='<khoá chung>' \
  --from-literal=SESSION_SIGNING_KEYS='<khoa-moi>,<khoa-cu>'

for s in identity documents finance petitions comms; do
  kubectl -n $NS create secret generic bi-mat-$s \
    --from-literal=DATABASE_DSN="<dsn của vigov_$s>" \
    --from-literal=GRPC_CALLER_KEY='<khoá chung>' \
    --from-literal=SESSION_SIGNING_KEYS='<khoa-moi>,<khoa-cu>' \
    --from-literal=REDIS_DSN='<dsn redis>'
done

kubectl apply -k deploy/overlays/${NS#vigov-}      # sinh ConfigMap cau-hinh-chung
```

**Xanh khi** `kubectl -n $NS get secret` có đủ 9 cái: `harbor-vigov`, 2 Secret TLS, 6 `bi-mat-*`.

| Thiếu | Triệu chứng |
|---|---|
| `harbor-vigov` | `ImagePullBackOff` |
| `DATABASE_DSN` / `GRPC_CALLER_KEY` / `SESSION_SIGNING_KEYS` | `CrashLoopBackOff`, log nêu tên biến |
| `REDIS_DSN` | pod xanh, nhưng 6 tuyến `POST` (cán bộ, văn bản đến/đi, chi, dự toán) trả 503 |
| Secret TLS sai tên theo môi trường | Ingress lên, chỉ HTTPS đứt |

## 3b. Cụm đang chạy — biến thêm từ 25/09/2026 phải đặt ở đâu

Cụm thật dựng TAY trong Rancher (`deploy/README.md`, khung ⚠ đầu tệp): ConfigMap **`common-config`**,
Secret **`<dịch vụ>-secrets`**, Deployment **`vigov-service-<dịch vụ>`** — không phải `cau-hinh-chung`
/ `bi-mat-*` của mục 1–3. Mọi biến dưới đây **tuỳ chọn**: không đặt thì pod vẫn lên, chỉ chức năng
ở cột cuối bị từ chối. Nghĩa và hình dạng giá trị: mục 4. Key viết **gạch dưới** (`envFrom`).

| Đặt vào | Key | Dịch vụ đọc | Không đặt thì |
|---|---|---|---|
| `common-config` | `TRUSTED_PROXY_CIDRS` | mọi dịch vụ Go | vết kiểm toán ghi IP pod web-admin thay IP người dùng |
| `common-config` | `CITIZEN_CORS_ALLOWED_ORIGINS` | `identity` · `petitions` · `comms` | trình duyệt chặn Mini App gọi tuyến công dân |
| `common-config` | `CITIZEN_SESSION_BRIDGE_LISTEN_ADDR` · `CITIZEN_SESSION_TTL` | `identity` | cầu phiên Mini App không mở (ADR 0045) · TTL 720h |
| `common-config` | `OBJECT_STORAGE_ENDPOINT` · `OBJECT_STORAGE_PUBLIC_ENDPOINT` · `OBJECT_STORAGE_REGION` · `OBJECT_STORAGE_BUCKET_PREFIX` · `MALWARE_SCANNER_ADDRESS` | `petitions` | mọi lần tải ảnh/tệp bị từ chối (ADR 0052) |
| `common-config` | `OBJECT_STORAGE_PUBLIC_MEDIA_BASE_URL` | `petitions` (qua `core/storage`); `comms` chưa nối kho tệp dù mục 4 dự kiến nó đăng media Mini App | media công khai không có URL |
| env của Deployment `vigov-service-identity` | `PETITIONS_GRPC_ADDR` (`petitions:9090`) · `DOCUMENTS_GRPC_ADDR` (`documents:9090`) — tên Service trên cụm thật có thể là `vigov-service-<dịch vụ>`, kiểm bằng `kubectl get svc` | `identity` | **Xoá bộ phận trả 503** (ADR 0056). Service petitions/documents phải mở cổng `9090` (`deploy/base/{petitions,documents}/service.yaml`) |
| `identity-secrets` | `CITIZEN_SESSION_BRIDGE_KEYS` | `identity` | cầu phiên Mini App không mở; **cùng giá trị** ở Secret của `vihat-miniapp` |
| `identity-secrets` | `IDENTITY_ADMIN_SEED_PASSWORD` | `identity` | không tự tạo `admin` cho xã mới. Gỡ key khi mọi xã đã đổi mật khẩu |
| `identity-secrets` | `OPERATOR_SESSION_SIGNING_KEYS` · `OPERATOR_TOTP_ENCRYPTION_KEY` | `identity` | mọi lần đăng nhập vận hành bị từ chối (ADR 0048) |
| `comms-secrets` | `SECRET_ENCRYPTION_KEYS` | `comms` | tab Máy chủ thư chỉ xem, không lưu được mật khẩu (ADR 0009). **Sao lưu khoá TÁCH khỏi bản sao lưu CSDL trước khi lưu mật khẩu đầu tiên** |
| `petitions-secrets` | `OBJECT_STORAGE_ACCESS_KEY` · `OBJECT_STORAGE_SECRET_KEY` | `petitions` | như dòng kho tệp ở trên — mỗi dịch vụ một cặp khoá riêng |

Đổi `common-config` hay `<dịch vụ>-secrets` **không** khởi động lại pod: `envFrom` chỉ đọc lúc pod
tạo ra. Sau khi thêm key, `kubectl -n vigov-prod rollout restart deploy/vigov-service-<dịch vụ>` cho
đúng các dịch vụ ở cột "Dịch vụ đọc".

Ngoài biến: gửi thư thử của `comms` cần NetworkPolicy mở cổng 465/587 ra ngoài
(`deploy/base/mang/netpol.yaml` hiện chỉ mở 5432/6379/9092) — không mở thì trả 502.

## 4. Toàn bộ biến — để đối chiếu

`tools/check_env_map.py` đối chiếu bảng này với `core/config/config.go` trong `make check`.

| Biến | Bắt buộc | Cấp bằng |
|---|---|---|
| `DATABASE_DSN` | **có** | Secret `bi-mat-<dịch vụ>` |
| `GRPC_CALLER_KEY` | **có** | Secret `bi-mat-<dịch vụ>` |
| `SESSION_SIGNING_KEYS` | **có** ở staging/prod | Secret `bi-mat-<dịch vụ>` |
| `REDIS_DSN` | không ở `config.Load`, cần ở prod | Secret — 5 dịch vụ, **không** `platform` |
| `ENV` | **có** | ConfigMap `cau-hinh-chung` (kustomize sinh) |
| `LISTEN_ADDR` | không | đã viết sẵn trong `deployment.yaml` |
| `PLATFORM_GRPC_ADDR` | không | đã viết sẵn trong `deployment.yaml` |
| `IDENTITY_GRPC_ADDR` | không | đã viết sẵn trong `deployment.yaml` |
| `PETITIONS_GRPC_ADDR` | không | đã viết sẵn trong `deployment.yaml` — **chỉ `identity`** (chặn xoá bộ phận còn giữ hồ sơ) |
| `DOCUMENTS_GRPC_ADDR` | không | đã viết sẵn trong `deployment.yaml` — **chỉ `identity`** (chặn xoá bộ phận còn giữ hồ sơ) |
| `GRPC_LISTEN_ADDR` | không | không khai — mặc định `:9090` |
| `TENANT_CACHE_TTL` | không | không khai — mặc định `30s` |
| `RABBITMQ_DSN` | không | chưa dùng — Secret ngày bật |
| `RABBITMQ_EXCHANGE` | không | chưa dùng — ConfigMap ngày bật |
| `ELASTICSEARCH_ADDRS` | không | chưa dùng — ConfigMap ngày bật |
| `ELASTICSEARCH_API_KEY` | không | chưa dùng — Secret ngày bật |
| `ELASTICSEARCH_INDEX_PREFIX` | không | không khai |
| `TRUSTED_PROXY_CIDRS` | không | ConfigMap `cau-hinh-chung` — dải IP pod web-admin. Trống thì audit ghi IP pod web-admin; mục sai hoặc `/0` thì pod không khởi động |
| `CITIZEN_CORS_ALLOWED_ORIGINS` | không | ConfigMap `cau-hinh-chung` — origin của webview Zalo Mini App được gọi **tuyến công dân** (không bao giờ tuyến cán bộ). Đề xuất `https://h5.zdn.vn,https://zalo.me,https://*.zdn.vn,https://*.zalo.me`. Trống thì không gửi header CORS nào — trình duyệt chặn Mini App; `*`, `http://` hoặc có đường dẫn thì pod không khởi động |
| `DANGEROUS_AUTH_BYPASS` | không | **không bao giờ khai** — `ENV=prod` từ chối khởi động |
| `CITIZEN_SESSION_BRIDGE_LISTEN_ADDR` | không | ConfigMap — **chỉ `identity`**. Cổng cầu phiên Mini App (ADR 0045). Phải khai **cùng** `…_KEYS`; cả hai trống thì cầu không mở |
| `CITIZEN_SESSION_BRIDGE_KEYS` | không | Secret `bi-mat-identity` — **chỉ `identity`**, và cùng giá trị ở Secret của `vihat-miniapp`. Danh sách `<khoa-moi>,<khoa-cu>`, mỗi khoá ≥ 32 byte. **Không bao giờ** trùng `GRPC_CALLER_KEY` |
| `CITIZEN_SESSION_TTL` | không | ConfigMap — **chỉ `identity`**. Trống thì `720h` (30 ngày); sai cú pháp hoặc ≤ 0 thì pod không khởi động |
| `IDENTITY_ADMIN_SEED_PASSWORD` | không | Secret `bi-mat-identity` — **chỉ `identity`**, key viết gạch dưới đúng như tên biến. Mật khẩu tài khoản `admin` tạo ở lần đăng nhập `admin` đầu tiên của một xã chưa có `admin` (bắt đổi mật khẩu ngay). Trống thì tắt; ngắn hơn 12 ký tự thì tắt và log báo lúc khởi động. **Gỡ key khi mọi xã đã đổi mật khẩu `admin`** |
| `OBJECT_STORAGE_ENDPOINT` | không | ConfigMap — chỉ dịch vụ lưu tệp (ADR 0052). Endpoint S3 nội bộ, `https://host:port`, **đúng một host** (MinIO phân tán thì đặt load balancer phía trước); nhiều host hoặc sai dạng thì pod không khởi động. Trống thì mọi lần tải lên bị từ chối |
| `OBJECT_STORAGE_PUBLIC_ENDPOINT` | không | ConfigMap — endpoint S3 trình duyệt thấy; presigned URL ký theo host này. Cùng luật một host |
| `OBJECT_STORAGE_PUBLIC_MEDIA_BASE_URL` | không | ConfigMap — chỉ dịch vụ đăng media Mini App (`comms`). URL gốc của bucket public, đặt CDN được |
| `OBJECT_STORAGE_ACCESS_KEY` | không | Secret `bi-mat-<dịch vụ>` — **mỗi dịch vụ một khoá**, IAM giới hạn vào `…/{service}/*` |
| `OBJECT_STORAGE_SECRET_KEY` | không | Secret `bi-mat-<dịch vụ>` — cặp với khoá trên |
| `OBJECT_STORAGE_REGION` | không | ConfigMap — trống thì `us-east-1` (mặc định của MinIO); phải trùng region của máy chủ, sai thì mọi presigned URL trả 403 |
| `OBJECT_STORAGE_BUCKET_PREFIX` | không | ConfigMap — bucket là `{prefix}-private`, `-public`, `-temp` (ví dụ tiền tố `vigov-prod`); sai dạng thì pod không khởi động |
| `MALWARE_SCANNER_ADDRESS` | không | ConfigMap — chỉ dịch vụ nhận tải tệp (ADR 0052 §9). Danh sách `host:port` của `clamd` (thường cổng 3310), ngăn bằng dấu phẩy; một Service k8s trước các replica là một mục. Trống thì **mọi lần tải lên bị từ chối** (không bao giờ lưu tệp chưa quét) và log khởi động báo nếu kho tệp đã cấu hình; có scheme, thiếu cổng hoặc trùng mục thì pod không khởi động |
| `OPERATOR_SESSION_SIGNING_KEYS` | không | Secret `bi-mat-identity` — **chỉ `identity`**. Khoá ký token người vận hành (ADR 0048). Danh sách `<khoa-moi>,<khoa-cu>`, mỗi khoá ≥ 32 byte. **Không bao giờ** trùng một khoá của `SESSION_SIGNING_KEYS` — trùng thì pod không khởi động. Trống thì mọi lần đăng nhập vận hành bị từ chối |
| `OPERATOR_TOTP_ENCRYPTION_KEY` | không | Secret `bi-mat-identity` — **chỉ `identity`**. Khoá mã hoá bí mật TOTP của người vận hành (AES-256-GCM). Mỗi mục là base64 chuẩn của **đúng 32 byte** (`openssl rand -base64 32`); danh sách `<khoa-moi>,<khoa-cu>`, khoá đầu mã hoá, mọi khoá giải mã. Sai dạng hoặc trùng mục thì pod không khởi động. Trống thì mọi lần đăng nhập vận hành bị từ chối |
| `SECRET_ENCRYPTION_KEYS` | không | Secret `bi-mat-<dịch vụ>` — **chỉ dịch vụ giữ bí mật riêng của xã** (trước hết `comms`: mật khẩu SMTP). Khoá gốc (KEK) bọc khoá dữ liệu của từng xã (ADR 0009). Cùng dạng `OPERATOR_TOTP_ENCRYPTION_KEY`: base64 chuẩn của **đúng 32 byte**, `<khoa-moi>,<khoa-cu>`, khoá đầu bọc, mọi khoá mở. Sai dạng hoặc trùng mục thì pod không khởi động. Trống thì mọi lần lưu/đọc bí mật của xã bị từ chối. **Mất hết khoá là mất vĩnh viễn bí mật của mọi xã** — sao lưu trước khi ghi bí mật đầu tiên, **tách khỏi** bản sao lưu CSDL |

Vì sao từng quyết định như vậy: `deploy/README.md` mục 3.

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
