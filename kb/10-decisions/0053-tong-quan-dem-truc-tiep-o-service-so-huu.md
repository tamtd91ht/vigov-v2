---
id: 0053-tong-quan-dem-truc-tiep-o-service-so-huu
tier: T1
source: CURATED
owner: architecture
derived_from_commit: c5bfb1ad
expires: null
owns_facts:
  - "/tong-quan đếm trực tiếp ở service sở hữu từng sổ, web-admin ghép, không bảng snapshot, không qua service-reporting — và cái giá"
  - "/bao-cao đếm trực tiếp ở service sở hữu như /tong-quan, không dùng service-reporting cho các con số ấy (chốt 04/10/2026)"
  - "kỳ so sánh của /bao-cao: Tuần/Tháng/Quý/Năm như /tong-quan; kỳ Tuỳ chọn so với cùng số ngày liền trước; đặc tả 13 §3/§9.1 bị thay cho kỳ có tên (chốt 04/10/2026)"
  - "bảng 'Tình hình thực hiện theo bộ phận' của /bao-cao: tên, phạm vi mọi đơn vị + dòng 'Chưa xác định bộ phận', chỉ nhiệm vụ, sắp theo Tổng việc, định nghĩa bốn cột, và vì sao không 'xếp hạng' (chốt 04/10/2026)"
  - "phạm vi /bao-cao đợt này (MVP) và những phần không làm: xuất PDF/XLSX/PPTX, biểu đồ So sánh với kỳ trước, job Gửi báo cáo định kỳ, Thành lập mới (chốt 04/10/2026)"
  - "lựa chọn khi dựng /bao-cao (không phải quyết định của khách): câu meta, cột Quá hạn là số tồn hiện tại, hai khoá cho task-unit-summary, đơn vị đã xoá mềm thành dòng riêng, ranh giới kỳ Tuỳ chọn"
  - "budget-indicators giữ riêng budget.read, không lồng report.read — đóng việc còn mở #6 (chốt 04/10/2026)"
  - "hai khoá lồng nhau report.read + khoá đọc của phân hệ trên mọi tuyến số liệu /tong-quan (trừ budget-indicators, sửa đổi 04/10/2026 B6), và vì sao web không được tin x-vigov-permission của các tuyến ấy"
  - "kỳ Tuần/Tháng/Quý/Năm (mặc định Tháng, tuần bắt đầu thứ Hai, Asia/Ho_Chi_Minh, client tính [from,to)) và kỳ so sánh = kỳ liền trước cùng loại, cùng phần đã trôi qua, chỉ cho số theo kỳ"
  - "định nghĩa từng ô của /tong-quan đợt 1: phản ánh nhận vào, đang xử lý, trễ theo trần phân loại; nhiệm vụ tạm dừng, việc con, đúng hạn theo han_ban_dau; văn bản đến theo ngay_den (tạm)"
  - "khối Cần xử lý ngay: top 10 gộp ba hàng đợi, những trường được hiện, ngưỡng nghiêm trọng 48 giờ làm việc qua identity, không có dòng 'quá hạn N ngày M giờ'"
  - "phạm vi /tong-quan đợt 1 và câu 'Chưa có dữ liệu nguồn' cho ô không có nguồn"
  - "drill-down ?metric= (+from/to) tới /nhiem-vu, /phan-anh, /van-ban và vì sao ca kiểm không-trạng-thái-URL của sổ văn bản đến được nới"
  - "ảnh trước/sau bị chặn, cảnh báo thiên tai để đợt sau, trên /tong-quan"
  - "danh sách chỗ /tong-quan của v2 cố ý khác kho yêu cầu, người dùng chốt giữ hướng v2"
  - "tab Báo cáo của /phan-anh: kỳ như §3, đúng hạn/trễ dùng đúng vị từ của /tong-quan, ô 'Đang trễ hạn (hiện tại)' là số tồn, bảng theo lĩnh vực, theo đơn vị (giờ làm việc từ lúc giao tới xu_ly_xong_luc), theo thôn — định nghĩa từng bảng (chủ dự án, 09/10/2026)"
---

# 0053. Tổng quan điều hành: đếm trực tiếp ở service sở hữu, không qua `reporting`

