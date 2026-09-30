/**
 * Tuyến đọc của tab "Phân quyền": `GET /api/v1/role-permissions`.
 *
 * KIỂU LẤY TỪ HỢP ĐỒNG, KHÔNG GÕ TAY: `identity_maTranQuyenRa` đến từ `schema.gen.ts`. Không tệp
 * nào trong ứng dụng này mô tả lại hình dạng của một nhóm quyền, một cột vai trò hay một ô đã cấp
 * — có bản thứ hai là có hai bản sẽ trôi (luật 9).
 *
 * MỘT LỜI GỌI CHO CẢ MA TRẬN, VÀ ĐÓ LÀ HÌNH DẠNG CỦA HỢP ĐỒNG chứ không phải một phép gộp ở đây:
 * hàng (`groups`), cột (`roles`) và ô đã cấp (`grants`) về trong cùng một phản hồi vì ma trận chỉ
 * đúng khi ba thứ ấy được đọc ở cùng một thời điểm. Ba lời gọi rời nhau có thể vắt qua một lần
 * đổi quyền và cho ra một dấu tích nằm ở cột không còn tồn tại (`service-identity/internal/http/quyen.go`).
 *
 * VÌ SAO ĐƯỜNG DẪN TƯƠNG ĐỐI, VÌ SAO KHÔNG ĐỤNG COOKIE, VÌ SAO KHÔNG CÓ `tenant_id`: ba quy tắc
 * ấy đúng cho MỌI tuyến và được nói một lần ở `goi.ts`. Đọc tệp ấy trước khi sửa gì ở đây.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * MỘT HÀM GHI, VÀ NÓ GHI ĐÚNG MỘT CỘT: `PUT /api/v1/roles/{id}/permissions`.
 *
 * Tuyến ấy mọc ở `service-identity` sau khi #13 và #14 được chốt (commit 85e6e84): máy chủ từ chối
 * thao tác làm xã mất người giữ `admin.user`/`admin.role` cuối cùng (409 `last_holder`), từ chối
 * người sửa chính vai trò mình đang giữ (403 `self_target_forbidden`), và từ chối cấp HOẶC gỡ một
 * khoá mà người sửa không giữ (403 `permission_escalation`). Cả ba là quy tắc CỦA MÁY CHỦ; màn
 * hình không dựng bản sao nào của chúng, chỉ hiện nguyên câu máy chủ viết.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 */

import { docJSON, docThanLoiGoi, goiGhi, type KetQua } from "./goi";
import type {
  identity_cotPhanQuyenRa,
  identity_get_role_permissions,
  identity_luuPhanQuyenVao,
  identity_maTranQuyenRa,
  identity_put_roles_by_id_permissions,
} from "./schema.gen";

/**
 * GET /api/v1/role-permissions — nhóm quyền, vai trò kèm hai số đếm, và các ô đã cấp của xã.
 *
 * Máy chủ đòi quyền `admin.role`; thiếu quyền là 403 và `docJSON` biến nó thành một câu thông
 * báo do chính máy chủ viết. KHÔNG PHÂN TRANG: tuyến trả nguyên ma trận hoặc trả 500 — một ma
 * trận bị cắt bớt đọc ra y hệt "vai trò này không có quyền đó".
 *
 * KHÔNG CÓ DỮ LIỆU CÁ NHÂN TRONG PHẢN HỒI NÀY (luật 3): con số duy nhất nói về người là số đếm.
 * Đừng gọi thêm tuyến nào để đổi số ấy thành danh sách tên — một màn hình sinh ra để xem quyền
 * không phải chỗ liệt kê ai giữ vai trò nào.
 */
export function layMaTranQuyen(): Promise<KetQua<identity_maTranQuyenRa>> {
  const duongDan: identity_get_role_permissions["duongDan"] = "/api/v1/role-permissions";
  return docJSON<identity_maTranQuyenRa>(duongDan);
}

/**
 * PUT /api/v1/roles/{id}/permissions — lưu MỘT cột của ma trận (`docs/ui-ux/14-cau-hinh.md §12.5`).
 *
 * THÂN LÀ TOÀN BỘ tập quyền của vai trò ấy, không phải phần chênh lệch: khoá nào không có trong
 * `permissions` là khoá máy chủ GỠ. `[]` hợp lệ và nghĩa là gỡ hết; `null`/thiếu trường là 400.
 * Vì vậy thân do bên gọi dựng từ đúng tập của một cột (`thanLuuCot` trong
 * `features/cau-hinh/sua-phan-quyen.ts`) — hàm này không tự gộp, không tự lọc, không gửi cột nào khác.
 *
 * 200 trả tập quyền ĐÃ LƯU, sắp theo chữ. Màn hình thay cột bằng tập ấy chứ không bằng thứ nó đã
 * gửi: câu trả lời của máy chủ là trạng thái thật, còn thứ gửi đi chỉ là một đề nghị.
 */
export function luuPhanQuyenVaiTro(
  vaiTroId: string,
  than: identity_luuPhanQuyenVao,
): Promise<KetQua<identity_cotPhanQuyenRa>> {
  const mau: identity_put_roles_by_id_permissions["duongDan"] = "/api/v1/roles/{id}/permissions";
  return docThanLoiGoi<identity_cotPhanQuyenRa>(
    goiGhi(mau.replace("{id}", encodeURIComponent(vaiTroId)), "PUT", than, 200),
  );
}
