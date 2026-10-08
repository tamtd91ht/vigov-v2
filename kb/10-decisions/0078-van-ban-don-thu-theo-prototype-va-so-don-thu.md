---
id: 0078-van-ban-don-thu-theo-prototype-va-so-don-thu
tier: T1
source: CURATED
owner: domain
derived_from_commit: a1e5e64f
expires: null
owns_facts:
  - "menu Văn bản & Đơn thư làm lại theo prototype (lượt đêm 07→08/10/2026): NGHIỆP VỤ/DỮ LIỆU/PHÁP LÝ theo các câu chủ dự án đã chốt (C-list 24/09+30/09, C2 30/09, ADR 0039/0064/0007/0028/0038, luật 1–13); TRÌNH BÀY theo prototype ../vigov-require/apps/admin/src/components/documents/* và hướng dẫn tmp/web/van-ban/vigov-van-ban-spec/ (mã nguồn thắng hướng dẫn)"
  - "sổ đơn thư dựng ở service-documents, tuyến /api/v1/citizen-letters; hạn của đơn do cán bộ có petition.create tự đặt hoặc để 'Không đặt' (ADR 0079 lô 5 Q18, 08/10/2026 — thay quy định 'để TRỐNG tới khi identity có loại việc đơn thư'); web không tự tính hạn (luật 10)"
  - "đơn tố cáo: danh sách/báo cáo/cảnh báo trùng không mang danh tính lẫn trích yếu; không có khoá xem đầy đủ nào cho đơn thư nên SĐT/địa chỉ luôn che, ô sửa người gửi không điền sẵn giá trị đầy đủ, không có liên kết tel: — đóng an toàn tới khi có khoá"
  - "khung trang Văn bản & Đơn thư: 4 tab theo thứ tự Văn bản đến · Văn bản đi · Đơn thư công dân · Báo cáo; tab mặc định Đơn thư công dân TRỪ khi đường dẫn mang lọc Tổng quan (metric=…) thì Văn bản đến; tiêu đề 'Văn bản & đơn thư'; bỏ nút 'Quét & OCR' — GIẢ ĐỊNH của phiên, chờ chủ dự án xác nhận"
  - "phần prototype cần thứ máy chủ chưa có (nhập/xuất Excel, chuyển thành nhiệm vụ, gửi Zalo, OCR, tệp đính kèm, lọc phạm vi văn bản đến, gợi ý cơ quan ban hành, ghi chú/nguồn văn bản đến) = control vô hiệu '?' đúng vị trí prototype (ADR 0068 lần 5 #5)"
---

# 0078. Văn bản & Đơn thư theo prototype, và sổ đơn thư ở service-documents

**Trạng thái:** đã chốt hướng, một phần là GIẢ ĐỊNH chờ xác nhận (đánh dấu bên dưới) · **Ngày:**
2026-10-08 · **Người quyết:** chủ dự án ngày 07/10/2026 giao làm trọn, kể cả backend, "làm theo
đúng prototype" rồi vắng mặt; phiên chính chọn các điểm còn mở theo thứ hạng dưới đây.

## Bối cảnh

Lệnh `/fix-web-admin --menu=van-ban-don-thu`: làm lại toàn bộ view và thao tác của menu theo
prototype, tham khảo bộ hướng dẫn `tmp/web/van-ban/vigov-van-ban-spec/`. Ba agent dò tìm cho thấy
hai tab Đơn thư công dân và Báo cáo chỉ là khung vô hiệu: máy chủ chưa có sổ đơn thư
(`service-documents/so-don-thu-cong-dan` chưa làm). Prototype mặc nhiên giả định một API đơn thư
đầy đủ, và ở nhiều chỗ nó trái với các câu chủ dự án đã chốt 24/09 và 30/09.

## Quyết định

1. **Thứ hạng khi prototype trái với câu đã chốt.** Câu đã chốt và luật 1–13 quyết hành vi nghiệp
   vụ, dữ liệu, pháp lý. Prototype quyết trình bày. Chỗ hai bên trái nhau: làm theo câu đã chốt, vẽ ở
   đúng vị trí và kiểu của prototype. Đây là thứ hạng của skill `ui-ux-prototype-fidelity` §1, không
   phải lựa chọn mới.
