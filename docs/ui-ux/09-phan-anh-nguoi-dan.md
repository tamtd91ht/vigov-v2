# 09 — Phản ánh của người dân

**Route:** `/phan-anh` · **Tiêu đề trang:** `Phản ánh của người dân · ViGov` · **Menu:** Phản ánh người dân (nhóm Điều hành)

---

## 1. Mục đích

Mô tả trong giao diện: *"Tiếp nhận từ Zalo Mini App và các kênh khác, theo dõi thời hạn, đối chiếu ảnh trước và sau khi xử lý."*

Module có **vòng đời phức tạp nhất** sau Nhiệm vụ: 9 trạng thái, SLA riêng cho từng lĩnh vực, bắt buộc ảnh sau xử lý, đánh giá sao của người dân có khả năng **tự mở lại phiếu**, và cơ chế kiểm duyệt trước khi hiện công khai.

---

## 2. Bố cục

```
PageHeader
  "Phản ánh của người dân"
  "Tiếp nhận từ Zalo Mini App và các kênh khác, theo dõi thời hạn, đối chiếu ảnh
   trước và sau khi xử lý."
  Nút: [+ Nhập hộ phản ánh]

4 thẻ KPI (lưới 4 cột)

Tabs: [Danh sách (21)] [Bản đồ nhiệt] [Báo cáo]

── Tab Danh sách ──
  [Toàn xã | Giao cho tôi | Liên quan đến tôi]
  [🔍 Tìm theo nội dung, mã phiếu, địa chỉ…] [Tất cả trạng thái ▾]
  [Tất cả lĩnh vực ▾] [Tất cả địa bàn ▾]
  ☐ Chỉ phiếu trễ hạn   ☐ Bị đánh giá thấp
  Danh sách thẻ phiếu (dạng card list, không phải bảng)
```

---

## 3. Bốn thẻ KPI

| Nhãn | Giá trị | Dòng phụ |
|---|---|---|
| TỔNG PHẢN ÁNH 90 NGÀY | 21 | `20 phiếu đang xử lý` |
| ĐÚNG HẠN / TRỄ HẠN | `0 / 21` *(số trễ đỏ)* | `⚠ 0% đúng hạn` |
| ĐIỂM HÀI LÒNG TRUNG BÌNH | `4/5` | `1 phiếu bị đánh giá thấp` |
| CHỜ KIỂM DUYỆT | 16 | `Kiểm duyệt trước khi hiển thị công khai` |

---

## 4. Bộ lọc

| Bộ lọc | Giá trị |
|---|---|
| Phạm vi | `Toàn xã` · `Giao cho tôi` · `Liên quan đến tôi` |
| Tìm | `Tìm theo nội dung, mã phiếu, địa chỉ…` |
| Trạng thái | 9 giá trị (mục 6) |
| Lĩnh vực | 12 giá trị (mục 5) |
| Địa bàn | `Tất cả địa bàn` + 6 thôn |
| ☐ Chỉ phiếu trễ hạn | |
| ☐ Bị đánh giá thấp | phiếu 1–2 sao |

---

## 5. Danh mục "Lĩnh vực phản ánh" (12 mục)

| Mã | Nhãn | Thứ tự |
|---|---|---|
| `rac-thai` | Rác thải – Vệ sinh môi trường | 1 |
| `giao-thong` | Hạ tầng giao thông | 2 |
| `cap-thoat-nuoc` | Cấp thoát nước | 3 |
| `dien` | Điện | 4 |
| `trat-tu-do-thi` | Trật tự đô thị – lấn chiếm vỉa hè | 5 |
| `an-ninh` | An ninh trật tự | 6 |
| `xay-dung` | Xây dựng không phép | 7 |
| `o-nhiem` | Ô nhiễm (tiếng ồn, khí thải, nước thải) | 8 |
| `y-te-giao-duc` | Y tế – Giáo dục | 9 |
| `can-bo` | Thái độ / tác phong cán bộ | 10 |
| `an-toan-thuc-pham` | An toàn thực phẩm | 11 |
| `khac` | Khác | 12 |

> Lĩnh vực `can-bo` là **lĩnh vực hạn chế**: chỉ người có quyền `feedback.restricted` mới xem được.

