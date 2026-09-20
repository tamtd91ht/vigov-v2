---
id: 0012-grpc-boundary-contract
tier: T1
source: CURATED
owner: architecture
derived_from_commit: b1c6cc7
expires: null
owns_facts:
  - "vì sao tenant_id đi trong metadata gRPC với khoá x-tenant-id, không bao giờ trong thân message"
  - "vì sao lời gọi gRPC giữa các service mặc định theo lô, và vì sao vượt trần là lỗi chứ không kẹp"
  - "quyết định 3 (xác thực giữa hai service) ĐÃ BỊ ADR 0025 THAY THẾ ngày 2026-09-20 — tệp này giữ nguyên văn lập luận cũ; sự thật hiện hành thuộc về ADR 0025"
  - "vì sao platform không tới được thì mọi Host thành 404 chứ không 503"
---

# 0012. Ranh giới gRPC giữa các service

**Trạng thái:** đã chốt · **Ngày:** 2026-09-16 · **Nối tiếp ADR 0001, 0003, 0011**

> **Quyết định 1, 2, 4 còn nguyên hiệu lực. QUYẾT ĐỊNH 3 ĐÃ BỊ ADR 0025 THAY THẾ (2026-09-20)**
> — nguyên văn giữ lại làm lịch sử, chi tiết ở đầu mục ấy.

## Bối cảnh

Luật 2 đã chốt *có* hai đường đi giữa các service — gRPC đồng bộ và sự kiện bất đồng bộ —
và `kb/00-foundation/domain-boundaries.md` sở hữu bảng "service nào được gọi thẳng service
nào". Cả hai đều **không nói lời gọi đó mang xã đi như thế nào**, mang bao nhiêu dữ liệu một
lần, ai chứng minh mình là ai, và điều gì xảy ra khi đầu bên kia không trả lời.

Bốn khoảng trống đó lẽ ra được lấp bằng cách đoán, mỗi service đoán một kiểu. Bốn quyết định
dưới đây lấp chúng **trước khi có bên tiêu thụ thứ hai**, khi giá còn bằng 0: hợp đồng đã
viết ra nhưng chưa service nào gọi service nào trong môi trường thật.

Tệp này trả lời **vì sao**. Hình dạng hợp đồng nằm ở `proto/` — đó mới là nguồn chuẩn.

---

## Quyết định 1 — Xã đi trong metadata, khoá `x-tenant-id`, không bao giờ trong thân message

### Vì sao metadata chứ không phải một trường trong message

Thân message là **dữ liệu bên gọi khai**. Một bên gọi tự khai mình thuộc xã nào là một bên
gọi tự cấp quyền cho mình — đúng điều luật 1 cấm #2 cấm ở tầng HTTP, và không có lý do gì nó
trở nên an toàn hơn khi đi qua gRPC. Metadata được **interceptor phía client lấy từ
`context.Context`** đặt vào, không bao giờ do mã nghiệp vụ truyền như một tham số. Đây là
cùng một lập luận đã làm cho `core/tenant` giữ xã trong context thay vì trong chữ ký hàm: một
thứ truyền được như tham số là một thứ sẽ có ngày bị truyền sai.

Hệ quả bắt buộc ở đầu nhận: **không thấy `x-tenant-id` thì từ chối** bằng `INVALID_ARGUMENT`.
Không suy ra xã từ payload, không có xã mặc định. Hỏng thì đóng.

### Vì sao đúng tên `x-tenant-id` — điểm chịu lực của quyết định này

Tên khoá không phải chuyện thẩm mỹ. `core/httpx/edge.go:39` — `StripTenantHeaders` — **xoá
mọi header từ ngoài vào có tiền tố `x-tenant`**, không liệt kê từng tên mà quét theo tiền tố.
Đặt khoá metadata trong đúng tiền tố đó nghĩa là: nếu có ngày một header client cung cấp lọt
được tới tầng trong, nó đã bị lớp phòng thủ **hiện có** xoá trước.

Chọn một tên ngoài tiền tố đó — `tenant`, `commune-id`, `vigov-tenant` — là tạo ra một cái
tên mà lớp phòng thủ **không quét**, tức là tự tay mở đúng cái lỗ luật 1 cấm #2 đang bịt. Một
quyết định đặt tên, hai lớp khớp nhau; và lớp kia đã viết rồi, không phải lời hứa.

(Khoá viết thường vì gRPC hạ chữ thường mọi khoá metadata — không phải một lựa chọn, là một
ràng buộc của giao thức.)

