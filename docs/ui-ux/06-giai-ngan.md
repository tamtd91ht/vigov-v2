# 06 — Theo dõi giải ngân

**Route:** `/giai-ngan` · **Tiêu đề trang:** `Theo dõi giải ngân · ViGov` · **Menu:** Giải ngân (nhóm Điều hành)
**Route con:** `/giai-ngan/du-an/:id` — chi tiết dự án (`:id` là **UUID**, tiêu đề trang `Chi tiết dự án · ViGov`)

---

## 1. Mục đích

Mô tả trong giao diện: *"Tiến độ giải ngân theo dự án, chứng từ và vướng mắc cần tháo gỡ."*

Banner cảnh báo **bắt buộc hiển thị** ở đầu trang (icon ⓘ, nền xám nhạt):
> *"ViGov là công cụ theo dõi và điều hành, không phải phần mềm kế toán. Số liệu phục vụ chỉ đạo, không thay thế sổ sách kế toán và không đối chiếu với Kho bạc."*

---

## 2. Bố cục

```
PageHeader
  "Theo dõi giải ngân"
  "Tiến độ giải ngân theo dự án, chứng từ và vướng mắc cần tháo gỡ."
  Nút: [☰ Hạng mục] [⬆ Nhập giải ngân] [+ Thêm dự án]   [Năm ngân sách 2026 ▾]

Banner cảnh báo "không phải phần mềm kế toán"

4 thẻ KPI (lưới 4 cột)
Biểu đồ "Luỹ kế giải ngân so với kế hoạch"
Bảng "Tiến độ theo hạng mục"
Khối "Tiến độ theo nguồn vốn"  [Quản lý nguồn vốn]
Hàng lọc dự án
Bảng dự án (gộp theo hạng mục)
```

**Chọn năm ngân sách**: select với các năm `2027`, `2026` (mặc định), `2025`, `2024`.

---

## 3. Bốn thẻ KPI

| Thẻ | Giá trị chính | Dòng phụ |
|---|---|---|
| KẾ HOẠCH VỐN NĂM | `33.230.000.000 đ` | `63 dự án` |
| ĐÃ GIẢI NGÂN | `3.433.990.000 đ` *(đỏ khi chậm)* | `10,33% kế hoạch · thời gian đã qua 70,96%` |
| CÒN PHẢI GIẢI NGÂN | `29.796.010.000 đ` | `Ngưỡng cảnh báo chậm: 10 điểm` |
| CẦN CHÚ Ý | `29 dự án chậm` *(đỏ)* | `0 vướng mắc đang theo dõi · 0 nguy cơ không giải ngân hết` |

### Công thức "điểm chậm"

```
thoi_gian_da_qua_pct = (hôm nay − 01/01/năm) / (31/12/năm − 01/01/năm) × 100
ty_le_giai_ngan_pct  = da_giai_ngan / ke_hoach_von_nam × 100
diem_cham            = thoi_gian_da_qua_pct − ty_le_giai_ngan_pct
dự án CHẬM khi diem_cham > nguong_canh_bao_cham   (mặc định 10 điểm)
```
Hiển thị trên hàng dự án: `chậm 31,36 điểm` (chip đỏ nhỏ dưới mã dự án).
Dự án giải ngân 0% giữa năm ⇒ `chậm 70,96 điểm` (bằng đúng % thời gian đã qua).

---

## 4. Biểu đồ "Luỹ kế giải ngân so với kế hoạch"

- Trục X: `T1` … `T12`. Trục Y: 0 đ → `8,5 tỷ` → `17 tỷ` → `25,5 tỷ` → `34 tỷ` (5 mốc, tự chia theo kế hoạch năm).
- Hai đường:
  - **Kế hoạch** — nét đứt, màu xám, đi tuyến tính từ 0 tới kế hoạch vốn năm.
  - **Thực hiện** — nét liền, màu xanh dương, có điểm tròn mỗi tháng, là luỹ kế thực giải ngân.
- Chú giải dưới biểu đồ: `— — Kế hoạch   —●— Thực hiện`.

---

## 5. Bảng "Tiến độ theo hạng mục"

