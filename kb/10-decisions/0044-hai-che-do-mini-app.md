---
id: 0044-hai-che-do-mini-app
tier: T1
source: CURATED
owner: architecture
derived_from_commit: ec3801f
expires: null
owns_facts:
  - "hai chế độ Mini App — app chính mở xã bằng QR, app riêng gắn cứng một xã theo App ID — dùng chung một bản build citizen-app"
  - "xã của app riêng do máy chủ suy ra từ App ID mà app secret xác minh được, qua bảng app_id → tenant_id của platform"
  - "mở app chính không kèm QR thì không vào được nội dung của xã nào"
---

# 0044. Hai chế độ Mini App, một bản build `citizen-app`

**Trạng thái:** đã chốt · **Ngày:** 2026-09-25 · **Thay thế một phần ADR 0005** (chỉ những điểm
liệt kê ở §*Thay thế gì*; ADR 0005 giữ nguyên từng chữ). ADR 0018 **không** bị thay — xem câu 5

## Bối cảnh

ADR 0005 chốt **một** Mini App cho mọi xã. ADR 0018/0031 chốt **một** OA xác thực cho app ấy.
Ngày 25/09/2026 chủ dự án chốt cách tiếp cận mới gồm hai giai đoạn:

| Giai đoạn | App | Dùng khi |
|---|---|---|
| **Demo** | App chính do `vihat-miniapp` sở hữu. Nộp lên kho Zalo. QR riêng mở vào đúng một xã | Trình diễn, trước khi có hợp đồng |
| **Chính thức** | **Mỗi xã một app riêng** (App ID riêng), nộp Zalo kèm công văn xác thực của xã | Sau khi ký hợp đồng và có công văn |

Ràng buộc chủ dự án đặt ra: **không** build `citizen-app` riêng cho từng xã. Cái gì khác nhau giữa
các xã thì cấu hình động trong database theo xã; còn lại giữ chung một khung.

Các câu đã được chủ dự án trả lời ngày 25/09/2026:

| # | Câu | Trả lời |
|---|---|---|
| 1 | Pháp nhân đứng tên các app riêng | **ViHAT Group** — không phải UBND xã |
| 2 | Pháp nhân vận hành backend | **ViHAT Group**. Toàn bộ chạy trên cloud do ViHAT quản lý. Đây là **lần xác nhận của người** mà câu hỏi mở #28 chờ trước khi nộp Zalo |
| 3 | Công dân dùng app của xã nào | **Chỉ tương tác với đúng xã đó**, không liên quan xã khác |
| 4 | App chính có người dân thật dùng không | **Có** — nộp lên kho. Nhưng chỉ xem được nội dung xã khi mở bằng QR riêng do ta phát hành |
| 5 | Một OA xác thực có xác thực được **nhiều** Mini App không | **Có** — chủ dự án xác nhận *"hiện đang làm được"*. Người phát triển xin quyền quản trị OA để đứng admin cả hai phía. Vậy OA `Vihat` của ADR 0031 xác thực mọi app; ADR 0018 đứng nguyên |
| 6 | App chính: đã quét QR xã A, tuần sau mở lại từ danh sách ghim (không QR) | **Quay lại đúng xã A** |

## Các phương án

| Phương án | Được | Mất |
|---|---|---|
| Build riêng cho từng xã (nhánh mã, cờ build, hằng số theo xã) | Đơn giản lúc đầu | N bản build trôi lệch nhau. Sửa lỗi phải phát hành N lần. Đưa giá trị theo xã vào bundle — trái luật 1 bất biến 10 |
| Client tự đọc App ID rồi tự chọn xã | Không cần sửa máy chủ | App ID do client khai là dữ liệu client cung cấp — trái luật 1 cấm #2 |
| **Một bản build. Xã do máy chủ suy ra, cấu hình hiển thị đọc lúc chạy** | Thêm xã không đụng mã. Một lần sửa lỗi phủ mọi app | Phải dựng cầu phiên và bảng `app_id → tenant_id` trước |

## Quyết định

**Một bản build `citizen-app` được nộp vào nhiều App ID. Chế độ và xã của mỗi phiên do MÁY CHỦ
quyết định, không do bản build hay tham số client.**

### Hai chế độ

| | App chính (demo) | App riêng của xã |
|---|---|---|
| Xã lấy từ | QR `t=<tenant_ulid>` (khuôn ADR 0005) → công dân xác nhận → máy chủ ghi vào phiên | App ID → bảng `app_id → tenant_id` → máy chủ gắn sẵn vào phiên |
| Mở không kèm QR | Đã từng xác nhận một xã → **quay lại xã ấy**. Chưa từng → **chỉ màn giới thiệu Tập đoàn**. Không có màn chọn xã | Vào thẳng xã của app |
| Ai nhớ "xã đã xác nhận" | **Máy chủ**, theo danh tính công dân sau khi đăng nhập. Client không lưu xã xuống máy | Không cần nhớ |
| Đổi xã | Không | Không |
| Gửi phản ánh tới xã khác | Không | Không |

