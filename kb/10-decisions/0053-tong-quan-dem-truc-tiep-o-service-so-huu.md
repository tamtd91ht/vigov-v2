---
id: 0053-tong-quan-dem-truc-tiep-o-service-so-huu
tier: T1
source: CURATED
owner: architecture
derived_from_commit: 085c023
expires: null
owns_facts:
  - "/tong-quan đếm trực tiếp ở service sở hữu từng sổ, web-admin ghép, không bảng snapshot, không qua service-reporting — và cái giá"
  - "hai khoá lồng nhau report.read + khoá đọc của phân hệ trên mọi tuyến số liệu /tong-quan, và vì sao web không được tin x-vigov-permission của các tuyến ấy"
  - "kỳ Tuần/Tháng/Quý/Năm (mặc định Tháng, tuần bắt đầu thứ Hai, Asia/Ho_Chi_Minh, client tính [from,to)) và kỳ so sánh = kỳ liền trước cùng loại, cùng phần đã trôi qua, chỉ cho số theo kỳ"
  - "định nghĩa từng ô của /tong-quan đợt 1: phản ánh nhận vào, đang xử lý, trễ theo trần phân loại; nhiệm vụ tạm dừng, việc con, đúng hạn theo han_ban_dau; văn bản đến theo ngay_den (tạm)"
  - "khối Cần xử lý ngay: top 10 gộp ba hàng đợi, những trường được hiện, ngưỡng nghiêm trọng 48 giờ làm việc qua identity, không có dòng 'quá hạn N ngày M giờ'"
  - "phạm vi /tong-quan đợt 1 và câu 'Chưa có dữ liệu nguồn' cho ô không có nguồn"
  - "drill-down ?metric= (+from/to) tới /nhiem-vu, /phan-anh, /van-ban và vì sao ca kiểm không-trạng-thái-URL của sổ văn bản đến được nới"
  - "ảnh trước/sau bị chặn, cảnh báo thiên tai để đợt sau, trên /tong-quan"
  - "danh sách chỗ /tong-quan của v2 cố ý khác kho yêu cầu, người dùng chốt giữ hướng v2"
---

# 0053. Tổng quan điều hành: đếm trực tiếp ở service sở hữu, không qua `reporting`

**Trạng thái:** đã chốt (người dùng — trong phiên vigov-v2-41, xác nhận và bổ sung ngày
**28/09/2026**) · mọi dòng ghi **"tạm"** hoặc **"chờ khách"** là **chưa được khách chốt** ·
**Lệch** ADR 0010 §*Vì sao Elasticsearch KHÔNG dùng cho báo cáo* (`:75-76`) và ADR 0001 (`:65`,
`:104`) **cho riêng `/tong-quan`** — không thay hai ADR ấy ở chỗ khác · Phạm vi: M7 đợt 1.

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
