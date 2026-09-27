---
id: doi-chieu-2026-09-26-feat-m8-multitenant-foundation-nhiem-vu
tier: T2
source: CURATED
owner: architecture
derived_from_commit: 9173b7b
expires: null
kho_nguon: vigov-require
branch: feat/m8-multitenant-foundation
sha_tu: 0053854
sha_den: 0053854
ngay_review: 2026-09-26
anh_huong: [service-petitions, web-admin]
owns_facts:
  - "bảy commit M1 nhiệm vụ TRƯỚC NEO (56cfc3b · 4e27144 · 52ec9b5 · a037b76 · f66bc93 · 7764c8a · 93cff7f) lệch gì so với vòng đời, mã, xoá và hạn nhiệm vụ của service-petitions và web-admin"
---

# Đối chiếu `feat/m8-multitenant-foundation` · Nhiệm vụ · 26/09/2026

`0053854` → `0053854` · 0 commit mới · đọc ngày 26/09/2026 · cộng một **danh sách** 7 commit trước neo

| Phần | Đọc gì |
|---|---|
| A | Kiểm lại neo: `fetch --all --prune`, cây bên kia sạch, `origin/feat/m8-multitenant-foundation` = `0053854`. `git log 0053854..origin/…` **rỗng**. Neo **không dời** |
| B | M1 nhiệm vụ **trước neo**, phạm vi vòng đời · mã · xoá · sửa hạn: `56cfc3b` · `4e27144` · `52ec9b5` · `a037b76` · `f66bc93` (27/08) · `7764c8a` (28/08) · `93cff7f` (06/09). Cả bảy là tổ tiên của `93cff7f` = `origin/main` bên kia, kiểm bằng `git merge-base --is-ancestor` |

**Vì sao có lượt này.** Ghi chú 23/09 mở khoảng `93cff7f..b159f0e` — nửa mở, nên `93cff7f` và mọi commit trước nó
**chưa ai đối chiếu** cho Nhiệm vụ. Ghi chú 25/09 biên bản họp chỉ đọc phần Meeting/Conclusion của `95195b1` · `ec0faed` · `63cad73`.

`sha_tu` = `sha_den` = `0053854` vì khoảng mới rỗng. Phần B là **danh sách**, không phải khoảng: nới `sha_tu` về trước
`56cfc3b` là khai đã đọc mọi commit giữa đó, mà không (xem §Chưa đọc).

**Tệp riêng**, cùng lý do ghi chú biên bản họp nêu: `tien-do/service-petitions.json` và `tien-do/web-admin.json` đã trích
tên các ghi chú 23–25/09; nối vào đó thì rào coi phần này là "đã tiếp nhận" ngay, không ai mở việc.

**Cách đọc vòng đời.** Không đọc diff từng commit rồi cộng lại: đọc **bảng ở neo** `0053854:apps/api/app/modules/tasks/service.py:63-102`
(`ALLOWED_TRANSITIONS`), rồi `git blame` để biết dòng nào của commit nào. Bảng **không đổi** trong `93cff7f..0053854`
(diff khoảng ấy chỉ chạm dòng ngữ cảnh). Kết quả blame: các cạnh mới đến từ **`52ec9b5`**, không phải `4e27144` —
`4e27144` chỉ đổi nhãn, cấu hình và `_apply_status`; `52ec9b5` ("put back what the stash swept up") mới đưa bảng vào.

---

## Tóm tắt — kho này phải làm gì

