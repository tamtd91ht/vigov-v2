---
id: 0041-bang-bao-cong-dan-theo-chuyen-trang-thai
tier: T1
source: CURATED
owner: architecture
derived_from_commit: 43250a9
expires: null
owns_facts:
  - "phiếu phản ánh chuyển VÀO trạng thái nào thì người dân được báo, trạng thái nào thì không — và vì sao"
  - "những gì không bao giờ được đi trong lời báo cho người dân về phiếu phản ánh"
  - "vì sao luật 10 bất biến 5 đọc là 'mọi chuyển trạng thái trong bảng báo', không phải 'mọi chuyển trạng thái'"
---

# 0041. Báo người dân theo bảng chuyển trạng thái, không phải mọi bước

**Trạng thái:** đã chốt · **Ngày:** 2026-09-24 · **Người dùng chốt** (cổng xác nhận của lượt
`/develop-feature Phản ánh người dân`, chọn theo khuyến nghị của chuyên gia nghiệp vụ) · Dựng ở
commit `43250a9`

## Bối cảnh

Ba nơi nói ba điều khác nhau về việc khi nào người dân được báo:

| Nơi | Nói |
|---|---|
| Luật 10 bất biến 5 (trước ADR này) | **Mọi** chuyển trạng thái đều báo người dân |
| `skills/petition-lifecycle` (trước ADR này) | Chỉ bước 1, 4, 6 báo |
| Mã trước `43250a9` | Báo khi vào `da-tiep-nhan`, `dang-xu-ly`, `da-dong` |

Ba câu trả lời cho một lời cam kết với người dân. Câu nào đúng thì hai câu kia là lời hứa sai.

## Các phương án

| Phương án | Được | Mất |
|---|---|---|
| A — báo **mọi** chuyển trạng thái | Đơn giản, không phải chọn | Nhiễu: `dang-phan-loai` gần như ngay lập tức nối tiếp bằng giao việc — hai tin trong vài phút. Mỗi tin ZNS **tốn tiền**. Kênh ồn là kênh người dân tắt |
| B — giữ bước 1, 4, 6 | Ít tin | Im lặng ở các nhánh kết thúc (`khong-tiep-nhan`, `chuyen-cap-tren`) và ở lúc mời xác nhận — đúng chỗ người dân cần biết nhất |
| **C (chọn)** — bảng chốt theo **trạng thái đích**, mỗi dòng mang đúng điều người dân cần | Mỗi tin trả lời một câu hỏi thật: mã tra cứu, **ai** đang giữ, **đến bao giờ**, **kết quả** | Phải giữ một bảng; thêm trạng thái mới phải quyết có báo hay không |

## Quyết định

### Báo khi phiếu chuyển VÀO

| Trạng thái đích | Lời báo mang |
|---|---|
| `da-tiep-nhan` | Mã tra cứu · hạn tiếp nhận · nhắc việc khẩn gọi 113/114/115 |
| `da-chuyen-xu-ly` | Đã giao bộ phận chuyên môn · hạn xử lý xong |
| `cho-dan-xac-nhan` | Mời mở ứng dụng xem kết quả, xác nhận, đánh giá |
| `da-dong` | Đã đóng · xem kết quả trong ứng dụng |
| `khong-tiep-nhan` | Lý do · nơi liên hệ tiếp — **chưa có tuyến dẫn tới** |
| `chuyen-cap-tren` | Cơ quan tiếp nhận · việc cần làm tiếp — **chưa có tuyến dẫn tới** |

Tương lai, khi dựng: **mở lại phiếu** và **gia hạn** cũng báo (hạn mới + lý do).

### Không báo

`dang-phan-loai` (nội bộ, và ngay sau đó là giao việc), `dang-xu-ly` (lời "bộ phận nào, đến bao giờ"
đã nói lúc `da-chuyen-xu-ly` — nói lại là nhiễu), `da-xu-ly` (nối tiếp ngay bằng `cho-dan-xac-nhan`,
tin đó mang lời mời).

### Không bao giờ gửi

Tên hay số điện thoại cán bộ · ghi chú nội bộ · lịch sử luân chuyển · nội dung phản ánh · ảnh. Phiếu
thuộc lĩnh vực `can-bo` (phản ánh **về** cán bộ): **chỉ** mã tra cứu và trạng thái — không hạn, không bộ
phận, không kết quả. Lý do: tin đi qua bên thứ ba (ZNS) và sổ thông báo mà đồng nghiệp của người bị
phản ánh đọc được; "bộ phận nào đang giữ" chính là chi tiết luân chuyển lộ ra ai đang xử lý đơn.

### Nguồn duy nhất của bảng

Bảng là `loiNhanChoDan` trong `service-petitions/internal/domain/xu_ly_phan_anh.go`. `BaoChoDan` suy
ra từ nó, không từ danh sách thứ hai. ADR này giữ **vì sao**; mã giữ **dòng nào**. Không chép bảng
sang skill hay luật (luật 9).

Luồng tiếp nhận nay ghi sự kiện outbox **cùng giao dịch** với dòng phiếu và mục nhật ký
(`service-petitions/internal/app/gui_phan_anh.go:405-414`): không có phiếu đã ghi mà lời báo chưa được
ghi nhận.

## Vì sao

1. **Nhiễu và tiền.** Báo mọi bước gửi tin vô nghĩa và tính phí ZNS từng tin.
2. **Điều người dân cần là cam kết**, không phải nhật ký nội bộ: mã tra cứu, đơn vị nào giữ, hạn đến
   bao giờ, kết quả ra sao.
3. **Im lặng ở nhánh kết thúc là cách niềm tin chết** (luật 10, "why #5"). Không tiếp nhận hay chuyển
   cấp trên mà không nói gì thì người dân không phân biệt được với bị bỏ mặc.
4. **Tối thiểu hoá dữ liệu cá nhân gửi bên thứ ba** (luật 3 bất biến 6).

## Hệ quả

- Luật 10 bất biến 5, cấm #4, điều kiện dừng #3 nay trỏ vào bảng này. Chuyển vào một trạng thái
  **trong bảng** mà không có lời báo vẫn bị cấm. **Đổi bảng** — thêm, bớt dòng — là điều kiện dừng.
- Thêm trạng thái mới (vốn đã là điều kiện dừng #2 của luật 10, ADR 0027) phải quyết luôn nó có vào
  bảng hay không.

## Còn mở

| # | Việc | Ai |
|---|---|---|
| 1 | **Kết quả xử lý và tên bộ phận KHÔNG đi trong sự kiện.** `proto/vigov/petitions/v1/events.proto:186-195` cấm chuyển tiếp văn bản cán bộ viết; service chỉ giữ `bo_phan_id`, tên ở `identity`. Muốn mang theo là **đổi hợp đồng** và một quyết định luật 3 bất biến 6 | contract-designer + người dùng |
| 2 | `khong-tiep-nhan`, `chuyen-cap-tren`: lý do, cơ quan tiếp nhận, nơi đi tiếp chưa có cột nào giữ. Ai dựng tuyến phải quyết lưu ở đâu và cái nào được đi trong tin | người dựng tuyến |
| 3 | Mở lại và gia hạn không phải trạng thái đích riêng, nên bảng khoá theo trạng thái không biểu diễn được. Ai dựng phải mở rộng cơ chế, không thêm danh sách thứ hai | người dựng tuyến |
| 4 | **Chưa gửi được thật.** Chưa có relay outbox / client Kafka (luật 11 điều kiện dừng #1); chưa có mẫu ZNS theo xã (ADR 0018) | người dùng |
