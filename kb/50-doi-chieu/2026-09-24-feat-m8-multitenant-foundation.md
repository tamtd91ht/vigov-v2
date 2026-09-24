---
id: doi-chieu-2026-09-24-feat-m8-multitenant-foundation
tier: T2
source: CURATED
owner: architecture
derived_from_commit: 0218b14
expires: null
kho_nguon: vigov-require
branch: feat/m8-multitenant-foundation
sha_tu: b159f0e
sha_den: 233f57f
ngay_review: 2026-09-24
anh_huong: [service-documents, service-petitions, web-admin]
owns_facts:
  - "khoảng b159f0e..233f57f của ../vigov-require đổi luật nghiệp vụ nào, và phân hệ nào của kho này phải đổi theo"
  - "vì sao lần đối chiếu này KHÔNG đưa service-comms, citizen-app, service-identity vào anh_huong"
---

# Đối chiếu `feat/m8-multitenant-foundation` · 24/09/2026

`b159f0e` → `233f57f` · 11 commit, cùng ngày 24/09/2026 (13:58 → 16:16) · 69 tệp · +5818/−224 · đọc ngày 24/09/2026

Cây làm việc bên kia sạch, đứng đúng branch này, `HEAD` = `origin/…` = `233f57f`. Không phải pull.

**`233f57f` là bản `docs/spec/` đuổi kịp mười commit trước nó.** Ghi chú đọc `docs/spec/` trước, rồi
mở mã ở những chỗ tài liệu tả chưa đủ để kết luận (migration `0051`, `tasks/service.py`,
`feedback/service.py`, `document-display.ts`).

## Tóm tắt — kho này phải làm gì

| # | Bên kia đổi | Kho này chạm | Module |
|---|---|---|---|
| 1 | Đơn thư và văn bản đến có **ô nhật ký tách khỏi việc đổi trạng thái**: một dòng nhật ký là một dòng bàn giao có `status_after = NULL` — `430c9ae` · `05-nghiep-vu.md` §M2 | `lich_su_chuyen_van_ban` **không chứa được dòng ấy**: `trang_thai_tai_thoi_diem` và `den_bo_phan_id` đều `NOT NULL` (`service-documents/migrations/0004_so_van_ban.sql:513,518`). `nhat_ky_don_thu` chưa tạo, và là bảng chỉ-thêm, nên phải chốt hình dạng cột **trước** dòng dữ liệu đầu tiên | service-documents |
| 2 | Giao đơn thư hoặc văn bản cho một cán bộ thì **người ấy được báo** (chuông + Zalo). Mỗi lần giao là một lần báo, không chống trùng — `8c1e2a7` · `07-viec-nen-va-thong-bao.md` | `service-documents` chưa phát sự kiện nào; sự kiện duy nhất của kho là `petitions.*`. Chuông của kho này chỉ có loại *"Được giao nhiệm vụ"* (`docs/ui-ux/08-thong-bao.md:167`). Luồng mới đòi một sự kiện mới, tức **luật 2 điều kiện dừng #3** | service-documents (bên phát) · service-comms **chưa rõ** |
| 3 | Sổ đơn thư lọc được theo **khoảng ngày nhận · đơn vị đang giữ · cán bộ xử lý · trạng thái**; thêm cột **"Số ngày xử lý"** — `430c9ae` · `8c1e2a7` | Đặc tả kho này chỉ có `?pham_vi=&loai=&trang_thai=&q=` (`05-van-ban-don-thu.md:250`) và không có cột ấy (`:43-52`). Cách đếm ngày → **§Cần người xác nhận, mục B** | service-documents · web-admin |
| 4 | **Luật người đang giữ** mở sang nhiệm vụ, văn bản và đơn thư: người được giao đích danh đổi được trạng thái; người liên quan (người giao, người theo dõi, người tạo, cán bộ cùng bộ phận đang giữ) chỉ ghi nhật ký — `a37ec96` · `8c1e2a7` · `02-da-tenant-va-bao-mat.md` | Chủ dự án chốt 23/09 *"theo require"* **chỉ cho phản ánh** (`tien-do/service-petitions.json`, mục `luat-nam-giu-hoi-khach`). Nhiệm vụ ở kho này vẫn khai `task.update` (`service-petitions/internal/http/routes.go:961-962`), và ADR 0038 nói rõ quyết định theo người-trên-bản-ghi **không tự áp** sang bản ghi khác → **§Cần người xác nhận, mục A** | service-petitions · service-documents · web-admin |
| 5 | Nhiệm vụ: **cơ quan chủ trì tham mưu ≡ cơ quan thực hiện**, **chuyên viên tham mưu ≡ người thực hiện chính**. Ghi thì điền cả hai vế; migration `0051` chép nửa đang có sang nửa đang trống — `e1d0204` | Kho này giữ **hai cặp cột rời** (`0006_nhiem_vu.sql:269-270` và `:287-288`), đúng như đặc tả của chính nó (`docs/ui-ux/02-nhiem-vu.md:298-302`). Hệ quả đo được: bộ lọc người thực hiện chỉ đọc `nguoi_thuc_hien_ma` (`service-petitions/internal/store/nhiem_vu.go:185-186`), nên việc chỉ ghi chuyên viên sẽ **không hiện** ở "Giao cho tôi" — đúng lỗi bên kia vừa sửa → **§Cần người xác nhận, mục C** | service-petitions · web-admin |
| 6 | Hạn nhiệm vụ **có ngày và giờ**, điền sẵn **bảy ngày nữa, 17:00** — `b9a9718` | Kho này đã chọn **23:59** và **cố ý từ chối 17:00** vì giờ tan làm là cấu hình từng xã; chỗ ấy ghi rõ đây là một giả định chờ khách chốt (`web-admin/src/features/nhiem-vu/nhan-nhiem-vu.ts:624-639`) → **§Cần người xác nhận, mục D** | web-admin |
| 7 | Phản ánh: bắt ảnh sau xử lý mới đóng được phiếu thành **công tắc của xã, mặc định TẮT**; ảnh đính trong ô đổi trạng thái cũng tính là ảnh chứng minh — `b9a9718` | **MÂU THUẪN** với câu **#7 `DECIDED` 16/09/2026**: `bat_buoc_anh_nghiem_thu` **mặc định `true`**. Xem §Mâu thuẫn | service-petitions |
| 8 | Dải trạng thái đơn thư hiện **cả hành trình** (kể cả bước đã qua, bước chưa bấm được), tô đậm bước đang đứng, cùng kiểu với Nhiệm vụ và Phản ánh. **Màn hình phải khoá đúng bằng máy chủ** — `9ad3689` · `06-giao-dien.md` | Drawer của kho này tả hàng bốn nút *"chuyển sang"* (`05-van-ban-don-thu.md:110-114`), tức đúng hình dạng bên kia vừa bỏ. Tab đơn thư chưa vẽ, nên đây là đầu vào thiết kế cho lượt dựng, không phải sửa màn đang có | web-admin |

