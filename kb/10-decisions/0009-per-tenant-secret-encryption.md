---
id: 0009-per-tenant-secret-encryption
tier: T1
source: CURATED
owner: architecture
derived_from_commit: 45f4f11
expires: null
owns_facts:
  - "cách lưu bí mật riêng của từng xã"
  - "mô hình envelope encryption và nơi đặt khoá gốc"
  - "vì sao SECRET_ENCRYPTION_KEYS tuỳ chọn lúc khởi động mà sao lưu KEK vẫn là cổng phát hành"
  - "thứ tự xoay KEK và vì sao gỡ khoá cũ sớm là mất bí mật"
  - "vì sao ngược kho yêu cầu: mật khẩu SMTP không lưu dạng rõ"
---

# 0009. Bí mật theo xã: mã hoá phong bì, khoá gốc ngoài CSDL

**Trạng thái:** đã chốt · **Ngày:** 2026-09-16

> 28/09/2026 → quyết định #7 đã xong: `core/crypto` có ở commit `45f4f11`, biến
> `SECRET_ENCRYPTION_KEYS`. Cách vận hành, thứ tự xoay khoá, cổng sao lưu và phần còn nợ: §*Sửa
> đổi 28/09/2026* cuối tệp. Phần trên giữ nguyên văn ngày 16/09.

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
| 7 | Cần `core/crypto` — chưa tồn tại |

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

## Sửa đổi 28/09/2026 — `core/crypto` đã dựng (commit `45f4f11`)

**Không sửa quyết định ở trên.** Mục này ghi cái đã dựng và cái giá của nó. Quyết định #7
(*"Cần `core/crypto` — chưa tồn tại"*) nay đã có: `core/crypto/keyring.go`, `core/crypto/envelope.go`.
Người dùng duyệt biến mới ngày 28/09/2026 (menu Cấu hình §10).

### Biến `SECRET_ENCRYPTION_KEYS`

| Điều | Nội dung | Vì sao |
|---|---|---|
| Đối tượng k8s | **Secret**, mỗi service giữ bí mật của xã một Secret riêng (`comms` trước tiên — mật khẩu SMTP) | Là khoá mở mọi bí mật của mọi xã (luật 11 bất biến 7) |
| Một giá trị hay nhiều | **Khuyến nghị mỗi service một giá trị riêng** — mã không cưỡng chế | Lộ một giá trị chỉ lộ bí mật của một service |
| Bắt buộc? | **Tuỳ chọn lúc `Load`** (`core/config/secret_encryption.go:14-18`) | Bắt buộc thì mọi service dừng trên mọi máy chưa khai, để bảo vệ một tính năng |
| Thiếu | Mọi thao tác niêm/mở bí mật **từ chối** bằng `crypto.ErrNotConfigured`, lỗi nêu tên biến (`core/crypto/keyring.go:74-77`) | Quyết định #3: không bao giờ lưu dạng rõ, không bỏ qua |
| Sai dạng, trùng mục | **Pod không khởi động** (`core/config/operator.go:84-107`) | Một khoá sai phát hiện lúc triển khai, không phải lúc cán bộ bấm Lưu |
| Định dạng | Cùng dạng `OPERATOR_TOTP_ENCRYPTION_KEY`, một bộ phân tích cho cả hai: danh sách ngăn bằng dấu phẩy, mỗi mục base64 chuẩn của **đúng 32 byte** (`openssl rand -base64 32`), **mới nhất đứng đầu**. Khoá đầu bọc DEK mới, mọi khoá mở được | Người vận hành đã xoay một biến thì biết xoay biến kia |

**Mã khoá (`kek_id`)** là 4 byte suy từ chính khoá bằng HMAC, lưu thành 8 ký tự hex, **không phụ
thuộc vị trí trong danh sách** (`core/crypto/keyring.go:55-58`). Vì xoay là *thêm khoá mới lên đầu*: mã theo vị
trí sẽ đánh số lại mọi khoá cũ đúng lúc ấy.

**DEK không bao giờ xoay** — chỉ bọc lại dưới KEK mới (`core/crypto/envelope.go:85-87`). Byte phiên bản ở đầu
định dạng để ngỏ một bản 2 về sau nếu cần xoay chính DEK.

**Bảng DEK thuộc từng service** (luật 2): mỗi service tự cài `DEKStore` trên bảng của mình,
`tenant_id` là khoá duy nhất, **không bao giờ xoá dòng** — xoá một DEK là huỷ mọi bí mật của xã
ấy (`core/crypto/envelope.go:36-52`, luật 7).

### Xoay KEK — thứ tự bắt buộc

1. Thêm khoá mới lên **đầu** danh sách, triển khai. DEK mới bọc dưới khoá mới; DEK cũ vẫn mở được.
2. Chạy `Envelope.RewrapDEK` cho **từng xã**.
3. Xác nhận **không còn dòng nào** mang `kek_id` của khoá cũ.
4. Gỡ khoá cũ, triển khai.

Gỡ khoá cũ trước bước 3 → `ErrUnknownKey`, bí mật của các xã chưa bọc lại **không đọc được nữa**
— cùng hậu quả với mất khoá (`core/crypto/keyring.go:24-27`).

### Sao lưu KEK là CỔNG PHÁT HÀNH, không phải việc vặt

Biến tuỳ chọn, nhưng sao lưu thì không. Sao lưu KEK **trước khi ghi bí mật đầu tiên**, và **tách
khỏi** bản sao lưu CSDL: một bản sao lưu chứa cả KEK lẫn DEK đã bọc là bản chứa dạng rõ. Mất hết
KEK là mất vĩnh viễn mọi bí mật đã lưu của mọi xã; gói này **cố ý không có đường khôi phục** — một
đường khôi phục là một lối vào thứ hai (`core/crypto/keyring.go:18-22`).

### Vì sao ngược kho yêu cầu

Kho yêu cầu lưu mật khẩu SMTP **dạng rõ**: cột `password VARCHAR(512)`
(`../vigov-require/docs/spec/03-mo-hinh-du-lieu.md:409`), gán thẳng
(`../vigov-require/apps/api/app/modules/admin/email_config.py:104-105`). v2 **cố ý khác** — chính là
quyết định của ADR này.

Hai điều v2 **lấy theo** kho yêu cầu (`../vigov-require/docs/spec/07-viec-nen-va-thong-bao.md:96-100`):
mật khẩu **chỉ ghi, không đọc ngược ra**; đổi máy chủ, cổng hay tài khoản mà không nhập mật khẩu mới
thì **từ chối** — mật khẩu cũ gần như chắc chắn không dùng được với máy chủ mới, và lỗi ấy chỉ lộ ra
lúc gửi thư thật.

### Còn nợ — chưa ai quyết

| # | Việc | Của ai |
|---|---|---|
| 1 | **Chu kỳ xoay KEK** (luật 8 bất biến 6 đòi có tuổi thọ) | Chủ dự án |
| 2 | **Runbook khi nghi lộ khoá** — vẫn nợ như §*Hệ quả* đã ghi | Chủ dự án / vận hành |
| 3 | **Ai giữ bản sao lưu KEK**, ở đâu | Chủ dự án |
| 4 | Ghi `CreateDEK` / `ReplaceDEK` có cần một mục vết không, dưới chủ thể hệ thống nào — gói không ghi vết, để service sở hữu quyết (`core/crypto/envelope.go:163-164`; luật 6 điều kiện dừng #1) | Người dựng `comms` / chủ dự án |

→ Luật 8: `.claude/rules/critical/8-secrets-config.md`
→ Luật 1: `.claude/rules/critical/1-tenant-isolation.md`
