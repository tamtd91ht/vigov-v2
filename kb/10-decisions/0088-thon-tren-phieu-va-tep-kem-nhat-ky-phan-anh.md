---
id: 0088-thon-tren-phieu-va-tep-kem-nhat-ky-phan-anh
tier: T1
source: CURATED
owner: domain
derived_from_commit: 8b06345e
expires: null
owns_facts:
  - "thôn trên phiếu phản ánh: dân tuỳ chọn ở Mini App; cán bộ chọn lúc nhập hộ, xác nhận/sửa lúc phân loại có vết; máy chủ kiểm thon_id với identity qua một gRPC MỚI; không suy từ toạ độ; phiếu giữ thôn lúc tiếp nhận (chủ dự án, 09/10/2026)"
  - "tệp kèm trên nhật ký xử lý phản ánh: gỡ = xoá mềm, lý do bắt buộc, khoá feedback.resolve; tệp được đi kèm hành vi đổi trạng thái và gắn với dòng nhật ký của hành vi ấy trong cùng giao dịch (chủ dự án, 09/10/2026)"
  - "không dựng ảnh thu nhỏ trên thẻ danh sách phản ánh, và vì sao (chủ dự án, 09/10/2026)"
---

# 0088. Thôn trên phiếu phản ánh, tệp kèm nhật ký, không ảnh thu nhỏ ở danh sách

**Trạng thái:** đã chốt · **Ngày:** 2026-10-09 · **Người quyết:** chủ dự án, 09/10/2026, trong phiên
chính (`/fix-web-admin --menu=phan-anh-nguoi-dan`), qua phiếu hỏi · **Chưa dựng.**
Nhật ký xử lý luôn nội bộ (không cờ công khai) thuộc **ADR 0041** §Sửa đổi 09/10/2026, không ghi lại ở đây.

## Bối cảnh

Prototype 09/10 của `/phan-anh` có thôn trên phiếu, bảng theo thôn ở tab Báo cáo (ADR 0053 §Sửa đổi
09/10/2026, C5), tệp kèm trên nhật ký và ảnh trên thẻ danh sách. Cột `thon_id` đã có
(`service-petitions/migrations/0004_phieu_phan_anh.sql:265`) nhưng nhập hộ cố ý **không** gửi nó: chưa có
RPC nào của `identity` kiểm một thôn, nên giá trị sẽ đi thẳng từ client vào sổ
(`service-petitions/internal/http/routes.go:932-937`).

## Quyết định

### 1. Thôn trên phiếu

| Điểm | Quyết định |
|---|---|
| Mini App | Dân **tuỳ chọn** chọn thôn |
| Nhập hộ | Cán bộ chọn thôn |
| Phân loại | Cán bộ **xác nhận hoặc sửa** thôn; mỗi lần sửa **có vết**, trước/sau (luật 6 bất biến 5) |
| Kiểm ở máy chủ | `petitions` kiểm `thon_id` thuộc xã và đang dùng, bằng một **gRPC MỚI** của `identity` (luật 2 bất biến 3). Hợp đồng: contract-designer dựng |
| Toạ độ | **Không bao giờ** suy thôn từ toạ độ |
| Theo thời gian | Phiếu **giữ thôn lúc tiếp nhận** (sau khi cán bộ xác nhận ở phân loại). Sửa danh mục thôn về sau không đổi phiếu cũ |
| Thôn ngưng dùng | Chỉ ngưng, không xoá — **ADR 0059 §2 sở hữu**. Phiếu cũ vẫn hiện tên thôn đã ngưng |

