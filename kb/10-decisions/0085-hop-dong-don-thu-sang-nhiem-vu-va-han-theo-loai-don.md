---
id: 0085-hop-dong-don-thu-sang-nhiem-vu-va-han-theo-loai-don
tier: T1
source: CURATED
owner: architecture
derived_from_commit: 4eb7ef27
expires: null
owns_facts:
  - "chuyển đơn thư thành nhiệm vụ: tuyến trình duyệt ở service-petitions, petitions HỎI documents (ResolveCitizenLetterForTask) rồi ghi nhiệm vụ trong MỘT giao dịch; documents không ghi gì; đơn tố cáo bị chặn ở documents và không trả về gì (đề xuất 08/10/2026)"
  - "nguồn giao nhiệm vụ thứ năm `don-thu` (nguon_id = citizen_letter.id), hạn nhiệm vụ = ngày (giờ Việt Nam) của hạn hiện tại của đơn lúc 17:00, đơn không hạn → nhiệm vụ không hạn (đề xuất 08/10/2026)"
  - "hạn đơn thư theo loại đơn đi qua RPC riêng IdentityService.ResolveCitizenLetterDeadline, không qua ResolveDeadlines; 'xã chưa cấu hình' là câu trả lời OK riêng, mọi lỗi là từ chối hành vi (đề xuất 08/10/2026)"
---

# 0085. Hợp đồng: đơn thư → nhiệm vụ, và hạn đơn thư theo loại đơn

**Trạng thái:** đề xuất của contract-designer, hợp đồng `.proto` đã viết (chưa có cài đặt) · **Ngày:**
2026-10-08 · **Chờ chủ dự án chốt** các câu ở §Còn mở trước khi dựng hành vi · **Thực thi** ADR 0084 #3,
#6 và ADR 0039 hệ quả #2 · **Đổi** chỗ tính mà ADR 0064 §"Tính thế nào" đề xuất (ResolveDeadlines).

## Bối cảnh

| Điều | Nguồn |
|---|---|
| Đơn thư thuộc `documents`, nhiệm vụ thuộc `petitions`; "chuyển thành nhiệm vụ" là xuyên service, chưa có hợp đồng | ADR 0039 hệ quả #2 |
| Nguồn và hạn suy ra **ở máy chủ**, hạn = ngày hạn của đơn lúc 17:00; đơn **tố cáo** không chuyển | C9 (24/09), ADR 0084 #6 |
| `POST /api/v1/tasks` nhận `source_id` + hạn từ client, không kiểm | `service-petitions/internal/http/nhiem_vu_ghi.go:519-534` |
| Hạn đơn thư tự điền lúc vào sổ theo loại đơn; chưa cấu hình → "Không đặt hạn" | ADR 0084 #3 |
| `ResolveDeadlines` trả FAILED_PRECONDITION khi xã chưa cấu hình, và bên gọi **từ chối tiếp nhận** | `identity.proto` (chú thích RPC), `transaction-boundaries.json` `tinh_han_xu_ly_luc_tiep_nhan` |
| Tố cáo: hạn xử lý đơn đếm **ngày làm việc**, hạn giải quyết đếm **ngày lịch** — hai đơn vị trên một loại đơn | ADR 0064 bảng 30/09 |

## Quyết định (đề xuất)

### A. Chuyển đơn thư thành nhiệm vụ

| # | Điểm | Đề xuất |
|---|---|---|
| A1 | Ai mở tuyến | `petitions` — nó ghi bản ghi duy nhất (nhiệm vụ). Tiền tố `citizen-letters` thuộc `documents` nên không lồng được; danh từ đề xuất `POST /api/v1/citizen-letter-tasks` (thân: `letter_id`, tiêu đề, bộ phận/cán bộ tuỳ chọn, mức ưu tiên…) — **danh từ chờ chủ dự án** |
| A2 | Chiều gọi | `petitions → documents.ResolveCitizenLetterForTask` (đọc), trước khi mở giao dịch |
| A3 | Nhất quán | **Mạnh**, một giao dịch của `petitions`; không có bù trừ vì `documents` không ghi gì |
| A4 | Tố cáo | `documents` trả `DENUNCIATION` và **không** trả số, hạn, người giữ, trích yếu; `petitions` trả 422 |
| A5 | Nguồn | Giá trị thứ năm `don-thu` của `nguon_giao` (giá trị tiếng Việt, ADR 0011); hằng Go `SourceCitizenLetter` |
| A6 | Hạn | Ngày (giờ Việt Nam) của `current_due_at` lúc 17:00; đơn không hạn → nhiệm vụ không hạn. Client không gửi hạn |
| A7 | Tiêu đề | **Màn hình** điền sẵn từ trích yếu (như prototype), cán bộ xác nhận; máy chủ **không** chép trích yếu |
| A8 | Quyền | `task.create` **và** `petition.read` — cả hai đã có trong `quyen` (`service-identity/migrations/0001_init.sql:303, 308`) |
| A9 | Luỹ đẳng | `Idempotency-Key` bắt buộc; một đơn được tạo **nhiều** nhiệm vụ như prototype |

