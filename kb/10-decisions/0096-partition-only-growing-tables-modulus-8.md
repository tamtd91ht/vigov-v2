---
id: 0096-partition-only-growing-tables-modulus-8
tier: T1
source: CURATED
owner: architecture
derived_from_commit: 54bb094f
expires: null
owns_facts:
  - "tiêu chí bảng nào được phân mảnh: chỉ bảng có số dòng tăng theo thời gian hoặc theo số công dân; bảng danh mục/cấu hình/quyền/thiết lập/bộ đếm là bảng thường (chủ dự án, 15/10/2026)"
  - "số mảnh hiện hành MODULUS 8, và vì sao 8 chứ không 32, 4 hay 5"
  - "đổi số mảnh về sau không phải viết lại toàn bộ bảng: tách từng mảnh sang MODULUS 16/32"
  - "cách dựng lại bảng đang chạy (chép đủ dòng, kiểm số dòng trong cùng giao dịch, rồi DROP cấu trúc cũ) và việc chủ dự án cho phép DROP ấy, kể cả audit_log (15/10/2026)"
---

# 0096. Chỉ bảng lớn mới phân mảnh, và phân `MODULUS 8`

**Trạng thái:** đã chốt · **Ngày:** 2026-10-15 · **Chủ dự án chốt** cả năm điểm dưới đây, từng
điểm một, trong phiên 15/10/2026 · **Thay thế một phần** ADR 0010 §*Phân mảnh vật lý theo tenant*
và ADR 0013 quyết định #4 (chỉ các điểm ở §*Thay thế gì*; thân hai ADR ấy giữ nguyên từng chữ, chỉ
thêm dòng trỏ có ngày)

## Bối cảnh

ADR 0010 chốt `PARTITION BY HASH (tenant_id)` với `MODULUS 32`. ADR 0013 #4 chốt "khai bảng phân
mảnh thì tạo mảnh trong cùng tệp". Hai câu đó đúng, nhưng khi áp, **mọi** bảng có `tenant_id` đều
được khai phân mảnh — kể cả bảng quyền, cấu hình, danh mục. Đếm trong phiên 15/10/2026: khoảng
**106 bảng × 32 = 3.392 bảng lá** trên 7 CSDL dịch vụ, cộng 32 bản sao của mỗi chỉ mục. Lệnh đếm lại:
`grep -ciE 'PARTITION BY HASH ?\(tenant_id\)' service-*/migrations/*.sql` (số dòng gồm cả chú thích,
nên cao hơn số bảng).

Chủ dự án: *"nó đang phình rất lớn, chỉ có các bảng lớn mới dùng thôi"*; *"các bảng nhỏ như phân
quyền, cấu hình... thì đâu cần"*.

ADR 0004 #3 vốn chỉ đòi phân mảnh **bảng lớn** (`vanban`, `donthu`, `audit`). Việc phân mảnh mọi bảng
là cách áp đã đi xa hơn ADR 0004; ADR này kéo về đúng ý ban đầu và chốt thêm số mảnh.

## Phương án

| Phương án | Được | Mất |
|---|---|---|
| Giữ `MODULUS 32` cho mọi bảng (hiện trạng) | Không phải di trú | ~3.520 lá; mỗi chỉ mục ×32; planner, `pg_class`, autovacuum, sao lưu đều trả theo số lá |
| Bỏ phân mảnh bảng nhỏ, giữ 32 cho bảng lớn | Bảng nhỏ gọn | 38 × 32 ≈ **1.200 lá** — vẫn nhiều, mà 200 xã / 32 ≈ 6 xã mỗi mảnh là mịn hơn nhu cầu hiện có |
| **Bỏ phân mảnh bảng nhỏ, bảng lớn `MODULUS 8`** — chọn | 38 × 8 = **304 lá**; ≈25 xã mỗi mảnh ở 200 xã, pruning còn bỏ được 7/8 dữ liệu | Phải dựng lại 106 bảng đang chạy; bảng nhỏ mất lớp chặn thứ hai (xem §*Cách ly không đổi*) |
| `MODULUS 4` | Ít lá hơn nữa | ≈50 xã mỗi mảnh — pruning gần như vô ích |
| Bảng nhỏ "tối đa 5 mảnh" (ý đầu tiên của chủ dự án) | — | 5 mảnh trên vài trăm dòng không cho gì, vẫn nhân mọi chỉ mục ×5; 5 không chia hết vào 8/16/32 nên không đi chung bậc tách mảnh |