**Vì sao kiểm ở máy chủ qua gRPC.** `thon_to_dan_pho` thuộc `identity` (ADR 0024). Một `thon_id` nhận
nguyên từ client mà không ai kiểm là một mã do client tự đặt — có thể của xã khác (luật 1 cấm #2 về
tinh thần). `petitions` không được đọc CSDL của `identity` (luật 2 cấm #2), nên chỉ còn gRPC.

**Vì sao không suy từ toạ độ.** Không có ranh giới thôn số hoá nào trong hệ thống; suy theo điểm gần
nhất sẽ gán sai thôn ở mọi phiếu gần ranh giới — sai mà không ai thấy, và sai ấy đi thẳng vào bảng
theo thôn. Toạ độ dân gửi cũng không phải nơi xảy ra chính xác.

**Vì sao giữ thôn lúc tiếp nhận.** Báo cáo theo thôn của một kỳ đã qua không được đổi khi danh mục đổi;
ghi lại thôn của phiếu cũ là sửa hồ sơ lưu trữ (luật 7 cấm #5).

### 2. Tệp kèm trên nhật ký xử lý

| Điểm | Quyết định |
|---|---|
| Gỡ tệp | **Xoá mềm**, **lý do bắt buộc**, khoá **`feedback.resolve`** |
| Tiền lệ | Gỡ tệp nhiệm vụ, commit `9d4b2684` (ADR 0076 §4b): xoá mềm, lý do bắt buộc, dòng nhật ký không chép tên tệp, vết cùng giao dịch |
| Đi kèm hành vi | Tệp **được** đi kèm một hành vi **đổi trạng thái**, và gắn với **dòng nhật ký của chính hành vi ấy**, **trong cùng giao dịch** (luật 6 bất biến 3) |

**Khác tiền lệ, có chủ ý hoặc chưa hỏi:** ADR 0076 cho cả **người tải lên** gỡ tệp nhiệm vụ. Chủ dự án
chỉ nêu `feedback.resolve` cho phản ánh; người tải lên không có khoá ấy có gỡ được không **chưa hỏi**.

Vì sao cùng giao dịch: tệp gắn vào dòng nhật ký của hành vi là bằng chứng của **chính hành vi ấy**.
Ghi tách thì hành vi có thể thành công mà tệp không gắn — một bước xử lý không có bằng chứng.

### 3. Không ảnh thu nhỏ trên thẻ danh sách

**Không dựng.** Ảnh hiện trường là dữ liệu cá nhân (luật 3). Mỗi lần đọc danh sách sẽ mở ảnh của hàng
chục phiếu cho người chỉ đang lướt — mỗi lần là một lần **tiết lộ** cần ghi vết (luật 6 bất biến 7),
và sổ vết đầy lượt xem không ai rà được. Ảnh xem trong ngăn chi tiết phiếu, từng phiếu một.

## Còn mở khi dựng — hỏi, không tự quyết

| # | Việc | Vì sao không tự chọn |
|---|---|---|
| 1 | Hình dạng gRPC kiểm thôn (một mã hay một lô; trả tên hay chỉ đúng/sai) | Hợp đồng — contract-designer; luật 2 điều kiện dừng #2 |
| 2 | Mini App lấy danh sách thôn ở đâu | Tuyến công dân mới — khai báo quyền (luật 5), trần tần suất (luật 13 #7) |
| 3 | Người tải lên không có `feedback.resolve` có gỡ được tệp mình tải lên không | §2 |

## ĐIỀU KIỆN DỪNG

1. Ghi `thon_id` nhận từ client mà không qua gRPC kiểm của `identity`
2. Suy thôn từ toạ độ, kể cả làm "gợi ý mặc định"
3. Ghi lại thôn của phiếu đã tiếp nhận khi danh mục thôn đổi
4. Xoá cứng tệp kèm, hay gỡ tệp không lý do
5. Thêm ảnh lên thẻ danh sách

→ ADR 0024 (`thon_to_dan_pho` thuộc `identity`) · 0028 (nhập hộ) · 0041 (nhật ký nội bộ) · 0052 (kho tệp) ·
0053 §Sửa đổi 09/10/2026 (bảng theo thôn) · 0059 §2 (ngưng thôn) · 0076 §4b · luật 1 · 2 · 3 · 6 · 7
