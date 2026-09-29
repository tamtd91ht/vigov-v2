---
id: ubiquitous-language
tier: T0
source: CURATED
owner: domain
derived_from_commit: 4882826
expires: null
owns_facts:
  - "ánh xạ thuật ngữ hành chính sang tên dùng trong mã"
  - "từ điển đổi tên Việt → Anh cho bảng, cột và gốc định danh (ADR 0061), và danh sách xung đột tên chờ chủ dự án chọn"
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

> **29/09/2026 → ADR 0061:** chủ dự án chốt đổi cả giá trị enum sang tiếng Anh. Với **ba từ này**
> thì chưa: chúng chưa nằm trong bảng nào, và từ tiếng Anh cho chúng là câu pháp lý chưa ai trả lời
> — §Từ điển đổi tên, xung đột **X19**. Lý do phân biệt ba thủ tục ở bảng trên vẫn nguyên.

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
| Phân loại | `phan_loai` | Cán bộ xác định lĩnh vực — **không** để dân tự chọn (câu mở #23 đã đóng theo hướng này, ADR 0028). *28/09/2026 → ADR 0050: kênh Mini App cho dân chọn, lĩnh vực ấy là của phiếu và `han_xu_ly_xong` đặt lúc tạo phiếu; cán bộ vẫn đổi được ở bước này.* Cũng là **hành vi ấn định `han_xu_ly_xong`** cho phiếu dân tự gửi |
| Phân công | `phan_cong` | Giao cán bộ/đơn vị xử lý |
| Nghiệm thu | `nghiem_thu` | Xác nhận đã xử lý trên thực địa, thường kèm ảnh |
| Đóng phiếu | `dong_phieu` | Kết thúc — **bắt buộc có kết quả dân đọc được** |
| Hạn tiếp nhận | `han_tiep_nhan` | Hạn **có cán bộ đọc phiếu**. Đặt lúc **sinh phiếu**, lấy dòng mặc định của bảng SLA. **Cho phép `NULL`, nghĩa là "KHÔNG ÁP DỤNG"** — phiếu nhập hộ, vì chính cán bộ là người vào sổ. `NULL` ở đây **không** phải "chưa có", và báo cáo phải **loại** các dòng ấy, tuyệt đối không đọc thành 0 giờ (ADR 0028) |
| Hạn xử lý xong | `han_xu_ly_xong` | Hạn **xử lý xong**, lấy theo lĩnh vực. Với phiếu dân tự gửi thì đặt lúc **chốt lĩnh vực**, nên **`NULL` trong khoảng chờ phân loại — nghĩa là "CHƯA CÓ"**. Với phiếu nhập hộ thì đặt ngay lúc vào sổ (ADR 0028). *28/09/2026 → ADR 0050: phiếu gửi qua Mini App đặt ngay lúc tạo phiếu, từ lĩnh vực dân chọn — không có khoảng `NULL` chờ phân loại.* |
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

> **29/09/2026 → ADR 0061:** chín mã dưới đây nằm trong đợt đổi tên; mã tiếng Anh đề xuất ở ADR
> 0061 §Ánh xạ giá trị enum. **Chưa đổi** cho tới khi chủ dự án chốt xung đột **X20** (khách đã
> duyệt nguyên văn chín chuỗi này — đoạn cuối mục này). Nhãn tiếng Việt không đổi.

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
| Thông báo | `thong_bao` | `announcements` · `public-notices` · `notifications` | **Một từ tiếng Việt, ba thứ khác nhau, ba nhóm người đọc.** Gộp một danh từ thì thông báo nội bộ chạy sang kênh công dân. `notifications` là **hộp chuông** của chính cán bộ — bảng `staff_notification`, không phải `hop_thu_thong_bao` của đặc tả (`service-comms/migrations/0010_staff_notification.sql:10-12`) |
| Tiếp nhận | `tiep_nhan` | `receipt` | |
| Thụ lý | `thu_ly` | `admission` | Thụ lý **bắt đầu đồng hồ luật định**; tiếp nhận thì không. Gọi cả hai là `accept` là xoá mất ranh giới đó |
| Nghiệm thu | `nghiem_thu` | `verification` | Với phiếu phản ánh đây là kiểm tra thực địa kèm ảnh, không phải nghiệm thu công trình có hội đồng (`acceptance`) |
| Đóng phiếu | `dong_phieu` | `closure` | |
| **Không tiếp nhận** (phiếu phản ánh — nhánh kết thúc) | `khong-tiep-nhan` (trạng thái) · `ly_do_ket_thuc_nhanh` | `rejection` — `POST /api/v1/citizen-reports/{maTraCuu}/rejection`, quyền `feedback.classify` | Danh từ do **người dùng** chốt 25/09/2026. Không phải `refusal`/`decline` tự dịch tại chỗ. Quyền `feedback.classify` vì nhánh chỉ rời từ `dang-phan-loai` — là **một kết cục của phân loại** (ADR 0030), không phải việc xử lý |
| **Chuyển cấp trên** (phiếu phản ánh — nhánh kết thúc) | `chuyen-cap-tren` (trạng thái) · `ly_do_ket_thuc_nhanh` + `co_quan_nhan` | `referral` — `POST /api/v1/citizen-reports/{maTraCuu}/referral`, quyền `feedback.classify` | Danh từ do **người dùng** chốt 25/09/2026. **Không phải `escalation`**: từ 7/2025 phiếu được chuyển **ngang** (điện lực, công an, sở…) nhiều như chuyển **lên**, nên chữ "lên cấp trên" trên URL sẽ nói sai nghiệp vụ. Cơ quan nhận là văn bản tự do, không có danh mục (migration 0011) |
| Cấp số văn bản | `so_di` | `number` | |
| **Cán bộ** | `nguoi_dung` (bảng) · `can_bo` (nghiệp vụ) | `staff` — `GET /api/v1/staff` · `GET …/{id}`, quyền `admin.user`. **`web-admin` đã gọi cả hai tuyến** — `web-admin/src/lib/api/can-bo.ts` | `user` thì trùng với công dân — hai lớp tin cậy khác hẳn nhau (luật 4). Xem mục dưới: khái niệm này mang **bốn** cái tên |
| **Danh bạ chọn người nhận việc** | `nguoi_dung` (cùng bảng) | `staff-directory` — `GET /api/v1/staff-directory`, `AnyAuthenticated` (người dùng duyệt 24/09/2026) | Danh từ do **người dùng** chốt 24/09/2026. **Tài nguyên RIÊNG, không phải `staff?view=…`**, vì khác `staff` ở cả ba chỗ: **hợp đồng** (chỉ `code` · `full_name` · `position` · `department_id` — không `id`, không số điện thoại, không email, không cờ tài khoản), **quyền** (mọi tài khoản của xã, còn `staff` là `admin.user`) và **ai được liệt kê** (chỉ người có tài khoản đang hoạt động, chưa xoá — người giao việc được; `staff` liệt kê cả sổ). Một đường dẫn mang hai mức quyền theo tham số là đường dẫn mà mức bảo vệ do client chọn. Dùng chung cho ô phân công của Phản ánh, Nhiệm vụ, Thông báo, Văn bản & đơn thư. Điểm neo: `service-identity/internal/http/danh_ba_chon_nguoi.go` |
| **Xã của yêu cầu này** | `tenant` (bảng, service `platform`) | `communes/current` — `GET /api/v1/communes/current`, công khai | Cho **cán bộ**, suy từ `Host` ở rìa (luật 1 bất biến 3), trả `thongTinXa` cho màn đăng nhập. Số nhiều **dù chỉ trả về một xã** — xem ngay dưới |
| **Phiếu phản ánh CỦA CHÍNH NGƯỜI GỬI** | `phan_anh` (cùng bảng `phieu_phan_anh`) | `my-citizen-reports` — `GET /api/v1/my-citizen-reports/{maTraCuu}` | Cùng dữ liệu, **khác bề mặt**: đọc bởi công dân, lọc theo danh tính phiên. Không thể dùng lại `citizen-reports` vì tuyến cán bộ đã chiếm đúng đường dẫn ấy — xem §Tiền tố `my-` |
| **Khoá tài khoản** (nghỉ hưu, chuyển công tác) | `dang_hoat_dong = false` | `lockout` — `POST`/`DELETE /api/v1/staff/{id}/lockout` | **KHÔNG phải `disable`, `deactivate`, `suspend`.** Đây là một TRẠNG THÁI có thể mở lại, nên nó là một **tài nguyên con** mà `POST` tạo và `DELETE` gỡ — không phải một động từ trong đường dẫn. Khoá ≠ xoá (#10): người bị khoá VẪN còn trong danh bạ và vẫn hiện trên mọi hồ sơ cũ |
| **Tài khoản đăng nhập của một cán bộ** | `co_tai_khoan` · `mat_khau_hash` | `account` — `POST /api/v1/staff/{id}/account` | Một dòng danh bạ **KHÔNG tự động là một tài khoản**: xã nhập cả người chưa cần đăng nhập. `account` là thứ CẤP THÊM cho một dòng đã có, nên nó là tài nguyên con của `staff`. Bác `login-account` (dài mà không thêm nghĩa), `credentials` (số nhiều mơ hồ, lẫn với cặp tên/mật khẩu), `sign-in` (động từ — `rest_api_guard` chặn) |
| **Mật khẩu** | `mat_khau_hash` (CSDL chỉ giữ chuỗi băm) | `password` — `PUT /api/v1/staff/{id}/password` · `PUT /api/v1/staff/current/password` | Giữ `password` chứ không `credential`/`passphrase`: đây là **đúng một chuỗi bí mật**, và `credential` trong hệ này còn gồm phiên và `sid`. `current` là của máy chủ lấy từ phiên — **không** có `id` nào trên tuyến tự đổi, vì một tham số định danh ở đó là đổi mật khẩu người khác (luật 4 cấm #1) |
| **Biên bản họp** | `bien_ban_hop` · **`@entity: Meeting`** | `meetings` — `GET /api/v1/meetings`, quyền `task.read` (`petitions`) · `PATCH`/`DELETE /api/v1/meetings/{id}`, quyền `task.create`, chỉ bản nháp (người dùng chốt 25/09/2026) | **KHÔNG tự dịch — lấy theo BẰNG CHỨNG, đúng điều ADR 0011 đòi.** Đặc tả `docs/ui-ux/04-bien-ban-hop.md` §6 đề nghị `/api/bien-ban`, mà `hooks/rest_api_guard.py` **chặn** danh từ tiếng Việt trên URL, nên đường ấy không ship được. `meetings` là chữ **kho anh em đang chạy dùng thật** — `../vigov-require/apps/api/app/modules/tasks/router.py:45`. Không phải `minutes` (là *bản ghi chép*, một trường của biên bản, không phải chính cuộc họp) và không phải `sessions` (đã bị phiên đăng nhập chiếm) |
| **Kết luận họp** | `ket_luan_hop` · **`@entity: MeetingConclusion`** (`service-petitions/migrations/0007_bien_ban.sql:270`; dòng này trước 29/09/2026 ghi `Conclusion` và đã sai) | `conclusions` — lồng dưới `meetings`: `POST /api/v1/meetings/{id}/conclusions`, `PATCH`/`DELETE /api/v1/meetings/{id}/conclusions/{stt}`, quyền `task.create` | Cùng nguồn bằng chứng: bên kia gọi `conclusions`. **Không có tuyến `/conclusions` độc lập** — kết luận luôn thuộc một biên bản và được gọi theo **số thứ tự `{stt}`** (①②③) chứ không theo id. Sửa/xoá chốt 25/09/2026 bởi **người dùng**: chỉ khi biên bản còn nháp và kết luận chưa có nhiệm vụ còn hiệu lực |
| **Ký biên bản** (nháp → đã ký) | `trang_thai` `du-thao` → `da-ky` · `ky_luc` · `ky_boi_ma` | `signature` — `POST /api/v1/meetings/{id}/signature`, quyền `task.approve` | Danh từ do **người dùng** chốt 25/09/2026. Tài nguyên con danh từ hoá, không phải động từ `sign`. Ký hai lần → 409, không phải chữ ký thứ hai. Quyền `task.approve` chứ không `task.create`: người gõ biên bản không mặc nhiên là người được ký |
| **Thông báo kết luận** | `tb_so_ky_hieu` · `tb_ngay` | `notice` — trường `{reference_no, issued_on}` của `meetings` (`PATCH /api/v1/meetings/{id}`, `POST …/signature`) | Chốt 25/09/2026 bởi **người dùng**. Là trường chép tay, không phải tài nguyên riêng. Sau khi ký chỉ ghi được **một lần** (trigger 0012); ở bản nháp thì sửa được |
| **Dấu "không phát sinh nhiệm vụ"** | `khong_phat_sinh` · `_luc` · `_boi_ma` (`ket_luan_hop`) | `no-task-marker` — `PUT`/`DELETE /api/v1/meetings/{id}/conclusions/{stt}/no-task-marker`, quyền `task.create` | Danh từ do **người dùng** chốt 25/09/2026. Một TRẠNG THÁI đặt/gỡ được nên là tài nguyên con (cùng khuôn `lockout`). Trên dây trả về dưới tên `no_task` của kết luận |
| **Biên bản bổ sung** | `bo_sung_cho_id` (`bien_ban_hop`) | `supplements_id` (gửi khi tạo) · `supplemented_by` (đọc) — trên `meetings` | Chốt 25/09/2026 bởi **người dùng**: chỉ lập cho biên bản **đã ký**; bản mới là nháp bình thường, kết luận đánh số lại từ ① |
| **Bài nội dung Mini App** | `noi_dung_mini_app` · **`@entity: ContentItem`** | `content-items` — `GET/POST /api/v1/content-items`, quyền `content.read` / `content.update` (`comms`) | **Lấy theo BẰNG CHỨNG, không tự dịch** — `../vigov-require/apps/api/app/modules/content/router.py:54,66`. Đặc tả `docs/ui-ux/11-noi-dung-mini-app.md` §9 đề nghị `/api/mini-app/noi-dung` (danh từ tiếng Việt — `rest_api_guard` chặn) và `/api/cong/…` (ngoài tiền tố `/api/v1/` mà `tools/ingress` bắt buộc → 404 lúc phát hành); cả hai đường không ship được. **Gạch ngang chứ không lồng `content/items`**: `tools/ingress` gom theo ĐOẠN ĐẦU sau `/api/v1/`, và `content` trần là một *module* chứ không phải tài nguyên — lồng sẽ tạo "một tài nguyên hai chủ" |
| **Danh mục tin Mini App** | `danh_muc_mini_app` · **`@entity: ContentCategory`** | `content-categories` — `GET/POST /api/v1/content-categories` (`comms`) | Cùng nguồn bằng chứng (`router.py:186`). Cây tự tham chiếu `cha_id`; **chưa có tuyến sửa/xoá** có chủ ý — đổi tên thì vô hại, đổi cha thì không: đó là lượt ghi duy nhất tạo được chu trình ≥2 đỉnh mà không `CHECK` nào từ chối được, nên tuyến ấy tới cùng lúc với phép duyệt tổ tiên của nó |
| **Đề nghị lùi hạn nhiệm vụ** | `de_nghi_lui_han` | `extensions` — `POST /api/v1/tasks/{ma}/extensions` (`task.update`) · `…/extensions/{id}/decision` (`task.extend` + ADR 0038) · **hàng chờ duyệt** `task-extensions` — `GET /api/v1/task-extensions`, quyền `task.read`, lọc `approver=me` (`petitions`) | `task-extensions` do **người dùng** chốt 27/09/2026. **Tài nguyên RIÊNG cho danh sách trên mọi nhiệm vụ**, cùng khuôn gạch ngang `task-types`: đặt dưới `tasks/extensions` thì trùng hình dạng `tasks/{ma}`. `approver=me` lấy mã cán bộ từ phiên, không bao giờ từ tham số. Hai danh từ `extensions` · `decision` có từ `nhiem-vu-tuyen-ghi` (cb3d0d5), ghi vào đây cùng lượt |
| **Nhật ký & Trao đổi của nhiệm vụ** | `nhat_ky_nhiem_vu` | `log-entries` — `GET /api/v1/tasks/{ma}/log-entries`, quyền `task.read` (`petitions`) | **Dùng lại danh từ của nhật ký phiếu phản ánh** (`citizen-reports/{maTraCuu}/log-entries`) để hai dòng thời gian đọc giống nhau. Chưa có `POST` — ai được ghi tay là câu luật người đang giữ, chưa quyết cho nhiệm vụ |
| **Xác thực lời khai cư trú** | `citizen.verify` (khoá quyền) | *(tuyến chưa dựng)* | Khoá quyền chốt 22/09/2026 — ADR 0035 §D. Nhóm `citizen` là **nhóm thứ mười một** mà `14-cau-hinh.md:104` đếm tới nhưng bảng dưới không liệt kê. **Chưa có migration nạp**: viết tuyến thì nạp cùng lượt, nếu không tuyến ấy trả 403 cho mọi tài khoản mãi mãi (luật 5 bất biến 3c) |
| **Mục nhật ký hệ thống** (vết thao tác của xã) | `audit_log` · `audit.Entry` | `<service>-audit-entries` — `comms-` · `documents-` · `finance-` · `identity-` · `petitions-audit-entries`, `GET`, quyền `admin.audit` *(tuyến chưa dựng)* | **Nhà cung cấp chọn** 28/09/2026 — lý do ở ADR 0054 §3. Một danh từ **mỗi service**, vì `tools/ingress` không cho hai service chung một đoạn đầu. Không `activity-logs` (kho yêu cầu), không `log-entries` (đã là nhật ký nhiệm vụ/phiếu ở dòng trên) |
| **Đánh giá của công dân** (1–5 sao) | `diem_hai_long` · `rating_comment` (`phieu_phan_anh`) | `rating` — `POST /api/v1/my-citizen-reports/{maTraCuu}/rating`, `CitizenOnly` (`petitions`) | Tài nguyên con danh từ hoá, không động từ `rate`. **Không có tuyến cán bộ ghi hộ** — chủ dự án chốt 28/09/2026 (`service-petitions/internal/http/petition_rating.go:11-12`). Danh từ **đang dùng trong mã, chưa người dùng chốt** |
| **Mở lại** | phiếu: `so_lan_mo_lai`, mục nhật ký `mo-lai-theo-danh-gia` · nhiệm vụ: rời `hoan-thanh` | **Không có tài nguyên riêng.** Phiếu mở lại **bên trong** `rating` khi chấm 1–2 sao (ADR 0050 điểm 2) và trả trường `reopen_count`; nhiệm vụ mở lại cần `task.approve` (`ErrReopenNeedsApproval`, `service-petitions/internal/app/nhiem_vu.go:232-236`) | Hai nghĩa, cùng một từ `reopen` trong mã. Không thêm tuyến `…/reopening`: mở lại phiếu là **hệ quả** của lời đánh giá, không phải một hành vi riêng của ai |
| **Công khai / kiểm duyệt** | `publication_status` — `cho-duyet` · `cong-khai` · `an` (`service-petitions/migrations/0017_petition_publication_and_rating_comment.sql:163`) | `publication` — `PUT /api/v1/citizen-reports/{maTraCuu}/publication` (`feedback.assign`, `petitions`) · `PUT /api/v1/staff/{id}/publication` (`content.update`, `identity`) | "Kiểm duyệt" của kho yêu cầu (`POST /{id}/moderation`) viết lại thành `publication` — danh từ `rest_api_guard` đề xuất cho `publish`. **Không đổi trạng thái vòng đời** phiếu (người dùng 28/09/2026). Mã tự ghi danh từ này là **quyết định đặt tên còn mở** (`service-identity/internal/http/routes.go:1175-1177`) |
| **Tự động hoá** (việc nền theo xã) | `automation_job_setting` · `automation_run_scope` · `automation_run` (`service-identity/migrations/0017_automation_jobs.sql`) | `automation-jobs` — `GET` · `PUT …/{job}` · `POST …/{job}/runs` ("chạy ngay"), quyền `admin.sla` (`identity`) | Số nhiều của `AutomationJob` trong hợp đồng. Đặc tả `/tu-dong-hoa` là đoạn tiếng Việt, kho yêu cầu `/automation` là danh từ không đếm được. Mã ghi rõ **"stated, not settled"** (`service-identity/internal/http/automation.go:27-31`) — chưa người dùng chốt. Nơi chạy: ADR 0058 |

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

### Miền vận hành ViHAT — ADR 0048

Chốt 28/09/2026 bởi **chủ dự án** (ADR 0048 §*Chốt* và §*Chốt bước 1*). Không tài nguyên URL nào
ở đây: tuyến tới ở bước 2, trên host vận hành — đừng điền sẵn.

| Khái niệm | Thực thể / bảng | Vì sao không phải từ dễ đoán |
|---|---|---|
| **Người vận hành** (nhân sự ViHAT) | `operator_account` | **Không** `admin`, `superadmin`, `platform_user`: `quan_tri` đã là vai trò **của một xã**, và "superadmin" là đúng thứ luật 5 cấm #2. `operator` nói miền chứ không nói mức quyền. Mã nghiệp vụ `VH-00001` |
| Phiên người vận hành | `operator_session` | Không dùng lại `phien` (cán bộ) hay `CitizenSession`: ba lớp tin cậy, ba bảng |
| Cấp quyền vận hành | `operator_permission_grant` · khoá `ops.<nhóm>.<việc>` | **Không** phải dòng trong `quyen`/`vai_tro_quyen` (theo xã). Cấp thẳng cho tài khoản, không qua vai trò |
| Mã khôi phục | `operator_recovery_code` | *Recovery code* — mã dùng một lần khi mất ứng dụng xác thực. Không gọi `backup_code` |
| Vết vận hành | `operator_audit_log` | Vết **không có xã** của miền vận hành trong `identity`; khác `platform_audit_log` (thuộc `platform`, luật 2) |

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

## Từ điển đổi tên Việt → Anh — ADR 0061

**MỘT từ điển cho cả đợt đổi tên.** Người làm lớp A/B tra ở đây; người thanh tra đọc khoá
`audit_log.delta` và chú thích migration cũ cũng tra ở đây. Vì thế **không bao giờ xoá dòng**: tên cũ
còn nằm trong hồ sơ chỉ-thêm và trong migration đã áp mãi mãi. Giá trị **enum** và giá trị
**hành vi nhật ký** không nằm ở đây — ADR 0061 §Ánh xạ giá trị enum sở hữu chúng.

**Trạng thái: ĐỀ XUẤT.** Nguồn của mỗi tên, theo thứ tự ưu tiên: (1) tên tiếng Anh **đã chạy** trên
dây — trường JSON, đoạn URL; (2) dấu `-- @entity:` trong migration; (3) kiểu Go tiếng Anh đã có; (4)
dịch mới, chỉ khi ba nguồn trên im lặng. Dòng mang **[X…]** chờ §Xung đột chờ chọn; **không làm lớp
B cho bảng mang [X…] trước khi mục ấy chốt.** `<P>` là từ X1 chọn cho phản ánh.

### Quy tắc ghép — áp cho mọi bảng trừ khi bảng dưới ghi khác

| Cũ | Mới | Ghi chú |
|---|---|---|
| `tao_luc` · `tao_boi` · `cap_nhat_luc` · `cap_nhat_boi` | `created_at` · `created_by` · `updated_at` · `updated_by` | Đã là tên ở mọi bảng tiếng Anh (`stored_file`, `upload_policy`) |
| `<động từ>_luc` | `<quá khứ phân từ>_at` | `ky_luc`→`signed_at`, `dong_luc`→`closed_at` |
| `<động từ>_boi`, `<động từ>_boi_ma` | `<quá khứ phân từ>_by` | Cột `_by` giữ **mã nghiệp vụ** cán bộ (luật 6 bất biến 8), không giữ id |
| `nguoi_tao_ma` | `created_by` | |
| `nguoi_<vai>_ma` | `<vai>_code` | `nguoi_nhan_ma`→`recipient_code` |
| `<thực thể>_id` | `<thực thể mới>_id` | `bo_phan_id`→`org_unit_id` **[X3]** |
| `ma` · `ten` · `nhan` · `mo_ta` · `ghi_chu` · `ly_do` | `code` · `name` · `label` · `description` · `note` · `reason` | `label` vs `name`: giữ đúng ranh giới §Danh mục tham chiếu (nhãn sửa được vs tên riêng) |
| `thu_tu` | `sort_order` | Trên dây đã là `order`; cột lấy tên của `map_field_schema`, `petition_field` |
| `trang_thai` · `loai` · `nhom` · `nguon` · `nam` · `ngay` | `status` · `type` · `group` · `source` · `year` · `date` | |
| `noi_dung` · `tieu_de` · `trich_yeu` · `tom_tat` | `content` · `title` · `summary` · `summary` | `trich_yeu` (văn bản) và `tom_tat` (bài) cùng thành `summary` — hai bảng khác nhau, không đụng |
| `dang_dung` · `dang_hoat_dong` · `la_*` · `co_*` · `phai_*` | `is_active` · `is_active` · `is_*` · `has_*` · `must_*` | **[X13]** |
| `trang_thai_tai_thoi_diem` · `thoi_diem` | `status_at_time` · `occurred_at` | |
| `dinh_kem` · `tep_dinh_kem` | `attachments` | |
| Bộ cột danh mục ba tầng: `moc_mac_dinh` · `ma_nguon_re_nhanh` | `default_marker` · `branched_in_source` | Bảy bảng danh mục, cùng hình dạng (ADR 0024) |
| `deleted_at` · `deleted_by` · `delete_reason` · `tenant_id` · `id` | không đổi | |

### Gốc định danh Go/TS — lớp A

| Cũ | Mới | Nguồn / ghi chú |
|---|---|---|
| `xa` | `commune` (nghiệp vụ) · `tenant` (khoá cô lập, bảng `tenant`) | **[X5]** |
| `tinh_thanh` | `province` | `@entity: Province` |
| `can_bo` · `nguoi_dung` | `staff` | `/api/v1/staff`, `Staff` (proto). Bốn tên của §"Một khái niệm, bốn cái tên" thu về một — **[X17]** |
| `cong_dan` | `citizen` | |
| `bo_phan` | `org_unit` | **[X3]** |
| `vai_tro` · `quyen` · `phan_quyen` | `role` · `permission` · `role_permission` | `/api/v1/role-permissions` |
| `tai_khoan` · `mat_khau` · `phien` · `dang_nhap` · `dang_xuat` | `account` · `password` · `session` · `log_in` · `log_out` | Phiên cán bộ: **[X16]** |
| `dinh_danh` · `ghep_phien` · `tai_khoan_zalo` | `identity` · `session_pairing` · `zalo_account` | ADR 0023 |
| `thon_to_dan_pho` · `don_vi_dan_cu` | `residential_unit` | |
| `danh_muc` · `ba_tang` · `danh_ba` | `catalogue` · `three_tier` · `directory` | `CatalogueImporter`, `staff-directory` |
| `nhiem_vu` · `khoi` · `muc_uu_tien` · `nhat_ky` · `lui_han` | `task` · `bloc` · `priority` · `log_entry` · `extension` | |
| `bien_ban_hop` · `ket_luan_hop` | `meeting` · `meeting_conclusion` | `@entity` |
| `phieu_phan_anh` · `phan_anh` | **[X1]** | |
| `don_thu` | **[X2]** | |
| `linh_vuc` · `ma_tra_cuu` · `nguoi_gui` · `kenh` | `field` · `lookup_code` · `reporter` · `channel` | `PetitionField`, `reporter_name` |
| `tiep_nhan` · `thu_ly` · `phan_loai` · `phan_cong` · `nghiem_thu` · `dong_phieu` | **[X15]** · `admission` · `classification` · `assignment` · `verification` · `closure` | §Tên tài nguyên trên URL |
| `khong_tiep_nhan` · `chuyen_cap_tren` · `mo_lai` · `danh_gia` · `cong_khai` · `xem_day_du` | `rejection` · `referral` · `reopen` · `rating` · `publication` · `unmask` | |
| `han` · `han_xu_ly` · `han_tiep_nhan` · `han_xu_ly_xong` · `han_phan_loai` · `goc_dem_han` | `due` · `due_at` · `acknowledge_due` · `resolve_due` · `classify_due` · `clock_from` | Trường JSON đang chạy |
| `lich_lam_viec` · `ca_lam_viec` · `ngay_nghi_le` · `ngay_lam_bu` · `gio_lam_viec` | `working_hours` · `working_shift` · `public_holiday` · `swap_working_day` · `working_hours` | Không `session` cho ca — §Lịch làm việc của xã |
| `van_ban` · `van_ban_den` · `van_ban_di` · `so_sach` · `so_vao_so` · `cap_so` · `chuyen` (văn bản) | `document` · `incoming_document` · `outgoing_document` · `register` · `arrival_no` · `issue_number` · `route` | `arrival_no`: luật 3 bất biến 2 |
| `giai_ngan` · `chung_tu` · `du_an` · `nguon_von` · `hang_muc` · `ke_hoach_von` | `disbursement` · `voucher` · **[X6]** · `funding_source` · `category` · `capital_plan` | **[X7]** |
| `ngan_sach` · `bang` · `khoan_muc` · `cot` · `dot` · `thu` · `chi` | `budget` · `sheet` · `line` · `column` · `entry` · `revenue` · `expenditure` | **[X11] [X12]** |
| `nguong` · `cham` | `threshold` · `delay` | `delay_threshold` trên dây |
| `thong_bao` (nội bộ) · `thong_bao` (chuông) · `thong_bao_gui_cong_dan` | `announcement` · `staff_notification` · `citizen_notification` | Ba thứ, §Tên tài nguyên trên URL dòng `Thông báo` |
| `noi_dung_mini_app` · `danh_muc_mini_app` · `ho_so_hien_thi` | `content_item` · `content_category` · `display_profile` | **[X5]** cho hồ sơ |
| `ban_do` · `tai_nguyen` · `tu_dong_hoa` · `su_kien_di` | `map` · `asset` · `automation` · `outbox_event` | |
| `tep` · `xoa_mem` · `go` · `khoa` · `mo_khoa` · `lich_su` | `file` · `soft_delete` · `remove` · `lock` · `unlock` · `history` | `go` ≠ `xoa` — ADR 0061 |
| Tiền tố kiểu: `YeuCau*` · `Kho*` · `Loc*` · `KetQua*` · `Loi*` · `Doc*` · `Ghi*` | hậu tố `*Request` · `*Store` · `*Filter` · `*Result` · `*Error` · `*View` · `*Input` | Khuôn đã có: `TaskAssignmentRequest`, `StaffImportResult` |
| Động từ: `them` · `tao` · `sua` · `xoa` · `gieo` · `nhap` · `xuat` | `create` · `create` · `update` · `delete` · `seed` · `import` · `export` | |

### Bảng — lớp B

Bảng đã tiếng Anh (`audit_log`, `tenant`, `tenant_domain`, `tenant_succession`, `mini_app`,
`upload_policy`, `platform_audit_log`, `petition_field`, `stored_file`, `task_issued_code`,
`task_log_attachment`, `system_message_override`, `staff_notification`, `map_field_schema`,
`mail_settings`, `data_encryption_key`, `operator_*`, `automation_*`) **không đổi**. Cột: chỉ ghi cột
**không** suy ra được từ §Quy tắc ghép.

| Service | Bảng cũ | Bảng mới | Cột không theo quy tắc ghép |
|---|---|---|---|
| platform | `tinh_thanh` | `province` | — |
| platform | `ho_so_hien_thi_xa` | **[X5]** (`tenant_display_profile` hoặc `commune_profile`) | `dia_chi_tru_so`→`office_address` · `duong_day_nong`→`hotline` · `gio_lam_viec_hien_thi`→`office_hours_text` · `gioi_thieu`→`introduction` |
| platform | `tenant` (cột) | — | `tinh_thanh`→`province_code` |
| platform | `tenant_domain` (cột) | — | `la_chinh`→`is_primary` |
| platform | `tenant_succession` (cột) | — | `tu_id`→`from_tenant_id` · `den_id`→`to_tenant_id` · `can_cu`→`legal_basis` · `hieu_luc_tu`→`effective_from` |
| platform | `mini_app` (cột) | — | `che_do`→`mode` |
| finance | `hang_muc_ke_hoach_von` | `capital_plan_category` | bộ cột danh mục ba tầng |
| finance | `du_an` | **[X6]** | `hang_muc_id`→`category_id` · `don_vi_thuc_hien_id`→`implementing_org_unit_id` · `can_bo_phu_trach_id`→`owner_staff_id` · `tong_muc_duoc_duyet`→`approved_amount` · `ke_hoach_von_nam`→`planned_amount` · `ngay_khoi_cong`→`start_date` · `ngay_hoan_thanh`→`completion_date` · `thoi_han_giai_ngan`→`disbursement_deadline` |
| finance | `chung_tu_giai_ngan` | **[X7]** | `du_an_id`→`project_id` · `ngay_chi`→`payment_date` · `so_tien`→`amount` · `doi_tac`→`counterparty` · `so_chung_tu`→`voucher_no` · `nguon_von_id`→`funding_source_id` · `nguoi_nhap_id`→`entered_by_id` · `nguoi_xac_nhan_id`→`confirmed_by_id` · `nguoi_khoa_id`→`locked_by_id` · `nguoi_mo_khoa_id`→`unlocked_by_id` · `thoi_diem_khoa`→`locked_at` · `thoi_diem_mo_khoa`→`unlocked_at` · `ly_do_mo_khoa`→`unlock_reason` · `so_lan_mo_khoa`→`unlock_count` |
| finance | `cau_hinh_giai_ngan` | `disbursement_settings` | `nguong_canh_bao_cham`→`delay_threshold` |
| finance | `bang_ngan_sach` | `budget_sheet` | `loai`→`kind` · `lan`→`revision` · `don_vi_tinh`→`unit` · `luy_ke_den`→`cumulative_to` · `nguon_tep`→`source_file` · `nap_luc`→`imported_at` |
| finance | `cot_ngan_sach` | `budget_column` | `bang_id`→`sheet_id` · `kieu`→`format` · `vai_tro`→`indicator` · `cong_thuc`→`formula` |
| finance | `khoan_muc_ngan_sach` | `budget_line` | `bang_id`→`sheet_id` · `cha_id`→`parent_id` · `tt`→`ordinal_label` · `cap`→`level` · `cach_tinh`→`method` |
| finance | `gia_tri_khoan_muc` | `budget_line_value` | `khoan_muc_id`→`line_id` · `cot_id`→`column_id` · `gia_tri`→`value` |
| finance | `nguon_von` | `funding_source` | `tong_nguon`→`total_amount` |
| finance | `phan_bo_nguon_von` | `funding_allocation` | `nguon_von_id`→`funding_source_id` · `du_an_id`→`project_id` · `so_tien_phan_bo`→`allocated_amount` |
| finance | `dot_thu_chi` | `budget_entry` **[X11]** | `khoan_muc_id`→`line_id` · `so_chung_tu`→`voucher_no` · `don_vi_ca_nhan`→`counterparty` · `nguoi_ghi_ma`→`entered_by` |
| finance | `gia_tri_dot` | `budget_entry_amount` | `dot_id`→`entry_id` · `cot_id`→`column_id` · `gia_tri`→`value` |
| comms | `loai_tai_nguyen_ban_do` | `map_asset_type` | bộ cột danh mục ba tầng |
| comms | `thong_bao_gui_cong_dan` | `citizen_notification` | `doi_tuong_loai`→`subject_type` · `doi_tuong_ma`→`subject_code` · `moc`→`milestone` · `mau_ma`→`template_code` · `tham_so`→`params` · `nguoi_nhan_che`→`masked_recipient` · `khoa_lan_gui`→`send_key` · `lan`→`round` · `so_lan_thu`→`attempt_count` · `loi_ma`→`error_code` |
| comms | `thong_bao` | `announcement` | `noi_dung`→`body` · `nguoi_soan_ma`→`author_code` · `trang_thai_thu`→`email_status` · `bat_buoc_xac_nhan`→`ack_required` · `ghim`→`pinned` · `gui_thu_dien_tu`→`email_requested` |
| comms | `thong_bao_bo_phan` | `announcement_org_unit` | `thong_bao_id`→`announcement_id` |
| comms | `thong_bao_nguoi_nhan` | `announcement_recipient` | `thong_bao_id`→`announcement_id` · `dich_danh`→`is_named` · `da_mo_luc`→`read_at` · `da_xac_nhan_luc`→`acknowledged_at` · `thu_gui_luc`→`email_sent_at` · `thu_loi_ma`→`email_error_code` |
| comms | `danh_muc_mini_app` | `content_category` | `cha_id`→`parent_id` |
| comms | `noi_dung_mini_app` | `content_item` | `danh_muc_id`→`category_id` · `noi_dung`→`body` · `anh_dai_dien_url`→`image_url` · `ngay_dang`→`published_on` · `luot_xem`→`view_count` · `nguon_url`→`source_url` · `nguon_id_ngoai`→`source_ref` · `da_sua_tay`→`hand_edited` · `nguoi_tao_ma`→`author_code` |
| documents | `loai_van_ban` | `document_type` | bộ cột danh mục ba tầng |
| documents | `day_so_van_ban` | `document_number_series` | `so_sach`→`register` · `so_cuoi`→`last_number` |
| documents | `van_ban_den` | `incoming_document` | `so_vao_so`→`arrival_no` · `ngay_den`→`received_date` · `so_ky_hieu`→`reference_no` · `ngay_van_ban`→`document_date` · `co_quan_ban_hanh`→`issuing_body` · `loai_van_ban`→`document_type` · `do_khan`→`urgency` · `bo_phan_dang_giu_id`→`holding_org_unit_id` · `can_bo_xu_ly_ma`→`assignee_code` · `han_xu_ly_xong`→`due_at` (tên trên dây của `documents`) |
| documents | `van_ban_di` | `outgoing_document` | `so_di`→`issued_no` · `nguoi_ky`→`signer` · `noi_nhan`→`recipient` |
| documents | `lich_su_chuyen_van_ban` | `document_routing` | `van_ban_den_id`→`incoming_document_id` · `tu_bo_phan_id`→`from_org_unit_id` · `den_bo_phan_id`→`to_org_unit_id` · `nguoi_ma`→`routed_by` · `noi_dung`→`instruction` · `thoi_diem`→`routed_at` |
| identity | `quyen` | `permission` | `ma`→`key` — ⚠ `tools/quyen_keys.py` đọc `INSERT INTO quyen` (ADR 0061 Lớp 0) |
| identity | `bo_phan` | `org_unit` **[X3]** | `cha_id`→`parent_id` |
| identity | `vai_tro` | `role` | `la_lanh_dao`→`is_leader` |
| identity | `vai_tro_quyen` | `role_permission` | `quyen_ma`→`permission_key` · `cap_boi`→`granted_by` · `cap_luc`→`granted_at` |
| identity | `nguoi_dung` | `staff` **[X17]** | `ho_ten`→`full_name` · `chuc_vu`→`position` · `dien_thoai`→`phone` · `di_dong_ca_nhan`→`mobile` · `mat_khau_hash`→`password_hash` · `dang_nhap_gan_nhat`→`last_login_at` · `hien_tren_mini_app`→`shown_in_mini_app` · `thu_tu_danh_ba`→`directory_order` · `dong_y_cong_khai_luc`→`publication_consent_at` · `dong_y_cong_khai_ghi_boi`→`publication_consent_by` |
| identity | `phien` | `staff_session` **[X16]** | `nguoi_dung_id`→`staff_id` · `thiet_bi`→`device` · `ip_tao`→`created_ip` · `dung_gan_nhat`→`last_used_at` · `het_han_luc`→`expires_at` · `thu_hoi_luc`→`revoked_at` · `thu_hoi_ly_do`→`revoked_reason` · `thay_the_boi`→`replaced_by` |
| identity | `dinh_danh_cong_dan` | `citizen_identity` | `so_dien_thoai`→`phone_number` |
| identity | `quan_he_cong_dan_xa` | `citizen_commune` | `cong_dan_id`→`citizen_id` · `khai_cu_tru`→`residence_claim` · `khai_luc`→`claimed_at` · `nguon_khai`→`claim_source` · `trang_thai_xac_thuc`→`verification_status` · `xet_duyet_boi`→`reviewed_by` · `xet_duyet_luc`→`reviewed_at` · `ly_do_tu_choi`→`rejection_reason` |
| identity | `phien_cong_dan` | `citizen_session` | như `staff_session`, cộng `cong_dan_id`→`citizen_id` · `bam_token`→`token_hash` · `tai_khoan_zalo_id`→`zalo_account_id` |
| identity | `ghep_phien` | `session_pairing` | `cong_dan_id`→`citizen_id` · `phien_id`→`session_id` · `bam_ma`→`code_hash` · `ky_tu_doi_chieu`→`check_characters` · `dung_luc`→`used_at` · `huy_luc`→`cancelled_at` · `huy_ly_do`→`cancel_reason` |
| identity | `loai_don_vi_dan_cu` · `khoi_nhiem_vu` | `residential_unit_type` · `task_bloc` | bộ cột danh mục ba tầng |
| identity | `thon_to_dan_pho` | `residential_unit` | `loai`→`type_code` · `so_ho`→`household_count` · `nhan_khau`→`population_count` |
| identity | `lich_lam_viec` | `working_hours` | `thu`→`weekday` · `bat_dau`→`start_time` · `ket_thuc`→`end_time` |
| identity | `ngay_nghi_le` · `ngay_lam_bu` | `public_holiday` · `swap_working_day` | `bat_dau`→`start_time` · `ket_thuc`→`end_time` |
| identity | `sla` | `sla` **[X8]** | `loai_viec`→`work_kind` · `linh_vuc`→`field_code` · `linh_vuc_khoa`→`field_key` · `gio_tiep_nhan`→`acknowledge_hours` · `gio_xu_ly_xong`→`resolve_hours` · `gio_sap_den_han`→`due_soon_hours` · `gio_bao_lanh_dao`→`escalate_leader_hours` · `gio_bao_chu_tich`→`escalate_president_hours` **[X4]** |
| identity | `tai_khoan_zalo` | `zalo_account` | `bam_zalo_user_id`→`zalo_user_id_hash` · `cong_dan_id`→`citizen_id` · `lien_ket_luc`→`linked_at` · `xa_da_nho`→`remembered_tenant_id` · `xa_da_nho_luc`→`remembered_at` **[X5]** |
| petitions | `loai_nhiem_vu` · `muc_uu_tien_nhiem_vu` | `task_type` · `task_priority` | bộ cột danh mục ba tầng |
| petitions | `nhan_linh_vuc` | **[X1]** `<P>_field_label` | `ma`→`field_code` |
| petitions | `phieu_phan_anh` | **[X1]** `<P>` (`petition` hoặc `citizen_report`) | `ma_tra_cuu`→`lookup_code` · `cong_dan_id`→`citizen_id` · `kenh_tiep_nhan`→`channel` · `linh_vuc`→`field_code` · `dia_chi`→`address` · `thon_id`→`residential_unit_id` · `an_danh`→`anonymous` · `nguoi_gui_ho_ten`→`reporter_name` · `nguoi_gui_dien_thoai`→`reporter_phone` · `can_bo_xu_ly_id`→`assignee_id` · `han_tiep_nhan`→`acknowledge_due` · `han_phan_loai`→`classify_due` · `han_xu_ly_xong`→`resolve_due` · `goc_dem_han`→`clock_from` · `vao_so_luc`→`booked_at` · `phan_loai_luc`→`classified_at` · `xu_ly_xong_luc`→`resolved_at` · `ket_qua_xu_ly`→`result` · `ket_thuc_nhanh_luc`→`branch_ended_at` · `ly_do_ket_thuc_nhanh`→`branch_end_reason` · `co_quan_nhan`→`receiving_body` · `hien_cong_khai`→`is_public` · `diem_hai_long`→`rating` · `danh_gia_luc`→`rated_at` · `so_lan_mo_lai`→`reopen_count` |
| petitions | `su_kien_di` | `outbox_event` | `doi_tuong`→`subject` · `than`→`payload` · `xay_ra_luc`→`occurred_at` · `gui_luc`→`sent_at` |
| petitions | `nhiem_vu` | `task` | `loai`→`type_code` · `khoi`→`bloc_code` · `muc_uu_tien`→`priority_code` · `nguon_giao`→`source` · `nhiem_vu_cha_id`→`parent_task_id` · `co_quan_chu_tri_id`→`lead_org_unit_id` · `lanh_dao_giao_viec_ma`→`assigner_code` · `nguoi_thuc_hien_ma`→`assignee_code` · `chuyen_vien_theo_doi_ma`→`monitor_code` · `han_xu_ly`→`due_at` · `han_ban_dau`→`original_due_at` · `tien_do`→`progress` · `ngay_hoan_thanh`→`completed_at` · `tom_tat_ket_qua`→`result_summary` · `lanh_dao_phe_duyet_hoan_thanh`→`leader_approved` · `cap_tren_cong_nhan_hoan_thanh`→`superior_acknowledged` |
| petitions | `nhat_ky_nhiem_vu` | `task_log_entry` | `nhiem_vu_id`→`task_id` · `nguoi_ma`→`actor_code` · `nguoi_phu_trach_ma`→`responsible_code` |
| petitions | `de_nghi_lui_han` | `task_extension_request` **[X10]** | `nhiem_vu_id`→`task_id` · `nguoi_de_nghi_ma`→`requested_by` · `nguoi_duyet_ma`→`decided_by` · `han_moi`→`new_due_at` · `thoi_diem`→`requested_at` · `duyet_luc`→`decided_at` · `moc_cho_duyet`→`pending_marker` |
| petitions | `bien_ban_hop` | `meeting` | `ten_cuoc_hop`→`title` · `ngay_hop`→`held_on` · `dia_diem`→`location` · `chu_tri_ma`→`chaired_by` · `thu_ky_ma`→`minutes_taker_code` · `thanh_phan`→`attendees` · `so_hieu`→`number` · `tb_so_ky_hieu`→`notice_reference_no` · `tb_ngay`→`notice_issued_on` · `bo_sung_cho_id`→`supplements_id` |
| petitions | `ket_luan_hop` | `meeting_conclusion` **[X9]** | `bien_ban_id`→`meeting_id` · `thu_tu`→`ordinal` · `khong_phat_sinh`→`no_task` · `khong_phat_sinh_luc`→`no_task_at` · `khong_phat_sinh_boi_ma`→`no_task_by` |
| petitions | `nhiem_vu_van_ban` | `task_document` | `nhiem_vu_id`→`task_id` · `so_ky_hieu`→`reference_no` · `ngay_van_ban`→`document_date` |
| petitions | `nhan_trang_thai_nhiem_vu` | `task_status_label` | `ma`→`status` |
| petitions | `nhat_ky_phan_anh` | **[X1]** `<P>_log_entry` | `phieu_phan_anh_id`→`<P>_id` · `hanh_vi`→`action` · `nguoi_ma`→`actor_code` · `can_bo_xu_ly_ma`→`assignee_code` |

Cột thêm bằng `ALTER TABLE … ADD COLUMN` ở migration sau (tài khoản tạm `0009`, ảnh nghiệm thu…) đi
theo §Quy tắc ghép; cột nào quy tắc không cho một tên **hiển nhiên** thì dừng và thêm dòng vào bảng
trên trước, không tự đặt trong migration.

**Tên trường JSON tiếng Việt còn trên dây** — thuộc lớp C (nhận cả hai trong cửa sổ chuyển tiếp),
không thuộc lớp A: `petitions` — `cong_dan_id`, `linh_vuc` (`service-petitions/internal/http/gui_phan_anh.go:87-91`,
thân yêu cầu Mini App gửi), `an_danh`, `kenh_tiep_nhan`, `han_tiep_nhan`, `trang_thai`, `do_dai_noi_dung`,
`quyen`, `truong_da_dien`, `truong_da_mo`; `identity` — `da_co`, `da_gieo`, `dong`, `truoc`, `sau`;
`comms` — `ma_tra_cuu`, `moc_nhan`, `viec_tiep_theo`. Danh sách dựng bằng quét thẻ `json:"…"` ngày
29/09/2026; một số có thể chỉ là khoá của `audit_log.delta` — kiểm lúc làm lớp C của từng service.

### Xung đột chờ chọn

Mỗi mục: **một khái niệm, hai tên tiếng Anh trở lên đang chạy thật**. Chọn một thì các tên còn lại
phải đổi — và tên nào đã trên dây thì đổi theo cửa sổ lớp C. Cột "Đề xuất" là ý kiến có lý do,
**không phải quyết định**.

| # | Khái niệm | Các tên đang dùng, và ở đâu | Đề xuất |
|---|---|---|---|
| **X1** | Phản ánh (`phieu_phan_anh`) | **`Petition`**: `@entity` (`service-petitions/migrations/0004_phieu_phan_anh.sql:214`), proto `PetitionStatusChanged`, `PetitionField` (platform), `petition_publication.go`, `PurposePetitionPhoto` = `petition-photo`, hành vi `update_petition_field`, `citizen-app` `PetitionCard`/`PetitionRating`, tên service `petitions` · **`CitizenReport`**: URL `citizen-reports` · `my-citizen-reports` · `citizen-report-fields` · `overdue-citizen-reports`, `CitizenReportMetric` (`summary_metrics.go`), `AutomationCitizenReport`, `web-admin` `citizen-report-blocks.tsx` · **`feedback`**: khoá quyền `feedback.*`, khoá lời hệ thống `feedback.*` (petitions 0020), `report.block.feedback`, `web-admin` `FeedbackDetailDrawer`/`FeedbackDraftStore` | `citizen_report` cho định danh và bảng — khớp **bề mặt duy nhất không đổi được** (URL), và nhường `petition` (xem X2). Giá: đổi `@entity`, proto message, `petition-photo`, và **không** đổi tên service/sự kiện `petitions.*` (tên miền, ADR 0011). `feedback.*` giữ ở khoá quyền (X25) |
| **X2** | Đơn thư (`don_thu`) | **`petition`**: khoá quyền `petition.create` · `petition.read` nhóm "ĐƠN THƯ" (`service-identity/migrations/0001_init.sql:302-303`), `report.metric.register.petition_arrived` (reporting 0003) · **`citizen_letter`**: URL `citizen-letters` (§Tên tài nguyên trên URL), tên migration `0016_sla_citizen_letter_kind…` | `citizen_letter` — khớp URL. Nhưng chữ `petition` ở khoá quyền thì **đúng nghĩa đơn thư**, nên X1 chọn `petition` là để một chữ mang hai khái niệm |
| **X3** | Bộ phận (`bo_phan`) | **`org_unit`**: URL `org-units`, `OrgUnit*` (identity, proto), JSON `org_unit_id`/`org_unit_name` · **`department`**: JSON `department_id` (`service-identity/internal/http/can_bo.go:44`, `danh_ba_chon_nguoi.go:29`), `department_name` · **`unit`**: JSON `from_unit`/`to_unit`/`holding_unit` (documents), `lead_unit`/`unit` (petitions) | `org_unit`. §Tên tài nguyên trên URL đã bác `departments` có lý do (cây gồm Đảng uỷ, HĐND). `department_*` trên dây đổi theo lớp C |
| **X4** | Chủ tịch xã | **`president`**: JSON `escalate_president_hours` (`service-identity/internal/http/sla.go:132`) · **`chairman`**: giá trị `EscalationChairman = "chairman"` (`service-documents/internal/domain/automation.go:86`, `service-petitions/internal/domain/automation.go:102`). Kèm: `leader` (`is_leader`, `escalate_leader_hours`) cạnh `unit_head` (`EscalationUnitHead`) — **chưa rõ là một hay hai khái niệm** | `chairperson`? — cần chủ dự án; và cần xác nhận `leader` ≠ `unit_head` |
| **X5** | Xã | **`tenant`**: bảng `tenant`, `tenant_id`, `TenantProfile`/`GetTenantProfile` (`proto/vigov/platform/v1/platform.proto:153-167`), `@entity: TenantDisplayProfile` · **`commune`**: URL `communes` · `commune-profiles` · `commune-staff` · `commune-news`, `CitizenCommune` | `tenant` **chỉ** cho khoá cô lập và sổ đăng ký của `platform` (luật 1); `commune` cho mọi nghĩa nghiệp vụ. Hồ sơ hiển thị đang mang **ba** tên (`TenantDisplayProfile` · `TenantProfile` · `commune-profiles`) — chọn một |
| **X6** | Dự án đầu tư (`du_an`) | **`Project`**: `@entity` (`service-finance/migrations/0004_du_an_va_chung_tu_giai_ngan.sql:171`), JSON `project_id` · **`investment_project`**: URL `investment-projects` | Bảng `investment_project`. §Tên tài nguyên trên URL giữ `@entity: Project` **chỉ vì** sửa migration đã áp là lệch checksum — lý do ấy hết khi đổi bằng migration mới |
| **X7** | Chứng từ giải ngân | **`DisbursementVoucher`**: `@entity` (`0004…:258`) · **`disbursement`**: URL `disbursements`, `…/{id}/confirmation`, `…/lockout` | `disbursement_voucher` cho bảng (một dòng là một **chứng từ**), `disbursement` giữ trên URL |
| **X8** | Thời hạn xử lý | **`sla`**: bảng `sla`, kiểu Go `SLA`, URL `sla` · **`ProcessingDeadline`**: `@entity` (`service-identity/migrations/0008_sla.sql:146`) | Giữ `sla`; bỏ `ProcessingDeadline` |
| **X9** | Kết luận họp | **`MeetingConclusion`** (`@entity`, `0007_bien_ban.sql:270`) · **`conclusions`** (URL) | `meeting_conclusion`; URL giữ |
| **X10** | Đề nghị lùi hạn | **`TaskExtensionRequest`** (`@entity`, `0006_nhiem_vu.sql:555`) · **`extension`** (URL `extensions`, `task-extensions`) | `task_extension_request` |
| **X11** | Đợt thu chi | **`BudgetEntry`** (`@entity`, URL `budget-entries`) · "batch" (`service-finance/internal/http/dot_thu_chi.go:169`: *"soft deletes one batch"*) | `budget_entry` |
| **X12** | Chi (ngân sách) | **`expenditure`**: JSON `expenditure_achievement` (`service-finance/internal/http/thu_chi_ngan_sach.go:200`) · **`expense`**: khoá `report.metric.fiscal.expense_percent` · `expense_amount` (reporting 0003:84) | `expenditure` — thuật ngữ ngân sách nhà nước; đổi hai khoá lời hệ thống |
| **X13** | Cờ bool | `is_active` (`map_field_schema`) · `active` (`petition_field`, JSON `active` ở identity) · `is_enabled` (`mail_settings`) · `enabled` (`automation_job_setting`, `nhan_linh_vuc`) | Cột: `is_…`; JSON giữ như đang chạy. Bảng tiếng Anh đã có **không** đổi |
| **X14** | Dạng giá trị enum tiếng Anh | snake: `sla_reminders`, `configuration_missing`, `in_progress`, `on_time` · kebab: `citizen-media`, `task-attachment`, `petition-photo` | **kebab-case** — số đông giá trị cũ, và bộ kiểm mã danh mục (`ChuanHoaMaDanhMuc`) chỉ nhận `a-z0-9-`. Tập snake đã có giữ nguyên |
| **X15** | Tiếp nhận | **`receipt`** (§Tên tài nguyên trên URL) · **`acknowledge`** (JSON `acknowledge_due`, `acknowledge_hours`) · **`received`** (sự kiện `petitions.received.v1`, chỉ số `received`) | Hai khái niệm, giữ hai từ: `received` = phần mềm ghi nhận phiếu; `acknowledge` = cán bộ đọc phiếu (`han_tiep_nhan`). Cần chủ dự án xác nhận `receipt` bỏ đi |
| **X16** | Phiên cán bộ (`phien`) | `session` (URL `sessions`) · cạnh `citizen_session`, `operator_session` | Bảng `staff_session` — ba lớp tin cậy, ba bảng cùng khuôn |
| **X17** | Cán bộ (`nguoi_dung`) | `staff` (URL, proto `Staff`) · `user` (khoá quyền `admin.user`, `admin.user.delete`) | Bảng `staff`; khoá quyền giữ (X25) |
| **X18** | Mã danh mục (`ma` của bảy danh mục, mã lĩnh vực tầng 1 `rac-thai`…) | — | **Ngoài phạm vi.** Mã đã cấp không đổi (luật 7 bất biến 3); ADR 0060 ghi mã tầng 1 không bao giờ đổi; mã tầng 1 của danh mục ba tầng do **xã tự gõ** — dịch nó là dịch dữ liệu của xã |
| **X19** | Thuật ngữ pháp lý | `khieu-nai` · `to-cao` · `kien-nghi` (loại đơn thư, chưa có bảng), `thu_ly`, độ khẩn `hoa-toc`, `thuong_tru`/`tam_tru`, các vai trò cột `du-toan-*` | ADR 0011 dặn không dịch vì dịch sai là **sai thủ tục**. Đề xuất: chỉ đổi sau khi người nắm thủ tục duyệt từng từ; ADR 0061 đã để đề xuất tạm |
| **X20** | Chín mã trạng thái phiếu | Khách **duyệt nguyên văn** ngày 20/09/2026 (mục §Chín trạng thái) | Câu quản trị: ai báo khách, và khách có cần duyệt lại chín chuỗi mới |
| **X21** | `UPDATE` giá trị trên hồ sơ đã đóng / đã khoá | Trigger chặn sửa chứng từ đã khoá (`chung_tu_da_khoa`), văn bản đã vào sổ (`so_van_ban_bat_bien`); luật 7 cấm #5 | (a) `UPDATE` bằng migration có vết hệ thống cho từng dòng (luật 6 bất biến 6), hàm trigger nhận cả hai giá trị trong migration ấy; hoặc (b) **không** `UPDATE`, giữ giá trị cũ trong dòng cũ và ánh xạ lúc đọc, như bảng chỉ-thêm. Cần chủ dự án chọn |
| **X22** | Giá trị `audit_log.action` | Hằng `HanhVi*` tiếng Việt cạnh `Action*` tiếng Anh | Đổi cho mục **mới** (ADR 0061 bảng hành vi); bộ lọc `?action=` nhận cả hai |
| **X23** | Bảng sổ `schema_migration` (`ten`, `ap_dung_luc`, `thoi_luong_ms` — `core/migrate/migrate.go:224-229`) | — | **Giữ.** `core/migrate` tạo và đọc nó trước migration đầu tiên; đổi tên cần mã khởi động tự nhận hai hình dạng — rủi ro trên đường mọi service khởi động, không có giá trị nghiệp vụ |
| **X24** | Route màn hình Mini App (`citizen-app`) | — | Giữ như route `web-admin` (ADR 0051); người dùng chỉ nói route `web-admin` |
| **X25** | Khoá quyền `feedback.*`, `petition.*`, `admin.user` | Đã tiếng Anh nhưng lệch X1, X2, X17 | **Ngoài phạm vi** trừ khi chủ dự án chọn khác: đổi là migration trên `quyen`/`vai_tro_quyen` đang cấp cho vai trò thật, và luật 5 bất biến 3c cấm khoá không có trong bảng |

## Quy ước đặt tên

> **28/09/2026 → ADR 0051:** hai dòng "Bảng, cột" và "Kiểu Go" dưới đây, cùng câu *"tên bảng dùng
> tiếng Việt"*, chỉ còn đúng cho mã và bảng **đã có**. Định danh, tệp, bảng và cột **mới** viết tiếng
> Anh — `kb/10-decisions/0051-english-code-identifiers.md`. Giá trị enum vẫn tiếng Việt không dấu.
>
> **29/09/2026 → ADR 0061:** cả bảng này đang được đổi — tên cũ sang tiếng Anh theo §Từ điển đổi
> tên ngay trên, giá trị enum theo ADR 0061 §Ánh xạ giá trị enum. Bảng dưới còn đúng cho service
> **chưa** qua lớp tương ứng của đợt đổi tên.

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
