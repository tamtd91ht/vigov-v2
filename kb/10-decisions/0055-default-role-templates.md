---
id: 0055-default-role-templates
tier: T1
source: CURATED
owner: architecture
derived_from_commit: 45f4f11
expires: null
owns_facts:
  - "vì sao v2 không có chức năng tạo vai trò mới, và tám vai trò mẫu đi vào xã bằng hành động của quản trị xã"
  - "tám vai trò mẫu: mã, tên, cờ lãnh đạo, thứ tự, số khoá — và luật ánh xạ khoá từ kho yêu cầu sang v2"
  - "vì sao gieo vai trò mẫu không bao giờ cấp lại khoá cho vai trò đã có"
  - "vì sao không vai trò mẫu nào giữ feedback.classify, feedback.unmask, admin.user.delete"
  - "hai chỗ nhà cung cấp theo kho yêu cầu mà khách chưa xác nhận: trưởng thôn đọc phản ánh cả xã; chuyên viên và cán bộ một cửa đọc đơn thư kể cả tố cáo"
---

# 0055. Tám vai trò mẫu gieo theo hành động của quản trị xã

**Trạng thái:** đã chốt · **Ngày:** 2026-09-28 · người dùng chọn "bỏ mục Thêm vai trò, gieo 8 vai trò mẫu" và "làm theo đề xuất, đối chiếu vigov-require" (28/09/2026); bảng quyền từng vai trò và F4/F5 chưa được duyệt từng dòng · mọi dòng ghi **"nhà cung cấp theo kho yêu cầu"**
là **chưa được khách xác nhận** · Phạm vi: menu `/cau-hinh`, tab Phân quyền
(`docs/ui-ux/14-cau-hinh.md` §4) · **Trả lời** dòng *"Ai được tạo vai trò mới"* của ADR 0046
§*Việc còn mở* · **Thay** mục *"Thêm vai trò mới ở tab Phân quyền (§4)"*
(`web-admin/src/features/cau-hinh/nhan-cau-hinh.ts:70`).

## Bối cảnh

| Sự thật | Nơi |
|---|---|
| Xã mới chỉ có **một** vai trò — `quan-tri-he-thong`, gieo ở lần đăng nhập `admin` đầu tiên | ADR 0046 Quyết định 2 |
| ADR 0046 để ngỏ: *"Ai được tạo vai trò mới — Chưa ai quyết"* | `kb/10-decisions/0046-quy-hoach-ten-mien-va-quan-tri-dau-tien-cua-xa.md:154` |
| Đặc tả §4 **chỉ** vẽ tám vai trò mẫu và ma trận tick — **không** có nút tạo vai trò | `docs/ui-ux/14-cau-hinh.md:85-102` |
| Kho yêu cầu cũng **không** có tuyến tạo vai trò: chỉ `GET /org/roles` và `PUT /org/roles/{id}/permissions` | `../vigov-require/docs/spec/04-api.md:221-222` |
| Kho yêu cầu gieo tám vai trò **lúc cấp xã** (`provision_tenant`) | `../vigov-require/apps/api/app/modules/org/provisioning.py:63-129` |
| Bảng `vai_tro` đã có `ma` duy nhất theo xã, `la_lanh_dao`, `thu_tu`, xoá mềm | `service-identity/migrations/0001_init.sql:107-121` |
| Bảng `quyen` hôm nay có **36** khoá: 33 ở `0001`, hai ở `0007`, `admin.user.delete` ở `0010` | `service-identity/migrations/0001_init.sql:279`, `0007_…sql:57-59`, `0010_…sql:267` |

## Quyết định

### 1. Không có chức năng tạo vai trò mới

Cả đặc tả lẫn kho yêu cầu đều không có nó. Dựng nó là **thêm một tính năng khách chưa đặt**, kèm
câu hỏi ai được tạo, tạo vai trò mạnh tới đâu — đúng thứ câu mở #14 vừa khép. Tám vai trò mẫu và
ma trận tick là **toàn bộ** bề mặt §4.

### 2. Tám vai trò đi vào xã bằng một hành động tường minh

