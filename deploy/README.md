# `deploy/` — sổ tay đưa ViGov lên Kubernetes

Đọc từ trên xuống, làm theo thứ tự. Tám job đóng ảnh dừng ở Harbor; **job `vigov-deploy` là
thứ duy nhất chạm cụm.**

| Thư mục | Nội dung |
|---|---|
| `cluster/` | Cài **một lần**: namespace, quyền của Jenkins trên cụm |
| `base/` | Hình dạng của từng đơn vị — không namespace, không thẻ ảnh |
| `overlays/<mt>/` | Namespace, cấu hình theo môi trường. Thẻ ảnh ở đây **chỉ dùng cho lần cài đầu** |
| `Jenkinsfile` | Job triển khai — nơi duy nhất gọi `kubectl` |

## 0. Đọc trước khi bấm

**Chưa lượt nào chạy thật.** Chưa job Jenkins nào được dựng, chưa manifest nào đi qua một API
server thật, `deploy/Jenkinsfile` chưa được Jenkins phân tích cú pháp lần nào. Danh sách đầy
đủ ở **mục 10** — đọc trước, không phải sau.

**Đi staging trước prod, không ngoại lệ.** Phép kiểm cách ly hai xã (luật 1) là phép kiểm mà
staging tồn tại để chạy; một lượt deploy prod không chứng minh lại được nó.

**Ba thứ phải có sẵn:** cụm k8s · wildcard DNS · chứng thư TLS. Chủ dự án xác nhận đã có
ngày 22/09/2026.

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
commit ghim thẻ. Muốn giữ vĩnh viễn thì phải có một sổ ngoài Jenkins — quyết định chưa ai đưa ra.

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

### Vì sao `Pipeline` chứ không phải `Multibranch Pipeline`

Câu hỏi đúng, và câu trả lời **phụ thuộc vào quyết định nhánh ở mục 1** — không phải vào việc
loại job nào tốt hơn:

| | `Pipeline` (đang dùng) | `Multibranch Pipeline` |
|---|---|---|
| Nó giải bài toán gì | một job = một `Jenkinsfile` cố định trên `main` | **tự phát hiện nhánh** và tạo một job con cho mỗi nhánh |
| Với kho một nhánh | vừa đủ | sinh đúng **một** job con, cộng thêm một tầng quét — trả tiền cho một tính năng không dùng |
| Với `dev`/`stg`/`master` như mẫu omicrm | phải tạo tay từng job cho từng nhánh | **đây mới là lựa chọn đúng** |

ViGov chốt **main-only** (mục 1), nên Multibranch không mua được gì. Hai lý do cụ thể hơn,
mỗi cái ứng với một cơ chế đang chạy:

1. **Mốc dựng lại nằm trong LỊCH SỬ BUILD của job.** `mocDaDongAnh()` đi ngược
   `currentBuild.previousBuild` tìm dòng `anh-tu-commit:` trong `description`
   (`service-*/Jenkinsfile`, cuối tệp). Một job `Pipeline` là hạng mục cấp cao nhất, lịch sử
   của nó không ai dọn. Job con của Multibranch thì gắn vào **nhánh**: đổi tên nhánh, hoặc
   chính sách *orphaned item* dọn nó, là mất lịch sử — và mất mốc nghĩa là **dựng lại cả tám
   ảnh**. Hỏng về phía an toàn, nhưng toàn bộ việc lọc biến mất mà không ai được báo.
2. **`vigov-deploy` KHÔNG ĐƯỢC tự chạy.** Multibranch build ngay khi phát hiện nhánh và mỗi
   lần push, nên muốn nó im thì phải thêm một thuộc tính chặn trigger. Một job triển khai mà
   mặc định là *tự chạy rồi chặn lại* thì cái sai nằm ở phía nguy hiểm: quên một ô là cụm bị
   đụng mà không ai bấm. Job `Pipeline` không khai trigger thì **không bao giờ tự chạy** —
   và kiểm "ai bấm" ở stage `Kiểm tham số` dựa đúng vào tính chất ấy.

**Ngày nào đổi sang nhánh-là-môi-trường thì đổi luôn sang Multibranch** — lúc ấy nó đúng, và
mục 1 phải sửa cùng lượt. Đừng đổi một nửa.

### Credentials — **một** mục duy nhất

| ID | Kiểu | Dùng ở |
|---|---|---|
| tuỳ đặt (gợi ý `git-vigov`) | Username with password (PAT) | ô `SCM` của cả 10 job |

**ID đặt tên gì cũng được** — bản tài liệu trước viết là phải khớp từng ký tự, và điều đó
**không còn đúng** kể từ khi `deploy/Jenkinsfile` bỏ `withCredentials`: hôm nay **không một
`Jenkinsfile` nào trong kho gọi tên một credential**. Nó chỉ được chọn trong ô SCM của từng
job, nên tên là việc của người dựng job. Lượt chạy thật 23/09/2026 dùng `huypdk` và đi qua
bình thường.

