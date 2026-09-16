# 14 — Cấu hình hệ thống

**Route:** `/cau-hinh` · **Tiêu đề trang:** `Cấu hình hệ thống · ViGov` · **Menu:** Cấu hình (nhóm Quản trị)

Mô tả: *"Tổ chức, phân quyền, danh mục nghiệp vụ và thời hạn xử lý của đơn vị."*

> **Dựng module này TRƯỚC**: gần như mọi enum, mọi thời hạn, mọi câu chữ của các module khác đều lấy từ đây.

---

## 0. Bố cục

```
PageHeader "Cấu hình hệ thống" + mô tả
Tablist 10 tab (nút chữ, tab đang chọn tô đậm + nền nhạt):
[Sơ đồ tổ chức][Thôn / Tổ dân phố][Người dùng][Phân quyền][Danh mục]
[Trường bản đồ][Lời hệ thống][Thời hạn xử lý][Tự động hoá][Máy chủ thư]
Panel nội dung tab
```

---

## 1. Tab "Sơ đồ tổ chức"

Nút: `⬆ Nhập từ Excel` · `+ Thêm bộ phận`

Danh sách thẻ bộ phận (cây cha–con):
```
🏛  LÃNH ĐẠO ỦY BAN NHÂN DÂN XÃ          👥 2 cán bộ   [＋][✎][🗑]
     lanh-dao-uy-ban-nhan-dan-xa
```
| Nút | aria-label |
|---|---|
| `＋` | Thêm bộ phận con |
| `✎` | Sửa bộ phận |
| `🗑` | Xoá bộ phận |

Form bộ phận: `Tên` (viết HOA), `Mã` (slug, tự sinh từ tên), `Bộ phận cha`, `Thứ tự`.

**Dữ liệu mẫu:** 5 bộ phận — LÃNH ĐẠO ỦY BAN NHÂN DÂN XÃ (2), THƯỜNG TRỰC ĐẢNG UỶ (2), THƯỜNG TRỰC HỘI ĐỒNG NHÂN DÂN (3), THƯỜNG TRỰC UBMTTQ VIỆT NAM (2), VĂN PHÒNG ĐẢNG ỦY (3).

Bảng: `bo_phan` (xem `00-tong-quan-he-thong.md` mục 5.1).

---

## 2. Tab "Thôn / Tổ dân phố"

Nút: `⬆ Nhập từ Excel` · `+ Thêm thôn / tổ dân phố`

| Cột | Ví dụ |
|---|---|
| Tên | Thôn Bình An |
| Mã | `thon-binh-an` |
| Loại | `—` / `Thôn` / `Tổ dân phố` |
| Số hộ | 284 |
| Nhân khẩu | 1.132 |
| Trạng thái | chip `Đang dùng` |
| *(hành động)* | `✎` `🗑` |

Bảng: `thon_to_dan_pho`. Sáu bản ghi mẫu ở `00-tong-quan-he-thong.md` mục 5.1.

---

## 3. Tab "Người dùng"

Nút: `⬆ Nhập từ Excel` · `+ Thêm cán bộ`
Ô tìm: `Tìm theo tên, thư điện tử, bộ phận…`

| Cột | Ví dụ |
|---|---|
| Họ và tên ⇅ | `Huỳnh Văn 1` + dòng phụ `demo1@thangbinh.demo.vigov.vn` |
| Chức danh ⇅ | `Chủ tịch UBND xã` |
| Bộ phận ⇅ | `LÃNH ĐẠO ỦY BAN NHÂN DÂN XÃ` |
| Điện thoại ⇅ | `0900 000 001` |
| Đăng nhập gần nhất ⇅ | `11:10:38 16/9/2026` hoặc `Chưa đăng nhập` |
| Trạng thái ⇅ | chip `Đang hoạt động` |
| *(hành động)* | `✎` `🗑` |

Form người dùng: `Họ và tên`, `Thư điện tử`, `Chức danh`, `Bộ phận`, `Vai trò`, `Điện thoại`, `Trạng thái`, `Đặt lại mật khẩu`.