2. **Sổ đơn thư** dựng ở `service-documents` (ADR 0039), theo C-list: bộ trạng thái TT 05/2021 (C3,
   phân công là thuộc tính), bốn loại đơn (C4), người gửi tuỳ chọn (C7), số tự sinh không sửa (C6),
   cảnh báo trùng ở máy chủ với dữ liệu trong thân POST (C11), kết quả là văn bản đã ban hành + tóm tắt
   (C10), khoá `petition.create` / `petition.read` (C13/C14).
3. **Hai hạn của đơn để trống.** Bảng SLA của `identity` chưa nhận loại việc đơn thư (C8); ADR 0064
   chốt khiếu nại/tố cáo tính ngày lịch nhưng các điểm #3–#5 còn chờ pháp chế. Tính tạm ở web là trái
   luật 10. Giao diện hiện "Không đặt hạn" — trạng thái có sẵn trong prototype.
4. **Bảo mật người tố cáo và che dữ liệu cá nhân.** Chưa có khoá xem đầy đủ nào cho đơn thư (khoá mới
   chủ dự án đồng ý ở C-list chưa được gieo). Vì vậy đóng an toàn: SĐT/địa chỉ luôn che, không `tel:`,
   ô sửa không điền sẵn giá trị đầy đủ; đơn tố cáo không mang danh tính lẫn trích yếu ở danh sách, báo
   cáo, cảnh báo trùng.
5. **GIẢ ĐỊNH — chờ xác nhận:** khung trang 4 tab (Văn bản đến · Văn bản đi · Đơn thư công dân · Báo
   cáo); tab mặc định Đơn thư công dân như prototype, trừ khi đường dẫn mang lọc từ Tổng quan
   (`metric=…`) thì mở Văn bản đến để lọc ấy không mất; tiêu đề giữ "Văn bản & đơn thư" vì "Đơn thư công
   dân" chỉ tên một trong bốn tab; bỏ nút "Quét & OCR" như prototype (xã yêu cầu bỏ 17/09).
6. **Phần cần máy chủ chưa có** = control vô hiệu "?" đúng vị trí prototype: nhập/xuất Excel, chuyển
   thành nhiệm vụ (hợp đồng documents→petitions chưa có, ADR 0039 #2), gửi Zalo, OCR, tệp đính kèm,
   lọc phạm vi và gợi ý cơ quan ban hành của văn bản đến, trường ghi chú/nguồn văn bản đến.

## Hệ quả

- Prototype có ba chỗ hiển thị trái luật 3 (SĐT đầy đủ, `tel:`, tên người gửi trên URL kiểm trùng) —
  không chép.
- Bộ trạng thái văn bản đến C2 (NĐ 30/2020) cần migration mã trạng thái + tuyến đổi trạng thái; nếu
  chưa dựng kịp lượt này thì dải trạng thái giữ "?".
- Danh sách các câu GIẢ ĐỊNH ở mục 5 nằm trong mục sổ tiến độ của menu để chủ dự án duyệt.

## Sửa đổi

- 08/10/2026 — Mục 3 không còn đúng nguyên văn: ngăn chi tiết đơn thư có ô **"Hạn xử lý" do cán bộ tự đặt** (quyền `petition.create`, chỉ khi đơn chưa kết thúc; "Không đặt" để bỏ hạn; `PATCH /api/v1/citizen-letters/{id}/deadline`), máy chủ ghi vào hạn của giai đoạn hiện tại và web vẫn không tự tính hạn — theo ADR 0079 lô 5 Q18 và mục "Bổ sung 08/10/2026".
- 08/10/2026 (lần 2) — ADR 0084 thay mục 3 (hạn tự điền theo loại đơn), tiêu đề ở mục 5 và phần đơn thư của mục 6 (nhập/xuất Excel, chuyển thành nhiệm vụ nay dựng); thu hẹp C10, đảo C12 của mục 2. Mục 4 đứng nguyên. Xem `0084-don-thu-bam-prototype-08-10.md`.
