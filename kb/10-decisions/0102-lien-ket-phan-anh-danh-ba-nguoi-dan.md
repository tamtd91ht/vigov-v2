---
id: 0102-lien-ket-phan-anh-danh-ba-nguoi-dan
tier: T1
source: CURATED
owner: architecture
derived_from_commit: 76dd2672
expires: null
owns_facts:
  - "liên kết phản ánh ↔ danh bạ người dân là SUY RA lúc xem theo khoá (tenant_id, SĐT đã chuẩn hoá); không bảng nào lưu mã của bảng kia; phụ thuộc một chiều petitions → identity qua gRPC (chủ dự án 17/10/2026)"
  - "phản ánh ẩn danh bị loại khỏi mọi đối chiếu với danh bạ, cả hai chiều (chủ dự án 17/10/2026)"
  - "nút Đồng bộ danh bạ ở phản ánh cần feedback.unmask + citizen.update; Xem phản ánh ở danh bạ cần citizen.read + feedback.read; không khoá mới (chủ dự án 17/10/2026)"
---

# 0102. Liên kết phản ánh ↔ danh bạ người dân: suy ra, không lưu

**Trạng thái:** đã chốt · **Ngày:** 2026-10-17 · **Người quyết:** chủ dự án, 17/10/2026, phiên chính (yêu cầu +
phiếu 4 câu, chọn cả 4 đề xuất) · **Dựng trên** ADR 0097 (danh bạ người dân), 0008 (ẩn danh), 0030 (gỡ che người
gửi), 0080 (số chưa xác thực) · **Sửa** ADR 0097 điều kiện dừng #2 (một ngoại lệ, #2 dưới đây).

## Nguồn yêu cầu

*"ở danh bạ người dân, khi vào chi tiết có thể xem được phản ánh của họ qua 1 nút view xem phản ánh … danh sách
phản ánh theo phân trang … click vào phản ánh nào thì chuyển qua view phản ánh tương ứng. Đứng ở phản ánh thì có
nút đồng bộ danh bạ, nếu chưa có thì tạo mới fill sẵn data đã có, nếu có rồi thì link qua chi tiết danh bạ để có
thể chỉnh sửa"* — kèm lưu ý: liên kết là ánh xạ dữ liệu nhạy cảm, thiết kế lỏng để dễ bảo trì, nâng cấp.

## Quyết định

| # | Điểm | Chốt |
|---|---|---|
| 1 | Hình dạng liên kết | **Suy ra mỗi lần xem**, khoá `(tenant_id, SĐT đã chuẩn hoá)`. Phản ánh không lưu `contact_id`; danh bạ không lưu mã phản ánh, `cong_dan_id`, `zalo_uid` (giữ ADR 0097 #4). Không có dữ liệu chép sang nên không trôi; đổi số ở một bên thì liên kết tự đúng; gỡ một phân hệ không để lại cột mồ côi |
| 2 | Chiều phụ thuộc | **Một chiều petitions → identity** qua gRPC (luật 2). identity thêm `FindCitizenContactByPhone` (số → có/không + mã dòng) và `GetCitizenContactMatchPhone` (mã dòng → số, **chỉ máy chủ**, giữ trong bộ nhớ, không tới trình duyệt, **mỗi lần một vết kiểm toán**). Đây là **ngoại lệ duy nhất** của ADR 0097 điều kiện dừng #2, chủ dự án cho phép; chỉ service-petitions được gọi. identity không gọi petitions |
| 3 | So khớp số | Hàm chuẩn hoá số đưa vào `core` dùng chung. petitions thêm cột số đã chuẩn hoá của người gửi + chỉ mục `(tenant_id, …)`, điền lại cho phản ánh cũ; cột cũ giữ nguyên (luật 7) |
| 4 | Ẩn danh | **Loại hẳn**, cả hai chiều: phản ánh `an_danh` không bao giờ hiện trong "Xem phản ánh" của danh bạ và không có nút "Đồng bộ danh bạ" (ADR 0008) |
| 5 | Quyền | "Đồng bộ danh bạ" ở phản ánh: `feedback.unmask` + `citizen.update` (số đầy đủ tới trình duyệt qua đường gỡ che đã có, đã ghi vết). "Xem phản ánh" ở danh bạ: `citizen.read` + `feedback.read`; phản ánh `can-bo` vẫn đòi `feedback.restricted`. **Không khoá mới** (câu mở #27) |
| 6 | Số chưa xác thực (ADR 0080) | **Cho**, kèm câu cảnh báo trong hộp Thêm người dân: số do người gửi tự khai, chưa xác thực; cán bộ quyết định lưu |
| 7 | Tạo từ phản ánh | Mở **hộp Thêm người dân của danh bạ**, điền sẵn họ tên + số; cán bộ bấm Lưu nên nguồn vẫn là `nhap-tay` — **không** nguồn thứ ba (giữ ADR 0097 #4). Thôn / địa chỉ của phản ánh là **nơi xảy ra sự việc**, không điền |
| 8 | Dữ liệu cá nhân trên đường đi | Danh sách phản ánh của một người đi theo **mã dòng danh bạ**, không bao giờ số trên URL / log / sự kiện; mỗi dòng chỉ mã tra cứu, tiêu đề, trạng thái, ngày — không người gửi |

## Hệ quả

- identity là nơi duy nhất biết "số này có trong danh bạ không"; petitions là nơi duy nhất biết "số này có phản
  ánh nào". Mỗi bên trả lời câu của mình; không bên nào giữ bản sao dữ liệu bên kia.
- identity tắt hoặc lỗi: phản ánh vẫn mở bình thường, chỉ mục "danh bạ" báo chưa đối chiếu được — liên kết là phần
  phụ, không phải điều kiện của luồng xử lý phản ánh (luật 10).
- Mỗi lần danh bạ xem phản ánh của một người là một lần identity nhả số cho petitions — có vết kiểm toán, nên ai
  tra cứu ai đều trả lời được.

## ĐIỀU KIỆN DỪNG

1. Lưu mã của bên này lên bảng của bên kia, hay chép số / họ tên sang
2. Gọi `GetCitizenContactMatchPhone` từ service khác ngoài petitions, hoặc trả số ấy ra HTTP
3. Đưa phản ánh ẩn danh vào bất kỳ chiều đối chiếu nào
4. identity gọi ngược petitions

→ ADR 0097 · 0008 · 0030 · 0080 · 0002 → Luật 1 · 2 · 3 · 4 · 5 · 6 · 7