**12 tài khoản mẫu** `Huỳnh Văn 1..12` / `Nguyễn Thị 11` / `Nguyễn Văn 2`, email `demoN@thangbinh.demo.vigov.vn`.

---

## 4. Tab "Phân quyền"

Hướng dẫn: *"Bấm vào ô để bật hoặc tắt quyền, sau đó bấm Lưu ở đầu cột của vai trò đó."*

**Ma trận quyền**: hàng = quyền, cột = vai trò. Mỗi cột có header `{Tên vai trò}` + `{n} cán bộ` + nhãn phụ `Lãnh đạo` nếu là vai trò lãnh đạo + nút `Lưu`.

### 4.1 Tám vai trò mẫu

| Vai trò | Số cán bộ | Nhãn |
|---|---|---|
| Cán bộ một cửa | 1 | |
| Chánh Văn phòng | 1 | |
| Chủ tịch UBND | 1 | Lãnh đạo |
| Chuyên viên chuyên môn | 3 | |
| Kế toán | 1 | |
| Phó Chủ tịch UBND | 1 | Lãnh đạo |
| Trưởng bộ phận | 2 | |
| Trưởng thôn, Tổ trưởng dân phố | 2 | |

### 4.2 Bốn mươi ba quyền, gom theo 11 nhóm

| Nhóm | Mã quyền | Nhãn |
|---|---|---|
| **QUẢN TRỊ** | `admin.audit` | Xem nhật ký hệ thống |
| | `admin.lookup` | Quản lý danh mục |
| | `admin.org` | Quản lý sơ đồ tổ chức |
| | `admin.role` | Phân quyền |
| | `admin.sla` | Cấu hình thời hạn xử lý |
| | `admin.user` | Quản lý người dùng |
| **ANNOUNCEMENT** | `announcement.create` | Soạn và gửi thông báo |
| **BẢN ĐỒ TÀI NGUYÊN** | `asset.read` | Xem bản đồ tài nguyên |
| | `asset.update` | Cập nhật tài nguyên |
| **GIẢI NGÂN** | `budget.confirm` | Xác nhận, khoá khoản giải ngân |
| | `budget.read` | Xem giải ngân |
| | `budget.update` | Cập nhật giải ngân |
| **CONTENT** | `content.read` | Xem nội dung và danh bạ Mini App |
| | `content.update` | Sửa nội dung và danh bạ Mini App |
| **VĂN BẢN** | `document.create` | Vào sổ văn bản |
| | `document.read` | Xem văn bản |
| | `document.route` | Phân luồng văn bản |
| **PHẢN ÁNH** | `feedback.assign` | Phân công xử lý phản ánh |
| | `feedback.create` | Tiếp nhận phản ánh |
| | `feedback.read` | Xem phản ánh |
| | `feedback.resolve` | Kết thúc xử lý phản ánh |
| | `feedback.restricted` | Xem phản ánh về tác phong cán bộ |
| **ĐƠN THƯ** | `petition.create` | Tiếp nhận đơn thư |
| | `petition.read` | Xem đơn thư |
| **BÁO CÁO** | `report.export` | Xuất báo cáo |
| | `report.read` | Xem báo cáo |
| **NHIỆM VỤ** | `task.approve` | Duyệt hoàn thành |
| | `task.assign` | Giao nhiệm vụ |
| | `task.create` | Tạo nhiệm vụ |
| | `task.delete` | Xoá nhiệm vụ khỏi sổ |
| | `task.extend` | Duyệt gia hạn |
| | `task.read` | Xem nhiệm vụ |
| | `task.update` | Cập nhật tiến độ |

> Hai nhóm `ANNOUNCEMENT` và `CONTENT` chưa được dịch nhãn nhóm sang tiếng Việt trong prototype. Khi dựng lại: đổi thành `THÔNG BÁO` và `NỘI DUNG MINI APP`.

