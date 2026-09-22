# `deploy/` — manifest Kubernetes

Tầng này chạm vào cụm. **Tám** job đóng ảnh **không** chạm — phạm vi của chúng dừng ở Harbor.

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

## Phương án đưa lên — bốn pha

**Chốt 22/09/2026.** Mô hình: Jenkins đóng ảnh và **dừng ở Harbor**; job `vigov-deploy` là
thứ duy nhất chạm cụm. Kubeconfig tới Jenkins bằng credentials `kubeconfig-vigov` kiểu
**Secret file** — tức giữ nguyên `deploy/Jenkinsfile` hiện tại, không đổi sang tệp nằm sẵn
trên đĩa như dự án `omicrm` dùng.

Bốn pha đi **tuần tự**. Mỗi pha có một điều kiện xanh **kiểm được**; chưa xanh thì không sang
pha sau. Không pha nào chạy song song cho nhanh: mỗi pha chứng minh đúng một tầng mà pha sau
đã giả định là đúng, và bỏ qua nó chỉ dời chỗ phát hiện lỗi ra xa nguyên nhân.

| Pha | Việc | Điều kiện xanh | Chi tiết |
|---|---|---|---|
| **0** | Hạ tầng: cụm, wildcard DNS, chứng thư | **ĐÃ CÓ** — chủ dự án xác nhận 22/09/2026 | — |
| **1** | Dựng 10 job, chứng minh máy chủ build đủ công cụ | `vigov-gate` xanh · một job đóng ảnh đẩy được lên Harbor | *Dựng 10 job* ↓ |
| **2** | Cài một lần lên cụm: namespace, RBAC, bí mật, TLS | `kubectl -n vigov-staging get secret` đủ mục | *Thứ tự cài lần đầu* ↓ |
| **3** | staging — 7 lượt `vigov-deploy` | 7 Deployment `READY`, một xã thử gọi được qua Ingress | *Triển khai* ↓ |
| **4** | prod — lặp lại pha 3 + phép kiểm hai xã | như trên, cộng phép kiểm cách ly hai xã | *Triển khai* ↓ |

### Pha 1 — Jenkins

1. Tạo 10 job theo bảng *Dựng 10 job trên Jenkins*, và tạo trước hai mục credentials
   (`kubeconfig-vigov`, `git-vigov`).
2. Chạy **`vigov-gate` trước mọi job khác.** Nó là job duy nhất kiểm cả 7 công cụ và chạy
   `make check`, nên nó biến "máy chủ build thiếu gì" thành **một** lần đỏ đọc được, thay vì
   8 lần đỏ rải rác. Rủi ro đã dự đoán: `buf`, `node`, `npm`, `python3` chưa từng được chứng
   minh có trên máy chủ này — `go`, `docker`, `make`, `gcc` thì đã, qua lượt chạy thật
   21/09/2026 của kho `vihat-miniapp`.
3. Chạy **một** job đóng ảnh, đề xuất `vigov-svc-platform` — nó là đơn vị đầu tiên phải lên
   cụm ở pha 3.
4. Lượt đầu của mọi job đóng ảnh sẽ báo *"Chưa lượt build nào ghi mốc đã đóng ảnh — dựng"* và
   dựng tất. Đúng, không phải lỗi — xem *Lượt chạy đầu tiên*.

**Xanh khi:** `vigov-gate` xanh **và** `skopeo inspect docker://harbor.omicrm.services/ci/vigov-service-platform:<thẻ>`
trả về manifest.

### Pha 2 — cụm

Chạy khối lệnh ở *Thứ tự cài lần đầu*, **cho `vigov-staging` trước**. Ba thứ dễ sót, và cả ba
đều hỏng ở chỗ cách xa nguyên nhân:

| Sót | Triệu chứng |
|---|---|
| Secret `harbor-vigov` (kéo ảnh) | `ImagePullBackOff`, không nói gì về quyền |
| `GRPC_CALLER_KEY` / `SESSION_SIGNING_KEYS` | pod `CrashLoopBackOff` — `core/config.Load` gom đủ tên còn thiếu rồi thoát |
| Secret TLS `vigov-staging-tls` | Ingress lên bình thường, chỉ HTTPS đứt |

**Xanh khi:** `kubectl -n vigov-staging get secret` liệt kê đủ `harbor-vigov`,
`vigov-staging-tls`, và `bi-mat-{platform,identity,comms,documents,finance,petitions}`.

