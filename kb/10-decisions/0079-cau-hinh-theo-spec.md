---
id: 0079-cau-hinh-theo-spec
tier: T1
source: CURATED
owner: domain
derived_from_commit: e98f2c4d
expires: null
owns_facts:
  - "từ 08/10/2026 menu Cấu hình web-admin theo spec Cấu hình 02–12 của chủ dự án (tmp/web/cau-hinh/vigov-cau-hinh-spec/, ngoài git, viết từ prototype ../vigov-require/apps/admin/src/components/admin/ConfigWorkspace.tsx); luật cứng và các điểm 'Giữ bất kể spec' của ADR này thắng spec"
  - "menu Cấu hình giữ hai tab ngoài spec — Nhật ký hệ thống (ADR 0054) và Nhận diện xã (ADR 0069) — ở CUỐI thanh tab (chốt 08/10/2026)"
  - "ba bảng lịch (giờ làm việc, ngày lễ, ngày làm bù) + nút gieo tách khỏi tab Thời hạn xử lý thành tab 'Lịch làm việc' đứng NGAY SAU tab Thời hạn xử lý; tab Thời hạn xử lý chỉ hiện khi có admin.sla; tab Lịch làm việc hiện cho mọi cán bộ (đọc any-authenticated), nút ghi theo khoá máy chủ đòi (chốt 08/10/2026)"
  - "chức năng máy chủ đang hỗ trợ mà spec Cấu hình bỏ (Chạy ngay + bảng lượt chạy ở Tự động hoá, thẻ 'Gửi báo cáo định kỳ ?', Trưởng thôn, sửa Loại thôn sau khi tạo, ô Thứ tự, cột 'Giữ chưa phân công' SLA, sửa Bắt buộc/tuỳ chọn trường bản đồ, Mặc định ngoài task_kind) GIỮ, trình bày theo spec (chốt 08/10/2026)"
  - "Kênh Zalo theo spec 11: bot RIÊNG của xã cấu hình ở web-admin (mã bot cất mã hoá, kiểm tra kết nối, đăng ký webhook, quay về bot chung), 18 sự kiện, danh sách cả cán bộ chưa ghép nối, lưu ngay mỗi thay đổi; web trước, backend Zalo ngay sau — THAY ADR 0074 #1, #3, #4 cho phần bot của xã (chốt 08/10/2026)"
  - "hành vi spec Cấu hình mà máy chủ chưa có (thêm/tắt/xoá câu hệ thống; thêm/xoá dòng thời hạn; màu mục danh mục; lưu kết quả gửi thư thử + dòng máy chủ thư nền tảng) sẽ làm backend; lượt web đặt control '?' (ADR 0068 §14) đúng vị trí spec (chốt 08/10/2026)"
  - "huy hiệu trạng thái menu Cấu hình giữ icon + chữ (ADR 0068 lần 2 #8b), lệch prototype có lý do (chốt 08/10/2026)"
---

# 0079. Menu Cấu hình theo spec Cấu hình 02–12

