/**
 * Gọi các route phiên làm việc của dịch vụ identity.
 *
 * KIỂU LẤY TỪ HỢP ĐỒNG, KHÔNG GÕ TAY: mọi hình dạng dưới đây đến từ `schema.gen.ts`, sinh ra
 * từ kb/20-contracts/openapi.json. Không có một `type` nào mô tả lại thân yêu cầu hay thân
 * phản hồi ở tệp này — đó là điều kiện để bản sao thứ hai không tồn tại.
 *
 * VÌ SAO ĐƯỜNG DẪN TƯƠNG ĐỐI, VÌ SAO KHÔNG ĐỤNG COOKIE, VÌ SAO KHÔNG CÓ `tenant_id`: ba quy
 * tắc ấy đúng cho MỌI tuyến, nên chúng được nói một lần ở `goi.ts` và dùng lại ở đây. Đọc tệp
 * ấy trước khi sửa bất kỳ lời gọi nào trong tệp này.
 */

import { CHUNG, LOI_KHONG_RO, docJSON, thongBaoLoi, type KetQua } from "./goi";
import type {
  identity_delete_sessions_by_sid,
  identity_get_sessions_current,
  identity_phanHoiDangNhap,
  identity_phienHienTaiRa,
  identity_post_sessions,
  identity_thanDangNhap,
} from "./schema.gen";

/**
 * POST /api/v1/sessions — mở một phiên.
 *
 * Trả về `sid` để về sau tự kết thúc phiên của mình, `expires_at`, và khối cán bộ rút gọn.
 * Token KHÔNG nằm trong phản hồi: nó ở trong cookie httpOnly, nơi JavaScript không với tới.
 *
 * ĐÂY LÀ CHỖ DỄ LÀM HỎNG NHẤT MÀN ĐĂNG NHẬP: dịch vụ identity cố ý trả **cùng một** `code` và
 * **cùng một** `message` cho email sai, mật khẩu sai và trường bỏ trống. Phân biệt chúng ở giao
 * diện là dựng lại đúng thứ máy chủ vừa giấu đi — tức là để người ngoài dò ra địa chỉ thư công
 * vụ nào có thật trên tên miền của xã, và danh bạ cán bộ của một cơ quan nhà nước không phải
 * thứ cho không. `thongBaoLoi` vì vậy chỉ đọc `message`, không bao giờ đọc `code` để đổi chữ.
 */
export async function dangNhap(
  than: identity_thanDangNhap,
): Promise<KetQua<identity_phanHoiDangNhap>> {
  const duongDan: identity_post_sessions["duongDan"] = "/api/v1/sessions";

  let phanHoi: Response;
  try {
    phanHoi = await fetch(duongDan, {
      ...CHUNG,
      method: "POST",
      headers: { "Content-Type": "application/json" },
      // Đúng hai trường của hợp đồng. Không kèm xã, không kèm host, không kèm thiết bị.
      body: JSON.stringify({ email: than.email, password: than.password }),
    });
  } catch {
    // Mạng hỏng. Không ghi log gì ở đây: thân yêu cầu chứa mật khẩu (luật 3).
    return { ok: false, thongBao: LOI_KHONG_RO };
  }

  if (phanHoi.status === 201) {
    const duLieu = (await phanHoi.json()) as identity_phanHoiDangNhap;
    return { ok: true, duLieu };
  }
  return { ok: false, thongBao: await thongBaoLoi(phanHoi) };
}

/**
 * GET /api/v1/sessions/current — phiên hiện tại là của ai, và mang những quyền nào.
 *
 * DÙNG ĐỂ ẨN/HIỆN, KHÔNG PHẢI ĐỂ CHO PHÉP. Danh sách `permissions` trả về đây chỉ quyết định
 * cán bộ có **thấy** một tab hay không. Việc **được làm** thì do dịch vụ kiểm trên từng yêu cầu
 * thật: `GET /api/v1/staff` đòi `admin.user` ở phía máy chủ và trả 403 cho ai không có, bất kể
 * trình duyệt đã hiện gì (luật 5, cấm #1 — kiểm quyền ở giao diện thay cho tầng dịch vụ là
 * không kiểm gì cả, vì mã client sửa được).
 *
 * Tuyến này là `any-authenticated` chứ không đòi quyền nào: một tài khoản vừa bị gỡ hết vai trò
 * vẫn phải hỏi được "tôi là ai", nếu không thì nó không còn đường nào biết mình mất quyền.
 *
 * PHẠM VI: hàm này mới chỉ phục vụ việc ẩn/hiện. Phần còn lại của tuyến — hiện họ tên và chức
 * vụ trên đầu trang, rẽ trang chủ theo vai trò (`15-phu-luc-giao-dien-chung §1`) — vẫn là việc
 * `tasks/web/open/4141103d6d0a.json`, chưa ai nhận.
 */
export async function layPhienHienTai(): Promise<KetQua<identity_phienHienTaiRa>> {
  const duongDan: identity_get_sessions_current["duongDan"] = "/api/v1/sessions/current";
  return docJSON<identity_phienHienTaiRa>(duongDan);
}

/**
 * DELETE /api/v1/sessions/{sid} — kết thúc phiên của chính mình.
 *
 * 401 và 404 được coi là **đã xong**: phiên không còn hiệu lực nữa, và đó đúng là điều người
 * dùng vừa yêu cầu. Bắt họ ở lại một màn hình lỗi trong khi phiên đã chết là sai cả về an
 * toàn lẫn về lễ độ. 404 ở đây cố ý không phân biệt với "sid của người khác" — sự tồn tại của
 * phiên người khác tự nó đã là thông tin.
 */
export async function dangXuat(sid: string): Promise<KetQua<null>> {
  const thamSo: identity_delete_sessions_by_sid["thamSo"] = { sid };
  const duongDan = `/api/v1/sessions/${encodeURIComponent(thamSo.sid)}`;

  let phanHoi: Response;
  try {
    phanHoi = await fetch(duongDan, { ...CHUNG, method: "DELETE" });
  } catch {
    return { ok: false, thongBao: LOI_KHONG_RO };
  }

  if (phanHoi.status === 204 || phanHoi.status === 401 || phanHoi.status === 404) {
    return { ok: true, duLieu: null };
  }
  return { ok: false, thongBao: await thongBaoLoi(phanHoi) };
}
