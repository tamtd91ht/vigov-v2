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
  - "mã bot riêng của xã niêm phong bằng khoá dữ liệu riêng từng xã của service-comms; webhook bot của xã trên tên miền của xã, xã lấy từ Host; secret webhook chỉ ghi, hiện một lần; đổi bot chấm dứt mọi ghép nối (lô thứ hai Q1, 08/10/2026)"
  - "không làm máy chủ thư dự phòng của nền tảng lượt này, platform_fallback luôn false (lô thứ hai Q1, 08/10/2026)"
  - "'Sắp đến hạn: nhắc trước' của Kênh Zalo sửa được (1–14 ngày), comms lưu due_soon_days theo từng xã, bộ phát Zalo áp dụng; ngưỡng Zalo được phép khác ngưỡng 'sắp đến hạn' của chuông — THAY lô thứ hai Q1 #7 (lô thứ tư Q10, 08/10/2026)"
  - "Kênh Zalo chỉ có 18 dòng của spec; dòng 'Văn bản, đơn thư quá hạn' và 'Phản ánh quá hạn' bật/tắt cùng lúc các loại quá hạn + chưa cử người + đôn đốc của miền mình; nhiệm vụ giữ dòng riêng 'Việc bị đôn đốc lên cấp trên' và 'Bộ phận chưa cử người làm' (lô thứ tư Q11, 08/10/2026)"
  - "'Kiểm tra kết nối' ẩn khi xã dùng bot chung, như Đăng ký webhook / Quay về bot chung (lô thứ tư Q12, 08/10/2026)"
  - "biểu mẫu thêm câu Lời hệ thống không có ô 'Nhóm'; nhóm lấy từ tiền tố mã (chung. / phan-anh. / giai-ngan.), như prototype (phiên chính, 08/10/2026)"
  - "Lời hệ thống: xã được thêm câu, tắt/bật lại, xoá mềm câu tự thêm — thay quyết định 28/09/2026 và phần bộ mã đóng của ADR 0024 (lô thứ hai Q2, 08/10/2026)"
  - "câu Lời hệ thống xã tự thêm vào nhóm 'Dùng chung' cất ở service-petitions; câu xã tự thêm chỉ lưu và quản lý, chưa hiện ra ở kênh nào (lô thứ hai Q5, 08/10/2026)"
  - "18 loại nhắn Zalo chia hai giai đoạn: loại đã có nơi phát làm ngay, 10 loại chưa có nơi phát giữ '?' và mỗi loại một ADR + outbox (lô thứ hai Q3, 08/10/2026)"
  - "dòng thời hạn riêng cho mọi loại việc (phản ánh theo lĩnh vực, văn bản theo loại văn bản, nhiệm vụ theo mức ưu tiên), không bước duyệt, tra lúc cố định hạn — thay ADR 0029 điểm nhiệm vụ dùng dòng mặc định (lô thứ hai Q4, 08/10/2026)"
  - "lời dẫn tab Lời hệ thống chỉ nói trang quản trị, không nói Mini App; menu Cấu hình bỏ các phần web có mà prototype không có (dòng Sửa lần cuối, ghi chú chưa dùng, chú thích hai cột báo lãnh đạo, nút gieo khi bảng đã có dòng, hộp xác nhận Khôi phục lời gốc — thay ADR 0068 §15 ở điểm này); Nhập từ Excel luôn ở đầu tab Danh mục; thanh tab xuống dòng khi tràn (lô thứ ba Q6–Q9, 08/10/2026)"
  - "luật thường trực của menu Cấu hình: mọi việc phải tham khảo prototype (chủ dự án, lô thứ năm, 08/10/2026)"
  - "'Sắp đến hạn: nhắc trước' của Zalo chỉ THU HẸP tin Zalo sắp đến hạn về các hồ sơ có hạn đã lưu nằm trong số ngày ấy; chuông và danh sách giữ cột SLA; comms lọc theo due_soon_items {code, deadline} do bên phát gửi, không tính hay đổi hạn — làm rõ lô thứ tư Q10 (lô thứ năm Q13, 08/10/2026)"
  - "nút 'Gieo thời hạn mặc định' khi bảng thời hạn đã có dòng chỉ hiện khi máy chủ báo thiếu dòng mặc định (lô thứ năm Q14, 08/10/2026)"
  - "ngưỡng webhook Zalo 600 lần/phút mỗi xã là con số của chủ dự án, không còn tạm (lô thứ năm Q15, 08/10/2026)"
  - "quyền thay bot của xã giữ admin.lookup (lô thứ năm Q16, 08/10/2026)"
  - "lỗi đọc đảo cờ dang_dung/la_mac_dinh không sửa dữ liệu hàng loạt; xã tự kiểm/sửa cờ ở màn Danh mục (lô thứ năm Q17, 08/10/2026)"
  - "nhắc việc cho đơn thư làm theo prototype: dùng hạn đã lưu và dòng SLA mặc định của đơn thư, gửi bằng loại DOCUMENT_* (lô thứ năm Q18, 08/10/2026)"
  - "câu chữ menu Cấu hình: lời prototype thắng nguyên văn khi prototype có câu (lô thứ năm Q19, 08/10/2026)"
  - "ô 'Sắp đến hạn: nhắc trước' của Zalo có lựa chọn trống 'Theo chuông' (không thu hẹp); lời dẫn 'Nhắc khi nào' thay câu prototype bằng câu đúng với Q13; bật/tắt một dòng thành công thì không toast, như prototype; đăng ký webhook lỗi do mã bot dùng nguyên câu prototype, lỗi khác nói lý do thật (lô thứ sáu, 08/10/2026)"
