---
id: 0035-muoi-lam-cau-mo-quyet-theo-thuc-te-cap-xa
tier: T1
source: CURATED
owner: architecture
derived_from_commit: 4c96e9a
expires: null
owns_facts:
  - "15 câu mở còn lại được quyết ngày 22/09/2026 bằng ĐỀ XUẤT CỦA NHÀ CUNG CẤP, không phải trả lời của khách — và điều đó nghĩa là gì"
  - "quy tắc chọn hướng khi tự quyết: hướng NỚI ĐƯỢC VỀ SAU, không phải hướng đúng nhất"
  - "vì sao `Cân đối thu - chi` lấy `Thu xã hưởng` chứ không lấy `Thu ngân sách NSNN`"
  - "vì sao `Thu đạt dự toán` chia cho `Dự toán TP giao` chứ không chia cho `Dự toán Xã giao`"
  - "vì sao khoản hoàn là CHỨNG TỪ RIÊNG chứ không phải một số âm"
  - "trần bắt buộc phân loại phiếu phản ánh, và mẫu số của chỉ số xử lý đúng hạn"
---

# ADR 0035 — Mười lăm câu mở, quyết theo thực tế vận hành cấp xã

**Ngày:** 22/09/2026 · **Trạng thái:** ĐỀ XUẤT CỦA NHÀ CUNG CẤP, ĐANG THI HÀNH

---

## Bối cảnh, và điều phải đọc trước mọi dòng còn lại

Tới 22/09/2026 còn **15 câu mở**, và chúng chặn **6 trong 13 phân hệ nghiệp vụ**. Chủ dự án
giao lại quyền quyết: *"tự đề xuất theo kinh nghiệm vận hành mô hình cấp xã, không treo quá
nhiều làm gián đoạn code"*.

**ĐÂY LÀ ĐỀ XUẤT CỦA NHÀ CUNG CẤP, KHÔNG PHẢI TRẢ LỜI CỦA KHÁCH — và sự khác nhau ấy không
phải chuyện chữ nghĩa.** Một câu khách đã chốt thì nhà cung cấp làm theo và không chịu trách
nhiệm về lựa chọn; một câu nhà cung cấp tự quyết thì trách nhiệm nằm ở đây, và người dùng đầu
tiên phát hiện nó sai sẽ là một cán bộ xã đang làm việc thật. Vì vậy mọi mục dưới đây ghi rõ
**hướng đã chọn · bằng chứng · và cái giá nếu chọn sai**, để một lần phủ quyết của khách chỉ
cần đọc một chỗ.

## Quy tắc đã dùng để chọn, và nó KHÔNG phải "chọn phương án đúng nhất"

> **Chọn hướng NỚI ĐƯỢC VỀ SAU, không chọn hướng có vẻ đúng nhất hôm nay.**

Khi không có khách để hỏi, thứ phân biệt được một quyết định chấp nhận được với một quyết định
liều là **tính đối xứng của cái giá**. Hai hướng cùng có thể sai; hướng đúng để chọn là hướng
mà lần sửa sau này là một migration, chứ không phải một lần sửa hồ sơ lưu trữ hay một văn bản
đính chính gửi lên cấp trên.

Cụ thể, ba thứ luôn chọn phía chặt hơn:

| Loại | Chọn phía nào | Vì sao |
|---|---|---|
| **Ghi vết** | Luôn GHI, kể cả khi chưa chắc có cần | Vết chỉ ghi được từ ngày có nó trở đi. Khoảng trống trước đó không dựng lại được từ bất cứ nguồn nào |
| **Ràng buộc** | Luôn CHẶT, nới sau | Nới là một dòng. Siết lại sau khi đã chạy vài tháng thì dữ liệu mang trạng thái trộn lẫn và không cột nào phân biệt được |
| **Bộ mã / khoá quyền** | Luôn ĐÓNG, mở sau | Sau khi vài xã đã tự thêm mã riêng, thu về một bộ chung là ánh xạ thủ công trên hồ sơ ĐÃ ĐÓNG (luật 7) |

---

## A. HAI CON SỐ BÁO CÁO — phần nặng nhất, và cả hai mâu thuẫn nằm TRONG cùng một tệp đặc tả

