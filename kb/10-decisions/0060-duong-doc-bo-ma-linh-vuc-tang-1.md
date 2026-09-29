---
id: 0060-duong-doc-bo-ma-linh-vuc-tang-1
tier: T1
source: CURATED
owner: architecture
derived_from_commit: d5a4625
expires: null
owns_facts:
  - "service nghiệp vụ đọc bộ mã lĩnh vực tầng 1 bằng gRPC PlatformService.ListPetitionFields, không bằng bản sao nuôi bằng sự kiện"
  - "vì sao được cache bộ mã tầng 1 tối đa 60 giây mà không cần sự kiện huỷ cache, và vì sao không được phục vụ bản đã hết hạn"
  - "platform không tới được thì bên đọc từ chối, không coi mã nào là hợp lệ và không hiện mã thô"
  - "một dòng tầng 1 mang mã, nhãn mặc định, thứ tự, biểu tượng, tông màu và cờ còn dùng — không mang gợi ý nhập, không mang cờ hạn chế"
  - "mã tầng 1 không bao giờ xoá, không bao giờ đổi; ngừng dùng là active = false"
---

# 0060. Đường đọc bộ mã lĩnh vực tầng 1: gRPC từ nghiệp vụ sang `platform`

**Trạng thái:** đã chốt · **Ngày:** 2026-09-29 · **Người dùng chốt phương án A ngày 29/09/2026**
("dựng ADR 0026 cho đúng") · **Lấp ô trống ADR 0026 §Bổ sung 2026-09-20** ("hình dạng đường đọc
của `petitions`: gRPC hay bản sao qua sự kiện — quyết định kỹ thuật chưa chốt, cần ADR riêng")

## Bối cảnh

ADR 0026 chốt bộ mã lĩnh vực phản ánh tầng 1 thuộc `platform`, bảng không mang `tenant_id`, và
để lại đúng một câu kỹ thuật: bên nghiệp vụ đọc nó bằng gRPC hay bằng bản sao nuôi bằng sự kiện.
Điều kiện dừng #2 của §Bổ sung cấm viết đường đọc khi chưa có ADR này. Hệ quả đang nằm trên đĩa:

| Chỗ | Đang thiếu vì chưa có đường đọc |
|---|---|
| ADR 0050 điểm 1 | Dân chọn lĩnh vực lúc gửi, hạn xử lý xong đặt lúc tạo phiếu — `petitions` phải **kiểm mã** trước khi hỏi hạn |
| `proto/vigov/identity/v1/identity.proto:1131` | `ResolveDeadlines` nhận mã viết sai như một mã chưa có dòng SLA và **lặng lẽ dùng dòng mặc định**. Lỗ ấy chỉ bịt được ở đường ghi, bằng bộ mã tầng 1 |
| `service-petitions/migrations/0004_phieu_phan_anh.sql:173-179` | `nhan_linh_vuc` (tầng 2) không có tuyến ghi |
| `service-identity/internal/domain/sla_gieo.go:69-71` | Nút "Thêm thời hạn cho một lĩnh vực" không có tuyến |
| `web-admin/src/features/phan-anh/nhan-phieu.ts:410-447` | Bản chép tay thứ ba của 12 mã |

## Quyết định

### 1. Đường đọc là gRPC: `PlatformService.ListPetitionFields`, cả bộ trong một lời gọi

Chiều gọi **nghiệp vụ → platform**, đúng điều kiện 2 của ADR 0026 §3. Một lời gọi trả **mọi** mã,
kể cả mã đã ngừng dùng — bộ mã vài chục dòng, và bên đọc cần cả mã cũ để hiện nhãn cho phiếu cũ.
Không có RPC hỏi từng mã (ADR 0012 quyết định 2: theo lô mặc định). Hình dạng và bảng mã trạng
thái nằm ở `proto/vigov/platform/v1/platform.proto`, không chép lại ở đây.

Lời gọi **mang `x-tenant-id`** như mọi RPC khác. Bên gọi luôn có xã (phiên công dân hoặc Host của
cán bộ); miễn kiểm là điều kiện dừng của ADR 0012 quyết định 1, và ở đây không có lý do nào.

### 2. Được cache, tối đa 60 giây, MỘT bản cho mọi xã — không sự kiện huỷ, không phục vụ bản hết hạn

**Vì sao không cần sự kiện huỷ cache:** bộ mã chỉ **thêm**, không bớt, không đổi mã (mục 4). Một
bản cache cũ tối đa 60 giây vì thế chỉ sai theo ba cách, và cả ba đều an toàn:

| Bản cũ thiếu gì | Hệ quả trong ≤ 60 giây | Vì sao chấp nhận được |
|---|---|---|
| Mã vừa được thêm | Bên đọc từ chối mã ấy | Hỏng về phía đóng. Nhà cung cấp chờ một phút sau khi thêm |
| Nhãn mặc định vừa sửa | Màn hình hiện nhãn cũ | Nhãn không mang nghĩa pháp lý; mã mới là thứ lưu trên phiếu |
| Mã vừa ngừng dùng | Vẫn nhận phiếu mới mang mã ấy | Mã ngừng dùng **vẫn là mã tầng 1 thật**, vẫn cộng được giữa các xã — đúng thứ ADR 0026 mua |

Không trường hợp nào để một mã **không có trong bộ** lọt vào phiếu. Đó là điều ADR 0026 §2 đòi,
và tính chỉ-thêm của bộ mã làm cho TTL đơn thuần đủ để giữ nó.

**Vì sao một bản cho mọi xã, khác `ListUploadPolicies`:** giới hạn tải lên cache theo xã vì
ngoại lệ theo xã đang **mở** ở `platform`. Ở đây ngoại lệ theo xã **đã có chủ**: nó là tầng 2, và
ADR 0026 §2 đặt tầng 2 ở `petitions`. Tầng 1 theo định nghĩa là một bộ cho mọi xã, nên cache theo
xã chỉ nhân bản cùng một câu trả lời vài trăm lần.

**Vì sao không phục vụ bản hết hạn khi `platform` không trả lời:** nếu cho phép, thời gian một
thay đổi có hiệu lực không còn chặn trên — nó bằng độ dài sự cố. Giống ADR 0052 §10 và ADR 0012
quyết định 4: quá một TTL mà không hỏi được thì từ chối. Lỗi không được cache.

### 3. `platform` không tới được → bên đọc TỪ CHỐI

Không coi mã nào là hợp lệ, không có danh sách dự phòng trong mã nguồn, không hiện mã thô thay
nhãn (đúng cạm bẫy `ve-sinh-moi-truong`, ADR 0026 §2). Đường tạo phiếu, đường phân loại và tuyến
danh mục phía dân trả 503. `core/platformclient` trả một lỗi canh riêng cho ca này để bên gọi
không nhầm "không hỏi được" với "mã không tồn tại".

### 4. Nội dung một dòng tầng 1

| Cột | Có | Vì sao |
|---|---|---|
| mã | có | Khoá. **Không bao giờ đổi, không bao giờ xoá** — phiếu lưu trữ giữ nó dưới dạng giá trị (luật 7). Cơ sở dữ liệu chặn cả hai bằng trigger |
| nhãn mặc định | có | Hiện khi xã không khai nhãn tầng 2 |
| thứ tự | có | Thứ tự mặc định; xã sắp lại ở tầng 2 (ADR 0026 §Bổ sung cuối ngày) |
| biểu tượng, tông màu | có | Bản đã triển khai của kho yêu cầu gắn chúng vào từng mã. Để ở tầng 1 thì mã thứ 13 thêm trên màn hình nhà cung cấp là **có ngay** biểu tượng, không cần phát hành Mini App — đúng thứ ADR 0026 cách đọc thứ hai mua |
| còn dùng (`active`) | có | Ngừng dùng một mã trên toàn nền tảng: ẩn khỏi đường **tạo mới và phân loại** ở mọi xã, không bao giờ khỏi đường đọc |
| gợi ý nhập (placeholder) | **không** | Nguồn duy nhất có giá trị là dữ liệu giả của prototype, với mã không tồn tại, và câu mẫu nêu tên phố của **một** xã — đó là nội dung của xã, nếu có thì ở tầng 2. Thêm sau là một trường tuỳ chọn, không phá hợp đồng |
| màu hex | **không** | Trùng nghĩa với tông màu; hai nguồn cho một sự thật |
| cờ "hạn chế" (`can-bo`) | **không** | Phần mềm **rẽ nhánh** theo nó (quyền `feedback.restricted`, luồng riêng của ADR 0050 điểm 10) — tức là một hằng số của phần mềm, theo đúng lập luận `quyen` mà ADR 0026 §1 viết ra. Nó ở mã nguồn của `petitions` |

**Không có `deleted_at`, và đó là chủ ý:** luật 7 bất biến 2 bắt mọi đường đọc loại dòng đã xoá
mềm, nên một mã xoá mềm sẽ làm phiếu cũ mất nhãn — đúng cái bẫy ADR 0026 §Bổ sung cuối ngày mô tả
cho tầng 2 (`WHERE dang_bat = true` đặt nhầm vào truy vấn danh sách). Ngừng dùng là `active = false`,
và đường đọc không bao giờ lọc theo nó.

**Nới điều kiện 1 của ADR 0026 §3 ("tầng 1 chỉ chứa mã và nhãn mặc định"):** biểu tượng, tông màu,
thứ tự, cờ còn dùng đều là **trình bày dùng chung cho mọi xã**. Nội dung của điều kiện ấy — không số
đếm, không phiếu, không tên người — giữ nguyên, nên ADR 0003 không bị chạm. Thêm vào `PetitionField`
bất cứ thứ gì mang nội dung của một xã là lúc mở lại ADR 0003.

### 5. Mười hai mã khởi tạo

Bốn nguồn độc lập khớp nhau **từng mã và từng nhãn**: bảng `docs/ui-ux/09-phan-anh-nguoi-dan.md`
§5, bộ gieo SLA của `identity` (`sla_gieo.go:141-152`, khoá của mọi hạn xử lý), ô chọn của
`web-admin` (`nhan-phieu.ts:434-447`), và danh mục đã triển khai của kho yêu cầu
(`../vigov-require` tại `0053854`, `apps/api/app/modules/org/data/default_config.json:336` và
`apps/miniapp/scripts/api-stub.mjs:32`). Thứ tự là của `09 §5`. Biểu tượng và tông màu lấy từ kho
yêu cầu. Không có xung đột nào để báo.

`apps/miniapp/src/mocks/data.ts:71` của kho yêu cầu mang bộ khác (`ve-sinh-moi-truong`,
`dien-chieu-sang`, `an-ninh-trat-tu`) — chính kho ấy ghi đó là bộ tự nghĩ ra và **đã bỏ**
(`api-stub.mjs:25-28`). Không dùng.

## Vì sao KHÔNG chọn bản sao nuôi bằng sự kiện

| Lý do | |
|---|---|
| **Điều kiện dừng #1 của ADR 0026 §Bổ sung** | Một bảng bản sao ở `petitions` **là** một bảng mã tầng 1 ở `petitions`, dù gọi tên gì |
| **Sự kiện không có xã** | Luật 1 bất biến 9: bên tiêu thụ thiếu `tenant_id` trong thông điệp thì **từ chối**. Một thay đổi của bộ mã dùng chung không thuộc xã nào; phát nó đòi một ngoại lệ trong bộ phân phối sự kiện — mở đúng cửa mà luật ấy đóng |
| **Vẫn cần RPC** | Bản sao mới dựng, hay bỏ lỡ một sự kiện, phải lấy lại toàn bộ — tức là vẫn phải có RPC này, cộng thêm outbox mà `platform` chưa có (`service-platform/internal/event` rỗng) |
| **N bản sao** | `petitions`, `identity` (tuyến thêm dòng SLA), `reporting` (nhãn cho số liệu xuyên xã) đều cần bộ mã — N bảng cho một sự thật |
| **Độ trễ không mua được gì** | `platform` đã là phụ thuộc đồng bộ của mọi yêu cầu (ADR 0012 quyết định 4: `ResolveHost`). Thêm một lời gọi có cache 60 giây không tạo ra kiểu hỏng mới nào mà hệ thống chưa chịu |

## Cái giá — đừng giấu

- **`platform` sập quá 60 giây** thì dân không gửi được phản ánh, cán bộ không phân loại được, và
  màn hình cần nhãn trả 503. Đó là cái giá của hỏng-thì-đóng, cùng loại ADR 0012 quyết định 4 đã trả.
- **Thêm mã thứ 13** có hiệu lực ở mọi service sau tối đa một TTL — không tức thì.
- **Tuyến ghi của nhà cung cấp chưa dựng** (thêm mã, sửa nhãn mặc định, ngừng dùng): thuộc khu vận
  hành (ADR 0048), ghi dòng và `platform_audit_log` trong cùng giao dịch (luật 6 bất biến 3). Tới
  lúc đó bộ mã chỉ đổi bằng migration.
- **Bản chép tay ở `web-admin`** vẫn còn tới khi `petitions` có tuyến danh mục; sau đó phải xoá.

## ĐIỀU KIỆN DỪNG

1. Một bảng mã tầng 1, hay bản sao của nó, trong bất kỳ service nghiệp vụ nào
2. Một danh sách mã dự phòng trong mã nguồn, hay phục vụ câu trả lời đã quá TTL khi `platform` hỏng
3. Cache bộ mã quá 60 giây, hoặc dựng ngoại lệ theo xã cho tầng 1 ở `platform` (ngoại lệ theo xã là tầng 2, ở `petitions`)
4. Thêm vào `PetitionField` một trường mang nội dung của một xã, số đếm, hay tên người — mở lại ADR 0003
5. Đổi hay bỏ một mã đã cấp — đó là di trú hồ sơ lưu trữ (luật 7), không phải sửa danh mục
6. Đưa `ListPetitionFields` vào `core/grpcx.methodsWithoutTenant`

→ ADR 0026 (hai tầng, chủ sở hữu tầng 1): `kb/10-decisions/0026-linh-vuc-phan-anh-hai-tang.md`
→ ADR 0050 điểm 1, 10 (lĩnh vực dân chọn, luồng tác phong cán bộ): `kb/10-decisions/0050-kenh-cong-dan-theo-kho-yeu-cau.md`
→ ADR 0012 (theo lô, `platform` sập): `kb/10-decisions/0012-grpc-boundary-contract.md`
→ ADR 0003 (platform chỉ siêu dữ liệu): `kb/10-decisions/0003-platform-admin-metadata-only.md`
→ ADR 0052 §10 (cùng hình dạng cache, cho giới hạn tải lên): `kb/10-decisions/0052-object-storage-minio.md`
→ Hợp đồng: `proto/vigov/platform/v1/platform.proto` · Bảng: `service-platform/migrations/0011_petition_field.sql`