---

# 0079. Menu Cấu hình theo spec Cấu hình 02–12

**Trạng thái:** đã chốt; lô câu trả lời thứ hai 08/10/2026 đóng phần lớn điều kiện dừng backend, còn dòng 9 (§*Còn mở*); lô thứ ba (chấm ảnh vòng 1) chốt phần trình bày; lô thứ tư (chấm ảnh vòng 4) thay lô 2 Q1 #7; lô thứ năm đóng các câu còn mở Q13–Q19 · **Ngày:** 2026-10-08 ·
**Người quyết:** chủ dự án, 08/10/2026, phiên chính, lệnh `/fix-web-admin --menu=cau-hinh --des=Cập
nhật lại ui ux toàn bộ view và action trong menu cấu hình này, tham khảo hướng dẫn tại
.\tmp\web\cau-hinh\vigov-cau-hinh-spec\*.md` · **Bổ sung** ADR 0068 §*Sửa đổi 06/10/2026 (lần 5)* và
§*Sửa đổi 07/10/2026 (lần 6)* cho riêng menu Cấu hình, như ADR 0076 cho Nhiệm vụ · **Thay một phần**
ADR 0074 (#1, #3, #4 — chỉ phần bot của xã, §*Quan hệ với ADR 0074*).

**Sửa đổi 09/10/2026** (người dùng, spec `tmp/web/updated/cap-nhat-menu-cau-hinh-theo-prototype.md`,
"theo prototype, prototype không có thì theo khuyến nghị của spec"). Các điểm dưới THAY điểm tương ứng
của ADR này:

| Điểm 08/10 | Thay bằng 09/10 |
|---|---|
| Lô ba Q9: thanh tab xuống dòng khi tràn | Thanh tab MỘT dòng, cuộn ngang ẩn thanh cuộn, tab đang chọn tự vào tầm nhìn |
| Tab Lịch làm việc ngay sau Thời hạn xử lý | 9 tab theo thứ tự prototype, rồi Lịch làm việc · Nhật ký hệ thống · Nhận diện xã |
| Huy hiệu trạng thái giữ icon + chữ | Chỉ chữ, như prototype |
| 'Giữ bất kể spec': bỏ câu 'Mặc định 72 giờ' | Câu 'Mặc định {N} giờ.' với N đọc từ dòng mặc định phản ánh của chính xã; không có dòng thì không hiện (luật 10 cấm #3 vẫn giữ: không ghi cứng số) |
| Trường bản đồ bỏ câu đầu về tệp Excel | Thêm nguyên câu prototype, dù chưa có tuyến nhập tài nguyên bản đồ |
| Lô hai Q3: 10 loại Zalo chưa có nơi phát, mỗi loại một ADR | Một ADR chung: ADR 0086 |
| Lời hệ thống: chỉ tắt được câu xã đã sửa/tự thêm | Tắt được mọi câu; câu bị tắt ẩn hẳn ở nơi dùng |
| Thẻ 'Gửi báo cáo định kỳ ?' | Dựng thật theo prototype, ADR 0086 B1 |
| Danh mục chưa có Lĩnh vực phản ánh | Thêm nhóm Lĩnh vực phản ánh qua tuyến có sẵn: chỉ sửa nhãn, thứ tự, bật/tắt (ADR 0026) |

Giữ nguyên, người dùng chốt cùng ngày: không đổi mã bộ phận đã cấp, không tách Loại đơn thư
`kien-nghi-phan-anh`, không gieo dữ liệu xã (thôn, mã nhiệm vụ, trường Doanh nghiệp), không thêm việc
nền Tính lại số liệu Tổng quan, Thôn giữ Ngừng dùng thay Xoá.

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

Không quyết ở đây. Lượt backend phải hỏi chủ dự án trước khi viết mã. Dòng đã đóng ghi nơi đóng
(§*Lô câu trả lời thứ hai*); dòng còn mở vẫn là điều kiện dừng:

| # | Điểm | Va với | Trạng thái |
|---|---|---|---|
| 1 | Cách cất mã bot riêng của xã | luật 8 điều kiện dừng #1 (bí mật bên thứ ba mới, theo xã) | **Đóng** 08/10 — Q1 mục 1 |
| 2 | Tuyến webhook không đăng nhập cho bot của xã | luật 13 điều kiện dừng (tuyến không xác thực mới) | **Đóng** 08/10 — Q1 mục 2, 3 |
| 3 | Nguồn phát của 18 sự kiện ở các service | luật 2 (chủ dữ liệu, hợp đồng sự kiện) | **Đóng một phần** 08/10 — Q3: loại đã có nơi phát làm lượt này; 10 loại còn lại → dòng 9 |
| 4 | `due_soon_days` của Zalo so với `due_soon_hours` của SLA | luật 9 (một sự kiện, một nguồn); 0074 đã chốt ngưỡng "sắp đến hạn" dùng chung với chuông | **Đóng** 08/10 — Q1 mục 7, rồi Q10 (lô thứ tư) thay: lưu riêng cho Zalo |
| 5 | Thêm câu hệ thống | ADR 0024 (bộ mã đóng, câu gốc sống trong mã) | **Đóng** 08/10 — Q2; còn hai câu hỏi tiếp → dòng 7, 8 |
| 6 | Thêm/xoá dòng SLA | ADR 0026 điều kiện dừng #2 · ADR 0029 điều kiện dừng #4 | **Đóng** 08/10 — Q4 |
| 7 | Nhóm "Dùng chung" của Lời hệ thống thuộc service nào | luật 2 (một chủ cho mỗi thực thể) | **Đóng** 08/10 — Q5a |
| 8 | Câu xã tự thêm hiện ở đâu (màn nào, cho ai) | ADR 0024 · luật 4 (câu hiện cho công dân) | **Đóng** 08/10 — Q5b |
| 9 | 10 loại nhắn Zalo chưa có nơi phát | luật 2 (hợp đồng sự kiện, outbox) | **Mở** — mỗi loại một ADR + outbox ở lượt sau (Q3) |

## Lô câu trả lời thứ hai — lượt backend (08/10/2026)

Chủ dự án trả lời năm câu hỏi của lượt backend, 08/10/2026. Lựa chọn ghi nguyên văn trong ngoặc kép.
Đóng các dòng 1, 2, 4–8 và một phần dòng 3 của §*Còn mở*; mở thêm dòng 9.

### Q1 — gói mặc định kỹ thuật 1–9: **"Chấp nhận cả 9"**

| # | Điểm | Chốt |
|---|---|---|
| 1 | Cách cất mã bot của xã | Niêm phong bằng khoá dữ liệu riêng từng xã của service-comms — cùng cách mật khẩu máy chủ thư (ADR 0009), tiền lệ ADR 0066. Chủ dự án chấp thuận theo luật 8 điều kiện dừng #1. Thay câu "cách cất còn mở" ở §*Quan hệ với ADR 0074* dòng #4 |
| 2 | Webhook bot của xã | Nhận trên chính tên miền của xã `https://<xã>/api/v1/zalo-bot-updates`; xã lấy từ `Host` (luật 1 bất biến 3). Giới hạn tần suất + so secret thời gian hằng, cùng thứ tự với `service-comms/internal/http/zalo_bot_updates.go:11-21`. Không đọc chéo xã. Đóng điều kiện dừng luật 13 |
| 3 | Secret webhook | Chỉ ghi; hiện **một lần** lúc sinh. Không hiện rõ trên GET như spec 11 §3 (ADR 0074 #4) |
| 4 | Đổi bot chung ↔ bot riêng | Chấm dứt mọi ghép nối đang sống; cán bộ phải ghép lại. Có hộp xác nhận nêu số người bị ảnh hưởng |
| 5 | Đường dẫn | `zalo-bots/current` (+ `/check`, `/webhook`; `DELETE` = quay về bot chung), song song `zalo-bots/shared` |
| 6 | Mã loại nhắn mới | Tiếng Việt không dấu (vd `nhiem-vu.sap-den-han`) theo ADR 0011 / 0051; web ánh xạ sang mã spec |
| 7 | "Nhắc trước khi đến hạn" (`due_soon_days`) | ~~**Không** lưu ở comms. Chỉ đọc, lấy từ cột "Sắp đến hạn khi còn" của bảng SLA, kèm liên kết sang tab Thời hạn xử lý (luật 9)~~ — **Đã bị thay** bởi §*Lô câu trả lời thứ tư* Q10 |
| 8 | Lần thử gửi thư gần nhất | Lưu người nhận đã che (`privacy.MaskEmail`, `core/privacy/mask.go`) và lỗi theo lớp; không lưu chữ thô của máy chủ SMTP. **Không** làm máy chủ thư dự phòng của nền tảng lượt này (luật 11 điều kiện dừng #1 + luật 8): `platform_fallback` luôn `false` |
| 9 | Màu mục danh mục | Sửa được cả trên mục "Hệ thống" — chỉ là trình bày |

### Q2 — Lời hệ thống: **"Làm đúng prototype"**

Câu hỏi đã nêu rõ quyết định 28/09/2026: xã không tự đặt câu, không có "+ Thêm câu mới"
(`service-petitions/migrations/0020_system_message_override.sql:9-17`; bản ghi cùng quyết định ở
`service-finance/migrations/0010_system_message_override.sql`).

Chốt: có **Thêm câu mới**; **Tắt / Bật lại** (giữ lời đã sửa, khi tắt dùng lời gốc); **Xoá** câu xã
tự thêm (xoá mềm, luật 7).

**Thay** quyết định 28/09/2026 và phần "bộ mã đóng" tương ứng của ADR 0024 §`loi_he_thong`. Hai chú
thích migration nói trên và ADR 0024 không sửa ở đây; điểm nào va thì ADR này thắng.

Hai câu hỏi tiếp đã trả lời cùng ngày — §*Q5* dưới.

### Q5 — câu hỏi tiếp của Lời hệ thống

| # | Câu hỏi | Chốt | Nghĩa |
|---|---|---|---|
| 5a | Câu xã tự thêm vào nhóm "Dùng chung" cất ở đâu | **"Service Phản ánh"** | Câu "Dùng chung" do xã thêm thuộc service-petitions. Nhóm Báo cáo điều hành vẫn là nhóm riêng như hiện tại; nhóm Phản ánh ↔ service-petitions, Giải ngân ↔ service-finance |
| 5b | Câu xã tự thêm hiện ra ở đâu | **"Chỉ lưu và quản lý, chưa hiện ra đâu"** | Thêm / sửa / tắt / xoá trong danh mục; màn ghi rõ chưa có chức năng nào dùng câu này. Nơi dùng (ZNS, Mini App) để lượt sau, khi có chỗ gọi |

### Q3 — 18 sự kiện Zalo: **"Chia hai giai đoạn"**

| Giai đoạn | Loại | Làm gì |
|---|---|---|
| Lượt này | Loại **đã có nơi phát**: sắp đến hạn / quá hạn theo nhiệm vụ, văn bản–đơn thư, phản ánh; đôn đốc; bộ phận chưa cử người (tách khỏi quá hạn); bản tin tuần | Mỗi loại thành ô bật/tắt riêng |
| Lượt sau | 10 loại **chưa có nơi phát**: giao việc mới, đề nghị lùi hạn, việc chờ duyệt, được nhắc tên, văn bản chuyển tới, phản ánh được phân công, phản ánh mở lại, thông báo mới, báo cáo sẵn sàng… | Giữ "?" (ADR 0068 §14). Mỗi loại một ADR + outbox (luật 2) — §*Còn mở* dòng 9 |

### Q4 — dòng thời hạn: **"Theo prototype"**

Câu hỏi đã nêu: hôm nay chỉ phản ánh đọc dòng theo lĩnh vực; văn bản, đơn thư, nhiệm vụ chỉ đọc
dòng mặc định; có cần lãnh đạo duyệt không.

| Điểm | Chốt |
|---|---|
| Dòng riêng | Thêm cho **mọi** loại việc như prototype `SlaTable`: phản ánh theo lĩnh vực phản ánh; văn bản theo loại văn bản; nhiệm vụ theo mức ưu tiên nhiệm vụ |
| Duyệt | **Không** có bước duyệt. Khoá `admin.sla` + vết kiểm toán; không áp ngược lên hạn đã cố định (ADR 0028) |
| Xoá | Xoá dòng riêng phải có lý do; **không** xoá dòng mặc định |

Hệ quả, chủ dự án chốt bằng chính câu trả lời này:

| # | Hệ quả |
|---|---|
| a | **Thay** ADR 0029 ở điểm "nhiệm vụ không mang lĩnh vực / dùng dòng mặc định". Dòng "Bảng SLA" của §*Giữ bất kể spec* ở trên cũng thôi hiệu lực ở nửa "nhiệm vụ dùng dòng mặc định"; nửa "giữ 4 loại việc gồm đơn thư" giữ nguyên |
| b | Để dòng riêng có tác dụng: service-documents tra thời hạn theo loại văn bản, service-petitions (nhiệm vụ) tra theo mức ưu tiên, **lúc cố định hạn** (luật 10 bất biến 2). Đây là đổi cách tính hạn — luật 10 điều kiện dừng #1 — chủ dự án chốt bằng câu trả lời này |
| c | Mã lĩnh vực phản ánh đối chiếu bộ mã tầng 1 qua `ListPetitionFields` (ADR 0060). ADR 0026 điều kiện dừng #2 đã đóng bởi ADR 0060 |
| d | ADR 0029 điều kiện dừng #4 (duyệt) **đóng**: không duyệt |

ADR 0024, 0029 và chú thích các migration nêu trên không sửa ở lượt này; trên các điểm đã kể, ADR
này thay chúng.

## Lô câu trả lời thứ ba — chấm ảnh vòng 1 (08/10/2026)

Chủ dự án trả lời bốn câu hỏi sau khi chấm ảnh vòng 1 của menu Cấu hình, 08/10/2026. Lựa chọn
ghi nguyên văn trong ngoặc kép.

| # | Câu hỏi | Chốt | Nghĩa |
|---|---|---|---|
| Q6 | Lời dẫn tab Lời hệ thống. Spec nói các câu hiện "ở cả trang quản trị và Zalo Mini App" — sai: citizen-app không đọc câu nào | **"Giữ spec, chỉ bỏ phần Mini App"** | Lời dẫn: "Những câu dưới đây là lời hệ thống hiện ra trên trang quản trị. Câu đi kèm phần mềm có thể sửa lời nhưng không xoá được — xoá đi thì lúc từ chối, hệ thống không còn gì để nói." |
| Q7 | Phần web có mà prototype không có: (a) dòng "Sửa lần cuối: CB-…" trên thẻ câu hệ thống; (b) ghi chú "Chưa có chức năng nào dùng câu này" + hộp ghi chú nhóm Báo cáo; (c) chú thích dưới bảng thời hạn về hai cột báo lãnh đạo; (d) nút "Gieo thời hạn mặc định" khi bảng đã có dòng; (e) hộp xác nhận trước "Khôi phục lời gốc" | **"Bỏ hết, đúng prototype"** | Bỏ cả năm. Xem các hệ quả dưới bảng |
| Q8 | Danh mục "Nhập từ Excel": prototype có một nút luôn ở đầu tab; máy chủ chỉ có tuyến nhập theo từng nhóm | **"Theo prototype"** | Nút luôn ở đầu tab. Nhóm đang lọc có tuyến nhập thì nút chạy thật; "Tất cả", hoặc nhóm không có tuyến nhập, thì nút là control "?" (ADR 0068 §14) |
| Q9 | Thanh 12 tab tràn ở bề rộng 1440px | **"theo prototype nếu prototype chưa có thì theo 1"** | Prototype có 9 tab và không nói gì về chỗ tràn, nên áp phương án 1: thanh tab **xuống dòng** khi không đủ chỗ |

Hệ quả của Q7:

| Điểm | Hệ quả |
|---|---|
| (b) so với Q5b | Q5b đòi màn ghi rõ câu xã tự thêm chưa được chức năng nào dùng. Q7 bỏ ghi chú ấy, theo prototype; Q7 trả lời sau nên thắng ở điểm trình bày này. Q5b vẫn đúng ở phần nghĩa: câu chỉ lưu và quản lý, chưa hiện ra đâu |
| (e) | **Thay** ADR 0068 §15, chỉ dòng "Xác nhận khôi phục câu mặc định" và chỉ cho nút Khôi phục lời gốc. Các dòng khác của §15 giữ nguyên, kể cả "Ngừng dùng <tên>?". ADR 0068 không sửa ở đây |
| Cùng nguyên tắc | Bỏ thêm đoạn chú thích về người nhận ở tab Tự động hoá, vì prototype không có |
| Không đổi | Khối gieo khi bảng thời hạn còn trống (`KhoiChuaKhai`) giữ. (d) chỉ bỏ nút gieo khi bảng **đã có** dòng |

### Phiên chính tự quyết theo luật/ADR sẵn có — không phải câu trả lời của chủ dự án

| Điểm | Quyết | Căn cứ |
|---|---|---|
| Huy hiệu "Mặc định" | Chỉ hiện chữ, như prototype | Đây là thuộc tính, không phải trạng thái. §*Quyết định* #6 ("Giữ icon + chữ") chỉ nói về huy hiệu trạng thái |
| Nút Tắt bị ẩn với một số câu | Cho phép lệch prototype | Theo luật ba tầng của ADR 0024, máy chủ từ chối thao tác ấy |
| Trạng thái nhiệm vụ chỉ có bút chì (#21) | Cho phép lệch prototype | Máy chủ từ chối thao tác khác |
| Khi nào hiện "Tắt" | Chỉ khi câu đã có lời của xã: lời sửa đè, hoặc câu xã tự thêm | Q2: khi tắt thì dùng lời gốc. Câu chưa có lời của xã thì không có gì để tắt |
| "Số cán bộ đã ghép nối", "Cán bộ chưa ghép nối" | Thôi là "?" | Danh sách "Ai đã ghép nối" ghép với danh bạ cán bộ (`GET /api/v1/staff-directory`, đã có). Hai mục này không nằm trong danh sách việc backend của lô 2, nên lô 2 không có dòng nào phải đóng |
| Đơn thư trong bảng thời hạn | Chỉ có dòng thời hạn mặc định | Prototype có 3 loại việc. Q4 chỉ thêm dòng riêng cho phản ánh, văn bản và nhiệm vụ |

## Lô câu trả lời thứ tư — chấm ảnh vòng 4 (08/10/2026)

Chủ dự án trả lời ba câu hỏi sau khi chấm ảnh vòng 4 của menu Cấu hình, 08/10/2026. Lựa chọn ghi
nguyên văn trong ngoặc kép.

| # | Câu hỏi | Chốt | Nghĩa |
|---|---|---|---|
| Q10 | Kênh Zalo, ô "Sắp đến hạn: nhắc trước". Spec 11 §2 là ô chọn 1–14 ngày; lô 2 Q1 #7 đã chốt chỉ đọc, lấy từ cột SLA, một nguồn với chuông | Lần đầu **"theo prototype"**. Hỏi xác nhận vì câu này đảo Q1 #7, chủ dự án chọn **"Đúng prototype: sửa được, lưu riêng cho Zalo"** | **Thay** lô 2 Q1 #7. Xem các hệ quả dưới bảng |
| Q11 | Bốn ô loại nhắn ngoài spec: (văn bản/phản ánh) × (đôn đốc / chưa cử người) | **"theo prototype"** | Không thêm dòng; chỉ 18 dòng của spec. Dòng "Văn bản, đơn thư quá hạn" bật/tắt cùng lúc các loại quá hạn + chưa cử người + đôn đốc của miền văn bản–đơn thư; dòng "Phản ánh quá hạn" làm như vậy cho miền phản ánh. Miền nhiệm vụ giữ hai dòng riêng của spec: "Việc bị đôn đốc lên cấp trên" và "Bộ phận chưa cử người làm" |
| Q12 | "Kiểm tra kết nối" khi xã dùng bot chung | **"Ẩn khi dùng bot chung"** | Ẩn, như Đăng ký webhook và Quay về bot chung |

Hệ quả của Q10:

| Điểm | Hệ quả |
|---|---|
| Lưu ở đâu | service-comms lưu `due_soon_days` theo từng xã: migration + API + bộ phát Zalo áp dụng giá trị này |
| Hai ngưỡng | Ngưỡng "sắp đến hạn" của tin Zalo có thể **khác** ngưỡng của chuông (cột "Sắp đến hạn khi còn" của SLA). Chủ dự án **chấp nhận** độ lệch này khi xác nhận Q10 |
| Q1 #7 | Dòng #7 của bảng Q1 giữ nguyên chữ, đánh dấu đã bị thay — không xoá (ADR không xoá) |

### Phiên chính tự quyết theo nguyên tắc "đúng prototype" (lô 3 Q7) — không phải câu trả lời của chủ dự án

| Điểm | Quyết | Căn cứ |
|---|---|---|
| Ô "Nhóm" ở biểu mẫu thêm câu Lời hệ thống | Không có. Nhóm lấy từ tiền tố mã (`chung.` / `phan-anh.` / `giai-ngan.`), như prototype | Lô 3 Q7 **"Bỏ hết, đúng prototype"** |

## Lô câu trả lời thứ năm — các câu còn mở (08/10/2026)

Chủ dự án trả lời bảy câu còn mở của menu Cấu hình, 08/10/2026. Lựa chọn ghi nguyên văn trong ngoặc kép.

**Luật thường trực của menu này.** Chủ dự án dặn kèm: **"lưu ý phải tham khảo prototype nhé"**. Mọi
việc tiếp theo ở menu Cấu hình đối chiếu prototype
`../vigov-require/apps/admin/src/components/admin/ConfigWorkspace.tsx` và các component cùng thư mục
trước khi quyết. Luật cứng và §*Giữ bất kể spec* vẫn thắng.

| # | Câu hỏi | Chốt | Nghĩa |
|---|---|---|---|
| Q13 | Kênh Zalo, ô "Sắp đến hạn: nhắc trước". Prototype tự mâu thuẫn: spec 11 nói số ngày điều khiển cả chuông; spec 08 nói cột SLA theo giờ của từng dòng điều khiển chuông | **"Zalo thu hẹp, chuông giữ cột SLA"** | Ô sửa được 1–14 ngày như prototype. Tin Zalo "sắp đến hạn" chỉ gồm hồ sơ có **hạn đã lưu** nằm trong số ngày ấy. Chuông và danh sách giữ cột SLA. **Làm rõ** lô 4 Q10 — xem hệ quả dưới bảng |
| Q14 | Nút "Gieo thời hạn mặc định" khi bảng đã có dòng mà máy chủ báo thiếu dòng mặc định | **"Hiện nút CHỈ khi máy chủ báo thiếu"** | Bảng đã có dòng và đủ dòng mặc định: không có nút (lô 3 Q7 (d) giữ). Máy chủ báo thiếu: hiện nút |
| Q15 | Ngưỡng webhook Zalo | **"Giữ 600 lần/phút mỗi xã"** | Con số 600/phút (`core/ratelimit/ratelimit.go:96`, `ZaloBotWebhookLimit`) thôi là số tạm; đóng điều kiện dừng luật 13 về ngưỡng của tuyến này. Đếm: webhook của **bot riêng xã** (afe3be2d) dùng `PublicHostIPKey` — theo xã + host + mạng khách, tức "mỗi xã"; webhook **bot chung** vẫn `WebhookIPKey` theo mạng khách (`ratelimit.go:312-318`). Chú thích "PROVISIONAL" ở `core/ratelimit/ratelimit.go:89-94` đã cũ — sửa ở lượt đụng tệp ấy |
| Q16 | Quyền thay bot của xã | **"Giữ admin.lookup"** | ADR 0074 #6 giữ nguyên |
| Q17 | Lỗi đọc đảo cờ `dang_dung`/`la_mac_dinh` ở loại nhiệm vụ / mức ưu tiên (lỗi từ `14e4819b` 22/09, đã sửa ở `9fff0136`) | **"tuân thủ prototype nhé"** | Xem đoạn *Cách phiên chính hiểu Q17* dưới bảng |
| Q18 | Nhắc việc cho đơn thư | **"Làm, theo prototype"** | Nhắc việc đơn thư dùng **hạn đã lưu** và dòng SLA mặc định của đơn thư; gửi bằng các loại `DOCUMENT_*`, như dòng Zalo gộp "Văn bản, đơn thư …" của prototype. Không tính lại hạn nào. Đóng luật 10 điều kiện dừng #1 cho điểm này |
| Q19 | Câu chữ do builder tự đặt | **"Đối chiếu prototype, thay chỗ prototype có câu"** | Chỗ prototype có câu: lời prototype thắng, nguyên văn. Chỗ prototype không có: giữ lời hiện tại và liệt kê lại |

Hệ quả của Q13:

| Điểm | Hệ quả |
|---|---|
| Hợp đồng | Proto `StaffNotification` thêm `due_soon_items {code, deadline}`; các bên phát gửi kèm danh sách này |
| comms | Lưu `due_soon_days` (lô 4 Q10) và **lọc** `due_soon_items` theo số ngày ấy. Không tính và không đổi hạn (luật 10 bất biến 2) |
| Lô 4 Q10 | Không bị thay. Q13 nói rõ phạm vi: `due_soon_days` chỉ thu hẹp tin Zalo, không quyết hồ sơ nào là "sắp đến hạn" ở chuông. Độ lệch hai ngưỡng chủ dự án đã chấp nhận ở Q10 giữ nguyên |

### Cách phiên chính hiểu Q17 — không phải câu trả lời của chủ dự án

Chủ dự án chỉ trả lời **"tuân thủ prototype nhé"**. Phiên chính hiểu như sau:

| Điểm | Cách hiểu |
|---|---|
| Prototype có gì | Không có thao tác sửa dữ liệu hàng loạt; chỉ có màn Danh mục |
| Hệ quả | Không chạy truy vấn, không đổi dữ liệu. Xã tự kiểm và sửa cờ ở màn Danh mục |
| Luật 7 | Giữ: không bản ghi nào bị sửa ngoài thao tác của cán bộ xã |


### Bổ sung 08/10/2026 — Q18 đọc theo prototype, và Q13 không cần lập lịch

| Điểm | Ghi nhận |
|---|---|
| Q18 — hạn của đơn thư | Hạn đơn thư hiện **chưa bao giờ được lưu** (`service-documents/migrations/0006_citizen_letter.sql:72-75,215-216`; ADR 0064 #3–#5 còn mở về ngày lịch/giờ làm việc cho khiếu nại, tố cáo). Hỏi lại, chủ dự án trả lời **"theo prototype"**. Prototype (`../vigov-require/apps/admin/src/components/documents/PetitionDetailDrawer.tsx:519-523, 329`): ô **"Hạn xử lý" do cán bộ tự đặt** khi sửa đơn, được để **"Không đặt"** — không tự tính. Vậy: hạn đơn thư là ngày cán bộ nhập (lưu tại hành vi đặt, luật 10 bất biến 2); **nhắc việc chỉ chạy với đơn đã có hạn**; không tính hạn theo luật, nên câu ngày lịch/giờ làm việc của ADR 0064 không bị quyết ở đây. Màn đơn thư thuộc menu Văn bản & Đơn thư (ADR 0078) |
| Q13 — tin tổng hợp mỗi ngày | Ô "Nhắc trước" tính theo **ngày** (1–14) và bản tin sắp đến hạn chạy mỗi ngày: một hồ sơ có hạn trong N ngày luôn nằm trong ít nhất một bản tin trước hạn, nên comms chỉ lọc lúc nhận, **không cần lập lịch gửi riêng**. Ngưỡng Zalo chỉ thu hẹp được ngưỡng của chuông (hồ sơ ngoài ngưỡng chuông không tới comms) — ghi rõ dưới ô |
| Q18 — cột lưu hạn | Một ô "Hạn xử lý" duy nhất, ghi vào cột hạn của **giai đoạn hiện tại** của đơn (`processing_due_at` trước thụ lý, `resolution_due_at` từ thụ lý; CHECK 0006:300-301 cấm hạn giải quyết trước thụ lý); đơn đã kết thúc từ chối 409; quyền `petition.create` như prototype (`router.py:430-433`); không tự điền hạn từ dòng SLA (khác prototype `service.py:708`, theo dòng Q18 trên). Sau thụ lý ô hiện "Không đặt" tới khi cán bộ đặt hạn giải quyết |

## Lô câu trả lời thứ sáu — câu chữ còn lại (08/10/2026)

| # | Câu hỏi | Chủ dự án trả lời | Hệ quả |
|---|---|---|---|
| 1 | Lựa chọn trống của ô "Sắp đến hạn: nhắc trước" (prototype chỉ có số ngày) | **"Theo chuông"** | Trống = không thu hẹp, `due_soon_days` null |
| 2 | Lời dẫn "Nhắc khi nào" (câu prototype nói số Zalo luôn bằng số trên màn — sai từ Q13) | **Câu mới** | "Tin Zalo “sắp đến hạn” chỉ gồm những hồ sơ cái chuông và danh sách “Sắp đến hạn” đang báo, nên không hồ sơ nào vào Zalo mà không có trên màn hình." |
| 3 | Toast khi bật/tắt một dòng thành công (Danh mục, Trường bản đồ, Lời hệ thống) | **"Bỏ, đúng prototype"** | Thành công: công tắc đổi là đủ; lỗi vẫn toast |
| 4 | Đăng ký webhook thất bại | **Câu prototype khi lỗi do mã bot, câu riêng khi lỗi khác** | Mã bot sai → "Chưa đăng ký được. Kiểm tra lại mã bot." (`ZaloChannelPanel.tsx:479`); lỗi mạng/Zalo → "Chưa đăng ký được." + lý do thật |

## Sửa đổi

- 08/10/2026 — Lô thứ năm Q18, câu "không tự điền hạn từ dòng SLA", **đảo cho đơn thư**: hạn tự điền lúc vào sổ theo loại đơn, cán bộ vẫn sửa được. Xem `0084-don-thu-bam-prototype-08-10.md` mục 3.
