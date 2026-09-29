---
id: 0058-automation-jobs-in-owning-service
tier: T1
source: CURATED
owner: architecture
derived_from_commit: 57d5101
expires: null
owns_facts:
  - "việc nền của tab Tự động hoá chạy trong tiến trình của service sở hữu dữ liệu, mỗi việc một ticker, một khoá advisory PostgreSQL mỗi việc — không dựng RabbitMQ"
  - "cấu hình việc nền theo xã thuộc identity cạnh bảng sla, sửa dưới admin.sla, mặc định tắt, bên chạy đọc qua gRPC"
  - "việc nào dựng, refresh_dashboards bị bỏ, send_scheduled_reports hoãn, và nhắc hạn gồm cả phiếu phản ánh"
  - "mọi độ trễ trong việc nền đếm bằng giờ làm việc qua identity, không chép giờ đồng hồ của kho yêu cầu"
  - "tự đóng phiếu cho-dan-xac-nhan không thuộc việc nền tự động hoá"
  - "tuyến chạy ngay có dù kho yêu cầu không có"
---

# 0058. Việc nền của tab Tự động hoá: chạy trong service sở hữu dữ liệu, không qua RabbitMQ

**Trạng thái:** đã chốt (người dùng, **29/09/2026**) · **Bổ sung** ADR 0010 (`:37-38`, `:86-93`)
cho riêng việc nền của `docs/ui-ux/14-cau-hinh.md` §9 — RabbitMQ không còn là nơi chạy chúng ·
Mốc leo thang thuộc ADR 0029 §Bổ sung 29/09, không chép sang đây.

## Bối cảnh

ADR 0010 xếp *"nhắc hạn, leo thang, 5 job tự động hoá"* vào **RabbitMQ**, vì plugin delayed
message (`0010:38`, `0010:89-90`). Tới 29/09/2026 cụm chưa có broker nào; ADR 0057 (đang viết song
song) chốt rằng tính năng chưa nối dây thì không service nào khai biến của nó, RabbitMQ nằm trong
danh sách ấy (`0057:13`).

Kho yêu cầu chạy năm việc bằng một tiến trình nền riêng, gõ cửa mỗi năm phút rồi đọc bảng lịch
của từng xã (`../vigov-require/apps/api/app/modules/admin/automation.py:9-10`). v2 không có tiến
trình ấy, và không có service nào sở hữu cả ba sổ nhiệm vụ, phản ánh, văn bản.

## Phương án

| Phương án | Được | Mất |
|---|---|---|
| RabbitMQ như ADR 0010 | Hàng đợi bền, trễ có hẹn giờ | Dựng, giám sát, vá thêm một broker trước khi có việc đầu tiên. Người dùng không chọn hạ tầng mới |
| Một service lập lịch riêng | Một chỗ cho mọi lịch | Nó phải đọc sổ của ba service, hoặc ra lệnh cho chúng — một service thứ tám không sở hữu dữ liệu nào |
| CronJob của k8s | Không mã lập lịch | Nhịp đặt trong manifest, không theo xã; xã đổi nhịp là đổi manifest (luật 1 bất biến 10) |
| **Trong tiến trình, ở service sở hữu dữ liệu, mỗi việc một ticker + khoá advisory** | Không hạ tầng mới; đọc sổ của chính mình, không gọi chéo để đếm | Mỗi service tự mang vòng lặp của mình. Không có hàng đợi bền: lượt lỡ khi mọi pod tắt thì chạy ở nhịp kế |

## Quyết định

> **Người dùng, 29/09/2026:** việc nền của tab Tự động hoá chạy **trong tiến trình** của service
> sở hữu dữ liệu — `petitions` cho nhiệm vụ và phiếu phản ánh, `documents` cho văn bản — mỗi việc
> một ticker, một **khoá advisory PostgreSQL mỗi việc** để chỉ một pod chạy. **Không** dựng
> RabbitMQ cho việc này.

Lý do người dùng nêu: chưa broker nào được triển khai, ADR 0057 đang viết, và không thêm hạ tầng.

### 1. Một pod chạy, nhờ khoá advisory

Mỗi nhịp, pod nào lấy được khoá của việc ấy thì chạy; pod khác bỏ qua nhịp đó. Khoá nằm trong CSDL
của **chính** service chạy việc, nên không có khoá xuyên service. Tiền lệ trong kho:
`core/migrate/migrate.go:124-145` — khoá theo phiên, nên phải giữ **một** kết nối suốt lượt.

Không có hàng đợi thì không có "giao ít nhất một lần" của hàng đợi, nhưng lượt chạy lại vẫn xảy ra
(pod chết giữa lượt, "chạy ngay" chồng lên nhịp). Nên mỗi thông báo phải **idempotent** theo một
khoá chống trùng (luật 2 bất biến 5) — cùng ý với `dedupe_key` của kho yêu cầu
(`../vigov-require/apps/api/app/workers/sla.py:186`).

