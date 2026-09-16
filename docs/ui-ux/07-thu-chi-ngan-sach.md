# 07 — Thu – Chi ngân sách xã

**Route:** `/giai-ngan/thu-chi` · **Tiêu đề trang:** `Thu - Chi ngân sách · ViGov` · **Menu:** Thu - Chi ngân sách (nhóm Điều hành)

---

## 1. Mục đích

Mô tả trong giao diện: *"Nạp thẳng tệp Excel của Phòng Tài chính. Một tệp hai sheet nạp được cả thu lẫn chi trong một lần, khoản mục và cột sinh ra theo đúng tệp."*

Đây là module **bảng cây ngân sách động**: cấu trúc khoản mục và tập cột **không cố định trong code** mà sinh ra từ file Excel do Phòng Tài chính cấp. Người dùng sau đó chỉnh sửa trực tiếp trên lưới.

Đơn vị tính: **Triệu đồng**.

---

## 2. Bố cục

```
PageHeader
  "Thu - Chi ngân sách xã"
  "Nạp thẳng tệp Excel của Phòng Tài chính. Một tệp hai sheet nạp được cả thu lẫn chi
   trong một lần, khoản mục và cột sinh ra theo đúng tệp."
  Nút phải: [⬆ Nạp từ Excel]  [🗑 Gỡ]

Tabs (segmented):  [Chi ngân sách 2026]  [Thu ngân sách 2026]

Thẻ tiêu đề báo cáo (nền trắng):
  "BÁO CÁO CHI NGÂN SÁCH NHÀ NƯỚC XÃ THĂNG BÌNH NĂM 2026"
  Đơn vị tính: Triệu đồng · Luỹ kế đến 25/8/2026 · 59 khoản mục
  "Con số tổng lấy từ dòng {Tổng số}. Bấm ngôi sao ở đầu một dòng khác để đổi."
  Hàng ô tóm tắt (sinh theo các cột của bảng)

Thanh công cụ:
  [› Chỉ xem mục lớn] [⌄ Mở hết chi tiết]  đang hiện 59/59 khoản mục
                                          [⊞ Thêm khoản mục cấp cao nhất]

Bảng cây khoản mục
```

---

## 3. Hai tab, hai bộ cột khác nhau

### 3.1 Tab "Chi ngân sách {năm}"

Tiêu đề: `BÁO CÁO CHI NGÂN SÁCH NHÀ NƯỚC XÃ THĂNG BÌNH NĂM 2026`

Ô tóm tắt:

| Nhãn | Giá trị |
|---|---|
| Dự toán năm | 3.794.740 |
| Chi ngân sách | 3.463.459,2 |
| So sánh TH/DT (%) | 91,3% |

Cột bảng: `TT` · `Nội dung` · `Dự toán năm` · `Chi ngân sách` · `So sánh TH/DT (%)` · `Cách tính` · *(3 nút hành động)*

### 3.2 Tab "Thu ngân sách {năm}"

Tiêu đề: `THU NGÂN SÁCH XÃ THĂNG BÌNH NĂM 2026`

Ô tóm tắt:

| Nhãn | Giá trị |
|---|---|
| Dự toán 2026 TP giao | 3.993.010 |
| Dự toán 2026 Xã giao | 4.681.290 |
| Thu ngân sách NSNN | 4.316.764,3 |
| Thu ngân sách Thu xã hưởng | 3.300.800,5 |
| Tỷ lệ % thu | 108,1% |

Cột bảng: `TT` · `Nội dung` · `Dự toán 2026 TP giao` · `Dự toán 2026 Xã giao` · `Thu ngân sách NSNN` · `Thu ngân sách Thu xã hưởng` · `Tỷ lệ % thu` · `Cách tính`

> **Kết luận thiết kế:** cột là dữ liệu, không phải schema. Bảng `cot_ngan_sach` định nghĩa từng cột (tên, kiểu, thứ tự, có phải cột tính %, công thức). Giá trị lưu ở `gia_tri_khoan_muc(khoan_muc_id, cot_id, gia_tri)`.

---

## 4. Bảng cây khoản mục

### 4.1 Cấu trúc hàng

```
⌄ ☆ │ A  │ CHI NGÂN SÁCH NHÀ NƯỚC     │ 5.502.660 │ 3.401.673,3 │ 61,8% │ [Nhập trực tiếp ▾] │ ⇄ ＋ 🗑
  ⌄ │ I  │   Chi đầu tư phát triển     │   977.310 │   870.952,7 │ 89,1% │ [Nhập trực tiếp ▾] │ ⇄ ＋ 🗑
    │    │     Đầu tư cho các DA…      │        —  │    82.962,7 │       │ [Nhập trực tiếp ▾] │ ⇄ ＋ 🗑
  ⌄ │    │   Tr.đó: Từ nguồn vốn TPCP  │   977.310 │   787.990,1 │ 80,6% │ [Nhập trực tiếp ▾] │ ⇄ ＋ 🗑
    │1.1 │     Chi quốc phòng          │    29.060 │    29.658,6 │       │ [Nhập trực tiếp ▾] │ ⇄ ＋ 🗑
```

