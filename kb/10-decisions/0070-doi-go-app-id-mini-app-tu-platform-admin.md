---
id: 0070-doi-go-app-id-mini-app-tu-platform-admin
tier: T1
source: CURATED
owner: architecture
derived_from_commit: 808b92a6
expires: null
owns_facts:
  - "người vận hành ViHAT đổi App ID Mini App riêng của xã từ màn chi tiết xã ở platform-admin: một giao dịch gắn App ID mới và XOÁ MỀM dòng mini_app cũ (deleted_at/by/delete_reason), ghi nhật ký; xã chỉ có MỘT App ID tại một thời điểm (chốt 02/10/2026, sửa 05/10/2026 — xoá mềm thay cho tắt)"
  - "gỡ Mini App khỏi xã = XOÁ MỀM dòng mini_app, BẮT BUỘC lý do, có nhật ký; KHÔNG bật lại — dòng cũ chỉ còn là lịch sử + nhật ký; khoá chính app_id vẫn cấm dùng lại App ID cũ; không bao giờ chuyển App ID sang xã khác (chốt 02/10/2026, sửa 05/10/2026 — bỏ bật lại)"
  - "khoá bí mật App ID chỉ ghi, không bao giờ hiện lại; màn chỉ hiện 'đặt lúc … bởi …'; một RPC đọc mới của identity chỉ trả siêu dữ liệu trạng thái (chốt 05/10/2026)"
  - "liên kết mở / QR của một xã: mọi xã đang hoạt động có QR app dùng chung https://zalo.me/s/<App ID app chung>/?d=<tên miền chính>&src=qr (phiên qua cầu của vihat-miniapp); xã có app riêng đang sống có THÊM QR app riêng https://zalo.me/s/<App ID xã>/?src=qr; người vận hành chọn in tấm nào, tuyến liên kết trả cả hai; QR từ đâu thì mở app từ đó; QR in trước vẫn chạy (chốt 05/10/2026, sửa 06/10/2026 — bỏ 'app riêng trước, app chung chỉ dự phòng'; THAY 13/10/2026 ở platform-admin và web-admin — §Sửa đổi 13/10/2026)"
  - "mỗi bề mặt (chi tiết xã ở platform-admin, Cấu hình → Kênh Zalo ở web-admin) hiện ĐÚNG MỘT QR của xã: xã có app riêng đang sống → https://zalo.me/s/<App ID xã>/?src=qr; không có → app chung chỉ với mã công khai ?x=<mã công khai>&src=qr; không hiện thông tin app chung, không còn link d= (chủ dự án, 13/10/2026)"
  - "cán bộ xã xem, tải PNG, in/lưu PDF QR mở Mini App của CHÍNH xã mình ở web-admin → Cấu hình → Kênh Zalo: tuyến GET /api/v1/commune-launch-link của service-platform, xã suy từ Host, quyền admin.lookup, chỉ đọc, KHÔNG ghi vết (chủ dự án, 13/10/2026)"
  - "QR app riêng chỉ khi người vận hành đã đánh dấu App ID ấy 'Đã phát hành trên Zalo' (cột mini_app.published_at/published_by, bật/tắt ở chi tiết xã của platform-admin, có ghi vết); chưa đánh dấu → QR app chung ?x= (chủ dự án, 13/10/2026)"
  - "tải PNG và 'In / Lưu PDF' (in của trình duyệt, khổ A4: tên xã, QR, câu 'Quét mã bằng Zalo để mở ứng dụng của xã') có ở cả platform-admin lẫn web-admin (chủ dự án, 13/10/2026)"
  - "cấu hình Mini App riêng của xã (App ID ↔ xã, khoá bí mật) làm ở platform-admin; stage Jenkins doi-app-id-thang-binh / dat-secret-mini-app gỡ khi màn đã kiểm trên prod, bat-demo-mini-app / tat-demo-mini-app gỡ ngay (chốt 05/10/2026)"
  - "khoá bí mật Zalo của App ID nhập ngay trên màn chi tiết xã, lưu mã hoá ở service-identity (ADR 0066 dòng 4); đổi hoặc gỡ App ID thì khoá của App ID cũ tự thu hồi (chốt 02/10/2026)"
  - "đổi/gỡ App ID bảo vệ bằng hộp xác nhận hỏi đúng việc + lý do, quyền ops.mini_app.manage; không đòi nhập lại TOTP (chốt 02/10/2026)"
  - "khoá bí mật App ID đi platform-admin → service-platform (tuyến vận hành) → identity qua gRPC 9093; platform chỉ chuyển tiếp, không lưu, không log — thay dòng 'không vòng qua service-platform' của ADR 0066 dòng 4 (chốt 02/10/2026)"
  - "thao tác của người vận hành ViHAT trên một xã chỉ hiện ở nhật ký vận hành, không hiện trên màn nhật ký hệ thống của cán bộ xã; dòng nhật ký vẫn ghi cùng giao dịch, actor_kind operator, mã VH- (chốt 02/10/2026)"
