# 05 — Văn bản đến & Đơn thư

**Route:** `/van-ban` · **Tiêu đề trang:** `Văn bản đến & đơn thư · ViGov` · **Menu:** Văn bản & Đơn thư (nhóm Điều hành)

---

## 1. Mục đích

Sổ vào công văn đến và **sổ theo dõi đơn khiếu nại, tố cáo, kiến nghị, đề nghị của công dân**. Mô tả trong giao diện: *"Vào sổ, phân công xử lý và theo dõi hạn giải quyết."*

Trong prototype, phần đang hoạt động là **Đơn thư công dân**; phần văn bản đến dùng chung khung sổ và cùng bảng SLA (`Văn bản đến` có dòng riêng trong `Cấu hình → Thời hạn xử lý`: tiếp nhận 8 giờ, xử lý xong 40 giờ).

---

## 2. Bố cục

```
PageHeader
  "Đơn thư công dân"
  "Vào sổ, phân công xử lý và theo dõi hạn giải quyết."
  Nút: [+ Nhập tay]  hoặc  [⬚ Quét & OCR  (Sắp có)]   ← nút thứ hai disabled, có badge "Sắp có"

Tabs:  [Đơn thư công dân (7)] [Báo cáo]

── Tab 1: Đơn thư công dân ──────────────────────────────
  [Toàn xã | Giao cho tôi | Liên quan đến tôi]
  Câu dẫn: "Sổ theo dõi đơn khiếu nại, tố cáo, kiến nghị và đề nghị của công dân.
            Hệ thống nhắc khi một người gửi lại đơn có nội dung tương tự."
  Nút phải: [+ Vào sổ đơn thư]
  Bảng sổ đơn thư

── Tab 2: Báo cáo ───────────────────────────────────────
  Tiêu đề "Tiến độ tiếp nhận và xử lý đơn thư năm {yyyy}"   [⬇ Xuất Excel]
  6 thẻ KPI + 2 bảng + 1 biểu đồ cột theo tháng
```

---

## 3. Tab "Đơn thư công dân"

### 3.1 Bảng sổ

| Cột | Nội dung | Ghi chú |
|---|---|---|
| SỐ | số thứ tự vào sổ | `7`, `6`, `5`… giảm dần |
| NGÀY NHẬN | `d/M/yyyy` | |
| NGƯỜI GỬI | họ tên (đậm) + dòng phụ số điện thoại | `Lê Văn Tám` / `12392312313` |
| LOẠI ĐƠN | chip | `Kiến nghị, phản ánh` · `Khiếu nại` · `Tố cáo` · `Đề nghị` |
| NỘI DUNG | trích yếu, cắt 1 dòng | |
| ĐANG GIỮ | bộ phận đang giữ hồ sơ (viết HOA) hoặc `—` | |
| HẠN GIẢI QUYẾT | `Không đặt hạn` hoặc `d/M/yyyy (trễ N ngày)` | |
| TRẠNG THÁI | StatusChip | |

Bấm một hàng → mở **DetailDrawer đơn thư**.

### 3.2 Trạng thái đơn thư

| Mã | Nhãn | Màu | Ghi chú |
|---|---|---|---|
| `moi-vao-so` | Mới vào sổ | xanh dương nhạt | trạng thái khởi tạo |
| `da-phan-cong` | Đã phân công | xanh dương | |
| `dang-xu-ly` | Đang xử lý | cam | |
| `da-giai-quyet` | Đã giải quyết | xanh lá | kết thúc |
| `chuyen-cap-tren` | Chuyển cấp trên | tím | rẽ nhánh |
| `luu-khong-thu-ly` | Lưu, không thụ lý | xám | rẽ nhánh |

Từ `Đã phân công`, drawer cho chuyển sang đúng 4 trạng thái: `Đang xử lý`, `Đã giải quyết`, `Chuyển cấp trên`, `Lưu, không thụ lý`.

### 3.3 Loại đơn (danh mục `Loại đơn thư`)

| Mã | Nhãn | Thứ tự |
|---|---|---|
| `kien-nghi` | Đơn kiến nghị | 1 |
| `phan-anh` | Đơn phản ánh | 2 |
| `khieu-nai` | Đơn khiếu nại | 3 |
| `to-cao` | Đơn tố cáo | 4 |
| `de-nghi` | Đơn đề nghị | 5 |

> Giao diện bảng hiển thị gộp `Kiến nghị, phản ánh` — khi dựng lại, giữ đúng 5 mã trong danh mục nhưng cho phép nhãn hiển thị gộp.

