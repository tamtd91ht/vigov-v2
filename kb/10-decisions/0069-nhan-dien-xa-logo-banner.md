---
id: 0069-nhan-dien-xa-logo-banner
tier: T1
source: CURATED
owner: architecture
derived_from_commit: f2e802a9
expires: null
owns_facts:
  - "logo xã và banner web-admin do CHÍNH XÃ tải lên trong web-admin (Cấu hình › Nhận diện xã), quyền admin.org; không có đường sửa ở platform-admin — thay hướng 'cấu hình ở platform-admin' của ADR 0047:246 và đợt 2 'hồ sơ hiển thị + logo' của ADR 0048 (chốt 02/10/2026)"
  - "logo xã dùng chung: thanh bên + màn đăng nhập web-admin và Mini App của xã; banner web-admin là ảnh riêng, hiện thành dải dưới topbar ở MỌI trang web-admin; banner Mini App vẫn là loại nội dung banner của ADR 0067 §5, không gộp (chốt 02/10/2026)"
  - "tải lên là hiện ngay, không qua duyệt; mọi lần đặt/gỡ ghi nhật ký cùng giao dịch, người làm là mã CB-; ảnh qua quét mã độc và chuẩn hoá trước khi công khai (chốt 02/10/2026)"
  - "logo: PNG/WebP/JPEG ≤ 2 MB, không HEIC; máy chủ chuẩn hoá về PNG vuông 512px giữ nền trong suốt; banner web-admin: PNG/WebP/JPEG ≤ 2 MB, chuẩn hoá về rộng 1600px (chốt 02/10/2026)"
  - "chưa có logo thì hiện biểu tượng toà nhà (Landmark) — không ảnh nào cơ quan chưa ban hành; chưa có banner thì không có dải (chốt 02/10/2026)"
  - "logo và banner đi ra qua tuyến công khai SẴN CÓ GET /api/v1/communes/current (thêm trường, không mở tuyến mới) để màn đăng nhập đọc trước khi có phiên; Mini App đọc logo qua /commune-profiles; đường dẫn ảnh công khai chứa t_<tenant_id> như mọi ảnh công khai của ADR 0052 — chấp nhận (chốt 02/10/2026)"
---

# 0069. Nhận diện xã — logo và banner web-admin do xã tự tải lên

**Trạng thái:** đã chốt · **Ngày:** 2026-10-02 · **Người quyết:** chủ dự án, 02/10/2026 · **Thay**
hướng "logo cấu hình ở platform-admin" của ADR 0047 (dòng 246) và mục đợt 2 "hồ sơ hiển thị + logo" của
ADR 0048.

## Bối cảnh

Thanh bên web-admin đứng tên Ủy ban nhân dân xã (ADR 0068 §13) nhưng ô logo là biểu tượng: bảng
`ho_so_hien_thi_xa` (service-platform) có cột `logo_url` dạng URL gõ tay, không đường ghi nào, và hai
tuyến công khai cố ý không trả logo. Mini App dùng logo và banner chép vào bản build lúc deploy.

## Quyết định

| # | Điểm | Chốt |
|---|---|---|
| 1 | Ai sửa | **Chỉ xã**, trong web-admin. Không có đường sửa ở platform-admin |
| 2 | Quyền | `admin.org` (khoá đã có trong `quyen`) |
| 3 | Duyệt | Không — lưu là hiện; ghi nhật ký cùng giao dịch, mã `CB-` |
| 4 | Logo | Dùng chung web-admin (thanh bên, màn đăng nhập) và Mini App. PNG/WebP/JPEG ≤ 2 MB, không HEIC → PNG vuông 512px **giữ nền trong** |
| 5 | Banner web-admin | Ảnh **riêng**, dải dưới topbar ở **mọi trang** web-admin. PNG/WebP/JPEG ≤ 2 MB → rộng 1600px |
| 6 | Banner Mini App | **Giữ** loại nội dung `banner` (ADR 0067 §5) ở Nội dung Mini App; không gộp với banner web-admin |
| 7 | Chưa có logo | Biểu tượng toà nhà; chưa có banner thì không có dải |
| 8 | Đường ra | Thêm trường vào tuyến công khai sẵn có `GET /api/v1/communes/current` (màn đăng nhập đọc trước phiên) và `/commune-profiles` (Mini App) — không mở tuyến mới |

**Vì sao chỉ xã sửa:** chủ dự án chốt "Xã tự tải lên"; nhận diện là nghiệp vụ của cơ quan xã, một
nguồn ghi thì không có hai bên ghi đè nhau (ADR 0048 §30/09 #7 cho phép hai bên — không còn cần).

**Vì sao banner web-admin tách khỏi banner Mini App:** chủ dự án hỏi "nếu tận dụng được thì quy về 1
chỗ", rồi chốt mục đích là một banner phía trên web-admin, "nếu không liên quan với mini app thì nên
lưu cả 2". Dải banner Mini App là nội dung cho dân (nhiều ảnh, có link, có thứ tự); banner web-admin
là nhận diện cho cán bộ — hai thứ khác nhau.

**Vì sao giữ nền trong của logo:** đường ảnh bìa hiện chuẩn hoá về JPEG, làm logo nền trong thành nền
trắng trên thanh bên. Cần một kiểu ảnh dẫn xuất PNG trong `core/storage`.

## Hệ quả

- `service-platform` lần đầu nhận tuyến ghi của cán bộ: cần kiểm quyền, kho tệp + quét mã độc (nhóm
  biến đã có trong `core/config`), bảng `stored_file` riêng của platform (luật 2: không dùng chung bảng
  của comms). Triển khai phải cấp cho platform các biến kho tệp/quét mã độc như comms đang có.
- Ảnh công khai có cache bất biến 1 năm (ADR 0052): đổi logo luôn sinh khoá mới; bản cũ có thể còn
  trong cache trình duyệt.
- `logo_url` đổi nghĩa: từ URL gõ tay sang ảnh do ViGov phát hành — migration riêng.
- Mini App: logo đọc lúc chạy thay ảnh chép lúc build; ảnh chép giữ làm dự phòng tới khi mọi xã đã
  tải logo (cùng cách ADR 0067 B11 giữ banner dự phòng).
