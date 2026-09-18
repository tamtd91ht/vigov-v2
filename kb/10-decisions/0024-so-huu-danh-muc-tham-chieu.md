---
id: 0024-so-huu-danh-muc-tham-chieu
tier: T1
source: CURATED
owner: architecture
derived_from_commit: 9abbf3f
expires: null
owns_facts:
  - "mỗi nhóm danh mục tham chiếu là một bảng riêng nằm trong service sở hữu"
  - "vì sao không có một bảng danh_muc gộp cho cả mười nhóm"
  - "phép thử phân biệt danh mục xã sửa được với hằng số vòng đời của phần mềm"
  - "vì sao mọi bảng danh mục tham chiếu mang tenant_id trong khi bảng quyen thì không"
  - "chủ sở hữu bảy nhóm danh mục đã chốt, và ba nhóm còn chờ khách"
  - "quy tắc chia khoá của bảng loi_he_thong theo bên phát ra câu nói"
---

# 0024. Danh mục tham chiếu: mỗi nhóm một bảng, đặt trong service sở hữu

**Trạng thái:** đã chốt · **Ngày:** 2026-09-18 · **Nối tiếp ADR 0001 · 0003 · 0021**

## Bối cảnh

Bản mẫu giao diện gom mười nhóm danh mục, 55 mục, vào **một bảng duy nhất**
(`docs/ui-ux/14-cau-hinh.md` §5):

```
danh_muc(id, nhom, ma, nhan, thu_tu, nguon, la_mac_dinh, dang_dung)
```

Một bảng cho một **màn hình** là sự thật về giao diện, không phải sự thật về dữ liệu. Mười
nhóm ấy nằm cạnh nhau trong một tab vì người quản trị muốn sửa chúng ở một chỗ; còn `Loại văn
bản` thì văn thư dùng, `Hạng mục kế hoạch vốn` thì kế toán dùng, và hai thứ đó không đổi cùng
nhịp, không do cùng người chịu trách nhiệm, không nằm cùng một miền.

Chép nguyên bảng ấy vào mã là dựng một bảng mà **năm service cùng phải đọc** — tức đúng cái
luật 2 cấm #2, chỉ khác là nó đến dưới hình dạng vô hại của một màn hình cấu hình.

## Bốn phương án, và vì sao ba phương án đầu hỏng

| # | Phương án | Vì sao hỏng |
|---|---|---|
| 1 | Một bảng `danh_muc` đặt ở `platform` | `platform` thành nơi giữ **dữ liệu nghiệp vụ** của xã — trái ADR 0003, thứ đã chốt với khách và là cam kết pháp lý chứ không phải lựa chọn kỹ thuật. Thêm nữa: mọi màn hình danh sách của mọi service phải gọi chéo để dịch một mã ra nhãn |
| 2 | Một bảng `danh_muc` **nhân bản** trong mỗi service | Cột `nhom` trở thành một không gian tên tự phát. Hai service cùng khai một thực thể `Catalogue`, và bộ sinh của ADR 0021 **đỏ ngay tại cổng** — quy tắc 2: hai service khai cùng một `@entity` là hỏng, không phải cảnh báo. Đây không còn là chuyện thẩm mỹ, nó **không dựng được** |
| 3 | Một bảng `danh_muc` đặt ở `identity` vì "identity là service cấu hình" | `identity` không phải service cấu hình, nó là **tổ chức – cán bộ**. Đặt ở đây thì mỗi lần văn thư thêm một loại văn bản là một lần phát hành `identity`, và ranh giới cắt theo nhịp thay đổi biến mất |
| 4 | **Mỗi nhóm một bảng, gõ đúng tên, nằm trong service sở hữu nhóm ấy** | Chọn phương án này |

**Cổng kiểm đã tự chọn hộ.** Phương án 2 — phương án dễ trượt vào nhất, vì nó giữ nguyên hình
dạng bản mẫu — dừng lại ở bộ sinh chỉ mục sở hữu, không dừng ở một vòng rà soát mà người ta có
thể bỏ qua. Ghi lại điều này vì nó là bằng chứng cho một điều ta hay nói mà ít khi chứng minh
được: một bất biến có cổng kiểm thì nó thật, không có cổng thì nó là ý kiến.

## Quyết định

