---
id: doi-chieu-2026-10-02-feat-m8-multitenant-foundation-phan-anh
tier: T2
source: CURATED
owner: architecture
derived_from_commit: d70bd7e9
expires: null
kho_nguon: vigov-require
branch: feat/m8-multitenant-foundation
sha_tu: 0053854
sha_den: 0053854
ngay_review: 2026-10-02
anh_huong: [web-admin, service-petitions]
owns_facts:
  - "ngày 02/10/2026 bên kia không có commit mới trên branch nào (neo 0053854 giữ nguyên)"
  - "màn cán bộ Phản ánh người dân (M4) của vigov-require tại 0053854 có những phần nào kho này chưa dựng, và chưa liệt kê ở PHAN_CHUA_DUNG"
---

# Đối chiếu `feat/m8-multitenant-foundation` · Phản ánh người dân (màn cán bộ) · 02/10/2026

`0053854` → `0053854` · **0 commit mới** · đọc ngày 02/10/2026 · cộng một lượt đọc **tại** `0053854` phạm vi M4 phía cán bộ

## A. Kiểm neo

| Kiểm | Kết quả |
|---|---|
| `fetch --all --prune` | `origin/feat/m8-multitenant-foundation` = `0053854` (24/09). `origin/main` = `93cff7f`, `origin/demo` = `8cd3924`, **cả hai là tổ tiên** của `0053854` (`git merge-base --is-ancestor`) |
| `git log 0053854..origin/<branch>` | **rỗng** ở cả ba branch. Neo **không dời** |
| Cây làm việc bên kia | **không sạch**: `M pnpm-lock.yaml` (+4 dòng), `?? apps/miniapp/HANDOFF.md`. Không checkout, không pull — không cần, vì không có commit mới. Hai tệp ấy **chưa commit**, nên không phải yêu cầu đã chốt; `HANDOFF.md` là tài liệu nhân bản Mini App, ngoài phạm vi lượt này |

Phần B là lượt đọc **tại một điểm**, không phải một khoảng. Lý do có lượt này: người dùng sắp giao việc *"phản ánh người dân trong web admin còn nhiều nội dung chưa làm"*, và
các ghi chú trước chỉ đọc phản ánh theo **từng commit**, chưa đọc toàn màn cán bộ.

## B. Tóm tắt — kho này phải làm gì

Chỉ ghi mục **chưa dựng mà cũng chưa có trong `PHAN_CHUA_DUNG`** (`web-admin/src/features/phan-anh/nhan-phieu.ts:1060-1135`), hoặc mục mà
`PHAN_CHUA_DUNG` đang nói sai. Mục đã có lý do trong danh sách ấy thì không chép lại.

| # | Bên kia | Kho này chạm | Module |
|---|---|---|---|
| 1 | **Đính kèm tệp vào nhật ký xử lý** và vào ô đổi trạng thái: `apps/api/app/modules/feedback/router.py:262-346` (`/log` multipart, `GET`/`DELETE /attachments/{id}` xoá mềm) · `apps/admin/src/components/feedback/FeedbackActivityPanel.tsx:215` · `FeedbackStatusPipeline.tsx:293`. Đặc tả kho này cũng đòi: `docs/ui-ux/09-phan-anh-nguoi-dan.md:197`, `:312` cột `dinh_kem` | Nhật ký phiếu **không có đính kèm**: `nhat-ky-phieu.tsx:278` (biểu mẫu) chỉ có chữ, và tuyến `POST …/log-entries` không nhận tệp. **Không có trong `PHAN_CHUA_DUNG`** → màn hình im lặng về một phần đặc tả. Tối thiểu: thêm một mục vào danh sách; dựng thật thì cần tuyến + hợp đồng (nhiệm vụ đã có khuôn `tasks/{ma}/attachments`) | web-admin · service-petitions |
| 2 | **Gộp phiếu trùng** (SRS M4.3.4, P1): `docs/SRS.md:321` · `docs/spec/05-nghiep-vu.md:207` · `router.py:364-388` · `service.py:79-84` (bán kính 50 m, 7 ngày, theo xã) · `FeedbackDetailDrawer.tsx:408-439` | Không có gì. Sổ `tien-do/service-petitions.json` ghi *"C-R3 gộp phiếu trùng"* là **CHƯA QUYẾT** (24/09). Đặc tả 09 không nhắc. Chưa có trong `PHAN_CHUA_DUNG` | chưa rõ — xem §D mục 3 |
| 3 | **Chín câu giải thích trạng thái**: `apps/admin/src/lib/feedback-display.ts:136-146` | `PHAN_CHUA_DUNG` (`nhan-phieu.ts:1121-1126`) nói *"đặc tả chỉ cho nguyên văn MỘT câu"*. Câu ấy vẫn đúng với đặc tả 09, nhưng **nguồn có tám câu còn lại** đã có ở prototype. Là chữ của BA, chưa phải chữ khách duyệt — xem §D mục 4 | web-admin (sau khi có người duyệt) |
| 4 | — | `PHAN_CHUA_DUNG` mục nhập hộ (`nhan-phieu.ts:1117-1118`) còn ghi câu mô tả modal *"đang chờ khách duyệt câu thay thế"*; đặc tả đã ghi **người dùng duyệt 30/09/2026** (`09-phan-anh-nguoi-dan.md:248-249`). Câu trên màn đang sai | web-admin |

