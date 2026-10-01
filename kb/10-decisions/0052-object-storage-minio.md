---
id: 0052-object-storage-minio
tier: T1
source: CURATED
owner: architecture
derived_from_commit: cfdd90a
expires: null
owns_facts:
  - "lưu trữ đối tượng dùng MinIO (S3) qua thư viện core/storage, metadata ở bảng stored_file của từng service sở hữu tệp"
  - "luồng tải lên ba bước: xin phép ở service → tải thẳng lên bucket temp bằng presigned POST → complete (dò kiểu, sha256, quét mã độc, bỏ EXIF, chép sang bucket đích, ghi nghiệp vụ + vết cùng giao dịch)"
  - "ba bucket theo chức năng: vigov-{env}-private · vigov-{env}-public · vigov-{env}-temp"
  - "dạng khoá đối tượng {class}/t_{tenant_id}/{yyyy}/{mm}/{service}/{purpose}/{object_id}/{variant}.{ext} và vì sao tiền tố xã viết t_ chứ không t:"
  - "vì sao tên tệp gốc không bao giờ nằm trong khoá đối tượng"
  - "xoá tệp nghiệp vụ do ứng dụng điều khiển (worker purge theo retain_until); vòng đời MinIO chỉ cho bucket temp"
  - "vì sao chưa mã hoá lúc nghỉ, và cái giá"
  - "quét mã độc bằng ClamAV lúc complete, máy quét không tới được thì từ chối"
  - "giới hạn kích thước và kiểu tệp theo mục đích do platform sở hữu, thiếu cấu hình thì từ chối tải lên"
  - "xử lý video ngay lúc complete, không hàng đợi"
  - "công cụ vận hành vigovctl storage purge và các giới hạn của nó"
---

# 0052. Lưu trữ đối tượng trên MinIO

**Trạng thái:** đã chốt (chủ dự án, 28/09/2026 — mười hai quyết định ở §*Quyết định*) · mọi dòng
ghi **"chưa chốt"** là **chưa được chốt** · **Bổ sung** ADR 0010 (thêm hai thành phần hạ tầng: kho
đối tượng và máy quét mã độc) · **Cụ thể hoá** ADR 0001 §*Hai thứ CỐ Ý không phải service* dòng
*Lưu trữ tệp* — không thay nó · Mọi tên dưới đây là tiếng Anh theo ADR 0051.

## Bối cảnh

ADR 0001:96 chốt lưu tệp là **thư viện `core/storage` + metadata do từng miền sở hữu**, không phải
service, nhưng không nói kho nào, bucket nào, khoá ra sao, xoá khi nào. Hôm nay `core/storage`
**chưa tồn tại** (`core/` không có thư mục ấy), nên mọi chỗ cần tệp đang giữ URL do hệ khác phục vụ:

| Chỗ | Nơi |
|---|---|
| `anh_dai_dien_url`, `tep_dinh_kem` của nội dung Mini App — *"there is no `core/storage`"* | `service-comms/migrations/0006_noi_dung_mini_app.sql:277-285` |
| `logo_url` của hồ sơ hiển thị xã | `service-platform/migrations/0006_mini_app_va_ho_so_hien_thi.sql:134` |
| Logo — đề xuất tiền tố `t:<tenant_id>`, công khai khai tường minh | ADR 0048 §*Thiết kế* #8 |

Nội dung Mini App loại `video` (`service-comms/internal/domain/noi_dung_mini_app.go:47`) và ảnh hiện
trường của phản ánh là hai nhu cầu đầu tiên không giải được bằng URL ngoài.

## Quyết định

### 1. Kiến trúc

`core/storage` là thư viện bọc client MinIO/S3, chỉ năm việc: presigned POST (có
`content-length-range`), stat, server-side copy, presigned GET, xoá **mọi phiên bản** (cho purge).
Mỗi service sở hữu tệp giữ bảng `stored_file` của riêng mình (luật 2 bất biến 1) — không service
nào đọc `stored_file` của service khác.

