# 10 — Bản đồ phát triển kinh tế số

**Route:** `/ban-do` · **Tiêu đề trang:** `Bản đồ phát triển kinh tế số` · **Menu:** Bản đồ kinh tế số (nhóm Điều hành)

---

## 1. Mục đích

Bản đồ GIS của xã: định vị mọi "tài nguyên" trên địa bàn — doanh nghiệp, hộ kinh doanh, hợp tác xã, chợ, trường học, cơ sở y tế, di tích, làng nghề, sản phẩm OCOP, hạ tầng công cộng, công trình đầu tư công. Kèm bộ trường tuỳ biến theo từng nhóm, lớp bản đồ nhiệt phản ánh, và sổ địa điểm dạng bảng.

Dòng meta dưới tiêu đề: `26 đối tượng · 42.3% đã xác minh`.

---

## 2. Bố cục

```
PageHeader
  "Bản đồ phát triển kinh tế số"
  "26 đối tượng · 42.3% đã xác minh"
  [Doanh nghiệp ▾]  [⬇ Mẫu Excel] [⬆ Nhập Excel] [+ Thêm đối tượng]
  [🗺 Bản đồ | ☰ Sổ địa điểm]   [⤢ Trình chiếu]

Hai cột:
 ┌─────────────────────┬──────────────────────────────────────┐
 │ Panel trái (~280px) │ Bản đồ toàn màn hình / Bảng sổ       │
 │  LỚP BẢN ĐỒ [Ẩn hết]│                                      │
 │  BỘ LỌC             │                                      │
 │  MẬT ĐỘ THEO THÔN   │                                      │
 └─────────────────────┴──────────────────────────────────────┘
```

Select đầu trang (`Doanh nghiệp ▾`) chọn **nhóm tài nguyên đang thao tác** — quyết định bộ trường của form `Thêm đối tượng` và mẫu Excel.

---

## 3. Mười một nhóm tài nguyên

| Mã | Nhãn | Số mẫu | Màu chấm |
|---|---|---|---|
| `enterprise` | Doanh nghiệp | 5 | xanh dương |
| `household_business` | Hộ kinh doanh | 3 | xanh ngọc |
| `coop` | Hợp tác xã | 1 | tím |
| `market` | Chợ, trung tâm thương mại | 2 | cam |
| `school` | Trường học | 3 | xanh lá |
| `health_facility` | Cơ sở y tế | 2 | đỏ |
| `heritage` | Di tích lịch sử – văn hoá | 2 | vàng nâu |
| `tourism` | Du lịch, làng nghề | 1 | xanh dương nhạt |
| `ocop` | Sản phẩm OCOP | 2 | nâu đỏ |
| `infrastructure` | Hạ tầng công cộng | 4 | xanh đen |
| `public_project` | Công trình đầu tư công | 1 | đen |

> Danh mục `Loại tài nguyên bản đồ` trong `Cấu hình` chỉ có **8 mục** (`doanh-nghiep`, `ho-kinh-doanh`, `cho`, `truong-hoc`, `co-so-y-te`, `di-tich`, `ocop`, `ha-tang`). Ba nhóm còn lại (`coop`, `tourism`, `public_project`) đang hard-code trong prototype. **Khi dựng lại: đưa cả 11 nhóm vào danh mục** để nhất quán.

---

## 4. Panel trái

### 4.1 LỚP BẢN ĐỒ
- Nút `👁 Ẩn hết` ở góc.
- Mỗi lớp = 1 dòng: chấm màu + tên nhóm + số lượng bên phải. Bấm để bật/tắt hiện lớp trên bản đồ.
- Cuối danh sách: công tắc **`🔥 Bản đồ nhiệt phản ánh`** (switch) — phủ heatmap phiếu phản ánh lên cùng bản đồ.

### 4.2 BỘ LỌC

