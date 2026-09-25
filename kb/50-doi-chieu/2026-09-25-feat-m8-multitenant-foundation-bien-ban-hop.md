---
id: doi-chieu-2026-09-25-feat-m8-multitenant-foundation-bien-ban-hop
tier: T2
source: CURATED
owner: architecture
derived_from_commit: 40dfc73
expires: null
kho_nguon: vigov-require
branch: feat/m8-multitenant-foundation
sha_tu: 0053854
sha_den: 0053854
ngay_review: 2026-09-25
anh_huong: [service-petitions, web-admin]
owns_facts:
  - "năm commit M1 TRƯỚC NEO (95195b1 · ec0faed · 63cad73 · a05b1ef · e99a5cf) lệch gì so với biên bản họp của service-petitions và web-admin, trong phạm vi meeting/conclusion"
  - "người dùng đã quyết gì về vòng đời, trường và trạng thái kết luận của biên bản họp ngày 25/09/2026"
---

# Đối chiếu `feat/m8-multitenant-foundation` · biên bản họp · 25/09/2026

`0053854` → `0053854` · 0 commit mới · đọc ngày 25/09/2026 · cộng một **danh sách** 5 commit trước neo

| Phần | Đọc gì |
|---|---|
| A | Kiểm lại neo: `fetch --all --prune`, cây bên kia sạch, `HEAD` = `origin/feat/m8-multitenant-foundation` = `0053854`. `git log 0053854..origin/…` **rỗng**. Neo **không dời** |
| B | M1 biên bản họp **trước neo**: `95195b1` · `ec0faed` · `63cad73` · `a05b1ef` · `e99a5cf` (cả năm ngày 24/08/2026, đều là tổ tiên của `0053854`, kiểm bằng `git merge-base --is-ancestor`). Chỉ đọc phần Meeting/Conclusion |

`sha_tu` = `sha_den` = `0053854` vì khoảng đọc mới **rỗng**. Phần B là **danh sách**, không phải khoảng — nới
`sha_tu` về `95195b1^` là khai đã đọc mọi commit giữa đó (cùng lý do ghi chú `2026-09-25-feat-m8-multitenant-foundation.md` §B nêu).

**Vì sao tệp riêng, không nối vào ghi chú 25/09 đã có.** `require_sync_guard.py:130-143` coi một ghi chú là đã tiếp
nhận khi sổ module **chứa tên tệp**. `tien-do/web-admin.json` đã trích `2026-09-25-feat-m8-multitenant-foundation.md`
(cho thu-chi). Nối phần biên bản vào tệp ấy thì `web-admin` thành "đã tiếp nhận" **ngay lập tức**, không ai mở việc
biên bản — rào xanh vì lý do sai. Tệp riêng giữ rào đúng nghĩa. Tên tệp này không là chuỗi con của tên kia và ngược lại.

**Phạm vi đọc bên kia.** `apps/api/app/modules/tasks/{models,router,schemas,service,repository}.py` phần Meeting/Conclusion ·
migration `20260824_0003-0003_tasks.py` `_create_meetings` · `apps/admin/src/components/tasks/MeetingMinutes.tsx` (tại `0053854`;
sau `e99a5cf` chỉ `d906f22` chạm tệp, đổi một lớp CSS) · `org/{router,schemas,service}.py` của `e99a5cf` ·
`docs/spec/03-mo-hinh-du-lieu.md:267-273,562-571` · `docs/spec/04-api.md:202-206` · `docs/SRS.md:130,134,177,619`.
Mô hình Meeting/Conclusion **không đổi** từ `95195b1` tới `0053854` (`git diff 95195b1 0053854 -- …/tasks/models.py` không chạm hai lớp ấy).

**Phạm vi đối chiếu kho này.** `service-petitions/migrations/0007_bien_ban.sql` · `internal/{domain,app,http,store}/bien_ban_hop*.go` ·
`internal/http/routes.go:1109-1224` · `web-admin/src/features/bien-ban/*` · `docs/ui-ux/04-bien-ban-hop.md`.

