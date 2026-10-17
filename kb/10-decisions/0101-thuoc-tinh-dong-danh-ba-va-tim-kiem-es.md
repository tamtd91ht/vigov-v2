---
id: 0101-thuoc-tinh-dong-danh-ba-va-tim-kiem-es
tier: T1
source: CURATED
owner: architecture
derived_from_commit: 295c2fdc
expires: null
owns_facts:
  - "danh bạ người dân có THUỘC TÍNH ĐỘNG theo từng xã (citizen_field): mã 6–8 ký tự a-z0-9 bất biến, tên, loại singleText · multiText · combobox (chọn NHIỀU) · singleBox (chọn MỘT) · number (nguyên/thập phân) · dateTime (+ định dạng), placeholder, lựa chọn {mã, nhãn}, bắt buộc, hiện ở danh sách, thứ tự kéo-thả; ngừng dùng = ẩn, giữ dữ liệu (chủ dự án 17/10/2026)"
  - "khoá quyền mới admin.citizen_field 'Cấu hình thuộc tính danh bạ người dân', tab Cấu hình → Thuộc tính danh bạ; cấp sẵn cho vai trò đang có admin.zalo (chủ dự án 17/10/2026)"
  - "danh bạ thêm cột tĩnh email (chủ dự án 17/10/2026)"
  - "tìm kiếm + lọc danh bạ chạy trên Elasticsearch; PostgreSQL vẫn là nguồn đúng; đồng bộ qua outbox → Kafka <ENV>.identity.citizen_contact_changed.v1 (chỉ mã dòng, không dữ liệu cá nhân) → consumer identity đọc lại PostgreSQL và ghi ES (chủ dự án 17/10/2026)"
  - "ES KHÔNG giữ số điện thoại hay email thật: lưu mảng HMAC của mọi chuỗi con liền nhau 3–10 ký tự của số, và của 10 ký tự đầu phần trước @ của email; truy vấn băm chuỗi gõ vào rồi so khớp; email dài hơn 10 ký tự so khớp băm nguyên email; họ tên lưu nguyên văn để tìm toàn văn (chủ dự án 17/10/2026)"
  - "lọc: number và dateTime theo khoảng, chỉ cần một cận; combobox/singleBox lọc nhiều giá trị; ngày tạo lưu millis để lọc khoảng (chủ dự án 17/10/2026)"
  - "gửi tin ZNS/SMS tới đối tượng danh bạ được map tham số/biến theo thuộc tính (tĩnh và động) của từng người (chủ dự án 17/10/2026)"
---

# 0101. Thuộc tính động của danh bạ người dân, tìm kiếm trên Elasticsearch

**Trạng thái:** đã chốt · **Ngày:** 2026-10-17 · **Người quyết:** chủ dự án, 17/10/2026, phiên chính (bản yêu
cầu dán vào phiên + phiếu 4 câu + một câu bổ sung) · **Dựng trên** ADR 0097 (danh bạ người dân), 0099 (gửi
đợt tin), 0100 (Kafka chung cụm, tiền tố `ENV`), 0010 (vai trò Elasticsearch) · **Sửa** ADR 0097 D11 (tìm kiếm).

## Nguồn yêu cầu

*"sẽ có 1 màn cấu hình thuộc tính động theo từng xã ngoài các thuộc tính tĩnh mặc định đang có… data ngoài
lưu ở postgresql cần đẩy lên es để hỗ trợ search keyword (fullname, phone, email) và filter"* — và *"khi có
thuộc tính thì gửi tin zns, sms có thể map nội dung theo thuộc tính nhé"*.

## Quyết định