| Trường | Kiểu | Giá trị |
|---|---|---|
| Ô tìm | text | `Tên, địa chỉ, mã số thuế…` |
| Ngành nghề | select | `Mọi ngành nghề` + 20 ngành (mục 5) |
| Thôn / Tổ dân phố | select | `Toàn xã` + 6 thôn |
| Quy mô lao động | select | `Mọi quy mô` + các bậc |
| Trạng thái | select | `Mọi trạng thái` (Đang hoạt động / Tạm ngừng / Đã giải thể…) |

### 4.3 MẬT ĐỘ THEO THÔN

| Thôn | Số cơ sở | Số phản ánh |
|---|---|---|
| Thôn Bình An | 7 | 1 PA |
| Thôn Bình Trị | 3 | 1 PA |
| Thôn Bình Dương | 3 | 1 PA |
| Thôn Hà Lam | 9 | 3 PA |
| Thôn Trường Giang | 2 | 1 PA |
| Thôn Phước Ấm | 2 | 1 PA |

Nền mỗi dòng tô đậm nhạt theo mật độ (thanh nền trong ô).
Chú thích: *"Cột trái: số cơ sở. Cột phải: số phản ánh của người dân trên cùng địa bàn."*

---

## 5. Danh mục ngành nghề (20 mã, theo VSIC cấp 2)

| Mã | Nhãn |
|---|---|
| 01 | Nông nghiệp, trồng trọt, chăn nuôi |
| 03 | Khai thác, nuôi trồng thuỷ sản |
| 10 | Chế biến thực phẩm |
| 13 | Dệt |
| 14 | May mặc |
| 16 | Chế biến gỗ |
| 22 | Sản phẩm cao su, nhựa |
| 23 | Vật liệu xây dựng |
| 25 | Sản phẩm kim loại, cơ khí |
| 41 | Xây dựng nhà |
| 42 | Xây dựng công trình |
| 45 | Bán, sửa chữa ô tô, xe máy |
| 46 | Bán buôn |
| 47 | Bán lẻ |
| 49 | Vận tải đường bộ |
| 55 | Lưu trú |
| 56 | Ăn uống |
| 85 | Giáo dục và đào tạo |
| 86 | Y tế |
| 96 | Dịch vụ khác |

---

## 6. Chế độ "Bản đồ"

- Nền OpenStreetMap, thanh tỷ lệ góc dưới phải (`300 m`), nút `+` / `−` góc trên phải.
- Marker tròn màu theo nhóm. Bấm marker → popup thông tin tóm tắt + link mở chi tiết.
- Khi bật heatmap phản ánh: lớp nhiệt đỏ/vàng phủ dưới marker.
- `⤢ Trình chiếu` — chế độ phòng họp.

---

## 7. Chế độ "Sổ địa điểm"

Bảng **gom nhóm theo loại tài nguyên**, mỗi nhóm có hàng tiêu đề `● Doanh nghiệp  5 địa điểm`.

| Cột | Nội dung |
|---|---|
| Địa điểm | tên (đậm) + dòng phụ địa chỉ/thôn |
| Thôn / Tổ dân phố | |
| Người đại diện | tên + dòng phụ số điện thoại; `—` khi trống |
| Trạng thái | `Đang hoạt động` · `Đã giải thể` … |
| Xác minh | chip `Đã xác minh` (xanh) / `Chưa xác minh` (xám) |
| *(hành động)* | `📍` xem trên bản đồ · `✎` sửa · `🗑` xoá |

---

## 8. Modal "Thêm đối tượng"

Tiêu đề: `Thêm đối tượng lên bản đồ`
Mô tả: *"Bắt buộc: nhóm, tên và vị trí. Phần còn lại điền được lúc nào cũng được."*

### 8.1 Trường chung

