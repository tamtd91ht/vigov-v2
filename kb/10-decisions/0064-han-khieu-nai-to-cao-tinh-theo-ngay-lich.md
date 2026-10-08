---
id: 0064-han-khieu-nai-to-cao-tinh-theo-ngay-lich
tier: T1
source: CURATED
owner: domain
derived_from_commit: b3cc9582
expires: null
owns_facts:
  - "hạn luật định của đơn khiếu nại và đơn tố cáo đếm theo NGÀY LỊCH, không theo giờ làm việc — ngoại lệ tường minh của ADR 0007 (người dùng chốt 30/09/2026)"
  - "phép tính hạn theo ngày lịch cũng thuộc service-identity, như phép tính giờ làm việc — không service nào khác tự cộng ngày"
  - "hai loại đơn thư còn lại (kien-nghi-phan-anh, de-nghi) giữ cách đếm theo SLA của xã"
---

# 0064. Hạn khiếu nại và tố cáo tính theo ngày lịch

**Trạng thái:** đã chốt hướng · **Ngày:** 2026-09-30 · **Người dùng chốt** (phiên chính,
30/09/2026) · **Ngoại lệ tường minh** của ADR 0007 (hạn đếm bằng giờ làm việc) — thân ADR 0007 giữ
nguyên · **Thay** phần đơn vị của câu C8 ngày 24/09/2026 (`kb/90-ephemeral/tien-do/service-documents.json`
mục `so-don-thu-cong-dan`), **chỉ** cho hai loại đơn này · **Chưa dựng gì**

## Bối cảnh

| Điều | Nguồn |
|---|---|
| Mọi hạn trong hệ đếm bằng **giờ làm việc** theo lịch từng xã | ADR 0007 |
| Sổ đơn thư có **bốn loại**: `kien-nghi-phan-anh` · `khieu-nai` · `to-cao` · `de-nghi` (câu C4, 24/09) | sổ tiến độ `so-don-thu-cong-dan` |
| **Hai hạn**, cả hai lưu: hạn xử lý đơn chốt lúc vào sổ, hạn giải quyết chốt lúc thụ lý (câu C8, 24/09) | cùng mục |
| C8 ngày 24/09 chọn *ngày làm việc theo lịch xã* và ghi rõ: **hỏi pháp chế ngày làm việc hay ngày lịch cho "KN Đ.28, TC Đ.29" trước khi gieo số** | cùng mục |
| `sla.loai_viec` hôm nay chỉ nhận `van-ban-den`, `phan-anh`, `nhiem-vu`; con số là **giờ** (`gio_tiep_nhan`, `gio_xu_ly_xong`) | `service-identity/migrations/0008_sla.sql:223`, `:244` |
| Sổ đơn thư thuộc `service-documents` | ADR 0039 |

Người dùng trả lời câu hỏi pháp chế ngày 30/09: luật viết **"ngày"**, không viết **"ngày làm việc"**,
nên hạn khiếu nại và tố cáo đếm theo **ngày lịch**.

## Quyết định

