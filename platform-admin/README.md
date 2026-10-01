# platform-admin

The console the **vendor** uses to operate the platform across many communes.

## What this application may do — and may not

| May | May not |
|---|---|
| Create, lock, rename a commune; assign its domain | Read the content of files, petitions, documents |
| Quotas, service tier, active status | Read citizen personal data |
| System logs, service health | Read a commune's business audit trail |
| Aggregate counts received **via events** | Query a business database directly |

This is not a permission setting. **There is no client for the business services in this
application** — see `src/lib/api.ts`. The vendor cannot read commune data because no path
exists, and adding one would have to be written and reviewed.

→ `kb/10-decisions/0003-platform-admin-metadata-only.md`

## Why this is a separate application, not a role in commune-admin

A role can be granted. An application that was never given the client cannot be granted its
way into the data. For a government platform holding citizen data on behalf of public
authorities, that difference is the whole point.

## If support genuinely needs to see real data

It must be a **support session granted by the commune**: time-limited, narrow in scope, fully
audited, and **visible to the commune**. Never a standing permission.

## Chạy

```sh
npm install
npm run dev        # phát triển
npm run build && npm run start
npm run typecheck && npm run lint && npm test
```

Trình duyệt chỉ gọi đường tương đối cùng nguồn `/api/v1/...`; máy chủ Next của app này chuyển
tiếp sang `service-platform` (`src/lib/server/gateway.ts`). Không có biến `NEXT_PUBLIC_*` nào
(ADR 0048 điều kiện dừng #5).

| Biến (chỉ phía máy chủ) | Nghĩa | Vắng thì |
|---|---|---|
| `PLATFORM_HTTP_ADDR` | Origin cổng REST của `service-platform` trong cụm, dạng `scheme://host:port`, không đường dẫn, không thông tin đăng nhập. Trong cụm: tên Service `platform`, cổng `rest` (`deploy/base/platform/service.yaml`) | Mọi `/api/v1/*` trả **503**, không đoán địa chỉ nào; log nêu tên biến, không nêu giá trị |
| `OPERATOR_HOST` | Cùng tên, cùng giá trị với biến của `service-platform` (`core/config/operator.go`): tên host trần, viết thường, không scheme / cổng / đường dẫn, ví dụ `admin.vigov.vn`. Cổng nối gửi giá trị này làm `Host` cho **mọi** lần gọi, không bao giờ chuyển `Host` của trình duyệt — `service-platform` đưa mọi Host khác vào chuỗi của xã | Như trên: **503**, không gọi đi; log nêu tên biến, không nêu giá trị |

Đợt 1 (ADR 0048 §01/10 #4) đã nối với tuyến vận hành của `service-platform`
(`service-platform/internal/http/operator_routes.go`): đăng nhập + đăng ký ứng dụng xác thực lần
đầu (`/dang-nhap`), đổi mật khẩu và tạo lại mã khôi phục (`/tai-khoan/*`), danh sách xã (`/xa`), tạo
xã (`/xa/moi`), chi tiết xã (`/xa/[id]`). Kiểu dữ liệu trong `src/lib/api.ts` viết tay có chủ ý —
tuyến vận hành không nằm trong `kb/20-contracts/openapi.json` — lý do ghi ở đầu tệp.