**Vì sao petitions hỏi, không phải documents ra lệnh.** Chiều ngược lại (`documents` gọi một RPC "tạo
nhiệm vụ") bắt `petitions` ghi theo mã đơn, hạn **và** mã cán bộ do bên gọi tự khai — khoá gọi dùng
chung chỉ chứng minh "trong triển khai", không chứng minh "service nào" (ADR 0025). Tiền lệ duy nhất
tin mã người thực hiện qua gRPC là ADR 0074 #3, do chủ dự án chốt riêng cho vận hành nền tảng. Chiều
ấy còn đòi `documents` ghi nhật ký đơn sau khi `petitions` đã ghi: hai giao dịch, phải bù trừ. Liên kết
đơn ↔ nhiệm vụ đã nằm ở cặp `nguon_giao`/`nguon_id` — một sự thật, một chỗ.

**Vì sao chặn tố cáo ở documents.** Chỉ `documents` biết loại đơn. Để `petitions` lọc thì câu trả lời
phải mang loại đơn và thông tin của đơn tố cáo đi qua ranh giới trước khi bị bỏ.

### B. Hạn đơn thư theo loại đơn

| # | Điểm | Đề xuất |
|---|---|---|
| B1 | Lối vào | RPC mới `ResolveCitizenLetterDeadline(letter_type, kind, count_from)` |
| B2 | Hai hạn | `PROCESSING` (vào sổ, từ 00:00 giờ VN của ngày nhận) · `RESOLUTION` (thụ lý, từ `accepted_at`) |
| B3 | Chưa cấu hình | OK + `not_configured` → đơn **không hạn**. Mọi lỗi (kể cả FAILED_PRECONDITION) → **từ chối hành vi** |
| B4 | Đơn vị | Nằm ở **quy tắc của xã** trong identity, không trên dây. Ba giá trị: giờ làm việc · ngày làm việc · ngày lịch |
| B5 | Dòng `don-thu` mặc định | Không đọc, không lùi về. Nó giữ ngưỡng nhắc việc (ADR 0079 Q18) |

**Vì sao không dùng ResolveDeadlines như ADR 0064 đề xuất.** Hai sự thật đến sau ADR 0064:

1. ADR 0084 #3 cho "chưa cấu hình" nghĩa **ngược** với phản ánh: đơn vẫn vào sổ, không hạn. Cùng một
   mã lỗi mà nghĩa tuỳ loại việc là chỗ bên gọi đọc nhầm. Giữ FAILED_PRECONDITION thì "chưa cấu hình"
   lẫn với "cấu hình hỏng / lịch hỏng" — mà ghi không hạn khi hỏng là thổi phồng tỷ lệ đúng hạn
   (ADR 0084 #4 tính đơn không hạn là đúng hạn).
2. Hai hạn của đơn đếm từ **hai mốc** và có thể **hai đơn vị** (tố cáo). `DeadlineKind` của phản ánh
   đếm hai hạn từ **một** mốc và đặt tên theo cột `han_tiep_nhan`/`han_xu_ly_xong`.

Phép tính vẫn **một chỗ** trong identity (ADR 0064 #4): RPC mới và ResolveDeadlines dùng chung phép
tiến giờ làm việc.

## Còn mở — hỏi chủ dự án, không tự chọn

| # | Câu | Chặn việc nào |
|---|---|---|
| 1 | Danh từ URL `citizen-letter-tasks` (A1) | TASK-08 (tuyến) |
| 2 | Nơi lưu quy tắc theo loại đơn: **bảng riêng** trong identity (đề xuất, xem dưới) hay thêm cột vào `sla` (ADR 0064 đề xuất). ADR 0084 §Hệ quả đã để câu này cho cổng | TASK-05 |
| 3 | Đơn vị của `kien-nghi-phan-anh`, `de-nghi`: **giờ làm việc** (ADR 0084 §3 "theo ADR 0007") hay **ngày làm việc** (C8 24/09 "ngày làm việc theo lịch xã"). Hai nguồn nói khác nhau | TASK-05 |
| 4 | ADR 0064 #3, #4 (ngày cuối rơi vào ngày nghỉ; ngày đầu có tính không; hạn hết lúc mấy giờ ngày cuối) — **vẫn mở** | Phép tính ngày lịch / ngày làm việc trong identity |
| 5 | Gieo sẵn số luật định của ADR 0064 (10 / 30 / 7 / 30) cho xã, hay để trống tới khi xã nhập | TASK-05 |
| 6 | Hộp thoại bỏ trống cả bộ phận lẫn cán bộ và đơn cũng không có người giữ: từ chối (prototype `assignment_required`) hay tạo nhiệm vụ "Chưa xác định" như cửa giao trực tiếp | TASK-08 |

**Bảng riêng (câu 2) — vì sao đề xuất:** cột `gio_*` của `sla` là **giờ** và NOT NULL năm ngưỡng nhắc
việc không áp cho đơn; hai lối vào đọc chung một dòng với hai nghĩa là cái bẫy. Bảng riêng
`citizen_letter_deadline_rule` một dòng = (loại đơn, hạn, số, đơn vị), "chưa cấu hình" = không có dòng.

## Hệ quả

- `documents` phải lên **trước** `petitions` (A) và `identity` **trước** `documents` (B): RPC cũ trả
  UNIMPLEMENTED và bên gọi từ chối, không đoán.
- Vào sổ đơn thư nay phụ thuộc `identity` sống: identity sập thì không vào sổ được (đúng chiều fail closed).
- `POST /api/v1/tasks` phải từ chối `source = don-thu` như đã từ chối `phan-anh`, `ket-luan-hop`.
- Còn nguyên: `source = van-ban-den` ở `POST /api/v1/tasks` vẫn tin `source_id` từ client (câu U11).

→ Hợp đồng: `proto/vigov/documents/v1/documents.proto` (`ResolveCitizenLetterForTask`) ·
`proto/vigov/identity/v1/identity.proto` (`ResolveCitizenLetterDeadline`)
→ Ranh giới giao dịch: `kb/30-indexes/transaction-boundaries.json` → `tao_nhiem_vu_tu_don_thu`,
`tinh_han_don_thu_theo_loai_don`
→ ADR 0084 (#3, #4, #6) · ADR 0064 (đơn vị, chỗ tính) · ADR 0039 (hệ quả #2) · ADR 0025 (khoá gọi chung)
