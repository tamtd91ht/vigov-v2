# ViGov — Tổng quan hệ thống

> Tài liệu đặc tả để dựng lại ứng dụng bằng Claude Code.
> Nguồn: bản prototype `https://vigov-admin-production.up.railway.app` (phiên bản 0.1.0, môi trường phát triển).
> Đơn vị mẫu: **Uỷ ban nhân dân xã Thăng Bình — Thành phố Đà Nẵng**.

---

## 1. Sản phẩm là gì

**ViGov — Điều hành số cấp xã**: một hệ quản trị điều hành dành cho chính quyền cấp xã/phường sau sáp nhập. Ứng dụng gom toàn bộ đầu việc điều hành của một xã vào một chỗ:

- Giao việc và theo dõi nhiệm vụ (kể cả nhiệm vụ sinh ra từ kết luận họp, từ văn bản cấp trên, từ phản ánh của dân).
- Vào sổ và xử lý đơn thư công dân.
- Theo dõi giải ngân vốn đầu tư công và thu – chi ngân sách xã.
- Tiếp nhận, phân loại, xử lý phản ánh của người dân gửi qua Zalo Mini App và các kênh khác.
- Bản đồ phát triển kinh tế số (doanh nghiệp, hộ kinh doanh, hạ tầng, di tích…).
- Quản trị nội dung và danh bạ hiển thị trên Zalo Mini App của xã.
- Báo cáo điều hành, xuất PDF/XLSX/PPTX, chế độ trình chiếu phòng họp.

Triết lý sản phẩm được ghi rõ trong chính giao diện và **phải giữ lại khi dựng lại**:

> *"ViGov là công cụ theo dõi và điều hành, không phải phần mềm kế toán. Số liệu phục vụ chỉ đạo, không thay thế sổ sách kế toán và không đối chiếu với Kho bạc."*

Toàn bộ giao diện là **tiếng Việt**, dùng thuật ngữ hành chính nhà nước (bộ phận, khối, đơn thư, giải ngân, dự toán, kết luận họp…). Không dịch sang tiếng Anh.

---

## 2. Vai trò người dùng

Người dùng là cán bộ, công chức của xã. Vai trò điển hình (lấy từ tab Phân quyền):

| Vai trò | Ghi chú |
|---|---|
| Chủ tịch UBND | gắn nhãn **Lãnh đạo** |
| Phó Chủ tịch UBND | gắn nhãn **Lãnh đạo** |
| Chánh Văn phòng | |
| Trưởng bộ phận | |
| Chuyên viên chuyên môn | |
| Kế toán | |
| Cán bộ một cửa | |
| Trưởng thôn, Tổ trưởng dân phố | |

Vai trò không cố định — quản trị viên thêm/bớt vai trò và tick quyền trong `Cấu hình → Phân quyền`. Xem file `14-cau-hinh.md` để có danh sách **43 quyền** (permission key) đầy đủ.

Người dùng đăng nhập hiện tại hiển thị ở góc phải header: avatar chữ cái + họ tên + chức danh (ví dụ `V1 · Huỳnh Văn 1 · Chủ tịch UBND xã`).

---

## 3. Kiến trúc điều hướng

### 3.1 Bố cục khung (AppShell)

```
┌───────────────┬──────────────────────────────────────────────────────┐
│               │  HEADER (cao ~64px, nền trắng, viền dưới mảnh)       │
│   SIDEBAR     │  [Tên đơn vị + tỉnh/TP]  [Ô tìm kiếm]  [🔔] [Avatar] │
│   (rộng       ├──────────────────────────────────────────────────────┤
│   ~230px,     │                                                      │
│   nền tối     │  PAGE HEADER: tiêu đề + mô tả 1 dòng + nút hành động │
│   #0F172A-ish)│                                                      │
│               │  NỘI DUNG TRANG (nền xám rất nhạt #F6F8FB)           │
│               │                                                      │
└───────────────┴──────────────────────────────────────────────────────┘
```

**Sidebar**
- Trên cùng: logo ô vuông bo góc chữ `VG` + tên `ViGov` + dòng nhỏ `ĐIỀU HÀNH SỐ CẤP XÃ`.
- Nút **Thu gọn menu** (icon) ở mép phải header sidebar — thu về dạng chỉ icon.
- Menu chia **2 nhóm** với nhãn nhóm viết hoa nhỏ, màu xám:
  - `ĐIỀU HÀNH`
  - `QUẢN TRỊ`
