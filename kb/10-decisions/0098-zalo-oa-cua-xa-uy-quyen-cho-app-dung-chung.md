---
id: 0098-zalo-oa-cua-xa-uy-quyen-cho-app-dung-chung
tier: T1
source: CURATED
owner: architecture
derived_from_commit: a42f1e21
expires: null
owns_facts:
  - "MỘT Zalo App dùng chung cho cả nền tảng gọi API OA/ZBS; mỗi xã UỶ QUYỀN OA của mình cho app ấy qua OAuth v4 + PKCE (chủ dự án chốt 16/10/2026)"
  - "App ID, secret và host callback của app dùng chung là BIẾN MÔI TRƯỜNG ZALO_OA_APP_ID / ZALO_OA_APP_SECRET / ZALO_OA_CALLBACK_HOST, nhóm config ZaloOA, chỉ comms khai, bắt buộc ở staging/prod (chủ dự án chốt 16/10/2026)"
  - "callback uỷ quyền OA nhận trên host nền tảng ZALO_OA_CALLBACK_HOST (Zalo chỉ cho một callback URL mỗi app); xã tìm từ `state` CSPRNG của dòng yêu cầu uỷ quyền, đọc chéo xã có @cross-tenant, không bao giờ từ Host; giới hạn 60 lượt/phút mỗi mạng máy gọi"
  - "access token + refresh token OA của xã niêm bằng DEK riêng từng xã của comms (như bot xã, 0022), chỉ ghi; việc nền làm mới trước 25 giờ; refresh token hỏng/quá 3 tháng = dòng 'cần uỷ quyền lại', sổ 0004 ghi chua-cau-hinh-kenh"
  - "tài nguyên URL: zalo-oas/current (+ /authorization-links, /check, /templates, /templates/sync, /templates/{id}) dưới admin.zalo; callback công khai GET zalo-oa-authorizations (chủ dự án chốt 16/10/2026)"
  - "mẫu ZBS nạp từ OA của xã (template/all + template/info), chỉ mẫu ENABLE được gắn mốc; tham số mẫu BẮT BUỘC đúng tên ma_tra_cuu / moc_nhan / viec_tiep_theo, không có bảng ánh xạ (chủ dự án chốt 16/10/2026); bản cài event.MauTinXa đọc bảng mẫu đã gắn mốc"
  - "kết quả tra tài liệu Zalo ngày 16/10/2026 (developers.zalo.me, mức CAO): access token 25 giờ, refresh token 3 tháng dùng một lần và xoay sau mỗi lần làm mới, authorization code 10 phút, callback URL khai trong cấu hình app, ZNS gộp vào ZBS Template Message từ 01/01/2026"
---

# 0098. Zalo OA của xã uỷ quyền cho app dùng chung — cấu hình, token, mẫu tin (khối A)

**Trạng thái:** đã chốt · **Ngày:** 2026-10-16 · **Người quyết:** chủ dự án, 16/10/2026, trong phiên chính
(bốn câu cổng trả lời theo đề xuất) · **Nguồn yêu cầu:** chủ dự án 16/10/2026 — *"tích hợp Zalo OA trên
web-admin để cho phép xã gửi tin ZNS, ZBS cho người dân"*, *"xử lý khối A trước, nó là 1 app dùng chung cho
toàn bộ dự án không phải riêng cho từng xã"*. `../vigov-require` không có gì về OA theo xã (prototype gửi qua
eSMS, `08-tich-hop-ngoai.md` §eSMS) · **Giữ nguyên** ADR 0006, 0018 (mỗi xã một OA gửi tin; comms sở hữu
adapter), 0009 (niêm bí mật theo xã), 0041 (bảng báo dân), 0091 §4 · **Dựng trên** tiền lệ 0022 / ADR 0079 Q1
(bot riêng của xã).

ADR này chỉ quyết **khối A**: xã nối OA, giữ token, nạp và gắn mẫu. Bộ phát ZNS (khối B), đường sự kiện
phản ánh → comms (khối C) và màn gửi đợt tin (khối D) là ADR sau.

## Bối cảnh

Kho đã có sổ thông báo gửi công dân (`service-comms/migrations/0004_thong_bao_gui_cong_dan.sql`), consumer
sự kiện phản ánh với cổng khô `event.MauTinXa` (`internal/event/phieu_doi_trang_thai.go:50-63`), công tắc
nhận thông báo của dân (ADR 0091 §4) và cách cất bí mật theo xã (bot xã, 0022). Thiếu đúng một thứ để mọi
cái trên chạy: **OA của xã và mẫu đã duyệt của OA ấy**, đọc lúc chạy theo xã (luật 1 bất biến 10).

