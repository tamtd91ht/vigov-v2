---
id: doi-chieu-2026-09-23-feat-m8-multitenant-foundation
tier: T2
source: CURATED
owner: architecture
derived_from_commit: 86a7c99
expires: null
kho_nguon: vigov-require
branch: feat/m8-multitenant-foundation
sha_tu: 93cff7f
sha_den: b159f0e
ngay_review: 2026-09-23
anh_huong: [service-petitions, service-finance]
owns_facts:
  - "khoảng 93cff7f..b159f0e của ../vigov-require đổi luật nghiệp vụ nào, và phân hệ nào của kho này phải đổi theo"
  - "vì sao lần đối chiếu này KHÔNG đưa service-identity, service-comms, platform-admin, citizen-app vào anh_huong"
---

# Đối chiếu `feat/m8-multitenant-foundation` · 23/09/2026

`93cff7f` → `b159f0e` · 59 commit · 06/09/2026 → 21/09/2026 · 255 tệp · +26267/−2209 · đọc ngày 23/09/2026

**Đây là branch BA/PM thật sự làm.** `origin/main` bên kia đứng yên từ 06/09/2026, nên `sha_tu`
lấy ở `origin/main` và toàn bộ 59 commit nằm trên nhánh này.

**Lần đọc này đọc COMMIT, không đọc lại `docs/spec`.** `kb/90-ephemeral/doi-chieu-vigov-require.md`
đã đo trên đúng `b159f0e` ngày 22/09/2026 và đã tiêu thụ hết `docs/spec/` — nhưng nó **chỉ đọc
tài liệu** (§X1: *"Đọc MÃ vigov-require — chưa làm"*). Ghi chú này là phần nó còn nợ: migration
`0035`…`0049` và mã, tức những luật nghiệp vụ **đổi trong khoảng này** chứ không phải toàn cảnh.

## Tóm tắt — kho này phải làm gì

| # | Bên kia đổi | Kho này chạm | Module |
|---|---|---|---|
| 1 | **Luật nắm giữ**: người được giao đích danh một phiếu đổi được trạng thái / ghi nhật ký / đính ảnh / chuyển tiếp **dù vai trò không có `feedback.resolve`** — `75a9ea9` | **MÂU THUẪN** với câu #7 (khách chốt 16/09) và ADR 0030. Xem §Mâu thuẫn — **đừng viết theo bản nào trước khi có người quyết** | service-petitions |
| 2 | Đổi trạng thái và **bàn giao gộp thành MỘT thao tác**: mỗi lần đổi trạng thái đều mang bộ phận + người tiếp nhận + ghi chú — `c403ee6` | `Đường xử lý phản ánh phía CÁN BỘ` đang `dang_lam`: tuyến chuyển trạng thái hiện tách rời tuyến phân công. **Sổ văn bản đã ở dạng ấy rồi** (thu hẹp 23/09 lần 2): một lượt bấm `Chuyển và ghi vết` gửi trọn `{denBoPhan, canBoXuLy, lyDo}` (`web-admin/src/features/van-ban/nhan-van-ban.ts:204-208` · `thao-tac-van-ban.ts:34-41`) | service-petitions |
| 3 | `feedback_events.assignee_id` — dòng thời gian ghi **giao cho AI ở bước ấy**, kèm tệp đính kèm — `92a722e` + migration `0040` | Vết của phiếu hiện chỉ ghi bộ phận. `assignee_id` của phiếu là *người giữ BÂY GIỜ*, không suy ra được người của từng bước | service-petitions |
| 4 | Mỗi **khoản chi chọn rút từ nguồn vốn nào**; một dự án lấy tiền từ **nhiều nguồn** (`budget_item_sources`) — `ddef1ab` + migration `0041` | `chung_tu_giai_ngan` ở `migrations/0004` **không có cột nguồn vốn**, và sáu tuyến GHI chứng từ đang `dang_lam` — sửa lược đồ bây giờ rẻ hơn sau khi có dữ liệu thật | service-finance |
| 5 | **Sửa và gỡ chứng từ chi**: sửa một chứng từ **đã xác nhận** thì đưa nó **về nháp**; chứng từ **đã khoá** không hiện nút nào — `c3f4d6a` | Đúng vùng sáu tuyến `nhập · sửa · gỡ · xác nhận · khoá · mở khoá` đang `dang_lam`. Câu #29/#30 **không nói** vế "sửa bản đã xác nhận thì về nháp" | service-finance |
| 6 | Trạng thái đầu của một đơn thư: **"Mới vào sổ"** khi chưa ai giữ, **"Đã phân công"** khi giao ngay lúc vào sổ — `e3d43d2` | **Không còn việc nào** (thu hẹp 23/09 lần 2). Lược đồ đã có cả sáu mã (`0004_so_van_ban.sql:364,530`); `web-admin` đã hiện đúng nhãn **và đã dùng chữ *giải quyết*** (`nhan-van-ban.ts:47-50`). Nửa "hai bộ nhãn cho hai sổ" **chưa tới lượt**: sổ đơn thư chưa dựng ở cả hai phía | — (lượt sau) |
| 7 | **Dòng cha LUÔN cộng từ dòng con**, chặn ở tầng nghiệp vụ chứ không chỉ giao diện (anh Hà chốt 06/09) — `502d6f4`; và **một dòng được người đánh dấu là "con số tổng"** (`is_headline`) — `0142452` + migration `0035` | **Hai luật ĐÃ KHỚP ở lược đồ**: `service-finance/migrations/0006_thu_chi_ngan_sach.sql:332,345` (`cach_tinh` + CHECK `'manual'/'children'`) và `:334` (`is_headline`). Còn mở: **tuyến GHI có cưỡng chế `cach_tinh='children'` ở tầng nghiệp vụ hay chưa** | service-finance |
| 8 | Bàn giao ghi **từ bộ phận nào sang bộ phận nào, từ người nào sang người nào** (`handover_from_*`, `handover_to_*`) — migration `0043` | **`service-documents` KHÔNG thiếu gì so với đặc tả của chính nó** (thu hẹp 23/09 lần 2 — xem §Đã sửa): năm dòng của `docs/ui-ux/05-van-ban-don-thu.md:149-153` ánh xạ đủ vào `lich_su_chuyen_van_ban`, và *người giao trước* **suy được** từ dòng liền trước. Thiếu thật **chỉ ở `service-petitions`**: chưa có bảng vết nào (`0005_duong_xu_ly_phan_anh.sql:34-36`) | service-petitions |

