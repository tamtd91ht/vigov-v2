# Job vận hành `vigov-van-hanh` — sổ tay người vận hành

Một việc duy nhất: **đổi hạ tầng** mà các dịch vụ trỏ tới — PostgreSQL, MinIO, Redis, ClamAV. Dựng lại
từ đầu ngày 11/10/2026 (chủ dự án: "đập đi xây lại"); 14 việc cũ đã bỏ. Vì sao có job và các ràng buộc nó
giữ: **ADR 0089** và §Sửa đổi 11/10/2026 (`kb/10-decisions/0089-jenkins-operations-job.md`). Tệp này chỉ
nói **làm thế nào**.

| Tệp | Là gì |
|---|---|
| `Jenkinsfile` | Form + một khối lệnh. Cuối tệp có vùng **SINH RA** (`opsCatalog()`, `opsInfra()`) — đừng sửa tay |
| `ops.py` | Mọi quyết định: hạ tầng nào gồm khoá nào, ghi vào Secret/ConfigMap nào, dịch vụ nào khởi động lại, kiểm gì trước khi ghi. Cũng giữ phần nền bản đồ mà `vigov-deploy` gọi |
| `test_ops.py` | Unit test của `ops.py` — chạy trong `make buildfiles` (tức `make check`) |
| `basemap-task.sh` | Việc `nen-ban-do` của job `vigov-deploy` — mục "Nền bản đồ" cuối tệp |

## 1. Dựng job — một lần

### 1.1 Plugin **Active Choices**

Manage Jenkins → Plugins → Available → **Active Choices** (id `uno-choice`) → Install. Thiếu plugin thì
job **dừng ngay** với câu nêu tên plugin. Chủ dự án chọn cài plugin (11/10/2026) thay vì form tĩnh.