- Mục đang chọn: nền sáng hơn + chữ trắng đậm + bo góc.
- Chân sidebar: `Phiên bản 0.1.0` / `Môi trường phát triển`.

**Header**
- Trái: `UỶ BAN NHÂN DÂN XÃ THĂNG BÌNH` (viết hoa, đậm) + dòng nhỏ `Thành phố Đà Nẵng`.
- Giữa: ô tìm kiếm toàn hệ thống, placeholder `Tìm nhiệm vụ, văn bản, phản ánh…`, aria-label `Tìm kiếm toàn hệ thống`.
- Phải: chuông thông báo (badge số chưa đọc, aria-label `Thông báo — N thông báo chưa đọc`) và khối người dùng.

### 3.2 Bản đồ route

| # | Route | Tiêu đề trang (`<title>`) | Nhãn menu | Nhóm | File đặc tả |
|---|---|---|---|---|---|
| 1 | `/tong-quan` | Tổng quan điều hành · ViGov | Tổng quan | Điều hành | `01-tong-quan-dieu-hanh.md` |
| 2 | `/nhiem-vu` | Quản lý nhiệm vụ · ViGov | Nhiệm vụ | Điều hành | `02-nhiem-vu.md` |
| 3 | `/nhiem-vu/so-tay` | Sổ tay lãnh đạo · ViGov | Sổ tay lãnh đạo | Điều hành | `03-so-tay-lanh-dao.md` |
| 4 | `/nhiem-vu/bien-ban` | Biên bản và kết luận họp · ViGov | Biên bản họp | Điều hành | `04-bien-ban-hop.md` |
| 5 | `/van-ban` | Văn bản đến & đơn thư · ViGov | Văn bản & Đơn thư | Điều hành | `05-van-ban-don-thu.md` |
| 6 | `/giai-ngan` | Theo dõi giải ngân · ViGov | Giải ngân | Điều hành | `06-giai-ngan.md` |
| 7 | `/giai-ngan/thu-chi` | Thu – Chi ngân sách xã | Thu - Chi ngân sách | Điều hành | `07-thu-chi-ngan-sach.md` |
| 8 | `/thong-bao` | Thông báo · ViGov | Thông báo | Điều hành | `08-thong-bao.md` |
| 9 | `/phan-anh` | Phản ánh của người dân · ViGov | Phản ánh người dân | Điều hành | `09-phan-anh-nguoi-dan.md` |
| 10 | `/ban-do` | Bản đồ phát triển kinh tế số | Bản đồ kinh tế số | Điều hành | `10-ban-do-kinh-te-so.md` |
| 11 | `/mini-app` | Quản trị nội dung Mini App | Nội dung Mini App | Quản trị | `11-noi-dung-mini-app.md` |
| 12 | `/danh-ba` | Danh bạ cán bộ | Danh bạ cán bộ | Quản trị | `12-danh-ba-can-bo.md` |
| 13 | `/bao-cao` | Báo cáo tổng hợp · ViGov | Báo cáo | Quản trị | `13-bao-cao.md` |
| 14 | `/cau-hinh` | Cấu hình hệ thống · ViGov | Cấu hình | Quản trị | `14-cau-hinh.md` |

Route con phát hiện thêm:
- `/giai-ngan/du-an/:id` — trang chi tiết một dự án; `:id` là **UUID** (ví dụ `8cd82a15-c425-41f1-9305-d27603b09af4`), tiêu đề trang `Chi tiết dự án · ViGov`, có nút `← Theo dõi giải ngân` quay lại.

---

## 4. Design system

### 4.1 Màu

| Vai trò | Giá trị gợi ý | Dùng ở đâu |
|---|---|---|
| Nền sidebar | `#101A2E` (xanh navy rất đậm) | sidebar |
| Nền trang | `#F5F7FA` | main |
| Nền thẻ/card | `#FFFFFF`, bo `12px`, viền `#E6EAF0`, đổ bóng rất nhẹ | mọi card |
| Primary (nút chính) | `#0F172A` → gần đen navy, chữ trắng | "Giao việc mới", "Thêm dự án", "Soạn thông báo" |
| Accent xanh dương | `#2563EB` | link, biểu đồ thực hiện, chip đang chọn |
| Cảnh báo / trễ hạn | `#DC2626` (đỏ) | số quá hạn, chip "Trễ hạn", "quá hạn N ngày" |
| Cam | `#EA580C` / `#F97316` | chip "Đang phân loại", "Quá hạn xử lý" |
| Xanh lá thành công | `#16A34A` | chip "Hoàn thành", "Đã giải quyết", "Đang hiện" |
| Xám phụ | `#64748B` | mô tả, nhãn cột, chú thích |

