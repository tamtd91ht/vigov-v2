# 15 — Phụ lục: giao diện dùng chung & xác thực

Những phần xuất hiện trên **mọi trang**, không thuộc module nào.

---

## 1. Xác thực

Prototype **không lộ trang đăng nhập**: mọi URL (kể cả `/dang-nhap`) đều chuyển hướng về `/tong-quan` với phiên đã đăng nhập sẵn (`Huỳnh Văn 1 — demo1@thangbinh.demo.vigov.vn — Chủ tịch UBND xã`).

Khi dựng lại, cần bổ sung:

| Route | Nội dung |
|---|---|
| `/dang-nhap` | Form `Thư điện tử công vụ` + `Mật khẩu` + `Ghi nhớ đăng nhập` + nút `Đăng nhập`. Nền trái là khối thương hiệu ViGov + tên xã. |
| `/quen-mat-khau` | Nhập email → gửi liên kết đặt lại qua máy chủ thư cấu hình. |
| `/dat-lai-mat-khau/:token` | |

Quy tắc:
- Trang chủ `/` chuyển hướng: vai trò **Lãnh đạo** → `/nhiem-vu/so-tay`; vai trò khác → `/tong-quan`.
- Chưa đăng nhập → `/dang-nhap?tiep-tuc={đường dẫn}`.
- Phiên hết hạn → modal `Phiên làm việc đã hết hạn` + nút đăng nhập lại, **không mất dữ liệu form đang nhập**.
- Ghi `dang_nhap_gan_nhat` vào `nguoi_dung` (hiển thị ở `Cấu hình → Người dùng`).
- Kiểm quyền hai lớp: ẩn menu/nút ở client **và** chặn ở server theo bảng quyền (`14-cau-hinh.md` mục 4).

---

## 2. Sidebar

```
┌────────────────────────────┐
│ [VG] ViGov              [⇤]│   ← nút "Thu gọn menu"
│      ĐIỀU HÀNH SỐ CẤP XÃ   │
├────────────────────────────┤
│ ĐIỀU HÀNH                  │
│  ▦ Tổng quan               │
│  ✎ Nhiệm vụ                │
│  ✐ Sổ tay lãnh đạo         │
│  ▤ Biên bản họp            │
│  ▤ Văn bản & Đơn thư       │
│  ⊟ Giải ngân               │
│  ⊞ Thu - Chi ngân sách     │
│  🔔 Thông báo               │
│  ⌸ Phản ánh người dân      │
│  🗺 Bản đồ kinh tế số       │
│                            │
│ QUẢN TRỊ                   │
│  ▯ Nội dung Mini App       │
│  ▤ Danh bạ cán bộ          │
│  ⊿ Báo cáo                 │
│  ⚙ Cấu hình                │
├────────────────────────────┤
│ Phiên bản 0.1.0            │
│ Môi trường phát triển      │
└────────────────────────────┘
```

- Nền navy rất đậm, chữ xám nhạt; mục đang chọn nền sáng hơn + chữ trắng + bo góc.
- Nhãn nhóm (`ĐIỀU HÀNH`, `QUẢN TRỊ`): 11px, viết HOA, letter-spacing rộng, màu xám mờ.
- Nút `Thu gọn menu` thu sidebar còn dải icon (~64px), tooltip hiện tên khi hover. Trạng thái thu gọn lưu ở `localStorage`.
- Chân sidebar hiển thị phiên bản + môi trường (`Môi trường phát triển` / `Môi trường thật`) — lấy từ biến môi trường.
- Ẩn mục menu mà người dùng không có quyền đọc tương ứng.

---

## 3. Header

| Vùng | Nội dung |
|---|---|
| Trái | `UỶ BAN NHÂN DÂN XÃ THĂNG BÌNH` (viết HOA, đậm, ~15px) + `Thành phố Đà Nẵng` (12px, xám) |
| Giữa | ô tìm kiếm toàn hệ thống — `type="search"`, placeholder `Tìm nhiệm vụ, văn bản, phản ánh…`, aria-label `Tìm kiếm toàn hệ thống`, có nút `✕` xoá khi có chữ |
| Phải | chuông thông báo + khối người dùng |

