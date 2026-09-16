---
id: 0013-schema-migration-and-append-only
tier: T1
source: CURATED
owner: architecture
derived_from_commit: f9f35d4
expires: null
owns_facts:
  - "vì sao append-only của audit_log cưỡng chế bằng trigger, không bằng REVOKE trong migration"
  - "vì sao trigger đặt trên bảng CHA và ở mức DÒNG"
  - "vì sao tạo mảnh nằm cùng tệp với khai bảng phân mảnh"
  - "cách migration schema được áp: nhúng trong binary, áp lúc khởi động, hỏng thì service không chạy"
  - "giới hạn: luật 7 bất biến 5 mới đạt phần DDL — backfill theo từng xã chưa tồn tại"
---

# 0013. Di trú schema và cưỡng chế append-only ở tầng CSDL

**Trạng thái:** đã chốt · **Ngày:** 2026-09-17

## Bối cảnh

Ba sự việc xảy ra cùng một chỗ và có cùng một hình dạng: **một thứ trông như biện pháp
nhưng không phải biện pháp.**

| # | Sự việc |
|---|---|
| 1 | `0001_init.sql` **tuyên bố** audit_log là append-only trong comment. Schema không cưỡng chế gì cả — không `REVOKE`, không rule, không trigger. Thứ duy nhất giữ bất biến là việc chưa ai gõ câu lệnh sửa |
| 2 | Sáu service khai `PARTITION BY HASH (tenant_id)` mà **không tạo mảnh nào**. Bảng phân mảnh không mảnh **từ chối mọi INSERT**; vì vết đi cùng giao dịch với nghiệp vụ (luật 6 #3), bản ghi nghiệp vụ **đầu tiên** rollback toàn bộ |
| 3 | 16 tệp migration đã commit và **không có gì áp chúng**. Hai chỗ duy nhất đọc migration là test tích hợp, cả hai hardcode `0001_init.sql` |

Ba sự việc cộng lại: một bất biến pháp lý (luật 6 #4 — sổ không sửa được) được bảo vệ bởi
một comment, trong một tệp chưa từng chạy, trên một bảng chưa nhận nổi một dòng.

ADR 0010 chốt `MODULUS 32` — **số mảnh**, và chỉ số mảnh. Nó không nói ai tạo ra chúng, nên
khoảng hở đó tồn tại hợp lệ cho tới đây.

## Quyết định

| # | Quyết định |
|---|---|
| 1 | Append-only cưỡng chế bằng **trigger CSDL**. `REVOKE` **không** nằm trong migration — nó thuộc khâu cấp phát CSDL |
| 2 | Trigger đặt trên **bảng cha**, **`FOR EACH ROW`**, cho UPDATE/DELETE. INSERT cố ý không có trong danh sách |
| 3 | Chống `TRUNCATE` gắn **theo từng lá** qua `pg_inherits` — PostgreSQL không cho gắn trigger loại đó lên bảng phân mảnh |
| 4 | **Khai một bảng phân mảnh gồm luôn việc tạo mảnh của nó**, trong **cùng một tệp** |
| 5 | Migration **nhúng vào binary** (`go:embed`), áp **một lần lúc khởi động**, mỗi tệp một giao dịch, hỏng thì **service không khởi động** |

Toàn văn lập luận của (1)–(3) nằm trong chính tệp migration —
`services/identity/migrations/0002_audit_log_append_only.sql`, bản giống nhau ở cả 8 service.
Dưới đây chỉ giữ phần một người đọc **sáu tháng sau** cần để khỏi "sửa lại cho đúng".

## Vì sao trigger chứ không REVOKE

`REVOKE UPDATE, DELETE, TRUNCATE` là câu trả lời ai cũng nghĩ tới trước, nên phải nói rõ vì
sao nó **bị bỏ ra khỏi migration**: tên role của ứng dụng khác nhau theo môi trường, mà luật
8 #5 cấm giá trị theo môi trường nằm trong mã nguồn. Ba cách viết nó vào migration:

| Cách | Vì sao tệ hơn việc bỏ ra |
|---|---|
| `FROM current_user` | Ở hệ này migration chạy **cùng kết nối với ứng dụng**, nên `current_user` vừa là app role vừa là **chủ bảng** — chủ bảng cấp lại quyền cho chính mình trong một câu. Ghi một **ý định** trong khi trông như một **biện pháp** |
| Đọc role từ `current_setting(...)` | Không nơi nào trong kho mã đặt setting đó ⇒ khối là **no-op ở mọi môi trường** mà đọc ra như bảo vệ. Tệ nhất trong ba |
| Ghi thẳng tên role | Vỡ ở môi trường đầu tiên viết khác. **Một migration vỡ là một migration bị sửa dưới áp lực** |

Trigger lại phủ đúng chỗ `REVOKE` **không với tới được**: chủ bảng và superuser — hai chủ thể
được bỏ qua kiểm quyền hoàn toàn, và cũng chính là hai chủ thể luật 6 #4 gọi tên
("không sửa được, **kể cả bởi quản trị viên**").

**Điều tệp này không chặn, nói thẳng:** `ALTER TABLE ... DISABLE TRIGGER` và `DROP TABLE`, cả
hai chủ bảng làm được. Đó là hành vi DDL có chủ đích, để lại dấu trong log máy chủ. Mục tiêu
ở đây là làm cho cái **vô tình và tiện tay** trở nên bất khả — một `UPDATE` viết nhầm trong
Go, một `DELETE` gõ ở dấu nhắc psql — chứ không phải đánh bại một quản trị viên đã quyết định
huỷ hồ sơ và chấp nhận bị nhìn thấy.

## Vì sao trên bảng CHA, và vì sao mức DÒNG

Đây là chỗ một lựa chọn sai **không có triệu chứng**:

| | Hành vi |
|---|---|
| `FOR EACH ROW` trên bảng cha | Được **nhân bản xuống mọi partition**, hiện có **và tạo sau**. Migration sau thêm mảnh thì mảnh đó tự thừa hưởng |
| `FOR EACH STATEMENT` trên bảng cha | **KHÔNG** được nhân bản xuống partition |

Sửa/xoá trên bảng phân mảnh chạy vào **lá**, nên bản sao ở lá mới là thứ bắn. Đúng cả khi
planner định tuyến lẫn khi ai đó **gõ thẳng tên partition** trong psql — và ca thứ hai mới là
mối đe doạ luật 6 #4 nêu tên. Chọn mức câu lệnh thì trigger vẫn tồn tại, vẫn hiện trong
`\d audit_log`, và đường gõ thẳng tên partition **đi qua im lặng**.

Cùng cái bẫy đó tồn tại ở tầng quyền: quyền kiểm theo **đúng quan hệ nêu trong câu lệnh** và
**không thừa hưởng** từ bảng cha, nên `REVOKE` trên bảng cha không thôi để hở cả 32 mảnh.
Câu đúng, đủ cả hai dòng, giữ làm comment trong tệp 0002 cho khâu cấp phát.

**Chốt phiên bản:** trigger BEFORE mức dòng trên bảng phân mảnh chỉ có từ **PostgreSQL 13**.
Dưới 13 migration **dừng với thông báo đọc được**, thay vì lỗi thô — vì lỗi thô sẽ mời người
ta "sửa" bằng cách dời trigger xuống partition, tạo đúng cái lỗ vừa nói.

## Vì sao tạo mảnh thuộc cùng tệp với khai bảng

Tách "khai bảng phân mảnh" và "tạo mảnh của nó" thành hai tệp nghe gọn hơn, và sai:

1. Tệp khai bảng thành **vĩnh viễn hỏng-khi-chạy-một-mình** — mà chạy một tệp một mình đúng
   là việc **các bộ test tích hợp đang làm**. Một harness tương lai viết bằng cách copy
   harness hiện có sẽ tái tạo lại đúng lỗi này.
2. Lỗi không phải một lần sơ suất mà là **template**: sáu tệp khung viết
   `) PARTITION BY HASH (tenant_id);` rồi **dừng**. Mọi bảng nghiệp vụ mọc ra từ đó
   (`phan_anh`, `van_ban`, `giai_ngan`…) sẽ thừa hưởng y nguyên chỗ thiếu.

Nên quy ước là **một đơn vị**: bảng phân mảnh chưa có mảnh thì chưa khai xong.

Cưỡng chế ở **hai lớp**, cố ý:

| Lớp | Bắt lúc | Giới hạn |
|---|---|---|
| Khối `DO $$` cuối tệp 0002 (truy `pg_class`/`pg_inherits` trong `current_schema()`) | Lúc **chạy** migration | Chỉ kiểm trạng thái sau migration **mang** khối đó. Một `0003` tương lai quên vòng lặp chỉ bị bắt nếu nó cũng kết thúc bằng khối này — mà "nhớ thêm khối vào cuối" đúng là loại chỉ dẫn vừa thất bại |
| `.claude/hooks/tenant_scope_guard.py` | Lúc **gõ** | Trước đó hook chỉ xem `.go`, nên **toàn bộ 16 tệp migration nằm ngoài tầm mọi hook** — đó là lý do lỗi sống sót tới khi một agent tình cờ đọc ra |

## Vì sao runner áp lúc khởi động, và không có rollback

| Tính chất | Lý do |
|---|---|
| `go:embed`, không đọc đĩa | Binary mang theo schema của chính nó. Đọc từ đĩa nghĩa là container phải chứa **đúng phiên bản tệp** — và "đúng phiên bản" là thứ sẽ lệch |
| Advisory lock theo **tên dịch vụ** | Hai bản sao cùng khởi động sẽ cùng áp một migration. Theo tên dịch vụ để `identity` không xếp hàng sau `finance` |
| **Tiền-kiểm** checksum toàn bộ trước khi áp gì | Sửa một migration đã chạy = hai CSDL có hai schema khác nhau mà cùng khai mình ở cùng phiên bản. Migration đã áp là **bất biến**, cưỡng chế chứ không dặn dò |
| Mỗi tệp một giao dịch, kèm dòng tiến độ **trong cùng giao dịch** | Không có schema nửa vời, không có dòng tiến độ mồ côi |
| Hỏng ⇒ **service không khởi động** | Khởi động với schema sai là chạy trên một CSDL không đúng hình dạng mình tưởng |
| **Không** rollback tự động | Hoàn tác DDL trên hồ sơ lưu trữ là **thủ tục hành chính**, không phải thứ một tiến trình tự quyết lúc 3 giờ sáng (luật 7). Hỏng thì dừng và **đợi người** |

## Giới hạn — đọc trước khi coi luật 7 bất biến 5 là đã xong

Luật 7 bất biến 5 đòi migration chạy **"theo từng xã, nối lại được, ghi tiến độ"**.
`pkg/migrate` làm **hai vế sau, cho DDL**, và **không làm gì cho vế đầu**.

Không phải thiếu sót của gói: **"theo từng xã" không áp cho DDL.** Schema là chung cho cả CSDL
của một service — một bộ bảng, phân mảnh theo `tenant_id` (ADR 0010), **không phải** một bộ
bảng cho mỗi xã. Đường chạy này không có xã trong nó và cố ý không nhận xã nào.

Vế đầu áp cho **BACKFILL DỮ LIỆU**: viết lại các dòng đã có, mỗi lần một xã, nối lại được
giữa chừng, để một lượt backfill đứt sau xã 40/200 đi tiếp từ 41 thay vì chạy lại từ đầu và
**chạm vào hồ sơ lưu trữ hai lần**.

**Cơ chế đó chưa tồn tại.** Nó khác hình dạng: cần bảng tiến độ theo xã, cỡ lô, và cách dừng
rồi chạy tiếp — không thứ nào một trình chạy DDL dùng đến.

> **Sự có mặt của `pkg/migrate` KHÔNG có nghĩa là luật 7 bất biến 5 đã đạt.** Nửa backfill
> vẫn đang mở. Ghi ở đây để không ai phải suy ra điều đó từ mã.

## Hệ quả

- **Dễ hơn:** "append-only" và "bảng phân mảnh có mảnh" thành thứ **CSDL từ chối vi phạm**,
  không còn là thứ người đọc comment phải nhớ
- **Khó hơn:** mọi bảng phân mảnh mới phải mang vòng lặp tạo mảnh; sửa một migration đã áp là
  **không làm được** — phải viết tệp mới
- **Phải trả sau:** cơ chế backfill theo từng xã; và `REVOKE` phải được thực hiện ở khâu cấp
  phát CSDL, nơi biết tên role — nếu khâu đó không làm, lớp quyền vẫn hở dù trigger vẫn đúng

**Chưa chạy thật ở đâu:** máy phát triển không có `VIGOV_TEST_DSN` nên hai bộ test tích hợp
**tự bỏ qua** — chúng báo ok mà không chạy một câu SQL nào. Nghĩa là tệp 0002, khối kiểm
PG ≥ 13, khối chặn thiếu mảnh và `pg_advisory_lock` thật **đều chưa từng thực thi**. Lần chạy
thật đầu tiên có thể làm CI đỏ; đọc thông báo trước khi sửa, rất có thể chúng đang làm đúng việc.

→ ADR 0010 (số mảnh, `MODULUS 32`): `kb/10-decisions/0010-data-infrastructure.md`
→ ADR 0004 (shard key): `kb/10-decisions/0004-shard-by-tenant.md`
→ Luật 6 (sổ không sửa được): `.claude/rules/critical/6-audit-log.md`
→ Luật 7 (giữ dữ liệu, migration): `.claude/rules/critical/7-data-preservation.md`
→ Toàn văn lập luận trong mã: `services/identity/migrations/0002_audit_log_append_only.sql` ·
  `pkg/migrate/migrate.go`
