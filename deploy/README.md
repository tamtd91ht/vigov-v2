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

## 5. Biến môi trường — khai ở đâu, k8s cấp bằng gì

Ba câu hỏi, ba nguồn. Đừng trả lời câu này bằng nguồn của câu kia:

| Câu hỏi | Nguồn |
|---|---|
| **Biến nào tồn tại** | `.env.example` — sổ đăng ký. Không có dòng ở đó là biến không ai tìm ra được (luật 11, bất biến 6) |
| **Ai đọc nó** | `core/config` — gói **DUY NHẤT** gọi `os.Getenv`. Mọi nơi khác nhận `config.Config` đã kiểu hoá |
| **k8s cấp bằng gì** | bảng dưới đây — đó là fact tệp này sở hữu, không suy ra được từ hai nguồn kia |

**Một tên, hai cách viết.** Dấu gạch ngang hợp lệ trong **key** của ConfigMap/Secret nhưng
**không** hợp lệ trong tên biến môi trường của container — shell chỉ cho `A-Z`, `0-9`, `_`:

| Ở đâu | Cách viết | Ví dụ |
|---|---|---|
| Key ConfigMap / Secret | `CHỮ-HOA-GẠCH-NGANG` | `ELASTICSEARCH-INDEX-PREFIX` |
| Env var của container, và Go | `CHỮ_HOA_GẠCH_DƯỚI` | `ELASTICSEARCH_INDEX_PREFIX` |

`-` → `_`, **không gì khác**. Lệch nhau ở bất cứ đâu ngoài dấu phân cách là **hai tên**.

### Quy tắc quyết định — ba câu, theo đúng thứ tự

**ConfigMap và Secret chỉ giữ khoá QUAN TRỌNG.** Mọi khoá còn lại lấy mặc định, hoặc là hằng
số viết thẳng trong `deployment.yaml`.

| # | Câu hỏi | Nếu ĐÚNG |
|---|---|---|
| 1 | In ra một dòng log thì **có đau không**? | **Secret** |
| 2 | Giá trị **khác nhau giữa staging và prod** không? | **ConfigMap** |
| 3 | Không cả hai | **Không vào ConfigMap/Secret.** Hằng số của manifest (`env: value:`), hoặc để `config.Load` lấy mặc định |

Câu 1 không phải "có nhạy cảm không": một DSN có mật khẩu là **Secret** dù nó trông như một
địa chỉ. Bốn biến DSN/khoá mang kiểu `secret.DSN`/`secret.Secret` trong Go đúng vì lý do ấy —
chúng từ chối tự in ra.

Câu 3 là câu hay bị bỏ qua, và nó có giá: **mỗi dòng thừa trong ConfigMap/Secret là một dòng
người vận hành phải đọc rồi tự hỏi mình có quên đặt không.** Một sổ 16 dòng mà 10 dòng chỉ
lặp lại mặc định của mã là một sổ không ai tin nữa — và ngày thiếu một khoá thật thì nó lẫn
giữa chín khoá không quan trọng.

### Bảng map — 16 biến

| Biến | Bắt buộc | Ở đâu | Hình dạng / mặc định |
|---|---|---|---|
| `DATABASE_DSN` | **có, mọi môi trường** | **Secret** `bi-mat-<dịch vụ>` | DSN, phần host **được phép nhiều host** |
| `GRPC_CALLER_KEY` | **có, mọi môi trường** | **Secret** `bi-mat-<dịch vụ>` | chuỗi khoá. Rỗng = cổng gRPC trả lời bất kỳ ai (ADR 0025) |
| `SESSION_SIGNING_KEYS` | **có ở staging/prod** | **Secret** `bi-mat-<dịch vụ>` | danh sách phẩy, **≥ 2 khoá ở prod** để xoay được mà không đăng xuất toàn bộ cán bộ |
| `REDIS_DSN` | không | **Secret** `bi-mat-<dịch vụ>` | DSN. Hôm nay chỉ `platform` cần |
| `RABBITMQ_DSN` | không | **Secret**, ngày bật | DSN |
| `ELASTICSEARCH_API_KEY` | không | **Secret**, ngày bật | chuỗi khoá |
| `ENV` | **có, mọi môi trường** | **ConfigMap** `cau-hinh-chung` | `dev` · `staging` · `prod`. Khác ba giá trị này là `config.Load` từ chối |
| `TENANT_CACHE_TTL` | không | **ConfigMap** `cau-hinh-chung` | **đang khác nhau thật**: staging `10s`, prod `30s`. Mặc định của mã là `30s`; dài hơn 1 phút thì `config.CanhBao()` kêu |
| `ELASTICSEARCH_ADDRS` | không | **ConfigMap**, ngày bật | **danh sách phẩy** `http://host:9200,http://host:9200` — địa chỉ cụm khác nhau giữa hai môi trường |
| `RABBITMQ_EXCHANGE` | không | **ConfigMap** *nếu* hai môi trường đặt tên khác nhau; giống nhau thì để mặc định | tên exchange |
| `LISTEN_ADDR` | không | `env: value:` trong `deployment.yaml` | `:8080`. **Bốn dịch vụ BẮT BUỘC phải có dòng này** — xem cảnh báo dưới bảng |
| `PLATFORM_GRPC_ADDR` | không, nhưng **từ chối tại chỗ dùng** | `env: value:` trong `deployment.yaml` | `platform:9090` — DNS nội cụm, giống nhau mọi môi trường |
| `IDENTITY_GRPC_ADDR` | không, nhưng **từ chối tại chỗ dùng** | `env: value:` trong `deployment.yaml` | `identity:9090` — DNS nội cụm, giống nhau mọi môi trường |
| `GRPC_LISTEN_ADDR` | không | **không khai ở đâu cả** | mặc định `:9090` trong `config.Load` |
| `ELASTICSEARCH_INDEX_PREFIX` | không | **không khai ở đâu cả** | mặc định rỗng |
| `DANGEROUS_AUTH_BYPASS` | không | **không khai ở đâu cả, có chủ ý** | `config.Load` **từ chối khởi động** nếu nó bật ở `ENV=prod` (luật 8, bất biến 7). Không khai là cách chắc nhất |