Đây là hai câu đắt nhất trong mười lăm câu, vì sai ở đây không phải một lỗi phần mềm: nó là
**một con số đã nằm trong văn bản xã gửi lên cấp trên**, và sửa nghĩa là xã phải đính chính một
văn bản đã phát hành. Chúng rẻ nhất đúng vào hôm nay, khi chưa có kỳ báo cáo nào.

### #32 — `Cân đối thu - chi` lấy `Tổng thu` từ **`Thu xã hưởng`**

`docs/ui-ux/07-thu-chi-ngan-sach.md:231` nói số tổng lấy từ dòng đánh sao, mặc định dòng đầu —
ở tab Thu là `TỔNG THU NỘI ĐỊA PHÁT SINH TRÊN ĐỊA BÀN`. Đi theo câu ấy thì `Tổng thu` là **số
phát sinh trên địa bàn**, và ô Cân đối cho ra một con số DƯƠNG trong khi xã đang thiếu tiền.

**Lý do chọn `Thu xã hưởng`, và nó là lý do nghiệp vụ chứ không phải thẩm mỹ:** ngân sách xã
cân đối trên **nguồn thu xã được hưởng theo phân cấp**, không trên số phát sinh trên địa bàn.
Phần lớn khoản phát sinh trên địa bàn điều tiết lên huyện/tỉnh và **xã không được chi**. Một ô
"Cân đối thu - chi" dựng trên số phát sinh trả lời câu *"địa bàn này tạo ra bao nhiêu tiền"* —
một câu thật, nhưng không phải câu người đọc ô ấy đang hỏi. Câu họ hỏi là *"tiền về xã có đủ
cho khoản xã đã chi không"*.

Chênh lệch trong chính số liệu mẫu của đặc tả: `4.316.764,3` (`:69`) so với `3.300.800,5`
(`:70`) — **hơn một triệu đơn vị**, đủ để đảo dấu ô Cân đối.

> **Hệ quả bắt buộc cho giao diện:** màn hình phải hiện **CẢ HAI** số và gọi đúng tên từng số.
> Xã cần cả hai — một để báo cáo thu ngân sách, một để biết mình còn bao nhiêu. Thứ chỉ có MỘT
> là ô Cân đối, và nó lấy `Thu xã hưởng`.

### #33 — `Thu đạt dự toán` chia cho **`Dự toán TP giao`**

Cùng một tệp cho hai mẫu số, lệch 16 điểm phần trăm. Ô tóm tắt (`:67-71`) hiện `108,1%`, ra
được khi chia cho `Dự toán 2026 TP giao` 3.993.010. Quy tắc nghiệp vụ ở `:233` lại viết chia
cho `Dự toán Xã giao` 4.681.290, ra `92,2%`.

**Chọn `Dự toán TP giao`, vì đó là con số xã BỊ ĐÁNH GIÁ theo.** Dự toán thành phố/huyện giao
là chỉ tiêu cấp trên ấn định; `Dự toán Xã giao` là chỉ tiêu xã tự đặt cho mình, thường cao hơn
để phấn đấu. Chỉ số "đạt dự toán" đi vào báo cáo lên cấp trên phải đo theo chỉ tiêu cấp trên
giao — đo theo chỉ tiêu tự đặt thì mỗi xã có một thước riêng, và không cộng ngang được giữa
các xã.

Bằng chứng phụ nhưng đúng hướng: **số liệu mẫu của chính đặc tả nhất quán với `TP giao`**
(`:67-71` cho ra 108,1%), còn dòng `:233` thì không khớp với bất kỳ ô nào trong cùng tệp. Khi
một tệp tự mâu thuẫn, **bảng số liệu mẫu đáng tin hơn câu văn xuôi** — bảng ấy được dựng bằng
số thật và tự kiểm chứng.

`docs/ui-ux/01-tong-quan-dieu-hanh.md:94` chép `108.1%` lên `/tong-quan` — cùng hướng.

> **Hệ quả bắt buộc:** cột `Dự toán TP giao` trở thành cột **BẮT BUỘC có dữ liệu**. Xã bỏ trống
> nó thì chỉ số biến mất — và điều đó phải hiện ra thành một câu, không thành một ô trống.