| # | Điểm | Chốt |
|---|---|---|
| 1 | Định nghĩa thuộc tính | Mỗi xã một bộ (`tenant_id`). `fieldCode` 6–8 ký tự `[a-z][a-z0-9]{5,7}`, duy nhất trong xã, **không đổi** sau khi tạo; loại cũng không đổi. Tên hiển thị, placeholder (ô chữ), lựa chọn `{value, label}` (combobox, singleBox; lựa chọn đã dùng chỉ được ngừng, không xoá), số `integer`/`decimal`, `dateTime` + định dạng hiển thị. Thêm (đề xuất phiên chính, chủ dự án đồng ý): bắt buộc, hiện thành cột ở danh sách, ≤ 50 thuộc tính mỗi xã |
| 2 | Loại | Đúng tên chủ dự án viết: `singleText` (một dòng), `multiText` (nhiều dòng), `combobox` (chọn **nhiều**, lưu mảng), `singleBox` (chọn **một**), `number`, `dateTime`. Lệch ADR 0011 (enum tiếng Việt) có chủ ý: chủ dự án đặt tên |
| 3 | Thứ tự | Kéo-thả trong màn cấu hình; form Thêm/Sửa hiện thuộc tính động ngay sau phần tĩnh theo thứ tự ấy |
| 4 | Ngừng dùng | Ẩn khỏi form và lọc, **giữ** giá trị đã nhập (luật 7) |
| 5 | Quyền | Khoá mới `admin.citizen_field`, tab Cấu hình → "Thuộc tính danh bạ"; migration identity gieo và cấp cho vai trò đang có `admin.zalo`. Đọc định nghĩa: `citizen.read`; nhập giá trị: `citizen.update` |
| 6 | Email | Cột tĩnh mới `email` (tuỳ chọn), che khi trả ra như số điện thoại |
| 7 | Tìm kiếm | Từ khoá + lọc chạy trên ES; ES trả **mã dòng**, identity đọc lại PostgreSQL (đã che) để trả web. Từ khoá vẫn đi trong thân yêu cầu (D11) |
| 8 | Số và email trên ES | Không lưu giá trị thật. Số 10 chữ số → mọi chuỗi con liền nhau dài 3…10 → **HMAC từng chuỗi** → một mảng (thứ tự vô nghĩa). Email: phần trước `@`, 10 ký tự đầu, cùng cách; chuỗi gõ dài hơn 10 → so khớp HMAC **nguyên email**. Khoá HMAC: **một khoá cho toàn hệ thống** (chủ dự án 17/10/2026), biến bí mật mới `SEARCH_HASH_KEY`; đổi khoá phải dựng lại chỉ mục mọi xã. Lộ chỉ mục ES mà không lộ khoá thì không lộ số |
| 9 | Họ tên | Lưu nguyên văn trên ES để tìm toàn văn, có bỏ dấu |
| 10 | Lọc | `number`, `dateTime`: khoảng, một cận là đủ (`from` hoặc `to`). `combobox`, `singleBox`: nhiều giá trị (`[ho-ngheo, can-ngheo]`). Ngày tạo lưu **millis** để lọc khoảng |
| 11 | Đồng bộ | Ghi danh bạ → dòng outbox cùng giao dịch → Kafka `<ENV>.identity.citizen_contact_changed.v1` (chỉ `tenant_id` + mã dòng + thao tác, **không** dữ liệu cá nhân) → consumer của identity (group `<ENV>.identity.citizen-search`) đọc lại PostgreSQL và ghi ES. Có lệnh dựng lại chỉ mục từng xã. ES hỏng: tìm kiếm báo lỗi, dữ liệu không mất |
| 12 | Gửi tin theo thuộc tính | Đối tượng từ danh bạ: tham số ZNS / biến SMS map được vào thuộc tính tĩnh hoặc động của từng người; identity trả giá trị cùng lúc mở số (ADR 0099 #5), comms giữ trong bộ nhớ. Thay ADR 0099 #6 "không tham số nào lấy từ hồ sơ của từng người" — chủ dự án yêu cầu |

## Hệ quả

- **Rủi ro nhận:** ES chứa họ tên nguyên văn và giá trị thuộc tính (có thể là dữ liệu cá nhân, vd. phân loại hộ
  nghèo); số và email chỉ ở dạng băm. Thuộc tính động do xã tự đặt nên phần mềm không biết thuộc tính nào nhạy
  cảm. Gửi giá trị thuộc tính sang Zalo/eSMS là gửi dữ liệu cá nhân ra ngoài theo yêu cầu chủ dự án.
- Một khoá chung nghĩa là cùng một số ở hai xã có cùng mã băm: chỉ mục ES nối được hai bản ghi của cùng một
  người giữa hai xã. Mọi truy vấn ES vẫn lọc theo `tenant_id` (luật 1) — đó là thứ duy nhất tách hai xã.
- Chuỗi con 3 chữ số chỉ có 1.000 giá trị: băm có khoá chặn đoán ngược, không chặn được thống kê tần suất
  trong một xã — chấp nhận theo thiết kế chủ dự án.
- Cùng một cụm ES cho dev và prod thì tên chỉ mục phải khác nhau theo `ELASTICSEARCH_INDEX_PREFIX` (vận hành đặt).

## Còn mở

| # | Việc | Ai |
|---|---|---|
| 1 | Nhập Excel danh bạ có cột thuộc tính động (theo mã) | lượt sau |
| 2 | Xuất Excel kèm thuộc tính động | lượt sau |

→ ADR 0097 · 0099 · 0100 · 0010 · 0009 · 0011 → Luật 1 · 2 · 3 · 5 · 6 · 7 · 13
