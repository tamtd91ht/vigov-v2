---
id: ban-giao-phien
tier: T5
source: CURATED
owner: architecture
derived_from_commit: b736232
expires: 2026-12-26
owns_facts:
  - "quyết định đã chốt với người dùng/khách, cạm bẫy đã gặp, và câu đang chờ người dùng — tại 28/09/2026"
---

# Bàn giao phiên — cập nhật 2026-09-28

**Đọc tệp này SAU `kb/INDEX.yaml` và tầng `always_load`, không thay thế chúng.** Nó chỉ trả lời:
*đã quyết gì, đang chờ ai, và cạm bẫy nào đã tốn thời gian của người trước.*

Viết bằng `/handover`. **MỘT tệp, ghi đè trọn vẹn mỗi lần** — tên không mang ngày, ngày nằm bên
trong. Hết hạn **2026-12-26**; sau ngày đó tin `git log`, đừng tin tệp này.

Viết lại 28/09 sau hai lượt `/develop-feature` (Nhiệm vụ, Danh bạ cán bộ); bản trước ở `836264f`.
Các dòng giữ lại từ bản 27/09 **không được kiểm lại từng dòng** — chúng đã đối chiếu đĩa 27/09. Chỉ
những dòng hai lượt này chạm tới được viết lại từ đĩa. Nợ sổ tiến độ ghi ở bản trước **đã trả**:
lượt Mini App do phiên song song ghi (`e53d995`), hai lượt 28/09 ghi ở `b736232`.

---

## 1. Đã làm — chỉ những gì `git log` không nói

**Không có danh sách commit ở đây.** `git log --oneline d4a9c18..HEAD` trả lời chính xác hơn.

### Quyết định đã chốt — và nơi ghi

Quyết định đã thành ADR thì chỉ trỏ, không chép. Quyết định chưa thành ADR thì nơi ghi là mục sổ
tiến độ tương ứng (khoá `tiep_theo`/`bang_chung`).

