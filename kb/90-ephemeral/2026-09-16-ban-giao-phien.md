---
id: 2026-09-16-ban-giao-phien
tier: T5
source: CURATED
owner: architecture
derived_from_commit: 5887496
expires: 2026-12-16
owns_facts:
  - "trạng thái thi công tại 2026-09-16 và việc kế tiếp phải làm"
---

# Bàn giao phiên — 2026-09-16

**Đọc tệp này SAU `kb/INDEX.yaml` và tầng `always_load`, không thay thế chúng.**
Nó chỉ trả lời: *đã làm tới đâu, việc kế tiếp là gì, và cạm bẫy nào đã gặp.*

Hết hạn **2026-12-16**. Sau ngày đó tin `git log` chứ đừng tin tệp này.

---

## 1. Đã làm — 8 commit, `8b813ff..039aa6d`

| Commit | Nội dung |
|---|---|
| `a406cf9` | `.gitattributes` ghim LF; mở ngoại lệ `docs/ui-ux/` cho `check_brain` |
| `1b542e7` | **4 ADR** (0007–0010) + vá mâu thuẫn tài liệu + 16 tệp đặc tả giao diện |
| `03e9309` | Skill `load-data-once` + một dòng vào 8 README service |
| `cd43dc9` | `platform`: schema `tenant` + `tenant_domain`, tầng domain |
| `4d78d05` | `platform`: `Directory` phân giải Host + cache TTL |
| `9f6c771` | 10 test tích hợp trên PostgreSQL thật |
| `e48bb85` | `pkg/config` + wiring `main.go` + 6 test edge chain |
| `de34508` | **`pkg/authz`: quyền thành khoá phẳng**, bỏ `(subsystem, action)` |
| `256d113` | `identity`: schema RBAC + `Checker` thật + 11 test |
| `039aa6d` | `identity`: `pkg/password` argon2id + bảng `phien` + use case đăng nhập + 8 test |

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

### Trạng thái mã

**Chạy được, có test:**

| Gói | Nội dung |
|---|---|
| `pkg/tenant` `pkg/store` `pkg/audit` `pkg/events` `pkg/privacy` `pkg/httpx` | Có sẵn từ khung sườn |
| `pkg/config` | **Mới** — hằng số nền tảng; cố ý không có `Get("...")` |
| `pkg/password` | **Mới** — argon2id, `CanBamLai` để nâng chi phí |
| `pkg/authz` | **Đã sửa** — `Perm` phẳng; 8 test gồm đủ 4 ca luật 5 |
| `platform/internal/{domain,store}` | Phân giải Host, cache, edge chain |
| `identity/internal/{domain,store,app}` | RBAC, `Checker`, phiên, đăng nhập |

**Còn trống hoàn toàn:** mọi route HTTP nghiệp vụ · `documents` `petitions` `dossiers`
`finance` `comms` `reporting` · toàn bộ frontend (`apps/*/src/` chỉ có `.gitkeep`) ·
`pkg/storage` · `gen/` từ `.proto` · `data-ownership.json` vẫn rỗng.

---

## 2. Việc kế tiếp — theo đúng thứ tự

**Trước khi viết route đầu tiên: đọc `.claude/skills/rest-api-design/SKILL.md` và ADR 0011.**
Route đầu tiên đặt khuôn cho toàn bộ phần còn lại, và đường dẫn là thứ không sửa lại được sau
khi một xã chạy thật.

### (b) Route đăng nhập + middleware dựng `Principal` ← **BẮT ĐẦU TỪ ĐÂY**

Đây là mảnh cuối để lát cắt dọc chạm được từ HTTP.

