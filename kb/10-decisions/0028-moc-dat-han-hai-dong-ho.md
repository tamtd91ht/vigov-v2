---
id: 0028-moc-dat-han-hai-dong-ho
tier: T1
source: CURATED
owner: domain
derived_from_commit: 3d43fa1
expires: null
owns_facts:
  - "hai đồng hồ hạn của phiếu phản ánh được ĐẶT ở hai thời điểm khác nhau, chung một gốc đếm"
  - "hạn xử lý xong của phiếu dân tự gửi đặt lúc cán bộ chốt lĩnh vực, không phải lúc sinh phiếu"
  - "mốc khởi động đồng hồ của phiếu nhập hộ và khoảng chặn bảy ngày"
  - "vì sao hạn tiếp nhận của phiếu nhập hộ ghi NULL chứ không ghi 0 giờ"
  - "nghĩa của chữ 'lúc tiếp nhận' trong luật 10 bất biến 2 khi một phiếu có hai đồng hồ"
---

# 0028. Hai đồng hồ đặt ở HAI thời điểm — và mốc đếm của phiếu nhập hộ

**Trạng thái:** đã chốt · **Ngày:** 2026-09-20 · **Đóng câu mở #23 và #24** · **Nối tiếp ADR 0027**

**Cả hai quyết định dưới đây là QUYẾT THEO UỶ QUYỀN, không phải khách tự chọn.** Với #23
khách nói *"theo rule chung của hành chính xã, bên bạn quyết"*; với #24 khách nói *"bạn đề
xuất đi"*. Ghi ra vì đó là thông tin của người đọc sau: một quyết định do phía thi công đặt
ra theo lệ hành chính **mở lại rẻ hơn** một quyết định khách đã ký bằng con số của mình —
nhưng chỉ rẻ hơn **cho tới phiếu đầu tiên có hạn thật**, sau đó thì hai loại bằng nhau
(luật 10 bất biến 2).

## Bối cảnh

ADR 0027 quyết định C (hạn chỉ rút ngắn) và quyết định D (cả hai đồng hồ đếm từ lúc dân bấm
gửi) đều đúng, nhưng **tổng của chúng** tạo ra một hệ quả không ai chọn: ở kênh công dân,
lúc sinh phiếu chưa ai biết lĩnh vực, nên hạn đầu tiên chỉ lấy được từ **dòng mặc định** của
bảng SLA — và vì hạn chỉ được rút ngắn, dòng mặc định thành **TRẦN**. Sáu dòng SLA dài hơn
56 giờ và ba dòng tiếp nhận 2 giờ trở thành không với tới được ở kênh ấy.

Chỗ hỏng thật sự **không nằm ở gốc đếm**, nó nằm ở chỗ ADR 0027 ngầm giả định cả hai hạn
phải được **ĐẶT** cùng một lúc — lúc chèn dòng. Tháo đúng giả định ấy ra thì trần biến mất
mà không phải sửa quyết định C, không phải sửa quyết định D, và không phải đổi mô hình phân
loại.

## Quyết định E — hai đồng hồ ĐẶT ở hai thời điểm, CHUNG một gốc đếm (#23)

| Đồng hồ | Đặt lúc nào | Lấy số ở đâu | Đếm từ |
|---|---|---|---|
| `han_tiep_nhan` | **sinh phiếu** | `gio_tiep_nhan` của **dòng mặc định** | `count_from` của ADR 0027 quyết định D |
| `han_xu_ly_xong` | **lúc chuyển sang `dang-phan-loai`**, khi cán bộ chốt lĩnh vực | `gio_xu_ly_xong` của **lĩnh vực đã chốt** | **cùng** `count_from` ấy — lúc dân bấm gửi |

Ba điều phải đọc kèm, không được tách rời:

| # | |
|---|---|
| 1 | **Gốc đếm không đổi.** Quyết định D vẫn nguyên: cả hai hạn đếm từ lúc dân bấm gửi. Thứ dời đi là **thời điểm ấn định**, không phải thời điểm bắt đầu đếm. Thời gian phiếu nằm chờ phân loại **bị trừ vào** hạn xử lý, đúng như trước |
| 2 | **Phiếu nhập hộ đặt cả hai cùng lúc**, vì biểu mẫu nhập hộ bắt buộc chọn lĩnh vực ngay lúc vào sổ (`docs/ui-ux/09-phan-anh-nguoi-dan.md` §11) — xem quyết định F |
| 3 | **Quyết định C vẫn áp dụng cho mọi lần đổi lĩnh vực SAU khi đã chốt.** Lần chốt đầu tiên là lần *ấn định*, không phải lần *đổi*; từ lần thứ hai trở đi hạn chỉ được rút ngắn |

Quy tắc chung rút ra — viết theo **hình dạng biểu mẫu**, không theo danh sách kênh, để kênh
thứ năm không phải sửa lại luật:

> Kênh nào có **lĩnh vực trong biểu mẫu lúc vào sổ** thì đặt cả hai hạn ngay lúc ấy. Kênh
> nào không có thì `han_xu_ly_xong` **hoãn tới lần chốt lĩnh vực đầu tiên**.

Hôm nay `kenh_tiep_nhan` có bốn giá trị (`09 §12`): `can-bo-nhap-ho` rơi vào vế đầu,
`zalo-mini-app` · `zalo-oa` · `web-xa` rơi vào vế sau.

### Vì sao lệ hành chính cho ra đúng mốc này

`Phiếu tiếp nhận và hẹn trả kết quả` của bộ phận một cửa ghi **ngày hẹn trả** sau khi cán bộ
đã nhận và **kiểm** hồ sơ — không phải lúc tờ giấy chạm mặt bàn. Lệ ấy không phải thủ tục
rườm rà: hẹn một ngày trả cho một hồ sơ **chưa ai đọc** là hứa một thứ không giữ được **một
cách có hệ thống**, và cơ quan nào cũng biết điều đó trước khi có phần mềm.

Ở đây cũng đúng như vậy: 2 giờ là con số cho `An ninh trật tự`, và chỉ người **đã đọc** phiếu
mới biết phiếu ấy có phải an ninh trật tự hay không.

### Hai đường KHÔNG chọn, và vì sao

| Đường | Vì sao bác |
|---|---|
| **Cho dân chọn lĩnh vực lúc gửi** | Ghép với quyết định C nó tạo một **lỗ khai thác thật**: dân chọn `An ninh trật tự` cho một ổ gà thì được 16 giờ, mà cán bộ phân loại lại **không kéo dài được** — nên mọi phiếu sẽ trôi về lĩnh vực khẩn nhất, và bảng SLA mất nghĩa từ dòng đầu tới dòng cuối. Nó còn đổi cả mô hình phân loại, vốn là việc của cán bộ |
| **Nâng dòng mặc định lên 168 giờ** | Đó là hứa **rộng hơn với MỌI phiếu**, kể cả phiếu đáng lẽ 16 giờ. Một xã không thể nói với dân "việc gì cũng bảy ngày" chỉ vì phần mềm chưa biết phân loại |

Đường được chọn không hứa rộng hơn với ai, không cho ai tự nâng mức khẩn của mình, và trả
lại đủ **12 dòng SLA** cho kênh công dân.

## Ba cái giá của quyết định E — đừng giấu cái nào

### (a) Có một khoảng phiếu dân gửi KHÔNG có hạn xử lý xong

Từ lúc gửi tới lúc phân loại, `han_xu_ly_xong` là `NULL`. Hệ quả cho màn hình công dân:

- Mini App nói **"sẽ được xem trong N giờ làm việc"** — N lấy từ `han_tiep_nhan`
- **Tuyệt đối không hiện một ngày trả kết quả bịa ra.** Một ngày hẹn hiện ra rồi đổi là
  đúng thứ quyết định C tồn tại để chặn, chỉ khác là nó xảy ra ở tầng giao diện

Khoảng trống ấy **có người canh**: đồng hồ `Tiếp nhận` chạy đúng trong khoảng đó và **quá hạn
được** — đó chính là việc của cột 2 giờ / 8 giờ. Một phiếu nằm chờ phân loại quá lâu không
"lọt sổ": nó hiện ra ở chỉ số tiếp nhận trễ hạn.

