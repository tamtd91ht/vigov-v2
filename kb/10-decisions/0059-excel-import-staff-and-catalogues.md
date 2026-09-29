---
id: 0059-excel-import-staff-and-catalogues
tier: T1
source: CURATED
owner: architecture
derived_from_commit: 1796286
expires: null
owns_facts:
  - "nhập cán bộ từ Excel tạo dòng cán bộ, gán vai trò và cấp tài khoản trong cùng một lượt"
  - "N mật khẩu tạm của lượt nhập chỉ trả MỘT LẦN trong phản hồi ghi, không lưu bản trần, không vào log, không vào delta của vết"
  - "nhập cán bộ là tất cả hoặc không; cột Vai trò không trống thì cần thêm admin.role; email trống lưu NULL — người dùng 29/09/2026"
  - "đơn vị dân cư có tuyến tạo, sửa, ngưng dùng (mềm) dưới admin.org; gộp và tách cố ý không hỗ trợ"
  - "đơn vị dân cư có tuyến nhập Excel; ngưng dùng không bị chặn khi còn hồ sơ trỏ tới — người dùng 29/09/2026"
  - "nhập danh mục từ Excel: mỗi service sở hữu một tuyến nhập cho nhóm của mình, không có tuyến nhập danh mục chung"
---

# 0059. Nhập từ Excel: cán bộ kèm tài khoản, đơn vị dân cư ghi được, danh mục nhập theo service sở hữu

**Trạng thái:** đã chốt (người dùng, **29/09/2026**) · Phạm vi: `docs/ui-ux/14-cau-hinh.md` §2
(Thôn), §3 (Người dùng), §5 (Danh mục) · Nối tiếp câu mở #9 · #15 · #16 (đã chốt 22/09/2026) và
ADR 0024.

## Bối cảnh

