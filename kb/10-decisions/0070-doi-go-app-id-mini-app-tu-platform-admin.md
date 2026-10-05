---
id: 0070-doi-go-app-id-mini-app-tu-platform-admin
tier: T1
source: CURATED
owner: architecture
derived_from_commit: ce5db182
expires: null
owns_facts:
  - "người vận hành ViHAT đổi App ID Mini App riêng của xã từ màn chi tiết xã ở platform-admin: một giao dịch gắn App ID mới và XOÁ MỀM dòng mini_app cũ (deleted_at/by/delete_reason), ghi nhật ký; xã chỉ có MỘT App ID tại một thời điểm (chốt 02/10/2026, sửa 05/10/2026 — xoá mềm thay cho tắt)"
  - "gỡ Mini App khỏi xã = XOÁ MỀM dòng mini_app, BẮT BUỘC lý do, có nhật ký; KHÔNG bật lại — dòng cũ chỉ còn là lịch sử + nhật ký; khoá chính app_id vẫn cấm dùng lại App ID cũ; không bao giờ chuyển App ID sang xã khác (chốt 02/10/2026, sửa 05/10/2026 — bỏ bật lại)"
  - "khoá bí mật App ID chỉ ghi, không bao giờ hiện lại; màn chỉ hiện 'đặt lúc … bởi …'; một RPC đọc mới của identity chỉ trả siêu dữ liệu trạng thái (chốt 05/10/2026)"
  - "liên kết mở / QR của một xã: xã có app riêng đang sống → https://zalo.me/s/<App ID xã>/?src=qr (không d=); không có → liên kết app dùng chung ?d=<tên miền chính>&src=qr; QR in trước vẫn chạy (chốt 05/10/2026)"
  - "cấu hình Mini App riêng của xã (App ID ↔ xã, khoá bí mật) làm ở platform-admin; stage Jenkins doi-app-id-thang-binh / dat-secret-mini-app gỡ khi màn đã kiểm trên prod, bat-demo-mini-app / tat-demo-mini-app gỡ ngay (chốt 05/10/2026)"
  - "khoá bí mật Zalo của App ID nhập ngay trên màn chi tiết xã, lưu mã hoá ở service-identity (ADR 0066 dòng 4); đổi hoặc gỡ App ID thì khoá của App ID cũ tự thu hồi (chốt 02/10/2026)"
  - "đổi/gỡ App ID bảo vệ bằng hộp xác nhận hỏi đúng việc + lý do, quyền ops.mini_app.manage; không đòi nhập lại TOTP (chốt 02/10/2026)"
  - "khoá bí mật App ID đi platform-admin → service-platform (tuyến vận hành) → identity qua gRPC 9093; platform chỉ chuyển tiếp, không lưu, không log — thay dòng 'không vòng qua service-platform' của ADR 0066 dòng 4 (chốt 02/10/2026)"
  - "thao tác của người vận hành ViHAT trên một xã chỉ hiện ở nhật ký vận hành, không hiện trên màn nhật ký hệ thống của cán bộ xã; dòng nhật ký vẫn ghi cùng giao dịch, actor_kind operator, mã VH- (chốt 02/10/2026)"
---

# 0070. Đổi và gỡ App ID Mini App của xã từ platform-admin

