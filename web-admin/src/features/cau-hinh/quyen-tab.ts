/**
 * Quyết định tab "Người dùng" có hiện hay không, tách khỏi phần dựng giao diện.
 *
 * VÌ SAO TÁCH: đây là chỗ duy nhất trong ứng dụng đọc `permissions` để ẩn/hiện, và nó phải
 * **đóng khi không chắc**. Nằm lẫn trong một component `.tsx` thì ca "không đọc được quyền"
 * không có bài test nào chạm tới — mà đó đúng là ca sẽ xảy ra khi phiên vừa hết hạn.
 *
 * NHẮC LẠI CHO NGƯỜI SỬA TỆP NÀY: hàm này là TIỆN DỤNG, không phải biện pháp. Nó trả `true` sai
 * thì hậu quả là một màn hình hiện lỗi 403 — dữ liệu không rời khỏi máy chủ, vì
 * `GET /api/v1/staff` kiểm `admin.user` trên từng yêu cầu ở phía máy chủ (luật 5, cấm #1).
 */

import type { KetQua } from "@/lib/api/goi";
import type { identity_phienHienTaiRa } from "@/lib/api/schema.gen";
import { coQuyen, QUYEN_QUAN_LY_NGUOI_DUNG } from "@/lib/quyen";

export type QuyetDinhTab =
  | { hien: true }
  /** Đọc được quyền, và tài khoản không có `admin.user`. Đặc tả §12.8: ẩn tab đó. */
  | { hien: false; vi: "khong-du-quyen" }
  /** Không đọc được quyền — phiên hết hạn, mạng hỏng, máy chủ lỗi. Vẫn là ẩn. */
  | { hien: false; vi: "khong-doc-duoc"; thongBao: string };

export function quyetDinhTabNguoiDung(
  ketQua: KetQua<identity_phienHienTaiRa>,
): QuyetDinhTab {
  // FAIL CLOSED: không đọc được danh sách quyền thì coi như KHÔNG có quyền. "Chưa rõ" không
  // được hành xử như "có" — trên đường cách ly thì không có giá trị mặc định nào (luật 1).
  if (!ketQua.ok) return { hien: false, vi: "khong-doc-duoc", thongBao: ketQua.thongBao };

  return coQuyen(ketQua.duLieu.permissions, QUYEN_QUAN_LY_NGUOI_DUNG)
    ? { hien: true }
    : { hien: false, vi: "khong-du-quyen" };
}