| Cột | Ví dụ |
|---|---|
| Hạng mục | `Các công trình chuyển tiếp` |
| Số DA | `8` |
| KH vốn năm | `800.000.000 đ` |
| Đã giải ngân | `260.690.000 đ` |
| Tỷ lệ | `32,59%` *(màu theo ngưỡng)* |
| Chưa giải ngân | `539.310.000 đ` |
| Tỷ lệ | `67,41%` |
| Thời hạn giải ngân | `31/12/2026` |

Hàng cuối `Tổng cộng` in đậm.
Chú thích dưới bảng: *"Chạm một hạng mục để lọc danh sách dự án bên dưới."* — bấm hàng = lọc bảng dự án.

### Sáu hạng mục kế hoạch vốn (danh mục `Hạng mục kế hoạch vốn`)

| Mã | Nhãn | Thứ tự |
|---|---|---|
| `chuyen-tiep` | Các công trình chuyển tiếp | 1 |
| `xay-dung-moi` | Vốn đầu tư các công trình xây dựng mới | 2 |
| `tra-no` | Vốn trả nợ các công trình tiếp nhận từ xã, thị trấn (cũ) | 3 |
| `keo-dai` | Vốn các công trình kéo dài (2025 sang 2026) | 4 |
| `nong-thon-moi` | Kế hoạch vốn nông thôn mới | 5 |
| `tra-no-chua-phan-bo` | Vốn trả nợ chưa phân bổ | 6 |

Nút `☰ Hạng mục` ở header mở màn quản lý hạng mục (thêm/sửa/xoá, đặt thời hạn giải ngân, đặt kế hoạch vốn năm cho từng hạng mục).

---

## 6. Khối "Tiến độ theo nguồn vốn"

Nút `Quản lý nguồn vốn` ở góc phải.
Mỗi nguồn vốn là một card có **3 thanh tiến độ**:

```
Ngân sách xã, phường                                      2 dự án
Đã phân bổ 70 triệu / 9,2 tỷ · còn 9,1 tỷ chưa phân bổ        1%   ▓░░░░░
Đã giải ngân 13,2 triệu / 70 triệu đã phân bổ              18,9%   ▓▓░░░░
Đã giải ngân 13,2 triệu / 9,2 tỷ tổng nguồn                 0,1%   ░░░░░░
```

Bốn nguồn vốn mẫu:

| Nguồn vốn | Tổng nguồn | Đã phân bổ | Số dự án |
|---|---|---|---|
| Ngân sách xã, phường | 9,2 tỷ | 70 triệu | 2 |
| Ngân sách thành phố hỗ trợ | 6,5 tỷ | 110 triệu | 2 |
| Chương trình mục tiêu quốc gia | 10,4 tỷ | 0 đ | 0 |
| Nguồn xã hội hoá | 1,1 tỷ | 20 triệu | 2 |

Chú thích cuối khối (màu cam): *"Còn 3,4 tỷ đã chi nhưng chưa ghi rút từ nguồn nào — thuộc các dự án chưa khai phân bổ nguồn vốn."*

---

## 7. Bảng dự án

### 7.1 Hàng lọc

| Thành phần | Giá trị |
|---|---|
| Ô tìm | `Tìm theo tên hoặc mã dự án…` |
| Select hạng mục | `Tất cả hạng mục` + 6 hạng mục |
| ☐ Chỉ dự án chậm | |
| ☑ Gộp theo hạng mục | mặc định **bật** |

### 7.2 Cột

| Cột | Nội dung |
|---|---|
| ☐ | chọn |
| Mã | `DA-2026-be-tong-hoa-duong-ngo-xo-2` (slug), dưới là chip đỏ `chậm 31,36 điểm` nếu chậm |
| Dự án | tên đầy đủ |
| Đơn vị / phụ trách | `Chưa phân công` |
| KH vốn năm | `100 triệu` |
| Đã giải ngân | `90 triệu` + thanh tiến độ nhỏ |
| Tiến độ | `90%` |
| *(dòng dưới ô Tiến độ)* | chip nguồn vốn: `Đủ · 3 nguồn` + liệt kê tên nguồn; hoặc `Chưa gắn nguồn` (cam) |
| Thời hạn giải ngân | `31/12/2026` |
| Vướng mắc mới nhất | nội dung + `27/8/2026 · đã gỡ`; hoặc `—` |