| Thành phần | Hành vi |
|---|---|
| Mũi tên `⌄` | thu gọn / mở nhánh con (chỉ hiện khi có con) |
| `☆` ngôi sao | aria-label `Đặt "{tên}" làm con số tổng` — chỉ định dòng nào là con số tổng hiển thị ở ô tóm tắt. Dòng đang là tổng có sao đặc màu vàng. |
| `TT` | số thứ tự theo tệp gốc: `A`, `I`, `1`, `1.1`, `-`… tự do, là **text** không phải số |
| `Nội dung` | button aria-label `Bấm để sửa tên khoản mục` — sửa tại chỗ |
| Các ô số | button — bấm để sửa tại chỗ. Giá trị rỗng hiện `—`. |
| Ô `%` | tính tự động, không sửa |
| `Cách tính` | select 3 giá trị (mục 4.2) |
| `⇄` | aria-label `Các đợt thu, chi của {tên}` — mở dialog đợt |
| `＋` | aria-label `Thêm khoản mục con dưới {tên}` |
| `🗑` | aria-label `Gỡ khoản mục {tên}` |

Màu chữ: cấp cao (A, I) tô xanh lá / cam theo % đạt; dòng chi tiết để màu thường.

### 4.2 "Cách tính" — ba chế độ mỗi khoản mục

| Giá trị | Mã | Nghĩa |
|---|---|---|
| Nhập trực tiếp *(mặc định)* | `manual` | Gõ số tay vào ô |
| Cộng theo đợt | `entries` | Số = tổng các đợt thu/chi đã ghi ở dialog `⇄`; ô số thành read-only |
| Cộng khoản mục con | `children` | Số = tổng các khoản mục con trực tiếp; ô số thành read-only |

Đây là điểm mạnh của module — cho phép trộn: cấp trên cộng con, cấp lá nhập tay hoặc cộng theo đợt.

### 4.3 Thanh công cụ

| Nút | Hành vi |
|---|---|
| `› Chỉ xem mục lớn` | thu gọn về các cấp trên cùng |
| `⌄ Mở hết chi tiết` | mở toàn cây |
| `đang hiện {x}/{y} khoản mục` | bộ đếm |
| `⊞ Thêm khoản mục cấp cao nhất` | thêm node gốc |

---

## 5. Dialog "Các đợt thu, chi"

Tiêu đề = tên khoản mục (viết HOA).
Mô tả: *"Ghi từng đợt thu, chi rồi hệ thống cộng lại. Con số của khoản mục này lấy từ tổng các đợt bên dưới, không gõ thẳng nữa."*

Form ghi một đợt:

| Trường | Kiểu | Ghi chú |
|---|---|---|
| Ngày | date | mặc định hôm nay |
| Nội dung | text | placeholder `Thu tiền sử dụng đất đợt 2` |
| Đơn vị, cá nhân | text | |
| Số chứng từ | text | |
| *(một ô số cho mỗi cột của tab hiện tại)* | number | ví dụ tab Thu: `Dự toán 2026 TP giao`, `Dự toán 2026 Xã giao`, `Thu ngân sách NSNN`, `Thu ngân sách Thu xã hưởng` |

Nút `+ Ghi đợt`. Bên dưới là danh sách đợt đã ghi, rỗng thì hiện `Chưa ghi đợt nào.`

---

## 6. Nạp từ Excel / Gỡ

- `⬆ Nạp từ Excel`: chọn `.xlsx`. **Một tệp hai sheet** (một sheet Thu, một sheet Chi) nạp được cả hai tab trong một lần. Hệ thống đọc hàng tiêu đề để sinh cột, đọc cột `TT` + thụt lề để dựng cây khoản mục.
- `🗑 Gỡ`: xoá toàn bộ dữ liệu của tab/năm hiện tại (phải có hộp xác nhận, cảnh báo không hoàn tác).
- Luỹ kế hiển thị `Luỹ kế đến {ngày}` — lấy từ tệp hoặc nhập tay.

---

## 7. Mô hình dữ liệu

**`bang_ngan_sach`** (một bảng = một tab × một năm)

| Trường | Kiểu | Ghi chú |
|---|---|---|
| `id` | uuid | |
| `nam` | int | |
| `loai` | enum | `thu` \| `chi` |
| `tieu_de` | text | `THU NGÂN SÁCH XÃ THĂNG BÌNH NĂM 2026` |
| `don_vi_tinh` | text | `Triệu đồng` |
| `luy_ke_den` | date | `25/8/2026` |
| `khoan_muc_tong_id` | uuid null | dòng được chọn làm con số tổng (ngôi sao) |
| `nguon_tep` | text null | tên file Excel đã nạp |
| `nap_luc` | timestamp | |

**`cot_ngan_sach`**

| Trường | Kiểu | Ghi chú |
|---|---|---|
| `id`, `bang_id` | | |
| `ten` | text | `Thu ngân sách NSNN` |
| `thu_tu` | int | |
| `kieu` | enum | `so` \| `phan_tram` |
| `cong_thuc` | text null | với cột %: ví dụ `col_4 / col_2 * 100` |

