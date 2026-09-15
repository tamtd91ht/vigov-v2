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
---

# 0001. Cắt 8 service theo bộ phận hành chính

**Trạng thái:** đã chốt · **Ngày:** 2026-09-15

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
| `nentang` | Nền tảng (nhà cung cấp vận hành) |
| `danhtinh` | Tổ chức – cán bộ |
| `vanban` | Văn thư |
| `donthu` | Tiếp dân, xử lý đơn thư |
| `hoso` | Một cửa |
| `taichinh` | Tài chính – kế toán |
| `truyenthong` | Thông tin – truyền thông |
| `baocao` | Read model — **không sở hữu dữ liệu gốc nào** |

**`admin-service` và `contract-service` không tồn tại.**

## Hai thứ CỐ Ý không phải service

| Thứ | Là gì | Vì sao không tách |
|---|---|---|
| Nhật ký thao tác | Thư viện `pkg/audit` + bảng audit trong từng service | Luật 6 bắt ghi vết **cùng giao dịch** với ghi nghiệp vụ. Service riêng thì không chia được giao dịch — tách ra là **khoá cứng** vào kiến trúc đúng lỗi đã đo ở v1 (0 giao dịch trên toàn backend) |
| Lưu trữ tệp | Thư viện `pkg/storage` + metadata do từng miền sở hữu | Tách thành service thì mọi luồng đính kèm thành 2 lời gọi mạng cho một thao tác. Vi phạm phép thử "cần nhất quán mạnh thì đừng tách" |

## Hệ quả

- **Dễ hơn:** một thay đổi nghiệp vụ chạm đúng một service; ranh giới khớp cách cơ quan vận hành
- **Khó hơn:** `baocao` phải dựng read model từ sự kiện, không được đọc CSDL ai. Tốn hơn lúc đầu, nhưng đây chính là thứ ngăn hệ thống thành monolith phân tán
- **Phải trả sau:** nếu một bộ phận tách đôi ngoài đời (ví dụ tiếp dân tách khỏi xử lý đơn thư), service tương ứng cũng phải tách — và lúc đó là việc thật, không phải refactor

→ Nguyên tắc cắt: `kb/00-foundation/domain-boundaries.md`