### Điều KHÔNG được làm với hai câu này

**Không đặt một mặc định "tạm thời" rồi tính tiếp.** Nếu dữ liệu chưa đủ để ra số thì để trống
kèm câu nói rõ vì sao — **fail closed**. Một ô trống kèm lý do là một việc cần làm; một con số
sai là một văn bản phải đính chính.

---

## B. TIỀN VÀ CHỨNG TỪ

### #30 — Khoản hoàn là **CHỨNG TỪ RIÊNG**, không phải số âm

`0004…sql:305` đặt `CHECK (so_tien > 0)`, và chú thích ngay trên tự khai đây là câu của khách.

**Xã CÓ thu hồi tạm ứng — đó là việc thường xuyên, không phải ngoại lệ.** Nhưng mô hình đúng
không phải một dấu trừ: một chứng từ âm và một lần gõ nhầm dấu trông giống hệt nhau trong sổ, và
sáu tháng sau không ai phân biệt được. Khoản hoàn là **một sự kiện nghiệp vụ khác, có tên khác,
có người ký khác**.

Quyết: **`CHECK (so_tien > 0)` GIỮ NGUYÊN.** Khoản hoàn dựng thành loại chứng từ riêng, mang
**tham chiếu BẮT BUỘC tới chứng từ gốc**, số tiền vẫn dương, và tổng đã giải ngân của dự án
tính bằng `tổng(chi) − tổng(hoàn)`.

Hướng này nới được: thêm một loại chứng từ là một migration. Hướng ngược lại — đã cho số âm rồi
muốn thu về một dấu — là ánh xạ thủ công những dòng âm ĐÃ CÓ NGƯỜI KÝ, tức sửa hồ sơ lưu trữ.

### #29 — Mở khoá chứng từ: **bắt buộc lý do · không tự mở lại · đếm nhưng không đặt trần**

| Điểm | Quyết | Vì sao |
|---|---|---|
| Lý do mở khoá | **BẮT BUỘC**, cột riêng | Vết của những lần mở khoá đã xảy ra không dựng lại được. Đó đúng là con số thanh tra hỏi: con số đã có người ký rồi bị sửa |
| Người vừa khoá tự mở lại | **KHÔNG** | Cùng hình dạng #13 và #14 khách đã chốt cho danh bạ. Ở xã có một kế toán thì điều này nghĩa là chủ tịch hoặc kế toán trưởng phải mở — đúng ý định của một bước khoá |
| Trần số lần mở | **KHÔNG đặt**, nhưng **ĐẾM và lưu** | Một trần là con số của khách. Đếm thì không sai được, và nó là thứ để về sau khách nhìn số THẬT rồi mới chọn trần |

### #31 — `nguong_canh_bao_cham` là **CẤU HÌNH CỦA XÃ**, mặc định 10 điểm

Đặc tả nói ba lần rằng con số là của xã (`06-giai-ngan.md:255`, `:342`, `:45`), mà hôm nay nó
là hằng số trong kho mã nhà cung cấp — tức **luật 1 bất biến 10 đang bị vi phạm**: một giá trị
theo xã nằm trong bundle dùng chung.

Quyết theo đúng đặc tả. Và **ngưỡng đang hiệu lực lúc tính phải được LƯU cùng chỉ số**, không
tính lại lúc đọc — cùng lý do luật 10 bất biến 2 bắt lưu hạn: đổi ngưỡng về sau mà tính lại thì
những con số đã gửi lên lãnh đạo không dựng lại được.

### #34 — `Hạng mục kế hoạch vốn` giữ mô hình **DANH MỤC tham chiếu**

Mã đã dựng theo bản `14-cau-hinh.md:168` — `danh_muc(id, nhom, ma, nhan, thu_tu, …)`, không
năm, không tiền. Giữ nguyên.

Hệ quả phải thi hành chứ không được lờ đi: **hai cột `ke_hoach_von_nam` và `thoi_han_giai_ngan`
mà `06-giai-ngan.md:257-264` vẽ thì KHÔNG CÓ CHỖ LƯU**, nên màn hình **không được hiện chúng**.
Hiện một ô suy ra trong lúc chưa có bảng là tạo ra hai nghĩa trong cùng một ô — về sau không
phân biệt được số suy ra với số xã đặt tay.

