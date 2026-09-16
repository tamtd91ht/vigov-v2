# 08 — Thông báo

**Route:** `/thong-bao` · **Tiêu đề trang:** `Thông báo · ViGov` · **Menu:** Thông báo (nhóm Điều hành)

---

## 1. Mục đích

Mô tả trong giao diện: *"Gửi tới các bộ phận, kèm thư điện tử. Người nhận thấy ngay ở trang này."*

Kênh phát thông báo nội bộ trong xã: chọn bộ phận nhận → phát hành → người nhận thấy trên trang này, trên chuông ở header, và (nếu bật) nhận email qua máy chủ thư cấu hình ở `Cấu hình → Máy chủ thư`. Có cơ chế **bắt buộc xác nhận đã đọc** và đếm tỷ lệ xác nhận.

---

## 2. Bố cục

```
PageHeader
  "Thông báo"
  "Gửi tới các bộ phận, kèm thư điện tử. Người nhận thấy ngay ở trang này."
  Nút: [Gửi cho tôi] [Cả sổ thông báo]  ← segmented lọc
       [+ Soạn thông báo]               ← nút chính

Hai cột:
  ┌──────────────────────────────┬───────────────────────────┐
  │ Danh sách thông báo (2/3)    │ Chi tiết thông báo (1/3)  │
  │ mới nhất ở trên              │ rỗng → "Chọn một thông    │
  │                              │ báo để xem."              │
  └──────────────────────────────┴───────────────────────────┘
```

Bộ lọc:
- `Gửi cho tôi` — chỉ thông báo tôi là người nhận.
- `Cả sổ thông báo` — toàn bộ (cần quyền xem).

---

## 3. Thẻ thông báo trong danh sách

```
Thông báo về việc triển khai hệ thống an ninh  [Bắt buộc xác nhận] [✓ Đã xác nhận]
Thông báo về việc triển khai hệ thống an ninh Thông báo về việc triển khai hệ thống
an ninh Thông báo về việc triển khai hệ thống an ninh…
16:35 07/09/2026    2/12 đã xác nhận    Đang gửi thư…
```

| Thành phần | Ghi chú |
|---|---|
| Tiêu đề | đậm |
| Chip `Bắt buộc xác nhận` | cam, chỉ hiện khi bật cờ |
| Chip `✓ Đã xác nhận` | xanh lá, chỉ hiện khi **người đang đăng nhập** đã xác nhận |
| Trích nội dung | 2 dòng, cắt bớt |
| `HH:mm dd/MM/yyyy` | thời điểm phát hành |
| `{x}/{y} đã xác nhận` | chỉ hiện khi bắt buộc xác nhận |
| Trạng thái thư | `Đang gửi thư…` / `Đã gửi thư` / `Gửi thư lỗi` |
| Thẻ đã chọn | viền xanh dương |

Thông báo **ghim** hiện đầu danh sách với icon ghim.

---

## 4. Panel chi tiết (cột phải)

```
Thông báo về việc triển khai hệ thống an ninh

(toàn văn nội dung)

BỘ PHẬN NHẬN
  [THƯỜNG TRỰC ĐẢNG UỶ] [LÃNH ĐẠO ỦY BAN NHÂN DÂN XÃ]
  [THƯỜNG TRỰC UBMTTQ VIỆT NAM] [VĂN PHÒNG ĐẢNG ỦY]
  [THƯỜNG TRỰC HỘI ĐỒNG NHÂN DÂN]
  [🗑 Gỡ]

NGƯỜI NHẬN (12)
  Huỳnh Văn 10      chưa mở
  Huỳnh Văn 5       chưa mở
  Huỳnh Văn 3       chưa mở
  Huỳnh Văn 4       chưa mở
  Huỳnh Văn 12          ✓     ← đã xác nhận (icon check xanh)
  Huỳnh Văn 9       chưa mở
  …
```

Trạng thái mỗi người nhận: `chưa mở` · `đã mở` · `✓ đã xác nhận` (kèm thời điểm khi hover).
Nút `🗑 Gỡ` thu hồi thông báo.

---

## 5. Modal "Soạn thông báo"

Tiêu đề: `Soạn thông báo`
Mô tả: *"Thông báo gửi tới từng cán bộ của các bộ phận đã chọn, kèm thư điện tử nếu bật."*

| Trường | Kiểu | Bắt buộc | Ghi chú |
|---|---|---|---|
| Tiêu đề | text | ✔ | placeholder `Mời họp giao ban tháng 9` |
| Nội dung | textarea | ✔ | |
| Bộ phận nhận thông báo | chip nhiều lựa chọn (toggle), liệt kê 5 bộ phận viết HOA | | chọn bộ phận ⇒ gửi cho **mọi cán bộ** thuộc bộ phận đó |
| Gửi thêm đích danh (ngoài các bộ phận đã chọn) | list-box nhiều lựa chọn, mỗi dòng `Họ tên — email` | | |
| ☐ Ghim lên đầu danh sách | checkbox | | |
| ☐ Bắt buộc xác nhận đã đọc | checkbox | | bật ⇒ hiện chip + bộ đếm `x/y` |
| ☑ Gửi thư điện tử cho người nhận | checkbox, **mặc định bật** | | dùng SMTP ở Cấu hình → Máy chủ thư |

