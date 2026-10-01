---
id: 0066-dang-nhap-app-rieng-cua-xa-o-vigov
tier: T1
source: CURATED
owner: architecture
derived_from_commit: e8cede77
expires: null
owns_facts:
  - "đăng nhập app riêng của xã (đổi accessToken/phoneToken bằng secret của App ID ấy) do ViGov service-identity làm; vihat-miniapp chỉ còn app chung/demo của ViHAT"
  - "vòng đời một xã: demo trên app ViHAT với tên miền demo dùng chung → app riêng của xã, đứng tên dev ViHAT, OA xác thực là OA của xã"
  - "hai nghĩa của cờ --demo: app ViHAT demo cho xã mới (qua vihat-miniapp) và app riêng đang chờ Zalo duyệt (qua ViGov)"
---

# 0066. Đăng nhập app riêng của xã chuyển vào ViGov

**Trạng thái:** đã chốt · **Ngày:** 2026-10-01 · **Thay một phần ADR 0032** (phần "lượt đổi token Mini
App thuộc `vihat-miniapp`", chỉ với app riêng của xã)

## Bối cảnh

Đến 30/09/2026, MỌI lượt đăng nhập công dân, kể cả app riêng của xã (`deploy.mjs --vao-thang`), đi
`App → vihat-miniapp → ViGov`. Lý do nằm ở bảng của CLAUDE.md và ADR 0032: *"bề mặt Zalo thuộc kho
nào do secret nào ký quyết định"*. Lượt đổi `accessToken`/`phoneToken` ký bằng app secret của bên
đứng tên app, nên thuộc `vihat-miniapp`. Quy tắc ấy viết khi Mini App là **một** app của ViHAT.

Người dùng mô tả lại vòng đời thật của một xã (01/10/2026), và đặt câu hỏi: *"sau khi build app riêng
cho xã A rồi mà vẫn phải qua vihat-miniapp, đây là chỗ tôi khó hiểu nhất"*. Các câu người dùng trả lời
trong lượt ấy:

| Câu | Trả lời của người dùng |
|---|---|
| App A đứng tên ai trên Zalo | Tài khoản dev của ViHAT, vì mã nguồn ở ViHAT; có thể mời xã A cùng vào |
| OA xác thực của App A | OA của xã A |
| Dữ liệu lúc demo | *"Không quan tâm"* (chỉ nằm trên web admin, người dân chỉ thấy dữ liệu của mình); việc xoá xét sau bằng cờ |
| Tên miền demo | Một tên miền demo dùng chung là được; *"bản chất chỉ thay đổi view như logo, name, banner thôi"* |
| `--demo` | Hai nghĩa: (1) app ViHAT demo cho xã mới; (2) app xã A **đang chờ Zalo duyệt**, vẫn phải vào xem để sửa lỗi |

Hai sự thật kỹ thuật làm rõ câu hỏi:

- **App secret theo từng App ID**, không theo đơn vị đứng tên. Đơn vị đứng tên chỉ quyết ai vào được
  trang quản lý để lấy secret và ai chịu trách nhiệm giữ nó.
- Điện thoại **không được** giữ secret (luật 8). Máy chủ nào cầm secret App A thì máy chủ ấy đổi mã
  được. Việc App A phải qua `vihat-miniapp` là **lựa chọn nơi giữ secret**, không phải giới hạn kỹ thuật.

Câu người dùng đặt để chọn (01/10/2026): *"nếu đứng ở góc độ vận hành lâu dài, và tính đến việc
customize cho app A khác với app chung, và việc bảo trì maintain, hoặc 1 dự án onpremise cho app A mà
không qua cloud của vihat thì cách nào là có lợi nhất"*.

## Phương án

| Phương án | Được | Mất |
|---|---|---|
| **A. Giữ:** mọi app đăng nhập qua `vihat-miniapp` | Không dựng gì mới; một chỗ giữ mọi secret Zalo | **On-premise không tự đứng được:** ViGov cài tại xã vẫn phải gọi ra cloud ViHAT, hoặc phải cài thêm kho thương mại. Tuỳ biến App A phải sửa hai kho. Hai kho, hai lần triển khai, một cầu nối phải giữ khớp (cổng 9091, khoá cầu, netpol) — lỗi cầu thì pod vẫn xanh (bàn giao 01/10 §5). `vihat-miniapp` sập là mọi xã không đăng nhập được |
| **B. Chuyển:** app riêng đăng nhập thẳng ViGov; `vihat-miniapp` chỉ giữ app chung/demo | Một bộ ViGov tự đứng được, kể cả on-premise. Tuỳ biến App A theo cấu hình của xã. Một kho, bớt cầu nối cho app riêng. Sự cố không lan giữa app chung và app riêng | Dựng lại phần đổi mã Zalo trong `service-identity` (hai lời gọi, đã có mẫu ở `vihat-miniapp`). ViGov giữ secret từng App A theo xã, có quy trình xoay vòng. Mở đường ra Zalo từ `service-identity`. Hai bên cùng có mã đổi mã Zalo |

## Quyết định

**Phương án B.** Người dùng, 01/10/2026: *"ok theo hướng này đi, chuyển vào vigov, lưu ý quan trọng app
vihat vẫn dùng demo được (dùng domain demo)"*.

| Bề mặt | Kho | Ghi chú |
|---|---|---|
| **App riêng của xã**: đổi `accessToken`/`phoneToken` bằng secret của **App ID ấy**, mở phiên công dân | **ViGov, `service-identity`** | App ID và secret là cấu hình **theo từng xã**, đọc lúc chạy (luật 1 bất biến 10), secret ở kho bí mật (luật 8) |
| **App chung / demo của ViHAT**: đổi token, webhook, QR | `vihat-miniapp` | Như cũ. Demo cho xã mới chạy trên app này với **tên miền demo dùng chung** |
| ZNS từ OA từng xã | ViGov, `service-comms` | Không đổi (ADR 0018) |
| OA xác thực của app riêng | OA **của xã** | Cấu hình trên console Zalo, không phải mã. Bổ sung cho ADR 0031, vốn nói về OA xác thực của app chung |

**Vòng đời một xã:**

| Giai đoạn | App | Đăng nhập qua | Dữ liệu |
|---|---|---|---|
| 1. Demo cho xã mới | App ViHAT + QR gắn **tên miền demo dùng chung** (xã demo); phần nhìn đổi theo xã được chào (logo, tên, banner) | `vihat-miniapp` | Vào xã demo |
| 2. Chờ Zalo duyệt app riêng | App A dựng với `--vao-thang --demo` | **ViGov** | Vào xã A |
| 3. Phát hành | App A dựng `--vao-thang` (không `--demo`) | **ViGov** | Vào xã A |

**Hai nghĩa của `--demo`, cùng một cơ chế:** danh tính cố định, không xin quyền Zalo (app chưa duyệt
thì chưa có quyền số điện thoại), mọi luồng chạy thật tới máy chủ (ADR 0047 §6 dòng "Cờ --demo làm
lại"). Chỗ nhận danh tính cố định đi theo nơi đăng nhập: app ViHAT → `vihat-miniapp`; app riêng → ViGov,
bật theo App ID trong cấu hình của xã, **mặc định tắt**.

## Hệ quả

**Dễ hơn:**
- Triển khai on-premise cho một xã chỉ cần bộ ViGov.
- Thay đổi luồng đăng nhập, quyền hay giao diện của app riêng chỉ sửa một kho.
- `vihat-miniapp` sập không kéo app riêng theo.

**Khó hơn — phải trả:**
- **Hai nơi có mã đổi mã Zalo** (`vihat-miniapp` cho app chung, `service-identity` cho app riêng). Zalo
  đổi API thì sửa hai chỗ.
- **ViGov giữ bí mật của bên đứng tên app (ViHAT).** Cần quy định ai được xoay secret, và với
  on-premise thì secret nằm trên hạ tầng của xã.
- **Đường ra Zalo** từ `service-identity` (netpol, cho phép tên miền Zalo).
- Danh tính cố định của `--demo` ở ViGov là **đường mở phiên không qua xác minh số điện thoại**. Chỉ bật
  theo App ID trong cấu hình xã, mặc định tắt, và phải tắt trước khi app được duyệt. Việc sửa
  `vihat-miniapp` cho `DEMO_APP_IDS` (app ViHAT) bị bộ phân loại an toàn của Claude Code chặn ngày
  01/10/2026 — loại thay đổi này cần chủ dự án duyệt quyền.
- Tên xã hiển thị trên bản demo dùng tên miền chung cần một tham số dựng riêng; tên xã của dữ liệu vẫn
  là xã demo.

**Văn bản phải sửa theo:** bảng *"Which repo a Zalo surface belongs to"* trong CLAUDE.md (chia theo
app); ADR 0032 đánh dấu bị thay một phần bởi ADR này. ADR 0045 và 0047 §6 mô tả luồng app riêng qua
`vihat-miniapp`: dòng ở ADR này thắng khi nói khác.

## Đã quyết 01/10/2026 — trả lời ba câu dưới (người dùng: *"ok theo đề xuất nhé, làm đi"*)

| Câu | Chốt |
|---|---|
| Nơi giữ secret | Bảng mới thuộc `service-identity`: App ID, secret **mã hoá AES-GCM**, ngày đặt, người đặt. Khoá giải mã là **một** bí mật nền tảng qua Secret k8s (luật 8 đk dừng #1 — người dùng đồng ý). Không để secret từng xã trong biến môi trường (luật 1 bất biến 10) |
| Ai nhập, xoay secret | Vận hành ViHAT ở `platform-admin`, cùng chỗ khai App ID của xã. Thay = ghi bản mới, vết kiểm toán không chứa giá trị. Xã on-premise: người vận hành của xã |
| Cầu 9091 | Giữ song song tới khi app riêng Thăng Bình chạy đường mới, rồi gỡ phần app riêng khỏi cầu; app chung vẫn dùng |
| `--demo` của app riêng | Cột bật danh tính cố định theo App ID, **mặc định tắt**; bật thì identity nhận đăng nhập không `phoneToken`, gán số cố định; tắt trước khi Zalo duyệt |
| Thứ tự | Phần không đụng `service-identity` làm trước; tuyến identity đợi phiên song song đang sửa `service-identity` commit xong |

## Đã quyết 01/10/2026 — sáu câu dựng tuyến (người dùng: *"ok"*, rồi *"ok không dùng đường ẩn danh nữa mà focus vào --demo option đi"*)

| # | Chốt |
|---|---|
| 1 | Tuyến công khai `POST /api/v1/citizen-sessions` trên `identity.api.vigov.vn`, `Public("app riêng của xã đổi accessToken/phoneToken lấy phiên công dân trước khi có phiên — ADR 0066")`. Thân và bảng mã trạng thái **giữ y như `vihat-miniapp`** (201 `{vigovSession}` · 400 · 401 · 422 · 429 · 502 · 503) |
| 2 | Giới hạn tần suất dựng ngay trong tuyến: **10 lượt / 5 phút / IP**, đếm trong bộ nhớ từng pod (ngưỡng an ninh — người dùng duyệt) |
| 3 | Khi `platform-admin` chưa có: nhập/xoay secret và bật/tắt demo theo App ID bằng `operatorctl` — bắt `--ticket`, đọc secret từ stdin, vết kiểm toán ở xã đích, không ghi giá trị |
| 4 | Về sau `platform-admin` gọi thẳng một tuyến vận hành ở identity, không vòng qua `service-platform` |
| 5 | Đổi mã vị trí (`getLocation`) của app riêng **cũng chuyển vào identity**, để on-premise tự đứng được |
| 6 | Mã hoá secret bằng `SECRET_ENCRYPTION_KEYS` + `core/crypto` (khoá theo xã, ADR 0009); identity bắt buộc biến này ở prod — thêm vào `identity-secrets` trước khi triển khai |

**Bản `--demo` không gọi lệnh Zalo nào**, kể cả `getAccessToken` (app chưa duyệt bị Zalo từ chối cả lệnh ấy — người dùng báo 01/10/2026). Thân demo là `{appId, demoIdentity: true}`; identity nhận khi App ID bật demo, danh tính cố định, không mã tài khoản Zalo. **Không** làm đường "gửi ẩn danh không phiên" (người dùng bỏ, 01/10/2026).

## Chưa quyết

1. Nơi giữ secret App A theo xã: bảng cấu hình mã hoá trong `service-identity`, hay Secret k8s theo xã
   — cần chọn trước khi dựng, theo luật 8 và `skills/infra-config`.
2. Ai được xoay secret App A, nhất là với xã on-premise.
3. Cầu phiên `vihat-miniapp → identity` (cổng 9091) còn dùng cho app chung; app riêng không còn đi qua.
   Có gỡ phần app riêng của cầu ngay hay để một thời gian song song.
