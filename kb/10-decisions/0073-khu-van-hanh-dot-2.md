---
id: 0073-khu-van-hanh-dot-2
tier: T1
source: CURATED
owner: architecture
derived_from_commit: e764bc08
expires: null
owns_facts:
  - "khu vận hành: người có BẤT KỲ khoá ops.* nào xem được danh sách và chi tiết xã; quyền SỬA vẫn tách theo từng khoá — thay việc mọi tuyến đọc xã đòi ops.tenant.manage (chốt 04/10/2026)"
  - "khu vận hành có màn xem nhật ký vận hành (chỉ đọc, lọc theo xã và thời gian, không dữ liệu cá nhân), ai có khoá ops.* cũng xem (chốt 04/10/2026)"
  - "mã lĩnh vực phản ánh cấp 1 (ADR 0060) sửa ở khu vận hành dưới khoá vận hành thứ bảy ops.petition_field.manage (chốt 04/10/2026)"
  - "hồ sơ hiển thị + logo xã KHÔNG sửa ở khu vận hành — bỏ mục đợt 2 'hồ sơ hiển thị + logo' của ADR 0048, vì ADR 0069 chốt chỉ xã tự sửa (chốt 04/10/2026)"
---

# 0073. Khu vận hành đợt 2 — quyền đọc xã, nhật ký vận hành, lĩnh vực cấp 1

**Trạng thái:** đã chốt · **Ngày:** 2026-10-04 · **Người quyết:** chủ dự án, 04/10/2026 · **Bổ sung**
ADR 0048 (đợt 2 của khu vận hành).

## Quyết định

| # | Điểm | Chốt |
|---|---|---|
| 1 | Ai xem được xã | **Bất kỳ khoá `ops.*` nào**. Người chỉ có `ops.mini_app.manage` vẫn mở được xã để làm việc của quyền mình. Quyền **sửa** vẫn theo từng khoá; không thêm khoá đọc |
| 2 | Nhật ký vận hành | **Có màn xem**: chỉ đọc, lọc theo xã và thời gian, không dữ liệu cá nhân, ai có khoá `ops.*` cũng xem — trong chi tiết xã và một màn chung |
| 3 | Mã lĩnh vực phản ánh cấp 1 (ADR 0060) | Sửa ở khu vận hành, dưới **khoá thứ bảy `ops.petition_field.manage`** (thêm vào danh mục khoá vận hành, cấp cho từng người) |
| 4 | Hồ sơ hiển thị + logo ở khu vận hành | **Bỏ** — ADR 0069: chỉ xã tự sửa |
| 5 | Giới hạn tải lên, phát hành QR | Làm theo khoá đã có `ops.upload_policy.manage`, `ops.qr.issue` (ADR 0048, 0052 §10) |

**Vì sao đọc theo bất kỳ khoá:** một người vận hành được giao việc Mini App mà không mở được xã thì
không làm được việc của chính quyền mình; danh sách xã không chứa dữ liệu cá nhân của dân.

## Hệ quả

- Khoá vận hành thứ bảy: thêm dòng vào danh mục khoá vận hành của identity bằng migration và vào tập
  khoá đóng của `service-platform/internal/opauth`; job Jenkins `tao-tai-khoan-van-hanh` đang cấp đủ
  6 khoá — cập nhật thành 7.
- Thao tác chuyển khoá bí mật App ID cũng ghi một dòng vết ở platform (chỉ siêu dữ liệu) để màn nhật
  ký vận hành thấy được; dòng vết chính vẫn ở identity (ADR 0070 bổ sung #7).

## Sửa đổi 05/10/2026

Mục này ghi thêm, không sửa phần trên. **#5 — phát hành QR:** liên kết trả về nay **theo app riêng
của xã** — xã có app riêng đang sống thì QR mở thẳng app ấy, không có thì app dùng chung như cũ. Điều
này do ADR 0070 §*Sửa đổi 05/10/2026* #4 sở hữu (chủ dự án, 05/10/2026). Khoá `ops.qr.issue` và việc
không ghi vết giữ nguyên.