Hai thứ còn lại **không** đi qua credentials của Jenkins, vì máy chủ này chỉ có bộ plugin lõi
và dùng chung với dự án khác:

- **Harbor** — phiên `docker login` sẵn của user `jenkins` (`~jenkins/.docker/config.json`).
  `skopeo` đọc cùng tệp ấy qua `--authfile`.
- **kubeconfig** — **tệp trên đĩa máy chủ**, hằng số `KUBECONFIG` ở đầu `deploy/Jenkinsfile`.
  `/u01/rancher/rancher-vigov.yaml` — **chủ dự án chỉ, 23/09/2026**. KHÔNG phải
  `rancher-omi.yaml` của dự án `omicrm` nằm cùng thư mục: hai tệp chỉ khác bốn ký tự, nên đây
  là chỗ một lần chép nhầm đưa lượt deploy của ViGov vào cụm của dự án khác. Stage đầu đọc thử
  tệp và **dừng** nếu không có, đồng thời in `kubectl config current-context` ra log — thiếu
  tệp thì đỏ ngay, sai cụm thì có một dòng đọc được để nhận ra.

Trong cụm k8s vẫn có một Secret **tên** `harbor-vigov` (`imagePullSecrets`). Trùng tên, khác
vật — đừng gộp.

### Plugin và công cụ

| Cần | Ghi chú |
|---|---|
| Git · Pipeline | bộ lõi |
| ~~Docker Pipeline~~ · ~~Kubernetes CLI~~ · ~~build-user-vars~~ · ~~Credentials Binding~~ | **không dùng cái nào.** `docker` CLI thay `docker.build`; `KUBECONFIG` thay `withKubeConfig`; `currentBuild.getBuildCauses()` thay `BUILD_USER_ID` |

Không xác định được người bấm ⇒ `vigov-deploy` **dừng** ở stage `Kiểm tham số`, trước khi
chạm cụm. Job không có trigger tự động nên mọi lượt hợp lệ đều do một người đã đăng nhập bấm
— gặp lần đỏ ấy thì kiểm xem có job/timer/script nào gọi hộ không, đừng nới hàm.

**Công cụ trên máy chủ:** `go` 1.26+ · `buf` · `node` 22+ · `npm` · `python3` **3.8+** · một trình
biên dịch C (`go test -race` cần cgo) · `docker` có BuildKit. Riêng `vigov-deploy` thêm
`kubectl` và `skopeo`. Mỗi pipeline tự kiểm ở stage đầu và **dừng ngay** nếu thiếu.

**Chạy `vigov-gate` TRƯỚC mọi job khác.** Nó là job duy nhất kiểm cả 7 công cụ và chạy
`make check`, nên nó biến "máy chủ build thiếu gì" thành **một** lần đỏ đọc được thay vì tám
lần đỏ rải rác.

Lượt `vigov-gate` ngày 24/09/2026 (commit `30aa72d`) đã chứng minh máy chủ có `go` 1.26.5,
`buf` 1.73.0, `node` 22.23.2, `npm` và `python3`. Phiên bản `python3` thì chưa ai đọc được.
Lượt đó đổ ở `make check` vì **`python` trên máy chủ là Python 2.7**: lỗi `SyntaxError:
Non-ASCII character` ở `tools/check_brain.py` là lỗi chỉ Python 2 báo. Vì vậy `Jenkinsfile`
gọi `make check PYTHON=python3` và dừng ở stage đầu nếu `python3` cũ hơn 3.8. CentOS/RHEL 7
cài sẵn 3.6. Gặp lần đỏ ấy thì cài bản mới hơn (vd. `python38`/`rh-python38`) và cho
`python3` trỏ tới nó. Đừng hạ yêu cầu của `tools/`.

**Lượt chạy đầu tiên:** tám job đóng ảnh báo *"Chưa lượt build nào ghi mốc đã đóng ảnh —
dựng"* và đóng cả tám ảnh. Đúng, không phải lỗi. `vigov-deploy` khai tham số bên trong
Jenkinsfile nên lượt đầu chạy không có ô nhập và đỏ ở bước kiểm `THE`; lượt sau mới hiện form.

## 3. Biến môi trường — khai ở đâu, k8s cấp bằng gì

Đọc mục này **trước** khi tạo Secret ở mục 4.

Ba câu hỏi, ba nguồn. Đừng trả lời câu này bằng nguồn của câu kia:

| Câu hỏi | Nguồn |
|---|---|
| **Biến nào tồn tại** | `.env.example` — sổ đăng ký. Không có dòng ở đó là biến không ai tìm ra được (luật 11, bất biến 6) |
| **Ai đọc nó** | `core/config` — gói **DUY NHẤT** gọi `os.Getenv`. Mọi nơi khác nhận `config.Config` đã kiểu hoá |
| **k8s cấp bằng gì** | bảng dưới đây — fact tệp này sở hữu, không suy ra được từ hai nguồn kia |

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
địa chỉ. Câu 3 là câu hay bị bỏ qua, và nó có giá: **mỗi dòng thừa trong ConfigMap/Secret là
một dòng người vận hành phải đọc rồi tự hỏi mình có quên đặt không.**

