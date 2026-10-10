# Job vận hành `vigov-van-hanh` — sổ tay người vận hành

Job Jenkins mới, chạy **song song** job cũ `vigov-deploy` (`deploy/Jenkinsfile` — không đổi). Vì sao có
nó và những ràng buộc nó giữ: **ADR 0089** (`kb/10-decisions/0089-jenkins-operations-job.md`). Tệp này
chỉ nói **làm thế nào**.

Một lượt bấm = **một việc**. Việc tay chỉ còn là **gõ giá trị**: form đọc cụm và điền sẵn phần còn lại.

| Tệp | Là gì |
|---|---|
| `Jenkinsfile` | Form + các bước. Cuối tệp có vùng **SINH RA** (`opsCatalog()` …) — đừng sửa tay |
| `ops.py` | Mọi quyết định: khoá nào được ghi, dịch vụ nào khởi động lại, kiểm tài nguyên, dựng DSN, pod một lượt |
| `test_ops.py` | Unit test của `ops.py` — chạy trong `make buildfiles` (tức `make check`) |

## 1. Dựng job — một lần

### 1.1 Plugin **Active Choices**

Manage Jenkins → Plugins → Available → **Active Choices** (id `uno-choice`) → Install. Thiếu plugin thì
job **dừng ngay** với câu nêu tên plugin — không lùi về form tĩnh.