**Bảng:** `vai_tro(id, ten, la_lanh_dao bool, thu_tu)` · `quyen(ma, nhom, nhan)` · `vai_tro_quyen(vai_tro_id, quyen_ma)`.

---

## 5. Tab "Danh mục"

Nút: `⬆ Nhập từ Excel` · `+ Thêm mục`
Chip lọc nhóm: `Tất cả (55)` + 10 nhóm danh mục.

| Cột | Ví dụ |
|---|---|
| Nhóm danh mục | `Loại tài nguyên bản đồ` |
| Mã | `doanh-nghiep` (font mono) |
| Nhãn hiển thị | `Doanh nghiệp` |
| Thứ tự | 1 |
| Nguồn | `Hệ thống` / `Đơn vị` |
| Trạng thái | `Đang dùng` / `Đã tắt` |
| *(hành động)* | `✎` `Tắt` `🗑` |

### 10 nhóm danh mục, 55 mục

| Nhóm | Số mục | Các mã |
|---|---|---|
| Loại tài nguyên bản đồ | 8 | `doanh-nghiep`, `ho-kinh-doanh`, `cho`, `truong-hoc`, `co-so-y-te`, `di-tich`, `ocop`, `ha-tang` |
| Hạng mục kế hoạch vốn | 6 | `chuyen-tiep`, `xay-dung-moi`, `tra-no`, `keo-dai`, `nong-thon-moi`, `tra-no-chua-phan-bo` |
| Loại văn bản | 7 | `cong-van`, `quyet-dinh`, `thong-bao`, `ke-hoach`, `giay-moi`, `bao-cao`, `to-trinh` |
| Lĩnh vực phản ánh | 12 | xem `09-phan-anh-nguoi-dan.md` mục 5 |
| Loại đơn vị dân cư | 2 | `thon`, `to-dan-pho` |
| Loại đơn thư | 5 | `kien-nghi`, `phan-anh`, `khieu-nai`, `to-cao`, `de-nghi` |
| Khối nhiệm vụ | 3 | `khoi-uy-ban`, `khoi-dang`, `khac` |
| Loại nhiệm vụ | 2 | `theo-van-ban` *(Mặc định)*, `co-ban` |
| Mức ưu tiên nhiệm vụ | 3 | `khan`, `cao`, `thuong` |
| Trạng thái nhiệm vụ | 7 | `moi-giao`, `da-tiep-nhan`, `dang-thuc-hien`, `cho-duyet`, `hoan-thanh`, `tam-dung`, `chuyen-tiep` |

Mục có badge `Mặc định` (ví dụ `Theo văn bản`); mục khác có nút `Đặt mặc định`.

**Bảng:** `danh_muc(id, nhom, ma, nhan, thu_tu, nguon enum('he-thong','don-vi'), la_mac_dinh bool, dang_dung bool)`

**Quy tắc:** mục `nguon = 'he-thong'` **không xoá được**, chỉ `Tắt`.

---

## 6. Tab "Trường bản đồ"

Select `Nhóm tài nguyên` (11 nhóm) · nút `+ Thêm trường`

| Cột | Ví dụ |
|---|---|
| Nhãn hiển thị | `Loại hình doanh nghiệp` |
| Mã trường | `legal_form` |
| Kiểu dữ liệu | `Chọn trong danh sách` |
| Bắt buộc | `—` / `Có` |
| Thứ tự | 1 |
| Trạng thái | `Đang dùng` |
| *(hành động)* | `✎` `Tắt` `🗑` |

Mẫu nhóm Doanh nghiệp: `Loại hình doanh nghiệp` (`legal_form`, Chọn trong danh sách) và `Doanh thu ước (đồng/năm)` (`revenue_estimate`, Số thập phân).

Chú thích: *"Cột trong tệp Excel nhập vào chỉ được giữ lại khi có trường tương ứng ở đây. Trường đã xoá thì dữ liệu cũ vẫn còn trong hồ sơ, chỉ không hiện lên trên biểu mẫu nữa."*

