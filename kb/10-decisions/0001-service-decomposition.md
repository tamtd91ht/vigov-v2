---
id: 0001-service-decomposition
tier: T1
source: CURATED
owner: architecture
derived_from_commit: null
expires: null
owns_facts:
  - "danh sách 8 service và bộ phận hành chính tương ứng"
  - "vì sao audit và storage là thư viện chứ không phải service"
  - "ngôn ngữ đặt tên service"
---

# 0001. Cắt 8 service theo bộ phận hành chính

**Trạng thái:** đã chốt · **Ngày:** 2026-09-15 · **Sửa bảng tên service:** 2026-09-16

## Bối cảnh

Đề xuất ban đầu của chủ đầu tư là `admin-service · auth-service · report-service ·
contract-service · user-service` — cắt theo **tầng kỹ thuật và theo màn hình**.

Đo trên bản v1 (NestJS, 22 module) cho thấy vì sao cách cắt đó nguy hiểm: quyền sở hữu dữ
liệu đã rối sẵn khi còn là monolith.

| Collection | Số module GHI vào |
|---|---|
| `CitizenUser` | 5 |
| `Feedback` | 5 |
| `IncomingDocument` | 5 |
| `Task` | 4 |
| `StaffUser` | 4 |

`reports` và `search` tiêm model của 4–5 module khác để đọc trực tiếp. Trong microservice
đường đó không tồn tại.

## Các phương án

| Phương án | Được | Mất |
|---|---|---|
| Cắt theo **màn hình** (`admin-service`) | Khớp cấu trúc giao diện | "Admin" không phải miền — mọi miền đều có phần admin. Service này sẽ phình thành monolith thứ hai |
| Cắt theo **tầng kỹ thuật** (`auth` tách khỏi `user`) | Nghe gọn | Coupling chatty ở mọi request; và v1 cho thấy cán bộ với công dân là hai **lớp tin cậy** khác nhau, không phải hai bảng cùng loại |
| Cắt theo **bộ phận hành chính** | Một thay đổi nghiệp vụ chạm đúng một service; khớp cách một UBND xã thật vận hành | Tên service không khớp tên màn hình, cần một lần làm quen |

## Quyết định

Tám service, cắt theo bộ phận chịu trách nhiệm trong một UBND xã:

| Service | Bộ phận ngoài đời |
|---|---|
| `platform` | Nền tảng (nhà cung cấp vận hành) |
| `identity` | Tổ chức – cán bộ |
| `documents` | Văn thư |
| `petitions` | Tiếp dân, xử lý đơn thư |
| `dossiers` | Một cửa |
| `finance` | Tài chính – kế toán |
| `comms` | Thông tin – truyền thông |
| `reporting` | Read model — **không sở hữu dữ liệu gốc nào** |

**`admin-service` và `contract-service` không tồn tại.**

## Ngôn ngữ đặt tên service — sửa ngày 2026-09-16

Bản ADR đầu tiên ghi tên service bằng tiếng Việt (`nentang`, `danhtinh`, `vanban`, `donthu`,
`hoso`, `taichinh`, `truyenthong`, `baocao`). Nhưng mã đã dựng theo tên **tiếng Anh**: 8 thư
mục `services/`, 9 tệp `.proto`, và `go_package_prefix` trong `buf.gen.yaml`.

Ba nguồn cùng mô tả một fact mà kết luận khác nhau là đúng thứ luật 9 cấm. Chốt **tiếng Anh**
vì đó là thứ mã đang chạy, và đổi mã thì phải sửa 8 thư mục + 9 `.proto` + đường dẫn import
để đổi lấy đúng một thứ: sự nhất quán với một bảng trong tài liệu.

Ranh giới ngôn ngữ sau khi chốt:

| Loại | Ngôn ngữ | Ví dụ |
|---|---|---|
| Tên service, proto package, import path | **Tiếng Anh** | `petitions`, `vigov.identity.v1` |
| Tên bảng, tên cột | **Tiếng Việt** không dấu | `don_thu`, `ngay_tiep_nhan` |
| Văn xuôi tài liệu `kb/` | **Tiếng Việt** | tệp này |
| Chuỗi giao diện | **Tiếng Việt** | `Giao việc mới` |

→ Bảng ánh xạ thuật ngữ: `kb/00-foundation/ubiquitous-language.md`

## Hai thứ CỐ Ý không phải service

| Thứ | Là gì | Vì sao không tách |
|---|---|---|
| Nhật ký thao tác | Thư viện `core/audit` + bảng audit trong từng service | Luật 6 bắt ghi vết **cùng giao dịch** với ghi nghiệp vụ. Service riêng thì không chia được giao dịch — tách ra là **khoá cứng** vào kiến trúc đúng lỗi đã đo ở v1 (0 giao dịch trên toàn backend) |
| Lưu trữ tệp | Thư viện `core/storage` + metadata do từng miền sở hữu | Tách thành service thì mọi luồng đính kèm thành 2 lời gọi mạng cho một thao tác. Vi phạm phép thử "cần nhất quán mạnh thì đừng tách" |

## Hệ quả

- **Dễ hơn:** một thay đổi nghiệp vụ chạm đúng một service; ranh giới khớp cách cơ quan vận hành
- **Khó hơn:** `reporting` phải dựng read model từ sự kiện, không được đọc CSDL ai. Tốn hơn lúc đầu, nhưng đây chính là thứ ngăn hệ thống thành monolith phân tán
- **Phải trả sau:** nếu một bộ phận tách đôi ngoài đời (ví dụ tiếp dân tách khỏi xử lý đơn thư), service tương ứng cũng phải tách — và lúc đó là việc thật, không phải refactor

