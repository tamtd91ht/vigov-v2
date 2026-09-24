---
id: ban-giao-phien
tier: T5
source: CURATED
owner: architecture
derived_from_commit: d4a9c18
expires: 2026-12-23
owns_facts:
  - "quyết định đã chốt với người dùng/khách, cạm bẫy đã gặp, và câu đang chờ người dùng — tại 24/09/2026"
---

# Bàn giao phiên — cập nhật 2026-09-24

**Đọc tệp này SAU `kb/INDEX.yaml` và tầng `always_load`, không thay thế chúng.** Nó chỉ trả lời:
*đã quyết gì, đang chờ ai, và cạm bẫy nào đã tốn thời gian của người trước.*

Viết bằng `/handover`. **MỘT tệp, ghi đè trọn vẹn mỗi lần** — tên không mang ngày, ngày nằm bên
trong. Hết hạn **2026-12-23**; sau ngày đó tin `git log`, đừng tin tệp này.

Mọi dòng giữ lại từ bản 22/09 (`ec80994`) đã được đối chiếu với đĩa ngày 24/09; dòng nào đã sai
thì đã bỏ, không giữ "cho đủ".

---

## 1. Đã làm — chỉ những gì `git log` không nói

**Không có danh sách commit ở đây.** `git log --oneline ec80994..HEAD` trả lời chính xác hơn.

### Quyết định đã chốt — và nơi ghi

Quyết định đã thành ADR thì chỉ trỏ, không chép. Quyết định chưa thành ADR thì nơi ghi là mục sổ
tiến độ tương ứng (khoá `tiep_theo`/`bang_chung`).