Cái nó **chưa** canh: phiếu chưa phân loại **không nằm trong mẫu số** của chỉ số *xử lý đúng
hạn*, nên một xã phân loại chậm lại có chỉ số xử lý **đẹp hơn**. Đó là **câu mở #26**, sinh ra
từ chính quyết định này, và nó là câu của khách — đừng tự chọn trần phân loại.

### (b) Cột `gio_tiep_nhan` = 2 giờ của ba lĩnh vực khẩn KHÔNG dùng tới ở kênh công dân

`An ninh trật tự` · `Điện` · `An toàn thực phẩm` có `gio_tiep_nhan` = 2 giờ
(`docs/ui-ux/14-cau-hinh.md` §8). Trên kênh công dân chúng **không bao giờ được áp**, vì
không ai biết phiếu thuộc lĩnh vực nào trước khi đọc — và quyết định E không sửa được điều
đó, nó chỉ cứu cột `gio_xu_ly_xong`.

Hệ quả bắt buộc cho màn hình gửi phiếu: **app phải nói rõ việc khẩn cấp thật thì gọi
113 / 114 / 115**, không gửi qua kênh phản ánh. Đây không phải lời khuyên giao diện — nó là
cách duy nhất khoảng cách giữa "2 giờ trong bảng" và "8 giờ trên thực tế" không rơi vào
người đang cần giúp.

### (c) Nó UỐN CHỮ của luật 10 bất biến 2 — phải viết ra, đừng để phiên sau tự hiểu

Luật 10 bất biến 2 viết: *"`sla_deadline` được tính **một lần, lúc tiếp nhận**, và được lưu.
Không bao giờ tính lại lúc đọc."*

Quyết định E **giữ nguyên phần cấm** và **uốn phần mô tả**:

| Phần của bất biến 2 | Sau quyết định E |
|---|---|
| Tính **một lần** | **giữ nguyên** — mỗi hạn tính đúng một lần |
| **Được lưu** | **giữ nguyên** — cả hai là cột, không phải biểu thức |
| Không tính lại **lúc đọc** | **giữ nguyên, và đây mới là phần có răng** |
| "Lúc tiếp nhận" | với `han_xu_ly_xong`, **"lúc tiếp nhận" nghĩa là HÀNH VI PHÂN LOẠI**, không phải lúc chèn dòng |

Một phiên đọc bất biến 2 theo nghĩa đen "lúc chèn dòng" sẽ viết lại đúng cái trần 56 giờ mà
ADR này vừa tháo, và sẽ **tưởng mình đang tuân luật**. Đó là lý do mục này tồn tại.

Cách đọc đúng, một câu: **hạn được ấn định tại HÀNH VI ẤN ĐỊNH NÓ, và sau đó không đổi trừ
khi một hành vi nghiệp vụ của con người rút ngắn nó** (quyết định C, ghi vết trước/sau theo
luật 6 bất biến 5). Tiền lệ đã có ở ADR 0008: `tinh_lai_han_khi_mo_lai` ghi một hạn mới lúc
mở lại phiếu — cột hạn chưa bao giờ là cột bất động; thứ bị cấm là **nguồn thứ hai** cho cùng
một con số.

→ Đề xuất sửa câu chữ của bất biến 2 đã gửi người dùng cùng ngày; **chưa sửa tệp luật**.

## Quyết định F — mốc đếm của phiếu nhập hộ (#24)

| # | Quyết định |
|---|---|
| 1 | `count_from` = **thời điểm dân thật sự phản ánh**, nếu cán bộ ghi được. Modal Nhập hộ có thêm một trường **tuỳ chọn** "dân phản ánh lúc" |
| 2 | Trống thì `count_from` = **lúc vào sổ** |
| 3 | **Chặn hai đầu:** không sớm hơn **7 ngày** trước lúc vào sổ, không muộn hơn lúc vào sổ. Ngoài khoảng thì **TỪ CHỐI** — không cắt về biên, không lặng lẽ lấy lúc vào sổ |
| 4 | Mọi giá trị khác mặc định (tức mọi lần cán bộ tự gõ mốc) đều **GHI VẾT, có trước/sau** (luật 6 bất biến 5) |
| 5 | `han_tiep_nhan` của phiếu nhập hộ ghi **`NULL`** kèm lý do "không áp dụng". **Tuyệt đối không ghi 0 giờ** |

