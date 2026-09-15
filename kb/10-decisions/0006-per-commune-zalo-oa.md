---
id: 0006-per-commune-zalo-oa
tier: T1
source: CURATED
owner: architecture
derived_from_commit: null
expires: null
owns_facts:
  - "mỗi xã dùng Official Account riêng để gửi thông báo cho công dân"
---

# 0006. Mỗi xã một Zalo OA riêng

**Trạng thái:** đã chốt · **Ngày:** 2026-09-15

## Bối cảnh

Thông báo gửi công dân đi qua Zalo OA. Có hai hướng:

| Hướng | Được | Mất |
|---|---|---|
| **Một OA tập trung** của nhà cung cấp | Vận hành đơn giản: một bộ khoá, một chỗ cấu hình | Người gửi hiện ra không phải xã. Người dân nhận tin về hồ sơ của mình từ một cái tên họ không biết |
| **Mỗi xã một OA** | Người gửi là **"UBND xã X"** — đúng kỳ vọng của người dân và đúng thẩm quyền | Vận hành nặng hơn: mỗi xã tự đăng ký OA, tự giữ khoá; onboard một xã có thêm một bước |

## Quyết định

**Mỗi xã một OA riêng.**

Uy tín của một cơ quan công quyền với chính người dân của mình là thứ nền tảng này tồn tại để
phục vụ, không phải thứ để đánh đổi lấy tiện lợi vận hành. Một thông báo về hồ sơ hành chính
đến từ người gửi lạ là một thông báo người dân sẽ nghi ngờ hoặc bỏ qua.

## Hệ quả kiến trúc

| # | Hệ quả |
|---|---|
| 1 | Kênh thông báo nằm sau **adapter**; mã nghiệp vụ không bao giờ nhắc tên Zalo |
| 2 | Khoá OA là **cấu hình theo xã**, nằm trong kho bí mật — không nằm trong tệp cấu hình, không nằm trong biến môi trường của tiến trình (luật 1 bất biến 10, luật 8) |
| 3 | Xã **chưa cấu hình OA** phải suy giảm **nhìn thấy được**: cán bộ thấy cờ "chưa báo được", không bao giờ im lặng |
| 4 | Gửi thông báo **nhất quán cuối cùng** với ghi nghiệp vụ: đơn thư không bao giờ được từ chối tiếp nhận chỉ vì không gửi được tin |
| 5 | Nội dung thông điệp mang **mã nghiệp vụ**, không mang dữ liệu cá nhân — hàng đợi được lưu và sao lưu (luật 3) |
| 6 | Onboard một xã có thêm bước "đăng ký và gắn OA" — phải nằm trong quy trình của `platform` |

Service `comms` sở hữu adapter này.

## ⚠ Điều kiện tiên quyết CHƯA kiểm chứng

Quyết định này giả định **một Mini App làm việc được với nhiều OA**. Chưa ai tra tài liệu Zalo
để xác nhận.

**Phải kiểm trước khi viết dòng mã đầu tiên của kênh thông báo.** Nếu Zalo ràng buộc một Mini
App với đúng một OA, quyết định này **không thực hiện được như đang mô tả**, và hình dạng phải
đổi — khi đó quay lại đây và viết ADR thay thế, không sửa tệp này.

Các câu khác cần tra cùng lúc:

| # | Câu hỏi | Ảnh hưởng |
|---|---|---|
| 1 | Một Mini App gắn được nhiều OA không? | Chính điều kiện tiên quyết ở trên |
| 2 | Tham số deep link có tới app trong mọi đường mở không? | Mức tin của lớp khám phá (ADR 0005) |
| 3 | QR do Zalo sinh hay tự sinh? Giới hạn độ dài, định dạng? | Khuôn tham số (ADR 0005) |
| 4 | `getPhoneNumber` có bị ràng buộc theo OA không? | Luồng OTP công dân |

→ Kỹ năng: `.claude/skills/zalo-miniapp-multi-tenant`
