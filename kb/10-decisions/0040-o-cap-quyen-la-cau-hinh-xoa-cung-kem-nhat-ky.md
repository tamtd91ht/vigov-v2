---
id: 0040-o-cap-quyen-la-cau-hinh-xoa-cung-kem-nhat-ky
tier: T1
source: CURATED
owner: architecture
derived_from_commit: 83d3be1
expires: null
owns_facts:
  - "ô cấp quyền `vai_tro_quyen` là cấu hình phân quyền, không phải hồ sơ lưu trữ nghiệp vụ"
  - "vì sao ô bị bỏ khi lưu cột vai trò được xoá cứng, cùng giao dịch với một mục nhật ký giữ đủ tập trước/sau"
  - "giới hạn: ngoại lệ luật 7 này chỉ áp cho `vai_tro_quyen`, không cho bảng nào khác"
---

# 0040. Ô cấp quyền là cấu hình — bỏ ô thì xoá cứng, nhật ký giữ bằng chứng

**Trạng thái:** đã chốt · **Ngày:** 2026-09-24 · **Người dùng chốt** (lượt `/develop-feature Cấu hình`)

## Bối cảnh

Tuyến `PUT /api/v1/roles/{id}/permissions` (khoá `admin.role`, seed ở
`service-identity/migrations/0001_init.sql:283`) lưu **cả cột** của một vai trò trên ma trận
Phân quyền theo kiểu thay-bằng-tập: gửi tập khoá mới, máy chủ tính phần thêm và phần bỏ. Chỗ tuyến
này còn vắng được giải thích ở `service-identity/internal/http/quyen.go:13-35`.

Phần **bỏ** chạm luật 7: cấm #1 (câu xoá cứng trên dữ liệu nghiệp vụ) và điều kiện dừng #1 ("xoá
thật một hồ sơ nghiệp vụ"). Builder dừng lại và hỏi. Câu hỏi thật là: **một ô cấp quyền có phải hồ
sơ lưu trữ không?**

Bảng `vai_tro_quyen` (`service-identity/migrations/0001_init.sql:142-150`) đã ngầm trả lời:

| Dấu hiệu trong schema | Hệ quả |
|---|---|
| Chỉ có `tenant_id, vai_tro_id, quyen_ma, cap_luc, cap_boi` — **không có** `deleted_at / deleted_by / delete_reason` | Bảng chưa bao giờ được thiết kế để xoá mềm |
| Khoá chính `(tenant_id, vai_tro_id, quyen_ma)` | Xoá mềm rồi cấp lại cùng ô sẽ **đụng khoá chính** — phải hồi sinh dòng cũ hoặc đổi khoá |

Một ô cấp quyền không phải văn bản, phiếu hay quyết định có thời hạn lưu trữ luật định. Nó là
**trạng thái hiện hành** của cấu hình phân quyền. Câu hỏi pháp lý có thể đến sau — "ai đã cho vai
trò X quyền Y, từ bao giờ, đến bao giờ" — được trả lời bằng **nhật ký kiểm toán** (luật 6: chỉ ghi
thêm, không sửa, không xoá; ADR 0013 cưỡng chế ở tầng CSDL), không bằng bảng sống.

## Các phương án

| Phương án | Được | Mất |
|---|---|---|
| **A (chọn)** — xoá cứng các ô bị bỏ, cùng giao dịch với một mục nhật ký giữ **đủ** tập khoá trước, tập sau, phần thêm, phần bỏ | Không migration. Không đường đọc quyền nào phải sửa. Khoá chính giữ nguyên nghĩa | Bảng sống không còn lịch sử. Muốn biết ai giữ gì lúc nào phải đọc nhật ký |
| B — thêm cột xoá mềm vào `vai_tro_quyen` | Lịch sử nằm ngay trong bảng | Migration trên bảng đang chạy. **Mọi** đường đọc quyền phải thêm `deleted_at IS NULL`: `truyVanQuyenGoc` (`service-identity/internal/store/checker.go:40-46`, chạy trên **mỗi** request của mọi service), `truyVanQuanTriDeGhi` (`service-identity/internal/store/can_bo_ghi.go:305-313`), `QuyenCuaVaiTro` (`can_bo_ghi.go:367-368`), `truyVanCapQuyen` (`service-identity/internal/store/quyen.go:163-172`). Quên một chỗ là **nới quyền im lặng**. Khoá chính phải thành khoá một phần, hoặc cấp lại phải hồi sinh dòng |
| C — bảng lịch sử chỉ ghi thêm riêng | Lịch sử có cấu trúc | Chép lại đúng điều mục nhật ký đã giữ — hai nguồn cho một sự thật (luật 9) |