## Chi tiết theo phân hệ

### M4 — Phản ánh của người dân

| Commit | Tệp bên kia | Đổi gì | Hệ quả ở kho này |
|---|---|---|---|
| `75a9ea9` | `apps/api/app/modules/feedback/service.py` (`_may_work_on`) · `router.py` | Năm tuyến hạ từ `feedback.resolve` xuống `feedback.read`; tầng nghiệp vụ cho qua nếu `entry.assignee_id == principal.user_id`. `_apply_handover` cũng bỏ qua `feedback.assign` cho người đang giữ | **ĐIỀU KIỆN DỪNG — xem §Mâu thuẫn.** Không mở việc theo mục này |
| `c403ee6` | `feedback/service.py` · `feedback/router.py` · `apps/api/app/tests/test_status_handover.py` | Một lần bấm trạng thái ghi trọn quyết định: trạng thái + bộ phận + người + ghi chú, trong cùng một thao tác | Tuyến chuyển trạng thái của `service-petitions` phải nhận đủ bốn thứ ấy trong **một** lần gọi. Tách hai lần gọi là để lại khoảng trống nơi phiếu đổi trạng thái mà không ai giữ |
| `92a722e` | `feedback/models.py` · migration `20260907_0040-0040_feedback_event_who.py` | `FeedbackEvent.assignee_id` + tệp đính kèm cho từng dòng nhật ký. Lý do ghi tại chỗ: `assignee_id` của phiếu là người giữ *bây giờ*, khác người mà bước ấy đã giao | Bảng vết phiếu phản ánh cần cột người-được-giao-ở-bước-này. Không có thì câu *"ai giữ phiếu lúc quá hạn"* không trả lời được |
| `97ef30d` | `feedback/service.py` | Ghi chú làm việc là **nội bộ**, và màn hình **nói rõ trên màn** là nó nội bộ | Luật 4 cấm #5 đã cấm lộ ghi chú nội bộ cho công dân; cái mới là **nhãn hiện cho chính cán bộ** biết dòng này dân không đọc được. Nhỏ, nhưng là thứ quyết định cán bộ dám viết gì |
| `ba19ab0` | `apps/api/app/core/scope.py` · `feedback/repository.py` | Hai bộ lọc "việc trên bàn tôi": theo người, theo bộ phận — dùng chung cho nhiệm vụ, văn bản, phiếu | Tuyến danh sách phiếu phía cán bộ cần hai bộ lọc ấy. Đây là màn hình cán bộ mở đầu ngày |
| `333c929` | `apps/admin/src/components/feedback/FeedbackDetailDrawer.tsx` (+2 tệp `documents/`) | Ô chọn người xử lý **lọc theo bộ phận** ở mọi chỗ nó xuất hiện | **Chưa áp được, và kho này đã ghi lý do tại chỗ** (thu hẹp 23/09 lần 2): ô người xử lý ở sổ văn bản là **ô CHỮ, không phải ô chọn** — `web-admin/src/features/van-ban/so-van-ban-den.tsx:978-987` giải thích vì sao: hợp đồng nhận **mã cán bộ**, còn danh bạ cán bộ đòi khoá `admin.user`, nên *"một người có `document.route` chưa chắc đọc được danh bạ; vẽ một ô chọn rỗng cho họ là vẽ một ô không bao giờ dùng được"*. Lọc theo bộ phận chỉ có nghĩa **sau khi** câu quyền ấy được trả lời — đó là câu hỏi, không phải việc |

**Bộ chín trạng thái KHÔNG đổi trong khoảng này** — đã kiểm `feedback/models.py` và toàn bộ diff
của `apps/api/app/modules/feedback/`: không dòng nào thêm/bớt/đổi tên một mã trạng thái. ADR 0027
không bị khoảng commit này chạm tới.

### M2 — Văn bản đến và đơn thư