## C. Chi tiết theo phần — tại `0053854`

| Phần | Bên kia (file:line) | Kho này (file:line) | Nhãn |
|---|---|---|---|
| Danh sách, lọc, tìm, chỉ trễ hạn, đánh giá thấp | `FeedbackWorkspace.tsx:143-206` · `schemas.py:231` | `lib/api/phieu-phan-anh.ts:148-173` · `so-phan-anh.tsx:443` | ĐÃ XONG |
| Phạm vi `Liên quan đến tôi` | `repository.py:85-111`: người được giao **hoặc** người tạo **hoặc** từng ghi nhật ký **hoặc** cùng bộ phận đang giữ | `nhan-phieu.ts:1093-1097`: máy chủ trả 400, *"chưa được chốt với khách"* | MỘT PHẦN — bên kia có định nghĩa, xem §D mục 2 |
| Thẻ phiếu (thumbnail) | `FeedbackCard.tsx` | `so-phan-anh.tsx:666-671` cố ý không thumbnail | MỘT PHẦN (cố ý) |
| Dải trạng thái, rẽ nhánh kèm lý do | `FeedbackStatusPipeline.tsx:76-90` · `05-nghiep-vu.md:205` | `so-phan-anh.tsx:42-43` · `:1257` `BieuMauReNhanh` | ĐÃ XONG |
| Đổi trạng thái **kèm** bàn giao một lượt | `router.py:277` `hand_over` | tuyến `assignment` tách rời — đã ghi ở ghi chú 23/09 §M2 mục 2 | đã ghi, không lặp |
| Ô `Hạn xử lý`, số ngày quá hạn | `feedback-display.ts:154-173`: `remaining_hours / 24` | `nhan-phieu.ts:1128-1133` không nói số ngày (giờ làm việc, ADR 0007) | MÂU THUẪN — lệch có chủ ý, §E |
| Ô `Đang giao cho` kèm email | `FeedbackDetailDrawer.tsx:255` | `nhan-phieu.ts:1107-1111` | đã có lý do |
| Kiểm duyệt công khai | **ba** trạng thái `pending/approved/hidden` (`05-nghiep-vu.md:192`, `feedback-display.ts:81-91`) | **hai** giá trị `cong-khai`/`an` (`phieu-phan-anh.ts:433`), cột bool (`09:294`); `can-bo` không bao giờ công khai (`service-petitions/internal/app/petition_publication.go:49-50`) | MỘT PHẦN — xem §D mục 5 |
| Ảnh trước | `FeedbackDetailDrawer.tsx:348-362` | `scene-photos.tsx` | ĐÃ XONG |
| Ảnh sau + tải lên | `router.py:146-198` · `FeedbackDetailDrawer.tsx:364-391` | `nhan-phieu.ts:1061-1070` (G8 hoãn) | đã có lý do |
| Chặn đóng khi thiếu ảnh sau | `05-nghiep-vu.md:195-203`, `service.py:93,563`: công tắc theo xã, **mặc định tắt** | `09:340-343`: mặc định **bật**, chưa dựng | MÂU THUẪN — lệch có chủ ý, §E |
| Vị trí, bản đồ nhỏ | `FeedbackMiniMap.tsx` | `nhan-phieu.ts:1072-1078` | đã có lý do |
| Đánh giá của dân (hiện), mở lại | `FeedbackDetailDrawer.tsx:441-482` | `citizen-report-blocks.tsx:64-87` · `so-phan-anh.tsx:831,902` | ĐÃ XONG |
| Cán bộ ghi đánh giá thay dân | `router.py:391-405` · `FeedbackDetailDrawer.tsx:516-540` | ADR 0062 | MÂU THUẪN — đã quyết, §E |
| Nhật ký xử lý (chữ) | `FeedbackActivityPanel.tsx` | `nhat-ky-phieu.tsx:69` | ĐÃ XONG |
| Nhật ký — đính kèm | xem §B mục 1 | không có | MỚI |
| Tạo nhiệm vụ từ phiếu | — | `petition-task.tsx:45` | ĐÃ XONG |
| Nhập hộ | `router.py:97-107` · `FeedbackEntryForm.tsx:133-232` | `nhan-phieu.ts:1114-1119`, không tuyến `POST /api/v1/citizen-reports` | MỚI (đã liệt kê) |
| KPI, Báo cáo, Bản đồ nhiệt | `router.py:110-121` · `reports.py` · `FeedbackReports.tsx` · `FeedbackHeatmap.tsx` | `nhan-phieu.ts:1081-1091` | MỚI (đã liệt kê) — cách tính bên kia MÂU THUẪN, §D mục 1 |
| Báo cáo `Xu hướng` (tuần/tháng, cùng kỳ) và `Mức hài lòng` (phân bố sao) | `docs/SRS.md:335-336`. Prototype và `reports.py` **không** dựng | đặc tả 09 §10 không có | CHƯA RÕ |
| Xuất báo cáo phản ánh | không có ở M4 | đặc tả 09 không có; `13-bao-cao.md:115` `report.export` là của module Báo cáo | CHƯA RÕ |
| Gợi ý bộ phận theo lĩnh vực + vị trí (M4.3.3, P1) | `docs/SRS.md:320`, không mã | không có | CHƯA RÕ |
| Leo thang khi quá hạn | `07-viec-nen-va-thong-bao.md:11` `sla_checker` | `service-petitions/internal/domain/automation.go:97-102` | ĐÃ XONG (máy chủ) |
| Lĩnh vực `can-bo` hạn chế | `05-nghiep-vu.md:204` | `so-phan-anh.tsx:147` · `petition_publication.go:49-50` | ĐÃ XONG |

