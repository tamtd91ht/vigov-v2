---
id: 0026-linh-vuc-phan-anh-hai-tang
tier: T1
source: CURATED
owner: architecture
derived_from_commit: 3d43fa1
expires: null
owns_facts:
  - "danh mục Lĩnh vực phản ánh chia hai tầng: bộ mã đóng do nền tảng cấp, nhãn do xã đặt lại"
  - "service giữ bộ mã lĩnh vực tầng 1 là platform, và thêm mã là thao tác của quản trị viên nhà cung cấp"
  - "vì sao xã không được thêm mã lĩnh vực trong khi bảy nhóm danh mục kia thì được"
  - "bảng nhãn lĩnh vực theo xã thuộc service petitions"
  - "bộ mã cấp xuống từ nền tảng không va chạm với ranh giới đọc của ADR 0003"
  - "phép thử thứ hai cho danh mục: con số mang mã này có phải cộng với xã khác không"
---

# 0026. `Lĩnh vực phản ánh`: bộ mã đóng ở tầng nền tảng, nhãn theo xã

**Trạng thái:** đã chốt · **Ngày:** 2026-09-20 · **Lấp ô trống §1 của ADR 0024**
**Trả lời MỘT PHẦN câu mở #4** — phần còn lại vẫn mở, xem §Điều ADR này KHÔNG đóng

## Bối cảnh

ADR 0024 chốt chủ sở hữu cho bảy nhóm danh mục tham chiếu và **cố ý để trống ba ô**. Ô thứ
nhất là `Lĩnh vực phản ánh` (12 mục, `docs/ui-ux/09-phan-anh-nguoi-dan.md` §5), và nó chờ
đúng một câu: cấp trên có cộng số liệu **theo lĩnh vực** xuyên 200+ xã hay không.

Ngày 2026-09-20 khách trả lời: **có**, và chọn hình dạng **"bộ mã đóng ở tầng nền tảng +
nhãn theo xã"** — đúng nhánh mà ADR 0024 đã mô tả sẵn là *"khác hẳn ADR này"*.

## Quyết định

**Danh mục này có HAI tầng, và chỉ tầng dưới thuộc về xã.**

| Tầng | Nội dung | Ai đổi được | `tenant_id` |
|---|---|---|---|
| **1 — bộ mã** | Tập mã đóng, 12 mã của `09 §5` làm giá trị khởi tạo | **Không phải xã.** Xã không thêm, không bớt mã | Không — bộ mã là một, dùng chung cho mọi xã |
| **2 — nhãn** | Chuỗi người đọc mà xã đặt lại cho từng mã | Xã | Có, khoá duy nhất `(tenant_id, ma)` |

Hệ quả một dòng cho người viết mã: **`phieu_phan_anh.linh_vuc` giữ mã của tầng 1**, và màn
hình hiển thị nhãn của tầng 2 nếu xã có khai, không thì nhãn mặc định của tầng 1.

Hình dạng chung của một bảng danh mục theo xã — khoá hợp thành, xoá mềm, hạt giống gieo theo
từng xã, phân mảnh — không chép lại ở đây: nó thuộc ADR 0024 §"Vì sao MỌI bảng ở đây mang
`tenant_id`". Tầng 2 theo đúng hình dạng ấy. Tầng 1 thì **không**, vì nó không phải dữ liệu
của xã.

## Vì sao phép thử của ADR 0024 cho ra kết quả khác ở đây

ADR 0024 hỏi: *xã thêm một dòng vào đây thì phần mềm có làm gì khác không?* Với lĩnh vực phản
ánh câu trả lời là **không** — thêm một lĩnh vực thì ô chọn có thêm một dòng, không nhánh mã
nào phải biết trước. Theo đúng phép thử ấy thì đây là danh mục của xã, xã sửa được, y như
`Loại văn bản`.

Phép thử ấy **không sai, nó thiếu một vế**. Nó đo *phần mềm* có phản ứng không, mà bỏ qua chỗ
giá trị đi tới:

> **Phép thử thứ hai — con số mang mã này có phải cộng với con số của xã khác không?**

| Trả lời | Hệ quả |
|---|---|
| **Không** | Mã chỉ có nghĩa bên trong một xã. Xã tự đặt, đúng như bảy nhóm của ADR 0024 |
| **Có** | Mã là **đơn vị đo dùng chung**. Hai xã gọi rác thải bằng hai mã khác nhau thì phép cộng không có nghĩa, và **không có cách sửa rẻ nào sau đó** |

