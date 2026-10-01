---
id: 0048-khu-van-hanh-vihat-trong-web-admin
tier: T1
source: CURATED
owner: architecture
derived_from_commit: c21ab453
expires: null
owns_facts:
  - "tài khoản vận hành ở service-identity, bảng riêng (operator_account, operator_session, operator_permission_grant), không tenant_id (chốt 28/09/2026)"
  - "token vận hành mang realm operator và sid tra sổ phiên vận hành; lớp biên thứ hai chỉ ở service-platform nhận đúng một host từ OPERATOR_HOST (tuỳ chọn; vắng → cả khu 404)"
  - "quyền vận hành là danh sách khoá ops.<nhóm>.<việc> của realm operator, không phải dòng trong bảng quyen theo xã"
  - "đăng nhập vận hành bắt buộc MFA: mật khẩu + TOTP, mã khôi phục dùng một lần"
  - "vì sao thay đổi cấu hình toàn nền tảng ghi vết ở platform_audit_log không tenant_id, còn thao tác nhắm một xã ghi ở audit_log của xã đích"
  - "khu vực vận hành ViHAT dựng trong app riêng platform-admin/ (sửa 27/09/2026, thay lựa chọn gộp vào web-admin), chạy trên host vận hành dành riêng"
  - "vì sao on-premise cho một xã không cần sửa mã khu vận hành: không triển khai platform-admin, hoặc triển khai cho người vận hành tại chỗ"
  - "backend khu vận hành thuộc service-platform; người vận hành chỉ chạm siêu dữ liệu, mọi thao tác ghi ghi vết theo xã ĐÍCH"
  - "tài khoản vận hành ViHAT là một miền tài khoản tách khỏi tài khoản cán bộ xã, không mang tenant_id, không phải vai trò trong một xã"
  - "lần người vận hành liệt kê mọi xã không ghi vết — ngoại lệ của luật 6 bất biến 7, chỉ cho siêu dữ liệu xã (người dùng chốt 30/09/2026)"
  - "ho_so_hien_thi_xa: người ghi sau thắng giữa người vận hành và cán bộ xã; mã VH-/CB- trong vết nói miền"
  - "phát hành QR không ghi vết (người dùng chốt 30/09/2026)"
  - "on-premise do ViHAT vận hành từ xa"
  - "tạo xã ở platform-admin không gieo cấu hình nội bộ của xã (vai trò, SLA, giờ làm việc, ngày lễ, cán bộ, phòng ban) — admin xã tự khai (chốt 01/10/2026)"
  - "service-platform kiểm chữ ký token op1. tại chỗ rồi gọi RPC identity tra sổ phiên vận hành mỗi yêu cầu, không cache; RPC vận hành không xã nằm trong methodsWithoutTenant (chốt 01/10/2026)"
  - "giới hạn đăng nhập vận hành 20 lần / 15 phút / IP, cửa sổ cố định trong Redis, vượt → 429 (chốt 01/10/2026)"
  - "phạm vi đợt 1 và đợt 2 của platform-admin; sửa tên xã ở đợt 1 chỉ là sửa lỗi gõ, có lý do và vết (chốt 01/10/2026)"
  - "kênh gRPC platform→identity của khu vận hành chung nợ plaintext-grpc với các kênh gRPC nội cụm và chặn lên sống (chốt 01/10/2026)"
  - "điều kiện lên sống trước khi đặt OPERATOR_HOST: thu hẹp quy tắc 3 NetworkPolicy theo cặp gọi, áp NetworkPolicy trên cụm thật, xong còn mở #12, đủ khoá ở platform-secrets và identity-secrets (chốt 01/10/2026)"
  - "xem chi tiết MỘT xã (siêu dữ liệu sổ xã) không ghi vết — ngoại lệ 30/09 #5 mở từ liệt kê sang liệt kê + xem một xã (chốt 01/10/2026)"
  - "vì sao kiểm trùng tên xã phân biệt dấu nhưng không phân biệt hoa thường và khoảng trắng, so trong cùng tỉnh"
  - "vì sao đăng nhập chỉ có mật khẩu vẫn được chuyển tới identity"
  - "vì sao hành động vết của khu vận hành giữ tên tao_xa / gan_mini_app của stage Jenkins"
---

# 0048. Khu vực vận hành ViHAT trong `web-admin`

> Tiêu đề và tên tệp giữ chữ *"trong `web-admin`"* để không gãy liên kết. Khu vận hành dựng trong
> app riêng **`platform-admin/`** — §*Sửa của chủ dự án — 27/09/2026*.

**Trạng thái:** **đã chốt hướng** (chủ dự án, 27/09/2026 — ba điều kiện ở §*Quyết định*; điều kiện
#1 sửa ở §*Sửa của chủ dự án — 27/09/2026*) · **§Thiết kế #1–#4, #10 đã chốt** 28/09/2026 (§*Chốt
của chủ dự án — 28/09/2026*); #8 trả lời bởi ADR 0052 · #6 chốt 28/09 (§*Chốt bước 1*) · **#5, #7,
#9, #11 chốt 30/09/2026** (§*Trả lời của người dùng — 30/09/2026*) · **đợt 1 `platform-admin` chốt
01/10/2026** (§*Chốt của chủ dự án — 01/10/2026*) · **điều kiện lên sống của `OPERATOR_HOST`
chốt 01/10/2026, sau khi dựng** (§*Chốt bổ sung — 01/10/2026, sau khi dựng*) · #12 là bước vận hành, chưa
làm — mọi phần khác ghi *"đề xuất, chờ xác nhận"* **chưa được chốt** · **Nối tiếp** ADR 0003 và ADR 0046
§*`admin.vigov.vn` — chưa dựng, và cần ADR riêng* · **Thay thế một phần** ADR 0003 §*Hệ quả* (chỉ
điểm ở §*Thay thế gì*; thân ADR 0003 giữ nguyên, chỉ thêm một dòng trỏ có ngày)

## Bối cảnh

Hai giai đoạn làm việc với một xã (ADR 0047 §*Trả lời của chủ dự án — 27/09/2026*, mục 2) đòi
người vận hành ViHAT làm được, **không qua SQL tay**: tạo xã, gắn tên miền, khai hồ sơ hiển thị,
khai dòng `mini_app`, sinh QR cho tên miền xã. Hôm nay không có bề mặt nào cho việc đó:

