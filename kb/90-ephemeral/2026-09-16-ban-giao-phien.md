---
id: 2026-09-16-ban-giao-phien
tier: T5
source: CURATED
owner: architecture
derived_from_commit: f9f35d4
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

**Danh sách commit KHÔNG chép vào đây** — `git log 8b813ff..HEAD` trả lời câu đó chính xác
hơn và không bao giờ lệch. Dưới đây chỉ những thứ `git log` không trả lời được.

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
| `pkg/tenant` `pkg/store` `pkg/audit` `pkg/events` `pkg/privacy` `pkg/httpx` | Có sẵn từ khung sườn; `pkg/tenant` nay giữ thêm `CachedDirectory` (chuyển lên từ `platform`, vì bảy service kia không import được `internal/` của nó — luật 2 cấm #1) |
| `pkg/config` | Hằng số nền tảng; cố ý không có `Get("...")`. Đã có tệp mẫu env |
| `pkg/password` | argon2id, `CanBamLai` để nâng chi phí |
| `pkg/authz` | `Perm` phẳng; test gồm đủ 4 ca luật 5 |
| `pkg/idem` | Chống trùng request, **khai opt-in từng route** |
| `pkg/grpcx` | Interceptor **hai đầu** + hằng khoá metadata. Danh sách miễn kiểm xã có **đúng một** thành viên: `ResolveHost` |
| `pkg/secret` | Kiểu **tự từ chối in**: chặn **mọi** verb `fmt` qua `Formatter`, không phải `String()`. Lối ra thô tên `Lo()`, đúng ba chỗ gọi trong toàn repo |
| `pkg/platformclient` | Client phân giải Host tới `platform`. Ở đây chứ không ở `pkg/tenant` vì `pkg/grpcx` đã import `pkg/tenant` → **vòng import** |
| `pkg/migrate` | Trình chạy migration: `go:embed`, advisory lock, tiền-kiểm checksum, mỗi tệp một giao dịch. **Cả 8 service đã nối vào `main.go`** — ADR 0013 |
| `platform/internal/{domain,store}` | Phân giải Host, cache, edge chain |
| `platform/internal/grpc` | Server gRPC — siêu dữ liệu, không có đường trả nội dung nghiệp vụ (ADR 0003) |
| `identity/internal/{domain,store,app}` | RBAC, `Checker`, phiên, đăng nhập |
| `identity/internal/http` | **Route đăng nhập/đăng xuất + middleware dựng `Principal`**, token ký HMAC |
| `identity/cmd/server` | **Wire chạy thật**: config → CSDL → migrate → Directory qua gRPC (có cache) → checker/canBo/phien → **một** signer → use case → idem → chuỗi edge → tắt êm |
| `proto/` + `gen/` | Hợp đồng đã sinh mã. `BatchGetStaff` thay `GetStaff`; `x-tenant-id` khai ở cả 8 tệp proto |
| `services/*/migrations/` | 16 tệp, 8 service. `0002` biến "append-only" từ comment thành ràng buộc CSDL. **Xem §2(a): chưa tệp nào chạy thật** |

**Còn trống hoàn toàn:** mọi route HTTP **nghiệp vụ** · **server gRPC của `identity`** (chưa
có `internal/grpc`) · `documents` `petitions` `dossiers` `finance` `comms` `reporting` mới có
`main.go` + migration, chưa có nghiệp vụ · toàn bộ frontend (`apps/*/src/` chỉ có `.gitkeep`)
· `pkg/storage` · `data-ownership.json` vẫn rỗng.

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

### (a) Chạy migration thật, một lần, trên máy có CSDL ← **BẮT ĐẦU TỪ ĐÂY**

Trước khi viết thêm nghiệp vụ nào. Máy phát triển không có `VIGOV_TEST_DSN` nên hai bộ test
tích hợp **tự bỏ qua** — chúng báo ok mà **không chạy một câu SQL nào**. Hệ quả: `0002`, khối
kiểm PostgreSQL ≥ 13, khối chặn "bảng phân mảnh thiếu mảnh" và `pg_advisory_lock` thật **đều
chưa từng thực thi**; test của `pkg/migrate` chạy trên driver giả.

Ba thứ phải tự mắt nhìn thấy xanh:

| # | Kiểm |
|---|---|
| 1 | `current_schema()` dưới `SET search_path` của bộ test có trả đúng schema tạm không. Nếu trả `public` thì khối chặn kiểm nhầm schema và **luôn qua** — đúng dạng guard rỗng mà ADR 0013 nói tới |
| 2 | 16 tệp áp hết, chạy lại lần hai không áp gì thêm |
| 3 | UPDATE/DELETE trên `audit_log` **và** trên một tên partition gõ thẳng đều bị từ chối |

CI đỏ ở đây thì **đọc thông báo trước khi sửa** — rất có thể chúng đang làm đúng việc.

### (b) CRUD cán bộ đầu tiên

Quyền `admin.user`. Đổi vai trò hoặc khoá tài khoản **phải gọi**
`PhienStore.ThuHoiCuaCanBo` trong cùng giao dịch (skill `session-and-token` #7).

Đường dẫn cũ `/cau-hinh/nguoi-dung` **không dùng nữa** (ADR 0011). Danh từ tài nguyên tiếng
Anh cho khái niệm "cán bộ" **chưa có trong bảng ánh xạ** của
`kb/00-foundation/ubiquitous-language.md` — **hỏi trước khi đặt**, đừng tự dịch.

Đây cũng là bên tiêu thụ đầu tiên của `BatchGetStaff`: ghép theo `Staff.id`, **không** theo
thứ tự và **không** giả định trả về đủ số id đã hỏi (ADR 0012, quyết định 2).

### (c) Còn lại của `identity`

`bo_phan` (cây), `vai_tro`, ma trận phân quyền, `thon_to_dan_pho`, `cong_dan` + OTP (ADR 0002).
**Server gRPC của `identity` vẫn chưa tồn tại** — khi dựng thì dùng lại interceptor trong
`pkg/grpcx`, không tự viết lại.

### (d) `platform`: `danh_muc` + `loi_he_thong`

Mọi service đọc qua gRPC, cache cục bộ. Xem `docs/ui-ux/14-cau-hinh.md` §5 và §7.

### (e) `petitions`

**Chỉ bắt đầu sau khi có `lich_lam_viec` + `ngay_nghi_le`** — ADR 0007 nêu 5 lỗ hổng
đặc tả chưa lấp (giờ hành chính mấy giờ, nghỉ trưa có trừ không…).

Tài nguyên là `/api/v1/citizen-reports`, **không phải** `feedback` — nhưng khoá quyền vẫn là
`feedback.*`. Lệch đó là cố ý và đã giải thích một lần ở
`kb/00-foundation/ubiquitous-language.md`; đừng "thống nhất lại" cho gọn.

---

## 3. Tài liệu phải đọc, theo thứ tự

1. `kb/INDEX.yaml` → tầng `always_load` (3 tệp T0)
2. **ADR 0007–0013** — bảy quyết định mới nhất, chưa vào bộ nhớ ai
3. `kb/00-foundation/open-questions.json` — còn **2 câu OPEN**: #1 sáp nhập xã, #4 báo cáo cấp huyện
4. `docs/ui-ux/00-tong-quan-he-thong.md` + tệp module đang làm
5. Skill: `rest-api-design` (trước route đầu tiên) · `session-and-token` · `go-tenant-context` ·
   `load-data-once` · `audit-trail`

**Lưu ý về `docs/ui-ux/`:** đó là đặc tả giao diện chép từ prototype **một xã**, đề xuất
stack Next+Prisma. Lấy nghiệp vụ và chuỗi tiếng Việt; **bỏ** §7 phần backend. Đường dẫn API
trong đó là tiếng Việt và **đã lỗi thời** theo ADR 0011.

---

## 4. Cạm bẫy đã gặp — đọc để khỏi mất thời gian lại

Một nửa bảng này có chung một hình dạng: **thứ trông như biện pháp mà không phải biện pháp.**
Gặp cái thứ tám cùng dạng thì đừng vá riêng nó — hỏi cả lớp đó còn ở đâu nữa.

| Vấn đề | Cách xử |
|---|---|
| **Không có `make`** trên máy Windows này | Chạy tay: `python tools/check_brain.py` · `python tools/test_hooks.py` · `go vet/build/test ./...` |
| `gofmt -l .` **in tên tệp chưa định dạng rồi thoát mã 0** | Một cổng báo rồi cho qua không phải cổng. Đã vá trong `Makefile`: thu kết quả vào biến, `exit 1` nếu khác rỗng. **Dạng lỗi này còn ở đâu nữa — hỏi trước khi tin một cổng** |
| `SET search_path` là trạng thái **SESSION** | Trên pool nó chỉ áp cho **kết nối đã phục vụ câu lệnh đó**. Trước đây may vì mọi thứ chạy trên một kết nối mở lười; `pkg/migrate` ghim kết nối riêng (advisory lock là session-scoped) nên là kết nối **thứ hai**, nằm ở `public`. Hai suite tích hợp phải `SetMaxOpenConns(1)` |
| Trigger gắn trên bảng cha mà **không bắn** khi gõ thẳng tên partition | Mức **câu lệnh** không được nhân bản xuống partition, mức **dòng** thì có. Ca hỏng **im lặng** — trigger vẫn hiện trong `\d`. → ADR 0013 |
| `fmt` **không gọi `String()`** cho `%d %c %U %b %o` | Một `[]byte` khoá ký in ra nguyên vẹn dưới dạng byte. Phải cài `fmt.Formatter`, không phải `Stringer`. `%p` và `%T` thì `fmt` **không bao giờ** gọi Formatter — đã xác minh chúng chỉ lộ địa chỉ và tên kiểu. → `pkg/secret` |
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
go test ./...
```

Không đặt biến → test tích hợp **tự bỏ qua và vẫn báo ok**. Đó là lý do §2(a) tồn tại: xanh ở
đây **không** có nghĩa là SQL đã chạy.

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
và có target `web` **bỏ qua một cách ồn ào** cho tới khi `apps/commune-admin/` có
`node_modules`.

---

## 7. Việc treo

| # | Việc | Ghi chú |
|---|---|---|
| 1 | **16 tệp migration chưa từng chạy thật** | Việc số một. Xem §2(a) |
| 2 | **`make check` chưa kiểm mã TypeScript** | Target `web` đã có nhưng in dòng BỎ QUA vì thiếu `node_modules`. Cài xong là nó kiểm thật, không phải sửa Makefile nữa |
| 3 | **Backfill dữ liệu theo từng xã chưa tồn tại** | `pkg/migrate` chỉ lo DDL ⇒ **luật 7 bất biến 5 mới đạt một nửa**. Đừng đọc sự có mặt của gói thành "đã xong". → ADR 0013, mục Giới hạn |
| 4 | **`REVOKE` trên `audit_log` thuộc khâu cấp phát CSDL** | Câu đúng, đủ cả dòng cho 32 mảnh, giữ trong comment tệp `0002`. Khâu cấp phát không làm thì lớp quyền vẫn hở dù trigger vẫn đúng |
| 5 | **Đặc tả ghi 43 quyền nhưng chỉ liệt kê 33** | Đã nạp 33 vào `quyen`. Mười khoá còn lại là câu hỏi cho khách, **đừng bịa** |
| 6 | **Bảng ánh xạ tên tài nguyên URL mới phủ một phần** | `kb/00-foundation/ubiquitous-language.md` — gặp khái niệm chưa có dòng thì hỏi, đừng tự dịch |
| 7 | **Xác thực service↔service chưa có** | Rủi ro đã chấp nhận có chủ ý, kèm điều kiện gỡ: ADR 0012, quyết định 3. **Không** dựng cơ chế bí mật chia sẻ tạm |