### Pha 3 và 4 — bảy lượt deploy, **theo đúng thứ tự này**

Mỗi lượt là một lần bấm `vigov-deploy` với `DICH_VU` + `THE` + `MT`. Thứ tự không phải thói
quen — nó là thứ tự phụ thuộc lúc chạy:

| # | Đơn vị | Đi trước vì |
|---|---|---|
| 1 | `platform` | Sáu đơn vị kia quay số cổng 9090 của nó để phân giải `Host` → xã. Chưa có nó thì **mọi xã trả 404** |
| 2 | `identity` | Bốn dịch vụ ở dưới đổi cookie lấy principal qua gRPC 9090 của nó. Chưa có nó thì **mọi tuyến có kiểm quyền trả 401 cho một phiên hợp lệ** — pod xanh, probe xanh, không gì báo |
| 3–6 | `comms` · `documents` · `finance` · `petitions` | Độc lập nhau, thứ tự tuỳ |
| 7 | `web-admin` | **Sau cùng, có chủ ý.** Nó là bề mặt cán bộ nhìn thấy; đưa lên trước khi API sau lưng trả lời được nghĩa là một màn hình lỗi mang tên một cơ quan nhà nước |

**Xanh khi:** 7 Deployment `READY`, **và** một xã thử — đã có bản ghi DNS và một hàng trong
sổ đăng ký của `platform` — gọi được một tuyến qua Ingress bằng `Host` của chính nó.

`/healthz` xanh **không** phải điều kiện xanh: nó nằm ngoài chuỗi phân giải xã có chủ ý — xem
*Bốn điều dễ hiểu sai*.

Trước khi sang **pha 4 (prod)**: chạy phép kiểm cách ly hai xã (luật 1) — hai `Host` khác
nhau, cùng một tuyến, và dữ liệu trả về không được giao nhau. Đó là phép kiểm mà staging tồn
tại để chạy; một lượt deploy prod không chứng minh lại được nó.

### Rút lui

Job tự `rollout undo` khi `rollout status` đỏ, rồi đợi bản cũ `READY`. **Commit ghim thẻ
được giữ nguyên, cố ý** (`deploy/Jenkinsfile:197`): nhật ký triển khai phải ghi cả lần thất
bại. Đưa nhật ký khớp lại với thực tế là việc của người — **chạy lại job với thẻ cũ**, đừng
`git revert`.

### Còn nợ trước khi gọi là chạy thật

| Nợ | Ở đâu | Hệ quả nếu bỏ qua |
|---|---|---|
| `BUILD_USER_ID` luôn rỗng | `deploy/Jenkinsfile:159` | Nhật ký triển khai không trả lời được *ai* — nửa lý do nó tồn tại |
| `git push` không rebase | *Chưa vá* ↓ | Lượt deploy đỏ ở chỗ khó đọc khi `main` nhích lên giữa chừng |
| 4 dịch vụ không bắt `SIGTERM` | `service-{comms,documents,finance,petitions}/cmd/server/main.go` | Một lượt deploy cắt ngang lúc tiếp nhận phản ánh để lại phiếu dở trong khi công dân đã cầm mã tra cứu. **Đáng vá TRƯỚC tuyến ghi đầu tiên của `petitions`** |
| Manifest chưa từng qua API server thật | — | `kubectl kustomize` chỉ chứng minh YAML dựng được, không chứng minh máy chủ chấp nhận. Pha 2 là lần đầu biết |

---

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
ý sai về phía dựng thừa — nhưng toàn bộ việc lọc biến mất, mỗi push thành tám lượt đóng ảnh.

**`Sparse checkout`.** Bảy dịch vụ Go lấy ngữ cảnh build ở **gốc kho** vì `go.mod` của chúng
`replace` module `core` bằng đường dẫn `../core`. Checkout thiếu `core/` là `docker build` đổ.

**`Lightweight checkout`** cũng phải bỏ: nó chỉ kéo riêng tệp Jenkinsfile qua API, không tạo
cây làm việc — mà `canDungLai()` chạy `git` thật trong stage `Chuẩn bị`.

### Vì sao 10 job, không phải một job có tham số `DICH_VU`

