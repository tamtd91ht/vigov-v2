# 12 — Danh bạ cán bộ

**Route:** `/danh-ba` · **Tiêu đề trang:** `Danh bạ cán bộ` · **Menu:** Danh bạ cán bộ (nhóm Quản trị)

---

## 1. Mục đích

Mô tả trong giao diện: *"Toàn bộ cán bộ của xã. Chọn người cần công khai rồi bấm 'Thêm vào danh bạ Mini App' để bà con gọi được."*

Danh bạ **hai mặt**:
- Danh sách nội bộ toàn bộ cán bộ, chức vụ, khối/đơn vị, số di động.
- Cờ **hiển thị trên Zalo Mini App** để người dân tra cứu và gọi.

> Phân biệt với `Cấu hình → Người dùng`: tab đó quản lý **tài khoản đăng nhập** (email, vai trò, quyền, trạng thái hoạt động). Trang này quản lý **thông tin liên hệ công khai**. Hai bảng nên dùng chung bản ghi `nguoi_dung`, khác nhau ở bộ trường hiển thị.

---

## 2. Bố cục

```
PageHeader
  "Danh bạ cán bộ"
  "Toàn bộ cán bộ của xã. Chọn người cần công khai rồi bấm 'Thêm vào danh bạ Mini App'
   để bà con gọi được."
  Nút: [⬆ Nhập từ Excel]  [+ Thêm cán bộ]

3 thẻ KPI:
  TỔNG SỐ CÁN BỘ 26 | ĐANG HIỆN TRÊN MINI APP 26 (xanh lá) | SỐ KHỐI / ĐƠN VỊ 5

Hàng lọc:
  [🔍 Tìm theo tên, chức vụ, số điện thoại…] [Tất cả khối / đơn vị ▾] [Hiện và chưa hiện ▾]

Bảng cán bộ
```

Khi chọn checkbox nhiều dòng → hiện thanh hành động hàng loạt `Thêm vào danh bạ Mini App` / `Rút khỏi danh bạ Mini App`.

---

## 3. Bộ lọc

| Bộ lọc | Giá trị |
|---|---|
| Tìm | `Tìm theo tên, chức vụ, số điện thoại…` |
| Khối / đơn vị | `Tất cả khối / đơn vị` + mỗi bộ phận kèm bộ đếm `{đang hiện}/{tổng}`: `LÃNH ĐẠO ỦY BAN NHÂN DÂN XÃ (3/3)`, `THƯỜNG TRỰC ĐẢNG UỶ (3/3)`, `THƯỜNG TRỰC HỘI ĐỒNG NHÂN DÂN (4/4)`, `THƯỜNG TRỰC UBMTTQ VIỆT NAM (6/6)`, `VĂN PHÒNG ĐẢNG ỦY (10/10)` |
| Trạng thái hiển thị | `Hiện và chưa hiện` (mặc định) · `Đang hiện trên Mini App` (value `1`) · `Chưa hiện` (value `0`) |

---

## 4. Bảng cán bộ

| Cột | Nội dung |
|---|---|
| ☐ | checkbox, aria-label `Chọn {họ tên}` |
| Họ và tên | tên (đậm, là button mở chi tiết) + dòng phụ email |
| Chức vụ | `Bí thư Đảng ủy`, `Phó Bí thư Thường trực, Chủ tịch HĐND`… |
| Khối / đơn vị | tên bộ phận viết HOA, màu xám |
| Di động | số điện thoại + dòng phụ `Có Zalo` khi có |
| Trên Mini App | chip `✓ Đang hiện` (xanh lá) hoặc `Chưa hiện` (xám) |
| *(hành động)* | `✕` aria-label `Rút khỏi danh bạ Mini App` · `✎` `Sửa thông tin cán bộ` · `🗑` `Xoá khỏi danh bạ` |

Với người chưa hiện, nút đầu tiên đổi thành `Thêm vào danh bạ Mini App`.

---

## 5. Form "Thêm cán bộ" / "Sửa thông tin cán bộ"

| Trường | Kiểu | Bắt buộc |
|---|---|---|
| Họ và tên | text | ✔ |
| Chức vụ | text | ✔ |
| Khối / đơn vị | select bộ phận | ✔ |
| Email | email | |
| Di động | text | |
| ☐ Có Zalo | checkbox | |
| Ảnh đại diện | file | |
| Thứ tự hiển thị | number | |
| ☐ Hiện trên danh bạ Mini App | checkbox | |

---

## 6. Nhập từ Excel

Mẫu cột: `Họ và tên | Chức vụ | Khối/đơn vị | Email | Di động | Có Zalo | Hiện trên Mini App | Thứ tự`.
Khớp theo **email** (nếu có) hoặc **họ tên + khối** để cập nhật thay vì tạo trùng.

