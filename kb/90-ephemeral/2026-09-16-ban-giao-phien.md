---
id: 2026-09-16-ban-giao-phien
tier: T5
source: CURATED
owner: architecture
derived_from_commit: 184869f
expires: 2026-12-16
owns_facts:
  - "trạng thái thi công tại 2026-09-16 và việc kế tiếp phải làm"
---

# Bàn giao phiên — 2026-09-16

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

### Trạng thái mã

**Chạy được, có test:**

| Gói | Nội dung |
|---|---|
| `pkg/tenant` `pkg/store` `pkg/audit` `pkg/events` `pkg/privacy` `pkg/httpx` | Có sẵn từ khung sườn |
| `pkg/config` | Hằng số nền tảng; cố ý không có `Get("...")`. Đã có tệp mẫu env |
| `pkg/password` | argon2id, `CanBamLai` để nâng chi phí |
| `pkg/authz` | `Perm` phẳng; test gồm đủ 4 ca luật 5 |
| `pkg/idem` | Chống trùng request, **khai opt-in từng route** |
| `platform/internal/{domain,store}` | Phân giải Host, cache, edge chain |
| `identity/internal/{domain,store,app}` | RBAC, `Checker`, phiên, đăng nhập |
| `identity/internal/http` | **Route đăng nhập/đăng xuất + middleware dựng `Principal`**, token ký HMAC |
| `proto/` + `gen/` | Hợp đồng đã sinh mã. `BatchGetStaff` thay `GetStaff`; `x-tenant-id` khai ở cả 8 tệp proto |

**Còn trống hoàn toàn:** mọi route HTTP **nghiệp vụ** · `documents` `petitions` `dossiers`
`finance` `comms` `reporting` · toàn bộ frontend (`apps/*/src/` chỉ có `.gitkeep`) ·
`pkg/storage` · `data-ownership.json` vẫn rỗng.

---

## 2. Việc kế tiếp — theo đúng thứ tự

### (a) Tầng gRPC: `pkg/grpcx` + server gRPC cho `platform` ← **BẮT ĐẦU TỪ ĐÂY**

Route HTTP đã chạm được tới `identity`, nhưng `main.go` chưa wire được service nào với service
nào vì **chưa có tầng gRPC**. Đây là mảnh chặn mọi việc phía sau.

**Đọc `kb/10-decisions/0012-grpc-boundary-contract.md` trước khi viết dòng đầu tiên** — bốn
quyết định ở đó là hợp đồng, không phải gợi ý.

| Việc | Ghi chú |
|---|---|
| `pkg/grpcx`: hằng khoá metadata + interceptor **hai đầu** | Client lấy xã từ `context.Context`; server đọc ra và đặt lại vào context |
| Server thiếu `x-tenant-id` → `INVALID_ARGUMENT` | Không đoán xã từ thân message, không có xã mặc định |
| Ngoại lệ `ResolveHost` / `GetTenant` khai theo **danh sách trắng tên RPC** | Không bao giờ miễn trừ theo mặc định — ADR 0012, quyết định 1 |
| Server gRPC cho `platform` | Chỉ siêu dữ liệu (ADR 0003). Không thêm RPC nào trả nội dung nghiệp vụ |
| `ByHost` hỏng vì lý do truyền tải → **log mức báo động** | 404 vẫn là hành vi đúng, nhưng phải phân biệt được "không có xã" với "không hỏi được" |

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
2. **ADR 0007–0012** — sáu quyết định mới nhất, chưa vào bộ nhớ ai
3. `kb/00-foundation/open-questions.json` — còn **2 câu OPEN**: #1 sáp nhập xã, #4 báo cáo cấp huyện
4. `docs/ui-ux/00-tong-quan-he-thong.md` + tệp module đang làm
5. Skill: `rest-api-design` (trước route đầu tiên) · `session-and-token` · `go-tenant-context` ·
   `load-data-once` · `audit-trail`

**Lưu ý về `docs/ui-ux/`:** đó là đặc tả giao diện chép từ prototype **một xã**, đề xuất
stack Next+Prisma. Lấy nghiệp vụ và chuỗi tiếng Việt; **bỏ** §7 phần backend. Đường dẫn API
trong đó là tiếng Việt và **đã lỗi thời** theo ADR 0011.

---

## 4. Cạm bẫy đã gặp — đọc để khỏi mất thời gian lại

| Vấn đề | Cách xử |
|---|---|
| **Không có `make`** trên máy Windows này | Chạy tay: `python tools/check_brain.py` · `python tools/test_hooks.py` · `go vet/build/test ./...` |
| **Race detector không chạy** — thiếu gcc | Test đồng thời có chạy nhưng **chưa được xác nhận**. Chạy `go test -race` ở CI |
| `doc_guard` chặn mọi `Edit` vào `kb/` | Hook chỉ đọc `new_string`, không thấy frontmatter. **Dùng `Write` toàn tệp** |
| `data_safety_guard` báo sai 2 lần | Đã vá cả hai (`delete()` của Go, UPDATE nhiều dòng). Nếu gặp lần ba: sửa hook + thêm ca test, **đừng đi vòng** |
| `psql` và DSN trong dòng lệnh bị chặn | Đặt biến môi trường ở lệnh riêng, dùng tên biến |
| `PARTITION BY HASH` mà quên tạo partition con | Mọi lệnh chèn lỗi *"no partition of relation found"*. Xem khối `DO $$` trong migration |
| Test migration mỗi test tốn 17 giây | Dùng `TestMain` chạy migration **một lần** cho cả gói; mỗi test tự cách ly bằng id xã riêng |
| `buf lint STANDARD` ép tên message theo tên RPC | Đổi tên RPC là đổi luôn tên `<Rpc>Request` / `<Rpc>Response`. Kiểu trả về phải **bọc**, không trả thẳng message nghiệp vụ |

---

## 5. Chạy test tích hợp

```
export VIGOV_TEST_DSN="postgres://postgres:<mật-khẩu>@192.168.3.135:5433/postgres?sslmode=disable"
go test ./...
```

Không đặt biến → test tích hợp tự bỏ qua, `go test ./...` vẫn xanh.

**Mật khẩu KHÔNG nằm trong kho mã** (luật 8). Hỏi chủ dự án. Máy chủ trên là **máy test**;
mỗi lượt chạy tự tạo schema riêng theo mốc thời gian rồi tự dọn.

---

## 6. Cổng kiểm chứng

**Không có bảng số liệu ở đây, cố ý.** Mọi con số chép vào tệp này đều sai trong vòng vài
commit, và một con số sai trông y hệt một con số đúng. Chạy lại để lấy con số thật:

```
python tools/check_brain.py
python tools/test_hooks.py
go vet ./... && go build ./... && go test ./...
```

---

## 7. Bốn việc treo

| # | Việc | Ghi chú |
|---|---|---|
| 1 | **`make check` không kiểm frontend** | Mã TS hỏng vẫn qua cổng. Cần thêm target gọi `npm run build` |
| 2 | **Đặc tả ghi 43 quyền nhưng chỉ liệt kê 33** | Đã nạp 33 vào `quyen`. Mười khoá còn lại là câu hỏi cho khách, **đừng bịa** |
| 3 | **Bảng ánh xạ tên tài nguyên URL mới phủ một phần** | `kb/00-foundation/ubiquitous-language.md` — gặp khái niệm chưa có dòng thì hỏi, đừng tự dịch |
| 4 | **Xác thực service↔service chưa có** | Rủi ro đã chấp nhận có chủ ý, kèm điều kiện gỡ: ADR 0012, quyết định 3. **Không** dựng cơ chế bí mật chia sẻ tạm |
