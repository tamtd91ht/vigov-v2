---
id: 0023-thuat-ngu-kenh-cong-dan
tier: T1
source: CURATED
owner: architecture
derived_from_commit: d2d2aa6
expires: null
owns_facts:
  - "vì sao kế thừa đơn vị hành chính gọi là succession chứ không phải alias"
  - "vì sao quan hệ công dân↔xã không còn giá trị vãng lai và giá trị thay thế là chua_khai"
  - "nơi cư trú là lời khai của công dân chờ cán bộ xã xác thực, và hệ quả của việc trộn lời khai với trạng thái pháp lý"
  - "vì sao lời khai cư trú đã được cán bộ xác thực vẫn không phải căn cứ cho hành vi có hệ quả pháp lý"
  - "lời khai cư trú bị từ chối giữ một trạng thái riêng kèm lý do công dân đọc được"
  - "chính quyền địa phương hai cấp và hệ quả cho cây chọn xã của Mini App"
  - "vì sao danh mục tỉnh thành do nhà cung cấp seed sẵn thay vì suy ra từ cột tinh_thanh"
  - "vì sao đường dẫn thông tin xã của màn đăng nhập đổi thành communes/current"
---

# 0023. Thuật ngữ kênh công dân: thực thể, bảng, tài nguyên URL

**Trạng thái:** đã chốt · **Ngày:** 2026-09-17 · **Nối tiếp ADR 0002 · 0005 · 0019 · 0021 · 0022**

**Bổ sung cùng ngày:** khách đã trả lời **câu 1 và câu 2** của §CÒN MỞ trong bản đầu. Câu trả
lời nằm ở §A1 và §A2; §CÒN MỞ giữ nguyên số hiệu cũ để trích dẫn không lệch.

## Bối cảnh

ADR 0021 §ĐIỀU KIỆN DỪNG để lại hai câu chưa trả lời: **tên thực thể** cho phiên công dân và
phiên ghép, và **tên tài nguyên URL** cho bảng kế thừa cùng danh mục xã. ADR 0011 cấm tự dịch
tại chỗ, nên trong lúc hai câu ấy còn mở thì **không viết được migration nào** của kênh công
dân — quyền sở hữu thực thể khai ngay trên `CREATE TABLE` (ADR 0021), và một cái tên đặt sai ở
đó thì mọi tầng trên sai theo.

Tệp này đóng chúng, và **không sửa một chữ nào** trong ADR 0002, 0005, 0019, 0021 — đúng tiền
lệ ADR 0020: một ADR đã chốt ghi đúng suy nghĩ của ngày nó được viết; hệ quả ghi ở tệp mới.

## Bảng chốt — sáu khái niệm

| Khái niệm | Thực thể (`@entity`) | Bảng | Tài nguyên URL |
|---|---|---|---|
| Phiên đăng nhập của công dân | `CitizenSession` | `phien_cong_dan` | `citizen-sessions` |
| Mã ghép phiên (ADR 0019) | `SessionPairing` | `ghep_phien` | `session-pairings` |
| Quan hệ công dân ↔ xã | `CitizenCommune` *(giữ)* | `quan_he_cong_dan_xa` | *(không có tài nguyên riêng)* |
| Đơn vị cũ → đơn vị kế thừa | `TenantSuccession` | `tenant_succession` | *(không bao giờ là tài nguyên)* |
| Danh mục xã cho Mini App | — (đọc `Tenant`) | — | `communes` |
| Định danh công dân | `CitizenIdentity` | `dinh_danh_cong_dan` | *(không bao giờ có danh sách)* |

Ánh xạ này cũng có dòng trong `kb/00-foundation/ubiquitous-language.md` — tệp đó là bảng tra
khi viết route; tệp này là **lý do**. Ba ô *(không có tài nguyên)* là quyết định, không phải chỗ
trống chờ điền: một danh sách công dân, một danh sách quan hệ cư trú, một danh sách đơn vị kế
thừa đều là thứ luật 4 bất biến 6 cấm đưa ra kênh công dân.

## Vì sao không phải những cái tên dễ đoán hơn

### 1. `alias` là từ sai, và nó sai theo hướng nguy hiểm

