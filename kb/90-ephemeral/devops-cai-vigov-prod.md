---
id: devops-cai-vigov-prod
tier: T5
source: CURATED
owner: architecture
derived_from_commit: 097c49e
expires: 2026-10-25
owns_facts:
  - "phiếu việc gửi DevOps: cài vigov-prod lần đầu (Secret, nhập manifest, đặt ảnh theo thứ tự) — tại 25/09/2026"
---

# ViGov — cài `vigov-prod` lần đầu (phiếu việc cho DevOps)

**Ngày:** 25/09/2026 · **Kho:** `github.com/tamtd91ht/vigov-v2`, nhánh `main`, commit `097c49e` trở đi

Đây là phiếu việc, hết hạn sau khi prod đã chạy. Tài liệu gốc vẫn là
`deploy/cau-hinh/README.md` (tên Secret, tên key) và `deploy/README.md` mục 7, 11.2–11.5. Phiếu
này lệch với hai tệp đó thì tin hai tệp đó.

## Bối cảnh

- Namespace `vigov-prod` **đã có** trên cụm `rancher-vigov`, nhưng **chưa có workload nào**.
- `vigov-staging` đã cài và chạy được. Prod làm y hệt, chỉ khác tên và giá trị.
- Job Jenkins `vigov-svc-<dịch vụ>` **chỉ đổi ảnh** (`kubectl set image`) của Deployment **đã có sẵn**. Nó không tạo Deployment. Vì vậy lúc này job báo:
  `Error from server (NotFound): deployments.apps "platform" not found`

Việc cần làm: tạo Secret, rồi nhập manifest vào `vigov-prod` **một lần duy nhất**.

## Cách nhanh — toàn bộ qua Jenkins, không SSH (từ commit sau `af15a8c`)

Job `vigov-deploy` → **Build with Parameters**, `MT=prod`:

| Lượt | `HANH_DONG` | `XAC_NHAN` | Kết quả |
|---|---|---|---|
| 1 | `kiem-tra` | — | In cụm, Deployment, pod, **tên** Secret, và Secret nào còn thiếu |
| 2 | `sao-chep-tu-staging` | `vigov-prod` | Chép mọi Secret + ConfigMap của staging sang prod. Chỉ tạo mới, không ghi đè. Bỏ qua `cau-hinh-chung`. `vigov-staging-tls` được chép thành `vigov-wildcard-tls` |
| 3 | `ap-manifest` | `vigov-prod` | Render `deploy/overlays/prod` rồi áp. Thiếu Secret thì từ chối |
| 4 | — | — | Bấm job dịch vụ theo thứ tự ở bước 4 bên dưới |

⚠ **Sau lượt 2, prod dùng CSDL + Redis + khoá ký phiên của staging** (các giá trị được chép
nguyên). Muốn prod có dữ liệu riêng thì sửa `DATABASE_DSN` / `REDIS_DSN` trong 6 `bi-mat-*` của
`vigov-prod` trên Rancher **trước lượt 4**. Chứng chỉ TLS của staging chỉ chạy được cho prod nếu
nó phủ tên miền prod.

Log pod khi có sự cố: `HANH_DONG=xem-log`, chọn `DICH_VU`.

Các bước tay dưới đây giữ lại làm phương án dự phòng.

## Bước 0 — chuẩn bị

| Cần có | Ghi chú |
|---|---|
| **6 CSDL PostgreSQL riêng cho prod** | `vigov_platform`, `vigov_identity`, `vigov_documents`, `vigov_finance`, `vigov_petitions`, `vigov_comms`. Mỗi CSDL **một tài khoản riêng**. Hai dịch vụ dùng chung một CSDL thì không báo lỗi gì, nhưng sẽ ghi chung một bảng `audit_log` |
| Redis cho prod | Thiếu Redis thì pod vẫn xanh, nhưng 6 tuyến `POST` (cán bộ, văn bản đến/đi, chi, dự toán) trả 503 |
| Chứng chỉ TLS wildcard của prod | `fullchain.pem` + `privkey.pem` |
| Tài khoản kéo ảnh từ `harbor.omicrm.services` | Giống staging được |
| **Sao lưu** | CSDL của `platform` và `identity` nếu đã có dữ liệu. Migration tự chạy lúc pod khởi động |

**Không chép nguyên Secret của staging sang prod.** DSN phải trỏ tới CSDL prod. Khoá nên sinh mới.

## Bước 1 — tạo Secret trong `vigov-prod`

Chạy trên máy có `kubectl` trỏ vào cụm, ví dụ máy chủ Jenkins với kubeconfig
`/u01/rancher/rancher-vigov.yaml`. Cũng có thể tạo trong Rancher → Storage → Secrets.

```sh
NS=vigov-prod
KC="--kubeconfig /u01/rancher/rancher-vigov.yaml"

# Khoá dùng chung — sinh MỘT lần, dán cùng một giá trị vào cả 6 Secret
GRPC_KEY=$(openssl rand -base64 48)
SIGN_NEW=$(openssl rand -base64 48)
SIGN_OLD=$(openssl rand -base64 48)      # prod cần >= 2 khoá, khoá mới đứng trước

# 1. Kéo ảnh
kubectl $KC -n $NS create secret docker-registry harbor-vigov \
  --docker-server=harbor.omicrm.services \
  --docker-username='<user>' --docker-password='<password>'

# 2. TLS — TÊN CỦA PROD là vigov-wildcard-tls (staging là vigov-staging-tls)
kubectl $KC -n $NS create secret tls vigov-wildcard-tls \
  --cert=<fullchain.pem> --key=<privkey.pem>

# 3. platform — KHÔNG có REDIS_DSN
kubectl $KC -n $NS create secret generic bi-mat-platform \
  --from-literal=DATABASE_DSN='<dsn của vigov_platform>' \
  --from-literal=GRPC_CALLER_KEY="$GRPC_KEY" \
  --from-literal=SESSION_SIGNING_KEYS="$SIGN_NEW,$SIGN_OLD"

# 4. Năm dịch vụ còn lại — có REDIS_DSN
for s in identity documents finance petitions comms; do
  kubectl $KC -n $NS create secret generic bi-mat-$s \
    --from-literal=DATABASE_DSN="<dsn của vigov_$s>" \
    --from-literal=GRPC_CALLER_KEY="$GRPC_KEY" \
    --from-literal=SESSION_SIGNING_KEYS="$SIGN_NEW,$SIGN_OLD" \
    --from-literal=REDIS_DSN='<dsn redis>'
done
```