Khi bật **Gộp theo hạng mục**: mỗi nhóm có hàng tiêu đề
`Vốn trả nợ các công trình tiếp nhận từ xã, thị trấn (cũ)` — `19 dự án · kế hoạch 1.900.000.000 đ · đã giải ngân 1.425.800.000 đ`.

Bấm tên dự án → `/giai-ngan/du-an/:id`.

---

## 8. Trang chi tiết dự án

```
← Theo dõi giải ngân

DA-2026-be-tong-hoa-duong-ngo-xo-2
Bê tông hóa đường ngõ xóm tổ 6 và Tuyến GTNT thôn Thanh Ly 1 (tổ 8;9;10)      [✎ Sửa dự án]
Ngân sách thành phố hỗ trợ · Ngân sách xã, phường · Nguồn xã hội hoá · phụ trách Chưa phân công

[Bám sát tiến độ]        ← chip trạng thái tiến độ (xanh lá) / "Chậm N điểm" (đỏ)

KẾ HOẠCH VỐN NĂM   TỔNG MỨC ĐƯỢC DUYỆT   ĐÃ GIẢI NGÂN    CÒN LẠI
100.000.000 đ      100.000.000 đ         90.000.000 đ    10.000.000 đ

THỜI GIAN THỰC HIỆN   ĐƠN VỊ THỰC HIỆN
— → —                 —

▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓░░  ← thanh tiến độ có vạch mốc "thời gian đã trôi qua"
Giải ngân 90% · thời gian đã trôi qua 70,96%

GIẢI NGÂN THEO NGUỒN VỐN
  Ngân sách thành phố hỗ trợ     0 đ / 50.000.000 đ · 0%   ░░░░░
  Ngân sách xã, phường           0 đ / 40.000.000 đ · 0%   ░░░░░
  Nguồn xã hội hoá               0 đ / 10.000.000 đ · 0%   ░░░░░

Tabs: [Vướng mắc (0)] [Chứng từ (3)] [Biểu đồ] [Trao đổi]
```

### 8.1 Tab "Vướng mắc"
- Ô nhập `Vướng mắc đang gặp ở dự án này…` + nút `➤ Ghi nhận`.
- Chú thích: *"Dự án đã có cán bộ phụ trách thì hệ thống tự sinh một nhiệm vụ theo dõi ở phần hồ sơ nhiệm vụ."*
- Dòng thời gian: `{người} {HH:mm dd/MM/yyyy}` + chip `Đã gỡ` + nội dung (gạch ngang khi đã gỡ).

### 8.2 Tab "Chứng từ"
Nút `+ Ghi nhận khoản chi`. Bảng:

| NGÀY CHI | SỐ TIỀN | NGUỒN VỐN | NỘI DUNG | CHỨNG TỪ | TRẠNG THÁI |
|---|---|---|---|---|---|
| 7/9/2026 | 30.000.000 đ | — | Thanh toán đợt 3 | — | `Kế toán nhập` + nút `Xác nhận` `Khoá` `✎ Sửa` `🗑 Gỡ` |
| 28/8/2026 | 12.000.000 đ | — | Thanh toán đợt 2 / *Công ty ABC* | — | `Kế toán nhập` |

**Vòng đời chứng từ:** `Kế toán nhập` → `Đã xác nhận` → `Đã khoá` (không sửa được nữa).
Quyền tương ứng: `budget.update` (nhập/sửa), `budget.confirm` (xác nhận, khoá), `budget.read`.

### 8.3 Tab "Biểu đồ"
Luỹ kế giải ngân của riêng dự án theo tháng, so với kế hoạch tuyến tính.

### 8.4 Tab "Trao đổi"
Nhật ký trao đổi tự do giữa các cán bộ về dự án (giống ActivityLog).

---

## 9. Modal "Thêm dự án"

Tiêu đề: `Thêm dự án`
Mô tả: *"Chọn hạng mục, đặt tên dự án và nhập số tiền bố trí cho năm 2026. Những mục còn lại bổ sung sau lúc nào cũng được."*

