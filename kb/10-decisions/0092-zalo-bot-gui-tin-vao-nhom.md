---
id: 0092-zalo-bot-gui-tin-vao-nhom
tier: T1
source: CURATED
owner: architecture
derived_from_commit: ee8dff33
expires: null
owns_facts:
  - "Zalo Bot gửi tin vào NHÓM Zalo đã thêm bot là yêu cầu MỚI của chủ dự án 11/10/2026 — ../vigov-require không có gì về nhóm"
  - "một xã gắn được NHIỀU nhóm Zalo; phạm vi mỗi nhóm = một bộ phận (bo_phan) HOẶC cả xã (để trống); tin nhóm đi qua bot xã đang dùng — bot riêng nếu có, không thì bot chung (chủ dự án chốt 11/10/2026)"
  - "loại tin vào nhóm cấu hình theo từng nhóm, là tập con các loại xã đang bật ở Kênh Zalo; loại mang tính cá nhân (nhắc tên, đề nghị lùi hạn, bản tin sắp đến hạn/bản tin tuần theo người) không bao giờ vào nhóm (chủ dự án chốt 11/10/2026)"
  - "gắn nhóm bằng MÃ GẮN NHÓM tạo ở web-admin Cấu hình → Kênh Zalo, quyền admin.lookup, không thêm khoá quyen; gõ mã vào nhóm, chấp nhận dạng '@Bot MÃ' (chủ dự án chốt 11/10/2026)"
  - "mã ghép nối CÁ NHÂN gõ trong nhóm bị TỪ CHỐI, bot nhắn hướng dẫn nhắn riêng, mã không bị tiêu (chủ dự án chốt 11/10/2026)"
  - "nội dung tin nhóm như tin riêng (tiêu đề + thân + liên kết) — chủ dự án nới luật 3 cho nhóm có thể có người ngoài cán bộ, nhận rủi ro; một việc = một tin nhóm, gộp theo idempotency_key (chủ dự án chốt 11/10/2026)"
  - "việc thuộc diện hạn chế (phản ánh lĩnh vực can-bo, ADR 0030) KHÔNG vào nhóm — ràng buộc bảo mật do phiên chính giữ, chủ dự án có thể đổi (11/10/2026)"
  - "phạm vi bộ phận của nhóm xét theo BỘ PHẬN CỦA VIỆC (nhiệm vụ, phản ánh, văn bản đến, đơn thư), phía sinh thông báo gửi kèm qua trường proto tuỳ chọn; giải ngân không có bộ phận nên chỉ vào nhóm cả xã (chủ dự án chốt 11/10/2026)"
  - "gỡ nhóm CHỈ từ web-admin, có vết; /dung gõ trong nhóm bị bỏ qua kèm hướng dẫn; nhóm đã gắn xã A gõ mã xã B bị từ chối tới khi xã A gỡ (chủ dự án chốt 11/10/2026)"
  - "chat_id của nhóm Zalo coi như dữ liệu cá nhân, như chat_id riêng ở ADR 0074 (11/10/2026)"
---

# 0092. Zalo Bot — gửi tin vào nhóm Zalo

**Trạng thái:** đã chốt · **Ngày:** 2026-10-11 · **Người quyết:** chủ dự án, 11/10/2026, trong phiên
chính · **Nguồn yêu cầu:** **không có** — `../vigov-require/docs` không nhắc tới nhóm Zalo (`chat_type`,
"nhóm Zalo" đều không có, kiểm 11/10/2026). Đây là **yêu cầu mới** của chủ dự án. · **Sửa** ADR 0074
(thêm kênh nhóm bên cạnh tin riêng; vá cách xử lý mã ghép gõ trong nhóm).

## Bối cảnh

ADR 0074 và 0079 chỉ dựng tin **riêng**: một cán bộ ghép một `chat_id` bằng mã cá nhân. Nhiều xã làm
việc trong nhóm Zalo của bộ phận hoặc của cả xã, và muốn bot báo việc vào đó.

Theo tài liệu Zalo Bot: `chat.chat_type` là `PRIVATE` hoặc `GROUP` (Beta); `from` có `id`,
`display_name`, `is_bot`. **Chưa rõ** bot trong nhóm nhận mọi tin hay chỉ tin @nhắc bot — phải kiểm thật
sau thẻ A. Vì thế mã gắn nhóm chấp nhận cả dạng `@Bot MÃ`.

**Lỗ đang có:** bộ đọc webhook không xét `chat_type`. Một cán bộ gõ mã ghép cá nhân trong nhóm thì
`chat_id` của **cả nhóm** được ghép vào tài khoản người ấy — tin riêng của một người đi tới mọi thành
viên nhóm.

## Phương án

| Phương án | Được | Mất |
|---|---|---|
| Một nhóm cho cả xã | Đơn giản | Xã chia việc theo bộ phận thì nhóm nhận việc không phải của mình |
| **Nhiều nhóm, phạm vi = bộ phận hoặc cả xã** (chọn) | Mở rộng theo bộ phận, vẫn chứa được mô hình một nhóm cả xã | Bên phát phải gửi kèm bộ phận của việc |
| Gắn nhóm bằng mã cá nhân của cán bộ | Không thêm khái niệm | Chính là lỗ đang có: nhóm mang danh một người |

