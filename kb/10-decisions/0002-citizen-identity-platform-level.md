---
id: 0002-citizen-identity-platform-level
tier: T1
source: CURATED
owner: architecture
derived_from_commit: null
expires: null
owns_facts:
  - "công dân tương tác với nhiều xã và định danh nằm ở tầng nền tảng"
---

# 0002. Định danh công dân ở tầng nền tảng

**Trạng thái:** đã chốt · **Ngày:** 2026-09-15 · **Đóng câu hỏi mở #2**

## Bối cảnh

Mô hình ngây thơ đặt số điện thoại duy nhất theo từng xã, nghĩa là *một công dân thuộc đúng
một xã, vĩnh viễn*. Sai trong đời thật ít nhất bốn cách: thường trú một nơi tạm trú nơi khác;
phản ánh sự cố **nhìn thấy ở xã khác**; chuyển hộ khẩu; nộp hồ sơ thay người thân.

## Quyết định

| Thứ | Phạm vi | Service sở hữu |
|---|---|---|
| `CitizenIdentity` (số điện thoại đã xác thực) | **Toàn nền tảng**, một bản ghi | `danhtinh`, vùng xuyên xã |
| `CitizenCommune` (quan hệ, vai trò `thuong_tru`/`tam_tru`/`vang_lai`, thời hạn) | **Nhiều-nhiều** | `danhtinh`, theo từng xã |
| Phiên đăng nhập | Mang **một xã đang chọn**, đổi được, mọi lần đổi ghi vết | `danhtinh` |

Khoá duy nhất: số điện thoại **toàn nền tảng**; quan hệ **(công dân, xã)**.
Kho OTP khoá theo **(số điện thoại, xã)** — xin OTP ở xã A không chặn xã B.

## Hệ quả

- `danhtinh` là service **duy nhất** có vùng dữ liệu xuyên xã hợp lệ ngoài `nentang`. Vùng đó
  phải nằm trong thư mục được đánh dấu tường minh, không rải trong mã theo từng xã
- Backend **không tin** xã do client gửi — đối chiếu với quan hệ trong phiên
- Định danh OTP vẫn là **danh tính yếu**: không đủ một mình cho hành vi có hậu quả pháp lý

## Phải trả nếu quyết ngược lại sau này

Di trú dữ liệu công dân trên toàn hệ thống, và mọi khoá ngoại trỏ tới công dân phải đổi.
