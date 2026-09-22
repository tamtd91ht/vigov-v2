---
id: 0034-doc-ten-can-bo-da-xoa-mem
tier: T1
source: CURATED
owner: architecture
derived_from_commit: 70e41e4
expires: null
owns_facts:
  - "vì sao đọc tên cán bộ đã xoá mềm là một RPC RIÊNG chứ không phải một cờ trên BatchGetStaff"
  - "vì sao đường ấy KHÔNG thành cách liệt kê cán bộ đã xoá — bốn tính chất của một đường tra theo id"
  - "vì sao trạng thái trả về nói về BẢN GHI (còn trong danh bạ hay không) chứ không nói về CON NGƯỜI (đã nghỉ hay chưa)"
  - "vì sao đọc đường này KHÔNG tự ghi vết kiểm toán, và điều gì làm câu trả lời ấy đổi"
---

# 0034. Đọc được tên một cán bộ đã xoá mềm — RPC riêng, tra theo id đã biết

**Trạng thái:** đã chốt · **Ngày:** 2026-09-22 · **Thi hành câu mở #10** · **Nối tiếp ADR 0012 · 0021 · 0023 · 0029**

## Bối cảnh — một khiếm khuyết vỡ nhiều năm sau khi gây ra

Câu mở **#10** chốt ngày 22/09/2026: nghỉ hưu / chuyển công tác = **KHOÁ** (vẫn hiện trong danh
bạ); nhập trùng một dòng = **XOÁ MỀM**. Hai thao tác khác nhau. Quyết định ấy **kèm một đòi hỏi
bắt buộc**, và tệp này là chỗ ghi cách trả nó.

Đường đi của khiếm khuyết, đã đo chứ không suy:

| Mắt xích | Thực trạng |
|---|---|
| `audit_log.actor_id` | `TEXT`, **không khoá ngoại** (`service-identity/migrations/0001_init.sql:22`) — cố ý, để vết sống độc lập với dòng nó trỏ tới. **Bản ghi vết vẫn còn** |
| Thứ dịch id ra tên người | Chỉ có `service-identity`. Không service nào khác được mở kết nối tới CSDL của nó (luật 2 cấm #2) |
| `BatchGetStaff` | Hợp đồng chốt: id đã xoá mềm **không trả về** — và `message Staff` **chưa bao giờ mang tên người** |

Hệ quả: một phiếu xử lý năm 2026 mở ra năm 2029 hiện **một chuỗi ULID** ở chỗ đáng lẽ là tên
người. **Luật 6 bất biến 2 không vỡ lúc xoá** — nó vỡ im lặng, nhiều năm sau, trước mặt người
đến thanh tra. Không test nào đỏ, không log nào kêu.

## Quyết định

**Một RPC RIÊNG: `ResolveStaffNames(ids) → StaffName{id, full_name, standing}`**, tra theo **id
mà bên gọi đã cầm sẵn**, trả lời cả id còn trong danh bạ lẫn id đã xoá mềm.

Hình dạng đầy đủ và mọi ràng buộc nằm ở `proto/vigov/identity/v1/identity.proto` — **đó là nguồn
chuẩn** (luật 2 bất biến 7). Tệp này chỉ giữ thứ `.proto` không trả lời được: **vì sao hình dạng
ấy chứ không phải hai hình dạng kia**.

## Vì sao không phải một cờ `bao_gom_da_xoa` trên `BatchGetStaff`

Rẻ nhất để viết, và **hỏng hai lần**:

| | Nó hỏng ở đâu |
|---|---|
| **Không tự nó trả lời được** | `message Staff` **không có tên người**. Muốn dùng cờ thì phải thêm `ho_ten` vào `Staff` — tức đường THƯỜNG NGÀY, đường mà một ô chọn người phụ trách gọi, bắt đầu mang dữ liệu cá nhân nó không cần |
| **Một phản hồi mang hai nghĩa** | `Staff` không có chỗ nào nói dòng này là dòng đã xoá. Một bản ghi đã gỡ khỏi danh bạ trở về **không phân biệt được** với một bản ghi đang tại chức — nên người gọi bật cờ "cho chắc" sẽ mời một người đã nghỉ vào ô chọn người phụ trách, **không có gì đỏ** |

Đây đúng hình dạng **ADR 0029 §119 đã bác một lần**, khi `ResolveDeadlines` lẽ ra có thể là một
cờ trên `AdvanceWorkingHours`. Thứ mua được ở đó mua lại được ở đây: **người review phân biệt
được hai chỗ gọi bằng mắt**, tại chính dòng gọi.

## Vì sao không phải hai danh sách trong phản hồi (`items` + `removed_items`)

Hình dạng này đặt sự phân biệt vào chỗ không bỏ sót được — và nó **dời chỗ hỏng chứ không gỡ
bỏ**: người gọi vẽ danh sách thứ nhất rồi quên danh sách thứ hai sẽ **đánh rơi đúng cái tên nó
mở lời gọi để lấy**, và hồ sơ lưu trữ hiện trống ở chỗ tên người. Một danh sách, ghép theo `id`,
kèm **một trường** nói cách gán nhãn, thì không có nhánh nào để quên.

**Cái giá của RPC thứ hai, nói thẳng:** thêm một method, và một màn hình cần cả tên lẫn vai trò
nay gọi hai lần. Đó là giá thật, và là giá nhỏ hơn — hai lời gọi hỏi **hai câu khác nhau về hai
tập người khác nhau**: `BatchGetStaff` trả lời *"id này có phải cán bộ của xã này hôm nay không"*
(câu **được phép quyết định theo**), `ResolveStaffNames` trả lời *"in tên gì cạnh dòng lịch sử
này"* (câu **chỉ để hiển thị**).

## Đường này có thành cách LIỆT KÊ cán bộ đã xoá không — KHÔNG, và đây là vì sao

Câu này phải trả lời ra giấy, vì *"đọc được tên người đã xoá"* chỉ cách *"liệt kê những người
một xã đã xoá"* đúng **một trường bất cẩn** — tức một bề mặt rò rỉ MỚI dựng lên bởi bản vá cho
một lỗ CŨ.

**Bốn tính chất, đều là cấu trúc chứ không phải lời hứa:**

1. **Yêu cầu có đúng MỘT trường, và đó là danh sách id.** Không bộ lọc, không con trỏ trang,
   không khoảng ngày, không tiền tố tên, không phòng ban. **Không có yêu cầu nào máy chủ trả lời
   được bằng một id mà bên gọi chưa tự nêu ra.**
2. **`ids` rỗng trả về rỗng** — không bao giờ là "tất cả". Đây chính là **một dòng** mà một lần
   sửa về sau có thể biến tra cứu thành liệt kê, nên nó nằm trong hợp đồng chứ không để cho cài đặt.
3. **Phản hồi không bao giờ mang id chưa được hỏi**, không mang tổng số, không mang con trỏ — bên
   gọi không học được rằng còn thứ gì khác tồn tại.
4. **Id là ULID và phạm vi cắt theo `x-tenant-id`**: 80 bit ngẫu nhiên không duyệt được, và id
   của xã khác **vắng mặt**, không phân biệt được với "không tồn tại" (ADR 0012 quyết định 2).

**Thứ nó THẬT SỰ cho, nói thẳng thay vì chối:** ai đã cầm hồ sơ của một xã thì lấy lại được tên
của mọi người từng tác động lên hồ sơ ấy, kể cả người đã bị gỡ khỏi danh bạ. **Hồ sơ lưu trữ tồn
tại chính là để thế.** Thứ nó **không** cho là một **bảng biên chế**: người không xuất hiện trên
hồ sơ nào bên gọi đang cầm thì không với tới được qua đường này, và việc đọc chính hồ sơ mang id
ấy đã có cổng quyền của service gọi.

> **Lằn ranh không được vượt:** không được thêm vào yêu cầu bất kỳ trường nào **không phải là một
> id bên gọi đã cầm**. Một vị từ ở đó — khoảng ngày, phòng ban, "mọi người bị gỡ từ ngày…" — biến
> RPC này thành đúng cái danh sách nó được thiết kế để không phải, **trong một dòng, với mọi test
> hiện có vẫn xanh**.

## Trạng thái trả về nói về BẢN GHI, không nói về CON NGƯỜI

Đòi hỏi ban đầu viết là *"trạng thái không còn công tác"*. **Hợp đồng cố ý không nói câu ấy**, và
đây là lý do — nó là một câu **sai** trên màn hình của một cơ quan nhà nước:

- Theo chính quyết định #10, **xoá mềm = dòng nhập trùng**. Con người trên dòng ấy **có thể đang
  tại chức** dưới dòng còn lại của họ. Dán nhãn "không còn công tác" là nói một người đã nghỉ
  trong khi họ chưa nghỉ.
- Với các dòng **cũ hơn quyết định** thì dữ liệu **không phân biệt nổi**: `reversal_cost` của
  chính câu #10 ghi rằng trước ngày ấy chỉ có MỘT nút, nên một dòng đã xoá có thể là nghỉ việc
  thật, có thể là nhập trùng, và **không cột nào phân biệt được về sau**.

Nên enum `StaffRecordStanding` chỉ nói thứ dữ liệu thật sự giữ — `deleted_at` — bằng hai giá trị:
**còn trong danh bạ** / **đã gỡ khỏi danh bạ**. Đặt tên theo trạng thái nhân sự sẽ **khẳng định
một sự thật dữ liệu không có**, đúng cái hỏng ADR 0023 §2 gọi tên: *một trạng thái dữ liệu không
đỡ nổi là một trạng thái rồi sẽ có người đem đi đếm.* Câu tiếng Việt in ra màn hình là **chữ của
bên gọi**, không phải của hợp đồng.

**Tài khoản bị KHOÁ vẫn là "còn trong danh bạ"** — đúng như #10 chốt. Thêm nữa,
`nguoi_dung.dang_hoat_dong` cũng là cột một lần đình chỉ tạm thời sẽ đặt: một cột, hai nghĩa, nên
nó cũng không đỡ nổi một giá trị thứ ba ở đây.

## Đọc đường này KHÔNG tự ghi vết, và điều gì làm câu trả lời ấy đổi

Luật 6 bất biến 7 ghi vết việc **đọc dữ liệu cá nhân đầy đủ**. Ở đây **không ghi**, vì hai lý do
phải cùng còn đúng:

| Lý do | Nội dung |
|---|---|
| **Service này không nêu được "AI"** | Yêu cầu chỉ mang id và một xã, **không mang principal**. Một khoá gọi dùng chung (ADR 0025) chứng minh lời gọi đến từ bên trong cụm, **không bao giờ chứng minh service nào gọi**. Vết ghi ở đây mang actor hệ thống và một địa chỉ trong cụm — không trả lời được câu nào trong sáu câu luật 6 bất biến 2 hỏi. Service **MỞ HỒ SƠ** mới cầm principal và địa chỉ thật, và **lần đọc ấy** mới là chỗ đặt vết, trong giao dịch của chính nó (cùng hình dạng `AdvanceWorkingHours`) |
| **Nó chạy theo nhịp tải trang** | Một lời gọi cho mỗi màn hình có hiện "ai đã làm". Ghi vết mỗi lời gọi sẽ đẻ dòng nhanh hơn chính những lần GHI mà vết sinh ra để truy, và **chôn** những dòng có giá trị pháp lý |

**Câu trả lời đổi đúng vào ngày message này mang một trường KHÔNG cần để in một dòng lịch sử** —
số điện thoại, email, số định danh. Người thêm trường ấy đang mở một đường đọc dữ liệu cá nhân
đầy đủ (luật 3 bất biến 3) và **nợ nó một khoá quyền cùng một vết kiểm toán**.

## Hệ quả

- **Dễ hơn:** hồ sơ cũ đọc được tên; mục `staff-thieu-ho-ten` treo ở
  `kb/90-ephemeral/tien-do/proto.json` được gỡ mà **không** phải thêm dữ liệu cá nhân vào
  `message Staff`. Toàn hệ thống có **đúng MỘT đường** để tên một cán bộ rời khỏi `identity` — một
  chỗ để nới, một chỗ để từ chối một số điện thoại, một chỗ để ghi vết nếu ngày ấy tới
- **Khó hơn:** màn hình cần cả tên lẫn vai trò gọi hai lần. Đừng "gộp cho gọn": gộp là dựng lại
  đúng cái cờ mục trên vừa bác
- **Phải trả ngay:** máy chủ + kho đọc + bọc client chưa có. Hợp đồng đã viết, `buf generate` đã
  chạy, **chưa RPC nào trả lời** — `UnimplementedIdentityServiceServer` trả `Unimplemented`, ồn ào,
  đúng thiết kế
- **Phải trả sau — CHƯA AI QUYẾT:** `audit_log.actor_id` hôm nay nhận **id nội bộ**
  (`core/audit/audit.go:89` ghi `authz.Principal.ID`), trong khi chú thích của cột `nguoi_dung.ma`
  (`0001_init.sql:167`) và lập luận câu #15 (migration 0009 §4) đều giả định vết mang **`ma`**.
  Hai cách đọc **mâu thuẫn nhau và chưa ai chọn**. Hợp đồng lấy **id**, vì đó là thứ bên ghi đang
  thật sự ghi. Ngày vết đổi sang `ma`, thêm một lối tra thứ hai ở đây là việc **cộng thêm** — và
  là **câu hỏi cho người dùng**, không được đoán: một vết mà RPC này không phân giải được sẽ hiện
  trống trên hồ sơ lưu trữ, đúng khiếm khuyết nó sinh ra để đóng

## ĐIỀU KIỆN DỪNG

1. Thêm **bất kỳ trường nào vào yêu cầu** ngoài danh sách id — đó là biến tra cứu thành liệt kê
2. Thêm vào `StaffName` một trường **không cần để in một dòng lịch sử** — kéo theo khoá quyền và
   vết kiểm toán (luật 3 bất biến 3, luật 6 bất biến 7)
3. Đổi chỗ `audit_log.actor_id` lưu từ **id nội bộ** sang **`ma`**, hoặc ngược lại
4. Cho phép một bên gọi **quyết định** điều gì từ RPC này thay vì chỉ hiển thị

→ Câu mở #10 (hai thao tác: khoá / xoá mềm): `kb/00-foundation/open-questions.json`
→ Câu mở #11 (không che dữ liệu cán bộ trong nội bộ xã — thứ gỡ chặn cho tên người đi qua biên)
→ ADR 0012 (xã đi trong metadata; ba ca vắng mặt cố ý không phân biệt được)
→ ADR 0021 (hợp đồng khai thứ ĐANG CÓ, không khai thứ đang định làm)
→ ADR 0023 (đặt tên theo thứ bản ghi GIỮ; trạng thái dữ liệu không đỡ nổi)
→ ADR 0029 §119 (RPC thứ hai chứ không phải một cờ — tiền lệ của quyết định này)
→ Hình dạng hợp đồng (nguồn chuẩn): `proto/vigov/identity/v1/identity.proto`
→ Luật 2 (`.proto` là nguồn chuẩn) · luật 3 (dữ liệu cá nhân) · luật 6 (vết kiểm toán) ·
  luật 7 (xoá mềm, không bao giờ huỷ)
