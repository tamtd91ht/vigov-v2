---
id: 2026-09-16-ban-giao-phien
tier: T5
source: CURATED
owner: architecture
derived_from_commit: 96574b4
expires: 2026-12-16
owns_facts:
  - "trạng thái thi công tại 2026-09-17 và việc kế tiếp phải làm"
---

# Bàn giao phiên — mở 2026-09-16, cập nhật 2026-09-17

**Đọc tệp này SAU `kb/INDEX.yaml` và tầng `always_load`, không thay thế chúng.**
Nó chỉ trả lời: *đã làm tới đâu, việc kế tiếp là gì, và cạm bẫy nào đã gặp.*

Hết hạn **2026-12-16**. Sau ngày đó tin `git log` chứ đừng tin tệp này.

---

## 1. Đã làm

**Danh sách commit KHÔNG chép vào đây** — `git log --oneline f9f35d4..HEAD` trả lời câu đó
chính xác hơn và không bao giờ lệch. Dưới đây chỉ những thứ `git log` không trả lời được.

### Quyết định đã chốt với khách trong phiên

| # | Quyết định | Ghi ở |
|---|---|---|
| 1 | Tên service **tiếng Anh** (`petitions`), tên bảng **tiếng Việt** (`don_thu`) | ADR 0001 |
| 2 | SLA đếm bằng **giờ làm việc**, không phải ngày | ADR 0007 |
| 3 | Vòng đời phiếu là **cấu hình theo xã**, không hard-code | ADR 0008 |
| 4 | Bí mật theo xã: **envelope encryption**, KEK ngoài CSDL | ADR 0009 |
| 5 | **Chỉ PostgreSQL** (bác MongoDB) · Redis · Kafka · RabbitMQ · Elastic chỉ cho tìm kiếm | ADR 0010 |
| 6 | `MODULUS 32` cho mọi `PARTITION BY HASH` | ADR 0010 |
| 7 | Quyền là **một khoá phẳng** `"task.extend"` | luật 5, bất biến 3b |
| 8 | Admin tổng chỉ siêu dữ liệu — khớp ADR 0003 sẵn có, không cần ADR mới | — |
| 9 | **URL path tiếng Anh, giá trị enum giữ tiếng Việt không dấu**, tiền tố `/api/v1/` | ADR 0011 |
| 10 | Tên sự kiện chốt dạng `petitions.received.v1` — chưa publish nên đổi giá 0 | ADR 0011 |
| 11 | Xã đi trong **metadata gRPC**, khoá `x-tenant-id`; RPC tra cứu **mặc định theo lô**; xác thực service↔service **tạm dựa vào cách ly mạng**; `platform` sập = **404** | ADR 0012 |
| 12 | Append-only cưỡng chế bằng **trigger**, `REVOKE` giao cho khâu cấp phát; **tạo mảnh thuộc cùng tệp với khai bảng**; migration áp lúc khởi động, hỏng thì service không chạy | ADR 0013 |

### Trạng thái mã

**Chạy được, có test:**

