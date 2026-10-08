---
id: 0074-zalo-bot-kenh-nhac-viec-can-bo
tier: T1
source: CURATED
owner: architecture
derived_from_commit: 285aecfd
expires: null
owns_facts:
  - "Zalo Bot là kênh nhắc việc CHO CÁN BỘ, kênh thêm bên cạnh chuông — không phải kênh công dân; đợt 1 chỉ MỘT bot dùng chung của nền tảng, bot riêng của xã để sau (schema chừa chỗ, khi làm thì cấu hình ở platform-admin) (chốt 05/10/2026)"
  - "dữ liệu Zalo Bot (cấu hình bot, mã ghép, liên kết cán bộ↔chat, cài đặt kênh của xã, lịch sử gửi) thuộc service-comms; platform-admin quản token/webhook qua service-platform (tuyến vận hành) → comms gRPC; comms TIN kết quả kiểm khoá của platform, không tự hỏi lại identity (chốt 05/10/2026)"
  - "khoá vận hành thứ tám ops.zalo_bot.manage canh màn Zalo Bot ở platform-admin (chốt 05/10/2026)"
  - "token bot dùng chung niêm bằng khoá dữ liệu CẤP NỀN TẢNG — một hàm niêm riêng trong core/crypto, không giả làm một xã; token chỉ ghi, không bao giờ hiện lại (chốt 05/10/2026)"
  - "webhook Zalo Bot ở host bot.api.vigov.vn trỏ vào service-comms (chủ dự án trỏ DNS); tuyến công khai, xác thực bằng X-Bot-Api-Secret-Token so thời gian hằng, sai khoá 403 không đọc thân, có giới hạn tần suất (chốt 05/10/2026)"
  - "quyền phía xã: cán bộ ghép/gỡ Zalo của chính mình = AnyAuthenticated lọc theo mã cán bộ của phiên; màn Cấu hình → Kênh Zalo = admin.lookup; không thêm khoá quyen (chốt 05/10/2026)"
  - "nội dung tin Zalo: chủ dự án NỚI luật 3 điểm dừng #2 (được gửi dữ liệu cá nhân sang Zalo), sẽ chặn lại sau; đợt 1 không dùng tới vì tin = tiêu đề + nội dung thông báo chuông, mà comms.proto cấm dữ liệu cá nhân ở đó (chốt 05/10/2026)"
  - "đợt 1 câu chữ tin Zalo là mẫu cố định trong mã; cho xã sửa mẫu để sau (chốt 05/10/2026)"
  - "sáu RPC vận hành Zalo Bot của comms (ZaloBotOperatorService) được gọi KHÔNG cần xã — vào core/grpcx methodsWithoutTenant, kể cả ListZaloBotCommuneStats trả ULID mọi xã cho riêng platform (người dùng chốt 05/10/2026)"
  - "thay token sang TÀI KHOẢN bot khác (id getMe khác) thì xoá mềm mọi liên kết cán bộ↔chat của bot cũ cùng giao dịch, có vết, hộp xác nhận báo trước số cán bộ phải ghép lại; đặt lại token của cùng bot thì giữ liên kết (người dùng chốt 05/10/2026)"
  - "host webhook Zalo Bot là biến toàn nền tảng ZALO_BOT_WEBHOOK_HOST, chỉ comms nạp, bắt buộc ở prod (người dùng chốt 05/10/2026)"
  - "vết thao tác vận hành trên bot dùng chung: comms ghi vết đầy đủ ở bảng vết cấp nền tảng của mình cùng giao dịch; platform ghi thêm dòng siêu dữ liệu cho màn nhật ký vận hành (người dùng chốt 05/10/2026)"
  - "tài nguyên URL Zalo Bot: zalo-bots/shared (+ /check, /webhook, /communes) ở khu vận hành; zalo-links, zalo-links/current (+ /pairing-codes, /test-messages), zalo-channel-settings phía xã; zalo-bot-updates cho webhook (người dùng chốt 05/10/2026)"
---

# 0074. Zalo Bot — kênh nhắc việc cho cán bộ

**Trạng thái:** đã chốt · **Ngày:** 2026-10-05 · **Người quyết:** chủ dự án, 05/10/2026, trong phiên
chính · **Nguồn yêu cầu:** `../vigov-require` `docs/spec/07-viec-nen-va-thong-bao.md` §Kênh Zalo Bot,
`08-tich-hop-ngoai.md` §Zalo Bot, `04-api.md:256-296` (bản đối chiếu N7, MB3). · **Sửa bởi ADR 0079**
(08/10/2026): bot riêng của xã làm ngay, xã tự cấu hình ở web-admin — thay #1, #3, #4 cho phần bot của
xã; bot dùng chung giữ nguyên.

## Bối cảnh

Đặc tả muốn nhắc việc cán bộ qua Zalo Bot (`bot-api.zaloplatforms.com/bot<TOKEN>/<hàm>`). Kho này chưa
có khái niệm nào. Ba ràng buộc của nền tảng Zalo quyết định thiết kế:

- Bot **không nhắn trước** được, và không có API đổi số điện thoại lấy `chat_id` → cán bộ phải tự ghép
  nối bằng mã.
- Token nằm **trong đường dẫn URL** → mọi lỗi HTTP in URL ra là in token ra (`*url.Error` của Go).
- Hạn mức không công bố, chỉ có mã 429.