| Commit | Tệp bên kia | Đổi gì | Hệ quả ở kho này |
|---|---|---|---|
| `e3d43d2` | `apps/admin/src/lib/document-display.ts` · `components/documents/PetitionTable.tsx` | Đơn thư có **nhãn riêng**, thôi mượn nhãn của văn bản: `Mới vào sổ` (chưa ai giữ) · `Đã phân công` (giao ngay lúc vào sổ). Dùng *giải quyết* thay *xử lý*, *chuyển cấp trên* thay *chuyển đơn vị khác* | **KHÔNG CÒN VIỆC NÀO** (thu hẹp 23/09 lần 2 — xem §Đã sửa). Lược đồ: `0004_so_van_ban.sql:364` và `:530` có đủ sáu mã, mặc định `'moi-vao-so'` (`:342`). Giao diện: `web-admin/src/features/van-ban/nhan-van-ban.ts:47-50` đã ánh xạ `moi-vao-so`→`Mới vào sổ`, `da-phan-cong`→`Đã phân công`, và **đã dùng `da-giai-quyet`→`Đã giải quyết`**, tức chữ *giải quyết* bên kia vừa đổi sang thì kho này đã dùng sẵn; mã lạ còn có đường lui *"chưa có nhãn"* (`:82`). Nửa **"hai bộ nhãn cho hai sổ"** chưa tới lượt: **sổ đơn thư chưa dựng ở đâu cả** — không bảng `don_thu` trong `service-documents/migrations/**`, không tuyến nào trong `kb/20-contracts/openapi.json`, và `web-admin/src/app/van-ban/page.tsx:21-23` nói thẳng vì sao nó cố ý chưa vẽ tab ấy. Đây là **đầu vào thiết kế cho lượt dựng đơn thư**, không phải việc đang chờ |
| `809460b` | `components/documents/PetitionDetailDrawer.tsx` · `TaskFromRecordDialog.tsx` | Bấm trạng thái mở hộp soạn mang bộ phận + cán bộ + ghi chú. **"Chuyển thành nhiệm vụ" không tạo ngay nữa**: hiện hộp xác nhận (tiêu đề, bộ phận, cán bộ, ưu tiên, hạn thừa kế) rồi mới tạo — *"đúng ba trên bốn lần; lần thứ tư để lại một nhiệm vụ sai tiêu đề trong một sổ không xoá được, chỉ thu hồi được"* | Đúng luật 7 của kho này: sổ nghiệp vụ không xoá cứng, nên một bản ghi tạo nhầm là một bản ghi sống mãi. Tuyến "đơn thư → nhiệm vụ" phải có bước xác nhận, không tạo thẳng |
| `000853a` | `components/documents/DocumentDetailDrawer.tsx` · `PetitionDetailDrawer.tsx` | Văn bản và đơn thư nhận **hình dạng màn nhiệm vụ**: hai cột, một dòng nhật ký nhận tệp | Bản mẫu của màn `Văn bản & Đơn thư` mà `web-admin` vừa dựng đã đổi hình |
| `c5b0c1b` | `components/documents/DocumentWorkspace.tsx` · `hooks/api/useExcelImport.ts` | Nhập đơn thư từ Excel theo mẫu tải về. Luật đi kèm (`docs/spec/05-nghiep-vu.md` §M2): **kiểm cả tệp trước khi ghi — một dòng sai thì không ghi dòng nào** | Chưa tới lượt ở kho này, nhưng luật "tất cả hoặc không gì" phải nằm sẵn trong thiết kế tuyến nhập: nhập nửa vời vào một sổ không xoá cứng được là thứ không gỡ lại được |
| migration `0043` | `20260910_0043-0043_progress_handover.py` | Dòng nhật ký mang luôn cú bàn giao: `handover_from_*` / `handover_to_*` (bộ phận và người) | **`service-documents` KHÔNG thiếu cột nào** (thu hẹp 23/09 lần 2). Đặc tả kho này — `docs/ui-ux/05-van-ban-don-thu.md:149-153` — vẽ đúng năm dòng, và lược đồ có đủ cả năm: `nguoi_ma` (`0004_so_van_ban.sql:509`) · `trang_thai_tai_thoi_diem` (`:513`) · `tu_bo_phan_id`→`den_bo_phan_id` (`:517,518`) · `can_bo_xu_ly_ma` (`:520`) · `noi_dung` (`:524`). Chú thích `:515` và `:519` **trích thẳng** hai dòng ấy của đặc tả — bảng này được dựng TỪ đặc tả đó. *Người giao trước* **suy được**: nó là `can_bo_xu_ly_ma` của dòng liền trước cùng `van_ban_den_id`. Thêm cột `tu_can_bo_ma` là nuôi bản sao thứ hai của một dữ kiện đã có — đúng thứ luật 9 (phép thử một dòng) và luật 10 bất biến 3 cấm. **Thiếu thật chỉ ở `service-petitions`**: `0005_duong_xu_ly_phan_anh.sql:34-36` khai rõ `nhat_ky_phan_anh` là *"a separate pass. Nothing below references it"* — chưa có bảng vết nào |

### M3 — Giải ngân và thu-chi ngân sách