| Trường | Kiểu | Bắt buộc | Ghi chú |
|---|---|---|---|
| Hạng mục | select | ✔ | `— Chọn hạng mục —`; chú thích *"Báo cáo tiến độ cộng dồn theo hạng mục, nên mỗi dự án thuộc đúng một hạng mục."* |
| Mã dự án | text + ☑ `Tự sinh mã` | | *"Tự sinh sẽ cấp số tiếp theo trong dãy DA01, DA02… Mã tự nhập phải chưa từng được dùng, kể cả bởi dự án đã rút khỏi danh sách."* |
| Tên dự án | text | ✔ | placeholder `Bê tông hoá đường trục chính thôn Hà Lam` |
| Số tiền bố trí năm 2026 (đồng) | number | ✔ | placeholder `7.500.000.000` |
| **Nguồn vốn** | danh sách động + `+ Thêm nguồn vốn` | | *"Chưa gắn nguồn nào. Xã theo dõi kế hoạch vốn theo hạng mục thì để trống cũng được."* |

**Thông tin thêm (không bắt buộc)** — khối gập:

| Trường | Kiểu | Ghi chú |
|---|---|---|
| Tổng mức được duyệt cả dự án (đồng) | number | *"Để trống thì lấy bằng số tiền bố trí năm nay."* |
| Đơn vị thực hiện | select bộ phận | `— Chưa xác định —` |
| Cán bộ phụ trách | select cán bộ | `— Chưa phân công —` |
| Ngày khởi công | date | |
| Ngày hoàn thành | date | |
| Thời hạn giải ngân | date | *"Mốc phải hoàn tất phần vốn của năm. Khác với ngày hoàn thành công trình: công trình xong tháng 3 vẫn có thể phải giải ngân trước 31/12."* |
| Mô tả | textarea | |

Nút: `Huỷ` · `Thêm dự án`.

---

## 10. Modal "Nhập giải ngân"

Tiêu đề: `Nhập lần giải ngân từ Excel`
Mô tả: *"Tệp được kiểm trước và chưa ghi gì. Còn một dòng sai thì không dòng nào được nhận — sửa tệp rồi nhập lại."*
- Link `⬇ Tải mẫu giải ngân`
- Vùng kéo thả `Chọn tệp .xlsx`
- Nút `Đóng` · `Nhập`

> Quy tắc **all-or-nothing** này phải giữ đúng: validate toàn bộ file, có lỗi thì rollback, trả về danh sách dòng lỗi kèm lý do.

---

## 11. Mô hình dữ liệu

**`nam_ngan_sach`**: `id, nam, ke_hoach_von_nam, nguong_canh_bao_cham (mặc định 10), khoa bool`

**`hang_muc_von`**

| Trường | Kiểu |
|---|---|
| `id`, `ma` (slug), `ten`, `thu_tu` | |
| `nam` | int |
| `ke_hoach_von_nam` | bigint |
| `thoi_han_giai_ngan` | date |

**`nguon_von`**

| Trường | Kiểu |
|---|---|
| `id`, `ten`, `thu_tu` | |
| `nam` | int |
| `tong_nguon` | bigint |

**`du_an`**

| Trường | Kiểu | Ghi chú |
|---|---|---|
| `id` | uuid | |
| `ma` | text unique | `DA-2026-be-tong-hoa-duong-ngo-xo-2` |
| `nam` | int | |
| `hang_muc_id` | uuid | |
| `ten` | text | |
| `mo_ta` | text null | |
| `ke_hoach_von_nam` | bigint | |
| `tong_muc_duoc_duyet` | bigint null | mặc định = kế hoạch vốn năm |
| `don_vi_thuc_hien_id` | uuid null | |
| `can_bo_phu_trach_id` | uuid null | |
| `ngay_khoi_cong`, `ngay_hoan_thanh` | date null | |
| `thoi_han_giai_ngan` | date | mặc định 31/12 |
| `tao_luc`, `cap_nhat_luc` | | |

Trường tính: `da_giai_ngan` = SUM(`chung_tu_giai_ngan.so_tien` WHERE trạng thái ≥ kế toán nhập).