Dạng DSN: `postgres://…@<host>:5432/<db>?sslmode=require`; có nhiều node thì ghi `h1:5432,h2:5432`.

⚠ **Key viết GẠCH DƯỚI** (`DATABASE_DSN`), không viết gạch ngang. Key sai dạng bị k8s bỏ qua mà
không báo gì, rồi pod chết với lỗi "thiếu biến môi trường bắt buộc".

⚠ Không đưa giá trị thật vào chat, ticket hay git.

**Kiểm:** lệnh dưới phải liệt kê đủ **8** Secret: `harbor-vigov`, `vigov-wildcard-tls`, 6 cái `bi-mat-*`.

```sh
kubectl $KC -n vigov-prod get secret
```

## Bước 2 — render manifest prod (trên máy chủ Jenkins)

Lệnh này chỉ đọc tệp YAML trong kho, **không** kết nối tới cụm.

```sh
cd /var/lib/jenkins/workspace/vigov-svc-platform
git log -1 --oneline                  # phải là 097c49e hoặc mới hơn
kubectl kustomize deploy/overlays/prod > /tmp/vigov-prod.yaml
```

## Bước 3 — nhập manifest vào `vigov-prod` (MỘT LẦN)

Chọn **một** trong hai cách:

- **Rancher:** Import YAML → chọn namespace **`vigov-prod`** → dán toàn bộ nội dung `/tmp/vigov-prod.yaml` → Import.
- **kubectl:** `kubectl $KC apply -f /tmp/vigov-prod.yaml`

**Kết quả đúng:** 7 Deployment (`platform`, `identity`, `comms`, `documents`, `finance`,
`petitions`, `web-admin`) đứng ở **`ImagePullBackOff`**, vì thẻ ảnh là
`CHUA-TRIEN-KHAI-LAN-NAO`. **Đây là thiết kế, không phải lỗi.** Thẻ thật do Jenkins đặt ở bước 4.

🚫 **Không nhập lại lần hai** khi dịch vụ đã chạy. Mỗi lần nhập lại, cả 7 dịch vụ bị kéo về
`ImagePullBackOff` cùng lúc. Sau này muốn sửa manifest thì làm theo `deploy/README.md` mục 7:
ghi lại thẻ đang chạy, áp, rồi đặt lại thẻ.

## Bước 4 — đặt ảnh thật (Jenkins, bấm tay)

Bấm **Build** từng job, **đúng thứ tự**. Đợi job trước xanh rồi mới bấm job sau.

1. `vigov-svc-platform`. Các dịch vụ khác gọi nó để phân giải tên miền ra xã.
2. `vigov-svc-identity`.
3. `vigov-svc-comms`, `vigov-svc-documents`, `vigov-svc-finance`, `vigov-svc-petitions`.
4. `vigov-web-admin`.

Mỗi job tự làm: test → đóng ảnh → `set image` → đợi `rollout status` tối đa 6 phút. Rollout đỏ
thì tự `rollout undo`.

**Kiểm sau mỗi job:**
- log job có dòng `XONG ... đang chạy ở vigov-prod`;
- với `platform` và `identity`: log pod có dòng `migration xong`.

## Bước 5 — kiểm cuối

```sh
kubectl $KC -n vigov-prod get deploy,pod
```

- 7 Deployment `READY`, không pod nào `CrashLoopBackOff`.
- Mở web-admin qua tên miền prod, đăng nhập được, không có lỗi 502.

## Lỗi thường gặp

| Triệu chứng | Nguyên nhân | Sửa |
|---|---|---|
| `ImagePullBackOff` **sau** bước 4 | Thiếu `harbor-vigov`, hoặc tạo nhầm namespace | Bước 1 |
| `CrashLoopBackOff`, log nêu tên biến | Thiếu key hoặc key viết gạch ngang | Bước 1 |
| Pod `platform`/`identity` không lên, log lỗi migration | CSDL chưa tạo, sai quyền, sai DSN | Kiểm CSDL, rồi bấm lại job |
| Ingress lên nhưng HTTPS lỗi | Secret TLS không đúng tên `vigov-wildcard-tls` | Bước 1 |
| Job Jenkins báo `cụm trả lời: ... Forbidden` | kubeconfig không có quyền trên `vigov-prod` | Cấp quyền cho danh tính trong kubeconfig (`deploy/README.md` mục 11.0.3) |
| Job Jenkins báo `cụm trả lời: ... NotFound` | Bước 3 chưa làm | Bước 3 |

## Nợ đang mở — biết để không bất ngờ

- **NetworkPolicy đường ra CSDL đang tạm mở `0.0.0.0/0`**, vẫn giới hạn cổng 5432/6379/9092. Có dải IP thật của PostgreSQL và Redis prod thì báo lại để siết (`deploy/base/mang/netpol.yaml`, sổ `_chung/netpol-duong-ra-tam-mo`).
- Cầu phiên Mini App (3 biến `CITIZEN_SESSION_*`, ADR 0045) **chưa bật**, không cần khai lúc này.
