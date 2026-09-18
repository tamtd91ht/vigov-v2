# `deploy/` — manifest Kubernetes

Tầng này chạm vào cụm. Chín job đóng ảnh **không** chạm — phạm vi của chúng dừng ở Harbor.

## Cái gì đang ở đây

| Thư mục | Nội dung |
|---|---|
| `cluster/` | Cài **một lần**: namespace, quyền của Jenkins trên cụm |
| `base/` | Hình dạng của từng đơn vị — **không** namespace, **không** thẻ ảnh |
| `overlays/<mt>/` | Namespace, cấu hình theo môi trường, và **thẻ ảnh** |
| `Jenkinsfile` | Job triển khai — nơi duy nhất gọi `kubectl` |

**Ba dịch vụ, không phải mười một.** `platform`, `identity`, `web-admin` có mã thật. Sáu
dịch vụ Go còn lại (`comms`, `documents`, `dossiers`, `finance`, `petitions`, `reporting`)
hôm nay là khung sinh sẵn ~220 dòng không có route nào; `platform-admin` còn chưa có
`Dockerfile`. Đưa chúng lên cụm là 12 pod không phục vụ gì, trên đúng 3 node. Thêm manifest
cho từng cái vào ngày nó có endpoint đầu tiên.

## Thứ tự cài lần đầu

```sh
kubectl apply -f deploy/cluster/namespace.yaml
kubectl apply -f deploy/cluster/rbac-jenkins.yaml

# Bí mật — TẠO NGOÀI KHO NÀY, không bao giờ commit (luật 8, bất biến 1)
kubectl -n vigov-prod create secret docker-registry harbor-vigov \
  --docker-server=registry.vihat.vn --docker-username=... --docker-password=...

kubectl -n vigov-prod create secret generic bi-mat-platform \
  --from-literal=DATABASE_DSN='postgres://...' \
  --from-literal=REDIS_DSN='redis://...' \
  --from-literal=SESSION_SIGNING_KEYS='...'
kubectl -n vigov-prod create secret generic bi-mat-identity --from-literal=...

kubectl -n vigov-prod create secret tls vigov-wildcard-tls --cert=... --key=...
```

Rồi triển khai **`platform` trước**. Bảy dịch vụ còn lại quay số tới cổng 9090 của nó để
phân giải `Host` → xã; platform chưa có thì mọi xã trả 404.

## Bí mật vào cụm bằng đường nào

Hôm nay: `kubectl create secret` bằng tay. Chấp nhận được khi có một người vận hành, nhưng
nó có hai lỗ: không ai biết khoá được xoay lần cuối lúc nào, và một lần dựng lại cụm là một
buổi chiều gõ lại.

Bước kế tiếp khi thấy phiền: **External Secrets Operator** trỏ về kho secret thật. Manifest
trong kho này khi ấy vẫn **chỉ chứa tên khoá**, không bao giờ giá trị.

`SESSION_SIGNING_KEYS` cần **ít nhất hai khoá** ở prod — một khoá thì không xoay được mà
không đăng xuất toàn bộ cán bộ của mọi xã cùng lúc (`core/config.CanhBao`).

## Triển khai

Qua job `vigov-deploy`, không bao giờ bằng `kubectl` tay:

```
DICH_VU = identity
THE     = 1b7b276a9c3f     ← commit 12 ký tự, lấy từ log job đóng ảnh
MT      = staging          ← rồi mới prod
```

Job kiểm ảnh có thật trong Harbor **trước khi** commit, ghim đúng một dòng `newTag`, đẩy
commit, `kubectl apply -k`, đợi rollout, và `rollout undo` nếu đỏ.

`git log -p deploy/overlays/prod/kustomization.yaml` là **nhật ký triển khai**: ai, bản nào,
lúc nào. Không có sổ nào khác.

## Bốn điều dễ hiểu sai

| Điều | Sự thật |
|---|---|
| "Mỗi xã một namespace" | **Không.** Cùng một pod phục vụ mọi xã; xã phân giải từ `Host` trong từng request (luật 1, bất biến 3). Thêm xã = thêm bản ghi DNS + một hàng trong sổ đăng ký của `platform`, không sửa manifest |
| "Cần Job di trú CSDL" | **Không.** Di trú chạy trong tiến trình lúc khởi động, có advisory lock (`core/migrate.Chay`). Vì thế `startupProbe` rộng tay — replica thứ hai **đợi** replica thứ nhất |
| "`/healthz` xanh nghĩa là hệ thống ổn" | **Không.** Nó nằm ngoài chuỗi phân giải xã có chủ ý, nên nó vẫn xanh khi CSDL hoặc `platform` hỏng. Bắt sự cố đó bằng **giám sát tỉ lệ 404** từ `TenantMiddleware`, không bằng probe |
| "NetworkPolicy là tuỳ chọn" | **Không.** gRPC 9090 dùng `insecure.NewCredentials()` — không TLS, không xác thực. NetworkPolicy là biên duy nhất giữ danh bạ xã của toàn hệ thống |

## Chưa chốt

Định tuyến `/api/v1/*`. Hôm nay mọi tài nguyên REST đều thuộc `identity` nên một dòng là
đúng. Ngày dịch vụ thứ hai có route, nó thành sai — xem chú giải đầu
`overlays/prod/ingress.yaml`.
