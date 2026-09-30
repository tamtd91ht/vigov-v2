---
id: ban-giao-phien
tier: T5
source: CURATED
owner: architecture
derived_from_commit: 417e9564
expires: 2026-12-29
owns_facts:
  - "quyết định đã chốt với người dùng/khách, cạm bẫy đã gặp, và câu đang chờ người dùng — tại 30/09/2026"
---

# Bàn giao phiên — cập nhật 2026-09-30

**Đọc tệp này SAU `kb/INDEX.yaml` và tầng `always_load`, không thay thế chúng.** Nó chỉ trả lời:
*đã quyết gì, đang chờ ai, và cạm bẫy nào đã tốn thời gian của người trước.*

Viết bằng `/handover`. **MỘT tệp, ghi đè trọn vẹn mỗi lần** — tên không mang ngày, ngày nằm bên
trong. Hết hạn **2026-12-29**; sau ngày đó tin `git log`, đừng tin tệp này.

Viết lại 30/09 sau hai việc: gỡ chiến dịch đổi tên (ADR 0061) và lượt `/develop-feature
phan-anh-nguoi-dan` nối chuỗi Mini App → phiếu → web admin. Bản trước ở `b736232` (28/09).
**Cách kiểm các dòng giữ lại:** mọi con trỏ sổ tiến độ ở §1/§3 đã đối chiếu 30/09 là còn tồn tại;
nội dung dòng trước 29/09 **không** kiểm lại từng chữ. Dòng nào đã sai thì đã sửa hoặc bỏ, có ghi.

---

## 1. Đã làm — chỉ những gì `git log` không nói

**Không có danh sách commit ở đây.** `git log --oneline b736232..HEAD` trả lời chính xác hơn.

### Quyết định 29–30/09 — mới

| Quyết định | Ngày | Ai quyết | Ghi ở |
|---|---|---|---|
| **GỠ TOÀN BỘ chiến dịch đổi tên sang tiếng Anh** (ADR 0061, lớp A/B, từ điển X1–X40). Revert 15 commit, cây về `7ff76a1`, giữ bản sửa finance. ADR 0061 và từ điển đổi tên **không còn** trong `kb/`. Luật 12 về bản cũ: **tên MỚI tiếng Anh, tên CŨ không đổi** (ADR 0051). Đừng đề xuất đổi tên hàng loạt lại | 30/09 | người dùng | commit `090d6b0c` · `.claude/rules/critical/12-english-identifiers.md` |
| Giữ lại khi gỡ: tên object/nhãn k8s khớp cụm thật (`common-config`, `<service>-secrets`, `48970cb`) và lỗi khởi động tự giải thích (`7ff76a1`) | 30/09 | người dùng | commit `090d6b0c` |
| Việc đổi tên identity/petitions lớp A đang dở: **bỏ hẳn**. Còn nằm trong `git stash` trên MỘT máy (máy của phiên 30/09) — không phải trạng thái hiện hành, đừng áp lại | 30/09 | người dùng | `git stash list` trên máy ấy |
| Chưa có **dữ liệu thật** trên môi trường nào; migration platform `0012` (đổi tên, đã gỡ) chưa chạy ở đâu | 30/09 | người dùng | `4603450`, `090d6b0c` |
| **Mini App: bản test là bản thật sẽ submit.** Gọi hàm thật (zmp-sdk, `vihat-miniapp`, API ViGov), không nhánh giả/dữ liệu mẫu; quyền Zalo chưa cấp thì lời gọi cứ hỏng và **màn hình báo lỗi** (tên quyền + mã lỗi SDK). **Không** có nghĩa là nới kiểm phân quyền/xã phía ViGov | 30/09 | người dùng | `citizen-app/bao-dung-loi-quyen-zalo` |
| Ưu tiên: **nối thông chuỗi** Mini App → phiếu lưu đúng xã → Sổ phản ánh của xã, chạy thật trên bản test — trước mọi tính năng mới của menu Phản ánh | 30/09 | người dùng | `deploy/mo-cong-cau-phien-cong-dan` |
| `vihat-miniapp` chạy **cùng cụm, cùng namespace** với ViGov; cổng cầu phiên identity = **9091** | 30/09 | người dùng | `deploy/base/identity/*`, `deploy/base/mang/netpol.yaml` quy tắc 8–9 |
| Cụm thật chạy **PostgreSQL 16** (không `ALTER COLUMN … SET EXPRESSION`); Mini App **chưa có bản nào lên Zalo** tính tới 29/09 — hỏi lại nếu đã quá vài tuần | 29/09 | người dùng | (trước ghi ở ADR 0061, đã gỡ theo — nay chỉ ở đây) |
| Tám quyết định con Cấu hình 29/09 (leo thang, bản tin thứ Hai, nhập Excel cán bộ/thôn, đơn thư, mốc việc chưa giao…) | 29/09 | người dùng | ADR 0058 · 0059 · 0029 |