**Trạng thái:** đã chốt (người dùng — trong phiên vigov-v2-41, xác nhận và bổ sung ngày
**28/09/2026**) · mọi dòng ghi **"tạm"** hoặc **"chờ khách"** là **chưa được khách chốt** ·
**Lệch** ADR 0010 §*Vì sao Elasticsearch KHÔNG dùng cho báo cáo* (`:75-76`) và ADR 0001 (`:65`,
`:104`) **cho riêng `/tong-quan`** — không thay hai ADR ấy ở chỗ khác · Phạm vi: M7 đợt 1.
**Sửa đổi 04/10/2026:** mở `/bao-cao` (cuối tệp). **Sửa đổi 09/10/2026:** tab Báo cáo của `/phan-anh`
(cuối tệp).

## Bối cảnh

Trước ngày 28/09/2026 các quyết định dưới đây chỉ nằm trong chú thích Go của các tuyến số liệu
đã dựng (`service-petitions/internal/http/summary.go:3-18`,
`service-petitions/internal/domain/summary_metrics.go:22-35,166-212`,
`service-petitions/internal/store/citizen_report_summary.go:26-86`,
`service-petitions/internal/store/task_summary.go:30-54`,
`service-documents/internal/domain/incoming_dashboard.go:22-50,103-158`). `kb/` không có dòng nào.
Chú thích không ai đọc được khi dựng màn web; và vài quyết định trong đó ngược ADR cũ, đặc tả
`docs/ui-ux/` và kho yêu cầu — chính là thứ sáu tháng sau sẽ bị hỏi lại.

## Quyết định

### 1. Đếm trực tiếp ở service sở hữu, web-admin ghép

> Người dùng, 28/09/2026: số liệu `/tong-quan` đếm **trực tiếp** ở service sở hữu từng sổ;
> web-admin ghép; **không** bảng snapshot; **không** dùng `service-reporting` cho `/tong-quan`.

| Khối | Service đếm | Tuyến |
|---|---|---|
| Nhiệm vụ · Phản ánh | `petitions` | `service-petitions/internal/http/routes.go:1349-1404` |
| Văn bản đến | `documents` | `service-documents/internal/http/routes.go:568-591` |
| Thu – Chi ngân sách | `finance` | `GET /api/v1/budget-indicators`, `service-finance/internal/http/routes.go:954-956` |

**Ngược ba nguồn, người dùng chọn hướng này:**

| Nguồn | Nói gì |
|---|---|
| ADR 0010:75-76 · ADR 0001:65,104 | `reporting` dựng read model từ sự kiện; báo cáo đi qua đó |
| `docs/ui-ux/01-tong-quan-dieu-hanh.md:133-144` | *"Nên tính sẵn (materialize)"*, bảng `snapshot_tong_quan` |
| `../vigov-require/docs/spec/03-mo-hinh-du-lieu.md:335-344` | Bảng `dashboard_snapshots` |

**Vì sao đếm trực tiếp giữ được điều ADR 0010 muốn giữ.** ADR 0010:69 dẫn `13-bao-cao §10`: con
số và danh sách **không được lệch**. Ở đây mỗi ô và danh sách đằng sau nó dùng **cùng một vị từ
SQL** (`task_summary.go:6-10`, `citizen_report_summary.go:6-9`, `incoming_dashboard.go:5-9`), nên
hai thứ không thể lệch — một snapshot thì lệch với danh sách sống ngay khi có một dòng mới.

**Cái giá:**

| Điều | Nội dung |
|---|---|
| Không `Tính lại ngay`, không badge snapshot cũ | `docs/ui-ux/01:173-174` không áp dụng — không có gì để cũ |
| Thời điểm số liệu | Mỗi tuyến trả `as_of`; tuyến nào chưa trả thì trang ghi **thời điểm tải** |
| Chi phí | Mỗi lần tải trang đếm **toàn bộ sổ** của xã (`count(*) FILTER`). Với ~10.000 bản ghi mỗi xã (ADR 0010:78) là chấp nhận được; đo được là chậm thì mới bàn lại, và bàn lại là **ADR mới** |
| `/bao-cao` về sau | Nếu `/bao-cao` đi qua `reporting` mà `/tong-quan` thì không, hai nơi có hai nguồn cho một con số — phải quyết khi mở `/bao-cao` |

### 2. Hai khoá, cả hai cưỡng chế ở máy chủ

> Người dùng, 28/09/2026: mỗi tuyến số liệu đòi `report.read` **VÀ** khoá đọc của phân hệ
> (`task.read` / `feedback.read` / `document.read` / `budget.read`).