Mốc "commit đã thành ảnh" được ghi vào **`description` của lượt build**, và mỗi job có lịch
sử build riêng. Gộp tám đơn vị đóng ảnh vào một job có tham số thì tám mốc chen nhau trong
**một** dòng lịch sử: job tìm ngược lại sẽ nhặt phải mốc của dịch vụ khác, rồi kết luận
"không có gì đổi" cho một dịch vụ vừa bị sửa.

Một job cho mỗi dịch vụ không phải để UI cho gọn — nó là **nơi lưu trạng thái** của cơ chế.

### Credentials — ID phải khớp từng ký tự với `defaultValue` trong Jenkinsfile

**HAI mục, không phải ba.** Mọi lời gọi `withCredentials` còn lại trong kho:

| ID | Kiểu | Dùng ở |
|---|---|---|
| `kubeconfig-vigov` | **Secret file** (nội dung là kubeconfig) | `deploy/Jenkinsfile:169,182,199` — ràng buộc `file`, đặt biến `KUBECONFIG` |
| `git-vigov` | Username with password (PAT) | `deploy/Jenkinsfile:151` — đẩy commit ghim thẻ · **và** ô `SCM` của cả 10 job |

**`harbor-vigov` KHÔNG còn là một mục credentials của Jenkins.** Tám job đóng ảnh gọi thẳng
`docker` CLI và `vigov-deploy` gọi `skopeo --authfile`, cả hai dựa vào phiên đăng nhập sẵn
trong `~jenkins/.docker/config.json` — chốt 21/09/2026, lý do đầy đủ ở `Jenkinsfile:144`.
Cái tên vẫn tồn tại ở **một chỗ khác, nghĩa khác**: Secret `harbor-vigov` **trong cụm k8s**
(`imagePullSecrets`, mục *Thứ tự cài lần đầu*). Hai vật khác nhau trùng tên — đừng gộp.

### Plugin và công cụ trên máy chủ build

**Máy chủ Jenkins này chỉ có bộ lõi**, và nó dùng chung với dự án khác nên cài thêm plugin
không phải quyết định của kho này. Mười `Jenkinsfile` đã được viết để chạy trên đúng bộ ấy:

| Cần | Ghi chú |
|---|---|
| Git · Pipeline · **Credentials Binding** | `credentials-binding` cấp ràng buộc `file` và `gitUsernamePassword` — hai thứ duy nhất job deploy dùng |
| ~~Docker Pipeline~~ | **KHÔNG dùng.** `docker.build` / `docker.withRegistry` đã bị gỡ khỏi 8 pipeline đóng ảnh, thay bằng `docker` CLI — `Jenkinsfile:127` |
| ~~Kubernetes CLI~~ | **KHÔNG dùng.** `withKubeConfig` đã bị thay bằng ràng buộc `file` — `deploy/Jenkinsfile:23` |

**`BUILD_USER_ID` hôm nay luôn rỗng**, nên commit ghim thẻ ghi `Bấm bởi: khong-ro`
(`deploy/Jenkinsfile:159`): biến ấy do plugin `build-user-vars` cấp, plugin không có, và
`deploy/Jenkinsfile` cũng không bọc `wrap([$class: 'BuildUser'])`. Nhật ký triển khai vì thế
trả lời được *bản nào, lúc nào*, **không** trả lời được *ai*. Chưa vá — xem *Còn nợ*.

**Công cụ:** `go` 1.26+ · `buf` · `node` 22+ · `npm` · `python3` · một trình biên dịch C
(`go test -race` cần cgo) · `docker` có BuildKit. Riêng `vigov-deploy` thêm: `kubectl`,
`kustomize` bản độc lập (`kubectl -k` **không** có `kustomize edit`), `skopeo`.

Mỗi pipeline tự kiểm công cụ ở stage đầu và **dừng ngay** nếu thiếu, thay vì đổ giữa chừng
với một lỗi không đọc được: `Jenkinsfile:52` (7 công cụ) · `service-*/Jenkinsfile:48`
(`go buf docker gcc`) · `web-admin/Jenkinsfile:39` (`node npm docker`) ·
`deploy/Jenkinsfile:79` (`git kubectl kustomize skopeo`).

### Lượt chạy đầu tiên

Tám job đóng ảnh sẽ báo `Chưa lượt build nào ghi mốc đã đóng ảnh — dựng` và đóng **cả tám
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
