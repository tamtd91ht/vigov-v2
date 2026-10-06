/**
 * Đọc danh sách quyền của phiên hiện tại để **ẩn/hiện** giao diện.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * LỚP NÀY LÀ TIỆN DỤNG, KHÔNG PHẢI BIỆN PHÁP.
 *
 * Kiểm quyền ở giao diện thay cho tầng dịch vụ là không kiểm gì cả — mã client sửa được, và
 * một cán bộ tò mò không cần sửa gì: chỉ cần gọi thẳng `/api/v1/staff` bằng chính cookie
 * phiên của mình (luật 5, cấm #1). Cái chặn thật là khai báo `RequirePermission("admin.user")`
 * trên tuyến, ở phía máy chủ, trên **từng** yêu cầu.
 *
 * Ẩn một tab vì vậy chỉ để cán bộ không phải bấm vào một thứ chắc chắn trả 403. Nếu có ngày
 * hàm này trả `true` sai, hậu quả là một màn hình hiện lỗi 403 — không phải một lần dữ liệu
 * rời khỏi máy chủ.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 */

import type { KetQua } from "./api/goi";
import type { identity_phienHienTaiRa } from "./api/schema.gen";

/**
 * Khoá quyền của tab "Người dùng" — `docs/ui-ux/14-cau-hinh.md §12.8`: tab nào thiếu quyền thì
 * ẩn tab đó. Cùng một chuỗi mà máy chủ đòi trên hai tuyến danh bạ (`x-vigov-permission` trong
 * `kb/20-contracts/openapi.json`), nên hai bên không thể lệch nhau mà không ai thấy.
 */
export const QUYEN_QUAN_LY_NGUOI_DUNG = "admin.user";

/**
 * Khoá quyền của tab "Phân quyền" — `docs/ui-ux/14-cau-hinh.md §12.8`, cùng một quy tắc: tab nào
 * thiếu quyền thì ẩn tab đó. Cùng chuỗi mà máy chủ đòi trên `GET /api/v1/role-permissions`
 * (`x-vigov-permission` trong `kb/20-contracts/openapi.json`).
 *
 * KHÔNG PHẢI `admin.user`, và hai khoá này không suy ra nhau: người quản lý danh bạ cán bộ chưa
 * chắc được xem ai đang giữ khoá nào, và ngược lại. Đó chính là lý do bộ quyền được liệt kê từng
 * khoá một (luật 5, bất biến 3b).
 */
export const QUYEN_PHAN_QUYEN = "admin.role";

/**
 * Khoá quyền của màn "Theo dõi giải ngân" — `budget.read`.
 *
 * KHÔNG GÕ TAY TỪ ĐẶC TẢ: đây đúng chuỗi máy chủ khai trên cả hai tuyến dự án
 * (`x-vigov-permission.key` của `GET /api/v1/investment-projects` và `.../{id}` trong
 * `kb/20-contracts/openapi.json`, sinh từ `service-finance/internal/http/routes.go`).
 *
 * `budget.read` MỞ MÀN ĐỌC, KHÔNG MỞ GÌ KHÁC. Đặc tả §8.2 liệt kê `budget.update` (nhập, sửa
 * chứng từ) và `budget.confirm` (xác nhận, khoá) là hai khoá RIÊNG, và không khoá nào trong ba
 * suy ra được từ khoá kia.
 *
 * ⚠ CÂU "chúng không có tuyến nào trong hợp đồng hôm nay" ĐÃ SAI TỪ 23/09/2026 — gỡ đi rồi để
 * lại dấu vết này, vì một chú thích nói sai về phân quyền là thứ người sau đọc rồi dựng cổng
 * theo. Tám tuyến ngân sách nay khai đủ ba khoá; xem hai hằng ngay dưới.
 */
export const QUYEN_XEM_GIAI_NGAN = "budget.read";

/**
 * Nhập và sửa — `budget.update`. Tuyến khai nó (`x-vigov-permission.key` trong
 * `kb/20-contracts/openapi.json`): `POST /budget-sheets` · `POST /budget-lines` ·
 * `PATCH /budget-lines/{id}` · `POST /disbursements` · `PATCH /disbursements/{id}`.
 */
export const QUYEN_GHI_NGAN_SACH = "budget.update";

/**
 * Xác nhận, khoá, GỠ, và chọn dòng tổng — `budget.confirm`.
 *
 * XOÁ ĐỨNG SAU KHOÁ NÀY, KHÔNG SAU `budget.update`, và đó là chỗ dễ gắn cổng nhầm nhất:
 * `DELETE /budget-sheets/{id}` gỡ cả một bảng ngân sách, `DELETE /budget-lines/{id}` gỡ một
 * khoản mục, và `POST /budget-lines/{id}/headline` chọn dòng nào là CON SỐ TỔNG của cả bảng —
 * ba thao tác đổi thứ lãnh đạo đã đọc, nên chúng đi cùng quyền xác nhận chứ không cùng quyền
 * nhập liệu. Cũng là khoá của `POST/DELETE /disbursements/{id}/lockout`.
 */