| Điều | Nơi |
|---|---|
| `admin.vigov.vn` là host dành riêng, **chưa dựng**, không phân giải thành xã | ADR 0046:42, `:89-93` |
| Biên Go từ chối host dành riêng **trước khi tra bảng**, cùng câu trả lời với host lạ | `service-platform/internal/domain/ten_mien_danh_rieng.go:51`, commit `59c72be` |
| Hai dòng cũ `admin.vigov.vn` / `admin-stg.vigov.vn` → Xã Thăng Bình vẫn **còn**, đóng băng | ADR 0046:75-81 |
| Bảng `mini_app` và `ho_so_hien_thi_xa` đã có ở platform, **không dòng mẫu** — *"Rows are entered by an operator"* | `service-platform/migrations/0006_mini_app_va_ho_so_hien_thi.sql:27-29`, `:70`, `:129` |
| `logo_url` là URL, **không** phải byte hay khoá lưu trữ — chưa có `core/storage` | cùng tệp `:134-138` |

Chủ dự án hỏi trước khi chọn: *"liệu có ổn về maintain không. và sau này khi onpremise cho 1 xã thì
có vấn đề gì không, nên tính ổn định về sau"* — rồi chọn **phương án 1** kèm ba điều kiện.

## Quyết định — đã chốt

