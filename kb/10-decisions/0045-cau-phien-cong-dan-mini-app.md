---
id: 0045-cau-phien-cong-dan-mini-app
tier: T1
source: CURATED
owner: architecture
derived_from_commit: c38f040
expires: null
owns_facts:
  - "cầu phiên vihat-miniapp → service-identity: lời gọi máy-chủ-với-máy-chủ bằng khoá riêng của cặp này, trên cổng riêng, ViGov phát phiên công dân"
  - "phiên công dân của app chính khi mở lại không QR: máy chủ tra xã đã xác nhận theo tài khoản Zalo, không có thì phiên không xã"
  - "đổi xã ở app chính chỉ bằng QR xã khác + xác nhận; không có nút Đổi xã"
  - "phiên chưa có số điện thoại: được xem, không được đọc hay ghi hồ sơ của chính mình"
  - "vì sao vihat-miniapp không bao giờ cầm GRPC_CALLER_KEY"
---

# 0045. Cầu phiên công dân: `vihat-miniapp` → `service-identity`

**Trạng thái:** đã chốt (năm quyết định của chủ dự án, 25/09/2026) · phần ghi *"đề xuất của
người thiết kế"* **chờ xác nhận** · **Thực thi ADR 0044** · ADR 0005, 0019, 0022, 0025, 0032
giữ nguyên

## Bối cảnh

Sổ tiến độ `citizen-app/cau-phien-cong-dan-vigov` ghi lỗ hổng: công dân đăng nhập Mini App qua
`vihat-miniapp` nhưng **không có đường nào ra phiên công dân của ViGov**. Mọi tuyến
`CitizenOnly` không công dân thật nào gọi được.

Hiện trạng đo được ở kho `vihat-miniapp` (chỉ đọc):

| Điều | Nơi |
|---|---|
| `POST /api/v1/sessions` **bắt buộc cả hai** `accessToken` và `phoneToken` | `internal/httpapi/sessions.go:58` |
| Nó tự sinh token **của riêng nó** và lưu phiên kèm số điện thoại vào CSDL của nó | `sessions.go:79`, `:88` |
| Lời gọi Zalo duy nhất là `GET /v2.0/me/info`, trả **chỉ số điện thoại** | `internal/zalo/wire.go:59`, `:75` |
| Chỉ có **một** App ID và **một** secret | `internal/config/config.go:33-34`, `:69-70` |

Phía ViGov có sẵn sổ phiên `phien_cong_dan` và RPC `ResolveCitizenSession`, nhưng chỉ test
gọi `Tao`/`TaoChuaChonXa` (`service-identity/internal/store/phien_cong_dan.go:354`, `:374`).

ADR 0044 cần thêm hai điều mà hiện trạng không có: **App ID đã xác minh** đi kèm phiên, và
**danh tính trước khi có số điện thoại** để nhớ xã ở app chính.

## Quyết định của chủ dự án — 25/09/2026, không mở lại

| # | Câu | Trả lời |
|---|---|---|
| 1 | Hai kho tin nhau bằng gì | **Máy chủ gọi máy chủ.** `vihat-miniapp` xác minh token Zalo bằng secret của đúng app, rồi gọi `service-identity` qua mạng cụm bằng **khoá dịch vụ riêng cho cặp này** (không phải app secret). ViGov phát phiên và trả về. Client không bao giờ cầm lời khẳng định |
| 2 | App chính mở lại không QR | Lấy `accessToken` âm thầm (không xin quyền số điện thoại). Máy chủ tra xã công dân đã xác nhận → phiên có xã ấy; không có → phiên không xã (chỉ giới thiệu). Số điện thoại **chỉ xin khi gửi** |
| 3 | App chính đã ở xã A, quét QR xã B | Hiện màn xác nhận. Đồng ý → **phiên MỚI cho B**, ghi vết; xã được nhớ thành B. **Không có nút "Đổi xã" ở đâu cả** |
| 4 | App riêng | Xã = tra `app_id → tenant_id` (bảng của `service-platform`, ADR 0044). Không khớp hoặc xã không hoạt động → **từ chối**, không mặc định |
| 5 | Hồ sơ hiển thị theo xã (địa chỉ, logo, đường dây nóng, giờ làm việc, giới thiệu) | Thuộc `service-platform`, cán bộ xã sửa trên web-admin (việc riêng). Phiên chỉ mang tên xã |

