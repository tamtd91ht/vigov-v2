# `deploy/` — manifest Kubernetes

Tầng này chạm vào cụm. Chín job đóng ảnh **không** chạm — phạm vi của chúng dừng ở Harbor.

## Cái gì đang ở đây

| Thư mục | Nội dung |
|---|---|
| `cluster/` | Cài **một lần**: namespace, quyền của Jenkins trên cụm |
| `base/` | Hình dạng của từng đơn vị — **không** namespace, **không** thẻ ảnh |
| `overlays/<mt>/` | Namespace, cấu hình theo môi trường, và **thẻ ảnh** |
| `Jenkinsfile` | Job triển khai — nơi duy nhất gọi `kubectl` |

**Bảy đơn vị, không phải mười — và tiêu chí là một câu kiểm được.** Manifest được thêm cho
một dịch vụ vào ngày nó có endpoint đầu tiên, và "có endpoint" đọc từ
`kb/20-contracts/openapi.json` chứ không từ cảm giác:

| Đơn vị | Số tuyến REST | Có manifest |
|---|---|---|
| `identity` | 15 | có |
| `finance` · `petitions` | 3 mỗi cái | có, thêm 21/09/2026 |
| `comms` · `documents` | 1 mỗi cái | có, thêm 21/09/2026 |
| `platform` | 0 REST, nhưng LÀ gRPC phân giải xã | có |
| `web-admin` | — (Next.js) | có |
| `reporting` | **0** | **không** — `internal/http/routes.go` chưa mount tuyến nào |
| `platform-admin` | — | **không** — chưa có `Dockerfile`, nên chưa có ảnh để đưa lên |
| `citizen-app` | — | **không** — chạy trong Zalo Mini App, không thành pod trên cụm |

Ba dòng cuối là **kết luận đã cân nhắc, không phải chỗ sót**: một pod không phục vụ gì vẫn
chiếm chỗ trên đúng 3 node, vẫn phải vá, vẫn phải theo dõi.

⚠ **Bốn dịch vụ mới lên cụm sẽ KHÔNG nhận được yêu cầu nào từ Internet** cho tới khi câu
"định tuyến `/api/v1/*`" ở mục *Chưa chốt* dưới đây được chủ dự án chốt: Ingress hiện đưa cả
`/api/v1` về `identity`. Chúng chạy được, di trú lược đồ của mình, và chờ.

## Dựng 10 job trên Jenkins

Jenkins **không tự tìm ra** mười `Jenkinsfile` nằm rải trong kho. Bạn tạo 10 job kiểu
**Pipeline**, tất cả trỏ về cùng một kho, và thứ duy nhất khác nhau giữa chúng là ô
**Script Path**.

| Tên job | Script Path | Kích hoạt |
|---|---|---|
| `vigov-gate` | `Jenkinsfile` | webhook / poll `main` |
| `vigov-svc-comms` | `service-comms/Jenkinsfile` | ″ |
| `vigov-svc-documents` | `service-documents/Jenkinsfile` | ″ |
| `vigov-svc-finance` | `service-finance/Jenkinsfile` | ″ |
| `vigov-svc-identity` | `service-identity/Jenkinsfile` | ″ |
| `vigov-svc-petitions` | `service-petitions/Jenkinsfile` | ″ |
| `vigov-svc-platform` | `service-platform/Jenkinsfile` | ″ |
| `vigov-svc-reporting` | `service-reporting/Jenkinsfile` | ″ |
| `vigov-web-admin` | `web-admin/Jenkinsfile` | ″ |
| `vigov-deploy` | `deploy/Jenkinsfile` | **không kích hoạt tự động — chỉ bấm tay** |

`New Item` → tên → **Pipeline** → OK, rồi:

| Mục trong form | Điền gì |
|---|---|
| General | **bỏ trống**. `timeout`, `buildDiscarder`, `disableConcurrentBuilds` đã khai trong Jenkinsfile — khai lại ở UI là hai nguồn sẽ lệch, và bản lỏng hơn là bản chạy |
| Build Triggers | Jenkins ra được Internet → webhook GitHub. Không → `Poll SCM` lịch `H/5 * * * *`. Job `vigov-deploy` **không tick gì** |
| Pipeline → Definition | `Pipeline script from SCM` |
| SCM | `Git` · URL kho · credential `git-vigov` · Branch `*/main` |
| Additional Behaviours | **để trống** — xem hai điều cấm dưới |
| Script Path | lấy từ bảng trên |
| Lightweight checkout | **BỎ TICK** |

