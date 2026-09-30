---
id: 0065-vong-doi-nhiem-vu-theo-kho-yeu-cau
tier: T1
source: CURATED
owner: domain
derived_from_commit: b3cc9582
expires: null
owns_facts:
  - "vòng đời nhiệm vụ cho nhảy bước, chờ duyệt không còn bắt buộc, sang hoan-thanh không cần task.approve (người dùng chốt 30/09/2026)"
  - "mở lại nhiệm vụ đã hoàn thành cần task.approve và lý do bắt buộc; không mở lại việc con khi việc cha đã hoàn thành"
  - "mã nhiệm vụ không bao giờ sửa sau khi cấp — lệch có chủ ý khỏi vigov-require 7764c8a"
  - "hạn nhiệm vụ sửa thẳng được trong form sửa, không chỉ qua đề nghị lùi hạn"
  - "chủ trì và người thực hiện là MỘT vai trò: chuyên viên theo dõi ≡ người thực hiện, cơ quan chủ trì tham mưu ≡ bộ phận thực hiện"
  - "hạn nhiệm vụ mặc định 17:00, điền sẵn +7 ngày"
  - "luật người giữ việc của nhiệm vụ: người được giao làm mọi việc, người liên quan chỉ ghi nhật ký, người giữ task.update vẫn làm được"
  - "nhãn mặc định cột Kanban đầu tiên là Mới giao, xã đổi được"
---

# 0065. Vòng đời nhiệm vụ theo kho yêu cầu

**Trạng thái:** đã chốt · **Ngày:** 2026-09-30 · **Người dùng chốt** từng câu trong phiên chính
30/09/2026 · **Thay** ba lựa chọn người dùng đưa ra ngày 28/09/2026 (giữ `task.approve` khi hoàn
thành; sửa mã theo kho yêu cầu; **không** gộp chủ trì với người thực hiện) — ba lựa chọn ấy chỉ nằm
trong sổ tiến độ và chú thích mã, chưa từng có ADR · **Thay** `docs/ui-ux/02-nhiem-vu.md` §6 (chuỗi
chặt), §5.4 (ô mã trong khối sửa), §4.1 (tên cột đầu) · **Chưa dựng** phần đổi — xem §*Việc phải
làm*

## Bối cảnh

Đối chiếu với `../vigov-require` ngày 24/09 và 26/09 nêu tám xung đột ở module Nhiệm vụ (sổ tiến độ
`service-petitions` → `doi-chieu-24-09-nhiem-vu-phan-anh`, `doi-chieu-26-09-nhiem-vu-truoc-neo`;
ghi chú `kb/50-doi-chieu/2026-09-26-feat-m8-multitenant-foundation-nhiem-vu.md`). Các commit bên
kia: `52ec9b5` (vòng đời), `7764c8a` (sửa mã), `93cff7f` (sửa hạn), `a37ec96` (người giữ việc),
`e1d0204` (gộp vai trò), `b9a9718` (giờ hạn).

Nhiều phần **đã dựng** ở kho này sau lựa chọn 28/09 — bảng dưới ghi trạng thái mã tại `b3cc9582`
để người dựng biết câu nào là **đổi**, câu nào chỉ là **xác nhận**.

## Quyết định

