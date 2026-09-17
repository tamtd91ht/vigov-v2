---
id: 0017-contract-field-naming
tier: T1
source: CURATED
owner: architecture
derived_from_commit: 0f96676
expires: null
owns_facts:
  - "quy tắc đặt tên trường trong hợp đồng: gọi theo thứ dữ liệu LÀ, không theo nhãn màn hình"
  - "vì sao tên trường trong hợp đồng được phép lệch với tên cùng giá trị ở tầng web"
  - "phép thử phân biệt một khái niệm hai tên với hai khái niệm trùng giá trị"
---

# 0017. Tên trường trong hợp đồng gọi theo thứ dữ liệu LÀ, không theo nhãn màn hình

**Trạng thái:** đã chốt · **Ngày:** 2026-09-17 · **Nối tiếp ADR 0011**

## Bối cảnh

ADR 0011 chốt **ngôn ngữ** của bề mặt hợp đồng: đoạn nào tiếng Anh, đoạn nào giữ tiếng Việt.
Nó không trả lời câu kế tiếp, và câu kế tiếp mới là câu hay gặp: **khi một giá trị có hai cách
gọi đều đúng ngữ pháp và đều đúng tiếng Anh, chọn cái nào.**

Khoảng trống đó vừa được chạm tới thật. `message Tenant` trong
`proto/vigov/platform/v1/platform.proto` có thêm một trường mang tên tỉnh/thành của xã. Trước
khi hợp đồng có trường ấy, tầng web **đã đặt tên cho cùng giá trị đó** ở bốn chỗ, và đặt là
`parentAuthority`. Nên đây không phải một lựa chọn trong chân không: có sẵn một cái tên đang
sống trong kho mã, và chọn tên cho hợp đồng là chọn giữa hai cái tên đang cùng tồn tại.

Trường đã đặt là **`province`**. Tệp này ghi **quy tắc tổng quát** đứng sau lựa chọn đó — vì
một ADR chỉ nói về một trường thì trường sau lại phải viết ADR mới.

Ngữ nghĩa riêng của chính trường ấy (chuỗi rỗng nghĩa là gì, vì sao không dùng `optional`) nằm
ở chú thích trong `.proto` — đó là nguồn chuẩn cho câu *cái gì · như thế nào*, và luật 9 cấm
chép nó sang đây.

## Quyết định

**Tên một trường trong hợp đồng đặt theo thứ dữ liệu LÀ, không theo nhãn mà màn hình gọi nó.**

Áp dụng cho mọi bề mặt hợp đồng: `.proto` giữa các service, và bề mặt REST sinh ra từ khai báo
route (ADR 0014).

| Câu phải hỏi khi đặt tên | Câu KHÔNG được hỏi |
|---|---|
| Giá trị này **là gì** — kho dữ liệu đang nắm giữ sự thật nào | Màn hình đang **gọi nó là gì** |
| Nó còn đúng tên đó khi không màn hình nào hiển thị nó không | Dòng nào của giao diện in nó ra |

## Vì sao — dùng lại phép thử của ADR 0011

ADR 0011, mục *"Vì sao giá trị enum KHÔNG dịch"*, đã dựng sẵn phép thử và **đó là chỗ duy nhất
giữ nó**: `kb/10-decisions/0011-contract-surface-language.md`. Ở đây chỉ nối dài một bước.

0011 phân biệt **dịch một tên** (một lựa chọn đặt tên) với **dịch một giá trị** (một khẳng định
nghiệp vụ). Đặt tên trường theo nhãn màn hình rơi vào **vế thứ hai**: nó khẳng định rằng thứ dữ
liệu đang giữ và thứ màn hình đang nói là **cùng một khái niệm**.

Với trường vừa thêm, khẳng định đó là *"tỉnh/thành CHÍNH LÀ cơ quan cấp trên"*. Chạy phép thử —
*đọc sai từ này thì sai thủ tục, hay chỉ sai thẩm mỹ?*:

