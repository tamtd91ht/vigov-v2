---
id: ubiquitous-language
tier: T0
source: CURATED
owner: domain
derived_from_commit: 0960b2a
expires: null
owns_facts:
  - "ánh xạ thuật ngữ hành chính sang tên dùng trong mã"
  - "ánh xạ khái niệm nghiệp vụ sang danh từ tài nguyên trên URL"
  - "vì sao dự án đầu tư là investment-projects chứ không projects hay disbursements/projects"
  - "vì sao khoá quyền feedback.* lệch với tài nguyên citizen-reports"
  - "tiền tố my- cho tuyến công dân đọc dữ liệu của chính mình"
  - "vì sao khái niệm cán bộ mang bốn cái tên trên bốn bề mặt"
  - "phân biệt phản ánh, khiếu nại, tố cáo"
  - "tên gọi các bước trong vòng đời phiếu phản ánh"
  - "mã và nhãn của chín trạng thái phiếu phản ánh"
  - "tên hai cột hạn của phiếu phản ánh, và nghĩa của NULL trên từng cột"
  - "tên gọi vai trò cán bộ cấp xã"
  - "tên thực thể tiếng Anh của tám danh mục tham chiếu"
  - "tên tài nguyên URL của ba bảng lịch làm việc của xã"
  - "vì sao giữ public-holidays dù closure-days đúng nghĩa hơn"
  - "vì sao hợp đồng danh mục trả trường label còn thực thể có tên riêng trả name"
  - "quy tắc: tên theo khái niệm, quyền sở hữu theo nhịp đổi"
---

# Ngôn ngữ chung

**Bảng duy nhất** ánh xạ thuật ngữ hành chính sang tên trong mã. Đặt tên sai ở schema thì
mọi tầng trên đều sai theo, và sửa về sau là di trú dữ liệu.

## Ba thuật ngữ khác nhau về pháp lý — hay bị dùng lẫn

| Thuật ngữ | Nghĩa | Tên trong mã | Hệ quả nếu gọi nhầm |
|---|---|---|---|
| **Phản ánh, kiến nghị** | Người dân nêu vấn đề, đề xuất | `phan_anh` | — |
| **Khiếu nại** | Không đồng ý với **quyết định hành chính** cụ thể | `khieu_nai` | Áp sai thủ tục và **sai thời hạn luật định** |
| **Tố cáo** | Báo hành vi vi phạm pháp luật | `to_cao` | Mất **bảo vệ người tố cáo** |

Ba từ này là **giá trị enum**, và giá trị enum **không dịch sang tiếng Anh** — ADR 0011.

## Bảng thuật ngữ

| Tiếng Việt hành chính | Tên trong mã | Ghi chú |
|---|---|---|
| Văn bản đến | `van_ban_den` | Từ nơi khác gửi tới xã |
| Văn bản đi | `van_ban_di` | Xã ban hành gửi đi |
| Số đến / số đi | `so_den` / `so_di` | Đánh số **theo từng cơ quan**, lại từ 01 mỗi năm |
| Thụ lý | `thu_ly` | Nhận và bắt đầu xử lý — không dùng "xử lý" |
| Luân chuyển | `luan_chuyen` | Chuyển giữa các bộ phận — không dùng "chuyển" |
| Ban hành | `ban_hanh` | Ký và phát hành chính thức |
| Hồ sơ một cửa | — | **NGOÀI PHẠM VI từ 20/09/2026** (ADR 0001 §Bổ sung). Không có bảng nào trong kho. Chính tả: không viết "1 cửa" |
| Đơn thư | `don_thu` | Bao gồm khiếu nại, tố cáo, kiến nghị |
| Giải ngân | `giai_ngan` | |
| Nhiệm vụ | `nhiem_vu` | Việc giao cho cán bộ |
| Cán bộ, công chức | `can_bo` | Người dùng nội bộ — **bảng lại tên là `nguoi_dung`**, xem mục "Một khái niệm, bốn cái tên" |
| Công dân | `cong_dan` | Người dân — **không** gọi là "khách hàng", "user" |
| Đơn vị hành chính | `don_vi_hanh_chinh` | Cấp xã gồm **ba** loại: xã / phường / **đặc khu**. Từ 01/7/2025 **không còn cấp huyện** — ADR 0023 |

## Phản ánh và SLA

Vòng đời đầy đủ ở `skills/petition-lifecycle`; bảng này chỉ chốt **từ dùng**.

