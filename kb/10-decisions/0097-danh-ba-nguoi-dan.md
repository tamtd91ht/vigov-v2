---
id: 0097-danh-ba-nguoi-dan
tier: T1
source: CURATED
owner: architecture
derived_from_commit: bdf36a29
expires: null
owns_facts:
  - "danh bạ người dân (menu /nguoi-dan) thuộc service-identity: bảng citizen_contact, citizen_group, citizen_contact_group; tài nguyên citizen-contacts, citizen-groups; tệp Go citizen_directory*.go (chủ dự án, 16/10/2026)"
  - "số điện thoại trong danh bạ che ở mọi danh sách và bản xuất; chỉ nút 'Xem số đầy đủ' trong hộp thoại sửa trả số đầy đủ, quyền citizen.update, mỗi lần bấm một vết không mang giá trị — vì sao không thêm khoá gỡ che riêng theo khuôn ADR 0030"
  - "hình dạng xuyên xã THỨ NĂM: cột Zalo là một boolean theo (tenant_id, số đã chuẩn hoá), nối sang phiên công dân đang sống CỦA CHÍNH XÃ; không bao giờ trả id, không bao giờ cho biết số ấy có ở xã khác"
  - "nguồn dòng danh bạ: nhap-tay và nhap-excel; không đồng bộ tự động từ Mini App trong đợt này"
  - "giữ cả Nhóm (danh mục của xã) lẫn Nhãn (chữ tự do trên dòng) như prototype, kèm câu cấm nhãn về dữ liệu nhạy cảm"
  - "một số điện thoại một dòng sống mỗi xã; household_code để dành; is_active boolean; xoá mềm có lý do bắt buộc với người, tuỳ chọn với nhóm; phân trang keyset + tuyến đếm riêng; phân mảnh theo ADR 0096"
  - "tìm theo từ khoá khớp cả một phần số điện thoại cho mọi người giữ citizen.read — rủi ro luật 3 đã nêu và chủ dự án chấp nhận (16/10/2026)"
  - "cấp quyền mặc định citizen.read / citizen.update cho vai trò mẫu theo prototype (migration identity 0034); trưởng thôn đọc cả xã"
  - "phạm vi để ngoài đợt này: Gửi tin ZNS / SMS (MC2), xuất Excel danh bạ, đồng bộ Mini App, lọc theo thôn của trưởng thôn, cột đồng ý riêng từng dòng"
  - "căn cứ pháp lý thu thập là NĐ 13/2023 Điều 17 khoản 5 theo đọc của chuyên gia miền, chưa pháp chế đối chiếu; Luật Bảo vệ dữ liệu cá nhân 2025 chưa được kiểm; chính sách riêng tư Mini App phải nói về phép nối số ↔ phiên trước khi lên thật"
---

# 0097. Danh bạ người dân: service, che số, cột Zalo là hình dạng xuyên xã thứ năm, nhóm và nhãn

**Trạng thái:** đã chốt · **Ngày:** 2026-10-16 · **Chủ dự án chốt** mười ba điểm trong phiên
16/10/2026, qua phiếu hỏi của phiên chính · **Đóng** MC1 (`kb/90-ephemeral/doi-chieu-vigov-require.md:125`)
· **Nối tiếp** migration `service-identity/migrations/0028_citizen_dossier_notice_permissions.sql`
(hai khoá `citizen.read` · `citizen.update` gieo ngày 09/10/2026, chưa tuyến nào kiểm)

**Một ADR, không phải hai.** Hình dạng xuyên xã thứ năm (§3) chỉ tồn tại để điền một cột của màn này;
nó không có đời sống riêng, không có người gọi thứ hai, và điều kiện dừng của nó là điều kiện dừng
của danh bạ. Tách thành hai tệp là bắt người đọc mở hai tệp cho một quyết định.

## Bối cảnh

