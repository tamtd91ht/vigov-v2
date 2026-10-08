---
id: 0082-nhiem-vu-cap-nhat-theo-prototype-08-10
tier: T1
source: CURATED
owner: domain
derived_from_commit: 29a1b904
expires: null
owns_facts:
  - "khi tài liệu giao việc Nhiệm vụ 08/10 và MÃ NGUỒN prototype (../vigov-require) nói khác nhau, theo mã nguồn prototype — giữ ADR 0065 NV5 (một ô đơn vị/người thực hiện, hai tên gọi), NV6 (hạn mặc định +7 ngày 17:00), ADR 0076 lần 2 #17 (form 500px, 800px khi loại Theo văn bản) (chốt 08/10/2026)"
  - "danh mục loại / khối / mức ưu tiên nhiệm vụ của xã sửa bằng CẤU HÌNH trên web (dữ liệu), không viết migration đổi mã trên nhiệm vụ đã có (§11 của tài liệu giao việc không làm) (chốt 08/10/2026)"
  - "Danh sách nhiệm vụ xếp trên máy chủ theo Mã, Tên việc, Ưu tiên, Hạn, Trạng thái; Người thực hiện / Bộ phận không xếp, không dấu '?'; màu vạch thẻ Kanban = màu xã chọn của mức ưu tiên, không có thì theo mã (khan đỏ, cao cam, còn lại xanh) (chốt 08/10/2026)"
  - "email cán bộ hiện ở dạng che (privacy.MaskEmail) trong ngăn chi tiết nhiệm vụ và ô chọn cán bộ, có nút 'Xem' lấy email đầy đủ từ máy chủ; ai có task.read đều xem được; MỖI lần xem ghi vết đọc dữ liệu cá nhân đầy đủ (luật 6 bất biến 7) — không thêm khoá quyền mới (chốt 08/10/2026)"
---

# 0082. Nhiệm vụ cập nhật theo prototype (08/10/2026)

**Trạng thái:** đã chốt · **Ngày:** 2026-10-08 · **Người quyết:** chủ dự án, trả lời khi giao tài
liệu `tmp/web/updated/nhiem-vu-theo-prototype.md`.

## Bối cảnh

Tài liệu giao việc 08/10 mô tả site Thăng Bình lệch prototype: Sổ theo dõi báo lỗi, form Giao việc
thiếu trường, ngăn chi tiết thiếu khối văn bản chỉ đạo, vạch thẻ Kanban đỏ hết. Dò mã cho thấy
phần lớn đã có ở HEAD; gốc là **dữ liệu danh mục của xã**: không có loại mã `theo-van-ban`
(máy chủ chỉ nhận ra loại "theo văn bản" qua đúng mã này, `domain/loai_nhiem_vu.go`) và mức ưu tiên
xếp `thuong` đầu tiên (màu vạch theo thứ hạng, hạng đầu là đỏ).

Một số mục của tài liệu trái quyết định cũ, và chính mã nguồn prototype lại khớp quyết định cũ
(`TaskAssignForm.tsx` điền sẵn hạn +7 ngày, rộng 500/800, một ô hai tên gọi).

## Quyết định

1. **Prototype nguồn thắng tài liệu giao việc** khi hai bên khác nhau. Không tách cơ quan chủ trì /
   chuyên viên theo dõi thành trường riêng, không để trống hạn mặc định, không ép form luôn 800px,
   không thêm lại hai cột chủ trì/theo dõi vào mẫu Excel. Danh sách, Kanban, nhãn trạng thái, sắp
   xếp cột, độ rộng cột Tên việc, chỉ hiện việc gốc: làm đúng như prototype.
2. **Danh mục là dữ liệu của xã.** Xã (hoặc người vận hành) thêm loại "Theo văn bản", xếp lại mức ưu
   tiên Khẩn · Cao · Thường, đánh dấu Thường là mặc định, tắt mã cũ — trên màn Cấu hình. Không
   migration đổi mã trên nhiệm vụ đã có: mã là khoá bất biến, nhiệm vụ cũ giữ mã cũ (dòng danh mục
   cũ tắt chứ không xoá). Không chép dữ liệu mẫu của prototype vào hồ sơ của xã.
