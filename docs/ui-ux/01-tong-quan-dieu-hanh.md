# 01 — Tổng quan điều hành

**Route:** `/tong-quan` · **Tiêu đề trang:** `Tổng quan điều hành · ViGov` · **Menu:** Tổng quan (nhóm Điều hành)

---

## 1. Mục đích

Màn hình mặc định khi đăng nhập. Gom toàn bộ chỉ số điều hành của xã vào một trang để lãnh đạo nắm tình hình trong 10 giây và **bấm thẳng vào con số để xem danh sách đằng sau nó**. Có chế độ trình chiếu để dùng trong phòng họp giao ban.

---

## 2. Bố cục

```
PageHeader
  Tiêu đề: "Tổng quan điều hành"
  Dòng meta: "Kỳ tháng này: 1/9/2026 – 30/9/2026 · tính đến 16:43 07/09/2026  [badge: số liệu cũ hơn 10 phút]"
  Hàng nút (phải): [Tuần này][Tháng này][Quý này][Năm nay]  [⟳ Tính lại ngay]  [PDF][XLSX][PPTX]  [⤢ Trình chiếu]

Lưới 2 cột (desktop ≥1280px) × 3 hàng — 6 thẻ nhóm KPI:
  ┌──────────────────────────┬──────────────────────────┐
  │ NHIỆM VỤ                 │ VĂN BẢN & ĐƠN THƯ        │
  ├──────────────────────────┼──────────────────────────┤
  │ GIẢI NGÂN NGÂN SÁCH      │ THU - CHI NGÂN SÁCH XÃ   │
  ├──────────────────────────┼──────────────────────────┤
  │ PHẢN ÁNH NGƯỜI DÂN       │ KINH TẾ & TÀI NGUYÊN     │
  └──────────────────────────┴──────────────────────────┘

Khối cuối: "CẦN XỬ LÝ NGAY" (badge số lượng) — danh sách việc trễ nặng nhất
```

Dưới 1024px: lưới về 1 cột.

---

## 3. Thanh điều khiển kỳ báo cáo

| Thành phần | Hành vi |
|---|---|
| `Tuần này` / `Tháng này` / `Quý này` / `Năm nay` | Segmented control, chọn 1. Mặc định **Tháng này**. Đổi kỳ → nạp lại toàn bộ KPI. |
| Dòng meta | `Kỳ {nhãn}: {từ ngày} – {đến ngày} · tính đến {HH:mm dd/MM/yyyy}` |
| Badge "số liệu cũ hơn 10 phút" | Hiện màu cam khi `now - snapshot_at > 10 phút`. Ẩn khi số liệu còn mới. |
| `⟳ Tính lại ngay` | aria-label `Tính lại ngay`. Gọi job tính lại snapshot, cập nhật `tính đến`. Liên quan tới job `Tính lại số liệu Tổng quan` ở Cấu hình → Tự động hoá: nếu job tắt, đây là cách duy nhất làm mới. |
| `PDF` / `XLSX` / `PPTX` | Xuất báo cáo đúng kỳ đang chọn. |
| `⤢ Trình chiếu` | aria-label `Chế độ trình chiếu phòng họp`. Ẩn sidebar + header, phóng to chữ, nền tối, điều hướng bằng phím. |

---

## 4. Sáu nhóm KPI (chi tiết từng ô)

Mỗi ô KPI gồm: **giá trị lớn** → **nhãn** → **dòng so sánh kỳ trước**.
Dòng so sánh có 3 dạng:
- `chưa có kỳ trước để so` (xám)
- `không đổi` (xám)
- `↓ -100,0% so với kỳ trước` / `↑ +12,5% so với kỳ trước` (đỏ khi xấu đi, xanh khi tốt lên)

Mỗi ô là một **button** với aria-label `Xem danh sách đằng sau: {nhãn}` → mở danh sách/điều hướng tới module tương ứng đã lọc sẵn.