---

## C. HỒ SƠ, TRẠNG THÁI, VÀ VẾT

### #21 — Trạng thái nhiệm vụ: xã sửa được **NHÃN và THỨ TỰ**, KHÔNG sửa được **DANH SÁCH MÃ**

Đặc tả tự mâu thuẫn: `02-nhiem-vu.md:214` gọi trạng thái là danh mục sửa được, còn `:227-231`
vẽ một máy trạng thái có hướng đi cố định và `:235` gắn `task.approve` vào đúng một bước.

Chọn phía đóng, vì **bất đối xứng**: đi từ "chỉ sửa nhãn" sang "sửa được cả mã" về sau là viết
lại máy trạng thái thành dữ liệu — lớn nhưng làm được. Chiều ngược lại thì không: sau khi vài
xã đã thêm mã riêng, thu về bộ mã chung là ánh xạ thủ công trạng thái của những nhiệm vụ **ĐÃ
ĐÓNG**.

Với một xã, cái mất gần như bằng không: xã đổi được chữ hiện trên màn hình và thứ tự cột, là
thứ họ thật sự muốn. Cái họ không làm được — thêm một trạng thái mới — cũng là thứ sẽ phá máy
trạng thái nếu cho làm.

**Hệ quả:** nút `Tắt` trên danh mục trạng thái nhiệm vụ phải **không có**. `14-cau-hinh.md:182`
nói mục nguồn hệ thống không xoá được nhưng ĐƯỢC TẮT — với nhóm này thì không: tắt một trạng
thái đang có nhiệm vụ nằm trong đó là làm những nhiệm vụ ấy rơi khỏi mọi bộ lọc.

### #19 — Sửa lời khai cư trú đã xác thực: **mất hiệu lực xác thực, và giữ LỊCH SỬ PHIÊN BẢN**

Quyết phía ghi, vì hướng ngược lại hỏng nặng hơn hẳn mất dữ liệu: nếu sửa là ghi đè lặng lẽ thì
một lời khai đã xác thực bị thay bằng nội dung khác **mà vẫn mang dấu "đã xác thực"** — tức tạo
ra một **xác nhận của cơ quan nhà nước cho một nội dung cán bộ chưa từng đọc**.

Hai nửa, và cả hai bắt buộc:
1. Công dân sửa → **cờ xác thực về `chưa xác thực`**, tự động, không hỏi.
2. Bản trước khi sửa **được giữ lại**. Lịch sử phiên bản không thêm được sau: những lần sửa đã
   xảy ra là mất vĩnh viễn.

### #25 — Vết của hành vi CHƯA THUỘC XÃ NÀO: **ghi, ở tầng nền tảng**

ADR 0020 bất biến 5 đòi mỗi lần đăng nhập công dân ghi vết; nhưng phiên công dân ra đời TRƯỚC
khi có xã, và `core/audit` đòi phạm vi xã.

Quyết: **bảng nhật ký riêng ở tầng nền tảng**, KHÔNG nới `core/audit.Write` nhận giao dịch
không phạm vi. Hai lý do, và lý do thứ hai mới là lý do chính:
1. Vết chỉ ghi được từ ngày có nó trở đi — không ghi là một khoảng trống vĩnh viễn, đúng khoảng
   thanh tra sẽ hỏi nếu kênh công dân có sự cố.
2. **Nới `core/audit` rồi muốn rút lại là đi sửa mọi chỗ đã dùng đường nới ở cả tám dịch vụ**, và
   một bức tường đã gỡ thì lần dựng lại phải chứng minh không còn ai đi vòng — khác hẳn lần đầu.
   Một bảng riêng thì gỡ được mà không ai phải chứng minh gì.

### #26 — Trần phân loại **1 NGÀY LÀM VIỆC**, và phiếu chưa phân loại **NẰM TRONG mẫu số**