| Bước | Ai | Việc |
|---|---|---|
| a. Xin tải | client → service sở hữu | Service kiểm quyền (luật 5) hoặc phiên công dân (luật 4), xã (luật 1), `purpose`, kích thước và kiểu **khai báo** theo giới hạn của mục đích (§10). Tạo `stored_file{status: pending}`. Trả presigned POST vào bucket temp, **TTL 15 phút**, giới hạn kích thước do **chính sách MinIO** cưỡng chế |
| b. Tải | client → MinIO | Tải thẳng, byte không đi qua service |
| c. Hoàn tất | client → service | Stat đối tượng · **dò magic bytes** (không tin `Content-Type` client gửi) · tính `sha256` · quét ClamAV (nhiễm → từ chối, xoá đối tượng temp, ghi vết) · bỏ EXIF với ảnh do công dân tải · server-side copy sang bucket/khoá đích · `status: stored` · gắn vào bản ghi nghiệp vụ + vết **CÙNG GIAO DỊCH** (luật 6 bất biến 3) |

**Đối tượng bất biến:** phiên bản mới là `object_id` mới. Không ghi đè khoá đã có.

**Bổ sung 30/09/2026:** thêm hai việc thứ sáu, thứ bảy — **đăng / gỡ đăng** bản dẫn xuất (§11)
thành thao tác có tên riêng (`PublishDerivative`, `UnpublishDerivative` trong
`core/storage/storage.go`), vì đó là **đường duy nhất** vào/ra bucket public: nguồn phải ở private,
đích là bản sinh đôi lớp `public-media` cùng xã/service/mục đích/`object_id`/biến thể; từ chối
bản `original`, mọi thứ lớp `citizen-media` (ĐIỀU KIỆN DỪNG #2) và mọi thứ lớp `records` (hồ sơ
nghiệp vụ không dành cho người đọc ẩn danh) — nguồn đăng được chỉ còn lớp `content-source`. Hệ quả:
ảnh bìa tin phải có **bản dẫn xuất** (ví dụ `thumb-1280`, mã hoá lại — cũng bỏ luôn EXIF) trước khi
đăng, không đăng thẳng bản gốc. Cache `immutable` một năm nghĩa là gỡ đăng không thu hồi được bản
CDN/trình duyệt đã tải — cần xoá cache CDN, chưa có. Câu "chỉ năm việc" ở trên giữ
nguyên làm lịch sử.

**Bổ sung 30/09/2026 (lần hai):** thêm việc thứ tám — **ghi byte do server tạo ra**
(`PutServerProduced` trong `core/storage/storage.go`), phục vụ đúng hai luồng người dùng đã duyệt
(ADR 0047 §6): (a) **ảnh bìa tin** — server giải mã bản gốc đã promote, xoay theo Orientation, thu
về ≤ 1280px, mã hoá lại JPEG, lưu thành biến thể `thumb-1280` lớp `content-source` để
`PublishDerivative` đăng; (b) **ảnh hiện trường của công dân (G3)** — lúc hoàn tất, server giải mã
tệp temp, xoay, mã hoá lại **bỏ toàn bộ EXIF** và chỉ lưu byte sạch vào private làm biến thể
`original` lớp `citizen-media`; bản công dân tải **không bao giờ** vào private, tệp temp xoá bằng
`PurgeAllVersions` trên bucket temp. Chỉ ghi bucket private; từ chối lớp `records` (server không tự
tạo hồ sơ) và `public-media` (chỉ `PublishDerivative` ghi vào đó). **Bất đối xứng có chủ ý:** với
`content-source`, `original` chỉ đến từ `Promote` (bản xã tải, đã quét) nên bị từ chối ở đây; với
`citizen-media`, bản mã hoá lại **chính là** `original`, vì bản thô mang EXIF (toạ độ nhà, định danh
thiết bị — luật 3) không được lưu. Dò magic bytes phải khớp đuôi khoá; `Content-Type` theo kết quả
dò; không metadata người dùng; kích thước phải đúng bằng số khai (thừa/thiếu → từ chối trước khi
thân yêu cầu đủ, nên không đối tượng nào bị cắt/độn được lưu), trần 32 MiB — chuyển mã video (§11)
vượt trần này, **không** đi đường này; khoá đã có → từ chối (bất biến). Trả về `sha256` (tính khi
truyền), kích thước, kiểu, ETag, phiên bản để service ghi `stored_file`.

### 2. Bucket — ít, theo chức năng chính

| Bucket | Chứa | Truy cập | Versioning |
|---|---|---|---|
| `vigov-{env}-private` | Mọi tệp nghiệp vụ riêng tư: hồ sơ/bản quét, media công dân, nguồn nội dung | Chỉ qua presigned GET | **Bật** |
| `vigov-{env}-public` | Bản dẫn xuất **đã duyệt** cho Mini App: video đã chuyển mã, poster, logo | Ẩn danh **chỉ `GetObject`**, không liệt kê | Tắt |
| `vigov-{env}-temp` | `upload/…` đang tải (vòng đời MinIO xoá sau **1 ngày**) · `export/…` tệp xuất (xoá sau **7 ngày**) | Presigned | Tắt |