### 3.4 Modal "Vào sổ đơn thư" / "Nhập tay"

| Trường | Kiểu | Bắt buộc |
|---|---|---|
| Số vào sổ | int, tự sinh tiếp theo | |
| Ngày nhận | date, mặc định hôm nay | ✔ |
| Họ tên người gửi | text | ✔ |
| Số điện thoại | text | |
| Địa chỉ | text | |
| Loại đơn | select (5 loại) | ✔ |
| Trích yếu nội dung | textarea | ✔ |
| Bộ phận thụ lý | select bộ phận | |
| Cán bộ xử lý | combobox cán bộ | |
| Hạn giải quyết | date (bỏ trống = `Không đặt hạn`) | |
| Tệp đính kèm | file (bản scan đơn) | |

Nút phụ `Quét & OCR` — **chưa triển khai**, hiện badge `Sắp có`, trạng thái disabled. Khi dựng lại: giữ nút ở trạng thái disabled hoặc ẩn sau feature flag.

### 3.5 DetailDrawer đơn thư

Lớp phủ gần toàn màn hình, nút `✕` góc phải trên.

**Header**
```
Đơn số 3/2026 · nhận ngày 15/8/2026
Khiếu nại về mốc giới thửa đất giáp ranh sau khi đo đạc lại, đề nghị xem xét kết quả đo đạc
👤 Người dân demo 3   📞 0900 111 003
```

**Hàng chuyển trạng thái** — 4 ô ngang, mỗi ô là nút có nhãn phụ `chuyển sang`:
```
[Đang xử lý]  [Đã giải quyết]  [Chuyển cấp trên]  [Lưu, không thụ lý]
 chuyển sang    chuyển sang       chuyển sang        chuyển sang
```

**Hàng chip** (trạng thái hiện tại + thuộc tính):
`Đã phân công` · `Khiếu nại` · `Nhập tay` · `Không đặt hạn`
(chip thứ ba là **nguồn vào sổ**: `Nhập tay` / `Quét & OCR`.)

**Ba ô thông tin**

| Ô | Nội dung |
|---|---|
| `ĐỊA CHỈ NGƯỜI GỬI` | `Thôn Bình Trị, xã Thăng Bình` |
| `BỘ PHẬN ĐANG GIỮ` | `VĂN PHÒNG ĐẢNG ỦY` |
| `HẠN XỬ LÝ` | `Không đặt` hoặc `d/M/yyyy (trễ N ngày)` |

Nút `✎ Sửa thông tin người gửi`.

**Nút `⇄ Chuyển thành nhiệm vụ`** (nền tối)
Chú thích: *"Nhiệm vụ kế thừa hạn xử lý của đơn, để hai bên không lệch nhau."*
→ sinh nhiệm vụ `nguồn giao = Từ văn bản đến`, tiêu đề mặc định `Giải quyết đơn thư số {n}: {trích yếu}`, `han_xu_ly` copy từ đơn.

**Khối `Chuyển cho bộ phận khác`**

| Trường | Kiểu | Mặc định |
|---|---|---|
| Chuyển đến | select bộ phận | `— Chọn bộ phận —` |
| Người xử lý (không bắt buộc) | select cán bộ (`Họ tên — email · Chức danh`) | `— Để bộ phận tự phân công —` |
| Lý do chuyển | text | placeholder `Thuộc thẩm quyền của bộ phận Địa chính` |

Nút `➤ Chuyển và ghi vết`.

**Khối `Nội dung trả lời công dân`**
Textarea, placeholder `Ghi rõ kết quả giải quyết để trả lời người gửi đơn.` + nút `Lưu nội dung trả lời`.

**Cột phải — `Dòng thời gian chuyển tiếp`**
```
V3  Huỳnh Văn 3          13:20 26/08/2026
    [Đã phân công]
    → Một cửa → VĂN PHÒNG ĐẢNG ỦY
    👤 Phụ trách: Huỳnh Văn 4
    Xác minh hồ sơ đo đạc và làm việc với hai hộ giáp ranh
```

**Cảnh báo đơn trùng lặp**: khi cùng người gửi (theo SĐT hoặc họ tên) đã có đơn nội dung tương tự → banner nhắc kèm link tới đơn cũ.

---

## 4. Tab "Báo cáo"

Tiêu đề: `Tiến độ tiếp nhận và xử lý đơn thư năm {yyyy}` · nút `⬇ Xuất Excel`.