### Quyết định trước 29/09 — giữ từ bản 28/09

| Quyết định | Ngày | Ai quyết | Ghi ở |
|---|---|---|---|
| Tiền tố **`vigov`** cho tên ảnh và module | — | người dùng | `service-identity/Jenkinsfile` |
| Jenkins dùng **docker CLI trên agent**, không Kaniko | — | người dùng | Jenkinsfile từng dịch vụ |
| Hạ tầng dừng ở **Dockerfile + Jenkinsfile**; cụm k8s do devops | — | người dùng | `deploy/README.md` §9, §11 |
| Trần `always_load` **28000 token** (27000 → 28000 ngày 28/09 cho luật 13) — đừng tự nâng (xem §5) | 21/09 · 28/09 | người dùng | `kb/INDEX.yaml` `budget` |
| **Miễn xã** cho `ResolveCitizenSession`; `ListTenants`/`ResolveTenantSuccession` vẫn ngoài danh sách | 21/09 | người dùng | chú thích `core/grpcx/grpcx.go` quanh `MethodResolveCitizenSession` (chưa có ADR — §3) |
| **Ingress SINH từ `openapi.json`**; web-admin là cổng `/api/v1/*`; k8s chỉ cấp Secret + ConfigMap; `/api/v1` lạ nhận **404 JSON** | 21/09 · 25/09 | người dùng | ADR 0043 · `tools/ingress/` |
| Quy hoạch **4 tên miền** và **người quản trị đầu tiên** của xã | 26/09 | chủ dự án | ADR 0046 · `service-identity/xa-moi-khong-co-vai-tro-va-quyen` |
| Mini App **hai chế độ**: app chính mở xã bằng QR, app riêng gắn xã theo App ID, một bản build | 25/09 | người dùng | ADR 0044 |
| **Cầu phiên công dân** Mini App → ViGov (`OpenCitizenSession`, `ResolveMiniApp`) | 26/09 | người dùng | ADR 0045 · `citizen-app/cau-phien-cong-dan-vigov` |
| **Hai giai đoạn với một xã**: GĐ1 app chung của ViHAT + QR gắn tên miền xã; GĐ2 app riêng, App ID riêng. "Demo" là tên giai đoạn, về kỹ thuật là app thật | 27/09 | chủ dự án | ADR 0047 §Trả lời 27/09 |
| Build: có `domain` → App ID riêng, không có → App ID chung; một biến thể | 27/09 | chủ dự án | ADR 0047 · `citizen-app/scripts/ung-dung-theo-ten-mien.mjs` |
| Tham số QR mang **tên miền xã**, bỏ `t=<ULID>`; máy chủ tra ra xã, chỉ lưu ULID. App chung: dân xác nhận một lần; app riêng: không | 27/09 | chủ dự án | ADR 0047 |
| Rủi ro **chấp nhận**: trỏ tên miền đúng lúc dân đang xác nhận → vào xã chưa thấy tên | 27/09 | chủ dự án | ADR 0047 §Trả lời mục 4 |
| **Cho phát hành app riêng trước khi đo UNKNOWN #1**; `ZMP_TOKEN` là credential Jenkins, mỗi App ID một token | 27/09 | chủ dự án | ADR 0047 D8 |
| Tuyến tra tên miền → tên xã thuộc **service-identity** | 27/09 | chủ dự án | ADR 0047 D6 |
| Khu vận hành ViHAT dựng trong **`platform-admin/`**, backend `service-platform`; 12 câu thiết kế **chưa chốt** | 27/09 | chủ dự án | ADR 0048 §Sửa của chủ dự án |
| Nhiệm vụ: mũi tên **trả lại để làm tiếp** `cho-duyet → dang-thuc-hien` | 28/09 | chủ dự án | `service-petitions/tra-lai-nhiem-vu-lam-tiep` |
| Lãnh đạo giao việc: máy chủ chỉ kiểm cán bộ đang hoạt động của xã; ô chọn lọc `staff-directory?permission=task.extend` | 28/09 | chủ dự án | `service-petitions/lanh-dao-giao-viec-khong-kiem-khi-tao` · `service-identity/danh-ba-hep-loc-quyen-task-extend` |
| Danh bạ: đổi số hoặc khoá người đang công khai → tự gỡ khỏi Mini App; kênh công khai trả số **không che** cho người đã đồng ý; Excel vẫn che | 28/09 | chủ dự án | `service-identity/rut-cong-khai-khi-doi-so-hoac-khoa` · `che-so-di-dong-can-bo` |
| Sổ **đơn thư công dân** thuộc **`documents`** | 24/09 | người dùng | ADR 0039 |
| Ô cấp quyền `vai_tro_quyen` là **cấu hình**: gỡ = xoá cứng ô ấy kèm vết | 24/09 | người dùng | ADR 0040 |
| **#12 là quyết định của KHÁCH (22/09)** | 24/09 | người dùng xác nhận | `service-identity/che-so-di-dong-can-bo` |
| Công khai số lên Mini App bản gọn; xoá dòng danh bạ có tài khoản → từ chối; tìm cán bộ trong thân POST | 24/09 | người dùng | `service-identity/danh-ba-can-bo-con-thieu` |
| Nút `+ Thêm cán bộ` giữ ở Cấu hình | 24/09 | người dùng | `web-admin/danh-ba-man-rieng` |
| Tuyến **danh bạ hẹp** `staff-directory` (AnyAuthenticated, không SĐT/email) | 24/09 | người dùng | `service-identity/tuyen-danh-ba-can-bo-hep` |
| Phân quyền: cấm lưu cột của vai trò mình đang giữ; chặn lượt lưu làm xã mất người giữ `admin.user`/`admin.role`. Sơ đồ tổ chức: khoá `admin.org`, chưa có Xoá | 24/09 | người dùng | `service-identity/cau-hinh-bon-chuc-nang-24-09` |
| Mục menu `/cau-hinh` hiện khi cầm **bất kỳ** khoá canh một tab | 26/09 | chủ dự án | `web-admin/cau-hinh-bon-chuc-nang-web` |
| Hợp đồng đổi làm web đỏ → sửa hai phía nhỏ nhất: trường mới tuỳ chọn; web chỉ sửa fixture | 24/09 | người dùng | commit 24dade9, ff4aaf2 |
| Nhiệm vụ §7.2, Sổ theo dõi §4.3, nhãn trạng thái một nguồn; lượt 27/09 (`task.read`, `task-extensions`, `approver=me`) | 24/09 · 27/09 | người dùng | `web-admin/man-nhiem-vu` · `service-petitions/nhan-trang-thai-nhiem-vu-theo-xa` · `hop-dong-nhiem-vu-thieu-ba-mon` |
| **Sổ đơn thư — 15 câu C3–C19**; chưa dựng — dựng đúng theo đó, không hỏi lại. Đơn thư từ Mini App: hoãn | 24/09 | người dùng | `service-documents/so-don-thu-cong-dan` · `don-thu-tu-mini-app` |
| **Báo công dân theo bảng** (6 chuyển trạng thái; không gửi tên cán bộ/nội dung/ảnh) | 24/09 | người dùng | ADR 0041 |
| Phản ánh: **1–2 sao tự mở lại**, không trần, không tính lại hạn (ADR 0050 điểm 2, thay quyết định 27/09); gia hạn có; tự đóng phiếu chờ dân sau N ngày | 24/09 · 28/09 | người dùng | `service-petitions/vong-doi-phieu-phan-anh` |
| Hai nhánh phiếu `…/rejection` · `…/referral`, khoá `feedback.classify` | 25/09 | người dùng | `service-petitions/duong-xu-ly-phan-anh-phia-can-bo` |
| Biên bản họp: dự thảo → đã ký (`task.approve`), sai thì biên bản bổ sung | 25/09 | người dùng | `service-petitions/bien-ban-hop-tang-du-lieu` |
| Thu chi: số lưu đồng, web quy đổi; URL `budget-entries`; KPI cân đối hỏi khách | 25/09 | người dùng | `service-finance/thu-chi-ngan-sach-82` |