Nút: `Huỷ` · `Lưu nháp` · `➤ Phát hành`

---

## 6. Mô hình dữ liệu

**`thong_bao`**

| Trường | Kiểu | Ghi chú |
|---|---|---|
| `id` | uuid | |
| `tieu_de` | text | |
| `noi_dung` | text | |
| `trang_thai` | enum | `nhap` \| `da_phat_hanh` \| `da_go` |
| `ghim` | bool | |
| `bat_buoc_xac_nhan` | bool | |
| `gui_thu_dien_tu` | bool | |
| `trang_thai_thu` | enum | `chua_gui` \| `dang_gui` \| `da_gui` \| `loi` |
| `nguoi_soan_id` | uuid | |
| `phat_hanh_luc` | timestamp null | |
| `tao_luc` | timestamp | |

**`thong_bao_bo_phan`**: `thong_bao_id, bo_phan_id` (PK kép)

**`thong_bao_nguoi_nhan`**

| Trường | Kiểu | Ghi chú |
|---|---|---|
| `thong_bao_id`, `nguoi_dung_id` | PK kép | |
| `dich_danh` | bool | true = thêm đích danh, false = sinh ra từ bộ phận |
| `da_mo_luc` | timestamp null | `chưa mở` khi null |
| `da_xac_nhan_luc` | timestamp null | |
| `thu_gui_luc` | timestamp null | |
| `thu_loi` | text null | |

Bộ đếm hiển thị: `x = count(da_xac_nhan_luc IS NOT NULL)`, `y = count(*)`.

---

## 7. API đề xuất

| Method | Endpoint | Mô tả |
|---|---|---|
| GET | `/api/thong-bao?pham_vi=gui-cho-toi\|ca-so` | |
| POST | `/api/thong-bao` | tạo nháp |
| GET | `/api/thong-bao/:id` | kèm danh sách người nhận + trạng thái |
| POST | `/api/thong-bao/:id/phat-hanh` | sinh `thong_bao_nguoi_nhan`, đẩy job gửi mail |
| POST | `/api/thong-bao/:id/go` | thu hồi |
| POST | `/api/thong-bao/:id/xac-nhan` | người nhận xác nhận đã đọc |
| POST | `/api/thong-bao/:id/da-mo` | đánh dấu đã mở (gọi khi mở chi tiết) |
| GET | `/api/thong-bao/chua-doc` | cho chuông ở header |

---

## 8. Quan hệ với chuông thông báo ở header

Chuông header (`aria-label: Thông báo — N thông báo chưa đọc`) là **hộp thư hợp nhất**, không chỉ module này. Nội dung panel chuông gồm:

| Loại mục | Ví dụ |
|---|---|
| Thông báo phát hành | `Thông báo về việc triển khai hệ thống an ninh` |
| Đề nghị gia hạn | `Đề nghị gia hạn: Rà soát công tác vệ sinh` / dòng phụ `Do vướng mắc giải phóng mặt bằng` |
| Được giao nhiệm vụ | `Bạn được giao nhiệm vụ: Kiểm tra vòng đời rút gọn` |
| Nhắc sắp/quá hạn | sinh bởi job `Nhắc việc sắp đến hạn và đã quá hạn` |

Mỗi mục: tiêu đề + dòng phụ + `HH:mm dd/MM/yyyy`. Header panel có nút `Đọc hết`.

⇒ Nên có bảng riêng **`hop_thu_thong_bao`**: `id, nguoi_dung_id, loai, tieu_de, mo_ta, link, doc_luc, tao_luc` — mọi module đẩy vào đây.

---

## 9. Quy tắc nghiệp vụ

1. Quyền soạn & phát hành: `announcement.create`.
2. Phát hành xong **không sửa nội dung** — muốn sửa thì gỡ và soạn lại.
3. Chọn bộ phận sinh danh sách người nhận **tại thời điểm phát hành** (cán bộ thêm sau không tự nhận).
4. Gửi thư chạy nền, có retry; trạng thái phản ánh trên thẻ.
5. Thông báo `Bắt buộc xác nhận` hiện lại trên chuông của người chưa xác nhận cho tới khi xác nhận.
6. Nếu chưa cấu hình máy chủ thư, checkbox gửi thư vẫn bật được nhưng cảnh báo *"Chưa khai thì hệ thống dùng máy chủ thư của nền tảng nếu có"*.

---

## 10. Dữ liệu mẫu

| Tiêu đề | Cờ | Xác nhận | Thời điểm |
|---|---|---|---|
| Thông báo về việc triển khai hệ thống an ninh | Bắt buộc xác nhận | 2/12 | 16:35 07/09/2026 |
| Thông báo khẩn về việc phổ biến ATTT | Bắt buộc xác nhận | 0/7 | 16:34 07/09/2026 |
| Thông báo họp giao ban tuần | — | 0/8 | 13:33 07/09/2026 |
