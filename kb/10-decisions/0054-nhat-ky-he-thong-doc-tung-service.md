---
id: 0054-nhat-ky-he-thong-doc-tung-service
tier: T1
source: CURATED
owner: architecture
derived_from_commit: f27fd6e
expires: null
owns_facts:
  - "màn Xem nhật ký hệ thống (admin.audit) đọc audit_log ở TỪNG service sở hữu, web-admin ghép; không kho trung tâm, không đọc chéo CSDL, không read model ở reporting — và cái giá"
  - "năm service được đọc ở đợt 1 (comms, documents, finance, identity, petitions); platform và reporting để sau, platform_audit_log và operator_audit_log ở ngoài hẳn"
  - "vì sao danh từ URL là <service>-audit-entries chứ không một audit-entries chung"
  - "tham số lọc, thứ tự và trường trả về của tuyến đọc nhật ký hệ thống; actor_ip và mã công dân không trả cho mục của công dân"
  - "đọc nhật ký hệ thống tự ghi một mục xem_nhat_ky_he_thong mỗi lần gọi, và mục ấy bị ẩn khỏi mặc định"
  - "cách web-admin ghép nhiều trang theo thời gian và hiện lỗi từng service khi một service không trả lời"
---

# 0054. Xem nhật ký hệ thống: đọc ở từng service sở hữu `audit_log`, web-admin ghép

