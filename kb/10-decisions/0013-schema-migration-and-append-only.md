---
id: 0013-schema-migration-and-append-only
tier: T1
source: CURATED
owner: architecture
derived_from_commit: 0122d554
expires: null
owns_facts:
  - "vì sao append-only của audit_log cưỡng chế bằng trigger, không bằng REVOKE trong migration"
  - "vì sao trigger đặt trên bảng CHA và ở mức DÒNG"
  - "vì sao tạo mảnh nằm cùng tệp với khai bảng phân mảnh"
  - "cách migration schema được áp: nhúng trong binary, áp lúc khởi động, hỏng thì service không chạy"
  - "đường thứ hai (14/10/2026): tạo bảng trước bằng `/server -migrate-only` từ việc tao-bang của vigov-deploy — vì sao cùng runner, vì sao ảnh Deployment đang chạy, cái gì vẫn đóng, và vì sao 'có bảng mà không có sổ' chỉ bị chặn ở đường hai"
  - "giới hạn: luật 7 bất biến 5 mới đạt phần DDL — backfill theo từng xã chưa tồn tại"
---

# 0013. Di trú schema và cưỡng chế append-only ở tầng CSDL

**Trạng thái:** đã chốt · **Ngày:** 2026-09-17

> 15/10/2026 → ADR 0096: chỉ bảng lớn phân mảnh, `MODULUS 8`. Quyết định #4 (tạo mảnh cùng tệp) vẫn
> đúng cho bảng phân mảnh; chỗ nào dưới đây nói "32 mảnh" hay ngụ ý mọi bảng nghiệp vụ đều phân mảnh
> thì đọc theo ADR 0096 (`kb/10-decisions/0096-partition-only-growing-tables-modulus-8.md`).
>
> 15/10/2026 → ADR 0096 §*Đưa thay đổi vào CSDL đang chạy*: **ngoại lệ một lần** với "migration đã áp
> là bất biến" (#5, dòng *Tiền-kiểm*) — chủ dự án cho sửa thẳng tệp đã áp và xoá sạch, dựng lại mọi CSDL
> dev/prod (chỉ có dữ liệu thử). Ngoài loạt tệp ấy, quy tắc dưới đây nguyên vẹn.

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
`identity/migrations/0002_audit_log_append_only.sql`, bản giống nhau ở cả 8 service.
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
`core/migrate` làm **hai vế sau, cho DDL**, và **không làm gì cho vế đầu**.

Không phải thiếu sót của gói: **"theo từng xã" không áp cho DDL.** Schema là chung cho cả CSDL
của một service — một bộ bảng, phân mảnh theo `tenant_id` (ADR 0010), **không phải** một bộ
bảng cho mỗi xã. Đường chạy này không có xã trong nó và cố ý không nhận xã nào.

Vế đầu áp cho **BACKFILL DỮ LIỆU**: viết lại các dòng đã có, mỗi lần một xã, nối lại được
giữa chừng, để một lượt backfill đứt sau xã 40/200 đi tiếp từ 41 thay vì chạy lại từ đầu và
**chạm vào hồ sơ lưu trữ hai lần**.

**Cơ chế đó chưa tồn tại.** Nó khác hình dạng: cần bảng tiến độ theo xã, cỡ lô, và cách dừng
rồi chạy tiếp — không thứ nào một trình chạy DDL dùng đến.

> **Sự có mặt của `core/migrate` KHÔNG có nghĩa là luật 7 bất biến 5 đã đạt.** Nửa backfill
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

## Sửa đổi 14/10/2026 — đường thứ hai: tạo bảng trước, từ Jenkins

Chủ dự án chốt 14/10/2026, cả bốn điều. Dựng ở `db849c25` (`kiem-tra`), `f510b746` (`tao-bang`) và
`0122d554` (`-migrate-only` trong `core/migrate` và bảy dịch vụ).

| # | Quyết định | Dựng ở |
|---|---|---|
| 1 | `HANH_DONG=kiem-tra` của `vigov-deploy` báo thêm CSDL của **từng dịch vụ**: nối được không, số bảng, sổ `schema_migration` so với tệp của kho. Chỉ đọc | `deploy/Jenkinsfile:333-459` |
| 2 | Việc mới `HANH_DONG=tao-bang`: tạo bảng **trước** khi dịch vụ khởi động, bằng `/server -migrate-only` | `deploy/Jenkinsfile:477-566`; `core/migrate/only.go:42`; nhánh đầu `main()` của bảy dịch vụ, ví dụ `service-platform/cmd/server/main.go:92-93` |
| 3 | Tệp đã có trong sổ → bỏ qua, không báo lỗi. Sổ lệch → **dừng dịch vụ ấy, không áp gì**. Chủ dự án chọn cách này, **không** chọn "nhận CSDL có sẵn vào sổ" (adopt) | `core/migrate/migrate.go:160-166`; `core/migrate/only.go:87-102`; xem "Có bảng mà không có sổ" ở dưới |
| 4 | Ảnh chạy `-migrate-only` là **ảnh Deployment đang chạy**, không phải thẻ gõ tay | `deploy/Jenkinsfile:509` |