`.claude/skills/admin-unit-merge/SKILL.md` bất biến 6 viết bằng chữ: *"The new commune is a
**new** tenant with a 'merged from' mapping — Commune A does not 'become' commune B"*, và bất
biến 2: hồ sơ giữ `tenant_id` gốc cùng cơ quan ban hành **tại thời điểm ban hành**.

Hai xã là **hai pháp nhân**. Một bảng tên `alias` khẳng định chúng là một thứ dưới hai cái tên,
và người đọc cái tên đó sáu tháng sau sẽ viết đúng câu truy vấn bất biến 2 cấm: thay `tenant_id`
cũ bằng `tenant_id` mới trên hồ sơ lưu trữ, *"vì chúng là alias của nhau mà"*. Không test nào đỏ.

`succession` = **kế thừa**, giữ lại đúng cái mà `alias` xoá mất: **có trước và có sau**.

**Cái phải trả, và nó có thật:** `proto/vigov/platform/v1/platform.proto` đã có
`rpc ResolveTenantAlias` và hai message của nó. Bảng thì **chưa có** (`service-platform/migrations/`
mới tới `0002`), nên phần đắt — dữ liệu và khoá ngoại — vẫn miễn phí hôm nay. Đổi tên trên bề mặt
hợp đồng là **lượt của `contract-designer`**, không phải việc của tệp này.

### 2. `vãng lai` khẳng định một sự thật mà dữ liệu không có

"Vãng lai" nghĩa là **đang có mặt trên địa bàn** mà không đăng ký cư trú. Nhưng dòng quan hệ này
sinh ra theo ADR 0005 khi *"hành vi gửi tạo quan hệ"* — một người **đang ngồi ở Hà Nội** gửi phản
ánh cho một xã ở Đà Nẵng cũng tạo ra nó. Hệ thống không biết người đó ở đâu, và không có đường nào
để biết.

Đúng phép thử mà `org-units` đã dạy một lần: từ dễ đoán mô tả **màn hình người ta tưởng tượng**,
không mô tả **thứ đang thật sự được lưu**.

**Giá trị thứ ba là `chua_khai`** — nói đúng điều hệ thống biết: chưa có khai báo cư trú nào cho
cặp *(công dân, xã)* này. Không nhiều hơn, không ít hơn.

*Ghi chú cho lượt hợp đồng:* enum trong `identity.proto` hiện đánh vần hai giá trị anh em bằng
tiếng Anh (`CAPACITY_PERMANENT_RESIDENT`, `CAPACITY_TEMPORARY_RESIDENT`) trong khi ADR 0002 gọi
chúng là `thuong_tru`/`tam_tru`. Lệch đó **có sẵn trước tệp này** và không phải thứ tệp này chốt.

### 3. `Citizen` → `CitizenIdentity`, và bảng là `dinh_danh_cong_dan`

Bản ghi giữ đúng hai thứ: một định danh và một số điện thoại đã che. Một bảng tên `cong_dan` là
**lời mời** thêm `ho_ten`, `so_cccd`, `ngay_sinh` — và nó nằm đúng vùng **xuyên xã, không có
`tenant_id` che chắn** (ADR 0002, ADR 0021 `@scope: cross-tenant`), tức mọi ràng buộc nặng nhất
của Nghị định 13/2023 dồn cả vào đó.

Tên `dinh_danh_cong_dan` không ngăn được ai thêm cột. Nó làm việc thêm `so_cccd` thành một câu
**phải nói thành lời** trước khi làm, và đó là toàn bộ điều một cái tên làm được.

### 4. `ghep_phien`, không phải `phien_ghep`

ADR 0019 tách **hai** thứ; ADR 0021 §"nơi sẽ đặt dấu" lại liệt *Phiên công dân* và *Phiên ghép*
thành hai dòng thực thể ngang hàng. Hai tệp đang mô tả hai mô hình khác nhau, và một trong hai
phải thắng trước khi có migration.