export const QUYEN_XAC_NHAN_NGAN_SACH = "budget.confirm";

/**
 * Khoá quyền của màn "Phản ánh của người dân" — `feedback.read`, cùng chuỗi máy chủ khai trên
 * `GET /api/v1/citizen-reports/{maTraCuu}`.
 *
 * KHÔNG PHẢI `feedback.restricted`. Khoá thứ hai ấy mở riêng lĩnh vực `can-bo` — phản ánh VỀ
 * tác phong cán bộ — và máy chủ cố ý trả **404** cho người thiếu nó, đúng câu trả lời mà một
 * mã không tồn tại nhận được (`service-petitions/internal/http/phieu_phan_anh.go`). Không có
 * cổng nào ở giao diện cho khoá ấy, và cũng không được dựng: một cổng ở đây sẽ nói ra sự tồn
 * tại của đúng những phiếu máy chủ vừa giấu đi.
 */
export const QUYEN_XEM_PHAN_ANH = "feedback.read";

/**
 * `feedback.classify` — CHỐT LĨNH VỰC, và nó **không** suy ra từ `feedback.assign`.
 *
 * Hai quyền cố ý không phải một (ADR 0030; luật 5 bất biến 3b): chốt lĩnh vực là hành vi ẤN ĐỊNH
 * hạn xử lý xong, tức **hứa thay cơ quan** rằng việc này xong trong bao lâu. Được giao việc không
 * phải là được hứa thay cơ quan.
 */
export const QUYEN_PHAN_LOAI_PHAN_ANH = "feedback.classify";

/** `feedback.assign` — chuyển phiếu cho bộ phận xử lý (`docs/ui-ux/09` §8.5). */
export const QUYEN_PHAN_CONG_PHAN_ANH = "feedback.assign";

/**
 * `feedback.resolve` — **KẾT THÚC** xử lý phản ánh. Cổng của nút `Đóng phiếu`, và **chỉ** nút ấy.
 *
 * ⚠ KHÔNG DÙNG KHOÁ NÀY CHO NÚT TIẾN TRẠNG THÁI. Tuyến `…/status` khai `feedback.read`, còn điều
 * kiện thật là `feedback.resolve` **HOẶC** chính là cán bộ được phân công phiếu ấy — luật nắm giữ
 * (`app.duocTienTrangThai`). Trưởng thôn nhận một phiếu phải tiến được phiếu ấy dù không có khoá
 * toàn xã; gắn nút ấy sau `feedback.resolve` là lấy mất đúng điều luật nắm giữ mở ra.
 *
 * VÀ CHIỀU NGƯỢC LẠI CŨNG KHÔNG: `…/closure` **không** được nới theo luật nắm giữ. Đóng phiếu ghi
 * một kết quả **người dân đọc** (luật 10 bất biến 6), và câu hỏi mở #7 chốt 16/09/2026:
 * *"`feedback.resolve` quyết định ai đóng được"*. Hai nút, hai cổng khác nhau.
 */
export const QUYEN_DONG_PHAN_ANH = "feedback.resolve";

/**
 * Khoá quyền của ba thao tác GHI ở tab "Danh mục" — `admin.lookup`.
 *
 * KHÔNG GÕ TAY TỪ ĐẶC TẢ: đúng chuỗi máy chủ khai trên cả mười lăm tuyến ghi danh mục
 * (`x-vigov-permission.key` của `POST/PATCH/DELETE` năm danh mục trong `kb/20-contracts/
 * openapi.json`), và đúng chuỗi migration gieo vào bảng `quyen`
 * (`service-identity/migrations/0001_init.sql:274` — "Quản lý danh mục"). Một khoá bảng `quyen`
 * không có là một khoá không quản trị viên nào cấp được, tức tuyến ấy 403 với MỌI tài khoản
 * (luật 5, bất biến 3c).
 *
 * KHOÁ NÀY KHÔNG CHE PHẦN ĐỌC, và sự bất đối xứng ấy đến từ máy chủ chứ không từ giao diện: bảy
 * tuyến ĐỌC danh mục khai `any-authenticated` vì nhãn danh mục xuất hiện ở ô chọn và bộ lọc của
 * gần như mọi màn hình. Dùng khoá này để ẩn cả bảng sẽ là giao diện từ chối điều máy chủ đang
 * phục vụ bình thường — xem lý lẽ đầy đủ ở `features/cau-hinh/tab-danh-muc.tsx`.
 */
export const QUYEN_QUAN_LY_DANH_MUC = "admin.lookup";