**Bảng:** `truong_tuy_bien_ban_do` (xem `10-ban-do-kinh-te-so.md` mục 10).

---

## 7. Tab "Lời hệ thống"

Hướng dẫn: *"Những câu dưới đây là lời hệ thống nói với người dân. Chúng hiện ở cả trang quản trị và Zalo Mini App, nên sửa ở đây là sửa cho cả hai. Câu đi kèm phần mềm có thể sửa lời nhưng không xoá được — xoá đi thì lúc từ chối, hệ thống không còn gì để nói."*

Nút `+ Thêm câu mới`.

Mỗi câu là một card, gom theo nhóm:
```
report.block.alerts   [Đi kèm phần mềm]
Tên khối cảnh báo trên báo cáo xuất ra.
┌──────────────────────────────────────────┐
│ Cần xử lý ngay                           │   ← textarea sửa được
└──────────────────────────────────────────┘
[Lưu]  [Tắt]
```

### Ba nhóm câu và toàn bộ khoá

**Nhóm `bao-cao` (32 khoá)**

| Khoá | Mô tả |
|---|---|
| `report.title` | Tiêu đề trang đầu của báo cáo xuất ra PDF, Excel và PowerPoint |
| `report.block.alerts` | Tên khối cảnh báo trên báo cáo xuất ra |
| `report.block.budget` | Tên khối giải ngân |
| `report.block.economy` | Tên khối kinh tế và tài nguyên |
| `report.block.feedback` | Tên khối phản ánh |
| `report.block.ranking` | Tên bảng xếp hạng bộ phận |
| `report.block.register` | Tên khối văn bản và đơn thư |
| `report.block.tasks` | Tên khối nhiệm vụ |
| `report.metric.tasks.open` | Nhiệm vụ đang thực hiện |
| `report.metric.tasks.overdue` | Nhiệm vụ quá hạn |
| `report.metric.tasks.done` | Nhiệm vụ hoàn thành trong kỳ |
| `report.metric.tasks.on_time_percent` | Tỷ lệ nhiệm vụ hoàn thành đúng hạn |
| `report.metric.register.arrived` | Văn bản, đơn thư đến trong kỳ |
| `report.metric.register.open` | Văn bản, đơn thư chưa xử lý xong |
| `report.metric.register.overdue` | Văn bản, đơn thư quá hạn xử lý |
| `report.metric.register.petition_arrived` | Đơn thư công dân đến trong kỳ |
| `report.metric.budget.disbursed_percent` | Phần trăm giải ngân trên kế hoạch năm |
| `report.metric.budget.time_percent` | Phần trăm thời gian năm ngân sách đã trôi qua |
| `report.metric.budget.behind` | Số dự án giải ngân chậm so với thời gian |
| `report.metric.budget.open_issues` | Số vướng mắc giải ngân chưa xử lý xong |
| `report.metric.budget.disbursed_amount` | Số tiền đã giải ngân trong năm |
| `report.metric.feedback.received` | Phản ánh tiếp nhận trong kỳ |
| `report.metric.feedback.open` | Phản ánh đang xử lý |
| `report.metric.feedback.on_time_percent` | Tỷ lệ phản ánh xử lý đúng hạn |
| `report.metric.feedback.late` | Số phản ánh xử lý trễ hạn |
| `report.metric.feedback.rating` | Điểm hài lòng trung bình của người dân |
| `report.metric.economy.enterprises` | Số doanh nghiệp trên địa bàn |
| `report.metric.economy.household_businesses` | Số hộ kinh doanh cá thể |
| `report.metric.economy.new_in_period` | Cơ sở kinh doanh thành lập mới trong kỳ |
| `report.metric.economy.total` | Tổng số đối tượng trong danh mục bản đồ |
| `report.notification.week` | Tiêu đề thông báo gửi lãnh đạo sáng thứ Hai, kèm các con số chính |
| `report.notification.month` | Tiêu đề thông báo gửi lãnh đạo đầu tháng |

**Nhóm `Theo dõi giải ngân` (1 khoá)**

