# 11 — Quản trị nội dung Mini App

**Route:** `/mini-app` · **Tiêu đề trang:** `Quản trị nội dung Mini App` · **Menu:** Nội dung Mini App (nhóm Quản trị)

---

## 1. Mục đích

Mô tả trong giao diện: *"Tin tức, sự kiện, thông báo, bản tin truyền thanh và video hiển thị cho bà con trên Zalo Mini App."*

Đây là **CMS cho ứng dụng Zalo Mini App của xã**. Nội dung đến từ hai nguồn:
1. **Đồng bộ tự động** từ Cổng thông tin điện tử của xã (crawl qua API của cổng DNN/DotNetNuke).
2. **Soạn thủ công** trong ViGov.

---

## 2. Bố cục

```
PageHeader
  "Quản trị nội dung Mini App"
  "Tin tức, sự kiện, thông báo, bản tin truyền thanh và video hiển thị cho bà con
   trên Zalo Mini App."
  Nút: [+ Thêm nội dung]

Card 1 — Đồng bộ tin từ Cổng thông tin điện tử
Card 2 — Danh bạ chính quyền  (link sang /danh-ba)

Tabs loại nội dung: [Tin tức][Sự kiện][Thông báo][Truyền thanh][Video][Banner]
  [🔍 Tìm theo tiêu đề…]  [Tất cả danh mục ▾]        [⊞ Danh mục tin]
  Bảng nội dung
```

---

## 3. Card "Đồng bộ tin từ Cổng thông tin điện tử"

```
🔗 Đồng bộ tin từ Cổng thông tin điện tử          [⟳ Đồng bộ ngay] [Cấu hình]
[Đang bật] · 60 chuyên mục · Mỗi 6 giờ · đăng thẳng
Chạy lần cuối 08:09 14/09/2026  [⏱ 1 tin mới]  bỏ qua 3563
{khối log lỗi màu đỏ, liệt kê từng chuyên mục kèm mã lỗi}
```

| Thành phần | Ghi chú |
|---|---|
| Chip trạng thái | `Đang bật` (xanh) / `Đang tắt` (xám) |
| Meta | `{n} chuyên mục · Mỗi {k} giờ · {chế độ đăng}` — chế độ: `đăng thẳng` hoặc `chờ duyệt` |
| Dòng chạy | `Chạy lần cuối {HH:mm dd/MM/yyyy}` + badge `{n} tin mới` + `bỏ qua {n}` |
| Khối log | Danh sách lỗi theo chuyên mục, dạng `{Tên chuyên mục}: {mã lỗi}` — ví dụ `Các dự án: RemoteProtocolError`, `Hoạt động của các doanh nghiệp: ConnectError`. Màu đỏ, cuộn được. |
| `⟳ Đồng bộ ngay` | chạy ngay một lượt |
| `Cấu hình` | mở modal cấu hình đồng bộ |

### Modal cấu hình đồng bộ

Tiêu đề: `Đồng bộ tin từ Cổng thông tin điện tử`
Mô tả: *"Tin đã đăng trên Cổng thông tin của xã sẽ tự về sổ tin này, khỏi phải gõ lại."*

| Trường | Kiểu | Ghi chú |
|---|---|---|
| Địa chỉ API của Cổng | url | ✔ — ví dụ `https://thangbinh.danang.gov.vn/DesktopModules/cttdt/api/apichiase` |
| Mã bảo mật | password | placeholder `Giữ nguyên mã cũ`; dòng phụ `Đang dùng ****654bf. Để trống nếu không đổi.` |
| Chuyên mục lấy về | danh sách checkbox nhóm theo **cây chuyên mục** của cổng, kèm badge `bỏ cả mục`. Có nút `Chọn tất cả` / `Bỏ chọn`. Header hiển thị `đã chọn 60/60` | |
| | Mỗi chuyên mục có select **ánh xạ loại nội dung**: `Tin tức` / `Sự kiện` / `Thông báo` / `Truyền thanh` / `Video` / `Banner` | |
| Nhịp đồng bộ | select giờ | `Mỗi 6 giờ` |
| Chế độ đăng | select | `đăng thẳng` / `chờ duyệt` |

Nút: `Huỷ` · `Lưu cấu hình`

Danh sách chuyên mục mẫu (60 mục, cây 2 cấp) gồm: `Ban chấp hành Đảng bộ xã Thăng Bình khóa I…`, `Báo chí viết về Thăng Bình`, `Bảo vệ nền tảng tư tưởng của Đảng`, `Bầu cử Đại biểu Quốc hội và HĐND các cấp`, `Đại hội Đảng bộ các cấp`, `Danh mục › Chuyển đổi số`, `Danh mục › Chiến lược quy hoạch, kế hoạch`, `Danh mục › Công khai ngân sách`, `Hoạt động của các cấp ủy › Tin hoạt động của Đảng ủy`, `Kinh tế › Nông thôn mới`, `Lịch sử Đảng › …`, `Nội chính › Công an`, `Thông tin - Tư liệu › …`, v.v.

---

## 4. Card "Danh bạ chính quyền"

```
📖 Danh bạ chính quyền                                        Mở →
Đang hiện 26 cán bộ cho bà con. Chọn thêm hoặc bớt ở màn Danh bạ cán bộ.
```
Link tới `/danh-ba`.

---

## 5. Sáu loại nội dung (tabs)

| Tab | Mã | Ghi chú |
|---|---|---|
| Tin tức | `tin-tuc` | bài viết thường |
| Sự kiện | `su-kien` | có ngày diễn ra |
| Thông báo | `thong-bao` | thông báo cho dân (khác module Thông báo nội bộ) |
| Truyền thanh | `truyen-thanh` | bản tin audio của loa xã |
| Video | `video` | |
| Banner | `banner` | ảnh quảng bá trên đầu Mini App |

