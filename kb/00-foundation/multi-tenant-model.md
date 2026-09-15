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

Thứ tự ưu tiên xác định xã ở kênh công dân:

| Ưu tiên | Nguồn | Độ tin |
|---|---|---|
| 1 | **Deep-link** — mã QR dán tại trụ sở xã, link xã gửi qua ZNS | Cao — xã chủ động phát hành |
| 2 | **Xã công dân đã chọn trước đó**, lưu trong hồ sơ | Cao |
| 3 | **Vị trí GPS** đối chiếu ranh giới hành chính | Trung bình — chỉ **gợi ý** |
| 4 | **Người dùng tự chọn** từ danh sách có tìm kiếm | Phương án cuối, **luôn phải có** |

Ràng buộc không thương lượng:

- GPS **gợi ý**, không **quyết định**. Vị trí giả mạo được, và ranh giới xã ở đô thị chạy
  giữa lòng đường.
- Đã chọn xã thì **hiện rõ tên xã trên mọi màn hình**. Gửi phản ánh nhầm xã là sự cố
  nghiệp vụ thật: xã nhận việc không thuộc địa bàn, người dân chờ vô ích.
- Đổi xã là **hành động tường minh**, không tự đổi theo GPS.
- Backend **không tin** xã do Mini App gửi — đối chiếu với quan hệ công dân ↔ xã trong phiên.

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