### Hai thứ tuyệt đối không bật

**`Shallow clone`.** Cơ chế dựng lại so bằng `git diff <mốc>..HEAD`, trong đó mốc là commit
của lượt đóng ảnh trước (xem `mocDaDongAnh()` ở cuối mỗi `Jenkinsfile`). Clone nông không
chứa commit đó, `git cat-file -e` trượt, và job **dựng lại mọi lần**. Không hỏng — cơ chế cố
ý sai về phía dựng thừa — nhưng toàn bộ việc lọc biến mất, mỗi push thành chín lượt đóng ảnh.

**`Sparse checkout`.** Tám dịch vụ Go lấy ngữ cảnh build ở **gốc kho** vì `go.mod` của chúng
`replace` module `core` bằng đường dẫn `../core`. Checkout thiếu `core/` là `docker build` đổ.

**`Lightweight checkout`** cũng phải bỏ: nó chỉ kéo riêng tệp Jenkinsfile qua API, không tạo
cây làm việc — mà `canDungLai()` chạy `git` thật trong stage `Chuẩn bị`.

### Vì sao 11 job, không phải một job có tham số `DICH_VU`

Mốc "commit đã thành ảnh" được ghi vào **`description` của lượt build**, và mỗi job có lịch
sử build riêng. Gộp tám dịch vụ vào một job có tham số thì tám mốc chen nhau trong **một**
dòng lịch sử: job tìm ngược lại sẽ nhặt phải mốc của dịch vụ khác, rồi kết luận "không có gì
đổi" cho một dịch vụ vừa bị sửa.

Một job cho mỗi dịch vụ không phải để UI cho gọn — nó là **nơi lưu trạng thái** của cơ chế.

### Credentials — ID phải khớp từng ký tự với `defaultValue` trong Jenkinsfile

| ID | Kiểu | Dùng ở |
|---|---|---|
| `harbor-vigov` | Username with password | 9 job đóng ảnh (`docker.withRegistry`) **và** `vigov-deploy` (`skopeo inspect`) |
| `git-vigov` | Username with password (PAT) | `vigov-deploy` — đẩy commit ghim thẻ |
| `kubeconfig-vigov` | Secret file | `vigov-deploy` (`withKubeConfig`) |

### Plugin và công cụ trên máy chủ build

**Plugin:** Git · Pipeline · **Docker Pipeline** (`docker.build`) · **Kubernetes CLI**
(`withKubeConfig`) · Credentials Binding · **build-user-vars**.

Thiếu `build-user-vars` thì commit ghim thẻ ghi `Bấm bởi: khong-ro` — nhật ký triển khai mất
phần "ai", vốn là nửa lý do nó tồn tại. Cài plugin **và** bọc stage bằng
`wrap([$class: 'BuildUser'])`.

**Công cụ:** `go` 1.26+ · `buf` · `node` 22+ · `npm` · `python3` · một trình biên dịch C
(`go test -race` cần cgo) · `docker`. Riêng `vigov-deploy` thêm: `kubectl`, `kustomize` bản
độc lập (`kubectl -k` **không** có `kustomize edit`), `skopeo`.

### Lượt chạy đầu tiên

Chín job đóng ảnh sẽ báo `Chưa lượt build nào ghi mốc đã đóng ảnh — dựng` và đóng **cả chín
ảnh**. Đó là đúng, không phải lỗi: chưa có mốc nào để so. Từ lượt thứ hai mới lọc.

`vigov-deploy` khai tham số **bên trong** Jenkinsfile, nên lần đầu nó chạy không có ô nhập
nào. Chạy một lượt cho Jenkins đọc khai báo, lượt sau mới hiện form — lượt đầu ấy sẽ đỏ ở
bước kiểm `THE`, và đỏ như vậy là đúng.

## Thứ tự cài lần đầu