---

## 6. Chín trạng thái

Danh sách **đóng** — xã không thêm, không bớt (ADR 0027 quyết định B, khách chốt 20/09/2026).
Mã viết **tiếng Việt không dấu**, kebab-case, theo ADR 0011: giá trị enum nằm trong hồ sơ lưu
trữ nên không di trú được về sau.

> ⚠ **CÁCH VIẾT từng mã dưới đây là do phía thi công SUY RA từ nhãn tiếng Việt, khách chưa
> duyệt.** Khách đã chốt *danh sách chín trạng thái* và chốt *mã viết tiếng Việt* — chưa chốt
> chín chuỗi cụ thể. Vì giá trị enum đi vào hồ sơ lưu trữ và không sửa lại được (luật 7),
> **phải hỏi một lần trước khi migration đầu tiên chạm cột `trang_thai`**.

| Mã | Nhãn | Nhóm |
|---|---|---|
| `da-tiep-nhan` | Đã tiếp nhận | luồng chính — **tự động**, phần mềm sinh phiếu |
| `dang-phan-loai` | Đang phân loại | luồng chính — **cán bộ động vào lần đầu tại đây** |
| `da-chuyen-xu-ly` | Đã chuyển xử lý | luồng chính |
| `dang-xu-ly` | Đang xử lý | luồng chính |
| `da-xu-ly` | Đã xử lý | luồng chính |
| `cho-dan-xac-nhan` | Chờ dân xác nhận | luồng chính |
| `da-dong` | Đã đóng | luồng chính |
| `khong-tiep-nhan` | Không tiếp nhận | rẽ nhánh |
| `chuyen-cap-tren` | Chuyển cấp trên | rẽ nhánh |

```
da-tiep-nhan → dang-phan-loai → da-chuyen-xu-ly → dang-xu-ly → da-xu-ly → cho-dan-xac-nhan → da-dong
                     ↘ khong-tiep-nhan
                     ↘ chuyen-cap-tren
```
Đánh giá 1–2 sao ở bước `cho-dan-xac-nhan`/`da-dong` ⇒ **tự mở lại phiếu** về `dang-xu-ly`.

> **Bản trước của mục này ghi mã tiếng Anh** (`received`, `screening`, `assigned`…), chép từ
> định danh bên trong bản mẫu xã đang chạy. Sửa ngày 20/09/2026 theo yêu cầu của khách.
> Một chỗ phải đọc bằng mắt chứ không bằng từ điển: `out_of_scope` dịch sát là *"ngoài phạm
> vi"*, còn việc hành chính thật là **chuyển cho nơi có thẩm quyền nhận** — nên mã mới lấy
> theo **nhãn tiếng Việt**, không lấy theo chuỗi tiếng Anh cũ.

---

## 7. Thẻ phiếu trong danh sách

```
┃ [ảnh thumbnail]  PA-2026-0021   [Rác thải – Vệ sinh môi trường]
┃                  test nè
┃                  📍 nnj njb
┃                  👤 Không rõ người gửi · 0905 214 778
┃                  [Đang phân loại] [⚠ Quá hạn 3 ngày]              🔗 Zalo Mini App
```

| Thành phần | Ghi chú |
|---|---|
| Viền trái | đỏ khi trễ hạn |
| Thumbnail | ảnh đầu tiên; placeholder icon khi không có |
| Mã phiếu | `PA-{yyyy}-{0000}` |
| Chip lĩnh vực | nền màu theo lĩnh vực |
| Nội dung | 2 dòng |
| Địa chỉ | `📍 {địa chỉ}` hoặc `Chưa rõ vị trí` |
| Người gửi | `{họ tên} · {sđt}`, hoặc `Không rõ người gửi`, hoặc `Gửi ẩn danh` |
| Chip trạng thái + chip trễ hạn | |
| Góc phải | **kênh tiếp nhận**: `Zalo Mini App` · `Zalo OA` · `Web của xã` · `Cán bộ, trưởng thôn nhập hộ`; hoặc **sao đánh giá** `★★★★★` / `★★☆☆☆` khi đã có đánh giá |