Hai `authz.RequirePermission` lồng nhau (`service-petitions/internal/http/routes.go:1321-1331`,
`service-documents/internal/http/routes.go:546-553`). Lĩnh vực `can-bo` cần thêm
`feedback.restricted` — ADR 0030 sở hữu điều ấy; ở đây chỉ ghi rằng mọi ô và hàng đợi phản ánh
loại `can-bo` bằng **cùng hằng** danh sách dùng (`citizen_report_summary.go:7-9`,
`routes.go:1354-1355`).

**Lỗi đã biết, web không được tin hợp đồng.** `tools/apidoc` chỉ ghi **một** khoá mỗi tuyến — khoá
`RequirePermission` trong cùng. Nên `x-vigov-permission` trong `kb/20-contracts/openapi.json` ghi
**thiếu**: bốn tuyến `petitions` chỉ ghi `report.read`, hai tuyến `documents` chỉ ghi
`document.read`. Web lấy cổng quyền từ hợp đồng sẽ hiện ô cho người máy chủ trả 403.

⚠ **Còn nợ:** `budget-indicators` hôm nay chỉ đòi `budget.read`
(`service-finance/internal/http/routes.go:954-956`) — **chưa** có `report.read`. Kiểm giao diện
không thay được (luật 5 cấm #1); phải lồng khoá ở `finance`.

### 3. Kỳ và so sánh

> Người dùng, 28/09/2026: kỳ **Tuần / Tháng / Quý / Năm**, mặc định **Tháng**; tuần bắt đầu **thứ
> Hai**; múi **Asia/Ho_Chi_Minh**; **client** tính `[from, to)` nửa mở; kỳ so sánh = **kỳ liền trước
> cùng loại, CÙNG PHẦN ĐÃ TRÔI QUA**; chỉ số **theo kỳ** mới có dòng so sánh, số **hiện trạng** thì không.

Máy chủ chỉ đếm trong hai mốc được đưa, và từ chối `from >= to` (`summary_metrics.go:22-47`). Cách đọc
của người viết (chưa ai xác nhận câu chữ): ngày 10 của tháng thì so với 10 ngày đầu tháng trước,
không với cả tháng trước.

| Nguồn ngược | Nói gì |
|---|---|
| `../vigov-require/docs/SRS.md:460` (M7.2.1) | *"so sánh cùng kỳ"* |
| `docs/ui-ux/13-bao-cao.md:112` | *"cùng độ dài ngay trước kỳ đang chọn"* |
| `../vigov-require/apps/api/app/modules/reports/service.py:88-129` | Cả kỳ cùng độ dài, múi **UTC** |

Người dùng chọn giữ hướng v2 ở mọi chỗ kho yêu cầu ngược, cho cụm việc này.

### 4. Định nghĩa từng ô

> Người dùng, 28/09/2026 — từng dòng dưới đây.

| Ô | Định nghĩa | Nơi |
|---|---|---|
| Phản ánh — **Nhận vào trong kỳ** | `vao_so_luc` trong kỳ, **kể cả** `khong-tiep-nhan`. Nhãn là *"Nhận vào trong kỳ"*, không phải *"Tiếp nhận"* — phiếu bị từ chối tiếp nhận vẫn được nhận vào | `citizen_report_summary.go:73-77` |
| Phản ánh — Đang xử lý | Mọi trạng thái trừ `da-dong`, `khong-tiep-nhan`, `chuyen-cap-tren`; phiếu **mở lại** được đếm | `citizen_report_summary.go:26-34` |
| Phản ánh — mẫu đúng hạn / trễ | Theo câu mở #26 — **ADR 0035 §#26 sở hữu**. Hệ quả ở đây: phiếu chưa phân loại quá trần tính trễ **trong kỳ trần của nó rơi vào**; `khong-tiep-nhan`, `chuyen-cap-tren` loại khỏi nhánh ấy | `citizen_report_summary.go:39-53` |
| Nhiệm vụ — Tạm dừng | `tam-dung` **không** vào Đang thực hiện, **không** vào Quá hạn; hiện thành **ô riêng "Tạm dừng"** | `task_summary.go:30-39`, `summary_metrics.go:56-66` |
| Nhiệm vụ — việc con | Đếm như dòng riêng | `task_summary.go:35-36` |
| Nhiệm vụ — Đúng hạn trong kỳ | So với `han_ban_dau` (`docs/ui-ux/01:67`); ô Quá hạn thì so `han_xu_ly` hiện hành | `task_summary.go:51-54` |
| Văn bản — **Đến trong kỳ** | `ngay_den` trong kỳ, đọc là ngày ở Asia/Ho_Chi_Minh. **TẠM — chờ khách**: đặc tả ghi *"vào sổ trong kỳ"* (`docs/ui-ux/01:73`); người dùng 28/09: giữ `ngay_den`, ghi là chờ khách | `incoming_dashboard.go:44-46,103-135` |

### 5. Khối "Cần xử lý ngay"

> Người dùng, 28/09/2026.

| Điều | Quyết định |
|---|---|
| Nội dung | **Top 10 gộp** ba hàng đợi (nhiệm vụ, phản ánh, văn bản đến), sắp theo **hạn đã lỡ**, lỡ lâu nhất trước |
| Mỗi dòng hiện **CHỈ** | phân hệ · mã · loại hạn · lĩnh vực/loại · *"quá hạn từ <hạn>"* · badge nghiêm trọng |
| Không hiện | Tiêu đề, nội dung, bộ phận đang giữ, người xử lý — luật 3 (`summary_metrics.go:166-172`). Ngược `docs/ui-ux/01:125-126` |
| Nghiêm trọng | `identity.AdvanceWorkingHours(hạn đã lỡ, 48 giờ làm việc) <= now` (`summary_metrics.go:194-212`, `incoming_dashboard.go:150-158`). Giờ **làm việc**, không giờ đồng hồ như `../vigov-require/apps/api/app/modules/reports/service.py:72-73` — luật 10 cấm #2. **Con số 48 chờ khách xác nhận** |
| Không có | Dòng *"quá hạn {N} ngày {M} giờ"* của `docs/ui-ux/01:125` — phép trừ giờ đồng hồ bị luật 10 cấm #2; đếm giờ làm việc thuộc `identity` (ADR 0007) |
| Rỗng | *"Không có việc nào cần xử lý ngay."* — theo `docs/ui-ux/01:175`, không theo prototype |

### 6. Phạm vi đợt 1

> Người dùng, 28/09/2026: gồm **Nhiệm vụ**, **Văn bản đến** (chỉ văn bản), **Phản ánh** (không có
> điểm hài lòng), **Thu – Chi ngân sách** với **nhãn KPI tạm**.

Nhãn tạm của Thu – Chi: theo tranh chấp câu mở #32 ghi ở
`kb/50-doi-chieu/2026-09-25-feat-m8-multitenant-foundation.md:73` (*"hỏi khách, tạm đổi nhãn ô
KPI"*) — tệp ấy sở hữu nội dung tranh chấp; #32 thuộc ADR 0035 §A.

> Người dùng, 28/09/2026: khối/ô **không có dữ liệu nguồn** hiện **MỘT dòng mờ "Chưa có dữ liệu
> nguồn"**, **không bao giờ là 0**.

Áp cho: Giải ngân · Kinh tế – Tài nguyên · Đơn thư trong kỳ · Điểm hài lòng · tỷ lệ đúng hạn văn
bản. Vì sao không 0: con số 0 trên bảng lãnh đạo là một khẳng định về cơ quan ("không có đơn
nào"), còn sự thật là "chưa đếm được".

**Ngoài đợt 1:** `/bao-cao` · xuất tệp (SRS M7.2.3) · chế độ trình chiếu (SRS M7.2.5).

### 7. Drill-down

> Người dùng, 28/09/2026: mọi con số mở danh sách của nó bằng `?metric=` (+ `from`/`to` cho ô theo
> kỳ); các màn danh sách `/nhiem-vu`, `/phan-anh`, `/van-ban` nhận tham số ấy — *"Làm luôn bộ nhận"*.

Theo SRS M7.2.2 (`../vigov-require/docs/SRS.md:461`). Giá trị `metric` là tên trường JSON của ô
(`summary.go:6-10`, `incoming_dashboard.go:36-42`). Metric lạ thì máy chủ từ chối, không bỏ lọc
(`summary_metrics.go:73-83`).

**Nới một ca kiểm, có chủ đích.** `web-admin/src/features/van-ban/ngan-van-ban-den.test.tsx:319-330`
cấm sổ văn bản đến dùng `useSearchParams`. Ca ấy được nới để cho **ĐỌC đúng các tham số này**; mọi
đường **ghi** vào URL vẫn bị cấm. Nới rộng hơn là mất rào canh lộ dữ liệu qua URL (luật 3 cấm #4).

### 8. Hai mảng ngoài đợt 1

| Mảng | Trạng thái | Vì sao |
|---|---|---|
| Ảnh trước/sau | **BỊ CHẶN** | MinIO, ClamAV chưa triển khai; chưa service nào nối `core/storage` — ADR 0052 §12 và §*Thứ tự dựng* bước 4 |
| Cảnh báo thiên tai | **ĐỂ ĐỢT SAU** — người dùng: *"Để đợt sau, hỏi khách hàng"* | Chưa biết service sở hữu (luật 2 dừng #1), khoá quyền (câu mở #27), loại cảnh báo (SRS ngược kho yêu cầu), adapter ZNS (ADR 0018) |

### 9. Chưa được chứng minh

Ca PostgreSQL "số dòng danh sách = con số" **chưa chạy lần nào** — thiếu `VIGOV_TEST_DSN`: ở
`documents` (`TestPgEachDrillDownHasAsManyRowsAsItsFigure`) và `petitions`
(`internal/store/summary_metrics_pg_test.go`, chỉ phía nhiệm vụ). Phía phản ánh **chưa có ca
PostgreSQL nào**. Tức phần SQL của các ô mới được kiểm bằng chuỗi, chưa bằng CSDL thật.

## Chỗ cố ý khác kho yêu cầu — người dùng đã quyết cho v2

| # | Điều | v2 | Kho yêu cầu | Chủ sự thật ở v2 |
|---|---|---|---|---|
| 1 | Ảnh nghiệm thu mặc định | Bắt buộc (`bat_buoc_anh_nghiem_thu` = true) | false | Câu mở #7, ADR 0008 |
| 2 | Số ảnh mỗi phiếu | ≤ 5 | 3 | ADR 0052 §10 |
| 3 | Link tệp | Ký, hết hạn, ràng danh tính + xã | Tệp không ký | ADR 0052 §12, luật 4 bất biến 7 |
| 4 | Mẫu số đúng hạn phản ánh | A ∪ B (phiếu chưa phân loại nằm trong mẫu) | — | ADR 0035 §#26 |
| 5 | Kỳ so sánh | Cùng loại, cùng phần đã trôi qua, Asia/Ho_Chi_Minh | Cả kỳ cùng độ dài, UTC | §3 ở đây |
| 6 | Loại cảnh báo thiên tai | Chưa chốt | Khác SRS | §8 ở đây — chờ khách |

## Còn mở — chưa ai quyết

| # | Việc | Của ai |
|---|---|---|
| 1 | Mốc `ngay_den` hay "vào sổ" cho *Đến trong kỳ* (§4) | Khách |
| 2 | Ngưỡng 48 giờ làm việc cho *nghiêm trọng* (§5) — có thành cấu hình theo xã không | Khách |
| 3 | Nhãn thật của ô Thu – Chi (§6, câu mở #32) | Khách |
| 4 | Cảnh báo thiên tai (§8) | Khách |
| 5 | `tools/apidoc` ghi đủ hai khoá (§2) | Phiên sửa `tools/` |
| 6 | Lồng `report.read` vào `budget-indicators` (§2) | Builder `service-finance` |

## ĐIỀU KIỆN DỪNG

1. Thêm bảng snapshot hay đưa `/tong-quan` qua `reporting` — là đảo §1, cần ADR mới
2. Hiện trên khối Cần xử lý ngay một trường ngoài danh sách §5
3. Một ô không có nguồn hiện 0 thay vì *"Chưa có dữ liệu nguồn"*
4. Tính khoảng quá hạn hay ngưỡng nghiêm trọng bằng giờ đồng hồ ở bất kỳ đâu ngoài `identity`
5. Cổng quyền của ô dựa vào `x-vigov-permission` khi lỗi §2 chưa sửa

→ ADR 0001 · 0007 (giờ làm việc) · 0008 · 0010 · 0030 (`feedback.restricted`) · 0035 §#26, §#32 ·
0052 (kho tệp)
→ Luật 1 · 3 · 5 bất biến 3c · 9 · 10

## Sửa đổi 04/10/2026 — mở `/bao-cao`

Người dùng chốt 04/10/2026 bốn điều cho menu `bao-cao` (đặc tả `docs/ui-ux/13-bao-cao.md`), B1–B4,
và đóng việc còn mở #6, B6. B5 là lựa chọn khi dựng, không phải quyết định của khách.
Mục này **đóng** dòng *"`/bao-cao` về sau"* ở §1 *Cái giá* (dòng 71 bản gốc). Không đảo điều nào
của §1–§9: `/bao-cao` **dùng lại** chúng. Đặc tả là của khách, không sửa; chỗ lệch ghi ở đây.

**Vì sao là sửa đổi, không ADR mới.** Câu hỏi do chính ADR này để mở; kỳ so sánh (§3) và định
nghĩa từng ô (§4) đã sở hữu ở đây. ADR mới sẽ đặt sự thật "kỳ so sánh" ở hai tệp — luật 9 cấm #2.
Đảo §1 mới cần ADR mới (ĐIỀU KIỆN DỪNG 1); đây là nới phạm vi, không đảo.

### B1. Nguồn số: đếm trực tiếp, không qua `reporting`

> Người dùng, 04/10/2026: `/bao-cao` đếm **trực tiếp** ở service sở hữu, đúng như `/tong-quan`;
> **không** dùng `service-reporting` cho các con số này.

Vì sao: đặc tả cấm hai trang lệch số (`docs/ui-ux/13-bao-cao.md:122`, cũng là lý do ADR 0010:74).
Hai nguồn cho một con số thì sẽ lệch. Hệ quả: §2 (hai khoá lồng nhau) áp nguyên cho mọi ô
`/bao-cao` dùng lại từ `/tong-quan` — trừ khối Thu – Chi, xem B6.

### B2. Kỳ và so sánh

> Người dùng, 04/10/2026: Tuần / Tháng / Quý / Năm trên `/bao-cao` so sánh theo **§3** — kỳ liền
> trước cùng loại, cùng phần đã trôi qua — giống hệt `/tong-quan`. Kỳ mới **Tuỳ chọn** (từ–đến)
> so với **cùng số ngày ngay liền trước**.

| Nguồn bị thay | Nói gì | Thay cho |
|---|---|---|
| `docs/ui-ux/13-bao-cao.md:41` (§3), `:112` (§9.1) | *"cùng độ dài"* liền trước cho mọi kỳ | Chỉ kỳ có tên. Kỳ Tuỳ chọn vẫn theo đặc tả |

Ranh giới ngày của kỳ Tuỳ chọn là lựa chọn khi dựng, không phải quyết định của khách — B5 mục e.

### B3. Bảng "Tình hình thực hiện theo bộ phận"

> Người dùng, 04/10/2026, theo đề xuất domain-expert.

| Điểm | Quyết định |
|---|---|
| Tên | **"Tình hình thực hiện theo bộ phận"** — không "Xếp hạng bộ phận" (`13-bao-cao.md:11,53`) |
| Vì sao không "xếp hạng" | Báo cáo của UBND không được "xếp hạng" Đảng uỷ, HĐND, MTTQ — những cơ quan UBND không chỉ đạo. Ví dụ đặc tả (`13-bao-cao.md:57-61`) chính là các khối ấy |
| Hàng | **Mọi** đơn vị trong cây tổ chức của xã, kể cả đơn vị không có việc, cộng một dòng **"Chưa xác định bộ phận"** |
| Sắp | `Tổng việc` giảm dần |
| Phạm vi | **Chỉ nhiệm vụ** |
| Đơn vị của một việc | `nhiem_vu.bo_phan_id` **hiện hành** (`service-petitions/migrations/0006_nhiem_vu.sql:269`), không bộ phận lúc giao |
| Tổng việc | Việc **đang trong tay** trong kỳ: tạo trước cuối kỳ, và không hoàn thành trước đầu kỳ |
| Hoàn thành | Cùng vị từ ô Hoàn thành của `/tong-quan` (`service-petitions/internal/store/task_summary.go:77-78`) |
| Đúng hạn | Cùng vị từ `/tong-quan`: so `han_ban_dau`, mẫu là việc hoàn thành có `han_ban_dau` (`task_summary.go:82,99`). Mẫu rỗng thì hiện `—` |
| Quá hạn | **Tồn hiện tại**, không theo kỳ — cùng vị từ ô Quá hạn của `/tong-quan` (`task_summary.go:62`) |

Bảng **không** có dòng so sánh kỳ trước.

### B4. Phạm vi đợt này

> Người dùng, 04/10/2026: MVP = trang + 6 nhóm KPI dùng lại từ `/tong-quan` + kỳ Tuỳ chọn + bảng B3.

| Không làm đợt này | Nguồn ở đặc tả | Khớp với |
|---|---|---|
| Xuất PDF / XLSX / PPTX | `13-bao-cao.md:87-95` | §6 *Ngoài đợt 1* ở trên; ADR 0068:260 |
| Biểu đồ "So sánh với kỳ trước" | `13-bao-cao.md:69-83` | — |
| Job "Gửi báo cáo định kỳ" | `13-bao-cao.md:116` | ADR 0058:126 — vẫn **HOÃN**: không có xuất tệp thì không có gì để gửi |
| Chỉ tiêu "Thành lập mới" | `13-bao-cao.md:81` | — |

Mục menu Báo cáo hôm nay mang dấu "?" — ADR 0068:287 sở hữu điều ấy.

### B5. Lựa chọn khi dựng — KHÔNG phải quyết định của khách

Phiên chính chọn 04/10/2026 khi dựng `/bao-cao`. Đây là **lựa chọn của nhà cung cấp**, chưa người
dùng hay khách nào chốt câu chữ; khách nói khác thì đổi theo khách.

| # | Điều | Lựa chọn | Vì sao |
|---|---|---|---|
| a | Dòng meta của trang (đặc tả `13-bao-cao.md:15,24` ghi *"cùng độ dài kỳ liền trước"*) | Kỳ có tên: dùng câu so sánh của `/tong-quan` (cùng loại, cùng phần đã trôi qua). Tuỳ chọn: *"so với cùng số ngày liền trước"* | Câu của đặc tả sai với kỳ có tên theo B2 |
| b | Cột Quá hạn (đặc tả `13-bao-cao.md:64` đòi thanh tỷ lệ quá hạn/tổng) | **Số trần**, nhãn ghi rõ là tồn **hiện tại**. Không thanh | Tử số là tồn hiện tại, mẫu số là việc trong kỳ — một tỷ lệ trộn hai thời điểm là con số không ai đọc đúng được |
| c | Khoá quyền của tuyến mới `GET /api/v1/task-unit-summary` | `report.read` **VÀ** `task.read`, lồng như `task-summary` | Cùng luật hai khoá của §2 |
| d | Đơn vị không còn trong danh sách đơn vị (đã xoá mềm) mà còn việc | **Dòng riêng**, nhãn ghi là đơn vị không còn trong danh sách. Không bao giờ gộp vào "Chưa xác định bộ phận" | Gộp vào là gán việc của một đơn vị có thật cho "không ai" — ghi sai trách nhiệm |
| e | Ranh giới kỳ Tuỳ chọn | *Từ ngày* D1 – *Đến ngày* D2 **tính cả hai ngày**: `[D1 00:00, D2+1 00:00)` ở Asia/Ho_Chi_Minh, N ngày; kỳ so sánh `[D1 − N ngày, D1)` | Người chọn "đến ngày 17" hiểu là gồm ngày 17; nửa mở giữ quy ước `[from, to)` của §3 |

### B6. Đóng việc còn mở #6 — `budget-indicators` giữ một khoá `budget.read`

> Người dùng, 04/10/2026: việc còn mở #6 (bảng *Còn mở* ở trên) **đóng**. Tuyến
> `GET /api/v1/budget-indicators` **giữ riêng `budget.read`**, không lồng `report.read`.

Điều này thay mục *⚠ Còn nợ* cuối §2 và dòng #6 của bảng *Còn mở*.

Vì sao: cùng tuyến ấy nuôi thẻ chỉ số năm của sổ Thu – Chi
(`web-admin/src/features/thu-chi/bang-thu-chi.tsx:223`; tài liệu tuyến `@screen 07-thu-chi-ngan-sach
§9 quy tắc 6`, `service-finance/internal/http/routes.go:979-987`). Lồng `report.read` sẽ giấu thẻ
khỏi cán bộ ngân sách không có `report.read`. Cổng cho khối Thu – Chi trên `/tong-quan` và
`/bao-cao` nằm ở chỗ khác: khối chỉ hiện với tài khoản có `budget.read`
(`web-admin/src/features/dashboard/overview.tsx:133`, `showBudget`); máy chủ vẫn đòi `budget.read`.
Hệ quả: với khối Thu – Chi, người có `budget.read` mà không có `report.read` vẫn đọc được ba chỉ số
năm — đúng điều sổ Thu – Chi vốn cho họ đọc.

## Sửa đổi 09/10/2026 — tab Báo cáo của `/phan-anh`

**Người quyết:** chủ dự án, 09/10/2026, trong phiên chính (`/fix-web-admin --menu=phan-anh-nguoi-dan`),
qua phiếu hỏi. **Chưa dựng.** Bề mặt: tab **Báo cáo** của màn `/phan-anh` (web-admin, cán bộ).

**Vì sao là sửa đổi, không ADR mới.** Kỳ (§3), vị từ đúng hạn (§4, dẫn ADR 0035 §#26) và luật "một ô
một vị từ với danh sách" (§1) đã sở hữu ở đây. Tab này **dùng lại** chúng; ADR mới sẽ đặt sự thật "kỳ"
và "đúng hạn" ở hai tệp — luật 9 cấm #2. Như B1: đếm trực tiếp ở `petitions`, không qua `reporting`.

### C1. Kỳ

Tuần / Tháng / Quý / Năm, Asia/Ho_Chi_Minh, `[from, to)` — **đúng §3** như `/bao-cao`. **Không** "90
ngày" của prototype: một cửa sổ trượt 90 ngày không khớp kỳ nào của `/tong-quan` hay `/bao-cao`, nên cùng
một con số sẽ hiện hai giá trị ở hai trang.

### C2. Định nghĩa chung

| Điểm | Quyết định |
|---|---|
| Đúng hạn / trễ | **Đúng vị từ** `citizenReportMetricCondition` (`service-petitions/internal/store/citizen_report_summary.go:40-84`, mẫu A ∪ B của câu mở #26 / ADR 0035). Không viết vị từ thứ hai |
| "Đang trễ hạn (hiện tại)" | Ô **mới**, **số tồn** lúc đọc, suy từ hạn đã lưu so với lúc đọc (luật 10 bất biến 3). **Không bao giờ là tỷ lệ** — cùng lý do B5 mục b: tử số tồn hiện tại, mẫu số theo kỳ là con số không ai đọc đúng |
| Xoá mềm | Loại **khắp nơi** (luật 7 bất biến 2) |
| Lĩnh vực `can-bo` | Loại khi người đọc không có `feedback.restricted` — cùng hằng với danh sách (§2) |
| Phiếu gộp | "Nhận vào" đếm mọi phiếu; đúng hạn, theo lĩnh vực, theo đơn vị chỉ phiếu chính — **ADR 0087 §7 sở hữu** |

### C3. Bảng theo lĩnh vực

| Điểm | Quyết định |
|---|---|
| Hàng | **Một hàng mỗi mã lĩnh vực**, cộng một hàng riêng **"Chưa phân loại"** — không gộp vào hàng nào |
| `can-bo` | Như C2 |
| Hài lòng | Hiện **kèm số mẫu** (số phiếu được đánh giá). Điểm trên 2 phiếu và trên 200 phiếu không đọc như nhau |

### C4. Bảng theo đơn vị

| Điểm | Quyết định |
|---|---|
| Thời gian xử lý trung bình | Tính bằng **giờ làm việc** (ADR 0007), từ lúc **giao cho đơn vị** tới `xu_ly_xong_luc` |
| Đo bằng | `identity` `MeasureWorkingHours` (`proto/vigov/identity/v1/identity.proto:1297`). **Không** trừ giờ đồng hồ — luật 10 cấm #2 |
| Quy về đơn vị nào | Đơn vị **đang giữ phiếu lúc `xu_ly_xong_luc`** |
| Mẫu | Chỉ phiếu **xử lý xong trong kỳ** |

### C5. Bảng theo thôn

| Điểm | Quyết định |
|---|---|
| Đếm | Phiếu **nhận vào trong kỳ** theo `thon_id`, cộng cột **trễ hạn hiện tại** (số tồn) |
| Hàng | Luôn có hàng **"Chưa xác định địa bàn"** — kể cả khi bằng 0 |
| Loại | `khong-tiep-nhan` và **phiếu phụ** đã gộp (ADR 0087) |
| Giữ | `chuyen-cap-tren` — vụ việc vẫn xảy ra trên địa bàn ấy |
| Thôn của phiếu | Thôn ghi trên phiếu lúc tiếp nhận, không suy từ toạ độ — ADR 0088 sở hữu |

### Còn mở khi dựng — hỏi, không tự quyết

| # | Việc | Vì sao không tự chọn |
|---|---|---|
| 1 | "Lúc giao cho đơn vị" (C4) khi phiếu được giao lại: lần giao cho đơn vị giữ phiếu lúc xong, hay lần giao đầu | Hai cách cho hai con số khác nhau cho cùng đơn vị |
| 2 | Tab có dòng so sánh kỳ trước (§3) không | Chủ dự án không nói |
| 3 | Ô "Đang trễ hạn (hiện tại)" có loại phiếu phụ không | ADR 0087 còn mở #4 |
| 4 | Ca PostgreSQL "số dòng danh sách = con số" cho các bảng mới | §9: phía phản ánh chưa có ca PostgreSQL nào |
