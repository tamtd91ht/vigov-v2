# `deploy/` — đưa ViGov lên Kubernetes

Tám job đóng ảnh dừng ở Harbor. **Job `vigov-deploy` là thứ duy nhất chạm cụm.**

| Thư mục | Nội dung |
|---|---|
| `cluster/` | Cài **một lần**: namespace, quyền của Jenkins trên cụm |
| `base/` | Hình dạng của từng đơn vị — không namespace, không thẻ ảnh |
| `overlays/<mt>/` | Namespace, cấu hình theo môi trường. Thẻ ảnh ở đây **chỉ dùng cho lần cài đầu** |
| `Jenkinsfile` | Job triển khai — nơi duy nhất gọi `kubectl` |

## 1. Cấu trúc git

**Một nhánh `main`.** Nhánh không quyết định môi trường — tham số `MT` của job triển khai
quyết định. Khác mẫu `omicrm` (`dev`/`stg`/`master`) vì đây là **monorepo 8 ảnh**: một nhánh
môi trường sẽ đóng lại cả 8 ảnh cho một thay đổi chạm một dịch vụ.

| Câu hỏi | Trả lời ở đâu |
|---|---|
| Ảnh này sinh từ mã nào | **thẻ ảnh = commit 12 ký tự** của `main`. Thẻ di động (`latest`, `main`) bị cấm |
| Bản nào đang chạy trên cụm | `kubectl -n vigov-<mt> get deploy -o wide` — **hỏi cụm**, không hỏi git |
| Ai đưa bản đó lên, lúc nào | **lịch sử build của job `vigov-deploy`** (`description` mỗi lượt) |

⚠ **Nhật ký triển khai bị cắt sau 200 lượt** (`buildDiscarder`). Đó là cái giá của việc bỏ
commit ghim thẻ — ghi ra đây chứ không giấu. Muốn giữ vĩnh viễn thì phải có một sổ ngoài
Jenkins, và đó là một quyết định chưa ai đưa ra.

Mã nguồn đi thẳng vào `main`, không nhánh phụ. Nhánh chỉ mở khi chủ dự án quyết.

## 2. Dựng 10 job trên Jenkins

Jenkins **không tự tìm ra** mười `Jenkinsfile` nằm rải trong kho. Tạo 10 job kiểu
**Pipeline**, tất cả trỏ về cùng một kho, khác nhau đúng ô **Script Path**.

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
| `vigov-deploy` | `deploy/Jenkinsfile` | **không tự động — chỉ bấm tay** |

`New Item` → tên → **Pipeline** → OK, rồi:

| Mục trong form | Điền gì |
|---|---|
| General | **bỏ trống**. `timeout`, `buildDiscarder`, `disableConcurrentBuilds` đã khai trong Jenkinsfile — khai lại ở UI là hai nguồn sẽ lệch, và bản lỏng hơn là bản chạy |
| Build Triggers | ra được Internet → webhook GitHub; không → `Poll SCM` `H/5 * * * *`. `vigov-deploy` **không tick gì** |
| Pipeline → Definition | `Pipeline script from SCM` |
| SCM | `Git` · URL kho · credential `git-vigov` · Branch `*/main` |
| Additional Behaviours | **để trống** — xem ba điều cấm dưới |
| Script Path | lấy từ bảng trên |
| Lightweight checkout | **BỎ TICK** |

### Ba thứ tuyệt đối không bật

| Không bật | Vì sao |
|---|---|
| `Shallow clone` | Cơ chế dựng lại so `git diff <mốc>..HEAD`, mốc là commit của lượt đóng ảnh trước (`mocDaDongAnh()` cuối mỗi Jenkinsfile). Clone nông không chứa commit đó → **dựng lại mọi lần**, mỗi push thành tám lượt đóng ảnh |
| `Sparse checkout` | Bảy dịch vụ Go lấy ngữ cảnh build ở **gốc kho**: `go.mod` của chúng `replace core => ../core`. Thiếu `core/` là `docker build` đổ |
| `Lightweight checkout` | Chỉ kéo riêng tệp Jenkinsfile qua API, không tạo cây làm việc — mà `canDungLai()` chạy `git` thật ở stage `Chuẩn bị` |