## D. Cần người xác nhận — không suy

| # | Câu | Hai phía |
|---|---|---|
| 1 | **Cách tính đúng hạn/trễ hạn** cho KPI và tab Báo cáo | Bên kia `reports.py:50-57`: phiếu **chưa có hạn** là `open`, **ra khỏi mẫu số**; mốc xong = `closed_at or resolved_at`; `reports.py:60-65` giờ xử lý TB đếm **giờ treo tường**. Kho này: câu `DECIDED` #26 (ADR 0035 §C) đòi phiếu quá trần chưa phân loại **nằm trong** mẫu số; quyết định 24/09 đo đúng hạn tại `xu_ly_xong_luc`; ADR 0007 giờ làm việc. **Điều kiện dừng #1** — người dựng tab Báo cáo phải theo kho này hay hỏi lại, không chép `reports.py` |
| 2 | Định nghĩa `Liên quan đến tôi` | Bên kia `repository.py:95-110` có định nghĩa cụ thể (gồm **cùng bộ phận** đang giữ). Kho này `nhan-phieu.ts:1095-1096` ghi chưa chốt. Ghi chú 24/09 §Cần người xác nhận mục A đã hỏi vế "người liên quan" cho luật nắm giữ — cùng một câu. **Điều kiện dừng #2** |
| 3 | Có dựng gộp phiếu trùng không, và phiếu gộp xử lý ra sao với hai mã tra cứu đã trả cho hai người dân | `05-nghiep-vu.md:207` `merged_into_id` ↔ sổ `service-petitions.json` *"C-R3 … CHƯA QUYẾT"*. Chủ dự án. **Điều kiện dừng #3** — vì thế §B mục 2 không có module |
| 4 | Tám câu giải thích trạng thái lấy từ `feedback-display.ts:137-145` được không | Một câu trong đó (`:143` *"Phải có ảnh sau xử lý mới đóng được"*) **sai** với cả hai kho hôm nay: bên kia công tắc mặc định tắt, kho này chưa dựng phép chặn. Chủ dự án duyệt chữ |
| 5 | `Chờ kiểm duyệt` (KPI `09:45`) đếm gì khi cột là bool | Bên kia tách `pending` (chưa duyệt) khỏi `hidden` (đã chặn). Cột bool của kho này không phân biệt *"chưa ai xem"* với *"đã xem và ẩn"*, nên thẻ KPI ấy hoặc đếm cả hai, hoặc cần thêm dữ kiện. Hỏi trước khi dựng thẻ |

## E. Mâu thuẫn với quyết định đã chốt

Ba chỗ, **cả ba đã có quyết định ở kho này** — ghi để người dựng không "sửa cho khớp prototype":

| Chỗ | Bên kia | Kho này |
|---|---|---|
| Cán bộ ghi đánh giá thay dân | `router.py:391-405` | ADR 0062 (30/09) — không dựng |
| Ảnh nghiệm thu bắt buộc | `05-nghiep-vu.md:195-197` mặc định tắt | ADR 0008 + quyết định 24/09, 30/09: mặc định bật (`09:340-343`) |
| Số ngày quá hạn | `feedback-display.ts:164-166` chia 24 | ADR 0007 giờ làm việc; `nhan-phieu.ts:1128-1133` |

## Không ảnh hưởng

| Đã đọc | Lý do bỏ |
|---|---|
| `router.py:413-689` (tuyến công dân) | Ngoài phạm vi màn cán bộ |
| `M pnpm-lock.yaml`, `?? apps/miniapp/HANDOFF.md` ở cây bên kia | Chưa commit; lockfile và tài liệu nhân bản Mini App |