| # | Đã chốt 30/09 | Mã tại `b3cc9582` | Đổi hay xác nhận |
|---|---|---|---|
| NV1 | **Theo kho yêu cầu**: cho nhảy bước (`moi-giao`→`dang-thuc-hien`, `dang-thuc-hien`→`hoan-thanh`); bước `cho-duyet` **không còn bắt buộc**. `task.approve` chỉ còn các việc khác của nó: **ký biên bản họp**, **mở lại** | Bảng chuyển đã cho nhảy bước (`service-petitions/internal/domain/nhiem_vu.go:97-105`), nhưng **mọi** bước vào `hoan-thanh` còn cần `task.approve` (`NeedsApproval`, `:175-177`) | **Đổi** — bỏ `task.approve` khỏi bước hoàn thành |
| NV2 | Mở lại việc đã xong **giữ**, cần `task.approve` **và** lý do bắt buộc. **Không** mở lại việc con khi việc cha đã `hoan-thanh` | `task.approve` + lý do đã dựng (`internal/app/nhiem_vu.go:1384-1397`, P12). Chặn mở việc con **chưa có** | **Đổi** một nửa — thêm lệnh chặn việc con |
| NV3 | Mã nhiệm vụ **không bao giờ sửa** sau khi cấp (luật 7 bất biến 3). **Lệch có chủ ý** khỏi `7764c8a` | Tuyến sửa mã **đã dựng** theo lựa chọn 28/09: PATCH `code`, `store.ChangeCode` (`internal/app/nhiem_vu.go:1097`), migration `0015_task_issued_code.sql` | **Đổi** — gỡ đường sửa mã |
| NV4 | **Theo kho yêu cầu**: hạn sửa thẳng trong form sửa, không chỉ qua đề nghị lùi hạn | **Đã dựng** theo lựa chọn 28/09: PATCH `due_at`, `store.CorrectDeadline`, migration `0016_task_deadline_correction.sql:89-103` | **Xác nhận** — xem câu còn mở #1 |
| NV5 | **Theo kho yêu cầu**: *chủ trì* và *người thực hiện* là **MỘT** vai trò. Theo `e1d0204`: chuyên viên theo dõi ≡ người thực hiện, cơ quan chủ trì tham mưu ≡ bộ phận thực hiện | Hai cặp **rời**: `chuyen_vien_theo_doi_ma` / `nguoi_thuc_hien_ma`, `co_quan_chu_tri_id` / `bo_phan_id` (`migrations/0006_nhiem_vu.sql:270`, `:287-288`). *Giao cho tôi* chỉ đọc `nguoi_thuc_hien_ma` (`internal/store/nhiem_vu.go:400`) | **Đổi** — đổi hình dạng dữ liệu |
| NV6 | Hạn nhiệm vụ mặc định **17:00**, điền sẵn **+7 ngày** — khớp đơn thư C9 (17:00) | Web đặt **23:59** (`web-admin/src/features/nhiem-vu/nhan-nhiem-vu.ts:1624-1635`); máy chủ không đặt giờ mặc định | **Đổi** ở web |
| NV7 | **Theo kho yêu cầu** (`a37ec96`): người được giao làm mọi việc trên nhiệm vụ, người liên quan **chỉ ghi nhật ký**, người giữ `task.update` (lãnh đạo) vẫn làm được. Tuyến ghi nhật ký tay đi theo từ đó | **Đã dựng**: `TaskWorkRightFor` (`internal/domain/task_participant.go:60`), cổng đổi trạng thái (`internal/app/nhiem_vu.go:1358-1362`), `POST /api/v1/tasks/{ma}/log-entries` (P3) | **Xác nhận** |
| NV8 | Nhãn mặc định cột Kanban đầu tiên: **"Mới giao"**; xã đổi được | Máy chủ đã giao `moi-giao` = "Mới giao" làm mặc định (`migrations/0010_nhan_trang_thai_nhiem_vu.sql:60`, `:180`; web `nhan-nhiem-vu.ts:113-114`) | **Xác nhận** — đặc tả §4.1 là chỗ lệch |

### Vì sao NV3 lệch khỏi kho yêu cầu mà NV4 thì theo

Mã nhiệm vụ là **số đã cấp** trên một hồ sơ hành chính: biên bản giấy, văn bản chỉ đạo, sổ theo dõi
in ra đều trích nó. Sửa mã làm mọi trích dẫn ấy trỏ vào hư không (URL theo mã cũ → 404), dù bảng
`task_issued_code` giữ được việc *không cấp lại*. Hạn thì khác: sửa hạn **có vết** trước/sau cả hai
hạn, và đề nghị lùi hạn vẫn là đường riêng khi cam kết thật sự dời (ADR 0038).

### Cái giá người dùng chấp nhận