**Về `open-questions.json`:** 39 câu; **#35–#39 OPEN** (mức nền an ninh mạng TCVN 14423: mật khẩu,
MFA, khoá phiên khi không dùng, khoá tài khoản khi sai). Nhiều câu DECIDED (#21, #27 và các câu ở
ADR 0035) là **đề xuất của nhà cung cấp**, không phải trả lời của khách — đọc bảng "cái gì đỏ nếu
bị phủ quyết" cuối ADR 0035 trước khi dựa vào.

---

## 2. Việc kế tiếp

**Không nằm ở đây.** `kb/90-ephemeral/tien-do.md` (theo module) và `python tools/tien_do.py --menu
"<menu>"` (theo menu). Đọc dòng "SỬA dd/09" ở đầu `tiep_theo` trước phần còn lại của một mục.
Chuỗi Mini App → phiếu: `python tools/tien_do.py --menu "phan anh nguoi dan"`, mục
`deploy/mo-cong-cau-phien-cong-dan` liệt kê việc vận hành theo thứ tự.

---

## 3. Đang bị chặn — và chặn bởi ai

Bảng *"Nợ khách chốt"* ở đầu `tien-do.md` sinh từ `no_confirm` — nay có **#35–#39** (an ninh). Những
câu dưới đây **không có trong tệp ấy** — chúng chờ NGƯỜI DÙNG, VẬN HÀNH hoặc KHÁCH:

| Câu | Chờ ai | Chặn gì | Chi tiết ở |
|---|---|---|---|
| **Chuỗi Mini App → phiếu chạy thật:** chép manifest lên Rancher, khoá cầu hai phía, CORS, dòng `mini_app` (stage `gan-mini-app-thang-binh`), xã khởi tạo SLA + tuần làm việc, `VIGOV_API_HOST` + `ZMP_TOKEN`, tên miền + quyền trên console Zalo | vận hành · quản trị viên xã · chủ dự án | mọi tuyến `CitizenOnly` trên máy thật | `deploy/mo-cong-cau-phien-cong-dan` · `deploy/gan-mini-app-thang-binh` |
| **Mã lỗi hiện cho MỌI người dân**: `skills/accessibility-elderly` REQUIRED #5 "không bao giờ hiện mã lỗi" ↔ người dùng 30/09 "chỗ nào cần quyền thì báo lỗi" (bản test = bản phát hành) | người dùng | câu lỗi quyền Zalo | `citizen-app/bao-dung-loi-quyen-zalo` |
| **Đo trên máy thật**: `-201` có đúng là mã từ chối của số điện thoại/vị trí (SDK còn `-2002`, `-1401`); UNKNOWN #1 của ADR 0045; dạng thật của `/v2.0/me` | người có console Zalo | câu "bạn đã từ chối" có đúng lúc không | `citizen-app/bao-dung-loi-quyen-zalo` · README `vihat-miniapp` |
| **Bốn xung đột Phản ánh C1–C4** (ảnh sau xử lý bật/tắt · cán bộ ghi nhận đánh giá · phiếu công khai trên Mini App · câu hộp nhập hộ) — trích nguyên hai phía | người dùng / khách | ảnh sau xử lý, nhập hộ, kiểm duyệt công khai | `service-petitions/vong-doi-phieu-phan-anh` (C1, C2, C4) · `phan-anh-tuyen-cong-dan-con-thieu` (C3) |
| **Tạo nhiệm vụ từ phiếu**: `2d34eba4` từ chối nguồn `phan-anh` trên `POST /tasks`, nay **không còn lối nào**; cần danh từ URL + cặp quyền. Kèm `task.approve` vừa ký biên bản vừa duyệt nhiệm vụ | người dùng | liên kết phiếu ↔ nhiệm vụ | `service-petitions/nguon-phan-anh-source-id-khong-kiem` (xong) · `task-approve-hai-viec` |
| Vòng đời nhiệm vụ lệch `vigov-require` + bảy xung đột, mỗi câu có đề xuất domain-expert 28/09 | khách | bảng chuyển trạng thái, ô sửa mã/hạn | `service-petitions/doi-chieu-26-09-nhiem-vu-truoc-neo` · `doi-chieu-24-09-nhiem-vu-phan-anh` |
| Danh bạ #12 phần còn lại (F1–F3, tự ghi đồng ý; phạm vi đồng ý khi đổi số) | người dùng | tuyến công khai Mini App | `service-identity/rut-cong-khai-khi-doi-so-hoac-khoa` · `danh-ba-can-bo-con-thieu` |
| Cột đầu Kanban "Mới giao" hay "Chưa thực hiện" | khách | nhãn | `web-admin/cau-hinh-bon-chuc-nang-web` |
| Gỡ `IDENTITY_ADMIN_SEED_PASSWORD` khỏi Secret khi mọi xã đã đổi mật khẩu (biến còn đọc ở `service-identity/cmd/server/main.go`) | vận hành | một bí mật mở `admin` ở mọi xã chưa đổi | `service-identity/xa-moi-khong-co-vai-tro-va-quyen` |
| Miễn xã cho `ResolveCitizenSession` có cần ADR riêng | người dùng | không chặn mã | `core/grpcx/grpcx.go` |
| Khối nhiệm vụ có phải "khối đơn vị" của danh bạ | đầu mối nghiệp vụ phía khách | F2 | `kb/50-doi-chieu/2026-09-23-feat-m8-multitenant-foundation.md` |
| Namespace ingress controller, CNI có thực thi NetworkPolicy — nay nặng hơn: cổng cầu 9091 và pod `vihat-miniapp` chỉ an toàn khi netpol được thực thi | vận hành | `netpol.yaml` | `deploy/README.md` §11.0 |
| Thu chi: biểu mẫu thật · KPI cân đối · chốt kỳ · công khai ngân sách | khách | nhập Excel, số nộp lên | `service-finance/thu-chi-ngan-sach-82` |
| Phiên không xã (app chung trước khi xác nhận) cần bảng "vết chưa thuộc xã" (#25) — 30/09 grep migration chưa thấy | người dùng | luồng QR → xác nhận | ADR 0045:247-250 · ADR 0047 |
| Tên tham số URL: mã dùng `d` + `src` (`citizen-app/src/lib/launch-params.ts`), ADR 0047 CÒN MỞ #5 chưa ghi chốt; chưa QR nào in | người dùng | mẫu QR | ADR 0047 §CÒN MỞ |
| 12 câu thiết kế khu vận hành | chủ dự án | `platform-admin/` | ADR 0048 §Thiết kế |
| `GET /api/v1/communes?host=` — `?host=` chưa có dòng trong `ubiquitous-language.md` (kiểm 30/09) | người dùng | đổi còn rẻ | `service-identity/internal/http/routes_cong_dan.go` |
| Bộ trạng thái riêng của văn bản đến (C2) | khách | tuyến đổi trạng thái | `service-documents/van-ban-den-tuyen-con-thieu` |
| Ngày làm việc hay ngày lịch cho hạn KN Đ.28 / TC Đ.29 — hỏi pháp chế | khách | SLA đơn thư | `service-documents/so-don-thu-cong-dan` |

Và **mười hai xung đột** sổ đơn thư (C2–C13), **tám** của danh bạ (U1–U8) — ghi nguyên hai phía ở
`service-documents/so-don-thu-cong-dan` và `service-identity/danh-ba-can-bo-con-thieu`.

**Đã bỏ khỏi bảng này ngày 30/09 vì đã giải:** UNKNOWN #2 của ADR 0045 (`vihat-miniapp` `4114f00`
gọi `graph.zalo.me /v2.0/me`; `ErrMaTaiKhoanChuaDo` không còn — chỉ còn đo trên máy thật, dòng ở
trên) · `/api/v1/requests` của `vihat-miniapp` mất phiên khi bật cầu (README `vihat-miniapp` nợ #15
ghi đã sửa 27/09) · App ID app chung / token từng app (gộp vào dòng chuỗi Mini App).