## Quyết định

| # | Điểm | Chốt |
|---|---|---|
| 1 | Phạm vi | Kênh nhắc việc **cán bộ**, thêm bên cạnh chuông — người chưa ghép vẫn nhận đủ chuông. Đợt 1 **một bot dùng chung**; bot riêng của xã để sau, khi làm thì cấu hình ở platform-admin như App ID (ADR 0070) |
| 2 | Chủ dữ liệu | **service-comms** — đã sở hữu `staff_notification` (0010) và kênh thư. Đúng ranh giới ADR 0003: platform-admin không giữ dữ liệu, chỉ là màn điều khiển |
| 3 | Đường quản trị | platform-admin → service-platform (tuyến vận hành, khoá **`ops.zalo_bot.manage`**, khoá thứ tám) → comms qua gRPC. Comms **tin** kết quả kiểm khoá của platform, không hỏi lại identity — chủ dự án chọn đơn giản thay cho cách identity tự kiểm ở ADR 0070 |
| 4 | Niêm token | **Khoá dữ liệu cấp nền tảng**: bảng khoá riêng ở comms + hàm niêm riêng trong `core/crypto`, tên nói rõ "nền tảng" — không dùng một `tenant_id` giả (luật 1 cấm #1). Token và `secret_token` webhook chỉ ghi, màn chỉ hiện "đặt lúc … bởi …" |
| 5 | Webhook | Host **`bot.api.vigov.vn` → service-comms**, chủ dự án trỏ DNS. Tuyến công khai: so `X-Bot-Api-Secret-Token` thời gian hằng, sai → 403 không đọc thân; giới hạn tần suất (luật 13 #7). Không có Host của xã: xã lấy từ **mã ghép** rồi từ **liên kết**; không ra xã thì bỏ gói, không đoán |
| 6 | Quyền phía xã | Ghép/gỡ của chính mình: `AnyAuthenticated`, lọc theo mã cán bộ của phiên (như chuông). Cấu hình kênh của xã: `admin.lookup` (như tab Máy chủ thư) |
| 7 | Nội dung tin | Chủ dự án **nới** luật 3 điểm dừng #2 — được gửi dữ liệu cá nhân sang Zalo, *"chặn sau"*. Đợt 1 không dùng tới: tin = tiêu đề + nội dung thông báo chuông, và `comms.proto:67-74` cấm dữ liệu cá nhân của dân ở hai trường ấy. Muốn tin giàu hơn thì phải sửa hợp đồng đó trước |
| 8 | Câu chữ | Mẫu cố định trong mã; cho xã sửa (`message_templates`) để sau |

**Luật nghiệp vụ mượn của đặc tả** (đợt 1 làm): mã ghép 8 ký tự không có ký tự dễ nhầm, sống 10 phút,
một lần, chỉ lưu bản băm; sai 3 lần huỷ mã, mỗi chat tối đa 5 lần thử/giờ; một chat một tài khoản và
ngược lại, ghép lại thì xoá mềm liên kết cũ; giờ yên tĩnh mặc định 21h–6h giờ Việt Nam; nhịp nhắc
việc trễ do xã đặt; ngưỡng "sắp đến hạn" dùng chung với chuông; mỗi lần bỏ qua ghi lý do; bộ đọc nhận
cả gói phẳng lẫn gói bọc `result`; 429/408/5xx thử lại có lùi, lỗi khác dừng.

### Tài nguyên URL — người dùng chốt 05/10/2026

| Nơi | Tuyến | Quyền |
|---|---|---|
| Khu vận hành (`OPERATOR_HOST`) | `GET·PUT zalo-bots/shared` · `POST zalo-bots/shared/check` · `GET·PUT zalo-bots/shared/webhook` · `GET zalo-bots/shared/communes` | `ops.zalo_bot.manage` |
| Xã — của chính mình | `GET·DELETE zalo-links/current` · `POST zalo-links/current/pairing-codes` · `POST zalo-links/current/test-messages` | `AnyAuthenticated` |
| Xã — quản trị | `GET zalo-links` · `GET·PUT zalo-channel-settings` | `admin.lookup` |
| Zalo gọi (`bot.api.vigov.vn`) | `POST zalo-bot-updates` | công khai, khoá webhook |

`zalo-bots/shared` chứ không `zalo-bot`: chừa chỗ cho bot riêng của xã (#1). `zalo-links/current` cùng
hình dạng `sessions/current`. `zalo-bot-updates` theo tên "update" của chính Zalo (`getUpdates`).

`chat_id` lưu nguyên dạng (cần để gửi), coi như dữ liệu cá nhân: không log, không trả ra giao diện,
không xuất.

## Hệ quả

- Đợt 1 chỉ có **4 loại tin** để nhắc — `DUE_SOON / OVERDUE / ESCALATION / WEEKLY_DIGEST`
  (`comms.proto:120-132`). Tin "được giao việc" chưa có ở dịch vụ nào; thêm sau.
- Phải mở mạng: platform → comms gRPC; comms → `bot-api.zaloplatforms.com:443`.
- Lấy mã ghép theo chat là đọc **chéo xã** theo bản chất → kho `// @cross-tenant` riêng.
- Hạn mức Zalo chưa biết → hỏi Zalo trước khi bật cho nhiều xã.