| # | Cái giá |
|---|---|
| NV1 | Người thực hiện tự đánh dấu *hoàn thành* mà không ai duyệt. `ngay_hoan_thanh` — tử số của tỷ lệ đúng hạn (§11.3) — nay do chính người làm đặt. Hai ô tick tay *"Lãnh đạo xã đã phê duyệt hoàn thành"* (§5.4) không đổi trạng thái, nên không thay được bước duyệt |
| NV4 | **Tỷ lệ đúng hạn báo lên lãnh đạo đổi được bằng cách dời hạn.** Mã hôm nay giới hạn điều đó ở một chỗ: `han_ban_dau` chỉ đi theo lần sửa khi **chưa từng** có đề nghị lùi hạn `da-duyet` (kể cả đề nghị đã xoá mềm); sau một lần gia hạn, sửa chỉ dời `han_xu_ly` (`0016_task_deadline_correction.sql:22-33`). Mỗi lần sửa ghi vết trước/sau hai hạn |
| NV5 | Ai đang được ghi là *chuyên viên theo dõi* sẽ thành **người thực hiện**: nhận đủ quyền làm việc (NV7), hiện ở *Giao cho tôi*, và dừng đồng hồ *chưa cử người* của bộ phận |

## Còn mở — hỏi người dùng, không tự chọn

| # | Câu | Vì sao |
|---|---|---|
| 1 | **Hạn GỐC có còn giữ cho tỷ lệ đúng hạn không** khi hạn sửa thẳng được (NV4)? Ngày 28/09 người dùng chọn *"hạn ban đầu chỉ đi theo khi chưa gia hạn lần nào"* và mã đã dựng đúng thế (`0016`). Ngày 30/09 người dùng **không nhắc lại** câu ấy | Còn đứng thì không có gì phải dựng. Nếu muốn hạn gốc **không bao giờ** đi theo lần sửa, trigger `nhiem_vu_bat_bien` phải thay lần nữa — và một lỗi gõ ngày sẽ ở mãi trong mẫu số |
| 2 | Bước `cho-duyet` → `hoan-thanh` (khi xã vẫn dùng bước duyệt), và bước **trả lại để làm tiếp** `cho-duyet` → `dang-thuc-hien` (chốt 27/09: cần `task.approve` + lý do, `domain/nhiem_vu.go:147-159`) — **còn cần `task.approve` không**? | NV1 nói `task.approve` chỉ còn ký biên bản và mở lại. Đọc đúng chữ thì cả hai bước này mất cổng; nhưng câu 27/09 chưa bị ai rút lại |
| 3 | Dòng cũ mà **cả hai nửa** của một cặp đã có giá trị và **khác nhau** (chuyên viên A, người thực hiện B) | `e1d0204` chỉ điền chỗ trống, để nguyên những dòng này — tức hai người vẫn rời. Chọn một người là **giao lại** việc, không phải dọn dữ liệu |
| 4 | Người trở thành người thực hiện nhờ lần gộp (NV5) **có được báo** không | Một người nhận việc mà không biết mình đã nhận là việc không ai làm, trong khi hạn vẫn chạy |

### Trả lời của người dùng — 30/09/2026 (cùng ngày, sau khi ADR này ghi bốn câu trên)

| # | Trả lời |
|---|---|
| 1 | **Giữ quy tắc 28/09**: sửa hạn thẳng thì hạn gốc đi theo, **trừ khi** việc đã từng được duyệt lùi hạn — khi ấy hạn gốc khoá. Không phải dựng gì thêm (`0016`) |
| 2 | **Giữ `task.approve`** cho `cho-duyet → hoan-thanh` và cho *trả lại để làm tiếp* `cho-duyet → dang-thuc-hien` (kèm lý do, 27/09). Bước duyệt thành **tuỳ chọn**: người thực hiện tự đi thẳng tới `hoan-thanh` được, nhưng việc **đã** gửi lên chờ duyệt thì chỉ người cầm `task.approve` duyệt hoặc trả lại |
| 3, 4 | **Không phải hỏi**: người dùng xác nhận 30/09 chưa có dữ liệu thật trên môi trường nào, nên không có dòng cũ mang hai người khác nhau và không có ai "được gộp" thành người thực hiện. Migration gộp vẫn phải đảo ngược được (luật 7 bất biến 4) |