### 4.1 Sáu thẻ KPI (lưới 3 cột × 2 hàng)

| Nhãn | Ghi chú | Ví dụ |
|---|---|---|
| Tiếp nhận trong năm | | 7 |
| Đã giải quyết | số xanh lá | 3 |
| Đang xử lý | dòng phụ `Gồm cả đơn tồn từ năm trước` | 4 |
| Quá hạn | | 0 |
| Giải quyết đúng hạn | % | 100% |
| Số ngày xử lý trung bình | có phần thập phân | 14,7 ngày |

### 4.2 Bảng "THEO LOẠI ĐƠN"

| LOẠI ĐƠN | TỔNG SỐ | ĐÃ GIẢI QUYẾT | ĐANG XỬ LÝ | QUÁ HẠN |
|---|---|---|---|---|
| Kiến nghị, phản ánh | 4 | 2 | 2 | 0 |
| Đề nghị | 1 | 0 | 1 | 0 |
| Khiếu nại | 1 | 0 | 1 | 0 |
| Tố cáo | 1 | 1 | 0 | 0 |

Cột `ĐÃ GIẢI QUYẾT` tô xanh lá, `QUÁ HẠN` tô đỏ khi > 0.

### 4.3 Bảng "TIẾN ĐỘ XỬ LÝ THEO ĐƠN VỊ"

| BỘ PHẬN | TỔNG SỐ | ĐANG XỬ LÝ | ĐÃ GIẢI QUYẾT | QUÁ HẠN | ĐÚNG HẠN |
|---|---|---|---|---|---|
| Chưa phân công | 3 | 2 | 1 | 0 | 100% |
| VĂN PHÒNG ĐẢNG ỦY | 3 | 2 | 1 | 0 | 100% |
| THƯỜNG TRỰC HỘI ĐỒNG NHÂN DÂN | 1 | 0 | 1 | 0 | 100% |

### 4.4 Biểu đồ "TIẾP NHẬN VÀ GIẢI QUYẾT THEO THÁNG"

Biểu đồ cột nhóm, trục X = `T1`…`T12`, hai chuỗi có chú giải: **Tiếp nhận** và **Đã giải quyết**.

---

## 5. Mô hình dữ liệu

**`don_thu`**

| Trường | Kiểu | Ghi chú |
|---|---|---|
| `id` | uuid | |
| `so_vao_so` | int | duy nhất theo năm |
| `nam` | int | |
| `ngay_nhan` | date | |
| `nguoi_gui_ho_ten` | text | |
| `nguoi_gui_dien_thoai` | text null | |
| `nguoi_gui_dia_chi` | text null | `Thôn Bình Trị, xã Thăng Bình` |
| `nguon_vao_so` | enum | `nhap-tay` \| `quet-ocr` — hiển thị thành chip trong drawer |
| `noi_dung_tra_loi` | text null | nội dung trả lời công dân |
| `loai_don` | enum | 5 mã ở mục 3.3 |
| `trich_yeu` | text | |
| `noi_dung` | text null | |
| `bo_phan_dang_giu_id` | uuid null | cột `ĐANG GIỮ` |
| `can_bo_xu_ly_id` | uuid null | |
| `han_giai_quyet` | date null | null ⇒ `Không đặt hạn` |
| `trang_thai` | enum | 6 mã ở mục 3.2 |
| `ngay_giai_quyet` | date null | |
| `dinh_kem` | jsonb | |
| `don_lien_quan_id` | uuid null | đơn trùng lặp / gửi lại |
| `nguoi_tao_id`, `tao_luc` | | |

**`van_ban_den`** (khung tương tự, cho công văn đến)

| Trường | Kiểu |
|---|---|
| `id`, `so_vao_so`, `nam`, `ngay_den` | |
| `so_ky_hieu` | text (`1742-CV/BTCTU`) |
| `ngay_van_ban` | date |
| `co_quan_ban_hanh` | text |
| `loai_van_ban` | enum — danh mục `Loại văn bản`: `cong-van`, `quyet-dinh`, `thong-bao`, `ke-hoach`, `giay-moi`, `bao-cao`, `to-trinh` |
| `trich_yeu` | text |
| `do_khan` | enum |
| `bo_phan_dang_giu_id`, `can_bo_xu_ly_id` | |
| `han_xu_ly` | date |
| `trang_thai` | enum |

**`nhat_ky_don_thu`**: `id, don_thu_id, nguoi_id, thoi_diem, trang_thai_tai_thoi_diem, noi_dung, dinh_kem`

---