Kho yêu cầu dựng **Danh bạ người dân** cùng **Gửi tin ZNS/SMS qua eSMS** trong một lượt
(`kb/50-doi-chieu/2026-09-24-feat-m8-multitenant-foundation.md:81`). Ghi chú đối chiếu ngày 24/09 kết
luận đúng: kho này **chưa có kho số điện thoại người dân nào** ngoài phiên đăng nhập, nên đây là một
kho dữ liệu cá nhân mới (Nghị định 13/2023) và phải hỏi khách có đưa vào phạm vi không. Khoá quyền đã
gieo trước (migration 0028) để ma trận khớp prototype, nhưng ô tick ấy chưa cấp gì.

Ngày 16/10/2026 chủ dự án đưa danh bạ vào phạm vi và chốt hình dạng. Phần gửi tin (MC2) **không** đi
cùng — xem §Ngoài phạm vi.

Danh bạ này **không phải** sổ hộ tịch, **không phải** `dinh_danh_cong_dan` (ADR 0002 — một số điện
thoại là một định danh cho toàn nền tảng, không có `tenant_id`). Nó là danh sách liên lạc **của một
xã**: ai, số nào, thuộc nhóm nào. Chính prototype nói thế ở đầu `models.py`
(`../vigov-require/apps/api/app/modules/citizens/models.py:3-6`): bắt nhập đủ nhân thân là bắt xã làm
việc của ngành công an, rồi danh sách không bao giờ đầy.

## Quyết định

Mỗi dòng ghi phương án chọn, và **vì sao chọn nó thay cho prototype hoặc mặc định của kho**. Lý do
nào là của người ghi (không phải lời chủ dự án) thì ghi rõ.

