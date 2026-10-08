---
id: 0084-don-thu-bam-prototype-08-10
tier: T1
source: CURATED
owner: domain
derived_from_commit: ac1fe186
expires: null
owns_facts:
  - "đơn thư công dân VẼ theo prototype nhưng DỮ LIỆU giữ bộ trạng thái TT 05/2021 (C3): thanh 4 bước Mới vào sổ → Đã phân công → Đang xử lý → Đã giải quyết + 2 nhánh Chuyển cấp trên / Lưu, không thụ lý là NHÓM HIỂN THỊ suy ra từ trạng thái + bộ phận đang giữ; không migration gộp trạng thái (chốt 08/10/2026)"
  - "bộ lọc trạng thái đơn thư theo nhãn nhóm của prototype, có 'Đã phân công', KHÔNG có 'Chờ phân công' — lệch prototype có chủ ý (chốt 08/10/2026)"
  - "che thông tin người gửi đơn thư GIỮ như ADR 0078 #4 (SĐT che, địa chỉ ẩn, đơn tố cáo không danh tính lẫn trích yếu) — lệch prototype có chủ ý (chốt 08/10/2026)"
  - "hạn đơn thư TỰ ĐIỀN lúc vào sổ = ngày nhận + số ngày cấu hình theo loại đơn của xã; chưa cấu hình → 'Không đặt hạn'; cán bộ vẫn sửa được; khiếu nại/tố cáo giữ hai hạn ADR 0064 sau một ô 'Hạn giải quyết' — đảo ADR 0079 lô 5 Q18 'không tự điền hạn' cho đơn thư (chốt 08/10/2026)"
  - "báo cáo đơn thư: đơn đã giải quyết KHÔNG có hạn tính là ĐÚNG hạn (đảo C12); chủ dự án chấp nhận rủi ro tỷ lệ đúng hạn báo lên trên bị thổi phồng (chốt 08/10/2026)"
  - "đóng đơn kiến nghị-phản ánh / đề nghị không bắt buộc nội dung trả lời; khiếu nại / tố cáo vẫn bắt buộc số, ngày, người ký, cơ quan ban hành văn bản kết quả — C10 thu hẹp vào KN/TC (chốt 08/10/2026)"
  - "lượt 08/10/2026 dựng cả backend: nhập Excel, xuất Excel báo cáo, chuyển thành nhiệm vụ (trừ đơn tố cáo, C9), nguồn vào sổ, số đếm trên tab, tiêu đề 'Đơn thư công dân' ở tab Đơn thư/Báo cáo (chốt 08/10/2026)"
---

# 0084. Đơn thư công dân bám prototype (08/10/2026)

