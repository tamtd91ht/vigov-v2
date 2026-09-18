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
 * KHÔNG CÓ HÀM GHI Ở ĐÂY, VÀ KHÔNG CÓ KHUNG ĐỂ VỀ SAU ĐIỀN VÀO.
 *
 * Hợp đồng REST không có tuyến nào đổi một ô của ma trận. Nút `Lưu` ở đầu mỗi cột mà đặc tả vẽ
 * (§4 và §12.5) nằm trên hai câu khách chưa chốt (`kb/00-foundation/open-questions.json`):
 *
 *   #13  có chặn thao tác làm xã mất người quản trị CUỐI CÙNG không. Gỡ khoá `admin.user` khỏi
 *        vai trò cuối cùng còn giữ nó là đúng một lần bấm trên màn hình này, và sau lần bấm ấy
 *        không ai trong xã mở lại được — nhà cung cấp cũng không được phép chạm vào (ADR 0003).
 *   #14  người giữ `admin.user` có được thao tác lên CHÍNH MÌNH không.
 *
 * Một hàm ghi viết dở trông y hệt một quyết định đã có người ra. Xem thêm chú thích đầu
 * `service-identity/internal/http/quyen.go` — lập luận đầy đủ nằm ở phía máy chủ, một chỗ.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 */

import { docJSON, type KetQua } from "./goi";
import type { identity_get_role_permissions, identity_maTranQuyenRa } from "./schema.gen";

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
