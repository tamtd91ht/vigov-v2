---
id: 0050-kenh-cong-dan-theo-kho-yeu-cau
tier: T1
source: CURATED
owner: domain
derived_from_commit: ed34ca3
expires: null
owns_facts:
  - "nguyên tắc chủ dự án 28/09/2026 cho kênh công dân: xung đột thì theo kho yêu cầu, không xung đột thì theo prototype"
  - "lĩnh vực dân chọn khi gửi là lĩnh vực của phiếu; hạn xử lý xong đặt lúc tạo phiếu từ lĩnh vực ấy"
  - "dân chấm 1–5 sao sau khi xử lý; 1–2 sao mở lại phiếu"
  - "Mini App không có công tắc ẩn danh; để trống ô tên là giấu tên"
  - "người dân thấy bốn nhóm trạng thái và được gọi là 'bà con'"
---

# 0050. Kênh công dân theo kho yêu cầu — xung đột thì theo yêu cầu, còn lại theo prototype

**Trạng thái:** đã chốt · **Ngày:** 2026-09-28 · **Chủ dự án chốt** · **Thay ADR 0049** · **Sửa một
phần ADR 0028** (quyết định E cho kênh Mini App), **ADR 0041** (thêm một bước chuyển), **ADR 0008**
(công tắc ẩn danh phía Mini App)

## Bối cảnh

Đối chiếu kho yêu cầu `../vigov-require` (HEAD `0053854`, trùng neo `kb/50-doi-chieu/neo.json`) với
app riêng của xã cho ra sáu chỗ kho yêu cầu làm NGƯỢC một quyết định đã chốt ở đây (bảng đầy đủ: sổ
tiến độ `citizen-app/api-con-thieu-app-rieng`). Chủ dự án trả lời: *"theo require nhé, cái nào không
xung đột với require thì làm theo prototype"*.

## Quyết định

| # | Điểm | Nay | Nguồn yêu cầu | Thay gì |
|---|---|---|---|---|
| 1 | Lĩnh vực và hạn | Lĩnh vực dân chọn là **lĩnh vực của phiếu**; hạn xử lý xong đặt **lúc tạo phiếu** từ lĩnh vực ấy; cán bộ đổi lĩnh vực ở bước phân loại theo luật đổi hạn hiện hành (ADR 0027 quyết định C) | `apps/api/.../feedback/service.py:232,272`; `NewFeedbackPage.tsx` "Dự kiến xử lý xong trước …" | ADR 0049 (bỏ); ADR 0028 quyết định E cho kênh Mini App |
| 2 | Đánh giá | Dân chấm **1–5 sao** kèm nhận xét tuỳ ý khi phiếu ở "đã xử lý" / "chờ dân xác nhận"; chấm là xác nhận; **1–2 sao mở lại phiếu** (về đang xử lý, đếm số lần mở lại, báo người giữ phiếu) | `service.py:804-840`; `05-nghiep-vu.md:206`; `RatingBlock.tsx` | ADR 0041: thêm một bước chuyển do dân gây ra (kèm thông báo) |
| 3 | Ẩn danh | Mini App **không có công tắc ẩn danh**; để trống ô tên là giấu tên ("Để trống nếu bà con muốn giấu tên") | `NewFeedbackPage.tsx:376-400` | ADR 0008 phía Mini App; máy chủ vẫn giữ khả năng ẩn danh cho kênh khác |
| 4 | Số điện thoại | Lấy từ tài khoản Zalo (xin quyền → máy chủ đổi mã ra số); sửa được | `NewFeedbackPage.tsx:90-116`; `SRS.md:432` | — (cần máy chủ đổi mã; trước đó dân tự gõ) |
| 5 | Nhãn trạng thái | Người dân thấy **bốn nhóm**: Đã tiếp nhận · Đang xử lý · Đã xử lý xong · Đã đóng; dòng thời gian dùng nhãn từng bước của prototype | `StatusChip.tsx`, `feedback-adapter.ts:79-101` | Chỉ phía người dân; cán bộ vẫn chín trạng thái (ADR 0027) |
| 6 | Xưng hô | "bà con" | toàn bộ prototype | `noi-dung.ts` phía công dân |
| 7 | Nháp | Theo yêu cầu: phản ánh đang soạn được giữ trên máy. **CHƯA LÀM** — xem Cái giá | `store/draft.ts`, `NewFeedbackPage.tsx:426-453` | Chờ: sửa chính sách quyền riêng tư + nới hai dây bẫy lưu trữ |
| 8 | Phiếu công khai | Yêu cầu ghi nhận: phiếu được kiểm duyệt thì hiện công khai (`pending`/`approved`/`hidden`), phiếu về tác phong cán bộ **không bao giờ** công khai. **Chưa dựng màn** — prototype không có màn này | `05-nghiep-vu.md:192-193, 204` | Chạm luật 4 bất biến 6 và luật 3 — cần thiết kế riêng trước khi dựng |