| Tiếng Việt hành chính | Tên trong mã | Ghi chú |
|---|---|---|
| Phiếu phản ánh | `phieu_phan_anh` | Một lượt phản ánh, có **mã tra cứu** trả cho dân |
| Mã tra cứu | `ma_tra_cuu` | Không tuần tự, không tái cấp (luật 4, 7) |
| Lĩnh vực | `linh_vuc` | Rác thải, giao thông, trật tự đô thị… — **mã do nền tảng cấp và đóng; xã chỉ đổi được NHÃN, không thêm mã** (ADR 0026). Dòng này trước 20/09/2026 ghi "cấu hình theo xã" và **đã sai**: câu ấy đúng cho bảy danh mục của ADR 0024, không đúng cho danh mục này — số liệu lĩnh vực phải cộng được giữa các xã |
| Trạng thái phiếu | `trang_thai` | Chín giá trị, danh sách đóng — §Chín trạng thái ngay dưới |
| Tiếp nhận | `tiep_nhan` | **Mốc bắt đầu đếm hạn** — không phải lúc phân công |
| Phân loại | `phan_loai` | Cán bộ xác định lĩnh vực — **không** để dân tự chọn (câu mở #23 đã đóng theo hướng này, ADR 0028). Cũng là **hành vi ấn định `han_xu_ly_xong`** cho phiếu dân tự gửi |
| Phân công | `phan_cong` | Giao cán bộ/đơn vị xử lý |
| Nghiệm thu | `nghiem_thu` | Xác nhận đã xử lý trên thực địa, thường kèm ảnh |
| Đóng phiếu | `dong_phieu` | Kết thúc — **bắt buộc có kết quả dân đọc được** |
| Hạn tiếp nhận | `han_tiep_nhan` | Hạn **có cán bộ đọc phiếu**. Đặt lúc **sinh phiếu**, lấy dòng mặc định của bảng SLA. **Cho phép `NULL`, nghĩa là "KHÔNG ÁP DỤNG"** — phiếu nhập hộ, vì chính cán bộ là người vào sổ. `NULL` ở đây **không** phải "chưa có", và báo cáo phải **loại** các dòng ấy, tuyệt đối không đọc thành 0 giờ (ADR 0028) |
| Hạn xử lý xong | `han_xu_ly_xong` | Hạn **xử lý xong**, lấy theo lĩnh vực. Với phiếu dân tự gửi thì đặt lúc **chốt lĩnh vực**, nên **`NULL` trong khoảng chờ phân loại — nghĩa là "CHƯA CÓ"**. Với phiếu nhập hộ thì đặt ngay lúc vào sổ (ADR 0028) |
| Hạn xử lý *(nói chung)* | `sla_deadline` | Từ dùng chung cho **một** trong hai cột trên khi ngữ cảnh đã rõ. Mỗi hạn tính **một lần, tại hành vi ấn định nó**, rồi lưu; tính bằng **giờ làm việc**. Hai cột, hai thời điểm ấn định, **một** gốc đếm — ADR 0028 |
| Giờ làm việc | `gio_lam_viec` | Theo `lich_lam_viec` + `ngay_nghi_le` + `ngay_lam_bu` của xã — **cả BA bảng**, ADR 0007. Thiếu bảng thứ ba là đếm xuyên ngày làm bù như thể xã đóng cửa, và ngày làm bù dồn quanh Tết và Quốc khánh. Tên tài nguyên URL và hình dạng từng dòng: §Lịch làm việc của xã |
| Quá hạn | — | **Suy ra**, không có cột. Xem luật 10 |

### Chín trạng thái của phiếu phản ánh — ADR 0027

**BƯỚC không phải TRẠNG THÁI, và bảng trên với bảng này nói hai thứ khác nhau.** Bảng trên đặt
tên cho **hành vi** của cán bộ (`phan_loai` là việc một người làm); bảng dưới đặt tên cho **chỗ
phiếu đang đứng** khi nhìn vào sổ. Bảy bước và chín trạng thái không trùng số nhau, và ép chúng
trùng là cách một cột `trang_thai` bị viết thành nhật ký thao tác.

Mã viết **tiếng Việt không dấu**, kebab-case — ADR 0011, và giá trị enum không dịch sang tiếng
Anh. Nhãn là chuỗi người đọc, lấy nguyên của khách.

| Mã | Nhãn | Nhóm |
|---|---|---|
| `da-tiep-nhan` | Đã tiếp nhận | luồng chính — **tự động**, phần mềm sinh phiếu |
| `dang-phan-loai` | Đang phân loại | luồng chính — **cán bộ động vào lần đầu tại đây**, ADR 0027 quyết định D |
| `da-chuyen-xu-ly` | Đã chuyển xử lý | luồng chính |
| `dang-xu-ly` | Đang xử lý | luồng chính |
| `da-xu-ly` | Đã xử lý | luồng chính |
| `cho-dan-xac-nhan` | Chờ dân xác nhận | luồng chính |
| `da-dong` | Đã đóng | luồng chính |
| `khong-tiep-nhan` | Không tiếp nhận | rẽ nhánh |
| `chuyen-cap-tren` | Chuyển cấp trên | rẽ nhánh |

**Danh sách này đóng.** Xã không thêm, không bớt — lý do và ca hỏng cụ thể ở ADR 0027, không
chép lại. Hệ quả cho người viết mã: máy chuyển trạng thái được phép gọi thẳng chín mã này.

**Đặc tả đã sửa theo bảng này ngày 2026-09-20**, sau khi hỏi khách: `docs/ui-ux/09-phan-anh-nguoi-dan.md`
§6 trước đó ghi chín mã bằng chuỗi tiếng Anh của bản mẫu, nay mang đúng chín mã trên. Gặp lại
`received` · `screening` · `assigned` ở đâu đó thì đó là **tên đã bị thay**, không phải cách
gọi thứ hai đang song song. Vì sao mã lấy theo nhãn chứ không dịch ngược chuỗi tiếng Anh: ADR
0027 §"Đặc tả đã sửa theo".

**Khách đã DUYỆT nguyên văn chín chuỗi mã ngày 2026-09-20** — không chỉ danh sách, mà từng ký
tự. Dịp đọc lại rẻ đã dùng hết: **từ nay đổi một trong chín chuỗi là DI TRÚ HỒ SƠ LƯU TRỮ
(luật 7), không phải đổi tên.** Gặp lại ở đâu đó một cảnh báo "cách viết còn chờ khách duyệt"
thì đó là bản sao sót lại, không phải một nghi ngờ còn sống.

## Vai trò cán bộ

| Tiếng Việt | Tên trong mã | Việc thật ở xã |
|---|---|---|
| Lãnh đạo | `lanh_dao` | Chủ tịch / Phó chủ tịch — **duyệt**, không tự xử lý |
| Chuyên viên | `chuyen_vien` | Công chức xử lý chuyên môn |
| Kế toán | `ke_toan` | Giải ngân |
| Tiếp nhận một cửa | `tiep_nhan` | Cán bộ tiếp dân |
| Quản trị | `quan_tri` | Phụ trách kỹ thuật của xã |

## Tên tài nguyên trên URL

ADR 0011 chốt: **đoạn đường dẫn dùng tiếng Anh, giá trị enum giữ tiếng Việt không dấu.**
Nghĩa là một khái niệm mang **hai từ vựng** — tên trong CSDL và danh từ tài nguyên trên URL.
Bảng dưới là **chỗ duy nhất** giữ ánh xạ đó. Tự dịch tại chỗ khi viết route là cách hai phiên
khác nhau đặt hai tên khác nhau cho cùng một thứ.

Cột **Tài nguyên URL** ghi danh từ đã chốt; đường dẫn đầy đủ **đang chạy** thì tra
`kb/20-contracts/openapi.json` — nó sinh từ mã (ADR 0014), nên nó không trôi, còn bảng này thì
có thể.

| Khái niệm | Tên trong mã | Tài nguyên URL | Vì sao không phải từ dễ đoán |
|---|---|---|---|
| Phản ánh | `phan_anh` | `citizen-reports` | "feedback" nghĩa là góp ý sản phẩm; phản ánh là **một loại đơn có thủ tục hành chính** |
| Đơn thư | `don_thu` | `citizen-letters` + `?type=` **bắt buộc** | Một sổ, **một dãy số liên tục**; tách thành nhiều đường dẫn là tách dãy số của hồ sơ lưu trữ |
| Văn bản đến | `van_ban_den` | `incoming-documents` | |
| Văn bản đi | `van_ban_di` | `outgoing-documents` | |
| Nhiệm vụ | `nhiem_vu` | `tasks` | |
| Giải ngân | `giai_ngan` | `disbursements` | |
| **Dự án đầu tư** | `du_an` (bảng) · **`@entity: Project`** — xem ô bên phải | `investment-projects` — `GET /api/v1/investment-projects` · `…/{id}` | **`disbursements/projects` LỒNG NGƯỢC quan hệ nghiệp vụ và đã bị bác:** một dự án tồn tại độc lập, còn chứng từ giải ngân mới là thứ **thuộc về** dự án (`service-finance/migrations/0004_du_an_va_chung_tu_giai_ngan.sql` — `chung_tu_giai_ngan.du_an_id`, không có chiều ngược lại). Lồng một danh từ dưới một danh từ khác là **khai quan hệ sở hữu trên URL**, nên lồng ngược là nói sai nghiệp vụ ở chỗ bên tích hợp đọc.<br><br>**`investment-projects` chứ không `projects` trần:** ADR 0011 lấy `org-units` làm ví dụ *"từ dễ đoán nhưng sai"*. Ngày xã có "dự án" ở nghĩa khác — dự án dân sinh, dự án chuyển đổi số — thì danh từ `projects` đã bị chiếm và **không đòi lại được**.<br><br>⚠ **Dấu trong migration là `@entity: Project`, KHÔNG phải `InvestmentProject` — chỗ lệch có thật, đừng "sửa cho khớp".** Đúng quy tắc §Quy ước đặt tên: tên thực thể đi theo **khái niệm** (trong miền tài chính của một xã, `Project` không mơ hồ — `:174` ghi rõ *"one investment project"*), còn danh từ URL phải sống chung với **mọi** nghĩa khác của chữ "dự án" mà hệ thống sẽ gặp sau này. Sửa dấu là sửa một migration **đã áp** — `core/migrate` so checksum mỗi lần khởi động (cùng ca với `PublicHoliday` ở §Lịch làm việc) |
| Phiên đăng nhập | `phien` | `sessions` — `POST /api/v1/sessions` · `GET …/current` · `DELETE …/{sid}` | Phiên **cán bộ**. Đăng nhập là **tạo một phiên**, không phải `POST /login`: động từ không thành đường dẫn (`rest-api-design` REQUIRED #8). Phiên công dân là khái niệm khác, bảng khác — xem bảng kênh công dân |
| Bộ phận | `bo_phan` | `org-units` — `GET /api/v1/org-units`, `AnyAuthenticated` | Cây này chứa Đảng uỷ, HĐND, UBMTTQ — **không phải** phòng ban của UBND, nên không gọi `departments` |
| Vai trò | `vai_tro` | `roles` — `GET /api/v1/roles`, `AnyAuthenticated` | Từ dễ đoán và lần này **đúng** — nhưng vẫn phải tra: `role` là chính từ luật 5 bất biến 3 dùng trong `(tenant_id, role, permission)`, nên hợp đồng và mô hình phân quyền nói cùng một từ cho cùng một thứ. `org-units` ngay trên là ví dụ từ dễ đoán sai |
| **Phân quyền** (quan hệ vai trò ↔ quyền) | `quyen` + `vai_tro_quyen` — **hai bảng**, xem ô bên phải | `role-permissions` — `GET /api/v1/role-permissions` | **KHÔNG phải `permissions`**, và lý do sẽ bị hỏi lại nên ghi ra: phản hồi mang **cả ba** thứ — danh mục quyền, danh sách vai trò của xã, và các ô đã cấp — mà **trọng tâm là QUAN HỆ**, không phải danh mục. `permissions` mô tả thiếu hai phần ba phản hồi, đồng thời **chiếm mất cái tên** mà một tuyến danh mục quyền thuần tuý sau này đáng được dùng. Cùng phép thử ADR 0017: gọi theo thứ dữ liệu **LÀ**.<br><br>**HAI BẢNG, và nửa thứ hai là nửa dễ đọc nhầm nhất.** `quyen` là **danh mục khoá quyền của PHẦN MỀM** — giống hệt nhau ở mọi xã, và vì thế **không có `tenant_id`**. `vai_tro_quyen` mới là thứ theo xã: xã nào cấp quyền nào cho vai trò nào. Không bảng nào một mình mang khái niệm này.<br><br>Câu về `quyen` phải nằm ở đây chứ không chỉ trong migration, vì người mở bảng ấy lần đầu sẽ thấy một bảng nghiệp vụ thiếu `tenant_id` và kết luận đó là **lỗ hổng luật 1** — rồi "sửa" bằng cách thêm cột và seed theo từng xã. Lúc đó 33 khoá nhân 200+ xã, và `vai_tro_quyen.quyen_ma REFERENCES quyen(ma)` vỡ. Bảng này là chỗ họ tra **trước** khi mở migration, nên là chỗ rẻ nhất để chặn. Điểm neo: `service-identity/internal/http/quyen.go` |
| Thông báo | `thong_bao` | `announcements` · `public-notices` · `notifications` | **Một từ tiếng Việt, ba thứ khác nhau, ba nhóm người đọc.** Gộp một danh từ thì thông báo nội bộ chạy sang kênh công dân |
| Tiếp nhận | `tiep_nhan` | `receipt` | |
| Thụ lý | `thu_ly` | `admission` | Thụ lý **bắt đầu đồng hồ luật định**; tiếp nhận thì không. Gọi cả hai là `accept` là xoá mất ranh giới đó |
| Nghiệm thu | `nghiem_thu` | `verification` | Với phiếu phản ánh đây là kiểm tra thực địa kèm ảnh, không phải nghiệm thu công trình có hội đồng (`acceptance`) |
| Đóng phiếu | `dong_phieu` | `closure` | |
| Cấp số văn bản | `so_di` | `number` | |
| **Cán bộ** | `nguoi_dung` (bảng) · `can_bo` (nghiệp vụ) | `staff` — `GET /api/v1/staff` · `GET …/{id}`, quyền `admin.user`. **`web-admin` đã gọi cả hai tuyến** — `web-admin/src/lib/api/can-bo.ts` | `user` thì trùng với công dân — hai lớp tin cậy khác hẳn nhau (luật 4). Xem mục dưới: khái niệm này mang **bốn** cái tên |
| **Danh bạ chọn người nhận việc** | `nguoi_dung` (cùng bảng) | `staff-directory` — `GET /api/v1/staff-directory`, `AnyAuthenticated` (người dùng duyệt 24/09/2026) | Danh từ do **người dùng** chốt 24/09/2026. **Tài nguyên RIÊNG, không phải `staff?view=…`**, vì khác `staff` ở cả ba chỗ: **hợp đồng** (chỉ `code` · `full_name` · `position` · `department_id` — không `id`, không số điện thoại, không email, không cờ tài khoản), **quyền** (mọi tài khoản của xã, còn `staff` là `admin.user`) và **ai được liệt kê** (chỉ người có tài khoản đang hoạt động, chưa xoá — người giao việc được; `staff` liệt kê cả sổ). Một đường dẫn mang hai mức quyền theo tham số là đường dẫn mà mức bảo vệ do client chọn. Dùng chung cho ô phân công của Phản ánh, Nhiệm vụ, Thông báo, Văn bản & đơn thư. Điểm neo: `service-identity/internal/http/danh_ba_chon_nguoi.go` |
| **Xã của yêu cầu này** | `tenant` (bảng, service `platform`) | `communes/current` — `GET /api/v1/communes/current`, công khai | Cho **cán bộ**, suy từ `Host` ở rìa (luật 1 bất biến 3), trả `thongTinXa` cho màn đăng nhập. Số nhiều **dù chỉ trả về một xã** — xem ngay dưới |
| **Phiếu phản ánh CỦA CHÍNH NGƯỜI GỬI** | `phan_anh` (cùng bảng `phieu_phan_anh`) | `my-citizen-reports` — `GET /api/v1/my-citizen-reports/{maTraCuu}` | Cùng dữ liệu, **khác bề mặt**: đọc bởi công dân, lọc theo danh tính phiên. Không thể dùng lại `citizen-reports` vì tuyến cán bộ đã chiếm đúng đường dẫn ấy — xem §Tiền tố `my-` |
| **Khoá tài khoản** (nghỉ hưu, chuyển công tác) | `dang_hoat_dong = false` | `lockout` — `POST`/`DELETE /api/v1/staff/{id}/lockout` | **KHÔNG phải `disable`, `deactivate`, `suspend`.** Đây là một TRẠNG THÁI có thể mở lại, nên nó là một **tài nguyên con** mà `POST` tạo và `DELETE` gỡ — không phải một động từ trong đường dẫn. Khoá ≠ xoá (#10): người bị khoá VẪN còn trong danh bạ và vẫn hiện trên mọi hồ sơ cũ |
| **Tài khoản đăng nhập của một cán bộ** | `co_tai_khoan` · `mat_khau_hash` | `account` — `POST /api/v1/staff/{id}/account` | Một dòng danh bạ **KHÔNG tự động là một tài khoản**: xã nhập cả người chưa cần đăng nhập. `account` là thứ CẤP THÊM cho một dòng đã có, nên nó là tài nguyên con của `staff`. Bác `login-account` (dài mà không thêm nghĩa), `credentials` (số nhiều mơ hồ, lẫn với cặp tên/mật khẩu), `sign-in` (động từ — `rest_api_guard` chặn) |
| **Mật khẩu** | `mat_khau_hash` (CSDL chỉ giữ chuỗi băm) | `password` — `PUT /api/v1/staff/{id}/password` · `PUT /api/v1/staff/current/password` | Giữ `password` chứ không `credential`/`passphrase`: đây là **đúng một chuỗi bí mật**, và `credential` trong hệ này còn gồm phiên và `sid`. `current` là của máy chủ lấy từ phiên — **không** có `id` nào trên tuyến tự đổi, vì một tham số định danh ở đó là đổi mật khẩu người khác (luật 4 cấm #1) |
| **Biên bản họp** | `bien_ban_hop` · **`@entity: Meeting`** | `meetings` — `GET /api/v1/meetings`, quyền `task.read` (`petitions`) | **KHÔNG tự dịch — lấy theo BẰNG CHỨNG, đúng điều ADR 0011 đòi.** Đặc tả `docs/ui-ux/04-bien-ban-hop.md` §6 đề nghị `/api/bien-ban`, mà `hooks/rest_api_guard.py` **chặn** danh từ tiếng Việt trên URL, nên đường ấy không ship được. `meetings` là chữ **kho anh em đang chạy dùng thật** — `../vigov-require/apps/api/app/modules/tasks/router.py:45`. Không phải `minutes` (là *bản ghi chép*, một trường của biên bản, không phải chính cuộc họp) và không phải `sessions` (đã bị phiên đăng nhập chiếm) |
| **Kết luận họp** | `ket_luan_hop` · **`@entity: Conclusion`** | *(chưa có tuyến riêng — trả lồng trong `meetings`)* | Cùng nguồn bằng chứng: bên kia gọi `conclusions`. **Chưa tách tuyến riêng có chủ ý:** thẻ biên bản §2 luôn hiện kết luận cùng biên bản, nên một tuyến `/conclusions` độc lập hôm nay là một danh từ chưa ai gọi tới. Ngày có tuyến tách (sửa một kết luận) thì dùng đúng chữ ấy |
| **Bài nội dung Mini App** | `noi_dung_mini_app` · **`@entity: ContentItem`** | `content-items` — `GET/POST /api/v1/content-items`, quyền `content.read` / `content.update` (`comms`) | **Lấy theo BẰNG CHỨNG, không tự dịch** — `../vigov-require/apps/api/app/modules/content/router.py:54,66`. Đặc tả `docs/ui-ux/11-noi-dung-mini-app.md` §9 đề nghị `/api/mini-app/noi-dung` (danh từ tiếng Việt — `rest_api_guard` chặn) và `/api/cong/…` (ngoài tiền tố `/api/v1/` mà `tools/ingress` bắt buộc → 404 lúc phát hành); cả hai đường không ship được. **Gạch ngang chứ không lồng `content/items`**: `tools/ingress` gom theo ĐOẠN ĐẦU sau `/api/v1/`, và `content` trần là một *module* chứ không phải tài nguyên — lồng sẽ tạo "một tài nguyên hai chủ" |
| **Danh mục tin Mini App** | `danh_muc_mini_app` · **`@entity: ContentCategory`** | `content-categories` — `GET/POST /api/v1/content-categories` (`comms`) | Cùng nguồn bằng chứng (`router.py:186`). Cây tự tham chiếu `cha_id`; **chưa có tuyến sửa/xoá** có chủ ý — đổi tên thì vô hại, đổi cha thì không: đó là lượt ghi duy nhất tạo được chu trình ≥2 đỉnh mà không `CHECK` nào từ chối được, nên tuyến ấy tới cùng lúc với phép duyệt tổ tiên của nó |
| **Xác thực lời khai cư trú** | `citizen.verify` (khoá quyền) | *(tuyến chưa dựng)* | Khoá quyền chốt 22/09/2026 — ADR 0035 §D. Nhóm `citizen` là **nhóm thứ mười một** mà `14-cau-hinh.md:104` đếm tới nhưng bảng dưới không liệt kê. **Chưa có migration nạp**: viết tuyến thì nạp cùng lượt, nếu không tuyến ấy trả 403 cho mọi tài khoản mãi mãi (luật 5 bất biến 3c) |

**Ba danh từ `lockout` · `account` · `password` chốt 22/09/2026 bởi NHÀ CUNG CẤP, không phải
khách** (ADR 0035). Ghi ra vì ADR 0011 bảo hỏi chứ đừng tự dịch, và ở đây đã tự dịch có chủ ý:
đổi bây giờ còn miễn phí vì **chưa xã nào chạy thật**. Ngày một xã chạy, đổi một đoạn đường dẫn
là đổi thứ `tools/ingress` đã sinh vào manifest và thứ `web-admin` đã gọi.

### Tiền tố `my-` — KHUÔN cho mọi tuyến công dân đọc dữ liệu của chính mình (chốt 22/09/2026)

**Quy tắc:** một tài nguyên đã có bề mặt cán bộ, khi mở cho công dân đọc **phần của chính họ**,
mang một **tài nguyên RIÊNG** tên `my-<tài nguyên cán bộ>`. Không phải một đoạn con
(`citizen-reports/{ma}/…`), không phải một tiền tố chung cho cả kênh (`/api/v1/me/…`).

| Vì sao không dùng lại đường dẫn cán bộ | Vì sao không phải đoạn con | Vì sao không phải `/api/v1/me/…` |
|---|---|---|
| `GET /api/v1/citizen-reports/{maTraCuu}` đã là tuyến cán bộ. Hai lớp tin cậy khác hẳn nhau (luật 4) **không dùng chung handler** — luật 4 bất biến 5 | Một tài nguyên riêng cho bộ sinh Ingress **một luật riêng**: hai bề mặt tách được ở **tầng mạng**, không chỉ ở mux trong tiến trình | `tools/ingress` gom theo **đoạn đầu** sau `/api/v1/` (`dinhtuyen.go` §`taiNguyen`). `me` sẽ do dịch vụ đầu tiên dùng nó **chiếm**, và tuyến công dân của dịch vụ **thứ hai** dưới cùng tiền tố là *"một tài nguyên hai chủ"* — điều kiện dừng của chính bộ sinh ấy |

**`/api/cong/…` (đề xuất ở `docs/ui-ux/09 §13`) KHÔNG dùng được, và lý do là máy chứ không phải
khẩu vị:** `tools/ingress` chốt mọi tuyến REST nằm dưới `/api/v1/` và **dừng hẳn** với đường dẫn
ngoài tiền tố ấy. Viết `/api/cong/` là sinh ra một tuyến **404 lúc phát hành**. Nó cũng là đoạn
đường dẫn tiếng Việt, trái ADR 0011.

Tuyến `my-…` **luôn** khai hai trục trong cùng câu lệnh route (ADR 0022):
`authz.CitizenOnly()` trả lời *ai*, `httpx.XaTuPhien()` trả lời *xã nào*.

### Hai đường dẫn `communes`, hai lớp người dùng — đọc trước khi động vào một trong hai

| Đường dẫn | Trả lời câu | Cho ai | Xã đến từ |
|---|---|---|---|
| `GET /api/v1/communes/current` | *"Yêu cầu này thuộc xã nào"* | **Cán bộ**, trước khi có phiên | `Host` (ADR 0022, đường cán bộ) |
| `GET /api/v1/communes` | *"Có những xã nào để chọn"* | **Công dân** trong Mini App, khi **chưa** có xã nào | không có — lớp `KhongThuocXa` (ADR 0022) |

Hai lớp người dùng có mức tin khác hẳn nhau (luật 4), và **trước 2026-09-17 hai đường dẫn cách
nhau đúng một chữ `s`**: tuyến cán bộ từng là `/api/v1/commune` số ít. Đổi tại `d2d2aa6`, lý do
ở ADR 0023 §B.

**Lập luận đã bị bác, ghi lại để không ai lập luận lại vòng đó:** *"người gọi không bao giờ
thấy quá một xã, nên số ít mới đúng"* — **nhầm chỗ**. Quy tắc số nhiều (`rest-api-design`
REQUIRED #1) đặt tên cho **loại tài nguyên**, không đo kích thước một câu trả lời. `current` là
một **bộ chọn** trên loại ấy, đúng hình dạng `sessions/current` ngay trong bảng trên.

### Kênh công dân — ADR 0023

Sáu khái niệm, chốt cùng một lượt. **Lý do đầy đủ nằm ở ADR 0023**, không chép lại ở đây.

| Khái niệm | Thực thể (`@entity`) | Bảng | Tài nguyên URL | Vì sao không phải từ dễ đoán |
|---|---|---|---|---|
| Danh mục xã cho Mini App | — (đọc `Tenant`) | — | `communes` | `commune` ở đây phủ **cả ba** loại đơn vị cấp xã. Đừng thêm `/wards`: danh mục bị tách đôi trong khi dữ liệu là một. Văn xuôi tiếng Việt gọi là **danh mục xã**, không phải "danh bạ" (danh bạ là danh sách liên hệ) |
| Phiên đăng nhập của **công dân** | `CitizenSession` | `phien_cong_dan` | `citizen-sessions` | `phien` đã là phiên cán bộ (khoá ngoại tới `nguoi_dung`). Một tên chung cho hai lớp tin cậy khác nhau là chỗ luật 4 vỡ mà không ai thấy |
| Mã ghép phiên | `SessionPairing` | `ghep_phien` | `session-pairings` | **Việc ghép**, không phải *một loại phiên* — phiên màn hình cầm chỉ là `CitizenSession` có `nguon = 'ghep'`. `qr_login` sai vì ADR 0019 bất biến 3: quét QR **không** tạo ra danh tính; `kiosk_*` sai vì màn hình là gì thì khách chưa trả lời |
| Quan hệ công dân ↔ xã | `CitizenCommune` | `quan_he_cong_dan_xa` | *(không có tài nguyên riêng)* | Giá trị: `thuong_tru` · `tam_tru` · **`chua_khai`**. Không dùng `vang_lai`: người gửi phản ánh có thể đang ngồi ở tỉnh khác, hệ thống không biết họ ở đâu. Hai sự thật tách riêng — **công dân khai gì** và **xã đã xác thực chưa** |
| Đơn vị cũ → đơn vị kế thừa | `TenantSuccession` | `tenant_succession` | *(không bao giờ là tài nguyên)* | **Không** gọi `alias`: hai xã là hai pháp nhân, và một cái tên nói chúng là một sẽ mời người sau viết `UPDATE tenant_id` trên hồ sơ lưu trữ — `admin-unit-merge` bất biến 2 cấm |
| Định danh công dân | `CitizenIdentity` | `dinh_danh_cong_dan` | *(không bao giờ có danh sách)* | **Không** gọi `cong_dan`: bảng này nằm ở vùng xuyên xã, không có `tenant_id` che chắn. Một cái tên rộng là lời mời thêm `so_cccd`, `ho_ten` vào đúng chỗ Nghị định 13/2023 nặng nhất |

**Bảng này còn thiếu.** Nó mới phủ các khái niệm đã xuất hiện trong mã, trong một ADR đã chốt,
hoặc trong `.claude/skills/rest-api-design/SKILL.md`. Gặp khái niệm chưa có dòng ở đây: **dừng
lại và hỏi**, đừng tự dịch rồi viết route — đường dẫn không sửa lại được sau khi một xã chạy thật.

### Danh mục tham chiếu — ADR 0024

Tám danh mục **đã có bảng và đã có tuyến đọc** (2026-09-20). Tên thực thể chốt từ trước khi có
bảng, vì ADR 0021 đưa `@entity` vào dấu, và dấu thì đã nằm trong chỉ mục — đổi tên bây giờ là sửa
dấu trên khắp các bảng đã tạo **và** đổi một đường dẫn đang chạy.

**Giá trị bên trong các danh mục này giữ tiếng Việt không dấu** (`uy-ban`, `thon`, `doanh-nghiep`)
— ADR 0011. Chỉ *tên thực thể* là tiếng Anh.

| Khái niệm | Thực thể (`@entity`) | Bảng | Tài nguyên URL | Vì sao không phải từ dễ đoán |
|---|---|---|---|---|
| Loại tài nguyên bản đồ | `MapAssetType` | `loai_tai_nguyen_ban_do` | `map-asset-types` — `GET /api/v1/map-asset-types`, `AnyAuthenticated` (`comms`) | **`asset` đã là từ đã dùng, không phải lựa chọn mới**: quyền `asset.read` / `asset.update` chốt tại `docs/ui-ux/10-ban-do-kinh-te-so.md:255`. Chọn `resource` hay `poi` ở đây là để một khái niệm mang hai từ tiếng Anh trên hai bề mặt — đúng cái giá mà mục `feedback.*` đang trả |
| Hạng mục kế hoạch vốn | `CapitalPlanCategory` | `hang_muc_ke_hoach_von` | `capital-plan-categories` — `GET /api/v1/capital-plan-categories`, `AnyAuthenticated` (`finance`) | `Category` chứ không `Item`: một *hạng mục* ở đây phân loại các khoản trong kế hoạch, không phải một dòng tiền cụ thể. `Item` sẽ mời người sau gắn số tiền vào chính bảng danh mục |
| Loại văn bản | `DocumentType` | `loai_van_ban` | `document-types` — `GET /api/v1/document-types`, `AnyAuthenticated` (`documents`) | Phân loại **văn bản nói chung**, dùng chung cho cả `van_ban_den` và `van_ban_di`. Đừng tách `IncomingDocumentType` — đến/đi là **hướng**, không phải loại |
| Thôn / Tổ dân phố | `ResidentialUnit` | `thon_to_dan_pho` | `residential-units` — `GET /api/v1/residential-units`, `AnyAuthenticated` (`identity`). Trả `name`, **không** `label` — xem ghi chú dưới bảng | Một tên phủ **cả hai** loại, vì chúng là cùng một thứ ở hai địa bàn: thôn ở nông thôn, tổ dân phố ở đô thị. Đừng gọi `Hamlet` (chỉ đúng nửa) hay `Village` (sai với phường) |
| Loại đơn vị dân cư | `ResidentialUnitType` | `loai_don_vi_dan_cu` | `residential-unit-types` — `GET /api/v1/residential-unit-types`, `AnyAuthenticated` (`identity`) | Đi kèm `ResidentialUnit`, cùng một hậu tố `…Type` với ba danh mục trên. ⚠ **Nếu hai giá trị `thon` / `to-dan-pho` là cố định theo luật thì đây nên là cột enum, không nên là bảng danh mục sửa được** — xem câu hỏi mở #21: một danh mục sửa được là một danh mục xã tắt được |
| Khối nhiệm vụ | `TaskBloc` | `khoi_nhiem_vu` | `task-blocs` — `GET /api/v1/task-blocs`, `AnyAuthenticated` — **service `identity`**, đúng như ô bên phải | Giá trị là `Khối Uỷ ban` · `Khối Đảng` · `Khác` (`02-nhiem-vu.md:56, 249`) — tức **tuyến bộ máy**, chính quyền hay Đảng, chứ không phải nhóm công việc. **Không** dùng `TaskBranch`: `branch` là từ tiếng Anh tự nhiên của `bo_phan`, và dùng nó ở đây là đặt hai khái niệm dưới một từ. **Không** dùng `TaskSector`: khối không phải lĩnh vực chuyên môn. ⚠ **BẢNG NÀY THUỘC `identity` DÙ TÊN MANG CHỮ `Task` — đừng "sửa cho gọn".** Tên đi theo KHÁI NIỆM (đặc tả chỉ cho khối xuất hiện như thuộc tính của nhiệm vụ: bộ lọc `:56`, trường biểu mẫu `:249`, nhãn dòng `:121`; `bo_phan` không có trường khối — `14-cau-hinh.md:38` và `identity/migrations/0001_init.sql:66`). Quyền sở hữu đi theo NHỊP ĐỔI: khối đổi cùng sơ đồ tổ chức, không cùng nhiệm vụ (ADR 0024:65). Hai thứ ấy được phép khác nhau. Kéo bảng sang `petitions` cho "khớp tên" là dựng lại đúng vòng hai đỉnh mà ADR 0024:130 đã cấm bằng tên |
| Loại nhiệm vụ | `TaskType` | `loai_nhiem_vu` | `task-types` — `GET /api/v1/task-types`, `AnyAuthenticated` (`petitions`) | `Type` chứ không `Kind`: bốn danh mục trên đã dùng hậu tố `…Type`, và hai hậu tố cho cùng một vai trò là thứ người sau phải tra mới biết dùng cái nào |
| Mức ưu tiên nhiệm vụ | `TaskPriority` | `muc_uu_tien_nhiem_vu` | `task-priorities` — `GET /api/v1/task-priorities`, `AnyAuthenticated` (`petitions`) | Không hậu tố `…Type`: đây là **thang độ** có thứ tự, không phải một phân loại ngang hàng. Thứ tự là thuộc tính có nghĩa của nó |

**Cột tài nguyên URL đã điền — cả tám tuyến ĐÃ CHẠY** (2026-09-20). Đây là **ghi lại sự thật**,
không phải đoán trước: mỗi ô đối chiếu với `kb/20-contracts/openapi.json`, tầng SINH ra từ mã
(ADR 0014). Cả tám đều là một tuyến `GET` đọc, đều khai `AnyAuthenticated` theo đúng lập luận đã
được chấp nhận cho `GET /api/v1/org-units`: nhãn danh mục xuất hiện ở ô chọn và bộ lọc của hầu
hết màn hình, nên đòi một quyền cấu hình là làm rỗng những ô ấy cho mọi tài khoản không phải
quản trị. Danh mục chỉ lộ **trong chính xã đó** — `Scoped` buộc `tenant_id` (luật 1). Luật 5 điều
kiện dừng #1 đã hỏi và **người dùng đã trả lời**; đừng mở lại vòng đó.

**Phần đúng của lý do cũ vẫn giữ nguyên, vì nó nói về NHỊP chứ không nói về trạng thái:** đường
dẫn được chốt lúc **có người gọi thật sự cần**, không phải lúc tạo bảng — và **một khi một xã đã
chạy thật thì đường dẫn không lấy lại được**. Hệ quả cho dòng sau này: khái niệm nào chưa có
tuyến thì **vẫn để trống**, đừng điền sẵn. Hệ quả cho tám dòng này: chúng không còn "chưa chốt".
Ai đọc lướt thấy hai chữ ấy rồi **nghĩ ra cái tên thứ chín** cho một thứ đã có đường dẫn đang
chạy là đúng sự cố mà cột này vừa được điền để chặn.

**Chuỗi người đọc trong hợp đồng là `label`, KHÔNG phải `name` — ranh giới vạch ở SCHEMA, không
ở tên tuyến.** Bảng danh mục mang cột `nhan`, tức một **NHÃN**: sửa lại chữ là thao tác **duy
nhất mà cả ba tầng của ADR 0024 đều cho phép** xã làm — ở tầng 3 thì nó là thứ duy nhất còn lại —
trong khi `ma` vẫn bất biến. Bảng ba tầng ở
`service-identity/migrations/0005_don_vi_dan_cu_va_danh_muc.sql:79-81`. Vì thế **bảy** danh mục
trả `label`. `bo_phan` và `vai_tro` mang cột `ten`, tức một **TÊN RIÊNG**, nên `org-units` và
`roles` trả `name`. `thon_to_dan_pho` cũng mang `ten` — nó là một địa bàn có tên, không phải một
dòng danh mục — nên `/residential-units` trả `name` trong khi `/residential-unit-types` ngay cạnh
trả `label`. Hai tuyến cạnh nhau, hai tên trường khác nhau, và đó là **cố ý**. Câu này nằm ở đây
vì đây là chỗ người viết route tra **trước** khi đặt tên trường: hôm 2026-09-20 bốn service mỗi
nơi tự nghĩ ra một đáp án, và cái giá là bốn lần đổi tên trên bề mặt hợp đồng.

**Câu đã hỏi, ĐÃ CÓ TRẢ LỜI — ghi lại chứ không xoá, để không ai hỏi lại vòng ba.** `TaskBloc`
thuộc `identity` còn `TaskType` thuộc `petitions`, dù cả hai đều phân loại `nhiem_vu`. Đây
**không** phải một khái niệm bị chẻ đôi còn bỏ ngỏ: ADR 0024:57-67 chốt chủ sở hữu cho cả bảy
nhóm, và dòng `:65` chốt khối thuộc `identity` vì khối đổi **cùng nhịp với sơ đồ tổ chức**, không
cùng nhịp với nhiệm vụ. Đúng quy tắc ở §Quy ước đặt tên: tên đi theo khái niệm, quyền sở hữu đi
theo nhịp đổi, và hai thứ ấy **được phép khác nhau**. Lý do đầy đủ ở ô `TaskBloc` trong bảng trên
và ở ADR 0024 §"Cái giá của dòng `Khối nhiệm vụ`" (`:115-136`) — không chép lại ở đây.

Mã đã đi theo quyết định đó, nên đừng đọc chỗ lệch này như một lỗi: dấu `-- @entity: TaskBloc`
nằm ở `service-identity/migrations/0005_don_vi_dan_cu_va_danh_muc.sql:295`, và
`GET /api/v1/task-blocs` chạy từ chính `identity`. **Nếu cái giá của tham chiếu xuyên service hoá
ra đắt hơn mức chịu được, đường ra KHÔNG phải là kéo bảng sang `petitions` trong im lặng — đó là
sửa ADR** (ADR 0024:135-136).

**Còn một chỗ chưa chốt, đã nêu với người viết migration:** `10-ban-do-kinh-te-so.md:53` nói danh
mục có **8 mục** trong khi `:37` nói **11 nhóm** — đặc tả lệch với chính nó, phải chốt trước khi seed.

### Lịch làm việc của xã — ADR 0007

Ba khái niệm, **người dùng chốt tên cùng một lượt 2026-09-20**. Cả ba thuộc **`service-identity`**,
cũng chốt hôm ấy: ADR 0007 đếm hạn cho **`Văn bản đến`** chứ không riêng `Phản ánh`, nên lịch
được ít nhất hai service đọc và **không thuộc service nào trong hai**; ADR 0024 chốt quyền sở hữu
đi theo **nhịp đổi**, mà giờ làm việc của xã đổi cùng bộ máy hành chính. Lập luận đầy đủ ở
`service-identity/migrations/0006_lich_lam_viec.sql:14-20` — không chép lại ở đây.

**Bảng `sla` đi theo ba bảng này, chốt 20/09/2026 — ADR 0029.** Cùng một lập luận, áp cho một
bảng khác: số giờ SLA phủ cả `van-ban-den` (`documents`) lẫn `phan-anh` / `nhiem-vu`
(`petitions`), nên nó **không thuộc service nào trong hai**, và nó đổi cùng nhịp với chính sách
hành chính của xã — cùng nhịp với lịch. **Chưa có tài nguyên URL**, vì chưa có tuyến nào: đừng
điền sẵn một cái tên (§Danh mục tham chiếu). Bảng này **chưa tồn tại trong kho** và đang chặn
mọi tuyến ghi của `petitions` lẫn `documents` — hệ quả và điều kiện dừng ở ADR 0029.

| Khái niệm | Thực thể (`@entity`) | Bảng | Tài nguyên URL | Vì sao không phải từ dễ đoán |
|---|---|---|---|---|
| Tuần làm việc của xã | `WorkingHours` | `lich_lam_viec` | `working-hours` | **Không phải `working-sessions`, dù một dòng đúng LÀ một ca chứ không phải một ngày.** `sessions` đã mang nghĩa phiên đăng nhập **cán bộ** (`/api/v1/sessions`), và `citizen-sessions` đã mang nghĩa lớp tin cậy còn lại (ADR 0023). Một nghĩa thứ ba cách đó **đúng một đoạn đường dẫn** là dựng lại ca `commune`/`communes` mà ADR 0023 §B phải tốn một lần đổi đường dẫn đang chạy mới gỡ xong. Đã bác thêm: `office-hours` (trôi khỏi dấu `@entity: WorkingHours`, tức hợp đồng và migration gọi một thứ bằng hai từ), `working-calendar` và `work-schedule` (số ít — trái quy ước; và chữ "calendar" mời người sau đổ **sự kiện** vào bảng này) |
| Ngày cơ quan đóng cửa | `PublicHoliday` | `ngay_nghi_le` | `public-holidays` — **đọc hết ô bên phải trước khi định "sửa" cái tên này** | **CẢNH BÁO LÀ MỘT PHẦN CỦA DÒNG, không phải ghi chú thêm.** Bảng còn giữ **lễ hội địa phương và ngày truyền thống** (ADR 0007 quyết định 4), những ngày **không** phải ngày lễ theo nghĩa quốc gia. Điều mọi dòng thật sự khẳng định là *"ngày đó cơ quan đóng cửa"*, nên `closure-days` là danh từ **đúng nghĩa hơn**. Đã nêu **đúng lập luận ấy** với người dùng, và **người dùng giữ `public-holidays`**: nó khớp dấu `-- @entity: PublicHoliday` đã viết sẵn ở `service-identity/migrations/0006_lich_lam_viec.sql:192`, mà đổi tên về sau là **sửa dấu trên một migration ĐÃ ÁP** — `core/migrate` so checksum mỗi lần khởi động, nên sửa tệp đã áp thì service dừng (`ErrChecksumLech`, ghi ở chính tệp ấy `:3-6`). Ghi **cả lập luận lẫn quyết định**: một dòng giấu lập luận là một dòng mời người sau "sửa cho đúng" |
| Ngày làm bù | `SwapWorkingDay` | `ngay_lam_bu` | `swap-working-days` | **Không** `makeup-days`: thành ngữ Mỹ, người tích hợp không đọc tiếng Anh Mỹ phải đoán. **Không** `compensatory-working-days`: trong tiếng Anh nhân sự nó đọc ra **ngày nghỉ bù**, tức ngược hẳn nghĩa — đây là ngày **LÀM**. **Không** `extra-working-days`: mất luôn lý do bảng tồn tại — ngày làm bù là ngày **trả lại** một đợt nghỉ dài theo **thông báo hằng năm của Thủ tướng**, không phải ngày làm thêm của cơ quan |

**HAI CÁI TÊN NÓI VỀ MỘT NGÀY HOẶC MỘT KHOẢNG GIỜ, TRONG KHI MỘT DÒNG LÀ MỘT CA — trên hai
trong ba bảng. Chỗ lệch ấy có thật và là cố ý.** `lich_lam_viec` và `ngay_lam_bu` đều lưu **một
dòng cho một ca làm việc**: nghỉ trưa là **hai dòng có khoảng hở**, ca trực thứ Bảy là **một
dòng**, ngày không làm việc thì **không có dòng nào**. Migration lập luận dài vì sao hình dạng
**ca** mới đúng — `service-identity/migrations/0006_lich_lam_viec.sql:83-96` và `:251-252`. Chỉ
`ngay_nghi_le` mới đúng một dòng một ngày. Tên kế thừa chỗ lệch đó, và câu này nằm ở đây để
**không ai đọc `working-hours` rồi chờ một dòng cho mỗi thứ trong tuần** — rồi sửa schema cho
khớp cái tên.

**CẢ BA TUYẾN ĐÃ CHẠY** (2026-09-20), đều là tuyến `GET` đọc và đều khai `AnyAuthenticated` —
cùng lập luận đã chấp nhận cho `/org-units` và cho tám danh mục ở trên: giờ làm việc và ngày nghỉ
của xã nằm dưới **mọi hạn xử lý hiện trên màn hình**, nên đòi một quyền cấu hình không bảo vệ
được gì mà làm rỗng những màn hình ấy cho mọi tài khoản không phải quản trị. Danh mục chỉ lộ
**trong chính xã đó** — `Scoped` buộc `tenant_id` (luật 1). **Chưa có tuyến GHI nào**: ai được sửa
lịch của xã thì chưa ai hỏi khách, và một đường ghi viết dở trông như một quyết định đã có người
ra. Ba ô trên đối chiếu với `kb/20-contracts/openapi.json`, tầng **SINH** ra từ mã (ADR 0014) —
đó là sự thật, bảng này chỉ giữ **danh từ đã chốt** (đoạn mở đầu §Tên tài nguyên trên URL). Lệch
một chữ về sau thì **sửa mã**, vì tên là **người dùng chốt** chứ không phải một phiên tự dịch.

Điều đó **không trái** câu *"khái niệm nào chưa có tuyến thì vẫn để trống, đừng điền sẵn"* ở
§Danh mục tham chiếu: câu ấy cấm **tự nghĩ ra tên khi chưa ai cần gọi**. Ở đây ngược lại — mã
**dừng lại và hỏi** vì bảng này chưa có dòng (đúng như `kb/INDEX.yaml` `not_here` dặn), người
dùng trả lời, rồi route mới được viết. Trước lúc đó các phép thử treo tạm ba handler dưới
`/api/v1/test-probe/...` với những chữ như `working-calendar`, `closure-days` — **khung thử,
không bao giờ là đề xuất**. Gặp lại những chữ ấy trong mã hay trong `git log`: chúng là **tên đã
bị bác**, không phải tên đang tranh chấp với ba ô trên.

### Một khái niệm, bốn cái tên: `nguoi_dung` · `can_bo` · `CanBo` · `Staff`

Ghi lại ở đây vì lệch **đã có thật trong mã**, và vì người đọc gặp cái tên thứ hai sẽ tưởng
mình gặp hai khái niệm. Chỉ có **một** khái niệm: con người làm việc trong cơ quan xã.

| Bề mặt | Tên đang dùng | Vì sao là tên đó |
|---|---|---|
| Bảng CSDL | `nguoi_dung` | `identity/migrations/0001_init.sql`. Bảng gộp **hai tập**: 26 người của danh bạ công khai và những người có tài khoản đăng nhập — phân biệt bằng cột `co_tai_khoan` (migration `0003`) |
| Ngôn ngữ nghiệp vụ | `can_bo` | Từ hành chính đúng cho con người; "người dùng" là từ của phần mềm, không phải của xã |
| Kiểu Go | `domain.CanBo`, `store.CanBoStore` | Tầng nghiệp vụ nói tiếng nghiệp vụ |
| Hợp đồng gRPC · REST | `Staff`, `BatchGetStaff` · `/api/v1/staff` | Bề mặt hợp đồng dùng **tiếng Anh** (ADR 0011). Hai tuyến đọc đã chạy, quyền `admin.user`, và **`web-admin` đã gọi cả hai** — `web-admin/src/lib/api/can-bo.ts` |

**Vì sao không thống nhất lại thành một từ** — cùng dạng lập luận với `feedback.*` ở mục dưới:
giá đổi tên khác nhau ở từng bề mặt. Tên bảng đã có migration đã áp và có hồ sơ lưu trữ trỏ
vào; kiểu Go đổi rẻ nhưng đổi thì lệch với bảng; `Staff` là bề mặt bên ngoài đọc và đã là lựa
chọn có lý do. Bề mặt URL **từng là bề mặt duy nhất còn miễn phí** — nay không còn: route đã
chạy **và đã có bên gọi**, nên `staff` là thứ phải giữ, không phải thứ còn cân nhắc.

**Điều KHÔNG được suy ra từ mục này:** rằng bốn tên cho một khái niệm là chuyện bình thường
nên cái thứ năm cũng được. Bốn cái tên này là **giá đã trả rồi**, không phải giấy phép. Khái
niệm mới thì đặt **một** tên và giữ nguyên qua các tầng.

### Khoá quyền `feedback.*` lệch với tài nguyên `citizen-reports` — cố ý

Đây là chỗ duy nhất ghi lệch này. Nơi khác **liên kết tới đây**, không chép lại.

| Bề mặt | Dùng | Trạng thái |
|---|---|---|
| Khoá quyền | `feedback.assign` `feedback.create` `feedback.read` `feedback.resolve` `feedback.restricted` — cộng **hai khoá chốt 20/09/2026**: `feedback.classify` `feedback.unmask` | Năm khoá đầu nạp ở `service-identity/migrations/0001_init.sql`, hai khoá sau ở `service-identity/migrations/0007_quyen_phan_loai_va_xem_day_du.sql`; ADR 0008 đã chốt `feedback.resolve` quyết định ai đóng phiếu, ADR 0030 chốt hai khoá mới |
| Tài nguyên URL | `citizen-reports` | ADR 0011 |

**Hai khoá mới KHÔNG suy ra được từ năm khoá cũ, và đó là điểm chính** (luật 5 bất biến 3b):
`feedback.classify` là hành vi **ấn định hạn** cho dân chứ không phải một dạng `assign`, và
`feedback.unmask` **gỡ che** họ tên + số điện thoại người gửi ở **mọi** lĩnh vực, khác hẳn
`feedback.restricted` vốn là phạm vi **nội dung** (lĩnh vực `can-bo`). Lý do đầy đủ, quy ước
đặt tên khoá (`nhóm.mộttừ` — vì sao `unmask` chứ không `view_full`), và ràng buộc ghi vết mỗi
lần đọc đầy đủ: **ADR 0030** — không chép lại ở đây.

**Vì sao không thống nhất lại thành một từ:** hai bề mặt này có **giá đổi tên khác hẳn nhau**.
Khoá quyền đã nằm trong migration và trong bảng `quyen`, và đã được một ADR đã chốt viện dẫn —
đổi nó là một migration trên dữ liệu phân quyền đang chạy, để đổi lấy sự dễ chịu khi đọc. Còn
đường dẫn URL thì chưa có route nào, nên đặt đúng ngay bây giờ **không tốn gì**.

Sai lầm cần tránh là suy ra rằng `feedback` được phép dùng trở lại: **không**. Khoá quyền là
một chuỗi định danh nội bộ mà chỉ cán bộ của chính hệ thống này thấy; đường dẫn URL là thứ bên
tích hợp đọc. `feedback` chỉ được giữ ở đúng nơi nó đã có mặt, không lan sang chỗ mới.

## Quy ước đặt tên

| Loại | Quy ước | Ví dụ |
|---|---|---|
| Bảng, cột | `snake_case` **tiếng Việt** không dấu, theo nghiệp vụ | `don_thu`, `ngay_tiep_nhan` |
| Kiểu Go | `PascalCase`, tiếng Anh nếu là khái niệm kỹ thuật | `DonThu`, `TenantID` |
| Service, proto package, import path | **tiếng Anh** | `petitions`, `vigov.identity.v1` |
| Đoạn đường dẫn URL | **tiếng Anh**, số nhiều, kebab-case | `/api/v1/citizen-reports` |
| Giá trị enum | **tiếng Việt không dấu** | `?type=khieu-nai` |

**Tên sự kiện không thuộc bảng này.** `kb/00-foundation/domain-boundaries.md` sở hữu quy tắc
đặt tên định danh máy đọc, tên sự kiện nằm trong đó. Lý do chọn dạng tiếng Anh số nhiều:
ADR 0011.

**Tên service dùng tiếng Anh, tên bảng dùng tiếng Việt** — chốt 2026-09-16, ADR 0001.
Hai tầng khác nhau: service là khái niệm kỹ thuật và là đường dẫn import, còn bảng và cột
mang khái niệm nghiệp vụ hành chính.

**Không trộn tiếng Anh vào khái niệm nghiệp vụ.** `feedback` không phải `phan_anh`:
"feedback" gợi ý góp ý sản phẩm, "phản ánh" là một loại đơn có quy trình hành chính. Ngoại lệ
duy nhất đã biết là khoá quyền `feedback.*` — xem mục trên, và đừng mở rộng nó.

**TÊN đi theo KHÁI NIỆM, QUYỀN SỞ HỮU đi theo NHỊP ĐỔI. Hai thứ ấy được phép khác nhau.**

Một thực thể được đặt tên theo **thứ nó là**, đọc từ chỗ khái niệm xuất hiện trong nghiệp vụ.
Service giữ nó thì chọn theo **thứ gì làm nó đổi** — cái gì đổi cùng nhịp thì ở cùng chỗ. Hai
câu hỏi ấy có hai câu trả lời, và ép chúng trùng nhau làm hỏng một trong hai.

Ca đã gặp: `TaskBloc` nằm ở `identity`. Tên mang chữ `Task` vì đặc tả chỉ cho khối xuất hiện
như thuộc tính của nhiệm vụ; service là `identity` vì khối đổi cùng sơ đồ tổ chức, không cùng
nhiệm vụ (ADR 0024 §65). Đổi tên cho khớp service thì tên nói sai khái niệm; kéo bảng cho khớp
tên thì dựng lại đúng vòng phụ thuộc hai đỉnh mà ADR 0024 §130 đã cấm.

**Hệ quả bắt buộc:** khi hai thứ lệch nhau, dòng bảng ánh xạ phải **nói ra chỗ lệch và lý do**.
Người sau mở migration thấy một cái tên lạc chỗ sẽ sửa một trong hai cho khớp cái kia — điều
quyết định là lúc ấy họ đọc được lý do, hay chỉ thấy một cái tên trông như lỗi.

**Tên cột trong `docs/ui-ux/` không phải cam kết.** Đó là sản phẩm của prototype một xã. Khi
tên của đặc tả gây nhầm lẫn thì đổi được, nhưng phải ghi lý do ngay tại migration — ví dụ
`co_tai_khoan` thay cho `tai_khoan_hoat_dong` của đặc tả, vì từ sau chỉ khác `dang_hoat_dong`
vài chữ cái mà nghĩa khác hẳn.

→ Kỹ năng: `skills/administrative-language` · `.claude/skills/rest-api-design/SKILL.md`
→ Ranh giới ngôn ngữ của hợp đồng: `kb/10-decisions/0011-contract-surface-language.md`
→ Lý do của sáu dòng kênh công dân và của `communes/current`: `kb/10-decisions/0023-thuat-ngu-kenh-cong-dan.md`
→ Vì sao `linh_vuc` không còn là "cấu hình theo xã": `kb/10-decisions/0026-linh-vuc-phan-anh-hai-tang.md`
→ Vì sao danh sách trạng thái phiếu đóng, và hai đồng hồ đếm từ đâu: `kb/10-decisions/0027-trang-thai-va-dong-ho-phieu-phan-anh.md`