## 6. API đề xuất

| Method | Endpoint | Mô tả |
|---|---|---|
| GET | `/api/don-thu` | `?pham_vi=&loai=&trang_thai=&q=` |
| POST | `/api/don-thu` | vào sổ |
| GET/PATCH | `/api/don-thu/:id` | |
| POST | `/api/don-thu/:id/chuyen-xu-ly` | `{bo_phan_id, can_bo_id, ghi_chu}` |
| POST | `/api/don-thu/:id/trang-thai` | |
| POST | `/api/don-thu/:id/tao-nhiem-vu` | sinh nhiệm vụ liên kết, kế thừa hạn xử lý |
| PATCH | `/api/don-thu/:id/nguoi-gui` | sửa thông tin người gửi |
| PUT | `/api/don-thu/:id/tra-loi` | lưu nội dung trả lời công dân |
| POST | `/api/don-thu/:id/nhat-ky` | |
| GET | `/api/don-thu/bao-cao?nam=2026` | dữ liệu tab Báo cáo |
| GET | `/api/don-thu/bao-cao/xuat-excel?nam=2026` | |
| GET | `/api/don-thu/trung-lap?dien_thoai=&noi_dung=` | kiểm tra đơn tương tự |
| GET | `/api/van-ban-den` … | bộ endpoint tương ứng cho công văn đến |

---

## 7. Quy tắc nghiệp vụ

1. **Số vào sổ** cấp tự động, liên tục theo năm, không tái sử dụng khi xoá.
2. **Nhắc đơn gửi lại**: khi vào sổ, so khớp người gửi (SĐT/họ tên) + độ tương đồng nội dung với các đơn cũ → cảnh báo, gợi ý gắn `don_lien_quan_id`.
3. SLA mặc định cho `Văn bản đến`: tiếp nhận 8 giờ, xử lý xong 40 giờ, sắp đến hạn 24 giờ, báo lãnh đạo trực tiếp sau 24 giờ, báo Chủ tịch sau 48 giờ. Đơn thư không có dòng SLA riêng trong prototype ⇒ mặc định `Không đặt hạn` cho tới khi cán bộ đặt tay.
4. Quyền: `petition.create`, `petition.read` cho đơn thư; `document.create`, `document.read`, `document.route` cho văn bản đến.
5. Đơn thư đã giải quyết vẫn giữ trong sổ (không xoá), phục vụ báo cáo năm.
6. Tỷ lệ `Giải quyết đúng hạn` chỉ tính trên các đơn **có đặt hạn** và đã giải quyết.

---

## 8. Dữ liệu mẫu để seed

| Số | Ngày | Người gửi | Loại | Nội dung | Đang giữ | Trạng thái |
|---|---|---|---|---|---|---|
| 7 | 9/9/2026 | Lê Văn Tám · 12392312313 | Kiến nghị, phản ánh | sdfsaf ấdf | — | Mới vào sổ |
| 6 | 9/9/2026 | Đinh Văn Linh | Kiến nghị, phản ánh | Quá bực mình | — | Mới vào sổ |
| 5 | 9/9/2026 | Nguyễn Văn Linh · 0909123213 | Kiến nghị, phản ánh | Quá khó chịu vì tiếng ồn | — | Đã giải quyết |
| 4 | 8/8/2026 | Người dân demo 4 · 0900 111 004 | Tố cáo | Tố cáo hành vi đổ trộm chất thải xây dựng ra khu đất công ven kênh vào ban đêm | VĂN PHÒNG ĐẢNG ỦY | Đã giải quyết |
| 3 | 15/8/2026 | Người dân demo 3 · 0900 111 003 | Khiếu nại | Khiếu nại về mốc giới thửa đất giáp ranh sau khi đo đạc lại | VĂN PHÒNG ĐẢNG ỦY | Đã phân công |
| 2 | 19/8/2026 | Người dân demo 2 · 0900 111 002 | Đề nghị | Đề nghị cấp lại giấy chứng nhận QSDĐ do bị mất trong đợt lụt năm 2025 | VĂN PHÒNG ĐẢNG ỦY | Đã phân công |
| 1 | 23/8/2026 | Người dân demo 1 · 0900 111 001 | Kiến nghị, phản ánh | Kiến nghị nâng cấp tuyến mương thoát nước dọc đường liên thôn Bình An | THƯỜNG TRỰC HĐND | Đã giải quyết |

Đơn số 4 đã sinh nhiệm vụ `NV32 — Giải quyết đơn thư số 4: …`.