### 4.1 NHIỆM VỤ

| Nhãn | Nguồn tính | Ví dụ |
|---|---|---|
| Đang thực hiện | đếm nhiệm vụ trạng thái ∈ {mới giao, đã tiếp nhận, đang thực hiện, chờ duyệt} | 24 |
| Quá hạn | `han_xu_ly < now` và chưa hoàn thành | 14 *(hiện màu đỏ)* |
| Hoàn thành trong kỳ | hoàn thành trong khoảng kỳ | 1 |
| Đúng hạn trong kỳ | % việc hoàn thành trong kỳ mà `ngay_hoan_thanh ≤ han_ban_dau` | 0.0% |

### 4.2 VĂN BẢN & ĐƠN THƯ

| Nhãn | Nguồn | Ví dụ |
|---|---|---|
| Đến trong kỳ | văn bản đến vào sổ trong kỳ | 0 |
| Chưa xử lý xong | đơn thư + văn bản chưa đóng | 6 |
| Quá hạn xử lý | quá `han_giai_quyet` | 4 *(đỏ)* |
| Đơn thư trong kỳ | đơn thư vào sổ trong kỳ | 0 |

### 4.3 GIẢI NGÂN NGÂN SÁCH

| Nhãn | Nguồn | Ví dụ |
|---|---|---|
| Tỷ lệ giải ngân | `đã giải ngân / kế hoạch vốn năm` | 10.3% |
| Thời gian đã trôi qua | % ngày đã qua trong năm ngân sách | 68.3% |
| Dự án chậm | số dự án có `điểm chậm > ngưỡng` | 30 *(đỏ)* |
| Vướng mắc chưa gỡ | vướng mắc trạng thái chưa gỡ | 0 |
| Đã giải ngân | tổng tiền, rút gọn | 3,4 tỷ |

> **Quy tắc đọc**: "Tỷ lệ giải ngân" so với "Thời gian đã trôi qua" chính là thước đo chậm/nhanh. Chênh lệch = điểm chậm.

### 4.4 THU - CHI NGÂN SÁCH XÃ

| Nhãn | Nguồn | Ví dụ |
|---|---|---|
| Thu đạt dự toán | `tổng thu / dự toán thu` | 108.1% |
| Tổng thu | | 4.317 tỷ |
| Chi đạt dự toán | `tổng chi / dự toán chi` | 91.3% |
| Tổng chi | | 3.463 tỷ |
| Cân đối thu - chi | `tổng thu − tổng chi` | 853,3 tỷ |

### 4.5 PHẢN ÁNH NGƯỜI DÂN

| Nhãn | Nguồn | Ví dụ |
|---|---|---|
| Tiếp nhận trong kỳ | | 2 |
| Đang xử lý | trạng thái chưa đóng | 14 |
| Đúng hạn trong kỳ | % phiếu đóng đúng hạn | 33.3% |
| Trễ hạn trong kỳ | | 2 *(đỏ)* |
| Điểm hài lòng | trung bình sao, `—` nếu chưa có | — |

### 4.6 KINH TẾ & TÀI NGUYÊN

| Nhãn | Nguồn | Ví dụ |
|---|---|---|
| Doanh nghiệp | đối tượng bản đồ loại `enterprise` | 5 |
| Hộ kinh doanh | loại `household_business` | 3 |
| Thành lập mới | tạo trong kỳ | 0 |
| Tổng tài nguyên | tổng mọi đối tượng bản đồ | 26 |

---

## 5. Khối "CẦN XỬ LÝ NGAY"