### Ngoại lệ — HAI CHUYỆN KHÁC NHAU, đừng gộp

Ở ranh giới này có hai thứ trông giống nhau và thường bị gộp làm một. Gộp chúng là cách
`GetTenant` suýt được miễn kiểm metadata, trong khi nó **không** được miễn:

| | **A. Miễn kiểm metadata** | **B. Id nằm trong thân message** |
|---|---|---|
| Trả lời câu | RPC này có bắt buộc mang `x-tenant-id` không | Giá trị id trong thân nghĩa là gì |
| Thành viên | **Đúng một: `ResolveHost`** | `GetTenant` (id xã), `ResolveHost` (host) |
| Quan hệ với nhau | **Không có.** B không kéo theo A | |

#### A. Miễn kiểm metadata — danh sách trắng đúng một thành viên

`ResolveHost` chạy **trước khi biết xã** — nó chính là thứ trả lời câu hỏi "xã nào". Bắt nó
mang `x-tenant-id` là một vòng lặp: phải biết xã mới hỏi được xã là gì.

Đó là lý do **duy nhất** được chấp nhận để miễn: RPC **không thể** có xã tại thời điểm gọi —
không phải "chưa tiện có", không phải "gọi nội bộ nên thôi".

**`GetTenant` BẮT BUỘC mang `x-tenant-id`.** Nói thẳng ra đây để người đọc sau không phải
suy luận. Nó luôn được gọi khi bên gọi **đã biết** mình đang phục vụ xã nào, nên mang thêm
metadata không tốn gì; và một danh sách miễn càng ngắn thì càng rà được bằng mắt trong một
lần review.

Danh sách phải khai **tường minh theo tên method đầy đủ** trong interceptor, và không bao giờ
được miễn trừ theo mặc định (kiểu "thiếu metadata thì bỏ qua kiểm tra"). Lý do: một miễn trừ
theo mặc định im lặng nuốt luôn mọi RPC quên đặt metadata, và cách hỏng của nó là **xử lý
thành công một lời gọi không có xã** — dạng mất cách ly mà không test nào đỏ. Danh sách
tường minh thì ca hỏng ngược lại: quên khai thì lời gọi **bị từ chối**, ồn ào, sửa trong một
phút.

**Phép thử trước khi thêm thành viên thứ hai:** *RPC này có thể biết xã tại thời điểm gọi
không?* Có → **không miễn**, dù bất tiện. Thêm một thành viên vào danh sách là **ĐIỀU KIỆN
DỪNG** — hỏi người dùng, đừng tự quyết trong lúc viết mã.

#### B. Id nằm trong thân message — không phải một ngoại lệ

`GetTenant` nhận id xã trong thân, `ResolveHost` nhận host trong thân. Ở cả hai, giá trị đó
là **chủ thể đang được tra**, không phải lời khai của bên gọi về chính mình. Hai câu hoàn
toàn khác nhau:

| Câu | Đi đâu | Ai đặt |
|---|---|---|
| "Tôi **đang thuộc về** xã nào" | **Metadata** | Interceptor, lấy từ context — bên gọi không tự khai được |
| "Tôi **đang hỏi về** xã nào" | **Thân message** | Bên gọi, vì đó là tham số tra cứu |

Nên `GetTenant` mang **cả hai**: `x-tenant-id` nói bên gọi là ai, id trong thân nói đang hỏi
về ai. Hai giá trị đó **có thể khác nhau** — một service hỏi siêu dữ liệu của xã khác.

Điều đó chỉ chấp nhận được **vì ADR 0003**: `platform` không có đường trả về nội dung nghiệp
vụ nào cả, nên một lời gọi nêu tên xã bất kỳ ở đây cũng không chạm tới được dữ liệu của xã
đó. **Service nghiệp vụ không được sao chép hình dạng B**: ở đó, một id xã nằm trong thân
message đúng là lời khai mà luật 1 cấm #2 cấm.

---

## Quyết định 2 — Lời gọi gRPC mặc định theo lô

`GetStaff(id)` đã được thay bằng `BatchGetStaff(ids)`. Đây là mặc định cho mọi RPC tra cứu
về sau, không phải một lần sửa lẻ.

### Vì sao