3. **Email cán bộ** hiện dạng che, có nút "Xem". Khoá quyền dùng `task.read` (đã có trong bảng
   `quyen`) vì chủ dự án chọn phạm vi này; mỗi lần xem là một dòng nhật ký kiểm toán (ai, xem email
   của cán bộ nào, lúc nào, IP, xã). Danh bạ chọn người không bao giờ trả email đầy đủ.
4. **Nhật ký** hiện chip trạng thái + ghi chú như prototype; dòng máy tự sinh
   "Chuyển trạng thái: <mã> → <mã>" đã lưu thì giữ nguyên (nhật ký chỉ thêm, luật 7), chỉ ẩn khi hiển thị.

5. **Sắp xếp Danh sách** (thu hẹp ở #11): cả 7 cột bấm được như prototype, nhưng xếp **trên máy chủ** (không xếp
   các dòng đã tải trên trình duyệt), để thứ tự đúng trên toàn bộ dữ liệu và Xuất Excel theo đúng
   thứ tự đang xem. Thay ADR 0076 lần 2 #11 (chỉ 4 cột, 3 cột còn lại "?").
6. **Màu vạch thẻ Kanban**: dùng màu xã chọn trong danh mục mức ưu tiên (cột `color`); chưa chọn thì
   theo mã như prototype (`khan` đỏ, `cao` cam, còn lại xanh thương hiệu). Thay quy tắc theo thứ hạng
   của ADR 0076 lần 2 #10.
7. **Nhãn trạng thái ở menu Nhiệm vụ không có icon**, như prototype (`TaskListTable.tsx`) — thay
   ADR 0068 lần 6 #7 cho riêng menu này. "Hoàn thành trễ hạn" giữ (prototype có).
8. **Chỉ hiện việc gốc** ở Kanban, Danh sách và Sổ theo dõi như prototype; riêng khi bấm sâu từ Tổng
   quan (`?metric=`) vẫn đếm cả việc con để số dòng bằng con số trên Tổng quan (ADR 0053).
9. **Sổ theo dõi** chọn loại như prototype: loại đang dùng có cờ văn bản chỉ đạo → loại mặc định →
   loại đầu tiên đang dùng → không lọc loại; không báo lỗi chặn màn.
10. **Nút "Xem" email** chỉ đặt ở ô Người thực hiện của ngăn chi tiết; trong ô chọn cán bộ chỉ hiện
    email đã che (một `<option>` không chứa được nút).

11. **Thu hẹp #5 (chủ dự án, cùng ngày):** chỉ làm đúng tài liệu giao việc. Thêm xếp theo **Trạng
    thái** (thứ tự trạng thái của xã); cột Người thực hiện / Bộ phận bỏ dấu "?" và **không** xếp
    được — tên nằm ở identity, xếp đúng theo tên cần lưu bản chụp tên trên nhiệm vụ (migration +
    chạy nền), quá phạm vi yêu cầu. Làm sau nếu khách cần.
12. **Email che** dùng hàm chung `privacy.MaskEmail` (ký tự đầu + tên miền, ví dụ `t***@vigov.vn`);
    mẫu `tamtd****@****.com` của chủ dự án là ví dụ minh hoạ, không phải định dạng bắt buộc.

## Hệ quả

- Site Thăng Bình chỉ đúng prototype sau khi danh mục được cấu hình lại; mã không đổi được việc này.
- Email cán bộ lộ (dạng đầy đủ) cho mọi người có `task.read` khi họ bấm Xem — đổi lấy dấu vết đầy đủ.
- Nếu sau này cần khoá riêng cho việc xem thông tin cán bộ, đó là câu hỏi mở #27, không tự thêm khoá.