**Chốt:** thứ cần một bảng là **mã ghép** — ngẫu nhiên ≥128 bit, TTL 120 giây, 4 ký tự đối chiếu,
dùng một lần (ADR 0019 luồng sáu bước). Còn **phiên mà màn hình cầm không phải thực thể mới**: nó
là một `CitizenSession` có cột `nguon = 'ghep'`. Đúng câu ADR 0019 đã viết ở §Sở hữu dữ liệu —
*"một phiên ghép chỉ là một phiên có nguồn gốc khác"*.

Trong tiếng Việt `phiên ghép` là danh ngữ — **một loại phiên**; `ghép phiên` là **việc ghép**. Bản
ghi này ghi một **lượt ghép**, nên nó mang tên việc.

### 5. Hai cái tên bị loại, và lý do loại nằm trong ADR 0019

| Tên bị loại | Vì sao |
|---|---|
| `qr_login` · `qr_sessions` | ADR 0019 bất biến 3 nói thẳng: *"Quét QR không tạo ra danh tính"*. Một cái tên có chữ `login` khẳng định điều ngược lại, ngay tại chỗ người viết mã nhìn nhiều nhất |
| `device_pairings` · `kiosk_*` | ADR 0019 §CÒN MỞ câu 1 — *"màn hình được ghép phiên cụ thể là màn hình gì"* — **chưa ai trả lời**. Đặt `kiosk` vào tên là tự trả lời hộ khách, bằng một danh từ không ai gỡ ra được sau khi có migration |

### 6. `commune` phủ cả **xã, phường và đặc khu**

Từ 01/7/2025 đơn vị hành chính cấp xã gồm **ba** loại: xã, phường, đặc khu. Từ `commune` trong
tiếng Anh chỉ phủ "xã".

Không ghi rõ điều này thì người đến sau, gặp một **phường** trong kết quả `/api/v1/communes`, sẽ
kết luận danh mục bị thiếu và thêm `/api/v1/wards` — danh mục bị tách đôi trong khi dữ liệu là
một, và Mini App phải gọi hai đường để dựng một màn hình chọn. Cùng đúng loại lỗi mà
`org-units` (không phải `departments`) đã tránh được một lần.

**Ba loại là ba loại, không phải ba tài nguyên.** Loại đơn vị là một **thuộc tính** của `Tenant`.

### 7. Trong văn xuôi tiếng Việt: "danh mục xã", KHÔNG phải "danh bạ xã"

"Danh bạ" là danh sách **liên hệ** — và kho đã dùng đúng nghĩa đó ở `docs/ui-ux/12-danh-ba-can-bo.md`.
Một "danh bạ xã" đọc như một danh sách số điện thoại của các xã. Thứ đang nói là một **danh mục**
để công dân chọn. URL vẫn là `communes`; đây là quy ước cho **văn xuôi**, để hai phiên không gọi
một thứ bằng hai tên khi bàn về nó.

## Ba câu người dùng vừa chốt

### A. Nơi cư trú là LỜI KHAI của công dân, ở trạng thái CHỜ XÁC THỰC

> *"Lời khai của công dân trong app nhưng đang để ở trạng thái chờ xác thực, cán bộ xã sẽ xác
> thực ở web admin."*

Câu này rộng hơn một quyết định đặt tên, và đây là phần đắt nhất của tệp.

**Quan hệ công dân↔xã mang hai sự thật khác nhau, không gộp vào một cột:**

| Sự thật | Ai tạo ra | Tin được tới đâu |
|---|---|---|
| Công dân **khai** gì (`thuong_tru` · `tam_tru` · `chua_khai`) | Công dân, trong Mini App | Danh tính yếu (luật 4, ADR 0020 bất biến 3) |
| Xã **đã xác thực** chưa | Cán bộ xã, trên web admin | Hành vi nghiệp vụ của cán bộ có tài khoản |

Gộp hai thứ này làm một cột là xoá mất đúng cái phân biệt vừa được chốt, và xoá theo cách không
ai nhìn thấy: cột vẫn có giá trị hợp lệ, truy vấn vẫn chạy, báo cáo vẫn ra số.