**Trạng thái:** đã chốt · **Ngày:** 2026-10-08 · **Người quyết:** chủ dự án, nói "bám prototype"
khi giao tài liệu `tmp/web/updated/van-ban-don-thu-theo-prototype-v2.md` (ngoài git), rồi trả lời
sáu câu ở cổng của lượt · **Thay một phần** ADR 0078 (#3, #5 phần tiêu đề, #6 phần đơn thư) và ADR
0079 lô thứ năm Q18 (chỉ cho đơn thư) · **Giữ** ADR 0078 #4, ADR 0064, ADR 0039.

## Bối cảnh

Tài liệu giao việc so site Thăng Bình với prototype và đòi làm theo prototype: bộ trạng thái 7 mã
mới + migration gộp dữ liệu cũ (§8), hiện SĐT/địa chỉ đầy đủ, bỏ "Không rõ người gửi", một ô trả lời,
hạn tự tính theo loại đơn, đơn không hạn tính đúng hạn. Nhiều mục trái các câu đã chốt 24/09
(C-list, `kb/90-ephemeral/tien-do/service-documents.json:63`) và luật 3, 7, 10. Chủ dự án chọn
từng điểm dưới đây.

## Quyết định

| # | Điểm | Chốt | So với prototype |
|---|---|---|---|
| 1 | Che người gửi | **Giữ** như ADR 0078 #4. Tài liệu §7 tự đánh dấu là cần người duyệt; không làm cờ `petition.mask_contact` / `petition.denunciation_masking` | Lệch có chủ ý |
| 2 | Trạng thái | **Vẽ** như prototype, **dữ liệu** giữ bộ TT 05/2021 (C3). Không migration §8 | Giống về hình, khác về dữ liệu |
| 3 | Hạn | **Theo prototype**: tự điền lúc vào sổ, sửa được | Giống, thêm hai hạn KN/TC |
| 4 | Báo cáo | **Theo prototype**: không hạn = đúng hạn, sắp theo tổng, dòng "Chưa phân công" | Giống |
| 5 | Nhãn | Đúng chữ prototype; giữ "Chuyển cấp trên"; bỏ "Chờ phân công" khỏi bộ lọc | Lệch có chủ ý ở một mục |
| 6 | Chuyển thành nhiệm vụ | **Bật**; đơn tố cáo không chuyển (C9 đứng) | Giống, trừ tố cáo |
| 7 | Phạm vi | Làm cả backend lượt này | — |

### 2. Nhóm hiển thị suy ra từ trạng thái TT 05/2021

| Nhóm (nhãn prototype) | Trạng thái dữ liệu |
|---|---|
| Mới vào sổ | `moi-vao-so`, **chưa** có bộ phận đang giữ |
| Đã phân công | `moi-vao-so`, **đã** có bộ phận đang giữ — phân công vẫn là thuộc tính (C3) |
| Đang xử lý | `dang-xu-ly-don` · `thu-ly` · `dang-giai-quyet` |
| Đã giải quyết | `da-giai-quyet` |
| Chuyển cấp trên (nhánh) | `chuyen-don` |
| Lưu, không thụ lý (nhánh) | `khong-thu-ly` · `huong-dan` · `luu-don` · `dinh-chi` |

- Bấm ô "Đang xử lý" hoặc "Lưu, không thụ lý": khung xác nhận có thêm **ô chọn bước cụ thể** (vd.
  Thụ lý, Hướng dẫn), vì một nhóm ứng với nhiều trạng thái thật.
- Bộ lọc lọc theo nhóm. "Chờ phân công" bị bỏ: prototype không có luồng nào đặt đơn vào trạng thái
  ấy — dải chính chỉ có 4 bước (`../vigov-require/apps/admin/src/lib/document-display.ts:135-140`),
  `pending_routing` chỉ là một giá trị enum (`../vigov-require/apps/api/app/modules/documents/models.py:45`).
- Giữ ô "Không rõ người gửi" (C7 đứng).
- Ô trả lời: **một** textarea như prototype, là phần tóm tắt kết quả. Kiến nghị-phản ánh, đề nghị
  chuyển "Đã giải quyết" không bắt buộc trả lời. Khiếu nại, tố cáo **vẫn bắt buộc** số, ngày, người ký,
  cơ quan ban hành văn bản kết quả — C10 thu hẹp vào hai loại này.

**Vì sao không gộp trạng thái (§8):** gộp 8 trạng thái TT 05/2021 vào 3 là mất hẳn thông tin đơn đã
thụ lý hay chưa, đã hướng dẫn hay đã lưu, trên hồ sơ lưu trữ — sửa lịch sử, luật 7 cấm #5, và không đảo
được. Vẽ theo nhóm cho cùng hình prototype mà không mất dòng nào.

### 3. Hạn tự điền

- Lúc vào sổ: hạn = **ngày nhận + số ngày cấu hình theo loại đơn của xã**. Số là tham số cấu hình
  theo xã, không viết cứng (luật 10 cấm #3). Loại chưa cấu hình → "Không đặt hạn". Cán bộ vẫn sửa
  được hạn như ADR 0079 Q18.
- Khiếu nại, tố cáo **giữ hai hạn** của ADR 0064: hạn tự điền lúc vào sổ là **hạn xử lý đơn**; lúc thụ
  lý tính **hạn giải quyết** từ ngày thụ lý. Giao diện chỉ hiện một ô "Hạn giải quyết" (hạn của giai
  đoạn hiện tại).
- Không đổi: đơn vị đếm và chỗ tính theo ADR 0064 #1–#4 (KN/TC) và ADR 0007 (hai loại còn lại); hạn
  tính một lần tại hành vi ấn định và lưu (luật 10 bất biến 2). Cờ **"cần pháp chế đối chiếu"** của
  ADR 0064 vẫn đứng: số điều luật domain-expert nêu là trí nhớ, chưa đối chiếu văn bản gốc.

### 4. Báo cáo

- Đơn đã giải quyết **không có hạn** tính là **đúng hạn** — đảo C12 (mẫu số chỉ gồm đơn có hạn).
- Bảng theo loại đơn sắp theo tổng giảm dần; dòng đơn chưa giao bộ phận tên "Chưa phân công".

**Rủi ro chủ dự án chấp nhận:** domain-expert cảnh báo quy tắc này **thổi phồng tỷ lệ đúng hạn** trong
số liệu báo lên cấp trên — xã nào chưa cấu hình hạn sẽ ra 100% dù không ai đo hạn. Chủ dự án vẫn chọn
theo prototype.

### 7. Phạm vi lượt này

Nhập Excel (nguồn "Nhập từ Excel"), xuất Excel báo cáo, chip nguồn vào sổ (Nhập tay · Nhập từ Excel ·
Mini App · Thư điện tử; đơn cũ gán Nhập tay), số đếm trên tab "Đơn thư công dân (N)", tiêu đề H1 "Đơn
thư công dân" khi ở tab Đơn thư hoặc Báo cáo — tab Văn bản đến/đi giữ tiêu đề cũ. Chuyển thành nhiệm
vụ: nhiệm vụ liên kết đơn, kế thừa hạn (C9), qua hợp đồng `documents → petitions` (ADR 0039 hệ quả #2).

## Hệ quả

- Màn đơn thư trông như prototype nhưng không lộ SĐT, địa chỉ, danh tính người tố cáo — người đối
  chiếu hai site sẽ thấy khác ở đúng các chỗ ấy, và đó là chủ ý.
- Mọi chỗ đếm theo nhóm (bộ lọc, thanh trạng thái, báo cáo) phải dùng **một** bảng ánh xạ ở mục 2;
  hai bảng là hai con số lệch nhau.
- Tỷ lệ đúng hạn không còn so được với số trước 08/10/2026 (C12 cũ).
- Chưa chốt ở đây: chỗ nhập bốn trường văn bản kết quả của KN/TC trên giao diện một-textarea; chỗ
  lưu số ngày cấu hình theo loại đơn (dòng SLA của identity theo ADR 0064 §Tính thế nào là đề xuất,
  chưa dựng) — câu hỏi ở cổng của người dựng.

→ ADR 0078 (menu theo prototype, thứ hạng prototype/câu đã chốt) · ADR 0079 lô 5 Q18 (hạn do cán bộ đặt)
→ ADR 0064 (hai hạn KN/TC, ngày lịch) · ADR 0039 (sổ đơn thư thuộc documents) · ADR 0007 (giờ làm việc)
→ C-list 24/09: `kb/90-ephemeral/tien-do/service-documents.json` → `so-don-thu-cong-dan`

## Sửa đổi 08/10/2026

- §3 "ADR 0007 (hai loại còn lại)" là sai: `kien-nghi-phan-anh`, `de-nghi` đếm **ngày làm việc**, không
  phải giờ làm việc — chủ dự án chốt, xem ADR 0085 §Trả lời 08/10/2026 câu 3.