## Quyết định

| # | Quyết định |
|---|---|
| 1 | Chỉ phân mảnh bảng có số dòng **tăng theo thời gian hoặc theo số công dân** (sổ, nhật ký, phiếu, phiên, lượt gửi, tệp, vết kiểm toán) |
| 2 | Bảng phân mảnh dùng **`PARTITION BY HASH (tenant_id)`, `MODULUS 8`** |
| 3 | Bảng danh mục, cấu hình, quyền, thiết lập, bộ đếm (vài chục – vài trăm dòng mỗi xã, không tăng theo thời gian) là **bảng thường** |
| 4 | Bảng đã chạy được **dựng lại** bằng migration mới của từng dịch vụ — không sửa migration đã áp (ADR 0013) |
| 5 | Chủ dự án **cho phép DROP** cấu trúc cũ sau khi chép đủ dòng, **kể cả `audit_log`** |

### 38 bảng giữ phân mảnh (`MODULUS 8`) ngày 15/10/2026

Đây là chính quyết định nên ghi tên. Danh sách **hiện hành** về sau đọc từ migration, không đọc
từ đây; bảng mới theo tiêu chí #1.

| Dịch vụ | Bảng |
|---|---|
| Cả 7 dịch vụ | `audit_log` |
| petitions | `phieu_phan_anh`, `nhat_ky_phan_anh`, `su_kien_di`, `petition_log_attachment`, `petition_merge_event`, `nhiem_vu`, `nhat_ky_nhiem_vu`, `task_log_attachment`, `stored_file`, `staff_notice_outbox` |
| documents | `van_ban_den`, `van_ban_di`, `lich_su_chuyen_van_ban`, `citizen_letter`, `citizen_letter_log`, `staff_notice_outbox` |
| comms | `thong_bao_gui_cong_dan`, `thong_bao`, `thong_bao_nguoi_nhan`, `staff_notification`, `zalo_delivery`, `zalo_group_delivery`, `stored_file`, `portal_sync_runs` |
| identity | `phien`, `phien_cong_dan`, `ghep_phien`, `quan_he_cong_dan_xa`, `automation_run` |
| finance | `chung_tu_giai_ngan`, `project_comments` |

Mọi bảng đang phân mảnh khác (68) thành bảng thường. Trong đó có 15 bảng nằm sát ranh giới mà chủ
dự án **chọn rõ** bỏ phân mảnh: `de_nghi_lui_han`, `nhiem_vu_van_ban`, `task_issued_code`,
`petition_staff_recorded_rating`, `bien_ban_hop`, `ket_luan_hop`, `thong_bao_bo_phan`,
`noi_dung_mini_app`, `map_asset`, `disaster_alerts`, `citizen_notification_preferences`, `du_an`,
`project_issues`, `gia_tri_khoan_muc`, `gia_tri_dot`. Ghi tên để phiên sau không "sửa lại" vì thấy
chúng có vẻ lớn.

## Vì sao 8

| Lý do | Chi tiết |
|---|---|
| Quy mô | 200 xã / 8 ≈ 25 xã mỗi mảnh — truy vấn mang `tenant_id` vẫn chỉ đọc 1/8 bảng |
| Tách dần được | 8 là luỹ thừa của 2. PostgreSQL cho các mảnh HASH có **modulus khác nhau** trên cùng bảng, miễn mỗi modulus chia hết modulus lớn hơn kế tiếp. Một mảnh `MODULUS 8, REMAINDER r` nóng lên thì tách riêng nó thành `MODULUS 16` dư `r` và `r+8` — chỉ dòng của mảnh ấy được chép, bảy mảnh kia không động tới |
| Số lá | 304 thay ~3.520 |