| Quyết định | Ngày | Ai quyết | Ghi ở |
|---|---|---|---|
| Tiền tố **`vigov`** cho tên ảnh và module | — | người dùng | `service-identity/Jenkinsfile` |
| Jenkins dùng **docker CLI trên agent**, không Kaniko | — | người dùng | Jenkinsfile từng dịch vụ |
| Hạ tầng dừng ở **Dockerfile + Jenkinsfile**; cụm k8s do devops | — | người dùng | `deploy/README.md` §9, §11 |
| Trần `always_load` **27000 token** — đừng nâng (xem §5) | 21/09 | người dùng | `kb/INDEX.yaml` `budget` |
| **Miễn xã** cho `ResolveCitizenSession`; `ListTenants`/`ResolveTenantSuccession` vẫn ngoài danh sách | 21/09 | người dùng | chú thích `core/grpcx/grpcx.go` quanh `MethodResolveCitizenSession` (chưa có ADR — §3) |
| **Ingress SINH từ `openapi.json`**; web-admin là cổng `/api/v1/*`; k8s chỉ cấp Secret + ConfigMap; `/api/v1` lạ nhận **404 JSON** | 21/09 · 25/09 | người dùng | ADR 0043 · `tools/ingress/` |
| Quy hoạch **4 tên miền** và **người quản trị đầu tiên** của xã (tài khoản `admin` gieo ở lần đăng nhập đầu tại tên miền xã, mật khẩu từ biến `IDENTITY_ADMIN_SEED_PASSWORD`) | 26/09 | chủ dự án | ADR 0046 · `service-identity/xa-moi-khong-co-vai-tro-va-quyen` |
| Mini App **hai chế độ**: app chính mở xã bằng QR, app riêng gắn xã theo App ID, một bản build | 25/09 | người dùng | ADR 0044 |
| **Cầu phiên công dân** Mini App → ViGov (`OpenCitizenSession`, `ResolveMiniApp`) | 26/09 | người dùng | ADR 0045 · `citizen-app/cau-phien-cong-dan-vigov` |
| **Hai giai đoạn với một xã** (lời chủ dự án): GĐ1 chưa có công văn → app chung của ViHAT, QR gắn **tên miền xã** trỏ vào bản test; GĐ2 có công văn → app riêng, App ID riêng, dân tìm trong kho. App chung **cũng phát hành**. "Demo" là cách gọi giai đoạn, **về kỹ thuật là app thật** | 27/09 | chủ dự án | ADR 0047 §Trả lời 27/09 |
| Build: có `domain` → App ID riêng, không có → App ID chung; **một biến thể** (bỏ `goc`/`day-du`, danh mục mẫu, bảng chẩn đoán); tệp domain → App ID chỉ chọn nơi đẩy | 27/09 | chủ dự án | ADR 0047 · `citizen-app/scripts/ung-dung-theo-ten-mien.mjs` |
| Tham số QR mang **tên miền xã**, bỏ hẳn `t=<ULID>`; máy chủ tra ra xã, chỉ lưu ULID; xã sáp nhập = trỏ tên miền cũ sang xã kế thừa, có vết. App chung: dân xác nhận một lần; app riêng: không xác nhận | 27/09 | chủ dự án | ADR 0047 |
| Rủi ro **chấp nhận**: người vận hành trỏ tên miền đúng lúc dân đang xác nhận → vào xã chưa thấy tên | 27/09 | chủ dự án | ADR 0047 §Trả lời mục 4 |
| **Cho phát hành app riêng trước khi đo UNKNOWN #1** (thay điều kiện dừng #4 của 0045); `ZMP_TOKEN` là credential Jenkins, **mỗi App ID một token** | 27/09 | chủ dự án | ADR 0047 D8 + §Trả lời mục 3 |
| Tuyến tra tên miền → tên xã thuộc **service-identity** | 27/09 | chủ dự án | ADR 0047 D6 |
| Khu vận hành ViHAT (tạo xã, tên miền, hồ sơ hiển thị, dòng `mini_app`, sinh QR) dựng trong **`platform-admin/`**, backend `service-platform`, tài khoản ViHAT là **miền riêng**. 12 câu thiết kế **chưa chốt** | 27/09 | chủ dự án | ADR 0048 — đọc §Sửa của chủ dự án; thân ADR ghi "trong web-admin" đã bị thay |
| Commit hộ việc dở 5 ngày của `vihat-miniapp` (yêu cầu tư vấn/gọi lại/ZNS) nguyên trạng | 27/09 | chủ dự án | `vihat-miniapp` `44c42d7` |
| Nhiệm vụ: thêm mũi tên **trả lại để làm tiếp** `cho-duyet → dang-thuc-hien` (task.approve + lý do), không thêm trạng thái | 28/09 | chủ dự án (chọn đề xuất domain-expert) | `service-petitions/tra-lai-nhiem-vu-lam-tiep` |
| Lãnh đạo giao việc: máy chủ CHỈ kiểm **cán bộ đang hoạt động của xã**, **không** kiểm task.extend; ô chọn lọc `staff-directory?permission=task.extend` (danh sách trắng một khoá) | 28/09 | chủ dự án | `service-petitions/lanh-dao-giao-viec-khong-kiem-khi-tao` · `service-identity/danh-ba-hep-loc-quyen-task-extend` |
| Danh bạ: **đổi số di động** hoặc **khoá** người đang công khai → tự gỡ khỏi Mini App, xoá dấu đồng ý; mở khoá không hiện lại. Kênh công khai trả số di động **không che** cho người đã đồng ý (thay cách đọc #11 cho kênh này); Excel vẫn che | 28/09 | chủ dự án | `service-identity/rut-cong-khai-khi-doi-so-hoac-khoa` · `che-so-di-dong-can-bo` |
| Sổ **đơn thư công dân** (`don_thu`) thuộc **`documents`** | 24/09 | người dùng | ADR 0039 |
| Ô cấp quyền `vai_tro_quyen` là **cấu hình**: gỡ quyền = xoá cứng ô ấy kèm vết trước/sau; CHỈ bảng này | 24/09 | người dùng | ADR 0040 |
| **#12 là quyết định của KHÁCH (22/09)**, không phải đề xuất nhà cung cấp | 24/09 | người dùng xác nhận | `service-identity/che-so-di-dong-can-bo` |
| Công khai số lên Mini App, **bản gọn** (bật từng người + tick "đã hỏi ý", lưu thời điểm + mã người ghi; khoá `content.update`) | 24/09 | người dùng | `service-identity/danh-ba-can-bo-con-thieu` |
| Xoá dòng danh bạ **có tài khoản** → từ chối; tìm cán bộ theo tên/SĐT đi trong **thân POST** | 24/09 | người dùng | cùng mục trên |
| Nút `+ Thêm cán bộ` **giữ ở Cấu hình** | 24/09 | người dùng | `web-admin/danh-ba-man-rieng` |
| Tuyến **danh bạ hẹp** `staff-directory` (AnyAuthenticated, không SĐT/email) cho ô chọn cán bộ ở 4 menu | 24/09 | người dùng | `service-identity/tuyen-danh-ba-can-bo-hep` · `ubiquitous-language.md` |
| Phân quyền: **cấm** lưu cột của vai trò mình đang giữ (#14); chặn lượt lưu làm xã mất người giữ `admin.user` **hoặc** `admin.role` | 24/09 | người dùng | `service-identity/cau-hinh-bon-chuc-nang-24-09` |
| Sơ đồ tổ chức: khoá `admin.org`, **chưa có Xoá**, nhận cả khối ngoài UBND | 24/09 | người dùng | cùng mục trên |
| `Loại đơn vị dân cư` và `Khối nhiệm vụ` là **danh mục đầy đủ**; Khối nhiệm vụ coi là khái niệm RIÊNG (vẫn phải hỏi đầu mối nghiệp vụ phía khách — §3) | 24/09 | người dùng | cùng mục trên |
| Mục menu `/cau-hinh` hiện khi cầm **bất kỳ** khoá canh một tab | 26/09 | chủ dự án | `web-admin/cau-hinh-bon-chuc-nang-web` |
| Mục menu Văn bản & Đơn thư canh bằng `document.read` | 24/09 | người dùng | commit b1da756 |
| Hợp đồng đổi làm web đỏ → **sửa hai phía nhỏ nhất**: trường mới phải tuỳ chọn; web chỉ sửa fixture | 24/09 | người dùng | commit 24dade9, ff4aaf2 |
| **Nhiệm vụ** §7.2: mỗi văn bản = textarea Trích yếu bắt buộc + hai ô tuỳ chọn; Sổ theo dõi §4.3 **dựng backend trước**. Nhãn trạng thái nhiệm vụ: **một nguồn**, đọc từ máy chủ | 24/09 | người dùng | `web-admin/man-nhiem-vu` · `service-petitions/nhan-trang-thai-nhiem-vu-theo-xa` |
| **Nhiệm vụ — lượt 27/09:** nhật ký và hàng chờ lùi hạn canh bằng **`task.read`**; URL hàng chờ **`task-extensions`**; mặc định đề nghị đang chờ của cả xã + lọc **`approver=me`** (mã từ phiên, không từ tham số), web mở ở lọc "chờ tôi duyệt" | 27/09 | người dùng | `ubiquitous-language.md` · `service-petitions/hop-dong-nhiem-vu-thieu-ba-mon` |
| **Sổ đơn thư — 15 câu C3–C19** (trạng thái theo TT 05/2021, 4 loại đơn, che danh tính người tố cáo, luật người đang giữ chỉ cho đơn thư, nhiệm vụ sinh từ đơn hạn 17:00 và cấm với tố cáo…). Chưa dựng — dựng đúng theo đó, không hỏi lại | 24/09 | người dùng (chọn đề xuất) | `service-documents/so-don-thu-cong-dan` |
| Đơn thư từ **Mini App**: hoãn | 24/09 | người dùng | `service-documents/don-thu-tu-mini-app` |
| **Báo công dân theo bảng** (6 chuyển trạng thái; không bao giờ gửi tên cán bộ/nội dung/ảnh) | 24/09 | người dùng | ADR 0041 |
| Phản ánh: cờ ảnh nghiệm thu **giữ #7 mặc định BẬT**; **gia hạn** có; **tự đóng** phiếu chờ dân sau N ngày; 1–2 sao vào **hàng lãnh đạo xem** — 27/09 chốt lại: KHÔNG tự mở lại, cán bộ không nhập hộ; phải viết ADR thay phần `nguong_sao_mo_lai` của ADR 0008 TRƯỚC khi dựng | 24/09 · 27/09 | người dùng | `service-petitions/vong-doi-phieu-phan-anh` · `service-petitions/phan-anh-tuyen-cong-dan-con-thieu` |
| Hai nhánh phiếu **`…/rejection` · `…/referral`**, khoá `feedback.classify`; máy chủ kiểm người được giao qua identity (`ResolveAssignableStaff`) | 25/09 | người dùng | `ubiquitous-language.md` · `service-petitions/duong-xu-ly-phan-anh-phia-can-bo` |
| Biên bản họp: **dự thảo → đã ký** (ký bằng `task.approve`, sai thì **biên bản bổ sung**); `POST /tasks` không nhận nguồn `ket-luan-hop` | 25/09 | người dùng | `service-petitions/bien-ban-hop-tang-du-lieu` |
| Thu chi ngân sách: **số lưu đồng, web quy đổi**; dựng đợt thu chi theo đặc tả; URL `budget-entries`; KPI cân đối **hỏi khách** | 25/09 | người dùng | `service-finance/thu-chi-ngan-sach-82` |

**Cần biết về 34 câu trong `open-questions.json`:** cả 34 đều DECIDED (đã kiểm 27/09), nhưng nhiều
câu (#21, #27 và mười ba câu khác) là **đề xuất của nhà cung cấp** ghi ở ADR 0035, không phải trả lời
của khách. Đọc bảng "cái gì đỏ nếu một mục bị phủ quyết" ở cuối ADR 0035 trước khi dựa vào một câu như thế.

---

## 2. Việc kế tiếp

**Không nằm ở đây.** `kb/90-ephemeral/tien-do.md` (theo module) và `python tools/tien_do.py --menu
"<menu>"` (theo menu). Đọc dòng "SỬA dd/09" ở đầu `tiep_theo` trước phần còn lại của một mục.

---

## 3. Đang bị chặn — và chặn bởi ai

Bảng *"Nợ khách chốt"* ở đầu `tien-do.md` sinh từ `no_confirm`; nó RỖNG vì mọi câu trong
`open-questions.json` đã DECIDED. Những câu dưới đây **không có trong tệp ấy** — chúng chờ NGƯỜI DÙNG
(hoặc người dùng chuyển cho khách):

| Câu | Chặn gì | Chi tiết ở |
|---|---|---|
| **Vòng đời nhiệm vụ lệch `vigov-require`** (52ec9b5 cho nhảy bước, bỏ chờ duyệt, mở lại việc đã xong) · **sửa mã NV** (7764c8a) · **sửa hạn không qua lùi hạn** (93cff7f) | mọi thay đổi bảng chuyển trạng thái / ô sửa mã, hạn ở §5.4 | `service-petitions/doi-chieu-26-09-nhiem-vu-truoc-neo` · `kb/50-doi-chieu/2026-09-26-feat-m8-multitenant-foundation-nhiem-vu.md` |
| **Bảy xung đột vòng đời Nhiệm vụ** chờ khách, mỗi câu đã có đề xuất domain-expert 28/09 (nhảy bước = một hành vi gộp; giữ bước duyệt; KHÔNG mở lại — tạo việc mới liên kết; không sửa mã NV; "đính chính hạn" là hành vi riêng; hết ngày 23:59 và thống nhất với đơn thư C9 17:00; chủ trì ≠ thực hiện) | mọi thay đổi bảng chuyển, ô sửa mã/hạn | `service-petitions/doi-chieu-26-09-nhiem-vu-truoc-neo` · `doi-chieu-24-09-nhiem-vu-phan-anh` |
| Nhiệm vụ nguồn `phan-anh` nhận `source_id` không kiểm · `task.approve` vừa ký biên bản vừa duyệt nhiệm vụ | liên kết phiếu↔nhiệm vụ; tách quyền | `service-petitions/nguon-phan-anh-source-id-khong-kiem` · `task-approve-hai-viec` |
| Danh bạ #12 phần còn lại: đổi **số cơ quan / co_zalo** có gỡ công khai không (F1) · dọn dấu đồng ý của người khoá trước bản vá (F2, migration) · chặn công khai người đang khoá (F3) · tự ghi đồng ý cho mình | tuyến công khai Mini App | `service-identity/rut-cong-khai-khi-doi-so-hoac-khoa` |
| Bốn câu đối chiếu 24/09 chạm Nhiệm vụ/Phản ánh: cờ ảnh nghiệm thu mặc định (b9a9718 TẮT ↔ #7 BẬT), luật người đang giữ cho NHIỆM VỤ (chặn cả phần ghi tay nhật ký §5.9), chủ trì ≡ thực hiện, hạn 17:00 ↔ 23:59 | các tuyến tương ứng | `service-petitions/doi-chieu-24-09-nhiem-vu-phan-anh` |
| Cột đầu Kanban mặc định **"Mới giao"** hay **"Chưa thực hiện"** (spec 02 §6 có hai tên) | nay hiện "Mới giao" | `web-admin/cau-hinh-bon-chuc-nang-web` |
| Người quản trị đầu tiên: **ADR 0046 đã có**, còn nợ gỡ `IDENTITY_ADMIN_SEED_PASSWORD` khỏi Secret khi mọi xã đã đổi mật khẩu | một bí mật mở `admin` ở mọi xã chưa đổi | `service-identity/xa-moi-khong-co-vai-tro-va-quyen` |
| Miễn xã cho `ResolveCitizenSession` có cần **ADR riêng** không | không chặn mã | chú thích `core/grpcx/grpcx.go` |
| **Khối nhiệm vụ** có phải "khối đơn vị" của danh bạ không (hỏi đầu mối nghiệp vụ phía khách) | nếu CÙNG thì phải xem lại F2 | `kb/50-doi-chieu/2026-09-23-feat-m8-multitenant-foundation.md` |
| Phạm vi đồng ý #12: đổi số của người **đang** công khai có phải hỏi lại; khoá người đang công khai có rút công khai; cán bộ tự ghi đồng ý cho mình | số mới lên kênh công khai dưới đồng ý cũ | `service-identity/danh-ba-can-bo-con-thieu` |
| Hai câu ở `deploy/README.md` §11.0 (namespace ingress controller, CNI có thực thi NetworkPolicy) | phiên CI/deploy sửa `netpol.yaml`, `deploy/Jenkinsfile` | `deploy/README.md` §11.0 |
| **Thu chi: biểu mẫu thật của xã** · KPI cân đối · chốt kỳ/quyết toán/người duyệt · công khai ngân sách | nhập Excel, số liệu nộp lên cấp trên | `service-finance/thu-chi-ngan-sach-82` |
| **Cầu phiên — UNKNOWN #2 của ADR 0045**: chưa endpoint Zalo nào ĐÃ ĐO trả mã tài khoản. `vihat-miniapp` đã dựng phía gọi (`e274d21`) nhưng chỗ nối `MaTaiKhoanZalo` trả `ErrMaTaiKhoanChuaDo` → bật cầu thì mọi lần đăng nhập 503. **Không đoán tên endpoint** — đo bằng `cmd/thu-zalo` (người có quyền console Zalo). Kèm ca pg của identity | công dân thật gọi mọi tuyến CitizenOnly | `citizen-app/cau-phien-cong-dan-vigov` · README `vihat-miniapp` |
| **Phiên không xã** (app chung trước khi xác nhận) cần bảng "vết chưa thuộc xã" của câu mở #25 — chưa có migration | luồng quét QR → xác nhận của app chung | ADR 0045:247-250 · ADR 0047 §Trả lời mục 5 |
| **Giá trị thật**: App ID app chung (`APP_ID_APP_CHUNG = null`), App ID từng xã, một `ZMP_TOKEN` mỗi App ID — chủ dự án cung cấp | mọi lần đẩy Zalo thật, TASK-03 | `citizen-app/scripts/ung-dung-theo-ten-mien.mjs` |
| **Tên tham số URL** mang tên miền (và `v`) | TASK-04b, mẫu QR | ADR 0047 CÒN MỞ #5 |
| **12 câu thiết kế khu vận hành** — nặng nhất: tài khoản ViHAT ở đâu, đăng nhập ra sao; nhận host vận hành ở rìa (rìa đang từ chối mọi host dành riêng, `59c72be`); ai thắng khi xã và ViHAT cùng sửa `ho_so_hien_thi_xa` | dựng `platform-admin/` (luật 5 điều kiện dừng #3) | ADR 0048 §Thiết kế |
| URL **`GET /api/v1/communes?host=`** — `?host=` do agent chọn, chưa có dòng trong `ubiquitous-language.md` | đổi còn rẻ, chưa client nào gọi | `service-identity/internal/http/routes_cong_dan.go` |
| Bật cầu thì tuyến thương mại `/api/v1/requests` của `vihat-miniapp` **mất phiên** — quyết định sản phẩm | bật cầu trên môi trường thật | README `vihat-miniapp` NỢ #15 |
| Phản ánh phía công dân: ảnh hiện trường (lần đầu gửi ảnh công dân vào kho tệp — luật 3 điều kiện dừng #2), phiếu công khai | các màn Mini App tương ứng | `service-petitions/phan-anh-tuyen-cong-dan-con-thieu` |
| **Bộ trạng thái riêng của VĂN BẢN ĐẾN** (C2) | tuyến đổi trạng thái văn bản đến | `service-documents/van-ban-den-tuyen-con-thieu` |
| Ngày làm việc hay ngày lịch cho hạn KN Đ.28 / TC Đ.29 — **hỏi pháp chế**; cần ADR vì ADR 0007 tính GIỜ | gieo số SLA đơn thư | `service-documents/so-don-thu-cong-dan` |

Và **mười hai xung đột yêu cầu** của sổ đơn thư (C2–C13) cùng **tám** của danh bạ (U1–U8) — ghi
nguyên hai phía ở `service-documents/so-don-thu-cong-dan` và `service-identity/danh-ba-can-bo-con-thieu`.

**Đã bỏ khỏi bảng này ngày 27/09 vì đã giải:** xã mới lấy quản trị đầu tiên thế nào (ADR 0046) ·
`/api/v1` chưa định tuyến trả HTML hay JSON (404 JSON, ADR 0043 — `deploy/base/mang/ingress.yaml`) ·
mục menu `/cau-hinh` chỉ canh `admin.lookup` (mở theo mọi khoá tab, 26/09).

---

## 4. Phiên song song

Lúc viết (28/09) ListAgents thấy hai phiên cùng kho, cả hai **idle**: `vigov-v2-45` — tự khai giữ
`citizen-app/src/App.tsx` (AppRieng), `citizen-app/src/cong-dan/man/{TrangXa,DanhBaXa,TinTucAppXa,
khung-xa,BieuTuong}.tsx`, khối `.xa-*` cuối `styles.css`, `XA_GIAO_DIEN`/`APP_RIENG` trong
`noi-dung.ts`, `citizen-app/scripts/*.mjs`, ADR 0047 §6; nhờ **giữ ổn định** `docDanhBa`
(`hop-dong-cong-khai.ts`) và `sauKhiTaiDanhBa`/`dichGoi` (`DanhBaCanBoScreen.tsx`). `vigov-v2-60` —
lượt Mini App tuyến công khai, không khai đường dẫn. Cây vigov-v2 sạch ngoài `bash.exe.stackdump`,
`img.png` (rác). Phiên sau tự kiểm lại, đừng tin dòng này.

---

## 5. Cạm bẫy đã gặp — đọc để khỏi mất thời gian lại

Một nửa bảng này có chung một hình dạng: **thứ trông như biện pháp mà không phải biện pháp** — một
phép kiểm xanh vì lý do sai. Gặp cái tiếp theo cùng dạng thì hỏi cả lớp ấy còn ở đâu.

### Rào chắn và công cụ sinh

| Triệu chứng | Sự thật |
|---|---|
| Builder bị `require_sync_guard` chặn **giữa card**, sau khi `require-watcher` chạy song song ghi một ghi chú đối chiếu mới | Rào chặn mọi lần ghi mã vào module có trong `anh_huong` cho tới khi sổ module ấy có mục trích TÊN TỆP ghi chú. Chạy `require-watcher` song song với builder thì **mở mục sổ tiếp nhận ngay khi ghi chú về**, trước khi builder chạm tệp. Builder không được tự mở (ngoài ranh giới ghi) — đúng là phải trả về |
| `make kb` in `ĐÃ CÓ MÀN HÌNH → done/` cho một tuyến web **chưa hề gọi** | `tools/apidoc` khớp ĐƯỜNG DẪN mà mù PHƯƠNG THỨC. Mở tệp `tasks/web/done/<id>.json` xem `method` trước khi commit. `_chung/apidoc-khop-man-hinh-mu-phuong-thuc` (còn mở) |
| Thêm một trường vào thân request, Go xanh, rồi **web-admin vỡ** lúc `gen:api` | `*T` hoặc trường không kèm `omitempty` → apidoc khai **bắt buộc**. Trường mới: luôn `omitempty`. Trường mới trong PHẢN HỒI làm đỏ fixture web — sửa fixture trong cùng commit hợp đồng |
| `go test` xanh, `make check` đỏ ở `lint` vì một tệp `_test.go` builder mới tạo | `go test` không chạy gofmt. Chạy `gofmt -l <module>` trước khi commit. 27/09 gặp hai lần (một tệp của lượt này, một tệp có sẵn từ 872ca37) |
| Truyền một câu JOIN làm **bảng phái sinh** vào `core/store.QueryPage` cho gọn | `WHERE tenant_id = $1` bọc ngoài chỉ buộc bảng nào lộ cột `tenant_id` ra ngoài — **một** phía. Phía kia chỉ được bảo vệ bằng mệnh đề ON; ca SQL giả kiểm được văn bản, không kiểm được dữ liệu. Chú thích ở `service-petitions/internal/store/de_nghi_lui_han_cho_duyet.go` |
| `data_safety_guard` chặn một tệp **.md** | Nó quét cả văn xuôi. Ngược lại có thể im trước câu xoá cứng thật. `_chung/data-safety-guard-chan-cau-trich-trong-md` — **đừng đổi chữ để lách** |
| `workflow_guard` nêu tên phiên ở mọi lần dừng | Nó đếm tệp theo cả cửa sổ phiên. `_chung/workflow-guard-dem-ca-cua-so-phien` |
| codegraph trả ký hiệu thật nhưng **không phải của kho này** | **Luôn truyền `projectPath`**. 27/09: scout báo không có `codegraph_status` trong bộ công cụ của nó — kiểm chỉ mục bằng việc `codegraph_explore` trả Go + TS, không Java |
| Tầng always_load sát trần 27000 | Mục kế tiếp đăng ký vào `kb/INDEX.yaml` dễ làm đỏ `check_brain` #5. **Đừng nâng trần**. `_chung/tang-luon-nap-da-day` |
| `make kb \| grep …` "xanh" mà hợp đồng KHÔNG đổi | Ống dẫn che mã thoát. Luôn `make kb > log; echo $?` |
| `tools/apidoc` không sinh được `enum` cho trường | `service-documents/apidoc-sinh-enum-cho-truong` (còn mở) |
| IDE báo hàng chục lỗi biên dịch ngay sau khi agent sửa | Ảnh chụp giữa chừng của language server. Tin `go vet`/`go test`/`tsc` |
| Commit 1145971 sửa chú thích trong hai migration **0001 đã áp** | `core/migrate` băm cả tệp → lệch checksum. `_chung/migration-0001-da-ap-bi-sua-chu-thich` (còn mở) |

### Lượt Mini App 27/09 — hai kho, Zalo, quyền phiên

| Triệu chứng | Sự thật |
|---|---|
| Hai scout báo "chưa có web quản trị cho ViHAT", chủ dự án chọn phương án dựa trên đó | Kho **đã có** `platform-admin/` (khung, cố ý tách theo ADR 0003). Chỉ lộ ra ở log `make check` (`typecheck platform-admin/`). Trước khi đề xuất một **bề mặt mới**: liệt kê thư mục gốc, đọc README từng app — đừng tin "không có" của scout |
| Đặt `APP_ID` để chọn app đích cho `zmp deploy` | zmp-cli 4.0.3 **không đọc** `APP_ID` khi deploy: đích là **claim `appId` trong `ZMP_TOKEN`** (đo, đầu tệp `citizen-app/scripts/dich-den.mjs`). Một token một app |
| Thêm tuyến Go, `make kb`, commit → `make check` đỏ ở `web` | `make kb` cập nhật `openapi.json` nhưng **không** sinh `web-admin/src/lib/api/schema.gen.ts`. Sau mọi thay đổi tuyến: `npm run gen:api` trong `web-admin`, cùng commit (`88c5bef` là bản vá cho đúng lỗi này) |
| IDE đỏ lòm các tệp `vihat-miniapp` (`could not import …`) | Kho ấy không nằm trong `go.work` của vigov-v2 — nhiễu gopls. Tin `go build/vet/test` chạy **trong** kho ấy |
| Kho anh em có việc dở chưa commit đúng các tệp agent sắp sửa | Luôn `git -C ../vihat-miniapp status` trước khi giao việc sang đó; 27/09 có 17 tệp dở 5 ngày |
| `git push` và một lần giao agent ghi sổ bị **lớp cấp quyền** từ chối | Không phải hook. Không làm vòng — người dùng tự chạy (`! git push origin main`) hoặc cấp quyền. Push sau đó người dùng cho phép và đã chạy |

### Lượt Nhiệm vụ + Danh bạ 28/09

| Triệu chứng | Sự thật |
|---|---|
| Quyết định "kiểm theo quyền" nghe rẻ, builder dừng giữa card | identity **không có RPC** nào trả lời "mã X có cầm khoá K không" — `ResolveAssignableStaff` cố ý không lọc (`identity.proto:1528`), `ResolveStaffPrincipal` chỉ tra người gọi. Trước khi hứa kiểm quyền của **người thứ ba**, mở proto |
| Bộ lọc chung `?permission=` trên tuyến AnyAuthenticated trông vô hại | Mọi tài khoản liệt kê được người giữ `admin.*`. Danh sách trắng một khoá, khoá khác trả **400** — trả rỗng cho khoá này, tên cho khoá kia cũng là lộ |
| Kiểm quyền theo `ma` bằng câu SQL của bộ kiểm quyền | Bộ kiểm khớp trên `nd.id`, danh bạ trên `nguoi_dung.ma` — dùng nguyên thì không tìm ra ai, mọi lãnh đạo bị từ chối. Fixture phải cho id ≠ mã |
| Agent báo "thay đổi không phải của tôi" trong `git status` | Là phiên song song đang làm dở. ListAgents + nhắn hỏi trước khi commit; stage **từng đường dẫn**. `make check` trên cây có việc dở của người khác là đo sai — kiểm riêng phần mình |
| `citizen_commitment_guard` chặn một `const quaHan = o.phanTre !== ""` ở web | Dương tính giả (giá trị suy ra, không lưu). Viết lại inline — không đổi ngữ nghĩa, không tắt rào |
| Builder sửa tệp bằng script Python thay vì Edit | PreToolUse hook **không chạy** trên các lần sửa ấy. Soát tay diff (số điện thoại, chuỗi bí mật) trước khi commit |

### Máy này (Windows, bộ nhớ hạn chế)

| Triệu chứng | Sự thật |
|---|---|
| `make check` **bị chính Claude Code dừng** ("system is running low on memory") | Không phải lỗi. Đừng tự chạy lại ngay; chạy khi không phiên nào khác đang biên dịch/vitest |
| vitest đổ `Fatal process out of memory: Zone` | Worker chết vì bộ nhớ, không phải ca đỏ. `--maxWorkers=2`; vẫn đổ thì `--maxWorkers=1` (27/09: 1 thì xanh) |
| `mingw32-make check` đỏ ở `build` với stack trace của `cmd/link` | Hết bộ nhớ khi dựng mọi module một lượt. Dựng **từng module** thì xanh |
| `make check` đổ ở `envmap` với `UnicodeEncodeError: 'charmap'` | `PYTHONIOENCODING=utf-8 mingw32-make check` |
| `go test -race` đổ ở link, `error code 1455` | Luôn `-p 1`; không quá 2 agent biên dịch Go cùng lúc; `GOCACHE=/d/gocache` vì ổ C đầy |
| Ca `_pg_test` "xanh" | Là **SKIP** (`VIGOV_TEST_DSN` trống). ~0,02 s là bỏ qua |
| `sed`, `awk` bị từ chối trong Bash | Chính sách quyền của phiên. Sửa JSON bằng python, đọc bằng Read/Grep |
| `git checkout --` / `git restore` để hoàn tác đột biến | Xoá việc chưa commit của phiên khác. Dùng Go `-overlay` hoặc chép tệp ra thư mục tạm (test-designer 27/09 làm vậy) |

### Máy build (CentOS 7) và triển khai — **chưa kiểm lại 27/09**, giữ từ bản 24/09

| Triệu chứng | Sự thật |
|---|---|
| Cổng kiểm trên Jenkins đổ với `SyntaxError: Non-ASCII character` | `python` là 2.7 trên máy build; `PYTHON ?= python` + Jenkinsfile tự chọn |
| Ca kiểm hook xanh ở máy trạm, đỏ trên CI | Hệ tệp Windows không phân biệt hoa thường |
| Thư mục `<tên>@tmp/` lọt vào ngữ cảnh docker build | `.dockerignore` có `*@tmp` |
| Rancher/RKE2: netpol, CNI flannel, Import YAML không chạy kustomize | `deploy/README.md` §11 |
| Mọi lời gọi API qua web-admin 502, mọi pod xanh | Thiếu luật netpol cho web-admin ↔ dịch vụ (ADR 0043) |
| IP trong nhật ký kiểm toán là IP pod web-admin | `TRUSTED_PROXY_CIDRS` chưa đặt — đúng thiết kế (ADR 0043) |

### Vẫn đúng từ bản trước

| Vấn đề | Cách xử |
|---|---|
| **Kết quả grep âm tính không phải bằng chứng vắng mặt** | Mở tệp, không kết luận từ grep |
| **Bản vá cho lỗi X dễ là một thể hiện mới của X** | Sau khi vá một hook, chạy nó trên kho thật |
| `SET search_path` là trạng thái SESSION | Suite tích hợp phải `SetMaxOpenConns(1)` |
| Hai cột cùng kiểu cạnh nhau, đọc theo vị trí trong `Scan` | Lỗi im lặng đối xứng; ca bắt được là giá trị KHÁC nhau |
| `fmt` không gọi `String()` cho `%d %c %U %b %o` | `core/secret` cài `fmt.Formatter` |

---

## 6. Cổng kiểm

```
PYTHONIOENCODING=utf-8 mingw32-make check    # GOCACHE=/d/gocache; trên máy này có thể phải chạy từng module
```

**28/09, chạy thật:** `PYTHONIOENCODING=utf-8 mingw32-make check` **exit 0** trên cây sạch ở
`b736232` (chỉ tệp này đang sửa). `vihat-miniapp` **không** kiểm lại ở lượt này (lần xanh cuối của
phiên này ở `e274d21`; `cf47da8` là của phiên song song).

**CHƯA KIỂM:** chưa lần `zmp deploy` thật nào; chưa chạy app trên máy thật / webview Zalo; cầu phiên
chưa từng gọi thật giữa hai kho; `CITIZEN_CORS_ALLOWED_ORIGINS` cho pod identity chưa đối chiếu
manifest; ca đột biến của `rut-cong-khai-khi-doi-so-hoac-khoa` bị lớp cấp quyền từ chối; nút gửi
khối Trả lại và lời gọi danh bạ trong effect (web không DOM).

**Thứ cổng KHÔNG phủ:**

| Không phủ | Hệ quả |
|---|---|
| **PostgreSQL thật** | Mọi `*_pg_test.go` SKIP. Ranh giới xã của JOIN hàng chờ lùi hạn chỉ được chứng minh qua ca kiểm văn bản SQL (đã đột biến: đỏ đúng) — chưa trên dữ liệu thật. `go run ./tools/schema-smoke` cần DSN |
| **`golangci-lint`** | Không có trên máy; `lint` bỏ qua nó bằng tiền tố `-` (dòng `Error 2 (ignored)` là nó) |
| **Jenkins / k8s** | Chưa ai đọc số build hay tag trên Harbor; `vigov-deploy` chưa từng chạm cụm |
| **Trình duyệt** | vitest chạy node không DOM — effect, click, luồng async chỉ phủ qua hàm thuần và render tĩnh |
| **Zalo** | Không ca kiểm nào gọi Zalo thật; `dich-den` kiểm claim token bằng JWT giả |
| **Hook có NHÌN THẤY gì không** | Phép thử duy nhất đáng tin là đột biến |

---

## 7. Việc treo

**Không nằm ở đây.** Mỗi việc treo ở module của nó với `trang_thai: "treo"` — `kb/90-ephemeral/tien-do.md`.
Phân biệt: **`treo`** là *không ai chặn, ta chọn chưa làm*; **bị chặn** là chờ một câu ở §3.