`POST /api/v1/roles/defaults`, sau `RequirePermission("admin.role")`.

| Điều | Quyết định | Vì sao |
|---|---|---|
| Ai kích | Quản trị xã bấm, **không** tự chạy lúc cấp xã hay lúc đăng nhập đầu | Tiền lệ `POST /api/v1/sla/defaults` (`service-identity/internal/http/routes.go:2085-2088`): con số của xã đi vào xã khi xã chọn, có vết, có người chịu trách nhiệm |
| Chạy lại | **Idempotent**: lần sau chỉ tạo vai trò còn thiếu | Bấm hai lần không được ghi đè gì |
| Người bấm phải cầm | **Mọi khoá** mà lượt gieo sẽ cấp | Câu mở #14: *"không gán được quyền mình KHÔNG CẦM"* (`kb/00-foundation/open-questions.json:320`). Gieo vai trò là gán quyền |
| Vai trò đã có cùng `ma` | **Không đụng** — cả tên, cờ, thứ tự **lẫn khoá đã cấp** | Xem §3 |
| Vai trò cùng `ma` đã xoá mềm | **Bỏ qua**, không hồi sinh, không tạo lại | Có dòng nghĩa là ai đó đã chọn gỡ; hồi sinh là đảo ngược lặng lẽ quyết định ấy (cùng lý lẽ ADR 0046 Quyết định 2) |
| `quan-tri-he-thong` | **Không đụng** | Không thuộc tám mẫu; chủ của nó là ADR 0046 |
| Vết | **Một mục mỗi vai trò tạo ra**, cùng giao dịch, "ai" là mã cán bộ người bấm | Luật 6 bất biến 1, 3, 8 |

Gieo bằng hành động quản trị, không phải migration — nên không chạm điều kiện dừng #3 của ADR 0030
(*"Gán khoá cho một vai trò trong migration hạt giống"*).

### 3. Không bao giờ cấp lại khoá cho vai trò đã có — cố ý khác kho yêu cầu

Kho yêu cầu chạy lại `_apply_roles` thì **cấp bù** mọi khoá mẫu còn thiếu cho vai trò đã có
(`../vigov-require/apps/api/app/modules/org/provisioning.py:131-150`). v2 **không**.

Một khoá thiếu trên vai trò đã có có thể là khoá quản trị xã **vừa bỏ tick**. Cấp bù là lật ngược
quyết định phân quyền của xã, trong im lặng, bằng một nút mà người bấm tưởng chỉ "thêm cái còn
thiếu".

### 4. Tám vai trò

| `thu_tu` | `ma` | `ten` | `la_lanh_dao` | Số khoá |
|---|---|---|---|---|
| 1 | `chu-tich-ubnd` | Chủ tịch UBND | có | 33 |
| 2 | `pho-chu-tich-ubnd` | Phó Chủ tịch UBND | có | 23 |
| 3 | `chanh-van-phong` | Chánh Văn phòng | không | 16 |
| 4 | `truong-bo-phan` | Trưởng bộ phận | không | 12 |
| 5 | `chuyen-vien` | Chuyên viên chuyên môn | không | 8 |
| 6 | `ke-toan` | Kế toán | không | 6 |
| 7 | `can-bo-mot-cua` | Cán bộ một cửa | không | 6 |
| 8 | `truong-thon` | Trưởng thôn, Tổ trưởng dân phố | không | 4 |

Tên và cờ lãnh đạo theo `docs/ui-ux/14-cau-hinh.md:93-102`. `ma` là của v2 (kho yêu cầu dùng
`chu-tich`, `tiep-nhan`…); `chu-tich-ubnd` đã là ví dụ trong `service-identity/migrations/0001_init.sql:111`.

### 5. Luật ánh xạ khoá: kho yêu cầu → v2

Nguồn: `../vigov-require/apps/api/app/modules/org/data/default_config.json:186-333`.

