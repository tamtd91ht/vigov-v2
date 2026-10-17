---
id: 0099-gui-dot-tin-zns-sms-esms-nhieu-oa
tier: T1
source: CURATED
owner: architecture
derived_from_commit: 50015576
expires: null
owns_facts:
  - "menu Gửi tin ZNS / SMS (/gui-thong-bao, notice.send) gửi ĐỢT tin tới danh bạ người dân và cán bộ: ZNS từ một OA của xã chọn lúc gửi, SMS qua eSMS từ một tài khoản của xã chọn lúc gửi (chủ dự án 17/10/2026)"
  - "mỗi xã có NHIỀU tài khoản eSMS (ApiKey, SecretKey niêm khoá xã, Brandname), cấu hình ở web-admin Cấu hình → Kênh SMS dưới khoá mới admin.sms; chọn tài khoản khi gửi (chủ dự án 17/10/2026)"
  - "mỗi xã có NHIỀU OA Zalo cùng lúc, chọn OA khi gửi; thay ADR 0098 'một OA sống mỗi xã'; tuyến zalo-oas/current/* bỏ, thay bằng zalo-oas, zalo-oas/{id}, zalo-oa-templates/{id} (chủ dự án 17/10/2026)"
  - "số điện thoại người nhận: comms hỏi identity một RPC mới lúc gửi, giữ số trong bộ nhớ đúng lúc gọi Zalo/eSMS; sổ chỉ lưu số đã che + mã dòng danh bạ; identity ghi vết mở số cho đợt gửi; chủ dự án duyệt gửi số + tham số mẫu sang eSMS (luật 3 điều kiện dừng #2, 17/10/2026)"
  - "người nhận: bỏ qua người 'ngừng liên hệ' (is_active=false) và hiện dòng Bỏ qua; giữ hai lựa chọn cán bộ như prototype, số lấy từ số di động trong danh bạ cán bộ (chủ dự án 17/10/2026)"
  - "chi phí: ước tính trong Xem trước (ZNS theo bảng giá ZBS theo loại mẫu, SMS theo hàm tạm tính giá của eSMS), không bước duyệt, không hạn mức (chủ dự án 17/10/2026)"
  - "'Đã gửi' của lượt này = nhà cung cấp đã nhận (eSMS CodeResult 100, Zalo msg_id); callback trạng thái giao của eSMS để lượt sau (chủ dự án 17/10/2026)"
  - "người nhận nhập tay (≤ 1.000 số) hoặc từ tệp .xlsx/.csv (≤ 10.000 dòng, chọn cột số; SMS: cột nội dung hoặc nội dung chung có biến {{variable_i}} map cột; ZNS: tham số giá trị chung hoặc cột); tệp không lưu; lịch sử lưu cả số không hợp lệ, kết quả khong-hop-le / that-bai (kèm lỗi nhà cung cấp) / da-gui; số đầy đủ + nội dung riêng mã hoá khoá xã tới lúc có kết quả rồi xoá (chủ dự án 17/10/2026, §Sửa đổi)"
  - "tên: SmsAccount/sms_account/sms-accounts, SmsTemplate/sms_template, NoticeCampaign/notice_campaign/notice-campaigns, NoticeRecipient/notice_recipient, notice-templates, notice-campaign-previews (chủ dự án chốt 17/10/2026)"
---

# 0099. Gửi đợt tin ZNS / SMS: nhiều OA, nhiều tài khoản eSMS, số chỉ trong bộ nhớ