Bên gọi của loại RPC này **luôn là một màn hình danh sách** đang điền tên người xử lý vào
từng dòng. Dạng số ít biến việc đó thành **một vòng mạng cho mỗi dòng**: 20 dòng là 20 lời
gọi, và con số đó tăng theo dữ liệu chứ không đứng yên. Đặc điểm tệ nhất của nó là **vô hình
trong test** — test chạy với ba dòng dữ liệu vẫn xanh và vẫn nhanh; chỗ hỏng chỉ xuất hiện ở
xã có nhiều hồ sơ nhất, tức là xã quan trọng nhất.

Hợp đồng số ít còn phản lại chính skill của kho mã: `.claude/skills/load-data-once` REQUIRED
#4 gọi đích danh trường hợp này. Bên tiêu thụ đầu tiên sẽ làm theo **hợp đồng**, không làm
theo skill — nên sai phải sửa ở hợp đồng.

### Vì sao vượt trần là LỖI, không phải kẹp im lặng

Trần 200 id; vượt thì `INVALID_ARGUMENT`. Điều này **cố ý lệch** quy ước phân trang ở
`.claude/skills/rest-api-design` §5 (kẹp `limit` về mức tối đa, không báo lỗi), và chỗ lệch
đó là chủ ý chứ không phải sót:

| | Phân trang | Tra theo lô |
|---|---|---|
| Kẹp bớt thì bên gọi biết không | **Có** — còn cursor để đi tiếp | **Không** — không có cursor nào |
| Hậu quả khi kẹp | Lấy thêm một trang | Dòng có thật hiện **tên trống**, không ai báo |

Cắt bớt một batch là **sai im lặng**: bản ghi có thật hiện ra thiếu thông tin, và không lớp
nào phát hiện được. Báo lỗi thì bên gọi buộc phải chia lô — phiền một lần lúc viết mã, còn
hơn sai âm thầm mãi mãi.

Con số 200 suy ra từ bên gọi chứ không bốc: một trang mặc định 20 dòng nên 200 là một bậc độ
lớn dự phòng, và 200 cũng phủ trọn biên chế một đơn vị cấp xã trong một lời gọi. Một danh
sách id **không chặn trên** từ bên gọi là một truy vấn không chặn trên ở bên nhận.

### Ba giả định sai mà bên gọi hay mắc

Ghi ở đây vì cả ba đều **hỏng im lặng** — không lỗi, không log, chỉ ra số liệu sai:

| Giả định | Thực tế | Hậu quả khi tin nhầm |
|---|---|---|
| Trả về đúng bằng số id đã hỏi | **Trả ít hơn là hợp lệ** | Ghép lệch, gán tên người này cho hồ sơ người kia |
| Thứ tự trả về theo thứ tự `ids` | **Thứ tự không mang nghĩa** | Như trên — và đây là dạng hồ sơ hành chính bị **làm sai lệch**, không phải lỗi hiển thị |
| Id vắng mặt nghĩa là không tồn tại | Có thể là **không tồn tại**, **đã xoá mềm**, hoặc **thuộc xã khác** | Suy ra sai; tệ hơn là đòi hỏi phân biệt được ba ca đó |

Ba ca vắng mặt **cố ý không phân biệt được**. Phân biệt chúng là trả lời câu "bản ghi này có
tồn tại ở xã khác không" cho một bên không được đọc xã đó — cùng dạng rò rỉ mà luật 4 cấm #2
cấm khi bắt `404` và `403` phải giống nhau. Bên gọi **phải ghép theo `Staff.id`**.

---

## Quyết định 3 — Xác thực giữa hai service: TẠM dựa vào cách ly mạng

> ## ĐÃ BỊ THAY THẾ — 2026-09-20, bởi **ADR 0025**
>
> **Toàn bộ quyết định 3 dưới đây không còn là hướng dẫn đang có hiệu lực.** Người dùng đã
> chốt ngày 2026-09-20: dựng xác thực bên gọi bằng **một cặp header/giá trị dùng chung**, giá
> trị đọc từ biến môi trường nguồn k8s secret, phòng thủ hai lớp cùng với mạng nội bộ của cụm.
>
> **Câu bị lật đổ cụ thể:** mục *"Vì sao KHÔNG dựng một cơ chế bí mật chia sẻ tạm ngay bây
> giờ"*. Chỉ thị ấy **không còn đúng** — đừng làm theo nó. Lý do người dùng nêu cho hình dạng
> một cặp (thay vì khoá riêng mỗi service, hay khoá kèm key-id để xoay) là **bảo trì**.
>
> **Nguyên văn được giữ lại có chủ ý, không xoá.** Ba lý do dưới đây **không** sai vào ngày
> chúng được viết, và lý do thứ hai — *"một bí mật dùng chung … không phân biệt được ai đang
> gọi, không thu hồi riêng được"* — **vẫn đúng nguyên vẹn hôm nay**: nó là **cái giá** người
> dùng chọn trả, không phải một điều đã được bác bỏ. ADR 0025 có nguyên một mục cho nó.
>
> **Thứ dưới đây còn hiệu lực:** điều kiện gỡ #1 (cổng gRPC không lắng nghe trên địa chỉ công
> khai) — ADR 0025 giữ nó làm **nửa thứ hai** của cơ chế, không phải một chú thích. Điều kiện
> #2 (mTLS/mesh) bị ADR 0025 thay bằng khoá dùng chung; cửa mTLS không đóng, chỉ không phải
> việc hôm nay.
>
> → `kb/10-decisions/0025-xac-thuc-giua-cac-service.md`

