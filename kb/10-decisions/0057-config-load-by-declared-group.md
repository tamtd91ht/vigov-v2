---
id: 0057-config-load-by-declared-group
tier: T1
source: CURATED
owner: architecture
derived_from_commit: 57d5101
expires: null
owns_facts:
  - "mỗi service khai báo nhóm biến cấu hình nó dùng; core/config.Load chỉ đọc và kiểm nhóm được khai, nhóm không khai bị bỏ qua (không từ chối, không cảnh báo)"
  - "ở prod và staging, service từ chối khởi động khi một biến của nhóm nó khai chưa được đặt — 'dùng thì bắt buộc', suy ra chứ không chọn từng biến"
  - "trường có mặc định được ghi rõ (TTL, vùng, địa chỉ lắng nghe) không tính là 'chưa đặt'; dev giữ từ chối-lúc-dùng"
  - "công tắc khởi tạo một lần (IDENTITY_ADMIN_SEED_PASSWORD) không bắt buộc ở prod"
  - "tính năng chưa nối dây (OPERATOR_* của ADR 0048, RabbitMQ, Elasticsearch) không service nào khai cho tới khi nối"
  - "vì sao không tách ConfigMap theo service và không dời việc kiểm hết vào gói tiêu thụ"
---

# 0057. `core/config` chỉ nạp nhóm biến mà service khai là dùng