---

## 6. Bảng nội dung

| Cột | Nội dung |
|---|---|
| Tiêu đề | tiêu đề (đậm) + dòng phụ tóm tắt cắt 1 dòng |
| Chuyên mục | tên chuyên mục nguồn |
| Tệp đính kèm | `🔗 Có ảnh` / `—` |
| Ngày đăng | `d/M/yyyy` |
| Lượt xem | `👁 0` |
| Trạng thái | chip `Đang hiện` (xanh) / `Chờ duyệt` (cam) / `Ẩn` (xám) |
| *(hành động)* | `✎` sửa |

Bộ lọc: ô `Tìm theo tiêu đề…` + select `Tất cả danh mục`.
Nút `⊞ Danh mục tin` — quản lý danh mục nội bộ của Mini App.

---

## 7. Modal "Thêm nội dung"

Tiêu đề: `Thêm nội dung cho Mini App`
Mô tả: *"Chưa bật "Đăng lên Mini App" thì bà con chưa thấy — soạn trước, đăng sau được."*

| Trường | Kiểu | Bắt buộc | Ghi chú |
|---|---|---|---|
| Loại nội dung | select 6 loại | | mặc định `Tin tức` |
| Danh mục | select | | `— Chưa xếp danh mục —` |
| Tiêu đề | text | ✔ | |
| Tóm tắt | textarea | | |
| Nội dung | textarea (rich text khi dựng lại) | | |
| Ảnh đại diện | file | | `Chọn tệp từ máy` · `JPG, PNG hoặc WebP — tối đa 50MB` |
| ☐ Đăng lên Mini App | checkbox | | |

Nút: `Huỷ` · `Lưu`

> Với loại `Video` nên bổ sung trường URL video; loại `Truyền thanh` bổ sung file audio + thời lượng; loại `Sự kiện` bổ sung thời gian & địa điểm; loại `Banner` bổ sung link đích và thứ tự hiển thị.

---

## 8. Mô hình dữ liệu

**`noi_dung_mini_app`**

| Trường | Kiểu | Ghi chú |
|---|---|---|
| `id` | uuid | |
| `loai` | enum | 6 mã mục 5 |
| `danh_muc_id` | uuid null | |
| `tieu_de` | text | |
| `tom_tat` | text null | |
| `noi_dung` | text null | HTML |
| `anh_dai_dien_url` | text null | |
| `tep_dinh_kem` | jsonb | |
| `ngay_dang` | date | |
| `luot_xem` | int default 0 | |
| `trang_thai` | enum | `dang-hien` \| `cho-duyet` \| `an` |
| `nguon` | enum | `thu-cong` \| `dong-bo-cong` |
| `nguon_url` | text null | link bài gốc trên cổng |
| `nguon_id_ngoai` | text null | id bài trên cổng — dùng khử trùng |
| `nguoi_tao_id`, `tao_luc`, `cap_nhat_luc` | | |

**`danh_muc_mini_app`**: `id, ten, slug, cha_id, thu_tu`

**`cau_hinh_dong_bo_cong`**

| Trường | Kiểu |
|---|---|
| `id` | uuid |
| `bat` | bool |
| `api_url` | text |
| `ma_bao_mat` | text (mã hoá) |
| `nhip_gio` | int (6) |
| `che_do_dang` | enum `dang-thang` \| `cho-duyet` |
| `chay_lan_cuoi` | timestamp |
| `so_tin_moi_lan_cuoi` | int |
| `so_bo_qua` | int |
| `log_loi` | jsonb |

**`chuyen_muc_cong`**: `id, ma_chuyen_muc, ten, cha, duoc_chon bool, anh_xa_loai (6 loại)`

---

## 9. API đề xuất

| Method | Endpoint |
|---|---|
| GET | `/api/mini-app/noi-dung?loai=&q=&danh_muc=` |
| POST | `/api/mini-app/noi-dung` |
| GET/PATCH/DELETE | `/api/mini-app/noi-dung/:id` |
| GET/POST | `/api/mini-app/danh-muc` |
| GET/PUT | `/api/mini-app/dong-bo/cau-hinh` |
| POST | `/api/mini-app/dong-bo/chay-ngay` |
| GET | `/api/mini-app/dong-bo/chuyen-muc` — nạp cây chuyên mục từ cổng |
| GET | `/api/cong/mini-app/noi-dung?loai=` — **endpoint công khai** cho Mini App đọc |
| GET | `/api/cong/mini-app/danh-ba` — **endpoint công khai** cho danh bạ |

---

## 10. Quy tắc nghiệp vụ

1. Đồng bộ **khử trùng theo `nguon_id_ngoai`**; bài đã có thì bỏ qua (đếm vào `bỏ qua {n}`).
2. Chế độ `đăng thẳng` ⇒ bài đồng bộ về có `trang_thai = dang-hien`; chế độ `chờ duyệt` ⇒ `cho-duyet`.
3. Lỗi từng chuyên mục **không làm hỏng cả lượt đồng bộ** — ghi vào `log_loi` và hiển thị trên card (đúng như prototype: hàng loạt `ConnectError` nhưng job vẫn chạy).
4. Bài do cán bộ sửa tay thì lần đồng bộ sau **không ghi đè** (đặt cờ `da_sua_tay`).
5. Quyền: `content.read` (xem nội dung và danh bạ Mini App), `content.update` (sửa).
6. Ảnh tối đa 50MB, chấp nhận JPG/PNG/WebP.
7. Danh bạ hiển thị trên Mini App được điều khiển ở `/danh-ba` (cờ `hien_tren_mini_app`), không phải ở đây.