| Commit | Tệp bên kia | Đổi gì | Hệ quả ở kho này |
|---|---|---|---|
| `ddef1ab` · migration `0041` | `20260907_0041-0041_item_funding_split.py` | Bảng `budget_item_sources` (một dự án ↔ nhiều nguồn vốn, mỗi nguồn một số tiền) và cột `disbursements.source_id` | `service-finance/migrations/0004` chỉ có `chung_tu_giai_ngan.du_an_id`. Đây đúng là mục **K4** của bản đối chiếu toàn cảnh, nay đã có bằng chứng bằng migration. Sáu tuyến GHI chứng từ đang `dang_lam` — thêm cột trước khi có chứng từ thật rẻ hơn hẳn sau |
| `c3f4d6a` | `components/budget/DisbursementForm.tsx` · `BudgetItemDetail.tsx` | Mỗi dòng chứng từ có **Sửa** và **Gỡ**. Chứng từ **đã khoá**: không hiện nút nào, máy chủ từ chối. **Sửa một chứng từ đã xác nhận thì nó về nháp** — *"lãnh đạo xác nhận những con số kia, không phải những con số này"*. Gỡ là xoá mềm, chứng từ rời khỏi mọi tổng và **ở lại trong sổ** | Câu #29 và #30 của kho này nói về **mở khoá** và **chứng từ hoàn**, **không** nói vế "sửa bản đã xác nhận". Quy tắc này lấp đúng khoảng trống ấy và không chống lại #29/#30. Đưa vào thiết kế sáu tuyến đang làm; xoá mềm đã đúng luật 7 |
| `03c1787` | `apps/admin/src` (m3) | Tiến độ giải ngân vẽ **từ ngày khởi công tới ngày hoàn thành dự kiến**, không phải tháng 1→12 | Cùng vùng câu mở #31 (`nguong_canh_bao_cham`). Mốc so sánh của "chậm" đổi thì ngưỡng cảnh báo đo trên nền khác |
| `502d6f4` | `components/budget/FiscalReportPanel.tsx` · `docs/open-questions.md` (Q-22) | **Anh Hà chốt 06/09/2026: khoản mục cha không cho gõ thẳng, luôn cộng từ dòng con.** Chặn ở tầng nghiệp vụ. Bên kia ghi thẳng cái giá: mẫu biểu thật có chỗ dòng cha ≠ tổng dòng con (khoản mục ngoài cân đối, dòng *"Trong đó:"*), ở đó **số trên màn hình sẽ khác bản giấy xã đã ký** | **Thu-chi ở kho này ĐÃ DỰNG, và lược đồ ĐÃ KHỚP luật này** (sửa 23/09 — xem §Đã sửa): `service-finance/migrations/0006_thu_chi_ngan_sach.sql:332` `cach_tinh TEXT NOT NULL DEFAULT 'manual'` + `:345` `CHECK (cach_tinh IN ('manual','children'))`, cùng `cha_id` (`:328`) và `cap` (`:333`). **Phần còn mở là tuyến GHI**: đã có nơi nào cưỡng chế `cach_tinh='children'` ở tầng nghiệp vụ chưa — tức dòng cha có thật sự bị từ chối khi gõ số thẳng không. Đó mới là câu chưa trả lời |
| `0142452` · migration `0035` | `20260906_0035-0035_fiscal_headline.py` | Cột `fiscal_lines.is_headline` — **người chọn** dòng nào là con số tổng, máy không đoán bừa. Sheet thu có hai dòng cấp cao nhất lồng nhau; sheet chi có "Tổng số" đứng ngang hàng A…E — cộng hết là đếm đôi | **Đã khớp** (sửa 23/09): `service-finance/migrations/0006_thu_chi_ngan_sach.sql:334` `is_headline BOOLEAN NOT NULL DEFAULT false`, và tệp ghi thẳng *"THE HEADLINE ROW IS CHOSEN BY A PERSON"* (`:38`) — cùng một lập luận, không nạp sẵn giá trị. Kèm theo, luật đơn vị tính đã ở `bang_ngan_sach.don_vi_tinh` (`:167`) và nhiều kỳ trong năm ở `bang_ngan_sach.lan` (`:160`). Không có việc đổi lược đồ nào phát sinh từ hai commit này |

### M1 — Nhiệm vụ

Kho này **chưa có bảng nhiệm vụ nào** (`service-petitions` mới có hai danh mục `task-types` /
`task-priorities`). Ba mục dưới đây là **đầu vào thiết kế**, không phải việc sửa mã đang có.

| Commit | Tệp bên kia | Đổi gì | Hệ quả ở kho này |
|---|---|---|---|
| `20372e0` · migration `0042` | `20260909_0042-0042_task_assigner.py` | `tasks.assigner_id` — **lãnh đạo giao việc** là người duyệt lùi hạn, không phải `created_by`. *"Đúng khi lãnh đạo tự nhập, sai phần còn lại: văn thư nhập hộ phần lớn nhiệm vụ, và đề nghị lùi hạn nằm lại trên bàn văn thư."* `Chờ duyệt lùi hạn` là **NHÃN**, không phải trạng thái | Cùng hình dạng với mục §Mâu thuẫn: quyền duyệt gắn vào **một người ghi trên bản ghi**, không vào một khoá quyền. Luật 5 bất biến 3b của kho này có `task.approve` và `task.extend` là hai khoá rời — nhưng **ai** cầm `task.approve` cho **nhiệm vụ nào** thì chưa có lời giải |
| migration `0042` | cùng tệp | `sla_rules.warn_before_hours` mặc định 24 → **72 giờ**, và migration **chỉ ghi đè xã còn để nguyên mặc định cũ** — *"xã đã tự đặt con số khác thì đó là quyết định của họ"* | **Ngưỡng cảnh báo sớm KHO NÀY ĐÃ CÓ** (sửa 23/09 — xem §Đã sửa): `service-identity/migrations/0008_sla.sql:178` `gio_sap_den_han INTEGER NOT NULL`, chú thích tại chỗ *"hours REMAINING at which «sắp đến hạn» begins"*. Theo `tenant_id` + `loai_viec` + `linh_vuc` (`:218`), nằm trong ràng buộc `sla_gio_phai_duong` (`:244`), và `loai_viec` đã nhận `'nhiem-vu'` sẵn (`:223`). **Không phải việc thêm cột.** Thứ còn đáng mượn là **hình dạng migration**: một lần nâng cấp mặc định **không được đè** lên xã đã tự khai con số khác |
| `9401323` | `apps/admin/src` (m1) | Bộ lọc **"sắp đến hạn"** đọc đúng `warn_before_hours` của xã — một con số dùng chung cho bản tin nhắc, ô lọc, số trên chuông và Zalo. Có test riêng để một hằng số chôn trong mã không lọt | Nguồn của vế *sắp* quá hạn ở kho này **đã có** và là `sla.gio_sap_den_han` (`service-identity/migrations/0008_sla.sql:178`). Cái đáng mượn là **luật một-ngưỡng-không-hai**: mọi nơi hiển thị "sắp đến hạn" phải đọc đúng cột ấy, không được có hằng số thứ hai trong mã |