### 2. Cấu hình theo xã thuộc `identity`, cạnh bảng `sla`

> **Người dùng, 29/09/2026:** cấu hình việc nền theo xã — bật/tắt, nhịp, giờ/phút chạy, thứ trong
> tuần, `last_run_at`, kết quả lần cuối — thuộc `service-identity`, cạnh bảng `sla`; sửa dưới
> **`admin.sla`**; bên chạy đọc qua **gRPC**.

| Điều | Căn cứ |
|---|---|
| Khoá `admin.sla` | Kho yêu cầu dùng đúng khoá này cho hai tuyến tự động hoá (`../vigov-require/apps/api/app/modules/admin/router.py:43-69`). Khoá đã gieo — không bịa khoá (luật 5 bất biến 3c) |
| Cạnh `sla` | Hai bên chạy (`petitions`, `documents`) cùng đọc; không thuộc bên nào — phép thử (a)(b) của ADR 0029 §Cái giá |
| **Mặc định tắt** | `automation.py:12-13`; đặc tả `14-cau-hinh.md:324` |
| Múi giờ | `Asia/Ho_Chi_Minh` (`14-cau-hinh.md:338`) |

**Cái giá:** `identity` phình thêm một bảng không dính tới con người — đúng cảnh báo của ADR 0029
§Cái giá. Phép thử (c) ở đó (*đổi cùng nhịp với bộ máy hành chính của xã*) là điểm yếu nhất của
chỗ đặt này. Điều bù lại (nhận xét của người ghi): hai bảng được sửa bởi cùng một người, dưới cùng
một khoá `admin.sla`.

`last_run_at` và kết quả lần cuối do **bên chạy** sinh ra nhưng **`identity`** giữ. Tức cần một
đường ghi ngược từ `petitions`/`documents` sang `identity`.

### 3. Thông báo cho cán bộ đi qua `service-comms`

> **Người dùng, 29/09/2026:** thông báo cho cán bộ đi qua thông báo nội bộ của `service-comms`.

`comms` sở hữu thông báo nội bộ (`service-comms/migrations/0005_thong_bao_noi_bo.sql`), nhưng hợp
đồng của nó hôm nay chỉ có `Health` (`proto/vigov/comms/v1/comms.proto:28`). Chưa có đường nào để
service khác gửi một thông báo — luật 2 điều kiện dừng #2.

### 4. Việc nào dựng, việc nào không

| Việc (khoá kho yêu cầu) | Quyết định | Vì sao |
|---|---|---|
| Nhắc sắp đến hạn và đã quá hạn (`sla_checker`) | **Dựng** — nhiệm vụ, văn bản **và phiếu phản ánh** | Đặc tả `14-cau-hinh.md:330` ghi cả ba. Kho yêu cầu quét nhiệm vụ, văn bản, đơn thư (`sla.py:108-208`), **không** quét phiếu phản ánh. Người dùng chọn theo đặc tả. "Sắp đến hạn" đọc `gio_sap_den_han` qua mốc cuối của ADR 0029 §Bổ sung 28/09 |
| Leo thang việc trễ hạn (`escalation`) | **Dựng** | Hai ngưỡng, mốc đếm: ADR 0029 §Bổ sung 29/09 |
| Bản tin đầu tuần (`weekly_digest`) | **Dựng** | — |
| Tính lại số liệu Tổng quan (`refresh_dashboards`) | **BỎ** | Ngược ADR 0053 §1: `/tong-quan` đếm trực tiếp, không snapshot, nên không có gì để tính lại |
| Gửi báo cáo định kỳ (`send_scheduled_reports`) | **HOÃN** tới khi có `/bao-cao` | `/bao-cao` ngoài đợt 1 (ADR 0053 §6). Chưa có báo cáo thì không có gì để gửi |

### 5. Độ trễ đếm bằng giờ làm việc

> **Người dùng, 29/09/2026:** mọi độ trễ tính bằng **giờ làm việc** qua `identity`.

Kho yêu cầu trừ giờ đồng hồ: `task.due_at - now` (`sla.py:129-133`), `now - task.due_at`
(`sla.py:299`), tiêu đề *"Quá hạn {day} ngày"* (`sla.py:242-248`). **Không chép** — luật 10 cấm
#2; phép cộng giờ làm việc thuộc `identity` (ADR 0007). Cùng lý do ADR 0053 §5 bỏ dòng *"quá hạn N
ngày M giờ"*.

### 6. Tự đóng phiếu `cho-dan-xac-nhan` KHÔNG thuộc tab này

> **Người dùng, 29/09/2026:** tự đóng phiếu chờ dân xác nhận **không** nằm trong việc nền tự động
> hoá; nó đi riêng.