| Trường | Kiểu | Bắt buộc |
|---|---|---|
| Nhóm | select 11 nhóm | ✔ |
| Trạng thái | select, mặc định `Đang hoạt động` | |
| Tên | text | ✔ |
| Địa chỉ | text | |
| Thôn / Tổ dân phố | select, mặc định `— Chưa xác định —` | |
| **Vị trí trên bản đồ** | ô tìm địa điểm (`Tìm theo tên: chợ Hà Lam, trường tiểu học, thôn Bình An…`) + bản đồ nhúng | ✔ |
| | chú thích: *"Bấm vào bản đồ để đặt ghim, rồi kéo ghim để chỉnh cho đúng."* | |
| Vĩ độ / Kinh độ | number (6 chữ số thập phân), mặc định `15.730507` / `108.378110` | |
| | nút `⊕ Vị trí của tôi` — lấy toạ độ GPS trình duyệt | |
| Người đại diện | text | |
| Điện thoại | text | |
| Mô tả | textarea | |

### 8.2 Trường riêng theo nhóm (trường tuỳ biến)

Với nhóm `Doanh nghiệp`:

| Trường | Kiểu | Ghi chú |
|---|---|---|
| Mã số thuế | text | placeholder `Trùng mã là cập nhật, không tạo bản ghi mới` |
| Ngành nghề | select 20 ngành | `— Chưa phân ngành —` |
| Số lao động | number | |
| Loại hình doanh nghiệp | select | `— Chọn —` — **trường tuỳ biến** khai ở `Cấu hình → Trường bản đồ` |
| Doanh thu ước (đồng/năm) | number | **trường tuỳ biến** |

> `Loại hình doanh nghiệp` (mã `legal_form`, kiểu `Chọn trong danh sách`) và `Doanh thu ước (đồng/năm)` (mã `revenue_estimate`, kiểu `Số thập phân`) là hai bản ghi trong bảng `truong_tuy_bien_ban_do`, **không hard-code**. Mỗi nhóm tài nguyên có bộ trường tuỳ biến riêng, khai ở `Cấu hình → Trường bản đồ`.

Nút: `Huỷ` · `Thêm vào bản đồ`.

---

## 9. Nhập / Mẫu Excel

- `⬇ Mẫu Excel` — tải mẫu **theo nhóm đang chọn**, cột = trường chung + trường tuỳ biến của nhóm đó.
- `⬆ Nhập Excel` — nạp hàng loạt. Khớp theo **Mã số thuế** (hoặc khoá định danh của nhóm): trùng mã ⇒ cập nhật, không tạo bản ghi mới.

---

## 10. Mô hình dữ liệu

**`doi_tuong_ban_do`**

| Trường | Kiểu | Ghi chú |
|---|---|---|
| `id` | uuid | |
| `nhom` | enum | 11 mã mục 3 |
| `ten` | text | |
| `dia_chi` | text null | |
| `thon_id` | uuid null | |
| `lat`, `lng` | numeric(10,6) | bắt buộc |
| `nguoi_dai_dien` | text null | |
| `dien_thoai` | text null | |
| `trang_thai` | enum | `dang-hoat-dong` \| `tam-ngung` \| `da-giai-the` |
| `da_xac_minh` | bool | → tỷ lệ `42.3% đã xác minh` |
| `xac_minh_luc`, `nguoi_xac_minh_id` | | |
| `ma_so_thue` | text null, unique khi có | khoá khớp khi nhập Excel |
| `nganh_nghe` | text null | mã VSIC 2 chữ số |
| `so_lao_dong` | int null | |
| `mo_ta` | text null | |
| `gia_tri_tuy_bien` | jsonb | `{truong_id/ma: giá trị}` |
| `ngay_thanh_lap` | date null | dùng cho KPI `Thành lập mới` |
| `tao_luc`, `cap_nhat_luc` | | |

**`truong_tuy_bien_ban_do`**

| Trường | Kiểu | Ví dụ |
|---|---|---|
| `id` | uuid | |
| `nhom` | enum | `enterprise` |
| `nhan_hien_thi` | text | `Loại hình doanh nghiệp` |
| `ma_truong` | slug | `legal_form` |
| `kieu_du_lieu` | enum | `Chọn trong danh sách` \| `Số thập phân` \| `Văn bản` \| `Ngày` \| `Số nguyên` \| `Đúng/Sai` |
| `lua_chon` | jsonb null | với kiểu chọn |
| `bat_buoc` | bool | |
| `thu_tu` | int | |
| `trang_thai` | enum | `Đang dùng` \| `Tắt` |