**Trạng thái:** đã chốt · **Ngày:** 2026-10-17 · **Người quyết:** chủ dự án, 17/10/2026, trong phiên chính,
hai phiếu hỏi (6 câu + 3 câu + 2 câu) · **Nguồn yêu cầu:** chủ dự án — *"dựng menu Gửi tin ZNS/SMS…, SMS dùng
nhà cung cấp eSMS của Vihat tích hợp qua publicAPI"*; prototype `../vigov-require/apps/admin/src/components/
citizens/NoticeWorkspace.tsx` (bản chạy `vigov-admin-production.up.railway.app/gui-thong-bao`, đối chiếu
17/10/2026) · **Thay một phần** ADR 0098 (#7 tuyến, "một OA sống mỗi xã") · **Dựng trên** ADR 0097 (danh bạ
người dân), 0098 (OA, mẫu ZBS), 0009 (niêm bí mật theo xã).

## Bối cảnh

Màn `/gui-thong-bao` đã dựng giao diện (`e1675e6d`) với "?" cho xem trước, gửi, lịch sử, đơn giá và SMS.
Khối D này lấp các "?" ấy. Prototype gửi qua eSMS và lưu số người nhận trong bảng đợt gửi; v2 khác ở ba chỗ
dưới (luật 3 thắng prototype, ADR 0090).

### eSMS — đọc tài liệu 17/10/2026 (`developers.esms.vn`, mức CAO)

| Điều | Nội dung |
|---|---|
| Gửi SMS CSKH | `POST https://rest.esms.vn/MainService.svc/json/SendMultipleMessage_V4_post_json/`, thân JSON `ApiKey`, `SecretKey`, `Brandname`, `Content`, `Phone`, `SmsType=2`, `IsUnicode`, `RequestId` (≤50 ký tự, chặn trùng 24 giờ), `campaignid`, `CallbackUrl`, `Sandbox=1` (thử, không về tin, không trừ tiền) |
| Kết quả | `CodeResult` `100` = eSMS **đã nhận yêu cầu**, không phải đã tới máy; `101` sai khoá; `104` Brandname chưa kích hoạt; `124` RequestId trùng; `146` sai mẫu CSKH; `99` yêu cầu hỏng |
| Mẫu | Mẫu CSKH phải đăng ký với nhà mạng. `POST …/GetTemplate/` (ApiKey, SecretKey, SmsType=2, Brandname) trả `BrandnameTemplates[{NetworkID, TempContent, TempId}]`; chỗ trống dạng `{P1,20}` |
| Khác | Lấy số dư (kiểm tra kết nối), tạm tính giá CSKH, lấy danh sách Brandname, callback trạng thái |

## Quyết định