| Gói | Nội dung |
|---|---|
| `core/tenant` `core/store` `core/audit` `core/events` `core/privacy` `core/httpx` | Có sẵn từ khung sườn; `core/tenant` nay giữ thêm `CachedDirectory` (chuyển lên từ `platform`, vì bảy service kia không import được `internal/` của nó — luật 2 cấm #1) |
| `core/config` | Hằng số nền tảng; cố ý không có `Get("...")`. Đã có tệp mẫu env |
| `core/password` | argon2id, `CanBamLai` để nâng chi phí |
| `core/authz` | `Perm` phẳng; test gồm đủ 4 ca luật 5 |
| `core/idem` | Chống trùng request, **khai opt-in từng route** |
| `core/grpcx` | Interceptor **hai đầu** + hằng khoá metadata. Danh sách miễn kiểm xã có **đúng một** thành viên: `ResolveHost` |
| `core/secret` | Kiểu **tự từ chối in**: chặn **mọi** verb `fmt` qua `Formatter`, không phải `String()`. Lối ra thô tên `Lo()`, đúng ba chỗ gọi trong toàn repo |
| `core/platformclient` | Client phân giải Host tới `platform`. Ở đây chứ không ở `core/tenant` vì `core/grpcx` đã import `core/tenant` → **vòng import** |
| `core/token` | Ký/kiểm token phiên, mang `sid` để thu hồi được (luật 5, bất biến 4) |
| `core/migrate` | Trình chạy migration: `go:embed`, advisory lock, tiền-kiểm checksum, mỗi tệp một giao dịch, hỏng thì service **không khởi động**. Cả 8 service đã nối vào `main.go` — ADR 0013 |
| `core/page` + `core/store/page.go` | Phân trang **cursor** dùng chung, viết **trước** route danh sách đầu tiên. Neo là `(cột sắp xếp, id)` — **khoá phá hoà là bắt buộc**, thiếu nó thì trang sau lặp hoặc nuốt bản ghi mà không ai thấy. Cursor **không ký**, **không mang `tenant_id`**, không mang dữ liệu cá nhân. Không có `total`, không có `page`. `QueryPage` đi qua `Scoped.Query`, không có đường nào tới `*sql.DB` |
| `platform/internal/{domain,store}` | Phân giải Host, cache, edge chain |
| `platform/internal/grpc` | Server gRPC — siêu dữ liệu, không có đường trả nội dung nghiệp vụ (ADR 0003) |
| `identity/internal/{domain,store,app}` | RBAC, `Checker`, phiên, đăng nhập |
| `identity/internal/http` | **Route đăng nhập/đăng xuất + middleware dựng `Principal`**, token ký HMAC |
| `identity/cmd/server` | **Wire chạy thật**: config → CSDL → migrate → Directory qua gRPC (có cache) → checker/canBo/phien → **một** signer → use case → idem → chuỗi edge → tắt êm |
| `proto/` + `gen/` | Hợp đồng đã sinh mã. `BatchGetStaff` thay `GetStaff`; `x-tenant-id` khai ở cả 8 tệp proto |
| `*/migrations/` | **17 tệp**, 8 service. `0002` biến "append-only" từ comment thành ràng buộc CSDL ở cả 8 service; `0003` (chỉ `identity`) tách `co_tai_khoan` khỏi `dang_hoat_dong`. **Xem §2.1: chưa tệp nào chạy thật** |

**Ba thay đổi gần nhất đáng nói vì cái `git log` không nói:**

| | |
|---|---|
| `co_tai_khoan` (schema `0003` + mã Go) | "Có tài khoản đăng nhập" và "chưa bị khoá" từng đọc chung **một** cột `dang_hoat_dong`. Hệ quả im lặng: 26 người chỉ có trong danh bạ bị đếm là "Đang hoạt động" — con số xã báo lên. Nay lọc ở **ba** đường: `CanBoStore.TheoEmail`, `TheoID`, và `truyVanQuyen` của `Checker`. Mặc định cột là **false** (đóng trước) |
| `Staff.username` gỡ, `reserved 3` + `reserved "username"` | Trường trỏ vào **một cột chưa từng tồn tại** và khai luôn một ràng buộc duy nhất không có thật. Số không tái dùng, **và tên cũng bị giữ chỗ** — để chữ đó không quay lại gắn vào dữ liệu khác |
| Gỡ **dữ liệu cá nhân thật** khỏi `docs/ui-ux/` | Họ tên + số di động thật của 7 lãnh đạo xã nằm trong đặc tả. Đã thay bằng số quy ước `0900000000` (luật 3, bất biến 5). Xem §7 #8: **lịch sử git vẫn còn** |

**Brain và cổng kiểm — đã siết trong phiên:**

| Thay đổi | |
|---|---|
| `tenant_scope_guard` nay đọc cả `.sql` | Bảng `PARTITION BY HASH` mà quên tạo mảnh bị chặn ngay lúc gõ, không phải lúc INSERT đầu tiên hỏng |
| `rest_api_guard` chặn `Public` đi kèm `idem.Required` | Một route công khai mà chống trùng bằng khoá do client tự đặt là một khoá do người lạ tự đặt |
| `doc_guard` cho **sửa** tệp có sẵn dưới `docs/ui-ux/` | Trước đó nó chặn đúng cái việc gỡ dữ liệu cá nhân ra khỏi đặc tả. Vẫn chặn **tạo mới** `.md` ngoài `kb/` |
| `data_safety_guard` vá lần thứ 3 | Xem §4 |
| `make check` | `gofmt` nay **chặn thật** (trước in tên tệp rồi thoát mã 0), `buf lint` bỏ tiền tố `-` nên lỗi không bị nuốt, `-race` bắt buộc, target `web` **bỏ qua một cách ồn ào** |

**Còn trống hoàn toàn:** **server gRPC của `identity`** (chưa có `internal/grpc`) · mọi route
HTTP **nghiệp vụ** · `documents` `petitions` `dossiers` `finance` `comms` `reporting` mới có
`main.go` + migration, chưa có nghiệp vụ · toàn bộ frontend (`*/src/` chỉ có `.gitkeep`)
· `core/storage` · `data-ownership.json` vẫn rỗng.

---

## 2. Việc kế tiếp — theo đúng thứ tự

### Luật chung cho mọi RPC viết từ đây trở đi

**Đọc `kb/10-decisions/0012-grpc-boundary-contract.md` trước khi thêm một RPC nào.** Ba điều
hay bị đọc sai nhất:

| | |
|---|---|
| `GetTenant` **bắt buộc** mang `x-tenant-id` | Việc nó nhận id xã **trong thân** là chuyện khác hẳn: id đó là **chủ thể đang được tra**, không phải lời khai của bên gọi về chính mình. Nhận id trong thân **không** kéo theo miễn trừ nào |
| Danh sách miễn kiểm xã có **đúng một** thành viên: `ResolveHost` | Vì nó **không thể** biết xã — nó chính là thứ đi tìm xã. Thêm thành viên thứ hai là **ĐIỀU KIỆN DỪNG**: hỏi người dùng |
| Service nghiệp vụ **không** được nhận id xã trong thân message | Hình dạng đó chỉ hợp lệ ở `platform` vì ADR 0003 chặn mọi đường tới nội dung nghiệp vụ |

### 2.1 Chạy migration thật, trên máy có `VIGOV_TEST_DSN` ← **BẮT ĐẦU TỪ ĐÂY**

Vẫn là việc chặn mọi thứ khác. Máy phát triển không có DSN nên các bộ test tích hợp **tự bỏ
qua** — chúng báo `ok` mà **không chạy một câu SQL nào**. Hệ quả: `0002`, `0003`, khối kiểm
PostgreSQL ≥ 13, khối chặn "bảng phân mảnh thiếu mảnh" và `pg_advisory_lock` thật **đều chưa
từng thực thi**; test của `core/migrate` chạy trên driver giả.

**Nay có một ca cụ thể phải kiểm, không chỉ là nguyên tắc.** Đã chứng minh được: xoá dòng
`AND nd.co_tai_khoan` khỏi `identity/internal/store/checker.go` mà **toàn bộ test vẫn
xanh** — vì bài canh nó nằm trong bộ tích hợp và bộ đó SKIP khi thiếu DSN. Tức hiện tại lớp
lọc quyền của người chỉ có trong danh bạ **không có ai canh trên máy này**.

```
VIGOV_TEST_DSN=... go test -count=1 ./identity/internal/store/
```

| # | Phải tự mắt nhìn thấy |
|---|---|
| 1 | `TestNguoiChiCoTrongDanhBaKhongCoQuyenNao` (`checker_pg_test.go`) **xanh** khi mã nguyên vẹn, và **ĐỎ** khi gỡ dòng `AND nd.co_tai_khoan`. Không đỏ ⇒ bài test không canh gì cả |
| 2 | `current_schema()` dưới `SET search_path` của bộ test có trả đúng schema tạm không. Nếu trả `public` thì khối chặn kiểm nhầm schema và **luôn qua** — đúng dạng guard rỗng mà ADR 0013 nói tới |
| 3 | 17 tệp áp hết, chạy lại lần hai không áp gì thêm |
| 4 | UPDATE/DELETE trên `audit_log` **và** trên một tên partition gõ thẳng đều bị từ chối |
| 5 | `0003`: backfill in NOTICE đúng số dòng; chạy lại không đổi thêm dòng nào |

CI đỏ ở đây thì **đọc thông báo trước khi sửa** — rất có thể chúng đang làm đúng việc.

### 2.2 CRUD cán bộ — **ĐANG BỊ CHẶN bởi một cụm câu hỏi chờ khách chốt**

Đây là màn hình kế tiếp theo kế hoạch, nhưng **đừng bắt đầu bằng cách viết mã**. Cả cụm đã
được ghi thành mục trong `kb/00-foundation/open-questions.json` — **#9 tới #16**, tám câu, mỗi
câu có `ask_before` riêng. Đọc `docs/ui-ux/14-cau-hinh.md` và `docs/ui-ux/12-danh-ba-can-bo.md`
rồi mang **cả cụm** đi hỏi một lượt; hỏi lẻ từng câu thì khách trả lời lệch nhau.

Ba câu đắt nhất:

| Câu | Vì sao không tự quyết được |
|---|---|
| **#9 — mật khẩu đầu tiên đến tay cán bộ bằng cách nào** | Đang chặn một ràng buộc CSDL (`CHECK` trên `co_tai_khoan`), và luồng "gửi link kích hoạt" có một vòng lặp chết với máy chủ thư theo xã |
| **#11 — số di động cán bộ: che hay không** | Trong 33 khoá quyền đã nạp **không có khoá nào** nghĩa là "xem đầy đủ thông tin cán bộ". Không có khoá thì không tồn tại đường nào là "tường minh" theo luật 3 bất biến 3 — chỉ còn che tất hoặc mở tất. Đi kèm **#16**: `dien_thoai` và `di_dong` là một trường hay hai quyết định luôn không gian trả lời |
| **#13 — chặn khoá / gỡ quyền người quản trị CUỐI CÙNG của xã** | Không chặn mà xảy ra thì **không ai mở lại được**: ADR 0003 cấm nhà cung cấp đụng dữ liệu nghiệp vụ của xã. Bế tắc thủ tục, không phải sự cố kỹ thuật |

Khi đã có câu trả lời, các ràng buộc kỹ thuật vẫn nguyên:

- Quyền `admin.user`. Đổi vai trò hoặc khoá tài khoản **phải gọi**
  `PhienStore.ThuHoiCuaCanBo` trong cùng giao dịch (skill `session-and-token` #7).
- Đường dẫn cũ `/cau-hinh/nguoi-dung` **không dùng nữa** (ADR 0011). Danh từ tài nguyên
  tiếng Anh cho khái niệm "cán bộ" **vẫn chưa có trong bảng ánh xạ** của
  `kb/00-foundation/ubiquitous-language.md` — **hỏi trước khi đặt**, đừng tự dịch.
- Danh sách cán bộ là route danh sách đầu tiên ⇒ dùng `core/page`, **không** tự chế cursor.
- Đây cũng là bên tiêu thụ đầu tiên của `BatchGetStaff`: ghép theo `Staff.id`, **không** theo
  thứ tự và **không** giả định trả về đủ số id đã hỏi (ADR 0012, quyết định 2). Lưu ý
  `Staff` hiện **không có trường họ tên** — xem §7 #10.

### 2.3 Còn lại của `identity`

`bo_phan` (cây), `vai_tro`, ma trận phân quyền, `thon_to_dan_pho`, `cong_dan` + OTP (ADR 0002).
**Server gRPC của `identity` vẫn chưa tồn tại** — khi dựng thì dùng lại interceptor trong
`core/grpcx`, không tự viết lại.

### 2.4 `platform`: `danh_muc` + `loi_he_thong`

Mọi service đọc qua gRPC, cache cục bộ. Xem `docs/ui-ux/14-cau-hinh.md` §5 và §7.

### 2.5 `petitions`

**Chỉ bắt đầu sau khi có `lich_lam_viec` + `ngay_nghi_le`** — ADR 0007 nêu 5 lỗ hổng
đặc tả chưa lấp (giờ hành chính mấy giờ, nghỉ trưa có trừ không…).

Tài nguyên là `/api/v1/citizen-reports`, **không phải** `feedback` — nhưng khoá quyền vẫn là
`feedback.*`. Lệch đó là cố ý và đã giải thích một lần ở
`kb/00-foundation/ubiquitous-language.md`; đừng "thống nhất lại" cho gọn.

---

## 3. Tài liệu phải đọc, theo thứ tự

1. `kb/INDEX.yaml` → tầng `always_load` (3 tệp T0)
2. **ADR 0007–0013** — bảy quyết định mới nhất, chưa vào bộ nhớ ai
3. `kb/00-foundation/open-questions.json` — nay **10 câu OPEN**: #1 sáp nhập xã · #4 báo cáo
   cấp huyện · **#9–#16 là cả cụm chặn CRUD cán bộ** (§2.2)
4. `docs/ui-ux/00-tong-quan-he-thong.md` + tệp module đang làm
5. Skill: `rest-api-design` (trước route đầu tiên) · `session-and-token` · `go-tenant-context` ·
   `load-data-once` · `audit-trail`

**Lưu ý về `docs/ui-ux/`:** đó là đặc tả giao diện chép từ prototype **một xã**, đề xuất
stack Next+Prisma. Lấy nghiệp vụ và chuỗi tiếng Việt; **bỏ** §7 phần backend. Đường dẫn API
trong đó là tiếng Việt và **đã lỗi thời** theo ADR 0011. Tên cột trong đó cũng **không phải
cam kết**: `0003` đặt `co_tai_khoan` thay cho `tai_khoan_hoat_dong` của đặc tả, có lý do viết
ngay trong tệp migration.

---

## 4. Cạm bẫy đã gặp — đọc để khỏi mất thời gian lại

Một nửa bảng này có chung một hình dạng: **thứ trông như biện pháp mà không phải biện pháp.**
Gặp cái tiếp theo cùng dạng thì đừng vá riêng nó — hỏi cả lớp đó còn ở đâu nữa.

| Vấn đề | Cách xử |
|---|---|
| **Test tích hợp SKIP nhưng cả gói vẫn báo `ok`** | Dạng nặng nhất của cái bảng này nói tới: đột biến một dòng vào **mã sản phẩm** mà không có gì đỏ. Đã kiểm chứng với `AND nd.co_tai_khoan`. Trước khi tin "có test canh chỗ này", **gỡ thử dòng đó ra và xem có đỏ không** |
| **Không có `make`** trên máy Windows này | Chạy tay: `python tools/check_brain.py` · `python tools/test_hooks.py` · `go vet/build/test ./...` |
| `gofmt -l .` **in tên tệp chưa định dạng rồi thoát mã 0** | Một cổng báo rồi cho qua không phải cổng. Đã vá trong `Makefile` (cả `buf lint` cũng từng bị nuốt lỗi vì tiền tố `-`). **Dạng lỗi này còn ở đâu nữa — hỏi trước khi tin một cổng** |
| `SET search_path` là trạng thái **SESSION** | Trên pool nó chỉ áp cho **kết nối đã phục vụ câu lệnh đó**. `core/migrate` ghim kết nối riêng (advisory lock là session-scoped) nên là kết nối **thứ hai**, nằm ở `public`. Hai suite tích hợp phải `SetMaxOpenConns(1)` |
| Trigger gắn trên bảng cha mà **không bắn** khi gõ thẳng tên partition | Mức **câu lệnh** không được nhân bản xuống partition, mức **dòng** thì có. Ca hỏng **im lặng** — trigger vẫn hiện trong `\d`. → ADR 0013 |
| **Index con của bảng phân mảnh không drop riêng lẻ được** | PostgreSQL từ chối: *"cannot drop index … because index … requires it"*. Drop index **cha** thì kéo theo cả 32 con, và `CREATE` dựng lại đủ 33 quan hệ. Không bao giờ viết thao tác index theo từng mảnh. Đổi vị từ của partial index thì bắt buộc drop + create — nằm trong **một** giao dịch của `core/migrate` nên không có khe hở không index |
| **Hai cột `bool` cạnh nhau, đọc theo vị trí trong `Scan`** | `co_tai_khoan` và `dang_hoat_dong` đứng liền nhau. Hoán đổi hai con trỏ là **lỗi im lặng đối xứng**: biên dịch được, test thường vẫn xanh, chỉ sai nghĩa. Ca duy nhất bắt được là `(false, true)` — phải có test đúng ca đó, và có test canh **thứ tự** của danh sách cột |
| **Lọc ở Go thay vì lọc trong SQL** | Khi hai nhánh tốn thời gian khác hẳn nhau (ví dụ: có đọc hàm băm mật khẩu hay không), khoảng chênh đo được từ bên ngoài ⇒ **kênh biên thời gian** cho biết một email có tồn tại hay không. Điều kiện phân biệt người dùng phải nằm trong `WHERE`, để hai nhánh tốn như nhau |
| `fmt` **không gọi `String()`** cho `%d %c %U %b %o` | Một `[]byte` khoá ký in ra nguyên vẹn dưới dạng byte. Phải cài `fmt.Formatter`, không phải `Stringer`. `%p` và `%T` thì `fmt` **không bao giờ** gọi Formatter — đã xác minh chúng chỉ lộ địa chỉ và tên kiểu. → `core/secret` |
| `doc_guard` chặn mọi `Edit` vào `kb/` | Hook chỉ đọc `new_string`, không thấy frontmatter. **Dùng `Write` toàn tệp** |
| `data_safety_guard` báo sai 3 lần | Đã vá cả ba (`delete()` của Go, UPDATE nhiều dòng, thân heredoc trong thông điệp commit). Nếu gặp lần bốn: sửa hook + thêm ca test, **đừng đi vòng** — hook nhiễu là hook bị tắt |
| `psql` và DSN trong dòng lệnh bị chặn | Đặt biến môi trường ở lệnh riêng, dùng tên biến |
| `PARTITION BY HASH` mà quên tạo mảnh | Mọi INSERT lỗi *"no partition of relation found"*, và vì vết đi cùng giao dịch nên **bản ghi nghiệp vụ rollback toàn bộ**. Nay `tenant_scope_guard` chặn ngay lúc gõ `.sql`. → ADR 0013 |
| Test migration mỗi test tốn 17 giây | Dùng `TestMain` chạy migration **một lần** cho cả gói; mỗi test tự cách ly bằng id xã riêng |
| `buf lint STANDARD` ép tên message theo tên RPC | Đổi tên RPC là đổi luôn tên `<Rpc>Request` / `<Rpc>Response`. Kiểu trả về phải **bọc**, không trả thẳng message nghiệp vụ |
| Gộp "miễn kiểm metadata" với "id nằm trong thân message" | Hai chuyện khác nhau, và gộp lại thì đọc ra thành `GetTenant` được miễn. Xem ADR 0012, mục Ngoại lệ |

---

## 5. Chạy test tích hợp

```
export VIGOV_TEST_DSN="postgres://postgres:<mật-khẩu>@192.168.3.135:5433/postgres?sslmode=disable"
go test -count=1 ./...
```

Không đặt biến → test tích hợp **tự bỏ qua và vẫn báo ok** (§4, dòng đầu bảng). Đó là lý do
§2.1 đứng trước mọi việc khác: xanh ở đây **không** có nghĩa là SQL đã chạy.

**Mật khẩu KHÔNG nằm trong kho mã** (luật 8). Hỏi chủ dự án. Máy chủ trên là **máy test**;
mỗi lượt chạy tự tạo schema riêng theo mốc thời gian rồi tự dọn.

---

## 6. Cổng kiểm chứng

**Không có bảng số liệu ở đây, cố ý.** Mọi con số chép vào tệp này đều sai trong vòng vài
commit, và một con số sai trông y hệt một con số đúng. Chạy lại để lấy con số thật:

```
python tools/check_brain.py
python tools/test_hooks.py
go vet ./... && go build ./... && go test -race -count=1 ./...
```

`make check` nay **chặn thật** ở `gofmt` và `buf lint` (trước đây cả hai cho qua), có `-race`,
và có target `web` **bỏ qua một cách ồn ào** cho tới khi `web-admin/` có
`node_modules`. Nhưng xanh ở đây vẫn **không** bao gồm SQL — xem §2.1.

---

## 7. Việc treo

| # | Việc | Ghi chú |
|---|---|---|
| 1 | **17 tệp migration chưa từng chạy thật** | Việc số một. Xem §2.1 |
| 2 | **Bài test canh `co_tai_khoan` chưa từng chạy** | Cùng nguyên nhân với #1, nhưng hệ quả khác: đây là một bất biến **phân quyền** hiện không có gì canh trên máy này |
| 3 | **`make check` chưa kiểm mã TypeScript** | Target `web` đã có nhưng in dòng BỎ QUA vì thiếu `node_modules`. Cài xong là nó kiểm thật, không phải sửa Makefile nữa |
| 4 | **Backfill dữ liệu theo từng xã chưa tồn tại** | `core/migrate` chỉ lo DDL ⇒ **luật 7 bất biến 5 mới đạt một nửa**. Backfill trong `0003` là một câu UPDATE, cố ý, vì bảng chỉ vài chục dòng mỗi xã — ngưỡng cần cơ chế thật là khi thời gian giữ khoá thành đáng kể. → ADR 0013, mục Giới hạn |
| 5 | **`REVOKE` trên `audit_log` thuộc khâu cấp phát CSDL** | Câu đúng, đủ cả dòng cho 32 mảnh, giữ trong comment tệp `0002`. **Không nằm trong tay mã nguồn**: khâu cấp phát không làm thì lớp quyền vẫn hở dù trigger vẫn đúng |
| 6 | **Đặc tả ghi 43 quyền nhưng chỉ liệt kê 33** | Đã nạp 33 vào `quyen`. Mười khoá còn lại là câu hỏi cho khách, **đừng bịa**. Ít nhất một khoá thiếu đã có tên: "xem đầy đủ thông tin cán bộ" — **câu hỏi mở #11** |
| 7 | **Bảng ánh xạ tên tài nguyên URL mới phủ một phần** | `kb/00-foundation/ubiquitous-language.md` — "cán bộ" đã có dòng nhưng cột tài nguyên ghi **CHƯA CHỐT**, và đó là khái niệm của màn hình kế tiếp. Hỏi, đừng tự dịch |
| 8 | **Dữ liệu cá nhân thật vẫn còn trong LỊCH SỬ GIT** | Cây làm việc đã dọn (họ tên + số di động thật của 7 lãnh đạo xã trong `docs/ui-ux/`). Gỡ khỏi lịch sử là **viết lại lịch sử** trên `main` — cần quyết định của chủ dự án, không phải việc agent tự làm |
| 9 | **Xác thực service↔service chưa có** | Rủi ro đã chấp nhận có chủ ý, kèm điều kiện gỡ: ADR 0012, quyết định 3. **Không** dựng cơ chế bí mật chia sẻ tạm |
| 10 | **`Staff` không có trường họ tên** | Nên `BatchGetStaff` **chưa phục vụ được mục đích nó tự khai**: "màn hình danh sách trang trí từng dòng" cần hiển thị tên người. Thêm trường là sửa hợp đồng — cùng lúc phải trả lời câu che/không che ở §2.2 (#11), vì họ tên cán bộ cũng là dữ liệu cá nhân |
| 11 | **Bàn giao việc đang xử lý khi khoá tài khoản — chưa hỏi, cố ý chưa ghi thành câu hỏi mở** | Grep toàn `docs/` cho "bàn giao" / "chuyển công tác" / "nghỉ việc": **không kết quả**. Tiền lệ gần nhất là `14-cau-hinh.md:390` — xoá **bộ phận** còn cán bộ hoặc hồ sơ thì chặn, yêu cầu chuyển trước. Chưa ghi vào `open-questions.json` vì **chưa có bảng giao việc nào tồn tại** để nói "việc đang giữ" nghĩa là gì; hỏi khách bây giờ là hỏi một câu trừu tượng. **Hỏi khi dựng bảng nghiệp vụ đầu tiên có người phụ trách** (`nhiem_vu`, `van_ban_den`) — và nhớ rằng câu trả lời nhiều khả năng là "tuỳ xã": xã 8 người để trưởng bộ phận gánh, xã 40 người có thủ tục bàn giao có biên bản |