- Tiêu đề khối viết HOA + badge số lượng (ví dụ `10`).
- Danh sách tối đa 10 mục, **sắp xếp giảm dần theo mức độ trễ**.
- Mỗi dòng: tiêu đề việc (1–2 dòng, cắt bớt) → dòng phụ đỏ `quá hạn {N} ngày {M} giờ` → nhãn bộ phận đang giữ (viết HOA, xám).
- Nguồn dữ liệu **hợp nhất từ nhiều module**: nhiệm vụ quá hạn, phản ánh quá hạn, đơn thư quá hạn. Ví dụ trong prototype có cả một mục là nội dung phản ánh (`Nước thải từ trại chăn nuôi…`).
- Bấm một dòng → mở chi tiết hồ sơ đúng module của nó.

---

## 6. Mô hình dữ liệu

Nên tính sẵn (materialize) thay vì query trực tiếp mỗi lần tải trang.

**`snapshot_tong_quan`**

| Trường | Kiểu | Ghi chú |
|---|---|---|
| `id` | uuid | |
| `don_vi_id` | uuid | |
| `ky` | enum | `tuan` \| `thang` \| `quy` \| `nam` |
| `tu_ngay` / `den_ngay` | date | |
| `tinh_den` | timestamp | hiển thị ở dòng meta |
| `du_lieu` | jsonb | mảng 6 nhóm × các ô KPI, mỗi ô `{ma, nhan, gia_tri, dinh_dang, so_sanh_ky_truoc, link_dich}` |

**`viec_can_xu_ly_ngay`** — view hợp nhất

```sql
SELECT 'nhiem_vu' AS nguon, id, tieu_de, han_xu_ly, bo_phan_id FROM nhiem_vu WHERE han_xu_ly < now() AND trang_thai NOT IN ('hoan-thanh')
UNION ALL
SELECT 'phan_anh', id, noi_dung, han_xu_ly, bo_phan_id FROM phan_anh WHERE han_xu_ly < now() AND trang_thai NOT IN ('da-dong','khong-tiep-nhan')
UNION ALL
SELECT 'don_thu', id, noi_dung, han_giai_quyet, bo_phan_id FROM don_thu WHERE han_giai_quyet < now() AND trang_thai <> 'da-giai-quyet'
ORDER BY han_xu_ly ASC LIMIT 10;
```

---

## 7. API đề xuất

| Method | Endpoint | Mô tả |
|---|---|---|
| GET | `/api/tong-quan?ky=thang` | Trả snapshot KPI + danh sách cần xử lý ngay |
| POST | `/api/tong-quan/tinh-lai` | Chạy lại job tính snapshot, trả snapshot mới |
| GET | `/api/tong-quan/xuat?dinh_dang=pdf\|xlsx\|pptx&ky=thang` | Trả file |
| GET | `/api/tong-quan/chi-tiet/:ma_kpi?ky=thang` | Danh sách bản ghi đằng sau một ô KPI |

---

## 8. Trạng thái đặc biệt

- **Chưa có dữ liệu kỳ trước**: mọi dòng so sánh ghi `chưa có kỳ trước để so`.
- **Snapshot cũ**: badge cam + gợi ý bấm `Tính lại ngay`.
- **Đang tính lại**: nút hiện spinner, disable; các thẻ KPI hiện skeleton.
- **Không có việc quá hạn**: khối `CẦN XỬ LÝ NGAY` hiện `Không có việc nào cần xử lý ngay.`

---

## 9. Ghi chú khi dựng lại

- Trang này **không có bộ lọc bộ phận** — luôn là toàn xã. Đừng thêm.
- Con số phải là button có `aria-label` như đã nêu; đây là điểm nhấn UX của sản phẩm ("bấm vào số để thấy danh sách").
- Trang `Báo cáo` (`/bao-cao`) dùng **lại đúng 6 nhóm KPI này**; hãy tách thành component dùng chung `KpiGroupCard` + `useKpiTongQuan(ky)` để không viết hai lần. Khác biệt: `/bao-cao` có thêm kỳ `Tuỳ chọn`, bảng `Xếp hạng bộ phận` và biểu đồ `So sánh với kỳ trước`, và không có khối `CẦN XỬ LÝ NGAY`.
