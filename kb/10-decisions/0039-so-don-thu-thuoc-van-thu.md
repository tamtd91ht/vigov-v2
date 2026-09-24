---
id: 0039-so-don-thu-thuoc-van-thu
tier: T1
source: CURATED
owner: architecture
derived_from_commit: c0d591a
expires: null
owns_facts:
  - "sổ đơn thư công dân `don_thu` thuộc service `documents` (văn thư), không thuộc `petitions`"
  - "vì sao sổ đơn thư đi theo người cấp số vào sổ chứ không theo người tiếp dân"
  - "tầng nhãn theo xã của danh mục `Loại đơn thư` thuộc `documents`"
---

# 0039. Sổ đơn thư công dân thuộc văn thư (`documents`)

**Trạng thái:** đã chốt · **Ngày:** 2026-09-24 · **Người dùng chốt** (cổng xác nhận của lượt
`/develop-web-admin Văn bản & đơn thư`)
**Sửa một dòng của ADR 0001** (`0001-service-decomposition.md:54`) · **Trả lời câu (b) của ADR
0024 §2** (`0024-so-huu-danh-muc-tham-chieu.md:197-215`)

## Bối cảnh

Sổ đơn thư công dân (`don_thu`: khiếu nại, tố cáo, kiến nghị, đề nghị — đặc tả
`docs/ui-ux/05-van-ban-don-thu.md`) chưa có bảng, tuyến hay kiểu nào trong kho. Trước dòng
migration đầu tiên phải biết service nào sở hữu nó (luật 2 bất biến 1).

Hai nguồn trong kho nói ngược nhau:

| Nguồn | Nói gì |
|---|---|
| ADR 0001 `:54` | `petitions` = "Tiếp dân, xử lý đơn thư" |
| ADR 0024 `:202-205` | Để ngỏ: sổ đơn thư do **văn thư** hay **tiếp dân** cấp số vào sổ — câu (b), chưa ai hỏi khách |
| `service-petitions/migrations/0006_nhiem_vu.sql:252` | Chú thích: "`don_thu` belongs to service-documents" |
| `.claude/skills/doi-chieu-require/SKILL.md:113` | Bảng dịch: `petitions` bên kia ↔ đơn thư ở `service-documents` |
| `../vigov-require/docs/spec/05-nghiep-vu.md:64` | "Hai sổ riêng, chung bộ máy: `documents` và `petitions`" — mô hình `Petition` nằm ở `../vigov-require/apps/api/app/modules/documents/models.py:164` |

Tức là một ADR đã chốt một hướng, còn mã và kho yêu cầu đã đi hướng kia mà không ADR nào ghi.

## Các phương án

| Phương án | Được | Mất |
|---|---|---|
| `petitions` (giữ nguyên ADR 0001) | Khớp chữ "tiếp dân" trong ADR 0001; gần `nhiem_vu` nên "chuyển thành nhiệm vụ" là gọi trong một service | Phải dựng **cỗ máy dãy số thứ hai** trong một service chưa có; tách khỏi bộ máy vào sổ mà kho yêu cầu nói là **dùng chung** |
| **`documents` (văn thư)** | Sổ đi cùng người **cấp số vào sổ**; dùng lại được bảng đếm `day_so_van_ban` và ba cơ chế chống cấp trùng/cấp lại số đã có và đã qua đột biến | "Chuyển thành nhiệm vụ" thành **xuyên service**; dữ liệu cá nhân người gửi nằm ở `documents` |

## Quyết định

**Bảng `don_thu` và dãy số vào sổ đơn thư thuộc service `documents`.** Dòng `:54` của ADR 0001
được hiểu là: `petitions` = tiếp dân qua **phản ánh** và **nhiệm vụ**; đơn thư không còn ở đó.

## Vì sao chiều này, và vì sao đắt nếu đảo

**Số vào sổ là định danh lưu trữ vĩnh viễn.** Đã cấp thì không cấp lại, kể cả sau xoá mềm (luật 7
bất biến 3). Dời sổ sang service khác sau khi xã đã vào sổ tức là **di trú những số đã cấp**
trên hồ sơ lưu trữ — đúng cái giá ADR 0024 `:205` đã báo trước. Chọn đúng lúc chưa có dòng nào
thì không tốn gì.

**Chỗ cấp số đã có sẵn ở `documents`.** `service-documents/migrations/0004_so_van_ban.sql:220-241`
dựng `day_so_van_ban` với khoá `(tenant_id, so_sach, nam)` và `CHECK (so_sach IN ('den', 'di'))`
ở `:233`. Sổ đơn thư là một `so_sach` thứ ba trên cùng bộ máy — không phải một bộ máy thứ hai
phải chứng minh lại từ đầu rằng nó không thủng lỗ.

**ADR 0001 đã dự liệu việc này.** `0001-service-decomposition.md:94` ghi: nếu tiếp dân tách khỏi
xử lý đơn thư ngoài đời thì service cũng phải tách. Kho yêu cầu mô tả hai sổ riêng từ đầu, nên
đây là ADR 0001 gộp nhầm, không phải thực tế đổi.

## Hệ quả cho ADR 0024 §2 — `Loại đơn thư`

