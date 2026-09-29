---
id: 0058-automation-jobs-in-owning-service
tier: T1
source: CURATED
owner: architecture
derived_from_commit: 1796286
expires: null
owns_facts:
  - "việc nền của tab Tự động hoá chạy trong tiến trình của service sở hữu dữ liệu, mỗi việc một ticker, một khoá advisory PostgreSQL mỗi việc — không dựng RabbitMQ"
  - "cấu hình việc nền theo xã thuộc identity cạnh bảng sla, sửa dưới admin.sla, mặc định tắt; cài đặt theo việc, trạng thái chạy theo phạm vi (việc, loại việc)"
  - "việc nào dựng (sla_reminders, escalation, weekly_digest), refresh_dashboards bị bỏ, send_scheduled_reports hoãn, và nhắc hạn gồm cả phiếu phản ánh lẫn đơn thư"
  - "mọi độ trễ trong việc nền đếm bằng giờ làm việc qua identity, không chép giờ đồng hồ của kho yêu cầu"
  - "tự đóng phiếu cho-dan-xac-nhan không thuộc việc nền tự động hoá"
  - "tuyến chạy ngay đặt ở identity, chỉ đánh dấu; việc chạy ở nhịp kế của bên chạy (≤ 1 phút)"
  - "thông báo của việc nền vào hộp chuông hop_thu_thong_bao của comms, không vào sổ thông báo nội bộ"
  - "bản tin đầu tuần giao hai phần theo service sở hữu, và phản ánh nóng nghĩa là quá hạn hoặc bị chấm 1–2 sao trong tuần — 29/09/2026"
  - "bên chạy tìm xã phải quét từ kho của chính nó, bỏ xã không active qua platform GetTenant"
  - "leo thang áp cho nhiệm vụ, phiếu phản ánh (chỉ han_xu_ly_xong) và văn bản đến — người dùng 29/09/2026"
  - "nhắc hạn đơn thư chưa chạy vì service-documents chưa có sổ đơn thư; phạm vi DON_THU cố ý không được nhận"
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

Lượt chạy và kết quả lần cuối do **bên chạy** sinh ra nhưng **`identity`** giữ. Hợp đồng 1796286
chốt hình dạng (`proto/vigov/identity/v1/identity.proto`):

| Điều | Hợp đồng |
|---|---|
| Ba việc | `sla_reminders` · `escalation` · `weekly_digest` (`enum AutomationJob`). `sla_checker` của kho yêu cầu đổi tên thành `sla_reminders` |
| Đơn vị trạng thái chạy | **(việc, loại việc)** — `AutomationRunScope`. `PHAN_ANH` và `NHIEM_VU` do `petitions` chạy, `VAN_BAN_DEN` do `documents` chạy. Cài đặt (bật/tắt, nhịp) vẫn **một dòng mỗi việc**; `last_run_at` theo việc thì bên nào chạy trước sẽ đánh dấu xong hộ bên kia |
| Ai quyết "tới lượt" | `identity`, qua `ClaimDueAutomationRuns` — một lệnh ghi có điều kiện, một người thắng. `claimed_at` là "bây giờ" của cả lượt. Khoá advisory của §1 vẫn giữ để một pod của bên chạy làm một lượt |
| Ghi kết quả | `RecordAutomationRunOutcome` — một vết mỗi lượt, chủ thể hệ thống, cùng giao dịch |

Ranh giới giao dịch: `kb/30-indexes/transaction-boundaries.json`, `chay_tac_vu_tu_dong_theo_xa`.

### 2b. Xã nào phải quét