| Khoá | Mô tả |
|---|---|
| `budget.scope_notice` | Dòng ghi rõ ranh giới sản phẩm, hiện ở đầu màn hình theo dõi giải ngân **và đi kèm mọi số liệu API trả về** |

**Nhóm `Phản ánh của người dân` (6 khoá)**

| Khoá | Mô tả |
|---|---|
| `feedback.after_photo_required` | Hiện khi cán bộ bấm đóng phiếu mà chưa đính kèm ảnh sau xử lý. Hiển thị ở cả web quản trị và Mini App |
| `feedback.assignment_required` | Hiện khi chuyển xử lý mà chưa chọn bộ phận lẫn người |
| `feedback.invalid_transition` | Hiện khi thao tác nhảy bước không hợp lệ trong quy trình |
| `feedback.never_public` | Hiện khi ai đó cố duyệt công khai một phiếu thuộc luồng riêng |
| `feedback.reason_required` | Hiện khi từ chối phiếu mà bỏ trống lý do |
| `feedback.unknown_field` | Hiện khi phiếu gửi lên với mã lĩnh vực không tồn tại |

**Bảng:** `loi_he_thong(id, nhom, khoa, mo_ta, noi_dung, di_kem_phan_mem bool, dang_dung bool)`

> **Đây là design pattern quan trọng:** mọi nhãn KPI trên báo cáo và mọi thông báo lỗi hướng tới người dân đều là **dữ liệu sửa được**, không hard-code. Khi dựng lại, viết một helper `t(khoa, mac_dinh)` đọc từ bảng này với fallback là giá trị mặc định trong code.

---

## 8. Tab "Thời hạn xử lý" (SLA)

Hai câu dẫn **bắt buộc giữ**:
> *"Thời hạn tính theo giờ làm việc, không tính ngày nghỉ và ngày lễ. Thay đổi chỉ áp dụng cho hồ sơ tiếp nhận sau thời điểm lưu."*
>
> *"Cột Sắp đến hạn khi còn quyết định cả ba: lúc nào gửi lời nhắc, ô lọc 'Sắp đến hạn' trên màn nhiệm vụ lấy ra việc nào, và con số trong thông báo ở chuông. Mặc định 72 giờ, tức ba ngày."*

Nút: `+ Thêm thời hạn cho một lĩnh vực`

| Loại việc | Lĩnh vực | Tiếp nhận | Xử lý xong | Sắp đến hạn khi còn | Báo lãnh đạo trực tiếp | Báo Chủ tịch |
|---|---|---|---|---|---|---|
| Văn bản đến | Mặc định cho mọi lĩnh vực | 8 giờ | 40 giờ | 24 giờ | sau 24 giờ | sau 48 giờ |
| Phản ánh của người dân | An ninh trật tự | 2 giờ | 16 giờ | 4 giờ | sau 8 giờ | sau 16 giờ |
| Phản ánh của người dân | An toàn thực phẩm | 2 giờ | 24 giờ | 6 giờ | sau 12 giờ | sau 24 giờ |
| Phản ánh của người dân | Thái độ / tác phong cán bộ | 8 giờ | 120 giờ | 24 giờ | sau 24 giờ | sau 48 giờ |
| Phản ánh của người dân | Cấp thoát nước | 4 giờ | 48 giờ | 8 giờ | sau 16 giờ | sau 32 giờ |
| Phản ánh của người dân | Điện | 2 giờ | 24 giờ | 6 giờ | sau 12 giờ | sau 24 giờ |
| Phản ánh của người dân | Hạ tầng giao thông | 8 giờ | 168 giờ | 24 giờ | sau 24 giờ | sau 48 giờ |
| Phản ánh của người dân | Khác | 8 giờ | 72 giờ | 24 giờ | sau 24 giờ | sau 48 giờ |
| Phản ánh của người dân | Ô nhiễm (tiếng ồn, khí thải, nước thải) | 8 giờ | 72 giờ | 24 giờ | sau 24 giờ | sau 48 giờ |
| Phản ánh của người dân | Rác thải – Vệ sinh môi trường | 4 giờ | 24 giờ | 8 giờ | sau 16 giờ | sau 32 giờ |
| Phản ánh của người dân | Trật tự đô thị – lấn chiếm vỉa hè | 6 giờ | 40 giờ | 12 giờ | sau 24 giờ | sau 48 giờ |
| Phản ánh của người dân | `ve-sinh-moi-truong` *(mã cũ, không còn trong danh mục)* | 4 giờ | 24 giờ | 8 giờ | sau 16 giờ | sau 32 giờ |
| Phản ánh của người dân | Xây dựng không phép | 4 giờ | 72 giờ | 12 giờ | sau 12 giờ | sau 24 giờ |
| Phản ánh của người dân | Y tế – Giáo dục | 8 giờ | 72 giờ | 24 giờ | sau 24 giờ | sau 48 giờ |
| Phản ánh của người dân | Mặc định cho mọi lĩnh vực | 8 giờ | 56 giờ | 24 giờ | sau 24 giờ | sau 48 giờ |
| Nhiệm vụ | Mặc định cho mọi lĩnh vực | 8 giờ | 40 giờ | 72 giờ | sau 24 giờ | sau 48 giờ |