---

## 5. Cạm bẫy đã gặp — đọc để khỏi mất thời gian lại

Một nửa bảng này có chung một hình dạng: **thứ trông như biện pháp mà không phải biện pháp** — một
phép kiểm xanh vì lý do sai. Gặp cái tiếp theo cùng dạng thì hỏi cả lớp ấy còn ở đâu.

### Lượt 30/09 — gỡ đổi tên, nối chuỗi Mini App (mới, đã gặp thật)

| Triệu chứng | Sự thật |
|---|---|
| `git reset --hard` bị `data_safety_guard` chặn | Đúng ý rào. Dùng `git stash` — cây cũng sạch, và bỏ vĩnh viễn thì `git stash drop` sau |
| Revert một commit đổi tên → xung đột ở tệp mà một commit SAU đã sửa dưới tên mới | Lấy phía cũ (`checkout --theirs`) rồi **áp lại tay** bản sửa vào tên cũ; tệp do commit đổi tên TẠO RA (không phải `git mv`) thì `git rm`. Xác minh bằng đột biến: đưa lỗi trở lại phải đỏ đúng số ca cũ |
| Sau revert đổi tên message proto, Go build theo mã sinh CŨ | `core/gen` bị gitignore — revert không đụng tới. Chạy `mingw32-make proto` trước `build` |
| `lint` đỏ ở `buf breaking` sau revert | Nó so cây làm việc với `HEAD`: đổi tên message là "breaking" trên nguồn, wire không đổi. Trên cây đã commit sạch thì xanh (30/09 exit 0) |
| `tien-do.md` xung đột khi revert | Tệp SINH — lấy một phía bất kỳ rồi `make kb`, đừng gộp tay |
| Edit một mục sổ bị `progress_guard` chặn vì mục **khác** | Rào kiểm CẢ tệp: một mục cũ có `"menu": null` (không phải slug) chặn mọi lần ghi vào tệp ấy. Bỏ khoá tuỳ chọn hỏng, đừng lách bằng python |
| Mở cổng cầu 9091 trên identity mà Mini App vẫn không đăng nhập được | `vihat-miniapp` **cùng namespace**: `deny-all` chặn chính nó cả vào (8080 từ ingress) lẫn ra (identity:9091, Zalo:443). Nó không mang nhãn `vigov.vn/surface: rest`. Pod xanh, không ai báo — netpol quy tắc 9 |
| Sổ tiến độ ghi "UNKNOWN #2 → 503" suốt một ngày sau khi kho bên kia đã sửa | Kho anh em thay đổi không làm sổ bên này đổi. Trước khi tin một câu "bị chặn bởi `vihat-miniapp`", `git -C ../vihat-miniapp log` |
| Builder sửa tệp bằng python làm đổi xuống dòng thành CRLF | Soát `git diff --stat` — số dòng phình bất thường là dấu hiệu; builder 30/09 đã tự đổi về LF |
| Lệnh Bash nối `; head` / `sed -n` / `awk` bị từ chối | Lớp cấp quyền của phiên, không phải hook. Đọc bằng Read/Grep, sửa JSON bằng python |