**Vì sao phải chặn hai đầu.** Một mốc quá khứ tuỳ ý là quyền **chế ra một phiếu đã quá hạn**
cho người khác, hoặc **giấu một phiếu đã trễ** bằng cách lùi mốc — cả hai đều là sửa số liệu
bằng một ô nhập liệu, không cần lỗi phần mềm nào. Bảy ngày là khoảng còn nhớ được: dân gọi
điện tuần trước, trưởng thôn ghi sổ tay rồi mới nhập. Xa hơn thế thì mốc là **ước đoán**, và
một ước đoán đi vào cột hạn thì không phân biệt được với một con số đúng.

**Vì sao từ chối chứ không cắt về biên** — cùng nguyên tắc với ADR 0007 quyết định 9: cắt về
biên là phần mềm **im lặng đổi** thứ người dùng vừa gõ, và người gõ không biết hạn đang tính
theo mốc nào.

### `han_tiep_nhan = NULL`, và vì sao 0 giờ là con số nguy hiểm

Đồng hồ `Tiếp nhận` đo **bao lâu thì có cán bộ đọc phiếu**. Với phiếu nhập hộ, **chính cán bộ
là người đọc và vào sổ** — khoảng ấy không tồn tại, nên đồng hồ không có nghĩa.

Ghi `0 giờ` thì mỗi phiếu nhập hộ trở thành một mẫu "tiếp nhận tức thì" hợp lệ trong mọi phép
trung bình. Một xã nhập hộ nhiều sẽ có **"thời gian tiếp nhận trung bình" gần 0** — một con số
**đẹp và SAI** gửi lên lãnh đạo, và không ai đọc ra được cái sai từ chính con số ấy. `NULL`
thì phép trung bình **buộc** phải nói nó bỏ qua bao nhiêu dòng.

**Ràng buộc kèm theo, không tách rời quyết định:** mọi báo cáo về **thời gian tiếp nhận** phải
**LOẠI** các dòng `NULL` khỏi mẫu số, **không được coi chúng là 0**. Một dòng `NULL` bị
`COALESCE(han_tiep_nhan, 0)` kéo về 0 là đúng cái hỏng này quay lại, lần này nằm trong một
truy vấn không ai đọc lại.

## Bổ sung 2026-09-20 (cuối ngày) — MẶC ĐỊNH TẠM: lĩnh vực đổi được SAU phân loại

Khách được hỏi *"lĩnh vực có đổi được sau bước Phân loại không"* và trả lời: **để mặc định,
sẽ cập nhật khi có màn hình web hoàn chỉnh và nhận phản hồi từ các bên liên quan.**

**Mặc định: ĐƯỢC**, và không cần luật mới — quyết định C đã đủ:

| | |
|---|---|
| Hạn sau khi đổi | vẫn là mốc **SỚM HƠN** trong hai mốc. Chỉ rút ngắn |
| Gốc đếm | không đổi — vẫn là lúc dân bấm gửi |
| Mỗi lần đổi | một dòng ghi vết, có lĩnh vực trước và sau (luật 6 bất biến 5) |
| Quyền | cùng quyền với bước phân loại |

**Vì sao mở chứ không khoá.** Bản chất thật của một vụ việc lộ ra **trong lúc xử lý**: một
phiếu vào sổ là *"ô nhiễm"* hoá ra là xây dựng không phép. Khoá lại thì cán bộ có đúng hai
đường, và cả hai đều tệ hơn: đóng phiếu rồi mở phiếu mới — **đếm trùng một vụ việc**, đúng
thứ ADR 0008 quyết định #8 sinh ra để tránh; hoặc để nguyên lĩnh vực sai — và **báo cáo theo
lĩnh vực của cả xã sai theo**.

**Chiều `min` là thứ làm việc mở này an toàn.** Đổi lĩnh vực bao nhiêu lần cũng được, hạn chỉ
đi xuống. Không thao tác nào trên màn hình nới được cam kết đã phát ra cho dân.

