---
id: 0005-miniapp-tenant-resolution
tier: T1
source: CURATED
owner: architecture
derived_from_commit: null
expires: null
owns_facts:
  - "cách Zalo Mini App xác định xã và khuôn deep link"
  - "ba lớp khám phá – phiên – uỷ quyền ở kênh công dân"
---

# 0005. Mini App xác định xã bằng ba lớp, không bằng routing theo tham số

**Trạng thái:** đã chốt · **Ngày:** 2026-09-15

## Bối cảnh

Đề xuất ban đầu: một Mini App chính, các "app con" routing qua tham số trên QR hoặc link —
chọn app Đà Nẵng, chọn xã, rồi routing tới page ứng với domain của xã đó.

Chẩn đoán đúng (một app, nhiều xã, phân biệt bằng tham số). Nhưng có ba chỗ hở.

## Ba vấn đề của mô hình tham số thuần

| # | Vấn đề | Hệ quả |
|---|---|---|
| 1 | Tham số trên QR/link là **dữ liệu client cung cấp** | Luật 1 cấm nhận `tenant_id` từ client. Client tự khai xã là client tự cấp quyền |
| 2 | "Routing tới domain của xã" đưa client đi **chọn backend** | Tái tạo N cấu hình CORS, N chứng chỉ; và QR đã in hỏng khi xã đổi tên miền |
| 3 | **Phần lớn lần mở app KHÔNG có tham số** | Ghim app, tìm trong Zalo, quay lại tuần sau — đều không mang tham số. Coi tham số là đường chính thì app hỏng ở lần mở thứ hai của mọi người dùng |

Thêm một điểm về "app của Đà Nẵng": không có app đó. Chỉ có **một** Mini App; tỉnh là bộ lọc
trong màn hình chọn. Làm thành app riêng cho từng tỉnh thì 34 lần đăng ký, 34 lần gắn OA — và
ranh giới tỉnh vừa được vẽ lại năm 2025, nên cây khám phá neo vào tỉnh là neo vào thứ đang dịch chuyển.

## Quyết định

Tách ba lớp, không bao giờ gộp:

| Lớp | Trả lời câu | Nguồn | Tin được |
|---|---|---|---|
| **Khám phá** | Công dân *muốn* làm việc với xã nào | QR · deep link · GPS · picker · hồ sơ | **Không** — chỉ gợi ý |
| **Phiên** | Phiên này *đang* thao tác ở xã nào | Server ghi sau khi công dân xác nhận | **Có** — server phát hành |
| **Uỷ quyền** | Công dân này được đọc/ghi gì ở xã đó | Quan hệ công dân↔xã + luật 4 | **Có** |

Hệ quả trực tiếp: Mini App gọi **một API host duy nhất**, không bao giờ dựng URL theo xã.

> *"Client nói đang ở xã nào"* — cấm.
> *"Server ghi nhận phiên này đang thao tác ở xã nào"* — đúng.

### Khuôn deep link

```
https://zalo.me/s/<APP_ID>/?t=<tenant_ulid>&src=qr&v=1
```

| Tham số | Vì sao |
|---|---|
| `t` | **ULID vô nghĩa** (ADR 0004). QR in trên bảng tin sống lâu hơn tên xã, tên miền, và cả xã |
| `src` | `qr` · `zns` · `share` — cân mức tin, và đo kênh nào thật sự hiệu quả |
| `v` | Phiên bản khuôn. QR in ra sống nhiều năm; đổi khuôn thì bản cũ vẫn phải đọc được |

### Mức tin theo nguồn

`qr` và `zns` → chọn sẵn, một chạm xác nhận. `share` → **luôn bắt chọn tường minh**, vì
nguồn gốc không xác định. Người đang đứng tại trụ sở xã không nên bị bắt tìm lại chính xã đó.

### Uỷ quyền theo thao tác

| Thao tác | Căn cứ |
|---|---|
| **Đọc** bản ghi của mình | Danh tính từ phiên **và** quan hệ đã có |
| **Gửi mới** | Mọi xã **đang hoạt động** đều hợp lệ; hành vi gửi tạo quan hệ `TRANSIENT` |
| **Đổi xã** | Hành động tường minh, phát hành lại phiên, **ghi vết** |

Gửi phản ánh tới xã mình không có quan hệ là **hợp lệ** — người dân báo ổ gà nhìn thấy khi đi
ngang. Đó chính là lý do `TRANSIENT` tồn tại trong ADR 0002.

## Yêu cầu phát sinh cho service `platform`

**Bảng alias `tenant cũ → tenant kế thừa`.** QR in cho xã đã sáp nhập vẫn phải mở được, kèm
thông báo "xã này đã sáp nhập vào X". Không có bảng này thì mọi tấm QR đã in thành rác ngay
lần sáp nhập đầu tiên — và sáp nhập là việc **chắc chắn xảy ra**.

Tìm kiếm trong picker cũng tra bảng này: tên người dân biết có thể không còn là tên chính thức.

## Hệ quả

- **Dễ hơn:** thêm một xã không đụng tới Mini App, không build lại, không duyệt lại
- **Khó hơn:** phải thiết kế kỹ đường **không có tham số**, và nó là đường phổ biến nhất
- **Phải trả sau:** bảng alias phải được duy trì mỗi lần sáp nhập — nhưng đó là việc vận hành,
  không phải việc viết mã

→ Kỹ năng: `.claude/skills/zalo-miniapp-multi-tenant`