| Luật | Nội dung |
|---|---|
| Khoá có ở v2 | Giữ nguyên chuỗi |
| Khoá đại diện `nhóm.*` | Mở thành **mọi khoá của nhóm ấy trong bảng `quyen` v2**, trừ ba khoá dưới |
| `dossier.*` (mọi dạng) | **Bỏ** — không phải phát hiện cho câu mở #27: hồ sơ một cửa **ngoài phạm vi hợp đồng** (ADR 0001 §*Bổ sung 2026-09-20*, `kb/10-decisions/0001-service-decomposition.md:109-127`) |
| `feedback.classify`, `feedback.unmask` | **Không vai trò mẫu nào** — ADR 0030:95-96 cấm tự cấp |
| `admin.user.delete` | **Không vai trò mẫu nào** — khoá đóng mặc định, chờ quản trị xã tick (`service-identity/migrations/0010_…sql:45-47`) |

Hệ quả kiểm được: Chủ tịch UBND mang `admin.*`, `task.*`, `feedback.*`… tức mọi khoá của bảng trừ
đúng ba khoá trên — 36 − 3 = 33.

## Cái giá

**Xã chỉ dùng vai trò mẫu thì chỉ `quan-tri-he-thong` phân loại được phiếu và gỡ che được người
gửi.** Phân loại là hành vi ấn định hạn (ADR 0028 quyết định E), và phiếu quá trần phân loại một
ngày làm việc **tính là trễ** (câu mở, `kb/00-foundation/open-questions.json:627`). Nên xã quên
tick `feedback.classify` cho ai đó thì chỉ số của xã xấu đi mà không ai hiểu vì sao.

Đề nghị cho màn Phân quyền: một khoá **không vai trò nào ngoài `quan-tri-he-thong` giữ** thì hiện
cảnh báo *"không vai trò nào giữ khoá"* ở hàng ấy. Đó là cách duy nhất một xã thấy được hậu quả
của quyết định "không tự cấp" mà không phải đọc ADR này.

**Chủ tịch UBND mang `admin.user` và `admin.role`** — tức toàn quyền quản trị, như kho yêu cầu.
Người bấm gieo phải cầm cả hai (câu mở #14), nên điều này không mở lối leo quyền; nó là một lựa
chọn của mẫu mà xã bỏ tick được.

## Còn mở — nhà cung cấp theo kho yêu cầu, khách xác nhận sau

| # | Việc | Vì sao phải hỏi | Của ai |
|---|---|---|---|
| F4 | `truong-thon` giữ `feedback.read` **toàn xã**, không chỉ thôn/tổ của mình | Phiếu mang nội dung, ảnh hiện trường, toạ độ — dữ liệu cá nhân theo Nghị định 13/2023; một trưởng thôn đọc phiếu của mọi thôn là rộng hơn việc cần | Khách |
| F5 | `petition.read` cho `chuyen-vien` và `can-bo-mot-cua` gồm cả **tố cáo** | Luật Tố cáo bắt giữ bí mật danh tính người tố cáo; mẫu đang cho hai vai trò đông người nhất đọc | Khách |

Đổi được miễn phí khi chưa xã nào bấm gieo; sau đó mỗi xã đã gieo phải tự bỏ tick, vì §3 cấm gieo
lại sửa vai trò đã có.

## ĐIỀU KIỆN DỪNG

1. Thêm chức năng tạo vai trò mới — đảo §1, cần ADR mới
2. Gieo tự chạy không cần người bấm (lúc cấp xã, lúc đăng nhập đầu, trong migration)
3. Gieo lại cấp bù khoá, sửa tên/cờ, hay hồi sinh vai trò đã xoá mềm
4. Một vai trò mẫu mang `feedback.classify`, `feedback.unmask` hoặc `admin.user.delete`
5. Một khoá mẫu không có trong bảng `quyen` — là phát hiện cho câu mở #27, không phải một `INSERT`

→ ADR 0001 (một cửa ngoài phạm vi) · 0028 (phân loại ấn định hạn) · 0030 (không tự cấp hai khoá
phản ánh) · 0046 (quản trị đầu tiên của xã)
→ Câu mở #14, #27: `kb/00-foundation/open-questions.json`
→ Luật 3 · 5 · 6 · 7
