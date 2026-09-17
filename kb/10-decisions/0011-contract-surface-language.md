---
id: 0011-contract-surface-language
tier: T1
source: CURATED
owner: architecture
derived_from_commit: 5887496
expires: null
owns_facts:
  - "vì sao URL path dùng tiếng Anh trong khi bảng và cột dùng tiếng Việt"
  - "vì sao giá trị enum không được dịch sang tiếng Anh"
  - "vì sao REST phải mang tiền tố phiên bản /api/v1/"
  - "chốt dạng tên sự kiện là petitions.received.v1"
---

# 0011. Ngôn ngữ của bề mặt hợp đồng — URL path và tên sự kiện

**Trạng thái:** đã chốt · **Ngày:** 2026-09-16 · **Nối tiếp ADR 0001**

## Bối cảnh

ADR 0001 chốt: định danh máy đọc (tên service, proto package, import path) dùng **tiếng
Anh**, bảng và cột dùng **tiếng Việt không dấu**. Nhưng ADR đó chỉ nói về hai đầu — tầng
trong cùng và tầng ngoài cùng của mã — và không nói gì về **bề mặt mà bên ngoài nhìn thấy**:
đường dẫn URL và tên sự kiện.

Khoảng trống đó đã bắt đầu được lấp bằng cách đoán, theo hai hướng ngược nhau:

| Nơi | Đang ghi | Vấn đề |
|---|---|---|
| `kb/90-ephemeral/2026-09-16-ban-giao-phien.md` §2 | `POST /dang-nhap` | Chọn tiếng Việt cho path — chưa ai cân nhắc, chỉ là việc kế tiếp viết vội |
| `kb/00-foundation/ubiquitous-language.md` | `petitions.da_tiep_nhan.v1` | Tên sự kiện tiếng Việt |
| `kb/00-foundation/domain-boundaries.md` | `petitions.received.v1` | Tên sự kiện tiếng Anh |

Hai tệp T0 nói khác nhau về **cùng một fact** là đúng thứ luật 9 cấm, và cả hai đều đang
được nạp như nguồn đáng tin.

**Thời điểm quyết định là bây giờ, vì bây giờ giá bằng 0:** chưa có route HTTP nghiệp vụ
nào, chưa có sự kiện nào được publish, chưa xã nào chạy thật. Sau khi một xã chạy thật thì
không lần nào khác giá còn bằng 0 nữa.

## Quyết định

| Bề mặt | Ngôn ngữ | Ví dụ |
|---|---|---|
| Đoạn đường dẫn URL | **Tiếng Anh**, số nhiều, kebab-case, danh từ | `/api/v1/incoming-documents` |
| Tên tham số truy vấn | **Tiếng Anh** | `?type=`, `?cursor=` |
| **Giá trị enum** | **Tiếng Việt không dấu** | `?type=khieu-nai` |
| Tên sự kiện | **Tiếng Anh**, miền số nhiều, thì quá khứ, có `.vN` | `petitions.received.v1` |
| Tên bảng, cột, định danh nghiệp vụ tầng domain | **Tiếng Việt không dấu** | `don_thu`, `ngay_tiep_nhan` |
| Chuỗi người đọc | **Tiếng Việt có dấu** | `"Không có nhiệm vụ"` |

Cách áp dụng nằm ở `.claude/skills/rest-api-design/SKILL.md`; bảng ánh xạ khái niệm sang
tên tài nguyên nằm ở `kb/00-foundation/ubiquitous-language.md`. Tệp này chỉ trả lời **vì sao**.

## Vì sao path tiếng Anh, trong khi bảng và cột tiếng Việt

Không phải vì tiếng Anh đẹp hơn. Vì **hai thứ đó không cùng loại**:

| | Tên bảng, cột, kiểu Go tầng domain | Đoạn đường dẫn URL |
|---|---|---|
| Ai đọc | Người viết mã của chính dự án này | Bên tích hợp: Mini App, cổng tỉnh, hệ thống cơ quan khác, API gateway, công cụ log |
| Đổi tên tốn gì | Một lần đổi tên trong IDE, hoặc một migration | **Không đổi được** — client nằm ngoài tầm kiểm soát |
| Mang gì | Khái niệm **pháp lý**, phải đúng từng chữ | Khái niệm **tài nguyên**, là địa chỉ để trỏ tới |

Đường dẫn là **hợp đồng với bên tích hợp**, không phải tên nội bộ. Tên nội bộ sai thì sửa
trong một buổi chiều. Đường dẫn mà một xã đã chạy thật thì không có cơ hội thứ hai rẻ tiền:
client đã phát hành nằm ngoài tầm kiểm soát, và luật 7 cấm xoá dữ liệu nên bản ghi cũ vẫn
phải tra cứu được — kể cả bằng đường dẫn cũ, mãi mãi.

Thêm một lý do cụ thể ở hệ thống này: đường dẫn là **bề mặt duy nhất** bị đọc bởi người
không đọc được tiếng Việt và bởi công cụ không xử lý được dấu — proxy, gateway, hệ thống
giám sát, báo cáo sự cố của bên thứ ba. Một đoạn path tiếng Việt không dấu vẫn đọc được,
nhưng nó buộc mọi bên tích hợp phải tra từ điển hành chính Việt Nam để gọi đúng một API.

**Chi phí phải trả, nói thẳng:** từ nay một khái niệm có **hai từ vựng** — `phan_anh` trong
CSDL và `citizen-reports` trên URL. Hai từ vựng thì phải có bảng ánh xạ, và bảng ánh xạ phải
có **đúng một chủ**: `kb/00-foundation/ubiquitous-language.md`. Không có bảng đó thì mỗi
phiên tự dịch lại và ra một tên khác — đúng kiểu trôi dạt luật 9 mô tả.

## Vì sao giá trị enum KHÔNG dịch — phần đắt nhất của ADR này

Dịch một đường dẫn là **một lựa chọn đặt tên**. Dịch một **giá trị** là **một khẳng định
nghiệp vụ**: rằng khái niệm tiếng Việt và từ tiếng Anh là cùng một thứ.

Với `kien-nghi` · `phan-anh` · `khieu-nai` · `to-cao` · `de-nghi` thì **không**. Năm loại
này chạy theo **hai luật khác nhau** và **hai đồng hồ thời hạn khác nhau**. Gọi `khieu_nai`
là `complaint` và `to_cao` là `denunciation` nghe như dịch đúng, nhưng tiếng Anh không giữ
được ranh giới pháp lý giữa chúng, và hệ quả không phải là thẩm mỹ:

| Gọi nhầm | Hậu quả |
|---|---|
| `khieu_nai` lẫn với `phan_anh` | Áp sai thủ tục và **sai thời hạn luật định** |
| `to_cao` lẫn với loại khác | Mất **bảo vệ người tố cáo** |

Còn một lý do kỹ thuật độc lập: giá trị enum **không chỉ đi qua API, nó nằm trong CSDL** và
nằm trong hồ sơ lưu trữ đã đóng. Dịch nó ở tầng API thì hoặc API lệch với CSDL (hai nguồn
cho một fact), hoặc phải di trú giá trị trên **hồ sơ lưu trữ** — thứ luật 7 không cho phép.

**Phép thử khi phân vân:** *đọc sai từ này thì sai thủ tục, hay chỉ sai thẩm mỹ?* Sai thủ tục
→ giữ nguyên tiếng Việt.

## Vì sao `/api/v1/`

`.proto` đã đánh phiên bản trong package (`vigov.petitions.v1`), và sự kiện đã đánh phiên bản
trong tên (`.v1` — luật 2, bất biến 4). REST không đánh phiên bản thì hệ thống có **ba tầng
hợp đồng mà chỉ hai tầng có chỗ để đặt phiên bản thứ hai**.