Đóng phiếu là một chuyển trạng thái có tin báo công dân (ADR 0041) — luật 10 điều kiện dừng #3. Gộp
vào một công tắc bật/tắt của tab Tự động hoá là để một ô cấu hình quyết một chuyển trạng thái.

### 7. Tuyến "chạy ngay"

> **Người dùng, 29/09/2026:** có tuyến **chạy ngay** theo đặc tả `14-cau-hinh.md:380`
> (`POST /:ma/chay-ngay`), dù kho yêu cầu không có (`router.py:43-69` chỉ có danh sách và sửa).

Chạy ngay phải đi qua **cùng khoá advisory** với nhịp thường, nếu không hai lượt chạy chồng nhau.

## Hệ quả

- **Dễ:** không broker, không biến môi trường mới cho broker (ADR 0057). Mỗi service đếm trên CSDL
  của mình, cùng vị từ với danh sách của nó.
- **Khó:** ba vòng lặp giống nhau ở hai service. Phần chung (ticker, khoá, đọc cấu hình) nên nằm ở
  `core/` thay vì chép — việc của lượt dựng.
- **Trả sau:** ngày cần hàng đợi bền thật (gửi hàng loạt, thử lại có hẹn), RabbitMQ của ADR 0010
  vẫn là chỗ đã chọn — nhưng mở lại là **ADR mới**, không lặng lẽ thêm.
- **ADR 0010 vẫn đúng ở chỗ khác:** Kafka *"không dùng cho tác vụ có lịch"* (`0010:37`) vẫn giữ.

## Còn mở — chưa ai quyết

| # | Câu | Ai |
|---|---|---|
| 1 | Bản tin đầu tuần chạy ở service nào — nó gộp nhiệm vụ, phản ánh (và văn bản?), còn tiêu đề `report.notification.week` thuộc `reporting` (ADR 0024 §Phụ, *Bổ sung 29/09/2026*) | Người dùng |
| 2 | "Phản ánh nóng" trong bản tin (`14-cau-hinh.md:332`) nghĩa là gì — không nguồn nào định nghĩa | Khách |
| 3 | Leo thang áp cho những loại việc nào. Bảng `sla` có hai cột leo thang cho cả ba `loai_viec`; kho yêu cầu chỉ leo thang nhiệm vụ (`sla.py:284-321`) | Người dùng |
| 4 | Đơn thư (`documents`, ADR 0039) có vào nhắc hạn không. Kho yêu cầu có; đặc tả §9 không nêu | Người dùng |
| 5 | Ngưỡng "bộ phận giữ mà chưa phân công ai" (`14-cau-hinh.md:330`). Kho yêu cầu cứng 24 giờ đồng hồ (`sla.py:42`) — cả con số lẫn đơn vị đều không chép được | Khách |
| 6 | Tuyến chạy ngay đặt ở `identity` (nơi giữ cấu hình) hay ở service chạy việc | `contract-designer`, người dùng duyệt |
| 7 | Ba hợp đồng: bên chạy đọc cấu hình và ghi kết quả lần cuối vào `identity`; bên chạy gửi thông báo sang `comms`. Mức nhất quán và bù trừ của từng luồng (luật 2 bất biến 6) | `contract-designer` |
| 8 | Danh sách xã phải quét là một lần đọc **xuyên xã** — phải khai `// @cross-tenant: <lý do>` (luật 1 cấm #6); mỗi xã sau đó chạy trong ngữ cảnh của riêng nó | Lượt dựng |
| 9 | Vết kiểm toán của việc nền cần một **chủ thể hệ thống** mang mã nghiệp vụ (luật 6 bất biến 6 và 8). `core/audit` và `core/authz` hôm nay chưa có | Người dùng — luật 6 điều kiện dừng #1 |

## ĐIỀU KIỆN DỪNG

1. Đưa một việc của tab này sang RabbitMQ, Kafka hay CronJob — đảo §Quyết định, cần ADR mới
2. Một việc đọc sổ của service khác để đếm — sai §1, và luật 2 cấm #2
3. Bật mặc định một việc cho xã mới
4. Bất kỳ phép trừ giờ đồng hồ nào cho độ trễ, ngoài `identity`
5. Dựng lại `refresh_dashboards`, hoặc dựng `send_scheduled_reports` trước khi có `/bao-cao`
6. Gắn tự đóng phiếu `cho-dan-xac-nhan` vào công tắc của tab này

→ ADR 0007 (giờ làm việc) · 0010 (hạ tầng dữ liệu) · 0029 (bảng `sla`, mốc leo thang) · 0041
(bảng tin báo công dân) · 0053 (không snapshot) · 0057 (nạp cấu hình theo nhóm)
→ Luật 1 · 2 · 5 · 6 · 10
→ Đặc tả: `docs/ui-ux/14-cau-hinh.md` §9, §11