## Việc phải làm — liệt kê, chưa làm

Mỗi dòng là một thẻ việc, qua cổng riêng (ROUTING §0.3).

| # | Việc | Loại | Ghi chú |
|---|---|---|---|
| NV1 | `domain.NeedsApproval` bỏ vế `sang == HoanThanh`; mô tả tuyến `POST /api/v1/tasks/{ma}/status` trong hợp đồng mất lời 403 cho bước hoàn thành | Mã + hợp đồng REST | Chờ câu mở #2. Mục sổ `task-approve-hai-viec` hẹp lại: `task.approve` không còn vừa ký biên bản vừa duyệt nhiệm vụ. ADR 0037 quyết định 4 (việc cha xong khi mọi việc con xong) **giữ nguyên** |
| NV2 | Mở lại việc con khi việc cha `hoan-thanh` → từ chối, câu lỗi nói lý do | Mã + mã lỗi mới trong hợp đồng | Quyết trên dòng đã khoá, như mọi kiểm khác của `DoiTrangThai` |
| NV3 | Gỡ `code` khỏi thân PATCH; gỡ `store.ChangeCode` khỏi đường ghi; **migration mới** trả `nhiem_vu_bat_bien` về luật *`ma` không đổi* (`CREATE OR REPLACE`, không sửa `0015`/`0016` — checksum). **Giữ** bảng `task_issued_code`: nó vẫn canh việc không cấp lại | Migration + mã + hợp đồng (bớt trường) | Nhiệm vụ đã đổi mã trong lúc đường ấy sống thì **giữ mã mới**, không đổi về (luật 7 cấm #4). Nhập mã tay **lúc tạo** (§7.1 *Tự sinh mã*) không bị chạm. Web gỡ ô mã khỏi khối sửa |
| NV4 | Không dựng gì nếu câu mở #1 giữ lựa chọn 28/09; còn thiếu phía web theo sổ tiến độ | — | — |
| NV5 | **Migration** điền nửa trống của mỗi cặp từ nửa đang có; **theo xã, tiếp tục được** (luật 7 bất biến 5); **đảo được** — ghi lại dòng nào, nửa nào được điền (bản `e1d0204` không đảo được, luật 7 bất biến 4 không cho thế); vết theo chủ thể hệ thống (luật 6 bất biến 6). Đường ghi (tạo, giao việc) ghi một nửa là điền cả hai. **Không** xoá cột (luật 7 cấm #3) | Migration dữ liệu + mã + hợp đồng | Đổi chuyên viên là **giao việc**: phải đi qua tuyến giao việc (`task.assign`), không qua PATCH — PATCH cố ý không dời người thực hiện (`internal/http/nhiem_vu_ghi.go:24-25`). Chờ câu mở #3, #4 |
| NV6 | Web điền sẵn +7 ngày lịch lúc 17:00 (như `b9a9718`); nhiệm vụ đã có hạn 23:59 **giữ nguyên** | Web | Điền sẵn là giá trị form cán bộ sửa được, không phải hạn phần mềm tự ấn định |
| NV7 | Còn: người của bộ phận / cơ quan chủ trì theo sổ tiến độ | Mã | Sau NV5, chuyên viên theo dõi hết là "người liên quan chỉ ghi nhật ký" |
| NV8 | Không đổi mã | — | — |

→ ADR 0037 (cây việc con) · ADR 0038 (duyệt lùi hạn theo người trên bản ghi)
→ Đặc tả: `docs/ui-ux/02-nhiem-vu.md` §4.1, §5.4, §6
→ Việc dựng: sổ tiến độ `service-petitions` → `vong-doi-nhiem-vu-theo-adr-0065`