### M8 / M6 — Quản trị, danh bạ, kênh thông báo

Không mục nào trong nhóm này vào `anh_huong` — lý do ghi ở từng dòng.

| Commit | Tệp bên kia | Đổi gì | Hệ quả ở kho này |
|---|---|---|---|
| `7b689df` · migration `0036` | `20260906_0036-0036_staff_org_unit.py` | **Anh Hà chốt 06/09/2026: "Khối đơn vị" của danh bạ và bộ phận của sơ đồ tổ chức là MỘT.** `staff_contacts.org_unit_id`; sơ đồ tổ chức phải nhận thêm chi bộ thôn, trưởng thôn, thường trực Đảng uỷ. So tên bằng `unaccent(lower(...))` vì xã gõ hoa toàn bộ còn sơ đồ gõ thường | Kho này **đã ở dạng ấy**: `nguoi_dung.bo_phan_id` là khoá ngoại tới `bo_phan` (`service-identity/migrations/0001_init.sql:171,186`), không có trường khối tự do. Hệ quả duy nhất cần kiểm: cây `bo_phan` có nhận được khối **ngoài Uỷ ban** không. **Chưa rõ ảnh hưởng tới `khoi_nhiem_vu`** — `0005_don_vi_dan_cu_va_danh_muc.sql:313` đã ghi sẵn điều kiện đổi ý, nhưng "khối đơn vị của danh bạ" và "khối nhiệm vụ" có phải một khái niệm hay không thì **phải hỏi anh Hà**, không suy |
| `6a0f383` · `924dca8` · migration `0037`–`0039` | `apps/api/app/modules/notifications/announcements.py` · `admin/email_config.py` · `docs/adr/0003-gui-thu-dien-tu.md` | **Thông báo nội bộ** gửi tới cả bộ phận + hộp thư, có xác nhận đã đọc. Kênh thư điện tử qua SMTP của chính đơn vị (ADR 0003 bên kia), **mỗi xã khai máy chủ thư riêng trên màn hình** | Kho này **không có kênh thông báo nội bộ cán bộ nào** và `service-comms` mới có sổ thông báo gửi **công dân**. Đây là **câu hỏi phạm vi**, không phải việc đổi mã → không đưa `service-comms` vào `anh_huong`. Nếu sau này có: máy chủ thư theo xã là cấu hình đọc lúc chạy, **không phải biến môi trường** (luật 1 bất biến 10, luật 11 cấm #6) |
| `08fb982` | `apps/api/app/modules/admin/email_config.py` · `integrations/email/client.py` | Hai lỗi bảo mật trong mã viết trong tuần: (1) mật khẩu SMTP chỉ-ghi, nhưng **host/cổng/tài khoản sửa được trong cùng một yêu cầu**, rồi nút "gửi thử" xác thực tới host MỚI bằng mật khẩu CŨ — một cán bộ có quyền sửa cấu hình thư trỏ host về máy mình và nhận được mật khẩu hộp thư công vụ thật. (2) tuyến xem chi tiết thông báo **không khai quyền nào** | **Bẫy, chưa phải việc.** Cái (1) áp thẳng vào ADR 0009 (bí mật mã hoá theo xã): ngày kho này có màn khai bí mật theo xã kèm nút thử kết nối, đổi đích đến **phải buộc gõ lại bí mật**. Cái (2) đúng là luật 5 bất biến 2, `rbac_guard` của kho này canh sẵn |
| `4f9a53c` | `apps/api/app/integrations/` (m1) | Mã bot nằm **trong đường dẫn** URL, `httpx` ghi URL ở mức INFO → kênh nhắc việc tự ghi bí mật của mình vào log | Xác nhận mục **K2** của bản đối chiếu toàn cảnh bằng một commit thật. Phép kiểm cho `service-comms` khi nó gọi Zalo: luật 8 cấm bí mật trong log nhưng **không ai nghĩ tới đường vòng qua URL của thư viện HTTP** |
| `cee260f` · `bb70120` · migration `0046`–`0049` | `20260919_0046-0046_zalo_bot.py` … `0049_zalo_admin_link` | Kênh **Zalo Bot nhắc việc cán bộ**: bot nền tảng dùng chung, xã nào khai token riêng thì có webhook secret riêng, và *"mã ghép nối của xã bên cạnh bị từ chối như thể chưa từng tồn tại"*. Xã khai Zalo được phép ngắt lời mình vì việc gì, bao lâu một lần | **Kênh mới, chưa có trong phạm vi kho này** (ADR 0006/0018 nói về OA của xã cho **ZNS gửi công dân**, không nói bot nhắc **cán bộ**). Câu hỏi phạm vi → không vào `anh_huong`. Đáng lưu: cách họ cách ly xã ở tầng webhook secret là đúng tinh thần luật 1 bất biến 8 |

## Mâu thuẫn với quyết định đã chốt

**MỘT mâu thuẫn. Cần người quyết trước khi ai viết tuyến chuyển trạng thái phiếu phản ánh.**

### Luật nắm giữ — quyền theo BẢN GHI đặt cạnh quyền theo VAI TRÒ

**1 · Bên kia nói gì.** `75a9ea9` · `apps/api/app/modules/feedback/service.py`, hàm `_may_work_on`:

> *"Quyền `feedback.resolve` là quyền xử lý phiếu của cả xã. Người được giao đích danh một phiếu
> thì không cần quyền ấy: trưởng thôn nhận phiếu rác ở thôn mình phải cập nhật được nó, mà cho họ
> quyền toàn xã chỉ vì thế là mở rộng quá tay — họ sẽ đóng được cả phiếu của thôn bên cạnh."*

Năm tuyến (`/media`, `/media/upload`, `/progress`, `/log`, `/attachments/{id}`) hạ khai báo từ
`feedback.resolve` xuống `feedback.read`; điều kiện thật chuyển xuống tầng nghiệp vụ và đọc
`entry.assignee_id == principal.user_id`. `_apply_handover` làm đúng như vậy với `feedback.assign`.
Cùng hình dạng ở `20372e0` cho nhiệm vụ: người duyệt lùi hạn là `assigner_id` ghi trên bản ghi,
không phải người cầm một khoá quyền.

**2 · Kho này đã chốt gì.**

| Nguồn | Câu chốt |
|---|---|
| Câu hỏi mở **#7**, `DECIDED` **16/09/2026** (của khách, cùng đợt #6 và #8 — luật 10 ghi rõ ba câu này là quyết định của khách) | *"Quyền `feedback.resolve` quyết định ai đóng được."* Hai cờ cấu hình theo xã đi kèm, **không** có cửa nào cho người được giao |
| **ADR 0030** (20/09/2026) | Khoá quyền canh **hành vi**, và các quyền này **không phải tích Đề-các**. Thêm khoá là quyết định có cân nhắc, không suy ra |
| **Luật 5** bất biến 1 · 3b · 3c | Mỗi tuyến khai **đúng một khoá phẳng**; thiếu khai là **từ chối**. Không có trục phân quyền thứ hai trong mô hình |

**3 · Ai phải quyết.** Khách / chủ dự án. Đề xuất đã có sẵn và **không phải của phiên này**:
`kb/90-ephemeral/doi-chieu-vigov-require.md` §4.3 và mục **D1** đã đề nghị mang luật nắm giữ hỏi
khách **cùng câu #27**, vì hai câu là một câu. Khoảng commit này thêm được hai thứ D1 chưa có:
**mã thật** (`_may_work_on`) và **lý do vận hành thật** (*"phiếu giao cho trưởng thôn, mở ra thì
được báo là không có quyền — cả hai vế đều đúng, và phần mềm đúng luật mà vô dụng với công việc"*).

**Không tự chọn bên nào.** Chọn bên kia là lặng lẽ bỏ một câu khách đã chốt ngày 16/09. Chọn bên
này là giữ nguyên tình trạng *"giao việc cho người ta rồi chặn họ làm"*. Cả hai đều phải có chữ
của người quyết.

⚠ Rào `require_sync_guard` đang chặn `service-petitions` vì ghi chú này. **Mở việc cho mục này
không phải là "chọn một bên rồi code"** — việc cần mở là *hỏi*, và tuyến chuyển trạng thái phiếu
đang `dang_lam` phải dừng ở chỗ nào cần câu trả lời ấy.

## Không ảnh hưởng

Đã đọc, cố ý bỏ qua. Ghi ra để lần sau khỏi đọc lại đúng những commit này để kết luận y hệt.

| Commit / vùng | Vì sao bỏ |
|---|---|
| `427eca2` · `62c185f` · migration `0044` `citizen_dossiers` · `0045` `dossier_edit_perm` | **Hồ sơ một cửa — khách chốt NGOÀI phạm vi hợp đồng 20/09/2026** (ADR 0001 §Bổ sung). Vùng cố ý lệch. Thiết kế bên kia dùng lại được nguyên vẹn nếu khách đổi ý; đừng dựng lại |
| `a878889` · `d332a62` · `87fd00f` · `15ef02e` · `c6b2458` · `b159f0e` — toàn bộ `docs/spec/**` (+2051 dòng) và sổ tay người dùng | `docs/spec/` **sinh từ mô hình của họ** và đã được đo trọn vẹn trên đúng `b159f0e` ngày 22/09/2026 tại `kb/90-ephemeral/doi-chieu-vigov-require.md`. Đọc lại là đọc lại cùng một thứ. Sổ tay + ảnh chụp màn hình là tài liệu của kho họ |
| Mọi commit chỉ đổi `apps/api/app/modules/*/` theo kiểu Python — tách hàm, sửa truy vấn, `b522084` (bước pre-deploy), `1603054` (gỡ hook không ai gọi), `da38625` (`railway.json`) | **Kiến trúc cố ý lệch**: FastAPI monolith 16 module ↔ Go 7 vi dịch vụ. Đổi cách cài đặt, không đổi luật nghiệp vụ (SKILL §2 câu 1) |
| Đường dẫn `/api/v1/feedbacks`, `/api/v1/dossiers`, `/api/v1/users` trong mọi commit | **Đặt tên tài nguyên URL cố ý lệch** — kho này chốt `citizen-reports` · `staff` · `org-units` (ADR 0011 + `ubiquitous-language.md`). Đừng ánh xạ vội |
| `427eca2` phần tra cứu Mini App bằng **số hồ sơ + 4 số cuối điện thoại**; mọi chỗ đọc `X-Citizen-Id` / `X-Tenant-Code` | **Danh tính công dân và cách nhận xã cố ý lệch** — kho này phát phiên từ máy chủ và nhận xã từ `Host` ở rìa (luật 1 bất biến 3, luật 4 bất biến 2). Bên kia làm khác *có chủ ý* |
| `d906f22` (hộp thoại 56rem hoá ra 24rem) · `8ea6e88` (ô chọn giữ câu trả lời của bản ghi trước) · `155b0c1` (số vừa khung trên máy chiếu) · `ac835c7` · `5ded46b` · `71c84ea` · `65608ea` | Sửa lỗi và bố cục giao diện trong bản mẫu của họ. Không đổi luật nghiệp vụ |
| `4d37c97` (bản đệm cũ hơn mã thì tính lại) · `622acdf` (kỳ rỗng không phải kỳ hỏng) · `796c39a` · `61d5228` · `07e661f` | M7 báo cáo. `service-reporting` ở kho này **chưa dựng** và câu chặn nằm ở phía khách (sổ tiến độ `service-reporting.json`). Ba luật đáng mượn (`None` ≠ `0`, bấm sâu ra đúng bấy nhiêu dòng, kỳ tự chọn không lưu đệm) **đã có** ở §5 và N6 của bản đối chiếu toàn cảnh — không chép lại ở đây |
| `e55dc0a` (kịch bản demo sắp xếp lại sơ đồ tổ chức một xã) · `3c7370f` (deep link sống qua lần đăng nhập) · `34c523f` (gộp menu) · `d7c6ac6` · `2250061` · `442fa37` · `b26e40a` · `cf21835` · `5b5fba8` · `16eb9ed` · `e4aa5e4` · `329a0d4` | Kịch bản vận hành, điều hướng, và màn hình của kênh Zalo Bot — kênh chưa có trong phạm vi kho này (xem §M8). `329a0d4` (dòng log nói chuyển từ ai sang ai) đã gộp vào mục bàn giao ở §M2 |

---

### Vì sao `anh_huong` chỉ có HAI module

**Bản đầu khai bốn. Sau hai lượt kiểm bằng mã ngày 23/09, còn hai** — `service-petitions` và
`service-finance`. Hai module bị bỏ đều bị bỏ vì **cùng một lý do**: mở mã ra thì việc không có
thật. Chi tiết ở §Đã sửa.

Hai module còn lại đều có mục đang `dang_lam` mà khoảng commit này **đổi thẳng vào**:
`service-petitions` (đường xử lý phản ánh phía cán bộ) · `service-finance` (sáu tuyến ghi chứng
từ giải ngân).

**Bỏ khỏi `anh_huong` ngày 23/09 sau khi mở mã kiểm:** `service-documents` (mục 6 và 8 đều không
đòi đổi gì — xem §Đã sửa hàng 5) · `web-admin` (cả ba lý do từng viện ra đều đã dựng xong hoặc bị
chặn bởi một câu hỏi khác — hàng 6).

**Cố ý không đưa vào từ đầu:** `service-identity` (0036 hoá ra kho này đã ở dạng ấy; phần còn lại là một
câu hỏi, không phải một việc) · `service-comms` và `platform-admin` (thông báo nội bộ, thư điện
tử, Zalo Bot — **câu hỏi phạm vi**, và chặn `service-comms` sẽ chặn nhầm việc sự kiện
`petitions → comms` đang chạy) · `citizen-app` (chỉ chạm hồ sơ công dân, ngoài phạm vi) ·
`service-reporting` (chưa dựng, câu chặn ở phía khách) · `core` (không commit nào chạm).

Rào chặn nhầm một module là rào người ta gỡ. Bốn cái tên trên là bốn lời khai chịu được chất vấn.

**Hai module còn lại, và việc cụ thể của từng cái:**

| Module | Việc còn lại thật sự đòi đổi mã |
|---|---|
| `service-finance` | Cột nguồn vốn cho `chung_tu_giai_ngan` (mục 4) · luật "sửa bản đã xác nhận thì về nháp" (mục 5) · cưỡng chế `cach_tinh='children'` ở tầng nghiệp vụ (mục 7). **Không** phải dựng lại thu-chi |
| `service-petitions` | Bảng vết `nhat_ky_phan_anh` **chưa tồn tại** (mục 3, 8) — và nó là bảng append-only, nên hình dạng cột phải chốt trước khi có dữ liệu thật · mục 1 đang bị §Mâu thuẫn chặn |

## Đã sửa 23/09/2026 sau khi phiên chính kiểm lại bằng mã

**Hai lượt sửa, cùng ngày.** Lượt một sửa ba dòng báo thiếu thứ kho này đã có. Lượt hai thu hẹp
`anh_huong` từ bốn module xuống hai. Cả hai lượt đều sai theo **chiều tốn kém nhất** — báo thiếu
thứ đã có, nên phiên sau sẽ làm lại thứ đã xong. Giữ nguyên dấu vết để lần sau biết ghi chú này
từng nói gì.

| # | Chỗ | Bản đầu viết SAI | Sự thật, có `file:line` |
|---|---|---|---|
| 1 | §M1, dòng migration `0042` (và dòng `9401323`) | *"Bảng `sla` … **chưa có** ngưỡng cảnh báo sớm"* | `service-identity/migrations/0008_sla.sql:178` `gio_sap_den_han INTEGER NOT NULL` — theo `tenant_id`+`loai_viec`+`linh_vuc` (`:218`), trong `sla_gio_phai_duong` (`:244`), `loai_viec` đã nhận `'nhiem-vu'` (`:223`) |
| 2 | §M3, dòng `502d6f4` + `0142452` · Tóm tắt mục 7 | *"Thu-chi ngân sách ở kho này **chưa dựng**"* và *"đầu vào thiết kế phải có **trước** migration đầu tiên"* | `service-finance/migrations/0006_thu_chi_ngan_sach.sql` đã dựng và đã khớp cả hai luật: `cach_tinh` + CHECK (`:332,345`), `is_headline` (`:334`), kèm `cha_id` (`:328`), `cap` (`:333`), `don_vi_tinh` (`:167`), `lan` (`:160`) |
| 3 | §M2, dòng `e3d43d2` · Tóm tắt mục 6 | Nhãn `Mới vào sổ` / `Đã phân công` là thứ **kho này cần thêm** | `service-documents/migrations/0004_so_van_ban.sql:364,530` đã có ràng buộc sáu mã, mặc định `'moi-vao-so'` (`:342`). Việc thật chỉ còn ở **giao diện** |
| 4 | **BỎ SÓT** — Tóm tắt mục 8 | Ghi chung chung *"chưa có hình dạng cột"* cho cả hai sổ | Khuyết thật **chỉ ở `service-petitions`**: `nhat_ky_phan_anh` chưa được tạo (`0005_duong_xu_ly_phan_anh.sql:34-36`). Xem tiếp hàng 5 |

### Lượt thu hẹp thứ hai, cùng ngày — `anh_huong` từ BỐN xuống HAI

Lượt sửa đầu vẫn giữ `service-documents` và `web-admin`, với lý do *"nửa người của cặp bàn giao"*
và *"hai bộ nhãn + cách dùng chữ"*. Phiên chính đưa thêm bằng chứng từ **đặc tả và giao diện** kho
này; mở ra kiểm thì **cả hai lý do đều tan**.

| # | Module bị bỏ | Lý do từng viện ra | Vì sao nó không phải một việc |
|---|---|---|---|
| 5 | `service-documents` | *"Thiếu cặp từ-người → sang-người"* | **Đặc tả kho này không đòi cặp ấy.** `docs/ui-ux/05-van-ban-don-thu.md:149-153` vẽ năm dòng, lược đồ có đủ năm (`0004_so_van_ban.sql:509,513,517-518,520,524`), và chú thích `:515`/`:519` **trích thẳng** đặc tả. *Người giao trước* **suy được** từ `can_bo_xu_ly_ma` của dòng liền trước — thêm cột là nuôi bản sao thứ hai của dữ kiện đã có, đúng thứ **luật 9** (phép thử một dòng) và **luật 10 bất biến 3** cấm. Mục 6 cũng không còn: sáu mã đã ở trong `lich_su_chuyen_trang_thai_hop_le` (`:529-531`) |
| 6 | `web-admin` | *"Hai bộ nhãn cho hai sổ · gộp trạng thái+bàn giao · ô chọn người lọc theo bộ phận"* | **Cả ba đều đã dựng xong hoặc bị chặn bởi câu khác.** (a) Nhãn: `nhan-van-ban.ts:47-50` đã đúng, **đã dùng chữ *giải quyết*** — thứ bên kia vừa đổi sang thì kho này dùng sẵn. (b) Một thao tác: `nhan-van-ban.ts:204-208` + `thao-tac-van-ban.ts:34-41` đã gửi trọn `{denBoPhan, canBoXuLy, lyDo}` một lượt. (c) Ô chọn lọc theo bộ phận: `so-van-ban-den.tsx:978-987` ghi rõ vì sao còn là ô chữ — danh bạ đòi `admin.user`. Nửa "hai bộ nhãn" thì **chưa tới lượt**: không bảng `don_thu`, không tuyến `don_thu`, và `van-ban/page.tsx:21-23` cố ý chưa vẽ tab ấy |

**Vì sao sai, cả hai lượt.** Mọi chỗ sai đều có một hình dạng: kết luận *"kho này chưa có"* mà
**không mở tệp của kho này ra đọc** — lượt đầu là migration, lượt hai là đặc tả và mã giao diện.
Lượt đầu sửa được các dòng nhưng vẫn để `anh_huong` rộng; và `anh_huong` rộng **đắt hơn một dòng
văn sai**, vì nó chặn mọi lần ghi vào module.

**Cái giá của lần khai rộng ấy, nói thẳng:** rào đã chặn `service-documents` và `web-admin` vì
một việc không tồn tại. Người gặp rào sẽ hoặc làm lại thứ đã xong, hoặc gỡ rào — và rào bị gỡ
một lần thì lần sau nó chặn thật cũng không ai tin. Đó đúng là câu `kb/50-doi-chieu/README.md`
viết: *"khai rộng tay là chặn nhầm, và rào chặn nhầm là rào người ta gỡ."*

Quy tắc rút ra, đúng bằng câu đã ghi ở `kb/90-ephemeral/ban-giao-phien.md`: **một khẳng định về
kho NÀY phải mở tệp và dẫn `file:line`, không được suy từ sổ tiến độ hay từ trí nhớ** — và với
`anh_huong` thì phải kiểm **cả ba tầng**: lược đồ, đặc tả, và mã đang chạy. Có cột không có nghĩa
là thiếu việc; thiếu cột cũng không có nghĩa là có việc, nếu đặc tả không đòi cột ấy.
