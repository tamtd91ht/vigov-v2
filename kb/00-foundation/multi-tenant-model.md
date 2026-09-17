---
id: multi-tenant-model
tier: T0
source: CURATED
owner: architecture
derived_from_commit: null
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

**Vì sao Mini App khác:** Zalo Mini App định danh bằng **Zalo App ID**, không bằng domain.
Mỗi App ID cần đăng ký và duyệt riêng với Zalo — 300 xã không thể là 300 mini app. Một
Mini App phục vụ mọi xã, nên nó **không có domain để mà phân biệt**.

Kênh công dân tách **ba lớp**, không bao giờ gộp:

| Lớp | Trả lời câu | Tin được |
|---|---|---|
| **Khám phá** | Công dân *muốn* làm việc với xã nào (QR · deep link · GPS · picker · hồ sơ) | **Không** — chỉ gợi ý |
| **Phiên** | Phiên này *đang* thao tác ở xã nào | **Có** — server phát hành sau khi công dân xác nhận |
| **Uỷ quyền** | Công dân này được đọc/ghi gì ở xã đó | **Có** — quan hệ công dân↔xã + luật 4 |

Tham số trên QR hay link là **dữ liệu client cung cấp**: nó dẫn giao diện, không cấp quyền.
Mini App gọi **một API host duy nhất**, không bao giờ dựng URL theo từng xã.

GPS **gợi ý**, không **quyết định**. Đã chọn xã thì tên xã hiện trên **mọi màn hình**, và xác
nhận lại ở bước cuối trước khi gửi — gửi nhầm xã là sự cố nghiệp vụ thật.

→ Chi tiết khuôn deep link, mức tin theo nguồn, màn hình chọn xã, và OA theo xã:
ADR 0005 · ADR 0006 → **thay thế bởi ADR 0018** · ADR 0019 · `skills/zalo-miniapp-multi-tenant`

## Ranh giới tin cậy: `Host`

Ngày hệ thống phân biệt xã bằng domain, **`Host` trở thành đầu vào an ninh quan trọng ngang
với token**.

| Tình huống | Hành vi bắt buộc |
|---|---|
| Host không khớp xã nào | **404** — không phải 400, không rơi về xã mặc định, không tiết lộ xã nào tồn tại |
| Token cấp cho xã A, gửi tới domain xã B | **401 + báo động + ghi nhật ký** — dấu hiệu tấn công, không phải lỗi người dùng |
| Client tự gửi header tenant | Reverse proxy **xoá sạch** mọi header tenant từ ngoài vào |
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
