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
| Secret `tls` | staging: **`vigov-staging-tls`** · prod: **`vigov-wildcard-tls`** | `tls.crt` · `tls.key` | Ingress |

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
TLS=vigov-staging-tls     # prod: vigov-wildcard-tls

kubectl -n $NS create secret docker-registry harbor-vigov \
  --docker-server=harbor.omicrm.services --docker-username='<...>' --docker-password='<...>'

kubectl -n $NS create secret tls $TLS --cert=<fullchain.pem> --key=<privkey.pem>

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

**Xanh khi** `kubectl -n $NS get secret` có đủ 8 cái: `harbor-vigov`, Secret TLS, 6 `bi-mat-*`.

| Thiếu | Triệu chứng |
|---|---|
| `harbor-vigov` | `ImagePullBackOff` |
| `DATABASE_DSN` / `GRPC_CALLER_KEY` / `SESSION_SIGNING_KEYS` | `CrashLoopBackOff`, log nêu tên biến |
| `REDIS_DSN` | pod xanh, nhưng 6 tuyến `POST` (cán bộ, văn bản đến/đi, chi, dự toán) trả 503 |
| Secret TLS sai tên theo môi trường | Ingress lên, chỉ HTTPS đứt |

## 4. Toàn bộ 21 biến — để đối chiếu

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
| `GRPC_LISTEN_ADDR` | không | không khai — mặc định `:9090` |
| `TENANT_CACHE_TTL` | không | không khai — mặc định `30s` |
| `RABBITMQ_DSN` | không | chưa dùng — Secret ngày bật |
| `RABBITMQ_EXCHANGE` | không | chưa dùng — ConfigMap ngày bật |
| `ELASTICSEARCH_ADDRS` | không | chưa dùng — ConfigMap ngày bật |
| `ELASTICSEARCH_API_KEY` | không | chưa dùng — Secret ngày bật |
| `ELASTICSEARCH_INDEX_PREFIX` | không | không khai |
| `TRUSTED_PROXY_CIDRS` | không | ConfigMap `cau-hinh-chung` — dải IP pod web-admin. Trống thì audit ghi IP pod web-admin; mục sai hoặc `/0` thì pod không khởi động |
| `DANGEROUS_AUTH_BYPASS` | không | **không bao giờ khai** — `ENV=prod` từ chối khởi động |
| `CITIZEN_SESSION_BRIDGE_LISTEN_ADDR` | không | ConfigMap — **chỉ `identity`**. Cổng cầu phiên Mini App (ADR 0045). Phải khai **cùng** `…_KEYS`; cả hai trống thì cầu không mở |
| `CITIZEN_SESSION_BRIDGE_KEYS` | không | Secret `bi-mat-identity` — **chỉ `identity`**, và cùng giá trị ở Secret của `vihat-miniapp`. Danh sách `<khoa-moi>,<khoa-cu>`, mỗi khoá ≥ 32 byte. **Không bao giờ** trùng `GRPC_CALLER_KEY` |
| `CITIZEN_SESSION_TTL` | không | ConfigMap — **chỉ `identity`**. Trống thì `720h` (30 ngày); sai cú pháp hoặc ≤ 0 thì pod không khởi động |
| `IDENTITY_ADMIN_SEED_PASSWORD` | không | Secret `bi-mat-identity` — **chỉ `identity`**, key viết gạch dưới đúng như tên biến. Mật khẩu tài khoản `admin` tạo ở lần đăng nhập `admin` đầu tiên của một xã chưa có `admin` (bắt đổi mật khẩu ngay). Trống thì tắt; ngắn hơn 12 ký tự thì tắt và log báo lúc khởi động. **Gỡ key khi mọi xã đã đổi mật khẩu `admin`** |

Vì sao từng quyết định như vậy: `deploy/README.md` mục 3.
