---
id: doi-chieu-2026-10-04-feat-m8-multitenant-foundation-so-tay
tier: T2
source: CURATED
owner: architecture
derived_from_commit: c97f0cd8
expires: null
kho_nguon: vigov-require
branch: feat/m8-multitenant-foundation
sha_tu: 0053854
sha_den: 0053854
ngay_review: 2026-10-04
anh_huong: [service-petitions, web-admin]
owns_facts:
  - "ngày 04/10/2026 bên kia không có commit mới trên branch nào (neo 0053854 giữ nguyên)"
  - "Sổ tay lãnh đạo của vigov-require tại 0053854 lọc hai cột Chờ tôi duyệt và Việc tôi đã giao bằng vị từ nào, và lệch đặc tả 03 cùng mã kho này ở đâu"
---

# Đối chiếu `feat/m8-multitenant-foundation` · Sổ tay lãnh đạo · 04/10/2026

`0053854` → `0053854` · **0 commit mới** · đọc ngày 04/10/2026 · cộng một lượt đọc **tại** `0053854`, phạm vi hai cột
*Chờ tôi duyệt* và *Việc tôi đã giao* của `/nhiem-vu/so-tay`. Lý do có lượt này: chủ dự án dặn *"đối chiếu require trước
nhé"* trước khi dựng menu `so-tay-lanh-dao` (sổ `web-admin` → `tong-quan-va-so-tay`: *"/nhiem-vu/so-tay: chưa khởi công"*).

## A. Kiểm neo

| Kiểm | Kết quả |
|---|---|
| `git fetch --all` | `origin/feat/m8-multitenant-foundation` = `0053854` (24/09). `origin/main` = `93cff7f`, `origin/demo` = `8cd3924` — `git log 0053854..origin/<b>` rỗng ở cả hai, tức là tổ tiên |
| Commit chưa đọc chạm nhiệm vụ / sổ tay / duyệt | **Không có** — không có commit chưa đọc nào. Neo **không dời** |
| Cây làm việc bên kia | `HEAD` = `0053854`; còn `M pnpm-lock.yaml`, `?? apps/miniapp/HANDOFF.md` như 02/10 — chưa commit, ngoài phạm vi |

## B. Tóm tắt — kho này phải làm gì