### Xã của app riêng — vì sao phải là máy chủ

Mỗi app có App ID và app secret riêng. `vihat-miniapp` đổi `accessToken` bằng secret nào thành
công thì **máy chủ biết chắc** token đến từ app nào. Từ App ID đã xác minh ấy, ViGov tra
`app_id → tenant_id` rồi phát hành phiên công dân gắn xã.

- Tra không ra, hoặc xã không hoạt động → **từ chối**. Không có xã mặc định (luật 1 cấm #1).
- App ID mà client đọc từ URL chỉ được dùng để **dẫn giao diện**, không bao giờ cấp gì.
- Bảng `app_id → tenant_id` thuộc **service platform**, vì `tenant_id` là khái niệm của ViGov.
  `vihat-miniapp` chỉ khẳng định "token này thuộc app X". Nó không biết xã.
- N app secret nằm trong kho bí mật của **một** bản chạy `vihat-miniapp` (luật 8).

### Cái gì cấu hình động, cái gì không

| Cấu hình động, đọc lúc chạy theo `tenant_id` của phiên | Không cấu hình động được |
|---|---|
| Tên xã, địa chỉ, logo, đường dây nóng, giờ làm việc, lĩnh vực, nội dung giới thiệu xã, SLA (đã theo xã sẵn — ADR 0007) | Tên app, icon app và liên kết tới OA xác thực: khai trên trang quản trị Zalo cho từng App ID, không nằm trong bundle |

Thông báo ZNS **không đổi**. Tin vẫn gửi từ OA của từng xã qua `service-comms` (ADR 0018 vai trò
thứ hai, ADR 0031 §*Cái gì KHÔNG đổi*).

## Thay thế gì của ADR cũ

| ADR | Điểm bị thay | Thay bằng |
|---|---|---|
| 0005 | *"Chỉ có **một** Mini App"* | Một app chính **cộng** N app riêng, chung một bản build |
| 0005 | Đường không tham số là màn chọn xã (picker, GPS, hồ sơ) | App chính: chỉ giới thiệu. App riêng: xã cố định |
| 0005 | Gửi mới tới mọi xã đang hoạt động, tạo quan hệ `CAPACITY_TRANSIENT` | Phiên chỉ thao tác với **một** xã. Không gửi sang xã khác |

**Không thay:** ba lớp khám phá – phiên – uỷ quyền của 0005. Một API host duy nhất. Khuôn deep
link `t`/`src`/`v`. Bảng alias xã sáp nhập. Toàn bộ ADR 0018: một OA xác thực (`Vihat`, ADR 0031)
cho mọi app, tách khỏi OA gửi thông báo của từng xã.

## Hệ quả

- **Dễ hơn:** thêm một xã = một dòng `app_id → tenant_id` + cấu hình hiển thị. Không build lại.
- **Khó hơn:** cầu phiên `vihat-miniapp` → phiên công dân ViGov (mục sổ tiến độ
  `citizen-app/cau-phien-cong-dan-vigov`) phải mang App ID đã xác minh. Đây là việc chung của hai kho.
- **Phải trả sau:** mỗi xã chính thức là một hồ sơ nộp Zalo kèm công văn của xã, và một lần liên
  kết App ID mới với OA `Vihat`. Đó là thủ tục, không phải mã.
- **Nhớ xã ở app chính cần danh tính:** máy chủ chỉ biết "công dân này đã xác nhận xã A" sau khi
  biết công dân là ai. Thiết kế cầu phiên phải trả lời mở lại không QR thì nhận danh tính bằng
  cách nào trước khi quyết định hiện giới thiệu hay vào xã A.
- Chính sách riêng tư dùng chung mọi app, vì cùng một pháp nhân đứng tên và vận hành.

## ĐIỀU KIỆN DỪNG

1. Có đề xuất đặt **giá trị theo xã vào bản build** — biến môi trường, hằng số, nhánh mã, tệp cấu hình theo App ID
2. Có đề xuất cho client **tự chọn xã** từ App ID hay tham số URL
3. Có xã muốn **UBND xã đứng tên** app riêng — đổi bên chịu trách nhiệm dữ liệu, cần ADR mới
4. Có yêu cầu một công dân trong một app thao tác với **nhiều xã**

→ ADR 0005: `kb/10-decisions/0005-miniapp-tenant-resolution.md`
→ ADR 0018: `kb/10-decisions/0018-oa-xac-thuc-tach-khoi-oa-thong-bao.md`
→ ADR 0031: `kb/10-decisions/0031-chuyen-phap-nhan-mini-app-sang-vihat-group.md`
→ Câu hỏi mở #28: `kb/00-foundation/open-questions.json`
→ Kỹ năng: `.claude/skills/zalo-miniapp-multi-tenant/SKILL.md`
