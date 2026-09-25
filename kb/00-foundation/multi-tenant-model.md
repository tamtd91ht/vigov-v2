---
id: multi-tenant-model
tier: T0
source: CURATED
owner: architecture
derived_from_commit: df0dae4
expires: null
owns_facts:
  - "mô hình triển khai đa xã trên cloud và cách phân biệt xã"
  - "cách kênh cán bộ và kênh công dân xác định xã khác nhau"
  - "ranh giới tin cậy của Host header"
---

# Mô hình triển khai đa xã

**Một hệ thống, nhiều xã, một hạ tầng cloud.** Không đóng gói riêng cho từng xã.

## Hai kênh, hai cách xác định xã

Đây là điểm dễ hiểu nhầm nhất của hệ thống.

| Kênh | Người dùng | Xác định xã bằng |
|---|---|---|
| Web Quản trị | Cán bộ | **Domain** — `tanphu.vigov.vn` |
| Zalo Mini App | Công dân | **KHÔNG dùng domain được** — xem dưới |

**Vì sao Mini App khác:** Mini App định danh bằng **Zalo App ID**, không có domain của xã.
**Hai chế độ, một bản build** (ADR 0044): app chính lấy xã từ QR, app riêng của xã từ App ID
máy chủ xác minh. Mỗi phiên **đúng một xã** — không chọn xã, không đổi xã, không gửi sang xã khác.

Ba lớp (ADR 0005), không bao giờ gộp:

| Lớp | Trả lời câu | Tin được |
|---|---|---|
| **Khám phá** | Lần mở này trỏ tới xã nào (QR · App ID client đọc) | **Không** — chỉ dẫn giao diện |
| **Phiên** | Phiên này *đang* thao tác ở xã nào | **Có** — máy chủ phát hành |
| **Uỷ quyền** | Công dân này được đọc/ghi gì ở xã đó | **Có** — quan hệ công dân↔xã + luật 4 |

Mọi thứ client tự đọc là **dữ liệu client cung cấp**: dẫn giao diện, không cấp quyền. Mini App
gọi **một API host duy nhất**. Tên xã của phiên hiện trên **mọi màn hình**, xác nhận lại ở bước
cuối trước khi gửi.

→ ADR 0044 · 0005 · 0019 · OA: 0018 (thay 0006) · 0031 · `skills/zalo-miniapp-multi-tenant`

## Ranh giới tin cậy: `Host`

Ngày hệ thống phân biệt xã bằng domain, **`Host` trở thành đầu vào an ninh quan trọng ngang
với token**.

| Tình huống | Hành vi bắt buộc |
|---|---|
| Host không khớp xã nào | **404** — không phải 400, không rơi về xã mặc định, không tiết lộ xã nào tồn tại |
| Token cấp cho xã A, gửi tới domain xã B | **401 + báo động + ghi nhật ký** — dấu hiệu tấn công, không phải lỗi người dùng |
| Client tự gửi header tenant | **Xoá sạch** ở cổng web-admin **và** ở biên Go (ADR 0043) |
| Không xác định được xã | **Từ chối.** Không bao giờ có tenant mặc định |
| Cookie phiên | Đặt đúng host của từng xã. **Cấm** `domain=.vigov.vn` |
| Việc nền (hàng đợi, cron) không có Host | Ngữ cảnh xã nằm **trong chính thông điệp** |

Dòng **cookie `.vigov.vn`** đáng gạch chân: một dòng cấu hình, trông vô hại, viết một lần,
và nó **vô hiệu hoá toàn bộ cách ly giữa các xã** trong khi mọi test chức năng vẫn xanh.

## Cấu hình theo xã

Giá trị riêng của một xã (tên, cơ quan cấp trên, logo, toạ độ, SLA, danh mục, số trực) đọc
**lúc chạy** từ cấu hình theo xã trong CSDL.

**Biến môi trường không dùng được** — nó là hằng số của tiến trình, mà một tiến trình phục
vụ N xã. `NEXT_PUBLIC_*` càng không — nó nung vào bundle lúc build, và một bundle không thể
mang tên của 300 xã.

→ Luật 1: `.claude/rules/critical/1-tenant-isolation.md`
→ Kỹ năng: `skills/go-tenant-context` · `skills/zalo-miniapp-multi-tenant` · `skills/nextjs-multi-tenant`