**Vì sao nó quan trọng hơn vẻ ngoài:** một báo cáo ghi *"1.200 công dân thường trú"* sẽ được đọc
là 1.200 người **có đăng ký** thường trú. Lời khai chưa xác thực **không phải** một trạng thái
pháp lý — theo Luật Cư trú 2020, thường trú và tạm trú là trạng thái **đã đăng ký với cơ quan
công an**. Trộn hai thứ là đưa một con số sai lên lãnh đạo, sai theo hướng **không ai phát hiện
được** cho tới khi có người đối chiếu với sổ hộ khẩu điện tử.

**Hệ quả bắt buộc, không phải gợi ý:**

| # | Hệ quả | Neo vào |
|---|---|---|
| 1 | Việc xác thực là **ghi dữ liệu nghiệp vụ** ⇒ ghi vết: ai · lúc nào · từ IP nào · xã nào | Luật 6 bất biến 1, 2 |
| 2 | Màn xác thực cần **một quyền tường minh**. Quyền đó **chưa tồn tại** trong bảng `quyen` | Luật 5 bất biến 1, 3b |
| 3 | Lời khai chưa xác thực **không đủ** làm căn cứ cho thủ tục có hậu quả pháp lý | Luật 4, ADR 0020 bất biến 3 |
| 4 | Công dân thấy lời khai của chính mình cùng trạng thái của nó; ghi chú nội bộ của cán bộ thì không | Luật 4 bất biến 5, 7 |

Hệ quả 2 là việc **phải làm**, không phải việc **được đặt tên ở đây**: `ubiquitous-language.md`
chưa có dòng nào cho nó, và ADR 0011 cấm tự dịch. Đặt tên khoá quyền là một câu hỏi cho khách.

### A1. Xác thực rồi thì vẫn KHÔNG phải căn cứ pháp lý — khách chốt 2026-09-17

Câu 1 của §CÒN MỞ bản đầu — *"có thủ tục nào của xã phụ thuộc giá trị nơi cư trú đã xác thực
không"* — khách trả lời: **không**.

**Quan hệ đã được cán bộ xác thực vẫn không đủ làm căn cứ cho hành vi có hệ quả pháp lý. Nó là
dữ liệu tiện lợi:** điền sẵn biểu mẫu, gợi ý xã, lọc danh sách. Mọi thủ tục có hệ quả pháp lý đi
theo quy trình giấy tờ riêng của nó.

**Vì sao đây không phải một quyết định thận trọng quá mức — và đây là chỗ dễ đọc nhầm nhất của
cả tệp.** Cán bộ xác thực được *"người này khai như thế"*. Cán bộ **không** xác thực được
*"người đang cầm điện thoại chính là người đó"*. ADR 0020 đã chốt: Zalo khẳng định số thuộc tài
khoản nào, **không** khẳng định ai đang cầm máy — máy mở sẵn, máy cho mượn, máy của con cháu, cả
ba đều qua cửa.

Nên một bước xác thực của cán bộ **nâng độ tin của nội dung lời khai**, chứ **không nâng độ tin
của người khai**. Hai thứ đó khác nhau, và chỉ thứ thứ hai mới là thứ luật 4 đòi trước khi cho
một hành vi có hệ quả pháp lý đi qua. Xác thực không biến danh tính yếu thành danh tính mạnh —
không có thao tác nào trong hệ thống này làm được điều đó.

**Cảnh báo hướng về tương lai — đọc trước khi đặt tên cột hay tên trạng thái.** Người viết module
hồ sơ sáu tháng nữa sẽ gặp một giá trị *"đã xác thực"* và rất dễ đọc ra **"giấy thông hành"**:
*"xã đã xác thực rồi, khỏi kiểm nữa"*. Tên trạng thái, tên cột và nhãn trên màn hình không được
để ai kết luận như vậy — và nếu một cái tên nào đó khiến kết luận ấy trở nên tự nhiên thì cái tên
đó sai, kể cả khi mã phía dưới vẫn đúng.

### A2. Bị từ chối là một TRẠNG THÁI RIÊNG, không quay về `chua_khai` — khách chốt 2026-09-17

Câu 2 của §CÒN MỞ bản đầu. Khách chốt: **một trạng thái "bị từ chối" riêng, giữ nguyên lời khai
và giữ lý do.**

