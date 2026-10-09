---
id: 0087-gop-phieu-trung-la-lien-ket
tier: T1
source: CURATED
owner: domain
derived_from_commit: 8b06345e
expires: null
owns_facts:
  - "gộp phiếu phản ánh trùng = LIÊN KẾT phiếu phụ vào phiếu chính (merged_into), không đóng phiếu phụ, không thêm trạng thái; mỗi người dân giữ mã tra cứu và hạn đã lưu của phiếu mình (chủ dự án, 09/10/2026)"
  - "hạn của phiếu chính khi gộp lấy mốc SỚM HƠN của hai phiếu, chỉ dời sớm (chủ dự án, 09/10/2026)"
  - "phiếu phụ theo phiếu chính khi phiếu chính vào cho-dan-xac-nhan hoặc da-dong: cùng câu kết quả, mỗi người dân nhận lời báo riêng, tự xác nhận và tự đánh giá (chủ dự án, 09/10/2026)"
  - "người dân của phiếu phụ không bao giờ thấy nội dung, ảnh hay người gửi của phiếu chính (chủ dự án, 09/10/2026)"
  - "tách phiếu đã gộp: được, lý do bắt buộc, có vết; ai gộp: feedback.classify, cùng xã; phiếu lĩnh vực can-bo không bao giờ gộp (chủ dự án, 09/10/2026)"
  - "ngưỡng nghi trùng (mặc định 50 m, 7 ngày) là cấu hình theo xã, không phải hằng số (chủ dự án, 09/10/2026)"
  - "thống kê khi có phiếu gộp: 'Nhận vào' đếm mọi phiếu; đúng hạn, theo lĩnh vực, theo đơn vị chỉ đếm phiếu chính (vụ việc) (chủ dự án, 09/10/2026)"
---

# 0087. Gộp phiếu trùng là LIÊN KẾT vào phiếu chính, không phải đóng phiếu

**Trạng thái:** đã chốt · **Ngày:** 2026-10-09 · **Người quyết:** chủ dự án, 09/10/2026, trong phiên
chính (`/fix-web-admin --menu=phan-anh-nguoi-dan`), trả lời qua phiếu hỏi · **Chưa dựng.**
**Đóng** việc *"C-R3 gộp phiếu trùng — CHƯA QUYẾT"* của sổ `kb/90-ephemeral/tien-do/service-petitions.json`
(mục `vong-doi-phieu-phan-anh`) và câu D3 của `kb/50-doi-chieu/2026-10-02-feat-m8-multitenant-foundation-phan-anh.md:81`.
**Sửa** bảng báo của ADR 0041 (§Sửa đổi 09/10/2026 ở đó).

## Bối cảnh

Kho yêu cầu có gộp phiếu trùng (SRS M4.3.4; `../vigov-require/docs/spec/05-nghiep-vu.md:207`,
cột `merged_into_id`; nghi trùng trong 50 m, 7 ngày). Hai người dân phản ánh một ổ gà là **hai lời
cam kết** đã phát ra: hai mã tra cứu, hai hạn đã lưu (luật 10 bất biến 1, 2). Gộp sai cách thì một
trong hai cam kết bị huỷ ngầm, hoặc số liệu đếm một vụ việc hai lần.

## Các phương án

| Phương án | Được | Mất |
|---|---|---|
| A — đóng phiếu phụ, trỏ sang phiếu chính | Một phiếu đang chạy, đơn giản | Người dân của phiếu phụ bị "đóng" khi việc chưa xong — đóng mà không có kết quả đọc được (luật 10 bất biến 6); mã tra cứu của họ thành ngõ cụt |
| B — thêm trạng thái `da-gop` | Hiện rõ trên vòng đời | Thêm trạng thái là điều kiện dừng #2 của luật 10; chín trạng thái là danh sách cố định (ADR 0027) |
| **C (chọn)** — liên kết `merged_into`, phiếu phụ giữ vòng đời riêng và đi theo phiếu chính lúc kết thúc | Không ai mất mã, không ai mất hạn, không thêm trạng thái | Phải giữ quan hệ hai chiều khi phiếu chính đổi trạng thái; thống kê phải phân biệt phiếu và vụ việc |

## Quyết định

### 1. Mô hình

| Điểm | Quyết định |
|---|---|
| Quan hệ | Phiếu phụ mang `merged_into` → phiếu chính. **Không** đóng phiếu phụ |
| Trạng thái | "Đã gộp" là **liên kết**, **không** phải trạng thái. Không thêm trạng thái nào vào vòng đời (ADR 0027) |
| Mã tra cứu | Mỗi người dân **giữ mã của mình**. Mã đã cấp không cấp lại (luật 7 bất biến 3) |
| Hạn của phiếu phụ | Giữ **hạn đã lưu** của chính nó (luật 10 bất biến 2) |
| Hạn của phiếu chính | Lấy mốc **SỚM HƠN** của hai phiếu. Hạn **chỉ dời sớm**, cùng chiều quyết định C của ADR 0027 |

**Vì sao lấy mốc sớm hơn.** Vụ việc giờ gánh lời hứa với hai người. Người được hứa sớm hơn không được
bị kéo dài chỉ vì cán bộ gộp phiếu của họ vào một phiếu đến sau.

### 2. Khi phiếu chính kết thúc