**Trạng thái:** đã chốt (chủ dự án, **29/09/2026**) · **Sửa** luật 11 bất biến 8 (bắt buộc không
còn chọn từng biến — xem §*Quyết định* 2) · **Đóng** câu hỏi mở ở ADR 0052 §*Hệ quả* ("bắt buộc lúc
khởi động hay tuỳ chọn") · **Sửa** ADR 0009 dòng *Bắt buộc?* của `SECRET_ENCRYPTION_KEYS` · Ảnh
hưởng ADR 0045 (khoá cầu phiên) · Danh sách biến từng service: `deploy/cau-hinh/README.md` —
không chép vào đây.

Nguyên văn chủ dự án:

> "nguyên tắc quan trọng là pod/service nào dùng thì mới load nó"

> "Tất cả dịch vụ đều từ chối khởi động nếu biến liên quan trực tiếp chưa được set. Này là bắt
> buộc và là bài toán logic."

## Bối cảnh

`core/config.Load` phân tích và kiểm **cả 35 biến** cho **mọi** service, bất kể service ấy có dùng
hay không. Cụm đang chạy cấp **một** ConfigMap chung `common-config` qua `envFrom` cho 6 pod Go
(và `web-admin`). Hai điều đó cộng lại cho kết quả đo được ở đợt rà 29/09/2026:

| Loại hỏng | Ví dụ đo được | Hậu quả |
|---|---|---|
| **Một biến sai dạng dừng cả 6 pod** | `OBJECT_STORAGE_*`, `MALWARE_SCANNER_ADDRESS` (chỉ petitions dùng) · `CITIZEN_SESSION_TTL` (chỉ identity) · `CITIZEN_CORS` (identity, petitions, comms) | Lỗi của một tính năng thành sự cố toàn hệ thống |
| **Khoá ký phiên cán bộ ở nơi không cần** | `Load` bắt buộc `SESSION_SIGNING_KEYS` ngoài dev cho mọi service (`core/config/config.go:539-541`), trong khi nơi đọc duy nhất ngoài `core` là `service-identity/cmd/server/main.go:208` | Khoá nằm trong cả 6 Secret → **một pod bất kỳ bị chiếm là giả được token cán bộ của mọi xã** |
| **Cảnh báo sai** | Pod không dùng một nhóm vẫn in cảnh báo về nhóm ấy | Cảnh báo thật chìm trong nhiễu |
| **Prod thiếu mà vẫn chạy** | `REDIS_DSN` trống → 20 route qua `DongKhiHong` (idem) trả 503 · `TRUSTED_PROXY_CIDRS` trống → vết kiểm toán ghi sai IP · petitions thiếu kho đối tượng / máy quét · identity thiếu `PETITIONS_GRPC_ADDR`/`DOCUMENTS_GRPC_ADDR` → xoá bộ phận 503 (ADR 0056) | Hỏng **im lặng** tới yêu cầu đầu tiên của người dùng thật |

Hàng đầu và hàng cuối là **cùng một lỗi nhìn từ hai phía**: `Load` kiểm quá rộng (biến không liên
quan dừng pod) và bắt buộc quá hẹp (biến liên quan thiếu mà pod vẫn lên).

## Quyết định

### 1. Service khai nhóm; `Load` chỉ nạp nhóm được khai

`main` của mỗi service khai **danh sách nhóm biến** nó dùng (kho đối tượng, Redis, cầu phiên công
dân, gRPC tới petitions…). `Load` chỉ đọc và kiểm các nhóm ấy. Nhóm **không khai** thì bị **bỏ
qua** — không từ chối, không cảnh báo, kể cả khi biến có mặt trong `common-config`.

Đơn vị là **nhóm**, không phải biến: các biến của một phụ thuộc chỉ có nghĩa khi đi cùng nhau
(endpoint + khoá + bucket). Khai từng biến là mời một nửa cấu hình.

### 2. Dùng thì bắt buộc — ở prod và staging

Ở `prod` **và `staging`** (để staging bắt lỗi trước prod), service **từ chối khởi động** khi một
biến thuộc nhóm nó khai chưa được đặt. Đây là **hệ quả logic**, không phải lựa chọn từng biến:
service đã khai là dùng thì thiếu biến nghĩa là một phần việc của nó chắc chắn hỏng.

- Trường có **mặc định được ghi rõ** (TTL, vùng lưu trữ, địa chỉ lắng nghe) không tính là "chưa
  đặt".
- `dev` giữ cách cũ: **từ chối lúc dùng** (thao tác cần biến thì từ chối, nêu tên biến), để máy lập
  trình không phải khai đủ mọi phụ thuộc.

### 3. Công tắc khởi tạo một lần không bắt buộc ở prod

`IDENTITY_ADMIN_SEED_PASSWORD` chỉ dùng để gieo tài khoản quản trị đầu tiên và **được gỡ** sau khi
các xã đổi mật khẩu quản trị. Bắt buộc nó ở prod là bắt người vận hành giữ mãi một mật khẩu lẽ ra
đã phải biến mất.

### 4. Tính năng chưa nối dây: không service nào khai

`OPERATOR_*` (ADR 0048), RabbitMQ, Elasticsearch: **không service nào khai** cho tới ngày mã thật sự
dùng chúng. Khai trước là bắt prod đặt biến cho một thứ chưa chạy.

## Sửa những gì đã viết trước

| Chỗ | Trước | Nay |
|---|---|---|
| Luật 11 bất biến 8 | Bắt buộc/tuỳ chọn **quyết từng biến**, lý do ghi cạnh biến | Bắt buộc ở prod **khi và chỉ khi** service khai nhóm; lý do chính là việc service dùng nhóm ấy |
| ADR 0052 §*Hệ quả*, câu hỏi "bắt buộc lúc khởi động hay tuỳ chọn" | Để chủ dự án chọn | **Đóng**: bắt buộc ở prod với `petitions` — service dùng kho. Service không khai kho thì không đọc biến kho |
| ADR 0009, `SECRET_ENCRYPTION_KEYS` | Tuỳ chọn lúc `Load` | Bắt buộc ở prod với `comms`. Hệ quả: **KEK và bản sao lưu riêng của nó phải có trước khi `comms` lên prod** — cổng sao lưu của ADR 0009 thành điều kiện triển khai, không còn là việc làm sau |
| ADR 0045, khoá cầu phiên | "Cả hai hoặc không" giữa khoá và địa chỉ lắng nghe | Nay là việc **bên trong identity**: identity khai nhóm cầu phiên thì ở prod phải đủ cả hai |

## Phương án đã loại

| Phương án | Vì sao loại |
|---|---|
| **Mỗi service một ConfigMap riêng** | Cùng một giá trị (địa chỉ Redis, CIDR proxy) nằm ở nhiều nơi, sửa một nơi quên nơi khác — hai bản sao của một sự thật sẽ lệch (luật 9). Vấn đề là `Load` đọc gì, không phải cụm cấp gì |
| **Chỉ kiểm lười bên trong từng gói tiêu thụ** | Giữ nguyên bề mặt kiểm toàn cục của `Load` (vẫn dừng pod vì biến không liên quan) và không cho tín hiệu lúc khởi động ở prod — đúng lỗi hàng cuối của bảng *Bối cảnh* |

## Hệ quả

- **Vận hành phải gỡ `SESSION_SIGNING_KEYS` khỏi 5 Secret `<svc>-secrets` không phải identity** trên
  cụm đang chạy. Chừng nào còn đó, rủi ro hàng hai của *Bối cảnh* vẫn còn, dù mã đã thôi đọc.
- **Mỗi lần triển khai prod cần đủ bộ biến của service ấy.** Bộ biến từng service:
  `deploy/cau-hinh/README.md`.
- **Quên khai nhóm là lỗi im lặng mới**: service dùng một phụ thuộc mà không khai thì `Load` bỏ qua
  biến, prod không bắt. Phải có **một phép kiểm tự động** bắt trường hợp này; cơ chế do người viết mã
  chọn. Phép kiểm ấy xanh vì lý do sai (không thấy lời gọi nào) là đúng loại hỏng kho này hay gặp —
  cần một ca đỏ chứng minh nó bắt được.
- **Dễ hơn:** biến sai của một tính năng chỉ dừng service dùng tính năng ấy; khoá ký cán bộ chỉ còn ở
  identity; cảnh báo lúc khởi động chỉ nói về thứ service thật sự dùng.
- **Khó hơn:** thêm một phụ thuộc vào service là hai việc — dùng nó **và** khai nhóm — và thêm một
  bộ biến phải có trước lần triển khai prod kế tiếp.