**Vì sao không đưa về `chua_khai`:** công dân sẽ không phân biệt được *"chưa ai xử lý"* với
*"đã bị từ chối"*. Đó đúng lớp lỗi mà luật 10 bất biến 6 cấm ở phiếu phản ánh — đóng phiếu
**bắt buộc có kết quả công dân đọc được**, không bao giờ đóng lặng lẽ. Cùng nguyên tắc, khác bề
mặt: im lặng là cách niềm tin vào kênh chết đi, và một kênh không ai tin thì không còn nhận được
những gì xã thật sự cần nghe.

Ba thứ kéo theo, không thứ nào tuỳ chọn:

| # | Kéo theo | Neo vào |
|---|---|---|
| 1 | **Lý do từ chối là văn bản cán bộ viết cho công dân đọc** — không phải mã lỗi nội bộ, không phải ghi chú nghiệp vụ | Luật 10 bất biến 6 · luật 4 bất biến 5 |
| 2 | Lời khai **giữ nguyên**, không xoá — công dân thấy mình đã khai gì và vì sao không được chấp nhận | Luật 7 |
| 3 | Từ chối là **ghi dữ liệu nghiệp vụ** ⇒ ghi vết đầy đủ như việc xác thực | Luật 6 bất biến 1 |

### B. `/api/v1/commune` → `/api/v1/communes/current`

Tại `derived_from_commit` của tệp này, hợp đồng REST sinh từ mã (ADR 0014) mang
`GET /api/v1/commune` — *"thông tin xã ứng với tên miền đang gọi, cho màn hình đăng nhập"*, khai
`public`. Kênh công dân thì sắp có `GET /api/v1/communes`.

Hai lý do, theo thứ tự trọng lượng:

1. `.claude/skills/rest-api-design/SKILL.md` REQUIRED #1 bắt danh từ tài nguyên ở dạng **số
   nhiều**. `commune` số ít vi phạm, và nó là route duy nhất còn vi phạm.
2. Để nguyên thì **hai đường dẫn cách nhau đúng một chữ `s`** trả về hai thứ khác hẳn nhau cho
   hai lớp người dùng khác hẳn nhau: một cái là xã của tên miền đang gọi (cán bộ, trước khi đăng
   nhập), một cái là danh mục xã để chọn (công dân, chưa có xã nào). Ai đọc log, ai viết client,
   ai rà soát hợp đồng cũng phải dừng lại một nhịp.

**Rủi ro ở đây là lẫn lộn, không phải rò rỉ** — phải nói rõ để đừng ai nâng mức nghiêm trọng lên
rồi vội: `identity.thongTinXa` không mang `id`, `TenantSummary` không mang `host`, nên nhầm hai
đường dẫn không đưa dữ liệu của bề mặt này sang bề mặt kia.

**Việc sửa mã thuộc một phiên khác** (`service-identity` + `web-admin`). Tệp này chỉ ghi quyết
định và lý do — nếu đọc lại mà route đã mang tên mới thì đó là phiên ấy đã chạy, không phải tệp
này tự sửa mã.

### C. Danh mục tỉnh/thành: nhà cung cấp seed sẵn 34 đơn vị, xã chỉ được chọn

Lý do thật **không phải** "vì danh sách ổn định" — nó vừa được vẽ lại năm 2025.

`service-platform/migrations/0001_init.sql:66` khai `tinh_thanh TEXT NOT NULL DEFAULT ''` kèm
chú thích `-- province, for display only`. Nghĩa là danh sách tỉnh cho bộ lọc trong picker, nếu
suy ra từ dữ liệu đang có, chính là `SELECT DISTINCT tinh_thanh`. Hai xã cùng một tỉnh, một xã
khai `Đà Nẵng`, xã kia khai `Thành phố Đà Nẵng`, thì **công dân thấy hai tỉnh**, mỗi tỉnh chứa
một nửa số xã. Không test nào đỏ. Người phát hiện ra là **công dân không tìm thấy xã mình**, và
họ không có chỗ nào để báo điều đó.

Ràng buộc bắt buộc khi seed:

| # | Ràng buộc | Vì sao |
|---|---|---|
| 1 | Khoá là **ULID vô nghĩa** | Luật 1 bất biến 2. Năm 2025 vừa chứng minh **tỉnh cũng sáp nhập được** — một khoá mang nghĩa buộc sửa khoá ngoại trên dữ liệu lịch sử ở lần sáp nhập kế tiếp |
| 2 | Mã thống kê là **thuộc tính**, không phải khoá, và **chưa thêm** | Câu hỏi mở #4 (tổng hợp nhiều xã) còn OPEN. ADR 0021 quy tắc 4: chỉ mục ghi cái **đang có**, không ghi dự định — một cột thêm sẵn "để sau này dùng" là một lời hứa người sau sẽ hành động theo |
| 3 | Xã **chọn** tỉnh từ danh mục, không gõ | Đó là toàn bộ điều khoản này mua được |

**Cảnh báo về thời điểm — phần dễ bỏ qua nhất của mục này.** Chuyển `tinh_thanh` từ chuỗi hiển
thị thành khoá ngoại **rẻ đúng hôm nay**, vì migration ghi rõ nó *for display only* và chưa có
hồ sơ nào trỏ vào. Ngày có module ban hành văn bản in tên tỉnh vào giấy tờ, giá trị ấy trở thành
*"cơ quan ban hành tại thời điểm ban hành"* — đúng thứ `admin-unit-merge` bất biến 2 cấm sửa. Cửa
sổ này đóng lại một lần và không mở lại.

## Chính quyền địa phương hai cấp — sự thật nền của cả tệp này

Từ 01/7/2025: **tỉnh → xã**. Cấp huyện **chấm dứt hoạt động**. Cây duyệt trong picker chọn xã vì
thế là **hai tầng**, không phải ba.

Ba hệ quả:

1. **Thứ tự picker hiện tại càng đúng hơn, không phải phải thiết kế lại.** `skills/zalo-miniapp-multi-tenant`
   xếp: gần đây → GPS → tìm theo tên → duyệt. Bỏ một tầng nghĩa là nhánh duyệt trở thành một danh
   sách phẳng trên trăm dòng — và đó chính là lý do nó phải nằm **cuối**, không phải lý do để sửa
   thứ tự.
2. **ĐIỀU KIỆN DỪNG:** có ai đề xuất dựng lại cấp huyện làm tầng gom "cho dễ duyệt" — kể cả lấy
   từ dữ liệu lịch sử — thì **dừng**. Bày trước mặt công dân một cấp chính quyền **không còn tồn
   tại và họ không liên hệ được nữa** là hệ thống của một cơ quan nhà nước nói sai về chính bộ
   máy nhà nước.
3. **Tên cũ gánh phần vai trò mà cấp huyện để lại.** Người dân nhớ *"xã Tân Phú, huyện Y"* sẽ gõ
   "Tân Phú". `TenantMatch.matched_former_name` (`proto/vigov/platform/v1/platform.proto`) chính
   là thứ trả lại ngữ cảnh đó. Nghĩa là bảng kế thừa **không chỉ phục vụ QR đã in** như ADR 0005
   mô tả — nó là **một nửa khả năng tìm thấy xã** trong picker.

**Mức chắc chắn — ghi để người sau không phải tra lại:** **cao** về cấu trúc hai cấp, về 34 đơn
vị cấp tỉnh, và về ba loại đơn vị cấp xã. **Đừng in con số đơn vị cấp xã vào bất cứ đâu** — nó
còn thay đổi, và đó cũng chính là lý do nó không được trở thành một hằng số trong mã.

## Phải trả

- **Trả ngay:** `ubiquitous-language.md` dài thêm sáu dòng; `service-platform` phải seed danh mục
  tỉnh trước khi onboard xã thứ hai
- **Trả sau:** đổi `ResolveTenantAlias` trên bề mặt hợp đồng — rẻ hôm nay vì chưa có cài đặt và
  chưa có bảng, đắt dần theo mỗi bên tích hợp
- **Không mua được:** một cái tên đúng không ngăn được ai viết sai. Nó chỉ làm cái sai phải được
  gõ ra thành lời trước đã

## CÒN MỞ — khách phải chốt

Số hiệu giữ nguyên theo bản đầu, để mọi trích dẫn đã có không lệch.

