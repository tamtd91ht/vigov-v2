---
id: 0050-kenh-cong-dan-theo-kho-yeu-cau
tier: T1
source: CURATED
owner: domain
derived_from_commit: c3e47d0
expires: null
owns_facts:
  - "nguyên tắc chủ dự án 28/09/2026 cho kênh công dân: xung đột thì theo kho yêu cầu, không xung đột thì theo prototype"
  - "lĩnh vực dân chọn khi gửi là lĩnh vực của phiếu; hạn xử lý xong đặt lúc tạo phiếu từ lĩnh vực ấy"
  - "dân chấm 1–5 sao sau khi xử lý; 1–2 sao tự mở lại, không trần số lần, không tính lại hạn; từ 3 sao giữ nguyên trạng thái (vòng 2, 28/09/2026 — thay luật mở lại theo cấu hình của ADR 0008)"
  - "Mini App có công tắc 'Gửi ẩn danh' tuỳ chọn; bật thì không gửi họ tên, số điện thoại"
  - "trường bắt buộc khi dân gửi phản ánh: lĩnh vực, mô tả, ảnh/video, vị trí, người gửi trừ khi ẩn danh"
  - "điểm 1 và 9 áp cả app chung: lĩnh vực là bước đầu, bắt buộc phía client, nhắc lại ở bước xác nhận; máy chủ còn tuỳ chọn (01/10/2026)"
  - "phản ánh 'Thái độ / tác phong cán bộ': luồng riêng, mặc định chỉ Bí thư/Chủ tịch xem, không công khai, xã tắt được"
  - "tra cứu hồ sơ phía dân: số điện thoại đầy đủ + 4 số cuối số hồ sơ"
  - "chatbot OA mức P1; hỏi đáp TTHC, lịch tiếp công dân, khảo sát (P1) và thanh toán (P2) thuộc phạm vi"
  - "người dân thấy bốn nhóm trạng thái, ở cả app chung lẫn app riêng (vòng 4, 28/09/2026); chỉ app riêng xưng 'bà con'"
  - "nháp phản ánh giữ trên máy CHỈ trong app riêng của xã: một khoá, sáu trường, họ tên và số điện thoại lưu cả khi ẩn danh (vòng 4–5, 28/09/2026; sửa 06/10/2026 — có cả ở app chung mở từ QR của xã)"
  - "ở app chung, khoá nháp mang tên miền xã vì một origin phục vụ nhiều xã — ràng buộc người dựng chọn, không phải lời chủ dự án (06/10/2026)"
  - "không có màn phiếu công khai phía dân; kiểm duyệt pending/approved/hidden là việc của cán bộ ở máy chủ; phiếu tác phong cán bộ không bao giờ công khai (vòng 4, 28/09/2026)"
  - "phiên đã xác thực số: màn gửi gộp họ tên + số thành một dòng 'Gửi bằng: <tên> · số Zalo đã xác thực' có nút 'Sửa'; ô số để trống thì service-petitions gắn số đã xác thực của phiên, che và ghi vết như số gõ tay; không bao giờ cho phiếu ẩn danh, không áp ADR 0080, 0083 (chủ dự án, 08/10/2026)"
---

# 0050. Kênh công dân theo kho yêu cầu — xung đột thì theo yêu cầu, còn lại theo prototype

**Trạng thái:** đã chốt · **Ngày:** 2026-09-28 · **Chủ dự án chốt** · **Thay ADR 0049** · **Sửa một
phần ADR 0028** (quyết định E cho kênh Mini App), **ADR 0041** (thêm một bước chuyển), **ADR 0008**
(luật mở lại: từ vòng 2 là 1–2 sao tự mở lại, không trần, không tính lại hạn — điểm 2; cờ `an_danh` của
0008 nay có công tắc phía Mini App — điểm 3)