---

# 0070. Đổi và gỡ App ID Mini App của xã từ platform-admin

**Trạng thái:** đã chốt · **Sửa đổi 05/10/2026** (xoá mềm, bỏ bật lại, khoá chỉ ghi, QR theo app
riêng — §*Sửa đổi 05/10/2026*) · **Sửa đổi 06/10/2026** (QR: cả app chung lẫn app riêng, người vận
hành chọn — §*Sửa đổi 06/10/2026*) · **Sửa đổi 13/10/2026** (cán bộ xã có QR của chính xã ở
web-admin; mỗi bề mặt một QR, app chung chỉ `x=`; tải PNG, in A4 — §*Sửa đổi 13/10/2026*) · **Ngày:** 2026-10-02 · **Người quyết:** chủ dự án, 02/10/2026 · **Thay** job
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
| 4 | Liên kết mở / QR của xã | **Thay 06/10/2026 — §*Sửa đổi 06/10/2026* #1** (bỏ "app riêng trước, app chung chỉ dự phòng"). Xã có **app riêng đang sống** → `https://zalo.me/s/<App ID xã>/?src=qr` — **không `d=`**, vì app riêng đã nung xã lúc dựng (ADR 0047 mục 6). Không có → liên kết **app dùng chung** như cũ: `https://zalo.me/s/<app dùng chung>/?d=<tên miền chính>&src=qr`. **Thay** dạng liên kết duy nhất chốt 04/10/2026 (`kb/00-foundation/ubiquitous-language.md` §*Tài nguyên URL của khu vận hành*) và dòng QR của ADR 0048 §*Phạm vi của khu*. QR đã in trước đó **vẫn chạy** |
| 5 | Danh tính cố định (`--demo`) | **Gỡ hẳn** — ADR 0066 §*Sửa đổi 05/10/2026* sở hữu điều này |

Lời chủ dự án cho #2: *"xóa luôn dòng cũ, lấy app mới nhất, hoặc nếu tắt thì nó chỉ là log thôi, ở 1
thời điểm chỉ có 1 appId của 1 xã thôi"*, rồi *"đúng rồi, xoá mềm giữ làm lịch sử"*; chọn *"Bỏ bật
lại"*.

**Dòng đã tắt trước 05/10/2026** (vd App ID `3291993990104489440` của Thăng Bình, tắt 01/10): chủ dự án
chọn *"Migration chuyển sang xoá mềm"* — `service-platform/migrations/0021_own_mini_app_switched_off_to_soft_delete.sql`,
chủ thể `system`, một dòng nhật ký `tat_mini_app` mỗi App ID, cùng giao dịch. Xã ngừng hoạt động hoặc đã
sáp nhập bị **bỏ qua** (luật 7 bất biến 6) — đổi chúng là quyết định riêng.