---

## 8. DetailDrawer phiếu phản ánh

Mở gần toàn màn hình, nút `✕` góc phải.

### 8.1 Header
```
PA-2026-0021 · Zalo Mini App · 14:20 09/09/2026
Rác thải – Vệ sinh môi trường
Không rõ người gửi · 0905 214 778
```

### 8.2 StatusStepper
Bảy ô luồng chính nằm ngang, ô hiện tại tô cam + nhãn `đang ở đây`, ô kế tiếp có nhãn `chuyển sang` (bấm để chuyển):
```
[Đã tiếp nhận —][Đang phân loại (đang ở đây)][Đã chuyển xử lý (chuyển sang)][Đang xử lý —]
[Đã xử lý —][Chờ dân xác nhận —][Đã đóng —]
Rẽ nhánh:  [Không tiếp nhận (chuyển sang)]   [Chuyển cấp trên (chuyển sang)]
```
Dưới stepper là câu giải thích trạng thái hiện tại, ví dụ: *"Đang xem phiếu thuộc lĩnh vực nào, có tiếp nhận không."*

### 8.3 Ba ô tóm tắt

| Ô | Nội dung |
|---|---|
| `HẠN XỬ LÝ` | `⚠ Quá hạn 3 ngày` (đỏ) + `Hạn cuối 10/9/2026` |
| `ĐANG GIAO CHO` | bộ phận (HOA) + `Họ tên — email` |
| `HIỂN THỊ VỚI NGƯỜI DÂN` | `Chưa cho hiện công khai` + chú thích *"Chỉ cán bộ trong xã xem được. Người gửi vẫn tra cứu được phiếu của mình."* + nút `👁 Cho hiện công khai` / `🚫 Ẩn khỏi trang công khai` |

### 8.4 Nội dung & ảnh

- `Nội dung phản ánh` — text.
- `Ảnh trước và sau khi xử lý` — hai cột:
  - `TRƯỚC KHI XỬ LÝ` — ảnh người dân gửi.
  - `SAU KHI XỬ LÝ` — rỗng thì hiện chữ đỏ: **"Chưa có ảnh. Bắt buộc phải có trước khi đóng phiếu."** + nút `⬆ Tải ảnh sau xử lý`.
- `Vị trí` — địa chỉ + bản đồ nhúng với marker.

### 8.5 Chuyển xử lý
Tiêu đề khối: `Chuyển xử lý, không đổi trạng thái`
- `Bộ phận` — select, mặc định `— Chọn bộ phận —`
- `Cán bộ xử lý` — select, mặc định `— Để bộ phận phân công —`, option `Họ tên — email · Chức danh`
- Nút `Chuyển xử lý`

### 8.6 Ghi nhận đánh giá của người dân
Chú thích: *"Dùng khi người dân đánh giá qua điện thoại hoặc tại quầy. Một đến hai sao sẽ tự mở lại phiếu."*
Năm nút `1 ★` … `5 ★`.

### 8.7 Nhật ký xử lý (cột phải)
- Ô nhập `Đã làm gì, ai làm, còn vướng gì…`
- Nút `📎 Đính kèm` · `➤ Ghi nhật ký`
- Dòng thời gian, mỗi mục: avatar + tên + thời điểm + chip trạng thái + (khi có thay đổi phân công) `Bộ phận: …` / `Phụ trách: … — email` + nội dung.

---

## 9. Tab "Bản đồ nhiệt"

Bản đồ nền OSM phủ heatmap mật độ phiếu theo toạ độ. Cùng dữ liệu với công tắc `Bản đồ nhiệt phản ánh` ở module Bản đồ kinh tế số.

---

## 10. Tab "Báo cáo"

### 10.1 Dải "Đúng hạn và trễ hạn"
Bốn số lớn nằm ngang + thanh ngang tỷ lệ (đỏ/xanh):
`0 đúng hạn` · `21 trễ hạn` · `0% tỷ lệ đúng hạn` · `4/5 điểm hài lòng`

### 10.2 Bảng "Theo lĩnh vực bị phản ánh nhiều nhất"