### Vì sao 10 job, không phải một job có tham số `DICH_VU`

Mốc "commit đã thành ảnh" được ghi vào **`description` của lượt build**, và mỗi job có lịch
sử riêng. Gộp tám đơn vị vào một job thì tám mốc chen nhau trong **một** dòng lịch sử: job
tìm ngược sẽ nhặt phải mốc của dịch vụ khác rồi kết luận "không có gì đổi" cho một dịch vụ
vừa bị sửa. Một job cho mỗi dịch vụ là **nơi lưu trạng thái** của cơ chế, không phải để UI gọn.

### Credentials — **một** mục duy nhất

| ID | Kiểu | Dùng ở |
|---|---|---|
| `git-vigov` | Username with password (PAT) | ô `SCM` của cả 10 job |

Hai thứ còn lại **không** đi qua credentials của Jenkins, vì máy chủ này chỉ có bộ plugin lõi
và dùng chung với dự án khác:

- **Harbor** — phiên `docker login` sẵn của user `jenkins` (`~jenkins/.docker/config.json`).
  `skopeo` đọc cùng tệp ấy qua `--authfile`.
- **kubeconfig** — **tệp trên đĩa máy chủ**, hằng số `KUBECONFIG` ở đầu `deploy/Jenkinsfile`.
  Giá trị hiện tại `/u01/rancher/rancher-omi.yaml` **lấy từ dự án `omicrm` trên chính máy chủ
  ấy** — cụm khác thì sửa đúng dòng đó. Stage đầu đọc thử tệp và **dừng** nếu không có.

Trong cụm k8s vẫn có một Secret **tên** `harbor-vigov` (`imagePullSecrets`). Trùng tên, khác
vật — đừng gộp.

### Plugin và công cụ

| Cần | Ghi chú |
|---|---|
| Git · Pipeline | bộ lõi |
| ~~Docker Pipeline~~ · ~~Kubernetes CLI~~ · ~~build-user-vars~~ · ~~Credentials Binding~~ | **không dùng cái nào.** `docker` CLI thay `docker.build`; `--kubeconfig`/`KUBECONFIG` thay `withKubeConfig`; `currentBuild.getBuildCauses()` thay `BUILD_USER_ID` |

Không xác định được người bấm ⇒ `vigov-deploy` **dừng** ở stage `Kiểm tham số`, trước khi
chạm cụm. Job không có trigger tự động nên mọi lượt hợp lệ đều do một người đã đăng nhập bấm
— gặp lần đỏ ấy thì kiểm xem có job/timer/script nào gọi hộ không, đừng nới hàm.

**Công cụ trên máy chủ:** `go` 1.26+ · `buf` · `node` 22+ · `npm` · `python3` · một trình
biên dịch C (`go test -race` cần cgo) · `docker` có BuildKit. Riêng `vigov-deploy` thêm
`kubectl` và `skopeo`. Mỗi pipeline tự kiểm ở stage đầu và **dừng ngay** nếu thiếu:
`Jenkinsfile:52` · `service-*/Jenkinsfile:48` · `web-admin/Jenkinsfile:39` ·
`deploy/Jenkinsfile` stage `Kiểm công cụ`.

**Lượt chạy đầu tiên:** tám job đóng ảnh báo *"Chưa lượt build nào ghi mốc đã đóng ảnh —
dựng"* và đóng cả tám ảnh. Đúng, không phải lỗi. `vigov-deploy` khai tham số bên trong
Jenkinsfile nên lượt đầu chạy không có ô nhập và đỏ ở bước kiểm `THE`; lượt sau mới hiện form.

## 3. Bảy đơn vị, và thứ tự đưa lên

Manifest được thêm cho một dịch vụ vào ngày nó có endpoint đầu tiên, đọc từ
`kb/20-contracts/openapi.json` chứ không từ cảm giác.