Hai hệ quả của việc không làm gì, cả hai đều có thật:
1. Mẫu số "xử lý đúng hạn" không chứa phiếu chưa phân loại → **xã phân loại chậm lại có chỉ số
   ĐẸP HƠN**. Một con số thưởng cho đúng hành vi nó phải phạt.
2. Không mốc nào buộc phiếu rời `da-tiep-nhan` → phiếu nằm đó vô hạn với một người dân đang chờ.

Quyết:
- **Trần phân loại: 1 ngày làm việc** kể từ khi tiếp nhận, đếm bằng **giờ làm việc** qua
  `identity.AdvanceWorkingHours` (ADR 0007) — cùng đơn vị với mọi hạn khác, không phải ngày lịch.
- Phiếu quá trần mà chưa phân loại **tính là TRỄ** trong mẫu số. Nó phải đau ở đúng chỗ nó chậm.
- Một ngày là đủ ở cấp xã: phiếu về là cán bộ văn phòng đọc trong ngày, không cần hội ý liên
  ngành. Trần dài hơn thì không còn là trần.

⚠ Trần này **ấn định một hạn cho phiếu**, và luật 10 bất biến 2 không cho tính lại — mọi phiếu
nhận trước khi đổi ý mang thang cũ. Đổi trần về sau chỉ áp cho phiếu nhận từ lúc đổi trở đi.

---

## D. QUYỀN, XÃ, VÀ PHÁP NHÂN

### #4 — Đọc xuyên xã: **chỉ SỐ ĐẾM tổng hợp, và lĩnh vực `can-bo` KHÔNG rời khỏi xã**

Chọn phía chặt nhất còn dùng được:
- Cấp huyện/tỉnh đọc **số đếm theo lĩnh vực**, không tới từng phiếu. Nội dung phiếu là lời một
  người dân cụ thể gửi cho xã của họ.
- **Lĩnh vực `can-bo` (Thái độ / tác phong cán bộ) KHÔNG nằm trong bất kỳ số liệu nào rời khỏi
  xã.** Ngay trong xã nó đã cần `feedback.restricted` (`09:80`); để nó vào một con số cấp tỉnh
  là mở một cửa mà trong chính xã còn đang khoá.
