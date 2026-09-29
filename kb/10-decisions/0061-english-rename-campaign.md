---
id: 0061-english-rename-campaign
tier: T1
source: CURATED
owner: architecture
derived_from_commit: 7197bae
expires: null
owns_facts:
  - "đợt đổi tên toàn kho sang tiếng Anh (chủ dự án chốt 29/09/2026): phạm vi, thứ tự service, ba lớp mỗi service"
  - "cách đổi tên bảng, cột và đối tượng CSDL đi kèm bằng migration ALTER … RENAME đảo ngược được"
  - "cửa sổ chuyển tiếp của giá trị enum trên dây: máy khách đọc được cả hai trước, máy chủ trả giá trị mới sau, bỏ giá trị cũ khi nào"
  - "ánh xạ giá trị enum cũ → mới và giá trị hành vi nhật ký cũ → mới, để đọc hồ sơ ghi trước đợt đổi tên"
  - "vì sao các dòng chỉ-thêm (audit_log, nhật ký, lịch sử chuyển) giữ nguyên giá trị cũ mãi mãi"
  - "những gì đợt đổi tên KHÔNG đụng tới"
---

# 0061. Đợt đổi tên toàn kho sang tiếng Anh

**Trạng thái:** **đã chốt** (chủ dự án, 29/09/2026 — hướng; **mọi mục X1–X25** cùng ngày, "theo đề
xuất hết") · Bảng ánh xạ còn sửa được cho tới khi lớp C của service đầu tiên chạy trên CSDL thật;
sau đó tệp **bất biến** như mọi ADR · **Thay một phần** ADR 0051 (dòng "Mã
ĐÃ CÓ" và dòng "Giá trị enum" của §Không thay đổi) và ADR 0011 (dòng "Giá trị enum" của bảng
§Quyết định, và §"Vì sao giá trị enum KHÔNG dịch") · **Đảo** luật 12 bất biến 3 cho riêng đợt này.

## Bối cảnh

ADR 0051 (28/09/2026) chốt tên **mới** viết tiếng Anh và **cấm** đổi tên hàng loạt mã cũ. Một ngày
sau, chủ dự án quyết định ngược lại cho mã cũ: đổi **toàn bộ** tên tiếng Việt sang tiếng Anh, kể cả
bảng, cột và giá trị enum lưu trong CSDL / đi trên API. Lý do của ADR 0051 vẫn đúng — hai thứ tiếng
trong một mô-đun, `ban`/`hop` đọc nhầm — và giá của việc sống chung với nó nhiều năm được chủ dự án
đánh giá là cao hơn giá một đợt đổi tên có kỷ luật **lúc chưa xã nào chạy thật**.

Ba lý do ADR 0051 đưa ra để **không** đổi vẫn là ba rủi ro thật. ADR này không phủ nhận chúng; nó
nói cách trả giá cho từng cái:

| Rủi ro ADR 0051 nêu | Cách đợt này trả |
|---|---|
| Đổi cột có dữ liệu là migration rủi ro trên hồ sơ lưu trữ (luật 7) | `ALTER … RENAME` không chép, không mất dữ liệu; có kịch bản đảo và bản sao lưu đã kiểm (§Lớp B) |
| Sửa migration đã áp làm `core/migrate` dừng vì lệch checksum | **Không sửa, không đổi tên** tệp migration đã áp nào. Đổi tên bằng tệp migration **mới** (§Lớp B) |
| Diff khổng lồ che mất thay đổi thật | Mỗi lớp một commit, không lẫn thay đổi nghiệp vụ nào (§Ba lớp) |

## Quyết định

**Phạm vi:** mọi tên tiếng Việt trong mã — định danh Go/TS (hàm, kiểu, biến, hằng, trường struct,
tệp, thư mục nguồn), tên bảng và cột, và **giá trị enum** lưu trong CSDL hoặc đi trên API.

**Thứ tự:** từng service, nhỏ trước: `platform` → `finance` → `reporting` → `comms` → `documents` →
`identity` → `petitions` → `core` → `web-admin` → `citizen-app`. Thứ tự này là thứ tự **bắt đầu**;
với giá trị đi xuyên service nó bị ràng buộc thêm bởi §Bên nhận trước bên gửi.

**Tên đích:** tra `kb/00-foundation/ubiquitous-language.md` §Từ điển đổi tên. Không tự dịch tại
chỗ — luật 12 cấm #2 vẫn nguyên hiệu lực, và đợt đổi tên là lúc dễ đặt tên thứ hai nhất.

### Ba lớp mỗi service — mỗi lớp một commit

| Lớp | Đổi gì | Không đổi gì | Bằng chứng xong |
|---|---|---|---|
| **A — định danh** | Định danh Go/TS, tệp và thư mục nguồn, dấu `vi-name-ok` hết lý do | Chuỗi SQL (vẫn gọi bảng/cột cũ), thẻ `json:"…"`, giá trị enum, chuỗi giao diện | `make check` xanh; không tệp `.sql` nào trong diff; `kb/20-contracts/openapi.json` sinh lại **không đổi một byte** |
| **B — lược đồ CSDL** | Bảng, cột, phân vùng, chỉ mục, ràng buộc, trigger, hàm, sequence; chuỗi SQL trong `store/` | Giá trị lưu trong cột, hợp đồng REST/gRPC | Migration mới áp được, **kịch bản đảo áp được**, áp lại lần nữa được — trên `tools/schema-smoke`; mọi phép thử "trigger từ chối" vẫn đỏ đúng chỗ (§Cạm bẫy) |
| **C — giá trị trên dây** | Giá trị enum lưu trữ + `CHECK`; tên trường JSON tiếng Việt còn sót; giá trị gửi qua gRPC/sự kiện | Đường dẫn URL | Cửa sổ chuyển tiếp chạy đúng bốn bước ở §Lớp C |

Lớp A đi trước vì nó **không đổi hành vi** — nếu nó đổi hành vi thì đó là lỗi, và phép thử thấy.
Lớp B tách khỏi A vì B là thay đổi trên CSDL thật, cần sao lưu và đường lui; trộn vào một commit
thì không lùi riêng được. Lớp C cuối cùng vì chỉ nó chạm tới máy khách nằm ngoài tầm kiểm soát.

### Lớp 0 — việc phải xong TRƯỚC lớp B của service đầu tiên

Các công cụ đang đọc lược đồ bằng cách tìm `CREATE TABLE`, và **không công cụ nào hiểu `ALTER …
RENAME`**. Sau lớp B đầu tiên chúng sẽ báo sai mà vẫn xanh — đúng dạng lỗi
"phép kiểm xanh sai lý do" mà kho này đã gặp nhiều lần nhất.

| Công cụ | Đọc gì hôm nay | Sau lớp B nếu không sửa |
|---|---|---|
| `tools/kb/ownership.go` | Dấu `@entity` ngay trên `CREATE TABLE` | `data-ownership.json` vẫn ghi tên bảng cũ |
| `tools/check_khoa_duy_nhat.py` | `UNIQUE` trong thân `CREATE TABLE` | Kiểm khoá ghép `tenant_id` (luật 1 bất biến 6) trên bảng không còn tồn tại |
| `tools/quyen_keys.py` · `tools/check_quyen.py` · `.claude/hooks/quyen_key_guard.py` | Đúng hai câu `INSERT INTO quyen` | Tập khoá quyền đọc từ tên bảng cũ; bảng mới tên khác thì luật 5 bất biến 3c mất cưỡng chế |
| `tools/schema-smoke` | Áp chuỗi migration | Phải thêm bước áp **kịch bản đảo** rồi áp lại |
| Hook trong `.claude/hooks/` và `code_signals` trong `kb/00-foundation/open-questions.json` | Mẫu regex trên tên tiếng Việt (`han_xu_ly_xong`, `tenant_id`, …) | Hook im lặng vì mẫu không còn khớp — không phải vì mã đúng |

Sổ quyết định (§Sổ quyết định đặt tên) đã chốt hết ngày 29/09/2026 — tên bảng là thứ đắt nhất để
đổi lần hai, nên đừng mở lại một dòng X giữa lúc đang làm lớp B.

### Lớp B — cơ chế đổi tên lược đồ

**Một tệp migration mới mỗi service**, tên tiếng Anh (`NNNN_rename_schema_to_english.sql`). `core/migrate`
áp mỗi tệp trong **một giao dịch** (`core/migrate/migrate.go:184-218`), và DDL của PostgreSQL có
giao dịch — nên một lược đồ đổi tên dở dang không thể tồn tại. Tệp phải giữ đúng tính chất đó: không
`COMMIT` giữa chừng, không `CREATE INDEX CONCURRENTLY`.

| Đối tượng | Cách đổi | Vì sao phải nói ra |
|---|---|---|
| Bảng cha | `ALTER TABLE … RENAME TO` | Khoá ngoại, `CHECK`, chỉ mục bám theo OID — tự đi theo |
| **32 phân vùng** mỗi bảng (`<bảng>_p00`…`_p31`, `PARTITION BY HASH (tenant_id)`) | `ALTER TABLE <cũ>_pNN RENAME TO <mới>_pNN` cho từng cái | Không đổi thì vẫn chạy, nhưng bảng tên Anh có phân vùng tên Việt — đúng thứ trộn hai thứ tiếng đợt này muốn gỡ |
| Cột | `ALTER TABLE … RENAME COLUMN` | Cột sinh (`GENERATED ALWAYS AS`), `CHECK`, biểu thức chỉ mục tự đi theo |
| Chỉ mục, ràng buộc, trigger, sequence | `ALTER INDEX/SEQUENCE … RENAME`, `ALTER TABLE … RENAME CONSTRAINT`, `ALTER TRIGGER … ON … RENAME TO` | Tên không tự đổi; tên ràng buộc hiện trong thông báo lỗi người đọc thấy |
| **Hàm plpgsql** | `CREATE OR REPLACE FUNCTION` với **thân mới**, rồi `ALTER FUNCTION … RENAME` | **Thân hàm KHÔNG được viết lại khi cột đổi tên** — xem §Cạm bẫy |
| `COMMENT ON` | Viết lại | Bám theo OID nên còn, nhưng chữ bên trong nhắc tên cũ |
| Bảng sổ `schema_migration` | **Không đổi** trong lớp B — xem §Xung đột X23 | `core/migrate` tạo và đọc nó **trước** migration đầu tiên; nó không thể tự đổi tên mình bằng một migration |

**Tệp migration đã áp: không đổi tên, không sửa một ký tự.** Tên tệp là khoá của dòng tiến độ
(`schema_migration.ten`); đổi tên tệp là một migration "mới" bị áp **lần hai**. Chú thích trong tệp
cũ vẫn gọi tên cũ — đó là lịch sử, và §Từ điển đổi tên là chỗ đọc nó.

**Đảo ngược được (luật 7 bất biến 4):** mỗi tệp lớp B đi kèm kịch bản đảo — cùng các câu `RENAME`
theo chiều ngược, cùng thân hàm cũ nguyên văn, và xoá dòng của tệp khỏi `schema_migration` trong
cùng giao dịch (khuôn đã có ở `service-finance/migrations/0007_nguon_von.sql:358-367`). Kịch bản đảo
**không** được đặt trong thư mục `migrations/` — `embed.FS` nhúng mọi tệp `.sql` ở đó và
`core/migrate` sẽ áp nó. Chỗ đặt chốt lúc viết lớp B của `platform`. Và **có bản sao lưu đã kiểm
khôi phục** trước khi chạy trên CSDL có dữ liệu thật — hai điều kiện, không phải chọn một.

**`@entity`:** dấu đứng trên câu `ALTER TABLE … RENAME TO` trong tệp mới, và `tools/kb/ownership.go`
phải học đọc nó (Lớp 0).

### Cạm bẫy — hàm trigger mở cửa trong im lặng

Đây là lý do lớp B cần phép thử **từ chối**, không chỉ phép thử **chạy được**:

| Hàm | Dòng | Sau đổi tên, nếu không viết lại thân |
|---|---|---|
| `so_van_ban_bat_bien` | `service-documents/migrations/0004_so_van_ban.sql:173` — `IF TG_TABLE_NAME LIKE 'van_ban_den%'` | Bảng tên `incoming_document` **không bao giờ khớp** → trigger thôi chặn đánh số lại `so_vao_so`. Không lỗi nào, không phép thử "chạy được" nào đỏ |
| `chung_tu_da_khoa` | `service-finance/migrations/0007_nguon_von.sql:316` — `IF OLD.trang_thai <> 'da-khoa' THEN RETURN NEW` | Sau lớp C, chứng từ đã khoá mang `locked` ≠ `'da-khoa'` → **mọi** chứng từ đã khoá thành sửa được |
| Mọi hàm đọc `NEW.<cột>` / `OLD.<cột>` | 57 câu `CREATE FUNCTION` trong `service-*/migrations` | Lỗi lúc chạy ở lượt `UPDATE` đầu tiên sau đổi tên — tức ở máy thật, không ở CI nếu CI không `UPDATE` bảng ấy |

Ba loại tham chiếu phải quét trong **mọi** thân hàm, ở cả lớp B lẫn lớp C: **tên cột**, **tên bảng**
(kể cả qua `TG_TABLE_NAME`), **giá trị enum** dạng chữ. Mỗi trigger chặn phải có một phép thử chứng
minh nó **vẫn chặn** sau lớp B và sau lớp C.

### Lớp C — giá trị trên dây: bốn bước, không gộp

Cách làm "máy chủ nhận cả hai, trả giá trị mới" **một mình thì chưa đủ**: bản Mini App đang phát hành
trên Zalo nhận giá trị mới mà nó không biết, và hiện "trạng thái không rõ" cho mọi phiếu
(`citizen-app/src/cong-dan/man/unknown-status-commune-app.test.tsx` cho thấy nó có đường ấy). Bản
mới lại phải qua Zalo duyệt, mất ngày. Nên thứ tự là:

| Bước | Việc | Điều kiện sang bước sau |
|---|---|---|
| **C1** | `web-admin` và `citizen-app` **đọc được cả giá trị cũ lẫn mới** trong phản hồi; vẫn **gửi** giá trị cũ | Bản `citizen-app` mang C1 **đã được Zalo duyệt và phát hành**; `web-admin` mang C1 đã triển khai |
| **C2** | Máy chủ **nhận cả hai** ở mọi đầu vào (REST, gRPC, sự kiện, hàm trigger); migration `UPDATE` giá trị lưu trên bảng nghiệp vụ — **kể cả hồ sơ đã đóng/đã khoá**, bằng migration hệ thống ghi vết từng dòng với tác nhân `system` (luật 6 bất biến 6; X21, không ánh xạ lúc đọc), hàm trigger chặn được viết lại trong chính migration để nhận việc ấy — và thay `CHECK`; máy chủ **trả giá trị mới** | Mọi bên nhận của mọi giá trị xuyên service đã ở C2 (§Bên nhận trước bên gửi) |
| **C3** | Máy khách **gửi** giá trị mới | Bản `citizen-app` mang C3 đã phát hành, và bản mang C1-trở-về-trước **không còn** chạy (Zalo không cho giữ bản cũ song song thì điều này tự đúng khi bản mới lên) |
| **C4** | Máy chủ **bỏ nhận** giá trị cũ ở đầu vào; `CHECK` trên bảng nghiệp vụ chỉ còn giá trị mới | — |

**`CHECK` trên bảng chỉ-thêm KHÔNG BAO GIỜ tới C4** — xem mục ngay dưới.

**Nếu chưa bản `citizen-app` nào đọc các giá trị này đang phát hành**, C1 và C3 rẻ gần bằng không. Đó
là điều phải **kiểm** lúc làm, không phải giả định.

### Bên nhận trước bên gửi — giá trị đi xuyên service

Thứ tự service ở trên **không** tự bảo đảm thứ tự đúng cho giá trị đi qua ranh giới. Ví dụ có thật:
`documents` và `petitions` gửi `work_kind` (`van-ban-den`, `phan-anh`, …) sang `identity` qua gRPC
(`proto/vigov/identity/v1/identity.proto:24-25`), và `comms` nhận giá trị trạng thái phiếu qua sự kiện
`PetitionStatusChanged` (`proto/vigov/petitions/v1/events.proto:99`). `documents` đứng **trước**
`identity` trong thứ tự — nên C3 của `documents` (gửi `incoming-document`) **phải chờ** C2 của
`identity`. Hàng đợi `su_kien_di` cũng giữ sự kiện mang giá trị cũ qua lần triển khai: bên nhận phải
đọc được cả hai **trước** khi bên gửi đổi.

### Dòng chỉ-thêm giữ giá trị cũ — mãi mãi

`audit_log`, `platform_audit_log`, `operator_audit_log`, `nhat_ky_phan_anh`, `nhat_ky_nhiem_vu`,
`lich_su_chuyen_van_ban` là hồ sơ chỉ-thêm; trigger của chúng từ chối `UPDATE` (luật 6 bất biến 4,
luật 7 cấm #5). Đợt đổi tên **không** viết lại chúng, và không tắt trigger để viết lại:

- Cột `trang_thai_tai_thoi_diem`, `hanh_vi`, `action` giữ nguyên giá trị đã ghi. `CHECK` trên các cột ấy
  nhận **hợp của hai tập** vĩnh viễn.
- `audit_log.delta` giữ **tên cột cũ** làm khoá JSON. Đọc bằng §Từ điển đổi tên của
  `ubiquitous-language.md`.
- Mọi đường đọc (màn nhật ký, báo cáo, bộ lọc `?action=` của `*-audit-entries`) phải hiểu **cả hai**
  tập giá trị. Một bộ lọc chỉ biết giá trị mới là bộ lọc giấu mọi mục trước đợt đổi tên.

Người thanh tra đọc mục cũ bằng hai bảng dưới.

## Ánh xạ giá trị enum cũ → mới

Định dạng giá trị mới: **kebab-case** — X14, chủ dự án chốt 29/09/2026. Cột "Nơi" là ràng buộc `CHECK` hoặc hằng Go
có giá trị ấy. Dấu `Xn` chỉ tới dòng quyết định trong sổ quyết định của glossary.

### `petitions`

| Tập | Cũ → mới | Nơi |
|---|---|---|
| Trạng thái phiếu phản ánh (9 — **khách đã duyệt nguyên văn chuỗi cũ**, ADR 0027; X20) | `da-tiep-nhan`→`received` · `dang-phan-loai`→`classifying` · `da-chuyen-xu-ly`→`dispatched` · `dang-xu-ly`→`in-progress` · `da-xu-ly`→`resolved` · `cho-dan-xac-nhan`→`awaiting-citizen` · `da-dong`→`closed` · `khong-tiep-nhan`→`rejected` · `chuyen-cap-tren`→`referred` | `0004_phieu_phan_anh.sql`, `nhat_ky_phan_anh.trang_thai_tai_thoi_diem` (0013); `service-petitions/internal/domain/phieu_phan_anh.go:37-45` |
| Kênh tiếp nhận | `zalo-mini-app`, `zalo-oa` giữ nguyên (tên sản phẩm) · `web-xa`→`commune-web` · `can-bo-nhap-ho`→`staff-entry` | 0004; `phieu_phan_anh.go:127-130` |
| Hành vi nhật ký phiếu | `phan-loai`→`classification` · `phan-cong`→`assignment` · `chuyen-trang-thai`→`status-change` · `dong-phieu`→`closure` · `khong-tiep-nhan`→`rejection` · `chuyen-cap-tren`→`referral` · `ghi-chu`→`note` · `danh-gia`→`rating` · `mo-lai-theo-danh-gia`→`reopen-by-rating` | 0013, 0018; `nhat_ky_phan_anh.go:26-42` |
| Công khai phiếu | `cho-duyet`→`pending` · `cong-khai`→`public` · `an`→`hidden` | 0017:163; `petition_publication.go:27-29` |
| Trạng thái nhiệm vụ (7) | `moi-giao`→`new` · `da-tiep-nhan`→`accepted` · `dang-thuc-hien`→`in-progress` · `cho-duyet`→`pending-approval` · `hoan-thanh`→`completed` · `tam-dung`→`suspended` · `chuyen-tiep`→`forwarded` | 0006, `nhat_ky_nhiem_vu.trang_thai_tai_thoi_diem`, `nhan_trang_thai_nhiem_vu.ma` (0010); `nhiem_vu.go:39-45` |
| Nguồn giao nhiệm vụ | `truc-tiep`→`direct` · `ket-luan-hop`→`meeting-conclusion` · `van-ban-den`→`incoming-document` · `phan-anh`→`citizen-report` | 0006; `nhiem_vu.go:196-199` |
| Đề nghị lùi hạn | `cho-duyet`→`pending` · `da-duyet`→`approved` · `tu-choi`→`rejected` | 0006; `nhiem_vu_ghi.go:626-628` |
| Nhóm văn bản của nhiệm vụ | `cap-tren-giao`→`assigned-by-superior` · `chi-dao-dang-uy`→`party-committee-directive` · `san-pham-dau-ra`→`output` | 0009; `nhiem_vu_van_ban.go:43-45` |
| Vai trò nhãn trạng thái (chỉ trong mã) | `chinh`→`main` · `re-nhanh`→`branch` | `nhan_trang_thai_nhiem_vu.go:30-32` |
| Biên bản họp | `du-thao`→`draft` · `da-ky`→`signed` | 0012; `bien_ban_hop.go:27-28` |
| Tình trạng kết luận (suy ra, chỉ trên dây) | `chua-giao`→`unassigned` · `dang-thuc-hien`→`in-progress` · `qua-han`→`overdue` · `hoan-thanh`→`completed` | `bien_ban_hop.go:165-168` |
| Loại hạn (chỉ trên dây) | `han-xu-ly`→`due-at` · `han-phan-loai`→`classify-due` · `han-xu-ly-xong`→`resolve-due` | `summary_metrics.go:157-163` |
| Loại việc tự động hoá | `nhiem-vu`→`task` · `phan-anh`→`citizen-report` | `service-petitions/internal/domain/automation.go:33-34` |

### `documents`

| Tập | Cũ → mới | Nơi |
|---|---|---|
| Trạng thái văn bản đến | `moi-vao-so`→`registered` · `da-phan-cong`→`assigned` · `dang-xu-ly`→`in-progress` · `da-giai-quyet`→`resolved` · `chuyen-cap-tren`→`referred` · `luu-khong-thu-ly`→`filed-not-admitted` | 0004 (cả `lich_su_chuyen_van_ban.trang_thai_tai_thoi_diem`); `van_ban.go:34-39` |
| Độ khẩn (**thuật ngữ pháp lý**, X19) | `thuong`→`normal` · `khan`→`urgent` · `thuong-khan`→`very-urgent` · `hoa-toc`→`immediate` | 0004; `van_ban.go:68-71` |
| Sổ | `den`→`incoming` · `di`→`outgoing` | 0004 (`day_so_van_ban.so_sach`) |

### `finance`

| Tập | Cũ → mới | Nơi |
|---|---|---|
| Trạng thái chứng từ | `ke-toan-nhap`→`entered` · `da-xac-nhan`→`confirmed` · `da-khoa`→`locked` | 0004; `chung_tu_giai_ngan.go:50-57`; **hàm `chung_tu_da_khoa`** (§Cạm bẫy) |
| Loại bảng ngân sách | `thu`→`revenue` · `chi`→`expenditure` (X12) | 0006; `thu_chi_ngan_sach.go:58-59` |
| Kiểu cột | `so`→`number` · `phan_tram`→`percent` | 0006; `:65-66` |
| Vai trò cột (**thuật ngữ ngân sách**, X19) | `du-toan-tp-giao`→`estimate-assigned-by-province` · `du-toan-xa-giao`→`estimate-assigned-by-commune` · `thu-nsnn`→`state-budget-revenue` · `thu-xa-huong`→`commune-retained-revenue` · `du-toan-nam`→`annual-estimate` · `chi-ngan-sach`→`budget-expenditure` | 0006; `:83-100` |
| Đơn vị tính | `dong`→`dong` · `nghin-dong`→`thousand-dong` · `trieu-dong`→`million-dong` | `:490-492` |
| Nguồn ngưỡng | `mac-dinh`→`default` · `xa`→`commune` | `nguong_canh_bao_cham.go:29-32` |
| Tình trạng gán nguồn | `chua-gan-nguon`→`unfunded` · `chua-du`→`underfunded` · `du`→`funded` | `nguon_von.go:58-62` |

### `identity`

| Tập | Cũ → mới | Nơi |
|---|---|---|
| Khai cư trú (**thuật ngữ pháp lý**, X19) | `thuong_tru`→`permanent-residence` · `tam_tru`→`temporary-residence` · `chua_khai`→`undeclared` | 0004; `quan_he_cong_dan_xa.go:74-82` |
| Nguồn khai | `cong_dan`→`citizen` · `can_bo`→`staff` | 0004; `:104-105` |
| Trạng thái xác thực | `cho_xac_thuc`→`pending` · `da_xac_thuc`→`verified` · `tu_choi`→`rejected` | 0004; `:90-97` |
| Nguồn phiên công dân | `app`→`app` · `ghep`→`pairing` | 0004 |
| Trạng thái mã ghép | `cho_ghep`→`pending` · `da_ghep`→`paired` · `da_huy`→`cancelled` · `het_han`→`expired` | `ghep_phien.go:34-37` |
| Nguồn xã của phiên | `app_rieng`→`dedicated-app` · `qr_xac_nhan`→`confirmed-qr` · `nho_lai`→`remembered` | `cau_phien.go:46-48` |
| Loại việc SLA / `work_kind` (**xuyên service**) | `van-ban-den`→`incoming-document` · `phan-anh`→`citizen-report` · `nhiem-vu`→`task` · `don-thu`→`citizen-letter` | 0008, 0016, 0017; `service-identity/internal/domain/sla.go:19-26` |

### `comms` và `platform`

| Tập | Cũ → mới | Nơi |
|---|---|---|
| Tầng danh mục (5 service) | `he-thong`→`system` · `don-vi`→`commune` | `nguon` ở mọi bảng danh mục ba tầng; `danh_muc_ba_tang.go` |
| Đối tượng thông báo công dân | `phieu-phan-anh`→`citizen-report` | comms 0004 |
| Trạng thái gửi công dân | `cho-gui`→`pending` · `da-gui`→`sent` · `that-bai`→`failed` · `chua-cau-hinh-kenh`→`channel-not-configured` | comms 0004; `thong_bao_gui_cong_dan.go:54-69` |
| Thông báo nội bộ | `nhap`→`draft` · `da-phat-hanh`→`published` · `da-go`→`withdrawn` | comms 0005 |
| Trạng thái thư điện tử | `chua-gui`→`not-sent` · `dang-gui`→`sending` · `da-gui`→`sent` · `loi`→`failed` | comms 0005 |
| Loại bài Mini App | `tin-tuc`→`news` · `su-kien`→`event` · `thong-bao`→`notice` · `truyen-thanh`→`broadcast` · `video`, `banner` giữ | comms 0006 |
| Trạng thái bài | `dang-hien`→`visible` · `cho-duyet`→`pending` · `an`→`hidden` | comms 0006 |
| Nguồn bài | `thu-cong`→`manual` · `dong-bo-cong`→`portal-sync` | comms 0006 |
| Kiểu trường bản đồ | `van-ban`→`text` · `so-nguyen`→`integer` · `so-thap-phan`→`decimal` · `dung-sai`→`boolean` · `ngay`→`date` · `chon`→`choice` | comms 0007; `map_field_schema.go:29-34` |
| Chuông cán bộ | `sap-den-han`→`due-soon` · `qua-han`→`overdue` · `leo-thang`→`escalation` · `ban-tin-tuan`→`weekly-digest` | comms 0010 |
| Chế độ Mini App | `chinh`→`main` · `rieng`→`dedicated` | platform 0006; `mini_app.go:13-14` |

**Tập đã tiếng Anh, không đổi:** `stored_file.status`, `retention_class`, `bucket`, `purpose`,
`automation_*`, `outcome`, `run_trigger`, `security`, `cach_tinh` (`manual`/`entries`/`children`),
`tone`, khoá `message_key` (trừ hai khoá `expense_*` của X12), khoá quyền `ops.*`, `EscalationLevel`
(`chairman`, `unit_head` — X4 chốt đó là hai khái niệm riêng, cùng `leader`). *(Mặc định của agent,
29/09/2026 — người dùng có thể đổi:)* X14 chỉ áp cho giá trị tiếng Việt đang được dịch, nên tập
tiếng Anh dạng snake (`sla_reminders`, `configuration_missing`) **giữ**; `purpose` = `petition-photo`
**giữ** dù X1 cho thôi chữ `Petition`, vì giá trị ấy nằm trong khoá đối tượng đã lưu
(`core/storage/key.go:64`) — đổi là dời tệp lưu trữ.

**Không nằm trong bảng, có chủ ý:** mã danh mục do xã tự gõ và mã lĩnh vực tầng 1 (`rac-thai`…)
**không đổi** — dữ liệu của xã, luật 7 bất biến 3, ADR 0060 (X18).

**Loại đơn thư** (chưa có bảng lưu; đặt tên sẵn để bảng đầu tiên sinh ra đã đúng — *mặc định của
agent, 29/09/2026, người dùng có thể đổi: năm từ này chưa có trong đề xuất người dùng đã duyệt*): `phan-anh`→`citizen-report`
· `kien-nghi`→`recommendation` · `khieu-nai`→`complaint` · `to-cao`→`denunciation` · `de-nghi`→`proposal`.
Nghĩa pháp lý: mục ngay dưới.

## Nghĩa pháp lý của các thuật ngữ được dịch — X19

Dịch một thuật ngữ pháp lý là khẳng định hai từ cùng nghĩa (ADR 0011). Bảng này ghi **nghĩa ở phía
Việt Nam** — thứ từ tiếng Anh phải chở — để người đọc mã không suy nghĩa từ chữ tiếng Anh. **Không bao
giờ gộp hai dòng vào một từ**: mỗi dòng là một thủ tục, một thời hạn hay một mức bảo vệ khác nhau.

| Tiếng Việt | Mã mới | Nghĩa pháp lý — không lẫn với |
|---|---|---|
| Phản ánh | `citizen-report` | Người dân báo một vấn đề thực tế để cơ quan biết và xử lý; **không** phản đối một quyết định. Không phải `complaint` |
| Kiến nghị | `recommendation` | Đề xuất cơ quan sửa, bổ sung chính sách hay cách làm; không kèm yêu cầu huỷ một quyết định |
| Đề nghị | `proposal` | Đề nghị cơ quan làm một việc cụ thể trong thẩm quyền |
| Khiếu nại | `complaint` | Không đồng ý với **quyết định hành chính / hành vi hành chính cụ thể** xâm phạm quyền của chính người khiếu nại (Luật Khiếu nại 2011) — có **thời hạn thụ lý và giải quyết luật định**. Gọi nhầm là sai thời hạn |
| Tố cáo | `denunciation` | Báo **hành vi vi phạm pháp luật** của bất kỳ ai (Luật Tố cáo 2018) — người tố cáo được **bảo vệ, giữ bí mật danh tính**. Gọi nhầm là mất bảo vệ ấy |
| Tiếp nhận (hành vi) | `acknowledge` | Cán bộ nhận và đọc hồ sơ; **không** bắt đầu đồng hồ luật định (X15) |
| Tiếp nhận (trạng thái) | `received` | Phần mềm đã ghi nhận phiếu (X15) |
| Thụ lý | `admission` | Cơ quan chấp nhận giải quyết — **bắt đầu đồng hồ luật định**. Không bao giờ gộp với `acknowledge` |
| Lưu không thụ lý | `filed-not-admitted` | Văn bản vào sổ nhưng cơ quan **không** nhận giải quyết |
| Thường / Khẩn / Thượng khẩn / Hỏa tốc | `normal` / `urgent` / `very-urgent` / `immediate` | Các mức độ khẩn của văn bản hành chính (Nghị định 30/2020/NĐ-CP về công tác văn thư), tăng dần |
| Thường trú / Tạm trú | `permanent-residence` / `temporary-residence` | Hai loại cư trú theo Luật Cư trú 2020 — ở đây là **lời khai** của công dân, xã xác thực riêng (ADR 0023) |
| Dự toán TP giao / xã giao / năm | `estimate-assigned-by-province` / `estimate-assigned-by-commune` / `annual-estimate` | Dự toán ngân sách do cấp có thẩm quyền giao (Luật Ngân sách nhà nước 2015); "TP" là cấp tỉnh/thành phố trực thuộc trung ương |
| Thu NSNN / Thu xã hưởng | `state-budget-revenue` / `commune-retained-revenue` | Tổng thu ngân sách nhà nước trên địa bàn, khác phần thu **xã được hưởng** theo phân cấp |
| Chủ tịch / Lãnh đạo / Trưởng bộ phận | `chairman` / `leader` / `unit_head` | Ba vai khác nhau (X4): người đứng đầu UBND xã; nhóm lãnh đạo xã; người đứng đầu một bộ phận |

**Chủ dự án chốt dịch (X19); các từ tiếng Anh chưa qua người nắm thủ tục soát.** Ai thấy một từ làm
lệch nghĩa ở cột phải thì đổi **từ** — trước lớp C của service giữ giá trị ấy — không đổi **nghĩa**.

## Ánh xạ giá trị hành vi `audit_log.action` cũ → mới

Chỉ **mục mới** mang giá trị mới; mục cũ giữ nguyên và đọc bằng bảng này (§Dòng chỉ-thêm; X22). Khuôn `động_từ_tân_ngữ`,
đúng dạng các giá trị đã tiếng Anh (`save_mail_settings`, `update_petition_field`). `gỡ` → `remove`,
`xoá` → `delete`: hai hành vi khác nhau trong mã hiện tại, giữ khác nhau. Từ cho phản ánh là `citizen_report` (X1).

| Service | Cũ → mới |
|---|---|
| `core` | `xem_nhat_ky_he_thong`→`read_audit_log` |
| `comms` | `boc_lai_khoa_du_lieu_xa`→`rewrap_data_encryption_key` · `tao_khoa_du_lieu_xa`→`create_data_encryption_key` · `doc_thong_bao_chuong`→`read_staff_notification` · `doc_het_thong_bao_chuong`→`read_all_staff_notifications` · `gui_thong_bao_chuong`→`deliver_staff_notifications` · `ghi_so_thong_bao`→`record_citizen_notification` · `ghi_ket_qua_thong_bao`→`record_citizen_notification_result` · `gui_thu_thu_may_chu_thu`→`send_test_mail` · `luu_cau_hinh_may_chu_thu`→`save_mail_settings` · `phat_hanh_thong_bao`→`publish_announcement` · `them_/sua_/xoa_loai_tai_nguyen_ban_do`→`create_/update_/delete_map_asset_type` · `nhap_loai_tai_nguyen_ban_do`→`import_map_asset_types` · `them_/sua_/xoa_truong_ban_do`→`create_/update_/delete_map_field` · `them_/sua_noi_dung_mini_app`→`create_/update_content_item` · `them_danh_muc_mini_app`→`create_content_category` |
| `documents` | `vao_so_van_ban_den`→`register_incoming_document` · `sua_van_ban_den`→`update_incoming_document` · `go_van_ban_den`→`remove_incoming_document` · `chuyen_van_ban_den`→`route_incoming_document` · `cap_so_van_ban_di`→`issue_outgoing_document_number` · `sua_van_ban_di`→`update_outgoing_document` · `go_van_ban_di`→`remove_outgoing_document` · `them_/sua_/xoa_loai_van_ban`→`create_/update_/delete_document_type` · `nhap_loai_van_ban`→`import_document_types` |
| `finance` | `them_/sua_/xoa_du_an`→`create_/update_/delete_investment_project` · `them_/sua_chung_tu_giai_ngan`→`create_/update_disbursement_voucher` · `go_chung_tu_giai_ngan`→`remove_disbursement_voucher` · `xac_nhan_chung_tu_giai_ngan`→`confirm_disbursement_voucher` · `khoa_/mo_khoa_chung_tu_giai_ngan`→`lock_/unlock_disbursement_voucher` · `them_/sua_/xoa_hang_muc_ke_hoach_von`→`create_/update_/delete_capital_plan_category` · `tao_/sua_/go_bang_ngan_sach`→`create_/update_/remove_budget_sheet` · `them_/sua_/go_khoan_muc_ngan_sach`→`create_/update_/remove_budget_line` · `dat_dong_tong_ngan_sach`→`set_budget_headline` · `ghi_/go_dot_thu_chi`→`record_/remove_budget_entry` · `sua_loi_he_thong`→`reword_system_message` · `khoi_phuc_loi_he_thong_mac_dinh`→`restore_system_message` |
| `reporting` | `sua_loi_he_thong`→`reword_system_message` · `khoi_phuc_loi_he_thong_mac_dinh`→`restore_system_message` |
| `identity` | `dang_nhap`→`log_in` · `dang_xuat`→`log_out` · `doi_mat_khau`→`change_password` · `them_can_bo`→`create_staff` · `sua_ho_so_can_bo`→`update_staff` · `xoa_can_bo_nhap_trung`→`delete_duplicate_staff` · `cap_tai_khoan_can_bo`→`issue_staff_account` · `dat_lai_mat_khau_can_bo`→`reset_staff_password` · `khoa_/mo_khoa_tai_khoan_can_bo`→`lock_/unlock_staff_account` · `doi_vai_tro_can_bo`→`change_staff_role` · `doi_thu_tu_danh_ba`→`reorder_staff_directory` · `cong_khai_/rut_cong_khai_mini_app`→`publish_/unpublish_staff_on_mini_app` · `luu_phan_quyen_vai_tro`→`save_role_permissions` · `gieo_vai_tro_mau`→`seed_role_template` · `gieo_quan_tri_mac_dinh`→`seed_default_administrator` · `gieo_lich_lam_viec_mac_dinh`→`seed_default_working_hours` · `gieo_ngay_nghi_le_mac_dinh`→`seed_default_public_holidays` · `gieo_thoi_han_xu_ly_mac_dinh`→`seed_default_processing_deadlines` · `them_/sua_/xoa_bo_phan`→`create_/update_/delete_org_unit` · `them_/sua_/xoa_ca_lam_viec`→`create_/update_/delete_working_shift` · `them_/sua_/xoa_ngay_nghi_le`→`create_/update_/delete_public_holiday` · `them_/sua_/xoa_ngay_lam_bu`→`create_/update_/delete_swap_working_day` · `sua_thoi_han_xu_ly`→`update_processing_deadline` · `them_/sua_/xoa_khoi_nhiem_vu`→`create_/update_/delete_task_bloc` · `them_/sua_/xoa_loai_don_vi_dan_cu`→`create_/update_/delete_residential_unit_type` · `them_/sua_thon_to_dan_pho`→`create_/update_residential_unit` · `ngung_dung_/dung_lai_thon_to_dan_pho`→`deactivate_/reactivate_residential_unit` · `sua_cau_hinh_tu_dong_hoa`→`update_automation_job` · `yeu_cau_chay_ngay_tu_dong_hoa`→`request_automation_run` · `ghi_ket_qua_luot_chay_tu_dong`→`record_automation_run` · `mo_phien_cong_dan`→`open_citizen_session` · `lien_ket_dinh_danh_zalo`→`link_zalo_identity` · `doi_xa_da_nho`→`change_remembered_commune` |
| `petitions` | `cong_dan_gui_phan_anh`→`submit_citizen_report` · `phan_loai_phan_anh`→`classify_citizen_report` · `phan_cong_phan_anh`→`assign_citizen_report` · `chuyen_trang_thai_phan_anh`→`change_citizen_report_status` · `dong_phan_anh`→`close_citizen_report` · `ghi_chu_phan_anh`→`add_citizen_report_note` · `khong_tiep_nhan_phan_anh`→`reject_citizen_report` · `chuyen_cap_tren_phan_anh`→`refer_citizen_report` · `xem_day_du_nguoi_gui`→`unmask_citizen_report_reporter` · `tao_/sua_/xoa_nhiem_vu`→`create_/update_/delete_task` · `phan_cong_nhiem_vu`→`assign_task` · `chuyen_trang_thai_nhiem_vu`→`change_task_status` · `ghi_nhat_ky_nhiem_vu`→`add_task_log_entry` · `de_nghi_lui_han_nhiem_vu`→`request_task_extension` · `quyet_dinh_lui_han_nhiem_vu`→`decide_task_extension` · `nhap_nhiem_vu_tu_excel`→`import_tasks` · `xuat_so_theo_doi_nhiem_vu`→`export_task_register` · `yeu_cau_tai_tep_nhiem_vu`→`request_task_attachment_upload` · `luu_tep_nhiem_vu`→`store_task_attachment` · `tu_choi_tep_nhiem_vu`→`reject_task_attachment` · `tep_nhiem_vu_het_han_tai`→`expire_task_attachment_upload` · `tao_/sua_/xoa_bien_ban_hop`→`create_/update_/delete_meeting` · `ky_bien_ban_hop`→`sign_meeting` · `ghi_thong_bao_ket_luan`→`record_meeting_notice` · `them_/sua_/xoa_ket_luan_hop`→`create_/update_/delete_meeting_conclusion` · `danh_dau_/bo_dau_khong_phat_sinh`→`set_/clear_no_task_marker` · `them_/sua_/xoa_loai_nhiem_vu`→`create_/update_/delete_task_type` · `them_/sua_/xoa_muc_uu_tien_nhiem_vu`→`create_/update_/delete_task_priority` · `update_petition_field` (đã tiếng Anh)→`update_citizen_report_field` (X1) · `sua_nhan_trang_thai_nhiem_vu`→`update_task_status_label` · `sua_loi_he_thong`→`reword_system_message` · `khoi_phuc_loi_he_thong_mac_dinh`→`restore_system_message` |

Bảng này dựng từ các hằng `HanhVi*` / `Action*` và giá trị chữ trong `service-*/internal/**` ngày
29/09/2026. Hành vi thêm sau ngày đó sinh ra đã tiếng Anh, không cần dòng ở đây.

## Không thay đổi — nêu rõ để không ai suy rộng

| Thứ | Vì sao |
|---|---|
| Đường dẫn URL `/api/v1/…` và tham số đường dẫn (`{maTraCuu}`, `{ma}`) | Hợp đồng với bên tích hợp (ADR 0011). Tham số đường dẫn là một phần của đường dẫn đã sinh vào `tools/ingress` |
| Đoạn route `web-admin/src/app/**` | Cán bộ nhìn thấy trên thanh địa chỉ — là giao diện (ADR 0051). `citizen-app` **không có router** nên tên tệp của nó đổi như mã (X24) |
| Chuỗi giao diện, thông báo lỗi người đọc, văn xuôi `kb/` | Người dùng và người giám sát đọc tiếng Việt |
| Tệp migration đã áp — tên lẫn nội dung | Checksum và khoá tiến độ (§Lớp B) |
| Dòng đã ghi trong bảng chỉ-thêm | Luật 6 bất biến 4, luật 7 cấm #5 (§Dòng chỉ-thêm) |
| ADR đã chốt | ADR không sửa; người đọc tra tên mới ở §Từ điển đổi tên |
| Tên service, proto package, tên sự kiện — kể cả `petitions`, `vigov.petitions.v1`, `petitions.*` | Tên hợp đồng giữa service, đã tiếng Anh (ADR 0001, 0011; luật 2 bất biến 4). *(Mặc định của agent, 29/09/2026 — người dùng có thể đổi.)* Chỉ kiểu và bảng thành `CitizenReport` |
| Bảng sổ `schema_migration` | X23 |
| Khoá quyền (`feedback.*`, `petition.*`, `admin.user`, …) | **Không đổi trong đợt theo service.** Đổi **sau cùng**, cùng câu hỏi mở #27, khi mọi service xong lớp C (X25) |

## Sổ quyết định đặt tên

**Mọi mục X1–X25 đã chốt ngày 29/09/2026** (chủ dự án, "theo đề xuất hết"). Quyết định, bằng chứng
từng cách viết cũ và các câu *mặc định của agent* nằm ở **một** chỗ:
`kb/00-foundation/ubiquitous-language.md` §Từ điển đổi tên → *Xung đột — sổ quyết định*. Không chép
lại ở đây. Đổi một quyết định sau này là sửa dòng X ấy kèm ngày, rồi sửa bảng ánh xạ của tệp này nếu
nó chạm giá trị lưu trữ.

## Cái giá

- **Máy khách ngoài tầm kiểm soát:** bốn bước lớp C kéo dài bằng hai vòng duyệt Zalo. Trong cửa sổ
  ấy máy chủ mang hai tập giá trị ở mọi đầu vào — mã nhiều nhánh hơn, phép thử gấp đôi.
- **Hai tập giá trị trong CSDL vĩnh viễn** ở các bảng chỉ-thêm; mọi đường đọc phải hiểu cả hai.
- **Lịch sử `git blame` bị cắt** ở commit lớp A của mỗi service. Người tra "vì sao dòng này" phải
  bước qua commit đổi tên.
- **ADR và chú thích migration cũ gọi tên cũ** mãi mãi. §Từ điển đổi tên là cầu nối duy nhất; nó
  không bao giờ được xoá dòng.
- **Công cụ và hook phải sửa trước** (Lớp 0) — nếu không, hàng rào im lặng vì hết khớp, không phải
  vì mã đúng.
- **Luật 12 bất biến 3 và ADR 0051 đảo cho đợt này.** Ngoài đợt này, đổi tên tiện tay vẫn bị cấm
  ("Stay in scope").

## Cưỡng chế

- Lớp A: `make check` xanh + `openapi.json` không đổi byte nào.
- Lớp B: `tools/schema-smoke` áp → đảo → áp lại; phép thử "trigger vẫn từ chối" cho từng trigger chặn.
- Lớp C: phép thử đầu vào nhận **cả hai** giá trị (C2–C3) và chỉ giá trị mới (C4).
- Luật: `.claude/rules/critical/12-english-identifiers.md` (bất biến 1: tên cũ đổi theo từng service,
  theo ADR đợt đổi tên này; bất biến 3: một từ điển duy nhất).

→ ADR bị thay một phần: `kb/10-decisions/0051-english-code-identifiers.md` §Quyết định (dòng "Mã ĐÃ
CÓ") và §Không thay đổi (dòng "Giá trị enum") · `kb/10-decisions/0011-contract-surface-language.md`
§Quyết định (dòng "Giá trị enum") và §"Vì sao giá trị enum KHÔNG dịch"
→ Từ điển tên bảng, cột, gốc định danh: `kb/00-foundation/ubiquitous-language.md` §Từ điển đổi tên
→ Chín trạng thái khách đã duyệt: `kb/10-decisions/0027-trang-thai-va-dong-ho-phieu-phan-anh.md`
→ Mã lĩnh vực tầng 1 không bao giờ đổi: `kb/10-decisions/0060-duong-doc-bo-ma-linh-vuc-tang-1.md`
