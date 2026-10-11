---
id: 0091-commune-app-prototype-disaster-alerts-notification-opt-out
tier: T1
source: CURATED
owner: domain
derived_from_commit: 99749b50
expires: null
owns_facts:
  - "app xã (citizen-app, src/cong-dan) làm hết theo prototype ../vigov-require/apps/miniapp nhánh feat/m8-multitenant-foundation; luật cứng thắng prototype; sàn chữ 16px giữ, công tắc cỡ chữ thành một công tắc 'Cỡ chữ lớn' (chủ dự án, 11/10/2026)"
  - "cảnh báo thiên tai (DisasterAlert) dựng đủ ba tầng: service-comms sở hữu, tuyến công khai theo xã qua ?host= có giới hạn tần suất, cán bộ đăng/sửa/kết thúc ở tab của 'Nội dung Mini App' bằng content.read/content.update, Mini App có màn và băng đỏ trang chủ; sáu loại; vùng nguy hiểm là đa giác trên lưới 0..100; không khoảng cách tới điểm sơ tán; không đẩy ZNS (chủ dự án, 11/10/2026)"
  - "tra cứu hồ sơ một cửa phía dân: dữ liệu không ở ViGov; nút Tra cứu mở trang tra cứu của Cổng Dịch vụ công quốc gia bằng openWebview, không nhúng, không gửi số điện thoại hay 4 số đi đâu (chủ dự án, 11/10/2026)"
  - "công tắc 'Nhận thông báo' ở màn Cá nhân là thật (CitizenNotificationPreference, service-comms, theo xã + công dân): tắt thì không gửi tin Zalo thường, vẫn gửi khong-tiep-nhan và chuyen-cap-tren; mỗi tin không gửi vẫn có dòng trong thong_bao_gui_cong_dan với trạng thái mới 'dân đã tắt nhận' (chủ dự án, 11/10/2026)"
  - "gợi ý mô tả theo lĩnh vực là cột mặc định ở danh mục platform petition_field; thời lượng video do cán bộ gõ (chủ dự án, 11/10/2026)"
---

# 0091. App xã theo prototype 11/10/2026: cảnh báo thiên tai, nhận thông báo, tra cứu Dịch vụ công

**Trạng thái:** đã chốt · **Ngày:** 2026-10-11 · **Người quyết:** chủ dự án, trả lời trực tiếp trong
phiên chính ngày 11/10/2026. Lời do phiên chính chuyển lại; kho không có phiếu hỏi để đối chiếu nguyên
văn · **Thay một phần** ADR 0053 §8 (cảnh báo thiên tai), ADR 0050 điểm 11, ADR 0047 dòng *"Theo
PROTOTYPE.md"* mục (4), (7), (11), (12) và G2 · **Đổi bảng** ADR 0041 (luật 10 điều kiện dừng #3). Mỗi
ADR bị thay có mục *Sửa đổi 11/10/2026* trỏ về đây.

## Bối cảnh

Các ADR trước để treo bốn việc của app xã: cảnh báo thiên tai "để đợt sau" (ADR 0053 §8), tra cứu hồ sơ
chờ một hệ thống một cửa không ai nối (ADR 0050 điểm 11), công tắc "Nhận thông báo" bị gỡ vì không nối
vào đâu (ADR 0047 mục 12), và giao diện lấy từng phần của prototype. Ngày 11/10/2026 chủ dự án chốt cả
bốn, theo cùng hướng của ADR 0090 cho web-admin: **chuẩn là prototype, luật cứng thắng prototype.**

Nguồn prototype: `../vigov-require/apps/miniapp`, nhánh `feat/m8-multitenant-foundation`, bản chạy
`https://vigov-production.up.railway.app`.

## Quyết định

### 1. App xã làm hết theo prototype

| Điểm | Chốt |
|---|---|
| Phạm vi | `citizen-app/src/cong-dan` (app riêng của xã và app chung mở từ QR của xã) theo prototype **từng màn** |
| Thứ tự khi va nhau | Luật cứng (dữ liệu cá nhân, cách ly công dân, bảo mật — luật 3, 4, 13) **thắng** prototype |
| Cỡ chữ | Sàn chữ **16px giữ**. Ba mức cỡ chữ của ADR 0047 mục (11) **đổi thành một công tắc "Cỡ chữ lớn"** như prototype (`ProfilePage.tsx:80-86`) |

### 2. Cảnh báo thiên tai — dựng đủ ba tầng