**`phan_bo_nguon_von`**: `id, du_an_id, nguon_von_id, so_tien_phan_bo`
→ chip `Đủ · N nguồn` khi `SUM(so_tien_phan_bo) ≥ ke_hoach_von_nam`, ngược lại `Chưa đủ`; không có bản ghi ⇒ `Chưa gắn nguồn`.

**`chung_tu_giai_ngan`**

| Trường | Kiểu |
|---|---|
| `id`, `du_an_id` | |
| `ngay_chi` | date |
| `so_tien` | bigint |
| `nguon_von_id` | uuid null |
| `noi_dung` | text |
| `doi_tac` | text null (`Công ty ABC`) |
| `so_chung_tu` | text null |
| `tep_dinh_kem` | jsonb |
| `trang_thai` | enum `ke-toan-nhap` \| `da-xac-nhan` \| `da-khoa` |
| `nguoi_nhap_id`, `nguoi_xac_nhan_id`, `thoi_diem_khoa` | |

**`vuong_mac`**: `id, du_an_id, noi_dung, nguoi_ghi_id, thoi_diem, da_go bool, thoi_diem_go, nhiem_vu_id (nhiệm vụ theo dõi tự sinh)`

---

## 12. API đề xuất

| Method | Endpoint |
|---|---|
| GET | `/api/giai-ngan/tong-quan?nam=2026` |
| GET | `/api/giai-ngan/hang-muc?nam=2026` |
| POST/PATCH/DELETE | `/api/giai-ngan/hang-muc/:id` |
| GET | `/api/giai-ngan/nguon-von?nam=2026` |
| POST/PATCH/DELETE | `/api/giai-ngan/nguon-von/:id` |
| GET | `/api/du-an?nam=&hang_muc=&q=&chi_du_an_cham=&gop_theo_hang_muc=` |
| POST | `/api/du-an` |
| GET/PATCH/DELETE | `/api/du-an/:id` |
| GET/POST | `/api/du-an/:id/chung-tu` |
| PATCH | `/api/chung-tu/:id` · `/api/chung-tu/:id/xac-nhan` · `/api/chung-tu/:id/khoa` |
| GET/POST | `/api/du-an/:id/vuong-mac` · `POST /api/vuong-mac/:id/go` |
| POST | `/api/giai-ngan/nhap-excel` (dry-run + commit) |
| GET | `/api/giai-ngan/mau-excel` |

---

## 13. Quy tắc nghiệp vụ

1. Dự án **không tự sinh trạng thái hoàn thành** — chỉ có tỷ lệ giải ngân và điểm chậm.
2. Tỷ lệ giải ngân có thể **> 100%** (ví dụ `176,5%`) — không chặn, chỉ hiển thị.
3. Chứng từ đã `Đã khoá` thì không sửa/xoá; muốn sửa phải mở khoá (quyền `budget.confirm`).
4. Ghi vướng mắc mà dự án đã có cán bộ phụ trách ⇒ **tự sinh nhiệm vụ theo dõi** liên kết ngược.
5. Ngưỡng cảnh báo chậm cấu hình theo năm ngân sách, mặc định 10 điểm.
6. Chứng từ chưa gắn nguồn vốn vẫn cộng vào tổng đã giải ngân, nhưng bị nêu ở cảnh báo *"đã chi nhưng chưa ghi rút từ nguồn nào"*.
7. Nhập Excel: all-or-nothing.
8. Năm ngân sách chuyển đổi không xoá dữ liệu năm cũ — mỗi năm là một tập dự án riêng.

---

## 14. Dữ liệu mẫu

63 dự án năm 2026, kế hoạch 33.230.000.000 đ, đã giải ngân 3.433.990.000 đ (10,33%), 29–30 dự án chậm. Phân bổ: chuyển tiếp 8 DA/800tr, xây dựng mới 13 DA/1,3 tỷ, trả nợ 19 DA/1,9 tỷ, kéo dài 21 DA/2,1 tỷ, nông thôn mới 1 DA/25 tỷ (0%), trả nợ chưa phân bổ 1 DA/2,13 tỷ (0%).