| # | Điều kiện | Nghĩa |
|---|---|---|
| 1 | **Cùng mã nguồn Next.js `web-admin`**, chỉ phục vụ trên **một host vận hành dành riêng** | Bật/tắt **lúc chạy** theo cấu hình triển khai (luật 1 bất biến 10), **không bao giờ** nướng vào bundle. Host của xã không bao giờ thấy khu này |
| 2 | **Backend ở `service-platform`** | Platform sở hữu sổ xã, tên miền, bảng `mini_app` (`app_id → tenant_id`, chế độ), hồ sơ hiển thị (ADR 0003, 0044, 0045). Mọi thao tác ghi **ghi vết theo xã ĐÍCH**. Người vận hành chỉ chạm **siêu dữ liệu**, không bao giờ chạm dữ liệu nghiệp vụ của xã (ADR 0003, câu mở #3 đã chốt) |
| 3 | **Tài khoản vận hành ViHAT là một miền riêng** | Không mang `tenant_id`. **Không** phải một vai trò "superadmin" trong một xã (luật 5 cấm #2) |

### Vì sao ba điều kiện trả lời câu hỏi bảo trì và on-premise

| Lo ngại | Điều kiện trả lời |
|---|---|
| Hai kho mã cho hai web thì hai bộ thư viện, hai bộ chốt chặn, hai lần nâng cấp | #1: một mã nguồn, một image, một bộ kiểm |
| Một mã nguồn thì dễ trượt thành "một app đổi vai trò" — đúng điều ADR 0003 §*Hệ quả* sợ | #1 + #3: tách bằng **host** và **miền tài khoản**, không bằng cờ vai trò. Không tài khoản nào đi được từ bề mặt này sang bề mặt kia |
| On-premise cho **một** xã | #1: **cùng image**. Không đặt host vận hành → cả khu **404**. Đặt host vận hành → một người vận hành tại chỗ dùng được. **Không sửa mã** |
| Khu vận hành với tay tới dữ liệu nghiệp vụ | #2: backend ở platform, và platform **không có** gRPC client đọc nội dung service nghiệp vụ (ADR 0003 §*Cưỡng chế bằng kiến trúc*) — không có đường, chứ không phải cờ đang tắt |

## Phạm vi của khu

| Việc | Ghi vào | Ghi chú |
|---|---|---|
| Tạo xã | sổ xã của platform | `tenant_id` là ULID sinh mới (luật 1 bất biến 2) |
| Gắn / **trỏ lại** tên miền | `tenant_domain` | Trỏ lại là đường sáp nhập của ADR 0047 câu 4 — **có vết** (ADR 0047 ĐIỀU KIỆN DỪNG #6) |
| Hồ sơ hiển thị: tên, địa chỉ, logo, đường dây nóng, giờ làm việc hiển thị, giới thiệu | `ho_so_hien_thi_xa` (và tên ở sổ xã) | Giờ ở đây **chỉ để hiện** — lịch tính hạn là `lich_lam_viec` của identity (ADR 0007; `0006_mini_app_va_ho_so_hien_thi.sql:146-149`) |
| Dòng `mini_app`: `app_id → tenant_id`, chế độ `chinh` / `rieng` | `mini_app` | Nguồn sự thật duy nhất về xã của một App ID (ADR 0047 §*Tệp dựng không phải nguồn sự thật*) |
| Sinh QR cho tên miền xã | — | Liên kết bản thử hoặc bản phát hành của **app chung**: `https://zalo.me/s/<APP_ID>/?<tham số tên miền>=<tên miền xã>&src=qr`. **Tên tham số chưa chốt** — ADR 0047 CÒN MỞ #5. Dạng liên kết của **bản thử** chưa đo |

## Thiết kế — đề xuất, chờ xác nhận

Mọi dòng dưới đây là **đề xuất của người thiết kế**. Không dòng nào được dựng trước khi chủ dự án
xác nhận.

| # | Câu chưa chốt | Đề xuất | Luật chạm tới |
|---|---|---|---|
| 1 | **Đã chốt 28/09 — xem §*Chốt của chủ dự án — 28/09/2026* #1.** Tài khoản vận hành nằm ở đâu | Bảng mới ở `service-identity` (identity đã giữ đăng nhập, băm mật khẩu, sổ phiên), **tách bảng** khỏi tài khoản cán bộ. Phương án kia — ở `service-platform` — gom khu vận hành vào một service nhưng nhân đôi mã đăng nhập | Luật 2 bất biến 1 (một chủ sở hữu) — cần khai ở `data-ownership.json` |
| 2 | **Đã chốt 28/09 — xem §*Chốt của chủ dự án — 28/09/2026* #2 + #4.** Đăng nhập thế nào | Phiên/token **miền riêng**: token không mang `tenant_id`, mang một trường miền `van_hanh`; có `sid` tra sổ phiên mỗi yêu cầu (luật 5 bất biến 4). Token cán bộ xã **không bao giờ** được nhận ở tuyến vận hành và ngược lại. Cookie host-only trên host vận hành (luật 1 cấm #3) | Luật 5 bất biến 4, luật 1 bất biến 8 |
| 3 | **Đã chốt 28/09 — xem §*Chốt của chủ dự án — 28/09/2026* #3.** Mô hình quyền cho người vận hành | Bảng `quyen` là **theo xã** (luật 5 bất biến 3c). Đề xuất: một danh sách khoá **riêng của miền vận hành**, không trộn vào `quyen`. Cần khoá mà `quyen` không có là **phát hiện cho câu mở #27**, không phải một `INSERT` mới | Luật 5 bất biến 3, 3c; câu mở #27 |
| 4 | **Đã chốt 28/09 — xem §*Chốt của chủ dự án — 28/09/2026* #2 + #4.** Biên nhận ra host vận hành ra sao | `core/httpx` hôm nay từ chối mọi host dành riêng (ADR 0046, `59c72be`). Đề xuất: một **lớp biên thứ hai** chỉ ở platform, nhận **đúng một** host đọc từ cấu hình triển khai (hằng số toàn nền tảng — luật 8 bất biến 5), mọi host khác → 404; biên xã giữ nguyên việc từ chối host dành riêng. Tên biến và đường đọc qua `core/config` là **điều kiện dừng #1 của luật 11** | Luật 1 bất biến 3, luật 11 |
| 5 | Liệt kê xã **liên xã** bởi người vận hành | Đọc sổ xã là siêu dữ liệu nền tảng, nhưng vẫn là một truy vấn qua nhiều xã. Đề xuất: đánh dấu `// @cross-tenant: <lý do>` và **ghi vết lần đọc** như một lần đọc liên xã | Luật 1 cấm #6, luật 6 bất biến 7 |
| 6 | Ghi vết theo xã đích | Mục vết ở `audit_log` của platform, `tenant_id` = **xã đích**, cùng giao dịch với thay đổi (luật 6 bất biến 3). "Ai" là **mã nghiệp vụ** của người vận hành (luật 6 bất biến 8) — **định dạng mã chưa chốt**. Tạo xã: xã đích là chính xã vừa tạo | Luật 6 bất biến 2, 3, 8 |
| 7 | Cột `tao_boi` / `cap_nhat_boi` của `ho_so_hien_thi_xa` chú thích *"staff business code"* (`0006…:155`) và bảng *"edited by its staff"* (`:113`) | Hai miền cùng ghi một bảng: người vận hành khai lúc nhận xã, cán bộ xã sửa sau. Mã của hai miền phải **phân biệt được bằng mắt** trong cùng một cột. Ai thắng khi cả hai cùng sửa — **chưa đề xuất**, cần chủ dự án | Luật 6 bất biến 8 |
| 8 | **Đã trả lời bởi ADR 0052 §2–§3 — xem §*Chốt của chủ dự án — 28/09/2026*.** Lưu tệp logo | Hôm nay `logo_url` là URL tới ảnh do hệ khác phục vụ (`0006…:134-137`). Luật *"tệp nghiệp vụ mới là riêng tư"* áp khi có kho tệp. Logo **vốn để công khai** cho công dân xem — đề xuất: đối tượng tiền tố `t:<tenant_id>` (luật 1 bất biến 7), công khai được khai **tường minh** kèm lý do, không phải mặc định | Luật 1 bất biến 7; nguyên tắc *Closed by default* |
| 9 | Sinh QR có phải một lần ghi không | QR in ra sống nhiều năm (ADR 0019). Đề xuất: ghi vết **lần phát hành QR** (tên miền, bản thử hay phát hành, ai) theo xã đích | Luật 6 bất biến 1 |
| 10 | **MFA đã chốt 28/09 — xem §*Chốt của chủ dự án — 28/09/2026* #10.** Ai ở ViHAT cầm tài khoản vận hành, có MFA không | **Chưa đề xuất** — quyết định tổ chức của ViHAT. Đề xuất duy nhất: MFA bắt buộc, vì một tài khoản ở đây chạm siêu dữ liệu của **mọi** xã | Luật 8 |
| 11 | On-premise: ai vận hành | **Chưa đề xuất** — tuỳ hợp đồng: ViHAT từ xa, hay người của xã. Mã không đổi theo câu trả lời (điều kiện #1); chỉ đổi ai cầm tài khoản | Luật 1 điều kiện dừng #5 nếu là ViHAT |
| 12 | Hai dòng cũ `admin*.vigov.vn` → Xã Thăng Bình | Gỡ theo bước `doi-ten-mien-thang-binh` của ADR 0046:160 **trước khi** host vận hành lên sống, để một host không thuộc cùng lúc hai bề mặt trong sổ | ADR 0046, luật 7 |

## Thay thế gì của ADR cũ

| ADR | Điểm bị thay | Thay bằng |
|---|---|---|
| 0003 | §*Hệ quả*: *"Web quản trị tổng và web quản trị xã là **hai ứng dụng khác nhau**"* | **Một mã nguồn, hai bề mặt** tách bằng host và miền tài khoản (điều kiện #1, #3). Ý của dòng cũ — **không** phải một ứng dụng đổi vai trò — **giữ nguyên** |

**Không thay:** ranh giới siêu dữ liệu của ADR 0003 (bảng *Được / Không được*). Cưỡng chế bằng kiến
trúc. Phiên hỗ trợ do xã cấp là đường **duy nhất** tới dữ liệu thật — chưa dựng. Quy hoạch tên miền
của ADR 0046.

## Hệ quả

- **Dễ hơn:** giai đoạn 1 của một xã (ADR 0047) là thao tác trên màn hình, không phải SQL tay; vết
  có sẵn.
- **Khó hơn:** `web-admin` mang hai bề mặt; mỗi tuyến mới phải khai nó thuộc bề mặt nào. Một tuyến
  vận hành lọt ra host xã là lỗi bảo mật, không phải lỗi giao diện.
- **Chưa dựng:** toàn bộ. Không tuyến, bảng, trang nào của khu vận hành tồn tại ở `6f01382`.

## Sửa của chủ dự án — 27/09/2026, sau khi ADR được viết

Mục này ghi thêm, không sửa phần trên: phần trên là quyết định lúc viết, mục này là câu trả lời sau.

**Điều kiện #1 bị thay.** Lúc chọn phương án 1, bảng so sánh đưa cho chủ dự án **bỏ sót** rằng kho
đã có sẵn app `platform-admin/`: một khung gồm `src/lib/api.ts` và ba thư mục rỗng, được **cố ý**
tách riêng theo ADR 0003 (`platform-admin/README.md`: *"A role can be granted. An application that
was never given the client cannot be granted its way into the data."*). Đưa lại câu hỏi kèm sự
thật ấy, chủ dự án chọn **dựng khu vận hành trong `platform-admin/`**.

| Điều kiện | Nay là |
|---|---|
| #1 | **App riêng `platform-admin/`**, chạy trên host vận hành dành riêng. On-premise cho một xã: không triển khai app này, hoặc triển khai cho người vận hành tại chỗ — **không sửa mã**. App này **không có client** gọi service nghiệp vụ; đó là cơ chế ADR 0003, không phải cờ |
| #2 | Giữ nguyên — backend ở `service-platform`, vết theo xã đích |
| #3 | Giữ nguyên — tài khoản vận hành là miền riêng |

Hệ quả: dòng *"Thay thế gì"* ở trên **không còn hiệu lực** — ADR 0003 §*Hệ quả* ("hai ứng dụng khác
nhau") **đứng nguyên**. Giá bảo trì là hai app Next.js; phần giao diện dùng chung (nếu có) tách thành
gói chung, không chép. Điều kiện dừng #5 đọc là: bật khu vận hành bằng `NEXT_PUBLIC_*` hay hằng số
dựng **trong `web-admin`** là đưa bề mặt vận hành vào app của xã — vẫn cấm.

## Chốt của chủ dự án — 28/09/2026 (§Thiết kế #1–#4, #10)

Mục này ghi thêm, không sửa phần trên. Ở cả năm câu, chủ dự án chọn **đúng đề xuất của người thiết
kế**; bảng dưới ghi hình dạng đã chốt. Tên bảng, trường, khoá viết tiếng Anh theo ADR 0051 — tên
tiếng Việt ở §*Thiết kế* (như miền `van_hanh`) nhường cho tên ở đây. **Chưa dựng gì**: mỗi dòng là
điều phải đúng khi dựng.

| # | Đã chốt | Vì sao | Luật |
|---|---|---|---|
| 1 | Tài khoản vận hành ở **`service-identity`**, **bảng riêng**: `operator_account`, `operator_session`, `operator_permission_grant`. **Không** `tenant_id`. Tách hẳn khỏi tài khoản cán bộ xã — không bảng chung, không cột phân loại. Khai chủ sở hữu bằng `@entity` lúc dựng để `data-ownership.json` sinh ra | identity đã giữ băm mật khẩu và sổ phiên; đặt ở platform là nhân đôi mã đăng nhập. Bảng riêng để không truy vấn nào của cán bộ xã chạm được tài khoản vận hành, và ngược lại | Luật 2 bất biến 1; luật 5 cấm #2 |
| 2 + 4 | **Token miền riêng**: không `tenant_id`, mang realm `operator`, có `sid` tra **sổ phiên vận hành** mỗi yêu cầu. Token cán bộ xã **không bao giờ** được nhận ở tuyến vận hành, và ngược lại (ĐIỀU KIỆN DỪNG #6). Cookie **host-only** trên host vận hành. **Lớp biên thứ hai chỉ ở `service-platform`** (và app `platform-admin`) nhận **đúng một** host, đọc từ biến mới **`OPERATOR_HOST`** — hằng số toàn nền tảng, **tuỳ chọn**: vắng → **cả khu vận hành 404**. Biên xã giữ nguyên việc từ chối host dành riêng. Biến đi qua `core/config` và `.env.example` lúc dựng | Vắng biến = 404 là đường on-premise của điều kiện #1: không đặt thì khu không tồn tại, không sửa mã. Tuỳ chọn chứ không bắt buộc, vì bắt buộc thì mọi triển khai chưa có khu vận hành đều không khởi động được (luật 11 dừng #2). Một host, không danh sách: hai host vận hành là hai bề mặt phải canh | Luật 1 bất biến 3, 8, cấm #3; luật 5 bất biến 4; luật 8 bất biến 5; luật 11 |
| 3 | Quyền vận hành là **danh sách khoá riêng của realm `operator`**, **không** phải dòng trong bảng `quyen` theo xã — luật 5 bất biến 3c không bị chạm. Dạng khoá `ops.<nhóm>.<việc>`. Bộ đầu: `ops.tenant.manage` · `ops.domain.manage` · `ops.profile.manage` · `ops.mini_app.manage` · `ops.upload_policy.manage` · `ops.qr.issue`. Cấp **thẳng cho tài khoản** vận hành (ít người, không cần vai trò). Mọi lần cấp / thu hồi đều **ghi vết** | `quyen` là theo xã; trộn khoá vận hành vào đó là cho một quyền vượt xã chỗ đứng trong sổ của xã. Tiền tố `ops.` để một khoá vận hành không bao giờ trùng tên một khoá xã | Luật 5 bất biến 3, 3c, 5 |
| 10 | **MFA bắt buộc**: mật khẩu + **TOTP** (ứng dụng xác thực), kèm **mã khôi phục dùng một lần**. Không có đăng nhập vận hành nào thiếu yếu tố thứ hai | Một tài khoản ở đây chạm siêu dữ liệu của **mọi** xã; lộ một mật khẩu là lộ sổ tên miền của cả nền tảng | Luật 8 |

Câu *ai ở ViHAT cầm tài khoản* của #10 là quyết định tổ chức của ViHAT, **không** thuộc ADR này.

**Liên quan, chốt cùng ngày (việc giới hạn tải lên, ADR 0052 §10):**

| Điều | Đã chốt |
|---|---|
| Thay đổi cấu hình **toàn nền tảng** (không có xã đích — ví dụ giới hạn tải lên) | Ghi vết ở bảng mới **`platform_audit_log`**, append-only, **không** `tenant_id`. Không bịa một xã giả cho chỗ trống ấy — một `tenant_id` giả là giá trị mặc định trên đường cô lập (luật 1 cấm #1). Migration đang dựng: `service-platform/migrations/0008_upload_policy.sql` (chưa commit ở `301c816`) |
| Thao tác vận hành **nhắm một xã** | Không đổi so với §Thiết kế #6: `audit_log` của platform, `tenant_id` = xã đích, cùng giao dịch |
| **Định dạng mã nghiệp vụ** của người vận hành (§Thiết kế #6) | **Vẫn chưa chốt.** Đề xuất của người thiết kế, **chưa chốt**: một tiền tố khác hẳn mã cán bộ xã, ví dụ `VH-…`, để trong cùng một cột "ai" (cả #7) nhìn là biết miền nào (luật 6 bất biến 8) |

**§Thiết kế #8 (lưu logo)** đã có câu trả lời ở ADR 0052: logo là bản dẫn xuất đã duyệt trong bucket
`vigov-{env}-public` (§2), khoá đối tượng mang tiền tố xã `t_{tenant_id}` chứ không `t:` (§3 — lý
do chính tả ở đó). Đề xuất `t:<tenant_id>` ở dòng #8 đọc theo ADR 0052.

### Còn mở — chưa chốt

| # | Câu | Ghi chú |
|---|---|---|
| 5 | ~~Ghi vết lần **liệt kê xã liên xã** của người vận hành~~ | **Đóng 30/09/2026** — không ghi vết, ngoại lệ có giới hạn, §*Trả lời của người dùng — 30/09/2026* |
| 6 | ~~**Định dạng mã nghiệp vụ** của người vận hành~~ | **Đóng 28/09/2026** — `VH-00001`, §*Chốt bước 1* |
| 7 | ~~Ai thắng khi người vận hành và cán bộ xã cùng sửa `ho_so_hien_thi_xa`~~ | **Đóng 30/09/2026** — người ghi sau thắng, §*Trả lời của người dùng — 30/09/2026* |
| 9 | ~~**Hình dạng vết** của lần phát hành QR~~ | **Đóng 30/09/2026** — không ghi vết, §*Trả lời của người dùng — 30/09/2026* |
| 11 | ~~On-premise: ai vận hành~~ | **Đóng 30/09/2026** — ViHAT từ xa, §*Trả lời của người dùng — 30/09/2026* |
| 12 | Gỡ hai dòng cũ `admin*.vigov.vn` → Xã Thăng Bình | Phải xong **trước khi** host vận hành lên sống |

### Chốt bước 1 — 28/09/2026 (cổng ROUTING §0.3)

Chủ dự án chọn đúng đề xuất ở cả bốn câu. Ghi thêm, không sửa phần trên.

| Câu | Đã chốt | Vì sao |
|---|---|---|
| Còn mở #6 — mã nghiệp vụ | **`VH-` + 5 chữ số** từ một dãy toàn nền tảng (`VH-00001`), cấp một lần, không cấp lại (luật 7 bất biến 3). Câu #6 **đóng** | Nhìn là phân biệt với mã cán bộ xã trong cùng một cột "ai" (luật 6 bất biến 8). Không dùng tên hay email: dữ liệu cá nhân nằm trong vết |
| Ai tạo, ai cấp quyền | **CLI phía máy chủ** trong `service-identity`, devops chạy: tạo tài khoản (mật khẩu tạm in **một lần**, buộc đổi + đăng ký TOTP ở lần đầu), cấp / thu hồi `ops.*`, khoá / mở, đặt lại MFA. Bắt buộc số ticket; vết ghi `actor = system`, ticket ở `reason`. **Không** thêm khoá quản lý người vận hành | Ít người, không cần màn hình; thêm khoá là điều kiện dừng #1. Seed bằng biến môi trường bị bác: một bí mật chung mở quyền trên mọi xã |
| Token và bí mật | Token dạng **`op1.`**, không có trường xã, ký bằng biến mới **`OPERATOR_SESSION_SIGNING_KEYS`** (tách khỏi `SESSION_SIGNING_KEYS`). Bí mật TOTP mã hoá AES-256-GCM bằng biến mới **`OPERATOR_TOTP_ENCRYPTION_KEY`**. Cả hai **tuỳ chọn**: vắng → đăng nhập vận hành từ chối, service vẫn khởi động | Khoá riêng: một bộ kiểm lẫn miền cũng hỏng MAC, không chỉ dựa vào một dòng kiểm. Mã hoá: lộ bản sao lưu DB không lộ yếu tố thứ hai. Câu *"tạm thời không cần mã hoá"* (ADR 0052 §8) nói về tệp trên MinIO, không nói về thông tin đăng nhập |
| Tham số | Phiên **8 giờ** tuyệt đối, không refresh · sai mật khẩu/TOTP **5 lần → khoá 15 phút** (lưu PG) · TOTP 30 s / 6 số / SHA1 / lệch ±1 bước, **chặn dùng lại** mã · **10 mã khôi phục**, băm SHA-256, tạo lại thì huỷ bộ cũ · mật khẩu tối thiểu 12 như cán bộ · đổi mật khẩu / khoá / đổi quyền → thu hồi mọi phiên · vết ở bảng mới **`operator_audit_log`** (identity, không `tenant_id`, chỉ ghi thêm) | Đăng nhập cán bộ hôm nay chưa có khoá sai nhiều lần; miền vận hành chạm mọi xã nên không chờ |

**Sửa cùng ngày, sau rà bảo mật (luật 13, TCVN 14423 §5.5.2.2):** chủ dự án đổi ba con số ở dòng
*Tham số* và thêm hai điều — dòng trên giữ nguyên làm lịch sử, bảng này thắng.

| Điều | Đã chốt | Vì sao |
|---|---|---|
| Thời gian khoá sau 5 lần sai | **12 giờ** (thay 15 phút) + lệnh `operatorctl unlock` (bắt buộc ticket, ghi vết) | TCVN đòi 12 giờ–30 ngày và mở khoá khẩn cấp bởi quản trị. Giá: đoán được email là khoá được người vận hành 12 giờ → tuyến ở bước 2 **phải** giới hạn theo địa chỉ IP, không chỉ theo tài khoản |
| Phiên nhàn rỗi | **5 phút** (thêm vào hạn tuyệt đối 8 giờ) | TCVN: phiên quản trị ≤ 5 phút nhàn rỗi. Không có thì token lộ dùng được suốt 8 giờ |
| Mật khẩu tạm / mã TOTP chờ | **24 giờ / 10 phút**, kiểm bằng SQL. Quá hạn thì devops cấp lại bằng `reset-mfa` | Không hạn thì ai thấy mật khẩu tạm trước người nhận là tự đăng ký ứng dụng xác thực của mình và chiếm tài khoản |
| Vết bắt đầu đăng ký TOTP | Thêm hành động `operator.totp_enrollment_started` | Luật 6 bất biến 1 — thay mã chờ là ghi vào trạng thái thông tin đăng nhập |
| Khoá khi đã có phiên | Khoá do người **cầm phiên** đoán sai (đổi mật khẩu, tạo lại mã khôi phục) → **thu hồi mọi phiên**. Khoá do đăng nhập sai **từ ngoài** → không đăng xuất người thật | Không thì kẻ cầm token lộ vẫn dùng tiếp sau khi bị khoá; còn thu hồi ở nhánh ngoài thì ai cũng đăng xuất được người vận hành bằng cách cố nhập sai |

Bảng thứ tư và thứ năm (`operator_recovery_code`, `operator_audit_log`) thêm vào ba bảng ở #1. Tuyến
gRPC/HTTP **không** thuộc bước 1 — thêm RPC không xã vào `methodsWithoutTenant` là điều kiện dừng
riêng (ADR 0012), hỏi ở bước 2.

### Thứ tự dựng

Mỗi bước là một thẻ việc riêng, **mỗi thẻ qua cổng riêng** (ROUTING §0.3) — chốt ở mục này không
thay cho cổng của từng thẻ.

1. `service-identity`: tài khoản vận hành + TOTP + mã khôi phục + sổ phiên vận hành + cấp quyền
   `ops.*` (và vết cấp / thu hồi)
2. `service-platform`: lớp biên vận hành đọc `OPERATOR_HOST` + middleware xác thực realm `operator`
3. `platform-admin`: màn đăng nhập (mật khẩu + TOTP)
4. Tuyến vận hành đầu tiên: giới hạn tải lên (ADR 0052 §10), quyền `ops.upload_policy.manage`, vết ở
   `platform_audit_log`

## Trả lời của người dùng — 30/09/2026 (§Thiết kế #5, #7, #9, #11)

Mục này ghi thêm, không sửa phần trên. Người dùng trả lời từng câu trong phiên chính 30/09/2026.
**Chưa dựng gì**: mỗi dòng là điều phải đúng khi dựng.

| # | Đã chốt | Giới hạn · hệ quả người dùng chấp nhận | Luật |
|---|---|---|---|
| 5 | Người vận hành liệt kê mọi xã: **KHÔNG ghi vết** từng lần liệt kê. Lý do người dùng nêu: danh sách chỉ mang **tên xã và tên miền**, không mang dữ liệu công dân | **Ngoại lệ tường minh** của luật 6 bất biến 7 (*đọc liên xã cũng phải ghi vết*). Ngoại lệ **chỉ** phủ lần liệt kê **siêu dữ liệu xã** (tên, tên miền, trạng thái) của sổ xã ở platform. **Không bao giờ** phủ một lần đọc dữ liệu nghiệp vụ của xã — việc đó vẫn là ĐIỀU KIỆN DỪNG #2 và #3 ở dưới. Thêm một cột ngoài siêu dữ liệu vào danh sách này là ra khỏi ngoại lệ, phải hỏi lại. Truy vấn **vẫn mang** `// @cross-tenant: <lý do>` (luật 1 cấm #6) — ngoại lệ là về vết, không phải về việc khai | Luật 6 bất biến 7; luật 1 cấm #6 |
| 7 | Người vận hành và cán bộ xã cùng sửa `ho_so_hien_thi_xa`: **người ghi sau thắng**. Mọi lần sửa đều ghi vết, "ai" là mã nghiệp vụ mà **tiền tố nói miền**: người vận hành `VH-…`, cán bộ xã `CB-…` | Không khoá lạc quan: một lần sửa ghi đè lặng lẽ lần sửa kia, giá trị bị đè chỉ còn ở giá trị *trước* của mục vết (luật 6 bất biến 5). Chú thích *"staff business code"* của `tao_boi` / `cap_nhat_boi` (`service-platform/migrations/0006_mini_app_va_ho_so_hien_thi.sql:155`) phải sửa lúc dựng — cột nay nhận mã của cả hai miền | Luật 6 bất biến 5, 8 |
| 9 | Phát hành QR: **KHÔNG ghi vết** như một lần ghi (người dùng: *"không cần"*). Khoá `ops.qr.issue` vẫn là cổng | Một QR in sai trỏ nhầm xã **không truy được ai đã phát hành**. QR in ra sống nhiều năm (ADR 0019) — đó là cái giá người dùng chấp nhận | Luật 6 bất biến 1 |
| 11 | On-premise: **ViHAT vận hành từ xa**. Mã không đổi (điều kiện #1) | Người của ViHAT chạm dữ liệu nghiệp vụ của xã là **luật 1 điều kiện dừng #5** — cần quyết định riêng, câu này không cấp | Luật 1 điều kiện dừng #5 |

**§Thiết kế #12** (gỡ hai dòng `admin*.vigov.vn`) là **bước vận hành**, không phải quyết định — giữ
nguyên ở bảng *Còn mở*, phải xong trước khi host vận hành lên sống.

## Chốt của chủ dự án — 01/10/2026 (cổng ROUTING §0.3, đợt 1 platform-admin)

Mục này ghi thêm, không sửa phần trên. Người dùng trả lời ở cổng §0.3 của đợt 1 trong phiên chính
01/10/2026. **Chưa dựng gì**: mỗi dòng là điều phải đúng khi dựng.

| # | Đã chốt | Vì sao | Giá · hệ quả người dùng chấp nhận | Luật |
|---|---|---|---|---|
| 1 | **Cấu hình nội bộ của xã — vai trò, SLA, giờ làm việc, ngày lễ, cán bộ, phòng ban — là việc của xã, admin xã tự khai.** Tạo xã ở `platform-admin` **không gieo gì** trong số đó. Người dùng: *"đây là nghiệp vụ của xã, vihat không nằm và cũng không được quyết cái này, nên nó là admin xã tự khai"*. Người quản trị đầu tiên của xã **giữ nguyên** như ADR 0046 §*Quyết định 2* | Đúng ranh giới siêu dữ liệu của ADR 0003 (bảng *Được / Không được*) — đây là áp dụng, không phải sửa ADR 0003 | **Không lấy** các hành vi sau của bản mẫu `../vigov-require/apps/platform`: gieo vai trò / danh mục / thời hạn / ngày lễ khi tạo xã (`../vigov-require/apps/platform/src/app/xa/moi/page.tsx:11-14`); tạo admin đầu tiên kèm mật khẩu tạm đưa qua URL (cùng tệp `:136-156`, `:45-47`); đẩy quyền xuống các xã (`../vigov-require/apps/platform/src/app/quyen/page.tsx:9-19`); đọc nhật ký thao tác của mọi xã (`../vigov-require/apps/platform/src/app/nhat-ky/page.tsx:9-15` — ADR 0003:32 cấm); số đếm sử dụng theo xã (`../vigov-require/apps/platform/src/app/xa/[tenantId]/page.tsx:83-99` — ADR 0003:33 chỉ cho số đếm **qua sự kiện**, hôm nay chưa có sự kiện nào). Xã vừa tạo đứng trống cho tới khi admin xã tự khai | ADR 0003; ADR 0046; luật 1 dừng #5 |
| 2 | **Xác thực người vận hành giữa các service: lai, KHÔNG cache.** `service-platform` — biên HTTP duy nhất của khu vận hành, trên `OPERATOR_HOST` — kiểm **chữ ký** token `op1.` tại chỗ trước (nên platform cũng giữ `OPERATOR_SESSION_SIGNING_KEYS`); token đúng chữ ký thì **mỗi yêu cầu** gọi một RPC mới của identity tra **sổ phiên vận hành** (`sid`, nhàn rỗi 5 phút, thu hồi). Các RPC vận hành mới của identity **không mang xã** và được **THÊM vào `methodsWithoutTenant`** — đây là câu trả lời cho điều kiện dừng của ADR 0012 (`0012-grpc-boundary-contract.md:96-98`) mà §*Chốt bước 1* hoãn sang bước 2 | Chỉ kiểm chữ ký thì **gãy** hai điều đã chốt 28/09: nhàn rỗi 5 phút không đo được, và thu hồi phải chờ tới hạn tuyệt đối 8 giờ. Lưu lượng vận hành là vài người ViHAT trên **một** host, không bao giờ là lưu lượng của xã. RPC phiên vận hành không thể biết xã vì người vận hành không thuộc xã nào — qua được phép thử của ADR 0012 | **Đã bác:** chỉ kiểm chữ ký (người dùng nghiêng về nó lúc đầu vì lo tải). Giá: một lời gọi gRPC tới identity mỗi yêu cầu vận hành; identity ngừng thì khu vận hành ngừng (đóng kín, đúng nguyên tắc *fail closed*). Khoá ký vận hành nằm ở hai service — xoay khoá chạm cả hai. **Ghi lại, không quyết ở đây:** nỗi lo tải người dùng nêu thực ra thuộc về kiểm phiên **cán bộ** (`ResolveStaffPrincipal`, không cache, mỗi yêu cầu của cán bộ) — một việc riêng | Luật 5 bất biến 4; luật 2 bất biến 8 (ADR 0012); luật 11 |
| 3 | **Giới hạn đăng nhập vận hành theo IP: 20 lần / 15 phút / IP**, cửa sổ cố định đếm trong Redis, áp cho bước **mật khẩu, TOTP và mã khôi phục** của khu vận hành; vượt → **HTTP 429** | §*Chốt bước 1* (sửa sau rà bảo mật) đã nói: khoá 12 giờ thì đoán được email là khoá được người vận hành 12 giờ, nên tuyến **phải** giới hạn theo IP, không chỉ theo tài khoản | Ngưỡng bảo mật do **người dùng chọn** (luật 13) — đổi con số là điều kiện dừng của luật 13, không phải chỉnh tham số | Luật 13 bất biến 7 |
| 4 | **Phạm vi đợt 1:** biên `OPERATOR_HOST`; đăng nhập (mật khẩu + TOTP + đăng ký lần đầu); liệt kê xã; tạo xã (tên, tỉnh chọn từ danh mục `tinh_thanh`, tên miền chính); thêm tên miền; đặt tên miền chính; bật / tắt hoạt động xã; gắn Mini App riêng của xã (`mini_app`, chế độ `rieng`). **Đợt 2:** hồ sơ hiển thị + logo, sửa giới hạn tải lên, phát hành QR, mã lĩnh vực phản ánh cấp 1, màn App secret (ADR 0066) | Đợt 1 là đủ để mở một xã mới không qua SQL tay (§*Bối cảnh*) | Thứ tự dựng ở §*Chốt bước 1* (bước 4 là giới hạn tải lên) **nhường** cho phạm vi này: giới hạn tải lên sang đợt 2 | — |
| 5 | **Tên miền và tên xã sau khi tạo:** đợt 1 chỉ **thêm** tên miền và **đặt tên miền chính**. **Không** gỡ tên miền, **không** trỏ một tên miền sang xã khác, **không** sáp nhập / chia — vẫn là ĐIỀU KIỆN DỪNG #4, qua `skills/admin-unit-merge`, một đợt riêng. **Sửa tên xã ĐƯỢC** ở đợt 1 nhưng **chỉ là sửa lỗi gõ**: bắt buộc lý do, ghi vết giá trị trước / sau. Người dùng chọn *"Thêm cả sửa tên khi gõ nhầm"* | Sửa lỗi nhập liệu không phải đổi tên đơn vị hành chính — đổi tên thật vẫn là ĐIỀU KIỆN DỪNG #4. Trỏ lại tên miền là đường sáp nhập (ADR 0047 câu 4), nên đi cùng sáp nhập | Hẹp hơn ADR 0003:30 (cho *đổi tên, gán tên miền*) và §*Phạm vi của khu* (*Gắn / trỏ lại*): trỏ lại chưa có màn. Ranh giới "lỗi gõ" với "đổi tên" dựa vào **lý do** người vận hành ghi — máy không phân biệt được | Luật 1 dừng #3; luật 6 bất biến 5; luật 7 bất biến 6 |
| 6 | **Mặc định đã nêu với người dùng, không bị phản đối:** (a) tài khoản vận hành vẫn quản lý bằng CLI `operatorctl` phía máy chủ (§*Chốt bước 1*) — **không** màn quản lý tài khoản; (b) bật / tắt xã do khoá **`ops.tenant.manage`** đã có canh — **không** khoá mới; (c) tuyến vận hành phải **loại khỏi** các bề mặt sinh ra cho xã (Ingress sinh từ openapi và cổng web-admin), chỉ tới được trên `OPERATOR_HOST`; (d) các stage SQL tạm trong Jenkins cho Xã Thăng Bình **giữ** tới khi khu vận hành được kiểm trên prod | (b) thêm khoá là điều kiện dừng #1. (c) một tuyến vận hành lọt ra host xã là ĐIỀU KIỆN DỪNG #6 và là lỗi bảo mật (§*Hệ quả*) | **Còn mở #12** (gỡ hai dòng `admin*.vigov.vn`) vẫn phải xong **trước khi** host vận hành lên sống — đợt 1 không đóng nó | Luật 5 bất biến 3; luật 1 bất biến 3 |

## Chốt bổ sung — 01/10/2026, sau khi dựng

Mục này ghi thêm, không sửa phần trên. Đợt 1 đã dựng ở `a62e2659`; người dùng trả lời các câu nảy
ra **trong lúc dựng**, trong phiên chính 01/10/2026. Tên tài nguyên URL đã dựng cũng được chốt cùng
lượt — ghi ở `kb/00-foundation/ubiquitous-language.md` §*Miền vận hành ViHAT*, không chép lại ở đây.

| # | Đã chốt | Vì sao · giá người dùng chấp nhận | Luật |
|---|---|---|---|
| 1 | **Kênh gRPC mới `platform → identity`** (RPC vận hành của #2 ở mục trên) chạy **không TLS** dù mang mật khẩu, mã TOTP và token `op1.`. Người dùng chọn **"chung nợ hiện có + chặn go-live"**: cùng món nợ `plaintext-grpc` với các kênh gRPC nội cụm đã có (`tools/security_debt.json`, hạn **2026-12-28**), khai bằng `// @security-exception:` trỏ tới sổ nợ ở `core/operatorclient/dial.go` | Kênh này không tệ hơn kênh `identityclient` đang mang token phiên cán bộ; tách một món nợ riêng là hai lời hứa cho một bản sửa. Bản sửa là TLS cho cả ba kênh cùng lúc. **Giá:** tới khi có TLS, biên duy nhất của cổng 9090 là NetworkPolicy — nên dòng #2 là điều kiện cứng | Luật 13 bất biến 1, điều kiện dừng *kênh không mã hoá mới* |
| 2 | **Phát hiện:** quy tắc 11 của `deploy/base/mang/netpol.yaml` (`allow-platform-to-identity-grpc`) hôm nay **không hẹp thêm gì**, vì quy tắc 3 (`allow-grpc-internal`) đã cho **mọi pod** trong namespace vào 9090, và NetworkPolicy chỉ **cộng** quyền. Người dùng chọn: **thu hẹp quy tắc 3 theo đúng cặp gọi thật TRƯỚC khi lên sống**. **Điều kiện để đặt `OPERATOR_HOST`** — đủ cả bốn: **(a)** quy tắc 3 đã thu hẹp theo từng cặp gọi; **(b)** NetworkPolicy đã áp trên cụm thật; **(c)** còn mở #12 (gỡ hai dòng `admin*.vigov.vn`) đã xong; **(d)** `platform-secrets` có `IDENTITY_GRPC_ADDR`, `REDIS_DSN`, `OPERATOR_SESSION_SIGNING_KEYS` (**cùng giá trị** với identity), và `identity-secrets` có `OPERATOR_SESSION_SIGNING_KEYS` + `OPERATOR_TOTP_ENCRYPTION_KEY` | Một quy tắc hẹp nằm cạnh một quy tắc rộng là một rào trông như đang canh mà không canh gì. (d): thiếu biến mà service dùng thì service **từ chối khởi động** (ADR 0057) — và platform không khởi động thì **không xã nào** phân giải được, không riêng khu vận hành | Luật 13 bất biến 1; luật 11 bất biến 8; ADR 0057; ADR 0046 |
| 3 | **Xem chi tiết MỘT xã** (tên, tỉnh, trạng thái, tên miền, App ID Mini App) **không ghi vết**. Ngoại lệ 30/09 #5 mở rộng từ *"liệt kê"* sang *"liệt kê + xem siêu dữ liệu một xã"* | Cùng lý do với #5: chỉ là siêu dữ liệu sổ xã, không dữ liệu công dân. **Giới hạn giữ nguyên:** thêm bất kỳ trường nào ngoài siêu dữ liệu sổ xã vào màn xem là **ra khỏi ngoại lệ**, phải hỏi lại | Luật 6 bất biến 7 |
| 4 | **Ba điều đã dựng, ghi lý do** (mã ở `service-platform/internal/domain/operator_commune.go:124-152`, `internal/http/operator_sessions.go:196-199`, `internal/store/operator_writes.go:47-56`): **(a)** kiểm trùng tên xã **không** phân biệt hoa thường và khoảng trắng nhưng **CÓ** phân biệt dấu, so **trong cùng tỉnh**, bỏ tiền tố *"Thành phố"* / *"Tỉnh"* ở đầu tên tỉnh; **(b)** đăng nhập chỉ có mật khẩu vẫn **chuyển tới identity**; **(c)** hành động vết giữ tên **`tao_xa`** / **`gan_mini_app`** của các stage Jenkins | (a) Gộp dấu thì *"Tân Phú"* và *"Tấn Phú"* thành một tên — hai từ khác nhau, có thể là hai xã, và lời từ chối ở đây **không có đường vượt**. Bỏ tiền tố tỉnh vì `tenant.tinh_thanh` cũ ghi *"Thành phố Đà Nẵng"* còn danh mục ghi *"Đà Nẵng"*; bỏ ở cả hai phía chỉ làm phép kiểm **chặt hơn**. (b) identity trả `ENROLLMENT_REQUIRED` **trước** khi kiểm yếu tố thứ hai; chặn tại chỗ thì tài khoản mới không bao giờ biết mình phải đăng ký. (c) Cùng một hành vi đã có mục vết dưới tên ấy từ stage Jenkins; tên mới là hai tên cho một hành vi trong cùng một cột | (c) luật 6 bất biến 1; ADR 0011 |
| 5 | **Đổi bộ não, người dùng duyệt:** `rbac_guard` nhận các khai báo `opauth.*` (`RequireKey` / `SignedIn` / `Public`) như khai báo quyền hợp lệ; `check_security` theo dõi **hạn** của một `// @security-exception:` có trỏ tới một mục trong sổ nợ | Không có điều thứ nhất, mọi tuyến vận hành bị báo *thiếu khai báo* — báo động giả dạy người ta bỏ qua báo động. Không có điều thứ hai, một ngoại lệ trỏ sổ nợ sống qua hạn của chính món nợ ấy mà cổng vẫn xanh | Luật 5 bất biến 1; luật 13 |

## ĐIỀU KIỆN DỪNG

1. Một vai trò, hay tài khoản, có thẩm quyền **vượt một xã** — luật 5 điều kiện dừng #3. Chính
   miền tài khoản vận hành là trường hợp này: hình dạng của nó cần chủ dự án xác nhận (§*Thiết kế*
   #1–#3). **28/09/2026: #1–#3 đã chốt** — xem §*Chốt của chủ dự án — 28/09/2026*; điều kiện này còn
   áp cho mọi mở rộng thẩm quyền vượt các khoá `ops.*` đã chốt
2. Người vận hành cần chạm **dữ liệu nghiệp vụ** của một xã — luật 1 điều kiện dừng #5; ADR 0003
   chỉ cho phiên hỗ trợ do xã cấp
3. Một màn đọc dữ liệu **qua nhiều xã** ngoài sổ xã — luật 1 điều kiện dừng #2 (báo cáo huyện/tỉnh
   không thuộc khu này)
4. **Sáp nhập / chia / đổi tên** một đơn vị hành chính qua khu này — luật 1 điều kiện dừng #3,
   `skills/admin-unit-merge`
5. Đề xuất bật khu vận hành bằng biến `NEXT_PUBLIC_*` hay hằng số dựng — vi phạm điều kiện #1
6. Một token cán bộ xã được nhận ở tuyến vận hành, hoặc token vận hành ở tuyến xã

→ ADR 0003 (chỉ siêu dữ liệu): `kb/10-decisions/0003-platform-admin-metadata-only.md`
→ ADR 0046 (host dành riêng, `admin.vigov.vn`) · ADR 0047 (hai giai đoạn, trỏ lại tên miền, QR)
→ ADR 0044 · 0045 (bảng `mini_app`, hồ sơ hiển thị) · ADR 0043 (web-admin là cổng API)
→ Câu mở #3, #25, #27: `kb/00-foundation/open-questions.json`
