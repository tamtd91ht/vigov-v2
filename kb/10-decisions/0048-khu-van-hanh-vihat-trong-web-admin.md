---
id: 0048-khu-van-hanh-vihat-trong-web-admin
tier: T1
source: CURATED
owner: architecture
derived_from_commit: 6f01382
expires: null
owns_facts:
  - "khu vực vận hành ViHAT dựng trong app riêng platform-admin/ (sửa 27/09/2026, thay lựa chọn gộp vào web-admin), chạy trên host vận hành dành riêng"
  - "vì sao on-premise cho một xã không cần sửa mã khu vận hành: không triển khai platform-admin, hoặc triển khai cho người vận hành tại chỗ"
  - "backend khu vận hành thuộc service-platform; người vận hành chỉ chạm siêu dữ liệu, mọi thao tác ghi ghi vết theo xã ĐÍCH"
  - "tài khoản vận hành ViHAT là một miền tài khoản tách khỏi tài khoản cán bộ xã, không mang tenant_id, không phải vai trò trong một xã"
---

# 0048. Khu vực vận hành ViHAT trong `web-admin`

**Trạng thái:** **đã chốt hướng** (chủ dự án, 27/09/2026 — ba điều kiện ở §*Quyết định*) · mọi phần
ghi *"đề xuất, chờ xác nhận"* **chưa được chốt** · **Nối tiếp** ADR 0003 và ADR 0046
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
| 1 | Tài khoản vận hành nằm ở đâu | Bảng mới ở `service-identity` (identity đã giữ đăng nhập, băm mật khẩu, sổ phiên), **tách bảng** khỏi tài khoản cán bộ. Phương án kia — ở `service-platform` — gom khu vận hành vào một service nhưng nhân đôi mã đăng nhập | Luật 2 bất biến 1 (một chủ sở hữu) — cần khai ở `data-ownership.json` |
| 2 | Đăng nhập thế nào | Phiên/token **miền riêng**: token không mang `tenant_id`, mang một trường miền `van_hanh`; có `sid` tra sổ phiên mỗi yêu cầu (luật 5 bất biến 4). Token cán bộ xã **không bao giờ** được nhận ở tuyến vận hành và ngược lại. Cookie host-only trên host vận hành (luật 1 cấm #3) | Luật 5 bất biến 4, luật 1 bất biến 8 |
| 3 | Mô hình quyền cho người vận hành | Bảng `quyen` là **theo xã** (luật 5 bất biến 3c). Đề xuất: một danh sách khoá **riêng của miền vận hành**, không trộn vào `quyen`. Cần khoá mà `quyen` không có là **phát hiện cho câu mở #27**, không phải một `INSERT` mới | Luật 5 bất biến 3, 3c; câu mở #27 |
| 4 | Biên nhận ra host vận hành ra sao | `core/httpx` hôm nay từ chối mọi host dành riêng (ADR 0046, `59c72be`). Đề xuất: một **lớp biên thứ hai** chỉ ở platform, nhận **đúng một** host đọc từ cấu hình triển khai (hằng số toàn nền tảng — luật 8 bất biến 5), mọi host khác → 404; biên xã giữ nguyên việc từ chối host dành riêng. Tên biến và đường đọc qua `core/config` là **điều kiện dừng #1 của luật 11** | Luật 1 bất biến 3, luật 11 |
| 5 | Liệt kê xã **liên xã** bởi người vận hành | Đọc sổ xã là siêu dữ liệu nền tảng, nhưng vẫn là một truy vấn qua nhiều xã. Đề xuất: đánh dấu `// @cross-tenant: <lý do>` và **ghi vết lần đọc** như một lần đọc liên xã | Luật 1 cấm #6, luật 6 bất biến 7 |
| 6 | Ghi vết theo xã đích | Mục vết ở `audit_log` của platform, `tenant_id` = **xã đích**, cùng giao dịch với thay đổi (luật 6 bất biến 3). "Ai" là **mã nghiệp vụ** của người vận hành (luật 6 bất biến 8) — **định dạng mã chưa chốt**. Tạo xã: xã đích là chính xã vừa tạo | Luật 6 bất biến 2, 3, 8 |
| 7 | Cột `tao_boi` / `cap_nhat_boi` của `ho_so_hien_thi_xa` chú thích *"staff business code"* (`0006…:155`) và bảng *"edited by its staff"* (`:113`) | Hai miền cùng ghi một bảng: người vận hành khai lúc nhận xã, cán bộ xã sửa sau. Mã của hai miền phải **phân biệt được bằng mắt** trong cùng một cột. Ai thắng khi cả hai cùng sửa — **chưa đề xuất**, cần chủ dự án | Luật 6 bất biến 8 |
| 8 | Lưu tệp logo | Hôm nay `logo_url` là URL tới ảnh do hệ khác phục vụ (`0006…:134-137`). Luật *"tệp nghiệp vụ mới là riêng tư"* áp khi có kho tệp. Logo **vốn để công khai** cho công dân xem — đề xuất: đối tượng tiền tố `t:<tenant_id>` (luật 1 bất biến 7), công khai được khai **tường minh** kèm lý do, không phải mặc định | Luật 1 bất biến 7; nguyên tắc *Closed by default* |
| 9 | Sinh QR có phải một lần ghi không | QR in ra sống nhiều năm (ADR 0019). Đề xuất: ghi vết **lần phát hành QR** (tên miền, bản thử hay phát hành, ai) theo xã đích | Luật 6 bất biến 1 |
| 10 | Ai ở ViHAT cầm tài khoản vận hành, có MFA không | **Chưa đề xuất** — quyết định tổ chức của ViHAT. Đề xuất duy nhất: MFA bắt buộc, vì một tài khoản ở đây chạm siêu dữ liệu của **mọi** xã | Luật 8 |
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

## ĐIỀU KIỆN DỪNG

1. Một vai trò, hay tài khoản, có thẩm quyền **vượt một xã** — luật 5 điều kiện dừng #3. Chính
   miền tài khoản vận hành là trường hợp này: hình dạng của nó cần chủ dự án xác nhận (§*Thiết kế*
   #1–#3)
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