Quy ước màu số liệu trong bảng: tỷ lệ **< ngưỡng** hiện đỏ/cam, tỷ lệ tốt hiện xanh lá hoặc xanh dương.

### 4.2 Kiểu chữ

- Font sans-serif hệ thống (Inter / Be Vietnam Pro đều hợp), hỗ trợ đầy đủ dấu tiếng Việt.
- Tiêu đề trang: ~24px, semibold.
- Mô tả dưới tiêu đề: 13–14px, màu xám.
- Nhãn cột bảng & nhãn nhóm KPI: **viết HOA**, 11px, letter-spacing rộng, màu xám.
- Con số KPI lớn: 28–34px, semibold.

### 4.3 Thành phần dùng lại (nên tách component)

| Component | Mô tả |
|---|---|
| `PageHeader` | tiêu đề + mô tả + cụm nút hành động bên phải |
| `KpiCard` | nhãn HOA nhỏ + số lớn + dòng phụ so sánh kỳ trước |
| `KpiGroupCard` | thẻ gom nhiều KPI theo nhóm (dùng ở Tổng quan / Báo cáo) |
| `SegmentedTabs` | nhóm nút chọn 1 trong N (Tuần này / Tháng này / Quý này / Năm nay; Kanban / Danh sách / Sổ theo dõi) |
| `ScopeTabs` | `Toàn xã` · `Giao cho tôi` · `Liên quan đến tôi` (dùng ở Nhiệm vụ, Văn bản, Phản ánh) |
| `FilterBar` | hàng select + ô tìm + checkbox lọc nhanh |
| `DataTable` | bảng có sticky header, cột sắp xếp (icon ⇅), chọn nhiều bằng checkbox, hàng tô nền nhạt khi quá hạn |
| `StatusChip` | chip trạng thái bo tròn, màu theo trạng thái |
| `StatusStepper` | dải các bước trạng thái nằm ngang + nhóm "Rẽ nhánh" bên dưới (dùng ở chi tiết Nhiệm vụ và Phản ánh) |
| `DetailDrawer` | lớp phủ toàn màn hình mở chi tiết hồ sơ, nút `✕` góc phải |
| `ActivityLog` | nhật ký xử lý: ô nhập + nút `Đính kèm` + nút `Ghi nhật ký` + dòng thời gian |
| `EmptyState` | icon mờ + câu "Không có …" |
| `ExcelImportDialog` | tải mẫu + kéo thả `.xlsx` + nút `Nhập` |
| `PresentationMode` | chế độ trình chiếu phòng họp (phóng to, ẩn khung) |

### 4.4 Quy ước hiển thị dữ liệu

- **Số tiền**: dấu chấm phân tách nghìn, hậu tố ` đ` (ví dụ `33.230.000.000 đ`). Rút gọn khi chật: `3,4 tỷ`, `100 triệu`, `853,3 tỷ`.
- **Phần trăm**: dấu phẩy thập phân (`10,33%`, `75,96%`), riêng KPI dashboard dùng dấu chấm (`10.3%`) — nên thống nhất về dấu phẩy khi dựng lại.
- **Ngày**: `d/M/yyyy` (`20/6/2026`) hoặc `dd/MM/yyyy`. Ngày + giờ: `HH:mm dd/MM/yyyy` (`16:35 07/09/2026`).
- **Giá trị rỗng**: hiển thị `—` (em dash), không để trống.
- **Trễ hạn**: `Trễ N ngày`, `quá hạn N ngày M giờ`, `Quá hạn N ngày`.
- Đơn vị tính ngân sách thu–chi: **Triệu đồng** (ghi rõ trên đầu bảng).

---

## 5. Mô hình dữ liệu tổng thể

Sơ đồ quan hệ chính (chi tiết từng bảng nằm trong file module tương ứng):