**`khoan_muc_ngan_sach`**

| Trường | Kiểu | Ghi chú |
|---|---|---|
| `id`, `bang_id` | | |
| `cha_id` | uuid null | cây |
| `tt` | text | `A`, `I`, `1.1`, `-` |
| `ten` | text | |
| `thu_tu` | int | |
| `cach_tinh` | enum | `manual` \| `entries` \| `children` |
| `cap` | int | độ sâu, dùng để thụt lề |

**`gia_tri_khoan_muc`**: `khoan_muc_id, cot_id, gia_tri numeric` (PK kép)

**`dot_thu_chi`**

| Trường | Kiểu |
|---|---|
| `id`, `khoan_muc_id` | |
| `ngay` | date |
| `noi_dung` | text |
| `don_vi_ca_nhan` | text null |
| `so_chung_tu` | text null |
| `gia_tri` | jsonb — `{cot_id: số}` |
| `nguoi_ghi_id`, `tao_luc` | |

---

## 8. API đề xuất

| Method | Endpoint |
|---|---|
| GET | `/api/ngan-sach?nam=2026&loai=chi` — trả bảng + cột + cây khoản mục + giá trị |
| POST | `/api/ngan-sach/nap-excel` (multipart, nạp cả 2 sheet) |
| DELETE | `/api/ngan-sach?nam=2026&loai=chi` — Gỡ |
| POST | `/api/ngan-sach/:bangId/khoan-muc` — thêm (kèm `cha_id` hoặc null) |
| PATCH/DELETE | `/api/khoan-muc/:id` |
| PATCH | `/api/khoan-muc/:id/gia-tri` — `{cot_id, gia_tri}` |
| PATCH | `/api/khoan-muc/:id/cach-tinh` |
| PATCH | `/api/ngan-sach/:bangId/dong-tong` — `{khoan_muc_id}` (ngôi sao) |
| GET/POST | `/api/khoan-muc/:id/dot` |
| DELETE | `/api/dot/:id` |

---

## 9. Quy tắc nghiệp vụ

1. Đổi `cach_tinh` sang `entries`/`children` thì **giá trị nhập tay bị khoá** và tính lại ngay; đổi ngược lại thì giữ giá trị vừa tính làm giá trị khởi đầu.
2. Cộng dồn `children` chỉ tính **con trực tiếp** (không cộng cháu, tránh nhân đôi khi cây nhiều tầng đều đặt `children`).
3. Cột `%` không lưu, tính khi render; mẫu số = 0 ⇒ hiển thị trống.
4. Giá trị có thể âm hoặc trống; trống hiển thị `—`, không hiển thị `0`.
5. Số tổng ở thẻ tóm tắt lấy từ **dòng được đánh sao**, mặc định dòng đầu tiên (`Tổng số` ở tab Chi, `TỔNG THU NỘI ĐỊA PHÁT SINH TRÊN ĐỊA BÀN` ở tab Thu).
6. Số liệu này nuôi nhóm KPI `THU - CHI NGÂN SÁCH XÃ` ở `/tong-quan` và `/bao-cao`:
   - `Thu đạt dự toán = Thu ngân sách NSNN / Dự toán Xã giao`
   - `Chi đạt dự toán = Chi ngân sách / Dự toán năm`
   - `Cân đối thu - chi = Tổng thu − Tổng chi`
7. Quyền: xem/sửa theo nhóm `GIẢI NGÂN` (`budget.read`, `budget.update`, `budget.confirm`).

---

## 10. Dữ liệu mẫu

**Chi 2026**: 59 khoản mục, dự toán năm 3.794.740, chi 3.463.459,2 (91,3%). Cây: `Tổng số` → `A. CHI NGÂN SÁCH NHÀ NƯỚC` (5.502.660 / 3.401.673,3 / 61,8%) → `I. Chi đầu tư phát triển` (977.310 / 870.952,7 / 89,1%) → `Đầu tư cho các DA theo các lĩnh vực`, `Tr.đó: Từ nguồn vốn TPCP` → `1.1 Chi quốc phòng`, `1.2 Chi an ninh và trật tự, an toàn xã hội`, `1.3 Chi giáo dục, đào tạo và dạy nghề`, `1.4 Chi khoa học và công nghệ`, `1.5 Chi y tế, dân số và gia đình`…

**Thu 2026**: 52 khoản mục. Cây gồm `A. TỔNG THU NỘI ĐỊA PHÁT SINH TRÊN ĐỊA BÀN` → `I. THUẾ THÀNH PHỐ QUẢN LÝ THU` (10 mục con, mỗi mục có `Thuế GTGT / TNDN / tài nguyên`) và `II. THUẾ CƠ SỞ QUẢN LÝ THU` (9 mục con); `B. THU NGÂN SÁCH ĐỊA PHƯƠNG` → `1. Thu nội địa NSĐP được hưởng theo phân cấp`, `2. Thu bổ sung từ ngân sách cấp trên` (Bổ sung cân đối / CCTL / mục tiêu), `3. Thu từ ngân sách cấp dưới nộp lên`, `4. Thu chuyển nguồn ngân sách`.