/**
 * Khoá quyền của hai thao tác GHI ở tab "Sơ đồ tổ chức" — `admin.org`, "Quản lý sơ đồ tổ chức".
 *
 * KHÔNG GÕ TAY TỪ ĐẶC TẢ: đúng chuỗi máy chủ khai trên `POST /api/v1/org-units` và
 * `PATCH /api/v1/org-units/{id}` (`x-vigov-permission.key` trong `kb/20-contracts/openapi.json`),
 * và đúng chuỗi migration gieo vào bảng `quyen` (`service-identity/migrations/0001_init.sql:282`).
 *
 * KHOÁ NÀY KHÔNG CHE PHẦN ĐỌC — cùng bất đối xứng với `admin.lookup` ngay trên, và cũng do máy chủ
 * đặt: `GET /api/v1/org-units` khai `any-authenticated` vì tên bộ phận có ở ô phân công và bộ lọc
 * của mọi màn. Cây vì vậy hiện cho mọi tài khoản; chỉ nút `Thêm` và `Sửa` ẩn theo khoá này.
 *
 * KHÔNG SUY RA TỪ `admin.lookup`: người sửa được danh mục nghiệp vụ chưa chắc được dựng lại bộ máy
 * của đơn vị, và ngược lại (luật 5, bất biến 3b).
 */
export const QUYEN_QUAN_LY_SO_DO = "admin.org";

/**
 * Khoá quyền của mọi thao tác GHI trên tab Thời hạn xử lý & Lịch làm việc — `admin.sla`,
 * "Cấu hình thời hạn xử lý".
 *
 * KHÔNG GÕ TAY TỪ ĐẶC TẢ: đúng chuỗi máy chủ khai trên cả mười bốn tuyến ghi
 * (`x-vigov-permission.key` trong `kb/20-contracts/openapi.json`) và đúng chuỗi migration gieo vào
 * bảng `quyen` (`service-identity/migrations/0001_init.sql:277`).
 *
 * MỘT KHOÁ CHO CẢ BẢNG THỜI HẠN LẪN BA BẢNG LỊCH, và đó là quyết định của MÁY CHỦ chứ không phải
 * một lần gộp cho gọn ở đây: lịch làm việc là thứ số giờ trong bảng thời hạn được ĐẾM THEO, nên
 * một người sửa được cột "40 giờ" mà không sửa được giờ làm việc thì vẫn đổi được hạn thật của
 * mọi hồ sơ — chỉ bằng đường vòng.
 *
 * BẤT ĐỐI XỨNG ĐỌC/GHI, và nó KHÔNG giống danh mục: ba tuyến đọc lịch khai `any-authenticated`
 * (lịch đọc để VẼ), nhưng `GET /api/v1/sla` thì đòi chính khoá này (bảng ấy đọc để CẤU HÌNH).
 * Nên khoá này che phần ghi của cả bốn bảng, và che luôn phần đọc của riêng bảng thời hạn — việc
 * che ấy do máy chủ làm, giao diện chỉ hiện nguyên văn câu 403 của nó.
 */
export const QUYEN_CAU_HINH_THOI_HAN = "admin.sla";

/**
 * Khoá quyền của mọi thao tác GHI trên hai quyển sổ văn bản — `document.create`, "Vào sổ văn bản".
 *
 * KHÔNG GÕ TAY TỪ ĐẶC TẢ: đúng chuỗi máy chủ khai trên cả sáu tuyến ghi của hai sổ
 * (`x-vigov-permission.key` của `POST/PATCH/DELETE` hai bộ tuyến trong `kb/20-contracts/
 * openapi.json`) và đúng chuỗi migration gieo vào bảng `quyen`
 * (`service-identity/migrations/0001_init.sql:287`). Một khoá bảng `quyen` không có là một khoá
 * không quản trị viên nào cấp được, tức tuyến ấy 403 với MỌI tài khoản (luật 5, bất biến 3c).
 *
 * MỘT KHOÁ CHO CẢ SÁU TUYẾN, KỂ CẢ HAI TUYẾN GỠ, VÀ ĐÓ LÀ MỘT PHÁT HIỆN ĐÃ ĐƯỢC GHI Ở MÁY CHỦ
 * CHỨ KHÔNG PHẢI MỘT LẦN GỘP CHO GỌN Ở ĐÂY: bảng `quyen` không có `document.update` hay
 * `document.delete`, nên tuyến gỡ đứng sau khoá gần nhất là khoá vào sổ. Một xã hoàn toàn có thể
 * muốn người vào sổ KHÔNG gỡ được — đó là một quyền khác, và nó là câu hỏi mở #27, không phải một
 * dòng `INSERT INTO quyen` (`service-documents/internal/http/van_ban_den.go:11`).
 */
export const QUYEN_GHI_SO_VAN_BAN = "document.create";