### Bảng map — 16 biến

| Biến | Bắt buộc | Ở đâu | Hình dạng / mặc định |
|---|---|---|---|
| `DATABASE_DSN` | **có, mọi môi trường** | **Secret** `bi-mat-<dịch vụ>` | DSN, phần host **được phép nhiều host** |
| `GRPC_CALLER_KEY` | **có, mọi môi trường** | **Secret** `bi-mat-<dịch vụ>` | chuỗi khoá. Rỗng = cổng gRPC trả lời bất kỳ ai (ADR 0025) |
| `SESSION_SIGNING_KEYS` | **có ở staging/prod** | **Secret** `bi-mat-<dịch vụ>` | danh sách phẩy, **≥ 2 khoá ở prod** để xoay được mà không đăng xuất toàn bộ cán bộ |
| `REDIS_DSN` | không ở `config.Load`, **nhưng xem cảnh báo dưới bảng** | **Secret** `bi-mat-<dịch vụ>` | DSN. **Năm** dịch vụ đọc nó — `platform` thì KHÔNG |
| `RABBITMQ_DSN` | không | **Secret**, ngày bật | DSN |
| `ELASTICSEARCH_API_KEY` | không | **Secret**, ngày bật | chuỗi khoá |
| `ENV` | **có, mọi môi trường** | **ConfigMap** `cau-hinh-chung` | `dev` · `staging` · `prod`. Khác ba giá trị này là `config.Load` từ chối |
| `ELASTICSEARCH_ADDRS` | không | **ConfigMap**, ngày bật | **danh sách phẩy** `http://host:9200,http://host:9200` |
| `RABBITMQ_EXCHANGE` | không | **ConfigMap** *nếu* hai môi trường đặt tên khác nhau | tên exchange |
| `LISTEN_ADDR` | không | `env: value:` trong `deployment.yaml` | `:8080`. **Bốn dịch vụ BẮT BUỘC phải có dòng này** — cảnh báo thứ hai dưới bảng |
| `PLATFORM_GRPC_ADDR` | không, nhưng **từ chối tại chỗ dùng** | `env: value:` trong `deployment.yaml` | `platform:9090` — DNS nội cụm, giống nhau mọi môi trường |
| `IDENTITY_GRPC_ADDR` | không, nhưng **từ chối tại chỗ dùng** | `env: value:` trong `deployment.yaml` | `identity:9090` — DNS nội cụm, giống nhau mọi môi trường |
| `GRPC_LISTEN_ADDR` | không | **không khai ở đâu cả** | mặc định `:9090` trong `config.Load` |
| `TENANT_CACHE_TTL` | không | **không khai ở đâu cả** | mặc định `30s` trong `config.Load`. Dài hơn 1 phút thì `config.CanhBao()` kêu |
| `ELASTICSEARCH_INDEX_PREFIX` | không | **không khai ở đâu cả** | mặc định rỗng |
| `DANGEROUS_AUTH_BYPASS` | không | **không khai ở đâu cả, có chủ ý** | `config.Load` **từ chối khởi động** nếu nó bật ở `ENV=prod` (luật 8, bất biến 7) |

⚠ **`REDIS_DSN` KHÔNG BẮT BUỘC Ở `config.Load` NHƯNG BẮT BUỘC Ở PROD.** Nó là kho chống trùng
yêu cầu (`core/idem`). Thiếu nó, `idemStore` là `nil` và mỗi tuyến hành xử theo `CheDoHong` nó
khai — mà **bảy lời gọi `idem.Required(idem.DongKhiHong)` trên sáu đường dẫn trả 503**:

```
POST /api/v1/staff              POST /api/v1/incoming-documents
POST /api/v1/disbursements      POST /api/v1/outgoing-documents
POST /api/v1/budget-sheets      POST /api/v1/budget-lines
```

`DongKhiHong` là lựa chọn đúng — đó là tiền, số văn bản đã phát hành, tài khoản cán bộ: một
bản trùng không rút lại được. Nhưng nó nghĩa là **deploy prod không có Redis = sáu đường dẫn
ấy chết từ ngày đầu**, trong khi pod xanh và probe xanh. Năm dịch vụ đọc `REDIS_DSN`:
`identity` · `documents` · `finance` · `petitions` · `comms`. **`platform` không đọc** — đưa
`REDIS_DSN` vào `bi-mat-platform` là một khoá không ai dùng.