### Kết quả tra tài liệu Zalo — 16/10/2026

Đọc trực tiếp `developers.zalo.me` (nội dung nằm trong iframe `stc-developers.zdn.vn/docs/v2/…`), mức
chứng cứ **CAO** trừ dòng ghi khác. Ghi lại để không phải tra lại và để biết chỗ nào còn phải kiểm.

| # | Điều | Kết quả |
|---|---|---|
| 1 | Luồng uỷ quyền | OAuth v4 + **PKCE**. Admin OA mở `https://oauth.zaloapp.com/v4/oa/permission?app_id&redirect_uri&code_challenge&state`, chọn OA, bấm Cho phép; Zalo gọi **GET** về callback kèm `code` và `oa_id`. Zalo dặn: *"dùng biến state … để xác định authorization code mà bạn nhận được ứng với code verifier nào"*; mỗi yêu cầu một code verifier riêng |
| 2 | Callback | Khai **trong cấu hình app** trên Zalo for Developers — **một** callback URL cho app. Vậy callback không nằm được trên tên miền từng xã |
| 3 | Đổi code · làm mới | `POST https://oauth.zaloapp.com/v4/oa/access_token`, header `secret_key`, thân `app_id`, `grant_type=authorization_code` + `code` + `code_verifier`, hoặc `grant_type=refresh_token` + `refresh_token` |
| 4 | Thời hạn | Authorization code **10 phút, một lần**. Access token **25 giờ**. Refresh token **3 tháng, dùng một lần**: dùng xong thì token cũ vô hiệu, Zalo trả refresh token mới; access token cũ hết hiệu lực ngay khi cái mới được cấp |
| 5 | Giới hạn | *"Mỗi OA có giới hạn số lượng App được ủy quyền nhất định"* — con số không ghi trên trang |
| 6 | ZBS | Từ **01/01/2026** ZNS gộp vào **ZBS Template Message** (`oa.zalo.me/home/documents/guides/zbs-template-message`). OA **và** App đều phải gắn **ZBS Account** để trả phí; một ZBS Account gắn được nhiều app |
| 7 | Quyền app | Nhóm *"Gửi tin qua SĐT"* và *"Quản lý Message Template"*, chọn lúc tạo link cấp quyền |
| 8 | Gửi theo số | `POST https://business.openapi.zalo.me/message/template`, header `access_token`, thân `phone` (84…), `template_id`, `template_data`, `tracking_id`; trả `msg_id`, `quota.dailyQuota / remainingQuota` |
| 9 | Mẫu | `GET https://business.openapi.zalo.me/template/all?offset&limit≤100&status`; trạng thái `ENABLE · PENDING_REVIEW · REJECT · DISABLE · DELETE`; chất lượng `HIGH · MEDIUM · LOW · UNDEFINED`; ba tag `TRANSACTION · CUSTOMER_CARE · PROMOTION`. Chi tiết tham số: `template/info` |
| 10 | Giá | `zalo.solutions/zns/pricing`: 200 ₫/tin cho phần lớn loại, 300 ₫ cho xác thực / chuyển khoản / voucher, chưa VAT (mức TRUNG BÌNH — trang giá, không phải đặc tả) |

**Chưa kiểm:** khung giờ Zalo cho phép gửi (không có trên các trang đã đọc); số app tối đa một OA uỷ quyền
được; hạn mức ngày của OA mới. Ba điều này thuộc khối B, không chặn khối A.

## Phương án

| Phương án | Được | Mất |
|---|---|---|
| **Một app dùng chung, xã uỷ quyền OA** (chọn — chủ dự án) | Một bộ App ID/secret, một callback, một ZBS Account cho app; xã chỉ làm một việc: bấm Cho phép | Token của mọi xã mở bằng một secret app; callback phải nằm trên host nền tảng và tự tìm xã |
| Mỗi xã tự tạo Zalo App | Không có gì dùng chung | 200+ app, 200+ secret, 200+ callback; xã phải có tài khoản developer; không có người vận hành nào rà được |
| Dán refresh token tay (API Explorer) | Không cần tuyến callback | Refresh token xoay sau mỗi lần làm mới, dán tay là chết sau 3 tháng không ai thấy; cán bộ phải cầm một bí mật trong tay |