`env` = `prod` | `stg`. Staging **nên** là MinIO (hoặc tenant MinIO) riêng để khoá của nó không bao
giờ chạm prod. Khoá trong `upload/` lặp lại đúng dạng khoá đích (§3) sau tiền tố, để phạm vi IAM
theo service áp được cả ở temp.

Bucket public là **khai báo công khai tường minh**, đúng nguyên tắc *Closed by default*: chỉ bản
dẫn xuất đã qua duyệt vào đây; bản gốc luôn ở private.

### 3. Khoá đối tượng

```
{class}/t_{tenant_id}/{yyyy}/{mm}/{service}/{purpose}/{object_id}/{variant}.{ext}
```

| Đoạn | Luật |
|---|---|
| `class` | Đứng **đầu** để thao tác theo tiền tố từng lớp làm được trong một bucket: `records` · `citizen-media` · `content-source` · `public-media` … (danh sách đóng) |
| `t_{tenant_id}` | Tiền tố xã (luật 1 bất biến 7). **Ngoại lệ chính tả có chủ đích** — xem dưới |
| `yyyy/mm` | Tháng **tạo đối tượng**. Chỉ cho thao tác hàng loạt và thống kê — **không phải mốc lưu giữ** |
| `service`, `purpose` | Danh sách đóng, tiếng Anh. Một giá trị `purpose` **không bao giờ trùng tên một service** (lý do ở §*Cái giá*) |
| `object_id` | ULID do máy chủ sinh |
| `variant.ext` | `original`, `mp4-720p`, `poster`, `thumb-320`…; đuôi lấy từ kiểu **đã dò**, không từ tên tệp |

Ký tự cho phép: `[a-z0-9_-./]`. Mỗi service có access key MinIO riêng, IAM wildcard giới hạn vào
đoạn `…/{service}/*` của mình.

**Vì sao `t_` chứ không `t:`.** Luật 1 bất biến 7 viết `t:<tenant_id>`. Trong khoá S3, `:` phải mã
hoá đặc biệt ở chữ ký, ở CDN và trong URL; mỗi tầng mã hoá khác nhau là một chỗ khoá lệch. Giữ
**ý** của bất biến (mọi đối tượng mang tiền tố xã, thao tác theo xã làm được bằng tiền tố), đổi
**chính tả** thành `t_` — chỉ cho khoá đối tượng. Cache, phòng realtime, hàng đợi vẫn `t:`.

**Vì sao tên tệp gốc không bao giờ nằm trong khoá.** Tên tệp do người dùng đặt có thể chứa họ tên
một người (`don-nguyen-van-a.pdf`) — luật 3 cấm #4 (dữ liệu cá nhân trong đường dẫn, URL). Tên gốc
chỉ nằm trong metadata và trả về qua `Content-Disposition` đã làm sạch.

### 4. Tên miền

Dùng tên miền MinIO do devops cấp, đọc từ cấu hình — **không** thêm nhãn vào quy hoạch tên miền xã
của ADR 0046.

| Biến | Vai trò |
|---|---|
| `OBJECT_STORAGE_ENDPOINT` | Service gọi MinIO (nội bộ) |
| `OBJECT_STORAGE_PUBLIC_ENDPOINT` | Endpoint S3 trình duyệt thấy — presigned POST/GET ký theo host này |
| `OBJECT_STORAGE_PUBLIC_MEDIA_BASE_URL` | URL bucket public cho media Mini App, đặt CDN được, `Cache-Control: immutable` |

Nội dung người dùng tải lên **không bao giờ** phục vụ từ tên miền app của xã: không cookie nào đi
kèm, và một tệp HTML/SVG độc không chạy được trong origin của xã. Phản hồi luôn `nosniff`; không
phải media thì `attachment`. Tên miền MinIO phải vào **danh sách tên miền của app Zalo** (kho
`vihat-miniapp`) và CORS cho phép tên miền web của các xã + origin Zalo.

### 5. Metadata `stored_file`