**Đây là một rủi ro đã chấp nhận có chủ ý, không phải một thiết kế.** Ghi nó như một thiết kế
là cách nó biến thành vĩnh viễn: sáu tháng sau không ai nhớ đây là chỗ còn nợ.

Hiện tại không có mTLS, không có token giữa service với service. Một tiến trình nào đó gọi
được tới cổng gRPC nội bộ thì gọi được RPC.

### Vì sao chấp nhận được ở thời điểm này

| Căn cứ | Nội dung |
|---|---|
| ADR 0003 | Ranh giới này **chưa có RPC nào trả dữ liệu nghiệp vụ** — `platform` chỉ trả siêu dữ liệu, theo thiết kế, không theo cờ bật/tắt |
| ADR 0005 | ULID mà `ResolveHost` trả về **vốn đã công khai**: nó nằm trong deep link và trên QR in ở bảng tin xã |
| `GetTenant` | Trả tên hiển thị, trạng thái, hạn mức — thứ mà bất kỳ ai mở trang của xã đó cũng thấy |

Nói cách khác: thứ đang không được bảo vệ hôm nay có **giá trị bằng thứ đã in ra giấy dán ở
trụ sở**. Đó là toàn bộ lý do, và nó hết hiệu lực đúng vào ngày có RPC đầu tiên trả nội dung.

### Vì sao KHÔNG dựng một cơ chế bí mật chia sẻ tạm ngay bây giờ

Ba lý do, lý do thứ ba là lý do thật:

1. **Gần như chắc chắn bị thay.** Khi lên hạ tầng thật, lời giải là danh tính theo service
   (mTLS hoặc service mesh). Một cơ chế token chia sẻ viết hôm nay là mã sẽ bị xoá.
2. **Nó bảo vệ ít hơn vẻ ngoài.** Một bí mật dùng chung cho cả 8 service không phân biệt được
   ai đang gọi, không thu hồi riêng được, và phải nằm trong cấu hình của cả 8 — bề mặt rò rỉ
   rộng hơn thứ nó bảo vệ.
3. **Nó tạo cảm giác an toàn giả.** Người review sau này thấy "đã có xác thực" và không hỏi
   nữa. Rủi ro *đã chấp nhận* lặng lẽ trở thành rủi ro *đã quên* — và một dòng trong ADR này
   dễ đọc hơn, trung thực hơn, rẻ hơn một cơ chế nửa vời.

### Phải làm gì trước khi chạy thật — điều kiện gỡ

| # | Điều kiện |
|---|---|
| 1 | Cổng gRPC **không lắng nghe trên địa chỉ công khai**; chỉ mạng nội bộ, có network policy |
| 2 | Danh tính theo service (**mTLS** hoặc mesh) trước khi xã đầu tiên chạy thật |
| 3 | **ĐIỀU KIỆN DỪNG:** RPC đầu tiên trả dữ liệu nghiệp vụ qua ranh giới này → dừng, hỏi người dùng, viết ADR mới. Căn cứ ở mục trên tan biến đúng lúc đó |

> **Kết thúc phần đã bị thay thế.** Điều kiện dừng #3 ở trên đã chạy đúng như thiết kế: bốn
> service khung xếp hàng chờ đúng chỗ này, người dùng được hỏi, và ADR mới là **ADR 0025**.

---

## Quyết định 4 — `platform` sập thì mọi Host thành 404