| Việc | Ghi chú |
|---|---|
| `POST /api/v1/sessions` trong `identity/internal/http/routes.go` | Khai `authz.Public("màn hình đăng nhập, chưa có phiên")` |
| Middleware đọc cookie → `phien.KiemTra` → dựng `authz.Principal` | Chạy MỌI request (skill `session-and-token` #3) |
| Cookie đặt đúng host từng xã | **Cấm** `domain=.vigov.vn` (luật 1, cấm #3) |
| So `tenant_id` trong phiên với xã từ `Host`; lệch = 401 **+ cảnh báo** | Skill #2 |
| `DELETE /api/v1/sessions/{sid}` | Use case `DangXuat` đã có |
| Mỗi route khai lớp chống trùng | `idem.Required(...)` hoặc `idem.KhongCan("<lý do>")` — skill `rest-api-design` §4 |
| 4 test luật 5 cho mỗi route | 401 · 403 · 403 sai xã · 200 |

**Đổi so với bản bàn giao đầu:** hai đường dẫn trên trước ghi là `POST /dang-nhap` và
`POST /dang-xuat`. ADR 0011 bác tiếng Việt trên path; `.claude/hooks/rest_api_guard.py` nay
chặn thẳng, nên viết theo bản cũ sẽ bị hook từ chối.

Khung đã sẵn: `apps/commune-admin/src/lib/session.ts` có `SESSION_COOKIE` và
`cookieOptions(host)`.

### (c) CRUD cán bộ đầu tiên

Quyền `admin.user`. Đổi vai trò hoặc khoá tài khoản **phải gọi**
`PhienStore.ThuHoiCuaCanBo` trong cùng giao dịch (skill #7).

Đường dẫn cũ `/cau-hinh/nguoi-dung` **không dùng nữa** (ADR 0011). Danh từ tài nguyên tiếng
Anh cho khái niệm "cán bộ" **chưa có trong bảng ánh xạ** của
`kb/00-foundation/ubiquitous-language.md` — **hỏi trước khi đặt**, đừng tự dịch.

### (d) Còn lại của `identity`

`bo_phan` (cây), `vai_tro`, ma trận phân quyền, `thon_to_dan_pho`, `cong_dan` + OTP (ADR 0002).

### (e) `platform`: `danh_muc` + `loi_he_thong`

Mọi service đọc qua gRPC, cache cục bộ. Xem `docs/ui-ux/14-cau-hinh.md` §5 và §7.

### (f) `petitions`

**Chỉ bắt đầu sau khi có `lich_lam_viec` + `ngay_nghi_le`** — ADR 0007 nêu 5 lỗ hổng
đặc tả chưa lấp (giờ hành chính mấy giờ, nghỉ trưa có trừ không…).

Tài nguyên là `/api/v1/citizen-reports`, **không phải** `feedback` — nhưng khoá quyền vẫn là
`feedback.*`. Lệch đó là cố ý và đã giải thích một lần ở
`kb/00-foundation/ubiquitous-language.md`; đừng "thống nhất lại" cho gọn.

---

## 3. Tài liệu phải đọc, theo thứ tự

1. `kb/INDEX.yaml` → tầng `always_load` (3 tệp T0)
2. **ADR 0007–0011** — năm quyết định mới nhất, chưa vào bộ nhớ ai
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

## 6. Cổng kiểm chứng — trạng thái tại `039aa6d`

| Kiểm tra | Kết quả |
|---|---|
| `check_brain.py` | **7/7** |
| `test_hooks.py` | **57/57** |
| `go vet` · `go build` · `gofmt` | Sạch |
| `go test ./...` | 10 gói xanh, gồm **19 test tích hợp** trên PostgreSQL 17.6 |

Sau mốc đó có thêm `.claude/hooks/rest_api_guard.py`, nên số ca của `test_hooks.py` đã tăng —
chạy lại để lấy con số thật, đừng tin bảng này.

---

## 7. Hai việc treo

| # | Việc | Ghi chú |
|---|---|---|
| 1 | **`make check` không kiểm frontend** | Mã TS hỏng vẫn qua cổng. Cần thêm target gọi `npm run build` |
| 2 | **Đặc tả ghi 43 quyền nhưng chỉ liệt kê 33** | Đã nạp 33 vào `quyen`. Mười khoá còn lại là câu hỏi cho khách, **đừng bịa** |
| 3 | **Bảng ánh xạ tên tài nguyên URL mới phủ một phần** | `kb/00-foundation/ubiquitous-language.md` — gặp khái niệm chưa có dòng thì hỏi, đừng tự dịch |