Nhập sơ đồ tổ chức từ Excel đã chạy, và lúc ấy người dùng **hoãn** nhập cán bộ
(`service-identity/internal/http/routes.go:1402` — *"org units only (staff import deferred), all or
nothing"*). Đặc tả vẽ nút `⬆ Nhập từ Excel` ở ba tab nữa. Kho yêu cầu có một bộ nhập chung cho năm
loại, gồm cả người dùng và **một** loại "Danh mục" cho mọi nhóm
(`../vigov-require/apps/api/app/modules/admin/catalogues.py:415-481`).

Câu khó là **mật khẩu**. Cấp tài khoản cho một người đã có hình dạng: máy chủ sinh mật khẩu tạm,
trả **một lần** trong thân phản hồi, khai miễn trừ tường minh tại trường
(`service-identity/internal/http/tai_khoan_can_bo.go:62`, khách chốt 22/09/2026). Nhập N người thì
hoặc tạo N tài khoản cùng lúc, hoặc bắt quản trị viên bấm cấp N lần.

## Quyết định

### 1. Nhập cán bộ = tạo dòng + gán vai trò + cấp tài khoản

> **Người dùng, 29/09/2026:** lượt nhập **tạo dòng cán bộ** (mã tự sinh theo #15; máy bàn cơ quan
> và di động cá nhân là **hai cột** theo #16), **gán vai trò** (có vết, luật 5 bất biến 5) **và cấp
> tài khoản đăng nhập**. N mật khẩu tạm (#9, bắt đổi ở lần đăng nhập đầu) trả về **MỘT LẦN** trong
> phản hồi của bước ghi để quản trị viên tải xuống và phát cho từng người; **không bao giờ** lưu bản
> trần, **không bao giờ** vào log, **không bao giờ** vào delta của vết.

| Điều | Hệ quả |
|---|---|
| Mã tự sinh (#15) | Tệp mẫu **không** có cột Mã. Mã đã cấp không cấp lại, kể cả khi lượt nhập bị huỷ về sau (luật 7 bất biến 3) |
| Hai cột số điện thoại (#16) | Máy bàn là thông tin công vụ, di động là dữ liệu cá nhân (Nghị định 13). Lỗi xem trước chỉ nêu **dòng và cột**, không nhắc lại giá trị (luật 3 cấm #3). Mẫu dùng số giả `0900000000` — mẫu của kho yêu cầu có một số trông như thật (`catalogues.py:460`), không chép |
| Mật khẩu tạm | Cùng miễn trừ của `tai_khoan_can_bo.go:62`, nhân N. Mất tệp thì quản trị viên **đặt lại** từng người bằng tuyến đã có — mỗi lần một giá trị mới, một vết mới |

> **Người dùng, 29/09/2026:** nhập cán bộ là **tất cả hoặc không**, như nhập sơ đồ tổ chức
> (`routes.go:1402`). Tệp có cột **Vai trò không trống** thì ngoài `admin.user` còn cần
> **`admin.role`**. **Email trống lưu `NULL`**, không lưu chuỗi rỗng.

| Điều | Vì sao |
|---|---|
| Tất cả hoặc không | Nhận từng phần thì N mật khẩu tạm trả về ứng với một tập dòng mà quản trị viên phải dò lại xem dòng nào đã vào — và mã đã cấp không cấp lại (luật 7 bất biến 3) |
| Thêm `admin.role` | Gán vai trò là đổi quyền, có vết riêng (luật 5 bất biến 5). Kho yêu cầu chỉ đòi `admin.user` (`catalogues.py:441`) — tức ai tạo được người là cấp được quyền; không chép. Cột trống thì lượt nhập không gán gì và chỉ cần `admin.user` |
| Email `NULL` | `UNIQUE (tenant_id, email)` coi hai chuỗi rỗng là trùng, nhưng hai `NULL` thì không — nên nhiều người không có email cùng vào được. Chướng ngại ở §Hệ quả là thứ quyết định này gỡ |

**Cái giá, nói thẳng:** tệp tải xuống là **một danh sách thông tin đăng nhập** nằm trên máy quản trị
viên cho tới khi được xoá. Một lần cấp một người đã có rủi ro ấy với một mật khẩu; ở đây là N.
Người dùng chọn hình dạng này. Điều giới hạn rủi ro (nhận xét của người ghi, không phải lời người
dùng): mọi mật khẩu **phải đổi ở lần đăng nhập đầu**, nên tệp chỉ còn giá trị cho tới khi từng người
đăng nhập lần đầu.

### 2. Đơn vị dân cư có tuyến ghi; gộp và tách thì không

> **Người dùng, 29/09/2026:** thôn / tổ dân phố có tuyến **tạo**, **sửa**, **ngưng dùng (xoá
> mềm)** dưới `admin.org`, theo kho yêu cầu. **Gộp và tách cố ý KHÔNG hỗ trợ** — luật 1 điều kiện
> dừng #3.

Kho yêu cầu: `../vigov-require/apps/api/app/modules/org/router.py:156-193`,
`../vigov-require/apps/api/app/modules/org/service.py:311` (`create_hamlet`). Hôm nay v2 chỉ có tuyến đọc
(`service-identity/internal/http/routes.go:1671`).

**Vì sao không gộp/tách:** gộp hai thôn là sửa hồ sơ lưu trữ đang trỏ tới thôn cũ — đúng thứ luật 1
cấm #7 và `skills/admin-unit-merge` canh. Không có tuyến thì không có lối tắt; xã cần gộp thì đó
là một câu hỏi, không phải một nút bấm.

> **Người dùng, 29/09/2026:** đơn vị dân cư **có tuyến nhập Excel**. **Ngưng dùng được**, kể cả khi
> còn phiếu phản ánh hay hộ dân trỏ tới: hồ sơ **giữ nguyên**, đơn vị chỉ **ẩn khỏi ô chọn**.

Khác ADR 0056 (chặn xoá bộ phận còn người/việc) có chủ ý: ngưng dùng một thôn không sửa dòng nào
đang trỏ tới nó, nên không có hồ sơ nào bị đổi — chỉ không ai chọn được thôn ấy cho hồ sơ **mới**.
Hệ quả cho người dựng (nhận xét của người ghi): ô chọn loại thôn đã ngưng, nhưng hồ sơ cũ vẫn phải
hiện được **tên** thôn ấy — lọc ở ô chọn, không lọc ở nhãn. Tuyến nhập thôn là tất cả hay từng
phần thì người dùng chưa nói — câu mở #5.

### 3. Nhập danh mục: một tuyến nhập mỗi service sở hữu

> **Người dùng, 29/09/2026:** nhập danh mục là **một tuyến nhập cho mỗi service sở hữu**, cho bảy
> nhóm ghi được. Không có bảng chung.

Bảy nhóm và chủ sở hữu: ADR 0024 §Quyết định. Kho yêu cầu nhập mọi nhóm qua **một** loại
`lookup-values` với cột `Nhóm` (`catalogues.py:466-481`) — đó là bảng `danh_muc` gộp mà ADR 0024
điều kiện dừng #4 cấm, chỉ khác ở đường vào. Ba nhóm còn để trống của ADR 0024 **không** có tuyến
nhập.

## Hệ quả

- **Dễ:** mỗi tuyến nhập kiểm đúng quy tắc của bảng nó ghi, trong giao dịch của service ấy, có vết
  cùng giao dịch (luật 6 bất biến 3).
- **Khó:** màn `Cấu hình → Danh mục` phải chọn đúng service theo nhóm để gửi tệp — cùng cái giá
  "gom từ nhiều service" ADR 0024 §Phải trả đã ghi.
- **Chướng ngại đã biết — gỡ bằng quyết định 29/09 ở §1:** tạo cán bộ đang bắt buộc email, với
  `UNIQUE (tenant_id, email)` — mỗi xã chỉ chứa được một người không có email
  (`kb/90-ephemeral/tien-do/service-identity.json:70`, mục (4)). Lưu email trống thành `NULL` là
  việc của lượt dựng, cả ở tuyến tạo một người lẫn tuyến nhập; tới lúc ấy chướng ngại còn nguyên.

## Câu từng mở — người dùng trả lời 29/09/2026

| # | Câu | Trả lời |
|---|---|---|
| 1 | Nhập cán bộ tất cả hoặc không, hay từng phần | Tất cả hoặc không — §1 |
| 2 | Gán vai trò cần `admin.user` hay cả `admin.role` | Cột Vai trò không trống thì cần thêm `admin.role` — §1 |
| 3 | Thôn / tổ dân phố có tuyến nhập Excel không | Có — §2 |
| 4 | Ngưng dùng một thôn còn hồ sơ trỏ tới có chặn không | Không chặn; hồ sơ giữ nguyên, thôn ẩn khỏi ô chọn — §2 |

## Còn mở — chưa ai quyết

| # | Câu | Ai |
|---|---|---|
| 5 | Nhập thôn / tổ dân phố là tất cả hoặc không (như cán bộ và sơ đồ tổ chức), hay từng phần | Người dùng |

## ĐIỀU KIỆN DỪNG

1. Lưu mật khẩu tạm ở bất kỳ dạng đọc lại được nào, hoặc trả nó ở tuyến thứ hai
2. Một tuyến gộp hay tách đơn vị dân cư
3. Một tuyến nhập danh mục chung cho nhiều nhóm, ở bất kỳ service nào
4. Tệp mẫu có cột Mã cán bộ

→ Câu mở #9 · #15 · #16: `kb/00-foundation/open-questions.json`
→ ADR 0024 (chủ sở hữu từng nhóm danh mục) · 0056 (chặn xoá bộ phận)
→ Luật 1 · 3 · 5 · 6 · 7