| # | Đơn vị | Tuyến REST | Đi trước vì |
|---|---|---|---|
| 1 | `platform` | 0 (nhưng **là** gRPC phân giải xã) | Sáu đơn vị kia quay số cổng 9090 của nó. Chưa có nó thì **mọi xã trả 404** |
| 2 | `identity` | 15 | Bốn dịch vụ dưới đổi cookie lấy principal qua gRPC 9090 của nó. Chưa có nó thì **mọi tuyến có kiểm quyền trả 401 cho một phiên hợp lệ** — pod xanh, probe xanh, không gì báo |
| 3–6 | `comms` · `documents` · `finance` · `petitions` | 1 · 1 · 3 · 3 | độc lập nhau, thứ tự tuỳ |
| 7 | `web-admin` | — | **Sau cùng, có chủ ý.** Đưa bề mặt cán bộ lên trước khi API trả lời được nghĩa là một màn hình lỗi mang tên một cơ quan nhà nước |

Không có manifest, **có cân nhắc**: `reporting` (0 tuyến REST) · `platform-admin` (chưa có
`Dockerfile`) · `citizen-app` (chạy trong Zalo Mini App, không thành pod).

Mỗi lượt là một lần bấm `vigov-deploy` với `DICH_VU` + `THE` + `MT`. Job kiểm ảnh có thật
trong Harbor, `kubectl set image`, đợi `rollout status`, và `rollout undo` nếu đỏ.

## 4. Cài lần đầu

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

# BA KHOÁ LÀ ĐIỀU KIỆN KHỞI ĐỘNG, không phải tuỳ chọn: `core/config.Load` gom đủ tên còn
# thiếu rồi thoát. SESSION_SIGNING_KEYS nằm trong danh sách kể cả khi dịch vụ không tự phát
# phiên — Load đòi nó ở staging/prod.
for s in comms documents finance petitions; do
  kubectl -n vigov-prod create secret generic bi-mat-$s \
    --from-literal=DATABASE_DSN='postgres://...' \
    --from-literal=GRPC_CALLER_KEY='...' \
    --from-literal=SESSION_SIGNING_KEYS='...'
done

kubectl -n vigov-prod create secret tls vigov-wildcard-tls --cert=... --key=...

kubectl apply -k deploy/overlays/prod
```

`apply -k` dựng Deployment với thẻ `CHUA-TRIEN-KHAI-LAN-NAO`, tức **bảy pod ngồi
`ImagePullBackOff`** — đúng như thiết kế, không phải lỗi. Chạy `vigov-deploy` bảy lượt theo
thứ tự ở mục 3 để đặt thẻ thật.

Ba thứ dễ sót, cả ba đều hỏng ở chỗ cách xa nguyên nhân:

| Sót | Triệu chứng |
|---|---|
| Secret `harbor-vigov` | `ImagePullBackOff`, không nói gì về quyền |
| `GRPC_CALLER_KEY` / `SESSION_SIGNING_KEYS` | `CrashLoopBackOff` — `core/config.Load` gom đủ tên còn thiếu rồi thoát |
| Secret TLS | Ingress lên bình thường, chỉ HTTPS đứt |

`SESSION_SIGNING_KEYS` cần **ít nhất hai khoá** ở prod — một khoá thì không xoay được mà
không đăng xuất toàn bộ cán bộ của mọi xã cùng lúc (`core/config.CanhBao`).

Bí mật vào cụm hôm nay bằng `kubectl create secret` tay. Bước kế tiếp khi thấy phiền:
**External Secrets Operator**. Manifest trong kho này khi ấy vẫn chỉ chứa **tên** khoá.

## 5. Sửa manifest sau khi đã chạy — **cái bẫy của mô hình này**

`overlays/<mt>/kustomization.yaml` **không** ghi thẻ đang chạy. Chạy `apply -k` lên một
namespace đã có dịch vụ sẽ **đẩy cả bảy về `CHUA-TRIEN-KHAI-LAN-NAO`**, tức
`ImagePullBackOff` đồng loạt trên prod. Đó là cái giá của việc đổi sang `kubectl set image`.

Quy trình đúng:

```sh
# 1. Ghi lại thẻ đang chạy
kubectl -n vigov-prod get deploy \
  -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.spec.template.spec.containers[0].image}{"\n"}{end}'

# 2. Áp manifest mới
kubectl apply -k deploy/overlays/prod