**Mỗi nhóm danh mục là một bảng riêng, tên bảng gọi đúng thứ nó chứa, nằm trong migration của
service sở hữu miền ấy. Không có bảng `danh_muc` gộp.**

Bảy nhóm đã chốt chủ sở hữu:

| Nhóm trong bản mẫu | Số mục | Service sở hữu | Vì sao service ấy |
|---|---|---|---|
| Loại tài nguyên bản đồ | 8 | `comms` | Lớp bản đồ và hồ sơ tài nguyên nằm ở đây; danh mục này chỉ có nghĩa khi đứng cạnh chúng |
| Hạng mục kế hoạch vốn | 6 | `finance` | Phân loại một dòng kế hoạch vốn, đổi theo quy định ngân sách, không theo màn hình nào khác |
| Loại văn bản | 7 | `documents` | Sổ văn bản đến/đi, và cách đánh số đi theo loại |
| Loại đơn vị dân cư | 2 | `identity` | Phân loại `thon_to_dan_pho`, cùng chỗ với thứ nó phân loại |
| Khối nhiệm vụ | 3 | `identity` | Phân loại theo **bộ máy** (khối Uỷ ban / khối Đảng), đổi cùng nhịp với sơ đồ tổ chức chứ không cùng nhịp với nhiệm vụ. Xem §Cái giá của dòng này |
| Loại nhiệm vụ | 2 | `petitions` | Vòng đời nhiệm vụ nằm ở đây |
| Mức ưu tiên nhiệm vụ | 3 | `petitions` | Cùng lý do |

Cộng thêm, cùng lập luận và cùng lượt migration:

| Bảng | Service | Ghi chú |
|---|---|---|
| `thon_to_dan_pho` | `identity` | Đơn vị dân cư trong xã. Không phải danh mục tham chiếu mà là **thực thể có dữ liệu riêng** (số hộ, nhân khẩu, trưởng thôn), nhưng thuộc cùng ranh giới đang mở rộng nên chốt chung một lượt |

`identity` vì vậy **mở rộng ranh giới một cách công khai**: nó giữ thêm đơn vị dân cư và hai
danh mục treo vào bộ máy. Việc mở rộng ghi ở `kb/00-foundation/domain-boundaries.md` và ở
`service-identity/README.md` — sửa README chứ không sửa `kb/30-indexes/services.json`, vì tệp
sau là tầng SINH ra từ README (luật 9 bất biến 8).

## Vì sao MỌI bảng ở đây mang `tenant_id`

Người đọc tệp này rất dễ vừa đọc xong chú thích của bảng `quyen`
(`service-identity/migrations/0001_init.sql:44-61`) — bảng **duy nhất** trong `identity`
**không** có `tenant_id`, và chú thích nói rõ vì sao: tập quyền mà phần mềm cưỡng chế được là
thuộc tính của **phần mềm**, không phải của xã. Một xã không thể bịa ra một quyền mà mã không
kiểm, vì khoá không ai kiểm thì không cấp gì cả.

Kết luận sai rút ra từ đó: *"danh mục thì không cần `tenant_id`"*. Sai, và sai theo hướng làm
sập luật 1.

**Phép thử, dùng cho mọi nhóm sau này:**

> *Xã thêm một dòng vào đây thì phần mềm có làm gì khác không?*

| Trả lời | Nó là gì | Nơi ở |
|---|---|---|
| **Không — chỉ hiện thêm một lựa chọn, một nhãn** | **Dữ liệu của xã** | Bảng có `tenant_id`, khoá duy nhất `(tenant_id, ma)` |
| **Có — phải có nhánh mã mới xử lý dòng ấy** | **Hằng số của phần mềm** | Không phải danh mục. Đừng cho xã sửa |

Bảy nhóm đã chốt đều trả lời **Không**: thêm một loại tài nguyên bản đồ thì bản đồ có thêm một
lớp; thêm một loại văn bản thì sổ văn bản có thêm một dòng để chọn. Không nhánh mã nào phải
biết trước mã ấy.

`quyen` trả lời **Có**, và đó là toàn bộ khác biệt.

Hệ quả cụ thể của việc mang `tenant_id`, không được bỏ dòng nào:

| # | |
|---|---|
| 1 | Khoá duy nhất là **hợp thành** `(tenant_id, ma)`. Hai xã cùng dùng mã `cong-van` là bình thường và phải bình thường (luật 1, cấm #4) |
| 2 | Mục `nguon = 'he-thong'` là **hạt giống gieo cho từng xã**, không phải một dòng dùng chung toàn nền tảng. Dùng chung thì xã không tắt được nó, và §12.2 của bản mẫu cho phép tắt |
| 3 | Bảng phân mảnh theo `tenant_id` như mọi bảng nghiệp vụ khác (ADR 0004), **nhớ tạo đủ mảnh** |
| 4 | Xoá là xoá mềm; mục `nguon = 'he-thong'` chỉ tắt, không xoá (luật 7) |

## Cái giá của dòng `Khối nhiệm vụ`, nói thẳng

`Khối nhiệm vụ` thuộc `identity` nhưng thứ dùng nó — bản ghi nhiệm vụ — nằm ở `petitions`.
Đây là một **tham chiếu xuyên service**, và nó phải được đọc đúng ngay từ lượt migration đầu:

| Không được | Phải |
|---|---|
| Khoá ngoại từ bảng nhiệm vụ sang bảng khối | Bảng nhiệm vụ giữ **mã** dưới dạng giá trị |
| `JOIN` sang schema của `identity` để lấy nhãn | Lấy nhãn qua gRPC, hoặc giữ bản sao đọc cập nhật bằng sự kiện (luật 2 bất biến 3) |

Và một cạm bẫy đã có sẵn bằng chứng trong kho: `docs/ui-ux/14-cau-hinh.md` §8 còn một dòng SLA
trỏ tới mã `ve-sinh-moi-truong` **không còn trong danh mục**, hiển thị ra màn hình dưới dạng mã
thô. Đó chính là thứ xảy ra khi một mã được giữ làm giá trị mà không có gì canh — nên chỗ này
cần một phép kiểm lúc **ghi**, không phải một khoá ngoại lúc đọc.

Nếu cái giá này đắt hơn mức chấp nhận được khi dựng thật, đường ra **không phải** là kéo bảng
khối sang `petitions` trong im lặng: đó là đổi một quyết định đã ghi, tức sửa ADR này.

## `loi_he_thong` — chia theo bên PHÁT RA câu nói

Bảng `loi_he_thong` (`14-cau-hinh.md` §7) có cùng hình dạng bẫy: một màn hình, ba nhóm khoá,
39 câu. Quy tắc chia: **câu thuộc về service nào PHÁT RA nó**, không thuộc về màn hình nào hiển
thị nó.

| Nhóm | Số khoá | Service | Trạng thái |
|---|---|---|---|
| `feedback.*` | 6 | `petitions` | **Chốt** — cả sáu câu đều do đúng một nhánh từ chối trong luồng xử lý phiếu phát ra |
| `budget.scope_notice` | 1 | `finance` | **Chốt** — câu này đi kèm **mọi số liệu API trả về** của giải ngân, nên nó sinh ra tại chỗ tính số liệu |
| `report.*` | 32 | — | **CHƯA CHỐT**, xem §Ba ô để trống |

**Bác lập luận "hai kênh cùng đọc nên phải để chỗ dùng chung".** Đặc tả nói các câu `feedback.*`
hiện ở **cả web quản trị lẫn Mini App**, và từ đó rất dễ kết luận rằng chúng phải nằm ở một nơi
trung lập để hai kênh cùng với tới. Kết luận ấy nhầm chỗ: *hai kênh cùng **hiển thị*** là sự
thật về **đường ra**, không phải về **quyền sở hữu**. Câu `feedback.after_photo_required` chỉ
tồn tại vì `petitions` có một nhánh từ chối đóng phiếu khi thiếu ảnh nghiệm thu; ngày nhánh ấy
đổi điều kiện thì câu ấy phải đổi theo, và người đổi nhánh là người phải đổi câu. Đặt câu ở một
service trung lập chỉ làm hai thứ vốn phải đổi cùng nhau nằm ở hai nơi không ai buộc phải mở
cùng lúc — tức đúng dạng trôi mà luật 9 mô tả.

Nói cách khác: **nếu xoá nhánh mã thì câu ấy trở thành rác** ⇒ câu thuộc về service giữ nhánh
mã. Cả bảy khoá đã chốt ở trên đều qua được phép thử này.

## BA Ô ĐỂ TRỐNG — đang CHỜ, không phải bỏ sót

Ba nhóm dưới đây **không có chủ sở hữu trong ADR này**, và đó là kết quả của một lần cân nhắc
chứ không phải một lần quên. Mỗi ô ghi rõ: chờ ai, chờ câu nào, và **điều gì đổi nếu câu ấy
chốt theo hướng ngược lại**.

### 1. `Lĩnh vực phản ánh` (12 mục) — chờ **câu mở #4**

| | |
|---|---|
| **Chờ ai** | Khách |
| **Chờ câu gì** | Cấp huyện/tỉnh có tổng hợp theo **lĩnh vực** xuyên 200+ xã không |
| **Nếu trả lời KHÔNG** | Nó là danh mục theo xã như bảy nhóm trên, thuộc `petitions`, xong theo đúng ADR này |
| **Nếu trả lời CÓ** | **Khác hẳn ADR này.** Phải có một **bộ mã đóng ở tầng nền tảng** (để con số của 200 xã cộng được với nhau) cộng một **nhãn theo xã** đè lên. Hai tầng, hai chủ sở hữu, và xã **không** còn được thêm mã tự do — chỉ được đặt lại tên |
| **Vì sao không đoán trước** | Chọn sai chiều thì mọi báo cáo tổng hợp hoặc sai, hoặc phải ánh xạ thủ công 200 bộ mã khác nhau về một bộ. Không có đường sửa rẻ sau khi các xã đã tự đặt mã |

### 2. `Loại đơn thư` (5 mục) — chờ **câu mở #4**, cộng một câu chưa hỏi

| | |
|---|---|
| **Chờ ai** | Khách |
| **Chờ câu gì** | (a) đúng câu #4 ở trên — đơn thư cũng là thứ cấp trên tổng hợp; (b) **sổ đơn thư do ai cấp số vào sổ**: văn thư (`documents`) hay tiếp dân (`petitions`) |
| **Nếu (b) trả lời "văn thư"** | Danh mục thuộc `documents`, đi cùng quy tắc đánh số sổ |
| **Nếu (b) trả lời "tiếp dân"** | Thuộc `petitions` |
| **Vì sao không đoán trước** | `kieu-nghi / phan-anh / khieu-nai / to-cao / de-nghi` là **năm thứ khác nhau về pháp lý**, và số vào sổ là thứ đã cấp thì không cấp lại (luật 7 bất biến 3). Đặt sai chỗ thì việc sửa là di trú số sổ trên hồ sơ lưu trữ |

### 3. `Trạng thái nhiệm vụ` (7 mục) — chờ một **câu hỏi MỚI**, chưa có trong `open-questions.json`

| | |
|---|---|
| **Chờ ai** | Khách |
| **Chờ câu gì** | Danh sách **mã** trạng thái nhiệm vụ có sửa được theo xã không, hay xã chỉ được sửa **nhãn và thứ tự** |
| **Vì sao nó không phải danh mục cho tới khi có câu trả lời** | Nó **trượt phép thử** ở §Vì sao mọi bảng mang `tenant_id`: thêm một dòng vào đây thì phần mềm **phải** có nhánh mã mới xử lý. `docs/ui-ux/02-nhiem-vu.md:214` gọi nó là danh mục "sửa được ở Cấu hình", nhưng ngay dưới, dòng 227–231 vẽ một máy trạng thái có hướng đi cố định, và `task.approve` gắn cứng vào đúng bước `cho-duyet → hoan-thanh` |
| **Nếu chốt "chỉ sửa nhãn và thứ tự"** | Mã là hằng số của phần mềm — giống `quyen`. Bảng vẫn có, mang `tenant_id`, nhưng cột `ma` **không** cho xã thêm/bớt |
| **Nếu chốt "xã sửa được cả mã"** | Máy trạng thái phải trở thành **dữ liệu** (bảng chuyển trạng thái theo xã), và mọi nhánh mã đang gọi tên `hoan-thanh` phải đi qua một khái niệm gián tiếp. Đây là thiết kế khác, không phải một cột thêm vào |
| **Câu hỏi này chưa nằm trong `kb/00-foundation/open-questions.json`** | Nội dung đã soạn và **chuyển cho người dùng** trong phiên này; tệp ấy đang thuộc phạm vi một phiên song song nên ADR này **không** tự ghi vào đó |

### Phụ: 32 khoá `report.*` của `loi_he_thong`

Chưa chốt. Ba lối ra đã nêu, chưa chọn cái nào, và cả ba đều phải trả lời **cùng một câu**:
`reporting` **không sở hữu dữ liệu gốc nào** — vậy nó có được sở hữu **chữ** không?

| Lối ra | Được | Mất |
|---|---|---|
| Cả 32 khoá về `reporting` | Một chỗ duy nhất, đúng với chỗ báo cáo được dựng ra | `reporting` bắt đầu giữ dữ liệu xã sửa được, tức nhích ra khỏi vai trò read model thuần |
| Chia theo tiền tố khối (`report.metric.tasks.*` → `petitions`, `report.metric.budget.*` → `finance`, …) | Đúng phép thử "xoá nhánh mã thì câu thành rác" | Tiêu đề và khối chung (`report.title`, `report.block.*`) không thuộc về ai; và một màn cấu hình phải gom chữ từ năm service |
| Coi toàn bộ là **cấu hình trình bày**, không phải chữ nghiệp vụ | Tránh được cả hai vướng trên | Cần một khái niệm mới chưa có trong hệ thống, và khái niệm mới đặt sai chỗ là đúng thứ ADR này đang dọn |

Chốt việc này **trước khi** viết bảng `loi_he_thong` đầu tiên, không phải sau.

## Phải trả

- **Trả ngay:** tám bảng nhỏ thay cho một bảng, tám lượt `CREATE TABLE` + phân mảnh + hạt
  giống. Nhiều dòng SQL hơn bản mẫu
- **Trả ngay, phần đắt hơn:** màn hình `Cấu hình → Danh mục` **không còn một truy vấn duy nhất**.
  Nó gom từ bốn service qua hợp đồng. Đây là cái giá thật của việc làm microservice, và trả ở
  tầng đọc rẻ hơn hẳn trả ở tầng sở hữu
- **Trả sau:** nhóm thứ mười một sinh ra thì phải hỏi ai sở hữu, không tự thêm một dòng `nhom`
- **Không mua được:** bảy bảng nhỏ **không** làm màn hình cấu hình nhanh hơn hay đẹp hơn. Thứ
  chúng mua là: đổi danh mục của văn thư không đụng tới kế toán

## ĐIỀU KIỆN DỪNG

1. **Nhóm danh mục thứ mười một** mà chủ sở hữu không hiển nhiên — hỏi, đừng đặt vào service
   đang mở sẵn trong trình soạn thảo
2. Một nhóm **trượt phép thử** ở §Vì sao mọi bảng mang `tenant_id` (thêm một dòng thì phải thêm
   nhánh mã) — đó không phải danh mục, và cho xã sửa nó là một sự cố chờ sẵn
3. Ba ô để trống ở trên: **viết migration cho bất kỳ nhóm nào trong ba nhóm ấy là quyết hộ
   khách**, kể cả khi nó chỉ là một bảng giống hệt bảy bảng kia
4. Đề xuất gộp lại thành một bảng `danh_muc` "cho tiện màn hình cấu hình" — tiện cho màn hình
   là lý do bản mẫu làm thế, và nó không phải lý do cho mô hình dữ liệu

→ ADR 0001 (vì sao ranh giới cắt ở đó): `kb/10-decisions/0001-service-decomposition.md`
→ ADR 0003 (vì sao `platform` không giữ dữ liệu nghiệp vụ): `kb/10-decisions/0003-platform-admin-metadata-only.md`
→ ADR 0004 (phân mảnh theo xã): `kb/10-decisions/0004-shard-by-tenant.md`
→ ADR 0021 (dấu `@entity` trong migration, và cổng kiểm đỏ khi hai service trùng): `kb/10-decisions/0021-khai-quyen-so-huu-thuc-the.md`
→ Ranh giới `identity` sau lần mở rộng này: `kb/00-foundation/domain-boundaries.md`
→ Bản mẫu giao diện (10 nhóm, 55 mục, 39 câu): `docs/ui-ux/14-cau-hinh.md` §5 · §7
→ Luật 1 (khoá duy nhất hợp thành với xã): `.claude/rules/critical/1-tenant-isolation.md`
→ Luật 2 (một thực thể một service sở hữu): `.claude/rules/critical/2-service-boundary.md`