---

## 7. Mô hình dữ liệu

Dùng chung bảng **`nguoi_dung`** (xem `00-tong-quan-he-thong.md`), bổ sung các trường:

| Trường | Kiểu | Ghi chú |
|---|---|---|
| `chuc_vu` | text | khác `chuc_danh` dùng ở tài khoản? → **nên dùng chung một trường `chuc_vu`** |
| `di_dong` | text null | |
| `co_zalo` | bool | |
| `anh_dai_dien_url` | text null | |
| `hien_tren_mini_app` | bool | |
| `thu_tu_danh_ba` | int | |

> **Lưu ý nhất quán dữ liệu:** trong prototype, `/danh-ba` có 26 cán bộ với **tên thật và email công vụ thật** (đã thay bằng dữ liệu giả trong tệp này — luật 3, cấm #5), còn `Cấu hình → Người dùng` có 12 tài khoản demo (`Huỳnh Văn 1`… `demo1@thangbinh.demo.vigov.vn`). Đây là hai tập dữ liệu seed khác nhau. **Khi dựng lại phải hợp nhất về một bảng `nguoi_dung`**, trong đó `tai_khoan_hoat_dong = true` cho những người có thể đăng nhập.

---

## 8. API đề xuất

| Method | Endpoint |
|---|---|
| GET | `/api/danh-ba?q=&bo_phan=&hien_mini_app=` |
| POST | `/api/danh-ba` |
| GET/PATCH/DELETE | `/api/danh-ba/:id` |
| POST | `/api/danh-ba/hien-mini-app` | `{ids[], hien: true\|false}` — hàng loạt |
| POST | `/api/danh-ba/nhap-excel` |
| GET | `/api/danh-ba/mau-excel` |
| GET | `/api/cong/mini-app/danh-ba` | **công khai** — chỉ trả người có `hien_tren_mini_app = true`, sắp theo `thu_tu_danh_ba` rồi tên bộ phận |

---

## 9. Quy tắc nghiệp vụ

1. Chỉ người có `hien_tren_mini_app = true` mới xuất hiện trên Zalo Mini App.
2. Số điện thoại hiển thị công khai — nên có cảnh báo khi bật công khai cho một cán bộ mới.
3. Xoá khỏi danh bạ **không xoá tài khoản đăng nhập**; nếu người đó có tài khoản, chỉ gỡ thông tin liên hệ công khai.
4. Quyền: `content.read` / `content.update` (cùng nhóm với nội dung Mini App).
5. Card `Danh bạ chính quyền` ở `/mini-app` hiển thị bộ đếm `Đang hiện {n} cán bộ cho bà con` lấy từ đây.

---

## 10. Dữ liệu mẫu

26 cán bộ, 26 đang hiện, 5 khối. Trích:

> **Dữ liệu đã thay bằng dữ liệu giả — luật 3, cấm #5.**
> Bản gốc mang **họ tên thật và số di động cá nhân thật** của lãnh đạo xã, những người
> xác định được đích danh qua chức vụ. Chức vụ và tên khối là thông tin công khai nên giữ
> nguyên; tên người và số máy thì không. Số giả dùng dải đã thống nhất `0900000xxx`
> (luật 3, bất biến 5), khớp với `14-cau-hinh.md`.

| Họ và tên | Chức vụ | Khối / đơn vị | Di động |
|---|---|---|---|
| Nguyễn Văn A | Bí thư Đảng ủy | THƯỜNG TRỰC ĐẢNG UỶ | 0900000001 (Có Zalo) |
| Trần Thị B | Phó Bí thư Thường trực, Chủ tịch HĐND | THƯỜNG TRỰC ĐẢNG UỶ | 0900000002 |
| Lê Văn C | Phó Bí thư, Chủ tịch UBND xã | THƯỜNG TRỰC ĐẢNG UỶ | 0900000003 |
| Phạm Văn D | Phó Chủ tịch HĐND | THƯỜNG TRỰC HỘI ĐỒNG NHÂN DÂN | 0900000004 |
| Hoàng Thị E | Trưởng Ban Văn hoá – Xã hội | THƯỜNG TRỰC HỘI ĐỒNG NHÂN DÂN | 0900000005 |
| Vũ Thị G | Trưởng Ban Kinh tế – Ngân sách | THƯỜNG TRỰC HỘI ĐỒNG NHÂN DÂN | 0900000006 |
| Đặng Văn H | PCT. UBND xã | LÃNH ĐẠO ỦY BAN NHÂN DÂN XÃ | 0900000007 |
| Bùi Văn K | … | … | … |