---

## Tóm tắt — kho này phải làm gì

"Đã tiếp nhận" ở cột 4 nghĩa là **người dùng đã quyết ngày 25/09/2026** trong lượt `/develop-feature` Biên bản họp.
Theo `kb/50-doi-chieu/README.md`, ghi chú chỉ **được coi là** đã tiếp nhận khi `tien-do/service-petitions.json` và
`tien-do/web-admin.json` trích **tên tệp này**. Việc ấy thuộc agent xây module.

| # | Bên kia | Kho này đang có | Đã tiếp nhận (người dùng 25/09) | Module |
|---|---|---|---|---|
| 1 | Không có vòng đời. `meetings` không cột trạng thái (`95195b1` `models.py:68-85`); không `PATCH`/`DELETE` (`ec0faed` `router.py:275-343`) | Không trạng thái, không tuyến sửa/xoá. `0007:81-89` **cố ý để ngỏ** câu "kết luận đã tách nhiệm vụ còn sửa lời được không" | **Dự thảo → Đã ký.** Sau ký **khoá** nội dung, kết luận, chủ trì, thư ký. Sai sau ký thì lập **biên bản bổ sung**, không sửa bản đã ký | service-petitions · web-admin |
| 2 | Không có thư ký, không số/ngày Thông báo kết luận. Chỉ `reference_no` (số biên bản) | `so_hieu` chép tay, không duy nhất (`0007:91-96`). Không thư ký, không Thông báo kết luận | **Thêm thư ký** (mã cán bộ) + **số và ngày Thông báo kết luận**, **chép tay, không tự cấp số** | service-petitions · web-admin |
| 3 | Mỗi kết luận chỉ có hai đếm `task_count` · `done_count` (`ec0faed` `schemas.py:230-239`, `repository.py:292-312`). Dòng phụ: "Chưa tách" hoặc "x/y nhiệm vụ đã hoàn thành" (`MeetingMinutes.tsx:139-143`). **Không có quá hạn** | Y hệt hai đếm (`domain/bien_ban_hop.go:126-154`, `store/bien_ban_hop.go:165-168`; `nhan-bien-ban.ts:164-167`) | Trạng thái kết luận **SUY RA từ nhiệm vụ**: chưa giao · đang thực hiện · quá hạn · hoàn thành. Thêm **dấu do người đặt** "không phát sinh nhiệm vụ" | service-petitions · web-admin |
| 4 | Thẻ: "`N` kết luận · `x/y` **nhiệm vụ** xong" (`MeetingMinutes.tsx:122-125`) | Cùng câu (`nhan-bien-ban.ts:146-155`; đặc tả `04-bien-ban-hop.md:55,165`) | Số chính của thẻ là **"x/y kết luận hoàn thành"**. Trùng ý `SRS.md:177` ("bao nhiêu kết luận, bao nhiêu đã xong") hơn cả hai bản cài đặt | service-petitions · web-admin |
| 5 | Sắp theo `held_at DESC`, `LIMIT 100` cứng, không phân trang (`ec0faed` `repository.py:261-269`) | Phân trang theo `tao_luc` (`0007:254-262`; `store/bien_ban_hop.go:52-67`); đã hiện lên màn là phần chưa dựng (`nhan-bien-ban.ts:292-299`) | **Sắp theo ngày họp** | service-petitions · web-admin |
| 6 | `e99a5cf`: danh bạ hẹp `GET /org/directory` cho mọi tài khoản đăng nhập, không email/SĐT; hộp thoại tách dùng nó | Tuyến hẹp **đã có**: `GET /api/v1/staff-directory` (`service-identity/internal/http/routes.go:114,492`; sổ `service-identity/tuyen-danh-ba-can-bo-hep`, `8ba677c`). Nhưng màn biên bản vẫn ghi ô Chủ trì "không dựng được vì `/staff` đòi `admin.user`" (`nhan-bien-ban.ts:244-252`) — **đã lỗi thời** | Không cần quyết mới. Ô chọn **Chủ trì** và **Thư ký** (#2) lấy từ `staff-directory` | web-admin |

## Chi tiết theo phân hệ

### M1 — Biên bản và kết luận họp

| Commit | Tệp bên kia | Đổi gì | Hệ quả ở kho này |
|---|---|---|---|
| `95195b1` | `tasks/models.py:68-85` · migration `0003` `:88-106` | `meetings`: `title` · `held_at TIMESTAMPTZ` · `location` · `chaired_by_id` · `reference_no` · `minutes`. Không thành phần, không thư ký, không trạng thái, không tệp đính kèm | `held_at` giờ-phút vs `ngay_hop DATE` (`0007:180-193`): kho này **giữ DATE**, lý do đã ghi — ngày họp không đếm hạn, lưu thời điểm là bịa giờ. Không việc. Thiếu trạng thái/thư ký → Tóm tắt #1, #2 |
| `95195b1` | `models.py:88-108` · migration `:108-122` | `conclusions` `UNIQUE (tenant_id, meeting_id, ordinal)` **tính cả dòng xoá mềm** | Khớp `0007:308-316`. Không việc |
| `95195b1` | `models.py:50-54,133,156-160` | Nguồn nhiệm vụ `source_type = 'meeting'` + `source_id` = id kết luận, không khoá ngoại | Cùng hình với `nguon_giao = 'ket-luan-hop'` + `nguon_id` (`0007:56-76`). Mã nguồn tiếng Anh vs tiếng Việt là vùng đặt tên cố ý lệch. Không việc |
| `ec0faed` | `schemas.py:208-218` · `service.py:553-563` | `POST /meetings` nhận **đủ** `location`, `chaired_by_id`, `reference_no`, `minutes` | Tuyến ghi của kho này đã nhận `held_on`, `chaired_by`, `attendees` (`http/bien_ban_hop_ghi.go:81,90,102`). Cần thêm thư ký, số/ngày Thông báo kết luận (#2) |
| `ec0faed` | `service.py:566-588` | Số thứ tự kết luận = `len(kết luận chưa xoá) + 1` | **Lỗi tiềm ẩn bên kia, đo từ mã:** sau một lần xoá mềm, số mới trùng số đã cấp và vấp `UNIQUE` (tính cả dòng xoá). Bên kia chưa có tuyến xoá nên chưa lộ. Kho này cấp `max + 1` trên số đã cấp (`routes.go:1197`). Không việc |
| `ec0faed` | `service.py:591-602` · `router.py:328-343` | Tách = tạo nhiệm vụ với `source_type`/`source_id` **ghi đè từ đường dẫn**, client không đặt được | Khớp `routes.go:1219-1222`. Không việc |
| `ec0faed` | `repository.py:292-312` · `service.py:616-641` | Đếm nhiệm vụ theo kết luận: tổng và `status = done`. **Không đếm quá hạn** | Trạng thái suy ra (#3) cần thêm đếm quá hạn. Quá hạn nhiệm vụ đã suy ra ở kho này (`domain/nhiem_vu.go` `TreHan`, theo `han_xu_ly`); câu đếm của biên bản phải dùng **cùng** định nghĩa, không định nghĩa thứ hai (luật 10 bất biến 3) |
| `ec0faed` | `router.py:275-343` | Năm tuyến, khoá `task.read` / `task.create`; không khoá `meeting.*` | Khớp `routes.go:1118-1126`. Hành động **Ký** (#1) là hành động mới: khoá nào → xem §Câu còn mở, dòng 1 |
| `63cad73` | `MeetingMinutes.tsx:306-404` (`NewMeetingDialog`) | Form chỉ có tên, ngày (`type=date` đổi sang ISO lúc nửa đêm UTC), số hiệu, nội dung. **Không** địa điểm, chủ trì, thành phần, danh sách kết luận dù API nhận | Form kho này đã có địa điểm, thành phần, danh sách kết luận (`so-bien-ban.tsx:756,763,807`). Chủ trì còn thiếu → #6 |
| `63cad73` | `MeetingMinutes.tsx:187-304` (`SplitDialog`) | Hộp thoại tách **riêng**: tên, bộ phận, người, hạn (tuỳ chọn). Ưu tiên **cứng `"cao"`** (`:215`). Không gợi ý hạn từ câu kết luận (`:37-38`: M1.1.6 để sprint sau) | Kho này dùng lại nguyên `FormGiaoViec` (`so-bien-ban.tsx:13,582`), đúng `04-bien-ban-hop.md:71`. Hạn gợi ý: kho này cũng không đoán, lý do ở `routes.go:1134-1137` và `nhan-bien-ban.ts:230-243`. Bên kia **không trả lời** câu ấy — họ hoãn, không chốt. Không việc |
| `63cad73` | `TaskDetailDrawer.tsx` (tại `0053854`) | Không hiện liên kết về biên bản gốc | `04-bien-ban-hop.md:164` đòi liên kết ấy; kho này cũng chưa có. Bên kia không có bản thiết kế để mượn. Không việc mới |
| `e99a5cf` | `org/router.py` `read_directory` · `org/schemas.py` `DirectoryEntry` · `MeetingMinutes.tsx:195` | Danh bạ hẹp (id, họ tên, chức danh, bộ phận, cờ hoạt động) cho mọi tài khoản đăng nhập; hộp thoại tách chuyển sang dùng | Tóm tắt #6. Kho này đã quyết cùng hình ở identity (`8ba677c`), trả **mã cán bộ** chứ không id — khớp cột `chu_tri_ma` (`0007:201-206`). Việc còn lại chỉ ở `web-admin` |
| `0053854` | `docs/SRS.md:619` (J1 câu 2) | *"Quy trình họp giao ban thực tế: bao lâu một lần, biên bản lập thế nào, ai theo dõi kết luận hiện nay?"* — câu bên kia **vẫn mở** với khách | Quyết định 25/09 (#1–#4) là của người dùng kho này, trả lời vế "biên bản lập thế nào, ai theo dõi". Bên kia chưa có chữ của khách cho vế ấy. Nên báo BA để câu J1.2 hỏi khách có thêm vòng đời dự thảo/đã ký |

### Câu còn mở — lời quyết 25/09 chưa nói tới, builder phải hỏi, không tự đặt

| # | Câu | Vì sao không tự quyết |
|---|---|---|
| 1 | Hành động **Ký** cần khoá quyền nào? Bảng `quyen` không có `meeting.*` (`routes.go:1118-1126`) | Luật 5 bất biến 3c: khoá không có trong `quyen` là phát hiện cho câu mở **#27**, không phải một `INSERT` mới. Dùng lại `task.create` là cho mọi người tạo nhiệm vụ được ký biên bản |
| 2 | Có được **tách nhiệm vụ khi biên bản còn dự thảo** không? | Được thì sửa lời kết luận ở dự thảo vẫn đổi câu mà nhiệm vụ đang trỏ vào — đúng lo ngại `0007:81-89`. Không được thì phải chờ ký mới giao việc |
| 3 | Biên bản bổ sung **nối** về bản gốc thế nào, và kết luận trong bản bổ sung đánh số riêng hay nối tiếp bản gốc | Lời quyết nói "lập biên bản bổ sung", không nói hình dạng liên kết. Đoán cột là chốt hình dạng trên hồ sơ lưu trữ |
| 4 | Kết luận có **nhiều nhiệm vụ lẫn trạng thái** (một xong, một quá hạn) thì ra trạng thái nào | Bốn trạng thái đã chốt, thứ tự ưu tiên thì chưa |
| 5 | Kết luận mang dấu "không phát sinh nhiệm vụ" có tính vào **x** của "x/y kết luận hoàn thành" không; đặt dấu rồi còn tách nhiệm vụ được không; dấu ấy khoá sau ký không | Ba câu đổi con số chính trên thẻ |

## Mâu thuẫn với quyết định đã chốt

**Không có.** Tra `kb/00-foundation/open-questions.json` và `kb/10-decisions/`: không câu `DECIDED` hay ADR nào về
biên bản họp. Năm commit không trả lời câu `OPEN` nào của kho này — dùng chung `task.read`/`task.create` ở hai bên
**không** trả lời #27 (câu của khách).

## Không ảnh hưởng

| Commit / vùng | Vì sao bỏ |
|---|---|
| `a05b1ef` `demo_thangbinh.json` · `seed_demo.py` | Dữ liệu mẫu của một xã có tên; kho này cố ý không gieo (`0007:31-34`). Chỉ đọc hình dạng khoá, **không chép dữ liệu** |
| `ec0faed` `importer.py` · `workers/sla.py` · `notifications/*` · `test_task_rules.py` | Không tham chiếu meeting/conclusion. Ngoài phạm vi lượt này |
| `63cad73` phần nhiệm vụ, sổ tay (`TaskKanbanBoard` · `TaskListTable` · `LeaderNotebook` · `useTasks.ts` ngoài phần meeting) | Ngoài phạm vi meeting/minutes. Nhiệm vụ đã đối chiếu ở ghi chú 23/09 và 24/09 |
| `e99a5cf` `BudgetWorkspace.tsx` · `LeaderNotebook.tsx` · `TaskAssignForm.tsx` · `TaskWorkspace.tsx` | Chỉ đổi `useUsers` → `useDirectory`; luật đã ghi ở dòng `e99a5cf` trên |
| `d906f22` `MeetingMinutes.tsx` | Chỉ đổi lớp độ rộng hộp thoại |
| URL `/api/v1/meetings/**` · `/org/directory` | Tên tài nguyên URL cố ý lệch; kho này đã chọn `meetings` (`routes.go:1111-1116`) và `staff-directory` |
| RLS + `GRANT` không `DELETE` trong migration `0003` | Kiến trúc cố ý lệch; kho này chặn xoá cứng bằng trigger (`0007:151-159`) |

---

## Vì sao `anh_huong` là hai module này

| Module | Việc thật sự đòi đổi mã hoặc thiết kế |
|---|---|
| `service-petitions` | Migration mới: trạng thái dự thảo/đã ký + khoá sau ký, thư ký, số/ngày Thông báo kết luận, dấu "không phát sinh nhiệm vụ" trên `ket_luan_hop`, chỉ mục + con trỏ theo `ngay_hop`; câu đếm thêm quá hạn và đếm kết luận hoàn thành; tuyến ký, tuyến sửa dự thảo, tuyến biên bản bổ sung (#1–#5) |
| `web-admin` | Nút ký và trạng thái; ô Chủ trì/Thư ký từ `staff-directory`; hai ô Thông báo kết luận; nhãn bốn trạng thái kết luận; số chính "x/y kết luận hoàn thành"; bỏ mục `PHAN_CHUA_DUNG` đã lỗi thời (Chủ trì, thứ tự thẻ) khi dựng xong (#1–#6) |

**Cố ý không đưa vào:** `service-identity` — tuyến danh bạ hẹp đã xong (`8ba677c`); khoá quyền cho hành động Ký là
câu #27, chưa phải việc của identity cho tới khi khách trả lời.

**`docs/ui-ux/04-bien-ban-hop.md` phải đổi theo** (§2 dòng 55, §4 bảng trường, §6, §7.5): không phải module nên
không vào `anh_huong`; người sở hữu tệp ấy cập nhật theo quyết định 25/09.