Không có RPC *"xã nào bật việc X"* — đó là một lần đọc xuyên xã không mang `x-tenant-id` (ADR 0012
quyết định 1). Theo phương án (b) của hợp đồng 1796286: **mỗi bên chạy liệt kê các xã từ kho của
chính nó**, khai `// @cross-tenant: <lý do>` (luật 1 cấm #6), **bỏ qua xã mà `platform` `GetTenant`
báo không `active`** (xã đã sáp nhập giữ dữ liệu — luật 7 bất biến 6), rồi hỏi `identity` từng xã
trong ngữ cảnh của xã ấy. Cái giá: xã chưa có bản ghi nào ở một service thì không nhận phần bản tin
của service đó.

### 3. Thông báo cho cán bộ đi qua `service-comms`

> **Người dùng, 29/09/2026:** thông báo cho cán bộ đi qua thông báo nội bộ của `service-comms`.

Thông báo vào **hộp chuông** `hop_thu_thong_bao` — hộp thư hợp nhất của `docs/ui-ux/08-thong-bao.md`
§8 — qua `CommsService.DeliverStaffNotifications` (`proto/vigov/comms/v1/comms.proto:30-105`, hợp
đồng 1796286). **Không** vào sổ thông báo nội bộ (`service-comms/migrations/0005_thong_bao_noi_bo.sql`):
đó là thông báo do một người có `announcement.create` soạn, và ghi nhắc việc vào đó là chôn thông
báo thật của xã dưới dòng máy sinh, với một tác giả giả. Chống trùng theo khoá của **việc nghiệp
vụ**, không theo `run_id` — công thức khoá nằm ở chú thích rpc, không chép sang đây.

### 4. Việc nào dựng, việc nào không

| Việc (khoá kho yêu cầu) | Quyết định | Vì sao |
|---|---|---|
| Nhắc sắp đến hạn và đã quá hạn (`sla_reminders`, kho yêu cầu gọi `sla_checker`) | **Dựng** — nhiệm vụ, văn bản, **phiếu phản ánh và đơn thư** | Đặc tả `14-cau-hinh.md:330` ghi ba loại đầu. Kho yêu cầu quét nhiệm vụ, văn bản, đơn thư (`sla.py:108-208`), **không** quét phiếu phản ánh. Người dùng chọn hợp của hai (**đơn thư: người dùng, 29/09/2026**, theo kho yêu cầu). "Sắp đến hạn" đọc `gio_sap_den_han` qua mốc cuối của ADR 0029 §Bổ sung 28/09. Ngưỡng "bộ phận giữ mà chưa phân công ai": một trường SLA mới — ADR 0029 §Bổ sung 29/09 |
| Leo thang việc trễ hạn (`escalation`) | **Dựng** | Hai ngưỡng, mốc đếm, người nhận mức 2: ADR 0029 §Bổ sung 29/09 |
| Bản tin đầu tuần (`weekly_digest`) | **Dựng** | Nội dung và nơi chạy: §8 dưới |
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

> **Người dùng, 29/09/2026:** tuyến chạy ngay đặt ở **`identity`** và chỉ **đánh dấu** việc ấy; việc
> chạy ở **nhịp kế** của bên chạy — **≤ 1 phút**.

Không có lời gọi từ `identity` sang bên chạy: lượt chạy ngay đi qua **cùng** `ClaimDueAutomationRuns`
và cùng khoá advisory với nhịp thường, nên hai lượt không chồng nhau. Một việc chạy ở hai service
(`sla_reminders`) thì dấu chạy ngay phải được **mỗi phạm vi** (việc, loại việc) nhận một lần — cùng
lý do §2 giữ trạng thái chạy theo phạm vi.

### 8. Bản tin đầu tuần

> **Người dùng, 29/09/2026:** bản tin giao **hai phần** — nhiệm vụ và phản ánh từ
> `service-petitions`, văn bản từ `service-documents`. **"Phản ánh nóng"** (`14-cau-hinh.md:332`) =
> phiếu phản ánh **quá hạn** hoặc bị **chấm 1–2 sao trong tuần**.

Hai phần vì mỗi service chỉ đếm sổ của mình (§1, điều kiện dừng #2); một phần gộp cần một bên đọc
sổ của bên kia. "Quá hạn" là **suy ra** từ hạn đã lưu (luật 10 bất biến 3), không phải một cột. Số
sao là `diem_hai_long` của phiếu (`service-petitions/migrations/0017_petition_publication_and_rating_comment.sql`).
Tiêu đề `report.notification.week` vẫn thuộc `reporting` (ADR 0024 §Phụ, *Bổ sung 29/09/2026*).

## Hệ quả

- **Dễ:** không broker, không biến môi trường mới cho broker (ADR 0057). Mỗi service đếm trên CSDL
  của mình, cùng vị từ với danh sách của nó.
- **Khó:** ba vòng lặp giống nhau ở hai service. Phần chung (ticker, khoá, đọc cấu hình) nên nằm ở
  `core/` thay vì chép — việc của lượt dựng.
- **Trả sau:** ngày cần hàng đợi bền thật (gửi hàng loạt, thử lại có hẹn), RabbitMQ của ADR 0010
  vẫn là chỗ đã chọn — nhưng mở lại là **ADR mới**, không lặng lẽ thêm.
- **ADR 0010 vẫn đúng ở chỗ khác:** Kafka *"không dùng cho tác vụ có lịch"* (`0010:37`) vẫn giữ.

## Câu từng mở — đã trả lời 29/09/2026

| # | Câu | Trả lời |
|---|---|---|
| 1 | Bản tin đầu tuần chạy ở service nào | **Người dùng:** hai phần, mỗi service một phần — §8 |
| 2 | "Phản ánh nóng" nghĩa là gì | **Người dùng:** quá hạn hoặc chấm 1–2 sao trong tuần — §8 |
| 4 | Đơn thư có vào nhắc hạn không | **Người dùng:** có, theo kho yêu cầu — §4. Xem câu mở #10 |
| 5 | Ngưỡng "bộ phận giữ mà chưa phân công ai" | **Người dùng:** trường SLA mới theo xã — ADR 0029 §Bổ sung 29/09 |
| 6 | Tuyến chạy ngay đặt ở đâu | **Người dùng:** ở `identity`, chỉ đánh dấu — §7 |
| 7 | Ba hợp đồng và mức nhất quán | **Hợp đồng 1796286** (`contract-designer`), không phải quyết định của người dùng — §2, §3 |
| 8 | Danh sách xã phải quét | **Hợp đồng 1796286**, phương án (b) — §2b |
| 3 | Leo thang áp cho những loại việc nào | **Người dùng, 29/09/2026:** nhiệm vụ · phiếu phản ánh (**chỉ** mốc `han_xu_ly_xong`) · văn bản đến. Nhắc hạn phủ nhiệm vụ, phiếu phản ánh, văn bản đến **và** đơn thư (khớp §4). Mã hiện chạy: §Bổ sung 29/09 dưới |
| 9 | Chủ thể hệ thống của việc nền | **Lượt dựng** (câu cũ ghi "người dùng duyệt" — chưa thấy lời duyệt riêng): `core/audit.SystemActor` (`"system"`, `core/audit/audit.go:35-37`), `Actor{ID: SystemActor, Kind: "system"}` — một vết mỗi lượt ở `service-identity/internal/app/automation.go:420-423`; `audit.go:53-54` từ chối chủ thể rỗng, nên không có fallback. Không thêm `Principal` hệ thống ở `core/authz` |

## Bổ sung 29/09/2026 — mã đang chạy gì

Ghi điều mã làm sau commit `765f32b` (bên chạy `petitions`) và `120df73` (bên chạy `documents`),
không phải quyết định mới.

| Bên chạy | Phạm vi nhận (`automationKinds`) | Căn cứ |
|---|---|---|
| `service-petitions` | `NHIEM_VU` · `PHAN_ANH`. Phiếu phản ánh leo thang theo `han_xu_ly_xong` duy nhất | `service-petitions/internal/app/automation_runner.go:138-141`, `automation_jobs.go:216-218` |
| `service-documents` | **Chỉ** `VAN_BAN_DEN` | `service-documents/internal/app/automation_runner.go:133-135` |

**Nhắc hạn đơn thư CHƯA chạy.** `service-documents` **không có sổ đơn thư**: không bảng `don_thu`,
không cột hạn — các migration chỉ có hai sổ văn bản (`service-documents/migrations/0004_so_van_ban.sql`;
`service-documents/internal/domain/automation.go:17-21`). Phạm vi `DON_THU` **có trong hợp đồng**
nhưng bên chạy **cố ý không nhận**: nhận một lượt thì phải ghi kết quả, và mọi kết quả đều sai
(`SUCCEEDED` nói "không có gì tới hạn" về thư chưa ai xem; `FAILED` mỗi 5 phút là lỗi giả) —
`automation_runner.go:127-132`. Bật lên bằng **một dòng** thêm `WORK_KIND_DON_THU` vào
`automationKinds`, trong cùng thay đổi dựng sổ đơn thư và cột `han_xu_ly_xong` của nó (luật 10 bất
biến 2).

Hộp chuông của §3 dựng dưới tên **`staff_notification`**, không phải `hop_thu_thong_bao` của đặc tả
(`service-comms/migrations/0010_staff_notification.sql:10-12`, luật 12) — cùng một thứ.

⚠ Chú thích tại `service-documents/internal/app/automation_jobs.go:214` còn ghi *"open question #3 …
is still the user's"* — đã cũ sau câu trả lời trên; sửa là việc của lượt dựng, không phải của `kb/`.

## Còn mở — chưa ai quyết

| # | Câu | Ai |
|---|---|---|
| 10 | **Phần đã trả lời (người dùng, 29/09/2026):** đơn thư là loại việc thứ tư — `WORK_KIND_DON_THU = 4` (`proto/vigov/identity/v1/identity.proto:2666`), `sla.loai_viec` nhận `don-thu` (`service-identity/migrations/0016_sla_citizen_letter_kind_and_unassigned_hold.sql:8-15`, `:62`). **Phần còn mở:** sổ đơn thư chưa có ở `service-documents` (ADR 0039), nên chưa có hạn đã lưu để nhắc — §Bổ sung 29/09 | Lượt dựng sổ đơn thư |

## ĐIỀU KIỆN DỪNG

1. Đưa một việc của tab này sang RabbitMQ, Kafka hay CronJob — đảo §Quyết định, cần ADR mới
2. Một việc đọc sổ của service khác để đếm — sai §1, và luật 2 cấm #2
3. Bật mặc định một việc cho xã mới
4. Bất kỳ phép trừ giờ đồng hồ nào cho độ trễ, ngoài `identity`
5. Dựng lại `refresh_dashboards`, hoặc dựng `send_scheduled_reports` trước khi có `/bao-cao`
6. Gắn tự đóng phiếu `cho-dan-xac-nhan` vào công tắc của tab này

→ Hợp đồng: `proto/vigov/identity/v1/identity.proto` (`ClaimDueAutomationRuns`,
`RecordAutomationRunOutcome`, `ResolveEscalationInstants`, `ResolveOrgUnitPermissionHolders`,
`ResolveLeadershipStaff`) · `proto/vigov/comms/v1/comms.proto` (`DeliverStaffNotifications`)
→ ADR 0007 (giờ làm việc) · 0010 (hạ tầng dữ liệu) · 0012 (`x-tenant-id`) · 0029 (bảng `sla`, mốc leo thang) · 0041
(bảng tin báo công dân) · 0053 (không snapshot) · 0057 (nạp cấu hình theo nhóm)
→ Luật 1 · 2 · 5 · 6 · 10
→ Đặc tả: `docs/ui-ux/14-cau-hinh.md` §9, §11