Câu (b) đã có trả lời: **văn thư**. Theo đúng dòng ADR 0024 `:203` ("danh mục thuộc `documents`,
đi cùng quy tắc đánh số sổ") và lập luận `:212-214` ("nhãn đi cùng sổ và sổ đi cùng người cấp
số"): **tầng nhãn theo xã của `Loại đơn thư` thuộc `documents`.**

ADR này **KHÔNG** chốt:

- **Chủ của bộ mã đóng** (tầng nền tảng, ADR 0024 `:207-210`). Tiền lệ `Lĩnh vực phản ánh` đặt ở
  `platform` (ADR 0026), nhưng tiền lệ không phải quyết định cho nhóm này.
- **Bộ mã có bao nhiêu mục** — xem xung đột "4 hay 5 loại" dưới đây. Chưa chốt thì chưa viết được
  hạt giống, nên điều kiện dừng #3 của ADR 0024 **vẫn chặn** việc dựng bảng danh mục này.

## Hệ quả — ghi ra, KHÔNG quyết ở đây

| # | Hệ quả | Trạng thái |
|---|---|---|
| 1 | Khoá quyền. Đặc tả §7.4 (`05-van-ban-don-thu.md:271`) gọi `petition.create` / `petition.read`; hai khoá đã nạp ở `service-identity/migrations/0001_init.sql:302-303`. Tên khoá không phải tên service, nên **giữ nguyên được**, `documents` kiểm hai khoá ấy | Không phải đổi |
| 2 | "Chuyển thành nhiệm vụ" từ đơn thư nay là **xuyên service** `documents → petitions`. Chưa có RPC, sự kiện hay dòng `kb/30-indexes/transaction-boundaries.json` nào — `proto/vigov/documents/v1/documents.proto:27-28` chỉ có `Health`. Hôm nay `POST /api/v1/tasks` tin `source` / `source_id` / hạn từ client (`service-petitions/internal/http/nhiem_vu_ghi.go:404-405, 414-415`) | **Luật 2 điều kiện dừng #2/#3** cho người dựng. Không được nối nút ấy qua `POST /tasks` trong lúc chờ hợp đồng |
| 3 | Họ tên, số điện thoại, địa chỉ người gửi nằm ở `documents` → che theo luật 3 bất biến 3. **Không có khoá xem đầy đủ cho đơn thư**; tiền lệ `feedback.unmask` (`service-identity/migrations/0007_quyen_phan_loai_va_xem_day_du.sql:59`) chỉ cho phản ánh | Luật 3 điều kiện dừng #1; khoá mới là việc của câu mở #27, không phải một `INSERT` (luật 5 #3c) |
| 4 | Dòng thực thể `don_thu` trong `kb/30-indexes/data-ownership.json` **chưa có** — tệp ấy SINH từ dấu `-- @entity` trong migration (ADR 0021). Nó hiện ra khi migration mang dấu ấy vào kho, không viết tay | Tự động |
| 5 | Các nơi còn mô tả `petitions` giữ đơn thư phải theo tệp này: `kb/00-foundation/domain-boundaries.md:82`, `service-petitions/README.md:3` | Việc của phiên chính |

## Các xung đột yêu cầu mà ADR này KHÔNG quyết

Quyết định chủ sở hữu **không** trả lời nghiệp vụ của sổ. Giữa đặc tả `05-van-ban-don-thu.md` và
`../vigov-require` còn chưa ai chốt:

- Bộ trạng thái đơn thư: sáu (§3.2, `05-van-ban-don-thu.md:56-65`) hay bảy (`Petition.status`
  dùng `DocumentStatus`, có `pending_routing` — `../vigov-require/apps/api/app/modules/documents/models.py:41-50, :209`)
- Loại đơn: **năm** có `phan-anh` (§3.3, `:69-77`) hay **bốn** (`../vigov-require/docs/spec/05-nghiep-vu.md:83`;
  `PetitionType` ở `../vigov-require/apps/api/app/modules/documents/models.py:73-79`)
- Số vào sổ: chỉ tự sinh (§3.4, §7.1) hay sửa được cho xã chuyển từ sổ tay
- Hạn mặc định: "Không đặt hạn" (§7.3, `:270`) hay theo SLA theo loại đơn — `sla.loai_viec` hôm
  nay không nhận đơn thư (`service-identity/migrations/0008_sla.sql:223`); thêm là luật 10 điều
  kiện dừng #1
- Cảnh báo đơn trùng: khớp theo gì, và §6 đặt số điện thoại trên URL (`:261`) — luật 3 cấm #4
- Trả lời công dân: chỉ lưu chữ hay gửi qua Zalo OA — gửi ra ngoài là luật 3 điều kiện dừng #2

Hỏi ở cổng của lượt dựng backend, không tự chọn.

→ ADR 0001 (bảng tám service, dòng `:54` mà tệp này sửa): `kb/10-decisions/0001-service-decomposition.md`
→ ADR 0024 §2 (ô `Loại đơn thư`, câu (b) mà tệp này trả lời): `kb/10-decisions/0024-so-huu-danh-muc-tham-chieu.md`
→ ADR 0026 (bộ mã đóng hai tầng, tiền lệ): `kb/10-decisions/0026-linh-vuc-phan-anh-hai-tang.md`
→ ADR 0021 (dấu `@entity`, chỉ mục sở hữu sinh ra): `kb/10-decisions/0021-khai-quyen-so-huu-thuc-the.md`
→ Tên tài nguyên URL `citizen-letters`: `kb/00-foundation/ubiquitous-language.md:141`