### 3.1 Tìm kiếm toàn hệ thống

Trong prototype, nhập từ khoá **chưa trả kết quả** (chức năng chưa hoàn thiện). Khi dựng lại, thiết kế:
- Gõ ≥ 2 ký tự → dropdown kết quả gom nhóm: `Nhiệm vụ`, `Văn bản & Đơn thư`, `Phản ánh`, `Dự án`, `Cán bộ`, `Đối tượng bản đồ`.
- Mỗi kết quả: mã + tiêu đề + dòng phụ ngữ cảnh.
- Enter → trang kết quả đầy đủ `/tim-kiem?q=`.
- Chỉ trả về hồ sơ người dùng có quyền xem.
- Nên dùng `pg_trgm` + `unaccent` để tìm không dấu.

### 3.2 Chuông thông báo

- aria-label: `Thông báo — {n} thông báo chưa đọc`; badge đỏ khi `n > 0`.
- Panel thả xuống (~420px), header `Thông báo` + nút `Đọc hết`, danh sách cuộn.
- Mỗi mục: tiêu đề (đậm) + dòng phụ mô tả + `HH:mm dd/MM/yyyy`; mục chưa đọc có chấm xanh bên trái.
- Nội dung hợp nhất từ nhiều nguồn — ví dụ thật:

| Tiêu đề | Dòng phụ | Nguồn |
|---|---|---|
| Thông báo về việc triển khai hệ thống an ninh | | module Thông báo |
| Thông báo khẩn về việc phổ biến ATTT | | module Thông báo |
| Đề nghị gia hạn: Rà soát công tác vệ sinh | Do vướng mắc giải phóng mặt bằng | Nhiệm vụ |
| Đề nghị gia hạn: Kiểm tra ô văn bản một dòng | Chậm nữa | Nhiệm vụ |
| Thông báo họp giao ban tuần | | module Thông báo |
| Bạn được giao nhiệm vụ: Kiểm tra vòng đời rút gọn | | Nhiệm vụ |
| Bạn được giao nhiệm vụ: Kiểm tra quy trình trạng thái | | Nhiệm vụ |

⇒ Bảng **`hop_thu_thong_bao`**: `id, nguoi_dung_id, loai, tieu_de, mo_ta, duong_dan, doc_luc, tao_luc`. Mọi module đẩy vào đây.

### 3.3 Khối người dùng

- Avatar tròn nền xanh dương với **2 ký tự viết tắt** (`V1` cho `Huỳnh Văn 1`) + họ tên + chức danh.
- Bấm → dropdown: họ tên (đậm), email (xám nhỏ), gạch ngang, `↪ Đăng xuất`.
- Khi dựng lại nên thêm `Hồ sơ của tôi` và `Đổi mật khẩu`.

---

## 4. Chế độ trình chiếu phòng họp

Có ở `/tong-quan` và `/ban-do` (nút `⤢ Trình chiếu`, aria-label `Chế độ trình chiếu phòng họp`).

- Ẩn sidebar + header, dùng toàn màn hình (Fullscreen API).
- Nền tối, cỡ chữ nhân ~1,6; mỗi nhóm KPI là một "slide".
- Điều hướng: `←` `→` đổi slide, `Esc` thoát.
- Ẩn mọi nút chỉnh sửa.

---

## 5. Mẫu thành phần lặp lại

### 5.1 `ScopeTabs` — phạm vi hồ sơ
Dùng ở Nhiệm vụ, Văn bản & Đơn thư, Phản ánh. Giữ nguyên tooltip:

| Nhãn | Tooltip |
|---|---|
| Toàn xã | Tất cả hồ sơ trong xã |
| Giao cho tôi | Đích danh tôi là người xử lý |
| Liên quan đến tôi | Tôi giao, tôi theo dõi, tôi đã xử lý, hoặc bộ phận tôi đang giữ |

