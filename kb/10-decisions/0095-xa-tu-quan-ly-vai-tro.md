---
id: 0095-xa-tu-quan-ly-vai-tro
tier: T1
source: CURATED
owner: domain
derived_from_commit: 36bc4747
expires: null
owns_facts:
  - "vai trò là chức danh mang bộ quyền (Chủ tịch, Phó Chủ tịch, Bí thư…); xã tự tạo, đổi tên, xoá mềm vai trò của mình trên màn Phân quyền — đảo ADR 0055 §1 (chủ dự án 15/10/2026)"
  - "xã mới tự có tám vai trò mẫu ngay khi owner tạo quản trị đầu tiên; xã cũ dùng nút 'Thêm vai trò mẫu' — đảo ADR 0055 §2 'chỉ khi quản trị xã bấm' (chủ dự án 15/10/2026)"
  - "Bí thư không vào danh sách vai trò mẫu; xã nào cần thì tự tạo (chủ dự án 15/10/2026)"
---

# 0095. Xã tự quản lý danh sách vai trò; xã mới có sẵn tám vai trò mẫu

**Trạng thái:** đã chốt · **Ngày:** 2026-10-15 · **Người quyết:** chủ dự án, phiên chính 15/10/2026 ·
**Đảo** ADR 0055 §1 (*"không có chức năng tạo vai trò mới"*) và một dòng của §2 (*"không tự chạy lúc cấp
xã"*) — đúng hai điều kiện dừng #1, #2 của ADR 0055 nên cần ADR này.

## Bối cảnh

| # | Sự việc |
|---|---|
| 1 | Chủ dự án tạo quản trị xã trong platform-admin, vào web-admin màn *Thêm cán bộ*: ô **Vai trò** chỉ có một lựa chọn "Quản trị hệ thống", và không tìm được chỗ cấu hình vai trò |
| 2 | Lời chủ dự án: *"vai trò ở đây theo prototype chính là chức danh theo đúng nghĩa đó, ví dụ chủ tịch xã/phường hay phó chủ tịch, bí thư… và 1 cán bộ có thể kiêm nhiệm nhiều vai trò"* (kiêm nhiệm đã có từ ADR 0094) |
| 3 | Tám vai trò mẫu có sẵn trong mã (`service-identity/internal/domain/role_template.go`) và tuyến `POST /api/v1/roles/defaults`, nhưng **không màn nào gọi** tuyến ấy — xã mới chỉ có `quan-tri-he-thong` |
| 4 | Không có tuyến tạo / đổi tên / xoá vai trò (ADR 0055 §1) — nên "Bí thư" hay bất kỳ chức danh nào ngoài tám mẫu không có đường vào |

## Quyết định

Chủ dự án chọn *"Cả hai"* và *"Bí thư: không, xã tự tạo"*.

| # | Quyết định | Ghi chú |
|---|---|---|
| 1 | **Xã mới tự có tám vai trò mẫu** ngay khi owner tạo tài khoản quản trị đầu tiên từ platform-admin (cùng giao dịch tạo vai trò `quan-tri-he-thong`) | Đảo dòng *"Ai kích"* của ADR 0055 §2. Các dòng khác của §2 và toàn bộ §3–§5 **giữ nguyên**: vai trò đã có không bị đụng, vai trò đã xoá không hồi sinh, không cấp bù khoá |
| 2 | **Xã cũ** (đã có quản trị trước ngày này): nút **"Thêm vai trò mẫu"** trên màn Phân quyền, gọi `POST /api/v1/roles/defaults` sẵn có | Luật "người bấm phải cầm mọi khoá sẽ cấp" (câu mở #14) giữ nguyên cho nút này |
| 3 | **Xã tự tạo, đổi tên, xoá mềm vai trò** trên màn Phân quyền, quyền `admin.role`. Vai trò mới **không mang khoá nào** — tích trên ma trận như mọi vai trò | Đảo ADR 0055 §1. Tạo vai trò không cấp khoá nên không mở lối leo quyền; gán khoá vẫn qua ma trận, nơi câu mở #14 đang canh |
| 4 | `quan-tri-he-thong` **không đổi tên, không xoá** | Là vai trò quản trị của xã, chủ là ADR 0046 |
| 5 | Xoá vai trò là **xoá mềm**, bắt buộc lý do, **từ chối khi còn cán bộ giữ** vai trò | Luật 7; gỡ vai trò khỏi cán bộ trước là hành động có người chịu trách nhiệm, không phải hệ quả phụ của việc xoá |
| 6 | "Bí thư" **không** vào danh sách vai trò mẫu | Xã nào cần thì tự tạo và tự tích quyền |

## Cái giá

- ADR 0055 §1 nói: dựng tạo vai trò là *"thêm một tính năng khách chưa đặt"* — nay là chủ dự án đặt.
- Mỗi xã có thể có danh sách vai trò khác nhau; báo cáo hay tính năng nào sau này dựa vào **mã** vai trò mẫu
  (ví dụ `chu-tich-ubnd`) sẽ không thấy vai trò xã tự tạo dù cùng ý nghĩa.
- Tự gieo lúc tạo quản trị đầu tiên nghĩa là xã nhận bộ quyền mẫu của ADR 0055 §4 mà chưa ai trong xã
  đọc qua; F4, F5 của ADR 0055 (trưởng thôn đọc phản ánh cả xã; chuyên viên đọc cả tố cáo) vì thế vào xã
  ngay, không chờ ai bấm. Quản trị xã bỏ tick được.

→ ADR 0055 (tám vai trò mẫu) · 0094 (một cán bộ nhiều vai trò) · 0046 (quản trị đầu tiên) · 0048 §Sửa
đổi 13/10/2026 (owner tạo quản trị xã)
→ Câu mở #14: `kb/00-foundation/open-questions.json`
→ Luật 5 · 6 · 7