`core/tenant.Directory.ByHost` trả `(Tenant, bool)` — **không có lỗi**. Nên khi `platform`
không tới được, mọi Host đều rơi vào nhánh "không khớp", và cán bộ của mọi xã thấy đúng cái
màn hình mà một tên miền không tồn tại sẽ cho: như thể xã mình không có trên hệ thống.

### Vì sao vẫn giữ 404

Luật 1 bất biến 3 ghi thẳng: *"Không phân giải được = 404"*. Đổi thành 503 lúc này nghe hợp
lý hơn về mặt vận hành nhưng kéo theo một hệ quả nặng hơn: chữ ký `ByHost` phải mang lỗi, và
khi đã mang lỗi thì mọi nơi gọi nó phải chọn **làm gì khi có lỗi** — đó chính là chỗ một
`?? default` mọc lên. 404 là hành vi **hỏng thì đóng** duy nhất không có nhánh nào để chọn
sai. Thêm nữa, 404 không tiết lộ xã nào tồn tại trên nền tảng, và điều đó vẫn đúng cả khi
`platform` khoẻ.

### Bù lại bằng hai thứ, không phải bằng việc đổi mã lỗi

| Bù | Vì sao |
|---|---|
| **Log mức báo động** khi `ByHost` hỏng vì lý do truyền tải | Đây là chỗ 404 nói dối: nó nói "không có xã này" trong khi sự thật là "không hỏi được". Người trực phải phân biệt được hai thứ đó, ngay cả khi công dân và cán bộ thì không |
| **Cache TTL ngắn** ở mỗi edge | Bản đồ Host→xã chỉ đổi khi đổi tên miền. Cache biến một sự cố ngắn của `platform` thành **không sự cố**, thay vì thành sự cố toàn hệ thống |

### Đánh đổi đã biết, nói thẳng

**Cache nguội + `platform` sập = toàn bộ hệ thống ngừng, và ngừng dưới dạng 404.** Đó là hệ
quả đã cân nhắc và chấp nhận, không phải điều chưa ai nghĩ tới. `platform` là phụ thuộc đồng
bộ trên đường đi của **mọi** request; đó là cái giá của việc phân giải xã ở một chỗ duy nhất,
và cái giá ngược lại — mỗi service tự giữ bản đồ xã — đắt hơn nhiều vì nó tạo ra nhiều nguồn
cho một fact ngay trên đường cách ly.

**Nếu sau này muốn phân biệt 404 với 503:** đó là đổi `core/tenant.Directory` — một giao diện
nằm trên đường đi của mọi request — và vì vậy là **một ADR mới**, không phải một quyết định
ứng biến trong lúc viết mã.

---

## Hệ quả

- **Dễ hơn:** mọi RPC mới chỉ cần theo mặc định đã chốt — metadata mang xã, hình dạng theo lô
- **Khó hơn:** bên gọi `BatchGetStaff` phải tự chia lô và phải ghép theo `id`; không được
  dựa vào thứ tự hay số lượng trả về
- **Phải trả ngay:** interceptor hai đầu và danh sách miễn phải tồn tại **trước** server gRPC
  đầu tiên phục vụ thật — không có interceptor thì hợp đồng trên chỉ là văn bản
- **Phải trả sau:** xác thực giữa service với service (quyết định 3) là món nợ có ngày đáo
  hạn xác định, không phải món nợ mở
  — **đã đáo hạn 2026-09-20: ADR 0025 chốt hình dạng trả nợ. Đọc ADR 0025, không đọc dòng này**

→ **ADR 0025 (xác thực giữa các service — THAY THẾ quyết định 3 của tệp này):** `kb/10-decisions/0025-xac-thuc-giua-cac-service.md`
→ ADR 0003 (`platform` chỉ siêu dữ liệu — căn cứ của quyết định 3 và của hình dạng B): `kb/10-decisions/0003-platform-admin-metadata-only.md`
→ ADR 0005 (ULID công khai trong deep link): `kb/10-decisions/0005-miniapp-tenant-resolution.md`
→ Đường đi hợp lệ giữa các service: `kb/00-foundation/domain-boundaries.md`
→ Luật 1 (xã từ Host, cấm nhận `tenant_id` từ client): `.claude/rules/critical/1-tenant-isolation.md`
→ Luật 2 (hai đường đi, `.proto` là nguồn chuẩn): `.claude/rules/critical/2-service-boundary.md`
→ Skill: `.claude/skills/load-data-once/SKILL.md` · `.claude/skills/rest-api-design/SKILL.md`