| # | Quyết định |
|---|---|
| 1 | Với đơn loại `khieu-nai` và `to-cao`, **hạn nào mà văn bản luật đếm bằng "ngày"** thì hệ đếm bằng **ngày lịch** — gồm cả cuối tuần và ngày nghỉ lễ |
| 2 | Hạn nào mà văn bản luật viết **"ngày làm việc"** thì đếm ngày làm việc theo lịch xã. Quyết định là **đọc đúng chữ của luật**, không phải "mọi hạn của hai loại này là ngày lịch" |
| 3 | `kien-nghi-phan-anh` và `de-nghi` **không đổi**: vẫn theo câu C8 ngày 24/09 — số theo SLA từng xã, đếm theo lịch làm việc của xã (ADR 0007) |
| 4 | Phép tính theo ngày lịch **thuộc `service-identity`**, cùng chỗ với giờ làm việc. `service-documents` không tự cộng ngày (luật 10 cấm #2, `hooks/citizen_commitment_guard.py`) |
| 5 | Hạn vẫn tính **một lần**, tại hành vi ấn định nó (vào sổ, thụ lý), và **lưu** (luật 10 bất biến 2, ADR 0028). Quá hạn vẫn **suy ra** (luật 10 bất biến 3) |

### Vì sao ngoại lệ — và vì sao chiều sai của phương án kia nặng hơn

ADR 0007 chọn giờ làm việc vì cam kết của **xã** với dân nhỏ hơn một ngày (2 giờ cho an ninh trật
tự). Hạn khiếu nại, tố cáo không phải cam kết xã tự đặt: đó là **hạn luật định**. Đếm nó bằng ngày
làm việc làm hạn **dài ra** gần gấp rưỡi (mỗi tuần mất hai ngày cuối tuần, cộng ngày lễ). Hệ sẽ hiện
*"đúng hạn"* cho một đơn mà cơ quan **đã quá hạn luật định** — con số lên lãnh đạo sai về đúng chiều
có hại, và người phát hiện đầu tiên là người khiếu nại.

## Tính thế nào — đề xuất, chưa dựng

Không dòng nào dưới đây được dựng trước cổng của thẻ việc (ROUTING §0.3). Đổi cách tính hạn là
**luật 10 điều kiện dừng #1**: người dùng đã chốt **đơn vị**; hình dạng hợp đồng vẫn qua cổng.

| Điều | Đề xuất |
|---|---|
| Chỗ tính | `identity.ResolveDeadlines` (`proto/vigov/identity/v1/identity.proto:1183`) — cùng lối vào mà `documents` đã phải gọi cho hạn văn bản đến. Không thêm vào `AdvanceWorkingHours` (ADR 0029 §119: tiền lệ hình dạng, không phải chỗ thêm trường) |
| Đơn vị nằm ở đâu | Ở **dòng SLA**, như C8 đã đòi *"mỗi dòng SLA ghi rõ đơn vị"*: một cột đơn vị cộng thêm vào `sla`, và `loai_viec` nhận thêm loại đơn thư. Thêm cột và nới CHECK là migration của identity; thêm giá trị enum ở proto là thay đổi cộng thêm (luật 2 bất biến 7) |
| Trả gì | Vẫn **thời điểm** đã tính xong, không trả số ngày cho bên gọi tự cộng — lý do ở chú thích của `ResolveDeadlines` (`identity.proto:1061-1073`) |
| Múi giờ | Ngày lịch vẫn là ngày **theo giờ Việt Nam** — cùng chỗ duy nhất mà hợp đồng hiện nay ghép giờ tường với ngày (`identity.proto:1035-1049`) |

## Còn mở — hỏi người dùng / pháp chế, không tự chọn

| # | Câu | Vì sao phải chốt trước khi gieo số |
|---|---|---|
| 1 | **Số điều luật và chữ của từng hạn.** Sổ tiến độ viết tắt *"KN Đ.28, TC Đ.29"* và tự ghi rằng số điều do agent trích **chưa kiểm với văn bản gốc**. Kho không có bản luật nào để đối chiếu | Quyết định #2 đọc đúng chữ luật cho từng hạn. Chưa biết hạn nào viết "ngày", hạn nào viết "ngày làm việc", thì chưa biết dòng SLA nào mang đơn vị nào |
| 2 | Hạn nào trong **hai hạn** của C8 (xử lý đơn lúc vào sổ, giải quyết lúc thụ lý) ứng với điều nào | Hai hạn có thể theo hai điều khác nhau, và hai chữ khác nhau |
| 3 | **Ngày cuối rơi vào ngày nghỉ** (cuối tuần, lễ) thì hạn dời sang ngày làm việc kế tiếp hay giữ nguyên | Đếm ngày lịch không có nghĩa là bỏ qua lịch nghỉ ở **điểm cuối**. Nếu phải dời, identity cần `ngay_nghi_le` của xã — lại là cấu hình theo xã |
| 4 | Ngày bắt đầu (ngày thụ lý / ngày nhận) **có tính** là ngày thứ nhất không, và hạn kết thúc **lúc mấy giờ** của ngày cuối | Lệch một ngày ở đầu hoặc ở cuối là lệch một ngày trên mọi đơn. C16/C17 (24/09) chốt *"Số ngày xử lý tính CẢ ngày nhận"* cho **cột báo cáo**, không phải cho **hạn**; C9 chốt 17:00 cho nhiệm vụ sinh từ đơn, không phải cho hạn của chính đơn |
| 5 | Gia hạn giải quyết (trường `gia_han` mà domain-expert 24/09 nêu) đếm theo đơn vị nào | Cùng câu hỏi #1 cho hạn gia hạn |

### Trả lời của người dùng — 30/09/2026 (câu #1, #2): dựng theo bảng này, CỜ "CẦN PHÁP CHẾ ĐỐI CHIẾU"

Bảng dưới là **trí nhớ của agent, chưa đối chiếu văn bản gốc**. Người dùng chọn dựng theo nó ngay, số
nằm trong cấu hình SLA nên sửa không cần phát hành lại, và **trước khi phát hành cho xã thật một người
đối chiếu văn bản gốc** — cờ này chỉ gỡ khi có người ghi tên và ngày đã đối chiếu vào đây.

| Loại | Hạn của C8 | Mốc tính | Số | Chữ của luật | Điều (chưa đối chiếu) |
|---|---|---|---|---|---|
| `khieu-nai` (lần đầu) | xử lý đơn — lúc vào sổ | từ ngày nhận | thụ lý trong 10 | "ngày" → ngày lịch | Luật Khiếu nại 2011, Đ.27 |
| `khieu-nai` (lần đầu) | giải quyết — lúc thụ lý | từ ngày thụ lý | 30; vụ phức tạp 45 | "ngày" → ngày lịch | Luật Khiếu nại 2011, Đ.28 |
| `to-cao` | xử lý đơn — lúc vào sổ | từ ngày nhận | kiểm tra, thụ lý trong 07 | **"ngày làm việc"** → lịch làm việc xã | Luật Tố cáo 2018, Đ.24 |
| `to-cao` | giải quyết — lúc thụ lý | từ ngày thụ lý | 30; gia hạn 1 lần ≤ 30 (đặc biệt phức tạp: 2 lần) | "ngày" → ngày lịch | Luật Tố cáo 2018, Đ.30 |

Câu #3 (ngày cuối rơi vào ngày nghỉ), #4 (ngày đầu có tính, giờ kết thúc ngày cuối), #5 (đơn vị của gia
hạn — bảng trên ghi theo trí nhớ là "ngày") **vẫn mở** và thuộc cùng lượt đối chiếu của pháp chế.