`Loại văn bản` trả lời **không**: sổ văn bản của xã A không cộng với sổ của xã B. `Lĩnh vực
phản ánh` vừa được khách trả lời **có**. Đó là toàn bộ khác biệt, và nó nằm ở **đường đi của
số liệu**, không nằm ở hình dạng của bảng.

**Vì sao không để xã tự đặt mã rồi ánh xạ sau:** ánh xạ 200+ bộ mã tự phát về một bộ là việc
làm tay, làm trên **dữ liệu đã đóng hồ sơ**, và luật 7 không cho sửa hồ sơ lưu trữ. Giá của
việc chốt đúng hôm nay bằng 0; giá của việc chốt sau khi vài chục xã đã tự đặt mã thì không
ai trả được.

## Ba điều mà lựa chọn này kéo theo

### 1. Service nào giữ bảng mã tầng 1 — **đã thành câu mở #22, và #22 ĐÃ ĐƯỢC TRẢ LỜI cùng ngày**

> **Đọc §Bổ sung 2026-09-20 ở cuối tệp trước khi làm gì với mục này.** Khách chốt **cách đọc
> thứ hai** trong bảng dưới. Mục này giữ nguyên vì nó ghi **vì sao câu hỏi tồn tại** — thứ
> câu trả lời không xoá đi được — nhưng **đáp án nằm ở cuối tệp**, không nằm ở đây.

Đây là chỗ dễ tưởng đã có đáp án nhất, nên viết ra vì sao **không** có:

| Cách đọc | Bộ mã nằm ở | Thêm mã thứ 13 là | Đường đọc của `petitions` |
|---|---|---|---|
| Mã là **hằng số của phần mềm**, giống bảng `quyen` | `petitions`, bảng không mang `tenant_id`, gieo bằng migration | một lần **phát hành phần mềm** | không cần đường nào — cùng schema |
| Mã là **dữ liệu nhà cung cấp quản trị** | `platform` | một thao tác trên màn hình của nhà cung cấp | gRPC hoặc bản sao đọc cập nhật bằng sự kiện (luật 2 bất biến 3) |

Tiền lệ `quyen` **không** kéo sang được, và lý do phải nói rõ kẻo phiên sau viện dẫn nhầm:
`quyen` là hằng số của phần mềm vì **có nhánh mã kiểm nó** — một khoá không ai kiểm thì không
cấp gì cả. Mã lĩnh vực thì **không có nhánh mã nào**; nó đóng vì lý do hoàn toàn khác, là để
số liệu cộng được giữa các xã. Hai lý do khác nhau dẫn tới hai chỗ đặt bảng khác nhau, và lý
do thứ hai **không nói gì** về chỗ đặt.

Câu phân biệt hai cách đọc là câu của khách, không phải của người viết mã: *ai được thêm mã
thứ 13, và bằng cách nào*. Nó nằm ở `kb/00-foundation/open-questions.json` **#22**.

**Cho tới khi #22 có trả lời: không viết migration cho tầng 1.** Điều kiện dừng #3 của
ADR 0024 vẫn còn hiệu lực cho đúng bảng này — hình dạng đã chốt không có nghĩa là chủ sở hữu
đã chốt. *(Câu này hết hiệu lực từ 2026-09-20 — xem §Bổ sung: chủ sở hữu nay đã chốt, và
migration viết được, nhưng viết ở `platform`.)*

### 2. Bảng nhãn tầng 2 nằm ở `petitions` — **đã trả lời được, không cần hỏi**

Không phải một lựa chọn mới, mà là hệ quả của hai quyết định đã chốt:

| Căn cứ | Kéo theo |
|---|---|
| Nhãn là **dữ liệu nghiệp vụ của xã** | ADR 0003 cấm `platform` giữ nó. Loại xong một trong hai ứng viên |
| Quyền sở hữu đi theo **nhịp đổi** (`ubiquitous-language.md` §Quy ước đặt tên) | Nhãn đổi khi xã đổi cách gọi trên màn hình phiếu phản ánh — cùng nhịp với `phieu_phan_anh`, tức `petitions` |

Cột giữ chuỗi người đọc tên là `nhan`, và hợp đồng vì thế trả `label` — quy tắc phân biệt
`label` với `name` có chủ ở `kb/00-foundation/ubiquitous-language.md`, không chép lại.