**Trạng thái:** đã chốt; phần backend còn điều kiện dừng (§*Còn mở*) · **Ngày:** 2026-10-08 ·
**Người quyết:** chủ dự án, 08/10/2026, phiên chính, lệnh `/fix-web-admin --menu=cau-hinh --des=Cập
nhật lại ui ux toàn bộ view và action trong menu cấu hình này, tham khảo hướng dẫn tại
.\tmp\web\cau-hinh\vigov-cau-hinh-spec\*.md` · **Bổ sung** ADR 0068 §*Sửa đổi 06/10/2026 (lần 5)* và
§*Sửa đổi 07/10/2026 (lần 6)* cho riêng menu Cấu hình, như ADR 0076 cho Nhiệm vụ · **Thay một phần**
ADR 0074 (#1, #3, #4 — chỉ phần bot của xã, §*Quan hệ với ADR 0074*).

## Bối cảnh

Nguồn: spec `tmp/web/cau-hinh/vigov-cau-hinh-spec/02`–`12` (ngoài git), chủ dự án viết từ prototype
`../vigov-require/apps/admin/src/components/admin/ConfigWorkspace.tsx` và các component cùng thư mục.
Spec va với mã hiện có ở ba kiểu: tab spec không có, chức năng máy chủ có mà spec bỏ, và hành vi spec
đòi mà máy chủ chưa có. Chủ dự án chọn từng điểm dưới đây (nguyên văn lựa chọn trong ngoặc kép).

## Quyết định

| # | Điểm | Chốt |
|---|---|---|
| 1 | Hai tab ngoài spec: Nhật ký hệ thống (ADR 0054), Nhận diện xã (ADR 0069) | **"Giữ cả hai ở cuối"** |
| 2 | Ba bảng lịch (giờ làm việc, ngày lễ, ngày làm bù) + nút gieo, đang ở tab Thời hạn xử lý | **"Tách thành tab 'Lịch làm việc' riêng"**, rồi **"Ẩn SLA theo spec; Lịch ngay sau SLA"**: tab Thời hạn xử lý chỉ hiện khi có `admin.sla`; tab Lịch làm việc đứng ngay sau, hiện cho mọi cán bộ (đọc any-authenticated), nút ghi theo khoá máy chủ đòi |
| 3 | Chức năng máy chủ có mà spec bỏ: Chạy ngay + bảng lượt chạy (Tự động hoá), thẻ "Gửi báo cáo định kỳ ?", Trưởng thôn, sửa Loại thôn sau khi tạo, ô Thứ tự, cột "Giữ chưa phân công" (SLA), sửa Bắt buộc/tuỳ chọn trường bản đồ, Mặc định ngoài `task_kind` | **"Giữ, trình bày theo spec"** |
| 4 | Kênh Zalo theo spec §11: bot riêng của xã (mã bot cất mã hoá, kiểm tra kết nối, đăng ký webhook, quay về bot chung); 18 sự kiện; danh sách cả cán bộ chưa ghép nối; lưu ngay mỗi thay đổi | **"Theo spec, làm luôn backend"**, xác nhận **"Đúng. Web trước, backend Zalo ngay sau"** |
| 5 | Hành vi spec đòi mà máy chủ chưa có: thêm/tắt/xoá câu hệ thống; thêm/xoá dòng thời hạn; màu mục danh mục; lưu kết quả gửi thư thử + dòng máy chủ thư nền tảng | **"Làm luôn backend cho cả những phần này"**. Lượt web đặt control "?" (ADR 0068 §14) đúng vị trí spec; backend lượt sau |
| 6 | Huy hiệu trạng thái | **"Giữ icon + chữ"** (ADR 0068 lần 2 #8b) — lệch prototype có lý do |

## Giữ bất kể spec — luật/ADR đã quyết, không hỏi

| Điểm spec | Giữ | Nguồn |
|---|---|---|
| Xoá không lý do | Bước nhập lý do vẫn có — máy chủ đòi `reason` | luật 7 · 0068 lần 5 #4 · ADR 0056 (bộ phận) |
| Xoá thôn | Thôn không có xoá; "Ngừng dùng" có xác nhận | ADR 0059 §2 · 0068 §15 |
| Khoá `admin.user` cho Sơ đồ/Thôn | `admin.org`; đọc Trường bản đồ bằng `asset.read` | luật 5 bất biến 3c · 0068 lần 5 #4 |
| Hai ô tick bảo mật máy chủ thư | Một lựa chọn STARTTLS/TLS | luật 13 |
| 11 mã nhóm tài nguyên bản đồ ghi cứng | Đọc từ danh mục xã | ADR 0072 |
| Bảng SLA | Giữ 4 loại việc gồm đơn thư; nhiệm vụ dùng dòng mặc định | ADR 0011 (migration 0016) · ADR 0029 |
| Câu "Mặc định 72 giờ, tức ba ngày" | Bỏ — số SLA ghi cứng, sai với phần lớn dòng | luật 10 cấm #3 |
| Placeholder có tên xã | Không ghi cứng tên xã | luật 1 bất biến 10 · 0068 lần 6 #2 |
| Số cán bộ | Lấy `staff_count` của máy chủ | spec 12 |
| Tên trường của spec | Đổi về tên máy chủ — spec 12 "giữ backend, chỉ map lại" | spec 12 |
| Thông báo | Toast sonner; lỗi biểu mẫu hiện tại chỗ | 0068 lần 6 #4 |

## Quan hệ với ADR 0074

Chỉ phần **bot riêng của xã**; bot dùng chung của nền tảng giữ nguyên mọi điểm của 0074.

| 0074 | Câu bị thay | Thay bằng |
|---|---|---|
| #1 | "Đợt 1 một bot dùng chung; bot riêng của xã để sau, khi làm thì cấu hình ở platform-admin" | Bot riêng của xã làm ngay sau lượt web, **xã tự cấu hình ở web-admin** (Cấu hình › Kênh Zalo), quay về bot chung được |
| #3 | Đường quản trị platform-admin → service-platform (`ops.zalo_bot.manage`) → comms | Với bot của xã: web-admin của xã → comms; đường vận hành giữ cho bot chung |
| #4 | Niêm token bằng khoá dữ liệu cấp nền tảng | Với bot của xã: mã bot là bí mật **theo xã**, cất mã hoá — cách cất **còn mở** (dưới) |

Hệ quả kèm: 4 loại tin của 0074 §*Hệ quả* không đủ cho 18 sự kiện của spec 11 — xem *Còn mở*.

## Còn mở — điều kiện dừng của lượt backend

Không quyết ở đây. Lượt backend phải hỏi chủ dự án trước khi viết mã:

| # | Điểm | Va với |
|---|---|---|
| 1 | Cách cất mã bot riêng của xã | luật 8 điều kiện dừng #1 (bí mật bên thứ ba mới, theo xã) |
| 2 | Tuyến webhook không đăng nhập cho bot của xã | luật 13 điều kiện dừng (tuyến không xác thực mới) |
| 3 | Nguồn phát của 18 sự kiện ở các service | luật 2 (chủ dữ liệu, hợp đồng sự kiện) |
| 4 | `due_soon_days` của Zalo so với `due_soon_hours` của SLA | luật 9 (một sự kiện, một nguồn); 0074 đã chốt ngưỡng "sắp đến hạn" dùng chung với chuông |
| 5 | Thêm câu hệ thống | ADR 0024 (bộ mã đóng, câu gốc sống trong mã) |
| 6 | Thêm/xoá dòng SLA | ADR 0026 điều kiện dừng #2 · ADR 0029 điều kiện dừng #4 |