/**
 * Khoá quyền của riêng thao tác CHUYỂN XỬ LÝ — `document.route`, "Phân luồng văn bản".
 *
 * KHÔNG SUY RA TỪ `document.create`, và sự tách ấy là của đặc tả (§7 quy tắc 4) rồi của máy chủ
 * (`routes.go`, `POST .../routings`): chuyển một văn bản cho bộ phận nào là hành vi quyết định AI
 * CHỊU TRÁCH NHIỆM, không phải hành vi gõ một văn bản vào sổ. Một xã giao việc nhập cho văn thư và
 * giữ việc phân luồng cho lãnh đạo là cách làm bình thường, nên hai khoá không được gộp ở giao
 * diện dù chúng thường cấp cùng nhau (luật 5, bất biến 3b).
 */
export const QUYEN_CHUYEN_VAN_BAN = "document.route";

/**
 * Khoá quyền XEM hai quyển sổ văn bản — `document.read`, "Xem văn bản"
 * (`service-identity/migrations/0001_init.sql:295`), đúng chuỗi `x-vigov-permission.key` của
 * `GET /api/v1/incoming-documents` và `GET /api/v1/outgoing-documents` trong
 * `kb/20-contracts/openapi.json`.
 *
 * CHỈ DÙNG CHO MỤC MENU, KHÔNG BAO GIỜ LÀ CỔNG THÂN MÀN. Thân màn `/van-ban` cố ý KHÔNG dựng cổng
 * quyền ở client: tài khoản thiếu khoá nhận 403 ngay ở lượt đọc và màn hình hiện NGUYÊN câu của
 * máy chủ — cùng khuôn với `GET /api/v1/sla` ở tab Thời hạn xử lý. Dựng thêm một cổng đoán trước
 * điều ấy chỉ thêm một chỗ có thể lệch với máy chủ, và khi nó lệch thì nó ẩn mất một quyển sổ mà
 * máy chủ đang phục vụ bình thường. Lý do mục menu vẫn cần một khoá nằm ở khối `task.read` dưới.
 *
 * VÌ SAO KHÔNG DÙNG `QUYEN_CHUYEN_VAN_BAN` CHO MỤC MENU (như trước 24/09/2026): `document.route`
 * là khoá GHI, không phải khoá tuyến đọc khai. Canh bằng nó thì cán bộ chỉ có `document.read`
 * không thấy nổi quyển sổ máy chủ sẵn sàng trả cho họ, còn người chỉ có `document.route` thấy một
 * mục dẫn thẳng vào 403.
 */
export const QUYEN_XEM_VAN_BAN = "document.read";

/**
 * Khoá quyền XEM sổ nhiệm vụ — `task.read`, "Xem nhiệm vụ"
 * (`service-identity/migrations/0001_init.sql:311`).
 *
 * MỘT HẰNG CHO MỘT KHOÁ ĐỌC — CÙNG KHUÔN `QUYEN_XEM_VAN_BAN` NGAY TRÊN: không cổng trong
 * THÂN MÀN, chỉ canh mục menu. Hằng này không dùng cho thân màn: `/nhiem-vu` và
 * `/nhiem-vu/bien-ban` đều KHÔNG có `<CongQuyen>`, tài khoản thiếu khoá vẫn nhận 403 nguyên văn
 * từ máy chủ, đúng khuôn `/van-ban`. Nó chỉ quyết định MỤC MENU có hiện hay không.
 *
 * MỘT NGOẠI LỆ CÓ CHỦ Ý (04/10/2026): thân `/nhiem-vu/so-tay` BỌC `<CongQuyen>` bằng khoá này. Màn ấy
 * đọc tới tám lần các tuyến nhiệm vụ cùng lúc và MỌI tuyến ấy khai đúng `task.read`, nên cổng không thể
 * giấu thứ máy chủ đang phục vụ — còn không cổng thì người thiếu khoá đọc một loạt câu 403 giống hệt nhau.
 *
 * VÌ SAO MỤC MENU CẦN MỘT KHOÁ, VÀ VÌ SAO PHẢI LÀ ĐÚNG KHOÁ NÀY: `muc-menu.ts` đã từ chối vẽ
 * chín mục chưa có màn thành liên kết, vì "vẽ ra thứ không bấm được là hứa một chức năng không
 * tồn tại". Một mục menu dẫn thẳng vào 403 là cùng lời hứa ấy. Nhưng khoá canh mục menu phải là
 * ĐÚNG khoá tuyến đọc khai, không phải một khoá gần đúng: mục `Văn bản & Đơn thư` từng canh bằng
 * `document.route` — một khoá GHI — và đã phải đổi sang `document.read` (24/09/2026). Lấy
 * `task.create` canh mục Nhiệm vụ sẽ chép lại đúng lỗi ấy.
 *
 * CÁC KHOÁ `task.*` GHI CÓ HẰNG RIÊNG NGAY DƯỚI (27/09/2026), vì màn `/nhiem-vu` nay canh nút
 * theo chúng. `task.assign` có hằng từ 28/09/2026 (`TASK_ASSIGN_PERMISSION`), đúng ngày tuyến
 * giao lại `POST /api/v1/tasks/{ma}/assignment` ra đời (764bb92) — trước đó nó cố ý không có hằng,
 * vì một hằng không nút nào dùng là một hằng không ai thấy khi nó sai.
 */
