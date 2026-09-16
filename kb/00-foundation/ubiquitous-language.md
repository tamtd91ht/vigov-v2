---
id: ubiquitous-language
tier: T0
source: CURATED
owner: domain
derived_from_commit: 5887496
expires: null
owns_facts:
  - "ánh xạ thuật ngữ hành chính sang tên dùng trong mã"
  - "ánh xạ khái niệm nghiệp vụ sang danh từ tài nguyên trên URL"
  - "vì sao khoá quyền feedback.* lệch với tài nguyên citizen-reports"
  - "phân biệt phản ánh, khiếu nại, tố cáo"
  - "tên gọi các bước trong vòng đời phiếu phản ánh"
  - "tên gọi vai trò cán bộ cấp xã"
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
| Hồ sơ một cửa | `ho_so_mot_cua` | Không viết "1 cửa" |
| Đơn thư | `don_thu` | Bao gồm khiếu nại, tố cáo, kiến nghị |
| Giải ngân | `giai_ngan` | |
| Nhiệm vụ | `nhiem_vu` | Việc giao cho cán bộ |
| Cán bộ, công chức | `can_bo` | Người dùng nội bộ |
| Công dân | `cong_dan` | Người dân — **không** gọi là "khách hàng", "user" |
| Đơn vị hành chính | `don_vi_hanh_chinh` | Xã / phường / thị trấn |

## Phản ánh và SLA

Vòng đời đầy đủ ở `skills/petition-lifecycle`; bảng này chỉ chốt **từ dùng**.

| Tiếng Việt hành chính | Tên trong mã | Ghi chú |
|---|---|---|
| Phiếu phản ánh | `phieu_phan_anh` | Một lượt phản ánh, có **mã tra cứu** trả cho dân |
| Mã tra cứu | `ma_tra_cuu` | Không tuần tự, không tái cấp (luật 4, 7) |
| Lĩnh vực | `linh_vuc` | Rác thải, giao thông, trật tự đô thị… — **cấu hình theo xã** |
| Tiếp nhận | `tiep_nhan` | **Mốc bắt đầu đếm hạn** — không phải lúc phân công |
| Phân loại | `phan_loai` | Cán bộ xác định lĩnh vực — **không** để dân tự chọn |
| Phân công | `phan_cong` | Giao cán bộ/đơn vị xử lý |
| Nghiệm thu | `nghiem_thu` | Xác nhận đã xử lý trên thực địa, thường kèm ảnh |
| Đóng phiếu | `dong_phieu` | Kết thúc — **bắt buộc có kết quả dân đọc được** |
| Hạn xử lý | `sla_deadline` | Lưu **một lần** lúc tiếp nhận, tính bằng **giờ làm việc** |
| Giờ làm việc | `gio_lam_viec` | Theo `lich_lam_viec` + `ngay_nghi_le` của xã — ADR 0007 |
| Quá hạn | — | **Suy ra**, không có cột. Xem luật 10 |

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

| Khái niệm | Tên trong mã | Tài nguyên URL | Vì sao không phải từ dễ đoán |
|---|---|---|---|
| Phản ánh | `phan_anh` | `citizen-reports` | "feedback" nghĩa là góp ý sản phẩm; phản ánh là **một loại đơn có thủ tục hành chính** |
| Đơn thư | `don_thu` | `citizen-letters` + `?type=` **bắt buộc** | Một sổ, **một dãy số liên tục**; tách thành nhiều đường dẫn là tách dãy số của hồ sơ lưu trữ |
| Văn bản đến | `van_ban_den` | `incoming-documents` | |
| Văn bản đi | `van_ban_di` | `outgoing-documents` | |
| Nhiệm vụ | `nhiem_vu` | `tasks` | |
| Giải ngân | `giai_ngan` | `disbursements` | |
| Phiên đăng nhập | `phien` | `sessions` | |
| Bộ phận | `bo_phan` | `org-units` | Cây này chứa Đảng uỷ, HĐND, UBMTTQ — **không phải** phòng ban của UBND, nên không gọi `departments` |
| Thông báo | `thong_bao` | `announcements` · `public-notices` · `notifications` | **Một từ tiếng Việt, ba thứ khác nhau, ba nhóm người đọc.** Gộp một danh từ thì thông báo nội bộ chạy sang kênh công dân |
| Tiếp nhận | `tiep_nhan` | `receipt` | |
| Thụ lý | `thu_ly` | `admission` | Thụ lý **bắt đầu đồng hồ luật định**; tiếp nhận thì không. Gọi cả hai là `accept` là xoá mất ranh giới đó |
| Nghiệm thu | `nghiem_thu` | `verification` | Với phiếu phản ánh đây là kiểm tra thực địa kèm ảnh, không phải nghiệm thu công trình có hội đồng (`acceptance`) |
| Đóng phiếu | `dong_phieu` | `closure` | |
| Cấp số văn bản | `so_di` | `number` | |

**Bảng này còn thiếu.** Nó mới phủ các khái niệm đã xuất hiện trong mã hoặc trong
`.claude/skills/rest-api-design/SKILL.md`. Gặp khái niệm chưa có dòng ở đây: **dừng lại và
hỏi**, đừng tự dịch rồi viết route — đường dẫn không sửa lại được sau khi một xã chạy thật.

### Khoá quyền `feedback.*` lệch với tài nguyên `citizen-reports` — cố ý

Đây là chỗ duy nhất ghi lệch này. Nơi khác **liên kết tới đây**, không chép lại.

| Bề mặt | Dùng | Trạng thái |
|---|---|---|
| Khoá quyền | `feedback.assign` `feedback.create` `feedback.read` `feedback.resolve` `feedback.restricted` | `services/identity/migrations/0001_init.sql` đã nạp; ADR 0008 đã chốt `feedback.resolve` quyết định ai đóng phiếu |
| Tài nguyên URL | `citizen-reports` | ADR 0011 |

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

→ Kỹ năng: `skills/administrative-language` · `.claude/skills/rest-api-design/SKILL.md`
→ Ranh giới ngôn ngữ của hợp đồng: `kb/10-decisions/0011-contract-surface-language.md`