**Mặc định do phiên chính chọn** (ghi rõ là mặc định, không phải lời chủ dự án):

- Liên kết trong ZNS trỏ tới **app riêng của xã nếu có**, nếu không thì app chính kèm `t`.
- ADR 0019 (QR ghép phiên) **không đổi**.
- App Zalo cho cán bộ **ngoài phạm vi**.

## Luồng

| # | Bước | Ở đâu |
|---|---|---|
| 1 | Mini App gọi `getAccessToken()` âm thầm; khi gửi hồ sơ thì thêm `getPhoneNumber()` | `citizen-app` |
| 2 | Gửi `accessToken` (+ `phoneToken` nếu có) + `t` và cờ "đã xác nhận" (nếu có) tới `POST /api/v1/sessions` | `citizen-app` → `vihat-miniapp` |
| 3 | Chọn secret của đúng app, đổi token tại `graph.zalo.me`, lấy **mã tài khoản Zalo** và (nếu có `phoneToken`) số điện thoại | `vihat-miniapp` — xem UNKNOWN #1, #2 |
| 4 | Gọi `OpenCitizenSession` trên cổng cầu, kèm khoá cầu | `vihat-miniapp` → `service-identity` |
| 5 | `ResolveMiniApp(app_id)` → chế độ + xã (nếu app riêng) | `service-identity` → `service-platform` |
| 6 | Chọn xã theo bảng §Chế độ; nếu có xã thì `GetTenant` với xã ấy trong context để chắc xã còn hoạt động | `service-identity` → `service-platform` |
| 7 | **Một giao dịch**: tài khoản Zalo + xã đã nhớ, danh tính theo số (nếu có), phiên mới, vết | `service-identity` |
| 8 | Trả token phiên ViGov + tên xã; `vihat-miniapp` chuyển nguyên cho client, **không lưu, không log** | `vihat-miniapp` → `citizen-app` |
| 9 | Mọi lời gọi sau đó: `Authorization: Bearer` thẳng tới API ViGov, rìa công dân của ADR 0022 | `citizen-app` → ViGov |

`vihat-miniapp` không bao giờ biết xã. Nó chỉ khẳng định *"token này thuộc app X, tài khoản Zalo
Y, (số Z)"*. Đúng vai ADR 0044 giao.

## Tin cậy giữa hai kho — vì sao một khoá riêng, trên một cổng riêng