Chú thích UI (giữ nguyên): *"Cột trong tệp Excel nhập vào chỉ được giữ lại khi có trường tương ứng ở đây. Trường đã xoá thì dữ liệu cũ vẫn còn trong hồ sơ, chỉ không hiện lên trên biểu mẫu nữa."*

---

## 11. API đề xuất

| Method | Endpoint |
|---|---|
| GET | `/api/ban-do/doi-tuong?nhom=&q=&nganh=&thon=&quy_mo=&trang_thai=` |
| POST | `/api/ban-do/doi-tuong` |
| GET/PATCH/DELETE | `/api/ban-do/doi-tuong/:id` |
| POST | `/api/ban-do/doi-tuong/:id/xac-minh` |
| GET | `/api/ban-do/lop` — số lượng từng nhóm |
| GET | `/api/ban-do/mat-do-thon` |
| GET | `/api/ban-do/mau-excel?nhom=enterprise` |
| POST | `/api/ban-do/nhap-excel?nhom=enterprise` |
| GET | `/api/ban-do/truong-tuy-bien?nhom=` |
| GET | `/api/phan-anh/ban-do-nhiet` — dữ liệu heatmap |
| GET | `/api/ban-do/tim-dia-diem?q=` — geocoding (Nominatim hoặc dịch vụ trong nước) |

---

## 12. Quy tắc nghiệp vụ

1. Vị trí (lat/lng) **bắt buộc** — không có toạ độ thì không lên bản đồ được.
2. Trùng `ma_so_thue` khi nhập Excel ⇒ **cập nhật** bản ghi cũ.
3. Tỷ lệ xác minh = `count(da_xac_minh) / count(*)`.
4. KPI `KINH TẾ & TÀI NGUYÊN` ở `/tong-quan` lấy trực tiếp từ bảng này: Doanh nghiệp = nhóm `enterprise`, Hộ kinh doanh = `household_business`, Thành lập mới = `ngay_thanh_lap` trong kỳ, Tổng tài nguyên = tổng mọi nhóm.
5. Quyền: `asset.read` (xem bản đồ tài nguyên), `asset.update` (cập nhật tài nguyên).
6. Xoá trường tuỳ biến **không xoá dữ liệu** đã nhập — chỉ ẩn khỏi biểu mẫu.
7. Toạ độ mặc định khi mở form = trung tâm xã (`15.730507, 108.378110` với Thăng Bình).

---

## 13. Dữ liệu mẫu

26 đối tượng, 42,3% đã xác minh. Ví dụ nhóm Doanh nghiệp (5):
- Công ty CP Chế biến Nông sản Bình An — Thôn Bình An — Người đại diện demo 2 · 0900 222 002 — Đang hoạt động — Đã xác minh
- Công ty TNHH Cơ khí Trường Giang — Thôn Trường Giang — demo 3 · 0900 222 003 — Chưa xác minh
- Công ty TNHH May Thăng Bình — Cụm công nghiệp Hà Lam — demo 1 · 0900 222 001 — Đã xác minh
- Công ty TNHH Thương mại Hà Lam (đã giải thể) — Thôn Hà Lam — Đã giải thể
- Công ty TNHH Xây dựng Phước Ấm — Thôn Phước Ấm — demo 4 · 0900 222 004 — Chưa xác minh

Hộ kinh doanh (3): Cơ sở nước mắm Hà Lam, Tạp hoá Bình Trị, Xưởng mộc Bình Dương.
Hợp tác xã (1): Hợp tác xã Nông nghiệp Thăng Bình — Thôn Bình An.
Chợ (2): Chợ Bình Trị, …
