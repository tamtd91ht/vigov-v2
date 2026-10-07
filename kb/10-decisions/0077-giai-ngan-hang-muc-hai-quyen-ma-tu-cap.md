---
id: 0077-giai-ngan-hang-muc-hai-quyen-ma-tu-cap
tier: T1
source: CURATED
owner: domain
derived_from_commit: ccef7ab3
expires: null
owns_facts:
  - "ghi danh mục hạng mục kế hoạch vốn (POST/PATCH/DELETE /api/v1/capital-plan-categories) nhận admin.lookup HOẶC budget.update qua authz.RequireAnyPermission; ba tuyến nhập Excel danh mục vẫn chỉ admin.lookup (chốt 07/10/2026)"
  - "thêm hạng mục không cần mã: máy chủ cấp mã kebab-case từ tên (DeriveCatalogueCode), trùng thì -2…-99, quá 99 thì 409 code_series_blocked thay vì đuôi hex ngẫu nhiên; không cấp lại mã đã dùng kể cả của dòng đã xoá mềm (chốt 07/10/2026)"
  - "xoá hạng mục: lý do không bắt buộc, để trống thì ghi câu cố định domain.CategoryRemovalDefaultReason; vẫn xoá mềm, có vết (chốt 07/10/2026)"
  - "danh sách dự án Giải ngân lệch đặc tả 06-giai-ngan theo lựa chọn người dùng: bấm cả dòng mở dự án; số tiền rút gọn trong cột; màu tiến độ theo ngưỡng 80/50/30 của prototype là luật HIỂN THỊ, cờ chậm của máy chủ vẫn là nguồn duy nhất của 'chậm' (chốt 07/10/2026)"
  - "hạn đã qua KHÔNG tô đỏ ở danh sách Giải ngân — cần đồng hồ trình duyệt (chốt 07/10/2026)"
---

# 0077. Giải ngân — danh mục hạng mục ghi bằng một trong hai quyền, mã tự cấp, lý do xoá tuỳ chọn, ba điểm lệch đặc tả ở danh sách

**Trạng thái:** đã chốt · **Ngày:** 2026-10-07 · **Người quyết:** chủ dự án, 07/10/2026, trong lượt
`/fix-web-admin --menu=giai-ngan` (*Hoàn thiện chức năng theo prototype*) · **Thay một phần** ADR 0075
#4b — xem §*Quan hệ với ADR 0075*.

## Bối cảnh

ADR 0075 #4b chốt ngày 06/10: nút **Hạng mục** ở màn Giải ngân hiện với `budget.update`, nhưng **ghi**
hạng mục vẫn cần `admin.lookup`. Hệ quả trên màn: kế toán mở được hộp thoại mà mọi thao tác ghi trả 403.

Một tuyến phục vụ hai màn, và hai nguồn nói hai khoá khác nhau:

| Nguồn | Khoá |
|---|---|
| Đặc tả `docs/ui-ux/14-cau-hinh.md:109` (màn Cấu hình) | `admin.lookup` |
| Prototype Giải ngân mở hộp thoại cho người có `budget.update` (`vigov-require/apps/admin` `BudgetWorkspace.tsx:64,154-156`) | `budget.update` |
| API của chính prototype (`router.py:252,264,295`) | `admin.lookup` |

Prototype tự mâu thuẫn: giao diện mở cửa, máy chủ đóng cửa. Lượt 07/10 hỏi chủ dự án chọn phía nào.

## Quyết định

| # | Điểm | Chốt | Phương án bị bác |
|---|---|---|---|
| 1 | Quyền ghi danh mục hạng mục | `POST` · `PATCH` · `DELETE /api/v1/capital-plan-categories` nhận **`admin.lookup` HOẶC `budget.update`**, qua bộ chặn lõi mới `authz.RequireAnyPermission` (`6af73b8f`, tuyến ở `e9f669f1`). **Ba tuyến nhập Excel** danh mục giữ **chỉ `admin.lookup`** | Giữ #4b của 0075 (nút hiện mà ghi bị 403) · đổi hẳn tuyến sang `budget.update` (màn Cấu hình mất quyền) · thêm khoá quyền mới (luật 5 bất biến 3c) |
| 2 | Mã hạng mục | Thêm **không cần mã**: máy chủ cấp mã kebab-case từ tên, như `slugify` của prototype (`apps/api/app/core/text.py:9-25`), dùng lại `domain.DeriveCatalogueCode` của đường nhập Excel. Trùng thì thử `-2` … `-99` như `_free_code` (`budget/service.py:269-279`); quá 99 thì **409 `code_series_blocked`**. **Không cấp lại** mã đã dùng, kể cả mã của dòng đã xoá mềm (luật 7 bất biến 3) | Đuôi hex ngẫu nhiên khi hết dãy, như prototype · bắt người dùng gõ mã |
| 3 | Lý do xoá hạng mục | **Không bắt buộc**; để trống thì máy chủ ghi câu cố định `domain.CategoryRemovalDefaultReason` vào `delete_reason`. Vẫn xoá mềm, có vết. Tiền lệ: ADR 0075 #4a (xoá dự án) | Lý do bắt buộc |
| 4 | Danh sách dự án — lệch `docs/ui-ux/06-giai-ngan.md` | (a) **Bấm cả dòng** mở dự án — đặc tả `:156` ghi *"Bấm tên"*. (b) **Số tiền rút gọn** trong cột — đặc tả `:146` cũng nói vậy. (c) **Màu tiến độ theo ngưỡng 80 / 50 / 30** của prototype (`budget-display.ts:62-67`; đặc tả `:79` chỉ ghi *"màu theo ngưỡng"*), đặt thành **một hằng số web** ở `web-admin/src/features/giai-ngan/progress-tone.ts`. Commit `4425e297` · `82771384` · `21177f29` | Theo đúng chữ đặc tả |
| 5 | Hạn đã qua tô đỏ | **Không làm** | Tô đỏ theo đồng hồ trình duyệt |