```
don_vi (xã)
 ├── bo_phan (org unit, cây cha-con)          ── nguoi_dung ── vai_tro ── quyen
 ├── thon_to_dan_pho
 ├── danh_muc (lookup: 10 nhóm, 55 mục)
 ├── sla (thời hạn xử lý theo loại việc × lĩnh vực)
 │
 ├── nhiem_vu ─┬─ nhiem_vu_con
 │             ├─ nhat_ky_nhiem_vu
 │             ├─ de_nghi_lui_han
 │             └─ so_theo_doi_van_ban (metadata khi loai = theo-van-ban)
 ├── bien_ban_hop ── ket_luan_hop ──(tách ra)──> nhiem_vu
 ├── don_thu ──(tạo việc)──> nhiem_vu
 ├── phan_anh ─┬─ nhat_ky_phan_anh
 │             ├─ anh_truoc / anh_sau
 │             └─ danh_gia_hai_long
 ├── du_an ─┬─ chung_tu_giai_ngan
 │          ├─ vuong_mac
 │          └─ phan_bo_nguon_von ── nguon_von
 ├── khoan_muc_ngan_sach (cây, thu & chi) ── dot_thu_chi
 ├── thong_bao ── nguoi_nhan_thong_bao (xác nhận)
 ├── doi_tuong_ban_do (11 loại) ── truong_tuy_bien_ban_do
 └── noi_dung_mini_app (6 loại) + danh_ba_mini_app
```

### 5.1 Bảng dùng chung

**`bo_phan`** (bộ phận / khối đơn vị)

| Trường | Kiểu | Ghi chú |
|---|---|---|
| `id` | uuid | |
| `ten` | text | viết HOA, ví dụ `VĂN PHÒNG ĐẢNG ỦY` |
| `ma` | slug | `van-phong-dang-uy` |
| `cha_id` | uuid null | cây cha–con, UI có nút "Thêm bộ phận con" |
| `thu_tu` | int | |

Dữ liệu mẫu (5 bộ phận):
`LÃNH ĐẠO ỦY BAN NHÂN DÂN XÃ`, `THƯỜNG TRỰC ĐẢNG UỶ`, `THƯỜNG TRỰC HỘI ĐỒNG NHÂN DÂN`, `THƯỜNG TRỰC UBMTTQ VIỆT NAM`, `VĂN PHÒNG ĐẢNG ỦY`.

**`nguoi_dung`**

| Trường | Kiểu | Ghi chú |
|---|---|---|
| `id` | uuid | |
| `ho_ten` | text | |
| `email` | text | dạng `demo1@thangbinh.demo.vigov.vn` |
| `chuc_danh` | text | `Chủ tịch UBND xã` |
| `bo_phan_id` | uuid | |
| `vai_tro_id` | uuid | |
| `dien_thoai` | text | |
| `co_tai_khoan` | bool | có tài khoản đăng nhập hay không — bảng này chứa cả danh bạ cán bộ. Mặc định `false`, cấp tường minh |
| `dang_hoat_dong` | bool | tài khoản **chưa bị khoá**. Chip `Đang hoạt động` chỉ có nghĩa khi `co_tai_khoan = true` |
| `dang_nhap_gan_nhat` | timestamp | hoặc `Chưa đăng nhập` |
| `hien_tren_mini_app` | bool | dùng ở Danh bạ |

**`thon_to_dan_pho`**

| Trường | Kiểu | Ghi chú |
|---|---|---|
| `ten` | text | `Thôn Bình An` |
| `ma` | slug | `thon-binh-an` |
| `loai` | enum | `thon` \| `to-dan-pho` |
| `so_ho` | int | |
| `nhan_khau` | int | |
| `trang_thai` | enum | `Đang dùng` |

Dữ liệu mẫu: Bình An (284 hộ/1.132 khẩu), Bình Trị (341/1.387), Bình Dương (219/869), Hà Lam (402/1.614), Trường Giang (176/703), Phước Ấm (253/1.018).

---

## 6. Quy tắc nghiệp vụ xuyên suốt