⚠ Controller **dùng chung với dự án khác** (ADR 0089 §Hệ quả #5): cài plugin và duyệt script ảnh hưởng
cả dự án kia. Ai được duyệt là câu hỏi mở #2 của ADR 0089.

### 1.2 Tạo job

`New Item` → `vigov-van-hanh` → **Pipeline**, giống các job khác (`deploy/README.md` mục 2):

| Mục | Điền |
|---|---|
| General / Build Triggers | **bỏ trống** — job tự khai `disableConcurrentBuilds`, giữ 200 lượt; không trigger nào |
| Definition | `Pipeline script from SCM` · Git · cùng URL + credential · Branch `*/main` |
| Script Path | `deploy/van-hanh/Jenkinsfile` |
| Lightweight checkout | **BỎ TICK** |
| Shallow clone | **KHÔNG BẬT** — đối chiếu migration đọc `git` theo commit của ảnh đang chạy |

### 1.3 Lượt đầu — và mỗi lần tệp đổi

1. Bấm **Build Now** một lần. Lượt này **dừng** với câu "Jenkins vừa học tham số mới" — đúng thiết kế,
   cụm không bị đụng.
2. **Manage Jenkins → In-process Script Approval**: duyệt **mọi** script của job (6 script form + 2
   script dự phòng). Script **không sandbox**: chúng gọi `kubectl` của controller — sandbox không cho
   chạy tiến trình.
3. Mở **Build with Parameters**: form hiện bảng đọc từ cụm.

Mỗi lần `Jenkinsfile` đổi phần form (kể cả vùng SINH RA, ví dụ khi thêm một biến môi trường mới) là
**duyệt lại**. Chưa duyệt thì ô ấy hiện câu "Form lỗi — duyệt script…".

### 1.4 Máy nào cần gì

| Máy | Cần | Vì sao |
|---|---|---|
| **Controller** | `kubectl` trong PATH, đọc được `/u01/rancher/rancher-vigov.yaml` | Script form chạy trên controller, CHỈ ĐỌC, hạn 6 giây. Không đọc được thì form hiện một câu và vẫn dùng được — job tự đọc lại cụm khi chạy |
| **Máy build** | `kubectl`, `git`, Python **3.8+** (`python3.X`) | Như job cũ — stage `Chuẩn bị` tự kiểm và dừng nếu thiếu |
| **Kubeconfig** | Quyền đọc/ghi Secret, ConfigMap, Deployment; tạo/xoá Pod | ⚠ `deploy/cluster/rbac-jenkins.yaml` ghi "không quyền trên secrets" — job cũ và job này đều ghi Secret, tức kubeconfig thật **rộng hơn** tệp ấy. Lệch này có từ trước, cần chủ dự án quyết |

## 2. Form — bốn vùng

| Vùng | Ô | Ghi chú |
|---|---|---|
| 1. Ngữ cảnh | `MT` · `NGU_CANH` | Bảng: Deployment, sẵn sàng/tổng, thẻ ảnh; khoá `common-config` **thiếu** so với sổ (đỏ đậm = bắt buộc) và khoá trên cụm **ngoài sổ** |
| 2. Việc | `NHOM` → `VIEC` | Mục mờ = chưa dựng (lý do hiện ở Vùng 3) |
| 3. Tham số | `DICH_VU` · `DICH_VU_CHON` · `THONG_SO` · `BUOC_CSDL` · `CSDL_*` · `BI_MAT_1..3` · `SO_DONG` · `EMAIL/TEN_VAN_HANH` | `THONG_SO` đổi theo việc, điền sẵn từ cụm |
| 4. An toàn | `CHAY_THU` (**bật** mặc định) · `KHOI_DONG_LAI` · `XAC_NHAN` · `XAC_NHAN_CSDL_MOI` · `TICKET` | Ghi thật: bỏ tick `CHAY_THU`, `XAC_NHAN` = namespace, `TICKET` bắt buộc ở prod |

**Chạy thử** in kế hoạch cũ → mới và dịch vụ sẽ khởi động lại, **không ghi gì**. Luôn chạy thử trước.

Mỗi lần ghi in một dòng `GHI · <người bấm> · <giờ UTC> · <namespace> · ticket <số> · <ghi gì>` và một bảng
**TÓM TẮT** cuối lượt. Không bao giờ in giá trị bí mật. Log **không bền** — cắt sau 200 lượt (ADR 0089).

## 3. Từng việc

| Nhóm | Việc | Gõ gì | Ghi gì |
|---|---|---|---|
| Kiểm tra | `tong-quan` | — | không — chỉ đọc |
| | `xem-log` | `DICH_VU` (Deployment), `SO_DONG` | không |
| | `trang-thai-migration` | — | không. Pod `postgres:16-alpine` một lượt đọc `schema_migration` của từng CSDL; so với tệp migration ở **commit của ảnh đang chạy** |
| | `kiem-ket-noi` | — | không. Từ trong cụm: CSDL từng dịch vụ (`pg_isready`), Redis, MinIO, ClamAV (`nc -z`). RabbitMQ/Kafka/Elasticsearch: chưa dịch vụ nào dùng — không kiểm |
| Cấu hình | `sua-configmap` | Ô "giá trị mới" từng dòng (hoặc gõ thẳng khung: `KHOÁ=giá trị`) | `common-config`, đọc lại kiểm, rồi khởi động lại **chỉ** dịch vụ đọc khoá ấy |
| Bí mật | `sua-secret` | `DICH_VU`; mỗi khoá chọn `giu` · `o1/o2/o3` (giá trị ở ô password `BI_MAT_1/2/3`) · `sinh` · `xoay` | Giá trị cũ → Secret **riêng** `<dịch vụ>-secrets-backup-<giờ>`; rồi `<dịch vụ>-secrets`; khởi động lại dịch vụ ấy |
| | `khoi-phuc-secret` | `DICH_VU`; `THONG_SO` = tên bản sao lưu (điền sẵn bản mới nhất) | Chép khoá của bản sao lưu về; giá trị hiện tại cũng được sao lưu trước |
| CSDL | `csdl-moi` | mục 4 | mục 4 |
| | `chay-migration` | `DICH_VU_CHON` | Khởi động lại — migration chạy **trong tiến trình lúc khởi động** (`core/migrate`); ảnh quyết định áp tới đâu |
| Tài nguyên | `sua-tai-nguyen` | Sửa số trong khung (điền sẵn từ cụm) hoặc bấm "Áp mẫu" | `replicas`, cpu/memory request/limit; đợi rollout |
| Triển khai | `khoi-dong-lai` | `DICH_VU_CHON` | `rollout restart` + đợi |
| | `lui-ban-truoc` | `DICH_VU_CHON` | `rollout undo` về revision liền trước |
| Bảo trì | `tao-tai-khoan-van-hanh` | `EMAIL_VAN_HANH`, `TEN_VAN_HANH`, `TICKET` (mọi môi trường) | `operatorctl create` + 7 quyền `ops.*`. **Mật khẩu tạm in vào log** — đổi + đăng ký TOTP ngay |

### Quy tắc job tự giữ

| Quy tắc | Nguồn |
|---|---|
| Chỉ ghi khoá có trong `.env.example` + bảng `deploy/cau-hinh/README.md`; khoá Secret không ghi vào ConfigMap; khoá env-của-Deployment (mục 3) không ghi vào `common-config` | Luật 11 cấm #4 |
| "Dịch vụ đọc" của mỗi khoá **suy ra** từ `core/config` (`r.read` × `config.Uses`) — không danh sách tay | Luật 9, ADR 0057 |
| Địa chỉ `*_ADDRESS`/`*_ADDR` là danh sách `host:port`; `*_ENDPOINT`/`*_URL` phải `https://`; `TRUSTED_PROXY_CIDRS` không `0.0.0.0/0`; `ENV` phải khớp `MT` | Luật 11 bất biến 5, luật 13 |
| `sinh` chỉ khi khoá **chưa có**; khoá đang có thì `xoay` (thêm khoá mới lên đầu, giữ khoá cũ) — ghi đè khoá mã hoá là mất mọi dữ liệu đã niêm | ADR 0009 |
| Khoá phải **cùng giá trị** ở nhiều Secret (`GRPC_CALLER_KEY`, `OPERATOR_SESSION_SIGNING_KEYS`) không `sinh`/`xoay` được — nhập cùng giá trị qua `o1` cho từng dịch vụ | README mục 1 |
| Bản sao lưu là Secret **không gắn Deployment nào** — không bao giờ một khoá thêm trong Secret nạp bằng `envFrom` | ADR 0089 câu mở #4, luật 11 cấm #4 |
| Không đặt ảnh mới, không áp Ingress, không việc của xã | ADR 0089, ADR 0046 #4, ADR 0048/0070 |

**Cách viết khoá:** job ghi đúng như Deployment đang đọc — `envFrom` nên khoá **gạch dưới**
(`MALWARE_SCANNER_ADDRESS`); `BASEMAP-URL` giữ gạch ngang vì web-admin ánh xạ nó tường minh. Lệch với luật 11
bất biến 4 là câu mở #3 của ADR 0089 — job **không** quyết.

### Mẫu tài nguyên — ĐỀ XUẤT, chủ dự án chốt số

| Mẫu | replicas | cpu request/limit | memory request/limit |
|---|---|---|---|
| `nho` | 1 | 50m / 500m | 128Mi / 256Mi |
| `vua` | 2 | 100m / 1000m | 256Mi / 512Mi |
| `lon` | 3 | 250m / 2000m | 512Mi / 1Gi |

Nguồn số: `PRESETS` trong `ops.py` (form đọc bản sinh ra). Đổi số = sửa `PRESETS`, chạy
`python deploy/van-hanh/ops.py catalog --write`, duyệt lại script.

## 4. CSDL mới cho prod — làm theo thứ tự

### Hệ quả — đọc TRƯỚC (job in lại khối này ở mọi lượt chạy thử trên prod)

1. CSDL mới **rỗng**: không có xã nào → mọi tên miền xã trên prod trả **404** cho tới khi tạo lại xã.
2. Không có tài khoản vận hành → **platform-admin không đăng nhập được** cho tới khi tạo lại.
3. Dòng `mini_app` và App Secret của xã **ở lại CSDL cũ** — app riêng của xã ngừng đăng nhập công dân.
4. Mọi hồ sơ đã ghi trên prod ở lại CSDL cũ — **bản duy nhất** (luật 7) — và không hiện trên prod.
5. **Staging** đang dùng chung CSDL cũ và **ở lại đó**.
6. `vihat-miniapp` (CSDL trong `vihat-miniapp-bi-mat`) **không thuộc** việc này — giữ nguyên.

CSDL cũ không bị ghi, không bị xoá. Không chuyển dữ liệu ở đợt này (ADR 0089 — chuyển dữ liệu là quyết định riêng).

### Các bước — mỗi dòng là một lượt bấm

| # | Việc | Thiết lập | Kiểm |
|---|---|---|---|
| 1 | `tong-quan` | `MT=prod` | Mọi Deployment sẵn sàng; ghi lại thẻ ảnh |
| 2 | `trang-thai-migration` | | Mọi dịch vụ "khớp" trên CSDL **cũ** — lệch thì sửa trước |
| 3 | Sao lưu CSDL cũ | **ngoài job** — người quản trị PostgreSQL (`pg_dump` từng CSDL) | Có tệp sao lưu |
| 4 | `csdl-moi`, **chạy thử** | `CSDL_MAY_CHU`, `CSDL_CONG`, `CSDL_SSLMODE`; `THONG_SO` điền sẵn `<dịch vụ> vigov_<dịch vụ>_prod vigov_<dịch vụ>_prod` | Bảng: CSDL đang dùng · CSDL mới · tài khoản mới; khối HỆ QUẢ |
| 5 | `csdl-moi`, thật, chỉ tick `tao-csdl` + `chay-migration` + `kiem-luoc-do` | + `CSDL_QUAN_TRI`, `MAT_KHAU_QUAN_TRI_CSDL`, bỏ `CHAY_THU`, `XAC_NHAN=vigov-prod`, `XAC_NHAN_CSDL_MOI=TAO-CSDL-MOI-TRONG`, `TICKET` | Mỗi dịch vụ in `"migration xong"`; bảng lược đồ "khớp". Pod chưa đụng gì đang chạy |
| 6 | `csdl-moi`, chạy thử, tick `dat-dsn` + `khoi-dong-lai` | như #4 (cùng `THONG_SO`) | Kế hoạch đúng |
| 7 | `csdl-moi`, thật, tick `dat-dsn` + `khoi-dong-lai` | như #5 | Mỗi dịch vụ: bản sao lưu `<dv>-secrets-backup-<giờ>`, DSN đọc lại khớp, rollout sẵn sàng (platform → identity → còn lại) |
| 8 | `kiem-ket-noi`, `trang-thai-migration` | | Mọi CSDL nhận kết nối, mọi dịch vụ "khớp" |
| 9 | `tao-tai-khoan-van-hanh` | email, tên, ticket | Đăng nhập platform-admin |
| 10 | Tạo lại xã, tên miền, Mini App | **platform-admin** | Tên miền xã trả trang đăng nhập, không 404 |

Chi tiết bước:

| Bước `BUOC_CSDL` | Làm gì |
|---|---|
| `tao-csdl` | Sinh mật khẩu mỗi tài khoản (CSPRNG, 48 ký tự hex), ghi DSN mới vào Secret `<dv>-dsn-pending` (**không gắn Deployment nào**) **trước**, rồi một pod `postgres:16-alpine` chạy: dừng nếu CSDL/tài khoản đã có → `CREATE ROLE` → `CREATE DATABASE … OWNER` → chỉ tài khoản ấy được `CONNECT`. Mật khẩu quản trị và mật khẩu mới vào pod qua Secret **tạm** (xoá khi xong). Hỏng giữa chừng: `*-dsn-pending` giữ mật khẩu đã cấp, chạy lại `tao-csdl` sẽ dừng (cố ý — người quản trị xem tay) |
| `chay-migration` | Mỗi dịch vụ một pod bằng **chính ảnh đang chạy**, `DATABASE_DSN` lấy từ `<dv>-dsn-pending`, đợi dòng log `"migration xong"` (tối đa 6 phút) rồi xoá pod. Pod không mang nhãn Deployment nên Service không gửi yêu cầu vào. Ghi chú: tiến trình khởi động đầy đủ trong vài giây trước khi bị xoá — cùng tác dụng phụ như lần khởi động ở bước `khoi-dong-lai`, trên CSDL mới rỗng |
| `kiem-luoc-do` | `schema_migration` của CSDL mới phải có đủ mọi tệp của ảnh, đúng checksum. Thay cho `tools/schema-smoke` — công cụ ấy tạo schema tạm cho dev/CI, không dùng trên prod |
| `dat-dsn` | Tự chạy lại phép kiểm lược đồ; lệch thì **không** đặt. Sao lưu `DATABASE_DSN` cũ sang `<dv>-secrets-backup-<giờ>` rồi đặt DSN mới, đọc lại so khớp (không in) |
| `khoi-dong-lai` | `rollout restart` theo thứ tự platform → identity → còn lại; một dịch vụ không sẵn sàng sau 6 phút thì in log và **dừng** |

`CSDL_SSLMODE`: `require` mã hoá đường truyền. `verify-full`/`verify-ca` còn kiểm chứng thư máy chủ — cần
CA trong pod và trong ảnh dịch vụ, **chưa có** ⇒ chọn chúng thì kết nối hỏng (đóng, không lùi). Không bao
giờ tắt TLS (luật 13).

## 5. Quay lui

| Đã làm | Quay lui |
|---|---|
| `dat-dsn` (một hay mọi dịch vụ) | `khoi-phuc-secret`, `DICH_VU=<dv>`, `THONG_SO=<dv>-secrets-backup-<giờ>` → khởi động lại → dịch vụ về CSDL cũ (CSDL cũ không bị đụng) |
| `sua-secret` | `khoi-phuc-secret` với bản sao lưu lượt ấy tạo |
| `sua-configmap` | Chạy lại với giá trị cũ — cột "CŨ" của kế hoạch in sẵn |
| `sua-tai-nguyen` | `lui-ban-truoc` cho Deployment ấy, hoặc chạy lại với số cũ in trong kế hoạch |
| Ảnh mới hỏng | `lui-ban-truoc` (đặt ảnh mới vẫn là job của dịch vụ) |

Secret `*-dsn-pending` và `*-secrets-backup-*` **không tự xoá**: chúng là đường quay lui. Dọn là quyết định
của người vận hành sau khi đã chắc chắn — làm tay, không qua job (luật 7).

## 6. Chưa dựng

| Việc | Vì sao · làm ở đâu |
|---|---|
| `tao-xa` · `gan-ten-mien` · `gan-mini-app` · `dat-secret-mini-app` | Việc của xã thuộc **platform-admin** (ADR 0048, 0070) — job không dựng lại |
| `dung-clamav` · `mo-netpol-identity-zalo` | Job cũ `vigov-deploy` |
| `sua-env-deployment` | Env đặt thẳng trong Deployment (`deploy/cau-hinh/README.md` mục 3) — job cũ có `bo-sung-cau-hinh-*` |

## 7. Giới hạn đã biết

| Giới hạn | Hệ quả |
|---|---|
| Jenkins **lưu tham số** mỗi lượt; tham số password được mã hoá nhưng quản trị Jenkins đọc được | Rủi ro chấp nhận ở ADR 0089 |
| `tao-tai-khoan-van-hanh`: email nằm trong lệnh của pod (`kubectl describe` thấy khi pod còn sống) | Như job cũ; pod bị xoá khi xong |
| Ô "giá trị mới" tự ghi vào khung nhờ JavaScript nội dòng; Jenkins chặn JS thì gõ thẳng khung | Không mất chức năng |
| Form đọc cụm trên controller; controller không có `kubectl`/kubeconfig thì Vùng 1 và 3 chỉ hiện một câu | Job vẫn chạy đúng, kế hoạch in ở bước chạy thử |