export const QUYEN_XEM_NHIEM_VU = "task.read";

/**
 * `task.create` — "Tạo nhiệm vụ" (`0001_init.sql:308`). Cổng của nút `+ Giao việc mới`.
 *
 * KHÔNG GÕ TAY TỪ ĐẶC TẢ: đúng chuỗi máy chủ khai trên `POST /api/v1/tasks` (`x-vigov-permission.key`
 * trong `kb/20-contracts/openapi.json`). Cũng là khoá của mọi tuyến ghi biên bản họp trừ tuyến ký.
 *
 * ẨN NÚT LÀ TIỆN DỤNG, KHÔNG PHẢI BIỆN PHÁP — máy chủ kiểm khoá này trên từng lời gọi (luật 5, cấm #1).
 */
export const QUYEN_TAO_NHIEM_VU = "task.create";

/**
 * `task.update` — "Cập nhật tiến độ" (`0001_init.sql:312`). Máy chủ khai nó trên BA tuyến:
 * `PATCH /api/v1/tasks/{ma}` (nút `✎ Sửa`), `POST …/status` (khối Chuyển trạng thái) và
 * `POST …/extensions` (GỬI đề nghị lùi hạn).
 *
 * GỬI ĐỀ NGHỊ LÙI HẠN ĐỨNG SAU KHOÁ NÀY, KHÔNG SAU `task.extend` — đó là toàn bộ ADR 0038: người
 * đang làm việc là người xin lùi hạn; gắn ô ấy sau khoá duyệt là "chỉ người duyệt được mới xin được".
 */
export const QUYEN_CAP_NHAT_NHIEM_VU = "task.update";

/**
 * `task.delete` — "Xoá nhiệm vụ khỏi sổ" (`0001_init.sql:309`), khoá của `DELETE /api/v1/tasks/{ma}`.
 * Xoá mềm, kèm lý do bắt buộc; mã sổ đã cấp không bao giờ cấp lại (luật 7).
 */
export const QUYEN_XOA_NHIEM_VU = "task.delete";

/**
 * `task.extend` — "Duyệt gia hạn" (`0001_init.sql:310`), khoá của
 * `POST /api/v1/tasks/{ma}/extensions/{deNghiID}/decision`.
 *
 * ⚠ KHOÁ NÀY MỚI LÀ LỚP MỘT. Có khoá chưa có nghĩa là duyệt được một đề nghị cụ thể: lớp hai của
 * ADR 0038 — đúng người ghi ở `lanh_dao_giao_viec_ma` của CHÍNH nhiệm vụ ấy — vẫn chạy ở
 * `quyetDinhDuyetLuiHan` và ở máy chủ. Bỏ lớp hai thì mọi lãnh đạo cầm khoá này duyệt được mọi
 * nhiệm vụ của cả xã, vì luật 5 kiểm `(tenant_id, role, permission)` và không có chiều "bản ghi nào".
 */
export const QUYEN_DUYET_GIA_HAN = "task.extend";

/**
 * `task.assign` (`service-identity/migrations/0001_init.sql:307`) — the key of
 * `POST /api/v1/tasks/{ma}/assignment` (764bb92): hand the SAME task to another unit or person
 * ("Chuyển tiếp", owner decision 28/09/2026) and edit lead unit / monitor. Gate of the drawer's
 * §5.7 block `Giao việc, chuyển việc`.
 *
 * NOT `task.update`: deciding who holds a piece of work is not updating its progress. Folding it
 * into `task.update` would hand every officer who logs progress the power to move work to another
 * department — the table seeds a separate key precisely to keep those apart (rule 5, inv. 3b).
 *
 * English name (rule 12); the constants above are Vietnamese and stay so.
 *
 * HIDING IS CONVENIENCE, NOT PROTECTION — the server checks this key on every call (rule 5, #1).
 */
export const TASK_ASSIGN_PERMISSION = "task.assign";