| Quyết định | Ngày | Ai quyết | Ghi ở |
|---|---|---|---|
| Tiền tố **`vigov`** cho tên ảnh và module | — | người dùng | `service-identity/Jenkinsfile` |
| Jenkins dùng **docker CLI trên agent**, không Kaniko | — | người dùng | Jenkinsfile từng dịch vụ |
| Hạ tầng dừng ở **Dockerfile + Jenkinsfile**; cụm k8s do devops (nay có hướng dẫn Rancher ở `deploy/README.md` §9, §11) | — | người dùng | `deploy/README.md` |
| Trần `always_load` **27000 token** — đừng nâng (xem §5) | 21/09 | người dùng | `kb/INDEX.yaml:15` |
| **Miễn xã** cho `ResolveCitizenSession`; `ListTenants`/`ResolveTenantSuccession` vẫn ngoài danh sách | 21/09 | người dùng | chú thích `core/grpcx/grpcx.go:158-177` (chưa có ADR — §3) |
| **Ingress SINH từ `openapi.json`**, không liệt kê tay | 21/09 | người dùng | `deploy/README.md`, `tools/ingress/` |
| Sổ **đơn thư công dân** (`don_thu`) thuộc **`documents`** | 24/09 | người dùng | ADR 0039 |
| Ô cấp quyền `vai_tro_quyen` là **cấu hình**: gỡ quyền = xoá cứng ô ấy kèm vết trước/sau đầy đủ; CHỈ bảng này | 24/09 | người dùng | ADR 0040 |
| **#12 là quyết định của KHÁCH (22/09)**, không phải đề xuất nhà cung cấp — sổ từng ghi sai | 24/09 | người dùng xác nhận | `service-identity/che-so-di-dong-can-bo` |
| Đồng ý công khai số lên Mini App, **bản gọn**: quản trị bật TỪNG người + tick "đã hỏi ý", máy chủ lưu thời điểm + mã người ghi; rút công khai xoá dấu đồng ý; khoá `content.update` | 24/09 | người dùng | `service-identity/danh-ba-can-bo-con-thieu` |
| Xoá dòng danh bạ **có tài khoản** → từ chối; tìm cán bộ theo tên/SĐT đi trong **thân POST**, không lên URL | 24/09 | người dùng | cùng mục trên |
| Nút `+ Thêm cán bộ` **giữ ở Cấu hình**, không đưa vào `/danh-ba` | 24/09 | người dùng | `web-admin/danh-ba-man-rieng` |
| Nhiệm vụ §7.2: mỗi văn bản = textarea Trích yếu bắt buộc + hai ô tuỳ chọn; `Ghi chú` ẩn ở form tạo; Sổ theo dõi §4.3 **dựng backend trước** | 24/09 | người dùng | `web-admin/man-nhiem-vu`, `service-petitions/so-theo-doi-can-tuyen-may-chu` |
| Mục menu Văn bản & Đơn thư canh bằng `document.read` | 24/09 | người dùng | commit b1da756 |
| Phân quyền: **cấm** lưu cột của vai trò mình đang giữ (#14); chặn lượt lưu làm xã mất người giữ `admin.user` **hoặc** `admin.role` (mở rộng #13); tuyến `PUT /api/v1/roles/{id}/permissions` | 24/09 | người dùng | `service-identity/cau-hinh-bon-chuc-nang-24-09` |
| Sơ đồ tổ chức: khoá `admin.org`, **chưa có Xoá**, nhận cả khối ngoài UBND | 24/09 | người dùng | cùng mục trên |
| `Loại đơn vị dân cư` và `Khối nhiệm vụ` là **danh mục đầy đủ**; Khối nhiệm vụ coi là khái niệm RIÊNG — trong khi câu ấy vẫn "phải hỏi anh Hà" (§3) | 24/09 | người dùng | cùng mục trên |
| Nhãn trạng thái nhiệm vụ: màn Nhiệm vụ đọc từ máy chủ, **một nguồn** | 24/09 | người dùng | `service-petitions/nhan-trang-thai-nhiem-vu-theo-xa` |
| Hợp đồng đổi làm web đỏ → **sửa hai phía nhỏ nhất**: trường PATCH mới phải tuỳ chọn; web chỉ sửa fixture | 24/09 | người dùng | commit 24dade9, ff4aaf2 |
| **Sổ đơn thư — 15 câu C3–C19** (bộ trạng thái theo TT 05/2021, 4 loại đơn, hai hạn theo SLA xã tính ngày làm việc, **che danh tính người tố cáo theo loại đơn + khoá riêng**, luật người đang giữ chỉ cho đơn thư, nhiệm vụ sinh từ đơn hạn 17:00 và cấm với tố cáo…). Chưa dựng — lượt sau dựng đúng theo đó, không hỏi lại | 24/09 | người dùng (chọn đề xuất) | `service-documents/so-don-thu-cong-dan` |
| Tuyến **danh bạ cán bộ hẹp** (AnyAuthenticated, không SĐT/email) cho ô chọn cán bộ ở 4 menu | 24/09 | người dùng | `service-identity/tuyen-danh-ba-can-bo-hep` |
| Đơn thư từ **Mini App**: hoãn | 24/09 | người dùng | `service-documents/don-thu-tu-mini-app` |
| **Báo công dân theo bảng** (6 chuyển trạng thái, không báo dang-phan-loai/dang-xu-ly/da-xu-ly, không bao giờ gửi tên cán bộ/nội dung/ảnh) | 24/09 | người dùng | ADR 0041 |
| Phản ánh: cờ ảnh nghiệm thu **giữ #7 mặc định BẬT**; **gia hạn** có (lãnh đạo duyệt, hạn gốc giữ); **tự đóng** phiếu chờ dân sau N ngày; 1–2 sao vào **hàng lãnh đạo xem**; ranh giới với đơn thư theo nội dung + hình thức (tố cáo cán bộ sang sổ đơn thư). Chưa dựng | 24/09 | người dùng (chọn đề xuất) | `service-petitions/vong-doi-phieu-phan-anh` |
| Tên URL danh bạ hẹp: **`staff-directory`** | 24/09 | người dùng | `ubiquitous-language.md` |
| Thu chi ngân sách: **số lưu đồng, web quy đổi theo đơn vị bảng** (đơn vị đóng dong/nghin-dong/trieu-dong); **dựng đợt thu chi theo đặc tả** (khác khuyến nghị domain-expert — sổ thứ hai cạnh Kho bạc); URL `budget-entries`, ghi `budget.update`, gỡ `budget.confirm`; KPI cân đối **hỏi khách**, nhãn tạm "Chênh lệch thu – chi luỹ kế" | 25/09 | người dùng | `service-finance/thu-chi-ngan-sach-82` |
| Hai nhánh phiếu: **`…/rejection` · `…/referral`**, khoá **`feedback.classify`**; máy chủ **kiểm người được giao** qua identity (RPC mới `ResolveAssignableStaff`) | 25/09 | người dùng | `ubiquitous-language.md` · `service-petitions/duong-xu-ly-phan-anh-phia-can-bo` |
| Mini App phản ánh: **dựng màn, nguồn phiên ViGov fail closed**, ẩn khỏi bản nộp — chưa có cầu phiên vihat-miniapp → ViGov | 24/09 | người dùng | `citizen-app/cau-phien-cong-dan-vigov` (cần ADR) |

**Cần biết về 34 câu trong `open-questions.json`:** cả 34 đều DECIDED, nhưng nhiều câu (#21, #27 và
mười ba câu khác) là **đề xuất của nhà cung cấp** ghi ở ADR 0035, không phải trả lời của khách. Đọc
bảng "cái gì đỏ nếu một mục bị phủ quyết" ở cuối ADR 0035 trước khi dựa vào một câu như thế.

---

## 2. Việc kế tiếp

**Không nằm ở đây.** `kb/90-ephemeral/tien-do.md` (theo module) và `python tools/tien_do.py --menu
"<menu>"` (theo menu). Ngày 24/09 `progress-reviewer` đã soát và 18 mục được thêm dòng "SỬA 24/09"
ở đầu `tiep_theo` — đọc dòng ấy trước phần còn lại của mục.

---

## 3. Đang bị chặn — và chặn bởi ai

Bảng *"Nợ khách chốt"* ở đầu `tien-do.md` sinh từ `no_confirm`; hôm nay nó RỖNG vì mọi câu trong
`open-questions.json` đã DECIDED. Những câu dưới đây **không có trong tệp ấy** — chúng chờ NGƯỜI DÙNG
(hoặc người dùng chuyển cho khách), và chưa ai trả lời lúc viết:

| Câu | Chặn gì | Chi tiết ở |
|---|---|---|
| **web-admin ra ngoài bằng đường nào** — A `node:http` + gốc nội bộ · B mở 443 · C biên đọc `X-Forwarded-Host` | **BLOCKER PHÁT HÀNH**: mọi trang web-admin 500. Kiểm lại 24/09: KHÔNG commit nào đụng `edge.go`, `netpol.yaml`, `tenant-config.ts` | `web-admin/goc-api-noi-bo` (treo) · §5 |
| Xã mới lấy **người quản trị, vai trò và quyền đầu tiên** bằng cách nào (ADR 0003 cấm nhà cung cấp đụng dữ liệu nghiệp vụ; #13 cấm lối khôi phục của nhà cung cấp) | Ở một xã thật **không tài khoản nào cầm khoá nào** → ~97 tuyến 403, kể cả nút gieo SLA | `service-identity/xa-moi-khong-co-vai-tro-va-quyen` — cần ADR |
| `/api/v1/<chưa định tuyến>` trả **404 HTML của web** hay **404 JSON** | hình dạng hiện tại là HTML | `deploy/base/mang/ingress.yaml:304-306` |
| Miễn xã cho `ResolveCitizenSession` có cần **ADR riêng** không | không chặn mã | chú thích `core/grpcx/grpcx.go:158` |
| **Khối nhiệm vụ** có phải "khối đơn vị" của danh bạ không (hỏi anh Hà) | người dùng đã quyết tạm là KHÁC; nếu anh Hà nói CÙNG thì phải xem lại F2 | `kb/50-doi-chieu/2026-09-23-feat-m8-multitenant-foundation.md:98` |
| Phạm vi đồng ý #12: đổi số di động của người **đang** công khai có phải hỏi lại; **khoá** người đang công khai có rút công khai; cán bộ có tự ghi đồng ý cho mình | số mới lên kênh công khai dưới đồng ý cũ; người đã nghỉ vẫn hiện số | `service-identity/danh-ba-can-bo-con-thieu` |
| Cột đầu Kanban mặc định **"Mới giao"** hay **"Chưa thực hiện"** (spec 02 §6 có hai tên cho một mã) | nay hiện "Mới giao" | `web-admin/cau-hinh-bon-chuc-nang-web` |
| Mục menu `/cau-hinh` chỉ canh `admin.lookup` | người cầm `admin.org`/`admin.role`/`admin.sla` không vào được | cùng mục trên |
| Hai câu ở `deploy/README.md` mục 11.0 (dải CIDR netpol, KUBECONFIG theo môi trường) | phiên CI/deploy chờ để sửa `netpol.yaml`, `deploy/Jenkinsfile` | `deploy/README.md` §11.0 |
| **Cờ ảnh nghiệm thu phản ánh**: vigov-require `b9a9718` mặc định TẮT, câu #7 (ADR 0008) chốt mặc định BẬT — mâu thuẫn với câu KHÁCH đã chốt | cờ chưa dựng; ai dựng phải hỏi trước | `service-petitions/doi-chieu-24-09-nhiem-vu-phan-anh` |
| **Thu chi: biểu mẫu thật của xã** (tệp mẫu của đặc tả có vẻ của HUYỆN) · KPI cân đối dòng B hay #32 · "cha = tổng con" quyết lại · chốt kỳ/quyết toán/người duyệt · công khai ngân sách | nhập Excel, số liệu nộp lên cấp trên | `service-finance/thu-chi-ngan-sach-82` |
| **Cầu phiên công dân**: đăng nhập Mini App (vihat-miniapp) không ra được CitizenSession ViGov — không công dân thật nào gọi được tuyến CitizenOnly | cả kênh công dân | `citizen-app/cau-phien-cong-dan-vigov` |
| **Bộ trạng thái riêng của VĂN BẢN ĐẾN** (C2; domain-expert đề xuất theo NĐ 30/2020) | tuyến đổi trạng thái văn bản đến | `service-documents/van-ban-den-tuyen-con-thieu` |
| Ngày làm việc hay ngày lịch cho hạn KN Đ.28 / TC Đ.29 — **hỏi pháp chế**; và cần ADR vì ADR 0007 tính GIỜ | gieo số SLA đơn thư | `service-documents/so-don-thu-cong-dan` |

Và **mười hai xung đột yêu cầu** của sổ đơn thư (C2–C13) cùng **tám** của danh bạ (U1–U8) — ghi
nguyên hai phía ở `service-documents/so-don-thu-cong-dan` và `service-identity/danh-ba-can-bo-con-thieu`;
chúng là câu của cổng `/develop-backend-api` lượt sau, chưa ai quyết.

---

## 4. Phiên song song

Ngày 24/09 có bốn phiên cùng chạy kho này (`vigov-v2-6d`, `-0b`, `-ca`, `-f9`). **Lúc viết, cả ba phiên
kia đã kết thúc và không giữ đường dẫn nào**, không việc dở chưa commit (hỏi trực tiếp từng phiên).
Phiên CI/deploy (`-f9`) có thể quay lại sửa `deploy/base/mang/netpol.yaml` và `deploy/Jenkinsfile` sau
khi người dùng trả lời §11.0. Phiên sau tự kiểm lại bằng ListAgents — đừng tin dòng này.

---

## 5. Cạm bẫy đã gặp — đọc để khỏi mất thời gian lại

Một nửa bảng này có chung một hình dạng: **thứ trông như biện pháp mà không phải biện pháp** — một
phép kiểm xanh vì lý do sai. Gặp cái tiếp theo cùng dạng thì hỏi cả lớp ấy còn ở đâu.

### Rào chắn và công cụ sinh

| Triệu chứng | Sự thật |
|---|---|
| `make kb` in `ĐÃ CÓ MÀN HÌNH → done/` cho một tuyến web **chưa hề gọi** | `tools/apidoc` khớp ĐƯỜNG DẪN mà mù PHƯƠNG THỨC: web gọi `PATCH /staff/{id}` là `DELETE /staff/{id}` bị coi là xong. Mỗi lần `make kb` nó sinh lại `tasks/web/done/c5eb6691f6de.json` — **đừng commit tệp ấy**, dời ra rồi mới sinh `tien-do-san-pham.md` (báo cáo đếm từ đĩa). `_chung/apidoc-khop-man-hinh-mu-phuong-thuc` |
| Thêm một trường vào thân PATCH, Go xanh, rồi **web-admin vỡ bản dựng** lúc `gen:api` | `*T` không kèm `omitempty` → apidoc khai trường **bắt buộc** (tools/apidoc/schema.go:723): vừa thêm một trường bắt buộc vào hợp đồng đã công bố. Trường mới: luôn `omitempty`. Trường mới trong PHẢN HỒI thì làm đỏ fixture kiểm thử web — sửa fixture trong cùng commit hợp đồng |
| `data_safety_guard` chặn một tệp **.md** | Nó quét cả văn xuôi; câu TRÍCH luật 7 cũng bị chặn. Ngược lại, câu xoá cứng thật trong Go có thể **im lặng** chỉ vì không có chữ nghiệp vụ trong ~150 ký tự quanh nó. Phán quyết là may rủi của chữ, không phải quyết định. `_chung/data-safety-guard-chan-cau-trich-trong-md` — **đừng đổi chữ để lách** |
| `workflow_guard` nêu tên phiên ở mọi lần dừng, kể cả lượt chỉ `git log` | Nó đếm tệp theo cả cửa sổ phiên. `_chung/workflow-guard-dem-ca-cua-so-phien` |
| codegraph trả ký hiệu thật nhưng **không phải của kho này** | MCP toàn cục trỏ dự án khác. **Luôn truyền `projectPath`**; kiểm `codegraph_status` phải có go + typescript, không java. Hook `codegraph_sync` giữ chỉ mục mới sau commit |
| Tầng always_load ~26980/27000 | Mục kế tiếp đăng ký vào `kb/INDEX.yaml` sẽ đỏ `check_brain` #5. **Đừng nâng trần**; nhường chỗ bằng cách bỏ chữ trùng. `_chung/tang-luon-nap-da-day` |
| `make web` / vitest đổ `Fatal process out of memory: Zone`, "Tests 357 passed (422)" | Worker chết vì bộ nhớ, không phải ca đỏ. `npx vitest run --maxWorkers=2` thì xanh; đừng chạy song song với một lượt biên dịch Go |
| `make check` đổ ở `envmap` với `UnicodeEncodeError: 'charmap'` | Console Windows cp1252, không phải lỗi mã. Chạy `PYTHONIOENCODING=utf-8 mingw32-make check` |
| `tools/apidoc` không sinh được `enum` cho trường | Chỉ có enum cho sort/dir của `@page`. `service-documents/apidoc-sinh-enum-cho-truong` |
| IDE báo hàng chục lỗi biên dịch Go/TS ngay sau khi agent sửa | Chẩn đoán của language server chụp GIỮA chừng. Tin `go vet`/`go test`/`tsc`, không tin bảng lỗi IDE |
| Commit 1145971 sửa chú thích trong hai migration **0001 đã áp** | `core/migrate/migrate.go:325` băm cả tệp → CSDL nào đã áp sẽ lệch checksum. `_chung/migration-0001-da-ap-bi-sua-chu-thich` |

### Máy này (Windows, bộ nhớ hạn chế)

| Triệu chứng | Sự thật |
|---|---|
| `mingw32-make check` đỏ ở `build` với stack trace của `cmd/link` | Hết bộ nhớ khi dựng 9 module MỘT lượt (`Makefile:119-120`). Dựng **từng module** thì xanh (24/09: cả 9 OK) |
| `tools/test_hooks.py` báo thất bại `exit=3221225773`/`3221225794` | Mã sập tiến trình Windows (hết bộ nhớ), không phải hook sai. Chạy lại khi máy rảnh. **Cổng đỏ sai lý do** — đừng đi vá rào không hỏng |
| `go test -race` đổ ở link, `error code 1455` | `ERROR_COMMITMENT_LIMIT`. Luôn `-p 1`; không quá 2 agent biên dịch Go cùng lúc, 1 là an toàn |
| Ca `_pg_test` "xanh" | Là **SKIP** (`VIGOV_TEST_DSN` trống) — gói vẫn in `ok`. Nhận ra bằng thời gian: ~0,02 s là bỏ qua, vài giây là chạy thật. Docker Desktop hay tắt/đổ trên máy này |
| `sed -i`, `awk`, `find` bị từ chối trong Bash | Chính sách quyền của phiên. Sửa JSON bằng python, tìm bằng Grep/Glob |

### Máy build (CentOS 7) và triển khai

| Triệu chứng | Sự thật |
|---|---|
| Cổng kiểm trên Jenkins đổ ngay mục đầu với `SyntaxError: Non-ASCII character` | `python` là 2.7, `python3` là 3.6.8; 3.9 ở `/usr/local/bin`. `PYTHON ?= python` + Jenkinsfile tự chọn (006037d, c506d1b, 0918e23) |
| Ca kiểm hook xanh ở máy trạm, đỏ trên CI | Hệ tệp Windows không phân biệt hoa thường che lỗi đường dẫn viết thường (47f5bf0). Ca phụ thuộc đĩa thật là ca không chạy như nhau ở hai nơi |
| Thư mục `<tên>@tmp/` lọt vào ngữ cảnh docker build | `dir()` của Jenkins để lại nó; `.dockerignore` có `*@tmp`, Jenkinsfile dùng `cd` (e0e1236, 29f8742) |
| Rancher/RKE2 | netpol nhận REST chỉ từ namespace `ingress-nginx` nhưng RKE2 để controller ở `kube-system`; chỉ có flannel thì NetworkPolicy **không được cưỡng chế**; Import YAML không chạy kustomize; kubeconfig tải từ Rancher mang quyền người tải. `deploy/README.md` §11 |
| Ingress dựng tay | Tuyến quên khai rơi về `/` và nhận 404 của web-admin; không bao giờ ghi lại Host; không mở 9090. `deploy/README.md` §9 |
| `deploy/README.md:270` nói web-admin gọi `/api/v1` tương đối, "không cần địa chỉ backend" | **Sai với phía máy chủ**: `tenant-config.ts:121` gọi `https://<Host>/api/v1/communes/current` mỗi trang → cần egress mà netpol không cho. Câu README che mất blocker |

### web-admin gọi ra ngoài — ba số đo (kiểm lại 24/09: vẫn đúng cả ba)

1. `fetch` của Node **không gửi được `Host`** do người gọi đặt (forbidden header; undici ghi đè). `node:http` thì được. Mã vẫn dùng `fetch` (`web-admin/src/lib/tenant-config.ts:121-129`).
2. `core/httpx/edge.go:24` **chỉ đọc `r.Host`**, không `X-Forwarded-Host`. Đổi gốc mà bỏ Host → 404 mọi trang (404 hợp lệ nên không ai nghi). "Chữa" bằng cách đăng ký host nội bộ thành một xã → mọi tên miền in tên đúng xã ấy: **rò giữa hai cơ quan** (luật 1).
3. REST identity là **8080** (`deploy/base/identity/service.yaml:8`), `netpol.yaml` chỉ mở **9090** và 443 thì không. Không phương án nào tránh được một dòng egress.

Kèm theo: bộ kiểm hiện tại thay `globalThis.fetch` rồi đọc `mock.calls[0]`, mà `Headers` GIỮ `Host` — nên ca "vẫn mang Host của xã" XANH trong khi sản xuất gửi host sai.

### Vẫn đúng từ bản trước (đã kiểm lại)

| Vấn đề | Cách xử |
|---|---|
| **Kết quả grep âm tính không phải bằng chứng vắng mặt** (template literal, gán qua biến) | Mở tệp, không kết luận từ grep |
| **Bản vá cho lỗi X dễ là một thể hiện mới của X** | Sau khi vá một hook, chạy nó trên kho thật và mở từng tệp nó tố cáo |
| **`git checkout --` / `git restore` để hoàn tác đột biến** xoá việc chưa commit của phiên khác | Chụp tệp ra thư mục tạm, hoặc Go `-overlay` — agent hôm nay đều làm vậy |
| `SET search_path` là trạng thái SESSION | Suite tích hợp phải `SetMaxOpenConns(1)` |
| Trigger chỉ-thêm của `audit_log` **chưa từng bị bắn thử** từ chối một lệnh xoá | Vẫn chưa ca nào (`_chung.json`, rào đối chiếu) |
| Hai cột `bool` cạnh nhau, đọc theo vị trí trong `Scan` | Lỗi im lặng đối xứng; ca bắt được là giá trị KHÁC nhau. Hôm nay test-designer thêm driver đối chiếu `cột = $n` với đối số n |
| `fmt` không gọi `String()` cho `%d %c %U %b %o` | `core/secret` cài `fmt.Formatter` |
| `buf lint STANDARD` ép tên message theo RPC | Kiểu trả về phải bọc |
| Hàng đợi việc sửa tay đã sai 36/36 lần | Để `tools/apidoc` suy từ mã — nhưng xem dòng đầu §5 về bộ khớp mù phương thức |

---

## 6. Cổng kiểm

```
mingw32-make check      # hoặc, trên máy này: build/vet/test TỪNG module với -p 1
```

**24/09, chạy thật** ở `d4a9c18`: cả 9 module Go build + vet + `go test -count=1 -p 1` xanh (từng
module, không một lượt); web-admin tsc · lint · vitest · `check:api` xanh; `check_brain`,
`test_hooks`, `check_quyen`, `check_audit_actor`, `check_khoa_duy_nhat`, `check_env_map`,
`check_build` đều PASS. **Không có số liệu ở đây, cố ý** — con số sai trông y hệt con số đúng.

**Thứ cổng KHÔNG phủ — đọc kỹ, đây là thứ quyết định "xanh" nghĩa là gì:**

| Không phủ | Hệ quả |
|---|---|
| **PostgreSQL thật** | Mọi `*_pg_test.go` SKIP ở máy này (Docker tắt). Tức CHECK/trigger/khoá duy nhất/FOR UPDATE của mọi migration và đường ghi mới hôm nay **chưa được máy chủ thật thi hành**. Riêng `PUT /api/v1/roles/{id}/permissions` **không có ca pg nào**, kể cả ca SKIP. Dựng lại bằng `go run ./tools/schema-smoke` (thiếu DSN thì thoát 2) |
| **`golangci-lint`** | Không có trên máy này lẫn máy Jenkins; `lint` bỏ qua nó bằng tiền tố `-` |
| **Jenkins cho ViGov** | Cổng kiểm đã chạy thật trên máy build (c8973ce; lượt 6 phần Go xanh). Người dùng báo ảnh đã lên Harbor — **chưa ai đọc số build hay tag**. `vigov-deploy` chưa từng chạm cụm |
| **k8s** | `kubectl kustomize` chứng minh YAML dựng được, KHÔNG chứng minh API server nhận. Chưa lượt deploy nào |
| **Trình duyệt** | Không ai mở màn web ở 320px, không kiểm tiêu điểm/Esc; vitest chạy node không DOM — effect, handler click, luồng async chỉ được phủ qua hàm thuần. Ca kiểm "đọc mã nguồn" chỉ bắt HÌNH DẠNG cú pháp |
| **Hook có NHÌN THẤY gì không · cảnh báo có ĐÚNG không** | `check_brain` kiểm luật có nêu tên hook, không kiểm hook đọc được gì. Phép thử duy nhất đáng tin là đột biến. Cảnh báo là chỗ đáng nhìn, không phải phán quyết |

---

## 7. Việc treo

**Không nằm ở đây.** Mỗi việc treo ở module của nó với `trang_thai: "treo"` — `kb/90-ephemeral/tien-do.md`.
Phân biệt: **`treo`** là *không ai chặn, ta chọn chưa làm*; **bị chặn** là chờ một câu ở §3.