| Điểm | Chốt |
|---|---|
| Thực thể | `DisasterAlert` (tên: `ubiquitous-language.md` §Kênh công dân) |
| Service sở hữu | **`service-comms`** — đóng luật 2 điều kiện dừng #1 cho thực thể này |
| Phía dân | Tuyến **công khai theo xã** qua `?host=`, giới hạn tần suất **như tin công khai**. Chủ dự án mở tuyến không xác thực mới này (luật 13, điều kiện dừng) |
| Phía cán bộ | Đăng · sửa · kết thúc trên web-admin, **một tab trong "Nội dung Mini App"**. Quyền có sẵn `content.read` / `content.update` (`service-identity/migrations/0001_init.sql:292-293`) — không khoá mới |
| Mini App | Màn cảnh báo + **băng đỏ** ở trang chủ, như prototype |
| Loại | **Sáu**: bão · lũ · sạt lở · mưa lớn · dịch bệnh · cháy (gộp prototype với SRS) |
| Vùng nguy hiểm | Cán bộ **vẽ đa giác trên lưới 0..100** ở web-admin; Mini App vẽ lại như prototype (`AlertPage.tsx:164-199`, `types/index.ts:149-150`). Lưới là khung minh hoạ, **không** phải kinh/vĩ độ |
| Khoảng cách tới điểm sơ tán | **Bỏ.** Cần vị trí nhà dân |
| Điểm sơ tán, số khẩn cấp | Nhập **theo từng cảnh báo** |
| Đẩy ZNS khi đăng | **Chưa** |
| Họ tên + số khẩn cấp | **Công khai** trên tuyến không xác thực, như prototype. Form web-admin ghi cạnh ô: "Số này hiện công khai cho mọi người dân — nên dùng số trực, đường dây nóng hoặc số cơ quan." Chủ dự án chốt 11/10/2026 (luật 3 điều kiện dừng #1) |
| Giới hạn tần suất | **Ngân sách riêng 1.200 lượt/phút** mỗi (tên miền, mạng máy gọi), tách khỏi tin công khai, kèm **cache 30 giây mỗi xã** phía máy chủ. Cảnh báo mới tới dân chậm tối đa 30 giây. Chủ dự án chốt 11/10/2026 (luật 13, ngưỡng) — thay dòng "như tin công khai" ở trên |

### 3. Tra cứu hồ sơ một cửa — mở trang của Cổng Dịch vụ công

| Điểm | Chốt |
|---|---|
| Dữ liệu | **Không ở ViGov.** Khớp `ubiquitous-language.md` dòng *Hồ sơ một cửa* (ngoài phạm vi từ 20/09/2026) |
| Màn | Giữ giao diện prototype |
| Nút Tra cứu | Mở `https://vpcp.dichvucong.gov.vn/p/home/dvc-tra-cuu-ho-so.html` trong trình duyệt của Zalo (`openWebview`) |
| Không nhúng | Trang trả `X-Frame-Options: SAMEORIGIN` (đo bằng `curl -I` ngày 11/10/2026) |
| Dữ liệu cá nhân | **Không** gửi số điện thoại hay 4 số cuối đi đâu. Dân gõ trên trang của Cổng |

### 4. Công tắc "Nhận thông báo" là thật

| Điểm | Chốt |
|---|---|
| Thực thể | `CitizenNotificationPreference`, ở **`service-comms`**, khoá theo **xã + công dân** (luật 1, 4) |
| Tắt thì | **Không** gửi tin Zalo thường (đã tiếp nhận, đang xử lý, đã xử lý xong…) |
| Vẫn gửi dù đã tắt | `khong-tiep-nhan` (kèm lý do) và `chuyen-cap-tren` |
| Bằng chứng | Mỗi tin không gửi vì dân đã tắt **vẫn có dòng** trong `thong_bao_gui_cong_dan`, với **trạng thái mới** "dân đã tắt nhận". Hôm nay CHECK chỉ nhận bốn trạng thái (`service-comms/migrations/0004_thong_bao_gui_cong_dan.sql:254-255`); thêm trạng thái là migration mới, mã viết theo ADR 0011 |
| Trong app | Vẫn hiện **tiến độ đầy đủ** |
| Có tác dụng từ | Ngày đường gửi ZNS chạy thật. Hôm nay chưa nối: ADR 0041 còn mở #4 |

### 5–7. Ba điểm nhỏ

| # | Điểm | Chốt |
|---|---|---|
| 5 | Giờ từng bước trên dòng thời gian phiếu | Theo ADR 0047 **G4** (hai cột giờ mới trên phiếu) — chỉ xác nhận, không đổi |
| 6 | Gợi ý mô tả theo lĩnh vực | **Cột mặc định** ở danh mục platform `petition_field` (`service-platform/migrations/0011_petition_field.sql:62`) |
| 7 | Thời lượng video | Cán bộ gõ, như truyền thanh (ADR 0047 G7) |

## Vì sao

- **Tắt nhận vẫn gửi hai nhánh kết thúc.** `khong-tiep-nhan` và `chuyen-cap-tren` là lúc xã **không**
  xử lý nữa. Dân không được báo thì không biết phải đi đâu tiếp — đúng chỗ ADR 0041 §*Vì sao* #3 gọi là
  "cách niềm tin chết". Các tin còn lại dân vẫn đọc được trong app.
- **Tin không gửi vẫn ghi sổ.** Luật 10 bất biến 5 đòi mỗi chuyển trạng thái trong bảng ADR 0041 đều
  báo. Có dòng "dân đã tắt nhận" thì khi bị hỏi, xã chứng minh được nghĩa vụ đã phát sinh và vì sao tin
  không đi. Không có dòng thì không phân biệt được "dân tắt" với "hệ thống quên".
- **Tra cứu bằng trang của Cổng, không nhúng.** ViGov không giữ hồ sơ một cửa. Mở trang của Cổng thì
  không có số điện thoại nào đi qua ViGov (luật 3 bất biến 6), và không cần tuyến tra cứu không phiên
  (luật 4 điều kiện dừng #2).
- **Vùng nguy hiểm trên lưới 0..100, không bản đồ thật.** Không gửi toạ độ cho nhà cung cấp bản đồ
  (luật 3 điều kiện dừng #2). Màn hiện được cả khi mạng yếu giữa bão, như chú thích của prototype
  (`AlertPage.tsx:160-162`).

## Cái giá

- **Vùng nguy hiểm chỉ là hình minh hoạ.** Người dân không đọc được nhà mình có trong vùng hay không.
  Cán bộ phải ghi rõ vùng bằng chữ trong nội dung cảnh báo.
- **Tuyến công khai mới.** Ai cũng đọc được cảnh báo của một xã nếu biết tên miền. Đây là ý định: cảnh
  báo là thông tin công khai. Đổi lại phải có giới hạn tần suất (luật 13 bất biến 7).
- **Chưa đẩy ZNS.** Dân chỉ thấy cảnh báo khi mở app. SRS M6.1.7 (P0) có đẩy thông báo; việc ấy còn nợ.
- **Công tắc chưa có tác dụng.** Đường ZNS chưa chạy, nên hôm nay tắt hay bật đều không đổi gì về tin
  Zalo. Giao diện không được nói rằng nó đã có tác dụng.
- **`thong_bao_gui_cong_dan` thêm trạng thái.** Báo cáo tỷ lệ gửi thành công phải loại dòng "dân đã tắt
  nhận", nếu không tỷ lệ sẽ thấp sai.

## Còn mở — người dựng không tự chọn

| # | Việc | Vì sao không tự chọn |
|---|---|---|
| 1 | Tin gộp/tách phiếu (ADR 0041 §*Sửa đổi 09/10/2026*) có bị chặn khi dân đã tắt không | Chủ dự án kể "tin thường" bằng ví dụ, không kể tin gộp/tách. Cách đọc của người ghi: chặn, vì không phải nhánh kết thúc. Cần chủ dự án xác nhận |
| 2 | Công dân chưa từng bật/tắt | Theo luật 10 bất biến 5, chưa có dòng thì phải **đang nhận**: mặc định tắt sẽ chặn mọi tin mà không ai chọn. Đây là suy luận của người ghi, chưa phải lời chủ dự án |
| 3 | Xã có sửa được gợi ý mô tả của từng lĩnh vực không | Chủ dự án chỉ nói "cột mặc định". ADR 0026: xã đổi được nhãn, không thêm mã |
| 4 | Ca phường | Chữ "xã" trong câu cảnh báo — như việc còn nợ ở ADR 0050 §*Chỗ không xung đột* |

## Hệ quả

- Đổi công tắc là một lần **ghi** của công dân: phải có vết kiểm toán cùng giao dịch (luật 6).
- Tuyến cảnh báo công khai và tuyến ghi của cán bộ cần ca 401/403/200 theo luật 5 bất biến 7.
- Bảng ADR 0041 không thêm hay bớt dòng. Thứ đổi là **có gửi hay không** theo lựa chọn của dân.

→ ADR 0090 (chuẩn là prototype) · 0041 · 0047 · 0050 · 0053 · 0018 (ZNS từ OA của xã) · 0026 · 0011
→ Luật 1 · 2 · 3 · 4 · 6 · 10 · 13