/**
 * `task.approve` — "Duyệt hoàn thành" (`0001_init.sql:306`). HAI chỗ dùng, cùng một nghĩa: hành vi
 * làm một hồ sơ trở thành CUỐI CÙNG.
 *
 *   `POST /api/v1/meetings/{id}/signature`           nút `Ký biên bản` — khoá của chính tuyến
 *   `POST /api/v1/tasks/{ma}/status` → `hoan-thanh`  bước cuối §6 — tuyến khai `task.update`, và
 *                                                    máy chủ đòi THÊM khoá này cho riêng bước ấy
 *                                                    (`service-petitions/internal/http/nhiem_vu_ghi.go`)
 *
 * KHÔNG PHẢI `task.create`: người gõ biên bản chưa chắc là người được xác nhận nó — ký là hành vi
 * KHOÁ một hồ sơ lưu trữ, sau đó mọi sai sót phải đi đường biên bản bổ sung (luật 5, bất biến 3b).
 * Và KHÔNG PHẢI `task.update`: cán bộ làm việc không tự duyệt hoàn thành việc của chính mình.
 *
 * ẨN NÚT LÀ TIỆN DỤNG, KHÔNG PHẢI BIỆN PHÁP — máy chủ kiểm khoá này trên từng lời gọi (luật 5, cấm #1).
 */
export const QUYEN_KY_BIEN_BAN = "task.approve";

/**
 * Cùng khoá `task.approve` với `QUYEN_KY_BIEN_BAN`, gọi theo nghĩa ở màn Nhiệm vụ. HAI TÊN, MỘT
 * CHUỖI — không phải hai chuỗi: bảng `quyen` chỉ có một dòng "Duyệt hoàn thành", và tên hằng đọc
 * theo chỗ dùng là để người đọc nút `Chuyển sang Hoàn thành` không phải hỏi vì sao nó canh bằng
 * khoá "ký biên bản".
 */
export const QUYEN_DUYET_HOAN_THANH_NHIEM_VU = QUYEN_KY_BIEN_BAN;

/**
 * Khoá quyền của sổ Thông báo nội bộ — `announcement.create`, "Soạn và gửi thông báo"
 * (`service-identity/migrations/0001_init.sql:286`).
 *
 * TÊN HẰNG NÓI `SOẠN` NHƯNG NÓ CŨNG CANH ĐƯỜNG ĐỌC, và điều ấy KHÔNG phải lỗi lặp lại của mục
 * `Văn bản & Đơn thư`. Ở đó mục menu từng canh bằng `document.route` trong khi tuyến đọc đòi
 * `document.read` — hai khoá khác nhau, nên cán bộ chỉ có khoá đọc mất luôn lối vào. Ở đây
 * `GET /api/v1/announcements` VÀ `POST` **cùng đòi đúng một khoá này**, vì nhóm THÔNG BÁO trong
 * bảng `quyen` chỉ có một dòng duy nhất. Nên canh mục menu bằng nó là canh bằng CHÍNH khoá tuyến
 * đọc khai — không có khoảng lệch nào để trôi vào.
 *
 * ĐÂY LÀ CẤP THIẾU, KHÔNG PHẢI CẤP THỪA, và chỗ thiếu ấy là một khiếm khuyết đã ghi: không có
 * khoá ĐỌC nghĩa là một cán bộ bình thường không xem được thông báo gửi cho chính mình, tức bộ
 * lọc `Gửi cho tôi` của §2 không ship được. Sổ tiến độ `service-comms/khoa-doc-so-thong-bao-chua-co`
 * giữ hai lối thoát; luật 5 bất biến 3c cấm tự thêm một khoá vào bảng, nên nó chờ người quyết.
 * Ngày khoá đọc ra đời thì mục menu đổi sang khoá ấy, và hằng này lui về đúng nghĩa tên nó.
 */
export const QUYEN_SOAN_THONG_BAO = "announcement.create";

/**
 * Khoá quyền XEM sổ Nội dung Mini App — `content.read`, "Xem nội dung Mini App"
 * (`service-identity/migrations/0001_init.sql:292`).
 *
 * ĐÚNG khoá tuyến đọc khai, không phải một khoá gần đúng — cùng lập luận với `QUYEN_XEM_NHIEM_VU`
 * ngay trên. Dùng ở đúng MỘT chỗ: mục menu. Màn `/noi-dung` không bọc `<CongQuyen>`, tài khoản
 * thiếu khoá vẫn nhận 403 nguyên văn từ máy chủ.
 *
 * `content.update` NAY CÓ HẰNG RIÊNG ngay dưới (24/09/2026), vì đã có chỗ ở client canh nó.
 */
export const QUYEN_XEM_NOI_DUNG = "content.read";

