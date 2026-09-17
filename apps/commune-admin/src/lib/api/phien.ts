/**
 * Gọi hai route phiên làm việc của dịch vụ identity.
 *
 * KIỂU LẤY TỪ HỢP ĐỒNG, KHÔNG GÕ TAY: mọi hình dạng dưới đây đến từ `schema.gen.ts`, sinh ra
 * từ kb/20-contracts/openapi.json. Không có một `type` nào mô tả lại thân yêu cầu hay thân
 * phản hồi ở tệp này — đó là điều kiện để bản sao thứ hai không tồn tại.
 *
 * VÌ SAO ĐƯỜNG DẪN TƯƠNG ĐỐI, KHÔNG PHẢI MỘT `NEXT_PUBLIC_API_BASE_URL`:
 *
 *   Hai bất biến ràng buộc nhau. (a) Xã được suy từ `Host` ở rìa ngoài cùng, nên yêu cầu phải
 *   mang đúng Host của xã. (b) Cookie phiên là host-only theo đúng host ấy (luật 1, cấm #3).
 *   Chỉ còn một hình trạng thoả cả hai: API phục vụ trên chính host của xã
 *   (`thangbinh.vigov.vn/api/v1/...`). Đường dẫn tương đối là cách duy nhất không nung một
 *   host nào vào bundle — và cũng khiến không ai *có thể* trỏ nhầm sang host của xã khác.
 *
 * KHÔNG CÓ `tenant_id` Ở BẤT KỲ ĐÂU trong tệp này — không trong thân, không trong query,
 * không trong header. Client tự khai xã là client tự cấp quyền (luật 1, cấm #2).
 *
 * KHÔNG ĐỤNG COOKIE. Cookie phiên do dịch vụ identity đặt bằng Set-Cookie (httpOnly, secure,
 * SameSite=Lax, KHÔNG có thuộc tính Domain). Trình duyệt tự giữ. Mã ở đây chỉ bật
 * `credentials` để nó được gửi kèm; không đọc, không ghi, không xoá.
 */

import type {
  httpx_Error,
  identity_delete_sessions_by_sid,
  identity_phanHoiDangNhap,
  identity_post_sessions,
  identity_thanDangNhap,
} from "./schema.gen";

/**
 * Kết quả một lời gọi: hoặc dữ liệu, hoặc **một** thông báo cho người dùng đọc.
 *
 * Cố ý KHÔNG mang theo mã lỗi ra tới giao diện. Giao diện chỉ có đúng một chuỗi để hiển thị,
 * nên không có chỗ nào để rẽ nhánh "email sai" so với "mật khẩu sai" — xem `thongBaoLoi`.
 */
export type KetQua<T> = { ok: true; duLieu: T } | { ok: false; thongBao: string };

/** Câu trả lời khi không đọc nổi thân lỗi của máy chủ. Không bao giờ lộ chi tiết kỹ thuật. */
const LOI_KHONG_RO = "Không kết nối được máy chủ. Vui lòng thử lại.";

/**
 * Lấy câu thông báo do máy chủ viết, nguyên văn.
 *
 * ĐÂY LÀ CHỖ DỄ LÀM HỎNG NHẤT MÀN ĐĂNG NHẬP: dịch vụ identity cố ý trả **cùng một** `code` và
 * **cùng một** `message` cho email sai, mật khẩu sai và trường bỏ trống. Phân biệt chúng ở
 * giao diện là dựng lại đúng thứ máy chủ vừa giấu đi — tức là để người ngoài dò ra địa chỉ thư
 * công vụ nào có thật trên tên miền của xã, và danh bạ cán bộ của một cơ quan nhà nước không
 * phải thứ cho không. Vì vậy hàm này chỉ lấy `message` và không bao giờ đọc `code` để đổi chữ.
 */
async function thongBaoLoi(phanHoi: Response): Promise<string> {
  try {
    const than = (await phanHoi.json()) as httpx_Error;
    return typeof than?.message === "string" && than.message !== "" ? than.message : LOI_KHONG_RO;
  } catch {
    return LOI_KHONG_RO;
  }
}

const CHUNG: RequestInit = {
  // same-origin: cookie phiên đi kèm vì API nằm trên chính host của xã. Không dùng "include" —
  // "include" chỉ cần thiết khi gửi sang origin khác, mà gửi phiên sang origin khác là đúng
  // điều không được phép xảy ra ở đây.
  credentials: "same-origin",
  // Không cache một lời gọi phiên làm việc, ở bất kỳ tầng nào.
  cache: "no-store",
};

/**
 * POST /api/v1/sessions — mở một phiên.
 *
 * Trả về `sid` để về sau tự kết thúc phiên của mình, `expires_at`, và khối cán bộ rút gọn.
 * Token KHÔNG nằm trong phản hồi: nó ở trong cookie httpOnly, nơi JavaScript không với tới.
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