Quyết định #5 ở trên nay đọc là: migration áp lúc khởi động, **hoặc** áp trước bằng `tao-bang`. Đường
khởi động không đổi.

### Vì sao là cùng một runner

`RunMigrateOnly` gọi đúng `migrate.Chay` mà lúc khởi động gọi (`core/migrate/only.go:105`), với cùng tệp
nhúng và cùng tên dịch vụ (`service-platform/cmd/server/main.go:93` và `:152`). Tên dịch vụ là khoá
advisory lock. Pod `tao-bang` và một bản sao đang khởi động vì vậy xếp hàng sau nhau, không áp chồng.
Sổ, tiền-kiểm checksum và "mỗi tệp một giao dịch" là của chung, không có bản thứ hai.

Không viết runner thứ hai bằng `psql`, dù job đã có sẵn một cái cho kho khác. `vihat-miniapp/deploy/migrate.sh`
áp tệp và ghi dòng sổ bằng **hai lệnh `psql` riêng** (`:49` và `:50`). Đứt ở giữa thì tệp đã chạy mà sổ
không ghi, lần sau áp lại lần nữa. Nó cũng không lưu checksum. Hai runner cho một CSDL là hai định nghĩa
"đã áp", và chúng sẽ lệch nhau.

Vì sao ảnh đang chạy, không phải thẻ gõ tay: thẻ mới hơn ảnh đang chạy sẽ áp tệp mà mã đang phục vụ
không mang. Bản sao kế tiếp khởi động sẽ gặp `ErrThieuTep` (`core/migrate/migrate.go:80-83`) và từ chối
chạy. Lấy ảnh của Deployment thì schema khớp đúng mã đang phục vụ nó.

### Cái gì vẫn đóng

| Ca | Hành vi | Ở đâu |
|---|---|---|
| Tệp đã áp bị sửa (checksum lệch) | Dừng **trước khi áp tệp nào** | `core/migrate/migrate.go:164`, `:269-274`; test `core/migrate/only_test.go:85` |
| CSDL có tệp mà ảnh không mang (CSDL mới hơn ảnh) | Dừng, nêu tên tệp | `core/migrate/migrate.go:279-291` |
| Có bảng mà không có sổ — **chỉ ở đường hai** | Dừng trước `Chay`, không áp gì, thoát mã 1 | `core/migrate/only.go:87-102`, `:123-133`; test `only_test.go:116`, `:143` |
| Thiếu hoặc sai `ENV` / `DATABASE_DSN` | Thoát mã 2, không mở CSDL | `core/migrate/only.go:64-68`; test `only_test.go:165` |
| Một tệp hỏng | Tệp ấy rollback, dừng; các tệp trước đã commit được in tên | `core/migrate/only.go:107-115` |
| Ảnh dựng trước 14/10/2026 (không hiểu `-migrate-only`, chạy máy chủ thật) | Không in dòng `migrate-only:` trong 60 giây → gỡ pod, bỏ qua dịch vụ | `deploy/Jenkinsfile:533-545` |
| Lỗi kết nối | In ra không kèm DSN hay mật khẩu | `core/migrate/only.go:136-147`; test `only_test.go:208-241` |

Không rollback tự động, không baseline, không bỏ qua tệp hỏng — vẫn như mục "Vì sao runner áp lúc khởi
động" ở trên. Sổ lệch là việc của **người**.

### Có bảng mà không có sổ: chặn ở đường hai, đường khởi động vẫn hở

Chủ dự án chốt cho `tao-bang`: sổ lệch → dừng. Ca "CSDL có bảng mà chưa có `schema_migration`" thuộc
nhóm lệch, vì `Chay` một mình **không** thấy nó:

1. `chayTrongKhoa` tạo `schema_migration` nếu chưa có, rồi áp từ tệp đầu (`core/migrate/migrate.go:151-181`).
2. Tệp `0001_init.sql` của cả bảy dịch vụ viết `CREATE TABLE IF NOT EXISTS` và `CREATE INDEX IF NOT EXISTS`
   (`deploy/README.md` mục 3 "Mẫu giá trị", cảnh báo "BẢY DỊCH VỤ PHẢI CÓ BẢY CSDL"). Trên CSDL đã có
   bảng, 0001 nhiều khả năng chạy qua im lặng. Các tệp sau có hỏng hay không thì **chưa biết**.
3. Kết quả là "nhận CSDL có sẵn vào sổ" (adopt), đúng cách chủ dự án đã bác — chỉ là không ai chọn.

Đường hai (`-migrate-only`) kiểm ca này **trước** `Chay`. `to_regclass('schema_migration')` cộng số bảng
thường trong `current_schema()` (`core/migrate/only.go:123-133`). Có bảng mà không có sổ → in
`migrate-only: <dv> FAILED: database has <n> tables but no schema_migration ledger — not adopting; a person
decides (ADR 0013)`, thoát mã 1, không áp gì (`core/migrate/only.go:98-101`).

**Đường khởi động KHÔNG đổi**: `Chay` vẫn tạo sổ rỗng và áp lại từ 0001. Giới hạn phép kiểm vào đường hai
là lựa chọn **phạm vi của phiên phát triển** (14/10/2026), **không** phải quyết định của chủ dự án. Lý do:
một phép từ chối ở đường khởi động có thể làm một dịch vụ prod đang chạy không lên lại được. Ví dụ
`vigov_comms` còn chứa bảng của `vihat-miniapp` (`deploy/README.md` mục 13.6, việc `di-tru-vihat-miniapp`).
Chủ dự án chưa được hỏi có đưa phép kiểm vào đường khởi động hay không.

| Đường | Có bảng mà không có sổ |
|---|---|
| `tao-bang` (`-migrate-only`) | **Dừng**, không áp gì |
| Dịch vụ khởi động (`Chay`) | **Không dừng**: tạo sổ, áp lại từ 0001 |
| `kiem-tra` | In `⚠ CÓ n BẢNG mà KHÔNG có sổ` (`deploy/Jenkinsfile:432-437`) |

Sau một lần dịch vụ khởi động, sổ đã tồn tại, nên `kiem-tra` không còn in dòng ⚠ ấy và `tao-bang` không
còn dừng. Vì vậy thấy dòng ấy thì **không khởi động dịch vụ đó trước** — người quyết.

### Rủi ro chưa kiểm

| Rủi ro | Biết đến đâu |
|---|---|
| NetworkPolicy chặn đường ra CSDL của pod một lần | Pod mang nhãn riêng `app.kubernetes.io/name=vigov-migrate-only` (`deploy/Jenkinsfile:519`), để Service không chuyển yêu cầu vào nó. Theo `deploy/base/mang/netpol.yaml:177-197` (`allow-egress`, `podSelector: {}`), mọi pod ra được DNS và cổng 5432. **Chưa kiểm trên cụm thật**: cụm có thể đang chạy NetworkPolicy khác bản trong kho. Bị chặn thì log in `FAILED: cannot reach the database` |
| Chưa chạy thật lần nào | `tao-bang` chỉ làm được khi bảy ảnh dựng từ commit có `-migrate-only` đã lên. Trước đó mọi dịch vụ báo "ảnh cũ" |
| `kiem-tra` với `MT=dev` so với nhánh `main` | Job checkout `main`, còn `vigov-dev` chạy ảnh dựng từ `dev`. "Chưa áp" ở dev có thể chỉ là dev đi trước main (`deploy/Jenkinsfile:331-332`) |

Không đổi: job **không** tạo CSDL, tài khoản hay `REVOKE`. Các việc đó vẫn thuộc khâu cấp phát CSDL
(`deploy/README.md` mục 3 "Mẫu giá trị"). Cách đọc kết quả hai việc: `deploy/README.md` mục 2, bảng việc của
`vigov-deploy`.

→ ADR 0010 (số mảnh, `MODULUS 32`): `kb/10-decisions/0010-data-infrastructure.md`
→ ADR 0004 (shard key): `kb/10-decisions/0004-shard-by-tenant.md`
→ Luật 6 (sổ không sửa được): `.claude/rules/critical/6-audit-log.md`
→ Luật 7 (giữ dữ liệu, migration): `.claude/rules/critical/7-data-preservation.md`
→ Toàn văn lập luận trong mã: `identity/migrations/0002_audit_log_append_only.sql` ·
  `core/migrate/migrate.go`