> Dòng `ve-sinh-moi-truong` hiển thị **mã thô** vì lĩnh vực đó đã bị đổi mã. Khi dựng lại: thêm ràng buộc khoá ngoại hoặc migration dọn.

**Bảng:** `sla(id, loai_viec enum('van-ban-den','phan-anh','nhiem-vu'), linh_vuc text null, gio_tiep_nhan, gio_xu_ly_xong, gio_sap_den_han, gio_bao_lanh_dao, gio_bao_chu_tich)` — `linh_vuc = null` là dòng mặc định.

**Lịch làm việc** cần bảng phụ: `lich_lam_viec(thu, gio_bat_dau, gio_ket_thuc)` + `ngay_nghi_le(ngay, ten)` để tính "giờ làm việc".

---

## 9. Tab "Tự động hoá"

Hướng dẫn: *"Những việc phần mềm tự làm cho xã. Mặc định tắt hết — bật cái nào thì xã tự chọn, và giờ giấc tính theo giờ Việt Nam. Đổi nhịp có hiệu lực ngay ở lượt chạy kế tiếp."*

Năm job, mỗi job một card: tên + mô tả + dòng chạy lần cuối + công tắc bật/tắt + (khi bật) chọn nhịp chạy.

| Job | Mô tả | Trạng thái mẫu |
|---|---|---|
| **Nhắc việc sắp đến hạn và đã quá hạn** | *"Quét nhiệm vụ, văn bản và phản ánh; nhắc người phụ trách trước khi đến hạn và báo khi đã quá hạn. Cả việc bộ phận giữ mà chưa phân công ai."* | Chưa chạy lần nào · Đang tắt |
| **Leo thang việc trễ hạn** | *"Việc trễ quá số ngày đã đặt thì báo lên trưởng bộ phận, trễ gấp đôi thì báo lên chủ tịch. Số ngày lấy từ Thời hạn xử lý."* | Chưa chạy lần nào · Đang tắt |
| **Bản tin đầu tuần cho lãnh đạo** | *"Tóm tắt việc tồn, việc trễ và phản ánh nóng của tuần trước."* | Chưa chạy lần nào · Đang tắt |
| **Tính lại số liệu Tổng quan** | *"Dựng sẵn số liệu bảng điều hành. Tắt thì màn Tổng quan chỉ đổi khi có người bấm Tính lại ngay."* | Chạy lần cuối 22:05 30/08/2026 · Đang tắt |
| **Gửi báo cáo định kỳ** | *"Báo cáo tuần vào đầu tuần, báo cáo tháng vào ngày mùng 1."* | Chưa chạy lần nào · Đang tắt |

**Bảng:** `tac_vu_tu_dong(ma, ten, mo_ta, bat bool, nhip_cron text, chay_lan_cuoi timestamp, ket_qua_lan_cuoi jsonb)`