**Vì sao #1 là "một trong hai" chứ không đổi khoá:** hai màn có hai người dùng thật — cán bộ quản trị
danh mục ở Cấu hình, kế toán ở Giải ngân. Đổi khoá của tuyến là lấy quyền của một bên. Thêm khoá mới thì
không quản trị viên nào cấp được cho tới khi có migration gieo khoá (luật 5 bất biến 3c, câu mở #27).
`RequireAnyPermission` giữ cả hai khoá có sẵn trong bảng `quyen`.

**Vì sao nhập Excel không nới theo:** một lần nhập ghi nhiều dòng một lúc. Chủ dự án chỉ mở thao tác
từng dòng cho kế toán — đúng những gì hộp thoại của prototype làm.

**Vì sao #2 từ chối thay vì đuôi hex:** mã ngẫu nhiên không đọc ngược ra được tên hạng mục, và trăm dòng
cùng một tên là lỗi nhập liệu cần lộ ra, không cần nuốt. Dùng chung `DeriveCatalogueCode` với đường nhập
Excel để cùng một tên cho cùng một mã dù gõ tay hay nhập tệp; chỗ lệch prototype duy nhất là độ dài cắt.

**Vì sao #4c không phải luật chậm:** ngưỡng 80/50/30 chỉ đổi **màu** thanh tiến độ. Dự án có **chậm**
hay không vẫn chỉ do cờ `is_delayed` của máy chủ quyết, từ ngưỡng riêng của xã. Hai thứ nằm cạnh nhau
trên một dòng nhưng không phải một sự thật: đổi hằng số web là tô lại thanh, không bao giờ gắn nhãn
chậm cho dự án nào. Đổi luật chậm vẫn là điều kiện dừng (luật 10 điều kiện dừng #1).

**Vì sao #5 không làm:** "đã qua hạn" tính ở trình duyệt là dựa vào đồng hồ máy người dùng — lệch giờ
là sai màu, và đó là một nguồn thứ hai cho trạng thái quá hạn (luật 10 bất biến 3).

## Cái giá

- Kế toán chỉ giữ `budget.update` nay **thêm, đổi tên, tắt và xoá mềm** được hạng mục kế hoạch vốn của
  xã. Ranh giới "người quản trị danh mục / người nhập số liệu" không còn áp cho danh mục này.
- Mã tự cấp có thể chạm trần 99 với một tên lặp nhiều lần; người dùng phải tự gõ mã hoặc đổi tên.
- `delete_reason` của các lần xoá không nêu lý do đều mang cùng một câu — người tra vết phải đọc
  `deleted_by` và mục vết để biết ai, khi nào.
- Danh sách dự án lệch đặc tả `06-giai-ngan.md` ở #4a; người đọc đặc tả phải biết ADR này tồn tại.

## Quan hệ với ADR 0075

ADR 0075 không bị sửa nội dung. ADR này **thay riêng #4b**: nửa *"ghi hạng mục vẫn cần `admin.lookup`"*
nay là *"`admin.lookup` hoặc `budget.update`"*, trừ nhập Excel. Nửa *"nút Hạng mục hiện với
`budget.update`"* giữ. #4a của 0075 giữ, và là tiền lệ của #3 ở đây.

## Còn mở — tác tử đã chọn, chờ chủ dự án xác nhận

| # | Câu | Mã hôm nay |
|---|---|---|
| 1 | Câu chữ lý do mặc định khi xoá hạng mục | `"Xoá khỏi danh mục hạng mục kế hoạch vốn"` (`service-finance/internal/domain/danh_muc_ba_tang.go:232`) |
| 2 | Chống gửi trùng của `POST` thêm hạng mục khi Redis hỏng: cho qua (`MoKhiHong`) hay từ chối 503 (`DongKhiHong`) | `MoKhiHong` (`service-finance/internal/http/routes.go:938-943`): dòng sinh đôi không mang tiền, không mang số văn bản, xoá mềm được ngay trên hộp thoại |

→ ADR 0075 #4a, #4b · ADR 0068 lần 5
→ Đặc tả: `docs/ui-ux/06-giai-ngan.md` `:79`, `:146`, `:156` · `docs/ui-ux/14-cau-hinh.md:109`
→ Sổ tiến độ: `python tools/tien_do.py --menu giai-ngan`