1. **Phạm vi xem (scope)** — bộ ba tab lặp lại ở Nhiệm vụ, Văn bản & Đơn thư, Phản ánh:
   - `Toàn xã` — tất cả hồ sơ trong xã.
   - `Giao cho tôi` — đích danh tôi là người xử lý.
   - `Liên quan đến tôi` — tôi giao, tôi theo dõi, tôi đã xử lý, hoặc bộ phận tôi đang giữ.
   (Các chuỗi mô tả này chính là tooltip/aria-label trong bản gốc — giữ nguyên.)

2. **Thời hạn (SLA)** tính theo **giờ làm việc**, không tính ngày nghỉ và ngày lễ. Thay đổi cấu hình SLA **chỉ áp dụng cho hồ sơ tiếp nhận sau thời điểm lưu**.

3. **"Sắp đến hạn"** là một ngưỡng giờ cấu hình được, quyết định đồng thời 3 thứ: lúc gửi lời nhắc, ô lọc "Sắp đến hạn" trên màn nhiệm vụ, và con số trong thông báo ở chuông. Mặc định **72 giờ**.

4. **Leo thang (escalation)** 2 tầng: trễ quá số ngày đặt → báo trưởng bộ phận; trễ gấp đôi → báo Chủ tịch.

5. **Hạn gốc không bị lùi**: khi duyệt gia hạn, hệ thống giữ `hạn ban đầu` riêng để thống kê "đúng hạn" không bị bóp méo.

6. **Truy vết ngược**: nhiệm vụ sinh từ kết luận họp / văn bản đến / phản ánh luôn giữ liên kết về nguồn gốc.

7. **Danh mục (lookup) mở**: gần như mọi enum nghiệp vụ đều nằm trong bảng `danh_muc` và sửa được ở `Cấu hình → Danh mục`; mục do hệ thống tạo có nguồn `Hệ thống` và chỉ bật/tắt chứ không xoá.

8. **Nhập liệu hàng loạt bằng Excel** là mẫu lặp lại: Nhiệm vụ, Đơn thư, Giải ngân, Thu–Chi, Bản đồ, Danh bạ, Sơ đồ tổ chức, Thôn/Tổ, Danh mục đều có nút `Nhập từ Excel` / `Nạp từ Excel` kèm `Tải mẫu`.

9. **Xuất báo cáo** PDF / XLSX / PPTX + **Chế độ trình chiếu phòng họp** có ở Tổng quan, Báo cáo, Bản đồ.

---

## 7. Gợi ý stack khi dựng lại

- **Frontend**: Next.js (App Router) + TypeScript + Tailwind CSS + shadcn/ui; TanStack Table cho bảng; Recharts cho biểu đồ; MapLibre GL + OpenStreetMap cho bản đồ; `@dnd-kit` cho kéo thả Kanban.
- **Backend**: Next.js Route Handlers hoặc NestJS; PostgreSQL + Prisma; hàng đợi nền (BullMQ / pg-boss) cho 5 tác vụ tự động ở `Cấu hình → Tự động hoá`.
- **File**: S3-compatible cho ảnh phản ánh, file đính kèm, ảnh đại diện Mini App.
- **Excel**: `exceljs` (đọc & ghi). **PDF**: Puppeteer render từ HTML. **PPTX**: `pptxgenjs`.
- **Mail**: SMTP cấu hình theo đơn vị (xem `14-cau-hinh.md`, tab Máy chủ thư).
- **i18n**: chỉ tiếng Việt, nhưng tách chuỗi ra file để sửa nhanh.
- **Múi giờ**: `Asia/Ho_Chi_Minh`. Mọi lịch chạy nền tính theo giờ Việt Nam.

---

## 8. Thứ tự dựng đề xuất

1. Khung ứng dụng + xác thực + `Cấu hình` (bộ phận, người dùng, vai trò/quyền, danh mục, thôn/tổ, SLA) — nền móng cho mọi module khác.
2. `Nhiệm vụ` (lõi sản phẩm) → `Sổ tay lãnh đạo` → `Biên bản họp`.
3. `Phản ánh người dân` (module có vòng đời phức tạp thứ hai).
4. `Văn bản & Đơn thư`.
5. `Giải ngân` → `Thu – Chi ngân sách`.
6. `Bản đồ kinh tế số`.
7. `Thông báo`, `Danh bạ`, `Nội dung Mini App`.
8. `Tổng quan` + `Báo cáo` (đọc số liệu từ tất cả module trên) + xuất file + trình chiếu.
9. `Tự động hoá` (job nền) sau cùng.