### 5.2 `StatusStepper`
Dùng ở chi tiết Nhiệm vụ (4 bước + 2 rẽ nhánh) và chi tiết Phản ánh (7 bước + 2 rẽ nhánh).
- Ô hiện tại: nền màu đậm + nhãn thời gian đã ở trạng thái (Nhiệm vụ) hoặc chữ `đang ở đây` (Phản ánh).
- Ô kế tiếp bấm được: nhãn `chuyển sang`.
- Ô chưa tới: `—`.
- Hàng `Rẽ nhánh:` nằm dưới, nhỏ hơn.
- Dưới cùng là **một câu giải thích** trạng thái hiện tại.

### 5.3 `ActivityLog` — Nhật ký
- Ô nhập placeholder theo ngữ cảnh (`Đã làm được gì, còn vướng gì…` / `Đã làm gì, ai làm, còn vướng gì…`).
- Nút `📎 Đính kèm` và `➤ Ghi nhật ký`.
- Dòng thời gian mới nhất trên cùng: avatar + tên + thời điểm + chip trạng thái + (nếu có) `Bộ phận: …` `Phụ trách: … — email` + nội dung.
- Rỗng: `Chưa có ghi chép nào.`

### 5.4 `ExcelImportDialog`
Xuất hiện ở 8 chỗ: Nhiệm vụ, Đơn thư, Giải ngân, Thu–Chi, Bản đồ, Danh bạ, Sơ đồ tổ chức, Thôn/Tổ, Danh mục.
Cấu trúc: tiêu đề → một câu mô tả quy tắc → `⬇ Tải mẫu …` → vùng kéo thả `Chọn tệp .xlsx` → `Đóng` / `Nhập`.
Mặc định **all-or-nothing**: kiểm toàn bộ trước, có lỗi thì không ghi dòng nào, trả danh sách dòng lỗi.

---

## 6. Trạng thái rỗng & tải

| Tình huống | Hiển thị |
|---|---|
| Cột Kanban rỗng | `Không có nhiệm vụ` |
| Cột Sổ tay rỗng | `Không có việc nào.` |
| Chưa chọn thông báo | `Chọn một thông báo để xem.` |
| Chưa ghi đợt thu chi | `Chưa ghi đợt nào.` |
| Chưa có nhật ký | `Chưa có ghi chép nào.` |
| Chưa tách nhiệm vụ | `Chưa tách thành nhiệm vụ nào` |
| Đang tải | skeleton xám theo hình dạng nội dung, không dùng spinner toàn trang |

---

## 7. Responsive

| Breakpoint | Hành vi |
|---|---|
| ≥ 1280px | bố cục đầy đủ, lưới KPI 2 cột, Kanban 5 cột ngang |
| 1024–1279px | sidebar tự thu gọn thành dải icon |
| 768–1023px | lưới KPI 1 cột; bảng cuộn ngang; Sổ tay xếp dọc |
| < 768px | sidebar thành drawer trượt; Kanban cuộn ngang theo cột; drawer chi tiết chiếm toàn màn hình |

---

## 8. Khả năng tiếp cận (a11y)

Prototype đã làm tốt phần `aria-label` — **giữ nguyên khi dựng lại**:
- `Xem danh sách đằng sau: {nhãn KPI}`
- `Thông báo — {n} thông báo chưa đọc`
- `Thu gọn menu`
- `Chế độ trình chiếu phòng họp`
- `Tính lại ngay`
- `Chọn {họ tên}` (checkbox danh bạ)
- `Thêm bộ phận con` / `Sửa bộ phận` / `Xoá bộ phận`
- `Đặt "{tên}" làm con số tổng` / `Bấm để sửa tên khoản mục` / `Các đợt thu, chi của {tên}`
- `Rút khỏi danh bạ Mini App` / `Sửa thông tin cán bộ` / `Xoá khỏi danh bạ`

Bổ sung thêm: tương phản màu ≥ 4.5:1 cho chữ nhỏ, focus ring rõ, `role="tablist"/"tab"/"tabpanel"` đúng chuẩn (đã có sẵn ở Văn bản, Phản ánh, Cấu hình).