| Điểm | Quyết định |
|---|---|
| Lúc phiếu chính vào `cho-dan-xac-nhan` hoặc `da-dong` | Các phiếu phụ **đi theo**, cùng **câu kết quả** |
| Lời báo | **Mỗi người dân nhận lời báo riêng** của phiếu mình (bảng ADR 0041) |
| Xác nhận, đánh giá | Mỗi người dân **tự xác nhận và tự đánh giá** phiếu của mình |

### 3. Lúc gộp — một lời báo

Người dân của phiếu phụ nhận **một** lời báo lúc gộp, ý: *"phiếu của anh/chị đã được ghép với phản ánh
cùng vụ việc, kết quả sẽ báo khi xử lý xong"*. Câu chữ cuối cùng do ADR 0041 sở hữu — xem §Sửa đổi
09/10/2026 ở đó.

### 4. Ranh giới người dân (luật 4)

Người dân của phiếu phụ **không bao giờ** thấy nội dung, ảnh hay người gửi của phiếu chính — và ngược
lại. Liên kết chỉ để cán bộ xử lý một vụ việc; nó không mở dữ liệu của người này cho người kia (luật 4
bất biến 1, 7).

### 5. Ai, ở đâu, tách lại

| Điểm | Quyết định |
|---|---|
| Ai gộp | Người giữ `feedback.classify` (`service-identity/migrations/0007_quyen_phan_loai_va_xem_day_du.sql:58`) |
| Phạm vi | **Cùng xã** — không có đường gộp xuyên xã (luật 1) |
| Lĩnh vực `can-bo` | **Không bao giờ** gộp. Phiếu về cán bộ mang quyền đọc riêng (`feedback.restricted`, ADR 0030); gộp vào phiếu thường là mở nó cho người không có quyền ấy |
| Tách | **Được**. **Lý do bắt buộc**. Gộp và tách đều **có vết** (luật 6), không sửa lịch sử (luật 7) |

### 6. Ngưỡng nghi trùng

Mặc định **50 m, 7 ngày**, là **cấu hình theo xã** (luật 1 bất biến 10) — không phải hằng số. Nghi
trùng chỉ **gợi ý** cho cán bộ; gộp luôn là hành vi của cán bộ.

### 7. Thống kê

| Chỉ số | Đếm |
|---|---|
| "Nhận vào" | **Mọi phiếu**, kể cả phiếu phụ — mỗi phiếu là một lần người dân phản ánh |
| Đúng hạn / trễ · theo lĩnh vực · theo đơn vị | **Chỉ phiếu chính** (vụ việc) |

Vì sao tách: "nhận vào" đo **tải** của kênh; đúng hạn và phân bổ đo **công việc**. Đếm phiếu phụ vào
đúng hạn là đếm một vụ việc hai lần — đúng thứ ADR 0008 #8 tránh. Định nghĩa từng ô của tab Báo cáo:
ADR 0053 §Sửa đổi 09/10/2026.

## Hệ quả

- Tách phiếu **không** kéo dài lại hạn phiếu chính: chiều "chỉ dời sớm" của §1 đọc theo nghĩa ấy
  (cách đọc của người ghi, chưa hỏi riêng).
- Lời báo lúc gộp **không** gắn với trạng thái đích, nên bảng khoá theo trạng thái của ADR 0041 không
  biểu diễn được — cùng hình dạng việc còn mở #3 ở đó (mở lại, gia hạn). Người dựng mở rộng cơ chế,
  không thêm danh sách thứ hai.
- Tuyến danh sách và tuyến đếm phải mang được điều kiện "chỉ phiếu chính" bằng **cùng vị từ** (ADR 0053 §1).

## Còn mở khi dựng — hỏi, không tự quyết

| # | Việc | Vì sao không tự chọn |
|---|---|---|
| 1 | Phiếu chưa phân loại (`han_xu_ly_xong` NULL) gộp với phiếu đã có hạn: mốc nào là "sớm hơn" | ADR 0028: NULL là "chưa có", không phải vô hạn hay 0 |
| 2 | Gộp được phiếu đang ở trạng thái nào (đã `cho-dan-xac-nhan`? đã đóng?) | Gộp vào phiếu đã đóng là sửa hồ sơ lưu trữ (luật 7 cấm #5) |
| 3 | Phiếu phụ có được gộp tiếp (chuỗi) hay phải trỏ thẳng phiếu chính | Hình dạng dữ liệu; chọn sai thì thống kê "chỉ phiếu chính" lệch |
| 4 | Ô "Đang trễ hạn (hiện tại)" có loại phiếu phụ không | Chủ dự án chỉ nói cho đúng hạn, lĩnh vực, đơn vị (§7) và theo thôn (ADR 0053 §Sửa đổi 09/10/2026) |

## ĐIỀU KIỆN DỪNG

1. Đóng phiếu phụ lúc gộp, hoặc thêm trạng thái "đã gộp"
2. Kéo dài hạn của bất kỳ phiếu nào khi gộp hay tách
3. Hiện cho người dân bất kỳ trường nào của phiếu khác
4. Gộp xuyên xã, hoặc gộp phiếu lĩnh vực `can-bo`
5. Viết cứng 50 m / 7 ngày

→ ADR 0027 (chín trạng thái, quyết định C) · 0028 (hạn đặt ở hành vi) · 0030 (`feedback.restricted`) ·
0041 (bảng báo) · 0053 (số liệu) · luật 1 · 4 · 6 · 7 · 10