## Hệ quả

- **Dễ hơn:** hạn trên màn hình khớp hạn luật định; thanh tra đối chiếu được mà không quy đổi.
- **Khó hơn:** một hệ, **hai đơn vị** đếm hạn. Mọi chỗ hiện *"trễ N ngày"* phải nói ngày gì — câu
  *"đơn vị 'trễ N ngày'"* sổ tiến độ đã ghi là còn chưa rõ, nay có hai lời đáp tuỳ loại đơn.
- **Phải trả sau:** dòng SLA mang đơn vị thì màn cấu hình SLA phải hiện đơn vị, và **khoá** đơn vị
  ngày lịch cho hai loại này — một xã đổi sang ngày làm việc là tự nới hạn luật định.
- **Không đổi:** ADR 0007 cho mọi hạn khác; không hồi tố (ADR 0007 quyết định 6).

→ ADR 0007 (giờ làm việc — quyết định bị làm ngoại lệ): `kb/10-decisions/0007-sla-working-hours.md`
→ ADR 0028 (hạn ấn định tại hành vi, lưu một lần) · ADR 0029 (chủ bảng SLA) · ADR 0039 (sổ đơn thư thuộc văn thư)
→ Việc dựng và các câu C4/C8/C9/C16: `kb/90-ephemeral/tien-do/service-documents.json` → `so-don-thu-cong-dan`
→ Hạn tự điền lúc vào sổ, hai hạn sau một ô (08/10/2026): xem `0084-don-thu-bam-prototype-08-10.md`

## Sửa đổi 08/10/2026

- Câu #3, #4 ở §Còn mở đã có trả lời (đếm theo Bộ luật Dân sự, cờ pháp chế vẫn đứng); chỗ tính và nơi
  lưu đơn vị của §Tính thế nào cũng đã đổi — xem ADR 0085 §Trả lời 08/10/2026 câu 2, 5.