⚠ Controller **dùng chung với dự án khác** (ADR 0089 §Hệ quả #5): cài plugin và duyệt script ảnh hưởng
cả dự án kia.

### 1.2 Tạo job

`New Item` → `vigov-van-hanh` → **Pipeline** (job đã có thì giữ, chỉ cần cấu hình như dưới):

| Mục | Điền |
|---|---|
| General / Build Triggers | **bỏ trống** — job tự khai `disableConcurrentBuilds`, giữ 200 lượt |
| Definition | `Pipeline script from SCM` · Git · cùng URL + credential · Branch `*/main` |
| Script Path | `deploy/van-hanh/Jenkinsfile` |
| Lightweight checkout | **BỎ TICK** |

### 1.3 Lượt đầu — và mỗi lần tệp đổi

1. Bấm **Build Now** một lần. Lượt này **dừng** với câu "Jenkins vừa học tham số mới" — đúng thiết kế,
   cụm không bị đụng.
2. **Manage Jenkins → In-process Script Approval**: duyệt **mọi** script của job (3 script form + 2 script
   dự phòng). Script **không sandbox**: chúng gọi `kubectl` của controller.
3. Mở **Build with Parameters**.

Mỗi lần `Jenkinsfile` đổi phần form (kể cả vùng SINH RA) là **duyệt lại**. Chưa duyệt thì ô ấy hiện câu
"Form lỗi — duyệt script…".

### 1.4 Máy nào cần gì

| Máy | Cần | Vì sao |
|---|---|---|
| **Controller** | `kubectl` trong PATH, đọc được `/u01/rancher/rancher-vigov.yaml` | Script form chạy trên controller, CHỈ ĐỌC, hạn 6 giây. Không đọc được thì form hiện một câu — job vẫn tự đọc lại cụm khi chạy |
| **Máy build** | `kubectl` (có `patch --patch-file`), `git`, Python **3.8+** | Stage `Chuẩn bị` tự kiểm và dừng nếu thiếu |
| **Kubeconfig** | Đọc/ghi Secret, ConfigMap; restart Deployment; tạo/xoá Pod | ⚠ `deploy/cluster/rbac-jenkins.yaml` ghi "không quyền trên secrets" — kubeconfig thật **rộng hơn** tệp ấy. Lệch có từ trước, chủ dự án quyết |

## 2. Form

| # | Ô | Làm gì |
|---|---|---|
| 1 | `MT` | `staging` (mặc định) hoặc `prod`. Bảng bên dưới: Deployment, sẵn sàng/tổng, thẻ ảnh |
| 2 | `HA_TANG` | Chọn **một** hạ tầng |
| 3 | `CAU_HINH` | Chỉ các ô của hạ tầng ấy, cạnh giá trị **đang chạy**. **Ô trống = giữ nguyên** — chỉ gõ thứ cần đổi |
| | `BI_MAT_1..7` | Ô mật khẩu. Gõ mật khẩu / khoá vào một ô, rồi ở bảng `CAU_HINH` chọn "ô BI_MAT_n" cho chỗ cần nó. Một ô dùng được cho nhiều chỗ cùng giá trị |
| 4 | `CHAY_THU` · `KHOI_DONG_LAI` · `XAC_NHAN` · `TICKET` | Chạy thử **bật** mặc định. Làm thật: bỏ tick, `XAC_NHAN` = namespace, `TICKET` bắt buộc ở prod |

Vì sao mật khẩu không nằm trong bảng `CAU_HINH`: giá trị của ô động (Active Choices) được Jenkins lưu
**thành chữ thường** trong lượt build. Chỉ tham số password được mã hoá.

## 3. Từng hạ tầng

| Hạ tầng | Ô | Ghi vào | Khởi động lại |
|---|---|---|---|
| **PostgreSQL** | Chung: máy chủ (`host:port`, nhiều máy cách dấu phẩy), `sslmode`. Từng dịch vụ: CSDL, tài khoản, mật khẩu | `DATABASE_DSN` của `<dịch vụ>-secrets` — giữ phần không gõ (tài khoản, mật khẩu, CSDL, tham số khác) | Dịch vụ có DSN đổi |
| **MinIO** | `OBJECT_STORAGE_ENDPOINT` · `_PUBLIC_ENDPOINT` · `_BUCKET_PREFIX` · `_REGION` · `_PUBLIC_MEDIA_BASE_URL`; cặp khoá riêng của từng dịch vụ | 5 khoá `common-config`; `OBJECT_STORAGE_ACCESS_KEY` / `_SECRET_KEY` của `<dịch vụ>-secrets` | Dịch vụ đọc khoá đổi |
| **Redis** | Máy chủ (**một** `host:port`), số db, giao thức `redis`/`rediss`, mật khẩu | `REDIS_DSN` của mọi dịch vụ dùng Redis — giữ số db riêng của từng dịch vụ nếu không gõ | Dịch vụ dùng Redis |
| **ClamAV** | `MALWARE_SCANNER_ADDRESS` (`host:3310`, danh sách được) | `common-config` | Dịch vụ quét tệp |

Danh sách "dịch vụ đọc khoá" **suy ra** từ `core/config` (`r.read` × `config.Uses`), không chép tay.
Hạ tầng gồm khoá nào: `INFRA` trong `ops.py` — chỗ duy nhất viết tay.

### Quy tắc job tự giữ

| Quy tắc | Vì sao |
|---|---|
| `sslmode` chỉ `require` / `verify-ca` / `verify-full`; DSN sau khi đổi không có `sslmode` → từ chối | Luật 13: đường tới CSDL phải mã hoá |
| `*_ENDPOINT`, `*_URL` phải `https://`; endpoint MinIO không đường dẫn, không thông tin đăng nhập | Luật 13, luật 8 |
| Redis đúng **một** máy chủ | `core/idem`, `core/ratelimit` đọc bằng go-redis `ParseURL` — danh sách làm dịch vụ không khởi động |
| Đổi tài khoản PostgreSQL phải kèm mật khẩu; đổi khoá MinIO phải đổi **cả cặp** | Nửa cũ + nửa mới không đăng nhập được |
| `OBJECT_STORAGE_PUBLIC_MEDIA_BASE_URL` để trống: tự dựng `<public endpoint>/<tiền tố>-public` **chỉ khi** một trong hai đổi | `deploy/cau-hinh/README.md` mục 6 |
| `BASEMAP_URL` **không** đổi; đổi endpoint MinIO thì in cảnh báo | Tệp nền phải có trên MinIO mới trước — `vigov-deploy`, việc `nen-ban-do` |
| Không chuyển dữ liệu | Hạ tầng mới phải có sẵn CSDL/bucket/user và dữ liệu — người quản trị làm ngoài job (luật 7) |
| Mật khẩu và DSN không bao giờ in, không lên dòng lệnh | Luật 8. Bảng kế hoạch chỉ in tài khoản, máy chủ, CSDL, sslmode |

## 4. Một lượt chạy — và quay lui

1. Đọc `common-config` và `<dịch vụ>-secrets` liên quan.
2. In **kế hoạch**: khoá cũ → mới (bí mật chỉ in "có / giá trị mới"), dịch vụ sẽ khởi động lại.
3. **Kiểm từ trong cụm**, trước khi ghi, bằng một pod `postgres:16-alpine` một lượt: mở được cổng của
   máy chủ mới (Redis, MinIO, ClamAV); **đăng nhập được** bằng từng DSN PostgreSQL mới (qua Secret tạm,
   xoá ngay). Không đạt → dừng, không ghi gì. Chạy thử cũng làm bước này.
4. Chạy thử dừng ở đây.
5. Làm thật: sao lưu giá trị cũ → Secret **riêng** `<dịch vụ>-secrets-backup-<giờ UTC>` (không gắn
   Deployment nào) → ghi Secret / ConfigMap → **đọc lại so khớp** → khởi động lại theo thứ tự platform →
   identity → còn lại, mỗi dịch vụ đợi tối đa 6 phút; hỏng thì in log và **dừng**.

Mỗi lần ghi in một dòng `GHI · <người bấm> · <giờ UTC> · <namespace> · ticket <số> · <ghi gì>` và bảng
**TÓM TẮT** cuối lượt. Log **không bền** — cắt sau 200 lượt (ADR 0089).

**Quay lui:**

| Đã đổi | Cách |
|---|---|
| Khoá `common-config` | Chạy lại job, gõ giá trị cũ — cột "CŨ" của kế hoạch in sẵn |
| Khoá trong Secret | Chép lại từ bản sao lưu (bằng tay — job không còn việc khôi phục): `kubectl -n <ns> get secret <dv>-secrets-backup-<giờ> -o json` → lấy `.data` thành tệp `{"data": {...}}` → `kubectl -n <ns> patch secret <dv>-secrets --type=merge --patch-file <tệp>` → `kubectl -n <ns> rollout restart deploy/vigov-service-<dv>`. Xoá tệp tạm sau đó |

Secret `*-secrets-backup-*` **không tự xoá**: chúng là đường quay lui. Dọn là quyết định của người vận
hành, làm tay (luật 7).

## 5. Không làm ở job này

| Việc | Ở đâu |
|---|---|
| Đặt ảnh mới, lùi bản | Job của từng dịch vụ; `kubectl rollout undo` bằng tay |
| Khoá ký / mã hoá (`GRPC_CALLER_KEY`, `SESSION_SIGNING_KEYS`, `SECRET_ENCRYPTION_KEYS`…) | Job `vigov-deploy` (việc tạo khoá) hoặc Rancher — không phải hạ tầng |
| CSDL mới rỗng (tạo DB + user + migration) | Đã bỏ cùng các việc cũ (11/10/2026). Hạ tầng mới phải được người quản trị dựng sẵn |
| Tạo tài khoản vận hành, nền bản đồ | Job `vigov-deploy` (`tao-tai-khoan-van-hanh`, `nen-ban-do`) |
| Việc của xã | platform-admin (ADR 0048, 0070) |

## Nền bản đồ — việc `nen-ban-do` của `vigov-deploy`

Quy trình đầy đủ, kiểm chủ quyền có ký, đưa vào dùng: `kb/40-runbooks/basemap-pmtiles.md`. Việc chạy
trên **máy build** (CentOS 7, có Internet), không trong pod, từ job `vigov-deploy` (`HANH_DONG=nen-ban-do`,
tham số `MT` · `NGAY_NEN` · `CHAY_THU` · `XAC_NHAN` · `TICKET`). Thân việc ở `basemap-task.sh` + `ops.py`
cạnh tệp này vì quyết định của nó có test ở đây.

Máy build cần thêm `bash`, `curl`, `sha256sum`, `tar`; ≥ 8 GB trống ở workspace; ra Internet tới
`github.com`, `raw.githubusercontent.com`, `build.protomaps.com`, `build-metadata.protomaps.dev`; tới
`OBJECT_STORAGE_PUBLIC_ENDPOINT`.

Không user MinIO mới, không credential Jenkins (chủ dự án 11/10/2026). Khoá MinIO là cặp
`OBJECT_STORAGE_ACCESS_KEY` / `OBJECT_STORAGE_SECRET_KEY` **có sẵn** trong `comms-secrets` (thiếu thì
`platform-secrets`; không có cả hai thì việc dừng). Job đọc bằng kubeconfig của chính nó → tệp 0600 trong
`$WORKSPACE/.mc` → biến `MC_HOST_<alias>` của `mc`: không lên dòng lệnh, không in, không tệp cấu hình.

Policy MinIO của user ấy phải phủ `<tiền tố>-public/config/map/*` (`s3:ListBucket` với prefix `config/map/*`,
`s3:GetObject`, `s3:PutObject`). Không phủ thì bước kiểm `mc ls` (trước khi dựng) dừng và in đúng câu cần
thêm vào policy.

### Hằng đã ghim — `deploy/van-hanh/ops.py`

| Hằng | Lấy ở đâu (trang phát hành **chính thức** trên GitHub, không chép từ nơi khác) |
|---|---|
| `PMTILES_SHA256` | `https://github.com/protomaps/go-pmtiles/releases/tag/v1.31.2` → mục **Assets**, digest `sha256:` cạnh `go-pmtiles_1.31.2_Linux_x86_64.tar.gz` (bản phát hành này **không có** `checksums.txt`) |
| `MC_RELEASE` + `MC_SHA256` | `https://github.com/minio/mc/releases/tag/<MC_RELEASE>` → cột đầu của tệp `mc.linux-amd64.<MC_RELEASE>.sha256sum`. **Không** dùng `dl.min.io` — trả 410 Gone (11/10/2026) |

Đổi phiên bản = đổi mã băm **trong cùng lần sửa**.

### Mỗi lần làm mới

| # | Lượt bấm (`vigov-deploy`) | Kết quả |
|---|---|---|
| 1 | `nen-ban-do`, `MT=staging`, `NGAY_NEN`, chạy thử | Bảng: bucket · địa chỉ tải lên · đích · giá trị `BASEMAP_URL` sẽ là; kiểm công cụ, chỗ trống |
| 2 | Như #1, bỏ `CHAY_THU`, `XAC_NHAN=vigov-staging` | Tải công cụ (kiểm SHA-256) → kiểm `mc ls` → dựng → tải lên `<tiền tố>-public/config/map/<ngày>/` → so `SHA256SUMS`. In SHA-256, `BUILD-INFO.txt`, giá trị `BASEMAP_URL` |
| 3 | Đặt `BASEMAP_URL` theo runbook mục "Đưa vào dùng" (`kubectl edit` + `rollout restart`) | Khởi động lại `vigov-web-admin`, `vigov-service-comms` |
| 4 | Kiểm chủ quyền có ký trên staging — runbook | Không đạt: dừng, giữ giá trị cũ |
| 5 | Như #3 với `vigov-prod` | Chỉ sau khi đã ký |

`$WORKSPACE/basemap`, `.tools`, `.mc` luôn bị xoá cuối lượt (vài GB). Hạn của việc là 180 phút.