**Trạng thái:** đã chốt · **Sửa đổi 05/10/2026** (xoá mềm, bỏ bật lại, khoá chỉ ghi, QR theo app
riêng — §*Sửa đổi 05/10/2026*) · **Ngày:** 2026-10-02 · **Người quyết:** chủ dự án, 02/10/2026 · **Thay** job
Jenkins một lần `doi-app-id-thang-binh` / `gan-mini-app-thang-binh` (gỡ khỏi job khi màn này đã kiểm
trên prod — ADR 0048 §01/10 #6d).

## Bối cảnh

platform-admin chỉ **gắn** được App ID (đợt 1, ADR 0048). Gắn thêm thì App ID cũ vẫn chạy — xã có hai
app riêng. Đổi App ID của Thăng Bình (01/10) phải làm bằng một stage Jenkins viết tay. Khoá bí mật của
app chỉ đặt được bằng `operatorctl`.

## Quyết định

| # | Điểm | Chốt |
|---|---|---|
| 1 | Đổi App ID | **Phần "tắt cũ" thay 05/10/2026 bằng xoá mềm — §*Sửa đổi 05/10/2026* #2.** **Thay hẳn trong một giao dịch**: gắn mới + tắt cũ, nhật ký `gan_mini_app` (có `thay_cho`) + `tat_mini_app` — đúng ngữ nghĩa stage Jenkins; xã luôn một app riêng đang chạy |
| 2 | Gỡ | **Thay 05/10/2026 — xoá mềm, §*Sửa đổi 05/10/2026* #2.** **Tắt** liên kết, bắt buộc lý do, nhật ký `tat_mini_app`; không xoá mềm, không xoá cứng |
| 3 | Bật lại | **BỎ 05/10/2026 — §*Sửa đổi 05/10/2026* #2.** App ID đã tắt **bật lại được cho chính xã đó**, bắt buộc lý do; **không** chuyển sang xã khác (đó là việc gộp xã — luật 1 STOP #3) |
| 4 | Khoá bí mật | Nhập **cùng màn** chi tiết xã, lưu mã hoá ở **service-identity** (platform-admin gọi tuyến vận hành của identity — ADR 0066 dòng 4); đổi/gỡ thì khoá của App ID cũ **tự thu hồi** |
| 5 | Bảo vệ | Hộp xác nhận hỏi đúng việc + lý do, quyền `ops.mini_app.manage` (khoá đã có); không nhập lại TOTP |

**Vì sao thay hẳn chứ không chạy song song:** chủ dự án đã chọn "TẮT, không để hai app cùng chạy" khi
đổi App ID Thăng Bình (01/10) và nay chốt thành quy tắc chung.

**Vì sao bật lại được (nới thiết kế cũ) — RÚT 05/10/2026, §*Sửa đổi 05/10/2026* #2:** khoá chính `mini_app.app_id` giữ cả dòng đã tắt nên trước
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

## Sửa đổi 05/10/2026 — một App ID một lúc, xoá mềm, khoá chỉ ghi, QR theo app riêng

Mục này ghi thêm, không sửa phần trên: phần trên là quyết định lúc viết, mục này thắng khi nói khác.
**Người quyết:** chủ dự án, 05/10/2026, trong phiên chính. **Chưa dựng** — mỗi dòng là điều phải đúng
khi dựng.

| # | Điểm | Chốt |
|---|---|---|
| 1 | Nơi cấu hình | Cấu hình Mini App riêng của xã (App ID ↔ xã, khoá bí mật) làm ở **platform-admin** (`admin.vigov.vn`, realm `operator`) — màn đã dựng theo ADR này. Stage Jenkins `doi-app-id-thang-binh` / `dat-secret-mini-app` **gỡ khi màn đã kiểm trên prod** (ADR 0048 §*01/10* #6d). Stage `bat-demo-mini-app` / `tat-demo-mini-app` **gỡ ngay** (#5) |
| 2 | Một App ID một lúc · xoá mềm · **bỏ bật lại** | Mỗi xã có **đúng một** App ID tại một thời điểm. Đổi App ID ("Đổi App ID") hay gỡ ("Gỡ khỏi xã") **xoá mềm** dòng `mini_app` cũ (`deleted_at` / `deleted_by` / `delete_reason`, luật 7 bất biến 1) — dòng ấy chỉ còn là **lịch sử + nhật ký**. **Không bật lại** nữa: **thay** #3 ở trên và đoạn *"Vì sao bật lại được"*. Khoá chính trên `app_id` giữ nguyên, nên một App ID cũ **không dùng lại được** |
| 3 | Khoá bí mật chỉ ghi | Khoá **không bao giờ hiện lại**; màn chỉ hiện *"đặt lúc … bởi …"*; đổi khoá = nhập khoá mới. Lưu mã hoá trong CSDL bằng KEK trên k8s (đã đúng — ADR 0066 §*Đã quyết 01/10/2026*). Một **RPC đọc mới** của identity trả **chỉ siêu dữ liệu trạng thái**, không bao giờ trả khoá |
| 4 | Liên kết mở / QR của xã | Xã có **app riêng đang sống** → `https://zalo.me/s/<App ID xã>/?src=qr` — **không `d=`**, vì app riêng đã nung xã lúc dựng (ADR 0047 mục 6). Không có → liên kết **app dùng chung** như cũ: `https://zalo.me/s/<app dùng chung>/?d=<tên miền chính>&src=qr`. **Thay** dạng liên kết duy nhất chốt 04/10/2026 (`kb/00-foundation/ubiquitous-language.md` §*Tài nguyên URL của khu vận hành*) và dòng QR của ADR 0048 §*Phạm vi của khu*. QR đã in trước đó **vẫn chạy** |
| 5 | Danh tính cố định (`--demo`) | **Gỡ hẳn** — ADR 0066 §*Sửa đổi 05/10/2026* sở hữu điều này |

Lời chủ dự án cho #2: *"xóa luôn dòng cũ, lấy app mới nhất, hoặc nếu tắt thì nó chỉ là log thôi, ở 1
thời điểm chỉ có 1 appId của 1 xã thôi"*, rồi *"đúng rồi, xoá mềm giữ làm lịch sử"*; chọn *"Bỏ bật
lại"*.

**Vì sao xoá mềm thay cho tắt:** một dòng tắt mà bật lại được là một App ID thứ hai **đang chờ** của
xã — đúng điều #1 muốn tránh (xã hai app). Xoá mềm nói rõ dòng ấy đã hết vai trò, mà vẫn giữ bản ghi
cho người tra cứu sau (luật 7).

**Giá chủ dự án chấp nhận:** lý do *"một App ID gỡ nhầm là mất vĩnh viễn"* (đoạn *Vì sao bật lại
được*) **quay lại**: gỡ nhầm thì App ID ấy không gắn lại được, kể cả cho chính xã đó — phải đăng ký app
mới trên Zalo. Hộp xác nhận + lý do (#5 ở trên) là lớp chặn duy nhất.

**Bước tay còn lại sau mỗi lần đổi App ID** (ngoài kho, như §*Hệ quả*): dựng lại và đẩy `citizen-app`
cho App ID mới (Zalo đòi). Biến `ZALO_MINIAPP_COMMUNE_APP_SECRETS` của `vihat-miniapp` (phục vụ đổi mã
vị trí) là **bản sao thứ hai** của khoá — ngoài phạm vi đợt này.