⚠ **`LISTEN_ADDR` không phải dòng thừa ở `comms` · `documents` · `finance` · `petitions`.**
Bốn dịch vụ ấy gọi `cfg.ListenAddrHoac(":8087")` chứ không đọc `cfg.ListenAddr`, nên **không
khai là tiến trình nghe ở cổng khác 8080** — rồi probe `httpGet` cổng 8080 không ai trả lời và
NetworkPolicy chỉ mở 8080/3000 từ ingress. Hai thứ gãy cùng lúc và cả hai đều im.

**Bắt buộc quyết theo TỪNG biến.** Đánh dấu bắt buộc cho mọi biến mới là sai lầm làm cả tám
dịch vụ không khởi động được trên máy chưa đặt thêm bốn giá trị, để bảo vệ đoạn mã chưa tồn
tại. Năm biến RabbitMQ/Elasticsearch **cố ý** không bắt buộc: chưa mã nào nối tới chúng.

### Rà toàn bộ nguồn — ai đọc biến môi trường, và đọc bao nhiêu

Quét cả kho ngày 23/09/2026, không chỉ `core/config`:

| Nơi đọc | Số biến | Ghi chú |
|---|---|---|
| `core/config` — **7 dịch vụ Go** | **16** | Gói DUY NHẤT gọi `os.Getenv`. Quét cả kho: **không có `os.Getenv`/`os.LookupEnv` nào khác** trong mã Go ngoài `tools/` (chạy ở máy trạm, không thành pod) |
| `web-admin` — **mã ứng dụng** | **0** | Không một `process.env` nào trong `web-admin/src`. Nó gọi API bằng đường dẫn **tương đối** `/api/v1/…` trên cùng tên miền, nên không cần địa chỉ backend — và đó là lý do nó **không có `envFrom`** |
| `web-admin` — máy chủ Next standalone | 4, **đã nằm trong ảnh** | `NODE_ENV` · `NEXT_TELEMETRY_DISABLED` nung trong `Dockerfile`; `PORT=3000` · `HOSTNAME=0.0.0.0` đặt lại ở `deployment.yaml` |
| `platform-admin` | 1 — `NEXT_PUBLIC_PLATFORM_API` | **chưa triển khai**: không có `Dockerfile`, nên chưa có ảnh. Ngày dựng nó thì đây là một `NEXT_PUBLIC_*`, tức **ship trong bundle trình duyệt** — không bao giờ chứa bí mật (luật 8, bất biến 4) |
| `citizen-app` | 0 | Chạy trong Zalo Mini App, không thành pod |

**Tổng cộng, thứ cụm phải cấp: đúng 16 biến, và chỉ cho 6 pod Go.** `web-admin` không cần một
khoá nào.

### Đối tượng k8s phải tạo — tên và key chính xác

Giá trị anh tự điền. Tên và key thì **không được đổi**: manifest tham chiếu đúng những chuỗi này.

| Loại | Tên đối tượng | Key | Ai tạo |
|---|---|---|---|
| ConfigMap | `cau-hinh-chung` | `ENV` | **kustomize sinh — ĐỪNG tạo tay.** Nó mang hậu tố băm (`cau-hinh-chung-679b259276`) để đổi giá trị là đổi tên, ép pod khởi động lại. Sửa giá trị ở `overlays/<mt>/kustomization.yaml` |
| Secret | `bi-mat-platform` | `DATABASE_DSN` · `GRPC_CALLER_KEY` · `SESSION_SIGNING_KEYS` | người, mục 4 |
| Secret | `bi-mat-identity` | bốn khoá: ba khoá trên **+ `REDIS_DSN`** | ″ |
| Secret | `bi-mat-documents` | ″ | ″ |
| Secret | `bi-mat-finance` | ″ | ″ |
| Secret | `bi-mat-petitions` | ″ | ″ |
| Secret | `bi-mat-comms` | ″ | ″ |
| Secret (`docker-registry`) | `harbor-vigov` | k8s tự đặt `.dockerconfigjson` | ″ — `imagePullSecrets` của cả 7 Deployment |
| Secret (`tls`) | **`vigov-staging-tls`** ở staging · **`vigov-wildcard-tls`** ở prod | `tls.crt` · `tls.key` | ″ |

⚠ **Hai môi trường dùng HAI TÊN Secret TLS khác nhau** —
`overlays/staging/ingress-moi-truong.yaml:16` và `overlays/prod/ingress-moi-truong.yaml:27`.
Chép lệnh của prod sang staging là Ingress lên bình thường và **chỉ HTTPS đứt**. Host cũng
khác: `*.staging.vigov.vn` với `*.vigov.vn`.

⚠ **KEY PHẢI VIẾT GẠCH DƯỚI, KHÔNG PHẢI GẠCH NGANG — vì manifest dùng `envFrom`.**
Bảng "một tên, hai cách viết" ở đầu mục này nói key ConfigMap/Secret viết `CÓ-GẠCH-NGANG`, và
điều đó chỉ đúng khi env var được ánh xạ **tường minh** bằng `valueFrom.secretKeyRef` — nơi
tên biến được khai riêng. Ở đây cả 7 Deployment dùng `envFrom`, mà `envFrom` lấy **chính key
làm tên biến**. Một key `DATABASE-DSN` không phải định danh shell hợp lệ, nên k8s **bỏ qua nó
trong im lặng** (chỉ còn một Event `InvalidVariableNames` mà không ai đọc) — rồi pod chết với
`thiếu biến môi trường bắt buộc: DATABASE_DSN`, đúng cái tên người vận hành đang nhìn thấy
trong Secret. Ngày nào đổi sang `secretKeyRef` thì key mới được viết gạch ngang.