| Phương án | Loại vì |
|---|---|
| Đưa `GRPC_CALLER_KEY` cho `vihat-miniapp` | Khoá ấy mở **mọi RPC của mọi service** (ADR 0025, *"không mua được"* #2). Một hệ thống thương mại ngoài ViGov cầm nó là cầm luôn `ResolveStaffPrincipal`, `ResolveStaffNames`… |
| Cùng cổng 9090, interceptor nhận thêm khoá thứ hai cho riêng RPC cầu | Đúng thứ ADR 0025 ĐIỀU KIỆN DỪNG #3 cấm trên ranh giới ấy: *"một header thứ hai"*. Và giới hạn "khoá cầu chỉ mở một RPC" thành một danh sách phải có người giữ đúng |
| Client mang một lời khẳng định đã ký tới ViGov | Chủ dự án loại ở câu 1 |
| **Cổng gRPC riêng của `service-identity`, chỉ phục vụ `CitizenSessionBridgeService`, kiểm khoá cầu** — đã chọn | "Khoá cầu mở đúng một RPC" là **tính chất của cách nối dây**, không phải một danh sách. ADR 0025 đứng nguyên: nó quản ranh giới giữa các service ViGov, còn đây là một ranh giới khác với một bên gọi không thuộc ViGov |

Khoá cầu **không nói ai là công dân**. Nó chỉ nói *"bên gọi là `vihat-miniapp`"*. Mọi thứ trong
thân yêu cầu (`app_id`, `zalo_user_id`, `verified_phone`, `client_ip`) là **lời khai của
`vihat-miniapp`**, và ViGov tin chúng đúng bằng mức tin `vihat-miniapp` — một pháp nhân, cùng
ViHAT Group (câu hỏi mở #28, đã chốt). Ghi vậy để không ai đọc nhầm khoá cầu thành bằng chứng về
công dân.

**Lớp mạng là nửa kia của cơ chế**, như ADR 0025: NetworkPolicy chỉ cho pod `vihat-miniapp` tới
đúng cổng cầu của `identity`; cổng cầu không qua ingress, không qua cổng `web-admin` (ADR 0043).
Nếu `vihat-miniapp` **không cùng cụm** thì chặng này ra khỏi mạng cụm và mang token phiên công dân
ở dạng rõ → cần TLS → **ĐIỀU KIỆN DỪNG** (UNKNOWN #5).

### Cấu hình — chỉ đặt tên vai trò, KHÔNG viết mã cấu hình trong việc này

Theo luật 8 và luật 11. Tên nói **vai trò**, không nói cụm.

| Biến (env / Go) | Khoá ConfigMap/Secret | Nguồn | Bắt buộc? | Ở đâu |
|---|---|---|---|---|
| `CITIZEN_SESSION_BRIDGE_KEYS` | `CITIZEN-SESSION-BRIDGE-KEYS` | **Secret**, `secret.Secret` | Tuỳ chọn — *đề xuất*: trống thì cổng cầu **không mở**, identity vẫn phục vụ cán bộ. Lý do: thiếu nó thì không phục vụ nổi *một luồng*, không phải *mọi yêu cầu* (luật 11 bất biến 8) | `service-identity` |
| `CITIZEN_SESSION_BRIDGE_LISTEN_ADDR` | `CITIZEN-SESSION-BRIDGE-LISTEN-ADDR` | ConfigMap | Tuỳ chọn. *Đề xuất*: đặt địa chỉ mà thiếu khoá → **từ chối khởi động** (nửa cấu hình là hỏng, không phải nửa bật) | `service-identity` |

Dòng `.env.example` cho cả hai, **giá trị giữ chỗ**, thêm cùng lúc với mã cấu hình (luật 11 bất
biến 6) — không phải trong việc này. Phía `vihat-miniapp` cần địa chỉ cổng cầu và **một** giá trị
khoá; đặt tên là việc của kho ấy.

**Xoay khoá — đề xuất, chờ xác nhận.** Biến mang **danh sách**: máy chủ nhận bất kỳ khoá nào trong
danh sách, `vihat-miniapp` gửi một. Khác hình dạng một-giá-trị của ADR 0025, vì hai kho **triển
khai độc lập**: một giá trị nghĩa là xoay khoá phải phát hành đồng thời hai kho của hai nhóm, và
trong khoảng lệch **mọi lần đăng nhập công dân hỏng**. Hình dạng danh sách đã có sẵn cạnh đó cho
khoá ký phiên (`core/config/config.go:534`, `khoaKy`; ADR 0025 phương án D). **Vòng đời khoá** (bao lâu xoay)
chưa ai chốt — luật 8 bất biến 6 đòi con số ấy.

## Chế độ và xã của phiên

| Chế độ app (`ResolveMiniApp`) | Điều kiện | Xã của phiên | Xã đã nhớ |
|---|---|---|---|
| — | `app_id` không đăng ký | **Từ chối** `FAILED_PRECONDITION` | — |
| Riêng | Xã gắn app đang hoạt động | Xã ấy. `t` **bị bỏ qua** | Không dùng |
| Riêng | Xã gắn app không hoạt động | **Từ chối.** *Đề xuất*: **không** tự đi theo `tenant_succession` | — |
| Chính | `t` + công dân **đã xác nhận** | `t` — phải đang hoạt động, không thì từ chối | Thành `t`; khác xã cũ thì ghi vết `doi_xa` kèm xã cũ |
| Chính | Không xác nhận (có hay không có `t`) | Xã đã nhớ nếu còn hoạt động, không thì `""` | Không đổi |

**Vì sao app riêng không tự đi theo xã kế thừa (đề xuất):** app riêng mang tên xã A trên kho Zalo.
Tự chuyển công dân sang xã X là để họ làm việc với một cơ quan khác dưới một cái tên họ không thấy
— đúng thứ ADR 0044 câu 3 cấm. Gắn lại app sau sáp nhập là việc của người vận hành, có vết.

**Vì sao xã đã nhớ mà không còn hoạt động thì về `""` chứ không từ chối (đề xuất):** công dân vẫn
mở được app và thấy giới thiệu; quét QR mới là vào xã mới. `""` không phải xã mặc định — nó là
*"không xã nào"*, đúng nghĩa `phien_cong_dan.tenant_id = ''` (migration 0004, `:317-322`).

**Xác nhận nằm ở client, quyết định nằm ở máy chủ.** Client đã biết mình đang ở xã A (phiên) và
QR nói `t=B`, nên nó tự hiện màn *"Chuyển sang xã B?"* — tên B đọc qua đường `KhongThuocXa` phân
giải QR (ADR 0022, gồm cả xã kế thừa nếu B đã sáp nhập). Máy chủ chỉ đổi xã khi nhận
`commune_confirmed = true`. Mở bằng QR mà chưa xác nhận thì phiên theo xã đã nhớ (hoặc `""`):
lần mở QR đầu tiên tốn **hai** lượt gọi. Đó là giá của việc QR không tự đưa ai vào xã nào.

**Phiên xã A cũ không bị thu hồi** bởi luồng này: yêu cầu không mang token cũ. Nó hết hạn theo
TTL. Muốn thu hồi thì thêm một trường tuỳ chọn — CÒN MỞ #3.

## Phiên chưa có số điện thoại — hệ quả bắt buộc của câu 2

Câu 2 tạo ra một thứ hệ thống chưa có: **phiên có xã nhưng chưa có số điện thoại**. Hiện trạng
không biểu diễn được nó:

| Chỗ chặn | Nơi |
|---|---|
| `phien_cong_dan.cong_dan_id NOT NULL REFERENCES dinh_danh_cong_dan` | `service-identity/migrations/0004_kenh_cong_dan.sql:354` |
| `dinh_danh_cong_dan.so_dien_thoai NOT NULL` | `0004_kenh_cong_dan.sql:101` |
| `ResolveCitizenSession` coi `citizen_id` rỗng là lỗi hợp đồng → `Internal` | `service-identity/internal/grpc/phien_cong_dan.go:108` |
| `core/identityclient` coi như vậy | `core/identityclient/phien_cong_dan.go:172` |

**Hướng đã chọn — đề xuất của người thiết kế, chờ xác nhận vì nó đổi mô hình tin:**

1. Thực thể mới **tài khoản Zalo** (`@entity: ZaloAccount`, `@scope: cross-tenant`, thuộc
   `service-identity`): khoá **(`app_id`, `zalo_user_id`)**, `cong_dan_id` **có thể rỗng**, xã đã
   nhớ (chỉ app chính). Không có `tenant_id` vì cùng lý do `dinh_danh_cong_dan` không có: nó là
   thứ trả lời câu *"xã nào"*.
2. Danh tính công dân **vẫn neo vào số điện thoại** (ADR 0002 đứng nguyên). Tài khoản Zalo chỉ
   *trỏ tới* danh tính khi số đã được xác thực — lần này hoặc lần trước.
3. `phien_cong_dan` thêm `tai_khoan_zalo_id`, và `cong_dan_id` thành **có thể rỗng** (nới NOT
   NULL — không mất dữ liệu). `CitizenSessionPrincipal` thêm trường tuỳ chọn cho tài khoản Zalo;
   `citizen_id = ""` từ nay nghĩa là **chưa xác thực số**.
4. **Mặc định là TỪ CHỐI:** `httpx.XaTuPhien` từ chối phiên chưa có số trên mọi tuyến; tuyến nào
   chỉ để xem (hồ sơ hiển thị, giới thiệu, danh mục) phải **khai tường minh** thêm một lớp kèm lý
   do — cùng kỷ luật `KhongThuocXa`. Tuyến quên khai thì phiên chưa có số bị từ chối, ồn ào.
5. Ba thay đổi 3–4 và bốn chỗ ở bảng trên đi **cùng một commit** của việc cài đặt. Hợp đồng này
   **chưa** sửa `CitizenSessionPrincipal`: sửa ngữ nghĩa `citizen_id` trước khi bên tiêu thụ đổi
   là để `core/identityclient` gặp một giá trị nó đang coi là lỗi.

**Loại:** *"Không phát phiên ViGov nào trước khi có số"*. Nó giữ nguyên mọi chỗ trên, nhưng khi
đó nội dung xã phải đọc **không qua phiên**, bằng `tenant_id` do client đưa lên — ngược ADR 0022
(xã chỉ đến từ phiên), và ngược câu 2 (*"phiên có xã ấy"*).

**Mở lại mà không xin lại số:** tài khoản Zalo đã trỏ tới danh tính thì lần mở âm thầm sau cho
phiên **đã có số**. Mức tin bằng một phiên còn hạn: Zalo chứng nhận cùng một tài khoản. Nếu tài
khoản Zalo đổi số thì ViGov chỉ biết ở lần `phoneToken` kế tiếp; khi đó tài khoản trỏ sang danh
tính của **số mới** và ghi vết. Việc ấy **không** gộp hai danh tính, **không** nhận hồ sơ cũ —
đó là CÒN MỞ #1 của ADR 0020 và không được đụng.

## Hợp đồng

| Tệp | Thêm |
|---|---|
| `proto/vigov/identity/v1/citizen_session_bridge.proto` (**mới**) | `CitizenSessionBridgeService.OpenCitizenSession`, `enum MiniAppMode` |
| `proto/vigov/platform/v1/platform.proto` | `PlatformService.ResolveMiniApp`, `message MiniApp` |

Hình dạng, mã trạng thái và lý do từng trường nằm **trong `.proto`** — không chép lại ở đây.
Hai điều phải đọc cùng tệp này:

- Yêu cầu **không có trường xã nào khai cho bên gọi**. `tenant_hint` là tham số `t` của QR, chỉ
  có hiệu lực kèm `commune_confirmed` ở app chính. Xã là **câu trả lời** (`tenant_id` ở phản hồi).
- Phản hồi mang `tenant_display_name` và **không gì khác về xã**. Địa chỉ, logo, đường dây nóng,
  giờ làm việc, giới thiệu là một lời gọi riêng của công dân tới hồ sơ hiển thị của
  `service-platform` (câu 5). Hợp đồng ấy thuộc việc hồ sơ hiển thị, **chưa viết**.

`vihat-miniapp` là module Go khác (`github.com/vihat/vihat-miniapp`). Nó sinh mã client từ
**cùng tệp `.proto`** này; ViGov sở hữu hợp đồng vì ViGov là bên phục vụ. Cách kho ấy lấy tệp
(chép, submodule, module chung) là việc của kho ấy.

### Bảng `app_id → tenant_id` (service-platform)

Thực thể `MiniApp` (`@entity: MiniApp`): `app_id` (khoá), chế độ `chinh | rieng`, `tenant_id` có
**đúng khi** chế độ là `rieng`. App chính cũng là **một dòng**, không phải biến môi trường: một quy
tắc *"không có dòng → từ chối"* cho mọi app, không có nhánh đặc biệt. Ai thêm/sửa dòng, có ghi vết
ở `audit_log` của platform, là việc cài đặt. **Không có secret nào ở bảng này** (ADR 0032).

## Miễn xã — ĐIỀU KIỆN DỪNG của ADR 0012, KHÔNG sửa `core/grpcx/grpcx.go`

Hai RPC mới không thể biết xã lúc gọi — chúng chính là thứ trả lời *"xã nào"*. Phép thử của ADR
0012 (quyết định 1, mục A) đạt, nhưng **thêm tên vào `methodsWithoutTenant` là việc người dùng
quyết**:

| Tên đầy đủ | Vì sao không mang được `x-tenant-id` | Bề mặt |
|---|---|---|
| `/vigov.identity.v1.CitizenSessionBridgeService/OpenCitizenSession` | Nó **quyết định** xã của phiên | Chỉ trên cổng cầu. Yêu cầu không có trường xã nào khai cho bên gọi |
| `/vigov.platform.v1.PlatformService/ResolveMiniApp` | Nó **trả lời** xã của một app riêng | Một app mỗi lần, không danh sách — cùng hình dạng `ResolveHost`, không phải `ListTenants` |

**Không cần miễn:** lời gọi `GetTenant` ở bước 6 — identity đặt chính xã sắp phát phiên vào
context trước khi gọi, cùng xã nó sắp mở giao dịch có phạm vi.

Hôm nay `methodsWithoutTenant` có hai tên (`grpcx.go:175-178`). Chưa ai trả lời thì hai RPC trên bị
interceptor từ chối `INVALID_ARGUMENT` — đúng như hợp đồng mong.

## Nhất quán và bù trừ

**Mạnh.** Một giao dịch trên CSDL của identity, hai lần đọc đồng bộ sang platform trước đó, không
sự kiện nào. Toàn văn và lý do: `kb/30-indexes/transaction-boundaries.json`, mục
`phat_hanh_phien_cong_dan_qua_cau_mini_app`. Platform không tới được → **503**, không bao giờ
*"dùng xã nhớ lần trước"*.

## Ghi vết

Mọi mục ghi **trong cùng giao dịch** với thay đổi (luật 6 bất biến 3). `actor_kind = citizen`.
"Ai" là mã danh tính công dân, hoặc mã tài khoản Zalo khi chưa có số — **không bao giờ** số điện
thoại hay mã Zalo. IP là `client_ip` do `vihat-miniapp` báo.

| Hành vi | Ghi ở | Nội dung (không dữ liệu cá nhân) |
|---|---|---|
| Mở phiên có xã | `audit_log` của identity, xã ấy | sid, `app_id`, chế độ, nguồn xã: `app_rieng` · `qr_xac_nhan` · `nho_lai`, đã có số hay chưa |
| Mở phiên **không xã** | **Bảng vết chưa thuộc xã** — câu mở #25, **đã chốt nhưng chưa dựng** | như trên |
| Xã đã nhớ đổi A → B | `audit_log`, xã B | trước: A, sau: B. Hai ULID xã, không phải dữ liệu cá nhân |
| Tài khoản Zalo trỏ sang danh tính (lần đầu, hoặc đổi số) | theo xã của phiên, hoặc bảng vết chưa thuộc xã | mã danh tính trước/sau. **Không** ghi số |

**Bảng vết chưa thuộc xã phải nằm trong CSDL của identity**, vì nó phải chung giao dịch với dòng
phiên. Câu #25 chốt *"tầng nền tảng"* nhưng chưa nêu service. Suy ra identity là **đề xuất**, và
khai sở hữu là luật 2 bất biến 1 — cần xác nhận. **Chưa có bảng ấy thì chưa được phát phiên không
xã** (câu #25 `ask_before`: *"gọi TaoChuaChonXa từ bất kỳ tuyến HTTP nào"*).

Không ghi vết: `ResolveMiniApp` (đọc siêu dữ liệu, không có "ai").

## Dữ liệu cá nhân đi trên dây

| Trường | Khi nào đi | Quy tắc |
|---|---|---|
| `zalo_user_id` | Mọi lần | Mã định danh trực tuyến → **dữ liệu cá nhân**. Không log, không trả về, không vào vết. Cách lưu (thô hay băm) chốt ở migration — băm tra được vì không bao giờ cần hiện ra |
| `verified_phone` | **Chỉ** khi lần này có `phoneToken` | Luật 3: không log, không trả về, chỉ đi xuống `dinh_danh_cong_dan` |
| `client_ip`, `device` | Mọi lần | Chẩn đoán và vết. Không phải dữ liệu định danh công dân theo luật 3 |
| `session_token` (phản hồi) | Mọi lần | **Chứng cứ** (luật 8). `vihat-miniapp` chuyển tiếp, không lưu, không log |

`String()` sinh ra in **mọi** trường; không log message của tệp `.proto` này ở cả hai đầu.

## Phải trả

- **Ngay:** một cổng gRPC thứ hai ở identity, một interceptor khoá cầu, một luật NetworkPolicy, hai
  bảng mới (`MiniApp`, `ZaloAccount`), đổi `phien_cong_dan` và bốn bên đọc `citizen_id`.
- **Sau:** `vihat-miniapp` phải giữ N cặp `app_id → secret` và chọn đúng cặp mỗi yêu cầu — UNKNOWN
  #1 quyết cách chọn. Mỗi app riêng mới là một dòng `MiniApp` và một secret ở kho bí mật của
  `vihat-miniapp`.
- **Không mua được:** khoá cầu không chứng minh gì về công dân. Mọi lời khai trong yêu cầu đứng
  hay đổ cùng `vihat-miniapp`.
- **ZNS theo mặc định đã chọn** cần tra ngược `tenant_id → app riêng` cho `service-comms`. Đó là một
  RPC nữa của platform, thêm khi có bên tiêu thụ — chưa viết.

## UNKNOWN — chưa có chứng cứ, phải đo

| # | Câu | Chứng cứ đang có | Vì sao quan trọng |
|---|---|---|---|
| 1 | **Zalo có từ chối `accessToken` khi gửi kèm `secret_key` của một app KHÁC không?** | **Không có.** Lần đo duy nhất dùng token giả, dừng ở lỗi 452 trước khi kiểm secret (`vihat-miniapp/internal/zalo/wire.go:22-35`). Chỉ có một cặp app/secret (`config.go:33-34`) | Có → `vihat-miniapp` nhận gợi ý App ID từ client, đổi bằng secret ấy, và Zalo xác nhận thay ta. Không → *"App ID đã xác minh"* của ADR 0044 không có thật và phải tìm cách khác (API tra token trả app, nếu có). **Chưa đo thì chưa phát hành app riêng.** Hậu quả tệ nhất: công dân vào được một xã mà họ vốn vào được bằng QR công khai — **không** lộ dữ liệu của công dân khác, vì phiên vẫn chỉ đọc hồ sơ của chính mình (luật 4) |
| 2 | Endpoint nào trả **mã tài khoản Zalo** từ `accessToken` mà không cần quyền số điện thoại; mã ấy **theo từng app** hay chung | **Không có.** Kho chỉ gọi `/v2.0/me/info`, trả mỗi số (`wire.go:59`, `:75`) | Không lấy được mã thì câu 2 không cài được. Mã theo app thì một người dùng hai app là hai tài khoản Zalo — vì thế khoá là (`app_id`, `zalo_user_id`) |
| 3 | `getAccessToken()` có **hiện hộp xin quyền** trên máy thật không | Không có — chủ dự án nêu là CHƯA ĐO | Có hộp thì "âm thầm" của câu 2 không đúng |
| 4 | `appsecret_proof` có bắt buộc cho lời gọi lấy mã tài khoản không | `wire.go:37-41` — chưa rõ, mặc định tắt | Sai thì mọi lần mở âm thầm hỏng |
| 5 | `vihat-miniapp` có chạy **cùng cụm k8s** với ViGov không | Không có trong kho này | Khác cụm → chặng cầu ra ngoài mạng cụm mang token rõ → cần TLS → ĐIỀU KIỆN DỪNG |

## CÒN MỞ — cần người dùng

| # | Câu | Đề xuất |
|---|---|---|
| 1 | Hai tên miễn xã ở §Miễn xã | Đạt phép thử ADR 0012 |
| 2 | Phiên chưa có số điện thoại (§Phiên chưa có số) — đổi mô hình tin | Hướng đã ghi, mặc định từ chối |
| 3 | Đổi xã có thu hồi phiên xã cũ không | Có, bằng một trường tuỳ chọn mang token cũ |
| 4 | TTL phiên app công dân ở ViGov (7 ngày của `vihat-miniapp`, `config.go:26`, là chính sách của kho ấy, không phải của ViGov) | Chưa đề xuất — chính sách sản phẩm |
| 5 | Khoá cầu dạng danh sách + vòng đời xoay | Danh sách; vòng đời chưa đề xuất |
| 6 | Service sở hữu bảng vết chưa thuộc xã | `service-identity` |
| 7 | App riêng gắn xã đã sáp nhập | Từ chối, gắn lại bằng tay |

### Trả lời của chủ dự án — 25/09/2026, sau khi ADR được viết

Mục này ghi thêm, không sửa phần trên: phần trên là đề xuất lúc viết, mục này là câu trả lời.

| CÒN MỞ / UNKNOWN | Trả lời |
|---|---|
| CÒN MỞ #1 — hai tên miễn xã | **Đồng ý** đưa `OpenCitizenSession` và `ResolveMiniApp` vào danh sách miễn của `core/grpcx` |
| CÒN MỞ #2 — phiên chưa có số | **Đồng ý hướng §Phiên chưa có số.** Số điện thoại xin **một lần**, lúc công dân lần đầu cần danh tính (gửi phản ánh, tra cứu phiếu của mình) — không xin khi mở app. Sau đó tài khoản Zalo mang **cờ đã xác thực số**, các lần mở sau không hỏi lại. Chỉ xem thông tin xã thì không cần số |
| CÒN MỞ #3 — thu hồi phiên xã cũ khi đổi xã | **Thu hồi ngay.** Yêu cầu đổi xã mang token cũ; mỗi lúc một phiên còn sống |
| UNKNOWN #5 — cùng cụm k8s | **Cùng cụm.** Cầu đi trong mạng cụm, có NetworkPolicy chỉ cho `vihat-miniapp` tới cổng cầu. TLS không bắt buộc; ĐIỀU KIỆN DỪNG #5 vẫn đứng nếu có ngày tách cụm |

| CÒN MỞ #4 — TTL phiên công dân ViGov | **Biến môi trường**, mặc định **1 tháng** (30 ngày). Hằng số toàn nền tảng, không theo xã (luật 1 bất biến 10 không áp) |
| `src=share` (ADR 0005 §Mức tin) | **Bỏ.** QR chỉ đích danh một xã, do ta phát hành. `src` chỉ nhận `qr` · `zns`; mọi giá trị khác, hoặc `t` không kèm `src`, thì **bỏ qua `t`** — cư xử như mở không tham số |

CÒN MỞ #5 (vòng đời xoay khoá), #6, #7 và UNKNOWN #1–#4 **vẫn mở**.

## ĐIỀU KIỆN DỪNG

1. Đề xuất đưa `GRPC_CALLER_KEY` cho `vihat-miniapp`, hoặc phục vụ RPC cầu trên cổng 9090
2. Đề xuất cho client gửi xã, App ID hay số điện thoại **thẳng tới ViGov** thay vì qua cầu
3. Một trường xã **khai cho bên gọi** thêm vào `OpenCitizenSessionRequest`
4. Phát hành app riêng khi UNKNOWN #1 chưa đo
5. Chặng cầu ra khỏi mạng cụm mà không có TLS
6. Một tuyến công dân đọc hay ghi hồ sơ của chính mình mà nhận phiên chưa có số
7. Gộp hai danh tính công dân, hoặc nhận hồ sơ cũ, khi tài khoản Zalo đổi số (ADR 0020 CÒN MỞ #1)

→ ADR 0044 (hai chế độ, bảng `app_id → tenant_id`): `kb/10-decisions/0044-hai-che-do-mini-app.md`
→ ADR 0005 · 0019 · 0022 (tham số dẫn giao diện, xã từ phiên, `KhongThuocXa`)
→ ADR 0020 (xác thực số, CÒN MỞ #1): `kb/10-decisions/0020-xac-thuc-so-dien-thoai-cong-dan.md`
→ ADR 0012 (danh sách miễn xã) · ADR 0025 (khoá dùng chung — không áp cho cầu, xem §Tin cậy)
→ ADR 0032 (app secret không vào ViGov) · ADR 0043 (cổng web-admin không chở cầu)
→ Câu mở #25 (vết chưa thuộc xã) · #28 (bên vận hành `vihat-miniapp`): `kb/00-foundation/open-questions.json`
→ Hợp đồng: `proto/vigov/identity/v1/citizen_session_bridge.proto` · `proto/vigov/platform/v1/platform.proto`
