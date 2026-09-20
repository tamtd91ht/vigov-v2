---
id: 0027-trang-thai-va-dong-ho-phieu-phan-anh
tier: T1
source: CURATED
owner: domain
derived_from_commit: 3d43fa1
expires: null
owns_facts:
  - "chín trạng thái phiếu phản ánh là danh sách cố định, xã không thêm không bớt"
  - "khách duyệt nguyên văn chín chuỗi mã trạng thái ngày 2026-09-20"
  - "đổi lĩnh vực lúc phân loại chỉ được rút ngắn hạn, không bao giờ kéo dài"
  - "cả hai đồng hồ hạn của phiếu khởi động từ lúc công dân bấm gửi"
  - "đồng hồ tiếp nhận dừng khi có cán bộ động vào, không phải khi sinh phiếu"
  - "vì sao mã trạng thái lấy theo nhãn tiếng Việt chứ không theo chuỗi tiếng Anh của bản mẫu"
---

# 0027. Chín trạng thái cố định, và hai đồng hồ đếm từ lúc dân bấm gửi

**Trạng thái:** đã chốt · **Ngày:** 2026-09-20 · **Nối tiếp ADR 0007 · 0008**
**Đóng điều kiện dừng #2 của luật 10** cho phiếu phản ánh — và **chỉ** cho phiếu phản ánh

## Bối cảnh