> 28/09/2026, sau rà soát cùng ngày: điểm 2, 3 sửa cho khớp ADR 0008 và mã; thêm điểm 9, 10 và phạm vi
> điểm 5 dưới dạng CHỜ CHỦ DỰ ÁN; câu bước 3 sửa theo nhãn trải nghiệm.
>
> 28/09/2026, **vòng 2** — chủ dự án: *"8 mục làm theo require nhé"*, rồi *"đồng ý, làm cả 3 việc và 8
> mục luôn"*. "Require" là `../vigov-require` tại `0053854`; chỗ `docs/SRS.md` và
> `docs/spec/05-nghiep-vu.md` nói khác nhau thì theo spec (cụ thể hơn, mới hơn) — chủ dự án duyệt cách
> đọc ấy. Điểm 2, 3, 9, 10 chuyển từ CHỜ sang ĐÃ CHỐT; thêm điểm 11–13.
>
> 28/09/2026, **vòng 4** — chủ dự án: *"3 điểm còn lại cũng theo require nhé"*. Phiên chính nhắc lại
> cách hiểu trước khi dựng — nháp giữ lĩnh vực, mô tả, nơi xảy ra, họ tên, số điện thoại và cờ ẩn danh
> như `store/draft.ts` của kho yêu cầu, chỉ trong app riêng của xã — chủ dự án không phản đối. Phạm vi
> điểm 5, điểm 7, điểm 8 chuyển từ CHỜ sang ĐÃ CHỐT.
>
> 01/10/2026 — chủ dự án chốt **phạm vi điểm 1 và 9 cho app chung**, trước đó chỉ ngầm hiểu: xem đoạn
> "Phạm vi điểm 1 và 9" dưới bảng.
>
> 06/10/2026 — chủ dự án: **nháp (điểm 7) có cả ở app chung** mở từ QR của xã — §*Sửa đổi 06/10/2026*.
>
> 08/10/2026 — chủ dự án: **người gửi đã xác thực số gọn một dòng, máy chủ gắn số của phiên** (điểm 4,
> 9) — §*Sửa đổi 08/10/2026*.

## Bối cảnh

Đối chiếu kho yêu cầu `../vigov-require` (HEAD `0053854`, trùng neo `kb/50-doi-chieu/neo.json`) với
app riêng của xã cho ra sáu chỗ kho yêu cầu làm NGƯỢC một quyết định đã chốt ở đây (bảng đầy đủ: sổ
tiến độ `citizen-app/api-con-thieu-app-rieng`). Chủ dự án trả lời: *"theo require nhé, cái nào không
xung đột với require thì làm theo prototype"*.

## Quyết định