| # | Điểm | Chốt | Vì sao, so với prototype / mặc định kho |
|---|---|---|---|
| D1 | Service sở hữu | **`service-identity`**. REST `citizen-contacts` · `citizen-groups`. Bảng `citizen_contact` · `citizen_group` · `citizen_contact_group`. Tệp Go `citizen_directory*.go` | Tên tệp **không** dùng tiền tố `citizen_contact_phone*`: `citizen_contact_phone.go` (fd620abf) là *số đã xác thực của một phiên công dân* — khái niệm khác, cùng chữ "contact". Lý do kỹ thuật ghi kèm (của người ghi): ba thứ cột Zalo cần nối — `phien_cong_dan`, `dinh_danh_cong_dan`, `thon_to_dan_pho` — đều ở identity, nên đặt ở service khác là mở một cuộc gọi gRPC cho mỗi dòng danh sách (luật 2 #2) |
| D2 | Số điện thoại | **Che** (`privacy.MaskPhone`) ở danh sách và mọi bản xuất. Số đầy đủ chỉ qua nút **"Xem số đầy đủ"** trong hộp thoại sửa, quyền **`citizen.update`**, **mỗi lần bấm một vết**, delta mang tên trường + khoá quyền, **không bao giờ mang giá trị** | Prototype hiện số trần ở bảng (`CitizenWorkspace.tsx:194`). Khuôn ADR 0030 sẽ đòi một khoá gỡ che riêng (`feedback.unmask`); chủ dự án **cố ý** dùng `citizen.update` thay vì khoá mới: *người sửa một lỗi gõ cần thấy số; người gửi thông báo thì không*. Tiền lệ cùng hình: ADR 0082 #3 (email cán bộ, khoá `task.read`), mã `service-identity/internal/app/staff_email_reveal.go` |
| D3 | Nguồn dòng | `nhap-tay` · `nhap-excel` (ADR 0011, tiếng Việt không dấu). **Không** đồng bộ tự động từ Mini App; giá trị thứ ba để sau. Cột **"Zalo"** = Có/— **suy ra** bằng phép nối số đã chuẩn hoá → `dinh_danh_cong_dan` → một `phien_cong_dan` **đang sống của chính xã** có `cong_dan_id` | Prototype có cột `zalo_uid` nhập tay/đồng bộ (`models.py:56`) và nguồn `manual` tiếng Anh. Kho này không lưu định danh Zalo trên dòng danh bạ: cột Zalo là **câu hỏi** đặt cho bảng phiên, không phải dữ liệu chép sang — nên không trôi. Hình dạng truy vấn: §3 |
| D4 | Nhóm và Nhãn | Giữ **cả hai** như prototype: Nhóm là danh mục của xã (bảng), Nhãn là chữ tự do trên dòng. Ô nhãn mang câu cố định: **"Không tạo nhãn về sức khoẻ, dân tộc, tôn giáo, chính trị"** | Chuyên gia miền đề nghị gộp làm một; chủ dự án chọn **khớp prototype**. Câu cố định vì nhãn tự do là chỗ dữ liệu nhạy cảm (NĐ 13 Điều 2 khoản 4) lọt vào mà không ai thấy — ví dụ của prototype `"đã tiêm mũi 3"` (`models.py:62`) đã là dữ liệu sức khoẻ |
| D5 | Khoá duy nhất | **Một số = một dòng sống mỗi xã**: `UNIQUE (tenant_id, phone, live_key)` với `live_key` sinh từ `deleted_at` (khuôn migration identity 0027:115-120; `make khoaduynhat` kiểm). `household_code` **nullable, để dành** cho ca "bà dùng số của cháu" — đợt này không dùng | Prototype dùng chỉ mục một phần `WHERE deleted_at IS NULL` (`models.py:42-48`) — trên bảng phân mảnh kho này không kiểm được tại chỗ, nên dùng `live_key` như 0027. Ca một số nhiều người có thật ở nông thôn; để dành cột thay vì bỏ qua, nhưng **không** dựng |
| D6 | Ngưng liên hệ | `is_active` boolean, mặc định `true` | Như prototype (`models.py:70`). Chủ dự án chọn thay cho một mốc thời gian từ chối nhận tin; mốc ấy thuộc phần gửi tin, chưa có |
| D7 | Xoá | Xoá mềm `deleted_at` · `deleted_by` · `delete_reason`. Lý do **bắt buộc** với người, **tuỳ chọn** với nhóm | Luật 7 bất biến 1. Prototype chỉ có `deleted_at` (`service.py:158`). Gỡ một người khỏi danh bạ là ngừng liên lạc với một công dân — phải trả lời được "vì sao"; gỡ một nhóm chỉ là dọn danh mục |
| D8 | Phân trang | **Keyset** + **tuyến đếm riêng** cùng bộ lọc (ADR 0071, khuôn `task-extension-counts`), 50 dòng/trang | Prototype `limit/offset` + `total` trong cùng phản hồi (`schemas.py:30-42`). Danh sách phân trang của kho này cố ý không trả tổng (`rest-api-design` §5 #4) |
| D9 | Phân mảnh | `citizen_contact` và `citizen_contact_group`: `PARTITION BY HASH (tenant_id) MODULUS 8`. `citizen_group`: bảng thường | ADR 0096 #1 và #3: hai bảng đầu tăng theo số công dân; nhóm là danh mục vài chục dòng mỗi xã |
| D10 | Đặc tả | Chương `docs/ui-ux/16-danh-ba-nguoi-dan.md`, ngoại lệ có tên trong `.claude/hooks/doc_guard.py` (`TAO_DUOC`) | Bộ `docs/ui-ux/` là bản chép prototype 16/09 — màn này ra đời sau, nên phải có chương mới; hook mở đúng một đường dẫn, không mở thư mục |
| D11 | Tìm kiếm | Từ khoá **trong thân yêu cầu**, khớp họ tên và địa chỉ (bỏ dấu) **và BẤT KỲ phần nào** của số điện thoại, cho mọi người giữ `citizen.read` | Như prototype (`schemas.py:36`, `CitizenWorkspace.tsx:95`). Từ khoá đi trong thân vì một mảnh số điện thoại trên URL là dữ liệu cá nhân trong log (luật 3 cấm #4). **Rủi ro đã nêu và được chấp nhận:** §4 |
| D12 | Cấp quyền mặc định | Theo prototype (`../vigov-require/apps/api/migrations/versions/20260924_0050-0050_citizens_notices.py:44-56`): **`citizen.read`** → `chu-tich-ubnd` · `pho-chu-tich-ubnd` · `chanh-van-phong` · `truong-bo-phan` · `chuyen-vien` · `tiep-nhan` · `truong-thon`; **`citizen.update`** → `chu-tich-ubnd` · `chanh-van-phong` · `tiep-nhan` · `truong-thon`; `quan-tri-he-thong` giữ cả hai từ 0028. Migration identity **0034** | Ngoại lệ **có chủ ý** với ADR 0030 điều kiện dừng #3 (không gán khoá trong migration hạt giống) — cùng loại ngoại lệ 0028 và 0032. **`truong-thon` đọc cả xã**, không lọc theo thôn của mình — chủ dự án chấp nhận cho đợt này |
| D13 | Phần còn lại | Địa chỉ và ghi chú **không** vào phản hồi danh sách, chỉ ở chi tiết/sửa. Tệp Excel tải lên **không lưu** — chỉ `import_batch_id` trên dòng + một vết cho cả lượt (tiền lệ ADR 0059 §1). Ba tuyến nhập dùng `citizen.update`. Xuất Excel danh bạ **ngoài đợt**. Nhãn cột giữ **"Zalo"**, tooltip *"Đã đăng nhập Mini App của xã bằng số này"*. **Không** có cột đồng ý riêng từng dòng | Địa chỉ ra khỏi danh sách để một màn không hiện 50 địa chỉ nhà một lúc (luật 3 bất biến 6 — chỉ trường cần). Không lưu tệp vì tệp là một bản sao thứ hai của dữ liệu cá nhân, không ai dọn. Cột đồng ý: căn cứ pháp lý ở §6 |

## 1. Che số và nút "Xem số đầy đủ" — vì sao không thêm khoá

ADR 0030 lập khuôn: *xem đầy đủ dữ liệu cá nhân là một khoá riêng* (`feedback.unmask`), vì quyền
xem nội dung không suy ra quyền xem người. Khuôn ấy **vẫn đúng** cho phiếu phản ánh. Ở danh bạ chủ dự
án chọn khác, và lý do là về **việc thật ở xã**: người bấm "Xem số đầy đủ" là người đang **sửa** dòng
ấy — sửa một số gõ nhầm thì phải thấy số. Người chỉ **gửi** thông báo (sau này, MC2) chọn nhóm và
nhãn, không cần đọc từng số. Nên quyền xem số trùng với quyền sửa, và một khoá thứ ba là một ô trên
màn Phân quyền mà quản trị xã phải hiểu — ADR 0030 §*Khách chốt đúng hai khoá* nói vì sao không bịa
khoá.

Cái giá, nói thẳng: mọi tài khoản giữ `citizen.update` (bốn vai trò mẫu + quản trị) đọc được số đầy
đủ của **bất kỳ** người nào trong danh bạ, từng số một, có vết. Không có cách xem hàng loạt — không
tuyến xuất, không trường số trần trong danh sách.

Ràng buộc kèm, không tách rời: **mỗi lần bấm một vết** (luật 6 bất biến 7), ghi trong **cùng giao
dịch** với lần đọc và số chỉ rời use case sau khi vết đã commit (thứ tự của
`staff_email_reveal.go:31-33`). Delta ghi **tên trường** (`phone`) và **khoá quyền** (`citizen.update`),
**không ghi số** — vết mà chứa số là vết thành kho số (luật 6 cấm #4). Động từ vết: tiếng Việt
`snake_case` theo khuôn `xem_email_can_bo`, người dựng đặt và ghi trong mã.

## 2. Mọi bản xuất che số — và đợt này không có bản xuất

Luật 3 bất biến 4: xuất là che mặc định, xuất đầy đủ là hành vi riêng, quyền riêng, có vết. Chủ dự
án để **xuất Excel danh bạ ngoài đợt này**. Hệ quả cho người dựng: không có tuyến xuất nào, kể cả
"xuất đã che" — một tuyến xuất đã che là tuyến ngày mai ai đó thêm cờ `?full=1`.

## 3. Hình dạng xuyên xã THỨ NĂM — cột "Zalo"

`service-identity/internal/store/crosstenant/doc.go:16-42` liệt kê **bốn** hình dạng đọc không mang
xã, và đòi một ADR trước khi thêm hình nào. Đây là ADR ấy. Danh sách trong `doc.go` phải thêm hình
dạng 5 trỏ về đây, cùng lượt với mã.

**Câu hỏi truy vấn trả lời:** *"số này, trong danh bạ của xã X, có đang đăng nhập Mini App của xã X
không?"*

| Bất biến | Nội dung |
|---|---|
| 1 | Khoá theo **`(tenant_id, số đã chuẩn hoá)`**. `tenant_id` là của yêu cầu (từ `Host`, luật 1 bất biến 3), **không bao giờ** từ client |
| 2 | Nối: `citizen_contact.phone` = `dinh_danh_cong_dan.so_dien_thoai` → `phien_cong_dan` **của chính `tenant_id` ấy** (`ScopedTx`, `$1` trên bảng phiên — đúng cách `citizen_contact_phone.go:19-23` làm), `cong_dan_id IS NOT NULL`, `thu_hoi_luc IS NULL`, `het_han_luc > now()` |
| 3 | Kết quả là **một boolean mỗi dòng danh bạ**. Không `cong_dan_id`, không `id` phiên, không `tai_khoan_zalo`, không thời điểm đăng nhập, không gì khác |
| 4 | **Không bao giờ** cho biết số ấy có trong `dinh_danh_cong_dan` hay không, và **không bao giờ** cho biết số ấy có phiên ở **xã khác** hay không. Số có phiên sống ở xã B, nằm trong danh bạ xã A → xã A thấy **"—"** |
| 5 | Lọc "Chỉ người có Zalo" là **cùng** phép nối ấy dưới dạng `WHERE`, không phải một cột lưu sẵn — cột lưu sẵn là `last_synced_at` của prototype, thứ trôi ngay khi phiên hết hạn (cùng lý lẽ luật 10 bất biến 3) |

**Vì sao đây là hình dạng mới chứ không phải hình 1 hay 3.** Hình 1 đọc `dinh_danh_cong_dan` **khoá
theo một tài khoản**; hình 3 khoá theo **một công dân từ phiên** và trả dòng của chính người ấy.
Truy vấn này khoá theo **một số trong danh bạ của xã**, hỏi về **người khác** chứ không phải người
gọi, và được gọi theo lô cho cả trang danh sách. Nó cũng **không phải** tổng hợp bị cấm ở `doc.go:61`:
không trả dòng của xã nào khác, không `COUNT` qua xã — mỗi câu trả lời thuộc về đúng một dòng danh bạ
của đúng xã đang hỏi. Thứ giữ nó ở bên đúng của ranh giới là bất biến 3 và 4; bỏ một trong hai là
biến nó thành cỗ máy dò "số này đã từng dùng Mini App ở đâu".

**Nghĩa vụ kiểm, bắt buộc có trước khi gộp:** một bài kiểm dựng số `0900000000` trong danh bạ xã A và
một phiên công dân **đang sống** mang số ấy ở xã B; danh sách của xã A trả **"—"**. Bài kiểm thứ hai:
phiên **đã thu hồi** hoặc **hết hạn** ở chính xã A → **"—"**.

**Nợ với khách, không phải điều kiện chặn:** phép nối này là một cách dùng mới dữ liệu phiên của người
dân (số đăng nhập ↔ danh bạ của xã). Chính sách riêng tư dùng chung mọi app (ADR 0044 §*Đã quyết
02/10/2026*, sổ `citizen-app/chinh-sach-dung-voi-ban-dung`) **phải nói về phép nối ấy trước khi lên
thật**. Ghi ở đây là mục mở; lời văn do chủ dự án duyệt.

## 4. Tìm kiếm khớp một phần số — rủi ro đã nêu, chủ dự án chấp nhận

Số trong danh sách bị che, nhưng ô tìm khớp **bất kỳ phần nào** của số cho mọi người giữ
`citizen.read` (bảy vai trò mẫu). Phiên chính nêu với chủ dự án, 16/10/2026: *một người giữ
`citizen.read` có thể dò các chữ số bị che bằng cách gõ từng mảnh số vào ô tìm và xem dòng nào còn
khớp — tức lớp che (luật 3 bất biến 3) bị lách qua ô tìm, không cần `citizen.update`, không để vết.*

Chủ dự án **chấp nhận rủi ro ấy** và giữ hành vi của prototype. Lời nguyên văn của chủ dự án không
được chuyển tới người ghi; đoạn trên ghi theo phiếu giao việc của phiên chính.

Người dựng **không** tự thu hẹp (ví dụ chỉ khớp 4 số cuối, hay đòi `citizen.update` cho tìm theo số):
đó là đổi một quyết định đã chốt. Muốn thu hẹp thì hỏi lại chủ dự án với con số: bao nhiêu vai trò,
bao nhiêu người mỗi xã.

## 5. Nhóm, nhãn và câu cấm

| Thứ | Là gì | Ai tạo | Ở đâu |
|---|---|---|---|
| Nhóm | Danh mục của xã: "Tổ trưởng dân phố", "Hộ kinh doanh" | `citizen.update`, tab Nhóm | `citizen_group` + `citizen_contact_group` |
| Nhãn | Chữ tự do, nhiều nhãn một dòng, cách nhau dấu phẩy | Người sửa dòng | cột trên `citizen_contact` |

Câu **"Không tạo nhãn về sức khoẻ, dân tộc, tôn giáo, chính trị"** đứng cố định dưới ô nhãn, không
tắt được. Nó không chặn được bằng máy — máy không biết "hộ nghèo" có phải dữ liệu nhạy cảm không — nên
nó là câu nhắc **đúng chỗ người gõ**, và là bằng chứng xã đã được báo nếu có thanh tra. Chuyên gia
miền đề nghị gộp Nhóm và Nhãn thành một để chỉ còn danh mục có kiểm soát; chủ dự án chọn khớp
prototype. Cái giá: nhãn tự do là chỗ một xã có thể ghi thứ không nên ghi, và kho này chỉ nhắc.

## 6. Căn cứ pháp lý — ghi đúng mức đã biết

Danh bạ là **thu thập dữ liệu cá nhân** (họ tên, số điện thoại, địa chỉ) do cán bộ nhập, không qua
người dân. Chuyên gia miền đọc **NĐ 13/2023 Điều 17 khoản 5** (xử lý phục vụ hoạt động của cơ quan
nhà nước theo quy định pháp luật) là căn cứ không cần đồng ý từng người — vì thế **không có cột đồng
ý**. **Theo đọc của chuyên gia miền, chưa pháp chế đối chiếu.** **Luật Bảo vệ dữ liệu cá nhân 2025,
hiệu lực 01/01/2026, chưa được kiểm** — ai dựng hay nghiệm thu thấy điều khoản nào ở luật ấy đòi đồng
ý hoặc thông báo cho người dân thì đó là **điều kiện dừng**, không phải việc làm thêm.

## Ngoài phạm vi đợt này — chủ dự án để lại, không phải bỏ sót

| Thứ | Ở đâu nếu quay lại |
|---|---|
| Gửi tin ZNS / SMS tới danh bạ | MC2 (`doi-chieu-vigov-require.md:126`); kho này gửi ZNS từ OA từng xã qua `service-comms` (ADR 0018), không qua eSMS |
| Xuất Excel danh bạ | §2 — khi làm thì che mặc định, đầy đủ là quyền riêng, có vết |
| Đồng bộ tự động từ Mini App (nguồn thứ ba) | D3 |
| Trưởng thôn chỉ thấy thôn mình | D12 |
| Cột đồng ý riêng từng dòng | §6 |
| `household_code` | D5 |

## Hệ quả

- **Dễ hơn:** hai khoá của 0028 có tuyến thật; menu "Danh bạ người dân" rời `PENDING_SCREENS`
  (`web-admin/src/components/muc-menu.ts:164`); cột Zalo không trôi vì không lưu.
- **Khó hơn:** mỗi trang danh sách là một phép nối thêm sang bảng phiên; `doc.go` có năm hình dạng
  và bài kiểm xã A / xã B phải sống mãi.
- **Mất / rủi ro còn lại:** §1 (mọi người sửa đều xem được từng số), §4 (dò số qua ô tìm), §5 (nhãn
  tự do chỉ được nhắc), §6 (căn cứ chưa pháp chế đối chiếu; luật 2025 chưa kiểm).
- **Phải trả ngay (việc khác làm, không thuộc tệp này):**

| Việc | Ở đâu |
|---|---|
| Migration bảng + `live_key` + phân mảnh 8 | `service-identity/migrations/` |
| Migration 0034 cấp quyền theo D12 | `service-identity/migrations/0034_*.sql` |
| Hình dạng 5 vào `service-identity/internal/store/crosstenant/doc.go` + dấu `@cross-tenant` trên câu lệnh | `service-identity/internal/store/` |
| Bài kiểm xã A / xã B, thu hồi, hết hạn | cùng gói với truy vấn |
| Hai dòng ngôn ngữ chung (`CitizenContact`, `CitizenGroup`) | `kb/00-foundation/ubiquitous-language.md` — đã ghi cùng lượt |
| Chương 16 | `docs/ui-ux/16-danh-ba-nguoi-dan.md` — đã ghi cùng lượt |
| Đoạn chính sách riêng tư về phép nối số ↔ phiên | sổ `citizen-app/chinh-sach-dung-voi-ban-dung`, chủ dự án duyệt |

## ĐIỀU KIỆN DỪNG

1. Truy vấn cột Zalo trả bất cứ gì ngoài một boolean, hoặc được gọi với `tenant_id` không từ `Host`
2. Một tuyến trả số đầy đủ ngoài nút "Xem số đầy đủ" — kể cả xuất, kể cả gRPC cho service khác (ngoại lệ duy
   nhất, chủ dự án 17/10/2026: `GetCitizenContactMatchPhone` cho service-petitions — ADR 0102 #2)
3. Thêm khoá quyền mới cho danh bạ (câu mở #27) hay gán khoá cho vai trò ngoài D12
4. Giá trị nguồn thứ ba, đồng bộ từ Mini App, hoặc ghi `zalo_uid` / `cong_dan_id` lên dòng danh bạ
5. Thu hẹp hay nới ô tìm so với D11 mà không hỏi lại
6. Một điều khoản của Luật Bảo vệ dữ liệu cá nhân 2025 đòi đồng ý hoặc thông báo cho người trong danh bạ

→ ADR 0002 (định danh công dân toàn nền tảng, không `tenant_id`): `kb/10-decisions/0002-citizen-identity-platform-level.md`
→ ADR 0030 (khuôn khoá gỡ che riêng, và vì sao không bịa khoá): `kb/10-decisions/0030-hai-khoa-quyen-phan-anh.md`
→ ADR 0082 #3 (xem email cán bộ bằng khoá đã có, mỗi lần một vết): `kb/10-decisions/0082-nhiem-vu-cap-nhat-theo-prototype-08-10.md`
→ ADR 0059 §1 (nhập Excel: không lưu tệp, N dòng một vết): `kb/10-decisions/0059-excel-import-staff-and-catalogues.md`
→ ADR 0071 (tuyến đếm riêng): `kb/10-decisions/0071-so-tay-lanh-dao-ba-cot.md`
→ ADR 0096 (bảng nào phân mảnh): `kb/10-decisions/0096-partition-only-growing-tables-modulus-8.md`
→ ADR 0044 §*Đã quyết 02/10/2026* (một chính sách riêng tư cho mọi app): `kb/10-decisions/0044-hai-che-do-mini-app.md`
→ ADR 0050 §*Sửa đổi 08/10/2026* (tiền lệ nối phiên → số đã xác thực): `kb/10-decisions/0050-kenh-cong-dan-theo-kho-yeu-cau.md`
→ Bốn hình dạng xuyên xã đã có: `service-identity/internal/store/crosstenant/doc.go`
→ Luật 1, 3, 5, 6, 7: `.claude/rules/critical/`