**Sửa một câu nói quá của ADR 0010** (§*Phân mảnh vật lý theo tenant*, *"Đổi số này sau khi có
dữ liệu là viết lại toàn bộ bảng"*): chỉ đúng nếu đổi modulus của **cả bảng một lần**. Theo dòng
"Tách dần được" ở trên, tăng số mảnh là tách **từng mảnh**, không viết lại cả bảng. Chính câu nói quá
đó là một lý do 32 bị chọn cao ngay từ đầu.

## Cách ly không đổi

Phân mảnh **không phải** cơ chế cách ly xã. Cách ly vẫn do luật 1: `tenant_id NOT NULL`, mọi khoá
duy nhất ghép với `tenant_id`, repository có phạm vi, chỉ mục bắt đầu bằng `tenant_id` (ADR 0004 #2).
Tất cả giữ nguyên trên bảng thường.

Cái mất đi là **lớp chặn thứ hai về hiệu năng** ADR 0010 nêu ("quên `tenant_id` thì chậm thấy rõ").
Trên bảng vài trăm dòng mỗi xã, quên `tenant_id` cũng không chậm thấy được, nên lớp đó vốn không có
tác dụng ở đây. Lớp chặn thật vẫn là `tenant_scope_guard` và repository có phạm vi.

## Dựng lại bảng đang chạy

CSDL dev và prod đều đã có bảng. Prod lúc này **chỉ chứa dữ liệu thử** (chủ dự án xác nhận
15/10/2026), nhưng thủ tục viết như thể là dữ liệu thật — migration này sẽ chạy y nguyên trên mọi CSDL.

Mỗi bảng bị ảnh hưởng, trong **một giao dịch** (một tệp migration, ADR 0013 #5):

| Bước | Việc |
|---|---|
| 1 | Tạo bảng mới (thường, hoặc `MODULUS 8` kèm vòng tạo 8 mảnh trong cùng tệp — ADR 0013 #4 vẫn đúng) |
| 2 | Chép **toàn bộ** dòng |
| 3 | **Kiểm số dòng hai bảng bằng nhau**; lệch thì `RAISE EXCEPTION` — giao dịch rollback, service không khởi động, không mất gì |
| 4 | Dựng lại ràng buộc, chỉ mục, trigger, khoá ngoại; đặt lại sequence về giá trị lớn nhất đã cấp (luật 7 #3: mã đã cấp không cấp lại) |
| 5 | Đổi tên: bảng mới mang tên cũ; **tên ràng buộc giữ đúng như cũ** (xem §Hệ quả, mục mã đọc tên ràng buộc) |
| 6 | `DROP` bảng phân mảnh cũ |

**Vì sao DROP được phép ở đây.** Luật 7 cấm `DROP` trên dữ liệu nghiệp vụ vì nó huỷ hồ sơ. Ở đây
không dòng nào bị huỷ: bước 3 chứng minh bảng mới có đủ dòng trước khi bước 6 chạy, và cả hai nằm
trong cùng giao dịch. Cái bị DROP là **cấu trúc**, không phải hồ sơ. Chủ dự án cho phép rõ ràng ngày
15/10/2026; ngoài loạt migration này, luật 7 vẫn nguyên.

**`audit_log` cũng được dựng lại** sang `MODULUS 8` bằng cách chép đủ. Luật 6 có điều kiện dừng về
sửa/xoá vết; chủ dự án đã được hỏi và cho phép. Không vết nào bị sửa hay mất. Trên bảng mới phải dựng
lại đúng như ADR 0013 #1–#3: trigger chặn UPDATE/DELETE mức dòng trên bảng **cha**, và trigger chặn
`TRUNCATE` trên **từng lá trong 8 lá mới**. Thiếu trigger `TRUNCATE` ở lá mới là lỗi không triệu
chứng — 0013 đã giải thích vì sao.

## Thay thế gì của ADR cũ

| ADR | Điểm bị thay | Thay bằng |
|---|---|---|
| 0010 | §*Phân mảnh vật lý theo tenant*: `MODULUS 32` và bảng lý do | `MODULUS 8`, §*Vì sao 8* |
| 0010 | Cùng mục: *"Đổi số này sau khi có dữ liệu là viết lại toàn bộ bảng"* | Tách từng mảnh, §*Vì sao 8* |
| 0010 | Cùng mục, dòng `PARTITION BY HASH` "Bắt buộc từ migration đầu", hiểu là cho mọi bảng | Chỉ bảng đạt tiêu chí #1 |
| 0010 | Cùng mục, *"Một tác dụng phụ có lợi"* (partition là lớp cưỡng chế thứ hai) | Chỉ còn đúng cho bảng phân mảnh; bảng thường không có lớp ấy — §*Cách ly không đổi* |
| 0013 | #4 hiểu theo cách áp: mọi bảng tenant đều phân mảnh, vòng 32 mảnh; câu *"Mọi bảng nghiệp vụ mọc ra từ đó"* (§*Vì sao tạo mảnh thuộc cùng tệp*); *"hở cả 32 mảnh"* (§*Vì sao trên bảng CHA*) | #4 **vẫn đúng** cho bảng phân mảnh (tạo mảnh cùng tệp); số mảnh là 8, và bảng thường không có vòng nào |
| 0024 | §Hệ quả, dòng 3 (`0024-so-huu-danh-muc-tham-chieu.md:113`): bảng danh mục *"phân mảnh theo `tenant_id` như mọi bảng nghiệp vụ khác"* | Bảng danh mục là **bảng thường** (#3) |
| 0029 | Dòng 3 bảng ràng buộc của bảng SLA (`0029-so-huu-bang-sla.md:92`): *"Phân mảnh theo `tenant_id`"* | Bảng thường (#3) |
| 0021 | Ví dụ *"256 phân mảnh `_p00`…`_p31`"* (`0021-khai-quyen-so-huu-thuc-the.md:61`) | Lý lẽ của dòng ấy không đổi; chỉ con số cũ đi (7 `audit_log` × 8 = 56 mảnh) |

**Khẳng định lại:** ADR 0004 #3 — chỉ bảng lớn phân mảnh. ADR này đưa cách áp về đúng câu ấy.

**Không thay:** `tenant_id` là shard key và cột đầu mọi chỉ mục (ADR 0004). Trigger append-only trên
bảng cha, `TRUNCATE` theo lá (ADR 0013 #1–#3). Migration đã áp là bất biến (ADR 0013).

## Hệ quả

- **Dễ hơn:** ~3.520 lá còn 304; bảng nhỏ đọc, sao lưu, `\d` như bảng thường; thêm bảng cấu hình mới
  không phải viết vòng tạo mảnh
- **Khó hơn:** mỗi bảng mới phải được xếp vào một bên của tiêu chí #1. Không rõ bên nào thì hỏi —
  đừng mặc định phân mảnh "cho an toàn", đó đúng là cách 106 bảng ra đời
- **Phải trả ngay (việc khác làm, không thuộc tệp này):**

| Việc | Ở đâu |
|---|---|
| Migration dựng lại bảng, mỗi dịch vụ một tệp | `service-*/migrations/` |
| Hook nhắc `MODULUS 32` trong thông báo chặn | `.claude/hooks/tenant_scope_guard.py` (thông báo quanh `:338-344`) |
| Test nhắc `MODULUS 32` | `core/migrate/phan_manh_test.go:73`; `tools/test_hooks.py:149` |
| Khoá ngoại trên bảng phân mảnh dựng lại ở bước 4 phải kiểm ngay, không `NOT VALID` | PostgreSQL trước 18 từ chối `ADD CONSTRAINT … NOT VALID` trên bảng phân vùng; cụm thật là PG 16 (`kb/90-ephemeral/ban-giao-phien.md:157`) |
| Báo người vận hành: số bảng `kiem-tra` in ra sẽ giảm khoảng 3.000 | `deploy/Jenkinsfile:352` đếm mọi `BASE TABLE`, tức đếm cả lá. Số tụt sau loạt migration này là **đúng**, không phải mất bảng |
| Khối `DO $$` chặn thiếu mảnh cuối các migration, thông báo nhắc vòng 32 | ví dụ `service-identity/migrations/0002_audit_log_append_only.sql:200` — tệp đã áp không sửa; khối mới mang số 8 |
| Test migration khẳng định `modulus 32` từng bảng | tìm bằng `grep -rl "modulus 32" service-*/migrations/*_test.go` (không liệt kê tay — luật 9) |
| Mã đọc **tên ràng buộc** trong lỗi từ CSDL | ví dụ `service-identity/internal/store/can_bo_ghi.go:571-586` khớp **đuôi** tên ràng buộc vì bảng phân mảnh báo tên của lá. Khớp đuôi vẫn chạy trên bảng thường, **nếu** bước 5 giữ đúng tên ràng buộc; tìm hết chỗ khớp tên ràng buộc trước khi đổi tên |
| Chú thích nhắc "32 partitions" trong mã và test | tìm chuỗi `32 partitions` và `32 mảnh` trong `*.go` |
| `make check` xanh sau tất cả | — |

→ ADR 0004 (bảng lớn phân mảnh, shard key): `kb/10-decisions/0004-shard-by-tenant.md`
→ ADR 0010 (hạ tầng dữ liệu, số mảnh cũ): `kb/10-decisions/0010-data-infrastructure.md`
→ ADR 0013 (tạo mảnh cùng tệp, trigger append-only): `kb/10-decisions/0013-schema-migration-and-append-only.md`
→ Luật 1, 6, 7: `.claude/rules/critical/`