### Mẫu giá trị — hình dạng, không phải giá trị thật

Mọi thứ trong `<...>` anh tự điền. Không có giá trị thật nào trong kho này, và **không được
có** (luật 8, bất biến 1): một mẫu trông như cấu hình chạy được là một mẫu có người ship.

**`DATABASE_DSN`** — chuỗi kết nối PostgreSQL dạng URL, giống hệt dòng mẫu ở `.env.example`.
`sql.Open("pgx", …)` đọc nó.

| Phần | Điền gì |
|---|---|
| tài khoản | riêng cho **từng dịch vụ**, không dùng chung, không phải `postgres` |
| máy chủ + cổng | **cụm-hình-dạng ngay từ dòng đầu**: nhiều node thì `h1:5432,h2:5432` — `pgx` hiểu danh sách. Đừng cắt bớt để giữ một host (luật 11, cấm #5) |
| tên CSDL | **RIÊNG của từng dịch vụ** — xem cảnh báo ngay dưới |
| `sslmode` | `require` trở lên. `disable` là mật khẩu và dữ liệu công dân đi trần trên dây |

⚠ **SÁU DỊCH VỤ PHẢI CÓ SÁU CSDL KHÁC NHAU, và dùng chung thì KHÔNG CÓ GÌ BÁO.** Cả bảy dịch
vụ đều chạy `CREATE TABLE IF NOT EXISTS audit_log` trong `migrations/0001_init.sql` của chính
nó, và không chỗ nào đặt `search_path` — tức không có lớp schema ngăn cách. Trỏ hai dịch vụ
vào cùng một CSDL thì `IF NOT EXISTS` làm dịch vụ thứ hai **lặng lẽ bỏ qua** và hai dịch vụ
**ghi chung một bảng vết**: không lỗi, không cảnh báo, di trú xanh, pod xanh. Đó vừa là luật 2
cấm #2 (một đường đọc vòng qua hợp đồng), vừa là một sổ vết pháp lý trộn hai nguồn mà về sau
không tách lại được.

Phần duy nhất bắt buộc khác nhau giữa sáu DSN là **tên tài khoản và tên CSDL**:

```
…@<host>:5432/vigov_platform?sslmode=require
…@<host>:5432/vigov_identity?sslmode=require
…@<host>:5432/vigov_documents?sslmode=require
…@<host>:5432/vigov_finance?sslmode=require
…@<host>:5432/vigov_petitions?sslmode=require
…@<host>:5432/vigov_comms?sslmode=require
```

Đặt tên thế nào là tuỳ anh — điều bắt buộc là **sáu cái khác nhau**.

**`REDIS_DSN`** — dạng `redis://…`, có mật khẩu thì đặt ở phần thông tin đăng nhập, số DB ở
cuối đường dẫn (`/0`). Năm dịch vụ **dùng chung một Redis được**: khoá chống trùng đã mang
tiền tố riêng, nên tách bằng số DB hay để chung đều đúng.

**`GRPC_CALLER_KEY`** — một chuỗi ngẫu nhiên, **giống nhau ở cả sáu dịch vụ**: nó là khoá
chung để bên gọi chứng minh mình thuộc hệ thống (ADR 0025). Sinh một lần rồi dùng lại:

```sh
openssl rand -base64 48
```

**`SESSION_SIGNING_KEYS`** — danh sách ngăn bằng dấu phẩy, **khoá mới đứng trước**, ít nhất
hai khoá ở prod. Phải **giống nhau ở cả sáu dịch vụ**, nếu không thì phiên do dịch vụ này phát
ra dịch vụ kia không đọc được. Sinh bằng lệnh trên. Xoay khoá = thêm khoá mới vào **đầu** danh
sách và đẩy khoá cũ nhất ra khỏi danh sách, không phải thay tại chỗ:

```
<khoa-moi>,<khoa-truoc-do>
```

**`ENV`** — đúng một trong ba chuỗi: `dev` · `staging` · `prod`. Khác đi là `config.Load` từ chối.

**Secret TLS** — tạo từ tệp chứng thư, không phải từ chuỗi:

```sh
kubectl -n vigov-prod create secret tls vigov-wildcard-tls \
  --cert=/duong/dan/fullchain.pem --key=/duong/dan/privkey.pem
```

### Kafka chưa có biến nào

ADR 0010 đã chốt Kafka mang sự kiện giữa các service, nhưng `core/events.Publisher` còn là
interface thuần, chưa gắn hạ tầng — nên không có dòng nào trong `.env.example` và không có
trường nào trong `config.Config`. Đặt tên cho nó là **STOP CONDITION của luật 11 (câu 1)**:
tên phải nói **VAI TRÒ** (`KAFKA_EVENT_ADDRESS`), không nói cụm (`KAFKA_02_ADDRESS`), và **ai
cấp là quyết định của chủ cụm**. `hooks/env_contract_guard.py` chặn lần ghi `os.Getenv("KAFKA_…")`.

Mã chỉ biết vai trò; manifest mới chọn cụm vật lý:

```yaml
env:
  - name: KAFKA_EVENT_ADDRESS        # VAI TRÒ — tên duy nhất mã biết
    valueFrom:
      configMapKeyRef:
        name: vigov-ha-tang
        key: KAFKA-02-ADDRESS        # CỤM — chọn ở đây, đổi ở đây
```

### Ba điều cấm

| Cấm | Vì sao |
|---|---|
| Số thứ tự cụm trong tên mã đọc (`KAFKA_02_ADDRESS`) | Hàn một tải công việc vào một cụm. Chuyển đi thành một lần sửa mã + phát hành mọi dịch vụ đọc nó |
| **Cắt danh sách địa chỉ hoặc DSN để giữ một host** | Đúng-trông-như-đúng suốt thời gian còn một node, sai im lặng vào đúng ngày lên HA. Đưa nguyên giá trị cho driver hiểu nhiều host (`pgx` hiểu) |
| Giá trị **riêng của một xã** trong biến môi trường | Luật 1, bất biến 10: môi trường chỉ mang hằng số **toàn nền tảng**. Giá trị theo xã đọc tại runtime từ sổ đăng ký của `platform` |

Bảng 16 biến ở trên được `tools/check_env_map.py` đối chiếu với `.env.example` và
`core/config/config.go` trong `make check`: thêm một biến mà quên cập nhật bảng là **đỏ**.

## 4. Cài lần đầu lên cụm

Chạy cho **`vigov-staging` trước**, rồi lặp lại y hệt cho `vigov-prod`.

```sh
kubectl apply -f deploy/cluster/namespace.yaml
kubectl apply -f deploy/cluster/rbac-jenkins.yaml

# Bí mật — TẠO NGOÀI KHO NÀY, không bao giờ commit (luật 8, bất biến 1)
kubectl -n vigov-prod create secret docker-registry harbor-vigov \
  --docker-server=harbor.omicrm.services --docker-username=... --docker-password=...

# platform: BA khoá. Nó KHÔNG đọc REDIS_DSN — cảnh báo thứ nhất ở mục 3.
kubectl -n vigov-prod create secret generic bi-mat-platform \
  --from-literal=DATABASE_DSN='postgres://...' \
  --from-literal=GRPC_CALLER_KEY='...' \
  --from-literal=SESSION_SIGNING_KEYS='<khoa-moi>,<khoa-cu>'

# Năm dịch vụ kia: BỐN khoá. REDIS_DSN không bắt buộc ở config.Load nhưng thiếu nó thì sáu
# đường dẫn POST trả 503 với pod xanh — cảnh báo thứ nhất ở mục 3.
for s in identity documents finance petitions comms; do
  kubectl -n vigov-prod create secret generic bi-mat-$s \
    --from-literal=DATABASE_DSN='postgres://...' \
    --from-literal=GRPC_CALLER_KEY='...' \
    --from-literal=SESSION_SIGNING_KEYS='<khoa-moi>,<khoa-cu>' \
    --from-literal=REDIS_DSN='redis://...'
done

# TÊN SECRET TLS KHÁC NHAU GIỮA HAI MÔI TRƯỜNG — chép nhầm thì Ingress lên bình thường và
# CHỈ HTTPS ĐỨT. Tên do overlay quy định, không đổi được ở đây:
#   staging -> vigov-staging-tls   (*.staging.vigov.vn)
#   prod    -> vigov-wildcard-tls  (*.vigov.vn)
kubectl -n vigov-prod create secret tls vigov-wildcard-tls --cert=... --key=...

kubectl apply -k deploy/overlays/prod
```

**ConfigMap `cau-hinh-chung` KHÔNG có trong khối lệnh trên, có chủ ý:** kustomize sinh nó từ
`overlays/<mt>/kustomization.yaml` với một hậu tố băm. Tạo tay một ConfigMap trùng tên là tạo
ra một đối tượng không Deployment nào trỏ tới.

**`GRPC_CALLER_KEY` bắt buộc ở MỌI dịch vụ** — `config.Load` không khởi động khi nó rỗng: một
cổng gRPC không có khoá gọi là một cổng trả lời **bất kỳ ai** chạm tới nó (ADR 0025).
**`SESSION_SIGNING_KEYS` cần ít nhất hai khoá ở prod** — một khoá thì không xoay được mà không
đăng xuất toàn bộ cán bộ của mọi xã cùng lúc (`core/config.CanhBao`).

`apply -k` dựng Deployment với thẻ `CHUA-TRIEN-KHAI-LAN-NAO`, tức **bảy pod ngồi
`ImagePullBackOff`** — đúng như thiết kế, không phải lỗi. Mục 5 đặt thẻ thật.

**Xanh khi:** `kubectl -n vigov-prod get secret` liệt kê đủ `harbor-vigov`, `vigov-wildcard-tls`
(staging: `vigov-staging-tls`), và `bi-mat-{platform,identity,comms,documents,finance,petitions}` — **sáu**, không phải bảy: `web-admin`
không đọc biến nào nên không có Secret của riêng nó.

Ba thứ dễ sót, cả ba đều hỏng ở chỗ cách xa nguyên nhân:

| Sót | Triệu chứng |
|---|---|
| Secret `harbor-vigov` | `ImagePullBackOff`, không nói gì về quyền |
| `GRPC_CALLER_KEY` / `SESSION_SIGNING_KEYS` | `CrashLoopBackOff` — `core/config.Load` gom đủ tên còn thiếu rồi thoát |
| Secret TLS, **hoặc đặt đúng tên nhưng sai môi trường** | Ingress lên bình thường, chỉ HTTPS đứt |

Bí mật vào cụm hôm nay bằng `kubectl create secret` tay. Bước kế tiếp khi thấy phiền:
**External Secrets Operator**. Manifest trong kho này khi ấy vẫn chỉ chứa **tên** khoá.

## 5. Bảy lượt deploy, theo đúng thứ tự này

Mỗi lượt là một lần bấm `vigov-deploy` với `DICH_VU` + `THE` + `MT`. Job kiểm ảnh có thật
trong Harbor, `kubectl set image`, đợi `rollout status`, `rollout undo` nếu đỏ.

Thứ tự không phải thói quen — nó là thứ tự phụ thuộc lúc chạy:

| # | Đơn vị | Tuyến REST | Đi trước vì |
|---|---|---|---|
| 1 | `platform` | 0 (nhưng **là** gRPC phân giải xã) | Sáu đơn vị kia quay số cổng 9090 của nó. Chưa có nó thì **mọi xã trả 404** |
| 2 | `identity` | 37 | Bốn dịch vụ dưới đổi cookie lấy principal qua gRPC 9090 của nó. Chưa có nó thì **mọi tuyến có kiểm quyền trả 401 cho một phiên hợp lệ** — pod xanh, probe xanh, không gì báo |
| 3 | `petitions` | 16 | độc lập với ba cái dưới, thứ tự tuỳ |
| 4 | `documents` | 13 | ″ |
| 5 | `finance` | 12 | ″ |
| 6 | `comms` | 4 | ″ |
| 7 | `web-admin` | — | **Sau cùng, có chủ ý.** Đưa bề mặt cán bộ lên trước khi API trả lời được nghĩa là một màn hình lỗi mang tên một cơ quan nhà nước |

Tổng **82 tuyến REST**, đếm từ `kb/20-contracts/openapi.json` ngày 23/09/2026 — không từ cảm giác.

Không có manifest, **có cân nhắc**: `reporting` (0 tuyến — `internal/http/routes.go` chưa mount
cái nào) · `platform-admin` (là app Next.js nhưng **chưa có `Dockerfile`**, nên chưa có ảnh) ·
`citizen-app` (chạy trong Zalo Mini App, không thành pod).

## 6. Kiểm sau khi lên

| Phép kiểm | Xanh nghĩa là |
|---|---|
| `kubectl -n vigov-<mt> get deploy` | 7 Deployment `READY` |
| Gọi một tuyến qua Ingress bằng `Host` của một xã thật | chuỗi Ingress → pod → phân giải xã chạy hết. Xã ấy phải có bản ghi DNS **và** một hàng trong sổ đăng ký của `platform` |
| **Hai `Host` khác nhau, cùng một tuyến** | dữ liệu trả về **không giao nhau** — phép kiểm cách ly hai xã, luật 1. **Chạy ở staging, trước khi đụng prod** |

`/healthz` xanh **không** phải điều kiện xanh: nó nằm ngoài chuỗi phân giải xã có chủ ý, nên
vẫn xanh khi CSDL hoặc `platform` hỏng — mục 8.

**Rút lui:** job tự `rollout undo` khi `rollout status` đỏ. Lịch sử build ghi cả lần thất bại;
đưa cụm về đúng bản mong muốn là **chạy lại job với thẻ cũ**, không sửa tay.

## 7. Sửa manifest sau khi đã chạy — **cái bẫy của mô hình này**

`overlays/<mt>/kustomization.yaml` **không** ghi thẻ đang chạy. Chạy `apply -k` lên một
namespace đã có dịch vụ sẽ **đẩy cả bảy về `CHUA-TRIEN-KHAI-LAN-NAO`**, tức
`ImagePullBackOff` đồng loạt trên prod. Đó là cái giá của việc đặt ảnh bằng `kubectl set image`.

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

## 8. Bốn điều dễ hiểu sai

| Điều | Sự thật |
|---|---|
| "Mỗi xã một namespace" | **Không.** Cùng một pod phục vụ mọi xã; xã phân giải từ `Host` trong từng request (luật 1, bất biến 3). Thêm xã = thêm bản ghi DNS + một hàng trong sổ đăng ký của `platform`, không sửa manifest |
| "Cần Job di trú CSDL" | **Không.** Di trú chạy trong tiến trình lúc khởi động, có advisory lock (`core/migrate.Chay`). Vì thế `startupProbe` rộng tay — replica thứ hai **đợi** replica thứ nhất |
| "`/healthz` xanh nghĩa là hệ thống ổn" | **Không.** Nó nằm ngoài chuỗi phân giải xã có chủ ý, nên vẫn xanh khi CSDL hoặc `platform` hỏng. Bắt sự cố đó bằng **giám sát tỉ lệ 404** từ `TenantMiddleware`, không bằng probe |
| "NetworkPolicy là tuỳ chọn" | **Không.** gRPC 9090 dùng `insecure.NewCredentials()` — không TLS, không xác thực. NetworkPolicy là biên duy nhất giữ danh bạ xã của toàn hệ thống |

## 9. Ingress sinh từ hợp đồng REST

Bảng định tuyến **được sinh ra** từ `kb/20-contracts/openapi.json`, nên "thêm tuyến trong Go"
và "tuyến ấy đi tới đúng dịch vụ" không còn là hai việc rời nhau. 82 tuyến → **26 luật**.

| Tệp | Vai trò |
|---|---|
| `tools/ingress/` | Bộ sinh. Chạy trong mục `kb` của `Makefile`, **sau** `apidoc` — trước thì nó sinh từ hợp đồng cũ |
| `base/mang/ingress.yaml` | **SINH RA.** Giống nhau ở mọi môi trường nên nó thuộc `base/` |
| `overlays/<mt>/ingress-moi-truong.yaml` | Bản vá **JSON6902** cho host + TLS |

**Vì sao là JSON6902 chứ không phải một `Ingress` đầy đủ trong overlay:** `spec.rules` là danh
sách không có khoá trộn, nên một strategic-merge patch chỉ cần *nhắc tới* `rules` là **thay cả
danh sách** — xoá sạch bảng sinh ra, và `kustomize` không kêu một tiếng.

**Gom tới mức TÀI NGUYÊN, không hơn.** Không gom theo tiền tố có gạch nối: `task-blocs` là
`identity` trong khi `task-priorities` và `task-types` là `petitions`.

**Một tuyến không xác định được dịch vụ chủ thì bộ sinh DỪNG**, không mặc định về `identity`
(luật 1: không có mặc định trên đường cách ly). `go test ./tools/ingress` đối chiếu tệp trên
đĩa với hợp đồng: xoá một luật hay đổi một backend là **đỏ**.

## 10. Chưa được chứng minh — đọc trước khi bấm

| Việc | Trạng thái |
|---|---|
| 10 job Jenkins | **`vigov-gate` đã chạy lần đầu 24/09/2026 và đổ ở `make check`**, vì `python` trên máy chủ là 2.7 (đã sửa: `PYTHON=python3`). Chín job còn lại chưa chạy lần nào. Máy chủ đã chứng minh có `go`, `buf`, `node`, `npm`, `python3`, `make`, `gcc`; `docker` được chứng minh qua lượt chạy 21/09/2026 của kho `vihat-miniapp`. **Phiên bản `python3` chưa được đọc** |
| Cụm mà kubeconfig trỏ tới | **chưa ai chạy `kubectl` với nó.** Đường dẫn `/u01/rancher/rancher-vigov.yaml` đã được chủ dự án xác nhận, nhưng lượt `vigov-deploy` đầu tiên vẫn là lần đầu biết nó mở được cụm nào — đọc dòng `current-context` ở stage đầu |
| `deploy/Jenkinsfile` | **chưa máy nào phân tích cú pháp.** Không có Jenkins ở máy trạm, và `tools/check_build.py` chỉ soi 8 Jenkinsfile của dịch vụ |
| Manifest qua API server thật | **chưa.** `kubectl kustomize` chỉ chứng minh YAML dựng được, không chứng minh máy chủ chấp nhận. Mục 4 là lần đầu biết |
| Redis ở prod | **chưa có DSN thật.** Thiếu nó thì sáu đường dẫn `POST` ở mục 3 trả 503 trong khi pod xanh |
| Đóng êm khi `SIGTERM` | **xong 22/09/2026** — cả sáu dịch vụ Go `signal.Notify` + `srv.Shutdown`, `terminationGracePeriodSeconds: 45` > ngữ cảnh 20 giây |
