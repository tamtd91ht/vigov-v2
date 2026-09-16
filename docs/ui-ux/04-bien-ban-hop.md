# 04 — Biên bản và kết luận họp

**Route:** `/nhiem-vu/bien-ban` · **Tiêu đề trang:** `Biên bản và kết luận họp · ViGov` · **Menu:** Biên bản họp (nhóm Điều hành)

---

## 1. Mục đích

Mô tả trong giao diện: *"Nhập một biên bản, tách thành nhiều nhiệm vụ. Mỗi nhiệm vụ giữ liên kết ngược về kết luận gốc để truy vết được về sau."*

Đây là **cầu nối giữa cuộc họp và việc làm**: văn phòng gõ biên bản họp giao ban → tách từng kết luận thành nhiệm vụ → nhiệm vụ đó vĩnh viễn mang liên kết `nguồn giao = Từ kết luận họp`.

---

## 2. Bố cục

```
PageHeader
  "Biên bản và kết luận họp"
  "Nhập một biên bản, tách thành nhiều nhiệm vụ. Mỗi nhiệm vụ giữ liên kết ngược về kết luận gốc để truy vết được về sau."
  Nút: [+ Nhập biên bản]  (nền tối)

Danh sách các biên bản, mỗi biên bản là một card lớn xếp dọc,
mới nhất ở trên.
```

### Card biên bản

```
┌──────────────────────────────────────────────────────────────────────┐
│ 📋  Giao ban Uỷ ban nhân dân xã tháng 8 năm 2026                      │
│     5/8/2026 · 31/BB-UBND · Phòng họp UBND xã     [3 kết luận · 1/3  │
│                                                    nhiệm vụ xong]    │
├──────────────────────────────────────────────────────────────────────┤
│ ① Giao bộ phận Địa chính rà soát tiến độ tuyến đường Hà Lam –        │
│    Bình Trị, báo cáo trước ngày 20/8.        [✂ Tách thành nhiệm vụ] │
│    0/1 nhiệm vụ đã hoàn thành                                        │
├──────────────────────────────────────────────────────────────────────┤
│ ② Giao Văn hoá – Xã hội hoàn tất hồ sơ hỗ trợ sinh kế đợt 3 cho      │
│    các hộ đã được phê duyệt.                 [✂ Tách thành nhiệm vụ] │
│    0/1 nhiệm vụ đã hoàn thành                                        │
├──────────────────────────────────────────────────────────────────────┤
│ ③ Giao Tài chính – Kế toán đối chiếu số liệu giải ngân sáu tháng     │
│    đầu năm với chủ đầu tư.                   [✂ Tách thành nhiệm vụ] │
│    1/1 nhiệm vụ đã hoàn thành                                        │
├──────────────────────────────────────────────────────────────────────┤
│ [Nhập một kết luận của cuộc họp…                    ] [+ Thêm kết luận]│
└──────────────────────────────────────────────────────────────────────┘
```

**Header card**
- Icon tài liệu trong ô vuông bo góc, nền xám nhạt.
- Tiêu đề biên bản (đậm).
- Dòng meta: `{ngày họp d/M/yyyy} · {số hiệu biên bản} · {địa điểm}` — phần nào thiếu thì bỏ, phân tách bằng ` · `.
- Badge phải: `{n} kết luận · {x}/{y} nhiệm vụ xong`.

**Dòng kết luận**
- Số thứ tự trong ô tròn màu xanh nhạt.
- Nội dung kết luận (text nhiều dòng).
- Dòng phụ xám: `{x}/{y} nhiệm vụ đã hoàn thành`, hoặc `Chưa tách thành nhiệm vụ nào`.
- Nút phải: `✂ Tách thành nhiệm vụ`.

**Hàng thêm kết luận** (luôn ở cuối mỗi card)
- Textarea 1 dòng, placeholder `Nhập một kết luận của cuộc họp…`
- Nút `+ Thêm kết luận`.

---

## 3. Luồng "Tách thành nhiệm vụ"

Bấm `✂ Tách thành nhiệm vụ` → mở modal **Giao việc mới** (dùng lại nguyên form ở `02-nhiem-vu.md`) với dữ liệu điền sẵn:

| Trường | Giá trị điền sẵn |
|---|---|
| Nội dung nhiệm vụ / Tên nhiệm vụ | nội dung kết luận |
| Nguồn giao | `Từ kết luận họp` (khoá, không sửa) |
| `nguon_id` | id của kết luận |
| Hạn hoàn thành | gợi ý từ ngày nêu trong kết luận nếu nhận diện được (`báo cáo trước ngày 20/8` → 20/8) |

Một kết luận có thể tách thành **nhiều** nhiệm vụ (đếm `y` trong `x/y`).
Sau khi tách, dòng kết luận cập nhật bộ đếm, và có thể mở rộng để xem danh sách nhiệm vụ đã sinh ra.

