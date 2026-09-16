---
id: 0007-sla-working-hours
tier: T1
source: CURATED
owner: domain
derived_from_commit: null
expires: null
owns_facts:
  - "đơn vị đếm hạn xử lý là giờ làm việc, không phải ngày làm việc"
  - "lịch làm việc và ngày nghỉ lễ là cấu hình theo xã"
  - "cách tính hạn xử lý từ mốc tiếp nhận"
---

# 0007. Hạn xử lý đếm bằng GIỜ LÀM VIỆC

**Trạng thái:** đã chốt · **Ngày:** 2026-09-16 · **Đóng câu hỏi mở #6**

## Bối cảnh

Luật 10 và `ubiquitous-language.md` ban đầu ghi hạn xử lý đếm bằng **ngày làm việc**. Nhưng
bộ đặc tả giao diện `docs/ui-ux/14-cau-hinh.md §8` — dựng từ prototype đang chạy tại một xã
thật — đếm bằng **giờ làm việc**, với các mốc nhỏ hơn một ngày:

| Loại việc | Lĩnh vực | Tiếp nhận | Xử lý xong |
|---|---|---|---|
| Phản ánh | An ninh trật tự | 2 giờ | 16 giờ |
| Phản ánh | Điện | 2 giờ | 24 giờ |
| Phản ánh | Hạ tầng giao thông | 8 giờ | 168 giờ |
| Văn bản đến | Mặc định | 8 giờ | 40 giờ |

Hai giờ cho an ninh trật tự **không quy đổi sang ngày làm việc được** mà không mất ý nghĩa.
Làm tròn lên một ngày là nới cam kết với dân gấp bốn lần.

## Quyết định

**Đơn vị đếm hạn là giờ làm việc.** Danh mục lĩnh vực và bảng SLA theo `docs/ui-ux/14-cau-hinh.md §8`
làm giá trị khởi tạo, nhưng **mọi con số là cấu hình theo xã**, không viết cứng trong mã.

| # | Quyết định |
|---|---|
| 1 | `sla_deadline` lưu **một lần** lúc tiếp nhận (luật 10 #2), kiểu `TIMESTAMPTZ` |
| 2 | Đếm theo **giờ làm việc**: trừ đêm, cuối tuần, ngày nghỉ lễ, và nghỉ trưa nếu xã khai |
| 3 | Lịch làm việc là cấu hình theo xã: `lich_lam_viec(thu, gio_bat_dau, gio_ket_thuc)` |
| 4 | Ngày nghỉ lễ là cấu hình theo xã: `ngay_nghi_le(ngay, ten)` — lễ quốc gia và lễ địa phương |
| 5 | Mốc bắt đầu đếm là **tiếp nhận**, không phải lúc phân công (`ubiquitous-language.md`) |
| 6 | Đổi cấu hình SLA **không hồi tố** — chỉ áp dụng cho hồ sơ tiếp nhận sau thời điểm lưu |
| 7 | Quá hạn vẫn là giá trị **suy ra** từ `sla_deadline` vs hiện tại (luật 10 #3) — không có cột |

## Vì sao KHÔNG hồi tố

Hạn là cam kết đã nói với dân lúc tiếp nhận. Đổi cấu hình rồi tính lại hạn của hồ sơ cũ là
sửa một cam kết đã phát ra — và làm mọi báo cáo đúng hạn/quá hạn của kỳ trước đổi theo.
Prototype đã ghi đúng quy tắc này (`14-cau-hinh.md §12.3`); giữ nguyên.

## Lỗ hổng đặc tả phải lấp trước khi viết mã

`docs/ui-ux` nói hạn tính theo giờ làm việc nhưng **không có màn hình nào để khai lịch làm
việc và ngày nghỉ lễ**, cũng không có dữ liệu mẫu. Những câu chưa có trả lời:

| Câu hỏi | Vì sao phải chốt |
|---|---|
| Giờ hành chính của xã là mấy giờ tới mấy giờ? | Không có thì không tính được |
| Nghỉ trưa có trừ khỏi giờ làm việc không? | Lệch 1–1,5 giờ mỗi ngày, cộng dồn thành lệch hạn thật |
| Thứ Bảy có làm không? | Một số xã trực sáng thứ Bảy |
| Lễ địa phương (lễ hội, ngày truyền thống) có tính không? | Khác nhau theo từng xã |
| Hồ sơ tiếp nhận ngoài giờ thì đếm từ lúc nào? | Đề xuất: từ đầu giờ làm việc kế tiếp |

→ Phải bổ sung tab khai lịch làm việc vào `/cau-hinh` trước khi phát hành module phản ánh.

## Hệ quả

- **Dễ hơn:** cam kết với dân đúng mức thật — 2 giờ cho việc gấp, không làm tròn lên 1 ngày
- **Khó hơn:** hàm tính hạn phức tạp hơn đếm ngày; cần test cho ngày lễ, nghỉ trưa, tiếp nhận
  ngoài giờ, và hồ sơ vắt qua nhiều tuần
- **Phải trả sau:** nếu một xã đổi giờ hành chính, hồ sơ đang mở vẫn giữ hạn cũ (không hồi
  tố) — báo cáo phải nêu được điều này khi có thanh tra

→ Luật 10: `.claude/rules/critical/10-citizen-commitment.md`
→ Kỹ năng: `skills/petition-lifecycle`