| # | Điểm | Nay | Nguồn yêu cầu | Thay gì |
|---|---|---|---|---|
| 1 | Lĩnh vực và hạn | Lĩnh vực dân chọn là **lĩnh vực của phiếu**; hạn xử lý xong đặt **lúc tạo phiếu** từ lĩnh vực ấy; cán bộ đổi lĩnh vực ở bước phân loại theo luật đổi hạn hiện hành (ADR 0027 quyết định C) | `apps/api/.../feedback/service.py:232,272`; `NewFeedbackPage.tsx` "Dự kiến xử lý xong trước …" | ADR 0049 (bỏ); ADR 0028 quyết định E cho kênh Mini App |
| 2 | Đánh giá | Dân chấm **1–5 sao** kèm nhận xét tuỳ ý khi phiếu ở "đã xử lý" / "chờ dân xác nhận". Phiếu mở lại rồi xử lý xong lần nữa thì dân **chấm lại được**, lần mới thay lần cũ (`service.py:813`; mã: `DanhGia.sau_lan_mo_lai`). **Vòng 2, 28/09/2026 — ĐÃ CHỐT:** (i) chấm **từ 3 sao** ở "chờ dân xác nhận" thì ghi nhận điểm, **giữ nguyên trạng thái** (`service.py:822-823`; tài liệu không nói) — phiếu chờ cán bộ đóng; (ii) chấm **1 hoặc 2 sao** thì phiếu **tự mở lại** về đang xử lý (`05-nghiep-vu.md:206`, `SRS.md:324`), **không trần số lần**, **không tính lại hạn** (`service.py:817-821` chỉ đổi trạng thái, cộng `reopen_count`, xoá `closed_at`). Ngưỡng là con số cố định "1 hoặc 2 sao", không phải cấu hình. Dòng này **thay** quyết định 27/09 "1–2 sao vào hàng lãnh đạo xem, KHÔNG tự mở lại" (`kb/90-ephemeral/ban-giao-phien.md:76`, sổ `service-petitions/phan-anh-tuyen-cong-dan-con-thieu`) và **thay** giá trị đã chốt của `nguong_sao_mo_lai`, `so_lan_mo_lai_toi_da`, `tinh_lai_han_khi_mo_lai` trong ADR 0008:50-53. `SAO_MO_LAI = 2` chỉ sống trong bản trải nghiệm (phiếu trong bộ nhớ), bỏ khi có đường máy chủ. Giao diện dân **không nói ngưỡng** | `service.py:804-840`; `SRS.md:324` (M4.3.7); `05-nghiep-vu.md:206`; `RatingBlock.tsx` | ADR 0041: thêm một bước chuyển do dân gây ra (kèm thông báo); ADR 0008 phần mở lại; quyết định 27/09 |
| 3 | Ẩn danh | **Vòng 2, 28/09/2026 — ĐÃ CHỐT:** trả lại công tắc **"Gửi ẩn danh"** tuỳ chọn (`SRS.md:308` *"Tuỳ chọn: gửi ẩn danh"*). Bật thì Mini App **không gửi họ tên, không gửi số điện thoại**; cán bộ không thấy cả hai (cờ `an_danh` phía máy chủ, `service-petitions/internal/http/phieu_phan_anh.go:208-220`). Thay cách cũ "để trống ô tên là giấu tên" của prototype (`NewFeedbackPage.tsx:376-400`) — cách ấy yếu hơn vì số điện thoại (đã che) vẫn tới cán bộ. Mã: `kiemNhapPhieu` bỏ yêu cầu người gửi khi `an_danh` (`citizen-app/src/cong-dan/man/trai-nghiem.ts:213-224`); công tắc ở `PhanAnhAppXa.tsx:561-577` | `SRS.md:308`; `schemas.py:116` | ADR 0008 phía Mini App: nay có công tắc cho cờ `an_danh` đã chốt ở đó |
| 4 | Số điện thoại | Lấy từ tài khoản Zalo (xin quyền → máy chủ đổi mã ra số); sửa được | `NewFeedbackPage.tsx:90-116`; `SRS.md:432` | — (cần máy chủ đổi mã; trước đó dân tự gõ) |
| 5 | Nhãn trạng thái | Người dân thấy **bốn nhóm**: Đã tiếp nhận · Đang xử lý · Đã xử lý xong · Đã đóng; dòng thời gian dùng nhãn từng bước của prototype. **Vòng 4, 28/09/2026 — ĐÃ CHỐT phạm vi:** áp cho **cả app chung** (`AppChung`), không chỉ app riêng. Một bảng duy nhất: `citizen-app/src/cong-dan/man/status-groups.ts:22-32` (`groupOf`, `STATUS_GROUP_LABEL`, `STEP_LABEL`); app chung đọc qua `nhanTrangThai` (`noi-dung.ts:51-54`); `trai-nghiem.ts:169-175` giữ `nhomCua`/`NHAN_NHOM`/`NHAN_BUOC` làm tên cũ trỏ về bảng ấy. Mã lạ: `groupOf` trả `null`, app chung hiện câu trung tính `TRANG_THAI_CHUA_CO_NHAN` (`noi-dung.ts:47-48`) thay vì đoán nhóm | `StatusChip.tsx`, `feedback-adapter.ts:79-101` | Chỉ phía người dân; cán bộ vẫn chín trạng thái (ADR 0027); mã trên dây không đổi |
| 6 | Xưng hô | "bà con" | toàn bộ prototype | `noi-dung.ts` phía công dân |
| 7 | Nháp | **Vòng 4, 28/09/2026 — ĐÃ CHỐT, ĐÃ DỰNG, chỉ app riêng.** **Phạm vi "chỉ app riêng" thay 06/10/2026 — §*Sửa đổi 06/10/2026*.** Phản ánh đang soạn giữ trong `localStorage`, một khoá `vigov.feedback.draft.v1` như kho yêu cầu. Tệp duy nhất được chạm bộ nhớ máy: `citizen-app/src/commune-app/feedback-draft-store.ts` — ghi đúng sáu trường của `NhapPhieu`, dựng lại từng trường (`:45-55`); họ tên, số điện thoại **lưu cả khi bật "Gửi ẩn danh"**, như `store/draft.ts` của kho yêu cầu (28/09/2026 vòng 5, chủ dự án: “theo require, ẩn danh cũng lưu họ tên”; `toStored`) — ẩn danh quyết thứ cán bộ thấy lúc GỬI, không quyết thứ nằm trong nháp; dựng app chung (`XA_CO_DINH === null`) thì không mở bộ nhớ nào (`:128-130`). Chỉ `AppRieng` tiêm nó (`App.tsx:212-216`). Mở màn gửi mà có nháp thì hỏi "Tiếp tục" / "Bỏ nháp" (`PhanAnhAppXa.tsx:487-491`); xoá khi gửi, bỏ nháp, huỷ (`:523, 529, 542`). Hai dây bẫy nay cấm mọi lưu trữ **trừ đúng tệp ấy**: `phase1-collects-nothing.test.ts:383-386`, `ranh-gioi-hai-nua.test.ts:403, 439` (§3b). Khác kho yêu cầu: không giữ ảnh (`photoSeeds`) vì app chưa tải được ảnh; không `savedAt` | `store/draft.ts:11`, `types/index.ts:71-81`, `NewFeedbackPage.tsx:426-453` | Dòng chính sách quyền riêng tư app riêng của ADR 0047 (`0047:252`) — xem Cái giá |
| 8 | Phiếu công khai | **Vòng 4, 28/09/2026 — ĐÃ CHỐT theo kho yêu cầu:** **không có màn bảng công khai** phía dân — prototype Mini App không có màn nào như vậy (không một chỗ `moderation` trong `apps/miniapp/src`). Thứ kho yêu cầu có là **kiểm duyệt của cán bộ ở máy chủ**: trạng thái kiểm duyệt riêng `pending`/`approved`/`hidden` (`05-nghiep-vu.md:192-193`), tuyến cán bộ `POST /{feedback_id}/moderation` (`router.py:349-361`), duyệt `approved` bị từ chối nếu lĩnh vực thuộc loại không công khai (`service.py:776-782`), phiếu loại ấy sinh ra đã `hidden` (`service.py:301-307`); SRS M4.3.1 mức **P0** (`SRS.md:318`); phiếu tác phong cán bộ **không bao giờ** công khai (`05-nghiep-vu.md:204`). Mini App: không dựng màn. Máy chủ: nợ ba thứ ấy — xem Cái giá | `05-nghiep-vu.md:192-193, 204`; `SRS.md:318`; `router.py:349-361`; `service.py:301-307, 776-782` | — |
| 9 | Trường bắt buộc | **Vòng 2, 28/09/2026 — ĐÃ CHỐT theo `SRS.md:308`:** lĩnh vực, mô tả, ảnh/video (≤5 tệp), vị trí trên bản đồ, người gửi (họ tên) — trừ khi gửi ẩn danh. Thay cách của prototype (chỉ bắt buộc mô tả, `NewFeedbackPage.tsx:412-415`) và của API kho yêu cầu (`field_code` + `content`, `schemas.py:106-107`). Mã: `kiemNhapPhieu` bắt người gửi khi không ẩn danh (`trai-nghiem.ts:221`), lĩnh vực do bước 1 chặn; màn gửi hiện ảnh và vị trí là "bắt buộc — sắp có" (`PhanAnhAppXa.tsx:553-558`). **Cái giá:** ứng dụng chưa tải được ảnh (chưa có kho tệp nối vào) và chưa đổi được vị trí ra toạ độ, nên bản trải nghiệm **không chặn nút gửi** vì hai ô ấy — chặn thì không gửi được phiếu nào. Máy chủ **phải kiểm đủ năm ô** khi kho tệp và vị trí đã có | `SRS.md:308` | Prototype `NewFeedbackPage.tsx:412-415` |
| 10 | "Thái độ / tác phong cán bộ" lúc gửi | **Vòng 2, 28/09/2026 — ĐÃ CHỐT:** dân **chọn được** lĩnh vực này (`STAFF_CONDUCT_FIELD`, `trai-nghiem.ts:102`, theo `SRS.md:310`). Phiếu loại này đi **luồng riêng**, mặc định **chỉ Bí thư/Chủ tịch xem**, **không bao giờ công khai**, **xã tắt được** tính năng (`SRS.md:325` M4.3.8; `SRS.md:591` R-05; `05-nghiep-vu.md:204`). Màn gửi báo dân chỉ lãnh đạo xã xem (`PhanAnhAppXa.tsx:547-552`). **Còn thiếu để dựng:** (a) một khoá quyền chỉ-lãnh-đạo mà bảng `quyen` chưa có — theo luật 5 bất biến 3c đây là **phát hiện cho câu mở #27**, không phải một `INSERT` mới; (b) cờ bật/tắt theo xã. Xã tắt thì màn gửi không được hiện lĩnh vực này. ADR 0041:63-66 (lời báo cho dân với lĩnh vực `can-bo`) giữ nguyên | `SRS.md:310, 325, 591`; `05-nghiep-vu.md:204` | — |
| 11 | Tra cứu hồ sơ TTHC (M6.1.3) | **Vòng 2, 28/09/2026 — ĐÃ CHỐT theo spec:** dân nhập **số điện thoại đầy đủ + 4 số cuối số hồ sơ**, cả hai bắt buộc; sai mảnh nào cũng trả **cùng một câu "không thấy"**; giới hạn **20 lần tra / 10 phút / một địa chỉ**; kết quả là **danh sách hồ sơ** của người ấy (`05-nghiep-vu.md:148-156`). "Nhập mã biên nhận" của `SRS.md:407` bị spec thay. Màn: `TraCuuHoSoXa` (`citizen-app/src/cong-dan/man/TienIchAppXa.tsx:29`). **Chưa có:** hệ thống một cửa nào nối vào, và service sở hữu thực thể hồ sơ (luật 2 điều kiện dừng #1) | `05-nghiep-vu.md:148-156` | `SRS.md:407` |
| 12 | Chatbot OA | **Vòng 2, 28/09/2026 — ĐÃ CHỐT mức P1** (`SRS.md:434`, mục riêng của kênh Zalo). Chạy trên **OA của từng xã** — phía `service-comms` (ADR 0018), **không** phải Mini App, **không** phải `vihat-miniapp` | `SRS.md:434` | — |
| 13 | Phạm vi M6.1 còn lại | **Vòng 2, 28/09/2026 — ĐÃ CHỐT thuộc phạm vi**, đúng mức ưu tiên của `SRS.md:412-417`: M6.1.8 hỏi đáp TTHC (P1), M6.1.10 lịch tiếp công dân + đăng ký lịch gặp (P1), M6.1.11 khảo sát – lấy ý kiến (P1), M6.1.13 thanh toán phí – lệ phí (P2). **Chưa có service nào sở hữu** cả bốn — luật 2 điều kiện dừng #1: tới lượt thì contract-designer đề xuất, chủ dự án quyết | `SRS.md:412-417` | — |

Phạm vi điểm 5: **ĐÃ CHỐT vòng 4** — cả app chung lẫn app riêng (dòng 5 ở bảng trên).

Phạm vi điểm 1 và 9: **ĐÃ CHỐT 01/10/2026** (chủ dự án) — áp cho **cả app chung** ViHAT
(`GuiPhanAnhScreen`), không chỉ app riêng: (a) bước chọn lĩnh vực có ở app chung và lĩnh vực **bắt
buộc phía client** như app riêng; (b) đó là **bước ĐẦU TIÊN** của luồng gửi, như app riêng và
`NewFeedbackPage` của kho yêu cầu; (c) bước xác nhận **nhắc lại lĩnh vực đã chọn**. Máy chủ **vẫn để
tuỳ chọn** — bắt buộc phía máy chủ là bước hoãn, theo dõi ở sổ `service-petitions/linh-vuc-cho-dan-tang-2`.

Chỗ không xung đột: theo prototype (bước 2 là bước gửi; bước 3 là màn kết quả — bản trải nghiệm ghi
"Đã lưu phản ánh (bản trải nghiệm)" vì phiếu chỉ nằm trong bộ nhớ máy, **chưa gửi tới xã**; trang chủ 2
phiếu · 2 tin; dòng thời gian chỉ các bước đã qua; tìm danh bạ bỏ dấu và theo số). **Chưa làm:** chữ "xã/phường" lấy từ
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
  cho kênh Mini App, tuyến danh mục lĩnh vực, tuyến chấm sao + tự mở lại ở 1–2 sao (điểm 2), tuyến đổi
  mã số điện thoại, nhận `an_danh` từ Mini App (điểm 3), kiểm đủ năm trường bắt buộc (điểm 9), luồng
  tác phong cán bộ (điểm 10), kiểm duyệt `pending`/`approved`/`hidden` + tuyến cán bộ + luật phiếu tác
  phong cán bộ không bao giờ tới `approved` (điểm 8). Bảng chuyển trạng thái của máy chủ **không có cạnh `da-xu-ly → dang-xu-ly`**
  (`service-petitions/internal/domain/phieu_phan_anh.go:74-84`, chỉ `cho-dan-xac-nhan` và `da-dong` về
  được `dang-xu-ly`) — cần thêm nếu chấm thấp ở "đã xử lý" mở lại phiếu. Sổ:
  `service-petitions/may-chu-no-adr-0050`.
- **Điểm 2 — mở lại không tính lại hạn:** luật 10 bất biến 2 vẫn đứng, vì hạn đặt lúc tạo phiếu **không
  bao giờ bị dời**. Hệ quả: phiếu mở lại thường đã **quá hạn ngay lúc mở lại**, và số liệu quá hạn gửi
  lên lãnh đạo sẽ phản ánh đúng điều đó. **Không trần số lần** nên một phiếu có thể quay vòng mãi; theo
  dõi bằng `so_lan_mo_lai`. Chấm từ 3 sao **không đóng** phiếu — phiếu nằm ở "chờ dân xác nhận" tới khi
  cán bộ đóng.
- **Điểm 2 — cờ `cho_phep_mo_lai` (ADR 0008:50): ĐÃ CHỐT 28/09/2026 (vòng 3, chủ dự án: "theo require, không cho xã tắt mở lại").** Mở lại khi chấm 1–2 sao
  là luật cố định của mọi xã, như kho yêu cầu (`service.py:817-821` không có công tắc). Cờ ấy KHÔNG
  được dựng; máy chủ không đọc cấu hình nào để quyết có mở lại hay không.
- **Điểm 9 — năm ô bắt buộc mà hai ô chưa làm được:** bản trải nghiệm hiện "bắt buộc — sắp có" và vẫn cho
  gửi. Ngày máy chủ bắt đủ năm ô mà ứng dụng chưa có ảnh và vị trí thì **không phiếu nào gửi được** — hai
  việc phải lên cùng lúc.
- **Điểm 10 — chờ câu mở #27:** chưa có khoá quyền chỉ-lãnh-đạo thì không dựng được luồng riêng; tới lúc
  đó phiếu tác phong cán bộ chỉ nằm trong bản trải nghiệm.
- **Điểm 11, 13 — chưa có chủ sở hữu:** hồ sơ một cửa, hỏi đáp TTHC, lịch tiếp công dân, khảo sát, thanh
  toán đều chưa có service sở hữu (luật 2 điều kiện dừng #1). Tra cứu hồ sơ không cần phiên là điều kiện
  dừng luật 4 #2 — hỏi khi dựng máy chủ.
- **Còn CHỜ CHỦ DỰ ÁN** cho mảng phản ánh: **không còn điểm nào** sau vòng 4 (28/09/2026). Những gì còn
  treo ở đây là việc chưa dựng, hoặc câu mở đã có số (#27) — không phải câu hỏi chưa được trả lời.
- **Điểm 5 — app riêng còn một chỗ đoán nhóm:** `nhomCua` (`trai-nghiem.ts:169-171`) cho mã lạ rơi vào
  "Đã đóng", khác app chung (câu trung tính). Hôm nay không tới được vì phiếu app riêng sinh trong bộ nhớ
  với một trong chín mã; ngày app riêng đọc phiếu từ máy chủ phải đổi sang `groupOf`, nếu không một phiếu
  đang mở có thể hiện "Đã đóng" với dân.
- **Điểm 7 — nháp có họ tên và số điện thoại trên máy, vì sao không phá lời hứa nào:** app riêng là một
  Zalo App ID riêng, tức một origin riêng, và **chưa có chính sách quyền riêng tư** (ADR 0047:252). Câu
  "không lưu bất kỳ dữ liệu nào của bạn xuống máy" (`citizen-app/src/content/chinh-sach-rieng-tu.ts:229`)
  là của app chung ViHAT và vẫn đúng: app chung không nhận kho nháp, và kho mặc định không mở bộ nhớ khi
  dựng app chung. **Cái giá phải trả khi viết chính sách của app riêng để nộp Zalo duyệt:** phải khai
  nháp này — có họ tên và số điện thoại, tên khoá, và lúc nào nó bị xoá (gửi xong, "Bỏ nháp", huỷ). Thiết
  bị cho mượn thì người sau thấy nháp của người trước. **Nháp không có hạn tự xoá** — ĐÃ CHỐT 28/09/2026 vòng 6
  (chủ dự án: “nháp không có hạn tự xoá, theo require”; `store/draft.ts` không có hạn): nháp chỉ mất
  khi gửi xong, "Bỏ nháp" hoặc huỷ. Khoá **không** mang tiền tố `t:<tenant>` vì một app riêng = một origin = một xã: không có
  hai xã chung một bộ nhớ.
- **Điểm 8 — không có bảng công khai, nhưng máy chủ vẫn nợ kiểm duyệt:** SRS xếp M4.3.1 mức P0
  (`SRS.md:318`). Sổ: `service-petitions/may-chu-no-adr-0050`.

## Sửa đổi 06/10/2026 — nháp có cả ở app chung

Mục này ghi thêm, không sửa phần trên; mục này thắng khi nói khác. **Người quyết:** chủ dự án,
06/10/2026, trong phiên chính. **Chưa dựng.** Lý do (app chung ViHAT được mượn để hiện trọn giao diện
của xã vì app riêng chưa được Zalo duyệt nhanh): ADR 0047 §*Sửa đổi 06/10/2026* sở hữu.

| # | Điểm | Chốt |
|---|---|---|
| 1 | Phạm vi điểm 7 | Nháp phản ánh giữ trên máy ở **cả app chung** khi mở từ QR của xã. **Thay** chữ *"chỉ app riêng"* của điểm 7 và câu *"app chung không nhận kho nháp"* ở §*Cái giá* điểm 7 |
| 2 | Khoá nháp ở app chung | **Mang tên miền xã** (tên miền trên QR). **Ràng buộc do người dựng chọn, không phải lời chủ dự án** — ghi ra để không ai tưởng là đã được duyệt |
| 3 | Giữ nguyên | Sáu trường của `NhapPhieu`, không ảnh, không hạn tự xoá, xoá khi gửi / "Bỏ nháp" / huỷ, đúng một tệp chạm bộ nhớ máy (`feedback-draft-store.ts`), không lưu mã phiên — ĐIỀU KIỆN DỪNG #3 |

**Vì sao khoá phải mang tên miền xã (luật 1):** lý do không cần tiền tố ở §*Cái giá* điểm 7 — *"một app
riêng = một origin = một xã"* — **không đúng với app chung**: một App ID, một origin, phục vụ **nhiều
xã**. Một khoá duy nhất thì nháp viết cho xã A (có họ tên, số điện thoại) hiện ra trong màn gửi của xã
B — rò dữ liệu giữa hai xã trên chính máy công dân, và phiếu có thể gửi nhầm cơ quan. Không dùng
`t:<tenant_id>` vì client không bao giờ nhận ULID của xã (ADR 0047 câu 6, điều kiện dừng #5); tên miền
là định danh xã duy nhất client có. Khoá chỉ nằm trên máy công dân, nên không phải *"tham chiếu xã"* mà
ADR 0047 điều kiện dừng #1 cấm (phiên, xã đã nhớ, vết, bản ghi nghiệp vụ).

**Cái giá:**

- Chính sách quyền riêng tư của app chung phải khai nháp này (họ tên, số điện thoại, tên khoá, lúc xoá)
  — câu *"không lưu bất kỳ dữ liệu nào của bạn xuống máy"* (`chinh-sach-rieng-tu.ts:229`) sai với app
  chung từ khi dựng. Sổ `citizen-app/chinh-sach-dung-voi-ban-dung`; không sửa ở đây.
- Hai dây bẫy hôm nay cấm app chung mở bộ nhớ (`phase1-collects-nothing.test.ts`,
  `ranh-gioi-hai-nua.test.ts` §3b — điểm 7) phải đổi **cùng lượt dựng**, sao cho vẫn cấm mọi thứ khác.
- Tên miền cũ trỏ sang xã kế thừa sau sáp nhập (ADR 0047 câu 4): nháp viết dưới tên miền cũ hiện trong
  giao diện của xã kế thừa. Chủ dự án **chưa nói** về ca này — không tự chọn.

## Sửa đổi 08/10/2026 — người gửi đã xác thực số: một dòng, máy chủ gắn số của phiên

Mục này ghi thêm, không sửa phần trên; mục này thắng khi nói khác. **Người quyết:** chủ dự án,
08/10/2026, trong phiên chính, qua phiếu hỏi. **Đang dựng** (phiên khác) — sổ
`citizen-app/gui-phan-anh-nguoi-gui-da-xac-thuc` và `service-petitions/gan-so-da-xac-thuc-cua-phien`.

| # | Điểm | Chốt |
|---|---|---|
| 1 | Màn gửi, công dân có phiên **đã xác thực số** | Hai ô "Họ và tên" và "Số điện thoại" **gộp thành một dòng**: "Gửi bằng: <tên> · số Zalo đã xác thực", kèm nút **"Sửa"** |
| 2 | Máy chủ gắn số | Phiếu tới từ phiên đã xác thực số mà **ô số điện thoại để trống** → `service-petitions` gắn **số đã xác thực của phiên** vào phiếu. Số ấy được đối xử **y như số gõ tay**: che khi ra API (luật 3 bất biến 3), che trong phần trước/sau của vết (luật 6 bất biến 5), không vào log (luật 3 bất biến 1) |
| 3 | Không bao giờ gắn | (a) Phiếu **ẩn danh** — điểm 3: bật ẩn danh thì không gửi số, nên máy chủ cũng **không tự thêm**; (b) phiếu từ phiên **chưa xác thực số** — ADR 0080, phiên không có số đã xác thực để gắn; (c) phiếu **không tài khoản** — ADR 0083, không có phiên. **ADR 0080 và 0083 không đổi** |
| 4 | Giữ nguyên | Điểm 9: người gửi (họ tên) vẫn bắt buộc khi không ẩn danh. Chủ phiếu vẫn là công dân **của phiên** (luật 4 bất biến 2) — số gắn vào là **số liên hệ** trên phiếu, không phải nguồn danh tính. Ô số **có chữ** thì máy chủ dùng chữ ấy như trước; quyết định này chỉ nói ca ô **trống** |

**Thay gì:** điểm 4 ghi số *"lấy từ tài khoản Zalo … sửa được"*, cột nguồn ghi *"trước đó dân tự gõ"*; nay
với phiên đã xác thực số, dân **không phải gõ lại** — số lấy từ phiên ở máy chủ, "Sửa" vẫn còn. Điểm 9 vẫn
đứng; chỉ cách **hiện** hai ô người gửi đổi.

**Vì sao máy chủ gắn, client không gửi lại số** (lý do của người ghi, không phải lời chủ dự án): số đã xác
thực nằm ở phiên phía máy chủ. Client tự điền rồi gửi lại thì số trên phiếu là chữ client nói, không còn là
số máy chủ đã xác minh — đúng thứ luật 4 bất biến 2 cấm cho danh tính. Gắn ở máy chủ thì nhãn "số Zalo đã
xác thực" trên màn là thật.

## ĐIỀU KIỆN DỪNG

1. Tính hạn ở client, ở bất kỳ đâu ngoài `identity`
2. Dựng màn phiếu công khai phía dân (kho yêu cầu không có màn này), hoặc để phiếu tác phong cán bộ tới
   `approved`
3. Lưu trên máy vượt ranh giới điểm 7 — bất kỳ vế nào sau đây:
   - lưu **mã phiên** (session token) ở bất kỳ đâu;
   - lưu **bất cứ thứ gì** trên máy trong **app chung** — **từ 06/10/2026 trừ đúng nháp của điểm 7, khoá
     mang tên miền xã** (§*Sửa đổi 06/10/2026*); một nháp app chung có khoá **không** mang tên miền xã
     cũng là vượt ranh giới;
   - lưu **nhiều hơn sáu trường** nháp của `NhapPhieu`;
   - lưu ở tệp nào khác ngoài `citizen-app/src/commune-app/feedback-draft-store.ts`
4. Chép cách lấy xã từ header client hoặc mã phiếu tuần tự của kho yêu cầu

→ ADR 0027 · 0028 · 0041 · 0008 · 0018 · 0047 · 0049 (bị thay) · câu mở #8, #27 · luật 1, 2, 3, 4, 5, 10
