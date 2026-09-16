---
id: system-invariants
tier: T0
source: CURATED
owner: architecture
derived_from_commit: 184869f
expires: null
owns_facts:
  - "những điều luôn đúng trong hệ thống ViGov, vi phạm là sự cố"
  - "ba chiều cách ly và quan hệ giữa chúng"
---

# Bất biến hệ thống ViGov

Những điều **luôn đúng**. Vi phạm bất kỳ điều nào là **sự cố**, không phải lỗi.

## Ba chiều cách ly — phải cùng đúng

| Chiều | Câu hỏi | Luật | Hook |
|---|---|---|---|
| **Xã** | Request này thuộc xã nào | 1 | `tenant_scope_guard` |
| **Công dân** | Người này được thấy dữ liệu của ai | 4 | `citizen_scope_guard` |
| **Vai trò** | Cán bộ này được làm gì | 5 | `rbac_guard` |

**Đúng hai trong ba vẫn là lộ dữ liệu.** Đây là bất biến quan trọng nhất của hệ thống.

## Bất biến dữ liệu

| # | Bất biến |
|---|---|
| 1 | Mọi bản ghi nghiệp vụ thuộc **đúng một xã** và **đúng một service sở hữu** |
| 2 | Hồ sơ hành chính **không bị xoá cứng** — chỉ xoá mềm, hoặc ẩn danh |
| 3 | Mọi thao tác ghi để lại **vết không sửa được**, trong cùng giao dịch |
| 4 | Mã nghiệp vụ đã cấp **không cấp lại**, kể cả sau khi xoá mềm |
| 5 | Số văn bản đánh theo **từng cơ quan**, không đánh toàn hệ thống |

## Bất biến định danh

| # | Bất biến |
|---|---|
| 1 | `tenant_id` **vô nghĩa và bất biến** — sáp nhập xã không đụng tới nó |
| 2 | Danh tính công dân là **yếu** (OTP) — không đủ cho hậu quả pháp lý |
| 3 | Danh tính cán bộ là **truy trách nhiệm được** — mọi thao tác quy được về người |

## Bất biến khi hỏng

| Tình huống | Hành vi bắt buộc |
|---|---|
| Không xác định được xã | **404** — không đoán, không mặc định |
| Token lệch xã với domain | **401** + ghi nhật ký báo động |
| Consumer nhận thông điệp thiếu `tenant_id` | **Từ chối xử lý** |
| Lời gọi gRPC không mang `x-tenant-id` trong metadata | **Từ chối** `INVALID_ARGUMENT` — ngoại lệ chỉ theo danh sách trắng tường minh (ADR 0012) |
| Không xác định được chủ sở hữu dữ liệu | **ĐIỀU KIỆN DỪNG** — hỏi người dùng |

> Nguyên tắc chung: **hỏng thì đóng.** Không bao giờ có giá trị mặc định trên đường cách ly.
> Một dòng `?? default` làm sập toàn bộ cách ly, im lặng, và mọi test vẫn xanh.