## Chi tiết theo phân hệ

### M2 — Văn bản đến và đơn thư (`van-ban-don-thu`)

Sổ đơn thư **chưa dựng ở cả hai phía** của kho này (`tien-do/service-documents.json`, mục
`so-don-thu-cong-dan`, `chua_lam`; ADR 0039). Mọi dòng dưới đây là **đầu vào cho cổng của lượt
dựng ấy**, cộng thêm vào mười hai xung đột C2…C13 mục ấy đã liệt kê. Không chép lại mười hai mục ấy.

| Commit | Tệp bên kia | Đổi gì | Hệ quả ở kho này |
|---|---|---|---|
| `430c9ae` | `apps/api/app/modules/documents/router.py` (`/{document_id}/note`, `/{petition_id}/note`) · `service.py` `add_note` | Tuyến ghi nhật ký riêng cho cả văn bản đến và đơn thư. Cửa khai `document.read` / `petition.read`; tầng nghiệp vụ quyết ai ghi được | **Đơn thư**: đặc tả kho này đã có `POST /api/don-thu/:id/nhat-ky` (`05-van-ban-don-thu.md:258`), nhưng `nhat_ky_don_thu` được tả **có** `trang_thai_tai_thoi_diem` (`:242`). Bên kia nói rõ phần lớn đời một lá đơn là những dòng không đổi trạng thái; thiết kế bảng phải cho cột ấy rỗng, hoặc tách hai loại dòng. **Văn bản đến**: bảng vết hiện tại không nhận dòng không-trạng-thái (`0004_so_van_ban.sql:513,518`); muốn có nhật ký cho văn bản đến là đổi lược đồ một bảng chỉ-thêm (luật 7 điều kiện dừng #2 nếu đụng dữ liệu đã có) |
| `430c9ae` | `documents/repository.py` | Lọc `assignee_id`, `arrived_from`, `arrived_to` | Thêm vào tuyến danh sách `citizen-letters` lúc dựng. Luật 3: ô tìm/lọc **không** mang dữ liệu cá nhân trên chuỗi truy vấn — bẫy đã ghi ở `tien-do/web-admin.json` mục `man-van-ban-den-va-di` cho sổ ĐI |
| `8c1e2a7` | `documents/service.py` · `admin/messages.py` · `zalo_bot/service.py` | Giao hồ sơ cho cán bộ → sinh thông báo `document.transferred`, **không khoá chống trùng**. Câu mở của tin Zalo chọn theo **loại bản ghi** trước, rồi mới theo vai; dòng dữ kiện của đơn thư là trích yếu · số đến · người gửi · hạn | Kho này **chưa có kênh Zalo nhắc cán bộ** (lần đối chiếu 23/09 đã xếp nó là câu hỏi phạm vi). Phần dùng được ngay là luật **"mỗi lần giao là một lần báo, người vừa bấm thì không báo"**. ⚠ Dòng *"người gửi"* trong tin nhắn đi qua hạ tầng bên thứ ba là **dữ liệu cá nhân gửi ra ngoài** — luật 3 điều kiện dừng #2 nếu kho này có kênh ấy |
| `8c1e2a7` | `apps/admin/src/lib/document-display.ts` `handlingDays` · `PetitionTable.tsx` | Cột "Số ngày xử lý": từ nửa đêm ngày nhận tới hôm nay (đơn mở) hoặc tới ngày giải quyết (đơn xong), **tính cả ngày nhận**, ngày lịch | → §Cần người xác nhận, mục B |
| `8c1e2a7` | `PetitionDetailDrawer.tsx` | Người được giao đích danh bấm được dải trạng thái. Máy chủ đã cho từ trước, chỉ màn hình còn khoá | → §Cần người xác nhận, mục A |
| `9ad3689` | `PetitionDetailDrawer.tsx` · `document-display.ts` | Dải trạng thái hiện toàn hành trình thay cho các nút lối ra | Đầu vào cho tab đơn thư (Tóm tắt mục 8) |

### M1 — Nhiệm vụ (`nhiem-vu`)

| Commit | Tệp bên kia | Đổi gì | Hệ quả ở kho này |
|---|---|---|---|
| `e1d0204` · migration `0051` | `migrations/versions/20260924_0051-0051_task_one_owner.py` · `tasks/service.py` · `TaskAssignForm.tsx` · `TaskDetailDrawer.tsx` | Hai cặp là **một**. Migration chép chéo cho việc cũ và **không đảo ngược được** (`downgrade` để trống, ghi rõ lý do) | → §Cần người xác nhận, mục C. Nếu kho này theo: bảng `nhiem_vu` hiện có dữ liệu thật hay chưa quyết định cách làm rẻ hay đắt — **chưa kiểm**, phải kiểm trước khi chọn |
| `a37ec96` | `tasks/router.py` (`/progress`, `/log`: `task.update` → `task.read`) · `tasks/service.py` `work_rights` | Ba mức `full` / `log` / `none`. `full` = có `task.update` **hoặc** là người được giao. `log` = người theo dõi, lãnh đạo giao việc, người tạo, người phối hợp, hoặc cán bộ của bộ phận đang giữ | → §Cần người xác nhận, mục A. Một câu phụ phải hỏi cùng: `full` của bên kia có gồm **hoàn thành** không. Ở kho này hoàn thành còn đòi quyền duyệt và mọi việc con đã xong (`routes.go:940-951`) |
| `b9a9718` | `TaskAssignForm.tsx` · `lib/task-display.ts` | Ô hạn có giờ; mặc định +7 ngày, 17:00 | → §Cần người xác nhận, mục D |
| `8c1e2a7` | `tasks/service.py` | Người nhận thông báo giao việc mang **vai** của mình (người thực hiện · người theo dõi · cả bộ phận) để câu chào đúng | Chỉ áp khi kho này có kênh nhắc cán bộ. Chưa có → không có việc |

### M4 — Phản ánh (`phan-anh`)

| Commit | Tệp bên kia | Đổi gì | Hệ quả ở kho này |
|---|---|---|---|
| `b9a9718` | `feedback/service.py` (`AFTER_PHOTO_KEY`, `_has_evidence`) · `tests/test_feedback_flow.py` · `conftest.py` | Công tắc `feedback_after_photo_required` theo xã, mặc định tắt. Khi bật, ảnh trong ô "ảnh sau xử lý" **hoặc** tệp `image/*` đính trong ô đổi trạng thái đều đủ. Bài kiểm chạy **cả hai chiều** của công tắc | **§Mâu thuẫn** cho vế mặc định. Vế *"ảnh ở đâu thì được tính"* không mâu thuẫn gì và nên vào thiết kế lúc dựng cờ `bat_buoc_anh_nghiem_thu` (cờ này **chưa dựng**, `tien-do/service-petitions.json` mục `luat-nam-giu-hoi-khach`) |
| `8c1e2a7` | `feedback/service.py` `notice_brief` | Phiếu thuộc lĩnh vực hạn chế (tố cáo cán bộ) → tin nhắn **không mang nội dung**, chỉ báo có việc | Cùng tinh thần `feedback.restricted` của kho này. Áp khi có kênh nhắc cán bộ |
| `233f57f` | `docs/spec/02-da-tenant-va-bao-mat.md` · `05-nghiep-vu.md` §M1 | Luật người đang giữ áp cho phản ánh có thêm vế **người liên quan chỉ ghi nhật ký** (gồm cán bộ cùng bộ phận đang giữ) | Quyết định 23/09 của chủ dự án chỉ nói về **người được giao**. Vế người liên quan là phần mới → §Cần người xác nhận, mục A. Bảng nhật ký phiếu (`nhat_ky_phan_anh`) kho này chưa tạo, nên chưa có mã nào phải sửa |

### Một cửa (`mot-cua`) và M6 — không vào `anh_huong`

| Commit | Tệp bên kia | Đổi gì | Hệ quả ở kho này |
|---|---|---|---|
| `e9011c2` · `8c1e2a7` · `e1d0204` (phần `dossiers/`) | `dossiers/service.py` · `router.py` · `apps/miniapp/src/pages/DossierPage.tsx` | Tra hồ sơ Mini App đổi từ *số hồ sơ + 4 số cuối điện thoại* sang **số điện thoại đầy đủ + 4 số cuối số hồ sơ**; kết quả là danh sách hồ sơ; thêm `POST /citizen/dossiers/detail` | **Không có việc.** Hồ sơ một cửa **ngoài phạm vi hợp đồng**, khách chốt 20/09/2026 (ADR 0001 §Bổ sung). Cách nhận danh tính công dân bên kia cũng là vùng cố ý lệch. Bài học đáng giữ nếu một cửa quay lại: *"một mảnh dữ liệu người khác cũng có thì chưa phải danh tính"* (`09-bay-va-bai-hoc.md`) |
| `5484162` · `e080d15` · `86c9f76` · migration `0050` | `modules/citizens/**` · `integrations/esms/client.py` · `components/citizens/**` · `navigation.ts` | **Danh bạ người dân** (`citizens` · `citizen_groups` · nhãn) và **gửi tin ZNS/SMS qua eSMS** theo mẫu đã duyệt, xem trước chi phí rồi mới gửi. Đường eSMS **chưa nối**, cố ý từ chối. Hai menu mới: "Danh bạ người dân", "Gửi tin ZNS / SMS"; menu cũ đổi tên "Thông báo nội bộ" | **Câu hỏi phạm vi, không phải việc.** Kho này gửi ZNS từ OA **của từng xã** (ADR 0018, `service-comms`), không qua eSMS, và **không có kho số điện thoại người dân** nào ngoài phiên đăng nhập. Một danh bạ người dân là một kho dữ liệu cá nhân mới (Nghị định 13): căn cứ thu thập, đồng ý của người dân, ai được xem đầy đủ — luật 3 điều kiện dừng #1 và #2. Phải hỏi khách **có đưa vào phạm vi không** trước khi bàn thiết kế |

## Mâu thuẫn với quyết định đã chốt

**MỘT mâu thuẫn.**

### Mặc định của cờ "bắt buộc ảnh nghiệm thu" khi đóng phiếu phản ánh

**1 · Bên kia nói gì.** `b9a9718` · `apps/api/app/modules/feedback/service.py`, hằng `AFTER_PHOTO_KEY`
và lời giải thích đặt ngay trên nó; `docs/spec/05-nghiep-vu.md` §M4 (`233f57f`): công tắc
`Tenant.settings.feedback_after_photo_required`, **mặc định tắt**. Lý do vận hành: phần lớn phản ánh
không có gì để chụp, nên bắt buộc thì cán bộ *"chụp bừa một bức tường, hoặc bỏ phiếu mở mãi"*.

**2 · Kho này đã chốt gì.** Câu hỏi mở **#7**, `DECIDED` **16/09/2026**, ADR 0008 (luật 10 ghi #6 · #7 ·
#8 là quyết định của khách): *"Hai cờ cấu hình theo xã: `bat_buoc_nguoi_khac_dong` (mặc định false)
và `bat_buoc_anh_nghiem_thu` (**mặc định true**)."*

Hai bên **đồng ý** đây là cấu hình theo xã. Chỉ lệch **giá trị mặc định**. Lệch này vẫn đắt: mặc định là
thứ mọi xã chưa tự khai sẽ chạy theo, tức phần lớn xã trong ngày đầu.

**3 · Ai phải quyết.** Khách, qua chủ dự án, vì #7 là câu khách đã ký. Không tự chọn bên nào. Cờ ở kho
này **chưa dựng**, nên hôm nay đổi hay giữ đều chưa chạm dữ liệu nào.

## Cần người xác nhận — không phải mâu thuẫn, nhưng không được tự quyết

| # | Câu hỏi | Bằng chứng hai bên | Ai quyết |
|---|---|---|---|
| A | **Luật người đang giữ có mở sang nhiệm vụ · văn bản đến · đơn thư không, và vế "người liên quan chỉ ghi nhật ký" (gồm cả cán bộ cùng bộ phận) có áp không?** | Bên kia: `a37ec96` `tasks/service.py` `work_rights`; `430c9ae` `documents/router.py` (cửa `petition.read`); `02-da-tenant-va-bao-mat.md`. Kho này: quyết định 23/09 chỉ phủ **phản ánh** và chỉ **người được giao**; ADR 0038 §"Vì sao KHÔNG tự động áp sang phiếu phản ánh" đặt nguyên tắc **không áp chéo bản ghi**; nhiệm vụ đang khai `task.update` (`routes.go:961-962`) | Chủ dự án. Hỏi gộp một lần cho ba sổ. Nếu theo: mỗi tuyến vẫn **một khoá phẳng ở cửa**, điều kiện người-trên-bản-ghi nằm ở tầng nghiệp vụ, đúng khuôn ADR 0038; và đủ bốn ô kiểm như mục `luat-nam-giu-hoi-khach` đã mô tả |
| B | **"Số ngày xử lý" của đơn thư đếm ngày lịch, tính cả ngày nhận?** Kho này đang ghi đơn vị của *"trễ N ngày"* là **CÒN CHƯA RÕ** (ngày lịch hay giờ làm việc) ở mục `so-don-thu-cong-dan` | Bên kia: `8c1e2a7` `document-display.ts` `handlingDays`, dẫn *"cách báo cáo tiếp công dân đếm"*. Kho này: KPI *"Số ngày xử lý trung bình"* (`05-van-ban-don-thu.md:173`). ADR 0007 buộc **hạn** đếm giờ làm việc; đây là số ngày **đã trôi**, không phải hạn, nên chưa chắc cùng luật | Khách. BA trả lời một câu kho này đang để mở, nhưng câu trả lời của BA **không phải** câu trả lời của khách |
| C | **Cơ quan chủ trì ≡ cơ quan thực hiện, chuyên viên ≡ người thực hiện chính?** | Bên kia: `e1d0204` + migration `0051` (không đảo ngược được). Kho này: đặc tả tả hai cặp rời (`02-nhiem-vu.md:298-302`); lược đồ hai cặp cột (`0006_nhiem_vu.sql:269-270,287-288`); form web gửi hai cặp riêng (`nhan-nhiem-vu.ts:946-951`); bộ lọc chỉ đọc `nguoi_thuc_hien_ma` (`store/nhiem_vu.go:185-186`) | Chủ dự án / BA. Đây là **đổi đặc tả** của kho này, không phải sửa lỗi. Chưa quyết thì ít nhất màn hình không được để một việc đã giao chỉ ghi chuyên viên biến khỏi "Giao cho tôi" mà không ai biết |
| D | **Hạn nhiệm vụ rơi vào giờ nào trong ngày, và form có điền sẵn +7 ngày không?** | Bên kia: `b9a9718`, 17:00 và +7 ngày. Kho này: 23:59 +07:00, từ chối 17:00 vì giờ làm việc là của từng xã (`nhan-nhiem-vu.ts:624-639`, dẫn luật 1 bất biến 10); form để trống hạn. Điền sẵn "+7 ngày" bằng ngày lịch là phép tính giờ đồng hồ, đúng thứ luật 10 cấm #2 nếu đó là **hạn**; nếu chỉ là **gợi ý trên ô nhập** thì phải nói rõ | Khách. Nếu khách muốn "cuối giờ làm việc" thì phép tính thuộc `identity` (ADR 0007), không phải một hằng số ở trình duyệt |

## Không ảnh hưởng

| Commit / vùng | Vì sao bỏ |
|---|---|
| `233f57f` phần `docs/spec/03-mo-hinh-du-lieu.md` bảng `platform_users` · `04-api.md` số đường 253 → 271 · `10-kiem-thu.md` lệnh `uv run` · `README.md` số bảng · `apps/api/app/scripts/make_spec.py` | Tài liệu đuổi kịp mã của chính họ và công cụ sinh đặc tả. `platform_users` không do khoảng này thêm vào mô hình. Không luật nghiệp vụ nào mới |
| `06-giao-dien.md` `/huong-dan/[muc]` · `apps/admin/public/tai-lieu/huong-dan/**` · `guide-sections.ts` · `scripts/screenshots.mjs` | Sổ tay người dùng và ảnh chụp màn hình của kho họ |
| Mọi đường dẫn `/api/v1/petitions`, `/api/v1/documents`, `/api/v1/citizens`, `/api/v1/notices` | Tên tài nguyên URL cố ý lệch. Kho này đã chốt `citizen-letters` cho đơn thư (`ubiquitous-language.md:141`) |
| `e9011c2` · phần Mini App của `8c1e2a7`, `e1d0204` (`apps/miniapp/**`, `dossiers/**`) | Hồ sơ một cửa ngoài phạm vi (ADR 0001 §Bổ sung); danh tính công dân qua `X-Citizen-Id` là vùng cố ý lệch |
| `86c9f76` | Chỉ đổi nhãn menu và tiêu đề trang của hai loại thông báo. Phần luật đã ghi ở dòng M6 |
| `test_zalo_bot.py` · `test_task_assigner.py` · `test_citizen_dossiers.py` · `test_message_templates.py` | Bài kiểm Python của họ. Luật chúng kiểm đã ghi ở dòng commit tương ứng |

---

### Vì sao `anh_huong` là ba module này

| Module | Việc thật sự đòi đổi mã hoặc đổi thiết kế |
|---|---|
| `service-documents` | Hình dạng `nhat_ky_don_thu` (dòng không đổi trạng thái) phải chốt trước khi tạo bảng; tuyến danh sách `citizen-letters` thêm ba bộ lọc; sự kiện "đã giao cho cán bộ". Sổ đơn thư được dựng **ngay trong lượt `/develop-feature` Văn bản & đơn thư hiện tại** |
| `service-petitions` | Mâu thuẫn mặc định cờ ảnh (#7); bộ lọc "Giao cho tôi" bỏ sót việc chỉ ghi chuyên viên (mục C); mục A cho nhiệm vụ |
| `web-admin` | Tab đơn thư sắp vẽ: dải trạng thái toàn hành trình, ô nhật ký luôn mở, cột "Số ngày xử lý", ba bộ lọc; màn nhiệm vụ: mục C (hiện cả hai cặp) và mục D (giờ của hạn) |

**Cố ý không đưa vào:**
- `service-comms`: báo cán bộ khi được giao **chưa rõ** có phải việc của nó không. Đặc tả chuông của kho này (`08-thong-bao.md:161-172`) chưa có loại "được giao đơn thư", và kênh Zalo nhắc cán bộ đã là câu hỏi phạm vi từ 23/09. Chặn nó bây giờ sẽ chặn nhầm việc sự kiện `petitions → comms` đang chạy.
- `citizen-app`: chỉ bị chạm bởi tra cứu hồ sơ một cửa, mà một cửa nằm ngoài phạm vi.
- `service-identity`: không commit nào đổi SLA, lịch làm việc, bộ phận hay khoá quyền.

**Hai đường thoát, theo đúng thứ tự:** mở việc thật trong `kb/90-ephemeral/tien-do/<module>.json` và
trích tên tệp này trong `bang_chung`. Với `service-petitions`, việc cần mở cho §Mâu thuẫn là **hỏi**,
không phải chọn một mặc định rồi viết mã.