/**
 * Khoá quyền của nút công khai / rút một cán bộ khỏi danh bạ Zalo Mini App — `content.update`,
 * "Sửa nội dung và danh bạ Mini App".
 *
 * KHÔNG GÕ TAY TỪ ĐẶC TẢ: đúng chuỗi máy chủ khai trên `PUT /api/v1/staff/{id}/publication`
 * (`x-vigov-permission.key` trong `kb/20-contracts/openapi.json`) và đúng chuỗi migration gieo vào
 * bảng `quyen` (`service-identity/migrations/0001_init.sql:293`).
 *
 * KHÔNG PHẢI `admin.user`, dù màn `/danh-ba` mở bằng `admin.user`: người sửa hồ sơ cán bộ chưa chắc
 * được đưa số di động cá nhân của ai ra kênh công khai, và câu mở #12 đặt việc ấy dưới khoá nội
 * dung Mini App chứ không dưới khoá quản lý người dùng (luật 5, bất biến 3b). Tài khoản có
 * `admin.user` mà thiếu khoá này vẫn xem được cột "Trên Mini App", chỉ không thấy hai nút.
 *
 * ẨN NÚT LÀ TIỆN DỤNG, KHÔNG PHẢI BIỆN PHÁP — máy chủ kiểm khoá này trên từng lời gọi (luật 5, cấm #1).
 */
export const QUYEN_CONG_KHAI_DANH_BA = "content.update";

/**
 * Khoá quyền của nút xoá một dòng danh bạ NHẬP TRÙNG — `admin.user.delete`, "Xoá dòng danh bạ nhập
 * trùng".
 *
 * KHÔNG GÕ TAY TỪ ĐẶC TẢ: đúng chuỗi máy chủ khai trên `DELETE /api/v1/staff/{id}`
 * (`x-vigov-permission.key` trong `kb/20-contracts/openapi.json`) và đúng chuỗi migration gieo vào
 * bảng `quyen` (`service-identity/migrations/0010_danh_ba_mini_app_va_khoa_xoa_dong_trung.sql:267`,
 * thứ tự 36). Migration ấy KHÔNG cấp khoá cho vai trò nào: cho tới khi quản trị viên của xã tick nó
 * ở màn Phân quyền, nút này ẩn với mọi tài khoản — và đó là chủ ý.
 *
 * KHÔNG SUY RA TỪ `admin.user` (câu mở #10, ADR 0035): sửa hồ sơ và khoá tài khoản là một việc, gỡ
 * một dòng khỏi danh bạ là việc khác. Cán bộ nghỉ hưu hay chuyển công tác thì KHOÁ, không xoá —
 * hồ sơ đã xử lý phải còn đọc được tên người thực hiện (luật 5, bất biến 3b).
 *
 * ẨN NÚT LÀ TIỆN DỤNG, KHÔNG PHẢI BIỆN PHÁP — máy chủ kiểm khoá này trên từng lời gọi (luật 5, cấm #1).
 */
export const QUYEN_XOA_DONG_DANH_BA = "admin.user.delete";

/**
 * `report.read` — "Xem báo cáo" (`service-identity/migrations/0001_init.sql:305`). The gate of the
 * leadership overview `/tong-quan`, its menu item, and HALF of every block's gate on that page.
 *
 * HALF, NOT ALL: each figure route checks `report.read` AND the read key of the module it counts —
 * `task.read`, `document.read`, `feedback.read` (the `routes.go` of `service-petitions` and
 * `service-documents`), while `x-vigov-permission` in openapi shows only one of the two. A block is
 * therefore shown only when BOTH keys are held (`features/dashboard/figures.ts` `canSeeBlock`):
 * `report.read` alone must not surface task figures to an account that cannot read one task.
 *
 * English name (rule 12): the older constants above are Vietnamese and stay so.
 *
 * HIDING IS CONVENIENCE, NOT PROTECTION — the server checks both keys on every call (rule 5, #1).
 */
export const REPORT_READ_PERMISSION = "report.read";

/**
 * `report.export` — "Xuất báo cáo" (`service-identity/migrations/0001_init.sql:304`). `/bao-cao`
 * draws its export buttons only for an account holding it (spec 13 §9.4); its export is not built
 * (ADR 0053, amendment 04/10/2026, B4), so there the key reveals three disabled buttons.
 *
 * `/tong-quan`'s export IS built (06/10/2026): files made in the browser from the figures on screen,
 * enabled only with this key (`features/dashboard/export-actions.tsx`). No server route sees that
 * export, so nothing server-side checks the key or audits the act yet — convenience, not protection.
 */
export const REPORT_EXPORT_PERMISSION = "report.export";

/**
 * `asset.read` — "Xem bản đồ tài nguyên" (`service-identity/migrations/0001_init.sql:287`). The READ
 * key of `GET /api/v1/map-field-schemas` (`x-vigov-permission` in `kb/20-contracts/openapi.json`),
 * and so the gate of the whole "Trường bản đồ" tab: without it the list answers 403 and the tab has
 * nothing to show.
 *
 * NOT `admin.lookup`: the four WRITE routes of that tab declare `admin.lookup`, so the tab gates its
 * buttons on `QUYEN_QUAN_LY_DANH_MUC` and its visibility on this key — two keys, never one inferred
 * from the other (rule 5, invariant 3b).
 */