| Cột | Ghi chú |
|---|---|
| `tenant_id`, `bucket`, `object_key` | |
| `retention_class`, `purpose`, `subject_type`, `subject_id` | Bản ghi nghiệp vụ mà tệp gắn vào |
| `original_name` | **Dữ liệu cá nhân** khi công dân tải (luật 3) — che khi ra API/log |
| `mime_type` (đã dò), `size_bytes`, `sha256` | |
| `status` | `pending → scanning → stored → processing → ready` \| `failed` \| `rejected` → `purged` |
| `uploaded_by` | Mã cán bộ (luật 6 bất biến 8) hoặc tham chiếu phiên công dân |
| `retain_until` | Tính **MỘT LẦN** ở hành vi cố định mốc (ví dụ phiếu phản ánh đóng) — cùng nguyên tắc luật 10 bất biến 2. Không tính lại khi đọc |
| `legal_hold` | Đang giữ vì khiếu nại/thanh tra → không purge |
| `purged_at`, `purged_by`, `purge_reason` | |
| `deleted_at`, `deleted_by`, `delete_reason` | Xoá mềm **dòng** metadata (luật 7) |

### 6. Lưu giữ và xoá

Bảng cấu hình lưu giữ **theo lớp**, toàn nền tảng. **Giá trị là của khách/pháp chế** — dưới đây là
giữ chỗ, **chưa chốt**:

| Lớp | Giữ chỗ (chưa chốt) |
|---|---|
| `citizen-media` | 24 tháng sau khi phiếu đóng |
| `content-source` | 12 tháng sau khi gỡ đăng |
| `records` | Theo danh mục lưu trữ; **không bao giờ tự động** |

**Xoá do ứng dụng điều khiển:** mỗi service có worker purge chạy dưới **system principal** (luật 6
bất biến 6): chọn `retain_until < now AND NOT legal_hold` → xoá **mọi phiên bản** → `purged` + vết;
lặp lại an toàn (idempotent). Vòng đời MinIO **chỉ** cho bucket temp. **Dòng nghiệp vụ được giữ**
(luật 7) — chỉ đối tượng đính kèm đã hết hạn bị purge.

Vì sao không dùng vòng đời MinIO cho tệp nghiệp vụ: vòng đời đếm từ ngày tạo đối tượng, còn mốc
lưu giữ là hành vi nghiệp vụ (phiếu đóng) — và MinIO không biết `legal_hold`, không ghi vết, không
cập nhật metadata. CSDL và kho sẽ lệch nhau trong im lặng.

### 7. Công cụ vận hành `vigovctl storage purge`

```
vigovctl storage purge --domain <tên miền xã> --class <class> --created-before <ngày> --ticket <mã> [--dry-run mặc định]
```

- Phân giải tên miền → xã qua `platform`; chọn theo **metadata**, không theo tiền tố trần, để CSDL
  và MinIO khớp nhau.
- Từ chối lớp `records` và mọi dòng `legal_hold`. Ghi vết kèm mã phiếu.
- Mặc định chạy thử; phải có cờ tường minh mới xoá thật.
- Phê duyệt theo quy trình vận hành chung của Vihat (ngoài hệ thống).
- **Không mở đường vòng luật 2:** công cụ không mở CSDL của service nào; nó gọi RPC purge của từng
  service sở hữu (hợp đồng mới).
