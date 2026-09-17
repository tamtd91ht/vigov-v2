/**
 * Cách gọi API dùng chung cho mọi tuyến — hình dạng yêu cầu và cách đọc thân lỗi.
 *
 * VÌ SAO TÁCH RA KHỎI `phien.ts`: hai quy tắc dưới đây phải giống nhau ở MỌI tuyến, và mỗi bản
 * sao của chúng là một bản sao sẽ trôi. Bản sao thứ hai không làm đỏ test nào: nó chỉ làm một
 * màn hình nào đó bắt đầu rẽ nhánh theo `code`, hoặc gửi `credentials: "include"`, ở đúng một
 * chỗ mà không ai đọc lại.
 *
 *   1. ĐƯỜNG DẪN TƯƠNG ĐỐI, KHÔNG BAO GIỜ MỘT HOST. API phục vụ trên chính host của xã, nên
 *      yêu cầu mang đúng `Host` ấy và cookie phiên (host-only) đi kèm được. Không một host nào
 *      bị nung vào bundle, và không ai *có thể* trỏ nhầm sang host của xã khác (luật 1, bất
 *      biến 10).
 *   2. KHÔNG CÓ `tenant_id` Ở BẤT KỲ ĐÂU — không thân, không query, không header. Client tự
 *      khai xã là client tự cấp quyền (luật 1, cấm #2).
 *
 * KHÔNG ĐỤNG COOKIE. Cookie phiên do dịch vụ identity đặt bằng Set-Cookie (httpOnly, secure,
 * SameSite=Lax, KHÔNG có thuộc tính Domain). Mã ở đây chỉ bật `credentials` để nó được gửi
 * kèm; không đọc, không ghi, không xoá.
 */

import type { httpx_Error } from "./schema.gen";

/**
 * Kết quả một lời gọi: hoặc dữ liệu, hoặc **một** thông báo cho người dùng đọc.
 *
 * Cố ý KHÔNG mang theo mã lỗi ra tới giao diện. Giao diện chỉ có đúng một chuỗi để hiển thị,
 * nên không có chỗ nào để rẽ nhánh theo `code` — xem `thongBaoLoi`.
 */
export type KetQua<T> = { ok: true; duLieu: T } | { ok: false; thongBao: string };

/** Câu trả lời khi không đọc nổi thân lỗi của máy chủ. Không bao giờ lộ chi tiết kỹ thuật. */
export const LOI_KHONG_RO = "Không kết nối được máy chủ. Vui lòng thử lại.";

/**
 * Lấy câu thông báo do máy chủ viết, nguyên văn.
 *
 * MỌI mã lỗi đều trả về cùng một hình dạng `httpx.Error` (`code` · `message` · `trace_id`) —
 * kể cả 401, 403 và 404. Nên ở đây chỉ cần đọc `message`, và **chỉ được** đọc `message`:
 *
 *   - KHÔNG rẽ nhánh giao diện theo `code` để đoán chuyện gì đã xảy ra. `code` là định danh
 *     cho máy, và ở màn đăng nhập máy chủ CỐ Ý trả cùng một `code` cho email sai và mật khẩu
 *     sai — dựng lại sự phân biệt ấy ở client là dựng lại đúng thứ máy chủ vừa giấu đi.
 *   - KHÔNG hiện `trace_id`. Nó là mốc để tra log, không phải mã lỗi nghiệp vụ; đặt nó cạnh
 *     một câu thông báo là mời cán bộ đọc nó thành "mã lỗi" rồi đọc lại qua điện thoại.
 */
export async function thongBaoLoi(phanHoi: Response): Promise<string> {
  try {
    const than = (await phanHoi.json()) as httpx_Error;
    return typeof than?.message === "string" && than.message !== "" ? than.message : LOI_KHONG_RO;
  } catch {
    return LOI_KHONG_RO;
  }
}

export const CHUNG: RequestInit = {
  // same-origin: cookie phiên đi kèm vì API nằm trên chính host của xã. Không dùng "include" —
  // "include" chỉ cần thiết khi gửi sang origin khác, mà gửi phiên sang origin khác là đúng
  // điều không được phép xảy ra ở đây.
  credentials: "same-origin",
  // Không cache lời gọi nào ở đây, ở bất kỳ tầng nào: cả phiên làm việc lẫn danh bạ cán bộ đều
  // là dữ liệu của một xã cụ thể, và một bản đệm sống lâu hơn yêu cầu là đúng hình dạng của
  // một lần dữ liệu xã này hiện trên màn hình xã khác.
  cache: "no-store",
};

/**
 * GET một tuyến, trả về `KetQua`. Chỉ 200 là thành công; mọi mã khác thành một câu thông báo.
 *
 * Kiểu `T` do bên gọi truyền vào TỪ `schema.gen.ts` — hàm này không biết hình dạng nào cả, nên
 * không có chỗ nào ở đây gõ tay lại một hình dạng của hợp đồng.
 */
export async function docJSON<T>(duongDan: string): Promise<KetQua<T>> {
  let phanHoi: Response;
  try {
    phanHoi = await fetch(duongDan, { ...CHUNG, method: "GET" });
  } catch {
    // Mạng hỏng. Không ghi log gì: phản hồi của các tuyến này chứa dữ liệu cá nhân (luật 3).
    return { ok: false, thongBao: LOI_KHONG_RO };
  }

  if (phanHoi.status !== 200) return { ok: false, thongBao: await thongBaoLoi(phanHoi) };

  try {
    return { ok: true, duLieu: (await phanHoi.json()) as T };
  } catch {
    // 200 mà thân không phải JSON là máy chủ hoặc proxy đang trả thứ khác. Với người dùng thì
    // đó vẫn là "không đọc được", không phải một trạng thái nghiệp vụ.
    return { ok: false, thongBao: LOI_KHONG_RO };
  }
}