Múi giờ: `Asia/Ho_Chi_Minh`.

---

## 10. Tab "Máy chủ thư"

Card `✉ Máy chủ thư của xã`
Mô tả: *"Thông báo nội bộ gửi từ hộp thư của cán bộ bằng chính địa chỉ công vụ của xã. Chưa khai thì hệ thống dùng máy chủ thư của tỉnh nếu tỉnh có."*
Cảnh báo cam: *"⚠ Chưa có máy chủ thư nào. Thông báo vẫn đúng đường nhưng chưa thật sự gửi đi."*

| Trường | Kiểu | Mẫu |
|---|---|---|
| Máy chủ SMTP | text | `smtp.danang.gov.vn` |
| Cổng | number | `587` — dòng phụ: *"587 dùng START TLS, 465 dùng TLS ngay từ đầu."* |
| Tài khoản | text | |
| Mật khẩu | password | |
| Địa chỉ gửi | email | `ubnd@xa.danang.gov.vn` |
| Tên hiển thị của người gửi | text | `ViGov` |
| ☐ Dùng máy chủ thư này cho xã | radio | |
| ☑ START TLS (thường dùng với cổng 587) | radio | **mặc định** |
| ☐ TLS ngay từ đầu (thường dùng với cổng 465) | radio | |

Nút `💾 Lưu cấu hình`.
Khối kiểm thử bên phải: `Gửi thư thử tới` [email] + nút `✉ Gửi thử`.

**Bảng:** `cau_hinh_thu(id, smtp_host, smtp_port, tai_khoan, mat_khau_ma_hoa, dia_chi_gui, ten_hien_thi, bao_mat enum('starttls','tls','none'), dang_dung bool)`

---

## 11. API đề xuất

| Method | Endpoint |
|---|---|
| GET/POST | `/api/cau-hinh/bo-phan` · PATCH/DELETE `/:id` · POST `/nhap-excel` |
| GET/POST | `/api/cau-hinh/thon` · PATCH/DELETE `/:id` |
| GET/POST | `/api/cau-hinh/nguoi-dung` · PATCH/DELETE `/:id` |
| GET | `/api/cau-hinh/quyen` — ma trận vai trò × quyền |
| PUT | `/api/cau-hinh/vai-tro/:id/quyen` — lưu cả cột |
| GET/POST | `/api/cau-hinh/danh-muc?nhom=` · PATCH/DELETE `/:id` · POST `/:id/mac-dinh` |
| GET/POST | `/api/cau-hinh/truong-ban-do?nhom=` |
| GET/PATCH | `/api/cau-hinh/loi-he-thong` |
| GET/POST/PATCH/DELETE | `/api/cau-hinh/sla` |
| GET/PATCH | `/api/cau-hinh/tu-dong-hoa` · POST `/:ma/chay-ngay` |
| GET/PUT | `/api/cau-hinh/may-chu-thu` · POST `/gui-thu-thu` |

---

## 12. Quy tắc nghiệp vụ

1. Mọi thay đổi ở tab này ghi vào **nhật ký hệ thống** (quyền `admin.audit` để xem).
2. Danh mục `Hệ thống` không xoá được — chỉ tắt.
3. SLA **không hồi tố**.
4. Xoá bộ phận có cán bộ hoặc hồ sơ đang giữ ⇒ chặn, yêu cầu chuyển trước.
5. Lưu phân quyền theo **từng cột vai trò** (nút `Lưu` ở đầu cột), không lưu toàn ma trận.
6. Mật khẩu SMTP mã hoá khi lưu, không trả về client (chỉ hiện `****654bf`).
7. Bật một job tự động hoá ⇒ lượt chạy kế tiếp áp dụng ngay.
8. Quyền để vào tab này: tương ứng `admin.org`, `admin.user`, `admin.role`, `admin.lookup`, `admin.sla`, `admin.audit`. Tab nào thiếu quyền thì ẩn tab đó.