| # | Nhãn | Bên kia | Kho này | Ai trả lời · việc sổ | Module |
|---|---|---|---|---|---|
| 1 | **CONFLICT** | Vòng đời lỏng: bỏ qua `accepted`, bỏ qua `pending_approval`, mở lại `done`, `transferred` đi tiếp — `52ec9b5` | Chuỗi chặt, `hoan-thanh` và `chuyen-tiep` là ngõ cụt (`domain/nhiem_vu.go:76-84`; bản sao web `nhan-nhiem-vu.ts:223-231`; đặc tả `02-nhiem-vu.md:227-231`) | Chủ dự án hỏi khách. Sổ: `service-petitions/nhiem-vu-tuyen-ghi` | service-petitions · web-admin |
| 2 | **CONFLICT** | Mã nhiệm vụ **sửa được** qua `PATCH` — `7764c8a` | Trigger `nhiem_vu_bat_bien` từ chối đổi `ma` (`0006_nhiem_vu.sql:155-160`), dẫn luật 7 bất biến 3 | Chủ dự án. Sổ: `service-petitions/nhiem-vu-tuyen-ghi` | service-petitions · web-admin |
| 3 | **CONFLICT** | Sửa hạn **không phải** gia hạn: `PATCH` nhận `due_at`; chưa gia hạn lần nào thì hạn ban đầu đi theo — `93cff7f` | `PATCH` không nhận `due_at` (`nhiem_vu_ghi.go:200-203`); trigger cấm đổi `han_ban_dau` (`0006:162-168`); web ghi lý do không dựng (`nhan-nhiem-vu.ts:1040-1044`) | Khách. Sổ: `service-petitions/nhiem-vu-tuyen-ghi` | service-petitions · web-admin |
| 4 | **NEW** | Máy chủ trả `allowed_transitions` trên mỗi nhiệm vụ, giao diện chỉ vẽ lại — `56cfc3b` | Hợp đồng không phát vòng đời; web giữ **bản thứ hai** của bảng (`nhan-nhiem-vu.ts:208-231`) và không biết trạng thái trước lúc tạm dừng (`:236-239`) | contract-designer, gộp vào món thiếu của `service-petitions/hop-dong-nhiem-vu-thieu-ba-mon` | service-petitions · web-admin |
| 5 | **NEW** | Xoá hàng loạt = chọn nhiều dòng, gọi **từng** `DELETE` một — `a037b76` | Web chưa dựng "Xoá đã chọn", lý do ghi là thiếu tuyến `xoa-nhieu` (`nhan-nhiem-vu.ts:1447-1451`) — bên kia cho thấy không cần tuyến mới. Còn vướng: `DELETE` ở đây **bắt buộc lý do** | Builder web hỏi: một lý do cho cả lô hay từng dòng. Sổ: `web-admin/man-nhiem-vu` | web-admin |
| 6 | **ALREADY DONE** | `DELETE /tasks/{id}` xoá mềm, khoá riêng `task.delete`, từ chối khi còn việc con — `a037b76` | `routes.go:1128-1157`; khoá đã gieo `service-identity/migrations/0001_init.sql:309` | — | — |
| 7 | **ALREADY DONE** | Mã `NV01…` tự sinh hoặc gõ tay, không cấp lại kể cả mã của dòng đã xoá; tiêu đề sửa được — `7764c8a` | `routes.go:1038-1053`; `Title` trong `suaNhiemVuVao` (`nhiem_vu_ghi.go:206`) | — | — |
| 8 | **ALREADY DONE** · **UNKNOWN** | `Trễ hạn` và `Hoàn thành trễ hạn` suy ra, không lưu; nhưng thêm **hai mục** `tre-han` · `hoan-thanh-tre-han` vào danh mục `task_status`, có nhãn và màu riêng — `4e27144` · `52ec9b5` | Suy ra: `domain/nhiem_vu.go:288` `TreHan`; chip ghép từ nhãn `hoan-thanh` của xã (`3ea137e`). Danh mục đóng **bảy mã** (#21, ADR 0035 §C) | Người giữ #21: nhãn của hai trạng thái suy ra có thuộc danh mục xã sửa được không | — |

**#1–#3 KHÔNG phải điều kiện dừng #1 theo nghĩa hẹp**: không ADR nào, không câu `DECIDED` nào của kho này chốt các cạnh
của vòng đời, việc sửa mã hay việc sửa hạn. Chúng mâu thuẫn với **đặc tả và luật của chính kho này**. Vẫn phải có người
quyết trước khi builder chạm, vì cả ba đều chạm hồ sơ lưu trữ (luật 7) hoặc một con số đi lên lãnh đạo (tỷ lệ đúng hạn §11.3).

---

## Mâu thuẫn — hai phía, không chọn bên

### #1 Vòng đời nhiệm vụ

**Bên kia** — `52ec9b5`, `0053854:apps/api/app/modules/tasks/service.py:63-102`:

> `TaskStatus.NEW: {ACCEPTED, IN_PROGRESS, TRANSFERRED, PAUSED}` · `IN_PROGRESS: {DONE, PENDING_APPROVAL, PAUSED, TRANSFERRED}` ·
> `PENDING_APPROVAL: {DONE, IN_PROGRESS}` · `PAUSED: {IN_PROGRESS, ACCEPTED, NEW}` · `TRANSFERRED: {ACCEPTED, IN_PROGRESS}` ·
> `DONE: {IN_PROGRESS}`
>
> *"A commune runs three steps: not started, doing it, done. The SRS draws five, with acknowledgement and approval in
> between, and communes that want those still have them — but they are not on the way any more."* (`:64-69`)
>
> *"Reopening is deliberate: work signed off and then found wanting is a thing that happens, and the alternative is a
> second task that hides the first one's history."* (`:98-100`)

Thông điệp `52ec9b5` nói *"the rules the commune has since overruled"*; thông điệp `4e27144` nói *"Communes track five
things"*. **Không dẫn biên bản, không dẫn người** — lời kể của BA, chưa kiểm được. Cùng chiều: `docs/spec/05-nghiep-vu.md:10-12`
(thêm ở `d332a62`, 21/09 — nằm trong khoảng ghi chú 23/09 nhưng ghi chú ấy không nhắc) nói `pending_approval` *"hiện đang
ẩn khỏi giao diện theo yêu cầu của xã"*. Ngược chiều, **ngay trong kho bên kia**: `docs/SRS.md:139` vẫn vẽ chuỗi chặt.

**Kho này** — `service-petitions/internal/domain/nhiem_vu.go:76-84`:

> `MoiGiao: {DaTiepNhanNV, TamDung, ChuyenTiep}` · `DangThucHien: {ChoDuyet, TamDung, ChuyenTiep}` ·
> `ChoDuyet: {HoanThanh, TamDung, ChuyenTiep}` · `HoanThanh: {}` · `ChuyenTiep: {}`
>
> *"`hoan-thanh` AND `chuyen-tiep` ARE TERMINAL … §6 draws no arrow leaving it"* (`:63-68`) · *"chuyen-tiep … the work
> continues as ANOTHER task in another department, and this row stops here"* (`:69-73`)

và `docs/ui-ux/02-nhiem-vu.md:227-231` (sơ đồ chuỗi chặt), `routes.go:1097-1104` (sang `hoan-thanh` cần `task.approve`).

**Năm cạnh lệch, mỗi cạnh một hệ quả riêng — hỏi từng cạnh, không hỏi gộp:**

| Cạnh | Bên kia | Kho này | Chạm gì nếu theo |
|---|---|---|---|
| `moi-giao → dang-thuc-hien` | có | không | Bên kia **đã thử rồi bỏ** việc đóng dấu `accepted_at` khi nhảy thẳng (`4e27144` thêm, `52ec9b5` gỡ; ở neo `_apply_status:681` chỉ đóng dấu khi sang `ACCEPTED`) — tức nhảy thẳng thì mốc tiếp nhận để trống vĩnh viễn |
| `dang-thuc-hien → hoan-thanh` | có | không | Bỏ bước `cho-duyet`, tức bỏ chỗ `task.approve` đang gắn (`routes.go:1097-1104`). Bên kia còn bắt minh chứng khi sang `Chờ duyệt` (`docs/SRS.md:149`) — bỏ bước là bỏ luôn cửa ấy. Đụng câu A của ghi chú 24/09 (ai được hoàn thành) |
| `hoan-thanh → dang-thuc-hien` (mở lại) | có, xoá `completed_at` (`_apply_status:686-687`) | không | `CHECK ((trang_thai = 'hoan-thanh') = (ngay_hoan_thanh IS NOT NULL))` (`0006:370`) buộc xoá mốc hoàn thành — mất mốc cũ khỏi dòng, tỷ lệ đúng hạn §11.3 đổi hồi tố. Nhật ký còn giữ vết |
| `tam-dung → moi-giao / da-tiep-nhan / dang-thuc-hien` | bất kể trước đó là gì; **không** về `pending_approval` | chỉ về đúng trạng thái trước lúc dừng (`TamDungVeDuoc`, `nhiem_vu.go:136-138`, đọc từ nhật ký) | Kho này chặt hơn; bên kia cho `tam-dung → moi-giao` từ bất cứ đâu |
| `chuyen-tiep → da-tiep-nhan / dang-thuc-hien` | cùng dòng đi tiếp | ngõ cụt, việc tiếp là **dòng khác** | Bên kia **trả lời ngầm** điều kiện dừng đang treo ở `nhiem-vu-tuyen-ghi.tiep_theo` (cột liên kết hai nhiệm vụ): không có dòng thứ hai. Không phải câu `OPEN` trong `open-questions.json`, nhưng cùng loại điều kiện dừng #2 — trùng nhau thì phải có người xác nhận |

**Ai quyết:** chủ dự án, hỏi khách — đặc tả §6 của kho này là nguồn hiện hành. **Việc sổ:** `service-petitions/nhiem-vu-tuyen-ghi`
(máy trạng thái, `duocHoanThanh`, CHECK `0006:370`) và `web-admin/man-nhiem-vu` (bản sao `CHUYEN_DUOC`).
Đổi mã trạng thái thì không cần: bảy mã hai bên khớp nhau một-một, #21 không bị chạm.

### #2 Sửa mã nhiệm vụ

**Bên kia** — `7764c8a`, `schemas.py` `TaskUpdate.code` (ở neo `schemas.py:102`); `service.py:393-399` ở neo:

> `if new_code is not None and new_code != task.code and await repository.code_ever_used(session, new_code): raise …task_code_taken`

**Kho này** — `service-petitions/migrations/0006_nhiem_vu.sql:135-138, 155-160`:

> *"`ma` an issued number is never reissued or renumbered (rule 7, invariant 3). `NV19` is written on paper minutes,
> quoted in meeting conclusions and printed in the Sổ theo dõi"* · `RAISE EXCEPTION 'administrative record %: `ma` is immutable'`

Ngược lại, **đặc tả kho này** đặt `Mã nhiệm vụ` trong khối có nút `✎ Sửa` (`02-nhiem-vu.md:162-166`).

**Đo từ mã bên kia, không suy:** `code_ever_used` (`7764c8a` `repository.py`) tìm `Task.code == code` trên các dòng hiện có.
Sửa `NV12` thành mã khác thì `NV12` không còn nằm ở dòng nào, nên lần tự sinh sau (`codes.py` `next_task_code`) **cấp lại
được `NV12`** — đúng thứ chính docstring ấy nói phải tránh. Đây là lỗi tiềm ẩn bên kia, ghi để người quyết biết giá của lối "cho sửa".

**Ai quyết:** chủ dự án — đây là luật 7 của kho này chạm một ô đặc tả, không phải câu của khách. **Việc sổ:** `service-petitions/nhiem-vu-tuyen-ghi`.

### #3 Sửa hạn xử lý

**Bên kia** — `93cff7f`, `0053854:…/tasks/service.py:415-423`:

> *"Sửa hạn xử lý KHÔNG phải gia hạn … Còn đây là sửa cho đúng — gõ nhầm ngày, hoặc văn bản giao việc ghi hạn khác. Nếu
> chưa có lần gia hạn nào thì hạn ban đầu phải đi theo"* · `if "due_at" in changes and task.extension_count == 0: task.original_due_at = task.due_at`

Chính xác hơn lời scout: `due_at` sửa được **ở mọi lúc** (vòng `setattr` `:412-413`), khoá `task.update`; chỉ việc hạn
ban đầu đi theo là có điều kiện `extension_count == 0`. Giao diện gắn giờ cứng `T17:00:00` giờ trình duyệt
(`93cff7f` `TaskRegisterSection.tsx`).

**Kho này** — `nhiem_vu_ghi.go:200-203`:

> *"`due_at` is absent because a deadline moves through the extension flow and nowhere else"*

`0006_nhiem_vu.sql:162-168` (`han_ban_dau` bất biến) · `web-admin/src/features/nhiem-vu/nhan-nhiem-vu.ts:1040-1044`
(*"Một ô ngày sửa trực tiếp ở form sẽ là đường vòng qua đúng vòng duyệt ấy"*). Ngược lại, **đặc tả kho này** đặt `Hạn xử lý`
trong khối `✎ Sửa` (`02-nhiem-vu.md:168`).

Theo bên kia ở kho này thì phải **gỡ một luật của trigger** (cho đổi `han_ban_dau` khi chưa gia hạn) — tức người quyết
phải chấp nhận rằng mẫu số của tỷ lệ đúng hạn §11.3 (`02-nhiem-vu.md:354`) đổi được bằng một khoá `task.update`, không cần
người duyệt. Vế "gắn 17:00" trùng câu D của ghi chú 24/09, không hỏi lại.

**Ai quyết:** khách (vì chạm con số §11.3). **Việc sổ:** `service-petitions/nhiem-vu-tuyen-ghi`, rồi `web-admin/man-nhiem-vu`.

---

## Chi tiết theo phân hệ

### M1 — Nhiệm vụ (`nhiem-vu`)

| Commit | Tệp bên kia | Đổi gì | Hệ quả ở kho này |
|---|---|---|---|
| `56cfc3b` | `schemas.py` `TaskRead.allowed_transitions` (neo `:309`) · `service.py` `_to_read` (neo `:193`) · `TaskProgressForm.tsx` | Máy chủ phát danh sách bước đi được, tính từ **cùng** bảng nó thi hành; ô chọn và Kanban chỉ vẽ lại | Tóm tắt #4. Kho này tính được chính xác hơn bên kia, vì biết trạng thái trước tạm dừng từ nhật ký. Thêm trường vào `nhiemVuRa` là đổi hợp đồng đã công bố (thêm tuỳ chọn) |
| `4e27144` | `task-display.ts` · `default_config.json` `task_status` | Nhãn `moi-giao` "Mới giao" → "Chưa thực hiện"; thêm hai mục suy ra vào danh mục; `_apply_status` đóng dấu `accepted_at` khi sang `IN_PROGRESS` | Nhãn: kho này để xã tự đổi (#21), đặc tả đã ghi Kanban gọi "Chưa thực hiện" (`02-nhiem-vu.md:218`), web cố ý bỏ nhãn Kanban riêng (`nhan-nhiem-vu.ts:95-98`). Không việc. Hai mục suy ra → Tóm tắt #8. `accepted_at` → bị `52ec9b5` gỡ, xem §Mâu thuẫn #1 |
| `52ec9b5` | `service.py` `ALLOWED_TRANSITIONS` · `completed_late` | Bảng vòng đời lỏng; `completed_late` suy ra, so với `due_at` **hiện hành** | Vòng đời → §Mâu thuẫn #1. `completed_late`: kho này tách hai câu — `TreHan` so với hạn hiện hành, `HoanThanhDungHanBanDau` so với hạn ban đầu (`domain/nhiem_vu.go:298-303`). Không việc |
| `a037b76` | `router.py` `DELETE /{task_id}` · `service.py` `withdraw_task` (neo `:1101-1131`) · migration `0023` · `BulkDelete.tsx` | Xoá mềm, khoá `task.delete`, từ chối khi còn việc con; chọn nhiều dòng rồi gọi từng tuyến một (*"a bulk UPDATE would bypass the ORM audit hooks"*) | Tuyến: Tóm tắt #6. Bên kia **không** ghi lý do xoá; kho này bắt buộc lý do (`routes.go:1138-1139`) — kho này chặt hơn, giữ. Chọn nhiều: Tóm tắt #5 |
| `f66bc93` | migration `0024_task_delete_leaders` | Cấp `task.delete` cho chủ tịch/phó chủ tịch ở xã đã tạo, vì `task.*` chỉ bung lúc tạo xã | Ai cầm khoá là cấu hình từng xã (ADR 0040), không phải việc của mã. Không việc |
| `7764c8a` | `codes.py` · `repository.py` `code_ever_used` · migration `0025` · `TaskDetailDrawer.tsx` | Mã NV tự sinh/gõ tay, `UNIQUE (tenant_id, code)`, mã **cho rỗng** với nhiệm vụ hệ thống sinh; sửa được tiêu đề và mã | Tự sinh + gõ tay + không cấp lại: Tóm tắt #7. Kho này `ma NOT NULL` (`0006:196`) — mọi nhiệm vụ đều có số, kể cả do hệ thống sinh; bên kia chưa có lý do buộc kho này đổi. Sửa mã → §Mâu thuẫn #2. Đua cấp số: bên kia thử lại tối đa 999 lần rồi rơi về `NV-<6 hex>`; kho này trả 500 ở lần đua (đã ghi ở `nhiem-vu-tuyen-ghi.tiep_theo`). Không việc mới |
| `93cff7f` | `service.py` `update_task` · `TaskRegisterSection.tsx` | Ô sửa hạn cạnh mã và tên | §Mâu thuẫn #3 |

## Mâu thuẫn với quyết định đã chốt

**Không có** theo nghĩa điều kiện dừng #1: tra `kb/00-foundation/open-questions.json` và `kb/10-decisions/`, không ADR
nào, không câu `DECIDED` nào chốt cạnh vòng đời, sửa mã hay sửa hạn. #21 (ADR 0035 §C) chỉ chốt **danh sách mã**, và bảy
mã hai bên khớp nhau. Ba mâu thuẫn với đặc tả/luật của kho này ghi ở §Mâu thuẫn trên, kèm tên người quyết.

Không commit nào trả lời một câu `OPEN` của kho này. Cạnh `chuyen-tiep` đi tiếp (#1) chạm một **điều kiện dừng ghi trong
sổ**, không phải câu mở của khách — ghi ở bảng #1 để người giữ sổ xác nhận, không suy.

## Không ảnh hưởng

| Commit / vùng | Vì sao bỏ |
|---|---|
| `a037b76` `BudgetItemTable.tsx` · `BudgetRankingTable.tsx` · `BudgetWorkspace.tsx` · `useBudget.ts` | M3 giải ngân, ngoài phạm vi Nhiệm vụ |
| `a037b76` · `f66bc93` migration `0023`/`0024` `INSERT … role_permissions` | Cơ chế cấp quyền của bên kia; kho này cấp quyền là cấu hình xã (ADR 0040) và khoá đã gieo ở `0001_init.sql:309` |
| `7764c8a` phần giao diện drawer (độ rộng `44rem`, màu khối sửa) · `56cfc3b` `TaskKanbanBoard.tsx` chặn thả sai cột | Cách cài đặt; điều thật sự đổi luật đã ghi ở Tóm tắt #4 |
| `4e27144` `TaskCard.tsx` · `TaskListTable.tsx` · `TaskRegisterTable.tsx` | Chỉ gọi `taskDisplayState` để tô màu |
| URL `/api/v1/tasks/{id}` (UUID) ↔ `/api/v1/tasks/{ma}` | Tên và khoá tài nguyên URL cố ý lệch |

## Chưa đọc trong lượt này

Các commit trước neo khác chạm `tasks/` hoặc `components/tasks/` — **không** đọc, nên đây là danh sách chứ không phải khoảng:
`95195b1` · `ec0faed` · `63cad73` (phần nhiệm vụ; phần họp đã đọc 25/09) · `0534afd` · `3bd4092` · `1065f1a` · `39fba4a` ·
`67c1ddb` · `9b58235` · `f5ed9be` · `7100d0f` · `db69542`. Vòng đời của chúng vẫn được phủ, vì lượt này đọc **bảng ở neo**
chứ không đọc diff. Phần còn lại (sổ theo dõi `3bd4092`, báo tiến độ `67c1ddb`, vùng sửa `db69542`) là việc của một lượt khác.

---

## Vì sao `anh_huong` là hai module này

| Module | Việc thật sự đòi đổi mã hoặc thiết kế |
|---|---|
| `service-petitions` | Ba câu quyết #1–#3 đều đổi máy trạng thái, trigger `nhiem_vu_bat_bien` hoặc thân `PATCH`; #4 thêm trường vào `nhiemVuRa` |
| `web-admin` | Bản sao `CHUYEN_DUOC` phải đổi theo #1 hoặc bỏ đi khi có #4; ô sửa mã/hạn theo #2/#3; nút "Xoá đã chọn" (#5) |

**Cố ý không đưa vào:** `service-identity` — `task.delete` đã gieo; ai được cấp là cấu hình xã. **`docs/ui-ux/02-nhiem-vu.md`**
tự mâu thuẫn với phần cài đặt ở §5.4 (`:166`, `:168` cho sửa mã và hạn) — không phải module nên không vào `anh_huong`; người sở
hữu tệp ấy sửa theo lời quyết #2/#3.
