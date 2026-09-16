# 03 — Sổ tay lãnh đạo

**Route:** `/nhiem-vu/so-tay` · **Tiêu đề trang:** `Sổ tay lãnh đạo · ViGov` · **Menu:** Sổ tay lãnh đạo (nhóm Điều hành)

---

## 1. Mục đích

Mô tả trong giao diện: *"Ba việc cần biết ngay: việc trễ, việc chờ duyệt và việc mình đã giao."*

Đây là **view cá nhân hoá** của module Nhiệm vụ dành riêng cho lãnh đạo. Không có dữ liệu riêng — chỉ là ba truy vấn được trình bày cạnh nhau để lãnh đạo mở đầu ngày làm việc.

---

## 2. Bố cục

```
PageHeader
  "Sổ tay lãnh đạo"
  "Ba việc cần biết ngay: việc trễ, việc chờ duyệt và việc mình đã giao."
  (không có nút hành động)

Lưới 3 cột bằng nhau, mỗi cột là một card cao cố định (~70vh) và cuộn riêng:
 ┌─────────────────┬─────────────────┬─────────────────┐
 │ ⚠ Việc quá hạn  │ ⏱ Chờ tôi duyệt │ 👤 Việc tôi đã  │
 │            [18] │             [0] │      giao  [26] │
 ├─────────────────┼─────────────────┼─────────────────┤
 │ (danh sách)     │ Không có việc   │ (danh sách)     │
 │                 │ nào.            │                 │
 └─────────────────┴─────────────────┴─────────────────┘
```

Dưới 1024px: xếp dọc 1 cột.

---

## 3. Ba cột

| Cột | Icon | Badge | Điều kiện lọc |
|---|---|---|---|
| **Việc quá hạn** | ⚠ tam giác, màu đỏ | số lượng | `han_xu_ly < now()` AND `trang_thai ∉ {hoan-thanh}` — phạm vi **toàn xã** (lãnh đạo thấy hết). Bao gồm cả **nhiệm vụ con**. |
| **Chờ tôi duyệt** | ⏱ đồng hồ | số lượng | `trang_thai = cho-duyet` AND (`lanh_dao_giao_viec_id = me` OR tôi có quyền `task.approve` với bộ phận đó). Cộng thêm **đề nghị lùi hạn đang chờ tôi duyệt**. |
| **Việc tôi đã giao** | 👤 người | số lượng | `nguoi_tao_id = me` OR `lanh_dao_giao_viec_id = me`, mọi trạng thái chưa đóng |

### Mục trong danh sách

```
Xử lý tồn động khó khăn của bà con trong việc giải phóng mặt bằng
Huỳnh Văn 6 · hạn 10/9/2026 · trễ 6 ngày
```

- Dòng 1: tiêu đề nhiệm vụ (2 dòng, cắt bớt), đậm vừa.
- Dòng 2, chữ nhỏ xám: `{người thực hiện} · hạn {d/M/yyyy} · trễ {N} ngày`
  - Phần `trễ N ngày` in **đỏ**.
  - Chưa có hạn: `hạn —` (bỏ phần trễ).
  - Chưa phân công: `Chưa phân công · hạn —`.
- Bấm vào mục → mở **DetailDrawer nhiệm vụ** (dùng lại component của `/nhiem-vu`).
- Sắp xếp: cột "quá hạn" giảm dần theo số ngày trễ; cột "tôi đã giao" mới nhất trước.

### Trạng thái rỗng
`Không có việc nào.` — căn giữa, chữ xám.

---

## 4. API đề xuất

| Method | Endpoint | Mô tả |
|---|---|---|
| GET | `/api/so-tay` | Trả `{qua_han: [], cho_toi_duyet: [], toi_da_giao: []}` kèm `tong` mỗi nhóm |

Hoặc tái dùng `/api/nhiem-vu` với ba tham số lọc dựng sẵn:
- `?qua_han=1&pham_vi=toan-xa`
- `?trang_thai=cho-duyet&pham_vi=cho-toi-duyet`
- `?pham_vi=toi-da-giao`

---

## 5. Ghi chú khi dựng lại

- Trang này **không có bộ lọc, không có ô tìm kiếm, không có nút tạo mới**. Giữ nguyên sự tối giản.
- Ba cột dùng chung component `TaskMiniList`.
- Số liệu phải khớp tuyệt đối với `/nhiem-vu` khi lọc tương đương — nếu lệch, người dùng mất tin tưởng. Viết test so sánh hai nguồn.
- Đây là ứng viên tốt cho trang mặc định sau đăng nhập **với vai trò Lãnh đạo** (Chủ tịch / Phó Chủ tịch UBND), trong khi vai trò khác vào `/tong-quan`.