**Vì sao xoá mềm thay cho tắt:** một dòng tắt mà bật lại được là một App ID thứ hai **đang chờ** của
xã — đúng điều #1 muốn tránh (xã hai app). Xoá mềm nói rõ dòng ấy đã hết vai trò, mà vẫn giữ bản ghi
cho người tra cứu sau (luật 7).

**Giá chủ dự án chấp nhận:** lý do *"một App ID gỡ nhầm là mất vĩnh viễn"* (đoạn *Vì sao bật lại
được*) **quay lại**: gỡ nhầm thì App ID ấy không gắn lại được, kể cả cho chính xã đó — phải đăng ký app
mới trên Zalo. Hộp xác nhận + lý do (#5 ở trên) là lớp chặn duy nhất.

**Bước tay còn lại sau mỗi lần đổi App ID** (ngoài kho, như §*Hệ quả*): dựng lại và đẩy `citizen-app`
cho App ID mới (Zalo đòi). Biến `ZALO_MINIAPP_COMMUNE_APP_SECRETS` của `vihat-miniapp` (phục vụ đổi mã
vị trí) là **bản sao thứ hai** của khoá — ngoài phạm vi đợt này.

## Sửa đổi 06/10/2026 — QR từ đâu thì mở app từ đó

Mục này ghi thêm, không sửa phần trên: mục này thắng khi nói khác. **Người quyết:** chủ dự án,
06/10/2026: *"QR từ đâu thì mở app từ đó."* **Chưa dựng** — mã ở `5e44473a` còn làm theo #4 cũ (app
riêng trước, thiếu thì app chung).

| # | Điểm | Chốt |
|---|---|---|
| 1 | QR app dùng chung | **Mọi xã đang hoạt động** in được QR app dùng chung ViHAT: `https://zalo.me/s/<App ID app chung>/?d=<tên miền chính>&src=qr` — **kể cả khi xã đã có app riêng đang sống**. Phiên của QR này đi qua cầu của `vihat-miniapp` (identity cổng 9091; ADR 0045, và ADR 0066 §*Đã quyết 01/10/2026* dòng *Cầu 9091*: app chung vẫn dùng cầu). **Thay** §*Sửa đổi 05/10/2026* #4 (*"app riêng trước, app chung chỉ dự phòng"*) |
| 2 | QR app riêng | Xã có app riêng **đang sống** có **thêm** QR `https://zalo.me/s/<App ID xã>/?src=qr` — không `d=`, như #4 cũ |
| 3 | Ai chọn tấm in | **Người vận hành** chọn. Tuyến liên kết mở (`communes/{id}/mini-app-launch-link`) trả **cả hai** — app chung luôn có, app riêng chỉ khi đang sống. Quyền `ops.qr.issue` và việc không ghi vết giữ nguyên (ADR 0073 §*Sửa đổi 05/10/2026*) |
| 4 | Không chuyển giữa hai app | Quét QR app nào thì làm việc **trong app ấy**. App chung không đẩy công dân sang app riêng của xã (đúng câu 3 của ADR 0047: *"không chuyển sang app riêng"*) |

**Vì sao:** app riêng của xã chưa được Zalo duyệt nhanh (thủ tục), nên app chung ViHAT được **mượn**
để xã dùng ngay toàn bộ kênh công dân. Lý do và những gì app chung hiện ra khi mở từ QR của xã: ADR
0047 §*Sửa đổi 06/10/2026* sở hữu — không chép ở đây.