- Xoá dữ liệu của một xã **ngoài thời hạn lưu giữ** vẫn là điều kiện dừng (luật 1 dừng #5, luật 7
  cấm #1).
- Dung lượng theo xã = `sum(size_bytes)`, đối chiếu với dung lượng theo tiền tố.

### 8. Mã hoá lúc nghỉ — CHƯA làm

Chủ dự án chốt: chưa. **Cái giá:** ai lấy được đĩa hoặc bản sao lưu MinIO đọc được ảnh hiện trường,
bản quét hồ sơ của công dân ở dạng rõ. **Thêm về sau:** KES + KMS, bật mã hoá mặc định của bucket
(SSE-KMS). Chỉ đối tượng **ghi sau** mới được mã hoá — đối tượng cũ phải chép đè lên chính nó
(server-side copy), tức một đợt di trú chạy theo xã, có ghi tiến độ (luật 7 bất biến 5).

### 9. Quét mã độc — ClamAV

`clamd` là **phụ thuộc hạ tầng mới**, biến `MALWARE_SCANNER_ADDRESS` (dạng cụm, luật 11 bất biến
5). Quét ở bước complete, **trước khi** đối tượng rời temp. Máy quét không tới được → **fail
closed**: từ chối hoặc cho thử lại, **không bao giờ** lưu tệp chưa quét.

### 10. Giới hạn kích thước và kiểu theo mục đích

Vihat cấu hình ở khu vận hành (ADR 0048); **`platform` sở hữu**; service đọc qua **một hợp đồng
platform mới** (contract-designer thiết kế — luật 2 dừng #2). **Không mặc định âm thầm trên giới
hạn bảo mật:** thiếu cấu hình cho một mục đích → **từ chối tải lên** cho mục đích đó. Giá trị đầu
đề xuất:

| Mục đích | Đề xuất |
|---|---|
| Video nội dung | ≤ 2 GB, mp4/mov |
| Ảnh | ≤ 10 MB, jpg/png/webp/heic |
| PDF | ≤ 50 MB |
| Ảnh phản ánh của công dân | ≤ 5 tệp × 10 MB |

### 11. Xử lý video — ngay, không hàng đợi

Lúc complete, service sở hữu (`comms` cho nội dung Mini App) chạy `ffprobe`/`ffmpeg` **ngay**:
chuyển mã MP4 H.264/AAC + poster; `processing → ready | failed`. Lưới an toàn: khởi động lại thì
service tiếp tục các tệp kẹt ở `processing`. **Đăng** thì chép bản dẫn xuất sang bucket public;
**gỡ đăng** thì xoá bản public; bản gốc luôn ở private.

### 12. Tệp công dân tải (ảnh phản ánh)

**Chờ cầu phiên công dân** (`kb/90-ephemeral/tien-do/citizen-app.json`, mục
`cau-phien-cong-dan-vigov`). Link đọc ký, sống ngắn, **chỉ cấp sau khi** kiểm danh tính phiên + xã
(luật 4 bất biến 7); bỏ EXIF; giới hạn tần suất theo công dân.

**02/10/2026:** điều chờ ấy đã thoả cho **app riêng của xã** — app ấy mở thẳng phiên công dân ViGov từ
commit `8df525c9` (ADR 0066); việc dựng do dòng *"Ảnh hiện trường khi gửi phản ánh"* trong bảng mục 6
của ADR 0047 quyết.

## Cái giá

| Điều | Nội dung |
|---|---|
| Presigned URL là bearer | Ràng buộc danh tính + xã xảy ra **lúc cấp**. Trong TTL, ai cầm URL cũng đọc được — giảm bằng TTL ngắn, không triệt tiêu |
| IAM wildcard vượt `/` | `*` của IAM khớp cả `/`, nên `…/*/comms/*` cũng khớp một khoá có `purpose` tên `comms`. Vì thế `purpose` không bao giờ trùng tên service (§3) — phải kiểm bằng danh sách đóng |
| Bỏ EXIF mất toạ độ GPS | Vị trí hiện trường phải đến từ trường vị trí của phiếu, không từ ảnh |
| Xử lý video trong tiến trình service | Tệp 2 GB chiếm CPU của pod `comms`; không hàng đợi thì không san tải. Đo trước khi thêm hàng đợi |
| Bucket public lộ `tenant_id` trong URL | `tenant_id` là ULID mờ (luật 1 bất biến 2) — không phải bí mật, nhưng nhìn URL biết hai tệp cùng xã |
| Versioning bật ở private | Phiên bản không hiện hành vẫn tốn chỗ; purge phải xoá **mọi phiên bản**, không thì dấu xoá che dữ liệu còn đó |

## Còn mở — chưa ai quyết

| # | Việc | Của ai |
|---|---|---|
| 1 | **Giá trị lưu giữ** từng lớp (§6) — cả ba dòng là giữ chỗ | Khách / pháp chế |
| 2 | **Hợp đồng platform** cho giới hạn theo mục đích và bảng lưu giữ theo lớp (§6, §10): hình dạng RPC, platform không tới được thì sao (đề xuất: từ chối tải lên, như ADR 0012) | contract-designer, chủ dự án duyệt |
| 3 | **Danh sách tên miền của app Zalo** cho tên miền MinIO — nằm ở kho `vihat-miniapp`, kho này không sửa được | Chủ dự án |
| 4 | **Cỡ ClamAV**: số replica, bộ nhớ (cơ sở dữ liệu chữ ký ~1 GB RAM mỗi `clamd`), giới hạn `StreamMaxLength` phải ≥ 2 GB cho video — hoặc quyết không quét video | Devops |
| 5 | `--created-before` của `vigovctl` chọn theo **ngày tạo**, không theo `retain_until`. Cách đọc của người viết: công cụ **chỉ** xoá dòng đã qua `retain_until`, không có cờ vượt — vì xoá trước hạn chính là điều kiện dừng ở §7. Cần chủ dự án xác nhận | Chủ dự án |
| 6 | Luật 1 bất biến 7 chưa ghi ngoại lệ `t_` của §3 — cần một dòng trỏ về ADR này trong tệp luật | Phiên sửa `.claude/` |

## Hệ quả

- **Biến cấu hình mới phải qua `core/config` + `.env.example`** (luật 11 bất biến 1, 6). Kho chưa
  có biến nào, nên đây là **luật 11 dừng #1** (phụ thuộc mới) và **dừng #2** (bắt buộc). Đề xuất:

  | Biến | Nguồn | Bắt buộc |
  |---|---|---|
  | `OBJECT_STORAGE_ENDPOINT` · `OBJECT_STORAGE_PUBLIC_ENDPOINT` · `OBJECT_STORAGE_PUBLIC_MEDIA_BASE_URL` · tên ba bucket | ConfigMap | **Chỉ ở service sở hữu tệp**; tuỳ chọn ở service khác |
  | `OBJECT_STORAGE_ACCESS_KEY` · `OBJECT_STORAGE_SECRET_KEY` (`secret.Secret`, riêng từng service) | Secret | Như trên |
  | `MALWARE_SCANNER_ADDRESS` | ConfigMap | Ở service nhận tải lên |

  Lý do "bắt buộc ở service sở hữu tệp": service ấy khởi động thiếu kho là một tính năng hỏng
  **im lặng** tới lần tải đầu; không khởi động được là tín hiệu ồn. **Căng với luật 11 bất biến 8**
  (bắt buộc = không phục vụ nổi một yêu cầu nào) — `comms` thiếu kho vẫn phục vụ tin bài. Chủ dự án
  chọn: bắt buộc lúc khởi động, hay tuỳ chọn + mọi lần tải lên từ chối. Lý do phải ghi cạnh biến.
- **Địa chỉ dạng cụm** (luật 11 bất biến 5): client MinIO nhận một endpoint. Nhiều host trong
  danh sách mà `core/storage` chưa biết dùng thế nào → **từ chối khởi động**, không lặng lẽ lấy
  host đầu (luật 11 cấm #5).
- **Presigned ký theo `OBJECT_STORAGE_PUBLIC_ENDPOINT`**, không theo endpoint nội bộ — chữ ký S3
  gồm host; ký sai host là mọi lần tải của trình duyệt trả 403.
- **Dễ hơn:** byte không qua service; một mô hình khoá cho mọi lớp tệp; xoá có vết và khớp CSDL.
- **Khó hơn:** thêm hai thứ phải vận hành (MinIO, ClamAV); mỗi service sở hữu tệp thêm bảng, worker
  purge, RPC purge.

## Thứ tự dựng

1. `core/storage` + biến cấu hình trong `core/config` + `.env.example` → kiểm bằng ca thử với MinIO thật
2. Video nội dung ở `comms` (tải lên, quét, chuyển mã, đăng/gỡ đăng)
3. `vigovctl storage` (purge, dung lượng theo xã)
4. Media công dân — **BỊ CHẶN** bởi cầu phiên công dân (§12)

## ĐIỀU KIỆN DỪNG

1. Xoá tệp của một xã **trước** `retain_until`, hoặc lớp `records` — luật 1 dừng #5, luật 7 cấm #1
2. Đưa một bản gốc (không phải bản dẫn xuất đã duyệt) vào bucket public
3. Gửi tệp công dân tới một dịch vụ ngoài lần đầu (OCR, CDN bên thứ ba) — luật 3 dừng #2
4. Một giới hạn tải lên có mặc định trong mã thay vì đọc từ platform
5. Cho một lớp tệp nghiệp vụ vào vòng đời MinIO

→ ADR 0001 (storage là thư viện): `kb/10-decisions/0001-service-decomposition.md:96`
→ ADR 0010 (hạ tầng dữ liệu): `kb/10-decisions/0010-data-infrastructure.md`
→ ADR 0046 (tên miền) · ADR 0048 (khu vận hành; #8 logo — trả lời bởi §2) · ADR 0051 (tên tiếng Anh)
→ Luật 1 bất biến 7 · luật 3 · luật 4 bất biến 7 · luật 6 · luật 7 · luật 8 · luật 11