**Trạng thái:** đã chốt (người dùng, **28/09/2026**, theo đề xuất: *"FAN-OUT theo khuôn ADR
0053"*) · mọi dòng ghi **"nhà cung cấp chọn"** là **chưa được khách chốt**, chọn để đổi được
khi chưa xã nào chạy thật · Phạm vi: menu `/cau-hinh`, quy tắc `docs/ui-ux/14-cau-hinh.md:387`
(*"Mọi thay đổi ở tab này ghi vào nhật ký hệ thống (quyền `admin.audit` để xem)"*).

## Bối cảnh

| Sự thật | Nơi |
|---|---|
| Khoá `admin.audit` · *"Xem nhật ký hệ thống"* đã gieo | `service-identity/migrations/0001_init.sql:280` |
| **Chưa tuyến nào kiểm khoá ấy**; menu cố ý không mở vì nó | `web-admin/src/components/muc-menu.ts:49-51`, `muc-menu.test.ts:154` |
| Vết ghi bằng thư viện, **trong giao dịch** của service ghi | `core/audit/audit.go:3-12,68` — lý do: ADR 0001 |
| Nên có **bảy** bảng `audit_log`, mỗi service một bảng, cùng cột | `service-{comms,documents,finance,identity,petitions,platform,reporting}/migrations/0001_init.sql` |
| Bảng chỉ có `actor_id · actor_kind · actor_ip · action · subject · at · delta` — **không có cột loại đối tượng** | `service-petitions/migrations/0001_init.sql:17-24` |
| Chỉ mục duy nhất `(tenant_id, subject, at DESC)` | `service-petitions/migrations/0001_init.sql:43-44` (các service khác cùng dạng) |
| Kho yêu cầu có **một** tuyến `GET /api/v1/admin/activity-logs` | `../vigov-require/docs/spec/04-api.md:27` |

Kho yêu cầu là **một khối, một bảng**, nên một tuyến là đủ. v2 có bảy bảng vì vết **phải** cùng
giao dịch với thay đổi (luật 6 bất biến 3) mà hai service không chung được giao dịch
(`core/audit/audit.go:6-11`). Một tuyến duy nhất ở v2 đòi hoặc kho vết trung tâm, hoặc đọc chéo
CSDL — cả hai bị bác dưới đây.

## Quyết định

### 1. Mỗi service tự đọc sổ của mình, web-admin ghép

> Người dùng, 28/09/2026: mỗi service sở hữu `audit_log` mở **một tuyến GET có phân trang** cho
> cán bộ, sau `RequirePermission("admin.audit")`, phạm vi **xã của yêu cầu**; web-admin ghép trên
> màn. Không kho trung tâm, không đọc chéo CSDL, không read model ở `reporting`.

Cùng khuôn ADR 0053 §1: số liệu đọc ở nơi nó được ghi.

| Phương án bị bác | Vì sao |
|---|---|
| Kho vết trung tâm (một service `audit`) | Phá chính lý do `core/audit` là thư viện: ghi sang service khác là ghi **ngoài giao dịch** (luật 6 cấm #2) |
| Một service đọc `audit_log` của các service khác | Luật 2 bất biến 2, cấm #2 |
| `reporting` dựng bản chiếu từ sự kiện | Chưa service nào phát sự kiện vết; bản chiếu là **bản sao thứ hai** của một sổ append-only, có độ trễ — mục vừa ghi chưa hiện là đúng lúc thanh tra hỏi |

**Cái giá:**

| Điều | Nội dung |
|---|---|
| Năm tuyến, năm bộ kiểm | Mỗi tuyến đủ bốn ca luật 5 bất biến 7 (401 · 403 sai khoá · 403 đúng khoá sai xã · 200) |
| Chỉ mục mới | Chỉ mục hiện có bắt đầu bằng `subject`; lọc theo thời gian không dùng được nó. Mỗi service cần thêm `(tenant_id, at DESC, id DESC)` — migration **thêm**, không đụng dòng cũ |
| Ghép ở web | Thứ tự đúng giữa năm nguồn là việc của web (§6), không phải của CSDL |

### 2. Đợt 1 đọc năm service — đúng năm service mà proxy web định tuyến

Proxy web chỉ biết `comms · documents · finance · identity · petitions`
(`web-admin/src/lib/api/dinh-tuyen.gen.ts:7`). Một tuyến ở service ngoài danh sách ấy là tuyến web
không gọi tới được.

| Bảng | Đợt 1 | Vì sao |
|---|---|---|
| `audit_log` của 5 service trên | **Có** | Nơi mọi thao tác nghiệp vụ của xã đang ghi vết |
| `audit_log` của `platform` | **Để sau** | Hôm nay **không mã sản xuất nào ghi** vào đó (dòng `INSERT` duy nhất là ca thử `service-platform/internal/store/directory_pg_test.go:301`), và proxy web không định tuyến `platform`. ADR 0048 §*Thiết kế* #6 (giữ nguyên ở `:153`) dự định ghi ở đây **thao tác vận hành nhắm một xã** (`tenant_id` = xã đích). Có cho quản trị xã xem thao tác ViHAT làm trên xã mình không là câu hỏi cho chủ dự án — xem §*Còn mở* |
| `audit_log` của `reporting` | **Để sau** | Không mã nào ghi; `reporting` chưa có thao tác ghi nghiệp vụ nào |
| `platform_audit_log` (`service-platform/migrations/0008_upload_policy.sql`) | **Ngoài hẳn** | Không `tenant_id` — thay đổi **toàn nền tảng** (ADR 0048). Không phải sổ của một xã |
| `operator_audit_log` (`service-identity/migrations/0012_operator_accounts.sql`) | **Ngoài hẳn** | Vết miền vận hành ViHAT, không `tenant_id` (ADR 0048). Cán bộ xã không có quyền trên miền ấy |

Chiều ngược lại đã có chủ: ViHAT **không** đọc nhật ký thao tác nghiệp vụ của xã (ADR 0003:32).
Nên năm tuyến này **chỉ** mở cho khoá `admin.audit` của xã, không có biến thể cho người vận hành.

⚠ **Hai nghĩa của "nhật ký hệ thống".** ADR 0003:32 dùng chữ ấy cho nhật ký kỹ thuật của nhà cung
cấp; `docs/ui-ux/14-cau-hinh.md:108,387` dùng cho **vết thao tác của xã**. ADR này theo nghĩa thứ hai.

### 3. Danh từ URL: `<service>-audit-entries` — nhà cung cấp chọn

`kb/00-foundation/ubiquitous-language.md` §*Tên tài nguyên trên URL* chưa có dòng cho khái niệm này.
ADR 0011 bảo hỏi; người dùng bảo theo đề xuất, nên đây là **nhà cung cấp chọn** và dòng ở bảng ấy
ghi như vậy.

| Service | Tuyến |
|---|---|
| `comms` | `GET /api/v1/comms-audit-entries` |
| `documents` | `GET /api/v1/documents-audit-entries` |
| `finance` | `GET /api/v1/finance-audit-entries` |
| `identity` | `GET /api/v1/identity-audit-entries` |
| `petitions` | `GET /api/v1/petitions-audit-entries` |

**Vì sao không một `audit-entries` chung.** `tools/ingress` gom theo **đoạn đầu** sau `/api/v1/` và
**dừng** khi hai service cùng nhận một đoạn (`tools/ingress/dinhtuyen.go:159-184`): một tiền tố
Ingress chỉ trỏ được một backend. Năm service cùng tên `audit-entries` là năm chủ cho một tiền tố.

**Vì sao `audit-entries`.** Trùng tên kiểu `audit.Entry` (`core/audit/audit.go:40`) và bảng
`audit_log`. Không `activity-logs` như kho yêu cầu: *activity* đọc thành nhật ký kỹ thuật, còn đây
là **dữ liệu nghiệp vụ** giữ ít nhất 12 tháng (luật 6). Không `log-entries`: danh từ ấy đã là nhật
ký & trao đổi của nhiệm vụ và phiếu phản ánh (bảng ngôn ngữ chung).

**Cái giá của tiền tố tên service:** tách `petitions` thành hai service về sau thì một tuyến mang
tên sai. Chấp nhận — đổi được miễn phí khi chưa xã nào chạy thật.

### 4. Tham số, thứ tự, trường trả về

**Tham số** (tất cả tuỳ chọn; lọc là **khớp đúng**, không tìm gần đúng):

| Tham số | Nghĩa |
|---|---|
| `from`, `to` | RFC 3339, nửa mở `[from, to)` trên `at`. `from >= to` → 400. Client tính kỳ, như ADR 0053 §3 |
| `actor` | Mã cán bộ (`CB-…`) hoặc `system` |
| `action` | Một động từ nghiệp vụ (`khoa_tai_khoan_can_bo`) |
| `subject` | Một mã nghiệp vụ (mã tra cứu, số văn bản, mã cán bộ…) — dùng được chỉ mục đang có |
| `limit`, `cursor` | `core/page`: mặc định 20, trần 100, con trỏ mờ không mang `tenant_id` (`core/page/page.go:15-29,45-54`) |

**Không có lọc theo loại đối tượng.** Bảng không có cột ấy. Thêm cột thì mọi dòng cũ để trống mãi,
vì trigger append-only (`0002_audit_log_append_only.sql` ở mỗi service) từ chối mọi `UPDATE` bù lại
— mà đó là đúng. Service được gọi và động từ (`them_du_an`, `sua_van_ban_den`) đã nói đối tượng là gì.

**Thứ tự:** chỉ một: `at DESC`, phá hoà bằng `id DESC`. Không `sort` nào khác — sổ vết đọc theo
thời gian.

**Trường trả về** (bao trong `items · next_cursor · has_more` của `core/page`, không `total`):

| Trường | Nội dung | Vì sao |
|---|---|---|
| `at` | Thời điểm | |
| `actor_kind` | `staff` · `citizen` · `system` | |
| `actor_code` | Mã cán bộ `CB-…` hoặc `system`. **Rỗng khi `actor_kind = citizen`** | Luật 6 bất biến 8: "ai" là mã nghiệp vụ. Công dân không có mã nghiệp vụ — cột giữ id nội bộ (`service-identity/internal/app/cau_phien_cong_dan.go:383-392`), một chuỗi không nói tên ai với cán bộ nhưng **nối được** các lần hành động của một người dân. Nhà cung cấp chọn |
| `actor_ip` | IP. **Rỗng khi `actor_kind = citizen`** | Luật 6 bất biến 2 bắt ghi IP để quy trách nhiệm **cán bộ** — người đọc sổ cần thấy nó. IP của người dân là dữ liệu gắn với hoạt động của một cá nhân trên mạng (Nghị định 13/2023); hiện nó cho vai trò `admin.audit` là **cho một vai trò mới xem dữ liệu cá nhân** — điều kiện dừng #1 luật 3, chưa ai hỏi khách. Nhà cung cấp chọn **không trả**, đổi được bằng cách thêm, không phá ai |
| `action` | Động từ nghiệp vụ, **nguyên văn** | Đợt 1 không có bảng nhãn — xem §*Còn mở* |
| `subject` | Mã nghiệp vụ, nguyên văn | |
| `delta` | JSON **đúng như đã lưu** | Đã che lúc ghi (luật 6 bất biến 5; ví dụ `service-petitions/internal/app/gui_phan_anh.go:365-386`). Tuyến đọc **không che lại**: nó không biết cấu trúc của từng động từ. Tìm thấy một `delta` chứa dữ liệu cá nhân chưa che là **sự cố luật 3 ở chỗ ghi**, sửa ở chỗ ghi — không vá ở tuyến đọc |

Không trả `id` của dòng: con trỏ đã mang nó, màn không cần.

**Lĩnh vực hạn chế ở `petitions` — nhà cung cấp chọn.** Mục có `subject` là mã tra cứu của phiếu
thuộc lĩnh vực `can-bo` **chỉ** hiện cho người giữ thêm `feedback.restricted` (ADR 0030). Không thế
thì `admin.audit` thành lối thứ hai biết một tố cáo cán bộ tồn tại và ai đã động vào nó — đúng thứ
ADR 0030 khoá. Loại bằng cùng hằng mà danh sách phiếu dùng (ADR 0053 §2).

### 5. Đọc nhật ký cũng ghi vết — một mục mỗi lần gọi

**Luật 6 bất biến 7 không bắt buộc việc này**, và ghi rõ để không ai dẫn nhầm: bất biến ấy nói đọc
**dữ liệu cá nhân đầy đủ** hoặc đọc **xuyên xã**. Tuyến này đọc dữ liệu đã che, trong một xã.

**Nhà cung cấp chọn ghi, vì:** sổ vết mang IP và toàn bộ nhịp làm việc của từng cán bộ. Khi có
khiếu nại, câu *"ai đã mở sổ vết trước đó"* sẽ được hỏi, và không có mục nào thì không trả lời được.

| Điều | Quyết định |
|---|---|
| Động từ | `xem_nhat_ky_he_thong` — cùng dạng `xem_day_du_nguoi_gui` (`service-petitions/internal/app/xem_nguoi_gui.go:44`) |
| Ở đâu | `audit_log` của **chính service được đọc**, cùng giao dịch với câu đọc. Một màn tải năm service → năm mục |
| `subject` | Mã cán bộ người đọc — cùng khuôn `dang_nhap` (`service-identity/internal/app/dang_nhap.go:169-173`) |
| `delta` | Tóm tắt bộ lọc: `from`, `to`, `actor`, `action`, `subject`, có con trỏ hay không, số mục trả về. Không chép mục nào đã trả |
| Ghi vết lỗi | Cả yêu cầu lỗi — không có lượt đọc sổ vết nào không để lại vết |
| Không đệ quy | **Một** mục mỗi yêu cầu, không phải mỗi dòng đọc được — đọc 100 dòng sinh 1 mục |
| Mặc định | **Ẩn** mục `xem_nhat_ky_he_thong` khi không truyền `action`. Truyền `action=xem_nhat_ky_he_thong` thì hiện. Không ẩn thì mỗi lần mở màn đẩy chính mình lên đầu, và sổ đầy lượt xem thay vì lượt làm |

### 6. Web-admin ghép

| Điều | Quyết định |
|---|---|
| Thứ tự | `at` giảm dần trên cả năm nguồn |
| Con trỏ | **Một con trỏ cho mỗi service**; web giữ năm con trỏ, không có con trỏ gộp |
| Ranh giới an toàn | Chỉ hiện các mục **không cũ hơn** mục cuối đã tải của mọi service còn `has_more`. Mục cũ hơn ranh giới giữ lại, chờ trang sau — hiện ngay là sai thứ tự, vì service kia có thể còn mục mới hơn chưa tải |
| "Tải thêm" | Gọi trang kế của service đang giữ ranh giới |
| Mỗi dòng | Ghi phân hệ nguồn |
| Tên cán bộ | Hiện mã; tra tên qua `staff-directory` được thì kèm tên. Người đã khoá tài khoản không có trong danh bạ ấy — khi đó **chỉ mã**, không để trống |
| Menu | Thêm `admin.audit` vào `KHOA_MO_CAU_HINH` cùng lượt dựng tab, và **sửa có chủ ý** ca `muc-menu.test.ts:154` |

### 7. Một service không trả lời

Cùng lý lẽ ADR 0053 §6 (ô không có nguồn hiện *"Chưa có dữ liệu nguồn"*, không bao giờ 0 — vì 0
là một khẳng định về cơ quan): hiện dữ liệu của các service còn lại **và** một dòng lỗi **nhìn
thấy được cho từng service lỗi** (*"Không tải được nhật ký của phân hệ …"*). **Không bao giờ ghép im lặng
một phần** — sổ thiếu một phân hệ mà trông như đủ là sổ nói dối người thanh tra. Service lỗi ra
khỏi phép tính ranh giới §6 cho tới khi tải lại được.

### 8. Thời hạn lưu

Ít nhất **12 tháng**, không xoá, không sửa (luật 6). Tuyến này chỉ đọc; không có TTL, không có
dọn dẹp. **Câu mở #35 (cấp độ TCVN 14423) còn OPEN nhưng không chặn việc này:** nó quyết thời hạn
lưu **nhật ký an ninh** (3 hay 6 tháng), còn `audit_log` là dữ liệu nghiệp vụ giữ 12 tháng trở
lên — dài hơn cả hai lựa chọn của #35.

## Còn mở — chưa ai quyết

| # | Việc | Của ai |
|---|---|---|
| 1 | Quản trị xã có được xem **thao tác ViHAT trên xã mình** (`platform.audit_log`, ADR 0048 §*Thiết kế* #6) không — cần mở `platform` trên proxy web | Chủ dự án |
| 2 | Hiện `actor_ip` và mã công dân cho mục của công dân (§4) | Khách — điều kiện dừng #1 luật 3 |
| 3 | Bảng nhãn tiếng Việt cho động từ (§4). Chép tay ở web là bản sao thứ hai của hằng Go, sẽ lệch; nguồn đúng là mỗi service tự trả | Phiên dựng màn |
| 4 | Năm danh từ `<service>-audit-entries` (§3) | Khách, nếu muốn đổi — trước khi xã đầu tiên chạy |

## ĐIỀU KIỆN DỪNG

1. Một service đọc `audit_log` của service khác, hay một kho vết trung tâm — là đảo §1, cần ADR mới
2. Mở tuyến này cho người vận hành ViHAT — trái ADR 0003:32
3. Đưa `platform_audit_log` hoặc `operator_audit_log` vào màn của xã
4. Tuyến đọc sửa, xoá hoặc che lại nội dung một mục đã lưu
5. Một lượt đọc không để lại mục `xem_nhat_ky_he_thong`
6. Ghép im lặng khi một service lỗi

→ ADR 0001 (vì sao vết là thư viện) · 0003 (nhà cung cấp chỉ chạm siêu dữ liệu) · 0011 (ngôn ngữ
bề mặt) · 0030 (`feedback.restricted`) · 0048 (miền vận hành, hai bảng vết không xã) · 0053 (khuôn
đọc ở service sở hữu)
→ Luật 1 · 2 · 3 · 5 · 6
