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
  - "hồ sơ tiếp nhận ngoài giờ làm việc thì đếm từ đầu ca làm việc kế tiếp"
  - "ngày làm bù rơi vào ngày đã có ca làm việc là lỗi cấu hình, hệ thống từ chối"
---

# 0007. Hạn xử lý đếm bằng GIỜ LÀM VIỆC

**Trạng thái:** đã chốt · **Ngày:** 2026-09-16 · **Đóng câu hỏi mở #6**
**Bổ sung 2026-09-20:** hai quyết định 8 và 9 ở cuối tệp — lấp lỗ hổng thứ năm, và chốt hình
dạng của ngày làm bù.

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

Bốn câu còn lại đều là **dữ liệu**, không phải quy tắc: mỗi câu trả lời là một số dòng trong
`lich_lam_viec` hoặc `ngay_nghi_le`, nên không có mã nào phải tự quyết chúng
(`service-identity/migrations/0006_lich_lam_viec.sql` ghi từng câu rơi vào đâu).

→ Phải bổ sung tab khai lịch làm việc vào `/cau-hinh` trước khi phát hành module phản ánh.

### Câu đã trả lời — giữ nguyên câu hỏi, đừng đọc thành câu còn mở

| Câu hỏi | Trả lời | Ngày |
|---|---|---|
| Hồ sơ tiếp nhận ngoài giờ thì đếm từ lúc nào? | Từ **đầu ca làm việc kế tiếp** — quyết định 8 | 2026-09-20 |

Đây là câu duy nhất trong năm câu **không phải là dòng dữ liệu**: nó là một quy tắc nằm trong
hàm tính hạn, và trước 20/09 chưa ai trả lời nên hàm ấy không viết được.

## Hệ quả

- **Dễ hơn:** cam kết với dân đúng mức thật — 2 giờ cho việc gấp, không làm tròn lên 1 ngày
- **Khó hơn:** hàm tính hạn phức tạp hơn đếm ngày; cần test cho ngày lễ, nghỉ trưa, tiếp nhận
  ngoài giờ, và hồ sơ vắt qua nhiều tuần
- **Phải trả sau:** nếu một xã đổi giờ hành chính, hồ sơ đang mở vẫn giữ hạn cũ (không hồi
  tố) — báo cáo phải nêu được điều này khi có thanh tra

## Bổ sung 2026-09-20 — quyết định 8: tiếp nhận ngoài giờ đếm từ đầu ca kế tiếp

**Trạng thái:** đã chốt · **Ngày:** 2026-09-20 · **Lấp lỗ hổng thứ năm ở trên**

| # | Quyết định |
|---|---|
| 8 | Mốc `count_from` rơi **ngoài mọi ca làm việc** thì đồng hồ bắt đầu chạy từ **đầu ca làm việc kế tiếp** của chính xã đó. Không ca nào tính một phần |

Bản thân ADR này đã **đề xuất** đúng phương án ấy mà chưa chốt; hợp đồng gRPC
(`AdvanceWorkingHours`) cũng cố ý không chốt thay. Nay người dùng chốt: đó là quy tắc.

**Trong thực tế nghĩa là gì** — phần người viết hàm cần:

| Hồ sơ tiếp nhận lúc | Đồng hồ bắt đầu chạy lúc |
|---|---|
| 22:00 thứ Sáu, tuần bình thường | Thứ Hai, đầu ca sáng (ví dụ 07:30) |
| 22:00 thứ Sáu, nhưng thứ Hai là ngày nghỉ lễ | Đầu ca của ngày làm việc kế tiếp **sau** ngày lễ |
| 22:00 thứ Sáu, nhưng thứ Bảy có dòng `ngay_lam_bu` | Đầu ca **làm bù** của thứ Bảy — sớm hơn |
| 12:00 giữa nghỉ trưa | Đầu ca chiều của chính ngày hôm đó |

Nói cách khác: "ca làm việc kế tiếp" là ca kế tiếp **theo lịch thật của xã** — tuần làm việc,
trừ ngày nghỉ lễ, cộng ngày làm bù — chứ không phải "ngày làm việc kế tiếp" hay "08:00 hôm sau".

**Hệ quả người dùng chấp nhận:** con số SLA giữ đúng nghĩa đen của nó — *bấy nhiêu GIỜ LÀM
VIỆC* — và **không có ca nào được tính một phần** cho khoảng thời gian cơ quan đóng cửa. Một
phản ánh gửi 22:00 thứ Sáu không được cộng chút nào cho đêm thứ Sáu hay cuối tuần: hạn của nó
đúng bằng hạn của một phản ánh gửi 07:30 thứ Hai. Đó là điều phải nói được khi dân hỏi vì sao
hai phiếu gửi cách nhau hai ngày lại cùng một hạn.

Mốc tiếp nhận vẫn là mốc tiếp nhận (quyết định 5): thứ dịch chuyển là **điểm bắt đầu đếm**,
không phải mốc nghiệp vụ được ghi lại. Và hạn vẫn lưu **một lần** lúc tiếp nhận (quyết định 1).

## Bổ sung 2026-09-20 — quyết định 9: ngày làm bù rơi vào ngày vốn đã làm việc

**Trạng thái:** đã chốt · **Ngày:** 2026-09-20

`ngay_lam_bu` sinh ra cho **ngày xã vốn KHÔNG làm việc** — những ngày hoán đổi theo thông báo
hằng năm của Thủ tướng. Một dòng làm bù rơi vào ngày mà **thứ của nó đã có ca** trong
`lich_lam_viec` đọc được theo hai cách trái ngược: ngày làm bù **THAY** giờ của ngày đó, hay
**CỘNG THÊM** vào giờ của ngày đó.

| # | Quyết định |
|---|---|
| 9 | Xã không cần việc đó. Một dòng `ngay_lam_bu` rơi vào ngày đã có ca là **LỖI CẤU HÌNH**, hệ thống **từ chối** — không chọn cách đọc nào |

**Vì sao từ chối chứ không chọn một cách đọc:**

- Nới ra về sau là **thêm tính năng**: nếu có xã thật cần một ca lẻ trong ngày làm việc bình
  thường, hỏi người dùng lấy quy tắc rồi bỏ lệnh từ chối — cộng thêm, không phá gì.
- **Gỡ một hạn đã tính từ giờ bị đếm hai lần thì không**: hạn đã phát ra cho dân, đã vào báo
  cáo, và luật 10 bất biến 2 cấm tính lại.
- Giờ đếm hai lần làm hạn **SỚM hơn** thực tế — đúng chiều báo cáo một cơ quan là **trễ trong
  khi nó không trễ**. Đây là cùng hướng hỏng mà hai ca chồng nhau trong `lich_lam_viec` gây ra.

Cùng nguyên tắc với ngày có mặt ở **cả** `ngay_nghi_le` lẫn `ngay_lam_bu`: không có quy tắc ưu
tiên ngầm, vì một bên thắng lặng lẽ làm một dòng cấu hình đang hiện trên màn hình trở thành vô
nghĩa mà không ai thấy.

→ Lược đồ ba bảng và lý do từng ràng buộc: `service-identity/migrations/0006_lich_lam_viec.sql`
→ Hợp đồng đọc lịch và danh sách lệnh từ chối: `proto/vigov/identity/v1/identity.proto`,
  `AdvanceWorkingHours`

→ Luật 10: `.claude/rules/critical/10-citizen-commitment.md`
→ Kỹ năng: `skills/petition-lifecycle`
