---
id: 0080-giai-ngan-cap-nhat-theo-prototype-08-10
tier: T1
source: CURATED
owner: domain
derived_from_commit: d77d739a
expires: null
owns_facts:
  - "tỷ lệ % của Giải ngân (hạng mục, dự án, phân bổ, nguồn vốn) làm tròn nửa xa số 0 tới phần trăm trăm (2 chữ số thập phân), như Thu - Chi đã làm — không cắt bớt (chốt 08/10/2026)"
  - "'nguy cơ không giải ngân hết' là CỜ do người có budget.update tự tick trong form Sửa dự án, có vết trước/sau; ô KPI 'Cần chú ý' đếm số dự án đang có cờ — không tính bằng công thức (chốt 08/10/2026, SRS M3.3.3)"
  - "vạch 'Hoàn thành dự kiến' theo prototype: tháng của ngày hoàn thành, mặc định T12; không dùng thời hạn giải ngân; ngày ở năm sau vẽ ở T12, năm trước vẽ ở tháng đầu (chốt 08/10/2026)"
  - "đường 'Thực hiện' của biểu đồ luỹ kế vẽ cong mượt (monotone) nhưng dừng ở tháng hiện tại, không kéo tới T12 (chốt 08/10/2026)"
  - "nhắc tên trong Trao đổi của dự án tạo thông báo CHUÔNG web-admin cho từng người được nhắc, trừ chính người viết, tiêu đề 'Bạn được nhắc trong trao đổi giải ngân: {tên dự án}', nội dung 280 ký tự đầu (theo prototype); gửi bằng gọi gRPC comms ngay sau khi lưu, có khoá chống trùng, lỗi thì ý kiến vẫn lưu — sửa ADR 0075 #5 (outbox) cho đúng loại thông báo này; không gửi Zalo (chốt 08/10/2026)"
  - "nạp Excel Thu - Chi theo tài liệu giao việc 08/10 + prototype: tự đánh sao dòng 'Tổng số', đoán vai trò cột từ tiêu đề (sửa lại được), chặn khi năm HOẶC tháng đã chốt, bảng đã có số liệu nhập tay phải Gỡ trước, cột % do máy chủ tính — thay các câu 30/09 trái với điều này (chốt 08/10/2026)"
---

# 0080. Giải ngân và Thu - Chi cập nhật theo prototype (08/10/2026)

**Trạng thái:** đã chốt · **Ngày:** 2026-10-08 · **Người quyết:** chủ dự án, trả lời từng câu khi giao
tài liệu `tmp/web/updated/giai-ngan-update-theo-prototype.md`.

## Bối cảnh

Tài liệu giao việc 08/10 liệt kê các chỗ bản build lệch prototype ở Giải ngân và Thu - Chi. Dò mã
cho thấy một số chỗ chạm quyết định cũ hoặc không có lời đáp trong prototype; chủ dự án trả lời
từng câu.

## Quyết định

1. **Làm tròn tỷ lệ.** Máy chủ đang chia nguyên rồi cắt bớt (`domain/disbursement_summary.go`
   `RatioOf`, `du_an.go` `TyLeGiaiNgan`…), nên hai cột tỷ lệ cộng không đủ 100%. Đổi sang làm tròn
   nửa xa số 0, như `PercentBasisPoints` của Thu - Chi. Hệ quả: điểm chậm có thể dịch tối đa 0,005
   và một dự án sát ngưỡng có thể đổi trạng thái chậm.
2. **Cờ nguy cơ.** Prototype lưu cờ `at_risk` do lãnh đạo đánh dấu (SRS M3.3.3) nhưng không có chỗ
   bật. Đặt ô tick trong form Sửa dự án, khoá `budget.update` như tuyến PATCH của prototype; mỗi lần
   đổi ghi vết trước/sau (luật 6).
3. **Vạch hoàn thành dự kiến** theo prototype, không theo tài liệu (tài liệu thêm thời hạn giải ngân
   và bỏ vạch khi ngoài năm; chủ dự án chọn giống prototype).
4. **Đường thực hiện** cong mượt, dừng ở tháng hiện tại: tháng chưa tới không có số thực hiện.
5. **Thông báo nhắc tên.** Hành vi theo prototype. Cách chuyển sang comms prototype không có (một
   khối, một cơ sở dữ liệu) nên theo đề xuất: gọi gRPC sau khi lưu. Đánh đổi: comms lỗi thì thông
   báo có thể mất (ghi nhật ký), đổi lại không phải dựng hạ tầng outbox chưa có ở đâu trong kho.
6. **Nạp Excel Thu - Chi** theo tài liệu và prototype; các câu 30/09 trái với nó (không tự đoán dòng
   tổng/vai trò cột, chỉ chặn khi chốt năm) được thay.

## Hệ quả

- Kiểu thông báo mới ở comms (proto + migration CHECK), loại khỏi danh sách Zalo.
- `core/xlsx` cần đọc nhiều sheet; mọi luồng nhập Excel đang dùng nó phải giữ nguyên hành vi.