export const ASSET_READ_PERMISSION = "asset.read";

/**
 * `admin.audit` — "Xem nhật ký hệ thống" (`service-identity/migrations/0001_init.sql:280`). The key
 * of all five `GET /api/v1/<service>-audit-entries` routes (`x-vigov-permission` in
 * `kb/20-contracts/openapi.json`, ADR 0054 §1), and so the gate of the whole "Nhật ký hệ thống" tab:
 * without it every read answers 403 and the tab has nothing to show.
 *
 * NOT implied by any other `admin.*` key, and implies none (rule 5, invariant 3b) — reading the
 * trail of who did what is not managing accounts, and managing accounts is not reading the trail.
 *
 * HIDING IS CONVENIENCE, NOT PROTECTION — each service checks this key on every call (rule 5, #1).
 */
export const AUDIT_READ_PERMISSION = "admin.audit";

/**
 * `feedback.unmask` — "Xem đầy đủ họ tên và số điện thoại người gửi"
 * (`service-identity/migrations/0007_quyen_phan_loai_va_xem_day_du.sql:59`). No screen gates on it
 * (the server decides what it returns); it is named here only because the Phân quyền tab warns when
 * no role but the default administrator holds it (ADR 0055 §Cái giá).
 */
export const CITIZEN_REPORT_UNMASK_PERMISSION = "feedback.unmask";

/**
 * Quyết định một phần giao diện có hiện hay không — BA trạng thái, không hai.
 *
 * TỪNG NẰM RIÊNG TRONG `features/cau-hinh/quyen-tab.ts` VÀ NAY Ở ĐÂY, vì nó có người dùng thứ
 * hai: hai màn hình mới (`/giai-ngan`, `/phan-anh`) cần đúng phép quyết định ấy. Chép lại nhánh
 * FAIL CLOSED sang một tệp thứ hai là chép lại đúng nhánh KHÔNG ai nhìn thấy khi nó sai — ngày
 * một bản đổi, bản kia vẫn xanh (luật 9, cấm #2). `quyen-tab.ts` nay gọi vào đây.
 */
export type QuyetDinhHien =
  | { hien: true }
  /** Đọc được quyền, và tài khoản không có khoá ấy. */
  | { hien: false; vi: "khong-du-quyen" }
  /** Không đọc được quyền — phiên hết hạn, mạng hỏng, máy chủ lỗi. Vẫn là ẩn. */
  | { hien: false; vi: "khong-doc-duoc"; thongBao: string };

/**
 * Có hiện phần giao diện gắn với **một** khoá quyền hay không.
 *
 * NHẬN ĐÚNG MỘT KHOÁ, KHÔNG NHẬN DANH SÁCH. Một hàm công khai nhận danh sách khoá là hàm mời
 * người gọi truyền hai khoá vào rồi mở màn khi có bất kỳ khoá nào — và `admin.audit` sẽ mở màn
 * của `admin.user` mà không ai thấy (luật 5, bất biến 3b).
 *
 * FAIL CLOSED: không đọc được danh sách quyền thì coi như KHÔNG có quyền. "Chưa rõ" không được
 * hành xử như "có" — trên đường cách ly không có giá trị mặc định nào (luật 1, cấm #1).
 */
export function quyetDinhTheoKhoa(
  ketQua: KetQua<identity_phienHienTaiRa>,
  khoa: string,
): QuyetDinhHien {
  if (!ketQua.ok) return { hien: false, vi: "khong-doc-duoc", thongBao: ketQua.thongBao };

  return coQuyen(ketQua.duLieu.permissions, khoa)
    ? { hien: true }
    : { hien: false, vi: "khong-du-quyen" };
}

/**
 * Có đúng khoá quyền này hay không. **So sánh chuỗi chính xác, không tiền tố, không ký tự thay
 * thế.**
 *
 * Một quyền là MỘT khoá phẳng `"<nhóm>.<việc>"` (luật 5, bất biến 3b), không phải một cây.
 * Nếu ở đây có một phép khớp kiểu `admin.*` thì `admin.audit` — quyền xem nhật ký hệ thống —
 * sẽ mở luôn tab quản lý người dùng, và bộ quyền của khách không phải tích Descartes: các khoá
 * ấy được liệt kê từng cái một chính vì `admin.audit` cố ý không phải `admin.user`.
 *
 * KHÔNG NHẮC MỘT CON SỐ TỔNG Ở ĐÂY: danh mục quyền nằm trong CSDL (`quyen`, do máy chủ phát ra
 * theo `GET /api/v1/role-permissions`), và một con số chép vào mã là con số sai vào ngày khách
 * chốt thêm một khoá — sai lặng lẽ, vì không bài test nào đọc lại nó.
 */
export function coQuyen(dsQuyen: readonly string[], khoa: string): boolean {
  return dsQuyen.includes(khoa);
}