- Phạm vi tính **từ cây tổ chức**, không nhận từ client. Mọi lần đọc **ghi vết** (luật 6 #7).

Nới về sau là một quyết định có chủ ý của khách; siết lại sau khi số liệu đã rời xã thì không
gọi về được.

### #20 — Khoá quyền xác thực lời khai cư trú: **`citizen.verify`**

Đặt theo đúng khuôn 35 khoá đang có: `<nhóm>.<việc>`, nhóm là danh từ chỉ đối tượng, việc là
động từ. Nhóm `citizen` là nhóm mới — đây là **nhóm thứ mười một** mà `14-cau-hinh.md:104` đếm
tới nhưng bảng dưới không liệt kê, tức nó khớp với khoảng trống của #27.

Cần một migration nạp khoá. **Rẻ đúng hôm nay** vì chưa xã nào chạy thật; sau đó là migration
trên dữ liệu phân quyền đang chạy.

### #27 — Chỉ nạp khoá nào có TUYẾN THẬT cần, không bịa đủ tám

Đặc tả đếm 43 quyền / 11 nhóm, bảng liệt 33 khoá / 10 nhóm; migration đã nạp 35.

**Không bịa tám khoá cho đủ số.** Một khoá không tuyến nào dùng là một dòng trong màn Phân quyền
mà không ai biết nó làm gì — và một quản trị viên xã sẽ tick nó. Chỉ nạp khi có tuyến thật:

| Khoá | Cho việc | Trạng thái |
|---|---|---|
| `citizen.verify` | cán bộ xác thực lời khai cư trú | nạp cùng #20 |
| `admin.user.delete` | xoá mềm một dòng danh bạ nhập trùng (#10) | nạp khi dựng tuyến |
| `admin.user.revoke` | thu hồi tài khoản đăng nhập, giữ dòng danh bạ | nạp khi dựng tuyến |

Con số 43 trong tiêu đề coi như **chưa giải thích được**, và ghi đúng như thế thay vì lấp đầy.

### #1 — Sáp nhập xã: `tenant_id` mờ đục + xã cũ **đánh dấu ngừng hoạt động**, dữ liệu giữ nguyên

Phần đắt nhất **đã đúng sẵn**: `tenant_id` là ULID mờ đục, không mang mã hành chính (luật 1 bất
biến 2), nên lần sáp nhập đầu tiên không buộc sửa khoá ngoại trên dữ liệu lịch sử.

Quyết phần còn lại: xã sáp nhập **đánh dấu ngừng hoạt động, dữ liệu và mã giữ nguyên** (luật 7
bất biến 6); xã mới nhận **`tenant_id` MỚI**; một bản ghi kế thừa nối hai xã cho đường tra cứu
lịch sử **chỉ đọc**. `ResolveTenantSuccession` trong `proto` đã có sẵn cho đúng việc này.

Không gộp dữ liệu hai xã vào một `tenant_id`: hồ sơ của xã cũ do xã cũ ban hành, và một hồ sơ
đổi cơ quan ban hành là sửa hồ sơ lưu trữ.

### #28 — Bên NHẬN dữ liệu khai với người dân: **ViHAT Group**

ADR 0031 đã chuyển Mini App sang ViHAT Group, và backend `vihat-miniapp` do cùng nhà cung cấp
vận hành. Khai `VihatSoftware` là bên nhận trong khi bên phát hành đã là ViHAT Group để lại
**hai pháp nhân trong một văn bản** mà người dân không nối được.

Quyết: **ViHAT Group** ở cả hai vế. Ba chuỗi phải sửa: `chinh-sach-rieng-tu.ts:157`, `:205`,
`:300`, cộng hai câu nằm NGOÀI tệp chính sách.

> ⚠ **ĐÂY LÀ MỤC DUY NHẤT CẦN NGƯỜI XÁC NHẬN TRƯỚC KHI PHÁT HÀNH**, và lý do khác mọi mục
> khác: nó là một **khẳng định pháp lý với người dân**, không phải một lựa chọn kỹ thuật. Nhà
> cung cấp không tự biết pháp nhân nào ký hợp đồng vận hành máy chủ.
>
> Sửa chữ thì **rẻ hôm nay** — bản chính sách vẫn là 1.0 và **chưa một người dùng nào đọc bản
> nào**. Từ lần phát hành đầu tiên thì quy tắc đảo ngược: đổi bên nhận dữ liệu là thay đổi về
> HÀNH VI XỬ LÝ DỮ LIỆU, phải lên số phiên bản mới, và **không gọi lại được từng người đã bấm
> đồng ý ở bản khai sai**.
>
> Chặn **nộp Zalo**, KHÔNG chặn viết mã.

---

## Hệ quả — cái gì đỏ nếu một mục ở đây bị phủ quyết

| Mục | Phủ quyết thì phải sửa gì |
|---|---|
| #32, #33 | Một truy vấn và một công thức. **Cộng đính chính mọi kỳ đã báo cáo** — nên phủ quyết TRƯỚC kỳ đầu tiên |
| #30 | Thêm loại chứng từ: migration. Nếu đổi sang số âm: ánh xạ thủ công mọi dòng đã ký |
| #21 | Từ đóng sang mở: viết lại máy trạng thái thành dữ liệu. Chiều ngược lại không làm được |
| #26 trần | Chỉ áp cho phiếu nhận từ lúc đổi trở đi. Phiếu cũ giữ thang cũ (luật 10 bất biến 2) |
| #20, #27 | Migration trên dữ liệu phân quyền. Rẻ tới khi xã đầu tiên gán khoá cho một vai trò |
| #28 | Trước phát hành: sửa chữ. Sau phát hành: phiên bản chính sách mới, không gọi lại được người đã đồng ý |
| #4 | Nới ra: một quyết định. Siết lại sau khi số liệu đã rời xã: không gọi về được |

---

→ Nguồn câu hỏi: `kb/00-foundation/open-questions.json`
→ Liên quan: ADR 0007 (giờ làm việc) · 0011 (đặt tên) · 0020 (xác thực công dân) · 0022 (rìa
kênh công dân) · 0028 (hai đồng hồ) · 0031 (pháp nhân Mini App)