## Quyết định

| # | Điểm | Chốt |
|---|---|---|
| 1 | Mô hình | Một xã gắn được **nhiều nhóm**. Mỗi nhóm có phạm vi = **một bộ phận** (`bo_phan`) hoặc **cả xã** (trống). Chủ dự án: *"hướng mô hình theo bộ phận để mở rộng, nhưng vẫn đáp ứng mô hình một nhóm cả xã"*. Tin đi qua bot xã đang dùng — bot riêng (ADR 0079) nếu có, không thì bot chung (ADR 0074) |
| 2 | Loại tin | **Cấu hình theo từng nhóm**: tập con các loại xã đang bật ở Kênh Zalo. Loại mang tính **cá nhân** — nhắc tên một người, đề nghị lùi hạn, bản tin sắp đến hạn / bản tin tuần theo người — **không bao giờ** vào nhóm |
| 3 | Gắn nhóm | **Mã gắn nhóm** tạo ở web-admin Cấu hình → Kênh Zalo, quyền `admin.lookup` như ADR 0074 #6 — **không thêm khoá `quyen`**. Gõ mã vào nhóm; chấp nhận dạng `@Bot MÃ` |
| 4 | Mã cá nhân trong nhóm | **Từ chối**; bot nhắn hướng dẫn nhắn riêng cho bot; mã **không bị tiêu**. Vá lỗ ở §Bối cảnh |
| 5 | Nội dung | **Như tin riêng** (tiêu đề + thân + liên kết). Chủ dự án **nới luật 3** cho nhóm — nhóm có thể có người ngoài cán bộ — và nhận rủi ro. Một việc = **một tin nhóm**, gộp theo `idempotency_key` dù việc có nhiều người nhận |
| 6 | Ràng buộc bảo mật | Việc thuộc **diện hạn chế** — phản ánh lĩnh vực `can-bo` (ADR 0030) — **không vào nhóm**. Phiên chính giữ giới hạn này khi chủ dự án nới #5; chủ dự án có thể đổi |
| 7 | Xét bộ phận | Theo **bộ phận của việc** (nhiệm vụ, phản ánh, văn bản đến, đơn thư), không theo bộ phận người nhận. Bên sinh thông báo gửi kèm qua **trường proto tuỳ chọn** (luật 2 cấm #4). **Giải ngân** không có bộ phận → chỉ vào nhóm cả xã |
| 8 | Gỡ nhóm | **Chỉ từ web-admin**, có vết (luật 6). `/dung` gõ trong nhóm bị bỏ qua, bot nhắn hướng dẫn |
| 9 | Nhóm đã thuộc xã khác | Nhóm đã gắn xã A gõ mã xã B → **từ chối** tới khi xã A gỡ. Không chuyển nhóm im lặng giữa hai xã (luật 1) |

**Vì sao #7 xét bộ phận của việc:** nhóm bộ phận là nơi bộ phận ấy theo dõi việc **của mình**. Xét theo
người nhận thì một việc của bộ phận A giao cho người thuộc bộ phận B sẽ vào nhóm B — nhóm thấy việc
không phải của mình, còn nhóm A không thấy.

**Vì sao #9 từ chối chứ không chuyển:** gắn lại sang xã B mà không cần xã A đồng ý là để một xã lấy kênh
của xã khác bằng một mã. Xã A gỡ trước, có vết, rồi xã B mới gắn.

## Hệ quả

- Giờ yên tĩnh của tin nhóm lấy từ **cấu hình của xã** (ADR 0074 §Luật nghiệp vụ), không có giờ riêng
  cho nhóm.
- Hạn mức Zalo vẫn **chưa biết** (ADR 0074 §Hệ quả); tin nhóm làm **tăng lưu lượng** — một tin cho mỗi
  nhóm khớp, cộng với tin riêng.
- `chat_id` của nhóm coi như **dữ liệu cá nhân** như `chat_id` riêng: không log, không trả ra giao diện,
  không xuất.
- Bên phát (petitions, tasks, documents, …) phải gửi kèm bộ phận của việc; chưa gửi thì việc ấy chỉ vào
  nhóm cả xã.
- Nhóm có người ngoài cán bộ nhận tiêu đề, thân và liên kết của việc — đó là rủi ro chủ dự án nhận ở #5.

## Còn mở

| # | Việc | Ai |
|---|---|---|
| 1 | Bot trong nhóm nhận mọi tin hay chỉ tin @nhắc bot — kiểm thật sau thẻ A | phiên làm thẻ A |
| 2 | Bổ sung `kb/30-indexes/transaction-boundaries.json` (qua nguồn sinh, `make kb`) cho **gắn nhóm** và **gửi nhóm** | contract-designer |
| 3 | Hạn mức gửi của Zalo khi bật tin nhóm cho nhiều xã | hỏi Zalo |