Điều đó hỏng đúng vào lúc cần nhất. Luật 2 bất biến 4 nói hai phiên bản chạy song song trong
lúc di trú. Nếu REST không có tiền tố phiên bản thì cách duy nhất còn lại là **đổi nghĩa
đường dẫn cũ tại chỗ** — tức là làm vỡ client đang chạy của một xã đang vận hành, giữa giờ
hành chính.

Phiên bản đặt **trong path**, không đặt trong header: header vô hình trong log truy cập,
trong cache của proxy, và trong báo cáo lỗi của bên tích hợp — ba nơi ta sẽ phải đọc khi có
sự cố thật.

## Tên sự kiện: chốt `petitions.received.v1`

Ba nguồn tài liệu ghi ba kiểu, nhưng **mã đã chạy chỉ ghi một kiểu**: `core/events/events.go`,
`core/events/events_test.go` và `proto/vigov/events/v1/events.proto` đều dùng
`petitions.received.v1`. Theo luật 9, câu *"tên nó là gì"* thuộc về mã — tài liệu nào khác
thì tài liệu đó sai.

| Thành phần | Chốt | Vì sao |
|---|---|---|
| `petitions` | **Số nhiều** | Đoạn đầu là tên **miền** (cũng là tên service), không phải tên thực thể |
| `received` | **Tiếng Anh, thì quá khứ** | Cùng loại định danh máy đọc với tên service — ADR 0001 |
| `.v1` | Bắt buộc | Luật 2, bất biến 4 |

**Chưa có sự kiện nào được publish, nên đổi bây giờ giá bằng 0.** Sau khi có consumer thật,
luật 2 (cấm #4) biến việc này thành một đợt di trú hai phiên bản song song. Đây là lý do duy
nhất cần cho việc chốt ngay hôm nay thay vì để lại.

Tệp **sở hữu** fact này: `kb/00-foundation/domain-boundaries.md` — cùng chỗ đang sở hữu quy
tắc đặt tên định danh máy đọc. `kb/00-foundation/ubiquitous-language.md` **liên kết tới**,
không chép lại.

## Hệ quả

- **Dễ hơn:** bên tích hợp đọc được API mà không cần biết tiếng Việt; và hợp đồng REST,
  gRPC, sự kiện có cùng một cách đánh phiên bản
- **Khó hơn:** một khái niệm mang hai từ vựng, nên mọi tên tài nguyên mới phải tra bảng ánh
  xạ trước khi viết route — không được tự dịch tại chỗ
- **Phải trả ngay:** `kb/90-ephemeral/2026-09-16-ban-giao-phien.md` §2 đang ghi
  `POST /dang-nhap`, nay là `POST /api/v1/sessions`; `kb/00-foundation/ubiquitous-language.md`
  đang ghi tên sự kiện tiếng Việt
- **Phải trả sau:** giá trị enum tiếng Việt sẽ luôn trông lạc lõng bên cạnh path tiếng Anh.
  Đó là chủ ý, không phải sót — chỗ nào thấy lạc lõng thì đọc lại mục "Vì sao giá trị enum
  KHÔNG dịch" trước khi sửa

→ ADR 0001 (định danh máy đọc tiếng Anh): `kb/10-decisions/0001-service-decomposition.md`
→ Bảng ánh xạ khái niệm sang tên tài nguyên: `kb/00-foundation/ubiquitous-language.md`
→ Cách áp dụng: `.claude/skills/rest-api-design/SKILL.md`
→ Cưỡng chế: `.claude/hooks/rest_api_guard.py` (chặn path tiếng Việt)
→ Luật 2 (phiên bản trong tên sự kiện): `.claude/rules/critical/2-service-boundary.md`
→ Luật 7 (không di trú trên hồ sơ lưu trữ): `.claude/rules/critical/7-data-preservation.md`
