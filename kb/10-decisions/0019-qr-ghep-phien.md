---
id: 0019-qr-ghep-phien
tier: T1
source: CURATED
owner: architecture
derived_from_commit: d952aaf
expires: null
owns_facts:
  - "QR ghép phiên cho màn hình dùng chung: hướng ghép, luồng sáu bước và tám bất biến"
  - "phân biệt hai loại QR trong hệ thống và vì sao chúng không dùng chung quy tắc phiên bản"
  - "giá trị src=pair và cách nó rẽ luồng khác với các giá trị src còn lại"
  - "service nào sở hữu thực thể phiên ghép"
---

# 0019. QR ghép phiên cho màn hình dùng chung

**Trạng thái:** đã chốt · **Ngày:** 2026-09-17 · **Nối tiếp ADR 0005**

## Bối cảnh

ADR 0005 đã chốt một loại QR: **QR khám phá**, in tĩnh, mang `t=<ulid>`, trả lời câu *công dân
muốn làm việc với xã nào*.

Nay phát sinh loại thứ hai: một màn hình đặt tại trụ sở cần được cấp phiên của một công dân cụ
thể, và công dân dùng điện thoại của mình để cấp. Nhìn qua thì cùng là "quét một mã vuông", nên
sức ép gộp chúng vào một cơ chế là có thật. **Cái giá của việc gộp là chiếm phiên người khác.**

| | QR khám phá (ADR 0005) | QR ghép phiên (ADR này) |
|---|---|---|
| Ở đâu | In tĩnh, bảng tin / trụ sở xã | Sinh động trên một màn hình |
| Trả lời câu | Công dân *muốn* làm việc với xã nào | Màn hình này được cấp phiên của *ai* |
| Nội dung | `?t=<ulid>&src=qr&v=1` | Mã ghép dùng một lần + `t` + `src=pair` |
| Vòng đời | **Nhiều năm** — đã in ra giấy | **≤ 2 phút** |
| Hỏng thì mất gì | Người dân vào nhầm xã | **Chiếm phiên của người khác** |

Hai dòng cuối là toàn bộ lý do tệp này tồn tại. Một cơ chế mà hậu quả hỏng là *"vào nhầm xã"*
và một cơ chế mà hậu quả hỏng là *"người lạ thao tác dưới danh nghĩa mình"* không được phép
dùng chung quy tắc, dù bề ngoài giống hệt nhau.

## Quyết định

**Màn hình hiện QR → Mini App quét và xác nhận. Danh tính luôn đi TỪ Mini App RA, không bao
giờ ngược lại.**

| Hướng | Vì sao |
|---|---|
| **Màn hình hiện QR, điện thoại quét** — đã chọn | Căn cứ danh tính nằm ở điện thoại của công dân: phiên Zalo, số điện thoại đã xác thực. Màn hình chỉ **nhận** thứ nó không tự có |
| Điện thoại hiện QR, kiosk quét — **loại** | Kiosk là thiết bị dùng chung, đặt nơi công cộng, không ai chịu trách nhiệm cho nó. Nó **không phải căn cứ danh tính**, nên không được đứng ở đầu cấp phát |

Nói cách khác: màn hình dùng chung là nơi **tiêu thụ** một phiên, không bao giờ là nơi **sinh
ra** một phiên.

## Luồng — sáu bước

| # | Bước |
|---|---|
| 1 | Màn hình xin mã ghép. Server sinh mã ngẫu nhiên **≥ 128 bit**, gắn `tenant_id` của màn hình, **TTL 120 giây**, kèm **4 ký tự đối chiếu** |
| 2 | Màn hình hiện QR **và** 4 ký tự, rồi chờ kết quả |
| 3 | Công dân quét bằng Zalo, hoặc bằng `scanQRCode` ngay trong app nếu app đang mở → Mini App mở. Chưa đăng nhập thì **đăng nhập trước** |
| 4 | Mini App hiện màn xác nhận: *"Đăng nhập cho màn hình tại UBND xã X"* kèm **4 ký tự**, để công dân **đối chiếu bằng mắt với màn hình trước mặt** |
| 5 | Server kiểm: mã **còn hạn** · **chưa dùng** · `tenant_id` của mã ghép **==** `tenant_id` của phiên công dân. Lệch thì **401 + báo động** (luật 1 bất biến 8). Đạt thì phát hành phiên màn hình với `sid` riêng, TTL **ngắn hơn** phiên app, đánh dấu nguồn `ghep`. **Ghi vết trong cùng giao dịch** (luật 6 bất biến 3) |
| 6 | Màn hình nhận phiên. Phiên **tự hết hạn khi không thao tác**, có **nút thoát**, và công dân **thu hồi được từ trong app** |

