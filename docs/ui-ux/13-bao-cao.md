# 13 — Báo cáo điều hành

**Route:** `/bao-cao` · **Tiêu đề trang:** `Báo cáo tổng hợp · ViGov` · **Menu:** Báo cáo (nhóm Quản trị)

---

## 1. Mục đích

Bản báo cáo tổng hợp toàn xã theo kỳ, để in/xuất và trình bày. Khác `/tong-quan` ở ba điểm:
- Có kỳ **`Tuỳ chọn`** (chọn khoảng ngày bất kỳ).
- Có bảng **`Xếp hạng bộ phận`**.
- Có biểu đồ **`So sánh với kỳ trước`**.
- **Không có** khối `CẦN XỬ LÝ NGAY` và không có nút `Tính lại ngay` / `Trình chiếu`.

Dòng meta: *"Số liệu tính đến {HH:mm dd/MM/yyyy}. So sánh với cùng độ dài kỳ liền trước."*

---

## 2. Bố cục

```
PageHeader
  "Báo cáo điều hành"
  "Số liệu tính đến 16:43 07/09/2026. So sánh với cùng độ dài kỳ liền trước."
  Nút phải: [Tuần này][Tháng này][Quý này][Năm nay][Tuỳ chọn]

Hàng xuất file:  [⬇ Xuất PDF] [⬇ Xuất XLSX] [⬇ Xuất PPTX]

Lưới 2 cột × 3 hàng — 6 thẻ nhóm KPI (giống hệt /tong-quan)
Bảng "Xếp hạng bộ phận"
Biểu đồ "So sánh với kỳ trước"
```

---

## 3. Chọn kỳ

| Nút | Hành vi |
|---|---|
| `Tuần này` / `Tháng này` (mặc định) / `Quý này` / `Năm nay` | như `/tong-quan` |
| `Tuỳ chọn` | mở date-range picker: `Từ ngày` – `Đến ngày`. Kỳ so sánh = khoảng **cùng độ dài** liền trước. |

---

## 4. Sáu nhóm KPI

Giống hệt `/tong-quan` — xem `01-tong-quan-dieu-hanh.md` mục 4. Tái sử dụng component `KpiGroupCard` và hook `useKpiTongQuan(ky)`.

Tóm tắt các nhóm: `NHIỆM VỤ` · `VĂN BẢN & ĐƠN THƯ` · `GIẢI NGÂN NGÂN SÁCH` · `THU - CHI NGÂN SÁCH XÃ` · `PHẢN ÁNH NGƯỜI DÂN` · `KINH TẾ & TÀI NGUYÊN`.

---

## 5. Bảng "Xếp hạng bộ phận"

| Bộ phận | Tổng việc | Quá hạn | Hoàn thành | Đúng hạn |
|---|---|---|---|---|
| VĂN PHÒNG ĐẢNG ỦY | 21 | *(thanh ngang đỏ tỷ lệ)* | 13 | 0 — |
| THƯỜNG TRỰC UBMTTQ VIỆT NAM | 2 | | 0 | 0 — |
| THƯỜNG TRỰC HỘI ĐỒNG NHÂN DÂN | 0 | | 0 | 0 — |
| LÃNH ĐẠO ỦY BAN NHÂN DÂN XÃ | 0 | | 0 | 0 — |
| THƯỜNG TRỰC ĐẢNG UỶ | 0 | | 0 | 0 — |

- Sắp xếp giảm dần theo `Tổng việc`.
- Cột `Quá hạn` hiển thị **thanh ngang** tỷ lệ quá hạn/tổng, không phải số trần.
- Cột `Đúng hạn` hiển thị `{n} — {tỷ lệ}%`; `—` khi chưa có việc hoàn thành để tính.

---

## 6. Biểu đồ "So sánh với kỳ trước"

Biểu đồ **thanh ngang** (horizontal bar), trục X từ `-100%` → `0%` (chia mốc `-100% / -75% / -50% / -25% / 0%`, tự giãn khi có giá trị dương).

Các chỉ tiêu so sánh (thứ tự đúng như prototype):
1. Hoàn thành trong kỳ
2. Đúng hạn trong kỳ *(nhiệm vụ)*
3. Đến trong kỳ *(văn bản)*
4. Đơn thư trong kỳ
5. Tiếp nhận trong kỳ *(phản ánh)*
6. Đúng hạn trong kỳ *(phản ánh)*
7. Điểm hài lòng
8. Thành lập mới

Thanh âm màu đỏ, thanh dương màu xanh lá.

---

## 7. Xuất file

| Nút | Nội dung xuất |
|---|---|
| `Xuất PDF` | Toàn bộ trang, khổ A4 ngang, có header tên xã + kỳ báo cáo + ngày xuất. Render bằng Puppeteer từ chính trang này với `?print=1`. |
| `Xuất XLSX` | Mỗi nhóm KPI một sheet + sheet `Xếp hạng bộ phận` + sheet `So sánh kỳ trước`. |
| `Xuất PPTX` | Mỗi nhóm KPI một slide + slide xếp hạng + slide so sánh. Dùng `pptxgenjs`. |

Tên tệp gợi ý: `bao-cao-dieu-hanh-{xa}-{ky}-{yyyyMMdd}.{ext}`.

---

## 8. API đề xuất

| Method | Endpoint |
|---|---|
| GET | `/api/bao-cao?ky=thang` hoặc `?tu_ngay=&den_ngay=` |
| GET | `/api/bao-cao/xep-hang-bo-phan?ky=` |
| GET | `/api/bao-cao/so-sanh-ky-truoc?ky=` |
| GET | `/api/bao-cao/xuat?dinh_dang=pdf\|xlsx\|pptx&ky=` |

---

## 9. Quy tắc nghiệp vụ

1. **Kỳ so sánh** luôn là khoảng cùng độ dài ngay trước kỳ đang chọn (tháng này ↔ tháng trước; tuỳ chọn 17 ngày ↔ 17 ngày liền trước).
2. Không có dữ liệu kỳ trước ⇒ ghi `chưa có kỳ trước để so`, không tính 0%.
3. Giá trị kỳ trước = 0 và kỳ này > 0 ⇒ hiển thị `mới` thay vì `+∞%`.
4. Quyền: `report.read` (xem báo cáo), `report.export` (xuất báo cáo). Nút xuất ẩn khi thiếu `report.export`.
5. Job `Gửi báo cáo định kỳ` (Cấu hình → Tự động hoá) dùng chính endpoint xuất này: báo cáo tuần vào đầu tuần, báo cáo tháng vào ngày mùng 1.

---

## 10. Ghi chú khi dựng lại

- `/tong-quan` và `/bao-cao` **không được lệch số**. Dùng chung một service tính KPI.
- Prototype hiển thị KPI dùng dấu chấm thập phân (`10.3%`) trong khi bảng dùng dấu phẩy (`10,33%`). Thống nhất về **dấu phẩy** theo chuẩn tiếng Việt.
- Trang này là ứng viên tốt cho chế độ in: thêm stylesheet `@media print` ẩn sidebar/header.