| Từ | Nó là gì | Loại |
|---|---|---|
| Tỉnh/thành | Xã **nằm ở đâu** | Một sự kiện địa giới hành chính |
| Cơ quan cấp trên | Xã **báo cáo cho ai** | Một quan hệ báo cáo hành chính |

Hai thứ khác loại. **Sai thủ tục, không phải sai thẩm mỹ** → không được khẳng định chúng là một
chỉ bằng cách đặt tên.

### Hai căn cứ còn lại, đều kiểm được tại chỗ

**1. Dữ liệu thật đang là tỉnh/thành.** Cột nằm ở `service-platform/migrations/0001_init.sql:66`
— `tinh_thanh`, chú thích ngay tại chỗ: *"province, for display only"*. Đặc tả giao diện cũng
đứng về phía này: `docs/ui-ux/15-phu-luc-giao-dien-chung.md` §3 in dòng phụ dưới tên xã là
`Thành phố Đà Nẵng` — đúng một tỉnh/thành. Đáng ghi rõ: **đặc tả không dùng chữ "cơ quan cấp
trên" ở bất kỳ đâu trong mục ấy**; chữ đó do tầng web đặt ra khi dựng màn hình.

**2. Gỡ một cái tên sai thì không rút lại được.** Bằng chứng nằm ngay một dòng phía trên trường
mới, trong cùng message: `reserved 5` — chỗ `admin_code` từng đứng. Một trường đã gỡ không trả
lại được số hiệu; nó chỉ để lại một dòng `reserved` vĩnh viễn. Đó là **giá thật, đã trả một
lần**, không phải một rủi ro giả định.

Và nó sẽ tới lần nữa nếu đặt sai: Việt Nam sắp xếp lại đơn vị hành chính cấp xã theo chu kỳ
(luật 1 bất biến 2 · `.claude/skills/admin-unit-merge/SKILL.md`). Ngày tỉnh/thành và cơ quan cấp
trên tách nhau, một trường tên `parent_authority` sẽ **mang một giá trị không còn là nó** — và
lúc đó lựa chọn chỉ còn giữa nói dối trong hợp đồng, hoặc thêm một dòng `reserved` nữa.

## "Web gọi nó là `parentAuthority`, sao hợp đồng lại gọi khác?"

Câu này chắc chắn có người hỏi, nên trả lời thẳng ở đây. Bốn chỗ tầng web đã đặt tên, tất cả
đều có **trước** khi hợp đồng có trường này:

| Nơi | Vai trò |
|---|---|
| `web-admin/src/lib/tenant-config.ts:22` | Khai kiểu `TenantConfig` |
| `web-admin/src/components/cau-hinh-xa.tsx:19,24` | Khai và chuyển tiếp cấu hình xã |
| `web-admin/src/components/dau-trang.tsx:32` | In ra đầu trang, class `co-quan-cap-tren` |
| `web-admin/src/app/dang-nhap/khoi-thuong-hieu.tsx:28` | In ra khối thương hiệu màn đăng nhập |

**Hợp đồng KHÔNG ép tầng web đổi tên.** Mỗi tầng đặt tên theo **thứ nó biết**, và hai tầng biết
hai chuyện khác nhau:

| Tầng | Nó biết gì | Nên tên đúng của nó là |
|---|---|---|
| Hợp đồng | Kho dữ liệu đang giữ tỉnh/thành của xã | `province` |
| Web | Nó đang in dòng "cơ quan cấp trên" trên đầu trang | `parentAuthority` |

Cả hai đều đúng **trong phạm vi tầng của mình**. Web không nói dối: trên màn hình đó, dòng ấy
**đang** là cơ quan cấp trên theo cách người dùng đọc. Hợp đồng cũng không nói dối: thứ nó
truyền đi **là** tỉnh/thành.

### Phân biệt với "Một khái niệm, bốn cái tên"

`kb/00-foundation/ubiquitous-language.md` §*"Một khái niệm, bốn cái tên"* kết luận: **khái niệm
mới thì đặt một tên và giữ nguyên qua các tầng**. Chỗ này trông như một ngoại lệ. **Nó không
phải ngoại lệ** — và đọc nhầm thành ngoại lệ là cách câu kết luận kia bị vô hiệu hoá:

> Đây **không phải** một khái niệm mang hai tên. Đây là **hai khái niệm hôm nay trùng giá trị**.

Nếu chúng là một khái niệm thì quy tắc kia áp dụng nguyên vẹn và tầng web phải đổi tên theo.

**Phép thử để phân biệt hai ca:**

| Hỏi | Kết luận |
|---|---|
| Tưởng tượng được một ngày hai giá trị **khác nhau** không? **Có** | Hai khái niệm. Mỗi tầng giữ tên của tầng mình |
| **Không** — chúng buộc phải bằng nhau mãi mãi | Một khái niệm. **Một tên**, giữ nguyên qua các tầng |

Ở đây câu trả lời là **có**, và đó không phải giả định xa: đúng cái ngày một đợt sắp xếp đơn vị
hành chính tách chúng ra.

**Ngày chúng tách ra, chi phí nằm ở đâu:** tầng web sửa chỗ nó **lấy** giá trị — một thay đổi
trong tầng trình bày, không có hồ sơ lưu trữ nào trỏ vào. Hợp đồng không phải đổi gì, vì
`province` vẫn đúng là tỉnh/thành. **Đó chính là thứ đã mua được** bằng việc đặt tên theo dữ
liệu; nếu hợp đồng mang tên `parent_authority` thì cùng ngày đó chi phí rơi vào `.proto`, tức
vào bề mặt khó lấy lại nhất.

## Hệ quả

- **Dễ hơn:** trường mới trong hợp đồng có đúng một câu phải trả lời — *giá trị này là gì* — và
  câu đó tra được trong migration, không cần mở màn hình nào
- **Khó hơn:** tên ở hợp đồng và tên ở web **cố ý lệch nhau**. Người đọc gặp lần đầu sẽ tưởng
  gặp lỗi ánh xạ. Tệp này là chỗ duy nhất giải thích — gặp lệch thì đọc đây, **đừng "sửa cho
  khớp"**
- **Phải trả ngay:** không có. Tầng web giữ nguyên `parentAuthority`; không ai phải đổi gì
- **Phải trả sau:** nếu khách xác nhận "cơ quan cấp trên" là một khái niệm nghiệp vụ thật (có
  luồng báo cáo, có dữ liệu riêng), thì nó là **một trường KHÁC** được thêm vào — không phải
  đổi tên trường này. Hai khái niệm, hai trường

## ĐIỀU KIỆN DỪNG

1. Một trường mà **không tra được kho dữ liệu đang giữ sự thật nào** — thì chưa đủ căn cứ để
   đặt tên. Hỏi người dùng, đừng lấy nhãn màn hình làm tên tạm
2. Quyết định "cơ quan cấp trên" của một xã **là gì** — đây là thực tế hành chính, thay đổi theo
   địa phương, và không phải việc của người viết mã

→ ADR 0011 (phép thử tên/giá trị, ngôn ngữ bề mặt hợp đồng): `kb/10-decisions/0011-contract-surface-language.md`
→ ADR 0012 (ranh giới gRPC — hình dạng lời gọi, không phải cách đặt tên trường): `kb/10-decisions/0012-grpc-boundary-contract.md`
→ ADR 0014 (bề mặt REST sinh từ mã, cũng chịu quy tắc này): `kb/10-decisions/0014-rest-contract-generated-from-code.md`
→ Một khái niệm mang nhiều tên trên nhiều bề mặt: `kb/00-foundation/ubiquitous-language.md`
→ Luật 1 (định danh không mang nghĩa, sắp xếp đơn vị hành chính): `.claude/rules/critical/1-tenant-isolation.md`
→ Luật 2 (`.proto` là nguồn chuẩn giữa các service): `.claude/rules/critical/2-service-boundary.md`
→ Luật 9 (một sự thật một tệp sở hữu): `.claude/rules/critical/9-knowledge-single-source.md`
→ Skill: `.claude/skills/proto-contract/SKILL.md` · `.claude/skills/admin-unit-merge/SKILL.md`
