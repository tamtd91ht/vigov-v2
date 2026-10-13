---
id: 0094-staff-multiple-roles
tier: T1
source: CURATED
owner: domain
derived_from_commit: 027a8e3d
expires: null
owns_facts:
  - "một cán bộ giữ NHIỀU vai trò; quyền của người ấy là HỢP quyền các vai trò đang giữ — đảo lựa chọn 'một vai trò/người' ngày 08/10/2026 (chủ dự án chốt 13/10/2026, bảng lỗi web-admin-131026 dòng 10)"
  - "vì sao vai trò của cán bộ nằm ở bảng gán riêng mà vẫn giữ nguoi_dung.vai_tro_id làm vai trò chính (13/10/2026)"
  - "chặn leo quyền (câu mở #14) khi đổi tập vai trò chỉ xét các vai trò ĐƯỢC THÊM, không xét vai trò người ấy đã giữ (giả định 13/10/2026)"
  - "popup thêm người dùng không có ô mật khẩu, giữ mật khẩu tạm do hệ thống cấp — từ chối lại ô '≥8 ký tự' của prototype (chủ dự án 13/10/2026)"
---

# 0094. Một cán bộ giữ nhiều vai trò, quyền là hợp các vai trò

**Trạng thái:** đã chốt · **Ngày:** 2026-10-13 · **Người quyết:** chủ dự án, 13/10/2026, trả lời bảng lỗi
web-admin-131026 dòng 10; lời do phiên chính chuyển lại · **Đảo** lựa chọn "một vai trò/người" ngày
08/10/2026 · **Đã làm:** commit `027a8e3d`.

## Bối cảnh

| # | Sự việc |
|---|---|
| 1 | Ngày 08/10/2026 web-admin chọn mỗi cán bộ **một** vai trò. Lựa chọn ấy không có ADR; nó chỉ nằm ở chú thích `service-identity/internal/domain/danh_ba_ghi.go` và sổ tiến độ |
| 2 | Prototype cho chọn nhiều vai trò: biểu mẫu người dùng là lưới ô tích, gửi `role_ids` (`../vigov-require/apps/admin/src/components/admin/UserFormDialog.tsx:223-245`, dẫn ở `service-identity/migrations/0031_staff_role_assignment.sql:2-3`) |
| 3 | Bảng lỗi 13/10/2026 dòng 10 đòi theo prototype. Chủ dự án đồng ý |

## Quyết định

| # | Điểm | Ở đâu |
|---|---|---|
| 1 | Một cán bộ giữ **nhiều** vai trò. Quyền = **hợp** quyền các vai trò còn hiệu lực | `service-identity/internal/store/checker.go:47-63` (`noiGiuQuyen`) |
| 2 | Vai trò của cán bộ nằm ở bảng `staff_role_assignment`: `tenant_id` ghép, khoá ngoại tới vai trò cùng xã, gỡ vai trò là xoá mềm | `migrations/0031_staff_role_assignment.sql` |
| 3 | `nguoi_dung.vai_tro_id` **giữ lại** làm vai trò chính (vai trò đầu của tập), chỉ để hiển thị. Ai cầm quyền gì chỉ đọc từ bảng gán | `0031_staff_role_assignment.sql:12-16` |
| 4 | Đếm "quản trị viên cuối cùng" (câu mở #13) dùng **cùng** vị từ với bộ kiểm quyền | `internal/store/can_bo_ghi.go:410-415` |
| 5 | Chặn leo quyền (câu mở #14): không tự đổi vai trò của mình; mỗi vai trò **được thêm** không được mang khoá người thao tác không cầm | `internal/app/danh_ba_can_bo.go:836-889` |
| 6 | `PUT /api/v1/staff/{id}/role` nhận `role_ids` = **cả tập** vai trò mới. `GET /api/v1/sessions/current` thêm `roles[]` | `internal/app/danh_ba_can_bo.go:831-832` · `internal/http/phien_hien_tai.go:51-56` |
| 7 | Nhập Excel giữ **một vai trò mỗi dòng** (mẫu có một cột Vai trò) | `internal/app/staff_import.go:60` |
| 8 | Popup thêm người dùng **không** có ô mật khẩu. Giữ mật khẩu tạm do hệ thống cấp (câu mở #9 chọn (b)); độ dài theo câu mở #36. Ô "≥8 ký tự" của prototype bị từ chối vì yếu hơn chính sách mật khẩu | Như ADR 0090 §*Từ chối* dòng 41; ADR 0059 §1 |

**Giả định ở #5 (của lượt dựng, chủ dự án chưa xác nhận riêng):** vai trò người ấy **đã giữ** không bị xét
lại. Giữ nguyên một vai trò không trao thêm gì, còn gỡ vai trò chỉ làm hẹp quyền. Nếu xét lại cả vai trò
cũ, quản trị viên yếu hơn đồng nghiệp sẽ không gỡ được vai trò nào của đồng nghiệp ấy, dù gỡ là thao tác
an toàn.

## Hệ quả

- **Câu "người này có quyền X không" không còn đọc được từ một cột.** Phải đi qua bảng gán. Đọc
  `vai_tro_id` để quyết quyền là sai, kể cả khi kết quả trông đúng với cán bộ chỉ có một vai trò.
- **#13 có thêm một đường lọt nếu hai phép kiểm tách nhau.** Một người còn quyền quản trị nhờ vai trò
  thứ hai; nếu phép đếm chỉ nhìn vai trò chính, xã bị báo "mất quản trị viên cuối" sai, hoặc ngược lại
  lọt thật. Vì vậy #4 dùng chung vị từ, không viết lại.
- **Mỗi vai trò mạnh trong tập vẫn là một lần trao quyền.** Một vai trò mạnh lẫn giữa các vai trò yếu
  vẫn bị #14 chặn; xét theo từng vai trò được thêm, không theo vai trò chính.
- **Nhập Excel và màn sửa khác nhau.** Excel tạo người với một vai trò; muốn thêm vai trò thì sửa trên
  màn Người dùng sau khi nhập.

→ Câu mở #9 · #13 · #14 · #36 (`kb/00-foundation/open-questions.json`) · ADR 0055 (vai trò mẫu) · ADR 0059
§1 · ADR 0090