| LĨNH VỰC | TỔNG | ĐÃ XỬ LÝ | TRỄ HẠN | HÀI LÒNG |
|---|---|---|---|---|
| Rác thải – Vệ sinh môi trường | 9 | 0 | 9 | — |
| Điện | 3 | 0 | 3 | 5 |
| An ninh trật tự | 3 | 0 | 3 | — |
| … | | | | |

Sắp xếp giảm dần theo `TỔNG`. Cột `TRỄ HẠN` đỏ.

### 10.3 Bảng "Theo bộ phận"

| BỘ PHẬN | TỔNG | ĐÚNG HẠN | TRỄ | TỶ LỆ ĐÚNG HẠN | TB GIỜ XỬ LÝ | HÀI LÒNG |
|---|---|---|---|---|---|---|
| VĂN PHÒNG ĐẢNG ỦY | 9 | 0 | 9 | 0% | 73 giờ | 4 |
| Chưa chuyển bộ phận | 7 | 0 | 7 | 0% | — | — |

### 10.4 Bảng "Mật độ theo thôn, tổ dân phố"
Chú thích: *"Điểm đen là nơi vừa nhiều phản ánh vừa xử lý không kịp."*

| THÔN, TỔ DÂN PHỐ | SỐ PHẢN ÁNH | ĐANG TRỄ HẠN |
|---|---|---|
| Chưa xác định địa bàn | 13 | 13 |
| Thôn Hà Lam | 3 | 3 |
| … | | |

---

## 11. Modal "Nhập hộ phản ánh"

Tiêu đề: `Nhập hộ phản ánh của người dân`
Mô tả: *"Dùng khi người dân gọi điện, ghé trụ sở, hoặc gặp trưởng thôn ngoài địa bàn. Phiếu nhập ở đây đi cùng quy trình và cùng thời hạn với phiếu gửi từ Zalo."*

| Trường | Kiểu | Bắt buộc | Ghi chú |
|---|---|---|---|
| Lĩnh vực | select | ✔ | `— Chọn lĩnh vực —` + 12 lĩnh vực |
| Nội dung phản ánh | textarea | ✔ | placeholder `Ghi lại lời người dân: sự việc gì, ở đâu, từ khi nào.` |
| Địa chỉ, vị trí | text | | placeholder `Đầu ngõ thôn Hà Lam` |
| Thôn, tổ dân phố | select | | `— Chưa xác định —` + 6 thôn |
| Người gửi | text | | |
| Số điện thoại | text | | |
| ☐ Người dân đề nghị gửi ẩn danh | checkbox | | |
| Tiếp nhận qua kênh | select | | mặc định `Cán bộ, trưởng thôn nhập hộ` |
| `⬆ Đính ảnh hiện trường` | file nhiều ảnh | | |

Nút: `Huỷ` · `Vào sổ phản ánh`

---

## 12. Mô hình dữ liệu

**`phan_anh`**

| Trường | Kiểu | Ghi chú |
|---|---|---|
| `id` | uuid | |
| `ma` | text unique | `PA-2026-0021` |
| `linh_vuc` | enum | 12 mã |
| `noi_dung` | text | |
| `dia_chi` | text null | `Chưa rõ vị trí` khi null |
| `thon_id` | uuid null | |
| `lat`, `lng` | numeric null | cho heatmap |
| `nguoi_gui_ho_ten` | text null | |
| `nguoi_gui_dien_thoai` | text null | |
| `an_danh` | bool | |
| `kenh_tiep_nhan` | enum | `zalo-mini-app` \| `zalo-oa` \| `web-xa` \| `can-bo-nhap-ho` |
| `trang_thai` | enum | 9 mã |
| `bo_phan_id` | uuid null | |
| `can_bo_xu_ly_id` | uuid null | |
| `han_xu_ly` | timestamp | tính từ SLA theo lĩnh vực |
| `tiep_nhan_luc` | timestamp | |
| `xu_ly_xong_luc` | timestamp null | |
| `dong_luc` | timestamp null | |
| `hien_cong_khai` | bool | mặc định false ⇒ `CHỜ KIỂM DUYỆT` |
| `diem_hai_long` | int null | 1–5 |
| `danh_gia_luc` | timestamp null | |
| `da_mo_lai` | bool | do đánh giá thấp |