Bước 4 là bước duy nhất không có lý do kỹ thuật và cũng là bước không được bỏ. Giải thích ở
bất biến 4 bên dưới.

## Bất biến

| # | Bất biến | Neo vào |
|---|---|---|
| 1 | Mã ghép **ngẫu nhiên, dùng một lần, TTL ≤ 2 phút**, trạng thái lưu ở **server** | Luật 4 bất biến 4 |
| 2 | QR ghép **không bao giờ** mang token, phiên, hay mã hồ sơ | Luật 4 — một liên kết không cấp quyền đọc |
| 3 | **Quét QR không tạo ra danh tính.** Danh tính vẫn đến từ luồng đăng nhập trong Mini App | Luật 4 |
| 4 | **Bắt buộc có bước xác nhận tường minh kèm 4 ký tự đối chiếu** | Chống quét nhầm QR của người khác trên màn hình dùng chung |
| 5 | Phiên ghép mang `sid`, **liệt kê được và thu hồi được** | Luật 5 bất biến 4 |
| 6 | Mỗi lần ghép ghi vết: **ai · thiết bị · IP · xã · lúc nào** | Luật 6 bất biến 2 |
| 7 | Hành vi có **hậu quả pháp lý** trên phiên ghép phải **xác thực lại** | Luật 4 — danh tính yếu không đứng một mình |
| 8 | **Xã của mã ghép phải khớp xã của phiên công dân** | Luật 1 bất biến 8 |

**Vì sao bất biến 4 không phải thừa:** ở bộ phận một cửa có thể có nhiều màn hình cạnh nhau,
mỗi màn hình một QR, và các QR trông giống hệt nhau. Không có gì ngăn người dân quét nhầm màn
hình bên cạnh — màn hình của người đang ngồi đó. Bốn ký tự là thứ duy nhất người dùng **đối
chiếu được bằng mắt** giữa điện thoại và màn hình, và nó rẻ: bốn ký tự, một cái nhìn.

**Vì sao bất biến 7 không thể nới:** phiên ghép sống trên thiết bị của cơ quan, người dân đứng
dậy đi là hết kiểm soát. Nó yếu hơn phiên trong điện thoại của chính công dân, nên nó không
được quyền làm nhiều hơn.

## Hai loại QR, hai quy tắc phiên bản — đừng quản bằng một quy tắc

| | QR khám phá | QR ghép phiên |
|---|---|---|
| Sống bao lâu | Nhiều năm, đã in ra giấy | Hai phút, trên màn hình |
| Đổi khuôn tham số | **Khoá khuôn.** `v` chỉ tăng, bản cũ phải đọc được **mãi mãi** | **Đổi tự do.** Không có nợ tương thích nào |

Vòng đời lệch nhau **hai bậc độ lớn**, nên áp chung một quy tắc phiên bản thì hoặc là trói tay
loại thứ hai một cách vô cớ, hoặc là nới loại thứ nhất tới mức một tấm QR đã in thành rác. Lý
do `v` tồn tại và vì sao nó chỉ tăng nằm ở ADR 0005, không chép lại ở đây.

### `src=pair`

`src=pair` là **giá trị mới thêm** vào tập `src` mà ADR 0005 đã lập (`qr` · `zns` · `share`).
Nó khác mọi giá trị còn lại ở một điểm phải nói rõ:

> `src=pair` **không** kích hoạt cơ chế chọn sẵn xã theo mức tin. Nó **rẽ thẳng vào luồng ghép**.