→ Nguyên tắc cắt: `kb/00-foundation/domain-boundaries.md`

## Bổ sung 2026-09-20 — một cửa ra khỏi phạm vi

**KHÔNG SỬA phần trên.** Bảng tám service ở §Quyết định là quyết định ngày 15/09/2026 và nó
đã đúng vào ngày ấy; mục này ghi cái đã đổi, không xoá cái đã ghi. Kể từ hôm nay kho có
**bảy** service Go — `dossiers` đã gỡ.

**Khách chốt ngày 20/09/2026: hồ sơ một cửa KHÔNG thuộc phạm vi hợp đồng.** Đây chính là câu
chặn nặng nhất mà sổ `service-dossiers` treo từ trước, nay đã có đáp án.

### Bằng chứng khảo sát (đợt 2026-09-20) — vì sao gỡ chứ không để khung rỗng

| # | Rào | Bằng chứng |
|---|---|---|
| 1 | Không có chương đặc tả | `docs/ui-ux/` có 16 chương, không chương nào cho một cửa; sơ đồ dữ liệu tổng thể `docs/ui-ux/00-tong-quan-he-thong.md:163-188` liệt kê 14 nhánh, không nhánh nào là hồ sơ một cửa; thứ tự dựng `:276-284` chín mục cũng không có |
| 2 | Không có danh mục thủ tục | 10 nhóm danh mục ở `docs/ui-ux/14-cau-hinh.md:164-176`, không nhóm nào là Thủ tục hành chính |
| 3 | Không có chỗ cho hạn | Bảng SLA `docs/ui-ux/14-cau-hinh.md:316` khai `loai_viec enum('van-ban-den','phan-anh','nhiem-vu')` — ba giá trị, không có hồ sơ; ADR 0007:28-33 cũng ba dòng ấy |
| 4 | Không có khoá quyền | 33 khoá nạp ở `service-identity/migrations/0001_init.sql`, không khoá nào chạm hồ sơ |
| 5 | Không có tên tài nguyên URL | `kb/00-foundation/ubiquitous-language.md` mới chốt tên BẢNG `ho_so_mot_cua`; bảng tên tài nguyên không có dòng nào cho một cửa |
| 6 | Vai trò có thật nhưng không có màn hình | `Cán bộ một cửa` có trong bộ máy (`docs/ui-ux/00-tong-quan-he-thong.md:41`, `14-cau-hinh.md:95`) — prototype không cấp cho vai trò ấy một màn hình nào |

Estimate cũng đã cảnh báo đúng chỗ này trước khi ký: `kb/90-ephemeral/estimate-vigov.md:156`
ghi thẳng “không estimate được”, và `:567` xếp phạm vi một cửa là rủi ro **Cao**.

Tiền đề DUY NHẤT đã có: lịch làm việc theo xã đã chạy
(`service-identity/migrations/0006_lich_lam_viec.sql`, RPC `AdvanceWorkingHours`, và bọc
client `identityclient.TienGioLamViec`). Nó được **giữ nguyên**: `petitions` và `documents`
cần nó theo bảng SLA của ADR 0007, `finance` là chỗ thứ ba hiển nhiên. Kiểm 2026-09-20:
ngoài `service-identity` chưa service nào GỌI bọc client ấy — nó là chỗ đã dựng sẵn, chưa
phải chỗ đang dùng.

### Đã gỡ ở commit này

`service-dossiers/` · `proto/vigov/dossiers/v1/` · mục trong `go.work` · sổ tiến độ
`kb/90-ephemeral/tien-do/service-dossiers.json` · dòng trong `kb/00-foundation/domain-boundaries.md`
· dòng trong `deploy/README.md`.

### ĐƯỜNG QUAY LẠI — nếu khách đưa một cửa vào phạm vi

**Thứ tự bắt buộc, không được đảo:** phải có **chương đặc tả** trước; chưa có đặc tả thì
không dựng lại service, vì đúng sáu rào trên sẽ hiện ra lần nữa và mỗi rào là một chỗ để
đoán sai trong im lặng.

Sau chương đặc tả, phải chốt với khách các câu dưới đây trước dòng mã nghiệp vụ đầu tiên —
chúng phải được đánh số vào `kb/00-foundation/open-questions.json` chứ không để trong sổ
module:

1. Một cửa có trong phạm vi hợp đồng không — **đã trả lời 20/09/2026: KHÔNG**
2. Danh mục **thủ tục hành chính** lấy từ đâu, ai cập nhật, có phải nhóm danh mục thứ 11 không
3. Hạn xử lý hồ sơ đếm thế nào, và `loai_viec` có thêm giá trị thứ tư không (ADR 0007, ADR 0028)
4. Khoá quyền cho một cửa là những khoá nào — 33 khoá hiện có không khoá nào chạm hồ sơ (luật 5)
5. Tên tài nguyên URL tiếng Anh cho hồ sơ một cửa (`ubiquitous-language` cấm tự dịch rồi viết route)
6. Vai trò `Cán bộ một cửa` được cấp những màn hình nào
7. Hồ sơ một cửa có phải service riêng, hay một miền trong `documents`

**GHI RÕ MỘT CHỖ HỤT:** sổ `service-dossiers.json` nói có **chín** câu, nhưng chỉ liệt kê
được câu chặn và sáu rào. Hai câu còn lại **chưa từng được viết ra ở đâu** — đã tìm trong
`git log`, trong `open-questions.json` (26 câu, không câu nào về một cửa) và trong estimate.
Bảy câu trên là tất cả những gì có bằng chứng; phiên nào dựng lại một cửa phải khảo sát lại
từ chương đặc tả chứ đừng tin rằng danh sách này đủ chín.