**Giữ nguyên:** tham số QR chỉ **dẫn giao diện**, không cấp gì (ADR 0005, 0019, 0022; ADR 0047 câu
3); QR đã in trước đó — cả hai dạng — **vẫn chạy**; xã chỉ có **một** App ID riêng tại một thời điểm
(#2 của §*Sửa đổi 05/10/2026*).

**Cái giá:**

- Một xã có app riêng nay có **hai cửa vào cùng lúc**. Tài khoản Zalo khoá theo `(app_id, zalo user)`
  (§*Hệ quả*), nên một công dân dùng cả hai cửa là hai tài khoản Zalo. Phiếu gửi ở cửa này có hiện ở
  "Phản ánh của tôi" của cửa kia hay không **chưa kiểm** — phụ thuộc ADR 0045 UNKNOWN #2, còn mở.
- Credential đẩy bản dựng của app chung chạm **mọi xã** đang in QR app chung, nay gồm cả xã đã có app
  riêng (ADR 0047 §*Trả lời 27/09* mục 3).

## Sửa đổi 13/10/2026 — cán bộ xã có QR của chính xã; mỗi bề mặt một QR

Mục này ghi thêm, không sửa phần trên: mục này thắng khi nói khác. **Người quyết:** chủ dự án,
13/10/2026, trong phiên chính: *"Trên web admin chỉ có QR app với params trỏ về xã thôi, không có thông
tin app chung. trên platform admin cũng vậy nếu xem ở trong chi tiết từng xã."* Chọn *"chỉ x="* cho
link app chung. **Đã dựng 13/10/2026** — `1e1b8461` (service-platform), `bf1fa420` (platform-admin),
`2aa3a8fc` (web-admin); chưa kiểm trên trình duyệt thật.

| # | Điểm | Chốt |
|---|---|---|
| 1 | Cán bộ xã có QR của **chính xã** | web-admin → Cấu hình → tab **Kênh Zalo**: cán bộ xem, tải, in QR mở Mini App của xã mình. Tuyến `GET /api/v1/commune-launch-link` của **service-platform**; xã suy từ `Host` (luật 1 bất biến 3), không nhận xã từ client; quyền `admin.lookup` — khoá của chính tab ấy (ADR 0074 #6), không thêm khoá `quyen`; chỉ đọc. **Nới** #3 của §*Sửa đổi 06/10/2026*: người vận hành **không còn là người duy nhất** phát QR |
| 2 | Một QR mỗi bề mặt | Chi tiết xã ở platform-admin và tab Kênh Zalo ở web-admin hiện **đúng một** QR. Xã có app riêng **đang sống** → `https://zalo.me/s/<App ID xã>/?src=qr`. Không có → app chung, **chỉ** mã công khai: `https://zalo.me/s/<App ID chung>/?x=<mã công khai>&src=qr` (dạng `x`: ADR 0047 §*Sửa đổi 12/10/2026* #1). **Không** hiện App ID hay thông tin nào khác của app chung. **Thay** #1–#3 của §*Sửa đổi 06/10/2026* (hai QR, người vận hành chọn) **cho hai bề mặt này**; #4 (không chuyển giữa hai app) giữ |
| 3 | Bỏ link `d=` | platform-admin và web-admin **không** đưa link `d=` (`chung_ten_mien`) nữa. Sửa ADR 0047 §*Sửa đổi 12/10/2026* #4 ở vế *"thẻ QR ở platform-admin sinh cả hai"* — xem §*Sửa đổi 13/10/2026* của ADR 0047. `npm run qr` của `citizen-app` **không** thuộc quyết định này |
| 4 | Tải PNG · In / Lưu PDF | Cả hai bề mặt có **tải PNG** và **"In / Lưu PDF"** (in của trình duyệt, khổ A4: tên xã, QR, câu *"Quét mã bằng Zalo để mở ứng dụng của xã"*). Đây là quyết định mà sổ tiến độ `platform-admin` ghi là còn thiếu (*"không có nút tải PNG … cần quyết định"*). **Chép liên kết chỉ ở platform-admin** — chủ dự án, 13/10/2026: *"nút chép liên kết chỉ áp dụng cho platform admin thôi vì nó chứa appId của Vihat trong đó nên không show ở web-admin của xã"* |
| 5 | Không ghi vết | Lần xem/tải/in của cán bộ **không** ghi vết — cùng cái giá ADR 0048 §*30/09* #9 đã chấp nhận cho người vận hành, nay áp thêm cho cán bộ xã |
| 6 | Danh từ URL | `commune-launch-link` — chủ dự án chọn (luật 12). Ghi ở `kb/00-foundation/ubiquitous-language.md` |

**Vì sao một QR:** chủ dự án chỉ nêu câu trích ở trên — màn của xã chỉ mang QR trỏ về xã, không mang
thông tin app chung. Lý do nào khác **chưa ghi**; đừng suy thêm.

**Giữ nguyên:** tham số QR chỉ dẫn giao diện, không cấp gì (ADR 0005, 0019, 0022; ADR 0047 câu 3); xã
chỉ có một App ID riêng tại một thời điểm (§*Sửa đổi 05/10/2026* #2); tuyến vận hành
`communes/{id}/mini-app-launch-link` vẫn dưới `ops.qr.issue`.

**Cái giá:**

- **QR `x=` in trước khi bản app ViHAT đọc `x` được PHÁT HÀNH trên Zalo mở app mà không có xã** (ADR
  0047 §*Sửa đổi 12/10/2026* cái giá (1)). Bỏ `d=` khỏi cả hai bề mặt nghĩa là trong khoảng ấy **không
  còn tấm in nào chạy đúng** cho xã chưa có app riêng. Tại lúc quyết, trạng thái phát hành bản đọc `x`
  **chưa xác nhận**.
- Cán bộ xã nào có `admin.lookup` cũng phát được QR của xã, không truy được ai đã in một tấm sai
  (#5). Tấm sai vẫn chỉ trỏ về chính xã đó, vì xã lấy từ `Host`.
- Xã có app riêng nay chỉ thấy QR app riêng trên hai bề mặt này: cửa app chung của xã ấy vẫn chạy với
  QR đã in, nhưng không in thêm được từ đây.

### Bổ sung 13/10/2026 — QR app riêng chỉ khi App ID đã phát hành trên Zalo

**Người quyết:** chủ dự án, 13/10/2026. Sự cố: *"qr app tạo ra ở trên web-admin đang là bản trải nghiệm,
tôi quét bằng tài khoản zalo thường (không phải admin develop) không vào được"*. Link
`https://zalo.me/s/<App ID>/?src=qr` là link bản phát hành; App ID mà Zalo **chưa phát hành** chỉ mở
được cho tài khoản quản trị/phát triển của app (bản trải nghiệm). Hệ thống không hỏi được Zalo trạng
thái này, nên chủ dự án chọn **cờ do người vận hành bật**.

| # | Chốt |
|---|---|
| 1 | Dòng `mini_app` chế độ `rieng` có thêm `published_at` / `published_by` (mã người vận hành). Trống = **chưa phát hành** |
| 2 | Quy tắc chọn một QR (#2 ở trên) đổi thành: app riêng **đang sống VÀ đã đánh dấu phát hành** → QR app riêng; còn lại → QR app chung `?x=` |
| 3 | Người vận hành bật/tắt cờ ở chi tiết xã (platform-admin), quyền `ops.mini_app.manage`, **ghi vết** cùng giao dịch (luật 6) — khác với việc xem QR, đây là một lần ghi |
| 4 | Không điền sẵn: mọi App ID riêng hiện có bắt đầu ở **chưa phát hành**, nên QR các xã ấy quay về app chung tới khi có người xác nhận — sai theo hướng an toàn |

**Cái giá:** cờ có thể lệch với Zalo (quên bật sau khi phát hành, hoặc Zalo gỡ bản phát hành mà không
ai tắt cờ). Lệch kiểu thứ nhất chỉ làm xã in QR app chung — vẫn vào đúng xã; kiểu thứ hai lặp lại sự cố
này cho tới khi có người tắt cờ.