| # | Điểm | Chốt |
|---|---|---|
| 1 | Nhiều OA | Một xã nối **nhiều** OA cùng lúc. Migration comms mới bỏ `zalo_commune_oa_one_live_per_commune`; floor xuyên xã trên `oa_id` sống **giữ** (một OA không sống ở hai xã). Mẫu ZBS thuộc OA của nó; mỗi mốc báo dân vẫn MỘT mẫu trong cả xã (`zalo_oa_template_one_per_milestone` giữ), nên tin báo dân theo mốc đi từ OA của mẫu đã gắn — không cần "OA mặc định" |
| 2 | Nhiều tài khoản eSMS | Bảng `sms_account` (`provider = 'esms'`): tên gợi nhớ, ApiKey + SecretKey niêm DEK xã (AAD `sms_account/<bí mật>/<xã>/<id>`), Brandname, mặc định (một mỗi xã), ngừng dùng (xoá mềm). Kiểm tra kết nối = hàm lấy số dư. Mẫu: `sms_template` nạp bằng `GetTemplate` |
| 3 | Quyền | Khoá **mới `admin.sms`** "Cấu hình kênh SMS", migration identity gieo và cấp sẵn cho đúng các vai trò đang có `admin.zalo` (cách 15/10 làm `admin.zalo`). Gửi đợt tin: `notice.send` (có sẵn, identity 0028) |
| 4 | Mẫu để soạn | `GET notice-templates` (`notice.send`) trả chung mẫu ZBS `ENABLE` của mọi OA sống và mẫu SMS của mọi tài khoản sống, mỗi mẫu mang `channel`, nguồn (OA/tài khoản), tên, tham số, đơn giá ước tính. Người chỉ có `notice.send` đọc được — sửa khoảng hở của `e1675e6d` |
| 5 | Số điện thoại | identity thêm RPC trả người nhận của một đối tượng gửi (`citizen_all` / nhóm / nhãn / `staff_all` / bộ phận): mã dòng + số đầy đủ, **bỏ** người `is_active = false` và đếm riêng. identity ghi **một** vết "mở số cho đợt gửi" (số lượng, không số). comms giữ số **trong bộ nhớ** đúng lúc gọi nhà cung cấp; `notice_recipient` lưu mã dòng danh bạ + số **đã che** (`MaskPhone`) — không bao giờ số đầy đủ. Hợp đồng `.proto` thuộc contract-designer |
| 6 | Gửi ra ngoài | Chủ dự án **duyệt** gửi số + giá trị tham số mẫu sang eSMS (luật 3 điều kiện dừng #2, 17/10/2026); sang Zalo đã có ADR 0018. Chỉ những trường ấy, khai ở adapter (luật 3 bất biến 6). Tham số do cán bộ gõ cho cả đợt — không tham số nào lấy từ hồ sơ của từng người |
| 7 | Chi phí | Xem trước: người nhận, bỏ qua, đơn giá, chi phí dự kiến. ZNS: bảng giá ZBS theo loại mẫu (200 ₫ thường, 300 ₫ xác thực/chuyển khoản/voucher — đo 16/10, chưa VAT); SMS: hàm tạm tính giá của eSMS. **Không** bước duyệt, **không** hạn mức |
| 8 | Gửi | `POST notice-campaigns` ghi đợt + mọi dòng người nhận (`cho-gui`) **một giao dịch** + vết; việc nền comms gửi theo lô (khoá advisory, theo xã), mỗi dòng một lần gọi nhà cung cấp với `RequestId`/`tracking_id` = id dòng (chặn trùng phía nhà cung cấp); kết quả `da-gui` (nhà cung cấp nhận) / `that-bai` (lớp lỗi, không chữ) / `bo-qua`. Đợt: `dang-gui` → `da-gui` / `gui-mot-phan` / `that-bai` |
| 9 | Trạng thái giao | "Đã gửi" = nhà cung cấp đã nhận. Callback eSMS (trạng thái tới máy) — **lượt sau** (tuyến công khai không chữ ký, cần thiết kế riêng theo luật 13) |
| 10 | Tên | Như owns_facts; ghi ở `ubiquitous-language.md`. `zalo-oas/current/*` (chỉ có trên dev) **bỏ** |

## Lệch prototype, có chủ ý

| Prototype | v2 | Vì sao |
|---|---|---|
| `notice_recipients.phone`, `full_name` | Chỉ mã dòng + số đã che | Luật 3; ADR 0097 — danh bạ là chủ duy nhất của số |
| "Ví dụ người nhận" họ tên · số | Không có | Luật 3 |
| Một danh sách `notice_templates` cho cả ZNS và SMS, nhập tay | Mẫu ZBS nạp từ OA, mẫu SMS nạp từ eSMS | ADR 0098 C1; mẫu phải là mẫu nhà cung cấp đã duyệt |
| Một tài khoản eSMS cấu hình cứng | Nhiều tài khoản, chọn khi gửi | Chủ dự án 17/10/2026 |

## Hệ quả

- **Phải dựng:** identity (migration khoá `admin.sms`, RPC người nhận); comms (migration nhiều OA + 4 bảng
  mới, adapter `internal/esms` theo kỷ luật `zalooa`, use case tài khoản SMS, đợt gửi, việc nền gửi); tuyến
  HTTP; web (tab Kênh SMS, danh sách nhiều OA ở Kênh Zalo, màn Gửi tin dùng tuyến thật, ô chọn OA/tài khoản).
- **Mất:** tuyến `zalo-oas/current/*` và mã web đang gọi nó phải đổi; chưa có trạng thái "đã tới máy".
- **Rủi ro nhận:** cán bộ có `notice.send` gửi được tới toàn xã mà không ai duyệt, tốn tiền thật — chủ dự án
  chọn "ước tính, không chặn". Ai trả phí vẫn là việc thương mại ngoài phần mềm.

## Sửa đổi 17/10/2026 — người nhận nhập tay hoặc từ tệp

**Người quyết:** chủ dự án, 17/10/2026, phiên chính (phiếu 4 câu) — *"luồng gửi zns, sms cho phép nhập số
điện thoại trực tiếp hoặc import file nhé, nếu import file cho định nghĩa cột số điện thoại, cột nội dung (với
sms)"*. Mở rộng #5, #6, #8; danh bạ (#5) vẫn là một nguồn như cũ.

| # | Điểm | Chốt |
|---|---|---|
| S1 | Nguồn người nhận | Thêm hai: **nhập số** (gõ/dán, mỗi dòng hoặc dấu phẩy một số, ≤ 1.000) và **nhập tệp** (`.xlsx`/`.csv`, ≤ 10.000 dòng, ≤ 5 MB, sheet đầu, dòng 1 là tiêu đề; cán bộ chọn cột số điện thoại). Số chuẩn hoá về dạng Việt Nam (`0…`, `84…`, `+84…`), bỏ trùng. Người ngoài danh bạ **không** kiểm "ngừng liên hệ" |
| S2 | Tệp | Tệp **không lưu** — chủ dự án: "file import thì có thể xoá sau khi import xong". Trình duyệt đọc tệp, gửi các dòng lên; máy chủ kiểm lại từng dòng |
| S3 | Nội dung SMS (nguồn nhập) | Hai cách: (1) **cột nội dung** trong tệp — mỗi người một nội dung; (2) **nội dung chung** cho cả danh sách, có thể chèn biến `{{variable_1}}`…`{{variable_n}}` (tuỳ chọn); màn đếm các biến trong nội dung và với mỗi biến cho chọn cột tệp tương ứng; có dấu "?" giải thích khi rê chuột. Không kiểm khớp mẫu eSMS trước — dòng sai mẫu do eSMS từ chối, ghi lỗi trả về |
| S4 | Tham số ZNS (nguồn tệp) | Như S3 cách (2): mỗi tham số mẫu ZBS gõ giá trị chung hoặc chọn một cột tệp. Nhập số tay: chỉ giá trị chung |
| S5 | Lịch sử | Lưu lịch sử từng số **kể cả số không hợp lệ** và kết quả gửi. Lượt này 3 kết quả: **số không hợp lệ** (`khong-hop-le`), **gửi thất bại** (`that-bai`, kèm **thông báo lỗi nhà cung cấp trả về** nếu có), **đã gửi** (`da-gui`); "thành công tới máy" để lượt callback (#9). Giá trị enum theo ADR 0011 |
| S6 | Giữ số | Số đầy đủ và nội dung riêng từng dòng chỉ cần tới lúc gửi: lưu **mã hoá bằng khoá của xã** (cùng cơ chế khoá eSMS, ADR 0009) trong dòng người nhận, **xoá bản mã hoá** khi dòng có kết quả cuối; lịch sử chỉ còn số đã che + kết quả + lỗi. Chủ dự án: "chỉ cần lưu lịch sử gửi và kết quả" |
| S8 | Thông tin map biến | Chủ dự án: *"lịch sử nên lưu thông tin map biến"*. Đợt nhập tệp lưu (bất biến) cột số điện thoại, cột nội dung, biến → cột, tham số ZNS → cột, cùng nội dung chung và cách lấy nội dung; màn lịch sử hiện lại. Lưu **tên cột**, không lưu giá trị từng dòng (S6) |
| S7 | Gửi ra ngoài | Nội dung/tham số riêng từng người (từ tệp của cán bộ) được gửi sang eSMS/Zalo — chủ dự án yêu cầu trực tiếp (luật 3 điều kiện dừng #2), thay câu "không tham số nào lấy từ hồ sơ của từng người" của #6 cho riêng hai nguồn nhập |

`.xlsx` đọc ở trình duyệt bằng `read-excel-file` (cùng tác giả `write-excel-file` đang dùng) — giả định của phiên
chính, đổi được.

## Còn mở

| # | Việc | Ai |
|---|---|---|
| 1 | Callback trạng thái giao của eSMS (và của ZBS) | lượt sau |
| 2 | Cấu hình tài khoản eSMS từ platform-admin (ViHAT làm hộ xã) | khi cần |
| 3 | Giờ được gửi (đêm, ngày nghỉ) cho đợt tin — chưa ai nêu | chủ dự án |

→ ADR 0097 · 0098 · 0009 · 0018 · 0090 · 0011 → Luật 1 · 2 · 3 · 5 · 6 · 7 · 8 · 13