Một ràng buộc không được bỏ: `petitions` **không** đặt khoá ngoại sang schema của service giữ
tầng 1 (nếu #22 chốt là `platform`), và **không** `JOIN` sang đó. Mã được giữ dưới dạng giá
trị, và phải có **phép kiểm lúc ghi** — đúng cạm bẫy mà ADR 0024 §"Cái giá của dòng `Khối
nhiệm vụ`" đã chỉ ra bằng một bằng chứng có sẵn: `docs/ui-ux/14-cau-hinh.md:308` còn một dòng
SLA trỏ tới `ve-sinh-moi-truong`, một mã không còn trong danh mục, và màn hình hiện ra mã thô.

### 3. ADR 0003 — **đã kiểm, không va chạm. Phiên sau đừng kiểm lại**

ADR 0003 cấm quản trị viên nhà cung cấp **ĐỌC** dữ liệu nghiệp vụ của xã, và cưỡng chế bằng
kiến trúc: `platform` không có đường gọi vào service nghiệp vụ.

Bộ mã tầng 1 đi **ngược chiều đó**: nền tảng **cấp xuống** một tập hằng số không mang nội
dung của xã nào, còn các service nghiệp vụ là bên **đọc**. Không có chiều nào từ nhà cung cấp
đọc vào hồ sơ, đơn thư, hay dữ liệu cá nhân; cũng không service nghiệp vụ nào bị mở thêm một
cửa vào.

Hai điều kiện để câu trên còn đúng — nếu một ngày vi phạm thì đó là lúc mở lại ADR 0003:

1. Tầng 1 chỉ chứa **mã và nhãn mặc định**. Nó không bao giờ mang số đếm, không mang phiếu,
   không mang tên người.
2. Nếu #22 chốt tầng 1 thuộc `platform` thì chiều gọi là **nghiệp vụ → platform**. Không sinh
   ra client theo chiều ngược lại; `platform` vẫn nhận số đếm qua sự kiện như ADR 0003 mô tả.

## Bổ sung 2026-09-20 — #22 đã trả lời: bộ mã tầng 1 nằm ở `platform`

**Trạng thái:** đã chốt · **Ngày:** 2026-09-20 · **Đóng câu mở #22**

Khách trả lời nguyên văn: *"admin tổng nhé, trang quản trị admin cao nhất"* — tức màn hình
quản trị của **nhà cung cấp**, service `platform`. Đó là **cách đọc thứ hai** trong bảng §1.

| | |
|---|---|
| Bộ mã tầng 1 nằm ở | service **`platform`**, bảng **không** mang `tenant_id` |
| Thêm mã thứ 13 là | **một thao tác trên màn hình quản trị của nhà cung cấp** — không phải một đợt phát hành phần mềm |
| `petitions` đọc bộ mã bằng | **gRPC hoặc bản sao đọc cập nhật bằng sự kiện** (luật 2 bất biến 3). Chọn hình dạng nào là **quyết định kỹ thuật, chưa chốt** — xem ngay dưới |
| Khoá ngoại / `JOIN` sang tầng 1 | **cấm**, đúng như §2 đã ghi. Mã giữ dưới dạng giá trị, kèm **phép kiểm lúc ghi** |

**Phần CHƯA chốt, và nó KHÔNG phải câu của khách.** gRPC gọi thẳng hay bản sao đọc nuôi bằng
sự kiện là đánh đổi giữa *một phụ thuộc lúc chạy* và *một bản sao có thể trễ nhịp* — cả hai
đều hợp luật 2. Nó thuộc về **người viết mã**, và phải có ADR riêng vào ngày `petitions` đọc
bộ mã lần đầu. **Đừng đưa nó vào `open-questions.json`**: tệp đó giữ những câu chỉ khách trả
lời được, và trộn câu kỹ thuật vào đó là cách danh sách ấy mất nghĩa.

### ADR 0003 KHÔNG bị vi phạm — đã kiểm, phiên sau đừng kiểm lại

ADR 0003 cấm quản trị viên nhà cung cấp **ĐỌC** dữ liệu nghiệp vụ của xã. Quyết định này đi
**ngược chiều đó**, và ba điều làm nó ngược chiều:

1. Nền tảng **cấp xuống** một tập hằng số — **mã và nhãn mặc định**. Không mang nội dung hồ
   sơ, không mang tên người, không mang **số đếm của xã nào**
2. Chiều gọi là **NGHIỆP VỤ → PLATFORM**. `petitions` là bên đọc; `platform` không có đường
   nào gọi vào service nghiệp vụ
3. `platform` vẫn chỉ nhận **số đếm tổng hợp qua sự kiện**, đúng như ADR 0003 mô tả. Quyết
   định này không mở thêm một cửa nào

**Hai điều kiện ở §3 nay là RÀNG BUỘC ĐANG HIỆU LỰC, không còn là giả định.** Điều kiện 2 —
*"nếu #22 chốt tầng 1 thuộc `platform` thì chiều gọi là nghiệp vụ → platform, không sinh ra
client theo chiều ngược lại"* — vừa trở thành điều kiện áp dụng thật. **Sinh một client từ
`platform` gọi vào service nghiệp vụ là lúc mở lại ADR 0003**, không phải một chi tiết cài
đặt.

### Cảnh báo ở §Phải trả nay là sự thật vận hành, không còn là dự báo

Mục "Trả sau" đã viết: xã muốn một lĩnh vực mà bộ mã không có thì **không tự thêm được**.
Sau khi #22 chốt, câu ấy có hình dạng cụ thể:

> Một xã cần mã mới phải **ĐỀ NGHỊ nhà cung cấp**. Tới khi nhà cung cấp thêm mã, phiếu thuộc
> loại ấy **rơi vào `khac`**.

Đây là **cái giá của việc số liệu 200+ xã cộng được với nhau**, và nó phải được **nói ra với
xã** — không để xã tự phát hiện vào ngày họ không tìm thấy lĩnh vực mình cần. Một điểm dễ
tưởng là chi tiết nhỏ: thời gian chờ ấy **không còn là một đợt phát hành phần mềm** (cách đọc
thứ nhất), nó là một thao tác màn hình — nhanh hơn hẳn, và đó chính là thứ cách đọc thứ hai
mua được.

### Hai điều kiện dừng mới

1. Viết bảng mã tầng 1 **ở `petitions`** — chủ sở hữu đã chốt là `platform`, và đặt nhầm chỗ
   thì sửa về sau là đổi đường đọc của mọi màn hình hiện nhãn lĩnh vực
2. Viết đường đọc tầng 1 cho `petitions` **mà không có ADR** chốt gRPC hay bản sao qua sự kiện

## Điều ADR này KHÔNG đóng

| Còn mở | Ở đâu |
|---|---|
| Cấp trên xem tổng hợp **tới mức chi tiết nào**, và lĩnh vực hạn chế `can-bo` có vào số liệu xuyên xã không | câu mở **#4**, đã thu hẹp lại ngày 2026-09-20 |
| ~~Service giữ bộ mã tầng 1~~ | **đã chốt 2026-09-20: `platform`** — §Bổ sung ở trên |
| **Hình dạng** đường đọc của `petitions`: gRPC hay bản sao qua sự kiện | **quyết định kỹ thuật chưa chốt**, cần ADR riêng — **không** phải câu của khách |
| Xã có được **TẮT** một lĩnh vực mình không dùng không, và **thứ tự hiển thị** có theo xã không | **chưa hỏi.** Khách mới nói xã "chỉ được đổi tên"; tắt và sắp xếp lại không phải đổi tên, và ADR 0024 ghi §12.2 của bản mẫu cho phép tắt một mục nguồn hệ thống |

Ô để trống thứ hai của ADR 0024 (`Loại đơn thư`) **không** được ADR này lấp: xem ADR 0024 §2
sau lần cập nhật cùng ngày.

## Phải trả

- **Trả ngay:** hai bảng thay cho một, và mọi màn hình hiện lĩnh vực phải ghép mã với nhãn —
  không còn đọc một dòng ra cả hai thứ
- **Trả ngay:** một đường đọc xuyên ranh giới service nếu #22 chốt tầng 1 ở `platform`
- **Trả sau:** xã muốn một lĩnh vực mà bộ mã không có thì **không tự thêm được**. Đường duy
  nhất là đề nghị nền tảng, và tới lúc đó phiếu ấy rơi vào `khac`. Đây là cái giá của việc
  cộng được số liệu, và nó phải nói ra với xã chứ không để xã tự phát hiện
- **Không mua được:** hai tầng **không** làm báo cáo của một xã đơn lẻ đúng hơn hay nhanh hơn.
  Thứ chúng mua là con số của xã A cộng được với con số của xã B

## ĐIỀU KIỆN DỪNG

1. Một danh mục khác mà con số của nó **rời khỏi xã** — áp phép thử thứ hai ở trên, đừng mặc
   định nó giống bảy nhóm của ADR 0024
2. ~~Viết migration cho tầng 1 khi #22 chưa có trả lời~~ — **thay bằng hai điều kiện dừng của
   §Bổ sung 2026-09-20**: migration viết được, nhưng viết ở `platform`, và đường đọc cần ADR riêng
3. Đề nghị cho xã thêm mã lĩnh vực "vì xã này có nhu cầu riêng" — đó là quyết định của khách,
   và nó phá đúng thứ ADR này mua
4. Viết đường đọc xuyên xã cho báo cáo theo lĩnh vực khi **#4 chưa chốt mức chi tiết** — luật 1
   cấm #6 đòi `// @cross-tenant: <lý do>`, và lý do ấy chưa có người ký

## Bổ sung 2026-09-20 (cuối ngày) — MẶC ĐỊNH TẠM: xã TẮT và SẮP LẠI được, không xoá

Khách được hỏi *"xã có được tắt một lĩnh vực nền tảng cấp xuống, và thứ tự hiển thị có theo
xã không"* và trả lời: **để mặc định, sẽ cập nhật khi có màn hình web hoàn chỉnh và nhận
phản hồi từ các bên liên quan.** Đây là **uỷ quyền có điều kiện**, không phải một quyết định
đã chín — và khác biệt ấy phải giữ được, nếu không phiên sau sẽ đọc dòng này như thể khách
đã cân nhắc kỹ.

| Mặc định | |
|---|---|
| **Tắt** một lĩnh vực | **ĐƯỢC** — ẩn khỏi biểu mẫu gửi mới, theo `(tenant_id, ma)` |
| **Xoá** một lĩnh vực | **KHÔNG BAO GIỜ** — luật 7. Phiếu cũ mang mã ấy vẫn hiện đúng nhãn |
| **Thứ tự** hiển thị | theo xã |
| Bộ mã tầng 1 | **không đổi một chữ** bởi thao tác nào ở trên |

**Vì sao tắt được mà vẫn cộng được số:** tắt là thao tác trên **tầng nhãn**, tầng 1 không
biết nó xảy ra. Một xã không có đô thị thì `Trật tự đô thị – lấn chiếm vỉa hè` chỉ làm rối
biểu mẫu của dân; ẩn nó đi không làm mã biến mất khỏi bộ mã, nên số liệu 200 xã vẫn cộng
được và một xã bật lại lúc nào cũng được.

**Cái bẫy phải chặn bằng mã, không bằng lời:** tắt một lĩnh vực **đang có phiếu chưa đóng**.
Phiếu ấy vẫn phải hiện nhãn, vẫn phải lọc được, vẫn phải vào báo cáo. Tắt chỉ tác động lên
**đường TẠO MỚI**, không bao giờ lên đường đọc. Một `WHERE dang_bat = true` đặt nhầm vào truy
vấn danh sách sẽ làm phiếu biến mất khỏi màn hình trong khi đồng hồ của nó vẫn chạy — và
người phát hiện là công dân đang tra mã.

**MỞ LẠI KHI NÀO:** có màn hình web hoàn chỉnh và có phản hồi các bên. Đây là mặc định để
đi tiếp, không phải câu trả lời cuối.

→ ADR 0024 (ô để trống §1, và hình dạng chung của một bảng danh mục): `kb/10-decisions/0024-so-huu-danh-muc-tham-chieu.md`
→ ADR 0003 (ranh giới của quản trị nhà cung cấp): `kb/10-decisions/0003-platform-admin-metadata-only.md`
→ ADR 0027 (bốn quyết định cùng ngày, phần vòng đời phiếu): `kb/10-decisions/0027-trang-thai-va-dong-ho-phieu-phan-anh.md`
→ ADR 0028 (mốc đặt hạn, quyết định cùng ngày): `kb/10-decisions/0028-moc-dat-han-hai-dong-ho.md`
→ Câu mở #4 (còn mở) · #22 (đã đóng 2026-09-20): `kb/00-foundation/open-questions.json`
→ Bản mẫu 12 lĩnh vực: `docs/ui-ux/09-phan-anh-nguoi-dan.md` §5
→ Luật 1 (đọc xuyên xã phải khai lý do): `.claude/rules/critical/1-tenant-isolation.md`
→ Luật 2 (đọc dữ liệu service khác: gRPC hoặc sự kiện): `.claude/rules/critical/2-service-boundary.md`
