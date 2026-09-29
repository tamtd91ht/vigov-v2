---
id: 0029-so-huu-bang-sla
tier: T1
source: CURATED
owner: architecture
derived_from_commit: 1796286
expires: null
owns_facts:
  - "bảng sla thuộc service identity, đặt cạnh ba bảng lịch làm việc"
  - "vì sao bảng sla không thuộc petitions cũng không thuộc documents"
  - "vì sao một bảng cấu hình theo xã lại không phải danh mục tham chiếu của ADR 0024"
  - "cái giá của việc identity giữ thêm một khái niệm không dính tới con người"
  - "hệ quả: bảng sla chưa tồn tại và đang chặn mọi tuyến ghi của petitions lẫn documents"
  - "RPC đọc bảng sla trả MỐC HẠN đã cộng xong chứ không trả số giờ — chốt 22/09/2026"
  - "bộ lọc sắp đến hạn đọc gio_sap_den_han qua một MỐC CUỐI đi xuôi từ now, không qua số giờ — 28/09/2026"
  - "leo thang dùng hai ngưỡng gio_bao_lanh_dao và gio_bao_chu_tich tính bằng giờ làm việc, không nhân đôi một số — 29/09/2026"
  - "hai ngưỡng leo thang đếm từ hạn đã lỡ, buộc Y ≥ X lúc ghi — người dùng 29/09/2026"
  - "mức báo Chủ tịch gửi mọi vai trò la_lanh_dao, gồm Phó Chủ tịch; không đổi lược đồ — người dùng 29/09/2026"
  - "ngưỡng bộ phận giữ mà chưa phân công là trường SLA mới theo xã, giờ làm việc, NULL = không báo, gieo 8 — người dùng 29/09/2026"
  - "chú thích anchor UNDECIDED của migration 0008 đã cũ và được ghi đè bằng migration mới"
---

# 0029. Bảng `sla` thuộc `identity` — đặt cạnh lịch làm việc

**Trạng thái:** đã chốt · **Ngày:** 2026-09-20 · **Nối tiếp ADR 0007 · 0024** · **Cùng lập luận với ba bảng lịch**

## Bối cảnh

Bảng `sla` của bản mẫu (`docs/ui-ux/14-cau-hinh.md:316`) giữ số giờ cho **ba loại việc**:

```
sla(id, loai_viec enum('van-ban-den','phan-anh','nhiem-vu'), linh_vuc text null,
    gio_tiep_nhan, gio_xu_ly_xong, gio_sap_den_han, gio_bao_lanh_dao, gio_bao_chu_tich)
```

`van-ban-den` là việc của `documents`; `phan-anh` và `nhiem-vu` là việc của `petitions`. **Hai
service khác nhau cùng phải đọc một bảng**, nên đây là điều kiện dừng #1 của luật 2 — không
phải một lựa chọn kỹ thuật mà người viết migration được tự quyết.

Bảng này **chưa tồn tại ở đâu trong kho**, và sự vắng mặt của nó đang được ghi lại đúng chỗ nó
gây hại: `service-petitions/internal/http/routes.go:225-231` nêu thẳng rằng ba tuyến ghi của
`docs/ui-ux/09` §13 không viết được vì *"A TABLE THAT EXISTS IN NO SERVICE"*.

## Quyết định

**Bảng `sla` nằm trong `service-identity`, cạnh `lich_lam_viec` · `ngay_nghi_le` · `ngay_lam_bu`.**

Ba căn cứ, theo thứ tự sức nặng:

| # | Căn cứ |
|---|---|
| 1 | **Không thuộc service nào trong hai bên đọc.** Đặt ở `petitions` thì `documents` phải gọi sang một service tiếp dân để hỏi hạn của sổ văn thư; đặt ở `documents` thì ngược lại. Cả hai chiều đều dựng một phụ thuộc mà ranh giới hành chính ngoài đời không có |
| 2 | **Quyền sở hữu đi theo NHỊP ĐỔI** — ADR 0024. Số giờ SLA đổi khi **chính sách hành chính của xã** đổi, đúng cùng nhịp với giờ làm việc và ngày nghỉ. Hai thứ đổi cùng nhau thì ở cùng chỗ (`domain-boundaries.md` §Nguyên tắc cắt ranh giới #3) |
| 3 | **Đây là cùng một lập luận đã dùng cho ba bảng lịch**, không phải một lập luận mới bịa cho ca này. Ba bảng ấy cũng được ít nhất hai service đọc, cũng không thuộc service nào trong hai, và người dùng đã chốt chúng thuộc `identity` ngày 2026-09-20 (`service-identity/migrations/0006_lich_lam_viec.sql:14-20`) |

### Lợi thêm, không phải lý do chính: một chặng thay vì hai

Tính một hạn cần **hai** thứ: *bao lâu* (số giờ, bảng `sla`) và *lúc nào* (giờ làm việc của xã).
Đặt chúng cùng một service thì một lần tính hạn là **MỘT** chặng gRPC. Tách ra là hai chặng, và
hai chặng cho một phép tính nghĩa là hai lần hỏng độc lập trên **đường ghi** của một phiếu —
đúng chỗ luật 2 bất biến 6 bắt khai ranh giới giao dịch.

Ghi đây là **lợi thêm** chứ không phải căn cứ, có chủ ý: nếu số chặng gRPC là lý do chính thì
lập luận này cũng sẽ biện hộ được cho việc gom mọi bảng vào một service.

### Một bằng chứng phụ đã có sẵn trong bảng phân quyền

Khoá `admin.sla` — *"Cấu hình thời hạn xử lý"* — đã nằm trong 33 khoá nạp từ đầu
(`service-identity/migrations/0001_init.sql`). Tức **cấu hình thời hạn đã được hiểu là MỘT
việc, của MỘT người**, từ trước khi có câu hỏi này. Một bảng chẻ đôi theo service sẽ có một
khoá quyền mà không có một chỗ để thực thi.

## VÌ SAO NÓ KHÔNG PHẢI DANH MỤC THAM CHIẾU — đọc trước khi áp ADR 0024 vào đây

Rất dễ đọc `sla` như nhóm danh mục thứ mười một rồi áp §Ba ô để trống. **Không phải**, và chỗ
khác nhau có hệ quả thật:

| | Danh mục tham chiếu (ADR 0024) | `sla` |
|---|---|---|
| Một dòng là | một **lựa chọn** hiện ra ở ô chọn | một **con số ràng buộc** phần mềm phải tính theo |
| Xã sửa một dòng thì | màn hình có thêm/bớt một mục | **một cam kết với dân đổi** |
| Người sửa | quản trị viên, `admin.lookup` | `admin.sla` — khoá khác, cố ý |

Nhưng **phép thử của ADR 0024 vẫn phải chạy**, và nó cho kết quả rõ: *xã thêm một dòng vào đây
thì phần mềm có cần nhánh mã mới không?* — **Không**. Vậy đây là **dữ liệu của xã**, và bốn hệ
quả của ADR 0024 §"Vì sao MỌI bảng ở đây mang `tenant_id`" áp nguyên văn:

| # | |
|---|---|
| 1 | Mang `tenant_id`. Khoá duy nhất **hợp thành** — `(tenant_id, loai_viec, linh_vuc)`, với `linh_vuc IS NULL` là dòng mặc định và phải có một chỉ mục duy nhất **riêng** cho ca `NULL` (PostgreSQL không coi hai `NULL` là trùng nhau) |
| 2 | Hạt giống là **gieo cho từng xã**, không phải dòng dùng chung toàn nền tảng |
| 3 | Phân mảnh theo `tenant_id` (ADR 0004) |
| 4 | Xoá là xoá mềm (luật 7) |

**`linh_vuc` giữ dưới dạng GIÁ TRỊ, không khoá ngoại.** Mã lĩnh vực là bộ mã tầng nền tảng ở
service `platform` (ADR 0026), nên một khoá ngoại ở đây là khoá ngoại xuyên service — luật 2
cấm. Cạm bẫy đã có bằng chứng sẵn trong chính bản mẫu: `14-cau-hinh.md:308` còn một dòng SLA
trỏ tới mã `ve-sinh-moi-truong` **không còn trong danh mục**, hiện ra màn hình dưới dạng mã
thô. Chỗ này cần **phép kiểm lúc GHI**, không phải ràng buộc lúc đọc — đúng kết luận ADR 0024
§"Cái giá của dòng Khối nhiệm vụ".

## CÁI GIÁ, NÓI THẲNG

**`identity` phình thêm một khái niệm không dính gì tới con người.** Service này đã giữ: tổ
chức, cán bộ, định danh công dân toàn nền tảng, đơn vị dân cư, hai danh mục treo vào bộ máy, ba
bảng lịch — và nay là số giờ cam kết xử lý. Câu *"identity là service cấu hình"* chính là
phương án **#3 mà ADR 0024 đã bác**, và mỗi lần thêm một bảng vào đây là một bước xích lại gần
cái phương án ấy.

Ghi ra vì một ranh giới nới ra trong im lặng là ranh giới không còn ai kiểm được.

**Phép thử cho lần sau — dùng nó thay vì lặp lại quyết định này:** một bảng chỉ được vào
`identity` theo đường này khi nó thoả **cả ba**: (a) ít nhất hai service đọc, (b) không thuộc
service nào trong số đó, (c) đổi cùng nhịp với **bộ máy hành chính của xã**. Thiếu (c) thì nó
không phải ca này — nó chỉ là một bảng người ta chưa biết đặt đâu, và chỗ đúng của một bảng như
thế là một câu hỏi, không phải `identity`.

## HỆ QUẢ NGAY — thứ đang bị chặn

| Bị chặn | Bằng chứng |
|---|---|
| **Mọi tuyến ghi của `petitions`** — nhập hộ, phân loại, công dân gửi | `service-petitions/internal/http/routes.go:219-259` |
| **Đường tính hạn của `documents`** — `van-ban-den` lấy số giờ từ chính bảng này | `14-cau-hinh.md:297`, `kb/10-decisions/0007-sla-working-hours.md` |

**Cần một RPC để hai service ấy đọc số giờ. HÌNH DẠNG RPC CHƯA CÓ AI QUYẾT.** Đó là việc của
`contract-designer` và **chưa làm**. Tiền lệ đã có ở cùng service: `AdvanceWorkingHours`
(`proto/vigov/identity/v1/identity.proto`) — nhưng nó trả lời *"lúc nào"*, không trả lời *"bao
lâu"*, nên nó là **tiền lệ về hình dạng, không phải chỗ để nhét thêm trường**.

Câu chưa ai hỏi lúc viết ADR này: một lời gọi trả **số giờ** rồi bên gọi tự cộng, hay một lời
gọi trả thẳng **mốc hạn** đã cộng xong. Hai đường cho ra hai chỗ đặt phép tính giờ làm việc, và
ADR 0007 đã chốt là *không được có hai bản cài đặt của phép tính ấy*.

→ **ĐÃ TRẢ LỜI 22/09/2026 — xem §Bổ sung ở cuối tệp.** Đoạn trên giữ nguyên câu hỏi vì nó là thứ
giải thích vì sao hình dạng hôm nay là hình dạng ấy.

## ĐIỀU KIỆN DỪNG

1. **Một giá trị `loai_viec` thứ tư.** Enum hôm nay có đúng ba, và thêm một giá trị là thêm một
   miền nghiệp vụ vào bảng — hỏi trước khi thêm
2. **Viết bảng `sla` ở `petitions` hoặc `documents`** — kể cả "tạm thời, để chạy được tuyến
   này". Một bảng tạm có dữ liệu thật thì không còn tạm
3. ~~**Viết đường đọc mà chưa có hợp đồng chốt hình dạng RPC**~~ — **ĐÃ ĐÓNG 22/09/2026**, xem
   §Bổ sung. Giữ lại chứ không xoá: nó là tiền lệ cho lần sau, và nó **không** kéo theo điều kiện
   dừng #2 của ADR 0026 (đường GHI bảng `sla` vẫn chặn vì mã lĩnh vực phải kiểm lúc ghi)
4. **Ai được SỬA cấu hình SLA của xã** — `admin.sla` đã trả lời *ai bấm được nút*, nhưng chưa ai
   hỏi khách việc đổi số giờ có cần duyệt của lãnh đạo không. Đổi một dòng ở đây là đổi cam kết
   của cơ quan với dân, và nó **không hồi tố** (luật 10 bất biến 2, ADR 0007)

## BỔ SUNG 22/09/2026 — RPC trả **MỐC HẠN**, không trả số giờ

**Người dùng chốt.** `IdentityService.ResolveDeadlines` nhận (loại việc, lĩnh vực, gốc đếm) và
trả về **mốc hạn đã cộng xong giờ làm việc**.

**Lập luận đã nêu và được chấp nhận:** `identity` giữ **toàn bộ dữ liệu của phép tính** — bảng
`sla` (*bao lâu*) và ba bảng lịch + `AdvanceWorkingHours` (*lúc nào*). Trả số giờ thì `petitions`
**và** `documents` mỗi bên phải tự cộng giờ làm việc, và ADR 0007 cấm hai bản cài đặt của phép
cộng ấy. Hai bản sẽ lệch — và chúng lệch đúng ở **đêm, cuối tuần, `ngay_nghi_le` và
`ngay_lam_bu`**, tức đúng những ngày người dân nhận ra.

| | |
|---|---|
| Hợp đồng | `proto/vigov/identity/v1/identity.proto` — `rpc ResolveDeadlines`, `enum WorkKind`, `enum DeadlineKind`, `message Deadline` |
| Máy chủ | `service-identity/internal/grpc/sla.go` — tra `sla` rồi gọi **chính** `Server.tienGioLamViec`, đường dùng chung với `AdvanceWorkingHours` |
| Bọc | `core/identityclient` — `Client.HanXuLy` |

### Ba điều đi kèm quyết định, không tách rời

| # | |
|---|---|
| 1 | **`AdvanceWorkingHours` KHÔNG bị thay.** Nó vẫn phục vụ bên gọi **tự giữ số giờ** và không đọc `sla`. Hai rpc, **một** phép cộng |
| 2 | **Bên gọi phải NÓI RÕ đang ấn định đồng hồ nào** (`deadlines` bắt buộc, không có mặc định "trả cả hai"). Trả cả hai thì kênh công dân nhận `han_xu_ly_xong` tính từ **dòng mặc định** — đúng cái trần 56 giờ ADR 0028 vừa tháo, và là điều kiện dừng #1 của ADR ấy |
| 3 | **Xã chưa cấu hình → `FAILED_PRECONDITION`, không bao giờ một con số.** Xem mục dưới |

### Xã chưa cấu hình: TỪ CHỐI là câu trả lời

Ba ca đều là `FAILED_PRECONDITION` — *"mở màn hình cấu hình của xã"*:

- xã không có dòng `sla` nào;
- loại việc được hỏi không có dòng nào dùng được;
- lĩnh vực được hỏi không có dòng riêng **và** loại việc ấy cũng không có dòng mặc định để rơi về.

**Không có mặc định, và không được có.** Không phải 24 giờ, cũng không phải 16 dòng của
`docs/ui-ux/14-cau-hinh.md` §8 — những dòng ấy là bản mẫu của **một** xã, và một cam kết do phần
mềm bịa ra vẫn đến tai người dân như thể **cơ quan** đã nói (luật 10 cấm #3). Bên gọi nhận lỗi thì
**hỏng lượt tiếp nhận**: không ghi dòng, không cấp mã tra cứu
(`kb/30-indexes/transaction-boundaries.json`, `tinh_han_xu_ly_luc_tiep_nhan`).

**Hôm nay đó là câu trả lời của MỌI xã** — migration 0008 cố ý không gieo dòng nào và bước cấu
hình chưa tồn tại. Nghĩa là thứ đang chặn tuyến ghi của `petitions` và `documents` **đã chuyển từ
HỢP ĐỒNG sang DỮ LIỆU**: cần một đường ghi bảng `sla` (vẫn vướng điều kiện dừng #2 của ADR 0026) và
một màn hình để xã điền. Đó là tiến bộ thật, nhưng **không** phải "đã xong".

### Ba cột không ra khỏi service, và không phải vì quên

`gio_sap_den_han` cần phép đếm **ngược** từ mốc hạn — phép tính ấy chưa tồn tại và chưa ai đặc tả.
`gio_bao_lanh_dao` và `gio_bao_chu_tich` **chưa có mốc neo**: đặc tả nói hai kiểu mâu thuẫn nhau
(§8 ghi "sau 24 giờ" không nói sau cái gì; §9 đếm từ lúc trễ hạn và **nhân đôi** cho chủ tịch thay
vì đọc cột thứ hai). Tính leo thang từ chúng là báo cho lãnh đạo một xã trên một căn cứ **không ai
chọn**, và bản tin không nói được nó dùng căn cứ nào.

→ **Hai cột báo lãnh đạo: ĐÃ TRẢ LỜI 29/09/2026 — xem §Bổ sung 29/09 ở cuối tệp.** Đoạn trên giữ
nguyên vì nó giải thích vì sao câu trả lời phải chọn giữa §8 và §9.

## BỔ SUNG 28/09/2026 — `gio_sap_den_han` ra khỏi service dưới dạng **MỐC CUỐI**, vẫn không phải số giờ

**Người dùng chốt làm** bộ lọc "Sắp đến hạn" của sổ nhiệm vụ (`soon=true`). **Hình dạng do
`contract-designer` chọn** và cần người dùng xác nhận: yêu cầu ban đầu ghi *"trả số giờ"*, nhưng
trả số giờ là đi ngược §Bổ sung 22/09 ngay trên — bên gọi cầm 72 thì phép cộng duy nhất trong tầm
tay là giờ đồng hồ (luật 10 cấm #2).

**`IdentityService.ResolveDueSoonCutoff`** nhận (loại việc, lĩnh vực, `as_of` = `now` của bên gọi) và
trả **một mốc**: mốc **MUỘN NHẤT** mà tính từ `as_of` đã trôi qua **tối đa N giờ làm việc**, N là
`gio_sap_den_han` của xã. Bên gọi lọc `as_of < hạn ≤ mốc` bằng một phép so trong SQL.

| Câu §"Ba cột không ra khỏi service" nói | Còn đúng không |
|---|---|
| `gio_sap_den_han` cần phép đếm **ngược** từ mốc hạn | **Đúng cho một MỐC RIÊNG TỪNG PHIẾU** (lúc nào rung chuông cho một việc) — vẫn chưa có, vẫn chưa ai đặc tả |
| … nên không ra khỏi service | **Không cần cho một BỘ LỌC**: với mọi hạn D, *giờ làm việc còn lại ≤ N* ⟺ *D ≤ mốc cuối* — một phép đi **xuôi**, cùng một cài đặt (`domain.TienGioLamViec` và `domain.DueSoonCutoff` dùng chung `workingWalk`) |

**Vì sao "muộn nhất" chứ không "sớm nhất" như `due_at`:** khi giờ thứ N hết đúng ranh giới ca (17:00
thứ Sáu), mọi thời điểm tới đầu ca kế tiếp vẫn còn đúng N giờ. Lấy mốc sớm nhất thì một nhiệm vụ hạn
23:59 tối ấy rơi khỏi bộ lọc — và tính từ đêm hay cuối tuần với N tròn ngày thì lần nào cũng rơi.
Kiểm bằng phép so tương đương với một bộ đếm từng phút độc lập
(`service-identity/internal/domain/due_soon_cutoff_test.go`).

**Xã chưa cấu hình → `FAILED_PRECONDITION`**, đúng ba ca của `ResolveDeadlines`. Không 72, không lặng lẽ
bỏ bộ lọc. Nhiệm vụ không mang lĩnh vực nên đọc **dòng mặc định** của `nhiem-vu`.

**Hai cột báo lãnh đạo vẫn không ra** — mốc neo vẫn chưa ai trả lời. *(Hết đúng từ 29/09/2026 —
§Bổ sung ngay dưới.)*

→ Ranh giới giao dịch: `kb/30-indexes/transaction-boundaries.json`, `loc_so_nhiem_vu_theo_cau_hinh_cua_identity`

## BỔ SUNG 29/09/2026 — leo thang dùng **HAI NGƯỠNG** của §8, đếm bằng giờ làm việc

> **Người dùng, 29/09/2026 — hai ngưỡng:** leo thang theo **hai ngưỡng** của `docs/ui-ux/14-cau-hinh.md` §8 —
> báo **lãnh đạo trực tiếp** sau X giờ làm việc, báo **Chủ tịch** sau Y giờ làm việc. X là
> `gio_bao_lanh_dao`, Y là `gio_bao_chu_tich` (JSON `escalate_leader_hours` /
> `escalate_president_hours`, `service-identity/internal/http/sla.go:131-132`). **Không** dùng một
> số rồi nhân đôi.

| Nguồn | Nói gì | Chọn không |
|---|---|---|
| `14-cau-hinh.md` §8 | Hai cột, hai con số, xã sửa từng cột | **Chọn** |
| `14-cau-hinh.md` §9 (`:331`) | *"trễ gấp đôi thì báo lên chủ tịch"* | Bỏ phần "gấp đôi" |
| `../vigov-require/apps/api/app/workers/sla.py:292-301` | Một số `escalate_after_hours`, mức 2 = gấp đôi; trễ tính bằng **giờ đồng hồ** (`now - task.due_at`) | Không chép — hai lý do dưới |

**Vì sao không nhân đôi:** cột thứ hai tồn tại trên màn hình và trong bảng. Nhân đôi thì con số xã
gõ vào cột Chủ tịch không có tác dụng gì — một ô cấu hình im lặng bị bỏ qua.
**Vì sao không giờ đồng hồ:** luật 10 cấm #2. Đêm, cuối tuần, `ngay_nghi_le`, `ngay_lam_bu` không
phải giờ làm việc; phép cộng thuộc `identity` (ADR 0007).

> **Người dùng, 29/09/2026 — mốc đếm:** hai ngưỡng đếm **từ HẠN ĐÃ LỠ** của việc. Mức lãnh đạo trực
> tiếp khi đã trôi X giờ làm việc sau hạn lỡ; mức Chủ tịch khi đã trôi Y giờ. **Buộc Y ≥ X**, kiểm
> lúc ghi.

Không đếm từ lúc tiếp nhận: đếm từ đó thì leo thang cả việc **chưa trễ**. Hợp đồng 1796286 trả
hai mốc cho mỗi hạn đã lỡ — `ResolveEscalationInstants` →
`unit_head_due_at` / `chairman_due_at` (`proto/vigov/identity/v1/identity.proto:1268-1357`,
`:2812-2842`); bên chạy so `mốc <= claimed_at`, **không bao giờ lưu mốc** (luật 10 bất biến 3).
Người dùng muốn đếm từ mốc khác thì sửa **ở đây**, không ở ADR 0058.

**Y ≥ X đảo một dòng của hợp đồng:** chú thích `chairman_due_at` (`identity.proto:2837-2840`) ghi
*"NOT NECESSARILY LATER … deliberately"*, và `0008_sla.sql:245` chỉ kiểm `> 0`. Từ 29/09 phép kiểm
thuộc **tuyến ghi** `PATCH /api/v1/sla/{id}` (và dòng gieo); chú thích proto phải sửa theo — việc
của `contract-designer`, không phải tệp này.

> **Người dùng, 29/09/2026 — người nhận mức 2:** "Báo Chủ tịch" gửi cho **MỌI** vai trò lãnh đạo
> (`vai_tro.la_lanh_dao`), gồm cả Phó Chủ tịch — như `_leaders` của kho yêu cầu (`sla.py:305`).
> **Không** đổi lược đồ.

Tra qua `ResolveLeadershipStaff`. Mức 1 ("lãnh đạo trực tiếp") là người **trong bộ phận giữ việc**
nắm khoá quyền giao việc của miền ấy — `ResolveOrgUnitPermissionHolders`; lý do chọn khoá quyền
thay vì mã vai trò hay cột trưởng bộ phận nằm ở chú thích rpc ấy, không chép sang đây. Muốn chỉ
riêng Chủ tịch thì là một cờ **mới** trên vai trò — câu mới, không so chuỗi mã vai trò (luật 5 cấm #3).

> **Người dùng, 29/09/2026 — ngưỡng "bộ phận giữ mà chưa phân công ai"** (`14-cau-hinh.md:330`, việc
> `sla_reminders` của ADR 0058): một **trường SLA mới theo xã**, tính bằng **giờ làm việc**;
> **`NULL` = không báo**. `POST /api/v1/sla/defaults` gieo mặc định **8**.

Không có con số cứng trong mã (luật 10 cấm #3); 24 giờ đồng hồ của kho yêu cầu (`sla.py:42`) không
chép — sai cả số lẫn đơn vị. `NULL` là một lựa chọn của xã, không phải mặc định rơi về: bên chạy
gặp `NULL` thì không gửi, và không thay bằng 8. Số 8 chỉ sống trong **dòng gieo** — một hành vi
tường minh của quản trị xã, cùng tiền lệ với các dòng gieo khác của tuyến ấy.

### Còn mở — chưa ai quyết

| # | Câu | Vì sao không đoán |
|---|---|---|
| 1 | Phiếu phản ánh có hai đồng hồ (ADR 0028): lỡ `han_tiep_nhan` có leo thang không, hay chỉ `han_xu_ly_xong`. Hợp đồng để bên chạy nêu hạn nào (`identity.proto:1288-1290`), không chọn thay | Lỡ hạn tiếp nhận 2 giờ của lĩnh vực an ninh là chuyện lãnh đạo có thể muốn biết ngay; chọn thay là quyết hộ |
| 4 | Trường ngưỡng "chưa phân công" là một cột trên **mỗi dòng** `sla` (theo loại việc, lĩnh vực) hay **một** giá trị mỗi xã | Người dùng nói "trường SLA theo xã" và gieo qua `sla/defaults`, không nói hạt. Lượt dựng hỏi ở cổng, không tự chọn |

Câu từng mở #2 (Y ≥ X) và #3 (ai là hai mức): đã trả lời ngay trên.

**Chú thích trong mã đã cũ:** `service-identity/migrations/0008_sla.sql:107-124` và
`COMMENT ON COLUMN` ở `:265-269` vẫn ghi *"anchor UNDECIDED"* — **sai từ 29/09/2026**. Migration 0008
đã chạy nên không sửa tại chỗ; lượt dựng leo thang ghi đè bằng `COMMENT ON COLUMN` trong **một
migration mới**. Tới lúc ấy, đọc mốc đếm ở đây chứ không ở chú thích cột.

→ ADR 0007 (giờ làm việc, ba bảng lịch, SLA không hồi tố): `kb/10-decisions/0007-sla-working-hours.md`
→ ADR 0024 (quyền sở hữu đi theo nhịp đổi, phép thử `tenant_id`): `kb/10-decisions/0024-so-huu-danh-muc-tham-chieu.md`
→ ADR 0026 (mã lĩnh vực là bộ mã đóng ở `platform`): `kb/10-decisions/0026-linh-vuc-phan-anh-hai-tang.md`
→ ADR 0028 (hành vi nào ấn định hạn nào): `kb/10-decisions/0028-moc-dat-han-hai-dong-ho.md`
→ ADR 0004 (phân mảnh theo xã): `kb/10-decisions/0004-shard-by-tenant.md`
→ Ranh giới `identity`: `kb/00-foundation/domain-boundaries.md`
→ Bảng SLA 12 dòng và dòng mặc định: `docs/ui-ux/14-cau-hinh.md` §8
→ Luật 2 (một thực thể một service sở hữu): `.claude/rules/critical/2-service-boundary.md`
→ Kỹ năng: `.claude/skills/petition-lifecycle/SKILL.md`