### Rào chắn và công cụ sinh — giữ từ 28/09, không kiểm lại từng dòng

| Triệu chứng | Sự thật |
|---|---|
| Builder bị `require_sync_guard` chặn giữa card, sau khi `require-watcher` chạy song song | Mở mục sổ tiếp nhận ngay khi ghi chú về, trước khi builder chạm tệp. Builder không tự mở |
| `make kb` in `ĐÃ CÓ MÀN HÌNH → done/` cho một tuyến web chưa hề gọi | `tools/apidoc` khớp đường dẫn, mù phương thức. Xem `method` trong `tasks/web/done/<id>.json`. `_chung/apidoc-khop-man-hinh-mu-phuong-thuc` (còn mở) |
| Thêm trường thân request, Go xanh, web-admin vỡ lúc `gen:api` | `*T` hoặc thiếu `omitempty` → apidoc khai bắt buộc. Trường mới luôn `omitempty`. 30/09 thấy cùng họ: `GET /my-citizen-reports` khai `status` bắt buộc dù máy chủ coi là tuỳ chọn |
| `go test` xanh, `make check` đỏ ở `lint` vì `_test.go` mới | `gofmt -l <module>` trước khi commit |
| JOIN làm bảng phái sinh trong `core/store.QueryPage` | `WHERE tenant_id` bọc ngoài chỉ buộc một phía. `service-petitions/internal/store/de_nghi_lui_han_cho_duyet.go` |
| `data_safety_guard` chặn tệp `.md` | Quét cả văn xuôi. `_chung/data-safety-guard-chan-cau-trich-trong-md` — đừng đổi chữ để lách |
| `workflow_guard` nêu tên phiên ở mọi lần dừng | `_chung/workflow-guard-dem-ca-cua-so-phien` |
| codegraph trả ký hiệu thật nhưng không phải của kho này | **Luôn truyền `projectPath`**. Scout không có `codegraph_status` (kiểm lại 30/09) — kiểm chỉ mục bằng `codegraph_explore` trả Go + TS, không Java |
| Tầng always_load sát trần 28000 | Đừng tự nâng trần — chỉ người dùng nâng. `_chung/tang-luon-nap-da-day` |
| `make kb \| grep …` "xanh" mà hợp đồng không đổi | Ống dẫn che mã thoát. `make kb > log; echo $?` |
| Commit 1145971 sửa chú thích trong migration 0001 đã áp | `core/migrate` băm cả tệp. `_chung/migration-0001-da-ap-bi-sua-chu-thich` (còn mở) |
| Thêm tuyến Go, `make kb`, commit → `make check` đỏ ở `web` | `make kb` không sinh `schema.gen.ts`. `npm run gen:api` trong `web-admin`, cùng commit |
| Scout báo "không có X", đề xuất dựng bề mặt mới | Kho đã có `platform-admin/` từ trước. Liệt kê thư mục gốc, đọc README trước khi tin "không có" |
| Đặt `APP_ID` để chọn app đích cho `zmp deploy` | zmp-cli đọc claim `appId` trong `ZMP_TOKEN` (`citizen-app/scripts/dich-den.mjs`). Một token một app |
| Kho anh em có việc dở chưa commit | `git -C ../vihat-miniapp status` trước khi giao việc sang đó |
| Hứa "kiểm theo quyền" của người thứ ba | identity không có RPC trả lời "mã X có khoá K không" (`ResolveAssignableStaff` cố ý không lọc). Mở proto trước |
| Bộ lọc `?permission=` chung trên tuyến AnyAuthenticated | Mọi tài khoản liệt kê được người giữ `admin.*`. Danh sách trắng một khoá |
| `citizen_commitment_guard` chặn `const quaHan = …` ở web | Dương tính giả; viết lại inline, không tắt rào |
| Builder sửa bằng script python | PreToolUse hook không chạy trên các lần sửa ấy. Soát tay diff trước khi commit |