---

## 4. Modal "Nhập biên bản"

| Trường | Kiểu | Bắt buộc | Ghi chú |
|---|---|---|---|
| Tên cuộc họp | text | ✔ | `Giao ban Uỷ ban nhân dân xã tháng 8 năm 2026` |
| Ngày họp | date | ✔ | |
| Số hiệu biên bản | text | | `31/BB-UBND` |
| Địa điểm | text | | `Phòng họp UBND xã` |
| Chủ trì | combobox cán bộ | | |
| Thành phần tham dự | multi-select cán bộ / text | | |
| Nội dung biên bản | textarea lớn | | toàn văn |
| Các kết luận | danh sách động textarea + `+ Thêm kết luận` | | có thể thêm sau |
| Tệp đính kèm | file | | bản scan biên bản |

Nút: `Huỷ` · `Lưu biên bản`.

> Prototype có một biên bản nhập nháp tên `giao ban` với meta `6/9/2026 · 12/233`, cho thấy các trường phụ đều tuỳ chọn.

---

## 5. Mô hình dữ liệu

**`bien_ban_hop`**

| Trường | Kiểu | Ghi chú |
|---|---|---|
| `id` | uuid | |
| `ten_cuoc_hop` | text | |
| `ngay_hop` | date | |
| `so_hieu` | text null | `31/BB-UBND` |
| `dia_diem` | text null | |
| `chu_tri_id` | uuid null | |
| `thanh_phan` | jsonb null | danh sách người dự |
| `noi_dung` | text null | |
| `dinh_kem` | jsonb | |
| `nguoi_tao_id`, `tao_luc` | | |

**`ket_luan_hop`**

| Trường | Kiểu | Ghi chú |
|---|---|---|
| `id` | uuid | |
| `bien_ban_id` | uuid | |
| `thu_tu` | int | số hiển thị trong ô tròn |
| `noi_dung` | text | |
| `tao_luc` | timestamp | |

**Liên kết ngược:** `nhiem_vu.nguon_giao = 'ket-luan-hop'` và `nhiem_vu.nguon_id = ket_luan_hop.id`.

Bộ đếm hiển thị:
```sql
-- cho một kết luận
SELECT count(*) FILTER (WHERE trang_thai='hoan-thanh') AS x, count(*) AS y
FROM nhiem_vu WHERE nguon_giao='ket-luan-hop' AND nguon_id = :ket_luan_id;

-- cho cả biên bản: cộng dồn mọi kết luận của biên bản đó
```

---

## 6. API đề xuất

| Method | Endpoint | Mô tả |
|---|---|---|
| GET | `/api/bien-ban` | danh sách biên bản + kết luận + bộ đếm nhiệm vụ |
| POST | `/api/bien-ban` | tạo biên bản (kèm mảng kết luận) |
| PATCH/DELETE | `/api/bien-ban/:id` | |
| POST | `/api/bien-ban/:id/ket-luan` | thêm một kết luận |
| PATCH/DELETE | `/api/ket-luan/:id` | |
| POST | `/api/ket-luan/:id/tach-nhiem-vu` | body = payload tạo nhiệm vụ; trả nhiệm vụ mới |
| GET | `/api/ket-luan/:id/nhiem-vu` | các nhiệm vụ đã tách ra |

---

## 7. Quy tắc nghiệp vụ

1. Xoá kết luận đã tách nhiệm vụ: **không xoá cứng**, cảnh báo và giữ liên kết (nhiệm vụ vẫn phải truy vết được).
2. Thứ tự kết luận đánh số liên tục từ 1 trong phạm vi một biên bản; thêm mới thì nối tiếp.
3. Biên bản không có kết luận nào vẫn lưu được (nhập nháp trước, bổ sung sau).
4. Nhiệm vụ tách ra hiển thị ở `/nhiem-vu` với bộ lọc `Nguồn giao = Từ kết luận họp`, và trong drawer chi tiết phải hiện link quay về biên bản gốc.
5. Bộ đếm `x/y nhiệm vụ xong` ở header card = tổng trên tất cả kết luận của biên bản.

---

## 8. Dữ liệu mẫu để seed

Biên bản `Giao ban Uỷ ban nhân dân xã tháng 8 năm 2026`, 5/8/2026, số `31/BB-UBND`, Phòng họp UBND xã, 3 kết luận:
1. Giao bộ phận Địa chính rà soát tiến độ tuyến đường Hà Lam – Bình Trị, báo cáo trước ngày 20/8. → NV10 (0/1 xong)
2. Giao Văn hoá – Xã hội hoàn tất hồ sơ hỗ trợ sinh kế đợt 3 cho các hộ đã được phê duyệt. → NV06 (0/1 xong)
3. Giao Tài chính – Kế toán đối chiếu số liệu giải ngân sáu tháng đầu năm với chủ đầu tư. → NV02 (1/1 xong)
