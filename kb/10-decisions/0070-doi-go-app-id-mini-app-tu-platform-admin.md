---
id: 0070-doi-go-app-id-mini-app-tu-platform-admin
tier: T1
source: CURATED
owner: architecture
derived_from_commit: 05ad9f2c
expires: null
owns_facts:
  - "người vận hành ViHAT đổi App ID Mini App riêng của xã từ màn chi tiết xã ở platform-admin: một giao dịch gắn App ID mới và TẮT App ID cũ (không xoá), ghi nhật ký gan_mini_app + tat_mini_app; xã luôn chỉ có một app riêng đang chạy (chốt 02/10/2026)"
  - "gỡ Mini App khỏi xã = tắt liên kết (dang_hoat_dong=false), BẮT BUỘC lý do, có nhật ký; App ID đã tắt bật lại được cho CHÍNH xã đó (cũng bắt buộc lý do); không bao giờ chuyển App ID sang xã khác (chốt 02/10/2026)"
  - "khoá bí mật Zalo của App ID nhập ngay trên màn chi tiết xã, lưu mã hoá ở service-identity (ADR 0066 dòng 4); đổi hoặc gỡ App ID thì khoá của App ID cũ tự thu hồi (chốt 02/10/2026)"
  - "đổi/gỡ/bật lại App ID bảo vệ bằng hộp xác nhận hỏi đúng việc + lý do, quyền ops.mini_app.manage; không đòi nhập lại TOTP (chốt 02/10/2026)"
  - "khoá bí mật App ID đi platform-admin → service-platform (tuyến vận hành) → identity qua gRPC 9093; platform chỉ chuyển tiếp, không lưu, không log — thay dòng 'không vòng qua service-platform' của ADR 0066 dòng 4 (chốt 02/10/2026)"
  - "thao tác của người vận hành ViHAT trên một xã chỉ hiện ở nhật ký vận hành, không hiện trên màn nhật ký hệ thống của cán bộ xã; dòng nhật ký vẫn ghi cùng giao dịch, actor_kind operator, mã VH- (chốt 02/10/2026)"
---

# 0070. Đổi và gỡ App ID Mini App của xã từ platform-admin

**Trạng thái:** đã chốt · **Ngày:** 2026-10-02 · **Người quyết:** chủ dự án, 02/10/2026 · **Thay** job
Jenkins một lần `doi-app-id-thang-binh` / `gan-mini-app-thang-binh` (gỡ khỏi job khi màn này đã kiểm
trên prod — ADR 0048 §01/10 #6d).

## Bối cảnh

platform-admin chỉ **gắn** được App ID (đợt 1, ADR 0048). Gắn thêm thì App ID cũ vẫn chạy — xã có hai
app riêng. Đổi App ID của Thăng Bình (01/10) phải làm bằng một stage Jenkins viết tay. Khoá bí mật của
app chỉ đặt được bằng `operatorctl`.

## Quyết định

| # | Điểm | Chốt |
|---|---|---|
| 1 | Đổi App ID | **Thay hẳn trong một giao dịch**: gắn mới + tắt cũ, nhật ký `gan_mini_app` (có `thay_cho`) + `tat_mini_app` — đúng ngữ nghĩa stage Jenkins; xã luôn một app riêng đang chạy |
| 2 | Gỡ | **Tắt** liên kết, bắt buộc lý do, nhật ký `tat_mini_app`; không xoá mềm, không xoá cứng |
| 3 | Bật lại | App ID đã tắt **bật lại được cho chính xã đó**, bắt buộc lý do; **không** chuyển sang xã khác (đó là việc gộp xã — luật 1 STOP #3) |
| 4 | Khoá bí mật | Nhập **cùng màn** chi tiết xã, lưu mã hoá ở **service-identity** (platform-admin gọi tuyến vận hành của identity — ADR 0066 dòng 4); đổi/gỡ thì khoá của App ID cũ **tự thu hồi** |
| 5 | Bảo vệ | Hộp xác nhận hỏi đúng việc + lý do, quyền `ops.mini_app.manage` (khoá đã có); không nhập lại TOTP |

**Vì sao thay hẳn chứ không chạy song song:** chủ dự án đã chọn "TẮT, không để hai app cùng chạy" khi
đổi App ID Thăng Bình (01/10) và nay chốt thành quy tắc chung.

**Vì sao bật lại được (nới thiết kế cũ):** khoá chính `mini_app.app_id` giữ cả dòng đã tắt nên trước
đây một App ID gỡ nhầm là mất vĩnh viễn. Chỉ cho bật lại **trong cùng xã** giữ đúng điều thiết kế cũ
muốn chặn — một App ID lặng lẽ đổi sang xã khác.

### Bổ sung 02/10/2026 — đường đi của khoá bí mật, nhật ký

Agent dựng phát hiện quyết định #4 ("platform-admin gọi thẳng tuyến vận hành của identity", chép từ
ADR 0066 dòng 4) **không làm được như viết**: identity không có cổng HTTP vận hành; platform-admin chỉ
tới được service-platform (bộ định tuyến + NetworkPolicy luật 12); ADR 0048 §01/10 #2 chốt
service-platform là cổng HTTP **duy nhất** của khu vận hành. Chủ dự án chốt:

| # | Điểm | Chốt |
|---|---|---|
| 6 | Đường đi của khoá bí mật | platform-admin → **service-platform** (tuyến vận hành, `ops.mini_app.manage`) → **identity qua gRPC 9093** (kênh vận hành đã có). Platform chỉ chuyển khoá đi, không lưu, không ghi log. **Thay** dòng "không vòng qua service-platform" của ADR 0066 dòng 4 và #4 ở trên |
| 7 | Nhật ký | Thao tác của người vận hành trên một xã **chỉ hiện ở nhật ký vận hành** — không hiện trên màn nhật ký hệ thống cán bộ xã xem (ADR 0054). Dòng nhật ký **vẫn ghi** cùng giao dịch (luật 6 bất biến 1), loại người làm `operator`, mã VH- |

**Vì sao qua platform:** không mở kênh mạng mới mang khoá bí mật (luật 13 STOP), không chép phần kiểm
phiên vận hành sang dịch vụ khác, không thêm biến cấu hình, không đổi bộ định tuyến.

## Hệ quả

- Đổi App ID là việc **nhiều dịch vụ, không nguyên tử**: platform (liên kết) → identity (khoá mới, thu
  hồi khoá cũ) → ngoài kho (`vihat-miniapp` biến `ZALO_MINIAPP_COMMUNE_APP_SECRETS`, bản build
  citizen-app trỏ App ID mới, token đẩy Zalo theo App ID). Màn chi tiết xã hiện rõ các bước ngoài kho
  còn phải làm. Ghi ranh giới trong `kb/30-indexes/transaction-boundaries.json`.
- Sau khi đổi, dân đăng nhập qua App ID mới cần khoá mới; tài khoản Zalo khoá theo `(app_id, zalo user)`
  nên dân hiện như tài khoản Zalo mới dưới App ID mới (ADR 0045 UNKNOWN #2 còn mở).
- Gỡ có hiệu lực ngay với lượt đăng nhập mới (không có cache); phiên đang mở sống tới hết hạn phiên.