### Máy này (Windows, bộ nhớ hạn chế)

| Triệu chứng | Sự thật |
|---|---|
| `make check` một lượt bị Claude Code dừng vì hết RAM, hoặc `cmd/link` đổ | Chạy từng đích (`brain hooks quyen … build standalone`) và `go test -p 1` từng module |
| vitest `Fatal process out of memory: Zone` | `--maxWorkers=1` |
| `UnicodeEncodeError: 'charmap'` ở tool python | `PYTHONIOENCODING=utf-8` |
| Go đổ vì ổ C đầy / đường dẫn tạm lệch | `GOCACHE='D:\gocache-vigov' GOTMPDIR='D:\gotmp-vigov'` — dấu gạch NGƯỢC |
| Ca `_pg_test` "xanh" | Là SKIP (`VIGOV_TEST_DSN` trống) |
| `git checkout --` / `git restore` để hoàn tác đột biến | Xoá việc chưa commit của phiên khác. Đột biến bằng Edit rồi Edit trả lại, hoặc Go `-overlay` |

### Máy build (CentOS 7) và triển khai — giữ từ 24/09, chưa kiểm lại

| Triệu chứng | Sự thật |
|---|---|
| Jenkins `SyntaxError: Non-ASCII character` | `python` là 2.7 trên máy build; `PYTHON ?= python` |
| Ca kiểm hook xanh ở máy trạm, đỏ trên CI | Windows không phân biệt hoa thường |
| `<tên>@tmp/` lọt vào ngữ cảnh docker build | `.dockerignore` có `*@tmp` |
| Rancher/RKE2: netpol, CNI flannel, Import YAML không chạy kustomize | `deploy/README.md` §11. Cụm dựng tay — mọi thay đổi `deploy/base` phải chép tay lên |
| Mọi lời gọi API qua web-admin 502, mọi pod xanh | Thiếu luật netpol web-admin ↔ dịch vụ (ADR 0043) |
| IP trong nhật ký kiểm toán là IP pod web-admin | `TRUSTED_PROXY_CIDRS` chưa đặt (ADR 0043) |