## Quyết định

| # | Điểm | Chốt |
|---|---|---|
| 1 | App | **Một Zalo App của nền tảng** cho mọi xã. Xã uỷ quyền OA của mình cho app ấy bằng OAuth v4 + PKCE |
| 2 | Secret app | Biến môi trường, nhóm config mới **`ZaloOA`**: `ZALO_OA_APP_ID` (ConfigMap), `ZALO_OA_APP_SECRET` (Secret `comms-secrets`), `ZALO_OA_CALLBACK_HOST` (khối `env:` của Deployment comms). **Chỉ comms khai**, bắt buộc ở staging/prod (ADR 0057). Dev để trống = nút "Kết nối OA" bị từ chối bằng tên |
| 3 | Host callback | Biến riêng `ZALO_OA_CALLBACK_HOST`, cùng hình dạng và cùng phép kiểm `ZALO_BOT_WEBHOOK_HOST` (host trần, không bao giờ host dạng xã). Triển khai **được phép** trỏ cùng giá trị với `bot.api.vigov.vn`; hai tên vì hai vai trò (luật 11 #3). comms tự dựng `https://<host>/api/v1/zalo-oa-authorizations`; đó là callback khai với Zalo |
| 4 | Tìm xã ở callback | Zalo gọi GET không có Host của xã. comms tra **`state`** (CSPRNG, 32 byte, một lần, sống 10 phút như authorization code) trong bảng yêu cầu uỷ quyền — một phép đọc **chéo xã có `// @cross-tenant`**, trả **chỉ định danh xã**, như mã ghép nối bot (ADR 0074 #5, `crosstenant.ZaloBot`). Không khớp, hết hạn, đã dùng → 404 một câu, không nói gì về xã nào. Tìm được xã → mọi việc còn lại chạy **trong ngữ cảnh xã ấy** qua `core/store` |
| 5 | Niêm token | `access_token` và `refresh_token` niêm bằng **DEK riêng từng xã** của comms (`core/crypto.Envelope`, bảng `data_encryption_key` 0008), additional data `zalo_commune_oa/<bí mật>/<xã>/<id dòng>`, như 0022. `code_verifier` của yêu cầu đang chờ cũng niêm. Chỉ ghi: không tuyến nào trả token ra; màn chỉ thấy "nối lúc … bởi …" |
| 6 | Vòng đời token | Việc nền trong comms (khoá advisory như bộ phát Zalo) làm mới access token khi còn **< 2 giờ** hiệu lực; refresh token mới ghi đè cái cũ **trong một giao dịch** (Zalo đã vô hiệu cái cũ). Zalo từ chối refresh token, hoặc refresh token quá 3 tháng chưa dùng → dòng chuyển **`can-uy-quyen-lai`**, màn hiện cờ, bộ phát (khối B) ghi sổ `chua-cau-hinh-kenh`. Không bao giờ gửi bằng token đã hết hạn |
| 7 | Tài nguyên URL (ADR 0011, chủ dự án chốt) | `GET · DELETE zalo-oas/current` · `POST zalo-oas/current/authorization-links` (trả link Zalo) · `POST zalo-oas/current/check` (gọi `getoa`) · `GET zalo-oas/current/templates` · `POST zalo-oas/current/templates/sync` · `PATCH zalo-oas/current/templates/{id}` (gắn/gỡ mốc). Tất cả **`admin.zalo`** (khoá có sẵn, identity 0032). Callback: `GET zalo-oa-authorizations`, **công khai**, trên host callback, giới hạn **60 lượt/phút mỗi mạng máy gọi** (luật 13 #7; chủ dự án duyệt 16/10/2026) |
| 8 | Mẫu tin | "Nạp mẫu từ OA" gọi `template/all` rồi `template/info` từng mẫu; lưu `template_id`, tên, trạng thái, chất lượng, tag, danh sách tham số. **Chỉ mẫu `ENABLE`** được gắn mốc. Mốc = mã trạng thái đích của ADR 0041 (`da-tiep-nhan` · `da-chuyen-xu-ly` · `cho-dan-xac-nhan` · `da-dong` · `khong-tiep-nhan` · `chuyen-cap-tren`) và hai mốc gộp/tách (`gop-phieu` · `tach-phieu`). Một mốc một mẫu, **và một mẫu một mốc** *(sửa cùng phiên 16/10/2026, trước khi có mã đọc: bản đầu ghi "một mẫu có thể gắn nhiều mốc"; mỗi mốc là một câu khác nhau gửi dân, nên một mẫu hai mốc là hai câu cho một văn bản — migration 0031 giữ một cột `milestone` trên dòng mẫu)* |
| 9 | Tham số mẫu | **Bắt buộc đúng tên**: mẫu xã đăng ký với Zalo phải có đúng ba tham số `ma_tra_cuu`, `moc_nhan`, `viec_tiep_theo` — ba khoá sổ 0004 đang CHECK (`domain.ThamSoThongBao`). Thiếu tên nào thì **không cho gắn mốc** và nói rõ thiếu gì. Không có bảng ánh xạ |
| 10 | Bản cài `MauTinXa` | Đọc bảng mẫu: mốc có mẫu `ENABLE` đã gắn → `template_id`; xã không có OA sống, hoặc mốc chưa gắn, hoặc mẫu không còn `ENABLE` → `event.ErrXaChuaCauHinhKenh` (sổ ghi `chua-cau-hinh-kenh`, suy giảm nhìn thấy được — ADR 0006 hệ quả 3) |
| 11 | Ngắt OA | `DELETE zalo-oas/current` có lý do: xoá mềm dòng (giữ token đã niêm làm lịch sử, luật 7), gỡ mọi mốc đã gắn; câu trả lời nhắc xã **thu hồi quyền của app trong OA Manager** — ngắt ở đây không thu hồi gì ở Zalo. Uỷ quyền OA khác = ngắt rồi nối, dòng mới |
| 12 | Vết · nhật ký an ninh | Mỗi hành vi (tạo link, nối xong, kiểm, nạp mẫu, gắn mốc, ngắt) một mục `audit_log` của xã **cùng giao dịch**, before/after không bao giờ mang cột đã niêm; thêm một dòng nhật ký an ninh như bot xã (`zalo_commune_oa.*`). Callback là hành vi của người **ngoài phiên**: chủ thể vết là mã cán bộ đã tạo yêu cầu uỷ quyền (ghi trong dòng yêu cầu), IP là của trình duyệt admin OA |
| 13 | Lỗi Zalo | Lớp, không chữ (khuôn `internal/zalobot`): token không chứa trong URL nên có thể giữ mã lỗi số của Zalo, nhưng **không bao giờ** lưu hay log thân phản hồi, URL kèm `code`, hay bất kỳ token nào |

### Hai cột thời điểm trên dòng OA, vì 0022 có một

0022 chặn "token mới mà `set_at` không đổi" để màn không mô tả token cũ. Dòng OA có **hai** người ghi
token: cán bộ (nối OA) và **việc nền làm mới** (luật 6 bất biến 6, chủ thể hệ thống). Vì vậy dòng OA mang
`authorized_at` / `authorized_by` (người, đổi khi nối) **và** `token_refreshed_at` (đổi mỗi lần làm mới);
guard của bảng đòi `token_refreshed_at` mới khi cột niêm đổi. Chép nguyên guard 0022 thì việc nền bị chặn
ngay lần làm mới đầu tiên.

## Xung đột với kho yêu cầu — ghi cả hai phía, chủ dự án đã chọn

| # | `../vigov-require` | ViGov (chọn) | Vì sao không ghép |
|---|---|---|---|
| C1 | Gửi qua **eSMS**, mẫu duyệt bên eSMS, mã mẫu do eSMS cấp (`docs/spec/05-nghiep-vu.md:214-229`, `08-tich-hop-ngoai.md:70-84`) | ZBS từ **OA của từng xã** qua `service-comms` (ADR 0018; ADR 0097 §Ngoài phạm vi) | Người gửi mà dân thấy phải là UBND xã (ADR 0006). Chủ dự án 16/10/2026 chọn phía ViGov bằng chính yêu cầu "tích hợp Zalo OA" |
| C2 | Tham số rút **tự do** từ thân mẫu (`{ten}`, `{ngay}`; `apps/api/app/modules/citizens/service.py:42-70`) | **Đúng ba tên** `ma_tra_cuu` · `moc_nhan` · `viec_tiep_theo` (sổ 0004 CHECK; `proto/vigov/petitions/v1/events.proto:182-183`) | Mẫu ZBS khai tham số trên cổng Zalo, không ở ViGov; tham số tự do là chỗ dữ liệu cá nhân lọt vào tin (luật 3 bất biến 6) |
| C3 | Prototype `MessageTemplateTable.tsx` | Là tab **Lời hệ thống**, không phải mẫu ZBS | Không trộn hai thứ: mẫu ZBS là `notice_templates` của kho yêu cầu, và nó đi eSMS (C1) |

Spec Cấu hình 11 trên đĩa (`tmp/web/[done]cau-hinh/vigov-cau-hinh-spec/11-kenh-zalo.md`) **không có mục
OA**: phần "OA của xã" không có prototype, dựng theo khuôn mục "Con bot của xã" và ADR 0068 (control "?" cho
phần chưa có).

## Hệ quả

- **Dễ hơn:** khối B chỉ cần "mở token của xã, gọi `message/template`"; khối C/D có mẫu để trỏ. Xã onboard
  = bấm Cho phép + nạp mẫu + gắn mốc; không ai chép token.
- **Khó hơn:** một tuyến công khai mới trên host nền tảng, và một phép đọc chéo xã mới (bằng `state`).
  Cả hai phải có ca kiểm riêng: `state` lạ / hết hạn / dùng lại → 404 cùng một câu; tìm được xã A thì
  không ghi gì sang xã B.
- **Phải trả sau:** (1) thủ tục với Zalo — khai callback URL vào app, gắn ZBS Account cho app và cho từng
  OA, xác thực OA từng xã, đăng ký và duyệt mẫu mang đúng ba tham số; (2) **chi phí ZNS ai trả** (ADR 0018
  còn mở #1) — chưa vào `open-questions.json`, phải đưa vào trước khối D; (3) migration mới của comms:
  `zalo_commune_oa`, `zalo_oa_authorization`, `zalo_oa_template`; (4) ba mục `transaction-boundaries.json`:
  `noi_oa_zalo_cua_xa`, `lam_moi_token_oa_zalo`, `nap_mau_zbs_cua_xa`; (5) `.env.example`, `hints.go`,
  `deploy/cau-hinh/README.md`, Secret/ConfigMap mẫu thêm ba biến; (6) chuỗi kiểm `make kb` sinh lại
  openapi, `go run ./tools/ingress` cho web-admin chuyển tiếp `zalo-oas`.
- **Rủi ro nhận:** secret app là khoá mở token của mọi xã; lộ là xoay secret ở Zalo và uỷ quyền lại mọi xã.
  Chủ dự án chọn biến môi trường (Secret k8s) thay vì màn vận hành; xoay secret = thay Secret + rollout.

## Còn mở — không tự quyết

| # | Việc | Ai |
|---|---|---|
| 1 | Khung giờ gửi, hạn mức ngày, số app tối đa một OA uỷ quyền | hỏi Zalo trước khối B |
| 2 | Ai trả phí ZNS, theo xã hay tập trung (ADR 0018 còn mở #1) | chủ dự án, trước khối D |
| 3 | Mẫu cho mốc gộp/tách có bị chặn khi dân tắt nhận không (ADR 0091 còn mở #1) | chủ dự án |
| 4 | Xã ngắt OA hoặc Zalo thu hồi uỷ quyền khi sổ 0004 còn dòng `cho-gui`: giữ chờ tới khi nối lại, hay chuyển `chua-cau-hinh-kenh` | khối B |
| 5 | Bộ phát cần số điện thoại theo `nguoi_nhan_ma`, nhưng `IdentityService.ResolveCitizenContactPhone` **từ chối** hình `citizen_id → phone` (`proto/vigov/identity/v1/identity.proto:311-316`) — luật 3 điều kiện dừng #2 | contract-designer + chủ dự án, khối B |
| 6 | Tên tiếng Anh của ba thực thể mới (`ZaloCommuneOA` · `ZaloOAAuthorization` · `ZaloOATemplate`) và dòng URL `zalo-oas` phải vào `ubiquitous-language.md` cùng lượt này | knowledge-keeper, TASK-A7 |

→ ADR 0006 · 0018 · 0009 · 0022 (migration) · 0041 · 0074 · 0079 · 0091
→ Luật 1 (bất biến 4, 7, 10; cấm #1, #2) · 3 · 6 · 7 · 8 · 11 · 13
→ Kỹ năng: `.claude/skills/zalo-miniapp-multi-tenant/SKILL.md` §Notifications