| # | Câu hỏi | Trạng thái |
|---|---|---|
| 1 | Có **thủ tục nào của xã phụ thuộc** giá trị nơi cư trú đã xác thực không? | **ĐÃ CHỐT 2026-09-17 — không.** → §A1 |
| 2 | Lời khai **bị từ chối** thì về trạng thái nào? | **ĐÃ CHỐT 2026-09-17 — trạng thái riêng, giữ lý do.** → §A2 |
| 3 | Công dân **sửa lời khai đã được xác thực** thì sao? | **CÒN MỞ.** Mất hiệu lực xác thực cũ, hay cần một quy trình? Đây là câu quyết định liệu bản ghi có lịch sử phiên bản hay không — và lịch sử phiên bản là thứ không thêm được sau khi đã có dữ liệu thật |
| 4 | **Tên khoá quyền** cho việc xác thực lời khai cư trú | **CÒN MỞ, và nó chặn việc viết route.** ADR 0011 cấm tự dịch; luật 5 bất biến 3b bắt khoá quyền đúng là chuỗi màn Phân quyền hiện ra và bảng `quyen` lưu |

Hai câu còn lại **không chặn** việc đặt tên bảng và tài nguyên — sáu dòng trong bảng chốt đúng
với mọi đáp án của chúng. Cả hai đã vào `kb/00-foundation/open-questions.json`.

## ĐIỀU KIỆN DỪNG

1. Một đề xuất **gộp lời khai cư trú và trạng thái xác thực vào một cột** — xem §A
2. Một báo cáo, thống kê hay số liệu nào **đếm lời khai chưa xác thực như trạng thái pháp lý**
3. Một luồng nghiệp vụ đọc trạng thái **đã xác thực** như **điều kiện đủ** cho một hành vi có hệ
   quả pháp lý, hoặc để bỏ qua một bước kiểm — §A1 chốt là không, và lý do nằm ở luật 4
4. Đóng một lời khai bị từ chối **mà không có lý do công dân đọc được** — §A2
5. Dựng lại **cấp huyện** ở bất kỳ đâu người dân nhìn thấy
6. Thêm cột dữ liệu cá nhân vào `dinh_danh_cong_dan` — vùng xuyên xã, không có `tenant_id` che
7. Đặt **mã hành chính** làm khoá cho tỉnh hoặc cho xã — luật 1 bất biến 2
8. Một khái niệm mới của kênh công dân **chưa có dòng** trong `ubiquitous-language.md`: hỏi,
   đừng dịch

→ ADR 0002 (định danh công dân ở tầng nền tảng, quan hệ nhiều-nhiều): `kb/10-decisions/0002-citizen-identity-platform-level.md`
→ ADR 0005 (bảng kế thừa là yêu cầu phát sinh cho `platform`, khuôn deep link): `kb/10-decisions/0005-miniapp-tenant-resolution.md`
→ ADR 0019 (mã ghép: TTL, 4 ký tự đối chiếu, tám bất biến): `kb/10-decisions/0019-qr-ghep-phien.md`
→ ADR 0020 (công dân vẫn là danh tính yếu sau khi xác thực số): `kb/10-decisions/0020-xac-thuc-so-dien-thoai-cong-dan.md`
→ ADR 0021 (dấu `@entity` trên `CREATE TABLE`, và hai câu tệp này đóng): `kb/10-decisions/0021-khai-quyen-so-huu-thuc-the.md`
→ ADR 0022 (lớp `KhongThuocXa` mà `communes` sẽ dùng): `kb/10-decisions/0022-ria-kenh-cong-dan.md`
→ ADR 0011 (tiếng Anh trên URL, và cấm tự dịch tại chỗ): `kb/10-decisions/0011-contract-surface-language.md`
→ Bảng tra khi viết route: `kb/00-foundation/ubiquitous-language.md`
→ Câu còn mở 3 và 4: `kb/00-foundation/open-questions.json`
→ Kỹ năng: `.claude/skills/admin-unit-merge/SKILL.md` · `.claude/skills/zalo-miniapp-multi-tenant/SKILL.md` · `.claude/skills/rest-api-design/SKILL.md`