### Vẫn đúng từ bản trước

| Vấn đề | Cách xử |
|---|---|
| Kết quả grep âm tính không phải bằng chứng vắng mặt | Mở tệp, không kết luận từ grep |
| Bản vá cho lỗi X dễ là một thể hiện mới của X | Sau khi vá một hook, chạy nó trên kho thật |
| `SET search_path` là trạng thái SESSION | Suite tích hợp phải `SetMaxOpenConns(1)` |
| Hai cột cùng kiểu cạnh nhau, đọc theo vị trí trong `Scan` | Lỗi im lặng đối xứng (30/09 gặp lại ở finance `la_mac_dinh`/`dang_dung`; cùng lỗi ở petitions loại/mức ưu tiên nhiệm vụ **chưa sửa**) |
| `fmt` không gọi `String()` cho `%d %c %U %b %o` | `core/secret` cài `fmt.Formatter` |

---

## 6. Cổng kiểm

```
PYTHONIOENCODING=utf-8 GOCACHE='D:\gocache-vigov' GOTMPDIR='D:\gotmp-vigov' mingw32-make <đích>
```

**30/09, chạy thật (từng đích, không một lượt `check`):**
- Trên cây sau revert: `brain hooks quyen vet-actor khoaduynhat envmap buildfiles security build standalone` PASS; `go test -p 1` cả 9 module xanh; web-admin + citizen-app tsc/test/check:api, platform-admin tsc xanh.
- Sau ba thẻ Mini App: cùng các đích tĩnh + `build standalone` PASS; `go test` service-petitions, service-identity, core, tools xanh; citizen-app vitest xanh; web-admin tsc + check:api xanh.
- `lint` trên cây sạch tại `417e9564`: exit 0 (`golangci-lint` bị bỏ qua — không có trên máy).

**CHƯA KIỂM:** chưa lần `zmp deploy` thật nào; app chưa chạy trên máy thật / webview Zalo; cầu phiên
chưa từng gọi thật giữa hai kho; manifest `deploy/base` mới chỉ qua `kubectl kustomize`, chưa lên cụm;
`vihat-miniapp` không build/test lại ở lượt này; `web` (vitest web-admin) không chạy lại sau ba thẻ.

**Thứ cổng KHÔNG phủ:**

| Không phủ | Hệ quả |
|---|---|
| **PostgreSQL thật** | Mọi `*_pg_test.go` SKIP. Migration và truy vấn chỉ qua ca kiểm văn bản SQL |
| **`golangci-lint`** | Không có trên máy; dòng `Error 2 (ignored)` là nó |
| **Jenkins / k8s** | Cụm dựng tay trên Rancher, không từ `deploy/base`; không phép kiểm nào so hai bên |
| **Trình duyệt** | vitest chạy node không DOM |
| **Zalo** | Không ca kiểm nào gọi Zalo thật; mã lỗi SDK mới là mã đọc từ `zmp-sdk`, chưa đo |
| **Hook có NHÌN THẤY gì không** | Phép thử duy nhất đáng tin là đột biến |

---

## 7. Việc treo

**Không nằm ở đây.** Mỗi việc treo ở module của nó với `trang_thai: "treo"` — `kb/90-ephemeral/tien-do.md`.
Phân biệt: **`treo`** là *không ai chặn, ta chọn chưa làm*; **bị chặn** là chờ một câu ở §3.
