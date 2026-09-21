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

Cả năm dịch vụ có tuyến REST **đều có đường vào từ Internet** kể từ 21/09/2026: bảng định
tuyến của Ingress **được sinh ra** từ `kb/20-contracts/openapi.json` bởi `tools/ingress`, nên
"thêm tuyến trong Go" và "tuyến ấy đi tới đúng dịch vụ" không còn là hai việc rời nhau. Xem
mục *Ingress sinh từ hợp đồng REST* dưới đây.

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

## Ingress sinh từ hợp đồng REST — CHỐT 21/09/2026, lối (b)

Chú giải cũ đầu `overlays/prod/ingress.yaml` hẹn: "ngày `petitions` có route đầu tiên, dòng
`/api/v1` → `identity` thành sai". Ngày ấy tới, và nó tới với **năm** dịch vụ chứ không phải
hai: 23 tuyến REST — `identity` 15 · `finance` 3 · `petitions` 3 · `documents` 1 · `comms` 1.
Tám tuyến trong đó sẽ trả **404** dù pod chạy đúng và probe xanh.

Ba lối ra đã viết sẵn ở chú giải ấy. Chủ dự án chốt **(b): sinh Ingress từ `openapi.json`**.

| Tệp | Vai trò |
|---|---|
| `tools/ingress/` | Bộ sinh. `go run ./tools/ingress`, đã gắn vào mục `kb` của `Makefile` **sau** `apidoc` |
| `base/mang/ingress.yaml` | **SINH RA.** Bảng định tuyến — giống nhau ở mọi môi trường nên nó thuộc `base/` |
| `overlays/<mt>/ingress-moi-truong.yaml` | Bản vá **JSON6902** cho host + TLS — thứ duy nhất khác nhau giữa hai môi trường |

**Vì sao bản vá là JSON6902 chứ không phải một `Ingress` đầy đủ trong overlay:** `spec.rules`
là danh sách không có khoá trộn, nên một strategic-merge patch chỉ cần *nhắc tới* `rules` là
**thay cả danh sách** — xoá sạch bảng sinh ra, và `kustomize` không kêu một tiếng.

**Gom tới mức TÀI NGUYÊN, không hơn.** 23 tuyến thành 19 luật (`sessions` gom 3, `staff` 2,
`investment-projects` 2). Không gom theo tiền tố có gạch nối: `task-blocs` là `identity` trong
khi `task-priorities` và `task-types` là `petitions` — một luật `task` sẽ gửi hai tuyến của
`petitions` sang `identity`. `pathType: Prefix` khớp theo **đoạn** đường dẫn, nên
`/api/v1/roles` không nuốt `/api/v1/role-permissions`.

**Một tuyến không xác định được dịch vụ chủ thì bộ sinh DỪNG** và không ghi tệp nào — không
mặc định về `identity` (luật 1: không có mặc định trên đường cách ly). Ba tín hiệu phải khớp
nhau: đúng một `tag`, `operationId` mở đầu bằng chính tag ấy, và mọi phương thức của một
đường dẫn cùng một chủ. Dịch vụ có tuyến nhưng **không có** `base/<tên>/service.yaml` cũng là
DỪNG — Ingress trỏ vào hư không trả 503 và không manifest nào giải thích được.

`go test ./tools/ingress` đối chiếu tệp **trên đĩa** với hợp đồng: mọi tuyến phải có đúng một
luật và trỏ đúng dịch vụ, và không luật nào được thừa. Xoá một luật hay đổi một backend là
**đỏ**, cả hai đã thử.

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