⚠ **`LISTEN_ADDR` không phải một dòng thừa ở `comms` · `documents` · `finance` · `petitions`.**
Bốn dịch vụ ấy gọi `cfg.ListenAddrHoac(":8087")` chứ không đọc `cfg.ListenAddr`, nên **không
khai là tiến trình nghe ở cổng khác 8080** — rồi probe `httpGet` cổng 8080 không ai trả lời và
NetworkPolicy chỉ mở 8080/3000 từ ingress. Hai thứ gãy cùng lúc và cả hai đều im.

**Bắt buộc quyết theo TỪNG biến, có lý do viết bên cạnh.** Đánh dấu bắt buộc cho mọi biến mới
là sai lầm làm cả tám dịch vụ không khởi động được trên máy chưa đặt thêm bốn giá trị, để bảo
vệ đoạn mã chưa tồn tại. Năm biến RabbitMQ/Elasticsearch **cố ý** không bắt buộc: chưa mã nào
nối tới chúng, và cái đầu tiên thật sự cần sẽ **từ chối theo tên** tại chỗ nối.

### Hôm nay thực tế có bao nhiêu khoá

| Nơi | Khoá | Ai tạo |
|---|---|---|
| ConfigMap `cau-hinh-chung` | **2**: `ENV` · `TENANT_CACHE_TTL` | `configMapGenerator` ở `overlays/<mt>/kustomization.yaml` |
| Secret `bi-mat-platform` | **4**: `DATABASE_DSN` · `REDIS_DSN` · `GRPC_CALLER_KEY` · `SESSION_SIGNING_KEYS` | `kubectl create secret` bằng tay, mục 4 |
| Secret `bi-mat-<sáu dịch vụ kia>` | **3**: `DATABASE_DSN` · `GRPC_CALLER_KEY` · `SESSION_SIGNING_KEYS` | ″ |
| `env: value:` trong `deployment.yaml` | `LISTEN_ADDR` · `PLATFORM_GRPC_ADDR` · `IDENTITY_GRPC_ADDR` | nằm trong kho, đi qua review |
| `web-admin` | **không có `envFrom`** | Next.js không dùng `core/config`; nó đọc cấu hình theo tên miền **tại runtime** (luật 1, bất biến 10) |

Sáu biến còn lại không nằm ở đâu cả, và đó là trạng thái đúng — ba biến lấy mặc định của mã,
ba biến chờ ngày RabbitMQ/Elasticsearch được nối.

### Hai lỗ phải biết TRƯỚC khi bật RabbitMQ / Elasticsearch

1. **Năm biến ấy hôm nay không có nguồn nào cấp** — không nằm trong `cau-hinh-chung`, không
   nằm trong `bi-mat-*`. Đúng với hiện trạng (chưa mã nào nối), nhưng ngày nối thì phải thêm
   `RABBITMQ_EXCHANGE` · `ELASTICSEARCH_ADDRS` · `ELASTICSEARCH_INDEX_PREFIX` vào
   `configMapGenerator`, và `RABBITMQ_DSN` · `ELASTICSEARCH_API_KEY` vào `bi-mat-<dịch vụ>`.