```sh
kubectl apply -f deploy/cluster/namespace.yaml
kubectl apply -f deploy/cluster/rbac-jenkins.yaml

# Bí mật — TẠO NGOÀI KHO NÀY, không bao giờ commit (luật 8, bất biến 1)
kubectl -n vigov-prod create secret docker-registry harbor-vigov \
  --docker-server=harbor.omicrm.services --docker-username=... --docker-password=...

kubectl -n vigov-prod create secret generic bi-mat-platform \
  --from-literal=DATABASE_DSN='postgres://...' \
  --from-literal=REDIS_DSN='redis://...' \
  --from-literal=SESSION_SIGNING_KEYS='...' \
  --from-literal=GRPC_CALLER_KEY='...'   # BẮT BUỘC ở MỌI dịch vụ — `config.Load` không khởi
                                         # động khi rỗng: một cổng gRPC không có khoá gọi là
                                         # một cổng trả lời BẤT KỲ AI chạm tới nó (ADR 0025)
kubectl -n vigov-prod create secret generic bi-mat-identity --from-literal=...

# Bốn dịch vụ thêm ngày 21/09/2026. BA KHOÁ LÀ ĐIỀU KIỆN KHỞI ĐỘNG, không phải tuỳ chọn:
# `core/config.Load` gom đủ tên còn thiếu rồi thoát. SESSION_SIGNING_KEYS nằm trong danh sách
# kể cả khi dịch vụ không tự phát phiên — Load đòi nó ở staging/prod.
for s in comms documents finance petitions; do
  kubectl -n vigov-prod create secret generic bi-mat-$s \
    --from-literal=DATABASE_DSN='postgres://...' \
    --from-literal=GRPC_CALLER_KEY='...' \
    --from-literal=SESSION_SIGNING_KEYS='...'
done

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

**Định tuyến `/api/v1/*` — NGÀY ẤY ĐÃ TỚI, và đây là việc đang chặn.** Chú giải đầu
`overlays/prod/ingress.yaml` viết "ngày `petitions` có route đầu tiên, dòng đó thành sai".
Hôm nay **bốn** dịch vụ có route: `comms` · `documents` · `finance` · `petitions`, tổng 8
tuyến (`kb/20-contracts/openapi.json`). Ingress vẫn đưa cả `/api/v1` về `identity`, nên 8
tuyến ấy trả **404** dù pod chạy đúng và probe xanh.

Ba lối ra nằm trong chính chú giải đó — (a) liệt kê từng tài nguyên, (b) sinh Ingress từ
`openapi.json`, (c) thêm gateway. **Đây là câu hỏi cho chủ dự án, không phải cho agent**:
đừng chọn hộ, và đừng thêm path vào `ingress.yaml` trước khi có câu trả lời.

## Chưa vá

`Jenkinsfile` đẩy commit ghim thẻ bằng `git push origin HEAD:main` từ một detached HEAD. Nếu
`main` nhích lên trong lúc job chạy, push bị từ chối **sau khi** `kustomize edit` đã sửa tệp
— lượt deploy đỏ ở một chỗ khó đọc. Cần `git fetch origin main && git rebase origin/main`
trước khi push. Chưa cháy vì chưa lượt nào chạy.

**HAI TÊN REGISTRY — ĐÃ VÁ 21/09/2026.** Giữ lại đoạn này vì cách hỏng của nó đáng nhớ, không
phải vì nó còn đang hỏng.

Chủ dự án chốt `harbor.omicrm.services/ci` (cùng registry mà mọi job khác trên máy chủ Jenkins
ấy đang đẩy). Cả hai nơi nay mang **một** tên: 21 tham chiếu trong `base/*/deployment.yaml` và
`overlays/*/kustomization.yaml` đã đổi theo, cùng lệnh `create secret docker-registry
--docker-server=` ở mục *Thứ tự cài lần đầu*.

Vì sao nó nguy hiểm hơn một chỗ lệch tên bình thường: `kustomize edit set image` khớp theo TÊN
ẢNH. Hai tên khác nhau thì kustomize **không đổi gì cả** — nhưng `git diff` vẫn thấy tệp đổi,
vì một mục `images:` mới được thêm vào. Nên bước "không có diff thì dừng" ở `deploy/Jenkinsfile`
vẫn cho đi tiếp: lượt deploy ghim một dòng vô tác dụng, đẩy một commit nói dối vào nhật ký
triển khai, rồi ngồi `ImagePullBackOff` với thẻ `CHUA-TRIEN-KHAI-LAN-NAO`. Ba bước xanh trước
khi có gì đỏ, và chỗ đỏ cách nguyên nhân rất xa.

Bài học giữ lại: **đổi registry là đổi ở HAI hệ thống tên** — Jenkinsfile sinh ra tên, manifest
tiêu thụ tên. Đổi một bên là dựng ra đúng cái bẫy trên.
