---
id: 0009-per-tenant-secret-encryption
tier: T1
source: CURATED
owner: architecture
derived_from_commit: null
expires: null
owns_facts:
  - "cách lưu bí mật riêng của từng xã"
  - "mô hình envelope encryption và nơi đặt khoá gốc"
---

# 0009. Bí mật theo xã: mã hoá phong bì, khoá gốc ngoài CSDL

**Trạng thái:** đã chốt · **Ngày:** 2026-09-16

## Bối cảnh

Một số cấu hình của xã **là bí mật thật**:

| Bí mật | Nguồn |
|---|---|
| Mật khẩu SMTP của xã | `docs/ui-ux/14-cau-hinh.md §10` |
| Khoá API cổng thông tin điện tử | `docs/ui-ux/11-noi-dung-mini-app.md §8` |
| Khoá Zalo OA của xã | ADR 0006 |

Hai luật kéo ngược nhau:

- **Luật 8 #1** — bí mật chỉ nằm ở kho bí mật hoặc môi trường chạy, không nằm trong mã,
  không nằm trong tài liệu
- **Luật 1 #10** — giá trị riêng của xã đọc **lúc chạy**; biến môi trường không dùng được vì
  một tiến trình phục vụ N xã

Kho bí mật ngoài (Vault, cloud KMS) thoả cả hai nhưng thêm một phụ thuộc hạ tầng phải vận
hành, giám sát và vá — với 200+ xã thì chi phí đó là thật.

## Quyết định

**Envelope encryption.** Ciphertext nằm trong CSDL, khoá gốc nằm ngoài.

| Thành phần | Nơi lưu | Ghi chú |
|---|---|---|
| Giá trị bí mật | CSDL, dạng **ciphertext** | AES-256-GCM |
| Khoá dữ liệu theo xã (DEK) | CSDL, đã được KEK bọc | Mỗi xã một DEK riêng |
| Khoá gốc (KEK) | **K8s Secret** hoặc `.env.local` | **Không bao giờ** vào CSDL, không vào git |

| # | Quyết định |
|---|---|
| 1 | Mỗi xã một DEK riêng — lộ một xã không lộ xã khác |
| 2 | DEK lưu trong CSDL ở dạng đã được KEK bọc |
| 3 | KEK chỉ tồn tại trong môi trường chạy; tiến trình không có KEK thì **không giải mã được, và phải từ chối chứ không chạy tiếp** |
| 4 | Thuật toán AES-256-GCM (có xác thực, chống sửa ciphertext) |
| 5 | Xoay KEK chỉ cần bọc lại DEK — không phải giải mã rồi mã hoá lại toàn bộ dữ liệu |
| 6 | Giá trị bí mật **không bao giờ** trả về client; giao diện chỉ hiện dạng che (`****654bf`) |
| 7 | Cần `pkg/crypto` — chưa tồn tại |

## Vì sao đây KHÔNG vi phạm luật 8

Luật 8 #1 cấm bí mật nằm trong **mã nguồn** và trong **tài liệu**. CSDL không phải hai chỗ
đó. Và thứ nằm trong CSDL là ciphertext — không có KEK thì nó không phải bí mật, chỉ là dữ
liệu ngẫu nhiên.

Ranh giới thật của luật 8 là: **bí mật không được nằm ở nơi bị sao chép ngoài tầm kiểm soát.**
Mã nguồn bị sao sang mọi máy clone; tài liệu bị dán vào chat và ticket. Bản sao lưu CSDL thì
có kiểm soát — và nếu bản sao lưu lọt ra ngoài mà không kèm KEK, ciphertext vẫn vô dụng.

Ghi rõ lập luận này ở đây để phiên sau không đọc luật 8 rồi tưởng thiết kế sai.

## Vì sao không chọn Vault ngay

Không bác Vault — bác việc thêm nó **lúc này**. Envelope encryption đạt cùng mục tiêu an
ninh cho quy mô hiện tại, không thêm dịch vụ phải trực. Khi nào cần xoay khoá tự động, cần
nhật ký truy cập khoá, hoặc cần khoá phần cứng, thì chuyển sang Vault/KMS **mà không đổi mô
hình** — chỉ thay chỗ lấy KEK.

## Hệ quả

- **Dễ hơn:** không thêm dịch vụ hạ tầng; triển khai chỉ cần một biến môi trường
- **Khó hơn:** mất KEK là **mất toàn bộ bí mật của mọi xã**. Quy trình sao lưu KEK phải có
  trước khi phát hành, và phải tách khỏi sao lưu CSDL
- **Phải trả sau:** xoay KEK là thao tác vận hành có kịch bản riêng; cần runbook khi có sự cố
  nghi lộ khoá

→ Luật 8: `.claude/rules/critical/8-secrets-config.md`
→ Luật 1: `.claude/rules/critical/1-tenant-isolation.md`