# 3. Đặt lại từng thẻ vừa ghi (hoặc chạy lại vigov-deploy cho từng dịch vụ)
kubectl -n vigov-prod set image deploy/<tên> server=<ảnh>:<thẻ>
```

Tên thùng là `server` với sáu dịch vụ Go, `web` với `web-admin`.

## 6. Bốn điều dễ hiểu sai

| Điều | Sự thật |
|---|---|
| "Mỗi xã một namespace" | **Không.** Cùng một pod phục vụ mọi xã; xã phân giải từ `Host` trong từng request (luật 1, bất biến 3). Thêm xã = thêm bản ghi DNS + một hàng trong sổ đăng ký của `platform`, không sửa manifest |
| "Cần Job di trú CSDL" | **Không.** Di trú chạy trong tiến trình lúc khởi động, có advisory lock (`core/migrate.Chay`). Vì thế `startupProbe` rộng tay — replica thứ hai **đợi** replica thứ nhất |
| "`/healthz` xanh nghĩa là hệ thống ổn" | **Không.** Nó nằm ngoài chuỗi phân giải xã có chủ ý, nên vẫn xanh khi CSDL hoặc `platform` hỏng. Bắt sự cố đó bằng **giám sát tỉ lệ 404** từ `TenantMiddleware`, không bằng probe |
| "NetworkPolicy là tuỳ chọn" | **Không.** gRPC 9090 dùng `insecure.NewCredentials()` — không TLS, không xác thực. NetworkPolicy là biên duy nhất giữ danh bạ xã của toàn hệ thống |

## 7. Ingress sinh từ hợp đồng REST

Chốt 21/09/2026: bảng định tuyến **được sinh ra** từ `kb/20-contracts/openapi.json`, nên
"thêm tuyến trong Go" và "tuyến ấy đi tới đúng dịch vụ" không còn là hai việc rời nhau.

| Tệp | Vai trò |
|---|---|
| `tools/ingress/` | Bộ sinh. `go run ./tools/ingress`, đã gắn vào mục `kb` của `Makefile` sau `apidoc` |
| `base/mang/ingress.yaml` | **SINH RA.** Giống nhau ở mọi môi trường nên nó thuộc `base/` |
| `overlays/<mt>/ingress-moi-truong.yaml` | Bản vá **JSON6902** cho host + TLS |

**Vì sao là JSON6902 chứ không phải một `Ingress` đầy đủ trong overlay:** `spec.rules` là
danh sách không có khoá trộn, nên một strategic-merge patch chỉ cần *nhắc tới* `rules` là
**thay cả danh sách** — xoá sạch bảng sinh ra, và `kustomize` không kêu một tiếng.

**Gom tới mức TÀI NGUYÊN, không hơn.** Không gom theo tiền tố có gạch nối: `task-blocs` là
`identity` trong khi `task-priorities` và `task-types` là `petitions`.

**Một tuyến không xác định được dịch vụ chủ thì bộ sinh DỪNG**, không mặc định về `identity`
(luật 1: không có mặc định trên đường cách ly). `go test ./tools/ingress` đối chiếu tệp trên
đĩa với hợp đồng: xoá một luật hay đổi một backend là **đỏ**.

## 8. Chưa được chứng minh

| Việc | Trạng thái |
|---|---|
| 10 job trên Jenkins | **chưa dựng.** `buf`, `node`, `npm`, `python3` chưa từng được chứng minh có trên máy chủ này — `go`, `docker`, `make`, `gcc` thì đã, qua lượt chạy thật 21/09/2026 của kho `vihat-miniapp` |
| Manifest qua API server thật | **chưa.** `kubectl kustomize` chỉ chứng minh YAML dựng được, không chứng minh máy chủ chấp nhận |
| Đường dẫn kubeconfig | **giả định** — lấy từ `omicrm` trên cùng máy chủ |
| Đóng êm khi `SIGTERM` | **xong 22/09/2026** — cả sáu dịch vụ Go `signal.Notify` + `srv.Shutdown`, `terminationGracePeriodSeconds: 45` > ngữ cảnh 20 giây |