Ba câu về vòng đời phiếu chưa có ai ký. Chúng không nằm trong `open-questions.json` vì chúng
chỉ lộ ra khi bắt tay viết hàm tính hạn và máy trạng thái — và cả ba đều là thứ luật 10 cấm
người viết mã tự quyết: danh sách trạng thái (điều kiện dừng #2), cách tính hạn (điều kiện
dừng #1), và lúc nào đồng hồ bắt đầu chạy.

Ngày 2026-09-20 khách trả lời cả ba.

## Quyết định B — chín trạng thái, danh sách CỐ ĐỊNH

**Vòng đời phiếu phản ánh có đúng chín trạng thái, theo `docs/ui-ux/09-phan-anh-nguoi-dan.md`
§6. Xã không thêm, không bớt.**

Bảng mã ↔ nhãn nằm ở `kb/00-foundation/ubiquitous-language.md` §Phản ánh và SLA — **một chủ,
không chép sang đây**. Ba điều thuộc về ADR này:

| # | |
|---|---|
| 1 | Danh sách là **hằng số của phần mềm**, không phải danh mục ở màn hình Cấu hình. Nếu có bảng thì bảng ấy không cho xã thêm dòng |
| 2 | Mã viết **tiếng Việt không dấu**, kebab-case — ADR 0011 cấm dịch giá trị enum sang tiếng Anh |
| 3 | Máy chuyển trạng thái được phép **gọi thẳng tên mã** trong mã nguồn. Đó là hệ quả trực tiếp của việc danh sách cố định, và nó chỉ đúng **sau** quyết định này |

**Vì sao cố định, nói bằng ca hỏng cụ thể.** Một danh sách trạng thái sửa được theo xã cho
phép hai thao tác màn hình làm sập vòng đời mà không cần lỗi phần mềm nào: thêm một mã thì
sinh ra một trạng thái không lối vào và không lối ra — phiếu rơi vào đó nằm lại vĩnh viễn;
tắt mã đóng phiếu thì **không phiếu nào của cả xã kết thúc được**, mà mỗi phiếu đang mở là
một cam kết đang chạy đồng hồ với một người dân có mã tra cứu trong tay. Đó là lý do luật 10
đặt danh sách trạng thái vào điều kiện dừng ngay từ đầu.

**ADR này KHÔNG đóng câu mở #21.** Câu #21 hỏi về trạng thái **nhiệm vụ** (`02-nhiem-vu.md`),
là vòng đời khác, của đối tượng khác, do người khác vận hành — và ô để trống §3 của ADR 0024
vẫn chờ nó. Hai vòng đời trả lời giống nhau là chuyện có thể xảy ra, nhưng phải do khách nói,
không do một phiên suy ra từ tệp này.

### Đặc tả đã sửa theo — và vì sao mã lấy theo NHÃN, không theo chuỗi tiếng Anh cũ

`docs/ui-ux/09-phan-anh-nguoi-dan.md` §6 trước 2026-09-20 liệt kê chín trạng thái bằng chuỗi
tiếng Anh (`received`, `screening`, `assigned`, …) — định danh bên trong bản mẫu mà một xã
đang chạy, chép nguyên vào đặc tả. Mã của hệ thống này thì phải là **tiếng Việt không dấu**
(ADR 0011), nên hai bề mặt nói hai thứ khác nhau về cùng chín trạng thái.

**Phiên này hỏi khách chứ không tự xử.** `docs/` là đặc tả của khách; sửa nó là việc của người
sở hữu nó. Khách trả lời: sửa cho khớp. Đặc tả nay mang đúng chín mã tiếng Việt, kèm một khối
trích dẫn ghi lại bản trước và lý do đổi, và §14 trỏ sang ADR 0007 · 0027 thay vì chép lại.

**Nên không còn chỗ lệch nào để dung hoà** — và đó là lý do mục này tồn tại: người đọc lần sau
gặp chuỗi `received` trong `git log`, trong ảnh chụp màn hình cũ, hoặc trong chính bản mẫu xã
đang chạy, phải biết đó là **tên đã bị thay**, không phải một cách gọi thứ hai đang song song.

**Vì sao mã mới lấy theo nhãn tiếng Việt chứ không dịch ngược chuỗi tiếng Anh** — chỗ này phải
đọc bằng mắt, không bằng từ điển: `out_of_scope` dịch sát là *"ngoài phạm vi"*, nghe như một
lời từ chối; nhãn tiếng Việt cùng dòng là **`Chuyển cấp trên`**, một việc hành chính **có nơi
nhận**. Hai thứ ấy khác nhau ở hệ quả với người dân, không khác nhau ở cách diễn đạt. Nhãn là
thứ xã đọc và nói với dân, nên nhãn là bản đúng; chuỗi tiếng Anh là bản dịch của một lập trình
viên khác, đã một lần đi chệch.

Bảng chín mã có **một chủ**: `kb/00-foundation/ubiquitous-language.md`. Không chép sang đây,
và cũng đừng đọc bản trong `docs/` như nguồn chuẩn — đặc tả là thứ được sửa theo, không phải
thứ quyết định.

## Quyết định C — đổi lĩnh vực lúc phân loại: chỉ RÚT NGẮN

**Hạn sau khi đổi lĩnh vực = mốc SỚM HƠN trong hai mốc:** hạn đã hứa lúc tiếp nhận, và hạn
tính theo lĩnh vực mới từ **cùng một** mốc khởi động. Không bao giờ lấy mốc muộn hơn.

**Lý do khách ký, viết nguyên ý:** hạn là lời **đã nói** với dân, kèm mã tra cứu để họ mở ra
xem lại. Kéo dài nó là lặng lẽ rút lại lời đã hứa — và người phát hiện đầu tiên luôn là người
đã được nghe con số cũ.

### Vì sao điều này KHÔNG trái luật 10 bất biến 2

Bất biến 2 cấm **tính lại lúc đọc**: một giá trị dựng lại mỗi lần truy vấn thì nó trôi theo
cấu hình, theo ngày lễ mới khai, theo cả lỗi đồng hồ — và bản trôi là bản đi vào báo cáo.

Ở đây hạn vẫn là **một giá trị được lưu**, và nó chỉ đổi khi có một **hành vi nghiệp vụ** của
một con người, hành vi ấy có ghi vết mang giá trị trước và sau (luật 6 bất biến 5). Tiền lệ
đã có trong ADR 0008: `tinh_lai_han_khi_mo_lai` ghi một hạn mới lúc mở lại phiếu. Cột hạn
chưa bao giờ là cột bất động; thứ bị cấm là **nguồn thứ hai** cho cùng một con số.

Và chiều của phép `min` khép luôn cửa còn lại: một hạn chỉ đi xuống thì không thao tác nào
trên màn hình nới được cam kết đã phát ra.

### Hệ quả đắt nhất: sáu dòng SLA trở thành KHÔNG VỚI TỚI ĐƯỢC ở kênh công dân

> **ĐÃ ĐƯỢC GỠ CÙNG NGÀY — ADR 0028 quyết định E.** Trần 56 giờ **không còn**, và cả 12 dòng
> SLA dùng được ở kênh công dân. Giữ nguyên mục này vì nó ghi **vì sao** cái trần từng tồn
> tại — một phiên đặt cả hai hạn lúc sinh phiếu sẽ dựng lại đúng nó. **Đừng hành động theo
> mục này; đọc ADR 0028 trước.** Phần vẫn còn đúng: ba dòng `2 giờ` của cột **Tiếp nhận** vẫn
> không với tới được ở kênh công dân (ADR 0028 §Ba cái giá, mục b).

Đây là phần khách nên được nghe lại trước khi mã chạy thật, vì nó không lộ ra từ câu hỏi.

`ubiquitous-language.md` ghi lĩnh vực do **cán bộ** xác định ở bước Phân loại, không để dân tự
chọn; và bộ đặc tả không có màn gửi phiếu của Mini App để đọc ngược lại. Vậy ở kênh công dân,
lúc tiếp nhận **chưa ai biết lĩnh vực**, nên hạn đầu tiên chỉ có thể lấy từ dòng mặc định của
bảng SLA — `docs/ui-ux/14-cau-hinh.md` §8 ghi 8 giờ / 56 giờ cho phản ánh.

Ghép với quyết định C: **dòng mặc định trở thành TRẦN cho mọi phiếu dân tự gửi.** Sáu dòng
trong bảng SLA dài hơn 56 giờ — `Hạ tầng giao thông` 168 giờ, `Thái độ / tác phong cán bộ`
120 giờ, và bốn dòng 72 giờ — không bao giờ được áp ở kênh ấy. Chúng chỉ áp được cho phiếu
**nhập hộ**, vì biểu mẫu nhập hộ bắt buộc chọn lĩnh vực ngay lúc vào sổ (`09 §11`).

Hai phiếu về cùng một ổ gà, một do dân gửi qua Mini App và một do trưởng thôn nhập hộ, vì thế
mang **hai hạn khác nhau** — 56 giờ và 168 giờ. Không có gì sai trong từng bước; nó là tổng
của hai quyết định đều đúng.

Ba đường ra, và **cả ba đều là câu của khách**, không phải của người viết mã: cho dân chọn
lĩnh vực lúc gửi (câu mở **#23**); nâng dòng mặc định lên cho bằng dòng dài nhất, tức hứa
rộng hơn với mọi phiếu; hoặc chấp nhận trần 56 giờ và nói điều đó ra với xã. **Không viết mã
theo đường nào trước khi có trả lời.**

## Quyết định D — cả hai đồng hồ khởi động lúc dân bấm gửi

| # | |
|---|---|
| 1 | `count_from` của **cả hai** đồng hồ — `Tiếp nhận` và `Xử lý xong` — là **thời điểm công dân bấm gửi** |
| 2 | Trạng thái đầu tiên là **tự động**: phần mềm sinh phiếu, không cán bộ nào tham gia |
| 3 | Đồng hồ `Tiếp nhận` dừng khi **có cán bộ động vào phiếu** — lần chuyển sang bước phân loại — chứ không phải lúc phiếu được sinh ra |

**Vì sao #3 là phần có ý nghĩa.** Đo từ lúc gửi tới lúc phiếu tồn tại là đo phần mềm với chính
nó: khoảng ấy luôn gần bằng 0, nên con số "tiếp nhận đúng hạn" sẽ đẹp tuyệt đối ở mọi xã và
**không nói gì về việc có ai đọc phiếu hay không**. Mốc `Tiếp nhận` trong bảng SLA là một hạn
đặt lên **một hành vi của con người**; đo sai chỗ thì nó đo một thứ không ai quan tâm, trong
khi vẫn hiện ra màn hình như một chỉ số đang được canh.

Trường hợp gửi ngoài giờ hành chính không lặp lại ở đây: ADR 0007 quyết định 8 đã chốt điểm
bắt đầu đếm, và nó áp cho `count_from` này y như mọi mốc khác.

**Hệ quả thứ hai, cùng hình dạng với hệ quả của quyết định C:** hành vi làm dừng đồng hồ
`Tiếp nhận` **chính là** hành vi xác định lĩnh vực. Nên ở kênh công dân, hạn tiếp nhận cũng
chỉ có thể là 8 giờ của dòng mặc định — ba dòng `2 giờ` của bảng SLA (`An ninh trật tự`,
`Điện`, `An toàn thực phẩm`) không với tới được. Con số 2 giờ ấy là ví dụ mà ADR 0007 dùng để
bác đơn vị ngày; nó vẫn đúng cho phiếu nhập hộ, và chưa dùng được cho phiếu dân tự gửi.

**Phiếu nhập hộ không có động tác "bấm gửi"**, nên mốc tương đương và ý nghĩa của đồng hồ
`Tiếp nhận` trên kênh ấy là câu mở **#24** — chưa trả lời, đừng suy ra.

## Phải trả

- **Trả ngay:** máy trạng thái viết được bằng nhánh mã, test được, và không phải đợi cấu hình
  của xã — đây là thứ quyết định B mua
- **Trả ngay:** đường phân loại phải đọc lại hạn, so sánh, ghi vết cả hai giá trị. Một lần
  ghi thêm trong cùng giao dịch với lần đổi lĩnh vực (luật 6 bất biến 3)
- **Trả sau:** hai kênh tiếp nhận cho hai hạn khác nhau trên cùng một vụ việc, cho tới khi
  #23 có trả lời. Báo cáo đúng hạn/quá hạn phải tách được theo kênh, nếu không thì con số
  trộn hai thang đo
- **Không mua được:** chín trạng thái cố định **không** làm xã nào bớt việc. Thứ chúng mua là
  không xã nào tự tay làm vòng đời phiếu ngừng chạy

## ĐIỀU KIỆN DỪNG

1. Thêm hoặc bớt một trạng thái của phiếu phản ánh — luật 10 điều kiện dừng #2, và quyết định
   B không phải giấy phép, nó là bản chốt danh sách
2. Bất kỳ đường nào **kéo dài** một hạn đã phát ra, kể cả khi lĩnh vực mới có SLA dài hơn
3. Áp quyết định B cho vòng đời **nhiệm vụ** — đó là câu mở #21
4. Viết đường tiếp nhận của kênh nhập hộ khi #24 chưa có trả lời
5. Cho dân chọn lĩnh vực lúc gửi "vì như thế hạn mới đúng" — đó là #23, và nó đổi cả mô hình
   phân loại (`ubiquitous-language.md` ghi lĩnh vực là việc của cán bộ)

## Bổ sung 2026-09-20 (muộn hơn trong ngày) — khách DUYỆT nguyên văn chín chuỗi mã

**Trạng thái:** đã chốt · **Ngày:** 2026-09-20

Quyết định B chốt **danh sách** chín trạng thái và chốt **mã viết tiếng Việt không dấu**;
cách viết từng chuỗi khi ấy vẫn là thứ phía thi công gõ ra từ nhãn tiếng Việt. Phiên này đưa
đúng chín chuỗi cho khách đọc lại. Khách trả lời: *"ok làm luôn"*.

`da-tiep-nhan` · `dang-phan-loai` · `da-chuyen-xu-ly` · `dang-xu-ly` · `da-xu-ly` ·
`cho-dan-xac-nhan` · `da-dong` · `khong-tiep-nhan` · `chuyen-cap-tren`

**Từ nay đổi một trong chín chuỗi là DI TRÚ HỒ SƠ LƯU TRỮ (luật 7), không phải đổi tên.**
Trước hôm nay còn một dịp rẻ — đọc lại trước khi migration đầu tiên chạm cột `trang_thai`.
Dịp ấy đã dùng. Mọi cảnh báo "chưa duyệt từng ký tự" trong `kb/` và trong
`docs/ui-ux/09-phan-anh-nguoi-dan.md` **đã gỡ cùng ngày**; gặp lại một cảnh báo như thế ở đâu
đó thì đó là bản sao sót lại, không phải một nghi ngờ còn sống.

## Bổ sung 2026-09-20 (cuối ngày) — MẶC ĐỊNH TẠM: xã KHÔNG đổi nhãn chín trạng thái

Khách được hỏi *"xã có được đổi nhãn của chín trạng thái không"* và trả lời: **để mặc định,
sẽ cập nhật khi có màn hình web hoàn chỉnh và nhận phản hồi từ các bên liên quan.**

**Mặc định: KHÔNG.** Chín nhãn là chuỗi của phần mềm, giống chín mã.

**Vì sao chặt hơn nhãn lĩnh vực, dù hai thứ nghe giống nhau.** Nhãn lĩnh vực chỉ sống trong
một xã. Nhãn trạng thái thì không:

| | |
|---|---|
| Công dân đọc nó | trên màn tra cứu của chính họ, và một người ở hai xã sẽ thấy hai chữ cho cùng một bước |
| Báo cáo xuyên xã đọc nó | cấp trên tổng hợp theo trạng thái thì 200 cách gọi là 200 cột |
| Hỗ trợ và tập huấn đọc nó | một tài liệu hướng dẫn không viết được nếu mỗi xã một chữ |

Nới ra sau thì **rẻ** — thêm một bảng nhãn theo xã là thao tác cộng thêm. Thu lại thì **đắt**:
phải đi hỏi từng xã rằng chữ họ đang dùng nay không dùng nữa. Đó là lý do mặc định nằm ở
phía chặt.

**MỞ LẠI KHI NÀO:** có màn hình web hoàn chỉnh và có phản hồi các bên.

## Bổ sung 2026-09-20 (muộn hơn trong ngày) — #23 và #24 đã đóng, ADR 0028

**Ba điều đổi ở tệp này, và đúng ba điều đó:**

| # | |
|---|---|
| 1 | **Trần 56 giờ không còn.** ADR 0028 quyết định E tách *thời điểm ĐẶT hạn* khỏi *gốc đếm*: `han_xu_ly_xong` của phiếu dân tự gửi đặt **lúc chốt lĩnh vực**. §"Hệ quả đắt nhất" ở trên nay chỉ còn giá trị **lịch sử** |
| 2 | **Quyết định D KHÔNG đổi.** Gốc đếm của cả hai đồng hồ vẫn là lúc dân bấm gửi; thời gian nằm chờ phân loại vẫn bị trừ vào hạn xử lý |
| 3 | **Quyết định C KHÔNG đổi**, chỉ được nói rõ phạm vi: nó áp cho mọi lần **đổi** lĩnh vực **sau** lần chốt đầu tiên. Lần chốt đầu tiên là lần *ấn định*, không phải lần *đổi* |

**Điều kiện dừng #4 và #5 ở trên được thay:** #24 và #23 đã có trả lời, nên hai dòng ấy nay
đọc là *"làm trái ADR 0028"* chứ không còn là *"chưa ai trả lời"*. Danh sách điều kiện dừng
đang có hiệu lực cho hai câu đó nằm ở ADR 0028.

→ ADR 0007 (giờ làm việc, ba bảng lịch, điểm bắt đầu đếm khi gửi ngoài giờ): `kb/10-decisions/0007-sla-working-hours.md`
→ ADR 0008 (cờ đóng phiếu và mở lại, tiền lệ ghi hạn mới): `kb/10-decisions/0008-petition-lifecycle-config.md`
→ ADR 0011 (vì sao mã giữ tiếng Việt): `kb/10-decisions/0011-contract-surface-language.md`
→ ADR 0026 (lĩnh vực hai tầng, quyết định cùng ngày): `kb/10-decisions/0026-linh-vuc-phan-anh-hai-tang.md`
→ ADR 0028 (mốc ĐẶT hạn — đóng #23 và #24, gỡ trần 56 giờ): `kb/10-decisions/0028-moc-dat-han-hai-dong-ho.md`
→ Bảng mã trạng thái: `kb/00-foundation/ubiquitous-language.md`
→ Câu mở #21 · #23 · #24: `kb/00-foundation/open-questions.json`
→ Luật 10: `.claude/rules/critical/10-citizen-commitment.md`
→ Kỹ năng: `.claude/skills/petition-lifecycle/SKILL.md`
