/**
 * LỚP DUY NHẤT CHẠM VÀO `zmp-sdk` — và nó cố tình nhỏ đúng bằng ba lời gọi.
 *
 * Tệp này là chỗ duy nhất trong cả kho được phép nhắc tên mô-đun `zmp-sdk` và tên ba hàm
 * `getPhoneNumber` · `getLocation` · `scanQRCode`. `phase1-collects-nothing.test.ts` cấm chúng ở
 * MỌI tệp khác — lệnh cấm không bị xoá, nó được thu hẹp phạm vi, và có một ca chứng minh rằng
 * ngoài `src/features/quyen/` thì nó vẫn bắt.
 *
 * ⚠ NHẬP ĐỘNG, TRONG HÀM, BỌC TRY/CATCH — BA ĐIỀU KIỆN, KHÔNG PHẢI KHẨU VỊ:
 *
 *   `zmp-sdk` chạm `window` NGAY LÚC NẠP MÔ-ĐUN. Một `import` tĩnh ở đầu tệp làm sập mọi test
 *   chạy dưới Node (không có `window`) trước khi một phép kiểm nào kịp chạy, và làm sập ngay cả
 *   những test không liên quan gì tới ba màn này. Nhập động bên trong hàm thì mô-đun chỉ được nạp
 *   khi công dân đã bấm nút.
 *
 *   Ngoài Zalo, lời nhập ấy hỏng. Đó KHÔNG phải một sự cố cần giấu: hàm trả về `ngoai-zalo` và
 *   màn hình nói ra bằng tiếng Việt. Một `catch {}` im lặng ở đây là một màn hình trắng trên máy
 *   người duyệt, và một màn hình trắng là câu trả lời tệ nhất cho một vòng xét duyệt.
 *
 * ⚠ ĐỌC ĐÚNG `token`, KHÔNG ĐỌC `number` / `latitude` / `longitude`.
 *
 *   Đo từ `node_modules/zmp-sdk/index.d.ts` (2.53.0), không phải từ tài liệu web:
 *
 *     GetPhoneNumberReturns = { number?: @deprecated; token?: string }
 *     GetLocationReturns    = { latitude?/longitude?/timestamp?/provider?: @deprecated; token?: string }
 *     ScanQRCodeReturns     = { content: string }
 *
 *   Nền tảng vừa bỏ đường đưa số điện thoại và toạ độ về thiết bị. Đọc lại mấy trường ấy là tự
 *   rước dữ liệu cá nhân về máy đúng lúc nó đã hết cần thiết — và dữ liệu cá nhân nằm trên thiết
 *   bị là thứ luật 3 nói tới, không phải một chi tiết kỹ thuật.
 *
 * ⚠ KHÔNG `console.log`, KHÔNG `fetch`, KHÔNG CHỖ LƯU NÀO. Nội dung mã QR có thể là bất cứ thứ
 * gì, kể cả dữ liệu cá nhân của người khác. Hiện lên màn hình rồi thôi; ghi nó ra log là đưa nó
 * vào một nơi không ai gỡ lại được.
 */

/**
 * Kết quả một lần xin quyền. TỪ CHỐI LÀ MỘT NHÁNH RIÊNG, NGANG HÀNG VỚI THÀNH CÔNG — không phải
 * một lỗi: người dân có quyền nói không, và một app coi đó là lỗi sẽ hiện một câu trách móc.
 */
export type KetQuaXin<T> =
  | { kieu: "xong"; du_lieu: T }
  | { kieu: "tu-choi" }
  | { kieu: "ngoai-zalo" }
  | { kieu: "khong-lay-duoc" };

/**
 * Mã lỗi người dùng từ chối. Lấy từ chính ví dụ trong `zmp-sdk/index.d.ts` (`if (code === -201)`
 * — "Người dùng đã từ chối cấp quyền"), không phải từ trí nhớ.
 *
 * Không khớp mã này thì rơi vào `khong-lay-duoc`, và câu hiện ra vẫn nói việc cần làm tiếp. Đoán
 * sai theo hướng ấy thì tệ nhất là một câu chung chung; đoán sai theo hướng ngược lại là mắng
 * người vừa bấm "Từ chối" rằng họ gặp lỗi.
 */
const MA_TU_CHOI = -201;

/** Lỗi SDK ném ra: `{ code: number; message?: string }`. Chỉ đọc `code` — `message` là chữ kỹ
 *  thuật, và README §Error message shape cấm đưa nó ra cho người dùng. */
function laTuChoi(loi: unknown): boolean {
  return typeof loi === "object" && loi !== null && (loi as { code?: unknown }).code === MA_TU_CHOI;
}

/**
 * Gọi một API của Zalo và quy mọi đường về bốn nhánh trên.
 *
 * `nap` tách khỏi `goi` vì hai lời hứa khác nhau: nhập mô-đun hỏng nghĩa là KHÔNG Ở TRONG ZALO
 * (nói ra được, sửa được bằng cách mở trong Zalo), còn lời gọi hỏng nghĩa là người dùng từ chối
 * hoặc nền tảng không trả lời. Gộp hai thứ vào một `catch` thì màn hình nói sai một trong hai.
 */
async function xin<T>(goi: (sdk: typeof import("zmp-sdk")) => Promise<T>): Promise<KetQuaXin<T>> {
  let sdk: typeof import("zmp-sdk");
  try {
    sdk = await import("zmp-sdk");
  } catch {
    return { kieu: "ngoai-zalo" };
  }

  try {
    return { kieu: "xong", du_lieu: await goi(sdk) };
  } catch (loi) {
    return laTuChoi(loi) ? { kieu: "tu-choi" } : { kieu: "khong-lay-duoc" };
  }
}

/** Token số điện thoại. Chuỗi rỗng là câu trả lời thật của nền tảng ở môi trường phát triển. */
export function xinTokenSoDienThoai(): Promise<KetQuaXin<string>> {
  return xin(async (sdk) => (await sdk.getPhoneNumber()).token ?? "");
}

/** Token vị trí. Không đọc `latitude`/`longitude` — xem khối chú thích đầu tệp. */
export function xinTokenViTri(): Promise<KetQuaXin<string>> {
  return xin(async (sdk) => (await sdk.getLocation()).token ?? "");
}

/** Nội dung mã QR — API DUY NHẤT ở đây trả về dữ liệu thật, không phải token. */
export function quetMaQR(): Promise<KetQuaXin<string>> {
  return xin(async (sdk) => (await sdk.scanQRCode()).content);
}