**Một chỗ mã phải chặn:** đổi lĩnh vực **sau khi phiếu đã đóng**. Lúc ấy không còn cam kết
nào đang chạy, và sửa lĩnh vực của một phiếu đã đóng là **sửa hồ sơ lưu trữ** (luật 7 cấm #5).
Sai số thống kê thì sửa bằng một phiếu đính chính, không bằng một lần `UPDATE`.

**MỞ LẠI KHI NÀO:** có màn hình web hoàn chỉnh và có phản hồi các bên.

## Phải trả

- **Trả ngay:** `phieu_phan_anh` có **hai cột hạn**, không phải một. `docs/ui-ux/09 §12` hiện
  ghi một cột `han_xu_ly` — đặc tả thiếu cột thứ hai, và thiếu trường "dân phản ánh lúc" ở
  modal Nhập hộ (`09 §11`). Hai thứ ấy phải bổ sung trước khi dựng màn hình
- **Trả ngay:** `han_xu_ly_xong` **cho phép NULL**, và mọi màn hình hiện hạn phải chịu được
  `NULL` — danh sách, thẻ phiếu, bộ lọc "sắp đến hạn", job nhắc việc
- **Trả ngay:** `han_tiep_nhan` cũng **cho phép NULL**, với nghĩa **khác hẳn**: "không áp
  dụng" chứ không phải "chưa có". Hai lý do `NULL` khác nhau trên hai cột là chỗ một phiên sau
  sẽ nhầm nếu không đọc bảng thuật ngữ
- **Trả sau:** báo cáo đúng hạn/quá hạn phải **tách được theo kênh**, và phải loại `NULL` đúng
  chỗ. Trộn hai loại `NULL` vào một phép trung bình là cách con số hỏng mà vẫn hiện ra
- **Không mua được:** quyết định E **không** làm phiếu nào được xử lý nhanh hơn. Thứ nó mua là
  **con số hạn nói đúng thứ nó nói** — và 12 dòng SLA khách đã khai được dùng thật

## ĐIỀU KIỆN DỪNG

1. Đặt `han_xu_ly_xong` **lúc sinh phiếu** cho phiếu dân tự gửi "cho đơn giản" — đó là dựng
   lại trần 56 giờ, và lần này không ai nhìn ra vì mã trông hợp lý
2. Ghi **0 giờ** (hay bất kỳ giá trị nào khác `NULL`) vào `han_tiep_nhan` của phiếu nhập hộ
3. `COALESCE` một cột hạn về 0 trong bất kỳ truy vấn báo cáo nào
4. Nới khoảng chặn 7 ngày, hoặc **cắt về biên** thay vì từ chối
5. Cho dân chọn lĩnh vực lúc gửi — #23 đã đóng theo hướng ngược lại, và lý do là một lỗ khai
   thác, không phải sở thích
6. Thêm một `kenh_tiep_nhan` thứ năm mà **không** trả lời trước: biểu mẫu của kênh ấy có
   lĩnh vực lúc vào sổ hay không

→ ADR 0027 (chín trạng thái, quyết định C và D, và cái trần mà ADR này tháo): `kb/10-decisions/0027-trang-thai-va-dong-ho-phieu-phan-anh.md`
→ ADR 0007 (giờ làm việc, điểm bắt đầu đếm khi gửi ngoài giờ, và lệ "từ chối chứ không đoán"): `kb/10-decisions/0007-sla-working-hours.md`
→ ADR 0008 (tiền lệ ghi một hạn mới khi mở lại phiếu): `kb/10-decisions/0008-petition-lifecycle-config.md`
→ ADR 0026 (bộ mã lĩnh vực hai tầng): `kb/10-decisions/0026-linh-vuc-phan-anh-hai-tang.md`
→ Tên trong mã của hai cột hạn: `kb/00-foundation/ubiquitous-language.md`
→ Bảng SLA 12 dòng và dòng mặc định: `docs/ui-ux/14-cau-hinh.md` §8
→ Modal Nhập hộ: `docs/ui-ux/09-phan-anh-nguoi-dan.md` §11
→ Câu mở #26 (phiếu chưa phân loại và mẫu số của chỉ số xử lý): `kb/00-foundation/open-questions.json`
→ Luật 10: `.claude/rules/critical/10-citizen-commitment.md`
→ Luật 6 (ghi vết trước/sau): `.claude/rules/critical/6-audit-log.md`
→ Kỹ năng: `.claude/skills/petition-lifecycle/SKILL.md`