| # | Loại | Bên kia (tại `0053854`) | Kho này | Kho này phải làm | Module |
|---|---|---|---|---|---|
| 1 | **CONFLICT** | *Chờ tôi duyệt* = mọi việc `pending_approval` của xã, **không lọc theo người, không lọc theo khoá** (`LeaderNotebook.tsx:49-51`) | Đặc tả: `cho-duyet` AND (`lanh_dao_giao_viec = me` OR `task.approve` **với bộ phận đó**) (`docs/ui-ux/03-so-tay-lanh-dao.md:42`). Mã: duyệt `cho-duyet` là khoá **phẳng toàn xã** `task.approve` (`service-petitions/internal/domain/nhiem_vu.go:189-191`; ADR 0065 trả lời #2, `:81`); vế "theo bộ phận" không biểu diễn được bằng luật 5 bất biến 3 (`kb/90-ephemeral/ke-hoach-so-tay-bien-ban.md:62-67`) | Chủ dự án chốt vị từ trước khi dựng — §D #1 | service-petitions · web-admin |
| 2 | **PARTIALLY DONE** | Cột **không** gom đề nghị lùi hạn; chỉ in `đã gia hạn N lần` (`LeaderNotebook.tsx:178-183`). Cờ `pending_extension` có trên mỗi nhiệm vụ (`schemas.py:293`, `service.py:176,189`) nhưng màn không đọc | Đặc tả **thêm** đề nghị lùi hạn chờ tôi duyệt (`03:42`). Máy chủ **đã có** hàng chờ `GET /api/v1/task-extensions`, `approver=me` lọc `lanh_dao_giao_viec_ma` lấy từ phiên (`store/de_nghi_lui_han_cho_duyet.go:40-46`, `:116-119`) — đúng người theo ADR 0038:35 | Web: gọi hàng chờ ấy cho nửa này. Không đổi máy chủ | web-admin |
| 3 | **CHANGED** | *Việc tôi đã giao* = `created_by == me` AND `status !== "done"` (`LeaderNotebook.tsx:52-54`) — **không** đọc `assigner_id`, dù chính họ ghi hai người khác nhau (`models.py:260-264`, `docs/spec/05-nghiep-vu.md:22-23`) | Đặc tả: `nguoi_tao_id = me` OR `lanh_dao_giao_viec_id = me` (`03:43`). Mã: tách `nguoi_tao_ma` / `lanh_dao_giao_viec_ma` (ADR 0038:25-28); scope `related` đã đọc **cả hai** (`store/nhiem_vu.go:489-491`, `:514-515`) nhưng còn kèm người thực hiện, nhật ký, bộ phận — không phải cột này | Thêm một phạm vi "tôi giao" phía máy chủ — hôm nay `scope` chỉ nhận `all`/`mine`/`related` (`http/nhiem_vu.go:731-742`) | service-petitions · web-admin |
| 4 | **ALREADY DONE** (khớp) | "Đã đóng" = chỉ `done`; `paused`, `transferred`, `pending_approval` **vẫn hiện** (`LeaderNotebook.tsx:53`) | Đặc tả chỉ viết *"mọi trạng thái chưa đóng"* (`03:43`), không định nghĩa. Mã: "chưa xong" = `ngay_hoan_thanh IS NULL`, `chuyen-tiep` tính là chưa xong (`store/nhiem_vu.go:449-452`) | Ghi rõ "chưa đóng = ≠ `hoan-thanh`" vào đặc tả khi dựng | — |
| 5 | **CHANGED** (lệch có lợi) | Ba cột tính **ở trình duyệt** trên **một** lần gọi `GET /api/v1/tasks`, `roots_only=false`, không `scope`, **`limit: 300`** (`useTasks.ts:166-168`, `LeaderNotebook.tsx:26,47-54`); máy chủ xếp `created_at desc` rồi cắt (`repository.py:142-144`). Xã quá 300 nhiệm vụ → **số trên huy hiệu đếm thiếu**, im lặng | Đặc tả đòi số **khớp tuyệt đối** với `/nhiem-vu` và test so hai nguồn (`03:82`); đề xuất tuyến riêng hoặc ba bộ lọc (`03:65-74`) | Đếm phía máy chủ, mỗi cột một vị từ dùng chung với `/nhiem-vu`. **Không** chép cách 300 dòng | service-petitions · web-admin |
| 6 | **UNKNOWN** | Đặc tả của chính họ: `pending_approval` **ẩn khỏi giao diện theo yêu cầu của xã** (`docs/spec/05-nghiep-vu.md:10-12`) — vậy mà Sổ tay và Kanban vẫn dựng cột ấy (`task-display.ts:97-103`) | Bước duyệt là **tuỳ chọn** (ADR 0065 NV1 + trả lời #2) — cột sẽ thường rỗng; hình mẫu đặc tả cũng vẽ `[0]` (`03:26`) | Không việc mã. Lý do cột này hay rỗng là đúng, không phải lỗi | — |

## C. Chi tiết

### C.1 Cột "Chờ tôi duyệt" — bên kia nói gì

| Chỗ | Nguyên văn / hành vi |
|---|---|
| `docs/SRS.md:168` | *"Màn hình "Sổ tay lãnh đạo": mặc định chỉ hiện việc trễ + việc cần duyệt + việc mình giao. (P0)"* — **"việc cần duyệt"**, không có chữ "tôi" |
| `LeaderNotebook.tsx:49-51` | `all.filter((task) => task.status === "pending_approval")` |
| `LeaderNotebook.tsx:85` | Tiêu đề **"Chờ tôi duyệt"** — chữ "tôi" chỉ có ở nhãn, vị từ không có |
| `LeaderNotebook.tsx:33` | `canApprove = hasPermission(permissions, "task.approve")` — chỉ chuyền vào ngăn chi tiết, **không** lọc cột |
| `navigation.ts:69-75` | Menu mở cho `task.read` — mọi cán bộ đọc được nhiệm vụ đều thấy cột này đầy đủ |
| `service.py:564-567`, `:622-641` | Máy chủ **không kiểm `task.approve`** ở bước `pending_approval → done`: ai có `task.update` hoặc là người được giao thì chuyển được. `task.approve` chỉ được gieo (`org/data/default_config.json:29`) và kiểm ở giao diện |
| `service.py:657-665` | Khi việc vào `pending_approval`, chuông báo về **`created_by`** — không về `assigner_id` |
| `workers/sla.py:337-349` | Bản tin đầu tuần đếm `pending_approval` **toàn xã**, gửi mọi vai `is_leadership` |
| Lùi hạn | Báo/thư về `assigner_id or created_by` (`service.py:727-740`; `docs/spec/05-nghiep-vu.md:69-71`). Duyệt: cửa `task.extend` (`router.py:227-230`), tầng nghiệp vụ chỉ cấm **người tạo tự duyệt đề nghị của chính mình** (`service.py:814-818`). Nút duyệt trong ngăn chi tiết hiện theo `task.approve` (`TaskDetailDrawer.tsx:547`) — **ba khoá khác nhau cho một việc**, và đề nghị **không** vào Sổ tay |

**Ai thấy mục "chờ duyệt" ở bên kia: mọi người có `task.read`.** Không phải người duyệt, không phải người giao.

### C.2 Cột "Việc tôi đã giao" — bên kia nói gì

| Chỗ | Nguyên văn / hành vi |
|---|---|
| `docs/SRS.md:168` | *"… + việc mình giao"* |
| `LeaderNotebook.tsx:52-54` | `all.filter((task) => myId !== null && task.created_by === myId && task.status !== "done")` |
| `repository.py:65-70` | Phạm vi "liên quan" của họ cũng chỉ đọc `created_by` cho vế "tôi giao" — `assigner_id` vắng ở cả hai chỗ |
| `models.py:260-264` | Chú thích của chính họ: *"Lãnh đạo giao việc: người ra lệnh, không phải người bấm nút tạo … văn thư nhập hộ là cách phần lớn nhiệm vụ vào hệ thống, và khi ấy hai người là hai người khác nhau"* |
| `tests/test_task_assigner.py:3`, `:82` | Kiểm `assigner_id` cho **lùi hạn**; không test nào cho Sổ tay |

**Hệ quả đọc được ở bên kia:** nhiệm vụ văn thư nhập hộ hiện ở cột của **văn thư**, cột của lãnh đạo ra lệnh thì trống. Bên
kia không có test hay tài liệu nào nói đó là chủ ý.

### C.3 Ngoài hai cột — ghi để khỏi đọc lại

| Mục | Bên kia | Kho này |
|---|---|---|
| Cột quá hạn | `is_overdue` loại `done` **và** `transferred` (`service.py:117-127`); không xếp theo số ngày trễ | `03:41` chỉ loại `hoan-thanh`, xếp giảm theo ngày trễ (`03:58`). Ngoài phạm vi lượt này |
| Trang mặc định cho lãnh đạo | Không có | `03:83` đề xuất. Chưa ai chốt |
| Hướng dẫn sử dụng | Chú thích ảnh: *"ghi nhanh việc cần giao, chuyển thành nhiệm vụ khi đã rõ người và hạn"* (`apps/admin/public/tai-lieu/huong-dan/index.html:255`) — **không khớp** mã (`LeaderNotebook.tsx` không có ô ghi nào) | **UNKNOWN**: lời BA cho tính năng tương lai hay chú thích sai. Không ghi vào phạm vi khi chưa hỏi |

## D. Mâu thuẫn với quyết định đã chốt — CHƯA CHỌN BÊN

### #1 Vị từ "Chờ tôi duyệt" — ba nguồn, ba vị từ

| Nguồn | Vị từ |
|---|---|
| vigov-require `LeaderNotebook.tsx:49-51` | `pending_approval`, toàn xã, ai đọc được nhiệm vụ cũng thấy |
| Đặc tả kho này `03:42` | `cho-duyet` AND (`lanh_dao_giao_viec = me` OR `task.approve` **với bộ phận đó**) + đề nghị lùi hạn chờ tôi |
| Quyết định kho này | ADR 0065 trả lời #2 (`:81`): *"việc đã gửi lên chờ duyệt thì chỉ người cầm `task.approve` duyệt hoặc trả lại"* — khoá phẳng, toàn xã, **không** theo `lanh_dao_giao_viec_ma`. Lùi hạn thì ngược lại: **đúng người** `lanh_dao_giao_viec_ma` (ADR 0038:35) |

Đặc tả `03:42` mâu thuẫn với ADR 0065 ở hai chỗ: (a) vế "theo bộ phận" không có trong luật 5; (b) vế `lanh_dao_giao_viec = me`
không cầm `task.approve` thì thấy việc trong cột "chờ **tôi** duyệt" nhưng máy chủ từ chối khi bấm. Không chọn bên ở đây —
**ai quyết: chủ dự án.** Câu hỏi gọn ở báo cáo trả về.

### Điều kiện dừng khác

Không có commit mới, nên không có commit nào đổi `DECIDED` hay trả lời câu `OPEN`.

## E. Không ảnh hưởng

Không có commit nào để bỏ qua. Đã đọc và **không** ghi thành mục: `so-tay/page.tsx:1-9` (chỉ bọc component) ·
`guide-sections.ts:20` (mục lục hướng dẫn) · `sla.py:84-98` `_leaders` (chọn người nhận bản tin, không phải vị từ cột).

→ Ghi chú trước: `kb/50-doi-chieu/2026-09-26-feat-m8-multitenant-foundation-nhiem-vu.md` · Kế hoạch: `kb/90-ephemeral/ke-hoach-so-tay-bien-ban.md`
