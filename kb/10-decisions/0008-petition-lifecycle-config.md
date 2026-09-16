---
id: 0008-petition-lifecycle-config
tier: T1
source: CURATED
owner: domain
derived_from_commit: null
expires: null
owns_facts:
  - "ai được đóng phiếu phản ánh và điều kiện đóng"
  - "quy tắc mở lại phiếu sau đánh giá thấp"
  - "công dân phải đăng nhập mới gửi được phản ánh, và ẩn danh nghĩa là gì"
---

# 0008. Vòng đời phiếu phản ánh là cấu hình theo xã

**Trạng thái:** đã chốt · **Ngày:** 2026-09-16 · **Đóng câu hỏi mở #7 và #8**

## Bối cảnh

Ba câu về vòng đời phiếu phản ánh chưa chốt: ai được đóng phiếu, nghiệm thu ảnh có bắt buộc
không (#7), và công dân có được mở lại phiếu đã đóng không (#8).

Bộ đặc tả `docs/ui-ux/09-phan-anh-nguoi-dan.md` trả lời sẵn cả ba, nhưng đó là cách vận hành
của **một xã mẫu**. Hệ thống phục vụ 200+ xã có quy mô rất khác nhau: xã 8 cán bộ và xã 40
cán bộ không thể chung một quy tắc cứng.

## Quyết định

**Không viết cứng quy tắc nào. Quyền quyết định ai làm được gì; cấu hình theo xã quyết định
điều kiện.**

### Đóng phiếu (#7)

| # | Quyết định |
|---|---|
| 1 | Quyền `feedback.resolve` quyết định ai đóng được phiếu |
| 2 | Cờ theo xã `bat_buoc_nguoi_khac_dong` (mặc định `false`) — bật thì người trực tiếp xử lý không tự đóng phiếu của mình |
| 3 | Cờ theo xã `bat_buoc_anh_nghiem_thu` (mặc định `true`) — thiếu ảnh sau xử lý thì không đóng được |
| 4 | Chặn ở **tầng service**, không chỉ ở giao diện (luật 5 cấm #1) |
| 5 | Đóng phiếu bắt buộc ghi **kết quả dân đọc được** (luật 10 #6) |

**Vì sao không hard-code "người xử lý không tự đóng":** xã nhỏ có khi chỉ một người phụ trách
cả lĩnh vực. Hard-code thì xã đó không dùng được, mà nới ra sau lại là đổi luật trên hồ sơ đã
đóng. Để cờ, xã nào cần siết thì siết.

### Mở lại phiếu (#8)

| Khoá cấu hình | Mặc định | Ý nghĩa |
|---|---|---|
| `cho_phep_mo_lai` | `true` | Có cho mở lại sau đánh giá thấp không |
| `nguong_sao_mo_lai` | `2` | Từ mấy sao trở xuống thì mở lại |
| `so_lan_mo_lai_toi_da` | `1` | Chặn vòng lặp vô hạn |
| `tinh_lai_han_khi_mo_lai` | `true` | Hạn mới tính từ thời điểm mở lại |

Đổi `cho_phep_mo_lai` **không hồi tố**: phiếu đã đóng theo luật cũ giữ nguyên luật cũ. Nếu
không thì báo cáo quá khứ đổi theo — dạng sai lệch mà thanh tra sẽ hỏi.

**Vì sao cho mở lại:** không cho thì dân gửi phiếu mới cho cùng một vụ việc, và số liệu đếm
trùng. Vòng đời một chiều nghe gọn hơn nhưng đẩy sai lệch sang chỗ khác.

### Công dân gửi phản ánh

| # | Quyết định |
|---|---|
| 1 | **Phải đăng nhập mới gửi được.** Không có endpoint ghi công khai |
| 2 | Khai `CitizenOnly()` (luật 5 #1), qua phiên Mini App đã xác thực (ADR 0005 lớp "Phiên") |
| 3 | Có tuỳ chọn **gửi ẩn danh** — cờ `an_danh` trên phiếu |
| 4 | Hai endpoint đọc nội dung công khai (`nội dung Mini App`, `danh bạ`) khai `Public("nội dung xã chủ động công bố")` |

**Ẩn danh nghĩa là ẩn với ai:**

| Với | Thấy danh tính người gửi? |
|---|---|
| Cán bộ xử lý thông thường | **Không** — giao diện hiện "Gửi ẩn danh" |
| Trang công khai, Mini App | **Không** |
| Hệ thống (lưu trong CSDL) | **Có** — vẫn lưu `cong_dan_id` |
| Truy trách nhiệm khi cần | **Có**, và lần xem đó **tự nó bị ghi vết** (luật 6 #7) |

**Ẩn danh không phải là không lưu danh tính.** Không lưu thì không chống được spam, dân không
tra cứu lại được phiếu của mình, và phiếu vu khống thành vô danh tuyệt đối — thứ một cơ quan
nhà nước không nhận được.

## Vì sao đường ghi phải xác thực

`docs/ui-ux/09 §13` đề xuất `POST /api/cong/phan-anh` là endpoint công khai. Bác bỏ: phiếu
phản ánh là **hồ sơ hành chính** có thời hạn xử lý và có người chịu trách nhiệm. Đường ghi
không xác thực là cửa cho spam hàng loạt và phiếu giả mạo, mà mỗi phiếu giả tạo ra một cam
kết thật mà xã phải xử lý.

ADR 0005 đã có sẵn cơ chế phiên cho Mini App, nên chi phí thêm là một bước OTP — không phải
hạ tầng mới.

## Hệ quả

- **Dễ hơn:** mỗi xã vận hành theo quy mô của mình mà không cần nhánh mã riêng
- **Khó hơn:** mọi đường đóng phiếu và mở lại phải đọc cấu hình xã; thêm test cho từng tổ hợp cờ
- **Phải trả sau:** cấu hình không hồi tố nghĩa là hai phiếu cùng xã có thể theo hai luật khác
  nhau tuỳ thời điểm tiếp nhận. Báo cáo phải nêu được điều đó

→ Luật 4: `.claude/rules/critical/4-citizen-isolation.md`
→ Luật 10: `.claude/rules/critical/10-citizen-commitment.md`
→ Kỹ năng: `skills/petition-lifecycle`