Phương án B nguy hiểm theo đúng hướng luật 5 sợ nhất: đường đọc quyền thiếu một điều kiện thì
**cấp thêm** quyền, không báo lỗi, test vẫn xanh.

## Quyết định

Ô cấp quyền trong `vai_tro_quyen` là **cấu hình phân quyền**. Khi lưu cột vai trò, các ô bị bỏ được
**xoá cứng** — lọc theo `tenant_id`, `vai_tro_id` và `quyen_ma = ANY($3)` — **trong cùng giao
dịch** với một mục nhật ký giữ đủ tập khoá trước/sau cùng phần thêm/bỏ.

Điều kiện đi kèm, bỏ một điều là quyết định không còn đứng:

| # | Điều kiện | Vì sao |
|---|---|---|
| 1 | Câu xoá luôn lọc theo `tenant_id` **và** `vai_tro_id` **và** tập khoá cụ thể | Luật 7 cấm #2 (không lọc) vẫn áp nguyên. Luật 1 |
| 2 | Mục nhật ký ghi trong **cùng giao dịch**. Nhật ký hỏng thì xoá cũng không xảy ra | Luật 6 bất biến 3 — sau khi xoá, nhật ký là bằng chứng **duy nhất** còn lại |
| 3 | Nhật ký giữ **tập đầy đủ**, không chỉ phần chênh | Dựng lại "vai trò X giữ gì tại thời điểm T" không phải cộng dồn chuỗi chênh lệch. Một mục mất không làm sai mọi mục sau |
| 4 | Khoá quyền không phải dữ liệu cá nhân, nên ghi nguyên văn, không che | Luật 6 bất biến 5 chỉ đòi che trường dữ liệu cá nhân |

## Giới hạn — không phải tiền lệ

Quyết định này **chỉ** áp cho ô cấp quyền `vai_tro_quyen`. Nó **không** nới luật 7 cho bảng nào
khác: bản thân vai trò `vai_tro`, cán bộ, danh mục, và mọi hồ sơ nghiệp vụ vẫn xoá mềm. Xoá mềm một
vai trò vẫn để nguyên các dòng cấp quyền của nó (đường đọc lọc qua `vt.deleted_at`,
`service-identity/internal/store/quyen.go:156-159`) — ADR này không đổi điều đó.

Ai muốn viện dẫn ADR này cho bảng khác phải chứng minh đủ cả ba: bảng không có cột xoá mềm và
không được thiết kế cho nó, dòng là trạng thái cấu hình chứ không phải hồ sơ, và nhật ký cùng giao
dịch giữ đủ để dựng lại. Thiếu một điều thì đó là điều kiện dừng #1 của luật 7, hỏi người dùng.

Cùng lượt, người dùng chốt thêm hai điều **không** thuộc ADR này (chỉ ghi để đọc có bối cảnh; nguồn
sở hữu là `kb/00-foundation/open-questions.json`): câu #14 — cấm tự lưu cột của vai trò mình đang
giữ; câu #13 — mở rộng chặn "người giữ cuối cùng" sang khoá `admin.role`.

## Hệ quả

- **Dễ:** tuyến lưu cột không cần migration. Bộ kiểm quyền trên mọi request giữ nguyên câu truy vấn.
- **Khó hơn:** câu "ai giữ quyền gì lúc nào" chỉ trả lời được từ nhật ký. Nếu sau này cần báo cáo
  lịch sử phân quyền, nó đọc nhật ký — không được thêm cột xoá mềm để "tiện hơn" mà không có ADR mới.
- **Còn nợ:** `.claude/hooks/data_safety_guard.py:139-147` chặn câu xoá cứng theo **từ ngữ lân cận**
  (danh sách `BUSINESS` ở `:83-85` và đường dẫn tệp), nên phán quyết của hook trên câu xoá này phụ
  thuộc vào tên tệp và chữ đứng gần, không phụ thuộc vào quyết định này. Một ca kiểm của hook làm
  phán quyết đó **có chủ đích** là việc của người sở hữu `.claude/`.