2. **KAFKA CHƯA CÓ BIẾN NÀO.** ADR 0010 đã chốt Kafka mang sự kiện giữa các service, nhưng
   `core/events.Publisher` còn là interface thuần, chưa gắn hạ tầng — nên không có dòng nào
   trong `.env.example` và không có trường nào trong `config.Config`. Đặt tên cho nó là
   **STOP CONDITION của luật 11 (câu 1)**: tên phải nói **VAI TRÒ** (`KAFKA_EVENT_ADDRESS`),
   không nói cụm (`KAFKA_02_ADDRESS`), và **ai cấp — ConfigMap hay Secret — là quyết định của
   chủ cụm**. Đừng viết `os.Getenv("KAFKA_…")` trước khi có câu trả lời ấy;
   `hooks/env_contract_guard.py` chặn lần ghi đó.

### Tách VAI TRÒ khỏi CỤM — cơ chế trả tiền cho chính nó

Mã chỉ biết vai trò. Manifest mới chọn cụm vật lý:

```yaml
env:
  - name: KAFKA_EVENT_ADDRESS        # VAI TRÒ — tên duy nhất mã biết
    valueFrom:
      configMapKeyRef:
        name: vigov-ha-tang
        key: KAFKA-02-ADDRESS        # CỤM — chọn ở đây, đổi ở đây
```

Chuyển tải log từ kafka-02 sang kafka-05 khi ấy là **một dòng trong một tệp**. Cũng việc đó
với `KAFKA_02_ADDRESS` nằm trong Go là: sửa mã, review, dựng lại **mọi** ảnh có đọc nó, phát
hành cùng lúc, và hy vọng không sót cái nào.

### Ba điều cấm

| Cấm | Vì sao |
|---|---|
| Số thứ tự cụm trong tên mã đọc (`KAFKA_02_ADDRESS`, `REDIS_1_DSN`) | Hàn một tải công việc vào một cụm. Chuyển đi thành một lần sửa mã + phát hành mọi dịch vụ đọc nó |
| **Cắt danh sách địa chỉ hoặc DSN để giữ một host** | Đúng-trông-như-đúng suốt thời gian còn một node, sai im lặng vào đúng ngày lên HA. Đưa nguyên giá trị cho driver hiểu nhiều host (`pgx` hiểu) |
| Giá trị **riêng của một xã** trong biến môi trường | Luật 1, bất biến 10: môi trường chỉ mang hằng số **toàn nền tảng**. Giá trị theo xã đọc tại runtime từ sổ đăng ký của `platform` |

Bảng 16 biến ở trên được `tools/check_env_map.py` đối chiếu với `.env.example` và
`core/config/config.go` trong `make check`: thêm một biến mà quên cập nhật bảng là **đỏ**.

## 6. Sửa manifest sau khi đã chạy — **cái bẫy của mô hình này**

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

## 7. Bốn điều dễ hiểu sai

| Điều | Sự thật |
|---|---|
| "Mỗi xã một namespace" | **Không.** Cùng một pod phục vụ mọi xã; xã phân giải từ `Host` trong từng request (luật 1, bất biến 3). Thêm xã = thêm bản ghi DNS + một hàng trong sổ đăng ký của `platform`, không sửa manifest |
| "Cần Job di trú CSDL" | **Không.** Di trú chạy trong tiến trình lúc khởi động, có advisory lock (`core/migrate.Chay`). Vì thế `startupProbe` rộng tay — replica thứ hai **đợi** replica thứ nhất |
| "`/healthz` xanh nghĩa là hệ thống ổn" | **Không.** Nó nằm ngoài chuỗi phân giải xã có chủ ý, nên vẫn xanh khi CSDL hoặc `platform` hỏng. Bắt sự cố đó bằng **giám sát tỉ lệ 404** từ `TenantMiddleware`, không bằng probe |
| "NetworkPolicy là tuỳ chọn" | **Không.** gRPC 9090 dùng `insecure.NewCredentials()` — không TLS, không xác thực. NetworkPolicy là biên duy nhất giữ danh bạ xã của toàn hệ thống |

## 8. Ingress sinh từ hợp đồng REST

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

## 9. Chưa được chứng minh

| Việc | Trạng thái |
|---|---|
| 10 job trên Jenkins | **chưa dựng.** `buf`, `node`, `npm`, `python3` chưa từng được chứng minh có trên máy chủ này — `go`, `docker`, `make`, `gcc` thì đã, qua lượt chạy thật 21/09/2026 của kho `vihat-miniapp` |
| Manifest qua API server thật | **chưa.** `kubectl kustomize` chỉ chứng minh YAML dựng được, không chứng minh máy chủ chấp nhận |
| Đường dẫn kubeconfig | **giả định** — lấy từ `omicrm` trên cùng máy chủ |
| Đóng êm khi `SIGTERM` | **xong 22/09/2026** — cả sáu dịch vụ Go `signal.Notify` + `srv.Shutdown`, `terminationGracePeriodSeconds: 45` > ngữ cảnh 20 giây |
