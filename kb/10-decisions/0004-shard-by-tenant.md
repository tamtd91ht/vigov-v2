---
id: 0004-shard-by-tenant
tier: T1
source: CURATED
owner: architecture
derived_from_commit: null
expires: null
owns_facts:
  - "chiến lược lưu trữ và phân mảnh theo tenant ở quy mô 200+ xã"
---

# 0004. `tenant_id` là shard key

**Trạng thái:** đã chốt · **Ngày:** 2026-09-15 · **Đóng câu hỏi mở #5**

## Bối cảnh

Quy mô dự kiến 12 tháng: **200+ xã, nhiều tỉnh**. Ở quy mô đó, một bảng phẳng chứa dữ liệu
mọi xã sẽ gặp vấn đề về kích thước chỉ mục và về việc một xã lớn làm chậm mọi xã khác.

## Quyết định

| # | Quyết định |
|---|---|
| 1 | Mỗi service một **schema riêng**; không service nào truy cập schema của service khác |
| 2 | `tenant_id` là **shard key** và là **cột đầu tiên của mọi chỉ mục** |
| 3 | Bảng lớn (`vanban`, `donthu`, `audit`) **partition theo `tenant_id`** ngay từ migration đầu |
| 4 | Di trú chạy **theo từng xã**, dừng và tiếp tục được, ghi tiến độ |
| 5 | Phân giải `Host` → tenant có **cache TTL ngắn**; huỷ cache khi xã đổi tên miền |
| 6 | Một xã đủ lớn có thể tách sang cụm riêng **mà không đổi mã** — vì mọi truy vấn đã mang `tenant_id` |

## Vì sao partition từ đầu chứ không đợi

Thêm partition vào bảng đã có dữ liệu thật là một lần di trú trên **tài liệu lưu trữ**, phải
làm ngoài giờ, có rủi ro. Làm từ migration đầu thì tốn 0 đồng.

## Hệ quả

- **Dễ hơn:** tách cụm về sau là việc vận hành, không phải việc viết mã
- **Khó hơn:** mọi truy vấn phải mang `tenant_id` — nhưng luật 1 đã bắt buộc điều đó rồi, nên
  không phát sinh thêm ràng buộc nào
- **Phải trả sau:** truy vấn xuyên xã (báo cáo cấp huyện) đắt hơn hẳn, và đó là một lý do nữa
  để `baocao` là read model riêng thay vì truy vấn thẳng