Mức tin theo nguồn của ADR 0005 trả lời câu *có nên chọn sẵn xã cho người dùng không*. Luồng
ghép không hỏi câu đó: xã đã nằm trong mã ghép do server sinh, và bất biến 8 bắt nó phải khớp
với phiên công dân. Đưa `pair` vào bảng mức tin là để mở đúng cái cửa mà bất biến 8 vừa đóng.

## Sở hữu dữ liệu

`phien_ghep` thuộc **`service-identity`** — ADR 0002 đã chốt phiên đăng nhập do `danhtinh` sở
hữu, và một phiên ghép chỉ là một phiên có nguồn gốc khác.

**Phải khai tường minh, không để suy diễn:** `kb/30-indexes/data-ownership.json` hiện **rỗng**
vì chưa service nào khai schema. Nghĩa là hôm nay không có chỗ nào kiểm được câu trên. Khi bảng
đầu tiên của luồng này ra đời, quyền sở hữu phải được khai ngay tại đó (luật 2 bất biến 1) —
không dựa vào việc "ai cũng biết phiên thuộc identity".

## Điều kiện kỹ thuật đã xác nhận

`scanQRCode` có trong `zmp-sdk` **từ 2.5.3**, theo tài liệu chính thức. Nhánh *"quét ngay trong
app"* ở bước 3 vì thế là khả thi, không phải giả định. Kết quả tra và mức chứng cứ nằm ở
`kb/10-decisions/0018-oa-xac-thuc-tach-khoi-oa-thong-bao.md` §*Kết quả tra tài liệu*, câu 3.

## CÒN MỞ — chưa chốt

| # | Câu hỏi | Vì sao chưa trả lời được |
|---|---|---|
| 1 | **Màn hình được ghép phiên cụ thể là màn hình gì?** Kiosk tại bộ phận một cửa? Màn hình nào khác? | Chưa có mô tả nghiệp vụ. Thiết bị khác nhau thì rủi ro vật lý khác nhau, và TTL cùng chính sách hết hạn phải bám theo đó |
| 2 | **Phiên ghép được làm những gì ngoài đọc?** | Bất biến 7 mới chặn nhóm hành vi có hậu quả pháp lý. Phần giữa — sửa nháp, tải tệp lên, ký nhận — chưa ai chốt, và đó là việc của khách |

Hai câu này **không chặn** việc ghi ADR, vì hướng ghép và tám bất biến đúng với mọi đáp án của
chúng. Nhưng chúng **chặn việc viết mã**: chưa biết màn hình là gì thì chưa đặt được TTL, và
chưa biết phiên ghép làm được gì thì chưa khai được quyền.

## ĐIỀU KIỆN DỪNG

1. Có đề xuất cho **kiosk quét QR trên điện thoại công dân** — đảo hướng ghép, tức bỏ chính
   quyết định của tệp này
2. Có đề xuất **bỏ bước xác nhận** hoặc **bỏ 4 ký tự đối chiếu** để "một chạm cho nhanh"
3. Một hành vi mới trên phiên ghép mà **không rõ có hậu quả pháp lý hay không** — bất biến 7
   không tự phân loại hộ

→ ADR 0005 (khuôn deep link, tham số `v`, bảng `src`, mức tin theo nguồn): `kb/10-decisions/0005-miniapp-tenant-resolution.md`
→ ADR 0002 (phiên đăng nhập và mô hình định danh công dân): `kb/10-decisions/0002-citizen-identity-platform-level.md`
→ ADR 0018 (`scanQRCode` và các điều kiện nền tảng Zalo đã tra): `kb/10-decisions/0018-oa-xac-thuc-tach-khoi-oa-thong-bao.md`
→ Luật 1 (khớp xã, 401 + báo động khi lệch): `.claude/rules/critical/1-tenant-isolation.md`
→ Luật 4 (cách ly công dân, danh tính yếu, mã không đoán được): `.claude/rules/critical/4-citizen-isolation.md`
→ Luật 5 (`sid` và thu hồi phiên): `.claude/rules/critical/5-rbac.md`
→ Luật 6 (ghi vết trong cùng giao dịch): `.claude/rules/critical/6-audit-log.md`
→ Kỹ năng: `.claude/skills/zalo-miniapp-multi-tenant/SKILL.md` · `.claude/skills/session-and-token/SKILL.md`
