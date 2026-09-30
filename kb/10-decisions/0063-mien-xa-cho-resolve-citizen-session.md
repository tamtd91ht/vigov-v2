---
id: 0063-mien-xa-cho-resolve-citizen-session
tier: T1
source: CURATED
owner: architecture
derived_from_commit: b3cc9582
expires: null
owns_facts:
  - "vì sao ResolveCitizenSession được miễn x-tenant-id (nằm trong core/grpcx methodsWithoutTenant) — người dùng chốt 21/09/2026"
  - "miễn xã không phải miễn xác thực: RPC được miễn vẫn phải mang khoá gọi nội bộ"
  - "vì sao ListTenants và ResolveTenantSuccession cố ý nằm NGOÀI danh sách miễn"
  - "điều kiện để thêm bất kỳ tên nào vào methodsWithoutTenant: người dùng quyết, có ADR"
---

# 0063. Miễn xã cho `ResolveCitizenSession`

**Trạng thái:** đã chốt · **Ngày ghi:** 2026-09-30 · **Người dùng chốt ngày 21/09/2026** (trả lời
điều kiện dừng của ADR 0012 quyết định 1) · Đã dựng từ `d9ea3499` · **Nới** danh sách "đúng một
thành viên" của ADR 0012 quyết định 1 mục A — thân ADR 0012 giữ nguyên

> Tệp này ghi **sau** khi mã đã đi theo quyết định. Trước đó lý do chỉ nằm trong chú thích
> `core/grpcx/grpcx.go:167-179` và trong ca kiểm — không có chủ trong `kb/`.

## Bối cảnh

ADR 0012 quyết định 1 bắt mọi RPC mang `x-tenant-id`, trừ một danh sách trắng khai theo tên đầy
đủ, lúc viết có đúng một thành viên: `ResolveHost`. Phép thử của nó: *RPC này có biết được xã tại
thời điểm gọi không?* Thêm thành viên là **điều kiện dừng** — người dùng quyết.

Kênh công dân không có tên miền: xã của một yêu cầu công dân đến **từ phiên** (ADR 0022). RPC trả
lời câu ấy là `IdentityService/ResolveCitizenSession`. Bắt nó mang xã là vòng lặp *biết xã để tìm
ra xã* — đúng hình dạng đã miễn cho `ResolveHost`.

## Quyết định

`/vigov.identity.v1.IdentityService/ResolveCitizenSession` nằm trong
`core/grpcx.methodsWithoutTenant` (`core/grpcx/grpcx.go:199-204`, hằng `MethodResolveCitizenSession`
ở `:119`). Interceptor phía nhận không đòi `x-tenant-id` cho nó.

## Vì sao miễn không mở rộng lỗ

| Điều | Vì sao an toàn |
|---|---|
| Yêu cầu chỉ mang `session_token` | Không có trường xã nào — bên gọi không hỏi được *"phiên này có hợp lệ ở xã X không"*, nên miễn không biến được lời khai thành câu trả lời (luật 1 cấm #2) |
| Xã nằm ở **phản hồi** | Suy từ sổ phiên, không từ bên gọi |
| Phản hồi không mang khoá quyền, không mang dữ liệu cá nhân | Nó phân giải một phiên, không cấp gì (luật 4) |

## Miễn KHÔNG cấp gì

| Không cấp | Chỗ giữ |
|---|---|
| **Miễn xác thực.** RPC được miễn xã vẫn phải mang khoá gọi nội bộ (ADR 0025) | `core/grpcx/caller_auth_exempt_test.go:55-79` — duyệt chính `methodsWithoutTenant`, không bản chép, nên tên thứ N thêm vào cũng bị kiểm |
| Miễn cho RPC khác "cùng họ" | Danh sách theo tên đầy đủ; tên gần giống (`ResolveHostSomethingElse`) không được miễn — `core/grpcx/grpcx_test.go:105-119` |
| Miễn cho các lời gọi tiếp theo | Miễn gắn với **một** RPC, không với yêu cầu. Mọi RPC sau khi đã biết xã vẫn phải mang `x-tenant-id` (ADR 0012 quyết định 1) |

## Vì sao `ListTenants` và `ResolveTenantSuccession` VẪN NGOÀI danh sách

Cả hai cũng được hỏi trước khi biết xã, nên một ngày sẽ cần miễn. Chưa miễn vì **phơi bày khác
hẳn**: `ListTenants` trả **toàn bộ** sổ xã, mà ULID trong đó là tiền tố của mọi khoá cache, hàng
đợi, phòng realtime, đường dẫn tệp (luật 1 bất biến 7). Xác thực bên gọi (ADR 0025) đã có nhưng chỉ
nói *bên gọi ở trong cụm*, không nói *là service nào* — nên nó không phải giấy phép. Câu của hai RPC
này **chưa được đặt cho người dùng**. Một câu đã trả lời không trả lời câu khác.

`core/grpcx/grpcx_test.go:105-108` ghim hai tên ấy vào danh sách **không** miễn: sửa chú thích
thôi thì không gỡ được quyết định.

## Điều kiện để thêm một tên

Thêm bất kỳ tên nào vào `methodsWithoutTenant` là **điều kiện dừng của ADR 0012 quyết định 1**:
hỏi người dùng, và ghi quyết định thành ADR. Đạt phép thử của ADR 0012 là **điều kiện cần**, không
phải giấy phép. `OpenCitizenSession` và `ResolveMiniApp` đi đúng đường này ngày 25/09/2026 — lý do
của hai tên ấy thuộc ADR 0045, không chép ở đây.

## Cái giá

- Danh sách miễn không còn "rà bằng mắt trong một lần" như ADR 0012 mong: hôm nay bốn tên
  (`grpcx.go:199-204`). Mỗi tên thêm là một diff phải có ADR đi kèm.
- `ResolveCitizenSession` nằm trên đường nóng của mọi yêu cầu công dân; miễn xã không làm nó rẻ hơn,
  chỉ làm nó gọi được.

→ ADR 0012 quyết định 1 (metadata, danh sách trắng, điều kiện dừng): `kb/10-decisions/0012-grpc-boundary-contract.md`
→ ADR 0022 (xã của kênh công dân đến từ phiên): `kb/10-decisions/0022-ria-kenh-cong-dan.md`
→ ADR 0025 (khoá gọi nội bộ — miễn xã không miễn nó): `kb/10-decisions/0025-xac-thuc-giua-cac-service.md`
→ ADR 0045 (`OpenCitizenSession`, `ResolveMiniApp` vào danh sách): `kb/10-decisions/0045-cau-phien-cong-dan-mini-app.md`
→ Mã: `core/grpcx/grpcx.go` (`methodsWithoutTenant`) · Ca kiểm: `core/grpcx/grpcx_test.go` (`TestDanhSachMienLaTuongMinh`), `core/grpcx/caller_auth_exempt_test.go` (`TestMoiRpcDuocMienXaVanPhaiXacThuc`)