Chỗ không xung đột: theo prototype (gửi ngay ở bước 2, bước 3 là màn "Đã gửi"; trang chủ 2 phiếu · 2 tin;
dòng thời gian chỉ các bước đã qua; tìm danh bạ bỏ dấu và theo số). **Chưa làm:** chữ "xã/phường" lấy từ
tên đơn vị — chuỗi chữ hôm nay viết cứng "xã", đúng với Thăng Bình; phải làm trước khi một **phường**
nhận app riêng.

Phạm vi điểm 6: chỉ app riêng của xã (`XA_TN`, `XA_PA`). Màn công dân của app chung vẫn xưng "bạn".

**KHÔNG theo kho yêu cầu, vì không phải nghiệp vụ mà là lỗ an ninh** (luật cứng, không thuộc câu hỏi
của chủ dự án): xã lấy từ header client và định danh `X-Citizen-Id` (luật 1 cấm #2, luật 4 bất biến 2);
mã phiếu tuần tự `PA-2026-0004` (luật 4 bất biến 4).

## Cái giá — đừng giấu

- **Điểm 1 mở lại lỗ khai thác ADR 0028 đã chặn:** dân chọn lĩnh vực khẩn nhất để được hạn gấp, và vì
  hạn chỉ được rút ngắn nên cán bộ không kéo dài được. Chủ dự án chấp nhận theo yêu cầu của khách. Theo
  dõi bằng số liệu: tỷ lệ phiếu cán bộ đổi lĩnh vực ở bước phân loại.
- **Hạn vẫn chỉ `identity` đếm** (luật 10 cấm #2): Mini App không tự tính "Dự kiến xử lý xong trước …";
  nó hiện đúng mốc máy chủ trả. Bản trải nghiệm (phiếu trong bộ nhớ) nói thẳng là chưa tính hạn.
- **Máy chủ phải đổi**: nhận `field` khi dân gửi (hôm nay bị từ chối 400), đặt cả hai hạn lúc tạo phiếu
  cho kênh Mini App, tuyến danh mục lĩnh vực, tuyến chấm sao + luật mở lại, tuyến đổi mã số điện thoại.
- **Nháp trên máy (điểm 7) CHƯA LÀM, có chủ đích.** Hai dây bẫy (`phase1-collects-nothing.test.ts`,
  `ranh-gioi-hai-nua.test.ts` §3b) cấm MỌI lưu trữ trên máy vì hai nửa dùng chung một origin và thiết bị
  cho mượn được; câu "không lưu lại" trong chính sách quyền riêng tư đã khai với Zalo đứng được nhờ chúng.
  Làm điểm 7 là SỬA chính sách ấy trước (nộp lại), rồi mới mở một ngoại lệ hẹp: một khoá, chỉ lĩnh vực ·
  nội dung · nơi xảy ra, không họ tên, không số điện thoại, xoá khi gửi. Cần chủ dự án đồng ý cái giá ấy.

## ĐIỀU KIỆN DỪNG

1. Tính hạn ở client, ở bất kỳ đâu ngoài `identity`
2. Dựng màn phiếu công khai khi chưa có thiết kế che dữ liệu cá nhân và loại trừ phiếu tác phong cán bộ
3. Lưu họ tên / số điện thoại / mã phiên vào bộ nhớ máy
4. Chép cách lấy xã từ header client hoặc mã phiếu tuần tự của kho yêu cầu

→ ADR 0027 · 0028 · 0041 · 0008 · 0049 (bị thay) · luật 1, 3, 4, 10