**`anh_phan_anh`**: `id, phan_anh_id, loai (truoc|sau), url, nguoi_tai_id, tao_luc`

**`nhat_ky_phan_anh`**: `id, phan_anh_id, nguoi_id, thoi_diem, trang_thai_tai_thoi_diem, bo_phan_id, can_bo_id, noi_dung, dinh_kem`

---

## 13. API đề xuất

| Method | Endpoint |
|---|---|
| GET | `/api/phan-anh?pham_vi=&q=&trang_thai=&linh_vuc=&thon=&tre_han=&danh_gia_thap=` |
| POST | `/api/phan-anh` (nhập hộ) |
| GET/PATCH | `/api/phan-anh/:ma` |
| POST | `/api/phan-anh/:ma/trang-thai` |
| POST | `/api/phan-anh/:ma/chuyen-xu-ly` |
| POST | `/api/phan-anh/:ma/anh-sau` (multipart) |
| POST | `/api/phan-anh/:ma/cong-khai` `{hien: true\|false}` |
| POST | `/api/phan-anh/:ma/danh-gia` `{sao}` |
| POST | `/api/phan-anh/:ma/nhat-ky` |
| POST | `/api/phan-anh/:ma/tao-nhiem-vu` | sinh nhiệm vụ `nguồn giao = Từ phản ánh` |
| GET | `/api/phan-anh/ban-do-nhiet` |
| GET | `/api/phan-anh/bao-cao` |
| POST | `/api/cong/phan-anh` | **endpoint công khai** cho Zalo Mini App gửi phiếu |

---

## 14. Quy tắc nghiệp vụ

1. **Hạn xử lý** tính từ bảng SLA theo **lĩnh vực** (xem `14-cau-hinh.md`). Ví dụ `An ninh trật tự`: tiếp nhận 2 giờ, xử lý xong 16 giờ; `Hạ tầng giao thông`: 8 giờ / 168 giờ. Nếu lĩnh vực không có dòng riêng thì dùng dòng `Mặc định cho mọi lĩnh vực` (8 giờ / 56 giờ). Giờ ở đây là **giờ làm việc** của xã, không phải giờ treo tường — ADR 0007. Mốc khởi động cả hai đồng hồ, và quy tắc khi cán bộ đổi lĩnh vực lúc phân loại: **ADR 0027**, không chép lại ở đây.
2. **Không đóng phiếu được khi thiếu ảnh sau xử lý.** Chặn ở cả client và server.
3. Đánh giá **1–2 sao tự mở lại phiếu** về `Đang xử lý` và ghi nhật ký tự động.
4. Phiếu mặc định **không công khai**; phải kiểm duyệt (`Cho hiện công khai`) mới lên trang công khai/Mini App. Người gửi luôn tra cứu được phiếu của mình.
5. Lĩnh vực `Thái độ / tác phong cán bộ` chỉ hiện với quyền `feedback.restricted`.
6. Quyền: `feedback.create` (tiếp nhận), `feedback.read`, `feedback.assign` (phân công), `feedback.resolve` (kết thúc xử lý).
7. Ẩn danh: ẩn tên và SĐT khỏi mọi giao diện, chỉ để `Gửi ẩn danh`.
8. Phiếu quá hạn tự leo thang theo cột `Báo lãnh đạo trực tiếp` / `Báo Chủ tịch` trong bảng SLA.

---

## 15. Dữ liệu mẫu

21 phiếu `PA-2026-0001` → `PA-2026-0021`. Phân bố lĩnh vực: Rác thải 9, Điện 3, An ninh 3, các lĩnh vực khác 1 mỗi loại. Phân bố bộ phận: VĂN PHÒNG ĐẢNG ỦY 9, chưa chuyển 7, UBMTTQ 4, Lãnh đạo UBND 1. Địa bàn: chưa xác định 13, Hà Lam 3, mỗi thôn còn lại 1.
Có 3 phiếu đã đánh giá: `PA-2026-0015` (5★), `PA-2026-0005` (5★), `PA-2026-0006` (2★ — bị đánh giá thấp).
